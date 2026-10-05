// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type boundFailPageRepo struct {
	repo.IHookExecutionInitializationRepo
	fail bool
}

type boundMissingPageSource struct {
	repo.IHookExecutionInitializationRepo
}

func (r boundMissingPageSource) ReadExecutionInitializationPage(ctx context.Context, in entity.HookExecutionInitializationReadInput) (*entity.HookExecutionInitializationPage, error) {
	p, err := r.IHookExecutionInitializationRepo.ReadExecutionInitializationPage(ctx, in)
	if err == nil {
		p.BoundSnapshotHash = ""
	}
	return p, err
}

func TestHookBoundInitializationRejectsMissingPageBindingMySQL(t *testing.T) {
	f := newBoundInitFixture(t, 0, entity.EvaluationModeSubmit, false, true)
	r := boundInitializationRepo(t, f)
	_, err := f.initialize(t, r)
	require.NoError(t, err)
	_, err = f.initialize(t, boundMissingPageSource{r})
	require.Error(t, err, "explicit bound initializer must not accept an unbound initialized receipt")
}

func (r *boundFailPageRepo) WriteExecutionInitializationPage(ctx context.Context, in entity.HookExecutionInitializationWriteInput) (*entity.HookExecutionInitializationPage, error) {
	if r.fail && in.StartOrdinal >= 100 {
		return nil, entity.ErrHookExecutionStorage
	}
	return r.IHookExecutionInitializationRepo.WriteExecutionInitializationPage(ctx, in)
}

func boundCount(t *testing.T, f *boundInitFixture, table any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, f.sql.Unscoped().Model(table).Where("space_id=? AND expt_id=?", f.space, f.expt).Count(&n).Error)
	return n
}

func TestHookBoundInitializationPagesResumeFrozenConfigMySQL(t *testing.T) {
	f := newBoundInitFixture(t, 205, entity.EvaluationModeTrialRun, false, true)
	r := boundInitializationRepo(t, f)
	failure := &boundFailPageRepo{IHookExecutionInitializationRepo: r, fail: true}
	_, err := f.initialize(t, failure)
	require.ErrorIs(t, err, entity.ErrHookExecutionStorage)
	require.EqualValues(t, 100, boundCount(t, f, &model.ExptItemRef{}))
	first, err := r.ReadExecutionInitializationPage(context.Background(), entity.HookPlanReadInput{Key: f.key, ExecutionScope: "local", Limit: 100})
	require.NoError(t, err)
	f.dataset.denied = map[int64]bool{}
	for _, item := range f.items[:100] {
		f.dataset.denied[item.ItemID] = true
	}
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumns(map[string]any{"eval_conf": []byte(`{}`), "target_id": 9999, "target_space_id": 9998}).Error)
	failure.fail = false
	result, err := f.initialize(t, failure)
	require.NoError(t, err)
	require.True(t, result.Initialized)
	after, err := r.ReadExecutionInitializationPage(context.Background(), entity.HookPlanReadInput{Key: f.key, ExecutionScope: "local", Limit: 100})
	require.NoError(t, err)
	require.Equal(t, first.Items, after.Items)
	require.EqualValues(t, 205, boundCount(t, f, &model.ExptItemRef{}))
	var raw model.ExptItemRef
	require.NoError(t, f.sql.Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, f.items[204].ItemID).First(&raw).Error)
	require.Contains(t, string(*raw.ItemConfig), `"private-target":"first"`)
	var stats model.ExptStats
	require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&stats).Error)
	require.EqualValues(t, 205, stats.PendingCnt)
	reads := f.dataset.calls.Load()
	f.dataset.fail = true
	result, err = f.initialize(t, r)
	require.NoError(t, err)
	require.True(t, result.Initialized)
	require.False(t, result.Changed)
	require.Equal(t, reads, f.dataset.calls.Load())
}

func TestHookBoundInitializationEmptyAndOldConstructorMySQL(t *testing.T) {
	f := newBoundInitFixture(t, 0, entity.EvaluationModeSubmit, true, true)
	_, err := f.initialize(t, exptinfra.NewHookExecutionInitializationRepo(f.p))
	require.ErrorIs(t, err, entity.ErrHookExecutionUnsupported)
	r := boundInitializationRepo(t, f)
	result, err := f.initialize(t, r)
	require.NoError(t, err)
	require.True(t, result.Initialized)
	require.Zero(t, f.dataset.calls.Load())
	require.Zero(t, boundCount(t, f, &model.ExptItemRef{}))
	require.False(t, finalizationRead(t, f.finalizationManagerFixture).State.After.Activated)
}

func TestHookBoundInitializationRollsBackRefAndRecordsMySQL(t *testing.T) {
	for _, table := range []string{"expt_item_ref", "expt_turn_result", "expt_lifecycle_run_item"} {
		t.Run(table, func(t *testing.T) {
			f := newBoundInitFixture(t, 2, entity.EvaluationModeSubmit, false, true)
			r := boundInitializationRepo(t, f)
			before := finalizationRead(t, f.finalizationManagerFixture)
			injected := errors.New("bound write failure")
			callback := func(tx *gorm.DB) {
				if tx.Statement.Table == table {
					tx.AddError(injected)
				}
			}
			if table == "expt_lifecycle_run_item" {
				require.NoError(t, f.sql.Callback().Update().Before("gorm:update").Register("bound-fault", callback))
			} else {
				require.NoError(t, f.sql.Callback().Create().Before("gorm:create").Register("bound-fault", callback))
			}
			_, err := f.initialize(t, r)
			require.Error(t, err)
			if table == "expt_lifecycle_run_item" {
				require.NoError(t, f.sql.Callback().Update().Remove("bound-fault"))
			} else {
				require.NoError(t, f.sql.Callback().Create().Remove("bound-fault"))
			}
			for _, table := range []any{&model.ExptItemRef{}, &model.ExptItemResult{}, &model.ExptTurnResult{}, &model.ExptItemResultRunLog{}} {
				require.Zero(t, boundCount(t, f, table))
			}
			require.Equal(t, before, finalizationRead(t, f.finalizationManagerFixture))
			_, err = f.initialize(t, r)
			require.NoError(t, err)
		})
	}
}

func TestHookBoundInitializationRejectsUnreadyOrCancelledMySQL(t *testing.T) {
	for _, state := range []string{"unprepared", "before-wait", "cancelled", "not-latest", "deleted", "hash-changed"} {
		t.Run(state, func(t *testing.T) {
			var stage []string
			if state == "unprepared" || state == "before-wait" {
				stage = []string{state}
			}
			f := newBoundInitFixture(t, 2, entity.EvaluationModeSubmit, true, true, stage...)
			r := boundInitializationRepo(t, f)
			ctx := context.Background()
			switch state {
			case "cancelled":
				run := finalizationRead(t, f.finalizationManagerFixture)
				_, err := f.repo.BeginFinalize(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: run.Version}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated}})
				require.NoError(t, err)
			case "not-latest":
				key := f.key
				key.RunID = finalizationTestIDs.Add(1)
				initial, err := f.manager.hooks.Initialization.ReadRunInitialization(ctx, key)
				require.NoError(t, err)
				attempted := false
				err = f.manager.initializeHookRun(ctx, &entity.ExptRunLog{ID: key.RunID, ExptRunID: key.RunID, SpaceID: f.space, ExptID: f.expt, Mode: int32(entity.EvaluationModeSubmit), Status: int64(entity.ExptStatus_Pending), CreatedBy: "successor"}, initial, &attempted, nil)
				require.NoError(t, err)
				require.True(t, attempted)
			case "deleted":
				require.NoError(t, f.sql.Delete(&model.Experiment{}, f.expt).Error)
			case "hash-changed":
				require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("snapshot_hash", fmt.Sprintf("%064d", 1)).Error)
			}
			_, err := f.initialize(t, r)
			require.Error(t, err)
			require.Zero(t, boundCount(t, f, &model.ExptItemRef{}))
			require.Zero(t, boundCount(t, f, &model.ExptItemResult{}))
		})
	}
}

func TestHookBoundInitializationProofTamperingMySQL(t *testing.T) {
	for _, kind := range []string{"missing", "soft-deleted", "config", "order", "set", "item-version", "ref-id", "manifest-digest", "manifest-missing", "extra", "result-missing"} {
		t.Run(kind, func(t *testing.T) {
			f := newBoundInitFixture(t, 101, entity.EvaluationModeSubmit, false, true)
			r := boundInitializationRepo(t, f)
			_, err := f.initialize(t, r)
			require.NoError(t, err)
			var ref model.ExptItemRef
			item := f.items[100]
			require.NoError(t, f.sql.Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, item.ItemID).First(&ref).Error)
			switch kind {
			case "result-missing":
				require.NoError(t, f.sql.Unscoped().Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, item.ItemID).Delete(&model.ExptItemResult{}).Error)
			case "missing":
				require.NoError(t, f.sql.Unscoped().Delete(&model.ExptItemRef{}, ref.ID).Error)
			case "soft-deleted":
				require.NoError(t, f.sql.Delete(&model.ExptItemRef{}, ref.ID).Error)
			case "config":
				require.NoError(t, f.sql.Model(&ref).UpdateColumn("item_config", []byte(`{}`)).Error)
			case "order":
				require.NoError(t, f.sql.Model(&ref).UpdateColumn("order_idx", 999).Error)
			case "set":
				require.NoError(t, f.sql.Model(&ref).UpdateColumn("eval_set_id", 999).Error)
			case "item-version":
				require.NoError(t, f.sql.Model(&ref).UpdateColumn("item_version_id", 999).Error)
			case "ref-id":
				require.NoError(t, f.sql.Model(&ref).UpdateColumn("id", finalizationTestIDs.Add(1)).Error)
			case "manifest-digest", "manifest-missing":
				var ledger model.ExptLifecycleRunItem
				require.NoError(t, f.sql.First(&ledger, item.ID).Error)
				var m entity.HookExecutionManifest
				require.NoError(t, json.Unmarshal(*ledger.ExecutionManifest, &m))
				if kind == "manifest-digest" {
					m.ItemRef.ConfigHash = fmt.Sprintf("%064d", 1)
				} else {
					m.ItemRef = nil
				}
				raw, err := json.Marshal(m)
				require.NoError(t, err)
				require.NoError(t, f.sql.Model(&ledger).UpdateColumn("execution_manifest", raw).Error)
			case "extra":
				ref.ID = finalizationTestIDs.Add(1)
				ref.ItemID = finalizationTestIDs.Add(1)
				require.NoError(t, f.sql.Create(&ref).Error)
			}
			_, err = r.ReadExecutionInitializationPage(context.Background(), entity.HookPlanReadInput{Key: f.key, ExecutionScope: "local", StartOrdinal: 100, Limit: 1})
			require.Error(t, err)
			before := finalizationRead(t, f.finalizationManagerFixture)
			_, err = r.CompleteExecutionInitialization(context.Background(), entity.HookExecutionInitializationCompleteInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: before.Version}, ExecutionScope: "local", PlanHash: before.PlanHash, ExpectedItemCount: 101, ExpectedTurnCount: 101})
			require.Error(t, err)
			f.dataset.fail = true
			_, err = f.initialize(t, r)
			require.Error(t, err, "initialized replay must check later pages too")
			require.Equal(t, before, finalizationRead(t, f.finalizationManagerFixture))
		})
	}
}
