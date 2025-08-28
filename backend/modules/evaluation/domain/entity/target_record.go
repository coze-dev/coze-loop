// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"errors"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/json"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"

	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

type EvalTargetRecord struct {
	// 评估记录ID
	ID int64
	// 空间ID
	SpaceID         int64
	TargetID        int64
	TargetVersionID int64
	// 实验执行ID
	ExperimentRunID int64
	// 评测集数据项ID
	ItemID int64
	// 评测集数据项轮次ID
	TurnID int64
	// 链路ID
	TraceID string
	// 链路ID
	LogID string
	// 输入数据
	EvalTargetInputData *EvalTargetInputData
	// 输出数据
	EvalTargetOutputData *EvalTargetOutputData
	Status               *EvalTargetRunStatus

	BaseInfo *BaseInfo
}

type EvalTargetInputData struct {
	// 历史会话记录
	HistoryMessages []*Message
	// 变量
	InputFields map[string]*Content
	Ext         map[string]string
}

// ValidateInputSchema  common valiate input schema
func (e *EvalTargetInputData) ValidateInputSchema(inputSchema []*ArgsSchema) error {
	for fieldKey, content := range e.InputFields {
		if content == nil {
			continue
			// return errno.Wrapf(errors.NewByCode(""), "field %s is required", fieldKey)
		}
		schemaMap := make(map[string]*ArgsSchema)
		for _, schema := range inputSchema {
			schemaMap[gptr.Indirect(schema.Key)] = schema
		}
		// schema中不存在的字段无需校验
		if argsSchema, ok := schemaMap[fieldKey]; ok {
			contentType := content.ContentType
			if contentType == nil {
				return errorx.Wrapf(errors.New(""), "field %s content type is nil", fieldKey)
			}
			if !gslice.Contains(argsSchema.SupportContentTypes, gptr.Indirect(contentType)) {
				return errorx.Wrapf(errors.New(""), "field %s content type %v not support", fieldKey, content.ContentType)
			}
			if *contentType == ContentTypeText {
				valid, err := json.ValidateJSONSchema(*argsSchema.JsonSchema, content.GetText())
				if err != nil {
					return err
				}
				if !valid {
					return errorx.Wrapf(errors.New(""), "field %s content not valid", fieldKey)
				}
			}
		}
	}
	return nil
}

type EvalTargetOutputData struct {
	// 变量
	OutputFields map[string]*Content
	// 运行消耗
	EvalTargetUsage *EvalTargetUsage
	// 运行报错
	EvalTargetRunError *EvalTargetRunError
	// 运行耗时
	TimeConsumingMS *int64
}

type EvalTargetUsage struct {
	InputTokens  int64
	OutputTokens int64
}

type EvalTargetRunError struct {
	Code    int32
	Message string
}

type EvalTargetRunStatus int64

const (
	EvalTargetRunStatusUnknown EvalTargetRunStatus = 0
	EvalTargetRunStatusSuccess EvalTargetRunStatus = 1
	EvalTargetRunStatusFail    EvalTargetRunStatus = 2
)

type ExecuteTargetCtx struct {
	// 实验执行ID
	ExperimentRunID *int64
	// 评测集数据项ID
	ItemID int64
	// 评测集数据项轮次ID
	TurnID int64
}
