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
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

func foundationManagerFixture(t *testing.T, before, after bool) *finalizationManagerFixture {
	t.Helper()
	f := newFinalizationManagerFixture(t, "empty")
	ctx := context.Background()
	codec := hookinfra.NewStorageCodec(new(managerProtector))
	owner := hook.ConfigOwner{WorkspaceID: f.space, ObjectID: f.expt, Kind: hook.ConfigOwnerExperiment, ExecutionScope: "local"}
	raw, err := codec.EncodeConfig(ctx, "key", owner, managerEnabledConfig(before, after))
	require.NoError(t, err)
	conf := &entity.EvaluationConfiguration{
		ItemConcurNum: gptr.Of(77),
		ConnectorConf: entity.Connector{TargetConf: &entity.TargetConf{TargetVersionID: 92, IngressConf: &entity.TargetIngressConf{EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{FieldName: "global", FromField: "question"}}}}}},
		RunModeConfig: &entity.RunModeConfig{MaxRunMinutes: 7, MaxTurns: 4, SuaGoal: "private-goal"},
		EvalSetConfigs: []*entity.EvalSetConfig{
			{EvalSetID: 71, EvalSetVersionID: 71, TargetConfs: []*entity.ExptTargetConf{{TargetID: 999, TargetVersionID: 1000, RuntimeParam: map[string]string{"private-target": "first"}}}, EvaluatorConfs: []*entity.ExptEvaluatorConf{{EvaluatorVersionID: 111, Alias: "first", RuntimeParam: map[string]string{"private-evaluator": "one"}, ScoreWeight: gptr.Of(0.5)}}},
			{EvalSetID: 81, EvalSetVersionID: 82, SourceSpaceID: f.space + 50, EvaluatorConfs: []*entity.ExptEvaluatorConf{{EvaluatorVersionID: 222, Alias: "second", FilterMode: 1}}},
		},
	}
	encoded, err := json.Marshal(conf)
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumns(map[string]any{"lifecycle_hook_conf": raw, "eval_conf": encoded, "status": int32(entity.ExptStatus_Pending), "eval_set_source_type": 2, "eval_set_id": 71, "eval_set_version_id": 71, "trial_run_item_count": 1, "target_id": 91, "target_version_id": 92, "target_type": int32(entity.EvalTargetTypeSandboxAgent), "target_space_id": f.space + 90}).Error)
	f.manager.exptRepo = afterOnlyExperimentReader{sql: f.sql}
	f.manager.idgenerator = &preparerIDs{next: finalizationTestIDs.Add(1000) - 1000}
	f.manager.evaluationSetVersionService = &loaderVersions{value: &entity.EvaluationSetVersion{ID: 82, EvaluationSetID: 81, SpaceID: f.space + 50}}
	identity, err := hookinfra.NewIdentityProvider(nil, 0)
	require.NoError(t, err)
	m, err := NewExptManagerWithHooks(f.manager, ExptManagerHookDependencies{Initialization: exptinfra.NewHookRunInitializationRepo(f.p), Runs: f.repo, Configs: exptinfra.NewHookConfigRepo(f.p, codec), Codec: codec, Identity: identity, Runtime: &managerHookRuntime{admission: true}, Wake: new(managerHookWake), ExecutionScope: "local", SnapshotKeyID: "key"})
	require.NoError(t, err)
	f.manager = m.(*ExptMangerImpl)
	return f
}

func foundationCreate(t *testing.T, f *finalizationManagerFixture, mode entity.ExptRunMode) {
	t.Helper()
	ctx := context.Background()
	initial, err := f.manager.hooks.Initialization.ReadRunInitialization(ctx, f.key)
	require.NoError(t, err)
	attempted := false
	raw := "[801,802]"
	require.NoError(t, f.manager.initializeHookRun(ctx, &entity.ExptRunLog{ID: f.key.RunID, ExptRunID: f.key.RunID, SpaceID: f.space, ExptID: f.expt, Mode: int32(mode), Status: int64(entity.ExptStatus_Pending), CreatedBy: "original-user"}, initial, &attempted, &raw))
	require.True(t, attempted)
}

func TestHookExecutionSnapshotManagerCaptureReplayMySQL(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeTrialRun} {
		for _, stages := range [][2]bool{{true, false}, {false, true}, {true, true}} {
			t.Run(fmt.Sprintf("%d/%v", mode, stages), func(t *testing.T) {
				f := foundationManagerFixture(t, stages[0], stages[1])
				foundationCreate(t, f, mode)
				ctx := context.Background()
				stored := finalizationRead(t, f)
				var payload struct {
					Execution json.RawMessage `json:"execution"`
				}
				require.NoError(t, json.Unmarshal(stored.Snapshot.Cipher, &payload))
				require.NotEmpty(t, payload.Execution, "new multi-set Run must persist a private execution description")
				decoded, err := f.manager.hooks.Codec.DecodeSnapshot(ctx, f.key, "local", stored.Snapshot)
				require.NoError(t, err)
				execution := decoded.Input().Execution
				require.NotNil(t, execution)
				require.Equal(t, 1, execution.Version)
				require.Equal(t, int64(91), execution.Target.ID, "per-set target does not replace GLOBAL target")
				require.Len(t, execution.Sets, 2)
				require.Equal(t, int64(92), execution.Target.VersionID)
				require.Equal(t, entity.EvalTargetTypeSandboxAgent, execution.Target.Type)
				require.Equal(t, f.space+90, execution.Target.SourceSpaceID)
				require.Equal(t, "question", execution.Target.Config.IngressConf.EvalSetAdapter.FieldConfs[0].FromField)
				require.Equal(t, 7, execution.RunModeConfig.MaxRunMinutes)
				require.Equal(t, int64(71), execution.Sets[0].EvalSetID)
				require.Equal(t, int64(71), execution.Sets[0].EvalSetVersionID)
				require.Zero(t, execution.Sets[0].SourceSpaceID)
				require.Equal(t, int64(1000), execution.Sets[0].ItemConfig.EvalTargetConf.TargetVersionID)
				require.Equal(t, map[string]string{"private-target": "first"}, execution.Sets[0].ItemConfig.EvalTargetConf.DynamicConf)
				require.Equal(t, 7, execution.Sets[0].ItemConfig.EvalTargetConf.RunConf.MaxRunMinutes)
				require.Equal(t, int64(111), execution.Sets[0].ItemConfig.EvaluatorConfs[0].EvaluatorVersionID)
				require.Equal(t, "first", execution.Sets[0].ItemConfig.EvaluatorConfs[0].Alias)
				require.Equal(t, 0.5, *execution.Sets[0].ItemConfig.EvaluatorConfs[0].ScoreWeight)
				require.Equal(t, int64(81), execution.Sets[1].EvalSetID)
				require.Equal(t, int64(82), execution.Sets[1].EvalSetVersionID)
				require.Equal(t, f.space+50, execution.Sets[1].ItemConfig.EvalSetSourceSpaceID)
				require.Nil(t, execution.Sets[1].ItemConfig.EvalTargetConf)
				require.Equal(t, int64(222), execution.Sets[1].ItemConfig.EvaluatorConfs[0].EvaluatorVersionID)
				require.Nil(t, execution.Sets[1].ItemConfig.ExpectedQuotaConsumption)
				require.False(t, stored.PlanReady)
				var refs int64
				require.NoError(t, f.sql.Model(&model.ExptItemRef{}).Where("space_id=? AND expt_id=?", f.space, f.expt).Count(&refs).Error)
				require.Zero(t, refs, "foundation does not initialize ItemRefs")
				// A replay must not rebuild the descriptor from the current experiment.
				require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumns(map[string]any{"eval_conf": []byte(`{}`), "target_id": 777, "trial_run_item_count": 99}).Error)
				require.NoError(t, f.manager.LogRunWithPlanSeed(ctx, f.expt, f.key.RunID, mode, f.space, "[801,802]", &entity.Session{UserID: "original-user"}))
				if mode == entity.EvaluationModeTrialRun {
					require.Error(t, f.manager.LogRunWithPlanSeed(ctx, f.expt, f.key.RunID, mode, f.space, "[802]", &entity.Session{UserID: "original-user"}))
				}
				require.Error(t, f.manager.LogRunWithPlanSeed(ctx, f.expt, f.key.RunID, mode, f.space, "[801,802]", &entity.Session{UserID: "other-user"}))
				require.Equal(t, stored, finalizationRead(t, f))
				require.False(t, entity.HookExecutionInitializationRequired(true, mode, entity.ExptType_Offline, entity.ExptEvalSetSourceType_MultiSetConfig))
			})
		}
	}
}

func TestHookExecutionSnapshotCaptureExcludesLiveControls(t *testing.T) {
	f := foundationManagerFixture(t, true, false)
	expt, err := f.manager.exptRepo.GetByID(context.Background(), f.expt, f.space)
	require.NoError(t, err)
	log := &entity.ExptRunLog{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, Mode: int32(entity.EvaluationModeSubmit)}
	first, err := newHookExecutionSnapshot(expt, log, "local")
	require.NoError(t, err)
	before, err := json.Marshal(first)
	require.NoError(t, err)
	expt.Status, expt.LatestRunID, expt.TotalItemCount = entity.ExptStatus_Failed, 12345, 999
	expt.EvalConf.ItemConcurNum, expt.EvalConf.ItemRetryNum = gptr.Of(1), gptr.Of(9)
	expt.EvalConf.ExpectedQuotaConsumption = &entity.ExpectedQuotaConsumption{Resources: []*entity.ExpectedResourceConsumption{{Category: "concurrency", ResourceKey: "item", Amount: 3}}}
	after, err := newHookExecutionSnapshot(expt, log, "local")
	require.NoError(t, err)
	encoded, err := json.Marshal(after)
	require.NoError(t, err)
	require.Equal(t, before, encoded)
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeAppend} {
		log.Mode = int32(mode)
		d, err := newHookExecutionSnapshot(expt, log, "local")
		require.NoError(t, err)
		require.Nil(t, d)
	}
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeFailRetry, entity.EvaluationModeRetryAll, entity.EvaluationModeRetryItems} {
		log.Mode = int32(mode)
		d, err := newHookExecutionSnapshot(expt, log, "local")
		require.NoError(t, err)
		require.NotNil(t, d)
		require.Equal(t, mode, d.Mode)
	}
}

func TestHookExecutionSnapshotMissingNewMultiSetConfigurationFails(t *testing.T) {
	f := foundationManagerFixture(t, false, true)
	ctx := context.Background()
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumn("eval_conf", []byte(`{}`)).Error)
	initial, err := f.manager.hooks.Initialization.ReadRunInitialization(ctx, f.key)
	require.NoError(t, err)
	attempted := false
	err = f.manager.initializeHookRun(ctx, &entity.ExptRunLog{ID: f.key.RunID, ExptRunID: f.key.RunID, SpaceID: f.space, ExptID: f.expt, Mode: int32(entity.EvaluationModeSubmit), Status: int64(entity.ExptStatus_Pending), CreatedBy: "original-user"}, initial, &attempted, nil)
	require.Error(t, err)
	require.False(t, attempted)
	var count int64
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).Count(&count).Error)
	require.Zero(t, count)
}
