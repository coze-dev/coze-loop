// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/bytedance/sonic"
	v1 "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// ExportTraceServiceRequest Internal struct, compared to PB struct: TraceID & SpanID & ParentSpanId is string, int64/uint64 -> string, can support otel json source data
type ExportTraceServiceRequest struct {
	ResourceSpans []*ResourceSpans `json:"resourceSpans,omitempty"`
}

type ResourceSpans struct {
	Resource   *Resource     `json:"resource,omitempty"`
	ScopeSpans []*ScopeSpans `json:"scopeSpans,omitempty"`
	SchemaUrl  string        `json:"schemaUrl,omitempty"`
}

type Resource struct {
	Attributes             []*KeyValue  `json:"attributes,omitempty"`
	DroppedAttributesCount uint32       `json:"droppedAttributesCount,omitempty"`
	EntityRefs             []*EntityRef `json:"entityRefs,omitempty"`
}

type EntityRef struct {
	SchemaUrl       string   `json:"schemaUrl,omitempty"`
	Type            string   `json:"type,omitempty"`
	IdKeys          []string `json:"idKeys,omitempty"`
	DescriptionKeys []string `json:"descriptionKeys,omitempty"`
}

type ScopeSpans struct {
	Scope     *InstrumentationScope `json:"scope,omitempty"`
	Spans     []*Span               `json:"spans,omitempty"`
	SchemaUrl string                `json:"schemaUrl,omitempty"`
}

type InstrumentationScope struct {
	Name                   string      `json:"name,omitempty"`
	Version                string      `json:"version,omitempty"`
	Attributes             []*KeyValue `json:"attributes,omitempty"`
	DroppedAttributesCount uint32      `json:"droppedAttributesCount,omitempty"`
}

type Span struct {
	TraceId                string           `json:"traceId,omitempty"`
	SpanId                 string           `json:"spanId,omitempty"`
	TraceState             string           `json:"traceState,omitempty"`
	ParentSpanId           string           `json:"parentSpanId,omitempty"`
	Flags                  uint32           `json:"flags,omitempty"`
	Name                   string           `json:"name,omitempty"`
	Kind                   v1.Span_SpanKind `json:"kind,omitempty"`
	StartTimeUnixNano      string           `json:"startTimeUnixNano,omitempty"`
	EndTimeUnixNano        string           `json:"endTimeUnixNano,omitempty"`
	Attributes             []*KeyValue      `json:"attributes,omitempty"`
	DroppedAttributesCount uint32           `json:"droppedAttributesCount,omitempty"`
	Events                 []*SpanEvent     `json:"events,omitempty"`
	DroppedEventsCount     uint32           `json:"droppedEventsCount,omitempty"`
	Links                  []*SpanLink      `json:"links,omitempty"`
	DroppedLinksCount      uint32           `json:"droppedLinksCount,omitempty"`
	Status                 *v1.Status       `json:"status,omitempty"`
}

type SpanLink struct {
	TraceId                string      `json:"traceId,omitempty"`
	SpanId                 string      `json:"spanId,omitempty"`
	TraceState             string      `json:"traceState,omitempty"`
	Attributes             []*KeyValue `json:"attributes,omitempty"`
	DroppedAttributesCount uint32      `json:"droppedAttributesCount,omitempty"`
	Flags                  uint32      `json:"flags,omitempty"`
}

type SpanEvent struct {
	TimeUnixNano           string      `json:"timeUnixNano,omitempty"`
	Name                   string      `json:"name,omitempty"`
	Attributes             []*KeyValue `json:"attributes,omitempty"`
	DroppedAttributesCount uint32      `json:"droppedAttributesCount,omitempty"`
}

type KeyValue struct {
	Key   string    `json:"key,omitempty"`
	Value *AnyValue `json:"value,omitempty"`
}

type AnyValue struct {
	Value isAnyValue_Value
}

func (anyV *AnyValue) GetStringValue() string {
	if x, ok := anyV.Value.(*AnyValue_StringValue); ok {
		return x.StringValue
	}
	return ""
}

func (anyV *AnyValue) IsStringValue() bool {
	if _, ok := anyV.Value.(*AnyValue_StringValue); ok {
		return true
	}
	return false
}

func (anyV *AnyValue) GetBoolValue() bool {
	if x, ok := anyV.Value.(*AnyValue_BoolValue); ok {
		return x.BoolValue
	}
	return false
}

func (anyV *AnyValue) IsBoolValue() bool {
	if _, ok := anyV.Value.(*AnyValue_BoolValue); ok {
		return true
	}
	return false
}

func (anyV *AnyValue) GetIntValue() int64 {
	if x, ok := anyV.Value.(*AnyValue_IntValue); ok {
		return x.IntValue
	}
	return 0
}

func (anyV *AnyValue) IsIntValue() bool {
	if _, ok := anyV.Value.(*AnyValue_IntValue); ok {
		return true
	}

	return false
}

func (anyV *AnyValue) GetDoubleValue() float64 {
	if x, ok := anyV.Value.(*AnyValue_DoubleValue); ok {
		return x.DoubleValue
	}
	return 0
}

func (anyV *AnyValue) IsDoubleValue() bool {
	if _, ok := anyV.Value.(*AnyValue_DoubleValue); ok {
		return true
	}
	return false
}

func (anyV *AnyValue) GetArrayValue() *ArrayValue {
	if x, ok := anyV.Value.(*AnyValue_ArrayValue); ok {
		return x.ArrayValue
	}
	return nil
}

func (anyV *AnyValue) IsArrayValue() bool {
	if _, ok := anyV.Value.(*AnyValue_ArrayValue); ok {
		return true
	}
	return false
}

func (anyV *AnyValue) GetKvlistValue() *KeyValueList {
	if x, ok := anyV.Value.(*AnyValue_KvlistValue); ok {
		return x.KvlistValue
	}
	return nil
}

func (anyV *AnyValue) IsKvlistValue() bool {
	if _, ok := anyV.Value.(*AnyValue_KvlistValue); ok {
		return true
	}
	return false
}

func (anyV *AnyValue) GetBytesValue() []byte {
	if x, ok := anyV.Value.(*AnyValue_BytesValue); ok {
		return x.BytesValue
	}
	return nil
}

func (anyV *AnyValue) IsBytesValue() bool {
	if _, ok := anyV.Value.(*AnyValue_BytesValue); ok {
		return true
	}
	return false
}

type isAnyValue_Value interface {
	isAnyValue_Value()
}

type AnyValue_StringValue struct {
	StringValue string `json:"stringValue,omitempty"`
}

type AnyValue_BoolValue struct {
	BoolValue bool `json:"boolValue,omitempty"`
}

type AnyValue_IntValue struct {
	IntValue int64 `json:"intValue,omitempty"`
}

type AnyValue_DoubleValue struct {
	DoubleValue float64 `json:"doubleValue,omitempty"`
}

type AnyValue_ArrayValue struct {
	ArrayValue *ArrayValue `json:"arrayValue,omitempty"`
}

type AnyValue_KvlistValue struct {
	KvlistValue *KeyValueList `json:"kvlistValue,omitempty"`
}

type AnyValue_BytesValue struct {
	BytesValue []byte `json:"bytesValue,omitempty"`
}

type KeyValueList struct {
	Values []*KeyValue `json:"values,omitempty"`
}

func (x *KeyValueList) String(ctx context.Context) string {
	marshalString, err := sonic.MarshalString(x)
	if err != nil {
		logs.CtxError(ctx, "KeyValueList marshal failed err=%+v", err)
		return ""
	}
	return marshalString
}

type ArrayValue struct {
	Values []*AnyValue `json:"values,omitempty"`
}

func (x *ArrayValue) String(ctx context.Context) string {
	marshalString, err := sonic.MarshalString(x)
	if err != nil {
		logs.CtxError(ctx, "ArrayValue marshal failed err=%+v", err)
		return ""
	}
	return marshalString
}

func (anyV *AnyValue) UnmarshalJSON(data []byte) error {
	var rawMap map[string]json.RawMessage
	if err := sonic.Unmarshal(data, &rawMap); err != nil {
		return err
	}

	switch {
	case len(rawMap["stringValue"]) > 0:
		var v AnyValue_StringValue
		if err := sonic.Unmarshal(rawMap["stringValue"], &v.StringValue); err != nil {
			return err
		}
		anyV.Value = &v
	case len(rawMap["boolValue"]) > 0:
		var v AnyValue_BoolValue
		if err := sonic.Unmarshal(rawMap["boolValue"], &v.BoolValue); err != nil {
			return err
		}
		anyV.Value = &v
	case len(rawMap["intValue"]) > 0:
		var v AnyValue_IntValue
		if err := sonic.Unmarshal(rawMap["intValue"], &v.IntValue); err != nil {
			return err
		}
		anyV.Value = &v
	case len(rawMap["doubleValue"]) > 0:
		var v AnyValue_DoubleValue
		if err := sonic.Unmarshal(rawMap["doubleValue"], &v.DoubleValue); err != nil {
			return err
		}
		anyV.Value = &v
	case len(rawMap["arrayValue"]) > 0:
		var v AnyValue_ArrayValue
		if err := sonic.Unmarshal(rawMap["arrayValue"], &v.ArrayValue); err != nil {
			return err
		}
		anyV.Value = &v
	case len(rawMap["kvlistValue"]) > 0:
		var v AnyValue_KvlistValue
		if err := sonic.Unmarshal(rawMap["kvlistValue"], &v.KvlistValue); err != nil {
			return err
		}
		anyV.Value = &v
	case len(rawMap["bytesValue"]) > 0:
		var v AnyValue_BytesValue
		if err := sonic.Unmarshal(rawMap["bytesValue"], &v.BytesValue); err != nil {
			return err
		}
		anyV.Value = &v
	default:
		anyV.Value = nil
	}

	return nil
}

func (*AnyValue_StringValue) isAnyValue_Value() {}

func (*AnyValue_BoolValue) isAnyValue_Value() {}

func (*AnyValue_IntValue) isAnyValue_Value() {}

func (*AnyValue_DoubleValue) isAnyValue_Value() {}

func (*AnyValue_ArrayValue) isAnyValue_Value() {}

func (*AnyValue_KvlistValue) isAnyValue_Value() {}

func (*AnyValue_BytesValue) isAnyValue_Value() {}

func (anyV *AnyValue) GetCorrectTypeValue() interface{} {
	if anyV == nil {
		return nil
	}
	if anyV.IsStringValue() {
		return anyV.GetStringValue()
	} else if anyV.IsIntValue() {
		return anyV.GetIntValue()
	} else if anyV.IsDoubleValue() {
		return anyV.GetDoubleValue()
	} else if anyV.IsBoolValue() {
		return anyV.GetBoolValue()
	} else if anyV.IsArrayValue() {
		arrayRes := make([]interface{}, 0)
		for _, anyValue := range anyV.GetArrayValue().Values {
			arrayRes = append(arrayRes, anyValue.GetCorrectTypeValue())
		}
		return arrayRes
	} else if anyV.IsBytesValue() {
		return string(anyV.GetBytesValue())
	} else if anyV.IsKvlistValue() {
		arrayRes := make([]interface{}, 0)
		for _, keyValue := range anyV.GetKvlistValue().Values {
			arrayRes = append(arrayRes, map[string]interface{}{
				keyValue.Key: keyValue.Value.GetCorrectTypeValue(),
			})
		}
		return arrayRes
	}

	return nil
}

func (anyV *AnyValue) TryGetFloat64Value() float64 {
	dv := anyV.GetDoubleValue()
	if dv != 0 {
		return dv
	}

	iv := anyV.GetIntValue()
	if iv != 0 {
		return float64(iv)
	}

	sv := anyV.GetStringValue()
	if sv != "" {
		tempF, err := strconv.ParseFloat(sv, 64)
		if err != nil {
			return 0
		}
		return tempF
	}

	return 0
}

func (anyV *AnyValue) TryGetInt64Value() int64 {
	dv := anyV.GetDoubleValue()
	if dv != 0 {
		return int64(dv)
	}

	iv := anyV.GetIntValue()
	if iv != 0 {
		return iv
	}

	sv := anyV.GetStringValue()
	if sv != "" {
		tempI, err := strconv.ParseInt(sv, 10, 64)
		if err != nil {
			return 0
		}
		return tempI
	}

	return 0
}

func (anyV *AnyValue) TryGetBoolValue() bool {
	bv := anyV.GetBoolValue()
	if bv {
		return true
	}
	sv := anyV.GetStringValue()
	return sv == "true"
}
