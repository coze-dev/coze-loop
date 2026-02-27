// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"strings"

	"github.com/bytedance/gg/gptr"
	openapiEvaluator "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/evaluator"
	common_convertor "github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/common"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func OpenAPIEvaluatorDO2DTO(do *entity.Evaluator) *openapiEvaluator.Evaluator {
	if do == nil {
		return nil
	}
	dto := &openapiEvaluator.Evaluator{
		ID:               gptr.Of(do.ID),
		WorkspaceID:      gptr.Of(do.SpaceID),
		Name:             gptr.Of(do.Name),
		Description:      gptr.Of(do.Description),
		EvaluatorType:    OpenAPIEvaluatorTypeDO2DTO(do.EvaluatorType),
		IsDraftSubmitted: gptr.Of(do.DraftSubmitted),
		LatestVersion:    gptr.Of(do.LatestVersion),
		BaseInfo:         common_convertor.OpenAPIBaseInfoDO2DTO(do.BaseInfo),
	}

	dto.CurrentVersion = OpenAPIEvaluatorVersionDO2DTO(do)

	return dto
}

func OpenAPIEvaluatorDO2DTOs(dos []*entity.Evaluator) []*openapiEvaluator.Evaluator {
	if len(dos) == 0 {
		return nil
	}
	dtos := make([]*openapiEvaluator.Evaluator, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		dtos = append(dtos, OpenAPIEvaluatorDO2DTO(do))
	}
	return dtos
}

func OpenAPIEvaluatorTypeDO2DTO(doType entity.EvaluatorType) *openapiEvaluator.EvaluatorType {
	var openapiType openapiEvaluator.EvaluatorType
	switch doType {
	case entity.EvaluatorTypePrompt:
		openapiType = openapiEvaluator.EvaluatorTypePrompt
	case entity.EvaluatorTypeCode:
		openapiType = openapiEvaluator.EvaluatorTypeCode
	case entity.EvaluatorTypeCustomRPC:
		openapiType = openapiEvaluator.EvaluatorTypeCustomRPC
	default:
		return nil
	}
	return &openapiType
}

func OpenAPIEvaluatorVersionDO2DTO(do *entity.Evaluator) *openapiEvaluator.EvaluatorVersion {
	if do == nil {
		return nil
	}
	var id int64
	var version string
	var description string
	var baseInfo *entity.BaseInfo

	switch do.EvaluatorType {
	case entity.EvaluatorTypePrompt:
		if do.PromptEvaluatorVersion != nil {
			id = do.PromptEvaluatorVersion.ID
			version = do.PromptEvaluatorVersion.Version
			description = do.PromptEvaluatorVersion.Description
			baseInfo = do.PromptEvaluatorVersion.BaseInfo
		}
	case entity.EvaluatorTypeCode:
		if do.CodeEvaluatorVersion != nil {
			id = do.CodeEvaluatorVersion.ID
			version = do.CodeEvaluatorVersion.Version
			description = do.CodeEvaluatorVersion.Description
			baseInfo = do.CodeEvaluatorVersion.BaseInfo
		}
	case entity.EvaluatorTypeCustomRPC:
		if do.CustomRPCEvaluatorVersion != nil {
			id = do.CustomRPCEvaluatorVersion.ID
			version = do.CustomRPCEvaluatorVersion.Version
			description = do.CustomRPCEvaluatorVersion.Description
			baseInfo = do.CustomRPCEvaluatorVersion.BaseInfo
		}
	}

	if id == 0 && version == "" {
		return nil
	}

	dto := &openapiEvaluator.EvaluatorVersion{
		ID:               gptr.Of(id),
		Version:          gptr.Of(version),
		Description:      gptr.Of(description),
		EvaluatorContent: OpenAPIEvaluatorContentDO2DTO(do),
		BaseInfo:         common_convertor.OpenAPIBaseInfoDO2DTO(baseInfo),
	}
	return dto
}

func OpenAPIEvaluatorVersionDO2DTOs(dos []*entity.Evaluator) []*openapiEvaluator.EvaluatorVersion {
	if len(dos) == 0 {
		return nil
	}
	dtos := make([]*openapiEvaluator.EvaluatorVersion, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		dto := OpenAPIEvaluatorVersionDO2DTO(do)
		if dto != nil {
			dtos = append(dtos, dto)
		}
	}
	return dtos
}

func OpenAPIEvaluatorContentDO2DTO(do *entity.Evaluator) *openapiEvaluator.EvaluatorContent {
	if do == nil {
		return nil
	}
	dto := &openapiEvaluator.EvaluatorContent{}

	switch do.EvaluatorType {
	case entity.EvaluatorTypePrompt:
		if v := do.PromptEvaluatorVersion; v != nil {
			dto.IsReceiveChatHistory = v.ReceiveChatHistory
			dto.InputSchemas = common_convertor.OpenAPIArgsSchemaDO2DTOs(v.InputSchemas)
			dto.PromptEvaluator = &openapiEvaluator.PromptEvaluator{
				Messages:    common_convertor.OpenAPIMessageDO2DTOs(v.MessageList),
				ModelConfig: common_convertor.OpenAPIModelConfigDO2DTO(v.ModelConfig),
			}
		}
	case entity.EvaluatorTypeCode:
		if v := do.CodeEvaluatorVersion; v != nil {
			dto.CodeEvaluator = &openapiEvaluator.CodeEvaluator{
				LanguageType: OpenAPILanguageTypeDO2DTO(v.LanguageType),
				CodeContent:  gptr.Of(v.CodeContent),
			}
		}
	case entity.EvaluatorTypeCustomRPC:
		if v := do.CustomRPCEvaluatorVersion; v != nil {
			dto.InputSchemas = common_convertor.OpenAPIArgsSchemaDO2DTOs(v.InputSchemas)
			dto.OutputSchemas = common_convertor.OpenAPIArgsSchemaDO2DTOs(v.OutputSchemas)
			dto.CustomRPCEvaluator = &openapiEvaluator.CustomRPCEvaluator{
				ProviderEvaluatorCode: v.ProviderEvaluatorCode,
				AccessProtocol:        openapiAccessProtocolFromEntity(v.AccessProtocol),
				ServiceName:           v.ServiceName,
				Cluster:               v.Cluster,
				InvokeHTTPInfo:        OpenAPIEvaluatorHTTPInfoDO2DTO(v.InvokeHTTPInfo),
				Timeout:               v.Timeout,
				RateLimit:             common_convertor.OpenAPIRateLimitDO2DTO(v.RateLimit),
				Ext:                   v.Ext,
			}
		}
	}

	return dto
}

func OpenAPILanguageTypeDO2DTO(do entity.LanguageType) *openapiEvaluator.LanguageType {
	var openapiType openapiEvaluator.LanguageType
	switch do {
	case entity.LanguageTypePython:
		openapiType = openapiEvaluator.LanguageTypePython
	case entity.LanguageTypeJS:
		openapiType = openapiEvaluator.LanguageTypeJS
	default:
		return nil
	}
	return &openapiType
}

// openapiAccessProtocolFromEntity 将 entity 协议转为 openapi（仅 rpc/faas_http，old 版本映射为当前）
func openapiAccessProtocolFromEntity(protocol entity.EvaluatorAccessProtocol) *openapiEvaluator.EvaluatorAccessProtocol {
	switch protocol {
	case entity.EvaluatorAccessProtocolRPCOld:
		return gptr.Of(openapiEvaluator.EvaluatorAccessProtocolRPC)
	case entity.EvaluatorAccessProtocolFaasHTTPOld:
		return gptr.Of(openapiEvaluator.EvaluatorAccessProtocolFaasHTTP)
	case entity.EvaluatorAccessProtocolRPC, entity.EvaluatorAccessProtocolFaasHTTP:
		var t openapiEvaluator.EvaluatorAccessProtocol = protocol
		return &t
	default:
		if protocol == "" {
			return nil
		}
		var t openapiEvaluator.EvaluatorAccessProtocol = protocol
		return &t
	}
}

func OpenAPIEvaluatorHTTPInfoDO2DTO(do *entity.EvaluatorHTTPInfo) *openapiEvaluator.EvaluatorHTTPInfo {
	if do == nil {
		return nil
	}
	var method *openapiEvaluator.EvaluatorHTTPMethod
	if do.Method != nil {
		var m openapiEvaluator.EvaluatorHTTPMethod = *do.Method
		method = &m
	}
	return &openapiEvaluator.EvaluatorHTTPInfo{
		Method: method,
		Path:   do.Path,
	}
}

func OpenAPIEvaluatorHTTPInfoDTO2DO(dto *openapiEvaluator.EvaluatorHTTPInfo) *entity.EvaluatorHTTPInfo {
	if dto == nil {
		return nil
	}
	var method *entity.EvaluatorHTTPMethod
	if dto.Method != nil {
		m := entity.EvaluatorHTTPMethod(*dto.Method)
		method = &m
	}
	return &entity.EvaluatorHTTPInfo{
		Method: method,
		Path:   dto.Path,
	}
}

func OpenAPIEvaluatorRecordDO2DTO(do *entity.EvaluatorRecord) *openapiEvaluator.EvaluatorRecord {
	if do == nil {
		return nil
	}
	dto := &openapiEvaluator.EvaluatorRecord{
		ID:                  gptr.Of(do.ID),
		EvaluatorVersionID:  gptr.Of(do.EvaluatorVersionID),
		ItemID:              gptr.Of(do.ItemID),
		TurnID:              gptr.Of(do.TurnID),
		Status:              OpenAPIEvaluatorRunStatusDO2DTO(do.Status),
		EvaluatorOutputData: OpenAPIEvaluatorOutputDataDO2DTO(do.EvaluatorOutputData),
		Logid:               gptr.Of(do.LogID),
		TraceID:             gptr.Of(do.TraceID),
		BaseInfo:            common_convertor.OpenAPIBaseInfoDO2DTO(do.BaseInfo),
	}
	return dto
}

func OpenAPIEvaluatorRecordDO2DTOs(dos []*entity.EvaluatorRecord) []*openapiEvaluator.EvaluatorRecord {
	if len(dos) == 0 {
		return nil
	}
	dtos := make([]*openapiEvaluator.EvaluatorRecord, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		dtos = append(dtos, OpenAPIEvaluatorRecordDO2DTO(do))
	}
	return dtos
}

func OpenAPIEvaluatorRunStatusDO2DTO(do entity.EvaluatorRunStatus) *openapiEvaluator.EvaluatorRunStatus {
	var openapiStatus openapiEvaluator.EvaluatorRunStatus
	switch do {
	case entity.EvaluatorRunStatusSuccess:
		openapiStatus = openapiEvaluator.EvaluatorRunStatusSuccess
	case entity.EvaluatorRunStatusFail:
		openapiStatus = openapiEvaluator.EvaluatorRunStatusFailed
	case entity.EvaluatorRunStatusUnknown:
		openapiStatus = openapiEvaluator.EvaluatorRunStatusUnknown
	default:
		openapiStatus = openapiEvaluator.EvaluatorRunStatusProcessing
	}
	return &openapiStatus
}

func OpenAPIEvaluatorOutputDataDO2DTO(do *entity.EvaluatorOutputData) *openapiEvaluator.EvaluatorOutputData {
	if do == nil {
		return nil
	}
	dto := &openapiEvaluator.EvaluatorOutputData{
		EvaluatorResult_:  OpenAPIEvaluatorResultDO2DTO(do.EvaluatorResult),
		EvaluatorUsage:    OpenAPIEvaluatorUsageDO2DTO(do.EvaluatorUsage),
		EvaluatorRunError: OpenAPIEvaluatorRunErrorDO2DTO(do.EvaluatorRunError),
		TimeConsumingMs:   gptr.Of(do.TimeConsumingMS),
		Stdout:            gptr.Of(do.Stdout),
	}
	return dto
}

func OpenAPIEvaluatorResultDO2DTO(do *entity.EvaluatorResult) *openapiEvaluator.EvaluatorResult_ {
	if do == nil {
		return nil
	}
	dto := &openapiEvaluator.EvaluatorResult_{
		Score:      do.Score,
		Reasoning:  gptr.Of(do.Reasoning),
		Correction: OpenAPICorrectionDO2DTO(do.Correction),
	}
	return dto
}

func OpenAPICorrectionDO2DTO(do *entity.Correction) *openapiEvaluator.Correction {
	if do == nil {
		return nil
	}
	dto := &openapiEvaluator.Correction{
		Score:     do.Score,
		Explain:   gptr.Of(do.Explain),
		UpdatedBy: gptr.Of(do.UpdatedBy),
	}
	return dto
}

func OpenAPIEvaluatorUsageDO2DTO(do *entity.EvaluatorUsage) *openapiEvaluator.EvaluatorUsage {
	if do == nil {
		return nil
	}
	dto := &openapiEvaluator.EvaluatorUsage{
		InputTokens:  gptr.Of(do.InputTokens),
		OutputTokens: gptr.Of(do.OutputTokens),
	}
	return dto
}

func OpenAPIEvaluatorRunErrorDO2DTO(do *entity.EvaluatorRunError) *openapiEvaluator.EvaluatorRunError {
	if do == nil {
		return nil
	}
	dto := &openapiEvaluator.EvaluatorRunError{
		Code:    gptr.Of(do.Code),
		Message: gptr.Of(do.Message),
	}
	return dto
}

func OpenAPIEvaluatorInputDataDTO2DO(dto *openapiEvaluator.EvaluatorInputData) *entity.EvaluatorInputData {
	if dto == nil {
		return nil
	}
	return &entity.EvaluatorInputData{
		HistoryMessages:            common_convertor.OpenAPIMessageDTO2DOs(dto.HistoryMessages),
		InputFields:                common_convertor.OpenAPIContentDTO2DOs(dto.InputFields),
		EvaluateDatasetFields:      common_convertor.OpenAPIContentDTO2DOs(dto.EvaluateDatasetFields),
		EvaluateTargetOutputFields: common_convertor.OpenAPIContentDTO2DOs(dto.EvaluateTargetOutputFields),
	}
}

func OpenAPIEvaluatorRunConfigDTO2DO(dto *openapiEvaluator.EvaluatorRunConfig) *entity.EvaluatorRunConfig {
	if dto == nil {
		return nil
	}
	return &entity.EvaluatorRunConfig{
		Env:                   dto.Env,
		EvaluatorRuntimeParam: common_convertor.OpenAPIRuntimeParamDTO2DO(dto.EvaluatorRuntimeParam),
	}
}

// OpenAPIEvaluatorRunConfigDO2DTO entity.EvaluatorRunConfig -> openapi EvaluatorRunConfig（用于 ExptTemplate.EvaluatorIDVersionItems 等）
func OpenAPIEvaluatorRunConfigDO2DTO(do *entity.EvaluatorRunConfig) *openapiEvaluator.EvaluatorRunConfig {
	if do == nil {
		return nil
	}
	return &openapiEvaluator.EvaluatorRunConfig{
		Env:                   do.Env,
		EvaluatorRuntimeParam: common_convertor.OpenAPIRuntimeParamDO2DTO(do.EvaluatorRuntimeParam),
	}
}

func OpenAPICorrectionDTO2DO(dto *openapiEvaluator.Correction) *entity.Correction {
	if dto == nil {
		return nil
	}
	return &entity.Correction{
		Score:   dto.Score,
		Explain: dto.GetExplain(),
	}
}

func OpenAPIEvaluatorFiltersDTO2DO(dto *openapiEvaluator.EvaluatorFilters) *entity.EvaluatorFilters {
	if dto == nil {
		return nil
	}
	res := &entity.EvaluatorFilters{
		LogicOp: gptr.Of(OpenAPIEvaluatorFilterLogicOpDTO2DO(dto.LogicOp)),
	}
	for _, cond := range dto.FilterConditions {
		if cond == nil {
			continue
		}
		res.FilterConditions = append(res.FilterConditions, &entity.EvaluatorFilterCondition{
			TagKey:   entity.EvaluatorTagKey(cond.GetTagKey()),
			Operator: OpenAPIEvaluatorFilterOperatorTypeDTO2DO(cond.GetOperator()),
			Value:    cond.GetValue(),
		})
	}
	for _, sub := range dto.SubFilters {
		if sub == nil {
			continue
		}
		res.SubFilters = append(res.SubFilters, OpenAPIEvaluatorFiltersDTO2DO(sub))
	}
	return res
}

func OpenAPIEvaluatorFilterLogicOpDTO2DO(dto *openapiEvaluator.EvaluatorFilterLogicOp) entity.FilterLogicOp {
	if dto == nil {
		return entity.FilterLogicOp_Unknown
	}
	switch *dto {
	case openapiEvaluator.EvaluatorFilterLogicOpAnd:
		return entity.FilterLogicOp_And
	case openapiEvaluator.EvaluatorFilterLogicOpOr:
		return entity.FilterLogicOp_Or
	default:
		return entity.FilterLogicOp_Unknown
	}
}

func OpenAPIEvaluatorFilterOperatorTypeDTO2DO(dto string) entity.EvaluatorFilterOperatorType {
	switch strings.ToUpper(strings.TrimSpace(dto)) {
	case "EQUAL":
		return entity.EvaluatorFilterOperatorType_Equal
	case "NOT_EQUAL":
		return entity.EvaluatorFilterOperatorType_NotEqual
	case "IN":
		return entity.EvaluatorFilterOperatorType_In
	case "NOT_IN":
		return entity.EvaluatorFilterOperatorType_NotIn
	case "LIKE":
		return entity.EvaluatorFilterOperatorType_Like
	case "IS_NULL":
		return entity.EvaluatorFilterOperatorType_IsNull
	case "IS_NOT_NULL":
		return entity.EvaluatorFilterOperatorType_IsNotNull
	default:
		return entity.EvaluatorFilterOperatorType_Unknown
	}
}

func OpenAPIEvaluatorFilterOptionDTO2DO(dto *openapiEvaluator.EvaluatorFilterOption) *entity.EvaluatorFilterOption {
	if dto == nil {
		return nil
	}
	res := &entity.EvaluatorFilterOption{
		Filters: OpenAPIEvaluatorFiltersDTO2DO(dto.Filters),
	}
	if dto.SearchKeyword != nil {
		res.SearchKeyword = gptr.Of(dto.GetSearchKeyword())
	}
	return res
}

func OpenAPIEvaluatorContentDTO2DO(dto *openapiEvaluator.EvaluatorContent, evalType entity.EvaluatorType) (*entity.Evaluator, error) {
	if dto == nil {
		return nil, nil
	}
	res := &entity.Evaluator{
		EvaluatorType: evalType,
	}

	switch evalType {
	case entity.EvaluatorTypePrompt:
		res.PromptEvaluatorVersion = &entity.PromptEvaluatorVersion{
			ReceiveChatHistory: dto.IsReceiveChatHistory,
			InputSchemas:       common_convertor.OpenAPIArgsSchemaDTO2DOs(dto.InputSchemas),
		}
		if dto.PromptEvaluator != nil {
			res.PromptEvaluatorVersion.MessageList = common_convertor.OpenAPIMessageDTO2DOs(dto.PromptEvaluator.Messages)
			res.PromptEvaluatorVersion.ModelConfig = common_convertor.OpenAPIModelConfigDTO2DO(dto.PromptEvaluator.ModelConfig)
		}
	case entity.EvaluatorTypeCode:
		res.CodeEvaluatorVersion = &entity.CodeEvaluatorVersion{}
		if dto.CodeEvaluator != nil {
			res.CodeEvaluatorVersion.LanguageType = OpenAPILanguageTypeDTO2DO(dto.CodeEvaluator.LanguageType)
			res.CodeEvaluatorVersion.CodeContent = dto.CodeEvaluator.GetCodeContent()
		}
	case entity.EvaluatorTypeCustomRPC:
		res.CustomRPCEvaluatorVersion = &entity.CustomRPCEvaluatorVersion{
			InputSchemas:  common_convertor.OpenAPIArgsSchemaDTO2DOs(dto.InputSchemas),
			OutputSchemas: common_convertor.OpenAPIArgsSchemaDTO2DOs(dto.OutputSchemas),
		}
		if dto.CustomRPCEvaluator != nil {
			c := dto.CustomRPCEvaluator
			if c.IsSetProviderEvaluatorCode() {
				res.CustomRPCEvaluatorVersion.ProviderEvaluatorCode = gptr.Of(c.GetProviderEvaluatorCode())
			}
			if c.IsSetAccessProtocol() {
				res.CustomRPCEvaluatorVersion.AccessProtocol = entity.EvaluatorAccessProtocol(c.GetAccessProtocol())
			}
			res.CustomRPCEvaluatorVersion.ServiceName = gptr.Of(c.GetServiceName())
			res.CustomRPCEvaluatorVersion.Cluster = gptr.Of(c.GetCluster())
			res.CustomRPCEvaluatorVersion.InvokeHTTPInfo = OpenAPIEvaluatorHTTPInfoDTO2DO(c.GetInvokeHTTPInfo())
			if c.IsSetTimeout() {
				res.CustomRPCEvaluatorVersion.Timeout = gptr.Of(c.GetTimeout())
			}
			if c.IsSetExt() && len(c.GetExt()) > 0 {
				res.CustomRPCEvaluatorVersion.Ext = c.GetExt()
			}
			if c.IsSetRateLimit() && c.RateLimit != nil {
				rateLimit, err := common_convertor.OpenAPIRateLimitDTO2DO(c.RateLimit)
				if err != nil {
					return nil, err
				}
				res.CustomRPCEvaluatorVersion.RateLimit = rateLimit
			}
		}
	}
	return res, nil
}

func OpenAPILanguageTypeDTO2DO(dto *openapiEvaluator.LanguageType) entity.LanguageType {
	if dto == nil {
		return entity.LanguageTypePython
	}
	switch *dto {
	case openapiEvaluator.LanguageTypePython:
		return entity.LanguageTypePython
	case openapiEvaluator.LanguageTypeJS:
		return entity.LanguageTypeJS
	default:
		return entity.LanguageTypePython
	}
}

func OpenAPIEvaluatorDTO2DO(dto *openapiEvaluator.Evaluator) (*entity.Evaluator, error) {
	if dto == nil {
		return nil, nil
	}
	evalType := OpenAPIEvaluatorTypeDTO2DO(dto.EvaluatorType)
	res := &entity.Evaluator{
		ID:            dto.GetID(),
		SpaceID:       dto.GetWorkspaceID(),
		Name:          dto.GetName(),
		Description:   dto.GetDescription(),
		EvaluatorType: evalType,
	}
	if dto.CurrentVersion != nil {
		verDO, err := OpenAPIEvaluatorContentDTO2DO(dto.CurrentVersion.EvaluatorContent, evalType)
		if err != nil {
			return nil, err
		}
		res.SetEvaluatorVersion(verDO)
		res.SetVersion(dto.CurrentVersion.GetVersion())
		res.SetEvaluatorVersionDescription(dto.CurrentVersion.GetDescription())
	}
	return res, nil
}

func OpenAPIEvaluatorTypeDTO2DO(dto *openapiEvaluator.EvaluatorType) entity.EvaluatorType {
	if dto == nil {
		return entity.EvaluatorTypePrompt
	}
	switch *dto {
	case openapiEvaluator.EvaluatorTypePrompt:
		return entity.EvaluatorTypePrompt
	case openapiEvaluator.EvaluatorTypeCode:
		return entity.EvaluatorTypeCode
	case openapiEvaluator.EvaluatorTypeCustomRPC:
		return entity.EvaluatorTypeCustomRPC
	default:
		return entity.EvaluatorTypePrompt
	}
}
