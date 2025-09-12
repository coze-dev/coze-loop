// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"github.com/bytedance/gg/gptr"

	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	evaluatorentity "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// ConvertEvaluatorOutputDataDTO2DO 将 DTO 转换为 evaluatorentity.EvaluatorOutputData 结构体
func ConvertEvaluatorOutputDataDTO2DO(dto *evaluatordto.EvaluatorOutputData) *evaluatorentity.EvaluatorOutputData {
	if dto == nil {
		return nil
	}
	return &evaluatorentity.EvaluatorOutputData{
		EvaluatorResult:   ConvertEvaluatorResultDTO2DO(dto.EvaluatorResult_),
		EvaluatorUsage:    ConvertEvaluatorUsageDTO2DO(dto.EvaluatorUsage),
		EvaluatorRunError: ConvertEvaluatorRunErrorDTO2DO(dto.EvaluatorRunError),
		TimeConsumingMS:   dto.GetTimeConsumingMs(),
		Stdout:            dto.GetStdout(),
	}
}

// ConvertEvaluatorOutputDataDO2DTO 将 evaluatorentity.EvaluatorOutputData 结构体转换为 DTO
func ConvertEvaluatorOutputDataDO2DTO(do *evaluatorentity.EvaluatorOutputData) *evaluatordto.EvaluatorOutputData {
	if do == nil {
		return nil
	}
	return &evaluatordto.EvaluatorOutputData{
		EvaluatorResult_:  ConvertEvaluatorResultDO2DTO(do.EvaluatorResult),
		EvaluatorUsage:    ConvertEvaluatorUsageDO2DTO(do.EvaluatorUsage),
		EvaluatorRunError: ConvertEvaluatorRunErrorDO2DTO(do.EvaluatorRunError),
		TimeConsumingMs:   gptr.Of(do.TimeConsumingMS),
		Stdout:            gptr.Of(do.Stdout),
	}
}

// ConvertCorrectionDTO2DO 将 DTO 转换为 evaluatorentity.Correction 结构体
func ConvertCorrectionDTO2DO(dto *evaluatordto.Correction) *evaluatorentity.Correction {
	if dto == nil {
		return nil
	}
	return &evaluatorentity.Correction{
		Score:     dto.Score,
		Explain:   dto.GetExplain(),
		UpdatedBy: dto.GetUpdatedBy(),
	}
}

// ConvertCorrectionDO2DTO 将 evaluatorentity.Correction 结构体转换为 DTO
func ConvertCorrectionDO2DTO(do *evaluatorentity.Correction) *evaluatordto.Correction {
	if do == nil {
		return nil
	}
	return &evaluatordto.Correction{
		Score:     do.Score,
		Explain:   gptr.Of(do.Explain),
		UpdatedBy: gptr.Of(do.UpdatedBy),
	}
}

// ConvertEvaluatorResultDTO2DO 将 DTO 转换为 evaluatorentity.EvaluatorResult 结构体
func ConvertEvaluatorResultDTO2DO(dto *evaluatordto.EvaluatorResult_) *evaluatorentity.EvaluatorResult {
	if dto == nil {
		return nil
	}
	return &evaluatorentity.EvaluatorResult{
		Score:      dto.Score,
		Correction: ConvertCorrectionDTO2DO(dto.Correction),
		Reasoning:  dto.GetReasoning(),
	}
}

// ConvertEvaluatorResultDO2DTO 将 evaluatorentity.EvaluatorResult 结构体转换为 DTO
func ConvertEvaluatorResultDO2DTO(do *evaluatorentity.EvaluatorResult) *evaluatordto.EvaluatorResult_ {
	if do == nil {
		return nil
	}
	return &evaluatordto.EvaluatorResult_{
		Score:      do.Score,
		Correction: ConvertCorrectionDO2DTO(do.Correction),
		Reasoning:  gptr.Of(do.Reasoning),
	}
}

// ConvertEvaluatorUsageDTO2DO 将 DTO 转换为 evaluatorentity.EvaluatorUsage 结构体
func ConvertEvaluatorUsageDTO2DO(dto *evaluatordto.EvaluatorUsage) *evaluatorentity.EvaluatorUsage {
	if dto == nil {
		return nil
	}
	return &evaluatorentity.EvaluatorUsage{
		InputTokens:  dto.GetInputTokens(),
		OutputTokens: dto.GetOutputTokens(),
	}
}

// ConvertEvaluatorUsageDO2DTO 将 evaluatorentity.EvaluatorUsage 结构体转换为 DTO
func ConvertEvaluatorUsageDO2DTO(do *evaluatorentity.EvaluatorUsage) *evaluatordto.EvaluatorUsage {
	if do == nil {
		return nil
	}
	return &evaluatordto.EvaluatorUsage{
		InputTokens:  gptr.Of(do.InputTokens),
		OutputTokens: gptr.Of(do.OutputTokens),
	}
}

// ConvertEvaluatorRunErrorDTO2DO 将 DTO 转换为 evaluatorentity.EvaluatorRunError 结构体
func ConvertEvaluatorRunErrorDTO2DO(dto *evaluatordto.EvaluatorRunError) *evaluatorentity.EvaluatorRunError {
	if dto == nil {
		return nil
	}
	return &evaluatorentity.EvaluatorRunError{
		Code:    dto.GetCode(),
		Message: dto.GetMessage(),
	}
}

// ConvertEvaluatorRunErrorDO2DTO 将 evaluatorentity.EvaluatorRunError 结构体转换为 DTO
func ConvertEvaluatorRunErrorDO2DTO(do *evaluatorentity.EvaluatorRunError) *evaluatordto.EvaluatorRunError {
	if do == nil {
		return nil
	}
	return &evaluatordto.EvaluatorRunError{
		Code:    gptr.Of(do.Code),
		Message: gptr.Of(do.Message),
	}
}