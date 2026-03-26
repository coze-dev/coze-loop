// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/exporter"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/processor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/receiver"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
)

//go:generate mockgen -destination=mocks/ingestion.go -package=mocks . IngestionService
type IngestionService interface {
	RunAsync(ctx context.Context)
	RunSync(ctx context.Context) error
}

type IngestionCollectorFactory interface {
	GetCollectorFactory() (collector.Factories, error)
}

type IngestionCollectorFactoryImpl struct {
	receiverFactories  []receiver.Factory
	processorFactories []processor.Factory
	exporterFactories  []exporter.Factory
}

func (i *IngestionCollectorFactoryImpl) GetCollectorFactory() (collector.Factories, error) {
	var err error
	factories := collector.Factories{}
	factories.Receivers, err = receiver.MakeFactoryMap(i.receiverFactories...)
	if err != nil {
		return collector.Factories{}, err
	}
	factories.Exporters, err = exporter.MakeFactoryMap(i.exporterFactories...)
	if err != nil {
		return collector.Factories{}, err
	}
	factories.Processors, err = processor.MakeFactoryMap(i.processorFactories...)
	if err != nil {
		return collector.Factories{}, err
	}
	return factories, nil
}

func NewIngestionCollectorFactory(
	receiverFactories []receiver.Factory,
	processorFactories []processor.Factory,
	exporterFactories []exporter.Factory,
) IngestionCollectorFactory {
	return &IngestionCollectorFactoryImpl{
		receiverFactories:  receiverFactories,
		processorFactories: processorFactories,
		exporterFactories:  exporterFactories,
	}
}

type IngestionServiceImpl struct {
	c *collector.Collector
}

func (i *IngestionServiceImpl) RunSync(ctx context.Context) error {
	return i.c.RunInOne(ctx, func() error {
		return nil
	})
}

func (i *IngestionServiceImpl) RunAsync(ctx context.Context) {
	go func() {
		err := i.c.Run(ctx, func() error {
			return nil
		})
		if err != nil {
			panic(err)
		}
	}()
	i.c.WaitForReady()
}

func NewIngestionServiceImpl(
	traceConfig conf.IConfigLoader,
	collectorFactory IngestionCollectorFactory,
) (IngestionService, error) {
	c, err := collector.New(collector.Settings{
		Factories:      collectorFactory.GetCollectorFactory,
		ConfigProvider: collector.NewConfigProvider(traceConfig),
	})
	if err != nil {
		return nil, err
	}
	return &IngestionServiceImpl{
		c: c,
	}, nil
}
