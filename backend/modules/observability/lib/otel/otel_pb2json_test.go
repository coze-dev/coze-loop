// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	v3 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	v2 "go.opentelemetry.io/proto/otlp/common/v1"
	v1resource "go.opentelemetry.io/proto/otlp/resource/v1"
	v1 "go.opentelemetry.io/proto/otlp/trace/v1"
)

func TestOtelTraceRequestPbToJson(t *testing.T) {
	tests := []struct {
		name     string
		input    *v3.ExportTraceServiceRequest
		expected *ExportTraceServiceRequest
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "empty request",
			input: &v3.ExportTraceServiceRequest{
				ResourceSpans: []*v1.ResourceSpans{},
			},
			expected: &ExportTraceServiceRequest{
				ResourceSpans: []*ResourceSpans{},
			},
		},
		{
			name: "complete request",
			input: func() *v3.ExportTraceServiceRequest {
				traceID, _ := hex.DecodeString("0102030405060708090a0b0c0d0e0f10")
				spanID, _ := hex.DecodeString("0102030405060708")
				parentSpanID, _ := hex.DecodeString("0807060504030201")

				return &v3.ExportTraceServiceRequest{
					ResourceSpans: []*v1.ResourceSpans{
						{
							Resource: &v1resource.Resource{
								Attributes: []*v2.KeyValue{
									{
										Key: "service.name",
										Value: &v2.AnyValue{
											Value: &v2.AnyValue_StringValue{StringValue: "test-service"},
										},
									},
								},
							},
							SchemaUrl: "test-schema",
							ScopeSpans: []*v1.ScopeSpans{
								{
									Scope: &v2.InstrumentationScope{
										Name:    "test-scope",
										Version: "1.0.0",
									},
									SchemaUrl: "scope-schema",
									Spans: []*v1.Span{
										{
											TraceId:           traceID,
											SpanId:            spanID,
											ParentSpanId:      parentSpanID,
											Name:              "test-span",
											Kind:              v1.Span_SPAN_KIND_CLIENT,
											StartTimeUnixNano: 1640995200000000000,
											EndTimeUnixNano:   1640995201000000000,
											Attributes: []*v2.KeyValue{
												{
													Key: "test.key",
													Value: &v2.AnyValue{
														Value: &v2.AnyValue_StringValue{StringValue: "test-value"},
													},
												},
											},
											Events: []*v1.Span_Event{
												{
													TimeUnixNano: 1640995200500000000,
													Name:         "test-event",
													Attributes: []*v2.KeyValue{
														{
															Key: "event.key",
															Value: &v2.AnyValue{
																Value: &v2.AnyValue_StringValue{StringValue: "event-value"},
															},
														},
													},
												},
											},
											Links: []*v1.Span_Link{
												{
													TraceId: traceID,
													SpanId:  spanID,
													Flags:   1,
													Attributes: []*v2.KeyValue{
														{
															Key: "link.key",
															Value: &v2.AnyValue{
																Value: &v2.AnyValue_StringValue{StringValue: "link-value"},
															},
														},
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
			}(),
			expected: &ExportTraceServiceRequest{
				ResourceSpans: []*ResourceSpans{
					{
						Resource: &Resource{
							Attributes: []*KeyValue{
								{
									Key:   "service.name",
									Value: &AnyValue{Value: &AnyValue_StringValue{StringValue: "test-service"}},
								},
							},
						},
						SchemaUrl: "test-schema",
						ScopeSpans: []*ScopeSpans{
							{
								Scope: &InstrumentationScope{
									Name:       "test-scope",
									Version:    "1.0.0",
									Attributes: []*KeyValue{},
								},
								SchemaUrl: "scope-schema",
								Spans: []*Span{
									{
										TraceId:           "0102030405060708090a0b0c0d0e0f10",
										SpanId:            "0102030405060708",
										ParentSpanId:      "0807060504030201",
										Name:              "test-span",
										Kind:              v1.Span_SPAN_KIND_CLIENT,
										StartTimeUnixNano: "1640995200000000000",
										EndTimeUnixNano:   "1640995201000000000",
										Attributes: []*KeyValue{
											{
												Key:   "test.key",
												Value: &AnyValue{Value: &AnyValue_StringValue{StringValue: "test-value"}},
											},
										},
										Events: []*SpanEvent{
											{
												TimeUnixNano: "1640995200500000000",
												Name:         "test-event",
												Attributes: []*KeyValue{
													{
														Key:   "event.key",
														Value: &AnyValue{Value: &AnyValue_StringValue{StringValue: "event-value"}},
													},
												},
											},
										},
										Links: []*SpanLink{
											{
												TraceId: "0102030405060708090a0b0c0d0e0f10",
												SpanId:  "0102030405060708",
												Flags:   1,
												Attributes: []*KeyValue{
													{
														Key:   "link.key",
														Value: &AnyValue{Value: &AnyValue_StringValue{StringValue: "link-value"}},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "request with nil resource spans",
			input: &v3.ExportTraceServiceRequest{
				ResourceSpans: []*v1.ResourceSpans{nil},
			},
			expected: &ExportTraceServiceRequest{
				ResourceSpans: []*ResourceSpans{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := OtelTraceRequestPbToJson(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOtelAnyValuePbToJson(t *testing.T) {
	tests := []struct {
		name     string
		input    *v2.AnyValue
		expected *AnyValue
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "string value",
			input: &v2.AnyValue{
				Value: &v2.AnyValue_StringValue{StringValue: "test-string"},
			},
			expected: &AnyValue{
				Value: &AnyValue_StringValue{StringValue: "test-string"},
			},
		},
		{
			name: "bool value",
			input: &v2.AnyValue{
				Value: &v2.AnyValue_BoolValue{BoolValue: true},
			},
			expected: &AnyValue{
				Value: &AnyValue_BoolValue{BoolValue: true},
			},
		},
		{
			name: "int value",
			input: &v2.AnyValue{
				Value: &v2.AnyValue_IntValue{IntValue: 123},
			},
			expected: &AnyValue{
				Value: &AnyValue_IntValue{IntValue: 123},
			},
		},
		{
			name: "double value",
			input: &v2.AnyValue{
				Value: &v2.AnyValue_DoubleValue{DoubleValue: 123.45},
			},
			expected: &AnyValue{
				Value: &AnyValue_DoubleValue{DoubleValue: 123.45},
			},
		},
		{
			name: "bytes value",
			input: &v2.AnyValue{
				Value: &v2.AnyValue_BytesValue{BytesValue: []byte("test-bytes")},
			},
			expected: &AnyValue{
				Value: &AnyValue_BytesValue{BytesValue: []byte("test-bytes")},
			},
		},
		{
			name: "array value",
			input: &v2.AnyValue{
				Value: &v2.AnyValue_ArrayValue{
					ArrayValue: &v2.ArrayValue{
						Values: []*v2.AnyValue{
							{Value: &v2.AnyValue_StringValue{StringValue: "item1"}},
							{Value: &v2.AnyValue_IntValue{IntValue: 42}},
						},
					},
				},
			},
			expected: &AnyValue{
				Value: &AnyValue_ArrayValue{
					ArrayValue: &ArrayValue{
						Values: []*AnyValue{
							{Value: &AnyValue_StringValue{StringValue: "item1"}},
							{Value: &AnyValue_IntValue{IntValue: 42}},
						},
					},
				},
			},
		},
		{
			name: "kvlist value",
			input: &v2.AnyValue{
				Value: &v2.AnyValue_KvlistValue{
					KvlistValue: &v2.KeyValueList{
						Values: []*v2.KeyValue{
							{
								Key:   "key1",
								Value: &v2.AnyValue{Value: &v2.AnyValue_StringValue{StringValue: "value1"}},
							},
						},
					},
				},
			},
			expected: &AnyValue{
				Value: &AnyValue_KvlistValue{
					KvlistValue: &KeyValueList{
						Values: []*KeyValue{
							{
								Key:   "key1",
								Value: &AnyValue{Value: &AnyValue_StringValue{StringValue: "value1"}},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := otelAnyValuePbToJson(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOtelArrayValuePbToJson(t *testing.T) {
	tests := []struct {
		name     string
		input    *v2.ArrayValue
		expected *ArrayValue
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "empty array",
			input: &v2.ArrayValue{
				Values: []*v2.AnyValue{},
			},
			expected: &ArrayValue{
				Values: []*AnyValue{},
			},
		},
		{
			name: "array with values",
			input: &v2.ArrayValue{
				Values: []*v2.AnyValue{
					{Value: &v2.AnyValue_StringValue{StringValue: "item1"}},
					{Value: &v2.AnyValue_IntValue{IntValue: 42}},
				},
			},
			expected: &ArrayValue{
				Values: []*AnyValue{
					{Value: &AnyValue_StringValue{StringValue: "item1"}},
					{Value: &AnyValue_IntValue{IntValue: 42}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := otelArrayValuePbToJson(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOtelKeyValueListPbToJson(t *testing.T) {
	tests := []struct {
		name     string
		input    *v2.KeyValueList
		expected *KeyValueList
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "empty list",
			input: &v2.KeyValueList{
				Values: []*v2.KeyValue{},
			},
			expected: &KeyValueList{
				Values: []*KeyValue{},
			},
		},
		{
			name: "list with values",
			input: &v2.KeyValueList{
				Values: []*v2.KeyValue{
					{
						Key:   "key1",
						Value: &v2.AnyValue{Value: &v2.AnyValue_StringValue{StringValue: "value1"}},
					},
					{
						Key:   "key2",
						Value: &v2.AnyValue{Value: &v2.AnyValue_IntValue{IntValue: 123}},
					},
				},
			},
			expected: &KeyValueList{
				Values: []*KeyValue{
					{
						Key:   "key1",
						Value: &AnyValue{Value: &AnyValue_StringValue{StringValue: "value1"}},
					},
					{
						Key:   "key2",
						Value: &AnyValue{Value: &AnyValue_IntValue{IntValue: 123}},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := otelKeyValueListPbToJson(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOtelInstrumentationScopePbToJson(t *testing.T) {
	tests := []struct {
		name     string
		input    *v2.InstrumentationScope
		expected *InstrumentationScope
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "scope with attributes",
			input: &v2.InstrumentationScope{
				Name:    "test-scope",
				Version: "1.0.0",
				Attributes: []*v2.KeyValue{
					{
						Key:   "scope.key",
						Value: &v2.AnyValue{Value: &v2.AnyValue_StringValue{StringValue: "scope.value"}},
					},
				},
			},
			expected: &InstrumentationScope{
				Name:    "test-scope",
				Version: "1.0.0",
				Attributes: []*KeyValue{
					{
						Key:   "scope.key",
						Value: &AnyValue{Value: &AnyValue_StringValue{StringValue: "scope.value"}},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := otelInstrumentationScopePbToJson(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOtelSpanEventsPbToJson(t *testing.T) {
	tests := []struct {
		name     string
		input    []*v1.Span_Event
		expected []*SpanEvent
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty slice",
			input:    []*v1.Span_Event{},
			expected: nil,
		},
		{
			name: "events with nil element",
			input: []*v1.Span_Event{
				nil,
				{
					TimeUnixNano: 1640995200000000000,
					Name:         "test-event",
				},
			},
			expected: []*SpanEvent{
				{
					TimeUnixNano: "1640995200000000000",
					Name:         "test-event",
					Attributes:   nil,
				},
			},
		},
		{
			name: "events with attributes",
			input: []*v1.Span_Event{
				{
					TimeUnixNano:           1640995200000000000,
					Name:                   "test-event",
					DroppedAttributesCount: 1,
					Attributes: []*v2.KeyValue{
						{
							Key:   "event.key",
							Value: &v2.AnyValue{Value: &v2.AnyValue_StringValue{StringValue: "event.value"}},
						},
					},
				},
			},
			expected: []*SpanEvent{
				{
					TimeUnixNano:           "1640995200000000000",
					Name:                   "test-event",
					DroppedAttributesCount: 1,
					Attributes: []*KeyValue{
						{
							Key:   "event.key",
							Value: &AnyValue{Value: &AnyValue_StringValue{StringValue: "event.value"}},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := otelSpanEventsPbToJson(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOtelSpanLinksPbToJson(t *testing.T) {
	tests := []struct {
		name     string
		input    []*v1.Span_Link
		expected []*SpanLink
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty slice",
			input:    []*v1.Span_Link{},
			expected: nil,
		},
		{
			name: "links with nil element",
			input: []*v1.Span_Link{
				nil,
				{
					TraceId: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
					SpanId:  []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
					Flags:   1,
				},
			},
			expected: []*SpanLink{
				{
					TraceId: "0102030405060708090a0b0c0d0e0f10",
					SpanId:  "0102030405060708",
					Flags:   1,
				},
			},
		},
		{
			name: "links with attributes",
			input: []*v1.Span_Link{
				{
					TraceId:                []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
					SpanId:                 []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
					TraceState:             "trace-state",
					DroppedAttributesCount: 1,
					Flags:                  2,
					Attributes: []*v2.KeyValue{
						{
							Key:   "link.key",
							Value: &v2.AnyValue{Value: &v2.AnyValue_StringValue{StringValue: "link.value"}},
						},
					},
				},
			},
			expected: []*SpanLink{
				{
					TraceId:                "0102030405060708090a0b0c0d0e0f10",
					SpanId:                 "0102030405060708",
					TraceState:             "trace-state",
					DroppedAttributesCount: 1,
					Flags:                  2,
					Attributes: []*KeyValue{
						{
							Key:   "link.key",
							Value: &AnyValue{Value: &AnyValue_StringValue{StringValue: "link.value"}},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := otelSpanLinksPbToJson(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOtelAttributeListPbToJson(t *testing.T) {
	tests := []struct {
		name     string
		input    []*v2.KeyValue
		expected []*KeyValue
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty slice",
			input:    []*v2.KeyValue{},
			expected: nil,
		},
		{
			name: "attributes with nil element",
			input: []*v2.KeyValue{
				nil,
				{
					Key:   "test.key",
					Value: &v2.AnyValue{Value: &v2.AnyValue_StringValue{StringValue: "test.value"}},
				},
			},
			expected: []*KeyValue{
				{
					Key:   "test.key",
					Value: &AnyValue{Value: &AnyValue_StringValue{StringValue: "test.value"}},
				},
			},
		},
		{
			name: "normal attributes",
			input: []*v2.KeyValue{
				{
					Key:   "key1",
					Value: &v2.AnyValue{Value: &v2.AnyValue_StringValue{StringValue: "value1"}},
				},
				{
					Key:   "key2",
					Value: &v2.AnyValue{Value: &v2.AnyValue_IntValue{IntValue: 123}},
				},
			},
			expected: []*KeyValue{
				{
					Key:   "key1",
					Value: &AnyValue{Value: &AnyValue_StringValue{StringValue: "value1"}},
				},
				{
					Key:   "key2",
					Value: &AnyValue{Value: &AnyValue_IntValue{IntValue: 123}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := otelAttributeListPbToJson(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
