// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	exptpb "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRetryExperimentRuntimeOverrideAdmission(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  entity.ExptStatus
		target  entity.EvalTargetType
		raw     string
		wantErr bool
	}{
		{"completed", entity.ExptStatus_Failed, entity.EvalTargetTypeSandboxAgent, `{"binary_version":"v0.0.250-rc.1"}`, false},
		{"running", entity.ExptStatus_Processing, entity.EvalTargetTypeSandboxAgent, `{"binary_version":"v0.0.250-rc.1"}`, true},
		{"pending", entity.ExptStatus_Pending, entity.EvalTargetTypeSandboxAgent, `{"binary_version":"v0.0.250-rc.1"}`, true},
		{"unsupported target", entity.ExptStatus_Failed, entity.EvalTargetTypeCustomRPCServer, `{"binary_version":"v0.0.250-rc.1"}`, true},
		{"invalid override", entity.ExptStatus_Failed, entity.EvalTargetTypeSandboxAgent, `{"model_name":"changed"}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			manager := servicemocks.NewMockIExptManager(ctrl)
			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			ids := idgenmocks.NewMockIIDGenerator(ctrl)
			app := &experimentApplication{manager: manager, auth: auth, idgen: ids}
			manager.EXPECT().Get(gomock.Any(), int64(2), int64(1), gomock.Any()).Return(&entity.Experiment{ID: 2, SpaceID: 1, Status: tc.status, TargetType: tc.target, EvalConf: &entity.EvaluationConfiguration{ItemRetryNum: gptr.Of(3)}}, nil)
			auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
			ext := map[string]string{entity.RetryRuntimeParamExtKey: tc.raw}
			if !tc.wantErr {
				ids.EXPECT().GenID(gomock.Any()).Return(int64(8), nil)
				manager.EXPECT().LogRun(gomock.Any(), int64(2), int64(8), entity.EvaluationModeFailRetry, int64(1), gomock.Any(), gomock.Any()).Return(nil)
				manager.EXPECT().Run(gomock.Any(), int64(2), int64(8), int64(1), 3, gomock.Any(), entity.EvaluationModeFailRetry, ext).Return(nil)
			}
			resp, err := app.RetryExperiment(context.Background(), &exptpb.RetryExperimentRequest{WorkspaceID: gptr.Of(int64(1)), ExptID: gptr.Of(int64(2)), Ext: ext})
			if tc.wantErr {
				require.Error(t, err)
				require.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.Equal(t, int64(8), resp.GetRunID())
			}
		})
	}
}
