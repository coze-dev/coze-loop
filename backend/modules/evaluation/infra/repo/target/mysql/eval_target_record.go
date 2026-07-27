// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"

	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/gorm_gen/query"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

//go:generate mockgen -destination=mocks/eval_target_record.go -package=mocks . EvalTargetRecordDAO
type EvalTargetRecordDAO interface {
	Create(ctx context.Context, record *model.TargetRecord) (id int64, err error)
	Save(ctx context.Context, record *model.TargetRecord) error
	Update(ctx context.Context, record *model.TargetRecord) error
	GetByIDAndSpaceID(ctx context.Context, recordID, spaceID int64) (*model.TargetRecord, error)
	GetByRunIDItemIDTurnID(ctx context.Context, spaceID, runID, itemID, turnID int64) (*model.TargetRecord, error)
	ListByIDsAndSpaceID(ctx context.Context, recordIDs []int64, spaceID int64) ([]*model.TargetRecord, error)
	// AppendStep 在行锁 tx 中 append 一个 step 到 output_data.eval_target_steps。
	// mutate 返回新的 output_data JSON（[]byte）。传入 record 已加行锁，可安全修改。
	// record 不存在时 mutate 不会被调用，方法返回 nil（best-effort）。
	AppendStep(ctx context.Context, invokeID int64, mutate func(current []byte) ([]byte, error)) error
}

type EvalTargetRecordDAOImpl struct {
	db    db.Provider
	query *query.Query
}

func (e *EvalTargetRecordDAOImpl) Update(ctx context.Context, record *model.TargetRecord) error {
	if err := e.db.NewSession(ctx).Model(&model.TargetRecord{}).Where("id = ?", record.ID).Updates(record).Error; err != nil {
		return errorx.Wrapf(err, "TargetRecord update fail, id: %v, updated: %v", record.ID, json.Jsonify(record))
	}
	return nil
}

func NewEvalTargetRecordDAO(db db.Provider) EvalTargetRecordDAO {
	return &EvalTargetRecordDAOImpl{db: db, query: query.Use(db.NewSession(context.Background()))}
}

func (e *EvalTargetRecordDAOImpl) Save(ctx context.Context, record *model.TargetRecord) error {
	if err := e.db.NewSession(ctx).Save(record).Error; err != nil {
		return errorx.WrapByCode(err, errno.CommonMySqlErrorCode)
	}
	return nil
}

func (e *EvalTargetRecordDAOImpl) Create(ctx context.Context, record *model.TargetRecord) (id int64, err error) {
	// 写DB
	err = e.db.NewSession(ctx).Create(record).Error
	if err != nil {
		return 0, errorx.WrapByCode(err, errno.CommonMySqlErrorCode)
	}
	return record.ID, nil
}

func (e *EvalTargetRecordDAOImpl) GetByIDAndSpaceID(ctx context.Context, recordID, spaceID int64) (*model.TargetRecord, error) {
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

func (e *EvalTargetRecordDAOImpl) GetByRunIDItemIDTurnID(ctx context.Context, spaceID, runID, itemID, turnID int64) (*model.TargetRecord, error) {
	q := e.query
	first, err := q.WithContext(ctx).TargetRecord.Where(
		q.TargetRecord.SpaceID.Eq(spaceID),
		q.TargetRecord.ExperimentRunID.Eq(runID),
		q.TargetRecord.ItemID.Eq(itemID),
		q.TargetRecord.TurnID.Eq(turnID),
		q.TargetRecord.DeletedAt.IsNull(),
	).First()

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

// AppendStep 走行锁 tx 原子更新 output_data。
// 流程: BEGIN → SELECT output_data ... FOR UPDATE → mutate(current) → UPDATE output_data → COMMIT。
// row 不存在时静默返回 nil (best-effort, 与 metrics 打点容忍度一致)。
func (e *EvalTargetRecordDAOImpl) AppendStep(ctx context.Context, invokeID int64, mutate func(current []byte) ([]byte, error)) error {
	return e.db.NewSession(ctx).Transaction(func(tx *gorm.DB) error {
		var row model.TargetRecord
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", invokeID).
			Select("id", "output_data").
			Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// row 未预创建（异常场景，例如沙箱先于 asyncExecuteTarget 上报）→ 静默丢弃 step
			return nil
		}
		if err != nil {
			return errorx.WrapByCode(err, errno.CommonMySqlErrorCode)
		}

		var current []byte
		if row.OutputData != nil {
			current = *row.OutputData
		}
		newData, err := mutate(current)
		if err != nil {
			return err
		}
		if err := tx.Model(&model.TargetRecord{}).
			Where("id = ?", invokeID).
			UpdateColumn("output_data", newData).Error; err != nil {
			return errorx.WrapByCode(err, errno.CommonMySqlErrorCode)
		}
		return nil
	})
}
