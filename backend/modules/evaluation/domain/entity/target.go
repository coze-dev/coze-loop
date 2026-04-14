// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"database/sql"
	"database/sql/driver"
)

type EvalTarget struct {
	ID                int64
	SpaceID           int64
	SourceTargetID    string
	EvalTargetType    EvalTargetType
	EvalTargetVersion *EvalTargetVersion
	BaseInfo          *BaseInfo
}

type EvalTargetVersion struct {
	ID                  int64
	SpaceID             int64
	TargetID            int64
	SourceTargetVersion string

	EvalTargetType EvalTargetType

	CozeBot         *CozeBot
	Prompt          *LoopPrompt
	CozeWorkflow    *CozeWorkflow
	VolcengineAgent *VolcengineAgent
	CustomRPCServer *CustomRPCServer
	WebAgent        *WebAgent

	InputSchema      []*ArgsSchema
	OutputSchema     []*ArgsSchema
	RuntimeParamDemo *string

	BaseInfo *BaseInfo
}

type EvalTargetType int64

const (
	// CozeBot
	EvalTargetTypeCozeBot EvalTargetType = 1
	// Prompt
	EvalTargetTypeLoopPrompt EvalTargetType = 2
	// Trace
	EvalTargetTypeLoopTrace EvalTargetType = 3
	// CozeWorkflow
	EvalTargetTypeCozeWorkflow EvalTargetType = 4
	// 火山智能体
	EvalTargetTypeVolcengineAgent EvalTargetType = 5
	// 自定义服务 for内场
	EvalTargetTypeCustomRPCServer EvalTargetType = 6

	// 火山智能体Agentkit
	EvalTargetTypeVolcengineAgentAgentkit EvalTargetType = 7
	// Web智能体
	EvalTargetTypeWebAgent EvalTargetType = 8

	// 以下为仅记录型：评测过程中不执行对象，仅用于记录对象类型和基本信息
	EvalTargetTypeCozeBotOnline                 EvalTargetType = 11
	EvalTargetTypeCozeLoopPromptOnline          EvalTargetType = 12
	EvalTargetTypeCozeWorkflowOnline            EvalTargetType = 13
	EvalTargetTypeVolcengineAgentOnline         EvalTargetType = 14
	EvalTargetTypeCustomRPCServerOnline         EvalTargetType = 15
	EvalTargetTypeVolcengineAgentAgentkitOnline EvalTargetType = 16
)

// NeedExecuteTarget 是否需要执行评测对象。仅记录型（*Online）不需要执行，仅用于记录对象类型和基本信息
func (p EvalTargetType) NeedExecuteTarget() bool {
	return !p.IsRecordOnlyType()
}

// IsRecordOnlyType 是否为仅记录型（评测过程中不执行对象，仅用于记录对象类型和基本信息）
func (p EvalTargetType) IsRecordOnlyType() bool {
	switch p {
	case EvalTargetTypeCozeBotOnline, EvalTargetTypeCozeLoopPromptOnline, EvalTargetTypeCozeWorkflowOnline,
		EvalTargetTypeVolcengineAgentOnline, EvalTargetTypeCustomRPCServerOnline, EvalTargetTypeVolcengineAgentAgentkitOnline:
		return true
	default:
		return false
	}
}

// RecordOnlyTypeToBaseType 仅记录型映射到对应的基础类型（用于 CreateEvalTarget 复用 base 的 operator）
func (p EvalTargetType) RecordOnlyTypeToBaseType() (EvalTargetType, bool) {
	switch p {
	case EvalTargetTypeCozeBotOnline:
		return EvalTargetTypeCozeBot, true
	case EvalTargetTypeCozeLoopPromptOnline:
		return EvalTargetTypeLoopPrompt, true
	case EvalTargetTypeCozeWorkflowOnline:
		return EvalTargetTypeCozeWorkflow, true
	case EvalTargetTypeVolcengineAgentOnline:
		return EvalTargetTypeVolcengineAgent, true
	case EvalTargetTypeCustomRPCServerOnline:
		return EvalTargetTypeCustomRPCServer, true
	case EvalTargetTypeVolcengineAgentAgentkitOnline:
		return EvalTargetTypeVolcengineAgentAgentkit, true
	default:
		return 0, false
	}
}

// ToOperatorBaseType 拼装源信息（PackSource*）及与 typedOperators 对齐分支时使用：仅记录型映射为对应基础类型，否则原样返回。
func (p EvalTargetType) ToOperatorBaseType() EvalTargetType {
	if b, ok := p.RecordOnlyTypeToBaseType(); ok {
		return b
	}
	return p
}

// BaseTypeToRecordOnlyType 基础类型映射到在线实验/模板在库中存储的仅记录型（与 RecordOnlyTypeToBaseType 互逆）
func (p EvalTargetType) BaseTypeToRecordOnlyType() (EvalTargetType, bool) {
	switch p {
	case EvalTargetTypeCozeBot:
		return EvalTargetTypeCozeBotOnline, true
	case EvalTargetTypeLoopPrompt:
		return EvalTargetTypeCozeLoopPromptOnline, true
	case EvalTargetTypeCozeWorkflow:
		return EvalTargetTypeCozeWorkflowOnline, true
	case EvalTargetTypeVolcengineAgent:
		return EvalTargetTypeVolcengineAgentOnline, true
	case EvalTargetTypeCustomRPCServer:
		return EvalTargetTypeCustomRPCServerOnline, true
	case EvalTargetTypeVolcengineAgentAgentkit:
		return EvalTargetTypeVolcengineAgentAgentkitOnline, true
	default:
		return 0, false
	}
}

func (p EvalTargetType) String() string {
	switch p {
	case EvalTargetTypeCozeBot:
		return "CozeBot"
	case EvalTargetTypeLoopPrompt:
		return "LoopPrompt"
	case EvalTargetTypeLoopTrace:
		return "LoopTrace"
	case EvalTargetTypeCozeWorkflow:
		return "CozeWorkflow"
	case EvalTargetTypeVolcengineAgent:
		return "VolcengineAgent"
	case EvalTargetTypeCustomRPCServer:
		return "CustomRPCServer"
	case EvalTargetTypeVolcengineAgentAgentkit:
		return "VolcengineAgentKit"
	case EvalTargetTypeWebAgent:
		return "WebAgent"
	case EvalTargetTypeCozeBotOnline:
		return "CozeBotOnline"
	case EvalTargetTypeCozeLoopPromptOnline:
		return "CozeLoopPromptOnline"
	case EvalTargetTypeCozeWorkflowOnline:
		return "CozeWorkflowOnline"
	case EvalTargetTypeVolcengineAgentOnline:
		return "VolcengineAgentOnline"
	case EvalTargetTypeCustomRPCServerOnline:
		return "CustomRPCServerOnline"
	case EvalTargetTypeVolcengineAgentAgentkitOnline:
		return "VolcengineAgentAgentkitOnline"
	}
	return "<UNSET>"
}

func (p EvalTargetType) SupptTrajectory() bool {
	switch p {
	case EvalTargetTypeVolcengineAgent, EvalTargetTypeCustomRPCServer, EvalTargetTypeLoopPrompt:
		return true
	default:
		return false
	}
}

func EvalTargetTypePtr(v EvalTargetType) *EvalTargetType { return &v }

func (p *EvalTargetType) Scan(value interface{}) (err error) {
	var result sql.NullInt64
	err = result.Scan(value)
	*p = EvalTargetType(result.Int64)
	return err
}

func (p *EvalTargetType) Value() (driver.Value, error) {
	if p == nil {
		return nil, nil
	}
	return int64(*p), nil
}
