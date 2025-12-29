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
	po := &model.ExptTemplate{
		ID:               template.ID,
		SpaceID:          template.SpaceID,
		CreatedBy:        template.CreatedBy,
		Name:             template.Name,
		Description:      template.Description,
		EvalSetID:        template.EvalSetID,
		EvalSetVersionID: template.EvalSetVersionID,
		TargetID:         template.TargetID,
		TargetType:       int64(template.TargetType),
		TargetVersionID:  template.TargetVersionID,
		ExptType:         int32(template.ExptType),
	}

	if template.TemplateConf != nil {
		bytes, err := json.Marshal(template.TemplateConf)
		if err != nil {
			return nil, errorx.Wrapf(err, "ExptTemplateConfiguration json marshal fail")
		}
		po.TemplateConf = &bytes
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
	for _, ref := range refs {
		evaluatorVersionRef = append(evaluatorVersionRef, &entity.ExptTemplateEvaluatorVersionRef{
			EvaluatorVersionID: ref.EvaluatorVersionID,
			EvaluatorID:        ref.EvaluatorID,
		})
	}

	return &entity.ExptTemplate{
		ID:                po.ID,
		SpaceID:           po.SpaceID,
		CreatedBy:         po.CreatedBy,
		Name:              po.Name,
		Description:       po.Description,
		EvalSetID:         po.EvalSetID,
		EvalSetVersionID:  po.EvalSetVersionID,
		TargetID:          po.TargetID,
		TargetType:        entity.EvalTargetType(po.TargetType),
		TargetVersionID:   po.TargetVersionID,
		EvaluatorVersionRef: evaluatorVersionRef,
		TemplateConf:      templateConf,
		ExptType:          entity.ExptType(po.ExptType),
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
			TemplateID:         ref.ExptTemplateID,
			EvaluatorID:        ref.EvaluatorID,
			EvaluatorVersionID: ref.EvaluatorVersionID,
		})
	}
	return pos
}
