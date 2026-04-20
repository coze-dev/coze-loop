// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/gorm_gen/model"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"gorm.io/gorm"
)

//go:generate mockgen -destination=mocks/column_extract_config.go -package=mocks . IColumnExtractConfigDao
type IColumnExtractConfigDao interface {
	GetColumnExtractConfig(ctx context.Context, workspaceID int64, platformType, spanListType, agentName string) (*model.ObservabilityColumnExtractConfig, error)
	UpdateColumnExtractConfig(ctx context.Context, po *model.ObservabilityColumnExtractConfig) error
	CreateColumnExtractConfig(ctx context.Context, po *model.ObservabilityColumnExtractConfig) error
	ListColumnExtractConfigs(ctx context.Context, platformType, spanListType string) ([]*model.ObservabilityColumnExtractConfig, error)
}

func NewColumnExtractConfigDaoImpl(db db.Provider) IColumnExtractConfigDao {
	return &ColumnExtractConfigDaoImpl{
		dbMgr: db,
	}
}

type ColumnExtractConfigDaoImpl struct {
	dbMgr db.Provider
}

func (t ColumnExtractConfigDaoImpl) UpdateColumnExtractConfig(ctx context.Context, po *model.ObservabilityColumnExtractConfig) error {
	if err := t.dbMgr.NewSession(ctx).WithContext(ctx).Save(po).Error; err != nil {
		return errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
	}

	return nil
}

func (t ColumnExtractConfigDaoImpl) CreateColumnExtractConfig(ctx context.Context, po *model.ObservabilityColumnExtractConfig) error {
	if err := t.dbMgr.NewSession(ctx, db.WithMaster()).WithContext(ctx).Create(po).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("column extract config duplicate key"))
		} else {
			return errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
		}
	}

	return nil
}

func (t ColumnExtractConfigDaoImpl) GetColumnExtractConfig(ctx context.Context, workspaceID int64, platformType, spanListType, agentName string) (*model.ObservabilityColumnExtractConfig, error) {
	var po model.ObservabilityColumnExtractConfig
	err := t.dbMgr.NewSession(ctx, db.WithMaster()).WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Where("platform_type = ?", platformType).
		Where("span_list_type = ?", spanListType).
		Where("agent_name = ?", agentName).
		Where("is_deleted = ?", false).
		First(&po).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		} else {
			return nil, errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
		}
	}

	return &po, nil
}

func (t ColumnExtractConfigDaoImpl) ListColumnExtractConfigs(ctx context.Context, platformType, spanListType string) ([]*model.ObservabilityColumnExtractConfig, error) {
	var pos []*model.ObservabilityColumnExtractConfig
	err := t.dbMgr.NewSession(ctx).WithContext(ctx).
		Where("platform_type = ?", platformType).
		Where("span_list_type = ?", spanListType).
		Where("is_deleted = ?", false).
		Find(&pos).Error
	if err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
	}
	return pos, nil
}
