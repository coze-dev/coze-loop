// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"errors"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/json"
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
				return errorx.New("field %s content type %v not support", fieldKey, content.ContentType)
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
	TotalTokens  int64
}

func (e *EvalTargetUsage) GetInputTokens() int64 {
	if e != nil {
		return e.InputTokens
	}
	return 0
}

func (e *EvalTargetUsage) GetOutputTokens() int64 {
	if e != nil {
		return e.OutputTokens
	}
	return 0
}

func (e *EvalTargetUsage) GetTotalTokens() int64 {
	if e != nil {
		return e.TotalTokens
	}
	return 0
}

type EvalTargetRunError struct {
	Code    int32
	Message string
}

type EvalTargetRunStatus int64

const (
	EvalTargetRunStatusUnknown       EvalTargetRunStatus = 0
	EvalTargetRunStatusSuccess       EvalTargetRunStatus = 1
	EvalTargetRunStatusFail          EvalTargetRunStatus = 2
	EvalTargetRunStatusAsyncInvoking EvalTargetRunStatus = 3
)

type ExecuteTargetCtx struct {
	// 实验执行ID
	ExperimentRunID *int64
	// 评测集数据项ID
	ItemID int64
	// 评测集数据项轮次ID
	TurnID int64
}

type TargetTrajectoryConf struct {
	ExtractIntervalSecond      int64           `json:"extract_interval_second" mapstructure:"extract_interval_second"`
	SpaceExtractIntervalSecond map[int64]int64 `json:"space_extract_interval_second" mapstructure:"space_extract_interval_second"`
}

func (t *TargetTrajectoryConf) GetExtractInterval(spaceID int64) time.Duration {
	const defaultInterval = time.Second * 15
	if t == nil {
		return defaultInterval
	}
	if interval := t.SpaceExtractIntervalSecond[spaceID]; spaceID > 0 && interval > 0 {
		return time.Duration(interval) * time.Second
	}
	if t.ExtractIntervalSecond > 0 {
		return time.Duration(t.ExtractIntervalSecond) * time.Second
	}
	return defaultInterval
}
