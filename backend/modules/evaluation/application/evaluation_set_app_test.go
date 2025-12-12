// 新增：CreateEvaluationSetWithImport、ParseImportSourceFile、GetEvaluationSetItemField 单测
package application

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	domain_eval_set "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_set"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_set"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	metricsmock "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
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

	app := &EvaluationSetApplicationImpl{
		auth:                 mockAuth,
		evaluationSetService: mockSvc,
		metric:              mockMetric,
	}

	workspaceID := int64(1001)

	baseReq := func() *eval_set.CreateEvaluationSetWithImportRequest {
		return &eval_set.CreateEvaluationSetWithImportRequest{
			WorkspaceID:         workspaceID,
			Name:                gptr.Of("dataset"),
			EvaluationSetSchema: &domain_eval_set.EvaluationSetSchema{},
			SourceType:          gptr.Of(domain_eval_set.SetSourceType_File),
			Source:              &domain_eval_set.DatasetIOEndpoint{File: &domain_eval_set.DatasetIOFile{}},
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

func TestEvaluationSetApplicationImpl_ParseImportSourceFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockSvc := servicemocks.NewMockIEvaluationSetService(ctrl)

	app := &EvaluationSetApplicationImpl{
		auth:                 mockAuth,
		evaluationSetService: mockSvc,
	}

	workspaceID := int64(2002)

	baseReq := func() *eval_set.ParseImportSourceFileRequest {
		return &eval_set.ParseImportSourceFileRequest{
			WorkspaceID: workspaceID,
			File:        &domain_eval_set.DatasetIOFile{Path: gptr.Of("/path")},
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
			name: "nil file",
			req: func() *eval_set.ParseImportSourceFileRequest { r := baseReq(); r.File = nil; return r }(),
			setup:   func() {},
			wantErr: errno.CommonInvalidParamCode,
		},
		{
			name: "鉴权失败",
			req:  baseReq(),
			setup: func() { mockAuth.EXPECT().Authorization(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationParam{})).Return(errorx.NewByCode(errno.CommonNoPermissionCode)) },
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
					Bytes:        int64(123),
					FieldSchemas: []*entity.FieldSchema{{Name: "f1"}},
					Conflicts:    []*entity.ConflictField{{FieldName: "c1"}},
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
			if tc.setup != nil { tc.setup() }
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
				if tc.check != nil { tc.check(t, resp) }
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
			ItemPk:          gptr.Of(itemPK),
			FieldName:       gptr.Of(fieldName),
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
				mockEvalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gptr.Of(workspaceID), evalSetID, gomock.AssignableToTypeOf(gptr.Of(true))).Return(nil, nil)
			},
			wantErr: errno.ResourceNotFoundCode,
		},
		{
			name: "auth failed",
			req:  baseReq(),
			setup: func() {
				mockEvalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gptr.Of(workspaceID), evalSetID, gomock.AssignableToTypeOf(gptr.Of(true))).Return(validSet, nil)
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr: errno.CommonNoPermissionCode,
		},
		{
			name: "get field error",
			req:  baseReq(),
			setup: func() {
				mockEvalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gptr.Of(workspaceID), evalSetID, gomock.AssignableToTypeOf(gptr.Of(true))).Return(validSet, nil)
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationWithoutSPIParam{})).Return(nil)
				mockItemSvc.EXPECT().GetEvaluationSetItemField(gomock.Any(), gomock.AssignableToTypeOf(&entity.GetEvaluationSetItemFieldParam{})).Return(nil, errors.New("svc err"))
			},
			wantErr: -1,
		},
		{
			name: "成功",
			req:  baseReq(),
			setup: func() {
				mockEvalSetSvc.EXPECT().GetEvaluationSet(gomock.Any(), gptr.Of(workspaceID), evalSetID, gomock.AssignableToTypeOf(gptr.Of(true))).Return(validSet, nil)
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
			if tc.setup != nil { tc.setup() }
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
				if tc.check != nil { tc.check(t, resp) }
			}
		})
	}
}
