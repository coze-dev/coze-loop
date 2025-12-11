// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/ck/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/dao"
	"github.com/stretchr/testify/assert"
)

func TestSpanListPO2CKModels(t *testing.T) {
	method1 := "POST"
	psm1 := "service-1"
	logid1 := "log-1"
	callType1 := "http"
	objectStorage1 := "tos://bucket/key1"
	reserveCreateTime1 := "2222222222"
	method2 := "GET"
	psm2 := "service-2"
	logid2 := "log-2"
	callType2 := "grpc"
	objectStorage2 := "tos://bucket/key2"

	tests := []struct {
		name  string
		spans []*dao.Span
	}{
		{
			name: "multiple spans conversion",
			spans: []*dao.Span{
				{
					TraceID:           "trace-1",
					SpanID:            "span-1",
					SpaceID:           "space-1",
					SpanType:          "model",
					SpanName:          "test-span-1",
					ParentID:          "parent-1",
					Method:            &method1,
					Psm:               &psm1,
					Logid:             &logid1,
					StartTime:         1234567890,
					CallType:          &callType1,
					Duration:          987654321,
					StatusCode:        0,
					ObjectStorage:     &objectStorage1,
					LogicDeleteDate:   1111111111,
					ReserveCreateTime: &reserveCreateTime1,
					TagsBool: map[string]uint8{
						"bool1": 1,
					},
					TagsFloat: map[string]float64{
						"float1": 1.23,
					},
					TagsString: map[string]string{
						"str1": "value1",
					},
					TagsLong: map[string]int64{
						"long1": 123,
					},
					TagsByte: map[string]string{
						"bytes1": "0101",
					},
					SystemTagsFloat: map[string]float64{
						"sys_float1": 4.56,
					},
					SystemTagsLong: map[string]int64{
						"sys_long1": 456,
					},
					SystemTagsString: map[string]string{
						"sys_str1": "sys_value1",
					},
				},
				{
					TraceID:       "trace-2",
					SpanID:        "span-2",
					SpaceID:       "space-2",
					SpanType:      "prompt",
					SpanName:      "test-span-2",
					ParentID:      "parent-2",
					Method:        &method2,
					Psm:           &psm2,
					Logid:         &logid2,
					StartTime:     1234567891,
					CallType:      &callType2,
					Duration:      987654322,
					StatusCode:    1,
					ObjectStorage: &objectStorage2,
					TagsBool: map[string]uint8{
						"bool2": 0,
					},
					TagsFloat: map[string]float64{
						"float2": 2.34,
					},
					TagsString: map[string]string{
						"str2": "value2",
					},
					TagsLong: map[string]int64{
						"long2": 234,
					},
					TagsByte: map[string]string{
						"bytes2": "1010",
					},
					SystemTagsFloat: map[string]float64{
						"sys_float2": 5.67,
					},
					SystemTagsLong: map[string]int64{
						"sys_long2": 567,
					},
					SystemTagsString: map[string]string{
						"sys_str2": "sys_value2",
					},
				},
			},
		},
		{
			name:  "empty spans list",
			spans: []*dao.Span{},
		},
		{
			name:  "nil spans list",
			spans: nil,
		},
		{
			name: "spans with nil elements",
			spans: []*dao.Span{
				{
					SpanID:    "valid-span",
					TraceID:   "valid-trace",
					SpaceID:   "valid-space",
					SpanName:  "valid-name",
					StartTime: 1234567890,
				},
				nil,
				{
					SpanID:    "another-valid",
					TraceID:   "another-trace",
					SpaceID:   "another-space",
					SpanName:  "another-name",
					StartTime: 1234567891,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SpanListPO2CKModels(tt.spans)

			if tt.spans == nil {
				assert.NotNil(t, result)
				assert.Len(t, result, 0)
				return
			}

			assert.NotNil(t, result)
			assert.Len(t, result, len(tt.spans))

			for i, span := range tt.spans {
				if span == nil {
					// SpanPO2CKModel 会处理 nil，返回 nil
					assert.Nil(t, result[i])
				} else {
					assert.NotNil(t, result[i])
					assert.Equal(t, span.TraceID, result[i].TraceID)
					assert.Equal(t, span.SpanID, result[i].SpanID)
					assert.Equal(t, span.SpaceID, result[i].SpaceID)
					assert.Equal(t, span.SpanType, result[i].SpanType)
					assert.Equal(t, span.SpanName, result[i].SpanName)
					assert.Equal(t, span.ParentID, result[i].ParentID)
					assert.Equal(t, span.Method, result[i].Method)
					assert.Equal(t, span.Psm, result[i].Psm)
					assert.Equal(t, span.Logid, result[i].Logid)
					assert.Equal(t, span.StartTime, result[i].StartTime)
					assert.Equal(t, span.CallType, result[i].CallType)
					assert.Equal(t, span.Duration, result[i].Duration)
					assert.Equal(t, span.StatusCode, result[i].StatusCode)
					assert.Equal(t, span.ObjectStorage, result[i].ObjectStorage)
					assert.Equal(t, span.LogicDeleteDate, result[i].LogicDeleteDate)
					assert.Equal(t, span.ReserveCreateTime, result[i].ReserveCreateTime)
					assert.Equal(t, span.TagsBool, result[i].TagsBool)
					assert.Equal(t, span.TagsFloat, result[i].TagsFloat)
					assert.Equal(t, span.TagsString, result[i].TagsString)
					assert.Equal(t, span.TagsLong, result[i].TagsLong)
					assert.Equal(t, span.TagsByte, result[i].TagsByte)
					assert.Equal(t, span.SystemTagsFloat, result[i].SystemTagsFloat)
					assert.Equal(t, span.SystemTagsLong, result[i].SystemTagsLong)
					assert.Equal(t, span.SystemTagsString, result[i].SystemTagsString)
				}
			}
		})
	}
}

func TestSpanListCKModels2PO(t *testing.T) {
	method1 := "POST"
	psm1 := "service-1"
	logid1 := "log-1"
	callType1 := "http"
	objectStorage1 := "tos://bucket/key1"
	reserveCreateTime1 := "2222222222"
	method2 := "GET"
	psm2 := "service-2"
	logid2 := "log-2"
	callType2 := "grpc"
	objectStorage2 := "tos://bucket/key2"

	tests := []struct {
		name  string
		spans []*model.ObservabilitySpan
	}{
		{
			name: "multiple ck models conversion",
			spans: []*model.ObservabilitySpan{
				{
					TraceID:           "trace-1",
					SpanID:            "span-1",
					SpaceID:           "space-1",
					SpanType:          "model",
					SpanName:          "test-span-1",
					ParentID:          "parent-1",
					Method:            &method1,
					Psm:               &psm1,
					Logid:             &logid1,
					StartTime:         1234567890,
					CallType:          &callType1,
					Duration:          987654321,
					StatusCode:        0,
					ObjectStorage:     &objectStorage1,
					LogicDeleteDate:   1111111111,
					ReserveCreateTime: &reserveCreateTime1,
					TagsBool: map[string]uint8{
						"bool1": 1,
					},
					TagsFloat: map[string]float64{
						"float1": 1.23,
					},
					TagsString: map[string]string{
						"str1": "value1",
					},
					TagsLong: map[string]int64{
						"long1": 123,
					},
					TagsByte: map[string]string{
						"bytes1": "0101",
					},
					SystemTagsFloat: map[string]float64{
						"sys_float1": 4.56,
					},
					SystemTagsLong: map[string]int64{
						"sys_long1": 456,
					},
					SystemTagsString: map[string]string{
						"sys_str1": "sys_value1",
					},
				},
				{
					TraceID:       "trace-2",
					SpanID:        "span-2",
					SpaceID:       "space-2",
					SpanType:      "prompt",
					SpanName:      "test-span-2",
					ParentID:      "parent-2",
					Method:        &method2,
					Psm:           &psm2,
					Logid:         &logid2,
					StartTime:     1234567891,
					CallType:      &callType2,
					Duration:      987654322,
					StatusCode:    1,
					ObjectStorage: &objectStorage2,
					TagsBool: map[string]uint8{
						"bool2": 0,
					},
					TagsFloat: map[string]float64{
						"float2": 2.34,
					},
					TagsString: map[string]string{
						"str2": "value2",
					},
					TagsLong: map[string]int64{
						"long2": 234,
					},
					TagsByte: map[string]string{
						"bytes2": "1010",
					},
					SystemTagsFloat: map[string]float64{
						"sys_float2": 5.67,
					},
					SystemTagsLong: map[string]int64{
						"sys_long2": 567,
					},
					SystemTagsString: map[string]string{
						"sys_str2": "sys_value2",
					},
				},
			},
		},
		{
			name:  "empty ck models list",
			spans: []*model.ObservabilitySpan{},
		},
		{
			name:  "nil ck models list",
			spans: nil,
		},
		{
			name: "ck models with nil elements",
			spans: []*model.ObservabilitySpan{
				{
					SpanID:    "valid-span",
					TraceID:   "valid-trace",
					SpaceID:   "valid-space",
					SpanName:  "valid-name",
					StartTime: 1234567890,
				},
				nil,
				{
					SpanID:    "another-valid",
					TraceID:   "another-trace",
					SpaceID:   "another-space",
					SpanName:  "another-name",
					StartTime: 1234567891,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SpanListCKModels2PO(tt.spans)

			if tt.spans == nil {
				assert.NotNil(t, result)
				assert.Len(t, result, 0)
				return
			}

			assert.NotNil(t, result)
			assert.Len(t, result, len(tt.spans))

			for i, span := range tt.spans {
				if span == nil {
					// SpanCKModel2PO 会处理 nil，返回 nil
					assert.Nil(t, result[i])
				} else {
					assert.NotNil(t, result[i])
					assert.Equal(t, span.TraceID, result[i].TraceID)
					assert.Equal(t, span.SpanID, result[i].SpanID)
					assert.Equal(t, span.SpaceID, result[i].SpaceID)
					assert.Equal(t, span.SpanType, result[i].SpanType)
					assert.Equal(t, span.SpanName, result[i].SpanName)
					assert.Equal(t, span.ParentID, result[i].ParentID)
					assert.Equal(t, span.Method, result[i].Method)
					assert.Equal(t, span.Psm, result[i].Psm)
					assert.Equal(t, span.Logid, result[i].Logid)
					assert.Equal(t, span.StartTime, result[i].StartTime)
					assert.Equal(t, span.CallType, result[i].CallType)
					assert.Equal(t, span.Duration, result[i].Duration)
					assert.Equal(t, span.StatusCode, result[i].StatusCode)
					assert.Equal(t, span.ObjectStorage, result[i].ObjectStorage)
					assert.Equal(t, span.LogicDeleteDate, result[i].LogicDeleteDate)
					assert.Equal(t, span.ReserveCreateTime, result[i].ReserveCreateTime)
					assert.Equal(t, span.TagsBool, result[i].TagsBool)
					assert.Equal(t, span.TagsFloat, result[i].TagsFloat)
					assert.Equal(t, span.TagsString, result[i].TagsString)
					assert.Equal(t, span.TagsLong, result[i].TagsLong)
					assert.Equal(t, span.TagsByte, result[i].TagsByte)
					assert.Equal(t, span.SystemTagsFloat, result[i].SystemTagsFloat)
					assert.Equal(t, span.SystemTagsLong, result[i].SystemTagsLong)
					assert.Equal(t, span.SystemTagsString, result[i].SystemTagsString)
				}
			}
		})
	}
}

func TestSpanPO2CKModel(t *testing.T) {
	method := "POST"
	psm := "test-service"
	logid := "log-456"
	callType := "http"
	objectStorage := "tos://bucket/key"
	reserveCreateTime := "2222222222"

	tests := []struct {
		name string
		span *dao.Span
	}{
		{
			name: "complete span conversion",
			span: &dao.Span{
				TraceID:           "trace-123",
				SpanID:            "span-456",
				SpaceID:           "space-789",
				SpanType:          "model",
				SpanName:          "test-span",
				ParentID:          "parent-123",
				Method:            &method,
				Psm:               &psm,
				Logid:             &logid,
				StartTime:         1234567890,
				CallType:          &callType,
				Duration:          987654321,
				StatusCode:        0,
				ObjectStorage:     &objectStorage,
				LogicDeleteDate:   1111111111,
				ReserveCreateTime: &reserveCreateTime,
				TagsBool: map[string]uint8{
					"bool_key": 1,
				},
				TagsFloat: map[string]float64{
					"float_key": 1.23,
				},
				TagsString: map[string]string{
					"str_key": "str_value",
				},
				TagsLong: map[string]int64{
					"long_key": 123,
				},
				TagsByte: map[string]string{
					"bytes_key": "0101",
				},
				SystemTagsFloat: map[string]float64{
					"sys_float_key": 4.56,
				},
				SystemTagsLong: map[string]int64{
					"sys_long_key": 456,
				},
				SystemTagsString: map[string]string{
					"sys_str_key": "sys_str_value",
				},
			},
		},
		{
			name: "minimal span conversion",
			span: &dao.Span{
				SpanID:    "minimal",
				TraceID:   "trace-min",
				SpaceID:   "space-min",
				SpanName:  "minimal-span",
				StartTime: 0,
			},
		},
		{
			name: "nil span",
			span: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SpanPO2CKModel(tt.span)

			if tt.span == nil {
				assert.Nil(t, result)
				return
			}

			assert.NotNil(t, result)
			assert.Equal(t, tt.span.TraceID, result.TraceID)
			assert.Equal(t, tt.span.SpanID, result.SpanID)
			assert.Equal(t, tt.span.SpaceID, result.SpaceID)
			assert.Equal(t, tt.span.SpanType, result.SpanType)
			assert.Equal(t, tt.span.SpanName, result.SpanName)
			assert.Equal(t, tt.span.ParentID, result.ParentID)
			assert.Equal(t, tt.span.Method, result.Method)
			assert.Equal(t, tt.span.Psm, result.Psm)
			assert.Equal(t, tt.span.Logid, result.Logid)
			assert.Equal(t, tt.span.StartTime, result.StartTime)
			assert.Equal(t, tt.span.CallType, result.CallType)
			assert.Equal(t, tt.span.Duration, result.Duration)
			assert.Equal(t, tt.span.StatusCode, result.StatusCode)
			assert.Equal(t, tt.span.ObjectStorage, result.ObjectStorage)
			assert.Equal(t, tt.span.LogicDeleteDate, result.LogicDeleteDate)
			assert.Equal(t, tt.span.ReserveCreateTime, result.ReserveCreateTime)
			assert.Equal(t, tt.span.TagsBool, result.TagsBool)
			assert.Equal(t, tt.span.TagsFloat, result.TagsFloat)
			assert.Equal(t, tt.span.TagsString, result.TagsString)
			assert.Equal(t, tt.span.TagsLong, result.TagsLong)
			assert.Equal(t, tt.span.TagsByte, result.TagsByte)
			assert.Equal(t, tt.span.SystemTagsFloat, result.SystemTagsFloat)
			assert.Equal(t, tt.span.SystemTagsLong, result.SystemTagsLong)
			assert.Equal(t, tt.span.SystemTagsString, result.SystemTagsString)
		})
	}
}

func TestSpanCKModel2PO(t *testing.T) {
	method := "POST"
	psm := "test-service"
	logid := "log-456"
	callType := "http"
	objectStorage := "tos://bucket/key"
	reserveCreateTime := "2222222222"

	tests := []struct {
		name string
		span *model.ObservabilitySpan
	}{
		{
			name: "complete ck model conversion",
			span: &model.ObservabilitySpan{
				TraceID:           "trace-123",
				SpanID:            "span-456",
				SpaceID:           "space-789",
				SpanType:          "model",
				SpanName:          "test-span",
				ParentID:          "parent-123",
				Method:            &method,
				Psm:               &psm,
				Logid:             &logid,
				StartTime:         1234567890,
				CallType:          &callType,
				Duration:          987654321,
				StatusCode:        0,
				ObjectStorage:     &objectStorage,
				LogicDeleteDate:   1111111111,
				ReserveCreateTime: &reserveCreateTime,
				TagsBool: map[string]uint8{
					"bool_key": 1,
				},
				TagsFloat: map[string]float64{
					"float_key": 1.23,
				},
				TagsString: map[string]string{
					"str_key": "str_value",
				},
				TagsLong: map[string]int64{
					"long_key": 123,
				},
				TagsByte: map[string]string{
					"bytes_key": "0101",
				},
				SystemTagsFloat: map[string]float64{
					"sys_float_key": 4.56,
				},
				SystemTagsLong: map[string]int64{
					"sys_long_key": 456,
				},
				SystemTagsString: map[string]string{
					"sys_str_key": "sys_str_value",
				},
			},
		},
		{
			name: "minimal ck model conversion",
			span: &model.ObservabilitySpan{
				SpanID:    "minimal",
				TraceID:   "trace-min",
				SpaceID:   "space-min",
				SpanName:  "minimal-span",
				StartTime: 0,
			},
		},
		{
			name: "nil ck model",
			span: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SpanCKModel2PO(tt.span)

			if tt.span == nil {
				assert.Nil(t, result)
				return
			}

			assert.NotNil(t, result)
			assert.Equal(t, tt.span.TraceID, result.TraceID)
			assert.Equal(t, tt.span.SpanID, result.SpanID)
			assert.Equal(t, tt.span.SpaceID, result.SpaceID)
			assert.Equal(t, tt.span.SpanType, result.SpanType)
			assert.Equal(t, tt.span.SpanName, result.SpanName)
			assert.Equal(t, tt.span.ParentID, result.ParentID)
			assert.Equal(t, tt.span.Method, result.Method)
			assert.Equal(t, tt.span.Psm, result.Psm)
			assert.Equal(t, tt.span.Logid, result.Logid)
			assert.Equal(t, tt.span.StartTime, result.StartTime)
			assert.Equal(t, tt.span.CallType, result.CallType)
			assert.Equal(t, tt.span.Duration, result.Duration)
			assert.Equal(t, tt.span.StatusCode, result.StatusCode)
			assert.Equal(t, tt.span.ObjectStorage, result.ObjectStorage)
			assert.Equal(t, tt.span.LogicDeleteDate, result.LogicDeleteDate)
			assert.Equal(t, tt.span.ReserveCreateTime, result.ReserveCreateTime)
			assert.Equal(t, tt.span.TagsBool, result.TagsBool)
			assert.Equal(t, tt.span.TagsFloat, result.TagsFloat)
			assert.Equal(t, tt.span.TagsString, result.TagsString)
			assert.Equal(t, tt.span.TagsLong, result.TagsLong)
			assert.Equal(t, tt.span.TagsByte, result.TagsByte)
			assert.Equal(t, tt.span.SystemTagsFloat, result.SystemTagsFloat)
			assert.Equal(t, tt.span.SystemTagsLong, result.SystemTagsLong)
			assert.Equal(t, tt.span.SystemTagsString, result.SystemTagsString)
		})
	}
}
