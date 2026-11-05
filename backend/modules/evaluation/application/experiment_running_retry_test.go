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
			if kind == "zero_expected" || kind == "without_request_id" {
				if kind == "zero_expected" {
					req.ExpectedRunID = gptr.Of(int64(0))
				}
				manager.EXPECT().RetryInCurrentRun(gomock.Any(), &entity.RetryInCurrentRunRequest{
					SpaceID: 9, ExptID: 7, ExpectedRunID: req.GetExpectedRunID(), Mode: entity.EvaluationModeFailRetry,
					Session: entity.NewSession(context.Background()),
				}).Return(&entity.ManualRetryReceipt{RunID: 8, AcceptedCount: 1}, nil)
			}
			if kind == "negative_expected" {
				req.ExpectedRunID = gptr.Of(int64(-1))
			}
			app := &experimentApplication{manager: manager, auth: auth, idgen: generator}
			response, err := app.RetryExperiment(context.Background(), req)
			if kind == "zero_expected" || kind == "without_request_id" {
				require.NoError(t, err)
				require.NotNil(t, response.RunID)
				require.Equal(t, int64(8), response.GetRunID())
				require.Empty(t, response.GetRejectedItems())
				require.Equal(t, int32(1), response.GetAcceptedCount())
				return
			}
			require.Error(t, err)
			require.Nil(t, response)
			if denied != nil {
				require.ErrorIs(t, err, denied)
			}
		})
	}
}

func TestRunningRetryWithoutIdentityUsesResolvedRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	manager := svcmocks.NewMockIExptManager(ctrl)
	auth := rpcmocks.NewMockIAuthProvider(ctrl)
	ctx := context.Background()
	session := entity.NewSession(ctx)
	req := &exptpb.RetryExperimentRequest{
		WorkspaceID: gptr.Of(int64(9)), ExptID: gptr.Of(int64(7)),
		RetryMode: gptr.Of(domain_expt.ExptRetryMode_RetryTargetItems), ItemIds: []int64{11},
		Ext: map[string]string{"caller": "keep"},
	}
	manager.EXPECT().Get(ctx, int64(7), int64(9), session).Return(&entity.Experiment{
		ID: 7, SpaceID: 9, LatestRunID: 8, CreatedBy: "owner", Status: entity.ExptStatus_Processing, ExptType: entity.ExptType_Offline,
	}, nil)
	auth.EXPECT().AuthorizationWithoutSPI(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, p *rpc.AuthorizationWithoutSPIParam) error {
		require.Equal(t, "7", p.ObjectID)
		require.Equal(t, int64(9), p.SpaceID)
		require.Equal(t, int64(9), p.ResourceSpaceID)
		require.Equal(t, "owner", *p.OwnerID)
		return nil
	})
	manager.EXPECT().RetryInCurrentRun(ctx, &entity.RetryInCurrentRunRequest{
		SpaceID: 9, ExptID: 7, ExpectedRunID: 0,
		Mode: entity.EvaluationModeRetryItems, ItemIDs: []int64{11}, Ext: req.Ext, Session: session,
	}).Return(&entity.ManualRetryReceipt{RunID: 18, AcceptedCount: 1, SnapshotAtMS: 1700000000123}, nil)

	app := &experimentApplication{manager: manager, auth: auth}
	resp, err := app.RetryExperiment(ctx, req)
	require.NoError(t, err)
	require.Nil(t, req.ExpectedRunID)
	require.NotNil(t, resp.RunID)
	// The manager may resolve a newer run than the application's initial read.
	require.Equal(t, int64(18), resp.GetRunID())
	require.NotContains(t, fmt.Sprintf("%+v", resp), "RequestID")
	require.Equal(t, int32(1), resp.GetAcceptedCount())
	require.Equal(t, int64(1700000000123), resp.GetSnapshotAtMs())
	require.Empty(t, resp.RejectedItems)
}

func TestRunningRetryExplicitReceiptAndLegacySwitch(t *testing.T) {
	for _, kind := range []string{"explicit_sandbox", "old_live_join", "old_terminal_failure", "old_terminal_all", "old_terminal_items", "default_without_identity", "online_live", "pending_offline", "finishing_offline", "processing_retry_all", "explicit_retry_all"} {
		t.Run(kind, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			manager := svcmocks.NewMockIExptManager(ctrl)
			auth := rpcmocks.NewMockIAuthProvider(ctrl)
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
			case "old_live_join", "old_terminal_items":
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
			case "explicit_sandbox", "default_without_identity", "old_live_join":
				manager.EXPECT().RetryInCurrentRun(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r *entity.RetryInCurrentRunRequest) (*entity.ManualRetryReceipt, error) {
					want := &entity.RetryInCurrentRunRequest{SpaceID: 9, ExptID: 7, Mode: entity.EvaluationModeFailRetry, ItemIDs: []int64{11}, Session: entity.NewSession(context.Background())}
					if kind == "old_live_join" {
						want.Mode = entity.EvaluationModeRetryItems
					}
					if kind == "explicit_sandbox" {
						want.ExpectedRunID = 8
						want.Ext = map[string]string{"caller": "keep"}
					}
					require.Equal(t, want, r)
					return &entity.ManualRetryReceipt{RunID: 8, AcceptedCount: 1, RejectedItems: []*entity.ManualRetryRejectedItem{{ItemID: 12, Reason: "busy"}}, SnapshotAtMS: 1700000000123}, nil
				})
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
			app := &experimentApplication{manager: manager, auth: auth, idgen: generator, sandboxSchedulerAdapter: sandbox}
			resp, err := app.RetryExperiment(context.Background(), req)
			if kind == "explicit_retry_all" {
				require.Error(t, err)
				require.Nil(t, resp)
				return
			}
			require.NoError(t, err)
			if kind == "explicit_sandbox" || kind == "default_without_identity" || kind == "old_live_join" {
				require.Equal(t, int64(8), resp.GetRunID())
				require.NotContains(t, fmt.Sprintf("%+v", resp), "RequestID")
				require.Equal(t, int32(1), resp.GetAcceptedCount())
				require.Equal(t, int64(1700000000123), resp.GetSnapshotAtMs())
				require.Equal(t, "busy", resp.RejectedItems[0].GetReason())
			}
		})
	}
}
