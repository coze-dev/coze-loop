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
