// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package storage

import "context"

//go:generate mockgen -destination=mocks/storage_provider.go -package=mocks . IStorageProvider
type IStorageProvider interface {
	GetTraceStorage(ctx context.Context, WorkSpaceID string) string
}
