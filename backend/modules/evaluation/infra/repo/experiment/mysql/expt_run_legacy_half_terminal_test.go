// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package mysql

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

func TestLegacyHalfTerminalMySQLNaturalRechecksCanonical(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failed"}[failed], func(t *testing.T) {
			f := terminalFinishFixture(t)
			require.NoError(t, f.conn.Model(&model.ExptTurnResult{}).Where("space_id=?", f.space).Update("status", int32(entity.TurnRunState_Success)).Error)
			status := entity.ExptStatus_Success
			if failed {
				status = entity.ExptStatus_Failed
				require.NoError(t, f.conn.Model(&model.ExptTurnResult{}).Where("space_id=?", f.space).Update("status", int32(entity.TurnRunState_Fail)).Error)
				for _, table := range []string{"expt_item_result", "expt_item_result_run_log"} {
					require.NoError(t, f.conn.Table(table).Where("space_id=?", f.space).Update("status", int32(entity.ItemRunState_Fail)).Error)
				}
			}
			require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=?", f.space).Update("status", int64(status)).Error)
			result, err := finishFixture(f)
			require.NoError(t, err)
			require.Equal(t, entity.ExptRunFinishFinished, result.Disposition)
			require.Equal(t, status, result.Experiment.Status)
			require.NotNil(t, result.Experiment.EndAt)
			before := f.snapshot()
			result, err = finishFixture(f)
			require.NoError(t, err)
			require.Equal(t, entity.ExptRunFinishAlreadyFinished, result.Disposition)
			require.Equal(t, before, f.snapshot())
		})
	}
}

func TestLegacyHalfTerminalMySQLNaturalRejectsCorruptBindings(t *testing.T) {
	for _, fault := range []string{"run_status", "pending_experiment", "terminating_experiment", "terminated_run", "missing_log", "version", "canonical_status"} {
		t.Run(fault, func(t *testing.T) {
			f := terminalFinishFixture(t)
			require.NoError(t, f.conn.Model(&model.ExptTurnResult{}).Where("space_id=?", f.space).Update("status", int32(entity.TurnRunState_Success)).Error)
			require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=?", f.space).Update("status", int64(entity.ExptStatus_Success)).Error)
			switch fault {
			case "run_status":
				require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=?", f.space).Update("status", int64(entity.ExptStatus_Failed)).Error)
			case "pending_experiment":
				require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Update("status", int32(entity.ExptStatus_Pending)).Error)
			case "terminating_experiment":
				require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Update("status", int32(entity.ExptStatus_Terminating)).Error)
			case "terminated_run":
				require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=?", f.space).Update("status", int64(entity.ExptStatus_Terminated)).Error)
			case "missing_log":
				require.NoError(t, f.conn.Where("space_id=?", f.space).Delete(&model.ExptItemResultRunLog{}).Error)
			case "version":
				require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=?", f.space).Update("item_version_id", f.base+99999).Error)
			case "canonical_status":
				require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=?", f.space).Update("status", int32(entity.ItemRunState_Fail)).Error)
			}
			before := f.snapshot()
			result, err := finishFixture(f)
			require.Error(t, err)
			require.Nil(t, result)
			require.Equal(t, before, f.snapshot())
		})
	}
}

func TestLegacyHalfTerminalMySQLExplicitSourceAndLatestProtection(t *testing.T) {
	for _, fault := range []string{"none", "request", "references", "latest"} {
		t.Run(fault, func(t *testing.T) {
			f := newCurrentResultFixture(t, "9f6fe7ea-7db6-4d16-8f5b-2d1344556677")
			source := f.source()
			require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=?", f.space).Update("status", int64(entity.ExptStatus_Success)).Error)
			switch fault {
			case "request":
				f.token("aa6fe7ea-7db6-4d16-8f5b-2d1344556677")
			case "references":
				require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).Update("target_result_id", f.base+99999).Error)
			case "latest":
				require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Update("latest_run_id", f.run+100).Error)
			}
			before := f.snapshot()
			port := NewExptRunFinishDAO(f.provider).(repo.IExptRunTerminationRepo)
			result, err := port.TerminateCurrentRun(context.Background(), &entity.ExptRunTerminationRequest{SpaceID: f.space, ExptID: f.expt, RunID: f.run, Items: []*entity.ExptCurrentItemRunSource{source}, ExpectedSource: source, Finalize: true, Status: entity.ExptStatus_Failed})
			switch fault {
			case "none":
				require.NoError(t, err)
				require.True(t, result.Applied)
				require.Equal(t, entity.ExptStatus_Failed, result.Experiment.Status)
				var run model.ExptRunLog
				require.NoError(t, f.conn.Where("space_id=?", f.space).Take(&run).Error)
				require.Equal(t, int64(entity.ExptStatus_Failed), gptr.Indirect(run.Status))
			case "references":
				require.Error(t, err)
				require.Nil(t, result)
			default:
				require.NoError(t, err)
				require.True(t, result.NonCurrent)
			}
			if fault != "none" {
				require.Equal(t, before, f.snapshot())
			}
		})
	}
}
