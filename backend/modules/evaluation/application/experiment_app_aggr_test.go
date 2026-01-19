// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestExperimentApplication_CalculateExperimentAggrResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockManager := servicemocks.NewMockIExptManager(ctrl)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockAggrSvc := servicemocks.NewMockExptAggrResultService(ctrl)

	app := &experimentApplication{
		manager:               mockManager,
		auth:                  mockAuth,
		ExptAggrResultService: mockAggrSvc,
	}

	ctx := context.Background()
	workspaceID := int64(100)
	exptID := int64(200)

	req := &expt.CalculateExperimentAggrResultRequest{
		WorkspaceID: workspaceID,
		ExptID:      exptID,
	}

	tests := []struct {
		name    string
		setup   func()
		wantErr bool
		errCode int32
	}{
		{
			name: "Calculate aggregation result successfully",
			setup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockManager.EXPECT().Get(gomock.Any(), exptID, workspaceID, gomock.Any()).Return(&entity.Experiment{
					ID:     exptID,
					Status: entity.ExptStatus_Success,
				}, nil)
				mockAggrSvc.EXPECT().PublishExptAggrResultEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "Authorization failed",
			setup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr: true,
			errCode: errno.CommonNoPermissionCode,
		},
		{
			name: "Experiment not finished",
			setup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockManager.EXPECT().Get(gomock.Any(), exptID, workspaceID, gomock.Any()).Return(&entity.Experiment{
					ID:     exptID,
					Status: entity.ExptStatus_Processing,
				}, nil)
			},
			wantErr: true,
			errCode: errno.IncompleteExptCalcAggrResultErrorCode,
		},
		{
			name: "Publish event failed",
			setup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockManager.EXPECT().Get(gomock.Any(), exptID, workspaceID, gomock.Any()).Return(&entity.Experiment{
					ID:     exptID,
					Status: entity.ExptStatus_Success,
				}, nil)
				mockAggrSvc.EXPECT().PublishExptAggrResultEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(errorx.NewByCode(500))
			},
			wantErr: true,
			errCode: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			resp, err := app.CalculateExperimentAggrResult_(ctx, req)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.errCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			}
		})
	}
}
