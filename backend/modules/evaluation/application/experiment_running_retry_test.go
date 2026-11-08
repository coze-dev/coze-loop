// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	exptpb "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

func TestRunningRetryExplicitNeverFallsBack(t *testing.T) {
	for _, kind := range []string{"zero_expected", "negative_expected", "without_request_id", "permission_denied"} {
		t.Run(kind, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			manager := svcmocks.NewMockIExptManager(ctrl)
			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			generator := idgenmocks.NewMockIIDGenerator(ctrl)
			got := &entity.Experiment{ID: 7, SpaceID: 9, LatestRunID: 8, CreatedBy: "owner", Status: entity.ExptStatus_Processing, ExptType: entity.ExptType_Offline, EvalConf: &entity.EvaluationConfiguration{ItemRetryNum: gptr.Of(3)}}
			manager.EXPECT().Get(gomock.Any(), int64(7), int64(9), gomock.Any()).Return(got, nil)
			var denied error
			if kind == "permission_denied" {
				denied = errors.New("permission denied")
			}
			auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *rpc.AuthorizationWithoutSPIParam) error {
				require.Equal(t, "7", p.ObjectID)
				require.Equal(t, int64(9), p.ResourceSpaceID)
				require.Equal(t, "owner", *p.OwnerID)
				return denied
			})
			req := &exptpb.RetryExperimentRequest{ExptID: gptr.Of(int64(7)), WorkspaceID: gptr.Of(int64(9)), ExpectedRunID: gptr.Of(int64(8))}
			if kind == "zero_expected" {
				req.ExpectedRunID = gptr.Of(int64(0))
			}
			if kind == "negative_expected" {
				req.ExpectedRunID = gptr.Of(int64(-1))
			}
			app := &experimentApplication{manager: manager, auth: auth, idgen: generator}
			response, err := app.RetryExperiment(context.Background(), req)

			require.Error(t, err)
			require.Nil(t, response)
			if denied != nil {
				require.ErrorIs(t, err, denied)
			}
		})
	}
}

// Legacy 入口不通过执行配置选择重试语义。
func TestLegacyRetryAdmissionIsConfigIndependent(t *testing.T) {
	for _, configKind := range []string{"config_nil", "legacy_false", "legacy_true"} {
		for _, mode := range []struct {
			retryMode domain_expt.ExptRetryMode
			runMode   entity.ExptRunMode
		}{
			{domain_expt.ExptRetryMode_RetryFailure, entity.EvaluationModeFailRetry},
			{domain_expt.ExptRetryMode_RetryTargetItems, entity.EvaluationModeRetryItems},
		} {
			t.Run(configKind+"_"+mode.retryMode.String(), func(t *testing.T) {
				ctrl := gomock.NewController(t)
				manager := svcmocks.NewMockIExptManager(ctrl)
				auth := rpcmocks.NewMockIAuthProvider(ctrl)
				config := componentmocks.NewMockIConfiger(ctrl)
				generator := idgenmocks.NewMockIIDGenerator(ctrl)
				got := &entity.Experiment{ID: 7, SpaceID: 9, LatestRunID: 8, CreatedBy: "owner", Status: entity.ExptStatus_Processing, ExptType: entity.ExptType_Offline, EvalConf: &entity.EvaluationConfiguration{ItemRetryNum: gptr.Of(3)}}
				req := &exptpb.RetryExperimentRequest{
					WorkspaceID: gptr.Of(int64(9)), ExptID: gptr.Of(int64(7)),
					RetryMode: gptr.Of(mode.retryMode), ItemIds: []int64{11}, Ext: map[string]string{"caller": "keep"},
				}
				manager.EXPECT().Get(gomock.Any(), int64(7), int64(9), gomock.Any()).Return(got, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
				if mode.runMode == entity.EvaluationModeRetryItems {
					manager.EXPECT().LogRetryItemsRun(gomock.Any(), int64(7), mode.runMode, int64(9), []int64{11}, gomock.Any()).Return(int64(8), true, nil)
				} else {
					generator.EXPECT().GenID(gomock.Any()).Return(int64(8), nil)
					manager.EXPECT().LogRun(gomock.Any(), int64(7), int64(8), mode.runMode, int64(9), gomock.Any(), gomock.Any()).Return(nil)
					manager.EXPECT().Run(gomock.Any(), int64(7), int64(8), int64(9), 3, gomock.Any(), mode.runMode, req.Ext).Return(nil)
				}
				// 任何对执行配置的读取都会让 mock 直接失败，证明门禁与配置无关。
				config.EXPECT().GetExptExecConf(gomock.Any(), gomock.Any()).Times(0)
				app := &experimentApplication{manager: manager, auth: auth, configer: config, idgen: generator}
				if configKind == "config_nil" {
					app.configer = nil
				}
				resp, err := app.RetryExperiment(context.Background(), req)
				require.NoError(t, err)
				require.Equal(t, int64(8), resp.GetRunID())
				require.Nil(t, resp.AcceptedCount)
				require.Nil(t, resp.SnapshotAtMs)
				require.NotContains(t, fmt.Sprintf("%+v", resp), "RequestID")
			})
		}
	}
}

func TestRunningRetryLegacyFallbacks(t *testing.T) {
	for _, kind := range []string{"explicit_sandbox", "old_terminal_failure", "old_terminal_all", "old_terminal_items", "processing_without_identity", "online_live", "pending_offline", "finishing_offline", "processing_retry_all", "explicit_retry_all"} {
		t.Run(kind, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			manager := svcmocks.NewMockIExptManager(ctrl)
			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			config := componentmocks.NewMockIConfiger(ctrl)
			generator := idgenmocks.NewMockIIDGenerator(ctrl)
			sandbox := rpcmocks.NewMockISandboxSchedulerAdapter(ctrl)
			got := &entity.Experiment{ID: 7, SpaceID: 9, LatestRunID: 8, CreatedBy: "owner", Status: entity.ExptStatus_Processing, ExptType: entity.ExptType_Offline, EvalConf: &entity.EvaluationConfiguration{ItemRetryNum: gptr.Of(3)}}
			req := &exptpb.RetryExperimentRequest{WorkspaceID: gptr.Of(int64(9)), ExptID: gptr.Of(int64(7)), ItemIds: []int64{11}}
			switch kind {
			case "explicit_sandbox":
				req.ExpectedRunID = gptr.Of(int64(8))
				req.Ext = map[string]string{"caller": "keep"}
				got.Target = &entity.EvalTarget{EvalTargetType: entity.EvalTargetTypeSandboxAgent}
			case "explicit_retry_all":
				// 显式并发保护 + RetryAll：必须 fail-closed，不得静默起新 run。
				req.ExpectedRunID = gptr.Of(int64(8))
				req.RetryMode = gptr.Of(domain_expt.ExptRetryMode_RetryAll)
				got.Target = &entity.EvalTarget{EvalTargetType: entity.EvalTargetTypeSandboxAgent}
			case "old_terminal_items":
				req.RetryMode = gptr.Of(domain_expt.ExptRetryMode_RetryTargetItems)
			case "old_terminal_all", "processing_retry_all":
				req.RetryMode = gptr.Of(domain_expt.ExptRetryMode_RetryAll)
			}
			if strings.HasPrefix(kind, "old_terminal") {
				got.Status = entity.ExptStatus_Success
			}
			switch kind {
			case "online_live":
				got.ExptType = entity.ExptType_Online
			case "pending_offline":
				got.Status = entity.ExptStatus_Pending
			case "finishing_offline":
				got.Status = entity.ExptStatus_Terminating
			}
			manager.EXPECT().Get(gomock.Any(), int64(7), int64(9), gomock.Any()).Return(got, nil)
			auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
			switch kind {
			case "explicit_sandbox":
			case "old_terminal_items":
				manager.EXPECT().LogRetryItemsRun(gomock.Any(), int64(7), entity.EvaluationModeRetryItems, int64(9), []int64{11}, gomock.Any()).Return(int64(18), false, nil)
				manager.EXPECT().RetryItems(gomock.Any(), int64(7), int64(18), int64(9), 3, []int64{11}, gomock.Any(), gomock.Any()).Return(nil)
			default:
				if kind == "explicit_retry_all" {
					// 仅验证拒绝分支：没有任何 GenID/LogRun/Run 期望。
				} else {
					mode := entity.EvaluationModeFailRetry
					if kind == "old_terminal_all" || kind == "processing_retry_all" {
						mode = entity.EvaluationModeRetryAll
					}
					generator.EXPECT().GenID(gomock.Any()).Return(int64(18), nil)
					manager.EXPECT().LogRun(gomock.Any(), int64(7), int64(18), mode, int64(9), gomock.Any(), gomock.Any()).Return(nil)
					manager.EXPECT().Run(gomock.Any(), int64(7), int64(18), int64(9), 3, gomock.Any(), mode, gomock.Any()).Return(nil)
				}
			}
			app := &experimentApplication{manager: manager, auth: auth, idgen: generator, configer: config, sandboxSchedulerAdapter: sandbox}
			resp, err := app.RetryExperiment(context.Background(), req)
			if req.ExpectedRunID != nil {
				require.Error(t, err)
				require.Nil(t, resp)
				return
			}
			require.NoError(t, err)
			require.Equal(t, int64(18), resp.GetRunID())
		})
	}
}
