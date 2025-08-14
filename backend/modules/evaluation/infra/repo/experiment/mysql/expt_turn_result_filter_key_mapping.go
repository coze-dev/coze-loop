// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"

	"gorm.io/gorm/clause" // 导入 GORM 的 clause 包

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/query"
)

//go:generate  mockgen -destination=mocks/expt_turn_result_filter_key_mapping.go  -package mocks . IExptTurnResultFilterKeyMappingDAO
type IExptTurnResultFilterKeyMappingDAO interface {
	GetByExptID(ctx context.Context, spaceID, exptID int64, opts ...db.Option) ([]*model.ExptTurnResultFilterKeyMapping, error)
	Insert(ctx context.Context, exptTurnResultFilterKeyMappings []*model.ExptTurnResultFilterKeyMapping) error
}

type ExptTurnResultFilterKeyMappingDAOImpl struct {
	provider db.Provider
}

func NewExptTurnResultFilterKeyMappingDAO(db db.Provider) IExptTurnResultFilterKeyMappingDAO {
	return &ExptTurnResultFilterKeyMappingDAOImpl{
		provider: db,
	}
}

func (dao *ExptTurnResultFilterKeyMappingDAOImpl) GetByExptID(ctx context.Context, spaceID, exptID int64, opts ...db.Option) ([]*model.ExptTurnResultFilterKeyMapping, error) {
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptTurnResultFilterKeyMapping
	ret, err := q.WithContext(ctx).Where(
		q.SpaceID.Eq(spaceID),
		q.ExptID.Eq(exptID),
		// 使用 gorm.Expr 检查 deleted_at 字段是否为 NULL
		q.DeletedAt.IsNull(),
	).Find()
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (dao *ExptTurnResultFilterKeyMappingDAOImpl) Insert(ctx context.Context, exptTurnResultFilterKeyMappings []*model.ExptTurnResultFilterKeyMapping) error {
	// 避免变量名与导入包名冲突，修改变量名
	sessionDB := dao.provider.NewSession(ctx)
	q := query.Use(sessionDB).ExptTurnResultFilterKeyMapping

	// 使用 GORM 的 clause 实现 ON DUPLICATE KEY UPDATE 逻辑
	// 唯一键是 SpaceID+ExptID+FromField+FieldType
	// 当唯一键冲突时，更新 ToKey 和 CreatedBy 字段
	err := q.WithContext(ctx).Clauses(
		clause.OnConflict{
			Columns: []clause.Column{
				{Name: "space_id"},
				{Name: "expt_id"},
				{Name: "from_field"},
				{Name: "field_type"},
			},
			DoUpdates: clause.AssignmentColumns([]string{"to_key", "created_by"}),
		},
	).CreateInBatches(exptTurnResultFilterKeyMappings, len(exptTurnResultFilterKeyMappings))
	return err
}
