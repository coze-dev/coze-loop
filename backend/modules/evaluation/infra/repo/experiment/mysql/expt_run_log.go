// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"time"

	"github.com/bytedance/gg/gptr"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/query"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

//go:generate  mockgen -destination=mocks/expt_run_log.go  -package mocks . IExptRunLogDAO
type IExptRunLogDAO interface {
	Create(ctx context.Context, exptRunLog *model.ExptRunLog, opts ...db.Option) error
	Save(ctx context.Context, exptRunLog *model.ExptRunLog, opts ...db.Option) error
	Update(ctx context.Context, exptID, exptRunID, spaceID int64, ufields map[string]any, opts ...db.Option) error
	Get(ctx context.Context, exptID, exptRunID int64, opts ...db.Option) (*model.ExptRunLog, error)
	AppendRunItemIDs(ctx context.Context, exptID, exptRunID, spaceID int64, itemIDs []int64) error
	UpdateRunStats(ctx context.Context, exptID, exptRunID, spaceID int64, fields map[string]any, stats *model.ExptStats) error
}

// IExptRunLogLatestRunDAO is the narrow CAS write used by completion paths.
// Kept separate from IExptRunLogDAO so existing implementations and mocks stay valid.
type IExptRunLogLatestRunDAO interface {
	UpdateIfLatestRun(ctx context.Context, exptID, exptRunID, spaceID int64, ufields map[string]any, opts ...db.Option) (bool, error)
}

func (dao *ExptRunLogDAOImpl) UpdateRunStats(ctx context.Context, exptID, exptRunID, spaceID int64, fields map[string]any, stats *model.ExptStats) error {
	return dao.provider.Transaction(ctx, func(tx *gorm.DB) error {
		opts := []db.Option{db.WithTransaction(tx), db.WithMaster(), db.WithSelectForUpdate()}
		expt, err := NewExptDAO(dao.provider).GetByID(ctx, exptID, opts...)
		if err != nil {
			return err
		}
		if expt.SpaceID != spaceID || expt.LatestRunID != exptRunID ||
			(expt.Status != int32(entity.ExptStatus_Pending) && expt.Status != int32(entity.ExptStatus_Processing)) {
			return nil
		}
		run, err := dao.Get(ctx, exptID, exptRunID, opts...)
		if err != nil {
			return err
		}
		if run.SpaceID != spaceID || (gptr.Indirect(run.Status) != int64(entity.ExptStatus_Pending) && gptr.Indirect(run.Status) != int64(entity.ExptStatus_Processing)) {
			return nil
		}
		if err := NewExptStatsDAO(dao.provider).UpdateByExptID(ctx, exptID, spaceID, stats, opts...); err != nil {
			return err
		}
		return dao.Update(ctx, exptID, exptRunID, spaceID, fields, opts...)
	}, db.WithMaster())
}

type ExptRunLogDAOImpl struct {
	provider db.Provider
}

func NewExptRunLogDAO(db db.Provider) IExptRunLogDAO {
	return &ExptRunLogDAOImpl{
		provider: db,
	}
}

func (dao *ExptRunLogDAOImpl) Get(ctx context.Context, exptID, exptRunID int64, opts ...db.Option) (*model.ExptRunLog, error) {
	var exptRunLog model.ExptRunLog
	db := dao.provider.NewSession(ctx, opts...)
	if contexts.CtxWriteDB(ctx) {
		db = db.Clauses(dbresolver.Write)
	}
	cond := db.WithContext(ctx).Where("id = ?", exptRunID)
	// exptID 未知时只按 run id 查：destroy 链路要按 run id 反查 expt（resolveSandboxTaskIDByRunID 传 0），
	// 这里再要求 expt_id 相等会让查询恒空、taskID 变空串，双沙箱的 -orch 销毁请求被丢弃。
	if exptID > 0 {
		cond = db.WithContext(ctx).Where("id = ? AND expt_id = ? AND expt_run_id = ?", exptRunID, exptID, exptRunID)
	}
	if err := cond.First(&exptRunLog).Error; err != nil {
		return nil, errorx.Wrapf(err, "mget expt fail, expt_id: %v, expt_run_id: %v", exptID, exptRunID)
	}
	return &exptRunLog, nil
}

func (dao *ExptRunLogDAOImpl) Create(ctx context.Context, exptRunLog *model.ExptRunLog, opts ...db.Option) error {
	db := dao.provider.NewSession(ctx, opts...)
	if err := db.WithContext(ctx).Create(exptRunLog).Error; err != nil {
		return errorx.Wrapf(err, "create expt_run_log fail, model: %v", json.Jsonify(exptRunLog))
	}
	return nil
}

func (dao *ExptRunLogDAOImpl) Save(ctx context.Context, exptRunLog *model.ExptRunLog, opts ...db.Option) error {
	// Existing callers may hold a stale snapshot; immutable metadata is never saved.
	return dao.Update(ctx, exptRunLog.ExptID, exptRunLog.ExptRunID, exptRunLog.SpaceID, map[string]any{
		"status": exptRunLog.Status, "status_message": exptRunLog.StatusMessage,
		"pending_cnt": exptRunLog.PendingCnt, "processing_cnt": exptRunLog.ProcessingCnt,
		"success_cnt": exptRunLog.SuccessCnt, "fail_cnt": exptRunLog.FailCnt,
		"terminated_cnt": exptRunLog.TerminatedCnt, "credit_cost": exptRunLog.CreditCost,
		"token_cost": exptRunLog.TokenCost, "updated_at": time.Now(),
	}, opts...)
}

func (dao *ExptRunLogDAOImpl) AppendRunItemIDs(ctx context.Context, exptID, exptRunID, spaceID int64, itemIDs []int64) error {
	return dao.provider.Transaction(ctx, func(tx *gorm.DB) error {
		opts := []db.Option{db.WithTransaction(tx), db.WithMaster(), db.WithSelectForUpdate()}
		var row model.ExptRunLog
		if err := dao.provider.NewSession(ctx, opts...).
			Where("space_id = ? AND expt_id = ? AND expt_run_id = ?", spaceID, exptID, exptRunID).First(&row).Error; err != nil {
			return err
		}
		var items []entity.ExptRunLogItems
		if raw := gptr.Indirect(row.ItemIds); len(raw) > 0 {
			if err := json.Unmarshal(raw, &items); err != nil {
				return errorx.Wrapf(err, "parse current run items")
			}
		}
		run := &entity.ExptRunLog{ItemIds: items}
		if err := run.AppendItemIDs(itemIDs); err != nil {
			return err
		}
		raw, err := entity.EncodeExptRunLogItems(run.ItemIds)
		if err != nil {
			return err
		}
		return dao.Update(ctx, exptID, exptRunID, spaceID, map[string]any{"item_ids": raw, "updated_at": time.Now()}, opts...)
	}, db.WithMaster())
}

func (dao *ExptRunLogDAOImpl) Update(ctx context.Context, exptID, exptRunID, spaceID int64, ufields map[string]any, opts ...db.Option) error {
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptRunLog
	_, err := q.WithContext(ctx).
		Where(q.SpaceID.Eq(spaceID)).
		Where(q.ExptID.Eq(exptID)).
		Where(q.ExptRunID.Eq(exptRunID)).
		UpdateColumns(ufields)
	if err != nil {
		return errorx.Wrapf(err, "update expt_run_log fail, expt_id: %v, expt_run_id: %v, ufields: %v", exptID, exptRunID, ufields)
	}
	logs.CtxInfo(ctx, "update expt_run_log success, expt_id: %v, expt_run_id: %v, ufields: %v", exptID, exptRunID, ufields)
	return nil
}

func (dao *ExptRunLogDAOImpl) UpdateIfLatestRun(ctx context.Context, exptID, exptRunID, spaceID int64, ufields map[string]any, opts ...db.Option) (bool, error) {
	owned := func(q *gorm.DB) *gorm.DB {
		return q.Where("space_id = ? AND expt_id = ? AND expt_run_id = ?", spaceID, exptID, exptRunID).
			Where("EXISTS (SELECT 1 FROM experiment WHERE id = ? AND space_id = ? AND latest_run_id = ?)", exptID, spaceID, exptRunID)
	}
	result := owned(dao.provider.NewSession(ctx, opts...).Model(&model.ExptRunLog{})).UpdateColumns(ufields)
	if result.Error != nil {
		return false, errorx.Wrapf(result.Error, "update expt_run_log if latest run fail, expt_id: %v, expt_run_id: %v, ufields: %v", exptID, exptRunID, ufields)
	}
	if result.RowsAffected > 0 {
		return true, nil
	}
	// MySQL reports zero affected rows for an idempotent same-values rewrite.
	// Distinguish that from a replacement run owning the experiment.
	var count int64
	if err := owned(dao.provider.NewSession(ctx, opts...).Clauses(dbresolver.Write).Model(&model.ExptRunLog{})).Count(&count).Error; err != nil {
		return false, errorx.Wrapf(err, "verify expt_run_log latest run fail, expt_id: %v, expt_run_id: %v", exptID, exptRunID)
	}
	return count == 1, nil
}
