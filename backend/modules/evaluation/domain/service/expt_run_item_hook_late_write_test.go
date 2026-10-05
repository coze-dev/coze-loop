// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	store "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	dao "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

type hookLateWriteFixture struct {
	*executionHookFixture
	sql  *gorm.DB
	exec *ExptItemEvalCtxExecutor
	base *entity.ExptTurnResultRunLog
	item *model.ExptItemResultRunLog
}

// Exposes only the old contract, even when the underlying repository supports more.
type hookLateProgressOnly struct{ repo.IHookTurnProgressRepo }

func TestHookLateWriteMissingCapabilityNeverFallsBackMySQL(t *testing.T) {
	for _, action := range []string{"turn", "start", "complete"} {
		t.Run(action, func(t *testing.T) {
			f := newHookLateWriteFixture(t)
			before := f.readTurn(t)
			b := f.ctx.Value(itemHookProgressContextKey{}).(itemHookProgressBinding)
			b.repo = hookLateProgressOnly{b.repo}
			f.ctx = context.WithValue(f.ctx, itemHookProgressContextKey{}, b)
			var err error
			switch action {
			case "turn":
				err = f.exec.storeTurnRunResult(f.ctx, f.etec, f.result(false))
			case "start":
				err = f.exec.SetItemRunProcessing(f.ctx, f.base.ExptID, f.base.ExptRunID, f.base.ItemID, f.base.SpaceID, nil)
			case "complete":
				err = f.exec.CompleteItemRun(f.ctx, f.etec.ExptItemEvalCtx, nil)
			}
			assert.True(t, itemHookControlOnly(err))
			assert.Equal(t, before, f.readTurn(t))
			var got model.ExptItemResultRunLog
			require.NoError(t, f.sql.Where("id=?", f.item.ID).First(&got).Error)
			assert.Equal(t, int32(entity.ItemRunState_Processing), got.Status)
			assert.Equal(t, int32(entity.ExptItemResultStateDefault), gptr.Indirect(got.ResultState))
		})
	}
}

func TestHookLateWriteItemOrdinaryAndRejectedMySQL(t *testing.T) {
	for _, action := range []string{"start", "success", "failure"} {
		for _, mode := range []string{"ordinary", "run", "version", "missing", "deleted"} {
			t.Run(action+"/"+mode, func(t *testing.T) {
				f := newHookLateWriteFixture(t)
				require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("id=?", f.item.ID).Updates(map[string]any{"status": int32(entity.ItemRunState_Queueing), "err_msg": []byte("prior error")}).Error)
				switch mode {
				case "run":
					f.etec.Event.ExptRunID++
				case "version":
					require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("id=?", f.item.ID).UpdateColumn("item_version_id", 8).Error)
				case "missing":
					require.NoError(t, f.sql.Unscoped().Where("id=?", f.item.ID).Delete(&model.ExptItemResultRunLog{}).Error)
				case "deleted":
					require.NoError(t, f.sql.Where("id=?", f.item.ID).Delete(&model.ExptItemResultRunLog{}).Error)
				}
				var err error
				if action == "start" {
					event := f.etec.Event
					err = f.exec.SetItemRunProcessing(f.ctx, event.ExptID, event.ExptRunID, event.EvalSetItemID, event.SpaceID, nil)
				} else {
					var evalErr error
					if action == "failure" {
						evalErr = errors.New("ordinary failure")
					}
					err = f.exec.CompleteItemRun(f.ctx, f.etec.ExptItemEvalCtx, evalErr)
				}
				if mode == "ordinary" {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
				}
				if mode == "missing" || mode == "deleted" {
					var count int64
					require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("id=?", f.item.ID).Count(&count).Error)
					assert.Zero(t, count)
					return
				}
				var got model.ExptItemResultRunLog
				require.NoError(t, f.sql.Where("id=?", f.item.ID).First(&got).Error)
				if mode != "ordinary" {
					assert.Equal(t, int32(entity.ItemRunState_Queueing), got.Status)
					assert.Equal(t, "prior error", string(gptr.Indirect(got.ErrMsg)))
					assert.Equal(t, int32(entity.ExptItemResultStateDefault), gptr.Indirect(got.ResultState))
					return
				}
				assert.Equal(t, int32(map[string]entity.ItemRunState{"start": entity.ItemRunState_Processing, "success": entity.ItemRunState_Success, "failure": entity.ItemRunState_Fail}[action]), got.Status)
				if action == "failure" {
					assert.Equal(t, "ordinary failure", string(gptr.Indirect(got.ErrMsg)))
				} else {
					assert.Equal(t, "prior error", string(gptr.Indirect(got.ErrMsg)))
				}
				if action == "start" {
					assert.Equal(t, int32(entity.ExptItemResultStateDefault), gptr.Indirect(got.ResultState))
				} else {
					assert.Equal(t, int32(entity.ExptItemResultStateLogged), gptr.Indirect(got.ResultState))
				}
			})
		}
	}
}

func TestHookLateWriteFreshTargetOwnsReferencesMySQL(t *testing.T) {
	f := newHookLateWriteFixture(t)
	require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("id=?", f.base.ID).Updates(map[string]any{"status": int32(entity.TurnRunState_Terminal), "err_msg": []byte("user terminated")}).Error)
	fresh := f.result(false)
	fresh.TargetResult.ID = 101
	require.NoError(t, f.exec.storeTurnRunResult(f.ctx, f.etec, fresh))
	got := f.readTurn(t)
	assert.Equal(t, int64(101), got.TargetResultID)
	assert.Equal(t, entity.TurnRunState_Terminal, got.Status)
	assert.False(t, itemHookProgressHasRecord(got.EvaluatorResultIds, 11), "new target must sever old target's scores")
	assert.True(t, itemHookProgressHasRecord(got.EvaluatorResultIds, 14))
	assert.True(t, itemHookProgressHasRecord(got.EvaluatorResultIds, 15))
	stale := f.result(false)
	stale.EvaluatorResults[0].ID = 16
	assert.Error(t, f.exec.storeTurnRunResult(f.ctx, f.etec, stale))
	assert.Equal(t, got, f.readTurn(t))
	require.NoError(t, f.exec.storeTurnRunResult(f.ctx, f.etec, fresh))
	assert.True(t, itemHookProgressHasRecord(f.readTurn(t).EvaluatorResultIds, 14))
}

func TestHookLateWriteOldTargetCannotCompleteReplacementMySQL(t *testing.T) {
	f := newHookLateWriteFixture(t)
	fresh := f.result(true)
	fresh.TargetResult.ID = 101
	require.NoError(t, f.exec.storeTurnRunResult(f.ctx, f.etec, fresh))
	before := f.readTurn(t)
	stale := &entity.ExptTurnRunResult{TargetResult: &entity.EvalTargetRecord{ID: 100}, EvaluatorResults: []*entity.EvaluatorRecord{{ID: 11, EvaluatorVersionID: 401, Status: entity.EvaluatorRunStatusSuccess}}}
	assert.Error(t, f.exec.storeTurnRunResult(f.ctx, f.etec, stale))
	assert.Equal(t, before, f.readTurn(t), "old target's success cannot complete the replacement target")
}

func TestHookLateWriteItemConcurrentTerminalAndDeleteMySQL(t *testing.T) {
	for _, deleted := range []bool{false, true} {
		t.Run(map[bool]string{false: "terminal", true: "delete"}[deleted], func(t *testing.T) {
			f := newHookLateWriteFixture(t)
			tx := f.sql.Begin()
			require.NoError(t, tx.Error)
			defer tx.Rollback()
			if deleted {
				require.NoError(t, tx.Where("id=?", f.item.ID).Delete(&model.ExptItemResultRunLog{}).Error)
			} else {
				require.NoError(t, tx.Model(&model.ExptItemResultRunLog{}).Where("id=?", f.item.ID).Updates(map[string]any{"status": int32(entity.ItemRunState_Terminal), "result_state": int32(entity.ExptItemResultStateResulted)}).Error)
			}
			done := make(chan error, 1)
			go func() { done <- f.exec.CompleteItemRun(f.ctx, f.etec.ExptItemEvalCtx, nil) }()
			select {
			case err := <-done:
				t.Fatalf("write bypassed locked item: %v", err)
			case <-time.After(40 * time.Millisecond):
			}
			require.NoError(t, tx.Commit().Error)
			if deleted {
				require.Error(t, <-done)
			} else {
				require.NoError(t, <-done)
			}
			var got model.ExptItemResultRunLog
			err := f.sql.Where("id=?", f.item.ID).First(&got).Error
			if deleted {
				assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
			} else {
				require.NoError(t, err)
				assert.Equal(t, int32(entity.ItemRunState_Terminal), got.Status)
				assert.Equal(t, int32(entity.ExptItemResultStateResulted), gptr.Indirect(got.ResultState))
			}
		})
	}
}

func TestHookLateWriteNoHookKeepsLegacyPersistenceMySQL(t *testing.T) {
	f := newHookLateWriteFixture(t)
	ctx := context.Background()
	require.NoError(t, f.exec.storeTurnRunResult(ctx, f.etec, f.result(false)))
	got := f.readTurn(t)
	assert.Equal(t, entity.TurnRunState_Success, got.Status)
	assert.Equal(t, f.etec.Ext, got.Ext)
	require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("id=?", f.item.ID).Updates(map[string]any{"status": int32(entity.ItemRunState_Terminal), "result_state": int32(entity.ExptItemResultStateResulted)}).Error)
	require.NoError(t, f.exec.CompleteItemRun(ctx, f.etec.ExptItemEvalCtx, nil))
	var item model.ExptItemResultRunLog
	require.NoError(t, f.sql.Where("id=?", f.item.ID).First(&item).Error)
	assert.Equal(t, int32(entity.ItemRunState_Terminal), item.Status)
	assert.Equal(t, int32(entity.ExptItemResultStateLogged), gptr.Indirect(item.ResultState), "noHook retains the legacy completion write")
}

func TestHookLateWriteAsyncAndExtConflictMySQL(t *testing.T) {
	f := newHookLateWriteFixture(t)
	async := f.result(false)
	async.AsyncAbort = true
	require.NoError(t, f.exec.storeTurnRunResult(f.ctx, f.etec, async))
	before := f.readTurn(t)
	assert.Equal(t, entity.TurnRunState_Processing, before.Status)
	assert.Equal(t, "real-output", before.Ext["output"])
	f.etec.Ext["output"] = "conflicting-stale-output"
	assert.Error(t, f.exec.storeTurnRunResult(f.ctx, f.etec, f.result(false)))
	assert.Equal(t, before, f.readTurn(t))
}

func newHookLateWriteFixture(t *testing.T) *hookLateWriteFixture {
	t.Helper()
	f := newExecutionHookFixture(t, true, false, false)
	f.etec.Expt.EvalConf = nil
	f.etec.Ext = map[string]string{"output": "real-output", "keep": "original"}
	row := f.etec.GetExistTurnResultRunLog(1)
	row.TargetResultID, row.TraceID, row.LogID = 100, 42, "original-log"
	row.Ext = map[string]string{"keep": "original"}
	row.EvaluatorResultIds = &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{401: 11}}
	_, sql := hookRecoveryStorage(t, f)
	require.NoError(t, sql.Model(&model.ExptTurnResultRunLog{}).Where("id=?", row.ID).UpdateColumn("trace_id", 42).Error)
	p, err := db.NewDB(mysql.Open(os.Getenv("HOOK_MYSQL_TX_DSN")), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	pool, err := p.NewSession(context.Background()).DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	config := configmocks.NewMockIConfiger(gomock.NewController(t))
	config.EXPECT().GetErrCtrl(gomock.Any()).Return(entity.DefaultExptErrCtrl()).AnyTimes()
	config.EXPECT().GetErrRetryConf(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.RetryConf{}).AnyTimes()
	f.etec.Event.MaxRetryTimes = 0
	exec := &ExptItemEvalCtxExecutor{TurnResultRepo: store.NewExptTurnResultRepo(nil, dao.NewExptTurnResultDAO(p), nil), ItemResultRepo: store.NewExptItemResultRepo(dao.NewExptItemResultDAO(p)), Configer: config}
	item := &model.ExptItemResultRunLog{ID: hookRecoveryIDs.Add(1), SpaceID: row.SpaceID, ExptID: row.ExptID, ExptRunID: row.ExptRunID, ItemID: row.ItemID, ItemVersionID: 7, Status: int32(entity.ItemRunState_Processing), ResultState: gptr.Of(int32(entity.ExptItemResultStateDefault))}
	require.NoError(t, sql.Create(item).Error)
	t.Cleanup(func() {
		require.NoError(t, sql.Unscoped().Where("id=?", item.ID).Delete(&model.ExptItemResultRunLog{}).Error)
	})
	return &hookLateWriteFixture{executionHookFixture: f, sql: sql, exec: exec, base: row, item: item}
}

func (f *hookLateWriteFixture) readTurn(t *testing.T) *entity.ExptTurnResultRunLog {
	t.Helper()
	var po model.ExptTurnResultRunLog
	require.NoError(t, f.sql.Where("id=?", f.base.ID).First(&po).Error)
	row, err := convert.NewExptTurnResultRunLogConvertor().PO2DO(&po)
	require.NoError(t, err)
	row.TraceID = po.TraceID
	return row
}

func (f *hookLateWriteFixture) result(failed bool) *entity.ExptTurnRunResult {
	r := &entity.ExptTurnRunResult{TargetResult: &entity.EvalTargetRecord{ID: 100}, EvaluatorResults: []*entity.EvaluatorRecord{{ID: 14, EvaluatorVersionID: 402, Alias: "new", Status: entity.EvaluatorRunStatusSuccess}, {ID: 15, InlineKey: "inline-new", SourceType: entity.EvaluatorRecordSourceTypeInline, Status: entity.EvaluatorRunStatusSuccess}}}
	if failed {
		r.EvalErr = errors.New("real execution failure")
	}
	return r
}

// Removing the managed persistence route must overwrite Terminal and lose disjoint refs.
func TestHookLateWriteTurnTerminalRetainsResultsMySQL(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[failed], func(t *testing.T) {
			f := newHookLateWriteFixture(t)
			refs, err := json.Marshal(&entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{401: 11}, Registered: []*entity.RegisteredEvalResult{{VersionID: 403, Alias: "concurrent", RecordID: 12}}, Inline: []*entity.InlineEvalResult{{InlineKey: "existing", RecordID: 13}}})
			require.NoError(t, err)
			tx := f.sql.Begin()
			require.NoError(t, tx.Error)
			defer tx.Rollback()
			require.NoError(t, tx.Model(&model.ExptTurnResultRunLog{}).Where("id=?", f.base.ID).Updates(map[string]any{"status": int32(entity.TurnRunState_Terminal), "err_msg": []byte("user terminated"), "evaluator_result_ids": refs, "ext": []byte(`{"keep":"original","concurrent":"saved"}`)}).Error)
			done := make(chan error, 1)
			result := f.result(failed)
			go func() { done <- f.exec.storeTurnRunResult(f.ctx, f.etec, result) }()
			select {
			case err := <-done:
				t.Fatalf("write bypassed locked turn: %v", err)
			case <-time.After(40 * time.Millisecond):
			}
			require.NoError(t, tx.Commit().Error)
			require.NoError(t, <-done)
			got := f.readTurn(t)
			assert.Equal(t, entity.TurnRunState_Terminal, got.Status)
			assert.Equal(t, "user terminated", got.ErrMsg)
			assert.Equal(t, int64(100), got.TargetResultID)
			assert.Equal(t, int64(42), got.TraceID)
			assert.Equal(t, "original-log", got.LogID)
			assert.Equal(t, map[string]string{"keep": "original", "concurrent": "saved", "output": "real-output"}, got.Ext)
			assert.Equal(t, int64(11), got.EvaluatorResultIds.EvalVerIDToResID[401])
			assert.Contains(t, got.EvaluatorResultIds.Registered, &entity.RegisteredEvalResult{VersionID: 403, Alias: "concurrent", RecordID: 12})
			assert.Contains(t, got.EvaluatorResultIds.Registered, &entity.RegisteredEvalResult{VersionID: 402, Alias: "new", RecordID: 14})
			assert.Contains(t, got.EvaluatorResultIds.Inline, &entity.InlineEvalResult{InlineKey: "existing", RecordID: 13})
			assert.Contains(t, got.EvaluatorResultIds.Inline, &entity.InlineEvalResult{InlineKey: "inline-new", RecordID: 15})
			assert.Equal(t, failed, result.EvalErr != nil)
		})
	}
}

func TestHookLateWriteOrdinaryManagedResultsMySQL(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[failed], func(t *testing.T) {
			f := newHookLateWriteFixture(t)
			result := f.result(failed)
			require.NoError(t, f.exec.storeTurnRunResult(f.ctx, f.etec, result))
			got := f.readTurn(t)
			assert.Equal(t, map[bool]entity.TurnRunState{false: entity.TurnRunState_Success, true: entity.TurnRunState_Fail}[failed], got.Status)
			assert.Equal(t, failed, got.ErrMsg != "")
			assert.Equal(t, "real-output", got.Ext["output"])
			assert.Equal(t, int64(100), got.TargetResultID)
			assert.Contains(t, got.EvaluatorResultIds.Inline, &entity.InlineEvalResult{InlineKey: "inline-new", RecordID: 15})
		})
	}
}

func TestHookLateWriteRejectsWrongIdentityAndDeletedMySQL(t *testing.T) {
	for _, mode := range []string{"run", "turn", "version", "binding-version", "missing", "deleted"} {
		t.Run(mode, func(t *testing.T) {
			f := newHookLateWriteFixture(t)
			before := f.readTurn(t)
			switch mode {
			case "run":
				f.etec.Event.ExptRunID++
			case "turn":
				f.base.TurnID++
			case "version":
				f.base.ItemVersionID++
			case "binding-version":
				b := f.ctx.Value(itemHookProgressContextKey{}).(itemHookProgressBinding)
				b.itemVersion++
				f.ctx = context.WithValue(f.ctx, itemHookProgressContextKey{}, b)
			case "missing":
				require.NoError(t, f.sql.Unscoped().Where("id=?", f.base.ID).Delete(&model.ExptTurnResultRunLog{}).Error)
			case "deleted":
				require.NoError(t, f.sql.Where("id=?", f.base.ID).Delete(&model.ExptTurnResultRunLog{}).Error)
			}
			assert.Error(t, f.exec.storeTurnRunResult(f.ctx, f.etec, f.result(false)))
			if mode == "missing" || mode == "deleted" {
				var live int64
				require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("id=?", f.base.ID).Count(&live).Error)
				assert.Zero(t, live)
			} else {
				assert.Equal(t, before, f.readTurn(t))
			}
		})
	}
}

func TestHookLateWriteItemTerminalMySQL(t *testing.T) {
	for _, action := range []string{"start", "success", "failure"} {
		t.Run(action, func(t *testing.T) {
			f := newHookLateWriteFixture(t)
			require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("id=?", f.item.ID).Updates(map[string]any{"status": int32(entity.ItemRunState_Terminal), "result_state": int32(entity.ExptItemResultStateResulted), "err_msg": []byte("user terminated")}).Error)
			if action == "start" {
				err := f.exec.SetItemRunProcessing(f.ctx, f.base.ExptID, f.base.ExptRunID, f.base.ItemID, f.base.SpaceID, nil)
				assert.True(t, itemHookControlOnly(err), "terminal start must stop execution")
			} else {
				var evalErr error
				if action == "failure" {
					evalErr = errors.New("late failure")
				}
				require.NoError(t, f.exec.CompleteItemRun(f.ctx, f.etec.ExptItemEvalCtx, evalErr))
			}
			var got model.ExptItemResultRunLog
			require.NoError(t, f.sql.Where("id=?", f.item.ID).First(&got).Error)
			assert.Equal(t, int32(entity.ItemRunState_Terminal), got.Status)
			assert.Equal(t, int32(entity.ExptItemResultStateResulted), gptr.Indirect(got.ResultState))
			assert.Equal(t, "user terminated", string(gptr.Indirect(got.ErrMsg)))
		})
	}
}
