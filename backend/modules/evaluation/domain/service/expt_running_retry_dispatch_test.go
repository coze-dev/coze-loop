// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCurrentRequestProducerMQBeforeCAS(t *testing.T) {
	ctrl := gomock.NewController(t)
	items := repomocks.NewMockIExptItemResultRepo(ctrl)
	mutex := lockmocks.NewMockILocker(ctrl)
	publisher := eventmocks.NewMockExptEventPublisher(ctrl)
	config := componentmocks.NewMockIConfiger(ctrl)
	event := &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2, ExptRunMode: entity.EvaluationModeSubmit, ItemRetryTimes: 5, Ext: map[string]string{entity.RetryYieldExtKey: "true", "frozen": "keep"}, Session: &entity.Session{UserID: "original", AppID: 4}}
	source := &entity.ExptItemCurrentRequest{RunID: 2, ItemID: 11, RequestID: "rq1", Mode: entity.EvaluationModeFailRetry, Status: entity.ItemRunState_Queueing, RetryTimes: 3}
	published := false
	mutex.EXPECT().LockWithRenew(gomock.Any(), "expt_item_eval_run_lock:1:11", gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
	mutex.EXPECT().Unlock("expt_item_eval_run_lock:1:11").Return(true, nil)
	items.EXPECT().GetCurrentRequest(gomock.Any(), int64(3), int64(1), int64(2), int64(11)).Return(source, nil)
	config.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).Return(&entity.ExptExecConf{ExptItemEvalConf: &entity.ExptItemEvalConf{IntervalSecond: 2}})
	publisher.EXPECT().BatchPublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, got []*entity.ExptItemEvalEvent, interval *time.Duration) error {
		require.Len(t, got, 1)
		require.Equal(t, "rq1", got[0].RequestID)
		require.Equal(t, entity.EvaluationModeFailRetry, got[0].ExptRunMode)
		require.Equal(t, 3, got[0].RetryTimes)
		require.Equal(t, 5, got[0].MaxRetryTimes)
		require.Equal(t, event.Session, got[0].Session)
		require.Equal(t, event.Ext, got[0].Ext)
		require.Equal(t, 2*time.Second, *interval)
		require.NotSame(t, event.Session, got[0].Session)
		got[0].Ext["frozen"] = "mutated"
		require.Equal(t, "keep", event.Ext["frozen"])
		published = true
		return nil
	})
	want := errors.New("DB unavailable after MQ")
	items.EXPECT().MarkProcessingIfCurrent(gomock.Any(), int64(3), int64(1), int64(2), int64(11), "rq1").DoAndReturn(func(context.Context, int64, int64, int64, int64, string) (bool, error) {
		require.True(t, published)
		return false, want
	})
	svc := &ExptSchedulerImpl{ExptItemResultRepo: items, Mutex: mutex, itemLocker: func() lock.ILocker { return mutex }, Configer: config, Publisher: publisher}
	err := svc.handleToSubmits(context.Background(), event, []*entity.ExptEvalItem{{ItemID: 11, State: entity.ItemRunState_Queueing, RequestID: "rq1"}})
	require.ErrorIs(t, err, want)
}

func TestCurrentRequestScanOneObservation(t *testing.T) {
	ctrl := gomock.NewController(t)
	items := repomocks.NewMockIExptItemResultRepo(ctrl)
	config := componentmocks.NewMockIConfiger(ctrl)
	config.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).Return(&entity.ExptExecConf{ExptItemEvalConf: &entity.ExptItemEvalConf{ConcurNum: 2, MaxItemConcurNum: 2}}).AnyTimes()
	items.EXPECT().ScanCurrentRequests(gomock.Any(), int64(3), int64(1), int64(2), false).Return([]*entity.ExptItemCurrentRequest{{ItemID: 10, Status: entity.ItemRunState_Processing}, {ItemID: 11, Status: entity.ItemRunState_Queueing, RequestID: "rq1", Mode: entity.EvaluationModeFailRetry, RetryTimes: 3}, {ItemID: 12, Status: entity.ItemRunState_Queueing, RequestID: "rq2", Mode: entity.EvaluationModeRetryItems}}, nil)
	expt := &entity.Experiment{SpaceID: 3, EvalConf: &entity.EvaluationConfiguration{}, EvalSet: &entity.EvaluationSet{EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 7}}}
	exec := &exptBaseExec{exptItemResultRepo: items, configer: config}
	to, incomplete, _, err := exec.ScanEvalItems(context.Background(), &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2}, expt)
	require.NoError(t, err)
	require.Len(t, incomplete, 1)
	require.Len(t, to, 1)
	require.Equal(t, "rq1", to[0].RequestID)
	require.Equal(t, int32(3), to[0].RetryTimes)
}

func TestCurrentRequestProducerInterleavings(t *testing.T) {
	for _, kind := range []string{"publish_error", "publish_unknown", "fast_consumer", "busy", "changed_token", "raw", "fail_retry", "retry_items"} {
		t.Run(kind, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			items := repomocks.NewMockIExptItemResultRepo(ctrl)
			mutex := lockmocks.NewMockILocker(ctrl)
			pub := eventmocks.NewMockExptEventPublisher(ctrl)
			config := componentmocks.NewMockIConfiger(ctrl)
			mode := entity.EvaluationModeSubmit
			request := ""
			if kind == "fail_retry" {
				mode = entity.EvaluationModeFailRetry
				request = "rq1"
			}
			if kind == "retry_items" {
				mode = entity.EvaluationModeRetryItems
				request = "rq2"
			}
			src := &entity.ExptItemCurrentRequest{RunID: 2, ItemID: 11, Status: entity.ItemRunState_Queueing, Mode: mode, RequestID: request, RetryTimes: 3}
			event := &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2, ExptRunMode: entity.EvaluationModeSubmit, ItemRetryTimes: 5}
			mutex.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(kind != "busy", context.Background(), func() {}, nil)
			wantErr := false
			if kind != "busy" {
				mutex.EXPECT().Unlock(gomock.Any()).Return(true, nil)
				if kind == "changed_token" {
					src.RequestID = "new"
				}
				items.EXPECT().GetCurrentRequest(gomock.Any(), int64(3), int64(1), int64(2), int64(11)).Return(src, nil)
				if kind != "changed_token" {
					config.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).Return(&entity.ExptExecConf{})
					pub.EXPECT().BatchPublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, events []*entity.ExptItemEvalEvent, _ *time.Duration) error {
						require.Equal(t, mode, events[0].ExptRunMode)
						require.NotZero(t, events[0].ExptRunMode)
						require.Equal(t, request, events[0].RequestID)
						require.Equal(t, 3, events[0].RetryTimes)
						if kind == "publish_error" || kind == "publish_unknown" {
							return errors.New(kind)
						}
						if kind == "fast_consumer" {
							src.Status = entity.ItemRunState_Success
						}
						return nil
					})
					if kind == "publish_error" || kind == "publish_unknown" {
						wantErr = true
					} else {
						items.EXPECT().MarkProcessingIfCurrent(gomock.Any(), int64(3), int64(1), int64(2), int64(11), request).DoAndReturn(func(context.Context, int64, int64, int64, int64, string) (bool, error) {
							return src.Status == entity.ItemRunState_Queueing, nil
						})
					}
				}
			}
			svc := &ExptSchedulerImpl{ExptItemResultRepo: items, Publisher: pub, Configer: config, itemLocker: func() lock.ILocker { return mutex }}
			err := svc.handleToSubmits(context.Background(), event, []*entity.ExptEvalItem{{ItemID: 11, State: entity.ItemRunState_Queueing, RequestID: request}})
			if wantErr {
				require.Error(t, err)
				require.Equal(t, entity.ItemRunState_Queueing, src.Status)
			} else {
				require.NoError(t, err)
			}
			if kind == "fast_consumer" {
				require.Equal(t, entity.ItemRunState_Success, src.Status)
			}
			require.Equal(t, entity.EvaluationModeSubmit, event.ExptRunMode)
		})
	}
}

func TestCurrentRequestProducerSameBatchModes(t *testing.T) {
	ctrl := gomock.NewController(t)
	items := repomocks.NewMockIExptItemResultRepo(ctrl)
	pub := eventmocks.NewMockExptEventPublisher(ctrl)
	config := componentmocks.NewMockIConfiger(ctrl)
	sources := []*entity.ExptItemCurrentRequest{{RunID: 2, ItemID: 10, Status: entity.ItemRunState_Queueing, Mode: entity.EvaluationModeSubmit, RetryTimes: 0}, {RunID: 2, ItemID: 11, Status: entity.ItemRunState_Queueing, Mode: entity.EvaluationModeFailRetry, RequestID: "rq1", RetryTimes: 3}, {RunID: 2, ItemID: 12, Status: entity.ItemRunState_Queueing, Mode: entity.EvaluationModeRetryItems, RequestID: "rq2", RetryTimes: 1}}
	locks := make([]lock.ILocker, 0, 3)
	candidates := make([]*entity.ExptEvalItem, 0, 3)
	for _, source := range sources {
		mutex := lockmocks.NewMockILocker(ctrl)
		locks = append(locks, mutex)
		mutex.EXPECT().LockWithRenew(gomock.Any(), fmt.Sprintf("expt_item_eval_run_lock:1:%d", source.ItemID), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		mutex.EXPECT().Unlock(gomock.Any()).Return(true, nil)
		candidates = append(candidates, &entity.ExptEvalItem{ItemID: source.ItemID, RequestID: source.RequestID, State: source.Status})
		items.EXPECT().GetCurrentRequest(gomock.Any(), int64(3), int64(1), int64(2), source.ItemID).Return(source, nil)
		items.EXPECT().MarkProcessingIfCurrent(gomock.Any(), int64(3), int64(1), int64(2), source.ItemID, source.RequestID).Return(true, nil)
	}
	config.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).Return(&entity.ExptExecConf{}).Times(3)
	got := make([]*entity.ExptItemEvalEvent, 0, 3)
	pub.EXPECT().BatchPublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, events []*entity.ExptItemEvalEvent, _ *time.Duration) error {
		got = append(got, events...)
		return nil
	}).Times(3)
	n := 0
	svc := &ExptSchedulerImpl{ExptItemResultRepo: items, Publisher: pub, Configer: config, itemLocker: func() lock.ILocker { mutex := locks[n]; n++; return mutex }}
	event := &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2, ExptRunMode: entity.EvaluationModeSubmit, ItemRetryTimes: 5, Session: &entity.Session{UserID: "frozen"}}
	require.NoError(t, svc.handleToSubmits(context.Background(), event, candidates))
	require.Len(t, got, 3)
	require.Equal(t, 3, n)
	for i, source := range sources {
		require.Equal(t, source.Mode, got[i].ExptRunMode)
		require.Equal(t, source.RequestID, got[i].RequestID)
		require.Equal(t, int(source.RetryTimes), got[i].RetryTimes)
		require.Equal(t, 5, got[i].MaxRetryTimes)
	}
	require.Equal(t, entity.EvaluationModeSubmit, event.ExptRunMode)
}
