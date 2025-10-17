// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"context"
	"os"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/observability/application"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/slices"
)

const (
	TceCluster = "TCE_CLUSTER"
)

func NewConsumerWorkers(
	loader conf.IConfigLoader,
	handler application.IAnnotationQueueConsumer,
	taskConsumer application.ITaskQueueConsumer,
) ([]mq.IConsumerWorker, error) {
	workers := []mq.IConsumerWorker{}
	workers = append(workers,
		newAnnotationConsumer(handler, loader),
	)
	const key = "consumer_listening"
	cfg := &config.ConsumerListening{}
	if err := loader.UnmarshalKey(context.Background(), key, cfg); err != nil {
		return nil, err
	}
	if cfg.IsEnabled && slices.Contains(cfg.Clusters, os.Getenv(TceCluster)) {
		workers = append(workers,
			newTaskConsumer(taskConsumer, loader),
			newCallbackConsumer(taskConsumer, loader),
			newCorrectionConsumer(taskConsumer, loader),
			newBackFillConsumer(taskConsumer, loader),
		)
	}
	return workers, nil
}
