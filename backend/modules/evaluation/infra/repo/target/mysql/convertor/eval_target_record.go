// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"fmt"
	"time"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

func EvalTargetRecordDO2PO(e *entity.EvalTargetRecord) (*model.TargetRecord, error) {
	if e == nil {
		return nil, nil
	}

	// 处理输入数据序列化
	var inputData []byte
	if e.EvalTargetInputData != nil {
		data, err := json.Marshal(e.EvalTargetInputData)
		if err != nil {
			return nil, fmt.Errorf("marshal EvalTargetInputData failed: %w", err)
		}
		inputData = data
	}

	// 处理输出数据序列化
	var outputData []byte
	if e.EvalTargetOutputData != nil {
		data, err := json.Marshal(e.EvalTargetOutputData)
		if err != nil {
			return nil, fmt.Errorf("marshal EvalTargetOutputData failed: %w", err)
		}
		outputData = data
	}

	// 处理状态转换（包含nil指针处理）
	var status int32
	if e.Status != nil {
		status = int32(*e.Status)
	} else {
		status = int32(entity.EvalTargetRunStatusUnknown)
	}

	return &model.TargetRecord{
		ID:              e.ID,
		SpaceID:         e.SpaceID,
		TargetID:        e.TargetID,
		TargetVersionID: e.TargetVersionID,
		ExperimentRunID: e.ExperimentRunID,
		ItemID:          e.ItemID,
		TurnID:          e.TurnID,
		LogID:           e.LogID,
		TraceID:         e.TraceID,
		InputData:       &inputData,
		OutputData:      &outputData,
		Status:          status,
		CreatedAt:       time.UnixMilli(gptr.Indirect(e.BaseInfo.CreatedAt)),
		UpdatedAt:       time.UnixMilli(gptr.Indirect(e.BaseInfo.UpdatedAt)),
	}, nil
}

func EvalTargetRecordPO2DO(m *model.TargetRecord) (*entity.EvalTargetRecord, error) {
	if m == nil {
		return nil, nil
	}

	// 处理输入数据反序列化
	var targetInputData *entity.EvalTargetInputData
	if m.InputData != nil && len(*m.InputData) > 0 {
		var input entity.EvalTargetInputData
		if err := json.Unmarshal(*m.InputData, &input); err != nil {
			return nil, fmt.Errorf("unmarshal InputData failed: %w", err)
		}
		targetInputData = &input
	}

	// 处理输出数据反序列化
	var targetOutputData *entity.EvalTargetOutputData
	if m.OutputData != nil && len(*m.OutputData) > 0 {
		var output entity.EvalTargetOutputData
		if err := json.Unmarshal(*m.OutputData, &output); err != nil {
			return nil, fmt.Errorf("unmarshal OutputData failed: %w", err)
		}
		targetOutputData = &output
		if targetOutputData != nil && targetOutputData.EvalTargetUsage != nil && targetOutputData.EvalTargetUsage.TotalTokens == 0 {
			targetOutputData.EvalTargetUsage.TotalTokens = targetOutputData.EvalTargetUsage.InputTokens + targetOutputData.EvalTargetUsage.OutputTokens
		}
	}

	// 状态类型转换
	status := entity.EvalTargetRunStatus(m.Status)

	return &entity.EvalTargetRecord{
		ID:                   m.ID,
		SpaceID:              m.SpaceID,
		TargetID:             m.TargetID,
		TargetVersionID:      m.TargetVersionID,
		ExperimentRunID:      m.ExperimentRunID,
		ItemID:               m.ItemID,
		TurnID:               m.TurnID,
		LogID:                m.LogID,
		TraceID:              m.TraceID,
		EvalTargetInputData:  targetInputData,
		EvalTargetOutputData: targetOutputData,
		Status:               &status,
		BaseInfo: &entity.BaseInfo{
			CreatedAt: gptr.Of(m.CreatedAt.UnixMilli()),
			UpdatedAt: gptr.Of(m.UpdatedAt.UnixMilli()),
		},
	}, nil
}
