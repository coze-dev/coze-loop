// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

type retryResetPort interface {
	ResetItemForRetry(context.Context, int64, int64, int64, *entity.ExptItemResultRunLog, []int64) (bool, error)
}

type retryResetFaultProvider struct {
	db.Provider
	operation string
	table     string
	failed    atomic.Bool
}

func (p *retryResetFaultProvider) Transaction(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
	return p.Provider.Transaction(ctx, func(tx *gorm.DB) error {
		inject := func(tx *gorm.DB) {
			if tx.Statement.Table == p.table && p.failed.CompareAndSwap(false, true) {
				_ = tx.AddError(errors.New("injected retry reset write failure"))
			}
		}
		switch p.operation {
		case "create":
			if err := tx.Callback().Create().Before("gorm:create").Register("retry_reset_fault_create", inject); err != nil {
				return err
			}
		case "update":
			if err := tx.Callback().Update().Before("gorm:update").Register("retry_reset_fault_update", inject); err != nil {
				return err
			}
		}
		return fc(tx)
	}, opts...)
}

func newRetryResetFixture(t *testing.T, operation, table string) (db.Provider, retryResetPort) {
	t.Helper()
	provider := db.NewTestDB(t, &model.Experiment{}, &model.ExptRunLog{}, &model.ExptItemResult{}, &model.ExptItemResultRunLog{}, &model.ExptTurnResult{}, &model.ExptTurnResultRunLog{}, &model.ExptStats{})
	ctx := context.Background()
	require.NoError(t, provider.NewSession(ctx).Create(&model.Experiment{ID: 7, SpaceID: 9, LatestRunID: 2, Status: int32(entity.ExptStatus_Success)}).Error)
	require.NoError(t, provider.NewSession(ctx).Create(&model.ExptRunLog{ID: 2, SpaceID: 9, ExptID: 7, ExptRunID: 2, Status: gptr.Of(int64(entity.ExptStatus_Processing))}).Error)
	require.NoError(t, provider.NewSession(ctx).Create(&model.ExptItemResult{ID: 1, SpaceID: 9, ExptID: 7, ExptRunID: 1, ItemID: 11, ItemVersionID: 101, Status: int32(entity.ItemRunState_Success)}).Error)
	require.NoError(t, provider.NewSession(ctx).Create(&model.ExptTurnResult{ID: 2, SpaceID: 9, ExptID: 7, ExptRunID: 1, ItemID: 11, ItemVersionID: 101, TurnID: 21, Status: int32(entity.TurnRunState_Success), TargetResultID: 31}).Error)
	require.NoError(t, provider.NewSession(ctx).Create(&model.ExptStats{ID: 3, SpaceID: 9, ExptID: 7, SuccessCnt: 1}).Error)
	if operation == "" {
		return provider, mustRetryResetPort(t, provider)
	}
	return provider, mustRetryResetPort(t, &retryResetFaultProvider{Provider: provider, operation: operation, table: table})
}

func mustRetryResetPort(t *testing.T, provider db.Provider) retryResetPort {
	t.Helper()
	port, ok := any(NewExptItemResultDAO(provider)).(retryResetPort)
	require.True(t, ok, "retry reset transaction port missing")
	return port
}

func retryResetRunLog() *entity.ExptItemResultRunLog {
	return &entity.ExptItemResultRunLog{ID: 4, SpaceID: 9, ExptID: 7, ExptRunID: 2, ItemID: 11, ItemVersionID: 101, Status: int32(entity.ItemRunState_Queueing)}
}

func requireRetryResetApplied(t *testing.T, provider db.Provider) {
	t.Helper()
	ctx := context.Background()
	var expt model.Experiment
	require.NoError(t, provider.NewSession(ctx).Where("space_id=9 AND id=7").Take(&expt).Error)
	require.Equal(t, int32(entity.ExptStatus_Processing), expt.Status)
	var item model.ExptItemResult
	require.NoError(t, provider.NewSession(ctx).Where("space_id=9 AND expt_id=7 AND item_id=11").Take(&item).Error)
	require.Equal(t, int64(2), item.ExptRunID)
	require.Equal(t, int32(entity.ItemRunState_Queueing), item.Status)
	var turn model.ExptTurnResult
	require.NoError(t, provider.NewSession(ctx).Where("space_id=9 AND expt_id=7 AND item_id=11 AND turn_id=21").Take(&turn).Error)
	require.Equal(t, int64(2), turn.ExptRunID)
	require.Equal(t, int32(entity.TurnRunState_Queueing), turn.Status)
	require.Zero(t, turn.TargetResultID)
	var runLog model.ExptItemResultRunLog
	require.NoError(t, provider.NewSession(ctx).Where("space_id=9 AND expt_id=7 AND expt_run_id=2 AND item_id=11").Take(&runLog).Error)
	require.Equal(t, int32(entity.ItemRunState_Queueing), runLog.Status)
	var stats model.ExptStats
	require.NoError(t, provider.NewSession(ctx).Where("space_id=9 AND expt_id=7").Take(&stats).Error)
	require.Equal(t, int32(1), stats.PendingCnt)
	require.Zero(t, stats.SuccessCnt)
}

func TestRetryResetWriteFailureRollsBackAndRedeliveryRecovers(t *testing.T) {
	for _, tc := range []struct {
		name      string
		operation string
		table     string
	}{
		{name: "turn", operation: "update", table: model.TableNameExptTurnResult},
		{name: "run_log", operation: "create", table: model.TableNameExptItemResultRunLog},
		{name: "stats", operation: "update", table: model.TableNameExptStats},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider, port := newRetryResetFixture(t, tc.operation, tc.table)
			ctx := context.Background()
			applied, err := port.ResetItemForRetry(ctx, 9, 7, 2, retryResetRunLog(), []int64{21})
			require.Error(t, err)
			require.False(t, applied)

			var item model.ExptItemResult
			require.NoError(t, provider.NewSession(ctx).Where("id=1").Take(&item).Error)
			require.Equal(t, int64(1), item.ExptRunID)
			require.Equal(t, int32(entity.ItemRunState_Success), item.Status)
			var turn model.ExptTurnResult
			require.NoError(t, provider.NewSession(ctx).Where("id=2").Take(&turn).Error)
			require.Equal(t, int64(1), turn.ExptRunID)
			require.Equal(t, int32(entity.TurnRunState_Success), turn.Status)
			require.Equal(t, int64(31), turn.TargetResultID)
			var runLogs int64
			require.NoError(t, provider.NewSession(ctx).Model(&model.ExptItemResultRunLog{}).Where("space_id=9 AND expt_id=7 AND expt_run_id=2").Count(&runLogs).Error)
			require.Zero(t, runLogs)
			var stats model.ExptStats
			require.NoError(t, provider.NewSession(ctx).Where("id=3").Take(&stats).Error)
			require.Zero(t, stats.PendingCnt)
			require.Equal(t, int32(1), stats.SuccessCnt)

			applied, err = port.ResetItemForRetry(ctx, 9, 7, 2, retryResetRunLog(), []int64{21})
			require.NoError(t, err)
			require.True(t, applied)
			requireRetryResetApplied(t, provider)

			applied, err = port.ResetItemForRetry(ctx, 9, 7, 2, retryResetRunLog(), []int64{21})
			require.NoError(t, err)
			require.False(t, applied)
			requireRetryResetApplied(t, provider)
		})
	}
}

func TestRetryResetRequiresCanonicalAndSkipsMaterializedItem(t *testing.T) {
	t.Run("canonical_missing", func(t *testing.T) {
		provider, port := newRetryResetFixture(t, "", "")
		ctx := context.Background()
		require.NoError(t, provider.NewSession(ctx).Unscoped().Where("id=1").Delete(&model.ExptItemResult{}).Error)
		applied, err := port.ResetItemForRetry(ctx, 9, 7, 2, retryResetRunLog(), []int64{21})
		require.Error(t, err)
		require.False(t, applied)
		var count int64
		require.NoError(t, provider.NewSession(ctx).Model(&model.ExptItemResultRunLog{}).Where("expt_run_id=2").Count(&count).Error)
		require.Zero(t, count)
	})

	t.Run("replaced_run_does_not_mutate_experiment_or_item", func(t *testing.T) {
		provider, port := newRetryResetFixture(t, "", "")
		ctx := context.Background()
		require.NoError(t, provider.NewSession(ctx).Model(&model.Experiment{}).Where("id=7").Update("latest_run_id", int64(3)).Error)
		applied, err := port.ResetItemForRetry(ctx, 9, 7, 2, retryResetRunLog(), []int64{21})
		require.Error(t, err)
		require.False(t, applied)
		var expt model.Experiment
		require.NoError(t, provider.NewSession(ctx).Where("id=7").Take(&expt).Error)
		require.Equal(t, int32(entity.ExptStatus_Success), expt.Status)
		var item model.ExptItemResult
		require.NoError(t, provider.NewSession(ctx).Where("id=1").Take(&item).Error)
		require.Equal(t, int64(1), item.ExptRunID)
		require.Equal(t, int32(entity.ItemRunState_Success), item.Status)
	})

	t.Run("terminated_run_skips_reset", func(t *testing.T) {
		provider, port := newRetryResetFixture(t, "", "")
		ctx := context.Background()
		require.NoError(t, provider.NewSession(ctx).Model(&model.ExptRunLog{}).Where("expt_run_id=2").Update("status", int64(entity.ExptStatus_Terminated)).Error)
		applied, err := port.ResetItemForRetry(ctx, 9, 7, 2, retryResetRunLog(), []int64{21})
		require.NoError(t, err)
		require.False(t, applied)
		var expt model.Experiment
		require.NoError(t, provider.NewSession(ctx).Where("id=7").Take(&expt).Error)
		require.Equal(t, int32(entity.ExptStatus_Success), expt.Status)
		var item model.ExptItemResult
		require.NoError(t, provider.NewSession(ctx).Where("id=1").Take(&item).Error)
		require.Equal(t, int64(1), item.ExptRunID)
		require.Equal(t, int32(entity.ItemRunState_Success), item.Status)
	})

	t.Run("canonical_current_run_still_backfills", func(t *testing.T) {
		provider, port := newRetryResetFixture(t, "", "")
		ctx := context.Background()
		require.NoError(t, provider.NewSession(ctx).Model(&model.ExptItemResult{}).Where("id=1").Updates(map[string]any{"expt_run_id": int64(2), "status": int32(entity.ItemRunState_Queueing)}).Error)
		require.NoError(t, provider.NewSession(ctx).Model(&model.ExptStats{}).Where("id=3").Updates(map[string]any{"pending_cnt": 1, "success_cnt": 0}).Error)
		applied, err := port.ResetItemForRetry(ctx, 9, 7, 2, retryResetRunLog(), []int64{21})
		require.NoError(t, err)
		require.True(t, applied)
		requireRetryResetApplied(t, provider)
	})

	t.Run("canonical_current_run_success_backfills_stats", func(t *testing.T) {
		provider, port := newRetryResetFixture(t, "", "")
		ctx := context.Background()
		require.NoError(t, provider.NewSession(ctx).Model(&model.ExptItemResult{}).Where("id=1").Update("expt_run_id", int64(2)).Error)
		applied, err := port.ResetItemForRetry(ctx, 9, 7, 2, retryResetRunLog(), []int64{21})
		require.NoError(t, err)
		require.True(t, applied)
		requireRetryResetApplied(t, provider)
	})

	t.Run("current_run_log_is_absorbing", func(t *testing.T) {
		provider, port := newRetryResetFixture(t, "", "")
		ctx := context.Background()
		require.NoError(t, provider.NewSession(ctx).Model(&model.ExptItemResult{}).Where("id=1").Updates(map[string]any{"expt_run_id": int64(2), "status": int32(entity.ItemRunState_Terminal)}).Error)
		require.NoError(t, provider.NewSession(ctx).Model(&model.ExptTurnResult{}).Where("id=2").Updates(map[string]any{"expt_run_id": int64(2), "status": int32(entity.TurnRunState_Terminal), "target_result_id": int64(41)}).Error)
		require.NoError(t, provider.NewSession(ctx).Create(&model.ExptItemResultRunLog{ID: 5, SpaceID: 9, ExptID: 7, ExptRunID: 2, ItemID: 11, ItemVersionID: 101, Status: int32(entity.ItemRunState_Terminal)}).Error)
		require.NoError(t, provider.NewSession(ctx).Model(&model.Experiment{}).Where("id=7").Update("status", int32(entity.ExptStatus_Terminating)).Error)
		applied, err := port.ResetItemForRetry(ctx, 9, 7, 2, retryResetRunLog(), []int64{21})
		require.NoError(t, err)
		require.False(t, applied)
		var expt model.Experiment
		require.NoError(t, provider.NewSession(ctx).Where("id=7").Take(&expt).Error)
		require.Equal(t, int32(entity.ExptStatus_Terminating), expt.Status)
		var item model.ExptItemResult
		require.NoError(t, provider.NewSession(ctx).Where("id=1").Take(&item).Error)
		require.Equal(t, int32(entity.ItemRunState_Terminal), item.Status)
		var turn model.ExptTurnResult
		require.NoError(t, provider.NewSession(ctx).Where("id=2").Take(&turn).Error)
		require.Equal(t, int32(entity.TurnRunState_Terminal), turn.Status)
		require.Equal(t, int64(41), turn.TargetResultID)
		var stats model.ExptStats
		require.NoError(t, provider.NewSession(ctx).Where("id=3").Take(&stats).Error)
		require.Zero(t, stats.PendingCnt)
		require.Equal(t, int32(1), stats.SuccessCnt)
	})
}

func TestMarkRetryProcessingGuardsRunAndTerminating(t *testing.T) {
	for _, tc := range []struct {
		name           string
		latestRunID    int64
		status         entity.ExptStatus
		runStatus      entity.ExptStatus
		wantProcessing bool
		wantStatus     entity.ExptStatus
		wantErr        bool
	}{
		{name: "current_finished", latestRunID: 2, status: entity.ExptStatus_Success, wantProcessing: true, wantStatus: entity.ExptStatus_Processing},
		{name: "replaced", latestRunID: 3, status: entity.ExptStatus_Success, wantStatus: entity.ExptStatus_Success, wantErr: true},
		{name: "terminating", latestRunID: 2, status: entity.ExptStatus_Terminating, wantStatus: entity.ExptStatus_Terminating},
		{name: "terminated_run", latestRunID: 2, status: entity.ExptStatus_Success, runStatus: entity.ExptStatus_Terminated, wantStatus: entity.ExptStatus_Success},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider, _ := newRetryResetFixture(t, "", "")
			ctx := context.Background()
			require.NoError(t, provider.NewSession(ctx).Model(&model.Experiment{}).Where("id=7").Updates(map[string]any{"latest_run_id": tc.latestRunID, "status": int32(tc.status)}).Error)
			runStatus := tc.runStatus
			if runStatus == 0 {
				runStatus = entity.ExptStatus_Processing
			}
			require.NoError(t, provider.NewSession(ctx).Model(&model.ExptRunLog{}).Where("expt_run_id=2").Update("status", int64(runStatus)).Error)
			marker, ok := any(NewExptDAO(provider)).(IExptRetryRunStatusDAO)
			require.True(t, ok)
			processing, err := marker.MarkRetryProcessing(ctx, 9, 7, 2)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.wantProcessing, processing)
			var expt model.Experiment
			require.NoError(t, provider.NewSession(ctx).Where("id=7").Take(&expt).Error)
			require.Equal(t, int32(tc.wantStatus), expt.Status)
		})
	}
}
