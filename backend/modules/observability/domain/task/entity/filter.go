// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// QueryType represents the operator applied to filter values.
type QueryType string

// QueryRelation represents the logical relation between multiple filter expressions.
type QueryRelation string

// FieldType describes the type of a field used in filter expressions.
type FieldType string

// TaskFieldName defines the supported task field names for filtering.
type TaskFieldName string

const (
	QueryTypeMatch    QueryType = "match"
	QueryTypeEq       QueryType = "eq"
	QueryTypeNotEq    QueryType = "not_eq"
	QueryTypeLte      QueryType = "lte"
	QueryTypeGte      QueryType = "gte"
	QueryTypeLt       QueryType = "lt"
	QueryTypeGt       QueryType = "gt"
	QueryTypeExist    QueryType = "exist"
	QueryTypeNotExist QueryType = "not_exist"
	QueryTypeIn       QueryType = "in"
	QueryTypeNotIn    QueryType = "not_in"
	QueryTypeNotMatch QueryType = "not_match"

	QueryRelationAnd QueryRelation = "and"
	QueryRelationOr  QueryRelation = "or"

	FieldTypeString FieldType = "string"
	FieldTypeLong   FieldType = "long"
	FieldTypeDouble FieldType = "double"
	FieldTypeBool   FieldType = "bool"

	TaskFieldNameTaskStatus TaskFieldName = "task_status"
	TaskFieldNameTaskName   TaskFieldName = "task_name"
	TaskFieldNameTaskType   TaskFieldName = "task_type"
	TaskFieldNameSampleRate TaskFieldName = "sample_rate"
	TaskFieldNameCreatedBy  TaskFieldName = "created_by"
	TaskFieldNameTaskSource TaskFieldName = "task_source"
)

// TaskFilterFields aggregates multiple TaskFilterField expressions.
type TaskFilterFields struct {
	QueryAndOr   *QueryRelation
	FilterFields []*TaskFilterField
}

// GetQueryAndOr returns the relation between filter expressions.
func (f *TaskFilterFields) GetQueryAndOr() string {
	if f == nil || f.QueryAndOr == nil {
		return string(QueryRelationAnd)
	}
	return string(*f.QueryAndOr)
}

// TaskFilterField describes a single filter clause.
type TaskFilterField struct {
	FieldName  *TaskFieldName
	FieldType  *FieldType
	Values     []string
	QueryType  *QueryType
	QueryAndOr *QueryRelation
	SubFilter  *TaskFilterField
}
