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
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName:  SpanFieldParentID,
						FieldType:  FieldTypeString,
						Values:     []string{"0", ""},
						QueryType:  ptr.Of(QueryTypeEnumIn),
						QueryAndOr: ptr.Of(QueryAndOrEnumOr),
						SubFilter: &FilterFields{
							FilterFields: []*FilterField{
								{
									FieldName: "abc",
									FieldType: FieldTypeBool,
									Values:    []string{"true"},
									QueryType: ptr.Of(QueryTypeEnumEq),
								},
							},
						},
					},
				},
			},
			span: &Span{
				ParentID: "0",
			},
			satisfied: true,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName:  SpanFieldParentID,
						FieldType:  FieldTypeString,
						Values:     []string{"0", ""},
						QueryType:  ptr.Of(QueryTypeEnumIn),
						QueryAndOr: ptr.Of(QueryAndOrEnumOr),
						SubFilter: &FilterFields{
							FilterFields: []*FilterField{
								{
									FieldName: "abc",
									FieldType: FieldTypeBool,
									Values:    []string{"true"},
									QueryType: ptr.Of(QueryTypeEnumEq),
								},
							},
						},
					},
				},
			},
			span: &Span{
				ParentID: "",
			},
			satisfied: true,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName:  SpanFieldParentID,
						FieldType:  FieldTypeString,
						Values:     []string{"0", ""},
						QueryType:  ptr.Of(QueryTypeEnumIn),
						QueryAndOr: ptr.Of(QueryAndOrEnumOr),
						SubFilter: &FilterFields{
							FilterFields: []*FilterField{
								{
									FieldName: "abc",
									FieldType: FieldTypeBool,
									Values:    []string{"true"},
									QueryType: ptr.Of(QueryTypeEnumEq),
								},
							},
						},
					},
				},
			},
			span: &Span{
				ParentID: "anc",
			},
			satisfied: false,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName:  SpanFieldParentID,
						FieldType:  FieldTypeString,
						Values:     []string{"0", ""},
						QueryType:  ptr.Of(QueryTypeEnumIn),
						QueryAndOr: ptr.Of(QueryAndOrEnumOr),
						SubFilter: &FilterFields{
							FilterFields: []*FilterField{
								{
									FieldName: "abc",
									FieldType: FieldTypeBool,
									Values:    []string{"true"},
									QueryType: ptr.Of(QueryTypeEnumEq),
								},
							},
						},
					},
				},
			},
			span: &Span{
				ParentID: "cnv",
				TagsBool: map[string]bool{
					"abc": true,
				},
			},
			satisfied: true,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumOr),
				FilterFields: []*FilterField{
					{
						FieldName:  SpanFieldParentID,
						FieldType:  FieldTypeString,
						Values:     []string{"0"},
						QueryType:  ptr.Of(QueryTypeEnumIn),
						QueryAndOr: ptr.Of(QueryAndOrEnumOr),
						SubFilter: &FilterFields{
							FilterFields: []*FilterField{
								{
									FieldName: "abc",
									FieldType: FieldTypeBool,
									Values:    []string{"true"},
									QueryType: ptr.Of(QueryTypeEnumEq),
								},
							},
						},
					},
					{
						FieldName:  SpanFieldTraceId,
						FieldType:  FieldTypeString,
						Values:     []string{"123"},
						QueryType:  ptr.Of(QueryTypeEnumEq),
						QueryAndOr: ptr.Of(QueryAndOrEnumOr),
					},
				},
			},
			span: &Span{
				TraceID: "123",
			},
			satisfied: true,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumOr),
				FilterFields: []*FilterField{
					{
						FieldName:  SpanFieldParentID,
						FieldType:  FieldTypeString,
						Values:     []string{"0"},
						QueryType:  ptr.Of(QueryTypeEnumIn),
						QueryAndOr: ptr.Of(QueryAndOrEnumOr),
						SubFilter: &FilterFields{
							FilterFields: []*FilterField{
								{
									FieldName: "abc",
									FieldType: FieldTypeBool,
									Values:    []string{"true"},
									QueryType: ptr.Of(QueryTypeEnumEq),
								},
							},
						},
					},
					{
						FieldName:  SpanFieldTraceId,
						FieldType:  FieldTypeString,
						Values:     []string{"123"},
						QueryType:  ptr.Of(QueryTypeEnumEq),
						QueryAndOr: ptr.Of(QueryAndOrEnumOr),
					},
				},
			},
			span: &Span{
				ParentID: "0",
			},
			satisfied: true,
		},
		{
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName:  SpanFieldParentID,
						FieldType:  FieldTypeString,
						Values:     []string{"0"},
						QueryType:  ptr.Of(QueryTypeEnumIn),
						QueryAndOr: ptr.Of(QueryAndOrEnumOr),
						SubFilter: &FilterFields{
							FilterFields: []*FilterField{
								{
									FieldName: "abc",
									FieldType: FieldTypeBool,
									Values:    []string{"true"},
									QueryType: ptr.Of(QueryTypeEnumEq),
								},
							},
						},
					},
					{
						FieldName:  SpanFieldTraceId,
						FieldType:  FieldTypeString,
						Values:     []string{"123"},
						QueryType:  ptr.Of(QueryTypeEnumEq),
						QueryAndOr: ptr.Of(QueryAndOrEnumOr),
					},
				},
			},
			span: &Span{
				ParentID: "0",
				TraceID:  "123",
			},
			satisfied: true,
		},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.filter.Satisfied(tc.span), tc.satisfied)
	}
}

// TestQueryTypeEnumNotMatchExceptionCases 测试 QueryTypeEnumNotMatch 的异常流程
func TestQueryTypeEnumNotMatchExceptionCases(t *testing.T) {
	tests := []struct {
		name      string
		filter    *FilterFields
		span      *Span
		satisfied bool
	}{
		// 边界情况测试
		{
			name: "Empty values array should return true",
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "span_type",
						FieldType: FieldTypeString,
						Values:    []string{}, // 空数组
						QueryType: ptr.Of(QueryTypeEnumNotMatch),
					},
				},
			},
			span: &Span{
				SpanType: "span_type_a",
			},
			satisfied: true, // 空值时应该返回true
		},
		{
			name: "Empty string in values should work correctly",
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "span_type",
						FieldType: FieldTypeString,
						Values:    []string{""}, // 包含空字符串
						QueryType: ptr.Of(QueryTypeEnumNotMatch),
					},
				},
			},
			span: &Span{
				SpanType: "span_type_a",
			},
			satisfied: false, // span_type_a 包含空字符串（任何字符串都包含空字符串）
		},
		{
			name: "Empty string in values with empty span field",
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "span_type",
						FieldType: FieldTypeString,
						Values:    []string{""}, // 包含空字符串
						QueryType: ptr.Of(QueryTypeEnumNotMatch),
					},
				},
			},
			span: &Span{
				SpanType: "", // 空字符串字段
			},
			satisfied: false, // 空字符串包含空字符串
		},
		{
			name: "Nil span field should return false",
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "non_existent_field",
						FieldType: FieldTypeString,
						Values:    []string{"test"},
						QueryType: ptr.Of(QueryTypeEnumNotMatch),
					},
				},
			},
			span: &Span{
				SpanType: "span_type_a",
			},
			satisfied: true, // 不存在的字段返回nil，nil不包含任何内容，所以NotMatch应该返回true
		},
		{
			name: "Multiple values should only use first value",
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "span_type",
						FieldType: FieldTypeString,
						Values:    []string{"_a", "_b", "_c"}, // 多个值，只应该使用第一个
						QueryType: ptr.Of(QueryTypeEnumNotMatch),
					},
				},
			},
			span: &Span{
				SpanType: "span_type_a",
			},
			satisfied: false, // span_type_a 包含 "_a"（第一个值）
		},
		{
			name: "Multiple values with first not matching",
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "span_type",
						FieldType: FieldTypeString,
						Values:    []string{"_x", "_a", "_b"}, // 多个值，第一个不匹配
						QueryType: ptr.Of(QueryTypeEnumNotMatch),
					},
				},
			},
			span: &Span{
				SpanType: "span_type_a",
			},
			satisfied: true, // span_type_a 不包含 "_x"（第一个值）
		},
		// 类型错误测试
		{
			name: "Non-string field type should return false",
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: SpanFieldStatusCode, // 这是一个long类型字段
						FieldType: FieldTypeLong,       // 但我们尝试用string的查询类型
						Values:    []string{"100"},
						QueryType: ptr.Of(QueryTypeEnumNotMatch), // 这个查询类型只支持string
					},
				},
			},
			span: &Span{
				StatusCode: 100,
			},
			satisfied: false, // 非字符串类型应该返回false
		},
		// 组合场景测试
		{
			name: "NotMatch with other query types using AND",
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "span_type",
						FieldType: FieldTypeString,
						Values:    []string{"_b"},
						QueryType: ptr.Of(QueryTypeEnumNotMatch),
					},
					{
						FieldName: "service_name",
						FieldType: FieldTypeString,
						Values:    []string{"service_name_a"},
						QueryType: ptr.Of(QueryTypeEnumEq),
					},
				},
			},
			span: &Span{
				SpanType: "span_type_a",
				TagsString: map[string]string{
					"service_name": "service_name_a",
				},
			},
			satisfied: true, // span_type_a不包含"_b" AND service_name等于"service_name_a"
		},
		{
			name: "NotMatch with other query types using OR",
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumOr),
				FilterFields: []*FilterField{
					{
						FieldName: "span_type",
						FieldType: FieldTypeString,
						Values:    []string{"_a"},
						QueryType: ptr.Of(QueryTypeEnumNotMatch),
					},
					{
						FieldName: "service_name",
						FieldType: FieldTypeString,
						Values:    []string{"service_name_b"},
						QueryType: ptr.Of(QueryTypeEnumEq),
					},
				},
			},
			span: &Span{
				SpanType: "span_type_a",
				TagsString: map[string]string{
					"service_name": "service_name_a",
				},
			},
			satisfied: false, // span_type_a包含"_a" OR service_name不等于"service_name_b" = false OR false = false
		},
		{
			name: "Complex nested filters with NotMatch",
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "span_type",
						FieldType: FieldTypeString,
						Values:    []string{"_test"},
						QueryType: ptr.Of(QueryTypeEnumNotMatch),
					},
					{
						SubFilter: &FilterFields{
							QueryAndOr: ptr.Of(QueryAndOrEnumOr),
							FilterFields: []*FilterField{
								{
									FieldName: "service_name",
									FieldType: FieldTypeString,
									Values:    []string{"service_name_a"},
									QueryType: ptr.Of(QueryTypeEnumEq),
								},
								{
									FieldName: "span_type",
									FieldType: FieldTypeString,
									Values:    []string{"_b"},
									QueryType: ptr.Of(QueryTypeEnumNotMatch),
								},
							},
						},
					},
				},
			},
			span: &Span{
				SpanType: "span_type_a",
				TagsString: map[string]string{
					"service_name": "service_name_a",
				},
			},
			satisfied: true, // span_type_a不包含"_test" AND (service_name等于"service_name_a" OR span_type_a不包含"_b") = true AND (true OR true) = true
		},
		// 特殊字符处理测试
		{
			name: "Special characters in match value",
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "span_type",
						FieldType: FieldTypeString,
						Values:    []string{"[special]"},
						QueryType: ptr.Of(QueryTypeEnumNotMatch),
					},
				},
			},
			span: &Span{
				SpanType: "span_type_[special]_test",
			},
			satisfied: false, // 包含特殊字符的匹配
		},
		{
			name: "Unicode characters in match value",
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "span_type",
						FieldType: FieldTypeString,
						Values:    []string{"测试"},
						QueryType: ptr.Of(QueryTypeEnumNotMatch),
					},
				},
			},
			span: &Span{
				SpanType: "span_type_测试_unicode",
			},
			satisfied: false, // 包含Unicode字符的匹配
		},
		{
			name: "Unicode characters not matching",
			filter: &FilterFields{
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				FilterFields: []*FilterField{
					{
						FieldName: "span_type",
						FieldType: FieldTypeString,
						Values:    []string{"测试"},
						QueryType: ptr.Of(QueryTypeEnumNotMatch),
					},
				},
			},
			span: &Span{
				SpanType: "span_type_english_only",
			},
			satisfied: true, // 不包含Unicode字符
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			satisfied := test.filter.Satisfied(test.span)
			assert.Equal(t, test.satisfied, satisfied, "Test case: %s", test.name)
		})
	}
}

// TestQueryTypeEnumNotMatchValidation 测试 QueryTypeEnumNotMatch 的验证逻辑
func TestQueryTypeEnumNotMatchValidation(t *testing.T) {
	// 测试有效的组合
	validFilter := &FilterFields{
		QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
		FilterFields: []*FilterField{
			{
				FieldName: "test_field",
				FieldType: FieldTypeString, // 只有string类型支持NotMatch
				Values:    []string{"test"},
				QueryType: ptr.Of(QueryTypeEnumNotMatch),
			},
		},
	}
	err := validFilter.Validate()
	assert.NoError(t, err, "Valid NotMatch filter should pass validation")

	// 测试无效的类型组合
	invalidFilters := []*FilterFields{
		{
			QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
			FilterFields: []*FilterField{
				{
					FieldName: "test_field",
					FieldType: FieldTypeLong, // long类型不支持NotMatch
					Values:    []string{"123"},
					QueryType: ptr.Of(QueryTypeEnumNotMatch),
				},
			},
		},
		{
			QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
			FilterFields: []*FilterField{
				{
					FieldName: "test_field",
					FieldType: FieldTypeDouble, // double类型不支持NotMatch
					Values:    []string{"123.45"},
					QueryType: ptr.Of(QueryTypeEnumNotMatch),
				},
			},
		},
		{
			QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
			FilterFields: []*FilterField{
				{
					FieldName: "test_field",
					FieldType: FieldTypeBool, // bool类型不支持NotMatch
					Values:    []string{"true"},
					QueryType: ptr.Of(QueryTypeEnumNotMatch),
				},
			},
		},
	}

	for i, filter := range invalidFilters {
		err := filter.Validate()
		assert.Error(t, err, "Invalid NotMatch filter %d should fail validation", i)
	}
}
