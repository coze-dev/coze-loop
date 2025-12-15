// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package ck

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/ck"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/dao"
)

func NewAnnotationCkDaoImpl(db ck.Provider) (dao.IAnnotationDao, error) {
	return &AnnotationCkDaoImpl{
		db: db,
	}, nil
}

type AnnotationCkDaoImpl struct {
	db ck.Provider
}

func (a *AnnotationCkDaoImpl) Insert(ctx context.Context, params *dao.InsertAnnotationParam) error {
	return nil
}

func (a *AnnotationCkDaoImpl) Get(ctx context.Context, params *dao.GetAnnotationParam) (*dao.Annotation, error) {
	return nil, nil
}

func (a *AnnotationCkDaoImpl) List(ctx context.Context, params *dao.ListAnnotationsParam) ([]*dao.Annotation, error) {
	return nil, nil
}
