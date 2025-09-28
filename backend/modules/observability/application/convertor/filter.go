// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func FilterFieldsDTO2DO(f *filter.FilterFields) *loop_span.FilterFields {
	if f == nil {
		return nil
	}
	ret := &loop_span.FilterFields{}
	if f.QueryAndOr != nil {
		ret.QueryAndOr = ptr.Of(loop_span.QueryAndOrEnum(*f.QueryAndOr))
	}
	ret.FilterFields = make([]*loop_span.FilterField, 0)
	for _, field := range f.GetFilterFields() {
		if field == nil {
			continue
		}
		fieldName := ""
		if field.FieldName != nil {
			fieldName = *field.FieldName
		}
		fField := &loop_span.FilterField{
			FieldName: fieldName,
			Values:    field.Values,
			FieldType: fieldTypeDTO2DO(field.FieldType),
		}
		if field.QueryAndOr != nil {
			fField.QueryAndOr = ptr.Of(loop_span.QueryAndOrEnum(*field.QueryAndOr))
		}
		if field.QueryType != nil {
			fField.QueryType = ptr.Of(loop_span.QueryTypeEnum(*field.QueryType))
		}
		if field.SubFilter != nil {
			fField.SubFilter = FilterFieldsDTO2DO(field.SubFilter)
		}
		ret.FilterFields = append(ret.FilterFields, fField)
	}
	return ret
}

func fieldTypeDTO2DO(fieldType *filter.FieldType) loop_span.FieldType {
	if fieldType == nil {
		return loop_span.FieldTypeString
	}
	return loop_span.FieldType(*fieldType)
}

func FilterFieldsDO2DTO(f *loop_span.FilterFields) *filter.FilterFields {
	if f == nil {
		return nil
	}
	ret := &filter.FilterFields{}
	if f.QueryAndOr != nil {
		ret.QueryAndOr = ptr.Of(filter.QueryRelation(*f.QueryAndOr))
	}
	ret.FilterFields = make([]*filter.FilterField, 0)
	for _, field := range f.FilterFields {
		if field == nil {
			continue
		}
		fField := &filter.FilterField{
			FieldName: ptr.Of(field.FieldName),
			Values:    field.Values,
			FieldType: fieldTypeDO2DTO(field.FieldType),
		}
		if field.QueryAndOr != nil {
			fField.QueryAndOr = ptr.Of(filter.QueryRelation(*field.QueryAndOr))
		}
		if field.QueryType != nil {
			fField.QueryType = ptr.Of(filter.QueryType(*field.QueryType))
		}
		if field.SubFilter != nil {
			fField.SubFilter = FilterFieldsDO2DTO(field.SubFilter)
		}
		ret.FilterFields = append(ret.FilterFields, fField)
	}
	return ret
}

func fieldTypeDO2DTO(fieldType loop_span.FieldType) *filter.FieldType {
	result := filter.FieldType(fieldType)
	return &result
}