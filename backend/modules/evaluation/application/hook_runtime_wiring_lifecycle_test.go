// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	infraHook "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/mq/rocket"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	"github.com/stretchr/testify/require"
)

type wiringBrokerLoader struct {
	wiringLoader
	broker rocket.RMQConf
	keys   []string
}

func (l *wiringBrokerLoader) UnmarshalKey(_ context.Context, key string, out any, _ ...conf.DecodeOptionFn) error {
	l.keys = append(l.keys, key)
	if key != HookWakeRMQConfigKey {
		return errors.New("unexpected broker config")
	}
	*out.(*rocket.RMQConf) = l.broker
	return nil
}

type wiringProducer struct {
	mq.IProducer
	starts, closes, sends int
	failure               error
	message               *mq.Message
	environment           any
}

func (p *wiringProducer) Start() error { p.starts++; return nil }
func (p *wiringProducer) Close() error { p.closes++; return nil }
func (p *wiringProducer) Send(ctx context.Context, m *mq.Message) (mq.SendResponse, error) {
	p.sends++
	p.message = m
	p.environment = ctx.Value("K_ENV")
	return mq.SendResponse{}, p.failure
}

type wiringConsumer struct {
	starts, closes int
	handler        mq.IConsumerHandler
	failure        error
	closeFailure   error
}

func (c *wiringConsumer) Start() error                          { c.starts++; return c.failure }
func (c *wiringConsumer) Close() error                          { c.closes++; return c.closeFailure }
func (c *wiringConsumer) RegisterHandler(h mq.IConsumerHandler) { c.handler = h }

type wiringBrokerFactory struct {
	producer                     *wiringProducer
	consumer                     *wiringConsumer
	pc                           mq.ProducerConfig
	cc                           mq.ConsumerConfig
	producerCalls, consumerCalls int
}

func (f *wiringBrokerFactory) NewProducer(cfg mq.ProducerConfig) (mq.IProducer, error) {
	f.producerCalls++
	f.pc = cfg
	return f.producer, nil
}
func (f *wiringBrokerFactory) NewConsumer(cfg mq.ConsumerConfig) (mq.IConsumer, error) {
	f.consumerCalls++
	f.cc = cfg
	return f.consumer, nil
}

type wiringHandler struct{}

func (wiringHandler) HandleMessage(context.Context, *mq.MessageExt) error { return nil }

func wiringLifecycleFixture(t *testing.T, enabled bool) (*HookRuntimeServices, *wiringBrokerFactory, *wiringBrokerLoader) {
	t.Helper()
	value := `{"worker_enabled":true}`
	if !enabled {
		value = `{"worker_enabled":false}`
	}
	loader := &wiringBrokerLoader{wiringLoader: wiringLoader{value: value}, broker: rocket.RMQConf{Addr: "test-broker", Topic: "test-hook-wake", ConsumerGroup: "test-hook-worker", ProducerGroup: "test-hook-producer", ProduceTimeout: time.Second, ConsumeTimeout: time.Second, WorkerNum: 1}}
	factory := &wiringBrokerFactory{producer: &wiringProducer{}, consumer: &wiringConsumer{}}
	config := infraHook.NewRuntimeConfigProvider(loader, true)
	deps := workerDeps()
	deps.Config = config
	worker, err := NewHookWorker(deps)
	require.NoError(t, err)
	return &HookRuntimeServices{Worker: worker, Wake: NewHookRuntimeWake(loader, factory, config, "worker-local", nil)}, factory, loader
}

func TestHookRuntimeWiringLifecycleDisabledStartsNoBrokerAndJoins(t *testing.T) {
	s, f, _ := wiringLifecycleFixture(t, false)
	stop, err := s.Start(context.Background(), wiringHandler{})
	require.NoError(t, err)
	require.Zero(t, f.consumerCalls)
	require.Zero(t, f.producerCalls)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, stop(ctx))
	require.NoError(t, stop(ctx))
	s.Worker.mu.Lock()
	active := s.Worker.session
	s.Worker.mu.Unlock()
	require.Nil(t, active)
}

func TestHookRuntimeWiringLifecycleRejectsDoubleStart(t *testing.T) {
	s, _, _ := wiringLifecycleFixture(t, false)
	stop, err := s.Start(context.Background(), wiringHandler{})
	require.NoError(t, err)
	defer stop(context.Background())
	second, err := s.Start(context.Background(), wiringHandler{})
	require.ErrorIs(t, err, ErrHookWorkerRunning)
	require.Nil(t, second)
}

func TestHookRuntimeWiringOldStopCannotStopNextGeneration(t *testing.T) {
	s, _, _ := wiringLifecycleFixture(t, false)
	first, err := s.Start(context.Background(), wiringHandler{})
	require.NoError(t, err)
	require.NoError(t, first(context.Background()))
	second, err := s.Start(context.Background(), wiringHandler{})
	require.NoError(t, err)
	defer second(context.Background())
	require.NoError(t, first(context.Background()))
	third, err := s.Start(context.Background(), wiringHandler{})
	require.ErrorIs(t, err, ErrHookWorkerRunning)
	require.Nil(t, third)
}

func TestHookRuntimeWiringWakeUsesDeploymentLane(t *testing.T) {
	s, f, _ := wiringLifecycleFixture(t, true)
	s.Wake.environment = "ppe_trusted"
	ctx := context.WithValue(context.Background(), "K_ENV", "ppe_untrusted") //nolint:staticcheck
	require.NoError(t, s.Wake.PublishWake(ctx, entity.HookWakeEvent{Run: entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}, OperationID: "before-3", ExecutionScope: "worker-local"}))
	require.Equal(t, "ppe_trusted", f.producer.environment)
}

func TestHookRuntimeWiringLifecycleOwnsIndependentSubscription(t *testing.T) {
	s, f, l := wiringLifecycleFixture(t, true)
	stop, err := s.Start(context.Background(), wiringHandler{})
	require.NoError(t, err)
	require.Equal(t, []string{"expt_lifecycle_hook_wake_rmq"}, l.keys)
	require.Equal(t, "test-hook-wake", f.cc.Topic)
	require.Equal(t, "test-hook-worker", f.cc.ConsumerGroup)
	require.Equal(t, hook.WakeMessageTag, f.cc.TagExpression)
	require.Equal(t, 1, f.consumer.starts)
	require.NotNil(t, f.consumer.handler)
	require.Zero(t, f.producerCalls)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, stop(ctx))
	require.NoError(t, stop(ctx))
	require.Equal(t, 1, f.consumer.closes)
}

func TestHookRuntimeWiringConsumerStartFailureClosesAndJoins(t *testing.T) {
	s, f, _ := wiringLifecycleFixture(t, true)
	f.consumer.failure = errors.New("private broker error")
	stop, err := s.Start(context.Background(), wiringHandler{})
	require.ErrorIs(t, err, ErrHookWakeUnavailable)
	require.Nil(t, stop)
	require.Equal(t, 1, f.consumer.closes)
	s.Worker.mu.Lock()
	active := s.Worker.session
	s.Worker.mu.Unlock()
	require.Nil(t, active)
}

func TestHookRuntimeWiringConsumerCloseFailureIsReported(t *testing.T) {
	s, f, _ := wiringLifecycleFixture(t, true)
	f.consumer.closeFailure = errors.New("private broker close error")
	stop, err := s.Start(context.Background(), wiringHandler{})
	require.NoError(t, err)
	err = stop(context.Background())
	require.ErrorIs(t, err, ErrHookWakeUnavailable)
	require.NotContains(t, err.Error(), "private")
	require.Equal(t, 1, f.consumer.closes)
}

func TestHookRuntimeWiringWakeOwnsProducerOnSendFailure(t *testing.T) {
	s, f, l := wiringLifecycleFixture(t, true)
	f.producer.failure = errors.New("private broker error")
	key := entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}
	err := s.Wake.PublishWake(context.Background(), entity.HookWakeEvent{Run: key, OperationID: "before-3", ExecutionScope: "worker-local"})
	require.Error(t, err)
	require.NotContains(t, err.Error(), "private")
	require.Equal(t, []string{"expt_lifecycle_hook_wake_rmq"}, l.keys)
	require.Equal(t, 1, f.producer.starts)
	require.Equal(t, 1, f.producer.closes)
	require.Equal(t, 1, f.producer.sends)
	require.Equal(t, "test-hook-wake", f.producer.message.Topic)
	require.Equal(t, hook.WakeMessageTag, f.producer.message.Tag)
	decoded, err := entity.DecodeHookWakeEvent(f.producer.message.Body)
	require.NoError(t, err)
	require.Equal(t, key, decoded.Run)
}

func TestHookRuntimePollingOnlyPublishWake(t *testing.T) {
	for _, mode := range []string{"ready", "no_factory", "no_broker_loader", "nil_context", "cancelled", "invalid_event", "wrong_scope", "worker_disabled", "malformed_config"} {
		t.Run(mode, func(t *testing.T) {
			s, f, l := wiringLifecycleFixture(t, true)
			l.value = `{"mq_wake_enabled":false}`
			ctx := context.Background()
			event := entity.HookWakeEvent{Run: entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}, OperationID: "before-3", ExecutionScope: "worker-local"}
			switch mode {
			case "no_factory":
				s.Wake.factory = nil
			case "no_broker_loader":
				s.Wake.loader = nil
			case "nil_context":
				ctx = nil
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "invalid_event":
				event.Run.RunID = 0
			case "wrong_scope":
				event.ExecutionScope = "other-scope"
			case "worker_disabled":
				l.value = `{"mq_wake_enabled":false,"worker_enabled":false}`
			case "malformed_config":
				l.value = `{"mq_wake_enabled":false,"worker_concurrency":0}`
			}
			err := s.Wake.PublishWake(ctx, event)
			if mode == "ready" || mode == "no_factory" || mode == "no_broker_loader" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			if mode == "wrong_scope" {
				require.ErrorIs(t, err, ErrHookWorkerScope)
			}
			require.Empty(t, l.keys)
			require.Zero(t, f.producerCalls)
			require.Zero(t, f.consumerCalls)
		})
	}
}

func TestHookRuntimePollingOnlyStartScansWithoutMessagesAndStops(t *testing.T) {
	s, f, l := wiringLifecycleFixture(t, true)
	l.value = `{"mq_wake_enabled":false,"scan_interval_seconds":1}`
	var scans atomic.Int32
	executed := make(chan hook.AttemptExecutionInput, 1)
	exited := make(chan struct{})
	s.Worker.deps.ScanRepo = &workerScan{operations: func(kind entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
		if kind == entity.HookScanDue && in.Status == entity.HookOperationPending && scans.Add(1) == 2 {
			return entity.HookScanPage[entity.HookOperationCandidate]{Candidates: []entity.HookOperationCandidate{workerCandidate(42, 3)}}, nil
		}
		return entity.HookScanPage[entity.HookOperationCandidate]{}, nil
	}}
	s.Worker.deps.Executor = workerExecute(func(ctx context.Context, in hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
		executed <- in
		<-ctx.Done()
		close(exited)
		return hook.AttemptExecutionResult{}, ctx.Err()
	})
	stop, err := s.Start(context.Background(), nil)
	require.NoError(t, err)
	defer stop(context.Background())
	require.Empty(t, l.keys)
	require.Zero(t, f.consumerCalls)
	require.Zero(t, f.producerCalls)
	select {
	case in := <-executed:
		require.Equal(t, workerCandidate(42, 3).Key, in.Key)
		require.Equal(t, "worker-local", in.ExecutionScope)
		require.Equal(t, "op-3", in.OperationID)
	case <-time.After(4 * time.Second):
		t.Fatal("periodic scan did not execute the candidate without any wake message")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, stop(ctx))
	require.NoError(t, stop(ctx))
	select {
	case <-exited:
	default:
		t.Fatal("stop did not join the in-flight executor")
	}
	s.Worker.mu.Lock()
	active := s.Worker.session
	s.Worker.mu.Unlock()
	require.Nil(t, active)
	require.Zero(t, f.consumer.closes)
}

func TestHookRuntimeMQWakeDefaultAndExplicitTrue(t *testing.T) {
	for _, raw := range []string{`{}`, `{"mq_wake_enabled":true}`} {
		t.Run(raw, func(t *testing.T) {
			for _, mode := range []string{"ready", "no_broker", "disabled_producer", "disabled_consumer", "no_factory"} {
				t.Run(mode, func(t *testing.T) {
					s, f, l := wiringLifecycleFixture(t, true)
					l.value = raw
					disabled := true
					switch mode {
					case "no_broker":
						l.broker = rocket.RMQConf{}
					case "disabled_producer":
						l.broker.DisableProduce = &disabled
					case "disabled_consumer":
						l.broker.DisableConsume = &disabled
					case "no_factory":
						s.Wake.factory = nil
					}
					err := s.Wake.PublishWake(context.Background(), entity.HookWakeEvent{Run: entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}, OperationID: "before-3", ExecutionScope: "worker-local"})
					if mode == "ready" || mode == "disabled_consumer" {
						require.NoError(t, err)
						require.Equal(t, 1, f.producer.sends)
						require.Equal(t, 1, f.producer.closes)
					} else {
						require.ErrorIs(t, err, ErrHookWakeUnavailable)
						require.Zero(t, f.producerCalls)
					}
					stop, err := s.Start(context.Background(), wiringHandler{})
					require.NoError(t, err)
					require.NoError(t, stop(context.Background()))
					if mode == "ready" || mode == "disabled_producer" {
						require.Equal(t, 1, f.consumerCalls)
						require.Equal(t, 1, f.consumer.starts)
						require.Equal(t, 1, f.consumer.closes)
					} else {
						require.Zero(t, f.consumerCalls)
					}
				})
			}
		})
	}
}

func TestHookRuntimePollingOnlyStartWithoutBrokerDependencies(t *testing.T) {
	s, f, l := wiringLifecycleFixture(t, true)
	l.value = `{"mq_wake_enabled":false}`
	s.Wake.factory, s.Wake.loader = nil, nil
	stop, err := s.Start(context.Background(), nil)
	require.NoError(t, err)
	require.NoError(t, stop(context.Background()))
	require.Empty(t, l.keys)
	require.Zero(t, f.consumerCalls)
	require.Zero(t, f.producerCalls)
}

func TestHookRuntimePollingOnlyStartInvalidOrDisabledStaysIdle(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		code hook.WorkerEventCode
	}{
		{`{"mq_wake_enabled":false,"worker_enabled":false}`, hook.WorkerPaused},
		{`{"mq_wake_enabled":false,"worker_concurrency":0}`, hook.WorkerConfigFailed},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			s, f, l := wiringLifecycleFixture(t, true)
			l.value = tc.raw
			observer := s.Worker.deps.Observer.(*workerObservation)
			clock := s.Worker.deps.Clock.(*workerClock)
			stop, err := s.Start(context.Background(), nil)
			require.NoError(t, err)
			defer stop(context.Background())
			require.Eventually(t, func() bool { return observer.count(tc.code) > 0 }, time.Second, time.Millisecond)
			require.NoError(t, stop(context.Background()))
			require.Zero(t, clock.calls.Load(), "disabled or invalid config must not reach scans")
			require.Empty(t, l.keys)
			require.Zero(t, f.consumerCalls)
			require.Zero(t, f.producerCalls)
		})
	}
}
