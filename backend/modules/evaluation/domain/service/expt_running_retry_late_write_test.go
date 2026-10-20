// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"github.com/bytedance/gg/gptr"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"strconv"
	"testing"
	"time"
)

// Removing the source check would let the endpoint confirm quota or call providers.
func TestLateWriteConsumerOriginalResume(t *testing.T) {
	for _, kind := range []string{"target_stale", "evaluator_stale", "reader_failure", "turn0", "builtin0", "inline_stale", "alias_stale", "oldmap_stale", "missing_legacy", "incomplete_record", "unknown_record"} {
		t.Run(kind, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			items := repomocks.NewMockIExptItemResultRepo(ctrl)
			turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
			records := svcmocks.NewMockEvaluatorRecordService(ctrl)
			target := svcmocks.NewMockIEvalTargetService(ctrl)
			event := &entity.ExptItemEvalEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2, EvalSetItemID: 11, ExptRunMode: entity.EvaluationModeSubmit, RequestID: "rq1"}
			source := &entity.ExptItemCurrentRequest{SpaceID: 3, ExptID: 1, RunID: 2, ItemID: 11, ItemVersionID: 31, Mode: entity.EvaluationModeSubmit, RequestID: "rq1", Status: entity.ItemRunState_Processing}
			items.EXPECT().GetCurrentRequest(gomock.Any(), int64(3), int64(1), int64(2), int64(11)).Return(source, nil)
			log := &entity.ExptTurnResultRunLog{SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 11, ItemVersionID: 31, TurnID: 0, TargetResultID: 41, Status: entity.TurnRunState_Processing}
			log.EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 51, Alias: "a", RecordID: 61}}}
			event.ResumeSource = &entity.ExptResumeSource{Kind: "evaluator", TurnID: 0, RecordID: 61, InvokeID: strconv.FormatInt(61, 10)}
			event.AsyncEvaluatorReportTrigger = true
			record := &entity.EvaluatorRecord{ID: 61, SpaceID: 3, ExperimentID: 1, ExperimentRunID: 2, ItemID: 11, ItemVersionID: 31, TurnID: 0, EvaluatorVersionID: 51, Alias: "a", TargetRecordID: 0, Status: entity.EvaluatorRunStatusSuccess}
			var readErr error
			switch kind {
			case "target_stale", "turn0":
				event.ResumeSource.Kind = "target"
				event.ResumeSource.RecordID = 41
				event.ResumeSource.InvokeID = "41"
				event.AsyncEvaluatorReportTrigger = false
				event.AsyncReportTrigger = true
				target.EXPECT().GetRecordByID(gomock.Any(), int64(3), int64(41)).Return(&entity.EvalTargetRecord{ID: 41, SpaceID: 3, ExperimentRunID: 2, ItemID: 11, ItemVersionID: 31, TurnID: 0, Status: gptr.Of(entity.EvalTargetRunStatusSuccess)}, nil).AnyTimes()
				if kind == "target_stale" {
					log.TargetResultID = 42
				}
			case "evaluator_stale":
				log.EvaluatorResultIds.Registered[0].RecordID = 62
			case "inline_stale":
				record.SourceType = entity.EvaluatorRecordSourceTypeInline
				record.InlineKey = "a"
				record.TargetRecordID = 41
				log.EvaluatorResultIds = &entity.EvaluatorResults{Inline: []*entity.InlineEvalResult{{InlineKey: "a", RecordID: 62}}}
			case "alias_stale":
				log.EvaluatorResultIds.Registered[0].Alias = "b"
			case "oldmap_stale":
				record.Alias = ""
				log.EvaluatorResultIds = &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{51: 62}}
			case "reader_failure":
				readErr = errors.New("primary source unavailable")
			case "incomplete_record":
				record.ExperimentRunID = 0
			case "unknown_record":
				record.Status = entity.EvaluatorRunStatusUnknown
			case "missing_legacy":
				event.ResumeSource = nil
				event.RequestID = ""
				source.RequestID = ""
			}
			turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), int64(1), int64(2), int64(11), int64(3)).Return([]*entity.ExptTurnResultRunLog{log}, readErr).AnyTimes()
			records.EXPECT().GetEvaluatorRecord(gomock.Any(), int64(61), false).Return(record, nil).AnyTimes()
			svc := &ExptItemEventEvalServiceImpl{exptItemResultRepo: items, exptTurnResultRepo: turns, evaluatorRecordService: records, evaTargetService: target}
			called := false
			err := svc.HandleCurrentSource(func(context.Context, *entity.ExptItemEvalEvent) error { called = true; return nil })(context.Background(), event)
			require.Equal(t, kind == "turn0" || kind == "builtin0", called, "noncurrent original invocation must have zero downstream side effects")
			if kind == "reader_failure" || kind == "missing_legacy" || kind == "incomplete_record" || kind == "unknown_record" {
				require.True(t, errno.NeedMQRetry(err))
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// Cancelled lock ownership must stop storage and error handling before any fallback writes.
func TestLateWriteCancelledExecution(t *testing.T) {
	ctrl := gomock.NewController(t)
	items := repomocks.NewMockIExptItemResultRepo(ctrl)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	executor := &ExptItemEvalCtxExecutor{ItemResultRepo: items}
	event := &entity.ExptItemEvalEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2, EvalSetItemID: 11, ExptRunMode: entity.EvaluationModeSubmit}
	require.True(t, errno.NeedMQRetry(executor.CompleteItemRun(ctx, &entity.ExptItemEvalCtx{Event: event}, nil)))
	svc := &ExptItemEventEvalServiceImpl{exptItemResultRepo: items}
	require.True(t, errno.NeedMQRetry(svc.HandleEventErr(func(context.Context, *entity.ExptItemEvalEvent) error { return errors.New("business error") })(ctx, event)))
}

func TestLateWriteStaleSweepNoSideEffects(t *testing.T) {
	ctrl := gomock.NewController(t)
	items := repomocks.NewMockIExptItemResultRepo(ctrl)
	config := componentmocks.NewMockIConfiger(ctrl)
	config.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{ExptExecConf: &entity.ExptExecConf{ExptItemEvalConf: &entity.ExptItemEvalConf{ZombieSecond: 1}}})
	old := time.Now().Add(-time.Hour)
	turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
	turns.EXPECT().MGetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	svc := &ExptSchedulerImpl{ExptItemResultRepo: items, ExptTurnResultRepo: turns, Configer: config}
	// A same-scan original source is mandatory; an absent source must protect, never write.
	_, _, err := svc.handleZombies(context.Background(), &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2}, []*entity.ExptEvalItem{{ExptID: 1, ItemID: 11, RequestID: "rq1", State: entity.ItemRunState_Processing, UpdatedAt: &old}}, nil)
	require.True(t, errno.NeedMQRetry(err))
}

func TestLateWriteArmStopsAfterOwnershipCancellation(t *testing.T) {
	ctrl := gomock.NewController(t)
	turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
	evaluators := svcmocks.NewMockEvaluatorService(ctrl)
	ctx, cancel := context.WithCancel(context.Background())
	turns.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, []*entity.ExptTurnResultRunLog) error { cancel(); return nil })
	evaluators.EXPECT().ArmEvaluatorResume(gomock.Any(), gomock.Any()).Times(0)
	executor := &ExptItemEvalCtxExecutor{TurnResultRepo: turns, evaluatorService: evaluators}
	etec := &entity.ExptTurnEvalCtx{Turn: &entity.Turn{ID: 0}, ExptItemEvalCtx: &entity.ExptItemEvalCtx{Event: &entity.ExptItemEvalEvent{ExptRunID: 2}, Expt: &entity.Experiment{ID: 1, SpaceID: 3}, EvalSetItem: &entity.EvaluationSetItem{ItemID: 11}, ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{0: {ID: 21, TurnID: 0}}}}}
	err := executor.storeTurnRunResult(ctx, etec, &entity.ExptTurnRunResult{EvaluatorResults: []*entity.EvaluatorRecord{{ID: 61, EvaluatorVersionID: 51, Status: entity.EvaluatorRunStatusAsyncInvoking}}})
	require.True(t, errno.NeedMQRetry(err))
}

func TestLateWriteZombieUsesFrozenRecordIDs(t *testing.T) {
	ctrl := gomock.NewController(t)
	turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
	records := repomocks.NewMockIEvaluatorRecordRepo(ctrl)
	// A reader returning a newer invocation must never supply cleanup identity.
	turns.EXPECT().MGetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{{TargetResultID: 42, EvaluatorResultIds: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{51: 62}}}}, nil).AnyTimes()
	records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{61}, false, false).Return(nil, nil)
	source := &entity.ExptCurrentItemRunSource{Turns: map[int64]*entity.ExptTurnResultRunLog{0: {ItemID: 11, TargetResultID: 41, EvaluatorResultIds: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{51: 61}}}}}
	ctx := context.WithValue(context.Background(), currentItemSourceKey{}, &currentItemExecution{Source: source})
	svc := &ExptSchedulerImpl{ExptTurnResultRepo: turns, EvaluatorRecordRepo: records}
	require.NoError(t, svc.terminateZombieEvaluatorRecords(ctx, &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2}, []int64{11}))
}
