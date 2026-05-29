// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	infrawebhook "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/webhook"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// IWebhookDispatcher Webhook 通知分发器接口
type IWebhookDispatcher interface {
	// Dispatch 根据通知配置匹配触发条件并发送 Webhook
	Dispatch(ctx context.Context, event *entity.ExptLifecycleEvent, expt *entity.Experiment) error
}

// SecretProvider 签名密钥提供者接口
type SecretProvider interface {
	GetSecret(ctx context.Context, spaceID int64) (string, error)
}

// RetryPublisher 重试事件发布者接口
type RetryPublisher interface {
	PublishRetry(ctx context.Context, event *entity.WebhookRetryEvent, delay time.Duration) error
}

// retryDelays 重试间隔：1min → 5min → 30min
var retryDelays = []time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	30 * time.Minute,
}

// Dispatcher WebhookDispatcher 实现
type Dispatcher struct {
	sender         *infrawebhook.Sender
	secretProvider SecretProvider
	retryPublisher RetryPublisher
}

// NewDispatcher 创建 WebhookDispatcher
func NewDispatcher(secretProvider SecretProvider, retryPublisher RetryPublisher) *Dispatcher {
	return &Dispatcher{
		sender:         infrawebhook.NewSender(),
		secretProvider: secretProvider,
		retryPublisher: retryPublisher,
	}
}

// Dispatch 分发 Webhook 通知
func (d *Dispatcher) Dispatch(ctx context.Context, event *entity.ExptLifecycleEvent, expt *entity.Experiment) error {
	if expt == nil || expt.NotificationConf == nil {
		return nil
	}

	conf := expt.NotificationConf
	if !conf.ShouldWebhook(event.ToStatus) {
		return nil
	}

	// 构造 payload
	eventType := entity.ExptStatusToWebhookEvent(event.ToStatus)
	if eventType == "" {
		return nil
	}

	deliveryID := "evt_" + uuid.New().String()
	payload := &entity.WebhookPayload{
		DeliveryID:   deliveryID,
		CreateTime:   time.Now().Format(time.RFC3339),
		EventType:    eventType,
		ResourceType: "experiment",
		Summary:      buildSummary(eventType),
		Data: &entity.WebhookPayloadData{
			ExperimentID:   strconv.FormatInt(expt.ID, 10),
			ExperimentName: expt.Name,
			Status:         string(eventType),
			Progress:       buildProgress(expt),
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload failed: %w", err)
	}

	// 获取签名密钥
	secret, err := d.secretProvider.GetSecret(ctx, expt.SpaceID)
	if err != nil {
		logs.CtxWarn(ctx, "[Webhook] get secret failed, space_id: %d, err: %v", expt.SpaceID, err)
		secret = "" // 无密钥时不签名
	}

	// 发送
	result := d.sender.Send(ctx, conf.Webhook.URL, body, secret)
	if result.IsSuccess() {
		logs.CtxInfo(ctx, "[Webhook] sent successfully, expt_id: %d, delivery_id: %s", expt.ID, deliveryID)
		return nil
	}

	// 发送失败，触发重试
	logs.CtxWarn(ctx, "[Webhook] send failed, expt_id: %d, delivery_id: %s, status: %d, err: %v",
		expt.ID, deliveryID, result.StatusCode, result.Err)

	if d.retryPublisher != nil && len(retryDelays) > 0 {
		retryEvent := &entity.WebhookRetryEvent{
			DeliveryID: deliveryID,
			AttemptNum: 1,
		}
		if err := d.retryPublisher.PublishRetry(ctx, retryEvent, retryDelays[0]); err != nil {
			logs.CtxWarn(ctx, "[Webhook] publish retry failed, delivery_id: %s, err: %v", deliveryID, err)
		}
	}

	return nil
}

func buildSummary(eventType entity.WebhookEventType) string {
	switch eventType {
	case entity.WebhookEventStarted:
		return "Experiment started"
	case entity.WebhookEventSucceeded:
		return "Experiment completed successfully"
	case entity.WebhookEventFailed:
		return "Experiment failed"
	case entity.WebhookEventTerminated:
		return "Experiment terminated"
	default:
		return "Experiment status changed"
	}
}

func buildProgress(expt *entity.Experiment) *entity.WebhookPayloadProgress {
	if expt.Stats == nil {
		return &entity.WebhookPayloadProgress{}
	}
	s := expt.Stats
	return &entity.WebhookPayloadProgress{
		Total:     int64(s.PendingItemCnt) + int64(s.SuccessItemCnt) + int64(s.FailItemCnt) + int64(s.ProcessingItemCnt) + int64(s.TerminatedItemCnt),
		Succeeded: int64(s.SuccessItemCnt),
		Failed:    int64(s.FailItemCnt),
	}
}
