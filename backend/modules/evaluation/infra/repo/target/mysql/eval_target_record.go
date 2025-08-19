// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/gorm_gen/query"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

//go:generate mockgen -destination=mocks/eval_target_record.go -package=mocks . EvalTargetRecordDAO
type EvalTargetRecordDAO interface {
	Create(ctx context.Context, record *model.TargetRecord) (id int64, err error)
	GetByIDAndSpaceID(ctx context.Context, recordID int64, spaceID int64) (*model.TargetRecord, error)
	ListByIDsAndSpaceID(ctx context.Context, recordIDs []int64, spaceID int64) ([]*model.TargetRecord, error)
}

type EvalTargetRecordDAOImpl struct {
	db    db.Provider
	query *query.Query
}

func NewEvalTargetRecordDAO(db db.Provider) EvalTargetRecordDAO {
	return &EvalTargetRecordDAOImpl{db: db, query: query.Use(db.NewSession(context.Background()))}
}

func (e *EvalTargetRecordDAOImpl) Create(ctx context.Context, record *model.TargetRecord) (id int64, err error) {
	// 写DB
	err = e.db.NewSession(ctx).Create(record).Error
	if err != nil {
		return 0, errorx.WrapByCode(err, errno.CommonMySqlErrorCode)
	}
	return record.ID, nil
}

func (e *EvalTargetRecordDAOImpl) GetByIDAndSpaceID(ctx context.Context, recordID int64, spaceID int64) (*model.TargetRecord, error) {
	q := e.query
	first, err := q.WithContext(ctx).TargetRecord.Where(q.TargetRecord.SpaceID.Eq(spaceID), q.TargetRecord.ID.Eq(recordID), q.TargetRecord.DeletedAt.IsNull()).First()

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.CommonMySqlErrorCode)
	}

	return first, nil
}

func (e *EvalTargetRecordDAOImpl) ListByIDsAndSpaceID(ctx context.Context, recordIDs []int64, spaceID int64) ([]*model.TargetRecord, error) {
	q := e.query
	if contexts.CtxWriteDB(ctx) {
		q = q.WriteDB()
	}
	records, err := q.WithContext(ctx).TargetRecord.Where(q.TargetRecord.ID.In(recordIDs...), q.TargetRecord.SpaceID.Eq(spaceID)).Find()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.CommonMySqlErrorCode)
	}
	return records, nil
}
