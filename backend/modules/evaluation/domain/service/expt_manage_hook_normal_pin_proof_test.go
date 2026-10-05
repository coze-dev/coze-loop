// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func normalPinnedFixture(t *testing.T) (*finalizationManagerFixture, entity.HookExecutionManifest, *entity.HookFinalizationStats) {
	t.Helper()
	f, manifests := activeTerminationFixture(t, entity.ItemRunState_Queueing)
	m := manifests[0]
	ctx, eiec, pre := admitLazyTurn(t, f, m)
	require.NoError(t, pre.PreEval(ctx, eiec))
	var ledger model.ExptLifecycleRunItem
	require.NoError(t, f.sql.First(&ledger, m.Frozen.ID).Error)
	require.NoError(t, json.Unmarshal(gptr.Indirect(ledger.ExecutionManifest), &m))
	require.True(t, gptr.Indirect(m.TurnLogsInitialized))
	base := lateProofTurn(t, f)
	next := *base
	next.Status = entity.TurnRunState_Success
	writer := ctx.Value(itemHookProgressContextKey{}).(itemHookProgressBinding).repo
	_, err := writer.(repo.IHookTurnResultWriteRepo).WriteTurnResult(ctx, entity.HookTurnProgressInput{Base: base, Progress: &next})
	require.NoError(t, err)
	_, err = writer.(repo.IHookItemRunWriteRepo).WriteItemRun(ctx, entity.HookItemRunWriteInput{HookRunKey: f.key, ItemID: m.Frozen.ItemID, ItemVersionID: m.Frozen.ItemVersionID, Status: entity.ItemRunState_Success})
	require.NoError(t, err)
	result, err := (&ExptResultServiceImpl{}).WithHookArchive(f.deps.Repository.(repo.IHookItemArchiveRepo), "local")
	require.NoError(t, err)
	source, err := f.deps.readSource(ctx, f.key)
	require.NoError(t, err)
	_, err = result.RecordItemRunLogs(ctx, f.expt, f.key.RunID, m.Frozen.ItemID, f.space, source.Experiment)
	require.NoError(t, err)
	stats, err := f.deps.readStats(ctx, f.key)
	require.NoError(t, err)
	return f, m, stats
}

// Same-count replacement logs must not satisfy a manifest that names different records.
func TestHookNormalFinalizeValidatesPinnedExecutionSetMySQL(t *testing.T) {
	for _, kind := range []string{"valid", "legacy", "false-receipt", "missing", "deleted", "replaced", "foreign-replaced"} {
		for _, newer := range []bool{false, true} {
			for _, entry := range []string{"manager", "commit"} {
				t.Run(fmt.Sprintf("%s/newer=%t/%s", kind, newer, entry), func(t *testing.T) {
					f, m, stats := normalPinnedFixture(t)
					ctx := context.Background()
					oldID := m.Turns[0].RunLogID
					var old model.ExptTurnResultRunLog
					require.NoError(t, f.sql.First(&old, oldID).Error)
					checkLatest := func() {}
					if newer {
						checkLatest = lateProofSupersede(t, f)
					}
					switch kind {
					case "legacy", "false-receipt":
						m.TurnLogsInitialized = nil
						if kind == "false-receipt" {
							m.TurnLogsInitialized = gptr.Of(false)
						}
						m.Turns[0].RunLogID = 0
						raw, err := json.Marshal(m)
						require.NoError(t, err)
						require.NoError(t, f.sql.Model(&model.ExptLifecycleRunItem{}).Where("id=?", m.Frozen.ID).UpdateColumn("execution_manifest", raw).Error)
					case "missing", "replaced":
						require.NoError(t, f.sql.Unscoped().Delete(&model.ExptTurnResultRunLog{}, oldID).Error)
					case "deleted":
						require.NoError(t, f.sql.Delete(&model.ExptTurnResultRunLog{}, oldID).Error)
					case "foreign-replaced":
						require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("id=?", oldID).UpdateColumn("space_id", f.space+1).Error)
						t.Cleanup(func() { require.NoError(t, f.sql.Unscoped().Delete(&model.ExptTurnResultRunLog{}, oldID).Error) })
					}
					if kind == "replaced" || kind == "foreign-replaced" {
						old.ID = finalizationTestIDs.Add(1)
						require.NoError(t, f.sql.Create(&old).Error)
					}
					valid := kind == "valid" || kind == "legacy"
					_, statsErr := f.deps.readStats(ctx, f.key)
					if valid {
						require.NoError(t, statsErr)
					} else {
						assert.Error(t, statsErr, "normal statistics cannot certify contradictory pins or receipt")
					}
					var finishErr error
					if entry == "manager" {
						finishErr = f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{})
					} else {
						state := finalizationRead(t, f)
						intent := entity.HookTerminalIntent{Status: stats.NormalStatus()}
						begun, err := f.repo.BeginFinalize(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: state.Version}, Intent: intent})
						require.NoError(t, err)
						_, finishErr = f.repo.CommitFinalize(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: begun.Run.Version}, Intent: intent, Stats: stats})
					}
					if valid {
						require.NoError(t, finishErr)
						assert.True(t, finalizationRead(t, f).State.After.Activated)
					} else {
						assert.Error(t, finishErr, "stale pre-fault statistics cannot authorize after")
						assert.False(t, finalizationRead(t, f).State.After.Activated)
					}
					checkLatest()
				})
			}
		}
	}
}
