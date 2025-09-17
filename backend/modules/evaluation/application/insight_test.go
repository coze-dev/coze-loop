// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	exptpb "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repo_mocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
)

func setupTestApp(t *testing.T) (context.Context, *experimentApplication, *servicemocks.MockIExptManager, *repo_mocks.MockIExperimentRepo, *servicemocks.MockIExptInsightAnalysisService, *rpcmocks.MockIAuthProvider) {
	ctrl := gomock.NewController(t)
	mockManager := servicemocks.NewMockIExptManager(ctrl)
	mockRepo := repo_mocks.NewMockIExperimentRepo(ctrl)
	mockInsightService := servicemocks.NewMockIExptInsightAnalysisService(ctrl)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)

	app := &experimentApplication{
		manager:                     mockManager,
		IExptInsightAnalysisService: mockInsightService,
		auth:                        mockAuth,
	}

	return context.Background(), app, mockManager, mockRepo, mockInsightService, mockAuth
}

func TestInsightAnalysisExperiment(t *testing.T) {
	ctx, app, mockManager, _, mockInsightService, mockAuth := setupTestApp(t)

	req := &exptpb.InsightAnalysisExperimentRequest{
		WorkspaceID: 123,
		ExptID:      456,
		Session: &common.Session{
			UserID: &[]int64{789}[0],
		},
	}

	// Mock the manager.Get call
	mockManager.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Experiment{CreatedBy: "test-user"}, nil)
	// Mock the auth.AuthorizationWithoutSPI call
	mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
	// Mock the CreateAnalysisRecord call
	mockInsightService.EXPECT().CreateAnalysisRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(123), nil)

	_, err := app.InsightAnalysisExperiment(ctx, req)
	if err != nil {
		t.Errorf("InsightAnalysisExperiment failed: %v", err)
	}
}

func TestListExptInsightAnalysisRecord(t *testing.T) {
	ctx, app, mockManager, _, mockInsightService, mockAuth := setupTestApp(t)

	req := &exptpb.ListExptInsightAnalysisRecordRequest{
		WorkspaceID: 123,
		ExptID:      456,
		PageNumber:  &[]int32{1}[0],
		PageSize:    &[]int32{10}[0],
		Session: &common.Session{
			UserID: &[]int64{789}[0],
		},
	}

	// Mock the manager.Get call
	mockManager.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Experiment{CreatedBy: "test-user"}, nil)
	// Mock the auth.AuthorizationWithoutSPI call
	mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
	mockInsightService.EXPECT().ListAnalysisRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptInsightAnalysisRecord{}, int64(0), nil)

	_, err := app.ListExptInsightAnalysisRecord(ctx, req)
	if err != nil {
		t.Errorf("ListExptInsightAnalysisRecord failed: %v", err)
	}
}

func TestGetExptInsightAnalysisRecord(t *testing.T) {
	ctx, app, _, _, mockInsightService, mockAuth := setupTestApp(t)

	userID := int64(789)
	req := &exptpb.GetExptInsightAnalysisRecordRequest{
		WorkspaceID:             123,
		ExptID:                  456,
		InsightAnalysisRecordID: 789,
		Session: &common.Session{
			UserID: &userID,
		},
	}

	// Mock the auth.Authorization call
	mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
	// Mock the service call
	mockInsightService.EXPECT().GetAnalysisRecordByID(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.ExptInsightAnalysisRecord{
		ID:        789,
		ExptID:    456,
		SpaceID:   123,
		Status:    entity.InsightAnalysisStatus_Running,
		CreatedBy: "test-user",
	}, nil)

	resp, err := app.GetExptInsightAnalysisRecord(ctx, req)
	if err != nil {
		t.Errorf("GetExptInsightAnalysisRecord failed: %v", err)
	}
	if resp == nil {
		t.Error("Expected non-nil response")
	}
}

func TestDeleteExptInsightAnalysisRecord(t *testing.T) {
	ctx, app, mockManager, _, mockInsightService, mockAuth := setupTestApp(t)

	req := &exptpb.DeleteExptInsightAnalysisRecordRequest{
		WorkspaceID:             123,
		ExptID:                  456,
		InsightAnalysisRecordID: 789,
		Session: &common.Session{
			UserID: &[]int64{789}[0],
		},
	}

	// Mock the manager.Get call
	mockManager.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Experiment{CreatedBy: "test-user"}, nil)
	// Mock the auth.AuthorizationWithoutSPI call
	mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
	mockInsightService.EXPECT().DeleteAnalysisRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	_, err := app.DeleteExptInsightAnalysisRecord(ctx, req)
	if err != nil {
		t.Errorf("DeleteExptInsightAnalysisRecord failed: %v", err)
	}
}

func TestFeedbackExptInsightAnalysisReport(t *testing.T) {
	ctx, app, mockManager, _, mockInsightService, mockAuth := setupTestApp(t)

	req := &exptpb.FeedbackExptInsightAnalysisReportRequest{
		WorkspaceID:             123,
		ExptID:                  456,
		InsightAnalysisRecordID: 789,
		FeedbackActionType:      domain_expt.FeedbackActionTypeUpvote,
		Session: &common.Session{
			UserID: &[]int64{789}[0],
		},
	}

	// Mock the manager.Get call
	mockManager.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Experiment{CreatedBy: "test-user"}, nil)
	// Mock the auth.AuthorizationWithoutSPI call
	mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
	mockInsightService.EXPECT().FeedbackExptInsightAnalysis(gomock.Any(), gomock.Any()).Return(nil)

	_, err := app.FeedbackExptInsightAnalysisReport(ctx, req)
	if err != nil {
		t.Errorf("FeedbackExptInsightAnalysisReport failed: %v", err)
	}
}

func TestListExptInsightAnalysisComment(t *testing.T) {
	ctx, app, _, _, mockInsightService, mockAuth := setupTestApp(t)

	req := &exptpb.ListExptInsightAnalysisCommentRequest{
		WorkspaceID:             123,
		ExptID:                  456,
		InsightAnalysisRecordID: 789,
		PageNumber:              &[]int32{1}[0],
		PageSize:                &[]int32{10}[0],
		Session: &common.Session{
			UserID: &[]int64{789}[0],
		},
	}

	// Mock the auth.Authorization call
	mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
	mockInsightService.EXPECT().ListExptInsightAnalysisFeedbackComment(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptInsightAnalysisFeedbackComment{}, int64(0), nil)

	_, err := app.ListExptInsightAnalysisComment(ctx, req)
	if err != nil {
		t.Errorf("ListExptInsightAnalysisComment failed: %v", err)
	}
}