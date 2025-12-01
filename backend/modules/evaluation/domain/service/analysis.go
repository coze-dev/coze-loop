// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate mockgen -destination=mocks/evaluation_analysis.go -package=mocks . IEvaluationAnalysisService
type IEvaluationAnalysisService interface {
	GetAnalysisRecord(ctx context.Context, id int64) (record *entity.AnalysisRecord, err error)
	BatchGetAnalysisRecordByUniqueKeys(ctx context.Context, uniqueKey []string) (record map[string]*entity.AnalysisRecord, err error)
	TrajectoryAnalysis(ctx context.Context, param TrajectoryAnalysisParam) (recordID int64, err error)
}

type TrajectoryAnalysisParam struct {
	WorkspaceID int64
	ExptID      int64
	ItemID      int64
	TurnID      int64
}
