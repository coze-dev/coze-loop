// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"testing"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
	kitexspan "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/stretchr/testify/assert"
)

func TestSpanDO2DTO(t *testing.T) {
	tests := []struct {
		name             string
		span             *loop_span.Span
		needOriginalTags bool
	}{
		{
			name: "basic conversion",
			span: &loop_span.Span{
				TraceID:         "trace-1",
				SpanID:          "span-1",
				ParentID:        "parent-1",
				SpanName:        "test-span",
				SpanType:        loop_span.SpanTypeModel,
				CallType:        "http",
				StartTime:       1_234_567,
				DurationMicros:  2_345_678,
				StatusCode:      0,
				Input:           "input",
				Output:          "output",
				LogicDeleteTime: 3_456_789,
				PSM:             "test-service",
				LogID:           "logid-1",
				SystemTagsString: map[string]string{
					"normal":                              "value",
					loop_span.SpanFieldStartTimeFirstResp: "1000",
				},
				SystemTagsLong: map[string]int64{
					loop_span.SpanFieldLatencyFirstResp: 2000,
				},
				SystemTagsDouble: map[string]float64{
					"double": 1.5,
				},
				TagsString: map[string]string{
					"tag": "value",
				},
				TagsLong: map[string]int64{
					loop_span.SpanFieldLatencyFirstResp: 2000,
				},
				TagsDouble: map[string]float64{
					"double_tag": 2.5,
				},
				TagsBool: map[string]bool{
					"bool_tag": true,
				},
				TagsByte: map[string]string{
					"bytes_tag": "0101",
				},
				AttrTos: &loop_span.AttrTos{
					InputDataURL:   "input-url",
					OutputDataURL:  "output-url",
					MultimodalData: map[string]string{"key": "value"},
				},
			},
			needOriginalTags: false,
		},
		{
			name: "with original tags",
			span: &loop_span.Span{
				TraceID:         "trace-2",
				SpanID:          "span-2",
				ParentID:        "parent-2",
				SpanName:        "span-original",
				SpanType:        "unknown-type",
				CallType:        "",
				StartTime:       5_000,
				DurationMicros:  6_000,
				StatusCode:      1,
				Input:           "input-2",
				Output:          "output-2",
				LogicDeleteTime: 0,
				SystemTagsString: map[string]string{
					"keep": "origin",
				},
				SystemTagsLong: map[string]int64{
					"long": 5,
				},
				SystemTagsDouble: map[string]float64{
					"double": 1.25,
				},
				TagsString: map[string]string{
					"tag": "val",
				},
				TagsLong: map[string]int64{
					"long": 10,
				},
				TagsDouble: map[string]float64{
					"double": 2.75,
				},
				TagsBool: map[string]bool{
					"flag": true,
				},
				TagsByte: map[string]string{
					"bytes": "data",
				},
			},
			needOriginalTags: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SpanDO2DTO(tt.span, nil, nil, nil, tt.needOriginalTags)

			assert.Equal(t, tt.span.TraceID, got.TraceID)
			assert.Equal(t, tt.span.SpanID, got.SpanID)
			assert.Equal(t, tt.span.ParentID, got.ParentID)
			assert.Equal(t, tt.span.SpanName, got.SpanName)
			assert.Equal(t, tt.span.SpanType, got.SpanType)

			if tt.span.SpanType == loop_span.SpanTypeModel {
				assert.Equal(t, kitexspan.SpanTypeModel, got.GetType())
			} else {
				assert.Equal(t, kitexspan.SpanTypeUnknown, got.GetType())
			}

			expectedStatus := kitexspan.SpanStatusError
			if tt.span.StatusCode == 0 {
				expectedStatus = kitexspan.SpanStatusSuccess
			}
			assert.Equal(t, expectedStatus, got.GetStatus())

			assert.Equal(t, tt.span.StatusCode, got.GetStatusCode())
			assert.Equal(t, tt.span.Input, got.GetInput())
			assert.Equal(t, tt.span.Output, got.GetOutput())
			assert.Equal(t, tt.span.StartTime/1000, got.GetStartedAt())
			assert.Equal(t, tt.span.DurationMicros/1000, got.GetDuration())

			if assert.NotNil(t, got.LogicDeleteDate) {
				assert.Equal(t, tt.span.LogicDeleteTime/1000, *got.LogicDeleteDate)
			}

			if tt.span.PSM != "" {
				if assert.NotNil(t, got.ServiceName) {
					assert.Equal(t, tt.span.PSM, *got.ServiceName)
				}
			} else {
				assert.Nil(t, got.ServiceName)
			}

			if tt.span.LogID != "" {
				if assert.NotNil(t, got.Logid) {
					assert.Equal(t, tt.span.LogID, *got.Logid)
				}
			} else {
				assert.Nil(t, got.Logid)
			}

			if assert.NotNil(t, got.CallType) {
				assert.Equal(t, tt.span.CallType, *got.CallType)
			}

			if tt.span.AttrTos != nil {
				if assert.NotNil(t, got.AttrTos) {
					assert.Equal(t, tt.span.AttrTos.InputDataURL, got.AttrTos.GetInputDataURL())
					assert.Equal(t, tt.span.AttrTos.OutputDataURL, got.AttrTos.GetOutputDataURL())
					assert.Equal(t, tt.span.AttrTos.MultimodalData, got.AttrTos.GetMultimodalData())
				}
			}

			systemTags := got.GetSystemTags()
			customTags := got.GetCustomTags()
			if !tt.needOriginalTags {
				assert.Equal(t, "value", systemTags["normal"]) //nolint:testifylint
				assert.Equal(t, "1", systemTags[loop_span.SpanFieldStartTimeFirstResp])
				assert.Equal(t, "2", systemTags[loop_span.SpanFieldLatencyFirstResp])
				assert.Equal(t, "1.5", systemTags["double"])
				assert.Equal(t, "value", customTags["tag"]) //nolint:testifylint
				assert.Equal(t, "2", customTags[loop_span.SpanFieldLatencyFirstResp])
				assert.Equal(t, "2.5", customTags["double_tag"])
				assert.Equal(t, "true", customTags["bool_tag"])
				assert.Equal(t, "0101", customTags["bytes_tag"])
				assert.Nil(t, got.SystemTagsString)
				assert.Nil(t, got.SystemTagsLong)
				assert.Nil(t, got.SystemTagsDouble)
				assert.Nil(t, got.TagsString)
				assert.Nil(t, got.TagsLong)
				assert.Nil(t, got.TagsDouble)
				assert.Nil(t, got.TagsBool)
				assert.Nil(t, got.TagsBytes)
			} else {
				assert.Equal(t, "origin", got.SystemTagsString["keep"])
				assert.Equal(t, tt.span.SystemTagsLong, got.SystemTagsLong)
				assert.Equal(t, tt.span.SystemTagsDouble, got.SystemTagsDouble)
				assert.Equal(t, tt.span.TagsString, got.TagsString)
				assert.Equal(t, tt.span.TagsLong, got.TagsLong)
				assert.Equal(t, tt.span.TagsDouble, got.TagsDouble)
				assert.Equal(t, tt.span.TagsBool, got.TagsBool)
				assert.Equal(t, tt.span.TagsByte, got.TagsBytes)
			}
		})
	}
}

func TestSpanDTO2DO(t *testing.T) {
	durationMicros := int64(222)
	input := &kitexspan.InputSpan{
		StartedAtMicros: 111,
		SpanID:          "span",
		ParentID:        "parent",
		TraceID:         "trace",
		Duration:        333,
		CallType:        ptr.Of("grpc"),
		WorkspaceID:     "workspace",
		SpanName:        "name",
		SpanType:        "type",
		Method:          "method",
		StatusCode:      1,
		Input:           "in",
		Output:          "out",
		ObjectStorage:   ptr.Of("tos-key"),
		SystemTagsString: map[string]string{
			"sys": "val",
		},
		TagsString: map[string]string{
			"tag": "val",
		},
		DurationMicros: &durationMicros,
		LogID:          ptr.Of("log"),
		ServiceName:    ptr.Of("service"),
	}

	got := SpanDTO2DO(input)

	assert.Equal(t, input.StartedAtMicros, got.StartTime)
	assert.Equal(t, *input.DurationMicros, got.DurationMicros)
	assert.Equal(t, "grpc", got.CallType)
	assert.Equal(t, *input.ServiceName, got.PSM)
	assert.Equal(t, *input.LogID, got.LogID)
	assert.Equal(t, input.SystemTagsString, got.SystemTagsString)
	assert.Equal(t, input.TagsString, got.TagsString)
	assert.Equal(t, ptr.From(input.ObjectStorage), got.ObjectStorage)
}

func TestSpanListConversions(t *testing.T) {
	span1 := &loop_span.Span{SpanID: "1"}
	span2 := &loop_span.Span{SpanID: "2"}
	list := loop_span.SpanList{span1, span2}

	dto := SpanListDO2DTO(list, nil, nil, nil, false)
	assert.Len(t, dto, 2)
	assert.Equal(t, span1.SpanID, dto[0].SpanID)
	assert.Equal(t, span2.SpanID, dto[1].SpanID)

	input1 := &kitexspan.InputSpan{SpanID: "a", ParentID: "p", TraceID: "t", StartedAtMicros: 1, Duration: 2, SpanName: "n", SpanType: "type", Method: "m", StatusCode: 0, Input: "in", Output: "out", WorkspaceID: "w"}
	input2 := &kitexspan.InputSpan{SpanID: "b", ParentID: "p2", TraceID: "t2", StartedAtMicros: 3, Duration: 4, SpanName: "n2", SpanType: "type", Method: "m2", StatusCode: 1, Input: "in2", Output: "out2", WorkspaceID: "w2"}

	doList := SpanListDTO2DO([]*kitexspan.InputSpan{input1, input2})
	assert.Len(t, doList, 2)
	assert.Equal(t, input1.SpanID, doList[0].SpanID)
	assert.Equal(t, input2.SpanID, doList[1].SpanID)
}

func TestFilterFieldsDTO2DO(t *testing.T) {
	relation := filter.QueryRelationOr
	fieldType := filter.FieldTypeLong
	queryType := filter.QueryTypeGte
	subFieldName := "sub"
	field := &filter.FilterField{
		FieldName:  ptr.Of("field"),
		FieldType:  &fieldType,
		Values:     []string{"1", "2"},
		QueryType:  &queryType,
		QueryAndOr: &relation,
		SubFilter: &filter.FilterFields{
			QueryAndOr: ptr.Of(filter.QueryRelationAnd),
			FilterFields: []*filter.FilterField{
				{
					FieldName: &subFieldName,
					Values:    []string{"sub-val"},
				},
				nil,
			},
		},
		IsCustom: ptr.Of(true),
	}
	fields := &filter.FilterFields{
		QueryAndOr: &relation,
		FilterFields: []*filter.FilterField{
			field,
		},
	}

	converted := FilterFieldsDTO2DO(fields)
	assert.NotNil(t, converted)
	if assert.NotNil(t, converted.QueryAndOr) {
		assert.Equal(t, loop_span.QueryAndOrEnum(relation), *converted.QueryAndOr)
	}
	assert.Len(t, converted.FilterFields, 1)
	convertedField := converted.FilterFields[0]
	assert.Equal(t, "field", convertedField.FieldName)
	assert.Equal(t, loop_span.FieldType(fieldType), convertedField.FieldType)
	assert.Equal(t, []string{"1", "2"}, convertedField.Values)
	if assert.NotNil(t, convertedField.QueryType) {
		assert.Equal(t, loop_span.QueryTypeEnum(queryType), *convertedField.QueryType)
	}
	if assert.NotNil(t, convertedField.QueryAndOr) {
		assert.Equal(t, loop_span.QueryAndOrEnum(relation), *convertedField.QueryAndOr)
	}
	assert.True(t, convertedField.IsCustom)
	if assert.NotNil(t, convertedField.SubFilter) {
		assert.Len(t, convertedField.SubFilter.FilterFields, 1)
		assert.Equal(t, subFieldName, convertedField.SubFilter.FilterFields[0].FieldName)
	}

	assert.Nil(t, FilterFieldsDTO2DO(nil))
}

func TestFilterFieldListDTO2DO(t *testing.T) {
	list := FilterFieldListDTO2DO([]*filter.FilterField{
		nil,
		&filter.FilterField{
			FieldName: ptr.Of("name"),
		},
	})
	assert.Len(t, list, 1)
	assert.Equal(t, "name", list[0].FieldName)
}

func TestFieldTypeDTO2DO(t *testing.T) {
	fieldType := filter.FieldTypeDouble
	assert.Equal(t, loop_span.FieldType(fieldType), fieldTypeDTO2DO(&fieldType))
	assert.Equal(t, loop_span.FieldTypeString, fieldTypeDTO2DO(nil))
}
