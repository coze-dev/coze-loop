// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func resultReadMySQLFixture(t *testing.T) (*hookTxFixture, entity.HookRunKey) {
	t.Helper()
	f := newHookTxFixture(t)
	in := f.input(true, 0)
	_, err := f.repo.CreateRunWithHooks(context.Background(), in)
	require.NoError(t, err)
	return f, in.Key
}

func resultReadMySQLRow(t *testing.T, f *hookTxFixture, table string, space, expt, run int64, status int32, deleted bool) {
	t.Helper()
	id, item := hookTxSequence.Add(1), hookTxSequence.Add(1)
	var deletedAt gorm.DeletedAt
	if deleted {
		deletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
	}
	var row any
	if table == "item" {
		row = &model.ExptItemResult{ID: id, SpaceID: space, ExptID: expt, ExptRunID: run, ItemID: item, Status: status, DeletedAt: deletedAt}
	} else {
		row = &model.ExptTurnResult{ID: id, SpaceID: space, ExptID: expt, ExptRunID: run, ItemID: item, TurnID: hookTxSequence.Add(1), Status: status, DeletedAt: deletedAt}
	}
	// Registered after the Hook fixture: exact base rows are removed before it closes the pool.
	t.Cleanup(func() {
		require.NoError(t, f.sql.Unscoped().Where("id=? AND space_id=? AND expt_id=?", id, space, expt).Delete(row).Error)
	})
	require.NoError(t, f.sql.Create(row).Error)
}

func TestHookPlanResultReaderMySQLAnyStatus(t *testing.T) {
	for _, table := range []string{"item", "turn"} {
		for _, state := range []struct {
			name       string
			item, turn int32
		}{
			{"zero", 0, 0},
			{"success", int32(entity.ItemRunState_Success), int32(entity.TurnRunState_Success)},
			{"failure", int32(entity.ItemRunState_Fail), int32(entity.TurnRunState_Fail)},
			{"unknown", 987654, 987654},
		} {
			t.Run(table+"/"+state.name, func(t *testing.T) {
				f, key := resultReadMySQLFixture(t)
				reader := NewHookPlanResultReader(f.p)
				got, err := reader.HasExperimentResults(context.Background(), key, "local")
				require.NoError(t, err)
				require.False(t, got)
				status := state.item
				if table == "turn" {
					status = state.turn
				}
				resultReadMySQLRow(t, f, table, f.space, f.expt, key.RunID, status, false)
				got, err = reader.HasExperimentResults(context.Background(), key, "local")
				require.NoError(t, err)
				require.True(t, got)
			})
		}
	}
}

func TestHookPlanResultReaderMySQLIgnoresDeletedAndForeignRows(t *testing.T) {
	f, key := resultReadMySQLFixture(t)
	foreignSpace, foreignExpt := hookTxSequence.Add(1), hookTxSequence.Add(1)
	for _, table := range []string{"item", "turn"} {
		resultReadMySQLRow(t, f, table, foreignSpace, f.expt, key.RunID, 987654, false)
		resultReadMySQLRow(t, f, table, f.space, foreignExpt, key.RunID, 987654, false)
		resultReadMySQLRow(t, f, table, f.space, f.expt, key.RunID, 987654, true)
	}
	got, err := NewHookPlanResultReader(f.p).HasExperimentResults(context.Background(), key, "local")
	require.NoError(t, err)
	require.False(t, got)
}

func TestHookPlanResultReaderMySQLHistoricalRunSeesCurrentBase(t *testing.T) {
	f, source := resultReadMySQLFixture(t)
	newer := f.input(false, source.RunID)
	_, err := f.repo.CreateRunWithHooks(context.Background(), newer)
	require.NoError(t, err)
	require.Equal(t, newer.Key.RunID, f.latest(t))
	reader := NewHookPlanResultReader(f.p)
	got, err := reader.HasExperimentResults(context.Background(), source, "local")
	require.NoError(t, err)
	require.False(t, got)
	resultReadMySQLRow(t, f, "turn", f.space, f.expt, newer.Key.RunID, 987654, false)
	got, err = reader.HasExperimentResults(context.Background(), source, "local")
	require.NoError(t, err)
	require.True(t, got)
}

func TestHookPlanResultReaderMySQLRejectsScopeAndMarker(t *testing.T) {
	f, key := resultReadMySQLFixture(t)
	reader := NewHookPlanResultReader(f.p)
	got, err := reader.HasExperimentResults(context.Background(), key, "foreign")
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	require.False(t, got)
	foreign := key
	foreign.WorkspaceID = hookTxSequence.Add(1)
	got, err = reader.HasExperimentResults(context.Background(), foreign, "local")
	require.ErrorIs(t, err, entity.ErrHookStoreMissing)
	require.False(t, got)
	for _, marker := range []int32{0, 2} {
		require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=? AND space_id=? AND expt_id=?", key.RunID, f.space, f.expt).UpdateColumn("lifecycle_hook_version", marker).Error)
		got, err = reader.HasExperimentResults(context.Background(), key, "local")
		require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
		require.False(t, got)
	}
}
