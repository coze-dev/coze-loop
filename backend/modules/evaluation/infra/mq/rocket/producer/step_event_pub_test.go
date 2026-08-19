// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	mqmocks "github.com/coze-dev/coze-loop/backend/infra/mq/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/mq/rocket"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	confmocks "github.com/coze-dev/coze-loop/backend/pkg/conf/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

func stepEventFixture() *entity.SandboxStepEventMessage {
	return &entity.SandboxStepEventMessage{
		EventType: "FINISHED",
		StepName:  "agent_run",
		InvokeID:  999,
	}
}

// newLoaderFactory 造一个把 expt_sandbox_step_event_rmq 解成 cfg 的 config loader factory。
func newLoaderFactory(t *testing.T, ctrl *gomock.Controller, cfg rocket.RMQConf, unmarshalErr error) conf.IConfigLoaderFactory {
	t.Helper()
	loader := confmocks.NewMockIConfigLoader(ctrl)
	loader.EXPECT().UnmarshalKey(gomock.Any(), rocket.ExptSandboxStepEventRMQKey, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, out any, _ ...conf.DecodeOptionFn) error {
			if unmarshalErr != nil {
				return unmarshalErr
			}
			target, ok := out.(*rocket.RMQConf)
			require.True(t, ok)
			*target = cfg
			return nil
		}).AnyTimes()

	factory := confmocks.NewMockIConfigLoaderFactory(ctrl)
	factory.EXPECT().NewConfigLoader(gomock.Any()).Return(loader, nil).AnyTimes()
	return factory
}

func validStepEventCfg() rocket.RMQConf {
	return rocket.RMQConf{
		Addr:  "namesrv:9876",
		Topic: "evaluation_expt_sandbox_step_event",
		// 刻意不设 consumer_group：本 topic 只生产，消费方是数仓侧的同步任务。
		// 如果实现用了 RMQConf.Valid()，这条用例会失败。
		ProducerGroup: "evaluation_expt_sandbox_step_event_pg",
	}
}

// TestNewStepEventPublisher_DegradesToNoop 覆盖全部构造失败路径。
// 关键点：**一条都不能返回 error 或 panic**——埋点 topic 配错了不该让整个评测服务起不来。
// 这与同包 newExptEventPublisher 的 fail-fast 行为刻意相反。
func TestNewStepEventPublisher_DegradesToNoop(t *testing.T) {
	ctx := context.Background()

	t.Run("nil factories", func(t *testing.T) {
		assert.IsType(t, &noopStepEventPublisher{}, newStepEventPublisher(ctx, nil, nil))
	})

	t.Run("config loader creation fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		factory := confmocks.NewMockIConfigLoaderFactory(ctrl)
		factory.EXPECT().NewConfigLoader(gomock.Any()).Return(nil, errors.New("no such config"))
		assert.IsType(t, &noopStepEventPublisher{}, newStepEventPublisher(ctx, factory, mqmocks.NewMockIFactory(ctrl)))
	})

	t.Run("unmarshal fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		factory := newLoaderFactory(t, ctrl, rocket.RMQConf{}, errors.New("bad yaml"))
		assert.IsType(t, &noopStepEventPublisher{}, newStepEventPublisher(ctx, factory, mqmocks.NewMockIFactory(ctrl)))
	})

	t.Run("disable_produce kill switch", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		cfg := validStepEventCfg()
		cfg.DisableProduce = gptr.Of(true)
		// mqFactory 上不加任何 EXPECT：开关生效就不该去建 producer。
		factory := newLoaderFactory(t, ctrl, cfg, nil)
		assert.IsType(t, &noopStepEventPublisher{}, newStepEventPublisher(ctx, factory, mqmocks.NewMockIFactory(ctrl)))
	})

	t.Run("empty addr", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		cfg := validStepEventCfg()
		cfg.Addr = ""
		factory := newLoaderFactory(t, ctrl, cfg, nil)
		assert.IsType(t, &noopStepEventPublisher{}, newStepEventPublisher(ctx, factory, mqmocks.NewMockIFactory(ctrl)))
	})

	t.Run("empty topic", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		cfg := validStepEventCfg()
		cfg.Topic = ""
		factory := newLoaderFactory(t, ctrl, cfg, nil)
		assert.IsType(t, &noopStepEventPublisher{}, newStepEventPublisher(ctx, factory, mqmocks.NewMockIFactory(ctrl)))
	})

	t.Run("producer creation fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mqFactory := mqmocks.NewMockIFactory(ctrl)
		mqFactory.EXPECT().NewProducer(gomock.Any()).Return(nil, errors.New("dial fail"))
		factory := newLoaderFactory(t, ctrl, validStepEventCfg(), nil)
		assert.IsType(t, &noopStepEventPublisher{}, newStepEventPublisher(ctx, factory, mqFactory))
	})

	t.Run("producer start fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		p := mqmocks.NewMockIProducer(ctrl)
		p.EXPECT().Start().Return(errors.New("start fail"))
		mqFactory := mqmocks.NewMockIFactory(ctrl)
		mqFactory.EXPECT().NewProducer(gomock.Any()).Return(p, nil)
		factory := newLoaderFactory(t, ctrl, validStepEventCfg(), nil)
		assert.IsType(t, &noopStepEventPublisher{}, newStepEventPublisher(ctx, factory, mqFactory))
	})
}

// TestNewStepEventPublisher_HappyPath 配置有效时拿到真实现，且 consumer_group 缺失不影响
// （只生产的 topic 不该被 RMQConf.Valid() 的 consumer_group 检查挡住）。
func TestNewStepEventPublisher_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := validStepEventCfg()
	require.False(t, cfg.Valid(), "fixture 必须是 Valid() 判否的（缺 consumer_group），否则这条用例没在测东西")

	p := mqmocks.NewMockIProducer(ctrl)
	p.EXPECT().Start().Return(nil)
	mqFactory := mqmocks.NewMockIFactory(ctrl)
	mqFactory.EXPECT().NewProducer(gomock.Any()).Return(p, nil)

	pub := newStepEventPublisher(context.Background(), newLoaderFactory(t, ctrl, cfg, nil), mqFactory)
	impl, ok := pub.(*stepEventPublisher)
	require.True(t, ok)
	assert.Equal(t, cfg.Topic, impl.topic)
}

func TestStepEventPublisher_PublishStepEvent(t *testing.T) {
	t.Run("sends a flat json body to the configured topic", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		var got *mq.Message
		p := mqmocks.NewMockIProducer(ctrl)
		p.EXPECT().Send(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, msg *mq.Message) (mq.SendResponse, error) {
				got = msg
				return mq.SendResponse{MessageID: "mid-1", Offset: 7}, nil
			}).Times(1)

		pub := &stepEventPublisher{topic: "t", p: p}
		pub.PublishStepEvent(context.Background(), stepEventFixture())

		require.NotNil(t, got)
		assert.Equal(t, "t", got.Topic)

		// 扁平：反序列化成 map 后每个 value 都不是对象 / 数组，Dorado→Hive 能直接映射成列。
		var flat map[string]any
		require.NoError(t, json.Unmarshal(got.Body, &flat))
		for k, v := range flat {
			switch v.(type) {
			case map[string]any, []any:
				t.Fatalf("field %q is nested; the Hive sync maps flat columns only", k)
			}
		}
		assert.Contains(t, flat, "sandbox_event_time_ms")
		assert.Contains(t, flat, "server_receive_time_ms")
	})

	t.Run("send failure is swallowed after retry", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		var calls atomic.Int32
		p := mqmocks.NewMockIProducer(ctrl)
		p.EXPECT().Send(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, _ *mq.Message) (mq.SendResponse, error) {
				calls.Add(1)
				return mq.SendResponse{}, errors.New("connection reset")
			}).MinTimes(1)

		pub := &stepEventPublisher{topic: "t", p: p}
		// 没有返回值可断言——这正是重点：签名里没有 error，调用点不可能把 MQ 故障
		// 变成上报接口的失败。
		assert.NotPanics(t, func() {
			pub.PublishStepEvent(context.Background(), stepEventFixture())
		})
		assert.GreaterOrEqual(t, calls.Load(), int32(1))
	})

	t.Run("nil receiver / nil producer / nil event are no-ops", func(t *testing.T) {
		var nilPub *stepEventPublisher
		assert.NotPanics(t, func() { nilPub.PublishStepEvent(context.Background(), stepEventFixture()) })

		assert.NotPanics(t, func() {
			(&stepEventPublisher{topic: "t"}).PublishStepEvent(context.Background(), stepEventFixture())
		})

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		p := mqmocks.NewMockIProducer(ctrl) // 无 EXPECT：nil event 不该触发发送
		assert.NotPanics(t, func() {
			(&stepEventPublisher{topic: "t", p: p}).PublishStepEvent(context.Background(), nil)
		})
	})

	t.Run("noop publisher never sends", func(t *testing.T) {
		assert.NotPanics(t, func() {
			(&noopStepEventPublisher{}).PublishStepEvent(context.Background(), stepEventFixture())
		})
	})
}
