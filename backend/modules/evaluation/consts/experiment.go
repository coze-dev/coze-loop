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
)

const (
	FieldAdapterBuiltinFieldNameRuntimeParam = "builtin_runtime_param"
	TargetExecuteExtRuntimeParamKey          = "builtin_runtime_param"
	// FieldAdapterBuiltinFieldNameSkillTOSKeys 执行期透传 EvalConf.SkillTOSKeys 用的 Ext key。
	// 值须与商业版 consts.ExtKeySkillTOSKeys 逐字一致，供商业版 buildSkillsConfig 命中 TOS 现签复用。
	FieldAdapterBuiltinFieldNameSkillTOSKeys = "agent_buddy_skill_tos_keys"
	// TargetExecuteExtRunConfKey 执行期透传题目级多轮/SUA 运行配置 (ItemRunConf JSON) 用的 Ext key。
	// callTarget 从 ItemConfig.EvalTargetConf.RunConf 序列化写入, SandboxAgent 算子读出组 case-file。
	TargetExecuteExtRunConfKey = "builtin_run_conf"
	// TargetExecuteExtRunModeConfigKey 执行期透传实验级跑法配置 (RunModeConfig JSON: run_mode/sua_mode/
	// sua_model_id/max_run_minutes) 用的 Ext key。SandboxAgent 算子读出填 case-file experiment_info。
	TargetExecuteExtRunModeConfigKey = "builtin_run_mode_config"
)

const (
	InsightAnalysisNotifyCardID = "AAq9DvIYd2qHu"

	ExptEventNotifyCardID = "AAqvJsfSSLQtN"

	ExptEventNotifyTitle           = "title"
	ExptEventNotifyTitleSuccess    = "已成功执行"
	ExptEventNotifyTitleFailed     = "执行失败"
	ExptEventNotifyTitleTerminated = "执行已被终止"
	ExptEventNotifyTitleStarting   = "开始执行"

	ExptEventNotifyTitleColor           = "title_color"
	ExptEventNotifyTitleColorSuccess    = "green"
	ExptEventNotifyTitleColorFailed     = "red"
	ExptEventNotifyTitleColorTerminated = "orange"
	ExptEventNotifyTitleColorStarting   = "yellow"

	ExptEventNotifyTerminatedCardID = "AAq2fx2rVilOw"
	ExptEventNotifyTerminatedUser   = "terminated_user"
)

// 沙箱 agent 实验专用飞书卡片: 每 1h 进度快照 + 每行失败即时通知。
// 卡片模板 ID 需在飞书开放平台创建后填入; 空串时 Notifier 会静默跳过发送。
const (
	// SandboxAgentProgressNotifyCardID 沙箱 agent 实验每 1h 进度快照卡。
	SandboxAgentProgressNotifyCardID = "AAqPubiuyeOmy"
	// SandboxAgentItemFailNotifyCardID 沙箱 agent 实验单行失败即时通知卡。
	SandboxAgentItemFailNotifyCardID = "AAqPubXfnS7nr"

	// 进度卡字段 key (跟卡片 template 对齐, 待 template 创建后可能调整)
	SandboxAgentProgressKeyExptName      = "expt_name"
	SandboxAgentProgressKeyExptID        = "expt_id"
	SandboxAgentProgressKeySpaceID       = "space_id"
	SandboxAgentProgressKeySuccessCnt    = "success_cnt"
	SandboxAgentProgressKeyFailCnt       = "fail_cnt"
	SandboxAgentProgressKeyProcessingCnt = "processing_cnt"
	SandboxAgentProgressKeyPendingCnt    = "pending_cnt"
	SandboxAgentProgressKeyTerminatedCnt = "terminated_cnt"
	SandboxAgentProgressKeyTotalCnt      = "total_cnt"

	// 失败卡字段 key
	SandboxAgentItemFailKeyExptName = "expt_name"
	SandboxAgentItemFailKeyExptID   = "expt_id"
	SandboxAgentItemFailKeyItemID   = "item_id"
	SandboxAgentItemFailKeySpaceID  = "space_id"
	SandboxAgentItemFailKeyErrMsg   = "err_msg"
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
