// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
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
	writeTracker                          platestwrite.ILatestWriteTracker
}

func NewExptInsightAnalysisRecordRepo(
	exptInsightAnalysisRecordDAO mysql.IExptInsightAnalysisRecordDAO,
	exptInsightAnalysisFeedbackCommentDAO mysql.IExptInsightAnalysisFeedbackCommentDAO,
	exptInsightAnalysisFeedbackVoteDAO mysql.IExptInsightAnalysisFeedbackVoteDAO,
	idgenerator idgen.IIDGenerator,
	writeTracker platestwrite.ILatestWriteTracker,
) repo.IExptInsightAnalysisRecordRepo {
	return &ExptInsightAnalysisRecordRepo{
		exptInsightAnalysisRecordDAO:          exptInsightAnalysisRecordDAO,
		exptInsightAnalysisFeedbackCommentDAO: exptInsightAnalysisFeedbackCommentDAO,
		exptInsightAnalysisFeedbackVoteDAO:    exptInsightAnalysisFeedbackVoteDAO,
		idgenerator:                           idgenerator,
		writeTracker:                          writeTracker,
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

	if e.writeTracker != nil {
		e.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypeExptInsightAnalysisRecord, id,
			platestwrite.SetWithSearchParam(buildRecordSearchParam(record.SpaceID, record.ExptID)))
	}

	return id, nil
}

func (e ExptInsightAnalysisRecordRepo) UpdateAnalysisRecord(ctx context.Context, record *entity.ExptInsightAnalysisRecord, opts ...db.Option) error {
	if err := e.exptInsightAnalysisRecordDAO.Update(ctx, convert.ExptInsightAnalysisRecordDOToPO(record), opts...); err != nil {
		return err
	}

	if e.writeTracker != nil {
		e.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypeExptInsightAnalysisRecord, record.ID,
			platestwrite.SetWithSearchParam(buildRecordSearchParam(record.SpaceID, record.ExptID)))
	}

	return nil
}

func (e ExptInsightAnalysisRecordRepo) GetAnalysisRecordByID(ctx context.Context, spaceID, exptID, recordID int64) (*entity.ExptInsightAnalysisRecord, error) {
	opts := make([]db.Option, 0)
	if e.needForceMasterByRecord(ctx, platestwrite.ResourceTypeExptInsightAnalysisRecord, recordID, buildRecordSearchParam(spaceID, exptID)) {
		opts = append(opts, db.WithMaster())
	}

	po, err := e.exptInsightAnalysisRecordDAO.GetByID(ctx, spaceID, exptID, recordID, opts...)
	if err != nil {
		return nil, err
	}

	return convert.ExptInsightAnalysisRecordPOToDO(po), nil
}

func (e ExptInsightAnalysisRecordRepo) ListAnalysisRecord(ctx context.Context, spaceID, exptID int64, page entity.Page) ([]*entity.ExptInsightAnalysisRecord, int64, error) {
	opts := make([]db.Option, 0)
	if e.needForceMasterByRecord(ctx, platestwrite.ResourceTypeExptInsightAnalysisRecord, 0, buildRecordSearchParam(spaceID, exptID)) {
		opts = append(opts, db.WithMaster())
	}
	pos, total, err := e.exptInsightAnalysisRecordDAO.List(ctx, spaceID, exptID, page, opts...)
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
	if err := e.exptInsightAnalysisRecordDAO.Delete(ctx, spaceID, exptID, recordID); err != nil {
		return err
	}
	if e.writeTracker != nil {
		e.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypeExptInsightAnalysisRecord, recordID,
			platestwrite.SetWithSearchParam(buildRecordSearchParam(spaceID, exptID)))
	}
	return nil
}

func (e ExptInsightAnalysisRecordRepo) CreateFeedbackComment(ctx context.Context, feedbackComment *entity.ExptInsightAnalysisFeedbackComment, opts ...db.Option) error {
	id, err := e.idgenerator.GenID(ctx)
	if err != nil {
		return err
	}
	feedbackComment.ID = id
	if err := e.exptInsightAnalysisFeedbackCommentDAO.Create(ctx, convert.ExptInsightAnalysisFeedbackCommentDOToPO(feedbackComment), opts...); err != nil {
		return err
	}
	if e.writeTracker != nil {
		e.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypeExptInsightAnalysisFeedback, feedbackComment.AnalysisRecordID,
			platestwrite.SetWithSearchParam(buildFeedbackSearchParam(feedbackComment.SpaceID, feedbackComment.ExptID, feedbackComment.AnalysisRecordID)))
	}
	return nil
}

func (e ExptInsightAnalysisRecordRepo) UpdateFeedbackComment(ctx context.Context, feedbackComment *entity.ExptInsightAnalysisFeedbackComment, opts ...db.Option) error {
	if err := e.exptInsightAnalysisFeedbackCommentDAO.Update(ctx, convert.ExptInsightAnalysisFeedbackCommentDOToPO(feedbackComment), opts...); err != nil {
		return err
	}
	if e.writeTracker != nil {
		e.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypeExptInsightAnalysisFeedback, feedbackComment.AnalysisRecordID,
			platestwrite.SetWithSearchParam(buildFeedbackSearchParam(feedbackComment.SpaceID, feedbackComment.ExptID, feedbackComment.AnalysisRecordID)))
	}
	return nil
}

func (e ExptInsightAnalysisRecordRepo) GetFeedbackCommentByRecordID(ctx context.Context, spaceID, exptID, recordID int64, opts ...db.Option) (*entity.ExptInsightAnalysisFeedbackComment, error) {
	innerOpts := append([]db.Option{}, opts...)
	if e.needForceMasterByRecord(ctx, platestwrite.ResourceTypeExptInsightAnalysisFeedback, recordID, buildFeedbackSearchParam(spaceID, exptID, recordID)) && !db.ContainWithMasterOpt(innerOpts) {
		innerOpts = append(innerOpts, db.WithMaster())
	}
	po, err := e.exptInsightAnalysisFeedbackCommentDAO.GetByRecordID(ctx, spaceID, exptID, recordID, innerOpts...)
	if err != nil {
		return nil, err
	}
	return convert.ExptInsightAnalysisFeedbackCommentPOToDO(po), nil
}

func (e ExptInsightAnalysisRecordRepo) DeleteFeedbackComment(ctx context.Context, spaceID, exptID, commentID int64) error {
	po, err := e.exptInsightAnalysisFeedbackCommentDAO.GetByID(ctx, spaceID, exptID, commentID, db.WithMaster())
	if err != nil {
		return err
	}
	if err := e.exptInsightAnalysisFeedbackCommentDAO.Delete(ctx, spaceID, exptID, commentID); err != nil {
		return err
	}
	recordID := int64(0)
	if po.AnalysisRecordID != nil {
		recordID = *po.AnalysisRecordID
	}
	if e.writeTracker != nil && recordID > 0 {
		e.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypeExptInsightAnalysisFeedback, recordID,
			platestwrite.SetWithSearchParam(buildFeedbackSearchParam(po.SpaceID, po.ExptID, recordID)))
	}
	return nil
}

func (e ExptInsightAnalysisRecordRepo) CreateFeedbackVote(ctx context.Context, feedbackVote *entity.ExptInsightAnalysisFeedbackVote, opts ...db.Option) error {
	id, err := e.idgenerator.GenID(ctx)
	if err != nil {
		return err
	}
	feedbackVote.ID = id
	if err := e.exptInsightAnalysisFeedbackVoteDAO.Create(ctx, convert.ExptInsightAnalysisFeedbackVoteDOToPO(feedbackVote), opts...); err != nil {
		return err
	}
	if e.writeTracker != nil {
		e.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypeExptInsightAnalysisFeedback, feedbackVote.AnalysisRecordID,
			platestwrite.SetWithSearchParam(buildFeedbackSearchParam(feedbackVote.SpaceID, feedbackVote.ExptID, feedbackVote.AnalysisRecordID)))
	}
	return nil
}

func (e ExptInsightAnalysisRecordRepo) UpdateFeedbackVote(ctx context.Context, feedbackVote *entity.ExptInsightAnalysisFeedbackVote, opts ...db.Option) error {
	if err := e.exptInsightAnalysisFeedbackVoteDAO.Update(ctx, convert.ExptInsightAnalysisFeedbackVoteDOToPO(feedbackVote), opts...); err != nil {
		return err
	}
	if e.writeTracker != nil {
		e.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypeExptInsightAnalysisFeedback, feedbackVote.AnalysisRecordID,
			platestwrite.SetWithSearchParam(buildFeedbackSearchParam(feedbackVote.SpaceID, feedbackVote.ExptID, feedbackVote.AnalysisRecordID)))
	}
	return nil
}

func (e ExptInsightAnalysisRecordRepo) GetFeedbackVoteByUser(ctx context.Context, spaceID, exptID, recordID int64, userID string, opts ...db.Option) (*entity.ExptInsightAnalysisFeedbackVote, error) {
	innerOpts := append([]db.Option{}, opts...)
	if e.needForceMasterByRecord(ctx, platestwrite.ResourceTypeExptInsightAnalysisFeedback, recordID, buildFeedbackSearchParam(spaceID, exptID, recordID)) && !db.ContainWithMasterOpt(innerOpts) {
		innerOpts = append(innerOpts, db.WithMaster())
	}
	po, err := e.exptInsightAnalysisFeedbackVoteDAO.GetByUser(ctx, spaceID, exptID, recordID, userID, innerOpts...)
	if err != nil {
		return nil, err
	}
	return convert.ExptInsightAnalysisFeedbackVotePOToDO(po), nil
}

func (e ExptInsightAnalysisRecordRepo) CountFeedbackVote(ctx context.Context, spaceID, exptID, recordID int64) (int64, int64, error) {
	opts := make([]db.Option, 0)
	if e.needForceMasterByRecord(ctx, platestwrite.ResourceTypeExptInsightAnalysisFeedback, recordID, buildFeedbackSearchParam(spaceID, exptID, recordID)) {
		opts = append(opts, db.WithMaster())
	}
	return e.exptInsightAnalysisFeedbackVoteDAO.Count(ctx, spaceID, exptID, recordID, opts...)
}

func (e ExptInsightAnalysisRecordRepo) List(ctx context.Context, spaceID, exptID, recordID int64, page entity.Page) ([]*entity.ExptInsightAnalysisFeedbackComment, int64, error) {
	opts := make([]db.Option, 0)
	if e.needForceMasterByRecord(ctx, platestwrite.ResourceTypeExptInsightAnalysisFeedback, recordID, buildFeedbackSearchParam(spaceID, exptID, recordID)) {
		opts = append(opts, db.WithMaster())
	}
	pos, total, err := e.exptInsightAnalysisFeedbackCommentDAO.List(ctx, spaceID, exptID, recordID, page, opts...)
	if err != nil {
		return nil, 0, err
	}
	dos := make([]*entity.ExptInsightAnalysisFeedbackComment, 0)
	for _, po := range pos {
		dos = append(dos, convert.ExptInsightAnalysisFeedbackCommentPOToDO(po))
	}
	return dos, total, nil
}

func (e ExptInsightAnalysisRecordRepo) needForceMasterByRecord(ctx context.Context, resourceType platestwrite.ResourceType, resourceID int64, searchParam string) bool {
	if e.writeTracker == nil {
		return false
	}
	if resourceID > 0 && e.writeTracker.CheckWriteFlagByID(ctx, resourceType, resourceID) {
		return true
	}
	if searchParam != "" && e.writeTracker.CheckWriteFlagBySearchParam(ctx, resourceType, searchParam) {
		return true
	}
	return false
}

func buildRecordSearchParam(spaceID, exptID int64) string {
	return fmt.Sprintf("%d:%d", spaceID, exptID)
}

func buildFeedbackSearchParam(spaceID, exptID, recordID int64) string {
	return fmt.Sprintf("%d:%d:%d", spaceID, exptID, recordID)
}
