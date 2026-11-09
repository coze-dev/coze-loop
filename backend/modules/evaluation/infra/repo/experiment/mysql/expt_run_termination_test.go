// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package mysql

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm/clause"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRunCancelMySQLRollbackEveryWrite(t *testing.T) {
	for _, table := range []string{"expt_turn_result_run_log", "expt_turn_result", "expt_item_result_run_log", "expt_item_result", "expt_stats", "expt_run_log", "experiment", "commit"} {
		t.Run(table, func(t *testing.T) {
			f := newCurrentResultFixture(t, "9f6fe7ea-7db6-4d16-8f5b-2d1344556677")
			source := f.source()
			before := f.snapshot()
			provider := f.provider
			if table == "commit" {
				provider = finishFaultProvider{provider}
			} else {
				require.NoError(t, f.conn.Callback().Update().Before("gorm:update").Register("termination_fault", func(tx *gorm.DB) {
					if tx.Statement.Table == table {
						_ = tx.AddError(errors.New("termination write fault"))
					}
				}))
			}
			port := NewExptRunFinishDAO(provider).(repo.IExptRunTerminationRepo)
			result, err := port.TerminateCurrentRun(context.Background(), &entity.ExptRunTerminationRequest{SpaceID: f.space, ExptID: f.expt, RunID: f.run, Items: []*entity.ExptCurrentItemRunSource{source}, Finalize: true, Status: entity.ExptStatus_Terminated})
			require.Error(t, err)
			require.Nil(t, result)
			require.Equal(t, before, f.snapshot())
		})
	}
}

func TestTerminateSourceOldRequestCannotEndSameRun(t *testing.T) {
	f := newCurrentResultFixture(t, "9f6fe7ea-7db6-4d16-8f5b-2d1344556677")
	source := f.source()
	f.token("aa6fe7ea-7db6-4d16-8f5b-2d1344556677")
	before := f.snapshot()
	port := NewExptRunFinishDAO(f.provider).(repo.IExptRunTerminationRepo)
	for _, whole := range []bool{false, true} {
		result, err := port.TerminateCurrentRun(context.Background(), &entity.ExptRunTerminationRequest{SpaceID: f.space, ExptID: f.expt, RunID: f.run, Items: []*entity.ExptCurrentItemRunSource{source}, ExpectedSource: source, Finalize: whole, Status: entity.ExptStatus_Terminated})
		require.NoError(t, err)
		require.True(t, result.NonCurrent)
		require.Equal(t, before, f.snapshot())
	}
}

func TestRunCancelExecutionWriterRemainsFenced(t *testing.T) {
	f := newCurrentResultFixture(t, "")
	source := f.source()
	port := NewExptRunFinishDAO(f.provider).(repo.IExptRunTerminationRepo)
	_, err := port.TerminateCurrentRun(context.Background(), &entity.ExptRunTerminationRequest{SpaceID: f.space, ExptID: f.expt, RunID: f.run, Begin: true})
	require.NoError(t, err)
	before := f.snapshot()
	applied, err := f.port.CompleteCurrentItemRun(context.Background(), source, &entity.ExptCurrentItemRunWrite{Status: entity.ItemRunState_Success})
	require.NoError(t, err)
	require.False(t, applied)
	require.Equal(t, before, f.snapshot())
	result, err := port.TerminateCurrentRun(context.Background(), &entity.ExptRunTerminationRequest{SpaceID: f.space, ExptID: f.expt, RunID: f.run, Items: []*entity.ExptCurrentItemRunSource{source}, Finalize: true, Status: entity.ExptStatus_Terminated})
	require.NoError(t, err)
	require.True(t, result.Applied)
}

func TestTerminateRawZeroVersionAndMissingLogs(t *testing.T) {
	f := newCurrentResultFixture(t, "")
	for _, table := range []string{"expt_item_result", "expt_item_result_run_log", "expt_turn_result", "expt_turn_result_run_log", "expt_item_ref"} {
		require.NoError(t, f.conn.Table(table).Where("space_id=?", f.space).Update("item_version_id", 0).Error)
	}
	require.NoError(t, f.conn.Model(&model.ExptTurnResult{}).Where("space_id=?", f.space).Update("target_result_id", 0).Error)
	require.NoError(t, f.conn.Exec("DELETE FROM expt_turn_result_run_log WHERE space_id=? AND expt_id=?", f.space, f.expt).Error)
	source := f.source()
	require.Zero(t, source.Current.ItemVersionID)
	missing := []*entity.ExptTurnResultRunLog{}
	for i, turn := range source.CanonicalTurns {
		missing = append(missing, &entity.ExptTurnResultRunLog{ID: f.base + 9000 + int64(i), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: f.item, ItemVersionID: 0, TurnID: turn.TurnID, Status: entity.TurnRunState_Terminal})
	}
	port := NewExptRunFinishDAO(f.provider).(repo.IExptRunTerminationRepo)
	result, err := port.TerminateCurrentRun(context.Background(), &entity.ExptRunTerminationRequest{SpaceID: f.space, ExptID: f.expt, RunID: f.run, Items: []*entity.ExptCurrentItemRunSource{source}, MissingTurnLogs: missing})
	require.NoError(t, err)
	require.True(t, result.Applied)
	var item model.ExptItemResultRunLog
	require.NoError(t, f.conn.Where("space_id=? AND item_id=?", f.space, f.item).Take(&item).Error)
	require.Equal(t, int32(entity.ItemRunState_Terminal), item.Status)
	require.Equal(t, int32(entity.ExptItemResultStateLogged), gptr.Indirect(item.ResultState))
}

func TestNewRunResetRemovesOnlyManualContextAtMigration(t *testing.T) {
	f := newCurrentResultFixture(t, "9f6fe7ea-7db6-4d16-8f5b-2d1344556677")
	replacement := f.run + 100
	require.NoError(t, f.conn.Create(&model.ExptRunLog{ID: replacement, SpaceID: f.space, ExptID: f.expt, ExptRunID: replacement, Status: gptr.Of(int64(entity.ExptStatus_Pending)), Mode: gptr.Of(int32(entity.EvaluationModeRetryItems))}).Error)
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Update("latest_run_id", replacement).Error)
	before := f.snapshot()
	require.NoError(t, NewExptItemResultDAO(f.provider).UpdateItemsResult(context.Background(), f.space, f.expt, []int64{f.item}, map[string]any{"expt_run_id": replacement, "status": int32(entity.ItemRunState_Queueing)}))
	var item model.ExptItemResult
	require.NoError(t, f.conn.Where("space_id=? AND item_id=?", f.space, f.item).Take(&item).Error)
	ext, err := entity.ParseExptItemResultExt(gptr.Indirect(item.Ext))
	require.NoError(t, err)
	require.Equal(t, map[string]string{"other": "keep"}, ext)
	after := f.snapshot()
	require.Equal(t, before["expt_run_log"], after["expt_run_log"])
	require.Equal(t, before["expt_item_result_run_log"], after["expt_item_result_run_log"])
}

func TestTerminateRowPreservesOtherItemStatsBuckets(t *testing.T) {
	f := newCurrentResultFixture(t, "")
	source := f.source()
	require.NoError(t, f.conn.Model(&model.ExptStats{}).Where("space_id=?", f.space).Updates(map[string]any{"pending_cnt": 7, "processing_cnt": 9, "success_cnt": 11, "fail_cnt": 13, "terminated_cnt": 17}).Error)
	_, err := NewExptRunFinishDAO(f.provider).(repo.IExptRunTerminationRepo).TerminateCurrentRun(context.Background(), &entity.ExptRunTerminationRequest{SpaceID: f.space, ExptID: f.expt, RunID: f.run, Items: []*entity.ExptCurrentItemRunSource{source}})
	require.NoError(t, err)
	var stats model.ExptStats
	require.NoError(t, f.conn.Where("space_id=?", f.space).Take(&stats).Error)
	require.Equal(t, int32(7), stats.PendingCnt)
	require.Equal(t, int32(8), stats.ProcessingCnt)
	require.Equal(t, int32(11), stats.SuccessCnt)
	require.Equal(t, int32(13), stats.FailCnt)
	require.Equal(t, int32(18), stats.TerminatedCnt)
}

func TestRunCancelNoCompleteTurnsKeepsCanonicalProjection(t *testing.T) {
	f := newCurrentResultFixture(t, "")
	source := f.source()
	before := f.snapshot()
	_, err := NewExptRunFinishDAO(f.provider).(repo.IExptRunTerminationRepo).TerminateCurrentRun(context.Background(), &entity.ExptRunTerminationRequest{SpaceID: f.space, ExptID: f.expt, RunID: f.run, Items: []*entity.ExptCurrentItemRunSource{source}, Finalize: true, Status: entity.ExptStatus_Failed, NoCompleteItemTurn: true})
	require.NoError(t, err)
	after := f.snapshot()
	require.Equal(t, before["expt_turn_result"], after["expt_turn_result"])
	require.Equal(t, before["expt_item_result"], after["expt_item_result"])
}

func TestTerminateSameRequestRefAdvanceMustRetry(t *testing.T) {
	f := newCurrentResultFixture(t, "aa6fe7ea-7db6-4d16-8f5b-2d1344556677")
	source := f.source()
	require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).Update("target_result_id", f.base+777).Error)
	before := f.snapshot()
	_, err := NewExptRunFinishDAO(f.provider).(repo.IExptRunTerminationRepo).TerminateCurrentRun(context.Background(), &entity.ExptRunTerminationRequest{SpaceID: f.space, ExptID: f.expt, RunID: f.run, Items: []*entity.ExptCurrentItemRunSource{source}, Finalize: true, Status: entity.ExptStatus_Terminated})
	require.Error(t, err, "same execution progressed while cancellation waited; retain retry")
	require.Equal(t, before, f.snapshot())
}

func TestRunCancelBeginSerializesInflightCanonicalWriter(t *testing.T) {
	f := newCurrentResultFixture(t, "")
	held := make(chan struct{})
	release := make(chan struct{})
	writer := make(chan error, 1)
	go func() {
		writer <- f.provider.Transaction(context.Background(), func(tx *gorm.DB) error {
			var item model.ExptItemResult
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("space_id=? AND item_id=?", f.space, f.item).Take(&item).Error; err != nil {
				return err
			}
			close(held)
			<-release
			return nil
		})
	}()
	<-held
	done := make(chan error, 1)
	go func() {
		_, err := NewExptRunFinishDAO(f.provider).(repo.IExptRunTerminationRepo).TerminateCurrentRun(context.Background(), &entity.ExptRunTerminationRequest{SpaceID: f.space, ExptID: f.expt, RunID: f.run, Begin: true})
		done <- err
	}()
	select {
	case err := <-done:
		close(release)
		require.NoError(t, <-writer)
		require.Fail(t, "cancel begin committed before inflight canonical writer completed", err)
		return
	case <-time.After(150 * time.Millisecond):
	}
	close(release)
	require.NoError(t, <-writer)
	require.NoError(t, <-done)
}
