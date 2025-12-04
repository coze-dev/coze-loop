// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package storage

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/storage"
)

type TraceStorageProviderImpl struct {
}

func NewTraceStorageProvider() storage.IStorageProvider {
	return &TraceStorageProviderImpl{}
}

func (r *TraceStorageProviderImpl) GetTraceStorage(ctx context.Context, workspaceID string, tenants []string) storage.Storage {
	return storage.Storage{
		StorageName: "ck",
	}
}

func (r *TraceStorageProviderImpl) PrepareStorageForTask(ctx context.Context, workspaceID string, tenants []string) error {
	return nil
}
