// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/external/audit"
	auditmocks "github.com/coze-dev/coze-loop/backend/infra/external/audit/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	benefitmocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	evaluatorservice "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/evaluator"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/evaluator"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	metricsmock "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	userinfomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/userinfo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/conf/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestEvaluatorHandlerImpl_ListEvaluators(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
	mockUserInfoService := userinfomocks.NewMockUserInfoService(ctrl)

	app := &EvaluatorHandlerImpl{
		auth:             mockAuth,
		evaluatorService: mockEvaluatorService,
		userInfoService:  mockUserInfoService,
	}

	// Test data
	validSpaceID := int64(123)
	validEvaluators := []*entity.Evaluator{
		{
			ID:             1,
			SpaceID:        validSpaceID,
			Name:           "test-evaluator-1",
			EvaluatorType:  entity.EvaluatorTypePrompt,
			Description:    "test description 1",
			DraftSubmitted: true,
		},
		{
			ID:             2,
			SpaceID:        validSpaceID,
			Name:           "test-evaluator-2",
			EvaluatorType:  entity.EvaluatorTypeCode,
			Description:    "test description 2",
			DraftSubmitted: false,
		},
	}

	tests := []struct {
		name        string
		req         *evaluatorservice.ListEvaluatorsRequest
		mockSetup   func()
		wantResp    *evaluatorservice.ListEvaluatorsResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &evaluatorservice.ListEvaluatorsRequest{
				WorkspaceID: validSpaceID,
			},
			mockSetup: func() {
				// Mock auth
				mockAuth.EXPECT().Authorization(gomock.Any(), &rpc.AuthorizationParam{
					ObjectID:      strconv.FormatInt(validSpaceID, 10),
					SpaceID:       validSpaceID,
					ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
				}).Return(nil)

				// Mock service call
				mockEvaluatorService.EXPECT().ListEvaluator(gomock.Any(), gomock.Any()).
					Return(validEvaluators, int64(2), nil)

				// Mock user info service
				mockUserInfoService.EXPECT().PackUserInfo(gomock.Any(), gomock.Any()).Return()
			},
			wantResp: &evaluatorservice.ListEvaluatorsResponse{
				Total: gptr.Of(int64(2)),
				Evaluators: []*evaluatordto.Evaluator{
					evaluator.ConvertEvaluatorDO2DTO(validEvaluators[0]),
					evaluator.ConvertEvaluatorDO2DTO(validEvaluators[1]),
				},
			},
			wantErr: false,
		},
		{
			name: "error - auth failed",
			req: &evaluatorservice.ListEvaluatorsRequest{
				WorkspaceID: validSpaceID,
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
		{
			name: "error - service failure",
			req: &evaluatorservice.ListEvaluatorsRequest{
				WorkspaceID: validSpaceID,
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockEvaluatorService.EXPECT().ListEvaluator(gomock.Any(), gomock.Any()).
					Return(nil, int64(0), errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
		{
			name: "success - with pagination",
			req: &evaluatorservice.ListEvaluatorsRequest{
				WorkspaceID: validSpaceID,
				PageSize:    gptr.Of(int32(1)),
				PageNumber:  gptr.Of(int32(1)),
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockEvaluatorService.EXPECT().ListEvaluator(gomock.Any(), gomock.Any()).
					Return(validEvaluators[:1], int64(2), nil)
				mockUserInfoService.EXPECT().PackUserInfo(gomock.Any(), gomock.Any()).Return()
			},
			wantResp: &evaluatorservice.ListEvaluatorsResponse{
				Total: gptr.Of(int64(2)),
				Evaluators: []*evaluatordto.Evaluator{
					evaluator.ConvertEvaluatorDO2DTO(validEvaluators[0]),
				},
			},
			wantErr: false,
		},
		{
			name: "success - with search name",
			req: &evaluatorservice.ListEvaluatorsRequest{
				WorkspaceID: validSpaceID,
				SearchName:  gptr.Of("test-evaluator-1"),
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockEvaluatorService.EXPECT().ListEvaluator(gomock.Any(), gomock.Any()).
					Return(validEvaluators[:1], int64(1), nil)
				mockUserInfoService.EXPECT().PackUserInfo(gomock.Any(), gomock.Any()).Return()
			},
			wantResp: &evaluatorservice.ListEvaluatorsResponse{
				Total: gptr.Of(int64(1)),
				Evaluators: []*evaluatordto.Evaluator{
					evaluator.ConvertEvaluatorDO2DTO(validEvaluators[0]),
				},
			},
			wantErr: false,
		},
		{
			name: "success - with evaluator type filter",
			req: &evaluatorservice.ListEvaluatorsRequest{
				WorkspaceID:   validSpaceID,
				EvaluatorType: []evaluatordto.EvaluatorType{evaluatordto.EvaluatorType_Prompt},
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockEvaluatorService.EXPECT().ListEvaluator(gomock.Any(), gomock.Any()).
					Return(validEvaluators[:1], int64(1), nil)
				mockUserInfoService.EXPECT().PackUserInfo(gomock.Any(), gomock.Any()).Return()
			},
			wantResp: &evaluatorservice.ListEvaluatorsResponse{
				Total: gptr.Of(int64(1)),
				Evaluators: []*evaluatordto.Evaluator{
					evaluator.ConvertEvaluatorDO2DTO(validEvaluators[0]),
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := app.ListEvaluators(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantResp.Total, resp.Total)
				assert.Equal(t, len(tt.wantResp.Evaluators), len(resp.Evaluators))
				for i, evaluator := range tt.wantResp.Evaluators {
					assert.Equal(t, evaluator.GetEvaluatorID(), resp.Evaluators[i].GetEvaluatorID())
					assert.Equal(t, evaluator.GetWorkspaceID(), resp.Evaluators[i].GetWorkspaceID())
					assert.Equal(t, evaluator.GetName(), resp.Evaluators[i].GetName())
					assert.Equal(t, evaluator.GetEvaluatorType(), resp.Evaluators[i].GetEvaluatorType())
				}
			}
		})
	}
}

func TestEvaluatorHandlerImpl_GetEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
	mockUserInfoService := userinfomocks.NewMockUserInfoService(ctrl)

	app := &EvaluatorHandlerImpl{
		auth:             mockAuth,
		evaluatorService: mockEvaluatorService,
		userInfoService:  mockUserInfoService,
	}

	// Test data
	validSpaceID := int64(123)
	validEvaluatorID := int64(456)
	validEvaluator := &entity.Evaluator{
		ID:             validEvaluatorID,
		SpaceID:        validSpaceID,
		Name:           "Test Evaluator",
		EvaluatorType:  entity.EvaluatorTypePrompt,
		Description:    "Test Description",
		DraftSubmitted: true,
	}

	tests := []struct {
		name        string
		req         *evaluatorservice.GetEvaluatorRequest
		mockSetup   func()
		wantResp    *evaluatorservice.GetEvaluatorResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &evaluatorservice.GetEvaluatorRequest{
				WorkspaceID: validSpaceID,
				EvaluatorID: &validEvaluatorID,
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), validSpaceID, validEvaluatorID, false).
					Return(validEvaluator, nil)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(validEvaluator.ID, 10),
						SpaceID:       validSpaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
					}).
					Return(nil)

				mockUserInfoService.EXPECT().
					PackUserInfo(gomock.Any(), gomock.Any()).
					Return()
			},
			wantResp: &evaluatorservice.GetEvaluatorResponse{
				Evaluator: evaluator.ConvertEvaluatorDO2DTO(validEvaluator),
			},
			wantErr: false,
		},
		{
			name: "error - evaluator not found",
			req: &evaluatorservice.GetEvaluatorRequest{
				WorkspaceID: validSpaceID,
				EvaluatorID: &validEvaluatorID,
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), validSpaceID, validEvaluatorID, false).
					Return(nil, nil)
			},
			wantResp: &evaluatorservice.GetEvaluatorResponse{},
			wantErr:  false,
		},
		{
			name: "error - auth failed",
			req: &evaluatorservice.GetEvaluatorRequest{
				WorkspaceID: validSpaceID,
				EvaluatorID: &validEvaluatorID,
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), validSpaceID, validEvaluatorID, false).
					Return(validEvaluator, nil)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := app.GetEvaluator(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantResp, resp)
			}
		})
	}
}

func TestEvaluatorHandlerImpl_CreateEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
	mockAuditClient := auditmocks.NewMockIAuditService(ctrl)
	mockMetrics := metricsmock.NewMockEvaluatorExecMetrics(ctrl)

	app := &EvaluatorHandlerImpl{
		auth:             mockAuth,
		evaluatorService: mockEvaluatorService,
		auditClient:      mockAuditClient,
		metrics:          mockMetrics,
	}

	// Test data
	validSpaceID := int64(123)
	validName := "Test Evaluator"
	validDescription := "Test Description"
	validEvaluatorType := evaluatordto.EvaluatorType_Prompt
	validVersion := "1.0.0"

	tests := []struct {
		name        string
		req         *evaluatorservice.CreateEvaluatorRequest
		mockSetup   func()
		wantResp    *evaluatorservice.CreateEvaluatorResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &evaluatorservice.CreateEvaluatorRequest{
				Evaluator: &evaluatordto.Evaluator{
					WorkspaceID:   gptr.Of(validSpaceID),
					Name:          gptr.Of(validName),
					Description:   gptr.Of(validDescription),
					EvaluatorType: &validEvaluatorType,
					CurrentVersion: &evaluatordto.EvaluatorVersion{
						Version: gptr.Of(validVersion),
						EvaluatorContent: &evaluatordto.EvaluatorContent{
							PromptEvaluator: &evaluatordto.PromptEvaluator{},
						},
					},
				},
			},
			mockSetup: func() {
				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(validSpaceID, 10),
						SpaceID:       validSpaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("createLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
					}).
					Return(nil)

				mockAuditClient.EXPECT().
					Audit(gomock.Any(), gomock.Any()).
					Return(audit.AuditRecord{AuditStatus: audit.AuditStatus_Approved}, nil).Times(1)

				mockEvaluatorService.EXPECT().
					CreateEvaluator(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(int64(1), nil).Times(1)
				mockMetrics.EXPECT().
					EmitCreate(gomock.Any(), gomock.Any()).
					Return().Times(1)
			},
			wantResp: &evaluatorservice.CreateEvaluatorResponse{
				EvaluatorID: gptr.Of(int64(1)),
			},
			wantErr: false,
		},
		{
			name: "error - audit rejected",
			req: &evaluatorservice.CreateEvaluatorRequest{
				Evaluator: &evaluatordto.Evaluator{
					WorkspaceID:   gptr.Of(validSpaceID),
					Name:          gptr.Of(validName),
					Description:   gptr.Of(validDescription),
					EvaluatorType: &validEvaluatorType,
					CurrentVersion: &evaluatordto.EvaluatorVersion{
						Version: gptr.Of(validVersion),
						EvaluatorContent: &evaluatordto.EvaluatorContent{
							PromptEvaluator: &evaluatordto.PromptEvaluator{},
						},
					},
				},
			},
			mockSetup: func() {
				mockAuditClient.EXPECT().
					Audit(gomock.Any(), gomock.Any()).
					Return(audit.AuditRecord{AuditStatus: audit.AuditStatus_Rejected}, nil).Times(1)
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.RiskContentDetectedCode,
		},
		{
			name: "error - auth failed",
			req: &evaluatorservice.CreateEvaluatorRequest{
				Evaluator: &evaluatordto.Evaluator{
					WorkspaceID:   gptr.Of(validSpaceID),
					Name:          gptr.Of(validName),
					Description:   gptr.Of(validDescription),
					EvaluatorType: &validEvaluatorType,
					CurrentVersion: &evaluatordto.EvaluatorVersion{
						Version: gptr.Of(validVersion),
						EvaluatorContent: &evaluatordto.EvaluatorContent{
							PromptEvaluator: &evaluatordto.PromptEvaluator{},
						},
					},
				},
			},
			mockSetup: func() {
				mockAuditClient.EXPECT().
					Audit(gomock.Any(), gomock.Any()).
					Return(audit.AuditRecord{AuditStatus: audit.AuditStatus_Approved}, nil).Times(1)
				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(errno.CommonNoPermissionCode)).Times(1)
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := app.CreateEvaluator(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantResp, resp)
			}
		})
	}
}

func TestEvaluatorHandlerImpl_UpdateEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
	mockAuditClient := auditmocks.NewMockIAuditService(ctrl)

	app := &EvaluatorHandlerImpl{
		auth:             mockAuth,
		evaluatorService: mockEvaluatorService,
		auditClient:      mockAuditClient,
	}

	// Test data
	validSpaceID := int64(123)
	validEvaluatorID := int64(1)
	validName := "Updated Evaluator"
	validDescription := "Updated Description"

	tests := []struct {
		name        string
		req         *evaluatorservice.UpdateEvaluatorRequest
		mockSetup   func()
		wantResp    *evaluatorservice.UpdateEvaluatorResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &evaluatorservice.UpdateEvaluatorRequest{
				WorkspaceID: validSpaceID,
				EvaluatorID: validEvaluatorID,
				Name:        gptr.Of(validName),
				Description: gptr.Of(validDescription),
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), validSpaceID, validEvaluatorID, false).
					Return(&entity.Evaluator{
						ID:      validEvaluatorID,
						SpaceID: validSpaceID,
					}, nil).Times(1)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				mockAuditClient.EXPECT().
					Audit(gomock.Any(), gomock.Any()).
					Return(audit.AuditRecord{AuditStatus: audit.AuditStatus_Approved}, nil).Times(1)

				mockEvaluatorService.EXPECT().
					UpdateEvaluatorMeta(gomock.Any(), validEvaluatorID, validSpaceID, validName, validDescription, gomock.Any()).
					Return(nil).Times(1)
			},
			wantResp: &evaluatorservice.UpdateEvaluatorResponse{},
			wantErr:  false,
		},
		{
			name: "error - evaluator not found",
			req: &evaluatorservice.UpdateEvaluatorRequest{
				WorkspaceID: validSpaceID,
				EvaluatorID: validEvaluatorID,
				Name:        gptr.Of(validName),
				Description: gptr.Of(validDescription),
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), validSpaceID, validEvaluatorID, false).
					Return(nil, nil).Times(1)
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.EvaluatorNotExistCode,
		},
		{
			name: "error - auth failed",
			req: &evaluatorservice.UpdateEvaluatorRequest{
				WorkspaceID: validSpaceID,
				EvaluatorID: validEvaluatorID,
				Name:        gptr.Of(validName),
				Description: gptr.Of(validDescription),
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), validSpaceID, validEvaluatorID, false).
					Return(&entity.Evaluator{
						ID:      validEvaluatorID,
						SpaceID: validSpaceID,
					}, nil).Times(1)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(errno.CommonNoPermissionCode)).Times(1)
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
		{
			name: "error - audit rejected",
			req: &evaluatorservice.UpdateEvaluatorRequest{
				WorkspaceID: validSpaceID,
				EvaluatorID: validEvaluatorID,
				Name:        gptr.Of(validName),
				Description: gptr.Of(validDescription),
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), validSpaceID, validEvaluatorID, false).
					Return(&entity.Evaluator{
						ID:      validEvaluatorID,
						SpaceID: validSpaceID,
					}, nil).Times(1)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				mockAuditClient.EXPECT().
					Audit(gomock.Any(), gomock.Any()).
					Return(audit.AuditRecord{AuditStatus: audit.AuditStatus_Rejected}, nil).Times(1)
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.RiskContentDetectedCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := app.UpdateEvaluator(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantResp, resp)
			}
		})
	}
}

func TestEvaluatorHandlerImpl_UpdateEvaluatorDraft(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockConfiger := confmocks.NewMockIConfiger(ctrl)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
	mockEvaluatorRecordService := mocks.NewMockEvaluatorRecordService(ctrl)
	mockMetrics := metricsmock.NewMockEvaluatorExecMetrics(ctrl)
	mockUserInfoService := userinfomocks.NewMockUserInfoService(ctrl)
	mockAuditService := auditmocks.NewMockIAuditService(ctrl)
	mockBenefitService := benefitmocks.NewMockIBenefitService(ctrl)

	handler := NewEvaluatorHandlerImpl(
		mockIDGen,
		mockConfiger,
		mockAuth,
		mockEvaluatorService,
		mockEvaluatorRecordService,
		mockMetrics,
		mockUserInfoService,
		mockAuditService,
		mockBenefitService,
	)

	validSpaceID := int64(1)
	validEvaluatorID := int64(1)
	validPromptTemplate := &evaluatordto.PromptEvaluator{
		MessageList: []*common.Message{
			{
				Role: common.RolePtr(common.Role_System),
				Content: &common.Content{
					Text: gptr.Of("Hello, world!"),
				},
			},
		},
	}

	tests := []struct {
		name          string
		request       *evaluatorservice.UpdateEvaluatorDraftRequest
		mockSetup     func()
		expectedResp  *evaluatorservice.UpdateEvaluatorDraftResponse
		expectedError error
	}{
		{
			name: "success - normal request",
			request: &evaluatorservice.UpdateEvaluatorDraftRequest{
				WorkspaceID: validSpaceID,
				EvaluatorID: validEvaluatorID,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					PromptEvaluator: validPromptTemplate,
				},
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockEvaluatorService.EXPECT().GetEvaluator(gomock.Any(), validSpaceID, validEvaluatorID, false).Return(&entity.Evaluator{
					ID:            validEvaluatorID,
					SpaceID:       validSpaceID,
					EvaluatorType: entity.EvaluatorTypePrompt,
					BaseInfo: &entity.BaseInfo{
						CreatedBy: &entity.UserInfo{
							UserID: ptr.Of("1"),
						},
					},
					PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
						ID: 1,
					},
				}, nil).Times(1)
				mockEvaluatorService.EXPECT().UpdateEvaluatorDraft(gomock.Any(), gomock.Any()).Return(nil).Times(1)
				mockUserInfoService.EXPECT().PackUserInfo(gomock.Any(), gomock.Any()).Return().Times(1)
			},
			expectedResp:  &evaluatorservice.UpdateEvaluatorDraftResponse{},
			expectedError: nil,
		},
		{
			name: "error - evaluator not found",
			request: &evaluatorservice.UpdateEvaluatorDraftRequest{
				WorkspaceID: validSpaceID,
				EvaluatorID: validEvaluatorID,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					PromptEvaluator: validPromptTemplate,
				},
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().GetEvaluator(gomock.Any(), validSpaceID, validEvaluatorID, false).Return(nil, nil).Times(1)
			},
			expectedResp:  nil,
			expectedError: errors.New("evaluator not found"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			resp, err := handler.UpdateEvaluatorDraft(context.Background(), tt.request)
			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			}
		})
	}
}

func TestEvaluatorHandlerImpl_DeleteEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockConfiger := confmocks.NewMockIConfiger(ctrl)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
	mockEvaluatorRecordService := mocks.NewMockEvaluatorRecordService(ctrl)
	mockMetrics := metricsmock.NewMockEvaluatorExecMetrics(ctrl)
	mockUserInfoService := userinfomocks.NewMockUserInfoService(ctrl)
	mockAuditService := auditmocks.NewMockIAuditService(ctrl)
	mockBenefitService := benefitmocks.NewMockIBenefitService(ctrl)

	handler := NewEvaluatorHandlerImpl(
		mockIDGen,
		mockConfiger,
		mockAuth,
		mockEvaluatorService,
		mockEvaluatorRecordService,
		mockMetrics,
		mockUserInfoService,
		mockAuditService,
		mockBenefitService,
	)

	validSpaceID := int64(1)
	validEvaluatorID := int64(1)
	validUserID := "1"

	tests := []struct {
		name          string
		request       *evaluatorservice.DeleteEvaluatorRequest
		mockSetup     func()
		expectedResp  *evaluatorservice.DeleteEvaluatorResponse
		expectedError error
	}{
		{
			name: "success - normal request",
			request: &evaluatorservice.DeleteEvaluatorRequest{
				WorkspaceID: validSpaceID,
				EvaluatorID: ptr.Of(validEvaluatorID),
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().BatchGetEvaluator(gomock.Any(), validSpaceID, []int64{validEvaluatorID}, false).Return([]*entity.Evaluator{
					{
						ID:      validEvaluatorID,
						SpaceID: validSpaceID,
						BaseInfo: &entity.BaseInfo{
							CreatedBy: &entity.UserInfo{
								UserID: ptr.Of(validUserID),
							},
						},
					},
				}, nil).Times(1)
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				mockEvaluatorService.EXPECT().DeleteEvaluator(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			expectedResp:  &evaluatorservice.DeleteEvaluatorResponse{},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			resp, err := handler.DeleteEvaluator(context.Background(), tt.request)
			if tt.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			}
		})
	}
}

func TestEvaluatorHandlerImpl_GetEvaluatorVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
	mockUserInfoService := userinfomocks.NewMockUserInfoService(ctrl)

	handler := &EvaluatorHandlerImpl{
		evaluatorService: mockEvaluatorService,
		auth:             mockAuth,
		userInfoService:  mockUserInfoService,
	}

	// Test data
	validSpaceID := int64(1)
	validEvaluatorVersionID := int64(10)
	validEvaluatorID := int64(100)
	validVersion := "v1.0.0"

	validEvaluator := &entity.Evaluator{
		ID:            validEvaluatorID,
		SpaceID:       validSpaceID,
		EvaluatorType: entity.EvaluatorTypePrompt,
		PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
			ID:      validEvaluatorVersionID,
			Version: validVersion,
		},
	}

	tests := []struct {
		name        string
		req         *evaluatorservice.GetEvaluatorVersionRequest
		mockSetup   func()
		wantResp    *evaluatorservice.GetEvaluatorVersionResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "成功 - 正常请求",
			req: &evaluatorservice.GetEvaluatorVersionRequest{
				EvaluatorVersionID: validEvaluatorVersionID,
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					GetEvaluatorVersion(gomock.Any(), validEvaluatorVersionID, false).
					Return(validEvaluator, nil)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(validEvaluatorID, 10),
						SpaceID:       validSpaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
					}).
					Return(nil)

				mockUserInfoService.EXPECT().
					PackUserInfo(gomock.Any(), gomock.Any()).
					Times(2) // 一次用于 Evaluator，一次用于 EvaluatorVersion
			},
			wantResp: &evaluatorservice.GetEvaluatorVersionResponse{
				Evaluator: evaluator.ConvertEvaluatorDO2DTO(validEvaluator),
			},
			wantErr: false,
		},
		{
			name: "成功 - 评估器不存在",
			req: &evaluatorservice.GetEvaluatorVersionRequest{
				EvaluatorVersionID: validEvaluatorVersionID,
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					GetEvaluatorVersion(gomock.Any(), validEvaluatorVersionID, false).
					Return(nil, nil)
			},
			wantResp: &evaluatorservice.GetEvaluatorVersionResponse{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := handler.GetEvaluatorVersion(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantResp, resp)
			}
		})
	}
}

func TestEvaluatorHandlerImpl_BatchGetEvaluatorVersions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
	mockUserInfoService := userinfomocks.NewMockUserInfoService(ctrl)

	handler := &EvaluatorHandlerImpl{
		evaluatorService: mockEvaluatorService,
		auth:             mockAuth,
		userInfoService:  mockUserInfoService,
	}

	// Test data
	validSpaceID := int64(1)
	validEvaluatorVersionIDs := []int64{10, 11}
	validEvaluatorID1 := int64(100)
	validEvaluatorID2 := int64(101)
	validVersion1 := "v1.0.0"
	validVersion2 := "v1.0.1"

	validEvaluators := []*entity.Evaluator{
		{
			ID:            validEvaluatorID1,
			SpaceID:       validSpaceID,
			EvaluatorType: entity.EvaluatorTypePrompt,
			PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
				ID:      validEvaluatorVersionIDs[0],
				Version: validVersion1,
			},
		},
		{
			ID:            validEvaluatorID2,
			SpaceID:       validSpaceID,
			EvaluatorType: entity.EvaluatorTypePrompt,
			PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
				ID:      validEvaluatorVersionIDs[1],
				Version: validVersion2,
			},
		},
	}

	tests := []struct {
		name        string
		req         *evaluatorservice.BatchGetEvaluatorVersionsRequest
		mockSetup   func()
		wantResp    *evaluatorservice.BatchGetEvaluatorVersionsResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "成功 - 正常请求",
			req: &evaluatorservice.BatchGetEvaluatorVersionsRequest{
				EvaluatorVersionIds: validEvaluatorVersionIDs,
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					BatchGetEvaluatorVersion(gomock.Any(), gomock.Any(), validEvaluatorVersionIDs, false).
					Return(validEvaluators, nil)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(validSpaceID, 10),
						SpaceID:       validSpaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
					}).
					Return(nil)

				mockUserInfoService.EXPECT().
					PackUserInfo(gomock.Any(), gomock.Any()).
					Times(2) // 一次用于 Evaluator 列表，一次用于 EvaluatorVersion 列表
			},
			wantResp: &evaluatorservice.BatchGetEvaluatorVersionsResponse{
				Evaluators: []*evaluatordto.Evaluator{
					evaluator.ConvertEvaluatorDO2DTO(validEvaluators[0]),
					evaluator.ConvertEvaluatorDO2DTO(validEvaluators[1]),
				},
			},
			wantErr: false,
		},
		{
			name: "成功 - 评估器列表为空",
			req: &evaluatorservice.BatchGetEvaluatorVersionsRequest{
				EvaluatorVersionIds: validEvaluatorVersionIDs,
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					BatchGetEvaluatorVersion(gomock.Any(), gomock.Any(), validEvaluatorVersionIDs, false).
					Return([]*entity.Evaluator{}, nil)
			},
			wantResp: &evaluatorservice.BatchGetEvaluatorVersionsResponse{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := handler.BatchGetEvaluatorVersions(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.wantResp.Evaluators), len(resp.Evaluators))
				for i, evaluator := range tt.wantResp.Evaluators {
					assert.Equal(t, evaluator.GetEvaluatorID(), resp.Evaluators[i].GetEvaluatorID())
					assert.Equal(t, evaluator.GetCurrentVersion().GetID(), resp.Evaluators[i].GetCurrentVersion().GetID())
					assert.Equal(t, evaluator.GetCurrentVersion().GetVersion(), resp.Evaluators[i].GetCurrentVersion().GetVersion())
				}
			}
		})
	}
}

func TestEvaluatorHandlerImpl_SubmitEvaluatorVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
	mockUserInfoService := userinfomocks.NewMockUserInfoService(ctrl)
	mockAuditClient := auditmocks.NewMockIAuditService(ctrl)

	handler := &EvaluatorHandlerImpl{
		evaluatorService: mockEvaluatorService,
		auth:             mockAuth,
		userInfoService:  mockUserInfoService,
		auditClient:      mockAuditClient,
	}

	// Test data
	validSpaceID := int64(1)
	validEvaluatorID := int64(100)
	validVersion := "1.0.0"
	validDescription := "test description"
	validCID := "test-cid"

	validEvaluator := &entity.Evaluator{
		ID:            validEvaluatorID,
		SpaceID:       validSpaceID,
		EvaluatorType: entity.EvaluatorTypePrompt,
		PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
			ID:          validEvaluatorID,
			Version:     validVersion,
			Description: validDescription,
		},
	}

	tests := []struct {
		name        string
		req         *evaluatorservice.SubmitEvaluatorVersionRequest
		mockSetup   func()
		wantResp    *evaluatorservice.SubmitEvaluatorVersionResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "成功 - 正常请求",
			req: &evaluatorservice.SubmitEvaluatorVersionRequest{
				WorkspaceID: validSpaceID,
				EvaluatorID: validEvaluatorID,
				Version:     validVersion,
				Description: &validDescription,
				Cid:         &validCID,
			},
			mockSetup: func() {
				// Mock GetEvaluator
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), validSpaceID, validEvaluatorID, false).
					Return(validEvaluator, nil).
					Times(1)

				// Mock Authorization
				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(validEvaluator.ID, 10),
						SpaceID:       validSpaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
					}).
					Return(nil).
					Times(1)

				// Mock Audit
				mockAuditClient.EXPECT().
					Audit(gomock.Any(), gomock.Any()).
					Return(audit.AuditRecord{AuditStatus: audit.AuditStatus_Approved}, nil).
					Times(1)

				// Mock SubmitEvaluatorVersion
				mockEvaluatorService.EXPECT().
					SubmitEvaluatorVersion(gomock.Any(), validEvaluator, validVersion, validDescription, validCID).
					Return(validEvaluator, nil).
					Times(1)
			},
			wantResp: &evaluatorservice.SubmitEvaluatorVersionResponse{
				Evaluator: evaluator.ConvertEvaluatorDO2DTO(validEvaluator),
			},
			wantErr: false,
		},
		{
			name: "成功 - 评估器不存在",
			req: &evaluatorservice.SubmitEvaluatorVersionRequest{
				WorkspaceID: validSpaceID,
				EvaluatorID: validEvaluatorID,
				Version:     validVersion,
				Description: &validDescription,
				Cid:         &validCID,
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), validSpaceID, validEvaluatorID, false).
					Return(nil, nil).
					Times(1)
				// Mock Audit
				mockAuditClient.EXPECT().
					Audit(gomock.Any(), gomock.Any()).
					Return(audit.AuditRecord{AuditStatus: audit.AuditStatus_Approved}, nil).
					Times(1)
			},

			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.EvaluatorNotExistCode,
		},
		{
			name: "成功 - 审核拒绝",
			req: &evaluatorservice.SubmitEvaluatorVersionRequest{
				WorkspaceID: validSpaceID,
				EvaluatorID: validEvaluatorID,
				Version:     validVersion,
				Description: &validDescription,
				Cid:         &validCID,
			},
			mockSetup: func() {
				mockAuditClient.EXPECT().
					Audit(gomock.Any(), gomock.Any()).
					Return(audit.AuditRecord{AuditStatus: audit.AuditStatus_Rejected}, nil).
					Times(1)
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.RiskContentDetectedCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := handler.SubmitEvaluatorVersion(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantResp.Evaluator.GetEvaluatorID(), resp.Evaluator.GetEvaluatorID())
				assert.Equal(t, tt.wantResp.Evaluator.GetCurrentVersion().GetVersion(), resp.Evaluator.GetCurrentVersion().GetVersion())
				assert.Equal(t, tt.wantResp.Evaluator.GetCurrentVersion().GetDescription(), resp.Evaluator.GetCurrentVersion().GetDescription())
			}
		})
	}
}

func TestEvaluatorHandlerImpl_ListTemplates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockConfiger := confmocks.NewMockIConfiger(ctrl)

	handler := &EvaluatorHandlerImpl{
		configer: mockConfiger,
	}

	// Test data
	validTemplateType := evaluatordto.TemplateType(1)
	validTemplates := map[string]*evaluatordto.EvaluatorContent{
		"template1": {
			PromptEvaluator: &evaluatordto.PromptEvaluator{
				PromptTemplateKey:  gptr.Of("key1"),
				PromptTemplateName: gptr.Of("name1"),
			},
		},
		"template2": {
			PromptEvaluator: &evaluatordto.PromptEvaluator{
				PromptTemplateKey:  gptr.Of("key2"),
				PromptTemplateName: gptr.Of("name2"),
			},
		},
	}

	tests := []struct {
		name        string
		req         *evaluatorservice.ListTemplatesRequest
		mockSetup   func()
		wantResp    *evaluatorservice.ListTemplatesResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "成功 - 正常请求",
			req: &evaluatorservice.ListTemplatesRequest{
				BuiltinTemplateType: validTemplateType,
			},
			mockSetup: func() {
				mockConfiger.EXPECT().
					GetEvaluatorTemplateConf(gomock.Any()).
					Return(map[string]map[string]*evaluatordto.EvaluatorContent{
						strings.ToLower(evaluatordto.TemplateType_Prompt.String()): validTemplates,
					}).
					Times(1)
			},
			wantResp: &evaluatorservice.ListTemplatesResponse{
				BuiltinTemplateKeys: []*evaluatordto.EvaluatorContent{
					{
						PromptEvaluator: &evaluatordto.PromptEvaluator{
							PromptTemplateKey:  gptr.Of("key1"),
							PromptTemplateName: gptr.Of("name1"),
						},
					},
					{
						PromptEvaluator: &evaluatordto.PromptEvaluator{
							PromptTemplateKey:  gptr.Of("key2"),
							PromptTemplateName: gptr.Of("name2"),
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := handler.ListTemplates(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.wantResp.BuiltinTemplateKeys), len(resp.BuiltinTemplateKeys))
				for i, key := range tt.wantResp.BuiltinTemplateKeys {
					assert.Equal(t, key.GetPromptEvaluator().GetPromptTemplateKey(), resp.BuiltinTemplateKeys[i].GetPromptEvaluator().GetPromptTemplateKey())
					assert.Equal(t, key.GetPromptEvaluator().GetPromptTemplateName(), resp.BuiltinTemplateKeys[i].GetPromptEvaluator().GetPromptTemplateName())
				}
			}
		})
	}
}

func TestEvaluatorHandlerImpl_GetTemplateInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockConfiger := confmocks.NewMockIConfiger(ctrl)

	handler := &EvaluatorHandlerImpl{
		configer: mockConfiger,
	}

	// Test data
	validTemplateType := evaluatordto.TemplateType(1)
	validTemplates := map[string]*evaluatordto.EvaluatorContent{
		"key1": {
			PromptEvaluator: &evaluatordto.PromptEvaluator{
				PromptTemplateKey:  gptr.Of("key1"),
				PromptTemplateName: gptr.Of("name1"),
			},
		},
		"key2": {
			PromptEvaluator: &evaluatordto.PromptEvaluator{
				PromptTemplateKey:  gptr.Of("key2"),
				PromptTemplateName: gptr.Of("name2"),
			},
		},
	}
	validTemplateKey := "key1"
	validTemplate := validTemplates[validTemplateKey]

	tests := []struct {
		name        string
		req         *evaluatorservice.GetTemplateInfoRequest
		mockSetup   func()
		wantResp    *evaluatorservice.GetTemplateInfoResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "成功 - 正常请求",
			req: &evaluatorservice.GetTemplateInfoRequest{
				BuiltinTemplateType: validTemplateType,
				BuiltinTemplateKey:  validTemplateKey,
			},
			mockSetup: func() {
				mockConfiger.EXPECT().
					GetEvaluatorTemplateConf(gomock.Any()).
					Return(map[string]map[string]*evaluatordto.EvaluatorContent{
						strings.ToLower(evaluatordto.TemplateType_Prompt.String()): validTemplates,
					}).
					Times(1)
			},
			wantResp: &evaluatorservice.GetTemplateInfoResponse{
				EvaluatorContent: validTemplate,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := handler.GetTemplateInfo(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantResp.EvaluatorContent.GetPromptEvaluator().GetPromptTemplateKey(), resp.EvaluatorContent.GetPromptEvaluator().GetPromptTemplateKey())
				assert.Equal(t, tt.wantResp.EvaluatorContent.GetPromptEvaluator().GetPromptTemplateName(), resp.EvaluatorContent.GetPromptEvaluator().GetPromptTemplateName())
			}
		})
	}
}

func TestEvaluatorHandlerImpl_RunEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)

	handler := &EvaluatorHandlerImpl{
		auth:             mockAuth,
		evaluatorService: mockEvaluatorService,
	}

	// Test data
	validSpaceID := int64(123)
	validEvaluatorVersionID := int64(456)
	validEvaluatorName := "test-evaluator"
	validExperimentID := int64(789)
	validExperimentRunID := int64(101)
	validItemID := int64(202)
	validTurnID := int64(303)
	validInputData := &evaluatordto.EvaluatorInputData{
		InputFields: map[string]*common.Content{
			"input": {
				Text: gptr.Of("test input"),
			},
		},
	}
	validRecord := &entity.EvaluatorRecord{
		ID:                 1,
		SpaceID:            validSpaceID,
		EvaluatorVersionID: validEvaluatorVersionID,
		EvaluatorInputData: evaluator.ConvertEvaluatorInputDataDTO2DO(validInputData),
	}

	tests := []struct {
		name        string
		req         *evaluatorservice.RunEvaluatorRequest
		mockSetup   func()
		wantResp    *evaluatorservice.RunEvaluatorResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "成功 - 正常请求",
			req: &evaluatorservice.RunEvaluatorRequest{
				WorkspaceID:        validSpaceID,
				EvaluatorVersionID: validEvaluatorVersionID,
				ExperimentID:       &validExperimentID,
				ExperimentRunID:    &validExperimentRunID,
				ItemID:             &validItemID,
				TurnID:             &validTurnID,
				InputData:          validInputData,
			},
			mockSetup: func() {
				// Mock GetEvaluatorVersion
				mockEvaluatorService.EXPECT().
					GetEvaluatorVersion(gomock.Any(), validEvaluatorVersionID, false).
					Return(&entity.Evaluator{
						ID:      validEvaluatorVersionID,
						SpaceID: validSpaceID,
						Name:    validEvaluatorName,
					}, nil).
					Times(1)

				// Mock Authorization
				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(validEvaluatorVersionID, 10),
						SpaceID:       validSpaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Run), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
					}).
					Return(nil).
					Times(1)

				// Mock RunEvaluator
				mockEvaluatorService.EXPECT().
					RunEvaluator(gomock.Any(), gomock.Any()).
					Return(validRecord, nil).
					Times(1)
			},
			wantResp: &evaluatorservice.RunEvaluatorResponse{
				Record: evaluator.ConvertEvaluatorRecordDO2DTO(validRecord),
			},
			wantErr: false,
		},
		{
			name: "错误 - 评估器不存在",
			req: &evaluatorservice.RunEvaluatorRequest{
				WorkspaceID:        validSpaceID,
				EvaluatorVersionID: validEvaluatorVersionID,
				InputData:          validInputData,
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					GetEvaluatorVersion(gomock.Any(), validEvaluatorVersionID, false).
					Return(nil, nil).
					Times(1)
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.EvaluatorNotExistCode,
		},
		{
			name: "错误 - 权限验证失败",
			req: &evaluatorservice.RunEvaluatorRequest{
				WorkspaceID:        validSpaceID,
				EvaluatorVersionID: validEvaluatorVersionID,
				InputData:          validInputData,
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					GetEvaluatorVersion(gomock.Any(), validEvaluatorVersionID, false).
					Return(&entity.Evaluator{
						ID:      validEvaluatorVersionID,
						SpaceID: validSpaceID,
						Name:    validEvaluatorName,
					}, nil).
					Times(1)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(errno.CommonNoPermissionCode)).
					Times(1)
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			resp, err := handler.RunEvaluator(context.Background(), tt.req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantResp.Record.ID, resp.Record.ID)
				assert.Equal(t, tt.wantResp.Record.EvaluatorVersionID, resp.Record.EvaluatorVersionID)
			}
		})
	}
}

func TestEvaluatorHandlerImpl_DebugEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 设置基础 mock 对象
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
	mockBenefitService := benefitmocks.NewMockIBenefitService(ctrl)

	// 创建服务实例
	handler := &EvaluatorHandlerImpl{
		auth:             mockAuth,
		evaluatorService: mockEvaluatorService,
		benefitService:   mockBenefitService,
	}

	// 基础测试数据
	validSpaceID := int64(123)
	validEvaluatorType := evaluatordto.EvaluatorType_Prompt
	validInputData := &evaluatordto.EvaluatorInputData{
		InputFields: map[string]*common.Content{
			"input": {
				ContentType: gptr.Of(common.ContentTypeText),
				Text:        gptr.Of("test input"),
			},
		},
	}
	validOutputData := &evaluatordto.EvaluatorOutputData{
		EvaluatorResult_: &evaluatordto.EvaluatorResult_{
			Reasoning: ptr.Of("test output"),
		},
	}

	tests := []struct {
		name        string
		req         *evaluatorservice.DebugEvaluatorRequest
		mockSetup   func()
		wantResp    *evaluatorservice.DebugEvaluatorResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "成功 - 正常请求",
			req: &evaluatorservice.DebugEvaluatorRequest{
				WorkspaceID:   validSpaceID,
				EvaluatorType: validEvaluatorType,
				InputData:     validInputData,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					PromptEvaluator: &evaluatordto.PromptEvaluator{
						PromptTemplateKey:  gptr.Of("test-template"),
						PromptTemplateName: gptr.Of("Test Template"),
					},
				},
			},
			mockSetup: func() {
				// Mock 权限检查
				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(validSpaceID, 10),
						SpaceID:       validSpaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("debugLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
					}).
					Return(nil)

				// Mock 权益检查
				mockBenefitService.EXPECT().
					CheckEvaluatorBenefit(gomock.Any(), gomock.Any()).
					Return(&benefit.CheckEvaluatorBenefitResult{
						DenyReason: nil,
					}, nil)

				// Mock 调试服务
				mockEvaluatorService.EXPECT().
					DebugEvaluator(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(evaluator.ConvertEvaluatorOutputDataDTO2DO(validOutputData), nil)
			},
			wantResp: &evaluatorservice.DebugEvaluatorResponse{
				EvaluatorOutputData: validOutputData,
			},
			wantErr: false,
		},
		{
			name: "错误 - 权限检查失败",
			req: &evaluatorservice.DebugEvaluatorRequest{
				WorkspaceID:   validSpaceID,
				EvaluatorType: validEvaluatorType,
				InputData:     validInputData,
			},
			mockSetup: func() {
				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
		{
			name: "错误 - 权益检查失败",
			req: &evaluatorservice.DebugEvaluatorRequest{
				WorkspaceID:   validSpaceID,
				EvaluatorType: validEvaluatorType,
				InputData:     validInputData,
			},
			mockSetup: func() {
				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil)

				mockBenefitService.EXPECT().
					CheckEvaluatorBenefit(gomock.Any(), gomock.Any()).
					Return(&benefit.CheckEvaluatorBenefitResult{
						DenyReason: ptr.Of(benefit.DenyReason(1)),
					}, nil)
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.EvaluatorBenefitDenyCode,
		},
		{
			name: "错误 - 权益服务异常",
			req: &evaluatorservice.DebugEvaluatorRequest{
				WorkspaceID:   validSpaceID,
				EvaluatorType: validEvaluatorType,
				InputData:     validInputData,
			},
			mockSetup: func() {
				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil)

				mockBenefitService.EXPECT().
					CheckEvaluatorBenefit(gomock.Any(), gomock.Any()).
					Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := handler.DebugEvaluator(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantResp.EvaluatorOutputData.EvaluatorResult_.Reasoning, resp.EvaluatorOutputData.EvaluatorResult_.Reasoning)
			}
		})
	}
}

func TestEvaluatorHandlerImpl_UpdateEvaluatorRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 设置基础 mock 对象
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
	mockEvaluatorRecordService := mocks.NewMockEvaluatorRecordService(ctrl)
	mockAuditClient := auditmocks.NewMockIAuditService(ctrl)

	// 创建服务实例
	handler := &EvaluatorHandlerImpl{
		auth:                   mockAuth,
		evaluatorService:       mockEvaluatorService,
		evaluatorRecordService: mockEvaluatorRecordService,
		auditClient:            mockAuditClient,
	}

	// 基础测试数据
	validSpaceID := int64(123)
	validRecordID := int64(456)
	validEvaluatorVersionID := int64(789)
	validCorrection := &evaluatordto.Correction{
		Explain: ptr.Of("test correction"),
	}
	validRecord := &entity.EvaluatorRecord{
		ID:                 validRecordID,
		SpaceID:            validSpaceID,
		EvaluatorVersionID: validEvaluatorVersionID,
		EvaluatorInputData: &entity.EvaluatorInputData{
			InputFields: map[string]*entity.Content{
				"input": {
					ContentType: gptr.Of(entity.ContentTypeText),
					Text:        gptr.Of("test input"),
				},
			},
		},
		EvaluatorOutputData: &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{
				Reasoning: "test output",
			},
		},
	}
	validEvaluator := &entity.Evaluator{
		ID:      validEvaluatorVersionID,
		SpaceID: validSpaceID,
		Name:    "test-evaluator",
	}

	tests := []struct {
		name        string
		req         *evaluatorservice.UpdateEvaluatorRecordRequest
		mockSetup   func()
		wantResp    *evaluatorservice.UpdateEvaluatorRecordResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "成功 - 正常请求",
			req: &evaluatorservice.UpdateEvaluatorRecordRequest{
				EvaluatorRecordID: validRecordID,
				Correction:        validCorrection,
			},
			mockSetup: func() {
				// Mock 获取记录
				mockEvaluatorRecordService.EXPECT().
					GetEvaluatorRecord(gomock.Any(), validRecordID, false).
					Return(validRecord, nil)

				// Mock 获取评估器
				mockEvaluatorService.EXPECT().
					GetEvaluatorVersion(gomock.Any(), validRecord.EvaluatorVersionID, false).
					Return(validEvaluator, nil)

				// Mock 权限检查
				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(validEvaluator.ID, 10),
						SpaceID:       validEvaluator.SpaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
					}).
					Return(nil)

				// Mock 审核
				mockAuditClient.EXPECT().
					Audit(gomock.Any(), gomock.Any()).
					Return(audit.AuditRecord{
						AuditStatus: audit.AuditStatus_Approved,
					}, nil)

				// Mock 更新记录
				mockEvaluatorRecordService.EXPECT().
					CorrectEvaluatorRecord(gomock.Any(), validRecord, gomock.Any()).
					Return(nil)
			},
			wantResp: &evaluatorservice.UpdateEvaluatorRecordResponse{
				Record: evaluator.ConvertEvaluatorRecordDO2DTO(validRecord),
			},
			wantErr: false,
		},
		{
			name: "错误 - 记录不存在",
			req: &evaluatorservice.UpdateEvaluatorRecordRequest{
				EvaluatorRecordID: validRecordID,
				Correction:        validCorrection,
			},
			mockSetup: func() {
				mockEvaluatorRecordService.EXPECT().
					GetEvaluatorRecord(gomock.Any(), validRecordID, false).
					Return(nil, nil)
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.EvaluatorRecordNotFoundCode,
		},
		{
			name: "错误 - 评估器不存在",
			req: &evaluatorservice.UpdateEvaluatorRecordRequest{
				EvaluatorRecordID: validRecordID,
				Correction:        validCorrection,
			},
			mockSetup: func() {
				mockEvaluatorRecordService.EXPECT().
					GetEvaluatorRecord(gomock.Any(), validRecordID, false).
					Return(validRecord, nil)

				mockEvaluatorService.EXPECT().
					GetEvaluatorVersion(gomock.Any(), validRecord.EvaluatorVersionID, false).
					Return(nil, nil)
			},
			wantResp: &evaluatorservice.UpdateEvaluatorRecordResponse{},
			wantErr:  false,
		},
		{
			name: "错误 - 权限检查失败",
			req: &evaluatorservice.UpdateEvaluatorRecordRequest{
				EvaluatorRecordID: validRecordID,
				Correction:        validCorrection,
			},
			mockSetup: func() {
				mockEvaluatorRecordService.EXPECT().
					GetEvaluatorRecord(gomock.Any(), validRecordID, false).
					Return(validRecord, nil)

				mockEvaluatorService.EXPECT().
					GetEvaluatorVersion(gomock.Any(), validRecord.EvaluatorVersionID, false).
					Return(validEvaluator, nil)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
		{
			name: "错误 - 审核拒绝",
			req: &evaluatorservice.UpdateEvaluatorRecordRequest{
				EvaluatorRecordID: validRecordID,
				Correction:        validCorrection,
			},
			mockSetup: func() {
				mockEvaluatorRecordService.EXPECT().
					GetEvaluatorRecord(gomock.Any(), validRecordID, false).
					Return(validRecord, nil)

				mockEvaluatorService.EXPECT().
					GetEvaluatorVersion(gomock.Any(), validRecord.EvaluatorVersionID, false).
					Return(validEvaluator, nil)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil)

				mockAuditClient.EXPECT().
					Audit(gomock.Any(), gomock.Any()).
					Return(audit.AuditRecord{
						AuditStatus: audit.AuditStatus_Rejected,
					}, nil)
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.RiskContentDetectedCode,
		},
		{
			name: "错误 - 更新记录失败",
			req: &evaluatorservice.UpdateEvaluatorRecordRequest{
				EvaluatorRecordID: validRecordID,
				Correction:        validCorrection,
			},
			mockSetup: func() {
				mockEvaluatorRecordService.EXPECT().
					GetEvaluatorRecord(gomock.Any(), validRecordID, false).
					Return(validRecord, nil)

				mockEvaluatorService.EXPECT().
					GetEvaluatorVersion(gomock.Any(), validRecord.EvaluatorVersionID, false).
					Return(validEvaluator, nil)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil)

				mockAuditClient.EXPECT().
					Audit(gomock.Any(), gomock.Any()).
					Return(audit.AuditRecord{
						AuditStatus: audit.AuditStatus_Approved,
					}, nil)

				mockEvaluatorRecordService.EXPECT().
					CorrectEvaluatorRecord(gomock.Any(), validRecord, gomock.Any()).
					Return(errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := handler.UpdateEvaluatorRecord(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				if tt.wantResp != nil {
					assert.Equal(t, tt.wantResp.Record.GetID(), resp.Record.GetID())
					assert.Equal(t, tt.wantResp.Record.GetEvaluatorVersionID(), resp.Record.GetEvaluatorVersionID())
				}
			}
		})
	}
}

func TestEvaluatorHandlerImpl_GetEvaluatorRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 设置基础 mock 对象
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
	mockEvaluatorRecordService := mocks.NewMockEvaluatorRecordService(ctrl)
	mockUserInfoService := userinfomocks.NewMockUserInfoService(ctrl)

	// 创建服务实例
	handler := &EvaluatorHandlerImpl{
		auth:                   mockAuth,
		evaluatorService:       mockEvaluatorService,
		evaluatorRecordService: mockEvaluatorRecordService,
		userInfoService:        mockUserInfoService,
	}

	// 基础测试数据
	validSpaceID := int64(123)
	validRecordID := int64(456)
	validEvaluatorVersionID := int64(789)
	validRecord := &entity.EvaluatorRecord{
		ID:                 validRecordID,
		SpaceID:            validSpaceID,
		EvaluatorVersionID: validEvaluatorVersionID,
		EvaluatorInputData: &entity.EvaluatorInputData{
			InputFields: map[string]*entity.Content{
				"input": {
					ContentType: gptr.Of(entity.ContentTypeText),
					Text:        gptr.Of("test input"),
				},
			},
		},
		EvaluatorOutputData: &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{
				Reasoning: "test output",
			},
		},
	}
	validEvaluator := &entity.Evaluator{
		ID:      validEvaluatorVersionID,
		SpaceID: validSpaceID,
		Name:    "test-evaluator",
	}

	tests := []struct {
		name        string
		req         *evaluatorservice.GetEvaluatorRecordRequest
		mockSetup   func()
		wantResp    *evaluatorservice.GetEvaluatorRecordResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "成功 - 正常请求",
			req: &evaluatorservice.GetEvaluatorRecordRequest{
				EvaluatorRecordID: validRecordID,
			},
			mockSetup: func() {
				// Mock 获取记录
				mockEvaluatorRecordService.EXPECT().
					GetEvaluatorRecord(gomock.Any(), validRecordID, false).
					Return(validRecord, nil)

				// Mock 获取评估器
				mockEvaluatorService.EXPECT().
					GetEvaluatorVersion(gomock.Any(), validRecord.EvaluatorVersionID, false).
					Return(validEvaluator, nil)

				// Mock 权限检查
				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(validEvaluator.ID, 10),
						SpaceID:       validEvaluator.SpaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
					}).
					Return(nil)

				// Mock 用户信息打包
				mockUserInfoService.EXPECT().
					PackUserInfo(gomock.Any(), gomock.Any()).
					Return()
			},
			wantResp: &evaluatorservice.GetEvaluatorRecordResponse{
				Record: evaluator.ConvertEvaluatorRecordDO2DTO(validRecord),
			},
			wantErr: false,
		},
		{
			name: "成功 - 记录不存在",
			req: &evaluatorservice.GetEvaluatorRecordRequest{
				EvaluatorRecordID: validRecordID,
			},
			mockSetup: func() {
				mockEvaluatorRecordService.EXPECT().
					GetEvaluatorRecord(gomock.Any(), validRecordID, false).
					Return(nil, nil)
			},
			wantResp: &evaluatorservice.GetEvaluatorRecordResponse{},
			wantErr:  false,
		},
		{
			name: "错误 - 评估器不存在",
			req: &evaluatorservice.GetEvaluatorRecordRequest{
				EvaluatorRecordID: validRecordID,
			},
			mockSetup: func() {
				mockEvaluatorRecordService.EXPECT().
					GetEvaluatorRecord(gomock.Any(), validRecordID, false).
					Return(validRecord, nil)

				mockEvaluatorService.EXPECT().
					GetEvaluatorVersion(gomock.Any(), validRecord.EvaluatorVersionID, false).
					Return(nil, nil)
			},
			wantResp: &evaluatorservice.GetEvaluatorRecordResponse{},
			wantErr:  false,
		},
		{
			name: "错误 - 权限检查失败",
			req: &evaluatorservice.GetEvaluatorRecordRequest{
				EvaluatorRecordID: validRecordID,
			},
			mockSetup: func() {
				mockEvaluatorRecordService.EXPECT().
					GetEvaluatorRecord(gomock.Any(), validRecordID, false).
					Return(validRecord, nil)

				mockEvaluatorService.EXPECT().
					GetEvaluatorVersion(gomock.Any(), validRecord.EvaluatorVersionID, false).
					Return(validEvaluator, nil)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := handler.GetEvaluatorRecord(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				if tt.wantResp != nil {
					assert.Equal(t, tt.wantResp.Record.GetID(), resp.Record.GetID())
					assert.Equal(t, tt.wantResp.Record.GetEvaluatorVersionID(), resp.Record.GetEvaluatorVersionID())
				}
			}
		})
	}
}

func TestEvaluatorHandlerImpl_BatchGetEvaluatorRecords(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 设置基础 mock 对象
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvaluatorRecordService := mocks.NewMockEvaluatorRecordService(ctrl)

	// 创建服务实例
	handler := &EvaluatorHandlerImpl{
		auth:                   mockAuth,
		evaluatorRecordService: mockEvaluatorRecordService,
	}

	// 基础测试数据
	validSpaceID := int64(123)
	validRecordIDs := []int64{456, 789}
	validRecords := []*entity.EvaluatorRecord{
		{
			ID:                 456,
			SpaceID:            validSpaceID,
			EvaluatorVersionID: 789,
		},
		{
			ID:                 789,
			SpaceID:            validSpaceID,
			EvaluatorVersionID: 101,
		},
	}

	tests := []struct {
		name        string
		req         *evaluatorservice.BatchGetEvaluatorRecordsRequest
		mockSetup   func()
		wantResp    *evaluatorservice.BatchGetEvaluatorRecordsResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "成功 - 正常请求",
			req: &evaluatorservice.BatchGetEvaluatorRecordsRequest{
				EvaluatorRecordIds: validRecordIDs,
			},
			mockSetup: func() {
				mockEvaluatorRecordService.EXPECT().
					BatchGetEvaluatorRecord(gomock.Any(), validRecordIDs, false).
					Return(validRecords, nil)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(validRecords[0].SpaceID, 10),
						SpaceID:       validRecords[0].SpaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
					}).
					Return(nil)
			},
			wantResp: &evaluatorservice.BatchGetEvaluatorRecordsResponse{
				Records: []*evaluatordto.EvaluatorRecord{
					evaluator.ConvertEvaluatorRecordDO2DTO(validRecords[0]),
					evaluator.ConvertEvaluatorRecordDO2DTO(validRecords[1]),
				},
			},
			wantErr: false,
		},
		{
			name: "成功 - 无记录",
			req: &evaluatorservice.BatchGetEvaluatorRecordsRequest{
				EvaluatorRecordIds: validRecordIDs,
			},
			mockSetup: func() {
				mockEvaluatorRecordService.EXPECT().
					BatchGetEvaluatorRecord(gomock.Any(), validRecordIDs, false).
					Return([]*entity.EvaluatorRecord{}, nil)
			},
			wantResp: &evaluatorservice.BatchGetEvaluatorRecordsResponse{},
			wantErr:  false,
		},
		{
			name: "错误 - 权限检查失败",
			req: &evaluatorservice.BatchGetEvaluatorRecordsRequest{
				EvaluatorRecordIds: validRecordIDs,
			},
			mockSetup: func() {
				mockEvaluatorRecordService.EXPECT().
					BatchGetEvaluatorRecord(gomock.Any(), validRecordIDs, false).
					Return(validRecords, nil)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := handler.BatchGetEvaluatorRecords(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.wantResp.Records), len(resp.Records))
				for i, record := range tt.wantResp.Records {
					assert.Equal(t, record.GetID(), resp.Records[i].GetID())
					assert.Equal(t, record.GetEvaluatorVersionID(), resp.Records[i].GetEvaluatorVersionID())
				}
			}
		})
	}
}

func TestEvaluatorHandlerImpl_GetDefaultPromptEvaluatorTools(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 设置基础 mock 对象
	mockConfiger := confmocks.NewMockIConfiger(ctrl)

	// 创建服务实例
	handler := &EvaluatorHandlerImpl{
		configer: mockConfiger,
	}

	// 基础测试数据
	validTool := &evaluatordto.Tool{
		Type: evaluatordto.ToolType_Function,
		Function: &evaluatordto.Function{
			Name:        "test-tool",
			Description: ptr.Of("test-tool-description"),
		},
	}

	tests := []struct {
		name        string
		req         *evaluatorservice.GetDefaultPromptEvaluatorToolsRequest
		mockSetup   func()
		wantResp    *evaluatorservice.GetDefaultPromptEvaluatorToolsResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "成功 - 正常请求",
			req:  &evaluatorservice.GetDefaultPromptEvaluatorToolsRequest{},
			mockSetup: func() {
				mockConfiger.EXPECT().
					GetEvaluatorToolConf(gomock.Any()).
					Return(map[string]*evaluatordto.Tool{
						consts.DefaultEvaluatorToolKey: validTool,
					})
			},
			wantResp: &evaluatorservice.GetDefaultPromptEvaluatorToolsResponse{
				Tools: []*evaluatordto.Tool{validTool},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := handler.GetDefaultPromptEvaluatorTools(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.wantResp.Tools), len(resp.Tools))
			}
		})
	}
}

func TestEvaluatorHandlerImpl_CheckEvaluatorName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 设置基础 mock 对象
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)

	// 创建服务实例
	handler := &EvaluatorHandlerImpl{
		auth:             mockAuth,
		evaluatorService: mockEvaluatorService,
	}

	// 基础测试数据
	validSpaceID := int64(123)
	validEvaluatorID := int64(456)
	validName := "test-evaluator"

	tests := []struct {
		name        string
		req         *evaluatorservice.CheckEvaluatorNameRequest
		mockSetup   func()
		wantResp    *evaluatorservice.CheckEvaluatorNameResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "成功 - 名称可用",
			req: &evaluatorservice.CheckEvaluatorNameRequest{
				WorkspaceID: validSpaceID,
				EvaluatorID: &validEvaluatorID,
				Name:        validName,
			},
			mockSetup: func() {
				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(validSpaceID, 10),
						SpaceID:       validSpaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
					}).
					Return(nil)

				mockEvaluatorService.EXPECT().
					CheckNameExist(gomock.Any(), validSpaceID, validEvaluatorID, validName).
					Return(false, nil)
			},
			wantResp: &evaluatorservice.CheckEvaluatorNameResponse{
				Pass: gptr.Of(true),
			},
			wantErr: false,
		},
		{
			name: "成功 - 名称已存在",
			req: &evaluatorservice.CheckEvaluatorNameRequest{
				WorkspaceID: validSpaceID,
				EvaluatorID: &validEvaluatorID,
				Name:        validName,
			},
			mockSetup: func() {
				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil)

				mockEvaluatorService.EXPECT().
					CheckNameExist(gomock.Any(), validSpaceID, validEvaluatorID, validName).
					Return(true, nil)
			},
			wantResp: &evaluatorservice.CheckEvaluatorNameResponse{
				Pass:    gptr.Of(false),
				Message: gptr.Of(fmt.Sprintf("evaluator_version name %s already exists", validName)),
			},
			wantErr: false,
		},
		{
			name: "错误 - 权限检查失败",
			req: &evaluatorservice.CheckEvaluatorNameRequest{
				WorkspaceID: validSpaceID,
				EvaluatorID: &validEvaluatorID,
				Name:        validName,
			},
			mockSetup: func() {
				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
		{
			name: "错误 - 服务调用失败",
			req: &evaluatorservice.CheckEvaluatorNameRequest{
				WorkspaceID: validSpaceID,
				EvaluatorID: &validEvaluatorID,
				Name:        validName,
			},
			mockSetup: func() {
				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil)

				mockEvaluatorService.EXPECT().
					CheckNameExist(gomock.Any(), validSpaceID, validEvaluatorID, validName).
					Return(false, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := handler.CheckEvaluatorName(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantResp.Pass, resp.Pass)
				if tt.wantResp.Message != nil {
					assert.Equal(t, tt.wantResp.Message, resp.Message)
				}
			}
		})
	}
}
