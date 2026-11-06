// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// Completion uses the retained admission instead of deleting it and racing to
// SETNX a cleanup lease. Its completed replay cannot clear the next run's quota.
func TestCompleteRunCleanupRetainsAdmissionAndSkipsCompletedReplay(t *testing.T) {
	ctrl := gomock.NewController(t)
	provider := db.NewTestDB(t, &model.Experiment{}, &model.ExptRunLog{}, &model.ExptStats{}, &model.ExptEvaluatorRef{})
	ctx := context.Background()
	endedAt := time.Now().Add(-time.Minute)
	require.NoError(t, provider.NewSession(ctx).Create(&model.Experiment{ID: 7, SpaceID: 9, ExptType: int32(entity.ExptType_Offline), LatestRunID: 8, Status: int32(entity.ExptStatus_Success), EndAt: &endedAt}).Error)
	require.NoError(t, provider.NewSession(ctx).Create(&model.ExptRunLog{ID: 8, ExptID: 7, ExptRunID: 8, SpaceID: 9, Status: gptr.Of(int64(entity.ExptStatus_Success))}).Error)
	factory, server := finishRealLockerFactory(t)
	key := "expt_run_mutex_lock:7"
	require.NoError(t, server.Set(key, "8"))
	quota := repomocks.NewMockQuotaRepo(ctrl)
	quota.EXPECT().CreateOrUpdate(gomock.Any(), int64(9), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ int64, update func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
		holder, err := server.Get(key)
		require.NoError(t, err)
		require.Equal(t, "8", holder, "quota release must retain original admission")
		locked, _, err := factory().BackoffLockWithValue(ctx, key, "9", time.Hour, time.Millisecond)
		require.NoError(t, err)
		require.False(t, locked, "replacement must remain excluded during quota release")
		_, _, err = update(&entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{7: 1}})
		return err
	})
	manager := &ExptMangerImpl{exptRepo: exptrepo.NewExptRepo(exptmysql.NewExptDAO(provider), exptmysql.NewExptEvaluatorRefDAO(provider), nil), runLogRepo: exptrepo.NewExptRunLogRepo(exptmysql.NewExptRunLogDAO(provider)), runFinishRepo: exptrepo.NewExptRunFinishRepo(provider), manualRetryLocker: factory, quotaRepo: quota, idem: &cleanupTestIdem{}}
	require.NoError(t, manager.CompleteRun(ctx, 7, 8, 9, &entity.Session{}))
	require.False(t, server.Exists(key))
	// B has only acquired the fixed holder: completed A replay must exit before
	// touching it, even though SQL has not yet advanced LatestRunID.
	require.NoError(t, server.Set(key, "evaluation"))
	require.NoError(t, manager.CompleteRun(ctx, 7, 8, 9, &entity.Session{}))
	holder, err := server.Get(key)
	require.NoError(t, err)
	require.Equal(t, "evaluation", holder)
	require.NoError(t, manager.exptRepo.Update(ctx, &entity.Experiment{ID: 7, LatestRunID: 9, Status: entity.ExptStatus_Processing}))
	require.NoError(t, manager.CompleteRun(ctx, 7, 8, 9, &entity.Session{}))
	holder, err = server.Get(key)
	require.NoError(t, err)
	require.Equal(t, "evaluation", holder)
	current, err := manager.exptRepo.GetByID(ctx, 7, 9)
	require.NoError(t, err)
	require.Equal(t, int64(9), current.LatestRunID)
	require.Equal(t, entity.ExptStatus_Processing, current.Status)
}
