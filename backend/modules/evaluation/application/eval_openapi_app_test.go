// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	evaluation "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation"
	domainexpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/eval_set"
	exptpb "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	configermocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

type fakeOpenAPIMetric struct {
	called          bool
	spaceID         int64
	evaluationSetID int64
	method          string
	startTime       int64
	err             error
}

func (f *fakeOpenAPIMetric) EmitOpenAPIMetric(_ context.Context, spaceID, evaluationSetID int64, method string, startTime int64, err error) {
	f.called = true
	f.spaceID = spaceID
	f.evaluationSetID = evaluationSetID
	f.method = method
	f.startTime = startTime
	f.err = err
}

func TestEvalOpenAPIApplication_CreateEvaluationSetOApi(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		req     *openapi.CreateEvaluationSetOApiRequest
		setup   func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService)
		wantErr int32
		wantID  int64
	}{
		{
			name: "invalid name",
			req: &openapi.CreateEvaluationSetOApiRequest{
				WorkspaceID: gptr.Of(int64(1001)),
			},
			setup:   func(_ *rpcmocks.MockIAuthProvider, _ *servicemocks.MockIEvaluationSetService) {},
			wantErr: errno.CommonInvalidParamCode,
		},
		{
			name: "auth failed",
			req: &openapi.CreateEvaluationSetOApiRequest{
				WorkspaceID: gptr.Of(int64(2002)),
				Name:        gptr.Of("dataset"),
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, _ *servicemocks.MockIEvaluationSetService) {
				auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr: errno.CommonNoPermissionCode,
		},
		{
			name: "service failed",
			req: &openapi.CreateEvaluationSetOApiRequest{
				WorkspaceID: gptr.Of(int64(3003)),
				Name:        gptr.Of("dataset"),
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService) {
				auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				evalSetSvc.EXPECT().CreateEvaluationSet(gomock.Any(), gomock.Any()).Return(int64(0), errors.New("create failed"))
			},
			wantErr: -1,
		},
		{
			name: "success",
			req: &openapi.CreateEvaluationSetOApiRequest{
				WorkspaceID:         gptr.Of(int64(4004)),
				Name:                gptr.Of("dataset"),
				EvaluationSetSchema: &eval_set.EvaluationSetSchema{},
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService) {
				auth.EXPECT().Authorization(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationParam{})).Return(nil)
				evalSetSvc.EXPECT().CreateEvaluationSet(gomock.Any(), gomock.AssignableToTypeOf(&entity.CreateEvaluationSetParam{})).Return(int64(12345), nil)
			},
			wantID: 12345,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			evalSetSvc := servicemocks.NewMockIEvaluationSetService(ctrl)
			metric := &fakeOpenAPIMetric{}

			app := &EvalOpenAPIApplication{
				auth:                 auth,
				evaluationSetService: evalSetSvc,
				metric:               metric,
			}

			if tc.name == "invalid name" {
				auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Times(0)
				evalSetSvc.EXPECT().CreateEvaluationSet(gomock.Any(), gomock.Any()).Times(0)
			} else {
				tc.setup(auth, evalSetSvc)
			}

			resp, err := app.CreateEvaluationSetOApi(context.Background(), tc.req)

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
				if assert.NotNil(t, resp) && assert.NotNil(t, resp.Data) {
					assert.Equal(t, tc.wantID, resp.Data.GetEvaluationSetID())
				}
			}

			if tc.req != nil {
				assert.True(t, metric.called)
				assert.Equal(t, tc.req.GetWorkspaceID(), metric.spaceID)
				if resp != nil {
					assert.Equal(t, tc.wantID, metric.evaluationSetID)
				}
			}
		})
	}
}

func TestEvalOpenAPIApplication_GetEvaluationSetOApi(t *testing.T) {
	t.Parallel()

	workspaceID := int64(6006)
	evaluationSetID := int64(7007)

	tests := []struct {
		name     string
		buildReq func() *openapi.GetEvaluationSetOApiRequest
		setup    func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService)
		wantErr  int32
	}{
		{
			name: "not found",
			buildReq: func() *openapi.GetEvaluationSetOApiRequest {
				return &openapi.GetEvaluationSetOApiRequest{
					WorkspaceID:     gptr.Of(workspaceID),
					EvaluationSetID: gptr.Of(evaluationSetID),
				}
			},
			setup: func(_ *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService) {
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(nil, nil)
			},
			wantErr: errno.ResourceNotFoundCode,
		},
		{
			name: "service error",
			buildReq: func() *openapi.GetEvaluationSetOApiRequest {
				return &openapi.GetEvaluationSetOApiRequest{
					WorkspaceID:     gptr.Of(workspaceID),
					EvaluationSetID: gptr.Of(evaluationSetID),
				}
			},
			setup: func(_ *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService) {
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(nil, errors.New("svc error"))
			},
			wantErr: -1,
		},
		{
			name: "auth failed",
			buildReq: func() *openapi.GetEvaluationSetOApiRequest {
				return &openapi.GetEvaluationSetOApiRequest{
					WorkspaceID:     gptr.Of(workspaceID),
					EvaluationSetID: gptr.Of(evaluationSetID),
				}
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService) {
				set := &entity.EvaluationSet{ID: evaluationSetID, SpaceID: workspaceID}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr: errno.CommonNoPermissionCode,
		},
		{
			name: "success",
			buildReq: func() *openapi.GetEvaluationSetOApiRequest {
				return &openapi.GetEvaluationSetOApiRequest{
					WorkspaceID:     gptr.Of(workspaceID),
					EvaluationSetID: gptr.Of(evaluationSetID),
				}
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService) {
				ownerID := gptr.Of("owner")
				set := &entity.EvaluationSet{
					ID:      evaluationSetID,
					SpaceID: workspaceID,
					Name:    "name",
					BaseInfo: &entity.BaseInfo{
						CreatedBy: &entity.UserInfo{UserID: ownerID},
					},
				}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).DoAndReturn(func(_ context.Context, param *rpc.AuthorizationWithoutSPIParam) error {
					assert.Equal(t, strconv.FormatInt(evaluationSetID, 10), param.ObjectID)
					assert.Equal(t, workspaceID, param.SpaceID)
					return nil
				})
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			evalSetSvc := servicemocks.NewMockIEvaluationSetService(ctrl)
			metric := &fakeOpenAPIMetric{}

			app := &EvalOpenAPIApplication{
				auth:                 auth,
				evaluationSetService: evalSetSvc,
				metric:               metric,
			}

			req := tc.buildReq()

			tc.setup(auth, evalSetSvc)

			resp, err := app.GetEvaluationSetOApi(context.Background(), req)

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
				if assert.NotNil(t, resp) && assert.NotNil(t, resp.Data) {
					assert.NotNil(t, resp.Data.EvaluationSet)
					assert.Equal(t, evaluationSetID, resp.Data.EvaluationSet.GetID())
				}
			}

			assert.True(t, metric.called)
			assert.Equal(t, workspaceID, metric.spaceID)
		})
	}
}

func TestEvalOpenAPIApplication_UpdateEvaluationSetOApi(t *testing.T) {
	t.Parallel()

	workspaceID := int64(9101)
	evaluationSetID := int64(9102)

	tests := []struct {
		name    string
		req     *openapi.UpdateEvaluationSetOApiRequest
		setup   func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService)
		wantErr int32
	}{
		{
			name: "nil request",
			req:  nil,
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService) {
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Times(0)
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				evalSetSvc.EXPECT().UpdateEvaluationSet(gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr: errno.CommonInvalidParamCode,
		},
		{
			name: "set not found",
			req: &openapi.UpdateEvaluationSetOApiRequest{
				WorkspaceID:     gptr.Of(workspaceID),
				EvaluationSetID: gptr.Of(evaluationSetID),
			},
			setup: func(_ *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService) {
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(nil, nil)
			},
			wantErr: errno.ResourceNotFoundCode,
		},
		{
			name: "auth failed",
			req: &openapi.UpdateEvaluationSetOApiRequest{
				WorkspaceID:     gptr.Of(workspaceID),
				EvaluationSetID: gptr.Of(evaluationSetID),
				Name:            gptr.Of("new name"),
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService) {
				set := &entity.EvaluationSet{ID: evaluationSetID, SpaceID: workspaceID}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr: errno.CommonNoPermissionCode,
		},
		{
			name: "update failed",
			req: &openapi.UpdateEvaluationSetOApiRequest{
				WorkspaceID:     gptr.Of(workspaceID),
				EvaluationSetID: gptr.Of(evaluationSetID),
				Name:            gptr.Of("new name"),
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService) {
				ownerID := gptr.Of("owner")
				set := &entity.EvaluationSet{ID: evaluationSetID, SpaceID: workspaceID, BaseInfo: &entity.BaseInfo{CreatedBy: &entity.UserInfo{UserID: ownerID}}}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(nil)
				evalSetSvc.EXPECT().UpdateEvaluationSet(gomock.Any(), gomock.AssignableToTypeOf(&entity.UpdateEvaluationSetParam{})).Return(errors.New("update error"))
			},
			wantErr: -1,
		},
		{
			name: "success",
			req: &openapi.UpdateEvaluationSetOApiRequest{
				WorkspaceID:     gptr.Of(workspaceID),
				EvaluationSetID: gptr.Of(evaluationSetID),
				Name:            gptr.Of("new name"),
				Description:     gptr.Of("desc"),
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService) {
				ownerID := gptr.Of("owner")
				set := &entity.EvaluationSet{ID: evaluationSetID, SpaceID: workspaceID, BaseInfo: &entity.BaseInfo{CreatedBy: &entity.UserInfo{UserID: ownerID}}}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).DoAndReturn(func(_ context.Context, param *rpc.AuthorizationWithoutSPIParam) error {
					assert.Equal(t, strconv.FormatInt(evaluationSetID, 10), param.ObjectID)
					assert.Equal(t, workspaceID, param.SpaceID)
					return nil
				})
				evalSetSvc.EXPECT().UpdateEvaluationSet(gomock.Any(), gomock.AssignableToTypeOf(&entity.UpdateEvaluationSetParam{})).DoAndReturn(func(_ context.Context, param *entity.UpdateEvaluationSetParam) error {
					assert.Equal(t, workspaceID, param.SpaceID)
					assert.Equal(t, evaluationSetID, param.EvaluationSetID)
					assert.Equal(t, "new name", gptr.Indirect(param.Name))
					assert.Equal(t, "desc", gptr.Indirect(param.Description))
					return nil
				})
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			evalSetSvc := servicemocks.NewMockIEvaluationSetService(ctrl)
			metric := &fakeOpenAPIMetric{}

			app := &EvalOpenAPIApplication{
				auth:                 auth,
				evaluationSetService: evalSetSvc,
				metric:               metric,
			}

			if tc.setup != nil {
				tc.setup(auth, evalSetSvc)
			}

			resp, err := app.UpdateEvaluationSetOApi(context.Background(), tc.req)

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
				assert.NotNil(t, resp)
				assert.NotNil(t, resp.Data)
			}

			if tc.req != nil {
				assert.True(t, metric.called)
				assert.Equal(t, tc.req.GetWorkspaceID(), metric.spaceID)
				assert.Equal(t, tc.req.GetEvaluationSetID(), metric.evaluationSetID)
			}
		})
	}
}

func TestEvalOpenAPIApplication_DeleteEvaluationSetOApi(t *testing.T) {
	t.Parallel()

	workspaceID := int64(9201)
	evaluationSetID := int64(9202)

	tests := []struct {
		name    string
		req     *openapi.DeleteEvaluationSetOApiRequest
		setup   func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService)
		wantErr int32
	}{
		{
			name: "nil request",
			req:  nil,
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService) {
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Times(0)
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				evalSetSvc.EXPECT().DeleteEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr: errno.CommonInvalidParamCode,
		},
		{
			name: "set not found",
			req: &openapi.DeleteEvaluationSetOApiRequest{
				WorkspaceID:     gptr.Of(workspaceID),
				EvaluationSetID: gptr.Of(evaluationSetID),
			},
			setup: func(_ *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService) {
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(nil, nil)
			},
			wantErr: errno.ResourceNotFoundCode,
		},
		{
			name: "auth failed",
			req: &openapi.DeleteEvaluationSetOApiRequest{
				WorkspaceID:     gptr.Of(workspaceID),
				EvaluationSetID: gptr.Of(evaluationSetID),
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService) {
				set := &entity.EvaluationSet{ID: evaluationSetID, SpaceID: workspaceID}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr: errno.CommonNoPermissionCode,
		},
		{
			name: "delete failed",
			req: &openapi.DeleteEvaluationSetOApiRequest{
				WorkspaceID:     gptr.Of(workspaceID),
				EvaluationSetID: gptr.Of(evaluationSetID),
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService) {
				ownerID := gptr.Of("owner")
				set := &entity.EvaluationSet{ID: evaluationSetID, SpaceID: workspaceID, BaseInfo: &entity.BaseInfo{CreatedBy: &entity.UserInfo{UserID: ownerID}}}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(nil)
				evalSetSvc.EXPECT().DeleteEvaluationSet(gomock.Any(), workspaceID, evaluationSetID).Return(errors.New("delete error"))
			},
			wantErr: -1,
		},
		{
			name: "success",
			req: &openapi.DeleteEvaluationSetOApiRequest{
				WorkspaceID:     gptr.Of(workspaceID),
				EvaluationSetID: gptr.Of(evaluationSetID),
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService) {
				ownerID := gptr.Of("owner")
				set := &entity.EvaluationSet{ID: evaluationSetID, SpaceID: workspaceID, BaseInfo: &entity.BaseInfo{CreatedBy: &entity.UserInfo{UserID: ownerID}}}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(nil)
				evalSetSvc.EXPECT().DeleteEvaluationSet(gomock.Any(), workspaceID, evaluationSetID).Return(nil)
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			evalSetSvc := servicemocks.NewMockIEvaluationSetService(ctrl)
			metric := &fakeOpenAPIMetric{}

			app := &EvalOpenAPIApplication{
				auth:                 auth,
				evaluationSetService: evalSetSvc,
				metric:               metric,
			}

			if tc.setup != nil {
				tc.setup(auth, evalSetSvc)
			}

			resp, err := app.DeleteEvaluationSetOApi(context.Background(), tc.req)

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
				assert.NotNil(t, resp)
				assert.NotNil(t, resp.Data)
			}

			if tc.req != nil {
				assert.True(t, metric.called)
				assert.Equal(t, tc.req.GetWorkspaceID(), metric.spaceID)
				assert.Equal(t, tc.req.GetEvaluationSetID(), metric.evaluationSetID)
			}
		})
	}
}

func TestEvalOpenAPIApplication_ListEvaluationSetsOApi(t *testing.T) {
	t.Parallel()

	workspaceID := int64(8080)

	tests := []struct {
		name     string
		buildReq func() *openapi.ListEvaluationSetsOApiRequest
		setup    func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService)
		wantErr  int32
		wantLen  int
	}{
		{
			name: "auth failed",
			buildReq: func() *openapi.ListEvaluationSetsOApiRequest {
				pageSize := int32(10)
				return &openapi.ListEvaluationSetsOApiRequest{WorkspaceID: gptr.Of(workspaceID), PageSize: &pageSize}
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, _ *servicemocks.MockIEvaluationSetService) {
				auth.EXPECT().Authorization(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationParam{})).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr: errno.CommonNoPermissionCode,
		},
		{
			name: "service error",
			buildReq: func() *openapi.ListEvaluationSetsOApiRequest {
				return &openapi.ListEvaluationSetsOApiRequest{WorkspaceID: gptr.Of(workspaceID)}
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService) {
				auth.EXPECT().Authorization(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationParam{})).Return(nil)
				evalSetSvc.EXPECT().ListEvaluationSets(gomock.Any(), gomock.AssignableToTypeOf(&entity.ListEvaluationSetsParam{})).Return(nil, nil, nil, errors.New("list error"))
			},
			wantErr: -1,
		},
		{
			name: "success",
			buildReq: func() *openapi.ListEvaluationSetsOApiRequest {
				pageSize := int32(5)
				return &openapi.ListEvaluationSetsOApiRequest{WorkspaceID: gptr.Of(workspaceID), PageSize: &pageSize}
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService) {
				auth.EXPECT().Authorization(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationParam{})).Return(nil)
				total := gptr.Of(int64(2))
				next := gptr.Of("next")
				sets := []*entity.EvaluationSet{{ID: 1, Name: "a"}, {ID: 2, Name: "b"}}
				evalSetSvc.EXPECT().ListEvaluationSets(gomock.Any(), gomock.AssignableToTypeOf(&entity.ListEvaluationSetsParam{})).Return(sets, total, next, nil)
			},
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			evalSetSvc := servicemocks.NewMockIEvaluationSetService(ctrl)
			metric := &fakeOpenAPIMetric{}

			app := &EvalOpenAPIApplication{
				auth:                 auth,
				evaluationSetService: evalSetSvc,
				metric:               metric,
			}

			req := tc.buildReq()
			tc.setup(auth, evalSetSvc)

			resp, err := app.ListEvaluationSetsOApi(context.Background(), req)

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
				if assert.NotNil(t, resp) && assert.NotNil(t, resp.Data) {
					assert.Len(t, resp.Data.Sets, tc.wantLen)
				}
			}

			assert.True(t, metric.called)
			assert.Equal(t, workspaceID, metric.spaceID)
		})
	}
}

func TestEvalOpenAPIApplication_CreateEvaluationSetVersionOApi(t *testing.T) {
	t.Parallel()

	workspaceID := int64(9009)
	evaluationSetID := int64(10010)

	tests := []struct {
		name     string
		buildReq func() *openapi.CreateEvaluationSetVersionOApiRequest
		setup    func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, versionSvc *servicemocks.MockEvaluationSetVersionService)
		wantErr  int32
		wantID   int64
	}{
		{
			name: "missing version",
			buildReq: func() *openapi.CreateEvaluationSetVersionOApiRequest {
				return &openapi.CreateEvaluationSetVersionOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID)}
			},
			setup: func(_ *rpcmocks.MockIAuthProvider, _ *servicemocks.MockIEvaluationSetService, _ *servicemocks.MockEvaluationSetVersionService) {
			},
			wantErr: errno.CommonInvalidParamCode,
		},
		{
			name: "set not found",
			buildReq: func() *openapi.CreateEvaluationSetVersionOApiRequest {
				version := "v1"
				return &openapi.CreateEvaluationSetVersionOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID), Version: &version}
			},
			setup: func(_ *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, _ *servicemocks.MockEvaluationSetVersionService) {
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(nil, nil)
			},
			wantErr: errno.ResourceNotFoundCode,
		},
		{
			name: "auth failed",
			buildReq: func() *openapi.CreateEvaluationSetVersionOApiRequest {
				version := "v1"
				return &openapi.CreateEvaluationSetVersionOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID), Version: &version}
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, _ *servicemocks.MockEvaluationSetVersionService) {
				set := &entity.EvaluationSet{ID: evaluationSetID, SpaceID: workspaceID}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr: errno.CommonNoPermissionCode,
		},
		{
			name: "create failed",
			buildReq: func() *openapi.CreateEvaluationSetVersionOApiRequest {
				version := "v1"
				return &openapi.CreateEvaluationSetVersionOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID), Version: &version}
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, versionSvc *servicemocks.MockEvaluationSetVersionService) {
				set := &entity.EvaluationSet{ID: evaluationSetID, SpaceID: workspaceID}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(nil)
				versionSvc.EXPECT().CreateEvaluationSetVersion(gomock.Any(), gomock.AssignableToTypeOf(&entity.CreateEvaluationSetVersionParam{})).Return(int64(0), errors.New("create error"))
			},
			wantErr: -1,
		},
		{
			name: "success",
			buildReq: func() *openapi.CreateEvaluationSetVersionOApiRequest {
				version := "v2"
				description := "desc"
				return &openapi.CreateEvaluationSetVersionOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID), Version: &version, Description: &description}
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, versionSvc *servicemocks.MockEvaluationSetVersionService) {
				owner := gptr.Of("owner")
				set := &entity.EvaluationSet{ID: evaluationSetID, SpaceID: workspaceID, BaseInfo: &entity.BaseInfo{CreatedBy: &entity.UserInfo{UserID: owner}}}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(nil)
				versionSvc.EXPECT().CreateEvaluationSetVersion(gomock.Any(), gomock.AssignableToTypeOf(&entity.CreateEvaluationSetVersionParam{})).Return(int64(321), nil)
			},
			wantID: 321,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			evalSetSvc := servicemocks.NewMockIEvaluationSetService(ctrl)
			versionSvc := servicemocks.NewMockEvaluationSetVersionService(ctrl)
			metric := &fakeOpenAPIMetric{}

			app := &EvalOpenAPIApplication{
				auth:                        auth,
				evaluationSetService:        evalSetSvc,
				evaluationSetVersionService: versionSvc,
				metric:                      metric,
			}

			req := tc.buildReq()
			if req.GetVersion() == "" {
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Times(0)
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				versionSvc.EXPECT().CreateEvaluationSetVersion(gomock.Any(), gomock.Any()).Times(0)
			} else {
				tc.setup(auth, evalSetSvc, versionSvc)
			}

			resp, err := app.CreateEvaluationSetVersionOApi(context.Background(), req)

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
				if assert.NotNil(t, resp) && assert.NotNil(t, resp.Data) {
					assert.Equal(t, tc.wantID, resp.Data.GetVersionID())
				}
			}

			assert.True(t, metric.called)
			assert.Equal(t, workspaceID, metric.spaceID)
		})
	}
}

func TestEvalOpenAPIApplication_ListEvaluationSetVersionsOApi(t *testing.T) {
	t.Parallel()

	workspaceID := int64(1111)
	evaluationSetID := int64(2222)

	tests := []struct {
		name     string
		buildReq func() *openapi.ListEvaluationSetVersionsOApiRequest
		setup    func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, versionSvc *servicemocks.MockEvaluationSetVersionService)
		wantErr  int32
		wantLen  int
	}{
		{
			name: "nil request",
			buildReq: func() *openapi.ListEvaluationSetVersionsOApiRequest {
				return nil
			},
			setup: func(_ *rpcmocks.MockIAuthProvider, _ *servicemocks.MockIEvaluationSetService, _ *servicemocks.MockEvaluationSetVersionService) {
			},
			wantErr: errno.CommonInvalidParamCode,
		},
		{
			name: "set not found",
			buildReq: func() *openapi.ListEvaluationSetVersionsOApiRequest {
				return &openapi.ListEvaluationSetVersionsOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID)}
			},
			setup: func(_ *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, _ *servicemocks.MockEvaluationSetVersionService) {
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(nil, nil)
			},
			wantErr: errno.ResourceNotFoundCode,
		},
		{
			name: "auth failed",
			buildReq: func() *openapi.ListEvaluationSetVersionsOApiRequest {
				return &openapi.ListEvaluationSetVersionsOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID)}
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, _ *servicemocks.MockEvaluationSetVersionService) {
				set := &entity.EvaluationSet{ID: evaluationSetID, SpaceID: workspaceID}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr: errno.CommonNoPermissionCode,
		},
		{
			name: "service error",
			buildReq: func() *openapi.ListEvaluationSetVersionsOApiRequest {
				return &openapi.ListEvaluationSetVersionsOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID)}
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, versionSvc *servicemocks.MockEvaluationSetVersionService) {
				set := &entity.EvaluationSet{ID: evaluationSetID, SpaceID: workspaceID}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(nil)
				versionSvc.EXPECT().ListEvaluationSetVersions(gomock.Any(), gomock.AssignableToTypeOf(&entity.ListEvaluationSetVersionsParam{})).Return(nil, nil, nil, errors.New("list error"))
			},
			wantErr: -1,
		},
		{
			name: "success",
			buildReq: func() *openapi.ListEvaluationSetVersionsOApiRequest {
				pageSize := int32(3)
				return &openapi.ListEvaluationSetVersionsOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID), PageSize: &pageSize}
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, versionSvc *servicemocks.MockEvaluationSetVersionService) {
				owner := gptr.Of("owner")
				set := &entity.EvaluationSet{ID: evaluationSetID, SpaceID: workspaceID, BaseInfo: &entity.BaseInfo{CreatedBy: &entity.UserInfo{UserID: owner}}}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(nil)
				versions := []*entity.EvaluationSetVersion{{ID: 1, Version: "v1"}, {ID: 2, Version: "v2"}}
				total := gptr.Of(int64(2))
				next := gptr.Of("token")
				versionSvc.EXPECT().ListEvaluationSetVersions(gomock.Any(), gomock.AssignableToTypeOf(&entity.ListEvaluationSetVersionsParam{})).Return(versions, total, next, nil)
			},
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			evalSetSvc := servicemocks.NewMockIEvaluationSetService(ctrl)
			versionSvc := servicemocks.NewMockEvaluationSetVersionService(ctrl)

			app := &EvalOpenAPIApplication{
				auth:                        auth,
				evaluationSetService:        evalSetSvc,
				evaluationSetVersionService: versionSvc,
			}

			req := tc.buildReq()
			if req == nil {
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Times(0)
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				versionSvc.EXPECT().ListEvaluationSetVersions(gomock.Any(), gomock.Any()).Times(0)
			} else {
				tc.setup(auth, evalSetSvc, versionSvc)
			}

			resp, err := app.ListEvaluationSetVersionsOApi(context.Background(), req)

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
				if assert.NotNil(t, resp) && assert.NotNil(t, resp.Data) {
					assert.Len(t, resp.Data.Versions, tc.wantLen)
				}
			}
		})
	}
}

func TestEvalOpenAPIApplication_BatchCreateEvaluationSetItemsOApi(t *testing.T) {
	t.Parallel()

	workspaceID := int64(3333)
	evaluationSetID := int64(4444)

	tests := []struct {
		name     string
		buildReq func() *openapi.BatchCreateEvaluationSetItemsOApiRequest
		setup    func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, itemSvc *servicemocks.MockEvaluationSetItemService)
		wantErr  int32
		wantLen  int
	}{
		{
			name: "empty items",
			buildReq: func() *openapi.BatchCreateEvaluationSetItemsOApiRequest {
				return &openapi.BatchCreateEvaluationSetItemsOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID)}
			},
			setup: func(_ *rpcmocks.MockIAuthProvider, _ *servicemocks.MockIEvaluationSetService, _ *servicemocks.MockEvaluationSetItemService) {
			},
			wantErr: errno.CommonInvalidParamCode,
		},
		{
			name: "set not found",
			buildReq: func() *openapi.BatchCreateEvaluationSetItemsOApiRequest {
				items := []*eval_set.EvaluationSetItem{{}}
				return &openapi.BatchCreateEvaluationSetItemsOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID), Items: items}
			},
			setup: func(_ *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, _ *servicemocks.MockEvaluationSetItemService) {
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(nil, nil)
			},
			wantErr: errno.ResourceNotFoundCode,
		},
		{
			name: "auth failed",
			buildReq: func() *openapi.BatchCreateEvaluationSetItemsOApiRequest {
				items := []*eval_set.EvaluationSetItem{{}}
				return &openapi.BatchCreateEvaluationSetItemsOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID), Items: items}
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, _ *servicemocks.MockEvaluationSetItemService) {
				set := &entity.EvaluationSet{ID: evaluationSetID, SpaceID: workspaceID}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr: errno.CommonNoPermissionCode,
		},
		{
			name: "service error",
			buildReq: func() *openapi.BatchCreateEvaluationSetItemsOApiRequest {
				items := []*eval_set.EvaluationSetItem{{}}
				skip := true
				return &openapi.BatchCreateEvaluationSetItemsOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID), Items: items, IsSkipInvalidItems: &skip}
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, itemSvc *servicemocks.MockEvaluationSetItemService) {
				set := &entity.EvaluationSet{ID: evaluationSetID, SpaceID: workspaceID}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(nil)
				itemSvc.EXPECT().BatchCreateEvaluationSetItems(gomock.Any(), gomock.AssignableToTypeOf(&entity.BatchCreateEvaluationSetItemsParam{})).Return(nil, nil, nil, errors.New("create error"))
			},
			wantErr: -1,
		},
		{
			name: "success",
			buildReq: func() *openapi.BatchCreateEvaluationSetItemsOApiRequest {
				items := []*eval_set.EvaluationSetItem{{}}
				allowPartial := true
				return &openapi.BatchCreateEvaluationSetItemsOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID), Items: items, IsAllowPartialAdd: &allowPartial}
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, itemSvc *servicemocks.MockEvaluationSetItemService) {
				owner := gptr.Of("owner")
				set := &entity.EvaluationSet{ID: evaluationSetID, SpaceID: workspaceID, BaseInfo: &entity.BaseInfo{CreatedBy: &entity.UserInfo{UserID: owner}}}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(nil)
				errType := entity.ItemErrorType_MismatchSchema
				summary := gptr.Of("summary")
				errCount := gptr.Of(int32(1))
				detailMsg := gptr.Of("detail")
				detailIdx := gptr.Of(int32(0))
				errorsList := []*entity.ItemErrorGroup{{Type: &errType, Summary: summary, ErrorCount: errCount, Details: []*entity.ItemErrorDetail{{Message: detailMsg, Index: detailIdx}}}}
				itemKey := gptr.Of("key")
				itemID := gptr.Of(int64(10))
				isNew := gptr.Of(true)
				idx := gptr.Of(int32(0))
				outputs := []*entity.DatasetItemOutput{{ItemKey: itemKey, ItemID: itemID, IsNewItem: isNew, ItemIndex: idx}}
				itemSvc.EXPECT().BatchCreateEvaluationSetItems(gomock.Any(), gomock.AssignableToTypeOf(&entity.BatchCreateEvaluationSetItemsParam{})).Return(nil, errorsList, outputs, nil)
			},
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			evalSetSvc := servicemocks.NewMockIEvaluationSetService(ctrl)
			itemSvc := servicemocks.NewMockEvaluationSetItemService(ctrl)
			metric := &fakeOpenAPIMetric{}

			app := &EvalOpenAPIApplication{
				auth:                     auth,
				evaluationSetService:     evalSetSvc,
				evaluationSetItemService: itemSvc,
				metric:                   metric,
			}

			req := tc.buildReq()
			if len(req.Items) == 0 {
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Times(0)
				itemSvc.EXPECT().BatchCreateEvaluationSetItems(gomock.Any(), gomock.Any()).Times(0)
			} else {
				tc.setup(auth, evalSetSvc, itemSvc)
			}

			resp, err := app.BatchCreateEvaluationSetItemsOApi(context.Background(), req)

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
				if assert.NotNil(t, resp) && assert.NotNil(t, resp.Data) {
					assert.Len(t, resp.Data.ItemOutputs, tc.wantLen)
				}
			}

			assert.True(t, metric.called)
			assert.Equal(t, workspaceID, metric.spaceID)
			if resp != nil && resp.Data != nil {
				assert.NotNil(t, resp.Data.Errors)
			}
		})
	}
}

func TestEvalOpenAPIApplication_BatchUpdateEvaluationSetItemsOApi(t *testing.T) {
	t.Parallel()

	workspaceID := int64(5555)
	evaluationSetID := int64(6666)

	tests := []struct {
		name     string
		buildReq func() *openapi.BatchUpdateEvaluationSetItemsOApiRequest
		setup    func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, itemSvc *servicemocks.MockEvaluationSetItemService)
		wantErr  int32
	}{
		{
			name: "empty items",
			buildReq: func() *openapi.BatchUpdateEvaluationSetItemsOApiRequest {
				return &openapi.BatchUpdateEvaluationSetItemsOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID)}
			},
			setup: func(_ *rpcmocks.MockIAuthProvider, _ *servicemocks.MockIEvaluationSetService, _ *servicemocks.MockEvaluationSetItemService) {
			},
			wantErr: errno.CommonInvalidParamCode,
		},
		{
			name: "set not found",
			buildReq: func() *openapi.BatchUpdateEvaluationSetItemsOApiRequest {
				items := []*eval_set.EvaluationSetItem{{}}
				return &openapi.BatchUpdateEvaluationSetItemsOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID), Items: items}
			},
			setup: func(_ *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, _ *servicemocks.MockEvaluationSetItemService) {
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(nil, nil)
			},
			wantErr: errno.ResourceNotFoundCode,
		},
		{
			name: "auth failed",
			buildReq: func() *openapi.BatchUpdateEvaluationSetItemsOApiRequest {
				items := []*eval_set.EvaluationSetItem{{}}
				return &openapi.BatchUpdateEvaluationSetItemsOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID), Items: items}
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, _ *servicemocks.MockEvaluationSetItemService) {
				set := &entity.EvaluationSet{ID: evaluationSetID, SpaceID: workspaceID}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr: errno.CommonNoPermissionCode,
		},
		{
			name: "service error",
			buildReq: func() *openapi.BatchUpdateEvaluationSetItemsOApiRequest {
				items := []*eval_set.EvaluationSetItem{{}}
				return &openapi.BatchUpdateEvaluationSetItemsOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID), Items: items}
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, itemSvc *servicemocks.MockEvaluationSetItemService) {
				set := &entity.EvaluationSet{ID: evaluationSetID, SpaceID: workspaceID}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(nil)
				itemSvc.EXPECT().BatchUpdateEvaluationSetItems(gomock.Any(), gomock.AssignableToTypeOf(&entity.BatchUpdateEvaluationSetItemsParam{})).Return(nil, nil, errors.New("update error"))
			},
			wantErr: -1,
		},
		{
			name: "success",
			buildReq: func() *openapi.BatchUpdateEvaluationSetItemsOApiRequest {
				items := []*eval_set.EvaluationSetItem{{}}
				return &openapi.BatchUpdateEvaluationSetItemsOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID), Items: items}
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, itemSvc *servicemocks.MockEvaluationSetItemService) {
				owner := gptr.Of("owner")
				set := &entity.EvaluationSet{ID: evaluationSetID, SpaceID: workspaceID, BaseInfo: &entity.BaseInfo{CreatedBy: &entity.UserInfo{UserID: owner}}}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(nil)
				itemSvc.EXPECT().BatchUpdateEvaluationSetItems(gomock.Any(), gomock.AssignableToTypeOf(&entity.BatchUpdateEvaluationSetItemsParam{})).Return(nil, nil, nil)
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			evalSetSvc := servicemocks.NewMockIEvaluationSetService(ctrl)
			itemSvc := servicemocks.NewMockEvaluationSetItemService(ctrl)
			metric := &fakeOpenAPIMetric{}

			app := &EvalOpenAPIApplication{
				auth:                     auth,
				evaluationSetService:     evalSetSvc,
				evaluationSetItemService: itemSvc,
				metric:                   metric,
			}

			req := tc.buildReq()
			if len(req.Items) == 0 {
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Times(0)
				itemSvc.EXPECT().BatchUpdateEvaluationSetItems(gomock.Any(), gomock.Any()).Times(0)
			} else {
				tc.setup(auth, evalSetSvc, itemSvc)
			}

			resp, err := app.BatchUpdateEvaluationSetItemsOApi(context.Background(), req)

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
				if assert.NotNil(t, resp) {
					assert.NotNil(t, resp.Data)
				}
			}

			assert.True(t, metric.called)
			assert.Equal(t, workspaceID, metric.spaceID)
		})
	}
}

func TestEvalOpenAPIApplication_BatchDeleteEvaluationSetItemsOApi(t *testing.T) {
	t.Parallel()

	workspaceID := int64(7070)
	evaluationSetID := int64(8080)

	tests := []struct {
		name     string
		buildReq func() *openapi.BatchDeleteEvaluationSetItemsOApiRequest
		setup    func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, itemSvc *servicemocks.MockEvaluationSetItemService)
		wantErr  int32
	}{
		{
			name: "missing item ids",
			buildReq: func() *openapi.BatchDeleteEvaluationSetItemsOApiRequest {
				return &openapi.BatchDeleteEvaluationSetItemsOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID)}
			},
			setup: func(_ *rpcmocks.MockIAuthProvider, _ *servicemocks.MockIEvaluationSetService, _ *servicemocks.MockEvaluationSetItemService) {
			},
			wantErr: errno.CommonInvalidParamCode,
		},
		{
			name: "set not found",
			buildReq: func() *openapi.BatchDeleteEvaluationSetItemsOApiRequest {
				items := []int64{1, 2}
				return &openapi.BatchDeleteEvaluationSetItemsOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID), ItemIds: items}
			},
			setup: func(_ *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, _ *servicemocks.MockEvaluationSetItemService) {
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(nil, nil)
			},
			wantErr: errno.ResourceNotFoundCode,
		},
		{
			name: "auth failed",
			buildReq: func() *openapi.BatchDeleteEvaluationSetItemsOApiRequest {
				items := []int64{1}
				return &openapi.BatchDeleteEvaluationSetItemsOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID), ItemIds: items}
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, _ *servicemocks.MockEvaluationSetItemService) {
				set := &entity.EvaluationSet{ID: evaluationSetID, SpaceID: workspaceID}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr: errno.CommonNoPermissionCode,
		},
		{
			name: "clear all success",
			buildReq: func() *openapi.BatchDeleteEvaluationSetItemsOApiRequest {
				deleteAll := true
				return &openapi.BatchDeleteEvaluationSetItemsOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID), IsDeleteAll: &deleteAll}
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, itemSvc *servicemocks.MockEvaluationSetItemService) {
				owner := gptr.Of("owner")
				set := &entity.EvaluationSet{ID: evaluationSetID, SpaceID: workspaceID, BaseInfo: &entity.BaseInfo{CreatedBy: &entity.UserInfo{UserID: owner}}}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(nil)
				itemSvc.EXPECT().ClearEvaluationSetDraftItem(gomock.Any(), workspaceID, evaluationSetID).Return(nil)
			},
		},
		{
			name: "batch delete error",
			buildReq: func() *openapi.BatchDeleteEvaluationSetItemsOApiRequest {
				items := []int64{9}
				return &openapi.BatchDeleteEvaluationSetItemsOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID), ItemIds: items}
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, itemSvc *servicemocks.MockEvaluationSetItemService) {
				set := &entity.EvaluationSet{ID: evaluationSetID, SpaceID: workspaceID}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(nil)
				itemSvc.EXPECT().BatchDeleteEvaluationSetItems(gomock.Any(), workspaceID, evaluationSetID, []int64{9}).Return(errors.New("delete error"))
			},
			wantErr: -1,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			evalSetSvc := servicemocks.NewMockIEvaluationSetService(ctrl)
			itemSvc := servicemocks.NewMockEvaluationSetItemService(ctrl)
			metric := &fakeOpenAPIMetric{}

			app := &EvalOpenAPIApplication{
				auth:                     auth,
				evaluationSetService:     evalSetSvc,
				evaluationSetItemService: itemSvc,
				metric:                   metric,
			}

			req := tc.buildReq()
			if !req.GetIsDeleteAll() && len(req.GetItemIds()) == 0 {
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Times(0)
				itemSvc.EXPECT().ClearEvaluationSetDraftItem(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				itemSvc.EXPECT().BatchDeleteEvaluationSetItems(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			} else {
				tc.setup(auth, evalSetSvc, itemSvc)
			}

			resp, err := app.BatchDeleteEvaluationSetItemsOApi(context.Background(), req)

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
				assert.NotNil(t, resp)
			}

			assert.True(t, metric.called)
			assert.Equal(t, workspaceID, metric.spaceID)
		})
	}
}

func TestEvalOpenAPIApplication_ListEvaluationSetVersionItemsOApi(t *testing.T) {
	t.Parallel()

	workspaceID := int64(9090)
	evaluationSetID := int64(100100)

	tests := []struct {
		name     string
		buildReq func() *openapi.ListEvaluationSetVersionItemsOApiRequest
		setup    func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, itemSvc *servicemocks.MockEvaluationSetItemService)
		wantErr  int32
		wantLen  int
	}{
		{
			name: "set not found",
			buildReq: func() *openapi.ListEvaluationSetVersionItemsOApiRequest {
				return &openapi.ListEvaluationSetVersionItemsOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID)}
			},
			setup: func(_ *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, _ *servicemocks.MockEvaluationSetItemService) {
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Any()).Return(nil, nil)
			},
			wantErr: errno.ResourceNotFoundCode,
		},
		{
			name: "auth failed",
			buildReq: func() *openapi.ListEvaluationSetVersionItemsOApiRequest {
				pageSize := int32(10)
				return &openapi.ListEvaluationSetVersionItemsOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID), PageSize: &pageSize}
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, _ *servicemocks.MockEvaluationSetItemService) {
				set := &entity.EvaluationSet{ID: evaluationSetID, SpaceID: workspaceID}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Any()).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr: errno.CommonNoPermissionCode,
		},
		{
			name: "service error",
			buildReq: func() *openapi.ListEvaluationSetVersionItemsOApiRequest {
				return &openapi.ListEvaluationSetVersionItemsOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID)}
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, itemSvc *servicemocks.MockEvaluationSetItemService) {
				set := &entity.EvaluationSet{ID: evaluationSetID, SpaceID: workspaceID}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Any()).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(nil)
				itemSvc.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.AssignableToTypeOf(&entity.ListEvaluationSetItemsParam{})).Return(nil, nil, nil, errors.New("list error"))
			},
			wantErr: -1,
		},
		{
			name: "success",
			buildReq: func() *openapi.ListEvaluationSetVersionItemsOApiRequest {
				pageSize := int32(2)
				return &openapi.ListEvaluationSetVersionItemsOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID), PageSize: &pageSize}
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, itemSvc *servicemocks.MockEvaluationSetItemService) {
				owner := gptr.Of("owner")
				set := &entity.EvaluationSet{ID: evaluationSetID, SpaceID: workspaceID, BaseInfo: &entity.BaseInfo{CreatedBy: &entity.UserInfo{UserID: owner}}}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Any()).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(nil)
				items := []*entity.EvaluationSetItem{{ID: 1}, {ID: 2}}
				total := gptr.Of(int64(2))
				next := gptr.Of("cursor")
				itemSvc.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.AssignableToTypeOf(&entity.ListEvaluationSetItemsParam{})).Return(items, total, next, nil)
			},
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			evalSetSvc := servicemocks.NewMockIEvaluationSetService(ctrl)
			itemSvc := servicemocks.NewMockEvaluationSetItemService(ctrl)
			metric := &fakeOpenAPIMetric{}

			app := &EvalOpenAPIApplication{
				auth:                     auth,
				evaluationSetService:     evalSetSvc,
				evaluationSetItemService: itemSvc,
				metric:                   metric,
			}

			req := tc.buildReq()
			tc.setup(auth, evalSetSvc, itemSvc)

			resp, err := app.ListEvaluationSetVersionItemsOApi(context.Background(), req)

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
				if assert.NotNil(t, resp) && assert.NotNil(t, resp.Data) {
					assert.Len(t, resp.Data.Items, tc.wantLen)
				}
			}

			assert.True(t, metric.called)
			assert.Equal(t, workspaceID, metric.spaceID)
		})
	}
}

func TestEvalOpenAPIApplication_UpdateEvaluationSetSchemaOApi(t *testing.T) {
	t.Parallel()

	workspaceID := int64(120120)
	evaluationSetID := int64(130130)

	tests := []struct {
		name     string
		buildReq func() *openapi.UpdateEvaluationSetSchemaOApiRequest
		setup    func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, schemaSvc *servicemocks.MockEvaluationSetSchemaService)
		wantErr  int32
	}{
		{
			name: "set not found",
			buildReq: func() *openapi.UpdateEvaluationSetSchemaOApiRequest {
				fields := []*eval_set.FieldSchema{{}}
				return &openapi.UpdateEvaluationSetSchemaOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID), Fields: fields}
			},
			setup: func(_ *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, _ *servicemocks.MockEvaluationSetSchemaService) {
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(nil, nil)
			},
			wantErr: errno.ResourceNotFoundCode,
		},
		{
			name: "auth failed",
			buildReq: func() *openapi.UpdateEvaluationSetSchemaOApiRequest {
				fields := []*eval_set.FieldSchema{{}}
				return &openapi.UpdateEvaluationSetSchemaOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID), Fields: fields}
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, _ *servicemocks.MockEvaluationSetSchemaService) {
				set := &entity.EvaluationSet{ID: evaluationSetID, SpaceID: workspaceID}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr: errno.CommonNoPermissionCode,
		},
		{
			name: "update error",
			buildReq: func() *openapi.UpdateEvaluationSetSchemaOApiRequest {
				fields := []*eval_set.FieldSchema{{}}
				return &openapi.UpdateEvaluationSetSchemaOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID), Fields: fields}
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, schemaSvc *servicemocks.MockEvaluationSetSchemaService) {
				set := &entity.EvaluationSet{ID: evaluationSetID, SpaceID: workspaceID}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(nil)
				schemaSvc.EXPECT().UpdateEvaluationSetSchema(gomock.Any(), workspaceID, evaluationSetID, gomock.Any()).Return(errors.New("update error"))
			},
			wantErr: -1,
		},
		{
			name: "success",
			buildReq: func() *openapi.UpdateEvaluationSetSchemaOApiRequest {
				fields := []*eval_set.FieldSchema{{}}
				return &openapi.UpdateEvaluationSetSchemaOApiRequest{WorkspaceID: gptr.Of(workspaceID), EvaluationSetID: gptr.Of(evaluationSetID), Fields: fields}
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, schemaSvc *servicemocks.MockEvaluationSetSchemaService) {
				owner := gptr.Of("owner")
				set := &entity.EvaluationSet{ID: evaluationSetID, SpaceID: workspaceID, BaseInfo: &entity.BaseInfo{CreatedBy: &entity.UserInfo{UserID: owner}}}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evaluationSetID, gomock.Nil()).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(nil)
				schemaSvc.EXPECT().UpdateEvaluationSetSchema(gomock.Any(), workspaceID, evaluationSetID, gomock.Any()).Return(nil)
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			evalSetSvc := servicemocks.NewMockIEvaluationSetService(ctrl)
			schemaSvc := servicemocks.NewMockEvaluationSetSchemaService(ctrl)
			metric := &fakeOpenAPIMetric{}

			app := &EvalOpenAPIApplication{
				auth:                       auth,
				evaluationSetService:       evalSetSvc,
				evaluationSetSchemaService: schemaSvc,
				metric:                     metric,
			}

			req := tc.buildReq()
			tc.setup(auth, evalSetSvc, schemaSvc)

			resp, err := app.UpdateEvaluationSetSchemaOApi(context.Background(), req)

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
				assert.NotNil(t, resp)
			}

			assert.True(t, metric.called)
			assert.Equal(t, workspaceID, metric.spaceID)
		})
	}
}

func TestEvalOpenAPIApplication_ReportEvalTargetInvokeResult(t *testing.T) {
	t.Parallel()

	repoErrorReq := newSuccessInvokeResultReq(11, 101)
	reportErrorReq := newSuccessInvokeResultReq(22, 202)
	publisherErrorReq := newSuccessInvokeResultReq(33, 303)
	successReq := newSuccessInvokeResultReq(44, 404)
	failedReq := newFailedInvokeResultReq(55, 505, "invoke failed")

	tests := []struct {
		name    string
		req     *openapi.ReportEvalTargetInvokeResultRequest
		setup   func(t *testing.T, asyncRepo *repomocks.MockIEvalAsyncRepo, targetSvc *servicemocks.MockIEvalTargetService, publisher *eventmocks.MockExptEventPublisher, configer *configermocks.MockIConfiger)
		wantErr bool
	}{
		{
			name: "repo returns error",
			req:  repoErrorReq,
			setup: func(t *testing.T, asyncRepo *repomocks.MockIEvalAsyncRepo, _ *servicemocks.MockIEvalTargetService, _ *eventmocks.MockExptEventPublisher, _ *configermocks.MockIConfiger) {
				asyncRepo.EXPECT().GetEvalAsyncCtx(gomock.Any(), strconv.FormatInt(repoErrorReq.GetInvokeID(), 10)).Return(nil, errors.New("repo error"))
			},
			wantErr: true,
		},
		{
			name: "report invoke records returns error",
			req:  reportErrorReq,
			setup: func(t *testing.T, asyncRepo *repomocks.MockIEvalAsyncRepo, targetSvc *servicemocks.MockIEvalTargetService, publisher *eventmocks.MockExptEventPublisher, _ *configermocks.MockIConfiger) {
				actx := &entity.EvalAsyncCtx{AsyncUnixMS: time.Now().Add(-200 * time.Millisecond).UnixMilli()}
				asyncRepo.EXPECT().GetEvalAsyncCtx(gomock.Any(), strconv.FormatInt(reportErrorReq.GetInvokeID(), 10)).Return(actx, nil)
				targetSvc.EXPECT().ReportInvokeRecords(gomock.Any(), gomock.AssignableToTypeOf(&entity.ReportTargetRecordParam{})).DoAndReturn(func(_ context.Context, param *entity.ReportTargetRecordParam) error {
					assert.Equal(t, reportErrorReq.GetWorkspaceID(), param.SpaceID)
					assert.Equal(t, reportErrorReq.GetInvokeID(), param.RecordID)
					assert.Equal(t, entity.EvalTargetRunStatusSuccess, param.Status)
					if assert.NotNil(t, param.OutputData) {
						assert.NotNil(t, param.OutputData.EvalTargetUsage)
						assert.NotNil(t, param.OutputData.TimeConsumingMS)
						if param.OutputData.TimeConsumingMS != nil {
							assert.Greater(t, *param.OutputData.TimeConsumingMS, int64(0))
						}
					}
					assert.Nil(t, param.Session)
					return errors.New("report error")
				})
				publisher.EXPECT().PublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr: true,
		},
		{
			name: "publisher returns error",
			req:  publisherErrorReq,
			setup: func(t *testing.T, asyncRepo *repomocks.MockIEvalAsyncRepo, targetSvc *servicemocks.MockIEvalTargetService, publisher *eventmocks.MockExptEventPublisher, configer *configermocks.MockIConfiger) {
				session := &entity.Session{UserID: "user"}
				event := &entity.ExptItemEvalEvent{}
				actx := &entity.EvalAsyncCtx{AsyncUnixMS: time.Now().Add(-150 * time.Millisecond).UnixMilli(), Event: event, Session: session}
				asyncRepo.EXPECT().GetEvalAsyncCtx(gomock.Any(), strconv.FormatInt(publisherErrorReq.GetInvokeID(), 10)).Return(actx, nil)
				targetSvc.EXPECT().ReportInvokeRecords(gomock.Any(), gomock.AssignableToTypeOf(&entity.ReportTargetRecordParam{})).DoAndReturn(func(_ context.Context, param *entity.ReportTargetRecordParam) error {
					assert.Equal(t, session, param.Session)
					return nil
				})
				conf := &entity.TargetTrajectoryConf{}
				configer.EXPECT().GetTargetTrajectoryConf(gomock.Any()).Return(conf)
				publisher.EXPECT().PublishExptRecordEvalEvent(gomock.Any(), event, gomock.Any()).DoAndReturn(func(_ context.Context, evt *entity.ExptItemEvalEvent, duration *time.Duration) error {
					assert.Equal(t, event, evt)
					if assert.NotNil(t, duration) {
						assert.Equal(t, 13*time.Second, *duration)
					}
					return errors.New("publish error")
				})
			},
			wantErr: true,
		},
		{
			name: "success without event",
			req:  successReq,
			setup: func(t *testing.T, asyncRepo *repomocks.MockIEvalAsyncRepo, targetSvc *servicemocks.MockIEvalTargetService, publisher *eventmocks.MockExptEventPublisher, _ *configermocks.MockIConfiger) {
				actx := &entity.EvalAsyncCtx{AsyncUnixMS: time.Now().Add(-100 * time.Millisecond).UnixMilli()}
				asyncRepo.EXPECT().GetEvalAsyncCtx(gomock.Any(), strconv.FormatInt(successReq.GetInvokeID(), 10)).Return(actx, nil)
				targetSvc.EXPECT().ReportInvokeRecords(gomock.Any(), gomock.AssignableToTypeOf(&entity.ReportTargetRecordParam{})).DoAndReturn(func(_ context.Context, param *entity.ReportTargetRecordParam) error {
					assert.Nil(t, param.Session)
					return nil
				})
				publisher.EXPECT().PublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr: false,
		},
		{
			name: "success with event on failure status",
			req:  failedReq,
			setup: func(t *testing.T, asyncRepo *repomocks.MockIEvalAsyncRepo, targetSvc *servicemocks.MockIEvalTargetService, publisher *eventmocks.MockExptEventPublisher, configer *configermocks.MockIConfiger) {
				session := &entity.Session{UserID: "owner"}
				event := &entity.ExptItemEvalEvent{}
				actx := &entity.EvalAsyncCtx{AsyncUnixMS: time.Now().Add(-120 * time.Millisecond).UnixMilli(), Event: event, Session: session}
				asyncRepo.EXPECT().GetEvalAsyncCtx(gomock.Any(), strconv.FormatInt(failedReq.GetInvokeID(), 10)).Return(actx, nil)
				targetSvc.EXPECT().ReportInvokeRecords(gomock.Any(), gomock.AssignableToTypeOf(&entity.ReportTargetRecordParam{})).DoAndReturn(func(_ context.Context, param *entity.ReportTargetRecordParam) error {
					assert.Equal(t, entity.EvalTargetRunStatusFail, param.Status)
					if assert.NotNil(t, param.OutputData) {
						if assert.NotNil(t, param.OutputData.EvalTargetRunError) {
							assert.Equal(t, failedReq.GetErrorMessage(), param.OutputData.EvalTargetRunError.Message)
						}
						assert.NotNil(t, param.OutputData.TimeConsumingMS)
					}
					assert.Equal(t, session, param.Session)
					return nil
				})
				conf := &entity.TargetTrajectoryConf{}
				configer.EXPECT().GetTargetTrajectoryConf(gomock.Any()).Return(conf)
				publisher.EXPECT().PublishExptRecordEvalEvent(gomock.Any(), event, gomock.Any()).DoAndReturn(func(_ context.Context, evt *entity.ExptItemEvalEvent, duration *time.Duration) error {
					assert.Equal(t, event, evt)
					if assert.NotNil(t, duration) {
						assert.Equal(t, 13*time.Second, *duration)
					}
					return nil
				})
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		caseData := tc
		t.Run(caseData.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			asyncRepo := repomocks.NewMockIEvalAsyncRepo(ctrl)
			targetSvc := servicemocks.NewMockIEvalTargetService(ctrl)
			publisher := eventmocks.NewMockExptEventPublisher(ctrl)
			configer := configermocks.NewMockIConfiger(ctrl)

			app := &EvalOpenAPIApplication{
				targetSvc: targetSvc,
				asyncRepo: asyncRepo,
				publisher: publisher,
				configer:  configer,
			}

			caseData.setup(t, asyncRepo, targetSvc, publisher, configer)

			resp, err := app.ReportEvalTargetInvokeResult_(context.Background(), caseData.req)
			if caseData.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
				return
			}

			assert.NoError(t, err)
			if assert.NotNil(t, resp) {
				assert.NotNil(t, resp.BaseResp)
			}
		})
	}
}

func TestEvalOpenAPIApplication_SubmitExperimentOApi(t *testing.T) {
	t.Parallel()

	workspaceID := int64(30101)
	evalSetID := int64(30102)
	evaluatorID := int64(40101)
	evaluatorVersionID := int64(40102)

	buildBaseReq := func() *openapi.SubmitExperimentOApiRequest {
		evalVersion := "v1"
		evaluatorVersion := "1.0"
		return &openapi.SubmitExperimentOApiRequest{
			WorkspaceID: gptr.Of(workspaceID),
			Name:        gptr.Of("experiment"),
			EvalSetParam: &openapi.SubmitExperimentEvalSetParam{
				EvalSetID: gptr.Of(evalSetID),
				Version:   &evalVersion,
			},
			EvaluatorParams: []*openapi.SubmitExperimentEvaluatorParam{
				{
					EvaluatorID: gptr.Of(evaluatorID),
					Version:     &evaluatorVersion,
				},
			},
		}
	}

	tests := []struct {
		name     string
		buildReq func() *openapi.SubmitExperimentOApiRequest
		setup    func(req *openapi.SubmitExperimentOApiRequest, auth *rpcmocks.MockIAuthProvider, manager *servicemocks.MockIExptManager, versionSvc *servicemocks.MockEvaluationSetVersionService, evaluatorSvc *servicemocks.MockEvaluatorService, fakeApp *fakeExperimentApp)
		wantErr  int32
	}{
		{
			name:     "nil request",
			buildReq: func() *openapi.SubmitExperimentOApiRequest { return nil },
			setup: func(_ *openapi.SubmitExperimentOApiRequest, auth *rpcmocks.MockIAuthProvider, manager *servicemocks.MockIExptManager, versionSvc *servicemocks.MockEvaluationSetVersionService, evaluatorSvc *servicemocks.MockEvaluatorService, _ *fakeExperimentApp) {
				auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Times(0)
				manager.EXPECT().CheckName(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				versionSvc.EXPECT().ListEvaluationSetVersions(gomock.Any(), gomock.Any()).Times(0)
				evaluatorSvc.EXPECT().ListEvaluatorVersion(gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr: errno.CommonInvalidParamCode,
		},
		{
			name: "invalid workspace",
			buildReq: func() *openapi.SubmitExperimentOApiRequest {
				req := buildBaseReq()
				req.WorkspaceID = gptr.Of(int64(0))
				return req
			},
			setup: func(_ *openapi.SubmitExperimentOApiRequest, auth *rpcmocks.MockIAuthProvider, manager *servicemocks.MockIExptManager, versionSvc *servicemocks.MockEvaluationSetVersionService, evaluatorSvc *servicemocks.MockEvaluatorService, _ *fakeExperimentApp) {
				auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Times(0)
				manager.EXPECT().CheckName(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				versionSvc.EXPECT().ListEvaluationSetVersions(gomock.Any(), gomock.Any()).Times(0)
				evaluatorSvc.EXPECT().ListEvaluatorVersion(gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr: errno.CommonInvalidParamCode,
		},
		{
			name: "missing eval set version",
			buildReq: func() *openapi.SubmitExperimentOApiRequest {
				req := buildBaseReq()
				req.EvalSetParam.Version = nil
				return req
			},
			setup: func(_ *openapi.SubmitExperimentOApiRequest, auth *rpcmocks.MockIAuthProvider, manager *servicemocks.MockIExptManager, versionSvc *servicemocks.MockEvaluationSetVersionService, evaluatorSvc *servicemocks.MockEvaluatorService, _ *fakeExperimentApp) {
				auth.EXPECT().Authorization(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationParam{})).Times(0)
				manager.EXPECT().CheckName(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				versionSvc.EXPECT().ListEvaluationSetVersions(gomock.Any(), gomock.Any()).Times(0)
				evaluatorSvc.EXPECT().ListEvaluatorVersion(gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr: errno.CommonInvalidParamCode,
		},
		{
			name:     "auth failed",
			buildReq: buildBaseReq,
			setup: func(req *openapi.SubmitExperimentOApiRequest, auth *rpcmocks.MockIAuthProvider, manager *servicemocks.MockIExptManager, versionSvc *servicemocks.MockEvaluationSetVersionService, evaluatorSvc *servicemocks.MockEvaluatorService, _ *fakeExperimentApp) {
				auth.EXPECT().Authorization(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationParam{})).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
				manager.EXPECT().CheckName(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				versionSvc.EXPECT().ListEvaluationSetVersions(gomock.Any(), gomock.Any()).Times(0)
				evaluatorSvc.EXPECT().ListEvaluatorVersion(gomock.Any(), gomock.Any()).Times(0)
				_ = req
			},
			wantErr: errno.CommonNoPermissionCode,
		},
		{
			name:     "name duplicate",
			buildReq: buildBaseReq,
			setup: func(req *openapi.SubmitExperimentOApiRequest, auth *rpcmocks.MockIAuthProvider, manager *servicemocks.MockIExptManager, versionSvc *servicemocks.MockEvaluationSetVersionService, evaluatorSvc *servicemocks.MockEvaluatorService, _ *fakeExperimentApp) {
				auth.EXPECT().Authorization(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationParam{})).Return(nil)
				manager.EXPECT().CheckName(gomock.Any(), req.GetName(), req.GetWorkspaceID(), gomock.AssignableToTypeOf(&entity.Session{})).Return(false, nil)
				versionSvc.EXPECT().ListEvaluationSetVersions(gomock.Any(), gomock.Any()).Times(0)
				evaluatorSvc.EXPECT().ListEvaluatorVersion(gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr: errno.CommonInvalidParamCode,
		},
		{
			name:     "eval set version not found",
			buildReq: buildBaseReq,
			setup: func(req *openapi.SubmitExperimentOApiRequest, auth *rpcmocks.MockIAuthProvider, manager *servicemocks.MockIExptManager, versionSvc *servicemocks.MockEvaluationSetVersionService, evaluatorSvc *servicemocks.MockEvaluatorService, _ *fakeExperimentApp) {
				auth.EXPECT().Authorization(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationParam{})).Return(nil)
				manager.EXPECT().CheckName(gomock.Any(), req.GetName(), req.GetWorkspaceID(), gomock.AssignableToTypeOf(&entity.Session{})).Return(true, nil)
				versionSvc.EXPECT().ListEvaluationSetVersions(gomock.Any(), gomock.AssignableToTypeOf(&entity.ListEvaluationSetVersionsParam{})).Return(nil, nil, nil, nil)
				evaluatorSvc.EXPECT().ListEvaluatorVersion(gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr: errno.ResourceNotFoundCode,
		},
		{
			name:     "evaluator not found",
			buildReq: buildBaseReq,
			setup: func(req *openapi.SubmitExperimentOApiRequest, auth *rpcmocks.MockIAuthProvider, manager *servicemocks.MockIExptManager, versionSvc *servicemocks.MockEvaluationSetVersionService, evaluatorSvc *servicemocks.MockEvaluatorService, _ *fakeExperimentApp) {
				auth.EXPECT().Authorization(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationParam{})).Return(nil)
				manager.EXPECT().CheckName(gomock.Any(), req.GetName(), req.GetWorkspaceID(), gomock.AssignableToTypeOf(&entity.Session{})).Return(true, nil)
				versionSvc.EXPECT().ListEvaluationSetVersions(gomock.Any(), gomock.AssignableToTypeOf(&entity.ListEvaluationSetVersionsParam{})).Return([]*entity.EvaluationSetVersion{{ID: evaluatorVersionID}}, nil, nil, nil)
				evaluatorSvc.EXPECT().ListEvaluatorVersion(gomock.Any(), gomock.AssignableToTypeOf(&entity.ListEvaluatorVersionRequest{})).Return(nil, int64(0), nil)
			},
			wantErr: errno.ResourceNotFoundCode,
		},
		{
			name:     "success",
			buildReq: buildBaseReq,
			setup: func(req *openapi.SubmitExperimentOApiRequest, auth *rpcmocks.MockIAuthProvider, manager *servicemocks.MockIExptManager, versionSvc *servicemocks.MockEvaluationSetVersionService, evaluatorSvc *servicemocks.MockEvaluatorService, fakeApp *fakeExperimentApp) {
				auth.EXPECT().Authorization(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationParam{})).Return(nil)
				manager.EXPECT().CheckName(gomock.Any(), req.GetName(), req.GetWorkspaceID(), gomock.AssignableToTypeOf(&entity.Session{})).Return(true, nil)
				versionSvc.EXPECT().ListEvaluationSetVersions(gomock.Any(), gomock.AssignableToTypeOf(&entity.ListEvaluationSetVersionsParam{})).Return([]*entity.EvaluationSetVersion{{ID: evaluatorVersionID}}, nil, nil, nil)
				evaluator := &entity.Evaluator{
					EvaluatorType: entity.EvaluatorTypePrompt,
					PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
						ID:          evaluatorVersionID,
						EvaluatorID: evaluatorID,
						Version:     "1.0",
					},
				}
				evaluatorSvc.EXPECT().ListEvaluatorVersion(gomock.Any(), gomock.AssignableToTypeOf(&entity.ListEvaluatorVersionRequest{})).Return([]*entity.Evaluator{evaluator}, int64(1), nil)
				fakeApp.submitResp = &exptpb.SubmitExperimentResponse{Experiment: &domainexpt.Experiment{ID: gptr.Of(int64(8888))}}
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			manager := servicemocks.NewMockIExptManager(ctrl)
			versionSvc := servicemocks.NewMockEvaluationSetVersionService(ctrl)
			evaluatorSvc := servicemocks.NewMockEvaluatorService(ctrl)
			metric := &fakeOpenAPIMetric{}
			fakeApp := &fakeExperimentApp{}

			app := &EvalOpenAPIApplication{
				auth:                        auth,
				manager:                     manager,
				evaluationSetVersionService: versionSvc,
				evaluatorService:            evaluatorSvc,
				experimentApp:               fakeApp,
				metric:                      metric,
			}

			req := tc.buildReq()
			if tc.setup != nil {
				tc.setup(req, auth, manager, versionSvc, evaluatorSvc, fakeApp)
			}

			resp, err := app.SubmitExperimentOApi(context.Background(), req)

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
				if assert.NotNil(t, resp) && assert.NotNil(t, resp.Data) {
					assert.NotNil(t, resp.Data.Experiment)
					assert.Equal(t, int64(8888), resp.Data.Experiment.GetID())
				}
				if assert.NotNil(t, fakeApp.lastReq) {
					assert.Equal(t, workspaceID, fakeApp.lastReq.GetWorkspaceID())
					assert.Len(t, fakeApp.lastReq.EvaluatorVersionIds, 1)
					assert.Equal(t, evaluatorVersionID, fakeApp.lastReq.EvaluatorVersionIds[0])
				}
			}

			if req != nil {
				assert.True(t, metric.called)
				assert.Equal(t, req.GetWorkspaceID(), metric.spaceID)
			}
		})
	}
}

func TestEvalOpenAPIApplication_GetExperimentsOApi(t *testing.T) {
	t.Parallel()

	workspaceID := int64(60101)
	experimentID := int64(70101)

	buildReq := func() *openapi.GetExperimentsOApiRequest {
		return &openapi.GetExperimentsOApiRequest{
			WorkspaceID:  gptr.Of(workspaceID),
			ExperimentID: gptr.Of(experimentID),
		}
	}

	tests := []struct {
		name    string
		setup   func(req *openapi.GetExperimentsOApiRequest, auth *rpcmocks.MockIAuthProvider, manager *servicemocks.MockIExptManager)
		wantErr int32
	}{
		{
			name: "manager error",
			setup: func(req *openapi.GetExperimentsOApiRequest, _ *rpcmocks.MockIAuthProvider, manager *servicemocks.MockIExptManager) {
				manager.EXPECT().GetDetail(gomock.Any(), req.GetExperimentID(), req.GetWorkspaceID(), gomock.AssignableToTypeOf(&entity.Session{})).Return(nil, errors.New("detail error"))
			},
			wantErr: -1,
		},
		{
			name: "auth failed",
			setup: func(req *openapi.GetExperimentsOApiRequest, auth *rpcmocks.MockIAuthProvider, manager *servicemocks.MockIExptManager) {
				detail := &entity.Experiment{ID: req.GetExperimentID(), SpaceID: req.GetWorkspaceID(), CreatedBy: "owner"}
				manager.EXPECT().GetDetail(gomock.Any(), req.GetExperimentID(), req.GetWorkspaceID(), gomock.AssignableToTypeOf(&entity.Session{})).Return(detail, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr: errno.CommonNoPermissionCode,
		},
		{
			name: "success",
			setup: func(req *openapi.GetExperimentsOApiRequest, auth *rpcmocks.MockIAuthProvider, manager *servicemocks.MockIExptManager) {
				detail := &entity.Experiment{ID: req.GetExperimentID(), SpaceID: req.GetWorkspaceID(), CreatedBy: "owner", Name: "exp"}
				manager.EXPECT().GetDetail(gomock.Any(), req.GetExperimentID(), req.GetWorkspaceID(), gomock.AssignableToTypeOf(&entity.Session{})).Return(detail, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).DoAndReturn(func(_ context.Context, param *rpc.AuthorizationWithoutSPIParam) error {
					assert.Equal(t, strconv.FormatInt(req.GetExperimentID(), 10), param.ObjectID)
					assert.Equal(t, req.GetWorkspaceID(), param.SpaceID)
					return nil
				})
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			manager := servicemocks.NewMockIExptManager(ctrl)
			metric := &fakeOpenAPIMetric{}

			app := &EvalOpenAPIApplication{
				auth:    auth,
				manager: manager,
				metric:  metric,
			}

			req := buildReq()
			if tc.setup != nil {
				tc.setup(req, auth, manager)
			}

			resp, err := app.GetExperimentsOApi(context.Background(), req)

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
				if assert.NotNil(t, resp) && assert.NotNil(t, resp.Data) {
					assert.NotNil(t, resp.Data.Experiment)
					assert.Equal(t, experimentID, resp.Data.Experiment.GetID())
				}
			}

			assert.True(t, metric.called)
			assert.Equal(t, workspaceID, metric.spaceID)
		})
	}
}

func TestEvalOpenAPIApplication_ListExperimentResultOApi(t *testing.T) {
	t.Parallel()

	workspaceID := int64(80101)
	experimentID := int64(80102)

	buildReq := func() *openapi.ListExperimentResultOApiRequest {
		pageSize := int32(10)
		return &openapi.ListExperimentResultOApiRequest{
			WorkspaceID:  gptr.Of(workspaceID),
			ExperimentID: gptr.Of(experimentID),
			PageSize:     &pageSize,
		}
	}

	tests := []struct {
		name    string
		setup   func(auth *rpcmocks.MockIAuthProvider, resultSvc *servicemocks.MockExptResultService)
		wantErr int32
	}{
		{
			name: "auth failed",
			setup: func(auth *rpcmocks.MockIAuthProvider, resultSvc *servicemocks.MockExptResultService) {
				auth.EXPECT().Authorization(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationParam{})).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
				resultSvc.EXPECT().MGetExperimentResult(gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr: errno.CommonNoPermissionCode,
		},
		{
			name: "service error",
			setup: func(auth *rpcmocks.MockIAuthProvider, resultSvc *servicemocks.MockExptResultService) {
				auth.EXPECT().Authorization(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationParam{})).Return(nil)
				resultSvc.EXPECT().MGetExperimentResult(gomock.Any(), gomock.AssignableToTypeOf(&entity.MGetExperimentResultParam{})).
					Return(nil, errors.New("result error"))
			},
			wantErr: -1,
		},
		{
			name: "success",
			setup: func(auth *rpcmocks.MockIAuthProvider, resultSvc *servicemocks.MockExptResultService) {
				auth.EXPECT().Authorization(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationParam{})).Return(nil)
				columnEvaluators := []*entity.ColumnEvaluator{{EvaluatorVersionID: 1}}
				columnFields := []*entity.ColumnEvalSetField{{Key: gptr.Of("field"), Name: gptr.Of("Field"), ContentType: entity.ContentTypeText}}
				itemResults := []*entity.ItemResult{{ItemID: 10}}
				resultSvc.EXPECT().MGetExperimentResult(gomock.Any(), gomock.AssignableToTypeOf(&entity.MGetExperimentResultParam{})).
					Return(&entity.MGetExperimentReportResult{
						ColumnEvaluators:    columnEvaluators,
						ColumnEvalSetFields: columnFields,
						ItemResults:         itemResults,
						Total:               int64(3),
					}, nil)
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			resultSvc := servicemocks.NewMockExptResultService(ctrl)
			metric := &fakeOpenAPIMetric{}

			app := &EvalOpenAPIApplication{
				auth:      auth,
				resultSvc: resultSvc,
				metric:    metric,
			}

			req := buildReq()
			tc.setup(auth, resultSvc)

			resp, err := app.ListExperimentResultOApi(context.Background(), req)

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
				if assert.NotNil(t, resp) && assert.NotNil(t, resp.Data) {
					assert.EqualValues(t, 3, resp.Data.GetTotal())
					assert.Len(t, resp.Data.ColumnEvalSetFields, 1)
					assert.Len(t, resp.Data.ColumnEvaluators, 1)
				}
			}

			assert.True(t, metric.called)
			assert.Equal(t, workspaceID, metric.spaceID)
		})
	}
}

func TestEvalOpenAPIApplication_GetExperimentAggrResultOApi(t *testing.T) {
	t.Parallel()

	workspaceID := int64(90101)
	experimentID := int64(90102)
	evaluatorID := int64(90103)
	evaluatorVersionID := int64(90104)
	score := 0.9

	buildReq := func() *openapi.GetExperimentAggrResultOApiRequest {
		return &openapi.GetExperimentAggrResultOApiRequest{
			WorkspaceID:  gptr.Of(workspaceID),
			ExperimentID: gptr.Of(experimentID),
		}
	}

	tests := []struct {
		name    string
		setup   func(auth *rpcmocks.MockIAuthProvider, aggSvc *servicemocks.MockExptAggrResultService)
		wantErr int32
	}{
		{
			name: "auth failed",
			setup: func(auth *rpcmocks.MockIAuthProvider, aggSvc *servicemocks.MockExptAggrResultService) {
				auth.EXPECT().Authorization(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationParam{})).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
				aggSvc.EXPECT().BatchGetExptAggrResultByExperimentIDs(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr: errno.CommonNoPermissionCode,
		},
		{
			name: "service error",
			setup: func(auth *rpcmocks.MockIAuthProvider, aggSvc *servicemocks.MockExptAggrResultService) {
				auth.EXPECT().Authorization(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationParam{})).Return(nil)
				aggSvc.EXPECT().BatchGetExptAggrResultByExperimentIDs(gomock.Any(), workspaceID, []int64{experimentID}).Return(nil, errors.New("aggr error"))
			},
			wantErr: -1,
		},
		{
			name: "result not found",
			setup: func(auth *rpcmocks.MockIAuthProvider, aggSvc *servicemocks.MockExptAggrResultService) {
				auth.EXPECT().Authorization(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationParam{})).Return(nil)
				aggSvc.EXPECT().BatchGetExptAggrResultByExperimentIDs(gomock.Any(), workspaceID, []int64{experimentID}).Return([]*entity.ExptAggregateResult{}, nil)
			},
			wantErr: errno.ResourceNotFoundCode,
		},
		{
			name: "success",
			setup: func(auth *rpcmocks.MockIAuthProvider, aggSvc *servicemocks.MockExptAggrResultService) {
				auth.EXPECT().Authorization(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationParam{})).Return(nil)
				aggregator := &entity.AggregatorResult{
					AggregatorType: entity.Average,
					Data: &entity.AggregateData{
						DataType: entity.Double,
						Value:    &score,
					},
				}
				evaluatorResult := &entity.EvaluatorAggregateResult{
					EvaluatorID:        evaluatorID,
					EvaluatorVersionID: evaluatorVersionID,
					AggregatorResults:  []*entity.AggregatorResult{aggregator},
					Name:               gptr.Of("eval"),
					Version:            gptr.Of("1.0"),
				}
				aggResult := &entity.ExptAggregateResult{
					ExperimentID:     experimentID,
					EvaluatorResults: map[int64]*entity.EvaluatorAggregateResult{evaluatorID: evaluatorResult},
				}
				aggSvc.EXPECT().BatchGetExptAggrResultByExperimentIDs(gomock.Any(), workspaceID, []int64{experimentID}).Return([]*entity.ExptAggregateResult{aggResult}, nil)
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			aggSvc := servicemocks.NewMockExptAggrResultService(ctrl)
			metric := &fakeOpenAPIMetric{}

			app := &EvalOpenAPIApplication{
				auth:                  auth,
				ExptAggrResultService: aggSvc,
				metric:                metric,
			}

			req := buildReq()
			tc.setup(auth, aggSvc)

			resp, err := app.GetExperimentAggrResultOApi(context.Background(), req)

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
				if assert.NotNil(t, resp) && assert.NotNil(t, resp.Data) {
					assert.Len(t, resp.Data.EvaluatorResults, 1)
					assert.Equal(t, evaluatorID, resp.Data.EvaluatorResults[0].GetEvaluatorID())
					assert.NotNil(t, resp.Data.EvaluatorResults[0].AggregatorResults)
				}
			}

			assert.True(t, metric.called)
			assert.Equal(t, workspaceID, metric.spaceID)
		})
	}
}

type fakeExperimentApp struct {
	evaluation.ExperimentService
	service.ExptSchedulerEvent
	service.ExptItemEvalEvent
	service.ExptAggrResultService
	service.IExptResultExportService
	service.IExptInsightAnalysisService

	submitResp *exptpb.SubmitExperimentResponse
	submitErr  error
	lastReq    *exptpb.SubmitExperimentRequest
}

func (f *fakeExperimentApp) SubmitExperiment(ctx context.Context, req *exptpb.SubmitExperimentRequest) (*exptpb.SubmitExperimentResponse, error) {
	f.lastReq = req
	if f.submitResp != nil || f.submitErr != nil {
		return f.submitResp, f.submitErr
	}
	return &exptpb.SubmitExperimentResponse{}, nil
}

var _ IExperimentApplication = (*fakeExperimentApp)(nil)

func newSuccessInvokeResultReq(workspaceID, invokeID int64) *openapi.ReportEvalTargetInvokeResultRequest {
	status := spi.InvokeEvalTargetStatus_SUCCESS
	contentType := spi.ContentTypeText
	text := "result"
	inputTokens := int64(10)
	outputTokens := int64(20)

	return &openapi.ReportEvalTargetInvokeResultRequest{
		WorkspaceID: gptr.Of(workspaceID),
		InvokeID:    gptr.Of(invokeID),
		Status:      &status,
		Output: &spi.InvokeEvalTargetOutput{
			ActualOutput: &spi.Content{
				ContentType: &contentType,
				Text:        gptr.Of(text),
			},
		},
		Usage: &spi.InvokeEvalTargetUsage{
			InputTokens:  &inputTokens,
			OutputTokens: &outputTokens,
		},
	}
}

func newFailedInvokeResultReq(workspaceID, invokeID int64, errorMessage string) *openapi.ReportEvalTargetInvokeResultRequest {
	status := spi.InvokeEvalTargetStatus_FAILED

	return &openapi.ReportEvalTargetInvokeResultRequest{
		WorkspaceID:  gptr.Of(workspaceID),
		InvokeID:     gptr.Of(invokeID),
		Status:       &status,
		ErrorMessage: gptr.Of(errorMessage),
	}
}

func TestEvalOpenAPIApplication_GetEvaluationItemFieldOApi(t *testing.T) {
	t.Parallel()

	workspaceID := int64(61001)
	evalSetID := int64(61002)
	itemID := int64(9001)
	versionID := int64(8001)
	turnID := int64(7001)
	fieldName := "question"

	buildReq := func() *openapi.GetEvaluationItemFieldOApiRequest {
		return &openapi.GetEvaluationItemFieldOApiRequest{
			WorkspaceID:     gptr.Of(workspaceID),
			EvaluationSetID: gptr.Of(evalSetID),
			VersionID:       gptr.Of(versionID),
			ItemID:          gptr.Of(itemID),
			FieldName:       gptr.Of(fieldName),
			TurnID:          gptr.Of(turnID),
		}
	}

	tests := []struct {
		name    string
		req     *openapi.GetEvaluationItemFieldOApiRequest
		setup   func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, itemSvc *servicemocks.MockEvaluationSetItemService)
		wantErr int32
		check   func(t *testing.T, resp *openapi.GetEvaluationItemFieldOApiResponse)
	}{
		{
			name: "nil request",
			req:  nil,
			setup: func(_ *rpcmocks.MockIAuthProvider, _ *servicemocks.MockIEvaluationSetService, _ *servicemocks.MockEvaluationSetItemService) {
			},
			wantErr: errno.CommonInvalidParamCode,
		},
		{
			name: "set not found",
			req:  buildReq(),
			setup: func(_ *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, _ *servicemocks.MockEvaluationSetItemService) {
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evalSetID, gomock.AssignableToTypeOf(gptr.Of(true))).Return(nil, nil)
			},
			wantErr: errno.ResourceNotFoundCode,
		},
		{
			name: "auth failed",
			req:  buildReq(),
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, _ *servicemocks.MockEvaluationSetItemService) {
				set := &entity.EvaluationSet{ID: evalSetID, SpaceID: workspaceID}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evalSetID, gomock.AssignableToTypeOf(gptr.Of(true))).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr: errno.CommonNoPermissionCode,
		},
		{
			name: "batch get items error",
			req:  buildReq(),
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, itemSvc *servicemocks.MockEvaluationSetItemService) {
				set := &entity.EvaluationSet{ID: evalSetID, SpaceID: workspaceID}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evalSetID, gomock.AssignableToTypeOf(gptr.Of(true))).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(nil)
				itemSvc.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.AssignableToTypeOf(&entity.BatchGetEvaluationSetItemsParam{})).Return(nil, errors.New("batch error"))
			},
			wantErr: -1,
		},
		{
			name: "item not found",
			req:  buildReq(),
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, itemSvc *servicemocks.MockEvaluationSetItemService) {
				set := &entity.EvaluationSet{ID: evalSetID, SpaceID: workspaceID}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evalSetID, gomock.AssignableToTypeOf(gptr.Of(true))).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(nil)
				itemSvc.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.AssignableToTypeOf(&entity.BatchGetEvaluationSetItemsParam{})).Return([]*entity.EvaluationSetItem{}, nil)
			},
			wantErr: errno.ResourceNotFoundCode,
		},
		{
			name: "get field error",
			req:  buildReq(),
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, itemSvc *servicemocks.MockEvaluationSetItemService) {
				set := &entity.EvaluationSet{ID: evalSetID, SpaceID: workspaceID}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evalSetID, gomock.AssignableToTypeOf(gptr.Of(true))).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(nil)
				itemSvc.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.AssignableToTypeOf(&entity.BatchGetEvaluationSetItemsParam{})).Return([]*entity.EvaluationSetItem{{ID: itemID}}, nil)
				itemSvc.EXPECT().GetEvaluationSetItemField(gomock.Any(), gomock.AssignableToTypeOf(&entity.GetEvaluationSetItemFieldParam{})).Return(nil, errors.New("field error"))
			},
			wantErr: -1,
		},
		{
			name: "success",
			req:  buildReq(),
			setup: func(auth *rpcmocks.MockIAuthProvider, evalSetSvc *servicemocks.MockIEvaluationSetService, itemSvc *servicemocks.MockEvaluationSetItemService) {
				owner := gptr.Of("owner")
				set := &entity.EvaluationSet{ID: evalSetID, SpaceID: workspaceID, BaseInfo: &entity.BaseInfo{CreatedBy: &entity.UserInfo{UserID: owner}}}
				evalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), evalSetID, gomock.AssignableToTypeOf(gptr.Of(true))).Return(set, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).DoAndReturn(func(_ context.Context, param *rpc.AuthorizationWithoutSPIParam) error {
					assert.Equal(t, strconv.FormatInt(evalSetID, 10), param.ObjectID)
					assert.Equal(t, workspaceID, param.SpaceID)
					return nil
				})
				itemSvc.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.AssignableToTypeOf(&entity.BatchGetEvaluationSetItemsParam{})).Return([]*entity.EvaluationSetItem{{ID: itemID}}, nil)
				fd := &entity.FieldData{Name: fieldName}
				itemSvc.EXPECT().GetEvaluationSetItemField(gomock.Any(), gomock.AssignableToTypeOf(&entity.GetEvaluationSetItemFieldParam{})).DoAndReturn(func(_ context.Context, param *entity.GetEvaluationSetItemFieldParam) (*entity.FieldData, error) {
					assert.Equal(t, workspaceID, param.SpaceID)
					assert.Equal(t, evalSetID, param.EvaluationSetID)
					assert.Equal(t, itemID, param.ItemPK)
					assert.Equal(t, fieldName, param.FieldName)
					assert.Equal(t, turnID, gptr.Indirect(param.TurnID))
					return fd, nil
				})
			},
			check: func(t *testing.T, resp *openapi.GetEvaluationItemFieldOApiResponse) {
				if assert.NotNil(t, resp) {
					assert.NotNil(t, resp.FieldData)
					assert.Equal(t, fieldName, resp.FieldData.GetName())
				}
			},
		},
	}

	for _, tt := range tests {
		caseData := tt
		t.Run(caseData.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			evalSetSvc := servicemocks.NewMockIEvaluationSetService(ctrl)
			itemSvc := servicemocks.NewMockEvaluationSetItemService(ctrl)
			metric := &fakeOpenAPIMetric{}

			app := &EvalOpenAPIApplication{
				auth:                     auth,
				evaluationSetService:     evalSetSvc,
				evaluationSetItemService: itemSvc,
				metric:                   metric,
			}

			if caseData.setup != nil {
				caseData.setup(auth, evalSetSvc, itemSvc)
			}

			resp, err := app.GetEvaluationItemFieldOApi(context.Background(), caseData.req)

			if caseData.wantErr != 0 {
				assert.Error(t, err)
				if caseData.wantErr > 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, caseData.wantErr, statusErr.Code())
				}
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				if caseData.check != nil {
					caseData.check(t, resp)
				}
			}

			if caseData.req != nil {
				assert.True(t, metric.called)
				assert.Equal(t, caseData.req.GetWorkspaceID(), metric.spaceID)
				assert.Equal(t, caseData.req.GetEvaluationSetID(), metric.evaluationSetID)
			}
		})
	}
}
