// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	"gorm.io/gen"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/hints"
	"gorm.io/plugin/dbresolver"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/query"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

//go:generate  mockgen -destination=mocks/expt_item_result.go  -package mocks . IExptItemResultDAO
type IExptItemResultDAO interface {
	BatchGet(ctx context.Context, spaceID, exptID int64, itemIDs []int64, opts ...db.Option) ([]*model.ExptItemResult, error)
	BatchCreateNX(ctx context.Context, itemResults []*model.ExptItemResult, opts ...db.Option) error
	ScanItemResults(ctx context.Context, exptID, cursor, limit int64, status []int32, spaceID int64, opts ...db.Option) (results []*model.ExptItemResult, ncursor int64, err error)
	MGetItemResults(ctx context.Context, spaceID, exptID int64, itemIDs []int64, opts ...db.Option) (results []*model.ExptItemResult, err error)
	GetItemIDListByExptID(ctx context.Context, exptID, spaceID int64) (itemIDList []int64, err error)
	ListItemResultsByExptID(ctx context.Context, exptID, spaceID int64, page entity.Page, desc bool) ([]*model.ExptItemResult, int64, error)
	SaveItemResults(ctx context.Context, itemResults []*model.ExptItemResult, opts ...db.Option) error
	GetItemTurnResults(ctx context.Context, spaceID, exptID, itemID int64, opts ...db.Option) ([]*model.ExptTurnResult, error)
	MGetItemTurnResults(ctx context.Context, spaceID, exptID int64, itemIDs []int64, opts ...db.Option) ([]*model.ExptTurnResult, error)
	UpdateItemsResult(ctx context.Context, spaceID, exptID int64, itemID []int64, ufields map[string]any, opts ...db.Option) error
	// CountItemsByStatus 按 status 聚合该实验的 item 行数，返回 status -> 行数。
	// 供 expt_stats 计数行的对账使用：主表是完成侧记账的锚点，所以要对账就得以它为准。
	CountItemsByStatus(ctx context.Context, spaceID, exptID int64, opts ...db.Option) (map[int32]int64, error)
	GetMaxItemIdxByExptID(ctx context.Context, exptID, spaceID int64, opts ...db.Option) (int32, error)

	BatchCreateNXRunLogs(ctx context.Context, itemRunLogs []*model.ExptItemResultRunLog, opts ...db.Option) error
	FillItemRunLogLogIDIfEmpty(ctx context.Context, exptID, exptRunID, spaceID int64, itemIDToLogID map[int64]string, opts ...db.Option) error
	ScanItemRunLogs(ctx context.Context, exptID, exptRunID int64, filter *entity.ExptItemRunLogFilter, cursor, limit, spaceID int64, opts ...db.Option) ([]*model.ExptItemResultRunLog, int64, error)
	UpdateItemRunLog(ctx context.Context, exptID, exptRunID int64, itemID []int64, ufields map[string]any, spaceID int64, opts ...db.Option) error
	// UpdateItemRunLogIfNotTerminal 与 UpdateItemRunLog 相同，但 WHERE 追加 status <> Terminal，
	// 用一条原子 UPDATE 保证「Terminal 是吸收态」，替代「先 SELECT 判定再写」的非原子实现。
	UpdateItemRunLogIfNotTerminal(ctx context.Context, exptID, exptRunID int64, itemID []int64, ufields map[string]any, spaceID int64, opts ...db.Option) error
	YieldItemRunForRetry(ctx context.Context, exptID, exptRunID, itemID, spaceID int64, expectedRetryTimes int32, errMsg string, opts ...db.Option) (bool, error)
	ClaimItemRunForSubmit(ctx context.Context, exptID, exptRunID, itemID, spaceID int64, expectedRetryTimes int32, opts ...db.Option) (bool, error)
	RollbackItemRunSubmit(ctx context.Context, exptID, exptRunID, itemID, spaceID int64, expectedRetryTimes int32, opts ...db.Option) (bool, error)
	GetItemRunLog(ctx context.Context, exptID, exptRunID, itemID, spaceID int64, opts ...db.Option) (*model.ExptItemResultRunLog, error)
	MGetItemRunLog(ctx context.Context, exptID, exptRunID int64, itemIDs []int64, spaceID int64, opts ...db.Option) ([]*model.ExptItemResultRunLog, error)
}

type exptItemResultDAOImpl struct {
	provider db.Provider
}

func NewExptItemResultDAO(db db.Provider) IExptItemResultDAO {
	return &exptItemResultDAOImpl{
		provider: db,
	}
}

func (dao *exptItemResultDAOImpl) BatchGet(ctx context.Context, spaceID, exptID int64, itemIDs []int64, opts ...db.Option) ([]*model.ExptItemResult, error) {
	db := dao.provider.NewSession(ctx, opts...)
	if contexts.CtxWriteDB(ctx) {
		db = db.Clauses(dbresolver.Write)
	}
	q := query.Use(db).ExptItemResult
	finds, err := q.WithContext(ctx).Where(q.SpaceID.Eq(spaceID), q.ExptID.Eq(exptID), q.ItemID.In(itemIDs...)).Find()
	if err != nil {
		return nil, err
	}
	return finds, nil
}

func (dao *exptItemResultDAOImpl) UpdateItemsResult(ctx context.Context, spaceID, exptID int64, itemIDs []int64, ufields map[string]any, opts ...db.Option) error {
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptItemResult
	_, err := q.WithContext(ctx).
		Where(q.SpaceID.Eq(spaceID),
			q.ExptID.Eq(exptID),
			q.ItemID.In(itemIDs...)).
		Updates(ufields)
	if err != nil {
		return errorx.Wrapf(err, "UpdateItemsResult fail, expt_id: %v, item_id: %v, ufields: %v", exptID, itemIDs, ufields)
	}
	return nil
}

// CountItemsByStatus 一条 GROUP BY 拿到该实验 item 的状态分布。
//
// 走 (space_id, expt_id) 前缀，不带 item_id，所以是一次索引扫描而非逐行取。
// 刻意只查主表不查 turn 表：完成侧的 statsCntOp 是按 items_result.Status 做「-1」的
// （见 expt_result_impl.go），对账要能修正它就必须以同一张表为准 ——
// 拿 turn 表对账会在多轮实验上得出另一套数字。
func (dao *exptItemResultDAOImpl) CountItemsByStatus(ctx context.Context, spaceID, exptID int64, opts ...db.Option) (map[int32]int64, error) {
	type row struct {
		Status int32 `gorm:"column:status"`
		Cnt    int64 `gorm:"column:cnt"`
	}
	var rows []*row
	session := dao.provider.NewSession(ctx, opts...)
	if err := session.Model(&model.ExptItemResult{}).
		Select("status, count(*) as cnt").
		Where("space_id = ? AND expt_id = ?", spaceID, exptID).
		Group("status").
		Find(&rows).Error; err != nil {
		return nil, errorx.Wrapf(err, "CountItemsByStatus fail, expt_id: %v", exptID)
	}
	out := make(map[int32]int64, len(rows))
	for _, r := range rows {
		if r == nil {
			continue
		}
		out[r.Status] = r.Cnt
	}
	return out, nil
}

func (dao *exptItemResultDAOImpl) GetItemTurnResults(ctx context.Context, spaceID, exptID, itemID int64, opts ...db.Option) ([]*model.ExptTurnResult, error) {
	db := dao.provider.NewSession(ctx, opts...)
	if contexts.CtxWriteDB(ctx) {
		db = db.Clauses(dbresolver.Write)
	}
	q := query.Use(db).ExptTurnResult
	finds, err := q.WithContext(ctx).Where(q.SpaceID.Eq(spaceID), q.ExptID.Eq(exptID), q.ItemID.Eq(itemID)).Find()
	if err != nil {
		return nil, err
	}
	return finds, nil
}

func (dao *exptItemResultDAOImpl) MGetItemTurnResults(ctx context.Context, spaceID, exptID int64, itemIDs []int64, opts ...db.Option) ([]*model.ExptTurnResult, error) {
	if len(itemIDs) == 0 {
		return nil, nil
	}
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptTurnResult
	found, err := q.WithContext(ctx).Where(q.SpaceID.Eq(spaceID), q.ExptID.Eq(exptID), q.ItemID.In(itemIDs...)).Find()
	if err != nil {
		return nil, err
	}
	return found, nil
}

func (dao *exptItemResultDAOImpl) SaveItemResults(ctx context.Context, itemResults []*model.ExptItemResult, opts ...db.Option) error {
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptItemResult
	if err := q.WithContext(ctx).Save(itemResults...); err != nil {
		return errorx.Wrapf(err, "SaveItemResults fail, model: %v", json.Jsonify(itemResults))
	}
	return nil
}

func (dao *exptItemResultDAOImpl) SaveItemRunLogs(ctx context.Context, itemRunLogs []*model.ExptItemResultRunLog, opts ...db.Option) error {
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptItemResultRunLog
	if err := q.WithContext(ctx).Save(itemRunLogs...); err != nil {
		return errorx.Wrapf(err, "SaveItemRunLogs fail, model: %v", json.Jsonify(itemRunLogs))
	}
	return nil
}

func (dao *exptItemResultDAOImpl) GetItemRunLog(ctx context.Context, exptID, exptRunID, itemID, spaceID int64, opts ...db.Option) (*model.ExptItemResultRunLog, error) {
	db := dao.provider.NewSession(ctx, opts...)
	// 读后写场景（如 Terminal 吸收态判定）需强制读主库：普通 SELECT 会被 dbresolver 路由到只读从库，
	// 主从延迟窗口内会读回过期状态。与本文件 BatchGet 的写法一致，调用方用 contexts.WithCtxWriteDB 开启。
	if contexts.CtxWriteDB(ctx) {
		db = db.Clauses(dbresolver.Write)
	}
	q := query.Use(db).ExptItemResultRunLog
	found, err := q.WithContext(ctx).
		Where(q.SpaceID.Eq(spaceID),
			q.ExptID.Eq(exptID),
			q.ExptRunID.Eq(exptRunID),
			q.ItemID.Eq(itemID)).
		First()
	if err != nil {
		return nil, errorx.Wrapf(err, "GetItemRunLog fail, expt_id: %v, expt_run_id: %v, item_id: %v", exptID, exptRunID, itemID)
	}
	return found, nil
}

func (dao *exptItemResultDAOImpl) MGetItemRunLog(ctx context.Context, exptID, exptRunID int64, itemIDs []int64, spaceID int64, opts ...db.Option) ([]*model.ExptItemResultRunLog, error) {
	db := dao.provider.NewSession(ctx, opts...)
	if contexts.CtxWriteDB(ctx) {
		db = db.Clauses(dbresolver.Write)
	}
	q := query.Use(db).ExptItemResultRunLog
	found, err := q.WithContext(ctx).
		Where(q.SpaceID.Eq(spaceID),
			q.ExptID.Eq(exptID),
			q.ExptRunID.Eq(exptRunID),
			q.ItemID.In(itemIDs...)).
		Find()
	if err != nil {
		return nil, errorx.Wrapf(err, "GetItemRunLog fail, expt_id: %v, expt_run_id: %v, item_ids: %v", exptID, exptRunID, itemIDs)
	}
	return found, nil
}

func (dao *exptItemResultDAOImpl) UpdateItemRunLog(ctx context.Context, exptID, exptRunID int64, itemID []int64, ufields map[string]any, spaceID int64, opts ...db.Option) error {
	logs.CtxInfo(ctx, "UpdateItemRunLog, expt_id: %v, expt_run_id: %v, item_ids: %v, ufields: %v", exptID, exptRunID, itemID, ufields)
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptItemResultRunLog
	_, err := q.WithContext(ctx).
		Where(
			q.SpaceID.Eq(spaceID),
			q.ExptID.Eq(exptID),
			q.ExptRunID.Eq(exptRunID),
			q.ItemID.In(itemID...),
		).
		UpdateColumns(ufields)
	if err != nil {
		return errorx.Wrapf(err, "ExptItemResultRepo.UpdateItemRunLog failed, expt_id: %v, run_id: %v, item_id: %v, ufields: %v", exptID, exptRunID, itemID, ufields)
	}
	return nil
}

// UpdateItemRunLogIfNotTerminal 条件更新：仅当当前 status 不是 Terminal 时才写入。
//
// 为什么必须是条件 UPDATE 而不是「先 SELECT 判 Terminal 再 UPDATE」：普通 SELECT 会被 dbresolver
// 路由到只读从库（商业化侧 db 用 WithReadReplicas 构建），主从延迟窗口内会读回终止前的旧状态
// (Processing)，把主库上已经写好的 Terminal 覆盖成 Success/Fail —— 用户的终止操作被静默吞掉，
// 该行还会被重新扫成成功行、发 item-complete MQ、记进 success_cnt。
// 即便强制读主库，读-判-写三步之间仍有 TOCTOU 窗口，故根治手段是把判定下沉进 WHERE。
//
// 影响行数为 0（即命中 Terminal 被拦下）不视为错误：吸收态生效就是预期结果，调用方按业务语义处理。
func (dao *exptItemResultDAOImpl) UpdateItemRunLogIfNotTerminal(ctx context.Context, exptID, exptRunID int64, itemID []int64, ufields map[string]any, spaceID int64, opts ...db.Option) error {
	logs.CtxInfo(ctx, "UpdateItemRunLogIfNotTerminal, expt_id: %v, expt_run_id: %v, item_ids: %v, ufields: %v", exptID, exptRunID, itemID, ufields)
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptItemResultRunLog
	info, err := q.WithContext(ctx).
		Where(
			q.SpaceID.Eq(spaceID),
			q.ExptID.Eq(exptID),
			q.ExptRunID.Eq(exptRunID),
			q.ItemID.In(itemID...),
			q.Status.Neq(int32(entity.ItemRunState_Terminal)),
		).
		UpdateColumns(ufields)
	if err != nil {
		return errorx.Wrapf(err, "ExptItemResultRepo.UpdateItemRunLogIfNotTerminal failed, expt_id: %v, run_id: %v, item_id: %v, ufields: %v", exptID, exptRunID, itemID, ufields)
	}
	if info.RowsAffected == 0 {
		logs.CtxInfo(ctx, "[ExptItemTerminate] UpdateItemRunLogIfNotTerminal skipped by terminal absorbing state, expt_id: %v, expt_run_id: %v, item_ids: %v", exptID, exptRunID, itemID)
	}
	return nil
}

func (dao *exptItemResultDAOImpl) ScanItemResults(ctx context.Context, exptID, cursor, limit int64, status []int32, spaceID int64, opts ...db.Option) (results []*model.ExptItemResult, ncursor int64, err error) {
	if len(status) == 0 {
		return nil, 0, fmt.Errorf("ExptItemResultRepo.ScanItemResults with null status")
	}
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptItemResult
	conds := []gen.Condition{
		q.SpaceID.Eq(spaceID),
		q.ExptID.Eq(exptID),
		q.Status.In(status...),
	}

	if cursor > 0 {
		conds = append(conds, q.ID.Gt(cursor))
	}

	query := q.WithContext(ctx).
		Where(conds...).
		Order(q.ID.Asc())
	if limit > 0 {
		query = query.Limit(int(limit))
	}

	res, err := query.Find()
	if err != nil {
		return nil, 0, errorx.Wrapf(err, "ScanItemResults fail, exptID=%d, cursor=%d", exptID, cursor)
	}

	if len(res) == 0 {
		return nil, 0, nil
	}

	return res, res[len(res)-1].ID, nil
}

func (dao *exptItemResultDAOImpl) MGetItemResults(ctx context.Context, spaceID, exptID int64, itemIDs []int64, opts ...db.Option) (results []*model.ExptItemResult, err error) {
	if len(itemIDs) == 0 {
		return nil, nil
	}
	db := dao.provider.NewSession(ctx, opts...)
	if contexts.CtxWriteDB(ctx) {
		db = db.Clauses(dbresolver.Write)
	}
	q := query.Use(db).ExptItemResult
	res, err := q.WithContext(ctx).Where(q.SpaceID.Eq(spaceID), q.ExptID.Eq(exptID), q.ItemID.In(itemIDs...)).Find()
	if err != nil {
		return nil, errorx.Wrapf(err, "MGetItemResults fail, exptID=%d, spaceID=%d, itemIDs=%v", exptID, spaceID, itemIDs)
	}
	return res, nil
}

func (dao *exptItemResultDAOImpl) ListItemResultsByExptID(ctx context.Context, exptID, spaceID int64, page entity.Page, desc bool) ([]*model.ExptItemResult, int64, error) {
	db := dao.provider.NewSession(ctx)
	q := query.Use(db).ExptItemResult
	conds := []gen.Condition{
		q.SpaceID.Eq(spaceID),
		q.ExptID.Eq(exptID),
	}

	query := q.WithContext(ctx).
		Where(conds...)
	total, err := query.Count()
	if err != nil {
		return nil, 0, errorx.Wrapf(err, "ListItemResultsByExptID fail, exptID=%d, spaceID=%d, page=%v, desc=%v", exptID, spaceID, page, desc)
	}
	if desc {
		query = query.Order(q.ItemIdx.Desc())
	} else {
		query = query.Order(q.ItemIdx.Asc())
	}
	if page.Limit() > 0 {
		query = query.Limit(page.Limit())
	}
	if page.Offset() > 0 {
		query = query.Offset(page.Offset())
	}
	res, err := query.Find()
	if err != nil {
		return nil, 0, errorx.Wrapf(err, "ListItemResultsByExptID fail, exptID=%d, spaceID=%d, page=%v, desc=%v", exptID, spaceID, page, desc)
	}
	return res, total, nil
}

func (dao *exptItemResultDAOImpl) GetItemIDListByExptID(ctx context.Context, exptID, spaceID int64) (itemIDList []int64, err error) {
	db := dao.provider.NewSession(ctx)
	q := query.Use(db).ExptItemResult

	conds := []gen.Condition{
		q.SpaceID.Eq(spaceID),
		q.ExptID.Eq(exptID),
	}

	query := q.WithContext(ctx).
		Select(q.ItemID).
		Where(conds...).
		Group(q.ItemID)
	query = query.Limit(consts.MaxEvalSetItemLimit)

	res, err := query.Find()
	if err != nil {
		return nil, errorx.Wrapf(err, "ScanItemResults fail, exptID=%d", exptID)
	}

	if len(res) == 0 {
		return nil, nil
	}

	for _, itemResult := range res {
		itemIDList = append(itemIDList, itemResult.ItemID)
	}
	return itemIDList, nil
}

func (dao *exptItemResultDAOImpl) ScanItemRunLogs(ctx context.Context, exptID, exptRunID int64, filter *entity.ExptItemRunLogFilter, cursor, limit, spaceID int64, opts ...db.Option) ([]*model.ExptItemResultRunLog, int64, error) {
	if filter == nil {
		filter = &entity.ExptItemRunLogFilter{}
	}
	session := dao.provider.NewSession(ctx, opts...)

	// RawFilter: use raw gorm.DB Where(sql, vars...) to avoid gen clause conversion / unknown clause issues.
	if filter.RawFilter && filter.RawCond.SQL != "" {
		var res []*model.ExptItemResultRunLog
		// ⚠️ ORDER BY 按 filter flag 局部决定, 不得无条件改: 本方法是游标分页协议(id > cursor 依赖 id asc),
		// 无条件改排序会让走游标循环的调用方漏行+重复行(技术方案 §4.0)。
		// flag=true(让位降权): retry_times asc, id asc; 与游标翻页互斥, 校验 cursor==0。
		//   ForceIndex 仅在 RetryPickIndexReady=true(索引已建成)时下发; 未建成则留空由优化器自选,
		//   排序语义不变(仅退化 filesort), 避免 Key doesn't exist。
		// flag=false: 一字不变, 维持 ForceIndex(uk_expt_run_item_turn) + id asc。
		forceIndex := "uk_expt_run_item_turn"
		orderBy := "id asc"
		if filter.OrderByRetryTimesFirst {
			if cursor > 0 {
				return nil, 0, fmt.Errorf("ScanItemRunLogs OrderByRetryTimesFirst is incompatible with cursor paging, exptID=%d, exptRunID=%d, cursor=%d", exptID, exptRunID, cursor)
			}
			forceIndex = ""
			if filter.RetryPickIndexReady {
				forceIndex = "idx_expt_run_retry_pick"
			}
			orderBy = "retry_times asc, id asc"
		}
		tx := session.WithContext(ctx).Model(&model.ExptItemResultRunLog{})
		if forceIndex != "" {
			tx = tx.Clauses(hints.ForceIndex(forceIndex))
		}
		tx = tx.Where("space_id = ? AND expt_id = ? AND expt_run_id = ?", spaceID, exptID, exptRunID).
			Where(filter.RawCond.SQL, filter.RawCond.Vars...)
		if cursor > 0 {
			tx = tx.Where("id > ?", cursor)
		}
		tx = tx.Order(orderBy)
		if limit > 0 {
			tx = tx.Limit(int(limit))
		}
		if err := tx.Find(&res).Error; err != nil {
			return nil, 0, errorx.Wrapf(err, "ScanItemRunLogs fail, exptID=%d, exptRunID=%d, cursor=%d", exptID, exptRunID, cursor)
		}
		if len(res) == 0 {
			return nil, 0, nil
		}
		return res, res[len(res)-1].ID, nil
	}

	q := query.Use(session).ExptItemResultRunLog
	conds := []gen.Condition{
		q.SpaceID.Eq(spaceID),
		q.ExptID.Eq(exptID),
		q.ExptRunID.Eq(exptRunID),
	}
	if filter.ResultState != nil {
		conds = append(conds, q.ResultState.In(int32(filter.GetResultState())))
	}
	if len(filter.Status) > 0 {
		conds = append(conds, q.Status.In(filter.GetStatus()...))
	}
	if cursor > 0 {
		conds = append(conds, q.ID.Gt(cursor))
	}

	// gen 分支同样按 flag 局部决定 ORDER BY + ForceIndex(见上方 RawFilter 分支注释)。
	// ForceIndex 仅在 RetryPickIndexReady=true(索引已建成)时下发; 未建成则不下 hint、由优化器自选,
	// 排序语义完全不变(retry_times asc, id asc), 只失去索引序 + LIMIT 提前停止(退化 filesort)。
	if filter.OrderByRetryTimesFirst {
		if cursor > 0 {
			return nil, 0, fmt.Errorf("ScanItemRunLogs OrderByRetryTimesFirst is incompatible with cursor paging, exptID=%d, exptRunID=%d, cursor=%d", exptID, exptRunID, cursor)
		}
		query := q.WithContext(ctx)
		if filter.RetryPickIndexReady {
			query = query.Clauses(hints.ForceIndex("idx_expt_run_retry_pick"))
		}
		query = query.Where(conds...).Order(q.RetryTimes, q.ID)
		if limit > 0 {
			query = query.Limit(int(limit))
		}
		res, err := query.Find()
		if err != nil {
			return nil, 0, errorx.Wrapf(err, "ScanItemRunLogs fail, exptID=%d, exptRunID=%d, cursor=%d", exptID, exptRunID, cursor)
		}
		if len(res) == 0 {
			return nil, 0, nil
		}
		return res, res[len(res)-1].ID, nil
	}

	query := q.WithContext(ctx).
		Clauses(hints.ForceIndex("uk_expt_run_item_turn")).
		Where(conds...).
		Order(q.ID.Asc())
	if limit > 0 {
		query = query.Limit(int(limit))
	}

	res, err := query.Find()
	if err != nil {
		return nil, 0, errorx.Wrapf(err, "ScanItemRunLogs fail, exptID=%d, exptRunID=%d, cursor=%d", exptID, exptRunID, cursor)
	}

	if len(res) == 0 {
		return nil, 0, nil
	}

	return res, res[len(res)-1].ID, nil
}

func (dao *exptItemResultDAOImpl) BatchCreateNX(ctx context.Context, itemResults []*model.ExptItemResult, opts ...db.Option) error {
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptItemResult
	if err := q.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(itemResults, 50); err != nil {
		return errorx.Wrapf(err, "ExptItemResult.BatchCreateNX fail, cnt: %v", len(itemResults))
	}
	return nil
}

func (dao *exptItemResultDAOImpl) BatchCreateNXRunLogs(ctx context.Context, itemResults []*model.ExptItemResultRunLog, opts ...db.Option) error {
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptItemResultRunLog
	if err := q.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(itemResults, 50); err != nil {
		return errorx.Wrapf(err, "ExptItemResult.BatchCreateNXRunLogs fail, cnt: %v", len(itemResults))
	}
	return nil
}

func (dao *exptItemResultDAOImpl) FillItemRunLogLogIDIfEmpty(ctx context.Context, exptID, exptRunID, spaceID int64, itemIDToLogID map[int64]string, opts ...db.Option) error {
	if len(itemIDToLogID) == 0 {
		return nil
	}

	itemIDs := make([]int64, 0, len(itemIDToLogID))
	for itemID := range itemIDToLogID {
		itemIDs = append(itemIDs, itemID)
	}
	sort.Slice(itemIDs, func(i, j int) bool { return itemIDs[i] < itemIDs[j] })

	caseExpr := "CASE item_id"
	args := make([]any, 0, len(itemIDs)*2)
	for _, itemID := range itemIDs {
		caseExpr += " WHEN ? THEN ?"
		args = append(args, itemID, itemIDToLogID[itemID])
	}
	caseExpr += " ELSE log_id END"

	session := dao.provider.NewSession(ctx, opts...)
	// UpdateColumn intentionally avoids touching updated_at: retry selection uses run-log
	// timestamps to choose the latest execution fact, and filling a missing LogID is not a new execution.
	if err := session.WithContext(ctx).
		Model(&model.ExptItemResultRunLog{}).
		Where("space_id = ? AND expt_id = ? AND expt_run_id = ? AND item_id IN ? AND log_id = ''", spaceID, exptID, exptRunID, itemIDs).
		UpdateColumn("log_id", gorm.Expr(caseExpr, args...)).Error; err != nil {
		return errorx.Wrapf(err, "FillItemRunLogLogIDIfEmpty fail, expt_id: %v, expt_run_id: %v, item_ids: %v", exptID, exptRunID, itemIDs)
	}
	return nil
}

func (dao *exptItemResultDAOImpl) GetMaxItemIdxByExptID(ctx context.Context, exptID, spaceID int64, opts ...db.Option) (int32, error) {
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptItemResult

	// 使用结构体接收聚合结果
	var result struct {
		MaxItemIdx sql.NullInt32 `gorm:"column:max_item_idx"`
	}

	err := q.WithContext(ctx).
		Select(q.ItemIdx.Max().As("max_item_idx")). // 明确指定别名
		Where(q.SpaceID.Eq(spaceID), q.ExptID.Eq(exptID)).
		Scan(&result) // 使用 Scan 代替 Row().Scan()
	if err != nil {
		return 0, errorx.Wrapf(err, "GetMaxItemIdxByExptID fail, expt_id: %v", exptID)
	}
	if !result.MaxItemIdx.Valid {
		return 0, nil // 无记录
	}
	return result.MaxItemIdx.Int32, nil
}

// YieldItemRunForRetry uses the same item-then-run lock order as result projection.
func (dao *exptItemResultDAOImpl) YieldItemRunForRetry(ctx context.Context, exptID, exptRunID, itemID, spaceID int64, expectedRetryTimes int32, errMsg string, opts ...db.Option) (bool, error) {
	if expectedRetryTimes < 0 || expectedRetryTimes >= 1<<31-1 {
		return false, fmt.Errorf("invalid retry attempt: %d", expectedRetryTimes)
	}
	applied := false
	err := dao.provider.NewSession(ctx, opts...).Clauses(dbresolver.Write).Transaction(func(tx *gorm.DB) error {
		var item model.ExptItemResult
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("space_id = ? AND expt_id = ? AND item_id = ?", spaceID, exptID, itemID).First(&item).Error; err != nil {
			return err
		}
		if item.ExptRunID != exptRunID || item.Status != int32(entity.ItemRunState_Processing) {
			return nil
		}
		var runLog model.ExptItemResultRunLog
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("space_id = ? AND expt_id = ? AND expt_run_id = ? AND item_id = ?", spaceID, exptID, exptRunID, itemID).First(&runLog).Error; err != nil {
			return err
		}
		if runLog.Status != int32(entity.ItemRunState_Processing) || runLog.RetryTimes != expectedRetryTimes {
			return nil
		}
		result := tx.Model(&model.ExptItemResultRunLog{}).Where("space_id = ? AND expt_id = ? AND expt_run_id = ? AND item_id = ? AND status = ? AND retry_times = ?", spaceID, exptID, exptRunID, itemID, int32(entity.ItemRunState_Processing), expectedRetryTimes).UpdateColumns(map[string]any{
			"status": int32(entity.ItemRunState_Queueing), "retry_times": expectedRetryTimes + 1, "err_msg": errMsg,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("retry yield lost processing attempt, expt_id=%d, run_id=%d, item_id=%d", exptID, exptRunID, itemID)
		}
		result = tx.Model(&model.ExptItemResult{}).Where("space_id = ? AND expt_id = ? AND item_id = ? AND expt_run_id = ? AND status = ?", spaceID, exptID, itemID, exptRunID, int32(entity.ItemRunState_Processing)).UpdateColumn("status", int32(entity.ItemRunState_Queueing))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("retry yield lost current item, expt_id=%d, run_id=%d, item_id=%d", exptID, exptRunID, itemID)
		}
		result = tx.Model(&model.ExptStats{}).Where("space_id = ? AND expt_id = ?", spaceID, exptID).UpdateColumns(map[string]any{
			"processing_cnt": gorm.Expr("processing_cnt - ?", 1), "pending_cnt": gorm.Expr("pending_cnt + ?", 1),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("retry yield stats missing, expt_id=%d", exptID)
		}
		applied = true
		return nil
	})
	return applied && err == nil, err
}

func (dao *exptItemResultDAOImpl) ClaimItemRunForSubmit(ctx context.Context, exptID, exptRunID, itemID, spaceID int64, expectedRetryTimes int32, opts ...db.Option) (bool, error) {
	return dao.setItemRunSubmission(ctx, exptID, exptRunID, itemID, spaceID, expectedRetryTimes, true, opts...)
}

func (dao *exptItemResultDAOImpl) RollbackItemRunSubmit(ctx context.Context, exptID, exptRunID, itemID, spaceID int64, expectedRetryTimes int32, opts ...db.Option) (bool, error) {
	return dao.setItemRunSubmission(ctx, exptID, exptRunID, itemID, spaceID, expectedRetryTimes, false, opts...)
}

// Submission and its compensation lock the same current attempt as final projection.
func (dao *exptItemResultDAOImpl) setItemRunSubmission(ctx context.Context, exptID, exptRunID, itemID, spaceID int64, expectedRetryTimes int32, claim bool, opts ...db.Option) (bool, error) {
	from, to := entity.ItemRunState_Queueing, entity.ItemRunState_Processing
	if !claim {
		from, to = to, from
	}
	applied := false
	err := dao.provider.NewSession(ctx, opts...).Clauses(dbresolver.Write).Transaction(func(tx *gorm.DB) error {
		var item model.ExptItemResult
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("space_id = ? AND expt_id = ? AND item_id = ?", spaceID, exptID, itemID).First(&item).Error; err != nil {
			return err
		}
		if item.ExptRunID != exptRunID || item.Status != int32(from) {
			return nil
		}
		var runLog model.ExptItemResultRunLog
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("space_id = ? AND expt_id = ? AND expt_run_id = ? AND item_id = ?", spaceID, exptID, exptRunID, itemID).First(&runLog).Error; err != nil {
			return err
		}
		if runLog.Status != int32(from) || runLog.RetryTimes != expectedRetryTimes || (runLog.ResultState != nil && *runLog.ResultState != int32(entity.ExptItemResultStateDefault)) {
			return nil
		}
		result := tx.Model(&model.ExptItemResultRunLog{}).Where("space_id = ? AND expt_id = ? AND expt_run_id = ? AND item_id = ? AND status = ? AND retry_times = ?", spaceID, exptID, exptRunID, itemID, int32(from), expectedRetryTimes).UpdateColumn("status", int32(to))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("item submission lost attempt, expt_id=%d, run_id=%d, item_id=%d", exptID, exptRunID, itemID)
		}
		result = tx.Model(&model.ExptItemResult{}).Where("space_id = ? AND expt_id = ? AND item_id = ? AND expt_run_id = ? AND status = ?", spaceID, exptID, itemID, exptRunID, int32(from)).UpdateColumn("status", int32(to))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("item submission lost owner, expt_id=%d, run_id=%d, item_id=%d", exptID, exptRunID, itemID)
		}
		turnStatus := entity.TurnRunState_Processing
		if !claim {
			turnStatus = entity.TurnRunState_Queueing
		}
		if err := tx.Model(&model.ExptTurnResult{}).Where("space_id = ? AND expt_id = ? AND item_id = ? AND expt_run_id = ? AND status <> ?", spaceID, exptID, itemID, exptRunID, int32(entity.TurnRunState_Terminal)).UpdateColumn("status", int32(turnStatus)).Error; err != nil {
			return err
		}
		fromColumn, toColumn := ItemRunStateStatsField(from), ItemRunStateStatsField(to)
		result = tx.Model(&model.ExptStats{}).Where("space_id = ? AND expt_id = ?", spaceID, exptID).UpdateColumns(map[string]any{fromColumn: gorm.Expr(fromColumn+" - ?", 1), toColumn: gorm.Expr(toColumn+" + ?", 1)})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("item submission stats missing, expt_id=%d", exptID)
		}
		applied = true
		return nil
	})
	return applied && err == nil, err
}
