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

	"github.com/coze-dev/coze-loop/backend/infra/ck"
	metrics_entity "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/ck/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/ck/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/dao"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/samber/lo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	TraceStorageTypeCK = "ck"

	// 人工标注标签
	AnnotationManualFeedbackFieldPrefix = "manual_feedback_"

	// 人工标注标签类型
	AnnotationManualFeedbackType = "manual_feedback"
)

func NewSpansCkDaoImpl(db ck.Provider) (dao.ISpansDao, error) {
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

func (s *SpansCkDaoImpl) Insert(ctx context.Context, param *dao.InsertParam) error {
	db := s.newSession(ctx)
	retryTimes := 3
	var lastErr error
	// 满足条件的批写入会保证幂等性；
	// 如果是网络问题导致错误, 重试可能会导致重复写入;
	// https://clickhouse.com/docs/guides/developer/transactional。
	spans := convertor.SpanListPO2CKModels(param.Spans)
	for i := 0; i < retryTimes; i++ {
		if err := db.Table(param.Table).Create(spans).Error; err != nil {
			logs.CtxError(ctx, "fail to insert spans, count %d, %v", len(spans), err)
			lastErr = err
		} else {
			return nil
		}
	}
	return lastErr
}

func (s *SpansCkDaoImpl) Get(ctx context.Context, param *dao.QueryParam) ([]*dao.Span, error) {
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
	for _, span := range spans {
		if span.SystemTagsString == nil {
			span.SystemTagsString = make(map[string]string)
		}
		span.SystemTagsString[loop_span.SpanFieldTenant] = "cozeloop" // tenant
	}
	return convertor.SpanListCKModels2PO(spans), nil
}

// select/inner_query/group_by/order_by/with_fill
var metricsSqlTemplate = `SELECT %s FROM (%s) %s %s`

func (s *SpansCkDaoImpl) GetMetrics(ctx context.Context, param *dao.GetMetricsParam) ([]map[string]any, error) {
	sql, err := s.buildMetricsSql(ctx, param)
	if err != nil {
		return nil, err
	}
	logs.CtxInfo(ctx, "Get Metrics SQL: %+v", sql)
	db := s.newSession(ctx)
	result := make([]map[string]any, 0)
	if err := db.Raw(sql).Find(&result).Error; err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonRPCErrorCodeCode)
	}
	return result, nil
}

func (s *SpansCkDaoImpl) buildMetricsSql(ctx context.Context, param *dao.GetMetricsParam) (string, error) {
	// 直接复用现有的SQL获取所有数据, 然后再计算指标
	sql, err := s.buildSql(ctx, &dao.QueryParam{
		Tables:           param.Tables,
		StartTime:        param.StartAt,
		EndTime:          param.EndAt,
		Filters:          param.Filters,
		Limit:            -1,
		OrderByStartTime: false,
		OmitColumns:      []string{"input", "output"}, // 当前不用, 先忽略
	})
	if err != nil {
		return "", errorx.WrapByCode(err, obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid build metric request"))
	}
	var (
		innerQuery     string
		selectClauses  []string
		groupByClauses []string
		orderByClauses []string
	)
	innerQuery = sql.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Find(nil)
	})
	if param.Granularity != "" {
		ckTimeInterval := getTimeInterval(param.Granularity)
		selectClauses = append(selectClauses,
			fmt.Sprintf("toUnixTimestamp(toStartOfInterval(fromUnixTimestamp64Micro(start_time), %s)) * 1000 AS time_bucket", ckTimeInterval))
		groupByClauses = append(groupByClauses, "time_bucket")
		orderByClauses = append(orderByClauses, "time_bucket") // 代码填充, 不在SQL中实现
	}
	for _, dimension := range param.Aggregations {
		expr, err := s.formatAggregationExpression(ctx, dimension)
		if err != nil {
			return "", err
		}
		selectClauses = append(selectClauses,
			fmt.Sprintf("%s AS %s", expr, dimension.Alias))
	}
	for _, dimension := range param.GroupBys {
		fieldName, err := s.convertFieldName(ctx, dimension.Field)
		if err != nil {
			return "", errorx.WrapByCode(err, obErrorx.CommercialCommonInvalidParamCodeCode)
		}
		selectClauses = append(selectClauses,
			fmt.Sprintf("%s AS %s", fieldName, dimension.Alias))
		groupByClauses = append(groupByClauses, fieldName)
	}
	wholeSql := fmt.Sprintf(metricsSqlTemplate,
		strings.Join(selectClauses, ", "),
		innerQuery,
		lo.Ternary(len(groupByClauses) == 0,
			"", "GROUP BY "+strings.Join(groupByClauses, ", ")),
		lo.Ternary(len(orderByClauses) == 0,
			"", "ORDER BY "+strings.Join(orderByClauses, ", ")),
	)
	return wholeSql, nil
}

func (s *SpansCkDaoImpl) formatAggregationExpression(ctx context.Context, dimension *metrics_entity.Dimension) (string, error) {
	if dimension.Expression == nil {
		return "", nil
	}
	replacements := make([]any, 0, len(dimension.Expression.Fields))
	for _, field := range dimension.Expression.Fields {
		if field == nil {
			continue
		}
		expr, err := s.convertFieldName(ctx, field)
		if err != nil {
			return "", err
		}
		replacements = append(replacements, expr)
	}
	return fmt.Sprintf(dimension.Expression.Expression, replacements...), nil
}

type buildSqlParam struct {
	spanTable     string
	annoTable     string
	queryParam    *dao.QueryParam
	db            *gorm.DB
	selectColumns []string
	omitColumns   []string
}

func (s *SpansCkDaoImpl) buildSql(ctx context.Context, param *dao.QueryParam) (*gorm.DB, error) {
	db := s.newSession(ctx)
	var tableQueries []*gorm.DB
	for _, table := range param.Tables {
		query, err := s.buildSingleSql(ctx, &buildSqlParam{
			spanTable:     table,
			annoTable:     param.AnnoTableMap[table],
			queryParam:    param,
			db:            db,
			selectColumns: param.SelectColumns,
			omitColumns:   param.OmitColumns,
		})
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
		if param.Limit >= 0 {
			sql += fmt.Sprintf(" LIMIT %d", param.Limit)
		}
		return db.Raw(sql), nil
	}
}

func (s *SpansCkDaoImpl) buildSingleSql(ctx context.Context, param *buildSqlParam) (*gorm.DB, error) {
	sqlQuery, err := s.buildSqlForFilterFields(ctx, param, param.queryParam.Filters)
	if err != nil {
		return nil, err
	}
	queryColumns := lo.Ternary(
		len(param.selectColumns) == 0,
		getColumnStr(spanColumns, param.omitColumns),
		getColumnStr(param.selectColumns, param.omitColumns),
	)
	sqlQuery = param.db.
		Table(param.spanTable).Select(queryColumns).
		Where(sqlQuery).
		Where("start_time >= ?", param.queryParam.StartTime).
		Where("start_time <= ?", param.queryParam.EndTime)
	if param.queryParam.OrderByStartTime {
		sqlQuery = sqlQuery.Order(clause.OrderBy{Columns: []clause.OrderByColumn{
			{Column: clause.Column{Name: "start_time"}, Desc: true},
			{Column: clause.Column{Name: "span_id"}, Desc: true},
		}})
	}
	sqlQuery = sqlQuery.Limit(int(param.queryParam.Limit))
	return sqlQuery, nil
}

// chain
func (s *SpansCkDaoImpl) buildSqlForFilterFields(ctx context.Context, param *buildSqlParam, filter *loop_span.FilterFields) (*gorm.DB, error) {
	if filter == nil {
		return param.db, nil
	}
	queryChain := param.db
	if filter.QueryAndOr != nil && *filter.QueryAndOr == loop_span.QueryAndOrEnumOr {
		for _, subFilter := range filter.FilterFields {
			if subFilter == nil {
				continue
			}
			subSql, err := s.buildSqlForFilterField(ctx, param, subFilter)
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
			subSql, err := s.buildSqlForFilterField(ctx, param, subFilter)
			if err != nil {
				return nil, err
			}
			queryChain = queryChain.Where(subSql)
		}
	}
	return queryChain, nil
}

func (s *SpansCkDaoImpl) buildSqlForFilterField(ctx context.Context, param *buildSqlParam, filter *loop_span.FilterField) (*gorm.DB, error) {
	queryChain := param.db
	if s.isAnnotationFilter(filter.FieldName) {
		annoSql, err := s.buildAnnotationSql(ctx, param, filter)
		if err != nil {
			return nil, fmt.Errorf("failed to build annotation sql: %v", err)
		}
		queryChain = queryChain.Where(annoSql)
	} else if filter.FieldName != "" {
		fieldName, err := s.convertFieldName(ctx, filter)
		if err != nil {
			return nil, err
		}
		sql, err := s.buildFieldCondition(ctx, param.db, &loop_span.FilterField{
			FieldName: fieldName,
			FieldType: filter.FieldType,
			Values:    filter.Values,
			QueryType: filter.QueryType,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to build field condition: %v", err)
		}
		queryChain = queryChain.Where(sql)
	}
	if filter.SubFilter != nil {
		subQuery, err := s.buildSqlForFilterFields(ctx, param, filter.SubFilter)
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

func (s *SpansCkDaoImpl) buildFieldCondition(ctx context.Context, db *gorm.DB, filter *loop_span.FilterField) (*gorm.DB, error) {
	queryChain := db
	if filter.QueryType == nil {
		return nil, fmt.Errorf("query type is required, not supposed to be here")
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
		queryChain = queryChain.Where(fmt.Sprintf("%s like ?", filter.FieldName), fmt.Sprintf("%%%v%%", fieldValues[0]))
	case loop_span.QueryTypeEnumNotMatch:
		if len(fieldValues) != 1 {
			return nil, fmt.Errorf("filter field %s should have one value", filter.FieldName)
		}
		queryChain = queryChain.Where(fmt.Sprintf("%s NOT like ?", filter.FieldName), fmt.Sprintf("%%%v%%", fieldValues[0]))
	case loop_span.QueryTypeEnumEq:
		if len(fieldValues) != 1 {
			return nil, fmt.Errorf("filter field %s should have one value", filter.FieldName)
		}
		queryChain = queryChain.Where(fmt.Sprintf("%s = ?", filter.FieldName), fieldValues[0])
	case loop_span.QueryTypeEnumNotEq:
		if len(fieldValues) != 1 {
			return nil, fmt.Errorf("filter field %s should have one value", filter.FieldName)
		}
		queryChain = queryChain.Where(fmt.Sprintf("%s != ?", filter.FieldName), fieldValues[0])
	case loop_span.QueryTypeEnumLte:
		if len(fieldValues) != 1 {
			return nil, fmt.Errorf("filter field %s should have one value", filter.FieldName)
		}
		queryChain = queryChain.Where(fmt.Sprintf("%s <= ?", filter.FieldName), fieldValues[0])
	case loop_span.QueryTypeEnumGte:
		if len(fieldValues) != 1 {
			return nil, fmt.Errorf("filter field %s should have one value", filter.FieldName)
		}
		queryChain = queryChain.Where(fmt.Sprintf("%s >= ?", filter.FieldName), fieldValues[0])
	case loop_span.QueryTypeEnumLt:
		if len(fieldValues) != 1 {
			return nil, fmt.Errorf("filter field %s should have one value", filter.FieldName)
		}
		queryChain = queryChain.Where(fmt.Sprintf("%s < ?", filter.FieldName), fieldValues[0])
	case loop_span.QueryTypeEnumGt:
		if len(fieldValues) != 1 {
			return nil, fmt.Errorf("filter field %s should have one value", filter.FieldName)
		}
		queryChain = queryChain.Where(fmt.Sprintf("%s > ?", filter.FieldName), fieldValues[0])
	case loop_span.QueryTypeEnumExist:
		defaultVal := getFieldDefaultValue(filter)
		queryChain = queryChain.
			Where(fmt.Sprintf("%s IS NOT NULL", filter.FieldName)).
			Where(fmt.Sprintf("%s != ?", filter.FieldName), defaultVal)
	case loop_span.QueryTypeEnumNotExist:
		defaultVal := getFieldDefaultValue(filter)
		queryChain = queryChain.
			Where(fmt.Sprintf("%s IS NULL", filter.FieldName)).
			Or(fmt.Sprintf("%s = ?", filter.FieldName), defaultVal)
	case loop_span.QueryTypeEnumIn:
		if len(fieldValues) < 1 {
			return nil, fmt.Errorf("filter field %s should have at least one value", filter.FieldName)
		}
		queryChain = queryChain.Where(fmt.Sprintf("%s IN (?)", filter.FieldName), fieldValues)
	case loop_span.QueryTypeEnumNotIn:
		if len(fieldValues) < 1 {
			return nil, fmt.Errorf("filter field %s should have at least one value", filter.FieldName)
		}
		queryChain = queryChain.Where(fmt.Sprintf("%s NOT IN (?)", filter.FieldName), fieldValues)
	case loop_span.QueryTypeEnumAlwaysTrue:
		queryChain = queryChain.Where("1 = 1")
	default:
		return nil, fmt.Errorf("filter field type %s not supported", filter.FieldType)
	}
	return queryChain, nil
}

func (s *SpansCkDaoImpl) isAnnotationFilter(fieldName string) bool {
	if strings.HasPrefix(fieldName, AnnotationManualFeedbackFieldPrefix) {
		return true
	} else {
		return false
	}
}

func (s *SpansCkDaoImpl) buildAnnotationSql(ctx context.Context, param *buildSqlParam, filter *loop_span.FilterField) (*gorm.DB, error) {
	queryChain := param.db
	fieldName := filter.FieldName
	if strings.HasPrefix(fieldName, AnnotationManualFeedbackFieldPrefix) {
		// manual_feedback_{tag_key_id}
		tagKeyId := fieldName[len(AnnotationManualFeedbackFieldPrefix):]
		if tagKeyId == "" {
			return nil, fmt.Errorf("invalid manual feedback field name %s", fieldName)
		}
		queryChain = queryChain.
			Where("annotation_type = ?", AnnotationManualFeedbackType).
			Where("key = ?", tagKeyId)
	} else {
		return nil, fmt.Errorf("field name %s not supported for annotation, not supposed to be here", fieldName)
	}
	if filter.QueryType != nil && *filter.QueryType != loop_span.QueryTypeEnumExist {
		condition := &loop_span.FilterField{
			FieldType: filter.FieldType,
			Values:    filter.Values,
			QueryType: filter.QueryType,
		}
		switch filter.FieldType {
		case loop_span.FieldTypeString:
			condition.FieldName = "value_string"
		case loop_span.FieldTypeLong:
			condition.FieldName = "value_long"
		case loop_span.FieldTypeDouble:
			condition.FieldName = "value_float"
		case loop_span.FieldTypeBool:
			condition.FieldName = "value_bool"
		default:
			return nil, fmt.Errorf("field type %s not supported", filter.FieldType)
		}
		fieldSql, err := s.buildFieldCondition(ctx, param.db, condition)
		if err != nil {
			return nil, err
		}
		queryChain = queryChain.Where(fieldSql)
	}
	_ = param.queryParam.Filters.Traverse(func(f *loop_span.FilterField) error {
		if f.FieldName == loop_span.SpanFieldSpaceId {
			commonSql, err := s.buildFieldCondition(ctx, param.db, f)
			if err != nil {
				return err
			}
			queryChain = queryChain.Where(commonSql)
		}
		return nil
	})
	subsql := param.db.
		Table(param.annoTable).
		Select("span_id").
		Where(queryChain).
		Where("deleted_at = 0").
		Where("start_time >= ?", param.queryParam.StartTime).
		Where("start_time <= ?", param.queryParam.EndTime)
	query := subsql.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Find(nil)
	})
	return param.db.Where("span_id in (?)", param.db.Raw(query+" SETTINGS final = 1")), nil
}

func (s *SpansCkDaoImpl) getSuperFieldsMap(ctx context.Context) map[string]bool {
	return defSuperFieldsMap
}

// convertFieldName IsCustom > IsSystem > superField, default custom
func (s *SpansCkDaoImpl) convertFieldName(ctx context.Context, filter *loop_span.FilterField) (string, error) {
	if !isSafeColumnName(filter.FieldName) {
		return "", fmt.Errorf("filter field name %s is not safe", filter.FieldName)
	}
	if filter.IsCustom {
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
	if filter.IsSystem {
		switch filter.FieldType {
		case loop_span.FieldTypeString:
			return fmt.Sprintf("system_tags_string['%s']", filter.FieldName), nil
		case loop_span.FieldTypeLong:
			return fmt.Sprintf("system_tags_long['%s']", filter.FieldName), nil
		case loop_span.FieldTypeDouble:
			return fmt.Sprintf("system_tags_float['%s']", filter.FieldName), nil
		default: // not expected to be here
			return fmt.Sprintf("system_tags_string['%s']", filter.FieldName), nil
		}
	}
	superFieldsMap := s.getSuperFieldsMap(ctx)
	if superFieldsMap[filter.FieldName] {
		return quoteSQLName(filter.FieldName), nil
	}
	switch filter.FieldType {
	case loop_span.FieldTypeString:
		if filter.IsSystem {
			return fmt.Sprintf("system_tags_string['%s']", filter.FieldName), nil
		} else {
			return fmt.Sprintf("tags_string['%s']", filter.FieldName), nil
		}
	case loop_span.FieldTypeLong:
		if filter.IsSystem {
			return fmt.Sprintf("system_tags_long['%s']", filter.FieldName), nil
		} else {
			return fmt.Sprintf("tags_long['%s']", filter.FieldName), nil
		}
	case loop_span.FieldTypeDouble:
		if filter.IsSystem {
			return fmt.Sprintf("system_tags_double['%s']", filter.FieldName), nil
		} else {
			return fmt.Sprintf("tags_float['%s']", filter.FieldName), nil
		}
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

var spanColumns = []string{
	"start_time",
	"logid",
	"span_id",
	"trace_id",
	"parent_id",
	"duration",
	"psm",
	"call_type",
	"space_id",
	"span_type",
	"span_name",
	"method",
	"status_code",
	"input",
	"output",
	"object_storage",
	"system_tags_string",
	"system_tags_long",
	"system_tags_float",
	"tags_string",
	"tags_long",
	"tags_bool",
	"tags_float",
	"tags_byte",
	"reserve_create_time",
	"logic_delete_date",
}

var validColumnRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_.]*$`)

func isSafeColumnName(name string) bool {
	return validColumnRegex.MatchString(name)
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

func getTimeInterval(granularity metrics_entity.MetricGranularity) string {
	switch granularity {
	case metrics_entity.MetricGranularity1Min:
		return "INTERVAL 1 MINUTE"
	case metrics_entity.MetricGranularity1Hour:
		return "INTERVAL 1 HOUR"
	case metrics_entity.MetricGranularity1Day, metrics_entity.MetricGranularity1Week:
		return "INTERVAL 1 DAY"
	default:
		return "INTERVAL 1 DAY"
	}
}
