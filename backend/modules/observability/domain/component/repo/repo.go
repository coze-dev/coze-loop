// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package repo

import "context"

type GetTraceRepoParam struct {
	WorkSpaceID string
}

//go:generate mockgen -destination=mocks/repo_provider.go -package=mocks . IRopeProvider
type IRopeProvider interface {
	GetTraceRepo(ctx context.Context, param GetTraceRepoParam) string
}
