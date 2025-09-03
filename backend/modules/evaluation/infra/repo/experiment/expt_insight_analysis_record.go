// Copyright (c) 2025 Bytedance Ltd. and/or its affiliates
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
)

type ExptInsightAnalysisRecordRepo struct {
	exptInsightAnalysisRecordDAO          mysql.IExptInsightAnalysisRecordDAO
	exptInsightAnalysisFeedbackCommentDAO mysql.IExptInsightAnalysisFeedbackCommentDAO
	exptInsightAnalysisFeedbackVoteDAO    mysql.IExptInsightAnalysisFeedbackVoteDAO
	idgenerator                           idgen.IIDGenerator
}

func NewExptInsightAnalysisRecordRepo(
	exptInsightAnalysisRecordDAO mysql.IExptInsightAnalysisRecordDAO,
	exptInsightAnalysisFeedbackCommentDAO mysql.IExptInsightAnalysisFeedbackCommentDAO,
	exptInsightAnalysisFeedbackVoteDAO mysql.IExptInsightAnalysisFeedbackVoteDAO,
	idgenerator idgen.IIDGenerator,
) repo.IExptInsightAnalysisRecordRepo {
	return &ExptInsightAnalysisRecordRepo{
		exptInsightAnalysisRecordDAO:          exptInsightAnalysisRecordDAO,
		exptInsightAnalysisFeedbackCommentDAO: exptInsightAnalysisFeedbackCommentDAO,
		exptInsightAnalysisFeedbackVoteDAO:    exptInsightAnalysisFeedbackVoteDAO,
		idgenerator:                           idgenerator,
	}
}

func (e ExptInsightAnalysisRecordRepo) CreateAnalysisRecord(ctx context.Context, record *entity.ExptInsightAnalysisRecord, opts ...db.Option) (int64, error) {
	id, err := e.idgenerator.GenID(ctx)
	if err != nil {
		return 0, err
	}
	record.ID = id

	err = e.exptInsightAnalysisRecordDAO.Create(ctx, convert.ExptInsightAnalysisRecordDOToPO(record), opts...)
	if err != nil {
		return 0, err
	}

	return id, nil
}

func (e ExptInsightAnalysisRecordRepo) UpdateAnalysisRecord(ctx context.Context, record *entity.ExptInsightAnalysisRecord, opts ...db.Option) error {
	return e.exptInsightAnalysisRecordDAO.Update(ctx, convert.ExptInsightAnalysisRecordDOToPO(record), opts...)
}

func (e ExptInsightAnalysisRecordRepo) GetAnalysisRecordByID(ctx context.Context, spaceID, exptID, recordID int64) (*entity.ExptInsightAnalysisRecord, error) {
	po, err := e.exptInsightAnalysisRecordDAO.GetByID(ctx, spaceID, exptID, recordID)
	if err != nil {
		return nil, err
	}

	return convert.ExptInsightAnalysisRecordPOToDO(po), nil
}

func (e ExptInsightAnalysisRecordRepo) ListAnalysisRecord(ctx context.Context, spaceID, exptID int64, page entity.Page) ([]*entity.ExptInsightAnalysisRecord, int64, error) {
	pos, total, err := e.exptInsightAnalysisRecordDAO.List(ctx, spaceID, exptID, page)
	if err != nil {
		return nil, 0, err
	}

	dos := make([]*entity.ExptInsightAnalysisRecord, 0)
	for _, po := range pos {
		dos = append(dos, convert.ExptInsightAnalysisRecordPOToDO(po))
	}
	return dos, total, nil
}

func (e ExptInsightAnalysisRecordRepo) DeleteAnalysisRecord(ctx context.Context, spaceID, exptID, recordID int64) error {
	return e.exptInsightAnalysisRecordDAO.Delete(ctx, spaceID, exptID, recordID)
}

func (e ExptInsightAnalysisRecordRepo) CreateFeedbackComment(ctx context.Context, feedbackComment *entity.ExptInsightAnalysisFeedbackComment, opts ...db.Option) error {
	id, err := e.idgenerator.GenID(ctx)
	if err != nil {
		return err
	}
	feedbackComment.ID = id
	return e.exptInsightAnalysisFeedbackCommentDAO.Create(ctx, convert.ExptInsightAnalysisFeedbackCommentDOToPO(feedbackComment), opts...)
}

func (e ExptInsightAnalysisRecordRepo) UpdateFeedbackComment(ctx context.Context, feedbackComment *entity.ExptInsightAnalysisFeedbackComment, opts ...db.Option) error {
	return e.exptInsightAnalysisFeedbackCommentDAO.Update(ctx, convert.ExptInsightAnalysisFeedbackCommentDOToPO(feedbackComment), opts...)
}

func (e ExptInsightAnalysisRecordRepo) GetFeedbackCommentByRecordID(ctx context.Context, spaceID, exptID, recordID int64, opts ...db.Option) (*entity.ExptInsightAnalysisFeedbackComment, error) {
	po, err := e.exptInsightAnalysisFeedbackCommentDAO.GetByRecordID(ctx, spaceID, exptID, recordID, opts...)
	if err != nil {
		return nil, err
	}
	return convert.ExptInsightAnalysisFeedbackCommentPOToDO(po), nil
}

func (e ExptInsightAnalysisRecordRepo) DeleteFeedbackComment(ctx context.Context, spaceID, exptID, commentID int64) error {
	return e.exptInsightAnalysisFeedbackCommentDAO.Delete(ctx, spaceID, exptID, commentID)
}

func (e ExptInsightAnalysisRecordRepo) CreateFeedbackVote(ctx context.Context, feedbackVote *entity.ExptInsightAnalysisFeedbackVote, opts ...db.Option) error {
	id, err := e.idgenerator.GenID(ctx)
	if err != nil {
		return err
	}
	feedbackVote.ID = id
	return e.exptInsightAnalysisFeedbackVoteDAO.Create(ctx, convert.ExptInsightAnalysisFeedbackVoteDOToPO(feedbackVote), opts...)
}

func (e ExptInsightAnalysisRecordRepo) UpdateFeedbackVote(ctx context.Context, feedbackVote *entity.ExptInsightAnalysisFeedbackVote, opts ...db.Option) error {
	return e.exptInsightAnalysisFeedbackVoteDAO.Update(ctx, convert.ExptInsightAnalysisFeedbackVoteDOToPO(feedbackVote), opts...)
}

func (e ExptInsightAnalysisRecordRepo) GetFeedbackVoteByUser(ctx context.Context, spaceID, exptID, recordID int64, userID string, opts ...db.Option) (*entity.ExptInsightAnalysisFeedbackVote, error) {
	po, err := e.exptInsightAnalysisFeedbackVoteDAO.GetByUser(ctx, spaceID, exptID, recordID, userID, opts...)
	if err != nil {
		return nil, err
	}
	return convert.ExptInsightAnalysisFeedbackVotePOToDO(po), nil
}

func (e ExptInsightAnalysisRecordRepo) CountFeedbackVote(ctx context.Context, spaceID, exptID, recordID int64) (int64, int64, error) {
	return e.exptInsightAnalysisFeedbackVoteDAO.Count(ctx, spaceID, exptID, recordID)
}

func (e ExptInsightAnalysisRecordRepo) List(ctx context.Context, spaceID, exptID, recordID int64, page entity.Page) ([]*entity.ExptInsightAnalysisFeedbackComment, int64, error) {
	pos, total, err := e.exptInsightAnalysisFeedbackCommentDAO.List(ctx, spaceID, exptID, recordID, page)
	if err != nil {
		return nil, 0, err
	}
	dos := make([]*entity.ExptInsightAnalysisFeedbackComment, 0)
	for _, po := range pos {
		dos = append(dos, convert.ExptInsightAnalysisFeedbackCommentPOToDO(po))
	}
	return dos, total, nil
}
