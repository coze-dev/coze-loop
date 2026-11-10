// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

// ExptFilterProjectionBuilder rebuilds complete items without changing source data.
type ExptFilterProjectionBuilder interface {
	BuildExptFilterProjection(context.Context, int64, int64, []int64) ([]*entity.ExptTurnResultFilterEntity, error)
}

// NewExptFilterProjectionBuilder does not initialize execution services or publishers.
func NewExptFilterProjectionBuilder(experiments repo.IExperimentRepo, items repo.IExptItemResultRepo, turns repo.IExptTurnResultRepo, annotations repo.IExptAnnotateRepo, index repo.IExptTurnResultFilterRepo, refs repo.IExptItemRefRepo, targets repo.IEvalTargetRepo, evaluators repo.IEvaluatorRecordRepo, configer component.IConfiger, versions ...EvaluationSetVersionService) ExptFilterProjectionBuilder {
	var versionService EvaluationSetVersionService
	if len(versions) > 0 {
		versionService = versions[0]
	}
	return &ExptResultServiceImpl{ExperimentRepo: experiments, ExptItemResultRepo: items, ExptTurnResultRepo: turns, ExptAnnotateRepo: annotations, exptTurnResultFilterRepo: index, exptItemRefRepo: refs, evalTargetRepo: targets, filterEvaluatorRepo: evaluators, evaluationSetVersionService: versionService, scoreCalculator: NewEvaluatorScoreCalculator(configer, nil)}
}
