// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

func TestGetExperimentsOpenAPIReadsStoredHooks(t *testing.T) {
	for _, denied := range []bool{false, true} {
		t.Run(map[bool]string{false: "configured", true: "denied"}[denied], func(t *testing.T) {
			ctrl := gomock.NewController(t)
			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			manager := servicemocks.NewMockIExptManager(ctrl)
			detail := &entity.Experiment{ID: 42, SpaceID: 7, CreatedBy: "owner", Name: "test", LatestRunID: 90}
			manager.EXPECT().GetDetail(gomock.Any(), int64(42), int64(7), gomock.Any()).Return(detail, nil)
			var permissionError error
			if denied {
				permissionError = errors.New("denied")
			}
			auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(permissionError)
			auth.EXPECT().MAuthorizeWithoutSPI(gomock.Any(), int64(7), gomock.Any()).Return(nil).AnyTimes()
			storage := &hookApplicationConfigStore{record: entity.HookConfigRecord{Config: &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(`{"retained":true}`)}}}}
			summaries := &hookApplicationSummaryStore{}
			app := &EvalOpenAPIApplication{auth: auth, manager: manager, metric: &fakeOpenAPIMetric{}, experimentApp: &experimentApplication{auth: auth, hooks: &ExperimentHookApplicationDependencies{Configs: storage, Summaries: summaries, ExecutionScope: "ppe-test"}}}
			out, err := app.GetExperimentsOApi(context.Background(), &openapi.GetExperimentsOApiRequest{WorkspaceID: gptr.Of(int64(7)), ExperimentID: gptr.Of(int64(42))})
			if denied {
				require.ErrorIs(t, err, permissionError)
				require.Nil(t, out)
				require.Zero(t, storage.batchCalls)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, out.Data.Experiment.LifecycleHookConf)
			require.Equal(t, `{"retained":true}`, out.Data.Experiment.LifecycleHookConf.Before.GetParametersJSON())
			require.False(t, out.Data.Experiment.LifecycleHookConf.Before.GetEnabled())
			require.Equal(t, int64(42), out.Data.Experiment.GetID())
			require.NotNil(t, out.Data.Experiment.LifecycleHookSummary)
			require.Equal(t, "90", out.Data.Experiment.LifecycleHookSummary.GetRunID())
			require.Equal(t, "running", string(out.Data.Experiment.LifecycleHookSummary.Before.GetStatus()))
			require.Equal(t, "failed", string(out.Data.Experiment.LifecycleHookSummary.After.GetStatus()))
			require.Equal(t, "BUSINESS", out.Data.Experiment.LifecycleHookSummary.After.Error.GetCode())
			require.Equal(t, "ppe-test", storage.owner.ExecutionScope)
			require.Equal(t, 1, summaries.calls)
		})
	}
}
