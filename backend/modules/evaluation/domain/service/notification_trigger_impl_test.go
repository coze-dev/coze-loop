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

	componentMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	rpcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
)

type testNotificationTriggerMocks struct {
	webhookDeliverySvc *componentMocks.MockIWebhookDeliveryService
	notifyRPCAdapter   *rpcMocks.MockINotifyRPCAdapter
	userProvider       *rpcMocks.MockIUserProvider
	exptRepo           *repoMocks.MockIExperimentRepo
	exptStatsRepo      *repoMocks.MockIExptStatsRepo
}

func newTestNotificationTriggerService(ctrl *gomock.Controller) (*NotificationTriggerServiceImpl, *testNotificationTriggerMocks) {
	mocks := &testNotificationTriggerMocks{
		webhookDeliverySvc: componentMocks.NewMockIWebhookDeliveryService(ctrl),
		notifyRPCAdapter:   rpcMocks.NewMockINotifyRPCAdapter(ctrl),
		userProvider:       rpcMocks.NewMockIUserProvider(ctrl),
		exptRepo:           repoMocks.NewMockIExperimentRepo(ctrl),
		exptStatsRepo:      repoMocks.NewMockIExptStatsRepo(ctrl),
	}

	svc := &NotificationTriggerServiceImpl{
		webhookDeliverySvc: mocks.webhookDeliverySvc,
		notifyRPC:          mocks.notifyRPCAdapter,
		userProvider:       mocks.userProvider,
		exptRepo:           mocks.exptRepo,
		exptStatsRepo:      mocks.exptStatsRepo,
	}

	return svc, mocks
}

func TestNewNotificationTriggerService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mocks := &testNotificationTriggerMocks{
		webhookDeliverySvc: componentMocks.NewMockIWebhookDeliveryService(ctrl),
		notifyRPCAdapter:   rpcMocks.NewMockINotifyRPCAdapter(ctrl),
		userProvider:       rpcMocks.NewMockIUserProvider(ctrl),
		exptRepo:           repoMocks.NewMockIExperimentRepo(ctrl),
		exptStatsRepo:      repoMocks.NewMockIExptStatsRepo(ctrl),
	}

	svc := NewNotificationTriggerService(mocks.webhookDeliverySvc, mocks.notifyRPCAdapter, mocks.userProvider, mocks.exptRepo, mocks.exptStatsRepo)
	assert.NotNil(t, svc)

	impl, ok := svc.(*NotificationTriggerServiceImpl)
	assert.True(t, ok)
	assert.Equal(t, mocks.webhookDeliverySvc, impl.webhookDeliverySvc)
	assert.Equal(t, mocks.notifyRPCAdapter, impl.notifyRPC)
}

func TestMatchFilterCondition(t *testing.T) {
	t.Run("nil condition returns false", func(t *testing.T) {
		assert.False(t, MatchFilterCondition(nil, entity.ExptStatus_Success))
	})

	t.Run("empty values returns false", func(t *testing.T) {
		cond := &entity.NotificationFilterCondition{
			Operator: entity.NotificationOperatorIncludes,
			Values:   []entity.ExptStatus{},
		}
		assert.False(t, MatchFilterCondition(cond, entity.ExptStatus_Success))
	})

	t.Run("includes operator - status in values", func(t *testing.T) {
		cond := &entity.NotificationFilterCondition{
			Operator: entity.NotificationOperatorIncludes,
			Values:   []entity.ExptStatus{entity.ExptStatus_Success, entity.ExptStatus_Failed},
		}
		assert.True(t, MatchFilterCondition(cond, entity.ExptStatus_Success))
		assert.True(t, MatchFilterCondition(cond, entity.ExptStatus_Failed))
		assert.False(t, MatchFilterCondition(cond, entity.ExptStatus_Processing))
	})

	t.Run("excludes operator - status not in values", func(t *testing.T) {
		cond := &entity.NotificationFilterCondition{
			Operator: entity.NotificationOperatorExcludes,
			Values:   []entity.ExptStatus{entity.ExptStatus_Processing},
		}
		assert.True(t, MatchFilterCondition(cond, entity.ExptStatus_Success))
		assert.False(t, MatchFilterCondition(cond, entity.ExptStatus_Processing))
	})

	t.Run("terminated covers system_terminated - includes", func(t *testing.T) {
		cond := &entity.NotificationFilterCondition{
			Operator: entity.NotificationOperatorIncludes,
			Values:   []entity.ExptStatus{entity.ExptStatus_Terminated},
		}
		assert.True(t, MatchFilterCondition(cond, entity.ExptStatus_Terminated))
		assert.True(t, MatchFilterCondition(cond, entity.ExptStatus_SystemTerminated))
	})

	t.Run("terminated covers system_terminated - excludes", func(t *testing.T) {
		cond := &entity.NotificationFilterCondition{
			Operator: entity.NotificationOperatorExcludes,
			Values:   []entity.ExptStatus{entity.ExptStatus_Terminated},
		}
		assert.False(t, MatchFilterCondition(cond, entity.ExptStatus_Terminated))
		assert.False(t, MatchFilterCondition(cond, entity.ExptStatus_SystemTerminated))
	})

	t.Run("system_terminated in values does NOT cover terminated", func(t *testing.T) {
		cond := &entity.NotificationFilterCondition{
			Operator: entity.NotificationOperatorIncludes,
			Values:   []entity.ExptStatus{entity.ExptStatus_SystemTerminated},
		}
		assert.True(t, MatchFilterCondition(cond, entity.ExptStatus_SystemTerminated))
		assert.False(t, MatchFilterCondition(cond, entity.ExptStatus_Terminated))
	})

	t.Run("unknown operator returns false", func(t *testing.T) {
		cond := &entity.NotificationFilterCondition{
			Operator: entity.NotificationOperatorUnknown,
			Values:   []entity.ExptStatus{entity.ExptStatus_Success},
		}
		assert.False(t, MatchFilterCondition(cond, entity.ExptStatus_Success))
	})
}

func TestExptStatusToEventString(t *testing.T) {
	tests := []struct {
		status entity.ExptStatus
		want   string
	}{
		{entity.ExptStatus_Processing, "started"},
		{entity.ExptStatus_Success, "succeeded"},
		{entity.ExptStatus_Failed, "failed"},
		{entity.ExptStatus_Terminated, "terminated"},
		{entity.ExptStatus_SystemTerminated, "terminated"},
		{entity.ExptStatus_Pending, ""},
		{entity.ExptStatus_Unknown, ""},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, exptStatusToEventString(tt.status))
	}
}

func TestBuildWebhookDeliveryMessage(t *testing.T) {
	t.Run("valid event with stats", func(t *testing.T) {
		event := &entity.ExptLifecycleEvent{
			ExptID:   1,
			SpaceID:  100,
			ToStatus: entity.ExptStatus_Success,
		}
		expt := &entity.Experiment{
			ID:      1,
			SpaceID: 100,
			Name:    "test-experiment",
			Status:  entity.ExptStatus_Success,
			Stats: &entity.ExptStats{
				SuccessItemCnt:    8,
				FailItemCnt:       2,
				PendingItemCnt:    0,
				ProcessingItemCnt: 0,
				TerminatedItemCnt: 0,
			},
		}

		msg := BuildWebhookDeliveryMessage(event, expt, "https://example.com/webhook", 100)
		assert.NotNil(t, msg)
		assert.NotEmpty(t, msg.DeliveryID)
		assert.Equal(t, "https://example.com/webhook", msg.URL)
		assert.Equal(t, 0, msg.RetryCount)
		assert.Equal(t, int64(100), msg.SpaceID)

		assert.Equal(t, "succeeded", msg.Payload.Event)
		assert.Equal(t, msg.DeliveryID, msg.Payload.DeliveryID)
		assert.NotEmpty(t, msg.Payload.Timestamp)

		assert.Equal(t, int64(1), msg.Payload.Experiment.ID)
		assert.Equal(t, "test-experiment", msg.Payload.Experiment.Name)
		assert.Equal(t, "success", msg.Payload.Experiment.Status)
		assert.Equal(t, int32(10), msg.Payload.Experiment.Progress.Total)
		assert.Equal(t, int32(8), msg.Payload.Experiment.Progress.Succeeded)
		assert.Equal(t, int32(2), msg.Payload.Experiment.Progress.Failed)
	})

	t.Run("valid event without stats", func(t *testing.T) {
		event := &entity.ExptLifecycleEvent{
			ExptID:   1,
			SpaceID:  100,
			ToStatus: entity.ExptStatus_Processing,
		}
		expt := &entity.Experiment{
			ID:     1,
			Name:   "test-experiment",
			Status: entity.ExptStatus_Processing,
		}

		msg := BuildWebhookDeliveryMessage(event, expt, "https://example.com/webhook", 100)
		assert.NotNil(t, msg)
		assert.Equal(t, "started", msg.Payload.Event)
		assert.Nil(t, msg.Payload.Experiment.Progress)
	})

	t.Run("unmapped status returns nil", func(t *testing.T) {
		event := &entity.ExptLifecycleEvent{
			ExptID:   1,
			SpaceID:  100,
			ToStatus: entity.ExptStatus_Pending,
		}
		expt := &entity.Experiment{
			ID:     1,
			Name:   "test-experiment",
			Status: entity.ExptStatus_Pending,
		}

		msg := BuildWebhookDeliveryMessage(event, expt, "https://example.com/webhook", 100)
		assert.Nil(t, msg)
	})

	t.Run("terminated and system_terminated both map to terminated event", func(t *testing.T) {
		expt := &entity.Experiment{ID: 1, Name: "test", Status: entity.ExptStatus_Terminated}

		msg1 := BuildWebhookDeliveryMessage(&entity.ExptLifecycleEvent{ToStatus: entity.ExptStatus_Terminated}, expt, "https://example.com", 100)
		assert.Equal(t, "terminated", msg1.Payload.Event)

		msg2 := BuildWebhookDeliveryMessage(&entity.ExptLifecycleEvent{ToStatus: entity.ExptStatus_SystemTerminated}, expt, "https://example.com", 100)
		assert.Equal(t, "terminated", msg2.Payload.Event)
	})
}

func TestTriggerNotification(t *testing.T) {
	ctx := context.Background()

	t.Run("nil notification conf does nothing", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _ := newTestNotificationTriggerService(ctrl)

		err := svc.TriggerNotification(ctx, &entity.ExptLifecycleEvent{}, nil)
		assert.NoError(t, err)
	})

	t.Run("empty rules does nothing", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _ := newTestNotificationTriggerService(ctrl)

		err := svc.TriggerNotification(ctx, &entity.ExptLifecycleEvent{}, &entity.NotificationConf{Rules: nil})
		assert.NoError(t, err)
	})

	t.Run("get experiment error returns error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, mocks := newTestNotificationTriggerService(ctrl)

		event := &entity.ExptLifecycleEvent{ExptID: 1, SpaceID: 100, ToStatus: entity.ExptStatus_Success}
		conf := &entity.NotificationConf{Rules: []*entity.NotificationRule{{
			Condition: &entity.NotificationFilterCondition{
				Operator: entity.NotificationOperatorIncludes,
				Values:   []entity.ExptStatus{entity.ExptStatus_Success},
			},
		}}}

		mocks.exptRepo.EXPECT().GetByID(ctx, int64(1), int64(100)).Return(nil, errors.New("db error"))

		err := svc.TriggerNotification(ctx, event, conf)
		assert.EqualError(t, err, "db error")
	})

	t.Run("webhook delivery for matching rule", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, mocks := newTestNotificationTriggerService(ctrl)

		event := &entity.ExptLifecycleEvent{ExptID: 1, SpaceID: 100, ToStatus: entity.ExptStatus_Success}
		conf := &entity.NotificationConf{Rules: []*entity.NotificationRule{{
			Condition: &entity.NotificationFilterCondition{
				Operator: entity.NotificationOperatorIncludes,
				Values:   []entity.ExptStatus{entity.ExptStatus_Success},
			},
			Webhook: &entity.WebhookChannelConf{
				Enabled: true,
				URLs:    []string{"https://example.com/hook1", "https://example.com/hook2"},
			},
		}}}

		expt := &entity.Experiment{ID: 1, SpaceID: 100, Name: "test", Status: entity.ExptStatus_Success}
		stats := &entity.ExptStats{SuccessItemCnt: 5, FailItemCnt: 1}

		mocks.exptRepo.EXPECT().GetByID(ctx, int64(1), int64(100)).Return(expt, nil)
		mocks.exptStatsRepo.EXPECT().Get(ctx, int64(1), int64(100)).Return(stats, nil)
		mocks.webhookDeliverySvc.EXPECT().DeliverWebhook(ctx, gomock.Any()).Return(nil).Times(2)

		err := svc.TriggerNotification(ctx, event, conf)
		assert.NoError(t, err)
	})

	t.Run("feishu notification for matching rule", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, mocks := newTestNotificationTriggerService(ctrl)

		event := &entity.ExptLifecycleEvent{ExptID: 1, SpaceID: 100, ToStatus: entity.ExptStatus_Success}
		conf := &entity.NotificationConf{Rules: []*entity.NotificationRule{{
			Condition: &entity.NotificationFilterCondition{
				Operator: entity.NotificationOperatorIncludes,
				Values:   []entity.ExptStatus{entity.ExptStatus_Success},
			},
			Feishu: &entity.FeishuChannelConf{Enabled: true},
		}}}

		expt := &entity.Experiment{ID: 1, SpaceID: 100, Name: "test", Status: entity.ExptStatus_Success, CreatedBy: "user1"}

		mocks.exptRepo.EXPECT().GetByID(ctx, int64(1), int64(100)).Return(expt, nil)
		mocks.exptStatsRepo.EXPECT().Get(ctx, int64(1), int64(100)).Return(nil, nil)
		mocks.userProvider.EXPECT().MGetUserInfo(ctx, []string{"user1"}).Return([]*entity.UserInfo{
			{Email: gptr.Of("user1@example.com")},
		}, nil)
		mocks.notifyRPCAdapter.EXPECT().SendMessageCard(ctx, "user1@example.com", gomock.Any(), gomock.Any()).Return(nil)

		err := svc.TriggerNotification(ctx, event, conf)
		assert.NoError(t, err)
	})

	t.Run("non-matching rule does not trigger", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, mocks := newTestNotificationTriggerService(ctrl)

		event := &entity.ExptLifecycleEvent{ExptID: 1, SpaceID: 100, ToStatus: entity.ExptStatus_Processing}
		conf := &entity.NotificationConf{Rules: []*entity.NotificationRule{{
			Condition: &entity.NotificationFilterCondition{
				Operator: entity.NotificationOperatorIncludes,
				Values:   []entity.ExptStatus{entity.ExptStatus_Success, entity.ExptStatus_Failed},
			},
			Webhook: &entity.WebhookChannelConf{
				Enabled: true,
				URLs:    []string{"https://example.com/hook"},
			},
		}}}

		expt := &entity.Experiment{ID: 1, SpaceID: 100, Name: "test", Status: entity.ExptStatus_Processing}

		mocks.exptRepo.EXPECT().GetByID(ctx, int64(1), int64(100)).Return(expt, nil)
		mocks.exptStatsRepo.EXPECT().Get(ctx, int64(1), int64(100)).Return(nil, nil)
		// No webhook delivery expected

		err := svc.TriggerNotification(ctx, event, conf)
		assert.NoError(t, err)
	})

	t.Run("webhook disabled does not trigger", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, mocks := newTestNotificationTriggerService(ctrl)

		event := &entity.ExptLifecycleEvent{ExptID: 1, SpaceID: 100, ToStatus: entity.ExptStatus_Success}
		conf := &entity.NotificationConf{Rules: []*entity.NotificationRule{{
			Condition: &entity.NotificationFilterCondition{
				Operator: entity.NotificationOperatorIncludes,
				Values:   []entity.ExptStatus{entity.ExptStatus_Success},
			},
			Webhook: &entity.WebhookChannelConf{
				Enabled: false,
				URLs:    []string{"https://example.com/hook"},
			},
		}}}

		expt := &entity.Experiment{ID: 1, SpaceID: 100, Name: "test", Status: entity.ExptStatus_Success}

		mocks.exptRepo.EXPECT().GetByID(ctx, int64(1), int64(100)).Return(expt, nil)
		mocks.exptStatsRepo.EXPECT().Get(ctx, int64(1), int64(100)).Return(nil, nil)
		// No delivery expected since webhook is disabled

		err := svc.TriggerNotification(ctx, event, conf)
		assert.NoError(t, err)
	})

	t.Run("webhook delivery error does not block other notifications", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, mocks := newTestNotificationTriggerService(ctrl)

		event := &entity.ExptLifecycleEvent{ExptID: 1, SpaceID: 100, ToStatus: entity.ExptStatus_Success}
		conf := &entity.NotificationConf{Rules: []*entity.NotificationRule{{
			Condition: &entity.NotificationFilterCondition{
				Operator: entity.NotificationOperatorIncludes,
				Values:   []entity.ExptStatus{entity.ExptStatus_Success},
			},
			Webhook: &entity.WebhookChannelConf{
				Enabled: true,
				URLs:    []string{"https://example.com/hook1", "https://example.com/hook2"},
			},
		}}}

		expt := &entity.Experiment{ID: 1, SpaceID: 100, Name: "test", Status: entity.ExptStatus_Success}

		mocks.exptRepo.EXPECT().GetByID(ctx, int64(1), int64(100)).Return(expt, nil)
		mocks.exptStatsRepo.EXPECT().Get(ctx, int64(1), int64(100)).Return(nil, nil)
		// First delivery fails, second succeeds
		gomock.InOrder(
			mocks.webhookDeliverySvc.EXPECT().DeliverWebhook(ctx, gomock.Any()).Return(errors.New("delivery failed")),
			mocks.webhookDeliverySvc.EXPECT().DeliverWebhook(ctx, gomock.Any()).Return(nil),
		)

		err := svc.TriggerNotification(ctx, event, conf)
		assert.NoError(t, err) // should not propagate webhook delivery errors
	})

	t.Run("feishu skipped when no creator", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, mocks := newTestNotificationTriggerService(ctrl)

		event := &entity.ExptLifecycleEvent{ExptID: 1, SpaceID: 100, ToStatus: entity.ExptStatus_Success}
		conf := &entity.NotificationConf{Rules: []*entity.NotificationRule{{
			Condition: &entity.NotificationFilterCondition{
				Operator: entity.NotificationOperatorIncludes,
				Values:   []entity.ExptStatus{entity.ExptStatus_Success},
			},
			Feishu: &entity.FeishuChannelConf{Enabled: true},
		}}}

		expt := &entity.Experiment{ID: 1, SpaceID: 100, Name: "test", Status: entity.ExptStatus_Success, CreatedBy: ""}

		mocks.exptRepo.EXPECT().GetByID(ctx, int64(1), int64(100)).Return(expt, nil)
		mocks.exptStatsRepo.EXPECT().Get(ctx, int64(1), int64(100)).Return(nil, nil)
		// userProvider and notifyRPC should NOT be called

		err := svc.TriggerNotification(ctx, event, conf)
		assert.NoError(t, err)
	})

	t.Run("stats error does not block notification", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, mocks := newTestNotificationTriggerService(ctrl)

		event := &entity.ExptLifecycleEvent{ExptID: 1, SpaceID: 100, ToStatus: entity.ExptStatus_Success}
		conf := &entity.NotificationConf{Rules: []*entity.NotificationRule{{
			Condition: &entity.NotificationFilterCondition{
				Operator: entity.NotificationOperatorIncludes,
				Values:   []entity.ExptStatus{entity.ExptStatus_Success},
			},
			Webhook: &entity.WebhookChannelConf{
				Enabled: true,
				URLs:    []string{"https://example.com/hook"},
			},
		}}}

		expt := &entity.Experiment{ID: 1, SpaceID: 100, Name: "test", Status: entity.ExptStatus_Success}

		mocks.exptRepo.EXPECT().GetByID(ctx, int64(1), int64(100)).Return(expt, nil)
		mocks.exptStatsRepo.EXPECT().Get(ctx, int64(1), int64(100)).Return(nil, errors.New("stats error"))
		mocks.webhookDeliverySvc.EXPECT().DeliverWebhook(ctx, gomock.Any()).Return(nil)

		err := svc.TriggerNotification(ctx, event, conf)
		assert.NoError(t, err)
	})

	t.Run("multiple rules with mixed matching", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, mocks := newTestNotificationTriggerService(ctrl)

		event := &entity.ExptLifecycleEvent{ExptID: 1, SpaceID: 100, ToStatus: entity.ExptStatus_Success}
		conf := &entity.NotificationConf{Rules: []*entity.NotificationRule{
			{
				// This rule matches (includes success)
				Condition: &entity.NotificationFilterCondition{
					Operator: entity.NotificationOperatorIncludes,
					Values:   []entity.ExptStatus{entity.ExptStatus_Success},
				},
				Webhook: &entity.WebhookChannelConf{
					Enabled: true,
					URLs:    []string{"https://example.com/hook1"},
				},
			},
			{
				// This rule does NOT match (includes only failed)
				Condition: &entity.NotificationFilterCondition{
					Operator: entity.NotificationOperatorIncludes,
					Values:   []entity.ExptStatus{entity.ExptStatus_Failed},
				},
				Webhook: &entity.WebhookChannelConf{
					Enabled: true,
					URLs:    []string{"https://example.com/hook2"},
				},
			},
		}}

		expt := &entity.Experiment{ID: 1, SpaceID: 100, Name: "test", Status: entity.ExptStatus_Success}

		mocks.exptRepo.EXPECT().GetByID(ctx, int64(1), int64(100)).Return(expt, nil)
		mocks.exptStatsRepo.EXPECT().Get(ctx, int64(1), int64(100)).Return(nil, nil)
		// Only 1 delivery for the matching rule
		mocks.webhookDeliverySvc.EXPECT().DeliverWebhook(ctx, gomock.Any()).Return(nil).Times(1)

		err := svc.TriggerNotification(ctx, event, conf)
		assert.NoError(t, err)
	})

	t.Run("nil rule or condition is skipped", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, mocks := newTestNotificationTriggerService(ctrl)

		event := &entity.ExptLifecycleEvent{ExptID: 1, SpaceID: 100, ToStatus: entity.ExptStatus_Success}
		conf := &entity.NotificationConf{Rules: []*entity.NotificationRule{
			nil,
			{Condition: nil},
			{
				Condition: &entity.NotificationFilterCondition{
					Operator: entity.NotificationOperatorIncludes,
					Values:   []entity.ExptStatus{entity.ExptStatus_Success},
				},
				Webhook: &entity.WebhookChannelConf{Enabled: true, URLs: []string{"https://example.com/hook"}},
			},
		}}

		expt := &entity.Experiment{ID: 1, SpaceID: 100, Name: "test", Status: entity.ExptStatus_Success}

		mocks.exptRepo.EXPECT().GetByID(ctx, int64(1), int64(100)).Return(expt, nil)
		mocks.exptStatsRepo.EXPECT().Get(ctx, int64(1), int64(100)).Return(nil, nil)
		mocks.webhookDeliverySvc.EXPECT().DeliverWebhook(ctx, gomock.Any()).Return(nil).Times(1)

		err := svc.TriggerNotification(ctx, event, conf)
		assert.NoError(t, err)
	})
}
