// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

// Package retry 提供 webhook 首投失败后基于 RocketMQ 三档延迟的重试消费者。
//
// 契约（tech_design + test_cases 已锁）：
//   - Producer.Enqueue(deliveryID, attempt) 只投 delivery_id，不塞 payload/URL/secret；
//     消息体 JSON = {"delivery_id":"<uuid>"}，topic=webhook_delivery_retry，
//     tag=retry，DelayLevel 由 attempt 决定（test_case 30）。
//   - Consumer.Consume(deliveryID) 按 delivery_id 拉行：
//     status ∈ {succeeded, failed} → 幂等短路直接 ack（test_case 18）；
//     否则 re-Sign + Send + UpdateStatus，attempt_count 递增；
//     attempt_count 达 MaxAttempts=4 时置 status=failed 终态且不再入队
//     （test_case 15 三档 1min/5min/30min 递进 + 4 次耗尽）。
//   - 重试消息 body / URL 从 delivery 行 request_body / url 还原，签名 timestamp
//     取当前时间（test_case 23：每次投递独立 timestamp 参与签名，不缓存旧签名）。
package retry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/webhook/dispatcher"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/webhook/sender"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/webhook/signer"
)

// MaxAttempts 首投 + 重试合计上限。首投占 attempt=1，最多 3 次重试（attempt=2/3/4），
// attempt_count==4 后即使 Retryable 也置 failed 终态。（test_case 15）
const MaxAttempts int32 = 4

// Topic / Tag 常量与 RocketMQ 侧对齐（test_case 30）。
const (
	Topic = "webhook_delivery_retry"
	Tag   = "retry"
)

// DelayLevel 三档 RocketMQ 延迟等级（1min=5 / 5min=9 / 30min=16，test_case 15）。
// 索引 = 即将发起的重试轮次（第 1 次重试 → 1min，第 2 次 → 5min，第 3 次 → 30min）。
// 超过 3 时兜底取 30min。
func DelayLevel(nextAttempt int32) int32 {
	switch nextAttempt {
	case 1:
		return 5
	case 2:
		return 9
	default:
		return 16
	}
}

// TopicMessage retry topic 消息体（test_case 30 契约：body 只带 delivery_id）。
type TopicMessage struct {
	DeliveryID string `json:"delivery_id"`
}

// MQClient 抽象 RocketMQ 延迟消息投递能力；生产走 cloudwego rocketmq producer，
// 单测走桩。DelayLevel / Body 由 Producer 组装完毕后一次性下发。
type MQClient interface {
	SendDelay(ctx context.Context, topic, tag string, body []byte, delayLevel int32) error
}

// Producer 组装 retry 消息 + 计算 DelayLevel + 委托 MQClient 下发。
// 实现 dispatcher.RetryEnqueuer 接口，可直接注入 dispatcher。
type Producer struct {
	MQ MQClient
}

// Enqueue 把一次首投 / 中途失败结果转成延迟消息。nextAttempt = 即将发起的重试轮次
// （首投失败 → 传 1；attempt=1 重试仍失败 → 传 2；attempt=2 重试仍失败 → 传 3）。
func (p *Producer) Enqueue(ctx context.Context, deliveryID string, nextAttempt int32) error {
	if p == nil || p.MQ == nil {
		return nil
	}
	body, err := json.Marshal(TopicMessage{DeliveryID: deliveryID})
	if err != nil {
		return fmt.Errorf("marshal retry msg: %w", err)
	}
	return p.MQ.SendDelay(ctx, Topic, Tag, body, DelayLevel(nextAttempt))
}

// Consumer 拉取 retry topic 消息 → 复用 signer + sender 重投 → 状态机推进。
// Repo / Client / Secrets / Retry / Now 与 dispatcher 一致，便于共享注入。
type Consumer struct {
	Repo    repo.IWebhookDeliveryRepo
	Client  *http.Client
	Secrets dispatcher.SecretProvider
	Retry   dispatcher.RetryEnqueuer
	Now     func() time.Time
}

// Consume 处理一次 retry 消息。ctx 已由上层 MQ 消费框架带过来（含 traceID 等）。
// 返回 error 时由 MQ 侧决定 nack 重投；正常路径（含短路、终态、成功、继续重试）
// 均返 nil 让消费框架 ack。
func (c *Consumer) Consume(ctx context.Context, deliveryID string) error {
	delivery, err := c.Repo.GetByDeliveryID(ctx, deliveryID)
	if err != nil {
		return fmt.Errorf("load delivery %s: %w", deliveryID, err)
	}
	if delivery == nil {
		return nil
	}
	// test_case 18：succeeded / failed 终态直接短路，不再发 HTTP，attempt / last_sent_at 不动。
	if delivery.Status == entity.DeliveryStatus_Succeeded || delivery.Status == entity.DeliveryStatus_Failed {
		return nil
	}
	secret := ""
	if c.Secrets != nil {
		if s, sErr := c.Secrets.GetSecret(ctx, delivery.WorkspaceID); sErr == nil {
			secret = s
		}
	}
	now := c.timeNow()
	ts := strconv.FormatInt(now.Unix(), 10)
	body := []byte(delivery.RequestBody)
	sig := signer.Sign(secret, ts, body)
	result := sender.Send(ctx, c.Client, sender.Request{
		URL:        delivery.URL,
		Body:       body,
		DeliveryID: delivery.DeliveryID,
		Event:      dispatcher.EventName(delivery.Event),
		Timestamp:  ts,
		Signature:  sig,
	})
	attempt := delivery.AttemptCount + 1
	sent := c.timeNow()
	upd := &repo.UpdateWebhookDeliveryStatusRequest{
		DeliveryID:       delivery.DeliveryID,
		AttemptCount:     attempt,
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
		if attempt >= MaxAttempts {
			// test_case 15：4 次耗尽 → failed 终态，不再入 retry MQ，next_retry_at 保留旧值不递增。
			upd.Status = entity.DeliveryStatus_Failed
		} else {
			upd.Status = entity.DeliveryStatus_Retrying
			next := sent.Add(nextBackoff(attempt))
			upd.NextRetryAt = &next
			if c.Retry != nil {
				_ = c.Retry.Enqueue(ctx, delivery.DeliveryID, attempt)
			}
		}
	default:
		upd.Status = entity.DeliveryStatus_Failed
	}
	return c.Repo.UpdateStatus(ctx, upd)
}

// nextBackoff 与 DelayLevel 保持一致的时长映射，用于回写 next_retry_at
// 便于 sweeper 兜底扫描（test_case 15 / 14 中的 next_retry_at 递进契约）。
func nextBackoff(nextAttempt int32) time.Duration {
	switch nextAttempt {
	case 1:
		return time.Minute
	case 2:
		return 5 * time.Minute
	default:
		return 30 * time.Minute
	}
}

func (c *Consumer) timeNow() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}
