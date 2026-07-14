// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/mq/rocket"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
)

// fakeLoader is a minimal conf.IConfigLoader used to exercise the two
// branches of WebhookDeliveryEventConsumer.ConsumerCfg:
//   - a nil-error load that lands an invalid RMQConf (missing topic /
//     addr / consumer_group) -> topic-not-provisioned scenario
//   - a load that returns an error -> yaml malformed / auth failure etc.
type fakeLoader struct {
	unmarshalErr error
	rmqCfg       rocket.RMQConf
	unmarshalHit int
}

func (f *fakeLoader) Get(_ context.Context, _ string) any { return nil }
func (f *fakeLoader) Unmarshal(_ context.Context, _ any, _ ...conf.DecodeOptionFn) error {
	return nil
}

func (f *fakeLoader) UnmarshalKey(_ context.Context, _ string, v any, _ ...conf.DecodeOptionFn) error {
	f.unmarshalHit++
	if f.unmarshalErr != nil {
		return f.unmarshalErr
	}
	if dst, ok := v.(*rocket.RMQConf); ok {
		*dst = f.rmqCfg
	}
	return nil
}

// TestWebhookConsumerCfg_TopicMissingReturnsDisabledCfgNoError models the boe
// deploy failure: the yaml key is absent so UnmarshalKey silently leaves
// RMQConf zero-value and Valid() returns false. Registry.StartAll then reads
// IsEnabled=false and skips subscribe, letting the pod reach readiness. This
// is the "topic-not-exist" tolerance path — must not return an error to the
// boot layer.
func TestWebhookConsumerCfg_TopicMissingReturnsDisabledCfgNoError(t *testing.T) {
	loader := &fakeLoader{} // empty rmqCfg → Valid()==false
	worker := NewWebhookDeliveryEventConsumer(nil, loader)

	cfg, err := worker.ConsumerCfg(context.Background())
	require.NoError(t, err, "topic-missing must NOT propagate as boot error")
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.IsEnabled)
	require.False(t, gptr.Indirect(cfg.IsEnabled), "cfg must be flagged disabled so registry skips subscribe")
	require.Equal(t, 1, loader.unmarshalHit)
}

// TestWebhookConsumerCfg_UnmarshalErrorStillReturnsError guards the "other
// error" branch: a genuine loader failure (yaml malformed, auth denied) must
// still surface as a boot error rather than being masked by the graceful
// topic-missing tolerance path.
func TestWebhookConsumerCfg_UnmarshalErrorStillReturnsError(t *testing.T) {
	loader := &fakeLoader{unmarshalErr: errors.New("boom: config store 5xx")}
	worker := NewWebhookDeliveryEventConsumer(nil, loader)

	cfg, err := worker.ConsumerCfg(context.Background())
	require.Error(t, err)
	require.Nil(t, cfg)
	require.Contains(t, err.Error(), "boom")
}

// TestWebhookConsumerCfg_ValidConfigReturnsEnabledCfg pins the happy path:
// once ops provisions the topic and yaml gains the key, ConsumerCfg returns
// an enabled cfg (IsEnabled left nil == default enabled) so registry.StartAll
// proceeds normally. Only DisableConsume on the RMQConf side flips the flag.
func TestWebhookConsumerCfg_ValidConfigReturnsEnabledCfg(t *testing.T) {
	loader := &fakeLoader{
		rmqCfg: rocket.RMQConf{
			Addr:          "rmq.local:9876",
			Topic:         "evaluation_webhook_delivery",
			ConsumerGroup: "cg_evaluation_webhook_delivery",
		},
	}
	worker := NewWebhookDeliveryEventConsumer(nil, loader)

	cfg, err := worker.ConsumerCfg(context.Background())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "evaluation_webhook_delivery", cfg.Topic)
	require.Equal(t, "cg_evaluation_webhook_delivery", cfg.ConsumerGroup)
	require.Nil(t, cfg.IsEnabled, "valid cfg with DisableConsume=false leaves IsEnabled at nil (default enabled)")
}

// TestNilWebhookConsumer_ConsumerCfgDisabledHandleMessageNoop verifies the
// rollback stub: NewNilWebhookDeliveryEventConsumer produces a worker that
// reports IsEnabled=false (so registry skips subscribe) AND treats any
// message delivered to HandleMessage as a no-op (defence in depth if some
// caller invokes it directly). Both properties are load-bearing for the
// E-I-03 rollback path.
func TestNilWebhookConsumer_ConsumerCfgDisabledHandleMessageNoop(t *testing.T) {
	worker := NewNilWebhookDeliveryEventConsumer()

	cfg, err := worker.ConsumerCfg(context.Background())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.IsEnabled)
	require.False(t, gptr.Indirect(cfg.IsEnabled))

	require.NoError(t, worker.HandleMessage(context.Background(), &mq.MessageExt{}))
	// idempotent when called repeatedly (registry.StopAll safety).
	require.NoError(t, worker.HandleMessage(context.Background(), nil))
}

// TestWebhookDeliveryConsumerFrom_NilDepUsesNilConsumer covers the NewConsumerWorkers
// guard: when any of the 4 core deps is nil (mirrors commercial's
// provideNilWebhookDispatcher rollout / E-I-03 rollback) the wire-time helper
// returns the nil-consumer stub — never the real WebhookDeliveryConsumer,
// which would panic on the first message with a nil sender / repo.
func TestWebhookDeliveryConsumerFrom_NilDepUsesNilConsumer(t *testing.T) {
	senderReal := &fakeSender{}
	repoReal := &fakeDeliveryRepo{}
	pubReal := &fakePublisher{}
	cfgerReal := fakeConfiger{}

	t.Run("nil_sender", func(t *testing.T) {
		worker := webhookDeliveryConsumerFrom(nil, nil, repoReal, pubReal, cfgerReal, nil, nil, nil)
		require.IsType(t, &nilWebhookDeliveryEventConsumer{}, worker)
	})
	t.Run("nil_delivery_repo", func(t *testing.T) {
		worker := webhookDeliveryConsumerFrom(nil, senderReal, nil, pubReal, cfgerReal, nil, nil, nil)
		require.IsType(t, &nilWebhookDeliveryEventConsumer{}, worker)
	})
	t.Run("nil_publisher", func(t *testing.T) {
		worker := webhookDeliveryConsumerFrom(nil, senderReal, repoReal, nil, cfgerReal, nil, nil, nil)
		require.IsType(t, &nilWebhookDeliveryEventConsumer{}, worker)
	})
	t.Run("nil_configer", func(t *testing.T) {
		worker := webhookDeliveryConsumerFrom(nil, senderReal, repoReal, pubReal, nil, nil, nil, nil)
		require.IsType(t, &nilWebhookDeliveryEventConsumer{}, worker)
	})
}

// TestWebhookDeliveryConsumerFrom_DisableConsumerConfSelectsNilConsumer models the
// ops-side kill switch: if the runtime WebhookGlobalConf turns DisableConsumer on,
// the wire-time helper must return the nil stub so operators can drop the
// consumer without a redeploy / wire regen.
func TestWebhookDeliveryConsumerFrom_DisableConsumerConfSelectsNilConsumer(t *testing.T) {
	sender := &fakeSender{}
	deliveryRepo := &fakeDeliveryRepo{}
	publisher := &fakePublisher{}
	configer := fakeConfiger{global: &entity.WebhookGlobalConf{Enabled: true, DisableConsumer: true}}

	worker := webhookDeliveryConsumerFrom(nil, sender, deliveryRepo, publisher, configer, nil, nil, nil)
	require.IsType(t, &nilWebhookDeliveryEventConsumer{}, worker,
		"DisableConsumer=true must select the nil consumer stub")
}

// TestWebhookDeliveryConsumerFrom_HappyPathSelectsRealConsumer pins the
// positive case: with all deps wired and no kill switch, we return the real
// worker so retries actually fire once ops provisions the topic.
func TestWebhookDeliveryConsumerFrom_HappyPathSelectsRealConsumer(t *testing.T) {
	sender := &fakeSender{}
	deliveryRepo := &fakeDeliveryRepo{}
	publisher := &fakePublisher{}
	configer := fakeConfiger{}
	loader := &fakeLoader{
		rmqCfg: rocket.RMQConf{
			Addr:          "rmq.local:9876",
			Topic:         "evaluation_webhook_delivery",
			ConsumerGroup: "cg_evaluation_webhook_delivery",
		},
	}

	worker := webhookDeliveryConsumerFrom(loader, sender, deliveryRepo, publisher, configer, nil, nil, nil)
	require.IsType(t, &WebhookDeliveryEventConsumer{}, worker)

	// ConsumerCfg must reflect the loader-driven cfg, not the nil stub.
	cfg, err := worker.ConsumerCfg(context.Background())
	require.NoError(t, err)
	require.Nil(t, cfg.IsEnabled, "happy-path worker must not carry the disabled flag")
	require.Equal(t, "evaluation_webhook_delivery", cfg.Topic)
}
