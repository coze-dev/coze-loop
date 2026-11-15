// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	centraldao "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/dao"
)

type recordingPublisher struct {
	events.ExptEventPublisher
	scheduleEvents []*entity.ExptScheduleEvent
}

func (p *recordingPublisher) PublishExptScheduleEvent(_ context.Context, event *entity.ExptScheduleEvent, _ *time.Duration) error {
	p.scheduleEvents = append(p.scheduleEvents, event)
	return nil
}

func (p *recordingPublisher) PublishExptRecordEvalEvent(context.Context, *entity.ExptItemEvalEvent, *time.Duration, func(*entity.ExptItemEvalEvent)) error {
	return nil
}
func (p *recordingPublisher) BatchPublishExptRecordEvalEvent(context.Context, []*entity.ExptItemEvalEvent, *time.Duration) error {
	return nil
}
func (p *recordingPublisher) PublishExptAggrCalculateEvent(context.Context, []*entity.AggrCalculateEvent, *time.Duration) error {
	return nil
}
func (p *recordingPublisher) PublishExptOnlineEvalResult(context.Context, *entity.OnlineExptEvalResultEvent, *time.Duration) error {
	return nil
}
func (p *recordingPublisher) PublishExptTurnResultFilterEvent(context.Context, *entity.ExptTurnResultFilterEvent, *time.Duration) error {
	return nil
}
func (p *recordingPublisher) PublishExptExportCSVEvent(context.Context, *entity.ExportCSVEvent, *time.Duration) error {
	return nil
}
func (p *recordingPublisher) PublishExptLifecycleEvent(context.Context, *entity.ExptLifecycleEvent, *time.Duration, string) error {
	return nil
}
func (p *recordingPublisher) PublishExptWebhookNotifyEvent(context.Context, *entity.WebhookRetryEvent, *time.Duration) error {
	return nil
}

func TestCentralTickerPublishesOneShotInPriorityOrder(t *testing.T) {
	ctx := context.Background()
	dao := centraldao.NewCentralSchedulingDAO(redis.NewTestRedis(t))
	scope := entity.CentralEnvScope{Env: "test_psm", Lane: "ppe_unit"}
	require.NoError(t, dao.RegisterActive(ctx, scope, &entity.CentralActiveRun{SpaceID: 1, ExptID: 10, ExptRunID: 100, Priority: 50}, time.Hour))
	require.NoError(t, dao.RegisterActive(ctx, scope, &entity.CentralActiveRun{SpaceID: 1, ExptID: 20, ExptRunID: 200, Priority: 5, SessionUserID: "u20"}, time.Hour))

	provider := &fakeCentralConfigProvider{cfg: &entity.CentralSchedulerConfig{Enabled: true}}
	pub := &recordingPublisher{}
	ticker := NewCentralTicker(dao, pub, provider, func(context.Context) entity.CentralEnvScope { return scope })

	require.NoError(t, ticker.Tick(ctx))
	require.Len(t, pub.scheduleEvents, 2)
	assert.Equal(t, int64(20), pub.scheduleEvents[0].ExptID, "priority 5 must be published first")
	assert.Equal(t, int64(10), pub.scheduleEvents[1].ExptID)
	for _, event := range pub.scheduleEvents {
		assert.True(t, event.OneShot, "central tick events must be one-shot")
	}
	assert.Equal(t, "u20", pub.scheduleEvents[0].Session.UserID)
}

func TestCentralTickerDisabledDoesNothing(t *testing.T) {
	ctx := context.Background()
	dao := centraldao.NewCentralSchedulingDAO(redis.NewTestRedis(t))
	pub := &recordingPublisher{}
	ticker := NewCentralTicker(dao, pub, &fakeCentralConfigProvider{cfg: &entity.CentralSchedulerConfig{Enabled: false}}, nil)

	require.NoError(t, ticker.Tick(ctx))
	assert.Empty(t, pub.scheduleEvents)
}
