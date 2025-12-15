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

// ... (原有测试内容省略，保持不变)

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
