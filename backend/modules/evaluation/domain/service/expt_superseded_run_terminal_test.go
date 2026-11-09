// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func TestSupersededRunStillReceivesTerminalState(t *testing.T) {
	p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
	p.publishSuccess(t, false)
	p.start.resultSvc = p.manager.exptResultService
	ctx := context.Background()
	oldRun := p.f.run
	replacement := oldRun + 1
	require.NoError(t, p.f.conn.Model(&model.ExptRunLog{}).
		Where("space_id=? AND expt_id=? AND expt_run_id=?", p.f.space, p.f.expt, oldRun).
		Updates(map[string]any{"status": int64(entity.ExptStatus_Processing), "success_cnt": 1}).Error)
	require.NoError(t, p.f.conn.Model(&model.ExptTurnResult{}).
		Where("space_id=? AND expt_id=?", p.f.space, p.f.expt).
		Updates(map[string]any{"expt_run_id": replacement, "status": int32(entity.TurnRunState_Queueing)}).Error)
	require.NoError(t, p.f.conn.Model(&model.Experiment{}).
		Where("space_id=? AND id=?", p.f.space, p.f.expt).
		Update("latest_run_id", replacement).Error)
	beforeRun, err := p.manager.runLogRepo.Get(ctx, p.f.expt, oldRun)
	require.NoError(t, err)

	nextTick, err := p.start.ExptEnd(ctx, p.event, p.expt, 0, 0)
	require.NoError(t, err)
	require.False(t, nextTick)
	marked, err := p.manager.idem.Exist(ctx, makeEndIdemKey(p.event))
	require.NoError(t, err)
	require.True(t, marked, "completion is marked finished, so no later tick will repair R1")

	run, err := p.manager.runLogRepo.Get(ctx, p.f.expt, oldRun)
	require.NoError(t, err)
	t.Logf("SUPERSEDED_RUN_FINAL_STATE status=%d next_tick=%v end_marked=%v", run.Status, nextTick, marked)
	require.Equal(t, int64(entity.ExptStatus_Success), run.Status,
		"R1 must still receive its own terminal state after ownership protection rejects current-run writes")
	require.Equal(t, beforeRun.PendingCnt, run.PendingCnt)
	require.Equal(t, beforeRun.ProcessingCnt, run.ProcessingCnt)
	require.Equal(t, beforeRun.SuccessCnt, run.SuccessCnt)
	require.Equal(t, beforeRun.FailCnt, run.FailCnt)
	require.Equal(t, beforeRun.TerminatedCnt, run.TerminatedCnt)
	current, err := p.manager.exptRepo.GetByID(ctx, p.f.expt, p.f.space)
	require.NoError(t, err)
	require.Equal(t, replacement, current.LatestRunID)
}

func TestSupersededTerminatingRunStillReceivesTerminalState(t *testing.T) {
	p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
	p.publishSuccess(t, false)
	ctx := context.Background()
	oldRun := p.f.run
	replacement := oldRun + 1
	require.NoError(t, p.f.conn.Model(&model.ExptRunLog{}).
		Where("space_id=? AND expt_id=? AND expt_run_id=?", p.f.space, p.f.expt, oldRun).
		Update("status", int64(entity.ExptStatus_Terminating)).Error)
	require.NoError(t, p.f.conn.Model(&model.ExptTurnResult{}).
		Where("space_id=? AND expt_id=?", p.f.space, p.f.expt).
		Updates(map[string]any{"expt_run_id": replacement, "status": int32(entity.TurnRunState_Queueing)}).Error)
	require.NoError(t, p.f.conn.Model(&model.Experiment{}).
		Where("space_id=? AND id=?", p.f.space, p.f.expt).
		Updates(map[string]any{"latest_run_id": replacement, "status": int32(entity.ExptStatus_Processing)}).Error)
	beforeRun, err := p.manager.runLogRepo.Get(ctx, p.f.expt, oldRun)
	require.NoError(t, err)

	applied, err := p.manager.TerminateCurrentRun(ctx, p.f.expt, oldRun, p.f.space, p.event.Session, nil, entity.WithStatus(entity.ExptStatus_Terminated))
	require.NoError(t, err)
	require.False(t, applied)

	run, err := p.manager.runLogRepo.Get(ctx, p.f.expt, oldRun)
	require.NoError(t, err)
	t.Logf("SUPERSEDED_TERMINATING_FINAL_STATE status=%d", run.Status)
	require.Equal(t, int64(entity.ExptStatus_Terminated), run.Status)
	require.Equal(t, beforeRun.PendingCnt, run.PendingCnt)
	require.Equal(t, beforeRun.ProcessingCnt, run.ProcessingCnt)
	require.Equal(t, beforeRun.SuccessCnt, run.SuccessCnt)
	require.Equal(t, beforeRun.FailCnt, run.FailCnt)
	require.Equal(t, beforeRun.TerminatedCnt, run.TerminatedCnt)
	current, err := p.manager.exptRepo.GetByID(ctx, p.f.expt, p.f.space)
	require.NoError(t, err)
	require.Equal(t, replacement, current.LatestRunID)
}
