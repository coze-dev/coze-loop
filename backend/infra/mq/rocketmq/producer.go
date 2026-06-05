// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rocketmq

import (
	"context"
	"time"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/primitive"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
)

type Producer struct {
	producer rocketmq.Producer
}

func (p *Producer) Start() error {
	return p.producer.Start()
}

func (p *Producer) Close() error {
	return p.producer.Shutdown()
}

func (p *Producer) Send(ctx context.Context, message *mq.Message) (mq.SendResponse, error) {
	msg := p.convertMessage(message)
	result, err := p.producer.SendSync(ctx, msg)
	if err != nil {
		return mq.SendResponse{}, err
	}
	return mq.SendResponse{
		MessageID: result.MsgID,
		Offset:    result.QueueOffset,
	}, nil
}

func (p *Producer) SendBatch(ctx context.Context, messages []*mq.Message) (mq.SendResponse, error) {
	msgs := make([]*primitive.Message, 0, len(messages))
	for _, message := range messages {
		msgs = append(msgs, p.convertMessage(message))
	}
	result, err := p.producer.SendSync(ctx, msgs...)
	if err != nil {
		return mq.SendResponse{}, err
	}
	return mq.SendResponse{
		MessageID: result.MsgID,
		Offset:    result.QueueOffset,
	}, nil
}

func (p *Producer) SendAsync(ctx context.Context, callback mq.AsyncSendCallback, message *mq.Message) error {
	msg := p.convertMessage(message)
	return p.producer.SendAsync(ctx, func(ctx context.Context, result *primitive.SendResult, err error) {
		var resp mq.SendResponse
		if result != nil {
			resp = mq.SendResponse{
				MessageID: result.MsgID,
				Offset:    result.QueueOffset,
			}
		}
		callback(ctx, resp, err)
	}, msg)
}

func (p *Producer) convertMessage(message *mq.Message) *primitive.Message {
	msg := primitive.NewMessage(message.Topic, message.Body).
		WithTag(message.Tag).
		WithShardingKey(message.PartitionKey)
	if len(message.Keys) > 0 {
		msg.WithKeys(message.Keys)
	}
	if len(message.Properties) > 0 {
		msg.WithProperties(message.Properties)
	}
	if message.DeferDuration > 0 {
		// rocketmq 老版本无法准确设置延迟时间，通过DelayLevel来实现
		msg.WithDelayTimeLevel(p.toDelayLevel(message.DeferDuration))
	}

	return msg
}

// rocketmq 默认delayLevel对应的秒数
var delayLevels = [...]int{
	1,    // level 1: 1s
	5,    // level 2: 5s
	10,   // level 3: 10s
	30,   // level 4: 30s
	60,   // level 5: 1m
	120,  // level 6: 2m
	180,  // level 7: 3m
	240,  // level 8: 4m
	300,  // level 9: 5m
	360,  // level 10: 6m
	420,  // level 11: 7m
	480,  // level 12: 8m
	540,  // level 13: 9m
	600,  // level 14: 10m
	1200, // level 15: 20m
	1800, // level 16: 30m
	3600, // level 17: 1h
	7200, // level 18: 2h
}

func (p *Producer) toDelayLevel(d time.Duration) int {
	seconds := int(d.Seconds())

	// 找到第一个大于等于目标值的级别
	for i, level := range delayLevels {
		if seconds <= level {
			return i + 1 // 级别从1开始
		}
	}

	// 超过最大级别返回最大值
	return len(delayLevels)
}
