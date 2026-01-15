// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"fmt"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	domain_evaluator "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	evaluatorpkg "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_target"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/evaluation_set"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/evaluator"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/target"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

// ConvertCreateExptTemplateReq 转换创建实验模板请求为实体参数
func ConvertCreateExptTemplateReq(req *expt.CreateExperimentTemplateRequest) (*entity.CreateExptTemplateParam, error) {
	param := &entity.CreateExptTemplateParam{
		SpaceID:               req.GetWorkspaceID(),
		CreateEvalTargetParam: CreateEvalTargetParamDTO2DOForTemplate(req.CreateEvalTargetParam),
	}

	fillCreateTemplateMeta(param, req)
	fillCreateTemplateTripleConfig(param, req)

	targetFieldMapping, evaluatorFieldMapping, itemConcurNum :=
		buildTemplateFieldMappingsForCreate(req, param)

	evaluatorScoreWeights := buildEvaluatorScoreWeights(param.EvaluatorIDVersionItems)
	evaluatorConfs := buildEvaluatorConfsFromItems(param.EvaluatorIDVersionItems, evaluatorFieldMapping)
	applyScoreWeightsToEvaluatorConfs(evaluatorScoreWeights, evaluatorConfs)

	param.TemplateConf = buildTemplateConfForCreate(param, req, targetFieldMapping, evaluatorConfs, itemConcurNum)

	return param, nil
}

// 拆分的子函数：填充创建模板场景下的 Meta
func fillCreateTemplateMeta(param *entity.CreateExptTemplateParam, req *expt.CreateExperimentTemplateRequest) {
	if req.GetMeta() == nil {
		return
	}
		meta := req.GetMeta()
		param.Name = meta.GetName()
		param.Description = meta.GetDesc()
		param.ExptType = entity.ExptType(gptr.Indirect(meta.ExptType))
	}

// 拆分的子函数：从 triple_config 中提取三元组配置
func fillCreateTemplateTripleConfig(param *entity.CreateExptTemplateParam, req *expt.CreateExperimentTemplateRequest) {
	if req.GetTripleConfig() == nil {
		return
	}

		tripleConfig := req.GetTripleConfig()
		param.EvalSetID = tripleConfig.GetEvalSetID()
		param.EvalSetVersionID = tripleConfig.GetEvalSetVersionID()
		param.TargetID = tripleConfig.GetTargetID()
		param.TargetVersionID = tripleConfig.GetTargetVersionID()

	// 从 EvaluatorIDVersionItems 构建 entity 层的 EvaluatorIDVersionItems
		evaluatorIDVersionItems := make([]*entity.EvaluatorIDVersionItem, 0)
		if items := tripleConfig.GetEvaluatorIDVersionItems(); len(items) > 0 {
			for _, item := range items {
				if item == nil {
					continue
				}
				entityItem := &entity.EvaluatorIDVersionItem{
					EvaluatorID:        item.GetEvaluatorID(),
					Version:            item.GetVersion(),
					EvaluatorVersionID: item.GetEvaluatorVersionID(),
				}
				if item.IsSetScoreWeight() {
					entityItem.ScoreWeight = item.GetScoreWeight()
				}
				evaluatorIDVersionItems = append(evaluatorIDVersionItems, entityItem)
			}
		}
		param.EvaluatorIDVersionItems = evaluatorIDVersionItems
	}

// 拆分的子函数：构造创建模板场景下的字段映射配置
func buildTemplateFieldMappingsForCreate(
	req *expt.CreateExperimentTemplateRequest,
	param *entity.CreateExptTemplateParam,
) (*entity.TargetIngressConf, []*entity.EvaluatorConf, *int32) {
	var targetFieldMapping *entity.TargetIngressConf
	var evaluatorFieldMapping []*entity.EvaluatorConf
	var itemConcurNum *int32

	if req.FieldMappingConfig == nil {
		return targetFieldMapping, evaluatorFieldMapping, itemConcurNum
	}

		fieldMappingConfig := req.FieldMappingConfig
	targetFieldMapping = toTargetFieldMappingDOForTemplate(fieldMappingConfig.TargetFieldMapping, fieldMappingConfig.TargetRuntimeParam)
		evaluatorFieldMapping = toEvaluatorFieldMappingDoForTemplate(fieldMappingConfig.EvaluatorFieldMapping, param)
		itemConcurNum = fieldMappingConfig.ItemConcurNum

	return targetFieldMapping, evaluatorFieldMapping, itemConcurNum
}

// 通用子函数：从 EvaluatorIDVersionItems 中提取评分权重
// 使用 EvaluatorID + Version 作为 key（因为 EvaluatorVersionID 是 service 层填充的）
func buildEvaluatorScoreWeights(items []*entity.EvaluatorIDVersionItem) map[string]float64 {
	if len(items) == 0 {
		return nil
	}
	scoreWeights := make(map[string]float64)
	for _, item := range items {
		if item == nil || item.EvaluatorID <= 0 || item.Version == "" || item.ScoreWeight <= 0 {
			continue
		}
		key := fmt.Sprintf("%d#%s", item.EvaluatorID, item.Version)
		scoreWeights[key] = item.ScoreWeight
	}
	if len(scoreWeights) == 0 {
		return nil
	}
	return scoreWeights
}

// 通用子函数：基于 EvaluatorIDVersionItems + 字段映射构建 EvaluatorConf 列表
func buildEvaluatorConfsFromItems(
	items []*entity.EvaluatorIDVersionItem,
	evaluatorFieldMapping []*entity.EvaluatorConf,
) []*entity.EvaluatorConf {
	// 优先使用 EvaluatorIDVersionItems 构造 EvaluatorConf，保证有完整的 evaluator_id / version / evaluator_version_id 信息。
	// 但仅当其中存在有效的 evaluator_version_id 时才启用该路径；否则退化为仅基于字段映射的构造逻辑。
	if len(items) > 0 {
		hasValidVersionID := false
		for _, item := range items {
			if item != nil && item.EvaluatorVersionID > 0 {
				hasValidVersionID = true
				break
			}
		}

		if hasValidVersionID {
			ingressByVersionID := make(map[int64]*entity.EvaluatorIngressConf)
			for _, ec := range evaluatorFieldMapping {
				if ec == nil || ec.EvaluatorVersionID <= 0 {
					continue
				}
				ingressByVersionID[ec.EvaluatorVersionID] = ec.IngressConf
			}

			evaluatorConfs := make([]*entity.EvaluatorConf, 0, len(items))
			for _, item := range items {
				if item == nil || item.EvaluatorVersionID <= 0 {
					continue
				}
				conf := &entity.EvaluatorConf{
					EvaluatorVersionID: item.EvaluatorVersionID,
					EvaluatorID:        item.EvaluatorID,
					Version:            item.Version,
					IngressConf:        ingressByVersionID[item.EvaluatorVersionID],
				}
				evaluatorConfs = append(evaluatorConfs, conf)
			}
			if len(evaluatorConfs) > 0 {
				return evaluatorConfs
			}
			// 如果 items 全部被过滤掉（虽然 hasValidVersionID 为 true），则后续仍可退化为基于字段映射的逻辑
		}
	}

	// 兼容老用法：没有有效的 EvaluatorIDVersionItems（或根本没有）时，直接透传字段映射里的 EvaluatorConf
	if len(evaluatorFieldMapping) == 0 {
		return nil
	}
	evaluatorConfs := make([]*entity.EvaluatorConf, 0, len(evaluatorFieldMapping))
	for _, ec := range evaluatorFieldMapping {
		if ec == nil {
			continue
		}
		evaluatorConfs = append(evaluatorConfs, ec)
	}
	if len(evaluatorConfs) == 0 {
		return nil
	}
	return evaluatorConfs
}

// 通用子函数：将评分权重下沉到 EvaluatorConf.ScoreWeight
// 使用 EvaluatorID + Version 来匹配权重（因为 EvaluatorVersionID 是 service 层填充的）
func applyScoreWeightsToEvaluatorConfs(
	evaluatorScoreWeights map[string]float64,
	evaluatorConfs []*entity.EvaluatorConf,
) {
	if len(evaluatorScoreWeights) == 0 || len(evaluatorConfs) == 0 {
		return
	}
	for _, ec := range evaluatorConfs {
		if ec == nil || ec.EvaluatorID <= 0 || ec.Version == "" {
			continue
		}
		key := fmt.Sprintf("%d#%s", ec.EvaluatorID, ec.Version)
		if w, ok := evaluatorScoreWeights[key]; ok && w > 0 {
			ec.ScoreWeight = gptr.Of(w)
		}
	}
}

// 拆分的子函数：构造创建模板场景下的 TemplateConf
func buildTemplateConfForCreate(
	param *entity.CreateExptTemplateParam,
	req *expt.CreateExperimentTemplateRequest,
	targetFieldMapping *entity.TargetIngressConf,
	evaluatorConfs []*entity.EvaluatorConf,
	itemConcurNum *int32,
) *entity.ExptTemplateConfiguration {
	templateConf := &entity.ExptTemplateConfiguration{
		ItemConcurNum:       ptr.ConvIntPtr[int32, int](itemConcurNum),
		EvaluatorsConcurNum: ptr.ConvIntPtr[int32, int](req.DefaultEvaluatorsConcurNum),
	}

	if targetFieldMapping == nil && len(evaluatorConfs) == 0 {
		return templateConf
	}

		templateConf.ConnectorConf = entity.Connector{
			TargetConf: &entity.TargetConf{
				TargetVersionID: param.TargetVersionID,
				IngressConf:     targetFieldMapping,
			},
		}

	if len(evaluatorConfs) > 0 {
			templateConf.ConnectorConf.EvaluatorsConf = &entity.EvaluatorsConf{
			EvaluatorConf: evaluatorConfs,
		}
	}

	return templateConf
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
// 将 evaluator_id 和 version 信息保存到 EvaluatorConf 中，供 service 层回填使用
func toEvaluatorFieldMappingDoForTemplate(mapping []*domain_expt.EvaluatorFieldMapping, param interface{}) []*entity.EvaluatorConf {
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
		// 从 EvaluatorIDVersionItem 中提取信息，如果不存在则使用 EvaluatorVersionID
		var evaluatorID int64
		var version string
		var evaluatorVersionID int64 = fm.GetEvaluatorVersionID()

		if fm.IsSetEvaluatorIDVersionItem() {
			item := fm.GetEvaluatorIDVersionItem()
			if item != nil {
				if item.IsSetEvaluatorID() {
					evaluatorID = item.GetEvaluatorID()
				}
				if item.IsSetVersion() {
					version = item.GetVersion()
				}
				if item.IsSetEvaluatorVersionID() {
					evaluatorVersionID = item.GetEvaluatorVersionID()
				}
			}
		}

		// 将 evaluator_id 和 version 信息保存到 EvaluatorConf 中
		conf := &entity.EvaluatorConf{
			EvaluatorVersionID: evaluatorVersionID,
			EvaluatorID:        evaluatorID,
			Version:            version,
			IngressConf: &entity.EvaluatorIngressConf{
				EvalSetAdapter: &entity.FieldAdapter{FieldConfs: esf},
				TargetAdapter:  &entity.FieldAdapter{FieldConfs: tf},
			},
		}
		result = append(result, conf)
	}
	return result
}

// ToExptTemplateDTO 转换实验模板实体为DTO
func ToExptTemplateDTO(template *entity.ExptTemplate) *domain_expt.ExptTemplate {
	if template == nil {
		return nil
	}

	dto := &domain_expt.ExptTemplate{}

	fillTemplateMetaDTO(template, dto)
	dto.TripleConfig = buildTemplateTripleConfigDTO(template)
	dto.FieldMappingConfig = buildTemplateFieldMappingDTO(template)
	dto.ScoreWeightConfig = buildTemplateScoreWeightConfigDTO(template)

	// 填充关联数据（EvalSet、EvalTarget、Evaluators）到 TripleConfig
	if dto.TripleConfig != nil {
		dto.TripleConfig.EvalTarget = target.EvalTargetDO2DTO(template.Target)
		if template.Meta != nil && template.Meta.ExptType != entity.ExptType_Online {
			dto.TripleConfig.EvalSet = evaluation_set.EvaluationSetDO2DTO(template.EvalSet)
		}
		dto.TripleConfig.Evaluators = make([]*domain_evaluator.Evaluator, 0, len(template.Evaluators))
		for _, evaluatorDO := range template.Evaluators {
			if evaluatorDO != nil {
				dto.TripleConfig.Evaluators = append(dto.TripleConfig.Evaluators, evaluator.ConvertEvaluatorDO2DTO(evaluatorDO))
			}
		}
	}

	// 填充 BaseInfo
	if template.BaseInfo != nil {
		dto.BaseInfo = &common.BaseInfo{
			CreatedAt: template.BaseInfo.CreatedAt,
			UpdatedAt: template.BaseInfo.UpdatedAt,
			DeletedAt: template.BaseInfo.DeletedAt,
		}
		if template.BaseInfo.CreatedBy != nil {
			dto.BaseInfo.CreatedBy = &common.UserInfo{
				UserID: template.BaseInfo.CreatedBy.UserID,
			}
		}
		if template.BaseInfo.UpdatedBy != nil {
			dto.BaseInfo.UpdatedBy = &common.UserInfo{
				UserID: template.BaseInfo.UpdatedBy.UserID,
			}
		}
	}

	// 填充 ExptInfo
	if template.ExptInfo != nil {
		exptInfo := &domain_expt.ExptInfo{
			CreatedExptCount: gptr.Of(template.ExptInfo.CreatedExptCount),
			LatestExptID:     gptr.Of(template.ExptInfo.LatestExptID),
			LatestExptStatus: gptr.Of(domain_expt.ExptStatus(template.ExptInfo.LatestExptStatus)),
		}
		dto.SetExptInfo(exptInfo)
	}

	return dto
}

// ConvertUpdateExptTemplateMetaReq 转换更新实验模板 Meta 请求为实体参数
func ConvertUpdateExptTemplateMetaReq(req *expt.UpdateExperimentTemplateMetaRequest) (*entity.UpdateExptTemplateMetaParam, error) {
	param := &entity.UpdateExptTemplateMetaParam{
		TemplateID: req.GetTemplateID(),
		SpaceID:    req.GetWorkspaceID(),
	}

	// 从 meta 中提取基本信息
	if req.GetMeta() != nil {
		meta := req.GetMeta()
		if meta.IsSetName() {
			param.Name = meta.GetName()
		}
		if meta.IsSetDesc() {
			param.Description = meta.GetDesc()
		}
		if meta.IsSetExptType() {
			param.ExptType = entity.ExptType(meta.GetExptType())
		}
	}

	return param, nil
}

// 拆分子函数：Meta -> DTO
func fillTemplateMetaDTO(template *entity.ExptTemplate, dto *domain_expt.ExptTemplate) {
	if template.Meta == nil {
		return
	}
		dto.Meta = &domain_expt.ExptTemplateMeta{
			ID:          gptr.Of(template.Meta.ID),
			WorkspaceID: gptr.Of(template.Meta.WorkspaceID),
			Name:        gptr.Of(template.Meta.Name),
			Desc:        gptr.Of(template.Meta.Desc),
			ExptType:    gptr.Of(domain_expt.ExptType(template.Meta.ExptType)),
		}
	}

// 拆分子函数：TripleConfig -> DTO
func buildTemplateTripleConfigDTO(template *entity.ExptTemplate) *domain_expt.ExptTuple {
	if template.TripleConfig == nil {
		return nil
	}

	evaluatorIDVersionItems := buildEvaluatorIDVersionItemsDTO(template)

	return &domain_expt.ExptTuple{
		EvalSetID:               gptr.Of(template.TripleConfig.EvalSetID),
		EvalSetVersionID:        gptr.Of(template.TripleConfig.EvalSetVersionID),
		TargetID:                gptr.Of(template.TripleConfig.TargetID),
		TargetVersionID:         gptr.Of(template.TripleConfig.TargetVersionID),
		EvaluatorIDVersionItems: evaluatorIDVersionItems,
	}
}

// 拆分子函数：根据模板信息构建 EvaluatorIDVersionItems DTO 列表
func buildEvaluatorIDVersionItemsDTO(template *entity.ExptTemplate) []*domain_evaluator.EvaluatorIDVersionItem {
		evaluatorIDVersionItems := make([]*domain_evaluator.EvaluatorIDVersionItem, 0)

		// 优先使用 entity 层的 EvaluatorIDVersionItems
		if template.TripleConfig != nil && len(template.TripleConfig.EvaluatorIDVersionItems) > 0 {
			for _, entityItem := range template.TripleConfig.EvaluatorIDVersionItems {
				if entityItem == nil {
					continue
				}
				item := domain_evaluator.NewEvaluatorIDVersionItem()
				item.EvaluatorID = gptr.Of(entityItem.EvaluatorID)
				if entityItem.Version != "" {
					item.Version = gptr.Of(entityItem.Version)
				}
				item.EvaluatorVersionID = gptr.Of(entityItem.EvaluatorVersionID)
				if entityItem.ScoreWeight > 0 {
					item.ScoreWeight = gptr.Of(entityItem.ScoreWeight)
				}
				evaluatorIDVersionItems = append(evaluatorIDVersionItems, item)
			}
		return evaluatorIDVersionItems
	}

	if len(template.Evaluators) > 0 {
		appendEvaluatorIDVersionItemsFromEvaluators(template, &evaluatorIDVersionItems)
		return evaluatorIDVersionItems
	}

	if len(template.EvaluatorVersionRef) > 0 {
		appendEvaluatorIDVersionItemsFromVersionRef(template, &evaluatorIDVersionItems)
	}

	return evaluatorIDVersionItems
}

// 细分：从 Evaluators 构建 EvaluatorIDVersionItems
func appendEvaluatorIDVersionItemsFromEvaluators(
	template *entity.ExptTemplate,
	dst *[]*domain_evaluator.EvaluatorIDVersionItem,
) {
			for _, evaluator := range template.Evaluators {
				if evaluator == nil {
					continue
				}
				evaluatorID := evaluator.GetEvaluatorID()
				version := evaluator.GetVersion()
				evaluatorVersionID := evaluator.GetEvaluatorVersionID()
		if evaluatorID <= 0 || evaluatorVersionID <= 0 {
			continue
		}
					item := domain_evaluator.NewEvaluatorIDVersionItem()
					item.EvaluatorID = gptr.Of(evaluatorID)
					item.Version = gptr.Of(version)
					item.EvaluatorVersionID = gptr.Of(evaluatorVersionID)

					if template.TripleConfig != nil && len(template.TripleConfig.EvaluatorIDVersionItems) > 0 {
						for _, entityItem := range template.TripleConfig.EvaluatorIDVersionItems {
							if entityItem != nil && entityItem.EvaluatorVersionID == evaluatorVersionID && entityItem.ScoreWeight > 0 {
								item.ScoreWeight = gptr.Of(entityItem.ScoreWeight)
								break
							}
						}
					}
		*dst = append(*dst, item)
	}
}

// 细分：从 EvaluatorVersionRef 构建 EvaluatorIDVersionItems
func appendEvaluatorIDVersionItemsFromVersionRef(
	template *entity.ExptTemplate,
	dst *[]*domain_evaluator.EvaluatorIDVersionItem,
) {
			for _, ref := range template.EvaluatorVersionRef {
		if ref.EvaluatorID <= 0 || ref.EvaluatorVersionID <= 0 {
			continue
		}
					item := domain_evaluator.NewEvaluatorIDVersionItem()
					item.EvaluatorID = gptr.Of(ref.EvaluatorID)
					item.EvaluatorVersionID = gptr.Of(ref.EvaluatorVersionID)

					if template.TripleConfig != nil && len(template.TripleConfig.EvaluatorIDVersionItems) > 0 {
						for _, entityItem := range template.TripleConfig.EvaluatorIDVersionItems {
							if entityItem != nil && entityItem.EvaluatorVersionID == ref.EvaluatorVersionID && entityItem.ScoreWeight > 0 {
								item.ScoreWeight = gptr.Of(entityItem.ScoreWeight)
								break
							}
						}
					}
		*dst = append(*dst, item)
	}
}

// 拆分子函数：FieldMappingConfig -> DTO
func buildTemplateFieldMappingDTO(template *entity.ExptTemplate) *domain_expt.ExptFieldMapping {
	if template.FieldMappingConfig == nil {
		return nil
	}

		fieldMapping := &domain_expt.ExptFieldMapping{
			ItemConcurNum: ptr.ConvIntPtr[int, int32](template.FieldMappingConfig.ItemConcurNum),
		}

		if template.FieldMappingConfig.TargetFieldMapping != nil {
			targetMapping := &domain_expt.TargetFieldMapping{}
			for _, fm := range template.FieldMappingConfig.TargetFieldMapping.FromEvalSet {
				targetMapping.FromEvalSet = append(targetMapping.FromEvalSet, &domain_expt.FieldMapping{
					FieldName:     gptr.Of(fm.FieldName),
					FromFieldName: gptr.Of(fm.FromFieldName),
					ConstValue:    gptr.Of(fm.ConstValue),
				})
			}
			fieldMapping.TargetFieldMapping = targetMapping
		}

		if len(template.FieldMappingConfig.EvaluatorFieldMapping) > 0 {
			evaluatorMappings := make([]*domain_expt.EvaluatorFieldMapping, 0, len(template.FieldMappingConfig.EvaluatorFieldMapping))
			for _, em := range template.FieldMappingConfig.EvaluatorFieldMapping {
				m := &domain_expt.EvaluatorFieldMapping{
					EvaluatorVersionID: em.EvaluatorVersionID,
				}

				// 构建 EvaluatorIDVersionItem
				if em.EvaluatorID > 0 || em.Version != "" || em.EvaluatorVersionID > 0 {
					item := &evaluatorpkg.EvaluatorIDVersionItem{}
				if em.EvaluatorID > 0 {
						item.SetEvaluatorID(gptr.Of(em.EvaluatorID))
				}
				if em.Version != "" {
						item.SetVersion(gptr.Of(em.Version))
					}
					if em.EvaluatorVersionID > 0 {
						item.SetEvaluatorVersionID(gptr.Of(em.EvaluatorVersionID))
					}
					m.SetEvaluatorIDVersionItem(item)
				}
				for _, fm := range em.FromEvalSet {
					m.FromEvalSet = append(m.FromEvalSet, &domain_expt.FieldMapping{
						FieldName:     gptr.Of(fm.FieldName),
						FromFieldName: gptr.Of(fm.FromFieldName),
						ConstValue:    gptr.Of(fm.ConstValue),
					})
				}
				for _, fm := range em.FromTarget {
					m.FromTarget = append(m.FromTarget, &domain_expt.FieldMapping{
						FieldName:     gptr.Of(fm.FieldName),
						FromFieldName: gptr.Of(fm.FromFieldName),
						ConstValue:    gptr.Of(fm.ConstValue),
					})
				}
				evaluatorMappings = append(evaluatorMappings, m)
			}
			fieldMapping.EvaluatorFieldMapping = evaluatorMappings
		}

		if template.FieldMappingConfig.TargetRuntimeParam != nil {
			fieldMapping.TargetRuntimeParam = &common.RuntimeParam{
				JSONValue: gptr.Of(template.FieldMappingConfig.TargetRuntimeParam.JSONValue),
			}
		}

	return fieldMapping
}

// 拆分子函数：ScoreWeightConfig -> DTO
func buildTemplateScoreWeightConfigDTO(template *entity.ExptTemplate) *domain_expt.ExptScoreWeight {
	// 1) 优先使用 TemplateConf.ConnectorConf.EvaluatorsConf 中的 EvaluatorConf.ScoreWeight
	evaluatorScoreWeights := buildScoreWeightsFromTemplateConf(template)

	// 2) 若为空，再从 TripleConfig.EvaluatorIDVersionItems.ScoreWeight 补充（向后兼容）
	if (evaluatorScoreWeights == nil || len(evaluatorScoreWeights) == 0) &&
		template.TripleConfig != nil && len(template.TripleConfig.EvaluatorIDVersionItems) > 0 {
		evaluatorScoreWeights = make(map[int64]float64)
		for _, item := range template.TripleConfig.EvaluatorIDVersionItems {
			if item == nil || item.EvaluatorVersionID <= 0 || item.ScoreWeight <= 0 {
				continue
			}
			evaluatorScoreWeights[item.EvaluatorVersionID] = item.ScoreWeight
		}
	}

	// 检查是否启用加权分数：从 EvaluatorsConf.EnableScoreWeight 或是否有权重配置
	hasWeightedScore := len(evaluatorScoreWeights) > 0
	if template.TemplateConf != nil && template.TemplateConf.ConnectorConf.EvaluatorsConf != nil {
		hasWeightedScore = hasWeightedScore || template.TemplateConf.ConnectorConf.EvaluatorsConf.EnableScoreWeight
	}
	if !hasWeightedScore {
		return nil
	}

	return &domain_expt.ExptScoreWeight{
			EnableWeightedScore:   gptr.Of(hasWeightedScore),
			EvaluatorScoreWeights: evaluatorScoreWeights,
		}
	}

// 细分：从 TemplateConf 中抽取权重
func buildScoreWeightsFromTemplateConf(template *entity.ExptTemplate) map[int64]float64 {
	if template.TemplateConf == nil || template.TemplateConf.ConnectorConf.EvaluatorsConf == nil {
		return nil
	}

	var evaluatorScoreWeights map[int64]float64
	for _, ec := range template.TemplateConf.ConnectorConf.EvaluatorsConf.EvaluatorConf {
		if ec == nil || ec.ScoreWeight == nil || *ec.ScoreWeight <= 0 {
			continue
		}
		if evaluatorScoreWeights == nil {
			evaluatorScoreWeights = make(map[int64]float64)
		}
		evaluatorScoreWeights[ec.EvaluatorVersionID] = *ec.ScoreWeight
	}
	return evaluatorScoreWeights
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

			// 构建 EvaluatorIDVersionItem
			if evaluatorConf.EvaluatorID > 0 || evaluatorConf.Version != "" || evaluatorConf.EvaluatorVersionID > 0 {
				item := &evaluatorpkg.EvaluatorIDVersionItem{}
			if evaluatorConf.EvaluatorID > 0 {
					item.SetEvaluatorID(gptr.Of(evaluatorConf.EvaluatorID))
			}
			if evaluatorConf.Version != "" {
					item.SetVersion(gptr.Of(evaluatorConf.Version))
				}
				if evaluatorConf.EvaluatorVersionID > 0 {
					item.SetEvaluatorVersionID(gptr.Of(evaluatorConf.EvaluatorVersionID))
				}
				// 如果 EvaluatorConf 中有 ScoreWeight，也填充到 item 中
				if evaluatorConf.ScoreWeight != nil && *evaluatorConf.ScoreWeight > 0 {
					item.SetScoreWeight(gptr.Of(*evaluatorConf.ScoreWeight))
				}
				m.SetEvaluatorIDVersionItem(item)
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
		CreateEvalTargetParam: CreateEvalTargetParamDTO2DOForTemplate(req.CreateEvalTargetParam),
	}

	// 从 meta 中提取基本信息
	if req.GetMeta() != nil {
		meta := req.GetMeta()
		param.Name = meta.GetName()
		param.Description = meta.GetDesc()
		param.ExptType = entity.ExptType(gptr.Indirect(meta.ExptType))
	}

	// 从 triple_config 中提取三元组配置（注意：eval_set_id / target_id 不允许修改，仅允许调整版本与配置）
	if req.GetTripleConfig() != nil {
		tripleConfig := req.GetTripleConfig()
		param.EvalSetVersionID = tripleConfig.GetEvalSetVersionID()
		param.TargetVersionID = tripleConfig.GetTargetVersionID()
		// 从 EvaluatorIDVersionItems 构建 entity 层的 EvaluatorIDVersionItems
		evaluatorIDVersionItems := make([]*entity.EvaluatorIDVersionItem, 0)
		if items := tripleConfig.GetEvaluatorIDVersionItems(); len(items) > 0 {
			for _, item := range items {
				if item == nil {
					continue
				}
				// 构建 entity 层的 EvaluatorIDVersionItem
				entityItem := &entity.EvaluatorIDVersionItem{
					EvaluatorID:        item.GetEvaluatorID(),
					Version:            item.GetVersion(),
					EvaluatorVersionID: item.GetEvaluatorVersionID(),
				}
				if item.IsSetScoreWeight() {
					entityItem.ScoreWeight = item.GetScoreWeight()
				}
				evaluatorIDVersionItems = append(evaluatorIDVersionItems, entityItem)
			}
		}
		param.EvaluatorIDVersionItems = evaluatorIDVersionItems
	}

	// 从 field_mapping_config 中提取字段映射和运行时参数
	var targetFieldMapping *entity.TargetIngressConf
	var evaluatorFieldMapping []*entity.EvaluatorConf
	var itemConcurNum *int32
	if req.GetFieldMappingConfig() != nil {
		fieldMappingConfig := req.GetFieldMappingConfig()
		targetFieldMapping = toTargetFieldMappingDOForTemplate(fieldMappingConfig.TargetFieldMapping, fieldMappingConfig.TargetRuntimeParam)
		evaluatorFieldMapping = toEvaluatorFieldMappingDoForTemplate(fieldMappingConfig.EvaluatorFieldMapping, param)
		itemConcurNum = fieldMappingConfig.ItemConcurNum
	}

	// 从 triple_config.evaluator_id_version_items 中提取得分加权配置，并下沉到 EvaluatorConf.ScoreWeight
	evaluatorScoreWeights := buildEvaluatorScoreWeights(param.EvaluatorIDVersionItems)

	// 基于 EvaluatorIDVersionItems 构建完整的 EvaluatorConf 列表（包含字段映射与权重）
	evaluatorConfs := buildEvaluatorConfsFromItems(param.EvaluatorIDVersionItems, evaluatorFieldMapping)
	applyScoreWeightsToEvaluatorConfs(evaluatorScoreWeights, evaluatorConfs)

	// 构建模板配置
	hasFieldMapping := targetFieldMapping != nil || len(evaluatorFieldMapping) > 0
	hasScoreWeight := len(evaluatorScoreWeights) > 0
	hasConcurNum := itemConcurNum != nil || req.DefaultEvaluatorsConcurNum != nil

	if hasFieldMapping || hasScoreWeight || hasConcurNum {
		templateConf := &entity.ExptTemplateConfiguration{
			ItemConcurNum:       ptr.ConvIntPtr[int32, int](itemConcurNum),
			EvaluatorsConcurNum: ptr.ConvIntPtr[int32, int](req.DefaultEvaluatorsConcurNum),
		}

		// 构建 ConnectorConf
		if hasFieldMapping || len(evaluatorConfs) > 0 {
			templateConf.ConnectorConf = entity.Connector{
				TargetConf: &entity.TargetConf{
					TargetVersionID: param.TargetVersionID,
					IngressConf:     targetFieldMapping,
				},
			}

			if len(evaluatorConfs) > 0 {
				templateConf.ConnectorConf.EvaluatorsConf = &entity.EvaluatorsConf{
					EvaluatorConf: evaluatorConfs,
				}
			}
		}

		param.TemplateConf = templateConf
	}

	return param, nil
}
