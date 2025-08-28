// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"

	"github.com/bytedance/gg/gptr"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/query"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

//go:generate  mockgen -destination=mocks/expt_aggr_result.go  -package mocks . ExptAggrResultDAO
type ExptAggrResultDAO interface {
	GetExptAggrResult(ctx context.Context, experimentID int64, fieldType int32, fieldKey string, opts ...db.Option) (*model.ExptAggrResult, error)
	GetExptAggrResultByExperimentID(ctx context.Context, experimentID int64, opts ...db.Option) ([]*model.ExptAggrResult, error)
	BatchGetExptAggrResultByExperimentIDs(ctx context.Context, experimentIDs []int64, opts ...db.Option) ([]*model.ExptAggrResult, error)
	CreateExptAggrResult(ctx context.Context, exptAggrResult *model.ExptAggrResult, opts ...db.Option) error
	BatchCreateExptAggrResult(ctx context.Context, exptAggrResults []*model.ExptAggrResult, opts ...db.Option) error
	UpdateExptAggrResultByVersion(ctx context.Context, exptAggrResult *model.ExptAggrResult, taskVersion int64, opts ...db.Option) error
	UpdateAndGetLatestVersion(ctx context.Context, experimentID int64, fieldType int32, fieldKey string, opts ...db.Option) (int64, error)
	DeleteExptAggrResult(ctx context.Context, exptAggrResult *model.ExptAggrResult, opts ...db.Option) error
}

type ExptAggrResultDAOImpl struct {
	provider db.Provider
}

func NewExptAggrResultDAO(db db.Provider) ExptAggrResultDAO {
	return &ExptAggrResultDAOImpl{
		provider: db,
	}
}

const (
	calculateStatusIdle        int32 = 1
	calculateStatusCalculating int32 = 2
)

func (dao *ExptAggrResultDAOImpl) GetConn(ctx context.Context) *gorm.DB {
	return dao.provider.NewSession(ctx).Table(model.TableNameExptAggrResult)
}

func (dao *ExptAggrResultDAOImpl) GetExptAggrResult(ctx context.Context, experimentID int64, fieldType int32, fieldKey string, opts ...db.Option) (*model.ExptAggrResult, error) {
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptAggrResult
	ret, err := q.WithContext(ctx).Where(q.ExperimentID.Eq(experimentID), q.FieldType.Eq(fieldType), q.FieldKey.Eq(fieldKey)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("expt aggr result not found"))
		}
		return nil, err
	}

	return ret, nil
}

func (dao *ExptAggrResultDAOImpl) GetExptAggrResultByExperimentID(ctx context.Context, experimentID int64, opts ...db.Option) ([]*model.ExptAggrResult, error) {
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptAggrResult
	ret, err := q.WithContext(ctx).Where(q.ExperimentID.Eq(experimentID)).Find()
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (dao *ExptAggrResultDAOImpl) BatchGetExptAggrResultByExperimentIDs(ctx context.Context, experimentIDs []int64, opts ...db.Option) ([]*model.ExptAggrResult, error) {
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptAggrResult
	ret, err := q.WithContext(ctx).Where(q.ExperimentID.In(experimentIDs...)).Find()
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (dao *ExptAggrResultDAOImpl) CreateExptAggrResult(ctx context.Context, exptAggrResult *model.ExptAggrResult, opts ...db.Option) error {
	db := dao.provider.NewSession(ctx, opts...)
	err := db.WithContext(ctx).Model(&model.ExptAggrResult{}).Create(exptAggrResult).Error
	if err != nil {
		return err
	}

	return nil
}

func (dao *ExptAggrResultDAOImpl) BatchCreateExptAggrResult(ctx context.Context, exptAggrResults []*model.ExptAggrResult, opts ...db.Option) error {
	const batchSize = 10

	db := dao.provider.NewSession(ctx, opts...)
	err := db.WithContext(ctx).Model(&model.ExptAggrResult{}).Clauses(clause.OnConflict{
		DoUpdates: clause.AssignmentColumns([]string{"score", "aggr_result"}),
	}).CreateInBatches(exptAggrResults, batchSize).Error
	if err != nil {
		return err
	}

	return nil
}

func (dao *ExptAggrResultDAOImpl) UpdateExptAggrResultByVersion(ctx context.Context, exptAggrResult *model.ExptAggrResult, taskVersion int64, opts ...db.Option) error {
	exptAggrResult.Status = calculateStatusIdle
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptAggrResult
	_, err := q.WithContext(ctx).
		Where(q.ExperimentID.Eq(exptAggrResult.ExperimentID),
			q.FieldType.Eq(gptr.Indirect(exptAggrResult.FieldType)),
			q.FieldKey.Eq(exptAggrResult.FieldKey),
			q.Version.Eq(taskVersion),
			q.Status.Eq(calculateStatusCalculating)).
		Updates(exptAggrResult)
	if err != nil {
		return err
	}

	return nil
}

// UpdateAndGetLatestVersion 返回更新后的version, clause.Returning 需要开启conf.WithReturning = true.
func (dao *ExptAggrResultDAOImpl) UpdateAndGetLatestVersion(ctx context.Context, experimentID int64, fieldType int32, fieldKey string, opts ...db.Option) (int64, error) {
	po := &model.ExptAggrResult{}
	db := dao.provider.NewSession(ctx, opts...)
	err := db.Model(po).
		Clauses(clause.Returning{Columns: []clause.Column{{Name: "version"}}}).
		Where("experiment_id = ? AND field_type = ? AND field_key = ?", experimentID, fieldType, fieldKey).
		Updates(map[string]interface{}{
			"version": gorm.Expr("version + ?", 1),
			"status":  calculateStatusCalculating,
		}).Error
	if err != nil {
		return 0, err
	}

	return po.Version, nil
}

func (dao *ExptAggrResultDAOImpl) DeleteExptAggrResult(ctx context.Context, exptAggrResult *model.ExptAggrResult, opts ...db.Option) error {
	// 硬删除 可能删除后再创建
	po := &model.ExptAggrResult{}
	db := dao.provider.NewSession(ctx, opts...)
	err := db.Unscoped().Where("space_id = ? AND experiment_id = ?  AND field_type = ? AND field_key = ?", exptAggrResult.SpaceID, exptAggrResult.ExperimentID, exptAggrResult.FieldType, exptAggrResult.FieldKey).
		Delete(po).Error
	if err != nil {
		return err
	}

	return nil
}
