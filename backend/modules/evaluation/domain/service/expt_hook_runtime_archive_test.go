// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

// Execution records are fixtures here; archival, score calculation and finalization are real.
func boundRuntimeArchiveFixture(t *testing.T, badAlias bool) (*boundInitFixture, entity.HookExecutionManifest, ExptResultService) {
	t.Helper()
	f, _ := boundConsumerFixtureConfig(t, func(conf *entity.EvaluationConfiguration) {
		conf.ConnectorConf.EvaluatorsConf.EnableScoreWeight = true
		conf.EvalSetConfigs[1].EvaluatorConfs[0].ScoreWeight = gptr.Of(1.0)
		conf.EvalSetConfigs[1].EvaluatorConfs = append(conf.EvalSetConfigs[1].EvaluatorConfs, &entity.ExptEvaluatorConf{EvaluatorVersionID: 111, Alias: "other", ScoreWeight: gptr.Of(3.0)})
	})
	bindRuntimeFinalization(t, f)
	ctx := context.Background()
	run := finalizationRead(t, f.finalizationManagerFixture)
	_, err := f.repo.AdmitItem(ctx, entity.HookAdmitItemInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: run.Version}, ItemID: f.items[0].ItemID})
	require.NoError(t, err)
	var row model.ExptLifecycleRunItem
	require.NoError(t, f.sql.Where("id=?", f.items[0].ID).First(&row).Error)
	var m entity.HookExecutionManifest
	require.NoError(t, json.Unmarshal(gptr.Indirect(row.ExecutionManifest), &m))
	m.TurnLogsInitialized = gptr.Of(true)
	m.Turns[0].RunLogID = finalizationTestIDs.Add(1)
	refs := &entity.EvaluatorResults{}
	reader := &boundRuntimeRecordReader{records: map[int64]*entity.EvaluatorRecord{}}
	for i, version := range []int64{222, 111} {
		alias, score := "second", 1.0
		if i == 1 {
			alias, score = "other", 0
		}
		if badAlias && i == 1 {
			alias = "not-in-original-item"
		}
		id := finalizationTestIDs.Add(1)
		reader.records[id] = &entity.EvaluatorRecord{ID: id, SpaceID: f.space, ExperimentID: f.expt, ExperimentRunID: f.key.RunID, ItemID: m.Frozen.ItemID, ItemVersionID: m.Frozen.ItemVersionID, TurnID: m.Turns[0].TurnID, EvaluatorVersionID: version, Alias: alias, SourceType: entity.EvaluatorRecordSourceTypeBuiltin, Status: entity.EvaluatorRunStatusSuccess, EvaluatorOutputData: &entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: gptr.Of(score)}}}
		refs.Registered = append(refs.Registered, &entity.RegisteredEvalResult{VersionID: version, Alias: alias, RecordID: id})
	}
	t.Cleanup(func() {
		require.NoError(t, f.sql.Unscoped().Where("space_id=? AND expt_id=?", f.space, f.expt).Delete(&model.ExptTurnEvaluatorResultRef{}).Error)
	})
	raw, err := json.Marshal(refs)
	require.NoError(t, err)
	require.NoError(t, f.sql.Create(&model.ExptTurnResultRunLog{ID: m.Turns[0].RunLogID, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: m.Frozen.ItemID, ItemVersionID: m.Frozen.ItemVersionID, TurnID: m.Turns[0].TurnID, Status: int32(entity.TurnRunState_Success), EvaluatorResultIds: &raw}).Error)
	raw, err = json.Marshal(m)
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRunItem{}).Where("id=?", row.ID).UpdateColumn("execution_manifest", raw).Error)
	require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("id=?", m.ItemRunLogID).UpdateColumns(map[string]any{"status": int32(entity.ItemRunState_Success), "result_state": int32(entity.ExptItemResultStateLogged)}).Error)
	result := &ExptResultServiceImpl{idgen: activeReferenceIDs{}, evaluatorRecordService: reader, scoreCalculator: NewEvaluatorScoreCalculator(nil, nil)}
	svc, err := result.WithHookArchive(f.manager.finalization.Repository.(repo.IHookItemArchiveRepo), "local")
	require.NoError(t, err)
	return f, m, svc
}

type boundRuntimeRecordReader struct {
	EvaluatorRecordService
	records map[int64]*entity.EvaluatorRecord
}

func (r *boundRuntimeRecordReader) BatchGetEvaluatorRecord(_ context.Context, ids []int64, deleted, full bool) ([]*entity.EvaluatorRecord, error) {
	if deleted || full {
		return nil, entity.ErrHookStoreCorrupt
	}
	var records []*entity.EvaluatorRecord
	for _, id := range ids {
		if r.records[id] == nil {
			return nil, entity.ErrHookStoreMissing
		}
		records = append(records, r.records[id])
	}
	return records, nil
}

func TestHookBoundRuntimeNormalArchiveUsesFrozenWeightsMySQL(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[failed], func(t *testing.T) {
			f, m, result := boundRuntimeArchiveFixture(t, false)
			if failed {
				require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("id=?", m.Turns[0].RunLogID).UpdateColumn("status", int32(entity.TurnRunState_Fail)).Error)
				require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("id=?", m.ItemRunLogID).UpdateColumn("status", int32(entity.ItemRunState_Fail)).Error)
			}
			require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumn("eval_conf", []byte(`{}`)).Error)
			ctx := context.Background()
			refs, err := result.RecordItemRunLogs(ctx, f.expt, f.key.RunID, m.Frozen.ItemID, f.space, &entity.Experiment{ID: f.expt, SpaceID: f.space})
			require.NoError(t, err)
			require.Len(t, refs, 2)
			var turn model.ExptTurnResult
			require.NoError(t, f.sql.First(&turn, m.Turns[0].ResultID).Error)
			require.Equal(t, 0.25, gptr.Indirect(turn.WeightedScore))
			require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
			run := finalizationRead(t, f.finalizationManagerFixture)
			if failed {
				require.Equal(t, entity.ExptStatus_Failed, run.State.Intent.Status)
			} else {
				require.Equal(t, entity.ExptStatus_Success, run.State.Intent.Status)
			}
			require.True(t, run.State.After.Activated)
		})
	}
}

func TestHookBoundRuntimeArchiveRejectsUnboundEvaluatorMySQL(t *testing.T) {
	f, m, result := boundRuntimeArchiveFixture(t, true)
	_, err := result.RecordItemRunLogs(context.Background(), f.expt, f.key.RunID, m.Frozen.ItemID, f.space, &entity.Experiment{ID: f.expt, SpaceID: f.space})
	require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
	var refs int64
	require.NoError(t, f.sql.Model(&model.ExptTurnEvaluatorResultRef{}).Where("space_id=? AND expt_id=?", f.space, f.expt).Count(&refs).Error)
	require.Zero(t, refs)
}
