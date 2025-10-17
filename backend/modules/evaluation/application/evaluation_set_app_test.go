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

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	domain_eval_set "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_set"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_set"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/evaluation_set"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	metricsmock "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/userinfo"
	userinfomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/userinfo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestEvaluationSetApplicationImpl_CreateEvaluationSet(t *testing.T) {
	type fields struct {
		auth                        rpc.IAuthProvider
		metric                      metrics.EvaluationSetMetrics
		evaluationSetService        service.IEvaluationSetService
		evaluationSetSchemaService  service.EvaluationSetSchemaService
		evaluationSetVersionService service.EvaluationSetVersionService
		evaluationSetItemService    service.EvaluationSetItemService
		userInfoService             userinfo.UserInfoService
	}
	type args struct {
		ctx context.Context
		req *eval_set.CreateEvaluationSetRequest
	}
	// 创建 mock 控制器
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建 mock 实例
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvaluationSetService := mocks.NewMockIEvaluationSetService(ctrl)
	mockEvaluationSetMetrics := metricsmock.NewMockEvaluationSetMetrics(ctrl)

	// 定义测试用例
	tests := []struct {
		name     string
		fields   fields
		args     args
		wantResp *eval_set.CreateEvaluationSetResponse
		wantErr  bool
	}{
		{
			name: "成功创建评估集",
			fields: fields{
				auth:                 mockAuth,
				evaluationSetService: mockEvaluationSetService,
				metric:               mockEvaluationSetMetrics,
			},
			args: args{
				ctx: context.Background(),
				req: &eval_set.CreateEvaluationSetRequest{
					// 填充请求参数
					Name:                gptr.Of("test"),
					EvaluationSetSchema: &domain_eval_set.EvaluationSetSchema{},
				},
			},
			wantResp: &eval_set.CreateEvaluationSetResponse{
				// 填充期望响应
				EvaluationSetID: gptr.Of(int64(123)),
			},
			wantErr: false,
		},
		{
			name: "创建评估集失败",
			fields: fields{
				auth:                 mockAuth,
				evaluationSetService: mockEvaluationSetService,
				metric:               mockEvaluationSetMetrics,
			},
			args: args{
				ctx: context.Background(),
				req: &eval_set.CreateEvaluationSetRequest{
					// 填充请求参数
					Name:                gptr.Of("test"),
					EvaluationSetSchema: &domain_eval_set.EvaluationSetSchema{},
				},
			},
			wantResp: nil,
			wantErr:  true,
		},
	}

	// 为每个测试用例设置 mock 行为
	for _, tt := range tests {
		// 模拟鉴权方法
		mockAuth.EXPECT().Authorization(tt.args.ctx, gomock.Any()).Return(nil)
		mockEvaluationSetMetrics.EXPECT().EmitCreate(gomock.Any(), gomock.Any()).Return()

		if tt.wantErr {
			// 模拟创建评估集失败
			mockEvaluationSetService.EXPECT().CreateEvaluationSet(tt.args.ctx, gomock.Any()).Return(int64(0), fmt.Errorf("创建评估集失败"))
		} else {
			// 模拟创建评估集成功
			mockEvaluationSetService.EXPECT().CreateEvaluationSet(tt.args.ctx, gomock.Any()).Return(int64(123), nil)
		}

		t.Run(tt.name, func(t *testing.T) {
			e := &EvaluationSetApplicationImpl{
				auth:                        tt.fields.auth,
				metric:                      tt.fields.metric,
				evaluationSetService:        tt.fields.evaluationSetService,
				evaluationSetSchemaService:  tt.fields.evaluationSetSchemaService,
				evaluationSetVersionService: tt.fields.evaluationSetVersionService,
				evaluationSetItemService:    tt.fields.evaluationSetItemService,
				userInfoService:             tt.fields.userInfoService,
			}
			gotResp, err := e.CreateEvaluationSet(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateEvaluationSet() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResp, tt.wantResp) {
				t.Errorf("CreateEvaluationSet() gotResp = %v, want %v", gotResp, tt.wantResp)
			}
		})
	}
}

func TestEvaluationSetApplicationImpl_UpdateEvaluationSet(t *testing.T) {
	type fields struct {
		auth                        rpc.IAuthProvider
		metric                      metrics.EvaluationSetMetrics
		evaluationSetService        service.IEvaluationSetService
		evaluationSetSchemaService  service.EvaluationSetSchemaService
		evaluationSetVersionService service.EvaluationSetVersionService
		evaluationSetItemService    service.EvaluationSetItemService
		userInfoService             userinfo.UserInfoService
	}
	type args struct {
		ctx context.Context
		req *eval_set.UpdateEvaluationSetRequest
	}
	// 创建 mock 控制器
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建 mock 实例
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvaluationSetService := mocks.NewMockIEvaluationSetService(ctrl)
	mockEvaluationSetMetrics := metricsmock.NewMockEvaluationSetMetrics(ctrl)

	// 定义测试用例
	tests := []struct {
		name     string
		fields   fields
		args     args
		wantResp *eval_set.UpdateEvaluationSetResponse
		wantErr  bool
	}{
		{
			name: "成功更新评估集",
			fields: fields{
				auth:                 mockAuth,
				evaluationSetService: mockEvaluationSetService,
				metric:               mockEvaluationSetMetrics,
			},
			args: args{
				ctx: context.Background(),
				req: &eval_set.UpdateEvaluationSetRequest{
					// 填充请求参数
					WorkspaceID:     int64(123),
					EvaluationSetID: int64(123),
					Name:            gptr.Of("updated_test"),
				},
			},
			wantResp: &eval_set.UpdateEvaluationSetResponse{
				// 填充期望响应
			},
			wantErr: false,
		},
		{
			name: "更新评估集失败",
			fields: fields{
				auth:                 mockAuth,
				evaluationSetService: mockEvaluationSetService,
				metric:               mockEvaluationSetMetrics,
			},
			args: args{
				ctx: context.Background(),
				req: &eval_set.UpdateEvaluationSetRequest{
					// 填充请求参数
					WorkspaceID:     int64(123),
					EvaluationSetID: int64(123),
					Name:            gptr.Of("updated_test"),
				},
			},
			wantResp: nil,
			wantErr:  true,
		},
	}

	// 为每个测试用例设置 mock 行为
	for _, tt := range tests {
		// 模拟鉴权方法
		mockAuth.EXPECT().AuthorizationWithoutSPI(tt.args.ctx, gomock.Any()).Return(nil)
		mockEvaluationSetService.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvaluationSet{}, nil)

		if tt.wantErr {
			// 模拟更新评估集失败
			mockEvaluationSetService.EXPECT().UpdateEvaluationSet(tt.args.ctx, gomock.Any()).Return(fmt.Errorf("更新评估集失败"))
		} else {
			// 模拟更新评估集成功
			mockEvaluationSetService.EXPECT().UpdateEvaluationSet(tt.args.ctx, gomock.Any()).Return(nil)
		}

		t.Run(tt.name, func(t *testing.T) {
			e := &EvaluationSetApplicationImpl{
				auth:                        tt.fields.auth,
				metric:                      tt.fields.metric,
				evaluationSetService:        tt.fields.evaluationSetService,
				evaluationSetSchemaService:  tt.fields.evaluationSetSchemaService,
				evaluationSetVersionService: tt.fields.evaluationSetVersionService,
				evaluationSetItemService:    tt.fields.evaluationSetItemService,
				userInfoService:             tt.fields.userInfoService,
			}
			gotResp, err := e.UpdateEvaluationSet(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateEvaluationSet() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResp, tt.wantResp) {
				t.Errorf("UpdateEvaluationSet() gotResp = %v, want %v", gotResp, tt.wantResp)
			}
		})
	}
}

func TestEvaluationSetApplicationImpl_DeleteEvaluationSet(t *testing.T) {
	type fields struct {
		auth                        rpc.IAuthProvider
		metric                      metrics.EvaluationSetMetrics
		evaluationSetService        service.IEvaluationSetService
		evaluationSetSchemaService  service.EvaluationSetSchemaService
		evaluationSetVersionService service.EvaluationSetVersionService
		evaluationSetItemService    service.EvaluationSetItemService
		userInfoService             userinfo.UserInfoService
	}
	type args struct {
		ctx context.Context
		req *eval_set.DeleteEvaluationSetRequest
	}
	// 创建 mock 控制器
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建 mock 实例
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvaluationSetService := mocks.NewMockIEvaluationSetService(ctrl)
	mockEvaluationSetMetrics := metricsmock.NewMockEvaluationSetMetrics(ctrl)

	// 定义测试用例
	tests := []struct {
		name     string
		fields   fields
		args     args
		wantResp *eval_set.DeleteEvaluationSetResponse
		wantErr  bool
	}{
		{
			name: "成功删除评估集",
			fields: fields{
				auth:                 mockAuth,
				evaluationSetService: mockEvaluationSetService,
				metric:               mockEvaluationSetMetrics,
			},
			args: args{
				ctx: context.Background(),
				req: &eval_set.DeleteEvaluationSetRequest{
					// 填充请求参数
					WorkspaceID:     int64(123),
					EvaluationSetID: int64(123),
				},
			},
			wantResp: &eval_set.DeleteEvaluationSetResponse{
				// 填充期望响应
			},
			wantErr: false,
		},
		{
			name: "删除评估集失败",
			fields: fields{
				auth:                 mockAuth,
				evaluationSetService: mockEvaluationSetService,
				metric:               mockEvaluationSetMetrics,
			},
			args: args{
				ctx: context.Background(),
				req: &eval_set.DeleteEvaluationSetRequest{
					// 填充请求参数
					WorkspaceID:     int64(123),
					EvaluationSetID: int64(123),
				},
			},
			wantResp: nil,
			wantErr:  true,
		},
	}

	// 为每个测试用例设置 mock 行为
	for _, tt := range tests {
		// 模拟参数校验通过
		// 模拟鉴权方法
		mockAuth.EXPECT().AuthorizationWithoutSPI(tt.args.ctx, gomock.Any()).Return(nil)
		mockEvaluationSetService.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvaluationSet{}, nil)
		if tt.wantErr {
			// 模拟删除评估集失败
			mockEvaluationSetService.EXPECT().DeleteEvaluationSet(tt.args.ctx, gomock.Any(), gomock.Any()).Return(fmt.Errorf("删除评估集失败"))
		} else {
			// 模拟删除评估集成功
			mockEvaluationSetService.EXPECT().DeleteEvaluationSet(tt.args.ctx, gomock.Any(), gomock.Any()).Return(nil)
		}

		t.Run(tt.name, func(t *testing.T) {
			e := &EvaluationSetApplicationImpl{
				auth:                        tt.fields.auth,
				metric:                      tt.fields.metric,
				evaluationSetService:        tt.fields.evaluationSetService,
				evaluationSetSchemaService:  tt.fields.evaluationSetSchemaService,
				evaluationSetVersionService: tt.fields.evaluationSetVersionService,
				evaluationSetItemService:    tt.fields.evaluationSetItemService,
				userInfoService:             tt.fields.userInfoService,
			}
			gotResp, err := e.DeleteEvaluationSet(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteEvaluationSet() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResp, tt.wantResp) {
				t.Errorf("DeleteEvaluationSet() gotResp = %v, want %v", gotResp, tt.wantResp)
			}
		})
	}
}

func TestEvaluationSetApplicationImpl_GetEvaluationSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluationSetService := mocks.NewMockIEvaluationSetService(ctrl)
	mockAuthProvider := rpcmocks.NewMockIAuthProvider(ctrl)
	mockUserInfoService := userinfomocks.NewMockUserInfoService(ctrl)

	service := &EvaluationSetApplicationImpl{
		evaluationSetService: mockEvaluationSetService,
		auth:                 mockAuthProvider,
		userInfoService:      mockUserInfoService,
	}

	// Test data
	validSpaceID := int64(123)
	validEvaluationSetID := int64(456)
	validSet := &entity.EvaluationSet{
		ID:      validEvaluationSetID,
		SpaceID: validSpaceID,
		BaseInfo: &entity.BaseInfo{
			CreatedBy: &entity.UserInfo{
				UserID: gptr.Of("user-123"),
			},
		},
	}

	tests := []struct {
		name           string
		req            *eval_set.GetEvaluationSetRequest
		mockSetup      func()
		wantResp       *eval_set.GetEvaluationSetResponse
		wantErr        bool
		wantErrCode    int32
		wantErrMessage string
	}{
		{
			name: "success - valid request",
			req: &eval_set.GetEvaluationSetRequest{
				WorkspaceID:     validSpaceID,
				EvaluationSetID: validEvaluationSetID,
			},
			mockSetup: func() {
				mockEvaluationSetService.EXPECT().
					GetEvaluationSet(gomock.Any(), &validSpaceID, validEvaluationSetID, nil).
					Return(validSet, nil)

				mockAuthProvider.EXPECT().
					AuthorizationWithoutSPI(gomock.Any(), &rpc.AuthorizationWithoutSPIParam{
						ObjectID:        strconv.FormatInt(validEvaluationSetID, 10),
						SpaceID:         validSpaceID,
						ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
						OwnerID:         gptr.Of("user-123"),
						ResourceSpaceID: validSpaceID,
					}).
					Return(nil)

				mockUserInfoService.EXPECT().
					PackUserInfo(gomock.Any(), gomock.Any()).
					Return()
			},
			wantResp: &eval_set.GetEvaluationSetResponse{
				EvaluationSet: evaluation_set.EvaluationSetDO2DTO(validSet),
			},
			wantErr: false,
		},
		{
			name: "error - nil request",
			req:  nil,
			mockSetup: func() {
				// No mocks needed
			},
			wantErr:        true,
			wantErrCode:    errno.CommonInvalidParamCode,
			wantErrMessage: "req is nil",
		},
		{
			name: "error - evaluation set not found",
			req: &eval_set.GetEvaluationSetRequest{
				WorkspaceID:     validSpaceID,
				EvaluationSetID: validEvaluationSetID,
			},
			mockSetup: func() {
				mockEvaluationSetService.EXPECT().
					GetEvaluationSet(gomock.Any(), &validSpaceID, validEvaluationSetID, nil).
					Return(nil, nil)
			},
			wantErr:        true,
			wantErrCode:    errno.ResourceNotFoundCode,
			wantErrMessage: "experiment set not found",
		},
		{
			name: "error - evaluation set service error",
			req: &eval_set.GetEvaluationSetRequest{
				WorkspaceID:     validSpaceID,
				EvaluationSetID: validEvaluationSetID,
			},
			mockSetup: func() {
				mockEvaluationSetService.EXPECT().
					GetEvaluationSet(gomock.Any(), &validSpaceID, validEvaluationSetID, nil).
					Return(nil, errors.New("service error"))
			},
			wantErr:        true,
			wantErrMessage: "service error",
		},
		{
			name: "error - auth failed",
			req: &eval_set.GetEvaluationSetRequest{
				WorkspaceID:     validSpaceID,
				EvaluationSetID: validEvaluationSetID,
			},
			mockSetup: func() {
				mockEvaluationSetService.EXPECT().
					GetEvaluationSet(gomock.Any(), &validSpaceID, validEvaluationSetID, nil).
					Return(validSet, nil)

				mockAuthProvider.EXPECT().
					AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr:        true,
			wantErrCode:    errno.CommonNoPermissionCode,
			wantErrMessage: "no access permission",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := service.GetEvaluationSet(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
				if tt.wantErrMessage != "" {
					assert.Contains(t, err.Error(), tt.wantErrMessage)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantResp, resp)
			}
		})
	}
}

func TestEvaluationSetApplicationImpl_ListEvaluationSets(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvalSetService := mocks.NewMockIEvaluationSetService(ctrl)
	mockUserInfoService := userinfomocks.NewMockUserInfoService(ctrl)

	app := &EvaluationSetApplicationImpl{
		auth:                 mockAuth,
		evaluationSetService: mockEvalSetService,
		userInfoService:      mockUserInfoService,
	}

	tests := []struct {
		name        string
		req         *eval_set.ListEvaluationSetsRequest
		mockSetup   func()
		wantResp    *eval_set.ListEvaluationSetsResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &eval_set.ListEvaluationSetsRequest{
				WorkspaceID:      123,
				EvaluationSetIds: []int64{1, 2},
				Name:             gptr.Of("test"),
				Creators:         []string{"user1"},
				PageNumber:       gptr.Of(int32(1)),
				PageSize:         gptr.Of(int32(10)),
				PageToken:        gptr.Of("token"),
			},
			mockSetup: func() {
				// Setup auth mock
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)

				// Setup service mock
				mockEvalSetService.EXPECT().ListEvaluationSets(gomock.Any(), gomock.Any()).
					Return([]*entity.EvaluationSet{
						{ID: 1, Name: "set1"},
						{ID: 2, Name: "set2"},
					}, gptr.Of(int64(2)), gptr.Of("next_token"), nil)

				// Setup user info mock
				mockUserInfoService.EXPECT().PackUserInfo(gomock.Any(), gomock.Any())
			},
			wantResp: &eval_set.ListEvaluationSetsResponse{
				EvaluationSets: []*domain_eval_set.EvaluationSet{
					{ID: gptr.Of(int64(1)), Name: gptr.Of("set1")},
					{ID: gptr.Of(int64(2)), Name: gptr.Of("set2")},
				},
				Total:         gptr.Of(int64(2)),
				NextPageToken: gptr.Of("next_token"),
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
			name: "error - auth failed",
			req: &eval_set.ListEvaluationSetsRequest{
				WorkspaceID: 123,
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
			name: "error - service returns error",
			req: &eval_set.ListEvaluationSetsRequest{
				WorkspaceID: 123,
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockEvalSetService.EXPECT().ListEvaluationSets(gomock.Any(), gomock.Any()).
					Return(nil, nil, nil, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
		{
			name: "success - empty evaluation set IDs",
			req: &eval_set.ListEvaluationSetsRequest{
				WorkspaceID:      123,
				EvaluationSetIds: []int64{},
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockEvalSetService.EXPECT().ListEvaluationSets(gomock.Any(), gomock.Any()).
					Return([]*entity.EvaluationSet{}, gptr.Of(int64(0)), nil, nil)
				mockUserInfoService.EXPECT().PackUserInfo(gomock.Any(), gomock.Any())
			},
			wantResp: &eval_set.ListEvaluationSetsResponse{
				EvaluationSets: []*domain_eval_set.EvaluationSet{},
				Total:          gptr.Of(int64(0)),
			},
			wantErr: false,
		},
		{
			name: "success - nil pagination params",
			req: &eval_set.ListEvaluationSetsRequest{
				WorkspaceID: 123,
				PageNumber:  nil,
				PageSize:    nil,
				PageToken:   nil,
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockEvalSetService.EXPECT().ListEvaluationSets(gomock.Any(), gomock.Any()).
					Return([]*entity.EvaluationSet{
						{ID: 1, Name: "set1"},
					}, gptr.Of(int64(1)), nil, nil)
				mockUserInfoService.EXPECT().PackUserInfo(gomock.Any(), gomock.Any())
			},
			wantResp: &eval_set.ListEvaluationSetsResponse{
				EvaluationSets: []*domain_eval_set.EvaluationSet{
					{ID: gptr.Of(int64(1)), Name: gptr.Of("set1")},
				},
				Total: gptr.Of(int64(1)),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := app.ListEvaluationSets(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.wantResp.EvaluationSets), len(resp.EvaluationSets))
			}
		})
	}
}

func TestEvaluationSetApplicationImpl_BatchCreateEvaluationSetItems(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvalSetService := mocks.NewMockIEvaluationSetService(ctrl)
	mockEvalSetItemService := mocks.NewMockEvaluationSetItemService(ctrl)

	app := &EvaluationSetApplicationImpl{
		auth:                     mockAuth,
		evaluationSetService:     mockEvalSetService,
		evaluationSetItemService: mockEvalSetItemService,
	}

	// Test data
	validSpaceID := int64(123)
	validEvalSetID := int64(456)
	validSet := &entity.EvaluationSet{
		ID:      validEvalSetID,
		SpaceID: validSpaceID,
		BaseInfo: &entity.BaseInfo{
			CreatedBy: &entity.UserInfo{
				UserID: gptr.Of("user-123"),
			},
		},
	}
	validItems := []*domain_eval_set.EvaluationSetItem{
		{ID: gptr.Of(int64(1))},
		{ID: gptr.Of(int64(2))},
	}

	tests := []struct {
		name        string
		req         *eval_set.BatchCreateEvaluationSetItemsRequest
		mockSetup   func()
		wantResp    *eval_set.BatchCreateEvaluationSetItemsResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &eval_set.BatchCreateEvaluationSetItemsRequest{
				WorkspaceID:     validSpaceID,
				EvaluationSetID: validEvalSetID,
				Items:           validItems,
			},
			mockSetup: func() {
				// Mock get evaluation set
				mockEvalSetService.EXPECT().
					GetEvaluationSet(gomock.Any(), &validSpaceID, validEvalSetID, nil).
					Return(validSet, nil)

				// Mock auth
				mockAuth.EXPECT().
					AuthorizationWithoutSPI(gomock.Any(), &rpc.AuthorizationWithoutSPIParam{
						ObjectID:        strconv.FormatInt(validEvalSetID, 10),
						SpaceID:         validSpaceID,
						ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
						OwnerID:         gptr.Of("user-123"),
						ResourceSpaceID: validSpaceID,
					}).
					Return(nil)

				// Mock batch create
				mockEvalSetItemService.EXPECT().
					BatchCreateEvaluationSetItems(gomock.Any(), gomock.Any()).
					Return(map[int64]int64{1: 101, 2: 102}, nil, nil, nil)
			},
			wantResp: &eval_set.BatchCreateEvaluationSetItemsResponse{
				AddedItems:  map[int64]int64{1: 101, 2: 102},
				Errors:      nil,
				ItemOutputs: nil,
			},
			wantErr: false,
		},
		{
			name: "error - nil request",
			req:  nil,
			mockSetup: func() {
				// No mocks needed
			},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - empty items",
			req: &eval_set.BatchCreateEvaluationSetItemsRequest{
				WorkspaceID:     validSpaceID,
				EvaluationSetID: validEvalSetID,
				Items:           nil,
			},
			mockSetup: func() {
				// No mocks needed
			},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - evaluation set not found",
			req: &eval_set.BatchCreateEvaluationSetItemsRequest{
				WorkspaceID:     validSpaceID,
				EvaluationSetID: validEvalSetID,
				Items:           validItems,
			},
			mockSetup: func() {
				mockEvalSetService.EXPECT().
					GetEvaluationSet(gomock.Any(), &validSpaceID, validEvalSetID, nil).
					Return(nil, nil)
			},
			wantErr:     true,
			wantErrCode: errno.ResourceNotFoundCode,
		},
		{
			name: "error - auth failed",
			req: &eval_set.BatchCreateEvaluationSetItemsRequest{
				WorkspaceID:     validSpaceID,
				EvaluationSetID: validEvalSetID,
				Items:           validItems,
			},
			mockSetup: func() {
				mockEvalSetService.EXPECT().
					GetEvaluationSet(gomock.Any(), &validSpaceID, validEvalSetID, nil).
					Return(validSet, nil)

				mockAuth.EXPECT().
					AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
		{
			name: "error - batch create failed",
			req: &eval_set.BatchCreateEvaluationSetItemsRequest{
				WorkspaceID:     validSpaceID,
				EvaluationSetID: validEvalSetID,
				Items:           validItems,
			},
			mockSetup: func() {
				mockEvalSetService.EXPECT().
					GetEvaluationSet(gomock.Any(), &validSpaceID, validEvalSetID, nil).
					Return(validSet, nil)

				mockAuth.EXPECT().
					AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).
					Return(nil)

				mockEvalSetItemService.EXPECT().
					BatchCreateEvaluationSetItems(gomock.Any(), gomock.Any()).
					Return(nil, nil, nil, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := app.BatchCreateEvaluationSetItems(context.Background(), tt.req)

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

func TestEvaluationSetApplicationImpl_UpdateEvaluationSetItem(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvalSetService := mocks.NewMockIEvaluationSetService(ctrl)
	mockEvalSetItemService := mocks.NewMockEvaluationSetItemService(ctrl)

	app := &EvaluationSetApplicationImpl{
		auth:                     mockAuth,
		evaluationSetService:     mockEvalSetService,
		evaluationSetItemService: mockEvalSetItemService,
	}

	// Test data
	validSpaceID := int64(123)
	validEvalSetID := int64(456)
	validItemID := int64(789)
	validSet := &entity.EvaluationSet{
		ID:      validEvalSetID,
		SpaceID: validSpaceID,
		BaseInfo: &entity.BaseInfo{
			CreatedBy: &entity.UserInfo{
				UserID: gptr.Of("user-123"),
			},
		},
	}
	validTurns := []*domain_eval_set.Turn{
		{ID: gptr.Of(int64(1))},
	}

	tests := []struct {
		name        string
		req         *eval_set.UpdateEvaluationSetItemRequest
		mockSetup   func()
		wantResp    *eval_set.UpdateEvaluationSetItemResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal update",
			req: &eval_set.UpdateEvaluationSetItemRequest{
				WorkspaceID:     validSpaceID,
				EvaluationSetID: validEvalSetID,
				ItemID:          validItemID,
				Turns:           validTurns,
			},
			mockSetup: func() {
				// Mock get evaluation set
				mockEvalSetService.EXPECT().
					GetEvaluationSet(gomock.Any(), &validSpaceID, validEvalSetID, nil).
					Return(validSet, nil)

				// Mock auth
				mockAuth.EXPECT().
					AuthorizationWithoutSPI(gomock.Any(), &rpc.AuthorizationWithoutSPIParam{
						ObjectID:        strconv.FormatInt(validEvalSetID, 10),
						SpaceID:         validSpaceID,
						ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
						OwnerID:         gptr.Of("user-123"),
						ResourceSpaceID: validSpaceID,
					}).
					Return(nil)

				// Mock update
				mockEvalSetItemService.EXPECT().
					UpdateEvaluationSetItem(gomock.Any(), validSpaceID, validEvalSetID, validItemID, gomock.Any()).
					Return(nil)
			},
			wantResp: &eval_set.UpdateEvaluationSetItemResponse{},
			wantErr:  false,
		},
		{
			name: "error - nil request",
			req:  nil,
			mockSetup: func() {
				// No mocks needed
			},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - evaluation set not found",
			req: &eval_set.UpdateEvaluationSetItemRequest{
				WorkspaceID:     validSpaceID,
				EvaluationSetID: validEvalSetID,
				ItemID:          validItemID,
				Turns:           validTurns,
			},
			mockSetup: func() {
				mockEvalSetService.EXPECT().
					GetEvaluationSet(gomock.Any(), &validSpaceID, validEvalSetID, nil).
					Return(nil, nil)
			},
			wantErr:     true,
			wantErrCode: errno.ResourceNotFoundCode,
		},
		{
			name: "error - auth failed",
			req: &eval_set.UpdateEvaluationSetItemRequest{
				WorkspaceID:     validSpaceID,
				EvaluationSetID: validEvalSetID,
				ItemID:          validItemID,
				Turns:           validTurns,
			},
			mockSetup: func() {
				mockEvalSetService.EXPECT().
					GetEvaluationSet(gomock.Any(), &validSpaceID, validEvalSetID, nil).
					Return(validSet, nil)

				mockAuth.EXPECT().
					AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
		{
			name: "error - update failed",
			req: &eval_set.UpdateEvaluationSetItemRequest{
				WorkspaceID:     validSpaceID,
				EvaluationSetID: validEvalSetID,
				ItemID:          validItemID,
				Turns:           validTurns,
			},
			mockSetup: func() {
				mockEvalSetService.EXPECT().
					GetEvaluationSet(gomock.Any(), &validSpaceID, validEvalSetID, nil).
					Return(validSet, nil)

				mockAuth.EXPECT().
					AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).
					Return(nil)

				mockEvalSetItemService.EXPECT().
					UpdateEvaluationSetItem(gomock.Any(), validSpaceID, validEvalSetID, validItemID, gomock.Any()).
					Return(errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := app.UpdateEvaluationSetItem(context.Background(), tt.req)

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

func TestEvaluationSetApplicationImpl_BatchDeleteEvaluationSetItems(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvalSetService := mocks.NewMockIEvaluationSetService(ctrl)
	mockEvalSetItemService := mocks.NewMockEvaluationSetItemService(ctrl)

	app := &EvaluationSetApplicationImpl{
		auth:                     mockAuth,
		evaluationSetService:     mockEvalSetService,
		evaluationSetItemService: mockEvalSetItemService,
	}

	// Test data
	validSpaceID := int64(123)
	validEvalSetID := int64(456)
	validItemIDs := []int64{789, 790}
	validSet := &entity.EvaluationSet{
		ID:      validEvalSetID,
		SpaceID: validSpaceID,
		BaseInfo: &entity.BaseInfo{
			CreatedBy: &entity.UserInfo{
				UserID: gptr.Of("user-123"),
			},
		},
	}

	tests := []struct {
		name        string
		req         *eval_set.BatchDeleteEvaluationSetItemsRequest
		mockSetup   func()
		wantResp    *eval_set.BatchDeleteEvaluationSetItemsResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal delete",
			req: &eval_set.BatchDeleteEvaluationSetItemsRequest{
				WorkspaceID:     validSpaceID,
				EvaluationSetID: validEvalSetID,
				ItemIds:         validItemIDs,
			},
			mockSetup: func() {
				// Mock get evaluation set
				mockEvalSetService.EXPECT().
					GetEvaluationSet(gomock.Any(), &validSpaceID, validEvalSetID, nil).
					Return(validSet, nil)

				// Mock auth
				mockAuth.EXPECT().
					AuthorizationWithoutSPI(gomock.Any(), &rpc.AuthorizationWithoutSPIParam{
						ObjectID:        strconv.FormatInt(validEvalSetID, 10),
						SpaceID:         validSpaceID,
						ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
						OwnerID:         gptr.Of("user-123"),
						ResourceSpaceID: validSpaceID,
					}).
					Return(nil)

				// Mock batch delete
				mockEvalSetItemService.EXPECT().
					BatchDeleteEvaluationSetItems(gomock.Any(), validSpaceID, validEvalSetID, validItemIDs).
					Return(nil)
			},
			wantResp: &eval_set.BatchDeleteEvaluationSetItemsResponse{},
			wantErr:  false,
		},
		{
			name: "error - nil request",
			req:  nil,
			mockSetup: func() {
				// No mocks needed
			},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - evaluation set not found",
			req: &eval_set.BatchDeleteEvaluationSetItemsRequest{
				WorkspaceID:     validSpaceID,
				EvaluationSetID: validEvalSetID,
				ItemIds:         validItemIDs,
			},
			mockSetup: func() {
				mockEvalSetService.EXPECT().
					GetEvaluationSet(gomock.Any(), &validSpaceID, validEvalSetID, nil).
					Return(nil, nil)
			},
			wantErr:     true,
			wantErrCode: errno.ResourceNotFoundCode,
		},
		{
			name: "error - auth failed",
			req: &eval_set.BatchDeleteEvaluationSetItemsRequest{
				WorkspaceID:     validSpaceID,
				EvaluationSetID: validEvalSetID,
				ItemIds:         validItemIDs,
			},
			mockSetup: func() {
				mockEvalSetService.EXPECT().
					GetEvaluationSet(gomock.Any(), &validSpaceID, validEvalSetID, nil).
					Return(validSet, nil)

				mockAuth.EXPECT().
					AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
		{
			name: "error - batch delete failed",
			req: &eval_set.BatchDeleteEvaluationSetItemsRequest{
				WorkspaceID:     validSpaceID,
				EvaluationSetID: validEvalSetID,
				ItemIds:         validItemIDs,
			},
			mockSetup: func() {
				mockEvalSetService.EXPECT().
					GetEvaluationSet(gomock.Any(), &validSpaceID, validEvalSetID, nil).
					Return(validSet, nil)

				mockAuth.EXPECT().
					AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).
					Return(nil)

				mockEvalSetItemService.EXPECT().
					BatchDeleteEvaluationSetItems(gomock.Any(), validSpaceID, validEvalSetID, validItemIDs).
					Return(errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := app.BatchDeleteEvaluationSetItems(context.Background(), tt.req)

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

func TestEvaluationSetApplicationImpl_ListEvaluationSetItems(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建 mock 依赖
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockItemService := mocks.NewMockEvaluationSetItemService(ctrl)
	mockUserInfo := userinfomocks.NewMockUserInfoService(ctrl)
	mockEvalSetService := mocks.NewMockIEvaluationSetService(ctrl)
	// 初始化测试对象
	service := &EvaluationSetApplicationImpl{
		auth:                     mockAuth,
		evaluationSetService:     mockEvalSetService,
		evaluationSetItemService: mockItemService,
		userInfoService:          mockUserInfo,
	}

	// 定义测试用例
	tests := []struct {
		name        string
		req         *eval_set.ListEvaluationSetItemsRequest
		mockSetup   func()
		wantResp    *eval_set.ListEvaluationSetItemsResponse
		wantErr     bool
		wantErrCode int32
	}{{
		name: "正常场景: 获取评估集条目列表成功",
		req: &eval_set.ListEvaluationSetItemsRequest{
			WorkspaceID:     int64(123),
			EvaluationSetID: int64(456),
			PageNumber:      gptr.Of(int32(1)),
			PageSize:        gptr.Of(int32(10)),
		},
		mockSetup: func() {
			// 模拟鉴权通过
			mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
			mockEvalSetService.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvaluationSet{ID: int64(456)}, nil)
			// 模拟服务层返回数据
			mockItemService.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).
				Return([]*entity.EvaluationSetItem{
					{ID: int64(1), EvaluationSetID: int64(456)},
					{ID: int64(2), EvaluationSetID: int64(456)},
				}, gptr.Of(int64(2)), gptr.Of("test"), nil)
		},
		wantResp: &eval_set.ListEvaluationSetItemsResponse{
			Items: []*domain_eval_set.EvaluationSetItem{
				{ID: gptr.Of(int64(1))},
				{ID: gptr.Of(int64(2))},
			},
			Total: gptr.Of(int64(2)),
		},
		wantErr: false,
	}}

	// 执行测试用例
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := service.ListEvaluationSetItems(context.Background(), tt.req)

			// 验证错误
			if tt.wantErr {
				assert.Error(t, err)
				statusErr, ok := errorx.FromStatusError(err)
				assert.True(t, ok)
				assert.Equal(t, tt.wantErrCode, statusErr.Code())
				return
			}

			// 验证成功结果
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, tt.wantResp.Total, resp.Total)
			assert.Equal(t, len(tt.wantResp.Items), len(resp.Items))
		})
	}
}

func TestEvaluationSetApplicationImpl_BatchGetEvaluationSetItems(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvalSetService := mocks.NewMockIEvaluationSetService(ctrl)
	mockEvalSetItemService := mocks.NewMockEvaluationSetItemService(ctrl)
	mockUserInfo := userinfomocks.NewMockUserInfoService(ctrl)

	app := &EvaluationSetApplicationImpl{
		auth:                     mockAuth,
		evaluationSetService:     mockEvalSetService,
		evaluationSetItemService: mockEvalSetItemService,
		userInfoService:          mockUserInfo,
	}

	// Test data
	validSpaceID := int64(123)
	validEvalSetID := int64(456)
	validItemIDs := []int64{789, 790}
	validSet := &entity.EvaluationSet{
		ID:      validEvalSetID,
		SpaceID: validSpaceID,
		BaseInfo: &entity.BaseInfo{
			CreatedBy: &entity.UserInfo{
				UserID: gptr.Of("user-123"),
			},
		},
	}
	validItems := []*entity.EvaluationSetItem{
		{
			ID:              789,
			EvaluationSetID: validEvalSetID,
			BaseInfo: &entity.BaseInfo{
				CreatedBy: &entity.UserInfo{
					UserID: gptr.Of("user-123"),
				},
			},
		},
		{
			ID:              790,
			EvaluationSetID: validEvalSetID,
			BaseInfo: &entity.BaseInfo{
				CreatedBy: &entity.UserInfo{
					UserID: gptr.Of("user-123"),
				},
			},
		},
	}

	tests := []struct {
		name        string
		req         *eval_set.BatchGetEvaluationSetItemsRequest
		mockSetup   func()
		wantResp    *eval_set.BatchGetEvaluationSetItemsResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal get",
			req: &eval_set.BatchGetEvaluationSetItemsRequest{
				WorkspaceID:     validSpaceID,
				EvaluationSetID: validEvalSetID,
				ItemIds:         validItemIDs,
			},
			mockSetup: func() {
				// Mock get evaluation set
				mockEvalSetService.EXPECT().
					GetEvaluationSet(gomock.Any(), &validSpaceID, validEvalSetID, gomock.Any()).
					Return(validSet, nil)

				// Mock auth
				mockAuth.EXPECT().
					AuthorizationWithoutSPI(gomock.Any(), &rpc.AuthorizationWithoutSPIParam{
						ObjectID:        strconv.FormatInt(validEvalSetID, 10),
						SpaceID:         validSpaceID,
						ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
						OwnerID:         gptr.Of("user-123"),
						ResourceSpaceID: validSpaceID,
					}).
					Return(nil)

				// Mock batch get
				mockEvalSetItemService.EXPECT().
					BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).
					Return(validItems, nil)
			},
			wantResp: &eval_set.BatchGetEvaluationSetItemsResponse{
				Items: []*domain_eval_set.EvaluationSetItem{
					{ID: gptr.Of(int64(789))},
					{ID: gptr.Of(int64(790))},
				},
			},
			wantErr: false,
		},
		{
			name: "error - nil request",
			req:  nil,
			mockSetup: func() {
				// No mocks needed
			},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - evaluation set not found",
			req: &eval_set.BatchGetEvaluationSetItemsRequest{
				WorkspaceID:     validSpaceID,
				EvaluationSetID: validEvalSetID,
				ItemIds:         validItemIDs,
			},
			mockSetup: func() {
				mockEvalSetService.EXPECT().
					GetEvaluationSet(gomock.Any(), &validSpaceID, validEvalSetID, gomock.Any()).
					Return(nil, nil)
			},
			wantErr:     true,
			wantErrCode: errno.ResourceNotFoundCode,
		},
		{
			name: "error - auth failed",
			req: &eval_set.BatchGetEvaluationSetItemsRequest{
				WorkspaceID:     validSpaceID,
				EvaluationSetID: validEvalSetID,
				ItemIds:         validItemIDs,
			},
			mockSetup: func() {
				mockEvalSetService.EXPECT().
					GetEvaluationSet(gomock.Any(), &validSpaceID, validEvalSetID, gomock.Any()).
					Return(validSet, nil)

				mockAuth.EXPECT().
					AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
		{
			name: "error - batch get failed",
			req: &eval_set.BatchGetEvaluationSetItemsRequest{
				WorkspaceID:     validSpaceID,
				EvaluationSetID: validEvalSetID,
				ItemIds:         validItemIDs,
			},
			mockSetup: func() {
				mockEvalSetService.EXPECT().
					GetEvaluationSet(gomock.Any(), &validSpaceID, validEvalSetID, gomock.Any()).
					Return(validSet, nil)

				mockAuth.EXPECT().
					AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).
					Return(nil)

				mockEvalSetItemService.EXPECT().
					BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).
					Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := app.BatchGetEvaluationSetItems(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.wantResp.Items), len(resp.Items))
				for i, item := range tt.wantResp.Items {
					assert.Equal(t, item.ID, resp.Items[i].ID)
				}
			}
		})
	}
}

func TestEvaluationSetApplicationImpl_UpdateEvaluationSetSchema(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvalSetService := mocks.NewMockIEvaluationSetService(ctrl)
	mockEvalSetSchemaService := mocks.NewMockEvaluationSetSchemaService(ctrl)

	app := &EvaluationSetApplicationImpl{
		auth:                       mockAuth,
		evaluationSetService:       mockEvalSetService,
		evaluationSetSchemaService: mockEvalSetSchemaService,
	}

	// Test data
	validSpaceID := int64(123)
	validEvalSetID := int64(456)
	validSchema := &domain_eval_set.EvaluationSetSchema{
		FieldSchemas: []*domain_eval_set.FieldSchema{
			{
				Name: gptr.Of("field1"),
			},
		},
	}
	validSet := &entity.EvaluationSet{
		ID:      validEvalSetID,
		SpaceID: validSpaceID,
		BaseInfo: &entity.BaseInfo{
			CreatedBy: &entity.UserInfo{
				UserID: gptr.Of("user-123"),
			},
		},
	}

	tests := []struct {
		name        string
		req         *eval_set.UpdateEvaluationSetSchemaRequest
		mockSetup   func()
		wantResp    *eval_set.UpdateEvaluationSetSchemaResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal update",
			req: &eval_set.UpdateEvaluationSetSchemaRequest{
				WorkspaceID:     validSpaceID,
				EvaluationSetID: validEvalSetID,
				Fields:          validSchema.FieldSchemas,
			},
			mockSetup: func() {
				// Mock get evaluation set
				mockEvalSetService.EXPECT().
					GetEvaluationSet(gomock.Any(), &validSpaceID, validEvalSetID, gomock.Any()).
					Return(validSet, nil)

				// Mock auth
				mockAuth.EXPECT().
					AuthorizationWithoutSPI(gomock.Any(), &rpc.AuthorizationWithoutSPIParam{
						ObjectID:        strconv.FormatInt(validEvalSetID, 10),
						SpaceID:         validSpaceID,
						ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
						OwnerID:         gptr.Of("user-123"),
						ResourceSpaceID: validSpaceID,
					}).
					Return(nil)

				// Mock update schema
				mockEvalSetSchemaService.EXPECT().
					UpdateEvaluationSetSchema(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)
			},
			wantResp: &eval_set.UpdateEvaluationSetSchemaResponse{},
			wantErr:  false,
		},
		{
			name: "error - nil request",
			req:  nil,
			mockSetup: func() {
				// No mocks needed
			},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - evaluation set not found",
			req: &eval_set.UpdateEvaluationSetSchemaRequest{
				WorkspaceID:     validSpaceID,
				EvaluationSetID: validEvalSetID,
				Fields:          validSchema.FieldSchemas,
			},
			mockSetup: func() {
				mockEvalSetService.EXPECT().
					GetEvaluationSet(gomock.Any(), &validSpaceID, validEvalSetID, gomock.Any()).
					Return(nil, nil)
			},
			wantErr:     true,
			wantErrCode: errno.ResourceNotFoundCode,
		},
		{
			name: "error - auth failed",
			req: &eval_set.UpdateEvaluationSetSchemaRequest{
				WorkspaceID:     validSpaceID,
				EvaluationSetID: validEvalSetID,
				Fields:          validSchema.FieldSchemas,
			},
			mockSetup: func() {
				mockEvalSetService.EXPECT().
					GetEvaluationSet(gomock.Any(), &validSpaceID, validEvalSetID, gomock.Any()).
					Return(validSet, nil)

				mockAuth.EXPECT().
					AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
		{
			name: "error - update schema failed",
			req: &eval_set.UpdateEvaluationSetSchemaRequest{
				WorkspaceID:     validSpaceID,
				EvaluationSetID: validEvalSetID,
				Fields:          validSchema.FieldSchemas,
			},
			mockSetup: func() {
				mockEvalSetService.EXPECT().
					GetEvaluationSet(gomock.Any(), &validSpaceID, validEvalSetID, gomock.Any()).
					Return(validSet, nil)

				mockAuth.EXPECT().
					AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).
					Return(nil)

				mockEvalSetSchemaService.EXPECT().
					UpdateEvaluationSetSchema(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := app.UpdateEvaluationSetSchema(context.Background(), tt.req)

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

func TestEvaluationSetApplicationImpl_CreateEvaluationSetVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvalSetService := mocks.NewMockIEvaluationSetService(ctrl)
	mockEvalSetVersionService := mocks.NewMockEvaluationSetVersionService(ctrl)

	app := &EvaluationSetApplicationImpl{
		auth:                        mockAuth,
		evaluationSetService:        mockEvalSetService,
		evaluationSetVersionService: mockEvalSetVersionService,
	}

	// Test data
	validSpaceID := int64(123)
	validEvalSetID := int64(456)
	validVersion := &domain_eval_set.EvaluationSetVersion{
		Version:     gptr.Of("v1.0"),
		Description: gptr.Of("test version"),
	}
	validSet := &entity.EvaluationSet{
		ID:      validEvalSetID,
		SpaceID: validSpaceID,
		BaseInfo: &entity.BaseInfo{
			CreatedBy: &entity.UserInfo{
				UserID: gptr.Of("user-123"),
			},
		},
	}

	tests := []struct {
		name        string
		req         *eval_set.CreateEvaluationSetVersionRequest
		mockSetup   func()
		wantResp    *eval_set.CreateEvaluationSetVersionResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal create",
			req: &eval_set.CreateEvaluationSetVersionRequest{
				WorkspaceID:     validSpaceID,
				EvaluationSetID: validEvalSetID,
				Version:         validVersion.Version,
			},
			mockSetup: func() {
				// Mock get evaluation set
				mockEvalSetService.EXPECT().
					GetEvaluationSet(gomock.Any(), &validSpaceID, validEvalSetID, gomock.Any()).
					Return(validSet, nil)

				// Mock auth
				mockAuth.EXPECT().
					AuthorizationWithoutSPI(gomock.Any(), &rpc.AuthorizationWithoutSPIParam{
						ObjectID:        strconv.FormatInt(validEvalSetID, 10),
						SpaceID:         validSpaceID,
						ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
						OwnerID:         gptr.Of("user-123"),
						ResourceSpaceID: validSpaceID,
					}).
					Return(nil)

				// Mock create version
				mockEvalSetVersionService.EXPECT().
					CreateEvaluationSetVersion(gomock.Any(), gomock.Any()).
					Return(int64(789), nil)
			},
			wantResp: &eval_set.CreateEvaluationSetVersionResponse{
				ID: gptr.Of(int64(789)),
			},
			wantErr: false,
		},
		{
			name: "error - nil request",
			req:  nil,
			mockSetup: func() {
				// No mocks needed
			},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - evaluation set not found",
			req: &eval_set.CreateEvaluationSetVersionRequest{
				WorkspaceID:     validSpaceID,
				EvaluationSetID: validEvalSetID,
				Version:         validVersion.Version,
			},
			mockSetup: func() {
				mockEvalSetService.EXPECT().
					GetEvaluationSet(gomock.Any(), &validSpaceID, validEvalSetID, gomock.Any()).
					Return(nil, nil)
			},
			wantErr:     true,
			wantErrCode: errno.ResourceNotFoundCode,
		},
		{
			name: "error - auth failed",
			req: &eval_set.CreateEvaluationSetVersionRequest{
				WorkspaceID:     validSpaceID,
				EvaluationSetID: validEvalSetID,
				Version:         validVersion.Version,
			},
			mockSetup: func() {
				mockEvalSetService.EXPECT().
					GetEvaluationSet(gomock.Any(), &validSpaceID, validEvalSetID, gomock.Any()).
					Return(validSet, nil)

				mockAuth.EXPECT().
					AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
		{
			name: "error - create version failed",
			req: &eval_set.CreateEvaluationSetVersionRequest{
				WorkspaceID:     validSpaceID,
				EvaluationSetID: validEvalSetID,
				Version:         validVersion.Version,
			},
			mockSetup: func() {
				mockEvalSetService.EXPECT().
					GetEvaluationSet(gomock.Any(), &validSpaceID, validEvalSetID, gomock.Any()).
					Return(validSet, nil)

				mockAuth.EXPECT().
					AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).
					Return(nil)

				mockEvalSetVersionService.EXPECT().
					CreateEvaluationSetVersion(gomock.Any(), gomock.Any()).
					Return(int64(0), errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := app.CreateEvaluationSetVersion(context.Background(), tt.req)

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
