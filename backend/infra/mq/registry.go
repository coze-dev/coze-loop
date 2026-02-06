// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mq

import (
	"context"
)

//go:generate mockgen -destination=mocks/registry.go -package=mocks . ConsumerRegistry,IConsumerWorker

type ConsumerRegistry interface {
	Register(worker []IConsumerWorker) ConsumerRegistry
	StartAll(ctx context.Context) error
	StopAll(ctx context.Context) error
}

type IConsumerWorker interface {
	ConsumerCfg(ctx context.Context) (*ConsumerConfig, error)
	IConsumerHandler
}
