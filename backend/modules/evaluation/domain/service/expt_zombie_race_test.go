// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
)

func TestHandleZombies_ActiveConsumerLeasePreventsReclaim(t *testing.T) {
	ctrl := gomock.NewController(t)
	configer := configmocks.NewMockIConfiger(ctrl)
	mutex := lockmocks.NewMockILocker(ctrl)
	itemRepo := repomocks.NewMockIExptItemResultRepo(ctrl)
	turnRepo := repomocks.NewMockIExptTurnResultRepo(ctrl)
	evaluatorRepo := repomocks.NewMockIEvaluatorRecordRepo(ctrl)

	configer.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{
		ExptExecConf: &entity.ExptExecConf{ExptItemEvalConf: &entity.ExptItemEvalConf{ZombieSecond: 1}},
	})
	mutex.EXPECT().Lock(gomock.Any(), "expt_item_eval_run_lock:10:30", gomock.Any()).Return(false, nil)

	updatedAt := time.Now().Add(-time.Minute)
	item := &entity.ExptEvalItem{ExptID: 10, ItemID: 30, State: entity.ItemRunState_Processing, UpdatedAt: &updatedAt}
	svc := &ExptSchedulerImpl{
		Configer:            configer,
		Mutex:               mutex,
		ExptItemResultRepo:  itemRepo,
		ExptTurnResultRepo:  turnRepo,
		EvaluatorRecordRepo: evaluatorRepo,
	}

	alives, zombies, err := svc.handleZombies(context.Background(), &entity.ExptScheduleEvent{
		SpaceID: 20, ExptID: 10, ExptRunID: 40,
	}, []*entity.ExptEvalItem{item}, nil)

	require.NoError(t, err)
	assert.Equal(t, []*entity.ExptEvalItem{item}, alives)
	assert.Empty(t, zombies)
}

func TestCompleteItemRun_CancelledLeaseDoesNotPersistLateResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	itemRepo := repomocks.NewMockIExptItemResultRepo(ctrl)
	executor := &ExptItemEvalCtxExecutor{ItemResultRepo: itemRepo}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := executor.CompleteItemRun(ctx, &entity.ExptItemEvalCtx{
		Event: &entity.ExptItemEvalEvent{SpaceID: 20, ExptID: 10, ExptRunID: 40, EvalSetItemID: 30},
	}, nil)

	require.ErrorIs(t, err, context.Canceled)
}

func TestStoreTurnRunResult_CancelledLeaseDoesNotPersistLateTurn(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := (&ExptItemEvalCtxExecutor{}).storeTurnRunResult(ctx, nil, nil)

	require.ErrorIs(t, err, context.Canceled)
}

func TestCompleteItemRun_DoesNotReopenAlreadySettledRunLog(t *testing.T) {
	ctrl := gomock.NewController(t)
	itemRepo := repomocks.NewMockIExptItemResultRepo(ctrl)
	executor := &ExptItemEvalCtxExecutor{ItemResultRepo: itemRepo}

	itemRepo.EXPECT().CompareAndSwapItemRunLog(
		gomock.Any(), int64(10), int64(40), int64(30), entity.ItemRunState_Processing, gomock.Any(), int64(20),
	).Return(false, nil)

	require.NoError(t, executor.CompleteItemRun(context.Background(), &entity.ExptItemEvalCtx{
		Event: &entity.ExptItemEvalEvent{SpaceID: 20, ExptID: 10, ExptRunID: 40, EvalSetItemID: 30},
	}, nil))
}

func TestHandleZombies_LeaseErrorDoesNotReclaim(t *testing.T) {
	ctrl := gomock.NewController(t)
	configer := configmocks.NewMockIConfiger(ctrl)
	mutex := lockmocks.NewMockILocker(ctrl)

	configer.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{
		ExptExecConf: &entity.ExptExecConf{ExptItemEvalConf: &entity.ExptItemEvalConf{ZombieSecond: 1}},
	})
	mutex.EXPECT().Lock(gomock.Any(), "expt_item_eval_run_lock:10:30", gomock.Any()).Return(false, assert.AnError)

	updatedAt := time.Now().Add(-time.Minute)
	item := &entity.ExptEvalItem{ExptID: 10, ItemID: 30, State: entity.ItemRunState_Processing, UpdatedAt: &updatedAt}
	svc := &ExptSchedulerImpl{Configer: configer, Mutex: mutex}

	alives, zombies, err := svc.handleZombies(context.Background(), &entity.ExptScheduleEvent{
		SpaceID: 20, ExptID: 10, ExptRunID: 40,
	}, []*entity.ExptEvalItem{item}, nil)

	require.Error(t, err)
	assert.Empty(t, zombies)
	assert.Empty(t, alives)
}

func TestHandleZombies_LegacyUnlockedStaleItemIsReclaimed(t *testing.T) {
	ctrl := gomock.NewController(t)
	configer := configmocks.NewMockIConfiger(ctrl)
	mutex := lockmocks.NewMockILocker(ctrl)
	itemRepo := repomocks.NewMockIExptItemResultRepo(ctrl)
	turnRepo := repomocks.NewMockIExptTurnResultRepo(ctrl)
	evaluatorRepo := repomocks.NewMockIEvaluatorRecordRepo(ctrl)

	configer.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{
		ExptExecConf: &entity.ExptExecConf{ExptItemEvalConf: &entity.ExptItemEvalConf{ZombieSecond: 1}},
	})
	mutex.EXPECT().Lock(gomock.Any(), "expt_item_eval_run_lock:10:30", gomock.Any()).Return(true, nil)
	mutex.EXPECT().Unlock("expt_item_eval_run_lock:10:30").Return(true, nil)
	itemRepo.EXPECT().CompareAndSwapItemRunLog(
		gomock.Any(), int64(10), int64(40), int64(30), entity.ItemRunState_Processing, gomock.Any(), int64(20),
	).Return(true, nil)
	turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), int64(10), int64(40), []int64{30}, int64(20)).Return(nil, nil)
	itemRepo.EXPECT().UpdateItemsResult(gomock.Any(), int64(20), int64(10), []int64{30}, gomock.Any()).Return(nil)
	turnRepo.EXPECT().CreateOrUpdateItemsTurnRunLogStatus(
		gomock.Any(), int64(20), int64(10), int64(40), []int64{30}, entity.TurnRunState_Fail,
	).Return(nil)

	updatedAt := time.Now().Add(-time.Minute)
	item := &entity.ExptEvalItem{ExptID: 10, ItemID: 30, State: entity.ItemRunState_Processing, UpdatedAt: &updatedAt}
	svc := &ExptSchedulerImpl{
		Configer: configer, Mutex: mutex, ExptItemResultRepo: itemRepo,
		ExptTurnResultRepo: turnRepo, EvaluatorRecordRepo: evaluatorRepo,
	}

	alives, zombies, err := svc.handleZombies(context.Background(), &entity.ExptScheduleEvent{
		SpaceID: 20, ExptID: 10, ExptRunID: 40,
	}, []*entity.ExptEvalItem{item}, &entity.Experiment{ExptDispatchMode: entity.ExptDispatchModeLegacy})

	require.NoError(t, err)
	assert.Empty(t, alives)
	require.Len(t, zombies, 1)
	assert.Equal(t, entity.ItemRunState_Fail, zombies[0].State)
}

func TestHandleZombies_CentralCASMissHasNoTerminalSideEffects(t *testing.T) {
	ctrl := gomock.NewController(t)
	configer := configmocks.NewMockIConfiger(ctrl)
	mutex := lockmocks.NewMockILocker(ctrl)
	itemRepo := repomocks.NewMockIExptItemResultRepo(ctrl)
	turnRepo := repomocks.NewMockIExptTurnResultRepo(ctrl)
	guard := configmocks.NewMockICentralReservationGuard(ctrl)

	configer.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{
		ExptExecConf: &entity.ExptExecConf{ExptItemEvalConf: &entity.ExptItemEvalConf{ZombieSecond: 1}},
	})
	mutex.EXPECT().Lock(gomock.Any(), "expt_item_eval_run_lock:10:30", gomock.Any()).Return(true, nil)
	mutex.EXPECT().Unlock("expt_item_eval_run_lock:10:30").Return(true, nil)
	itemRepo.EXPECT().CompareAndSwapItemRunLog(
		gomock.Any(), int64(10), int64(40), int64(30), entity.ItemRunState_Processing, gomock.Any(), int64(20),
	).Return(false, nil)

	updatedAt := time.Now().Add(-time.Minute)
	item := &entity.ExptEvalItem{ExptID: 10, ItemID: 30, State: entity.ItemRunState_Processing, UpdatedAt: &updatedAt}
	svc := &ExptSchedulerImpl{
		Configer: configer, Mutex: mutex, ExptItemResultRepo: itemRepo,
		ExptTurnResultRepo: turnRepo, centralGuard: guard,
	}

	alives, zombies, err := svc.handleZombies(context.Background(), &entity.ExptScheduleEvent{
		SpaceID: 20, ExptID: 10, ExptRunID: 40,
	}, []*entity.ExptEvalItem{item}, &entity.Experiment{
		ID: 10, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: "cn",
	})

	require.NoError(t, err)
	assert.Equal(t, []*entity.ExptEvalItem{item}, alives)
	assert.Empty(t, zombies)
}
