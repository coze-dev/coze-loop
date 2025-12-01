// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func NewEvaluationAnalysisService() IEvaluationAnalysisService {
	return &evaluationAnalysisServiceImpl{}
}

type evaluationAnalysisServiceImpl struct {
}

func (e evaluationAnalysisServiceImpl) GetAnalysisRecord(ctx context.Context, id int64) (record *entity.AnalysisRecord, err error) {
	return nil, err
}

func (e evaluationAnalysisServiceImpl) BatchGetAnalysisRecordByUniqueKeys(ctx context.Context, uniqueKey []string) (record map[string]*entity.AnalysisRecord, err error) {
	return nil, err
}

func (e evaluationAnalysisServiceImpl) TrajectoryAnalysis(ctx context.Context, param TrajectoryAnalysisParam) (recordID int64, err error) {
	return 0, err
}
