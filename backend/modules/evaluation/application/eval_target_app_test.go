// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	stdjson "encoding/json"
	"strconv"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	domaincommon "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	domain_eval_target "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	evaltargetapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_target"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/target"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
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
		req         *evaltargetapi.CreateEvalTargetRequest
		mockSetup   func()
		wantResp    *evaltargetapi.CreateEvalTargetResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &evaltargetapi.CreateEvalTargetRequest{
				WorkspaceID: validSpaceID,
				Param: &evaltargetapi.CreateEvalTargetParam{
					SourceTargetID:      &validSourceTargetID,
					SourceTargetVersion: &validSourceTargetVersion,
					EvalTargetType:      &validEvalTargetType,
					BotInfoType:         &validBotInfoType,
					BotPublishVersion:   &validBotPublishVersion,
					CustomEvalTarget: &domain_eval_target.CustomEvalTarget{
						Name: gptr.Of("test"),
					},
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
			wantResp: &evaltargetapi.CreateEvalTargetResponse{
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
			req: &evaltargetapi.CreateEvalTargetRequest{
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
			req: &evaltargetapi.CreateEvalTargetRequest{
				WorkspaceID: validSpaceID,
				Param: &evaltargetapi.CreateEvalTargetParam{
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
			req: &evaltargetapi.CreateEvalTargetRequest{
				WorkspaceID: validSpaceID,
				Param: &evaltargetapi.CreateEvalTargetParam{
					SourceTargetID: &validSourceTargetID,
					EvalTargetType: &validEvalTargetType,
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
					gomock.Any(),
					gomock.Any(),
					gomock.Any(), // options
				).Return(int64(1), int64(2), nil)
			},
			wantResp: &evaltargetapi.CreateEvalTargetResponse{
				ID:        gptr.Of(int64(1)),
				VersionID: gptr.Of(int64(2)),
			},
			wantErr: false,
		},
		{
			name: "error - missing eval target type",
			req: &evaltargetapi.CreateEvalTargetRequest{
				WorkspaceID: validSpaceID,
				Param: &evaltargetapi.CreateEvalTargetParam{
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
			req: &evaltargetapi.CreateEvalTargetRequest{
				WorkspaceID: validSpaceID,
				Param: &evaltargetapi.CreateEvalTargetParam{
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
			req: &evaltargetapi.CreateEvalTargetRequest{
				WorkspaceID: validSpaceID,
				Param: &evaltargetapi.CreateEvalTargetParam{
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

func TestNewEvalTargetHandlerImpl(t *testing.T) {
	handler := NewEvalTargetHandlerImpl(nil, nil, nil, nil)
	if handler == nil {
		t.Fatalf("handler is nil")
	}
	if handler2 := NewEvalTargetHandlerImpl(nil, nil, nil, nil); handler2 != handler {
		t.Fatalf("handler should be singleton")
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
		req         *evaltargetapi.BatchGetEvalTargetsBySourceRequest
		mockSetup   func()
		wantResp    *evaltargetapi.BatchGetEvalTargetsBySourceResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &evaltargetapi.BatchGetEvalTargetsBySourceRequest{
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
			wantResp: &evaltargetapi.BatchGetEvalTargetsBySourceResponse{
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
			req: &evaltargetapi.BatchGetEvalTargetsBySourceRequest{
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
			req: &evaltargetapi.BatchGetEvalTargetsBySourceRequest{
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
			req: &evaltargetapi.BatchGetEvalTargetsBySourceRequest{
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
			req: &evaltargetapi.BatchGetEvalTargetsBySourceRequest{
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
			req: &evaltargetapi.BatchGetEvalTargetsBySourceRequest{
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
		req         *evaltargetapi.GetEvalTargetVersionRequest
		mockSetup   func()
		wantResp    *evaltargetapi.GetEvalTargetVersionResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &evaltargetapi.GetEvalTargetVersionRequest{
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
			wantResp: &evaltargetapi.GetEvalTargetVersionResponse{
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
			req: &evaltargetapi.GetEvalTargetVersionRequest{
				WorkspaceID: validSpaceID,
			},
			mockSetup:   func() {},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "success - eval target not found",
			req: &evaltargetapi.GetEvalTargetVersionRequest{
				WorkspaceID:         validSpaceID,
				EvalTargetVersionID: &validVersionID,
			},
			mockSetup: func() {
				mockEvalTargetService.EXPECT().
					GetEvalTargetVersion(gomock.Any(), validSpaceID, validVersionID, false).
					Return(nil, nil)
			},
			wantResp: &evaltargetapi.GetEvalTargetVersionResponse{},
			wantErr:  false,
		},
		{
			name: "error - service failure",
			req: &evaltargetapi.GetEvalTargetVersionRequest{
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
			req: &evaltargetapi.GetEvalTargetVersionRequest{
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
		req         *evaltargetapi.BatchGetEvalTargetVersionsRequest
		mockSetup   func()
		wantResp    *evaltargetapi.BatchGetEvalTargetVersionsResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &evaltargetapi.BatchGetEvalTargetVersionsRequest{
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
			wantResp: &evaltargetapi.BatchGetEvalTargetVersionsResponse{
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
			req: &evaltargetapi.BatchGetEvalTargetVersionsRequest{
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
			req: &evaltargetapi.BatchGetEvalTargetVersionsRequest{
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
			req: &evaltargetapi.BatchGetEvalTargetVersionsRequest{
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
		req         *evaltargetapi.ListSourceEvalTargetsRequest
		mockSetup   func()
		wantResp    *evaltargetapi.ListSourceEvalTargetsResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &evaltargetapi.ListSourceEvalTargetsRequest{
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
			wantResp: &evaltargetapi.ListSourceEvalTargetsResponse{
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
			req: &evaltargetapi.ListSourceEvalTargetsRequest{
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
		req         *evaltargetapi.ListSourceEvalTargetVersionsRequest
		mockSetup   func()
		wantResp    *evaltargetapi.ListSourceEvalTargetVersionsResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &evaltargetapi.ListSourceEvalTargetVersionsRequest{
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
			wantResp: &evaltargetapi.ListSourceEvalTargetVersionsResponse{
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
			req: &evaltargetapi.ListSourceEvalTargetVersionsRequest{
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
		req         *evaltargetapi.BatchGetSourceEvalTargetsRequest
		mockSetup   func()
		wantResp    *evaltargetapi.BatchGetSourceEvalTargetsResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &evaltargetapi.BatchGetSourceEvalTargetsRequest{
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
			wantResp: &evaltargetapi.BatchGetSourceEvalTargetsResponse{
				EvalTargets: []*domain_eval_target.EvalTarget{
					target.EvalTargetDO2DTO(validEvalTargets[0]),
					target.EvalTargetDO2DTO(validEvalTargets[1]),
				},
			},
			wantErr: false,
		},
		{
			name: "error - nil target type",
			req: &evaltargetapi.BatchGetSourceEvalTargetsRequest{
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
			req: &evaltargetapi.BatchGetSourceEvalTargetsRequest{
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
			req: &evaltargetapi.BatchGetSourceEvalTargetsRequest{
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
			req: &evaltargetapi.BatchGetSourceEvalTargetsRequest{
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

func TestEvalTargetApplicationImpl_SearchCustomEvalTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockTypedOperator := mocks.NewMockISourceEvalTargetOperateService(ctrl)

	app := &EvalTargetApplicationImpl{
		auth: mockAuth,
		typedOperators: map[entity.EvalTargetType]service.ISourceEvalTargetOperateService{
			entity.EvalTargetTypeCustomRPCServer: mockTypedOperator,
		},
	}

	// Test data
	validSpaceID := int64(123)
	validApplicationID := int64(456)
	validKeyword := "test keyword"
	validRegion := "cn"
	validEnv := "prod"
	validPageSize := int32(10)
	validPageToken := "token123"
	validCustomRPCServer := &domain_eval_target.CustomRPCServer{
		ID:             gptr.Of(int64(789)),
		Name:           gptr.Of("test server"),
		Description:    gptr.Of("test description"),
		ServerName:     gptr.Of("test-server"),
		AccessProtocol: gptr.Of("rpc"),
		Cluster:        gptr.Of("test-cluster"),
	}

	validCustomEvalTargets := []*entity.CustomEvalTarget{
		{
			ID:        gptr.Of("target-1"),
			Name:      gptr.Of("Test Target 1"),
			AvatarURL: gptr.Of("http://example.com/avatar1.jpg"),
			Ext:       map[string]string{"type": "custom"},
		},
		{
			ID:        gptr.Of("target-2"),
			Name:      gptr.Of("Test Target 2"),
			AvatarURL: gptr.Of("http://example.com/avatar2.jpg"),
			Ext:       map[string]string{"type": "custom"},
		},
	}

	tests := []struct {
		name        string
		req         *evaltargetapi.SearchCustomEvalTargetRequest
		mockSetup   func()
		wantResp    *evaltargetapi.SearchCustomEvalTargetResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request with applicationID",
			req: &evaltargetapi.SearchCustomEvalTargetRequest{
				WorkspaceID:   &validSpaceID,
				Keyword:       &validKeyword,
				ApplicationID: &validApplicationID,
				Region:        &validRegion,
				Env:           &validEnv,
				PageSize:      &validPageSize,
				PageToken:     &validPageToken,
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), &rpc.AuthorizationParam{
					ObjectID:      strconv.FormatInt(validSpaceID, 10),
					SpaceID:       validSpaceID,
					ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluationTarget"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
				}).Return(nil)

				mockTypedOperator.EXPECT().SearchCustomEvalTarget(gomock.Any(), &entity.SearchCustomEvalTargetParam{
					WorkspaceID:     &validSpaceID,
					Keyword:         &validKeyword,
					ApplicationID:   &validApplicationID,
					CustomRPCServer: nil,
					Region:          &validRegion,
					Env:             &validEnv,
					PageSize:        &validPageSize,
					PageToken:       &validPageToken,
				}).Return(validCustomEvalTargets, "next-token", true, nil)
			},
			wantResp: &evaltargetapi.SearchCustomEvalTargetResponse{
				CustomEvalTargets: []*domain_eval_target.CustomEvalTarget{
					{
						ID:        gptr.Of("target-1"),
						Name:      gptr.Of("Test Target 1"),
						AvatarURL: gptr.Of("http://example.com/avatar1.jpg"),
						Ext:       map[string]string{"type": "custom"},
					},
					{
						ID:        gptr.Of("target-2"),
						Name:      gptr.Of("Test Target 2"),
						AvatarURL: gptr.Of("http://example.com/avatar2.jpg"),
						Ext:       map[string]string{"type": "custom"},
					},
				},
				NextPageToken: gptr.Of("next-token"),
				HasMore:       gptr.Of(true),
			},
			wantErr: false,
		},
		{
			name: "success - normal request with customRPCServer",
			req: &evaltargetapi.SearchCustomEvalTargetRequest{
				WorkspaceID:     &validSpaceID,
				Keyword:         &validKeyword,
				CustomRPCServer: validCustomRPCServer,
				Region:          &validRegion,
				Env:             &validEnv,
				PageSize:        &validPageSize,
				PageToken:       &validPageToken,
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), &rpc.AuthorizationParam{
					ObjectID:      strconv.FormatInt(validSpaceID, 10),
					SpaceID:       validSpaceID,
					ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluationTarget"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
				}).Return(nil)

				mockTypedOperator.EXPECT().SearchCustomEvalTarget(gomock.Any(), gomock.Any()).Return(validCustomEvalTargets, "next-token", true, nil)
			},
			wantResp: &evaltargetapi.SearchCustomEvalTargetResponse{
				CustomEvalTargets: []*domain_eval_target.CustomEvalTarget{
					{
						ID:        gptr.Of("target-1"),
						Name:      gptr.Of("Test Target 1"),
						AvatarURL: gptr.Of("http://example.com/avatar1.jpg"),
						Ext:       map[string]string{"type": "custom"},
					},
					{
						ID:        gptr.Of("target-2"),
						Name:      gptr.Of("Test Target 2"),
						AvatarURL: gptr.Of("http://example.com/avatar2.jpg"),
						Ext:       map[string]string{"type": "custom"},
					},
				},
				NextPageToken: gptr.Of("next-token"),
				HasMore:       gptr.Of(true),
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
			name: "error - nil workspaceID",
			req: &evaltargetapi.SearchCustomEvalTargetRequest{
				Keyword:       &validKeyword,
				ApplicationID: &validApplicationID,
				Region:        &validRegion,
			},
			mockSetup:   func() {},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - both applicationID and customRPCServer are nil",
			req: &evaltargetapi.SearchCustomEvalTargetRequest{
				WorkspaceID: &validSpaceID,
				Keyword:     &validKeyword,
				Region:      &validRegion,
			},
			mockSetup:   func() {},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - nil region",
			req: &evaltargetapi.SearchCustomEvalTargetRequest{
				WorkspaceID:   &validSpaceID,
				Keyword:       &validKeyword,
				ApplicationID: &validApplicationID,
			},
			mockSetup:   func() {},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - target type not support",
			req: &evaltargetapi.SearchCustomEvalTargetRequest{
				WorkspaceID:   &validSpaceID,
				Keyword:       &validKeyword,
				ApplicationID: &validApplicationID,
				Region:        &validRegion,
			},
			mockSetup: func() {
				// Create app without typedOperators for CustomRPCServer
				app.typedOperators = map[entity.EvalTargetType]service.ISourceEvalTargetOperateService{}
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - auth failed",
			req: &evaltargetapi.SearchCustomEvalTargetRequest{
				WorkspaceID:   &validSpaceID,
				Keyword:       &validKeyword,
				ApplicationID: &validApplicationID,
				Region:        &validRegion,
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
			req: &evaltargetapi.SearchCustomEvalTargetRequest{
				WorkspaceID:   &validSpaceID,
				Keyword:       &validKeyword,
				ApplicationID: &validApplicationID,
				Region:        &validRegion,
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockTypedOperator.EXPECT().SearchCustomEvalTarget(gomock.Any(), gomock.Any()).
					Return(nil, "", false, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
		{
			name: "success - empty results",
			req: &evaltargetapi.SearchCustomEvalTargetRequest{
				WorkspaceID:   &validSpaceID,
				Keyword:       &validKeyword,
				ApplicationID: &validApplicationID,
				Region:        &validRegion,
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), &rpc.AuthorizationParam{
					ObjectID:      strconv.FormatInt(validSpaceID, 10),
					SpaceID:       validSpaceID,
					ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluationTarget"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
				}).Return(nil)

				mockTypedOperator.EXPECT().SearchCustomEvalTarget(gomock.Any(), &entity.SearchCustomEvalTargetParam{
					WorkspaceID:     &validSpaceID,
					Keyword:         &validKeyword,
					ApplicationID:   &validApplicationID,
					CustomRPCServer: nil,
					Region:          &validRegion,
					Env:             nil,
					PageSize:        nil,
					PageToken:       nil,
				}).Return([]*entity.CustomEvalTarget{}, "", false, nil)
			},
			wantResp: &evaltargetapi.SearchCustomEvalTargetResponse{
				CustomEvalTargets: []*domain_eval_target.CustomEvalTarget{},
				NextPageToken:     gptr.Of(""),
				HasMore:           gptr.Of(false),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset typedOperators for each test
			app.typedOperators = map[entity.EvalTargetType]service.ISourceEvalTargetOperateService{
				entity.EvalTargetTypeCustomRPCServer: mockTypedOperator,
			}

			tt.mockSetup()

			resp, err := app.SearchCustomEvalTarget(context.Background(), tt.req)

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

func TestEvalTargetApplicationImpl_MockEvalTargetOutput(t *testing.T) {
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
	validSourceTargetID := int64(456)
	validEvalTargetVersion := "v1.0"
	validTargetType := domain_eval_target.EvalTargetType(1)
	unsupportedTargetType := domain_eval_target.EvalTargetType(99)

	validEvalTarget := &entity.EvalTarget{
		ID:             1,
		SpaceID:        validSpaceID,
		SourceTargetID: "456",
		EvalTargetType: 1,
		EvalTargetVersion: &entity.EvalTargetVersion{
			ID:                  1,
			SpaceID:             validSpaceID,
			TargetID:            1,
			SourceTargetVersion: validEvalTargetVersion,
			OutputSchema: []*entity.ArgsSchema{
				{
					Key:        gptr.Of("output"),
					JsonSchema: gptr.Of(`{"type": "string"}`),
				},
			},
		},
	}

	tests := []struct {
		name        string
		req         *evaltargetapi.MockEvalTargetOutputRequest
		mockSetup   func()
		wantResp    *evaltargetapi.MockEvalTargetOutputResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &evaltargetapi.MockEvalTargetOutputRequest{
				WorkspaceID:       validSpaceID,
				SourceTargetID:    validSourceTargetID,
				EvalTargetVersion: validEvalTargetVersion,
				TargetType:        validTargetType,
			},
			mockSetup: func() {
				mockTypedOperator.EXPECT().
					BuildBySource(gomock.Any(), validSpaceID, strconv.FormatInt(validSourceTargetID, 10), validEvalTargetVersion).
					Return(validEvalTarget, nil)

				mockAuth.EXPECT().Authorization(gomock.Any(), &rpc.AuthorizationParam{
					ObjectID:      strconv.FormatInt(validSpaceID, 10),
					SpaceID:       validSpaceID,
					ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("createLoopEvaluationTarget"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
				}).Return(nil)

				mockEvalTargetService.EXPECT().
					GenerateMockOutputData(validEvalTarget.EvalTargetVersion.OutputSchema).
					Return(map[string]string{"output": "mock output"}, nil)
			},
			wantResp: &evaltargetapi.MockEvalTargetOutputResponse{
				EvalTarget: target.EvalTargetDO2DTO(validEvalTarget),
				MockOutput: map[string]string{"output": "mock output"},
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
			name: "error - unsupported target type",
			req: &evaltargetapi.MockEvalTargetOutputRequest{
				WorkspaceID:       validSpaceID,
				SourceTargetID:    validSourceTargetID,
				EvalTargetVersion: validEvalTargetVersion,
				TargetType:        unsupportedTargetType,
			},
			mockSetup:   func() {},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - auth failed",
			req: &evaltargetapi.MockEvalTargetOutputRequest{
				WorkspaceID:       validSpaceID,
				SourceTargetID:    validSourceTargetID,
				EvalTargetVersion: validEvalTargetVersion,
				TargetType:        validTargetType,
			},
			mockSetup: func() {
				mockTypedOperator.EXPECT().
					BuildBySource(gomock.Any(), validSpaceID, strconv.FormatInt(validSourceTargetID, 10), validEvalTargetVersion).
					Return(validEvalTarget, nil)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
		{
			name: "error - build by source failed",
			req: &evaltargetapi.MockEvalTargetOutputRequest{
				WorkspaceID:       validSpaceID,
				SourceTargetID:    validSourceTargetID,
				EvalTargetVersion: validEvalTargetVersion,
				TargetType:        validTargetType,
			},
			mockSetup: func() {
				mockTypedOperator.EXPECT().
					BuildBySource(gomock.Any(), validSpaceID, strconv.FormatInt(validSourceTargetID, 10), validEvalTargetVersion).
					Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
		{
			name: "error - build by source returns nil",
			req: &evaltargetapi.MockEvalTargetOutputRequest{
				WorkspaceID:       validSpaceID,
				SourceTargetID:    validSourceTargetID,
				EvalTargetVersion: validEvalTargetVersion,
				TargetType:        validTargetType,
			},
			mockSetup: func() {
				mockTypedOperator.EXPECT().
					BuildBySource(gomock.Any(), validSpaceID, strconv.FormatInt(validSourceTargetID, 10), validEvalTargetVersion).
					Return(nil, nil)
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - generate mock data failed",
			req: &evaltargetapi.MockEvalTargetOutputRequest{
				WorkspaceID:       validSpaceID,
				SourceTargetID:    validSourceTargetID,
				EvalTargetVersion: validEvalTargetVersion,
				TargetType:        validTargetType,
			},
			mockSetup: func() {
				mockTypedOperator.EXPECT().
					BuildBySource(gomock.Any(), validSpaceID, strconv.FormatInt(validSourceTargetID, 10), validEvalTargetVersion).
					Return(validEvalTarget, nil)

				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)

				mockEvalTargetService.EXPECT().
					GenerateMockOutputData(validEvalTarget.EvalTargetVersion.OutputSchema).
					Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
		{
			name: "success - no output schema",
			req: &evaltargetapi.MockEvalTargetOutputRequest{
				WorkspaceID:       validSpaceID,
				SourceTargetID:    validSourceTargetID,
				EvalTargetVersion: validEvalTargetVersion,
				TargetType:        validTargetType,
			},
			mockSetup: func() {
				evalTargetWithoutSchema := &entity.EvalTarget{
					ID:             1,
					SpaceID:        validSpaceID,
					SourceTargetID: "456",
					EvalTargetType: 1,
					EvalTargetVersion: &entity.EvalTargetVersion{
						ID:                  1,
						SpaceID:             validSpaceID,
						TargetID:            1,
						SourceTargetVersion: validEvalTargetVersion,
						OutputSchema:        nil, // No output schema
					},
				}

				mockTypedOperator.EXPECT().
					BuildBySource(gomock.Any(), validSpaceID, strconv.FormatInt(validSourceTargetID, 10), validEvalTargetVersion).
					Return(evalTargetWithoutSchema, nil)

				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantResp: &evaltargetapi.MockEvalTargetOutputResponse{
				EvalTarget: target.EvalTargetDO2DTO(&entity.EvalTarget{
					ID:             1,
					SpaceID:        validSpaceID,
					SourceTargetID: "456",
					EvalTargetType: 1,
					EvalTargetVersion: &entity.EvalTargetVersion{
						ID:                  1,
						SpaceID:             validSpaceID,
						TargetID:            1,
						SourceTargetVersion: validEvalTargetVersion,
						OutputSchema:        nil,
					},
				}),
				MockOutput: map[string]string{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := app.MockEvalTargetOutput(context.Background(), tt.req)

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
				assert.NotNil(t, resp.EvalTarget)
				assert.NotNil(t, resp.MockOutput)
				if tt.wantResp != nil {
					assert.Equal(t, tt.wantResp.MockOutput, resp.MockOutput)
				}
			}
		})
	}
}

func TestEvalTargetApplicationImpl_AsyncExecuteEvalTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvalTargetService := mocks.NewMockIEvalTargetService(ctrl)

	app := &EvalTargetApplicationImpl{
		auth:              mockAuth,
		evalTargetService: mockEvalTargetService,
	}

	workspaceID := int64(101)
	targetID := int64(202)
	versionID := int64(303)
	inputData := &domain_eval_target.EvalTargetInputData{}
	record := &entity.EvalTargetRecord{ID: 888}

	tests := []struct {
		name        string
		req         *evaltargetapi.AsyncExecuteEvalTargetRequest
		mockSetup   func()
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success",
			req: &evaltargetapi.AsyncExecuteEvalTargetRequest{
				WorkspaceID:         workspaceID,
				EvalTargetID:        targetID,
				EvalTargetVersionID: versionID,
				InputData:           inputData,
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), &rpc.AuthorizationParam{
					ObjectID:      strconv.FormatInt(targetID, 10),
					SpaceID:       workspaceID,
					ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Run), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationTarget)}},
				}).Return(nil)
				mockEvalTargetService.EXPECT().AsyncExecuteTarget(
					gomock.Any(),
					workspaceID,
					targetID,
					versionID,
					gomock.Any(),
					gomock.Any(),
				).Return(record, "callee", nil)
			},
		},
		{
			name: "auth failure",
			req: &evaltargetapi.AsyncExecuteEvalTargetRequest{
				WorkspaceID:         workspaceID,
				EvalTargetID:        targetID,
				EvalTargetVersionID: versionID,
				InputData:           inputData,
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
		{
			name: "service failure",
			req: &evaltargetapi.AsyncExecuteEvalTargetRequest{
				WorkspaceID:         workspaceID,
				EvalTargetID:        targetID,
				EvalTargetVersionID: versionID,
				InputData:           inputData,
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockEvalTargetService.EXPECT().AsyncExecuteTarget(gomock.Any(), workspaceID, targetID, versionID, gomock.Any(), gomock.Any()).
					Return(nil, "", errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			if tc.mockSetup != nil {
				tc.mockSetup()
			}

			resp, err := app.AsyncExecuteEvalTarget(context.Background(), tc.req)

			if tc.wantErr {
				assert.Error(t, err)
				if tc.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tc.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.NotNil(t, resp.InvokeID)
				assert.Equal(t, record.ID, *resp.InvokeID)
				assert.NotNil(t, resp.BaseResp)
			}
		})
	}
}

func TestEvalTargetApplicationImpl_DebugEvalTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvalTargetService := mocks.NewMockIEvalTargetService(ctrl)

	app := &EvalTargetApplicationImpl{
		evalTargetService: mockEvalTargetService,
	}

	workspaceID := int64(1001)
	targetType := domain_eval_target.EvalTargetType_CustomRPCServer
	runtimeParamJSON := "{}"
	content := map[string]*spi.Content{
		"input": {
			ContentType: gptr.Of(spi.ContentType("text")),
			Text:        gptr.Of("hello"),
		},
	}
	paramBytes, _ := stdjson.Marshal(content)
	customRPC := &domain_eval_target.CustomRPCServer{Name: gptr.Of("debug")}
	record := &entity.EvalTargetRecord{
		ID:                   909,
		EvalTargetOutputData: &entity.EvalTargetOutputData{},
		BaseInfo: &entity.BaseInfo{
			CreatedAt: gptr.Of(int64(1)),
			UpdatedAt: gptr.Of(int64(1)),
		},
	}

	tests := []struct {
		name        string
		req         *evaltargetapi.DebugEvalTargetRequest
		mockSetup   func()
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success",
			req: &evaltargetapi.DebugEvalTargetRequest{
				WorkspaceID:    &workspaceID,
				EvalTargetType: &targetType,
				Param:          gptr.Of(string(paramBytes)),
				TargetRuntimeParam: &domaincommon.RuntimeParam{
					JSONValue: gptr.Of(runtimeParamJSON),
				},
				CustomRPCServer: customRPC,
			},
			mockSetup: func() {
				mockEvalTargetService.EXPECT().DebugTarget(gomock.Any(), gomock.Any()).Return(record, nil)
			},
		},
		{
			name: "invalid json",
			req: &evaltargetapi.DebugEvalTargetRequest{
				WorkspaceID:    &workspaceID,
				EvalTargetType: &targetType,
				Param:          gptr.Of("{"),
			},
			mockSetup:   func() {},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "service failure",
			req: &evaltargetapi.DebugEvalTargetRequest{
				WorkspaceID:        &workspaceID,
				EvalTargetType:     &targetType,
				Param:              gptr.Of(string(paramBytes)),
				TargetRuntimeParam: &domaincommon.RuntimeParam{JSONValue: gptr.Of(runtimeParamJSON)},
				CustomRPCServer:    customRPC,
			},
			mockSetup: func() {
				mockEvalTargetService.EXPECT().DebugTarget(gomock.Any(), gomock.Any()).
					Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
		{
			name: "unsupported type",
			req: &evaltargetapi.DebugEvalTargetRequest{
				WorkspaceID:    &workspaceID,
				EvalTargetType: gptr.Of(domain_eval_target.EvalTargetType(0)),
				Param:          gptr.Of(string(paramBytes)),
			},
			mockSetup: func() {},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			if tc.mockSetup != nil {
				tc.mockSetup()
			}

			resp, err := app.DebugEvalTarget(context.Background(), tc.req)

			if tc.wantErr {
				assert.Error(t, err)
				if tc.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tc.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.NotNil(t, resp.EvalTargetRecord)
			}
		})
	}
}

func TestEvalTargetApplicationImpl_AsyncDebugEvalTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvalTargetService := mocks.NewMockIEvalTargetService(ctrl)
	mockEvalAsyncRepo := repomocks.NewMockIEvalAsyncRepo(ctrl)

	app := &EvalTargetApplicationImpl{
		evalTargetService: mockEvalTargetService,
		evalAsyncRepo:     mockEvalAsyncRepo,
	}

	workspaceID := int64(2001)
	targetType := domain_eval_target.EvalTargetType_CustomRPCServer
	runtimeParamJSON := "{}"
	content := map[string]*spi.Content{
		"input": {
			ContentType: gptr.Of(spi.ContentType("text")),
			Text:        gptr.Of("world"),
		},
	}
	paramBytes, _ := stdjson.Marshal(content)
	customRPC := &domain_eval_target.CustomRPCServer{Name: gptr.Of("async")}
	record := &entity.EvalTargetRecord{
		ID:                   707,
		EvalTargetOutputData: &entity.EvalTargetOutputData{},
		BaseInfo: &entity.BaseInfo{
			CreatedAt: gptr.Of(int64(1)),
			UpdatedAt: gptr.Of(int64(1)),
		},
	}

	tests := []struct {
		name        string
		req         *evaltargetapi.AsyncDebugEvalTargetRequest
		mockSetup   func()
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success",
			req: &evaltargetapi.AsyncDebugEvalTargetRequest{
				WorkspaceID:    &workspaceID,
				EvalTargetType: &targetType,
				Param:          gptr.Of(string(paramBytes)),
				TargetRuntimeParam: &domaincommon.RuntimeParam{
					JSONValue: gptr.Of(runtimeParamJSON),
				},
				CustomRPCServer: customRPC,
			},
			mockSetup: func() {
				mockEvalTargetService.EXPECT().AsyncDebugTarget(gomock.Any(), gomock.Any()).Return(record, "callee", nil)
				mockEvalAsyncRepo.EXPECT().SetEvalAsyncCtx(gomock.Any(), strconv.FormatInt(record.ID, 10), gomock.Any()).Return(nil)
			},
		},
		{
			name: "invalid json",
			req: &evaltargetapi.AsyncDebugEvalTargetRequest{
				WorkspaceID:    &workspaceID,
				EvalTargetType: &targetType,
				Param:          gptr.Of("{"),
			},
			mockSetup:   func() {},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "service failure",
			req: &evaltargetapi.AsyncDebugEvalTargetRequest{
				WorkspaceID:        &workspaceID,
				EvalTargetType:     &targetType,
				Param:              gptr.Of(string(paramBytes)),
				TargetRuntimeParam: &domaincommon.RuntimeParam{JSONValue: gptr.Of(runtimeParamJSON)},
				CustomRPCServer:    customRPC,
			},
			mockSetup: func() {
				mockEvalTargetService.EXPECT().AsyncDebugTarget(gomock.Any(), gomock.Any()).
					Return(nil, "", errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
		{
			name: "set async ctx failure",
			req: &evaltargetapi.AsyncDebugEvalTargetRequest{
				WorkspaceID:        &workspaceID,
				EvalTargetType:     &targetType,
				Param:              gptr.Of(string(paramBytes)),
				TargetRuntimeParam: &domaincommon.RuntimeParam{JSONValue: gptr.Of(runtimeParamJSON)},
				CustomRPCServer:    customRPC,
			},
			mockSetup: func() {
				mockEvalTargetService.EXPECT().AsyncDebugTarget(gomock.Any(), gomock.Any()).Return(record, "callee", nil)
				mockEvalAsyncRepo.EXPECT().SetEvalAsyncCtx(gomock.Any(), strconv.FormatInt(record.ID, 10), gomock.Any()).
					Return(errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
		{
			name: "unsupported type",
			req: &evaltargetapi.AsyncDebugEvalTargetRequest{
				WorkspaceID:    &workspaceID,
				EvalTargetType: gptr.Of(domain_eval_target.EvalTargetType(0)),
				Param:          gptr.Of(string(paramBytes)),
			},
			mockSetup: func() {},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			if tc.mockSetup != nil {
				tc.mockSetup()
			}

			resp, err := app.AsyncDebugEvalTarget(context.Background(), tc.req)

			if tc.wantErr {
				assert.Error(t, err)
				if tc.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tc.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, record.ID, resp.InvokeID)
				assert.Equal(t, gptr.Of("callee"), resp.Callee)
				assert.NotNil(t, resp.BaseResp)
			}
		})
	}
}

func TestEvalTargetApplicationImpl_ExecuteEvalTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvalTargetService := mocks.NewMockIEvalTargetService(ctrl)

	app := &EvalTargetApplicationImpl{
		auth:              mockAuth,
		evalTargetService: mockEvalTargetService,
	}

	workspaceID := int64(100)
	targetID := int64(200)
	versionID := int64(300)
	inputData := &domain_eval_target.EvalTargetInputData{}
	record := &entity.EvalTargetRecord{ID: 1, SpaceID: workspaceID, TargetID: targetID, BaseInfo: &entity.BaseInfo{}}

	tests := []struct {
		name        string
		req         *evaltargetapi.ExecuteEvalTargetRequest
		mockSetup   func()
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success",
			req: &evaltargetapi.ExecuteEvalTargetRequest{
				WorkspaceID:         workspaceID,
				EvalTargetID:        targetID,
				EvalTargetVersionID: versionID,
				InputData:           inputData,
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), &rpc.AuthorizationParam{
					ObjectID:      strconv.FormatInt(targetID, 10),
					SpaceID:       workspaceID,
					ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Run), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationTarget)}},
				}).Return(nil)
				mockEvalTargetService.EXPECT().ExecuteTarget(
					gomock.Any(),
					workspaceID,
					targetID,
					versionID,
					gomock.AssignableToTypeOf(&entity.ExecuteTargetCtx{}),
					gomock.AssignableToTypeOf(&entity.EvalTargetInputData{}),
				).Return(record, nil)
			},
		},
		{
			name:        "nil request",
			req:         nil,
			mockSetup:   func() {},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "nil input data",
			req: &evaltargetapi.ExecuteEvalTargetRequest{
				WorkspaceID:         workspaceID,
				EvalTargetID:        targetID,
				EvalTargetVersionID: versionID,
			},
			mockSetup:   func() {},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "auth failure",
			req: &evaltargetapi.ExecuteEvalTargetRequest{
				WorkspaceID:         workspaceID,
				EvalTargetID:        targetID,
				EvalTargetVersionID: versionID,
				InputData:           inputData,
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
		{
			name: "service failure",
			req: &evaltargetapi.ExecuteEvalTargetRequest{
				WorkspaceID:         workspaceID,
				EvalTargetID:        targetID,
				EvalTargetVersionID: versionID,
				InputData:           inputData,
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockEvalTargetService.EXPECT().ExecuteTarget(gomock.Any(), workspaceID, targetID, versionID, gomock.Any(), gomock.Any()).
					Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			if tc.mockSetup != nil {
				tc.mockSetup()
			}

			resp, err := app.ExecuteEvalTarget(context.Background(), tc.req)

			if tc.wantErr {
				assert.Error(t, err)
				if tc.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tc.wantErrCode, statusErr.Code())
				}
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.NotNil(t, resp.EvalTargetRecord)
		})
	}
}

func TestEvalTargetApplicationImpl_GetEvalTargetRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvalTargetService := mocks.NewMockIEvalTargetService(ctrl)

	app := &EvalTargetApplicationImpl{
		auth:              mockAuth,
		evalTargetService: mockEvalTargetService,
	}

	workspaceID := int64(111)
	recordID := int64(222)
	record := &entity.EvalTargetRecord{TargetID: 333, ID: recordID, BaseInfo: &entity.BaseInfo{}}

	tests := []struct {
		name        string
		req         *evaltargetapi.GetEvalTargetRecordRequest
		mockSetup   func()
		wantErr     bool
		wantErrCode int32
	}{
		{
			name:        "nil request",
			req:         nil,
			mockSetup:   func() {},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "service failure",
			req: &evaltargetapi.GetEvalTargetRecordRequest{
				WorkspaceID:        workspaceID,
				EvalTargetRecordID: recordID,
			},
			mockSetup: func() {
				mockEvalTargetService.EXPECT().GetRecordByID(gomock.Any(), workspaceID, recordID).
					Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
		{
			name: "record not found",
			req: &evaltargetapi.GetEvalTargetRecordRequest{
				WorkspaceID:        workspaceID,
				EvalTargetRecordID: recordID,
			},
			mockSetup: func() {
				mockEvalTargetService.EXPECT().GetRecordByID(gomock.Any(), workspaceID, recordID).Return(nil, nil)
			},
		},
		{
			name: "auth failure",
			req: &evaltargetapi.GetEvalTargetRecordRequest{
				WorkspaceID:        workspaceID,
				EvalTargetRecordID: recordID,
			},
			mockSetup: func() {
				mockEvalTargetService.EXPECT().GetRecordByID(gomock.Any(), workspaceID, recordID).Return(record, nil)
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
		{
			name: "success",
			req: &evaltargetapi.GetEvalTargetRecordRequest{
				WorkspaceID:        workspaceID,
				EvalTargetRecordID: recordID,
			},
			mockSetup: func() {
				mockEvalTargetService.EXPECT().GetRecordByID(gomock.Any(), workspaceID, recordID).Return(record, nil)
				mockAuth.EXPECT().Authorization(gomock.Any(), &rpc.AuthorizationParam{
					ObjectID:      strconv.FormatInt(record.TargetID, 10),
					SpaceID:       workspaceID,
					ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationTarget)}},
				}).Return(nil)
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			if tc.mockSetup != nil {
				tc.mockSetup()
			}

			resp, err := app.GetEvalTargetRecord(context.Background(), tc.req)

			if tc.wantErr {
				assert.Error(t, err)
				if tc.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tc.wantErrCode, statusErr.Code())
				}
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, resp)
		})
	}
}

func TestEvalTargetApplicationImpl_BatchGetEvalTargetRecords(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvalTargetService := mocks.NewMockIEvalTargetService(ctrl)

	app := &EvalTargetApplicationImpl{
		auth:              mockAuth,
		evalTargetService: mockEvalTargetService,
	}

	workspaceID := int64(777)
	recordIDs := []int64{1, 2}
	records := []*entity.EvalTargetRecord{{ID: 1, BaseInfo: &entity.BaseInfo{}}, {ID: 2, BaseInfo: &entity.BaseInfo{}}}

	tests := []struct {
		name        string
		req         *evaltargetapi.BatchGetEvalTargetRecordsRequest
		mockSetup   func()
		wantErr     bool
		wantErrCode int32
	}{
		{
			name:        "nil request",
			req:         nil,
			mockSetup:   func() {},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "auth failure",
			req: &evaltargetapi.BatchGetEvalTargetRecordsRequest{
				WorkspaceID:         workspaceID,
				EvalTargetRecordIds: recordIDs,
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
		{
			name: "service failure",
			req: &evaltargetapi.BatchGetEvalTargetRecordsRequest{
				WorkspaceID:         workspaceID,
				EvalTargetRecordIds: recordIDs,
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockEvalTargetService.EXPECT().BatchGetRecordByIDs(gomock.Any(), workspaceID, recordIDs).
					Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
		{
			name: "success",
			req: &evaltargetapi.BatchGetEvalTargetRecordsRequest{
				WorkspaceID:         workspaceID,
				EvalTargetRecordIds: recordIDs,
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), &rpc.AuthorizationParam{
					ObjectID:      strconv.FormatInt(workspaceID, 10),
					SpaceID:       workspaceID,
					ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluationTarget"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
				}).Return(nil)
				mockEvalTargetService.EXPECT().BatchGetRecordByIDs(gomock.Any(), workspaceID, recordIDs).Return(records, nil)
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			if tc.mockSetup != nil {
				tc.mockSetup()
			}

			resp, err := app.BatchGetEvalTargetRecords(context.Background(), tc.req)

			if tc.wantErr {
				assert.Error(t, err)
				if tc.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tc.wantErrCode, statusErr.Code())
				}
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Len(t, resp.EvalTargetRecords, len(recordIDs))
		})
	}
}
