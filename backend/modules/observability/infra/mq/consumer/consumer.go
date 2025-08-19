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
) ([]mq.IConsumerWorker, error) {
	return []mq.IConsumerWorker{
		newAnnotationConsumer(handler, loader),
	}, nil
}
