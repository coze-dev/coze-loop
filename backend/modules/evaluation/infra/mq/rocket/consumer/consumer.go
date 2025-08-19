// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"context"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/mq/rocket"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
)

func NewConsumerWorkers(
	cfgFactory conf.IConfigLoaderFactory,
	exptApp application.IExperimentApplication,
) ([]mq.IConsumerWorker, error) {
	loader, err := cfgFactory.NewConfigLoader(consts.EvaluationConfigFileName)
	if err != nil {
		return nil, err
	}

	return []mq.IConsumerWorker{
		newExptSchedulerEventConsumer(newExptSchedulerConsumer(exptApp), loader),
		newExptRecordEvalEventConsumer(NewExptRecordEvalConsumer(exptApp), loader),
		newExptAggrCalculateEventConsumer(NewAggrCalculateConsumer(exptApp), loader),
		newExptTurnResultFilterEventConsumer(NewExptTurnResultFilterConsumer(exptApp), loader),
	}, nil
}

func newExptSchedulerEventConsumer(handler mq.IConsumerHandler, loader conf.IConfigLoader) mq.IConsumerWorker {
	return &ExptSchedulerEventConsumer{
		IConsumerHandler: handler,
		IConfigLoader:    loader,
	}
}

type ExptTurnResultFilterEventConsumer struct {
	mq.IConsumerHandler
	conf.IConfigLoader
}

func newExptTurnResultFilterEventConsumer(handler mq.IConsumerHandler, loader conf.IConfigLoader) mq.IConsumerWorker {
	return &ExptTurnResultFilterEventConsumer{
		IConsumerHandler: handler,
		IConfigLoader:    loader,
	}
}

func (e *ExptTurnResultFilterEventConsumer) ConsumerCfg(ctx context.Context) (*mq.ConsumerConfig, error) {
	rmqCfg := &rocket.RMQConf{}
	if err := e.UnmarshalKey(ctx, rocket.ExptTurnResultFilterRMQKey, rmqCfg); err != nil {
		return nil, err
	}
	return gptr.Of(rmqCfg.ToConsumerCfg()), nil
}

type ExptSchedulerEventConsumer struct {
	mq.IConsumerHandler
	conf.IConfigLoader
}

func (e *ExptSchedulerEventConsumer) ConsumerCfg(ctx context.Context) (*mq.ConsumerConfig, error) {
	rmqCfg := &rocket.RMQConf{}
	if err := e.UnmarshalKey(ctx, rocket.ExptScheduleEventRMQKey, rmqCfg); err != nil {
		return nil, err
	}
	return gptr.Of(rmqCfg.ToConsumerCfg()), nil
}

func (e *ExptSchedulerEventConsumer) GetConsumerCfg(ctx context.Context, loader conf.IConfigLoader) (*mq.ConsumerConfig, error) {
	rmqCfg := &rocket.RMQConf{}
	if err := loader.UnmarshalKey(ctx, rocket.ExptScheduleEventRMQKey, rmqCfg); err != nil {
		return nil, err
	}
	return gptr.Of(rmqCfg.ToConsumerCfg()), nil
}

func newExptRecordEvalEventConsumer(handler mq.IConsumerHandler, loader conf.IConfigLoader) mq.IConsumerWorker {
	return &ExptRecordEvalEventConsumer{
		IConsumerHandler: handler,
		IConfigLoader:    loader,
	}
}

type ExptRecordEvalEventConsumer struct {
	mq.IConsumerHandler
	conf.IConfigLoader
}

func (e *ExptRecordEvalEventConsumer) ConsumerCfg(ctx context.Context) (*mq.ConsumerConfig, error) {
	rmqCfg := &rocket.RMQConf{}
	if err := e.UnmarshalKey(ctx, rocket.ExptRecordEvalEventRMQKey, rmqCfg); err != nil {
		return nil, err
	}
	return gptr.Of(rmqCfg.ToConsumerCfg()), nil
}

func newExptAggrCalculateEventConsumer(handler mq.IConsumerHandler, loader conf.IConfigLoader) mq.IConsumerWorker {
	return &ExptAggrCalculateEventConsumer{
		IConsumerHandler: handler,
		IConfigLoader:    loader,
	}
}

type ExptAggrCalculateEventConsumer struct {
	mq.IConsumerHandler
	conf.IConfigLoader
}

func (e *ExptAggrCalculateEventConsumer) ConsumerCfg(ctx context.Context) (*mq.ConsumerConfig, error) {
	rmqCfg := &rocket.RMQConf{}
	if err := e.UnmarshalKey(ctx, rocket.ExptAggrCalculateEventRMQKey, rmqCfg); err != nil {
		return nil, err
	}
	return gptr.Of(rmqCfg.ToConsumerCfg()), nil
}
