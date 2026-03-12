// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mq

import (
	"time"
)

//go:generate mockgen -destination=mocks/factory.go -package=mocks . IFactory
type IFactory interface {
	NewProducer(ProducerConfig) (IProducer, error)
	NewConsumer(ConsumerConfig) (IConsumer, error)
}

type ProducerConfig struct {
	// Name server address
	Addr []string
	// Timeout for producing one message
	ProduceTimeout time.Duration
	// Retry times for producing
	RetryTimes int
	// Use compression, default is no compression
	Compression CompressionCodec
	// Producer group name
	ProducerGroup *string

	// The maximum permitted size of a message
	MaxMessageBytes int
	// The best-effort frequency of flushes
	FlushFrequency time.Duration
	// How long to wait for the cluster to settle between retries
	RetryBackoff time.Duration

	AccessKey    *string
	AccessSecret *string
}

type ConsumerConfig struct {
	// Name server address
	Addr []string
	// Topic name
	Topic string
	// Consumer group name
	ConsumerGroup string
	// Whether to consume orderly
	Orderly bool
	// Consume specific tags, such as "tag" or "tag1 || tag2 || tag3"
	TagExpression string
	// Max number of messages consumed concurrently
	ConsumeGoroutineNums int
	// Timeout for consumer one message
	ConsumeTimeout time.Duration
	EnablePPE      *bool
	IsEnabled      *bool

	AccessKey    *string
	AccessSecret *string
}

type CompressionCodec int

const (
	// CompressionNone no compression
	CompressionNone CompressionCodec = iota
	// CompressionZSTD compression using ZSTD
	CompressionZSTD
	// CompressionSnappy compression using Snappy
	CompressionSnappy
)
