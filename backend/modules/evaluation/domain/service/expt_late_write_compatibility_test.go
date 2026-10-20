// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bytedance/gg/gptr"
	experimentrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	experimentmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/pkg/ctxcache"
	"gorm.io/gorm"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	idmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	metricmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	evaluatorrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator"
	evaluatormysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	targetmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql"
	targetconvert "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

// Keep the real producer's record fields and the real SQL DAO, mocking only unrelated target configuration.
type compatibilityTargetRepo struct {
	repo.IEvalTargetRepo
	dao targetmysql.EvalTargetRecordDAO
}

func (r *compatibilityTargetRepo) CreateEvalTargetRecord(ctx context.Context, record *entity.EvalTargetRecord, _ *bool) (int64, error) {
	po, err := targetconvert.EvalTargetRecordDO2PO(record)
	if err != nil {
		return 0, err
	}
	return r.dao.Create(ctx, po)
}
func (r *compatibilityTargetRepo) GetEvalTargetRecordByIDAndSpaceID(ctx context.Context, space, id int64) (*entity.EvalTargetRecord, error) {
	po, err := r.dao.GetByIDAndSpaceID(ctx, id, space)
	if err != nil {
		return nil, err
	}
	return targetconvert.EvalTargetRecordPO2DO(po)
}

func TestLateCompatProductionRecordVersionZero(t *testing.T) {
	f := newLateWriteFixture(t, "rq1")
	ctrl := gomock.NewController(t)
	ctx := context.Background()
	// Use the real async producer; its record has no ItemVersionID assignment.
	dao := targetmysql.NewEvalTargetRecordDAO(f.provider)
	operator := svcmocks.NewMockISourceEvalTargetOperateService(ctrl)
	metric := metricmocks.NewMockEvalTargetMetrics(ctrl)
	config := componentmocks.NewMockIConfiger(ctrl)
	operator.EXPECT().ValidateInput(gomock.Any(), f.space, gomock.Any(), gomock.Any()).Return(nil)
	operator.EXPECT().AsyncExecute(gomock.Any(), f.space, gomock.Any()).Return(f.base+901, "callee", map[string]string{}, nil)
	metric.EXPECT().EmitRun(f.space, gomock.Any(), gomock.Any())
	config.EXPECT().BuildEvalExt(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	baseRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
	baseRepo.EXPECT().GetEvalTargetVersion(gomock.Any(), f.space, f.base+43).Return(&entity.EvalTarget{ID: f.base + 42, SpaceID: f.space, EvalTargetType: entity.EvalTargetTypeCustomRPCServer, EvalTargetVersion: &entity.EvalTargetVersion{ID: f.base + 43}}, nil).AnyTimes()
	targetSvc := &EvalTargetServiceImpl{evalTargetRepo: &compatibilityTargetRepo{IEvalTargetRepo: baseRepo, dao: dao}, typedOperators: map[entity.EvalTargetType]ISourceEvalTargetOperateService{entity.EvalTargetTypeCustomRPCServer: operator}, metric: metric, configer: config}
	target := &entity.EvalTarget{ID: f.base + 42, SpaceID: f.space, EvalTargetType: entity.EvalTargetTypeCustomRPCServer, EvalTargetVersion: &entity.EvalTargetVersion{ID: f.base + 43}}
	produced, _, err := targetSvc.asyncExecuteTarget(ctx, f.space, target, &entity.ExecuteTargetCtx{ExperimentID: &f.expt, ExperimentRunID: &f.run, ItemID: f.item, TurnID: 0}, &entity.EvalTargetInputData{})
	require.NoError(t, err)
	require.Zero(t, produced.ItemVersionID)
	stored, err := targetSvc.GetRecordByID(ctx, f.space, produced.ID)
	require.NoError(t, err)
	require.Zero(t, stored.ItemVersionID)
	require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND turn_id=0", f.space).Update("target_result_id", produced.ID).Error)
	event := f.event()
	event.AsyncReportTrigger = true
	event.ResumeSource = &entity.ExptResumeSource{Kind: "target", TurnID: 0, RecordID: produced.ID, InvokeID: strconv.FormatInt(produced.ID, 10)}
	consumer := &ExptItemEventEvalServiceImpl{exptItemResultRepo: f.itemRepo(), exptTurnResultRepo: f.turnRepo(), evaTargetService: targetSvc}
	called := false
	require.NoError(t, consumer.HandleCurrentSource(func(context.Context, *entity.ExptItemEvalEvent) error { called = true; return nil })(ctx, event))
	assert.True(t, called, "real producer V0 must resume versioned current item")
	// RunEvaluator also produces V0, via real record repository/DAO.
	records := evaluatorrepo.NewEvaluatorRecordRepo(nil, f.provider, evaluatormysql.NewEvaluatorRecordDAO(f.provider), nil)
	versions := repomocks.NewMockIEvaluatorRepo(ctrl)
	limiter := repomocks.NewMockRateLimiter(ctrl)
	execution := svcmocks.NewMockEvaluatorSourceService(ctrl)
	ids := idmocks.NewMockIIDGenerator(ctrl)
	version := &entity.Evaluator{ID: f.base + 51, SpaceID: f.space, EvaluatorType: entity.EvaluatorTypePrompt, PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{ID: f.base + 51}}
	versions.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), gomock.Any(), []int64{f.base + 51}, false, false).Return([]*entity.Evaluator{version}, nil)
	limiter.EXPECT().AllowInvoke(gomock.Any(), f.space).Return(true)
	execution.EXPECT().PreHandle(gomock.Any(), version).Return(nil)
	execution.EXPECT().Run(gomock.Any(), version, gomock.Any(), gomock.Any(), f.space, gomock.Any()).Return(&entity.EvaluatorOutputData{}, entity.EvaluatorRunStatusSuccess, "trace")
	ids.EXPECT().GenID(gomock.Any()).Return(f.base+902, nil)
	plain := repomocks.NewMockIPlainRateLimiter(ctrl)
	plain.EXPECT().AllowInvokeWithKeyLimit(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	evalSvc := &EvaluatorServiceImpl{plainRateLimiter: plain, evaluatorRepo: versions, evaluatorRecordRepo: records, idgen: ids, limiter: limiter, evaluatorSourceServices: map[entity.EvaluatorType]EvaluatorSourceService{entity.EvaluatorTypePrompt: execution}}
	record, err := evalSvc.RunEvaluator(ctx, &entity.RunEvaluatorRequest{SpaceID: f.space, ExperimentID: f.expt, ExperimentRunID: f.run, ItemID: f.item, TurnID: 0, EvaluatorVersionID: f.base + 51, Alias: "alias"})
	require.NoError(t, err)
	require.Zero(t, record.ItemVersionID)
	loaded, err := records.GetEvaluatorRecord(ctx, record.ID, false)
	require.NoError(t, err)
	require.Zero(t, loaded.ItemVersionID)
	refs, _ := json.Marshal(&entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: f.base + 51, Alias: "alias", RecordID: record.ID}}})
	require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND turn_id=0", f.space).Update("evaluator_result_ids", refs).Error)
	consumer.evaluatorRecordService = &EvaluatorRecordServiceImpl{evaluatorRecordRepo: records}
	event.AsyncReportTrigger = false
	event.AsyncEvaluatorReportTrigger = true
	event.ResumeSource = &entity.ExptResumeSource{Kind: "evaluator", TurnID: 0, RecordID: record.ID, InvokeID: strconv.FormatInt(record.ID, 10)}
	called = false
	require.NoError(t, consumer.HandleCurrentSource(func(context.Context, *entity.ExptItemEvalEvent) error { called = true; return nil })(ctx, event))
	require.True(t, called, "real evaluator producer V0 must resume versioned item")
}

func TestLateCompatRawRetryPreEvalVersion(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeRetryAll, entity.EvaluationModeRetryItems} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			f := newLateWriteFixture(t, "")
			require.NoError(t, f.conn.Unscoped().Where("space_id=?", f.space).Delete(&model.ExptTurnResultRunLog{}).Error)
			require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=?", f.space).Update("mode", int32(mode)).Error)
			ctrl := gomock.NewController(t)
			gen := idmocks.NewMockIIDGenerator(ctrl)
			gen.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{f.base + 910, f.base + 911}, nil)
			event := f.event()
			event.RequestID = ""
			event.ExptRunMode = mode
			execCtx := &entity.ExptItemEvalCtx{Event: event, EvalSetItem: &entity.EvaluationSetItem{ItemID: f.item, ItemVersionID: &f.version, Turns: []*entity.Turn{{ID: 0}, {ID: f.base + 22}}}, ExistItemEvalResult: &entity.ExptItemEvalResult{ItemResultRunLog: &entity.ExptItemResultRunLog{ItemVersionID: f.version}, TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{}}}
			require.NoError(t, (&ExptRecordEvalModeRetryIgnoreResult{exptTurnResultRepo: f.turnRepo(), idgen: gen}).PreEval(context.Background(), execCtx))
			source, err := f.port.GetCurrentItemRunSource(context.Background(), f.space, f.expt, f.run, f.item)
			require.NoError(t, err)
			require.Equal(t, f.version, source.Turns[0].ItemVersionID)
			result := *source.Turns[0]
			result.Status = entity.TurnRunState_Success
			ok, err := f.port.StoreCurrentTurnRunResult(context.Background(), source, &result)
			require.NoError(t, err)
			require.True(t, ok)
		})
	}
}

func TestLateCompatInlineZeroAndVersionFence(t *testing.T) {
	for _, kind := range []string{"inline0", "builtin0", "version0", "version_mismatch", "negative_target", "target_mismatch", "inlinekey_mismatch", "record_mismatch", "read_error"} {
		t.Run(kind, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			items := repomocks.NewMockIExptItemResultRepo(ctrl)
			turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
			records := svcmocks.NewMockEvaluatorRecordService(ctrl)
			event := &entity.ExptItemEvalEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2, EvalSetItemID: 11, ExptRunMode: entity.EvaluationModeSubmit, AsyncEvaluatorReportTrigger: true, ResumeSource: &entity.ExptResumeSource{Kind: "evaluator", TurnID: 0, RecordID: 61, InvokeID: "61"}}
			current := &entity.ExptItemCurrentRequest{SpaceID: 3, ExptID: 1, RunID: 2, ItemID: 11, ItemVersionID: 31, Mode: entity.EvaluationModeSubmit, Status: entity.ItemRunState_Processing}
			items.EXPECT().GetCurrentRequest(gomock.Any(), int64(3), int64(1), int64(2), int64(11)).Return(current, nil)
			refs := &entity.EvaluatorResults{Inline: []*entity.InlineEvalResult{{InlineKey: "inline", RecordID: 61}}}
			record := &entity.EvaluatorRecord{ID: 61, SpaceID: 3, ExperimentID: 1, ExperimentRunID: 2, ItemID: 11, ItemVersionID: 31, TurnID: 0, SourceType: entity.EvaluatorRecordSourceTypeInline, InlineKey: "inline", Status: entity.EvaluatorRunStatusSuccess}
			var readErr error
			switch kind {
			case "builtin0":
				record.SourceType = entity.EvaluatorRecordSourceTypeBuiltin
				record.EvaluatorVersionID = 51
				refs = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 51, RecordID: 61}}}
			case "version0":
				record.ItemVersionID = 0
			case "version_mismatch":
				record.ItemVersionID = 32
			case "negative_target":
				record.TargetRecordID = -1
			case "target_mismatch":
				record.TargetRecordID = 42
			case "inlinekey_mismatch":
				record.InlineKey = "old"
			case "record_mismatch":
				refs.Inline[0].RecordID = 62
			case "read_error":
				readErr = errors.New("record storage read error")
			}
			turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), int64(1), int64(2), int64(11), int64(3)).Return([]*entity.ExptTurnResultRunLog{{SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 11, ItemVersionID: 31, TurnID: 0, TargetResultID: 41, Status: entity.TurnRunState_Processing, EvaluatorResultIds: refs}}, nil)
			records.EXPECT().GetEvaluatorRecord(gomock.Any(), int64(61), false).Return(record, readErr)
			svc := &ExptItemEventEvalServiceImpl{exptItemResultRepo: items, exptTurnResultRepo: turns, evaluatorRecordService: records}
			called := false
			err := svc.HandleCurrentSource(func(context.Context, *entity.ExptItemEvalEvent) error { called = true; return nil })(context.Background(), event)
			require.Equal(t, kind == "inline0" || kind == "builtin0" || kind == "version0", called)
			if kind == "read_error" {
				require.True(t, errno.NeedMQRetry(err))
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestLateCompatFailRetryDependencyBeforePort(t *testing.T) {
	f := newLateWriteFixture(t, "")
	source := f.source()
	ctrl := gomock.NewController(t)
	resultSvc := svcmocks.NewMockExptResultService(ctrl)
	fault := errors.New("frozen successful-stage read unavailable")
	resultSvc.EXPECT().GetExptItemTurnResults(gomock.Any(), f.expt, f.item, f.space, gomock.Any()).Return(nil, fault)
	config := componentmocks.NewMockIConfiger(ctrl)
	config.EXPECT().GetErrRetryConf(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.RetryConf{}).AnyTimes()
	svc := &ExptItemEventEvalServiceImpl{exptItemResultRepo: f.itemRepo(), exptTurnResultRepo: f.turnRepo(), configer: config, itemLocker: lateWriteLocker(t, ctrl)}
	event := f.event()
	event.RequestID = ""
	event.MaxRetryTimes = 0
	before := f.snapshot()
	err := svc.HandleEventErr(func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		executionSource(ctx).Source = source
		eiec := &entity.ExptItemEvalCtx{Event: event, ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{}}}
		return (&ExptRecordEvalModeFailRetry{resultSvc: resultSvc}).PreEval(ctx, eiec)
	})(context.Background(), event)
	require.True(t, errno.NeedMQRetry(err))
	require.ErrorIs(t, err, errRetryStartDependencyFailure)
	require.ErrorIs(t, err, fault)
	require.Equal(t, before, f.snapshot())
}

func TestLateCompatDebtFinishedAndPreExec(t *testing.T) {
	for _, kind := range []string{"completed_debt", "preexec_debt", "rq2", "cancelled", "completed_late"} {
		t.Run(kind, func(t *testing.T) {
			f := newLateWriteFixture(t, "rq1")
			source := f.source()
			ctrl := gomock.NewController(t)
			config := componentmocks.NewMockIConfiger(ctrl)
			manager := svcmocks.NewMockIExptManager(ctrl)
			guard := componentmocks.NewMockICentralReservationGuard(ctrl)
			config.EXPECT().GetErrRetryConf(gomock.Any(), f.space, gomock.Any()).Return(&entity.RetryConf{RetryTimes: 3, IsInDebt: true}).AnyTimes()
			event := f.event()
			event.RetryTimes = 3
			ctx := ctxcache.Init(context.Background())
			event.WithCtxCentralQuotaHeld(ctx, "frozen-scope")
			svc := &ExptItemEventEvalServiceImpl{exptItemResultRepo: f.itemRepo(), exptTurnResultRepo: f.turnRepo(), configer: config, manager: manager, centralGuard: guard, itemLocker: lateWriteLocker(t, ctrl)}
			expected := kind == "completed_debt" || kind == "preexec_debt"
			finishes, releases := 0, 0
			manager.EXPECT().CompleteRun(gomock.Any(), f.expt, f.run, f.space, gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, int64, int64, int64, *entity.Session, ...entity.CompleteExptOptionFn) error {
				finishes++
				return nil
			}).Times(map[bool]int{true: 1, false: 0}[expected])
			manager.EXPECT().CompleteExpt(gomock.Any(), f.expt, &f.run, f.space, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(map[bool]int{true: 1, false: 0}[expected])
			guard.EXPECT().Release(gomock.Any(), "frozen-scope", f.run, f.item, gomock.Any()).DoAndReturn(func(ctx context.Context, _ string, _, _ int64, _ string) error {
				require.NoError(t, ctx.Err())
				releases++
				return nil
			}).Times(map[bool]int{true: 1, false: 0}[expected])
			if kind == "rq2" {
				f.token("rq2")
			}
			if kind == "completed_late" {
				require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Updates(map[string]any{"status": int32(entity.ItemRunState_Fail), "result_state": int32(entity.ExptItemResultStateLogged)}).Error)
			}
			before := f.snapshot()
			if kind == "cancelled" {
				child, cancel := context.WithCancel(ctx)
				cancel()
				ctx = child
			}
			err := svc.HandleEventErr(func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
				executionSource(ctx).Source = source
				debt := errors.New("space in debt")
				if kind == "completed_debt" {
					executor := &ExptItemEvalCtxExecutor{ItemResultRepo: f.itemRepo(), Configer: config}
					return executor.CompleteItemRun(ctx, &entity.ExptItemEvalCtx{Event: event}, debt)
				}
				return debt
			})(ctx, event)
			if kind == "cancelled" {
				require.True(t, errno.NeedMQRetry(err))
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, map[bool]int{true: 1, false: 0}[expected], finishes)
			require.Equal(t, map[bool]int{true: 1, false: 0}[expected], releases)
			if kind != "completed_debt" {
				require.Equal(t, before, f.snapshot())
			}
		})
	}
}

func TestLateCompatSweepMissingRawLogs(t *testing.T) {
	for _, kind := range []string{"zombie_all", "zombie_partial", "sandbox_all", "sandbox_partial", "idgen_error", "insert_error", "second_insert_error", "idgen_cancel", "version0", "manual_missing"} {
		t.Run(kind, func(t *testing.T) {
			f := newLateWriteFixture(t, "")
			ctrl := gomock.NewController(t)
			partial := kind == "zombie_partial" || kind == "sandbox_partial"
			if partial {
				require.NoError(t, f.conn.Unscoped().Where("space_id=? AND turn_id=?", f.space, f.base+22).Delete(&model.ExptTurnResultRunLog{}).Error)
			} else {
				require.NoError(t, f.conn.Unscoped().Where("space_id=?", f.space).Delete(&model.ExptTurnResultRunLog{}).Error)
			}
			require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).Update("evaluator_result_ids", []byte(`null`)).Error)
			if kind == "version0" {
				for _, table := range []string{"expt_item_result", "expt_item_result_run_log", "expt_turn_result", "expt_turn_result_run_log", "expt_item_ref"} {
					require.NoError(t, f.conn.Table(table).Where("space_id=?", f.space).Update("item_version_id", 0).Error)
				}
				f.version = 0
			}
			if kind == "manual_missing" {
				f.token("rq1")
			}
			source, sourceErr := f.port.GetCurrentItemRunSource(context.Background(), f.space, f.expt, f.run, f.item)
			if kind == "manual_missing" {
				require.Error(t, sourceErr)
				return
			}
			require.NoError(t, sourceErr)
			config := componentmocks.NewMockIConfiger(ctrl)
			config.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{ExptExecConf: &entity.ExptExecConf{ExptItemEvalConf: &entity.ExptItemEvalConf{ZombieSecond: 1}}}).AnyTimes()
			ids := idmocks.NewMockIIDGenerator(ctrl)
			fault := errors.New("prepared raw ID unavailable")
			count := 2
			if partial {
				count = 1
			}
			calls := 0
			taskCtx, cancelTask := context.WithCancel(context.Background())
			defer cancelTask()
			ids.EXPECT().GenMultiIDs(gomock.Any(), count).DoAndReturn(func(context.Context, int) ([]int64, error) {
				calls++
				if kind == "idgen_cancel" {
					cancelTask()
				}
				if kind == "idgen_error" {
					return nil, fault
				}
				out := []int64{f.base + 920, f.base + 921}
				return out[:count], nil
			}).AnyTimes()
			target := svcmocks.NewMockIEvalTargetService(ctrl)
			records := repomocks.NewMockIEvaluatorRecordRepo(ctrl)
			records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).Return(nil, nil).AnyTimes()
			target.EXPECT().GetRecordByID(gomock.Any(), f.space, f.base+41).Return(&entity.EvalTargetRecord{ID: f.base + 41, SpaceID: f.space, ExperimentRunID: f.run, ItemID: f.item, TurnID: 0, Status: gptr.Of(entity.EvalTargetRunStatusFail)}, nil).AnyTimes()
			target.EXPECT().TerminateAsyncRecordsAndDestroySandbox(gomock.Any(), f.space, []int64{f.base + 41}, gomock.Any(), gomock.Any(), gomock.Any()).Times(map[bool]int{true: 0, false: 1}[kind == "idgen_error" || kind == "idgen_cancel"])
			if kind == "sandbox_all" || kind == "sandbox_partial" {
				target.EXPECT().CheckSandboxTerminated(gomock.Any(), f.space, []int64{f.base + 41}).Return([]int64{f.base + 41}, map[int64]string{f.base + 41: "Failed"})
			}
			scheduler := &ExptSchedulerImpl{ExptItemResultRepo: f.itemRepo(), ExptTurnResultRepo: f.turnRepo(), IDGen: ids, itemLocker: lateWriteLocker(t, ctrl), Configer: config, evalTargetService: target, EvaluatorRecordRepo: records}
			old := time.Now().Add(-time.Hour)
			source.Current.UpdatedAt = &old
			require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("updated_at", old).Error)
			require.NoError(t, f.conn.Model(&model.ExptTurnResult{}).Where("space_id=? AND turn_id<>0", f.space).Update("target_result_id", 0).Error)
			require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND turn_id<>0", f.space).Update("target_result_id", 0).Error)
			source = f.source()
			before := f.snapshot()
			event := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run}
			item := &entity.ExptEvalItem{Source: source, ItemID: f.item, ExptID: f.expt, State: entity.ItemRunState_Processing, UpdatedAt: source.Current.UpdatedAt}
			inserted := 0
			if kind == "insert_error" || kind == "second_insert_error" {
				require.NoError(t, f.conn.Callback().Create().Before("gorm:create").Register("compat_missing_insert_fault", func(tx *gorm.DB) {
					if tx.Statement.Table == "expt_turn_result_run_log" {
						inserted++
						if kind == "insert_error" || inserted == 2 {
							_ = tx.AddError(fault)
						}
					}
				}))
			}
			var completed []*entity.ExptEvalItem
			var err error
			if kind == "sandbox_all" || kind == "sandbox_partial" {
				_, completed, err = scheduler.sweepCurrentSandboxItems(taskCtx, event, []*entity.ExptEvalItem{item}, &entity.Experiment{})
			} else {
				_, completed, err = scheduler.handleZombies(taskCtx, event, []*entity.ExptEvalItem{item}, nil)
			}
			if kind == "insert_error" || kind == "second_insert_error" {
				require.NoError(t, f.conn.Callback().Create().Remove("compat_missing_insert_fault"))
				require.Positive(t, inserted)
			}
			if kind == "idgen_error" || kind == "idgen_cancel" || kind == "insert_error" || kind == "second_insert_error" {
				require.True(t, errno.NeedMQRetry(err))
				if kind == "idgen_cancel" {
					require.ErrorIs(t, err, context.Canceled)
				} else {
					require.ErrorIs(t, err, fault)
				}
				require.Equal(t, before, f.snapshot())
				require.Empty(t, completed)
				return
			}
			require.NoError(t, err)
			require.Equal(t, 1, calls)
			require.Len(t, completed, 1)
			var logs []model.ExptTurnResultRunLog
			require.NoError(t, f.conn.Where("space_id=?", f.space).Find(&logs).Error)
			require.Len(t, logs, 2)
			for _, log := range logs {
				require.Equal(t, f.version, log.ItemVersionID)
				require.Equal(t, int32(entity.TurnRunState_Fail), log.Status)
				require.NotEmpty(t, gptr.Indirect(log.ErrMsg))
			}
			stats, err := experimentrepo.NewExptStatsRepo(experimentmysql.NewExptStatsDAO(f.provider)).Get(context.Background(), f.expt, f.space)
			require.NoError(t, err)
			require.Equal(t, int32(1), stats.FailItemCnt)
			require.Equal(t, int32(1), stats.ProcessingItemCnt)
			archive := &ExptResultServiceImpl{ExptItemResultRepo: f.itemRepo(), ExptTurnResultRepo: f.turnRepo(), ExptStatsRepo: experimentrepo.NewExptStatsRepo(experimentmysql.NewExptStatsDAO(f.provider)), resultPublishRepo: experimentrepo.NewExptResultPublishRepo(f.provider), resultPublishLocker: lateWriteLocker(t, ctrl), evalTargetService: target}
			expt := &entity.Experiment{ID: f.expt, SpaceID: f.space, EvalConf: &entity.EvaluationConfiguration{}}
			_, err = archive.RecordItemRunLogs(context.Background(), f.expt, f.run, f.item, f.space, expt)
			require.NoError(t, err)
			after, err := archive.ExptStatsRepo.Get(context.Background(), f.expt, f.space)
			require.NoError(t, err)
			require.Equal(t, *stats, *after)
			turnResults, err := f.itemRepo().GetItemTurnResults(context.Background(), f.space, f.expt, f.item)
			require.NoError(t, err)
			builder := &ExptResultBuilder{turnResultDO: turnResults, ItemIDTurnID2TurnResultID: map[int64]map[int64]int64{f.item: {}}}
			for _, turn := range turnResults {
				builder.ItemIDTurnID2TurnResultID[f.item][turn.TurnID] = turn.ID
				info := builder.getTurnSystemInfo(context.Background(), f.item, turn.TurnID)
				require.NotNil(t, info.Error)
				if kind == "sandbox_all" || kind == "sandbox_partial" {
					require.Contains(t, gptr.Indirect(info.Error.Detail), "Failed")
				} else {
					require.Contains(t, gptr.Indirect(info.Error.Detail), "1")
				}
			}
			_, err = archive.RecordItemRunLogs(context.Background(), f.expt, f.run, f.item, f.space, expt)
			require.NoError(t, err)
			final, err := archive.ExptStatsRepo.Get(context.Background(), f.expt, f.space)
			require.NoError(t, err)
			require.Equal(t, *after, *final)
		})
	}
}

func TestLateCompatTurnFailureReasonReplacesOld(t *testing.T) {
	for _, terminal := range []bool{false, true} {
		t.Run(fmt.Sprint(terminal), func(t *testing.T) {
			f := newLateWriteFixture(t, "")
			old := []byte(errno.SerializeErr(errno.NewTurnOtherErr("old reason", errors.New("old error"))))
			require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).Update("err_msg", old).Error)
			state := entity.ItemRunState_Fail
			if terminal {
				state = entity.ItemRunState_Terminal
			}
			source := f.source()
			reason := []byte(errno.SerializeErr(errno.NewItemZombieTimeoutErr(1, false)))
			ok, err := f.port.CompleteCurrentItemRun(context.Background(), source, &entity.ExptCurrentItemRunWrite{Status: state, ErrMsg: reason, FailPendingTurns: true})
			require.NoError(t, err)
			require.True(t, ok)
			logs, err := f.turnRepo().GetItemTurnRunLogs(context.Background(), f.expt, f.run, f.item, f.space)
			require.NoError(t, err)
			for _, log := range logs {
				require.NotEqual(t, string(old), log.ErrMsg, "pending turn must show this completion reason")
				parsed := errno.DeserializeErr([]byte(log.ErrMsg))
				if terminal {
					ok, _ := errno.ParseItemManuallyTerminatedErr(parsed)
					require.True(t, ok)
				} else {
					ok, msg := errno.ParseTurnOtherErr(parsed)
					require.True(t, ok)
					require.Contains(t, msg, "1")
				}
			}
		})
	}
}

func TestLateCompatDebtQuotaReleasedOnlyOnce(t *testing.T) {
	ctrl := gomock.NewController(t)
	guard := componentmocks.NewMockICentralReservationGuard(ctrl)
	dispatch := repomocks.NewMockIExptItemDispatchRepo(ctrl)
	ctx := ctxcache.Init(context.Background())
	execution := &currentItemExecution{}
	ctx = context.WithValue(ctx, currentItemSourceKey{}, execution)
	event := &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 11}
	event.WithCtxCentralQuotaHeld(ctx, "scope")
	dispatch.EXPECT().MGetDispatchObservations(gomock.Any(), int64(3), int64(1), int64(2), []int64{11}).Return([]*repo.ExptDispatchObservation{{ItemID: 11, Status: int32(entity.ItemRunState_Fail)}}, nil)
	guard.EXPECT().Release(gomock.Any(), "scope", int64(2), int64(11), gomock.Any()).Return(nil).Times(1)
	svc := &ExptItemEventEvalServiceImpl{centralGuard: guard, dispatchRepo: dispatch}
	svc.releaseQuotaIfItemTerminal(ctx, event, "scope", guard, errors.New("debt"))
	svc.releaseCentralQuotaOutsideGate(ctx, event, errors.New("debt"), "expt terminated on indebt")
}

func TestLateCompatCompletionPreservesSuccessfulTurn(t *testing.T) {
	for _, kind := range []string{"fail", "terminal", "yield", "rollback"} {
		t.Run(kind, func(t *testing.T) {
			f := newLateWriteFixture(t, "")
			successErr := []byte("successful stage metadata")
			oldPending := []byte(errno.SerializeErr(errno.NewTurnOtherErr("old pending", errors.New("previous error"))))
			require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND turn_id=0", f.space).Updates(map[string]any{"status": int32(entity.TurnRunState_Success), "err_msg": successErr}).Error)
			require.NoError(t, f.conn.Model(&model.ExptTurnResult{}).Where("space_id=? AND turn_id=0", f.space).Update("status", int32(entity.TurnRunState_Success)).Error)
			require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND turn_id=?", f.space, f.base+22).Update("err_msg", oldPending).Error)
			source := f.source()
			before := f.snapshot()
			write := &entity.ExptCurrentItemRunWrite{Status: entity.ItemRunState_Fail, FailPendingTurns: true, ErrMsg: []byte(errno.SerializeErr(errno.NewSandboxTerminatedBeforeReportErr("Failed"))), RetryTimes: 2}
			if kind == "terminal" {
				write.Status = entity.ItemRunState_Terminal
			}
			if kind == "yield" {
				write.Yield = true
			}
			fault := errors.New("turn metadata transaction fault")
			if kind == "rollback" {
				require.NoError(t, f.conn.Callback().Update().Before("gorm:update").Register("compat_turn_metadata_fault", func(tx *gorm.DB) {
					if tx.Statement.Table == "expt_stats" {
						_ = tx.AddError(fault)
					}
				}))
			}
			ok, err := f.port.CompleteCurrentItemRun(context.Background(), source, write)
			if kind == "rollback" {
				require.NoError(t, f.conn.Callback().Update().Remove("compat_turn_metadata_fault"))
				require.ErrorIs(t, err, fault)
				require.False(t, ok)
				require.Equal(t, before, f.snapshot())
				return
			}
			require.NoError(t, err)
			require.True(t, ok)
			logs, err := f.turnRepo().GetItemTurnRunLogs(context.Background(), f.expt, f.run, f.item, f.space)
			require.NoError(t, err)
			for _, log := range logs {
				if log.TurnID == 0 {
					require.Equal(t, source.Turns[0], log)
				} else if kind == "yield" {
					require.Equal(t, string(oldPending), log.ErrMsg)
				} else {
					require.NotEqual(t, string(oldPending), log.ErrMsg)
				}
			}
			after := f.snapshot()
			require.Equal(t, before["expt_turn_evaluator_result_ref"], after["expt_turn_evaluator_result_ref"])
			var canonical model.ExptTurnResult
			require.NoError(t, f.conn.Where("space_id=? AND turn_id=0", f.space).Take(&canonical).Error)
			require.Equal(t, 0.0, gptr.Indirect(canonical.WeightedScore))
			require.Equal(t, f.base+41, canonical.TargetResultID)
		})
	}
}
