// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

type boundConsumerDBManager struct {
	IExptManager
	f *finalizationManagerFixture
}

func (m boundConsumerDBManager) GetDetail(ctx context.Context, id, space int64, _ *entity.Session, _ ...entity.GetExptTupleOptionFn) (*entity.Experiment, error) {
	expt, err := (afterOnlyExperimentReader{sql: m.f.sql}).GetByID(ctx, id, space)
	if err != nil {
		return nil, err
	}
	expt.EvalSet = &entity.EvaluationSet{ID: expt.EvalSetID, SpaceID: space, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: expt.EvalSetVersionID, EvaluationSetID: expt.EvalSetID, SpaceID: space}}
	return expt, nil
}

type boundConsumerDataset struct{ boundInitDataset }

func (s *boundConsumerDataset) BatchGetEvaluationSetItems(ctx context.Context, in *entity.BatchGetEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, error) {
	if len(in.ItemVersionQueries) == 0 {
		return s.boundInitDataset.BatchGetEvaluationSetItems(ctx, in)
	}
	var out []*entity.EvaluationSetItem
	for _, q := range in.ItemVersionQueries {
		out = append(out, &entity.EvaluationSetItem{ID: q.ItemID, ItemID: q.ItemID, SpaceID: in.SpaceID, EvaluationSetID: in.EvaluationSetID, ItemVersionID: q.ItemVersionID, Turns: []*entity.Turn{{ID: 0, ItemID: q.ItemID, EvalSetID: in.EvaluationSetID}}})
	}
	return out, nil
}

type boundConsumerTarget struct {
	IEvalTargetService
	value          *entity.EvalTarget
	reads, effects int
	afterRead      func()
}

func (s *boundConsumerTarget) GetEvalTargetVersion(_ context.Context, space, version int64, _ bool) (*entity.EvalTarget, error) {
	s.reads++
	if space != s.value.SpaceID || version != s.value.EvalTargetVersion.ID {
		return nil, errors.New("unexpected target routing")
	}
	if s.afterRead != nil {
		s.afterRead()
	}
	return s.value, nil
}
func (s *boundConsumerTarget) ExecuteTarget(context.Context, int64, int64, int64, *entity.ExecuteTargetCtx, *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error) {
	s.effects++
	return nil, errors.New("model effects outside context slice")
}

type boundConsumerEvaluators struct {
	EvaluatorService
	value          *entity.Evaluator
	reads, effects int
}

func (s *boundConsumerEvaluators) BatchGetEvaluatorVersion(_ context.Context, _ *int64, ids []int64, _ bool) ([]*entity.Evaluator, error) {
	s.reads++
	if len(ids) != 1 || ids[0] != s.value.GetEvaluatorVersionID() {
		return nil, errors.New("unexpected evaluator routing")
	}
	return []*entity.Evaluator{s.value}, nil
}
func (s *boundConsumerEvaluators) RunEvaluator(context.Context, *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
	s.effects++
	return nil, errors.New("model effects outside context slice")
}

func boundConsumerFixture(t *testing.T, own ...bool) (*boundInitFixture, *ExptItemEventEvalServiceImpl) {
	t.Helper()
	return boundConsumerFixtureConfig(t, nil, own...)
}

func boundConsumerFixtureConfig(t *testing.T, configure func(*entity.EvaluationConfiguration), own ...bool) (*boundInitFixture, *ExptItemEventEvalServiceImpl) {
	t.Helper()
	f := foundationManagerFixture(t, false, true)
	ctx := context.Background()
	expt, err := f.manager.exptRepo.GetByID(ctx, f.expt, f.space)
	require.NoError(t, err)
	expt.EvalConf.ConnectorConf.EvaluatorsConf = &entity.EvaluatorsConf{EvaluatorConcurNum: gptr.Of(2), EvaluatorConf: []*entity.EvaluatorConf{{EvaluatorVersionID: 222, IngressConf: &entity.EvaluatorIngressConf{EvalSetAdapter: &entity.FieldAdapter{}}, RunConf: &entity.EvaluatorRunConfig{Env: gptr.Of("original-env"), EvaluatorRuntimeParam: &entity.RuntimeParam{JSONValue: gptr.Of(`{"static":"original"}`)}}}}}
	expt.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConf = append(expt.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConf, &entity.EvaluatorConf{EvaluatorVersionID: 111, IngressConf: &entity.EvaluatorIngressConf{EvalSetAdapter: &entity.FieldAdapter{}}, RunConf: &entity.EvaluatorRunConfig{Env: gptr.Of("first-env")}})
	if configure != nil {
		configure(expt.EvalConf)
	}
	raw, err := json.Marshal(expt.EvalConf)
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumn("eval_conf", raw).Error)
	foundationCreate(t, f, entity.EvaluationModeSubmit)
	if len(own) > 1 && own[1] || len(own) > 2 && own[2] {
		run := finalizationRead(t, f)
		snapshot, err := f.manager.hooks.Codec.DecodeSnapshot(ctx, f.key, "local", run.Snapshot)
		require.NoError(t, err)
		in := snapshot.Input()
		if own[1] {
			in.Execution.Sets[0].ItemConfig.EvalTargetConf.RunConf.FixedQueryList = []*entity.FixedQuery{{Evaluators: map[string]interface{}{"opaque_id": json.Number("9007199254740993")}}}
		}
		if len(own) > 2 && own[2] {
			in.Execution.Sets[0].ItemConfig.TargetSourceSpaceID = f.space + 91
		}
		snapshot, err = entity.NewHookRunSnapshot(in)
		require.NoError(t, err)
		protected, err := f.manager.hooks.Codec.EncodeSnapshot(ctx, "key", snapshot)
		require.NoError(t, err)
		require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumns(map[string]interface{}{"snapshot_cipher": protected.Cipher, "snapshot_hash": protected.Hash}).Error)
	}
	item := entity.HookPlanItem{ID: finalizationTestIDs.Add(1), ItemID: finalizationTestIDs.Add(1), ItemVersionID: 7, SourceSpaceID: f.space + 50, EvalSetID: 81, EvalSetVersionID: 82}
	if len(own) > 0 && own[0] {
		item.ItemVersionID, item.SourceSpaceID, item.EvalSetID, item.EvalSetVersionID = 0, f.space, 71, 0
	}
	run := finalizationRead(t, f)
	page, err := f.repo.AppendPlanPage(ctx, entity.HookPlanPageInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: run.Version}, Items: []entity.HookPlanItem{item}})
	require.NoError(t, err)
	digest, err := entity.AppendHookPlanDigest(entity.NewHookPlanDigest(), []entity.HookPlanItem{item})
	require.NoError(t, err)
	_, err = f.repo.FinishPlan(ctx, entity.HookFinishPlanInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: page.Run.Version}, Count: 1, Hash: digest.Hash})
	require.NoError(t, err)
	b := &boundInitFixture{finalizationManagerFixture: f, items: []entity.HookPlanItem{item}, dataset: new(boundInitDataset)}
	b.loader, err = NewHookFrozenPlanLoader(HookFrozenPlanLoaderDependencies{Plans: exptinfra.NewHookPlanRepo(f.p), Items: b.dataset, Versions: &loaderVersions{value: &entity.EvaluationSetVersion{ID: 82, SpaceID: f.space + 50, EvaluationSetID: 81, EvaluationSetSchema: &entity.EvaluationSetSchema{ID: 83, SpaceID: f.space + 50, EvaluationSetID: 81}}}, Sets: &loaderSets{value: &entity.EvaluationSet{ID: 71, SpaceID: f.space, EvaluationSetVersion: &entity.EvaluationSetVersion{EvaluationSetSchema: &entity.EvaluationSetSchema{ID: 73, SpaceID: f.space, EvaluationSetID: 71}}}}})
	require.NoError(t, err)
	r := boundInitializationRepo(t, b)
	_, err = b.initialize(t, r)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, f.sql.Unscoped().Where("space_id=? AND expt_id=?", f.space, f.expt).Delete(&model.ExptItemRef{}).Error)
	})
	items, turns := lazyTurnRepos(f)
	base := &ExptItemEventEvalServiceImpl{manager: boundConsumerDBManager{IExptManager: f.manager, f: f}, publisher: &schedulerHookPublisher{}, mutex: f.manager.mutex, exptItemResultRepo: items, exptTurnResultRepo: turns, exptItemRefRepo: exptinfra.NewExptItemRefRepo(exptmysql.NewExptItemRefDAO(f.p)), evaluationSetItemService: new(boundConsumerDataset)}
	base.evaTargetService = &boundConsumerTarget{value: &entity.EvalTarget{ID: 91, SpaceID: f.space + 90, SourceTargetID: "global-source", EvalTargetType: entity.EvalTargetTypeSandboxAgent, EvalTargetVersion: &entity.EvalTargetVersion{ID: 92, TargetID: 91, SpaceID: f.space + 90, EvalTargetType: entity.EvalTargetTypeSandboxAgent, SourceTargetVersion: "global-v1"}}}
	base.evaluatorService = &boundConsumerEvaluators{value: &entity.Evaluator{ID: 333, SpaceID: f.space, EvaluatorType: entity.EvaluatorTypeCode, CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{ID: 222, EvaluatorID: 333, SpaceID: f.space, CodeContent: "original-code"}}}
	if len(own) > 0 && own[0] {
		base.evaluatorService.(*boundConsumerEvaluators).value.CodeEvaluatorVersion.ID = 111
	}
	aware, err := NewHookAwareExptRecordEvalService(base, exptinfra.NewHookExecutionGateRepo(f.p, func(context.Context) (string, error) { return "local", nil }), exptinfra.NewHookItemSourceRepo(f.p), f.repo, exptinfra.NewHookTurnProgressRepo(f.p, func(context.Context) (string, error) { return "local", nil }))
	require.NoError(t, err)
	reader, err := exptinfra.NewBoundHookConsumerRepo(f.p, b.binding)
	require.NoError(t, err)
	consumer, err := NewBoundHookExptRecordContextService(aware, b.binding, reader, b.loader)
	require.NoError(t, err)
	return b, consumer
}

func TestHookBoundConsumerOriginalExecutionContextMySQL(t *testing.T) {
	f, consumer := boundConsumerFixture(t)
	ctx := context.Background()
	expt, err := f.manager.exptRepo.GetByID(ctx, f.expt, f.space)
	require.NoError(t, err)
	expt.EvalConf.RunModeConfig.MaxRunMinutes = 99
	expt.EvalConf.ConnectorConf.TargetConf.TargetVersionID = 778
	raw, err := json.Marshal(expt.EvalConf)
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumns(map[string]interface{}{"eval_conf": raw, "target_id": 777, "target_version_id": 778, "target_space_id": f.space + 999}).Error)
	event := &entity.ExptItemEvalEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ExptRunMode: entity.EvaluationModeSubmit, EvalSetItemID: f.items[0].ItemID}
	got, err := consumer.BuildExptRecordEvalCtx(ctx, event)
	require.NoError(t, err)
	require.Equal(t, int64(91), got.Expt.TargetID, "the real consumer must use original GLOBAL target, not current config")
	require.Equal(t, int64(92), got.Expt.TargetVersionID)
	require.Equal(t, 7, got.Expt.EvalConf.RunModeConfig.MaxRunMinutes)
	require.Equal(t, int64(222), got.ItemConfig.EvaluatorConfs[0].EvaluatorVersionID)
}

func TestHookBoundConsumerOriginalEvaluatorFallbackMySQL(t *testing.T) {
	f, consumer := boundConsumerFixture(t)
	ctx := context.Background()
	expt, err := f.manager.exptRepo.GetByID(ctx, f.expt, f.space)
	require.NoError(t, err)
	static := expt.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConf[0].RunConf
	static.Env = gptr.Of("changed-env")
	static.EvaluatorRuntimeParam.JSONValue = gptr.Of(`{"static":"changed"}`)
	raw, err := json.Marshal(expt.EvalConf)
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumn("eval_conf", raw).Error)
	got, err := consumer.BuildExptRecordEvalCtx(ctx, &entity.ExptItemEvalEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ExptRunMode: entity.EvaluationModeSubmit, EvalSetItemID: f.items[0].ItemID})
	require.NoError(t, err)
	require.Equal(t, "original-env", *got.Expt.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConf[0].RunConf.Env)
}
