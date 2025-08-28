// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"context"
	"encoding/hex"
	"strconv"

	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	v3 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	v2 "go.opentelemetry.io/proto/otlp/common/v1"
	v1 "go.opentelemetry.io/proto/otlp/trace/v1"
)

func otelAnyValuePbToJson(src *v2.AnyValue) *AnyValue {
	if src == nil {
		return nil
	}
	innerAnyValue := &AnyValue{}
	switch src.Value.(type) {
	case *v2.AnyValue_StringValue:
		innerAnyValue.Value = &AnyValue_StringValue{StringValue: src.GetStringValue()}
	case *v2.AnyValue_BoolValue:
		innerAnyValue.Value = &AnyValue_BoolValue{BoolValue: src.GetBoolValue()}
	case *v2.AnyValue_IntValue:
		innerAnyValue.Value = &AnyValue_IntValue{IntValue: src.GetIntValue()}
	case *v2.AnyValue_DoubleValue:
		innerAnyValue.Value = &AnyValue_DoubleValue{DoubleValue: src.GetDoubleValue()}
	case *v2.AnyValue_ArrayValue:
		innerAnyValue.Value = &AnyValue_ArrayValue{ArrayValue: otelArrayValuePbToJson(src.GetArrayValue())}
	case *v2.AnyValue_KvlistValue:
		innerAnyValue.Value = &AnyValue_KvlistValue{KvlistValue: otelKeyValueListPbToJson(src.GetKvlistValue())}
	case *v2.AnyValue_BytesValue:
		innerAnyValue.Value = &AnyValue_BytesValue{BytesValue: src.GetBytesValue()}
	default:
		logs.CtxError(context.Background(), "OtelAnyValuePbToJson unknown type: %v", src.Value)
	}
	return innerAnyValue
}

func otelArrayValuePbToJson(src *v2.ArrayValue) *ArrayValue {
	if src == nil {
		return nil
	}
	innerArrayValue := &ArrayValue{}
	innerArrayValue.Values = make([]*AnyValue, 0, len(src.Values))
	for _, value := range src.Values {
		innerArrayValue.Values = append(innerArrayValue.Values, otelAnyValuePbToJson(value))
	}
	return innerArrayValue
}

func otelKeyValueListPbToJson(src *v2.KeyValueList) *KeyValueList {
	if src == nil {
		return nil
	}
	innerKeyValueList := &KeyValueList{}
	innerKeyValueList.Values = make([]*KeyValue, 0, len(src.Values))
	for _, value := range src.Values {
		innerKeyValueList.Values = append(innerKeyValueList.Values, &KeyValue{
			Key:   value.Key,
			Value: otelAnyValuePbToJson(value.Value),
		})
	}
	return innerKeyValueList
}

func otelInstrumentationScopePbToJson(src *v2.InstrumentationScope) *InstrumentationScope {
	if src == nil {
		return nil
	}
	innerInstrumentationScope := &InstrumentationScope{}
	innerInstrumentationScope.Name = src.Name
	innerInstrumentationScope.Version = src.Version
	innerInstrumentationScope.Attributes = make([]*KeyValue, 0, len(src.Attributes))
	for _, attribute := range src.Attributes {
		innerInstrumentationScope.Attributes = append(innerInstrumentationScope.Attributes, &KeyValue{
			Key:   attribute.Key,
			Value: otelAnyValuePbToJson(attribute.Value),
		})
	}
	return innerInstrumentationScope
}

func otelSpanEventsPbToJson(src []*v1.Span_Event) []*SpanEvent {
	if len(src) == 0 {
		return nil
	}
	innerSpanEvents := make([]*SpanEvent, 0, len(src))
	for _, event := range src {
		if event == nil {
			continue
		}
		innerSpanEvents = append(innerSpanEvents, &SpanEvent{
			TimeUnixNano:           strconv.FormatUint(event.TimeUnixNano, 10),
			Name:                   event.Name,
			Attributes:             otelAttributeListPbToJson(event.Attributes),
			DroppedAttributesCount: event.DroppedAttributesCount,
		})
	}
	return innerSpanEvents
}

func otelSpanLinksPbToJson(src []*v1.Span_Link) []*SpanLink {
	if len(src) == 0 {
		return nil
	}
	innerSpanLinks := make([]*SpanLink, 0, len(src))
	for _, link := range src {
		if link == nil {
			continue
		}

		innerSpanLinks = append(innerSpanLinks, &SpanLink{
			TraceId:                hex.EncodeToString(link.TraceId),
			SpanId:                 hex.EncodeToString(link.SpanId),
			TraceState:             link.TraceState,
			Attributes:             otelAttributeListPbToJson(link.Attributes),
			DroppedAttributesCount: link.DroppedAttributesCount,
			Flags:                  link.Flags,
		})
	}
	return innerSpanLinks
}

func otelAttributeListPbToJson(src []*v2.KeyValue) []*KeyValue {
	if len(src) == 0 {
		return nil
	}
	innerAttributeList := make([]*KeyValue, 0, len(src))
	for _, attribute := range src {
		if attribute == nil {
			continue
		}
		innerAttributeList = append(innerAttributeList, &KeyValue{
			Key:   attribute.Key,
			Value: otelAnyValuePbToJson(attribute.Value),
		})
	}
	return innerAttributeList
}

func OtelTraceRequestPbToJson(src *v3.ExportTraceServiceRequest) *ExportTraceServiceRequest {
	if src == nil {
		return nil
	}

	innerReq := &ExportTraceServiceRequest{
		ResourceSpans: make([]*ResourceSpans, 0, len(src.ResourceSpans)),
	}
	for _, rs := range src.ResourceSpans {
		if rs == nil {
			continue
		}

		resource := &Resource{
			Attributes: make([]*KeyValue, 0, len(rs.Resource.Attributes)),
		}
		for _, attribute := range rs.Resource.Attributes {
			if attribute == nil {
				continue
			}
			resource.Attributes = append(resource.Attributes, &KeyValue{
				Key:   attribute.Key,
				Value: otelAnyValuePbToJson(attribute.Value),
			})
		}

		innerRs := &ResourceSpans{
			Resource:   resource,
			SchemaUrl:  rs.SchemaUrl,
			ScopeSpans: make([]*ScopeSpans, 0, len(rs.ScopeSpans)),
		}

		for _, ss := range rs.ScopeSpans {
			if ss == nil {
				continue
			}

			innerSs := &ScopeSpans{
				Scope:     otelInstrumentationScopePbToJson(ss.Scope),
				SchemaUrl: ss.SchemaUrl,
				Spans:     make([]*Span, 0, len(ss.Spans)),
			}
			for _, s := range ss.Spans {
				if s == nil {
					continue
				}
				innerSpan := &Span{
					TraceId:                hex.EncodeToString(s.TraceId),
					SpanId:                 hex.EncodeToString(s.SpanId),
					TraceState:             s.TraceState,
					ParentSpanId:           hex.EncodeToString(s.ParentSpanId),
					Flags:                  s.Flags,
					Name:                   s.Name,
					Kind:                   s.Kind,
					StartTimeUnixNano:      strconv.FormatUint(s.StartTimeUnixNano, 10),
					EndTimeUnixNano:        strconv.FormatUint(s.EndTimeUnixNano, 10),
					Attributes:             otelAttributeListPbToJson(s.Attributes),
					DroppedAttributesCount: s.DroppedAttributesCount,
					Events:                 otelSpanEventsPbToJson(s.Events),
					DroppedEventsCount:     s.DroppedEventsCount,
					Links:                  otelSpanLinksPbToJson(s.Links),
					DroppedLinksCount:      s.DroppedLinksCount,
					Status:                 s.Status,
				}
				innerSs.Spans = append(innerSs.Spans, innerSpan)
			}
			innerRs.ScopeSpans = append(innerRs.ScopeSpans, innerSs)
		}
		innerReq.ResourceSpans = append(innerReq.ResourceSpans, innerRs)
	}

	return innerReq
}
