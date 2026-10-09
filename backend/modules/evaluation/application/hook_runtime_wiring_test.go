// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	infraHook "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/mq/rocket"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	"github.com/stretchr/testify/require"
)

type wiringLoader struct {
	conf.IConfigLoader
	value any
}

func (l wiringLoader) Get(context.Context, string) any { return l.value }
func (l wiringLoader) UnmarshalKey(context.Context, string, any, ...conf.DecodeOptionFn) error {
	return errors.New("missing broker configuration")
}

func TestHookRuntimeWiringStorageKeyIsNotSigningKey(t *testing.T) {
	for _, value := range []any{nil, "{}", `{"storage_key_id":""}`, `{"storage_key_id":"bad key"}`} {
		p := infraHook.NewRuntimeConfigProvider(wiringLoader{value: value}, true)
		key, err := p.StorageKeyID(context.Background())
		require.Error(t, err)
		require.Empty(t, key)
	}
	p := infraHook.NewRuntimeConfigProvider(wiringLoader{value: `{"storage_key_id":"storage-v1","worker_enabled":false}`}, true)
	key, err := p.StorageKeyID(context.Background())
	require.NoError(t, err)
	require.Equal(t, "storage-v1", key)
}

func TestHookRuntimeWiringAdmissionRequiresStorageBackendAndIndependentBroker(t *testing.T) {
	const value = `{"admission_enabled":true,"storage_key_id":"storage-v1","endpoint_policy":[{"workspace_id":42,"host":"example.com","port":443,"environment":"Prod"}],"signing_key_refs":[{"workspace_id":42,"host":"example.com","port":443,"environment":"Prod","key_id":"sign-v1","key_ref":"sign-ref"}]}`
	for _, mode := range []string{"ready", "no_backend", "no_storage_key", "no_broker", "disabled_producer"} {
		t.Run(mode, func(t *testing.T) {
			loader := &wiringBrokerLoader{wiringLoader: wiringLoader{value: value}, broker: rocket.RMQConf{Addr: "test-broker", Topic: "test-hook-wake", ConsumerGroup: "test-hook-worker", ProduceTimeout: time.Second, ConsumeTimeout: time.Second, WorkerNum: 1}}
			config := infraHook.NewRuntimeConfigProvider(loader, true)
			p := &HookRuntimePlatform{Config: config, StorageKeyID: "storage-v1", ProtectedBackend: true, Wake: NewHookRuntimeWake(loader, &wiringMQ{}, config, "test", nil)}
			switch mode {
			case "no_backend":
				p.ProtectedBackend = false
			case "no_storage_key":
				loader.value = strings.Replace(value, `"storage_key_id":"storage-v1",`, "", 1)
			case "no_broker":
				loader.broker = rocket.RMQConf{}
			case "disabled_producer":
				disabled := true
				loader.broker.DisableProduce = &disabled
			}
			cfg, err := (hookRuntimeAdmissionConfig{p}).GetRuntimeConfig(context.Background())
			if mode == "ready" {
				require.NoError(t, err)
				require.True(t, cfg.AdmissionEnabled)
			} else {
				require.Error(t, err)
			}
		})
	}
}

type wiringMQ struct {
	mq.IFactory
	calls int
}

func (m *wiringMQ) NewProducer(mq.ProducerConfig) (mq.IProducer, error) {
	m.calls++
	panic("unexpected external producer")
}
func (m *wiringMQ) NewConsumer(mq.ConsumerConfig) (mq.IConsumer, error) {
	m.calls++
	panic("unexpected external consumer")
}

func TestHookRuntimeWiringDisabledNeverCreatesBroker(t *testing.T) {
	for _, value := range []any{nil, "{}", `{"worker_enabled":false,"admission_enabled":false}`, "invalid"} {
		factory := &wiringMQ{}
		loader := wiringLoader{value: value}
		config := infraHook.NewRuntimeConfigProvider(loader, true)
		wake := NewHookRuntimeWake(loader, factory, config, "local/test", nil)
		err := wake.PublishWake(context.Background(), entity.HookWakeEvent{Run: entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}, OperationID: "before-3", ExecutionScope: "local/test"})
		require.Error(t, err)
		require.Zero(t, factory.calls)
	}
}

func TestHookRuntimePollingOnlyAdmission(t *testing.T) {
	const value = `{"mq_wake_enabled":false,"admission_enabled":true,"storage_key_id":"storage-v1","endpoint_policy":[{"workspace_id":42,"host":"example.com","port":443,"environment":"Prod"}],"signing_key_refs":[{"workspace_id":42,"host":"example.com","port":443,"environment":"Prod","key_id":"sign-v1","key_ref":"sign-ref"}]}`
	for _, mode := range []string{"ready", "no_factory", "no_backend", "no_storage_key", "changed_key", "worker_disabled", "worker_not_installed", "invalid_policy", "admission_disabled"} {
		t.Run(mode, func(t *testing.T) {
			loader := &wiringBrokerLoader{wiringLoader: wiringLoader{value: value}}
			factory := &wiringBrokerFactory{}
			config := infraHook.NewRuntimeConfigProvider(loader, mode != "worker_not_installed")
			p := &HookRuntimePlatform{Config: config, StorageKeyID: "storage-v1", ProtectedBackend: true, Wake: NewHookRuntimeWake(loader, factory, config, "test", nil)}
			switch mode {
			case "no_factory":
				p.Wake.factory = nil
			case "no_backend":
				p.ProtectedBackend = false
			case "no_storage_key":
				loader.value = strings.Replace(value, `"storage_key_id":"storage-v1",`, "", 1)
			case "changed_key":
				p.StorageKeyID = "storage-v2"
			case "worker_disabled":
				loader.value = strings.Replace(value, `"mq_wake_enabled":false`, `"mq_wake_enabled":false,"worker_enabled":false`, 1)
			case "invalid_policy":
				loader.value = strings.Replace(value, `"port":443`, `"port":0`, 1)
			case "admission_disabled":
				loader.value = `{"mq_wake_enabled":false,"admission_enabled":false}`
			}
			cfg, err := (hookRuntimeAdmissionConfig{p}).GetRuntimeConfig(context.Background())
			if mode == "ready" || mode == "no_factory" || mode == "admission_disabled" {
				require.NoError(t, err)
				require.Equal(t, mode != "admission_disabled", cfg.AdmissionEnabled)
				require.True(t, cfg.WorkerEnabled)
			} else {
				require.Error(t, err)
				require.Equal(t, entity.HookRuntimeConfig{}, cfg)
			}
			require.Empty(t, loader.keys, "polling-only admission must not read broker configuration")
			require.Zero(t, factory.producerCalls)
			require.Zero(t, factory.consumerCalls)
		})
	}
}
