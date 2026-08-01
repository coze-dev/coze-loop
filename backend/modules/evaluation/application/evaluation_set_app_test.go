// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset_job"
	domain_common "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	domain_eval_set "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_set"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_set"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	metricsmock "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	userinfomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/userinfo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestEvaluationSetApplicationImpl_CreateEvaluationSetWithImport(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockSvc := servicemocks.NewMockIEvaluationSetService(ctrl)
	mockMetric := metricsmock.NewMockEvaluationSetMetrics(ctrl)
	mockAuthorizer := servicemocks.NewMockResourceAccessAuthorizer(ctrl)

	app := &EvaluationSetApplicationImpl{
		auth:                     mockAuth,
		evaluationSetService:     mockSvc,
		metric:                   mockMetric,
		resourceAccessAuthorizer: mockAuthorizer,
	}

	workspaceID := int64(1001)

	baseReq := func() *eval_set.CreateEvaluationSetWithImportRequest {
		return &eval_set.CreateEvaluationSetWithImportRequest{
			WorkspaceID:         workspaceID,
			Name:                gptr.Of("dataset"),
			EvaluationSetSchema: &domain_eval_set.EvaluationSetSchema{},
			SourceType:          gptr.Of(dataset_job.SourceType_File),
			Source:              &dataset_job.DatasetIOEndpoint{File: &dataset_job.DatasetIOFile{}},
		}
	}

	tests := []struct {
		name    string
		req     *eval_set.CreateEvaluationSetWithImportRequest
		setup   func()
		wantErr int32
		check   func(t *testing.T, resp *eval_set.CreateEvaluationSetWithImportResponse)
	}{
		{
			name: "缺少name",
			req: func() *eval_set.CreateEvaluationSetWithImportRequest {
				r := baseReq()
				r.Name = nil
				return r
			}(),
			setup: func() {
				mockMetric.EXPECT().EmitCreate(workspaceID, gomock.Any())
			},
			wantErr: errno.CommonInvalidParamCode,
		},
		{
			name: "缺少schema",
			req: func() *eval_set.CreateEvaluationSetWithImportRequest {
				r := baseReq()
				r.EvaluationSetSchema = nil
				return r
			}(),
			setup: func() {
				mockMetric.EXPECT().EmitCreate(workspaceID, gomock.Any())
			},
			wantErr: errno.CommonInvalidParamCode,
		},
		{
			name: "缺少source",
			req: func() *eval_set.CreateEvaluationSetWithImportRequest {
				r := baseReq()
				r.Source = nil
				return r
			}(),
			setup: func() {
				mockMetric.EXPECT().EmitCreate(workspaceID, gomock.Any())
			},
			wantErr: errno.CommonInvalidParamCode,
		},
		{
			name: "鉴权失败",
			req:  baseReq(),
			setup: func() {
				mockMetric.EXPECT().EmitCreate(workspaceID, gomock.Any())
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationParam{})).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr: errno.CommonNoPermissionCode,
		},
		{
			name: "服务错误",
			req:  baseReq(),
			setup: func() {
				mockMetric.EXPECT().EmitCreate(workspaceID, gomock.Any())
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationParam{})).Return(nil)
				mockSvc.EXPECT().CreateEvaluationSetWithImport(gomock.Any(), gomock.AssignableToTypeOf(&entity.CreateEvaluationSetWithImportParam{})).Return(int64(0), int64(0), errors.New("svc err"))
			},
			wantErr: -1,
		},
		{
			name: "成功",
			req:  baseReq(),
			setup: func() {
				mockMetric.EXPECT().EmitCreate(workspaceID, gomock.Any())
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationParam{})).Return(nil)
				mockSvc.EXPECT().CreateEvaluationSetWithImport(gomock.Any(), gomock.AssignableToTypeOf(&entity.CreateEvaluationSetWithImportParam{})).Return(int64(12345), int64(67890), nil)
			},
			check: func(t *testing.T, resp *eval_set.CreateEvaluationSetWithImportResponse) {
				if assert.NotNil(t, resp) && assert.NotNil(t, resp.EvaluationSetID) && assert.NotNil(t, resp.JobID) {
					assert.Equal(t, int64(12345), resp.GetEvaluationSetID())
					assert.Equal(t, int64(67890), resp.GetJobID())
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup()
			}
			resp, err := app.CreateEvaluationSetWithImport(context.Background(), tc.req)
			if tc.wantErr != 0 {
				assert.Error(t, err)
				if tc.wantErr > 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tc.wantErr, statusErr.Code())
				}
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				if tc.check != nil {
					tc.check(t, resp)
				}
			}
		})
	}
}

func TestEvaluationSetApplicationImpl_ListEvaluationSets_SharedPagination(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const workspaceID = int64(1001)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockSvc := servicemocks.NewMockIEvaluationSetService(ctrl)
	mockAuthorizer := servicemocks.NewMockResourceAccessAuthorizer(ctrl)
	mockUserInfo := userinfomocks.NewMockUserInfoService(ctrl)
	app := &EvaluationSetApplicationImpl{
		auth:                     mockAuth,
		evaluationSetService:     mockSvc,
		resourceAccessAuthorizer: mockAuthorizer,
		userInfoService:          mockUserInfo,
	}
	accessCtxs := []*entity.ResourceAccessContext{
		{ResourceSpaceID: 20, ResourceID: 3, AccessMode: entity.AccessModeShared, AccessLevel: entity.SharedAccessLevelReadable},
		{ResourceSpaceID: 10, ResourceID: 2, AccessMode: entity.AccessModeShared, AccessLevel: entity.SharedAccessLevelReadable},
		{ResourceSpaceID: 10, ResourceID: 1, AccessMode: entity.AccessModeShared, AccessLevel: entity.SharedAccessLevelReadable},
	}
	mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil).Times(2)
	mockAuthorizer.EXPECT().ListSharedResources(gomock.Any(), gomock.Any()).Return(accessCtxs, nil).Times(2)
	mockUserInfo.EXPECT().PackUserInfo(gomock.Any(), gomock.Any()).Times(2)
	mockSvc.EXPECT().BatchGetEvaluationSets(gomock.Any(), gptr.Of(workspaceID), []int64{1, 2}, gomock.Any(), gomock.Any()).Return(
		[]*entity.EvaluationSet{{ID: 2, SpaceID: 10}, {ID: 1, SpaceID: 10}},
		nil,
	)

	pageSize := int32(2)
	req := &eval_set.ListEvaluationSetsRequest{
		WorkspaceID: workspaceID,
		PageSize:    &pageSize,
		SharedOption: &domain_common.SharedResourceOption{
			IsShared: gptr.Of(true),
		},
	}
	firstPage, err := app.ListEvaluationSets(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, firstPage.EvaluationSets, 2)
	assert.Equal(t, int64(1), firstPage.EvaluationSets[0].GetID())
	assert.Equal(t, int64(2), firstPage.EvaluationSets[1].GetID())
	assert.Equal(t, int64(3), firstPage.GetTotal())
	require.NotNil(t, firstPage.NextPageToken)

	mockSvc.EXPECT().BatchGetEvaluationSets(gomock.Any(), gptr.Of(workspaceID), []int64{3}, gomock.Any(), gomock.Any()).Return(
		[]*entity.EvaluationSet{{ID: 3, SpaceID: 20}},
		nil,
	)
	req.PageToken = firstPage.NextPageToken
	secondPage, err := app.ListEvaluationSets(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, secondPage.EvaluationSets, 1)
	assert.Equal(t, int64(3), secondPage.EvaluationSets[0].GetID())
	assert.Nil(t, secondPage.NextPageToken)
}

func TestEvaluationSetApplicationImpl_ParseImportSourceFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockSvc := servicemocks.NewMockIEvaluationSetService(ctrl)

	app := &EvaluationSetApplicationImpl{
		auth:                     mockAuth,
		evaluationSetService:     mockSvc,
		resourceAccessAuthorizer: servicemocks.NewMockResourceAccessAuthorizer(ctrl),
	}

	workspaceID := int64(2002)

	baseReq := func() *eval_set.ParseImportSourceFileRequest {
		return &eval_set.ParseImportSourceFileRequest{
			WorkspaceID: workspaceID,
			File:        &dataset_job.DatasetIOFile{Path: "/path"},
		}
	}

	tests := []struct {
		name    string
		req     *eval_set.ParseImportSourceFileRequest
		setup   func()
		wantErr int32
		check   func(t *testing.T, resp *eval_set.ParseImportSourceFileResponse)
	}{
		{"nil req", nil, func() {}, errno.CommonInvalidParamCode, nil},
		{
			name:    "nil file",
			req:     func() *eval_set.ParseImportSourceFileRequest { r := baseReq(); r.File = nil; return r }(),
			setup:   func() {},
			wantErr: errno.CommonInvalidParamCode,
		},
		{
			name: "鉴权失败",
			req:  baseReq(),
			setup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationParam{})).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr: errno.CommonNoPermissionCode,
		},
		{
			name: "服务错误",
			req:  baseReq(),
			setup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationParam{})).Return(nil)
				mockSvc.EXPECT().ParseImportSourceFile(gomock.Any(), gomock.AssignableToTypeOf(&entity.ParseImportSourceFileParam{})).Return(nil, errors.New("svc err"))
			},
			wantErr: -1,
		},
		{
			name: "成功",
			req:  baseReq(),
			setup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationParam{})).Return(nil)
				res := &entity.ParseImportSourceFileResult{
					Bytes:                    int64(123),
					FieldSchemas:             []*entity.FieldSchema{{Name: "f1"}},
					Conflicts:                []*entity.ConflictField{{FieldName: "c1"}},
					FilesWithAmbiguousColumn: []string{"a.csv"},
				}
				mockSvc.EXPECT().ParseImportSourceFile(gomock.Any(), gomock.AssignableToTypeOf(&entity.ParseImportSourceFileParam{})).Return(res, nil)
			},
			check: func(t *testing.T, resp *eval_set.ParseImportSourceFileResponse) {
				if assert.NotNil(t, resp) {
					assert.NotNil(t, resp.BaseResp)
					assert.Equal(t, int64(123), resp.GetBytes())
					assert.NotNil(t, resp.FieldSchemas)
					assert.NotNil(t, resp.Conflicts)
					assert.Equal(t, []string{"a.csv"}, resp.FilesWithAmbiguousColumn)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup()
			}
			resp, err := app.ParseImportSourceFile(context.Background(), tc.req)
			if tc.wantErr != 0 {
				assert.Error(t, err)
				if tc.wantErr > 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tc.wantErr, statusErr.Code())
				}
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				if tc.check != nil {
					tc.check(t, resp)
				}
			}
		})
	}
}

func TestEvaluationSetApplicationImpl_EvaluationSetValidateMultiPartData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockSvc := servicemocks.NewMockIEvaluationSetService(ctrl)

	app := &EvaluationSetApplicationImpl{
		auth:                     mockAuth,
		evaluationSetService:     mockSvc,
		resourceAccessAuthorizer: servicemocks.NewMockResourceAccessAuthorizer(ctrl),
	}

	spaceID := int64(2002)

	baseReq := func() *eval_set.ValidateEvaluationSetMultiPartDataRequest {
		return &eval_set.ValidateEvaluationSetMultiPartDataRequest{
			SpaceID:     spaceID,
			PreviewData: []string{"https://example.com/a.png"},
		}
	}
	tests := []struct {
		name    string
		req     *eval_set.ValidateEvaluationSetMultiPartDataRequest
		setup   func()
		wantErr int32
		check   func(t *testing.T, resp *eval_set.ValidateEvaluationSetMultiPartDataResponse)
	}{
		{"nil req", nil, func() {}, errno.CommonInvalidParamCode, nil},
		{
			name: "鉴权失败",
			req:  baseReq(),
			setup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationParam{})).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr: errno.CommonNoPermissionCode,
		},
		{
			name: "成功",
			req:  baseReq(),
			setup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationParam{})).Return(nil)
				mockSvc.EXPECT().ValidateMultiPartData(gomock.Any(), spaceID, []string{"https://example.com/a.png"}, gomock.Nil()).Return(nil, nil)
			},
			check: func(t *testing.T, resp *eval_set.ValidateEvaluationSetMultiPartDataResponse) {
				if assert.NotNil(t, resp) {
					assert.NotNil(t, resp.BaseResp)
					assert.Nil(t, resp.AttachmentUrlsCheckDetail)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup()
			}
			resp, err := app.ValidateEvaluationSetMultiPartData(context.Background(), tc.req)
			if tc.wantErr != 0 {
				assert.Error(t, err)
				if tc.wantErr > 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tc.wantErr, statusErr.Code())
				}
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				if tc.check != nil {
					tc.check(t, resp)
				}
			}
		})
	}
}

func TestEvaluationSetApplicationImpl_UpdateEvaluationSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvalSetSvc := servicemocks.NewMockIEvaluationSetService(ctrl)

	app := &EvaluationSetApplicationImpl{
		auth:                 mockAuth,
		evaluationSetService: mockEvalSetSvc,
	}

	workspaceID := int64(3003)
	evaluationSetID := int64(4004)
	validSet := &entity.EvaluationSet{ID: evaluationSetID, SpaceID: workspaceID + 1, BaseInfo: &entity.BaseInfo{CreatedBy: &entity.UserInfo{UserID: gptr.Of("owner")}}}

	baseReq := func() *eval_set.UpdateEvaluationSetRequest {
		return &eval_set.UpdateEvaluationSetRequest{
			WorkspaceID:     workspaceID,
			EvaluationSetID: evaluationSetID,
			Name:            gptr.Of("new name"),
			Description:     gptr.Of("new desc"),
		}
	}

	tests := []struct {
		name    string
		req     *eval_set.UpdateEvaluationSetRequest
		setup   func()
		wantErr int32
		check   func(t *testing.T, resp *eval_set.UpdateEvaluationSetResponse)
	}{
		{
			name: "nil req",
			req:  nil,
			setup: func() {
				mockEvalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Times(0)
				mockEvalSetSvc.EXPECT().UpdateEvaluationSet(gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr: errno.CommonInvalidParamCode,
		},
		{
			name: "get evaluation set error",
			req:  baseReq(),
			setup: func() {
				mockEvalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gptr.Of(workspaceID), evaluationSetID, gomock.Nil(), gomock.Nil()).Return(nil, errors.New("get err"))
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Times(0)
				mockEvalSetSvc.EXPECT().UpdateEvaluationSet(gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr: -1,
		},
		{
			name: "evaluation set not found",
			req:  baseReq(),
			setup: func() {
				mockEvalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gptr.Of(workspaceID), evaluationSetID, gomock.Nil(), gomock.Nil()).Return(nil, nil)
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Times(0)
				mockEvalSetSvc.EXPECT().UpdateEvaluationSet(gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr: errno.ResourceNotFoundCode,
		},
		{
			name: "auth failed",
			req:  baseReq(),
			setup: func() {
				mockEvalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gptr.Of(workspaceID), evaluationSetID, gomock.Nil(), gomock.Nil()).Return(validSet, nil)
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
				mockEvalSetSvc.EXPECT().UpdateEvaluationSet(gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr: errno.CommonNoPermissionCode,
		},
		{
			name: "update service error",
			req:  baseReq(),
			setup: func() {
				mockEvalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gptr.Of(workspaceID), evaluationSetID, gomock.Nil(), gomock.Nil()).Return(validSet, nil)
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(nil)
				mockEvalSetSvc.EXPECT().UpdateEvaluationSet(gomock.Any(), gomock.AssignableToTypeOf(&entity.UpdateEvaluationSetParam{})).Return(errors.New("update err"))
			},
			wantErr: -1,
		},
		{
			name: "success",
			req:  baseReq(),
			setup: func() {
				mockEvalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gptr.Of(workspaceID), evaluationSetID, gomock.Nil(), gomock.Nil()).Return(validSet, nil)
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).DoAndReturn(func(_ context.Context, p *rpc.AuthorizationWithoutSPIParam) error {
					assert.Equal(t, strconv.FormatInt(validSet.ID, 10), p.ObjectID)
					assert.Equal(t, workspaceID, p.SpaceID)
					assert.Equal(t, validSet.SpaceID, p.ResourceSpaceID)
					assert.Equal(t, validSet.BaseInfo.CreatedBy.UserID, p.OwnerID)
					if assert.Len(t, p.ActionObjects, 1) {
						assert.Equal(t, consts.Edit, gptr.Indirect(p.ActionObjects[0].Action))
						assert.Equal(t, rpc.AuthEntityType_EvaluationSet, gptr.Indirect(p.ActionObjects[0].EntityType))
					}
					return nil
				})
				mockEvalSetSvc.EXPECT().UpdateEvaluationSet(gomock.Any(), gomock.AssignableToTypeOf(&entity.UpdateEvaluationSetParam{})).DoAndReturn(func(_ context.Context, p *entity.UpdateEvaluationSetParam) error {
					assert.Equal(t, workspaceID, p.SpaceID)
					assert.Equal(t, evaluationSetID, p.EvaluationSetID)
					assert.Equal(t, gptr.Indirect(baseReq().Name), gptr.Indirect(p.Name))
					assert.Equal(t, gptr.Indirect(baseReq().Description), gptr.Indirect(p.Description))
					return nil
				})
			},
			check: func(t *testing.T, resp *eval_set.UpdateEvaluationSetResponse) {
				assert.NotNil(t, resp)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup()
			}
			resp, err := app.UpdateEvaluationSet(context.Background(), tc.req)
			if tc.wantErr != 0 {
				assert.Error(t, err)
				if tc.wantErr > 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tc.wantErr, statusErr.Code())
				}
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				if tc.check != nil {
					tc.check(t, resp)
				}
			}
		})
	}
}

func TestEvaluationSetApplicationImpl_GetEvaluationSetItemField(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvalSetSvc := servicemocks.NewMockIEvaluationSetService(ctrl)
	mockItemSvc := servicemocks.NewMockEvaluationSetItemService(ctrl)

	app := &EvaluationSetApplicationImpl{
		auth:                     mockAuth,
		evaluationSetService:     mockEvalSetSvc,
		evaluationSetItemService: mockItemSvc,
	}

	workspaceID := int64(3003)
	evalSetID := int64(4004)
	itemPK := int64(5555)
	fieldName := "field"
	turnID := gptr.Of(int64(777))

	validSet := &entity.EvaluationSet{ID: evalSetID, SpaceID: workspaceID, BaseInfo: &entity.BaseInfo{CreatedBy: &entity.UserInfo{UserID: gptr.Of("owner")}}}

	baseReq := func() *eval_set.GetEvaluationSetItemFieldRequest {
		return &eval_set.GetEvaluationSetItemFieldRequest{
			WorkspaceID:     workspaceID,
			EvaluationSetID: evalSetID,
			ItemPk:          itemPK,
			FieldName:       fieldName,
			TurnID:          turnID,
		}
	}

	tests := []struct {
		name    string
		req     *eval_set.GetEvaluationSetItemFieldRequest
		setup   func()
		wantErr int32
		check   func(t *testing.T, resp *eval_set.GetEvaluationSetItemFieldResponse)
	}{
		{"nil req", nil, func() {}, errno.CommonInvalidParamCode, nil},
		{
			name: "set not found",
			req:  baseReq(),
			setup: func() {
				mockEvalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gptr.Of(workspaceID), evalSetID, gomock.AssignableToTypeOf(gptr.Of(true)), gomock.Nil()).Return(nil, nil)
			},
			wantErr: errno.ResourceNotFoundCode,
		},
		{
			name: "auth failed",
			req:  baseReq(),
			setup: func() {
				mockEvalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gptr.Of(workspaceID), evalSetID, gomock.AssignableToTypeOf(gptr.Of(true)), gomock.Nil()).Return(validSet, nil)
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr: errno.CommonNoPermissionCode,
		},
		{
			name: "get field error",
			req:  baseReq(),
			setup: func() {
				mockEvalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gptr.Of(workspaceID), evalSetID, gomock.AssignableToTypeOf(gptr.Of(true)), gomock.Nil()).Return(validSet, nil)
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(nil)
				mockItemSvc.EXPECT().GetEvaluationSetItemField(gomock.Any(), gomock.AssignableToTypeOf(&entity.GetEvaluationSetItemFieldParam{})).Return(nil, errors.New("svc err"))
			},
			wantErr: -1,
		},
		{
			name: "成功",
			req:  baseReq(),
			setup: func() {
				mockEvalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gptr.Of(workspaceID), evalSetID, gomock.AssignableToTypeOf(gptr.Of(true)), gomock.Nil()).Return(validSet, nil)
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).DoAndReturn(func(_ context.Context, p *rpc.AuthorizationWithoutSPIParam) error {
					assert.Equal(t, strconv.FormatInt(evalSetID, 10), p.ObjectID)
					assert.Equal(t, workspaceID, p.SpaceID)
					return nil
				})
				fd := &entity.FieldData{Name: fieldName}
				mockItemSvc.EXPECT().GetEvaluationSetItemField(gomock.Any(), gomock.AssignableToTypeOf(&entity.GetEvaluationSetItemFieldParam{})).DoAndReturn(func(_ context.Context, p *entity.GetEvaluationSetItemFieldParam) (*entity.FieldData, error) {
					assert.Equal(t, workspaceID, p.SpaceID)
					assert.Equal(t, evalSetID, p.EvaluationSetID)
					assert.Equal(t, itemPK, p.ItemPK)
					assert.Equal(t, fieldName, p.FieldName)
					assert.Equal(t, gptr.Indirect(turnID), gptr.Indirect(p.TurnID))
					return fd, nil
				})
			},
			check: func(t *testing.T, resp *eval_set.GetEvaluationSetItemFieldResponse) {
				if assert.NotNil(t, resp) && assert.NotNil(t, resp.FieldData) {
					assert.Equal(t, fieldName, resp.FieldData.GetName())
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup()
			}
			resp, err := app.GetEvaluationSetItemField(context.Background(), tc.req)
			if tc.wantErr != 0 {
				assert.Error(t, err)
				if tc.wantErr > 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tc.wantErr, statusErr.Code())
				}
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				if tc.check != nil {
					tc.check(t, resp)
				}
			}
		})
	}
}

func TestEvaluationSetApplicationImpl_ListEvaluationSetVersions_NonSharedPreservesPagination(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		workspaceID = int64(1001)
		setID       = int64(2001)
	)
	mockSetSvc := servicemocks.NewMockIEvaluationSetService(ctrl)
	mockVersionSvc := servicemocks.NewMockEvaluationSetVersionService(ctrl)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockUserInfo := userinfomocks.NewMockUserInfoService(ctrl)
	app := &EvaluationSetApplicationImpl{
		auth:                        mockAuth,
		evaluationSetService:        mockSetSvc,
		evaluationSetVersionService: mockVersionSvc,
		userInfoService:             mockUserInfo,
	}

	set := &entity.EvaluationSet{ID: setID, SpaceID: workspaceID}
	mockSetSvc.EXPECT().
		GetEvaluationSet(gomock.Any(), gptr.Of(workspaceID), setID, nil, nil).
		Return(set, nil)
	mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, param *rpc.AuthorizationWithoutSPIParam) error {
			require.Len(t, param.ActionObjects, 1)
			assert.Equal(t, consts.Read, gptr.Indirect(param.ActionObjects[0].Action))
			assert.Equal(t, workspaceID, param.ResourceSpaceID)
			return nil
		},
	)

	pageSize := int32(7)
	pageNumber := int32(3)
	pageToken := "dataset-token"
	versionLike := "v"
	total := int64(9)
	nextToken := "dataset-next"
	mockVersionSvc.EXPECT().
		ListEvaluationSetVersions(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, param *entity.ListEvaluationSetVersionsParam) ([]*entity.EvaluationSetVersion, *int64, *string, error) {
			assert.Equal(t, workspaceID, param.SpaceID)
			assert.Equal(t, setID, param.EvaluationSetID)
			assert.Equal(t, &pageSize, param.PageSize)
			assert.Equal(t, &pageNumber, param.PageNumber)
			assert.Equal(t, &pageToken, param.PageToken)
			assert.Equal(t, &versionLike, param.VersionLike)
			assert.Nil(t, param.SharedOption)
			return []*entity.EvaluationSetVersion{{ID: 11, Version: "v1"}}, &total, &nextToken, nil
		})
	mockUserInfo.EXPECT().PackUserInfo(gomock.Any(), gomock.Any())

	resp, err := app.ListEvaluationSetVersions(context.Background(), &eval_set.ListEvaluationSetVersionsRequest{
		WorkspaceID:     workspaceID,
		EvaluationSetID: setID,
		PageSize:        &pageSize,
		PageNumber:      &pageNumber,
		PageToken:       &pageToken,
		VersionLike:     &versionLike,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Versions, 1)
	assert.Equal(t, total, resp.GetTotal())
	assert.Equal(t, nextToken, resp.GetNextPageToken())
}

func TestEvaluationSetApplicationImpl_ListEvaluationSetVersions_SharedSpecifiedFiltersBeforePagination(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		workspaceID = int64(1001)
		sourceID    = int64(3001)
		setID       = int64(2001)
	)
	mockSetSvc := servicemocks.NewMockIEvaluationSetService(ctrl)
	mockVersionSvc := servicemocks.NewMockEvaluationSetVersionService(ctrl)
	mockAuthorizer := servicemocks.NewMockResourceAccessAuthorizer(ctrl)
	mockUserInfo := userinfomocks.NewMockUserInfoService(ctrl)
	app := &EvaluationSetApplicationImpl{
		evaluationSetService:        mockSetSvc,
		evaluationSetVersionService: mockVersionSvc,
		resourceAccessAuthorizer:    mockAuthorizer,
		userInfoService:             mockUserInfo,
	}

	set := &entity.EvaluationSet{ID: setID, SpaceID: sourceID, LatestVersion: "v5"}
	mockSetSvc.EXPECT().
		GetEvaluationSet(gomock.Any(), gptr.Of(workspaceID), setID, nil, gomock.Any()).
		Return(set, nil)
	mockAuthorizer.EXPECT().
		AuthorizeRead(gomock.Any(), gomock.Any()).
		Return(&entity.ResourceAccessContext{
			CallerSpaceID:   workspaceID,
			ResourceSpaceID: sourceID,
			AccessMode:      entity.AccessModeShared,
			AccessLevel:     entity.SharedAccessLevelReadable,
			VersionPolicy:   entity.SharedVersionPolicySpecified,
			SpecifiedIDs:    []int64{4, 2},
		}, nil)

	scanToken := "scan-next"
	gomock.InOrder(
		mockVersionSvc.EXPECT().
			ListEvaluationSetVersions(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, param *entity.ListEvaluationSetVersionsParam) ([]*entity.EvaluationSetVersion, *int64, *string, error) {
				assert.Equal(t, int32(maxSharedPageSize), gptr.Indirect(param.PageSize))
				assert.Nil(t, param.PageToken)
				require.NotNil(t, param.SharedOption)
				assert.Equal(t, sourceID, gptr.Indirect(param.SharedOption.SourceSpaceID))
				return []*entity.EvaluationSetVersion{
					{ID: 5, Version: "v5"},
					{ID: 4, Version: "v4"},
				}, nil, &scanToken, nil
			}),
		mockVersionSvc.EXPECT().
			ListEvaluationSetVersions(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, param *entity.ListEvaluationSetVersionsParam) ([]*entity.EvaluationSetVersion, *int64, *string, error) {
				require.NotNil(t, param.PageToken)
				assert.Equal(t, scanToken, *param.PageToken)
				return []*entity.EvaluationSetVersion{
					{ID: 3, Version: "v3"},
					{ID: 2, Version: "v2"},
				}, nil, nil, nil
			}),
	)
	mockUserInfo.EXPECT().PackUserInfo(gomock.Any(), gomock.Any())

	pageSize := int32(1)
	resp, err := app.ListEvaluationSetVersions(context.Background(), &eval_set.ListEvaluationSetVersionsRequest{
		WorkspaceID:     workspaceID,
		EvaluationSetID: setID,
		PageSize:        &pageSize,
		SharedOption: &domain_common.SharedResourceOption{
			IsShared:      gptr.Of(true),
			SourceSpaceID: gptr.Of(sourceID),
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Versions, 1)
	assert.Equal(t, int64(4), resp.Versions[0].GetID())
	assert.Equal(t, int64(2), resp.GetTotal())
	assert.NotEmpty(t, resp.GetNextPageToken())
}

func TestEvaluationSetApplicationImpl_SharedExecuteVersionSchemaRedaction(t *testing.T) {
	const (
		workspaceID = int64(1001)
		sourceID    = int64(3001)
		setID       = int64(2001)
		versionID   = int64(4001)
	)
	sharedOption := &domain_common.SharedResourceOption{
		IsShared:      gptr.Of(true),
		SourceSpaceID: gptr.Of(sourceID),
	}
	accessCtx := &entity.ResourceAccessContext{
		CallerSpaceID:   workspaceID,
		ResourceSpaceID: sourceID,
		AccessMode:      entity.AccessModeShared,
		AccessLevel:     entity.SharedAccessLevelExecute,
		VersionPolicy:   entity.SharedVersionPolicyAll,
	}

	t.Run("get version hides schema without mutating domain entity", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockVersionSvc := servicemocks.NewMockEvaluationSetVersionService(ctrl)
		mockAuthorizer := servicemocks.NewMockResourceAccessAuthorizer(ctrl)
		mockUserInfo := userinfomocks.NewMockUserInfoService(ctrl)
		app := &EvaluationSetApplicationImpl{
			evaluationSetVersionService: mockVersionSvc,
			resourceAccessAuthorizer:    mockAuthorizer,
			userInfoService:             mockUserInfo,
		}

		version := &entity.EvaluationSetVersion{
			ID:              versionID,
			EvaluationSetID: setID,
			Version:         "v1",
			EvaluationSetSchema: &entity.EvaluationSetSchema{
				FieldSchemas: []*entity.FieldSchema{{Name: "input"}},
			},
		}
		set := &entity.EvaluationSet{
			ID:                   setID,
			SpaceID:              sourceID,
			LatestVersion:        "v1",
			EvaluationSetVersion: version,
		}
		mockVersionSvc.EXPECT().
			GetEvaluationSetVersion(gomock.Any(), workspaceID, versionID, nil, gomock.Any()).
			Return(version, set, nil)
		mockAuthorizer.EXPECT().AuthorizeRead(gomock.Any(), gomock.Any()).Return(accessCtx, nil)
		mockUserInfo.EXPECT().PackUserInfo(gomock.Any(), gomock.Any())

		resp, err := app.GetEvaluationSetVersion(context.Background(), &eval_set.GetEvaluationSetVersionRequest{
			WorkspaceID:     workspaceID,
			EvaluationSetID: gptr.Of(setID),
			VersionID:       versionID,
			SharedOption:    sharedOption,
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.NotNil(t, resp.Version)
		assert.Nil(t, resp.Version.EvaluationSetSchema)
		require.NotNil(t, resp.EvaluationSet)
		require.NotNil(t, resp.EvaluationSet.EvaluationSetVersion)
		assert.Nil(t, resp.EvaluationSet.EvaluationSetVersion.EvaluationSetSchema)
		assert.NotNil(t, version.EvaluationSetSchema)
	})

	t.Run("list versions hides schema without mutating domain entity", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockSetSvc := servicemocks.NewMockIEvaluationSetService(ctrl)
		mockVersionSvc := servicemocks.NewMockEvaluationSetVersionService(ctrl)
		mockAuthorizer := servicemocks.NewMockResourceAccessAuthorizer(ctrl)
		mockUserInfo := userinfomocks.NewMockUserInfoService(ctrl)
		app := &EvaluationSetApplicationImpl{
			evaluationSetService:        mockSetSvc,
			evaluationSetVersionService: mockVersionSvc,
			resourceAccessAuthorizer:    mockAuthorizer,
			userInfoService:             mockUserInfo,
		}

		set := &entity.EvaluationSet{ID: setID, SpaceID: sourceID}
		version := &entity.EvaluationSetVersion{
			ID:              versionID,
			EvaluationSetID: setID,
			Version:         "v1",
			EvaluationSetSchema: &entity.EvaluationSetSchema{
				FieldSchemas: []*entity.FieldSchema{{Name: "input"}},
			},
		}
		mockSetSvc.EXPECT().
			GetEvaluationSet(gomock.Any(), gptr.Of(workspaceID), setID, nil, gomock.Any()).
			Return(set, nil)
		mockAuthorizer.EXPECT().AuthorizeRead(gomock.Any(), gomock.Any()).Return(accessCtx, nil)
		mockVersionSvc.EXPECT().
			ListEvaluationSetVersions(gomock.Any(), gomock.Any()).
			Return([]*entity.EvaluationSetVersion{version}, gptr.Of(int64(1)), nil, nil)
		mockUserInfo.EXPECT().PackUserInfo(gomock.Any(), gomock.Any())

		resp, err := app.ListEvaluationSetVersions(context.Background(), &eval_set.ListEvaluationSetVersionsRequest{
			WorkspaceID:     workspaceID,
			EvaluationSetID: setID,
			SharedOption:    sharedOption,
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Len(t, resp.Versions, 1)
		assert.Nil(t, resp.Versions[0].EvaluationSetSchema)
		assert.NotNil(t, version.EvaluationSetSchema)
	})
}
