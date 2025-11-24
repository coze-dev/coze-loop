// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consts

const (
	ActionCreateExpt = "createLoopEvaluationExperiment"
	ActionReadExpt   = "listLoopEvaluationExperiment"

	ActionDebugEvalTarget = "debugLoopEvalTarget"
)

const (
	SortDesc = "desc"
	SortAsc  = "asc"
)

const (
	DefaultSourceTargetVersion = "0.0.1"
)

const (
	MaxEvalSetItemLimit = 5000

	MaxItemConcurrentNum = 50 // TODO(@liushengyang): value
)

const (
	FieldAdapterBuiltinFieldNameRuntimeParam = "builtin_runtime_param"
	TargetExecuteExtRuntimeParamKey          = "builtin_runtime_param"
)

const (
	InsightAnalysisNotifyCardID = "AAq9DvIYd2qHu"
)

const (
	ReportColumnNameEvalTargetActualOutput  = "actual_output"
	ReportColumnLabelEvalTargetActualOutput = "实际输出"
	ReportColumnNameEvalTargetTrajectory    = "trajectory"
	ReportColumnLabelEvalTargetTrajectory   = "轨迹"
)
