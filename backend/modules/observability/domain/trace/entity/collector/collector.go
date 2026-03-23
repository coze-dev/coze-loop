// Original Files: open-telemetry/opentelemetry-collector
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// This file may have been modified by ByteDance Ltd.

package collector

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/component"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/exporter"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/processor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/receiver"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/service"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type Factories struct {
	Receivers  map[component.Type]receiver.Factory
	Processors map[component.Type]processor.Factory
	Exporters  map[component.Type]exporter.Factory
}

type Config struct {
	Receivers  map[component.ID]component.Config
	Processors map[component.ID]component.Config
	Exporters  map[component.ID]component.Config
	Tenants    map[string]*service.Config
}

func (cfg *Config) Validate() error {
	if len(cfg.Receivers) == 0 {
		return fmt.Errorf("no receivers configured")
	}
	for recvID, recvCfg := range cfg.Receivers {
		if err := component.ValidateConfig(recvCfg); err != nil {
			return fmt.Errorf("receivers::%s validation fails, %v", recvID, err)
		}
	}
	// Currently, there is no default processor enabled.
	// The configuration must specify at least one exporter to be valid.
	if len(cfg.Processors) == 0 {
		return fmt.Errorf("no processors configured")
	}
	// Validate the processor configuration.
	for procID, procCfg := range cfg.Processors {
		if err := component.ValidateConfig(procCfg); err != nil {
			return fmt.Errorf("processor::%s validation fails, %v", procID, err)
		}
	}
	// Currently, there is no default exporter enabled.
	// The configuration must specify at least one exporter to be valid.
	if len(cfg.Exporters) == 0 {
		return fmt.Errorf("no exporters configured")
	}
	// Validate the exporter configuration.
	for expID, expCfg := range cfg.Exporters {
		if err := component.ValidateConfig(expCfg); err != nil {
			return fmt.Errorf("exporter::%s validation fails, %v", expID, err)
		}
	}
	// Check that all tenant pipelines reference only configured components.
	for tenantName, tenantPipelineConfig := range cfg.Tenants {
		if len(tenantPipelineConfig.Receivers) == 0 {
			return fmt.Errorf("tenants::%s: no receiver is configured", tenantName)
		}
		if len(tenantPipelineConfig.Processors) == 0 {
			return fmt.Errorf("tenants::%s: no processor is configured", tenantName)
		}
		if len(tenantPipelineConfig.Exporters) == 0 {
			return fmt.Errorf("tenants::%s: no exporter is configured", tenantName)
		}
		// Validate pipeline receiver name references.
		for _, ref := range tenantPipelineConfig.Receivers {
			// Check that the name referenced in the pipeline's receivers exists in the top-level receivers.
			if _, ok := cfg.Receivers[ref]; ok {
				continue
			}
			return fmt.Errorf("tenants::%s: references receiver %q which is not configured", tenantName, ref)
		}

		// Validate pipeline processor name references.
		for _, ref := range tenantPipelineConfig.Processors {
			// Check that the name referenced in the pipeline's processors exists in the top-level processors.
			if _, ok := cfg.Processors[ref]; ok {
				continue
			}
			return fmt.Errorf("tenants::%s: references processor %q which is not configured", tenantName, ref)
		}

		// Validate pipeline exporter name references.
		for _, ref := range tenantPipelineConfig.Exporters {
			// Check that the name referenced in the pipeline's Exporters exists in the top-level Exporters.
			if _, ok := cfg.Exporters[ref]; ok {
				continue
			}
			return fmt.Errorf("tenants::%s: references exporter %q which is not configured", tenantName, ref)
		}
	}
	return nil
}

type Settings struct {
	Factories      func() (Factories, error)
	ConfigProvider ConfigProvider
}

type Collector struct {
	set            Settings
	tenantService  map[string]*service.Service
	signalsChannel chan os.Signal
	readyChannel   chan struct{}
}

func New(set Settings) (*Collector, error) {
	if set.ConfigProvider == nil {
		return nil, fmt.Errorf("invalid setting: nil config provider")
	}
	return &Collector{
		set:            set,
		tenantService:  make(map[string]*service.Service),
		signalsChannel: make(chan os.Signal, 3),
		readyChannel:   make(chan struct{}),
	}, nil
}

func (col *Collector) WaitForReady() {
	<-col.readyChannel
}

// 通常在异步线程中进行, 主线程需要等待初始化完成
func (col *Collector) Run(ctx context.Context, hook func() error) error {
	if err := col.setupConfigurationComponentsWithHook(ctx, hook); err != nil {
		return err
	}
	signal.Notify(col.signalsChannel, os.Interrupt, syscall.SIGTERM)
	col.readyChannel <- struct{}{}
	select {
	case s := <-col.signalsChannel:
		logs.CtxInfo(ctx, "Received signal from OS: %s", s.String())
	case <-ctx.Done():
		return col.shutdown(ctx)
	}
	return col.shutdown(ctx)
}

// 同步阻塞执行
func (col *Collector) RunInOne(ctx context.Context, hook func() error) error {
	if err := col.setupConfigurationComponentsWithHook(ctx, hook); err != nil {
		return err
	}
	signal.Notify(col.signalsChannel, os.Interrupt, syscall.SIGTERM)
	select {
	case s := <-col.signalsChannel:
		logs.CtxInfo(ctx, "Received signal from OS: %s", s.String())
	case <-ctx.Done():
		return col.shutdown(ctx)
	}
	return col.shutdown(ctx)
}

func (col *Collector) setupConfigurationComponentsWithHook(ctx context.Context, hook func() error) error {
	factories, err := col.set.Factories()
	if err != nil {
		return err
	}
	cfg, err := col.set.ConfigProvider.Get(ctx, factories)
	if err != nil {
		return err
	}
	if err = cfg.Validate(); err != nil {
		return err
	}
	for tenantName, tenantCfg := range cfg.Tenants {
		col.tenantService[tenantName], err = service.NewService(ctx, service.Settings{
			ReceiverBuilder:  receiver.NewBuilder(cfg.Receivers, factories.Receivers),
			ProcessorBuilder: processor.NewBuilder(cfg.Processors, factories.Processors),
			ExporterBuilder:  exporter.NewBuilder(cfg.Exporters, factories.Exporters),
			PipelineConfig:   tenantCfg,
		})
		if err != nil {
			return err
		}
		if err = col.tenantService[tenantName].Start(ctx); err != nil {
			shutdownErr := col.shutdown(ctx)
			if shutdownErr != nil {
				fmt.Printf("shutdown failed, %v\n", shutdownErr)
			}
			return fmt.Errorf("failed to start tenant %q, %v", tenantName, err)
		}
	}

	if err = hook(); err != nil {
		fmt.Printf("hook failed, %v\n", err)
	}
	return nil
}

func (col *Collector) shutdown(ctx context.Context) error {
	var errs []error
	for tenantName, tenantService := range col.tenantService {
		if err := tenantService.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown tenant %q, %v", tenantName, err))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
