// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// WebhookDeliveryService 负责构造 Webhook payload、签名、投递
type WebhookDeliveryService struct {
	secretProvider rpc.IWebhookSecretProvider
	httpClient     rpc.IWebhookHTTPClient
	publisher      events.ExptEventPublisher
}

// NewWebhookDeliveryService 创建 WebhookDeliveryService
func NewWebhookDeliveryService(
	secretProvider rpc.IWebhookSecretProvider,
	httpClient rpc.IWebhookHTTPClient,
	publisher events.ExptEventPublisher,
) *WebhookDeliveryService {
	return &WebhookDeliveryService{
		secretProvider: secretProvider,
		httpClient:     httpClient,
		publisher:      publisher,
	}
}

// WebhookPayload Webhook 投递 JSON Body
type WebhookPayload struct {
	DeliveryID string          `json:"delivery_id"`
	Event      string          `json:"event"`
	Timestamp  int64           `json:"timestamp"`
	Experiment *ExptPayloadInfo `json:"experiment"`
}

// ExptPayloadInfo 实验信息（包含在 webhook payload 中）
type ExptPayloadInfo struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// DeliverFirstAttempt 首次投递：构造 payload/签名，执行 HTTP POST，失败则发布重试消息
func (s *WebhookDeliveryService) DeliverFirstAttempt(ctx context.Context, expt *entity.Experiment, trigger entity.NotificationTrigger, url string) {
	deliveryID := fmt.Sprintf("d_%s", uuid.New().String())
	timestamp := time.Now().Unix()

	payload := &WebhookPayload{
		DeliveryID: deliveryID,
		Event:      trigger,
		Timestamp:  timestamp,
		Experiment: &ExptPayloadInfo{
			ID:     expt.ID,
			Name:   expt.Name,
			Status: statusString(expt.Status),
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		logs.CtxWarn(ctx, "webhook payload marshal fail, expt_id: %d, url: %s, err: %v", expt.ID, url, err)
		return
	}

	// 计算签名
	signature := s.computeSignature(ctx, expt.SpaceID, timestamp, payloadBytes)

	headers := map[string]string{
		"Content-Type":       "application/json",
		"X-Fornax-Signature": signature,
		"X-Fornax-Timestamp": strconv.FormatInt(timestamp, 10),
		"X-Fornax-Delivery":  deliveryID,
	}

	statusCode, err := s.httpClient.DoPost(ctx, url, payloadBytes, headers)
	if err != nil {
		logs.CtxWarn(ctx, "webhook delivery fail, delivery_id: %s, url: %s, status: %d, err: %v", deliveryID, url, statusCode, err)
		// 发布重试消息
		s.publishRetryEvent(ctx, &entity.WebhookRetryEvent{
			DeliveryID: deliveryID,
			ExptID:     expt.ID,
			SpaceID:    expt.SpaceID,
			Event:      trigger,
			URL:        url,
			Payload:    string(payloadBytes),
			Timestamp:  timestamp,
			RetryCount: 0,
			MaxRetries: entity.MaxWebhookRetries,
		})
		return
	}

	logs.CtxInfo(ctx, "webhook delivery success, delivery_id: %s, url: %s, status: %d", deliveryID, url, statusCode)
}

// DeliverRetry 重试投递：使用已有的 payload 和 delivery_id
func (s *WebhookDeliveryService) DeliverRetry(ctx context.Context, event *entity.WebhookRetryEvent) {
	// 重新计算签名（使用原始 timestamp + payload）
	signature := s.computeSignature(ctx, event.SpaceID, event.Timestamp, []byte(event.Payload))

	headers := map[string]string{
		"Content-Type":       "application/json",
		"X-Fornax-Signature": signature,
		"X-Fornax-Timestamp": strconv.FormatInt(event.Timestamp, 10),
		"X-Fornax-Delivery":  event.DeliveryID,
	}

	statusCode, err := s.httpClient.DoPost(ctx, event.URL, []byte(event.Payload), headers)
	if err != nil {
		logs.CtxWarn(ctx, "webhook retry delivery fail, delivery_id: %s, url: %s, retry_count: %d, status: %d, err: %v",
			event.DeliveryID, event.URL, event.RetryCount, statusCode, err)

		if event.RetryCount+1 < event.MaxRetries {
			nextEvent := *event
			nextEvent.RetryCount = event.RetryCount + 1
			s.publishRetryEvent(ctx, &nextEvent)
		} else {
			logs.CtxError(ctx, "webhook delivery final failure, delivery_id: %s, url: %s, retry_count: %d, err: %v",
				event.DeliveryID, event.URL, event.RetryCount, err)
		}
		return
	}

	logs.CtxInfo(ctx, "webhook retry delivery success, delivery_id: %s, url: %s, retry_count: %d, status: %d",
		event.DeliveryID, event.URL, event.RetryCount, statusCode)
}

func (s *WebhookDeliveryService) publishRetryEvent(ctx context.Context, event *entity.WebhookRetryEvent) {
	delay := rpc.RetryDelayForAttempt(event.RetryCount)
	if err := s.publisher.PublishWebhookRetryEvent(ctx, event, &delay); err != nil {
		logs.CtxError(ctx, "publish webhook retry event fail, delivery_id: %s, err: %v", event.DeliveryID, err)
	}
}

func (s *WebhookDeliveryService) computeSignature(ctx context.Context, spaceID int64, timestamp int64, payload []byte) string {
	sk, err := s.secretProvider.GetSpaceSK(ctx, spaceID)
	if err != nil || sk == "" {
		if err != nil {
			logs.CtxWarn(ctx, "get space sk fail, space_id: %d, err: %v, skip signature", spaceID, err)
		}
		return ""
	}

	signContent := strconv.FormatInt(timestamp, 10) + "\n" + string(payload)
	mac := hmac.New(sha256.New, []byte(sk))
	mac.Write([]byte(signContent))
	return hex.EncodeToString(mac.Sum(nil))
}

func statusString(status entity.ExptStatus) string {
	switch status {
	case entity.ExptStatus_Pending:
		return "pending"
	case entity.ExptStatus_Processing:
		return "processing"
	case entity.ExptStatus_Success:
		return "succeeded"
	case entity.ExptStatus_Failed:
		return "failed"
	case entity.ExptStatus_Terminated:
		return "terminated"
	case entity.ExptStatus_SystemTerminated:
		return "system_terminated"
	default:
		return "unknown"
	}
}
