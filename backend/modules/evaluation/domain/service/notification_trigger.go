// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// INotificationTriggerService 通知触发服务，负责在实验生命周期事件发生时匹配规则并分发通知
type INotificationTriggerService interface {
	TriggerNotification(ctx context.Context, event *entity.ExptLifecycleEvent, notificationConf *entity.NotificationConf) error
}
