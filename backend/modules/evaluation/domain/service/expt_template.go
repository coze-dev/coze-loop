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
	Update(ctx context.Context, param *entity.UpdateExptTemplateParam, session *entity.Session) (*entity.ExptTemplate, error)
	UpdateMeta(ctx context.Context, param *entity.UpdateExptTemplateMetaParam, session *entity.Session) (*entity.ExptTemplate, error)
	// adjustCount: 实验数量的增量（创建实验时为 +1，删除实验时为 -1，状态变更时为 0）
	UpdateExptInfo(ctx context.Context, templateID, spaceID int64, exptID int64, exptStatus entity.ExptStatus, adjustCount int64) error
	Delete(ctx context.Context, templateID, spaceID int64, session *entity.Session) error
	List(ctx context.Context, page, pageSize int32, spaceID int64, filter *entity.ExptTemplateListFilter, orderBys []*entity.OrderBy, session *entity.Session) ([]*entity.ExptTemplate, int64, error)
	// ListOnline 查询在线实验模板（通过 Task 查询）
	ListOnline(ctx context.Context, page, pageSize int32, spaceID int64, filter *entity.ExptTemplateListFilter, orderBys []*entity.OrderBy, session *entity.Session) ([]*entity.ExptTemplate, int64, error)
}
