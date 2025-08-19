// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/query"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

//go:generate  mockgen -destination=mocks/expt_stats.go  -package mocks . IExptStatsDAO
type IExptStatsDAO interface {
	Create(ctx context.Context, stats *model.ExptStats) error
	Get(ctx context.Context, exptID, spaceID int64) (*model.ExptStats, error)
	MGet(ctx context.Context, exptIDs []int64, spaceID int64) ([]*model.ExptStats, error)
	UpdateByExptID(ctx context.Context, exptID, spaceID int64, stats *model.ExptStats) error
	ArithOperateCount(ctx context.Context, exptID, spaceID int64, cntArithOp *entity.StatsCntArithOp) error
	Save(ctx context.Context, stats *model.ExptStats) error
}

func NewExptStatsDAO(db db.Provider) IExptStatsDAO {
	return &exptStatsDAOImpl{
		db:    db,
		query: query.Use(db.NewSession(context.Background())),
	}
}

type exptStatsDAOImpl struct {
	db    db.Provider
	query *query.Query
}

func (e *exptStatsDAOImpl) Create(ctx context.Context, stats *model.ExptStats) error {
	if err := e.db.NewSession(ctx).Create(stats).Error; err != nil {
		return errorx.Wrapf(err, "create ExptStats fail, model: %v", json.Jsonify(stats))
	}
	return nil
}

func (e *exptStatsDAOImpl) Get(ctx context.Context, exptID, spaceID int64) (*model.ExptStats, error) {
	exptStats, err := e.query.ExptStats.WithContext(ctx).
		Where(e.query.ExptStats.ExptID.Eq(exptID), e.query.ExptStats.SpaceID.Eq(spaceID)).
		First()
	if err != nil {
		return nil, errorx.Wrapf(err, "get ExptStats fail, exptID: %v, spaceID: %v", exptID, spaceID)
	}
	return exptStats, nil
}

func (e *exptStatsDAOImpl) MGet(ctx context.Context, exptIDs []int64, spaceID int64) ([]*model.ExptStats, error) {
	exptStats, err := e.query.ExptStats.WithContext(ctx).
		Where(e.query.ExptStats.ExptID.In(exptIDs...), e.query.ExptStats.SpaceID.Eq(spaceID)).
		Find()
	if err != nil {
		return nil, errorx.Wrapf(err, "mget ExptStats fail, exptIDs: %v, spaceID: %v", exptIDs, spaceID)
	}
	return exptStats, nil
}

func (e *exptStatsDAOImpl) UpdateByExptID(ctx context.Context, exptID, spaceID int64, stats *model.ExptStats) error {
	_, err := e.query.ExptStats.WithContext(ctx).
		Where(e.query.ExptStats.ExptID.Eq(exptID), e.query.ExptStats.SpaceID.Eq(spaceID)).
		Updates(map[string]interface{}{
			"processing_cnt": stats.ProcessingCnt,
			"pending_cnt":    stats.PendingCnt,
			"success_cnt":    stats.SuccessCnt,
			"fail_cnt":       stats.FailCnt,
			"terminated_cnt": stats.TerminatedCnt,
		})
	if err != nil {
		return errorx.Wrapf(err, "update ExptStats fail, exptID: %v, spaceID: %v, stats: %v", exptID, spaceID, json.Jsonify(stats))
	}
	return nil
}

func (e *exptStatsDAOImpl) Save(ctx context.Context, stats *model.ExptStats) error {
	err := e.db.NewSession(ctx).Save(stats).Error
	if err != nil {
		return errorx.Wrapf(err, "update ExptStats fail, stats: %v", json.Jsonify(stats))
	}
	return nil
}

func (e *exptStatsDAOImpl) ArithOperateCount(ctx context.Context, exptID, spaceID int64, cntArithOp *entity.StatsCntArithOp) error {
	if cntArithOp == nil {
		return nil
	}

	if len(cntArithOp.OpStatusCnt) == 0 {
		return nil
	}

	update := false
	db := e.db.NewSession(ctx).
		Model(&model.ExptStats{}).
		Where("space_id = ? AND expt_id = ?", spaceID, exptID).Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate})

	for status, opCnt := range cntArithOp.OpStatusCnt {
		col := ItemRunStateStatsField(status)
		if len(col) == 0 || opCnt == 0 {
			continue
		}

		update = true
		db.Update(col, gorm.Expr(col+" + ?", opCnt))
	}

	if !update {
		logs.CtxInfo(ctx, "ArithOperateCount without update, cntArithOp: %v", json.Jsonify(cntArithOp))
		return nil
	}

	logs.CtxInfo(ctx, "[StatsCnt] ArithOperateCount expt_id: %v, op_cnt: %v", exptID, json.Jsonify(cntArithOp))

	if err := db.Error; err != nil {
		return errorx.Wrapf(err, "update ExptStats cnt fail, expt_id: %v, ufields: %v", exptID, json.Jsonify(cntArithOp))
	}
	return nil
}

func TurnRunStateStatsField(state entity.TurnRunState) string {
	switch state {
	case entity.TurnRunState_Queueing:
		return "pending_cnt"
	case entity.TurnRunState_Fail:
		return "fail_cnt"
	case entity.TurnRunState_Success:
		return "success_cnt"
	case entity.TurnRunState_Processing:
		return "processing_cnt"
	case entity.TurnRunState_Terminal:
		return "terminated_cnt"
	default:
		return ""
	}
}

func ItemRunStateStatsField(state entity.ItemRunState) string {
	switch state {
	case entity.ItemRunState_Queueing:
		return "pending_cnt"
	case entity.ItemRunState_Fail:
		return "fail_cnt"
	case entity.ItemRunState_Success:
		return "success_cnt"
	case entity.ItemRunState_Processing:
		return "processing_cnt"
	case entity.ItemRunState_Terminal:
		return "terminated_cnt"
	default:
		return ""
	}
}
