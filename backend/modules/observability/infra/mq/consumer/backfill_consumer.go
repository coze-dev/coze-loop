// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	obapp "github.com/coze-dev/coze-loop/backend/modules/observability/application"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/conv"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type BackFillConsumer struct {
	handler obapp.ITaskQueueConsumer
	conf.IConfigLoader
}

func newBackFillConsumer(handler obapp.ITaskQueueConsumer, loader conf.IConfigLoader) mq.IConsumerWorker {
	return &BackFillConsumer{
		handler:       handler,
		IConfigLoader: loader,
	}
}

func (e *BackFillConsumer) ConsumerCfg(ctx context.Context) (*mq.ConsumerConfig, error) {
	const key = "backfill_mq_consumer_config"
	cfg := &config.MqConsumerCfg{}
	if err := e.UnmarshalKey(ctx, key, cfg); err != nil {
		return nil, err
	}
	res := &mq.ConsumerConfig{
		Addr:                 cfg.Addr,
		Topic:                cfg.Topic,
		ConsumerGroup:        cfg.ConsumerGroup,
		ConsumeTimeout:       time.Duration(cfg.Timeout) * time.Millisecond,
		ConsumeGoroutineNums: cfg.WorkerNum,
		EnablePPE:            cfg.EnablePPE,
	}
	return res, nil
}

func (e *BackFillConsumer) HandleMessage(ctx context.Context, ext *mq.MessageExt) error {
	logID := logs.NewLogID()
	ctx = logs.SetLogID(ctx, logID)
	event := new(entity.BackFillEvent)
	if err := json.Unmarshal(ext.Body, event); err != nil {
		logs.CtxError(ctx, "BackFill msg json unmarshal fail, raw: %v, err: %s", conv.UnsafeBytesToString(ext.Body), err)
		return nil
	}
	logs.CtxInfo(ctx, "BackFill msg %+v,msgID=%s", event, ext.MsgID)
	return e.handler.BackFill(ctx, event)
}
