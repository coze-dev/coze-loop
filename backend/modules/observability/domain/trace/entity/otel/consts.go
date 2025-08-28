// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"github.com/coze-dev/cozeloop-go/spec/tracespec"
	semconv1_27_0 "go.opentelemetry.io/otel/semconv/v1.27.0"
)

const (
	ContentTypeJson     = "application/json"
	ContentTypeProtoBuf = "application/x-protobuf"
)

// cozeloop attribute key
const (
	// common
	OtelAttributeWorkSpaceID = "cozeloop.workspace_id"
	otelAttributeSpanType    = "cozeloop.span_type"
	otelAttributeInput       = "cozeloop.input"
	otelAttributeOutput      = "cozeloop.output"
	otelAttributeLogID       = "cozeloop.logid"

	// model
	otelTraceLoopAttributeModelSpanType = "gen_ai.request.type" // traceloop span type
	otelAttributeModelTimeToFirstToken  = "cozeloop.time_to_first_token"
	otelAttributeModelStream            = "cozeloop.stream"

	// prompt
	otelAttributePromptKey      = "cozeloop.prompt_key"
	otelAttributePromptVersion  = "cozeloop.prompt_version"
	otelAttributePromptProvider = "cozeloop.prompt_provider"
)

// openinference attribute key
const (
	// common
	openInferenceAttributeInput     = "input.value"
	openInferenceAttributeOutput    = "output.value"
	openInferenceAttributeSpanKind  = "openinference.span.kind"
	openInferenceAttributeException = "exception"

	// model
	openInferenceAttributeModelInputMessages  = "llm.input_messages"
	openInferenceAttributeModelInputTools     = "llm.tools"
	openInferenceAttributeModelOutputMessages = "llm.output_messages"
	openInferenceAttributeModelName           = "llm.model_name"
	openInferenceAttributeModelInputTokens    = "llm.token_count.prompt"
	openInferenceAttributeModelOutputTokens   = "llm.token_count.completion"

	// tool
	openInferenceAttributeToolInput = "tool"
)

// otel event name
const (
	// model
	// input
	otelEventModelSystemMessage    = "gen_ai.system.message"
	otelEventModelUserMessage      = "gen_ai.user.message"
	otelEventModelAssistantMessage = "gen_ai.assistant.message"
	otelEventModelToolMessage      = "gen_ai.tool.message"
	otelSpringAIEventModelPrompt   = "gen_ai.content.prompt" // springAI prompt event name

	// output
	otelEventModelChoice             = "gen_ai.choice"
	otelSpringAIEventModelCompletion = "gen_ai.content.completion" // springAI completion event name
)

// otel attribute key prefix
const (
	otelAttributeErrorPrefix = "error"
	otelAttributeToolsPrefix = "gen_ai.request.functions" // tools
)

var otelMessageEventNameMap = []string{
	otelEventModelSystemMessage,
	otelEventModelUserMessage,
	otelEventModelToolMessage,
	otelEventModelAssistantMessage,
	otelEventModelChoice,
}

var otelMessageAttributeKeyMap = []string{ //nolint:unused
	string(semconv1_27_0.GenAIPromptKey),
	string(semconv1_27_0.GenAICompletionKey),
}

// tag key
const (
	// common
	tagKeyThreadID           = "thread_id"
	tagKeyLogID              = "logid"
	tagKeyUserID             = "user_id"
	tagKeyMessageID          = "message_id"
	tagKeyStartTimeFirstResp = "start_time_first_resp"
)

var otelModelSpanTypeMap = map[string]string{
	"": "custom",
	// 以下为otel的span type
	"chat":             tracespec.VModelSpanType,
	"execute_tool":     tracespec.VToolSpanType,
	"generate_content": tracespec.VModelSpanType,
	"text_completion":  tracespec.VModelSpanType,
	// 以下为openinference的span type
	"TOOL":      tracespec.VToolSpanType,
	"LLM":       tracespec.VModelSpanType,
	"RETRIEVER": tracespec.VRetrieverSpanType,
}

// inner process key
const (
	innerArray = "cozeloop-inner-array-key"
)

const (
	dataTypeDefault     = ""
	dataTypeString      = "string"
	dataTypeInt64       = "int64"
	dataTypeFloat64     = "float64"
	dataTypeBool        = "bool"
	dataTypeArrayString = "array_string"
)
