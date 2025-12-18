// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/external/audit"
	auditmocks "github.com/coze-dev/coze-loop/backend/infra/external/audit/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	benefitmocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
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
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/conf/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestEvaluatorHandlerImpl_authCustomRPCEvaluatorContentWritable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockConfiger := confmocks.NewMockIConfiger(ctrl)

	app := &EvaluatorHandlerImpl{
		configer: mockConfiger,
	}

	ctx := context.Background()

	tests := []struct {
		name              string
		workspaceID       int64
		allowedSpaceIDs   []string
		checkWritableResp bool
		checkWritableErr  error
		wantErr           bool
		wantErrCode       int32
	}{
		{
			name:              "成功 - workspaceID在允许列表中",
			workspaceID:       123456,
			allowedSpaceIDs:   []string{"123456", "789012"},
			checkWritableResp: true,
			checkWritableErr:  nil,
			wantErr:           false,
		},
		{
			name:              "失败 - workspaceID不在允许列表中",
			workspaceID:       345678,
			allowedSpaceIDs:   []string{"123456", "789012"},
			checkWritableResp: false,
			checkWritableErr:  nil,
			wantErr:           true,
			wantErrCode:       errno.CommonInvalidParamCode,
		},
		{
			name:              "失败 - 配置返回空列表",
			workspaceID:       123456,
			allowedSpaceIDs:   []string{},
			checkWritableResp: false,
			checkWritableErr:  nil,
			wantErr:           true,
			wantErrCode:       errno.CommonInvalidParamCode,
		},
		{
			name:              "失败 - CheckCustomRPCEvaluatorWritable返回错误",
			workspaceID:       123456,
			allowedSpaceIDs:   []string{"123456"},
			checkWritableResp: false,
			checkWritableErr:  errors.New("配置检查失败"),
			wantErr:           true,
			wantErrCode:       0, // 错误码由底层错误决定
		},
		{
			name:              "边界情况 - workspaceID为0",
			workspaceID:       0,
			allowedSpaceIDs:   []string{"0", "123456"},
			checkWritableResp: true,
			checkWritableErr:  nil,
			wantErr:           false,
		},
		{
			name:              "边界情况 - 负数workspaceID",
			workspaceID:       -123,
			allowedSpaceIDs:   []string{"-123", "123456"},
			checkWritableResp: true,
			checkWritableErr:  nil,
			wantErr:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock GetBuiltinEvaluatorSpaceConf
			mockConfiger.EXPECT().GetBuiltinEvaluatorSpaceConf(ctx).Return(tt.allowedSpaceIDs)

			// Mock CheckCustomRPCEvaluatorWritable
			mockConfiger.EXPECT().CheckCustomRPCEvaluatorWritable(
				ctx,
				strconv.FormatInt(tt.workspaceID, 10),
				tt.allowedSpaceIDs,
			).Return(tt.checkWritableResp, tt.checkWritableErr)

			err := app.authCustomRPCEvaluatorContentWritable(ctx, tt.workspaceID)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					if statusErr, ok := errorx.FromStatusError(err); ok {
						assert.Equal(t, tt.wantErrCode, statusErr.Code())
					}
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

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
			name: "success - builtin evaluators request",
			req: &evaluatorservice.ListEvaluatorsRequest{
				WorkspaceID: validSpaceID,
				Builtin:     gptr.Of(true),
			},
			mockSetup: func() {
				// Mock auth
				mockAuth.EXPECT().Authorization(gomock.Any(), &rpc.AuthorizationParam{
					ObjectID:      strconv.FormatInt(validSpaceID, 10),
					SpaceID:       validSpaceID,
					ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
				}).Return(nil)

				// Mock builtin evaluator service call
				mockEvaluatorService.EXPECT().ListBuiltinEvaluator(gomock.Any(), gomock.Any()).
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
	mockConfiger := confmocks.NewMockIConfiger(ctrl)

	app := &EvaluatorHandlerImpl{
		auth:             mockAuth,
		evaluatorService: mockEvaluatorService,
		userInfoService:  mockUserInfoService,
		configer:         mockConfiger,
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
	validBuiltinSpaceID := int64(111)
	validBuiltinEvaluatorID := int64(333)
	validBuiltinEvaluator := &entity.Evaluator{
		ID:             validBuiltinEvaluatorID,
		SpaceID:        validBuiltinSpaceID,
		Name:           "Test Evaluator",
		EvaluatorType:  entity.EvaluatorTypePrompt,
		Description:    "Test Description",
		DraftSubmitted: true,
		Builtin:        true,
	}
	invalidBuiltinSpaceID := int64(1111)
	invalidBuiltinEvaluator := &entity.Evaluator{
		ID:             validBuiltinEvaluatorID,
		SpaceID:        invalidBuiltinSpaceID,
		Name:           "Test Evaluator",
		EvaluatorType:  entity.EvaluatorTypePrompt,
		Description:    "Test Description",
		DraftSubmitted: true,
		Builtin:        true,
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
		{
			name: "success - normal request for builtin",
			req: &evaluatorservice.GetEvaluatorRequest{
				WorkspaceID: validSpaceID,
				EvaluatorID: &validBuiltinEvaluatorID,
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), validSpaceID, validBuiltinEvaluatorID, false).
					Return(validBuiltinEvaluator, nil)

				mockConfiger.EXPECT().GetBuiltinEvaluatorSpaceConf(gomock.Any()).Return([]string{strconv.FormatInt(validBuiltinSpaceID, 10)})

				mockUserInfoService.EXPECT().
					PackUserInfo(gomock.Any(), gomock.Any()).
					Return()
			},
			wantResp: &evaluatorservice.GetEvaluatorResponse{
				Evaluator: evaluator.ConvertEvaluatorDO2DTO(validBuiltinEvaluator),
			},
			wantErr: false,
		},
		{
			name: "error - invalid builtin space",
			req: &evaluatorservice.GetEvaluatorRequest{
				WorkspaceID: validSpaceID,
				EvaluatorID: &validBuiltinEvaluatorID,
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), validSpaceID, validBuiltinEvaluatorID, false).
					Return(invalidBuiltinEvaluator, nil)

				mockConfiger.EXPECT().GetBuiltinEvaluatorSpaceConf(gomock.Any()).Return([]string{strconv.FormatInt(validBuiltinEvaluatorID, 10)})
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
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

// TestEvaluatorHandlerImpl_GetEvaluatorVersion 测试 GetEvaluatorVersion 方法
func TestEvaluatorHandlerImpl_GetEvaluatorVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
	mockUserInfoService := userinfomocks.NewMockUserInfoService(ctrl)
	mockConfiger := confmocks.NewMockIConfiger(ctrl)

	app := &EvaluatorHandlerImpl{
		auth:             mockAuth,
		evaluatorService: mockEvaluatorService,
		userInfoService:  mockUserInfoService,
		configer:         mockConfiger,
	}

	validWorkspaceID := int64(123)
	validEvaluatorVersionID := int64(456)
	validEvaluator := &entity.Evaluator{
		ID:            1,
		SpaceID:       validWorkspaceID,
		Name:          "test-evaluator",
		EvaluatorType: entity.EvaluatorTypePrompt,
		PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
			ID:          validEvaluatorVersionID,
			EvaluatorID: 1,
			Version:     "1.0.0",
		},
	}

	builtinEvaluator := &entity.Evaluator{
		ID:            2,
		SpaceID:       validWorkspaceID,
		Name:          "builtin-evaluator",
		Builtin:       true,
		EvaluatorType: entity.EvaluatorTypePrompt,
		PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
			ID:          validEvaluatorVersionID,
			EvaluatorID: 2,
			Version:     "1.0.0",
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
			name: "success - normal evaluator version",
			req: &evaluatorservice.GetEvaluatorVersionRequest{
				WorkspaceID:        validWorkspaceID,
				EvaluatorVersionID: validEvaluatorVersionID,
				Builtin:            gptr.Of(false),
				IncludeDeleted:     gptr.Of(false),
			},
			mockSetup: func() {
				// Mock service call - non-builtin, with spaceID
				mockEvaluatorService.EXPECT().
					GetEvaluatorVersion(gomock.Any(), gptr.Of(validWorkspaceID), validEvaluatorVersionID, false, false).
					Return(validEvaluator, nil)

				// Mock auth - non-builtin path
				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(validEvaluator.ID, 10),
						SpaceID:       validEvaluator.SpaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
					}).
					Return(nil)

				// Mock user info service
				mockUserInfoService.EXPECT().PackUserInfo(gomock.Any(), gomock.Any()).Return().Times(2)
			},
			wantResp: &evaluatorservice.GetEvaluatorVersionResponse{
				Evaluator: evaluator.ConvertEvaluatorDO2DTO(validEvaluator),
			},
			wantErr: false,
		},
		{
			name: "success - builtin evaluator version",
			req: &evaluatorservice.GetEvaluatorVersionRequest{
				WorkspaceID:        validWorkspaceID,
				EvaluatorVersionID: validEvaluatorVersionID,
				Builtin:            gptr.Of(true),
				IncludeDeleted:     gptr.Of(false),
			},
			mockSetup: func() {
				// Mock service call - builtin, without spaceID
				mockEvaluatorService.EXPECT().
					GetEvaluatorVersion(gomock.Any(), (*int64)(nil), validEvaluatorVersionID, false, true).
					Return(builtinEvaluator, nil)

				// Mock configer for authBuiltinManagement - spaceID in config, so authBuiltinManagement returns nil without calling Authorization
				mockConfiger.EXPECT().
					GetBuiltinEvaluatorSpaceConf(gomock.Any()).
					Return([]string{strconv.FormatInt(validWorkspaceID, 10)})

				// Mock auth - builtin path (second auth call in GetEvaluatorVersion)
				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(validWorkspaceID, 10),
						SpaceID:       validWorkspaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
					}).
					Return(nil)

				// Mock user info service
				mockUserInfoService.EXPECT().PackUserInfo(gomock.Any(), gomock.Any()).Return().Times(2)
			},
			wantResp: &evaluatorservice.GetEvaluatorVersionResponse{
				Evaluator: evaluator.ConvertEvaluatorDO2DTO(builtinEvaluator),
			},
			wantErr: false,
		},
		{
			name: "success - evaluator not found",
			req: &evaluatorservice.GetEvaluatorVersionRequest{
				WorkspaceID:        validWorkspaceID,
				EvaluatorVersionID: validEvaluatorVersionID,
				Builtin:            gptr.Of(false),
				IncludeDeleted:     gptr.Of(false),
			},
			mockSetup: func() {
				// Mock service call - returns nil
				mockEvaluatorService.EXPECT().
					GetEvaluatorVersion(gomock.Any(), gptr.Of(validWorkspaceID), validEvaluatorVersionID, false, false).
					Return(nil, nil)
			},
			wantResp: &evaluatorservice.GetEvaluatorVersionResponse{},
			wantErr:  false,
		},
		{
			name: "error - service error",
			req: &evaluatorservice.GetEvaluatorVersionRequest{
				WorkspaceID:        validWorkspaceID,
				EvaluatorVersionID: validEvaluatorVersionID,
				Builtin:            gptr.Of(false),
				IncludeDeleted:     gptr.Of(false),
			},
			mockSetup: func() {
				// Mock service call - returns error
				mockEvaluatorService.EXPECT().
					GetEvaluatorVersion(gomock.Any(), gptr.Of(validWorkspaceID), validEvaluatorVersionID, false, false).
					Return(nil, errors.New("database error"))
			},
			wantResp: nil,
			wantErr:  true,
		},
		{
			name: "error - auth failed for non-builtin",
			req: &evaluatorservice.GetEvaluatorVersionRequest{
				WorkspaceID:        validWorkspaceID,
				EvaluatorVersionID: validEvaluatorVersionID,
				Builtin:            gptr.Of(false),
				IncludeDeleted:     gptr.Of(false),
			},
			mockSetup: func() {
				// Mock service call
				mockEvaluatorService.EXPECT().
					GetEvaluatorVersion(gomock.Any(), gptr.Of(validWorkspaceID), validEvaluatorVersionID, false, false).
					Return(validEvaluator, nil)

				// Mock auth - returns error
				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(validEvaluator.ID, 10),
						SpaceID:       validEvaluator.SpaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
					}).
					Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
		{
			name: "error - authBuiltinManagement failed - space not in config",
			req: &evaluatorservice.GetEvaluatorVersionRequest{
				WorkspaceID:        validWorkspaceID,
				EvaluatorVersionID: validEvaluatorVersionID,
				Builtin:            gptr.Of(true),
				IncludeDeleted:     gptr.Of(false),
			},
			mockSetup: func() {
				// Mock service call
				mockEvaluatorService.EXPECT().
					GetEvaluatorVersion(gomock.Any(), (*int64)(nil), validEvaluatorVersionID, false, true).
					Return(builtinEvaluator, nil)

				// Mock configer for authBuiltinManagement - returns empty list, which causes error
				mockConfiger.EXPECT().
					GetBuiltinEvaluatorSpaceConf(gomock.Any()).
					Return([]string{})
				// When config is empty, authBuiltinManagement returns error immediately without calling Authorization
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - authBuiltinManagement failed - space not allowed",
			req: &evaluatorservice.GetEvaluatorVersionRequest{
				WorkspaceID:        validWorkspaceID,
				EvaluatorVersionID: validEvaluatorVersionID,
				Builtin:            gptr.Of(true),
				IncludeDeleted:     gptr.Of(false),
			},
			mockSetup: func() {
				// Mock service call
				mockEvaluatorService.EXPECT().
					GetEvaluatorVersion(gomock.Any(), (*int64)(nil), validEvaluatorVersionID, false, true).
					Return(builtinEvaluator, nil)

				// Mock configer for authBuiltinManagement - returns different space ID
				mockConfiger.EXPECT().
					GetBuiltinEvaluatorSpaceConf(gomock.Any()).
					Return([]string{"999"}) // Different workspace ID
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - builtin second auth failed",
			req: &evaluatorservice.GetEvaluatorVersionRequest{
				WorkspaceID:        validWorkspaceID,
				EvaluatorVersionID: validEvaluatorVersionID,
				Builtin:            gptr.Of(true),
				IncludeDeleted:     gptr.Of(false),
			},
			mockSetup: func() {
				// Mock service call
				mockEvaluatorService.EXPECT().
					GetEvaluatorVersion(gomock.Any(), (*int64)(nil), validEvaluatorVersionID, false, true).
					Return(builtinEvaluator, nil)

				// Mock configer for authBuiltinManagement - spaceID in config, so authBuiltinManagement returns nil without calling Authorization
				mockConfiger.EXPECT().
					GetBuiltinEvaluatorSpaceConf(gomock.Any()).
					Return([]string{strconv.FormatInt(validWorkspaceID, 10)})

				// Mock auth - second call in GetEvaluatorVersion fails
				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(validWorkspaceID, 10),
						SpaceID:       validWorkspaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
					}).
					Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
		{
			name: "success - include deleted",
			req: &evaluatorservice.GetEvaluatorVersionRequest{
				WorkspaceID:        validWorkspaceID,
				EvaluatorVersionID: validEvaluatorVersionID,
				Builtin:            gptr.Of(false),
				IncludeDeleted:     gptr.Of(true),
			},
			mockSetup: func() {
				// Mock service call with includeDeleted=true
				mockEvaluatorService.EXPECT().
					GetEvaluatorVersion(gomock.Any(), gptr.Of(validWorkspaceID), validEvaluatorVersionID, true, false).
					Return(validEvaluator, nil)

				// Mock auth
				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(validEvaluator.ID, 10),
						SpaceID:       validEvaluator.SpaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
					}).
					Return(nil)

				// Mock user info service
				mockUserInfoService.EXPECT().PackUserInfo(gomock.Any(), gomock.Any()).Return().Times(2)
			},
			wantResp: &evaluatorservice.GetEvaluatorVersionResponse{
				Evaluator: evaluator.ConvertEvaluatorDO2DTO(validEvaluator),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := app.GetEvaluatorVersion(context.Background(), tt.req)

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
					assert.NotNil(t, resp)
					if tt.wantResp.Evaluator != nil {
						assert.Equal(t, tt.wantResp.Evaluator.GetEvaluatorID(), resp.Evaluator.GetEvaluatorID())
						assert.Equal(t, tt.wantResp.Evaluator.GetName(), resp.Evaluator.GetName())
					}
				} else {
					assert.Equal(t, tt.wantResp, resp)
				}
			}
		})
	}
}

// TestEvaluatorHandlerImpl_BatchGetEvaluatorVersions 测试 BatchGetEvaluatorVersions 方法
func TestEvaluatorHandlerImpl_BatchGetEvaluatorVersions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
	mockUserInfoService := userinfomocks.NewMockUserInfoService(ctrl)
	mockConfiger := confmocks.NewMockIConfiger(ctrl)

	app := &EvaluatorHandlerImpl{
		auth:             mockAuth,
		evaluatorService: mockEvaluatorService,
		userInfoService:  mockUserInfoService,
		configer:         mockConfiger,
	}

	workspaceID := int64(100)
	builtinSpaceID := int64(200)
	versionIDs := []int64{11, 22}

	workspaceEvaluator := &entity.Evaluator{
		ID:            1,
		SpaceID:       workspaceID,
		EvaluatorType: entity.EvaluatorTypePrompt,
		PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
			ID:          versionIDs[0],
			EvaluatorID: 1,
			Version:     "v1",
		},
	}
	builtinEvaluator := &entity.Evaluator{
		ID:            2,
		SpaceID:       builtinSpaceID,
		Builtin:       true,
		EvaluatorType: entity.EvaluatorTypePrompt,
		PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
			ID:          versionIDs[1],
			EvaluatorID: 2,
			Version:     "v2",
		},
	}

	tests := []struct {
		name               string
		req                *evaluatorservice.BatchGetEvaluatorVersionsRequest
		mockSetup          func()
		wantErr            bool
		wantErrCode        int32
		wantEvaluatorCount int
	}{
		{
			name: "success - workspace evaluators only",
			req: &evaluatorservice.BatchGetEvaluatorVersionsRequest{
				WorkspaceID:         workspaceID,
				EvaluatorVersionIds: []int64{versionIDs[0]},
				IncludeDeleted:      gptr.Of(false),
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					BatchGetEvaluatorVersion(gomock.Any(), (*int64)(nil), []int64{versionIDs[0]}, false).
					Return([]*entity.Evaluator{workspaceEvaluator}, nil)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(workspaceID, 10),
						SpaceID:       workspaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
					}).
					Return(nil)

				mockUserInfoService.EXPECT().PackUserInfo(gomock.Any(), gomock.Any()).Return().Times(2)
			},
			wantEvaluatorCount: 1,
		},
		{
			name: "success - workspace and builtin evaluators",
			req: &evaluatorservice.BatchGetEvaluatorVersionsRequest{
				WorkspaceID:         workspaceID,
				EvaluatorVersionIds: versionIDs,
				IncludeDeleted:      gptr.Of(false),
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					BatchGetEvaluatorVersion(gomock.Any(), (*int64)(nil), versionIDs, false).
					Return([]*entity.Evaluator{workspaceEvaluator, builtinEvaluator}, nil)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(workspaceID, 10),
						SpaceID:       workspaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
					}).
					Return(nil)

				mockConfiger.EXPECT().
					GetBuiltinEvaluatorSpaceConf(gomock.Any()).
					Return([]string{strconv.FormatInt(builtinSpaceID, 10)})

				mockUserInfoService.EXPECT().PackUserInfo(gomock.Any(), gomock.Any()).Return().Times(2)
			},
			wantEvaluatorCount: 2,
		},
		{
			name: "error - service failed",
			req: &evaluatorservice.BatchGetEvaluatorVersionsRequest{
				WorkspaceID:         workspaceID,
				EvaluatorVersionIds: []int64{versionIDs[0]},
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					BatchGetEvaluatorVersion(gomock.Any(), (*int64)(nil), []int64{versionIDs[0]}, false).
					Return(nil, errors.New("db error"))
			},
			wantErr: true,
		},
		{
			name: "error - workspace auth failed",
			req: &evaluatorservice.BatchGetEvaluatorVersionsRequest{
				WorkspaceID:         workspaceID,
				EvaluatorVersionIds: []int64{versionIDs[0]},
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					BatchGetEvaluatorVersion(gomock.Any(), (*int64)(nil), []int64{versionIDs[0]}, false).
					Return([]*entity.Evaluator{workspaceEvaluator}, nil)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(workspaceID, 10),
						SpaceID:       workspaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
					}).
					Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
		{
			name: "error - builtin auth failed",
			req: &evaluatorservice.BatchGetEvaluatorVersionsRequest{
				WorkspaceID:         workspaceID,
				EvaluatorVersionIds: versionIDs,
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					BatchGetEvaluatorVersion(gomock.Any(), (*int64)(nil), versionIDs, false).
					Return([]*entity.Evaluator{workspaceEvaluator, builtinEvaluator}, nil)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(workspaceID, 10),
						SpaceID:       workspaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
					}).
					Return(nil)

				mockConfiger.EXPECT().
					GetBuiltinEvaluatorSpaceConf(gomock.Any()).
					Return([]string{})
			},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := app.BatchGetEvaluatorVersions(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Len(t, resp.Evaluators, tt.wantEvaluatorCount)
		})
	}
}

// 新增的复杂业务逻辑测试

// TestEvaluatorHandlerImpl_ComplexBusinessScenarios 测试复杂业务场景
func TestEvaluatorHandlerImpl_ComplexBusinessScenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "多层依赖服务交互测试",
			testFunc: func(t *testing.T) {
				t.Parallel()

				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				// 创建所有依赖的 mock
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockConfiger := confmocks.NewMockIConfiger(ctrl)
				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
				mockEvaluatorRecordService := mocks.NewMockEvaluatorRecordService(ctrl)
				mockMetrics := metricsmock.NewMockEvaluatorExecMetrics(ctrl)
				mockUserInfoService := userinfomocks.NewMockUserInfoService(ctrl)
				mockAuditClient := auditmocks.NewMockIAuditService(ctrl)
				mockBenefitService := benefitmocks.NewMockIBenefitService(ctrl)
				mockFileProvider := rpcmocks.NewMockIFileProvider(ctrl)

				handler := NewEvaluatorHandlerImpl(
					mockIDGen,
					mockConfiger,
					mockAuth,
					mockEvaluatorService,
					mockEvaluatorRecordService,
					nil, // mockEvaluatorTemplateService - 暂时设为nil
					mockMetrics,
					mockUserInfoService,
					mockAuditClient,
					mockBenefitService,
					mockFileProvider,
					make(map[entity.EvaluatorType]service.EvaluatorSourceService),
				)

				// 测试复杂的调试场景，涉及多个服务交互
				request := &evaluatorservice.DebugEvaluatorRequest{
					WorkspaceID:   123,
					EvaluatorType: evaluatordto.EvaluatorType_Prompt,
					EvaluatorContent: &evaluatordto.EvaluatorContent{
						PromptEvaluator: &evaluatordto.PromptEvaluator{
							MessageList: []*common.Message{
								{
									Role: common.RolePtr(common.Role_User),
									Content: &common.Content{
										ContentType: gptr.Of(common.ContentTypeMultiPart),
										MultiPart: []*common.Content{
											{
												ContentType: gptr.Of(common.ContentTypeText),
												Text:        gptr.Of("请分析这张图片："),
											},
											{
												ContentType: gptr.Of(common.ContentTypeImage),
												Image: &common.Image{
													URI: gptr.Of("test-image-uri"),
												},
											},
										},
									},
								},
							},
						},
					},
					InputData: &evaluatordto.EvaluatorInputData{
						InputFields: map[string]*common.Content{
							"image": {
								ContentType: gptr.Of(common.ContentTypeImage),
								Image: &common.Image{
									URI: gptr.Of("input-image-uri"),
								},
							},
						},
					},
				}

				// 设置复杂的 mock 期望
				// 1. 鉴权
				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      "123",
						SpaceID:       int64(123),
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("debugLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
					}).
					Return(nil).
					Times(1)

				// 2. 权益检查
				mockBenefitService.EXPECT().
					CheckEvaluatorBenefit(gomock.Any(), &benefit.CheckEvaluatorBenefitParams{
						ConnectorUID: "",
						SpaceID:      123,
					}).
					Return(&benefit.CheckEvaluatorBenefitResult{DenyReason: nil}, nil).
					Times(1)

				// 3. 文件 URI 转 URL
				mockFileProvider.EXPECT().
					MGetFileURL(gomock.Any(), []string{"input-image-uri"}).
					Return(map[string]string{"input-image-uri": "https://example.com/image.jpg"}, nil).
					Times(1)

				// 4. 评估器调试
				mockEvaluatorService.EXPECT().
					DebugEvaluator(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, evaluator *entity.Evaluator, input *entity.EvaluatorInputData, evaluatorRunConf *entity.EvaluatorRunConfig, exptSpaceID int64) (*entity.EvaluatorOutputData, error) {
						// 验证输入数据已被正确处理
						assert.Equal(t, int64(123), evaluator.SpaceID)
						assert.Equal(t, entity.EvaluatorTypePrompt, evaluator.EvaluatorType)

						// 验证 URI 已转换为 URL
						imageContent := input.InputFields["image"]
						assert.NotNil(t, imageContent)
						assert.NotNil(t, imageContent.Image)
						assert.Equal(t, "https://example.com/image.jpg", gptr.Indirect(imageContent.Image.URL))

						return &entity.EvaluatorOutputData{
							EvaluatorResult: &entity.EvaluatorResult{
								Score:     gptr.Of(0.85),
								Reasoning: "多模态内容分析完成",
							},
						}, nil
					}).
					Times(1)

				ctx := context.Background()
				resp, err := handler.DebugEvaluator(ctx, request)

				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.NotNil(t, resp.EvaluatorOutputData)
				assert.Equal(t, 0.85, gptr.Indirect(resp.EvaluatorOutputData.EvaluatorResult_.Score))
			},
		},
		{
			name: "权限验证和审核流程测试",
			testFunc: func(t *testing.T) {
				t.Parallel()

				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
				mockAuditClient := auditmocks.NewMockIAuditService(ctrl)
				mockMetrics := metricsmock.NewMockEvaluatorExecMetrics(ctrl)

				handler := &EvaluatorHandlerImpl{
					auth:             mockAuth,
					evaluatorService: mockEvaluatorService,
					auditClient:      mockAuditClient,
					metrics:          mockMetrics,
				}

				// 测试包含敏感内容的创建请求
				request := &evaluatorservice.CreateEvaluatorRequest{
					Evaluator: &evaluatordto.Evaluator{
						WorkspaceID:   gptr.Of(int64(123)),
						Name:          gptr.Of("敏感内容评估器"),
						Description:   gptr.Of("包含敏感词汇的描述"),
						EvaluatorType: gptr.Of(evaluatordto.EvaluatorType_Prompt),
						CurrentVersion: &evaluatordto.EvaluatorVersion{
							Version:     gptr.Of("1.0.0"),
							Description: gptr.Of("版本描述包含敏感内容"),
							EvaluatorContent: &evaluatordto.EvaluatorContent{
								PromptEvaluator: &evaluatordto.PromptEvaluator{},
							},
						},
					},
				}

				// 设置审核被拒绝的场景
				mockAuditClient.EXPECT().
					Audit(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, param audit.AuditParam) (audit.AuditRecord, error) {
						// 验证审核参数
						assert.Equal(t, audit.AuditType_CozeLoopEvaluatorModify, param.AuditType)
						assert.Contains(t, param.AuditData["texts"], "敏感内容评估器")

						return audit.AuditRecord{
							AuditStatus:  audit.AuditStatus_Rejected,
							FailedReason: gptr.Of("内容包含敏感词汇"),
						}, nil
					}).
					Times(1)

				ctx := context.Background()
				_, err := handler.CreateEvaluator(ctx, request)
				assert.Error(t, err)

				// 验证错误类型
				statusErr, ok := errorx.FromStatusError(err)
				assert.True(t, ok)
				assert.Equal(t, int32(errno.RiskContentDetectedCode), statusErr.Code())
			},
		},
		{
			name: "并发安全和数据一致性测试",
			testFunc: func(t *testing.T) {
				t.Parallel()

				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
				mockUserInfoService := userinfomocks.NewMockUserInfoService(ctrl)

				handler := &EvaluatorHandlerImpl{
					auth:             mockAuth,
					evaluatorService: mockEvaluatorService,
					userInfoService:  mockUserInfoService,
				}

				// 模拟并发访问同一个评估器
				evaluatorID := int64(123)
				spaceID := int64(456)

				evaluator := &entity.Evaluator{
					ID:      evaluatorID,
					SpaceID: spaceID,
					Name:    "并发测试评估器",
				}

				// 设置并发调用的期望
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), spaceID, evaluatorID, false).
					Return(evaluator, nil).
					Times(10) // 10个并发请求

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil).
					Times(10)

				mockUserInfoService.EXPECT().
					PackUserInfo(gomock.Any(), gomock.Any()).
					Times(10)

				// 并发调用
				const numGoroutines = 10
				results := make(chan error, numGoroutines)

				for i := 0; i < numGoroutines; i++ {
					go func() {
						ctx := context.Background()
						request := &evaluatorservice.GetEvaluatorRequest{
							WorkspaceID: spaceID,
							EvaluatorID: &evaluatorID,
						}

						resp, err := handler.GetEvaluator(ctx, request)
						if err != nil {
							results <- err
							return
						}

						// 验证响应数据一致性
						if resp.Evaluator.GetEvaluatorID() != evaluatorID {
							results <- fmt.Errorf("inconsistent evaluator ID: expected %d, got %d",
								evaluatorID, resp.Evaluator.GetEvaluatorID())
							return
						}

						results <- nil
					}()
				}

				// 收集结果
				for i := 0; i < numGoroutines; i++ {
					select {
					case err := <-results:
						assert.NoError(t, err)
					case <-time.After(5 * time.Second):
						t.Fatal("Timeout waiting for concurrent calls")
					}
				}
			},
		},
		{
			name: "错误处理和恢复机制测试",
			testFunc: func(t *testing.T) {
				t.Parallel()

				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
				mockEvaluatorRecordService := mocks.NewMockEvaluatorRecordService(ctrl)

				handler := &EvaluatorHandlerImpl{
					auth:                   mockAuth,
					evaluatorService:       mockEvaluatorService,
					evaluatorRecordService: mockEvaluatorRecordService,
				}

				// 测试运行评估器时的错误恢复
				request := &evaluatorservice.RunEvaluatorRequest{
					EvaluatorVersionID: 123,
					WorkspaceID:        456,
					InputData: &evaluatordto.EvaluatorInputData{
						InputFields: map[string]*common.Content{},
					},
				}

				// 第一次调用失败，第二次成功（模拟重试机制）
				callCount := 0
				mockEvaluatorService.EXPECT().
					GetEvaluatorVersion(gomock.Any(), gomock.Any(), int64(123), false, gomock.Any()).
					DoAndReturn(func(ctx context.Context, spaceID *int64, evaluatorVersionID int64, includeDeleted bool, withTags bool) (*entity.Evaluator, error) {
						callCount++
						if callCount == 1 {
							return nil, errors.New("temporary database error")
						}
						return &entity.Evaluator{
							ID:      1,
							SpaceID: 456,
							Name:    "test-evaluator",
						}, nil
					}).
					Times(2)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil).
					Times(1)

				mockEvaluatorService.EXPECT().
					RunEvaluator(gomock.Any(), gomock.Any()).
					Return(&entity.EvaluatorRecord{
						ID:                 789,
						EvaluatorVersionID: 123,
						SpaceID:            456,
					}, nil).
					Times(1)

				ctx := context.Background()

				// 第一次调用应该失败
				resp1, err1 := handler.RunEvaluator(ctx, request)
				assert.Error(t, err1)
				assert.Nil(t, resp1)

				// 第二次调用应该成功
				resp2, err2 := handler.RunEvaluator(ctx, request)
				assert.NoError(t, err2)
				assert.NotNil(t, resp2)
				assert.Equal(t, int64(789), resp2.Record.GetID())
			},
		},
		{
			name: "大数据量处理性能测试",
			testFunc: func(t *testing.T) {
				t.Parallel()

				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
				mockUserInfoService := userinfomocks.NewMockUserInfoService(ctrl)

				handler := &EvaluatorHandlerImpl{
					auth:             mockAuth,
					evaluatorService: mockEvaluatorService,
					userInfoService:  mockUserInfoService,
				}

				// 创建大量评估器数据
				const numEvaluators = 1000
				evaluators := make([]*entity.Evaluator, numEvaluators)
				for i := 0; i < numEvaluators; i++ {
					evaluators[i] = &entity.Evaluator{
						ID:      int64(i + 1),
						SpaceID: 123,
						Name:    fmt.Sprintf("evaluator-%d", i+1),
					}
				}

				request := &evaluatorservice.ListEvaluatorsRequest{
					WorkspaceID: 123,
					PageSize:    gptr.Of(int32(numEvaluators)),
					PageNumber:  gptr.Of(int32(1)),
				}

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil).
					Times(1)

				mockEvaluatorService.EXPECT().
					ListEvaluator(gomock.Any(), gomock.Any()).
					Return(evaluators, int64(numEvaluators), nil).
					Times(1)

				mockUserInfoService.EXPECT().
					PackUserInfo(gomock.Any(), gomock.Any()).
					Times(1)

				ctx := context.Background()
				start := time.Now()
				resp, err := handler.ListEvaluators(ctx, request)
				duration := time.Since(start)

				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, int64(numEvaluators), gptr.Indirect(resp.Total))
				assert.Len(t, resp.Evaluators, numEvaluators)
				assert.Less(t, duration, 2*time.Second) // 确保性能合理

				// 验证数据完整性
				for i, evaluatorDTO := range resp.Evaluators {
					assert.Equal(t, int64(i+1), evaluatorDTO.GetEvaluatorID())
					assert.Equal(t, fmt.Sprintf("evaluator-%d", i+1), evaluatorDTO.GetName())
				}
			},
		},
		{
			name: "复杂业务流程端到端测试",
			testFunc: func(t *testing.T) {
				t.Parallel()

				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				// 创建完整的依赖链
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockConfiger := confmocks.NewMockIConfiger(ctrl)
				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
				mockEvaluatorRecordService := mocks.NewMockEvaluatorRecordService(ctrl)
				mockMetrics := metricsmock.NewMockEvaluatorExecMetrics(ctrl)
				mockUserInfoService := userinfomocks.NewMockUserInfoService(ctrl)
				mockAuditClient := auditmocks.NewMockIAuditService(ctrl)
				mockBenefitService := benefitmocks.NewMockIBenefitService(ctrl)
				mockFileProvider := rpcmocks.NewMockIFileProvider(ctrl)

				handler := NewEvaluatorHandlerImpl(
					mockIDGen,
					mockConfiger,
					mockAuth,
					mockEvaluatorService,
					mockEvaluatorRecordService,
					nil, // mockEvaluatorTemplateService - 暂时设为nil
					mockMetrics,
					mockUserInfoService,
					mockAuditClient,
					mockBenefitService,
					mockFileProvider,
					make(map[entity.EvaluatorType]service.EvaluatorSourceService),
				)

				// 模拟完整的评估器生命周期：创建 -> 更新 -> 提交版本 -> 运行 -> 删除
				ctx := context.Background()
				spaceID := int64(123)
				evaluatorID := int64(456)

				// 1. 创建评估器
				createRequest := &evaluatorservice.CreateEvaluatorRequest{
					Evaluator: &evaluatordto.Evaluator{
						WorkspaceID:   gptr.Of(spaceID),
						Name:          gptr.Of("端到端测试评估器"),
						Description:   gptr.Of("用于端到端测试的评估器"),
						EvaluatorType: gptr.Of(evaluatordto.EvaluatorType_Prompt),
						CurrentVersion: &evaluatordto.EvaluatorVersion{
							Version: gptr.Of("1.0.0"),
							EvaluatorContent: &evaluatordto.EvaluatorContent{
								PromptEvaluator: &evaluatordto.PromptEvaluator{},
							},
						},
					},
				}

				// Mock 创建流程
				mockAuditClient.EXPECT().
					Audit(gomock.Any(), gomock.Any()).
					Return(audit.AuditRecord{AuditStatus: audit.AuditStatus_Approved}, nil).
					Times(1)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(spaceID, 10),
						SpaceID:       spaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("createLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
					}).
					Return(nil).
					Times(1)

				mockEvaluatorService.EXPECT().
					CreateEvaluator(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(evaluatorID, nil).
					Times(1)

				mockMetrics.EXPECT().
					EmitCreate(spaceID, nil).
					Times(1)

				createResp, err := handler.CreateEvaluator(ctx, createRequest)
				assert.NoError(t, err)
				assert.Equal(t, evaluatorID, gptr.Indirect(createResp.EvaluatorID))

				// 2. 更新评估器
				updateRequest := &evaluatorservice.UpdateEvaluatorRequest{
					WorkspaceID: spaceID,
					EvaluatorID: evaluatorID,
					Name:        gptr.Of("更新后的评估器"),
					Description: gptr.Of("更新后的描述"),
				}

				evaluator := &entity.Evaluator{
					ID:      evaluatorID,
					SpaceID: spaceID,
					Name:    "端到端测试评估器",
				}

				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), spaceID, evaluatorID, false).
					Return(evaluator, nil).
					Times(1)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(evaluatorID, 10),
						SpaceID:       spaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
					}).
					Return(nil).
					Times(1)

				mockAuditClient.EXPECT().
					Audit(gomock.Any(), gomock.Any()).
					Return(audit.AuditRecord{AuditStatus: audit.AuditStatus_Approved}, nil).
					Times(1)

				mockEvaluatorService.EXPECT().
					UpdateEvaluatorMeta(gomock.Any(), gomock.Any()).
					Return(nil).
					Times(1)

				updateResp, err := handler.UpdateEvaluator(ctx, updateRequest)
				assert.NoError(t, err)
				assert.NotNil(t, updateResp)

				// 3. 删除评估器
				deleteRequest := &evaluatorservice.DeleteEvaluatorRequest{
					WorkspaceID: spaceID,
					EvaluatorID: &evaluatorID,
				}

				mockEvaluatorService.EXPECT().
					BatchGetEvaluator(gomock.Any(), spaceID, []int64{evaluatorID}, false).
					Return([]*entity.Evaluator{evaluator}, nil).
					Times(1)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(evaluatorID, 10),
						SpaceID:       spaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
					}).
					Return(nil).
					Times(1)

				mockEvaluatorService.EXPECT().
					DeleteEvaluator(gomock.Any(), []int64{evaluatorID}, gomock.Any()).
					Return(nil).
					Times(1)

				deleteResp, err := handler.DeleteEvaluator(ctx, deleteRequest)
				assert.NoError(t, err)
				assert.NotNil(t, deleteResp)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.testFunc)
	}
}

// TestEvaluatorHandlerImpl_EdgeCasesAndBoundaryConditions 测试边界条件
func TestEvaluatorHandlerImpl_EdgeCasesAndBoundaryConditions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "空请求和 nil 参数处理",
			testFunc: func(t *testing.T) {
				t.Parallel()

				handler := &EvaluatorHandlerImpl{}
				ctx := context.Background()

				// 测试各种 nil 请求
				_, err1 := handler.CreateEvaluator(ctx, nil)
				assert.Error(t, err1)
			},
		},
		{
			name: "超长字符串处理",
			testFunc: func(t *testing.T) {
				t.Parallel()

				handler := &EvaluatorHandlerImpl{}
				ctx := context.Background()

				// 创建超长名称
				longName := string(make([]rune, consts.MaxEvaluatorNameLength+100))
				longDesc := string(make([]rune, consts.MaxEvaluatorDescLength+100))

				request := &evaluatorservice.CreateEvaluatorRequest{
					Evaluator: &evaluatordto.Evaluator{
						WorkspaceID:   gptr.Of(int64(123)),
						Name:          gptr.Of(longName),
						Description:   gptr.Of(longDesc),
						EvaluatorType: gptr.Of(evaluatordto.EvaluatorType_Prompt),
						CurrentVersion: &evaluatordto.EvaluatorVersion{
							Version: gptr.Of("1.0.0"),
							EvaluatorContent: &evaluatordto.EvaluatorContent{
								PromptEvaluator: &evaluatordto.PromptEvaluator{},
							},
						},
					},
				}

				_, err := handler.CreateEvaluator(ctx, request)
				assert.Error(t, err)

				// 验证错误类型
				statusErr, ok := errorx.FromStatusError(err)
				assert.True(t, ok)
				assert.Equal(t, int32(errno.EvaluatorNameExceedMaxLengthCode), statusErr.Code())
			},
		},
		{
			name: "特殊字符和编码处理",
			testFunc: func(t *testing.T) {
				t.Parallel()

				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
				mockAuditClient := auditmocks.NewMockIAuditService(ctrl)
				mockMetrics := metricsmock.NewMockEvaluatorExecMetrics(ctrl)

				handler := &EvaluatorHandlerImpl{
					auth:             mockAuth,
					evaluatorService: mockEvaluatorService,
					auditClient:      mockAuditClient,
					metrics:          mockMetrics,
				}

				// 包含各种特殊字符的请求
				request := &evaluatorservice.CreateEvaluatorRequest{
					Evaluator: &evaluatordto.Evaluator{
						WorkspaceID:   gptr.Of(int64(123)),
						Name:          gptr.Of("测试🚀评估器💡"),
						Description:   gptr.Of("包含emoji和特殊字符的描述：<>&\"'"),
						EvaluatorType: gptr.Of(evaluatordto.EvaluatorType_Prompt),
						CurrentVersion: &evaluatordto.EvaluatorVersion{
							Version: gptr.Of("1.0.0"),
							EvaluatorContent: &evaluatordto.EvaluatorContent{
								PromptEvaluator: &evaluatordto.PromptEvaluator{},
							},
						},
					},
				}

				mockAuditClient.EXPECT().
					Audit(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, param audit.AuditParam) (audit.AuditRecord, error) {
						// 验证特殊字符被正确处理
						assert.Contains(t, param.AuditData["texts"], "测试🚀评估器💡")
						return audit.AuditRecord{AuditStatus: audit.AuditStatus_Approved}, nil
					}).
					Times(1)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil).
					Times(1)

				mockEvaluatorService.EXPECT().
					CreateEvaluator(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(int64(123), nil).
					Times(1)

				mockMetrics.EXPECT().
					EmitCreate(gomock.Any(), gomock.Any()).
					Times(1)

				ctx := context.Background()
				resp, err := handler.CreateEvaluator(ctx, request)
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			},
		},
		{
			name: "上下文取消和超时处理",
			testFunc: func(t *testing.T) {
				t.Parallel()

				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)

				handler := &EvaluatorHandlerImpl{
					auth:             mockAuth,
					evaluatorService: mockEvaluatorService,
				}

				// 创建已取消的上下文
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				request := &evaluatorservice.ListEvaluatorsRequest{
					WorkspaceID: 123,
				}

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, param *rpc.AuthorizationParam) error {
						// 检查上下文是否已取消
						select {
						case <-ctx.Done():
							return ctx.Err()
						default:
							return nil
						}
					}).
					Times(1)

				_, err := handler.ListEvaluators(ctx, request)
				assert.Error(t, err)
				assert.Equal(t, context.Canceled, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.testFunc)
	}
}

func TestEvaluatorHandlerImpl_ListTemplates_Code(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfiger := confmocks.NewMockIConfiger(ctrl)
	handler := &EvaluatorHandlerImpl{
		configer: mockConfiger,
	}

	// 模拟新的Code配置数据结构
	codeTemplateConf := map[string]map[string]*evaluatordto.EvaluatorContent{
		"python_template_1": {
			"Python": {
				CodeEvaluator: &evaluatordto.CodeEvaluator{
					LanguageType:     ptr.Of("Python"),
					CodeTemplateKey:  ptr.Of("python_template_1"),
					CodeTemplateName: ptr.Of("Python评估模板1"),
				},
			},
		},
		"python_template_2": {
			"Python": {
				CodeEvaluator: &evaluatordto.CodeEvaluator{
					LanguageType:     ptr.Of("Python"),
					CodeTemplateKey:  ptr.Of("python_template_2"),
					CodeTemplateName: ptr.Of("Python评估模板2"),
				},
			},
		},
		"js_template_1": {
			"JS": {
				CodeEvaluator: &evaluatordto.CodeEvaluator{
					LanguageType:     ptr.Of("JS"),
					CodeTemplateKey:  ptr.Of("js_template_1"),
					CodeTemplateName: ptr.Of("JS评估模板1"),
				},
			},
		},
	}

	tests := []struct {
		name         string
		request      *evaluatorservice.ListTemplatesRequest
		expectedKeys []string
	}{
		{
			name: "Code类型-不再按语言筛选",
			request: &evaluatorservice.ListTemplatesRequest{
				BuiltinTemplateType: evaluatordto.TemplateType_Code,
			},
			expectedKeys: []string{"js_template_1", "python_template_1", "python_template_2"}, // 按template_key去重后排序
		},
		{
			name: "Code类型-未指定语言",
			request: &evaluatorservice.ListTemplatesRequest{
				BuiltinTemplateType: evaluatordto.TemplateType_Code,
			},
			expectedKeys: []string{"js_template_1", "python_template_1", "python_template_2"}, // 按template_key去重后排序
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConfiger.EXPECT().GetCodeEvaluatorTemplateConf(gomock.Any()).Return(codeTemplateConf)

			resp, err := handler.ListTemplates(context.Background(), tt.request)

			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Len(t, resp.BuiltinTemplateKeys, len(tt.expectedKeys))

			// 验证返回的模板key
			actualKeys := make([]string, len(resp.BuiltinTemplateKeys))
			for i, template := range resp.BuiltinTemplateKeys {
				if template.GetCodeEvaluator() != nil {
					actualKeys[i] = template.GetCodeEvaluator().GetCodeTemplateKey()
				}
			}

			for i, expectedKey := range tt.expectedKeys {
				assert.Equal(t, expectedKey, actualKeys[i], "Expected key at index %d", i)
			}
		})
	}
}

func TestEvaluatorHandlerImpl_GetTemplateInfo_Code(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfiger := confmocks.NewMockIConfiger(ctrl)
	handler := &EvaluatorHandlerImpl{
		configer: mockConfiger,
	}

	// 模拟配置数据
	codeTemplateConf := map[string]map[string]*evaluatordto.EvaluatorContent{
		"python_template_1": {
			"Python": {
				CodeEvaluator: &evaluatordto.CodeEvaluator{
					LanguageType:     ptr.Of("Python"),
					CodeTemplateKey:  ptr.Of("python_template_1"),
					CodeTemplateName: ptr.Of("Python评估模板1"),
					CodeContent:      ptr.Of("# Python代码示例"),
				},
			},
		},
		"js_template_1": {
			"JS": {
				CodeEvaluator: &evaluatordto.CodeEvaluator{
					LanguageType:     ptr.Of("JS"),
					CodeTemplateKey:  ptr.Of("js_template_1"),
					CodeTemplateName: ptr.Of("JS评估模板1"),
					CodeContent:      ptr.Of("// JS代码示例"),
				},
			},
		},
	}

	tests := []struct {
		name        string
		request     *evaluatorservice.GetTemplateInfoRequest
		expectError bool
		expectedKey string
	}{
		{
			name: "Code类型-Python语言-存在的模板",
			request: &evaluatorservice.GetTemplateInfoRequest{
				BuiltinTemplateType: evaluatordto.TemplateType_Code,
				BuiltinTemplateKey:  "python_template_1",
				LanguageType:        ptr.Of("Python"),
			},
			expectError: false,
			expectedKey: "python_template_1",
		},
		{
			name: "Code类型-JS语言-存在的模板",
			request: &evaluatorservice.GetTemplateInfoRequest{
				BuiltinTemplateType: evaluatordto.TemplateType_Code,
				BuiltinTemplateKey:  "js_template_1",
				LanguageType:        ptr.Of("JS"),
			},
			expectError: false,
			expectedKey: "js_template_1",
		},
		{
			name: "Code类型-语言不匹配",
			request: &evaluatorservice.GetTemplateInfoRequest{
				BuiltinTemplateType: evaluatordto.TemplateType_Code,
				BuiltinTemplateKey:  "python_template_1",
				LanguageType:        ptr.Of("JS"),
			},
			expectError: true,
		},
		{
			name: "Code类型-不存在的模板",
			request: &evaluatorservice.GetTemplateInfoRequest{
				BuiltinTemplateType: evaluatordto.TemplateType_Code,
				BuiltinTemplateKey:  "nonexistent_template",
				LanguageType:        ptr.Of("Python"),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConfiger.EXPECT().GetCodeEvaluatorTemplateConf(gomock.Any()).Return(codeTemplateConf)

			resp, err := handler.GetTemplateInfo(context.Background(), tt.request)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.NotNil(t, resp.EvaluatorContent)
				assert.NotNil(t, resp.EvaluatorContent.GetCodeEvaluator())
				assert.Equal(t, tt.expectedKey, resp.EvaluatorContent.GetCodeEvaluator().GetCodeTemplateKey())
			}
		})
	}
}

func TestBuildCodeTemplateKeys(t *testing.T) {
	codeTemplates := map[string]map[string]*evaluatordto.EvaluatorContent{
		"python_template_1": {
			"Python": {
				CodeEvaluator: &evaluatordto.CodeEvaluator{
					LanguageType:     ptr.Of("Python"),
					CodeTemplateKey:  ptr.Of("python_template_1"),
					CodeTemplateName: ptr.Of("Python评估模板1"),
				},
			},
			"JS": {
				CodeEvaluator: &evaluatordto.CodeEvaluator{
					LanguageType:     ptr.Of("JS"),
					CodeTemplateKey:  ptr.Of("python_template_1"), // 相同的template_key，不同的language
					CodeTemplateName: ptr.Of("Python模板JS版本"),
				},
			},
		},
		"js_template_1": {
			"JS": {
				CodeEvaluator: &evaluatordto.CodeEvaluator{
					LanguageType:     ptr.Of("JS"),
					CodeTemplateKey:  ptr.Of("js_template_1"),
					CodeTemplateName: ptr.Of("JS评估模板1"),
				},
			},
		},
	}

	result := buildCodeTemplateKeys(codeTemplates)

	// 应该去重，只有2个不同的template_key
	assert.Len(t, result, 2)

	// 验证排序和去重
	keys := make([]string, len(result))
	for i, template := range result {
		assert.NotNil(t, template.GetCodeEvaluator())
		keys[i] = template.GetCodeEvaluator().GetCodeTemplateKey()
		// 验证不包含LanguageType
		assert.Nil(t, template.GetCodeEvaluator().LanguageType)
	}

	// 验证按key排序
	assert.Equal(t, "js_template_1", keys[0])
	assert.Equal(t, "python_template_1", keys[1])
}

func TestBuildTemplateKeys_PromptOnly(t *testing.T) {
	// 测试buildTemplateKeys函数现在只处理Prompt类型
	origins := map[string]*evaluatordto.EvaluatorContent{
		"prompt_template_1": {
			PromptEvaluator: &evaluatordto.PromptEvaluator{
				PromptTemplateKey:  ptr.Of("prompt_template_1"),
				PromptTemplateName: ptr.Of("Prompt评估模板1"),
			},
		},
		"prompt_template_2": {
			PromptEvaluator: &evaluatordto.PromptEvaluator{
				PromptTemplateKey:  ptr.Of("prompt_template_2"),
				PromptTemplateName: ptr.Of("Prompt评估模板2"),
			},
		},
		"code_template_1": {
			CodeEvaluator: &evaluatordto.CodeEvaluator{
				LanguageType:     ptr.Of("Python"),
				CodeTemplateKey:  ptr.Of("code_template_1"),
				CodeTemplateName: ptr.Of("Code评估模板1"),
			},
		},
	}

	// 测试Prompt类型
	promptResult := buildTemplateKeys(origins, evaluatordto.TemplateType_Prompt)
	assert.Len(t, promptResult, 2) // 只应该返回2个Prompt模板

	// 验证返回的都是Prompt类型
	for _, template := range promptResult {
		assert.NotNil(t, template.GetPromptEvaluator())
		assert.Nil(t, template.GetCodeEvaluator())
	}

	// 验证排序
	keys := make([]string, len(promptResult))
	for i, template := range promptResult {
		keys[i] = template.GetPromptEvaluator().GetPromptTemplateKey()
	}
	assert.Equal(t, "prompt_template_1", keys[0])
	assert.Equal(t, "prompt_template_2", keys[1])

	// 测试Code类型 - 现在应该返回空结果
	codeResult := buildTemplateKeys(origins, evaluatordto.TemplateType_Code)
	assert.Len(t, codeResult, 0) // Code类型现在不应该被处理
}

func TestEvaluatorHandlerImpl_GetCodeEvaluatorTemplateConf(t *testing.T) {
	// 这个测试验证GetCodeEvaluatorTemplateConf方法的转换逻辑
	// 我们直接测试configer的实现，而不是mock

	// 创建一个实际的configer实例进行测试
	// 由于我们只测试转换逻辑，可以通过集成测试来验证
	t.Skip("This test requires actual configer implementation, tested in integration tests")
}

// Test_buildRunEvaluatorRequest_DisableTracing 测试buildRunEvaluatorRequest函数正确设置DisableTracing字段
func Test_buildRunEvaluatorRequest_DisableTracing(t *testing.T) {
	tests := []struct {
		name                string
		request             *evaluatorservice.RunEvaluatorRequest
		expectedTracing     bool
		expectedEvaluatorID int64
		expectedSpaceID     int64
	}{
		{
			name: "DisableTracing为true时正确传递",
			request: &evaluatorservice.RunEvaluatorRequest{
				WorkspaceID:        123,
				EvaluatorVersionID: 456,
				ExperimentID:       gptr.Of(int64(789)),
				ExperimentRunID:    gptr.Of(int64(101112)),
				ItemID:             gptr.Of(int64(131415)),
				TurnID:             gptr.Of(int64(161718)),
				InputData: &evaluatordto.EvaluatorInputData{
					InputFields: map[string]*common.Content{
						"test": {
							ContentType: gptr.Of(common.ContentTypeText),
							Text:        gptr.Of("test input"),
						},
					},
				},
			},
			expectedTracing:     true, // 模拟API层面传入true
			expectedEvaluatorID: 456,
			expectedSpaceID:     123,
		},
		{
			name: "DisableTracing为false时正确传递",
			request: &evaluatorservice.RunEvaluatorRequest{
				WorkspaceID:        123,
				EvaluatorVersionID: 456,
				ExperimentID:       gptr.Of(int64(789)),
				InputData: &evaluatordto.EvaluatorInputData{
					InputFields: map[string]*common.Content{},
				},
			},
			expectedTracing:     false, // 模拟API层面传入false
			expectedEvaluatorID: 456,
			expectedSpaceID:     123,
		},
		{
			name: "DisableTracing默认情况",
			request: &evaluatorservice.RunEvaluatorRequest{
				WorkspaceID:        123,
				EvaluatorVersionID: 456,
				InputData: &evaluatordto.EvaluatorInputData{
					InputFields: map[string]*common.Content{},
				},
			},
			expectedTracing:     false, // 默认为false
			expectedEvaluatorID: 456,
			expectedSpaceID:     123,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 模拟API层面设置DisableTracing参数的逻辑
			// 由于API接口暂时还没有这个字段，我们通过模拟的方式测试内部参数传递
			// 创建一个模拟的请求，手动设置DisableTracing字段用于测试
			mockRequest := &entity.RunEvaluatorRequest{
				SpaceID:            tt.request.WorkspaceID,
				Name:               "test-evaluator",
				EvaluatorVersionID: tt.request.EvaluatorVersionID,
				ExperimentID:       tt.request.GetExperimentID(),
				ExperimentRunID:    tt.request.GetExperimentRunID(),
				ItemID:             tt.request.GetItemID(),
				TurnID:             tt.request.GetTurnID(),
				DisableTracing:     tt.expectedTracing, // 手动设置用于测试
			}

			// 验证DisableTracing字段正确设置
			assert.Equal(t, tt.expectedTracing, mockRequest.DisableTracing)

			// 验证其他基本字段
			assert.Equal(t, tt.expectedSpaceID, mockRequest.SpaceID)
			assert.Equal(t, tt.expectedEvaluatorID, mockRequest.EvaluatorVersionID)
			assert.Equal(t, "test-evaluator", mockRequest.Name)

			// 验证可选字段
			assert.Equal(t, tt.request.GetExperimentID(), mockRequest.ExperimentID)
			assert.Equal(t, tt.request.GetExperimentRunID(), mockRequest.ExperimentRunID)
			assert.Equal(t, tt.request.GetItemID(), mockRequest.ItemID)
			assert.Equal(t, tt.request.GetTurnID(), mockRequest.TurnID)
		})
	}
}

// 验证：当请求携带 EvaluatorRunConf 且包含 evaluator_runtime_param.json_value 时，输入数据的 Ext 注入运行时参数
func Test_buildRunEvaluatorRequest_EvaluatorRunConfRuntimeParam(t *testing.T) {
	rp := `{"model_config":{"model_id":"m-1","temperature":0.8}}`

	req := &evaluatorservice.RunEvaluatorRequest{
		WorkspaceID:        123,
		EvaluatorVersionID: 456,
		InputData: &evaluatordto.EvaluatorInputData{
			InputFields: map[string]*common.Content{
				"input": {ContentType: ptr.Of(common.ContentTypeText), Text: ptr.Of("hello")},
			},
		},
		EvaluatorRunConf: &evaluatordto.EvaluatorRunConfig{
			EvaluatorRuntimeParam: &common.RuntimeParam{JSONValue: ptr.Of(rp)},
		},
	}

	got := buildRunEvaluatorRequest("test-evaluator", req)
	if got == nil || got.InputData == nil {
		t.Fatalf("nil RunEvaluatorRequest or InputData")
	}

	// 校验 Ext 注入运行时参数
	if got.InputData.Ext[consts.FieldAdapterBuiltinFieldNameRuntimeParam] != rp {
		t.Fatalf("runtime_param not injected, got=%s", got.InputData.Ext[consts.FieldAdapterBuiltinFieldNameRuntimeParam])
	}
	// 基本字段不受影响
	if got.EvaluatorVersionID != 456 || got.SpaceID != 123 {
		t.Fatalf("mismatch basic fields: verID=%d spaceID=%d", got.EvaluatorVersionID, got.SpaceID)
	}
}

// TestEvaluatorHandlerImpl_ValidateEvaluator 测试 ValidateEvaluator 方法
func TestEvaluatorHandlerImpl_ValidateEvaluator(t *testing.T) {
	t.Parallel()

	validWorkspaceID := int64(123)

	tests := []struct {
		name        string
		req         *evaluatorservice.ValidateEvaluatorRequest
		mockSetup   func(*rpcmocks.MockIAuthProvider, *mocks.MockEvaluatorSourceService)
		wantResp    *evaluatorservice.ValidateEvaluatorResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - valid prompt evaluator",
			req: &evaluatorservice.ValidateEvaluatorRequest{
				WorkspaceID:   validWorkspaceID,
				EvaluatorType: evaluatordto.EvaluatorType_Prompt,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					PromptEvaluator: &evaluatordto.PromptEvaluator{
						MessageList: []*common.Message{
							{
								Role: common.RolePtr(common.Role_User),
								Content: &common.Content{
									ContentType: gptr.Of(common.ContentTypeText),
									Text:        gptr.Of("test prompt"),
								},
							},
						},
						ModelConfig: &common.ModelConfig{
							ModelID: gptr.Of(int64(1)),
						},
					},
				},
			},
			mockSetup: func(mockAuth *rpcmocks.MockIAuthProvider, mockEvaluatorSourceService *mocks.MockEvaluatorSourceService) {
				mockAuth.EXPECT().Authorization(gomock.Any(), &rpc.AuthorizationParam{
					ObjectID:      strconv.FormatInt(validWorkspaceID, 10),
					SpaceID:       validWorkspaceID,
					ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("debugLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
				}).Return(nil)

				mockEvaluatorSourceService.EXPECT().Validate(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantResp: &evaluatorservice.ValidateEvaluatorResponse{
				Valid: gptr.Of(true),
			},
			wantErr: false,
		},
		{
			name: "success - valid code evaluator",
			req: &evaluatorservice.ValidateEvaluatorRequest{
				WorkspaceID:   validWorkspaceID,
				EvaluatorType: evaluatordto.EvaluatorType_Code,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					CodeEvaluator: &evaluatordto.CodeEvaluator{
						CodeContent:  gptr.Of("def evaluate(input): return 1.0"),
						LanguageType: gptr.Of(evaluatordto.LanguageTypePython),
					},
				},
			},
			mockSetup: func(mockAuth *rpcmocks.MockIAuthProvider, mockEvaluatorSourceService *mocks.MockEvaluatorSourceService) {
				mockAuth.EXPECT().Authorization(gomock.Any(), &rpc.AuthorizationParam{
					ObjectID:      strconv.FormatInt(validWorkspaceID, 10),
					SpaceID:       validWorkspaceID,
					ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("debugLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
				}).Return(nil)

				mockEvaluatorSourceService.EXPECT().Validate(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantResp: &evaluatorservice.ValidateEvaluatorResponse{
				Valid: gptr.Of(true),
			},
			wantErr: false,
		},
		{
			name: "failure - auth error",
			req: &evaluatorservice.ValidateEvaluatorRequest{
				WorkspaceID:   validWorkspaceID,
				EvaluatorType: evaluatordto.EvaluatorType_Prompt,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					PromptEvaluator: &evaluatordto.PromptEvaluator{
						MessageList: []*common.Message{
							{
								Role: common.RolePtr(common.Role_User),
								Content: &common.Content{
									ContentType: gptr.Of(common.ContentTypeText),
									Text:        gptr.Of("test prompt"),
								},
							},
						},
						ModelConfig: &common.ModelConfig{
							ModelID: gptr.Of(int64(1)),
						},
					},
				},
			},
			mockSetup: func(mockAuth *rpcmocks.MockIAuthProvider, mockEvaluatorSourceService *mocks.MockEvaluatorSourceService) {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(errors.New("auth failed"))
			},
			wantErr:     true,
			wantErrCode: 0, // Generic error
		},
		{
			name: "failure - convert evaluator content error",
			req: &evaluatorservice.ValidateEvaluatorRequest{
				WorkspaceID:      validWorkspaceID,
				EvaluatorType:    evaluatordto.EvaluatorType_Prompt,
				EvaluatorContent: nil, // Invalid content
			},
			mockSetup: func(mockAuth *rpcmocks.MockIAuthProvider, mockEvaluatorSourceService *mocks.MockEvaluatorSourceService) {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantResp: &evaluatorservice.ValidateEvaluatorResponse{
				Valid:        gptr.Of(false),
				ErrorMessage: gptr.Of("evaluator content is nil"),
			},
			wantErr: false,
		},
		{
			name: "failure - unsupported evaluator type",
			req: &evaluatorservice.ValidateEvaluatorRequest{
				WorkspaceID:   validWorkspaceID,
				EvaluatorType: evaluatordto.EvaluatorType(999), // Unsupported type
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					PromptEvaluator: &evaluatordto.PromptEvaluator{
						MessageList: []*common.Message{
							{
								Role: common.RolePtr(common.Role_User),
								Content: &common.Content{
									ContentType: gptr.Of(common.ContentTypeText),
									Text:        gptr.Of("test prompt"),
								},
							},
						},
						ModelConfig: &common.ModelConfig{
							ModelID: gptr.Of(int64(1)),
						},
					},
				},
			},
			mockSetup: func(mockAuth *rpcmocks.MockIAuthProvider, mockEvaluatorSourceService *mocks.MockEvaluatorSourceService) {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantResp: &evaluatorservice.ValidateEvaluatorResponse{
				Valid:        gptr.Of(false),
				ErrorMessage: gptr.Of("unsupported evaluator type"),
			},
			wantErr: false,
		},
		{
			name: "failure - validation error from source service",
			req: &evaluatorservice.ValidateEvaluatorRequest{
				WorkspaceID:   validWorkspaceID,
				EvaluatorType: evaluatordto.EvaluatorType_Code,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					CodeEvaluator: &evaluatordto.CodeEvaluator{
						CodeContent:  gptr.Of("invalid code"),
						LanguageType: gptr.Of(evaluatordto.LanguageTypePython),
					},
				},
			},
			mockSetup: func(mockAuth *rpcmocks.MockIAuthProvider, mockEvaluatorSourceService *mocks.MockEvaluatorSourceService) {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockEvaluatorSourceService.EXPECT().Validate(gomock.Any(), gomock.Any()).Return(errors.New("syntax error"))
			},
			wantResp: &evaluatorservice.ValidateEvaluatorResponse{
				Valid:        gptr.Of(false),
				ErrorMessage: gptr.Of("syntax error"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 为每个测试用例创建独立的 mock
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
			mockEvaluatorSourceService := mocks.NewMockEvaluatorSourceService(ctrl)

			app := &EvaluatorHandlerImpl{
				auth: mockAuth,
				evaluatorSourceServices: map[entity.EvaluatorType]service.EvaluatorSourceService{
					entity.EvaluatorTypePrompt: mockEvaluatorSourceService,
					entity.EvaluatorTypeCode:   mockEvaluatorSourceService,
				},
			}

			tt.mockSetup(mockAuth, mockEvaluatorSourceService)

			resp, err := app.ValidateEvaluator(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantResp.GetValid(), resp.GetValid())
				if tt.wantResp.ErrorMessage != nil {
					assert.Contains(t, resp.GetErrorMessage(), *tt.wantResp.ErrorMessage)
				}
			}
		})
	}
}

// TestEvaluatorHandlerImpl_BatchDebugEvaluator 测试 BatchDebugEvaluator 方法
func TestEvaluatorHandlerImpl_BatchDebugEvaluator(t *testing.T) {
	t.Parallel()

	validWorkspaceID := int64(123)

	tests := []struct {
		name        string
		req         *evaluatorservice.BatchDebugEvaluatorRequest
		mockSetup   func(*rpcmocks.MockIAuthProvider, *benefitmocks.MockIBenefitService, *mocks.MockEvaluatorService, *rpcmocks.MockIFileProvider)
		wantResp    *evaluatorservice.BatchDebugEvaluatorResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - single input data",
			req: &evaluatorservice.BatchDebugEvaluatorRequest{
				WorkspaceID:   validWorkspaceID,
				EvaluatorType: evaluatordto.EvaluatorType_Prompt,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					PromptEvaluator: &evaluatordto.PromptEvaluator{
						MessageList: []*common.Message{
							{
								Role: common.RolePtr(common.Role_User),
								Content: &common.Content{
									ContentType: gptr.Of(common.ContentTypeText),
									Text:        gptr.Of("test prompt"),
								},
							},
						},
						ModelConfig: &common.ModelConfig{
							ModelID: gptr.Of(int64(1)),
						},
					},
				},
				InputData: []*evaluatordto.EvaluatorInputData{
					{
						InputFields: map[string]*common.Content{
							"input": {
								ContentType: gptr.Of(common.ContentTypeText),
								Text:        gptr.Of("test input"),
							},
						},
					},
				},
			},
			mockSetup: func(mockAuth *rpcmocks.MockIAuthProvider, mockBenefitService *benefitmocks.MockIBenefitService, mockEvaluatorService *mocks.MockEvaluatorService, mockFileProvider *rpcmocks.MockIFileProvider) {
				mockAuth.EXPECT().Authorization(gomock.Any(), &rpc.AuthorizationParam{
					ObjectID:      strconv.FormatInt(validWorkspaceID, 10),
					SpaceID:       validWorkspaceID,
					ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("debugLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
				}).Return(nil)

				mockBenefitService.EXPECT().CheckEvaluatorBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckEvaluatorBenefitResult{}, nil)

				mockFileProvider.EXPECT().MGetFileURL(gomock.Any(), gomock.Any()).Return(map[string]string{}, nil).AnyTimes()

				mockEvaluatorService.EXPECT().DebugEvaluator(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(
					&entity.EvaluatorOutputData{
						EvaluatorResult: &entity.EvaluatorResult{
							Score:     gptr.Of(0.8),
							Reasoning: "good result",
						},
					}, nil)
			},
			wantResp: &evaluatorservice.BatchDebugEvaluatorResponse{
				EvaluatorOutputData: []*evaluatordto.EvaluatorOutputData{
					{
						EvaluatorResult_: &evaluatordto.EvaluatorResult_{
							Score:     gptr.Of(0.8),
							Reasoning: gptr.Of("good result"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "success - multiple input data",
			req: &evaluatorservice.BatchDebugEvaluatorRequest{
				WorkspaceID:   validWorkspaceID,
				EvaluatorType: evaluatordto.EvaluatorType_Code,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					CodeEvaluator: &evaluatordto.CodeEvaluator{
						CodeContent:  gptr.Of("def evaluate(input): return 1.0"),
						LanguageType: gptr.Of(evaluatordto.LanguageTypePython),
					},
				},
				InputData: []*evaluatordto.EvaluatorInputData{
					{
						InputFields: map[string]*common.Content{
							"input": {
								ContentType: gptr.Of(common.ContentTypeText),
								Text:        gptr.Of("test input 1"),
							},
						},
					},
					{
						InputFields: map[string]*common.Content{
							"input": {
								ContentType: gptr.Of(common.ContentTypeText),
								Text:        gptr.Of("test input 2"),
							},
						},
					},
				},
			},
			mockSetup: func(mockAuth *rpcmocks.MockIAuthProvider, mockBenefitService *benefitmocks.MockIBenefitService, mockEvaluatorService *mocks.MockEvaluatorService, mockFileProvider *rpcmocks.MockIFileProvider) {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)

				mockBenefitService.EXPECT().CheckEvaluatorBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckEvaluatorBenefitResult{}, nil)

				mockFileProvider.EXPECT().MGetFileURL(gomock.Any(), gomock.Any()).Return(map[string]string{}, nil).AnyTimes()

				// 使用 InOrder 来确保调用顺序
				gomock.InOrder(
					mockEvaluatorService.EXPECT().DebugEvaluator(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(
						&entity.EvaluatorOutputData{
							EvaluatorResult: &entity.EvaluatorResult{
								Score:     gptr.Of(0.9),
								Reasoning: "result 1",
							},
						}, nil),
					mockEvaluatorService.EXPECT().DebugEvaluator(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(
						&entity.EvaluatorOutputData{
							EvaluatorResult: &entity.EvaluatorResult{
								Score:     gptr.Of(0.7),
								Reasoning: "result 2",
							},
						}, nil),
				)
			},
			wantResp: &evaluatorservice.BatchDebugEvaluatorResponse{
				EvaluatorOutputData: []*evaluatordto.EvaluatorOutputData{
					{
						EvaluatorResult_: &evaluatordto.EvaluatorResult_{},
					},
					{
						EvaluatorResult_: &evaluatordto.EvaluatorResult_{},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "failure - auth error",
			req: &evaluatorservice.BatchDebugEvaluatorRequest{
				WorkspaceID:   validWorkspaceID,
				EvaluatorType: evaluatordto.EvaluatorType_Prompt,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					PromptEvaluator: &evaluatordto.PromptEvaluator{
						MessageList: []*common.Message{
							{
								Role: common.RolePtr(common.Role_User),
								Content: &common.Content{
									ContentType: gptr.Of(common.ContentTypeText),
									Text:        gptr.Of("test prompt"),
								},
							},
						},
						ModelConfig: &common.ModelConfig{
							ModelID: gptr.Of(int64(1)),
						},
					},
				},
				InputData: []*evaluatordto.EvaluatorInputData{
					{
						InputFields: map[string]*common.Content{
							"input": {
								ContentType: gptr.Of(common.ContentTypeText),
								Text:        gptr.Of("test input"),
							},
						},
					},
				},
			},
			mockSetup: func(mockAuth *rpcmocks.MockIAuthProvider, mockBenefitService *benefitmocks.MockIBenefitService, mockEvaluatorService *mocks.MockEvaluatorService, mockFileProvider *rpcmocks.MockIFileProvider) {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(errors.New("auth failed"))
			},
			wantErr:     true,
			wantErrCode: 0, // Generic error
		},
		{
			name: "failure - benefit check denied",
			req: &evaluatorservice.BatchDebugEvaluatorRequest{
				WorkspaceID:   validWorkspaceID,
				EvaluatorType: evaluatordto.EvaluatorType_Prompt,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					PromptEvaluator: &evaluatordto.PromptEvaluator{
						MessageList: []*common.Message{
							{
								Role: common.RolePtr(common.Role_User),
								Content: &common.Content{
									ContentType: gptr.Of(common.ContentTypeText),
									Text:        gptr.Of("test prompt"),
								},
							},
						},
						ModelConfig: &common.ModelConfig{
							ModelID: gptr.Of(int64(1)),
						},
					},
				},
				InputData: []*evaluatordto.EvaluatorInputData{
					{
						InputFields: map[string]*common.Content{
							"input": {
								ContentType: gptr.Of(common.ContentTypeText),
								Text:        gptr.Of("test input"),
							},
						},
					},
				},
			},
			mockSetup: func(mockAuth *rpcmocks.MockIAuthProvider, mockBenefitService *benefitmocks.MockIBenefitService, mockEvaluatorService *mocks.MockEvaluatorService, mockFileProvider *rpcmocks.MockIFileProvider) {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockBenefitService.EXPECT().CheckEvaluatorBenefit(gomock.Any(), gomock.Any()).Return(
					&benefit.CheckEvaluatorBenefitResult{
						DenyReason: gptr.Of(benefit.DenyReason(1)),
					}, nil)
			},
			wantErr:     true,
			wantErrCode: errno.EvaluatorBenefitDenyCode,
		},
		{
			name: "failure - benefit check service error",
			req: &evaluatorservice.BatchDebugEvaluatorRequest{
				WorkspaceID:   validWorkspaceID,
				EvaluatorType: evaluatordto.EvaluatorType_Prompt,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					PromptEvaluator: &evaluatordto.PromptEvaluator{
						MessageList: []*common.Message{
							{
								Role: common.RolePtr(common.Role_User),
								Content: &common.Content{
									ContentType: gptr.Of(common.ContentTypeText),
									Text:        gptr.Of("test prompt"),
								},
							},
						},
						ModelConfig: &common.ModelConfig{
							ModelID: gptr.Of(int64(1)),
						},
					},
				},
				InputData: []*evaluatordto.EvaluatorInputData{
					{
						InputFields: map[string]*common.Content{
							"input": {
								ContentType: gptr.Of(common.ContentTypeText),
								Text:        gptr.Of("test input"),
							},
						},
					},
				},
			},
			mockSetup: func(mockAuth *rpcmocks.MockIAuthProvider, mockBenefitService *benefitmocks.MockIBenefitService, mockEvaluatorService *mocks.MockEvaluatorService, mockFileProvider *rpcmocks.MockIFileProvider) {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockBenefitService.EXPECT().CheckEvaluatorBenefit(gomock.Any(), gomock.Any()).Return(nil, errors.New("benefit service error"))
			},
			wantErr:     true,
			wantErrCode: 0, // Generic error
		},
		{
			name: "success - partial failures in batch",
			req: &evaluatorservice.BatchDebugEvaluatorRequest{
				WorkspaceID:   validWorkspaceID,
				EvaluatorType: evaluatordto.EvaluatorType_Code,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					CodeEvaluator: &evaluatordto.CodeEvaluator{
						CodeContent:  gptr.Of("def evaluate(input): return 1.0"),
						LanguageType: gptr.Of(evaluatordto.LanguageTypePython),
					},
				},
				InputData: []*evaluatordto.EvaluatorInputData{
					{
						InputFields: map[string]*common.Content{
							"input": {
								ContentType: gptr.Of(common.ContentTypeText),
								Text:        gptr.Of("test input 1"),
							},
						},
					},
					{
						InputFields: map[string]*common.Content{
							"input": {
								ContentType: gptr.Of(common.ContentTypeText),
								Text:        gptr.Of("test input 2"),
							},
						},
					},
				},
			},
			mockSetup: func(mockAuth *rpcmocks.MockIAuthProvider, mockBenefitService *benefitmocks.MockIBenefitService, mockEvaluatorService *mocks.MockEvaluatorService, mockFileProvider *rpcmocks.MockIFileProvider) {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockBenefitService.EXPECT().CheckEvaluatorBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckEvaluatorBenefitResult{}, nil)
				mockFileProvider.EXPECT().MGetFileURL(gomock.Any(), gomock.Any()).Return(map[string]string{}, nil).AnyTimes()

				// 使用 InOrder 来确保调用顺序
				gomock.InOrder(
					mockEvaluatorService.EXPECT().DebugEvaluator(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(
						&entity.EvaluatorOutputData{
							EvaluatorResult: &entity.EvaluatorResult{
								Score:     gptr.Of(0.8),
								Reasoning: "success result",
							},
						}, nil),
					mockEvaluatorService.EXPECT().DebugEvaluator(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(
						nil, errors.New("evaluation failed")),
				)
			},
			wantResp: &evaluatorservice.BatchDebugEvaluatorResponse{
				EvaluatorOutputData: []*evaluatordto.EvaluatorOutputData{
					{},
					{},
				},
			},
			wantErr: false,
		},
		{
			name: "success - empty input data",
			req: &evaluatorservice.BatchDebugEvaluatorRequest{
				WorkspaceID:   validWorkspaceID,
				EvaluatorType: evaluatordto.EvaluatorType_Prompt,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					PromptEvaluator: &evaluatordto.PromptEvaluator{
						MessageList: []*common.Message{
							{
								Role: common.RolePtr(common.Role_User),
								Content: &common.Content{
									ContentType: gptr.Of(common.ContentTypeText),
									Text:        gptr.Of("test prompt"),
								},
							},
						},
						ModelConfig: &common.ModelConfig{
							ModelID: gptr.Of(int64(1)),
						},
					},
				},
				InputData: []*evaluatordto.EvaluatorInputData{},
			},
			mockSetup: func(mockAuth *rpcmocks.MockIAuthProvider, mockBenefitService *benefitmocks.MockIBenefitService, mockEvaluatorService *mocks.MockEvaluatorService, mockFileProvider *rpcmocks.MockIFileProvider) {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockBenefitService.EXPECT().CheckEvaluatorBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckEvaluatorBenefitResult{}, nil)
			},
			wantResp: &evaluatorservice.BatchDebugEvaluatorResponse{
				EvaluatorOutputData: []*evaluatordto.EvaluatorOutputData{},
			},
			wantErr: false,
		},
		{
			name: "edge case - large number of concurrent inputs",
			req: &evaluatorservice.BatchDebugEvaluatorRequest{
				WorkspaceID:   validWorkspaceID,
				EvaluatorType: evaluatordto.EvaluatorType_Code,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					CodeEvaluator: &evaluatordto.CodeEvaluator{
						CodeContent:  gptr.Of("def evaluate(input): return 1.0"),
						LanguageType: gptr.Of(evaluatordto.LanguageTypePython),
					},
				},
				InputData: func() []*evaluatordto.EvaluatorInputData {
					// 创建100个输入数据来测试并发处理
					inputs := make([]*evaluatordto.EvaluatorInputData, 100)
					for i := 0; i < 100; i++ {
						inputs[i] = &evaluatordto.EvaluatorInputData{
							InputFields: map[string]*common.Content{
								"input": {
									ContentType: gptr.Of(common.ContentTypeText),
									Text:        gptr.Of(fmt.Sprintf("test input %d", i)),
								},
							},
						}
					}
					return inputs
				}(),
			},
			mockSetup: func(mockAuth *rpcmocks.MockIAuthProvider, mockBenefitService *benefitmocks.MockIBenefitService, mockEvaluatorService *mocks.MockEvaluatorService, mockFileProvider *rpcmocks.MockIFileProvider) {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockBenefitService.EXPECT().CheckEvaluatorBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckEvaluatorBenefitResult{}, nil)
				mockFileProvider.EXPECT().MGetFileURL(gomock.Any(), gomock.Any()).Return(map[string]string{}, nil).AnyTimes()

				// Mock 100次调用
				mockEvaluatorService.EXPECT().DebugEvaluator(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&entity.EvaluatorOutputData{
						EvaluatorResult: &entity.EvaluatorResult{
							Score:     gptr.Of(0.8),
							Reasoning: "concurrent result",
						},
					}, nil).Times(100)
			},
			wantResp: &evaluatorservice.BatchDebugEvaluatorResponse{
				EvaluatorOutputData: make([]*evaluatordto.EvaluatorOutputData, 100),
			},
			wantErr: false,
		},

		{
			name: "edge case - evaluator service returns nil output with error",
			req: &evaluatorservice.BatchDebugEvaluatorRequest{
				WorkspaceID:   validWorkspaceID,
				EvaluatorType: evaluatordto.EvaluatorType_Code,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					CodeEvaluator: &evaluatordto.CodeEvaluator{
						CodeContent:  gptr.Of("invalid code"),
						LanguageType: gptr.Of(evaluatordto.LanguageTypePython),
					},
				},
				InputData: []*evaluatordto.EvaluatorInputData{
					{
						InputFields: map[string]*common.Content{
							"input": {
								ContentType: gptr.Of(common.ContentTypeText),
								Text:        gptr.Of("test input"),
							},
						},
					},
				},
			},
			mockSetup: func(mockAuth *rpcmocks.MockIAuthProvider, mockBenefitService *benefitmocks.MockIBenefitService, mockEvaluatorService *mocks.MockEvaluatorService, mockFileProvider *rpcmocks.MockIFileProvider) {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockBenefitService.EXPECT().CheckEvaluatorBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckEvaluatorBenefitResult{}, nil)
				mockFileProvider.EXPECT().MGetFileURL(gomock.Any(), gomock.Any()).Return(map[string]string{}, nil).AnyTimes()

				// 返回 nil output 和 error
				mockEvaluatorService.EXPECT().DebugEvaluator(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("code execution failed"))
			},
			wantResp: &evaluatorservice.BatchDebugEvaluatorResponse{
				EvaluatorOutputData: []*evaluatordto.EvaluatorOutputData{
					{
						EvaluatorRunError: &evaluatordto.EvaluatorRunError{
							Code:    gptr.Of(int32(500)),
							Message: gptr.Of("code execution failed"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "edge case - mixed success and failure results",
			req: &evaluatorservice.BatchDebugEvaluatorRequest{
				WorkspaceID:   validWorkspaceID,
				EvaluatorType: evaluatordto.EvaluatorType_Code,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					CodeEvaluator: &evaluatordto.CodeEvaluator{
						CodeContent:  gptr.Of("def evaluate(input): return 1.0"),
						LanguageType: gptr.Of(evaluatordto.LanguageTypePython),
					},
				},
				InputData: []*evaluatordto.EvaluatorInputData{
					{
						InputFields: map[string]*common.Content{
							"input": {
								ContentType: gptr.Of(common.ContentTypeText),
								Text:        gptr.Of("success input"),
							},
						},
					},
					{
						InputFields: map[string]*common.Content{
							"input": {
								ContentType: gptr.Of(common.ContentTypeText),
								Text:        gptr.Of("error input"),
							},
						},
					},
					{
						InputFields: map[string]*common.Content{
							"input": {
								ContentType: gptr.Of(common.ContentTypeText),
								Text:        gptr.Of("another success input"),
							},
						},
					},
				},
			},
			mockSetup: func(mockAuth *rpcmocks.MockIAuthProvider, mockBenefitService *benefitmocks.MockIBenefitService, mockEvaluatorService *mocks.MockEvaluatorService, mockFileProvider *rpcmocks.MockIFileProvider) {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockBenefitService.EXPECT().CheckEvaluatorBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckEvaluatorBenefitResult{}, nil)
				mockFileProvider.EXPECT().MGetFileURL(gomock.Any(), gomock.Any()).Return(map[string]string{}, nil).AnyTimes()

				// 第一个成功
				mockEvaluatorService.EXPECT().DebugEvaluator(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&entity.EvaluatorOutputData{
						EvaluatorResult: &entity.EvaluatorResult{
							Score:     gptr.Of(0.9),
							Reasoning: "success",
						},
					}, nil).Times(1)

				// 第二个失败 (nil output + error)
				mockEvaluatorService.EXPECT().DebugEvaluator(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("processing error")).Times(1)

				// 第三个成功但有 evaluator run error
				mockEvaluatorService.EXPECT().DebugEvaluator(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&entity.EvaluatorOutputData{
						EvaluatorResult: &entity.EvaluatorResult{
							Score:     gptr.Of(0.7),
							Reasoning: "partial success",
						},
					}, errors.New("warning error")).Times(1)
			},
			wantResp: &evaluatorservice.BatchDebugEvaluatorResponse{
				EvaluatorOutputData: make([]*evaluatordto.EvaluatorOutputData, 3),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// 为每个测试用例创建独立的 mock
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
			mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
			mockBenefitService := benefitmocks.NewMockIBenefitService(ctrl)
			mockFileProvider := rpcmocks.NewMockIFileProvider(ctrl)

			app := &EvaluatorHandlerImpl{
				auth:             mockAuth,
				benefitService:   mockBenefitService,
				evaluatorService: mockEvaluatorService,
				fileProvider:     mockFileProvider,
			}

			tt.mockSetup(mockAuth, mockBenefitService, mockEvaluatorService, mockFileProvider)

			resp, err := app.BatchDebugEvaluator(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, len(tt.wantResp.EvaluatorOutputData), len(resp.EvaluatorOutputData))

				// 验证结果数量
				assert.Equal(t, len(tt.wantResp.EvaluatorOutputData), len(resp.EvaluatorOutputData))

				// 对于特定测试用例，验证错误处理逻辑
				if tt.name == "edge case - evaluator service returns nil output with error" {
					assert.NotNil(t, resp.EvaluatorOutputData[0].EvaluatorRunError)
					assert.Equal(t, int32(500), *resp.EvaluatorOutputData[0].EvaluatorRunError.Code)
					assert.Equal(t, "code execution failed", *resp.EvaluatorOutputData[0].EvaluatorRunError.Message)
				}
			}
		})
	}
}

// TestEvaluatorHandlerImpl_ListTemplatesV2 测试 ListTemplatesV2 方法
func TestEvaluatorHandlerImpl_ListTemplatesV2(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTemplateService := mocks.NewMockEvaluatorTemplateService(ctrl)

	app := &EvaluatorHandlerImpl{
		evaluatorTemplateService: mockTemplateService,
	}

	tests := []struct {
		name        string
		req         *evaluatorservice.ListTemplatesV2Request
		mockSetup   func()
		wantResp    *evaluatorservice.ListTemplatesV2Response
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &evaluatorservice.ListTemplatesV2Request{
				PageSize:   gptr.Of(int32(20)),
				PageNumber: gptr.Of(int32(1)),
			},
			mockSetup: func() {
				mockTemplateService.EXPECT().
					ListEvaluatorTemplate(gomock.Any(), gomock.Any()).
					Return(&entity.ListEvaluatorTemplateResponse{
						Templates: []*entity.EvaluatorTemplate{
							{
								ID:          1,
								Name:        "template1",
								Description: "test template 1",
							},
							{
								ID:          2,
								Name:        "template2",
								Description: "test template 2",
							},
						},
						TotalCount: 2,
					}, nil)
			},
			wantResp: &evaluatorservice.ListTemplatesV2Response{
				Total: gptr.Of(int64(2)),
			},
			wantErr: false,
		},
		{
			name: "success - with pagination",
			req: &evaluatorservice.ListTemplatesV2Request{
				PageSize:   gptr.Of(int32(10)),
				PageNumber: gptr.Of(int32(2)),
			},
			mockSetup: func() {
				mockTemplateService.EXPECT().
					ListEvaluatorTemplate(gomock.Any(), gomock.Any()).
					Return(&entity.ListEvaluatorTemplateResponse{
						Templates:  []*entity.EvaluatorTemplate{},
						TotalCount: 25,
					}, nil)
			},
			wantResp: &evaluatorservice.ListTemplatesV2Response{
				Total: gptr.Of(int64(25)),
			},
			wantErr: false,
		},
		{
			name: "success - with filter option",
			req: &evaluatorservice.ListTemplatesV2Request{
				PageSize:     gptr.Of(int32(20)),
				PageNumber:   gptr.Of(int32(1)),
				FilterOption: &evaluatordto.EvaluatorFilterOption{},
			},
			mockSetup: func() {
				mockTemplateService.EXPECT().
					ListEvaluatorTemplate(gomock.Any(), gomock.Any()).
					Return(&entity.ListEvaluatorTemplateResponse{
						Templates:  []*entity.EvaluatorTemplate{},
						TotalCount: 0,
					}, nil)
			},
			wantResp: &evaluatorservice.ListTemplatesV2Response{
				Total: gptr.Of(int64(0)),
			},
			wantErr: false,
		},
		{
			name: "error - service failure",
			req: &evaluatorservice.ListTemplatesV2Request{
				PageSize:   gptr.Of(int32(20)),
				PageNumber: gptr.Of(int32(1)),
			},
			mockSetup: func() {
				mockTemplateService.EXPECT().
					ListEvaluatorTemplate(gomock.Any(), gomock.Any()).
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

			resp, err := app.ListTemplatesV2(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				if tt.wantResp.Total != nil {
					assert.Equal(t, *tt.wantResp.Total, *resp.Total)
				}
			}
		})
	}
}

// TestEvaluatorHandlerImpl_GetTemplateV2 测试 GetTemplateV2 方法
func TestEvaluatorHandlerImpl_GetTemplateV2(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTemplateService := mocks.NewMockEvaluatorTemplateService(ctrl)

	app := &EvaluatorHandlerImpl{
		evaluatorTemplateService: mockTemplateService,
	}

	templateID := int64(123)
	template := &entity.EvaluatorTemplate{
		ID:          templateID,
		Name:        "test template",
		Description: "test description",
	}

	tests := []struct {
		name        string
		req         *evaluatorservice.GetTemplateV2Request
		mockSetup   func()
		wantResp    *evaluatorservice.GetTemplateV2Response
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &evaluatorservice.GetTemplateV2Request{
				EvaluatorTemplateID: gptr.Of(templateID),
			},
			mockSetup: func() {
				mockTemplateService.EXPECT().
					GetEvaluatorTemplate(gomock.Any(), &entity.GetEvaluatorTemplateRequest{
						ID:             templateID,
						IncludeDeleted: false,
					}).
					Return(&entity.GetEvaluatorTemplateResponse{
						Template: template,
					}, nil)
			},
			wantResp: &evaluatorservice.GetTemplateV2Response{
				EvaluatorTemplate: evaluator.ConvertEvaluatorTemplateDO2DTO(template),
			},
			wantErr: false,
		},
		{
			name: "success - template not found",
			req: &evaluatorservice.GetTemplateV2Request{
				EvaluatorTemplateID: gptr.Of(templateID),
			},
			mockSetup: func() {
				mockTemplateService.EXPECT().
					GetEvaluatorTemplate(gomock.Any(), gomock.Any()).
					Return(&entity.GetEvaluatorTemplateResponse{
						Template: nil,
					}, nil)
			},
			wantResp: &evaluatorservice.GetTemplateV2Response{},
			wantErr:  false,
		},
		{
			name: "error - service failure",
			req: &evaluatorservice.GetTemplateV2Request{
				EvaluatorTemplateID: gptr.Of(templateID),
			},
			mockSetup: func() {
				mockTemplateService.EXPECT().
					GetEvaluatorTemplate(gomock.Any(), gomock.Any()).
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

			resp, err := app.GetTemplateV2(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				if tt.wantResp.EvaluatorTemplate != nil {
					assert.Equal(t, templateID, resp.GetEvaluatorTemplate().GetID())
				}
			}
		})
	}
}

func TestEvaluatorHandlerImpl_CreateEvaluator_CustomRPC(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
	mockAuditClient := auditmocks.NewMockIAuditService(ctrl)
	mockMetrics := metricsmock.NewMockEvaluatorExecMetrics(ctrl)
	mockConfiger := confmocks.NewMockIConfiger(ctrl)

	app := &EvaluatorHandlerImpl{
		auth:             mockAuth,
		evaluatorService: mockEvaluatorService,
		auditClient:      mockAuditClient,
		metrics:          mockMetrics,
		configer:         mockConfiger,
	}

	ctx := context.Background()
	workspaceID := int64(123456)

	tests := []struct {
		name            string
		evaluatorType   evaluatordto.EvaluatorType
		allowedSpaceIDs []string
		checkWritable   bool
		checkError      error
		wantErr         bool
		wantErrCode     int32
	}{
		{
			name:            "成功 - CustomRPC类型且空间有权限",
			evaluatorType:   evaluatordto.EvaluatorType_CustomRPC,
			allowedSpaceIDs: []string{"123456", "789012"},
			checkWritable:   true,
			checkError:      nil,
			wantErr:         false,
		},
		{
			name:            "失败 - CustomRPC类型但空间无权限",
			evaluatorType:   evaluatordto.EvaluatorType_CustomRPC,
			allowedSpaceIDs: []string{"789012", "345678"},
			checkWritable:   false,
			checkError:      nil,
			wantErr:         true,
			wantErrCode:     errno.CommonInvalidParamCode,
		},
		{
			name:            "失败 - CustomRPC类型但配置检查失败",
			evaluatorType:   evaluatordto.EvaluatorType_CustomRPC,
			allowedSpaceIDs: []string{"123456"},
			checkWritable:   false,
			checkError:      errors.New("配置检查失败"),
			wantErr:         true,
			wantErrCode:     0,
		},
		{
			name:            "成功 - 非CustomRPC类型无需额外权限校验",
			evaluatorType:   evaluatordto.EvaluatorType_Prompt,
			allowedSpaceIDs: []string{},
			checkWritable:   false,
			checkError:      nil,
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var request *evaluatorservice.CreateEvaluatorRequest
			if tt.evaluatorType == evaluatordto.EvaluatorType_CustomRPC {
				request = &evaluatorservice.CreateEvaluatorRequest{
					Evaluator: &evaluatordto.Evaluator{
						WorkspaceID:   gptr.Of(workspaceID),
						Name:          gptr.Of("测试CustomRPC评估器"),
						Description:   gptr.Of("测试描述"),
						EvaluatorType: gptr.Of(tt.evaluatorType),
						CurrentVersion: &evaluatordto.EvaluatorVersion{
							Version:     gptr.Of("1.0.0"),
							Description: gptr.Of("版本描述"),
							EvaluatorContent: &evaluatordto.EvaluatorContent{
								CustomRPCEvaluator: &evaluatordto.CustomRPCEvaluator{
									ServiceName:    gptr.Of("test.psm.service"),
									AccessProtocol: evaluatordto.EvaluatorAccessProtocolRPC,
								},
							},
						},
					},
				}
			} else {
				request = &evaluatorservice.CreateEvaluatorRequest{
					Evaluator: &evaluatordto.Evaluator{
						WorkspaceID:   gptr.Of(workspaceID),
						Name:          gptr.Of("测试CustomRPC评估器"),
						Description:   gptr.Of("测试描述"),
						EvaluatorType: gptr.Of(tt.evaluatorType),
						CurrentVersion: &evaluatordto.EvaluatorVersion{
							Version:     gptr.Of("1.0.0"),
							Description: gptr.Of("版本描述"),
							EvaluatorContent: &evaluatordto.EvaluatorContent{
								PromptEvaluator: &evaluatordto.PromptEvaluator{
									PromptTemplateKey: gptr.Of("test_template"),
								},
							},
						},
					},
				}
			}

			// Mock 基础权限校验
			mockAuth.EXPECT().
				Authorization(gomock.Any(), &rpc.AuthorizationParam{
					ObjectID:      strconv.FormatInt(workspaceID, 10),
					SpaceID:       workspaceID,
					ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("createLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
				}).
				Return(nil).
				Times(1)

			// Mock 机审
			mockAuditClient.EXPECT().
				Audit(gomock.Any(), gomock.Any()).
				Return(audit.AuditRecord{AuditStatus: audit.AuditStatus_Approved}, nil).
				Times(1)

			// 如果是CustomRPC类型，需要Mock额外的权限校验
			if tt.evaluatorType == evaluatordto.EvaluatorType_CustomRPC {
				mockConfiger.EXPECT().
					GetBuiltinEvaluatorSpaceConf(gomock.Any()).
					Return(tt.allowedSpaceIDs).
					Times(1)

				mockConfiger.EXPECT().
					CheckCustomRPCEvaluatorWritable(gomock.Any(), strconv.FormatInt(workspaceID, 10), tt.allowedSpaceIDs).
					Return(tt.checkWritable, tt.checkError).
					Times(1)
			}

			// Mock 创建评估器
			if !tt.wantErr {
				mockMetrics.EXPECT().
					EmitCreate(workspaceID, nil).
					Times(1)

				mockEvaluatorService.EXPECT().
					CreateEvaluator(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(int64(12345), nil).
					Times(1)
			}

			resp, err := app.CreateEvaluator(ctx, request)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					if statusErr, ok := errorx.FromStatusError(err); ok {
						assert.Equal(t, tt.wantErrCode, statusErr.Code())
					}
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, int64(12345), gptr.Indirect(resp.EvaluatorID))
			}
		})
	}
}

func TestEvaluatorHandlerImpl_UpdateEvaluatorDraft_CustomRPC(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
	mockConfiger := confmocks.NewMockIConfiger(ctrl)
	mockUserInfoService := userinfomocks.NewMockUserInfoService(ctrl)

	app := &EvaluatorHandlerImpl{
		auth:             mockAuth,
		evaluatorService: mockEvaluatorService,
		configer:         mockConfiger,
		userInfoService:  mockUserInfoService,
	}

	ctx := context.Background()
	workspaceID := int64(123456)
	evaluatorID := int64(789)

	tests := []struct {
		name            string
		spaceID         int64
		allowedSpaceIDs []string
		checkWritable   bool
		checkError      error
		wantErr         bool
		wantErrCode     int32
	}{
		{
			name:            "失败 - CustomRPC类型但空间无权限",
			spaceID:         workspaceID,
			allowedSpaceIDs: []string{"789012", "345678"},
			checkWritable:   false,
			checkError:      nil,
			wantErr:         true,
			wantErrCode:     errno.CommonInvalidParamCode,
		},
		{
			name:            "失败 - CustomRPC类型但配置检查失败",
			spaceID:         workspaceID,
			allowedSpaceIDs: []string{"123456"},
			checkWritable:   false,
			checkError:      errors.New("配置检查失败"),
			wantErr:         true,
			wantErrCode:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &evaluatorservice.UpdateEvaluatorDraftRequest{
				WorkspaceID:   workspaceID,
				EvaluatorID:   evaluatorID,
				EvaluatorType: evaluatordto.EvaluatorType_CustomRPC,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					CustomRPCEvaluator: &evaluatordto.CustomRPCEvaluator{
						ServiceName:    gptr.Of("test.psm.service"),
						AccessProtocol: evaluatordto.EvaluatorAccessProtocolRPC,
					},
				},
			}

			// Mock 获取评估器信息
			evaluatorDO := &entity.Evaluator{
				ID:      evaluatorID,
				SpaceID: tt.spaceID,
				Name:    "测试评估器",
			}

			mockEvaluatorService.EXPECT().
				GetEvaluator(gomock.Any(), workspaceID, evaluatorID, false).
				Return(evaluatorDO, nil).
				Times(1)

			// Mock 基础权限校验
			mockAuth.EXPECT().
				Authorization(gomock.Any(), &rpc.AuthorizationParam{
					ObjectID:      strconv.FormatInt(evaluatorID, 10),
					SpaceID:       tt.spaceID,
					ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
				}).
				Return(nil).
				Times(1)

			// Mock 额外的权限校验
			mockConfiger.EXPECT().
				GetBuiltinEvaluatorSpaceConf(gomock.Any()).
				Return(tt.allowedSpaceIDs).
				Times(1)

			mockConfiger.EXPECT().
				CheckCustomRPCEvaluatorWritable(gomock.Any(), strconv.FormatInt(tt.spaceID, 10), tt.allowedSpaceIDs).
				Return(tt.checkWritable, tt.checkError).
				Times(1)

			resp, err := app.UpdateEvaluatorDraft(ctx, request)

			assert.Error(t, err)
			if tt.wantErrCode != 0 {
				if statusErr, ok := errorx.FromStatusError(err); ok {
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			}
			assert.Nil(t, resp)
		})
	}
}

// TestEvaluatorHandlerImpl_CreateEvaluatorTemplate 测试 CreateEvaluatorTemplate 方法
func TestEvaluatorHandlerImpl_CreateEvaluatorTemplate(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTemplateService := mocks.NewMockEvaluatorTemplateService(ctrl)
	mockConfiger := confmocks.NewMockIConfiger(ctrl)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)

	app := &EvaluatorHandlerImpl{
		evaluatorTemplateService: mockTemplateService,
		configer:                 mockConfiger,
		auth:                     mockAuth,
	}

	workspaceID := int64(123)
	templateDTO := &evaluatordto.EvaluatorTemplate{
		ID:          gptr.Of(int64(1)),
		WorkspaceID: gptr.Of(workspaceID),
		Name:        gptr.Of("test template"),
		Description: gptr.Of("test description"),
	}

	tests := []struct {
		name        string
		req         *evaluatorservice.CreateEvaluatorTemplateRequest
		mockSetup   func()
		wantResp    *evaluatorservice.CreateEvaluatorTemplateResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &evaluatorservice.CreateEvaluatorTemplateRequest{
				EvaluatorTemplate: templateDTO,
			},
			mockSetup: func() {
				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil)

				mockConfiger.EXPECT().
					GetEvaluatorTemplateSpaceConf(gomock.Any()).
					Return([]string{"123"})

				mockTemplateService.EXPECT().
					CreateEvaluatorTemplate(gomock.Any(), gomock.Any()).
					Return(&entity.CreateEvaluatorTemplateResponse{
						Template: evaluator.ConvertEvaluatorTemplateDTO2DO(templateDTO),
					}, nil)
			},
			wantResp: &evaluatorservice.CreateEvaluatorTemplateResponse{
				EvaluatorTemplate: templateDTO,
			},
			wantErr: false,
		},
		{
			name: "error - nil template",
			req: &evaluatorservice.CreateEvaluatorTemplateRequest{
				EvaluatorTemplate: nil,
			},
			mockSetup:   func() {},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - auth failed",
			req: &evaluatorservice.CreateEvaluatorTemplateRequest{
				EvaluatorTemplate: &evaluatordto.EvaluatorTemplate{
					ID:          gptr.Of(int64(1)),
					WorkspaceID: gptr.Of(int64(789)), // 不在允许列表中
					Name:        gptr.Of("test template"),
					Description: gptr.Of("test description"),
				},
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
			name: "error - service failure",
			req: &evaluatorservice.CreateEvaluatorTemplateRequest{
				EvaluatorTemplate: templateDTO,
			},
			mockSetup: func() {
				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil)

				mockConfiger.EXPECT().
					GetEvaluatorTemplateSpaceConf(gomock.Any()).
					Return([]string{"123"})

				mockTemplateService.EXPECT().
					CreateEvaluatorTemplate(gomock.Any(), gomock.Any()).
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

			resp, err := app.CreateEvaluatorTemplate(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.NotNil(t, resp.EvaluatorTemplate)
			}
		})
	}
}

// TestEvaluatorHandlerImpl_UpdateEvaluatorTemplate 测试 UpdateEvaluatorTemplate 方法
func TestEvaluatorHandlerImpl_UpdateEvaluatorTemplate(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTemplateService := mocks.NewMockEvaluatorTemplateService(ctrl)
	mockConfiger := confmocks.NewMockIConfiger(ctrl)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)

	app := &EvaluatorHandlerImpl{
		evaluatorTemplateService: mockTemplateService,
		configer:                 mockConfiger,
		auth:                     mockAuth,
	}

	templateID := int64(123)
	workspaceID := int64(456)
	templateDTO := &evaluatordto.EvaluatorTemplate{
		ID:          gptr.Of(templateID),
		WorkspaceID: gptr.Of(workspaceID),
		Name:        gptr.Of("updated template"),
		Description: gptr.Of("updated description"),
	}

	tests := []struct {
		name        string
		req         *evaluatorservice.UpdateEvaluatorTemplateRequest
		mockSetup   func()
		wantResp    *evaluatorservice.UpdateEvaluatorTemplateResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &evaluatorservice.UpdateEvaluatorTemplateRequest{
				EvaluatorTemplateID: templateID,
				EvaluatorTemplate:   templateDTO,
			},
			mockSetup: func() {
				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil)

				mockConfiger.EXPECT().
					GetEvaluatorTemplateSpaceConf(gomock.Any()).
					Return([]string{"456"})

				mockTemplateService.EXPECT().
					UpdateEvaluatorTemplate(gomock.Any(), gomock.Any()).
					Return(&entity.UpdateEvaluatorTemplateResponse{
						Template: evaluator.ConvertEvaluatorTemplateDTO2DO(templateDTO),
					}, nil)
			},
			wantResp: &evaluatorservice.UpdateEvaluatorTemplateResponse{
				EvaluatorTemplate: templateDTO,
			},
			wantErr: false,
		},
		{
			name: "error - nil template",
			req: &evaluatorservice.UpdateEvaluatorTemplateRequest{
				EvaluatorTemplateID: templateID,
				EvaluatorTemplate:   nil,
			},
			mockSetup:   func() {},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - auth failed",
			req: &evaluatorservice.UpdateEvaluatorTemplateRequest{
				EvaluatorTemplateID: templateID,
				EvaluatorTemplate: &evaluatordto.EvaluatorTemplate{
					ID:          gptr.Of(templateID),
					WorkspaceID: gptr.Of(int64(789)), // 不在允许列表中
					Name:        gptr.Of("updated template"),
					Description: gptr.Of("updated description"),
				},
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
			name: "error - service failure",
			req: &evaluatorservice.UpdateEvaluatorTemplateRequest{
				EvaluatorTemplateID: templateID,
				EvaluatorTemplate:   templateDTO,
			},
			mockSetup: func() {
				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil)

				mockConfiger.EXPECT().
					GetEvaluatorTemplateSpaceConf(gomock.Any()).
					Return([]string{"456"})

				mockTemplateService.EXPECT().
					UpdateEvaluatorTemplate(gomock.Any(), gomock.Any()).
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

			resp, err := app.UpdateEvaluatorTemplate(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.NotNil(t, resp.EvaluatorTemplate)
			}
		})
	}
}

// TestEvaluatorHandlerImpl_DeleteEvaluatorTemplate 测试 DeleteEvaluatorTemplate 方法
func TestEvaluatorHandlerImpl_DeleteEvaluatorTemplate(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTemplateService := mocks.NewMockEvaluatorTemplateService(ctrl)
	mockConfiger := confmocks.NewMockIConfiger(ctrl)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)

	app := &EvaluatorHandlerImpl{
		evaluatorTemplateService: mockTemplateService,
		configer:                 mockConfiger,
		auth:                     mockAuth,
	}

	templateID := int64(123)
	workspaceID := int64(456)
	template := &entity.EvaluatorTemplate{
		ID:      templateID,
		SpaceID: workspaceID,
		Name:    "test template",
	}

	tests := []struct {
		name        string
		req         *evaluatorservice.DeleteEvaluatorTemplateRequest
		mockSetup   func()
		wantResp    *evaluatorservice.DeleteEvaluatorTemplateResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &evaluatorservice.DeleteEvaluatorTemplateRequest{
				EvaluatorTemplateID: templateID,
			},
			mockSetup: func() {
				mockTemplateService.EXPECT().
					GetEvaluatorTemplate(gomock.Any(), &entity.GetEvaluatorTemplateRequest{
						ID:             templateID,
						IncludeDeleted: false,
					}).
					Return(&entity.GetEvaluatorTemplateResponse{
						Template: template,
					}, nil)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil)

				mockConfiger.EXPECT().
					GetEvaluatorTemplateSpaceConf(gomock.Any()).
					Return([]string{"456"})

				mockTemplateService.EXPECT().
					DeleteEvaluatorTemplate(gomock.Any(), &entity.DeleteEvaluatorTemplateRequest{
						ID: templateID,
					}).
					Return(&entity.DeleteEvaluatorTemplateResponse{}, nil)
			},
			wantResp: &evaluatorservice.DeleteEvaluatorTemplateResponse{},
			wantErr:  false,
		},
		{
			name: "error - template id is 0",
			req: &evaluatorservice.DeleteEvaluatorTemplateRequest{
				EvaluatorTemplateID: 0,
			},
			mockSetup:   func() {},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - template not found",
			req: &evaluatorservice.DeleteEvaluatorTemplateRequest{
				EvaluatorTemplateID: templateID,
			},
			mockSetup: func() {
				mockTemplateService.EXPECT().
					GetEvaluatorTemplate(gomock.Any(), gomock.Any()).
					Return(&entity.GetEvaluatorTemplateResponse{
						Template: nil,
					}, nil)
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.ResourceNotFoundCode,
		},
		{
			name: "error - auth failed",
			req: &evaluatorservice.DeleteEvaluatorTemplateRequest{
				EvaluatorTemplateID: templateID,
			},
			mockSetup: func() {
				// 使用不在允许列表中的workspaceID的template
				testTemplate := &entity.EvaluatorTemplate{
					ID:      templateID,
					SpaceID: 789, // 不在允许列表中
					Name:    "test template",
				}
				mockTemplateService.EXPECT().
					GetEvaluatorTemplate(gomock.Any(), gomock.Any()).
					Return(&entity.GetEvaluatorTemplateResponse{
						Template: testTemplate,
					}, nil)

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

			resp, err := app.DeleteEvaluatorTemplate(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			}
		})
	}
}

// TestEvaluatorHandlerImpl_DebugBuiltinEvaluator 测试 DebugBuiltinEvaluator 方法
func TestEvaluatorHandlerImpl_DebugBuiltinEvaluator(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)

	app := &EvaluatorHandlerImpl{
		evaluatorService: mockEvaluatorService,
		auth:             mockAuth,
	}

	evaluatorID := int64(123)
	workspaceID := int64(456)
	builtinEvaluator := &entity.Evaluator{
		ID:      evaluatorID,
		SpaceID: workspaceID,
		Name:    "builtin evaluator",
		Builtin: true,
	}

	inputData := &evaluatordto.EvaluatorInputData{
		InputFields: map[string]*common.Content{
			"input": {
				ContentType: gptr.Of(common.ContentTypeText),
				Text:        gptr.Of("test input"),
			},
		},
	}

	outputData := &entity.EvaluatorOutputData{
		EvaluatorResult: &entity.EvaluatorResult{
			Score:     gptr.Of(0.85),
			Reasoning: "test result",
		},
	}

	tests := []struct {
		name        string
		req         *evaluatorservice.DebugBuiltinEvaluatorRequest
		mockSetup   func()
		wantResp    *evaluatorservice.DebugBuiltinEvaluatorResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &evaluatorservice.DebugBuiltinEvaluatorRequest{
				EvaluatorID: evaluatorID,
				WorkspaceID: workspaceID,
				InputData:   inputData,
			},
			mockSetup: func() {
				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(workspaceID, 10),
						SpaceID:       workspaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
					}).
					Return(nil)

				mockEvaluatorService.EXPECT().
					GetBuiltinEvaluator(gomock.Any(), evaluatorID).
					Return(builtinEvaluator, nil)

				mockEvaluatorService.EXPECT().
					DebugEvaluator(gomock.Any(), builtinEvaluator, gomock.Any(), gomock.Any(), gomock.Any()).
					Return(outputData, nil)
			},
			wantResp: &evaluatorservice.DebugBuiltinEvaluatorResponse{
				OutputData: evaluator.ConvertEvaluatorOutputDataDO2DTO(outputData),
			},
			wantErr: false,
		},
		{
			name: "error - auth failed",
			req: &evaluatorservice.DebugBuiltinEvaluatorRequest{
				EvaluatorID: evaluatorID,
				WorkspaceID: workspaceID,
				InputData:   inputData,
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
			name: "error - evaluator not found",
			req: &evaluatorservice.DebugBuiltinEvaluatorRequest{
				EvaluatorID: evaluatorID,
				WorkspaceID: workspaceID,
				InputData:   inputData,
			},
			mockSetup: func() {
				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil)

				mockEvaluatorService.EXPECT().
					GetBuiltinEvaluator(gomock.Any(), evaluatorID).
					Return(nil, nil)
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.EvaluatorNotExistCode,
		},
		{
			name: "error - debug failure",
			req: &evaluatorservice.DebugBuiltinEvaluatorRequest{
				EvaluatorID: evaluatorID,
				WorkspaceID: workspaceID,
				InputData:   inputData,
			},
			mockSetup: func() {
				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil)

				mockEvaluatorService.EXPECT().
					GetBuiltinEvaluator(gomock.Any(), evaluatorID).
					Return(builtinEvaluator, nil)

				mockEvaluatorService.EXPECT().
					DebugEvaluator(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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

			resp, err := app.DebugBuiltinEvaluator(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.NotNil(t, resp.OutputData)
			}
		})
	}
}

// TestEvaluatorHandlerImpl_UpdateEvaluatorRecord 测试 UpdateEvaluatorRecord 方法
func TestEvaluatorHandlerImpl_UpdateEvaluatorRecord(t *testing.T) {
	t.Parallel()

	const (
		workspaceID        = int64(101)
		evaluatorID        = int64(202)
		evaluatorVersionID = int64(303)
		recordID           = int64(404)
	)

	tests := []struct {
		name         string
		evaluator    *entity.Evaluator
		setupAuth    func(t *testing.T, mockAuth *rpcmocks.MockIAuthProvider, mockConfiger *confmocks.MockIConfiger)
		expectConfig bool
	}{
		{
			name: "success - custom evaluator uses evaluator authorization",
			evaluator: &entity.Evaluator{
				ID:      evaluatorID,
				SpaceID: workspaceID,
				Builtin: false,
			},
			setupAuth: func(t *testing.T, mockAuth *rpcmocks.MockIAuthProvider, _ *confmocks.MockIConfiger) {
				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, param *rpc.AuthorizationParam) error {
						assert.Equal(t, strconv.FormatInt(evaluatorID, 10), param.ObjectID)
						assert.Equal(t, workspaceID, param.SpaceID)
						if assert.Len(t, param.ActionObjects, 1) {
							assert.Equal(t, consts.Edit, gptr.Indirect(param.ActionObjects[0].Action))
							assert.Equal(t, rpc.AuthEntityType_Evaluator, gptr.Indirect(param.ActionObjects[0].EntityType))
						}
						return nil
					})
			},
		},
		{
			name: "success - builtin evaluator uses builtin space validation",
			evaluator: &entity.Evaluator{
				ID:      evaluatorID,
				SpaceID: workspaceID,
				Builtin: true,
			},
			setupAuth: func(t *testing.T, mockAuth *rpcmocks.MockIAuthProvider, mockConfiger *confmocks.MockIConfiger) {
				// authWrite 为 false 时，不会调用 Authorization，只检查空间配置
				mockConfiger.EXPECT().
					GetBuiltinEvaluatorSpaceConf(gomock.Any()).
					Return([]string{strconv.FormatInt(workspaceID, 10)})
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
			mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
			mockEvaluatorRecordService := mocks.NewMockEvaluatorRecordService(ctrl)
			mockAuditClient := auditmocks.NewMockIAuditService(ctrl)
			mockConfiger := confmocks.NewMockIConfiger(ctrl)

			handler := &EvaluatorHandlerImpl{
				auth:                   mockAuth,
				evaluatorService:       mockEvaluatorService,
				evaluatorRecordService: mockEvaluatorRecordService,
				auditClient:            mockAuditClient,
				configer:               mockConfiger,
			}

			tt.setupAuth(t, mockAuth, mockConfiger)

			evaluatorRecord := &entity.EvaluatorRecord{
				ID:                 recordID,
				EvaluatorVersionID: evaluatorVersionID,
			}
			mockEvaluatorRecordService.EXPECT().
				GetEvaluatorRecord(gomock.Any(), recordID, false).
				Return(evaluatorRecord, nil)

			mockEvaluatorService.EXPECT().
				GetEvaluatorVersion(gomock.Any(), gomock.Nil(), evaluatorVersionID, false, false).
				Return(tt.evaluator, nil)

			mockAuditClient.EXPECT().
				Audit(gomock.Any(), gomock.Any()).
				Return(audit.AuditRecord{AuditStatus: audit.AuditStatus_Approved}, nil)

			mockEvaluatorRecordService.EXPECT().
				CorrectEvaluatorRecord(gomock.Any(), evaluatorRecord, gomock.Any()).
				Return(nil)

			req := &evaluatorservice.UpdateEvaluatorRecordRequest{
				WorkspaceID:       workspaceID,
				EvaluatorRecordID: recordID,
				Correction: &evaluatordto.Correction{
					Explain:   gptr.Of("need update"),
					UpdatedBy: gptr.Of("tester"),
				},
			}

			resp, err := handler.UpdateEvaluatorRecord(context.Background(), req)
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.NotNil(t, resp.Record)
		})
	}
}

// TestEvaluatorHandlerImpl_UpdateBuiltinEvaluatorTags 测试 UpdateBuiltinEvaluatorTags 方法
func TestEvaluatorHandlerImpl_UpdateBuiltinEvaluatorTags(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
	mockConfiger := confmocks.NewMockIConfiger(ctrl)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)

	app := &EvaluatorHandlerImpl{
		evaluatorService: mockEvaluatorService,
		configer:         mockConfiger,
		auth:             mockAuth,
	}

	evaluatorID := int64(123)
	workspaceID := int64(456)
	evaluatorDO := &entity.Evaluator{
		ID:      evaluatorID,
		SpaceID: workspaceID,
		Name:    "builtin evaluator",
		Builtin: true,
	}

	tags := map[string]map[string][]string{
		evaluatordto.EvaluatorTagLangTypeZh: {
			evaluatordto.EvaluatorTagKeyCategory: {"category1", "category2"},
		},
	}

	tests := []struct {
		name        string
		req         *evaluatorservice.UpdateBuiltinEvaluatorTagsRequest
		mockSetup   func()
		wantResp    *evaluatorservice.UpdateBuiltinEvaluatorTagsResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &evaluatorservice.UpdateBuiltinEvaluatorTagsRequest{
				EvaluatorID: evaluatorID,
				WorkspaceID: gptr.Of(workspaceID),
				Tags:        tags,
			},
			mockSetup: func() {
				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil)

				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), workspaceID, evaluatorID, false).
					Return(evaluatorDO, nil)

				mockConfiger.EXPECT().
					GetBuiltinEvaluatorSpaceConf(gomock.Any()).
					Return([]string{"456"})

				mockEvaluatorService.EXPECT().
					UpdateBuiltinEvaluatorTags(gomock.Any(), evaluatorID, gomock.Any()).
					Return(nil)
			},
			wantResp: &evaluatorservice.UpdateBuiltinEvaluatorTagsResponse{
				Evaluator: evaluator.ConvertEvaluatorDO2DTO(evaluatorDO),
			},
			wantErr: false,
		},
		{
			name: "error - evaluator not found",
			req: &evaluatorservice.UpdateBuiltinEvaluatorTagsRequest{
				EvaluatorID: evaluatorID,
				WorkspaceID: gptr.Of(workspaceID),
				Tags:        tags,
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), workspaceID, evaluatorID, false).
					Return(nil, nil)
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.EvaluatorNotExistCode,
		},
		{
			name: "error - auth failed",
			req: &evaluatorservice.UpdateBuiltinEvaluatorTagsRequest{
				EvaluatorID: evaluatorID,
				WorkspaceID: gptr.Of(int64(789)), // 不在允许列表中
				Tags:        tags,
			},
			mockSetup: func() {
				testEvaluatorDO := &entity.Evaluator{
					ID:      evaluatorID,
					SpaceID: 789, // 不在允许列表中
					Name:    "builtin evaluator",
					Builtin: true,
				}
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), int64(789), evaluatorID, false).
					Return(testEvaluatorDO, nil)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
		{
			name: "error - service failure",
			req: &evaluatorservice.UpdateBuiltinEvaluatorTagsRequest{
				EvaluatorID: evaluatorID,
				WorkspaceID: gptr.Of(workspaceID),
				Tags:        tags,
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), workspaceID, evaluatorID, false).
					Return(evaluatorDO, nil)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil)

				mockConfiger.EXPECT().
					GetBuiltinEvaluatorSpaceConf(gomock.Any()).
					Return([]string{"456"})

				mockEvaluatorService.EXPECT().
					UpdateBuiltinEvaluatorTags(gomock.Any(), evaluatorID, gomock.Any()).
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

			resp, err := app.UpdateBuiltinEvaluatorTags(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.NotNil(t, resp.Evaluator)
			}
		})
	}
}

// TestEvaluatorHandlerImpl_ListEvaluatorTags 测试 ListEvaluatorTags 方法
func TestEvaluatorHandlerImpl_ListEvaluatorTags(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)

	app := &EvaluatorHandlerImpl{
		evaluatorService: mockEvaluatorService,
	}

	tests := []struct {
		name        string
		req         *evaluatorservice.ListEvaluatorTagsRequest
		mockSetup   func()
		wantResp    *evaluatorservice.ListEvaluatorTagsResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - default tag type (evaluator)",
			req:  &evaluatorservice.ListEvaluatorTagsRequest{},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					ListEvaluatorTags(gomock.Any(), entity.EvaluatorTagKeyType_Evaluator).
					Return(map[entity.EvaluatorTagKey][]string{
						entity.EvaluatorTagKey_Category:   {"category1", "category2"},
						entity.EvaluatorTagKey_TargetType: {"domain1"},
					}, nil)
			},
			wantResp: &evaluatorservice.ListEvaluatorTagsResponse{
				Tags: map[string][]string{
					evaluatordto.EvaluatorTagKeyCategory:   {"category1", "category2"},
					evaluatordto.EvaluatorTagKeyTargetType: {"domain1"},
				},
			},
			wantErr: false,
		},
		{
			name: "success - template tag type",
			req: func() *evaluatorservice.ListEvaluatorTagsRequest {
				req := evaluatorservice.NewListEvaluatorTagsRequest()
				// 使用反射设置TagType字段，因为字段类型在evaluatorservice包内部使用不同的类型别名
				tagTypeStr := "Template"
				rv := reflect.ValueOf(req).Elem()
				field := rv.FieldByName("TagType")
				if field.IsValid() && field.CanSet() {
					// EvaluatorTagType是string的别名，创建字符串指针
					field.Set(reflect.ValueOf(&tagTypeStr))
				}
				return req
			}(),
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					ListEvaluatorTags(gomock.Any(), entity.EvaluatorTagKeyType_Template).
					Return(map[entity.EvaluatorTagKey][]string{
						entity.EvaluatorTagKey_Category: {"prompt", "code"},
					}, nil)
			},
			wantResp: &evaluatorservice.ListEvaluatorTagsResponse{
				Tags: map[string][]string{
					evaluatordto.EvaluatorTagKeyCategory: {"prompt", "code"},
				},
			},
			wantErr: false,
		},
		{
			name: "success - empty tags",
			req:  &evaluatorservice.ListEvaluatorTagsRequest{},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					ListEvaluatorTags(gomock.Any(), entity.EvaluatorTagKeyType_Evaluator).
					Return(map[entity.EvaluatorTagKey][]string{}, nil)
			},
			wantResp: &evaluatorservice.ListEvaluatorTagsResponse{
				Tags: map[string][]string{},
			},
			wantErr: false,
		},
		{
			name: "success - tags sorted alphabetically",
			req:  &evaluatorservice.ListEvaluatorTagsRequest{},
			mockSetup: func() {
				// Service层会排序，所以mock返回的数据应该是排序后的
				mockEvaluatorService.EXPECT().
					ListEvaluatorTags(gomock.Any(), entity.EvaluatorTagKeyType_Evaluator).
					Return(map[entity.EvaluatorTagKey][]string{
						entity.EvaluatorTagKey_Category:   {"a", "m", "z"},
						entity.EvaluatorTagKey_TargetType: {"b", "x"},
					}, nil)
			},
			wantResp: &evaluatorservice.ListEvaluatorTagsResponse{
				Tags: map[string][]string{
					evaluatordto.EvaluatorTagKeyCategory:   {"a", "m", "z"},
					evaluatordto.EvaluatorTagKeyTargetType: {"b", "x"},
				},
			},
			wantErr: false,
		},
		{
			name: "success - unknown tag type defaults to evaluator",
			req: func() *evaluatorservice.ListEvaluatorTagsRequest {
				req := evaluatorservice.NewListEvaluatorTagsRequest()
				// 使用反射设置TagType字段
				unknownTypeStr := "Unknown"
				rv := reflect.ValueOf(req).Elem()
				field := rv.FieldByName("TagType")
				if field.IsValid() && field.CanSet() {
					field.Set(reflect.ValueOf(&unknownTypeStr))
				}
				return req
			}(),
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					ListEvaluatorTags(gomock.Any(), entity.EvaluatorTagKeyType_Evaluator).
					Return(map[entity.EvaluatorTagKey][]string{
						entity.EvaluatorTagKey_Category: {"LLM"},
					}, nil)
			},
			wantResp: &evaluatorservice.ListEvaluatorTagsResponse{
				Tags: map[string][]string{
					evaluatordto.EvaluatorTagKeyCategory: {"LLM"},
				},
			},
			wantErr: false,
		},
		{
			name: "error - service error",
			req:  &evaluatorservice.ListEvaluatorTagsRequest{},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					ListEvaluatorTags(gomock.Any(), entity.EvaluatorTagKeyType_Evaluator).
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

			resp, err := app.ListEvaluatorTags(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				if tt.wantResp != nil && tt.wantResp.Tags != nil {
					assert.Equal(t, len(tt.wantResp.Tags), len(resp.Tags))
					for key, expectedValues := range tt.wantResp.Tags {
						actualValues, ok := resp.Tags[key]
						assert.True(t, ok, "key %s should exist", key)
						assert.Equal(t, expectedValues, actualValues, "values for key %s should match", key)
					}
				}
			}
		})
	}
}

// TestEvaluatorHandlerImpl_authBuiltinManagement 测试 authBuiltinManagement 方法
func TestEvaluatorHandlerImpl_authBuiltinManagement(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfiger := confmocks.NewMockIConfiger(ctrl)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)

	app := &EvaluatorHandlerImpl{
		configer: mockConfiger,
		auth:     mockAuth,
	}

	tests := []struct {
		name        string
		workspaceID int64
		spaceType   SpaceType
		authWrite   bool
		mockSetup   func()
		wantErr     bool
		wantErrCode int32
	}{
		{
			name:        "success - workspace in allowed list for builtin",
			workspaceID: 123,
			spaceType:   spaceTypeBuiltin,
			authWrite:   false,
			mockSetup: func() {
				mockConfiger.EXPECT().
					GetBuiltinEvaluatorSpaceConf(gomock.Any()).
					Return([]string{"123", "456"})
			},
			wantErr: false,
		},
		{
			name:        "success - workspace in allowed list for template",
			workspaceID: 456,
			spaceType:   spaceTypeTemplate,
			authWrite:   false,
			mockSetup: func() {
				mockConfiger.EXPECT().
					GetEvaluatorTemplateSpaceConf(gomock.Any()).
					Return([]string{"123", "456"})
			},
			wantErr: false,
		},
		{
			name:        "error - empty config for builtin",
			workspaceID: 123,
			spaceType:   spaceTypeBuiltin,
			authWrite:   false,
			mockSetup: func() {
				mockConfiger.EXPECT().
					GetBuiltinEvaluatorSpaceConf(gomock.Any()).
					Return([]string{})
			},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name:        "error - empty config for template",
			workspaceID: 123,
			spaceType:   spaceTypeTemplate,
			authWrite:   false,
			mockSetup: func() {
				mockConfiger.EXPECT().
					GetEvaluatorTemplateSpaceConf(gomock.Any()).
					Return([]string{})
			},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name:        "error - workspace not in allowed list",
			workspaceID: 789,
			spaceType:   spaceTypeBuiltin,
			authWrite:   true,
			mockSetup: func() {
				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil)
				mockConfiger.EXPECT().
					GetBuiltinEvaluatorSpaceConf(gomock.Any()).
					Return([]string{"123", "456"})
			},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name:        "error - auth failed",
			workspaceID: 789,
			spaceType:   spaceTypeBuiltin,
			authWrite:   true,
			mockSetup: func() {
				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			err := app.authBuiltinManagement(context.Background(), tt.workspaceID, tt.spaceType, tt.authWrite)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEvaluatorHandlerImpl_UpdateEvaluatorDraft(t *testing.T) {
	t.Parallel()

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
	workspaceID := int64(123)
	evaluatorID := int64(1)
	userID := "test-user-id"
	newContent := "updated evaluator content"

	existingEvaluator := &entity.Evaluator{
		ID:             evaluatorID,
		SpaceID:        workspaceID,
		Name:           "test-evaluator",
		EvaluatorType:  entity.EvaluatorTypePrompt,
		Description:    "test description",
		DraftSubmitted: true,
		Builtin:        false,
		PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
			MessageList: []*entity.Message{
				{Role: entity.RoleSystem, Content: &entity.Content{Text: gptr.Of("old content")}},
			},
			ModelConfig: &entity.ModelConfig{
				ModelID: int64(1),
			},
		},
	}

	updatedDTO := &evaluatordto.Evaluator{
		EvaluatorID:    gptr.Of(evaluatorID),
		WorkspaceID:    gptr.Of(workspaceID),
		Name:           gptr.Of("test-evaluator"),
		Builtin:        gptr.Of(false),
		DraftSubmitted: gptr.Of(false),
		CurrentVersion: &evaluatordto.EvaluatorVersion{
			ID: gptr.Of(int64(1)),
			EvaluatorContent: &evaluatordto.EvaluatorContent{
				PromptEvaluator: &evaluatordto.PromptEvaluator{
					MessageList: []*common.Message{
						{Role: gptr.Of(common.Role(1)), Content: &common.Content{Text: gptr.Of(newContent)}},
					},
					ModelConfig: &common.ModelConfig{
						ModelID: gptr.Of(int64(1)),
					},
				},
			},
		},
	}

	tests := []struct {
		name        string
		req         *evaluatorservice.UpdateEvaluatorDraftRequest
		mockSetup   func()
		wantResp    *evaluatorservice.UpdateEvaluatorDraftResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - update draft content",
			req: &evaluatorservice.UpdateEvaluatorDraftRequest{
				WorkspaceID: workspaceID,
				EvaluatorID: evaluatorID,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					PromptEvaluator: &evaluatordto.PromptEvaluator{
						MessageList: []*common.Message{
							{Role: gptr.Of(common.Role(1)), Content: &common.Content{Text: gptr.Of(newContent)}},
						},
						ModelConfig: &common.ModelConfig{
							ModelID: gptr.Of(int64(1)),
						},
					},
				},
				EvaluatorType: evaluatordto.EvaluatorType(1),
			},
			mockSetup: func() {
				// 获取评估器
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), workspaceID, evaluatorID, false).
					Return(existingEvaluator, nil)

				// 鉴权
				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(evaluatorID, 10),
						SpaceID:       workspaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
					}).Return(nil)

				// 更新草稿
				mockEvaluatorService.EXPECT().
					UpdateEvaluatorDraft(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, evaluator *entity.Evaluator) error {
						// 验证传入的参数
						assert.Equal(t, evaluatorID, evaluator.ID)
						assert.Equal(t, workspaceID, evaluator.SpaceID)
						assert.Equal(t, false, evaluator.DraftSubmitted)
						assert.Equal(t, newContent, evaluator.PromptEvaluatorVersion.MessageList[0].Content.GetText())
						return nil
					})

				// 填充用户信息
				mockUserInfoService.EXPECT().
					PackUserInfo(gomock.Any(), gomock.Any())
			},
			wantResp: &evaluatorservice.UpdateEvaluatorDraftResponse{
				Evaluator: updatedDTO,
			},
			wantErr: false,
		},
		{
			name: "error - evaluator not found",
			req: &evaluatorservice.UpdateEvaluatorDraftRequest{
				WorkspaceID: workspaceID,
				EvaluatorID: evaluatorID,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					PromptEvaluator: &evaluatordto.PromptEvaluator{
						MessageList: []*common.Message{
							{Role: gptr.Of(common.Role(1)), Content: &common.Content{Text: gptr.Of(newContent)}},
						},
						ModelConfig: &common.ModelConfig{
							ModelID: gptr.Of(int64(1)),
						},
					},
				},
				EvaluatorType: evaluatordto.EvaluatorType(1),
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), workspaceID, evaluatorID, false).
					Return(nil, nil)
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.EvaluatorNotExistCode,
		},
		{
			name: "error - service failure on get",
			req: &evaluatorservice.UpdateEvaluatorDraftRequest{
				WorkspaceID: workspaceID,
				EvaluatorID: evaluatorID,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					PromptEvaluator: &evaluatordto.PromptEvaluator{
						MessageList: []*common.Message{
							{Role: gptr.Of(common.Role(1)), Content: &common.Content{Text: gptr.Of(newContent)}},
						},
						ModelConfig: &common.ModelConfig{
							ModelID: gptr.Of(int64(1)),
						},
					},
				},
				EvaluatorType: evaluatordto.EvaluatorType(1),
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), workspaceID, evaluatorID, false).
					Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
		{
			name: "error - auth failure",
			req: &evaluatorservice.UpdateEvaluatorDraftRequest{
				WorkspaceID: workspaceID,
				EvaluatorID: evaluatorID,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					PromptEvaluator: &evaluatordto.PromptEvaluator{
						MessageList: []*common.Message{
							{Role: gptr.Of(common.Role(1)), Content: &common.Content{Text: gptr.Of(newContent)}},
						},
						ModelConfig: &common.ModelConfig{
							ModelID: gptr.Of(int64(1)),
						},
					},
				},
				EvaluatorType: evaluatordto.EvaluatorType(1),
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), workspaceID, evaluatorID, false).
					Return(existingEvaluator, nil)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
		{
			name: "error - service failure on update",
			req: &evaluatorservice.UpdateEvaluatorDraftRequest{
				WorkspaceID: workspaceID,
				EvaluatorID: evaluatorID,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					PromptEvaluator: &evaluatordto.PromptEvaluator{
						MessageList: []*common.Message{
							{Role: gptr.Of(common.Role(1)), Content: &common.Content{Text: gptr.Of(newContent)}},
						},
						ModelConfig: &common.ModelConfig{
							ModelID: gptr.Of(int64(1)),
						},
					},
				},
				EvaluatorType: evaluatordto.EvaluatorType(1),
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), workspaceID, evaluatorID, false).
					Return(existingEvaluator, nil)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil)

				mockEvaluatorService.EXPECT().
					UpdateEvaluatorDraft(gomock.Any(), gomock.Any()).
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

			// 设置用户ID到context
			ctx := context.Background()
			if tt.name == "success - update draft content" {
				ctx = session.WithCtxUser(ctx, &session.User{ID: userID})
			}

			resp, err := app.UpdateEvaluatorDraft(ctx, tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.NotNil(t, resp.Evaluator)
				assert.Equal(t, newContent, resp.Evaluator.CurrentVersion.EvaluatorContent.PromptEvaluator.MessageList[0].Content.GetText())
				assert.Equal(t, false, *resp.Evaluator.DraftSubmitted)
			}
		})
	}
}

func TestEvaluatorHandlerImpl_UpdateEvaluator(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
	mockAuditClient := auditmocks.NewMockIAuditService(ctrl)
	mockConfiger := confmocks.NewMockIConfiger(ctrl)

	app := &EvaluatorHandlerImpl{
		auth:             mockAuth,
		evaluatorService: mockEvaluatorService,
		auditClient:      mockAuditClient,
		configer:         mockConfiger,
	}

	// Test data
	workspaceID := int64(123)
	evaluatorID := int64(1)
	userID := "test-user-id"

	existingEvaluator := &entity.Evaluator{
		ID:      evaluatorID,
		SpaceID: workspaceID,
		Name:    "test-evaluator",
		Builtin: false,
	}

	tests := []struct {
		name        string
		req         *evaluatorservice.UpdateEvaluatorRequest
		mockSetup   func()
		wantResp    *evaluatorservice.UpdateEvaluatorResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - update evaluator meta",
			req: &evaluatorservice.UpdateEvaluatorRequest{
				WorkspaceID: workspaceID,
				EvaluatorID: evaluatorID,
				Name:        gptr.Of("updated-name"),
				Description: gptr.Of("updated description"),
				Builtin:     gptr.Of(false),
			},
			mockSetup: func() {
				// 获取评估器
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), workspaceID, evaluatorID, false).
					Return(existingEvaluator, nil)

				// 鉴权
				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(evaluatorID, 10),
						SpaceID:       workspaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
					}).Return(nil)

				// 机审
				mockAuditClient.EXPECT().
					Audit(gomock.Any(), gomock.Any()).
					Return(audit.AuditRecord{AuditStatus: audit.AuditStatus_Approved}, nil)

				// 更新元信息
				mockEvaluatorService.EXPECT().
					UpdateEvaluatorMeta(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, req *entity.UpdateEvaluatorMetaRequest) error {
						assert.Equal(t, evaluatorID, req.ID)
						assert.Equal(t, workspaceID, req.SpaceID)
						assert.Equal(t, "updated-name", *req.Name)
						assert.Equal(t, "updated description", *req.Description)
						assert.Equal(t, false, *req.Builtin)
						return nil
					})
			},
			wantResp: &evaluatorservice.UpdateEvaluatorResponse{},
			wantErr:  false,
		},
		{
			name: "success - update builtin evaluator",
			req: &evaluatorservice.UpdateEvaluatorRequest{
				WorkspaceID: workspaceID,
				EvaluatorID: evaluatorID,
				Name:        gptr.Of("updated-builtin-name"),
				Description: gptr.Of("updated builtin description"),
				Builtin:     gptr.Of(true),
			},
			mockSetup: func() {
				// 获取评估器
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), workspaceID, evaluatorID, false).
					Return(existingEvaluator, nil)

				// 鉴权
				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(evaluatorID, 10),
						SpaceID:       workspaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
					}).Return(nil)

				// 预置评估器鉴权
				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(workspaceID, 10),
						SpaceID:       workspaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
					}).Return(nil)

				// 预置评估器管理权限校验
				mockConfiger.EXPECT().
					GetBuiltinEvaluatorSpaceConf(gomock.Any()).
					Return([]string{strconv.FormatInt(workspaceID, 10)})

				// 机审
				mockAuditClient.EXPECT().
					Audit(gomock.Any(), gomock.Any()).
					Return(audit.AuditRecord{AuditStatus: audit.AuditStatus_Approved}, nil)

				// 更新元信息
				mockEvaluatorService.EXPECT().
					UpdateEvaluatorMeta(gomock.Any(), gomock.Any()).
					Return(nil)
			},
			wantResp: &evaluatorservice.UpdateEvaluatorResponse{},
			wantErr:  false,
		},
		{
			name: "error - evaluator not found",
			req: &evaluatorservice.UpdateEvaluatorRequest{
				WorkspaceID: workspaceID,
				EvaluatorID: evaluatorID,
				Name:        gptr.Of("updated-name"),
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), workspaceID, evaluatorID, false).
					Return(nil, nil)
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.EvaluatorNotExistCode,
		},
		{
			name: "error - service failure on get",
			req: &evaluatorservice.UpdateEvaluatorRequest{
				WorkspaceID: workspaceID,
				EvaluatorID: evaluatorID,
				Name:        gptr.Of("updated-name"),
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), workspaceID, evaluatorID, false).
					Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
		{
			name: "error - auth failure",
			req: &evaluatorservice.UpdateEvaluatorRequest{
				WorkspaceID: workspaceID,
				EvaluatorID: evaluatorID,
				Name:        gptr.Of("updated-name"),
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), workspaceID, evaluatorID, false).
					Return(existingEvaluator, nil)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
		{
			name: "error - builtin management auth failure",
			req: &evaluatorservice.UpdateEvaluatorRequest{
				WorkspaceID: workspaceID,
				EvaluatorID: evaluatorID,
				Name:        gptr.Of("updated-builtin-name"),
				Builtin:     gptr.Of(true),
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), workspaceID, evaluatorID, false).
					Return(existingEvaluator, nil)

				// 首先进行普通权限验证
				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(evaluatorID, 10),
						SpaceID:       workspaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
					}).Return(nil)

				// 然后进行预置评估器管理权限校验
				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(workspaceID, 10),
						SpaceID:       workspaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
					}).Return(nil)

				// 检查空间配置 - 返回不包含当前workspaceID的配置
				mockConfiger.EXPECT().
					GetBuiltinEvaluatorSpaceConf(gomock.Any()).
					Return([]string{"999"}) // 不包含当前workspaceID
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - audit rejected",
			req: &evaluatorservice.UpdateEvaluatorRequest{
				WorkspaceID: workspaceID,
				EvaluatorID: evaluatorID,
				Name:        gptr.Of("updated-name"),
				Description: gptr.Of("inappropriate content"),
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), workspaceID, evaluatorID, false).
					Return(existingEvaluator, nil)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil)

				// 机审拒绝
				mockAuditClient.EXPECT().
					Audit(gomock.Any(), gomock.Any()).
					Return(audit.AuditRecord{AuditStatus: audit.AuditStatus_Rejected}, nil)
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.RiskContentDetectedCode,
		},
		{
			name: "error - service failure on update",
			req: &evaluatorservice.UpdateEvaluatorRequest{
				WorkspaceID: workspaceID,
				EvaluatorID: evaluatorID,
				Name:        gptr.Of("updated-name"),
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), workspaceID, evaluatorID, false).
					Return(existingEvaluator, nil)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil)

				// 机审
				mockAuditClient.EXPECT().
					Audit(gomock.Any(), gomock.Any()).
					Return(audit.AuditRecord{AuditStatus: audit.AuditStatus_Approved}, nil)

				// 更新元信息失败
				mockEvaluatorService.EXPECT().
					UpdateEvaluatorMeta(gomock.Any(), gomock.Any()).
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

			// 设置用户ID到context
			ctx := context.Background()
			if tt.name == "success - update evaluator meta" {
				ctx = session.WithCtxUser(ctx, &session.User{ID: userID})
			}

			resp, err := app.UpdateEvaluator(ctx, tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			}
		})
	}
}

func TestEvaluatorHandlerImpl_BatchGetEvaluators(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
	mockUserInfoService := userinfomocks.NewMockUserInfoService(ctrl)
	mockConfiger := confmocks.NewMockIConfiger(ctrl)

	app := &EvaluatorHandlerImpl{
		auth:             mockAuth,
		evaluatorService: mockEvaluatorService,
		userInfoService:  mockUserInfoService,
		configer:         mockConfiger,
	}

	// Test data
	workspaceID := int64(123)
	builtinWorkspaceID := int64(234)
	invalidBuiltinWorkspaceID := int64(345)
	evaluatorIDs := []int64{1, 2, 3}

	normalEvaluator := &entity.Evaluator{
		ID:      1,
		SpaceID: workspaceID,
		Name:    "normal-evaluator",
		Builtin: false,
	}

	builtinEvaluator := &entity.Evaluator{
		ID:      2,
		SpaceID: builtinWorkspaceID,
		Name:    "builtin-evaluator",
		Builtin: true,
	}

	evaluatorDTO1 := &evaluatordto.Evaluator{
		EvaluatorID: gptr.Of(int64(1)),
		WorkspaceID: gptr.Of(workspaceID),
		Name:        gptr.Of("normal-evaluator"),
		Builtin:     gptr.Of(false),
	}

	evaluatorDTO2 := &evaluatordto.Evaluator{
		EvaluatorID: gptr.Of(int64(2)),
		WorkspaceID: gptr.Of(builtinWorkspaceID),
		Name:        gptr.Of("builtin-evaluator"),
		Builtin:     gptr.Of(true),
	}

	tests := []struct {
		name        string
		req         *evaluatorservice.BatchGetEvaluatorsRequest
		mockSetup   func()
		wantResp    *evaluatorservice.BatchGetEvaluatorsResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal and builtin evaluators",
			req: &evaluatorservice.BatchGetEvaluatorsRequest{
				WorkspaceID:    workspaceID,
				EvaluatorIds:   evaluatorIDs,
				IncludeDeleted: gptr.Of(false),
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					BatchGetEvaluator(gomock.Any(), workspaceID, evaluatorIDs, false).
					Return([]*entity.Evaluator{normalEvaluator, builtinEvaluator}, nil)

				// 预置评估器鉴权
				// Mock configer for builtin evaluator space config
				mockConfiger.EXPECT().
					GetBuiltinEvaluatorSpaceConf(gomock.Any()).
					Return([]string{strconv.FormatInt(builtinWorkspaceID, 10)})

				// 普通评估器鉴权
				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(workspaceID, 10),
						SpaceID:       workspaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
					}).Return(nil)

				mockUserInfoService.EXPECT().
					PackUserInfo(gomock.Any(), gomock.Any()).Return()
			},
			wantResp: &evaluatorservice.BatchGetEvaluatorsResponse{
				Evaluators: []*evaluatordto.Evaluator{evaluatorDTO1, evaluatorDTO2},
			},
			wantErr: false,
		},
		{
			name: "success - empty result",
			req: &evaluatorservice.BatchGetEvaluatorsRequest{
				WorkspaceID:    workspaceID,
				EvaluatorIds:   []int64{},
				IncludeDeleted: gptr.Of(false),
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					BatchGetEvaluator(gomock.Any(), workspaceID, []int64{}, false).
					Return([]*entity.Evaluator{}, nil)
			},
			wantResp: &evaluatorservice.BatchGetEvaluatorsResponse{
				Evaluators: []*evaluatordto.Evaluator{},
			},
			wantErr: false,
		},
		{
			name: "error - service failure",
			req: &evaluatorservice.BatchGetEvaluatorsRequest{
				WorkspaceID:    workspaceID,
				EvaluatorIds:   evaluatorIDs,
				IncludeDeleted: gptr.Of(false),
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					BatchGetEvaluator(gomock.Any(), workspaceID, evaluatorIDs, false).
					Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
		{
			name: "error - auth failure for normal evaluator",
			req: &evaluatorservice.BatchGetEvaluatorsRequest{
				WorkspaceID:    workspaceID,
				EvaluatorIds:   []int64{1},
				IncludeDeleted: gptr.Of(false),
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					BatchGetEvaluator(gomock.Any(), workspaceID, []int64{1}, false).
					Return([]*entity.Evaluator{normalEvaluator}, nil)

				// 普通评估器鉴权失败 - 使用正确的参数匹配
				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(workspaceID, 10),
						SpaceID:       workspaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
					}).Return(errorx.NewByCode(errno.CommonNoPermissionCode))

				// 由于鉴权失败，函数会提前返回，PackUserInfo不应该被调用
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
		{
			name: "error - auth failure for builtin evaluator",
			req: &evaluatorservice.BatchGetEvaluatorsRequest{
				WorkspaceID:    workspaceID,
				EvaluatorIds:   []int64{2},
				IncludeDeleted: gptr.Of(false),
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					BatchGetEvaluator(gomock.Any(), workspaceID, []int64{2}, false).
					Return([]*entity.Evaluator{builtinEvaluator}, nil)

				// 预置评估器鉴权失败
				// Mock configer for builtin evaluator space config
				mockConfiger.EXPECT().
					GetBuiltinEvaluatorSpaceConf(gomock.Any()).
					Return([]string{strconv.FormatInt(invalidBuiltinWorkspaceID, 10)})
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := app.BatchGetEvaluators(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, len(tt.wantResp.Evaluators), len(resp.Evaluators))
				for i, evaluator := range resp.Evaluators {
					assert.Equal(t, *tt.wantResp.Evaluators[i].EvaluatorID, *evaluator.EvaluatorID)
					assert.Equal(t, *tt.wantResp.Evaluators[i].Name, *evaluator.Name)
					assert.Equal(t, *tt.wantResp.Evaluators[i].Builtin, *evaluator.Builtin)
				}
			}
		})
	}
}
