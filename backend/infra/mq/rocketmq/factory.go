// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rocketmq

import (
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
)

type Factory struct{}

func NewFactory() mq.IFactory {
	return &Factory{}
}

func (f *Factory) NewProducer(config mq.ProducerConfig) (mq.IProducer, error) {
	if len(config.Addr) == 0 {
		return nil, errors.New("addr is empty")
	}
	opts := []producer.Option{
		producer.WithNsResolver(NewCustomResolver([]string{fmt.Sprintf("%s:%s", getRmqNamesrvDomain(), getRmqNamesrvPort())})),
		producer.WithRetry(config.RetryTimes),
	}
	if config.ProduceTimeout > 0 {
		opts = append(opts, producer.WithSendMsgTimeout(config.ProduceTimeout))
	}
	if config.RetryTimes > 0 {
		opts = append(opts, producer.WithRetry(config.RetryTimes))
	}
	if config.Compression != mq.CompressionNone {
		return nil, errors.New("compression not supported")
	}
	if config.ProducerGroup != nil {
		opts = append(opts, producer.WithGroupName(*config.ProducerGroup))
	}
	if getRmqNamesrvUser() != "" && getRmqNamesrvPassword() != "" {
		opts = append(opts, producer.WithCredentials(primitive.Credentials{
			AccessKey: getRmqNamesrvUser(),
			SecretKey: getRmqNamesrvPassword(),
		}))
	}

	p, err := rocketmq.NewProducer(opts...)
	if err != nil {
		return nil, err
	}
	return &Producer{producer: p}, nil
}

func (f *Factory) NewConsumer(config mq.ConsumerConfig) (mq.IConsumer, error) {
	if len(config.Addr) == 0 {
		return nil, errors.New("addr is empty")
	}
	if config.Topic == "" {
		return nil, errors.New("topic is empty")
	}
	if config.ConsumerGroup == "" {
		return nil, errors.New("consumer group is empty")
	}

	opts := []consumer.Option{
		consumer.WithNsResolver(NewCustomResolver([]string{fmt.Sprintf("%s:%s", getRmqNamesrvDomain(), getRmqNamesrvPort())})),
		consumer.WithGroupName(config.ConsumerGroup),
		consumer.WithConsumerOrder(config.Orderly),
	}
	if config.ConsumeGoroutineNums > 0 {
		opts = append(opts, consumer.WithConsumeGoroutineNums(config.ConsumeGoroutineNums))
	}
	if config.ConsumeTimeout > 0 {
		opts = append(opts, consumer.WithConsumeTimeout(config.ConsumeTimeout))
	}
	var selector *consumer.MessageSelector
	if config.TagExpression != "" {
		selector = &consumer.MessageSelector{
			Type:       consumer.TAG,
			Expression: config.TagExpression,
		}
	}

	c, err := rocketmq.NewPushConsumer(opts...)
	if err != nil {
		return nil, err
	}
	return &Consumer{
		consumer: c,
		topic:    config.Topic,
		selector: selector,
	}, nil
}

func NewCustomResolver(addrs []string) primitive.NsResolver {
	return &customResolver{addrs: addrs}
}

type customResolver struct {
	addrs []string
}

func (c *customResolver) Resolve() []string {
	ret := make([]string, len(c.addrs))
	for i, addr := range c.addrs {
		ret[i] = addr
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			continue
		}
		if net.ParseIP(host) != nil {
			continue
		}
		addrs, _ := net.LookupHost(host)
		if len(addrs) > 0 {
			ret[i] = net.JoinHostPort(addrs[0], port)
		}
	}
	return ret
}

func (c *customResolver) Description() string {
	return fmt.Sprintf("custom resolver: %v", c.addrs)
}

func getRmqNamesrvDomain() string {
	return os.Getenv("COZE_LOOP_RMQ_NAMESRV_DOMAIN")
}

func getRmqNamesrvPort() string {
	return os.Getenv("COZE_LOOP_RMQ_NAMESRV_PORT")
}

func getRmqNamesrvUser() string {
	return os.Getenv("COZE_LOOP_RMQ_NAMESRV_USER")
}

func getRmqNamesrvPassword() string {
	return os.Getenv("COZE_LOOP_RMQ_NAMESRV_PASSWORD")
}
