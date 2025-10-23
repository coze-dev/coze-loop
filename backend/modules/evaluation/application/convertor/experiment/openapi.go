// Copyright (c) 2025 Bytedance Ltd. and/or its affiliates
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"fmt"
	"strconv"

	evalsetopenapi "github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/evaluation_set"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
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

	result := &openapiExperiment.Experiment{
		ID:        gptr.Of(experiment.ID),
		Name:      gptr.Of(experiment.Name),
		ExptStats: openAPIExperimentStatsDO2DTO(experiment.Stats),
	}
	if experiment.Description != "" {
		result.Description = gptr.Of(experiment.Description)
	}

	if status := OpenAPIExperimentStatusDO2DTO(experiment.Status); status != nil {
		result.Status = status
	}

	if experiment.StartAt != nil {
		result.StartTime = gptr.Of(experiment.StartAt.Unix())
	}
	if experiment.EndAt != nil {
		result.EndTime = gptr.Of(experiment.EndAt.Unix())
	}

	if experiment.EvalConf != nil {
		if experiment.EvalConf.ItemConcurNum != nil {
			itemConcur := int32(*experiment.EvalConf.ItemConcurNum)
			result.ItemConcurNum = &itemConcur
		}

		mapping, runtimeParam := extractTargetIngressInfo(experiment.EvalConf.ConnectorConf.TargetConf)
		if mapping != nil {
			result.TargetFieldMapping = mapping
		}
		if runtimeParam != nil {
			result.TargetRuntimeParam = runtimeParam
		}

		if evaluatorMappings := openAPIEvaluatorFieldMappingsDO2DTO(experiment.EvalConf.ConnectorConf.EvaluatorsConf); len(evaluatorMappings) > 0 {
			result.EvaluatorFieldMapping = evaluatorMappings
		}

		if experiment.EvalConf.ConnectorConf.EvaluatorsConf != nil && experiment.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConcurNum != nil {
			evaluatorConcur := int32(*experiment.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConcurNum)
			result.EvaluatorsConcurNum = &evaluatorConcur
		}
	}

	return result
}

func OpenAPIExperimentStatusDO2DTO(status entity.ExptStatus) *openapiExperiment.ExperimentStatus {
	var openapiStatus openapiExperiment.ExperimentStatus
	switch status {
	case entity.ExptStatus_Pending:
		openapiStatus = openapiExperiment.ExperimentStatusPending
	case entity.ExptStatus_Processing:
		openapiStatus = openapiExperiment.ExperimentStatusProcessing
	case entity.ExptStatus_Success:
		openapiStatus = openapiExperiment.ExperimentStatusSuccess
	case entity.ExptStatus_Failed:
		openapiStatus = openapiExperiment.ExperimentStatusFailed
	case entity.ExptStatus_Terminated:
		openapiStatus = openapiExperiment.ExperimentStatusTerminated
	case entity.ExptStatus_SystemTerminated:
		openapiStatus = openapiExperiment.ExperimentStatusSystemTerminated
	case entity.ExptStatus_Draining:
		openapiStatus = openapiExperiment.ExperimentStatusDraining
	default:
		return nil
	}
	return &openapiStatus
}

func extractTargetIngressInfo(targetConf *entity.TargetConf) (*openapiExperiment.TargetFieldMapping, *openapiCommon.RuntimeParam) {
	if targetConf == nil || targetConf.IngressConf == nil {
		return nil, nil
	}

	var mapping *openapiExperiment.TargetFieldMapping
	if fields := convertFieldAdapterToMappings(targetConf.IngressConf.EvalSetAdapter); len(fields) > 0 {
		mapping = &openapiExperiment.TargetFieldMapping{FromEvalSet: fields}
	}

	runtimeParam := extractRuntimeParamFromAdapter(targetConf.IngressConf.CustomConf)

	return mapping, runtimeParam
}

func openAPIEvaluatorFieldMappingsDO2DTO(conf *entity.EvaluatorsConf) []*openapiExperiment.EvaluatorFieldMapping {
	if conf == nil || len(conf.EvaluatorConf) == 0 {
		return nil
	}

	mappings := make([]*openapiExperiment.EvaluatorFieldMapping, 0, len(conf.EvaluatorConf))
	for _, evaluatorConf := range conf.EvaluatorConf {
		if evaluatorConf == nil {
			continue
		}

		mapping := &openapiExperiment.EvaluatorFieldMapping{}
		if evaluatorConf.EvaluatorVersionID != 0 {
			mapping.EvaluatorVersionID = gptr.Of(evaluatorConf.EvaluatorVersionID)
		}

		if ingress := evaluatorConf.IngressConf; ingress != nil {
			if fields := convertFieldAdapterToMappings(ingress.EvalSetAdapter); len(fields) > 0 {
				mapping.FromEvalSet = fields
			}
			if fields := convertFieldAdapterToMappings(ingress.TargetAdapter); len(fields) > 0 {
				mapping.FromTarget = fields
			}
		}

		if mapping.EvaluatorVersionID == nil && len(mapping.FromEvalSet) == 0 && len(mapping.FromTarget) == 0 {
			continue
		}
		mappings = append(mappings, mapping)
	}

	if len(mappings) == 0 {
		return nil
	}
	return mappings
}

func convertFieldAdapterToMappings(adapter *entity.FieldAdapter) []*openapiExperiment.FieldMapping {
	if adapter == nil || len(adapter.FieldConfs) == 0 {
		return nil
	}

	result := make([]*openapiExperiment.FieldMapping, 0, len(adapter.FieldConfs))
	for _, conf := range adapter.FieldConfs {
		if conf == nil {
			continue
		}

		mapping := &openapiExperiment.FieldMapping{}
		if conf.FieldName != "" {
			mapping.FieldName = gptr.Of(conf.FieldName)
		}
		if conf.FromField != "" {
			mapping.FromFieldName = gptr.Of(conf.FromField)
		}

		if mapping.FieldName == nil && mapping.FromFieldName == nil {
			continue
		}
		result = append(result, mapping)
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

func extractRuntimeParamFromAdapter(adapter *entity.FieldAdapter) *openapiCommon.RuntimeParam {
	if adapter == nil || len(adapter.FieldConfs) == 0 {
		return nil
	}

	for _, conf := range adapter.FieldConfs {
		if conf == nil {
			continue
		}
		if conf.FieldName == consts.FieldAdapterBuiltinFieldNameRuntimeParam {
			runtimeParam := &openapiCommon.RuntimeParam{}
			runtimeParam.JSONValue = gptr.Of(conf.Value)
			return runtimeParam
		}
	}

	return nil
}

func openAPIExperimentStatsDO2DTO(stats *entity.ExptStats) *openapiExperiment.ExperimentStatistics {
	if stats == nil {
		return nil
	}
	return &openapiExperiment.ExperimentStatistics{
		PendingTurnCount:    gptr.Of(stats.PendingItemCnt),
		SuccessTurnCount:    gptr.Of(stats.SuccessItemCnt),
		FailedTurnCount:     gptr.Of(stats.FailItemCnt),
		TerminatedTurnCount: gptr.Of(stats.TerminatedItemCnt),
		ProcessingTurnCount: gptr.Of(stats.ProcessingItemCnt),
	}
}

func OpenAPIColumnEvalSetFieldsDO2DTOs(from []*entity.ColumnEvalSetField) []*openapiExperiment.ColumnEvalSetField {
	if len(from) == 0 {
		return nil
	}
	result := make([]*openapiExperiment.ColumnEvalSetField, 0, len(from))
	for _, field := range from {
		if field == nil {
			continue
		}
		result = append(result, &openapiExperiment.ColumnEvalSetField{
			Key:         field.Key,
			Name:        field.Name,
			Description: field.Description,
			ContentType: convertEntityContentTypeToOpenAPI(field.ContentType),
			TextSchema:  field.TextSchema,
		})
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
func OpenAPIColumnEvaluatorsDO2DTOs(from []*entity.ColumnEvaluator) []*openapiExperiment.ColumnEvaluator {
	if len(from) == 0 {
		return nil
	}
	result := make([]*openapiExperiment.ColumnEvaluator, 0, len(from))
	for _, evaluator := range from {
		if evaluator == nil {
			continue
		}
		result = append(result, &openapiExperiment.ColumnEvaluator{
			EvaluatorVersionID: gptr.Of(evaluator.EvaluatorVersionID),
			EvaluatorID:        gptr.Of(evaluator.EvaluatorID),
			EvaluatorType:      convertEntityEvaluatorTypeToOpenAPI(evaluator.EvaluatorType),
			Name:               evaluator.Name,
			Version:            evaluator.Version,
			Description:        evaluator.Description,
		})
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
func OpenAPIItemResultsDO2DTOs(from []*entity.ItemResult) []*openapiExperiment.ItemResult_ {
	if len(from) == 0 {
		return nil
	}
	result := make([]*openapiExperiment.ItemResult_, 0, len(from))
	for _, item := range from {
		if item == nil {
			continue
		}
		result = append(result, &openapiExperiment.ItemResult_{
			ItemID:      gptr.Of(item.ItemID),
			TurnResults: openAPITurnResultsDO2DTOs(item.TurnResults),
		})
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func convertEntityContentTypeToOpenAPI(contentType entity.ContentType) *openapiCommon.ContentType {
	var openapiType openapiCommon.ContentType
	switch contentType {
	case entity.ContentTypeText:
		openapiType = openapiCommon.ContentTypeText
	case entity.ContentTypeImage:
		openapiType = openapiCommon.ContentTypeImage
	case entity.ContentTypeAudio:
		openapiType = openapiCommon.ContentTypeAudio
	case entity.ContentTypeMultipart, entity.ContentTypeMultipartVariable:
		openapiType = openapiCommon.ContentTypeMultiPart
	default:
		return nil
	}
	return &openapiType
}

func convertEntityEvaluatorTypeToOpenAPI(typ entity.EvaluatorType) *openapiEvaluator.EvaluatorType {
	var openapiType openapiEvaluator.EvaluatorType
	switch typ {
	case entity.EvaluatorTypePrompt:
		openapiType = openapiEvaluator.EvaluatorTypePrompt
	case entity.EvaluatorTypeCode:
		openapiType = openapiEvaluator.EvaluatorTypeCode
	default:
		return nil
	}
	return &openapiType
}

func openAPITurnResultsDO2DTOs(from []*entity.TurnResult) []*openapiExperiment.TurnResult_ {
	if len(from) == 0 {
		return nil
	}
	result := make([]*openapiExperiment.TurnResult_, 0, len(from))
	for _, turn := range from {
		if turn == nil {
			continue
		}
		turnDTO := &openapiExperiment.TurnResult_{}
		if turn.TurnID != 0 {
			turnDTO.TurnID = gptr.Of(strconv.FormatInt(turn.TurnID, 10))
		}
		if len(turn.ExperimentResults) > 0 {
			if payload := openAPIResultPayloadDO2DTO(turn.ExperimentResults[0]); payload != nil {
				turnDTO.Payload = payload
			}
		}
		result = append(result, turnDTO)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func openAPIResultPayloadDO2DTO(result *entity.ExperimentResult) *openapiExperiment.ResultPayload {
	if result == nil || result.Payload == nil {
		return nil
	}
	payload := result.Payload
	res := &openapiExperiment.ResultPayload{}
	if payload.EvalSet != nil {
		res.EvalSetTurn = evalsetopenapi.OpenAPITurnDO2DTO(payload.EvalSet.Turn)
	}
	if payload.EvaluatorOutput != nil && len(payload.EvaluatorOutput.EvaluatorRecords) > 0 {
		res.EvaluatorRecords = openAPIEvaluatorRecordsMapDO2DTO(payload.EvaluatorOutput.EvaluatorRecords)
	}
	if res.EvalSetTurn == nil && len(res.EvaluatorRecords) == 0 {
		return nil
	}
	return res
}

func openAPIEvaluatorRecordsMapDO2DTO(records map[int64]*entity.EvaluatorRecord) []*openapiEvaluator.EvaluatorRecord {
	if len(records) == 0 {
		return nil
	}
	result := make([]*openapiEvaluator.EvaluatorRecord, 0, len(records))
	for _, record := range records {
		if record == nil {
			continue
		}
		result = append(result, openAPIEvaluatorRecordDO2DTO(record))
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func openAPIEvaluatorRecordDO2DTO(record *entity.EvaluatorRecord) *openapiEvaluator.EvaluatorRecord {
	if record == nil {
		return nil
	}
	res := &openapiEvaluator.EvaluatorRecord{
		ID:                 gptr.Of(record.ID),
		EvaluatorVersionID: gptr.Of(record.EvaluatorVersionID),
		ItemID:             gptr.Of(record.ItemID),
		TurnID:             gptr.Of(record.TurnID),
		Status:             convertEntityEvaluatorStatusToOpenAPI(record.Status),
	}
	return res
}

func convertEntityEvaluatorStatusToOpenAPI(status entity.EvaluatorRunStatus) *openapiEvaluator.EvaluatorRunStatus {
	var openapiStatus openapiEvaluator.EvaluatorRunStatus
	switch status {
	case entity.EvaluatorRunStatusSuccess:
		openapiStatus = openapiEvaluator.EvaluatorRunStatusSuccess
	case entity.EvaluatorRunStatusFail:
		openapiStatus = openapiEvaluator.EvaluatorRunStatusFailed
	case entity.EvaluatorRunStatusUnknown:
		return nil
	default:
		openapiStatus = openapiEvaluator.EvaluatorRunStatusProcessing
	}
	return &openapiStatus
}
