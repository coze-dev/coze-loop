// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"testing"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/stretchr/testify/assert"
)

func TestFilterFieldsDO2DTO(t *testing.T) {
	// nil 输入
	assert.Nil(t, FilterFieldsDO2DTO(nil))

	// 基本映射
	lf := &loop_span.FilterFields{
		QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumOr),
		FilterFields: []*loop_span.FilterField{
			{FieldName: loop_span.SpanFieldSpanType, FieldType: loop_span.FieldTypeString, Values: []string{"model"}},
		},
	}
	dto := FilterFieldsDO2DTO(lf)
	assert.NotNil(t, dto)
	assert.NotNil(t, dto.QueryAndOr)
	assert.Equal(t, filter.QueryRelationOr, *dto.QueryAndOr)
	assert.Len(t, dto.FilterFields, 1)
	assert.Equal(t, loop_span.SpanFieldSpanType, *dto.FilterFields[0].FieldName)
}

func TestFilterFieldListDO2DTO(t *testing.T) {
	// 包含 nil 的列表，nil 需要被跳过
	fields := []*loop_span.FilterField{
		nil,
		{FieldName: "a", FieldType: loop_span.FieldTypeString, Values: []string{"1"}},
	}
	dtoList := FilterFieldListDO2DTO(fields)
	assert.Len(t, dtoList, 1)
	assert.Equal(t, "a", *dtoList[0].FieldName)

	// 空列表
	assert.Equal(t, 0, len(FilterFieldListDO2DTO([]*loop_span.FilterField{})))
}

func TestFilterFieldDO2DTO_Full(t *testing.T) {
	sub := &loop_span.FilterFields{QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumOr)}
	f := &loop_span.FilterField{
		FieldName:  loop_span.SpanFieldSpaceId,
		Values:     []string{"123"},
		FieldType:  loop_span.FieldTypeLong,
		ExtraInfo:  map[string]string{"k": "v"},
		QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
		QueryType:  ptr.Of(loop_span.QueryTypeEnumIn),
		SubFilter:  sub,
		IsCustom:   true,
	}
	dto := FilterFieldDO2DTO(f)
	assert.NotNil(t, dto)
	assert.Equal(t, loop_span.SpanFieldSpaceId, *dto.FieldName)
	assert.Equal(t, []string{"123"}, dto.Values)
	assert.NotNil(t, dto.FieldType)
	assert.Equal(t, filter.FieldTypeLong, *dto.FieldType)
	assert.Equal(t, "v", dto.ExtraInfo["k"])
	assert.NotNil(t, dto.QueryAndOr)
	assert.Equal(t, filter.QueryRelationAnd, *dto.QueryAndOr)
	assert.NotNil(t, dto.QueryType)
	assert.Equal(t, filter.QueryTypeIn, *dto.QueryType)
	assert.NotNil(t, dto.SubFilter)
	assert.NotNil(t, dto.SubFilter.QueryAndOr)
	assert.Equal(t, filter.QueryRelationOr, *dto.SubFilter.QueryAndOr)
	assert.NotNil(t, dto.IsCustom)
	assert.True(t, *dto.IsCustom)
}

func TestFilterFieldDO2DTO_EmptyName_And_IsCustomFalse(t *testing.T) {
	f := &loop_span.FilterField{
		FieldName: "",
		FieldType: loop_span.FieldTypeString,
		Values:    []string{},
		IsCustom:  false,
	}
	dto := FilterFieldDO2DTO(f)
	assert.NotNil(t, dto)
	// FieldName 始终返回指针（即使为空字符串）
	assert.NotNil(t, dto.FieldName)
	assert.Equal(t, "", *dto.FieldName)
	// IsCustom 为 false 时，不应设置（保持 nil）
	assert.Nil(t, dto.IsCustom)
}

func Test_fieldTypeDO2DTO(t *testing.T) {
	// 空类型默认 string
	p := fieldTypeDO2DTO("")
	assert.NotNil(t, p)
	assert.Equal(t, filter.FieldTypeString, *p)
	// 非空类型按原样映射
	p2 := fieldTypeDO2DTO(loop_span.FieldTypeLong)
	assert.NotNil(t, p2)
	assert.Equal(t, filter.FieldTypeLong, *p2)
}
