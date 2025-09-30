// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	commondto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	commonconvertor "github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/common"
	evaluatorentity "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// ConvertEvaluatorInputDataDTO2DO 将 DTO 转换为 evaluatorentity.EvaluatorInputData 结构体
func ConvertEvaluatorInputDataDTO2DO(dto *evaluatordto.EvaluatorInputData) *evaluatorentity.EvaluatorInputData {
	if dto == nil {
		return nil
	}

	// 转换 HistoryMessages
	historyMessages := make([]*evaluatorentity.Message, 0, len(dto.HistoryMessages))
	for _, msgDTO := range dto.HistoryMessages {
		msgDO := commonconvertor.ConvertMessageDTO2DO(msgDTO)
		historyMessages = append(historyMessages, msgDO)
	}

	// 转换 InputFields
	inputFields := make(map[string]*evaluatorentity.Content)
	for key, contentDTO := range dto.InputFields {
		contentDO := commonconvertor.ConvertContentDTO2DO(contentDTO)
		inputFields[key] = contentDO
	}

	// 转换 EvaluateDatasetFields
	evaluateDatasetFields := make(map[string]*evaluatorentity.Content)
	for key, contentDTO := range dto.EvaluateDatasetFields {
		contentDO := commonconvertor.ConvertContentDTO2DO(contentDTO)
		evaluateDatasetFields[key] = contentDO
	}

	// 转换 EvaluateTargetOutputFields
	evaluateTargetOutputFields := make(map[string]*evaluatorentity.Content)
	for key, contentDTO := range dto.EvaluateTargetOutputFields {
		contentDO := commonconvertor.ConvertContentDTO2DO(contentDTO)
		evaluateTargetOutputFields[key] = contentDO
	}

	return &evaluatorentity.EvaluatorInputData{
		HistoryMessages:            historyMessages,
		InputFields:                inputFields,
		EvaluateDatasetFields:      evaluateDatasetFields,
		EvaluateTargetOutputFields: evaluateTargetOutputFields,
		Ext:                        dto.Ext,
	}
}

// ConvertEvaluatorInputDataDO2DTO 将 evaluatorentity.EvaluatorInputData 结构体转换为 DTO
func ConvertEvaluatorInputDataDO2DTO(do *evaluatorentity.EvaluatorInputData) *evaluatordto.EvaluatorInputData {
	if do == nil {
		return nil
	}

	// 转换 HistoryMessages
	historyMessages := make([]*commondto.Message, 0, len(do.HistoryMessages))
	for _, msgDO := range do.HistoryMessages {
		msgDTO := commonconvertor.ConvertMessageDO2DTO(msgDO)
		historyMessages = append(historyMessages, msgDTO)
	}

	// 转换 InputFields
	inputFields := make(map[string]*commondto.Content)
	for key, contentDO := range do.InputFields {
		contentDTO := commonconvertor.ConvertContentDO2DTO(contentDO)
		inputFields[key] = contentDTO
	}

	// 转换 EvaluateDatasetFields
	evaluateDatasetFields := make(map[string]*commondto.Content)
	for key, contentDO := range do.EvaluateDatasetFields {
		contentDTO := commonconvertor.ConvertContentDO2DTO(contentDO)
		evaluateDatasetFields[key] = contentDTO
	}

	// 转换 EvaluateTargetOutputFields
	evaluateTargetOutputFields := make(map[string]*commondto.Content)
	for key, contentDO := range do.EvaluateTargetOutputFields {
		contentDTO := commonconvertor.ConvertContentDO2DTO(contentDO)
		evaluateTargetOutputFields[key] = contentDTO
	}

	return &evaluatordto.EvaluatorInputData{
		HistoryMessages:            historyMessages,
		InputFields:                inputFields,
		EvaluateDatasetFields:      evaluateDatasetFields,
		EvaluateTargetOutputFields: evaluateTargetOutputFields,
		Ext:                        do.Ext,
	}
}
