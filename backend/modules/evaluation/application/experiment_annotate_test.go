// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	mock_idgen "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	mock_rpc "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	mock_service "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

func TestExperimentApplication_AssociateAnnotationTag(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockManager := mock_service.NewMockIExptManager(ctrl)
	mockAuth := mock_rpc.NewMockIAuthProvider(ctrl)
	mockAnnotateService := mock_service.NewMockIExptAnnotateService(ctrl)

	app := &experimentApplication{
		manager:         mockManager,
		auth:            mockAuth,
		annotateService: mockAnnotateService,
	}

	tests := []struct {
		name        string
		req         *expt.AssociateAnnotationTagReq
		mockSetup   func()
		expectedErr error
	}{
		{
			name: "成功案例",
			req: &expt.AssociateAnnotationTagReq{
				WorkspaceID: 123,
				ExptID:      456,
				TagKeyID:    789,
			},
			mockSetup: func() {
				mockManager.EXPECT().Get(gomock.Any(), int64(456), int64(123), gomock.Any()).Return(&entity.Experiment{
					CreatedBy: "user123",
				}, nil)
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
				mockAnnotateService.EXPECT().CreateExptTurnResultTagRefs(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name: "获取实验失败",
			req: &expt.AssociateAnnotationTagReq{
				WorkspaceID: 123,
				ExptID:      456,
				TagKeyID:    789,
			},
			mockSetup: func() {
				mockManager.EXPECT().Get(gomock.Any(), int64(456), int64(123), gomock.Any()).Return(nil, errors.New("experiment not found"))
			},
			expectedErr: errors.New("experiment not found"),
		},
		{
			name: "鉴权失败",
			req: &expt.AssociateAnnotationTagReq{
				WorkspaceID: 123,
				ExptID:      456,
				TagKeyID:    789,
			},
			mockSetup: func() {
				mockManager.EXPECT().Get(gomock.Any(), int64(456), int64(123), gomock.Any()).Return(&entity.Experiment{
					CreatedBy: "user123",
				}, nil)
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(errors.New("auth failed"))
			},
			expectedErr: errors.New("auth failed"),
		},
		{
			name: "创建标签关联失败",
			req: &expt.AssociateAnnotationTagReq{
				WorkspaceID: 123,
				ExptID:      456,
				TagKeyID:    789,
			},
			mockSetup: func() {
				mockManager.EXPECT().Get(gomock.Any(), int64(456), int64(123), gomock.Any()).Return(&entity.Experiment{
					CreatedBy: "user123",
				}, nil)
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
				mockAnnotateService.EXPECT().CreateExptTurnResultTagRefs(gomock.Any(), gomock.Any()).Return(errors.New("create failed"))
			},
			expectedErr: errors.New("create failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.mockSetup()

			result, err := app.AssociateAnnotationTag(context.Background(), tt.req)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr.Error())
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.NotNil(t, result.BaseResp)
			}
		})
	}
}

func TestExperimentApplication_CreateAnnotateRecord(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockManager := mock_service.NewMockIExptManager(ctrl)
	mockAuth := mock_rpc.NewMockIAuthProvider(ctrl)
	mockAnnotateService := mock_service.NewMockIExptAnnotateService(ctrl)
	mockIDGen := mock_idgen.NewMockIIDGenerator(ctrl)

	app := &experimentApplication{
		manager:         mockManager,
		auth:            mockAuth,
		annotateService: mockAnnotateService,
		idgen:           mockIDGen,
	}

	tests := []struct {
		name        string
		req         *expt.CreateAnnotateRecordReq
		mockSetup   func()
		expectedErr error
		expected    *expt.CreateAnnotateRecordResp
	}{
		{
			name: "成功案例",
			req: &expt.CreateAnnotateRecordReq{
				WorkspaceID: 123,
				ExptID:      456,
				ItemID:      789,
				TurnID:      101112,
				AnnotateRecord: &expt.AnnotateRecord{
					TagKeyID:   gptr.Of(int64(131415)),
					TagValueID: gptr.Of(int64(161718)),
					PlainText:  gptr.Of("test annotation"),
				},
			},
			mockSetup: func() {
				mockManager.EXPECT().Get(gomock.Any(), int64(456), int64(123), gomock.Any()).Return(&entity.Experiment{
					CreatedBy: "user123",
				}, nil)
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(999), nil)
				mockAnnotateService.EXPECT().SaveAnnotateRecord(gomock.Any(), int64(456), int64(789), int64(101112), gomock.Any()).Return(nil)
			},
			expected: &expt.CreateAnnotateRecordResp{
				AnnotateRecordID: gptr.Of(int64(999)),
			},
		},
		{
			name: "获取实验失败",
			req: &expt.CreateAnnotateRecordReq{
				WorkspaceID: 123,
				ExptID:      456,
			},
			mockSetup: func() {
				mockManager.EXPECT().Get(gomock.Any(), int64(456), int64(123), gomock.Any()).Return(nil, errors.New("experiment not found"))
			},
			expectedErr: errors.New("experiment not found"),
		},
		{
			name: "生成ID失败",
			req: &expt.CreateAnnotateRecordReq{
				WorkspaceID: 123,
				ExptID:      456,
				AnnotateRecord: &expt.AnnotateRecord{
					TagKeyID: gptr.Of(int64(131415)),
				},
			},
			mockSetup: func() {
				mockManager.EXPECT().Get(gomock.Any(), int64(456), int64(123), gomock.Any()).Return(&entity.Experiment{
					CreatedBy: "user123",
				}, nil)
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(0), errors.New("gen id failed"))
			},
			expectedErr: errors.New("gen id failed"),
		},
		{
			name: "保存记录失败",
			req: &expt.CreateAnnotateRecordReq{
				WorkspaceID: 123,
				ExptID:      456,
				ItemID:      789,
				TurnID:      101112,
				AnnotateRecord: &expt.AnnotateRecord{
					TagKeyID: gptr.Of(int64(131415)),
				},
			},
			mockSetup: func() {
				mockManager.EXPECT().Get(gomock.Any(), int64(456), int64(123), gomock.Any()).Return(&entity.Experiment{
					CreatedBy: "user123",
				}, nil)
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(999), nil)
				mockAnnotateService.EXPECT().SaveAnnotateRecord(gomock.Any(), int64(456), int64(789), int64(101112), gomock.Any()).Return(errors.New("save failed"))
			},
			expectedErr: errors.New("save failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.mockSetup()

			result, err := app.CreateAnnotateRecord(context.Background(), tt.req)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr.Error())
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				if tt.expected != nil && tt.expected.AnnotateRecordID != nil {
					assert.Equal(t, *tt.expected.AnnotateRecordID, *result.AnnotateRecordID)
				}
			}
		})
	}
}