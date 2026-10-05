// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	mm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	ec "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/convertor"
	em "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	asyncdao "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/dao"
	idemrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/idem"
	idemredis "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/idem/redis"
	targetmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql"
	tc "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/convertor"
	tm "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type executionChainConfig struct{ schedulerHookConfig }

func (*executionChainConfig) GetErrRetryConf(context.Context, int64, error) *entity.RetryConf {
	return &entity.RetryConf{}
}
func (*executionChainConfig) GetErrCtrl(context.Context) *entity.ExptErrCtrl {
	return &entity.ExptErrCtrl{}
}
func (*executionChainConfig) BuildEvalExt(context.Context, int64, *entity.Turn) map[string]string {
	return map[string]string{}
}

type executionChainPublisher struct {
	schedulerHookPublisher
	items []*entity.ExptItemEvalEvent
}

func (p *executionChainPublisher) BatchPublishExptRecordEvalEvent(_ context.Context, events []*entity.ExptItemEvalEvent, _ *time.Duration) error {
	p.items = append(p.items, events...)
	return nil
}

type executionChainTarget struct {
	activeStoredTargetCleaner
	f             *finalizationManagerFixture
	calls         int
	beforeExecute func(context.Context)
	kind          entity.EvalTargetType
	fail          bool
}

func (t *executionChainTarget) GetEvalTargetVersion(_ context.Context, space, version int64, _ bool) (*entity.EvalTarget, error) {
	if space != t.f.space+90 || version != 92 {
		return nil, fmt.Errorf("wrong original target lookup")
	}
	return &entity.EvalTarget{ID: 91, SpaceID: space, EvalTargetType: t.kind, EvalTargetVersion: &entity.EvalTargetVersion{ID: 92, TargetID: 91, SpaceID: space, EvalTargetType: t.kind}}, nil
}
func (t *executionChainTarget) ExecuteTarget(ctx context.Context, space, id, version int64, in *entity.ExecuteTargetCtx, _ *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error) {
	if space != t.f.space+90 || id != 91 || version != 92 || gptr.Indirect(in.ExperimentRunID) != t.f.key.RunID {
		return nil, fmt.Errorf("wrong original target execution")
	}
	t.calls++
	if t.fail {
		return nil, fmt.Errorf("deliberate target failure")
	}
	if t.beforeExecute != nil {
		t.beforeExecute(ctx)
	}
	r := &entity.EvalTargetRecord{ID: finalizationTestIDs.Add(1), SpaceID: space, TargetID: id, TargetVersionID: version, ExperimentRunID: t.f.key.RunID, ItemID: in.ItemID, ItemVersionID: chainItemVersion(in.ItemID), TurnID: in.TurnID, Status: gptr.Of(entity.EvalTargetRunStatusSuccess), EvalTargetOutputData: &entity.EvalTargetOutputData{OutputFields: map[string]*entity.Content{}}}
	po, err := tc.EvalTargetRecordDO2PO(r)
	if err != nil {
		return nil, err
	}
	if err = t.f.sql.WithContext(ctx).Create(po).Error; err != nil {
		return nil, err
	}
	return r, nil
}

func (t *executionChainTarget) AsyncExecuteTarget(ctx context.Context, space, id, version int64, in *entity.ExecuteTargetCtx, data *entity.EvalTargetInputData) (*entity.EvalTargetRecord, string, error) {
	r, err := t.ExecuteTarget(ctx, space, id, version, in, data)
	if err != nil {
		return nil, "", err
	}
	r.Status = gptr.Of(entity.EvalTargetRunStatusAsyncInvoking)
	if err = t.f.sql.WithContext(ctx).Model(&tm.TargetRecord{}).Where("id=?", r.ID).UpdateColumn("status", int32(entity.EvalTargetRunStatusAsyncInvoking)).Error; err != nil {
		return nil, "", err
	}
	return r, "fixture-target", nil
}
func (*executionChainTarget) LoadRecordOutputFields(context.Context, *entity.EvalTargetRecord, []string) error {
	return nil
}

type executionChainEvaluators struct {
	EvaluatorService
	f    *finalizationManagerFixture
	fail bool
}

func (s *executionChainEvaluators) BatchGetEvaluatorVersion(_ context.Context, _ *int64, ids []int64, _ bool) ([]*entity.Evaluator, error) {
	var out []*entity.Evaluator
	for _, id := range ids {
		if id != 111 && id != 222 {
			return nil, fmt.Errorf("unexpected evaluator")
		}
		out = append(out, &entity.Evaluator{ID: id + 1, SpaceID: s.f.space, EvaluatorType: entity.EvaluatorTypeCode, CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{ID: id, EvaluatorID: id + 1, SpaceID: s.f.space, CodeContent: "code"}})
	}
	return out, nil
}
func (*executionChainEvaluators) ShouldInterceptEvaluator(context.Context, *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, bool, error) {
	return nil, false, nil
}
func (s *executionChainEvaluators) RunEvaluator(ctx context.Context, in *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
	want := int64(111)
	alias := "first"
	if in.ItemID == 802 {
		want, alias = 222, "second"
	}
	if in.ExperimentRunID != s.f.key.RunID || in.SpaceID != s.f.space || in.EvaluatorVersionID != want || in.Alias != alias || in.EvaluatorRunConf == nil || gptr.Indirect(in.EvaluatorRunConf.Env) != "original-env" {
		return nil, fmt.Errorf("wrong frozen evaluator execution")
	}
	r := &entity.EvaluatorRecord{ID: finalizationTestIDs.Add(1), SpaceID: in.SpaceID, ExperimentID: in.ExperimentID, ExperimentRunID: in.ExperimentRunID, ItemID: in.ItemID, ItemVersionID: chainItemVersion(in.ItemID), TurnID: in.TurnID, EvaluatorVersionID: in.EvaluatorVersionID, Alias: in.Alias, SourceType: entity.EvaluatorRecordSourceTypeBuiltin, Status: entity.EvaluatorRunStatusSuccess, EvaluatorOutputData: &entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: gptr.Of(0.75)}}}
	if s.fail {
		r.Status = entity.EvaluatorRunStatusFail
		r.EvaluatorOutputData.EvaluatorRunError = &entity.EvaluatorRunError{Code: 1, Message: "deliberate evaluator failure"}
	}
	po := ec.ConvertEvaluatorRecordDO2PO(r)
	if err := s.f.sql.WithContext(ctx).Create(po).Error; err != nil {
		return nil, err
	}
	return r, nil
}
func chainItemVersion(id int64) int64 {
	if id == 802 {
		return 7
	}
	return 0
}

func executionChainFixture(t *testing.T, mode entity.ExptRunMode, options ...string) (*finalizationManagerFixture, HookRuntimeExecutionDependencies, *executionChainPublisher, *executionChainTarget) {
	t.Helper()
	ctx := context.Background()
	before, async := false, false
	for _, option := range options {
		before = before || option == "before"
		async = async || option == "async"
	}
	f := foundationManagerFixture(t, before, true)
	expt, err := f.manager.exptRepo.GetByID(ctx, f.expt, f.space)
	require.NoError(t, err)
	expt.EvalConf.ConnectorConf.TargetConf.IngressConf.EvalSetAdapter = &entity.FieldAdapter{}
	expt.EvalConf.ConnectorConf.EvaluatorsConf = &entity.EvaluatorsConf{EvaluatorConcurNum: gptr.Of(2)}
	for _, id := range []int64{111, 222} {
		expt.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConf = append(expt.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConf, &entity.EvaluatorConf{EvaluatorVersionID: id, IngressConf: &entity.EvaluatorIngressConf{EvalSetAdapter: &entity.FieldAdapter{}}, RunConf: &entity.EvaluatorRunConfig{Env: gptr.Of("original-env")}})
	}
	for _, option := range options {
		if option == "retry-stage" {
			expt.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConf = append(expt.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConf, &entity.EvaluatorConf{EvaluatorVersionID: 333, IngressConf: &entity.EvaluatorIngressConf{EvalSetAdapter: &entity.FieldAdapter{}}, RunConf: &entity.EvaluatorRunConfig{Env: gptr.Of("original-env")}})
			expt.EvalConf.EvalSetConfigs[1].EvaluatorConfs = append(expt.EvalConf.EvalSetConfigs[1].EvaluatorConfs, &entity.ExptEvaluatorConf{EvaluatorVersionID: 333, Alias: "retry-failure"})
		}
	}
	expt.EvalConf.EvalSetConfigs[0].TargetConfs = nil
	expt.EvalConf.ItemConcurNum = gptr.Of(2)
	raw, err := json.Marshal(expt.EvalConf)
	require.NoError(t, err)
	kind := entity.EvalTargetTypeLoopPrompt
	if async {
		kind = entity.EvalTargetTypeSandboxAgent
	}
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumns(map[string]any{"eval_conf": raw, "target_type": int32(kind)}).Error)
	foundationCreate(t, f, mode)
	plans := exptinfra.NewHookPlanRepo(f.p)
	selector := &preparerSelector{selectFn: func(_ context.Context, in entity.HookSelectionInput) (entity.HookSelectionPage, error) {
		require.Equal(t, mode, in.Mode)
		return entity.HookSelectionPage{Items: []entity.HookPlanItem{{SourceSpaceID: f.space, EvalSetID: 71, ItemID: 801}, {SourceSpaceID: f.space + 50, EvalSetID: 81, EvalSetVersionID: 82, ItemID: 802, ItemVersionID: 7}}, NextCursor: "done", Done: true}, nil
	}}
	preparer, err := NewHookPlanPreparer(HookPlanPreparerDependencies{Runs: f.repo, Plans: plans, Selector: selector, Codec: f.manager.hooks.Codec, Experiments: f.manager.exptRepo, Initialization: f.manager.hooks.Initialization, ResultReader: exptinfra.NewHookPlanResultReader(f.p), IDs: activeReferenceIDs{}})
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		require.NoError(t, preparer.PreparePlan(ctx, hook.WorkerRunInput{ExecutionScope: "local", Candidate: entity.HookRunCandidate{Key: f.key}}))
	}
	require.True(t, finalizationRead(t, f).PlanReady)
	loader, err := NewHookFrozenPlanLoader(HookFrozenPlanLoaderDependencies{Plans: plans, Items: new(boundInitDataset), Versions: &loaderVersions{value: &entity.EvaluationSetVersion{ID: 82, SpaceID: f.space + 50, EvaluationSetID: 81, EvaluationSetSchema: &entity.EvaluationSetSchema{ID: 83, SpaceID: f.space + 50, EvaluationSetID: 81}}}, Sets: &loaderSets{value: &entity.EvaluationSet{ID: 71, SpaceID: f.space, EvaluationSetVersion: &entity.EvaluationSetVersion{EvaluationSetSchema: &entity.EvaluationSetSchema{ID: 73, SpaceID: f.space, EvaluationSetID: 71}}}}})
	require.NoError(t, err)
	loader = boundConsumerLoaderMutation{loader, func(p *entity.HookLoadedPlanPage) {
		for _, item := range p.Items {
			item.Item.BaseInfo = &entity.BaseInfo{}
		}
	}}
	binding, err := LoadHookExecutionInitializationBinding(ctx, f.repo, f.manager.hooks.Codec, f.key, "local")
	require.NoError(t, err)
	stores, err := exptinfra.NewBoundHookRuntimeRepositories(f.p, binding)
	require.NoError(t, err)
	d, pub, target := executionChainDependencies(t, f, binding, stores, loader, kind)
	return f, d, pub, target
}

func executionChainDependencies(t *testing.T, f *finalizationManagerFixture, binding *entity.HookExecutionInitializationBinding, stores repo.HookBoundRuntimeRepositories, loader hook.PlanPageLoader, kind entity.EvalTargetType) (HookRuntimeExecutionDependencies, *executionChainPublisher, *executionChainTarget) {
	t.Helper()
	f.manager.finalization.NewItemLocker = func() lock.ILocker { return lock.NewRedisLocker(f.redis) }
	items, turns := lazyTurnRepos(f)
	pub := new(executionChainPublisher)
	cfg := new(executionChainConfig)
	ctrl := gomock.NewController(t)
	metric := mm.NewMockExptMetric(ctrl)
	metric.EXPECT().EmitItemExecEval(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	metric.EXPECT().EmitItemExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	metric.EXPECT().EmitTurnExecEval(gomock.Any(), gomock.Any()).AnyTimes()
	metric.EXPECT().EmitTurnExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	metric.EXPECT().EmitTurnExecTargetResult(gomock.Any(), gomock.Any()).AnyTimes()
	metric.EXPECT().EmitTurnExecEvaluatorResult(gomock.Any(), gomock.Any()).AnyTimes()
	target := &executionChainTarget{f: f, kind: kind, activeStoredTargetCleaner: activeStoredTargetCleaner{dao: targetmysql.NewEvalTargetRecordDAO(f.p)}}
	evals := &executionChainEvaluators{f: f}
	records := activeStoredEvaluatorReader{sql: f.sql}
	result := &ExptResultServiceImpl{idgen: activeReferenceIDs{}, evaluatorRecordService: records, scoreCalculator: NewEvaluatorScoreCalculator(nil, nil)}
	consumer := &ExptItemEventEvalServiceImpl{manager: f.manager, experimentRepo: f.manager.exptRepo, publisher: pub, mutex: lock.NewRedisLocker(f.redis), exptItemResultRepo: items, exptTurnResultRepo: turns, configer: cfg, metric: metric, evaTargetService: target, evaluatorService: evals, evaluatorRecordService: records, benefitService: benefit.NoopBenefitServiceImpl{}, evaluationSetItemService: new(boundConsumerDataset), evalAsyncRepo: exptinfra.NewEvalAsyncRepo(asyncdao.NewEvalAsyncDAO(f.redis, nil))}
	scheduler := &ExptSchedulerImpl{Manager: f.manager, ExptRepo: f.manager.exptRepo, Publisher: pub, Configer: cfg, Metric: metric, Mutex: lock.NewRedisLocker(f.redis), ExptItemResultRepo: items, ExptTurnResultRepo: turns, ExptStatsRepo: exptinfra.NewExptStatsRepo(exptmysql.NewExptStatsDAO(f.p)), ResultSvc: result, Idem: idemrepo.NewIdempotentService(idemredis.NewIdemDAO(f.redis))}
	scheduler.schedulerModeFactory = NewSchedulerModeFactory(f.manager, items, scheduler.ExptStatsRepo, turns, activeReferenceIDs{}, new(boundConsumerDataset), f.manager.exptRepo, nil, scheduler.Idem, cfg, pub, records, result, nil, nil, nil)
	t.Cleanup(func() {
		require.NoError(t, f.sql.Unscoped().Where("space_id=? AND expt_id=?", f.space, f.expt).Delete(&model.ExptItemRef{}).Error)
		require.NoError(t, f.sql.Unscoped().Where("experiment_run_id=?", f.key.RunID).Delete(&tm.TargetRecord{}).Error)
		require.NoError(t, f.sql.Unscoped().Where("experiment_run_id=?", f.key.RunID).Delete(&em.EvaluatorRecord{}).Error)
		require.NoError(t, f.sql.Unscoped().Where("space_id=? AND expt_id=?", f.space, f.expt).Delete(&model.ExptTurnEvaluatorResultRef{}).Error)
	})
	return HookRuntimeExecutionDependencies{Manager: f.manager, Scheduler: scheduler, Consumer: consumer, Binding: binding, Repositories: stores, Loader: loader, IDs: activeReferenceIDs{}}, pub, target
}
