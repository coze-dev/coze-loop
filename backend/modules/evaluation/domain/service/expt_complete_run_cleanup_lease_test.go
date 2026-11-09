// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// First completion releases admission before terminal visibility; a completed
// replay cannot clear the next run's shared resources.
func TestCompleteRunPairsInitialAdmissionAndSkipsCompletedReplay(t *testing.T) {
	ctrl := gomock.NewController(t)
	provider := db.NewTestDB(t, &model.Experiment{}, &model.ExptRunLog{}, &model.ExptStats{}, &model.ExptEvaluatorRef{}, &model.ExptItemResult{}, &model.ExptItemResultRunLog{}, &model.ExptTurnResult{})
	ctx := context.Background()
	require.NoError(t, provider.NewSession(ctx).Create(&model.Experiment{ID: 7, SpaceID: 9, ExptType: int32(entity.ExptType_Offline), LatestRunID: 8, Status: int32(entity.ExptStatus_Processing)}).Error)
	require.NoError(t, provider.NewSession(ctx).Create(&model.ExptRunLog{ID: 8, ExptID: 7, ExptRunID: 8, SpaceID: 9, Mode: gptr.Of(int32(entity.EvaluationModeSubmit)), Status: gptr.Of(int64(entity.ExptStatus_Processing))}).Error)
	require.NoError(t, provider.NewSession(ctx).Create(&model.ExptStats{ID: 10, SpaceID: 9, ExptID: 7}).Error)
	factory, server := finishRealLockerFactory(t)
	key := "expt_run_mutex_lock:7"
	require.NoError(t, server.Set(key, "evaluation"))
	quota := repomocks.NewMockQuotaRepo(ctrl)
	quotaState := &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{7: 1}}
	writes := 0
	quota.EXPECT().CreateOrUpdate(gomock.Any(), int64(9), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ int64, update func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
		if _, exists := quotaState.ExptID2RunTime[7]; exists {
			require.False(t, server.Exists(key), "first admission release precedes post-commit quota cleanup")
		}
		var row model.Experiment
		require.NoError(t, provider.NewSession(ctx).First(&row, 7).Error)
		require.Equal(t, int32(entity.ExptStatus_Success), row.Status)
		next, changed, err := update(quotaState)
		quotaState = next
		if changed {
			writes++
		}
		return err
	}).Times(2)
	cleanup := svcmocks.NewMockExptResultService(ctrl)
	cleanup.EXPECT().GetIncompleteTurns(gomock.Any(), int64(7), int64(9), gomock.Any()).Return(nil, nil).AnyTimes()
	manager := &ExptMangerImpl{exptRepo: exptrepo.NewExptRepo(exptmysql.NewExptDAO(provider), exptmysql.NewExptEvaluatorRefDAO(provider), nil), runLogRepo: exptrepo.NewExptRunLogRepo(exptmysql.NewExptRunLogDAO(provider)), runFinishRepo: exptrepo.NewExptRunFinishRepo(provider), mutex: factory().WithHolder("evaluation"), manualRetryLocker: factory, quotaRepo: quota, idem: &cleanupTestIdem{}, exptResultService: &finishCalibrationTestResult{ExptResultService: cleanup, ready: true}}
	require.NoError(t, manager.CompleteRun(ctx, 7, 8, 9, &entity.Session{}))
	require.False(t, server.Exists(key))
	// B has only acquired the fixed holder: completed A replay must exit before
	// touching it, even though SQL has not yet advanced LatestRunID.
	require.NoError(t, server.Set(key, "evaluation"))
	manager.idem = &cleanupTestIdem{}
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
	require.Equal(t, 1, writes, "replacement replay must not refund a second slot")
}
