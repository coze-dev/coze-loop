// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package retry

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// NoopEnqueuer no-op RetryEnqueuer,用于 RocketMQ Producer 未 wire 的 OSS/本地部署。
// Enqueue 只打 warn log 不真投消息;dispatcher 侧首投失败 → status=retrying 但
// 不产生 retry 消息,后续 iter_22 换真实 Producer 后无缝切换。
type NoopEnqueuer struct{}

// NewNoopEnqueuer 供 wire DI 使用;实现 dispatcher.RetryEnqueuer 接口。
func NewNoopEnqueuer() *NoopEnqueuer { return &NoopEnqueuer{} }

// Enqueue 记录 warn log 后返 nil,不真投 RocketMQ。
func (*NoopEnqueuer) Enqueue(ctx context.Context, deliveryID string, attempt int32) error {
	logs.CtxWarn(ctx, "webhook retry enqueuer is noop, skip: delivery_id=%s attempt=%d", deliveryID, attempt)
	return nil
}
