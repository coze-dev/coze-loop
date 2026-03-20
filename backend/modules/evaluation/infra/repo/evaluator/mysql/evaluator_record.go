// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"sync"

	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
)

// EvaluatorRecordDAO 定义 EvaluatorRecord 的 Dao 接口
//
//go:generate mockgen -destination mocks/evaluator_record_mock.go -package=mocks . EvaluatorRecordDAO
type EvaluatorRecordDAO interface {
	CreateEvaluatorRecord(ctx context.Context, evaluatorRecord *model.EvaluatorRecord, opts ...db.Option) error
	UpdateEvaluatorRecord(ctx context.Context, evaluatorRecord *model.EvaluatorRecord, opts ...db.Option) error
	GetEvaluatorRecord(ctx context.Context, evaluatorRecordID int64, includeDeleted bool, opts ...db.Option) (*model.EvaluatorRecord, error)
	BatchGetEvaluatorRecord(ctx context.Context, evaluatorRecordIDs []int64, includeDeleted bool, opts ...db.Option) ([]*model.EvaluatorRecord, error)
}

var (
	evaluatorRecordDaoOnce      = sync.Once{}
	singletonEvaluatorRecordDao EvaluatorRecordDAO
)

type EvaluatorRecordDAOImpl struct {
	provider db.Provider
}

func NewEvaluatorRecordDAO(p db.Provider) EvaluatorRecordDAO {
	evaluatorRecordDaoOnce.Do(func() {
		singletonEvaluatorRecordDao = &EvaluatorRecordDAOImpl{
			provider: p,
		}
	})
	return singletonEvaluatorRecordDao
}

func (dao *EvaluatorRecordDAOImpl) CreateEvaluatorRecord(ctx context.Context, evaluatorRecord *model.EvaluatorRecord, opts ...db.Option) error {
	// 通过opts获取当前的db session实例
	dbsession := dao.provider.NewSession(ctx, opts...)

	return dbsession.WithContext(ctx).Create(evaluatorRecord).Error
}

func (dao *EvaluatorRecordDAOImpl) UpdateEvaluatorRecord(ctx context.Context, evaluatorRecord *model.EvaluatorRecord, opts ...db.Option) error {
	if evaluatorRecord == nil {
		// FIXME: errno
		// return errno.New(experiment.EvaluatorRecordNotFoundCode)
		return errors.New("evaluation.EvaluatorRecordNotFoundCode")
	}

	// 通过opts获取当前的db session实例
	dbsession := dao.provider.NewSession(ctx, opts...)

	return dbsession.WithContext(ctx).
		Model(&model.EvaluatorRecord{}).
		Where("id = ? AND deleted_at IS NULL", evaluatorRecord.ID).
		Save(evaluatorRecord).Error
}

func (dao *EvaluatorRecordDAOImpl) GetEvaluatorRecord(ctx context.Context, evaluatorRecordID int64, includeDeleted bool, opts ...db.Option) (*model.EvaluatorRecord, error) {
	po := &model.EvaluatorRecord{}

	// 通过opts获取当前的db session实例
	dbsession := dao.provider.NewSession(ctx, opts...)

	query := dbsession.WithContext(ctx).Where("id = ?", evaluatorRecordID)
	if includeDeleted {
		query = query.Unscoped() // 解除软删除过滤
	}
	err := query.First(po).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return po, nil
}

func (dao *EvaluatorRecordDAOImpl) BatchGetEvaluatorRecord(ctx context.Context, evaluatorRecordIDs []int64, includeDeleted bool, opts ...db.Option) ([]*model.EvaluatorRecord, error) {
	var pos []*model.EvaluatorRecord

	// 通过opts获取当前的db session实例
	dbsession := dao.provider.NewSession(ctx, opts...)

	query := dbsession.WithContext(ctx).Where("id IN (?)", evaluatorRecordIDs)
	if contexts.CtxWriteDB(ctx) {
		// 使用 FOR UPDATE 语句，强制使用写库
		query = query.Clauses(dbresolver.Write)
	}
	if includeDeleted {
		query = query.Unscoped() // 解除软删除过滤
	}
	err := query.Find(&pos).Error
	if err != nil {
		return nil, err
	}
	return pos, nil
}
