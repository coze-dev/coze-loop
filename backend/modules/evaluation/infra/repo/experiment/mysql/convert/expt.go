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
	"github.com/coze-dev/coze-loop/backend/pkg/lang/conv"
)

func NewExptConverter() ExptConverter {
	return ExptConverter{}
}

type ExptConverter struct{}

func (ExptConverter) DO2PO(experiment *entity.Experiment) (*model.Experiment, error) {
	expt := &model.Experiment{
		ID:               experiment.ID,
		SpaceID:          experiment.SpaceID,
		CreatedBy:        experiment.CreatedBy,
		Name:             experiment.Name,
		Description:      experiment.Description,
		EvalSetVersionID: experiment.EvalSetVersionID,
		EvalSetID:        experiment.EvalSetID,
		TargetVersionID:  experiment.TargetVersionID,
		TargetType:       int64(experiment.TargetType),
		TargetID:         experiment.TargetID,
		Status:           int32(experiment.Status),
		StatusMessage:    gptr.Of(conv.UnsafeStringToBytes(experiment.StatusMessage)),
		StartAt:          experiment.StartAt,
		EndAt:            experiment.EndAt,
		LatestRunID:      experiment.LatestRunID,
		CreditCost:       int32(experiment.CreditCost),
		SourceType:       int32(experiment.SourceType),
		SourceID:         experiment.SourceID,
		ExptType:         int32(experiment.ExptType),
	}

	if experiment.MaxAliveTime != 0 {
		expt.MaxAliveTime = gptr.Of(experiment.MaxAliveTime)
	}

	if experiment.EvalConf != nil {
		bytes, err := json.Marshal(experiment.EvalConf)
		if err != nil {
			return nil, errorx.Wrapf(err, "EvaluationConfiguration json marshal fail")
		}
		expt.EvalConf = &bytes
	}

	return expt, nil
}

func (ExptConverter) PO2DO(expt *model.Experiment, refs []*model.ExptEvaluatorRef) (*entity.Experiment, error) {
	evalConf := new(entity.EvaluationConfiguration)
	if err := lo.TernaryF(
		len(gptr.Indirect(expt.EvalConf)) == 0,
		func() error { return nil },
		func() error { return json.Unmarshal(gptr.Indirect(expt.EvalConf), evalConf) },
	); err != nil {
		return nil, errorx.Wrapf(err, "EvaluationConfiguration json unmarshal fail, expt_id: %v, raw: %v", expt.ID, conv.UnsafeBytesToString(gptr.Indirect(expt.EvalConf)))
	}

	evaluatorVersionRef := make([]*entity.ExptEvaluatorVersionRef, 0, len(refs))
	for _, ref := range refs {
		evaluatorVersionRef = append(evaluatorVersionRef, &entity.ExptEvaluatorVersionRef{
			EvaluatorVersionID: ref.EvaluatorVersionID,
			EvaluatorID:        ref.EvaluatorID,
		})
	}

	return &entity.Experiment{
		ID:                  expt.ID,
		SpaceID:             expt.SpaceID,
		CreatedBy:           expt.CreatedBy,
		Name:                expt.Name,
		Description:         expt.Description,
		EvalSetVersionID:    expt.EvalSetVersionID,
		EvalSetID:           expt.EvalSetID,
		TargetVersionID:     expt.TargetVersionID,
		TargetType:          entity.EvalTargetType(expt.TargetType),
		TargetID:            expt.TargetID,
		EvaluatorVersionRef: evaluatorVersionRef,
		EvalConf:            evalConf,
		Status:              entity.ExptStatus(expt.Status),
		StatusMessage:       conv.UnsafeBytesToString(gptr.Indirect(expt.StatusMessage)),
		LatestRunID:         expt.LatestRunID,
		CreditCost:          entity.CreditCost(expt.CreditCost),
		StartAt:             expt.StartAt,
		EndAt:               expt.EndAt,
		SourceType:          entity.SourceType(expt.SourceType),
		SourceID:            expt.SourceID,
		ExptType:            entity.ExptType(expt.ExptType),
		MaxAliveTime:        gptr.Indirect(expt.MaxAliveTime),
	}, nil
}
