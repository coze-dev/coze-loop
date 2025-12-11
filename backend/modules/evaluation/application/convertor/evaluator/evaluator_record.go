// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"github.com/bytedance/gg/gptr"

	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	commonconvertor "github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/common"
	evaluatordo "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func ConvertEvaluatorRecordDTO2DO(dto *evaluatordto.EvaluatorRecord) *evaluatordo.EvaluatorRecord {
	if dto == nil {
		return nil
	}
	do := &evaluatordo.EvaluatorRecord{
		ID:                  gptr.Indirect(dto.ID),
		ExperimentID:        gptr.Indirect(dto.ExperimentID),
		ExperimentRunID:     gptr.Indirect(dto.ExperimentRunID),
		ItemID:              gptr.Indirect(dto.ItemID),
		TurnID:              gptr.Indirect(dto.TurnID),
		EvaluatorVersionID:  gptr.Indirect(dto.EvaluatorVersionID),
		TraceID:             gptr.Indirect(dto.TraceID),
		LogID:               gptr.Indirect(dto.LogID),
		EvaluatorInputData:  ConvertEvaluatorInputDataDTO2DO(dto.EvaluatorInputData),
		EvaluatorOutputData: ConvertEvaluatorOutputDataDTO2DO(dto.EvaluatorOutputData),
		Status:              evaluatordo.EvaluatorRunStatus(dto.GetStatus()),
		BaseInfo:            commonconvertor.ConvertBaseInfoDTO2DO(dto.BaseInfo),
	}
	// 填充 ext 字段
	if len(dto.Ext) > 0 {
		do.Ext = dto.Ext
	}
	return do
}

func ConvertEvaluatorRecordDO2DTO(do *evaluatordo.EvaluatorRecord) *evaluatordto.EvaluatorRecord {
	if do == nil {
		return nil
	}
	dto := &evaluatordto.EvaluatorRecord{
		ID:                  gptr.Of(do.ID),
		ExperimentID:        gptr.Of(do.ExperimentID),
		ExperimentRunID:     gptr.Of(do.ExperimentRunID),
		ItemID:              gptr.Of(do.ItemID),
		TurnID:              gptr.Of(do.TurnID),
		EvaluatorVersionID:  gptr.Of(do.EvaluatorVersionID),
		TraceID:             gptr.Of(do.TraceID),
		LogID:               gptr.Of(do.LogID),
		EvaluatorInputData:  ConvertEvaluatorInputDataDO2DTO(do.EvaluatorInputData),
		EvaluatorOutputData: ConvertEvaluatorOutputDataDO2DTO(do.EvaluatorOutputData),
		Status:              evaluatordto.EvaluatorRunStatusPtr(evaluatordto.EvaluatorRunStatus(do.Status)),
		BaseInfo:            commonconvertor.ConvertBaseInfoDO2DTO(do.BaseInfo),
	}
	// 填充 ext 字段，使用 evaluator_record 表里的 ext
	if len(do.Ext) > 0 {
		dto.Ext = do.Ext
	}
	return dto
}
