// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"context"
	"errors"
	"reflect"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type hookWakeConsumer struct {
	scope func(context.Context) (string, error)
	wake  hook.WakeHandler
}

// scope must resolve the current trusted deployment scope, never message
// properties or user headers. No subscriptions or workers are started here.
func NewHookWakeConsumer(scope func(context.Context) (string, error), wake hook.WakeHandler) (mq.IConsumerHandler, error) {
	switch value := reflect.ValueOf(wake); value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		if value.IsNil() {
			wake = nil
		}
	}
	if scope == nil || wake == nil {
		return nil, errors.New("invalid hook wake consumer configuration")
	}
	return &hookWakeConsumer{scope: scope, wake: wake}, nil
}

func (c *hookWakeConsumer) HandleMessage(ctx context.Context, msg *mq.MessageExt) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if msg == nil || msg.Tag != hook.WakeMessageTag {
		return entity.ErrInvalidHookWakeEvent
	}
	event, err := entity.DecodeHookWakeEvent(msg.Body)
	if err != nil {
		return err
	}
	scope, err := c.scope(ctx)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return errors.New("hook wake scope resolution failed")
	}
	if scope != event.ExecutionScope {
		return errors.New("hook wake scope mismatch")
	}
	// Duplicates and reordering only trigger another inspection. The handler must
	// acquire permission from DB; this component never spends an attempt budget.
	err = c.wake.Wake(ctx, event)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return errors.New("hook wake dispatch failed")
	}
	return nil
}
