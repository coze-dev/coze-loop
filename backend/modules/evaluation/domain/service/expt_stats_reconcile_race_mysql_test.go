// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	experimentrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	experimentmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

type countThenStartReservedRepo struct {
	repo.IExptItemResultRepo
	started bool
	start   func()
}

func (r *countThenStartReservedRepo) CountItemsByStatus(ctx context.Context, spaceID, exptID int64) (map[entity.ItemRunState]int64, error) {
	actual, err := r.IExptItemResultRepo.CountItemsByStatus(ctx, spaceID, exptID)
	if err == nil && !r.started {
		r.started = true
		r.start()
	}
	return actual, err
}

func TestReconcileStatsDoesNotOverwriteCommittedItemProgress(t *testing.T) {
	f := newLateWriteFixture(t, "")
	ctx := context.Background()
	itemID := f.item + 2
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).
		Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", f.space, f.expt, f.run, itemID).
		Update("quota_reservation_state", int32(entity.QuotaReservationStateReserved)).Error)
	dispatch := experimentmysql.NewExptItemDispatchDAO(f.provider)
	start := func() {
		result, err := dispatch.StartReservedItem(ctx, f.space, f.expt, f.run, itemID, "")
		require.NoError(t, err)
		require.True(t, result.Started)
	}
	items := &countThenStartReservedRepo{
		IExptItemResultRepo: experimentrepo.NewExptItemResultRepo(experimentmysql.NewExptItemResultDAO(f.provider)),
		start:               start,
	}
	stats := experimentrepo.NewExptStatsRepo(experimentmysql.NewExptStatsDAO(f.provider))
	ctrl := gomock.NewController(t)
	idem := idemmocks.NewMockIdempotentService(ctrl)
	idem.EXPECT().SetNX(gomock.Any(), gomock.Any(), 5*time.Minute).Return(true, nil)
	scheduler := &ExptSchedulerImpl{ExptItemResultRepo: items, ExptStatsRepo: stats, Idem: idem}

	scheduler.reconcileExptStats(ctx, &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt}, &entity.Experiment{
		ID: f.expt, SpaceID: f.space, ExptDispatchMode: entity.ExptDispatchModeEnforce,
	})
	require.True(t, items.started, "the real item transaction must commit between the first count and stats reconciliation")

	current, err := stats.Get(ctx, f.expt, f.space)
	require.NoError(t, err)
	t.Logf("STATS_RECONCILE_FINAL pending=%d processing=%d", current.PendingItemCnt, current.ProcessingItemCnt)
	require.Zero(t, current.PendingItemCnt)
	require.Equal(t, int32(3), current.ProcessingItemCnt)
}
