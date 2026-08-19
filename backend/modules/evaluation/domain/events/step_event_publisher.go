// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// StepEventPublisher 把评测链路阶段事件的明细投递到 MQ，供数仓消费同步到 Hive。
//
// ⚠️ **PublishStepEvent 没有 error 返回值，这是刻意的**，而且与本模块其它 publisher
// （ExptEventPublisher 全部方法 `return err`）相反。
//
// 那些是实验调度事件：消息丢了实验会卡住，fail-fast 是对的。这条是埋点：丢一条不影响任何评测的
// 正确性，但如果它能返回 error，早晚会有人在调用点写 `if err != nil { return err }`，
// 于是 MQ 抖动就变成了沙箱侧上报接口 5xx——一个观测通道把被观测的链路搞挂了。
//
// 签名里没有 error，这件事就在编译期不可能发生。失败的处理全部在实现内部完成：warn 一条带 topic
// 的日志，然后返回。
//
//go:generate mockgen -destination mocks/step_event_publisher_mock.go -package mocks . StepEventPublisher
type StepEventPublisher interface {
	PublishStepEvent(ctx context.Context, event *entity.SandboxStepEventMessage)
}
