// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"github.com/bytedance/gg/gptr"
	"github.com/samber/lo"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

func NewExptTemplateConverter() ExptTemplateConverter {
	return ExptTemplateConverter{}
}

type ExptTemplateConverter struct{}

// DO2PO 将实体转换为持久化对象
func (ExptTemplateConverter) DO2PO(template *entity.ExptTemplate) (*model.ExptTemplate, error) {
	// 从 Meta 获取基础信息
	var id, spaceID int64
	var name, description string
	var exptType entity.ExptType
	if template.Meta != nil {
		id = template.Meta.ID
		spaceID = template.Meta.WorkspaceID
		name = template.Meta.Name
		description = template.Meta.Desc
		exptType = template.Meta.ExptType
	}

	// 从 BaseInfo 获取 CreatedBy
	var createdBy string
	if template.BaseInfo != nil && template.BaseInfo.CreatedBy != nil && template.BaseInfo.CreatedBy.UserID != nil {
		createdBy = *template.BaseInfo.CreatedBy.UserID
	}

	// 从 TripleConfig 获取三元组信息
	var evalSetID, evalSetVersionID, targetID, targetVersionID int64
	var targetType entity.EvalTargetType
	if template.TripleConfig != nil {
		evalSetID = template.TripleConfig.EvalSetID
		evalSetVersionID = template.TripleConfig.EvalSetVersionID
		targetID = template.TripleConfig.TargetID
		targetVersionID = template.TripleConfig.TargetVersionID
		targetType = template.TripleConfig.TargetType
	}

	// 从 BaseInfo 获取 UpdatedBy
	var updatedBy string
	if template.BaseInfo != nil && template.BaseInfo.UpdatedBy != nil && template.BaseInfo.UpdatedBy.UserID != nil {
		updatedBy = *template.BaseInfo.UpdatedBy.UserID
	}

	po := &model.ExptTemplate{
		ID:               id,
		SpaceID:          spaceID,
		CreatedBy:        createdBy,
		UpdatedBy:        updatedBy,
		Name:             name,
		Description:      description,
		EvalSetID:        evalSetID,
		EvalSetVersionID: evalSetVersionID,
		TargetID:         targetID,
		TargetType:       int64(targetType),
		TargetVersionID:  targetVersionID,
		ExptType:         int32(exptType),
	}

	if template.TemplateConf != nil {
		bytes, err := json.Marshal(template.TemplateConf)
		if err != nil {
			return nil, errorx.Wrapf(err, "ExptTemplateConfiguration json marshal fail")
		}
		po.TemplateConf = &bytes
	}

	// 序列化 ExptInfo
	if template.ExptInfo != nil {
		bytes, err := json.Marshal(template.ExptInfo)
		if err != nil {
			return nil, errorx.Wrapf(err, "ExptInfo json marshal fail")
		}
		po.ExptInfo = &bytes
	}

	return po, nil
}

// PO2DO 将持久化对象转换为实体
func (ExptTemplateConverter) PO2DO(po *model.ExptTemplate, refs []*model.ExptTemplateEvaluatorRef) (*entity.ExptTemplate, error) {
	templateConf := new(entity.ExptTemplateConfiguration)
	if err := lo.TernaryF(
		len(gptr.Indirect(po.TemplateConf)) == 0,
		func() error { return nil },
		func() error { return json.Unmarshal(gptr.Indirect(po.TemplateConf), templateConf) },
	); err != nil {
		return nil, errorx.Wrapf(err, "ExptTemplateConfiguration json unmarshal fail, template_id: %v", po.ID)
	}

	evaluatorVersionRef := make([]*entity.ExptTemplateEvaluatorVersionRef, 0, len(refs))
	evaluatorVersionIds := make([]int64, 0, len(refs))
	evaluatorIDVersionItems := make([]*entity.EvaluatorIDVersionItem, 0, len(refs))
	for _, ref := range refs {
		evaluatorVersionRef = append(evaluatorVersionRef, &entity.ExptTemplateEvaluatorVersionRef{
			EvaluatorVersionID: ref.EvaluatorVersionID,
			EvaluatorID:        ref.EvaluatorID,
		})
		evaluatorVersionIds = append(evaluatorVersionIds, ref.EvaluatorVersionID)
		// 构建 EvaluatorIDVersionItem（Version 字段将在服务层从 Evaluators 关联数据中填充）
		evaluatorIDVersionItems = append(evaluatorIDVersionItems, &entity.EvaluatorIDVersionItem{
			EvaluatorID:        ref.EvaluatorID,
			EvaluatorVersionID: ref.EvaluatorVersionID,
			// Version 和 ScoreWeight 将在服务层填充
		})
	}

	// 构建 Meta
	meta := &entity.ExptTemplateMeta{
		ID:          po.ID,
		WorkspaceID: po.SpaceID,
		Name:        po.Name,
		Desc:        po.Description,
		ExptType:    entity.ExptType(po.ExptType),
	}

	// 构建 TripleConfig
	tripleConfig := &entity.ExptTemplateTuple{
		EvalSetID:              po.EvalSetID,
		EvalSetVersionID:       po.EvalSetVersionID,
		TargetID:               po.TargetID,
		TargetVersionID:        po.TargetVersionID,
		TargetType:             entity.EvalTargetType(po.TargetType),
		EvaluatorVersionIds:    evaluatorVersionIds,
		EvaluatorIDVersionItems: evaluatorIDVersionItems,
	}

	// 从 TemplateConf 构建 FieldMappingConfig，并根据 EvaluatorConf.ScoreWeight 设置是否启用分数权重
	var fieldMappingConfig *entity.ExptFieldMapping

	if templateConf != nil {
		// 构建 FieldMappingConfig
		fieldMappingConfig = &entity.ExptFieldMapping{
			ItemConcurNum: templateConf.ItemConcurNum,
		}

		// 从 ConnectorConf 转换字段映射
		if templateConf.ConnectorConf.TargetConf != nil && templateConf.ConnectorConf.TargetConf.IngressConf != nil {
			ingressConf := templateConf.ConnectorConf.TargetConf.IngressConf
			targetMapping := &entity.TargetFieldMapping{}
			if ingressConf.EvalSetAdapter != nil {
				for _, fc := range ingressConf.EvalSetAdapter.FieldConfs {
					targetMapping.FromEvalSet = append(targetMapping.FromEvalSet, &entity.ExptTemplateFieldMapping{
						FieldName:     fc.FieldName,
						FromFieldName: fc.FromField,
						ConstValue:    fc.Value,
					})
				}
			}
			fieldMappingConfig.TargetFieldMapping = targetMapping

			// 提取运行时参数
			if ingressConf.CustomConf != nil {
				for _, fc := range ingressConf.CustomConf.FieldConfs {
					// 运行时参数存储在 CustomConf 中，字段名为 runtime_param
					if fc.FieldName == "runtime_param" {
						fieldMappingConfig.TargetRuntimeParam = &entity.RuntimeParam{
							JSONValue: gptr.Of(fc.Value),
						}
						break
					}
				}
			}
		}

		if templateConf.ConnectorConf.EvaluatorsConf != nil {
			evaluatorMappings := make([]*entity.EvaluatorFieldMapping, 0, len(templateConf.ConnectorConf.EvaluatorsConf.EvaluatorConf))
			for _, ec := range templateConf.ConnectorConf.EvaluatorsConf.EvaluatorConf {
				if ec.IngressConf == nil {
					continue
				}
				em := &entity.EvaluatorFieldMapping{
					EvaluatorVersionID: ec.EvaluatorVersionID,
				}
				if ec.IngressConf.EvalSetAdapter != nil {
					for _, fc := range ec.IngressConf.EvalSetAdapter.FieldConfs {
						em.FromEvalSet = append(em.FromEvalSet, &entity.ExptTemplateFieldMapping{
							FieldName:     fc.FieldName,
							FromFieldName: fc.FromField,
							ConstValue:    fc.Value,
						})
					}
				}
				if ec.IngressConf.TargetAdapter != nil {
					for _, fc := range ec.IngressConf.TargetAdapter.FieldConfs {
						em.FromTarget = append(em.FromTarget, &entity.ExptTemplateFieldMapping{
							FieldName:     fc.FieldName,
							FromFieldName: fc.FromField,
							ConstValue:    fc.Value,
						})
					}
				}
				evaluatorMappings = append(evaluatorMappings, em)
			}
			fieldMappingConfig.EvaluatorFieldMapping = evaluatorMappings

			// 如果有任一评估器配置了分数权重，则标记模板支持分数权重
			if templateConf.ConnectorConf.EvaluatorsConf != nil {
				templateConf.ConnectorConf.EvaluatorsConf.EnableScoreWeight = false
				for _, ec := range templateConf.ConnectorConf.EvaluatorsConf.EvaluatorConf {
					if ec != nil && ec.ScoreWeight != nil && *ec.ScoreWeight > 0 {
						templateConf.ConnectorConf.EvaluatorsConf.EnableScoreWeight = true
						break
					}
				}
			}
		}
	}

	// 构建 BaseInfo
	baseInfo := &entity.BaseInfo{
		CreatedAt: gptr.Of(po.CreatedAt.UnixMilli()),
		UpdatedAt: gptr.Of(po.UpdatedAt.UnixMilli()),
		CreatedBy: &entity.UserInfo{UserID: gptr.Of(po.CreatedBy)},
	}
	if len(po.UpdatedBy) > 0 {
		baseInfo.UpdatedBy = &entity.UserInfo{UserID: gptr.Of(po.UpdatedBy)}
	}
	if po.DeletedAt.Valid {
		baseInfo.DeletedAt = gptr.Of(po.DeletedAt.Time.UnixMilli())
	}

	// 反序列化 ExptInfo
	var exptInfo *entity.ExptInfo
	if po.ExptInfo != nil && len(*po.ExptInfo) > 0 {
		exptInfo = new(entity.ExptInfo)
		if err := json.Unmarshal(*po.ExptInfo, exptInfo); err != nil {
			return nil, errorx.Wrapf(err, "ExptInfo json unmarshal fail, template_id: %v", po.ID)
		}
	}

	return &entity.ExptTemplate{
		Meta:                meta,
		TripleConfig:        tripleConfig,
		FieldMappingConfig:  fieldMappingConfig,
		EvaluatorVersionRef: evaluatorVersionRef,
		TemplateConf:        templateConf,
		BaseInfo:            baseInfo,
		ExptInfo:            exptInfo,
	}, nil
}

func NewExptTemplateEvaluatorRefConverter() ExptTemplateEvaluatorRefConverter {
	return ExptTemplateEvaluatorRefConverter{}
}

type ExptTemplateEvaluatorRefConverter struct{}

// DO2PO 将实体引用转换为持久化对象
func (ExptTemplateEvaluatorRefConverter) DO2PO(refs []*entity.ExptTemplateEvaluatorRef) []*model.ExptTemplateEvaluatorRef {
	pos := make([]*model.ExptTemplateEvaluatorRef, 0, len(refs))
	for _, ref := range refs {
		pos = append(pos, &model.ExptTemplateEvaluatorRef{
			ID:                 ref.ID,
			SpaceID:            ref.SpaceID,
			ExptTemplateID:     ref.ExptTemplateID,
			EvaluatorID:        ref.EvaluatorID,
			EvaluatorVersionID: ref.EvaluatorVersionID,
		})
	}
	return pos
}
