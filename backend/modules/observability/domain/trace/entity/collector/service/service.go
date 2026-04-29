// Original Files: open-telemetry/opentelemetry-collector
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// This file may have been modified by ByteDance Ltd.

package service

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/coze-dev/coze-loop/backend/infra/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/component"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/exporter"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/processor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/receiver"
)

type State int

const (
	StateStarting State = iota
	StateRunning
	StateClosing
	StateClosed
)

func (s State) String() string {
	switch s {
	case StateStarting:
		return "Starting"
	case StateRunning:
		return "Running"
	case StateClosing:
		return "Closing"
	case StateClosed:
		return "Closed"
	}
	return "UNKNOWN"
}

type Config struct {
	Receivers  []component.ID `mapstructure:"receivers"`
	Processors []component.ID `mapstructure:"processors"`
	Exporters  []component.ID `mapstructure:"exporters"`
}

type Settings struct {
	ReceiverBuilder  *receiver.Builder
	ProcessorBuilder *processor.Builder
	ExporterBuilder  *exporter.Builder
	PipelineConfig   *Config
	ConsumeMetric    metrics.Metric
}

type Service struct {
	g     *Graph
	state *atomic.Int32
}

func (s *Service) Start(ctx context.Context) error {
	if s.state.Load() == int32(StateRunning) {
		return nil
	}
	s.state.Store(int32(StateStarting))
	if err := s.g.StartAll(ctx); err != nil {
		return fmt.Errorf("cannot start pipelines, %v", err)
	}
	s.state.Store(int32(StateRunning))
	return nil
}

func (s *Service) Shutdown(ctx context.Context) error {
	if s.state.Load() == int32(StateClosed) ||
		s.state.Load() == int32(StateClosing) {
		return nil
	}
	s.state.Store(int32(StateClosing))
	if err := s.g.ShutdownAll(ctx); err != nil {
		return fmt.Errorf("failed to shutdown pipelines, %v", err)
	}
	s.state.Store(int32(StateClosed))
	return nil
}

func (s *Service) State() State {
	return State(s.state.Load())
}

func NewService(ctx context.Context, set Settings) (*Service, error) {
	svc := &Service{
		state: &atomic.Int32{},
	}
	g, err := BuildGraph(ctx, set)
	if err != nil {
		return nil, err
	}
	svc.g = g
	return svc, nil
}
