// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

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

	service := &ExptInsightAnalysisServiceImpl{
		repo:                    mockRepo,
		exptPublisher:           mockPublisher,
		fileClient:              mockFileClient,
		agentAdapter:            mockAgentAdapter,
		exptResultExportService: mockExptResultExportService,
		notifyRPCAdapter:        mockNotifyRPCAdapter,
		userProvider:            mockUserProvider,
		exptRepo:                mockExptRepo,
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
}

func TestExptInsightAnalysisServiceImpl_CreateAnalysisRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()

	tests := []struct {
		name    string
		setup   func()
		record  *entity.ExptInsightAnalysisRecord
		session *entity.Session
		wantErr bool
		wantID  int64
	}{
		{
			name: "success",
			setup: func() {
				mocks.repo.EXPECT().CreateAnalysisRecord(gomock.Any(), gomock.Any()).Return(int64(1), nil)
				mocks.publisher.EXPECT().PublishExptExportCSVEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			record: &entity.ExptInsightAnalysisRecord{
				SpaceID:   1,
				ExptID:    1,
				CreatedBy: "user1",
				Status:    entity.InsightAnalysisStatus_Unknown,
			},
			session: &entity.Session{UserID: "user1"},
			wantErr: false,
			wantID:  1,
		},
		{
			name: "repo error",
			setup: func() {
				mocks.repo.EXPECT().CreateAnalysisRecord(gomock.Any(), gomock.Any()).Return(int64(0), errors.New("repo error"))
			},
			record: &entity.ExptInsightAnalysisRecord{
				SpaceID:   1,
				ExptID:    1,
				CreatedBy: "user1",
				Status:    entity.InsightAnalysisStatus_Unknown,
			},
			session: &entity.Session{UserID: "user1"},
			wantErr: true,
			wantID:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			result, err := service.CreateAnalysisRecord(ctx, tt.record, tt.session)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantID, result)
			}
		})
	}
}

func TestExptInsightAnalysisServiceImpl_GenAnalysisReport(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func()
		spaceID  int64
		exptID   int64
		recordID int64
		createAt int64
		wantErr  bool
	}{
		{
			name: "success - pending record",
			setup: func() {
				mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), int64(1), int64(1), int64(1)).Return(&entity.ExptInsightAnalysisRecord{
					ID:      1,
					SpaceID: 1,
					ExptID:  1,
					Status:  entity.InsightAnalysisStatus_Unknown,
				}, nil)
				mocks.exptResultExportService.EXPECT().DoExportCSV(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mocks.fileClient.EXPECT().SignDownloadReq(gomock.Any(), gomock.Any(), gomock.Any()).Return("http://test-url.com", make(map[string][]string), nil)
				mocks.agentAdapter.EXPECT().CallTraceAgent(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(123), nil)
				mocks.publisher.EXPECT().PublishExptExportCSVEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any()).Return(nil)
			},
			spaceID:  1,
			exptID:   1,
			recordID: 1,
			createAt: time.Now().Unix(),
			wantErr:  false,
		},
		{
			name: "success - record with report ID",
			setup: func() {
				mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), int64(1), int64(1), int64(1)).Return(&entity.ExptInsightAnalysisRecord{
					ID:               1,
					SpaceID:          1,
					ExptID:           1,
					Status:           entity.InsightAnalysisStatus_Running,
					AnalysisReportID: gptr.Of(int64(123)),
				}, nil)
				mocks.agentAdapter.EXPECT().GetReport(gomock.Any(), gomock.Any(), gomock.Any()).Return("", entity.ReportStatus_Running, nil)
				mocks.publisher.EXPECT().PublishExptExportCSVEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			spaceID:  1,
			exptID:   1,
			recordID: 1,
			createAt: time.Now().Unix(),
			wantErr:  false,
		},
		{
			name: "repo error",
			setup: func() {
				mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), int64(1), int64(1), int64(1)).Return(nil, errors.New("repo error"))
			},
			spaceID:  1,
			exptID:   1,
			recordID: 1,
			createAt: time.Now().Unix(),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := service.GenAnalysisReport(ctx, tt.spaceID, tt.exptID, tt.recordID, tt.createAt)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptInsightAnalysisServiceImpl_GetAnalysisRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func()
		spaceID  int64
		exptID   int64
		recordID int64
		session  *entity.Session
		wantErr  bool
	}{
		{
			name: "success",
			setup: func() {
				mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), int64(1), int64(1), int64(1)).Return(&entity.ExptInsightAnalysisRecord{
					ID:               1,
					SpaceID:          1,
					ExptID:           1,
					Status:           entity.InsightAnalysisStatus_Success,
					AnalysisReportID: gptr.Of(int64(123)),
					CreatedBy:        "user1",
				}, nil)
				mocks.agentAdapter.EXPECT().GetReport(gomock.Any(), gomock.Any(), gomock.Any()).Return("test report content", entity.ReportStatus_Success, nil)
				mocks.repo.EXPECT().CountFeedbackVote(gomock.Any(), int64(1), int64(1), int64(1)).Return(int64(5), int64(2), nil)
				mocks.repo.EXPECT().GetFeedbackVoteByUser(gomock.Any(), int64(1), int64(1), int64(1), "user1").Return(nil, nil)
			},
			spaceID:  1,
			exptID:   1,
			recordID: 1,
			session:  &entity.Session{UserID: "user1"},
			wantErr:  false,
		},
		{
			name: "repo error",
			setup: func() {
				mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), int64(1), int64(1), int64(1)).Return(nil, errors.New("repo error"))
			},
			spaceID:  1,
			exptID:   1,
			recordID: 1,
			session:  &entity.Session{UserID: "user1"},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			result, err := service.GetAnalysisRecordByID(ctx, tt.spaceID, tt.exptID, tt.recordID, tt.session)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, int64(1), result.ID)
			}
		})
	}
}

func TestExptInsightAnalysisServiceImpl_ListAnalysisRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()

	tests := []struct {
		name    string
		setup   func()
		spaceID int64
		exptID  int64
		page    entity.Page
		session *entity.Session
		wantErr bool
	}{
		{
			name: "success",
			setup: func() {
				mocks.repo.EXPECT().ListAnalysisRecord(gomock.Any(), int64(1), int64(1), gomock.Any()).Return([]*entity.ExptInsightAnalysisRecord{
					{ID: 1, SpaceID: 1, ExptID: 1},
					{ID: 2, SpaceID: 1, ExptID: 1},
				}, int64(2), nil)
			},
			spaceID: 1,
			exptID:  1,
			page:    entity.NewPage(0, 10),
			session: &entity.Session{UserID: "user1"},
			wantErr: false,
		},
		{
			name: "repo error",
			setup: func() {
				mocks.repo.EXPECT().ListAnalysisRecord(gomock.Any(), int64(1), int64(1), gomock.Any()).Return(nil, int64(0), errors.New("repo error"))
			},
			spaceID: 1,
			exptID:  1,
			page:    entity.NewPage(0, 10),
			session: &entity.Session{UserID: "user1"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			result, total, err := service.ListAnalysisRecord(ctx, tt.spaceID, tt.exptID, tt.page, tt.session)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, int64(2), total)
			}
		})
	}
}

func TestExptInsightAnalysisServiceImpl_DeleteAnalysisRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func()
		spaceID  int64
		exptID   int64
		recordID int64
		wantErr  bool
	}{
		{
			name: "success",
			setup: func() {
				mocks.repo.EXPECT().DeleteAnalysisRecord(gomock.Any(), int64(1), int64(1), int64(1)).Return(nil)
			},
			spaceID:  1,
			exptID:   1,
			recordID: 1,
			wantErr:  false,
		},
		{
			name: "repo error",
			setup: func() {
				mocks.repo.EXPECT().DeleteAnalysisRecord(gomock.Any(), int64(1), int64(1), int64(1)).Return(errors.New("repo error"))
			},
			spaceID:  1,
			exptID:   1,
			recordID: 1,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := service.DeleteAnalysisRecord(ctx, tt.spaceID, tt.exptID, tt.recordID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptInsightAnalysisServiceImpl_FeedbackExptInsightAnalysis(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()

	tests := []struct {
		name    string
		setup   func()
		param   *entity.ExptInsightAnalysisFeedbackParam
		wantErr bool
	}{
		{
			name: "success - create comment",
			setup: func() {
				mocks.repo.EXPECT().CreateFeedbackComment(gomock.Any(), gomock.Any()).Return(nil)
			},
			param: &entity.ExptInsightAnalysisFeedbackParam{
				SpaceID:            1,
				ExptID:             1,
				AnalysisRecordID:   1,
				FeedbackActionType: entity.FeedbackActionType_CreateComment,
				Comment:            ptr.Of("test comment"),
				Session:            &entity.Session{UserID: "user1"},
			},
			wantErr: false,
		},
		{
			name: "success - update comment",
			setup: func() {
				mocks.repo.EXPECT().UpdateFeedbackComment(gomock.Any(), gomock.Any()).Return(nil)
			},
			param: &entity.ExptInsightAnalysisFeedbackParam{
				SpaceID:            1,
				ExptID:             1,
				AnalysisRecordID:   1,
				FeedbackActionType: entity.FeedbackActionType_Update_Comment,
				Comment:            ptr.Of("updated comment"),
				CommentID:          ptr.Of(int64(1)),
				Session:            &entity.Session{UserID: "user1"},
			},
			wantErr: false,
		},
		{
			name: "success - delete comment",
			setup: func() {
				mocks.repo.EXPECT().DeleteFeedbackComment(gomock.Any(), int64(1), int64(1), int64(1)).Return(nil)
			},
			param: &entity.ExptInsightAnalysisFeedbackParam{
				SpaceID:            1,
				ExptID:             1,
				AnalysisRecordID:   1,
				FeedbackActionType: entity.FeedbackActionType_Delete_Comment,
				CommentID:          ptr.Of(int64(1)),
				Session:            &entity.Session{UserID: "user1"},
			},
			wantErr: false,
		},
		{
			name: "success - create vote",
			setup: func() {
				mocks.repo.EXPECT().CreateFeedbackVote(gomock.Any(), gomock.Any()).Return(nil)
			},
			param: &entity.ExptInsightAnalysisFeedbackParam{
				SpaceID:            1,
				ExptID:             1,
				AnalysisRecordID:   1,
				FeedbackActionType: entity.FeedbackActionType_Upvote,
				Session:            &entity.Session{UserID: "user1"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := service.FeedbackExptInsightAnalysis(ctx, tt.param)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptInsightAnalysisServiceImpl_ListExptInsightAnalysisFeedbackComment(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func()
		spaceID  int64
		exptID   int64
		recordID int64
		page     entity.Page
		wantErr  bool
	}{
		{
			name: "success",
			setup: func() {
				mocks.repo.EXPECT().List(gomock.Any(), int64(1), int64(1), int64(1), gomock.Any()).Return([]*entity.ExptInsightAnalysisFeedbackComment{
					{ID: 1, SpaceID: 1, ExptID: 1, AnalysisRecordID: 1, Comment: "test comment"},
				}, int64(1), nil)
			},
			spaceID:  1,
			exptID:   1,
			recordID: 1,
			page:     entity.NewPage(0, 10),
			wantErr:  false,
		},
		{
			name: "repo error",
			setup: func() {
				mocks.repo.EXPECT().List(gomock.Any(), int64(1), int64(1), int64(1), gomock.Any()).Return(nil, int64(0), errors.New("repo error"))
			},
			spaceID:  1,
			exptID:   1,
			recordID: 1,
			page:     entity.NewPage(0, 10),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			result, total, err := service.ListExptInsightAnalysisFeedbackComment(ctx, tt.spaceID, tt.exptID, tt.recordID, tt.page)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, int64(1), total)
			}
		})
	}
}
