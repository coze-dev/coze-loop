// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"context"
	"strings"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/mq/rocket"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
)

func NewConsumerWorkers(
	loader conf.IConfigLoader,
	exptApp application.IExperimentApplication,
	publisher events.ExptEventPublisher,
	settlement ...service.CentralItemSettlementDeps,
) ([]mq.IConsumerWorker, error) {
	webhookHandler := NewWebhookRetryConsumer(publisher, service.NewNoopWebhookSecretProvider())
	// 中心调度事件在原有 eval 链路后做预占结算；旧事件 CentralReservationID 为空，包装器直接透传。
	recordEval := service.NewCentralItemEvalService(exptApp, settlement...)
	workers := []mq.IConsumerWorker{
		NewExptSchedulerEventConsumer(NewExptSchedulerConsumer(exptApp), loader),
		NewExptRecordEvalEventConsumer(NewExptRecordEvalConsumer(recordEval), loader),
		NewExptAggrCalculateEventConsumer(NewAggrCalculateConsumer(exptApp), loader),
		NewExptTurnResultFilterEventConsumer(NewExptTurnResultFilterConsumer(exptApp), loader),
		NewExptExportEventConsumer(NewExptExportConsumer(exptApp, exptApp), loader),
		NewExptLifecycleEventConsumer(NewExptLifecycleConsumer(exptApp, webhookHandler), loader),
	}
	if len(settlement) > 0 && settlement[0].Env != nil {
		env, err := settlement[0].Env.CurrentEnvID(context.Background())
		if err != nil {
			return nil, err
		}
		// env 专属 group：同一 group 跨泳道会造成消息被任一泳道抢走；消息内容仍按 CentralEnv 二次过滤。
		suffix := strings.NewReplacer(":", "-", "{", "_", "}", "_").Replace(env)
		workers = append(workers,
			NewCentralExptRecordEvalEventConsumer(NewCentralExptRecordEvalConsumer(recordEval, settlement[0].Env), loader, suffix))
	}
	return workers, nil
}

func NewExptSchedulerEventConsumer(handler mq.IConsumerHandler, loader conf.IConfigLoader) mq.IConsumerWorker {
	return &ExptSchedulerEventConsumer{
		IConsumerHandler: handler,
		IConfigLoader:    loader,
	}
}

type ExptTurnResultFilterEventConsumer struct {
	mq.IConsumerHandler
	conf.IConfigLoader
}

func NewExptTurnResultFilterEventConsumer(handler mq.IConsumerHandler, loader conf.IConfigLoader) mq.IConsumerWorker {
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

func NewExptRecordEvalEventConsumer(handler mq.IConsumerHandler, loader conf.IConfigLoader) mq.IConsumerWorker {
	return &ExptRecordEvalEventConsumer{
		IConsumerHandler: handler,
		IConfigLoader:    loader,
	}
}

// NewCentralExptRecordEvalEventConsumer 使用 env 专属 consumer group，避免多泳道争抢同一 group。
func NewCentralExptRecordEvalEventConsumer(handler mq.IConsumerHandler, loader conf.IConfigLoader, envSuffix string) mq.IConsumerWorker {
	return &ExptRecordEvalEventConsumer{
		IConsumerHandler: handler,
		IConfigLoader:    loader,
		groupSuffix:      envSuffix,
	}
}

type ExptRecordEvalEventConsumer struct {
	mq.IConsumerHandler
	conf.IConfigLoader
	groupSuffix string
}

func (e *ExptRecordEvalEventConsumer) ConsumerCfg(ctx context.Context) (*mq.ConsumerConfig, error) {
	rmqCfg := &rocket.RMQConf{}
	if err := e.UnmarshalKey(ctx, rocket.ExptRecordEvalEventRMQKey, rmqCfg); err != nil {
		return nil, err
	}
	cfg := rmqCfg.ToConsumerCfg()
	if e.groupSuffix != "" {
		cfg.ConsumerGroup = cfg.ConsumerGroup + "-central-" + e.groupSuffix
	}
	return gptr.Of(cfg), nil
}

func NewExptAggrCalculateEventConsumer(handler mq.IConsumerHandler, loader conf.IConfigLoader) mq.IConsumerWorker {
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

func NewExptExportEventConsumer(handler mq.IConsumerHandler, loader conf.IConfigLoader) mq.IConsumerWorker {
	return &ExptExportEventConsumer{
		IConsumerHandler: handler,
		IConfigLoader:    loader,
	}
}

type ExptExportEventConsumer struct {
	mq.IConsumerHandler
	conf.IConfigLoader
}

func (e *ExptExportEventConsumer) ConsumerCfg(ctx context.Context) (*mq.ConsumerConfig, error) {
	rmqCfg := &rocket.RMQConf{}
	if err := e.UnmarshalKey(ctx, rocket.ExptExportCSVEventRMQKey, rmqCfg); err != nil {
		return nil, err
	}
	return gptr.Of(rmqCfg.ToConsumerCfg()), nil
}

func NewExptLifecycleEventConsumer(handler mq.IConsumerHandler, loader conf.IConfigLoader) mq.IConsumerWorker {
	return &ExptLifecycleEventConsumer{
		IConsumerHandler: handler,
		IConfigLoader:    loader,
	}
}

type ExptLifecycleEventConsumer struct {
	mq.IConsumerHandler
	conf.IConfigLoader
}

func (e *ExptLifecycleEventConsumer) ConsumerCfg(ctx context.Context) (*mq.ConsumerConfig, error) {
	rmqCfg := &rocket.RMQConf{}
	if err := e.UnmarshalKey(ctx, rocket.ExptLifecycleEventRMQKey, rmqCfg); err != nil {
		return nil, err
	}
	return gptr.Of(rmqCfg.ToConsumerCfg()), nil
}
