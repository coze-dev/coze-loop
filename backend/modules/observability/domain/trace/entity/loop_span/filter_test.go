// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package loop_span

import (
	"testing"

	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/stretchr/testify/assert"
)

func TestFilterValidate(t *testing.T) {
	badFilters := []*FilterFields{
		{
			QueryAndOr: ptr.Of(QueryAndOrEnum("aa")),
		},
		{
			QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
			FilterFields: []*FilterField{
				{
					FieldName: "a",
					FieldType: FieldTypeString,
					Values:    []string{"aa"},
				},
			},
		},
		{
			QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
			FilterFields: []*FilterField{
				{
					FieldName: "a",
					FieldType: FieldTypeString,
					QueryType: ptr.Of(QueryTypeEnumLt),
					Values:    []string{"aa"},
				},
			},
		},
		{
			QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
			FilterFields: []*FilterField{
				{
					FieldName: "a",
					FieldType: FieldTypeDouble,
					QueryType: ptr.Of(QueryTypeEnumIn),
					Values:    []string{"aa"},
				},
			},
		},
		{
			QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
			FilterFields: []*FilterField{
				{
					FieldName: "a",
					FieldType: FieldTypeLong,
					QueryType: ptr.Of(QueryTypeEnumIn),
					Values:    []string{"aa"},
				},
			},
		},
		{
			QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
			FilterFields: []*FilterField{
				{
					FieldName: "a",
					FieldType: FieldTypeBool,
					QueryType: ptr.Of(QueryTypeEnumEq),
					Values:    []string{"aa"},
				},
			},
		},
		{
			QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
			FilterFields: []*FilterField{
				{
					FieldName: "a",
					FieldType: FieldTypeBool,
					QueryType: ptr.Of(QueryTypeEnumEq),
					Values:    []string{"true"},
				},
				{
					SubFilter: &FilterFields{
						FilterFields: []*FilterField{
							{
								FieldName: "a",
								FieldType: FieldTypeLong,
								QueryType: ptr.Of(QueryTypeEnumIn),
								Values:    []string{"123"},
							},
							{
								FieldName: "a",
								FieldType: FieldTypeLong,
								QueryType: ptr.Of(QueryTypeEnumIn),
								Values:    []string{"1234"},
							},
							{
								FieldName: "a",
								FieldType: FieldTypeBool,
								QueryType: ptr.Of(QueryTypeEnumEq),
								Values:    []string{"aa"},
							},
						},
					},
				},
			},
		},
		{
			QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
			FilterFields: []*FilterField{
				{
					FieldName: "a",
					FieldType: FieldTypeBool,
					QueryType: ptr.Of(QueryTypeEnumEq),
					Values:    []string{"true"},
				},
				{
					SubFilter: &FilterFields{
						FilterFields: []*FilterField{
							{
								FieldName: "a",
								FieldType: FieldTypeLong,
								QueryType: ptr.Of(QueryTypeEnumIn),
								Values:    []string{"123"},
							},
							{
								FieldName: "a",
								FieldType: FieldTypeLong,
								QueryType: ptr.Of(QueryTypeEnumIn),
								Values:    []string{"1234"},
							},
							{
								FieldName: "a",
								FieldType: FieldTypeBool,
								QueryType: ptr.Of(QueryTypeEnumEq),
								Values:    []string{"1"},
							},
							{
								SubFilter: &FilterFields{
									FilterFields: []*FilterField{
										{
											FieldName: "a",
											FieldType: FieldTypeLong,
											QueryType: ptr.Of(QueryTypeEnumIn),
											Values:    []string{"zz"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	for _, filter := range badFilters {
		if err := filter.Validate(); err == nil {
			t.Errorf("Filter validation should have failed for bad filter: %+v", filter)
		} else {
			t.Log(err)
		}
	}
	goodFilters := []*FilterFields{
		{
			QueryAndOr: nil,
		},
		{
			QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
			FilterFields: []*FilterField{
				{
					FieldName: "a",
					FieldType: FieldTypeBool,
					QueryType: ptr.Of(QueryTypeEnumEq),
					Values:    []string{"true"},
				},
				{
					SubFilter: &FilterFields{
						FilterFields: []*FilterField{
							{
								FieldName: "a",
								FieldType: FieldTypeLong,
								QueryType: ptr.Of(QueryTypeEnumIn),
								Values:    []string{"123"},
							},
							{
								FieldName: "a",
								FieldType: FieldTypeLong,
								QueryType: ptr.Of(QueryTypeEnumIn),
								Values:    []string{"1234"},
							},
							{
								FieldName: "a",
								FieldType: FieldTypeBool,
								QueryType: ptr.Of(QueryTypeEnumEq),
								Values:    []string{"1"},
							},
							{
								SubFilter: &FilterFields{
									FilterFields: []*FilterField{
										{
											FieldName: "a",
											FieldType: FieldTypeLong,
											QueryType: ptr.Of(QueryTypeEnumIn),
											Values:    []string{"123"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	for _, filter := range goodFilters {
		if err := filter.Validate(); err != nil {
			t.Errorf("Filter validation should not have failed for good filter")
		}
	}
}

func TestFilterTraverse(t *testing.T) {
	filter := &FilterFields{
		QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
		FilterFields: []*FilterField{
			{
				FieldName: "==1",
				FieldType: FieldTypeBool,
				QueryType: ptr.Of(QueryTypeEnumEq),
				Values:    []string{"true"},
			},
			{
				SubFilter: &FilterFields{
					FilterFields: []*FilterField{
						{
							FieldName: "====1",
							FieldType: FieldTypeLong,
							QueryType: ptr.Of(QueryTypeEnumIn),
							Values:    []string{"123"},
						},
						{
							FieldName: "====2",
							FieldType: FieldTypeLong,
							QueryType: ptr.Of(QueryTypeEnumIn),
							Values:    []string{"1234"},
						},
						{
							FieldName: "====3",
							FieldType: FieldTypeBool,
							QueryType: ptr.Of(QueryTypeEnumEq),
							Values:    []string{"1"},
						},
						{
							SubFilter: &FilterFields{
								FilterFields: []*FilterField{
									{
										FieldName: "======1",
										FieldType: FieldTypeLong,
										QueryType: ptr.Of(QueryTypeEnumIn),
										Values:    []string{"123"},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	_ = filter.Traverse(func(f *FilterField) error {
		return nil
	})
}

func TestFilterSpan(t *testing.T) {
	tests := []struct {
		filter    *FilterFields
		span      *Span
		satisfied bool
	}{
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "service_name",
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumEq),
						Values:    []string{"service_name_a"},
					},
					{
						FieldName: "span_type",
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumEq),
						Values:    []string{"span_type_a"},
					},
				},
			},
			span: &Span{
				SpanID:     "aaa",
				TraceID:    "zz",
				ParentID:   "zzz",
				SpanType:   "span_type_a",
				StatusCode: 100,
				TagsString: map[string]string{
					"service_name":  "service_name_a",
					"service_name2": "z",
				},
				TagsLong: map[string]int64{
					"service_name3": 1,
				},
				TagsDouble: map[string]float64{
					"customs_double_tag_exist": 12,
				},
				TagsBool: map[string]bool{
					"bool_test": true,
				},
			},
			satisfied: true,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "service_name",
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumEq),
						Values:    []string{"service_name_a"},
					},
					{
						FieldName: "span_type",
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumEq),
						Values:    []string{"span_type_a"},
					},
				},
			},
			span: &Span{
				SpanID:   "aaa",
				TraceID:  "zz",
				ParentID: "zzz",
				SpanType: "span_type_a",
				TagsString: map[string]string{
					"service_name": "service_name_b",
				},
			},
			satisfied: false,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumOr),
				FilterFields: []*FilterField{
					{
						FieldName: "service_name",
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumEq),
						Values:    []string{"service_name_b"},
					},
					{
						FieldName: "span_type",
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumEq),
						Values:    []string{"span_type_b"},
					},
				},
			},
			span: &Span{
				SpanID:     "aaa",
				TraceID:    "zz",
				ParentID:   "zzz",
				SpanType:   "span_type_a",
				StatusCode: 100,
				TagsString: map[string]string{
					"service_name":  "service_name_a",
					"service_name2": "z",
				},
				TagsLong: map[string]int64{
					"service_name3": 1,
				},
				TagsDouble: map[string]float64{
					"customs_double_tag_exist": 12,
				},
				TagsBool: map[string]bool{
					"bool_test": true,
				},
			},
			satisfied: false,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumOr),
				FilterFields: []*FilterField{
					{
						FieldName: "service_name",
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumEq),
						Values:    []string{"service_name_b"},
					},
					{
						FieldName: "span_type",
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumEq),
						Values:    []string{"span_type_b"},
					},
				},
			},
			span: &Span{
				SpanID:   "aaa",
				TraceID:  "zz",
				ParentID: "zzz",
				SpanType: "span_type_a",
				TagsString: map[string]string{
					"service_name": "service_name_b",
				},
			},
			satisfied: true,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "service_name",
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumIn),
						Values:    []string{"service_name_b", "service_name_a"},
					},
					{
						FieldName: "span_type",
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumIn),
						Values:    []string{"span_type_b", "span_type_a"},
					},
				},
			},
			span: &Span{
				SpanID:     "aaa",
				TraceID:    "zz",
				ParentID:   "zzz",
				SpanType:   "span_type_a",
				StatusCode: 100,
				TagsString: map[string]string{
					"service_name":  "service_name_a",
					"service_name2": "z",
				},
				TagsLong: map[string]int64{
					"service_name3": 1,
				},
				TagsDouble: map[string]float64{
					"customs_double_tag_exist": 12,
				},
				TagsBool: map[string]bool{
					"bool_test": true,
				},
			},
			satisfied: true,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "service_name",
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumIn),
						Values:    []string{"service_name_b", "service_name_a"},
					},
					{
						FieldName: "span_type",
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumIn),
						Values:    []string{"span_type_b", "span_type_a"},
					},
				},
			},
			span: &Span{
				SpanID:   "aaa",
				TraceID:  "zz",
				ParentID: "zzz",
				SpanType: "span_type_a",
				TagsString: map[string]string{
					"service_name": "service_name_b",
				},
			},
			satisfied: true,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "service_name",
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumNotIn),
						Values:    []string{"service_name_b", "service_name_a"},
					},
					{
						FieldName: "span_type",
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumNotIn),
						Values:    []string{"span_type_b", "span_type_a"},
					},
				},
			},
			span: &Span{
				SpanID:     "aaa",
				TraceID:    "zz",
				ParentID:   "zzz",
				SpanType:   "span_type_a",
				StatusCode: 100,
				TagsString: map[string]string{
					"service_name":  "service_name_a",
					"service_name2": "z",
				},
				TagsLong: map[string]int64{
					"service_name3": 1,
				},
				TagsDouble: map[string]float64{
					"customs_double_tag_exist": 12,
				},
				TagsBool: map[string]bool{
					"bool_test": true,
				},
			},
			satisfied: false,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "service_name",
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumNotIn),
						Values:    []string{"service_name_b", "service_name_a"},
					},
					{
						FieldName: "span_type",
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumNotIn),
						Values:    []string{"span_type_b", "span_type_a"},
					},
				},
			},
			span: &Span{
				SpanID:   "aaa",
				TraceID:  "zz",
				ParentID: "zzz",
				SpanType: "span_type_a",
				TagsString: map[string]string{
					"service_name": "service_name_b",
				},
			},
			satisfied: false,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "span_type",
						FieldType: FieldTypeString,
						Values:    []string{"_a"},
						QueryType: ptr.Of(QueryTypeEnumMatch),
					},
				},
			},
			span: &Span{
				SpanID:     "aaa",
				TraceID:    "zz",
				ParentID:   "zzz",
				SpanType:   "span_type_a",
				StatusCode: 100,
				TagsString: map[string]string{
					"service_name":  "service_name_a",
					"service_name2": "z",
				},
				TagsLong: map[string]int64{
					"service_name3": 1,
				},
				TagsDouble: map[string]float64{
					"customs_double_tag_exist": 12,
				},
				TagsBool: map[string]bool{
					"bool_test": true,
				},
			},
			satisfied: true,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "span_type",
						FieldType: FieldTypeString,
						Values:    []string{"_a"},
						QueryType: ptr.Of(QueryTypeEnumMatch),
					},
				},
			},
			span: &Span{
				SpanID:   "aaa",
				TraceID:  "zz",
				ParentID: "zzz",
				SpanType: "span_type_a",
				TagsString: map[string]string{
					"service_name": "service_name_b",
				},
			},
			satisfied: true,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: SpanFieldTraceId,
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumExist),
					},
				},
			},
			span: &Span{
				SpanID:     "aaa",
				TraceID:    "zz",
				ParentID:   "zzz",
				SpanType:   "span_type_a",
				StatusCode: 100,
				TagsString: map[string]string{
					"service_name":  "service_name_a",
					"service_name2": "z",
				},
				TagsLong: map[string]int64{
					"service_name3": 1,
				},
				TagsDouble: map[string]float64{
					"customs_double_tag_exist": 12,
				},
				TagsBool: map[string]bool{
					"bool_test": true,
				},
			},
			satisfied: true,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: SpanFieldTraceId,
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumNotExist),
					},
				},
			},
			span: &Span{
				SpanID:     "aaa",
				TraceID:    "zz",
				ParentID:   "zzz",
				SpanType:   "span_type_a",
				StatusCode: 100,
				TagsString: map[string]string{
					"service_name":  "service_name_a",
					"service_name2": "z",
				},
				TagsLong: map[string]int64{
					"service_name3": 1,
				},
				TagsDouble: map[string]float64{
					"customs_double_tag_exist": 12,
				},
				TagsBool: map[string]bool{
					"bool_test": true,
				},
			},
			satisfied: false,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "service_name",
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumExist),
					},
				},
			},
			span: &Span{
				SpanID:     "aaa",
				TraceID:    "zz",
				ParentID:   "zzz",
				SpanType:   "span_type_a",
				StatusCode: 100,
				TagsString: map[string]string{
					"service_name":  "service_name_a",
					"service_name2": "z",
				},
				TagsLong: map[string]int64{
					"service_name3": 1,
				},
				TagsDouble: map[string]float64{
					"customs_double_tag_exist": 12,
				},
				TagsBool: map[string]bool{
					"bool_test": true,
				},
			},
			satisfied: true,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "service_name",
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumNotExist),
					},
				},
			},
			span: &Span{
				SpanID:     "aaa",
				TraceID:    "zz",
				ParentID:   "zzz",
				SpanType:   "span_type_a",
				StatusCode: 100,
				TagsString: map[string]string{
					"service_name":  "service_name_a",
					"service_name2": "z",
				},
				TagsLong: map[string]int64{
					"service_name3": 1,
				},
				TagsDouble: map[string]float64{
					"customs_double_tag_exist": 12,
				},
				TagsBool: map[string]bool{
					"bool_test": true,
				},
			},
			satisfied: false,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "service_name2",
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumExist),
					},
				},
			},
			span: &Span{
				SpanID:     "aaa",
				TraceID:    "zz",
				ParentID:   "zzz",
				SpanType:   "span_type_a",
				StatusCode: 100,
				TagsString: map[string]string{
					"service_name":  "service_name_a",
					"service_name2": "z",
				},
				TagsLong: map[string]int64{
					"service_name3": 1,
				},
				TagsDouble: map[string]float64{
					"customs_double_tag_exist": 12,
				},
				TagsBool: map[string]bool{
					"bool_test": true,
				},
			},
			satisfied: true,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "service_name3",
						FieldType: FieldTypeLong,
						QueryType: ptr.Of(QueryTypeEnumNotExist),
					},
				},
			},
			span: &Span{
				SpanID:     "aaa",
				TraceID:    "zz",
				ParentID:   "zzz",
				SpanType:   "span_type_a",
				StatusCode: 100,
				TagsString: map[string]string{
					"service_name":  "service_name_a",
					"service_name2": "z",
				},
				TagsLong: map[string]int64{
					"service_name3": 1,
				},
				TagsDouble: map[string]float64{
					"customs_double_tag_exist": 12,
				},
				TagsBool: map[string]bool{
					"bool_test": true,
				},
			},
			satisfied: false,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "double_name3",
						FieldType: FieldTypeDouble,
						QueryType: ptr.Of(QueryTypeEnumNotExist),
					},
				},
			},
			span: &Span{
				SpanID:     "aaa",
				TraceID:    "zz",
				ParentID:   "zzz",
				SpanType:   "span_type_a",
				StatusCode: 100,
				TagsDouble: map[string]float64{
					"double_name3": 12,
				},
			},
			satisfied: false,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "double_name3",
						FieldType: FieldTypeDouble,
						QueryType: ptr.Of(QueryTypeEnumExist),
					},
				},
			},
			span: &Span{
				SpanID:     "aaa",
				TraceID:    "zz",
				ParentID:   "zzz",
				SpanType:   "span_type_a",
				StatusCode: 100,
				TagsDouble: map[string]float64{
					"double_name3": 12,
				},
			},
			satisfied: true,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "service_name4",
						FieldType: FieldTypeLong,
						QueryType: ptr.Of(QueryTypeEnumExist),
					},
				},
			},
			span: &Span{
				SpanID:     "aaa",
				TraceID:    "zz",
				ParentID:   "zzz",
				SpanType:   "span_type_a",
				StatusCode: 100,
				TagsString: map[string]string{
					"service_name":  "service_name_a",
					"service_name2": "z",
				},
				TagsLong: map[string]int64{
					"service_name3": 1,
				},
				TagsDouble: map[string]float64{
					"customs_double_tag_exist": 12,
				},
				TagsBool: map[string]bool{
					"bool_test": true,
				},
			},
			satisfied: false,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "service_name4",
						FieldType: FieldTypeLong,
						QueryType: ptr.Of(QueryTypeEnumExist),
					},
				},
			},
			span: &Span{
				SpanID:     "aaa",
				TraceID:    "zz",
				ParentID:   "zzz",
				SpanType:   "span_type_a",
				StatusCode: 100,
				TagsString: map[string]string{
					"service_name":  "service_name_a",
					"service_name2": "z",
				},
				TagsLong: map[string]int64{
					"service_name3": 1,
				},
				TagsDouble: map[string]float64{
					"customs_double_tag_exist": 12,
				},
				TagsBool: map[string]bool{
					"bool_test": true,
				},
			},
			satisfied: false,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "service_name4",
						FieldType: FieldTypeLong,
						QueryType: ptr.Of(QueryTypeEnumNotExist),
					},
				},
			},
			span: &Span{
				SpanID:     "aaa",
				TraceID:    "zz",
				ParentID:   "zzz",
				SpanType:   "span_type_a",
				StatusCode: 100,
				TagsString: map[string]string{
					"service_name":  "service_name_a",
					"service_name2": "z",
				},
				TagsLong: map[string]int64{
					"service_name3": 1,
				},
				TagsDouble: map[string]float64{
					"customs_double_tag_exist": 12,
				},
				TagsBool: map[string]bool{
					"bool_test": true,
				},
			},
			satisfied: true,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "bool_test",
						FieldType: FieldTypeBool,
						QueryType: ptr.Of(QueryTypeEnumEq),
						Values:    []string{"true"},
					},
				},
			},
			span: &Span{
				SpanID:     "aaa",
				TraceID:    "zz",
				ParentID:   "zzz",
				SpanType:   "span_type_a",
				StatusCode: 100,
				TagsString: map[string]string{
					"service_name":  "service_name_a",
					"service_name2": "z",
				},
				TagsLong: map[string]int64{
					"service_name3": 1,
				},
				TagsDouble: map[string]float64{
					"customs_double_tag_exist": 12,
				},
				TagsBool: map[string]bool{
					"bool_test": true,
				},
			},
			satisfied: true,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "bool_test",
						FieldType: FieldTypeBool,
						QueryType: ptr.Of(QueryTypeEnumNotEq),
						Values:    []string{"false"},
					},
				},
			},
			span: &Span{
				SpanID:     "aaa",
				TraceID:    "zz",
				ParentID:   "zzz",
				SpanType:   "span_type_a",
				StatusCode: 100,
				TagsString: map[string]string{
					"service_name":  "service_name_a",
					"service_name2": "z",
				},
				TagsLong: map[string]int64{
					"service_name3": 1,
				},
				TagsDouble: map[string]float64{
					"customs_double_tag_exist": 12,
				},
				TagsBool: map[string]bool{
					"bool_test": true,
				},
			},
			satisfied: true,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "bool_test",
						FieldType: FieldTypeBool,
						QueryType: ptr.Of(QueryTypeEnumEq),
						Values:    []string{"false"},
					},
				},
			},
			span: &Span{
				SpanID:     "aaa",
				TraceID:    "zz",
				ParentID:   "zzz",
				SpanType:   "span_type_a",
				StatusCode: 100,
				TagsString: map[string]string{
					"service_name":  "service_name_a",
					"service_name2": "z",
				},
				TagsLong: map[string]int64{
					"service_name3": 1,
				},
				TagsDouble: map[string]float64{
					"customs_double_tag_exist": 12,
				},
				TagsBool: map[string]bool{
					"bool_test": true,
				},
			},
			satisfied: false,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: SpanFieldStatusCode,
						FieldType: FieldTypeLong,
						QueryType: ptr.Of(QueryTypeEnumNotIn),
						Values:    []string{"0"},
					},
				},
			},
			span: &Span{
				SpanID:     "aaa",
				TraceID:    "zz",
				ParentID:   "zzz",
				SpanType:   "span_type_a",
				StatusCode: 100,
				TagsString: map[string]string{
					"service_name":  "service_name_a",
					"service_name2": "z",
				},
				TagsLong: map[string]int64{
					"service_name3": 1,
				},
				TagsDouble: map[string]float64{
					"customs_double_tag_exist": 12,
				},
				TagsBool: map[string]bool{
					"bool_test": true,
				},
			},
			satisfied: true,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: SpanFieldStatusCode,
						FieldType: FieldTypeLong,
						QueryType: ptr.Of(QueryTypeEnumIn),
						Values:    []string{"0"},
					},
				},
			},
			span: &Span{
				SpanID:     "aaa",
				TraceID:    "zz",
				ParentID:   "zzz",
				SpanType:   "span_type_a",
				StatusCode: 100,
				TagsString: map[string]string{
					"service_name":  "service_name_a",
					"service_name2": "z",
				},
				TagsLong: map[string]int64{
					"service_name3": 1,
				},
				TagsDouble: map[string]float64{
					"customs_double_tag_exist": 12,
				},
				TagsBool: map[string]bool{
					"bool_test": true,
				},
			},
			satisfied: false,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "customs_double_tag_exist",
						FieldType: FieldTypeDouble,
						QueryType: ptr.Of(QueryTypeEnumEq),
						Values:    []string{"12.0"},
					},
				},
			},
			span: &Span{
				SpanID:     "aaa",
				TraceID:    "zz",
				ParentID:   "zzz",
				SpanType:   "span_type_a",
				StatusCode: 100,
				TagsString: map[string]string{
					"service_name":  "service_name_a",
					"service_name2": "z",
				},
				TagsLong: map[string]int64{
					"service_name3": 1,
				},
				TagsDouble: map[string]float64{
					"customs_double_tag_exist": 12,
				},
				TagsBool: map[string]bool{
					"bool_test": true,
				},
			},
			satisfied: true,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "customs_double_tag_not_exist",
						FieldType: FieldTypeDouble,
						QueryType: ptr.Of(QueryTypeEnumGte),
						Values:    []string{"0"},
					},
				},
			},
			span: &Span{
				SpanID:     "aaa",
				TraceID:    "zz",
				ParentID:   "zzz",
				SpanType:   "span_type_a",
				StatusCode: 100,
				TagsString: map[string]string{
					"service_name":  "service_name_a",
					"service_name2": "z",
				},
				TagsLong: map[string]int64{
					"service_name3": 1,
				},
				TagsDouble: map[string]float64{
					"customs_double_tag_exist": 12,
				},
				TagsBool: map[string]bool{
					"bool_test": true,
				},
			},
			satisfied: true,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: SpanFieldDuration,
						FieldType: FieldTypeLong,
						QueryType: ptr.Of(QueryTypeEnumNotEq),
						Values:    []string{"100"},
					},
				},
			},
			span: &Span{
				SpanID:         "aaa",
				TraceID:        "zz",
				ParentID:       "zzz",
				SpanType:       "span_type_a",
				StatusCode:     100,
				DurationMicros: 100,
			},
			satisfied: false,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: SpanFieldDuration,
						FieldType: FieldTypeLong,
						QueryType: ptr.Of(QueryTypeEnumLte),
						Values:    []string{"100"},
					},
				},
			},
			span: &Span{
				SpanID:         "aaa",
				TraceID:        "zz",
				ParentID:       "zzz",
				SpanType:       "span_type_a",
				StatusCode:     100,
				DurationMicros: 100,
			},
			satisfied: true,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: SpanFieldDuration,
						FieldType: FieldTypeLong,
						QueryType: ptr.Of(QueryTypeEnumLte),
						Values:    []string{"99"},
					},
				},
			},
			span: &Span{
				SpanID:         "aaa",
				TraceID:        "zz",
				ParentID:       "zzz",
				SpanType:       "span_type_a",
				StatusCode:     100,
				DurationMicros: 100,
			},
			satisfied: false,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: SpanFieldDuration,
						FieldType: FieldTypeLong,
						QueryType: ptr.Of(QueryTypeEnumLt),
						Values:    []string{"100"},
					},
				},
			},
			span: &Span{
				SpanID:         "aaa",
				TraceID:        "zz",
				ParentID:       "zzz",
				SpanType:       "span_type_a",
				StatusCode:     100,
				DurationMicros: 100,
			},
			satisfied: false,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: SpanFieldDuration,
						FieldType: FieldTypeLong,
						QueryType: ptr.Of(QueryTypeEnumGt),
						Values:    []string{"99"},
					},
				},
			},
			span: &Span{
				SpanID:         "aaa",
				TraceID:        "zz",
				ParentID:       "zzz",
				SpanType:       "span_type_a",
				StatusCode:     100,
				DurationMicros: 100,
			},
			satisfied: true,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "abc",
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumEq),
						Values:    []string{""},
						IsSystem:  true,
					},
				},
			},
			span: &Span{
				SpanID:         "aaa",
				TraceID:        "zz",
				ParentID:       "zzz",
				SpanType:       "span_type_a",
				StatusCode:     100,
				DurationMicros: 100,
			},
			satisfied: true,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "abc",
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumEq),
						Values:    []string{"123"},
						IsSystem:  true,
					},
				},
			},
			span: &Span{
				SpanID:         "aaa",
				TraceID:        "zz",
				ParentID:       "zzz",
				SpanType:       "span_type_a",
				StatusCode:     100,
				DurationMicros: 100,
				SystemTagsString: map[string]string{
					"abc": "123",
				},
			},
			satisfied: true,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "abc",
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumEq),
						Values:    []string{"123"},
						IsSystem:  true,
					},
				},
			},
			span: &Span{
				SpanID:         "aaa",
				TraceID:        "zz",
				ParentID:       "zzz",
				SpanType:       "span_type_a",
				StatusCode:     100,
				DurationMicros: 100,
				SystemTagsLong: map[string]int64{
					"abc": 123,
				},
			},
			satisfied: false,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "span_type",
						FieldType: FieldTypeString,
						Values:    []string{"_b"},
						QueryType: ptr.Of(QueryTypeEnumNotMatch),
					},
				},
			},
			span: &Span{
				SpanType: "span_type_a",
			},
			satisfied: true,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "span_type",
						FieldType: FieldTypeString,
						Values:    []string{"_a"},
						QueryType: ptr.Of(QueryTypeEnumNotMatch),
					},
				},
			},
			span: &Span{
				SpanType: "span_type_a",
			},
			satisfied: false,
		},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.filter.Satisfied(tc.span), tc.satisfied)
	}
}
