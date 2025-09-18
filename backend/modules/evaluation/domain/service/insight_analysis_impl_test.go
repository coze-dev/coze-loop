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
		{
			name: "publish event error",
			setup: func() {
				mocks.repo.EXPECT().CreateAnalysisRecord(gomock.Any(), gomock.Any()).Return(int64(123), nil)
				mocks.publisher.EXPECT().PublishExptExportCSVEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("publish error"))
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
				mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
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
		{
			name: "export csv error - defer should update status to failed",
			setup: func() {
				mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), int64(1), int64(1), int64(1)).Return(&entity.ExptInsightAnalysisRecord{
					ID:      1,
					SpaceID: 1,
					ExptID:  1,
					Status:  entity.InsightAnalysisStatus_Unknown,
				}, nil)
				mocks.exptResultExportService.EXPECT().DoExportCSV(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("export error"))
				mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, record *entity.ExptInsightAnalysisRecord, opts ...db.Option) error {
					assert.Equal(t, entity.InsightAnalysisStatus_Failed, record.Status)
					return nil
				})
			},
			spaceID:  1,
			exptID:   1,
			recordID: 1,
			createAt: time.Now().Unix(),
			wantErr:  true,
		},
		{
			name: "sign download req error - defer should update status to failed",
			setup: func() {
				mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), int64(1), int64(1), int64(1)).Return(&entity.ExptInsightAnalysisRecord{
					ID:      1,
					SpaceID: 1,
					ExptID:  1,
					Status:  entity.InsightAnalysisStatus_Unknown,
				}, nil)
				mocks.exptResultExportService.EXPECT().DoExportCSV(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mocks.fileClient.EXPECT().SignDownloadReq(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil, errors.New("sign error"))
				mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, record *entity.ExptInsightAnalysisRecord, opts ...db.Option) error {
					assert.Equal(t, entity.InsightAnalysisStatus_Failed, record.Status)
					return nil
				})
			},
			spaceID:  1,
			exptID:   1,
			recordID: 1,
			createAt: time.Now().Unix(),
			wantErr:  true,
		},
		{
			name: "call trace agent error - defer should update status to failed",
			setup: func() {
				mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), int64(1), int64(1), int64(1)).Return(&entity.ExptInsightAnalysisRecord{
					ID:      1,
					SpaceID: 1,
					ExptID:  1,
					Status:  entity.InsightAnalysisStatus_Unknown,
				}, nil)
				mocks.exptResultExportService.EXPECT().DoExportCSV(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mocks.fileClient.EXPECT().SignDownloadReq(gomock.Any(), gomock.Any(), gomock.Any()).Return("http://test-url.com", make(map[string][]string), nil)
				mocks.agentAdapter.EXPECT().CallTraceAgent(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(0), errors.New("agent error"))
				mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, record *entity.ExptInsightAnalysisRecord, opts ...db.Option) error {
					assert.Equal(t, entity.InsightAnalysisStatus_Failed, record.Status)
					return nil
				})
			},
			spaceID:  1,
			exptID:   1,
			recordID: 1,
			createAt: time.Now().Unix(),
			wantErr:  true,
		},
		{
			name: "publish event error - defer should update status to failed",
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
				mocks.publisher.EXPECT().PublishExptExportCSVEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("publish error"))
				mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, record *entity.ExptInsightAnalysisRecord, opts ...db.Option) error {
					assert.Equal(t, entity.InsightAnalysisStatus_Failed, record.Status)
					return nil
				})
			},
			spaceID:  1,
			exptID:   1,
			recordID: 1,
			createAt: time.Now().Unix(),
			wantErr:  true,
		},
		{
			name: "defer update error - should not affect main error",
			setup: func() {
				mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), int64(1), int64(1), int64(1)).Return(&entity.ExptInsightAnalysisRecord{
					ID:      1,
					SpaceID: 1,
					ExptID:  1,
					Status:  entity.InsightAnalysisStatus_Unknown,
				}, nil)
				mocks.exptResultExportService.EXPECT().DoExportCSV(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("export error"))
				mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("update error"))
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
		{
			name: "status running - early return",
			setup: func() {
				mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), int64(1), int64(1), int64(1)).Return(&entity.ExptInsightAnalysisRecord{
					ID:      1,
					SpaceID: 1,
					ExptID:  1,
					Status:  entity.InsightAnalysisStatus_Running,
				}, nil)
			},
			spaceID:  1,
			exptID:   1,
			recordID: 1,
			session:  &entity.Session{UserID: "user1"},
			wantErr:  false,
		},
		{
			name: "status failed - early return",
			setup: func() {
				mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), int64(1), int64(1), int64(1)).Return(&entity.ExptInsightAnalysisRecord{
					ID:      1,
					SpaceID: 1,
					ExptID:  1,
					Status:  entity.InsightAnalysisStatus_Failed,
				}, nil)
			},
			spaceID:  1,
			exptID:   1,
			recordID: 1,
			session:  &entity.Session{UserID: "user1"},
			wantErr:  false,
		},
		{
			name: "agent adapter get report error",
			setup: func() {
				mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), int64(1), int64(1), int64(1)).Return(&entity.ExptInsightAnalysisRecord{
					ID:               1,
					SpaceID:          1,
					ExptID:           1,
					Status:           entity.InsightAnalysisStatus_Success,
					AnalysisReportID: gptr.Of(int64(123)),
				}, nil)
				mocks.agentAdapter.EXPECT().GetReport(gomock.Any(), gomock.Any(), gomock.Any()).Return("", entity.ReportStatus_Unknown, errors.New("agent error"))
			},
			spaceID:  1,
			exptID:   1,
			recordID: 1,
			session:  &entity.Session{UserID: "user1"},
			wantErr:  true,
		},
		{
			name: "count feedback vote error",
			setup: func() {
				mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), int64(1), int64(1), int64(1)).Return(&entity.ExptInsightAnalysisRecord{
					ID:               1,
					SpaceID:          1,
					ExptID:           1,
					Status:           entity.InsightAnalysisStatus_Success,
					AnalysisReportID: gptr.Of(int64(123)),
				}, nil)
				mocks.agentAdapter.EXPECT().GetReport(gomock.Any(), gomock.Any(), gomock.Any()).Return("test report", entity.ReportStatus_Success, nil)
				mocks.repo.EXPECT().CountFeedbackVote(gomock.Any(), int64(1), int64(1), int64(1)).Return(int64(0), int64(0), errors.New("count error"))
			},
			spaceID:  1,
			exptID:   1,
			recordID: 1,
			session:  &entity.Session{UserID: "user1"},
			wantErr:  true,
		},
		{
			name: "get feedback vote by user error",
			setup: func() {
				mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), int64(1), int64(1), int64(1)).Return(&entity.ExptInsightAnalysisRecord{
					ID:               1,
					SpaceID:          1,
					ExptID:           1,
					Status:           entity.InsightAnalysisStatus_Success,
					AnalysisReportID: gptr.Of(int64(123)),
				}, nil)
				mocks.agentAdapter.EXPECT().GetReport(gomock.Any(), gomock.Any(), gomock.Any()).Return("test report", entity.ReportStatus_Success, nil)
				mocks.repo.EXPECT().CountFeedbackVote(gomock.Any(), int64(1), int64(1), int64(1)).Return(int64(5), int64(2), nil)
				mocks.repo.EXPECT().GetFeedbackVoteByUser(gomock.Any(), int64(1), int64(1), int64(1), "user1").Return(nil, errors.New("get vote error"))
			},
			spaceID:  1,
			exptID:   1,
			recordID: 1,
			session:  &entity.Session{UserID: "user1"},
			wantErr:  true,
		},
		{
			name: "success with user vote",
			setup: func() {
				mocks.repo.EXPECT().GetAnalysisRecordByID(gomock.Any(), int64(1), int64(1), int64(1)).Return(&entity.ExptInsightAnalysisRecord{
					ID:               1,
					SpaceID:          1,
					ExptID:           1,
					Status:           entity.InsightAnalysisStatus_Success,
					AnalysisReportID: gptr.Of(int64(123)),
				}, nil)
				mocks.agentAdapter.EXPECT().GetReport(gomock.Any(), gomock.Any(), gomock.Any()).Return("test report", entity.ReportStatus_Success, nil)
				mocks.repo.EXPECT().CountFeedbackVote(gomock.Any(), int64(1), int64(1), int64(1)).Return(int64(5), int64(2), nil)
				mocks.repo.EXPECT().GetFeedbackVoteByUser(gomock.Any(), int64(1), int64(1), int64(1), "user1").Return(&entity.ExptInsightAnalysisFeedbackVote{
					VoteType: entity.Upvote,
				}, nil)
			},
			spaceID:  1,
			exptID:   1,
			recordID: 1,
			session:  &entity.Session{UserID: "user1"},
			wantErr:  false,
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
			name:  "empty session error",
			setup: func() {},
			param: &entity.ExptInsightAnalysisFeedbackParam{
				SpaceID:            1,
				ExptID:             1,
				AnalysisRecordID:   1,
				FeedbackActionType: entity.FeedbackActionType_Upvote,
				Session:            nil,
			},
			wantErr: true,
		},
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
			name: "success - upvote",
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
		{
			name: "success - downvote",
			setup: func() {
				mocks.repo.EXPECT().CreateFeedbackVote(gomock.Any(), gomock.Any()).Return(nil)
			},
			param: &entity.ExptInsightAnalysisFeedbackParam{
				SpaceID:            1,
				ExptID:             1,
				AnalysisRecordID:   1,
				FeedbackActionType: entity.FeedbackActionType_Downvote,
				Session:            &entity.Session{UserID: "user1"},
			},
			wantErr: false,
		},
		{
			name: "success - cancel upvote",
			setup: func() {
				mocks.repo.EXPECT().UpdateFeedbackVote(gomock.Any(), gomock.Any()).Return(nil)
			},
			param: &entity.ExptInsightAnalysisFeedbackParam{
				SpaceID:            1,
				ExptID:             1,
				AnalysisRecordID:   1,
				FeedbackActionType: entity.FeedbackActionType_CancelUpvote,
				Session:            &entity.Session{UserID: "user1"},
			},
			wantErr: false,
		},
		{
			name: "success - cancel downvote",
			setup: func() {
				mocks.repo.EXPECT().UpdateFeedbackVote(gomock.Any(), gomock.Any()).Return(nil)
			},
			param: &entity.ExptInsightAnalysisFeedbackParam{
				SpaceID:            1,
				ExptID:             1,
				AnalysisRecordID:   1,
				FeedbackActionType: entity.FeedbackActionType_CancelDownvote,
				Session:            &entity.Session{UserID: "user1"},
			},
			wantErr: false,
		},
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
				CommentID:          ptr.Of(int64(123)),
				Session:            &entity.Session{UserID: "user1"},
			},
			wantErr: false,
		},
		{
			name: "success - delete comment",
			setup: func() {
				mocks.repo.EXPECT().DeleteFeedbackComment(gomock.Any(), int64(1), int64(1), int64(123)).Return(nil)
			},
			param: &entity.ExptInsightAnalysisFeedbackParam{
				SpaceID:            1,
				ExptID:             1,
				AnalysisRecordID:   1,
				FeedbackActionType: entity.FeedbackActionType_Delete_Comment,
				CommentID:          ptr.Of(int64(123)),
				Session:            &entity.Session{UserID: "user1"},
			},
			wantErr: false,
		},
		{
			name: "default case - no action",
			setup: func() {
				// No mock expectations for default case
			},
			param: &entity.ExptInsightAnalysisFeedbackParam{
				SpaceID:            1,
				ExptID:             1,
				AnalysisRecordID:   1,
				FeedbackActionType: 999, // Unknown action type
				Session:            &entity.Session{UserID: "user1"},
			},
			wantErr: false,
		},
		{
			name: "repo error - create comment",
			setup: func() {
				mocks.repo.EXPECT().CreateFeedbackComment(gomock.Any(), gomock.Any()).Return(errors.New("repo error"))
			},
			param: &entity.ExptInsightAnalysisFeedbackParam{
				SpaceID:            1,
				ExptID:             1,
				AnalysisRecordID:   1,
				FeedbackActionType: entity.FeedbackActionType_CreateComment,
				Comment:            ptr.Of("test comment"),
				Session:            &entity.Session{UserID: "user1"},
			},
			wantErr: true,
		},
		{
			name: "repo error - upvote",
			setup: func() {
				mocks.repo.EXPECT().CreateFeedbackVote(gomock.Any(), gomock.Any()).Return(errors.New("repo error"))
			},
			param: &entity.ExptInsightAnalysisFeedbackParam{
				SpaceID:            1,
				ExptID:             1,
				AnalysisRecordID:   1,
				FeedbackActionType: entity.FeedbackActionType_Upvote,
				Session:            &entity.Session{UserID: "user1"},
			},
			wantErr: true,
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

// TestNewInsightAnalysisService 测试构造函数
func TestNewInsightAnalysisService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repoMocks.NewMockIExptInsightAnalysisRecordRepo(ctrl)
	mockPublisher := eventsMocks.NewMockExptEventPublisher(ctrl)
	mockFileClient := fileMocks.NewMockObjectStorage(ctrl)
	mockExptResultExportService := serviceMocks.NewMockIExptResultExportService(ctrl)
	mockExptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	mockAgentAdapter := rpcMocks.NewMockIAgentAdapter(ctrl)
	mockNotifyRPCAdapter := rpcMocks.NewMockINotifyRPCAdapter(ctrl)
	mockUserProvider := rpcMocks.NewMockIUserProvider(ctrl)

	service := NewInsightAnalysisService(
		mockRepo,
		mockPublisher,
		mockFileClient,
		mockAgentAdapter,
		mockExptResultExportService,
		mockNotifyRPCAdapter,
		mockUserProvider,
		mockExptRepo,
	)

	assert.NotNil(t, service)
	assert.IsType(t, &ExptInsightAnalysisServiceImpl{}, service)
}

// TestExptInsightAnalysisServiceImpl_notifyAnalysisComplete 测试通知分析完成
func TestExptInsightAnalysisServiceImpl_notifyAnalysisComplete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()

	tests := []struct {
		name    string
		setup   func()
		userID  string
		spaceID int64
		exptID  int64
		wantErr bool
	}{
		{
			name: "success",
			setup: func() {
				mocks.exptRepo.EXPECT().GetByID(gomock.Any(), int64(1), int64(1)).Return(&entity.Experiment{
					ID:   1,
					Name: "test experiment",
				}, nil)
				mocks.userProvider.EXPECT().MGetUserInfo(gomock.Any(), []string{"user1"}).Return([]*entity.UserInfo{
					{Email: ptr.Of("user1@example.com")},
				}, nil)
				mocks.notifyRPCAdapter.EXPECT().SendMessageCard(gomock.Any(), gomock.Any(), "AAq9DvIYd2qHu", gomock.Any()).Return(nil)
			},
			userID:  "user1",
			spaceID: 1,
			exptID:  1,
			wantErr: false,
		},
		{
			name: "expt repo error",
			setup: func() {
				mocks.exptRepo.EXPECT().GetByID(gomock.Any(), int64(1), int64(1)).Return(nil, errors.New("expt repo error"))
			},
			userID:  "user1",
			spaceID: 1,
			exptID:  1,
			wantErr: true,
		},
		{
			name: "user provider error",
			setup: func() {
				mocks.exptRepo.EXPECT().GetByID(gomock.Any(), int64(1), int64(1)).Return(&entity.Experiment{
					ID:   1,
					Name: "test experiment",
				}, nil)
				mocks.userProvider.EXPECT().MGetUserInfo(gomock.Any(), []string{"user1"}).Return(nil, errors.New("user provider error"))
			},
			userID:  "user1",
			spaceID: 1,
			exptID:  1,
			wantErr: true,
		},
		{
			name: "empty user info - no error",
			setup: func() {
				mocks.exptRepo.EXPECT().GetByID(gomock.Any(), int64(1), int64(1)).Return(&entity.Experiment{
					ID:   1,
					Name: "test experiment",
				}, nil)
				mocks.userProvider.EXPECT().MGetUserInfo(gomock.Any(), []string{"user1"}).Return([]*entity.UserInfo{}, nil)
			},
			userID:  "user1",
			spaceID: 1,
			exptID:  1,
			wantErr: false,
		},
		{
			name: "nil user info - no error",
			setup: func() {
				mocks.exptRepo.EXPECT().GetByID(gomock.Any(), int64(1), int64(1)).Return(&entity.Experiment{
					ID:   1,
					Name: "test experiment",
				}, nil)
				mocks.userProvider.EXPECT().MGetUserInfo(gomock.Any(), []string{"user1"}).Return([]*entity.UserInfo{nil}, nil)
			},
			userID:  "user1",
			spaceID: 1,
			exptID:  1,
			wantErr: false,
		},
		{
			name: "notify adapter error",
			setup: func() {
				mocks.exptRepo.EXPECT().GetByID(gomock.Any(), int64(1), int64(1)).Return(&entity.Experiment{
					ID:   1,
					Name: "test experiment",
				}, nil)
				mocks.userProvider.EXPECT().MGetUserInfo(gomock.Any(), []string{"user1"}).Return([]*entity.UserInfo{
					{Email: ptr.Of("user1@example.com")},
				}, nil)
				mocks.notifyRPCAdapter.EXPECT().SendMessageCard(gomock.Any(), gomock.Any(), "AAq9DvIYd2qHu", gomock.Any()).Return(errors.New("notify error"))
			},
			userID:  "user1",
			spaceID: 1,
			exptID:  1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := service.notifyAnalysisComplete(ctx, tt.userID, tt.spaceID, tt.exptID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestExptInsightAnalysisServiceImpl_checkAnalysisReportGenStatus 测试检查分析报告生成状态
func TestExptInsightAnalysisServiceImpl_checkAnalysisReportGenStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, mocks := newTestInsightAnalysisService(ctrl)
	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func()
		record   *entity.ExptInsightAnalysisRecord
		createAt int64
		wantErr  bool
	}{
		{
			name: "agent adapter error",
			setup: func() {
				mocks.agentAdapter.EXPECT().GetReport(gomock.Any(), int64(1), int64(123)).Return("", entity.ReportStatus_Unknown, errors.New("agent error"))
			},
			record: &entity.ExptInsightAnalysisRecord{
				ID:               1,
				SpaceID:          1,
				ExptID:           1,
				AnalysisReportID: gptr.Of(int64(123)),
				CreatedBy:        "user1",
			},
			createAt: time.Now().Unix(),
			wantErr:  true,
		},
		{
			name: "report status failed",
			setup: func() {
				mocks.agentAdapter.EXPECT().GetReport(gomock.Any(), int64(1), int64(123)).Return("", entity.ReportStatus_Failed, nil)
				mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any()).Return(nil)
			},
			record: &entity.ExptInsightAnalysisRecord{
				ID:               1,
				SpaceID:          1,
				ExptID:           1,
				AnalysisReportID: gptr.Of(int64(123)),
				CreatedBy:        "user1",
			},
			createAt: time.Now().Unix(),
			wantErr:  false,
		},
		{
			name: "report status failed - update error",
			setup: func() {
				mocks.agentAdapter.EXPECT().GetReport(gomock.Any(), int64(1), int64(123)).Return("", entity.ReportStatus_Failed, nil)
				mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any()).Return(errors.New("update error"))
			},
			record: &entity.ExptInsightAnalysisRecord{
				ID:               1,
				SpaceID:          1,
				ExptID:           1,
				AnalysisReportID: gptr.Of(int64(123)),
				CreatedBy:        "user1",
			},
			createAt: time.Now().Unix(),
			wantErr:  true,
		},
		{
			name: "report status success",
			setup: func() {
				mocks.agentAdapter.EXPECT().GetReport(gomock.Any(), int64(1), int64(123)).Return("report content", entity.ReportStatus_Success, nil)
				mocks.exptRepo.EXPECT().GetByID(gomock.Any(), int64(1), int64(1)).Return(&entity.Experiment{
					ID:   1,
					Name: "test experiment",
				}, nil)
				mocks.userProvider.EXPECT().MGetUserInfo(gomock.Any(), []string{"user1"}).Return([]*entity.UserInfo{
					{Email: ptr.Of("user1@example.com")},
				}, nil)
				mocks.notifyRPCAdapter.EXPECT().SendMessageCard(gomock.Any(), gomock.Any(), "AAq9DvIYd2qHu", gomock.Any()).Return(nil)
				mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any()).Return(nil)
			},
			record: &entity.ExptInsightAnalysisRecord{
				ID:               1,
				SpaceID:          1,
				ExptID:           1,
				AnalysisReportID: gptr.Of(int64(123)),
				CreatedBy:        "user1",
			},
			createAt: time.Now().Unix(),
			wantErr:  false,
		},
		{
			name: "report status success - notify error (should not fail)",
			setup: func() {
				mocks.agentAdapter.EXPECT().GetReport(gomock.Any(), int64(1), int64(123)).Return("report content", entity.ReportStatus_Success, nil)
				mocks.exptRepo.EXPECT().GetByID(gomock.Any(), int64(1), int64(1)).Return(&entity.Experiment{
					ID:   1,
					Name: "test experiment",
				}, nil)
				mocks.userProvider.EXPECT().MGetUserInfo(gomock.Any(), []string{"user1"}).Return([]*entity.UserInfo{
					{Email: ptr.Of("user1@example.com")},
				}, nil)
				mocks.notifyRPCAdapter.EXPECT().SendMessageCard(gomock.Any(), gomock.Any(), "AAq9DvIYd2qHu", gomock.Any()).Return(errors.New("notify error"))
				mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any()).Return(nil)
			},
			record: &entity.ExptInsightAnalysisRecord{
				ID:               1,
				SpaceID:          1,
				ExptID:           1,
				AnalysisReportID: gptr.Of(int64(123)),
				CreatedBy:        "user1",
			},
			createAt: time.Now().Unix(),
			wantErr:  false,
		},
		{
			name: "timeout case",
			setup: func() {
				mocks.agentAdapter.EXPECT().GetReport(gomock.Any(), int64(1), int64(123)).Return("", entity.ReportStatus_Running, nil)
				mocks.repo.EXPECT().UpdateAnalysisRecord(gomock.Any(), gomock.Any()).Return(nil)
			},
			record: &entity.ExptInsightAnalysisRecord{
				ID:               1,
				SpaceID:          1,
				ExptID:           1,
				AnalysisReportID: gptr.Of(int64(123)),
				CreatedBy:        "user1",
			},
			createAt: time.Now().Unix() - 3700, // 超过1小时
			wantErr:  false,
		},
		{
			name: "running status - publish event",
			setup: func() {
				mocks.agentAdapter.EXPECT().GetReport(gomock.Any(), int64(1), int64(123)).Return("", entity.ReportStatus_Running, nil)
				mocks.publisher.EXPECT().PublishExptExportCSVEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			record: &entity.ExptInsightAnalysisRecord{
				ID:               1,
				SpaceID:          1,
				ExptID:           1,
				AnalysisReportID: gptr.Of(int64(123)),
				CreatedBy:        "user1",
			},
			createAt: time.Now().Unix(),
			wantErr:  false,
		},
		{
			name: "running status - publish event error",
			setup: func() {
				mocks.agentAdapter.EXPECT().GetReport(gomock.Any(), int64(1), int64(123)).Return("", entity.ReportStatus_Running, nil)
				mocks.publisher.EXPECT().PublishExptExportCSVEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("publish error"))
			},
			record: &entity.ExptInsightAnalysisRecord{
				ID:               1,
				SpaceID:          1,
				ExptID:           1,
				AnalysisReportID: gptr.Of(int64(123)),
				CreatedBy:        "user1",
			},
			createAt: time.Now().Unix(),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := service.checkAnalysisReportGenStatus(ctx, tt.record, tt.createAt)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
