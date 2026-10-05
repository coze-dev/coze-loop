// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	driver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

type hookProgressFixture struct {
	p     db.Provider
	sql   *gorm.DB
	row   *entity.ExptTurnResultRunLog
	scope string
}

func newHookProgressFixture(t *testing.T) *hookProgressFixture {
	t.Helper()
	dsn := os.Getenv("HOOK_MYSQL_TX_DSN")
	if dsn == "" {
		t.Skip("requires HOOK_MYSQL_TX_DSN existing isolated MySQL")
	}
	cfg, err := driver.ParseDSN(dsn)
	require.NoError(t, err)
	require.Equal(t, "unix", cfg.Net)
	require.Equal(t, "/Users/bytedance/fornax_workspace/_local/scratch/hook-mysql-20260922/m.sock", cfg.Addr)
	require.Equal(t, "hook_7378265404_tx", cfg.DBName)
	p, err := db.NewDB(mysql.Open(dsn), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	s := p.NewSession(context.Background(), db.WithMaster())
	pool, err := s.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(6)
	space, expt, run, item, id := hookTxSequence.Add(1), hookTxSequence.Add(1), hookTxSequence.Add(1), hookTxSequence.Add(1), hookTxSequence.Add(1)
	f := &hookProgressFixture{p: p, sql: s, scope: "local", row: &entity.ExptTurnResultRunLog{ID: id, SpaceID: space, ExptID: expt, ExptRunID: run, ItemID: item, ItemVersionID: 7, TurnID: 1, Status: entity.TurnRunState_Processing, TargetResultID: 100, EvaluatorResultIds: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{401: 11}}, Ext: map[string]string{"keep": "original"}}}
	t.Cleanup(func() {
		for _, table := range []string{"expt_turn_result_run_log", "expt_lifecycle_run_item", "expt_lifecycle_run"} {
			require.NoError(t, s.Table(table).Where("space_id=? AND expt_id=?", space, expt).Delete(nil).Error)
		}
		require.NoError(t, pool.Close())
	})
	require.NoError(t, s.Create(&model.ExptLifecycleRun{SpaceID: space, ExptID: expt, ExptRunID: run, AfterEnabled: true, ExecutionScope: "local", SnapshotCipher: []byte{1}, SnapshotKeyID: "k", SnapshotHash: "hash", Gate: 1, PlanState: 1}).Error)
	require.NoError(t, s.Create(&model.ExptLifecycleRunItem{ID: hookTxSequence.Add(1), SpaceID: space, ExptID: expt, ExptRunID: run, ItemID: item, ItemVersionID: 7, AdmittedAt: gptr.Of(time.Now())}).Error)
	f.insert(t, f.row)
	return f
}

func (f *hookProgressFixture) insert(t *testing.T, row *entity.ExptTurnResultRunLog) {
	t.Helper()
	po, err := convert.NewExptTurnResultRunLogConvertor().DO2PO(row)
	require.NoError(t, err)
	require.NoError(t, f.sql.Create(po).Error)
}

func (f *hookProgressFixture) read(t *testing.T) *entity.ExptTurnResultRunLog {
	t.Helper()
	var po model.ExptTurnResultRunLog
	require.NoError(t, f.sql.Where("id=?", f.row.ID).First(&po).Error)
	row, err := convert.NewExptTurnResultRunLogConvertor().PO2DO(&po)
	require.NoError(t, err)
	return row
}

func (f *hookProgressFixture) write(ctx context.Context, base, next *entity.ExptTurnResultRunLog) error {
	_, err := NewHookTurnProgressRepo(f.p, func(context.Context) (string, error) { return f.scope, nil }).WriteTurnProgress(ctx, entity.HookTurnProgressInput{Base: base, Progress: next})
	return err
}

func TestHookTurnProgressCurrentTerminalWins(t *testing.T) {
	f := newHookProgressFixture(t)
	base := f.read(t)
	next := *base
	next.TargetResultID = 101
	tx := f.sql.Begin()
	require.NoError(t, tx.Error)
	defer tx.Rollback()
	require.NoError(t, tx.Model(&model.ExptTurnResultRunLog{}).Where("id=?", base.ID).Updates(map[string]any{"status": int32(entity.TurnRunState_Terminal), "err_msg": []byte("user terminated")}).Error)
	done := make(chan error, 1)
	go func() { done <- f.write(context.Background(), base, &next) }()
	select {
	case err := <-done:
		t.Fatalf("write bypassed row transaction: %v", err)
	case <-time.After(40 * time.Millisecond):
	}
	require.NoError(t, tx.Commit().Error)
	require.NoError(t, <-done)
	got := f.read(t)
	assert.Equal(t, entity.TurnRunState_Terminal, got.Status)
	assert.Equal(t, "user terminated", got.ErrMsg)
	assert.Equal(t, int64(101), got.TargetResultID)
	assert.Equal(t, base.Ext, got.Ext)
	snapshot, _ := json.Marshal(map[string]any{"test": t.Name(), "before": base, "after": got})
	t.Logf("HOOK_PROGRESS_SNAPSHOT %s", snapshot)
}

func TestHookTurnProgressMissingAndDeletedNeverUpsert(t *testing.T) {
	for _, soft := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "deleted"}[soft], func(t *testing.T) {
			f := newHookProgressFixture(t)
			base := f.read(t)
			next := *base
			next.TargetResultID = 101
			q := f.sql.Where("id=?", base.ID)
			if !soft {
				q = q.Unscoped()
			}
			require.NoError(t, q.Delete(&model.ExptTurnResultRunLog{}).Error)
			assert.Error(t, f.write(context.Background(), base, &next))
			var live int64
			require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("id=?", base.ID).Count(&live).Error)
			assert.Zero(t, live)
		})
	}
}

func TestHookTurnProgressStaleDisjointReferencesMerge(t *testing.T) {
	f := newHookProgressFixture(t)
	base := f.read(t)
	current := &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{401: 11}, Registered: []*entity.RegisteredEvalResult{{VersionID: 402, Alias: "other", RecordID: 12}}, Inline: []*entity.InlineEvalResult{{InlineKey: "quality", RecordID: 13}}}
	raw, err := json.Marshal(current)
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("id=?", base.ID).UpdateColumn("evaluator_result_ids", raw).Error)
	next := *base
	next.EvaluatorResultIds = &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{401: 11}, Registered: []*entity.RegisteredEvalResult{{VersionID: 402, Alias: "new", RecordID: 14}}}
	require.NoError(t, f.write(context.Background(), base, &next))
	got := f.read(t).EvaluatorResultIds
	assert.Contains(t, got.Registered, &entity.RegisteredEvalResult{VersionID: 402, Alias: "other", RecordID: 12})
	assert.Contains(t, got.Registered, &entity.RegisteredEvalResult{VersionID: 402, Alias: "new", RecordID: 14})
	assert.Equal(t, current.Inline, got.Inline)
}

func TestHookTurnProgressExactIdentityAndScope(t *testing.T) {
	for _, field := range []string{"space", "experiment", "run", "item", "version", "turn", "log", "scope", "not-admitted"} {
		t.Run(field, func(t *testing.T) {
			f := newHookProgressFixture(t)
			before := f.read(t)
			base := *before
			next := base
			next.TargetResultID = 101
			sibling := *before
			sibling.ID = hookTxSequence.Add(1)
			sibling.TurnID = 2
			f.insert(t, &sibling)
			otherRun := *before
			otherRun.ID = hookTxSequence.Add(1)
			otherRun.ExptRunID = hookTxSequence.Add(1)
			f.insert(t, &otherRun)
			switch field {
			case "space":
				base.SpaceID++
				next.SpaceID++
			case "experiment":
				base.ExptID++
				next.ExptID++
			case "run":
				base.ExptRunID = otherRun.ExptRunID
				next.ExptRunID = otherRun.ExptRunID
			case "item":
				base.ItemID++
				next.ItemID++
			case "version":
				base.ItemVersionID++
				next.ItemVersionID++
			case "turn":
				base.TurnID = 2
				next.TurnID = 2
			case "log":
				base.ID = sibling.ID
				next.ID = sibling.ID
			case "scope":
				f.scope = "other"
			case "not-admitted":
				require.NoError(t, f.sql.Model(&model.ExptLifecycleRunItem{}).Where("space_id=? AND expt_id=?", base.SpaceID, base.ExptID).UpdateColumn("admitted_at", nil).Error)
			}
			assert.Error(t, f.write(context.Background(), &base, &next))
			assert.Equal(t, before, f.read(t))
			for _, row := range []*entity.ExptTurnResultRunLog{&sibling, &otherRun} {
				var got model.ExptTurnResultRunLog
				require.NoError(t, f.sql.Where("id=?", row.ID).First(&got).Error)
				assert.Equal(t, int64(100), got.TargetResultID)
			}
		})
	}
}

func TestHookTurnProgressConflictRejectsStaleReplacement(t *testing.T) {
	for _, target := range []bool{false, true} {
		t.Run(map[bool]string{false: "evaluator", true: "target"}[target], func(t *testing.T) {
			f := newHookProgressFixture(t)
			base := f.read(t)
			newer := *base
			if target {
				newer.TargetResultID = 102
			} else {
				newer.EvaluatorResultIds = &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{401: 22}}
			}
			require.NoError(t, f.write(context.Background(), base, &newer))
			before := f.read(t)
			stale := *base
			if target {
				stale.TargetResultID = 101
			} else {
				stale.EvaluatorResultIds = &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{401: 21}}
			}
			assert.ErrorIs(t, f.write(context.Background(), base, &stale), entity.ErrHookStoreConflict)
			assert.Equal(t, before, f.read(t))
			require.NoError(t, f.write(context.Background(), base, &newer))
		})
	}
}

func TestHookTurnProgressSuccessfulWriteTouchesOneTurn(t *testing.T) {
	f := newHookProgressFixture(t)
	base := f.read(t)
	sibling := *base
	sibling.ID = hookTxSequence.Add(1)
	sibling.TurnID = 2
	f.insert(t, &sibling)
	before := f.read(t)
	for _, state := range []entity.TurnRunState{entity.TurnRunState_Success, entity.TurnRunState_Fail} {
		require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("id=?", base.ID).Updates(map[string]any{"status": int32(state), "err_msg": []byte("old")}).Error)
		next := *base
		next.TargetResultID = 101
		require.NoError(t, f.write(context.Background(), base, &next))
		got := f.read(t)
		assert.Equal(t, entity.TurnRunState_Processing, got.Status)
		assert.Empty(t, got.ErrMsg)
		assert.Equal(t, int64(101), got.TargetResultID)
	}
	var po model.ExptTurnResultRunLog
	require.NoError(t, f.sql.Where("id=?", sibling.ID).First(&po).Error)
	assert.Equal(t, before.TargetResultID, po.TargetResultID)
	assert.Equal(t, int32(before.Status), po.Status)
}

func TestHookTurnProgressStaleEvaluatorCannotAttachToNewTarget(t *testing.T) {
	f := newHookProgressFixture(t)
	base := f.read(t)
	newer := *base
	newer.TargetResultID = 102
	require.NoError(t, f.write(context.Background(), base, &newer))
	stale := *base
	stale.EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 402, RecordID: 14}}}
	assert.ErrorIs(t, f.write(context.Background(), base, &stale), entity.ErrHookStoreConflict)
	got := f.read(t)
	for _, ref := range got.EvaluatorResultIds.Registered {
		assert.NotEqual(t, int64(14), ref.RecordID)
	}
}

func TestHookTurnProgressAcceptsZeroTurn(t *testing.T) {
	f := newHookProgressFixture(t)
	require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("id=?", f.row.ID).UpdateColumn("turn_id", 0).Error)
	base := f.read(t)
	next := *base
	next.TargetResultID = 101
	require.NoError(t, f.write(context.Background(), base, &next))
	assert.Equal(t, int64(0), f.read(t).TurnID)
	key := entity.HookTurnProgressIdentity(base)
	key.TurnID = -1
	assert.Error(t, key.Validate())
}

func TestHookTurnProgressUnversionedRunLogCannotMatchVersionedLedger(t *testing.T) {
	f := newHookProgressFixture(t)
	require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("id=?", f.row.ID).UpdateColumn("item_version_id", 0).Error)
	base := f.read(t)
	next := *base
	next.TargetResultID = 101
	assert.ErrorIs(t, f.write(context.Background(), base, &next), entity.ErrHookStoreConflict)
	assert.Equal(t, base, f.read(t))
}

func TestHookTurnProgressConcurrentDeleteNeverResurrects(t *testing.T) {
	f := newHookProgressFixture(t)
	base := f.read(t)
	next := *base
	next.TargetResultID = 101
	tx := f.sql.Begin()
	require.NoError(t, tx.Error)
	defer tx.Rollback()
	require.NoError(t, tx.Where("id=?", base.ID).Delete(&model.ExptTurnResultRunLog{}).Error)
	done := make(chan error, 1)
	go func() { done <- f.write(context.Background(), base, &next) }()
	select {
	case err := <-done:
		t.Fatalf("write bypassed delete transaction: %v", err)
	case <-time.After(40 * time.Millisecond):
	}
	require.NoError(t, tx.Commit().Error)
	assert.ErrorIs(t, <-done, entity.ErrHookStoreMissing)
	var live int64
	require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("id=?", base.ID).Count(&live).Error)
	assert.Zero(t, live)
}
