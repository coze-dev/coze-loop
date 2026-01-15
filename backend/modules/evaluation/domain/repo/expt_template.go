// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate  mockgen -destination  ./mocks/expt_template.go  --package mocks . IExptTemplateRepo
type IExptTemplateRepo interface {
	Create(ctx context.Context, template *entity.ExptTemplate, refs []*entity.ExptTemplateEvaluatorRef) error
	GetByID(ctx context.Context, id, spaceID int64) (*entity.ExptTemplate, error)
	GetByName(ctx context.Context, name string, spaceID int64) (*entity.ExptTemplate, bool, error)
	MGetByID(ctx context.Context, ids []int64, spaceID int64) ([]*entity.ExptTemplate, error)
	Update(ctx context.Context, template *entity.ExptTemplate) error
	UpdateFields(ctx context.Context, templateID int64, ufields map[string]any) error
	UpdateWithRefs(ctx context.Context, template *entity.ExptTemplate, refs []*entity.ExptTemplateEvaluatorRef) error
	Delete(ctx context.Context, id, spaceID int64) error
	List(ctx context.Context, page, size int32, filter *entity.ExptTemplateListFilter, orders []*entity.OrderBy, spaceID int64) ([]*entity.ExptTemplate, int64, error)
}
