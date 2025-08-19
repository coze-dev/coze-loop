// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consts

const (
	SpanTypePromptExecutor = "prompt_executor"
	SpanTypeSequence       = "sequence"
)

const (
	SpanNamePromptExecutor = "PromptExecutor"
	SpanNamePromptHub      = "PromptHub"
	SpanNameSequence       = "Sequence"
	SpanNamePromptTemplate = "PromptTemplate"
)

const (
	SpanTagCallType             = "call_type"
	SpanTagDebugID              = "debug_id"
	SpanTagPromptVariables      = "prompt_variables"
	SpanTagMessages             = "messages"
	SpanTagPromptTemplate       = "prompt_template"
	SpanTagPromptID             = "prompt_id"
	SpanTagOverridePromptParams = "override_prompt_params"
)

const (
	SpanTagCallTypePromptPlayground = "PromptPlayground"
	SpanTagCallTypePromptDebug      = "PromptDebug"
	SpanTagCallTypeEvaluation       = "Evaluation"
)
