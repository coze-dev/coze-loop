// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type hookWakePublisher struct {
	producer mq.IProducer
	topic    string
	timeout  time.Duration
}

// The caller owns the factory-created producer's Start/Close lifecycle. This
// constructor performs no I/O and is not part of the legacy publisher provider.
func NewHookWakePublisher(p mq.IProducer, topic string, timeout time.Duration) (hook.WakePublisher, error) {
	switch value := reflect.ValueOf(p); value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		if value.IsNil() {
			p = nil
		}
	}
	if p == nil || topic == "" || len(topic) > 255 || strings.ContainsFunc(topic, func(c rune) bool { return c < 33 || c > 126 }) || timeout <= 0 {
		return nil, errors.New("invalid hook wake publisher configuration")
	}
	return &hookWakePublisher{producer: p, topic: topic, timeout: timeout}, nil
}

func (p *hookWakePublisher) PublishWake(ctx context.Context, event entity.HookWakeEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	body, err := entity.EncodeHookWakeEvent(event)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	_, err = p.producer.Send(ctx, mq.NewMessage(p.topic, body).WithTag(hook.WakeMessageTag))
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		// Broker errors may contain credentials or raw message data.
		return errors.New("hook wake publish failed")
	}
	return nil
}
