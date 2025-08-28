// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package loop_span

import (
	"cmp"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type (
	QueryAndOrEnum string
	QueryTypeEnum  string
	FieldType      string
	PlatformType   string
	SpanListType   string
)

const (
	QueryTypeEnumMatch    QueryTypeEnum = "match"
	QueryTypeEnumEq       QueryTypeEnum = "eq"
	QueryTypeEnumNotEq    QueryTypeEnum = "not_eq"
	QueryTypeEnumLte      QueryTypeEnum = "lte"
	QueryTypeEnumGte      QueryTypeEnum = "gte"
	QueryTypeEnumLt       QueryTypeEnum = "lt"
	QueryTypeEnumGt       QueryTypeEnum = "gt"
	QueryTypeEnumExist    QueryTypeEnum = "exist"
	QueryTypeEnumNotExist QueryTypeEnum = "not_exist"
	QueryTypeEnumIn       QueryTypeEnum = "in"
	QueryTypeEnumNotIn    QueryTypeEnum = "not_in"

	QueryTypeEnumAlwaysTrue QueryTypeEnum = "always_true" // 永远为真的条件, 为了保持语意一致还是需要放入SQL

	QueryAndOrEnumAnd QueryAndOrEnum = "and"
	QueryAndOrEnumOr  QueryAndOrEnum = "or"

	FieldTypeString FieldType = "string"
	FieldTypeLong   FieldType = "long"
	FieldTypeDouble FieldType = "double"
	FieldTypeBool   FieldType = "bool"

	PlatformCozeLoop   PlatformType = "cozeloop"
	PlatformPrompt     PlatformType = "prompt"
	PlatformEvaluator  PlatformType = "evaluator"
	PlatformEvalTarget PlatformType = "evaluation_target"
	PlatformOpenAPI    PlatformType = "open_api"

	SpanListTypeRootSpan SpanListType = "root_span"
	SpanListTypeAllSpan  SpanListType = "all_span"
	SpanListTypeLLMSpan  SpanListType = "llm_span"
)

var validFieldComb = map[FieldType]map[QueryTypeEnum]bool{
	FieldTypeString: {
		QueryTypeEnumMatch:    true,
		QueryTypeEnumIn:       true,
		QueryTypeEnumNotIn:    true,
		QueryTypeEnumExist:    true,
		QueryTypeEnumNotExist: true,
		QueryTypeEnumEq:       true,
		QueryTypeEnumNotEq:    true,
	},
	FieldTypeLong: {
		QueryTypeEnumGte:      true,
		QueryTypeEnumLte:      true,
		QueryTypeEnumGt:       true,
		QueryTypeEnumLt:       true,
		QueryTypeEnumIn:       true,
		QueryTypeEnumNotIn:    true,
		QueryTypeEnumExist:    true,
		QueryTypeEnumNotExist: true,
		QueryTypeEnumEq:       true,
		QueryTypeEnumNotEq:    true,
	},
	FieldTypeDouble: {
		QueryTypeEnumGte:      true,
		QueryTypeEnumLte:      true,
		QueryTypeEnumGt:       true,
		QueryTypeEnumLt:       true,
		QueryTypeEnumIn:       true,
		QueryTypeEnumNotIn:    true,
		QueryTypeEnumExist:    true,
		QueryTypeEnumNotExist: true,
		QueryTypeEnumEq:       true,
		QueryTypeEnumNotEq:    true,
	},
	FieldTypeBool: {
		QueryTypeEnumEq: true,
	},
}

type FieldOptions struct {
	I64List    []int64   `mapstructure:"i64_list" json:"i64_list"`
	F64List    []float64 `mapstructure:"f64_list" json:"f64_list"`
	StringList []string  `mapstructure:"string_list" json:"string_list"`
}

type FilterObject interface {
	GetFieldValue(fieldName string) any
}

type FilterFields struct {
	QueryAndOr   *QueryAndOrEnum `mapstructure:"query_and_or" json:"query_and_or"`
	FilterFields []*FilterField  `mapstructure:"filter_fields" json:"filter_fields"`
}

func (f *FilterFields) Validate() error {
	if f.QueryAndOr != nil &&
		*f.QueryAndOr != QueryAndOrEnumOr &&
		*f.QueryAndOr != QueryAndOrEnumAnd {
		return fmt.Errorf("invalid query and/or fields: %s", *f.QueryAndOr)
	}
	for _, filter := range f.FilterFields {
		if err := filter.Validate(); err != nil {
			return fmt.Errorf("invalid sub filter: %v", err)
		}
	}
	return nil
}

func (f *FilterFields) Traverse(fn func(f *FilterField) error) error {
	if f == nil {
		return nil
	}
	for _, filter := range f.FilterFields {
		if filter == nil {
			continue
		}
		if err := fn(filter); err != nil {
			return err
		}
		if filter.SubFilter != nil {
			if err := filter.SubFilter.Traverse(fn); err != nil {
				return err
			}
		}
	}
	return nil
}

//func (f *FilterFields) Filter[T FilterObject](objs []T) []T  {
//
//}

func (f *FilterFields) Satisfied(obj FilterObject) bool {
	op := QueryAndOrEnumAnd
	hit := true
	if f.QueryAndOr != nil && *f.QueryAndOr == QueryAndOrEnumOr {
		op = QueryAndOrEnumOr
		hit = false
	}
	for _, filter := range f.FilterFields {
		satisfied := filter.Satisfied(obj)
		if op == QueryAndOrEnumAnd {
			if !satisfied {
				return false
			}
		} else {
			if satisfied {
				return true
			}
		}
	}
	return hit
}

func (f *FilterFields) Debug() string {
	out, _ := json.Marshal(f)
	return string(out)
}

type FilterField struct {
	FieldName  string          `mapstructure:"field_name" json:"field_name"`
	FieldType  FieldType       `mapstructure:"field_type" json:"field_type"`
	Values     []string        `mapstructure:"values" json:"values"`
	QueryType  *QueryTypeEnum  `mapstructure:"query_type" json:"query_type"`
	QueryAndOr *QueryAndOrEnum `mapstructure:"query_and_or" json:"query_and_or"`
	SubFilter  *FilterFields   `mapstructure:"sub_filter" json:"sub_filter"`
	IsSystem   bool            `mapstructure:"is_system" json:"is_system"`
}

func (f *FilterField) Validate() error {
	if err := f.ValidateField(); err != nil {
		return err
	}
	if f.SubFilter != nil {
		if err := f.SubFilter.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (f *FilterField) ValidateField() error {
	if f.FieldName == "" {
		return nil
	}
	// should not be nil
	if f.QueryType == nil {
		return fmt.Errorf("query type not specified")
	}
	exist, ok := validFieldComb[f.FieldType][*f.QueryType]
	if !ok || !exist {
		return fmt.Errorf("invalid field type %s with query type: %s", f.FieldType, *f.QueryType)
	}
	// try to parse values
	switch f.FieldType {
	case FieldTypeString:
	// pass
	case FieldTypeLong:
		for _, value := range f.Values {
			_, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid value %s for type long", value)
			}
		}
	case FieldTypeDouble:
		for _, value := range f.Values {
			_, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return fmt.Errorf("invalid value %s for type double", value)
			}
		}
	case FieldTypeBool:
		for _, value := range f.Values {
			_, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("invalid value %s for type bool", value)
			}
		}
	}
	return nil
}

func (f *FilterField) Satisfied(obj FilterObject) bool {
	// 检测是否满足筛选条件
	if f.FieldName != "" {
		// 不满足field过滤条件
		if !f.CheckValue(obj.GetFieldValue(f.FieldName)) {
			return false
		}
	}
	if f.SubFilter != nil {
		if !f.SubFilter.Satisfied(obj) {
			return false
		}
	}
	return true
}

// 当前支持特定类型, 满足可用性和可拓展性
func (f *FilterField) CheckValue(val any) bool {
	if f.QueryType == nil {
		return false
	}
	if val == nil {
		if *f.QueryType == QueryTypeEnumNotExist {
			return true
		}
	}
	switch f.FieldType {
	case FieldTypeString:
		if val == nil {
			val = ""
		}
		str, ok := val.(string)
		if !ok {
			logs.Info("invalid string value: %v", val)
			return false
		}
		return Compare(str, f.Values, *f.QueryType)
	case FieldTypeLong:
		if val == nil {
			val = 0
		}
		longVal, err := anyToInt64(val)
		if err != nil {
			logs.Info("invalid long value: %v", val)
			return false
		}
		vals := make([]int64, 0)
		for _, value := range f.Values {
			integer, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return false
			}
			vals = append(vals, integer)
		}
		return Compare(longVal, vals, *f.QueryType)
	case FieldTypeDouble:
		if val == nil {
			val = 0.0
		}
		doubleVal, err := anyToFloat64(val)
		if err != nil {
			logs.Info("invalid float64 value: %v", val)
			return false
		}
		vals := make([]float64, 0)
		for _, value := range f.Values {
			float, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return false
			}
			vals = append(vals, float)
		}
		return Compare(doubleVal, vals, *f.QueryType)
	case FieldTypeBool:
		if val == nil {
			val = false
		}
		boolVal, ok := val.(bool)
		if !ok {
			logs.Info("invalid boolean value: %v", val)
			return false
		}
		vals := make([]bool, 0)
		for _, value := range f.Values {
			b, err := strconv.ParseBool(value)
			if err != nil {
				return false
			}
			vals = append(vals, b)
		}
		return CompareBool(boolVal, vals, *f.QueryType)
	default:
		return false
	}
}

func CompareBool(val bool, values []bool, qType QueryTypeEnum) bool {
	switch qType {
	case QueryTypeEnumEq:
		if len(values) == 0 {
			return false
		}
		return val == values[0]
	case QueryTypeEnumNotEq:
		if len(values) == 0 {
			return false
		}
		return val != values[0]
	default:
		return false
	}
}

// Compare
//
//nolint:staticcheck,S1034
func Compare[T cmp.Ordered](val T, values []T, qType QueryTypeEnum) bool {
	switch qType {
	case QueryTypeEnumMatch:
		if len(values) == 0 {
			return false
		}
		switch any(val).(type) {
		case string:
			return strings.Contains(any(val).(string), any(values[0]).(string))
		default:
			return false
		}
	case QueryTypeEnumEq:
		if len(values) == 0 {
			return false
		}
		return val == values[0]
	case QueryTypeEnumNotEq:
		if len(values) == 0 {
			return false
		}
		return val != values[0]
	case QueryTypeEnumLte:
		if len(values) == 0 {
			return false
		}
		return val <= values[0]
	case QueryTypeEnumGte:
		if len(values) == 0 {
			return false
		}
		return val >= values[0]
	case QueryTypeEnumLt:
		if len(values) == 0 {
			return false
		}
		return val < values[0]
	case QueryTypeEnumGt:
		if len(values) == 0 {
			return false
		}
		return val > values[0]
	case QueryTypeEnumIn:
		for _, value := range values {
			if val == value {
				return true
			}
		}
		return false
	case QueryTypeEnumNotIn:
		for _, value := range values {
			if val == value {
				return false
			}
		}
		return true
	case QueryTypeEnumNotExist:
		iVal := any(val)
		switch iVal.(type) {
		case string:
			return iVal.(string) == ""
		case int64:
			return iVal.(int64) == 0
		case float64:
			return iVal.(float64) == 0
		default:
			return false
		}
	case QueryTypeEnumExist:
		iVal := any(val)
		switch iVal.(type) {
		case string:
			return iVal.(string) != ""
		case int64:
			return iVal.(int64) != 0
		case float64:
			return iVal.(float64) != 0
		default:
			return false
		}
	default:
		return false
	}
}

func anyToInt64(val any) (int64, error) {
	v := reflect.ValueOf(val)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(v.Uint()), nil // 可能丢失精度
	default:
		return 0, fmt.Errorf("invalid integer")
	}
}

func anyToFloat64(val any) (float64, error) {
	v := reflect.ValueOf(val)
	switch v.Kind() {
	case reflect.Float32, reflect.Float64:
		return v.Float(), nil
	default:
		return 0, fmt.Errorf("invalid float")
	}
}
