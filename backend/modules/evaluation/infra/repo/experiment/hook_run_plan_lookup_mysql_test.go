// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"strings"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

func TestHookPlanLookupMySQLCrossPageMembership(t *testing.T) {
	f := newHookTxFixture(t)
	ctx := context.Background()
	in := f.input(true, 0)
	_, err := f.repo.CreateRunWithHooks(ctx, in)
	require.NoError(t, err)
	p := hookTestPage(in.Key, 0, 0, "", "page-1", 2)
	_, err = f.repo.AppendPlanPage(ctx, p)
	require.NoError(t, err)
	missing := hookTxSequence.Add(1)
	query := entity.HookPlanLookupInput{Key: in.Key, ExecutionScope: "local", ItemIDs: []int64{p.Items[1].ItemID, missing, p.Items[0].ItemID}}
	r := NewHookPlanRepo(f.p)
	got, err := r.MGetPlanItems(ctx, query)
	require.NoError(t, err)
	require.Equal(t, []entity.HookPlanItem{p.Items[1], p.Items[0]}, got.Items)
	require.Equal(t, int64(2), got.Count)
	require.Equal(t, int64(1), got.RunVersion)
	require.False(t, got.Ready)
	// The caller appends only the missing candidate from the next turn page.
	next := hookTestPage(in.Key, got.RunVersion, got.Count, "page-1", "page-2", 1)
	next.Items[0].ItemID = missing
	_, err = f.repo.AppendPlanPage(ctx, next)
	require.NoError(t, err)
	before, err := f.repo.GetRun(ctx, in.Key)
	require.NoError(t, err)
	got, err = r.MGetPlanItems(ctx, query)
	require.NoError(t, err)
	require.Equal(t, []entity.HookPlanItem{p.Items[1], next.Items[0], p.Items[0]}, got.Items)
	require.Equal(t, int64(3), got.Count)
	require.Equal(t, int64(2), got.RunVersion)
	after, err := f.repo.GetRun(ctx, in.Key)
	require.NoError(t, err)
	require.Equal(t, before, after)
	var count int64
	require.NoError(t, hookRunScope(f.sql.Model(&model.ExptLifecycleRunItem{}), in.Key).Count(&count).Error)
	require.Equal(t, int64(3), count)
}

func TestHookPlanLookupMySQLPreparingAndFrozenLedger(t *testing.T) {
	f := newHookTxFixture(t)
	ctx := context.Background()
	in := f.input(true, 0)
	_, err := f.repo.CreateRunWithHooks(ctx, in)
	require.NoError(t, err)
	p := hookTestPage(in.Key, 0, 0, "", "page-1", 1)
	appended, err := f.repo.AppendPlanPage(ctx, p)
	require.NoError(t, err)
	extra := model.ExptLifecycleRunItem{ID: hookTxSequence.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: in.Key.RunID, Ordinal: 1, SourceSpaceID: f.space, EvalSetID: 8, ItemID: hookTxSequence.Add(1)}
	require.NoError(t, f.sql.Create(&extra).Error)
	r := NewHookPlanRepo(f.p)
	query := entity.HookPlanLookupInput{Key: in.Key, ExecutionScope: "local", ItemIDs: []int64{extra.ItemID, p.Items[0].ItemID}}
	got, err := r.MGetPlanItems(ctx, query)
	require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
	require.Nil(t, got)
	// Repair only this fixture's extra row, freeze normally, then model later ledger growth.
	require.NoError(t, hookRunScope(f.sql, in.Key).Where("id=?", extra.ID).Delete(&model.ExptLifecycleRunItem{}).Error)
	_, err = f.repo.FinishPlan(ctx, entity.HookFinishPlanInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: appended.Run.Version}, Count: 1, Hash: strings.Repeat("a", 64)})
	require.NoError(t, err)
	require.NoError(t, f.sql.Create(&extra).Error)
	newer := f.input(false, in.Key.RunID)
	_, err = f.repo.CreateRunWithHooks(ctx, newer)
	require.NoError(t, err)
	got, err = r.MGetPlanItems(ctx, query)
	require.NoError(t, err)
	require.Equal(t, p.Items, got.Items)
	require.Equal(t, int64(1), got.Count)
	require.True(t, got.Ready)
	query.ExecutionScope = "foreign"
	got, err = r.MGetPlanItems(ctx, query)
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	require.Nil(t, got)
}
