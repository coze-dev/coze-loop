// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_target"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

// ConvertCreateExptTemplateReq 转换创建实验模板请求为实体参数
func ConvertCreateExptTemplateReq(req *expt.CreateExperimentTemplateRequest) (*entity.CreateExptTemplateParam, error) {
	param := &entity.CreateExptTemplateParam{
		SpaceID:              req.GetWorkspaceID(),
		Name:                 req.GetName(),
		Description:          req.GetDesc(),
		EvalSetID:            req.GetEvalSetID(),
		EvalSetVersionID:     req.GetEvalSetVersionID(),
		TargetID:             req.GetTargetID(),
		TargetVersionID:      req.GetTargetVersionID(),
		EvaluatorVersionIDs:  req.GetEvaluatorVersionIds(),
		ExptType:             entity.ExptType(gptr.Indirect(req.ExptType)),
		CreateEvalTargetParam: CreateEvalTargetParamDTO2DOForTemplate(req.CreateEvalTargetParam),
	}

	// 转换字段映射和运行时参数
	targetFieldMapping := toTargetFieldMappingDOForTemplate(req.TargetFieldMapping, req.TargetRuntimeParam)
	evaluatorFieldMapping := toEvaluatorFieldMappingDoForTemplate(req.EvaluatorFieldMapping)

	// 转换模板配置
	templateConf := &entity.ExptTemplateConfiguration{
		ItemConcurNum:       ptr.ConvIntPtr[int32, int](req.DefaultItemConcurNum),
		EvaluatorsConcurNum: ptr.ConvIntPtr[int32, int](req.DefaultEvaluatorsConcurNum),
	}

	// 构建 ConnectorConf
	if targetFieldMapping != nil || len(evaluatorFieldMapping) > 0 {
		templateConf.ConnectorConf = entity.Connector{
			TargetConf: &entity.TargetConf{
				TargetVersionID: req.GetTargetVersionID(),
				IngressConf:     targetFieldMapping,
			},
		}

		if len(evaluatorFieldMapping) > 0 {
			templateConf.ConnectorConf.EvaluatorsConf = &entity.EvaluatorsConf{
				EvaluatorConf:        evaluatorFieldMapping,
				EnableWeightedScore:   gptr.Indirect(req.EnableWeightedScore),
				EvaluatorScoreWeights: req.GetEvaluatorScoreWeights(),
			}
		}
	}

	param.TemplateConf = templateConf

	return param, nil
}

// toTargetFieldMappingDOForTemplate 转换目标字段映射（用于模板）
func toTargetFieldMappingDOForTemplate(mapping *domain_expt.TargetFieldMapping, rtp *common.RuntimeParam) *entity.TargetIngressConf {
	tic := &entity.TargetIngressConf{EvalSetAdapter: &entity.FieldAdapter{}}

	if mapping != nil {
		fc := make([]*entity.FieldConf, 0, len(mapping.GetFromEvalSet()))
		for _, fm := range mapping.GetFromEvalSet() {
			fc = append(fc, &entity.FieldConf{
				FieldName: fm.GetFieldName(),
				FromField: fm.GetFromFieldName(),
				Value:     fm.GetConstValue(),
			})
		}
		tic.EvalSetAdapter.FieldConfs = fc
	}

	if rtp != nil && len(rtp.GetJSONValue()) > 0 {
		tic.CustomConf = &entity.FieldAdapter{
			FieldConfs: []*entity.FieldConf{{
				FieldName: consts.FieldAdapterBuiltinFieldNameRuntimeParam,
				Value:     rtp.GetJSONValue(),
			}},
		}
	}
	return tic
}

// toEvaluatorFieldMappingDoForTemplate 转换评估器字段映射为EvaluatorConf（用于模板）
func toEvaluatorFieldMappingDoForTemplate(mapping []*domain_expt.EvaluatorFieldMapping) []*entity.EvaluatorConf {
	if mapping == nil {
		return nil
	}
	result := make([]*entity.EvaluatorConf, 0, len(mapping))
	for _, fm := range mapping {
		esf := make([]*entity.FieldConf, 0, len(fm.GetFromEvalSet()))
		for _, fes := range fm.GetFromEvalSet() {
			esf = append(esf, &entity.FieldConf{
				FieldName: fes.GetFieldName(),
				FromField: fes.GetFromFieldName(),
				Value:     fes.GetConstValue(),
			})
		}
		tf := make([]*entity.FieldConf, 0, len(fm.GetFromTarget()))
		for _, ft := range fm.GetFromTarget() {
			tf = append(tf, &entity.FieldConf{
				FieldName: ft.GetFieldName(),
				FromField: ft.GetFromFieldName(),
				Value:     ft.GetConstValue(),
			})
		}
		result = append(result, &entity.EvaluatorConf{
			EvaluatorVersionID: fm.GetEvaluatorVersionID(),
			IngressConf: &entity.EvaluatorIngressConf{
				EvalSetAdapter: &entity.FieldAdapter{FieldConfs: esf},
				TargetAdapter:  &entity.FieldAdapter{FieldConfs: tf},
			},
		})
	}
	return result
}

// ToExptTemplateDTO 转换实验模板实体为DTO
func ToExptTemplateDTO(template *entity.ExptTemplate) *domain_expt.ExptTemplate {
	if template == nil {
		return nil
	}

	evaluatorVersionIDs := make([]int64, 0, len(template.EvaluatorVersionRef))
	for _, ref := range template.EvaluatorVersionRef {
		evaluatorVersionIDs = append(evaluatorVersionIDs, ref.EvaluatorVersionID)
	}

	dto := &domain_expt.ExptTemplate{
		ID:          gptr.Of(template.ID),
		WorkspaceID: gptr.Of(template.SpaceID),
		Name:        gptr.Of(template.Name),
		Desc:        gptr.Of(template.Description),
		CreatorBy:   gptr.Of(template.CreatedBy),
		EvalSetID:   gptr.Of(template.EvalSetID),
		EvalSetVersionID: gptr.Of(template.EvalSetVersionID),
		TargetID:    gptr.Of(template.TargetID),
		TargetVersionID: gptr.Of(template.TargetVersionID),
		EvaluatorVersionIds: evaluatorVersionIDs,
		ExptType:    gptr.Of(domain_expt.ExptType(template.ExptType)),
	}

	// 转换模板配置
	if template.TemplateConf != nil {
		tm, ems, trtp := convertTemplateConfToDTO(template.TemplateConf)
		dto.TargetFieldMapping = tm
		dto.EvaluatorFieldMapping = ems
		dto.TargetRuntimeParam = trtp
		// 加权配置已移动到 ConnectorConf.EvaluatorsConf
		if template.TemplateConf.ConnectorConf.EvaluatorsConf != nil {
			dto.EnableWeightedScore = gptr.Of(template.TemplateConf.ConnectorConf.EvaluatorsConf.EnableWeightedScore)
			dto.EvaluatorScoreWeights = template.TemplateConf.ConnectorConf.EvaluatorsConf.EvaluatorScoreWeights
		}
		dto.DefaultItemConcurNum = ptr.ConvIntPtr[int, int32](template.TemplateConf.ItemConcurNum)
		dto.DefaultEvaluatorsConcurNum = ptr.ConvIntPtr[int, int32](template.TemplateConf.EvaluatorsConcurNum)
	}

	return dto
}

// convertTemplateConfToDTO 转换模板配置为DTO
func convertTemplateConfToDTO(conf *entity.ExptTemplateConfiguration) (*domain_expt.TargetFieldMapping, []*domain_expt.EvaluatorFieldMapping, *common.RuntimeParam) {
	var targetMapping *domain_expt.TargetFieldMapping
	var evaluatorMappings []*domain_expt.EvaluatorFieldMapping
	var runtimeParam *common.RuntimeParam

	if conf.ConnectorConf.TargetConf != nil && conf.ConnectorConf.TargetConf.IngressConf != nil {
		ingressConf := conf.ConnectorConf.TargetConf.IngressConf
		targetMapping = &domain_expt.TargetFieldMapping{}

		if ingressConf.EvalSetAdapter != nil {
			for _, fc := range ingressConf.EvalSetAdapter.FieldConfs {
				targetMapping.FromEvalSet = append(targetMapping.FromEvalSet, &domain_expt.FieldMapping{
					FieldName:     gptr.Of(fc.FieldName),
					FromFieldName: gptr.Of(fc.FromField),
					ConstValue:    gptr.Of(fc.Value),
				})
			}
		}

		if ingressConf.CustomConf != nil {
			for _, fc := range ingressConf.CustomConf.FieldConfs {
				if fc.FieldName == consts.FieldAdapterBuiltinFieldNameRuntimeParam {
					runtimeParam = &common.RuntimeParam{
						JSONValue: gptr.Of(fc.Value),
					}
					break
				}
			}
		}
	}

	if conf.ConnectorConf.EvaluatorsConf != nil {
		for _, evaluatorConf := range conf.ConnectorConf.EvaluatorsConf.EvaluatorConf {
			if evaluatorConf.IngressConf == nil {
				continue
			}
			m := &domain_expt.EvaluatorFieldMapping{
				EvaluatorVersionID: evaluatorConf.EvaluatorVersionID,
			}
			if evaluatorConf.IngressConf.EvalSetAdapter != nil {
				for _, fc := range evaluatorConf.IngressConf.EvalSetAdapter.FieldConfs {
					m.FromEvalSet = append(m.FromEvalSet, &domain_expt.FieldMapping{
						FieldName:     gptr.Of(fc.FieldName),
						FromFieldName: gptr.Of(fc.FromField),
						ConstValue:    gptr.Of(fc.Value),
					})
				}
			}
			if evaluatorConf.IngressConf.TargetAdapter != nil {
				for _, fc := range evaluatorConf.IngressConf.TargetAdapter.FieldConfs {
					m.FromTarget = append(m.FromTarget, &domain_expt.FieldMapping{
						FieldName:     gptr.Of(fc.FieldName),
						FromFieldName: gptr.Of(fc.FromField),
						ConstValue:    gptr.Of(fc.Value),
					})
				}
			}
			evaluatorMappings = append(evaluatorMappings, m)
		}
	}

	return targetMapping, evaluatorMappings, runtimeParam
}

// CreateEvalTargetParamDTO2DOForTemplate 转换创建评测对象参数（用于模板）
func CreateEvalTargetParamDTO2DOForTemplate(param *eval_target.CreateEvalTargetParam) *entity.CreateEvalTargetParam {
	if param == nil {
		return nil
	}

	res := &entity.CreateEvalTargetParam{
		SourceTargetID:      param.SourceTargetID,
		SourceTargetVersion: param.SourceTargetVersion,
		BotPublishVersion:   param.BotPublishVersion,
		Region:              param.Region,
		Env:                 param.Env,
	}
	if param.EvalTargetType != nil {
		res.EvalTargetType = gptr.Of(entity.EvalTargetType(*param.EvalTargetType))
	}
	if param.BotInfoType != nil {
		res.BotInfoType = gptr.Of(entity.CozeBotInfoType(*param.BotInfoType))
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

// ToExptTemplateDTOs 批量转换实验模板实体为DTO
func ToExptTemplateDTOs(templates []*entity.ExptTemplate) []*domain_expt.ExptTemplate {
	if len(templates) == 0 {
		return nil
	}
	dtos := make([]*domain_expt.ExptTemplate, 0, len(templates))
	for _, template := range templates {
		dtos = append(dtos, ToExptTemplateDTO(template))
	}
	return dtos
}

// ConvertUpdateExptTemplateReq 转换更新实验模板请求为实体参数
func ConvertUpdateExptTemplateReq(req *expt.UpdateExperimentTemplateRequest) (*entity.UpdateExptTemplateParam, error) {
	param := &entity.UpdateExptTemplateParam{
		TemplateID:            req.GetTemplateID(),
		SpaceID:               req.GetWorkspaceID(),
		Name:                  req.GetName(),
		Description:           req.GetDesc(),
		EvalSetVersionID:      req.GetEvalSetVersionID(),
		TargetVersionID:       req.GetTargetVersionID(),
		EvaluatorVersionIDs:   req.GetEvaluatorVersionIds(),
		ExptType:              entity.ExptType(gptr.Indirect(req.ExptType)),
		CreateEvalTargetParam: CreateEvalTargetParamDTO2DOForTemplate(req.CreateEvalTargetParam),
	}

	// 转换字段映射和运行时参数
	var targetFieldMapping *entity.TargetIngressConf
	var evaluatorFieldMapping []*entity.EvaluatorConf
	if req.TargetFieldMapping != nil || req.TargetRuntimeParam != nil {
		targetFieldMapping = toTargetFieldMappingDOForTemplate(req.TargetFieldMapping, req.TargetRuntimeParam)
	}
	if req.EvaluatorFieldMapping != nil {
		evaluatorFieldMapping = toEvaluatorFieldMappingDoForTemplate(req.EvaluatorFieldMapping)
	}

	// 构建模板配置
	if targetFieldMapping != nil || len(evaluatorFieldMapping) > 0 || req.EnableWeightedScore != nil || req.DefaultItemConcurNum != nil || req.DefaultEvaluatorsConcurNum != nil {
		templateConf := &entity.ExptTemplateConfiguration{
			ItemConcurNum:       ptr.ConvIntPtr[int32, int](req.DefaultItemConcurNum),
			EvaluatorsConcurNum: ptr.ConvIntPtr[int32, int](req.DefaultEvaluatorsConcurNum),
		}

		// 构建 ConnectorConf
		if targetFieldMapping != nil || len(evaluatorFieldMapping) > 0 {
			templateConf.ConnectorConf = entity.Connector{
				TargetConf: &entity.TargetConf{
					TargetVersionID: req.GetTargetVersionID(),
					IngressConf:     targetFieldMapping,
				},
			}

			if len(evaluatorFieldMapping) > 0 {
				templateConf.ConnectorConf.EvaluatorsConf = &entity.EvaluatorsConf{
					EvaluatorConf:        evaluatorFieldMapping,
					EnableWeightedScore:   gptr.Indirect(req.EnableWeightedScore),
					EvaluatorScoreWeights: req.GetEvaluatorScoreWeights(),
				}
			}
		}

		param.TemplateConf = templateConf
	}

	return param, nil
}
