// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consts

const (
	EvaluationConfigFileName = "evaluation.yaml"
)

const (
	RateLimitTccDynamicConfKey = "rate_limit_conf"
	RateLimitBizKeyEvaluator   = "run_evaluator"
)

const (
	Read  = "read"
	Edit  = "edit"
	Run   = "run"
	Debug = "debug"
)

const (
	IdemAbaseTableName = "evaluation_idem"

	IdemModuleEvaluator    = "evaluator_version"
	IdemKeyCreateEvaluator = "create_evaluator_idem"
	IdemKeySubmitEvaluator = "submit_evaluator_idem"
)

const (
	InputSchemaKey      = "input"
	OutputSchemaKey     = "actual_output"
	StringJsonSchema    = "{\"type\":\"string\"}"
	MapStringJsonSchema = "{\"type\":\"object\",\"additionalProperties\":{\"type\":\"string\"}}"
)

const ClusterNameConsumer = "consumer"

const (
	ResourceNotFoundCode = int32(777012040)
)

const PromptPersonalDraftVersion = "$Draft"

const (
	MaxEvaluatorNameLength        = 50
	MaxEvaluatorDescLength        = 200
	MaxEvaluatorVersionLength     = 50
	MaxEvaluatorVersionDescLength = 200
)
