// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func lateProofProgressFixture(t *testing.T, state int32) (*hookProgressFixture, *hookTurnProgressRepo, int64) {
	t.Helper()
	f := newHookProgressFixture(t)
	itemID := hookTxSequence.Add(1)
	require.NoError(t, f.sql.Create(&model.ExptItemResultRunLog{ID: itemID, SpaceID: f.row.SpaceID, ExptID: f.row.ExptID, ExptRunID: f.row.ExptRunID, ItemID: f.row.ItemID, ItemVersionID: f.row.ItemVersionID, Status: int32(entity.ItemRunState_Success), ResultState: gptr.Of(int32(entity.ExptItemResultStateResulted))}).Error)
	require.NoError(t, f.sql.Create(&model.Experiment{ID: f.row.ExptID, SpaceID: f.row.SpaceID, Name: t.Name(), LatestRunID: f.row.ExptRunID + 10000, Status: int32(entity.ExptStatus_Processing)}).Error)
	t.Cleanup(func() {
		require.NoError(t, f.sql.Unscoped().Where("id=?", itemID).Delete(&model.ExptItemResultRunLog{}).Error)
		require.NoError(t, f.sql.Unscoped().Where("id=?", f.row.ExptID).Delete(&model.Experiment{}).Error)
	})
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.row.SpaceID, f.row.ExptRunID).UpdateColumns(map[string]any{"finalize_state": state, "gate": 2, "terminal_status": int32(entity.ExptStatus_Terminated), "terminal_reason": "cancel", "terminal_at": time.Now().UTC()}).Error)
	require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("id=?", f.row.ID).UpdateColumn("status", int32(entity.TurnRunState_Success)).Error)
	r := NewHookTurnProgressRepo(f.p, func(context.Context) (string, error) { return "local", nil }).(*hookTurnProgressRepo)
	return f, r, itemID
}

func TestHookNormalProofRejectsChangesAfterArchiveMySQL(t *testing.T) {
	for _, state := range []int32{0, 1} {
		for _, change := range []string{"target", "registered", "ext", "status", "error", "item", "replay"} {
			t.Run(fmt.Sprintf("state=%d/%s", state, change), func(t *testing.T) {
				f, r, itemID := lateProofProgressFixture(t, state)
				fields := map[string]any{"terminal_status": nil, "terminal_reason": nil, "terminal_at": nil, "gate": 1}
				if state == 1 {
					fields = map[string]any{"terminal_status": int32(entity.ExptStatus_Success), "terminal_reason": "normal", "terminal_at": time.Now().UTC(), "gate": 2}
				}
				require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.row.SpaceID, f.row.ExptRunID).UpdateColumns(fields).Error)
				base, next := f.read(t), f.read(t)
				var beforeTurn model.ExptTurnResultRunLog
				var beforeItem model.ExptItemResultRunLog
				require.NoError(t, f.sql.First(&beforeTurn, base.ID).Error)
				require.NoError(t, f.sql.First(&beforeItem, itemID).Error)
				switch change {
				case "target":
					next.TargetResultID = 101
				case "registered":
					next.EvaluatorResultIds.Registered = append(next.EvaluatorResultIds.Registered, &entity.RegisteredEvalResult{VersionID: 402, Alias: "late", RecordID: 12})
				case "ext":
					next.Ext["late"] = "evidence"
				case "status":
					next.Status = entity.TurnRunState_Fail
				case "error":
					next.ErrMsg = "late error"
				}
				var err error
				if change == "item" {
					_, err = r.WriteItemRun(context.Background(), entity.HookItemRunWriteInput{HookRunKey: entity.HookTurnProgressIdentity(base).HookRunKey, ItemID: base.ItemID, ItemVersionID: base.ItemVersionID, Status: entity.ItemRunState_Processing})
				} else {
					_, err = r.WriteTurnResult(context.Background(), entity.HookTurnProgressInput{Base: base, Progress: next})
				}
				if change == "replay" {
					assert.NoError(t, err)
				} else {
					assert.ErrorIs(t, err, entity.ErrHookStoreConflict)
				}
				var afterTurn model.ExptTurnResultRunLog
				var afterItem model.ExptItemResultRunLog
				require.NoError(t, f.sql.First(&afterTurn, base.ID).Error)
				require.NoError(t, f.sql.First(&afterItem, itemID).Error)
				assert.Equal(t, beforeTurn, afterTurn)
				assert.Equal(t, beforeItem, afterItem)
			})
		}
	}
}

func TestHookLateProofInvalidationRollsBackTurnMySQL(t *testing.T) {
	f, r, itemID := lateProofProgressFixture(t, 1)
	base, next := f.read(t), f.read(t)
	next.TargetResultID = 101
	injected := errors.New("item proof update failed")
	const callback = "late-proof-invalidation-failure"
	require.NoError(t, f.sql.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "expt_item_result_run_log" {
			tx.AddError(injected)
		}
	}))
	t.Cleanup(func() { require.NoError(t, f.sql.Callback().Update().Remove(callback)) })
	_, err := r.WriteTurnResult(context.Background(), entity.HookTurnProgressInput{Base: base, Progress: next})
	assert.ErrorIs(t, err, entity.ErrHookExecutionStorage)
	assert.Equal(t, base, f.read(t), "failed invalidation cannot commit the changed reference")
	var item model.ExptItemResultRunLog
	require.NoError(t, f.sql.First(&item, itemID).Error)
	assert.Equal(t, int32(entity.ExptItemResultStateResulted), gptr.Indirect(item.ResultState))
}

func TestHookLateProofPendingItemReplayMySQL(t *testing.T) {
	for _, terminal := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "terminal"}[terminal], func(t *testing.T) {
			f, r, itemID := lateProofProgressFixture(t, 1)
			if terminal {
				require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("id=?", itemID).UpdateColumn("status", int32(entity.ItemRunState_Terminal)).Error)
			}
			var before model.ExptItemResultRunLog
			require.NoError(t, f.sql.First(&before, itemID).Error)
			gotTerminal, err := r.WriteItemRun(context.Background(), entity.HookItemRunWriteInput{HookRunKey: entity.HookTurnProgressIdentity(f.row).HookRunKey, ItemID: f.row.ItemID, ItemVersionID: f.row.ItemVersionID, Status: entity.ItemRunState_Success})
			require.NoError(t, err)
			assert.Equal(t, terminal, gotTerminal)
			var after model.ExptItemResultRunLog
			require.NoError(t, f.sql.First(&after, itemID).Error)
			assert.Equal(t, before, after)
		})
	}
}

// Removing Resulted invalidation must allow a superseded Run to retain stale archive proof.
func TestHookLateProofPendingEffectiveChangesMySQL(t *testing.T) {
	for _, change := range []string{"target", "registered", "inline", "ext", "status", "error", "replay"} {
		t.Run(change, func(t *testing.T) {
			f, r, itemID := lateProofProgressFixture(t, 1)
			base, next := f.read(t), f.read(t)
			switch change {
			case "target":
				next.TargetResultID = 101
			case "registered":
				next.EvaluatorResultIds.Registered = append(next.EvaluatorResultIds.Registered, &entity.RegisteredEvalResult{VersionID: 402, Alias: "late", RecordID: 12})
			case "inline":
				next.EvaluatorResultIds.Inline = append(next.EvaluatorResultIds.Inline, &entity.InlineEvalResult{InlineKey: "late", RecordID: 13})
			case "ext":
				next.Ext["late"] = "evidence"
			case "status":
				next.Status = entity.TurnRunState_Fail
			case "error":
				next.ErrMsg = "late failure"
			}
			_, err := r.WriteTurnResult(context.Background(), entity.HookTurnProgressInput{Base: base, Progress: next})
			require.NoError(t, err)
			var item model.ExptItemResultRunLog
			require.NoError(t, f.sql.First(&item, itemID).Error)
			want := entity.ExptItemResultStateLogged
			if change == "replay" {
				want = entity.ExptItemResultStateResulted
			}
			assert.Equal(t, int32(want), gptr.Indirect(item.ResultState), "effective Pending mutation must invalidate old-Run proof")
			var expt model.Experiment
			require.NoError(t, f.sql.First(&expt, f.row.ExptID).Error)
			assert.Equal(t, f.row.ExptRunID+10000, expt.LatestRunID)
			assert.Equal(t, int32(entity.ExptStatus_Processing), expt.Status)
			// Replay from the same stale base after re-archival must not invalidate it again.
			require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("id=?", itemID).UpdateColumn("result_state", int32(entity.ExptItemResultStateResulted)).Error)
			_, err = r.WriteTurnResult(context.Background(), entity.HookTurnProgressInput{Base: base, Progress: next})
			require.NoError(t, err)
			require.NoError(t, f.sql.First(&item, itemID).Error)
			assert.Equal(t, int32(entity.ExptItemResultStateResulted), gptr.Indirect(item.ResultState), "exact replay must preserve refreshed proof")
		})
	}
}

func TestHookLateProofPendingTerminalProgressAndItemMySQL(t *testing.T) {
	for _, write := range []string{"progress", "item"} {
		t.Run(write, func(t *testing.T) {
			f, r, itemID := lateProofProgressFixture(t, 1)
			if write == "progress" {
				require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("id=?", f.row.ID).UpdateColumns(map[string]any{"status": int32(entity.TurnRunState_Terminal), "err_msg": []byte("canceled")}).Error)
				base, next := f.read(t), f.read(t)
				next.Status, next.ErrMsg, next.TargetResultID = entity.TurnRunState_Processing, "", 101
				got, err := r.WriteTurnProgress(context.Background(), entity.HookTurnProgressInput{Base: base, Progress: next})
				require.NoError(t, err)
				assert.Equal(t, entity.TurnRunState_Terminal, got.Status)
				assert.Equal(t, "canceled", got.ErrMsg)
				assert.Equal(t, int64(101), got.TargetResultID)
			} else {
				_, err := r.WriteItemRun(context.Background(), entity.HookItemRunWriteInput{HookRunKey: entity.HookTurnProgressIdentity(f.row).HookRunKey, ItemID: f.row.ItemID, ItemVersionID: f.row.ItemVersionID, Status: entity.ItemRunState_Fail, ErrMsg: gptr.Of("late")})
				require.NoError(t, err)
			}
			var item model.ExptItemResultRunLog
			require.NoError(t, f.sql.First(&item, itemID).Error)
			assert.Equal(t, int32(entity.ExptItemResultStateLogged), gptr.Indirect(item.ResultState))
		})
	}
}

// A committed lifecycle is authoritative even if the stale caller still holds a valid admitted item.
func TestHookLateProofCommittedRejectsParentWritesMySQL(t *testing.T) {
	for _, action := range []string{"progress", "result", "replay", "item"} {
		t.Run(action, func(t *testing.T) {
			f, r, itemID := lateProofProgressFixture(t, 2)
			base, next := f.read(t), f.read(t)
			var beforeTurn model.ExptTurnResultRunLog
			var beforeItem model.ExptItemResultRunLog
			require.NoError(t, f.sql.First(&beforeTurn, base.ID).Error)
			require.NoError(t, f.sql.First(&beforeItem, itemID).Error)
			next.TargetResultID = 101
			var err error
			switch action {
			case "progress":
				next.Status = entity.TurnRunState_Processing
				_, err = r.WriteTurnProgress(context.Background(), entity.HookTurnProgressInput{Base: base, Progress: next})
			case "result", "replay":
				if action == "replay" {
					next = base
				}
				_, err = r.WriteTurnResult(context.Background(), entity.HookTurnProgressInput{Base: base, Progress: next})
			case "item":
				_, err = r.WriteItemRun(context.Background(), entity.HookItemRunWriteInput{HookRunKey: entity.HookTurnProgressIdentity(base).HookRunKey, ItemID: base.ItemID, ItemVersionID: base.ItemVersionID, Status: entity.ItemRunState_Fail})
			}
			assert.ErrorIs(t, err, entity.ErrHookStoreConflict)
			var afterTurn model.ExptTurnResultRunLog
			var afterItem model.ExptItemResultRunLog
			require.NoError(t, f.sql.First(&afterTurn, base.ID).Error)
			require.NoError(t, f.sql.First(&afterItem, itemID).Error)
			assert.Equal(t, beforeTurn, afterTurn)
			assert.Equal(t, beforeItem, afterItem)
			read, err := r.ReadTurnProgress(context.Background(), entity.HookTurnProgressIdentity(base))
			require.NoError(t, err, "read-only progress remains readable after Commit")
			assert.Equal(t, base.TargetResultID, read.TargetResultID)
		})
	}
}
