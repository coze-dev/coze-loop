// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/observability/application"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
)

func NewConsumerWorkers(
	loader conf.IConfigLoader,
	handler application.IAnnotationQueueConsumer,
	taskConsumer application.ITaskQueueConsumer,
) ([]mq.IConsumerWorker, error) {
	return []mq.IConsumerWorker{
		newAnnotationConsumer(handler, loader),
		newTaskConsumer(taskConsumer, loader),
	}, nil
}
