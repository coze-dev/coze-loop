// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/bytedance/gg/gptr"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCurrentRequestLockBusyBrokerRetry(t *testing.T) {
	ctrl := gomock.NewController(t)
	mutex := lockmocks.NewMockILocker(ctrl)
	mutex.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(false, context.Background(), func() {}, nil)
	svc := &ExptItemEventEvalServiceImpl{mutex: mutex}
	called := false
	err := svc.HandleEventLock(func(context.Context, *entity.ExptItemEvalEvent) error { called = true; return nil })(context.Background(), &entity.ExptItemEvalEvent{ExptID: 1, EvalSetItemID: 2})
	require.True(t, errno.NeedMQRetry(err), "busy lock must reach broker, never ACK")
	require.False(t, called)
}

func TestCurrentRequestBrokerRetryBypassesBudget(t *testing.T) {
	ctrl := gomock.NewController(t)
	config := componentmocks.NewMockIConfiger(ctrl)
	svc := &ExptItemEventEvalServiceImpl{configer: config}
	want := errno.NewMQRetryErr("source read unavailable")
	err := svc.HandleEventErr(func(context.Context, *entity.ExptItemEvalEvent) error { return want })(context.Background(), &entity.ExptItemEvalEvent{})
	require.Same(t, want, err, "broker errors must bypass budget, fail and release")
}

func TestCurrentRequestManualMissingTurnNoNX(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeFailRetry, entity.EvaluationModeRetryItems} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			ctx := &entity.ExptItemEvalCtx{Event: &entity.ExptItemEvalEvent{RequestID: "rq1", ExptID: 1, ExptRunID: 2, EvalSetItemID: 11, ExptRunMode: mode}, EvalSetItem: &entity.EvaluationSetItem{Turns: []*entity.Turn{{ID: 21}, {ID: 22}}}, ExistItemEvalResult: &entity.ExptItemEvalResult{ItemResultRunLog: &entity.ExptItemResultRunLog{}, TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{21: {TurnID: 21, ExptID: 1, ExptRunID: 2, ItemID: 11}}}}
			var pre RecordEvalMode
			if mode == entity.EvaluationModeFailRetry {
				pre = &ExptRecordEvalModeFailRetry{}
			} else {
				pre = &ExptRecordEvalModeRetryIgnoreResult{}
			}
			err := pre.PreEval(context.Background(), ctx)
			require.True(t, errno.NeedMQRetry(err), "missing frozen turn must not repair or ACK")
		})
	}
}

func TestCurrentRequestConsumerSourceGuard(t *testing.T) {
	for _, kind := range []string{"stale", "empty_on_manual", "terminal", "read_error", "queueing", "processing"} {
		t.Run(kind, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			items := repomocks.NewMockIExptItemResultRepo(ctrl)
			src := &entity.ExptItemCurrentRequest{RunID: 2, ItemID: 11, RequestID: "rq1", Mode: entity.EvaluationModeFailRetry, Status: entity.ItemRunState_Processing, RetryTimes: 3}
			event := &entity.ExptItemEvalEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2, EvalSetItemID: 11, RequestID: "rq1", ExptRunMode: entity.EvaluationModeFailRetry}
			readErr := error(nil)
			wantCalled := false
			switch kind {
			case "stale":
				event.RequestID = "rq0"
			case "empty_on_manual":
				event.RequestID = ""
			case "terminal":
				src.Status = entity.ItemRunState_Success
			case "read_error":
				readErr = errors.New("DB unavailable")
			case "queueing":
				src.Status = entity.ItemRunState_Queueing
				wantCalled = true
				items.EXPECT().MarkProcessingIfCurrent(gomock.Any(), int64(3), int64(1), int64(2), int64(11), "rq1").Return(true, nil)
			case "processing":
				wantCalled = true
			}
			items.EXPECT().GetCurrentRequest(gomock.Any(), int64(3), int64(1), int64(2), int64(11)).Return(src, readErr)
			svc := &ExptItemEventEvalServiceImpl{exptItemResultRepo: items}
			called := false
			err := svc.HandleCurrentSource(func(context.Context, *entity.ExptItemEvalEvent) error { called = true; return nil })(context.Background(), event)
			require.Equal(t, wantCalled, called)
			if readErr != nil {
				require.True(t, errno.NeedMQRetry(err))
			} else {
				require.NoError(t, err)
			}
			if wantCalled {
				require.Equal(t, 3, event.RetryTimes)
			}
		})
	}
}

func TestCurrentRequestDuplicateInFlightNoInvoke(t *testing.T) {
	for _, kind := range []string{"target", "registered", "inline", "old_map"} {
		t.Run(kind, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			manager := svcmocks.NewMockIExptManager(ctrl)
			items := repomocks.NewMockIExptItemResultRepo(ctrl)
			turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
			dataset := svcmocks.NewMockEvaluationSetItemService(ctrl)
			target := svcmocks.NewMockIEvalTargetService(ctrl)
			evaluators := svcmocks.NewMockEvaluatorRecordService(ctrl)
			expt := &entity.Experiment{ID: 1, SpaceID: 3, EvalSet: &entity.EvaluationSet{EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 7, EvaluationSetID: 8}}}
			item := &entity.EvaluationSetItem{ID: 11, Turns: []*entity.Turn{{ID: 21}}}
			log := &entity.ExptTurnResultRunLog{SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 11, TurnID: 21, Status: entity.TurnRunState_Processing}
			switch kind {
			case "target":
				log.TargetResultID = 41
				target.EXPECT().GetRecordByID(gomock.Any(), int64(3), int64(41)).Return(&entity.EvalTargetRecord{ID: 41, Status: gptr.Of(entity.EvalTargetRunStatusAsyncInvoking)}, nil)
			case "registered":
				log.EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 51, Alias: "a", RecordID: 61}}}
			case "inline":
				log.EvaluatorResultIds = &entity.EvaluatorResults{Inline: []*entity.InlineEvalResult{{RecordID: 61}}}
			case "old_map":
				log.EvaluatorResultIds = &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{51: 61}}
			}
			if kind != "target" {
				evaluators.EXPECT().GetEvaluatorRecord(gomock.Any(), int64(61), false).Return(&entity.EvaluatorRecord{ID: 61, Status: entity.EvaluatorRunStatusAsyncInvoking}, nil)
			}
			manager.EXPECT().GetDetail(gomock.Any(), int64(1), int64(3), gomock.Any()).Return(expt, nil)
			items.EXPECT().GetItemRunLog(gomock.Any(), int64(1), int64(2), int64(11), int64(3)).Return(&entity.ExptItemResultRunLog{Status: int32(entity.ItemRunState_Processing)}, nil).Times(2)
			turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), int64(1), int64(2), int64(11), int64(3)).Return([]*entity.ExptTurnResultRunLog{log}, nil)
			dataset.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{item}, nil)
			svc := &ExptItemEventEvalServiceImpl{manager: manager, exptItemResultRepo: items, exptTurnResultRepo: turns, evaluationSetItemService: dataset, evaTargetService: target, evaluatorRecordService: evaluators}
			require.NoError(t, svc.eval(context.Background(), &entity.ExptItemEvalEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2, EvalSetItemID: 11, ExptRunMode: entity.EvaluationModeSubmit}))
		})
	}
}

func TestCurrentRequestManualCompletePreEvalReuse(t *testing.T) {
	event := &entity.ExptItemEvalEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2, EvalSetItemID: 11, RequestID: "rq1"}
	log := &entity.ExptTurnResultRunLog{SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 11, TurnID: 21, Status: entity.TurnRunState_Success, TargetResultID: 41}
	ctx := &entity.ExptItemEvalCtx{Event: event, EvalSetItem: &entity.EvaluationSetItem{Turns: []*entity.Turn{{ID: 21}}}, ExistItemEvalResult: &entity.ExptItemEvalResult{ItemResultRunLog: &entity.ExptItemResultRunLog{}, TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{21: log}}}
	require.NoError(t, (&ExptRecordEvalModeFailRetry{}).PreEval(context.Background(), ctx))
	require.Same(t, log, ctx.GetExistTurnResultRunLog(21))
	require.NoError(t, (&ExptRecordEvalModeRetryIgnoreResult{}).PreEval(context.Background(), ctx))
	require.Equal(t, int64(41), log.TargetResultID)
}

func TestCurrentRequestConsumerCASLostToCompletion(t *testing.T) {
	for _, status := range []entity.ItemRunState{entity.ItemRunState_Processing, entity.ItemRunState_Success} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			items := repomocks.NewMockIExptItemResultRepo(ctrl)
			event := &entity.ExptItemEvalEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2, EvalSetItemID: 11, ExptRunMode: entity.EvaluationModeSubmit}
			items.EXPECT().GetCurrentRequest(gomock.Any(), int64(3), int64(1), int64(2), int64(11)).Return(&entity.ExptItemCurrentRequest{RunID: 2, ItemID: 11, Mode: entity.EvaluationModeSubmit, Status: entity.ItemRunState_Queueing}, nil)
			items.EXPECT().MarkProcessingIfCurrent(gomock.Any(), int64(3), int64(1), int64(2), int64(11), "").Return(false, nil)
			items.EXPECT().GetCurrentRequest(gomock.Any(), int64(3), int64(1), int64(2), int64(11)).Return(&entity.ExptItemCurrentRequest{RunID: 2, ItemID: 11, Mode: entity.EvaluationModeSubmit, Status: status}, nil)
			called := false
			svc := &ExptItemEventEvalServiceImpl{exptItemResultRepo: items}
			err := svc.HandleCurrentSource(func(context.Context, *entity.ExptItemEvalEvent) error { called = true; return nil })(context.Background(), event)
			require.NoError(t, err)
			require.Equal(t, status == entity.ItemRunState_Processing, called)
		})
	}
}

func TestCurrentRequestManualUnknownFrozenTurn(t *testing.T) {
	event := &entity.ExptItemEvalEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2, EvalSetItemID: 11, RequestID: "rq1"}
	eiec := &entity.ExptItemEvalCtx{Event: event, EvalSetItem: &entity.EvaluationSetItem{Turns: []*entity.Turn{{ID: 21}}}, ExistItemEvalResult: &entity.ExptItemEvalResult{ItemResultRunLog: &entity.ExptItemResultRunLog{}, TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{21: {SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 11, TurnID: 21, Status: 9}}}}
	require.True(t, errno.NeedMQRetry((&ExptRecordEvalModeFailRetry{}).PreEval(context.Background(), eiec)))
}
