package otel

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"
	"github.com/stretchr/testify/assert"
	semconv1_32_0 "go.opentelemetry.io/otel/semconv/v1.32.0"
)

func TestOtelSpansConvertToSendSpans(t *testing.T) {
	ctx := context.Background()
	spaceID := "test-space-id"

	tests := []struct {
		name     string
		spans    []*ResourceScopeSpan
		expected int
	}{
		{
			name:     "empty spans",
			spans:    []*ResourceScopeSpan{},
			expected: 0,
		},
		{
			name:     "nil spans",
			spans:    nil,
			expected: 0,
		},
		{
			name: "single valid span",
			spans: []*ResourceScopeSpan{
				createTestResourceScopeSpan(
					map[string]interface{}{"service.name": "test-service"},
					"test-scope", "1.0.0",
					"test-span", "0102030405060708090a0b0c0d0e0f10", "0102030405060708", "0807060504030201",
					map[string]interface{}{"test.key": "test-value"},
					nil,
				),
			},
			expected: 1,
		},
		{
			name: "multiple spans with nil",
			spans: []*ResourceScopeSpan{
				nil,
				createTestResourceScopeSpan(
					map[string]interface{}{"service.name": "test-service"},
					"test-scope", "1.0.0",
					"test-span", "0102030405060708090a0b0c0d0e0f10", "0102030405060708", "0807060504030201",
					map[string]interface{}{"test.key": "test-value"},
					nil,
				),
				nil,
			},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := OtelSpansConvertToSendSpans(ctx, spaceID, tt.spans)
			assert.Len(t, result, tt.expected)
		})
	}
}

func TestOtelSpanConvertToSendSpan(t *testing.T) {
	ctx := context.Background()
	spaceID := "test-space-id"

	tests := []struct {
		name     string
		input    *ResourceScopeSpan
		expected func(t *testing.T, result *loop_span.Span)
	}{
		{
			name:  "nil input",
			input: nil,
			expected: func(t *testing.T, result *loop_span.Span) {
				assert.Nil(t, result)
			},
		},
		{
			name: "nil span in ResourceScopeSpan",
			input: &ResourceScopeSpan{
				Resource: &Resource{},
				Scope:    &InstrumentationScope{},
				Span:     nil,
			},
			expected: func(t *testing.T, result *loop_span.Span) {
				assert.Nil(t, result)
			},
		},
		{
			name: "basic span conversion",
			input: createTestResourceScopeSpan(
				map[string]interface{}{
					string(semconv1_32_0.TelemetrySDKLanguageKey): "go",
					string(semconv1_32_0.TelemetrySDKVersionKey):  "1.0.0",
				},
				"test-scope", "1.0.0",
				"test-span", "0102030405060708090a0b0c0d0e0f10", "0102030405060708", "0807060504030201",
				map[string]interface{}{
					"test.key": "test-value",
				},
				nil,
			),
			expected: func(t *testing.T, result *loop_span.Span) {
				assert.NotNil(t, result)
				assert.Equal(t, spaceID, result.WorkspaceID)
				assert.Equal(t, "test-span", result.SpanName)
				assert.Equal(t, "0102030405060708090a0b0c0d0e0f10", result.TraceID)
				assert.Equal(t, "0102030405060708", result.SpanID)
				assert.Equal(t, "0807060504030201", result.ParentID)
				assert.Equal(t, "Custom", result.CallType)
				assert.Equal(t, int64(1000000), result.DurationMicros) // 1 second in microseconds
				assert.Contains(t, result.TagsString, "test.key")
				assert.Equal(t, "test-value", result.TagsString["test.key"])
			},
		},
		{
			name: "span with model attributes",
			input: createTestResourceScopeSpan(
				map[string]interface{}{},
				"test-scope", "1.0.0",
				"model-span", "0102030405060708090a0b0c0d0e0f10", "0102030405060708", "0807060504030201",
				map[string]interface{}{
					otelAttributeSpanType:                           "chat",
					string(semconv1_32_0.GenAISystemKey):            "openai",
					string(semconv1_32_0.GenAIRequestModelKey):      "gpt-4",
					otelAttributeInput:                              "test input",
					otelAttributeOutput:                             "test output",
					string(semconv1_32_0.GenAIUsageInputTokensKey):  int64(100),
					string(semconv1_32_0.GenAIUsageOutputTokensKey): int64(50),
					otelAttributeModelStream:                        true,
				},
				nil,
			),
			expected: func(t *testing.T, result *loop_span.Span) {
				assert.NotNil(t, result)
				assert.Equal(t, tracespec.VModelSpanType, result.SpanType)
				assert.Equal(t, "test input", result.Input)
				assert.Equal(t, "test output", result.Output)
				assert.Equal(t, "openai", result.TagsString[tracespec.ModelProvider])
				assert.Equal(t, "gpt-4", result.TagsString[tracespec.ModelName])
				assert.Equal(t, int64(100), result.TagsLong[tracespec.InputTokens])
				assert.Equal(t, int64(50), result.TagsLong[tracespec.OutputTokens])
				assert.Equal(t, int64(150), result.TagsLong[tracespec.Tokens]) // total tokens
				assert.Equal(t, true, result.TagsBool[tracespec.Stream])
			},
		},
		{
			name: "span with events",
			input: createTestResourceScopeSpan(
				map[string]interface{}{},
				"test-scope", "1.0.0",
				"event-span", "0102030405060708090a0b0c0d0e0f10", "0102030405060708", "0807060504030201",
				map[string]interface{}{},
				[]*SpanEvent{
					createTestSpanEvent(otelEventModelSystemMessage, 1640995200500000000, map[string]interface{}{
						"content": "You are a helpful assistant",
					}),
				},
			),
			expected: func(t *testing.T, result *loop_span.Span) {
				assert.NotNil(t, result)
				// Input should be set from events
				assert.NotEmpty(t, result.Input)
			},
		},
		{
			name: "span with error",
			input: createTestResourceScopeSpan(
				map[string]interface{}{},
				"test-scope", "1.0.0",
				"error-span", "0102030405060708090a0b0c0d0e0f10", "0102030405060708", "0807060504030201",
				map[string]interface{}{
					string(semconv1_32_0.ErrorTypeKey): "timeout_error",
				},
				nil,
			),
			expected: func(t *testing.T, result *loop_span.Span) {
				assert.NotNil(t, result)
				assert.Equal(t, int32(-1), result.StatusCode)
				assert.Contains(t, result.TagsString, tracespec.Error)
			},
		},
		{
			name: "span with call options",
			input: createTestResourceScopeSpan(
				map[string]interface{}{},
				"test-scope", "1.0.0",
				"options-span", "0102030405060708090a0b0c0d0e0f10", "0102030405060708", "0807060504030201",
				map[string]interface{}{
					string(semconv1_32_0.GenAIRequestTemperatureKey):      0.7,
					string(semconv1_32_0.GenAIRequestTopPKey):             0.9,
					string(semconv1_32_0.GenAIRequestMaxTokensKey):        int64(1000),
					string(semconv1_32_0.GenAIRequestFrequencyPenaltyKey): 0.1,
					string(semconv1_32_0.GenAIRequestPresencePenaltyKey):  0.2,
					string(semconv1_32_0.GenAIRequestStopSequencesKey):    []string{"stop1", "stop2"},
					string(semconv1_32_0.GenAIRequestTopKKey):             int64(40),
				},
				nil,
			),
			expected: func(t *testing.T, result *loop_span.Span) {
				assert.NotNil(t, result)
				assert.Contains(t, result.TagsString, tracespec.CallOptions)
				// Options should be removed from individual tags
				assert.NotContains(t, result.TagsDouble, "temperature")
				assert.NotContains(t, result.TagsLong, "max_tokens")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := OtelSpanConvertToSendSpan(ctx, spaceID, tt.input)
			tt.expected(t, result)
		})
	}
}

func TestProcessAttributesAndEvents(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		attributeMap map[string]*AnyValue
		events       []*SpanEvent
		expected     func(t *testing.T, result map[string]interface{})
	}{
		{
			name:         "empty input",
			attributeMap: map[string]*AnyValue{},
			events:       []*SpanEvent{},
			expected: func(t *testing.T, result map[string]interface{}) {
				assert.NotNil(t, result)
			},
		},
		{
			name: "attribute processing",
			attributeMap: map[string]*AnyValue{
				string(semconv1_32_0.GenAISystemKey):       createTestAnyValue("openai"),
				string(semconv1_32_0.GenAIRequestModelKey): createTestAnyValue("gpt-4"),
				otelAttributeInput:                         createTestAnyValue("test input"),
			},
			events: []*SpanEvent{},
			expected: func(t *testing.T, result map[string]interface{}) {
				assert.Equal(t, "openai", result[tracespec.ModelProvider])
				assert.Equal(t, "gpt-4", result[tracespec.ModelName])
				assert.Equal(t, "test input", result[tracespec.Input])
			},
		},
		{
			name:         "event processing",
			attributeMap: map[string]*AnyValue{},
			events: []*SpanEvent{
				createTestSpanEvent(otelEventModelSystemMessage, 1640995200500000000, map[string]interface{}{
					"content": "You are a helpful assistant",
				}),
			},
			expected: func(t *testing.T, result map[string]interface{}) {
				assert.Contains(t, result, tracespec.Input)
				assert.NotEmpty(t, result[tracespec.Input])
			},
		},
		{
			name: "attribute prefix processing",
			attributeMap: map[string]*AnyValue{
				"gen_ai.prompt.0.content": createTestAnyValue("Hello"),
				"gen_ai.prompt.0.role":    createTestAnyValue("user"),
				"gen_ai.prompt.1.content": createTestAnyValue("Hi there!"),
				"gen_ai.prompt.1.role":    createTestAnyValue("assistant"),
			},
			events: []*SpanEvent{},
			expected: func(t *testing.T, result map[string]interface{}) {
				assert.Contains(t, result, tracespec.Input)
				assert.NotEmpty(t, result[tracespec.Input])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processAttributesAndEvents(ctx, tt.attributeMap, tt.events)
			tt.expected(t, result)
		})
	}
}

func TestAggregateAttributes(t *testing.T) {
	tests := []struct {
		name     string
		srcInput map[string]interface{}
		prefix   string
		expected interface{}
	}{
		{
			name:     "no prefix - simple map",
			srcInput: map[string]interface{}{"key1": "value1", "key2": "value2"},
			prefix:   "",
			expected: map[string]interface{}{"key1": "value1", "key2": "value2"},
		},
		{
			name:     "with prefix - direct match",
			srcInput: map[string]interface{}{"gen_ai.prompt": "direct value"},
			prefix:   "gen_ai.prompt",
			expected: "direct value",
		},
		{
			name: "with prefix - nested attributes",
			srcInput: map[string]interface{}{
				"gen_ai.prompt.0.content": "Hello",
				"gen_ai.prompt.0.role":    "user",
				"gen_ai.prompt.1.content": "Hi there!",
				"gen_ai.prompt.1.role":    "assistant",
			},
			prefix: "gen_ai.prompt",
			expected: []interface{}{
				map[string]interface{}{"content": "Hello", "role": "user"},
				map[string]interface{}{"content": "Hi there!", "role": "assistant"},
			},
		},
		{
			name: "nested object structure",
			srcInput: map[string]interface{}{
				"user.profile.name":   "John",
				"user.profile.email":  "john@example.com",
				"user.settings.theme": "dark",
			},
			prefix: "",
			expected: map[string]interface{}{
				"user": map[string]interface{}{
					"profile": map[string]interface{}{
						"name":  "John",
						"email": "john@example.com",
					},
					"settings": map[string]interface{}{
						"theme": "dark",
					},
				},
			},
		},
		{
			name: "array structure with mixed keys",
			srcInput: map[string]interface{}{
				"items.0.name":  "item1",
				"items.0.value": 100,
				"items.1.name":  "item2",
				"items.1.value": 200,
				"total":         300,
			},
			prefix: "",
			expected: map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{"name": "item1", "value": 100},
					map[string]interface{}{"name": "item2", "value": 200},
				},
				"total": 300,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := aggregateAttributes(tt.srcInput, tt.prefix)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSpanTypeMapping(t *testing.T) {
	tests := []struct {
		name     string
		spanType string
		expected string
	}{
		{
			name:     "chat span type",
			spanType: "chat",
			expected: tracespec.VModelSpanType,
		},
		{
			name:     "execute_tool span type",
			spanType: "execute_tool",
			expected: tracespec.VToolSpanType,
		},
		{
			name:     "generate_content span type",
			spanType: "generate_content",
			expected: tracespec.VModelSpanType,
		},
		{
			name:     "text_completion span type",
			spanType: "text_completion",
			expected: tracespec.VModelSpanType,
		},
		{
			name:     "empty span type",
			spanType: "",
			expected: "custom",
		},
		{
			name:     "unknown span type",
			spanType: "unknown",
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := spanTypeMapping(tt.spanType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalLatencyFirstResp(t *testing.T) {
	tests := []struct {
		name                   string
		tagsLong               map[string]int64
		startTimeUnixNanoInt64 int64
		expectedLatency        int64
		expectLatencyKey       bool
	}{
		{
			name: "with start time first resp",
			tagsLong: map[string]int64{
				tagKeyStartTimeFirstResp: 1640995200500000,
			},
			startTimeUnixNanoInt64: 1640995200000000000,
			expectedLatency:        500000, // (1640995200500000 - 1640995200000000000/1000)
			expectLatencyKey:       true,
		},
		{
			name:                   "without start time first resp",
			tagsLong:               map[string]int64{},
			startTimeUnixNanoInt64: 1640995200000000000,
			expectedLatency:        0,
			expectLatencyKey:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calLatencyFirstResp(tt.tagsLong, tt.startTimeUnixNanoInt64)
			if tt.expectLatencyKey {
				assert.Contains(t, tt.tagsLong, tracespec.LatencyFirstResp)
				assert.Equal(t, tt.expectedLatency, tt.tagsLong[tracespec.LatencyFirstResp])
			} else {
				assert.NotContains(t, tt.tagsLong, tracespec.LatencyFirstResp)
			}
		})
	}
}

func TestCalTokens(t *testing.T) {
	tests := []struct {
		name            string
		tagsLong        map[string]int64
		expectedTotal   int64
		expectTokensKey bool
	}{
		{
			name: "with input and output tokens",
			tagsLong: map[string]int64{
				tracespec.InputTokens:  100,
				tracespec.OutputTokens: 50,
			},
			expectedTotal:   150,
			expectTokensKey: true,
		},
		{
			name: "with only input tokens",
			tagsLong: map[string]int64{
				tracespec.InputTokens: 100,
			},
			expectedTotal:   100,
			expectTokensKey: true,
		},
		{
			name: "with only output tokens",
			tagsLong: map[string]int64{
				tracespec.OutputTokens: 50,
			},
			expectedTotal:   50,
			expectTokensKey: true,
		},
		{
			name:            "without tokens",
			tagsLong:        map[string]int64{},
			expectedTotal:   0,
			expectTokensKey: false,
		},
		{
			name: "with zero tokens",
			tagsLong: map[string]int64{
				tracespec.InputTokens:  0,
				tracespec.OutputTokens: 0,
			},
			expectedTotal:   0,
			expectTokensKey: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calTokens(tt.tagsLong)
			if tt.expectTokensKey {
				assert.Contains(t, tt.tagsLong, tracespec.Tokens)
				assert.Equal(t, tt.expectedTotal, tt.tagsLong[tracespec.Tokens])
			} else {
				assert.NotContains(t, tt.tagsLong, tracespec.Tokens)
			}
		})
	}
}

func TestCalStatusCode(t *testing.T) {
	tests := []struct {
		name         string
		tagsString   map[string]string
		statusCode   int32
		expectedCode int32
	}{
		{
			name:         "with error and zero status code",
			tagsString:   map[string]string{tracespec.Error: "timeout"},
			statusCode:   0,
			expectedCode: -1,
		},
		{
			name:         "with error and non-zero status code",
			tagsString:   map[string]string{tracespec.Error: "timeout"},
			statusCode:   500,
			expectedCode: 500,
		},
		{
			name:         "without error",
			tagsString:   map[string]string{},
			statusCode:   0,
			expectedCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calStatusCode(tt.tagsString, tt.statusCode)
			assert.Equal(t, tt.expectedCode, result)
		})
	}
}

func TestGetValueByDataType(t *testing.T) {
	tests := []struct {
		name     string
		src      *AnyValue
		dataType string
		expected interface{}
	}{
		{
			name:     "nil source",
			src:      nil,
			dataType: dataTypeString,
			expected: nil,
		},
		{
			name:     "string type",
			src:      createTestAnyValue("test-string"),
			dataType: dataTypeString,
			expected: "test-string",
		},
		{
			name:     "int64 type",
			src:      createTestAnyValueInt(int64(123)),
			dataType: dataTypeInt64,
			expected: int64(123),
		},
		{
			name:     "bool type",
			src:      createTestAnyValueBool(true),
			dataType: dataTypeBool,
			expected: true,
		},
		{
			name:     "float64 type",
			src:      createTestAnyValueFloat(123.45),
			dataType: dataTypeFloat64,
			expected: 123.45,
		},
		{
			name:     "array string type",
			src:      createTestAnyValueArray(createTestArrayValue("item1", "item2")),
			dataType: dataTypeArrayString,
			expected: []string{"item1", "item2"},
		},
		{
			name:     "array string type with nil array",
			src:      &AnyValue{Value: &AnyValue_ArrayValue{ArrayValue: nil}},
			dataType: dataTypeArrayString,
			expected: nil,
		},
		{
			name:     "default type",
			src:      createTestAnyValue("default-value"),
			dataType: dataTypeDefault,
			expected: "default-value",
		},
		{
			name:     "unknown type",
			src:      createTestAnyValue("unknown-value"),
			dataType: "unknown",
			expected: "unknown-value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getValueByDataType(tt.src, tt.dataType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIterSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		fn       func(int) string
		expected []string
	}{
		{
			name:     "empty slice",
			input:    []int{},
			fn:       func(i int) string { return string(rune(i + 48)) },
			expected: []string{},
		},
		{
			name:     "normal slice",
			input:    []int{1, 2, 3},
			fn:       func(i int) string { return string(rune(i + 48)) },
			expected: []string{"1", "2", "3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := iterSlice(tt.input, tt.fn)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// createTestResourceScopeSpan creates a test ResourceScopeSpan with the given parameters
func createTestResourceScopeSpan(
	resourceAttributes map[string]interface{},
	scopeName, scopeVersion string,
	spanName, traceID, spanID, parentSpanID string,
	spanAttributes map[string]interface{},
	events []*SpanEvent,
) *ResourceScopeSpan {
	// Create resource attributes
	var resourceAttrs []*KeyValue
	for key, value := range resourceAttributes {
		resourceAttrs = append(resourceAttrs, &KeyValue{
			Key:   key,
			Value: createTestAnyValueFromInterface(value),
		})
	}

	// Create span attributes
	var spanAttrs []*KeyValue
	for key, value := range spanAttributes {
		spanAttrs = append(spanAttrs, &KeyValue{
			Key:   key,
			Value: createTestAnyValueFromInterface(value),
		})
	}

	// Create test timestamps (1 second duration)
	startTime := time.Unix(1640995200, 0).UnixNano() // 2022-01-01 00:00:00 UTC
	endTime := startTime + int64(time.Second)

	return &ResourceScopeSpan{
		Resource: &Resource{
			Attributes: resourceAttrs,
		},
		Scope: &InstrumentationScope{
			Name:    scopeName,
			Version: scopeVersion,
		},
		Span: &Span{
			TraceId:           traceID,
			SpanId:            spanID,
			ParentSpanId:      parentSpanID,
			Name:              spanName,
			StartTimeUnixNano: strconv.FormatInt(startTime, 10),
			EndTimeUnixNano:   strconv.FormatInt(endTime, 10),
			Attributes:        spanAttrs,
			Events:            events,
		},
	}
}

// createTestSpanEvent creates a test SpanEvent with the given parameters
func createTestSpanEvent(name string, timeUnixNano int64, attributes map[string]interface{}) *SpanEvent {
	var attrs []*KeyValue
	for key, value := range attributes {
		attrs = append(attrs, &KeyValue{
			Key:   key,
			Value: createTestAnyValueFromInterface(value),
		})
	}

	return &SpanEvent{
		Name:         name,
		TimeUnixNano: strconv.FormatInt(timeUnixNano, 10),
		Attributes:   attrs,
	}
}

// createTestAnyValue creates a test AnyValue with a string value
func createTestAnyValue(value string) *AnyValue {
	return &AnyValue{
		Value: &AnyValue_StringValue{
			StringValue: value,
		},
	}
}

// createTestAnyValueFromInterface creates a test AnyValue from any interface value
func createTestAnyValueFromInterface(value interface{}) *AnyValue {
	switch v := value.(type) {
	case string:
		return &AnyValue{
			Value: &AnyValue_StringValue{
				StringValue: v,
			},
		}
	case int64:
		return &AnyValue{
			Value: &AnyValue_IntValue{
				IntValue: v,
			},
		}
	case int:
		return &AnyValue{
			Value: &AnyValue_IntValue{
				IntValue: int64(v),
			},
		}
	case float64:
		return &AnyValue{
			Value: &AnyValue_DoubleValue{
				DoubleValue: v,
			},
		}
	case bool:
		return &AnyValue{
			Value: &AnyValue_BoolValue{
				BoolValue: v,
			},
		}
	case []string:
		var values []*AnyValue
		for _, item := range v {
			values = append(values, &AnyValue{
				Value: &AnyValue_StringValue{
					StringValue: item,
				},
			})
		}
		return &AnyValue{
			Value: &AnyValue_ArrayValue{
				ArrayValue: &ArrayValue{
					Values: values,
				},
			},
		}
	case *ArrayValue:
		return &AnyValue{
			Value: &AnyValue_ArrayValue{
				ArrayValue: v,
			},
		}
	default:
		// Default to string representation
		return &AnyValue{
			Value: &AnyValue_StringValue{
				StringValue: "",
			},
		}
	}
}

// createTestArrayValue creates a test ArrayValue with string values
func createTestArrayValue(values ...string) *ArrayValue {
	var anyValues []*AnyValue
	for _, value := range values {
		anyValues = append(anyValues, &AnyValue{
			Value: &AnyValue_StringValue{
				StringValue: value,
			},
		})
	}
	return &ArrayValue{
		Values: anyValues,
	}
}

// Additional helper functions for specific test cases

// createTestAnyValueInt creates a test AnyValue with an int64 value
func createTestAnyValueInt(value int64) *AnyValue {
	return &AnyValue{
		Value: &AnyValue_IntValue{
			IntValue: value,
		},
	}
}

// createTestAnyValueBool creates a test AnyValue with a bool value
func createTestAnyValueBool(value bool) *AnyValue {
	return &AnyValue{
		Value: &AnyValue_BoolValue{
			BoolValue: value,
		},
	}
}

// createTestAnyValueFloat creates a test AnyValue with a float64 value
func createTestAnyValueFloat(value float64) *AnyValue {
	return &AnyValue{
		Value: &AnyValue_DoubleValue{
			DoubleValue: value,
		},
	}
}

// createTestAnyValueArray creates a test AnyValue with an ArrayValue
func createTestAnyValueArray(arrayValue *ArrayValue) *AnyValue {
	return &AnyValue{
		Value: &AnyValue_ArrayValue{
			ArrayValue: arrayValue,
		},
	}
}
