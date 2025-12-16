// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	fileMocks "github.com/coze-dev/coze-loop/backend/infra/fileserver/mocks"
	rpcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	serviceMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func newTestInsightAnalysisService(ctrl *gomock.Controller) (*ExptInsightAnalysisServiceImpl, *testInsightAnalysisServiceMocks) {
	mockRepo := repoMocks.NewMockIExptInsightAnalysisRecordRepo(ctrl)
	mockPublisher := eventsMocks.NewMockExptEventPublisher(ctrl)
	mockFileClient := fileMocks.NewMockObjectStorage(ctrl)
	mockExptResultExportService := serviceMocks.NewMockIExptResultExportService(ctrl)
	mockExptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	mockAgentAdapter := rpcMocks.NewMockIAgentAdapter(ctrl)
	mockNotifyRPCAdapter := rpcMocks.NewMockINotifyRPCAdapter(ctrl)
	mockUserProvider := rpcMocks.NewMockIUserProvider(ctrl)
	mockTargetRepo := repoMocks.NewMockIEvalTargetRepo(ctrl)

	service := &ExptInsightAnalysisServiceImpl{
		repo:                    mockRepo,
		exptPublisher:           mockPublisher,
		fileClient:              mockFileClient,
		agentAdapter:            mockAgentAdapter,
		exptResultExportService: mockExptResultExportService,
		notifyRPCAdapter:        mockNotifyRPCAdapter,
		userProvider:            mockUserProvider,
		exptRepo:                mockExptRepo,
		targetRepo:              mockTargetRepo,
	}

	return service, &testInsightAnalysisServiceMocks{
		repo:                    mockRepo,
		publisher:               mockPublisher,
		fileClient:              mockFileClient,
		exptResultExportService: mockExptResultExportService,
		exptRepo:                mockExptRepo,
		agentAdapter:            mockAgentAdapter,
		notifyRPCAdapter:        mockNotifyRPCAdapter,
		userProvider:            mockUserProvider,
		targetRepo:              mockTargetRepo,
	}
}

type testInsightAnalysisServiceMocks struct {
	repo                    *repoMocks.MockIExptInsightAnalysisRecordRepo
	publisher               *eventsMocks.MockExptEventPublisher
	fileClient              *fileMocks.MockObjectStorage
	exptResultExportService *serviceMocks.MockIExptResultExportService
	exptRepo                *repoMocks.MockIExperimentRepo
	agentAdapter            *rpcMocks.MockIAgentAdapter
	notifyRPCAdapter        *rpcMocks.MockINotifyRPCAdapter
	userProvider            *rpcMocks.MockIUserProvider
	targetRepo              *repoMocks.MockIEvalTargetRepo
}

func TestNewInsightAnalysisService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repoMocks.NewMockIExptInsightAnalysisRecordRepo(ctrl)
	mockPublisher := eventsMocks.NewMockExptEventPublisher(ctrl)
	mockFileClient := fileMocks.NewMockObjectStorage(ctrl)
	mockAgentAdapter := rpcMocks.NewMockIAgentAdapter(ctrl)
	mockExptResultExportService := serviceMocks.NewMockIExptResultExportService(ctrl)
	mockNotifyRPCAdapter := rpcMocks.NewMockINotifyRPCAdapter(ctrl)
	mockUserProvider := rpcMocks.NewMockIUserProvider(ctrl)
	mockExptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	mockTargetRepo := repoMocks.NewMockIEvalTargetRepo(ctrl)

	service := NewInsightAnalysisService(
		mockRepo,
		mockPublisher,
		mockFileClient,
		mockAgentAdapter,
		mockExptResultExportService,
		mockNotifyRPCAdapter,
		mockUserProvider,
		mockExptRepo,
		mockTargetRepo,
	)

	impl, ok := service.(*ExptInsightAnalysisServiceImpl)
	assert.True(t, ok)
	assert.Equal(t, mockRepo, impl.repo)
	assert.Equal(t, mockPublisher, impl.exptPublisher)
	assert.Equal(t, mockFileClient, impl.fileClient)
	assert.Equal(t, mockAgentAdapter, impl.agentAdapter)
	assert.Equal(t, mockExptResultExportService, impl.exptResultExportService)
	assert.Equal(t, mockNotifyRPCAdapter, impl.notifyRPCAdapter)
	assert.Equal(t, mockUserProvider, impl.userProvider)
	assert.Equal(t, mockExptRepo, impl.exptRepo)
	assert.Equal(t, mockTargetRepo, impl.targetRepo)
}

// ... (原有其它测试保持不变)

// 并行子测试：GetAnalysisRecord 的多个分支
func TestExptInsightAnalysisServiceImpl_GetAnalysisRecord_Parallel(t *testing.T) {
	t.Parallel()
	// Running 且已超时：更新成功
	t.Run("RunningTimeout_UpdateSuccess", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		service, mocks := newTestInsightAnalysisService(ctrl)
		ctx := context.Background()
		mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), int64(1), int64(1), int64(1)).Return(&entity.ExptInsightAnalysisRecord{
			ID:        1,
			SpaceID:   1,
			ExptID:    1,
			Status:    entity.InsightAnalysisStatus_Running,
			CreatedAt: time.Now().Add(-entity.InsightAnalysisRunningTimeout - time.Second),
		}, nil)
		mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, rec *entity.ExptInsightAnalysisRecord, _ ...db.Option) error {
				assert.Equal(t, entity.InsightAnalysisStatus_Failed, rec.Status)
				return nil
			},
		)
		res, err := service.GetAnalysisRecordByID(ctx, 1, 1, 1, &entity.Session{UserID: "u"})
		assert.NoError(t, err)
		assert.Equal(t, entity.InsightAnalysisStatus_Failed, res.Status)
	})
	// Running 且已超时：更新失败
	t.Run("RunningTimeout_UpdateError", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		service, mocks := newTestInsightAnalysisService(ctrl)
		ctx := context.Background()
		mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), int64(1), int64(1), int64(1)).Return(&entity.ExptInsightAnalysisRecord{
			ID:        1,
			SpaceID:   1,
			ExptID:    1,
			Status:    entity.InsightAnalysisStatus_Running,
			CreatedAt: time.Now().Add(-entity.InsightAnalysisRunningTimeout - time.Second),
		}, nil)
		mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any()).Return(errors.New("update error"))
		_, err := service.GetAnalysisRecordByID(ctx, 1, 1, 1, &entity.Session{UserID: "u"})
		assert.Error(t, err)
	})
	// Running 未超时：直接返回
	t.Run("Running_NoTimeout_Return", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		service, mocks := newTestInsightAnalysisService(ctrl)
		ctx := context.Background()
		mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), int64(1), int64(1), int64(2)).Return(&entity.ExptInsightAnalysisRecord{
			ID:        2,
			SpaceID:   1,
			ExptID:    1,
			Status:    entity.InsightAnalysisStatus_Running,
			CreatedAt: time.Now(),
		}, nil)
		res, err := service.GetAnalysisRecordByID(ctx, 1, 1, 2, &entity.Session{UserID: "u"})
		assert.NoError(t, err)
		assert.Equal(t, entity.InsightAnalysisStatus_Running, res.Status)
	})
	// Failed：直接返回
	t.Run("Failed_Return", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		service, mocks := newTestInsightAnalysisService(ctrl)
		ctx := context.Background()
		mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), int64(1), int64(1), int64(3)).Return(&entity.ExptInsightAnalysisRecord{
			ID:      3,
			SpaceID: 1,
			ExptID:  1,
			Status:  entity.InsightAnalysisStatus_Failed,
		}, nil)
		res, err := service.GetAnalysisRecordByID(ctx, 1, 1, 3, &entity.Session{UserID: "u"})
		assert.NoError(t, err)
		assert.Equal(t, entity.InsightAnalysisStatus_Failed, res.Status)
	})
	// Success：拉取报告与反馈
	t.Run("Success_FullFlow", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		service, mocks := newTestInsightAnalysisService(ctrl)
		ctx := context.Background()
		mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), int64(1), int64(1), int64(4)).Return(&entity.ExptInsightAnalysisRecord{
			ID:               4,
			SpaceID:          1,
			ExptID:           1,
			Status:           entity.InsightAnalysisStatus_Success,
			AnalysisReportID: ptr.Of(int64(100)),
		}, nil)
		mocks.agentAdapter.EXPECT().GetReport(gomock.Any(), int64(1), int64(100)).Return("rep", []*entity.InsightAnalysisReportIndex{{ID: "k", Title: "t"}}, entity.ReportStatus_Success, nil)
		mocks.repo.EXPECT().CountFeedbackVote(gomock.Any(), int64(1), int64(1), int64(4)).Return(int64(10), int64(2), nil)
		mocks.repo.EXPECT().GetFeedbackVoteByUser(gomock.Any(), int64(1), int64(1), int64(4), "u").Return(&entity.ExptInsightAnalysisFeedbackVote{VoteType: entity.Upvote}, nil)
		res, err := service.GetAnalysisRecordByID(ctx, 1, 1, 4, &entity.Session{UserID: "u"})
		assert.NoError(t, err)
		assert.Equal(t, int64(10), res.ExptInsightAnalysisFeedback.UpvoteCount)
		assert.Equal(t, int64(2), res.ExptInsightAnalysisFeedback.DownvoteCount)
		assert.Equal(t, entity.Upvote, res.ExptInsightAnalysisFeedback.CurrentUserVoteType)
		assert.Equal(t, "rep", res.AnalysisReportContent)
		assert.Len(t, res.AnalysisReportIndex, 1)
	})
	// CountFeedbackVote error
	t.Run("Success_CountVoteError", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		service, mocks := newTestInsightAnalysisService(ctrl)
		ctx := context.Background()
		mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), int64(1), int64(1), int64(5)).Return(&entity.ExptInsightAnalysisRecord{
			ID:               5,
			SpaceID:          1,
			ExptID:           1,
			Status:           entity.InsightAnalysisStatus_Success,
			AnalysisReportID: ptr.Of(int64(100)),
		}, nil)
		mocks.agentAdapter.EXPECT().GetReport(gomock.Any(), int64(1), int64(100)).Return("rep", []*entity.InsightAnalysisReportIndex{}, entity.ReportStatus_Success, nil)
		mocks.repo.EXPECT().CountFeedbackVote(gomock.Any(), int64(1), int64(1), int64(5)).Return(int64(0), int64(0), errors.New("count err"))
		_, err := service.GetAnalysisRecordByID(ctx, 1, 1, 5, &entity.Session{UserID: "u"})
		assert.Error(t, err)
	})
	// GetFeedbackVoteByUser error
	t.Run("Success_GetUserVoteError", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		service, mocks := newTestInsightAnalysisService(ctrl)
		ctx := context.Background()
		mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), int64(1), int64(1), int64(6)).Return(&entity.ExptInsightAnalysisRecord{
			ID:               6,
			SpaceID:          1,
			ExptID:           1,
			Status:           entity.InsightAnalysisStatus_Success,
			AnalysisReportID: ptr.Of(int64(100)),
		}, nil)
		mocks.agentAdapter.EXPECT().GetReport(gomock.Any(), int64(1), int64(100)).Return("rep", []*entity.InsightAnalysisReportIndex{}, entity.ReportStatus_Success, nil)
		mocks.repo.EXPECT().CountFeedbackVote(gomock.Any(), int64(1), int64(1), int64(6)).Return(int64(1), int64(2), nil)
		mocks.repo.EXPECT().GetFeedbackVoteByUser(gomock.Any(), int64(1), int64(1), int64(6), "u").Return(nil, errors.New("get err"))
		_, err := service.GetAnalysisRecordByID(ctx, 1, 1, 6, &entity.Session{UserID: "u"})
		assert.Error(t, err)
	})
}

// 并行子测试：notifyAnalysisComplete 的多个分支
func TestExptInsightAnalysisServiceImpl_notifyAnalysisComplete_Parallel(t *testing.T) {
	t.Parallel()
	// 正常通知
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		service, mocks := newTestInsightAnalysisService(ctrl)
		ctx := context.Background()
		mocks.exptRepo.EXPECT().GetByID(gomock.Any(), int64(2), int64(1)).Return(&entity.Experiment{Name: "expt"}, nil)
		mocks.userProvider.EXPECT().MGetUserInfo(gomock.Any(), []string{"u"}).Return([]*entity.UserInfo{{Email: ptr.Of("u@c.com")}}, nil)
		mocks.notifyRPCAdapter.EXPECT().SendMessageCard(gomock.Any(), "u@c.com", gomock.Any(), gomock.Any()).Return(nil)
		assert.NoError(t, service.notifyAnalysisComplete(ctx, "u", 1, 2))
	})
	// GetByID 错误
	t.Run("GetByIDError", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		service, mocks := newTestInsightAnalysisService(ctrl)
		ctx := context.Background()
		mocks.exptRepo.EXPECT().GetByID(gomock.Any(), int64(2), int64(1)).Return(nil, errors.New("expt err"))
		assert.Error(t, service.notifyAnalysisComplete(ctx, "u", 1, 2))
	})
	// MGetUserInfo 错误
	t.Run("MGetUserInfoError", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		service, mocks := newTestInsightAnalysisService(ctrl)
		ctx := context.Background()
		mocks.exptRepo.EXPECT().GetByID(gomock.Any(), int64(2), int64(1)).Return(&entity.Experiment{Name: "expt"}, nil)
		mocks.userProvider.EXPECT().MGetUserInfo(gomock.Any(), []string{"u"}).Return(nil, errors.New("user err"))
		assert.Error(t, service.notifyAnalysisComplete(ctx, "u", 1, 2))
	})
	// 用户为空或长度不为1
	t.Run("UserEmptyOrNil", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		service, mocks := newTestInsightAnalysisService(ctrl)
		ctx := context.Background()
		mocks.exptRepo.EXPECT().GetByID(gomock.Any(), int64(2), int64(1)).Return(&entity.Experiment{Name: "expt"}, nil)
		mocks.userProvider.EXPECT().MGetUserInfo(gomock.Any(), []string{"u"}).Return([]*entity.UserInfo{}, nil)
		assert.NoError(t, service.notifyAnalysisComplete(ctx, "u", 1, 2))
	})
	// 用户指针为 nil
	t.Run("UserNil", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		service, mocks := newTestInsightAnalysisService(ctrl)
		ctx := context.Background()
		mocks.exptRepo.EXPECT().GetByID(gomock.Any(), int64(2), int64(1)).Return(&entity.Experiment{Name: "expt"}, nil)
		mocks.userProvider.EXPECT().MGetUserInfo(gomock.Any(), []string{"u"}).Return([]*entity.UserInfo{nil}, nil)
		assert.NoError(t, service.notifyAnalysisComplete(ctx, "u", 1, 2))
	})
}

// 并行子测试：ListAnalysisRecord 的多个分支
func TestExptInsightAnalysisServiceImpl_ListAnalysisRecord_Parallel(t *testing.T) {
	t.Parallel()
	t.Run("TotalZero_Return", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		service, mocks := newTestInsightAnalysisService(ctrl)
		ctx := context.Background()
		mocks.repo.EXPECT().ListAnalysisRecord(gomock.Any(), int64(1), int64(2), gomock.Any()).Return([]*entity.ExptInsightAnalysisRecord{}, int64(0), nil)
		recs, total, err := service.ListAnalysisRecord(ctx, 1, 2, entity.NewPage(1, 10), &entity.Session{UserID: "u"})
		assert.NoError(t, err)
		assert.Equal(t, int64(0), total)
		assert.Len(t, recs, 0)
	})
	// 正常路径：反馈计数与用户投票设置
	t.Run("Success_FeedbackFilled", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		service, mocks := newTestInsightAnalysisService(ctrl)
		ctx := context.Background()
		records := []*entity.ExptInsightAnalysisRecord{{ID: 11, SpaceID: 1, ExptID: 2}}
		mocks.repo.EXPECT().ListAnalysisRecord(gomock.Any(), int64(1), int64(2), gomock.Any()).Return(records, int64(2), nil)
		mocks.repo.EXPECT().CountFeedbackVote(gomock.Any(), int64(1), int64(2), int64(11)).Return(int64(5), int64(3), nil)
		mocks.repo.EXPECT().GetFeedbackVoteByUser(gomock.Any(), int64(1), int64(2), int64(11), "u").Return(&entity.ExptInsightAnalysisFeedbackVote{VoteType: entity.Downvote}, nil)
		recs, total, err := service.ListAnalysisRecord(ctx, 1, 2, entity.NewPage(1, 10), &entity.Session{UserID: "u"})
		assert.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Equal(t, int64(5), recs[0].ExptInsightAnalysisFeedback.UpvoteCount)
		assert.Equal(t, int64(3), recs[0].ExptInsightAnalysisFeedback.DownvoteCount)
		assert.Equal(t, entity.Downvote, recs[0].ExptInsightAnalysisFeedback.CurrentUserVoteType)
	})
	// 侧路错误：CountFeedbackVote 失败不阻塞主流程
	t.Run("SidePath_CountFeedbackVoteError", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		service, mocks := newTestInsightAnalysisService(ctrl)
		ctx := context.Background()
		records := []*entity.ExptInsightAnalysisRecord{{ID: 12, SpaceID: 1, ExptID: 2}}
		mocks.repo.EXPECT().ListAnalysisRecord(gomock.Any(), int64(1), int64(2), gomock.Any()).Return(records, int64(2), nil)
		mocks.repo.EXPECT().CountFeedbackVote(gomock.Any(), int64(1), int64(2), int64(12)).Return(int64(0), int64(0), errors.New("count err"))
		recs, total, err := service.ListAnalysisRecord(ctx, 1, 2, entity.NewPage(1, 10), &entity.Session{UserID: "u"})
		assert.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Equal(t, int64(12), recs[0].ID)
	})
	// 侧路错误：GetFeedbackVoteByUser 失败不阻塞主流程
	t.Run("SidePath_GetFeedbackVoteByUserError", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		service, mocks := newTestInsightAnalysisService(ctrl)
		ctx := context.Background()
		records := []*entity.ExptInsightAnalysisRecord{{ID: 13, SpaceID: 1, ExptID: 2}}
		mocks.repo.EXPECT().ListAnalysisRecord(gomock.Any(), int64(1), int64(2), gomock.Any()).Return(records, int64(2), nil)
		mocks.repo.EXPECT().CountFeedbackVote(gomock.Any(), int64(1), int64(2), int64(13)).Return(int64(1), int64(2), nil)
		mocks.repo.EXPECT().GetFeedbackVoteByUser(gomock.Any(), int64(1), int64(2), int64(13), "u").Return(nil, errors.New("get err"))
		recs, total, err := service.ListAnalysisRecord(ctx, 1, 2, entity.NewPage(1, 10), &entity.Session{UserID: "u"})
		assert.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Equal(t, int64(13), recs[0].ID)
	})
}

// 并行子测试：FeedbackExptInsightAnalysis 的多个分支
func TestExptInsightAnalysisServiceImpl_FeedbackExptInsightAnalysis_Parallel(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()
	spaceID, exptID, recordID := int64(1), int64(2), int64(3)
	// Upvote
	t.Run("Upvote", func(t *testing.T) {
		t.Parallel()
		param := &entity.ExptInsightAnalysisFeedbackParam{SpaceID: spaceID, ExptID: exptID, AnalysisRecordID: recordID, FeedbackActionType: entity.FeedbackActionType_Upvote, Session: &entity.Session{UserID: "u"}}
		mocks.repo.EXPECT().CreateFeedbackVote(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, v *entity.ExptInsightAnalysisFeedbackVote, _ ...db.Option) error {
			assert.Equal(t, entity.Upvote, v.VoteType)
			assert.Equal(t, "u", v.CreatedBy)
			return nil
		})
		assert.NoError(t, service.FeedbackExptInsightAnalysis(ctx, param))
	})
	// CancelUpvote
	t.Run("CancelUpvote", func(t *testing.T) {
		t.Parallel()
		param := &entity.ExptInsightAnalysisFeedbackParam{SpaceID: spaceID, ExptID: exptID, AnalysisRecordID: recordID, FeedbackActionType: entity.FeedbackActionType_CancelUpvote, Session: &entity.Session{UserID: "u"}}
		mocks.repo.EXPECT().UpdateFeedbackVote(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, v *entity.ExptInsightAnalysisFeedbackVote, _ ...db.Option) error {
			assert.Equal(t, entity.None, v.VoteType)
			return nil
		})
		assert.NoError(t, service.FeedbackExptInsightAnalysis(ctx, param))
	})
	// CancelDownvote
	t.Run("CancelDownvote", func(t *testing.T) {
		t.Parallel()
		param := &entity.ExptInsightAnalysisFeedbackParam{SpaceID: spaceID, ExptID: exptID, AnalysisRecordID: recordID, FeedbackActionType: entity.FeedbackActionType_CancelDownvote, Session: &entity.Session{UserID: "u"}}
		mocks.repo.EXPECT().UpdateFeedbackVote(gomock.Any(), gomock.Any()).Return(nil)
		assert.NoError(t, service.FeedbackExptInsightAnalysis(ctx, param))
	})
	// Downvote
	t.Run("Downvote", func(t *testing.T) {
		t.Parallel()
		param := &entity.ExptInsightAnalysisFeedbackParam{SpaceID: spaceID, ExptID: exptID, AnalysisRecordID: recordID, FeedbackActionType: entity.FeedbackActionType_Downvote, Session: &entity.Session{UserID: "u"}}
		mocks.repo.EXPECT().CreateFeedbackVote(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, v *entity.ExptInsightAnalysisFeedbackVote, _ ...db.Option) error {
			assert.Equal(t, entity.Downvote, v.VoteType)
			return nil
		})
		assert.NoError(t, service.FeedbackExptInsightAnalysis(ctx, param))
	})
	// CreateComment
	t.Run("CreateComment", func(t *testing.T) {
		t.Parallel()
		c := "hello"
		param := &entity.ExptInsightAnalysisFeedbackParam{SpaceID: spaceID, ExptID: exptID, AnalysisRecordID: recordID, FeedbackActionType: entity.FeedbackActionType_CreateComment, Comment: &c, Session: &entity.Session{UserID: "u"}}
		mocks.repo.EXPECT().CreateFeedbackComment(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, cmt *entity.ExptInsightAnalysisFeedbackComment, _ ...db.Option) error {
			assert.Equal(t, "hello", cmt.Comment)
			assert.Equal(t, "u", cmt.CreatedBy)
			return nil
		})
		assert.NoError(t, service.FeedbackExptInsightAnalysis(ctx, param))
	})
	// Update_Comment 参数缺失
	t.Run("UpdateComment_MissingID", func(t *testing.T) {
		t.Parallel()
		c := "hello"
		param := &entity.ExptInsightAnalysisFeedbackParam{SpaceID: spaceID, ExptID: exptID, AnalysisRecordID: recordID, FeedbackActionType: entity.FeedbackActionType_Update_Comment, Comment: &c, Session: &entity.Session{UserID: "u"}}
		assert.Error(t, service.FeedbackExptInsightAnalysis(ctx, param))
	})
	// Update_Comment 正常
	t.Run("UpdateComment_Success", func(t *testing.T) {
		t.Parallel()
		c := "hello"
		cid := int64(77)
		param := &entity.ExptInsightAnalysisFeedbackParam{SpaceID: spaceID, ExptID: exptID, AnalysisRecordID: recordID, FeedbackActionType: entity.FeedbackActionType_Update_Comment, Comment: &c, CommentID: &cid, Session: &entity.Session{UserID: "u"}}
		mocks.repo.EXPECT().UpdateFeedbackComment(gomock.Any(), gomock.Any()).Return(nil)
		assert.NoError(t, service.FeedbackExptInsightAnalysis(ctx, param))
	})
	// Delete_Comment 缺失ID
	t.Run("DeleteComment_MissingID", func(t *testing.T) {
		t.Parallel()
		param := &entity.ExptInsightAnalysisFeedbackParam{SpaceID: spaceID, ExptID: exptID, AnalysisRecordID: recordID, FeedbackActionType: entity.FeedbackActionType_Delete_Comment, Session: &entity.Session{UserID: "u"}}
		assert.Error(t, service.FeedbackExptInsightAnalysis(ctx, param))
	})
	// Delete_Comment 正常
	t.Run("DeleteComment_Success", func(t *testing.T) {
		t.Parallel()
		cid := int64(88)
		param := &entity.ExptInsightAnalysisFeedbackParam{SpaceID: spaceID, ExptID: exptID, AnalysisRecordID: recordID, FeedbackActionType: entity.FeedbackActionType_Delete_Comment, CommentID: &cid, Session: &entity.Session{UserID: "u"}}
		mocks.repo.EXPECT().DeleteFeedbackComment(gomock.Any(), int64(1), int64(2), int64(88)).Return(nil)
		assert.NoError(t, service.FeedbackExptInsightAnalysis(ctx, param))
	})
	// Session 为空
	t.Run("NilSession_Error", func(t *testing.T) {
		t.Parallel()
		param := &entity.ExptInsightAnalysisFeedbackParam{SpaceID: spaceID, ExptID: exptID, AnalysisRecordID: recordID, FeedbackActionType: entity.FeedbackActionType_Upvote}
		assert.Error(t, service.FeedbackExptInsightAnalysis(ctx, param))
	})
}

// 新增用例覆盖 GetAnalysisRecordByID 里的超时分支（insight_analysis_impl.go:242）
func TestExptInsightAnalysisServiceImpl_GetAnalysisRecord_StatusRunningTimeout_UpdateSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()

	// 返回 Running 且已超时的记录
	mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), int64(1), int64(1), int64(1)).Return(&entity.ExptInsightAnalysisRecord{
		ID:        1,
		SpaceID:   1,
		ExptID:    1,
		Status:    entity.InsightAnalysisStatus_Running,
		CreatedAt: time.Now().Add(-entity.InsightAnalysisRunningTimeout - time.Second),
	}, nil)
	// 期望 UpdateAnalysisRecord 被调用，并将状态置为 Failed
	mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, rec *entity.ExptInsightAnalysisRecord, _ ...db.Option) error {
			assert.Equal(t, entity.InsightAnalysisStatus_Failed, rec.Status)
			return nil
		},
	)

	res, err := service.GetAnalysisRecordByID(ctx, 1, 1, 1, &entity.Session{UserID: "user1"})
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, int64(1), res.ID)
	assert.Equal(t, entity.InsightAnalysisStatus_Failed, res.Status)
}

func TestExptInsightAnalysisServiceImpl_GetAnalysisRecord_StatusRunningTimeout_UpdateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()

	// 返回 Running 且已超时的记录
	mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), int64(1), int64(1), int64(1)).Return(&entity.ExptInsightAnalysisRecord{
		ID:        1,
		SpaceID:   1,
		ExptID:    1,
		Status:    entity.InsightAnalysisStatus_Running,
		CreatedAt: time.Now().Add(-entity.InsightAnalysisRunningTimeout - time.Second),
	}, nil)
	// UpdateAnalysisRecord 返回错误，GetAnalysisRecordByID 应返回该错误，同时记录状态已置为 Failed
	mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, rec *entity.ExptInsightAnalysisRecord, _ ...db.Option) error {
			assert.Equal(t, entity.InsightAnalysisStatus_Failed, rec.Status)
			return errors.New("update error")
		},
	)

	res, err := service.GetAnalysisRecordByID(ctx, 1, 1, 1, &entity.Session{UserID: "user1"})
	assert.Error(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, entity.InsightAnalysisStatus_Failed, res.Status)
}

// ------------------- CreateAnalysisRecord -------------------
func TestExptInsightAnalysisServiceImpl_CreateAnalysisRecord_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()
	record := &entity.ExptInsightAnalysisRecord{SpaceID: 1, ExptID: 2}

	mocks.repo.EXPECT().CreateAnalysisRecord(gomock.Any(), record).Return(int64(123), nil)
	mocks.publisher.EXPECT().PublishExptExportCSVEvent(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, ev *entity.ExportCSVEvent, d *time.Duration) error {
			assert.Equal(t, int64(123), ev.ExportID)
			assert.Equal(t, int64(2), ev.ExperimentID)
			assert.Equal(t, int64(1), ev.SpaceID)
			assert.Equal(t, entity.ExportSceneInsightAnalysis, ev.ExportScene)
			return nil
		},
	)

	id, err := service.CreateAnalysisRecord(ctx, record, &entity.Session{UserID: "user1"})
	assert.NoError(t, err)
	assert.Equal(t, int64(123), id)
}

func TestExptInsightAnalysisServiceImpl_CreateAnalysisRecord_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()
	record := &entity.ExptInsightAnalysisRecord{SpaceID: 1, ExptID: 2}

	mocks.repo.EXPECT().CreateAnalysisRecord(gomock.Any(), record).Return(int64(0), errors.New("create error"))

	id, err := service.CreateAnalysisRecord(ctx, record, &entity.Session{UserID: "user1"})
	assert.Error(t, err)
	assert.Equal(t, int64(0), id)
}

func TestExptInsightAnalysisServiceImpl_CreateAnalysisRecord_PublishError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()
	record := &entity.ExptInsightAnalysisRecord{SpaceID: 1, ExptID: 2}

	mocks.repo.EXPECT().CreateAnalysisRecord(gomock.Any(), record).Return(int64(123), nil)
	mocks.publisher.EXPECT().PublishExptExportCSVEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("publish error"))

	id, err := service.CreateAnalysisRecord(ctx, record, &entity.Session{UserID: "user1"})
	assert.Error(t, err)
	assert.Equal(t, int64(0), id)
}

// ------------------- GenAnalysisReport -------------------
func TestExptInsightAnalysisServiceImpl_GenAnalysisReport_AlreadyHasReport_CheckStatusSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()
	spaceID, exptID, recordID := int64(1), int64(2), int64(3)

	mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), spaceID, exptID, recordID).Return(&entity.ExptInsightAnalysisRecord{
		ID:               recordID,
		SpaceID:          spaceID,
		ExptID:           exptID,
		CreatedBy:        "user1",
		AnalysisReportID: ptr.Of(int64(100)),
		CreatedAt:        time.Now().Add(-time.Hour),
	}, nil)
	// GetReport -> Success
	mocks.agentAdapter.EXPECT().GetReport(gomock.Any(), spaceID, int64(100)).Return("report", []*entity.InsightAnalysisReportIndex{{ID: "1", Title: "t"}}, entity.ReportStatus_Success, nil)
	// notifyAnalysisComplete
	mocks.exptRepo.EXPECT().GetByID(gomock.Any(), exptID, spaceID).Return(&entity.Experiment{Name: "expt"}, nil)
	mocks.userProvider.EXPECT().MGetUserInfo(gomock.Any(), []string{"user1"}).Return([]*entity.UserInfo{{Email: ptr.Of("u@c.com")}}, nil)
	mocks.notifyRPCAdapter.EXPECT().SendMessageCard(gomock.Any(), "u@c.com", gomock.Any(), gomock.Any()).Return(nil)
	// UpdateAnalysisRecord -> Success
	mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, rec *entity.ExptInsightAnalysisRecord, _ ...db.Option) error {
			assert.Equal(t, entity.InsightAnalysisStatus_Success, rec.Status)
			return nil
		},
	)

	err := service.GenAnalysisReport(ctx, spaceID, exptID, recordID, time.Now().Unix())
	assert.NoError(t, err)
}

func TestExptInsightAnalysisServiceImpl_GenAnalysisReport_SuccessPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()
	spaceID, exptID, recordID := int64(1), int64(2), int64(3)
	createAt := time.Now().Unix()

	// no AnalysisReportID yet
	mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), spaceID, exptID, recordID).Return(&entity.ExptInsightAnalysisRecord{
		ID:        recordID,
		SpaceID:   spaceID,
		ExptID:    exptID,
		CreatedBy: "user1",
	}, nil)
	// export CSV
	fileName := "insight_analysis_1_3.csv"
	mocks.exptResultExportService.EXPECT().DoExportCSV(gomock.Any(), spaceID, exptID, fileName, true).Return(nil)
	// sign download
	mocks.fileClient.EXPECT().SignDownloadReq(gomock.Any(), fileName, gomock.Any()).Return("http://example.com/f", nil, nil)
	// expt info
	now := time.Now()
	end := now.Add(time.Hour)
	mocks.exptRepo.EXPECT().GetByID(gomock.Any(), exptID, spaceID).Return(&entity.Experiment{
		StartAt:         &now,
		EndAt:           &end,
		TargetType:      entity.EvalTargetTypeLoopPrompt,
		TargetID:        10,
		TargetVersionID: 20,
	}, nil)
	// target version
	mocks.targetRepo.EXPECT().GetEvalTargetVersion(gomock.Any(), spaceID, int64(20)).Return(&entity.EvalTarget{
		SourceTargetID:    "123",
		EvalTargetVersion: &entity.EvalTargetVersion{SourceTargetVersion: "1.2.3"},
	}, nil)
	// evaluators
	mocks.exptRepo.EXPECT().GetEvaluatorRefByExptIDs(gomock.Any(), []int64{exptID}, spaceID).Return([]*entity.ExptEvaluatorRef{{EvaluatorID: 1, EvaluatorVersionID: 2}}, nil)
	// call agent
	mocks.agentAdapter.EXPECT().CallTraceAgent(gomock.Any(), gomock.Any()).Return(int64(999), nil)
	// publish re-check event
	mocks.publisher.EXPECT().PublishExptExportCSVEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	// update record in defer
	mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, rec *entity.ExptInsightAnalysisRecord, _ ...db.Option) error {
			assert.Equal(t, recordID, rec.ID)
			assert.Equal(t, spaceID, rec.SpaceID)
			assert.Equal(t, exptID, rec.ExptID)
			assert.NotNil(t, rec.ExptResultFilePath)
			assert.Equal(t, fileName, *rec.ExptResultFilePath)
			assert.Equal(t, entity.InsightAnalysisStatus_Running, rec.Status)
			assert.NotNil(t, rec.AnalysisReportID)
			assert.Equal(t, int64(999), *rec.AnalysisReportID)
			return nil
		},
	)

	err := service.GenAnalysisReport(ctx, spaceID, exptID, recordID, createAt)
	assert.NoError(t, err)
}

func TestExptInsightAnalysisServiceImpl_GetAnalysisRecordFeedbackVoteByUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()

	tests := []struct {
		name    string
		setup   func()
		session *entity.Session
		want    *entity.ExptInsightAnalysisFeedbackVote
		wantErr bool
	}{
		{
			name:    "nil session returns nil",
			session: nil,
		},
		{
			name:    "empty user returns nil",
			session: &entity.Session{UserID: ""},
		},
		{
			name:    "no vote",
			session: &entity.Session{UserID: "user1"},
			setup: func() {
				mocks.repo.EXPECT().GetFeedbackVoteByUser(gomock.Any(), int64(1), int64(2), int64(3), "user1").Return(nil, nil)
			},
		},
		{
			name:    "success",
			session: &entity.Session{UserID: "user1"},
			setup: func() {
				mocks.repo.EXPECT().GetFeedbackVoteByUser(gomock.Any(), int64(1), int64(2), int64(3), "user1").Return(&entity.ExptInsightAnalysisFeedbackVote{VoteType: entity.Upvote}, nil)
			},
			want: &entity.ExptInsightAnalysisFeedbackVote{VoteType: entity.Upvote},
		},
		{
			name:    "repo error",
			session: &entity.Session{UserID: "user1"},
			setup: func() {
				mocks.repo.EXPECT().GetFeedbackVoteByUser(gomock.Any(), int64(1), int64(2), int64(3), "user1").Return(nil, errors.New("repo error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup()
			}
			got, err := service.GetAnalysisRecordFeedbackVoteByUser(ctx, 1, 2, 3, tc.session)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tc.want == nil {
				assert.Nil(t, got)
			} else {
				if assert.NotNil(t, got) {
					assert.Equal(t, tc.want.VoteType, got.VoteType)
				}
			}
		})
	}
}

func TestExptInsightAnalysisServiceImpl_GenAnalysisReport_DoExportCSVError_FailedAndReturn(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()
	spaceID, exptID, recordID := int64(1), int64(2), int64(3)

	mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), spaceID, exptID, recordID).Return(&entity.ExptInsightAnalysisRecord{ID: recordID, SpaceID: spaceID, ExptID: exptID}, nil)
	fileName := "insight_analysis_1_3.csv"
	mocks.exptResultExportService.EXPECT().DoExportCSV(gomock.Any(), spaceID, exptID, fileName, true).Return(errors.New("export error"))
	// should update as Failed in defer
	mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, rec *entity.ExptInsightAnalysisRecord, _ ...db.Option) error {
			assert.Equal(t, entity.InsightAnalysisStatus_Failed, rec.Status)
			assert.NotNil(t, rec.ExptResultFilePath)
			assert.Equal(t, fileName, *rec.ExptResultFilePath)
			return nil
		},
	)

	err := service.GenAnalysisReport(ctx, spaceID, exptID, recordID, time.Now().Unix())
	assert.Error(t, err)
}

func TestExptInsightAnalysisServiceImpl_GenAnalysisReport_UpdateRecordError_ReturnsUpdateErr(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()
	spaceID, exptID, recordID := int64(1), int64(2), int64(3)

	mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), spaceID, exptID, recordID).Return(&entity.ExptInsightAnalysisRecord{ID: recordID, SpaceID: spaceID, ExptID: exptID}, nil)
	fileName := "insight_analysis_1_3.csv"
	mocks.exptResultExportService.EXPECT().DoExportCSV(gomock.Any(), spaceID, exptID, fileName, true).Return(nil)
	mocks.fileClient.EXPECT().SignDownloadReq(gomock.Any(), fileName, gomock.Any()).Return("http://example.com/f", nil, nil)
	now := time.Now()
	end := now.Add(time.Hour)
	mocks.exptRepo.EXPECT().GetByID(gomock.Any(), exptID, spaceID).Return(&entity.Experiment{StartAt: &now, EndAt: &end, TargetType: entity.EvalTargetTypeLoopPrompt, TargetID: 10, TargetVersionID: 20}, nil)
	mocks.targetRepo.EXPECT().GetEvalTargetVersion(gomock.Any(), spaceID, int64(20)).Return(&entity.EvalTarget{SourceTargetID: "123", EvalTargetVersion: &entity.EvalTargetVersion{SourceTargetVersion: "1.2.3"}}, nil)
	mocks.exptRepo.EXPECT().GetEvaluatorRefByExptIDs(gomock.Any(), []int64{exptID}, spaceID).Return([]*entity.ExptEvaluatorRef{{EvaluatorID: 1, EvaluatorVersionID: 2}}, nil)
	mocks.agentAdapter.EXPECT().CallTraceAgent(gomock.Any(), gomock.Any()).Return(int64(999), nil)
	mocks.publisher.EXPECT().PublishExptExportCSVEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any()).Return(errors.New("update fail"))

	err := service.GenAnalysisReport(ctx, spaceID, exptID, recordID, time.Now().Unix())
	assert.Error(t, err)
}

// ------------------- checkAnalysisReportGenStatus -------------------
func TestExptInsightAnalysisServiceImpl_checkAnalysisReportGenStatus_StatusFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()
	record := &entity.ExptInsightAnalysisRecord{ID: 1, SpaceID: 1, ExptID: 2, AnalysisReportID: ptr.Of(int64(100))}

	mocks.agentAdapter.EXPECT().GetReport(gomock.Any(), int64(1), int64(100)).Return("", nil, entity.ReportStatus_Failed, nil)
	mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, rec *entity.ExptInsightAnalysisRecord, _ ...db.Option) error {
			assert.Equal(t, entity.InsightAnalysisStatus_Failed, rec.Status)
			return nil
		},
	)

	err := service.checkAnalysisReportGenStatus(ctx, record, time.Now().Unix())
	assert.NoError(t, err)
}

func TestExptInsightAnalysisServiceImpl_checkAnalysisReportGenStatus_StatusSuccess_Notify(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()
	record := &entity.ExptInsightAnalysisRecord{ID: 1, SpaceID: 1, ExptID: 2, AnalysisReportID: ptr.Of(int64(100)), CreatedBy: "user1"}

	mocks.agentAdapter.EXPECT().GetReport(gomock.Any(), int64(1), int64(100)).Return("content", []*entity.InsightAnalysisReportIndex{{ID: "1", Title: "t"}}, entity.ReportStatus_Success, nil)
	mocks.exptRepo.EXPECT().GetByID(gomock.Any(), int64(2), int64(1)).Return(&entity.Experiment{Name: "expt"}, nil)
	mocks.userProvider.EXPECT().MGetUserInfo(gomock.Any(), []string{"user1"}).Return([]*entity.UserInfo{{Email: ptr.Of("u@c.com")}}, nil)
	mocks.notifyRPCAdapter.EXPECT().SendMessageCard(gomock.Any(), "u@c.com", gomock.Any(), gomock.Any()).Return(nil)
	mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, rec *entity.ExptInsightAnalysisRecord, _ ...db.Option) error {
			assert.Equal(t, entity.InsightAnalysisStatus_Success, rec.Status)
			return nil
		},
	)

	err := service.checkAnalysisReportGenStatus(ctx, record, time.Now().Unix())
	assert.NoError(t, err)
}

func TestExptInsightAnalysisServiceImpl_checkAnalysisReportGenStatus_StatusRunningTimeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()
	old := time.Now().Add(-entity.InsightAnalysisRunningTimeout - time.Second)
	record := &entity.ExptInsightAnalysisRecord{ID: 1, SpaceID: 1, ExptID: 2, CreatedAt: old, AnalysisReportID: ptr.Of(int64(100))}

	mocks.agentAdapter.EXPECT().GetReport(gomock.Any(), int64(1), int64(100)).Return("", nil, entity.ReportStatus_Running, nil)
	mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, rec *entity.ExptInsightAnalysisRecord, _ ...db.Option) error {
			assert.Equal(t, entity.InsightAnalysisStatus_Failed, rec.Status)
			return nil
		},
	)

	err := service.checkAnalysisReportGenStatus(ctx, record, time.Now().Unix())
	assert.NoError(t, err)
}

func TestExptInsightAnalysisServiceImpl_checkAnalysisReportGenStatus_StatusRunningRequeue(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()
	record := &entity.ExptInsightAnalysisRecord{ID: 1, SpaceID: 1, ExptID: 2, CreatedAt: time.Now(), AnalysisReportID: ptr.Of(int64(100))}

	mocks.agentAdapter.EXPECT().GetReport(gomock.Any(), int64(1), int64(100)).Return("", nil, entity.ReportStatus_Running, nil)
	mocks.publisher.EXPECT().PublishExptExportCSVEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	err := service.checkAnalysisReportGenStatus(ctx, record, time.Now().Unix())
	assert.NoError(t, err)
}

// ------------------- GenAnalysisReport 错误分支补充 -------------------
func TestExptInsightAnalysisServiceImpl_GenAnalysisReport_GetAnalysisRecordError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()
	spaceID, exptID, recordID := int64(1), int64(2), int64(3)

	mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), spaceID, exptID, recordID).Return(nil, errors.New("get record err"))

	err := service.GenAnalysisReport(ctx, spaceID, exptID, recordID, time.Now().Unix())
	assert.Error(t, err)
}

func TestExptInsightAnalysisServiceImpl_GenAnalysisReport_SignDownloadReqError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()
	spaceID, exptID, recordID := int64(1), int64(2), int64(3)
	fileName := "insight_analysis_1_3.csv"

	mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), spaceID, exptID, recordID).Return(&entity.ExptInsightAnalysisRecord{ID: recordID, SpaceID: spaceID, ExptID: exptID}, nil)
	mocks.exptResultExportService.EXPECT().DoExportCSV(gomock.Any(), spaceID, exptID, fileName, true).Return(nil)
	mocks.fileClient.EXPECT().SignDownloadReq(gomock.Any(), fileName, gomock.Any()).Return("", nil, errors.New("sign err"))
	mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, rec *entity.ExptInsightAnalysisRecord, _ ...db.Option) error {
			assert.Equal(t, entity.InsightAnalysisStatus_Failed, rec.Status)
			assert.NotNil(t, rec.ExptResultFilePath)
			assert.Equal(t, fileName, *rec.ExptResultFilePath)
			return nil
		},
	)

	err := service.GenAnalysisReport(ctx, spaceID, exptID, recordID, time.Now().Unix())
	assert.Error(t, err)
}

func TestExptInsightAnalysisServiceImpl_GenAnalysisReport_GetExptByIDError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()
	spaceID, exptID, recordID := int64(1), int64(2), int64(3)
	fileName := "insight_analysis_1_3.csv"

	mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), spaceID, exptID, recordID).Return(&entity.ExptInsightAnalysisRecord{ID: recordID, SpaceID: spaceID, ExptID: exptID}, nil)
	mocks.exptResultExportService.EXPECT().DoExportCSV(gomock.Any(), spaceID, exptID, fileName, true).Return(nil)
	mocks.fileClient.EXPECT().SignDownloadReq(gomock.Any(), fileName, gomock.Any()).Return("http://example.com/f", nil, nil)
	mocks.exptRepo.EXPECT().GetByID(gomock.Any(), exptID, spaceID).Return(nil, errors.New("expt err"))
	mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, rec *entity.ExptInsightAnalysisRecord, _ ...db.Option) error {
			assert.Equal(t, entity.InsightAnalysisStatus_Failed, rec.Status)
			assert.NotNil(t, rec.ExptResultFilePath)
			assert.Equal(t, fileName, *rec.ExptResultFilePath)
			return nil
		},
	)

	err := service.GenAnalysisReport(ctx, spaceID, exptID, recordID, time.Now().Unix())
	assert.Error(t, err)
}

func TestExptInsightAnalysisServiceImpl_GenAnalysisReport_GetEvalTargetVersionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()
	spaceID, exptID, recordID := int64(1), int64(2), int64(3)
	fileName := "insight_analysis_1_3.csv"
	url := "http://example.com/f"

	mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), spaceID, exptID, recordID).Return(&entity.ExptInsightAnalysisRecord{ID: recordID, SpaceID: spaceID, ExptID: exptID}, nil)
	mocks.exptResultExportService.EXPECT().DoExportCSV(gomock.Any(), spaceID, exptID, fileName, true).Return(nil)
	mocks.fileClient.EXPECT().SignDownloadReq(gomock.Any(), fileName, gomock.Any()).Return(url, nil, nil)
	now := time.Now()
	end := now.Add(time.Hour)
	mocks.exptRepo.EXPECT().GetByID(gomock.Any(), exptID, spaceID).Return(&entity.Experiment{StartAt: &now, EndAt: &end, TargetType: entity.EvalTargetTypeLoopPrompt, TargetID: 10, TargetVersionID: 20}, nil)
	mocks.targetRepo.EXPECT().GetEvalTargetVersion(gomock.Any(), spaceID, int64(20)).Return(nil, errors.New("target err"))
	mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, rec *entity.ExptInsightAnalysisRecord, _ ...db.Option) error {
			assert.Equal(t, entity.InsightAnalysisStatus_Failed, rec.Status)
			assert.NotNil(t, rec.ExptResultFilePath)
			assert.Equal(t, fileName, *rec.ExptResultFilePath)
			return nil
		},
	)

	err := service.GenAnalysisReport(ctx, spaceID, exptID, recordID, time.Now().Unix())
	assert.Error(t, err)
}

func TestExptInsightAnalysisServiceImpl_GenAnalysisReport_TargetMissingSourceID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()
	spaceID, exptID, recordID := int64(1), int64(2), int64(3)
	fileName := "insight_analysis_1_3.csv"
	url := "http://example.com/f"

	mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), spaceID, exptID, recordID).Return(&entity.ExptInsightAnalysisRecord{ID: recordID, SpaceID: spaceID, ExptID: exptID}, nil)
	mocks.exptResultExportService.EXPECT().DoExportCSV(gomock.Any(), spaceID, exptID, fileName, true).Return(nil)
	mocks.fileClient.EXPECT().SignDownloadReq(gomock.Any(), fileName, gomock.Any()).Return(url, nil, nil)
	now := time.Now()
	end := now.Add(time.Hour)
	mocks.exptRepo.EXPECT().GetByID(gomock.Any(), exptID, spaceID).Return(&entity.Experiment{StartAt: &now, EndAt: &end, TargetType: entity.EvalTargetTypeLoopPrompt, TargetID: 10, TargetVersionID: 20}, nil)
	mocks.targetRepo.EXPECT().GetEvalTargetVersion(gomock.Any(), spaceID, int64(20)).Return(&entity.EvalTarget{SourceTargetID: "", EvalTargetVersion: &entity.EvalTargetVersion{SourceTargetVersion: "1.0.0"}}, nil)
	mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, rec *entity.ExptInsightAnalysisRecord, _ ...db.Option) error {
			assert.Equal(t, entity.InsightAnalysisStatus_Failed, rec.Status)
			assert.NotNil(t, rec.ExptResultFilePath)
			assert.Equal(t, fileName, *rec.ExptResultFilePath)
			return nil
		},
	)

	err := service.GenAnalysisReport(ctx, spaceID, exptID, recordID, time.Now().Unix())
	assert.Error(t, err)
}

func TestExptInsightAnalysisServiceImpl_GenAnalysisReport_TargetSourceIDParseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()
	spaceID, exptID, recordID := int64(1), int64(2), int64(3)
	fileName := "insight_analysis_1_3.csv"
	url := "http://example.com/f"

	mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), spaceID, exptID, recordID).Return(&entity.ExptInsightAnalysisRecord{ID: recordID, SpaceID: spaceID, ExptID: exptID}, nil)
	mocks.exptResultExportService.EXPECT().DoExportCSV(gomock.Any(), spaceID, exptID, fileName, true).Return(nil)
	mocks.fileClient.EXPECT().SignDownloadReq(gomock.Any(), fileName, gomock.Any()).Return(url, nil, nil)
	now := time.Now()
	end := now.Add(time.Hour)
	mocks.exptRepo.EXPECT().GetByID(gomock.Any(), exptID, spaceID).Return(&entity.Experiment{StartAt: &now, EndAt: &end, TargetType: entity.EvalTargetTypeLoopPrompt, TargetID: 10, TargetVersionID: 20}, nil)
	mocks.targetRepo.EXPECT().GetEvalTargetVersion(gomock.Any(), spaceID, int64(20)).Return(&entity.EvalTarget{SourceTargetID: "abc", EvalTargetVersion: &entity.EvalTargetVersion{SourceTargetVersion: "1.0.0"}}, nil)
	mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, rec *entity.ExptInsightAnalysisRecord, _ ...db.Option) error {
			assert.Equal(t, entity.InsightAnalysisStatus_Failed, rec.Status)
			assert.NotNil(t, rec.ExptResultFilePath)
			assert.Equal(t, fileName, *rec.ExptResultFilePath)
			return nil
		},
	)

	err := service.GenAnalysisReport(ctx, spaceID, exptID, recordID, time.Now().Unix())
	assert.Error(t, err)
}

func TestExptInsightAnalysisServiceImpl_GenAnalysisReport_TargetVersionMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()
	spaceID, exptID, recordID := int64(1), int64(2), int64(3)
	fileName := "insight_analysis_1_3.csv"
	url := "http://example.com/f"

	mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), spaceID, exptID, recordID).Return(&entity.ExptInsightAnalysisRecord{ID: recordID, SpaceID: spaceID, ExptID: exptID}, nil)
	mocks.exptResultExportService.EXPECT().DoExportCSV(gomock.Any(), spaceID, exptID, fileName, true).Return(nil)
	mocks.fileClient.EXPECT().SignDownloadReq(gomock.Any(), fileName, gomock.Any()).Return(url, nil, nil)
	now := time.Now()
	end := now.Add(time.Hour)
	mocks.exptRepo.EXPECT().GetByID(gomock.Any(), exptID, spaceID).Return(&entity.Experiment{StartAt: &now, EndAt: &end, TargetType: entity.EvalTargetTypeLoopPrompt, TargetID: 10, TargetVersionID: 20}, nil)
	mocks.targetRepo.EXPECT().GetEvalTargetVersion(gomock.Any(), spaceID, int64(20)).Return(&entity.EvalTarget{SourceTargetID: "123", EvalTargetVersion: nil}, nil)
	mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, rec *entity.ExptInsightAnalysisRecord, _ ...db.Option) error {
			assert.Equal(t, entity.InsightAnalysisStatus_Failed, rec.Status)
			assert.NotNil(t, rec.ExptResultFilePath)
			assert.Equal(t, fileName, *rec.ExptResultFilePath)
			return nil
		},
	)

	err := service.GenAnalysisReport(ctx, spaceID, exptID, recordID, time.Now().Unix())
	assert.Error(t, err)
}

func TestExptInsightAnalysisServiceImpl_GenAnalysisReport_GetEvaluatorsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()
	spaceID, exptID, recordID := int64(1), int64(2), int64(3)
	fileName := "insight_analysis_1_3.csv"
	url := "http://example.com/f"

	mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), spaceID, exptID, recordID).Return(&entity.ExptInsightAnalysisRecord{ID: recordID, SpaceID: spaceID, ExptID: exptID}, nil)
	mocks.exptResultExportService.EXPECT().DoExportCSV(gomock.Any(), spaceID, exptID, fileName, true).Return(nil)
	mocks.fileClient.EXPECT().SignDownloadReq(gomock.Any(), fileName, gomock.Any()).Return(url, nil, nil)
	now := time.Now()
	end := now.Add(time.Hour)
	mocks.exptRepo.EXPECT().GetByID(gomock.Any(), exptID, spaceID).Return(&entity.Experiment{StartAt: &now, EndAt: &end, TargetType: entity.EvalTargetTypeLoopPrompt, TargetID: 10, TargetVersionID: 20}, nil)
	mocks.targetRepo.EXPECT().GetEvalTargetVersion(gomock.Any(), spaceID, int64(20)).Return(&entity.EvalTarget{SourceTargetID: "123", EvalTargetVersion: &entity.EvalTargetVersion{SourceTargetVersion: "1.0.0"}}, nil)
	mocks.exptRepo.EXPECT().GetEvaluatorRefByExptIDs(gomock.Any(), []int64{exptID}, spaceID).Return(nil, errors.New("eval refs err"))
	mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, rec *entity.ExptInsightAnalysisRecord, _ ...db.Option) error {
			assert.Equal(t, entity.InsightAnalysisStatus_Failed, rec.Status)
			assert.NotNil(t, rec.ExptResultFilePath)
			assert.Equal(t, fileName, *rec.ExptResultFilePath)
			return nil
		},
	)

	err := service.GenAnalysisReport(ctx, spaceID, exptID, recordID, time.Now().Unix())
	assert.Error(t, err)
}

func TestExptInsightAnalysisServiceImpl_GenAnalysisReport_CallTraceAgentError_UpdateFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()
	spaceID, exptID, recordID := int64(1), int64(2), int64(3)
	fileName := "insight_analysis_1_3.csv"
	url := "http://example.com/f"

	mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), spaceID, exptID, recordID).Return(&entity.ExptInsightAnalysisRecord{ID: recordID, SpaceID: spaceID, ExptID: exptID}, nil)
	mocks.exptResultExportService.EXPECT().DoExportCSV(gomock.Any(), spaceID, exptID, fileName, true).Return(nil)
	mocks.fileClient.EXPECT().SignDownloadReq(gomock.Any(), fileName, gomock.Any()).Return(url, nil, nil)
	now := time.Now()
	end := now.Add(time.Hour)
	mocks.exptRepo.EXPECT().GetByID(gomock.Any(), exptID, spaceID).Return(&entity.Experiment{StartAt: &now, EndAt: &end, TargetType: entity.EvalTargetTypeLoopPrompt, TargetID: 10, TargetVersionID: 20}, nil)
	mocks.targetRepo.EXPECT().GetEvalTargetVersion(gomock.Any(), spaceID, int64(20)).Return(&entity.EvalTarget{SourceTargetID: "123", EvalTargetVersion: &entity.EvalTargetVersion{SourceTargetVersion: "1.0.0"}}, nil)
	mocks.exptRepo.EXPECT().GetEvaluatorRefByExptIDs(gomock.Any(), []int64{exptID}, spaceID).Return([]*entity.ExptEvaluatorRef{{EvaluatorID: 1, EvaluatorVersionID: 2}}, nil)
	mocks.agentAdapter.EXPECT().CallTraceAgent(gomock.Any(), gomock.Any()).Return(int64(0), errors.New("agent err"))
	mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, rec *entity.ExptInsightAnalysisRecord, _ ...db.Option) error {
			assert.Equal(t, entity.InsightAnalysisStatus_Failed, rec.Status)
			assert.NotNil(t, rec.ExptResultFilePath)
			assert.Equal(t, fileName, *rec.ExptResultFilePath)
			assert.Nil(t, rec.AnalysisReportID)
			return nil
		},
	)

	err := service.GenAnalysisReport(ctx, spaceID, exptID, recordID, time.Now().Unix())
	assert.Error(t, err)
}

func TestExptInsightAnalysisServiceImpl_GenAnalysisReport_PublishEventError_UpdateFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()
	spaceID, exptID, recordID := int64(1), int64(2), int64(3)
	fileName := "insight_analysis_1_3.csv"
	url := "http://example.com/f"

	mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), spaceID, exptID, recordID).Return(&entity.ExptInsightAnalysisRecord{ID: recordID, SpaceID: spaceID, ExptID: exptID}, nil)
	mocks.exptResultExportService.EXPECT().DoExportCSV(gomock.Any(), spaceID, exptID, fileName, true).Return(nil)
	mocks.fileClient.EXPECT().SignDownloadReq(gomock.Any(), fileName, gomock.Any()).Return(url, nil, nil)
	now := time.Now()
	end := now.Add(time.Hour)
	mocks.exptRepo.EXPECT().GetByID(gomock.Any(), exptID, spaceID).Return(&entity.Experiment{StartAt: &now, EndAt: &end, TargetType: entity.EvalTargetTypeLoopPrompt, TargetID: 10, TargetVersionID: 20}, nil)
	mocks.targetRepo.EXPECT().GetEvalTargetVersion(gomock.Any(), spaceID, int64(20)).Return(&entity.EvalTarget{SourceTargetID: "123", EvalTargetVersion: &entity.EvalTargetVersion{SourceTargetVersion: "1.0.0"}}, nil)
	mocks.exptRepo.EXPECT().GetEvaluatorRefByExptIDs(gomock.Any(), []int64{exptID}, spaceID).Return([]*entity.ExptEvaluatorRef{{EvaluatorID: 1, EvaluatorVersionID: 2}}, nil)
	mocks.agentAdapter.EXPECT().CallTraceAgent(gomock.Any(), gomock.Any()).Return(int64(888), nil)
	mocks.publisher.EXPECT().PublishExptExportCSVEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("publish err"))
	mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, rec *entity.ExptInsightAnalysisRecord, _ ...db.Option) error {
			assert.Equal(t, entity.InsightAnalysisStatus_Failed, rec.Status)
			assert.NotNil(t, rec.ExptResultFilePath)
			assert.Equal(t, fileName, *rec.ExptResultFilePath)
			assert.NotNil(t, rec.AnalysisReportID)
			assert.Equal(t, int64(888), *rec.AnalysisReportID)
			return nil
		},
	)

	err := service.GenAnalysisReport(ctx, spaceID, exptID, recordID, time.Now().Unix())
	assert.Error(t, err)
}
