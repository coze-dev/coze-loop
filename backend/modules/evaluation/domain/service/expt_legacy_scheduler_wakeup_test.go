// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type legacyWakeSourceRepo struct {
	repo.IExptItemResultRepo
	repo.IExptCurrentResultRepo
}

func TestLegacySchedulerWakeUsesCurrentRequestInsideItemLock(t *testing.T) {
	for _, kind := range []string{"processing", "queueing", "cas_lost", "old_target_async", "old_evaluator_async", "old_resume", "explicit_old_request", "wrong_run", "finished_item", "wrong_original_mode", "noncurrent_experiment", "source_changed", "unstable", "read_error", "ordinary_raw"} {
		t.Run(kind, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			items := repomocks.NewMockIExptItemResultRepo(ctrl)
			port := repomocks.NewMockIExptCurrentResultRepo(ctrl)
			expts := repomocks.NewMockIExperimentRepo(ctrl)
			manager := svcmocks.NewMockIExptManager(ctrl)
			factory, server := finishRealLockerFactory(t)
			source := &entity.ExptItemCurrentRequest{SpaceID: 9, ExptID: 7, RunID: 8, ItemID: 10, ItemVersionID: 11, RequestID: "current-request", Mode: entity.EvaluationModeRetryItems, Status: entity.ItemRunState_Processing}
			event := &entity.ExptItemEvalEvent{SpaceID: 9, ExptID: 7, ExptRunID: 8, EvalSetItemID: 10, ExptRunMode: entity.EvaluationModeSubmit, RetryTimes: 7, MaxRetryTimes: 12, Ext: map[string]string{"frozen": "original"}, Session: &entity.Session{UserID: "original-user"}}
			current := &entity.Experiment{ID: 7, SpaceID: 9, LatestRunID: 8, ExptType: entity.ExptType_Offline, Status: entity.ExptStatus_Processing}
			sourceErr := error(nil)
			wantCalled := kind == "processing" || kind == "queueing" || kind == "ordinary_raw"
			wantRetry := kind == "unstable" || kind == "read_error" || kind == "old_target_async" || kind == "old_evaluator_async" || kind == "old_resume"
			switch kind {
			case "queueing":
				source.Status = entity.ItemRunState_Queueing
			case "old_target_async":
				event.AsyncReportTrigger = true
			case "old_evaluator_async":
				event.AsyncEvaluatorReportTrigger = true
			case "old_resume":
				event.ResumeSource = &entity.ExptResumeSource{Kind: "target", RecordID: 71, InvokeID: "71"}
			case "explicit_old_request":
				event.RequestID = "old-request"
			case "wrong_run":
				event.ExptRunID = 18
			case "finished_item":
				source.Status = entity.ItemRunState_Success
			case "wrong_original_mode":
				event.ExptRunMode = entity.EvaluationModeRetryAll
			case "noncurrent_experiment":
				current.LatestRunID = 18
			case "unstable":
				source.ObservationUnstable = true
			case "read_error":
				sourceErr = errors.New("source read unavailable")
			case "ordinary_raw":
				source.RequestID = ""
				source.Mode = entity.EvaluationModeSubmit
			}
			items.EXPECT().GetCurrentRequest(gomock.Any(), int64(9), int64(7), event.ExptRunID, int64(10)).Return(source, sourceErr)
			observesExperiment := kind == "processing" || kind == "queueing" || kind == "cas_lost" || kind == "wrong_original_mode" || kind == "noncurrent_experiment" || kind == "source_changed" || kind == "old_target_async" || kind == "old_evaluator_async" || kind == "old_resume"
			if observesExperiment {
				expts.EXPECT().GetByID(gomock.Any(), int64(7), int64(9)).DoAndReturn(func(ctx context.Context, _, _ int64) (*entity.Experiment, error) {
					require.True(t, contexts.CtxWriteDB(ctx))
					require.True(t, server.Exists("expt_item_eval_run_lock:7:10"))
					return current, nil
				})
				if kind != "noncurrent_experiment" {
					manager.EXPECT().GetRunLog(gomock.Any(), int64(7), int64(8), int64(9), gomock.Any()).Return(&entity.ExptRunLog{SpaceID: 9, ExptID: 7, ExptRunID: 8, Mode: int32(entity.EvaluationModeSubmit), Status: int64(entity.ExptStatus_Processing)}, nil)
				}
			}
			reachesFrozen := wantCalled || kind == "cas_lost" || kind == "source_changed" || kind == "old_target_async" || kind == "old_evaluator_async" || kind == "old_resume"
			if reachesFrozen {
				frozen := &entity.ExptCurrentItemRunSource{Current: *source, Turns: map[int64]*entity.ExptTurnResultRunLog{}}
				if kind == "old_target_async" || kind == "old_evaluator_async" || kind == "old_resume" {
					frozen.Turns[0] = &entity.ExptTurnResultRunLog{SpaceID: 9, ExptID: 7, ExptRunID: 8, ItemID: 10, ItemVersionID: 11, TurnID: 0, Status: entity.TurnRunState_Processing}
				}
				if kind == "source_changed" {
					frozen.Current.RequestID = "replacement-request"
				}
				port.EXPECT().GetCurrentItemRunSource(gomock.Any(), int64(9), int64(7), int64(8), int64(10)).Return(frozen, nil)
				if kind == "queueing" {
					items.EXPECT().MarkProcessingIfCurrent(gomock.Any(), int64(9), int64(7), int64(8), int64(10), "current-request").Return(true, nil)
				}
				if kind != "source_changed" && kind != "old_target_async" && kind != "old_evaluator_async" && kind != "old_resume" {
					port.EXPECT().CompleteCurrentItemRun(gomock.Any(), frozen, gomock.Any()).DoAndReturn(func(_ context.Context, _ *entity.ExptCurrentItemRunSource, write *entity.ExptCurrentItemRunWrite) (bool, error) {
						require.True(t, write.RetryOnly)
						if kind == "ordinary_raw" {
							require.EqualValues(t, 7, write.RetryTimes)
						} else {
							require.Zero(t, write.RetryTimes)
						}
						return kind != "cas_lost", nil
					})
				}
			}
			svc := &ExptItemEventEvalServiceImpl{exptItemResultRepo: &legacyWakeSourceRepo{IExptItemResultRepo: items, IExptCurrentResultRepo: port}, experimentRepo: expts, manager: manager, itemLocker: factory}
			called := false
			err := svc.HandleEventLock(svc.HandleCurrentSource(func(ctx context.Context, got *entity.ExptItemEvalEvent) error {
				called = true
				require.True(t, server.Exists("expt_item_eval_run_lock:7:10"))
				require.Equal(t, 12, got.MaxRetryTimes)
				require.Equal(t, "original", got.Ext["frozen"])
				require.Equal(t, "original-user", got.Session.UserID)
				if kind == "ordinary_raw" {
					require.Empty(t, got.RequestID)
					require.Equal(t, 7, got.RetryTimes)
					require.Equal(t, entity.EvaluationModeSubmit, got.ExptRunMode)
				} else {
					require.Equal(t, "current-request", got.RequestID)
					require.Zero(t, got.RetryTimes)
					require.Equal(t, entity.EvaluationModeRetryItems, got.ExptRunMode)
				}
				return nil
			}))(context.Background(), event)
			require.Equal(t, wantCalled, called)
			if kind == "cas_lost" {
				require.ErrorIs(t, err, errExptNonCurrentSource)
			} else if wantRetry {
				require.True(t, errno.NeedMQRetry(err))
			} else {
				require.NoError(t, err)
			}
			require.False(t, server.Exists("expt_item_eval_run_lock:7:10"))
		})
	}
}

func TestLegacySchedulerWakeBusySourceStaysRetryable(t *testing.T) {
	ctrl := gomock.NewController(t)
	items := repomocks.NewMockIExptItemResultRepo(ctrl)
	port := repomocks.NewMockIExptCurrentResultRepo(ctrl)
	source := &entity.ExptCurrentItemRunSource{Current: entity.ExptItemCurrentRequest{SpaceID: 9, ExptID: 7, RunID: 8, ItemID: 10, RequestID: "current-request", Mode: entity.EvaluationModeRetryItems, Status: entity.ItemRunState_Processing}}
	items.EXPECT().GetCurrentRequest(gomock.Any(), int64(9), int64(7), int64(8), int64(10)).Return(&source.Current, nil)
	port.EXPECT().GetCurrentItemRunSource(gomock.Any(), int64(9), int64(7), int64(8), int64(10)).Return(source, nil)
	factory, server := finishRealLockerFactory(t)
	key := "expt_item_eval_run_lock:7:10"
	require.NoError(t, server.Set(key, "another-owner"))
	svc := &ExptItemEventEvalServiceImpl{exptItemResultRepo: &legacyWakeSourceRepo{IExptItemResultRepo: items, IExptCurrentResultRepo: port}, itemLocker: factory}
	called := false
	err := svc.HandleEventLock(func(context.Context, *entity.ExptItemEvalEvent) error { called = true; return nil })(context.Background(), &entity.ExptItemEvalEvent{SpaceID: 9, ExptID: 7, ExptRunID: 8, EvalSetItemID: 10, ExptRunMode: entity.EvaluationModeSubmit})
	require.True(t, errno.NeedMQRetry(err))
	require.False(t, called)
	holder, err := server.Get(key)
	require.NoError(t, err)
	require.Equal(t, "another-owner", holder)
}
