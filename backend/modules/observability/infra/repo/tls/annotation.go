// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package tls

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/ck"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/ck/gorm_gen/model"
)

func NewAnnotationTLSDaoImpl() (ck.IAnnotationDao, error) {
	return &AnnotationTLSDaoImpl{}, nil
}

type AnnotationTLSDaoImpl struct {
}

func (a *AnnotationTLSDaoImpl) Insert(context.Context, *ck.InsertAnnotationParam) error {
	return nil
}

func (a *AnnotationTLSDaoImpl) Get(context.Context, *ck.GetAnnotationParam) (*model.ObservabilityAnnotation, error) {
	return nil, nil
}

func (a *AnnotationTLSDaoImpl) List(context.Context, *ck.ListAnnotationsParam) ([]*model.ObservabilityAnnotation, error) {
	return nil, nil
}
