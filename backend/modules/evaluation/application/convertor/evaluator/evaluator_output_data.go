// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"github.com/bytedance/gg/gptr"

	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	evaluatorentity "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
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
		ExtraOutput:       ConvertEvaluatorExtraOutputContentDTO2DO(dto.ExtraOutput),
		EvidenceArchive:   ConvertEvaluatorEvidenceArchiveDTO2DO(dto.EvidenceArchive),
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
		ExtraOutput:       ConvertEvaluatorExtraOutputContentDO2DTO(do.ExtraOutput),
		EvidenceArchive:   ConvertEvaluatorEvidenceArchiveDO2DTO(do.EvidenceArchive),
	}
}

func ConvertEvaluatorEvidenceArchiveDTO2DO(dto *evaluatordto.EvaluatorEvidenceArchive) *evaluatorentity.EvaluatorEvidenceArchive {
	if dto == nil {
		return nil
	}
	return &evaluatorentity.EvaluatorEvidenceArchive{
		SchemaVersion:  dto.GetSchemaVersion(),
		ObjectKey:      dto.GetObjectKey(),
		Status:         dto.GetStatus(),
		Trigger:        dto.GetTrigger(),
		SizeBytes:      dto.GetSizeBytes(),
		SHA256:         dto.GetSha256(),
		TruncatedFiles: dto.GetTruncatedFiles(),
		Error:          dto.GetError(),
	}
}

func ConvertEvaluatorEvidenceArchiveDO2DTO(do *evaluatorentity.EvaluatorEvidenceArchive) *evaluatordto.EvaluatorEvidenceArchive {
	if do == nil {
		return nil
	}
	dto := &evaluatordto.EvaluatorEvidenceArchive{
		SchemaVersion:  gptr.Of(do.SchemaVersion),
		ObjectKey:      gptr.Of(do.ObjectKey),
		Status:         gptr.Of(do.Status),
		Trigger:        gptr.Of(do.Trigger),
		SizeBytes:      gptr.Of(do.SizeBytes),
		Sha256:         gptr.Of(do.SHA256),
		TruncatedFiles: gptr.Of(do.TruncatedFiles),
		Error:          gptr.Of(do.Error),
	}
	if do.FornaxEvaluatorLogURL != "" {
		dto.FornaxEvaluatorLogURL = gptr.Of(do.FornaxEvaluatorLogURL)
	}
	return dto
}

// ConvertEvaluatorExtraOutputContentDTO2DO 将 DTO 转换为 DO
func ConvertEvaluatorExtraOutputContentDTO2DO(dto *evaluatordto.EvaluatorExtraOutputContent) *evaluatorentity.EvaluatorExtraOutputContent {
	if dto == nil {
		return nil
	}
	result := &evaluatorentity.EvaluatorExtraOutputContent{
		URI: dto.URI,
		URL: dto.URL,
	}
	if dto.OutputType != nil {
		outputType := evaluatorentity.EvaluatorExtraOutputType(*dto.OutputType)
		result.OutputType = &outputType
	}
	return result
}

// ConvertEvaluatorExtraOutputContentDO2DTO 将 DO 转换为 DTO
func ConvertEvaluatorExtraOutputContentDO2DTO(do *evaluatorentity.EvaluatorExtraOutputContent) *evaluatordto.EvaluatorExtraOutputContent {
	if do == nil {
		return nil
	}
	result := &evaluatordto.EvaluatorExtraOutputContent{
		URI: do.URI,
		URL: do.URL,
	}
	if do.OutputType != nil {
		outputType := evaluatordto.EvaluatorExtraOutputType(*do.OutputType)
		result.OutputType = &outputType
	}
	return result
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

func ToEvaluatorRunStatusDO(status spi.InvokeEvaluatorRunStatus) evaluatorentity.EvaluatorRunStatus {
	switch status {
	case spi.InvokeEvaluatorRunStatus_FAILED:
		return evaluatorentity.EvaluatorRunStatusFail
	case spi.InvokeEvaluatorRunStatus_SUCCESS:
		return evaluatorentity.EvaluatorRunStatusSuccess
	default:
		return evaluatorentity.EvaluatorRunStatusUnknown
	}
}

func ToInvokeEvaluatorOutputDataDO(outputData *spi.InvokeEvaluatorOutputData, status spi.InvokeEvaluatorRunStatus) *evaluatorentity.EvaluatorOutputData {
	if outputData == nil {
		return nil
	}

	switch status {
	case spi.InvokeEvaluatorRunStatus_SUCCESS:
		return &evaluatorentity.EvaluatorOutputData{
			EvaluatorResult:   toInvokeEvaluatorResultDO(outputData.EvaluatorResult_),
			EvaluatorUsage:    toInvokeEvaluatorUsageDO(outputData.EvaluatorUsage),
			EvaluatorRunError: nil,
			ExtraOutput:       toInvokeEvaluatorExtraOutputDO(outputData.ExtraOutput),
			EvidenceArchive:   toInvokeEvaluatorEvidenceArchiveDO(outputData.EvidenceArchive),
		}
	case spi.InvokeEvaluatorRunStatus_FAILED:
		return &evaluatorentity.EvaluatorOutputData{
			EvaluatorResult:   nil,
			EvaluatorUsage:    toInvokeEvaluatorUsageDO(outputData.EvaluatorUsage),
			EvaluatorRunError: toInvokeEvaluatorRunErrorDO(outputData.EvaluatorRunError),
			ExtraOutput:       toInvokeEvaluatorExtraOutputDO(outputData.ExtraOutput),
			EvidenceArchive:   toInvokeEvaluatorEvidenceArchiveDO(outputData.EvidenceArchive),
		}
	default:
		return nil
	}
}

func toInvokeEvaluatorResultDO(result *spi.InvokeEvaluatorResult_) *evaluatorentity.EvaluatorResult {
	if result == nil {
		return nil
	}
	return &evaluatorentity.EvaluatorResult{
		Score:     result.Score,
		Reasoning: result.GetReasoning(),
	}
}

func toInvokeEvaluatorUsageDO(usage *spi.InvokeEvaluatorUsage) *evaluatorentity.EvaluatorUsage {
	if usage == nil {
		return nil
	}
	return &evaluatorentity.EvaluatorUsage{
		InputTokens:  usage.GetInputTokens(),
		OutputTokens: usage.GetOutputTokens(),
	}
}

func toInvokeEvaluatorRunErrorDO(runError *spi.InvokeEvaluatorRunError) *evaluatorentity.EvaluatorRunError {
	if runError == nil {
		return &evaluatorentity.EvaluatorRunError{
			Code:    errno.RunEvaluatorFailCode,
			Message: "unknown error",
		}
	}
	return &evaluatorentity.EvaluatorRunError{
		Code:    runError.GetCode(),
		Message: runError.GetMessage(),
	}
}

func toInvokeEvaluatorExtraOutputDO(extraOutput *spi.EvaluatorExtraOutputContent) *evaluatorentity.EvaluatorExtraOutputContent {
	if extraOutput == nil {
		return nil
	}
	result := &evaluatorentity.EvaluatorExtraOutputContent{}
	if extraOutput.OutputType != nil {
		outputType := evaluatorentity.EvaluatorExtraOutputType(*extraOutput.OutputType)
		result.OutputType = &outputType
	}
	result.URI = extraOutput.URI
	result.URL = extraOutput.URL
	return result
}

func toInvokeEvaluatorEvidenceArchiveDO(archive *spi.EvaluatorEvidenceArchive) *evaluatorentity.EvaluatorEvidenceArchive {
	if archive == nil {
		return nil
	}
	return &evaluatorentity.EvaluatorEvidenceArchive{
		SchemaVersion:  archive.GetSchemaVersion(),
		ObjectKey:      archive.GetObjectKey(),
		Status:         archive.GetStatus(),
		Trigger:        archive.GetTrigger(),
		SizeBytes:      archive.GetSizeBytes(),
		SHA256:         archive.GetSha256(),
		TruncatedFiles: archive.GetTruncatedFiles(),
		Error:          archive.GetError(),
	}
}
