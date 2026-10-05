// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

func boundConsumerEvent(f *boundInitFixture) *entity.ExptItemEvalEvent {
	return &entity.ExptItemEvalEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ExptRunMode: entity.EvaluationModeSubmit, EvalSetItemID: f.items[0].ItemID, Ext: map[string]string{"input": "original"}}
}

func TestHookBoundConsumerRejectsCorruptProofBeforeEffectsMySQL(t *testing.T) {
	for _, kind := range []string{"run", "space", "mode", "missing", "deleted", "replaced", "config", "version", "set", "hash", "not-latest", "uninitialized"} {
		t.Run(kind, func(t *testing.T) {
			f, c := boundConsumerFixture(t)
			event := boundConsumerEvent(f)
			var ref model.ExptItemRef
			require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&ref).Error)
			switch kind {
			case "run":
				event.ExptRunID++
			case "space":
				event.SpaceID++
			case "mode":
				event.ExptRunMode = entity.EvaluationModeRetryAll
			case "missing":
				require.NoError(t, f.sql.Unscoped().Delete(&ref).Error)
			case "deleted":
				require.NoError(t, f.sql.Delete(&ref).Error)
			case "replaced":
				require.NoError(t, f.sql.Model(&ref).UpdateColumn("id", finalizationTestIDs.Add(1)).Error)
			case "config":
				require.NoError(t, f.sql.Model(&ref).UpdateColumn("item_config", []byte(`{}`)).Error)
			case "version":
				require.NoError(t, f.sql.Model(&ref).UpdateColumn("item_version_id", 99).Error)
			case "set":
				require.NoError(t, f.sql.Model(&ref).UpdateColumn("eval_set_id", 999).Error)
			case "hash":
				require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("snapshot_hash", fmt.Sprintf("%064d", 1)).Error)
			case "not-latest":
				require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumn("latest_run_id", finalizationTestIDs.Add(1)).Error)
			case "uninitialized":
				require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("execution_initialized", false).Error)
			}
			reads := f.dataset.calls.Load()
			next := false
			err := c.HandleEventExec(func(context.Context, *entity.ExptItemEvalEvent) error { next = true; return nil })(context.Background(), event)
			require.Error(t, err)
			require.False(t, next)
			require.Equal(t, reads, f.dataset.calls.Load())
			require.Zero(t, c.evaTargetService.(*boundConsumerTarget).reads)
			require.Zero(t, c.evaTargetService.(*boundConsumerTarget).effects)
			require.Zero(t, c.evaluatorService.(*boundConsumerEvaluators).effects)
		})
	}
}

func TestHookBoundConsumerRechecksAfterExternalReadsMySQL(t *testing.T) {
	for _, kind := range []string{"cancel", "replace-ref", "wrong-target-version", "wrong-evaluator-version"} {
		t.Run(kind, func(t *testing.T) {
			f, c := boundConsumerFixture(t)
			target := c.evaTargetService.(*boundConsumerTarget)
			target.afterRead = func() {
				switch kind {
				case "cancel":
					run := finalizationRead(t, f.finalizationManagerFixture)
					_, err := f.repo.BeginFinalize(context.Background(), entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: run.Version}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated}})
					require.NoError(t, err)
				case "replace-ref":
					require.NoError(t, f.sql.Model(&model.ExptItemRef{}).Where("space_id=? AND expt_id=?", f.space, f.expt).UpdateColumn("id", finalizationTestIDs.Add(1)).Error)
				case "wrong-target-version":
					target.value.EvalTargetVersion.ID = 999
				case "wrong-evaluator-version":
					c.evaluatorService.(*boundConsumerEvaluators).value.CodeEvaluatorVersion.ID = 999
				}
			}
			got, err := c.BuildExptRecordEvalCtx(context.Background(), boundConsumerEvent(f))
			require.Error(t, err)
			require.Nil(t, got)
			require.Zero(t, target.effects)
			require.Zero(t, c.evaluatorService.(*boundConsumerEvaluators).effects)
		})
	}
}

func TestHookBoundConsumerCopiesAndLiveControlsMySQL(t *testing.T) {
	f, c := boundConsumerFixture(t, true)
	ctx := context.Background()
	event := boundConsumerEvent(f)
	expt, err := f.manager.exptRepo.GetByID(ctx, f.expt, f.space)
	require.NoError(t, err)
	expt.EvalConf.ItemConcurNum = gptr.Of(9)
	expt.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConcurNum = gptr.Of(8)
	expt.EvalConf.ExpectedQuotaConsumption = &entity.ExpectedQuotaConsumption{Resources: []*entity.ExpectedResourceConsumption{{Category: "live", ResourceKey: "quota", Amount: 4}}}
	raw, err := json.Marshal(expt.EvalConf)
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumns(map[string]interface{}{"eval_conf": raw, "scheduler_mode": "enforce", "scheduler_scope": "original-scheduler", "priority_level": 73}).Error)
	proof, err := c.boundContext.reader.ReadBoundConsumerItem(ctx, f.key, "local", f.items[0].ItemID)
	require.NoError(t, err)
	assertBoundConsumerLiveControls(t, proof.Experiment)
	require.Equal(t, int64(1000), proof.ItemConfig.EvalTargetConf.TargetVersionID)
	got, err := c.BuildExptRecordEvalCtx(ctx, event)
	require.NoError(t, err)
	assertBoundConsumerLiveControls(t, got.Expt)
	require.Zero(t, got.EvalSetSourceSpaceID())
	require.Equal(t, f.space+90, got.TargetSourceSpaceID())
	require.Equal(t, int64(1000), got.ItemConfig.EvalTargetConf.TargetVersionID)
	require.Equal(t, int64(92), got.Expt.TargetVersionID)
	require.NotNil(t, got.Expt.EvalConf.ItemConcurNum, "current control configuration must be read, not discarded by a narrow projection")
	require.Equal(t, 9, *got.Expt.EvalConf.ItemConcurNum)
	require.Equal(t, 8, *got.Expt.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConcurNum)
	require.EqualValues(t, 4, got.Expt.EvalConf.ExpectedQuotaConsumption.Resources[0].Amount)
	require.Equal(t, entity.ExptStatus_Processing, got.Expt.Status)
	got.ItemConfig.EvaluatorConfs[0].Alias = "changed"
	got.Expt.Target.EvalTargetVersion.SourceTargetVersion = "changed"
	got.Expt.Evaluators[0].CodeEvaluatorVersion.CodeContent = "changed"
	got.Expt.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConf[0].RunConf.Env = gptr.Of("changed")
	got.Event.Ext["input"] = "changed"
	next, err := c.BuildExptRecordEvalCtx(ctx, event)
	require.NoError(t, err)
	require.Equal(t, "first", next.ItemConfig.EvaluatorConfs[0].Alias)
	require.Equal(t, "global-v1", next.Expt.Target.EvalTargetVersion.SourceTargetVersion)
	require.Equal(t, "original-code", next.Expt.Evaluators[0].CodeEvaluatorVersion.CodeContent)
	require.Equal(t, "original-env", *next.Expt.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConf[0].RunConf.Env)
	require.Equal(t, "original", event.Ext["input"], "bound context must own request data too")
	require.Error(t, c.Eval(ctx, event), "context-only constructor must not execute the deferred path")
	require.Zero(t, c.evaTargetService.(*boundConsumerTarget).effects)
}

type boundConsumerLoaderMutation struct {
	hook.PlanPageLoader
	mutate func(*entity.HookLoadedPlanPage)
}

func (l boundConsumerLoaderMutation) LoadPage(ctx context.Context, in entity.HookPlanReadInput) (*entity.HookLoadedPlanPage, error) {
	p, err := l.PlanPageLoader.LoadPage(ctx, in)
	if err == nil {
		l.mutate(p)
	}
	return p, err
}

func TestHookBoundConsumerRejectsLoaderVersionDriftMySQL(t *testing.T) {
	f, c := boundConsumerFixture(t)
	require.Equal(t, int64(7), f.items[0].ItemVersionID)
	c.boundContext.loader = boundConsumerLoaderMutation{c.boundContext.loader, func(p *entity.HookLoadedPlanPage) { p.Items[0].Item.ItemVersionID = gptr.Of(int64(9)) }}
	got, err := c.BuildExptRecordEvalCtx(context.Background(), boundConsumerEvent(f))
	require.Error(t, err)
	require.Nil(t, got)
	require.Zero(t, c.evaTargetService.(*boundConsumerTarget).reads)
}

func assertBoundConsumerLiveControls(t *testing.T, expt *entity.Experiment) {
	t.Helper()
	require.Equal(t, "enforce", expt.ExptDispatchMode)
	require.Equal(t, "original-scheduler", expt.SchedulerScope)
	require.EqualValues(t, 73, expt.PriorityLevel)
	require.Equal(t, 9, *expt.EvalConf.ItemConcurNum)
	require.Equal(t, 8, *expt.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConcurNum)
	require.Equal(t, "live", expt.EvalConf.ExpectedQuotaConsumption.Resources[0].Category)
	require.Equal(t, "quota", expt.EvalConf.ExpectedQuotaConsumption.Resources[0].ResourceKey)
	require.EqualValues(t, 4, expt.EvalConf.ExpectedQuotaConsumption.Resources[0].Amount)
}

type boundConsumerZeroVersionDataset struct{ *boundInitDataset }

func (s boundConsumerZeroVersionDataset) BatchGetEvaluationSetItems(ctx context.Context, in *entity.BatchGetEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, error) {
	items, err := s.boundInitDataset.BatchGetEvaluationSetItems(ctx, in)
	for _, item := range items {
		item.ItemVersionID = gptr.Of(int64(99))
	}
	return items, err
}

func TestHookBoundConsumerActualLoaderAcceptsZeroVersionMetadataMySQL(t *testing.T) {
	f, c := boundConsumerFixture(t, true)
	require.Zero(t, f.items[0].ItemVersionID)
	f.loader.(*hookFrozenPlanLoader).deps.Items = boundConsumerZeroVersionDataset{f.dataset}
	got, err := c.BuildExptRecordEvalCtx(context.Background(), boundConsumerEvent(f))
	require.NoError(t, err, "zero is the existing unpinned sentinel, not a demand for zero metadata")
	require.Equal(t, int64(99), *got.EvalSetItem.ItemVersionID)
	require.Equal(t, int64(71), got.EvalSetItem.EvaluationSetID)
	require.Equal(t, int64(1000), got.ItemConfig.EvalTargetConf.TargetVersionID)
}

func TestHookBoundConsumerLegacySnapshotFallbackMissingMySQL(t *testing.T) {
	f, c := boundConsumerFixture(t)
	ctx := context.Background()
	run := finalizationRead(t, f.finalizationManagerFixture)
	s, err := f.manager.hooks.Codec.DecodeSnapshot(ctx, f.key, "local", run.Snapshot)
	require.NoError(t, err)
	in := s.Input()
	in.Execution.EvaluatorFallback = nil
	s, err = entity.NewHookRunSnapshot(in)
	require.NoError(t, err)
	run.Snapshot, err = f.manager.hooks.Codec.EncodeSnapshot(ctx, "key", s)
	require.NoError(t, err)
	binding, err := entity.NewHookExecutionInitializationBinding(run, s)
	require.NoError(t, err)
	reader, err := exptinfra.NewBoundHookConsumerRepo(f.p, binding)
	require.NoError(t, err)
	base := *c
	base.boundContext = nil
	_, err = NewBoundHookExptRecordContextService(&base, binding, reader, f.loader)
	require.ErrorIs(t, err, entity.ErrHookExecutionUnsupported)
}

func TestHookBoundConsumerPrecisionAndScopeMySQL(t *testing.T) {
	f, c := boundConsumerFixture(t, true, true)
	ctx := context.Background()
	got, err := c.BuildExptRecordEvalCtx(ctx, boundConsumerEvent(f))
	require.NoError(t, err)
	numbers := got.ItemConfig.EvalTargetConf.RunConf.FixedQueryList[0].Evaluators
	raw, err := json.Marshal(numbers)
	require.NoError(t, err)
	require.Equal(t, `{"opaque_id":9007199254740993}`, string(raw))
	numbers["opaque_id"] = json.Number("2")
	again, err := c.BuildExptRecordEvalCtx(ctx, boundConsumerEvent(f))
	require.NoError(t, err)
	raw, err = json.Marshal(again.ItemConfig.EvalTargetConf.RunConf.FixedQueryList[0].Evaluators)
	require.NoError(t, err)
	require.Equal(t, `{"opaque_id":9007199254740993}`, string(raw))
	_, err = c.boundContext.reader.ReadBoundConsumerItem(ctx, f.key, "wrong-scope", f.items[0].ItemID)
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
}

func TestHookBoundConsumerActualLoaderRejectsWrongSchemaMySQL(t *testing.T) {
	f, c := boundConsumerFixture(t)
	loader := f.loader.(*hookFrozenPlanLoader)
	loader.deps.Versions.(*loaderVersions).value.EvaluationSetSchema.SpaceID = f.space
	got, err := c.BuildExptRecordEvalCtx(context.Background(), boundConsumerEvent(f))
	require.ErrorIs(t, err, entity.ErrHookFrozenSchemaInvalid)
	require.Nil(t, got)
	require.Zero(t, c.evaTargetService.(*boundConsumerTarget).reads)
}

func TestHookBoundConsumerReadOnlyEntryDoesNotWriteMySQL(t *testing.T) {
	f, c := boundConsumerFixture(t)
	before := finalizationRead(t, f.finalizationManagerFixture)
	err := c.HandleEventExec(func(context.Context, *entity.ExptItemEvalEvent) error { return nil })(context.Background(), boundConsumerEvent(f))
	require.Error(t, err)
	require.Equal(t, before, finalizationRead(t, f.finalizationManagerFixture))
	var count int64
	require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).Count(&count).Error)
	require.Zero(t, count)
	require.Zero(t, c.evaTargetService.(*boundConsumerTarget).effects)
	require.Zero(t, c.evaluatorService.(*boundConsumerEvaluators).effects)
}

func TestHookBoundConsumerRejectsConflictingTargetSpaceMySQL(t *testing.T) {
	f, c := boundConsumerFixture(t, true, false, true)
	got, err := c.BuildExptRecordEvalCtx(context.Background(), boundConsumerEvent(f))
	require.Error(t, err, "a per-set source must not redirect the frozen GLOBAL target to a different owner")
	require.Nil(t, got)
	require.Zero(t, c.evaTargetService.(*boundConsumerTarget).effects)
}
