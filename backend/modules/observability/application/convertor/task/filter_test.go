// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"testing"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/stretchr/testify/assert"
)

func TestTaskFiltersDTO2DO(t *testing.T) {
	t.Parallel()

	t.Run("nil input", func(t *testing.T) {
		t.Parallel()
		result := TaskFiltersDTO2DO(nil)
		assert.Nil(t, result)
	})

	t.Run("empty filters", func(t *testing.T) {
		t.Parallel()
		filters := &filter.TaskFilterFields{
			FilterFields: []*filter.TaskFilterField{},
		}
		result := TaskFiltersDTO2DO(filters)
		assert.NotNil(t, result)
		assert.Empty(t, result.FilterFields)
	})

	t.Run("with query relation", func(t *testing.T) {
		t.Parallel()
		relation := filter.QueryRelationAnd
		filters := &filter.TaskFilterFields{
			QueryAndOr: &relation,
			FilterFields: []*filter.TaskFilterField{
				{
					FieldName: ptr.Of("name"),
					Values:    []string{"test"},
				},
			},
		}
		result := TaskFiltersDTO2DO(filters)
		assert.NotNil(t, result)
		assert.NotNil(t, result.QueryAndOr)
		assert.Equal(t, entity.QueryRelationAnd, *result.QueryAndOr)
		assert.Len(t, result.FilterFields, 1)
	})

	t.Run("with nil filter field", func(t *testing.T) {
		t.Parallel()
		filters := &filter.TaskFilterFields{
			FilterFields: []*filter.TaskFilterField{
				{
					FieldName: ptr.Of("field1"),
					Values:    []string{"value1"},
				},
				nil,
				{
					FieldName: ptr.Of("field2"),
					Values:    []string{"value2"},
				},
			},
		}
		result := TaskFiltersDTO2DO(filters)
		assert.NotNil(t, result)
		assert.Len(t, result.FilterFields, 2) // nil field should be skipped
	})

	t.Run("complete filter field", func(t *testing.T) {
		t.Parallel()
		fieldType := filter.FieldTypeString
		queryType := filter.QueryTypeEq
		relation := filter.QueryRelationOr
		
		filters := &filter.TaskFilterFields{
			QueryAndOr: &relation,
			FilterFields: []*filter.TaskFilterField{
				{
					FieldName: ptr.Of("task_name"),
					FieldType: &fieldType,
					QueryType: &queryType,
					QueryAndOr: &relation,
					Values:    []string{"test_task", "another_task"},
					SubFilter: &filter.TaskFilterField{
						FieldName: ptr.Of("sub_field"),
						Values:    []string{"sub_value"},
					},
				},
			},
		}
		result := TaskFiltersDTO2DO(filters)
		assert.NotNil(t, result)
		assert.Equal(t, entity.QueryRelationOr, *result.QueryAndOr)
		assert.Len(t, result.FilterFields, 1)
		
		field := result.FilterFields[0]
		assert.Equal(t, entity.TaskFieldName("task_name"), *field.FieldName)
		assert.Equal(t, entity.FieldTypeString, *field.FieldType)
		assert.Equal(t, entity.QueryTypeEq, *field.QueryType)
		assert.Equal(t, entity.QueryRelationOr, *field.QueryAndOr)
		assert.Equal(t, []string{"test_task", "another_task"}, field.Values)
		assert.NotNil(t, field.SubFilter)
		assert.Equal(t, entity.TaskFieldName("sub_field"), *field.SubFilter.FieldName)
	})
}

func TestTaskFiltersDO2DTO(t *testing.T) {
	t.Parallel()

	t.Run("nil input", func(t *testing.T) {
		t.Parallel()
		result := TaskFiltersDO2DTO(nil)
		assert.Nil(t, result)
	})

	t.Run("empty filters", func(t *testing.T) {
		t.Parallel()
		filters := &entity.TaskFilterFields{
			FilterFields: []*entity.TaskFilterField{},
		}
		result := TaskFiltersDO2DTO(filters)
		assert.NotNil(t, result)
		assert.Empty(t, result.FilterFields)
	})

	t.Run("with query relation", func(t *testing.T) {
		t.Parallel()
		relation := entity.QueryRelationOr
		filters := &entity.TaskFilterFields{
			QueryAndOr: &relation,
			FilterFields: []*entity.TaskFilterField{
				{
					FieldName: func() *entity.TaskFieldName { n := entity.TaskFieldName("status"); return &n }(),
					Values:    []string{"running"},
				},
			},
		}
		result := TaskFiltersDO2DTO(filters)
		assert.NotNil(t, result)
		assert.NotNil(t, result.QueryAndOr)
		assert.Equal(t, filter.QueryRelationOr, *result.QueryAndOr)
		assert.Len(t, result.FilterFields, 1)
	})

	t.Run("with nil filter field", func(t *testing.T) {
		t.Parallel()
		filters := &entity.TaskFilterFields{
			FilterFields: []*entity.TaskFilterField{
				{
					FieldName: func() *entity.TaskFieldName { n := entity.TaskFieldName("field1"); return &n }(),
					Values:    []string{"value1"},
				},
				nil,
				{
					FieldName: func() *entity.TaskFieldName { n := entity.TaskFieldName("field2"); return &n }(),
					Values:    []string{"value2"},
				},
			},
		}
		result := TaskFiltersDO2DTO(filters)
		assert.NotNil(t, result)
		assert.Len(t, result.FilterFields, 2) // nil field should be skipped
	})

	t.Run("complete filter field", func(t *testing.T) {
		t.Parallel()
		fieldType := entity.FieldTypeLong
		queryType := entity.QueryTypeGte
		relation := entity.QueryRelationAnd
		
		filters := &entity.TaskFilterFields{
			QueryAndOr: &relation,
			FilterFields: []*entity.TaskFilterField{
				{
					FieldName: func() *entity.TaskFieldName { n := entity.TaskFieldName("task_id"); return &n }(),
					FieldType: &fieldType,
					QueryType: &queryType,
					QueryAndOr: &relation,
					Values:    []string{"123", "456"},
					SubFilter: &entity.TaskFilterField{
						FieldName: func() *entity.TaskFieldName { n := entity.TaskFieldName("sub_field"); return &n }(),
						Values:    []string{"sub_value"},
					},
				},
			},
		}
		result := TaskFiltersDO2DTO(filters)
		assert.NotNil(t, result)
		assert.Equal(t, filter.QueryRelationAnd, *result.QueryAndOr)
		assert.Len(t, result.FilterFields, 1)
		
		field := result.FilterFields[0]
		assert.Equal(t, "task_id", *field.FieldName)
		assert.Equal(t, filter.FieldTypeLong, *field.FieldType)
		assert.Equal(t, filter.QueryTypeGte, *field.QueryType)
		assert.Equal(t, filter.QueryRelationAnd, *field.QueryAndOr)
		assert.Equal(t, []string{"123", "456"}, field.Values)
		assert.NotNil(t, field.SubFilter)
		assert.Equal(t, "sub_field", *field.SubFilter.FieldName)
	})
}

func TestTaskFiltersConversionRoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("round trip DTO to DO to DTO", func(t *testing.T) {
		t.Parallel()
		relation := filter.QueryRelationAnd
		fieldType := filter.FieldTypeString
		queryType := filter.QueryTypeIn
		
		original := &filter.TaskFilterFields{
			QueryAndOr: &relation,
			FilterFields: []*filter.TaskFilterField{
				{
					FieldName: ptr.Of("name"),
					FieldType: &fieldType,
					QueryType: &queryType,
					Values:    []string{"test", "prod"},
				},
			},
		}
		
		// DTO -> DO -> DTO
		do := TaskFiltersDTO2DO(original)
		assert.NotNil(t, do)
		
		result := TaskFiltersDO2DTO(do)
		assert.NotNil(t, result)
		assert.Equal(t, *original.QueryAndOr, *result.QueryAndOr)
		assert.Len(t, result.FilterFields, 1)
		assert.Equal(t, *original.FilterFields[0].FieldName, *result.FilterFields[0].FieldName)
		assert.Equal(t, *original.FilterFields[0].FieldType, *result.FilterFields[0].FieldType)
		assert.Equal(t, *original.FilterFields[0].QueryType, *result.FilterFields[0].QueryType)
		assert.Equal(t, original.FilterFields[0].Values, result.FilterFields[0].Values)
	})

	t.Run("round trip DO to DTO to DO", func(t *testing.T) {
		t.Parallel()
		relation := entity.QueryRelationOr
		fieldType := entity.FieldTypeDouble
		queryType := entity.QueryTypeLt
		
		original := &entity.TaskFilterFields{
			QueryAndOr: &relation,
			FilterFields: []*entity.TaskFilterField{
				{
					FieldName: func() *entity.TaskFieldName { n := entity.TaskFieldName("duration"); return &n }(),
					FieldType: &fieldType,
					QueryType: &queryType,
					Values:    []string{"100.5", "200.7"},
				},
			},
		}
		
		// DO -> DTO -> DO
		dto := TaskFiltersDO2DTO(original)
		assert.NotNil(t, dto)
		
		result := TaskFiltersDTO2DO(dto)
		assert.NotNil(t, result)
		assert.Equal(t, *original.QueryAndOr, *result.QueryAndOr)
		assert.Len(t, result.FilterFields, 1)
		assert.Equal(t, *original.FilterFields[0].FieldName, *result.FilterFields[0].FieldName)
		assert.Equal(t, *original.FilterFields[0].FieldType, *result.FilterFields[0].FieldType)
		assert.Equal(t, *original.FilterFields[0].QueryType, *result.FilterFields[0].QueryType)
		assert.Equal(t, original.FilterFields[0].Values, result.FilterFields[0].Values)
	})
}