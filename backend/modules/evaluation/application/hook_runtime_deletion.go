// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	experiment "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
)

// ForDeletion prepares the entire request before SQL mutation; ForRun still owns later cleanup.
func (f *HookRuntimeExecutionFactory) ForDeletion(ctx context.Context, ids []int64, spaceID int64) (*service.ExptMangerImpl, error) {
	if f == nil {
		return nil, entity.ErrHookExecutionUnsupported
	}
	d := f.deps
	prepared, err := experiment.PrepareHookDeletion(ctx, d.DB, d.Runs, d.Codec, ids, spaceID, d.Scope)
	if err != nil {
		return nil, err
	}
	return service.NewExptManagerForHookDeletion(d.Manager, prepared, d.Scope)
}
