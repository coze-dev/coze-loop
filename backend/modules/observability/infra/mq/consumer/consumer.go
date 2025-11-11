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
	workers := []mq.IConsumerWorker{}
	workers = append(workers,
		NewAnnotationConsumer(handler, loader),
		NewTaskConsumer(taskConsumer, loader),
		NewCallbackConsumer(taskConsumer, loader),
		NewCorrectionConsumer(taskConsumer, loader),
		NewBackFillConsumer(taskConsumer, loader),
		NewSpanWithAnnotationConsumer(taskConsumer, loader),
	)

	return workers, nil
}
