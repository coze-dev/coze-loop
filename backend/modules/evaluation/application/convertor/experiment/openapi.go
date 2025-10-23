// Copyright (c) 2025 Bytedance Ltd. and/or its affiliates
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"fmt"
	"strconv"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"

	"github.com/bytedance/gg/gptr"

	openapiCommon "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/common"
	openapiEvalTarget "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/eval_target"
	openapiEvaluator "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/evaluator"
	openapiExperiment "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"
	openapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"

	domainCommon "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	domaindoEvalTarget "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	domainEvaluator "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	domainExpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	domainEvalTarget "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_target"
)

// ---------- Request Converters ----------

func OpenAPITargetFieldMappingDTO2Domain(mapping *openapiExperiment.TargetFieldMapping) *domainExpt.TargetFieldMapping {
	if mapping == nil {
		return nil
	}

	result := &domainExpt.TargetFieldMapping{}
	for _, fm := range mapping.FromEvalSet {
		if fm == nil {
			continue
		}
		result.FromEvalSet = append(result.FromEvalSet, &domainExpt.FieldMapping{
			FieldName:     fm.FieldName,
			FromFieldName: fm.FromFieldName,
		})
	}
	return result
}

func OpenAPIEvaluatorFieldMappingDTO2Domain(mappings []*openapiExperiment.EvaluatorFieldMapping) []*domainExpt.EvaluatorFieldMapping {
	if len(mappings) == 0 {
		return nil
	}

	result := make([]*domainExpt.EvaluatorFieldMapping, 0, len(mappings))
	for _, mapping := range mappings {
		if mapping == nil {
			continue
		}
		domainMapping := &domainExpt.EvaluatorFieldMapping{
			EvaluatorVersionID: mapping.GetEvaluatorVersionID(),
		}
		for _, fromEval := range mapping.FromEvalSet {
			if fromEval == nil {
				continue
			}
			domainMapping.FromEvalSet = append(domainMapping.FromEvalSet, &domainExpt.FieldMapping{
				FieldName:     fromEval.FieldName,
				FromFieldName: fromEval.FromFieldName,
			})
		}
		for _, fromTarget := range mapping.FromTarget {
			if fromTarget == nil {
				continue
			}
			domainMapping.FromTarget = append(domainMapping.FromTarget, &domainExpt.FieldMapping{
				FieldName:     fromTarget.FieldName,
				FromFieldName: fromTarget.FromFieldName,
			})
		}
		result = append(result, domainMapping)
	}
	return result
}

func OpenAPIRuntimeParamDTO2Domain(param *openapiCommon.RuntimeParam) *domainCommon.RuntimeParam {
	if param == nil {
		return nil
	}
	if param.JSONValue == nil {
		return &domainCommon.RuntimeParam{}
	}
	return &domainCommon.RuntimeParam{JSONValue: param.JSONValue}
}

func OpenAPICreateEvalTargetParamDTO2Domain(param *openapi.SubmitExperimentEvalTargetParam) *domainEvalTarget.CreateEvalTargetParam {
	if param == nil {
		return nil
	}

	result := &domainEvalTarget.CreateEvalTargetParam{
		SourceTargetID:      param.SourceTargetID,
		SourceTargetVersion: param.SourceTargetVersion,
		BotPublishVersion:   param.BotPublishVersion,
	}

	if param.EvalTargetType != nil {
		evalType, err := mapOpenAPIEvalTargetType(*param.EvalTargetType)
		if err != nil {
			return nil
		}
		result.EvalTargetType = &evalType
	}

	if param.BotInfoType != nil {
		botInfoType, err := mapOpenAPICozeBotInfoType(*param.BotInfoType)
		if err != nil {
			return nil
		}
		result.BotInfoType = &botInfoType
	}

	return result
}

func ParseOpenAPIEvaluatorVersions(versions []string) ([]int64, error) {
	if len(versions) == 0 {
		return nil, nil
	}
	ids := make([]int64, 0, len(versions))
	for _, version := range versions {
		id, err := parseStringToInt64(version)
		if err != nil {
			return nil, fmt.Errorf("invalid evaluator version %q: %w", version, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func parseStringToInt64(value string) (int64, error) {
	if value == "" {
		return 0, fmt.Errorf("empty value")
	}
	return strconv.ParseInt(value, 10, 64)
}

func mapOpenAPIEvalTargetType(openapiType openapiEvalTarget.EvalTargetType) (domaindoEvalTarget.EvalTargetType, error) {
	switch openapiType {
	case openapiEvalTarget.EvalTargetTypeCozeBot:
		return domaindoEvalTarget.EvalTargetType_CozeBot, nil
	case openapiEvalTarget.EvalTargetTypeCozeLoopPrompt:
		return domaindoEvalTarget.EvalTargetType_CozeLoopPrompt, nil
	case openapiEvalTarget.EvalTargetTypeTrace:
		return domaindoEvalTarget.EvalTargetType_Trace, nil
	case openapiEvalTarget.EvalTargetTypeCozeWorkflow:
		return domaindoEvalTarget.EvalTargetType_CozeWorkflow, nil
	case openapiEvalTarget.EvalTargetTypeVolcengineAgent:
		return domaindoEvalTarget.EvalTargetType_VolcengineAgent, nil
	default:
		return 0, fmt.Errorf("unsupported eval target type: %s", openapiType)
	}
}

func mapOpenAPICozeBotInfoType(openapiType openapiEvalTarget.CozeBotInfoType) (domaindoEvalTarget.CozeBotInfoType, error) {
	switch openapiType {
	case openapiEvalTarget.CozeBotInfoTypeProductBot:
		return domaindoEvalTarget.CozeBotInfoType_ProductBot, nil
	case openapiEvalTarget.CozeBotInfoTypeDraftBot:
		return domaindoEvalTarget.CozeBotInfoType_DraftBot, nil
	default:
		return 0, fmt.Errorf("unsupported coze bot info type: %s", openapiType)
	}
}

// ---------- Response Converters ----------

func DomainExperimentDTO2OpenAPI(dto *domainExpt.Experiment) *openapiExperiment.Experiment {
	if dto == nil {
		return nil
	}

	result := &openapiExperiment.Experiment{
		ID:                    dto.ID,
		Name:                  dto.Name,
		Description:           dto.Desc,
		ItemConcurNum:         dto.ItemConcurNum,
		TargetFieldMapping:    DomainTargetFieldMappingDTO2OpenAPI(dto.TargetFieldMapping),
		EvaluatorFieldMapping: DomainEvaluatorFieldMappingDTO2OpenAPI(dto.EvaluatorFieldMapping),
		TargetRuntimeParam:    DomainRuntimeParamDTO2OpenAPI(dto.TargetRuntimeParam),
	}

	result.Status = mapExperimentStatus(dto.Status)
	result.StartTime = dto.StartTime
	result.EndTime = dto.EndTime
	result.ExptStats = DomainExperimentStatsDTO2OpenAPI(dto.ExptStats)
	result.BaseInfo = DomainBaseInfoDTO2OpenAPI(dto.BaseInfo)
	return result
}

func DomainExperimentDTOs2OpenAPI(dtos []*domainExpt.Experiment) []*openapiExperiment.Experiment {
	if len(dtos) == 0 {
		return nil
	}
	result := make([]*openapiExperiment.Experiment, 0, len(dtos))
	for _, dto := range dtos {
		result = append(result, DomainExperimentDTO2OpenAPI(dto))
	}
	return result
}

func DomainTargetFieldMappingDTO2OpenAPI(mapping *domainExpt.TargetFieldMapping) *openapiExperiment.TargetFieldMapping {
	if mapping == nil {
		return nil
	}
	result := &openapiExperiment.TargetFieldMapping{}
	for _, fm := range mapping.FromEvalSet {
		if fm == nil {
			continue
		}
		result.FromEvalSet = append(result.FromEvalSet, &openapiExperiment.FieldMapping{
			FieldName:     fm.FieldName,
			FromFieldName: fm.FromFieldName,
		})
	}
	return result
}

func DomainEvaluatorFieldMappingDTO2OpenAPI(mappings []*domainExpt.EvaluatorFieldMapping) []*openapiExperiment.EvaluatorFieldMapping {
	if len(mappings) == 0 {
		return nil
	}
	result := make([]*openapiExperiment.EvaluatorFieldMapping, 0, len(mappings))
	for _, mapping := range mappings {
		if mapping == nil {
			continue
		}
		info := &openapiExperiment.EvaluatorFieldMapping{}
		if mapping.EvaluatorVersionID != 0 {
			info.EvaluatorVersionID = gptr.Of(mapping.EvaluatorVersionID)
		}
		for _, fromEval := range mapping.FromEvalSet {
			if fromEval == nil {
				continue
			}
			info.FromEvalSet = append(info.FromEvalSet, &openapiExperiment.FieldMapping{
				FieldName:     fromEval.FieldName,
				FromFieldName: fromEval.FromFieldName,
			})
		}
		for _, fromTarget := range mapping.FromTarget {
			if fromTarget == nil {
				continue
			}
			info.FromTarget = append(info.FromTarget, &openapiExperiment.FieldMapping{
				FieldName:     fromTarget.FieldName,
				FromFieldName: fromTarget.FromFieldName,
			})
		}
		result = append(result, info)
	}
	return result
}

func DomainRuntimeParamDTO2OpenAPI(param *domainCommon.RuntimeParam) *openapiCommon.RuntimeParam {
	if param == nil {
		return nil
	}
	if param.JSONValue == nil {
		return &openapiCommon.RuntimeParam{}
	}
	return &openapiCommon.RuntimeParam{JSONValue: param.JSONValue}
}

func DomainExperimentStatsDTO2OpenAPI(stats *domainExpt.ExptStatistics) *openapiExperiment.ExperimentStatistics {
	if stats == nil {
		return nil
	}
	return &openapiExperiment.ExperimentStatistics{
		PendingTurnCount:    stats.PendingTurnCnt,
		SuccessTurnCount:    stats.SuccessTurnCnt,
		FailedTurnCount:     stats.FailTurnCnt,
		TerminatedTurnCount: stats.TerminatedTurnCnt,
		ProcessingTurnCount: stats.ProcessingTurnCnt,
	}
}

func DomainBaseInfoDTO2OpenAPI(info *domainCommon.BaseInfo) *openapiCommon.BaseInfo {
	if info == nil {
		return nil
	}
	return &openapiCommon.BaseInfo{
		CreatedBy: DomainUserInfoDTO2OpenAPI(info.CreatedBy),
		UpdatedBy: DomainUserInfoDTO2OpenAPI(info.UpdatedBy),
		CreatedAt: info.CreatedAt,
		UpdatedAt: info.UpdatedAt,
	}
}

func DomainUserInfoDTO2OpenAPI(info *domainCommon.UserInfo) *openapiCommon.UserInfo {
	if info == nil {
		return nil
	}
	return &openapiCommon.UserInfo{
		UserID:    info.UserID,
		Name:      info.Name,
		AvatarURL: info.AvatarURL,
		Email:     info.Email,
	}
}

func convertInt64SliceToStringSlice(values []int64) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, v := range values {
		result = append(result, strconv.FormatInt(v, 10))
	}
	return result
}

func mapExperimentStatus(status *domainExpt.ExptStatus) *openapiExperiment.ExperimentStatus {
	if status == nil {
		return nil
	}
	var openapiStatus openapiExperiment.ExperimentStatus
	switch *status {
	case domainExpt.ExptStatus_Pending:
		openapiStatus = openapiExperiment.ExperimentStatusPending
	case domainExpt.ExptStatus_Processing:
		openapiStatus = openapiExperiment.ExperimentStatusProcessing
	case domainExpt.ExptStatus_Success:
		openapiStatus = openapiExperiment.ExperimentStatusSuccess
	case domainExpt.ExptStatus_Failed:
		openapiStatus = openapiExperiment.ExperimentStatusFailed
	case domainExpt.ExptStatus_Terminated:
		openapiStatus = openapiExperiment.ExperimentStatusTerminated
	case domainExpt.ExptStatus_Draining:
		openapiStatus = openapiExperiment.ExperimentStatusDraining
	case domainExpt.ExptStatus_SystemTerminated:
		openapiStatus = openapiExperiment.ExperimentStatusSystemTerminated
	default:
		openapiStatus = ""
	}
	return &openapiStatus
}

func mapExperimentType(exptType domainExpt.ExptType) openapiExperiment.ExperimentType {
	switch exptType {
	case domainExpt.ExptType_Online:
		return openapiExperiment.ExperimentTypeOnline
	default:
		return openapiExperiment.ExperimentTypeOffline
	}
}

// ---------- Column Result Converters ----------

func DomainColumnEvalSetFieldsDTO2OpenAPI(fields []*domainExpt.ColumnEvalSetField) []*openapiExperiment.ColumnEvalSetField {
	if len(fields) == 0 {
		return nil
	}
	result := make([]*openapiExperiment.ColumnEvalSetField, 0, len(fields))
	for _, field := range fields {
		if field == nil {
			continue
		}
		result = append(result, &openapiExperiment.ColumnEvalSetField{
			Key:         field.Key,
			Name:        field.Name,
			Description: field.Description,
			ContentType: convertContentTypeToOpenAPI(field.ContentType),
			TextSchema:  field.TextSchema,
		})
	}
	return result
}

func DomainColumnEvaluatorsDTO2OpenAPI(columnEvaluators []*domainExpt.ColumnEvaluator) []*openapiExperiment.ColumnEvaluator {
	if len(columnEvaluators) == 0 {
		return nil
	}
	result := make([]*openapiExperiment.ColumnEvaluator, 0, len(columnEvaluators))
	for _, evaluator := range columnEvaluators {
		if evaluator == nil {
			continue
		}
		result = append(result, &openapiExperiment.ColumnEvaluator{
			EvaluatorVersionID: gptr.Of(evaluator.EvaluatorVersionID),
			EvaluatorID:        gptr.Of(evaluator.EvaluatorID),
			EvaluatorType:      mapEvaluatorType(&evaluator.EvaluatorType),
			Name:               evaluator.Name,
			Version:            evaluator.Version,
			Description:        evaluator.Description,
		})
	}
	return result
}

func convertInt64PtrToStringPtr(value *int64) *string {
	if value == nil {
		return nil
	}
	str := strconv.FormatInt(*value, 10)
	return &str
}

func mapEvaluatorType(typ *domainEvaluator.EvaluatorType) *openapiEvaluator.EvaluatorType {
	if typ == nil {
		return nil
	}
	var openapiType openapiEvaluator.EvaluatorType
	switch *typ {
	case domainEvaluator.EvaluatorType_Prompt:
		openapiType = openapiEvaluator.EvaluatorTypePrompt
	case domainEvaluator.EvaluatorType_Code:
		openapiType = openapiEvaluator.EvaluatorTypeCode
	}
	return &openapiType
}

func convertContentTypeToOpenAPI(contentType *domainCommon.ContentType) *openapiCommon.ContentType {
	if contentType == nil {
		return nil
	}
	var openapiContentType openapiCommon.ContentType
	switch *contentType {
	case domainCommon.ContentTypeText:
		openapiContentType = openapiCommon.ContentTypeText
	case domainCommon.ContentTypeImage:
		openapiContentType = openapiCommon.ContentTypeImage
	case domainCommon.ContentTypeAudio:
		openapiContentType = openapiCommon.ContentTypeAudio
	case domainCommon.ContentTypeMultiPart:
		openapiContentType = openapiCommon.ContentTypeMultiPart
	default:
		openapiContentType = openapiCommon.ContentTypeText
	}
	return &openapiContentType
}

func OpenAPIExptDO2DTO(experiment *entity.Experiment) *openapiExperiment.Experiment {
	if experiment == nil {
		return nil
	}

	domainExperiment := ToExptDTO(experiment)
	if domainExperiment == nil {
		return nil
	}

	return DomainExperimentDTO2OpenAPI(domainExperiment)
}
