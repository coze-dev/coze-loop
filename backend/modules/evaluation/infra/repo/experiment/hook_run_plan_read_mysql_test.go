// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestHookPlanStorageMySQLPrefixAndCursor(t *testing.T) {
	f := newHookTxFixture(t)
	ctx := context.Background()
	in := f.input(true, 0)
	created, err := f.repo.CreateRunWithHooks(ctx, in)
	require.NoError(t, err)
	r := NewHookPlanRepo(f.p)
	advance := entity.HookAdvancePlanInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: created.Run.Version}, ExecutionScope: "local", NextCursor: "empty-page"}
	advanced, err := r.AdvancePlanCursor(ctx, advance)
	require.NoError(t, err)
	require.True(t, advanced.Changed)
	require.Equal(t, int64(1), advanced.Run.Version)
	require.Zero(t, advanced.Run.PlanCount)
	require.Empty(t, advanced.Run.PlanHash)
	require.Equal(t, created.Run.Operations, advanced.Run.Operations)
	require.Equal(t, created.Run.State, advanced.Run.State)
	replay, err := r.AdvancePlanCursor(ctx, advance)
	require.NoError(t, err)
	require.False(t, replay.Changed)
	require.Equal(t, advanced.Run, replay.Run)
	read := entity.HookPlanReadInput{Key: in.Key, ExecutionScope: "local", Limit: 1}
	empty, err := r.ReadPlanPage(ctx, read)
	require.NoError(t, err)
	require.NotNil(t, empty.Items)
	require.Empty(t, empty.Items)
	require.False(t, empty.Ready)
	p := hookTestPage(in.Key, 1, 0, "empty-page", "selected", 2)
	_, err = f.repo.AppendPlanPage(ctx, p)
	require.NoError(t, err)
	first, err := r.ReadPlanPage(ctx, read)
	require.NoError(t, err)
	require.Equal(t, []entity.HookPlanItem{p.Items[0]}, first.Items)
	require.Equal(t, int64(1), first.NextOrdinal)
	require.True(t, first.HasMore)
	advance.ExpectedVersion = 2
	advance.ExpectedCount = 2
	advance.Cursor = "selected"
	advance.NextCursor = "verify"
	advanced, err = r.AdvancePlanCursor(ctx, advance)
	require.NoError(t, err)
	require.Equal(t, int64(3), advanced.Run.Version)
	require.Equal(t, int64(2), advanced.Run.PlanCount)
	require.False(t, advanced.Run.State.Before.Activated)
	hash := strings.Repeat("c", 64)
	finished, err := f.repo.FinishPlan(ctx, entity.HookFinishPlanInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: 3}, Count: 2, Hash: hash})
	require.NoError(t, err)
	later := model.ExptLifecycleRunItem{ID: hookTxSequence.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: in.Key.RunID, Ordinal: 2, SourceSpaceID: f.space, EvalSetID: 8, ItemID: hookTxSequence.Add(1)}
	require.NoError(t, f.sql.Create(&later).Error)
	newer := f.input(false, in.Key.RunID)
	_, err = f.repo.CreateRunWithHooks(ctx, newer)
	require.NoError(t, err)
	read.Limit = 100
	frozen, err := r.ReadPlanPage(ctx, read)
	require.NoError(t, err)
	require.Equal(t, p.Items, frozen.Items)
	require.Equal(t, int64(2), frozen.Count)
	require.Equal(t, int64(2), frozen.NextOrdinal)
	require.False(t, frozen.HasMore)
	require.True(t, frozen.Ready)
	require.Equal(t, hash, frozen.Hash)
	require.Equal(t, finished.Run.Version, frozen.RunVersion)
	read.StartOrdinal = 2
	end, err := r.ReadPlanPage(ctx, read)
	require.NoError(t, err)
	require.Empty(t, end.Items)
	require.False(t, end.HasMore)
	require.Equal(t, int64(2), end.NextOrdinal)
	read.ExecutionScope = "foreign"
	_, err = r.ReadPlanPage(ctx, read)
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	advance.ExpectedVersion = finished.Run.Version
	advance.Cursor = "verify"
	advance.NextCursor = "must-not-advance"
	_, err = r.AdvancePlanCursor(ctx, advance)
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	var retained int64
	require.NoError(t, hookRunScope(f.sql.Model(&model.ExptLifecycleRunItem{}), in.Key).Count(&retained).Error)
	require.Equal(t, int64(3), retained)
}

func TestHookPlanStorageMySQLCorruptOrdinal(t *testing.T) {
	for _, ordinal := range []int64{0, 2} {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			f := newHookTxFixture(t)
			ctx := context.Background()
			in := f.input(true, 0)
			_, err := f.repo.CreateRunWithHooks(ctx, in)
			require.NoError(t, err)
			p := hookTestPage(in.Key, 0, 0, "", "selected", 2)
			_, err = f.repo.AppendPlanPage(ctx, p)
			require.NoError(t, err)
			require.NoError(t, hookRunScope(f.sql.Model(&model.ExptLifecycleRunItem{}), in.Key).Where("id=?", p.Items[1].ID).UpdateColumn("ordinal", ordinal).Error)
			page, err := NewHookPlanRepo(f.p).ReadPlanPage(ctx, entity.HookPlanReadInput{Key: in.Key, ExecutionScope: "local", Limit: 2})
			require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
			require.Nil(t, page)
		})
	}
}

func TestHookPlanStorageMySQLAdvanceRollbackAndCancel(t *testing.T) {
	for _, cancelRun := range []bool{false, true} {
		t.Run(fmt.Sprint(cancelRun), func(t *testing.T) {
			f := newHookTxFixture(t)
			ctx := context.Background()
			in := f.input(true, 0)
			created, err := f.repo.CreateRunWithHooks(ctx, in)
			require.NoError(t, err)
			before := created.Run
			want := entity.ErrHookPlanStorage
			writes := 0
			if cancelRun {
				cancelled, err := f.repo.BeginFinalize(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: before.Version}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated, Reason: "cancel"}})
				require.NoError(t, err)
				before = cancelled.Run
				want = entity.ErrHookStoreConflict
			} else {
				name := fmt.Sprintf("plan_cursor_rollback_%d", f.expt)
				require.NoError(t, f.sql.Callback().Update().After("gorm:update").Register(name, func(tx *gorm.DB) {
					if tx.Error == nil && tx.Statement.Table == model.TableNameExptLifecycleRun {
						writes++
						tx.AddError(errors.New("fixture cursor write failure"))
					}
				}))
				t.Cleanup(func() { require.NoError(t, f.sql.Callback().Update().Remove(name)) })
			}
			out, err := NewHookPlanRepo(f.p).AdvancePlanCursor(ctx, entity.HookAdvancePlanInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: before.Version}, ExecutionScope: "local", NextCursor: "verify"})
			require.ErrorIs(t, err, want)
			require.Equal(t, entity.HookStoreResult{}, out)
			if !cancelRun {
				require.Equal(t, 1, writes)
			}
			after, err := f.repo.GetRun(ctx, in.Key)
			require.NoError(t, err)
			require.Equal(t, before, after)
		})
	}
}
