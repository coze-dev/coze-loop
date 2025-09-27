// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

//go:generate mockgen -destination=mocks/eval_target_version.go -package=mocks . EvalTargetVersionDAO
type EvalTargetVersionDAO interface {
	CreateEvalTargetVersion(ctx context.Context, target *model.TargetVersion, opts ...db.Option) (err error)
	GetEvalTargetVersion(ctx context.Context, spaceID, versionID int64, opts ...db.Option) (version *model.TargetVersion, err error)
	GetEvalTargetVersionByTarget(ctx context.Context, spaceID, targetID int64, sourceTargetVersion string, opts ...db.Option) (version *model.TargetVersion, err error)
	BatchGetEvalTargetVersion(ctx context.Context, spaceID int64, versionIDs []int64, opts ...db.Option) (versions []*model.TargetVersion, err error)
}

type EvalTargetVersionDAOImpl struct {
	provider db.Provider
}

var (
	evalTargetVersionDaoOnce = sync.Once{}
	evalTargetVersionDao     EvalTargetVersionDAO
)

func NewEvalTargetVersionDAO(provider db.Provider) EvalTargetVersionDAO {
	evalTargetVersionDaoOnce.Do(func() {
		evalTargetVersionDao = &EvalTargetVersionDAOImpl{
			provider: provider,
		}
	})
	return evalTargetVersionDao
}

func (e *EvalTargetVersionDAOImpl) CreateEvalTargetVersion(ctx context.Context, target *model.TargetVersion, opts ...db.Option) (err error) {
	dbSession := e.provider.NewSession(ctx, opts...)
	err = dbSession.WithContext(ctx).Create(target).Error
	if err != nil {
		return errorx.WrapByCode(err, errno.CommonMySqlErrorCode)
	}
	return nil
}

func (e *EvalTargetVersionDAOImpl) GetEvalTargetVersion(ctx context.Context, spaceID, versionID int64, opts ...db.Option) (version *model.TargetVersion, err error) {
	dbSession := e.provider.NewSession(ctx, opts...)
	po := &model.TargetVersion{}
	err = dbSession.Where("id = ?", versionID).
		Where("space_id = ?", spaceID).First(po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.CommonMySqlErrorCode)
	}
	return po, nil
}

func (e *EvalTargetVersionDAOImpl) GetEvalTargetVersionByTarget(ctx context.Context, spaceID, targetID int64, sourceTargetVersion string, opts ...db.Option) (version *model.TargetVersion, err error) {
	dbSession := e.provider.NewSession(ctx, opts...)
	po := &model.TargetVersion{}
	err = dbSession.Where("space_id = ?", spaceID).
		Where("target_id = ?", targetID).
		Where("source_target_version = ?", sourceTargetVersion).
		First(po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.CommonMySqlErrorCode)
	}
	return po, nil
}

func (e *EvalTargetVersionDAOImpl) BatchGetEvalTargetVersion(ctx context.Context, spaceID int64, versionIDs []int64, opts ...db.Option) (versions []*model.TargetVersion, err error) {
	dbSession := e.provider.NewSession(ctx, opts...)
	poList := make([]*model.TargetVersion, 0)
	err = dbSession.
		Where("space_id = ?", spaceID).
		Where("id in (?)", versionIDs).
		Find(&poList).Error
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.CommonMySqlErrorCode)
	}
	return poList, nil
}
