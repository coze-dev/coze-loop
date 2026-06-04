// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

func TestDispatcherDispatchesBitsCallbackForWorkflowTerminalEvent(t *testing.T) {
	repo := &fakeDeliveryRepo{}
	publisher := &fakeDeliveryPublisher{}
	dispatcher := NewWebhookDispatcher(
		repo,
		publisher,
		&fakeWebhookConfiger{
			global: &entity.WebhookGlobalConf{
				Enable:                  true,
				Secret:                  "secret",
				BitsCallbackURLTemplate: "https://bits.example.com/callback?workflow_id={source_id}&experiment_id={experiment_id}",
			},
			retry: &entity.WebhookRetryConf{MaxRetries: 3},
		},
	)

	err := dispatcher.Dispatch(
		context.Background(),
		&entity.Experiment{
			ID:         123,
			SpaceID:    456,
			CreatedBy:  "789",
			Status:     entity.ExptStatus_Success,
			SourceType: entity.SourceType_Workflow,
			SourceID:   "workflow-1",
		},
		&entity.ExptLifecycleEvent{
			ExptID:   123,
			SpaceID:  456,
			ToStatus: entity.ExptStatus_Success,
		},
	)

	require.NoError(t, err)
	require.Len(t, repo.created, 1)
	require.Len(t, publisher.published, 1)
	assert.Equal(t, "bits_callback", repo.created[0].ChannelType)
	assert.Equal(t, "https://bits.example.com/callback?workflow_id=workflow-1&experiment_id=123", repo.created[0].WebhookURL)
	assert.Equal(t, entity.WebhookEventSucceeded, repo.created[0].EventType)
	assert.Equal(t, "bits_callback", publisher.published[0].SourceType)
	assert.Equal(t, repo.created[0].DeliveryID, publisher.published[0].DeliveryID)
}

type fakeDeliveryRepo struct {
	created []*entity.WebhookDelivery
}

func (f *fakeDeliveryRepo) Create(ctx context.Context, delivery *entity.WebhookDelivery, opts ...db.Option) error {
	f.created = append(f.created, delivery)
	return nil
}

func (f *fakeDeliveryRepo) Update(ctx context.Context, delivery *entity.WebhookDelivery, opts ...db.Option) error {
	return nil
}

func (f *fakeDeliveryRepo) GetByDeliveryID(ctx context.Context, deliveryID string, opts ...db.Option) (*entity.WebhookDelivery, error) {
	return nil, nil
}

func (f *fakeDeliveryRepo) ListByExptID(ctx context.Context, params repo.ListDeliveryParams, opts ...db.Option) ([]*entity.WebhookDelivery, int64, error) {
	return nil, 0, nil
}

func (f *fakeDeliveryRepo) ListRetryable(ctx context.Context, params repo.ListRetryableParams, opts ...db.Option) ([]*entity.WebhookDelivery, error) {
	return nil, nil
}

type fakeDeliveryPublisher struct {
	published []*entity.WebhookDeliveryMessage
}

func (f *fakeDeliveryPublisher) PublishWebhookDeliveryEvent(ctx context.Context, event *entity.WebhookDeliveryMessage, duration *time.Duration) error {
	f.published = append(f.published, event)
	return nil
}

type fakeWebhookConfiger struct {
	global *entity.WebhookGlobalConf
	retry  *entity.WebhookRetryConf
}

func (f *fakeWebhookConfiger) GetWebhookConf(ctx context.Context) *entity.WebhookGlobalConf {
	return f.global
}

func (f *fakeWebhookConfiger) GetWebhookRetryConf(ctx context.Context) *entity.WebhookRetryConf {
	return f.retry
}

func (f *fakeWebhookConfiger) GetWebhookRateLimitConf(ctx context.Context) *entity.WebhookRateLimitConf {
	return entity.DefaultWebhookRateLimitConf()
}

func (f *fakeWebhookConfiger) GetWebhookURLLimitConf(ctx context.Context) *entity.WebhookURLLimitConf {
	return entity.DefaultWebhookURLLimitConf()
}

func (f *fakeWebhookConfiger) GetWebhookSecurityConf(ctx context.Context) *entity.WebhookSecurityConf {
	return entity.DefaultWebhookSecurityConf()
}
