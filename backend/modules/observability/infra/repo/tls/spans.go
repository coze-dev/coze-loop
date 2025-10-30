// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package tls

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/ck"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/ck/gorm_gen/model"
)

const (
	TraceStorageTypeTLS = "tls"
)

type SpansTLSDaoImpl struct {
}

func NewSpansTLSDaoImpl() (ck.ISpansDao, error) {
	return &SpansTLSDaoImpl{}, nil
}

func (s *SpansTLSDaoImpl) Insert(ctx context.Context, param *ck.InsertParam) error {
	return nil
}

func (s *SpansTLSDaoImpl) Get(context.Context, *ck.QueryParam) ([]*model.ObservabilitySpan, error) {
	return nil, nil
}
func (s *SpansTLSDaoImpl) GetMetrics(ctx context.Context, param *ck.GetMetricsParam) ([]map[string]any, error) {
	return nil, nil
}
