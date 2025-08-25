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

	idgenmock "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/base"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/tag"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	domain_eval_set "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_set"
	domain_eval_target "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_target"
	exptpb "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	componentMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/userinfo"
	userinfomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/userinfo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestExperimentApplication_CreateExperiment(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	// Create mock objects
	mockManager := servicemocks.NewMockIExptManager(ctrl)
	mockResultSvc := servicemocks.NewMockExptResultService(ctrl)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	// Test data
	validWorkspaceID := int64(123)
	validExptID := int64(456)
	validExpt := &entity.Experiment{
		ID:          validExptID,
		SpaceID:     validWorkspaceID,
		Name:        "test_experiment",
		Description: "test description",
		Status:      entity.ExptStatus_Pending,
	}

	tests := []struct {
		name      string
		req       *exptpb.CreateExperimentRequest
		mockSetup func()
		wantResp  *exptpb.CreateExperimentResponse
		wantErr   bool
		wantCode  int32
	}{
		{
			name: "successfully create experiment",
			req: &exptpb.CreateExperimentRequest{
				WorkspaceID: validWorkspaceID,
				Name:        gptr.Of("test_experiment"),
				Desc:        gptr.Of("test description"),
				CreateEvalTargetParam: &eval_target.CreateEvalTargetParam{
					EvalTargetType: gptr.Of(domain_eval_target.EvalTargetType_CozeBot),
				},
				Session: &common.Session{
					UserID: gptr.Of(int64(789)),
				},
				ItemConcurNum:       gptr.Of(int32(1)),
				EvaluatorsConcurNum: gptr.Of(int32(1)),
				TargetFieldMapping:  &expt.TargetFieldMapping{},
				EvaluatorFieldMapping: []*expt.EvaluatorFieldMapping{
					{},
				},
			},
			mockSetup: func() {
				mockManager.EXPECT().
					CreateExpt(gomock.Any(), gomock.Any(), &entity.Session{
						UserID: "789",
						AppID:  0,
					}).
					DoAndReturn(func(ctx context.Context, param *entity.CreateExptParam, session *entity.Session) (*entity.Experiment, error) {
						// Validate parameters
						if param.WorkspaceID != validWorkspaceID ||
							param.Name != "test_experiment" {
							t.Errorf("unexpected param: %+v", param)
						}
						return validExpt, nil
					})
			},
			wantResp: &exptpb.CreateExperimentResponse{
				Experiment: &expt.Experiment{
					ID:     gptr.Of(validExptID),
					Name:   gptr.Of("test_experiment"),
					Desc:   gptr.Of("test description"),
					Status: gptr.Of(expt.ExptStatus_Pending),
				},
				BaseResp: base.NewBaseResp(),
			},
			wantErr: false,
		},
		{
			name: "parameter validation failed - CreateEvalTargetParam is empty",
			req: &exptpb.CreateExperimentRequest{
				WorkspaceID: validWorkspaceID,
				Name:        gptr.Of("test_experiment"),
			},
			mockSetup: func() {
				// Mock will be called but should return an error
				mockManager.EXPECT().
					CreateExpt(gomock.Any(), gomock.Any(), &entity.Session{
						UserID: "",
						AppID:  0,
					}).
					Return(nil, fmt.Errorf("CreateEvalTargetParam is required"))
			},
			wantResp: nil,
			wantErr:  true,
			wantCode: 0, // Don't expect specific error code since it's a fmt.Errorf
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock behavior
			tt.mockSetup()

			// Create object under test
			app := &experimentApplication{
				manager:   mockManager,
				resultSvc: mockResultSvc,
				auth:      mockAuth,
			}

			// Execute test
			gotResp, err := app.CreateExperiment(context.Background(), tt.req)

			// Validate results
			// 验证结果
			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					if ok {
						assert.Equal(t, tt.wantCode, statusErr.Code())
					}
				}
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, gotResp)
			assert.Equal(t, tt.wantResp.Experiment.GetID(), gotResp.Experiment.GetID())
			assert.Equal(t, tt.wantResp.Experiment.GetName(), gotResp.Experiment.GetName())
			assert.Equal(t, tt.wantResp.Experiment.GetDesc(), gotResp.Experiment.GetDesc())
			assert.Equal(t, tt.wantResp.Experiment.GetStatus(), gotResp.Experiment.GetStatus())
		})
	}
}

func TestExperimentApplication_SubmitExperiment(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	// Create mock objects
	// 创建 mock 对象
	mockManager := servicemocks.NewMockIExptManager(ctrl)
	mockResultSvc := servicemocks.NewMockExptResultService(ctrl)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockScheduler := servicemocks.NewMockExptSchedulerEvent(ctrl)
	mockIDGen := idgenmock.NewMockIIDGenerator(ctrl)
	// Test data
	// 测试数据
	validWorkspaceID := int64(123)
	validExptID := int64(456)
	validRunID := int64(789)
	validExpt := &entity.Experiment{
		ID:          validExptID,
		SpaceID:     validWorkspaceID,
		Name:        "test_experiment",
		Description: "test description",
		Status:      entity.ExptStatus_Pending,
	}

	tests := []struct {
		name      string
		req       *exptpb.SubmitExperimentRequest
		mockSetup func()
		wantResp  *exptpb.SubmitExperimentResponse
		wantErr   bool
		wantCode  int32
	}{
		{
			name: "successfully submit experiment",
			req: &exptpb.SubmitExperimentRequest{
				WorkspaceID: validWorkspaceID,
				Name:        gptr.Of("test_experiment"),
				Desc:        gptr.Of("test description"),
				CreateEvalTargetParam: &eval_target.CreateEvalTargetParam{
					EvalTargetType: gptr.Of(domain_eval_target.EvalTargetType_CozeBot),
				},
				Session: &common.Session{
					UserID: gptr.Of(int64(789)),
				},
				ItemConcurNum:       gptr.Of(int32(1)),
				EvaluatorsConcurNum: gptr.Of(int32(1)),
				TargetFieldMapping:  &expt.TargetFieldMapping{},
				EvaluatorFieldMapping: []*expt.EvaluatorFieldMapping{
					{},
				},
			},
			mockSetup: func() {
				// Mock CreateExperiment call
				mockManager.EXPECT().
					CreateExpt(gomock.Any(), gomock.Any(), &entity.Session{
						UserID: "789",
						AppID:  0,
					}).
					DoAndReturn(func(ctx context.Context, param *entity.CreateExptParam, session *entity.Session) (*entity.Experiment, error) {
						if param.WorkspaceID != validWorkspaceID ||
							param.Name != "test_experiment" {
							t.Errorf("unexpected param: %+v", param)
						}
						return validExpt, nil
					})
				// Mock generate runID
				// 模拟生成 runID
				mockIDGen.EXPECT().
					GenID(gomock.Any()).
					Return(validRunID, nil)
				// Mock RunExperiment call
				// 模拟 RunExperiment 调用
				mockManager.EXPECT().
					LogRun(
						gomock.Any(),
						validExptID,
						validRunID,
						gomock.Any(),
						validWorkspaceID,
						&entity.Session{UserID: "789", AppID: 0},
					).Return(nil)

				mockManager.EXPECT().
					Run(
						gomock.Any(),
						validExptID,
						validRunID,
						validWorkspaceID,
						&entity.Session{UserID: "789", AppID: 0},
						gomock.Any(),
						gomock.Any(),
					).Return(nil)
				mockAuth.EXPECT().
					Authorization(
						gomock.Any(),
						gomock.Any(),
					).DoAndReturn(func(_ context.Context, param *rpc.AuthorizationParam) error {
					assert.Equal(t, strconv.FormatInt(validWorkspaceID, 10), param.ObjectID)
					assert.Equal(t, validWorkspaceID, param.SpaceID)
					assert.Equal(t, rpc.AuthEntityType_Space, *param.ActionObjects[0].EntityType)
					return nil
				}).AnyTimes()
			},
			wantResp: &exptpb.SubmitExperimentResponse{
				Experiment: &expt.Experiment{
					ID:     gptr.Of(validExptID),
					Name:   gptr.Of("test_experiment"),
					Desc:   gptr.Of("test description"),
					Status: gptr.Of(expt.ExptStatus_Pending),
				},
				RunID:    gptr.Of(validRunID),
				BaseResp: base.NewBaseResp(),
			},
			wantErr: false,
		},
		{
			name: "parameter validation failed - CreateEvalTargetParam is empty",
			req: &exptpb.SubmitExperimentRequest{
				WorkspaceID: validWorkspaceID,
				Name:        gptr.Of("test_experiment"),
			},
			mockSetup: func() {
				// Mock will be called but should return an error
				mockManager.EXPECT().
					CreateExpt(gomock.Any(), gomock.Any(), &entity.Session{
						UserID: "",
						AppID:  0,
					}).
					Return(nil, fmt.Errorf("CreateEvalTargetParam is required"))
			},
			wantResp: nil,
			wantErr:  true,
			wantCode: 0, // Don't expect specific error code since it's a fmt.Errorf
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { // Setup mock behavior
			tt.mockSetup()

			// Create object under test
			// 创建被测试对象
			app := &experimentApplication{
				manager:            mockManager,
				resultSvc:          mockResultSvc,
				auth:               mockAuth,
				ExptSchedulerEvent: mockScheduler,
				idgen:              mockIDGen,
			}
			// Execute test
			gotResp, err := app.SubmitExperiment(context.Background(), tt.req)

			// Validate results
			// 验证结果
			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					if ok {
						assert.Equal(t, tt.wantCode, statusErr.Code())
					}
				}
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, gotResp)
			assert.Equal(t, tt.wantResp.Experiment.GetID(), gotResp.Experiment.GetID())
			assert.Equal(t, tt.wantResp.Experiment.GetName(), gotResp.Experiment.GetName())
			assert.Equal(t, tt.wantResp.Experiment.GetDesc(), gotResp.Experiment.GetDesc())
			assert.Equal(t, tt.wantResp.Experiment.GetStatus(), gotResp.Experiment.GetStatus())
			assert.Equal(t, tt.wantResp.RunID, gotResp.RunID)
		})
	}
}

func TestExperimentApplication_CheckExperimentName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建 mock 对象
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockManager := servicemocks.NewMockIExptManager(ctrl)

	// 测试数据
	validWorkspaceID := int64(123)
	validName := "test_experiment"

	tests := []struct {
		name      string
		req       *exptpb.CheckExperimentNameRequest
		mockSetup func()
		wantResp  *exptpb.CheckExperimentNameResponse
		wantErr   bool
	}{
		{
			name: "experiment name available",
			req: &exptpb.CheckExperimentNameRequest{
				WorkspaceID: validWorkspaceID,
				Name:        gptr.Of(validName),
			},
			mockSetup: func() {
				mockManager.EXPECT().
					CheckName(gomock.Any(), validName, validWorkspaceID, &entity.Session{}).
					Return(true, nil)
				mockAuth.EXPECT().
					Authorization(
						gomock.Any(),
						gomock.Any(),
					).DoAndReturn(func(_ context.Context, param *rpc.AuthorizationParam) error {
					assert.Equal(t, strconv.FormatInt(validWorkspaceID, 10), param.ObjectID)
					assert.Equal(t, validWorkspaceID, param.SpaceID)
					assert.Equal(t, rpc.AuthEntityType_Space, *param.ActionObjects[0].EntityType)
					return nil
				})
			},
			wantResp: &exptpb.CheckExperimentNameResponse{
				Pass:    gptr.Of(true),
				Message: gptr.Of(""),
			},
			wantErr: false,
		},
		{
			name: "experiment name already exists",
			req: &exptpb.CheckExperimentNameRequest{
				WorkspaceID: validWorkspaceID,
				Name:        gptr.Of(validName),
			},
			mockSetup: func() {
				mockManager.EXPECT().
					CheckName(gomock.Any(), validName, validWorkspaceID, &entity.Session{}).
					Return(false, nil)
				mockAuth.EXPECT().
					Authorization(
						gomock.Any(),
						gomock.Any(),
					).DoAndReturn(func(_ context.Context, param *rpc.AuthorizationParam) error {
					assert.Equal(t, strconv.FormatInt(validWorkspaceID, 10), param.ObjectID)
					assert.Equal(t, validWorkspaceID, param.SpaceID)
					assert.Equal(t, rpc.AuthEntityType_Space, *param.ActionObjects[0].EntityType)
					return nil
				})
			},
			wantResp: &exptpb.CheckExperimentNameResponse{
				Pass:    gptr.Of(false),
				Message: gptr.Of("experiment name test_experiment already exist"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置 mock 行为
			tt.mockSetup()

			// 创建被测试对象
			app := &experimentApplication{
				manager: mockManager,
				auth:    mockAuth,
			}

			// 执行测试
			gotResp, err := app.CheckExperimentName(context.Background(), tt.req)

			// 验证结果
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, gotResp)
			assert.Equal(t, tt.wantResp.GetPass(), gotResp.GetPass())
			assert.Equal(t, tt.wantResp.GetMessage(), gotResp.GetMessage())
		})
	}
}

func TestExperimentApplication_BatchGetExperiments(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建 mock 对象
	mockManager := servicemocks.NewMockIExptManager(ctrl)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockUserInfoService := userinfomocks.NewMockUserInfoService(ctrl)

	// 测试数据
	validWorkspaceID := int64(123)
	validExptIDs := []int64{456, 457}
	validExpts := []*entity.Experiment{
		{
			ID:          validExptIDs[0],
			SpaceID:     validWorkspaceID,
			Name:        "test_experiment_1",
			Description: "test description 1",
			Status:      entity.ExptStatus_Pending,
			CreatedBy:   "789",
		},
		{
			ID:          validExptIDs[1],
			SpaceID:     validWorkspaceID,
			Name:        "test_experiment_2",
			Description: "test description 2",
			Status:      entity.ExptStatus_Processing,
			CreatedBy:   "789",
		},
	}

	tests := []struct {
		name      string
		req       *exptpb.BatchGetExperimentsRequest
		mockSetup func()
		wantResp  *exptpb.BatchGetExperimentsResponse
		wantErr   bool
	}{
		{
			name: "successfully batch get experiments",
			req: &exptpb.BatchGetExperimentsRequest{
				WorkspaceID: validWorkspaceID,
				ExptIds:     validExptIDs,
			},
			mockSetup: func() {
				// 模拟获取实验详情
				mockManager.EXPECT().
					MGetDetail(gomock.Any(), validExptIDs, validWorkspaceID, &entity.Session{}).
					Return(validExpts, nil)

				// 模拟权限验证
				mockAuth.EXPECT().
					MAuthorizeWithoutSPI(
						gomock.Any(),
						validWorkspaceID,
						gomock.Any(),
					).DoAndReturn(func(_ context.Context, spaceID int64, params []*rpc.AuthorizationWithoutSPIParam) error {
					assert.Equal(t, len(validExpts), len(params))
					for i, param := range params {
						assert.Equal(t, strconv.FormatInt(validExpts[i].ID, 10), param.ObjectID)
						assert.Equal(t, validWorkspaceID, param.SpaceID)
						assert.Equal(t, validWorkspaceID, param.ResourceSpaceID)
						assert.Equal(t, validExpts[i].CreatedBy, *param.OwnerID)
						assert.Equal(t, 1, len(param.ActionObjects))
						assert.Equal(t, "read", *param.ActionObjects[0].Action)
						assert.Equal(t, rpc.AuthEntityType_EvaluationExperiment, *param.ActionObjects[0].EntityType)
					}
					return nil
				})

				// 模拟填充用户信息
				mockUserInfoService.EXPECT().
					PackUserInfo(gomock.Any(), gomock.Any()).
					Do(func(_ context.Context, carriers []userinfo.UserInfoCarrier) {
						assert.Equal(t, len(validExpts), len(carriers))
					})
			},
			wantResp: &exptpb.BatchGetExperimentsResponse{
				Experiments: []*expt.Experiment{
					{
						ID:     gptr.Of(validExptIDs[0]),
						Name:   gptr.Of("test_experiment_1"),
						Desc:   gptr.Of("test description 1"),
						Status: gptr.Of(expt.ExptStatus_Pending),
						BaseInfo: &common.BaseInfo{
							CreatedBy: &common.UserInfo{
								UserID: gptr.Of("789"),
							},
						},
					},
					{
						ID:     gptr.Of(validExptIDs[1]),
						Name:   gptr.Of("test_experiment_2"),
						Desc:   gptr.Of("test description 2"),
						Status: gptr.Of(expt.ExptStatus_Processing),
						BaseInfo: &common.BaseInfo{
							CreatedBy: &common.UserInfo{
								UserID: gptr.Of("789"),
							},
						},
					},
				},
				BaseResp: base.NewBaseResp(),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置 mock 行为
			tt.mockSetup()

			// 创建被测试对象
			app := &experimentApplication{
				manager:         mockManager,
				auth:            mockAuth,
				userInfoService: mockUserInfoService,
			}

			// 执行测试
			gotResp, err := app.BatchGetExperiments(context.Background(), tt.req)

			// 验证结果
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, gotResp)
			assert.Equal(t, len(tt.wantResp.Experiments), len(gotResp.Experiments))

			for i, wantExpt := range tt.wantResp.Experiments {
				gotExpt := gotResp.Experiments[i]
				assert.Equal(t, wantExpt.GetID(), gotExpt.GetID())
				assert.Equal(t, wantExpt.GetName(), gotExpt.GetName())
				assert.Equal(t, wantExpt.GetDesc(), gotExpt.GetDesc())
				assert.Equal(t, wantExpt.GetStatus(), gotExpt.GetStatus())
				assert.Equal(t, wantExpt.GetBaseInfo().GetCreatedBy().GetUserID(),
					gotExpt.GetBaseInfo().GetCreatedBy().GetUserID())
			}
		})
	}
}

func TestExperimentApplication_ListExperiments(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建 mock 对象
	mockManager := servicemocks.NewMockIExptManager(ctrl)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockUserInfoService := userinfomocks.NewMockUserInfoService(ctrl)
	mockEvalTargetService := servicemocks.NewMockIEvalTargetService(ctrl)

	// 测试数据
	validWorkspaceID := int64(123)
	validExpts := []*entity.Experiment{
		{
			ID:          456,
			SpaceID:     validWorkspaceID,
			Name:        "test_experiment_1",
			Description: "test description 1",
			Status:      entity.ExptStatus_Pending,
			CreatedBy:   "789",
		},
		{
			ID:          457,
			SpaceID:     validWorkspaceID,
			Name:        "test_experiment_2",
			Description: "test description 2",
			Status:      entity.ExptStatus_Processing,
			CreatedBy:   "789",
		},
	}

	tests := []struct {
		name      string
		req       *exptpb.ListExperimentsRequest
		mockSetup func()
		wantResp  *exptpb.ListExperimentsResponse
		wantErr   bool
	}{
		{
			name: "successfully list experiments",
			req: &exptpb.ListExperimentsRequest{
				WorkspaceID:  validWorkspaceID,
				PageNumber:   gptr.Of(int32(1)),
				PageSize:     gptr.Of(int32(10)),
				FilterOption: &expt.ExptFilterOption{},
				OrderBys: []*common.OrderBy{
					{
						Field: gptr.Of("created_at"),
						IsAsc: gptr.Of(false),
					},
				},
			},
			mockSetup: func() {
				// 模拟权限验证
				mockAuth.EXPECT().
					Authorization(
						gomock.Any(),
						gomock.Any(),
					).DoAndReturn(func(_ context.Context, param *rpc.AuthorizationParam) error {
					assert.Equal(t, strconv.FormatInt(validWorkspaceID, 10), param.ObjectID)
					assert.Equal(t, validWorkspaceID, param.SpaceID)
					assert.Equal(t, 1, len(param.ActionObjects))
					assert.Equal(t, "listLoopEvaluationExperiment", *param.ActionObjects[0].Action)
					assert.Equal(t, rpc.AuthEntityType_Space, *param.ActionObjects[0].EntityType)
					return nil
				})

				// 模拟列表查询
				mockManager.EXPECT().
					List(
						gomock.Any(),
						gomock.Any(),
						gomock.Any(),
						validWorkspaceID,
						gomock.Any(),
						[]*entity.OrderBy{{Field: gptr.Of("created_at"), IsAsc: gptr.Of(false)}},
						&entity.Session{},
					).DoAndReturn(func(_ context.Context, pageNumber, pageSize int32, spaceID int64, filter *entity.ExptListFilter, orderBys []*entity.OrderBy, session *entity.Session) ([]*entity.Experiment, int64, error) {
					assert.Equal(t, int32(1), pageNumber)
					assert.Equal(t, int32(10), pageSize)
					return validExpts, int64(len(validExpts)), nil
				})

				// 模拟填充用户信息
				mockUserInfoService.EXPECT().
					PackUserInfo(gomock.Any(), gomock.Any()).
					Do(func(_ context.Context, carriers []userinfo.UserInfoCarrier) {
						assert.Equal(t, len(validExpts), len(carriers))
					}).AnyTimes()
			},
			wantResp: &exptpb.ListExperimentsResponse{
				Experiments: []*expt.Experiment{
					{
						ID:     gptr.Of(int64(456)),
						Name:   gptr.Of("test_experiment_1"),
						Desc:   gptr.Of("test description 1"),
						Status: gptr.Of(expt.ExptStatus_Pending),
						BaseInfo: &common.BaseInfo{
							CreatedBy: &common.UserInfo{
								UserID: gptr.Of("789"),
							},
						},
					},
					{
						ID:     gptr.Of(int64(457)),
						Name:   gptr.Of("test_experiment_2"),
						Desc:   gptr.Of("test description 2"),
						Status: gptr.Of(expt.ExptStatus_Processing),
						BaseInfo: &common.BaseInfo{
							CreatedBy: &common.UserInfo{
								UserID: gptr.Of("789"),
							},
						},
					},
				},
				Total:    gptr.Of(int32(2)),
				BaseResp: base.NewBaseResp(),
			},
			wantErr: false,
		},
		{
			name: "permission validation failed",
			req: &exptpb.ListExperimentsRequest{
				WorkspaceID: validWorkspaceID,
				PageNumber:  gptr.Of(int32(1)),
				PageSize:    gptr.Of(int32(10)),
			},
			mockSetup: func() {
				// 模拟权限验证失败
				mockAuth.EXPECT().
					Authorization(
						gomock.Any(),
						gomock.Any(),
					).DoAndReturn(func(_ context.Context, param *rpc.AuthorizationParam) error {
					assert.Equal(t, strconv.FormatInt(validWorkspaceID, 10), param.ObjectID)
					assert.Equal(t, validWorkspaceID, param.SpaceID)
					assert.Equal(t, 1, len(param.ActionObjects))
					assert.Equal(t, "listLoopEvaluationExperiment", *param.ActionObjects[0].Action)
					assert.Equal(t, rpc.AuthEntityType_Space, *param.ActionObjects[0].EntityType)
					return errorx.NewByCode(errno.CommonNoPermissionCode)
				})
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建被测试对象
			app := &experimentApplication{
				manager:           mockManager,
				auth:              mockAuth,
				userInfoService:   mockUserInfoService,
				evalTargetService: mockEvalTargetService,
			}

			// Setup mock behavior
			tt.mockSetup()

			// Execute test
			gotResp, err := app.ListExperiments(context.Background(), tt.req)

			// 验证结果
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, gotResp)
			assert.Equal(t, len(tt.wantResp.Experiments), len(gotResp.Experiments))
			assert.Equal(t, tt.wantResp.GetTotal(), gotResp.GetTotal())

			for i, wantExpt := range tt.wantResp.Experiments {
				gotExpt := gotResp.Experiments[i]
				assert.Equal(t, wantExpt.GetID(), gotExpt.GetID())
				assert.Equal(t, wantExpt.GetName(), gotExpt.GetName())
				assert.Equal(t, wantExpt.GetDesc(), gotExpt.GetDesc())
				assert.Equal(t, wantExpt.GetStatus(), gotExpt.GetStatus())
				assert.Equal(t, wantExpt.GetBaseInfo().GetCreatedBy().GetUserID(),
					gotExpt.GetBaseInfo().GetCreatedBy().GetUserID())
			}
		})
	}
}

func TestExperimentApplication_UpdateExperiment(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建 mock 对象
	mockManager := servicemocks.NewMockIExptManager(ctrl)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockUserInfoService := userinfomocks.NewMockUserInfoService(ctrl)

	// 测试数据
	validWorkspaceID := int64(123)
	validExptID := int64(456)
	validUserID := "789"
	validExpt := &entity.Experiment{
		ID:          validExptID,
		SpaceID:     validWorkspaceID,
		Name:        "test_experiment_1",
		Description: "test description 1",
		Status:      entity.ExptStatus_Pending,
		CreatedBy:   validUserID,
	}

	tests := []struct {
		name      string
		req       *exptpb.UpdateExperimentRequest
		mockSetup func()
		wantResp  *exptpb.UpdateExperimentResponse
		wantErr   bool
	}{
		{
			name: "successfully update experiment",
			req: &exptpb.UpdateExperimentRequest{
				ExptID:      validExptID,
				WorkspaceID: validWorkspaceID,
				Name:        gptr.Of("updated_experiment"),
				Desc:        gptr.Of("updated description"),
			},
			mockSetup: func() {
				// 模拟获取实验
				mockManager.EXPECT().
					Get(gomock.Any(), validExptID, validWorkspaceID, &entity.Session{}).
					Return(validExpt, nil)

				// 模拟检查名称
				mockManager.EXPECT().
					CheckName(gomock.Any(), "updated_experiment", validWorkspaceID, &entity.Session{}).
					Return(true, nil)

				// 模拟权限验证
				mockAuth.EXPECT().
					AuthorizationWithoutSPI(
						gomock.Any(),
						gomock.Any(),
					).DoAndReturn(func(_ context.Context, param *rpc.AuthorizationWithoutSPIParam) error {
					assert.Equal(t, strconv.FormatInt(validExptID, 10), param.ObjectID)
					assert.Equal(t, validWorkspaceID, param.SpaceID)
					assert.Equal(t, validWorkspaceID, param.ResourceSpaceID)
					assert.Equal(t, validUserID, *param.OwnerID)
					assert.Equal(t, 1, len(param.ActionObjects))
					assert.Equal(t, "edit", *param.ActionObjects[0].Action)
					assert.Equal(t, rpc.AuthEntityType_EvaluationExperiment, *param.ActionObjects[0].EntityType)
					return nil
				})

				// 模拟更新实验
				mockManager.EXPECT().
					Update(
						gomock.Any(),
						&entity.Experiment{
							ID:          validExptID,
							SpaceID:     validWorkspaceID,
							Name:        "updated_experiment",
							Description: "updated description",
						},
						&entity.Session{},
					).Return(nil)

				// 模拟获取更新后的实验
				updatedExpt := &entity.Experiment{
					ID:          validExptID,
					SpaceID:     validWorkspaceID,
					Name:        "updated_experiment",
					Description: "updated description",
					Status:      entity.ExptStatus_Pending,
					CreatedBy:   validUserID,
				}
				mockManager.EXPECT().
					Get(gomock.Any(), validExptID, validWorkspaceID, &entity.Session{}).
					Return(updatedExpt, nil)

				// 模拟填充用户信息
				mockUserInfoService.EXPECT().
					PackUserInfo(gomock.Any(), gomock.Any()).
					Do(func(_ context.Context, carriers []userinfo.UserInfoCarrier) {
						assert.Equal(t, 1, len(carriers))
					}).AnyTimes()
			},
			wantResp: &exptpb.UpdateExperimentResponse{
				Experiment: &expt.Experiment{
					ID:        gptr.Of(validExptID),
					Name:      gptr.Of("updated_experiment"),
					Desc:      gptr.Of("updated description"),
					Status:    gptr.Of(expt.ExptStatus_Pending),
					CreatorBy: gptr.Of(validUserID),
					BaseInfo: &common.BaseInfo{
						CreatedBy: &common.UserInfo{
							UserID: gptr.Of(validUserID),
						},
					},
				},
				BaseResp: base.NewBaseResp(),
			},
			wantErr: false,
		},
		{
			name: "experiment name already exists",
			req: &exptpb.UpdateExperimentRequest{
				ExptID:      validExptID,
				WorkspaceID: validWorkspaceID,
				Name:        gptr.Of("existing_experiment"),
				Desc:        gptr.Of("updated description"),
			},
			mockSetup: func() {
				// 模拟获取实验
				mockManager.EXPECT().
					Get(gomock.Any(), validExptID, validWorkspaceID, &entity.Session{}).
					Return(validExpt, nil)

				// 模拟检查名称失败
				mockManager.EXPECT().
					CheckName(gomock.Any(), "existing_experiment", validWorkspaceID, &entity.Session{}).
					Return(false, nil)
			},
			wantErr: true,
		},
		{
			name: "permission validation failed",
			req: &exptpb.UpdateExperimentRequest{
				ExptID:      validExptID,
				WorkspaceID: validWorkspaceID,
				Name:        gptr.Of("updated_experiment"),
				Desc:        gptr.Of("updated description"),
			},
			mockSetup: func() {
				// 模拟获取实验
				mockManager.EXPECT().
					Get(gomock.Any(), validExptID, validWorkspaceID, &entity.Session{}).
					Return(validExpt, nil)

				// 模拟检查名称
				mockManager.EXPECT().
					CheckName(gomock.Any(), "updated_experiment", validWorkspaceID, &entity.Session{}).
					Return(true, nil)

				// 模拟权限验证失败
				mockAuth.EXPECT().
					AuthorizationWithoutSPI(
						gomock.Any(),
						gomock.Any(),
					).DoAndReturn(func(_ context.Context, param *rpc.AuthorizationWithoutSPIParam) error {
					assert.Equal(t, strconv.FormatInt(validExptID, 10), param.ObjectID)
					assert.Equal(t, validWorkspaceID, param.SpaceID)
					assert.Equal(t, validWorkspaceID, param.ResourceSpaceID)
					assert.Equal(t, validUserID, *param.OwnerID)
					assert.Equal(t, 1, len(param.ActionObjects))
					assert.Equal(t, "edit", *param.ActionObjects[0].Action)
					assert.Equal(t, rpc.AuthEntityType_EvaluationExperiment, *param.ActionObjects[0].EntityType)
					return errorx.NewByCode(errno.CommonNoPermissionCode)
				})
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建被测试对象
			app := &experimentApplication{
				manager:         mockManager,
				auth:            mockAuth,
				userInfoService: mockUserInfoService,
			}

			// 设置 mock 行为
			tt.mockSetup()

			// 执行测试
			gotResp, err := app.UpdateExperiment(context.Background(), tt.req)

			// 验证结果
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, gotResp)
			assert.Equal(t, tt.wantResp.GetExperiment().GetID(), gotResp.GetExperiment().GetID())
			assert.Equal(t, tt.wantResp.GetExperiment().GetName(), gotResp.GetExperiment().GetName())
			assert.Equal(t, tt.wantResp.GetExperiment().GetDesc(), gotResp.GetExperiment().GetDesc())
			assert.Equal(t, tt.wantResp.GetExperiment().GetStatus(), gotResp.GetExperiment().GetStatus())
		})
	}
}

func TestExperimentApplication_DeleteExperiment(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock objects
	mockManager := servicemocks.NewMockIExptManager(ctrl)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)

	// Test data
	validWorkspaceID := int64(123)
	validExptID := int64(456)
	validUserID := "789"
	validExpt := &entity.Experiment{
		ID:          validExptID,
		SpaceID:     validWorkspaceID,
		Name:        "test_experiment_1",
		Description: "test description 1",
		Status:      entity.ExptStatus_Pending,
		CreatedBy:   validUserID,
	}

	tests := []struct {
		name      string
		req       *exptpb.DeleteExperimentRequest
		mockSetup func()
		wantResp  *exptpb.DeleteExperimentResponse
		wantErr   bool
	}{
		{
			name: "successfully delete experiment",
			req: &exptpb.DeleteExperimentRequest{
				ExptID:      validExptID,
				WorkspaceID: validWorkspaceID,
			},
			mockSetup: func() {
				// 模拟获取实验
				mockManager.EXPECT().
					Get(gomock.Any(), validExptID, validWorkspaceID, &entity.Session{}).
					Return(validExpt, nil)

				// 模拟权限验证
				mockAuth.EXPECT().
					AuthorizationWithoutSPI(
						gomock.Any(),
						gomock.Any(),
					).DoAndReturn(func(_ context.Context, param *rpc.AuthorizationWithoutSPIParam) error {
					assert.Equal(t, strconv.FormatInt(validExptID, 10), param.ObjectID)
					assert.Equal(t, validWorkspaceID, param.SpaceID)
					assert.Equal(t, validWorkspaceID, param.ResourceSpaceID)
					assert.Equal(t, validUserID, *param.OwnerID)
					assert.Equal(t, 1, len(param.ActionObjects))
					assert.Equal(t, "edit", *param.ActionObjects[0].Action)
					assert.Equal(t, rpc.AuthEntityType_EvaluationExperiment, *param.ActionObjects[0].EntityType)
					return nil
				})

				// 模拟删除实验
				mockManager.EXPECT().
					Delete(gomock.Any(), validExptID, validWorkspaceID, &entity.Session{}).
					Return(nil)
			},
			wantResp: &exptpb.DeleteExperimentResponse{
				BaseResp: base.NewBaseResp(),
			},
			wantErr: false,
		},
		{
			name: "experiment does not exist",
			req: &exptpb.DeleteExperimentRequest{
				ExptID:      validExptID,
				WorkspaceID: validWorkspaceID,
			},
			mockSetup: func() {
				// 模拟获取实验失败
				mockManager.EXPECT().
					Get(gomock.Any(), validExptID, validWorkspaceID, &entity.Session{}).
					Return(nil, errorx.NewByCode(errno.ResourceNotFoundCode))
			},
			wantErr: true,
		},
		{
			name: "permission validation failed",
			req: &exptpb.DeleteExperimentRequest{
				ExptID:      validExptID,
				WorkspaceID: validWorkspaceID,
			},
			mockSetup: func() {
				// 模拟获取实验
				mockManager.EXPECT().
					Get(gomock.Any(), validExptID, validWorkspaceID, &entity.Session{}).
					Return(validExpt, nil)

				// 模拟权限验证失败
				mockAuth.EXPECT().
					AuthorizationWithoutSPI(
						gomock.Any(),
						gomock.Any(),
					).DoAndReturn(func(_ context.Context, param *rpc.AuthorizationWithoutSPIParam) error {
					assert.Equal(t, strconv.FormatInt(validExptID, 10), param.ObjectID)
					assert.Equal(t, validWorkspaceID, param.SpaceID)
					assert.Equal(t, validWorkspaceID, param.ResourceSpaceID)
					assert.Equal(t, validUserID, *param.OwnerID)
					assert.Equal(t, 1, len(param.ActionObjects))
					assert.Equal(t, "edit", *param.ActionObjects[0].Action)
					assert.Equal(t, rpc.AuthEntityType_EvaluationExperiment, *param.ActionObjects[0].EntityType)
					return errorx.NewByCode(errno.CommonNoPermissionCode)
				})
			},
			wantErr: true,
		},
		{
			name: "delete operation failed",
			req: &exptpb.DeleteExperimentRequest{
				ExptID:      validExptID,
				WorkspaceID: validWorkspaceID,
			},
			mockSetup: func() {
				// 模拟获取实验
				mockManager.EXPECT().
					Get(gomock.Any(), validExptID, validWorkspaceID, &entity.Session{}).
					Return(validExpt, nil)

				// 模拟权限验证
				mockAuth.EXPECT().
					AuthorizationWithoutSPI(
						gomock.Any(),
						gomock.Any(),
					).Return(nil)

				// 模拟删除实验失败
				mockManager.EXPECT().
					Delete(gomock.Any(), validExptID, validWorkspaceID, &entity.Session{}).
					Return(errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建被测试对象
			app := &experimentApplication{
				manager: mockManager,
				auth:    mockAuth,
			}

			// 设置 mock 行为
			tt.mockSetup()

			// 执行测试
			gotResp, err := app.DeleteExperiment(context.Background(), tt.req)

			// 验证结果
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, gotResp)
			assert.NotNil(t, gotResp.GetBaseResp())
		})
	}
}

func TestExperimentApplication_BatchDeleteExperiments(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock objects
	mockManager := servicemocks.NewMockIExptManager(ctrl)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)

	// Test data
	validWorkspaceID := int64(123)
	validExptID1 := int64(456)
	validExptID2 := int64(457)
	validUserID := "789"
	validExpt1 := &entity.Experiment{
		ID:          validExptID1,
		SpaceID:     validWorkspaceID,
		Name:        "test_experiment_1",
		Description: "test description 1",
		Status:      entity.ExptStatus_Pending,
		CreatedBy:   validUserID,
	}
	validExpt2 := &entity.Experiment{
		ID:          validExptID2,
		SpaceID:     validWorkspaceID,
		Name:        "test_experiment_2",
		Description: "test description 2",
		Status:      entity.ExptStatus_Pending,
		CreatedBy:   validUserID,
	}

	tests := []struct {
		name      string
		req       *exptpb.BatchDeleteExperimentsRequest
		mockSetup func()
		wantResp  *exptpb.BatchDeleteExperimentsResponse
		wantErr   bool
	}{
		{
			name: "successfully batch delete experiments",
			req: &exptpb.BatchDeleteExperimentsRequest{
				ExptIds:     []int64{validExptID1, validExptID2},
				WorkspaceID: validWorkspaceID,
			},
			mockSetup: func() {
				// 模拟获取实验列表
				mockManager.EXPECT().
					MGet(gomock.Any(), []int64{validExptID1, validExptID2}, validWorkspaceID, &entity.Session{}).
					Return([]*entity.Experiment{validExpt1, validExpt2}, nil)

				// 模拟批量权限验证
				mockAuth.EXPECT().
					MAuthorizeWithoutSPI(
						gomock.Any(),
						validWorkspaceID,
						gomock.Any(),
					).DoAndReturn(func(_ context.Context, spaceID int64, params []*rpc.AuthorizationWithoutSPIParam) error {
					assert.Equal(t, 2, len(params))
					for i, param := range params {
						exptID := []int64{validExptID1, validExptID2}[i]
						assert.Equal(t, strconv.FormatInt(exptID, 10), param.ObjectID)
						assert.Equal(t, validWorkspaceID, param.SpaceID)
						assert.Equal(t, validWorkspaceID, param.ResourceSpaceID)
						assert.Equal(t, validUserID, *param.OwnerID)
						assert.Equal(t, 1, len(param.ActionObjects))
						assert.Equal(t, "edit", *param.ActionObjects[0].Action)
						assert.Equal(t, rpc.AuthEntityType_EvaluationExperiment, *param.ActionObjects[0].EntityType)
					}
					return nil
				})

				// 模拟批量删除实验
				mockManager.EXPECT().
					MDelete(gomock.Any(), []int64{validExptID1, validExptID2}, validWorkspaceID, &entity.Session{}).
					Return(nil)
			},
			wantResp: &exptpb.BatchDeleteExperimentsResponse{
				BaseResp: base.NewBaseResp(),
			},
			wantErr: false,
		},
		{
			name: "some experiments do not exist",
			req: &exptpb.BatchDeleteExperimentsRequest{
				ExptIds:     []int64{validExptID1, validExptID2},
				WorkspaceID: validWorkspaceID,
			},
			mockSetup: func() {
				// 模拟获取实验列表，只返回一个实验
				mockManager.EXPECT().
					MGet(gomock.Any(), []int64{validExptID1, validExptID2}, validWorkspaceID, &entity.Session{}).
					Return([]*entity.Experiment{validExpt1}, nil)

				// 模拟批量权限验证
				mockAuth.EXPECT().
					MAuthorizeWithoutSPI(
						gomock.Any(),
						validWorkspaceID,
						gomock.Any(),
					).DoAndReturn(func(_ context.Context, spaceID int64, params []*rpc.AuthorizationWithoutSPIParam) error {
					assert.Equal(t, 1, len(params))
					assert.Equal(t, strconv.FormatInt(validExptID1, 10), params[0].ObjectID)
					return nil
				})

				// 模拟批量删除实验
				mockManager.EXPECT().
					MDelete(gomock.Any(), []int64{validExptID1, validExptID2}, validWorkspaceID, &entity.Session{}).
					Return(nil)
			},
			wantResp: &exptpb.BatchDeleteExperimentsResponse{
				BaseResp: base.NewBaseResp(),
			},
			wantErr: false,
		},
		{
			name: "permission validation failed",
			req: &exptpb.BatchDeleteExperimentsRequest{
				ExptIds:     []int64{validExptID1, validExptID2},
				WorkspaceID: validWorkspaceID,
			},
			mockSetup: func() {
				// 模拟获取实验列表
				mockManager.EXPECT().
					MGet(gomock.Any(), []int64{validExptID1, validExptID2}, validWorkspaceID, &entity.Session{}).
					Return([]*entity.Experiment{validExpt1, validExpt2}, nil)

				// 模拟批量权限验证失败
				mockAuth.EXPECT().
					MAuthorizeWithoutSPI(
						gomock.Any(),
						validWorkspaceID,
						gomock.Any(),
					).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr: true,
		},
		{
			name: "batch delete operation failed",
			req: &exptpb.BatchDeleteExperimentsRequest{
				ExptIds:     []int64{validExptID1, validExptID2},
				WorkspaceID: validWorkspaceID,
			},
			mockSetup: func() {
				// 模拟获取实验列表
				mockManager.EXPECT().
					MGet(gomock.Any(), []int64{validExptID1, validExptID2}, validWorkspaceID, &entity.Session{}).
					Return([]*entity.Experiment{validExpt1, validExpt2}, nil)

				// 模拟批量权限验证
				mockAuth.EXPECT().
					MAuthorizeWithoutSPI(
						gomock.Any(),
						validWorkspaceID,
						gomock.Any(),
					).Return(nil)

				// 模拟批量删除实验失败
				mockManager.EXPECT().
					MDelete(gomock.Any(), []int64{validExptID1, validExptID2}, validWorkspaceID, &entity.Session{}).
					Return(errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建被测试对象
			app := &experimentApplication{
				manager: mockManager,
				auth:    mockAuth,
			}

			// 设置 mock 行为
			tt.mockSetup()

			// 执行测试
			gotResp, err := app.BatchDeleteExperiments(context.Background(), tt.req)

			// 验证结果
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, gotResp)
			assert.NotNil(t, gotResp.GetBaseResp())
		})
	}
}

func TestExperimentApplication_CloneExperiment(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建 mock 对象
	mockManager := servicemocks.NewMockIExptManager(ctrl)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockIDGen := idgenmock.NewMockIIDGenerator(ctrl)
	mockResultSvc := servicemocks.NewMockExptResultService(ctrl)
	mockUserInfoService := userinfomocks.NewMockUserInfoService(ctrl)

	// 测试数据
	validWorkspaceID := int64(123)
	validExptID := int64(456)
	validUserID := "789"
	newExptID := int64(789)
	newStatsID := int64(999)
	clonedExpt := &entity.Experiment{
		ID:          newExptID,
		SpaceID:     validWorkspaceID,
		Name:        "test_experiment_1_copy",
		Description: "test description 1",
		Status:      entity.ExptStatus_Pending,
		CreatedBy:   validUserID,
	}

	tests := []struct {
		name      string
		req       *exptpb.CloneExperimentRequest
		mockSetup func()
		wantResp  *exptpb.CloneExperimentResponse
		wantErr   bool
	}{
		{
			name: "successfully clone experiment",
			req: &exptpb.CloneExperimentRequest{
				ExptID:      gptr.Of(validExptID),
				WorkspaceID: gptr.Of(validWorkspaceID),
			},
			mockSetup: func() {
				// 模拟权限验证
				mockAuth.EXPECT().
					Authorization(
						gomock.Any(),
						gomock.Any(),
					).DoAndReturn(func(_ context.Context, param *rpc.AuthorizationParam) error {
					assert.Equal(t, strconv.FormatInt(validExptID, 10), param.ObjectID)
					assert.Equal(t, validWorkspaceID, param.SpaceID)
					assert.Equal(t, 1, len(param.ActionObjects))
					assert.Equal(t, consts.ActionCreateExpt, *param.ActionObjects[0].Action)
					assert.Equal(t, rpc.AuthEntityType_Space, *param.ActionObjects[0].EntityType)
					return nil
				})

				// 模拟克隆实验
				mockManager.EXPECT().
					Clone(gomock.Any(), validExptID, validWorkspaceID, &entity.Session{}).
					Return(clonedExpt, nil)

				// 模拟生成统计信息ID
				mockIDGen.EXPECT().
					GenID(gomock.Any()).
					Return(newStatsID, nil)

				// 模拟创建统计信息
				mockResultSvc.EXPECT().
					CreateStats(
						gomock.Any(),
						&entity.ExptStats{
							ID:      newStatsID,
							SpaceID: validWorkspaceID,
							ExptID:  newExptID,
						},
						&entity.Session{},
					).Return(nil)

				// 模拟填充用户信息
				mockUserInfoService.EXPECT().
					PackUserInfo(gomock.Any(), gomock.Any()).
					Do(func(_ context.Context, carriers []userinfo.UserInfoCarrier) {
						assert.Equal(t, 1, len(carriers))
					}).AnyTimes()
			},
			wantResp: &exptpb.CloneExperimentResponse{
				Experiment: &expt.Experiment{
					ID:        gptr.Of(newExptID),
					Name:      gptr.Of("test_experiment_1_copy"),
					Desc:      gptr.Of("test description 1"),
					Status:    gptr.Of(expt.ExptStatus_Pending),
					CreatorBy: gptr.Of(validUserID),
					BaseInfo: &common.BaseInfo{
						CreatedBy: &common.UserInfo{
							UserID: gptr.Of(validUserID),
						},
					},
				},
				BaseResp: base.NewBaseResp(),
			},
			wantErr: false,
		},
		{
			name: "permission validation failed",
			req: &exptpb.CloneExperimentRequest{
				ExptID:      gptr.Of(validExptID),
				WorkspaceID: gptr.Of(validWorkspaceID),
			},
			mockSetup: func() {
				// 模拟权限验证失败
				mockAuth.EXPECT().
					Authorization(
						gomock.Any(),
						gomock.Any(),
					).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr: true,
		},
		{
			name: "clone operation failed",
			req: &exptpb.CloneExperimentRequest{
				ExptID:      gptr.Of(validExptID),
				WorkspaceID: gptr.Of(validWorkspaceID),
			},
			mockSetup: func() {
				// 模拟权限验证
				mockAuth.EXPECT().
					Authorization(
						gomock.Any(),
						gomock.Any(),
					).Return(nil)

				// 模拟克隆实验失败
				mockManager.EXPECT().
					Clone(gomock.Any(), validExptID, validWorkspaceID, &entity.Session{}).
					Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantErr: true,
		},
		{
			name: "create statistics failed",
			req: &exptpb.CloneExperimentRequest{
				ExptID:      gptr.Of(validExptID),
				WorkspaceID: gptr.Of(validWorkspaceID),
			},
			mockSetup: func() {
				// 模拟权限验证
				mockAuth.EXPECT().
					Authorization(
						gomock.Any(),
						gomock.Any(),
					).Return(nil)

				// 模拟克隆实验
				mockManager.EXPECT().
					Clone(gomock.Any(), validExptID, validWorkspaceID, &entity.Session{}).
					Return(clonedExpt, nil)

				// 模拟生成统计信息ID
				mockIDGen.EXPECT().
					GenID(gomock.Any()).
					Return(newStatsID, nil)

				// 模拟创建统计信息失败
				mockResultSvc.EXPECT().
					CreateStats(
						gomock.Any(),
						&entity.ExptStats{
							ID:      newStatsID,
							SpaceID: validWorkspaceID,
							ExptID:  newExptID,
						},
						&entity.Session{},
					).Return(errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建被测试对象
			app := &experimentApplication{
				manager:         mockManager,
				auth:            mockAuth,
				idgen:           mockIDGen,
				resultSvc:       mockResultSvc,
				userInfoService: mockUserInfoService,
			}

			// 设置 mock 行为
			tt.mockSetup()

			// 执行测试
			gotResp, err := app.CloneExperiment(context.Background(), tt.req)

			// 验证结果
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, gotResp)
			assert.Equal(t, tt.wantResp.GetExperiment().GetID(), gotResp.GetExperiment().GetID())
			assert.Equal(t, tt.wantResp.GetExperiment().GetName(), gotResp.GetExperiment().GetName())
			assert.Equal(t, tt.wantResp.GetExperiment().GetDesc(), gotResp.GetExperiment().GetDesc())
			assert.Equal(t, tt.wantResp.GetExperiment().GetStatus(), gotResp.GetExperiment().GetStatus())
			assert.Equal(t, tt.wantResp.GetExperiment().GetCreatorBy(), gotResp.GetExperiment().GetCreatorBy())
		})
	}
}

func TestExperimentApplication_RunExperiment(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建 mock 对象
	mockManager := servicemocks.NewMockIExptManager(ctrl)
	mockIDGen := idgenmock.NewMockIIDGenerator(ctrl)

	// 测试数据
	validWorkspaceID := int64(123)
	validExptID := int64(456)
	validUserID := int64(789)
	validRunID := int64(999)

	tests := []struct {
		name      string
		req       *exptpb.RunExperimentRequest
		mockSetup func()
		wantResp  *exptpb.RunExperimentResponse
		wantErr   bool
	}{
		{
			name: "successfully run experiment",
			req: &exptpb.RunExperimentRequest{
				WorkspaceID: gptr.Of(validWorkspaceID),
				ExptID:      gptr.Of(validExptID),
				ExptType:    gptr.Of(expt.ExptType_Offline),
				Session: &common.Session{
					UserID: gptr.Of(validUserID),
				},
			},
			mockSetup: func() {
				// 模拟生成运行ID
				mockIDGen.EXPECT().
					GenID(gomock.Any()).
					Return(validRunID, nil)

				// 模拟记录运行
				mockManager.EXPECT().
					LogRun(
						gomock.Any(),
						validExptID,
						validRunID,
						entity.EvaluationModeSubmit,
						validWorkspaceID,
						&entity.Session{UserID: strconv.FormatInt(validUserID, 10)},
					).Return(nil)

				// 模拟运行实验
				mockManager.EXPECT().
					Run(
						gomock.Any(),
						validExptID,
						validRunID,
						validWorkspaceID,
						&entity.Session{UserID: strconv.FormatInt(validUserID, 10)},
						entity.EvaluationModeSubmit,
						gomock.Any(),
					).Return(nil)
			},
			wantResp: &exptpb.RunExperimentResponse{
				RunID:    gptr.Of(validRunID),
				BaseResp: base.NewBaseResp(),
			},
			wantErr: false,
		},
		{
			name: "run experiment failed",
			req: &exptpb.RunExperimentRequest{
				WorkspaceID: gptr.Of(validWorkspaceID),
				ExptID:      gptr.Of(validExptID),
				ExptType:    gptr.Of(expt.ExptType_Offline),
				Session: &common.Session{
					UserID: gptr.Of(validUserID),
				},
			},
			mockSetup: func() {
				// 模拟生成运行ID
				mockIDGen.EXPECT().
					GenID(gomock.Any()).
					Return(validRunID, nil)

				// 模拟记录运行
				mockManager.EXPECT().
					LogRun(
						gomock.Any(),
						validExptID,
						validRunID,
						entity.EvaluationModeSubmit,
						validWorkspaceID,
						&entity.Session{UserID: strconv.FormatInt(validUserID, 10)},
					).Return(nil)

				// 模拟运行实验失败
				mockManager.EXPECT().
					Run(
						gomock.Any(),
						validExptID,
						validRunID,
						validWorkspaceID,
						&entity.Session{UserID: strconv.FormatInt(validUserID, 10)},
						entity.EvaluationModeSubmit,
						gomock.Any(),
					).Return(errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建被测试对象
			app := &experimentApplication{
				manager: mockManager,
				idgen:   mockIDGen,
			}

			// 设置 mock 行为
			tt.mockSetup()

			// 执行测试
			gotResp, err := app.RunExperiment(context.Background(), tt.req)

			// 验证结果
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, gotResp)
			assert.Equal(t, tt.wantResp.GetRunID(), gotResp.GetRunID())
			assert.NotNil(t, gotResp.GetBaseResp())
		})
	}
}

func TestExperimentApplication_RetryExperiment(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建 mock 对象
	mockManager := servicemocks.NewMockIExptManager(ctrl)
	mockIDGen := idgenmock.NewMockIIDGenerator(ctrl)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)

	// 测试数据
	validWorkspaceID := int64(123)
	validExptID := int64(456)
	validUserID := int64(789)
	validRunID := int64(999)

	tests := []struct {
		name      string
		req       *exptpb.RetryExperimentRequest
		mockSetup func()
		wantResp  *exptpb.RetryExperimentResponse
		wantErr   bool
	}{
		{
			name: "successfully retry experiment",
			req: &exptpb.RetryExperimentRequest{
				WorkspaceID: gptr.Of(validWorkspaceID),
				ExptID:      gptr.Of(validExptID),
			},
			mockSetup: func() {
				// 获取实验信息
				mockManager.EXPECT().Get(gomock.Any(), validExptID, validWorkspaceID, gomock.Any()).Return(&entity.Experiment{
					ID:        validExptID,
					SpaceID:   validWorkspaceID,
					CreatedBy: strconv.FormatInt(validUserID, 10),
				}, nil)

				// 权限验证
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), &rpc.AuthorizationWithoutSPIParam{
					ObjectID:        strconv.FormatInt(validExptID, 10),
					SpaceID:         validWorkspaceID,
					ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Run), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExperiment)}},
					OwnerID:         gptr.Of(strconv.FormatInt(validUserID, 10)),
					ResourceSpaceID: validWorkspaceID,
				}).Return(nil)

				// 生成新的 runID
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(validRunID, nil)

				// 记录运行日志
				mockManager.EXPECT().LogRun(gomock.Any(), validExptID, validRunID, entity.EvaluationModeFailRetry, validWorkspaceID, gomock.Any()).Return(nil)

				// 重试失败的实验
				mockManager.EXPECT().RetryUnSuccess(gomock.Any(), validExptID, validRunID, validWorkspaceID, gomock.Any(), gomock.Any()).Return(nil)
			},
			wantResp: &exptpb.RetryExperimentResponse{
				RunID:    gptr.Of(validRunID),
				BaseResp: base.NewBaseResp(),
			},
			wantErr: false,
		},
		{
			name: "experiment does not exist",
			req: &exptpb.RetryExperimentRequest{
				WorkspaceID: gptr.Of(validWorkspaceID),
				ExptID:      gptr.Of(validExptID),
			},
			mockSetup: func() {
				mockManager.EXPECT().Get(gomock.Any(), validExptID, validWorkspaceID, gomock.Any()).Return(nil, errorx.NewByCode(errno.ResourceNotFoundCode))
			},
			wantResp: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置 mock 期望
			tt.mockSetup()

			// 创建被测试的 experimentApplication 实例
			app := NewExperimentApplication(
				nil, // aggResultSvc
				nil, // resultSvc
				mockManager,
				nil, // scheduler
				nil, // recordEval
				mockIDGen,
				nil, // configer
				mockAuth,
				nil, // userInfoService
				nil, // evalTargetService
				nil, // evaluationSetItemService
				nil,
				nil,
				nil,
			)

			// 执行测试
			gotResp, err := app.RetryExperiment(context.Background(), tt.req)

			// 验证结果
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantResp, gotResp)
		})
	}
}

func TestExperimentApplication_KillExperiment(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock objects
	mockManager := servicemocks.NewMockIExptManager(ctrl)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)

	// Test data
	validWorkspaceID := int64(123)
	validExptID := int64(456)
	validUserID := int64(789)

	tests := []struct {
		name      string
		req       *exptpb.KillExperimentRequest
		mockSetup func()
		wantResp  *exptpb.KillExperimentResponse
		wantErr   bool
	}{
		{
			name: "successfully terminate experiment",
			req: &exptpb.KillExperimentRequest{
				WorkspaceID: gptr.Of(validWorkspaceID),
				ExptID:      gptr.Of(validExptID),
			},
			mockSetup: func() {
				// 获取实验信息
				mockManager.EXPECT().Get(gomock.Any(), validExptID, validWorkspaceID, gomock.Any()).Return(&entity.Experiment{
					ID:        validExptID,
					SpaceID:   validWorkspaceID,
					CreatedBy: strconv.FormatInt(validUserID, 10),
				}, nil)

				// 权限验证
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), &rpc.AuthorizationWithoutSPIParam{
					ObjectID:        strconv.FormatInt(validExptID, 10),
					SpaceID:         validWorkspaceID,
					ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Run), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExperiment)}},
					OwnerID:         gptr.Of(strconv.FormatInt(validUserID, 10)),
					ResourceSpaceID: validWorkspaceID,
				}).Return(nil)

				// 终止实验
				mockManager.EXPECT().CompleteExpt(gomock.Any(), validExptID, validWorkspaceID, gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, exptID, spaceID int64, session *entity.Session, opts ...entity.CompleteExptOptionFn) error {
						// 验证传入的 opts 是否包含正确的状态设置
						opt := &entity.CompleteExptOption{}
						for _, fn := range opts {
							fn(opt)
						}
						if opt.Status != entity.ExptStatus_Terminated {
							t.Errorf("expected status %v, got %v", entity.ExptStatus_Terminated, opt.Status)
						}
						return nil
					})
			},
			wantResp: &exptpb.KillExperimentResponse{
				BaseResp: base.NewBaseResp(),
			},
			wantErr: false,
		},
		{
			name: "experiment does not exist",
			req: &exptpb.KillExperimentRequest{
				WorkspaceID: gptr.Of(validWorkspaceID),
				ExptID:      gptr.Of(validExptID),
			},
			mockSetup: func() {
				mockManager.EXPECT().Get(gomock.Any(), validExptID, validWorkspaceID, gomock.Any()).Return(nil, errorx.NewByCode(errno.ResourceNotFoundCode))
			},
			wantResp: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置 mock 期望
			tt.mockSetup()

			// 创建被测试的 experimentApplication 实例
			app := NewExperimentApplication(
				nil, // aggResultSvc
				nil, // resultSvc
				mockManager,
				nil, // scheduler
				nil, // recordEval
				nil,
				nil, // configer
				mockAuth,
				nil, // userInfoService
				nil, // evalTargetService
				nil, // evaluationSetItemService
				nil,
				nil,
				nil,
			)

			// 执行测试
			gotResp, err := app.KillExperiment(context.Background(), tt.req)

			// 验证结果
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantResp, gotResp)
		})
	}
}

func TestExperimentApplication_BatchGetExperimentResult_(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建 mock 对象
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockResultSvc := servicemocks.NewMockExptResultService(ctrl)

	// 测试数据
	validWorkspaceID := int64(123)
	validExptID := int64(456)
	validTotal := int64(10)

	tests := []struct {
		name      string
		req       *exptpb.BatchGetExperimentResultRequest
		mockSetup func()
		wantResp  *exptpb.BatchGetExperimentResultResponse
		wantErr   bool
	}{
		{
			name: "successfully get experiment results",
			req: &exptpb.BatchGetExperimentResultRequest{
				WorkspaceID:   validWorkspaceID,
				ExperimentIds: []int64{validExptID},
				PageNumber:    gptr.Of(int32(1)),
				PageSize:      gptr.Of(int32(10)),
			},
			mockSetup: func() {
				// 模拟权限验证
				mockAuth.EXPECT().
					Authorization(
						gomock.Any(),
						gomock.Any(),
					).DoAndReturn(func(_ context.Context, param *rpc.AuthorizationParam) error {
					assert.Equal(t, strconv.FormatInt(validWorkspaceID, 10), param.ObjectID)
					assert.Equal(t, validWorkspaceID, param.SpaceID)
					assert.Equal(t, rpc.AuthEntityType_Space, *param.ActionObjects[0].EntityType)
					return nil
				})
				mockResultSvc.EXPECT().MGetExperimentResult(
					gomock.Any(),
					gomock.Any(),
				).Return(
					[]*entity.ColumnEvaluator{
						{EvaluatorVersionID: 1, Name: gptr.Of("evaluator1")},
					},
					nil,
					[]*entity.ColumnEvalSetField{
						{Name: gptr.Of("field1"), ContentType: entity.ContentTypeText},
					},
					[]*entity.ExptColumnAnnotation{
						{
							ExptID: validExptID,
							ColumnAnnotations: []*entity.ColumnAnnotation{
								{
									TagKeyID:    1,
									TagName:     "name",
									Description: "desc",
									TagValues: []*entity.TagValue{
										{
											TagValueId:   1,
											TagValueName: "name",
											Status:       entity.TagStatusActive,
										},
									},
									TagContentType: entity.TagContentTypeContinuousNumber,
									TagContentSpec: &entity.TagContentSpec{ContinuousNumberSpec: &entity.ContinuousNumberSpec{
										MinValue:            ptr.Of(float64(1)),
										MinValueDescription: ptr.Of("1"),
										MaxValue:            ptr.Of(float64(2)),
										MaxValueDescription: ptr.Of("2"),
									}},
									TagStatus: entity.TagStatusActive,
								},
							},
						},
					},

					[]*entity.ItemResult{
						{
							ItemID: 1,
							SystemInfo: &entity.ItemSystemInfo{
								RunState: entity.ItemRunState_Success,
								Error:    nil,
							},
							TurnResults: []*entity.TurnResult{
								{
									TurnID: 1,
									ExperimentResults: []*entity.ExperimentResult{
										{
											ExperimentID: 1,
											Payload: &entity.ExperimentTurnPayload{
												TurnID: 1,
												AnnotateResult: &entity.TurnAnnotateResult{
													AnnotateRecords: map[int64]*entity.AnnotateRecord{
														1: {
															ID:           1,
															SpaceID:      1,
															TagKeyID:     1,
															ExperimentID: 1,
															AnnotateData: &entity.AnnotateData{
																Score:          ptr.Of(float64(1)),
																TagContentType: entity.TagContentTypeContinuousNumber,
															},
															TagValueID: 1,
														},
													},
												},
											},
										},
									},
									TurnIndex: nil,
								},
							},
						},
					},
					validTotal,
					nil,
				)
			},
			wantResp: &exptpb.BatchGetExperimentResultResponse{
				ColumnEvaluators: []*expt.ColumnEvaluator{
					{EvaluatorVersionID: 1, Name: gptr.Of("evaluator1")},
				},
				ColumnEvalSetFields: []*expt.ColumnEvalSetField{
					{Name: gptr.Of("field1"), ContentType: gptr.Of(string(entity.ContentTypeText))},
				},
				ExptColumnAnnotations: []*expt.ExptColumnAnnotation{
					{
						ExperimentID: 1,
						ColumnAnnotations: []*expt.ColumnAnnotation{
							{
								TagKeyID:    ptr.Of(int64(1)),
								TagKeyName:  ptr.Of("name"),
								Description: ptr.Of("desc"),
								TagValues: []*tag.TagValue{
									{
										TagValueID:   ptr.Of(int64(1)),
										TagValueName: ptr.Of("name"),
										Status:       ptr.Of(tag.TagStatusActive),
									},
								},
								ContentType: ptr.Of(tag.TagContentTypeContinuousNumber),
								ContentSpec: &tag.TagContentSpec{ContinuousNumberSpec: &tag.ContinuousNumberSpec{
									MinValue:            ptr.Of(float64(1)),
									MinValueDescription: ptr.Of("1"),
									MaxValue:            ptr.Of(float64(2)),
									MaxValueDescription: ptr.Of("2"),
								}},
								Status: ptr.Of(tag.TagStatusActive),
							},
						},
					},

					//{
					//	TagKeyID:    ptr.Of(int64(1)),
					//	TagKeyName:  ptr.Of("name"),
					//	Description: ptr.Of("desc"),
					//	TagValues: []*tag.TagValue{
					//		{
					//			TagValueID:   ptr.Of(int64(1)),
					//			TagValueName: ptr.Of("name"),
					//			Status:       ptr.Of(tag.TagStatusActive),
					//		},
					//	},
					//	ContentType: ptr.Of(tag.TagContentTypeContinuousNumber),
					//	ContentSpec: &tag.TagContentSpec{ContinuousNumberSpec: &tag.ContinuousNumberSpec{
					//		MinValue:            ptr.Of(float64(1)),
					//		MinValueDescription: ptr.Of("1"),
					//		MaxValue:            ptr.Of(float64(2)),
					//		MaxValueDescription: ptr.Of("2"),
					//	}},
					//	Status: ptr.Of(tag.TagStatusActive),
					//},
				},
				ItemResults: []*expt.ItemResult_{
					{
						ItemID: 1,
						SystemInfo: &expt.ItemSystemInfo{
							RunState: gptr.Of(expt.ItemRunState_Success),
							Error:    nil,
						},
						TurnResults: []*expt.TurnResult_{
							{
								TurnID: 1,
								ExperimentResults: []*expt.ExperimentResult_{
									{
										ExperimentID: 1,
										Payload: &expt.ExperimentTurnPayload{
											TurnID: 1,
											AnnotateResult_: &expt.TurnAnnotateResult_{
												AnnotateRecords: map[int64]*expt.AnnotateRecord{
													1: {
														AnnotateRecordID: ptr.Of(int64(1)),
														TagKeyID:         ptr.Of(int64(1)),
														Score:            ptr.Of("1"),
														TagContentType:   ptr.Of(tag.TagContentTypeContinuousNumber),
														TagValueID:       ptr.Of(int64(1)),
													},
												},
											},
										},
									},
								},
								TurnIndex: nil,
							},
						},
					},
				},
				Total:    gptr.Of(validTotal),
				BaseResp: base.NewBaseResp(),
			},
			wantErr: false,
		},
		{
			name: "filter condition parsing failed",
			req: &exptpb.BatchGetExperimentResultRequest{
				WorkspaceID:   validWorkspaceID,
				ExperimentIds: []int64{validExptID},
				Filters: map[int64]*expt.ExperimentFilter{
					validExptID: {
						Filters: &expt.Filters{
							FilterConditions: []*expt.FilterCondition{
								{
									Field: &expt.FilterField{
										FieldType: expt.FieldType_TurnRunState,
									},
									Operator: expt.FilterOperatorType_Equal,
									Value:    "invalid",
								},
							},
							LogicOp: gptr.Of(expt.FilterLogicOp_And),
						},
					},
				},
			},
			mockSetup: func() {
				mockAuth.EXPECT().
					Authorization(
						gomock.Any(),
						gomock.Any(),
					).DoAndReturn(func(_ context.Context, param *rpc.AuthorizationParam) error {
					assert.Equal(t, strconv.FormatInt(validWorkspaceID, 10), param.ObjectID)
					assert.Equal(t, validWorkspaceID, param.SpaceID)
					assert.Equal(t, rpc.AuthEntityType_Space, *param.ActionObjects[0].EntityType)
					return nil
				})
				// 不应该调用 MGetExperimentResult
			},
			wantResp: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &experimentApplication{
				resultSvc: mockResultSvc,
				auth:      mockAuth,
			}

			if tt.mockSetup != nil {
				tt.mockSetup()
			}

			got, err := app.BatchGetExperimentResult_(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("BatchGetExperimentResult_() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// 比较 ColumnEvaluators
				if len(got.ColumnEvaluators) != len(tt.wantResp.ColumnEvaluators) {
					t.Errorf("ColumnEvaluators length mismatch: got %v, want %v", len(got.ColumnEvaluators), len(tt.wantResp.ColumnEvaluators))
				} else {
					for i, gotEval := range got.ColumnEvaluators {
						wantEval := tt.wantResp.ColumnEvaluators[i]
						if gotEval.EvaluatorVersionID != wantEval.EvaluatorVersionID ||
							gptr.Indirect(gotEval.Name) != gptr.Indirect(wantEval.Name) {
							t.Errorf("ColumnEvaluator mismatch at index %d: got %v, want %v", i, gotEval, wantEval)
						}
					}
				}

				// 比较 ColumnEvalSetFields
				if len(got.ColumnEvalSetFields) != len(tt.wantResp.ColumnEvalSetFields) {
					t.Errorf("ColumnEvalSetFields length mismatch: got %v, want %v", len(got.ColumnEvalSetFields), len(tt.wantResp.ColumnEvalSetFields))
				} else {
					for i, gotField := range got.ColumnEvalSetFields {
						wantField := tt.wantResp.ColumnEvalSetFields[i]
						if gptr.Indirect(gotField.Name) != gptr.Indirect(wantField.Name) ||
							gptr.Indirect(gotField.ContentType) != gptr.Indirect(wantField.ContentType) {
							t.Errorf("ColumnEvalSetField mismatch at index %d: got %v, want %v", i, gotField, wantField)
						}
					}
				}

				// 比较 ItemResults
				if len(got.ItemResults) != len(tt.wantResp.ItemResults) {
					t.Errorf("ItemResults length mismatch: got %v, want %v", len(got.ItemResults), len(tt.wantResp.ItemResults))
				} else {
					for i, gotItem := range got.ItemResults {
						wantItem := tt.wantResp.ItemResults[i]
						if gotItem.ItemID != wantItem.ItemID ||
							gptr.Indirect(gotItem.SystemInfo.RunState) != gptr.Indirect(wantItem.SystemInfo.RunState) ||
							gotItem.SystemInfo.Error != wantItem.SystemInfo.Error {
							t.Errorf("ItemResult mismatch at index %d: got %v, want %v", i, gotItem, wantItem)
						}
					}
				}

				// 比较 Total
				if gptr.Indirect(got.Total) != gptr.Indirect(tt.wantResp.Total) {
					t.Errorf("Total mismatch: got %v, want %v", gptr.Indirect(got.Total), gptr.Indirect(tt.wantResp.Total))
				}

				// 比较 BaseResp
				if got.BaseResp == nil {
					t.Error("BaseResp is nil")
				} else if got.BaseResp.GetStatusCode() != tt.wantResp.BaseResp.GetStatusCode() ||
					got.BaseResp.GetStatusMessage() != tt.wantResp.BaseResp.GetStatusMessage() {
					t.Errorf("BaseResp mismatch: got %v, want %v", got.BaseResp, tt.wantResp.BaseResp)
				}
			}
		})
	}
}

func TestExperimentApplication_BatchGetExperimentAggrResult_(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建 mock 对象
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockAggrResultSvc := servicemocks.NewMockExptAggrResultService(ctrl)

	// 测试数据
	validWorkspaceID := int64(123)
	validExptID := int64(456)
	validEvaluatorVersionID := int64(789)

	tests := []struct {
		name      string
		req       *exptpb.BatchGetExperimentAggrResultRequest
		mockSetup func()
		wantResp  *exptpb.BatchGetExperimentAggrResultResponse
		wantErr   bool
	}{
		{
			name: "successfully get experiment aggregate results",
			req: &exptpb.BatchGetExperimentAggrResultRequest{
				WorkspaceID:   validWorkspaceID,
				ExperimentIds: []int64{validExptID},
			},
			mockSetup: func() {
				// 模拟权限验证
				mockAuth.EXPECT().
					Authorization(
						gomock.Any(),
						gomock.Any(),
					).DoAndReturn(func(_ context.Context, param *rpc.AuthorizationParam) error {
					assert.Equal(t, strconv.FormatInt(validWorkspaceID, 10), param.ObjectID)
					assert.Equal(t, validWorkspaceID, param.SpaceID)
					assert.Equal(t, rpc.AuthEntityType_Space, *param.ActionObjects[0].EntityType)
					return nil
				})
				mockAggrResultSvc.EXPECT().BatchGetExptAggrResultByExperimentIDs(
					gomock.Any(),
					validWorkspaceID,
					[]int64{validExptID},
				).Return(
					[]*entity.ExptAggregateResult{
						{
							ExperimentID: validExptID,
							EvaluatorResults: map[int64]*entity.EvaluatorAggregateResult{
								validEvaluatorVersionID: {
									EvaluatorVersionID: validEvaluatorVersionID,
									AggregatorResults: []*entity.AggregatorResult{
										{
											AggregatorType: entity.Average,
											Data: &entity.AggregateData{
												Value: gptr.Of(0.85),
											},
										},
									},
									Name:    gptr.Of("evaluator1"),
									Version: gptr.Of("v1"),
								},
							},
							Status: 0,
							AnnotationResults: map[int64]*entity.AnnotationAggregateResult{
								1: {
									TagKeyID: 1,
									Name:     ptr.Of("name"),
									AggregatorResults: []*entity.AggregatorResult{
										{
											AggregatorType: entity.Distribution,
											Data: &entity.AggregateData{
												Value:              gptr.Of(0.85),
												OptionDistribution: &entity.OptionDistributionData{},
											},
										},
									},
								},
							},
						},
					}, nil)
			},

			wantResp: &exptpb.BatchGetExperimentAggrResultResponse{
				ExptAggregateResults: []*expt.ExptAggregateResult_{
					{
						ExperimentID: validExptID,
						EvaluatorResults: map[int64]*expt.EvaluatorAggregateResult_{
							validEvaluatorVersionID: {
								EvaluatorVersionID: validEvaluatorVersionID,
								AggregatorResults: []*expt.AggregatorResult_{
									{
										AggregatorType: expt.AggregatorType_Average,
										Data: &expt.AggregateData{
											DataType: expt.DataType_Double,
											Value:    gptr.Of(0.85),
										},
									},
								},
								Name:    gptr.Of("evaluator1"),
								Version: gptr.Of("v1"),
							},
						},
						AnnotationResults: map[int64]*expt.AnnotationAggregateResult_{
							1: {
								TagKeyID: 1,
								Name:     ptr.Of("name"),
								AggregatorResults: []*expt.AggregatorResult_{
									{
										AggregatorType: expt.AggregatorType_Distribution,
										Data: &expt.AggregateData{
											Value:              gptr.Of(0.85),
											OptionDistribution: &expt.OptionDistribution{},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "get aggregate results failed",
			req: &exptpb.BatchGetExperimentAggrResultRequest{
				WorkspaceID:   validWorkspaceID,
				ExperimentIds: []int64{validExptID},
			},
			mockSetup: func() {
				// 模拟权限验证
				mockAuth.EXPECT().
					Authorization(
						gomock.Any(),
						gomock.Any(),
					).DoAndReturn(func(_ context.Context, param *rpc.AuthorizationParam) error {
					assert.Equal(t, strconv.FormatInt(validWorkspaceID, 10), param.ObjectID)
					assert.Equal(t, validWorkspaceID, param.SpaceID)
					assert.Equal(t, rpc.AuthEntityType_Space, *param.ActionObjects[0].EntityType)
					return nil
				})
				mockAggrResultSvc.EXPECT().BatchGetExptAggrResultByExperimentIDs(
					gomock.Any(),
					validWorkspaceID,
					[]int64{validExptID},
				).Return(nil, errors.New("mock error"))
			},
			wantResp: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &experimentApplication{
				ExptAggrResultService: mockAggrResultSvc,
				auth:                  mockAuth,
			}

			if tt.mockSetup != nil {
				tt.mockSetup()
			}

			got, err := app.BatchGetExperimentAggrResult_(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("BatchGetExperimentAggrResult_() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// 比较 ExptAggregateResults
				if len(got.ExptAggregateResults) != len(tt.wantResp.ExptAggregateResults) {
					t.Errorf("ExptAggregateResults length mismatch: got %v, want %v", len(got.ExptAggregateResults), len(tt.wantResp.ExptAggregateResults))
				} else {
					for i, gotResult := range got.ExptAggregateResults {
						wantResult := tt.wantResp.ExptAggregateResults[i]
						if gotResult.ExperimentID != wantResult.ExperimentID {
							t.Errorf("ExperimentID mismatch at index %d: got %v, want %v", i, gotResult.ExperimentID, wantResult.ExperimentID)
						}

						// 比较 EvaluatorResults
						if len(gotResult.EvaluatorResults) != len(wantResult.EvaluatorResults) {
							t.Errorf("EvaluatorResults length mismatch at index %d: got %v, want %v", i, len(gotResult.EvaluatorResults), len(wantResult.EvaluatorResults))
						} else {
							for versionID, gotEval := range gotResult.EvaluatorResults {
								wantEval := wantResult.EvaluatorResults[versionID]
								if gotEval.EvaluatorVersionID != wantEval.EvaluatorVersionID ||
									gptr.Indirect(gotEval.Name) != gptr.Indirect(wantEval.Name) ||
									gptr.Indirect(gotEval.Version) != gptr.Indirect(wantEval.Version) {
									t.Errorf("EvaluatorResult mismatch for version %d: got %v, want %v", versionID, gotEval, wantEval)
								}

								// 比较 AggregatorResults
								if len(gotEval.AggregatorResults) != len(wantEval.AggregatorResults) {
									t.Errorf("AggregatorResults length mismatch for version %d: got %v, want %v", versionID, len(gotEval.AggregatorResults), len(wantEval.AggregatorResults))
								} else {
									for j, gotAggr := range gotEval.AggregatorResults {
										wantAggr := wantEval.AggregatorResults[j]
										if gotAggr.AggregatorType != wantAggr.AggregatorType ||
											gptr.Indirect(gotAggr.Data.Value) != gptr.Indirect(wantAggr.Data.Value) {
											t.Errorf("AggregatorResult mismatch at index %d for version %d: got %v, want %v", j, versionID, gotAggr, wantAggr)
										}
									}
								}
							}
						}
					}
				}
			}
		})
	}
}

func TestExperimentApplication_AuthReadExperiments(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	app := &experimentApplication{
		auth: mockAuth,
	}

	validSpaceID := int64(1001)
	validExptID1 := int64(2001)
	validExptID2 := int64(2002)
	validCreatedBy := "user-123"

	testExpts := []*entity.Experiment{
		{
			ID:        validExptID1,
			SpaceID:   validSpaceID,
			CreatedBy: validCreatedBy,
		},
		{
			ID:        validExptID2,
			SpaceID:   validSpaceID,
			CreatedBy: validCreatedBy,
		},
	}

	tests := []struct {
		name      string
		dos       []*entity.Experiment
		spaceID   int64
		mockSetup func()
		wantErr   bool
	}{
		{
			name:    "success - valid experiments",
			dos:     testExpts,
			spaceID: validSpaceID,
			mockSetup: func() {
				mockAuth.EXPECT().
					MAuthorizeWithoutSPI(
						gomock.Any(),
						validSpaceID,
						[]*rpc.AuthorizationWithoutSPIParam{
							{
								ObjectID:        strconv.FormatInt(validExptID1, 10),
								SpaceID:         validSpaceID,
								ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExperiment)}},
								OwnerID:         gptr.Of(validCreatedBy),
								ResourceSpaceID: validSpaceID,
							},
							{
								ObjectID:        strconv.FormatInt(validExptID2, 10),
								SpaceID:         validSpaceID,
								ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExperiment)}},
								OwnerID:         gptr.Of(validCreatedBy),
								ResourceSpaceID: validSpaceID,
							},
						},
					).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name:    "error - authorization failed",
			dos:     testExpts,
			spaceID: validSpaceID,
			mockSetup: func() {
				mockAuth.EXPECT().
					MAuthorizeWithoutSPI(
						gomock.Any(),
						validSpaceID,
						gomock.Any(),
					).
					Return(errors.New("authorization failed"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := app.AuthReadExperiments(context.Background(), tt.dos, tt.spaceID)
			if (err != nil) != tt.wantErr {
				t.Errorf("AuthReadExperiments() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExperimentApplication_InvokeExperiment(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockManager := servicemocks.NewMockIExptManager(ctrl)
	mockEvalSetItemService := servicemocks.NewMockEvaluationSetItemService(ctrl)
	mockResultSvc := servicemocks.NewMockExptResultService(ctrl)

	app := &experimentApplication{
		auth:                     mockAuth,
		manager:                  mockManager,
		evaluationSetItemService: mockEvalSetItemService,
		resultSvc:                mockResultSvc,
	}

	validSpaceID := int64(1001)
	validExptID := int64(2001)
	validExptRunID := int64(3001)
	validEvalSetID := int64(4001)
	validUserID := int64(5001)
	validCreatedBy := "user-123"

	validExpt := &entity.Experiment{
		ID:        validExptID,
		SpaceID:   validSpaceID,
		CreatedBy: validCreatedBy,
		Status:    entity.ExptStatus_Processing,
	}

	validItems := []*domain_eval_set.EvaluationSetItem{
		{
			ID: gptr.Of(int64(6001)),
		},
		{
			ID: gptr.Of(int64(6002)),
		},
	}

	tests := []struct {
		name      string
		req       *exptpb.InvokeExperimentRequest
		mockSetup func()
		wantResp  *exptpb.InvokeExperimentResponse
		wantErr   bool
	}{
		{
			name: "success - valid request",
			req: &exptpb.InvokeExperimentRequest{
				WorkspaceID:      validSpaceID,
				ExperimentID:     gptr.Of(validExptID),
				ExperimentRunID:  gptr.Of(validExptRunID),
				EvaluationSetID:  validEvalSetID,
				Items:            validItems,
				Session:          &common.Session{UserID: gptr.Of(validUserID)},
				SkipInvalidItems: gptr.Of(true),
				AllowPartialAdd:  gptr.Of(true),
			},
			mockSetup: func() {
				// Mock Get experiment
				mockManager.EXPECT().
					Get(gomock.Any(), validExptID, validSpaceID, &entity.Session{UserID: strconv.FormatInt(validUserID, 10)}).
					Return(validExpt, nil)

				// Mock authorization
				mockAuth.EXPECT().
					AuthorizationWithoutSPI(
						gomock.Any(),
						&rpc.AuthorizationWithoutSPIParam{
							ObjectID:        strconv.FormatInt(validExptID, 10),
							SpaceID:         validSpaceID,
							ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Run), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExperiment)}},
							OwnerID:         gptr.Of(validCreatedBy),
							ResourceSpaceID: validSpaceID,
						},
					).
					Return(nil)

				// Mock BatchCreateEvaluationSetItems with matcher
				mockEvalSetItemService.EXPECT().
					BatchCreateEvaluationSetItems(
						gomock.Any(),
						gomock.Any(), // 使用 Any 匹配器，因为结构体内部包含指针
					).
					DoAndReturn(func(_ context.Context, param *entity.BatchCreateEvaluationSetItemsParam) (map[int64]int64, []*entity.ItemErrorGroup, error) {
						// 验证关键字段
						if param.SpaceID != validSpaceID || param.EvaluationSetID != validEvalSetID {
							t.Errorf("unexpected param values: got SpaceID=%v, EvaluationSetID=%v", param.SpaceID, param.EvaluationSetID)
						}
						return map[int64]int64{int64(0): 6001, int64(1): 6002}, nil, nil
					})

				// Mock Invoke experiment with matcher
				mockManager.EXPECT().
					Invoke(
						gomock.Any(),
						gomock.Any(), // 使用 Any 匹配器，因为结构体内部包含指针
					).
					DoAndReturn(func(_ context.Context, param *entity.InvokeExptReq) error {
						// 验证关键字段
						if param.ExptID != validExptID || param.RunID != validExptRunID || param.SpaceID != validSpaceID {
							t.Errorf("unexpected param values: got ExptID=%v, RunID=%v, SpaceID=%v", param.ExptID, param.RunID, param.SpaceID)
						}
						return nil
					})

				// Mock UpsertExptTurnResultFilter
				mockResultSvc.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			wantResp: &exptpb.InvokeExperimentResponse{
				AddedItems: map[int64]int64{int64(0): 6001, int64(1): 6002},
				BaseResp:   base.NewBaseResp(),
			},
			wantErr: false,
		},
		{
			name: "error - experiment status not allowed",
			req: &exptpb.InvokeExperimentRequest{
				WorkspaceID:     validSpaceID,
				ExperimentID:    gptr.Of(validExptID),
				ExperimentRunID: gptr.Of(validExptRunID),
				Session:         &common.Session{UserID: gptr.Of(validUserID)},
			},
			mockSetup: func() {
				// Mock Get experiment with invalid status
				invalidStatusExpt := &entity.Experiment{
					ID:        validExptID,
					SpaceID:   validSpaceID,
					CreatedBy: validCreatedBy,
					Status:    entity.ExptStatus_Success, // Invalid status for invoke
				}
				mockManager.EXPECT().
					Get(gomock.Any(), validExptID, validSpaceID, &entity.Session{UserID: strconv.FormatInt(validUserID, 10)}).
					Return(invalidStatusExpt, nil)

				// Mock authorization
				mockAuth.EXPECT().
					AuthorizationWithoutSPI(
						gomock.Any(),
						&rpc.AuthorizationWithoutSPIParam{
							ObjectID:        strconv.FormatInt(validExptID, 10),
							SpaceID:         validSpaceID,
							ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Run), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExperiment)}},
							OwnerID:         gptr.Of(validCreatedBy),
							ResourceSpaceID: validSpaceID,
						},
					).
					Return(nil)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			gotResp, err := app.InvokeExperiment(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("InvokeExperiment() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(gotResp, tt.wantResp) {
				t.Errorf("InvokeExperiment() gotResp = %v, want %v", gotResp, tt.wantResp)
			}
		})
	}
}

func TestExperimentApplication_FinishExperiment(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockManager := servicemocks.NewMockIExptManager(ctrl)

	app := &experimentApplication{
		auth:    mockAuth,
		manager: mockManager,
	}

	validSpaceID := int64(1001)
	validExptID := int64(2001)
	validExptRunID := int64(3001)
	validUserID := int64(5001)
	validCreatedBy := "user-123"

	validExpt := &entity.Experiment{
		ID:        validExptID,
		SpaceID:   validSpaceID,
		CreatedBy: validCreatedBy,
		Status:    entity.ExptStatus_Processing,
	}

	tests := []struct {
		name      string
		req       *exptpb.FinishExperimentRequest
		mockSetup func()
		wantResp  *exptpb.FinishExperimentResponse
		wantErr   bool
	}{
		{
			name: "success - valid request",
			req: &exptpb.FinishExperimentRequest{
				WorkspaceID:     gptr.Of(validSpaceID),
				ExperimentID:    gptr.Of(validExptID),
				ExperimentRunID: gptr.Of(validExptRunID),
				Session:         &common.Session{UserID: gptr.Of(validUserID)},
			},
			mockSetup: func() {
				// Mock Get experiment
				mockManager.EXPECT().
					Get(gomock.Any(), validExptID, validSpaceID, &entity.Session{UserID: strconv.FormatInt(validUserID, 10)}).
					Return(validExpt, nil)

				// Mock authorization
				mockAuth.EXPECT().
					AuthorizationWithoutSPI(
						gomock.Any(),
						&rpc.AuthorizationWithoutSPIParam{
							ObjectID:        strconv.FormatInt(validExptID, 10),
							SpaceID:         validSpaceID,
							ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Run), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExperiment)}},
							OwnerID:         gptr.Of(validCreatedBy),
							ResourceSpaceID: validSpaceID,
						},
					).
					Return(nil)

				// Mock Finish experiment
				mockManager.EXPECT().
					Finish(
						gomock.Any(),
						validExpt,
						validExptRunID,
						&entity.Session{UserID: strconv.FormatInt(validUserID, 10)},
					).
					Return(nil)
			},
			wantResp: &exptpb.FinishExperimentResponse{
				BaseResp: base.NewBaseResp(),
			},
			wantErr: false,
		},
		{
			name: "success - already finished",
			req: &exptpb.FinishExperimentRequest{
				WorkspaceID:     gptr.Of(validSpaceID),
				ExperimentID:    gptr.Of(validExptID),
				ExperimentRunID: gptr.Of(validExptRunID),
				Session:         &common.Session{UserID: gptr.Of(validUserID)},
			},
			mockSetup: func() {
				// Mock Get experiment with already finished status
				finishedExpt := &entity.Experiment{
					ID:        validExptID,
					SpaceID:   validSpaceID,
					CreatedBy: validCreatedBy,
					Status:    entity.ExptStatus_Success, // Already finished
				}
				mockManager.EXPECT().
					Get(gomock.Any(), validExptID, validSpaceID, &entity.Session{UserID: strconv.FormatInt(validUserID, 10)}).
					Return(finishedExpt, nil)
			},
			wantResp: &exptpb.FinishExperimentResponse{
				BaseResp: base.NewBaseResp(),
			},
			wantErr: false,
		},
		{
			name: "error - get experiment failed",
			req: &exptpb.FinishExperimentRequest{
				WorkspaceID:     gptr.Of(validSpaceID),
				ExperimentID:    gptr.Of(validExptID),
				ExperimentRunID: gptr.Of(validExptRunID),
				Session:         &common.Session{UserID: gptr.Of(validUserID)},
			},
			mockSetup: func() {
				// Mock Get experiment with error
				mockManager.EXPECT().
					Get(gomock.Any(), validExptID, validSpaceID, &entity.Session{UserID: strconv.FormatInt(validUserID, 10)}).
					Return(nil, errors.New("get experiment failed"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			gotResp, err := app.FinishExperiment(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("FinishExperiment() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(gotResp, tt.wantResp) {
				t.Errorf("FinishExperiment() gotResp = %v, want %v", gotResp, tt.wantResp)
			}
		})
	}
}

func TestExperimentApplication_GetExptResultExportRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建mock对象
	mockExptResultExportService := servicemocks.NewMockIExptResultExportService(ctrl)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockConfiger := componentMocks.NewMockIConfiger(ctrl)

	// 测试数据
	validWorkspaceID := int64(123)
	validExportID := int64(456)
	validExportRecord := &entity.ExptResultExportRecord{
		ID:              validExportID,
		SpaceID:         validWorkspaceID,
		ExptID:          int64(789),
		CsvExportStatus: entity.CSVExportStatus_Success,
	}

	tests := []struct {
		name      string
		req       *exptpb.GetExptResultExportRecordRequest
		mockSetup func()
		wantResp  *exptpb.GetExptResultExportRecordResponse
		wantErr   bool
		wantCode  int32
	}{{
		name: "成功获取导出记录",
		req: &exptpb.GetExptResultExportRecordRequest{
			WorkspaceID: validWorkspaceID,
			ExportID:    validExportID,
		},
		mockSetup: func() {
			// 模拟权限验证
			mockAuth.EXPECT().
				Authorization(gomock.Any(), gomock.Any()).
				Return(nil)

			// 模拟获取导出记录
			mockExptResultExportService.EXPECT().
				GetExptExportRecord(gomock.Any(), validWorkspaceID, validExportID).
				Return(validExportRecord, nil)
			mockConfiger.EXPECT().GetExptExportWhiteList(gomock.Any()).
				Return(&entity.ExptExportWhiteList{UserIDs: []int64{}}).AnyTimes()
		},
		wantResp: &exptpb.GetExptResultExportRecordResponse{
			ExptResultExportRecord: &expt.ExptResultExportRecord{
				ExportID:        validExportID,
				ExptID:          int64(789),
				CsvExportStatus: experiment.CSVExportStatusDO2DTO(entity.CSVExportStatus_Success),
			},
			BaseResp: base.NewBaseResp(),
		},
		wantErr: false,
	}, {
		name: "导出记录不存在",
		req: &exptpb.GetExptResultExportRecordRequest{
			WorkspaceID: validWorkspaceID,
			ExportID:    int64(999),
		},
		mockSetup: func() {
			// 模拟权限验证
			mockAuth.EXPECT().
				Authorization(gomock.Any(), gomock.Any()).
				Return(nil)

			// 模拟获取导出记录失败
			mockExptResultExportService.EXPECT().
				GetExptExportRecord(gomock.Any(), validWorkspaceID, int64(999)).
				Return(nil, fmt.Errorf("err"))
			mockConfiger.EXPECT().GetExptExportWhiteList(gomock.Any()).
				Return(&entity.ExptExportWhiteList{UserIDs: []int64{}}).AnyTimes()
		},
		wantResp: nil,
		wantErr:  true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置mock行为
			tt.mockSetup()

			// 创建被测试对象
			app := &experimentApplication{
				IExptResultExportService: mockExptResultExportService,
				auth:                     mockAuth,
				configer:                 mockConfiger,
			}

			// 执行测试
			gotResp, err := app.GetExptResultExportRecord(context.Background(), tt.req)

			// 验证结果
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, gotResp)
			assert.Equal(t, tt.wantResp.ExptResultExportRecord.GetExportID(), gotResp.ExptResultExportRecord.GetExportID())
			assert.Equal(t, tt.wantResp.ExptResultExportRecord.GetCsvExportStatus(), gotResp.ExptResultExportRecord.GetCsvExportStatus())
		})
	}
}

func TestExperimentApplication_ListExptResultExportRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建mock对象
	mockExptResultExportService := servicemocks.NewMockIExptResultExportService(ctrl)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockUserInfoService := userinfomocks.NewMockUserInfoService(ctrl)
	mockConfiger := componentMocks.NewMockIConfiger(ctrl)

	// 测试数据
	validWorkspaceID := int64(123)
	validExptID := int64(456)
	validExportRecords := []*entity.ExptResultExportRecord{{
		ID:      int64(789),
		SpaceID: validWorkspaceID,
		ExptID:  validExptID,
	}, {
		ID:      int64(890),
		SpaceID: validWorkspaceID,
		ExptID:  validExptID,
	}}

	tests := []struct {
		name      string
		req       *exptpb.ListExptResultExportRecordRequest
		mockSetup func()
		wantResp  *exptpb.ListExptResultExportRecordResponse
		wantErr   bool
	}{{
		name: "成功列出导出记录",
		req: &exptpb.ListExptResultExportRecordRequest{
			WorkspaceID: validWorkspaceID,
			ExptID:      validExptID,
			PageNumber:  gptr.Of(int32(1)),
			PageSize:    gptr.Of(int32(10)),
		},
		mockSetup: func() {
			// 模拟权限验证
			mockAuth.EXPECT().
				Authorization(gomock.Any(), gomock.Any()).
				Return(nil)

			// 模拟列出导出记录
			mockExptResultExportService.EXPECT().
				ListExportRecord(gomock.Any(), validWorkspaceID, validExptID, gomock.Any()).
				Return(validExportRecords, int64(len(validExportRecords)), nil)

			// 模拟填充用户信息
			mockUserInfoService.EXPECT().
				PackUserInfo(gomock.Any(), gomock.Any()).
				Do(func(_ context.Context, carriers []userinfo.UserInfoCarrier) {
					assert.Equal(t, len(validExportRecords), len(carriers))
				})
			mockConfiger.EXPECT().GetExptExportWhiteList(gomock.Any()).
				Return(&entity.ExptExportWhiteList{UserIDs: []int64{}}).AnyTimes()
		},
		wantResp: &exptpb.ListExptResultExportRecordResponse{
			ExptResultExportRecords: []*expt.ExptResultExportRecord{{
				ExportID: int64(789),
				ExptID:   validExptID,
			}, {
				ExportID: int64(890),
				ExptID:   validExptID,
			}},
			Total:    gptr.Of(int64(len(validExportRecords))),
			BaseResp: base.NewBaseResp(),
		},
		wantErr: false,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置mock行为
			tt.mockSetup()

			// 创建被测试对象
			app := &experimentApplication{
				IExptResultExportService: mockExptResultExportService,
				auth:                     mockAuth,
				userInfoService:          mockUserInfoService,
				configer:                 mockConfiger,
			}

			// 执行测试
			gotResp, err := app.ListExptResultExportRecord(context.Background(), tt.req)

			// 验证结果
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, gotResp)
			assert.Equal(t, tt.wantResp.Total, gotResp.Total)
			assert.Equal(t, len(tt.wantResp.ExptResultExportRecords), len(gotResp.ExptResultExportRecords))
		})
	}
}

func TestExperimentApplication_ExportExptResult_(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建mock对象
	mockExptResultExportService := servicemocks.NewMockIExptResultExportService(ctrl)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockManager := servicemocks.NewMockIExptManager(ctrl)
	mockConfiger := componentMocks.NewMockIConfiger(ctrl)

	// 测试数据
	validWorkspaceID := int64(123)
	validExptID := int64(456)
	validExportID := int64(789)

	tests := []struct {
		name      string
		req       *exptpb.ExportExptResultRequest
		mockSetup func()
		wantResp  *exptpb.ExportExptResultResponse
		wantErr   bool
		wantCode  int32
	}{{
		name: "成功导出实验结果",
		req: &exptpb.ExportExptResultRequest{
			WorkspaceID: validWorkspaceID,
			ExptID:      validExptID,
		},
		mockSetup: func() {
			// 模拟权限验证
			mockAuth.EXPECT().
				AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).
				Return(nil)

			// 模拟导出实验结果
			mockExptResultExportService.EXPECT().
				ExportCSV(gomock.Any(), validWorkspaceID, validExptID, gomock.Any()).
				Return(validExportID, nil)
			mockManager.EXPECT().
				Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&entity.Experiment{}, nil)
			mockConfiger.EXPECT().GetExptExportWhiteList(gomock.Any()).
				Return(&entity.ExptExportWhiteList{UserIDs: []int64{}}).AnyTimes()
		},
		wantResp: &exptpb.ExportExptResultResponse{
			ExportID: validExportID,
			BaseResp: base.NewBaseResp(),
		},
		wantErr: false,
	}, {
		name: "权限不足",
		req: &exptpb.ExportExptResultRequest{
			WorkspaceID: validWorkspaceID,
			ExptID:      validExptID,
		},
		mockSetup: func() {
			// 模拟权限验证失败
			mockAuth.EXPECT().
				AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).
				Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			mockManager.EXPECT().
				Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&entity.Experiment{}, nil)
			mockConfiger.EXPECT().GetExptExportWhiteList(gomock.Any()).
				Return(&entity.ExptExportWhiteList{UserIDs: []int64{}}).AnyTimes()
		},
		wantResp: nil,
		wantErr:  true,
		wantCode: errno.CommonNoPermissionCode,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置mock行为
			tt.mockSetup()

			// 创建被测试对象
			app := &experimentApplication{
				IExptResultExportService: mockExptResultExportService,
				auth:                     mockAuth,
				manager:                  mockManager,
				configer:                 mockConfiger,
			}

			// 执行测试
			gotResp, err := app.ExportExptResult_(context.Background(), tt.req)

			// 验证结果
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, gotResp)
			assert.Equal(t, tt.wantResp.ExportID, gotResp.ExportID)
		})
	}
}

func TestExperimentApplication_DeleteAnnotationTag(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建mock对象
	mockAnnotateService := servicemocks.NewMockIExptAnnotateService(ctrl)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockManager := servicemocks.NewMockIExptManager(ctrl)

	// 测试数据
	validWorkspaceID := int64(123)
	validTagID := int64(456)

	tests := []struct {
		name      string
		req       *exptpb.DeleteAnnotationTagReq
		mockSetup func()
		wantResp  *exptpb.DeleteAnnotationTagResp
		wantErr   bool
		wantCode  int32
	}{{
		name: "成功删除标注标签",
		req: &exptpb.DeleteAnnotationTagReq{
			WorkspaceID: validWorkspaceID,
			TagKeyID:    ptr.Of(validTagID),
		},
		mockSetup: func() {
			// 模拟权限验证
			mockAuth.EXPECT().
				AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).
				Return(nil)
			mockManager.EXPECT().
				Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&entity.Experiment{}, nil)

			// 模拟删除标签
			mockAnnotateService.EXPECT().
				DeleteExptTurnResultTagRef(gomock.Any(), gomock.Any(), validWorkspaceID, validTagID).
				Return(nil)
		},
		wantResp: &exptpb.DeleteAnnotationTagResp{
			BaseResp: base.NewBaseResp(),
		},
		wantErr: false,
	}, {
		name: "标签不存在",
		req: &exptpb.DeleteAnnotationTagReq{
			WorkspaceID: validWorkspaceID,
			TagKeyID:    ptr.Of(int64(999)),
		},
		mockSetup: func() {
			// 模拟权限验证
			mockAuth.EXPECT().
				AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).
				Return(nil)
			mockManager.EXPECT().
				Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&entity.Experiment{}, nil)

			// 模拟删除标签失败
			mockAnnotateService.EXPECT().
				DeleteExptTurnResultTagRef(gomock.Any(), gomock.Any(), validWorkspaceID, int64(999)).
				Return(errorx.NewByCode(errno.ResourceNotFoundCode))
		},
		wantResp: nil,
		wantErr:  true,
		wantCode: errno.ResourceNotFoundCode,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置mock行为
			tt.mockSetup()

			// 创建被测试对象
			app := &experimentApplication{
				annotateService: mockAnnotateService,
				auth:            mockAuth,
				manager:         mockManager,
			}

			// 执行测试
			gotResp, err := app.DeleteAnnotationTag(context.Background(), tt.req)

			// 验证结果
			if tt.wantErr {
				assert.Error(t, err)
				statusErr, ok := errorx.FromStatusError(err)
				assert.True(t, ok)
				assert.Equal(t, tt.wantCode, statusErr.Code())
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, gotResp)
		})
	}
}

func TestExperimentApplication_UpdateAnnotateRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建mock对象
	mockAnnotateService := servicemocks.NewMockIExptAnnotateService(ctrl)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockManager := servicemocks.NewMockIExptManager(ctrl)

	// 测试数据
	validWorkspaceID := int64(123)
	validRecordID := int64(456)

	tests := []struct {
		name      string
		req       *exptpb.UpdateAnnotateRecordReq
		mockSetup func()
		wantResp  *exptpb.UpdateAnnotateRecordResp
		wantErr   bool
		wantCode  int32
	}{{
		name: "成功更新标注记录",
		req: &exptpb.UpdateAnnotateRecordReq{
			WorkspaceID:      validWorkspaceID,
			AnnotateRecordID: validRecordID,
			AnnotateRecords:  &expt.AnnotateRecord{},
		},
		mockSetup: func() {
			// 模拟权限验证
			mockAuth.EXPECT().
				AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).
				Return(nil)

			// 模拟更新记录
			mockAnnotateService.EXPECT().
				UpdateAnnotateRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil)
			mockManager.EXPECT().
				Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&entity.Experiment{}, nil)
		},
		wantResp: &exptpb.UpdateAnnotateRecordResp{
			BaseResp: base.NewBaseResp(),
		},
		wantErr: false,
	}, {
		name: "标注记录不存在",
		req: &exptpb.UpdateAnnotateRecordReq{
			WorkspaceID:      validWorkspaceID,
			AnnotateRecordID: int64(999),
			AnnotateRecords:  &expt.AnnotateRecord{},
		},
		mockSetup: func() {
			// 模拟权限验证
			mockAuth.EXPECT().
				AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).
				Return(nil)

			// 模拟更新记录失败
			mockAnnotateService.EXPECT().
				UpdateAnnotateRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(errorx.NewByCode(errno.ResourceNotFoundCode))
			mockManager.EXPECT().
				Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&entity.Experiment{}, nil)
		},
		wantResp: nil,
		wantErr:  true,
		wantCode: errno.ResourceNotFoundCode,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置mock行为
			tt.mockSetup()

			// 创建被测试对象
			app := &experimentApplication{
				annotateService: mockAnnotateService,
				auth:            mockAuth,
				manager:         mockManager,
			}

			// 执行测试
			gotResp, err := app.UpdateAnnotateRecord(context.Background(), tt.req)

			// 验证结果
			if tt.wantErr {
				assert.Error(t, err)
				statusErr, ok := errorx.FromStatusError(err)
				assert.True(t, ok)
				assert.Equal(t, tt.wantCode, statusErr.Code())
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, gotResp)
		})
	}
}

func TestExperimentApplication_CreateAnnotateRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建mock对象
	mockAnnotateService := servicemocks.NewMockIExptAnnotateService(ctrl)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockIDGen := idgenmock.NewMockIIDGenerator(ctrl)
	mockManager := servicemocks.NewMockIExptManager(ctrl)

	// 测试数据
	validWorkspaceID := int64(123)
	validExptID := int64(456)
	validItemID := int64(789)
	validRecordID := int64(890)

	tests := []struct {
		name      string
		req       *exptpb.CreateAnnotateRecordReq
		mockSetup func()
		wantResp  *exptpb.CreateAnnotateRecordResp
		wantErr   bool
		wantCode  int32
	}{{
		name: "成功创建标注记录",
		req: &exptpb.CreateAnnotateRecordReq{
			WorkspaceID:    validWorkspaceID,
			ExptID:         validExptID,
			ItemID:         validItemID,
			AnnotateRecord: &expt.AnnotateRecord{},
		},
		mockSetup: func() {
			// 模拟权限验证
			mockAuth.EXPECT().
				AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).
				Return(nil)

			// 模拟生成ID
			mockIDGen.EXPECT().
				GenID(gomock.Any()).
				Return(validRecordID, nil)

			// 模拟创建记录
			mockAnnotateService.EXPECT().
				SaveAnnotateRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil)
			mockManager.EXPECT().
				Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&entity.Experiment{}, nil)
		},
		wantResp: &exptpb.CreateAnnotateRecordResp{
			AnnotateRecordID: validRecordID,
			BaseResp:         base.NewBaseResp(),
		},
		wantErr: false,
	}, {
		name: "权限校验失败",
		req: &exptpb.CreateAnnotateRecordReq{
			WorkspaceID: validWorkspaceID,
			ExptID:      validExptID,
			ItemID:      validItemID,
		},
		mockSetup: func() {
			mockManager.EXPECT().
				Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&entity.Experiment{}, nil)
			mockAuth.EXPECT().
				AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).
				Return(errorx.NewByCode(errno.CommonNoPermissionCode))
		},
		wantResp: nil,
		wantErr:  true,
		wantCode: errno.CommonNoPermissionCode,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置mock行为
			tt.mockSetup()

			// 创建被测试对象
			app := &experimentApplication{
				annotateService: mockAnnotateService,
				auth:            mockAuth,
				idgen:           mockIDGen,
				manager:         mockManager,
			}

			// 执行测试
			gotResp, err := app.CreateAnnotateRecord(context.Background(), tt.req)

			// 验证结果
			if tt.wantErr {
				assert.Error(t, err)
				statusErr, ok := errorx.FromStatusError(err)
				if ok {
					assert.Equal(t, tt.wantCode, statusErr.Code())
				}
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, gotResp)
			assert.Equal(t, tt.wantResp.AnnotateRecordID, gotResp.AnnotateRecordID)
		})
	}
}

func TestExperimentApplication_AssociateAnnotationTag(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建mock对象
	mockAnnotateService := servicemocks.NewMockIExptAnnotateService(ctrl)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockManager := servicemocks.NewMockIExptManager(ctrl)

	// 测试数据
	validWorkspaceID := int64(123)
	validExptID := int64(456)
	validKeyTagID := int64(789)

	tests := []struct {
		name      string
		req       *exptpb.AssociateAnnotationTagReq
		mockSetup func()
		wantResp  *exptpb.AssociateAnnotationTagResp
		wantErr   bool
		wantCode  int32
	}{{
		name: "成功关联标注标签",
		req: &exptpb.AssociateAnnotationTagReq{
			WorkspaceID: validWorkspaceID,
			ExptID:      validExptID,
			TagKeyID:    ptr.Of(validKeyTagID),
		},
		mockSetup: func() {
			// 模拟权限验证
			mockAuth.EXPECT().
				AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).
				Return(nil)

			// 模拟关联标签
			mockAnnotateService.EXPECT().
				CreateExptTurnResultTagRefs(gomock.Any(), gomock.Any()).
				Return(nil)
			mockManager.EXPECT().
				Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&entity.Experiment{}, nil)
		},
		wantResp: &exptpb.AssociateAnnotationTagResp{
			BaseResp: base.NewBaseResp(),
		},
		wantErr: false,
	}, {
		name: "标签不存在",
		req: &exptpb.AssociateAnnotationTagReq{
			WorkspaceID: validWorkspaceID,
			ExptID:      validExptID,
			TagKeyID:    ptr.Of(validKeyTagID),
		},
		mockSetup: func() {
			// 模拟权限验证
			mockAuth.EXPECT().
				AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).
				Return(nil)

			// 模拟关联标签失败
			mockAnnotateService.EXPECT().
				CreateExptTurnResultTagRefs(gomock.Any(), gomock.Any()).
				Return(errorx.NewByCode(errno.CommonInternalErrorCode))
			mockManager.EXPECT().
				Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&entity.Experiment{}, nil)
		},
		wantResp: nil,
		wantErr:  true,
		wantCode: errno.CommonInternalErrorCode,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置mock行为
			tt.mockSetup()

			// 创建被测试对象
			app := &experimentApplication{
				annotateService: mockAnnotateService,
				auth:            mockAuth,
				manager:         mockManager,
			}

			// 执行测试
			gotResp, err := app.AssociateAnnotationTag(context.Background(), tt.req)

			// 验证结果
			if tt.wantErr {
				assert.Error(t, err)
				statusErr, ok := errorx.FromStatusError(err)
				assert.True(t, ok)
				assert.Equal(t, tt.wantCode, statusErr.Code())
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, gotResp)
		})
	}
}
