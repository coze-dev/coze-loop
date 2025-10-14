// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"context"

	json "github.com/bytedance/sonic"
	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/data/application"
	dataset_conf "github.com/coze-dev/coze-loop/backend/modules/data/domain/component/conf"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/conv"
)

type DatasetJobConsumer struct {
	handler application.IJobRunMsgHandler
	conf.IConfigLoader
}

func newDatasetJobConsumer(handler application.IJobRunMsgHandler, loader conf.IConfigLoader) mq.IConsumerWorker {
	return &DatasetJobConsumer{handler: handler, IConfigLoader: loader}
}

func (e *DatasetJobConsumer) ConsumerCfg(ctx context.Context) (*mq.ConsumerConfig, error) {
	const key = "consumer_configs"

	cfg := &dataset_conf.ConsumerConfig{}
	if err := e.UnmarshalKey(ctx, key, cfg); err != nil {
		return nil, err
	}

	if cfg.ConsumeGoroutineNums <= 0 {
		cfg.ConsumeGoroutineNums = 10
	}
	res := &mq.ConsumerConfig{
		Addr:                 cfg.Addr,
		Topic:                cfg.Topic,
		ConsumerGroup:        cfg.ConsumerGroup,
		Orderly:              cfg.Orderly,
		ConsumeTimeout:       cfg.ConsumeTimeout,
		TagExpression:        cfg.TagExpression,
		ConsumeGoroutineNums: cfg.ConsumeGoroutineNums,
	}
	return res, nil
}

func (e *DatasetJobConsumer) HandleMessage(ctx context.Context, ext *mq.MessageExt) error {
	ese := new(entity.JobRunMessage)
	if err := json.Unmarshal(ext.Body, ese); err != nil {
		return errorx.Wrapf(err, "DatasetJobConsumer json unmarshal fail, raw: %v", conv.UnsafeBytesToString(ext.Body))
	}
	if ese.Type == entity.DatasetSnapshotJob {
		return e.handler.RunSnapshotItemJob(ctx, ese)
	}
	if ese.Type == entity.DatasetIOJob {
		return e.handler.RunIOJob(ctx, ese)
	}
	return nil
}
