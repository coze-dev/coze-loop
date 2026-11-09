// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

// Replays the production legacy shape end to end: no resume identity in the
// event, zero item versions in every old row, then a duplicate delivery.
func TestLegacyAsyncEventReplayWithZeroVersions(t *testing.T) {
	f := newLateWriteFixture(t, "")
	for _, table := range []string{"expt_item_result", "expt_item_result_run_log", "expt_turn_result", "expt_turn_result_run_log", "expt_item_ref", "eval_target_record", "evaluator_record"} {
		require.NoError(t, f.conn.Table(table).Where("space_id=?", f.space).Update("item_version_id", 0).Error)
	}
	event := f.event()
	event.RequestID = ""
	event.ExptRunMode = entity.EvaluationModeSubmit
	event.AsyncReportTrigger = true
	event.AsyncEvaluatorReportTrigger = false
	event.ResumeSource = nil
	event.RetryTimes = 0
	event.MaxRetryTimes = 1

	svc := &ExptItemEventEvalServiceImpl{exptItemResultRepo: f.itemRepo(), exptTurnResultRepo: f.turnRepo()}
	executor := &ExptItemEvalCtxExecutor{ItemResultRepo: f.itemRepo(), TurnResultRepo: f.turnRepo()}
	consume := svc.HandleCurrentSource(func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		frozen := executionSource(ctx).Source
		for _, turnID := range []int64{0, f.base + 22} {
			etec := &entity.ExptTurnEvalCtx{Turn: &entity.Turn{ID: turnID}, ExptItemEvalCtx: &entity.ExptItemEvalCtx{Event: event, Expt: &entity.Experiment{ID: f.expt, SpaceID: f.space}, EvalSetItem: &entity.EvaluationSetItem{ItemID: f.item}, ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: frozen.Turns}}}
			if err := executor.storeTurnRunResult(ctx, etec, &entity.ExptTurnRunResult{}); err != nil {
				return err
			}
		}
		return executor.CompleteItemRun(ctx, &entity.ExptItemEvalCtx{Event: event}, nil)
	})

	require.NoError(t, consume(context.Background(), event))
	var log model.ExptItemResultRunLog
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, f.item).Take(&log).Error)
	require.Zero(t, log.ItemVersionID)
	require.Equal(t, int32(entity.ItemRunState_Success), log.Status)
	require.Equal(t, int32(entity.ExptItemResultStateLogged), gptr.Indirect(log.ResultState))
	var targetRows, evaluatorRows int64
	require.NoError(t, f.conn.Table("eval_target_record").Where("space_id=? AND item_id=?", f.space, f.item).Count(&targetRows).Error)
	require.NoError(t, f.conn.Table("evaluator_record").Where("space_id=? AND item_id=?", f.space, f.item).Count(&evaluatorRows).Error)
	require.Equal(t, int64(1), targetRows)
	require.Equal(t, int64(1), evaluatorRows)
	afterFirst := f.snapshot()

	err := consume(context.Background(), event)
	require.False(t, errno.NeedMQRetry(err), "duplicate legacy delivery must not enter the MQ retry loop: %v", err)
	require.Equal(t, afterFirst, f.snapshot(), "duplicate delivery must not rewrite results or create duplicate records")
}
