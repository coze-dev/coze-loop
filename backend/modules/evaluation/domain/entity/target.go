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

	CozeBot      *CozeBot
	Prompt       *LoopPrompt
	CozeWorkflow *CozeWorkflow

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
)

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
	}
	return "<UNSET>"
}

func EvalTargetTypePtr(v EvalTargetType) *EvalTargetType { return &v }
func (p *EvalTargetType) Scan(value interface{}) (err error) {
	var result sql.NullInt64
	err = result.Scan(value)
	*p = EvalTargetType(result.Int64)
	return
}

func (p *EvalTargetType) Value() (driver.Value, error) {
	if p == nil {
		return nil, nil
	}
	return int64(*p), nil
}
