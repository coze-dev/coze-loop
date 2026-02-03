// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"errors"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/goroutine"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type defaultConsumerRegistry struct {
	factory   mq.IFactory
	workers   []mq.IConsumerWorker
	consumers []mq.IConsumer
}

func NewConsumerRegistry(factory mq.IFactory) mq.ConsumerRegistry {
	return &defaultConsumerRegistry{factory: factory}
}

func (d *defaultConsumerRegistry) Register(worker []mq.IConsumerWorker) mq.ConsumerRegistry {
	d.workers = append(d.workers, worker...)
	return d
}

func (d *defaultConsumerRegistry) StartAll(ctx context.Context) error {
	d.consumers = nil
	for _, worker := range d.workers {
		cfg, err := worker.ConsumerCfg(ctx)
		if err != nil {
			return err
		}

		consumer, err := d.factory.NewConsumer(ptr.From(cfg))
		if err != nil {
			return errorx.Wrapf(err, "NewConsumer fail, cfg: %v", json.Jsonify(cfg))
		}

		consumer.RegisterHandler(newSafeConsumerWrapper(worker))
		if err := consumer.Start(); err != nil {
			return errorx.Wrapf(err, "StartConsumer fail, cfg: %v", json.Jsonify(cfg))
		}
		d.consumers = append(d.consumers, consumer)
	}
	return nil
}

func (d *defaultConsumerRegistry) StopAll(ctx context.Context) error {
	if len(d.consumers) == 0 {
		return nil
	}
	var errs []error
	for i := len(d.consumers) - 1; i >= 0; i-- {
		select {
		case <-ctx.Done():
			errs = append(errs, ctx.Err())
			return errors.Join(errs...)
		default:
			consumer := d.consumers[i]
			done := make(chan error, 1)
			go func(c mq.IConsumer) { done <- c.Close() }(consumer)
			select {
			case err := <-done:
				if err != nil {
					errs = append(errs, err)
				}
			case <-ctx.Done():
				errs = append(errs, ctx.Err())
				return errors.Join(errs...)
			}
		}
	}
	return errors.Join(errs...)
}

type safeConsumerHandlerDecorator struct {
	handler mq.IConsumerHandler
}

func (s *safeConsumerHandlerDecorator) HandleMessage(ctx context.Context, msg *mq.MessageExt) error {
	defer goroutine.Recovery(ctx)
	return s.handler.HandleMessage(ctx, msg)
}

func newSafeConsumerWrapper(h mq.IConsumerHandler) mq.IConsumerHandler {
	return &safeConsumerHandlerDecorator{handler: h}
}
