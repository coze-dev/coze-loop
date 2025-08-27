// Copyright (c) 2025 Bytedance Ltd. and/or its affiliates
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

	// model
	otelTraceLoopAttributeModelSpanType = "gen_ai.request.type" // traceloop span type
	otelAttributeModelTimeToFirstToken  = "cozeloop.time_to_first_token"
	otelAttributeModelStream            = "cozeloop.stream"

	// prompt
	otelAttributePromptKey      = "cozeloop.prompt_key"
	otelAttributePromptVersion  = "cozeloop.prompt_version"
	otelAttributePromptProvider = "cozeloop.prompt_provider"
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

var otelMessageEventNameMap = []string{
	otelEventModelSystemMessage,
	otelEventModelUserMessage,
	otelEventModelToolMessage,
	otelEventModelAssistantMessage,
	otelEventModelChoice,
}

var otelMessageAttributeKeyMap = []string{
	string(semconv1_27_0.GenAIPromptKey),
	string(semconv1_27_0.GenAICompletionKey),
}

// tag key
const (
	// common
	tagKeyThreadID           = "thread_id"
	tagKeyUserID             = "user_id"
	tagKeyMessageID          = "message_id"
	tagKeyStartTimeFirstResp = "start_time_first_resp"
)

var (
	otelModelSpanTypeMap = map[string]string{
		"chat":             tracespec.VModelSpanType,
		"execute_tool":     tracespec.VToolSpanType,
		"generate_content": tracespec.VModelSpanType,
		"text_completion":  tracespec.VModelSpanType,
		"":                 "custom",
	}
)

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
