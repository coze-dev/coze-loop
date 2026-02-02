// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consts

import (
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
)

const (
	ActionCreateExpt = "createLoopEvaluationExperiment"
	ActionReadExpt   = "listLoopEvaluationExperiment"

	ActionDebugEvalTarget = "debugLoopEvalTarget"

	ActionCreateExptTemplate = "createLoopExptTemplate"
	ActionReadExptTemplate   = "listLoopExptTemplate"
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

	ExptEventNotifyCardID = "AAqvJsfSSLQtN"

	ExptEventNotifyTitle           = "title"
	ExptEventNotifyTitleSuccess    = "已成功执行"
	ExptEventNotifyTitleFailed     = "执行失败"
	ExptEventNotifyTitleTerminated = "执行已被终止"

	ExptEventNotifyTitleColor           = "title_color"
	ExptEventNotifyTitleColorSuccess    = "green"
	ExptEventNotifyTitleColorFailed     = "red"
	ExptEventNotifyTitleColorTerminated = "yellow"
)

const (
	ReportColumnNameEvalTargetActualOutput  = expt.ColumnEvalTargetNameActualOutput
	ReportColumnLabelEvalTargetActualOutput = "实际输出"
	ReportColumnLabelEvalTargetExtOutput    = "自定义输出"

	ReportColumnNameEvalTargetTrajectory  = expt.ColumnEvalTargetNameTrajectory
	ReportColumnLabelEvalTargetTrajectory = "轨迹"

	ReportColumnNameEvalTargetTotalLatency        = expt.ColumnEvalTargetNameEvalTargetTotalLatency
	ReportColumnDisplayNameEvalTargetTotalLatency = "Total Latency(ms)"
	ReportColumnNameEvalTargetInputTokens         = expt.ColumnEvalTargetNameEvaluatorInputTokens
	ReportColumnDisplayNameEvalTargetInputTokens  = "Input Tokens"
	ReportColumnNameEvalTargetOutputTokens        = expt.ColumnEvalTargetNameEvaluatorOutputTokens
	ReportColumnDisplayNameEvalTargetOutputTokens = "Output Tokens"
	ReportColumnNameEvalTargetTotalTokens         = expt.ColumnEvalTargetNameEvaluatorTotalTokens
	ReportColumnDisplayNameEvalTargetTotalTokens  = "Total Tokens"
)
