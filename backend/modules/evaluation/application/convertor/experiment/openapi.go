// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/common"
	evalsetopenapi "github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/evaluation_set"
	evaluator_convertor "github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/evaluator"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/slices"

	openapiCommon "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/common"
	openapiEvalTarget "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/eval_target"
	openapiEvaluator "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/evaluator"
	openapiExperiment "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"

	domainCommon "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	domaindoEvalTarget "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	domainEvaluator "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	domainExpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	domainEvalTarget "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_target"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
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

	if experiment.Target != nil {
		result.EvalTarget = OpenAPIEvalTargetDO2DTO(experiment.Target)
	}
	if experiment.EvalSet != nil && experiment.ExptType != entity.ExptType_Online {
		result.EvalSet = evalsetopenapi.OpenAPIEvaluationSetDO2DTO(experiment.EvalSet)
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

// openAPIExptTupleEvaluatorItemsToEntity 将 ExptTuple 的 evaluator_id_version_items 转为 entity.EvaluatorIDVersionItem。
func openAPIExptTupleEvaluatorItemsToEntity(tc *openapiExperiment.ExptTuple) []*entity.EvaluatorIDVersionItem {
	if tc == nil || tc.EvaluatorIDVersionItems == nil {
		return nil
	}
	items := tc.EvaluatorIDVersionItems
	out := make([]*entity.EvaluatorIDVersionItem, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, &entity.EvaluatorIDVersionItem{
			EvaluatorID:        item.GetEvaluatorID(),
			Version:            item.GetVersion(),
			EvaluatorVersionID: item.GetEvaluatorVersionID(),
			ScoreWeight:        item.GetScoreWeight(),
		})
	}
	return out
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
	case entity.ContentTypeVideo:
		openapiType = openapiCommon.ContentTypeVideo
	case entity.ContentTypeMultipart:
		openapiType = openapiCommon.ContentTypeMultiPart
	case entity.ContentTypeMultipartVariable:
		openapiType = openapiCommon.ContentTypeMultiPartVariable
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

func OpenTargetAggrResultDO2DTO(result *entity.EvalTargetMtrAggrResult) *openapiExperiment.EvalTargetAggregateResult_ {
	if result == nil {
		return nil
	}
	return &openapiExperiment.EvalTargetAggregateResult_{
		TargetID:        gptr.Of(result.TargetID),
		TargetVersionID: gptr.Of(result.TargetVersionID),
		Latency:         OpenAPIAggregatorResultsDO2DTOs(result.LatencyAggrResults),
		InputTokens:     OpenAPIAggregatorResultsDO2DTOs(result.InputTokensAggrResults),
		OutputTokens:    OpenAPIAggregatorResultsDO2DTOs(result.OutputTokensAggrResults),
		TotalTokens:     OpenAPIAggregatorResultsDO2DTOs(result.TotalTokensAggrResults),
	}
}

func TargetAggrResultDO2DTO(result *entity.EvalTargetMtrAggrResult) *domainExpt.EvalTargetAggregateResult_ {
	if result == nil {
		return nil
	}
	return &domainExpt.EvalTargetAggregateResult_{
		TargetID:        gptr.Of(result.TargetID),
		TargetVersionID: gptr.Of(result.TargetVersionID),
		Latency:         AggregatorResultDOsToDTOs(result.LatencyAggrResults),
		InputTokens:     AggregatorResultDOsToDTOs(result.InputTokensAggrResults),
		OutputTokens:    AggregatorResultDOsToDTOs(result.OutputTokensAggrResults),
		TotalTokens:     AggregatorResultDOsToDTOs(result.TotalTokensAggrResults),
	}
}

func OpenAPIEvaluatorParamsDTO2Domain(dtos []*openapi.SubmitExperimentEvaluatorParam) []*domainEvaluator.EvaluatorIDVersionItem {
	if len(dtos) == 0 {
		return nil
	}
	dos := make([]*domainEvaluator.EvaluatorIDVersionItem, 0, len(dtos))
	for _, dto := range dtos {
		if dto == nil {
			continue
		}
		dos = append(dos, OpenAPIEvaluatorParamDTO2Domain(dto))
	}
	return dos
}

func OpenAPIEvaluatorParamDTO2Domain(dto *openapi.SubmitExperimentEvaluatorParam) *domainEvaluator.EvaluatorIDVersionItem {
	if dto == nil {
		return nil
	}

	return &domainEvaluator.EvaluatorIDVersionItem{
		EvaluatorID: dto.EvaluatorID,
		Version:     dto.Version,
		RunConfig:   OpenAPIEvaluatorRunConfigDTO2Domain(dto.RunConfig),
	}
}

func OpenAPIEvaluatorRunConfigDTO2Domain(dto *openapiEvaluator.EvaluatorRunConfig) *domainEvaluator.EvaluatorRunConfig {
	if dto == nil {
		return nil
	}
	return &domainEvaluator.EvaluatorRunConfig{
		Env:                   dto.Env,
		EvaluatorRuntimeParam: OpenAPIRuntimeParamDTO2Domain(dto.EvaluatorRuntimeParam),
	}
}

func OpenAPIEvalTargetDO2DTO(targetDO *entity.EvalTarget) *openapiEvalTarget.EvalTarget {
	if targetDO == nil {
		return nil
	}

	targetDTO := &openapiEvalTarget.EvalTarget{
		ID:             gptr.Of(targetDO.ID),
		SourceTargetID: gptr.Of(targetDO.SourceTargetID),
	}
	if targetDO.EvalTargetType != 0 {
		targetDTO.EvalTargetType = gptr.Of(convertEntityEvalTargetTypeToOpenAPI(targetDO.EvalTargetType))
	}
	if targetDO.EvalTargetVersion != nil {
		targetDTO.EvalTargetVersion = OpenAPIEvalTargetVersionDO2DTO(targetDO.EvalTargetVersion, targetDO.EvalTargetType)
	}
	targetDTO.BaseInfo = common.OpenAPIBaseInfoDO2DTO(targetDO.BaseInfo)
	return targetDTO
}

func OpenAPIEvalTargetVersionDO2DTO(versionDO *entity.EvalTargetVersion, typ entity.EvalTargetType) *openapiEvalTarget.EvalTargetVersion {
	if versionDO == nil {
		return nil
	}

	versionDTO := &openapiEvalTarget.EvalTargetVersion{
		ID:                  gptr.Of(versionDO.ID),
		TargetID:            gptr.Of(versionDO.TargetID),
		SourceTargetVersion: gptr.Of(versionDO.SourceTargetVersion),
	}

	contentDTO := &openapiEvalTarget.EvalTargetContent{
		InputSchemas:  common.OpenAPIArgsSchemaDO2DTOs(versionDO.InputSchema),
		OutputSchemas: common.OpenAPIArgsSchemaDO2DTOs(versionDO.OutputSchema),
	}
	if versionDO.RuntimeParamDemo != nil {
		contentDTO.RuntimeParamJSONDemo = versionDO.RuntimeParamDemo
	}

	switch typ {
	case entity.EvalTargetTypeLoopPrompt:
		if versionDO.Prompt != nil {
			contentDTO.Prompt = &openapiEvalTarget.EvalPrompt{
				PromptID:     gptr.Of(versionDO.Prompt.PromptID),
				Version:      gptr.Of(versionDO.Prompt.Version),
				Name:         gptr.Of(versionDO.Prompt.Name),
				PromptKey:    gptr.Of(versionDO.Prompt.PromptKey),
				SubmitStatus: gptr.Of(mapEntitySubmitStatusToOpenAPI(versionDO.Prompt.SubmitStatus)),
				Description:  gptr.Of(versionDO.Prompt.Description),
			}
		}
	case entity.EvalTargetTypeCustomRPCServer:
		if versionDO.CustomRPCServer != nil {
			contentDTO.CustomRPCServer = OpenAPICustomRPCServerDO2DTO(versionDO.CustomRPCServer)
		}
	}

	versionDTO.EvalTargetContent = contentDTO
	versionDTO.BaseInfo = common.OpenAPIBaseInfoDO2DTO(versionDO.BaseInfo)

	return versionDTO
}

func mapEntitySubmitStatusToOpenAPI(status entity.SubmitStatus) openapiEvalTarget.SubmitStatus {
	switch status {
	case entity.SubmitStatus_UnSubmit:
		return openapiEvalTarget.SubmitStatusUnSubmit
	case entity.SubmitStatus_Submitted:
		return openapiEvalTarget.SubmitStatusSubmitted
	default:
		return ""
	}
}

func convertEntityEvalTargetTypeToOpenAPI(typ entity.EvalTargetType) openapiEvalTarget.EvalTargetType {
	switch typ {
	case entity.EvalTargetTypeCozeBot:
		return openapiEvalTarget.EvalTargetTypeCozeBot
	case entity.EvalTargetTypeLoopPrompt:
		return openapiEvalTarget.EvalTargetTypeCozeLoopPrompt
	case entity.EvalTargetTypeLoopTrace:
		return openapiEvalTarget.EvalTargetTypeTrace
	case entity.EvalTargetTypeCozeWorkflow:
		return openapiEvalTarget.EvalTargetTypeCozeWorkflow
	case entity.EvalTargetTypeVolcengineAgent:
		return openapiEvalTarget.EvalTargetTypeVolcengineAgent
	case entity.EvalTargetTypeCustomRPCServer:
		return openapiEvalTarget.EvalTargetTypeCustomRPCServer
	default:
		return ""
	}
}

func OpenAPICustomRPCServerDO2DTO(do *entity.CustomRPCServer) *openapiEvalTarget.CustomRPCServer {
	if do == nil {
		return nil
	}
	res := &openapiEvalTarget.CustomRPCServer{
		ID:                  gptr.Of(do.ID),
		Name:                gptr.Of(do.Name),
		Description:         gptr.Of(do.Description),
		ServerName:          gptr.Of(do.ServerName),
		AccessProtocol:      gptr.Of(do.AccessProtocol),
		Cluster:             gptr.Of(do.Cluster),
		InvokeHTTPInfo:      OpenAPIHTTPInfoDO2DTO(do.InvokeHTTPInfo),
		AsyncInvokeHTTPInfo: OpenAPIHTTPInfoDO2DTO(do.AsyncInvokeHTTPInfo),
		NeedSearchTarget:    do.NeedSearchTarget,
		SearchHTTPInfo:      OpenAPIHTTPInfoDO2DTO(do.SearchHTTPInfo),
		CustomEvalTarget:    OpenAPICustomEvalTargetDO2DTO(do.CustomEvalTarget),
		IsAsync:             do.IsAsync,
		ExecRegion:          gptr.Of(do.ExecRegion),
		ExecEnv:             do.ExecEnv,
		Timeout:             do.Timeout,
		AsyncTimeout:        do.AsyncTimeout,
		Ext:                 do.Ext,
	}
	res.Regions = append(res.Regions, do.Regions...)
	return res
}

func OpenAPIHTTPInfoDO2DTO(do *entity.HTTPInfo) *openapiEvalTarget.HTTPInfo {
	if do == nil {
		return nil
	}
	return &openapiEvalTarget.HTTPInfo{
		Method: gptr.Of(do.Method),
		Path:   gptr.Of(do.Path),
	}
}

func OpenAPICustomEvalTargetDO2DTO(do *entity.CustomEvalTarget) *openapiEvalTarget.CustomEvalTarget {
	if do == nil {
		return nil
	}
	return &openapiEvalTarget.CustomEvalTarget{
		ID:        do.ID,
		Name:      do.Name,
		AvatarURL: do.AvatarURL,
		Ext:       do.Ext,
	}
}

func OpenAPIExptTemplateDO2DTO(template *entity.ExptTemplate) *openapiExperiment.ExptTemplate {
	if template == nil {
		return nil
	}

	dto := &openapiExperiment.ExptTemplate{
		Meta: &openapiExperiment.ExptTemplateMeta{
			ID:          gptr.Of(template.Meta.ID),
			WorkspaceID: gptr.Of(template.Meta.WorkspaceID),
			Name:        gptr.Of(template.Meta.Name),
			Description: gptr.Of(template.Meta.Desc),
			ExptType:    OpenAPIExptTypeDO2DTO(template.Meta.ExptType),
		},
		BaseInfo: common.OpenAPIBaseInfoDO2DTO(template.BaseInfo),
	}

	if template.TripleConfig != nil {
		dto.TripleConfig = &openapiExperiment.ExptTuple{
			EvalSetID:        gptr.Of(template.TripleConfig.EvalSetID),
			EvalSetVersionID: gptr.Of(template.TripleConfig.EvalSetVersionID),
			TargetID:         gptr.Of(template.TripleConfig.TargetID),
			TargetVersionID:  gptr.Of(template.TripleConfig.TargetVersionID),
		}
		// run_config、score_weight、version 来源于 TemplateConf.ConnectorConf.EvaluatorsConf.EvaluatorConf，按 EvaluatorVersionID 映射后填入 DTO（从 DB 加载时 item 可能无这些字段）
		runConfByVersionID := make(map[int64]*entity.EvaluatorRunConfig)
		scoreWeightByVersionID := make(map[int64]float64)
		versionByVersionID := make(map[int64]string)
		if template.TemplateConf != nil && template.TemplateConf.ConnectorConf.EvaluatorsConf != nil {
			for _, ec := range template.TemplateConf.ConnectorConf.EvaluatorsConf.EvaluatorConf {
				if ec == nil || ec.EvaluatorVersionID <= 0 {
					continue
				}
				if ec.RunConf != nil {
					runConfByVersionID[ec.EvaluatorVersionID] = ec.RunConf
				}
				if ec.ScoreWeight != nil && *ec.ScoreWeight > 0 {
					scoreWeightByVersionID[ec.EvaluatorVersionID] = *ec.ScoreWeight
				}
				if ec.Version != "" {
					versionByVersionID[ec.EvaluatorVersionID] = ec.Version
				}
			}
		}
		for _, item := range template.TripleConfig.EvaluatorIDVersionItems {
			if item == nil {
				continue
			}
			scoreWeight := item.ScoreWeight
			if scoreWeight <= 0 && item.EvaluatorVersionID > 0 {
				scoreWeight = scoreWeightByVersionID[item.EvaluatorVersionID]
			}
			version := item.Version
			if version == "" && item.EvaluatorVersionID > 0 {
				version = versionByVersionID[item.EvaluatorVersionID]
			}
			dto.TripleConfig.EvaluatorIDVersionItems = append(dto.TripleConfig.EvaluatorIDVersionItems, &openapiEvaluator.EvaluatorIDVersionItem{
				EvaluatorID:        gptr.Of(item.EvaluatorID),
				Version:            gptr.Of(version),
				EvaluatorVersionID: gptr.Of(item.EvaluatorVersionID),
				ScoreWeight:        gptr.Of(scoreWeight),
				RunConfig:          evaluator_convertor.OpenAPIEvaluatorRunConfigDO2DTO(runConfByVersionID[item.EvaluatorVersionID]),
			})
		}
	}

	if template.FieldMappingConfig != nil {
		dto.FieldMappingConfig = &openapiExperiment.ExptFieldMapping{
			ItemConcurNum: ptr.ConvIntPtr[int, int32](template.FieldMappingConfig.ItemConcurNum),
		}
		if template.FieldMappingConfig.TargetFieldMapping != nil {
			dto.FieldMappingConfig.TargetFieldMapping = DomainTargetFieldMappingDTO2OpenAPI(&domainExpt.TargetFieldMapping{
				FromEvalSet: slices.Transform(template.FieldMappingConfig.TargetFieldMapping.FromEvalSet, func(e *entity.ExptTemplateFieldMapping, _ int) *domainExpt.FieldMapping {
					return &domainExpt.FieldMapping{FieldName: gptr.Of(e.FieldName), FromFieldName: gptr.Of(e.FromFieldName)}
				}),
			})
		}
		if template.FieldMappingConfig.TargetRuntimeParam != nil {
			dto.FieldMappingConfig.TargetRuntimeParam = &openapiCommon.RuntimeParam{
				JSONValue: template.FieldMappingConfig.TargetRuntimeParam.JSONValue,
			}
		}
		// 按 evaluator_version_id 从 TripleConfig 回填 evaluator_id / version（兼容从 DB 加载时仅含 EvaluatorVersionID 的情况）
		evaluatorIDByVersionID := make(map[int64]int64)
		versionByVersionID := make(map[int64]string)
		if template.TripleConfig != nil {
			for _, item := range template.TripleConfig.EvaluatorIDVersionItems {
				if item != nil && item.EvaluatorVersionID > 0 {
					evaluatorIDByVersionID[item.EvaluatorVersionID] = item.EvaluatorID
					versionByVersionID[item.EvaluatorVersionID] = item.Version
				}
			}
		}
		for _, em := range template.FieldMappingConfig.EvaluatorFieldMapping {
			evaluatorID, version := em.EvaluatorID, em.Version
			if (evaluatorID == 0 || version == "") && em.EvaluatorVersionID > 0 {
				if id, ok := evaluatorIDByVersionID[em.EvaluatorVersionID]; ok {
					evaluatorID = id
				}
				if v, ok := versionByVersionID[em.EvaluatorVersionID]; ok {
					version = v
				}
			}
			m := &openapiExperiment.EvaluatorFieldMapping{
				EvaluatorID: gptr.Of(evaluatorID),
				Version:     gptr.Of(version),
			}
			for _, fm := range em.FromEvalSet {
				m.FromEvalSet = append(m.FromEvalSet, &openapiExperiment.FieldMapping{
					FieldName:     gptr.Of(fm.FieldName),
					FromFieldName: gptr.Of(fm.FromFieldName),
				})
			}
			for _, fm := range em.FromTarget {
				m.FromTarget = append(m.FromTarget, &openapiExperiment.FieldMapping{
					FieldName:     gptr.Of(fm.FieldName),
					FromFieldName: gptr.Of(fm.FromFieldName),
				})
			}
			dto.FieldMappingConfig.EvaluatorFieldMapping = append(dto.FieldMappingConfig.EvaluatorFieldMapping, m)
		}
	}

	dto.ScoreWeightConfig = buildOpenAPIExptScoreWeightFromTemplate(template)
	return dto
}

// buildOpenAPIExptScoreWeightFromTemplate 从 entity.ExptTemplate 抽取评估器权重配置，转为 openapi ExptScoreWeight（与 expt_template.buildTemplateScoreWeightConfigDTO 逻辑一致）
func buildOpenAPIExptScoreWeightFromTemplate(template *entity.ExptTemplate) *openapiExperiment.ExptScoreWeight {
	evaluatorScoreWeights := buildScoreWeightsFromTemplateConf(template)
	if len(evaluatorScoreWeights) == 0 &&
		template.TripleConfig != nil && len(template.TripleConfig.EvaluatorIDVersionItems) > 0 {
		evaluatorScoreWeights = make(map[int64]float64)
		for _, item := range template.TripleConfig.EvaluatorIDVersionItems {
			if item == nil || item.EvaluatorVersionID <= 0 || item.ScoreWeight <= 0 {
				continue
			}
			evaluatorScoreWeights[item.EvaluatorVersionID] = item.ScoreWeight
		}
	}
	hasWeightedScore := len(evaluatorScoreWeights) > 0
	if template.TemplateConf != nil && template.TemplateConf.ConnectorConf.EvaluatorsConf != nil {
		hasWeightedScore = hasWeightedScore || template.TemplateConf.ConnectorConf.EvaluatorsConf.EnableScoreWeight
	}
	if !hasWeightedScore {
		return nil
	}
	return &openapiExperiment.ExptScoreWeight{
		EnableWeightedScore:   gptr.Of(hasWeightedScore),
		EvaluatorScoreWeights: evaluatorScoreWeights,
	}
}

func OpenAPIExptTemplateDO2DTOs(templates []*entity.ExptTemplate) []*openapiExperiment.ExptTemplate {
	if len(templates) == 0 {
		return nil
	}
	dtos := make([]*openapiExperiment.ExptTemplate, 0, len(templates))
	for _, t := range templates {
		dtos = append(dtos, OpenAPIExptTemplateDO2DTO(t))
	}
	return dtos
}

// OpenAPITemplateToSubmitExperimentRequest 将实验模板转换为 SubmitExperimentRequest（用于 SubmitExptFromTemplateOApi）
// 与 OpenAPIExptTemplateDO2DTO 逻辑一致：从 TemplateConf.EvaluatorConf 构建 runConf/scoreWeight/version 映射后填充
func OpenAPITemplateToSubmitExperimentRequest(template *entity.ExptTemplate, name string, workspaceID int64) *expt.SubmitExperimentRequest {
	req := TemplateToSubmitExperimentRequest(template, name, workspaceID)
	if req == nil {
		return nil
	}
	// 从 TemplateConf 构建 runConf/scoreWeight/version 映射（与 BatchGet 一致）
	runConfByVersionID, scoreWeightByVersionID, versionByVersionID := buildOpenAPITemplateConfMaps(template)

	// 填充 EvaluatorIDVersionList
	if items := req.GetEvaluatorIDVersionList(); len(items) > 0 {
		for _, item := range items {
			if item == nil || item.GetEvaluatorVersionID() <= 0 {
				continue
			}
			verID := item.GetEvaluatorVersionID()
			// version
			if item.GetVersion() == "" && versionByVersionID != nil {
				if v := versionByVersionID[verID]; v != "" {
					item.SetVersion(gptr.Of(v))
				}
			}
			// scoreWeight
			if (!item.IsSetScoreWeight() || item.GetScoreWeight() <= 0) && scoreWeightByVersionID != nil {
				if w := scoreWeightByVersionID[verID]; w > 0 {
					item.SetScoreWeight(gptr.Of(w))
				}
			}
			// RunConfig
			if !item.IsSetRunConfig() && runConfByVersionID != nil {
				if rc := runConfByVersionID[verID]; rc != nil {
					item.SetRunConfig(entityRunConfToDomainEvaluator(rc))
				}
			}
		}
	}

	// 填充 EvaluatorFieldMapping 中的 EvaluatorIDVersionItem
	if fm := req.GetEvaluatorFieldMapping(); len(fm) > 0 {
		for _, m := range fm {
			if m == nil || m.GetEvaluatorVersionID() <= 0 {
				continue
			}
			item := m.GetEvaluatorIDVersionItem()
			if item == nil {
				continue
			}
			verID := m.GetEvaluatorVersionID()
			if item.GetVersion() == "" && versionByVersionID != nil {
				if v := versionByVersionID[verID]; v != "" {
					item.SetVersion(gptr.Of(v))
				}
			}
			if (!item.IsSetScoreWeight() || item.GetScoreWeight() <= 0) && scoreWeightByVersionID != nil {
				if w := scoreWeightByVersionID[verID]; w > 0 {
					item.SetScoreWeight(gptr.Of(w))
				}
			}
			if !item.IsSetRunConfig() && runConfByVersionID != nil {
				if rc := runConfByVersionID[verID]; rc != nil {
					item.SetRunConfig(entityRunConfToDomainEvaluator(rc))
				}
			}
		}
	}

	return req
}

// buildOpenAPITemplateConfMaps 从 TemplateConf.EvaluatorConf 构建 runConf/scoreWeight/version 映射（与 OpenAPIExptTemplateDO2DTO 一致）
func buildOpenAPITemplateConfMaps(template *entity.ExptTemplate) (
	runConfByVersionID map[int64]*entity.EvaluatorRunConfig,
	scoreWeightByVersionID map[int64]float64,
	versionByVersionID map[int64]string,
) {
	if template == nil || template.TemplateConf == nil ||
		template.TemplateConf.ConnectorConf.EvaluatorsConf == nil {
		return nil, nil, nil
	}
	for _, ec := range template.TemplateConf.ConnectorConf.EvaluatorsConf.EvaluatorConf {
		if ec == nil || ec.EvaluatorVersionID <= 0 {
			continue
		}
		if ec.RunConf != nil {
			if runConfByVersionID == nil {
				runConfByVersionID = make(map[int64]*entity.EvaluatorRunConfig)
			}
			runConfByVersionID[ec.EvaluatorVersionID] = ec.RunConf
		}
		if ec.ScoreWeight != nil && *ec.ScoreWeight > 0 {
			if scoreWeightByVersionID == nil {
				scoreWeightByVersionID = make(map[int64]float64)
			}
			scoreWeightByVersionID[ec.EvaluatorVersionID] = *ec.ScoreWeight
		}
		if ec.Version != "" {
			if versionByVersionID == nil {
				versionByVersionID = make(map[int64]string)
			}
			versionByVersionID[ec.EvaluatorVersionID] = ec.Version
		}
	}
	return runConfByVersionID, scoreWeightByVersionID, versionByVersionID
}

// entityRunConfToDomainEvaluator 将 entity.EvaluatorRunConfig 转为 domainEvaluator.EvaluatorRunConfig
func entityRunConfToDomainEvaluator(rc *entity.EvaluatorRunConfig) *domainEvaluator.EvaluatorRunConfig {
	if rc == nil {
		return nil
	}
	dto := domainEvaluator.NewEvaluatorRunConfig()
	dto.Env = rc.Env
	if rc.EvaluatorRuntimeParam != nil {
		dto.EvaluatorRuntimeParam = &domainCommon.RuntimeParam{JSONValue: rc.EvaluatorRuntimeParam.JSONValue}
	}
	return dto
}

func OpenAPICreateExptTemplateReq2Domain(req *openapi.CreateExptTemplateOApiRequest) (*entity.CreateExptTemplateParam, error) {
	if req == nil {
		return nil, nil
	}
	param := &entity.CreateExptTemplateParam{
		SpaceID:               req.GetWorkspaceID(),
		CreateEvalTargetParam: OpenAPICreateEvalTargetParamDTO2DomainV2(req.GetCreateEvalTargetParam()),
	}

	if req.GetMeta() != nil {
		meta := req.GetMeta()
		param.Name = meta.GetName()
		param.Description = meta.GetDescription()
		param.ExptType = OpenAPIExptTypeDTO2DO(meta.ExptType)
	}

	if req.GetTripleConfig() != nil {
		tc := req.GetTripleConfig()
		param.EvalSetID = tc.GetEvalSetID()
		param.EvalSetVersionID = tc.GetEvalSetVersionID()
		param.TargetID = tc.GetTargetID()
		param.TargetVersionID = tc.GetTargetVersionID()
		param.EvaluatorIDVersionItems = openAPIExptTupleEvaluatorItemsToEntity(tc)
	}

	if req.GetFieldMappingConfig() != nil {
		fmc := req.GetFieldMappingConfig()
		var rtp *entity.RuntimeParam
		if fmc.TargetRuntimeParam != nil {
			rtp = &entity.RuntimeParam{JSONValue: fmc.TargetRuntimeParam.JSONValue}
		}
		param.TemplateConf = &entity.ExptTemplateConfiguration{
			ItemConcurNum:       ptr.ConvIntPtr[int32, int](fmc.ItemConcurNum),
			EvaluatorsConcurNum: ptr.ConvIntPtr[int32, int](req.DefaultEvaluatorsConcurNum),
			ConnectorConf: entity.Connector{
				TargetConf: &entity.TargetConf{
					TargetVersionID: param.TargetVersionID,
					IngressConf:     toTargetFieldMappingDOForTemplateV2(fmc.TargetFieldMapping, rtp),
				},
			},
		}
		tc := req.GetTripleConfig()
		for i, em := range fmc.EvaluatorFieldMapping {
			if em == nil {
				continue
			}
			ec := &entity.EvaluatorConf{
				EvaluatorID: em.GetEvaluatorID(),
				Version:     em.GetVersion(),
				IngressConf: &entity.EvaluatorIngressConf{
					EvalSetAdapter: &entity.FieldAdapter{},
					TargetAdapter:  &entity.FieldAdapter{},
				},
			}
			// 与 triple_config.evaluator_id_version_items 按索引对齐：若有则用其补全 id/version（便于 service 层 resolveAndFillEvaluatorVersionIDs 用 (id,version) 解析并回填 evaluator_version_id）、run_config、score_weight（写入 EvaluatorConf 后随 template_conf 落库）
			if tc != nil && i < len(tc.EvaluatorIDVersionItems) && tc.EvaluatorIDVersionItems[i] != nil {
				item := tc.EvaluatorIDVersionItems[i]
				if ec.EvaluatorID == 0 && item.GetEvaluatorID() != 0 {
					ec.EvaluatorID = item.GetEvaluatorID()
				}
				if ec.Version == "" && item.GetVersion() != "" {
					ec.Version = item.GetVersion()
				}
				if item.GetEvaluatorVersionID() > 0 {
					ec.EvaluatorVersionID = item.GetEvaluatorVersionID()
				}
				if item.GetRunConfig() != nil {
					ec.RunConf = evaluator_convertor.OpenAPIEvaluatorRunConfigDTO2DO(item.GetRunConfig())
				}
				if item.IsSetScoreWeight() {
					ec.ScoreWeight = gptr.Of(item.GetScoreWeight())
				}
			}
			for _, fm := range em.FromEvalSet {
				ec.IngressConf.EvalSetAdapter.FieldConfs = append(ec.IngressConf.EvalSetAdapter.FieldConfs, &entity.FieldConf{
					FieldName: fm.GetFieldName(),
					FromField: fm.GetFromFieldName(),
				})
			}
			for _, fm := range em.FromTarget {
				ec.IngressConf.TargetAdapter.FieldConfs = append(ec.IngressConf.TargetAdapter.FieldConfs, &entity.FieldConf{
					FieldName: fm.GetFieldName(),
					FromField: fm.GetFromFieldName(),
				})
			}
			if param.TemplateConf.ConnectorConf.EvaluatorsConf == nil {
				param.TemplateConf.ConnectorConf.EvaluatorsConf = &entity.EvaluatorsConf{}
			}
			param.TemplateConf.ConnectorConf.EvaluatorsConf.EvaluatorConf = append(param.TemplateConf.ConnectorConf.EvaluatorsConf.EvaluatorConf, ec)
		}
	}

	return param, nil
}

func OpenAPIUpdateExptTemplateReq2Domain(req *openapi.UpdateExptTemplateOApiRequest) (*entity.UpdateExptTemplateParam, error) {
	if req == nil {
		return nil, nil
	}
	param := &entity.UpdateExptTemplateParam{
		TemplateID:            req.GetTemplateID(),
		SpaceID:               req.GetWorkspaceID(),
		CreateEvalTargetParam: OpenAPICreateEvalTargetParamDTO2DomainV2(req.GetCreateEvalTargetParam()),
	}

	if req.GetMeta() != nil {
		meta := req.GetMeta()
		param.Name = meta.GetName()
		param.Description = meta.GetDescription()
		param.ExptType = OpenAPIExptTypeDTO2DO(meta.ExptType)
	}

	if req.GetTripleConfig() != nil {
		tc := req.GetTripleConfig()
		param.EvalSetVersionID = tc.GetEvalSetVersionID()
		param.TargetVersionID = tc.GetTargetVersionID()
		param.EvaluatorIDVersionItems = openAPIExptTupleEvaluatorItemsToEntity(tc)
	}

	if req.GetFieldMappingConfig() != nil {
		fmc := req.GetFieldMappingConfig()
		var rtp *entity.RuntimeParam
		if fmc.TargetRuntimeParam != nil {
			rtp = &entity.RuntimeParam{JSONValue: fmc.TargetRuntimeParam.JSONValue}
		}
		param.TemplateConf = &entity.ExptTemplateConfiguration{
			ItemConcurNum:       ptr.ConvIntPtr[int32, int](fmc.ItemConcurNum),
			EvaluatorsConcurNum: ptr.ConvIntPtr[int32, int](req.DefaultEvaluatorsConcurNum),
			ConnectorConf: entity.Connector{
				TargetConf: &entity.TargetConf{
					TargetVersionID: param.TargetVersionID,
					IngressConf:     toTargetFieldMappingDOForTemplateV2(fmc.TargetFieldMapping, rtp),
				},
			},
		}
		tc := req.GetTripleConfig()
		for i, em := range fmc.EvaluatorFieldMapping {
			if em == nil {
				continue
			}
			ec := &entity.EvaluatorConf{
				EvaluatorID: em.GetEvaluatorID(),
				Version:     em.GetVersion(),
				IngressConf: &entity.EvaluatorIngressConf{
					EvalSetAdapter: &entity.FieldAdapter{},
					TargetAdapter:  &entity.FieldAdapter{},
				},
			}
			// 与 triple_config.evaluator_id_version_items 按索引对齐：若有则用其补全 id/version、run_config、score_weight（写入 EvaluatorConf 后随 template_conf 落库）
			if tc != nil && i < len(tc.EvaluatorIDVersionItems) && tc.EvaluatorIDVersionItems[i] != nil {
				item := tc.EvaluatorIDVersionItems[i]
				if ec.EvaluatorID == 0 && item.GetEvaluatorID() != 0 {
					ec.EvaluatorID = item.GetEvaluatorID()
				}
				if ec.Version == "" && item.GetVersion() != "" {
					ec.Version = item.GetVersion()
				}
				if item.GetEvaluatorVersionID() > 0 {
					ec.EvaluatorVersionID = item.GetEvaluatorVersionID()
				}
				if item.GetRunConfig() != nil {
					ec.RunConf = evaluator_convertor.OpenAPIEvaluatorRunConfigDTO2DO(item.GetRunConfig())
				}
				if item.IsSetScoreWeight() {
					ec.ScoreWeight = gptr.Of(item.GetScoreWeight())
				}
			}
			for _, fm := range em.FromEvalSet {
				ec.IngressConf.EvalSetAdapter.FieldConfs = append(ec.IngressConf.EvalSetAdapter.FieldConfs, &entity.FieldConf{
					FieldName: fm.GetFieldName(),
					FromField: fm.GetFromFieldName(),
				})
			}
			for _, fm := range em.FromTarget {
				ec.IngressConf.TargetAdapter.FieldConfs = append(ec.IngressConf.TargetAdapter.FieldConfs, &entity.FieldConf{
					FieldName: fm.GetFieldName(),
					FromField: fm.GetFromFieldName(),
				})
			}
			if param.TemplateConf.ConnectorConf.EvaluatorsConf == nil {
				param.TemplateConf.ConnectorConf.EvaluatorsConf = &entity.EvaluatorsConf{}
			}
			param.TemplateConf.ConnectorConf.EvaluatorsConf.EvaluatorConf = append(param.TemplateConf.ConnectorConf.EvaluatorsConf.EvaluatorConf, ec)
		}
	}

	return param, nil
}

func OpenAPIExptTypeDO2DTO(t entity.ExptType) *openapiExperiment.ExperimentType {
	var s openapiExperiment.ExperimentType
	switch t {
	case entity.ExptType_Offline:
		s = openapiExperiment.ExperimentTypeOffline
	case entity.ExptType_Online:
		s = openapiExperiment.ExperimentTypeOnline
	default:
		return nil
	}
	return &s
}

func OpenAPIExptTypeDTO2DO(t *openapiExperiment.ExperimentType) entity.ExptType {
	if t == nil {
		return entity.ExptType_Offline
	}
	switch *t {
	case openapiExperiment.ExperimentTypeOffline:
		return entity.ExptType_Offline
	case openapiExperiment.ExperimentTypeOnline:
		return entity.ExptType_Online
	default:
		return entity.ExptType_Offline
	}
}

// parseExptTypeFromString 将字符串解析为 entity.ExptType，支持 "offline"/"online" 或 "1"/"2"
func parseExptTypeFromString(s string) (entity.ExptType, bool) {
	switch s {
	case "offline", "1":
		return entity.ExptType_Offline, true
	case "online", "2":
		return entity.ExptType_Online, true
	default:
		return entity.ExptType_Offline, false
	}
}

// isIncludeOperator 判断操作符是否表示包含（IN/EQ 等）
func isOpenAPIIncludeOperator(op string) bool {
	switch op {
	case "in", "eq", "equal", "=", "IN", "EQ", "EQUAL":
		return true
	default:
		return false
	}
}

// isExcludeOperator 判断操作符是否表示排除（NOT_IN/NE 等）
func isOpenAPIExcludeOperator(op string) bool {
	switch op {
	case "not_in", "ne", "not_equal", "!=", "NOT_IN", "NE", "NOT_EQUAL":
		return true
	default:
		return false
	}
}

// OpenAPIExptTemplateFilterDTO2DO 将 OpenAPI 实验模板筛选器转换为 entity.ExptTemplateListFilter（与 domain/expt 结构一致）
func OpenAPIExptTemplateFilterDTO2DO(dto *openapiExperiment.ExperimentTemplateFilter) *entity.ExptTemplateListFilter {
	if dto == nil {
		return nil
	}
	result := &entity.ExptTemplateListFilter{
		Includes: &entity.ExptTemplateFilterFields{},
		Excludes: &entity.ExptTemplateFilterFields{},
	}

	// KeywordSearch.keyword -> FuzzyName（与 domain/expt 一致）
	if dto.KeywordSearch != nil && dto.KeywordSearch.Keyword != nil {
		if k := strings.TrimSpace(*dto.KeywordSearch.Keyword); k != "" {
			result.FuzzyName = k
		}
	}

	// Filters
	filters := dto.Filters
	if filters == nil || len(filters.GetFilterConditions()) == 0 {
		if result.FuzzyName == "" && !result.Includes.IsValid() && !result.Excludes.IsValid() {
			return nil
		}
		return result
	}
	if filters.LogicOp != nil && strings.ToLower(*filters.LogicOp) != "and" {
		return nil
	}

	parseInt64List := func(s string) ([]int64, bool) {
		var ids []int64
		for _, p := range strings.Split(s, ",") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			v, err := strconv.ParseInt(p, 10, 64)
			if err != nil {
				return nil, false
			}
			ids = append(ids, v)
		}
		return ids, len(ids) > 0
	}
	parseStringList := func(s string) []string {
		var parts []string
		for _, p := range strings.Split(s, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				parts = append(parts, p)
			}
		}
		return parts
	}

	for _, cond := range filters.GetFilterConditions() {
		if cond == nil || cond.GetField() == nil {
			continue
		}
		fieldType := strings.TrimSpace(strings.ToLower(cond.GetField().GetFieldType()))
		operator := strings.TrimSpace(strings.ToLower(cond.GetOperator()))
		value := strings.TrimSpace(cond.GetValue())

		// name 支持任意操作符，直接作为模糊搜索
		if fieldType == "name" {
			result.FuzzyName = value
			continue
		}

		var targetIncludes, targetExcludes *entity.ExptTemplateFilterFields
		if isOpenAPIIncludeOperator(operator) {
			targetIncludes, targetExcludes = result.Includes, nil
		} else if isOpenAPIExcludeOperator(operator) {
			targetIncludes, targetExcludes = nil, result.Excludes
		} else {
			continue
		}

		ff := targetIncludes
		if ff == nil {
			ff = targetExcludes
		}
		if ff == nil {
			continue
		}

		switch fieldType {
		case "expt_type":
			for _, part := range strings.Split(value, ",") {
				part = strings.TrimSpace(part)
				if et, ok := parseExptTypeFromString(part); ok {
					ff.ExptType = append(ff.ExptType, int64(et))
				}
			}
		case "eval_set_id":
			if ids, ok := parseInt64List(value); ok {
				ff.EvalSetIDs = append(ff.EvalSetIDs, ids...)
			}
		case "target_id":
			if ids, ok := parseInt64List(value); ok {
				ff.TargetIDs = append(ff.TargetIDs, ids...)
			}
		case "evaluator_id":
			if ids, ok := parseInt64List(value); ok {
				ff.EvaluatorIDs = append(ff.EvaluatorIDs, ids...)
			}
		case "target_type":
			if ids, ok := parseInt64List(value); ok {
				ff.TargetType = append(ff.TargetType, ids...)
			}
		case "creator_by":
			if ss := parseStringList(value); len(ss) > 0 {
				ff.CreatedBy = append(ff.CreatedBy, ss...)
			}
		case "updated_by":
			if ss := parseStringList(value); len(ss) > 0 {
				ff.UpdatedBy = append(ff.UpdatedBy, ss...)
			}
		}
	}

	if result.FuzzyName == "" && !result.Includes.IsValid() && !result.Excludes.IsValid() {
		return nil
	}
	return result
}

func OpenAPICreateEvalTargetParamDTO2DomainV2(param *openapi.SubmitExperimentEvalTargetParam) *entity.CreateEvalTargetParam {
	if param == nil {
		return nil
	}

	res := &entity.CreateEvalTargetParam{
		SourceTargetID:      param.SourceTargetID,
		SourceTargetVersion: param.SourceTargetVersion,
		BotPublishVersion:   param.BotPublishVersion,
		Env:                 param.Env,
	}
	if param.EvalTargetType != nil {
		val, err := mapOpenAPIEvalTargetType(*param.EvalTargetType)
		if err == nil {
			res.EvalTargetType = gptr.Of(entity.EvalTargetType(val))
		}
	}
	if param.BotInfoType != nil {
		val, err := mapOpenAPICozeBotInfoType(*param.BotInfoType)
		if err == nil {
			res.BotInfoType = gptr.Of(entity.CozeBotInfoType(val))
		}
	}
	if param.Region != nil {
		val, err := mapOpenAPIRegion(*param.Region)
		if err == nil {
			res.Region = gptr.Of(val)
		}
	}
	if param.CustomEvalTarget != nil {
		res.CustomEvalTarget = &entity.CustomEvalTarget{
			ID:        param.CustomEvalTarget.ID,
			Name:      param.CustomEvalTarget.Name,
			AvatarURL: param.CustomEvalTarget.AvatarURL,
			Ext:       param.CustomEvalTarget.Ext,
		}
	}
	return res
}

func toTargetFieldMappingDOForTemplateV2(mapping *openapiExperiment.TargetFieldMapping, rtp *entity.RuntimeParam) *entity.TargetIngressConf {
	tic := &entity.TargetIngressConf{EvalSetAdapter: &entity.FieldAdapter{}}

	if mapping != nil {
		fc := make([]*entity.FieldConf, 0, len(mapping.GetFromEvalSet()))
		for _, fm := range mapping.GetFromEvalSet() {
			fc = append(fc, &entity.FieldConf{
				FieldName: fm.GetFieldName(),
				FromField: fm.GetFromFieldName(),
			})
		}
		tic.EvalSetAdapter.FieldConfs = fc
	}

	if rtp != nil && rtp.JSONValue != nil && len(*rtp.JSONValue) > 0 {
		tic.CustomConf = &entity.FieldAdapter{
			FieldConfs: []*entity.FieldConf{{
				FieldName: consts.FieldAdapterBuiltinFieldNameRuntimeParam,
				Value:     *rtp.JSONValue,
			}},
		}
	}
	return tic
}
