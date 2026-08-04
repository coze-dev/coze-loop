// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package storage

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

type Storage struct {
	StorageName   string
	StorageConfig map[string]string
}

//go:generate mockgen -destination=mocks/storage_provider.go -package=mocks . IStorageProvider
type IStorageProvider interface {
	GetTraceStorage(ctx context.Context, workSpaceID string, tenants []string, scene loop_span.TraceScene) Storage
	PrepareStorageForTask(ctx context.Context, workspaceID string, tenants []string) error
}
