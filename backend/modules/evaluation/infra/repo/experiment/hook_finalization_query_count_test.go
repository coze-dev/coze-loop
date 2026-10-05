// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func TestHookFinalizationResultReadsScaleWithPages(t *testing.T) {
	for _, count := range []int{1, 101, 201} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			f := finalizationQueryFixture(t, count)
			ctx := context.Background()
			var reads atomic.Int64
			require.NoError(t, f.sql.Callback().Query().After("gorm:query").Register("finalization_result_query_count", func(tx *gorm.DB) {
				if (tx.Statement.Table == "expt_item_result_run_log" || tx.Statement.Table == "expt_turn_result_run_log") && !strings.Contains(strings.ToUpper(tx.Statement.SQL.String()), "COUNT(") {
					reads.Add(1)
				}
			}))
			stats, err := NewHookFinalizationRepo(f.p).ReadFinalizationStats(ctx, f.key, "local")
			require.NoError(t, err)
			require.Equal(t, entity.HookFinalizationCounts{Success: int32(count)}, stats.Items)
			require.Equal(t, entity.HookFinalizationCounts{Success: int32(count + 1)}, stats.Turns)
			require.Len(t, stats.ItemIDs, count)
			expected := int64(2 * ((count + 99) / 100))
			t.Logf("members=%d standalone_result_reads=%d expected=%d", count, reads.Load(), expected)
			require.Equal(t, expected, reads.Load(), "one item and one turn read per bounded manifest page")
			state, err := f.repo.GetRun(ctx, f.key)
			require.NoError(t, err)
			in := entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: state.Version}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Success}, Stats: stats}
			begun, err := f.repo.BeginFinalize(ctx, in)
			require.NoError(t, err)
			in.ExpectedVersion = begun.Run.Version
			reads.Store(0)
			result, err := f.repo.CommitFinalize(ctx, in)
			require.NoError(t, err)
			require.True(t, result.Effects.ActivateAfter)
			t.Logf("members=%d commit_result_reads=%d expected=%d", count, reads.Load(), expected)
			require.Equal(t, expected, reads.Load(), "commit-time revalidation must stay page-scaled")
		})
	}
}

func finalizationQueryFixture(t *testing.T, count int) *executionFixture {
	t.Helper()
	f := settledFinalizationFixtureCount(t, 1)
	for i := 1; i < count; i++ {
		item := entity.HookPlanItem{ID: hookTxSequence.Add(1), SourceSpaceID: f.space, EvalSetID: 71, ItemID: hookTxSequence.Add(1), ItemVersionID: int64(i%2) * 42}
		m := entity.HookExecutionManifest{Version: 1, Key: f.key, Ordinal: int64(i), Frozen: item, ItemResultID: hookTxSequence.Add(1), ItemRunLogID: hookTxSequence.Add(1), Turns: []entity.HookExecutionTurnManifest{{TurnID: 0, ResultID: hookTxSequence.Add(1)}}}
		raw, err := json.Marshal(m)
		require.NoError(t, err)
		require.NoError(t, f.sql.Create(&model.ExptLifecycleRunItem{ID: item.ID, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, Ordinal: int64(i), SourceSpaceID: f.space, EvalSetID: 71, ItemID: item.ItemID, ItemVersionID: item.ItemVersionID, ExecutionManifest: &raw}).Error)
		require.NoError(t, f.sql.Create(&model.ExptTurnResult{ID: m.Turns[0].ResultID, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: item.ItemID, ItemVersionID: item.ItemVersionID, TurnIdx: gptr.Of(int32(0)), Status: int32(entity.TurnRunState_Success)}).Error)
		require.NoError(t, f.sql.Create(&model.ExptItemResult{ID: m.ItemResultID, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: item.ItemID, ItemVersionID: item.ItemVersionID, ItemIdx: gptr.Of(int32(i)), Status: int32(entity.ItemRunState_Success)}).Error)
		require.NoError(t, f.sql.Create(&model.ExptTurnResultRunLog{ID: hookTxSequence.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: item.ItemID, ItemVersionID: item.ItemVersionID, Status: int32(entity.TurnRunState_Success)}).Error)
		require.NoError(t, f.sql.Create(&model.ExptItemResultRunLog{ID: m.ItemRunLogID, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: item.ItemID, ItemVersionID: item.ItemVersionID, Status: int32(entity.ItemRunState_Success), ResultState: gptr.Of(int32(entity.ExptItemResultStateResulted))}).Error)
		f.manifests = append(f.manifests, m)
	}
	digest := entity.NewHookPlanDigest()
	for start := 0; start < len(f.manifests); start += 100 {
		items := []entity.HookPlanItem{}
		for _, m := range f.manifests[start:min(start+100, len(f.manifests))] {
			items = append(items, m.Frozen)
		}
		var err error
		digest, err = entity.AppendHookPlanDigest(digest, items)
		require.NoError(t, err)
	}
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumns(map[string]any{"plan_count": count, "plan_hash": digest.Hash}).Error)
	f.hash = digest.Hash
	return f
}
