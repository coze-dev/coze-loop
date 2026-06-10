// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	rpcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
)

type testLifecycleEventMocks struct {
	exptRepo         *repoMocks.MockIExperimentRepo
	notifyRPCAdapter *rpcMocks.MockINotifyRPCAdapter
	userProvider     *rpcMocks.MockIUserProvider
	publisher        *eventMocks.MockExptEventPublisher
}

func newTestLifecycleEventHandler(ctrl *gomock.Controller) (*ExptLifecycleEventHandlerImpl, *testLifecycleEventMocks) {
	mockExptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	mockNotifyRPCAdapter := rpcMocks.NewMockINotifyRPCAdapter(ctrl)
	mockUserProvider := rpcMocks.NewMockIUserProvider(ctrl)
	mockPublisher := eventMocks.NewMockExptEventPublisher(ctrl)

	// Build the webhook delivery service with noop secret provider and nil http client
	// (tests don't exercise webhook delivery path)
	webhookSvc := NewWebhookDeliveryService(
		&noopSecretProvider{},
		nil, // httpClient not needed for feishu-only tests
		mockPublisher,
	)
	dispatcher := NewNotificationDispatcher(webhookSvc, mockNotifyRPCAdapter, mockUserProvider)

	handler := &ExptLifecycleEventHandlerImpl{
		exptRepo:   mockExptRepo,
		dispatcher: dispatcher,
	}

	return handler, &testLifecycleEventMocks{
		exptRepo:         mockExptRepo,
		notifyRPCAdapter: mockNotifyRPCAdapter,
		userProvider:     mockUserProvider,
		publisher:        mockPublisher,
	}
}

// noopSecretProvider is a test helper that returns empty secret
type noopSecretProvider struct{}

func (n *noopSecretProvider) GetSpaceSK(_ context.Context, _ int64) (string, error) {
	return "", nil
}

func TestNewExptLifecycleEventHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	mockPublisher := eventMocks.NewMockExptEventPublisher(ctrl)

	webhookSvc := NewWebhookDeliveryService(&noopSecretProvider{}, nil, mockPublisher)
	dispatcher := NewNotificationDispatcher(webhookSvc, nil, nil)

	handler := NewExptLifecycleEventHandler(mockExptRepo, dispatcher)
	assert.NotNil(t, handler)

	impl, ok := handler.(*ExptLifecycleEventHandlerImpl)
	assert.True(t, ok)
	assert.Equal(t, mockExptRepo, impl.exptRepo)
	assert.Equal(t, dispatcher, impl.dispatcher)
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

	t.Run("ToStatus is Pending, returns nil without sending (no trigger mapping)", func(t *testing.T) {
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

	t.Run("ToStatus does not match expt Status, returns nil", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		handler, mocks := newTestLifecycleEventHandler(ctrl)

		event := &entity.ExptLifecycleEvent{
			ExptID:   1,
			SpaceID:  100,
			ToStatus: entity.ExptStatus_Success,
		}
		expt := &entity.Experiment{
			ID:      1,
			SpaceID: 100,
			Status:  entity.ExptStatus_Failed, // mismatch
		}
		mocks.exptRepo.EXPECT().GetByID(ctx, int64(1), int64(100)).Return(expt, nil)

		err := handler.HandleLifecycleEvent(ctx, event)
		assert.NoError(t, err)
	})
}
