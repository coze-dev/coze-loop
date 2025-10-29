// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package storage

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/storage"
)

type TraceStorageProviderImpl struct {
	traceConfig config.ITraceConfig
}

func NewTraceStorageProvider(traceConfig config.ITraceConfig) storage.IStorageProvider {
	return &TraceStorageProviderImpl{
		traceConfig: traceConfig,
	}
}

func (r *TraceStorageProviderImpl) GetTraceStorage(ctx context.Context, param storage.GetTraceStorageParam) string {
	return "ck"
}
