// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package ck

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/coze-dev/coze-loop/backend/infra/ck"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/ck/gorm_gen/model"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

const (
	QueryTypeGetTrace  = "get_trace"
	QueryTypeListSpans = "list_spans"
)

type QueryParam struct {
	QueryType        string // for sql optimization
	Tables           []string
	AnnoTableMap     map[string]string
	StartTime        int64 // us
	EndTime          int64 // us
	Filters          *loop_span.FilterFields
	Limit            int32
	OrderByStartTime bool
	OmitColumns      []string // omit specific columns
}

type InsertParam struct {
	Table string
	Spans []*model.ObservabilitySpan
}

//go:generate mockgen -destination=mocks/spans_dao.go -package=mocks . ISpansDao
type ISpansDao interface {
	Insert(context.Context, *InsertParam) error
	Get(context.Context, *QueryParam) ([]*model.ObservabilitySpan, error)
	GetMetrics(ctx context.Context, param *GetMetricsParam) ([]map[string]any, error)
}

// GetMetricsParam 指标查询参数
type GetMetricsParam struct {
	Tables       []string
	Aggregations []*Dimension
	GroupBys     []*Dimension
	Filters      *loop_span.FilterFields
	StartAt      int64
	EndAt        int64
	Granularity  string
}

// Dimension 维度定义
type Dimension struct {
	Expression string // 字段名或表达式
	Alias      string // 别名
}

func NewSpansCkDaoImpl(db ck.Provider) (ISpansDao, error) {
	return &SpansCkDaoImpl{
		db: db,
	}, nil
}

type SpansCkDaoImpl struct {
	db ck.Provider
}

func (s *SpansCkDaoImpl) newSession(ctx context.Context) *gorm.DB {
	return s.db.NewSession(ctx)
}

func (s *SpansCkDaoImpl) Insert(ctx context.Context, param *InsertParam) error {
	db := s.newSession(ctx)
	retryTimes := 3
	var lastErr error
	// 满足条件的批写入会保证幂等性；
	// 如果是网络问题导致错误, 重试可能会导致重复写入;
	// https://clickhouse.com/docs/guides/developer/transactional。
	for i := 0; i < retryTimes; i++ {
		if err := db.Table(param.Table).Create(param.Spans).Error; err != nil {
			logs.CtxError(ctx, "fail to insert spans, count %d, %v", len(param.Spans), err)
			lastErr = err
		} else {
			return nil
		}
	}
	return lastErr
}

func (s *SpansCkDaoImpl) Get(ctx context.Context, param *QueryParam) ([]*model.ObservabilitySpan, error) {
	sql, err := s.buildSql(ctx, param)
	if err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid get trace request"))
	}
	logs.CtxInfo(ctx, "Get Trace SQL: %s", sql.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Find(nil)
	}))
	spans := make([]*model.ObservabilitySpan, 0)
	if err := sql.Find(&spans).Error; err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonRPCErrorCodeCode)
	}
	return spans, nil
}

func (s *SpansCkDaoImpl) buildSql(ctx context.Context, param *QueryParam) (*gorm.DB, error) {
	db := s.newSession(ctx)
	var tableQueries []*gorm.DB
	for _, table := range param.Tables {
		query, err := s.buildSingleSql(ctx, db, table, param)
		if err != nil {
			return nil, err
		}
		tableQueries = append(tableQueries, query)
	}
	if len(tableQueries) == 0 {
		return nil, fmt.Errorf("not table configured")
	} else if len(tableQueries) == 1 {
		return tableQueries[0], nil
	} else {
		queries := make([]string, 0)
		for i := 0; i < len(tableQueries); i++ {
			query := tableQueries[i].ToSQL(func(tx *gorm.DB) *gorm.DB {
				return tx.Find(nil)
			})
			queries = append(queries, "("+query+")")
		}
		sql := fmt.Sprintf("SELECT * FROM (%s)", strings.Join(queries, " UNION ALL "))
		if param.OrderByStartTime {
			sql += " ORDER BY start_time DESC, span_id DESC"
		}
		sql += fmt.Sprintf(" LIMIT %d", param.Limit)
		return db.Raw(sql), nil
	}
}

func (s *SpansCkDaoImpl) buildSingleSql(ctx context.Context, db *gorm.DB, tableName string, param *QueryParam) (*gorm.DB, error) {
	sqlQuery, err := s.buildSqlForFilterFields(ctx, db, param.Filters)
	if err != nil {
		return nil, err
	}
	sqlQuery = db.
		Table(tableName).
		Where(sqlQuery).
		Where("start_time >= ?", param.StartTime).
		Where("start_time <= ?", param.EndTime)
	if param.OrderByStartTime {
		sqlQuery = sqlQuery.Order(clause.OrderBy{Columns: []clause.OrderByColumn{
			{Column: clause.Column{Name: "start_time"}, Desc: true},
			{Column: clause.Column{Name: "span_id"}, Desc: true},
		}})
	}
	sqlQuery = sqlQuery.Limit(int(param.Limit))
	return sqlQuery, nil
}

// chain
func (s *SpansCkDaoImpl) buildSqlForFilterFields(ctx context.Context, db *gorm.DB, filter *loop_span.FilterFields) (*gorm.DB, error) {
	if filter == nil {
		return db, nil
	}
	queryChain := db
	if filter.QueryAndOr != nil && *filter.QueryAndOr == loop_span.QueryAndOrEnumOr {
		for _, subFilter := range filter.FilterFields {
			if subFilter == nil {
				continue
			}
			subSql, err := s.buildSqlForFilterField(ctx, db, subFilter)
			if err != nil {
				return nil, err
			}
			queryChain = queryChain.Or(subSql)
		}
	} else {
		for _, subFilter := range filter.FilterFields {
			if subFilter == nil {
				continue
			}
			subSql, err := s.buildSqlForFilterField(ctx, db, subFilter)
			if err != nil {
				return nil, err
			}
			queryChain = queryChain.Where(subSql)
		}
	}
	return queryChain, nil
}

func (s *SpansCkDaoImpl) buildSqlForFilterField(ctx context.Context, db *gorm.DB, filter *loop_span.FilterField) (*gorm.DB, error) {
	queryChain := db
	if filter.FieldName != "" {
		if filter.QueryType == nil {
			return nil, fmt.Errorf("query type is required, not supposed to be here")
		}
		fieldName, err := s.convertFieldName(ctx, filter)
		if err != nil {
			return nil, err
		}
		fieldValues, err := convertFieldValue(filter)
		if err != nil {
			return nil, err
		}
		switch *filter.QueryType {
		case loop_span.QueryTypeEnumMatch:
			if len(fieldValues) != 1 {
				return nil, fmt.Errorf("filter field %s should have one value", filter.FieldName)
			}
			queryChain = queryChain.Where(fmt.Sprintf("%s like ?", fieldName), fmt.Sprintf("%%%v%%", fieldValues[0]))
		case loop_span.QueryTypeEnumEq:
			if len(fieldValues) != 1 {
				return nil, fmt.Errorf("filter field %s should have one value", filter.FieldName)
			}
			queryChain = queryChain.Where(fmt.Sprintf("%s = ?", fieldName), fieldValues[0])
		case loop_span.QueryTypeEnumNotEq:
			if len(fieldValues) != 1 {
				return nil, fmt.Errorf("filter field %s should have one value", filter.FieldName)
			}
			queryChain = queryChain.Where(fmt.Sprintf("%s != ?", fieldName), fieldValues[0])
		case loop_span.QueryTypeEnumLte:
			if len(fieldValues) != 1 {
				return nil, fmt.Errorf("filter field %s should have one value", filter.FieldName)
			}
			queryChain = queryChain.Where(fmt.Sprintf("%s <= ?", fieldName), fieldValues[0])
		case loop_span.QueryTypeEnumGte:
			if len(fieldValues) != 1 {
				return nil, fmt.Errorf("filter field %s should have one value", filter.FieldName)
			}
			queryChain = queryChain.Where(fmt.Sprintf("%s >= ?", fieldName), fieldValues[0])
		case loop_span.QueryTypeEnumLt:
			if len(fieldValues) != 1 {
				return nil, fmt.Errorf("filter field %s should have one value", filter.FieldName)
			}
			queryChain = queryChain.Where(fmt.Sprintf("%s < ?", fieldName), fieldValues[0])
		case loop_span.QueryTypeEnumGt:
			if len(fieldValues) != 1 {
				return nil, fmt.Errorf("filter field %s should have one value", filter.FieldName)
			}
			queryChain = queryChain.Where(fmt.Sprintf("%s > ?", fieldName), fieldValues[0])
		case loop_span.QueryTypeEnumExist:
			defaultVal := getFieldDefaultValue(filter)
			queryChain = queryChain.
				Where(fmt.Sprintf("%s IS NOT NULL", fieldName)).
				Where(fmt.Sprintf("%s != ?", fieldName), defaultVal)
		case loop_span.QueryTypeEnumNotExist:
			defaultVal := getFieldDefaultValue(filter)
			queryChain = queryChain.
				Where(fmt.Sprintf("%s IS NULL", fieldName)).
				Or(fmt.Sprintf("%s = ?", fieldName), defaultVal)
		case loop_span.QueryTypeEnumIn:
			if len(fieldValues) < 1 {
				return nil, fmt.Errorf("filter field %s should have at least one value", filter.FieldName)
			}
			queryChain = queryChain.Where(fmt.Sprintf("%s IN (?)", fieldName), fieldValues)
		case loop_span.QueryTypeEnumNotIn:
			if len(fieldValues) < 1 {
				return nil, fmt.Errorf("filter field %s should have at least one value", filter.FieldName)
			}
			queryChain = queryChain.Where(fmt.Sprintf("%s NOT IN (?)", fieldName), fieldValues)
		case loop_span.QueryTypeEnumAlwaysTrue:
			queryChain = queryChain.Where("1 = 1")
		default:
			return nil, fmt.Errorf("filter field type %s not supported", filter.FieldType)
		}
	}
	if filter.SubFilter != nil {
		subQuery, err := s.buildSqlForFilterFields(ctx, db, filter.SubFilter)
		if err != nil {
			return nil, err
		}
		if filter.QueryAndOr != nil && *filter.QueryAndOr == loop_span.QueryAndOrEnumOr {
			queryChain = queryChain.Or(subQuery)
		} else {
			queryChain = queryChain.Where(subQuery)
		}
	}
	return queryChain, nil
}

func (s *SpansCkDaoImpl) getSuperFieldsMap(ctx context.Context) map[string]bool {
	return defSuperFieldsMap
}

func (s *SpansCkDaoImpl) convertFieldName(ctx context.Context, filter *loop_span.FilterField) (string, error) {
	if !isSafeColumnName(filter.FieldName) {
		return "", fmt.Errorf("filter field name %s is not safe", filter.FieldName)
	}
	superFieldsMap := s.getSuperFieldsMap(ctx)
	if superFieldsMap[filter.FieldName] {
		return quoteSQLName(filter.FieldName), nil
	}
	switch filter.FieldType {
	case loop_span.FieldTypeString:
		return fmt.Sprintf("tags_string['%s']", filter.FieldName), nil
	case loop_span.FieldTypeLong:
		return fmt.Sprintf("tags_long['%s']", filter.FieldName), nil
	case loop_span.FieldTypeDouble:
		return fmt.Sprintf("tags_float['%s']", filter.FieldName), nil
	case loop_span.FieldTypeBool:
		return fmt.Sprintf("tags_bool['%s']", filter.FieldName), nil
	default: // not expected to be here
		return fmt.Sprintf("tags_string['%s']", filter.FieldName), nil
	}
}

func convertFieldValue(filter *loop_span.FilterField) ([]any, error) {
	switch filter.FieldType {
	case loop_span.FieldTypeString:
		ret := make([]any, len(filter.Values))
		for i, v := range filter.Values {
			ret[i] = v
		}
		return ret, nil
	case loop_span.FieldTypeLong:
		ret := make([]any, len(filter.Values))
		for i, v := range filter.Values {
			num, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("fail to convert field value %v to int64", v)
			}
			ret[i] = num
		}
		return ret, nil
	case loop_span.FieldTypeDouble:
		ret := make([]any, len(filter.Values))
		for i, v := range filter.Values {
			num, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return nil, fmt.Errorf("fail to convert field value %v to float64", v)
			}
			ret[i] = num
		}
		return ret, nil
	case loop_span.FieldTypeBool:
		ret := make([]any, len(filter.Values))
		for i, value := range filter.Values {
			if value == "true" {
				ret[i] = 1
			} else {
				ret[i] = 0
			}
		}
		return ret, nil
	default:
		ret := make([]any, len(filter.Values))
		for i, v := range filter.Values {
			ret[i] = v
		}
		return ret, nil
	}
}

func getFieldDefaultValue(filter *loop_span.FilterField) any {
	switch filter.FieldType {
	case loop_span.FieldTypeString:
		return ""
	case loop_span.FieldTypeLong:
		return int64(0)
	case loop_span.FieldTypeDouble:
		return float64(0)
	case loop_span.FieldTypeBool:
		return int64(0)
	default:
		return ""
	}
}

func quoteSQLName(data string) string {
	buf := bytes.NewBuffer(nil)
	buf.WriteByte('`')
	for _, c := range data {
		switch c {
		case '`':
			buf.WriteString("``")
		case '.':
			buf.WriteString("`.`")
		default:
			buf.WriteRune(c)
		}
	}
	buf.WriteByte('`')
	return buf.String()
}

var defSuperFieldsMap = map[string]bool{
	loop_span.SpanFieldStartTime:       true,
	loop_span.SpanFieldSpanId:          true,
	loop_span.SpanFieldTraceId:         true,
	loop_span.SpanFieldParentID:        true,
	loop_span.SpanFieldDuration:        true,
	loop_span.SpanFieldCallType:        true,
	loop_span.SpanFieldPSM:             true,
	loop_span.SpanFieldLogID:           true,
	loop_span.SpanFieldSpaceId:         true,
	loop_span.SpanFieldSpanType:        true,
	loop_span.SpanFieldSpanName:        true,
	loop_span.SpanFieldMethod:          true,
	loop_span.SpanFieldStatusCode:      true,
	loop_span.SpanFieldInput:           true,
	loop_span.SpanFieldOutput:          true,
	loop_span.SpanFieldObjectStorage:   true,
	loop_span.SpanFieldLogicDeleteDate: true,
}
var validColumnRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func isSafeColumnName(name string) bool {
	return validColumnRegex.MatchString(name)
}

// GetMetrics 获取指标数据
func (s *SpansCkDaoImpl) GetMetrics(ctx context.Context, param *GetMetricsParam) ([]map[string]any, error) {
	query, err := s.buildMetricsGormSQL(ctx, param)
	if err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid get metrics request"))
	}

	logs.CtxInfo(ctx, "Get Metrics SQL: %s", query.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Find(nil)
	}))

	var results []map[string]any
	if err := query.Scan(&results).Error; err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonRPCErrorCodeCode)
	}

	return results, nil
}

// buildMetricsGormSQL 构建指标查询的GORM对象
func (s *SpansCkDaoImpl) buildMetricsGormSQL(ctx context.Context, param *GetMetricsParam) (*gorm.DB, error) {
	if len(param.Tables) == 0 {
		return nil, fmt.Errorf("tables cannot be empty")
	}

	db := s.newSession(ctx)

	// 构建SELECT子句
	selectClause := s.buildSelectClause(param)

	// 构建FROM子句
	fromClause := s.buildFromClause(param.Tables)

	// 构建WHERE条件
	query := db.Select(selectClause).Table(fromClause)

	// 添加WHERE条件
	whereQuery, err := s.buildMetricsWhereClause(ctx, query, param)
	if err != nil {
		return nil, err
	}
	query = whereQuery

	// 构建GROUP BY子句
	groupByClause := s.buildGroupByClause(param)
	if groupByClause != "" {
		query = query.Group(groupByClause)
	}

	// 构建ORDER BY子句（时间序列需要）
	if param.Granularity != "" {
		orderByClause := s.buildOrderByClause(param)
		if orderByClause != "" {
			query = query.Order(orderByClause)
		}
	}

	return query, nil
}

// buildSelectClause 构建SELECT子句
func (s *SpansCkDaoImpl) buildSelectClause(param *GetMetricsParam) string {
	var selectFields []string

	// 添加时间分组（如果有granularity）
	if param.Granularity != "" {
		timeInterval := s.getTimeInterval(param.Granularity)
		selectFields = append(selectFields, fmt.Sprintf("toStartOfInterval(fromUnixTimestamp64Micro(start_time), INTERVAL %s) AS time_bucket", timeInterval))
	}

	// 添加聚合字段
	for _, agg := range param.Aggregations {
		if agg.Alias != "" {
			selectFields = append(selectFields, fmt.Sprintf("%s AS %s", agg.Expression, agg.Alias))
		} else {
			selectFields = append(selectFields, agg.Expression)
		}
	}

	// 添加分组字段
	for _, groupBy := range param.GroupBys {
		if groupBy.Alias != "" {
			selectFields = append(selectFields, fmt.Sprintf("%s AS %s", groupBy.Expression, groupBy.Alias))
		} else {
			selectFields = append(selectFields, groupBy.Expression)
		}
	}

	return strings.Join(selectFields, ", ")
}

// buildFromClause 构建FROM子句
func (s *SpansCkDaoImpl) buildFromClause(tables []string) string {
	if len(tables) == 1 {
		return tables[0]
	}

	// 多表联合查询
	var unionQueries []string
	for _, table := range tables {
		unionQueries = append(unionQueries, fmt.Sprintf("SELECT * FROM %s", table))
	}
	return fmt.Sprintf("(%s)", strings.Join(unionQueries, " UNION ALL "))
}

// buildMetricsWhereClause 构建WHERE条件
func (s *SpansCkDaoImpl) buildMetricsWhereClause(ctx context.Context, db *gorm.DB, param *GetMetricsParam) (*gorm.DB, error) {
	query := db

	// 添加时间范围条件
	if param.StartAt > 0 && param.EndAt > 0 {
		query = query.Where("start_time >= ?", param.StartAt).Where("start_time <= ?", param.EndAt)
	}

	// 复用现有过滤逻辑
	if param.Filters != nil {
		filterQuery, err := s.buildSqlForFilterFields(ctx, query, param.Filters)
		if err != nil {
			return nil, err
		}
		query = filterQuery
	}

	return query, nil
}

// buildGroupByClause 构建GROUP BY子句
func (s *SpansCkDaoImpl) buildGroupByClause(param *GetMetricsParam) string {
	var groupBys []string

	// 添加时间分组
	if param.Granularity != "" {
		groupBys = append(groupBys, "time_bucket")
	}

	// 添加其他分组字段
	for _, groupBy := range param.GroupBys {
		if groupBy.Alias != "" {
			groupBys = append(groupBys, groupBy.Alias)
		} else {
			groupBys = append(groupBys, groupBy.Expression)
		}
	}

	return strings.Join(groupBys, ", ")
}

// buildOrderByClause 构建ORDER BY子句
func (s *SpansCkDaoImpl) buildOrderByClause(param *GetMetricsParam) string {
	if param.Granularity != "" {
		// 对于时间序列查询，按时间桶排序
		// 注意：ClickHouse的WITH FILL需要在原生SQL中处理，这里先简化
		return "time_bucket"
	}
	return ""
}



func (s *SpansCkDaoImpl) getTimeInterval(granularity string) string {
	switch granularity {
	case "1min":
		return "1 MINUTE"
	case "5min":
		return "5 MINUTE"
	case "15min":
		return "15 MINUTE"
	case "1hour":
		return "1 HOUR"
	case "1day":
		return "1 DAY"
	default:
		return "5 MINUTE" // 默认5分钟
	}
}