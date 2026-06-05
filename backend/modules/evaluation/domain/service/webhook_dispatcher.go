// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// IWebhookDispatcher Webhook 分发器接口
type IWebhookDispatcher interface {
	Dispatch(ctx context.Context, event *entity.ExptLifecycleEvent, expt *entity.Experiment) error
}

// IWebhookSecretProvider 签名密钥提供者接口
type IWebhookSecretProvider interface {
	GetSecret(ctx context.Context, spaceID int64) (string, error)
}

// NoopWebhookSecretProvider 默认无签名密钥提供者（开源版默认实现）
type NoopWebhookSecretProvider struct{}

func NewNoopWebhookSecretProvider() *NoopWebhookSecretProvider {
	return &NoopWebhookSecretProvider{}
}

func (p *NoopWebhookSecretProvider) GetSecret(ctx context.Context, spaceID int64) (string, error) {
	return "", nil
}

// WebhookDispatcher Webhook 分发器实现
type WebhookDispatcher struct {
	httpClient     *http.Client
	publisher      events.ExptEventPublisher
	secretProvider IWebhookSecretProvider
}

// NewWebhookDispatcher 创建 WebhookDispatcher
func NewWebhookDispatcher(publisher events.ExptEventPublisher, secretProvider IWebhookSecretProvider) *WebhookDispatcher {
	return &WebhookDispatcher{
		httpClient:     &http.Client{Timeout: 5 * time.Second},
		publisher:      publisher,
		secretProvider: secretProvider,
	}
}

func (d *WebhookDispatcher) Dispatch(ctx context.Context, event *entity.ExptLifecycleEvent, expt *entity.Experiment) error {
	if expt == nil || expt.NotificationConf == nil {
		return nil
	}

	webhook := expt.NotificationConf.Webhook
	if webhook == nil || !webhook.Enable {
		return nil
	}

	// 匹配 filter 条件
	if expt.NotificationConf.Filter != nil {
		if !matchNotificationFilter(ctx, expt.NotificationConf.Filter, event.ToStatus) {
			return nil
		}
	}

	urls := parseWebhookURLs(webhook.Urls)
	if len(urls) == 0 {
		return nil
	}

	logs.CtxInfo(ctx, "webhook_dispatcher: dispatching, expt_id: %v, space_id: %v, from: %v, to: %v, urls: %v",
		expt.ID, expt.SpaceID, event.FromStatus, event.ToStatus, urls)

	// 构造 payload
	payload := buildWebhookPayload(event, expt)
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		logs.CtxError(ctx, "webhook_dispatcher: marshal payload failed, expt_id: %v, err: %v", expt.ID, err)
		return err
	}

	// 签名参数
	var secret string
	if d.secretProvider != nil {
		secret, _ = d.secretProvider.GetSecret(ctx, expt.SpaceID)
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := generateNonce()
	signMessage := timestamp + "\n" + nonce + "\n"
	signature := computeHMACSHA256(secret, signMessage)

	deliveryID := payload["delivery_id"].(string)
	payloadStr := string(payloadBytes)

	// 逐个调用 Webhook URL
	for _, url := range urls {
		if err := d.doPost(ctx, url, payloadBytes, timestamp, nonce, signature); err != nil {
			logs.CtxError(ctx, "webhook_dispatcher: post failed, url: %v, expt_id: %v, err: %v", url, expt.ID, err)
			// 发布重试事件
			retryEvent := &entity.WebhookRetryEvent{
				ExptID:     expt.ID,
				SpaceID:    expt.SpaceID,
				DeliveryID: deliveryID,
				WebhookURL: url,
				Payload:    payloadStr,
				AttemptNum: 1,
			}
			retryDuration := 1 * time.Minute
			if pubErr := d.publisher.PublishExptWebhookNotifyEvent(ctx, retryEvent, &retryDuration); pubErr != nil {
				logs.CtxError(ctx, "webhook_dispatcher: publish retry event failed, expt_id: %v, url: %v, err: %v", expt.ID, url, pubErr)
			}
		}
	}

	return nil
}

func (d *WebhookDispatcher) doPost(ctx context.Context, url string, body []byte, timestamp, nonce, signature string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CozeLoop-Timestamp", timestamp)
	req.Header.Set("X-CozeLoop-Nonce", nonce)
	req.Header.Set("X-CozeLoop-Signature", signature)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned non-2xx status: %d", resp.StatusCode)
	}
	return nil
}

// computeHMACSHA256 计算 HMAC-SHA256 签名，返回 hex 编码
func computeHMACSHA256(secret, message string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

// generateNonce 生成随机字符串
func generateNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// buildWebhookPayload 构造 webhook payload
func buildWebhookPayload(event *entity.ExptLifecycleEvent, expt *entity.Experiment) map[string]interface{} {
	eventType := mapExptStatusToEventType(event.ToStatus)
	summary := mapExptStatusToSummary(event.ToStatus)

	progress := map[string]interface{}{
		"total":     int32(0),
		"succeeded": int32(0),
		"failed":    int32(0),
	}
	if expt.Stats != nil {
		total := expt.Stats.SuccessItemCnt + expt.Stats.FailItemCnt + expt.Stats.PendingItemCnt + expt.Stats.ProcessingItemCnt + expt.Stats.TerminatedItemCnt
		progress["total"] = total
		progress["succeeded"] = expt.Stats.SuccessItemCnt
		progress["failed"] = expt.Stats.FailItemCnt
	}

	// deliveryID 直接透传 lifecycle 消息的 IdempotentKey，保证全链路幂等一致
	deliveryID := event.IdempotentKey

	return map[string]interface{}{
		"delivery_id":   deliveryID,
		"create_time":   time.Now().Format(time.RFC3339),
		"event_type":    eventType,
		"resource_type": "experiment",
		"summary":       summary,
		"data": map[string]interface{}{
			"experiment_id":   strconv.FormatInt(expt.ID, 10),
			"experiment_name": expt.Name,
			"status":          eventTypeToStatus(eventType),
			"progress":        progress,
		},
	}
}

// matchNotificationFilter 匹配通知过滤条件
func matchNotificationFilter(ctx context.Context, filter *entity.NotificationFilter, toStatus entity.ExptStatus) bool {
	if filter == nil || len(filter.FilterConditions) == 0 {
		logs.CtxInfo(ctx, "matchNotificationFilter: no filter or empty conditions, default match, to_status: %v", toStatus)
		return true
	}

	statusStr := strconv.FormatInt(int64(toStatus), 10)
	logs.CtxInfo(ctx, "matchNotificationFilter: to_status: %v, conditions_count: %d", toStatus, len(filter.FilterConditions))

	for i, cond := range filter.FilterConditions {
		if cond.Field == nil || cond.Field.FieldType != entity.NotificationFieldType_ExptStatus {
			logs.CtxInfo(ctx, "matchNotificationFilter: skip condition[%d], field_type not ExptStatus", i)
			continue
		}

		values := parseFilterValues(cond.Value)
		if len(values) == 0 {
			logs.CtxWarn(ctx, "matchNotificationFilter: condition[%d] parse values failed or empty, raw: %s", i, cond.Value)
			continue
		}
		matched := false
		for _, v := range values {
			if strings.TrimSpace(v) == statusStr {
				matched = true
				break
			}
		}

		logs.CtxInfo(ctx, "matchNotificationFilter: condition[%d] operator: %v, values: %v, status: %s, matched: %v",
			i, cond.Operator, values, statusStr, matched)

		switch cond.Operator {
		case entity.NotificationOperatorType_Equal, entity.NotificationOperatorType_In:
			if matched {
				logs.CtxInfo(ctx, "matchNotificationFilter: condition[%d] Equal/In matched, return true", i)
				return true
			}
		case entity.NotificationOperatorType_NotEqual, entity.NotificationOperatorType_NotIn:
			if !matched {
				logs.CtxInfo(ctx, "matchNotificationFilter: condition[%d] NotEqual/NotIn not matched, return true", i)
				return true
			}
		}
	}

	logs.CtxInfo(ctx, "matchNotificationFilter: no condition matched, return false, to_status: %v", toStatus)
	return false
}

// parseWebhookURLs 解析逗号分隔的 webhook URL 列表
func parseWebhookURLs(urls *string) []string {
	if urls == nil || *urls == "" {
		return nil
	}
	parts := strings.Split(*urls, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func mapExptStatusToEventType(status entity.ExptStatus) string {
	switch status {
	case entity.ExptStatus_Processing:
		return "experiment.started"
	case entity.ExptStatus_Success:
		return "experiment.succeeded"
	case entity.ExptStatus_Failed:
		return "experiment.failed"
	case entity.ExptStatus_Terminated, entity.ExptStatus_SystemTerminated:
		return "experiment.terminated"
	default:
		return "experiment.unknown"
	}
}

func mapExptStatusToSummary(status entity.ExptStatus) string {
	switch status {
	case entity.ExptStatus_Processing:
		return "Experiment started"
	case entity.ExptStatus_Success:
		return "Experiment completed successfully"
	case entity.ExptStatus_Failed:
		return "Experiment failed"
	case entity.ExptStatus_Terminated, entity.ExptStatus_SystemTerminated:
		return "Experiment terminated"
	default:
		return "Experiment status changed"
	}
}

func eventTypeToStatus(eventType string) string {
	switch eventType {
	case "experiment.started":
		return "processing"
	case "experiment.succeeded":
		return "succeeded"
	case "experiment.failed":
		return "failed"
	case "experiment.terminated":
		return "terminated"
	default:
		return "unknown"
	}
}

// parseFilterValues 解析 filter condition 的 value 字段（JSON 数组格式 `["11","12"]`）
func parseFilterValues(raw string) []string {
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	return values
}
