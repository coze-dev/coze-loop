// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package repo

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/repo"
)

type RepoProviderImpl struct {
	traceConfig config.ITraceConfig
}

func NewRepoProvider(traceConfig config.ITraceConfig) repo.IRopeProvider {
	return &RepoProviderImpl{
		traceConfig: traceConfig,
	}
}

func (r *RepoProviderImpl) GetTraceRepo(ctx context.Context, param repo.GetTraceRepoParam) string {
	return "ck"
}
