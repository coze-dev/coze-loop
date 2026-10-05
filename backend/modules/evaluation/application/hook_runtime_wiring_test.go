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
