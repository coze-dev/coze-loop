// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/stone/fornax/ml_flow/domain/filter"
)

func TestParseItemIDFilter(t *testing.T) {
	eq := filter.QueryTypeEq
	in := filter.QueryTypeIn
	tests := []struct {
		name   string
		filter *filter.Filter
		want   []int64
	}{
		{
			name:   "nil filter returns nil",
			filter: nil,
			want:   nil,
		},
		{
			name:   "no filter fields returns nil",
			filter: &filter.Filter{FilterFields: nil},
			want:   nil,
		},
		{
			name: "item_id eq single value",
			filter: &filter.Filter{FilterFields: []*filter.FilterField{
				{FieldName: filterFieldItemID, FieldType: filter.FieldTypeLong, QueryType: &eq, Values: []string{"123"}},
			}},
			want: []int64{123},
		},
		{
			name: "item_id in multiple values",
			filter: &filter.Filter{FilterFields: []*filter.FilterField{
				{FieldName: filterFieldItemID, FieldType: filter.FieldTypeLong, QueryType: &in, Values: []string{"123", "456", "789"}},
			}},
			want: []int64{123, 456, 789},
		},
		{
			name: "non item_id field is ignored",
			filter: &filter.Filter{FilterFields: []*filter.FilterField{
				{FieldName: "created_by", FieldType: filter.FieldTypeString, QueryType: &eq, Values: []string{"alice"}},
			}},
			want: nil,
		},
		{
			name: "mixed item_id and illegal field only item_id takes effect",
			filter: &filter.Filter{FilterFields: []*filter.FilterField{
				{FieldName: "created_by", FieldType: filter.FieldTypeString, QueryType: &eq, Values: []string{"alice"}},
				{FieldName: filterFieldItemID, FieldType: filter.FieldTypeLong, QueryType: &eq, Values: []string{"123"}},
			}},
			want: []int64{123},
		},
		{
			name: "invalid numeric value is skipped",
			filter: &filter.Filter{FilterFields: []*filter.FilterField{
				{FieldName: filterFieldItemID, FieldType: filter.FieldTypeLong, QueryType: &in, Values: []string{"123", "abc", "456"}},
			}},
			want: []int64{123, 456},
		},
		{
			name: "empty values returns nil",
			filter: &filter.Filter{FilterFields: []*filter.FilterField{
				{FieldName: filterFieldItemID, FieldType: filter.FieldTypeLong, QueryType: &in, Values: nil},
			}},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseItemIDFilter(tt.filter)
			assert.Equal(t, tt.want, got)
		})
	}
}
