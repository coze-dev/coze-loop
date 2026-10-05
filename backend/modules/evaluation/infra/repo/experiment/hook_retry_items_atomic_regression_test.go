// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type retryItemsFinalProbeSnapshot struct {
	Life     model.ExptLifecycleRun
	Stats    model.ExptStats
	Ledger   []model.ExptLifecycleRunItem
	Items    []model.ExptItemResult
	Turns    []model.ExptTurnResult
	ItemLogs []model.ExptItemResultRunLog
	TurnLogs []model.ExptTurnResultRunLog
	Refs     []model.ExptTurnEvaluatorResultRef
}

func readRetryItemsFinalProbeSnapshot(db *gorm.DB, f *executionFixture) (retryItemsFinalProbeSnapshot, error) {
	var out retryItemsFinalProbeSnapshot
	if err := db.Where("space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, f.key.RunID).First(&out.Life).Error; err != nil {
		return out, err
	}
	if err := db.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&out.Stats).Error; err != nil {
		return out, err
	}
	for _, rows := range []any{&out.Ledger, &out.Items, &out.Turns, &out.ItemLogs, &out.TurnLogs, &out.Refs} {
		if err := db.Unscoped().Where("space_id=? AND expt_id=?", f.space, f.expt).Order("id").Find(rows).Error; err != nil {
			return out, err
		}
	}
	return out, nil
}

func TestHookRetryItemsFinalProbeLateLifecycleRollbackMySQL(t *testing.T) {
	if os.Getenv("HOOK_MYSQL_EXECUTION_DSN") == "" {
		t.Skip("requires isolated HOOK_MYSQL_EXECUTION_DSN")
	}
	f, writer, manifest, cursor := retryItemsBoundFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	require.Len(t, manifest.Turns, 1)
	ref := model.ExptTurnEvaluatorResultRef{ID: hookTxSequence.Add(1), SpaceID: f.space, ExptID: f.expt, ExptTurnResultID: manifest.Turns[0].ResultID, EvaluatorVersionID: 701, EvaluatorResultID: hookTxSequence.Add(1), SourceType: 1, Alias_: "rollback-source"}
	require.NoError(t, f.sql.Create(&ref).Error)
	t.Cleanup(func() {
		require.NoError(t, f.sql.Unscoped().Where("id=? AND space_id=? AND expt_id=?", ref.ID, f.space, f.expt).Delete(&model.ExptTurnEvaluatorResultRef{}).Error)
	})
	page, err := writer.ReadRetryItemsSources(ctx, f.key, "local", cursor, []entity.HookPlanItem{manifest.Frozen})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.NotNil(t, page.Items[0].Reuse)
	logID := manifest.ItemRunLogID
	manifest = page.Items[0].Reuse.Clone()
	manifest.ItemRunLogID = logID
	before, err := readRetryItemsFinalProbeSnapshot(f.sql, f)
	require.NoError(t, err)
	require.Len(t, before.Ledger, 1)
	require.Len(t, before.Items, 2)
	require.Len(t, before.Turns, 3)
	require.Len(t, before.ItemLogs, 1)
	require.Empty(t, before.TurnLogs)
	require.Len(t, before.Refs, 1)
	require.Equal(t, manifest.Retry.RunID, before.Items[1].ExptRunID)
	require.Equal(t, int32(entity.ItemRunState_Fail), before.Items[1].Status)
	require.Equal(t, manifest.Retry.RunID, before.Turns[2].ExptRunID)
	require.Equal(t, int32(entity.TurnRunState_Fail), before.Turns[2].Status)
	require.Equal(t, int64(1), before.Life.PlanCount)
	require.Equal(t, int32(1), before.Stats.PendingCnt)
	require.Equal(t, int32(1), before.Stats.FailCnt)

	type faultContextKey struct{}
	ctx = context.WithValue(ctx, faultContextKey{}, f.key)
	fault := errors.New("final probe lifecycle write fault")
	name := fmt.Sprintf("retry_items_final_probe_lifecycle_%d", f.key.RunID)
	updates := f.sql.Callback().Update()
	require.Nil(t, updates.Get(name))
	t.Cleanup(func() {
		require.NoError(t, updates.Remove(name))
		require.Nil(t, updates.Get(name))
	})
	var during retryItemsFinalProbeSnapshot
	var readErr error
	hits := 0
	// Fail after the actual final UPDATE, but before either transaction can commit.
	require.NoError(t, updates.After("gorm:update").Before("gorm:commit_or_rollback_transaction").Register(name, func(tx *gorm.DB) {
		if tx.Error != nil || tx.Statement.Table != model.TableNameExptLifecycleRun || tx.Statement.Context.Value(faultContextKey{}) != f.key {
			return
		}
		fields, ok := tx.Statement.Dest.(map[string]any)
		if !ok || fields["plan_count"] != int64(2) || fields["plan_cursor"] == nil || fields["plan_hash"] == nil || tx.RowsAffected != 1 {
			return
		}
		hits++
		during, readErr = readRetryItemsFinalProbeSnapshot(tx.Session(&gorm.Session{NewDB: true}), f)
		tx.AddError(fault)
	}))
	changed, err := writer.AppendPreparedRetryItemsPage(ctx, f.key, "local", cursor, []entity.HookExecutionManifest{manifest})
	require.ErrorIs(t, err, entity.ErrHookExecutionStorage)
	require.False(t, changed)
	require.Equal(t, 1, hits, "the failure must reach the successful final lifecycle UPDATE")
	require.NoError(t, readErr)
	require.Equal(t, int64(2), during.Life.PlanCount)
	require.NotEqual(t, before.Life.PlanCursor, during.Life.PlanCursor)
	require.NotEqual(t, before.Life.PlanHash, during.Life.PlanHash)
	require.Len(t, during.Ledger, 2)
	require.NotNil(t, during.Ledger[1].ExecutionManifest)
	require.Len(t, during.ItemLogs, 2)
	require.Len(t, during.Items, 2)
	require.Len(t, during.Turns, 3)
	require.Equal(t, manifest.ItemResultID, during.Items[1].ID)
	require.Equal(t, f.key.RunID, during.Items[1].ExptRunID)
	require.Equal(t, f.key.RunID, during.Turns[2].ExptRunID)
	require.Empty(t, during.Refs, "source evaluator references were deleted inside the failed transaction")
	require.Equal(t, int32(2), during.Stats.PendingCnt)
	require.Zero(t, during.Stats.FailCnt)
	after, err := readRetryItemsFinalProbeSnapshot(f.sql, f)
	require.NoError(t, err)
	require.Equal(t, before, after, "all stored fields, owners, references, prefix bytes, statistics and cursor/hash must roll back")
}

func TestHookRetryItemsFinalProbeSamePageWorkersMySQL(t *testing.T) {
	if os.Getenv("HOOK_MYSQL_EXECUTION_DSN") == "" {
		t.Skip("requires isolated HOOK_MYSQL_EXECUTION_DSN")
	}
	f, writer, manifest, cursor := retryItemsBoundFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	before, err := readRetryItemsFinalProbeSnapshot(f.sql, f)
	require.NoError(t, err)
	require.Len(t, before.Ledger, 1)
	require.Len(t, before.Items, 2)
	require.Len(t, before.Turns, 3)
	require.Len(t, before.ItemLogs, 1)
	type outcome struct {
		changed bool
		err     error
	}
	start := make(chan struct{})
	results := make(chan outcome, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	for i := 0; i < 2; i++ {
		workerPage := []entity.HookExecutionManifest{manifest.Clone()}
		go func() {
			ready.Done()
			<-start
			changed, err := writer.AppendPreparedRetryItemsPage(ctx, f.key, "local", cursor, workerPage)
			results <- outcome{changed: changed, err: err}
		}()
	}
	ready.Wait()
	close(start)
	// Join both calls before any fatal assertion or fixture cleanup.
	completed := []outcome{<-results, <-results}
	winners := 0
	for _, result := range completed {
		if result.changed {
			winners++
			require.NoError(t, result.err)
		} else if result.err != nil {
			require.ErrorIs(t, result.err, entity.ErrHookStoreConflict)
		}
	}
	require.Equal(t, 1, winners)
	after, err := readRetryItemsFinalProbeSnapshot(f.sql, f)
	require.NoError(t, err)
	require.Len(t, after.Ledger, 2)
	require.Len(t, after.Items, 2)
	require.Len(t, after.Turns, 3)
	require.Len(t, after.ItemLogs, 2)
	require.Empty(t, after.TurnLogs)
	require.Empty(t, after.Refs)
	require.Equal(t, before.Ledger[0], after.Ledger[0])
	require.Equal(t, before.Items[0], after.Items[0])
	require.Equal(t, before.Turns[:2], after.Turns[:2])
	require.Equal(t, before.ItemLogs[0], after.ItemLogs[0])
	require.Equal(t, manifest.Frozen.ID, after.Ledger[1].ID)
	require.Equal(t, int64(1), after.Ledger[1].Ordinal)
	require.NotNil(t, after.Ledger[1].ExecutionManifest)
	expectedManifest, err := json.Marshal(manifest)
	require.NoError(t, err)
	// Empty refs are omitted by the persisted JSON contract.
	require.JSONEq(t, string(expectedManifest), string(*after.Ledger[1].ExecutionManifest))
	require.Equal(t, manifest.ItemResultID, after.Items[1].ID)
	require.Equal(t, f.key.RunID, after.Items[1].ExptRunID)
	require.Equal(t, int32(entity.ItemRunState_Queueing), after.Items[1].Status)
	require.Equal(t, manifest.Turns[0].ResultID, after.Turns[2].ID)
	require.Equal(t, f.key.RunID, after.Turns[2].ExptRunID)
	require.Equal(t, int32(entity.TurnRunState_Queueing), after.Turns[2].Status)
	require.Equal(t, manifest.ItemRunLogID, after.ItemLogs[1].ID)
	require.Equal(t, int32(2), after.Stats.PendingCnt)
	require.Zero(t, after.Stats.ProcessingCnt)
	require.Zero(t, after.Stats.SuccessCnt)
	require.Zero(t, after.Stats.FailCnt)
	require.Zero(t, after.Stats.TerminatedCnt)
	require.Equal(t, before.Stats.CreditCost, after.Stats.CreditCost)
	require.Equal(t, int64(2), after.Life.PlanCount)
	require.Equal(t, before.Life.Version+1, after.Life.Version)
	require.NotNil(t, after.Life.PlanCursor)
	require.NotNil(t, after.Life.PlanHash)
	var position struct {
		Version   int `json:"v"`
		Batch     int `json:"batch"`
		Offset    int `json:"offset"`
		Published struct {
			Count int64
			Hash  string
		} `json:"published"`
	}
	require.NoError(t, json.Unmarshal([]byte(*after.Life.PlanCursor), &position))
	require.Equal(t, 2, position.Version)
	require.Equal(t, 2, position.Batch)
	require.Zero(t, position.Offset)
	require.Equal(t, int64(2), position.Published.Count)
	require.Equal(t, *after.Life.PlanHash, position.Published.Hash)
	require.NotEqual(t, before.Life.PlanHash, after.Life.PlanHash)
}
