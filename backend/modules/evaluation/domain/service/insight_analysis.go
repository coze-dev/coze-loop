// Copyright (c) 2025 Bytedance Ltd. and/or its affiliates
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type IExptInsightAnalysisService interface {
	CreateAnalysisRecord(ctx context.Context, record *entity.ExptInsightAnalysisRecord, session *entity.Session) (int64, error)
	GenAnalysisReport(ctx context.Context, spaceID, exptID, recordID int64) error
	GetAnalysisRecordByID(ctx context.Context, spaceID, exptID, recordID int64, session *entity.Session) (*entity.ExptInsightAnalysisRecord, error)
	ListAnalysisRecord(ctx context.Context, spaceID, exptID int64, page entity.Page, session *entity.Session) ([]*entity.ExptInsightAnalysisRecord, int64, error)
	DeleteAnalysisRecord(ctx context.Context, spaceID, exptID, recordID int64) error
	FeedbackExptInsightAnalysis(ctx context.Context, param *entity.ExptInsightAnalysisFeedbackParam) error
	ListExptInsightAnalysisFeedbackComment(ctx context.Context, spaceID, exptID, recordID int64, page entity.Page) ([]*entity.ExptInsightAnalysisFeedbackComment, int64, error)
}
