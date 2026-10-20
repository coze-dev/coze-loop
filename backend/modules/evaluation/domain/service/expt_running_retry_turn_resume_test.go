// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"bytes"
	"context"
	"errors"
	"github.com/coze-dev/coze-loop/backend/infra/fileserver"
	fsmocks "github.com/coze-dev/coze-loop/backend/infra/fileserver/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	evalstorage "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/storage"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	benefitmocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/ctxcache"
)

// External boundaries only: EvalTurns, context construction, CallTarget and
// CallEvaluators stay real, so extra invokes, deductions or stores fail tests.
type turnResumeFixture struct {
	evalCalls atomic.Int64
	executor  *ExptItemEvalCtxExecutor
	item      *entity.ExptItemEvalCtx
	target    *svcmocks.MockIEvalTargetService
	evaluator *svcmocks.MockEvaluatorService
	records   *svcmocks.MockEvaluatorRecordService
	benefit   *benefitmocks.MockIBenefitService
	turns     *repomocks.MockIExptTurnResultRepo
	config    *configmocks.MockIConfiger
	items     *repomocks.MockIExptItemResultRepo
}

func newTurnResumeFixture(t *testing.T, mode entity.ExptRunMode, turnIDs ...int64) *turnResumeFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	f := &turnResumeFixture{target: svcmocks.NewMockIEvalTargetService(ctrl), evaluator: svcmocks.NewMockEvaluatorService(ctrl), records: svcmocks.NewMockEvaluatorRecordService(ctrl), benefit: benefitmocks.NewMockIBenefitService(ctrl), turns: repomocks.NewMockIExptTurnResultRepo(ctrl), config: configmocks.NewMockIConfiger(ctrl), items: repomocks.NewMockIExptItemResultRepo(ctrl)}
	metric := metricsmocks.NewMockExptMetric(ctrl)
	metric.EXPECT().EmitTurnExecEval(gomock.Any(), gomock.Any()).Do(func(_, _ int64) { f.evalCalls.Add(1) }).AnyTimes()
	metric.EXPECT().EmitTurnExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	metric.EXPECT().EmitTurnExecTargetResult(gomock.Any(), gomock.Any()).AnyTimes()
	metric.EXPECT().EmitTurnExecEvaluatorResult(gomock.Any(), gomock.Any()).AnyTimes()
	f.executor = &ExptItemEvalCtxExecutor{ItemResultRepo: f.items, TurnResultRepo: f.turns, Configer: f.config, Metric: metric, evalTargetService: f.target, evaluatorService: f.evaluator, evaluatorRecordService: f.records, benefitService: f.benefit}
	expt := buildRetryFailureSingleSetExpt(3, 401, 402)
	expt.Target.EvalTargetType = entity.EvalTargetTypeCustomRPCServer
	expt.EvalConf.ConnectorConf.TargetConf.IngressConf = &entity.TargetIngressConf{EvalSetAdapter: &entity.FieldAdapter{}}
	f.item = &entity.ExptItemEvalCtx{Expt: expt, Event: &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 103, SpaceID: 3, EvalSetItemID: 10, ExptRunMode: mode, RequestID: "RQ1", Session: &entity.Session{UserID: "u"}, MaxRetryTimes: 3}, EvalSetItem: &entity.EvaluationSetItem{ItemID: 10, BaseInfo: &entity.BaseInfo{CreatedAt: gptr.Of(int64(1))}}, ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{}}}
	for _, id := range turnIDs {
		f.item.EvalSetItem.Turns = append(f.item.EvalSetItem.Turns, &entity.Turn{ID: id, ItemID: 10})
		f.item.ExistItemEvalResult.TurnResultRunLogs[id] = &entity.ExptTurnResultRunLog{ID: 100 + id, ExptID: 1, ExptRunID: 103, SpaceID: 3, ItemID: 10, TurnID: id, Status: entity.TurnRunState_Processing}
	}
	f.items.EXPECT().GetItemRunLog(gomock.Any(), int64(1), int64(103), int64(10), int64(3)).Return(&entity.ExptItemResultRunLog{Status: int32(entity.ItemRunState_Processing)}, nil).AnyTimes()
	f.items.EXPECT().BatchGet(gomock.Any(), int64(3), int64(1), []int64{10}).Return(nil, nil).AnyTimes()
	f.config.EXPECT().BuildEvalExt(gomock.Any(), int64(3), gomock.Any()).Return(nil).AnyTimes()
	return f
}

func resumeTarget(id, turnID int64) *entity.EvalTargetRecord {
	return &entity.EvalTargetRecord{ID: id, TurnID: turnID, Status: gptr.Of(entity.EvalTargetRunStatusSuccess), EvalTargetOutputData: &entity.EvalTargetOutputData{OutputFields: map[string]*entity.Content{consts.OutputSchemaKey: {Text: gptr.Of("previous answer")}}}}
}
func resumeEvaluator(id, versionID, turnID int64, alias string) *entity.EvaluatorRecord {
	return &entity.EvaluatorRecord{ID: id, ExperimentID: 1, ExperimentRunID: 103, SpaceID: 3, ItemID: 10, TurnID: turnID, EvaluatorVersionID: versionID, Alias: alias, SourceType: entity.EvaluatorRecordSourceTypeBuiltin, Status: entity.EvaluatorRunStatusSuccess}
}

func TestRunningRetryTurnResume_SourceCallbackDoesNotMarkTargetCalled(t *testing.T) {
	for _, turnID := range []int64{0, 20} {
		t.Run(string(rune('a'+turnID)), func(t *testing.T) {
			f := newTurnResumeFixture(t, entity.EvaluationModeRetryItems, turnID)
			f.item.Event.AsyncReportTrigger = true
			f.item.Event.ResumeSource = &entity.ExptResumeSource{Kind: "target", TurnID: turnID, RecordID: 201, InvokeID: "201"}
			f.item.ExistItemEvalResult.TurnResultRunLogs[turnID].TargetResultID = 201
			target := resumeTarget(201, turnID)
			f.target.EXPECT().GetRecordByID(gomock.Any(), int64(3), int64(201)).Return(target, nil)
			ctx := ctxcache.Init(context.Background())
			etec, err := f.executor.buildExptTurnEvalCtx(ctx, f.item.EvalSetItem.Turns[0], f.item, nil)
			require.NoError(t, err)
			got, err := (&DefaultExptTurnEvaluationImpl{evalTargetService: f.target, benefitService: f.benefit}).CallTarget(ctx, etec)
			require.NoError(t, err)
			require.Same(t, target, got)
			assert.False(t, etec.Event.CtxTargetCalled(ctx), "callback reused target; no new invocation")
			assert.False(t, etec.Event.IgnoreExistedEvaluatorResult(ctx), "manual selected mode must not discard current callback refs")
		})
	}
}

func TestRunningRetryTurnResume_EventAndCacheIsolation(t *testing.T) {
	f := newTurnResumeFixture(t, entity.EvaluationModeFailRetry, 10, 20, 30)
	f.item.Event.AsyncEvaluatorReportTrigger = true
	f.item.Event.ResumeSource = &entity.ExptResumeSource{Kind: "evaluator", TurnID: 20, RecordID: 302, InvokeID: "302"}
	f.item.Event.Ext = map[string]string{"lane": "ppe_frozen"}
	ctx := ctxcache.Init(context.Background())
	first, err := f.executor.buildExptTurnEvalCtx(ctx, f.item.EvalSetItem.Turns[0], f.item, nil)
	require.NoError(t, err)
	middle, err := f.executor.buildExptTurnEvalCtx(ctx, f.item.EvalSetItem.Turns[1], f.item, nil)
	require.NoError(t, err)
	last, err := f.executor.buildExptTurnEvalCtx(ctx, f.item.EvalSetItem.Turns[2], f.item, nil)
	require.NoError(t, err)
	assert.False(t, first.Event.AsyncEvaluatorReportTrigger)
	assert.Equal(t, f.item.Event.ResumeSource, first.Event.ResumeSource)
	assert.NotSame(t, f.item.Event.ResumeSource, first.Event.ResumeSource)
	assert.True(t, middle.Event.AsyncEvaluatorReportTrigger)
	assert.Equal(t, int64(20), middle.Event.ResumeSource.TurnID)
	assert.False(t, last.Event.AsyncEvaluatorReportTrigger)
	assert.Equal(t, f.item.Event.ResumeSource, last.Event.ResumeSource)
	require.NotSame(t, f.item.Event, first.Event)
	require.NotSame(t, first.Event, last.Event)
	first.Event.WithCtxTargetCalled(ctx)
	assert.True(t, first.Event.IgnoreExistedEvaluatorResult(ctx))
	assert.False(t, middle.Event.CtxTargetCalled(ctx))
	assert.False(t, last.Event.IgnoreExistedEvaluatorResult(ctx))
	last.Event.Ext["lane"] = "mutated"
	assert.Equal(t, "ppe_frozen", f.item.Event.Ext["lane"])
	assert.Equal(t, "RQ1", last.Event.RequestID)
	assert.Equal(t, 3, last.Event.MaxRetryTimes)
	assert.Equal(t, "u", last.Event.Session.UserID)
}

func TestRunningRetryTurnResume_FailRetryOnlyInvokesE2(t *testing.T) {
	for _, format := range []string{"registered", "alias", "legacy", "inline"} {
		t.Run(format, func(t *testing.T) {
			f := newTurnResumeFixture(t, entity.EvaluationModeFailRetry, 20)
			log := f.item.ExistItemEvalResult.TurnResultRunLogs[20]
			log.TargetResultID = 201
			target := resumeTarget(201, 20)
			e1 := resumeEvaluator(301, 401, 20, "")
			switch format {
			case "legacy":
				log.EvaluatorResultIds = &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{401: 301}}
			case "alias":
				e1.Alias = "judge-a"
				f.item.Expt.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
				f.item.ItemConfig = &entity.ExptItemConfig{EvaluatorConfs: []*entity.ItemEvaluatorConf{{EvaluatorVersionID: 401, Alias: "judge-a"}, {EvaluatorVersionID: 402, Alias: "judge-b"}}}
				fallthrough
			default:
				log.EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 401, Alias: e1.Alias, RecordID: 301}}}
			}
			f.target.EXPECT().GetRecordByID(gomock.Any(), int64(3), int64(201)).Return(target, nil)
			loaded := []*entity.EvaluatorRecord{e1}
			ids := []int64{301}
			if format == "inline" {
				log.EvaluatorResultIds.Inline = []*entity.InlineEvalResult{{InlineKey: "quality", RecordID: 304}}
				inline := resumeEvaluator(304, 0, 20, "")
				inline.SourceType = entity.EvaluatorRecordSourceTypeInline
				inline.InlineKey = "quality"
				inline.TargetRecordID = 201
				loaded = append(loaded, inline)
				ids = append(ids, 304)
			}
			f.records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), ids, false, false).Return(loaded, nil)
			f.benefit.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil).Times(1)
			f.evaluator.EXPECT().ShouldInterceptEvaluator(gomock.Any(), gomock.Any()).Return(nil, false, nil).Times(1)
			var calls []int64
			var mu sync.Mutex
			f.evaluator.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
				mu.Lock()
				defer mu.Unlock()
				calls = append(calls, r.EvaluatorVersionID)
				require.Equal(t, int64(402), r.EvaluatorVersionID)
				return resumeEvaluator(303, 402, 20, r.Alias), nil
			}).Times(1)
			f.turns.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, rows []*entity.ExptTurnResultRunLog) error {
				require.Len(t, rows, 1)
				assert.Equal(t, int64(201), rows[0].TargetResultID)
				refs := rows[0].EvaluatorResultIds.Registered
				require.Len(t, refs, 2)
				assert.Equal(t, int64(301), refs[0].RecordID)
				assert.Equal(t, int64(303), refs[1].RecordID)
				if format == "inline" {
					require.Len(t, rows[0].EvaluatorResultIds.Inline, 1)
					assert.Equal(t, "quality", rows[0].EvaluatorResultIds.Inline[0].InlineKey)
					assert.Equal(t, int64(304), rows[0].EvaluatorResultIds.Inline[0].RecordID)
				}
				return nil
			})
			abort, err := f.executor.EvalTurns(ctxcache.Init(context.Background()), f.item)
			require.NoError(t, err)
			assert.False(t, abort)
			assert.Equal(t, []int64{402}, calls)
		})
	}
}

func TestRunningRetryTurnResume_RepeatedTargetCallbackKeepsAllRefs(t *testing.T) {
	for _, pending := range []bool{false, true} {
		t.Run(map[bool]string{false: "success no score", true: "async in flight"}[pending], func(t *testing.T) {
			f := newTurnResumeFixture(t, entity.EvaluationModeRetryItems, 0)
			f.item.Event.AsyncReportTrigger = true
			f.item.Event.ResumeSource = &entity.ExptResumeSource{Kind: "target", TurnID: 0, RecordID: 201, InvokeID: "201"}
			f.item.Expt.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
			f.item.ItemConfig = &entity.ExptItemConfig{EvaluatorConfs: []*entity.ItemEvaluatorConf{{EvaluatorVersionID: 401, Alias: "judge-a"}, {EvaluatorVersionID: 402, Alias: "judge-b"}}}
			log := f.item.ExistItemEvalResult.TurnResultRunLogs[0]
			log.TargetResultID = 201
			log.EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 401, Alias: "judge-a", RecordID: 301}, {VersionID: 402, Alias: "judge-b", RecordID: 302}}, Inline: []*entity.InlineEvalResult{{InlineKey: "quality", RecordID: 304}}}
			target := resumeTarget(201, 0)
			e1 := resumeEvaluator(301, 401, 0, "judge-a")
			e2 := resumeEvaluator(302, 402, 0, "judge-b")
			inline := resumeEvaluator(304, 0, 0, "")
			inline.SourceType = entity.EvaluatorRecordSourceTypeInline
			inline.InlineKey = "quality"
			inline.TargetRecordID = 201
			if pending {
				e2.Status = entity.EvaluatorRunStatusAsyncInvoking
				f.evaluator.EXPECT().ArmEvaluatorResume(gomock.Any(), int64(302)).Return(nil)
			}
			f.target.EXPECT().GetRecordByID(gomock.Any(), int64(3), int64(201)).Return(target, nil)
			f.records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{301, 302, 304}, false, false).Return([]*entity.EvaluatorRecord{e1, e2, inline}, nil)
			f.turns.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, rows []*entity.ExptTurnResultRunLog) error {
				assert.Equal(t, int64(201), rows[0].TargetResultID)
				ids := map[int64]bool{}
				for _, r := range rows[0].EvaluatorResultIds.Registered {
					ids[r.RecordID] = true
				}
				for _, r := range rows[0].EvaluatorResultIds.Inline {
					ids[r.RecordID] = true
				}
				assert.Equal(t, map[int64]bool{301: true, 302: true, 304: true}, ids)
				require.Len(t, rows[0].EvaluatorResultIds.Inline, 1)
				assert.Equal(t, "quality", rows[0].EvaluatorResultIds.Inline[0].InlineKey)
				return nil
			})
			abort, err := f.executor.EvalTurns(ctxcache.Init(context.Background()), f.item)
			require.NoError(t, err)
			assert.Equal(t, pending, abort)
		})
	}
}

func TestRunningRetryTurnResume_SuccessTurnOnlyFeedsHistory(t *testing.T) {
	for _, score := range []string{"zero", "absent"} {
		t.Run(score, func(t *testing.T) {
			f := newTurnResumeFixture(t, entity.EvaluationModeFailRetry, 0, 20)
			first := f.item.ExistItemEvalResult.TurnResultRunLogs[0]
			first.Status = entity.TurnRunState_Success
			first.TargetResultID = 201
			first.EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 401, RecordID: 301}, {VersionID: 402, RecordID: 302}}}
			target := resumeTarget(201, 0)
			e1 := resumeEvaluator(301, 401, 0, "")
			e2 := resumeEvaluator(302, 402, 0, "")
			if score == "zero" {
				e1.EvaluatorOutputData = &entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: gptr.Of(float64(0))}}
			}
			f.target.EXPECT().GetRecordByID(gomock.Any(), int64(3), int64(201)).Return(target, nil)
			f.records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{301, 302}, false, false).Return([]*entity.EvaluatorRecord{e1, e2}, nil)
			f.benefit.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil).Times(2)
			f.target.EXPECT().ExecuteTarget(gomock.Any(), int64(3), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, _ int64, _ int64, exec *entity.ExecuteTargetCtx, input *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error) {
				require.Equal(t, int64(20), exec.TurnID)
				require.Len(t, input.HistoryMessages, 1)
				assert.Equal(t, "previous answer", input.HistoryMessages[0].Content.GetText())
				return resumeTarget(202, 20), nil
			})
			f.evaluator.EXPECT().ShouldInterceptEvaluator(gomock.Any(), gomock.Any()).Return(nil, false, nil).Times(2)
			f.evaluator.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
				require.Equal(t, int64(20), r.TurnID)
				return resumeEvaluator(1000+r.EvaluatorVersionID, r.EvaluatorVersionID, 20, ""), nil
			}).Times(2)
			f.turns.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, rows []*entity.ExptTurnResultRunLog) error {
				assert.Equal(t, int64(20), rows[0].TurnID)
				return nil
			}).Times(1)
			abort, err := f.executor.EvalTurns(ctxcache.Init(context.Background()), f.item)
			require.NoError(t, err)
			assert.False(t, abort)
			assert.Equal(t, int64(1), f.evalCalls.Load(), "only the pending turn enters Eval")
			assert.Equal(t, entity.TurnRunState_Success, first.Status)
			assert.Equal(t, int64(301), first.EvaluatorResultIds.Registered[0].RecordID)
		})
	}
}

func TestRunningRetryTurnResume_ManualRetryExtNeverReachesActors(t *testing.T) {
	for _, multi := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "alias"}[multi], func(t *testing.T) {
			f := newTurnResumeFixture(t, entity.EvaluationModeFailRetry, 0)
			ext := map[string]string{entity.ManualRetryExtKey: `{"request_id":"RQ1"}`, "lane": "ppe_frozen", "session": "session-frozen", "topology": "double", "other": "keep"}
			f.item.Event.Ext = ext
			f.item.ItemConfig = &entity.ExptItemConfig{Ext: ext, EvalTargetConf: &entity.ItemTargetConf{RunConf: &entity.ItemRunConf{MaxTurns: 3}}}
			if multi {
				f.item.Expt.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
				f.item.ItemConfig.EvaluatorConfs = []*entity.ItemEvaluatorConf{{EvaluatorVersionID: 401, Alias: "a"}, {EvaluatorVersionID: 402, Alias: "b"}}
			}
			f.item.Expt.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConf[0].RunConf = &entity.EvaluatorRunConfig{EvaluatorRuntimeParam: &entity.RuntimeParam{JSONValue: gptr.Of(`{"model":"fixed"}`)}}
			f.benefit.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil).Times(2)
			check := func(got map[string]string) {
				t.Helper()
				assert.NotContains(t, got, entity.ManualRetryExtKey)
				for _, k := range []string{"lane", "session", "topology", "other"} {
					assert.Equal(t, ext[k], got[k])
				}
			}
			f.target.EXPECT().ExecuteTarget(gomock.Any(), int64(3), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, _ int64, _ int64, _ *entity.ExecuteTargetCtx, input *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error) {
				check(input.Ext)
				assert.Contains(t, input.Ext, consts.TargetExecuteExtRunConfKey)
				return resumeTarget(201, 0), nil
			})
			f.evaluator.EXPECT().ShouldInterceptEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, bool, error) {
				check(r.Ext)
				check(r.InputData.Ext)
				return nil, false, nil
			}).Times(2)
			f.evaluator.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
				check(r.Ext)
				check(r.InputData.Ext)
				if r.EvaluatorVersionID == 401 {
					assert.Equal(t, `{"model":"fixed"}`, r.InputData.Ext[consts.FieldAdapterBuiltinFieldNameRuntimeParam])
				}
				return resumeEvaluator(1000+r.EvaluatorVersionID, r.EvaluatorVersionID, 0, r.Alias), nil
			}).Times(2)
			f.turns.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).Return(nil)
			abort, err := f.executor.EvalTurns(ctxcache.Init(context.Background()), f.item)
			require.NoError(t, err)
			assert.False(t, abort)
			assert.Equal(t, `{"request_id":"RQ1"}`, ext[entity.ManualRetryExtKey])
		})
	}
}

func TestRunningRetryTurnResume_CallbackFlagsOnlyAffectMiddleTurn(t *testing.T) {
	for _, kind := range []string{"target", "evaluator"} {
		t.Run(kind, func(t *testing.T) {
			f := newTurnResumeFixture(t, entity.EvaluationModeFailRetry, 0, 20, 30)
			f.item.Event.AsyncReportTrigger = kind == "target"
			f.item.Event.AsyncEvaluatorReportTrigger = kind == "evaluator"
			f.item.Event.ResumeSource = &entity.ExptResumeSource{Kind: kind, TurnID: 20, RecordID: 202, InvokeID: "202"}
			first := f.item.ExistItemEvalResult.TurnResultRunLogs[0]
			first.Status = entity.TurnRunState_Success
			first.TargetResultID = 201
			middle := f.item.ExistItemEvalResult.TurnResultRunLogs[20]
			middle.TargetResultID = 202
			// The callback turn already owns both evaluator refs; target callback repeats
			// must wait/reuse them, while turn 30 retains ordinary execution behavior.
			middle.EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 401, RecordID: 301}, {VersionID: 402, RecordID: 302}}}
			f.target.EXPECT().GetRecordByID(gomock.Any(), int64(3), int64(201)).Return(resumeTarget(201, 0), nil)
			f.target.EXPECT().GetRecordByID(gomock.Any(), int64(3), int64(202)).Return(resumeTarget(202, 20), nil)
			f.records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{301, 302}, false, false).Return([]*entity.EvaluatorRecord{resumeEvaluator(301, 401, 20, ""), resumeEvaluator(302, 402, 20, "")}, nil)
			f.benefit.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil).Times(2)
			f.target.EXPECT().ExecuteTarget(gomock.Any(), int64(3), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, _ int64, _ int64, r *entity.ExecuteTargetCtx, input *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error) {
				assert.Equal(t, int64(30), r.TurnID)
				assert.Len(t, input.HistoryMessages, 2)
				return resumeTarget(203, 30), nil
			})
			f.evaluator.EXPECT().ShouldInterceptEvaluator(gomock.Any(), gomock.Any()).Return(nil, false, nil).Times(2)
			f.evaluator.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
				assert.Equal(t, int64(30), r.TurnID)
				return resumeEvaluator(1000+r.EvaluatorVersionID, r.EvaluatorVersionID, 30, ""), nil
			}).Times(2)
			var stored []int64
			f.turns.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, rows []*entity.ExptTurnResultRunLog) error {
				stored = append(stored, rows[0].TurnID)
				return nil
			}).Times(2)
			abort, err := f.executor.EvalTurns(ctxcache.Init(context.Background()), f.item)
			require.NoError(t, err)
			assert.False(t, abort)
			assert.Equal(t, []int64{20, 30}, stored)
			assert.Equal(t, int64(20), f.item.Event.ResumeSource.TurnID)
		})
	}
}

func TestRunningRetryTurnResume_NewTargetInOneTurnDoesNotInvalidateNextTurn(t *testing.T) {
	f := newTurnResumeFixture(t, entity.EvaluationModeFailRetry, 10, 20)
	// First turn needs a new target. Next turn has prepared successful T/E1;
	// the first turn's cache must not invalidate E1 and cause a second charge.
	next := f.item.ExistItemEvalResult.TurnResultRunLogs[20]
	next.TargetResultID = 202
	next.EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 401, RecordID: 301}}}
	f.target.EXPECT().GetRecordByID(gomock.Any(), int64(3), int64(202)).Return(resumeTarget(202, 20), nil)
	f.records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{301}, false, false).Return([]*entity.EvaluatorRecord{resumeEvaluator(301, 401, 20, "")}, nil)
	f.benefit.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil).Times(3)
	f.target.EXPECT().ExecuteTarget(gomock.Any(), int64(3), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(resumeTarget(201, 10), nil).Times(1)
	f.evaluator.EXPECT().ShouldInterceptEvaluator(gomock.Any(), gomock.Any()).Return(nil, false, nil).Times(3)
	var mu sync.Mutex
	calls := map[int64][]int64{}
	f.evaluator.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
		mu.Lock()
		defer mu.Unlock()
		calls[r.TurnID] = append(calls[r.TurnID], r.EvaluatorVersionID)
		if r.TurnID == 20 {
			assert.Equal(t, int64(402), r.EvaluatorVersionID)
		}
		return resumeEvaluator(2000+r.TurnID+r.EvaluatorVersionID, r.EvaluatorVersionID, r.TurnID, ""), nil
	}).Times(3)
	f.turns.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, rows []*entity.ExptTurnResultRunLog) error {
		if rows[0].TurnID == 20 {
			assert.Equal(t, int64(301), rows[0].EvaluatorResultIds.Registered[0].RecordID)
		}
		return nil
	}).Times(2)
	abort, err := f.executor.EvalTurns(ctxcache.Init(context.Background()), f.item)
	require.NoError(t, err)
	assert.False(t, abort)
	assert.Equal(t, []int64{402}, calls[20])
	assert.Len(t, calls[10], 2)
}

func TestRunningRetryTurnResume_ManualSelectionRunsEveryPreparedTurn(t *testing.T) {
	f := newTurnResumeFixture(t, entity.EvaluationModeRetryItems, 0, 20)
	// Admission prepared all turns by clearing refs and success state.
	for _, log := range f.item.ExistItemEvalResult.TurnResultRunLogs {
		log.Status = entity.TurnRunState_Processing
	}
	f.benefit.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil).Times(4)
	var targetCalls []int64
	f.target.EXPECT().ExecuteTarget(gomock.Any(), int64(3), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, _ int64, _ int64, r *entity.ExecuteTargetCtx, _ *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error) {
		targetCalls = append(targetCalls, r.TurnID)
		return resumeTarget(201+r.TurnID, r.TurnID), nil
	}).Times(2)
	f.evaluator.EXPECT().ShouldInterceptEvaluator(gomock.Any(), gomock.Any()).Return(nil, false, nil).Times(4)
	var mu sync.Mutex
	calls := map[int64]int{}
	f.evaluator.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
		mu.Lock()
		defer mu.Unlock()
		calls[r.TurnID]++
		return resumeEvaluator(1000+r.TurnID+r.EvaluatorVersionID, r.EvaluatorVersionID, r.TurnID, ""), nil
	}).Times(4)
	f.turns.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).Return(nil).Times(2)
	abort, err := f.executor.EvalTurns(ctxcache.Init(context.Background()), f.item)
	require.NoError(t, err)
	assert.False(t, abort)
	assert.Equal(t, []int64{0, 20}, targetCalls)
	assert.Equal(t, map[int64]int{0: 2, 20: 2}, calls)
}

func TestRunningRetryTurnResume_CallbackRefsReadIncompleteDoesNotStartEvaluators(t *testing.T) {
	f := newTurnResumeFixture(t, entity.EvaluationModeFailRetry, 0)
	f.item.Event.AsyncReportTrigger = true
	f.item.Event.ResumeSource = &entity.ExptResumeSource{Kind: "target", TurnID: 0, RecordID: 201, InvokeID: "201"}
	log := f.item.ExistItemEvalResult.TurnResultRunLogs[0]
	log.TargetResultID = 201
	log.EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 401, RecordID: 301}}}
	f.target.EXPECT().GetRecordByID(gomock.Any(), int64(3), int64(201)).Return(resumeTarget(201, 0), nil)
	f.records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{301}, false, false).Return(nil, nil)
	_, err := f.executor.EvalTurns(ctxcache.Init(context.Background()), f.item)
	require.Error(t, err)
}

func TestRunningRetryTurnResume_SourceWithEmptyRefsStartsEvaluatorsOnce(t *testing.T) {
	f := newTurnResumeFixture(t, entity.EvaluationModeRetryItems, 0)
	f.item.Event.AsyncReportTrigger = true
	f.item.Event.ResumeSource = &entity.ExptResumeSource{Kind: "target", TurnID: 0, RecordID: 201, InvokeID: "201"}
	f.item.ExistItemEvalResult.TurnResultRunLogs[0].TargetResultID = 201
	f.target.EXPECT().GetRecordByID(gomock.Any(), int64(3), int64(201)).Return(resumeTarget(201, 0), nil)
	f.benefit.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil).Times(1)
	f.evaluator.EXPECT().ShouldInterceptEvaluator(gomock.Any(), gomock.Any()).Return(nil, false, nil).Times(2)
	f.evaluator.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
		return resumeEvaluator(1000+r.EvaluatorVersionID, r.EvaluatorVersionID, 0, ""), nil
	}).Times(2)
	f.turns.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	abort, err := f.executor.EvalTurns(ctxcache.Init(context.Background()), f.item)
	require.NoError(t, err)
	assert.False(t, abort)
}

func TestRunningRetryTurnResume_AsyncEvaluatorExtAndIdentity(t *testing.T) {
	for _, alias := range []string{"", "judge"} {
		t.Run("alias="+alias, func(t *testing.T) {
			f := newTurnResumeFixture(t, entity.EvaluationModeFailRetry, 0)
			f.item.Event.Ext = map[string]string{entity.ManualRetryExtKey: "internal", "lane": "ppe_frozen"}
			ev := f.item.Expt.Evaluators[0]
			ev.EvaluatorType = entity.EvaluatorTypeAgent
			ev.AgentEvaluatorVersion = &entity.AgentEvaluatorVersion{ID: 401, EvaluatorID: 401}
			f.item.Expt.Evaluators = f.item.Expt.Evaluators[:1]
			if alias != "" {
				f.item.Expt.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
				f.item.ItemConfig = &entity.ExptItemConfig{EvaluatorConfs: []*entity.ItemEvaluatorConf{{EvaluatorVersionID: 401, Alias: alias}}}
			}
			f.benefit.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil)
			f.evaluator.EXPECT().ShouldInterceptEvaluator(gomock.Any(), gomock.Any()).Return(nil, false, nil)
			f.evaluator.EXPECT().AsyncRunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r *entity.AsyncRunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
				assert.NotContains(t, r.Ext, entity.ManualRetryExtKey)
				assert.NotContains(t, r.InputData.Ext, entity.ManualRetryExtKey)
				assert.Equal(t, "ppe_frozen", r.Ext["lane"])
				require.NotNil(t, r.AsyncCtx.Event)
				assert.Equal(t, "RQ1", r.AsyncCtx.Event.RequestID)
				assert.Equal(t, "internal", r.AsyncCtx.Event.Ext[entity.ManualRetryExtKey])
				assert.Equal(t, alias, r.Alias)
				return &entity.EvaluatorRecord{ID: 301, EvaluatorVersionID: 401, Alias: alias, Status: entity.EvaluatorRunStatusAsyncInvoking}, nil
			})
			f.records.EXPECT().GetEvaluatorRecord(gomock.Any(), int64(301), false).Return(&entity.EvaluatorRecord{ID: 301, EvaluatorVersionID: 401, Alias: alias, Status: entity.EvaluatorRunStatusAsyncInvoking}, nil)
			ctx := ctxcache.Init(context.Background())
			etec, err := f.executor.buildExptTurnEvalCtx(ctx, f.item.EvalSetItem.Turns[0], f.item, nil)
			require.NoError(t, err)
			results, err := (&DefaultExptTurnEvaluationImpl{metric: f.executor.Metric, evaluatorService: f.evaluator, evaluatorRecordService: f.records, benefitService: f.benefit}).CallEvaluators(ctx, etec, resumeTarget(201, 0))
			require.NoError(t, err)
			require.Len(t, results, 1)
			assert.Equal(t, int64(301), results[0].ID)
		})
	}
}

func TestRunningRetryTurnResume_DirectCallerIgnoresOtherTurnCallback(t *testing.T) {
	f := newTurnResumeFixture(t, entity.EvaluationModeRetryItems, 0)
	f.item.Event.AsyncEvaluatorReportTrigger = true
	f.item.Event.ResumeSource = &entity.ExptResumeSource{Kind: "evaluator", TurnID: 20, RecordID: 301, InvokeID: "301"}
	// A callback from turn 20 is still a continuation of RQ1 on turn zero.
	// Only its callback flags are ignored here; successful current refs survive.
	etec := &entity.ExptTurnEvalCtx{ExptItemEvalCtx: f.item, Turn: f.item.EvalSetItem.Turns[0], Ext: map[string]string{}, ExptTurnRunResult: &entity.ExptTurnRunResult{TargetResult: resumeTarget(201, 0), EvaluatorResults: []*entity.EvaluatorRecord{resumeEvaluator(301, 401, 0, ""), resumeEvaluator(302, 402, 0, "")}}}
	ctx := ctxcache.Init(context.Background())
	svc := &DefaultExptTurnEvaluationImpl{metric: f.executor.Metric, evalTargetService: f.target, evaluatorService: f.evaluator, benefitService: f.benefit}
	target, err := svc.CallTarget(ctx, etec)
	require.NoError(t, err)
	assert.Equal(t, int64(201), target.ID)
	scoped := etec.Event
	results, err := svc.CallEvaluators(ctx, etec, target)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, int64(301), results[0].ID)
	assert.Equal(t, int64(302), results[1].ID)
	assert.Same(t, scoped, etec.Event, "both stage calls share the scoped event")
	assert.False(t, etec.Event.AsyncEvaluatorReportTrigger)
	assert.Equal(t, int64(20), etec.Event.ResumeSource.TurnID)
	assert.True(t, f.item.Event.AsyncEvaluatorReportTrigger)
	assert.Equal(t, int64(20), f.item.Event.ResumeSource.TurnID)
}

func TestRunningRetryTurnResume_EventCopiesConcurrentCache(t *testing.T) {
	f := newTurnResumeFixture(t, entity.EvaluationModeFailRetry, 0)
	ctx := ctxcache.Init(context.Background())
	original := f.item.Event
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			called := original.Clone()
			other := original.Clone()
			called.WithCtxTargetCalled(ctx)
			assert.True(t, called.IgnoreExistedEvaluatorResult(ctx))
			assert.False(t, other.IgnoreExistedEvaluatorResult(ctx))
			called.Ext = map[string]string{"own": "value"}
		}()
	}
	wg.Wait()
	assert.False(t, original.CtxTargetCalled(ctx))
	assert.Nil(t, original.Ext)
}

// Storage boundaries return the same preview record that MySQL returns; the
// item/turn execution and history construction remain real.
func TestRunningRetryHistoryOutput_LoadsFullSuccessOutputBeforeNextTarget(t *testing.T) {
	for _, source := range []string{"same space", "frozen target space", "frozen item target space"} {
		t.Run(source, func(t *testing.T) {
			f := newTurnResumeFixture(t, entity.EvaluationModeFailRetry, 0, 20)
			spaceID := int64(3)
			switch source {
			case "frozen target space":
				f.item.Expt.TargetSpaceID = 9
				spaceID = 9
			case "frozen item target space":
				f.item.Expt.TargetSpaceID = 8
				f.item.ItemConfig = &entity.ExptItemConfig{TargetSourceSpaceID: 9}
				spaceID = 9
			}
			first := f.item.ExistItemEvalResult.TurnResultRunLogs[0]
			first.Status = entity.TurnRunState_Success
			first.TargetResultID = 201
			first.EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 401, RecordID: 301}, {VersionID: 402, RecordID: 302}}}
			full := strings.Repeat("complete previous answer ", 10000)
			target := resumeTarget(201, 0)
			target.SpaceID = spaceID
			target.EvalTargetOutputData.OutputFields[consts.OutputSchemaKey] = &entity.Content{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("preview..."), ContentOmitted: gptr.Of(true), FullContent: &entity.ObjectStorage{URI: gptr.Of("eval_record/history-output")}, FullContentBytes: gptr.Of(int32(len(full)))}
			preview := target.EvalTargetOutputData.OutputFields[consts.OutputSchemaKey]
			require.True(t, preview.IsContentOmitted())
			require.Greater(t, len(full), 204800)
			e1 := resumeEvaluator(301, 401, 0, "")
			e1.EvaluatorOutputData = &entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: gptr.Of(float64(0))}}
			e2 := resumeEvaluator(302, 402, 0, "") // Successful without a score stays reusable.
			f.target.EXPECT().GetRecordByID(gomock.Any(), spaceID, int64(201)).Return(target, nil)
			f.records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{301, 302}, false, false).Return([]*entity.EvaluatorRecord{e1, e2}, nil)
			var loaded bool
			f.target.EXPECT().LoadRecordOutputFields(gomock.Any(), gomock.Any(), []string{consts.OutputSchemaKey}).DoAndReturn(func(_ context.Context, got *entity.EvalTargetRecord, _ []string) error {
				require.Same(t, target, got)
				assert.Equal(t, spaceID, got.SpaceID)
				require.Equal(t, "eval_record/history-output", gptr.Indirect(got.EvalTargetOutputData.OutputFields[consts.OutputSchemaKey].FullContent.URI))
				got.EvalTargetOutputData.OutputFields[consts.OutputSchemaKey].Text = gptr.Of(full)
				loaded = true
				return nil
			}).Times(1)
			f.benefit.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ *benefit.CheckAndDeductEvalBenefitParams) (*benefit.CheckAndDeductEvalBenefitResult, error) {
				assert.True(t, loaded, "success history loads before next turn spends benefit")
				return &benefit.CheckAndDeductEvalBenefitResult{}, nil
			}).Times(2)
			f.target.EXPECT().ExecuteTarget(gomock.Any(), spaceID, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, _ int64, _ int64, exec *entity.ExecuteTargetCtx, input *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error) {
				assert.True(t, loaded)
				assert.Equal(t, int64(20), exec.TurnID)
				require.Len(t, input.HistoryMessages, 1)
				assert.Equal(t, entity.RoleAssistant, input.HistoryMessages[0].Role)
				assert.Equal(t, full, input.HistoryMessages[0].Content.GetText())
				assert.False(t, input.HistoryMessages[0].Content.IsContentOmitted())
				return resumeTarget(202, 20), nil
			}).Times(1)
			f.evaluator.EXPECT().ShouldInterceptEvaluator(gomock.Any(), gomock.Any()).Return(nil, false, nil).Times(2)
			f.evaluator.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
				assert.Equal(t, int64(20), r.TurnID)
				return resumeEvaluator(1000+r.EvaluatorVersionID, r.EvaluatorVersionID, 20, ""), nil
			}).Times(2)
			f.turns.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, rows []*entity.ExptTurnResultRunLog) error {
				require.Len(t, rows, 1)
				assert.Equal(t, int64(20), rows[0].TurnID)
				return nil
			}).Times(1)
			abort, err := f.executor.EvalTurns(ctxcache.Init(context.Background()), f.item)
			require.NoError(t, err)
			assert.False(t, abort)
			assert.Equal(t, int64(1), f.evalCalls.Load(), "reused success turn never enters Eval")
			assert.Equal(t, int64(201), first.TargetResultID)
			assert.Equal(t, entity.TurnRunState_Success, first.Status)
			assert.Equal(t, []*entity.RegisteredEvalResult{{VersionID: 401, RecordID: 301}, {VersionID: 402, RecordID: 302}}, first.EvaluatorResultIds.Registered)
		})
	}
}

func TestRunningRetryHistoryOutput_IncompleteSuccessStopsBeforeNextTurn(t *testing.T) {
	for _, failure := range []string{"missing target record", "storage read error", "still omitted", "field removed", "nil field", "nil output data"} {
		t.Run(failure, func(t *testing.T) {
			f := newTurnResumeFixture(t, entity.EvaluationModeFailRetry, 0, 20)
			first := f.item.ExistItemEvalResult.TurnResultRunLogs[0]
			first.Status = entity.TurnRunState_Success
			first.TargetResultID = 201
			target := resumeTarget(201, 0)
			cause := errors.New("object storage read failed")
			if failure == "missing target record" {
				target = nil
			} else {
				target.EvalTargetOutputData.OutputFields[consts.OutputSchemaKey] = &entity.Content{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("preview..."), ContentOmitted: gptr.Of(true), FullContent: &entity.ObjectStorage{URI: gptr.Of("eval_record/history-output")}, FullContentBytes: gptr.Of(int32(240000))}
				f.target.EXPECT().LoadRecordOutputFields(gomock.Any(), gomock.Any(), []string{consts.OutputSchemaKey}).DoAndReturn(func(_ context.Context, got *entity.EvalTargetRecord, _ []string) error {
					require.Same(t, target, got)
					switch failure {
					case "storage read error":
						return cause
					case "field removed":
						delete(got.EvalTargetOutputData.OutputFields, consts.OutputSchemaKey)
					case "nil field":
						got.EvalTargetOutputData.OutputFields[consts.OutputSchemaKey] = nil
					case "nil output data":
						got.EvalTargetOutputData = nil
					}
					return nil
				}).Times(1)
			}
			f.target.EXPECT().GetRecordByID(gomock.Any(), int64(3), int64(201)).Return(target, nil)
			// No expected Eval/store/target/evaluator/benefit calls: a missing or
			// incomplete old success must stop, rather than refill it with a new invoke.
			abort, err := f.executor.EvalTurns(ctxcache.Init(context.Background()), f.item)
			require.Error(t, err)
			assert.False(t, abort)
			if failure == "storage read error" {
				assert.ErrorIs(t, err, cause)
			}
			assert.Zero(t, f.evalCalls.Load())
			assert.Equal(t, int64(201), first.TargetResultID)
			assert.Equal(t, entity.TurnRunState_Success, first.Status)
		})
	}
}

func TestRunningRetryHistoryOutput_ShortNontextAndTargetFreeRemainCompatible(t *testing.T) {
	for _, shape := range []string{"short text", "nontext", "target-free zero record"} {
		t.Run(shape, func(t *testing.T) {
			f := newTurnResumeFixture(t, entity.EvaluationModeFailRetry, 0)
			first := f.item.ExistItemEvalResult.TurnResultRunLogs[0]
			first.Status = entity.TurnRunState_Success
			switch shape {
			case "target-free zero record":
				f.item.Expt.TargetVersionID = 0
			default:
				first.TargetResultID = 201
				target := resumeTarget(201, 0)
				if shape == "nontext" {
					target.EvalTargetOutputData.OutputFields[consts.OutputSchemaKey] = &entity.Content{ContentType: gptr.Of(entity.ContentTypeImage), Image: &entity.Image{URL: gptr.Of("image-ref")}, ContentOmitted: gptr.Of(true)}
				}
				f.target.EXPECT().GetRecordByID(gomock.Any(), int64(3), int64(201)).Return(target, nil)
			}
			abort, err := f.executor.EvalTurns(ctxcache.Init(context.Background()), f.item)
			require.NoError(t, err)
			assert.False(t, abort)
			assert.Zero(t, f.evalCalls.Load())
			assert.Equal(t, entity.TurnRunState_Success, first.Status)
		})
	}
}

func TestRunningRetryHistoryOutput_FreshOmittedTurnLoadsBeforeNextTarget(t *testing.T) {
	f := newTurnResumeFixture(t, entity.EvaluationModeSubmit, 0, 20)
	// ExecuteTarget persists before returning; storage mutates the returned
	// Content to a preview. With no evaluators, nothing else hydrates it.
	f.item.Expt.EvalConf.ConnectorConf.EvaluatorsConf = nil
	full := strings.Repeat("complete fresh answer ", 11000)
	first := resumeTarget(201, 0)
	first.EvalTargetOutputData.OutputFields[consts.OutputSchemaKey] = &entity.Content{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("preview..."), ContentOmitted: gptr.Of(true), FullContent: &entity.ObjectStorage{URI: gptr.Of("eval_record/fresh-history")}, FullContentBytes: gptr.Of(int32(len(full)))}
	require.True(t, first.EvalTargetOutputData.OutputFields[consts.OutputSchemaKey].IsContentOmitted())
	f.benefit.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil).Times(2)
	f.target.EXPECT().ExecuteTarget(gomock.Any(), int64(3), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, _ int64, _ int64, exec *entity.ExecuteTargetCtx, input *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error) {
		if exec.TurnID == 0 {
			assert.Empty(t, input.HistoryMessages)
			return first, nil
		}
		require.Len(t, input.HistoryMessages, 1)
		assert.Equal(t, full, input.HistoryMessages[0].Content.GetText())
		assert.False(t, input.HistoryMessages[0].Content.IsContentOmitted())
		return resumeTarget(202, 20), nil
	}).Times(2)
	f.target.EXPECT().LoadRecordOutputFields(gomock.Any(), gomock.Any(), []string{consts.OutputSchemaKey}).DoAndReturn(func(_ context.Context, got *entity.EvalTargetRecord, _ []string) error {
		require.Same(t, first, got)
		got.EvalTargetOutputData.OutputFields[consts.OutputSchemaKey].Text = gptr.Of(full)
		return nil
	}).Times(1)
	f.turns.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).Return(nil).Times(2)
	abort, err := f.executor.EvalTurns(ctxcache.Init(context.Background()), f.item)
	require.NoError(t, err)
	assert.False(t, abort)
	assert.Equal(t, int64(2), f.evalCalls.Load())
}

func TestRunningRetryHistoryOutput_FreshFinalTurnDoesNotLoadUnusedHistory(t *testing.T) {
	f := newTurnResumeFixture(t, entity.EvaluationModeSubmit, 0)
	f.item.Expt.EvalConf.ConnectorConf.EvaluatorsConf = nil
	target := resumeTarget(201, 0)
	target.EvalTargetOutputData.OutputFields[consts.OutputSchemaKey] = &entity.Content{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("preview..."), ContentOmitted: gptr.Of(true), FullContent: &entity.ObjectStorage{URI: gptr.Of("eval_record/final-output")}, FullContentBytes: gptr.Of(int32(240000))}
	f.benefit.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil)
	f.target.EXPECT().ExecuteTarget(gomock.Any(), int64(3), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(target, nil)
	f.turns.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).Return(nil)
	// No subsequent turn consumes history, so successful single-turn execution
	// must not add an object-storage dependency after persisting its result.
	abort, err := f.executor.EvalTurns(ctxcache.Init(context.Background()), f.item)
	require.NoError(t, err)
	assert.False(t, abort)
	assert.Equal(t, int64(1), f.evalCalls.Load())
}

func TestRunningRetryHistoryIsolation_ThreeTurnsSurviveActualRecordStorage(t *testing.T) {
	for _, reused := range []bool{true, false} {
		t.Run(map[bool]string{true: "reused success", false: "fresh output"}[reused], func(t *testing.T) {
			f := newTurnResumeFixture(t, entity.EvaluationModeFailRetry, 0, 20, 30)
			f.item.Expt.TargetSpaceID = 9
			f.item.Expt.EvalConf.ConnectorConf.EvaluatorsConf = nil
			f.config.EXPECT().GetEvaluationRecordStorage(gomock.Any()).Return(&component.EvaluationRecordStorage{Providers: []*component.EvaluationRecordProviderConfig{{Provider: "RDS", MaxSize: 204800}}}).AnyTimes()
			full := strings.Repeat("complete first answer ", 11000)
			require.Greater(t, len(full), 204800)
			firstTarget := resumeTarget(201, 0)
			firstTarget.SpaceID = 9
			firstTarget.EvalTargetOutputData.OutputFields[consts.OutputSchemaKey] = &entity.Content{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of(full)}
			blobs := map[string][]byte{"initial-full-output": []byte(full)}
			blob := fsmocks.NewMockBatchObjectStorage(gomock.NewController(t))
			uploadCount := 3
			executeCount := 3
			if reused {
				uploadCount = 2
				executeCount = 2
			}
			blob.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, key string, body io.Reader, _ ...fileserver.UploadOpt) error {
				data, err := io.ReadAll(body)
				require.NoError(t, err)
				blobs[key] = data
				return nil
			}).Times(uploadCount)
			blob.EXPECT().Read(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, key string, _ ...fileserver.DownloadOpt) (fileserver.Reader, error) {
				data, exists := blobs[key]
				require.True(t, exists)
				return &historyIsolationReader{Reader: bytes.NewReader(data)}, nil
			}).Times(1)
			storage := evalstorage.NewRecordDataStorage(blob, f.config)
			firstLog := f.item.ExistItemEvalResult.TurnResultRunLogs[0]
			if reused {
				firstLog.Status = entity.TurnRunState_Success
				firstLog.TargetResultID = 201
				firstLog.EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 401, RecordID: 301}, {VersionID: 402, RecordID: 302}}}
				output := firstTarget.EvalTargetOutputData.OutputFields[consts.OutputSchemaKey]
				output.Text = gptr.Of("preview...")
				output.ContentOmitted = gptr.Of(true)
				output.FullContent = &entity.ObjectStorage{URI: gptr.Of("initial-full-output")}
				output.FullContentBytes = gptr.Of(int32(len(full)))
				f.target.EXPECT().GetRecordByID(gomock.Any(), int64(9), int64(201)).Return(firstTarget, nil)
				e1 := resumeEvaluator(301, 401, 0, "")
				e1.EvaluatorOutputData = &entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: gptr.Of(float64(0))}}
				f.records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{301, 302}, false, false).Return([]*entity.EvaluatorRecord{e1, resumeEvaluator(302, 402, 0, "")}, nil)
			}
			f.target.EXPECT().LoadRecordOutputFields(gomock.Any(), gomock.Any(), []string{consts.OutputSchemaKey}).DoAndReturn(func(ctx context.Context, record *entity.EvalTargetRecord, keys []string) error {
				require.Same(t, firstTarget, record)
				return storage.LoadEvalTargetOutputFields(ctx, record, keys)
			}).Times(1)
			f.benefit.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil).Times(executeCount)
			var called []int64
			f.target.EXPECT().ExecuteTarget(gomock.Any(), int64(9), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ int64, _ int64, _ int64, exec *entity.ExecuteTargetCtx, input *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error) {
				called = append(called, exec.TurnID)
				var record *entity.EvalTargetRecord
				if exec.TurnID == 0 {
					assert.Empty(t, input.HistoryMessages)
					record = firstTarget
				} else {
					if exec.TurnID == 20 {
						require.Len(t, input.HistoryMessages, 1)
					} else {
						require.Len(t, input.HistoryMessages, 2)
						assert.Equal(t, "previous answer", input.HistoryMessages[1].Content.GetText())
					}
					original := firstTarget.EvalTargetOutputData.OutputFields[consts.OutputSchemaKey]
					assert.NotSame(t, original, input.HistoryMessages[0].Content, "invocation storage must receive independent Content")
					assert.True(t, full == input.HistoryMessages[0].Content.GetText(), "later target must see complete first output bytes")
					assert.False(t, input.HistoryMessages[0].Content.IsContentOmitted())
					record = resumeTarget(201+exec.TurnID, exec.TurnID)
					record.SpaceID = 9
				}
				record.EvalTargetInputData = input
				// Actual storage traverses and clips invocation input history before the
				// provider returns its record, matching ExecuteTarget's deferred save.
				require.NoError(t, storage.SaveEvalTargetRecordData(ctx, record, nil))
				if exec.TurnID != 0 {
					assert.True(t, input.HistoryMessages[0].Content.IsContentOmitted(), "actual invocation storage clipped its copy")
					assert.True(t, full == firstTarget.EvalTargetOutputData.OutputFields[consts.OutputSchemaKey].GetText(), "shared first output remains full after provider save")
					assert.False(t, firstTarget.EvalTargetOutputData.OutputFields[consts.OutputSchemaKey].IsContentOmitted())
				}
				return record, nil
			}).Times(executeCount)
			f.turns.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, rows []*entity.ExptTurnResultRunLog) error {
				require.Len(t, rows, 1)
				if reused {
					assert.NotEqual(t, int64(0), rows[0].TurnID)
				}
				return nil
			}).Times(executeCount)
			abort, err := f.executor.EvalTurns(ctxcache.Init(context.Background()), f.item)
			require.NoError(t, err)
			assert.False(t, abort)
			assert.Equal(t, int64(executeCount), f.evalCalls.Load())
			if reused {
				assert.Equal(t, []int64{20, 30}, called)
				assert.Equal(t, int64(201), firstLog.TargetResultID)
				assert.Equal(t, entity.TurnRunState_Success, firstLog.Status)
				assert.Equal(t, []*entity.RegisteredEvalResult{{VersionID: 401, RecordID: 301}, {VersionID: 402, RecordID: 302}}, firstLog.EvaluatorResultIds.Registered)
			} else {
				assert.Equal(t, []int64{0, 20, 30}, called)
			}
			assert.True(t, full == firstTarget.EvalTargetOutputData.OutputFields[consts.OutputSchemaKey].GetText())
		})
	}
}

func TestRunningRetryHistoryIsolation_TargetInputCopiesNestedContent(t *testing.T) {
	f := newTurnResumeFixture(t, entity.EvaluationModeFailRetry, 20)
	content := &entity.Content{ContentType: gptr.Of(entity.ContentTypeMultipart), Format: gptr.Of(entity.FieldDisplayFormat_JSON), Text: gptr.Of("root"), Image: &entity.Image{Name: gptr.Of("image"), URI: gptr.Of("image-uri"), URL: gptr.Of("image-url"), ThumbURL: gptr.Of("image-thumb"), StorageProvider: entity.StorageProviderPtr(entity.StorageProvider_S3)}, Audio: &entity.Audio{Format: gptr.Of("wav"), URI: gptr.Of("audio-uri"), Name: gptr.Of("audio"), URL: gptr.Of("audio-url"), StorageProvider: entity.StorageProviderPtr(entity.StorageProvider_S3)}, Video: &entity.Video{Name: gptr.Of("video"), URI: gptr.Of("video-uri"), URL: gptr.Of("video-url"), ThumbURL: gptr.Of("video-thumb"), StorageProvider: entity.StorageProviderPtr(entity.StorageProvider_S3)}, FullContent: &entity.ObjectStorage{Name: gptr.Of("full"), URI: gptr.Of("full-uri"), URL: gptr.Of("full-url"), ThumbURL: gptr.Of("full-thumb"), Provider: entity.StorageProviderPtr(entity.StorageProvider_S3)}, ContentOmitted: gptr.Of(true), FullContentBytes: gptr.Of(int32(240000)), MultiPart: []*entity.Content{{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("nested"), MultiPart: []*entity.Content{{Text: gptr.Of("deep")}}}}}
	history := []*entity.Message{{Role: entity.RoleAssistant, Content: content, Ext: map[string]string{"metadata": "frozen"}}, nil, {Role: entity.RoleTool}}
	etec := &entity.ExptTurnEvalCtx{ExptItemEvalCtx: f.item, Turn: f.item.EvalSetItem.Turns[0], History: history, Ext: map[string]string{}, ExptTurnRunResult: &entity.ExptTurnRunResult{}}
	f.benefit.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil)
	f.target.EXPECT().ExecuteTarget(gomock.Any(), int64(3), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, _ int64, _ int64, _ *entity.ExecuteTargetCtx, input *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error) {
		assert.Equal(t, history, input.HistoryMessages, "all nested input fields and nil messages are preserved")
		c := input.HistoryMessages[0].Content
		assert.NotSame(t, content, c)
		assert.NotSame(t, content.MultiPart[0], c.MultiPart[0])
		assert.NotSame(t, content.MultiPart[0].MultiPart[0], c.MultiPart[0].MultiPart[0])
		assert.NotSame(t, content.Image, c.Image)
		assert.NotSame(t, content.Audio, c.Audio)
		assert.NotSame(t, content.Video, c.Video)
		assert.NotSame(t, content.FullContent, c.FullContent)
		*c.Text = "changed"
		*c.Format = entity.FieldDisplayFormat_Markdown
		*c.ContentType = entity.ContentTypeText
		*c.ContentOmitted = false
		*c.FullContentBytes = 1
		*c.MultiPart[0].Text = "nested changed"
		*c.MultiPart[0].MultiPart[0].Text = "deep changed"
		*c.Image.URI = "image changed"
		*c.Image.StorageProvider = entity.StorageProvider(99)
		*c.Audio.URI = "audio changed"
		*c.Audio.Format = "mp3"
		*c.Video.URI = "video changed"
		*c.Video.ThumbURL = "thumb changed"
		*c.FullContent.URI = "full changed"
		*c.FullContent.Provider = entity.StorageProvider(99)
		input.HistoryMessages[0].Ext["metadata"] = "changed"
		input.HistoryMessages[0].Role = entity.RoleUser
		return resumeTarget(201, 20), nil
	})
	svc := &DefaultExptTurnEvaluationImpl{metric: f.executor.Metric, evalTargetService: f.target, benefitService: f.benefit}
	_, err := svc.CallTarget(ctxcache.Init(context.Background()), etec)
	require.NoError(t, err)
	assert.Equal(t, "root", content.GetText())
	assert.Equal(t, entity.ContentTypeMultipart, *content.ContentType)
	assert.Equal(t, entity.FieldDisplayFormat_JSON, *content.Format)
	assert.True(t, *content.ContentOmitted)
	assert.Equal(t, int32(240000), *content.FullContentBytes)
	assert.Equal(t, "nested", content.MultiPart[0].GetText())
	assert.Equal(t, "deep", content.MultiPart[0].MultiPart[0].GetText())
	assert.Equal(t, "image-uri", *content.Image.URI)
	assert.Equal(t, entity.StorageProvider_S3, *content.Image.StorageProvider)
	assert.Equal(t, "audio-uri", *content.Audio.URI)
	assert.Equal(t, "wav", *content.Audio.Format)
	assert.Equal(t, "video-uri", *content.Video.URI)
	assert.Equal(t, "video-thumb", *content.Video.ThumbURL)
	assert.Equal(t, "full-uri", *content.FullContent.URI)
	assert.Equal(t, entity.StorageProvider_S3, *content.FullContent.Provider)
	assert.Equal(t, "frozen", history[0].Ext["metadata"])
	assert.Equal(t, entity.RoleAssistant, history[0].Role)
}

func TestRunningRetryHistoryIsolation_LastSuccessDoesNotLoadUnusedHistory(t *testing.T) {
	for _, hasPrior := range []bool{false, true} {
		t.Run(map[bool]string{false: "single turn zero", true: "last of two successes"}[hasPrior], func(t *testing.T) {
			ids := []int64{0}
			if hasPrior {
				ids = []int64{0, 20}
			}
			f := newTurnResumeFixture(t, entity.EvaluationModeFailRetry, ids...)
			for _, id := range ids {
				log := f.item.ExistItemEvalResult.TurnResultRunLogs[id]
				log.Status = entity.TurnRunState_Success
				log.TargetResultID = 201 + id
				target := resumeTarget(201+id, id)
				if id == ids[len(ids)-1] {
					target.EvalTargetOutputData.OutputFields[consts.OutputSchemaKey] = &entity.Content{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("preview..."), ContentOmitted: gptr.Of(true), FullContent: &entity.ObjectStorage{URI: gptr.Of("unused-final-history")}, FullContentBytes: gptr.Of(int32(240000))}
				}
				f.target.EXPECT().GetRecordByID(gomock.Any(), int64(3), 201+id).Return(target, nil)
			}
			// No load is expected; lack of a history consumer must not create an
			// object-storage dependency, while nonzero target record reads still run.
			abort, err := f.executor.EvalTurns(ctxcache.Init(context.Background()), f.item)
			require.NoError(t, err)
			assert.False(t, abort)
			assert.Zero(t, f.evalCalls.Load())
			for _, id := range ids {
				assert.Equal(t, entity.TurnRunState_Success, f.item.ExistItemEvalResult.TurnResultRunLogs[id].Status)
			}
		})
	}
}

// A test-only object-storage reader retains ReadAt required by the real boundary.
type historyIsolationReader struct{ *bytes.Reader }

func (r *historyIsolationReader) Close() error { return nil }
