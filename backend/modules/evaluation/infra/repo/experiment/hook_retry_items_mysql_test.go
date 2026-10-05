// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type retryItemsPageWriter interface {
	AppendPreparedRetryItemsPage(context.Context, entity.HookRunKey, string, string, []entity.HookExecutionManifest) (bool, error)
	ReadRetryItemsSources(context.Context, entity.HookRunKey, string, string, []entity.HookPlanItem) (*entity.HookExecutionInitializationPage, error)
}

func TestHookRetryItemsSourceFailureBudgetMySQL(t *testing.T) {
	f, _, _, _ := retryItemsBoundFixture(t)
	w, ok := f.repo.(interface {
		RecordRetryItemsSourceFailure(context.Context, entity.HookRunKey, string, string) (bool, error)
	})
	require.True(t, ok, "tail source retry accounting must survive worker restarts")
	for i := 0; i < 11; i++ {
		run, err := f.repo.GetRun(context.Background(), f.key)
		require.NoError(t, err)
		exhausted, err := w.RecordRetryItemsSourceFailure(context.Background(), f.key, "local", run.PlanCursor)
		require.NoError(t, err)
		require.Equal(t, i == 10, exhausted)
	}
	run, err := f.repo.GetRun(context.Background(), f.key)
	require.NoError(t, err)
	var c entity.HookRetryItemsCursor
	require.NoError(t, json.Unmarshal([]byte(run.PlanCursor), &c))
	require.Equal(t, 10, c.Retries)
	require.Equal(t, 1, c.Batch)
	require.Equal(t, int64(1), run.PlanCount)
}

func TestHookRetryItemsLargeAcceptedBatchMySQL(t *testing.T) {
	f, w, _, cursor := retryItemsBoundFixture(t)
	ctx := context.Background()
	r := f.repo.(*hookRunRepo)
	ids := make([]int64, 201)
	for i := range ids {
		ids[i] = 999001 + int64(i)
		if i == 0 {
			continue
		}
		require.NoError(t, f.sql.Create(&model.ExptItemResult{ID: hookTxSequence.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: r.executionBinding.source.SourceRunID, ItemID: ids[i], ItemIdx: gptr.Of(int32(i)), Status: int32(entity.ItemRunState_Fail)}).Error)
		require.NoError(t, f.sql.Create(&model.ExptTurnResult{ID: hookTxSequence.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: r.executionBinding.source.SourceRunID, ItemID: ids[i], TurnIdx: gptr.Of(int32(0)), Status: int32(entity.TurnRunState_Fail)}).Error)
	}
	logs := []entity.ExptRunLogItems{{ItemIDs: []int64{f.manifests[0].Frozen.ItemID}}, {ItemIDs: ids}}
	raw, err := json.Marshal(logs)
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", f.key.RunID).UpdateColumn("item_ids", raw).Error)
	require.NoError(t, f.sql.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", f.space, f.expt).UpdateColumn("fail_cnt", 201).Error)
	for start := 0; start < len(ids); start += 100 {
		var items []entity.HookPlanItem
		for _, id := range ids[start:min(start+100, len(ids))] {
			items = append(items, entity.HookPlanItem{ID: hookTxSequence.Add(1), SourceSpaceID: f.space, EvalSetID: 71, ItemID: id})
		}
		page, err := w.ReadRetryItemsSources(ctx, f.key, "local", cursor, items)
		require.NoError(t, err)
		var manifests []entity.HookExecutionManifest
		for _, item := range page.Items {
			m := item.Reuse.Clone()
			m.ItemRunLogID = hookTxSequence.Add(1)
			manifests = append(manifests, m)
		}
		changed, err := w.AppendPreparedRetryItemsPage(ctx, f.key, "local", cursor, manifests)
		require.NoError(t, err)
		require.True(t, changed)
		run, err := f.repo.GetRun(ctx, f.key)
		require.NoError(t, err)
		cursor = run.PlanCursor
		c, err := entity.DecodeHookRetryItemsCursor(cursor, f.key, logs, run.PlanCount, run.PlanHash)
		require.NoError(t, err)
		require.Equal(t, int64(1+min(start+100, len(ids))), run.PlanCount)
		if start < 200 {
			require.Equal(t, 1, c.Batch)
			require.Equal(t, start+100, c.Offset)
		} else {
			require.Equal(t, 2, c.Batch)
			require.Zero(t, c.Offset)
		}
	}
	var stats model.ExptStats
	require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&stats).Error)
	require.Equal(t, int32(202), stats.PendingCnt)
	require.Zero(t, stats.FailCnt)
}

func retryItemsBoundFixture(t *testing.T) (*executionFixture, retryItemsPageWriter, entity.HookExecutionManifest, string) {
	t.Helper()
	f := retryItemsInitializedFixture(t)
	source := hookTxSequence.Add(1)
	require.NoError(t, f.sql.Create(&model.ExptRunLog{ID: source, SpaceID: f.space, ExptID: f.expt, ExptRunID: source, Mode: gptr.Of(int32(1)), Status: gptr.Of(int64(entity.ExptStatus_Failed))}).Error)
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("source_run_id", source).Error)
	require.NoError(t, f.sql.Create(&model.ExptItemResult{ID: hookTxSequence.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: source, ItemID: 999001, ItemIdx: gptr.Of(int32(0)), Status: int32(entity.ItemRunState_Fail)}).Error)
	require.NoError(t, f.sql.Create(&model.ExptTurnResult{ID: hookTxSequence.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: source, ItemID: 999001, TurnIdx: gptr.Of(int32(0)), Status: int32(entity.TurnRunState_Fail)}).Error)
	require.NoError(t, f.sql.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", f.space, f.expt).UpdateColumn("fail_cnt", 1).Error)
	run, err := f.repo.GetRun(context.Background(), f.key)
	require.NoError(t, err)
	r := f.repo.(*hookRunRepo)
	r.executionBinding = &boundHookExecution{source: entity.HookExecutionInitializationSource{Key: f.key, ExecutionScope: "local", SnapshotHash: run.Snapshot.Hash, Mode: entity.EvaluationModeRetryItems, CreatedBy: run.CreatedBy, SourceRunID: source, BeforeEnabled: true, AfterEnabled: run.State.After.Status != entity.HookOperationDisabled, Execution: &entity.HookExecutionSnapshot{SingleSet: true}}}
	w, ok := f.repo.(retryItemsPageWriter)
	require.True(t, ok, "missing atomic retry tail repository")
	item := entity.HookPlanItem{ID: hookTxSequence.Add(1), SourceSpaceID: f.space, EvalSetID: 71, ItemID: 999001}
	page, err := w.ReadRetryItemsSources(context.Background(), f.key, "local", run.PlanCursor, []entity.HookPlanItem{item})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.NotNil(t, page.Items[0].Reuse)
	m := page.Items[0].Reuse.Clone()
	m.ItemRunLogID = hookTxSequence.Add(1)
	return f, w, m, run.PlanCursor
}

func TestHookRetryItemsTailAtomicCommitAndReplayMySQL(t *testing.T) {
	f, w, m, cursor := retryItemsBoundFixture(t)
	ctx := context.Background()
	old := f.manifests[0]
	changed, err := w.AppendPreparedRetryItemsPage(ctx, f.key, "local", cursor, []entity.HookExecutionManifest{m})
	require.NoError(t, err)
	require.True(t, changed)
	run, err := f.repo.GetRun(ctx, f.key)
	require.NoError(t, err)
	require.Equal(t, int64(2), run.PlanCount)
	var row model.ExptLifecycleRunItem
	require.NoError(t, hookRunScope(f.sql, f.key).Where("item_id=?", old.Frozen.ItemID).First(&row).Error)
	stored, err := hookTerminationManifest(f.key, row)
	require.NoError(t, err)
	require.Equal(t, old, stored)
	changed, err = w.AppendPreparedRetryItemsPage(ctx, f.key, "local", cursor, []entity.HookExecutionManifest{m})
	require.NoError(t, err)
	require.False(t, changed)
	var stats model.ExptStats
	require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&stats).Error)
	require.Equal(t, int32(2), stats.PendingCnt)
	require.Zero(t, stats.FailCnt)
}

func TestHookRetryItemsTailRollbackMySQL(t *testing.T) {
	f, w, m, cursor := retryItemsBoundFixture(t)
	failure := errors.New("tail write fault")
	require.NoError(t, f.sql.Callback().Create().Before("gorm:create").Register("retry_tail_fault", func(tx *gorm.DB) {
		if tx.Statement.Table == model.TableNameExptItemResultRunLog {
			tx.AddError(failure)
		}
	}))
	t.Cleanup(func() { require.NoError(t, f.sql.Callback().Create().Remove("retry_tail_fault")) })
	changed, err := w.AppendPreparedRetryItemsPage(context.Background(), f.key, "local", cursor, []entity.HookExecutionManifest{m})
	require.Error(t, err)
	require.False(t, changed)
	run, err := f.repo.GetRun(context.Background(), f.key)
	require.NoError(t, err)
	require.Equal(t, int64(1), run.PlanCount)
	require.Equal(t, cursor, run.PlanCursor)
	var result model.ExptItemResult
	require.NoError(t, f.sql.First(&result, m.ItemResultID).Error)
	require.Equal(t, m.Retry.RunID, result.ExptRunID)
	require.Equal(t, int32(entity.ItemRunState_Fail), result.Status)
	var count int64
	require.NoError(t, hookRunScope(f.sql.Model(&model.ExptLifecycleRunItem{}), f.key).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestHookRetryItemsCancellationCoversTailMySQL(t *testing.T) {
	f, w, m, cursor := retryItemsBoundFixture(t)
	ctx := context.Background()
	before, err := f.repo.GetRun(ctx, f.key)
	require.NoError(t, err)
	intent := entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated, Reason: "cancel"}
	out, err := f.repo.BeginFinalize(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: before.Version}, Intent: intent})
	require.NoError(t, err)
	var log model.ExptRunLog
	require.NoError(t, f.sql.First(&log, f.key.RunID).Error)
	var batches []entity.ExptRunLogItems
	require.NoError(t, json.Unmarshal(*log.ItemIds, &batches))
	c, err := entity.DecodeHookRetryItemsCursor(out.Run.PlanCursor, f.key, batches, out.Run.PlanCount, out.Run.PlanHash)
	require.NoError(t, err)
	require.Equal(t, 2, c.Terminal.AcceptedBatches)
	require.Equal(t, 1, c.Terminal.Batch)
	changed, err := w.AppendPreparedRetryItemsPage(ctx, f.key, "local", cursor, []entity.HookExecutionManifest{m})
	require.ErrorIs(t, err, entity.ErrHookAdmissionDenied)
	require.False(t, changed)
}

func TestHookRetryItemsConcurrentPageAndFinalizeMySQL(t *testing.T) {
	f, w, m, cursor := retryItemsBoundFixture(t)
	ctx := context.Background()
	run, err := f.repo.GetRun(ctx, f.key)
	require.NoError(t, err)
	var wg sync.WaitGroup
	wg.Add(2)
	start := make(chan struct{})
	errs := make([]error, 2)
	go func() {
		defer wg.Done()
		<-start
		_, errs[0] = w.AppendPreparedRetryItemsPage(ctx, f.key, "local", cursor, []entity.HookExecutionManifest{m})
	}()
	go func() {
		defer wg.Done()
		<-start
		_, errs[1] = f.repo.BeginFinalize(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: run.Version}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Success}})
	}()
	close(start)
	wg.Wait()
	require.NoError(t, errs[0])
	require.Error(t, errs[1])
	run, err = f.repo.GetRun(ctx, f.key)
	require.NoError(t, err)
	require.Equal(t, int64(2), run.PlanCount)
	require.Equal(t, entity.HookFinalizeNone, run.State.Finalize)
}

func TestHookRetryItemsInitialCompletionPreservesB0MySQL(t *testing.T) {
	f, _, _, _ := retryItemsBoundFixture(t)
	ctx := context.Background()
	raw, err := json.Marshal(map[string]any{"v": 1, "key": f.key, "fingerprint": f.hash, "phase": "verify", "route": "normal", "batch": 1, "retries": 0, "selected": entity.HookPlanDigest{Count: 1, Hash: f.hash}, "verified": entity.HookPlanDigest{Count: 1, Hash: f.hash}, "source_digest": entity.NewHookPlanDigest()})
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumns(map[string]any{"execution_initialized": false, "plan_cursor": string(raw)}).Error)
	require.NoError(t, f.sql.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", f.space, f.expt).UpdateColumn("pending_cnt", 0).Error)
	run, err := f.repo.GetRun(ctx, f.key)
	require.NoError(t, err)
	r := f.repo.(*hookRunRepo)
	_, err = r.CompleteExecutionInitialization(ctx, f.completeInput(run.Version))
	require.NoError(t, err)
	run, err = f.repo.GetRun(ctx, f.key)
	require.NoError(t, err)
	var c entity.HookRetryItemsCursor
	require.NoError(t, json.Unmarshal([]byte(run.PlanCursor), &c))
	require.Equal(t, 2, c.Version)
	require.Equal(t, 1, c.Batch, "the second accepted batch is not part of the initial plan")
}

func retryItemsInitializedFixture(t *testing.T) *executionFixture {
	t.Helper()
	f := newExecutionFixture(t, 1)
	ctx := context.Background()
	page, err := f.init.ReadExecutionInitializationPage(ctx, f.readInput())
	require.NoError(t, err)
	written, err := f.init.WriteExecutionInitializationPage(ctx, f.writeInput(page.RunVersion))
	require.NoError(t, err)
	_, err = f.init.CompleteExecutionInitialization(ctx, f.completeInput(written.RunVersion))
	require.NoError(t, err)
	ids, err := json.Marshal([]entity.ExptRunLogItems{{ItemIDs: []int64{f.manifests[0].Frozen.ItemID}}, {ItemIDs: []int64{999001}}})
	require.NoError(t, err)
	cursor, err := json.Marshal(map[string]any{"v": 2, "key": f.key, "fingerprint": f.hash, "phase": "tail", "batch": 1, "offset": 0, "published": entity.HookPlanDigest{Count: 1, Hash: f.hash}, "retries": 0})
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", f.key.RunID).UpdateColumns(map[string]any{"mode": int32(entity.EvaluationModeRetryItems), "item_ids": ids}).Error)
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("plan_cursor", string(cursor)).Error)
	return f
}

func TestHookRetryItemsTailBlocksPrematureFinalizeMySQL(t *testing.T) {
	f := retryItemsInitializedFixture(t)
	ctx := context.Background()
	before, err := f.repo.GetRun(ctx, f.key)
	require.NoError(t, err)
	_, err = f.repo.BeginFinalize(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: before.Version}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Success}})
	require.ErrorIs(t, err, entity.ErrHookFinalizationUnsettled)
	after, err := f.repo.GetRun(ctx, f.key)
	require.NoError(t, err)
	require.Equal(t, before, after)
}

func TestHookRetryItemsAtomicTailCapabilityMySQL(t *testing.T) {
	f := retryItemsInitializedFixture(t)
	writer, ok := f.repo.(retryItemsPageWriter)
	require.True(t, ok, "the execution repository must publish a complete tail page atomically")
	run, err := f.repo.GetRun(context.Background(), f.key)
	require.NoError(t, err)
	m := entity.HookExecutionManifest{Version: 1, Key: f.key, Ordinal: 1, Frozen: entity.HookPlanItem{ID: hookTxSequence.Add(1), SourceSpaceID: f.space, EvalSetID: 71, ItemID: 999001}, ItemResultID: hookTxSequence.Add(1), ItemRunLogID: hookTxSequence.Add(1), Turns: []entity.HookExecutionTurnManifest{{TurnID: 0, ResultID: hookTxSequence.Add(1)}}, TurnLogsInitialized: gptr.Of(false)}
	changed, err := writer.AppendPreparedRetryItemsPage(context.Background(), f.key, "local", run.PlanCursor, []entity.HookExecutionManifest{m})
	// Unbound repositories must not turn a prepared page into execution authority.
	require.ErrorIs(t, err, entity.ErrHookExecutionUnsupported)
	require.False(t, changed)
}
