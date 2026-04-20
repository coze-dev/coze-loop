// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	rpcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
)

type testLifecycleEventMocks struct {
	exptRepo         *repoMocks.MockIExperimentRepo
	notifyRPCAdapter *rpcMocks.MockINotifyRPCAdapter
	userProvider     *rpcMocks.MockIUserProvider
}

func newTestLifecycleEventHandler(ctrl *gomock.Controller) (*ExptLifecycleEventHandlerImpl, *testLifecycleEventMocks) {
	mockExptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	mockNotifyRPCAdapter := rpcMocks.NewMockINotifyRPCAdapter(ctrl)
	mockUserProvider := rpcMocks.NewMockIUserProvider(ctrl)

	handler := &ExptLifecycleEventHandlerImpl{
		exptRepo:         mockExptRepo,
		notifyRPCAdapter: mockNotifyRPCAdapter,
		userProvider:     mockUserProvider,
	}

	return handler, &testLifecycleEventMocks{
		exptRepo:         mockExptRepo,
		notifyRPCAdapter: mockNotifyRPCAdapter,
		userProvider:     mockUserProvider,
	}
}

func TestNewExptLifecycleEventHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	mockNotifyRPCAdapter := rpcMocks.NewMockINotifyRPCAdapter(ctrl)
	mockUserProvider := rpcMocks.NewMockIUserProvider(ctrl)

	handler := NewExptLifecycleEventHandler(mockExptRepo, mockNotifyRPCAdapter, mockUserProvider)
	assert.NotNil(t, handler)

	impl, ok := handler.(*ExptLifecycleEventHandlerImpl)
	assert.True(t, ok)
	assert.Equal(t, mockExptRepo, impl.exptRepo)
	assert.Equal(t, mockNotifyRPCAdapter, impl.notifyRPCAdapter)
	assert.Equal(t, mockUserProvider, impl.userProvider)
}

func TestHandleLifecycleEvent(t *testing.T) {
	ctx := context.Background()

	t.Run("GetByID returns error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		handler, mocks := newTestLifecycleEventHandler(ctrl)

		event := &entity.ExptLifecycleEvent{
			ExptID:  1,
			SpaceID: 100,
		}
		mocks.exptRepo.EXPECT().GetByID(ctx, int64(1), int64(100)).Return(nil, errors.New("db error"))

		err := handler.HandleLifecycleEvent(ctx, event)
		assert.EqualError(t, err, "db error")
	})

	t.Run("ToStatus is Success, sends notify card", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		handler, mocks := newTestLifecycleEventHandler(ctrl)

		event := &entity.ExptLifecycleEvent{
			ExptID:   1,
			SpaceID:  100,
			ToStatus: entity.ExptStatus_Success,
		}
		expt := &entity.Experiment{
			ID:        1,
			SpaceID:   100,
			Name:      "test-expt",
			Status:    entity.ExptStatus_Success,
			CreatedBy: "user1",
		}
		mocks.exptRepo.EXPECT().GetByID(ctx, int64(1), int64(100)).Return(expt, nil)
		mocks.userProvider.EXPECT().MGetUserInfo(ctx, []string{"user1"}).Return([]*entity.UserInfo{
			{Email: gptr.Of("user1@example.com")},
		}, nil)
		mocks.notifyRPCAdapter.EXPECT().SendMessageCard(ctx, "user1@example.com", gomock.Any(), gomock.Any()).Return(nil)

		err := handler.HandleLifecycleEvent(ctx, event)
		assert.NoError(t, err)
	})

	t.Run("ToStatus is Failed, sends notify card", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		handler, mocks := newTestLifecycleEventHandler(ctrl)

		event := &entity.ExptLifecycleEvent{
			ExptID:   2,
			SpaceID:  200,
			ToStatus: entity.ExptStatus_Failed,
		}
		expt := &entity.Experiment{
			ID:        2,
			SpaceID:   200,
			Name:      "test-expt-failed",
			Status:    entity.ExptStatus_Failed,
			CreatedBy: "user2",
		}
		mocks.exptRepo.EXPECT().GetByID(ctx, int64(2), int64(200)).Return(expt, nil)
		mocks.userProvider.EXPECT().MGetUserInfo(ctx, []string{"user2"}).Return([]*entity.UserInfo{
			{Email: gptr.Of("user2@example.com")},
		}, nil)
		mocks.notifyRPCAdapter.EXPECT().SendMessageCard(ctx, "user2@example.com", gomock.Any(), gomock.Any()).Return(nil)

		err := handler.HandleLifecycleEvent(ctx, event)
		assert.NoError(t, err)
	})

	t.Run("ToStatus is Terminated, sends notify card", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		handler, mocks := newTestLifecycleEventHandler(ctrl)

		event := &entity.ExptLifecycleEvent{
			ExptID:   3,
			SpaceID:  300,
			ToStatus: entity.ExptStatus_Terminated,
		}
		expt := &entity.Experiment{
			ID:        3,
			SpaceID:   300,
			Name:      "test-expt-terminated",
			Status:    entity.ExptStatus_Terminated,
			CreatedBy: "user3",
		}
		mocks.exptRepo.EXPECT().GetByID(ctx, int64(3), int64(300)).Return(expt, nil)
		mocks.userProvider.EXPECT().MGetUserInfo(ctx, []string{"user3"}).Return([]*entity.UserInfo{
			{Email: gptr.Of("user3@example.com")},
		}, nil)
		mocks.notifyRPCAdapter.EXPECT().SendMessageCard(ctx, "user3@example.com", gomock.Any(), gomock.Any()).Return(nil)

		err := handler.HandleLifecycleEvent(ctx, event)
		assert.NoError(t, err)
	})

	t.Run("ToStatus is SystemTerminated, sends notify card", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		handler, mocks := newTestLifecycleEventHandler(ctrl)

		event := &entity.ExptLifecycleEvent{
			ExptID:   4,
			SpaceID:  400,
			ToStatus: entity.ExptStatus_SystemTerminated,
		}
		expt := &entity.Experiment{
			ID:        4,
			SpaceID:   400,
			Name:      "test-expt-sys-terminated",
			Status:    entity.ExptStatus_SystemTerminated,
			CreatedBy: "user4",
		}
		mocks.exptRepo.EXPECT().GetByID(ctx, int64(4), int64(400)).Return(expt, nil)
		mocks.userProvider.EXPECT().MGetUserInfo(ctx, []string{"user4"}).Return([]*entity.UserInfo{
			{Email: gptr.Of("user4@example.com")},
		}, nil)
		mocks.notifyRPCAdapter.EXPECT().SendMessageCard(ctx, "user4@example.com", gomock.Any(), gomock.Any()).Return(nil)

		err := handler.HandleLifecycleEvent(ctx, event)
		assert.NoError(t, err)
	})

	t.Run("ToStatus is Pending, returns nil without sending", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		handler, mocks := newTestLifecycleEventHandler(ctrl)

		event := &entity.ExptLifecycleEvent{
			ExptID:   5,
			SpaceID:  500,
			ToStatus: entity.ExptStatus_Pending,
		}
		expt := &entity.Experiment{
			ID:      5,
			SpaceID: 500,
			Status:  entity.ExptStatus_Pending,
		}
		mocks.exptRepo.EXPECT().GetByID(ctx, int64(5), int64(500)).Return(expt, nil)

		err := handler.HandleLifecycleEvent(ctx, event)
		assert.NoError(t, err)
	})

	t.Run("ToStatus is Processing, returns nil without sending", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		handler, mocks := newTestLifecycleEventHandler(ctrl)

		event := &entity.ExptLifecycleEvent{
			ExptID:   6,
			SpaceID:  600,
			ToStatus: entity.ExptStatus_Processing,
		}
		expt := &entity.Experiment{
			ID:      6,
			SpaceID: 600,
			Status:  entity.ExptStatus_Processing,
		}
		mocks.exptRepo.EXPECT().GetByID(ctx, int64(6), int64(600)).Return(expt, nil)

		err := handler.HandleLifecycleEvent(ctx, event)
		assert.NoError(t, err)
	})
}

func TestSendNotifyCard(t *testing.T) {
	ctx := context.Background()

	t.Run("event ToStatus does not match expt Status, returns nil", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		handler, _ := newTestLifecycleEventHandler(ctrl)

		event := &entity.ExptLifecycleEvent{
			ToStatus: entity.ExptStatus_Success,
		}
		expt := &entity.Experiment{
			Status: entity.ExptStatus_Failed,
		}

		err := handler.sendNotifyCard(ctx, event, expt)
		assert.NoError(t, err)
	})

	t.Run("MGetUserInfo returns error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		handler, mocks := newTestLifecycleEventHandler(ctrl)

		event := &entity.ExptLifecycleEvent{
			ToStatus: entity.ExptStatus_Success,
		}
		expt := &entity.Experiment{
			Status:    entity.ExptStatus_Success,
			CreatedBy: "user1",
		}
		mocks.userProvider.EXPECT().MGetUserInfo(ctx, []string{"user1"}).Return(nil, errors.New("user provider error"))

		err := handler.sendNotifyCard(ctx, event, expt)
		assert.EqualError(t, err, "user provider error")
	})

	t.Run("MGetUserInfo returns empty list, returns nil", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		handler, mocks := newTestLifecycleEventHandler(ctrl)

		event := &entity.ExptLifecycleEvent{
			ToStatus: entity.ExptStatus_Success,
		}
		expt := &entity.Experiment{
			Status:    entity.ExptStatus_Success,
			CreatedBy: "user1",
		}
		mocks.userProvider.EXPECT().MGetUserInfo(ctx, []string{"user1"}).Return([]*entity.UserInfo{}, nil)

		err := handler.sendNotifyCard(ctx, event, expt)
		assert.NoError(t, err)
	})

	t.Run("MGetUserInfo returns nil user, returns nil", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		handler, mocks := newTestLifecycleEventHandler(ctrl)

		event := &entity.ExptLifecycleEvent{
			ToStatus: entity.ExptStatus_Success,
		}
		expt := &entity.Experiment{
			Status:    entity.ExptStatus_Success,
			CreatedBy: "user1",
		}
		mocks.userProvider.EXPECT().MGetUserInfo(ctx, []string{"user1"}).Return([]*entity.UserInfo{nil}, nil)

		err := handler.sendNotifyCard(ctx, event, expt)
		assert.NoError(t, err)
	})

	t.Run("MGetUserInfo returns user with nil email, returns nil", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		handler, mocks := newTestLifecycleEventHandler(ctrl)

		event := &entity.ExptLifecycleEvent{
			ToStatus: entity.ExptStatus_Success,
		}
		expt := &entity.Experiment{
			Status:    entity.ExptStatus_Success,
			CreatedBy: "user1",
		}
		mocks.userProvider.EXPECT().MGetUserInfo(ctx, []string{"user1"}).Return([]*entity.UserInfo{
			{Email: nil},
		}, nil)

		err := handler.sendNotifyCard(ctx, event, expt)
		assert.NoError(t, err)
	})

	t.Run("MGetUserInfo returns user with empty email, returns nil", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		handler, mocks := newTestLifecycleEventHandler(ctrl)

		event := &entity.ExptLifecycleEvent{
			ToStatus: entity.ExptStatus_Success,
		}
		expt := &entity.Experiment{
			Status:    entity.ExptStatus_Success,
			CreatedBy: "user1",
		}
		mocks.userProvider.EXPECT().MGetUserInfo(ctx, []string{"user1"}).Return([]*entity.UserInfo{
			{Email: gptr.Of("")},
		}, nil)

		err := handler.sendNotifyCard(ctx, event, expt)
		assert.NoError(t, err)
	})

	t.Run("MGetUserInfo returns multiple users, returns nil", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		handler, mocks := newTestLifecycleEventHandler(ctrl)

		event := &entity.ExptLifecycleEvent{
			ToStatus: entity.ExptStatus_Success,
		}
		expt := &entity.Experiment{
			Status:    entity.ExptStatus_Success,
			CreatedBy: "user1",
		}
		mocks.userProvider.EXPECT().MGetUserInfo(ctx, []string{"user1"}).Return([]*entity.UserInfo{
			{Email: gptr.Of("user1@example.com")},
			{Email: gptr.Of("user2@example.com")},
		}, nil)

		err := handler.sendNotifyCard(ctx, event, expt)
		assert.NoError(t, err)
	})

	t.Run("SendMessageCard returns error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		handler, mocks := newTestLifecycleEventHandler(ctrl)

		now := time.Now()
		event := &entity.ExptLifecycleEvent{
			ToStatus: entity.ExptStatus_Success,
		}
		expt := &entity.Experiment{
			ID:        1,
			SpaceID:   100,
			Name:      "test",
			Status:    entity.ExptStatus_Success,
			CreatedBy: "user1",
			StartAt:   &now,
		}
		mocks.userProvider.EXPECT().MGetUserInfo(ctx, []string{"user1"}).Return([]*entity.UserInfo{
			{Email: gptr.Of("user1@example.com")},
		}, nil)
		mocks.notifyRPCAdapter.EXPECT().SendMessageCard(ctx, "user1@example.com", gomock.Any(), gomock.Any()).Return(errors.New("send error"))

		err := handler.sendNotifyCard(ctx, event, expt)
		assert.EqualError(t, err, "send error")
	})

	t.Run("SendMessageCard succeeds", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		handler, mocks := newTestLifecycleEventHandler(ctrl)

		now := time.Now()
		event := &entity.ExptLifecycleEvent{
			ToStatus: entity.ExptStatus_Success,
		}
		expt := &entity.Experiment{
			ID:        1,
			SpaceID:   100,
			Name:      "test",
			Status:    entity.ExptStatus_Success,
			CreatedBy: "user1",
			StartAt:   &now,
			EndAt:     &now,
		}
		mocks.userProvider.EXPECT().MGetUserInfo(ctx, []string{"user1"}).Return([]*entity.UserInfo{
			{Email: gptr.Of("user1@example.com")},
		}, nil)
		mocks.notifyRPCAdapter.EXPECT().SendMessageCard(ctx, "user1@example.com", gomock.Any(), gomock.Any()).Return(nil)

		err := handler.sendNotifyCard(ctx, event, expt)
		assert.NoError(t, err)
	})
}
