// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"context"
	"os"
	"sync"

	"github.com/bytedance/gg/gptr"

	infrabackoff "github.com/coze-dev/coze-loop/backend/infra/backoff"
	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/mq/rocket"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

var (
	stepEventPublisherSingleton events.StepEventPublisher
	stepEventPublisherOnce      sync.Once
)

// NewStepEventPublisher 构造阶段事件明细的 MQ 生产者。
//
// ⚠️ **不返回 error，任何构造失败都降级为 no-op**。这与同包的 newExptEventPublisher 相反：那个
// 在 topic 配置无效时 `return nil, fmt.Errorf(...)`，把整个 publisher 的构造带崩，进而让服务
// 起不来——对实验调度事件那是对的，消息丢了实验会卡住。
//
// 埋点不是这样：一个埋点 topic 配错了就让整个评测服务起不来，是本末倒置。所以这里所有失败路径
// （配置读不出来 / addr 或 topic 为空 / producer 创建失败 / Start 失败）都只 warn 并返回 no-op。
func NewStepEventPublisher(ctx context.Context, cfgFactory conf.IConfigLoaderFactory, mqFactory mq.IFactory) events.StepEventPublisher {
	stepEventPublisherOnce.Do(func() {
		stepEventPublisherSingleton = newStepEventPublisher(ctx, cfgFactory, mqFactory)
	})
	return stepEventPublisherSingleton
}

func newStepEventPublisher(ctx context.Context, cfgFactory conf.IConfigLoaderFactory, mqFactory mq.IFactory) events.StepEventPublisher {
	const key = rocket.ExptSandboxStepEventRMQKey

	if cfgFactory == nil || mqFactory == nil {
		logs.CtxWarn(ctx, "[step_event_pub] init: nil factory, fallback to no-op, producer_key: %v", key)
		return &noopStepEventPublisher{}
	}

	loader, err := cfgFactory.NewConfigLoader(consts.EvaluationConfigFileName)
	if err != nil {
		logs.CtxWarn(ctx, "[step_event_pub] init: new config loader fail, fallback to no-op, producer_key: %v, err: %v", key, err)
		return &noopStepEventPublisher{}
	}

	var cfg rocket.RMQConf
	if err := loader.UnmarshalKey(ctx, key, &cfg); err != nil {
		logs.CtxWarn(ctx, "[step_event_pub] init: unmarshal config fail, fallback to no-op, producer_key: %v, err: %v", key, err)
		return &noopStepEventPublisher{}
	}

	// DisableProduce 是白送的止血开关：MQ 出问题拖累上报接口时，在配置（TCC）上一键关掉生产，
	// 不用发版。沿用既有 producer 的做法。
	if gptr.Indirect(cfg.DisableProduce) {
		logs.CtxInfo(ctx, "[step_event_pub] init: produce disabled by config, fallback to no-op, producer_key: %v", key)
		return &noopStepEventPublisher{}
	}

	// 不用 cfg.Valid()：它还要求 consumer_group 非空，而本 topic 是**只生产**的（消费方是数仓侧
	// 的同步任务，不在本仓）。用 Valid() 会因为一个与生产无关的字段把可用配置判成不可用。
	if cfg.Addr == "" || cfg.Topic == "" {
		logs.CtxWarn(ctx, "[step_event_pub] init: invalid addr/topic, fallback to no-op, producer_key: %v, conf: %v", key, json.Jsonify(cfg))
		return &noopStepEventPublisher{}
	}

	pcfg := cfg.ToProducerCfg()
	p, err := mqFactory.NewProducer(pcfg)
	if err != nil {
		logs.CtxWarn(ctx, "[step_event_pub] init: new producer fail, fallback to no-op, producer_key: %v, cfg: %v, err: %v", key, json.Jsonify(pcfg), err)
		return &noopStepEventPublisher{}
	}
	if err := p.Start(); err != nil {
		logs.CtxWarn(ctx, "[step_event_pub] init: start producer fail, fallback to no-op, producer_key: %v, cfg: %v, err: %v", key, json.Jsonify(pcfg), err)
		return &noopStepEventPublisher{}
	}

	logs.CtxInfo(ctx, "[step_event_pub] init: ok, producer_key: %v, topic: %v", key, cfg.Topic)
	return &stepEventPublisher{topic: cfg.Topic, p: p}
}

type stepEventPublisher struct {
	topic string
	p     mq.IProducer
}

// PublishStepEvent 发一条阶段事件明细。
//
// ⚠️ **任何失败只 warn，不返回、不 panic**。接口签名里就没有 error（见
// events.StepEventPublisher 的注释）：埋点丢一条不影响评测的正确性，但让 MQ 抖动变成上报接口
// 5xx 就是用观测通道搞挂被观测的链路。
//
// 已知代价（spec D5，不是遗漏）：丢失完全静默，无重试队列无缓冲无落盘。排查「Hive 里少数据」
// 只能靠下面这两条日志锚点——成功一条、失败一条，都带 topic 与 message id，用来区分「没发出去」
// 还是「没消费到」。
func (s *stepEventPublisher) PublishStepEvent(ctx context.Context, event *entity.SandboxStepEventMessage) {
	if s == nil || s.p == nil || event == nil {
		return
	}

	body, err := json.Marshal(event)
	if err != nil {
		logs.CtxWarn(ctx, "[step_event_pub] marshal fail, dropping event, topic: %v, step_name: %v, invoke_id: %v, err: %v",
			s.topic, event.StepName, event.InvokeID, err)
		return
	}

	msg := mq.NewMessage(s.topic, body)
	if env := os.Getenv(XttEnv); env != "" {
		ctx = context.WithValue(ctx, CtxKeyEnv, env) //nolint:staticcheck
	}

	var resp mq.SendResponse
	sendErr := infrabackoff.RetryThreeSeconds(ctx, func() error {
		var err error
		resp, err = s.p.Send(ctx, msg)
		return err
	})
	if sendErr != nil {
		logs.CtxWarn(ctx, "[step_event_pub] send fail, dropping event, topic: %v, event_type: %v, step_name: %v, invoke_id: %v, err: %v",
			s.topic, event.EventType, event.StepName, event.InvokeID, sendErr)
		return
	}

	logs.CtxInfo(ctx, "[step_event_pub] send success, topic: %v, message_id: %v, offset: %v, event_type: %v, step_name: %v, invoke_id: %v",
		s.topic, resp.MessageID, resp.Offset, event.EventType, event.StepName, event.InvokeID)
}

type noopStepEventPublisher struct{}

func (n *noopStepEventPublisher) PublishStepEvent(_ context.Context, _ *entity.SandboxStepEventMessage) {
}
