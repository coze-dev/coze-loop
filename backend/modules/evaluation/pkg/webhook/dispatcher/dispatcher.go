// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

// Package dispatcher 组合 matcher + repo + signer + sender + retry hook，
// 提供 status_change → notifications 匹配 → build Request → signer.Sign →
// sender.Send → 写 webhook_delivery 状态机 → retryable 入 retry MQ 的一体化入口。
//
// 契约（tech_design + test_cases 已锁）：
//   - operator=contains/not_contains，triggers 全量枚举匹配（test_cases 9/10）；
//   - 每个匹配规则的 webhook action fan-out 独立 delivery（test_case 10），
//     feishu 通道由上层独立处理（test_case 24），不入本 dispatcher；
//   - Create pending → sender.Send → 按 Outcome 推进：
//     Succeeded → status=succeeded（test_case 13）；
//     Retryable → status=retrying + next_retry_at≈now+1min + Retry.Enqueue（test_cases 14/15/17）；
//     NonRetryable → status=failed（test_case 16）；
//   - 首投 attempt_count=1；后续重试 attempt_count 递增交由 retry consumer；
//   - RetryEnqueuer 保持 hook 接口，实际 RocketMQ Producer 由后一轮接入（test_case 30）。
package dispatcher

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/webhook/sender"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/webhook/signer"
)

// FirstRetryBackoff 首次 retryable 结果的 next_retry_at 补偿。
// 与 tech_design 三档延迟(1min/5min/30min)首档对齐，attempt 递增由 retry consumer 承接。
const FirstRetryBackoff = time.Minute

const (
	OperatorContains    = "contains"
	OperatorNotContains = "not_contains"
)

// SecretProvider 提供 workspace 维度 webhook 签名密钥（商业侧 workspace_sk / OSS 静态 secret）。
// 返回空字符串 → signer 走降级路径（test_case 22）。
type SecretProvider interface {
	GetSecret(ctx context.Context, workspaceID int64) (string, error)
}

// RetryEnqueuer 首投/中途 Retryable 转 RocketMQ 延迟消息（消息体仅 delivery_id，test_case 30）。
// 保持 hook 接口，实际 Producer impl 由后一轮接入。
type RetryEnqueuer interface {
	Enqueue(ctx context.Context, deliveryID string, attempt int32) error
}

// Dispatcher 组合 signer + sender + repo + retry hook。
// Now / NewID 保留注入位便于单测桩（生产传 nil 走默认）。
type Dispatcher struct {
	Repo    repo.IWebhookDeliveryRepo
	Client  *http.Client
	Secrets SecretProvider
	Retry   RetryEnqueuer
	Now     func() time.Time
	NewID   func() string
}

// Match 判断某规则是否命中 event。
//   - contains：triggers 数组包含 event → 命中；
//   - not_contains：triggers 数组不包含 event → 命中（test_case 9 排除语义）；
//   - 未识别 operator → 与 contains 等价（保守放行，与 tech_design 白名单校验交由 API 层）。
func Match(rule *entity.NotificationRule, event entity.NotificationTrigger) bool {
	found := false
	for _, t := range rule.Triggers {
		if t == event {
			found = true
			break
		}
	}
	if rule.Operator == OperatorNotContains {
		return !found
	}
	return found
}

// Dispatch fan-out 匹配规则的 webhook actions。
// body 必须已 canonicalize（与签名内容一致，test_case 29）。
func (d *Dispatcher) Dispatch(ctx context.Context, workspaceID, experimentID int64, event entity.NotificationTrigger, body []byte, rules []entity.NotificationRule) error {
	secret := ""
	if d.Secrets != nil {
		if s, err := d.Secrets.GetSecret(ctx, workspaceID); err == nil {
			secret = s
		}
	}
	now := d.timeNow()
	ts := strconv.FormatInt(now.Unix(), 10)
	for i := range rules {
		if !Match(&rules[i], event) {
			continue
		}
		for _, act := range rules[i].Actions {
			if act.Type != entity.NotificationActionType_Webhook {
				continue
			}
			d.dispatchOne(ctx, workspaceID, experimentID, event, act.URL, body, secret, ts, now)
		}
	}
	return nil
}

func (d *Dispatcher) dispatchOne(ctx context.Context, workspaceID, experimentID int64, event entity.NotificationTrigger, url string, body []byte, secret, ts string, first time.Time) {
	deliveryID := d.newID()
	sig := signer.Sign(secret, ts, body)
	delivery := &entity.WebhookDelivery{
		DeliveryID:   deliveryID,
		WorkspaceID:  workspaceID,
		ExperimentID: experimentID,
		Event:        event,
		URL:          url,
		Status:       entity.DeliveryStatus_Pending,
		RequestBody:  string(body),
		FirstSentAt:  &first,
	}
	_ = d.Repo.Create(ctx, delivery)
	result := sender.Send(ctx, d.Client, sender.Request{
		URL:        url,
		Body:       body,
		DeliveryID: deliveryID,
		Event:      EventName(event),
		Timestamp:  ts,
		Signature:  sig,
	})
	sent := d.timeNow()
	upd := &repo.UpdateWebhookDeliveryStatusRequest{
		DeliveryID:       deliveryID,
		AttemptCount:     1,
		LastResponseCode: int32(result.StatusCode),
		LastSentAt:       &sent,
	}
	if result.Err != nil {
		upd.LastError = result.Err.Error()
	}
	switch result.Outcome {
	case sender.OutcomeSucceeded:
		upd.Status = entity.DeliveryStatus_Succeeded
	case sender.OutcomeRetryable:
		upd.Status = entity.DeliveryStatus_Retrying
		next := sent.Add(FirstRetryBackoff)
		upd.NextRetryAt = &next
		if d.Retry != nil {
			_ = d.Retry.Enqueue(ctx, deliveryID, 1)
		}
	default:
		upd.Status = entity.DeliveryStatus_Failed
	}
	_ = d.Repo.UpdateStatus(ctx, upd)
}

func (d *Dispatcher) timeNow() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

func (d *Dispatcher) newID() string {
	if d.NewID != nil {
		return d.NewID()
	}
	return uuid.NewString()
}

// EventName 把 trigger 枚举映射为 header X-Fornax-Event / body event 字段用的字符串
// （started/succeeded/failed/terminated；Terminated 与 SystemTerminated 合并映射，test_case 7）。
func EventName(e entity.NotificationTrigger) string {
	switch e {
	case entity.NotificationTrigger_Started:
		return "started"
	case entity.NotificationTrigger_Succeeded:
		return "succeeded"
	case entity.NotificationTrigger_Failed:
		return "failed"
	case entity.NotificationTrigger_Terminated:
		return "terminated"
	default:
		return ""
	}
}
