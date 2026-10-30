// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
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
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestRunningRetryExplicitNeverFallsBack(t *testing.T) {
	for _, kind := range []string{"zero_expected", "missing_request", "permission_denied"} {
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
				req.RequestID = gptr.Of("72cc8921-40f4-4a43-b772-aaf2b5dcdf05")
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

func TestRunningRetryExplicitReceiptAndLegacySwitch(t *testing.T) {
	for _, kind := range []string{"explicit_sandbox", "old_live_join", "old_terminal_failure", "old_terminal_all", "old_terminal_items", "enabled_missing_identity", "enabled_online_live", "enabled_pending_offline", "enabled_finishing_offline"} {
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
				req.RequestID = gptr.Of("72cc8921-40f4-4a43-b772-aaf2b5dcdf05")
				req.Ext = map[string]string{"caller": "keep"}
				got.Target = &entity.EvalTarget{EvalTargetType: entity.EvalTargetTypeSandboxAgent}
			case "old_live_join", "old_terminal_items":
				req.RetryMode = gptr.Of(domain_expt.ExptRetryMode_RetryTargetItems)
			case "old_terminal_all":
				req.RetryMode = gptr.Of(domain_expt.ExptRetryMode_RetryAll)
			}
			if strings.HasPrefix(kind, "old_terminal") {
				got.Status = entity.ExptStatus_Success
			}
			switch kind {
			case "enabled_online_live":
				got.ExptType = entity.ExptType_Online
			case "enabled_pending_offline":
				got.Status = entity.ExptStatus_Pending
			case "enabled_finishing_offline":
				got.Status = entity.ExptStatus_Terminating
			}
			manager.EXPECT().Get(gomock.Any(), int64(7), int64(9), gomock.Any()).Return(got, nil)
			auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
			switch kind {
			case "explicit_sandbox":
				manager.EXPECT().RetryInCurrentRun(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r *entity.RetryInCurrentRunRequest) (*entity.ManualRetryReceipt, error) {
					require.Equal(t, int64(8), r.ExpectedRunID)
					require.Equal(t, map[string]string{"caller": "keep"}, r.Ext)
					return &entity.ManualRetryReceipt{RequestID: r.RequestID, AcceptedCount: 1, RejectedItems: []*entity.ManualRetryRejectedItem{{ItemID: 12, Reason: "busy"}}, SnapshotAtMS: 1700000000123}, nil
				})
			case "enabled_missing_identity":
				config.EXPECT().GetExptExecConf(gomock.Any(), int64(9)).Return(&entity.ExptExecConf{RunningRetryEnabled: gptr.Of(true)})
			case "old_live_join":
				config.EXPECT().GetExptExecConf(gomock.Any(), int64(9)).Return(&entity.ExptExecConf{})
				manager.EXPECT().LogRetryItemsRun(gomock.Any(), int64(7), entity.EvaluationModeRetryItems, int64(9), []int64{11}, gomock.Any()).Return(int64(8), true, nil)
			case "old_terminal_items":
				manager.EXPECT().LogRetryItemsRun(gomock.Any(), int64(7), entity.EvaluationModeRetryItems, int64(9), []int64{11}, gomock.Any()).Return(int64(18), false, nil)
				manager.EXPECT().RetryItems(gomock.Any(), int64(7), int64(18), int64(9), 3, []int64{11}, gomock.Any(), gomock.Any()).Return(nil)
			default:
				mode := entity.EvaluationModeFailRetry
				if kind == "old_terminal_all" {
					mode = entity.EvaluationModeRetryAll
				}
				generator.EXPECT().GenID(gomock.Any()).Return(int64(18), nil)
				manager.EXPECT().LogRun(gomock.Any(), int64(7), int64(18), mode, int64(9), gomock.Any(), gomock.Any()).Return(nil)
				manager.EXPECT().Run(gomock.Any(), int64(7), int64(18), int64(9), 3, gomock.Any(), mode, gomock.Any()).Return(nil)
			}
			app := &experimentApplication{manager: manager, auth: auth, idgen: generator, configer: config, sandboxSchedulerAdapter: sandbox}
			resp, err := app.RetryExperiment(context.Background(), req)
			if kind == "enabled_missing_identity" {
				statusErr, ok := errorx.FromStatusError(err)
				require.True(t, ok)
				require.EqualValues(t, errno.ManualRetryIdentityRequiredCode, statusErr.Code())
				require.Nil(t, resp)
				return
			}
			require.NoError(t, err)
			if kind == "explicit_sandbox" {
				require.Equal(t, int64(8), resp.GetRunID())
				require.Equal(t, req.GetRequestID(), resp.GetRequestID())
				require.Equal(t, int32(1), resp.GetAcceptedCount())
				require.Equal(t, int64(1700000000123), resp.GetSnapshotAtMs())
				require.Equal(t, "busy", resp.RejectedItems[0].GetReason())
			}
		})
	}
}
