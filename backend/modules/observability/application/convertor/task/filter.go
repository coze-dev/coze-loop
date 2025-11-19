// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TaskFiltersDTO2DO(filters *filter.TaskFilterFields) *entity.TaskFilterFields {
	if filters == nil {
		return nil
	}
	result := &entity.TaskFilterFields{}
	if filters.QueryAndOr != nil {
		relation := entity.QueryRelation(*filters.QueryAndOr)
		result.QueryAndOr = &relation
	}
	if len(filters.FilterFields) == 0 {
		return result
	}
	result.FilterFields = make([]*entity.TaskFilterField, 0, len(filters.FilterFields))
	for _, field := range filters.FilterFields {
		if field == nil {
			continue
		}
		result.FilterFields = append(result.FilterFields, taskFilterFieldDTO2DO(field))
	}
	return result
}

func taskFilterFieldDTO2DO(field *filter.TaskFilterField) *entity.TaskFilterField {
	if field == nil {
		return nil
	}
	result := &entity.TaskFilterField{
		Values:    append([]string(nil), field.Values...),
		SubFilter: taskFilterFieldDTO2DO(field.SubFilter),
	}
	if field.FieldName != nil {
		name := entity.TaskFieldName(*field.FieldName)
		result.FieldName = &name
	}
	if field.FieldType != nil {
		fieldType := entity.FieldType(*field.FieldType)
		result.FieldType = &fieldType
	}
	if field.QueryType != nil {
		queryType := entity.QueryType(*field.QueryType)
		result.QueryType = &queryType
	}
	if field.QueryAndOr != nil {
		relation := entity.QueryRelation(*field.QueryAndOr)
		result.QueryAndOr = &relation
	}
	return result
}

func TaskFiltersDO2DTO(filters *entity.TaskFilterFields) *filter.TaskFilterFields {
	if filters == nil {
		return nil
	}
	result := &filter.TaskFilterFields{}
	if filters.QueryAndOr != nil {
		result.QueryAndOr = ptr.Of(filter.QueryRelation(*filters.QueryAndOr))
	}
	if len(filters.FilterFields) == 0 {
		return result
	}
	result.FilterFields = make([]*filter.TaskFilterField, 0, len(filters.FilterFields))
	for _, field := range filters.FilterFields {
		if field == nil {
			continue
		}
		result.FilterFields = append(result.FilterFields, taskFilterFieldDO2DTO(field))
	}
	return result
}

func taskFilterFieldDO2DTO(field *entity.TaskFilterField) *filter.TaskFilterField {
	if field == nil {
		return nil
	}
	result := &filter.TaskFilterField{
		Values:    append([]string(nil), field.Values...),
		SubFilter: taskFilterFieldDO2DTO(field.SubFilter),
	}
	if field.FieldName != nil {
		result.FieldName = ptr.Of(string(*field.FieldName))
	}
	if field.FieldType != nil {
		result.FieldType = ptr.Of(filter.FieldType(*field.FieldType))
	}
	if field.QueryType != nil {
		result.QueryType = ptr.Of(filter.QueryType(*field.QueryType))
	}
	if field.QueryAndOr != nil {
		result.QueryAndOr = ptr.Of(filter.QueryRelation(*field.QueryAndOr))
	}
	return result
}
