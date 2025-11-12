// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/query"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

//go:generate  mockgen -destination=mocks/expt_insight_analysis_feedback_vote.go  -package mocks . IExptInsightAnalysisFeedbackVoteDAO
type IExptInsightAnalysisFeedbackVoteDAO interface {
	Create(ctx context.Context, feedbackVote *model.ExptInsightAnalysisFeedbackVote, opts ...db.Option) error
	Update(ctx context.Context, feedbackVote *model.ExptInsightAnalysisFeedbackVote, opts ...db.Option) error
	GetByUser(ctx context.Context, spaceID, exptID, recordID int64, userID string, opts ...db.Option) (*model.ExptInsightAnalysisFeedbackVote, error)
	Count(ctx context.Context, spaceID, exptID, recordID int64, opts ...db.Option) (int64, int64, error)
}

func NewExptInsightAnalysisFeedbackVoteDAO(db db.Provider) IExptInsightAnalysisFeedbackVoteDAO {
	return &exptInsightAnalysisFeedbackVoteDAO{
		db:    db,
		query: query.Use(db.NewSession(context.Background())),
	}
}

type exptInsightAnalysisFeedbackVoteDAO struct {
	db    db.Provider
	query *query.Query
}

func (e exptInsightAnalysisFeedbackVoteDAO) Create(ctx context.Context, feedbackVote *model.ExptInsightAnalysisFeedbackVote, opts ...db.Option) error {
	if err := e.db.NewSession(ctx, opts...).Save(feedbackVote).Error; err != nil {
		return errorx.Wrapf(err, "exptInsightAnalysisFeedbackVoteDAO create fail, model: %v", json.Jsonify(feedbackVote))
	}
	return nil
}

func (e exptInsightAnalysisFeedbackVoteDAO) Update(ctx context.Context, feedbackVote *model.ExptInsightAnalysisFeedbackVote, opts ...db.Option) error {
	if err := e.db.NewSession(ctx, opts...).Model(&model.ExptInsightAnalysisFeedbackVote{}).
		Where("space_id = ?", feedbackVote.SpaceID).
		Where("expt_id = ?", feedbackVote.ExptID).
		Where("analysis_record_id = ?", feedbackVote.AnalysisRecordID).
		Where("created_by = ?", feedbackVote.CreatedBy).Updates(map[string]interface{}{
		"vote_type": feedbackVote.VoteType,
	}).Error; err != nil {
		return errorx.Wrapf(err, "exptInsightAnalysisFeedbackVoteDAO update fail, model: %v", json.Jsonify(feedbackVote))
	}
	return nil
}

func (e exptInsightAnalysisFeedbackVoteDAO) GetByUser(ctx context.Context, spaceID, exptID, recordID int64, userID string, opts ...db.Option) (*model.ExptInsightAnalysisFeedbackVote, error) {
	db := e.db.NewSession(ctx, opts...)
	q := query.Use(db).ExptInsightAnalysisFeedbackVote

	feedbackVote, err := q.WithContext(ctx).Where(
		q.SpaceID.Eq(spaceID),
		q.ExptID.Eq(exptID),
		q.AnalysisRecordID.Eq(recordID),
		q.CreatedBy.Eq(userID),
	).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, errorx.Wrapf(err, "exptInsightAnalysisFeedbackVoteDAO GetByUser fail, spaceID: %v, exptID: %v", spaceID, exptID)
	}

	return feedbackVote, nil
}

func (e exptInsightAnalysisFeedbackVoteDAO) Count(ctx context.Context, spaceID, exptID, recordID int64, opts ...db.Option) (int64, int64, error) {
	db := e.db.NewSession(ctx, opts...)
	type VoteStatistic struct {
		UpvoteCount   int64 `json:"upvote_count"`
		DownvoteCount int64 `json:"downvote_count"`
	}
	voteStatistic := &VoteStatistic{}
	err := db.WithContext(ctx).Model(&model.ExptInsightAnalysisFeedbackVote{}).
		Select(fmt.Sprintf("SUM(CASE WHEN vote_type = %v THEN 1 ELSE 0 END) AS upvote_count, SUM(CASE WHEN vote_type = %v THEN 1 ELSE 0 END) AS downvote_count", entity.Upvote, entity.Downvote)).
		Where("space_id = ? AND expt_id = ? AND analysis_record_id = ?", spaceID, exptID, recordID).
		Scan(voteStatistic).Error
	if err != nil {
		return 0, 0, errorx.Wrapf(err, "exptInsightAnalysisFeedbackVoteDAO Count fail, spaceID: %v, exptID: %v", spaceID, exptID)
	}
	return voteStatistic.UpvoteCount, voteStatistic.DownvoteCount, nil
}
