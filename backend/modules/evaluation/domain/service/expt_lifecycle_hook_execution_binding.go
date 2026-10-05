// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

// Decode outside SQL; each bound repository transaction rechecks the immutable hash.
func LoadHookExecutionInitializationBinding(ctx context.Context, runs repo.IHookRepo, codec hook.StorageCodec, key entity.HookRunKey, scope string) (*entity.HookExecutionInitializationBinding, error) {
	if ctx == nil || missingManagerHookDependency(runs) || missingManagerHookDependency(codec) {
		return nil, entity.ErrHookStoreCorrupt
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	run, err := runs.GetRun(ctx, key)
	if err != nil {
		return nil, err
	}
	if run == nil || run.State.Key != key || run.Snapshot.ExecutionScope != scope {
		return nil, entity.ErrHookStoreConflict
	}
	snapshot, err := codec.DecodeSnapshot(ctx, key, scope, run.Snapshot)
	if err != nil {
		return nil, entity.ErrHookStoreCorrupt
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return entity.NewHookExecutionInitializationBinding(run, snapshot)
}

func NewBoundHookFrozenExecutionInitializer(d HookFrozenExecutionInitializerDependencies, binding *entity.HookExecutionInitializationBinding) (hook.ExecutionInitializer, error) {
	source := binding.Input()
	if source.Execution == nil {
		return nil, entity.ErrHookExecutionUnsupported
	}
	base, err := NewHookFrozenExecutionInitializer(d)
	if err != nil {
		return nil, err
	}
	s := base.(*hookFrozenExecutionInitializer)
	s.boundKey, s.boundScope, s.boundHash = source.Key, source.ExecutionScope, source.SnapshotHash
	s.boundMode = source.Mode
	return s, nil
}
