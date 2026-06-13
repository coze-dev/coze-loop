// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func NewExptEvaluatorRefConverter() *ExptEvaluatorRefConverter {
	return &ExptEvaluatorRefConverter{}
}

type ExptEvaluatorRefConverter struct{}

func (ExptEvaluatorRefConverter) DO2PO(refs []*entity.ExptEvaluatorRef) []*model.ExptEvaluatorRef {
	models := make([]*model.ExptEvaluatorRef, 0, len(refs))
	for _, ref := range refs {
		m := &model.ExptEvaluatorRef{
			ID:                 ref.ID,
			SpaceID:            ref.SpaceID,
			ExptID:             ref.ExptID,
			EvalSetID:          ref.EvalSetID,          // ★
			EvaluatorID:        ref.EvaluatorID,
			EvaluatorVersionID: ref.EvaluatorVersionID,
			Alias_:             ref.Alias,               // ★ gorm_gen 将 alias 生成为 Alias_
		}
		if ref.Filter != nil {
			filter := ref.Filter
			m.Filter = &filter
		}
		if ref.BindingConfig != nil {
			bc := ref.BindingConfig
			m.BindingConfig = &bc
		}
		models = append(models, m)
	}
	return models
}

func (ExptEvaluatorRefConverter) PO2DO(refs []*model.ExptEvaluatorRef) []*entity.ExptEvaluatorRef {
	entities := make([]*entity.ExptEvaluatorRef, 0, len(refs))
	for _, ref := range refs {
		e := &entity.ExptEvaluatorRef{
			ID:                 ref.ID,
			SpaceID:            ref.SpaceID,
			ExptID:             ref.ExptID,
			EvalSetID:          ref.EvalSetID,          // ★
			EvaluatorID:        ref.EvaluatorID,
			EvaluatorVersionID: ref.EvaluatorVersionID,
			Alias:              ref.Alias_,              // ★
		}
		if ref.Filter != nil {
			e.Filter = *ref.Filter
		}
		if ref.BindingConfig != nil {
			e.BindingConfig = *ref.BindingConfig
		}
		entities = append(entities, e)
	}
	return entities
}
