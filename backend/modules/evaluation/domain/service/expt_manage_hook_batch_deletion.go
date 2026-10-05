// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

// The canonical Manager retains its ordinary methods; this copy is only for Delete/MDelete.
func NewExptManagerForHookDeletion(base IExptManager, prepared repo.IHookDeletionRepo, scope string) (*ExptMangerImpl, error) {
	m, ok := base.(*ExptMangerImpl)
	if !ok || m == nil || m.finalization == nil || m.finalization.ExecutionScope != scope || missingManagerHookDependency(prepared) {
		return nil, entity.ErrHookStoreConflict
	}
	owner, ok := prepared.(interface{ HookDeletionScope() string })
	if !ok || owner.HookDeletionScope() != scope {
		return nil, entity.ErrHookStoreConflict
	}
	for _, v := range []any{m.finalization.Runs, m.finalization.Repository} {
		if missingManagerHookDependency(v) {
			return nil, entity.ErrHookExecutionUnsupported
		}
		if bound, ok := v.(repo.IHookBoundExecutionOwner); ok {
			key, _, hash := bound.HookExecutionBinding()
			if key != (entity.HookRunKey{}) || hash != "" {
				return nil, entity.ErrHookExecutionUnsupported
			}
		}
	}
	copy := *m
	copy.deletion = nil
	return NewExptManagerWithHookDeletion(&copy, prepared)
}
