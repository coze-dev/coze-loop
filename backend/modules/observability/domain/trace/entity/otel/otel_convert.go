// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/otel/open_inference"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"

	semconv1_26_0 "go.opentelemetry.io/otel/semconv/v1.26.0"
	semconv1_27_0 "go.opentelemetry.io/otel/semconv/v1.27.0"
	semconv1_32_0 "go.opentelemetry.io/otel/semconv/v1.32.0"

	"github.com/bytedance/sonic"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"
)

// Field configuration, supports configuring data sources and export methods for fields, currently supports attribute, event, is_tag, data_type
// Among them, attributes and events support configuring multiple, while tags and datatypes only support configuring one.
// Other types of configurations need to be manually processed in the code.
var (
	fieldConfMap = map[string]fieldConf{
		// common
		"span_type": {
			attributeKey: []string{
				otelAttributeSpanType,
				otelTraceLoopAttributeModelSpanType,
				string(semconv1_32_0.GenAIOperationNameKey),
				openInferenceAttributeSpanKind,
			},
			isTag:    false,
			dataType: dataTypeString,
		},
		tagKeyThreadID: {
			attributeKey: []string{string(semconv1_26_0.SessionIDKey)},
			isTag:        true,
			dataType:     dataTypeString,
		},
		tagKeyLogID: {
			attributeKey: []string{otelAttributeLogID},
			isTag:        true,
			dataType:     dataTypeString,
		},
		tagKeyUserID: {
			attributeKey: []string{string(semconv1_32_0.UserIDKey)},
			isTag:        true,
			dataType:     dataTypeString,
		},
		tagKeyMessageID: {
			attributeKey: []string{string(semconv1_32_0.MessagingMessageIDKey)},
			isTag:        true,
			dataType:     dataTypeString,
		},
		tracespec.Error: {
			attributeKeyPrefix: []string{
				otelAttributeErrorPrefix,
				openInferenceAttributeException,
			},
			eventName: []string{semconv1_32_0.ExceptionEventName},
			isTag:     true,
			dataType:  dataTypeString,
		},

		// model
		tracespec.ModelProvider: {
			attributeKey: []string{string(semconv1_32_0.GenAISystemKey)},
			isTag:        true,
			dataType:     dataTypeString,
		},
		tracespec.Input: {
			attributeKey: []string{
				openInferenceAttributeInput,
				otelAttributeInput,
			},
			attributeKeyPrefix: []string{
				openInferenceAttributeModelInputMessages,
				openInferenceAttributeToolInput,
				string(semconv1_27_0.GenAIPromptKey),
			},
			eventName: []string{otelEventModelSystemMessage, otelEventModelUserMessage, otelEventModelToolMessage, otelEventModelAssistantMessage, otelSpringAIEventModelPrompt},
			dataType:  dataTypeString,
			eventHighLevelKey: []highLevelKeyRuleConf{
				{
					key:  "messages",
					rule: highLevelKeyRuleMap,
				},
			},
			attributeHighLevelKey: []highLevelKeyRuleConf{
				{
					key:  "messages",
					rule: highLevelKeyRuleMap,
				},
			},
		},
		tracespec.Output: {
			attributeKey: []string{
				openInferenceAttributeOutput,
				otelAttributeOutput,
			},
			attributeKeyPrefix: []string{
				openInferenceAttributeModelOutputMessages,
				string(semconv1_27_0.GenAICompletionKey),
			},
			eventName: []string{otelEventModelChoice, otelSpringAIEventModelCompletion},
			dataType:  dataTypeString,
			eventHighLevelKey: []highLevelKeyRuleConf{
				{
					key:  "choices",
					rule: highLevelKeyRuleMap,
				},
			},
			attributeHighLevelKey: []highLevelKeyRuleConf{
				{
					key:  "message",
					rule: highLevelKeyRuleMap,
				},
				{
					key:  "choices",
					rule: highLevelKeyRuleList,
				},
			},
		},
		tagKeyStartTimeFirstResp: {
			attributeKey: []string{otelAttributeModelTimeToFirstToken},
			isTag:        true,
			dataType:     dataTypeInt64,
		},
		tracespec.Stream: {
			attributeKey: []string{otelAttributeModelStream},
			isTag:        true,
			dataType:     dataTypeBool,
		},
		tracespec.ModelName: {
			attributeKey: []string{
				string(semconv1_32_0.GenAIRequestModelKey),
				string(semconv1_27_0.GenAIResponseModelKey),
				openInferenceAttributeModelName,
			},
			isTag:    true,
			dataType: dataTypeString,
		},
		"temperature": {
			attributeKey: []string{string(semconv1_32_0.GenAIRequestTemperatureKey)},
			isTag:        true,
			dataType:     dataTypeFloat64,
		},
		"top_p": {
			attributeKey: []string{string(semconv1_32_0.GenAIRequestTopPKey)},
			isTag:        true,
			dataType:     dataTypeFloat64,
		},
		"top_k": {
			attributeKey: []string{string(semconv1_32_0.GenAIRequestTopKKey)},
			isTag:        true,
			dataType:     dataTypeInt64,
		},
		"max_tokens": {
			attributeKey: []string{string(semconv1_32_0.GenAIRequestMaxTokensKey)},
			isTag:        true,
			dataType:     dataTypeInt64,
		},
		"frequency_penalty": {
			attributeKey: []string{string(semconv1_32_0.GenAIRequestFrequencyPenaltyKey)},
			isTag:        true,
			dataType:     dataTypeFloat64,
		},
		"presence_penalty": {
			attributeKey: []string{string(semconv1_32_0.GenAIRequestPresencePenaltyKey)},
			isTag:        true,
			dataType:     dataTypeFloat64,
		},
		"stop_sequences": {
			attributeKey: []string{string(semconv1_32_0.GenAIRequestStopSequencesKey)},
			isTag:        true,
			dataType:     dataTypeArrayString,
		},
		tracespec.InputTokens: {
			attributeKey: []string{
				string(semconv1_32_0.GenAIUsageInputTokensKey),
				string(semconv1_26_0.GenAiUsagePromptTokensKey),
				openInferenceAttributeModelInputTokens,
			},
			isTag:    true,
			dataType: dataTypeInt64,
		},
		tracespec.OutputTokens: {
			attributeKey: []string{
				string(semconv1_32_0.GenAIUsageOutputTokensKey),
				string(semconv1_26_0.GenAiUsageCompletionTokensKey),
				openInferenceAttributeModelOutputTokens,
			},
			isTag:    true,
			dataType: dataTypeInt64,
		},

		// prompt
		tracespec.PromptKey: {
			attributeKey: []string{otelAttributePromptKey},
			isTag:        true,
			dataType:     dataTypeString,
		},
		tracespec.PromptVersion: {
			attributeKey: []string{otelAttributePromptVersion},
			isTag:        true,
			dataType:     dataTypeString,
		},
		tracespec.PromptProvider: {
			attributeKey: []string{otelAttributePromptProvider},
			isTag:        true,
			dataType:     dataTypeString,
		},
	}
)

type fieldConf struct {
	attributeKey          []string
	attributeKeyPrefix    []string
	eventName             []string
	isTag                 bool
	dataType              string
	eventHighLevelKey     []highLevelKeyRuleConf // config from inner to outer, such as choices.message.xxx, config is ["message", "choices"]
	attributeHighLevelKey []highLevelKeyRuleConf // config from inner to outer, such as choices.message.xxx, config is ["message", "choices"]
}

type highLevelKeyRuleConf struct {
	key  string
	rule highLevelPackRule
}

type highLevelPackRule string

const (
	highLevelKeyRuleMap  highLevelPackRule = "map"
	highLevelKeyRuleList highLevelPackRule = "list"
)

var (
	registeredAttributeMap       map[string]bool
	registeredAttributePrefixMap map[string]bool
)

func init() {
	registeredAttributeMap = make(map[string]bool)
	registeredAttributePrefixMap = make(map[string]bool)
	for _, fieldConf := range fieldConfMap {
		for _, attribute := range fieldConf.attributeKey {
			registeredAttributeMap[attribute] = true
		}
		for _, attributePrefix := range fieldConf.attributeKeyPrefix {
			registeredAttributePrefixMap[attributePrefix] = true
		}
	}
}

func OtelSpansConvertToSendSpans(ctx context.Context, spaceID string, spans []*ResourceScopeSpan) loop_span.SpanList {
	result := make(loop_span.SpanList, 0)
	for i := range spans {
		if span := OtelSpanConvertToSendSpan(ctx, spaceID, spans[i]); span != nil {
			result = append(result, span)
		}
	}
	return result
}

func OtelSpanConvertToSendSpan(ctx context.Context, spaceID string, resourceScopeSpan *ResourceScopeSpan) *loop_span.Span {
	if resourceScopeSpan == nil || resourceScopeSpan.Span == nil {
		return nil
	}
	span := resourceScopeSpan.Span
	startTimeUnixNanoInt64, err := strconv.ParseInt(span.StartTimeUnixNano, 10, 64)
	if err != nil {
		logs.CtxError(ctx, "startTimeUnixNano convert to int64 failed err=%+v", err)
	}
	endTimeUnixNanoInt64, err := strconv.ParseInt(span.EndTimeUnixNano, 10, 64)
	if err != nil {
		logs.CtxError(ctx, "endTimeUnixNano convert to int64 failed err=%+v", err)
	}

	attributeMap := make(map[string]*AnyValue)
	for _, spanAttribute := range span.Attributes {
		if spanAttribute == nil {
			continue
		}
		attributeMap[spanAttribute.Key] = spanAttribute.Value
	}
	resMap := processAttributesAndEvents(ctx, attributeMap, span.Events)

	spanType := ""
	input := ""
	output := ""
	statusCode := int32(0)
	tagsString := make(map[string]string)
	tagsLong := make(map[string]int64)
	tagsDouble := make(map[string]float64)
	tagsBool := make(map[string]bool)
	systemTagsString := make(map[string]string)
	for fieldKey, srcValue := range resMap {
		if srcValue == nil {
			continue
		}
		conf, ok := fieldConfMap[fieldKey]
		if !ok {
			continue
		}

		switch conf.dataType {
		case dataTypeString, dataTypeDefault:
			value, ok := srcValue.(string)
			if !ok {
				continue
			}
			if conf.isTag {
				tagsString[fieldKey] = value
			} else {
				switch fieldKey {
				case "span_type":
					spanType = spanTypeMapping(value)
				case "input":
					input = value
				case "output":
					output = value
				default:
				}
			}
		case dataTypeInt64:
			value, ok := srcValue.(int64)
			if !ok {
				continue
			}
			if conf.isTag {
				tagsLong[fieldKey] = value
			}
		case dataTypeBool:
			value, ok := srcValue.(bool)
			if !ok {
				continue
			}
			if conf.isTag {
				tagsBool[fieldKey] = value
			}
		case dataTypeFloat64:
			value, ok := srcValue.(float64)
			if !ok {
				continue
			}
			if conf.isTag {
				tagsDouble[fieldKey] = value
			}
		case dataTypeArrayString:
			value, ok := srcValue.([]string)
			if !ok {
				continue
			}
			if conf.isTag {
				tagsString[fieldKey] = strings.Join(value, ",")
			}
		default:
		}
	}

	// final processing: data that needs to be calculated by integrating overall data
	// calculate latency_first_resp
	calLatencyFirstResp(tagsLong, startTimeUnixNanoInt64)
	// calculate tokens
	calTokens(tagsLong)
	// merge call_options
	calCallOptions(ctx, tagsDouble, tagsLong, tagsString)
	// error mapping status_code
	statusCode = calStatusCode(tagsString, statusCode)
	// set attributes
	calOtherAttribute(ctx, span, tagsString, tagsLong, tagsDouble, tagsBool)
	// set runtime
	calRuntime(systemTagsString, resourceScopeSpan)

	result := &loop_span.Span{
		StartTime:        startTimeUnixNanoInt64 / 1000,
		SpanID:           span.SpanId,
		ParentID:         span.ParentSpanId,
		LogID:            "",
		TraceID:          span.TraceId,
		DurationMicros:   (endTimeUnixNanoInt64 - startTimeUnixNanoInt64) / 1000,
		PSM:              "",
		CallType:         "Custom",
		WorkspaceID:      spaceID,
		SpanName:         span.Name,
		SpanType:         spanType,
		Method:           "",
		StatusCode:       statusCode,
		Input:            input,
		Output:           output,
		ObjectStorage:    "",
		SystemTagsString: systemTagsString,
		SystemTagsLong:   nil,
		SystemTagsDouble: nil,
		TagsString:       tagsString,
		TagsLong:         tagsLong,
		TagsDouble:       tagsDouble,
		TagsBool:         tagsBool,
		TagsByte:         nil,
	}
	setLogID(result)

	return result
}

func setLogID(span *loop_span.Span) {
	if span == nil || span.TagsString == nil {
		return
	}
	span.LogID = span.TagsString["logid"]
	delete(span.TagsString, "logid")
}

func spanTypeMapping(spanType string) string {
	desSpanType, ok := otelModelSpanTypeMap[spanType]
	if ok {
		spanType = desSpanType
	}

	return spanType
}

func calLatencyFirstResp(tagsLong map[string]int64, startTimeUnixNanoInt64 int64) {
	startTimeFirstResp, ok := tagsLong[tagKeyStartTimeFirstResp]
	if ok {
		tagsLong[tracespec.LatencyFirstResp] = startTimeFirstResp - startTimeUnixNanoInt64/1000
	}
}

func calTokens(tagsLong map[string]int64) {
	inputTokens := tagsLong[tracespec.InputTokens]
	outputTokens := tagsLong[tracespec.OutputTokens]
	if inputTokens > 0 || outputTokens > 0 {
		tagsLong[tracespec.Tokens] = inputTokens + outputTokens
	}
}

func calCallOptions(ctx context.Context, tagsDouble map[string]float64, tagsLong map[string]int64, tagsString map[string]string) {
	modelCallOption := &tracespec.ModelCallOption{}
	temperature := tagsDouble["temperature"]
	topP := tagsDouble["top_p"]
	maxTokens := tagsLong["max_tokens"]
	frequencyPenalty := tagsDouble["frequency_penalty"]
	presencePenalty := tagsDouble["presence_penalty"]
	stopSequences := tagsString["stop_sequences"]
	topK := tagsLong["top_k"]
	if temperature > 0 || topP > 0 || topK > 0 || maxTokens > 0 || frequencyPenalty > 0 || presencePenalty > 0 || len(stopSequences) > 0 {
		modelCallOption.Temperature = float32(temperature)
		delete(tagsDouble, "temperature")
		modelCallOption.MaxTokens = maxTokens
		delete(tagsLong, "max_tokens")
		modelCallOption.TopP = float32(topP)
		delete(tagsLong, "top_p")
		modelCallOption.TopK = gptr.Of(topK)
		delete(tagsLong, "top_k")
		modelCallOption.FrequencyPenalty = gptr.Of(float32(frequencyPenalty))
		delete(tagsLong, "frequency_penalty")
		modelCallOption.PresencePenalty = gptr.Of(float32(presencePenalty))
		delete(tagsLong, "presence_penalty")
		modelCallOption.Stop = strings.Split(stopSequences, ",")
		delete(tagsLong, "stop_sequences")
		bytes, err := sonic.Marshal(modelCallOption)
		if err != nil {
			logs.CtxError(ctx, "modelCallOption marshal failed err=%+v", err)
		} else {
			tagsString[tracespec.CallOptions] = string(bytes)
		}
	}
}

func calStatusCode(tagsString map[string]string, statusCode int32) int32 {
	if _, ok := tagsString[tracespec.Error]; ok && statusCode == 0 {
		return -1
	}

	return statusCode
}

func calOtherAttribute(ctx context.Context, span *Span, tagsString map[string]string, tagsLong map[string]int64, tagsDouble map[string]float64, tagsBool map[string]bool) {
	for _, attribute := range span.Attributes {
		if attribute == nil {
			continue
		}
		// registered attribute processed, skip
		if _, ok := registeredAttributeMap[attribute.Key]; ok {
			continue
		}
		// registered attribute prefix processed, skip
		hasPrefix := false
		for prefix := range registeredAttributePrefixMap {
			if strings.HasPrefix(attribute.Key, prefix) {
				hasPrefix = true
				break
			}
		}
		if hasPrefix {
			continue
		}

		if attribute.Value.GetStringValue() != "" {
			tagsString[attribute.Key] = attribute.Value.GetStringValue()
		} else if attribute.Value.IsIntValue() {
			tagsLong[attribute.Key] = attribute.Value.GetIntValue()
		} else if attribute.Value.IsDoubleValue() {
			tagsDouble[attribute.Key] = attribute.Value.GetDoubleValue()
		} else if attribute.Value.IsBoolValue() {
			tagsBool[attribute.Key] = attribute.Value.GetBoolValue()
		} else if attribute.Value.IsArrayValue() {
			tagsString[attribute.Key] = attribute.Value.GetArrayValue().String(ctx)
		} else if attribute.Value.IsBytesValue() {
			tagsString[attribute.Key] = string(attribute.Value.GetBytesValue())
		} else if attribute.Value.IsKvlistValue() {
			tagsString[attribute.Key] = attribute.Value.GetKvlistValue().String(ctx)
		}
	}
}

func calRuntime(systemTagsString map[string]string, resourceScopeSpan *ResourceScopeSpan) {
	systemTagsString[tracespec.Runtime_] = getRuntime(resourceScopeSpan)
}

func getRuntime(resourceScopeSpan *ResourceScopeSpan) string {
	runtime := processRuntime(resourceScopeSpan)
	marshalString, err := sonic.MarshalString(runtime)
	if err != nil {
		return "" // unexpected
	}

	return marshalString
}

func processRuntime(resourceScopeSpan *ResourceScopeSpan) *tracespec.Runtime {
	res := &tracespec.Runtime{
		Library:      tracespec.VLibOpentelemetry,
		Scene:        "",
		SceneVersion: "",
	}

	resourceMap := make(map[string]interface{})
	if resourceScopeSpan.Resource != nil {
		for _, attribute := range resourceScopeSpan.Resource.Attributes {
			resourceMap[attribute.Key] = attribute.Value.GetCorrectTypeValue()
		}
	}

	if lang, ok := resourceMap[string(semconv1_32_0.TelemetrySDKLanguageKey)]; ok {
		res.Language = fmt.Sprintf("%v", lang)
	}
	if ver, ok := resourceMap[string(semconv1_32_0.TelemetrySDKVersionKey)]; ok {
		res.LibraryVersion = fmt.Sprintf("%v", ver)
	}
	if resourceScopeSpan.Scope != nil {
		res.Scene = resourceScopeSpan.Scope.Name
		res.SceneVersion = resourceScopeSpan.Scope.Version
	}

	return res
}

func processAttributesAndEvents(ctx context.Context, attributeMap map[string]*AnyValue, events []*SpanEvent) map[string]interface{} {
	result := make(map[string]interface{})

	// for a certain field, process it gradually according to its value priority,
	// first processing the low priority ones, and then processing the high priority ones.
	for fieldKey, conf := range fieldConfMap {
		var singleRes interface{}
		// attribute key
		attributeKeyRes := processAttributeKey(ctx, conf, attributeMap)
		if attributeKeyRes != nil {
			singleRes = attributeKeyRes
		}

		// attribute prefix
		attributePrefixRes := processAttributePrefix(ctx, fieldKey, conf, attributeMap)
		if attributePrefixRes != "" {
			singleRes = attributePrefixRes
		}

		// event
		eventRes := processEvent(ctx, fieldKey, conf, events, attributeMap)
		if len(eventRes) != 0 {
			singleRes = eventRes
		}

		result[fieldKey] = singleRes
	}

	return result
}

func getSamePrefixAttributesMap(attributeMap map[string]*AnyValue, prefixKey string) map[string]interface{} {
	samePrefixAttributesMap := make(map[string]interface{})
	for key, value := range attributeMap {
		if strings.HasPrefix(key, prefixKey) {
			samePrefixAttributesMap[key] = value.GetCorrectTypeValue()
		}
	}

	return samePrefixAttributesMap
}

func processAttributeKey(ctx context.Context, conf fieldConf, attributeMap map[string]*AnyValue) interface{} {
	if attributeKeys := conf.attributeKey; len(attributeKeys) > 0 {
		for _, key := range attributeKeys {
			if x, ok := attributeMap[key]; ok {
				return getValueByDataType(x, conf.dataType)
			}
		}
	}

	return nil
}

func processAttributePrefix(ctx context.Context, fieldKey string, conf fieldConf, attributeMap map[string]*AnyValue) string {
	for _, attributePrefixKey := range conf.attributeKeyPrefix {
		srcAttrAggrRes := aggregateAttributesByPrefix(attributeMap, attributePrefixKey)
		if srcAttrAggrRes == nil {
			continue
		}
		toBeMarshalObject := srcAttrAggrRes

		// special process
		switch attributePrefixKey {
		case string(semconv1_27_0.GenAIPromptKey), string(semconv1_27_0.GenAICompletionKey): // only the standard message attribute of otel requires packaging on the outer layer, and everything else is ignored
			if fieldKey == tracespec.Output { // output
				if srcAttrAggrSlice, ok := srcAttrAggrRes.([]interface{}); ok && len(srcAttrAggrSlice) > 0 {
					choices := make([]interface{}, 0)
					for _, singleMess := range srcAttrAggrSlice {
						choices = append(choices, map[string]interface{}{
							"message": singleMess,
						})
					}
					toBeMarshalObject = map[string]interface{}{
						"choices": choices,
					}
				}
			} else { // input
				toBeMarshalObject = packHighLevelKey(srcAttrAggrRes, conf.attributeHighLevelKey)
				// pack tools
				if temp, ok := toBeMarshalObject.(map[string]interface{}); ok {
					tools := getModelTools(ctx, attributeMap)
					if tools != nil {
						temp["tools"] = tools
						toBeMarshalObject = temp
					}
				}
			}
		case openInferenceAttributeModelInputMessages: // openInference input message
			srcInput, err := open_inference.ConvertToModelInput(srcAttrAggrRes)
			if err != nil {
				logs.CtxWarn(ctx, "input ConvertToModelInput failed err=%+v", err)
				continue
			}
			// pack tools
			srcTools := aggregateAttributesByPrefix(attributeMap, openInferenceAttributeModelInputTools)
			toBeMarshalObject, err = open_inference.AddTools2ModelInput(srcInput, srcTools)
			if err != nil {
				logs.CtxWarn(ctx, "openInference AddTools2ModelInput failed err=%+v", err)
				continue
			}
		case openInferenceAttributeModelOutputMessages: // openInference output message
			resObject, err := open_inference.ConvertToModelOutput(srcAttrAggrRes)
			if err != nil {
				logs.CtxWarn(ctx, "input ConvertToModelOutput failed err=%+v", err)
			} else {
				toBeMarshalObject = resObject
			}
		default:
		}

		tempBytes, err := sonic.Marshal(toBeMarshalObject)
		if err != nil {
			logs.CtxError(ctx, "input aggregateAttributes failed err=%+v", err)
		} else {
			return string(tempBytes)
		}

	}

	return ""
}

func aggregateAttributesByPrefix(attributeMap map[string]*AnyValue, attributePrefixKey string) interface{} {
	var srcAttrAggrRes interface{}
	samePrefixAttributesMap := getSamePrefixAttributesMap(attributeMap, attributePrefixKey)
	if len(samePrefixAttributesMap) > 0 {
		srcAttrAggrRes = aggregateAttributes(samePrefixAttributesMap, attributePrefixKey)
	}

	return srcAttrAggrRes
}

func processEvent(ctx context.Context, fieldKey string, conf fieldConf, events []*SpanEvent, attributeMap map[string]*AnyValue) string {
	if len(events) == 0 || len(conf.eventName) == 0 {
		return ""
	}
	eventSlice := make([]map[string]interface{}, 0)
	isAllOtelMessage := true // only otel standard message events require packaging on the outer layer, the rest are not included
	for _, event := range events {
		if !slices.Contains(conf.eventName, event.Name) {
			continue
		}
		if !slices.Contains(otelMessageEventNameMap, event.Name) {
			isAllOtelMessage = false
		}
		tempMap := make(map[string]interface{})
		for _, eventAttribute := range event.Attributes {
			if eventAttribute == nil {
				continue
			}
			tempMap[eventAttribute.Key] = eventAttribute.Value.GetCorrectTypeValue()
		}
		eventSlice = append(eventSlice, tempMap)
	}
	if len(eventSlice) > 0 {
		tempRes := make([]map[string]interface{}, 0, len(eventSlice))
		for _, m := range eventSlice {
			singleRes := aggregateAttributes(m, "")
			if r, ok := singleRes.(map[string]interface{}); ok {
				tempRes = append(tempRes, r)
			}
		}
		// determine whether to use an array based on the quantity
		var resBytes []byte
		var toBeMarshalObject interface{}
		if len(conf.eventHighLevelKey) != 0 && isAllOtelMessage {
			toBeMarshalObject = packHighLevelKey(tempRes, conf.eventHighLevelKey)
			if fieldKey == tracespec.Input {
				// pack tools
				if temp, ok := toBeMarshalObject.(map[string]interface{}); ok {
					tools := getModelTools(ctx, attributeMap)
					if tools != nil {
						temp["tools"] = tools
						toBeMarshalObject = temp
					}
				}
			}
		} else {
			if len(tempRes) == 1 {
				toBeMarshalObject = tempRes[0]
			} else {
				toBeMarshalObject = tempRes
			}
		}
		bytes, err := sonic.Marshal(toBeMarshalObject)
		if err != nil {
			logs.CtxError(ctx, "modelInputEventMessageSlice marshal failed err=%+v", err)
		} else {
			resBytes = bytes
		}

		return string(resBytes)
	}

	return ""
}

func getModelTools(ctx context.Context, attributeMap map[string]*AnyValue) interface{} {
	res := make([]interface{}, 0)
	srcTools := aggregateAttributesByPrefix(attributeMap, otelAttributeToolsPrefix)
	if srcToolSlice, ok := srcTools.([]interface{}); ok {
		for _, f := range srcToolSlice {
			if fMap, ok := f.(map[string]interface{}); ok {
				if fParam, ok := fMap["parameters"]; ok {
					if fParamStr, ok := fParam.(string); ok {
						tempParameter := make(map[string]interface{}, 0)
						if err := sonic.UnmarshalString(fParamStr, &tempParameter); err != nil {
							logs.CtxInfo(ctx, "getModelTools UnmarshalString failed err=%+v", err)
						} else {
							fMap["parameters"] = tempParameter
						}
					}
				}
			}
			res = append(res, map[string]interface{}{
				"type":     "function",
				"function": f,
			})
		}
	}

	if len(res) == 0 {
		return nil
	}

	return res
}

func packHighLevelKey(src interface{}, highLevelKeyConfs []highLevelKeyRuleConf) interface{} {
	if len(highLevelKeyConfs) == 0 {
		return src
	}

	result := src
	for _, conf := range highLevelKeyConfs {
		switch conf.rule {
		case highLevelKeyRuleMap:
			result = map[string]interface{}{conf.key: result}
		case highLevelKeyRuleList:
			result = map[string][]interface{}{conf.key: {result}}
		default:
		}
	}

	return result
}

func getValueByDataType(src *AnyValue, dataType string) interface{} {
	if src == nil {
		return nil
	}
	switch dataType {
	case dataTypeString:
		return src.GetStringValue()
	case dataTypeInt64:
		return src.TryGetInt64Value()
	case dataTypeBool:
		return src.TryGetBoolValue()
	case dataTypeFloat64:
		return src.TryGetFloat64Value()
	case dataTypeArrayString:
		if src.GetArrayValue() == nil {
			return nil
		}
		return iterSlice(src.GetArrayValue().Values, func(a *AnyValue) string {
			return a.GetStringValue()
		})
	}

	return src.GetStringValue()
}

// aggregateAttributes Aggregate otel properties, supporting properties with prefixes.
// It can convert nested attributes of any form into corresponding objects.
func aggregateAttributes(srcInput map[string]interface{}, prefix string) interface{} {
	if len(prefix) == 0 {
		return aggregateTrimPrefixAttributes(srcInput)
	}

	if _, ok := srcInput[prefix]; ok {
		return srcInput[prefix]
	}

	newInput := make(map[string]interface{})
	for k, v := range srcInput {
		if strings.HasPrefix(k, prefix+".") {
			newInput[strings.TrimPrefix(k, prefix+".")] = v
		}
	}

	return aggregateTrimPrefixAttributes(newInput)
}

func aggregateTrimPrefixAttributes(input map[string]interface{}) interface{} {
	result := make(map[string]interface{})
	higherLevelKeys := make(map[string]bool)

	// check if there are higher-level keys, if there are, use them directly
	for key := range input {
		parts := strings.Split(key, ".")
		if len(parts) == 1 {
			higherLevelKeys[key] = true
		} else {
			for i := 1; i < len(parts); i++ {
				parentKey := strings.Join(parts[:i], ".")
				if _, exists := input[parentKey]; exists {
					higherLevelKeys[parentKey] = true
				}
			}
		}
	}

	for key, value := range input {
		// if there are higher-level keys and the current key is not a directly matching higher-level key, skip processing
		skip := false
		for higherKey := range higherLevelKeys {
			if strings.HasPrefix(key, higherKey+".") && key != higherKey {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		parts := strings.Split(key, ".")
		if len(parts) == 1 {
			result[key] = value
		} else {
			insertIntoStructure(result, parts, value)
		}
	}

	return convertArrays(result)
}

func insertIntoStructure(structure map[string]interface{}, keys []string, value interface{}) {
	current := structure
	for i, key := range keys {
		if i == len(keys)-1 {
			current[key] = value
			return
		}

		// check if it is an array index
		if index, err := strconv.Atoi(key); err == nil {
			// ensure that the array exists
			if _, exists := current[innerArray]; !exists {
				current[innerArray] = make([]interface{}, index+1)
			}
			arr, ok := current[innerArray].([]interface{})
			if !ok {
				// no way, just skip code check
				continue
			}

			// expand array size
			if index >= len(arr) {
				newArr := make([]interface{}, index+1)
				copy(newArr, arr)
				arr = newArr
				current[innerArray] = arr
			}

			// create the next level map
			if arr[index] == nil {
				arr[index] = make(map[string]interface{})
			}
			current, ok = arr[index].(map[string]interface{}) //nolint:staticcheck
			if !ok {
				// no way, just skip code check
				continue
			}
		} else {
			// handling regular keys
			if _, exists := current[key]; !exists {
				current[key] = make(map[string]interface{})
			}
			var ok bool
			current, ok = current[key].(map[string]interface{}) //nolint:staticcheck
			if !ok {
				// no way, just skip code check
				continue
			}
		}
	}
}

// convert the array struct to an actual array, while keeping the map
func convertArrays(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		if arr, exists := v[innerArray]; exists {
			// processing arrays
			realArray, ok := arr.([]interface{})
			if !ok {
				// no way, just skip code check
				return v
			}
			for i, item := range realArray {
				realArray[i] = convertArrays(item)
			}
			return realArray
		}
		// processing map
		result := make(map[string]interface{})
		for key, value := range v {
			result[key] = convertArrays(value)
		}
		return result
	default:
		return v
	}
}

func iterSlice[A, B any](sa []A, fb func(a A) B) []B {
	r := make([]B, len(sa))
	for i := range sa {
		r[i] = fb(sa[i])
	}

	return r
}
