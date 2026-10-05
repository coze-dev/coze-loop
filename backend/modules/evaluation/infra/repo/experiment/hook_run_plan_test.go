// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

func hookTestPage(key entity.HookRunKey, version, ordinal int64, cursor, next string, count int) entity.HookPlanPageInput {
	p := entity.HookPlanPageInput{HookStoreGuard: entity.HookStoreGuard{Key: key, ExpectedVersion: version}, StartOrdinal: ordinal, Cursor: cursor, NextCursor: next}
	for i := 0; i < count; i++ {
		p.Items = append(p.Items, entity.HookPlanItem{ID: hookTxSequence.Add(1), SourceSpaceID: key.WorkspaceID, EvalSetID: 8, ItemID: hookTxSequence.Add(1)})
	}
	return p
}

func TestHookTxPlanPagesAndFreeze(t *testing.T) {
	f := newHookTxFixture(t)
	ctx := context.Background()
	in := f.input(true, 0)
	_, err := f.repo.CreateRunWithHooks(ctx, in)
	require.NoError(t, err)
	p := hookTestPage(in.Key, 0, 0, "", "page-1", 2)
	first, err := f.repo.AppendPlanPage(ctx, p)
	require.NoError(t, err)
	require.True(t, first.Changed)
	require.Equal(t, int64(2), first.Run.PlanCount)
	require.Equal(t, "page-1", first.Run.PlanCursor)
	require.Equal(t, int64(1), first.Run.Version)
	require.False(t, first.Run.State.Before.Activated)
	replay := p
	replay.Items = append([]entity.HookPlanItem(nil), p.Items...)
	for i := range replay.Items {
		replay.Items[i].ID = hookTxSequence.Add(1)
	}
	again, err := f.repo.AppendPlanPage(ctx, replay)
	require.NoError(t, err)
	require.False(t, again.Changed)
	require.Equal(t, first.Run, again.Run)
	wrong := hookTestPage(in.Key, 0, 2, "page-1", "page-2", 1)
	_, err = f.repo.AppendPlanPage(ctx, wrong)
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	finish := entity.HookFinishPlanInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: 1}, Count: 2, Hash: strings.Repeat("b", 64)}
	bad := finish
	bad.Count = 3
	_, err = f.repo.FinishPlan(ctx, bad)
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	got, err := f.repo.FinishPlan(ctx, finish)
	require.NoError(t, err)
	require.True(t, got.Changed)
	require.True(t, got.Run.PlanReady)
	require.Equal(t, int64(2), got.Run.Version)
	require.Equal(t, finish.Hash, got.Run.PlanHash)
	require.True(t, got.Run.State.Before.Activated)
	require.False(t, got.Run.State.After.Activated)
	again, err = f.repo.FinishPlan(ctx, finish)
	require.NoError(t, err)
	require.False(t, again.Changed)
	require.Equal(t, got.Run, again.Run)
	_, err = f.repo.AppendPlanPage(ctx, hookTestPage(in.Key, 2, 2, "page-1", "page-2", 1))
	require.Error(t, err)
	bad = finish
	bad.Hash = strings.Repeat("c", 64)
	_, err = f.repo.FinishPlan(ctx, bad)
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
}

func TestHookTxPlanPageRollback(t *testing.T) {
	f := newHookTxFixture(t)
	ctx := context.Background()
	in := f.input(true, 0)
	initial, err := f.repo.CreateRunWithHooks(ctx, in)
	require.NoError(t, err)
	p := hookTestPage(in.Key, 0, 0, "", "end", 2)
	trigger := fmt.Sprintf("hook_plan_fail_%d", f.expt)
	require.NoError(t, f.sql.Exec(fmt.Sprintf("CREATE TRIGGER %s BEFORE INSERT ON expt_lifecycle_run_item FOR EACH ROW BEGIN IF NEW.expt_id=%d AND NEW.item_id=%d THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='hook page failure'; END IF; END", trigger, f.expt, p.Items[1].ItemID)).Error)
	t.Cleanup(func() { require.NoError(t, f.sql.Exec("DROP TRIGGER "+trigger).Error) })
	got, err := f.repo.AppendPlanPage(ctx, p)
	require.ErrorContains(t, err, "hook page failure")
	require.Nil(t, got.Run)
	read, err := f.repo.GetRun(ctx, in.Key)
	require.NoError(t, err)
	require.Equal(t, initial.Run, read)
	var count int64
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRunItem{}).Where("space_id=? AND expt_id=?", f.space, f.expt).Count(&count).Error)
	require.Zero(t, count)
}

func TestHookTxOnlineEmptyPlan(t *testing.T) {
	f := newHookTxFixture(t)
	in := f.input(false, 0)
	ctx := context.Background()
	initial, err := f.repo.CreateRunWithHooks(ctx, in)
	require.NoError(t, err)
	require.Equal(t, entity.HookGateReady, initial.Run.State.Gate)
	require.False(t, initial.Run.PlanReady)
	got, err := f.repo.FinishPlan(ctx, entity.HookFinishPlanInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key}, Hash: strings.Repeat("e", 64)})
	require.NoError(t, err)
	require.True(t, got.Run.PlanReady)
	require.Zero(t, got.Run.PlanCount)
	require.Equal(t, entity.HookGateReady, got.Run.State.Gate)
	require.False(t, got.Run.State.After.Activated)
}
