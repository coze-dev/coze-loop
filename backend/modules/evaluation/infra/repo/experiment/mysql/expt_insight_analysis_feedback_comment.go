// Copyright (c) 2025 Bytedance Ltd. and/or its affiliates
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/query"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

type IExptInsightAnalysisFeedbackCommentDAO interface {
	Create(ctx context.Context, feedbackComment *model.ExptInsightAnalysisFeedbackComment, opts ...db.Option) error
	Update(ctx context.Context, feedbackComment *model.ExptInsightAnalysisFeedbackComment, opts ...db.Option) error
	GetByRecordID(ctx context.Context, spaceID, exptID, recordID int64, opts ...db.Option) (*model.ExptInsightAnalysisFeedbackComment, error)
	Delete(ctx context.Context, spaceID, exptID, commentID int64) error
	List(ctx context.Context, spaceID, exptID, recordID int64, page entity.Page) ([]*model.ExptInsightAnalysisFeedbackComment, int64, error)
}

func NewExptInsightAnalysisFeedbackCommentDAO(db db.Provider) IExptInsightAnalysisFeedbackCommentDAO {
	return &exptInsightAnalysisFeedbackCommentDAO{
		db:    db,
		query: query.Use(db.NewSession(context.Background())),
	}
}

type exptInsightAnalysisFeedbackCommentDAO struct {
	db    db.Provider
	query *query.Query
}

func (e exptInsightAnalysisFeedbackCommentDAO) Create(ctx context.Context, feedbackComment *model.ExptInsightAnalysisFeedbackComment, opts ...db.Option) error {
	if err := e.db.NewSession(ctx, opts...).Create(feedbackComment).Error; err != nil {
		return errorx.Wrapf(err, "exptInsightAnalysisFeedbackCommentDAO create fail, model: %v", json.Jsonify(feedbackComment))
	}
	return nil
}

func (e exptInsightAnalysisFeedbackCommentDAO) Update(ctx context.Context, feedbackComment *model.ExptInsightAnalysisFeedbackComment, opts ...db.Option) error {
	if err := e.db.NewSession(ctx, opts...).Model(&model.ExptInsightAnalysisFeedbackComment{}).Where("id = ?", feedbackComment.ID).Updates(feedbackComment).Error; err != nil {
		return errorx.Wrapf(err, "exptInsightAnalysisFeedbackCommentDAO update fail, model: %v", json.Jsonify(feedbackComment))
	}
	return nil
}

func (e exptInsightAnalysisFeedbackCommentDAO) GetByRecordID(ctx context.Context, spaceID, exptID, recordID int64, opts ...db.Option) (*model.ExptInsightAnalysisFeedbackComment, error) {
	db := e.db.NewSession(ctx)
	q := query.Use(db).ExptInsightAnalysisFeedbackComment

	feedbackVote, err := q.WithContext(ctx).Where(
		q.SpaceID.Eq(spaceID),
		q.ExptID.Eq(exptID),
		q.AnalysisRecordID.Eq(recordID),
	).First()
	if err != nil {
		return nil, errorx.Wrapf(err, "exptInsightAnalysisFeedbackCommentDAO GetByRecordID fail, spaceID: %v, exptID: %v", spaceID, exptID)
	}

	return feedbackVote, nil
}

func (e exptInsightAnalysisFeedbackCommentDAO) Delete(ctx context.Context, spaceID, exptID, commentID int64) error {
	po := &model.ExptInsightAnalysisFeedbackComment{}
	db := e.db.NewSession(ctx)
	err := db.Where("space_id = ? AND expt_id = ?  AND id = ?", spaceID, exptID, commentID).
		Delete(po).Error
	if err != nil {
		return err
	}

	return nil
}

func (e exptInsightAnalysisFeedbackCommentDAO) List(ctx context.Context, spaceID, exptID, recordID int64, page entity.Page) ([]*model.ExptInsightAnalysisFeedbackComment, int64, error) {
	var (
		finds []*model.ExptInsightAnalysisFeedbackComment
		total int64
	)
	db := e.db.NewSession(ctx).Model(&model.ExptInsightAnalysisFeedbackComment{}).
		Where("space_id =?", spaceID).
		Where("expt_id =?", exptID).
		Where("analysis_record_id =?", recordID)
	// 总记录数
	db = db.Count(&total)
	// 分页
	db = db.Offset(page.Offset()).Limit(page.Limit())
	err := db.Find(&finds).Error
	if err != nil {
		return nil, 0, err
	}
	return finds, total, nil
}
