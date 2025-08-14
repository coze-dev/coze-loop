// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"

	"gorm.io/gen"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

//go:generate  mockgen -destination=mocks/expt_turn_result.go  -package mocks . ExptTurnResultDAO
type ExptTurnResultDAO interface {
	ListTurnResult(ctx context.Context, spaceID, exptID int64, filter *entity.ExptTurnResultFilter, page entity.Page, desc bool, opts ...db.Option) ([]*model.ExptTurnResult, int64, error)
	ListTurnResultByItemIDs(ctx context.Context, spaceID, exptID int64, itemIDs []int64, page entity.Page, desc bool, opts ...db.Option) ([]*model.ExptTurnResult, int64, error)
	BatchGet(ctx context.Context, spaceID, exptID int64, itemIDs []int64, opts ...db.Option) ([]*model.ExptTurnResult, error)
	CreateTurnEvaluatorRefs(ctx context.Context, turnResults []*model.ExptTurnEvaluatorResultRef, opts ...db.Option) error
	BatchCreateNX(ctx context.Context, turnResults []*model.ExptTurnResult, opts ...db.Option) error
	GetItemTurnResults(ctx context.Context, exptID, itemID, spaceID int64, opts ...db.Option) ([]*model.ExptTurnResult, error)
	SaveTurnResults(ctx context.Context, turnResults []*model.ExptTurnResult, opts ...db.Option) error
	ScanTurnResults(ctx context.Context, exptID int64, status []int32, cursor, limit, spaceID int64, opts ...db.Option) ([]*model.ExptTurnResult, int64, error)
	UpdateTurnResults(ctx context.Context, exptID int64, itemTurnIDs []*entity.ItemTurnID, spaceID int64, ufields map[string]any, opts ...db.Option) error
	UpdateTurnResultsWithItemIDs(ctx context.Context, exptID int64, itemIDs []int64, spaceID int64, ufields map[string]any, opts ...db.Option) error

	BatchCreateNXRunLog(ctx context.Context, turnResults []*model.ExptTurnResultRunLog, opts ...db.Option) error
	GetItemTurnRunLogs(ctx context.Context, exptID, exptRunID, itemID, spaceID int64, opts ...db.Option) ([]*model.ExptTurnResultRunLog, error)
	MGetItemTurnRunLogs(ctx context.Context, exptID, exptRunID int64, itemIDs []int64, spaceID int64, opts ...db.Option) ([]*model.ExptTurnResultRunLog, error)
	SaveTurnRunLogs(ctx context.Context, turnResults []*model.ExptTurnResultRunLog, opts ...db.Option) error
	UpdateTurnRunLogWithItemIDs(ctx context.Context, spaceID, exptID, exptRunID int64, itemIDs []int64, ufields map[string]any, opts ...db.Option) error
	ScanTurnRunLogs(ctx context.Context, exptID, cursor, limit, spaceID int64, opts ...db.Option) ([]*model.ExptTurnResultRunLog, int64, error)
}

func NewExptTurnResultDAO(db db.Provider) ExptTurnResultDAO {
	return &ExptTurnResultDAOImpl{
		provider: db,
	}
}

type ExptTurnResultDAOImpl struct {
	provider db.Provider
}

func (dao *ExptTurnResultDAOImpl) UpdateTurnResultsWithItemIDs(ctx context.Context, exptID int64, itemIDs []int64, spaceID int64, ufields map[string]any, opts ...db.Option) error {
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptTurnResult
	if _, err := q.WithContext(ctx).
		Where(
			q.SpaceID.Eq(spaceID),
			q.ExptID.Eq(exptID),
			q.ItemID.In(itemIDs...),
		).
		Updates(ufields); err != nil {
		return errorx.Wrapf(err, "UpdateTurnResultsWithItemIDs fail, exptID: %v, itemIDs: %v, ufields: %v", exptID, itemIDs, ufields)
	}

	return nil
}

func (dao *ExptTurnResultDAOImpl) UpdateTurnResults(ctx context.Context, exptID int64, itemTurnIDs []*entity.ItemTurnID, spaceID int64, ufields map[string]any, opts ...db.Option) error {
	itParam := make([][]int64, 0, len(itemTurnIDs))
	for _, itID := range itemTurnIDs {
		itParam = append(itParam, []int64{itID.ItemID, itID.TurnID})
	}

	db := dao.provider.NewSession(ctx, opts...)
	err := db.WithContext(ctx).Model(&model.ExptTurnResult{}).
		Where("space_id = ? AND expt_id = ? AND (item_id, turn_id) IN ?", spaceID, exptID, itParam).
		Updates(ufields).Error
	if err != nil {
		return errorx.Wrapf(err, "UpdateTurnResults fail, exptID: %v, itemTurnIDs: %v, ufields: %v", exptID, json.Jsonify(itemTurnIDs), ufields)
	}

	return nil
}

func (dao *ExptTurnResultDAOImpl) ScanTurnResults(ctx context.Context, exptID int64, status []int32, cursor, limit, spaceID int64, opts ...db.Option) ([]*model.ExptTurnResult, int64, error) {
	db := dao.provider.NewSession(ctx, opts...)
	turnResult := query.Use(db).ExptTurnResult
	conds := []gen.Condition{
		turnResult.SpaceID.Eq(spaceID),
		turnResult.ExptID.Eq(exptID),
	}

	if len(status) > 0 {
		conds = append(conds, turnResult.Status.In(status...))
	}

	if cursor > 0 {
		conds = append(conds, turnResult.ID.Gt(cursor))
	}

	query := turnResult.WithContext(ctx).
		Where(conds...).
		Order(turnResult.ID.Asc())
	if limit > 0 {
		query = query.Limit(int(limit))
	}

	res, err := query.Find()
	if err != nil {
		return nil, 0, errorx.Wrapf(err, "ScanTurnResults fail, exptID=%d, cursor=%d, limit: %v, status: %v", exptID, cursor, limit, status)
	}

	if len(res) == 0 {
		return nil, 0, nil
	}

	return res, res[len(res)-1].ID, nil
}

func (dao *ExptTurnResultDAOImpl) ScanTurnRunLogs(ctx context.Context, exptID, cursor, limit, spaceID int64, opts ...db.Option) ([]*model.ExptTurnResultRunLog, int64, error) {
	db := dao.provider.NewSession(ctx, opts...)
	runLog := query.Use(db).ExptTurnResultRunLog
	conds := []gen.Condition{
		runLog.SpaceID.Eq(spaceID),
		runLog.ExptID.Eq(exptID),
	}

	if cursor > 0 {
		conds = append(conds, runLog.ID.Gt(cursor))
	}

	query := runLog.WithContext(ctx).
		Where(conds...).
		Order(runLog.ID.Asc())
	if limit > 0 {
		query = query.Limit(int(limit))
	}

	res, err := query.Find()
	if err != nil {
		return nil, 0, errorx.Wrapf(err, "ScanTurnResults fail, exptID=%d, cursor=%d", exptID, cursor)
	}

	if len(res) == 0 {
		return nil, 0, nil
	}

	return res, res[len(res)-1].ID, nil
}

func (dao *ExptTurnResultDAOImpl) BatchCreateNX(ctx context.Context, turnResults []*model.ExptTurnResult, opts ...db.Option) error {
	db := dao.provider.NewSession(ctx, opts...)
	if err := query.Use(db).ExptTurnResult.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(turnResults, 50); err != nil {
		return errorx.Wrapf(err, "ExptTurnResultRepo.BatchCreateNX fail, cnt: %v", len(turnResults))
	}
	return nil
}

func (dao *ExptTurnResultDAOImpl) CreateTurnEvaluatorRefs(ctx context.Context, refs []*model.ExptTurnEvaluatorResultRef, opts ...db.Option) error {
	db := dao.provider.NewSession(ctx, opts...)
	if err := query.Use(db).ExptTurnEvaluatorResultRef.WithContext(ctx).Save(refs...); err != nil {
		return errorx.Wrapf(err, "ExptTurnResultRepo.CreateTurnEvaluatorRefs fail, models: %v", json.Jsonify(refs))
	}
	return nil
}

func (dao *ExptTurnResultDAOImpl) BatchGet(ctx context.Context, spaceID int64, exptID int64, itemIDs []int64, opts ...db.Option) ([]*model.ExptTurnResult, error) {
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptTurnResult
	finds, err := q.WithContext(ctx).Where(q.SpaceID.Eq(spaceID), q.ExptID.Eq(exptID), q.ItemID.In(itemIDs...)).Find()
	if err != nil {
		return nil, err
	}
	return finds, nil
}

func (dao *ExptTurnResultDAOImpl) SaveTurnResults(ctx context.Context, turnResults []*model.ExptTurnResult, opts ...db.Option) error {
	logs.CtxInfo(ctx, "SaveTurnResults: %v", json.Jsonify(turnResults))
	db := dao.provider.NewSession(ctx, opts...)
	if err := query.Use(db).ExptTurnResult.WithContext(ctx).Save(turnResults...); err != nil {
		return errorx.Wrapf(err, "ExptTurnResultRepo.SaveTurnRunLogs fail, models: %v", json.Jsonify(turnResults))
	}
	return nil
}

func (dao *ExptTurnResultDAOImpl) UpdateTurnRunLogWithItemIDs(ctx context.Context, spaceID, exptID, exptRunID int64, itemIDs []int64, ufields map[string]any, opts ...db.Option) error {
	q := query.Use(dao.provider.NewSession(ctx, opts...)).ExptTurnResultRunLog
	if _, err := q.WithContext(ctx).
		Where(
			q.SpaceID.Eq(spaceID),
			q.ExptID.Eq(exptID),
			q.ExptRunID.Eq(exptRunID),
			q.ItemID.In(itemIDs...),
		).
		Updates(ufields); err != nil {
		return errorx.Wrapf(err, "UpdateTurnRunLogWithItemIDs fail, exptID: %v, itemIDs: %v, ufields: %v", exptID, itemIDs, ufields)
	}
	return nil
}

func (dao *ExptTurnResultDAOImpl) SaveTurnRunLogs(ctx context.Context, runLogs []*model.ExptTurnResultRunLog, opts ...db.Option) error {
	db := dao.provider.NewSession(ctx, opts...)
	if err := query.Use(db).ExptTurnResultRunLog.WithContext(ctx).Save(runLogs...); err != nil {
		return errorx.Wrapf(err, "ExptTurnResultRepo.SaveTurnRunLogs fail, models: %v", json.Jsonify(runLogs))
	}
	return nil
}

func (dao *ExptTurnResultDAOImpl) GetItemTurnResults(ctx context.Context, exptID, itemID, spaceID int64, opts ...db.Option) ([]*model.ExptTurnResult, error) {
	db := dao.provider.NewSession(ctx, opts...)
	tr := query.Use(db).ExptTurnResult
	found, err := tr.WithContext(ctx).
		Where(
			tr.SpaceID.Eq(spaceID),
			tr.ExptID.Eq(exptID),
			tr.ItemID.Eq(itemID),
		).Find()
	if err != nil {
		return nil, errorx.Wrapf(err, "GetItemTurnRunLogs fail, expt_id: %v, item_id: %v", exptID, itemID)
	}
	return found, nil
}

func (dao *ExptTurnResultDAOImpl) GetItemTurnRunLogs(ctx context.Context, exptID, exptRunID, itemID, spaceID int64, opts ...db.Option) ([]*model.ExptTurnResultRunLog, error) {
	db := dao.provider.NewSession(ctx, opts...)
	runLog := query.Use(db).ExptTurnResultRunLog
	found, err := runLog.WithContext(ctx).
		Where(
			runLog.SpaceID.Eq(spaceID),
			runLog.ExptID.Eq(exptID),
			runLog.ExptRunID.Eq(exptRunID),
			runLog.ItemID.Eq(itemID),
		).Find()
	if err != nil {
		return nil, errorx.Wrapf(err, "GetItemTurnRunLogs fail, expt_id: %v, expt_run_id: %v, item_id: %v", exptID, exptRunID, itemID)
	}
	return found, nil
}

func (dao *ExptTurnResultDAOImpl) MGetItemTurnRunLogs(ctx context.Context, exptID, exptRunID int64, itemIDs []int64, spaceID int64, opts ...db.Option) ([]*model.ExptTurnResultRunLog, error) {
	db := dao.provider.NewSession(ctx, opts...)
	runLog := query.Use(db).ExptTurnResultRunLog
	found, err := runLog.WithContext(ctx).
		Where(
			runLog.SpaceID.Eq(spaceID),
			runLog.ExptID.Eq(exptID),
			runLog.ExptRunID.Eq(exptRunID),
			runLog.ItemID.In(itemIDs...),
		).Find()
	if err != nil {
		return nil, errorx.Wrapf(err, "GetItemTurnRunLogs fail, expt_id: %v, expt_run_id: %v, item_ids: %v", exptID, exptRunID, itemIDs)
	}
	return found, nil
}

func (dao *ExptTurnResultDAOImpl) BatchCreateNXRunLog(ctx context.Context, turnResults []*model.ExptTurnResultRunLog, opts ...db.Option) error {
	db := dao.provider.NewSession(ctx, opts...)
	if err := query.Use(db).ExptTurnResultRunLog.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(turnResults, 50); err != nil {
		return errorx.Wrapf(err, "ExptTurnResultRepo.BatchCreateNXRunLogs fail, cnt: %v", len(turnResults))
	}
	return nil
}

func (dao *ExptTurnResultDAOImpl) ListTurnResult(ctx context.Context, spaceID, exptID int64, filter *entity.ExptTurnResultFilter, page entity.Page, desc bool, opts ...db.Option) ([]*model.ExptTurnResult, int64, error) {
	var (
		finds []*model.ExptTurnResult
		total int64
	)

	// score 排序本期不做
	db := dao.provider.NewSession(ctx, opts...)

	subQueries := make([]*gorm.DB, 0)
	if filter != nil && len(filter.ScoreFilters) > 0 {
		for _, scoreFilter := range filter.ScoreFilters {
			subQuery := db.Table("expt_turn_evaluator_result_ref").
				Select("1").
				Joins("INNER JOIN evaluator_record ON evaluator_record.id = expt_turn_evaluator_result_ref.evaluator_result_id").
				Where("evaluator_record.evaluator_version_id = ?", scoreFilter.EvaluatorVersionID).
				Where("evaluator_record.score "+scoreFilter.Operator+" ?", scoreFilter.Score). // 不判断deleted_at，软删除可以查到
				Where("expt_turn_evaluator_result_ref.expt_turn_result_id = expt_turn_result.id")

			subQueries = append(subQueries, subQuery)
		}
	}

	db = db.Table("expt_turn_result")
	db = db.Joins("INNER JOIN  expt_item_result ON expt_turn_result.space_id = expt_item_result.space_id AND expt_turn_result.expt_id = expt_item_result.expt_id AND expt_turn_result.item_id = expt_item_result.item_id")

	db = db.Where("expt_turn_result.space_id = ?", spaceID).
		Where("expt_turn_result.expt_id = ?", exptID)

	if filter != nil && len(filter.TrunRunStateFilters) == 1 && filter.TrunRunStateFilters[0] != nil {
		statusFilter := *filter.TrunRunStateFilters[0]
		if len(statusFilter.Status) > 0 {
			db = db.Where("expt_turn_result.status "+statusFilter.Operator+" (?)", statusFilter.Status)
		}
	}

	for _, subQuery := range subQueries {
		db = db.Where("EXISTS (?)", subQuery)
	}

	// join expt_item_result 先按item_idx排序，再按turn_idx排序
	if desc {
		db = db.Order("expt_item_result.item_idx desc")
	} else {
		db = db.Order("expt_item_result.item_idx asc")
	}
	db = db.Order("expt_turn_result.turn_idx asc")

	// 总记录数
	db = db.Count(&total)
	// 分页
	db = db.Offset(page.Offset()).Limit(page.Limit())
	err := db.Find(&finds).Error
	if err != nil {
		return nil, 0, err
	}

	filtered := make([]*model.ExptTurnResult, 0, len(finds))
	for _, got := range finds {
		if got != nil {
			filtered = append(filtered, got)
		}
	}

	logs.CtxInfo(ctx, "ListTurnResult done, finds len: %v, got len: %v", len(finds), len(filtered))

	return finds, total, nil
}

// nolint: byted_s_too_many_lines_in_func
func (dao *ExptTurnResultDAOImpl) ListTurnResultByItemIDs(ctx context.Context, spaceID, exptID int64, itemIDs []int64, page entity.Page, desc bool, opts ...db.Option) ([]*model.ExptTurnResult, int64, error) {
	var (
		finds []*model.ExptTurnResult
		total int64
	)

	db := dao.provider.NewSession(ctx, opts...)
	db = db.Table("expt_turn_result")

	if contexts.CtxWriteDB(ctx) {
		db = db.Clauses(dbresolver.Write)
	}
	if spaceID != 0 {
		db = db.Where("space_id = ?", spaceID)
	}
	if exptID != 0 {
		db = db.Where("expt_id = ?", exptID)
	}
	if len(itemIDs) > 0 {
		db = db.Where("item_id IN (?)", itemIDs)
	}

	// 总记录数
	db = db.Count(&total)
	// 分页
	if page.Offset() > 0 && page.Limit() > 0 {
		db = db.Offset(page.Offset()).Limit(page.Limit())
	}

	err := db.Find(&finds).Error
	if err != nil {
		return nil, 0, err
	}

	filtered := make([]*model.ExptTurnResult, 0, len(finds))
	for _, got := range finds {
		if got != nil {
			filtered = append(filtered, got)
		}
	}

	logs.CtxInfo(ctx, "ListTurnResult done, finds len: %v, got len: %v", len(finds), len(filtered))

	return finds, total, nil
}

func (dao *ExptTurnResultDAOImpl) GetExptTurnResultTable(ctx context.Context) *gorm.DB {
	return dao.provider.NewSession(ctx).Table(model.TableNameExptTurnResult)
}
