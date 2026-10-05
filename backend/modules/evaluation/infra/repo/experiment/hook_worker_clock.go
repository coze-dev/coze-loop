// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	hook "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
)

var errHookWorkerClock = errors.New("hook worker database clock unavailable")

type hookWorkerClock struct{ provider db.Provider }

func NewHookWorkerClock(p db.Provider) (hook.WorkerClock, error) {
	if p == nil {
		return nil, errHookWorkerClock
	}
	v := reflect.ValueOf(p)
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Func, reflect.Chan, reflect.Map, reflect.Slice:
		if v.IsNil() {
			return nil, errHookWorkerClock
		}
	}
	return &hookWorkerClock{provider: p}, nil
}

func (c *hookWorkerClock) Now(ctx context.Context) (time.Time, error) {
	if ctx == nil {
		return time.Time{}, errHookWorkerClock
	}
	if ctx.Err() != nil {
		return time.Time{}, ctx.Err()
	}
	// Read-only primary sampling, deliberately separate from lock-scoped attempt clocks.
	var now time.Time
	session := c.provider.NewSession(ctx, db.WithMaster())
	if session == nil {
		return time.Time{}, errHookWorkerClock
	}
	if err := session.Raw("SELECT CURRENT_TIMESTAMP(3)").Row().Scan(&now); err != nil {
		return time.Time{}, errHookWorkerClock
	}
	if ctx.Err() != nil {
		return time.Time{}, ctx.Err()
	}
	return now, nil
}
