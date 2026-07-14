// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"context"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	componentwebhook "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/webhook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/mq/rocket"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func NewConsumerWorkers(
	loader conf.IConfigLoader,
	exptApp application.IExperimentApplication,
) ([]mq.IConsumerWorker, error) {
	workers := []mq.IConsumerWorker{
		NewExptSchedulerEventConsumer(NewExptSchedulerConsumer(exptApp), loader),
		NewExptRecordEvalEventConsumer(NewExptRecordEvalConsumer(exptApp), loader),
		NewExptAggrCalculateEventConsumer(NewAggrCalculateConsumer(exptApp), loader),
		NewExptTurnResultFilterEventConsumer(NewExptTurnResultFilterConsumer(exptApp), loader),
		NewExptExportEventConsumer(NewExptExportConsumer(exptApp, exptApp), loader),
		NewExptLifecycleEventConsumer(NewExptLifecycleConsumer(exptApp), loader),
	}
	workers = append(workers, buildWebhookDeliveryConsumer(loader, exptApp))
	return workers, nil
}

// buildWebhookDeliveryConsumer selects between a real WebhookDeliveryConsumer
// worker and NewNilWebhookDeliveryEventConsumer. Kept thin so
// webhookDeliveryConsumerFrom holds the actual decision logic and can be
// covered by unit tests without a full application.IExperimentApplication fake.
func buildWebhookDeliveryConsumer(loader conf.IConfigLoader, exptApp application.IExperimentApplication) mq.IConsumerWorker {
	sender, deliveryRepo, publisher, configer, exptRepo, resultSvc, aggrResultSvc := exptApp.WebhookDeliveryComponents()
	return webhookDeliveryConsumerFrom(loader, sender, deliveryRepo, publisher, configer, exptRepo, resultSvc, aggrResultSvc)
}

// webhookDeliveryConsumerFrom decides whether to wire a real webhook_delivery
// consumer or a disabled stub. The stub keeps registry.StartAll happy (its
// ConsumerCfg returns IsEnabled=false so no subscribe is attempted) which
// unblocks pod readiness both during rollout (some deps nil, mirrors the E-I-03
// provideNilWebhookDispatcher rollback on commercial) and after ops flips
// WebhookGlobalConf.DisableConsumer for a topic-provisioning outage.
func webhookDeliveryConsumerFrom(
	loader conf.IConfigLoader,
	sender componentwebhook.IWebhookSender,
	deliveryRepo repo.IWebhookDeliveryRepo,
	publisher events.WebhookDeliveryEventPublisher,
	configer component.IWebhookConfiger,
	exptRepo repo.IExperimentRepo,
	resultSvc service.ExptResultService,
	aggrResultSvc service.ExptAggrResultService,
) mq.IConsumerWorker {
	if sender == nil || deliveryRepo == nil || publisher == nil || configer == nil {
		logs.CtxWarn(context.Background(),
			"webhook_delivery_consumer_dep_missing_skip_start sender=%t deliveryRepo=%t publisher=%t configer=%t",
			sender != nil, deliveryRepo != nil, publisher != nil, configer != nil)
		return NewNilWebhookDeliveryEventConsumer()
	}
	if g := configer.GetWebhookConf(context.Background()); g != nil && g.DisableConsumer {
		logs.CtxWarn(context.Background(),
			"webhook_delivery_consumer_disabled_by_conf skip_start=true")
		return NewNilWebhookDeliveryEventConsumer()
	}
	handler := NewWebhookDeliveryConsumer(sender, deliveryRepo, publisher, configer, exptRepo, resultSvc, aggrResultSvc)
	return NewWebhookDeliveryEventConsumer(handler, loader)
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
