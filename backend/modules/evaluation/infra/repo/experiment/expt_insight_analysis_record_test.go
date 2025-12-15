// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	mockidgen "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	platestwritemocks "github.com/coze-dev/coze-loop/backend/infra/platestwrite/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type testMocks struct {
	analysisRecordDAO  *mocks.MockIExptInsightAnalysisRecordDAO
	feedbackCommentDAO *mocks.MockIExptInsightAnalysisFeedbackCommentDAO
	feedbackVoteDAO    *mocks.MockIExptInsightAnalysisFeedbackVoteDAO
	idGenerator        *mockidgen.MockIIDGenerator
	writeTracker       *platestwritemocks.MockILatestWriteTracker
}

func newTestExptInsightAnalysisRecordRepo(ctrl *gomock.Controller) (*ExptInsightAnalysisRecordRepo, *testMocks) {
	mocks := &testMocks{
		analysisRecordDAO:  mocks.NewMockIExptInsightAnalysisRecordDAO(ctrl),
		feedbackCommentDAO: mocks.NewMockIExptInsightAnalysisFeedbackCommentDAO(ctrl),
		feedbackVoteDAO:    mocks.NewMockIExptInsightAnalysisFeedbackVoteDAO(ctrl),
		idGenerator:        mockidgen.NewMockIIDGenerator(ctrl),
		writeTracker:       platestwritemocks.NewMockILatestWriteTracker(ctrl),
	}

	repo := &ExptInsightAnalysisRecordRepo{
		exptInsightAnalysisRecordDAO:          mocks.analysisRecordDAO,
		exptInsightAnalysisFeedbackCommentDAO: mocks.feedbackCommentDAO,
		exptInsightAnalysisFeedbackVoteDAO:    mocks.feedbackVoteDAO,
		idgenerator:                           mocks.idGenerator,
		writeTracker:                          mocks.writeTracker,
	}

	return repo, mocks
}

func expectWriteFlagAny(m *testMocks) {
	m.writeTracker.EXPECT().SetWriteFlag(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
}

func expectNoWriteFlagRead(m *testMocks) {
	m.writeTracker.EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	m.writeTracker.EXPECT().CheckWriteFlagBySearchParam(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
}

func TestExptInsightAnalysisRecordRepo_CreateAnalysisRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo, mocks := newTestExptInsightAnalysisRecordRepo(ctrl)
	expectWriteFlagAny(mocks)
	expectWriteFlagAny(mocks)
	expectWriteFlagAny(mocks)

	record := &entity.ExptInsightAnalysisRecord{
		SpaceID:               1,
		ExptID:                1,
		Status:                entity.InsightAnalysisStatus_Running,
		AnalysisReportContent: "test content",
		CreatedBy:             "user123",
	}

	mocks.idGenerator.EXPECT().GenID(gomock.Any()).Return(int64(123), nil)
	mocks.analysisRecordDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	id, err := repo.CreateAnalysisRecord(context.Background(), record)

	assert.NoError(t, err)
	assert.Equal(t, int64(123), id)
	assert.Equal(t, int64(123), record.ID)
}

func TestExptInsightAnalysisRecordRepo_UpdateAnalysisRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo, mocks := newTestExptInsightAnalysisRecordRepo(ctrl)
	expectWriteFlagAny(mocks)
	expectWriteFlagAny(mocks)
	expectWriteFlagAny(mocks)

	record := &entity.ExptInsightAnalysisRecord{
		ID:                    1,
		SpaceID:               1,
		ExptID:                1,
		Status:                entity.InsightAnalysisStatus_Success,
		AnalysisReportContent: "updated content",
	}

	mocks.analysisRecordDAO.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	err := repo.UpdateAnalysisRecord(context.Background(), record)

	assert.NoError(t, err)
}

func TestExptInsightAnalysisRecordRepo_GetAnalysisRecordByID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo, mocks := newTestExptInsightAnalysisRecordRepo(ctrl)
	expectNoWriteFlagRead(mocks)

	mocks.analysisRecordDAO.EXPECT().GetByID(gomock.Any(), int64(1), int64(1), int64(1)).Return(&model.ExptInsightAnalysisRecord{
		ID:        1,
		SpaceID:   1,
		ExptID:    1,
		Status:    int32(entity.InsightAnalysisStatus_Success),
		CreatedBy: "test-user",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil)

	record, err := repo.GetAnalysisRecordByID(context.Background(), 1, 1, 1)

	assert.NoError(t, err)
	assert.NotNil(t, record)
	assert.Equal(t, int64(1), record.ID)
}

func TestExptInsightAnalysisRecordRepo_GetAnalysisRecordByID_ForceMaster(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo, mocks := newTestExptInsightAnalysisRecordRepo(ctrl)

	mocks.writeTracker.EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), int64(1)).Return(true)
	mocks.writeTracker.EXPECT().CheckWriteFlagBySearchParam(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	mocks.analysisRecordDAO.EXPECT().GetByID(gomock.Any(), int64(1), int64(1), int64(1), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ int64, _ int64, _ int64, opts ...db.Option) (*model.ExptInsightAnalysisRecord, error) {
			assert.True(t, db.ContainWithMasterOpt(opts))
			return &model.ExptInsightAnalysisRecord{ID: 1, SpaceID: 1, ExptID: 1}, nil
		},
	)

	record, err := repo.GetAnalysisRecordByID(context.Background(), 1, 1, 1)

	assert.NoError(t, err)
	assert.NotNil(t, record)
	assert.Equal(t, int64(1), record.ID)
}

func TestExptInsightAnalysisRecordRepo_ListAnalysisRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo, mocks := newTestExptInsightAnalysisRecordRepo(ctrl)
	expectNoWriteFlagRead(mocks)

	mocks.analysisRecordDAO.EXPECT().List(gomock.Any(), int64(1), int64(1), entity.NewPage(1, 10)).Return([]*model.ExptInsightAnalysisRecord{
		{
			ID:        1,
			SpaceID:   1,
			ExptID:    1,
			Status:    int32(entity.InsightAnalysisStatus_Success),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}, int64(1), nil)

	records, total, err := repo.ListAnalysisRecord(context.Background(), 1, 1, entity.NewPage(1, 10))

	assert.NoError(t, err)
	assert.Len(t, records, 1)
	assert.Equal(t, int64(1), total)
}

func TestExptInsightAnalysisRecordRepo_DeleteAnalysisRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo, mocks := newTestExptInsightAnalysisRecordRepo(ctrl)
	expectWriteFlagAny(mocks)

	mocks.analysisRecordDAO.EXPECT().Delete(gomock.Any(), int64(1), int64(1), int64(1)).Return(nil)

	err := repo.DeleteAnalysisRecord(context.Background(), 1, 1, 1)

	assert.NoError(t, err)
}

func TestExptInsightAnalysisRecordRepo_CreateFeedbackComment(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo, mocks := newTestExptInsightAnalysisRecordRepo(ctrl)
	expectWriteFlagAny(mocks)

	comment := &entity.ExptInsightAnalysisFeedbackComment{
		SpaceID:          1,
		ExptID:           1,
		AnalysisRecordID: 1,
		Comment:          "test comment",
		CreatedBy:        "user123",
	}

	mocks.idGenerator.EXPECT().GenID(gomock.Any()).Return(int64(456), nil)
	mocks.feedbackCommentDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	err := repo.CreateFeedbackComment(context.Background(), comment)

	assert.NoError(t, err)
	assert.Equal(t, int64(456), comment.ID)
}

func TestExptInsightAnalysisRecordRepo_UpdateFeedbackComment(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo, mocks := newTestExptInsightAnalysisRecordRepo(ctrl)
	expectWriteFlagAny(mocks)

	comment := &entity.ExptInsightAnalysisFeedbackComment{
		ID:               1,
		SpaceID:          1,
		ExptID:           1,
		AnalysisRecordID: 1,
		Comment:          "updated comment",
	}

	mocks.feedbackCommentDAO.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	err := repo.UpdateFeedbackComment(context.Background(), comment)

	assert.NoError(t, err)
}

func TestExptInsightAnalysisRecordRepo_DeleteFeedbackComment(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo, mocks := newTestExptInsightAnalysisRecordRepo(ctrl)
	expectWriteFlagAny(mocks)

	mocks.feedbackCommentDAO.EXPECT().GetByID(gomock.Any(), int64(1), int64(1), int64(1), gomock.Any()).Return(&model.ExptInsightAnalysisFeedbackComment{
		ID:               1,
		SpaceID:          1,
		ExptID:           1,
		AnalysisRecordID: ptr.Of(int64(1)),
	}, nil)
	mocks.feedbackCommentDAO.EXPECT().Delete(gomock.Any(), int64(1), int64(1), int64(1)).Return(nil)

	err := repo.DeleteFeedbackComment(context.Background(), 1, 1, 1)

	assert.NoError(t, err)
}

func TestExptInsightAnalysisRecordRepo_DeleteFeedbackComment_GetByIDError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo, mocks := newTestExptInsightAnalysisRecordRepo(ctrl)
	expectWriteFlagAny(mocks)

	mocks.feedbackCommentDAO.EXPECT().GetByID(gomock.Any(), int64(1), int64(1), int64(1), gomock.Any()).Return(nil, assert.AnError)

	err := repo.DeleteFeedbackComment(context.Background(), 1, 1, 1)

	assert.Error(t, err)
}

func TestExptInsightAnalysisRecordRepo_CreateFeedbackVote(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo, mocks := newTestExptInsightAnalysisRecordRepo(ctrl)
	expectWriteFlagAny(mocks)

	vote := &entity.ExptInsightAnalysisFeedbackVote{
		SpaceID:          1,
		ExptID:           1,
		AnalysisRecordID: 1,
		VoteType:         entity.Upvote,
		CreatedBy:        "user123",
	}

	mocks.idGenerator.EXPECT().GenID(gomock.Any()).Return(int64(789), nil)
	mocks.feedbackVoteDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	err := repo.CreateFeedbackVote(context.Background(), vote)

	assert.NoError(t, err)
	assert.Equal(t, int64(789), vote.ID)
}

func TestExptInsightAnalysisRecordRepo_UpdateFeedbackVote(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo, mocks := newTestExptInsightAnalysisRecordRepo(ctrl)
	expectWriteFlagAny(mocks)

	vote := &entity.ExptInsightAnalysisFeedbackVote{
		ID:               1,
		SpaceID:          1,
		ExptID:           1,
		AnalysisRecordID: 1,
		VoteType:         entity.Downvote,
	}

	mocks.feedbackVoteDAO.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	err := repo.UpdateFeedbackVote(context.Background(), vote)

	assert.NoError(t, err)
}

func TestExptInsightAnalysisRecordRepo_GetFeedbackVoteByUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo, mocks := newTestExptInsightAnalysisRecordRepo(ctrl)
	expectNoWriteFlagRead(mocks)

	mocks.feedbackVoteDAO.EXPECT().GetByUser(gomock.Any(), int64(1), int64(1), int64(1), "user123").Return(&model.ExptInsightAnalysisFeedbackVote{
		ID:               1,
		SpaceID:          1,
		ExptID:           1,
		AnalysisRecordID: ptr.Of(int64(1)),
		VoteType:         int32(entity.Upvote),
		CreatedBy:        "user123",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}, nil)

	vote, err := repo.GetFeedbackVoteByUser(context.Background(), 1, 1, 1, "user123")

	assert.NoError(t, err)
	assert.NotNil(t, vote)
	assert.Equal(t, entity.Upvote, vote.VoteType)
}

func TestExptInsightAnalysisRecordRepo_CountFeedbackVote(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo, mocks := newTestExptInsightAnalysisRecordRepo(ctrl)
	expectNoWriteFlagRead(mocks)

	mocks.feedbackVoteDAO.EXPECT().Count(gomock.Any(), int64(1), int64(1), int64(1)).Return(int64(3), int64(2), nil)

	upVoteCount, downVoteCount, err := repo.CountFeedbackVote(context.Background(), 1, 1, 1)

	assert.NoError(t, err)
	assert.Equal(t, int64(3), upVoteCount)
	assert.Equal(t, int64(2), downVoteCount)
}

func TestExptInsightAnalysisRecordRepo_CountFeedbackVote_ForceMaster(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo, mocks := newTestExptInsightAnalysisRecordRepo(ctrl)

	mocks.writeTracker.EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	mocks.writeTracker.EXPECT().CheckWriteFlagBySearchParam(gomock.Any(), gomock.Any(), gomock.Any()).Return(true)
	mocks.feedbackVoteDAO.EXPECT().Count(gomock.Any(), int64(1), int64(1), int64(1), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ int64, _ int64, _ int64, opts ...db.Option) (int64, int64, error) {
			assert.True(t, db.ContainWithMasterOpt(opts))
			return 3, 2, nil
		},
	)

	upVoteCount, downVoteCount, err := repo.CountFeedbackVote(context.Background(), 1, 1, 1)

	assert.NoError(t, err)
	assert.Equal(t, int64(3), upVoteCount)
	assert.Equal(t, int64(2), downVoteCount)
}

func TestExptInsightAnalysisRecordRepo_List(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo, mocks := newTestExptInsightAnalysisRecordRepo(ctrl)
	expectNoWriteFlagRead(mocks)

	mocks.feedbackCommentDAO.EXPECT().List(gomock.Any(), int64(1), int64(1), int64(1), entity.NewPage(1, 10)).Return([]*model.ExptInsightAnalysisFeedbackComment{
		{
			ID:               1,
			SpaceID:          1,
			ExptID:           1,
			AnalysisRecordID: ptr.Of(int64(1)),
			Comment:          ptr.Of("test comment"),
			CreatedBy:        "user123",
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		},
	}, int64(1), nil)

	comments, total, err := repo.List(context.Background(), 1, 1, 1, entity.NewPage(1, 10))

	assert.NoError(t, err)
	assert.Len(t, comments, 1)
	assert.Equal(t, int64(1), total)
}

// Test error cases for better coverage
func TestExptInsightAnalysisRecordRepo_CreateAnalysisRecord_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo, mocks := newTestExptInsightAnalysisRecordRepo(ctrl)

	record := &entity.ExptInsightAnalysisRecord{
		SpaceID:               1,
		ExptID:                1,
		Status:                entity.InsightAnalysisStatus_Running,
		AnalysisReportContent: "test content",
		CreatedBy:             "user123",
	}

	// Test ID generation error
	mocks.idGenerator.EXPECT().GenID(gomock.Any()).Return(int64(0), assert.AnError)

	id, err := repo.CreateAnalysisRecord(context.Background(), record)

	assert.Error(t, err)
	assert.Equal(t, int64(0), id)
}

func TestExptInsightAnalysisRecordRepo_CreateAnalysisRecord_DAOError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo, mocks := newTestExptInsightAnalysisRecordRepo(ctrl)

	record := &entity.ExptInsightAnalysisRecord{
		SpaceID:               1,
		ExptID:                1,
		Status:                entity.InsightAnalysisStatus_Running,
		AnalysisReportContent: "test content",
		CreatedBy:             "user123",
	}

	// Test DAO create error
	mocks.idGenerator.EXPECT().GenID(gomock.Any()).Return(int64(123), nil)
	mocks.analysisRecordDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(assert.AnError)

	id, err := repo.CreateAnalysisRecord(context.Background(), record)

	assert.Error(t, err)
	assert.Equal(t, int64(0), id)
}

func TestExptInsightAnalysisRecordRepo_CreateFeedbackComment_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo, mocks := newTestExptInsightAnalysisRecordRepo(ctrl)

	comment := &entity.ExptInsightAnalysisFeedbackComment{
		SpaceID:          1,
		ExptID:           1,
		AnalysisRecordID: 1,
		Comment:          "test comment",
		CreatedBy:        "user123",
	}

	// Test ID generation error
	mocks.idGenerator.EXPECT().GenID(gomock.Any()).Return(int64(0), assert.AnError)

	err := repo.CreateFeedbackComment(context.Background(), comment)

	assert.Error(t, err)
}

func TestExptInsightAnalysisRecordRepo_CreateFeedbackVote_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo, mocks := newTestExptInsightAnalysisRecordRepo(ctrl)

	vote := &entity.ExptInsightAnalysisFeedbackVote{
		SpaceID:          1,
		ExptID:           1,
		AnalysisRecordID: 1,
		VoteType:         entity.Upvote,
		CreatedBy:        "user123",
	}

	// Test ID generation error
	mocks.idGenerator.EXPECT().GenID(gomock.Any()).Return(int64(0), assert.AnError)

	err := repo.CreateFeedbackVote(context.Background(), vote)

	assert.Error(t, err)
}

func TestExptInsightAnalysisRecordRepo_GetFeedbackCommentByRecordID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo, mocks := newTestExptInsightAnalysisRecordRepo(ctrl)
	expectNoWriteFlagRead(mocks)

	mocks.feedbackCommentDAO.EXPECT().GetByRecordID(gomock.Any(), int64(1), int64(1), int64(1)).Return(&model.ExptInsightAnalysisFeedbackComment{
		ID:               1,
		SpaceID:          1,
		ExptID:           1,
		AnalysisRecordID: ptr.Of(int64(1)),
		Comment:          ptr.Of("test comment"),
		CreatedBy:        "user123",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}, nil)

	comment, err := repo.GetFeedbackCommentByRecordID(context.Background(), 1, 1, 1)

	assert.NoError(t, err)
	assert.NotNil(t, comment)
	assert.Equal(t, int64(1), comment.ID)
}

func TestExptInsightAnalysisRecordRepo_GetFeedbackCommentByRecordID_ForceMaster(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo, mocks := newTestExptInsightAnalysisRecordRepo(ctrl)

	mocks.writeTracker.EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), int64(1)).Return(true)
	mocks.writeTracker.EXPECT().CheckWriteFlagBySearchParam(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	mocks.feedbackCommentDAO.EXPECT().GetByRecordID(gomock.Any(), int64(1), int64(1), int64(1), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ int64, _ int64, _ int64, opts ...db.Option) (*model.ExptInsightAnalysisFeedbackComment, error) {
			assert.True(t, db.ContainWithMasterOpt(opts))
			return &model.ExptInsightAnalysisFeedbackComment{ID: 1, SpaceID: 1, ExptID: 1, AnalysisRecordID: ptr.Of(int64(1))}, nil
		},
	)

	comment, err := repo.GetFeedbackCommentByRecordID(context.Background(), 1, 1, 1)

	assert.NoError(t, err)
	assert.NotNil(t, comment)
	assert.Equal(t, int64(1), comment.ID)
}

func TestExptInsightAnalysisRecordRepo_GetFeedbackCommentByRecordID_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo, mocks := newTestExptInsightAnalysisRecordRepo(ctrl)
	expectNoWriteFlagRead(mocks)

	mocks.feedbackCommentDAO.EXPECT().GetByRecordID(gomock.Any(), int64(1), int64(1), int64(1)).Return(nil, assert.AnError)

	comment, err := repo.GetFeedbackCommentByRecordID(context.Background(), 1, 1, 1)

	assert.Error(t, err)
	assert.Nil(t, comment)
}

func TestExptInsightAnalysisRecordRepo_GetFeedbackVoteByUser_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo, mocks := newTestExptInsightAnalysisRecordRepo(ctrl)
	expectNoWriteFlagRead(mocks)

	mocks.feedbackVoteDAO.EXPECT().GetByUser(gomock.Any(), int64(1), int64(1), int64(1), "user123").Return(nil, assert.AnError)

	vote, err := repo.GetFeedbackVoteByUser(context.Background(), 1, 1, 1, "user123")

	assert.Error(t, err)
	assert.Nil(t, vote)
}

func TestExptInsightAnalysisRecordRepo_List_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo, mocks := newTestExptInsightAnalysisRecordRepo(ctrl)
	expectNoWriteFlagRead(mocks)

	mocks.feedbackCommentDAO.EXPECT().List(gomock.Any(), int64(1), int64(1), int64(1), entity.NewPage(1, 10)).Return(nil, int64(0), assert.AnError)

	comments, total, err := repo.List(context.Background(), 1, 1, 1, entity.NewPage(1, 10))

	assert.Error(t, err)
	assert.Nil(t, comments)
	assert.Equal(t, int64(0), total)
}

// New test for constructor NewExptInsightAnalysisRecordRepo
func TestNewExptInsightAnalysisRecordRepo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	analysisRecordDAO := mocks.NewMockIExptInsightAnalysisRecordDAO(ctrl)
	feedbackCommentDAO := mocks.NewMockIExptInsightAnalysisFeedbackCommentDAO(ctrl)
	feedbackVoteDAO := mocks.NewMockIExptInsightAnalysisFeedbackVoteDAO(ctrl)
	idGenerator := mockidgen.NewMockIIDGenerator(ctrl)
	writeTracker := platestwritemocks.NewMockILatestWriteTracker(ctrl)

	repoIface := NewExptInsightAnalysisRecordRepo(analysisRecordDAO, feedbackCommentDAO, feedbackVoteDAO, idGenerator, writeTracker)
	assert.NotNil(t, repoIface)

	impl, ok := repoIface.(*ExptInsightAnalysisRecordRepo)
	assert.True(t, ok)
	assert.Equal(t, analysisRecordDAO, impl.exptInsightAnalysisRecordDAO)
	assert.Equal(t, feedbackCommentDAO, impl.exptInsightAnalysisFeedbackCommentDAO)
	assert.Equal(t, feedbackVoteDAO, impl.exptInsightAnalysisFeedbackVoteDAO)
	assert.Equal(t, idGenerator, impl.idgenerator)
	assert.Equal(t, writeTracker, impl.writeTracker)
}
