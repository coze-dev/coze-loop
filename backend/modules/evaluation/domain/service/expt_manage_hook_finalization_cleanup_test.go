// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	cm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func TestHookFinalizationManagerCentralCleanupFailure(t *testing.T) {
	f := newFinalizationManagerFixture(t)
	ctx := context.Background()
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumns(map[string]any{"scheduler_mode": "enforce", "scheduler_scope": "local-scheduler"}).Error)
	stats, err := f.deps.Repository.ReadFinalizationStats(ctx, f.key, "local")
	require.NoError(t, err)
	guard := cm.NewMockICentralReservationGuard(gomock.NewController(t))
	guard.EXPECT().Release(gomock.Any(), "local-scheduler", f.key.RunID, stats.ItemIDs[0], gomock.Any()).Return(errors.New("central release failed"))
	f.base.(*ExptMangerImpl).centralGuard = guard
	finalizationRecreate(t, f)
	require.Error(t, f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil))
	s := finalizationRead(t, f)
	require.Equal(t, entity.HookFinalizePending, s.State.Finalize)
	require.False(t, s.State.After.Activated)
	guard.EXPECT().Release(gomock.Any(), "local-scheduler", f.key.RunID, stats.ItemIDs[0], gomock.Any()).Return(nil)
	require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
	require.True(t, finalizationRead(t, f).State.After.Activated)
}

type finalizationWrongSource struct{ repo.IHookFinalizationRepo }

func (r finalizationWrongSource) ReadFinalizationSource(ctx context.Context, k entity.HookRunKey) (*entity.HookFinalizationSource, error) {
	out, err := r.IHookFinalizationRepo.ReadFinalizationSource(ctx, k)
	if err == nil {
		out.Key.RunID++
		out.Managed = false
	}
	return out, err
}

func TestHookFinalizationManagerSourceMismatchMustNotFallBack(t *testing.T) {
	f := newFinalizationManagerFixture(t)
	f.deps.Repository = finalizationWrongSource{f.deps.Repository}
	finalizationRecreate(t, f)
	err := f.manager.CompleteRun(context.Background(), f.expt, f.key.RunID, f.space, nil)
	require.Error(t, err, "an inconsistent source must never downgrade to the legacy path")
}
