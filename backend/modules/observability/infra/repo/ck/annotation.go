// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package ck

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/ck"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/ck/gorm_gen/model"
)

type InsertAnnotationParam struct {
	Table      string
	Annotation *model.ObservabilityAnnotation
}

type GetAnnotationParam struct {
	Tables    []string
	ID        string
	StartTime int64 // us
	EndTime   int64 // us
	Limit     int32
}

type ListAnnotationsParam struct {
	Tables          []string
	SpanIDs         []string
	StartTime       int64 // us
	EndTime         int64 // us
	DescByUpdatedAt bool
	Limit           int32
}

//go:generate mockgen -destination=mocks/annotation_dao.go -package=mocks . IAnnotationDao
type IAnnotationDao interface {
	Insert(context.Context, *InsertAnnotationParam) error
	Get(context.Context, *GetAnnotationParam) (*model.ObservabilityAnnotation, error)
	List(context.Context, *ListAnnotationsParam) ([]*model.ObservabilityAnnotation, error)
}

func NewAnnotationCkDaoImpl(db ck.Provider) (IAnnotationDao, error) {
	return &AnnotationCkDaoImpl{
		db: db,
	}, nil
}

type AnnotationCkDaoImpl struct {
	db ck.Provider
}

func (a *AnnotationCkDaoImpl) Insert(ctx context.Context, params *InsertAnnotationParam) error {
	return nil
}

func (a *AnnotationCkDaoImpl) Get(ctx context.Context, params *GetAnnotationParam) (*model.ObservabilityAnnotation, error) {
	return nil, nil
}

func (a *AnnotationCkDaoImpl) List(ctx context.Context, params *ListAnnotationsParam) ([]*model.ObservabilityAnnotation, error) {
	return nil, nil
}
