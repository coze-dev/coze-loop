// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate  mockgen -destination  ./mocks/expt_template.go  --package mocks . IExptTemplateManager
type IExptTemplateManager interface {
	CheckName(ctx context.Context, name string, spaceID int64, session *entity.Session) (bool, error)
	Create(ctx context.Context, param *entity.CreateExptTemplateParam, session *entity.Session) (*entity.ExptTemplate, error)
	Get(ctx context.Context, templateID, spaceID int64, session *entity.Session) (*entity.ExptTemplate, error)
	MGet(ctx context.Context, templateIDs []int64, spaceID int64, session *entity.Session) ([]*entity.ExptTemplate, error)
	Update(ctx context.Context, template *entity.ExptTemplate, session *entity.Session) error
	Delete(ctx context.Context, templateID, spaceID int64, session *entity.Session) error
}
