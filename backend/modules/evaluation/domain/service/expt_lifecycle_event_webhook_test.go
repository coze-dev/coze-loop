// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
)

// fakeWebhookDispatcher is a minimal IWebhookDispatcher stub for use in tests.
type fakeWebhookDispatcher struct {
	called bool
	err    error
}

func (f *fakeWebhookDispatcher) Dispatch(_ context.Context, _ *entity.ExptLifecycleEvent, _ *entity.Experiment) error {
	f.called = true
	return f.err
}

func TestHandleLifecycleEvent_WithWebhookDispatcher_BitsUT(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("webhook dispatcher called when configured and dispatch succeeds", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockExptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
		dispatcher := &fakeWebhookDispatcher{}

		handler := &ExptLifecycleEventHandlerImpl{
			exptRepo:          mockExptRepo,
			webhookDispatcher: dispatcher,
		}

		event := &entity.ExptLifecycleEvent{
			ExptID: 10, SpaceID: 100,
			// Use Processing (non-terminal) to avoid triggering legacy feishu sendNotifyCard
			// when NotificationConf is nil and userProvider is not set.
			ToStatus: entity.ExptStatus_Processing,
		}
		// legacy expt (no NotificationConf) — feishu path skips non-terminal, webhook path runs
		expt := &entity.Experiment{
			ID: 10, SpaceID: 100,
			NotificationConf: nil,
		}
		mockExptRepo.EXPECT().GetByID(ctx, int64(10), int64(100)).Return(expt, nil)

		err := handler.HandleLifecycleEvent(ctx, event)
		assert.NoError(t, err)
		assert.True(t, dispatcher.called, "webhook dispatcher should have been called")
	})

	t.Run("webhook dispatcher error is logged but not propagated", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockExptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
		dispatcher := &fakeWebhookDispatcher{err: errors.New("dispatch error")}

		handler := &ExptLifecycleEventHandlerImpl{
			exptRepo:          mockExptRepo,
			webhookDispatcher: dispatcher,
		}

		event := &entity.ExptLifecycleEvent{
			ExptID: 11, SpaceID: 110,
			ToStatus: entity.ExptStatus_Failed,
		}
		expt := &entity.Experiment{
			ID: 11, SpaceID: 110,
			NotificationConf: &entity.ExptNotificationConf{
				Webhook: &entity.WebhookNotificationConf{Enable: true, Urls: gptr.Of("http://example.com/hook")},
			},
		}
		mockExptRepo.EXPECT().GetByID(ctx, int64(11), int64(110)).Return(expt, nil)

		// HandleLifecycleEvent must return nil even when dispatcher returns an error
		err := handler.HandleLifecycleEvent(ctx, event)
		assert.NoError(t, err)
		assert.True(t, dispatcher.called, "webhook dispatcher should have been called")
	})

	t.Run("nil webhook dispatcher skips dispatch without panic", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockExptRepo := repoMocks.NewMockIExperimentRepo(ctrl)

		handler := &ExptLifecycleEventHandlerImpl{
			exptRepo:          mockExptRepo,
			webhookDispatcher: nil,
		}

		event := &entity.ExptLifecycleEvent{
			ExptID: 12, SpaceID: 120,
			// Use Processing (non-terminal) to avoid legacy feishu path when userProvider is nil
			ToStatus: entity.ExptStatus_Processing,
		}
		expt := &entity.Experiment{
			ID: 12, SpaceID: 120,
			NotificationConf: nil,
		}
		mockExptRepo.EXPECT().GetByID(ctx, int64(12), int64(120)).Return(expt, nil)

		// No panic expected — nil dispatcher is handled gracefully
		err := handler.HandleLifecycleEvent(ctx, event)
		assert.NoError(t, err)
	})

	t.Run("webhook dispatcher called when webhook is enabled with filter match", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockExptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
		dispatcher := &fakeWebhookDispatcher{}

		handler := &ExptLifecycleEventHandlerImpl{
			exptRepo:          mockExptRepo,
			webhookDispatcher: dispatcher,
		}

		event := &entity.ExptLifecycleEvent{
			ExptID: 13, SpaceID: 130,
			ToStatus: entity.ExptStatus_Processing,
		}
		expt := &entity.Experiment{
			ID: 13, SpaceID: 130,
			NotificationConf: &entity.ExptNotificationConf{
				Webhook: &entity.WebhookNotificationConf{Enable: true, Urls: gptr.Of("http://example.com/hook")},
			},
		}
		mockExptRepo.EXPECT().GetByID(ctx, int64(13), int64(130)).Return(expt, nil)

		err := handler.HandleLifecycleEvent(ctx, event)
		assert.NoError(t, err)
		assert.True(t, dispatcher.called)
	})
}

func TestDispatchWebhook_BitsUT(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("dispatcher is nil: does nothing", func(t *testing.T) {
		t.Parallel()
		h := &ExptLifecycleEventHandlerImpl{webhookDispatcher: nil}
		// Should not panic
		h.dispatchWebhook(ctx, &entity.ExptLifecycleEvent{}, &entity.Experiment{})
	})

	t.Run("dispatcher is set: calls Dispatch", func(t *testing.T) {
		t.Parallel()
		d := &fakeWebhookDispatcher{}
		h := &ExptLifecycleEventHandlerImpl{webhookDispatcher: d}
		h.dispatchWebhook(ctx, &entity.ExptLifecycleEvent{ExptID: 1}, &entity.Experiment{ID: 1})
		assert.True(t, d.called)
	})

	t.Run("dispatcher returns error: logs but does not panic", func(t *testing.T) {
		t.Parallel()
		d := &fakeWebhookDispatcher{err: errors.New("dispatch error")}
		h := &ExptLifecycleEventHandlerImpl{webhookDispatcher: d}
		// Should not panic or propagate error
		h.dispatchWebhook(ctx, &entity.ExptLifecycleEvent{ExptID: 2}, &entity.Experiment{ID: 2})
		assert.True(t, d.called)
	})
}
