// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnyValue_GetMethods(t *testing.T) {
	tests := []struct {
		name     string
		anyValue *AnyValue
		testFunc func(t *testing.T, av *AnyValue)
	}{
		{
			name:     "string value",
			anyValue: &AnyValue{Value: &AnyValue_StringValue{StringValue: "test-string"}},
			testFunc: func(t *testing.T, av *AnyValue) {
				assert.Equal(t, "test-string", av.GetStringValue())
				assert.True(t, av.IsStringValue())
				assert.False(t, av.IsBoolValue())
				assert.False(t, av.IsIntValue())
				assert.False(t, av.IsDoubleValue())
				assert.False(t, av.IsArrayValue())
				assert.False(t, av.IsKvlistValue())
				assert.False(t, av.IsBytesValue())
			},
		},
		{
			name:     "bool value",
			anyValue: &AnyValue{Value: &AnyValue_BoolValue{BoolValue: true}},
			testFunc: func(t *testing.T, av *AnyValue) {
				assert.True(t, av.GetBoolValue())
				assert.True(t, av.IsBoolValue())
				assert.False(t, av.IsStringValue())
				assert.Equal(t, "", av.GetStringValue())
			},
		},
		{
			name:     "int value",
			anyValue: &AnyValue{Value: &AnyValue_IntValue{IntValue: 123}},
			testFunc: func(t *testing.T, av *AnyValue) {
				assert.Equal(t, int64(123), av.GetIntValue())
				assert.True(t, av.IsIntValue())
				assert.False(t, av.IsStringValue())
			},
		},
		{
			name:     "double value",
			anyValue: &AnyValue{Value: &AnyValue_DoubleValue{DoubleValue: 123.45}},
			testFunc: func(t *testing.T, av *AnyValue) {
				assert.Equal(t, 123.45, av.GetDoubleValue())
				assert.True(t, av.IsDoubleValue())
				assert.False(t, av.IsStringValue())
			},
		},
		{
			name: "array value",
			anyValue: &AnyValue{
				Value: &AnyValue_ArrayValue{
					ArrayValue: &ArrayValue{
						Values: []*AnyValue{
							{Value: &AnyValue_StringValue{StringValue: "item1"}},
						},
					},
				},
			},
			testFunc: func(t *testing.T, av *AnyValue) {
				arrayValue := av.GetArrayValue()
				assert.NotNil(t, arrayValue)
				assert.Len(t, arrayValue.Values, 1)
				assert.True(t, av.IsArrayValue())
			},
		},
		{
			name: "kvlist value",
			anyValue: &AnyValue{
				Value: &AnyValue_KvlistValue{
					KvlistValue: &KeyValueList{
						Values: []*KeyValue{
							{Key: "key1", Value: &AnyValue{Value: &AnyValue_StringValue{StringValue: "value1"}}},
						},
					},
				},
			},
			testFunc: func(t *testing.T, av *AnyValue) {
				kvList := av.GetKvlistValue()
				assert.NotNil(t, kvList)
				assert.Len(t, kvList.Values, 1)
				assert.True(t, av.IsKvlistValue())
			},
		},
		{
			name:     "bytes value",
			anyValue: &AnyValue{Value: &AnyValue_BytesValue{BytesValue: []byte("test-bytes")}},
			testFunc: func(t *testing.T, av *AnyValue) {
				assert.Equal(t, []byte("test-bytes"), av.GetBytesValue())
				assert.True(t, av.IsBytesValue())
			},
		},
		{
			name:     "nil value",
			anyValue: &AnyValue{Value: nil},
			testFunc: func(t *testing.T, av *AnyValue) {
				assert.Equal(t, "", av.GetStringValue())
				assert.False(t, av.GetBoolValue())
				assert.Equal(t, int64(0), av.GetIntValue())
				assert.Equal(t, float64(0), av.GetDoubleValue())
				assert.Nil(t, av.GetArrayValue())
				assert.Nil(t, av.GetKvlistValue())
				assert.Nil(t, av.GetBytesValue())
				assert.False(t, av.IsStringValue())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t, tt.anyValue)
		})
	}
}

func TestAnyValue_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		expected *AnyValue
		wantErr  bool
	}{
		{
			name:     "string value",
			jsonData: `{"stringValue": "test-string"}`,
			expected: &AnyValue{Value: &AnyValue_StringValue{StringValue: "test-string"}},
			wantErr:  false,
		},
		{
			name:     "bool value",
			jsonData: `{"boolValue": true}`,
			expected: &AnyValue{Value: &AnyValue_BoolValue{BoolValue: true}},
			wantErr:  false,
		},
		{
			name:     "int value",
			jsonData: `{"intValue": 123}`,
			expected: &AnyValue{Value: &AnyValue_IntValue{IntValue: 123}},
			wantErr:  false,
		},
		{
			name:     "double value",
			jsonData: `{"doubleValue": 123.45}`,
			expected: &AnyValue{Value: &AnyValue_DoubleValue{DoubleValue: 123.45}},
			wantErr:  false,
		},
		{
			name:     "bytes value",
			jsonData: `{"bytesValue": "dGVzdC1ieXRlcw=="}`,
			expected: &AnyValue{Value: &AnyValue_BytesValue{BytesValue: []byte("test-bytes")}},
			wantErr:  false,
		},
		{
			name:     "array value",
			jsonData: `{"arrayValue": {"values": [{"stringValue": "item1"}]}}`,
			expected: &AnyValue{
				Value: &AnyValue_ArrayValue{
					ArrayValue: &ArrayValue{
						Values: []*AnyValue{
							{Value: &AnyValue_StringValue{StringValue: "item1"}},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:     "kvlist value",
			jsonData: `{"kvlistValue": {"values": [{"key": "key1", "value": {"stringValue": "value1"}}]}}`,
			expected: &AnyValue{
				Value: &AnyValue_KvlistValue{
					KvlistValue: &KeyValueList{
						Values: []*KeyValue{
							{Key: "key1", Value: &AnyValue{Value: &AnyValue_StringValue{StringValue: "value1"}}},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:     "empty object",
			jsonData: `{}`,
			expected: &AnyValue{Value: nil},
			wantErr:  false,
		},
		{
			name:     "invalid json",
			jsonData: `{invalid}`,
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "multiple values - string takes precedence",
			jsonData: `{"stringValue": "test", "intValue": 123}`,
			expected: &AnyValue{Value: &AnyValue_StringValue{StringValue: "test"}},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result AnyValue
			err := result.UnmarshalJSON([]byte(tt.jsonData))

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, &result)
			}
		})
	}
}

func TestAnyValue_GetCorrectTypeValue(t *testing.T) {
	tests := []struct {
		name     string
		anyValue *AnyValue
		expected interface{}
	}{
		{
			name:     "nil anyvalue",
			anyValue: nil,
			expected: nil,
		},
		{
			name:     "string value",
			anyValue: &AnyValue{Value: &AnyValue_StringValue{StringValue: "test-string"}},
			expected: "test-string",
		},
		{
			name:     "int value",
			anyValue: &AnyValue{Value: &AnyValue_IntValue{IntValue: 123}},
			expected: int64(123),
		},
		{
			name:     "double value",
			anyValue: &AnyValue{Value: &AnyValue_DoubleValue{DoubleValue: 123.45}},
			expected: 123.45,
		},
		{
			name:     "bool value",
			anyValue: &AnyValue{Value: &AnyValue_BoolValue{BoolValue: true}},
			expected: true,
		},
		{
			name: "array value",
			anyValue: &AnyValue{
				Value: &AnyValue_ArrayValue{
					ArrayValue: &ArrayValue{
						Values: []*AnyValue{
							{Value: &AnyValue_StringValue{StringValue: "item1"}},
							{Value: &AnyValue_IntValue{IntValue: 42}},
						},
					},
				},
			},
			expected: []interface{}{"item1", int64(42)},
		},
		{
			name:     "bytes value",
			anyValue: &AnyValue{Value: &AnyValue_BytesValue{BytesValue: []byte("test-bytes")}},
			expected: "test-bytes",
		},
		{
			name: "kvlist value",
			anyValue: &AnyValue{
				Value: &AnyValue_KvlistValue{
					KvlistValue: &KeyValueList{
						Values: []*KeyValue{
							{Key: "key1", Value: &AnyValue{Value: &AnyValue_StringValue{StringValue: "value1"}}},
							{Key: "key2", Value: &AnyValue{Value: &AnyValue_IntValue{IntValue: 123}}},
						},
					},
				},
			},
			expected: []interface{}{
				map[string]interface{}{"key1": "value1"},
				map[string]interface{}{"key2": int64(123)},
			},
		},
		{
			name:     "nil value",
			anyValue: &AnyValue{Value: nil},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.anyValue.GetCorrectTypeValue()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAnyValue_TryGetFloat64Value(t *testing.T) {
	tests := []struct {
		name     string
		anyValue *AnyValue
		expected float64
	}{
		{
			name:     "double value",
			anyValue: &AnyValue{Value: &AnyValue_DoubleValue{DoubleValue: 123.45}},
			expected: 123.45,
		},
		{
			name:     "int value",
			anyValue: &AnyValue{Value: &AnyValue_IntValue{IntValue: 123}},
			expected: 123.0,
		},
		{
			name:     "string value - valid float",
			anyValue: &AnyValue{Value: &AnyValue_StringValue{StringValue: "123.45"}},
			expected: 123.45,
		},
		{
			name:     "string value - invalid float",
			anyValue: &AnyValue{Value: &AnyValue_StringValue{StringValue: "invalid"}},
			expected: 0,
		},
		{
			name:     "zero double value",
			anyValue: &AnyValue{Value: &AnyValue_DoubleValue{DoubleValue: 0}},
			expected: 0,
		},
		{
			name:     "zero int value",
			anyValue: &AnyValue{Value: &AnyValue_IntValue{IntValue: 0}},
			expected: 0,
		},
		{
			name:     "empty string",
			anyValue: &AnyValue{Value: &AnyValue_StringValue{StringValue: ""}},
			expected: 0,
		},
		{
			name:     "bool value",
			anyValue: &AnyValue{Value: &AnyValue_BoolValue{BoolValue: true}},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.anyValue.TryGetFloat64Value()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAnyValue_TryGetInt64Value(t *testing.T) {
	tests := []struct {
		name     string
		anyValue *AnyValue
		expected int64
	}{
		{
			name:     "int value",
			anyValue: &AnyValue{Value: &AnyValue_IntValue{IntValue: 123}},
			expected: 123,
		},
		{
			name:     "double value",
			anyValue: &AnyValue{Value: &AnyValue_DoubleValue{DoubleValue: 123.45}},
			expected: 123,
		},
		{
			name:     "string value - valid int",
			anyValue: &AnyValue{Value: &AnyValue_StringValue{StringValue: "123"}},
			expected: 123,
		},
		{
			name:     "string value - invalid int",
			anyValue: &AnyValue{Value: &AnyValue_StringValue{StringValue: "invalid"}},
			expected: 0,
		},
		{
			name:     "zero int value",
			anyValue: &AnyValue{Value: &AnyValue_IntValue{IntValue: 0}},
			expected: 0,
		},
		{
			name:     "zero double value",
			anyValue: &AnyValue{Value: &AnyValue_DoubleValue{DoubleValue: 0}},
			expected: 0,
		},
		{
			name:     "empty string",
			anyValue: &AnyValue{Value: &AnyValue_StringValue{StringValue: ""}},
			expected: 0,
		},
		{
			name:     "bool value",
			anyValue: &AnyValue{Value: &AnyValue_BoolValue{BoolValue: true}},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.anyValue.TryGetInt64Value()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAnyValue_TryGetBoolValue(t *testing.T) {
	tests := []struct {
		name     string
		anyValue *AnyValue
		expected bool
	}{
		{
			name:     "bool value - true",
			anyValue: &AnyValue{Value: &AnyValue_BoolValue{BoolValue: true}},
			expected: true,
		},
		{
			name:     "bool value - false",
			anyValue: &AnyValue{Value: &AnyValue_BoolValue{BoolValue: false}},
			expected: false,
		},
		{
			name:     "string value - true",
			anyValue: &AnyValue{Value: &AnyValue_StringValue{StringValue: "true"}},
			expected: true,
		},
		{
			name:     "string value - false",
			anyValue: &AnyValue{Value: &AnyValue_StringValue{StringValue: "false"}},
			expected: false,
		},
		{
			name:     "string value - other",
			anyValue: &AnyValue{Value: &AnyValue_StringValue{StringValue: "other"}},
			expected: false,
		},
		{
			name:     "int value",
			anyValue: &AnyValue{Value: &AnyValue_IntValue{IntValue: 123}},
			expected: false,
		},
		{
			name:     "empty string",
			anyValue: &AnyValue{Value: &AnyValue_StringValue{StringValue: ""}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.anyValue.TryGetBoolValue()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestKeyValueList_String(t *testing.T) {
	tests := []struct {
		name     string
		kvList   *KeyValueList
		validate func(t *testing.T, result string)
	}{
		{
			name: "normal kvlist",
			kvList: &KeyValueList{
				Values: []*KeyValue{
					{Key: "key1", Value: &AnyValue{Value: &AnyValue_StringValue{StringValue: "value1"}}},
					{Key: "key2", Value: &AnyValue{Value: &AnyValue_IntValue{IntValue: 123}}},
				},
			},
			validate: func(t *testing.T, result string) {
				assert.NotEmpty(t, result)
				assert.Contains(t, result, "key1")
				assert.Contains(t, result, "key2")
				assert.Contains(t, result, "value1")
				assert.Contains(t, result, "123")
			},
		},
		{
			name:   "empty kvlist",
			kvList: &KeyValueList{Values: []*KeyValue{}},
			validate: func(t *testing.T, result string) {
				assert.NotEmpty(t, result)
			},
		},
		{
			name:   "nil values",
			kvList: &KeyValueList{Values: nil},
			validate: func(t *testing.T, result string) {
				assert.NotEmpty(t, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.kvList.String(context.Background())
			tt.validate(t, result)
		})
	}
}

func TestArrayValue_String(t *testing.T) {
	tests := []struct {
		name     string
		arrValue *ArrayValue
		validate func(t *testing.T, result string)
	}{
		{
			name: "normal array",
			arrValue: &ArrayValue{
				Values: []*AnyValue{
					{Value: &AnyValue_StringValue{StringValue: "item1"}},
					{Value: &AnyValue_IntValue{IntValue: 42}},
				},
			},
			validate: func(t *testing.T, result string) {
				assert.NotEmpty(t, result)
				assert.Contains(t, result, "item1")
				assert.Contains(t, result, "42")
			},
		},
		{
			name:     "empty array",
			arrValue: &ArrayValue{Values: []*AnyValue{}},
			validate: func(t *testing.T, result string) {
				assert.NotEmpty(t, result)
			},
		},
		{
			name:     "nil values",
			arrValue: &ArrayValue{Values: nil},
			validate: func(t *testing.T, result string) {
				assert.NotEmpty(t, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.arrValue.String(context.Background())
			tt.validate(t, result)
		})
	}
}

func TestAnyValue_IsInterface(t *testing.T) {
	// Test that all AnyValue_* types implement isAnyValue_Value interface
	var _ isAnyValue_Value = &AnyValue_StringValue{}
	var _ isAnyValue_Value = &AnyValue_BoolValue{}
	var _ isAnyValue_Value = &AnyValue_IntValue{}
	var _ isAnyValue_Value = &AnyValue_DoubleValue{}
	var _ isAnyValue_Value = &AnyValue_ArrayValue{}
	var _ isAnyValue_Value = &AnyValue_KvlistValue{}
	var _ isAnyValue_Value = &AnyValue_BytesValue{}

	// This test just ensures the interface methods exist and compile
	assert.True(t, true)
}
