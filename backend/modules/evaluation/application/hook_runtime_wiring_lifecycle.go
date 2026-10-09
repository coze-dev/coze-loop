// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	infraHook "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/mq/rocket"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/mq/rocket/producer"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
)

const HookWakeRMQConfigKey = "expt_lifecycle_hook_wake_rmq"

var ErrHookWakeUnavailable = errors.New("hook wake broker binding unavailable")

type HookRuntimeWake struct {
	loader      conf.IConfigLoader
	factory     mq.IFactory
	config      *infraHook.RuntimeConfigProvider
	scope       string
	secrets     hook.SigningSecretProvider
	environment string
}

func NewHookRuntimeWake(loader conf.IConfigLoader, factory mq.IFactory, config *infraHook.RuntimeConfigProvider, scope string, secrets hook.SigningSecretProvider) *HookRuntimeWake {
	return &HookRuntimeWake{loader: loader, factory: factory, config: config, scope: scope, secrets: secrets}
}

// A nil config without an error means validated, intentional polling-only mode.
func (w *HookRuntimeWake) brokerConfig(ctx context.Context) (*rocket.RMQConf, error) {
	if w == nil || ctx == nil || ctx.Err() != nil {
		return nil, ErrHookWakeUnavailable
	}
	c, err := w.config.GetRuntimeConfig(ctx)
	if err != nil || !c.WorkerEnabled {
		return nil, ErrHookWakeUnavailable
	}
	if !c.MQWakeEnabled {
		return nil, nil
	}
	if hookWorkerNil(w.loader) || hookWorkerNil(w.factory) {
		return nil, ErrHookWakeUnavailable
	}
	cfg := &rocket.RMQConf{}
	if w.loader.UnmarshalKey(ctx, HookWakeRMQConfigKey, cfg) != nil || !cfg.Valid() || cfg.ProduceTimeout <= 0 || cfg.ConsumeTimeout <= 0 || cfg.WorkerNum <= 0 {
		return nil, ErrHookWakeUnavailable
	}
	return cfg, nil
}

func (w *HookRuntimeWake) PublishWake(ctx context.Context, event entity.HookWakeEvent) error {
	if _, err := entity.EncodeHookWakeEvent(event); err != nil {
		return err
	}
	if event.ExecutionScope != w.scope {
		return ErrHookWorkerScope
	}
	cfg, err := w.brokerConfig(ctx)
	if err != nil {
		return err
	}
	if cfg == nil {
		return nil
	}
	if cfg.DisableProduce != nil && *cfg.DisableProduce {
		return ErrHookWakeUnavailable
	}
	// Each bounded hint owns its producer, including failure cleanup. Injectors
	// used by OpenAPI have no separate background producer to leak at shutdown.
	p, err := w.factory.NewProducer(cfg.ToProducerCfg())
	if err != nil || hookWorkerNil(p) {
		return ErrHookWakeUnavailable
	}
	defer p.Close()
	if p.Start() != nil {
		return ErrHookWakeUnavailable
	}
	publisher, err := producer.NewHookWakePublisher(p, cfg.Topic, cfg.ProduceTimeout)
	if err != nil {
		return ErrHookWakeUnavailable
	}
	// Match the existing publisher's SDK key, using deployment identity rather
	// than the request's possibly inherited lane.
	ctx = context.WithValue(ctx, producer.CtxKeyEnv, w.environment) //nolint:staticcheck
	return publisher.PublishWake(ctx, event)
}

func (s *HookRuntimeServices) ExecutionScope(context.Context) (string, error) {
	if s == nil || s.Wake == nil {
		return "", ErrHookWorkerConfiguration
	}
	return s.Wake.scope, nil
}

// Start is called only by the process owning Hook scans (Commercial offline).
// The ordinary API and evaluator consumers still install the routing/Gate.
// Switching polling-only to MQ requires a process restart to subscribe.
func (s *HookRuntimeServices) Start(ctx context.Context, handler mq.IConsumerHandler) (func(context.Context) error, error) {
	if s == nil || s.Worker == nil || s.Wake == nil || ctx == nil {
		return nil, ErrHookWorkerConfiguration
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return nil, ErrHookWorkerRunning
	}
	s.started = true
	s.generation++
	generation := s.generation
	s.mu.Unlock()
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); _ = s.Worker.Run(runCtx) }()
	var consumer mq.IConsumer
	var once sync.Once
	var closeErr error
	stop := func(stopCtx context.Context) error {
		once.Do(func() {
			cancel()
			if !hookWorkerNil(consumer) {
				if consumer.Close() != nil {
					closeErr = ErrHookWakeUnavailable
				}
			}
			_ = s.Worker.Stop(stopCtx)
		})
		select {
		case <-done:
			s.mu.Lock()
			if s.generation == generation {
				s.started = false
			}
			s.mu.Unlock()
			return closeErr
		case <-stopCtx.Done():
			return stopCtx.Err()
		}
	}
	cfg, err := s.Wake.brokerConfig(ctx)
	if err != nil {
		// Missing/disabled config keeps scans idle; active config with no broker is
		// not an excuse to disable durable recovery. Polling remains authoritative.
		hookRuntimeObserver{}.Observe(hook.WorkerEvent{Code: hook.WorkerConfigFailed})
		return stop, nil
	}
	if cfg == nil || cfg.DisableConsume != nil && *cfg.DisableConsume {
		return stop, nil
	}
	if hookWorkerNil(handler) {
		cleanup, release := context.WithTimeout(context.Background(), 5*time.Second)
		defer release()
		_ = stop(cleanup)
		return nil, ErrHookWakeUnavailable
	}
	consumerCfg := cfg.ToConsumerCfg()
	consumerCfg.TagExpression = hook.WakeMessageTag
	consumer, err = s.Wake.factory.NewConsumer(consumerCfg)
	if err == nil && !hookWorkerNil(consumer) {
		consumer.RegisterHandler(handler)
		err = consumer.Start()
	} else if err == nil {
		err = ErrHookWakeUnavailable
	}
	if err != nil {
		cleanup, release := context.WithTimeout(context.Background(), 5*time.Second)
		defer release()
		_ = stop(cleanup)
		return nil, ErrHookWakeUnavailable
	}
	return stop, nil
}
