// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"fmt"
	"strconv"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/common"
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

func OpenAPIEvaluatorFieldMappingDTO2Domain(mappings []*openapiExperiment.EvaluatorFieldMapping, evaluatorMap map[string]int64) []*domainExpt.EvaluatorFieldMapping {
	if len(mappings) == 0 {
		return nil
	}

	result := make([]*domainExpt.EvaluatorFieldMapping, 0, len(mappings))
	for _, mapping := range mappings {
		if mapping == nil {
			continue
		}
		versionID := evaluatorMap[fmt.Sprintf("%d_%s", mapping.GetEvaluatorID(), mapping.GetVersion())]
		domainMapping := &domainExpt.EvaluatorFieldMapping{
			EvaluatorVersionID: versionID,
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
		Env:                 param.Env,
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
	if param.Region != nil {
		region, err := mapOpenAPIRegion(*param.Region)
		if err != nil {
			return nil
		}
		result.Region = &region
	}
	if param.CustomEvalTarget != nil {
		customTarget := &domaindoEvalTarget.CustomEvalTarget{
			ID:        param.CustomEvalTarget.ID,
			Name:      param.CustomEvalTarget.Name,
			AvatarURL: param.CustomEvalTarget.AvatarURL,
			Ext:       param.CustomEvalTarget.Ext,
		}
		result.CustomEvalTarget = customTarget
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
	case openapiEvalTarget.EvalTargetTypeCustomRPCServer:
		return domaindoEvalTarget.EvalTargetType_CustomRPCServer, nil
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

func mapOpenAPIRegion(region openapiEvalTarget.Region) (domaindoEvalTarget.Region, error) {
	switch region {
	case openapiEvalTarget.RegionBOE:
		return domaindoEvalTarget.RegionBOE, nil
	case openapiEvalTarget.RegionCN:
		return domaindoEvalTarget.RegionCN, nil
	case openapiEvalTarget.RegionI18N:
		return domaindoEvalTarget.RegionI18N, nil
	default:
		return "", fmt.Errorf("unsupported region: %s", region)
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
		EvaluatorFieldMapping: DomainEvaluatorFieldMappingDTO2OpenAPI(dto.EvaluatorFieldMapping, dto.Evaluators),
		TargetRuntimeParam:    DomainRuntimeParamDTO2OpenAPI(dto.TargetRuntimeParam),
	}

	result.Status = mapExperimentStatus(dto.Status)
	result.StartedAt = dto.StartTime
	result.EndedAt = dto.EndTime
	result.ExptStats = DomainExperimentStatsDTO2OpenAPI(dto.ExptStats)
	result.BaseInfo = DomainBaseInfoDTO2OpenAPI(dto.BaseInfo)
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

func DomainEvaluatorFieldMappingDTO2OpenAPI(mappings []*domainExpt.EvaluatorFieldMapping, evaluators []*domainEvaluator.Evaluator) []*openapiExperiment.EvaluatorFieldMapping {
	if len(mappings) == 0 {
		return nil
	}
	evaluatorMap := make(map[int64][]string)
	for _, e := range evaluators {
		evaluatorMap[e.GetCurrentVersion().GetID()] = []string{strconv.FormatInt(e.GetEvaluatorID(), 10), e.GetCurrentVersion().GetVersion()}
	}
	result := make([]*openapiExperiment.EvaluatorFieldMapping, 0, len(mappings))
	for _, mapping := range mappings {
		if mapping == nil {
			continue
		}
		infos := evaluatorMap[mapping.EvaluatorVersionID]
		var id int64
		var version string
		if len(infos) == 2 {
			id, _ = strconv.ParseInt(infos[0], 10, 64)
			version = infos[1]
		}
		info := &openapiExperiment.EvaluatorFieldMapping{}
		if mapping.EvaluatorVersionID != 0 {
			info.EvaluatorID = gptr.Of(id)
			info.Version = gptr.Of(version)
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

// ---------- Column Result Converters ----------

func OpenAPIExptDO2DTO(experiment *entity.Experiment) *openapiExperiment.Experiment {
	if experiment == nil {
		return nil
	}

	result := &openapiExperiment.Experiment{
		ID:        gptr.Of(experiment.ID),
		Name:      gptr.Of(experiment.Name),
		ExptStats: openAPIExperimentStatsDO2DTO(experiment.Stats),
		BaseInfo: &openapiCommon.BaseInfo{
			CreatedBy: &openapiCommon.UserInfo{
				UserID: gptr.Of(experiment.CreatedBy),
			},
		},
	}
	if experiment.Description != "" {
		result.Description = gptr.Of(experiment.Description)
	}

	if status := OpenAPIExperimentStatusDO2DTO(experiment.Status); status != nil {
		result.Status = status
	}

	if experiment.StartAt != nil {
		result.StartedAt = gptr.Of(experiment.StartAt.Unix())
	}
	if experiment.EndAt != nil {
		result.EndedAt = gptr.Of(experiment.EndAt.Unix())
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

		if evaluatorMappings := openAPIEvaluatorFieldMappingsDO2DTO(experiment.EvalConf.ConnectorConf.EvaluatorsConf, experiment.Evaluators); len(evaluatorMappings) > 0 {
			result.EvaluatorFieldMapping = evaluatorMappings
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

func openAPIEvaluatorFieldMappingsDO2DTO(conf *entity.EvaluatorsConf, evaluators []*entity.Evaluator) []*openapiExperiment.EvaluatorFieldMapping {
	if conf == nil || len(conf.EvaluatorConf) == 0 {
		return nil
	}
	evaluatorMap := make(map[int64][]string)
	for _, e := range evaluators {
		evaluatorMap[e.GetEvaluatorVersionID()] = []string{strconv.FormatInt(e.ID, 10), e.GetVersion()}
	}

	mappings := make([]*openapiExperiment.EvaluatorFieldMapping, 0, len(conf.EvaluatorConf))
	for _, evaluatorConf := range conf.EvaluatorConf {
		if evaluatorConf == nil {
			continue
		}
		infos := evaluatorMap[evaluatorConf.EvaluatorVersionID]
		var id int64
		var version string
		if len(infos) == 2 {
			id, _ = strconv.ParseInt(infos[0], 10, 64)
			version = infos[1]
		}
		mapping := &openapiExperiment.EvaluatorFieldMapping{}
		if evaluatorConf.EvaluatorVersionID != 0 {
			mapping.EvaluatorID = gptr.Of(id)
			mapping.Version = gptr.Of(version)
		}

		if ingress := evaluatorConf.IngressConf; ingress != nil {
			if fields := convertFieldAdapterToMappings(ingress.EvalSetAdapter); len(fields) > 0 {
				mapping.FromEvalSet = fields
			}
			if fields := convertFieldAdapterToMappings(ingress.TargetAdapter); len(fields) > 0 {
				mapping.FromTarget = fields
			}
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

func OpenAPIColumnEvalTargetDO2DTOs(columns []*entity.ColumnEvalTarget) []*openapiExperiment.ColumnEvalTarget {
	if len(columns) == 0 {
		return nil
	}
	result := make([]*openapiExperiment.ColumnEvalTarget, 0, len(columns))
	for _, column := range columns {
		if column == nil {
			continue
		}
		result = append(result, &openapiExperiment.ColumnEvalTarget{
			Name:        gptr.Of(column.Name),
			Description: gptr.Of(column.Desc),
			Label:       column.Label,
		})
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
		res := &openapiExperiment.ItemResult_{
			ItemID:      gptr.Of(item.ItemID),
			TurnResults: openAPITurnResultsDO2DTOs(item.TurnResults),
		}
		if item.SystemInfo != nil {
			res.SystemInfo = &openapiExperiment.ItemSystemInfo{
				RunState: ItemRunStateDO2DTO(item.SystemInfo.RunState),
			}
		}
		result = append(result, res)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func ItemRunStateDO2DTO(state entity.ItemRunState) *openapiExperiment.ItemRunState {
	var openapiState openapiExperiment.ItemRunState
	switch state {
	case entity.ItemRunState_Queueing:
		openapiState = openapiExperiment.ItemRunStateQueueing
	case entity.ItemRunState_Processing:
		openapiState = openapiExperiment.ItemRunStateProcessing
	case entity.ItemRunState_Success:
		openapiState = openapiExperiment.ItemRunStateSuccess
	case entity.ItemRunState_Fail:
		openapiState = openapiExperiment.ItemRunStateFail
	case entity.ItemRunState_Terminal:
		openapiState = openapiExperiment.ItemRunStateTerminal
	default:
		return nil
	}
	return &openapiState
}

func TurnRunStateDO2DTO(state entity.TurnRunState) *openapiExperiment.TurnRunState {
	var openapiState openapiExperiment.TurnRunState
	switch state {
	case entity.TurnRunState_Queueing:
		openapiState = openapiExperiment.TurnRunStateQueueing
	case entity.TurnRunState_Processing:
		openapiState = openapiExperiment.TurnRunStateProcessing
	case entity.TurnRunState_Success:
		openapiState = openapiExperiment.TurnRunStateSuccess
	case entity.TurnRunState_Fail:
		openapiState = openapiExperiment.TurnRunStateFail
	case entity.TurnRunState_Terminal:
		openapiState = openapiExperiment.TurnRunStateTerminal
	default:
		return nil
	}
	return &openapiState
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
	if payload.TargetOutput != nil {
		res.TargetRecord = openAPITargetRecordDO2DTO(payload.TargetOutput.EvalTargetRecord)
	}
	if payload.SystemInfo != nil {
		res.SystemInfo = &openapiExperiment.TurnSystemInfo{
			TurnRunState: TurnRunStateDO2DTO(payload.SystemInfo.TurnRunState),
		}
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
		Logid:              gptr.Of(record.LogID),
		TraceID:            gptr.Of(record.TraceID),
		BaseInfo:           common.OpenAPIBaseInfoDO2DTO(record.BaseInfo),
	}
	if output := openAPIEvaluatorOutputDataDO2DTO(record.EvaluatorOutputData); output != nil {
		res.EvaluatorOutputData = output
	}
	return res
}

func openAPITargetRecordDO2DTO(record *entity.EvalTargetRecord) *openapiEvalTarget.EvalTargetRecord {
	if record == nil {
		return nil
	}
	res := &openapiEvalTarget.EvalTargetRecord{
		ID:              gptr.Of(record.ID),
		TargetID:        gptr.Of(record.TargetID),
		TargetVersionID: gptr.Of(record.TargetVersionID),
		ItemID:          gptr.Of(record.ItemID),
		TurnID:          gptr.Of(record.TurnID),
		Logid:           gptr.Of(record.LogID),
		TraceID:         gptr.Of(record.TraceID),
		BaseInfo:        common.OpenAPIBaseInfoDO2DTO(record.BaseInfo),
	}
	if output := openAPITargetOutputDataDO2DTO(record.EvalTargetOutputData); output != nil {
		res.EvalTargetOutputData = output
	}
	if status := convertEntityTargetRunStatusToOpenAPI(record.Status); status != nil {
		res.Status = status
	}
	return res
}

func openAPITargetOutputDataDO2DTO(data *entity.EvalTargetOutputData) *openapiEvalTarget.EvalTargetOutputData {
	if data == nil {
		return nil
	}
	res := &openapiEvalTarget.EvalTargetOutputData{}
	if fields := openAPITargetOutputFieldsDO2DTO(data.OutputFields); len(fields) > 0 {
		res.OutputFields = fields
	}
	if usage := openAPITargetUsageDO2DTO(data.EvalTargetUsage); usage != nil {
		res.EvalTargetUsage = usage
	}
	if runErr := openAPITargetRunErrorDO2DTO(data.EvalTargetRunError); runErr != nil {
		res.EvalTargetRunError = runErr
	}
	if data.TimeConsumingMS != nil {
		res.TimeConsumingMs = data.TimeConsumingMS
	}
	if len(res.OutputFields) == 0 && res.EvalTargetUsage == nil && res.EvalTargetRunError == nil && res.TimeConsumingMs == nil {
		return nil
	}
	return res
}

func openAPITargetOutputFieldsDO2DTO(fields map[string]*entity.Content) map[string]*openapiCommon.Content {
	if len(fields) == 0 {
		return nil
	}
	converted := make(map[string]*openapiCommon.Content, len(fields))
	for key, value := range fields {
		if value == nil {
			continue
		}
		if content := evalsetopenapi.OpenAPIContentDO2DTO(value); content != nil {
			converted[key] = content
		}
	}
	if len(converted) == 0 {
		return nil
	}
	return converted
}

func openAPITargetUsageDO2DTO(usage *entity.EvalTargetUsage) *openapiEvalTarget.EvalTargetUsage {
	if usage == nil {
		return nil
	}
	return &openapiEvalTarget.EvalTargetUsage{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
	}
}

func openAPITargetRunErrorDO2DTO(err *entity.EvalTargetRunError) *openapiEvalTarget.EvalTargetRunError {
	if err == nil {
		return nil
	}
	res := &openapiEvalTarget.EvalTargetRunError{}
	if err.Code != 0 {
		res.Code = gptr.Of(err.Code)
	}
	if err.Message != "" {
		res.Message = gptr.Of(err.Message)
	}
	if res.Code == nil && res.Message == nil {
		return nil
	}
	return res
}

func convertEntityTargetRunStatusToOpenAPI(status *entity.EvalTargetRunStatus) *openapiEvalTarget.EvalTargetRunStatus {
	if status == nil {
		return nil
	}
	var openapiStatus openapiEvalTarget.EvalTargetRunStatus
	switch *status {
	case entity.EvalTargetRunStatusSuccess:
		openapiStatus = openapiEvalTarget.EvalTargetRunStatusSuccess
	case entity.EvalTargetRunStatusFail:
		openapiStatus = openapiEvalTarget.EvalTargetRunStatusFail
	default:
		return nil
	}
	return &openapiStatus
}

func openAPIEvaluatorOutputDataDO2DTO(data *entity.EvaluatorOutputData) *openapiEvaluator.EvaluatorOutputData {
	if data == nil {
		return nil
	}
	res := &openapiEvaluator.EvaluatorOutputData{}
	if result := openAPIEvaluatorResultDO2DTO(data.EvaluatorResult); result != nil {
		res.EvaluatorResult_ = result
	}
	if usage := openAPIEvaluatorUsageDO2DTO(data.EvaluatorUsage); usage != nil {
		res.EvaluatorUsage = usage
	}
	if runErr := openAPIEvaluatorRunErrorDO2DTO(data.EvaluatorRunError); runErr != nil {
		res.EvaluatorRunError = runErr
	}
	if data.TimeConsumingMS > 0 {
		res.TimeConsumingMs = gptr.Of(data.TimeConsumingMS)
	}
	if res.EvaluatorResult_ == nil && res.EvaluatorUsage == nil && res.EvaluatorRunError == nil && res.TimeConsumingMs == nil {
		return nil
	}
	return res
}

func openAPIEvaluatorResultDO2DTO(result *entity.EvaluatorResult) *openapiEvaluator.EvaluatorResult_ {
	if result == nil {
		return nil
	}
	res := &openapiEvaluator.EvaluatorResult_{}
	if result.Correction != nil {
		if result.Correction.Score != nil {
			res.Score = result.Correction.Score
		} else if result.Score != nil {
			res.Score = result.Score
		}
		if result.Correction.Explain != "" {
			res.Reasoning = gptr.Of(result.Correction.Explain)
		} else if result.Reasoning != "" {
			res.Reasoning = gptr.Of(result.Reasoning)
		}
	} else {
		if result.Score != nil {
			res.Score = result.Score
		}
		if result.Reasoning != "" {
			res.Reasoning = gptr.Of(result.Reasoning)
		}
	}
	if res.Score == nil && res.Reasoning == nil {
		return nil
	}
	return res
}

func openAPIEvaluatorUsageDO2DTO(usage *entity.EvaluatorUsage) *openapiEvaluator.EvaluatorUsage {
	if usage == nil {
		return nil
	}
	res := &openapiEvaluator.EvaluatorUsage{}
	if usage.InputTokens != 0 {
		res.InputTokens = gptr.Of(usage.InputTokens)
	}
	if usage.OutputTokens != 0 {
		res.OutputTokens = gptr.Of(usage.OutputTokens)
	}
	if res.InputTokens == nil && res.OutputTokens == nil {
		return nil
	}
	return res
}

func openAPIEvaluatorRunErrorDO2DTO(err *entity.EvaluatorRunError) *openapiEvaluator.EvaluatorRunError {
	if err == nil {
		return nil
	}
	res := &openapiEvaluator.EvaluatorRunError{}
	if err.Code != 0 {
		res.Code = gptr.Of(err.Code)
	}
	if err.Message != "" {
		res.Message = gptr.Of(err.Message)
	}
	if res.Code == nil && res.Message == nil {
		return nil
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

func OpenAPIAggregatorResultsDO2DTOs(results []*entity.AggregatorResult) []*openapiExperiment.AggregatorResult_ {
	if len(results) == 0 {
		return nil
	}
	converted := make([]*openapiExperiment.AggregatorResult_, 0, len(results))
	for _, result := range results {
		if result == nil {
			continue
		}
		aggregatorType := openAPIAggregatorTypeDO2DTO(result.AggregatorType)
		aggregateData := openAPIAggregateDataDO2DTO(result.Data)
		if aggregatorType == nil && aggregateData == nil {
			continue
		}
		converted = append(converted, &openapiExperiment.AggregatorResult_{
			AggregatorType: aggregatorType,
			Data:           aggregateData,
		})
	}
	if len(converted) == 0 {
		return nil
	}
	return converted
}

func openAPIAggregatorTypeDO2DTO(typ entity.AggregatorType) *openapiExperiment.AggregatorType {
	var openapiType openapiExperiment.AggregatorType
	switch typ {
	case entity.Average:
		openapiType = openapiExperiment.AggregatorTypeAverage
	case entity.Sum:
		openapiType = openapiExperiment.AggregatorTypeSum
	case entity.Max:
		openapiType = openapiExperiment.AggregatorTypeMax
	case entity.Min:
		openapiType = openapiExperiment.AggregatorTypeMin
	case entity.Distribution:
		openapiType = openapiExperiment.AggregatorTypeDistribution
	default:
		return nil
	}
	return &openapiType
}

func openAPIAggregateDataDO2DTO(data *entity.AggregateData) *openapiExperiment.AggregateData {
	if data == nil {
		return nil
	}
	aggregateData := &openapiExperiment.AggregateData{}
	switch data.DataType {
	case entity.Double:
		dataType := openapiExperiment.DataTypeDouble
		aggregateData.DataType = &dataType
		aggregateData.Value = data.Value
	case entity.ScoreDistribution:
		dataType := openapiExperiment.DataTypeScoreDistribution
		aggregateData.DataType = &dataType
		aggregateData.ScoreDistribution = openAPIScoreDistributionDO2DTO(data.ScoreDistribution)
	default:
		return nil
	}
	return aggregateData
}

func openAPIScoreDistributionDO2DTO(data *entity.ScoreDistributionData) *openapiExperiment.ScoreDistribution {
	if data == nil || len(data.ScoreDistributionItems) == 0 {
		return nil
	}
	items := make([]*openapiExperiment.ScoreDistributionItem, 0, len(data.ScoreDistributionItems))
	for _, item := range data.ScoreDistributionItems {
		if item == nil {
			continue
		}
		items = append(items, &openapiExperiment.ScoreDistributionItem{
			Score:      gptr.Of(item.Score),
			Count:      gptr.Of(item.Count),
			Percentage: gptr.Of(item.Percentage),
		})
	}
	if len(items) == 0 {
		return nil
	}
	return &openapiExperiment.ScoreDistribution{ScoreDistributionItems: items}
}
