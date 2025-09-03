// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package ck

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/ck"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/ck/gorm_gen/model"
	compare_model "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/ck/model"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// FieldFilter 用于描述map字段的筛选条件
// Op: =, >, >=, <, <=, BETWEEN, LIKE
// Values: 等值/范围/模糊
type FieldFilter struct {
	Key    string
	Op     string
	Values []any
}

type Page struct {
	Offset int
	Limit  int
}

type ItemSnapshotFilter struct {
	BoolMapFilters   []*FieldFilter
	FloatMapFilters  []*FieldFilter
	IntMapFilters    []*FieldFilter
	StringMapFilters []*FieldFilter
}

type ExptTurnResultFilterMapCond struct {
	EvalTargetDataFilters   []*FieldFilter
	EvaluatorScoreFilters   []*FieldFilter
	AnnotationFloatFilters  []*FieldFilter
	AnnotationBoolFilters   []*FieldFilter
	AnnotationStringFilters []*FieldFilter
}

type KeywordMapCond struct {
	ItemSnapshotFilter    *ItemSnapshotFilter
	EvalTargetDataFilters []*FieldFilter
	Keyword               *string
}

type ExptTurnResultFilterQueryCond struct {
	// 主表字段
	SpaceID                 *string
	ExptID                  *string
	ItemIDs                 []*FieldFilter // 支持多组itemID筛选
	ItemRunStatus           []*FieldFilter // 支持多组item状态筛选
	TurnRunStatus           []*FieldFilter // 支持多组turn状态筛选
	EvaluatorScoreCorrected *FieldFilter

	CreatedDate      *time.Time
	EvalSetVersionID *string

	// 主表map字段
	MapCond *ExptTurnResultFilterMapCond

	// 联表
	ItemSnapshotCond  *ItemSnapshotFilter
	EvalSetSyncCkDate string

	// 全文搜索
	KeywordSearch *KeywordMapCond

	// 分页
	Page Page
}

//go:generate mockgen -destination=mocks/expt.go -package=mocks . IExptTurnResultFilterDAO
type IExptTurnResultFilterDAO interface {
	Save(ctx context.Context, filter []*model.ExptTurnResultFilter) error
	QueryItemIDStates(ctx context.Context, cond *ExptTurnResultFilterQueryCond) (map[string]int32, int64, error)
	GetByExptIDItemIDs(ctx context.Context, spaceID, exptID, createdDate string, itemIDs []string) ([]*compare_model.ExptTurnResultFilter, error)
}

type exptTurnResultFilterDAOImpl struct {
	db ck.Provider
	// 新增字段，用于存储数据库名
	configer component.IConfiger
}

// NewExptTurnResultFilterDAO 创建 exptTurnResultFilterDAOImpl 实例
func NewExptTurnResultFilterDAO(db ck.Provider, configer component.IConfiger) IExptTurnResultFilterDAO {
	return &exptTurnResultFilterDAOImpl{
		db:       db,
		configer: configer,
	}
}

// newSession 创建一个新的 gorm 会话
func (d *exptTurnResultFilterDAOImpl) newSession(ctx context.Context) *gorm.DB {
	return d.db.NewSession(ctx)
}

// Save 实现 IExptTurnResultFilterDAO 接口的 Save 方法 todo 尚未真正实现
func (d *exptTurnResultFilterDAOImpl) Save(ctx context.Context, filter []*model.ExptTurnResultFilter) error {
	session := d.newSession(ctx)
	return session.Create(filter).Error
}

// 定义浮点数比较的精度
const floatEpsilon = 1e-8

func (d *exptTurnResultFilterDAOImpl) QueryItemIDStates(ctx context.Context, cond *ExptTurnResultFilterQueryCond) (map[string]int32, int64, error) {
	joinSQL, whereSQL, keywordCond, args := d.buildQueryConditions(ctx, cond)
	sql := d.buildBaseSQL(ctx, joinSQL, whereSQL, keywordCond, cond.EvalSetSyncCkDate, &args)
	total, err := d.getTotalCount(ctx, sql, args)
	if err != nil {
		return nil, total, err
	}
	// 调用修改后的方法
	sql, args = d.addPaginationAndSorting(sql, cond, args)
	return d.executeQuery(ctx, sql, args, total)
}

// buildQueryConditions 构建查询条件
func (d *exptTurnResultFilterDAOImpl) buildQueryConditions(ctx context.Context, cond *ExptTurnResultFilterQueryCond) (string, string, string, []interface{}) {
	joinSQL := ""
	whereSQL := ""
	keywordCond := ""
	args := []interface{}{}

	d.buildMainTableConditions(cond, &whereSQL, &args)
	d.buildMapFieldConditions(cond, &whereSQL, &args)
	d.buildItemSnapshotConditions(cond, &joinSQL, &args)
	d.buildKeywordSearchConditions(ctx, cond, &keywordCond, &args)

	return joinSQL, whereSQL, keywordCond, args
}

// buildMainTableConditions 构建主表字段条件
func (d *exptTurnResultFilterDAOImpl) buildMainTableConditions(cond *ExptTurnResultFilterQueryCond, whereSQL *string, args *[]interface{}) {
	if cond.SpaceID != nil {
		*whereSQL += " AND etrf.space_id = ?"
		*args = append(*args, ptr.From(cond.SpaceID))
	}
	if cond.ExptID != nil {
		*whereSQL += " AND etrf.expt_id = ?"
		*args = append(*args, ptr.From(cond.ExptID))
	}
	// 多组item_id filter
	for _, f := range cond.ItemIDs {
		switch f.Op {
		case "in", "IN":
			*whereSQL += " AND etrf.item_id IN ?"
			*args = append(*args, f.Values)
		case "=":
			*whereSQL += " AND etrf.item_id = ?"
			*args = append(*args, f.Values[0])
		case "between", "BETWEEN":
			*whereSQL += " AND etrf.item_id BETWEEN ? AND ?"
			*args = append(*args, f.Values[0], f.Values[1])
		case "!=":
			*whereSQL += " AND etrf.item_id != ?"
			*args = append(*args, f.Values[0])
		case "NOT IN":
			*whereSQL += " AND etrf.item_id NOT IN?"
			*args = append(*args, f.Values)
		}
	}
	// 多组item状态 filter
	for _, f := range cond.ItemRunStatus {
		switch f.Op {
		case "in", "IN":
			*whereSQL += " AND etrf.status IN ?"
			*args = append(*args, f.Values)
		case "=":
			*whereSQL += " AND etrf.status = ?"
			*args = append(*args, f.Values[0])
		case "between", "BETWEEN":
			*whereSQL += " AND etrf.status BETWEEN ? AND ?"
			*args = append(*args, f.Values[0], f.Values[1])
		case "!=":
			*whereSQL += " AND etrf.status!=?"
			*args = append(*args, f.Values[0])
		case "NOT IN":
			*whereSQL += " AND etrf.status NOT IN?"
			*args = append(*args, f.Values)
		}
	}

	if cond.CreatedDate != nil {
		*whereSQL += " AND etrf.created_date = ?"
		*args = append(*args, cond.CreatedDate.Format(time.DateOnly))
	}
	if cond.EvalSetVersionID != nil {
		*whereSQL += " AND etrf.eval_set_version_id = ?"
		*args = append(*args, ptr.From(cond.EvalSetVersionID))
	}
	if cond.EvaluatorScoreCorrected != nil {
		*whereSQL += " AND etrf.evaluator_score_corrected " + cond.EvaluatorScoreCorrected.Op + "?"
		*args = append(*args, cond.EvaluatorScoreCorrected.Values[0])
	}
}

// escapeSpecialChars 转义 SQL LIKE 操作中的特殊字符
func escapeSpecialChars(str string) string {
	str = strings.ReplaceAll(str, `\`, `\\`)
	str = strings.ReplaceAll(str, `%`, `\%`)
	str = strings.ReplaceAll(str, `_`, `\_`)
	return str
}

// buildMapFieldConditions 构建主表map字段条件
func (d *exptTurnResultFilterDAOImpl) buildMapFieldConditions(cond *ExptTurnResultFilterQueryCond, whereSQL *string, args *[]interface{}) {
	if cond.MapCond == nil {
		return
	}

	for _, f := range cond.MapCond.EvalTargetDataFilters {
		switch f.Op {
		case "=":
			// 删除 mapContains 条件
			*whereSQL += fmt.Sprintf(" AND etrf.eval_target_data{'%s'} = ?", f.Key)
			*args = append(*args, f.Values[0])
		case "LIKE":
			// 删除 mapContains 条件
			*whereSQL += fmt.Sprintf(" AND etrf.eval_target_data{'%s'} LIKE ?", f.Key)
			*args = append(*args, "%"+escapeSpecialChars(fmt.Sprintf("%v", f.Values[0]))+"%")
		case "NOT LIKE":
			// 删除 mapContains 条件
			*whereSQL += fmt.Sprintf(" AND etrf.eval_target_data{'%s'} NOT LIKE ?", f.Key)
			*args = append(*args, "%"+escapeSpecialChars(fmt.Sprintf("%v", f.Values[0]))+"%")
		case "!=":
			// 删除 mapContains 条件
			*whereSQL += fmt.Sprintf(" AND etrf.eval_target_data{'%s'}!=?", f.Key)
			*args = append(*args, f.Values[0])
		}
	}
	for _, f := range cond.MapCond.EvaluatorScoreFilters {
		switch f.Op {
		case "=":
			floatValue, err := strconv.ParseFloat(fmt.Sprintf("%v", f.Values[0]), 64)
			if err != nil {
				logs.CtxError(context.Background(), "Parse float value failed: %v", err)
				continue
			}
			// 删除 mapContains 条件
			*whereSQL += fmt.Sprintf(" AND abs(etrf.evaluator_score{'%s'} - ?) < %g", f.Key, floatEpsilon)
			*args = append(*args, floatValue)
		case ">", ">=", "<", "<=", "!=":
			floatValue, err := strconv.ParseFloat(fmt.Sprintf("%v", f.Values[0]), 64)
			if err != nil {
				logs.CtxError(context.Background(), "Parse float value failed: %v", err)
				continue
			}
			// 删除 mapContains 条件
			*whereSQL += fmt.Sprintf(" AND etrf.evaluator_score{'%s'} %s ?", f.Key, f.Op)
			*args = append(*args, floatValue)
		case "BETWEEN":
			floatValue1, err1 := strconv.ParseFloat(fmt.Sprintf("%v", f.Values[0]), 64)
			floatValue2, err2 := strconv.ParseFloat(fmt.Sprintf("%v", f.Values[1]), 64)
			if err1 != nil || err2 != nil {
				logs.CtxError(context.Background(), "Parse float value failed: %v, %v", err1, err2)
				continue
			}
			// 删除 mapContains 条件
			*whereSQL += fmt.Sprintf(" AND etrf.evaluator_score{'%s'} BETWEEN ? AND ?", f.Key)
			*args = append(*args, floatValue1, floatValue2)
		}
	}
	for _, f := range cond.MapCond.AnnotationFloatFilters {
		switch f.Op {
		case "=":
			floatValue, err := strconv.ParseFloat(fmt.Sprintf("%v", f.Values[0]), 64)
			if err != nil {
				logs.CtxError(context.Background(), "Parse float value failed: %v", err)
				continue
			}
			// 删除 mapContains 条件
			*whereSQL += fmt.Sprintf(" AND abs(etrf.annotation_float{'%s'} - ?) < %g", f.Key, floatEpsilon)
			*args = append(*args, floatValue)
		case ">", ">=", "<", "<=", "!=":
			floatValue, err := strconv.ParseFloat(fmt.Sprintf("%v", f.Values[0]), 64)
			if err != nil {
				logs.CtxError(context.Background(), "Parse float value failed: %v", err)
				continue
			}
			// 删除 mapContains 条件
			*whereSQL += fmt.Sprintf(" AND etrf.annotation_float{'%s'} %s ?", f.Key, f.Op)
			*args = append(*args, floatValue)
		case "BETWEEN":
			floatValue1, err1 := strconv.ParseFloat(fmt.Sprintf("%v", f.Values[0]), 64)
			floatValue2, err2 := strconv.ParseFloat(fmt.Sprintf("%v", f.Values[1]), 64)
			if err1 != nil || err2 != nil {
				logs.CtxError(context.Background(), "Parse float value failed: %v, %v", err1, err2)
				continue
			}
			// 删除 mapContains 条件
			*whereSQL += fmt.Sprintf(" AND etrf.annotation_float{'%s'} BETWEEN ? AND ?", f.Key)
			*args = append(*args, floatValue1, floatValue2)
		}
	}

	for _, f := range cond.MapCond.AnnotationStringFilters {
		switch f.Op {
		case "=":
			// 删除 mapContains 条件
			*whereSQL += fmt.Sprintf(" AND etrf.annotation_string{'%s'} = ?", f.Key)
			*args = append(*args, f.Values[0])
		case "LIKE":
			// 删除 mapContains 条件
			*whereSQL += fmt.Sprintf(" AND etrf.annotation_string{'%s'} LIKE ?", f.Key)
			*args = append(*args, "%"+escapeSpecialChars(fmt.Sprintf("%v", f.Values[0]))+"%")
		case "NOT LIKE":
			// 删除 mapContains 条件
			*whereSQL += fmt.Sprintf(" AND etrf.annotation_string{'%s'} NOT LIKE ?", f.Key)
			*args = append(*args, "%"+escapeSpecialChars(fmt.Sprintf("%v", f.Values[0]))+"%")
		case "!=":
			// 删除 mapContains 条件
			*whereSQL += fmt.Sprintf(" AND etrf.annotation_string{'%s'}!=?", f.Key)
			*args = append(*args, f.Values[0])

			// tag_value_id
		case "in", "IN":
			//*whereSQL += " AND etrf.annotation_string IN ?"
			*whereSQL += fmt.Sprintf(" AND etrf.annotation_string{'%s'} IN ?", f.Key)
			*args = append(*args, f.Values)
		case "NOT IN":
			//*whereSQL += " AND etrf.annotation_string NOT IN?"
			*whereSQL += fmt.Sprintf(" AND etrf.annotation_string{'%s'} NOT IN ?", f.Key)
			*args = append(*args, f.Values)
		}
	}
}

// buildItemSnapshotConditions 构建联表条件
func (d *exptTurnResultFilterDAOImpl) buildItemSnapshotConditions(cond *ExptTurnResultFilterQueryCond, joinSQL *string, args *[]interface{}) {
	if cond.ItemSnapshotCond == nil {
		return
	}
	for _, f := range cond.ItemSnapshotCond.FloatMapFilters {
		switch f.Op {
		case "=":
			floatValue, err := strconv.ParseFloat(fmt.Sprintf("%v", f.Values[0]), 64)
			if err != nil {
				logs.CtxError(context.Background(), "Parse float value failed: %v", err)
				continue
			}
			// 删除 mapContains 条件
			*joinSQL += fmt.Sprintf(" AND abs(dis.float_map{'%s'} - ?) < %g", f.Key, floatEpsilon)
			*args = append(*args, floatValue)
		case ">", ">=", "<", "<=", "!=":
			floatValue, err := strconv.ParseFloat(fmt.Sprintf("%v", f.Values[0]), 64)
			if err != nil {
				logs.CtxError(context.Background(), "Parse float value failed: %v", err)
				continue
			}
			// 删除 mapContains 条件
			*joinSQL += fmt.Sprintf(" AND dis.float_map{'%s'} %s ?", f.Key, f.Op)
			*args = append(*args, floatValue)
		case "BETWEEN":
			floatValue1, err1 := strconv.ParseFloat(fmt.Sprintf("%v", f.Values[0]), 64)
			floatValue2, err2 := strconv.ParseFloat(fmt.Sprintf("%v", f.Values[1]), 64)
			if err1 != nil || err2 != nil {
				logs.CtxError(context.Background(), "Parse float value failed: %v, %v", err1, err2)
				continue
			}
			// 删除 mapContains 条件
			*joinSQL += fmt.Sprintf(" AND dis.float_map{'%s'} BETWEEN ? AND ?", f.Key)
			*args = append(*args, floatValue1, floatValue2)
		}
	}
	// int_map
	for _, f := range cond.ItemSnapshotCond.IntMapFilters {
		switch f.Op {
		case "=", ">", ">=", "<", "<=", "!=":
			// 删除 mapContains 条件
			*joinSQL += fmt.Sprintf(" AND dis.int_map{'%s'} %s ?", f.Key, f.Op)
			*args = append(*args, f.Values[0])
		case "BETWEEN":
			// 删除 mapContains 条件
			*joinSQL += fmt.Sprintf(" AND dis.int_map{'%s'} BETWEEN ? AND ?", f.Key)
			*args = append(*args, f.Values[0], f.Values[1])
		}
	}
	// 处理 BoolMapFilters
	for _, f := range cond.ItemSnapshotCond.BoolMapFilters {
		switch f.Op {
		case "=":
			boolValue, err := strconv.ParseBool(fmt.Sprintf("%v", f.Values[0]))
			if err != nil {
				logs.CtxError(context.Background(), "Parse bool value failed: %v", err)
				continue
			}
			intBoolValue := 0
			if boolValue {
				intBoolValue = 1
			}
			// 删除 mapContains 条件
			*joinSQL += fmt.Sprintf(" AND dis.bool_map{'%s'} = ?", f.Key)
			*args = append(*args, intBoolValue)
		case "!=":
			boolValue, err := strconv.ParseBool(fmt.Sprintf("%v", f.Values[0]))
			if err != nil {
				logs.CtxError(context.Background(), "Parse bool value failed: %v", err)
				continue
			}
			intBoolValue := 0
			if boolValue {
				intBoolValue = 1
			}
			// 删除 mapContains 条件
			*joinSQL += fmt.Sprintf(" AND dis.bool_map{'%s'} != ?", f.Key)
			*args = append(*args, intBoolValue)
		default:
			logs.CtxWarn(context.Background(), "Unsupported operator %s for BoolMapFilters", f.Op)
		}
	}

	// string_map
	for _, f := range cond.ItemSnapshotCond.StringMapFilters {
		switch f.Op {
		case "LIKE":
			// 删除 mapContains 条件
			*joinSQL += fmt.Sprintf(" AND dis.string_map{'%s'} LIKE ?", f.Key)
			*args = append(*args, "%"+escapeSpecialChars(fmt.Sprintf("%v", f.Values[0]))+"%")
		case "=":
			// 删除 mapContains 条件
			*joinSQL += fmt.Sprintf(" AND dis.string_map{'%s'} = ?", f.Key)
			*args = append(*args, f.Values[0])
		case "NOT LIKE":
			// 删除 mapContains 条件
			*joinSQL += fmt.Sprintf(" AND dis.string_map{'%s'} NOT LIKE ?", f.Key)
			*args = append(*args, "%"+escapeSpecialChars(fmt.Sprintf("%v", f.Values[0]))+"%")
		case "!=":
			// 删除 mapContains 条件
			*joinSQL += fmt.Sprintf(" AND dis.string_map{'%s'}!=?", f.Key)
			*args = append(*args, f.Values[0])
		}
	}
}

// buildKeywordSearchConditions 构建全文搜索条件
func (d *exptTurnResultFilterDAOImpl) buildKeywordSearchConditions(ctx context.Context, cond *ExptTurnResultFilterQueryCond, keywordCond *string, args *[]interface{}) {
	if cond.KeywordSearch == nil || cond.KeywordSearch.Keyword == nil {
		return
	}

	kw := ptr.From(cond.KeywordSearch.Keyword)
	*keywordCond = " AND ("

	*keywordCond += "etrf.item_id = ?"
	*args = append(*args, kw)

	// 处理 EvalTargetDataFilters
	if len(cond.KeywordSearch.EvalTargetDataFilters) > 0 {
		for _, f := range cond.KeywordSearch.EvalTargetDataFilters {
			*keywordCond += " OR "
			// 删除 mapContains 条件
			*keywordCond += fmt.Sprintf("etrf.eval_target_data{'%s'} LIKE ?", f.Key)
			*args = append(*args, "%"+escapeSpecialChars(kw)+"%")
		}
	}

	// 处理 ItemSnapshotFilter
	if cond.KeywordSearch.ItemSnapshotFilter != nil {
		// float_map
		for _, f := range cond.KeywordSearch.ItemSnapshotFilter.FloatMapFilters {
			floatValue, err := strconv.ParseFloat(kw, 64)
			if err != nil {
				logs.CtxInfo(ctx, "Parse float value failed in keyword search: %v", err)
				continue
			}
			// 删除 mapContains 条件
			*keywordCond += " OR "
			*keywordCond += fmt.Sprintf("abs(dis.float_map{'%s'} - ?) < %g", f.Key, floatEpsilon)
			*args = append(*args, floatValue)
		}
		// int_map
		for _, f := range cond.KeywordSearch.ItemSnapshotFilter.IntMapFilters {
			intValue, err := strconv.ParseInt(kw, 10, 64)
			if err != nil {
				logs.CtxInfo(ctx, "Parse int value failed in keyword search: %v", err)
				continue
			}
			// 删除 mapContains 条件
			*keywordCond += " OR "
			*keywordCond += fmt.Sprintf("dis.int_map{'%s'} = ?", f.Key)
			*args = append(*args, intValue)
		}
		// string_map
		for _, f := range cond.KeywordSearch.ItemSnapshotFilter.StringMapFilters {
			*keywordCond += " OR "
			// 删除 mapContains 条件
			*keywordCond += fmt.Sprintf("dis.string_map{'%s'} LIKE ?", f.Key)
			*args = append(*args, "%"+escapeSpecialChars(kw)+"%")
		}
		// bool_map
		boolVal := 0
		switch kw {
		case "true":
			boolVal = 1
		case "false":
			boolVal = 0
		}
		if kw == "true" || kw == "false" {
			for _, f := range cond.KeywordSearch.ItemSnapshotFilter.BoolMapFilters {
				*keywordCond += " OR "
				// 删除 mapContains 条件
				*keywordCond += fmt.Sprintf("dis.bool_map{'%s'} = %d", f.Key, boolVal)
			}
		}
	}

	*keywordCond += ")"
}

// buildBaseSQL 构建基础SQL语句
func (d *exptTurnResultFilterDAOImpl) buildBaseSQL(ctx context.Context, joinSQL, whereSQL, keywordCond, evalSetSyncCkDate string, args *[]interface{}) string {
	sql := "SELECT  etrf.item_id, etrf.status FROM " + d.configer.GetCKDBName(ctx).ExptTurnResultFilterDBName + ".expt_turn_result_filter etrf"
	if joinSQL != "" || keywordCond != "" {
		sql += " INNER JOIN " + d.configer.GetCKDBName(ctx).DatasetItemsSnapshotDBName + ".dataset_item_snapshot dis ON etrf.eval_set_version_id = dis.version_id AND etrf.item_id = dis.item_id"
	}

	sql += " WHERE 1=1"

	if joinSQL != "" || keywordCond != "" {
		sql += " And dis.sync_ck_date = ?"
		// 将 evalSetSyncCkDate 插入到 args 切片的第一个位置
		newArgs := make([]interface{}, 0, len(*args)+1)
		newArgs = append(newArgs, evalSetSyncCkDate)
		newArgs = append(newArgs, *args...)
		*args = newArgs
	}
	if whereSQL != "" {
		sql += whereSQL
	}
	if joinSQL != "" {
		sql += joinSQL
	}
	if keywordCond != "" {
		sql += keywordCond
	}
	return sql
}

// getTotalCount 获取总记录数
func (d *exptTurnResultFilterDAOImpl) getTotalCount(ctx context.Context, sql string, args []interface{}) (int64, error) {
	countSQL := "SELECT COUNT(DISTINCT etrf.item_id) FROM (" + sql + ")"
	var total int64
	logs.CtxInfo(ctx, "Query count sql: %v, args: %v", countSQL, args)
	if err := d.db.NewSession(ctx).Raw(countSQL, args...).Scan(&total).Error; err != nil {
		logs.CtxError(ctx, "Query count failed: %v", err)
		return total, err
	}
	return total, nil
}

// addPaginationAndSorting 添加排序和分页，并更新参数列表
func (d *exptTurnResultFilterDAOImpl) addPaginationAndSorting(sql string, cond *ExptTurnResultFilterQueryCond, args []interface{}) (string, []interface{}) {
	sql += " ORDER BY etrf.item_idx"

	const defaultLimit = 20
	limit := defaultLimit
	offset := 0

	if cond.Page.Limit > 0 {
		limit = cond.Page.Limit
	}
	if cond.Page.Offset > 0 {
		offset = cond.Page.Offset
	}

	sql += " LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	return sql, args
}

// appendPaginationArgs 添加分页参数
func (d *exptTurnResultFilterDAOImpl) appendPaginationArgs(args []interface{}, cond *ExptTurnResultFilterQueryCond) []interface{} {
	const defaultLimit = 20
	offset := 0
	limit := defaultLimit
	if cond.Page.Offset > 0 {
		offset = cond.Page.Offset
	}
	if cond.Page.Limit > 0 {
		limit = cond.Page.Limit
	}
	args = append(args, limit, offset)
	return args
}

// executeQuery 执行查询
func (d *exptTurnResultFilterDAOImpl) executeQuery(ctx context.Context, sql string, args []interface{}, total int64) (map[string]int32, int64, error) {
	var results []map[string]interface{}
	logs.CtxInfo(ctx, "QueryItemIDStates sql: %v, args: %v", sql, args)
	if err := d.db.NewSession(ctx).Raw(sql, args...).Scan(&results).Error; err != nil {
		return nil, total, err
	}
	return parseOutput(ctx, results), total, nil
}

func parseOutput(ctx context.Context, results []map[string]interface{}) map[string]int32 {
	// 提取 item_id 和 status 字段，将 item_id 作为键，status 转换为 int32 后作为值
	filteredResults := make(map[string]int32)
	for _, result := range results {
		itemID, itemIDExists := result["item_id"]
		status, statusExists := result["status"]
		if itemIDExists && statusExists {
			// 将 itemID 转换为字符串
			itemIDStr, ok := itemID.(string)
			if !ok {
				logs.CtxError(ctx, "Failed to convert itemID to string")
				continue
			}
			// 将 status 转换为 int32
			var statusInt32 int32
			switch v := status.(type) {
			case int:
				statusInt32 = int32(v)
			case int64:
				statusInt32 = int32(v)
			case int32:
				statusInt32 = v
			default:
				logs.CtxError(ctx, "Failed to convert status to int32")
				continue
			}
			filteredResults[itemIDStr] = statusInt32
		}
	}
	return filteredResults
}

// GetByExptID 根据 spaceID 和 exptID 查询指定字段的数据
func (d *exptTurnResultFilterDAOImpl) GetByExptIDItemIDs(ctx context.Context, spaceID, exptID, createdDate string, itemIDs []string) ([]*compare_model.ExptTurnResultFilter, error) {
	sql, args := d.buildGetByExptIDItemIDsSQL(ctx, spaceID, exptID, createdDate, itemIDs)
	var results []*compare_model.ExptTurnResultFilter
	if err := d.db.NewSession(ctx).Raw(sql, args...).Scan(&results).Error; err != nil {
		logs.CtxError(ctx, "GetByExptID failed: %v", err)
		return nil, err
	}
	return results, nil
}

func (d *exptTurnResultFilterDAOImpl) buildGetByExptIDItemIDsSQL(ctx context.Context, spaceID, exptID, createdDate string, itemIDs []string) (string, []interface{}) {
	sql := "SELECT " +
		"etrf.space_id, " +
		"etrf.expt_id, " +
		"etrf.item_id, " +
		"etrf.item_idx, " +
		"etrf.turn_id, " +
		"etrf.status, " +
		"etrf.eval_set_version_id, " +
		"etrf.created_date, " +
		"etrf.eval_target_data{'actual_output'} as actual_output, " +
		"etrf.evaluator_score{'key1'} as evaluator_score_key_1, " +
		"etrf.evaluator_score{'key2'} as evaluator_score_key_2, " +
		"etrf.evaluator_score{'key3'} as evaluator_score_key_3, " +
		"etrf.evaluator_score{'key4'} as evaluator_score_key_4, " +
		"etrf.evaluator_score{'key5'} as evaluator_score_key_5, " +
		"etrf.evaluator_score{'key6'} as evaluator_score_key_6, " +
		"etrf.evaluator_score{'key7'} as evaluator_score_key_7, " +
		"etrf.evaluator_score{'key8'} as evaluator_score_key_8, " +
		"etrf.evaluator_score{'key9'} as evaluator_score_key_9, " +
		"etrf.evaluator_score{'key10'} as evaluator_score_key_10, " +
		"etrf.evaluator_score_corrected " +
		"FROM " + d.configer.GetCKDBName(ctx).ExptTurnResultFilterDBName + ".expt_turn_result_filter etrf " +
		"WHERE etrf.space_id = ? AND etrf.expt_id = ? AND etrf.created_date =?"
	if len(itemIDs) > 0 {
		sql += " AND etrf.item_id IN (?)"
	}

	args := []interface{}{spaceID, exptID, createdDate, itemIDs}
	logs.CtxInfo(ctx, "GetByExptID sql: %v, args: %v", sql, args)
	return sql, args
}
