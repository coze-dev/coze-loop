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

type TaskConsumer struct {
	handler obapp.ITaskQueueConsumer
	conf.IConfigLoader
}

func newTaskConsumer(handler obapp.ITaskQueueConsumer, loader conf.IConfigLoader) mq.IConsumerWorker {
	return &TaskConsumer{
		handler:       handler,
		IConfigLoader: loader,
	}
}

func (e *TaskConsumer) ConsumerCfg(ctx context.Context) (*mq.ConsumerConfig, error) {
	// 【1011】内场的topic，doubaogu过滤
	const key = "task_mq_consumer_config"
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
		IsEnabled:            cfg.IsEnabled,
		TagExpression:        *cfg.TagExpression,
	}
	return res, nil
}

func (e *TaskConsumer) HandleMessage(ctx context.Context, ext *mq.MessageExt) error {
	logID := logs.NewLogID()
	ctx = logs.SetLogID(ctx, logID)
	event := new(entity.RawSpan)
	if err := json.Unmarshal(ext.Body, event); err != nil {
		logs.CtxError(ctx, "Task msg json unmarshal fail, raw: %v, err: %s", conv.UnsafeBytesToString(ext.Body), err)
		return nil
	}
	logs.CtxInfo(ctx, "Span msg,log_id=%s, trace_id=%s, span_id=%s,msgID=%s", event.LogID, event.TraceID, event.SpanID, ext.MsgID)
	return e.handler.SpanTrigger(ctx, event)
}
