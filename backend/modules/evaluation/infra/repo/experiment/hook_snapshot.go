// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"database/sql"
	"reflect"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// ReadSnapshot runs the complete callback once in one RR, read-only snapshot.
// Implementations must not retain the callback or transaction after returning.
type HookSnapshotExecutor interface {
	ReadSnapshot(context.Context, func(*gorm.DB) error) error
}

func WithHookSnapshotExecutor(base db.Provider, executor HookSnapshotExecutor) (db.Provider, error) {
	if nilHookSnapshotDependency(base) || nilHookSnapshotDependency(executor) {
		return nil, entity.ErrHookStoreCorrupt
	}
	return &hookSnapshotProvider{Provider: base, executor: executor}, nil
}

type hookSnapshotProvider struct {
	db.Provider
	executor HookSnapshotExecutor
}

func (p *hookSnapshotProvider) ReadSnapshot(ctx context.Context, fn func(*gorm.DB) error) error {
	if p == nil || ctx == nil || fn == nil || nilHookSnapshotDependency(p.executor) {
		return entity.ErrHookStoreCorrupt
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := p.executor.ReadSnapshot(ctx, fn); err != nil {
		return err
	}
	return ctx.Err()
}

func readHookSnapshot(ctx context.Context, p db.Provider, fn func(*gorm.DB) error) error {
	if ctx == nil || fn == nil || nilHookSnapshotDependency(p) {
		return entity.ErrHookStoreCorrupt
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	read := func(tx *gorm.DB) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if tx == nil {
			return entity.ErrHookStoreCorrupt
		}
		return fn(tx.Session(&gorm.Session{Logger: logger.Discard}))
	}
	var err error
	if executor, ok := p.(HookSnapshotExecutor); ok {
		// An injected executor owns the transaction; its failures must never fall back.
		err = executor.ReadSnapshot(ctx, read)
	} else {
		err = p.NewSession(ctx, db.WithMaster()).Session(&gorm.Session{Logger: logger.Discard}).Transaction(read, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	}
	if err != nil {
		return err
	}
	return ctx.Err()
}

func nilHookSnapshotDependency(v any) bool {
	if v == nil {
		return true
	}
	r := reflect.ValueOf(v)
	switch r.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return r.IsNil()
	}
	return false
}
