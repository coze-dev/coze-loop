// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"strconv"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	domain_eval_target "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_target"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/target"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestEvalTargetApplicationImpl_CreateEvalTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvalTargetService := mocks.NewMockIEvalTargetService(ctrl)

	app := &EvalTargetApplicationImpl{
		auth:              mockAuth,
		evalTargetService: mockEvalTargetService,
	}

	// Test data
	validSpaceID := int64(123)
	validSourceTargetID := "source-123"
	validSourceTargetVersion := "v1.0"
	validEvalTargetType := domain_eval_target.EvalTargetType(1)
	validBotInfoType := domain_eval_target.CozeBotInfoType(1)
	validBotPublishVersion := "publish-v1"

	tests := []struct {
		name        string
		req         *eval_target.CreateEvalTargetRequest
		mockSetup   func()
		wantResp    *eval_target.CreateEvalTargetResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &eval_target.CreateEvalTargetRequest{
				WorkspaceID: validSpaceID,
				Param: &eval_target.CreateEvalTargetParam{
					SourceTargetID:      &validSourceTargetID,
					SourceTargetVersion: &validSourceTargetVersion,
					EvalTargetType:      &validEvalTargetType,
					BotInfoType:         &validBotInfoType,
					BotPublishVersion:   &validBotPublishVersion,
				},
			},
			mockSetup: func() {
				// Mock auth
				mockAuth.EXPECT().Authorization(gomock.Any(), &rpc.AuthorizationParam{
					ObjectID:      strconv.FormatInt(validSpaceID, 10),
					SpaceID:       validSpaceID,
					ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("createLoopEvaluationTarget"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
				}).Return(nil)

				// Mock service call
				mockEvalTargetService.EXPECT().CreateEvalTarget(
					gomock.Any(),
					validSpaceID,
					validSourceTargetID,
					validSourceTargetVersion,
					gomock.Any(),
					gomock.Any(), // options
				).Return(int64(1), int64(2), nil)
			},
			wantResp: &eval_target.CreateEvalTargetResponse{
				ID:        gptr.Of(int64(1)),
				VersionID: gptr.Of(int64(2)),
			},
			wantErr: false,
		},
		{
			name:        "error - nil request",
			req:         nil,
			mockSetup:   func() {},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - nil param",
			req: &eval_target.CreateEvalTargetRequest{
				WorkspaceID: validSpaceID,
				Param:       nil,
			},
			mockSetup:   func() {},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - missing source target id",
			req: &eval_target.CreateEvalTargetRequest{
				WorkspaceID: validSpaceID,
				Param: &eval_target.CreateEvalTargetParam{
					SourceTargetVersion: &validSourceTargetVersion,
					EvalTargetType:      &validEvalTargetType,
				},
			},
			mockSetup:   func() {},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - missing source target version",
			req: &eval_target.CreateEvalTargetRequest{
				WorkspaceID: validSpaceID,
				Param: &eval_target.CreateEvalTargetParam{
					SourceTargetID: &validSourceTargetID,
					EvalTargetType: &validEvalTargetType,
				},
			},
			mockSetup:   func() {},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - missing eval target type",
			req: &eval_target.CreateEvalTargetRequest{
				WorkspaceID: validSpaceID,
				Param: &eval_target.CreateEvalTargetParam{
					SourceTargetID:      &validSourceTargetID,
					SourceTargetVersion: &validSourceTargetVersion,
				},
			},
			mockSetup:   func() {},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - auth failed",
			req: &eval_target.CreateEvalTargetRequest{
				WorkspaceID: validSpaceID,
				Param: &eval_target.CreateEvalTargetParam{
					SourceTargetID:      &validSourceTargetID,
					SourceTargetVersion: &validSourceTargetVersion,
					EvalTargetType:      &validEvalTargetType,
				},
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
			req: &eval_target.CreateEvalTargetRequest{
				WorkspaceID: validSpaceID,
				Param: &eval_target.CreateEvalTargetParam{
					SourceTargetID:      &validSourceTargetID,
					SourceTargetVersion: &validSourceTargetVersion,
					EvalTargetType:      &validEvalTargetType,
				},
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockEvalTargetService.EXPECT().CreateEvalTarget(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(int64(0), int64(0), errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := app.CreateEvalTarget(context.Background(), tt.req)

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

func TestEvalTargetApplicationImpl_BatchGetEvalTargetsBySource(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvalTargetService := mocks.NewMockIEvalTargetService(ctrl)
	mockTypedOperator := mocks.NewMockISourceEvalTargetOperateService(ctrl)

	app := &EvalTargetApplicationImpl{
		auth:              mockAuth,
		evalTargetService: mockEvalTargetService,
		typedOperators: map[entity.EvalTargetType]service.ISourceEvalTargetOperateService{
			1: mockTypedOperator,
		},
	}

	// Test data
	validSpaceID := int64(123)
	validSourceTargetIDs := []string{"source-1", "source-2"}
	validEvalTargetType := domain_eval_target.EvalTargetType(1)
	validEvalTargets := []*entity.EvalTarget{
		{
			ID:             1,
			SpaceID:        validSpaceID,
			SourceTargetID: "source-1",
			EvalTargetType: 1,
		},
	}

	tests := []struct {
		name        string
		req         *eval_target.BatchGetEvalTargetsBySourceRequest
		mockSetup   func()
		wantResp    *eval_target.BatchGetEvalTargetsBySourceResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &eval_target.BatchGetEvalTargetsBySourceRequest{
				WorkspaceID:     validSpaceID,
				SourceTargetIds: validSourceTargetIDs,
				EvalTargetType:  &validEvalTargetType,
				NeedSourceInfo:  gptr.Of(true),
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), &rpc.AuthorizationParam{
					ObjectID:      strconv.FormatInt(validSpaceID, 10),
					SpaceID:       validSpaceID,
					ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluationTarget"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
				}).Return(nil)

				mockEvalTargetService.EXPECT().BatchGetEvalTargetBySource(gomock.Any(), &entity.BatchGetEvalTargetBySourceParam{
					SpaceID:        validSpaceID,
					SourceTargetID: validSourceTargetIDs,
					TargetType:     entity.EvalTargetType(validEvalTargetType),
				}).Return(validEvalTargets, nil)

				mockTypedOperator.EXPECT().PackSourceInfo(gomock.Any(), validSpaceID, validEvalTargets).Return(nil)
			},
			wantResp: &eval_target.BatchGetEvalTargetsBySourceResponse{
				EvalTargets: []*domain_eval_target.EvalTarget{
					{
						ID:             gptr.Of(int64(1)),
						WorkspaceID:    gptr.Of(validSpaceID),
						SourceTargetID: gptr.Of("source-1"),
						EvalTargetType: &validEvalTargetType,
					},
				},
			},
			wantErr: false,
		},
		{
			name:        "error - nil request",
			req:         nil,
			mockSetup:   func() {},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - empty source target ids",
			req: &eval_target.BatchGetEvalTargetsBySourceRequest{
				WorkspaceID:     validSpaceID,
				SourceTargetIds: []string{},
				EvalTargetType:  &validEvalTargetType,
			},
			mockSetup:   func() {},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - nil eval target type",
			req: &eval_target.BatchGetEvalTargetsBySourceRequest{
				WorkspaceID:     validSpaceID,
				SourceTargetIds: validSourceTargetIDs,
				EvalTargetType:  nil,
			},
			mockSetup:   func() {},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - auth failure",
			req: &eval_target.BatchGetEvalTargetsBySourceRequest{
				WorkspaceID:     validSpaceID,
				SourceTargetIds: validSourceTargetIDs,
				EvalTargetType:  &validEvalTargetType,
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
			req: &eval_target.BatchGetEvalTargetsBySourceRequest{
				WorkspaceID:     validSpaceID,
				SourceTargetIds: validSourceTargetIDs,
				EvalTargetType:  &validEvalTargetType,
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockEvalTargetService.EXPECT().BatchGetEvalTargetBySource(gomock.Any(), gomock.Any()).
					Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
		{
			name: "error - pack source info failure",
			req: &eval_target.BatchGetEvalTargetsBySourceRequest{
				WorkspaceID:     validSpaceID,
				SourceTargetIds: validSourceTargetIDs,
				EvalTargetType:  &validEvalTargetType,
				NeedSourceInfo:  gptr.Of(true),
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockEvalTargetService.EXPECT().BatchGetEvalTargetBySource(gomock.Any(), gomock.Any()).
					Return(validEvalTargets, nil)
				mockTypedOperator.EXPECT().PackSourceInfo(gomock.Any(), validSpaceID, validEvalTargets).
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

			resp, err := app.BatchGetEvalTargetsBySource(context.Background(), tt.req)

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

func TestEvalTargetApplicationImpl_GetEvalTargetVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvalTargetService := mocks.NewMockIEvalTargetService(ctrl)

	app := &EvalTargetApplicationImpl{
		auth:              mockAuth,
		evalTargetService: mockEvalTargetService,
	}

	// Test data
	validSpaceID := int64(123)
	validVersionID := int64(456)
	validEvalTarget := &entity.EvalTarget{
		ID:             1,
		SpaceID:        validSpaceID,
		SourceTargetID: "source-123",
		EvalTargetType: 1,
		EvalTargetVersion: &entity.EvalTargetVersion{
			ID:                  validVersionID,
			SpaceID:             validSpaceID,
			TargetID:            1,
			SourceTargetVersion: "v1.0",
		},
	}

	tests := []struct {
		name        string
		req         *eval_target.GetEvalTargetVersionRequest
		mockSetup   func()
		wantResp    *eval_target.GetEvalTargetVersionResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &eval_target.GetEvalTargetVersionRequest{
				WorkspaceID:         validSpaceID,
				EvalTargetVersionID: &validVersionID,
			},
			mockSetup: func() {
				mockEvalTargetService.EXPECT().
					GetEvalTargetVersion(gomock.Any(), validSpaceID, validVersionID, false).
					Return(validEvalTarget, nil)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(validEvalTarget.ID, 10),
						SpaceID:       validSpaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationTarget)}},
					}).
					Return(nil)
			},
			wantResp: &eval_target.GetEvalTargetVersionResponse{
				EvalTarget: target.EvalTargetDO2DTO(validEvalTarget),
			},
			wantErr: false,
		},
		{
			name:        "error - nil request",
			req:         nil,
			mockSetup:   func() {},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - nil version id",
			req: &eval_target.GetEvalTargetVersionRequest{
				WorkspaceID: validSpaceID,
			},
			mockSetup:   func() {},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "success - eval target not found",
			req: &eval_target.GetEvalTargetVersionRequest{
				WorkspaceID:         validSpaceID,
				EvalTargetVersionID: &validVersionID,
			},
			mockSetup: func() {
				mockEvalTargetService.EXPECT().
					GetEvalTargetVersion(gomock.Any(), validSpaceID, validVersionID, false).
					Return(nil, nil)
			},
			wantResp: &eval_target.GetEvalTargetVersionResponse{},
			wantErr:  false,
		},
		{
			name: "error - service failure",
			req: &eval_target.GetEvalTargetVersionRequest{
				WorkspaceID:         validSpaceID,
				EvalTargetVersionID: &validVersionID,
			},
			mockSetup: func() {
				mockEvalTargetService.EXPECT().
					GetEvalTargetVersion(gomock.Any(), validSpaceID, validVersionID, false).
					Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
		{
			name: "error - auth failed",
			req: &eval_target.GetEvalTargetVersionRequest{
				WorkspaceID:         validSpaceID,
				EvalTargetVersionID: &validVersionID,
			},
			mockSetup: func() {
				mockEvalTargetService.EXPECT().
					GetEvalTargetVersion(gomock.Any(), validSpaceID, validVersionID, false).
					Return(validEvalTarget, nil)

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

			resp, err := app.GetEvalTargetVersion(context.Background(), tt.req)

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

func TestEvalTargetApplicationImpl_BatchGetEvalTargetVersions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvalTargetService := mocks.NewMockIEvalTargetService(ctrl)

	app := &EvalTargetApplicationImpl{
		auth:              mockAuth,
		evalTargetService: mockEvalTargetService,
	}

	// Test data
	validSpaceID := int64(123)
	validVersionIDs := []int64{456, 789}
	validEvalTargets := []*entity.EvalTarget{
		{
			ID:             1,
			SpaceID:        validSpaceID,
			SourceTargetID: "source-123",
			EvalTargetType: 1,
			EvalTargetVersion: &entity.EvalTargetVersion{
				ID:                  456,
				SpaceID:             validSpaceID,
				TargetID:            1,
				SourceTargetVersion: "v1.0",
			},
		},
		{
			ID:             2,
			SpaceID:        validSpaceID,
			SourceTargetID: "source-456",
			EvalTargetType: 1,
			EvalTargetVersion: &entity.EvalTargetVersion{
				ID:                  789,
				SpaceID:             validSpaceID,
				TargetID:            2,
				SourceTargetVersion: "v2.0",
			},
		},
	}

	tests := []struct {
		name        string
		req         *eval_target.BatchGetEvalTargetVersionsRequest
		mockSetup   func()
		wantResp    *eval_target.BatchGetEvalTargetVersionsResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &eval_target.BatchGetEvalTargetVersionsRequest{
				WorkspaceID:          validSpaceID,
				EvalTargetVersionIds: validVersionIDs,
			},
			mockSetup: func() {
				mockEvalTargetService.EXPECT().
					BatchGetEvalTargetVersion(gomock.Any(), validSpaceID, validVersionIDs, gomock.Any()).
					Return(validEvalTargets, nil)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil)
			},
			wantResp: &eval_target.BatchGetEvalTargetVersionsResponse{
				EvalTargets: []*domain_eval_target.EvalTarget{
					target.EvalTargetDO2DTO(validEvalTargets[0]),
					target.EvalTargetDO2DTO(validEvalTargets[1]),
				},
			},
			wantErr: false,
		},
		{
			name:        "error - nil request",
			req:         nil,
			mockSetup:   func() {},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - empty version ids",
			req: &eval_target.BatchGetEvalTargetVersionsRequest{
				WorkspaceID:          validSpaceID,
				EvalTargetVersionIds: []int64{},
			},
			mockSetup:   func() {},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - auth failed",
			req: &eval_target.BatchGetEvalTargetVersionsRequest{
				WorkspaceID:          validSpaceID,
				EvalTargetVersionIds: validVersionIDs,
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
			req: &eval_target.BatchGetEvalTargetVersionsRequest{
				WorkspaceID:          validSpaceID,
				EvalTargetVersionIds: validVersionIDs,
			},
			mockSetup: func() {
				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil)

				mockEvalTargetService.EXPECT().
					BatchGetEvalTargetVersion(gomock.Any(), validSpaceID, validVersionIDs, gomock.Any()).
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

			resp, err := app.BatchGetEvalTargetVersions(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.wantResp.EvalTargets), len(resp.EvalTargets))
				for i, target := range tt.wantResp.EvalTargets {
					assert.Equal(t, target.ID, resp.EvalTargets[i].ID)
					assert.Equal(t, target.WorkspaceID, resp.EvalTargets[i].WorkspaceID)
					assert.Equal(t, target.SourceTargetID, resp.EvalTargets[i].SourceTargetID)
				}
			}
		})
	}
}

func TestEvalTargetApplicationImpl_ListSourceEvalTargets(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvalTargetService := mocks.NewMockIEvalTargetService(ctrl)
	mockTypedOperator := mocks.NewMockISourceEvalTargetOperateService(ctrl)

	app := &EvalTargetApplicationImpl{
		auth:              mockAuth,
		evalTargetService: mockEvalTargetService,
		typedOperators: map[entity.EvalTargetType]service.ISourceEvalTargetOperateService{
			1: mockTypedOperator,
		},
	}

	// Test data
	validSpaceID := int64(123)
	validEvalTargetType := domain_eval_target.EvalTargetType(1)
	validEvalTargets := []*entity.EvalTarget{
		{
			ID:             1,
			SpaceID:        validSpaceID,
			SourceTargetID: "source-123",
			EvalTargetType: 1,
		},
		{
			ID:             2,
			SpaceID:        validSpaceID,
			SourceTargetID: "source-456",
			EvalTargetType: 1,
		},
	}

	tests := []struct {
		name        string
		req         *eval_target.ListSourceEvalTargetsRequest
		mockSetup   func()
		wantResp    *eval_target.ListSourceEvalTargetsResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &eval_target.ListSourceEvalTargetsRequest{
				WorkspaceID: validSpaceID,
				TargetType:  &validEvalTargetType,
			},
			mockSetup: func() {
				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil)

				mockTypedOperator.EXPECT().
					ListSource(gomock.Any(), gomock.Any()).
					Return([]*entity.EvalTarget{{
						ID:             1,
						SpaceID:        validSpaceID,
						SourceTargetID: "source-123",
						EvalTargetType: 1,
					}, {
						ID:             2,
						SpaceID:        validSpaceID,
						SourceTargetID: "source-456",
						EvalTargetType: 1,
					}}, "", false, nil)
			},
			wantResp: &eval_target.ListSourceEvalTargetsResponse{
				EvalTargets: []*domain_eval_target.EvalTarget{
					target.EvalTargetDO2DTO(validEvalTargets[0]),
					target.EvalTargetDO2DTO(validEvalTargets[1]),
				},
			},
			wantErr: false,
		},
		{
			name:        "error - nil request",
			req:         nil,
			mockSetup:   func() {},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - nil eval target type",
			req: &eval_target.ListSourceEvalTargetsRequest{
				WorkspaceID: validSpaceID,
			},
			mockSetup:   func() {},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := app.ListSourceEvalTargets(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.wantResp.EvalTargets), len(resp.EvalTargets))
				for i, target := range tt.wantResp.EvalTargets {
					assert.Equal(t, target.ID, resp.EvalTargets[i].ID)
					assert.Equal(t, target.WorkspaceID, resp.EvalTargets[i].WorkspaceID)
					assert.Equal(t, target.SourceTargetID, resp.EvalTargets[i].SourceTargetID)
				}
			}
		})
	}
}

func TestEvalTargetApplicationImpl_ListSourceEvalTargetVersions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvalTargetService := mocks.NewMockIEvalTargetService(ctrl)
	mockTypedOperator := mocks.NewMockISourceEvalTargetOperateService(ctrl)

	app := &EvalTargetApplicationImpl{
		auth:              mockAuth,
		evalTargetService: mockEvalTargetService,
		typedOperators: map[entity.EvalTargetType]service.ISourceEvalTargetOperateService{
			1: mockTypedOperator,
		},
	}

	// Test data
	validSpaceID := int64(123)
	validEvalTargetType := domain_eval_target.EvalTargetType(1)
	validEvalTargets := []*entity.EvalTargetVersion{
		{
			ID:             1,
			SpaceID:        validSpaceID,
			EvalTargetType: 1,
			CozeBot: &entity.CozeBot{
				BotID:      456,
				BotVersion: "v1.0",
			},
		}, {
			ID:             2,
			SpaceID:        validSpaceID,
			EvalTargetType: 2,
			Prompt: &entity.LoopPrompt{
				PromptID: 789,
				Version:  "v2.0",
			},
		}, {
			ID:             2,
			SpaceID:        validSpaceID,
			EvalTargetType: 4,
			CozeWorkflow: &entity.CozeWorkflow{
				ID:      "123",
				Version: "v2.0",
			},
		},
	}

	tests := []struct {
		name        string
		req         *eval_target.ListSourceEvalTargetVersionsRequest
		mockSetup   func()
		wantResp    *eval_target.ListSourceEvalTargetVersionsResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &eval_target.ListSourceEvalTargetVersionsRequest{
				WorkspaceID: validSpaceID,
				TargetType:  &validEvalTargetType,
			},
			mockSetup: func() {
				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil)

				mockTypedOperator.EXPECT().
					ListSourceVersion(gomock.Any(), gomock.Any()).
					Return([]*entity.EvalTargetVersion{{
						ID:             1,
						SpaceID:        validSpaceID,
						EvalTargetType: 1,
						CozeBot: &entity.CozeBot{
							BotID:      456,
							BotVersion: "v1.0",
						},
					}, {
						ID:             2,
						SpaceID:        validSpaceID,
						EvalTargetType: 2,
						Prompt: &entity.LoopPrompt{
							PromptID: 789,
							Version:  "v2.0",
						},
					}, {
						ID:      2,
						SpaceID: validSpaceID,
						CozeWorkflow: &entity.CozeWorkflow{
							ID:      "123",
							Version: "v2.0",
						},
					}}, "", false, nil)
			},
			wantResp: &eval_target.ListSourceEvalTargetVersionsResponse{
				Versions: []*domain_eval_target.EvalTargetVersion{
					target.EvalTargetVersionDO2DTO(validEvalTargets[0]),
					target.EvalTargetVersionDO2DTO(validEvalTargets[1]),
					target.EvalTargetVersionDO2DTO(validEvalTargets[2]),
				},
			},
			wantErr: false,
		},
		{
			name:        "error - nil request",
			req:         nil,
			mockSetup:   func() {},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - nil target type",
			req: &eval_target.ListSourceEvalTargetVersionsRequest{
				WorkspaceID: validSpaceID,
			},
			mockSetup:   func() {},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := app.ListSourceEvalTargetVersions(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.wantResp.Versions), len(resp.Versions))
			}
		})
	}
}

func TestEvalTargetApplicationImpl_BatchGetSourceEvalTargets(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockTypedOperator := mocks.NewMockISourceEvalTargetOperateService(ctrl)

	app := &EvalTargetApplicationImpl{
		auth: mockAuth,
		typedOperators: map[entity.EvalTargetType]service.ISourceEvalTargetOperateService{
			1: mockTypedOperator,
		},
	}

	// Test data
	validSpaceID := int64(123)
	validEvalTargetType := domain_eval_target.EvalTargetType(1)
	unsupportedEvalTargetType := domain_eval_target.EvalTargetType(99)
	validSourceTargetIDs := []string{"source-1", "source-2"}
	validEvalTargets := []*entity.EvalTarget{
		{
			ID:             1,
			SpaceID:        validSpaceID,
			SourceTargetID: "source-1",
			EvalTargetType: 1,
		},
		{
			ID:             2,
			SpaceID:        validSpaceID,
			SourceTargetID: "source-2",
			EvalTargetType: 1,
		},
	}

	tests := []struct {
		name        string
		req         *eval_target.BatchGetSourceEvalTargetsRequest
		mockSetup   func()
		wantResp    *eval_target.BatchGetSourceEvalTargetsResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &eval_target.BatchGetSourceEvalTargetsRequest{
				WorkspaceID:     validSpaceID,
				TargetType:      &validEvalTargetType,
				SourceTargetIds: validSourceTargetIDs,
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockTypedOperator.EXPECT().
					BatchGetSource(gomock.Any(), validSpaceID, validSourceTargetIDs).
					Return(validEvalTargets, nil)
			},
			wantResp: &eval_target.BatchGetSourceEvalTargetsResponse{
				EvalTargets: []*domain_eval_target.EvalTarget{
					target.EvalTargetDO2DTO(validEvalTargets[0]),
					target.EvalTargetDO2DTO(validEvalTargets[1]),
				},
			},
			wantErr: false,
		},
		{
			name: "error - nil target type",
			req: &eval_target.BatchGetSourceEvalTargetsRequest{
				WorkspaceID:     validSpaceID,
				SourceTargetIds: validSourceTargetIDs,
			},
			mockSetup:   func() {},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - auth failed",
			req: &eval_target.BatchGetSourceEvalTargetsRequest{
				WorkspaceID:     validSpaceID,
				TargetType:      &validEvalTargetType,
				SourceTargetIds: validSourceTargetIDs,
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
			name: "error - unsupported target type",
			req: &eval_target.BatchGetSourceEvalTargetsRequest{
				WorkspaceID:     validSpaceID,
				TargetType:      &unsupportedEvalTargetType,
				SourceTargetIds: validSourceTargetIDs,
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - service failure",
			req: &eval_target.BatchGetSourceEvalTargetsRequest{
				WorkspaceID:     validSpaceID,
				TargetType:      &validEvalTargetType,
				SourceTargetIds: validSourceTargetIDs,
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockTypedOperator.EXPECT().
					BatchGetSource(gomock.Any(), validSpaceID, validSourceTargetIDs).
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

			resp, err := app.BatchGetSourceEvalTargets(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.wantResp.EvalTargets), len(resp.EvalTargets))
				for i, trgt := range tt.wantResp.EvalTargets {
					assert.Equal(t, trgt.ID, resp.EvalTargets[i].ID)
					assert.Equal(t, trgt.SourceTargetID, resp.EvalTargets[i].SourceTargetID)
				}
			}
		})
	}
}
