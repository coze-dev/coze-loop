// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

func NewBoundHookRuntimeRepositories(p db.Provider, binding *entity.HookExecutionInitializationBinding) (repo.HookBoundRuntimeRepositories, error) {
	base, err := NewBoundHookExecutionInitializationRepo(p, binding)
	if err != nil {
		return repo.HookBoundRuntimeRepositories{}, err
	}
	r := base.(*hookRunRepo)
	b := r.executionBinding
	scope := func(context.Context) (string, error) { return b.source.ExecutionScope, nil }
	return repo.HookBoundRuntimeRepositories{
		Runs: r, Initialization: r, Consumer: r,
		Finalization: &hookFinalizationRepo{provider: p, binding: b},
		Progress:     &hookTurnProgressRepo{provider: p, executionScope: scope, binding: b},
		Gate:         &hookGateRepo{provider: p, executionScope: scope, execution: true, binding: b},
		Source:       NewHookItemSourceRepo(p),
	}, nil
}

func boundRuntimeIdentity(b *boundHookExecution) (entity.HookRunKey, string, string) {
	if b == nil {
		return entity.HookRunKey{}, "", ""
	}
	return b.source.Key, b.source.ExecutionScope, b.source.SnapshotHash
}
func (r *hookRunRepo) HookExecutionBinding() (entity.HookRunKey, string, string) {
	return boundRuntimeIdentity(r.executionBinding)
}
func (r *hookFinalizationRepo) HookExecutionBinding() (entity.HookRunKey, string, string) {
	return boundRuntimeIdentity(r.binding)
}
func (r *hookTurnProgressRepo) HookExecutionBinding() (entity.HookRunKey, string, string) {
	return boundRuntimeIdentity(r.binding)
}
func (r *hookGateRepo) HookExecutionBinding() (entity.HookRunKey, string, string) {
	return boundRuntimeIdentity(r.binding)
}
