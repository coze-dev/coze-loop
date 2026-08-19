// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"github.com/google/wire"
)

var MQProducerSet = wire.NewSet(
	NewExptEventPublisher,
	NewEvaluatorEventPublisher,
)

// StepEventPublisherSet 单独一个 set：阶段事件生产者不返回 error（构造失败降级为 no-op），
// 与 MQProducerSet 里那些 fail-fast 的生产者不是一类东西，混在一起会让人以为它也会让启动失败。
var StepEventPublisherSet = wire.NewSet(
	NewStepEventPublisher,
)
