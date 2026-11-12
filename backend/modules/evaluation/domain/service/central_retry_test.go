// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	idgenMocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
)

func TestExptMangerImpl_ResetItemsForCentralRetry(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	const (
		spaceID = int64(10)
		exptID  = int64(100)
		runID   = int64(200)
	)
	mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo).EXPECT().
		BatchGet(ctx, spaceID, exptID, []int64{11, 12}).
		Return([]*entity.ExptItemResult{
			{ItemID: 11, Status: entity.ItemRunState_Fail},
			{ItemID: 12, Status: entity.ItemRunState_Success},
		}, nil)
	mgr.statsRepo.(*repoMocks.MockIExptStatsRepo).EXPECT().
		Get(ctx, exptID, spaceID).
		Return(&entity.ExptStats{PendingItemCnt: 0, FailItemCnt: 1, SuccessItemCnt: 1}, nil)
	mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo).EXPECT().
		UpdateItemRunLog(ctx, exptID, runID, []int64{11, 12}, gomock.Any(), spaceID).
		Return(nil)
	mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo).EXPECT().
		UpdateItemsResult(ctx, spaceID, exptID, []int64{11, 12}, gomock.Any()).
		Return(nil)
	mgr.turnResultRepo.(*repoMocks.MockIExptTurnResultRepo).EXPECT().
		UpdateTurnResultsWithItemIDs(ctx, exptID, []int64{11, 12}, spaceID, gomock.Any()).
		Return(nil)
	mgr.idgenerator.(*idgenMocks.MockIIDGenerator).EXPECT().
		GenMultiIDs(ctx, 2).
		Return([]int64{1001, 1002}, nil)
	mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo).EXPECT().
		BatchCreateNXRunLogs(ctx, gomock.Any()).
		Return(nil)
	mgr.statsRepo.(*repoMocks.MockIExptStatsRepo).EXPECT().
		Save(ctx, gomock.Any()).
		Return(nil)
	mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().
		Update(ctx, gomock.Any()).
		Return(nil)

	expt := &entity.Experiment{ID: exptID, SpaceID: spaceID, EvalConf: &entity.EvaluationConfiguration{
		Scheduling: &entity.SchedulingConfig{Mode: entity.SchedulingModeCentral},
	}}
	require.NoError(t, mgr.resetItemsForCentralRetry(ctx, expt, runID, []int64{11, 12}, spaceID))
}

func TestEntityScheduling_CentralOnlyOfflineGuard(t *testing.T) {
	central := &entity.EvaluationConfiguration{Scheduling: &entity.SchedulingConfig{Mode: entity.SchedulingModeCentral}}
	assert.True(t, central.Scheduling.IsCentral())
	legacy := &entity.EvaluationConfiguration{}
	assert.False(t, legacy.Scheduling.IsCentral())
}
