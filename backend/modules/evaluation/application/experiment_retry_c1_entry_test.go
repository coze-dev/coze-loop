// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	domainexpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	openapiexpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"
	exptpb "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func retryC1Entry(t *testing.T, public bool, app *experimentApplication, auth *rpcmocks.MockIAuthProvider, req *exptpb.RetryExperimentRequest) (int64, error) {
	t.Helper()
	if !public {
		resp, err := app.RetryExperiment(context.Background(), req)
		if err != nil {
			require.Nil(t, resp)
			return 0, err
		}
		require.NotNil(t, resp)
		require.Nil(t, resp.AcceptedCount)
		require.Nil(t, resp.SnapshotAtMs)
		return resp.GetRunID(), nil
	}
	auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
	var mode *openapiexpt.ExptRetryMode
	if req.RetryMode != nil {
		modes := map[domainexpt.ExptRetryMode]openapiexpt.ExptRetryMode{
			domainexpt.ExptRetryMode_RetryFailure:     openapiexpt.ExptRetryModeRetryFailure,
			domainexpt.ExptRetryMode_RetryTargetItems: openapiexpt.ExptRetryModeRetryTargetItems,
			domainexpt.ExptRetryMode_RetryAll:         openapiexpt.ExptRetryModeRetryAll,
		}
		mode = gptr.Of(modes[*req.RetryMode])
	}
	api := &EvalOpenAPIApplication{auth: auth, experimentApp: app, metric: &fakeOpenAPIMetric{}}
	resp, err := api.RetryExperimentOApi(context.Background(), &openapi.RetryExperimentOApiRequest{
		WorkspaceID: req.WorkspaceID, ExperimentID: req.ExptID, RetryMode: mode,
		ItemIds: req.ItemIds, ExpectedRunID: req.ExpectedRunID, Ext: req.Ext,
	})
	if err != nil {
		require.Nil(t, resp)
		return 0, err
	}
	require.NotNil(t, resp.Data)
	require.Nil(t, resp.Data.AcceptedCount)
	require.Nil(t, resp.Data.SnapshotAtMs)
	return resp.Data.GetRunID(), nil
}

func TestRetryC1LegacyEntry(t *testing.T) {
	for _, public := range []bool{false, true} {
		for _, status := range []entity.ExptStatus{entity.ExptStatus_Processing, entity.ExptStatus_Success, entity.ExptStatus_Failed, entity.ExptStatus_Terminated} {
			for _, mode := range []struct {
				name string
				api  *domainexpt.ExptRetryMode
				run  entity.ExptRunMode
			}{
				{"default", nil, entity.EvaluationModeFailRetry},
				{"failure", gptr.Of(domainexpt.ExptRetryMode_RetryFailure), entity.EvaluationModeFailRetry},
				{"items", gptr.Of(domainexpt.ExptRetryMode_RetryTargetItems), entity.EvaluationModeRetryItems},
				{"all", gptr.Of(domainexpt.ExptRetryMode_RetryAll), entity.EvaluationModeRetryAll},
			} {
				t.Run(fmt.Sprintf("public=%t/status=%d/%s", public, status, mode.name), func(t *testing.T) {
					ctrl := gomock.NewController(t)
					manager := svcmocks.NewMockIExptManager(ctrl)
					auth := rpcmocks.NewMockIAuthProvider(ctrl)
					generator := idgenmocks.NewMockIIDGenerator(ctrl)
					manager.EXPECT().Get(gomock.Any(), int64(7), int64(9), gomock.Any()).Return(&entity.Experiment{
						ID: 7, SpaceID: 9, LatestRunID: 8, CreatedBy: "owner", ExptType: entity.ExptType_Offline, Status: status,
						EvalConf: &entity.EvaluationConfiguration{ItemRetryNum: gptr.Of(3)},
					}, nil)
					auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
					var admissionErr error
					if status == entity.ExptStatus_Processing {
						admissionErr = errorx.NewByCode(errno.ExperimentRunningExistedCode)
					}
					if mode.run == entity.EvaluationModeRetryItems {
						manager.EXPECT().LogRetryItemsRun(gomock.Any(), int64(7), mode.run, int64(9), []int64{11}, gomock.Any()).Return(int64(18), false, admissionErr)
						if admissionErr == nil {
							manager.EXPECT().RetryItems(gomock.Any(), int64(7), int64(18), int64(9), 3, []int64{11}, gomock.Any(), map[string]string{"caller": "legacy"}).Return(nil)
						}
					} else {
						generator.EXPECT().GenID(gomock.Any()).Return(int64(18), nil)
						manager.EXPECT().LogRun(gomock.Any(), int64(7), int64(18), mode.run, int64(9), nil, gomock.Any()).Return(admissionErr)
						if admissionErr == nil {
							manager.EXPECT().Run(gomock.Any(), int64(7), int64(18), int64(9), 3, gomock.Any(), mode.run, map[string]string{"caller": "legacy"}).Return(nil)
						}
					}
					app := &experimentApplication{manager: manager, auth: auth, idgen: generator}
					runID, err := retryC1Entry(t, public, app, auth, &exptpb.RetryExperimentRequest{WorkspaceID: gptr.Of(int64(9)), ExptID: gptr.Of(int64(7)), RetryMode: mode.api, ItemIds: []int64{11}, Ext: map[string]string{"caller": "legacy"}})
					if admissionErr != nil {
						require.ErrorIs(t, err, admissionErr)
						return
					}
					require.NoError(t, err)
					require.Equal(t, int64(18), runID)
				})
			}
		}
	}
}

func TestRetryC1ExplicitIdentityRejectsBeforeWrites(t *testing.T) {
	for _, public := range []bool{false, true} {
		for _, expected := range []int64{8, 0, -1} {
			for _, mode := range []domainexpt.ExptRetryMode{0, domainexpt.ExptRetryMode_RetryFailure, domainexpt.ExptRetryMode_RetryTargetItems, domainexpt.ExptRetryMode_RetryAll} {
				for _, status := range []entity.ExptStatus{entity.ExptStatus_Processing, entity.ExptStatus_Success} {
					t.Run(fmt.Sprintf("public=%t/expected=%d/mode=%d/status=%d", public, expected, mode, status), func(t *testing.T) {
						ctrl := gomock.NewController(t)
						manager := svcmocks.NewMockIExptManager(ctrl)
						auth := rpcmocks.NewMockIAuthProvider(ctrl)
						generator := idgenmocks.NewMockIIDGenerator(ctrl)
						sandbox := rpcmocks.NewMockISandboxSchedulerAdapter(ctrl)
						manager.EXPECT().Get(gomock.Any(), int64(7), int64(9), gomock.Any()).Return(&entity.Experiment{
							ID: 7, SpaceID: 9, LatestRunID: 8, CreatedBy: "owner", ExptType: entity.ExptType_Offline, Status: status,
							TargetType: entity.EvalTargetTypeSandboxAgent, EvalConf: &entity.EvaluationConfiguration{ItemRetryNum: gptr.Of(3)},
							Target: &entity.EvalTarget{EvalTargetType: entity.EvalTargetTypeSandboxAgent, EvalTargetVersion: &entity.EvalTargetVersion{}},
						}, nil)
						auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
						app := &experimentApplication{manager: manager, auth: auth, idgen: generator, sandboxSchedulerAdapter: sandbox}
						req := &exptpb.RetryExperimentRequest{WorkspaceID: gptr.Of(int64(9)), ExptID: gptr.Of(int64(7)), ExpectedRunID: gptr.Of(expected), ItemIds: []int64{11}}
						if mode != 0 {
							req.RetryMode = gptr.Of(mode)
						}
						if mode != domainexpt.ExptRetryMode_RetryTargetItems {
							req.ItemIds = nil
						}
						_, err := retryC1Entry(t, public, app, auth, req)
						require.Error(t, err)
						statusErr, ok := errorx.FromStatusError(err)
						require.True(t, ok)
						require.Equal(t, int32(errno.CommonInvalidParamCode), statusErr.Code())
						require.ErrorContains(t, err, "expected_run_id")
					})
				}
			}
		}
	}
}

func TestRetryC1LegacyReinitBeforeAdmissionRejection(t *testing.T) {
	ctrl := gomock.NewController(t)
	manager := svcmocks.NewMockIExptManager(ctrl)
	auth := rpcmocks.NewMockIAuthProvider(ctrl)
	generator := idgenmocks.NewMockIIDGenerator(ctrl)
	sandbox := rpcmocks.NewMockISandboxSchedulerAdapter(ctrl)
	manager.EXPECT().Get(gomock.Any(), int64(7), int64(9), gomock.Any()).Return(&entity.Experiment{
		ID: 7, SpaceID: 9, LatestRunID: 8, CreatedBy: "owner", ExptType: entity.ExptType_Offline, Status: entity.ExptStatus_Processing,
		TargetType: entity.EvalTargetTypeSandboxAgent, EvalConf: &entity.EvaluationConfiguration{ItemRetryNum: gptr.Of(3)},
		Target: &entity.EvalTarget{EvalTargetType: entity.EvalTargetTypeSandboxAgent, EvalTargetVersion: &entity.EvalTargetVersion{}},
	}, nil)
	auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
	rejected := errorx.NewByCode(errno.ExperimentRunningExistedCode)
	gomock.InOrder(
		sandbox.EXPECT().Init(gomock.Any(), &rpc.SandboxInitRequest{TaskID: "7", Concurrency: 25, WorkspaceID: 9}).Return(&rpc.SandboxInitResponse{}, nil),
		generator.EXPECT().GenID(gomock.Any()).Return(int64(18), nil),
		manager.EXPECT().LogRun(gomock.Any(), int64(7), int64(18), entity.EvaluationModeFailRetry, int64(9), nil, gomock.Any()).Return(rejected),
	)
	app := &experimentApplication{manager: manager, auth: auth, idgen: generator, sandboxSchedulerAdapter: sandbox}
	_, err := retryC1Entry(t, false, app, auth, &exptpb.RetryExperimentRequest{WorkspaceID: gptr.Of(int64(9)), ExptID: gptr.Of(int64(7))})
	require.ErrorIs(t, err, rejected)
}

func TestRetryC1LegacyNumericJoin(t *testing.T) {
	for _, public := range []bool{false, true} {
		for _, kind := range []string{"join", "duplicate", "completing", "ordinary_holder", "ordinary_run"} {
			t.Run(fmt.Sprintf("public=%t/%s", public, kind), func(t *testing.T) {
				ctrl := gomock.NewController(t)
				manager := svcmocks.NewMockIExptManager(ctrl)
				auth := rpcmocks.NewMockIAuthProvider(ctrl)
				generator := idgenmocks.NewMockIIDGenerator(ctrl)
				locker := lockmocks.NewMockILocker(ctrl)
				config := configmocks.NewMockIConfiger(ctrl)
				experiments := repomocks.NewMockIExperimentRepo(ctrl)
				runs := repomocks.NewMockIExptRunLogRepo(ctrl)
				finish := repomocks.NewMockIExptRunFinishRepo(ctrl)
				got := &entity.Experiment{ID: 7, SpaceID: 9, LatestRunID: 8, CreatedBy: "owner", ExptType: entity.ExptType_Offline, Status: entity.ExptStatus_Processing}
				manager.EXPECT().Get(gomock.Any(), int64(7), int64(9), gomock.Any()).Return(got, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
				experiments.EXPECT().GetByID(gomock.Any(), int64(7), int64(9)).Return(got, nil).Times(2)
				config.EXPECT().GetExptExecConf(gomock.Any(), int64(9)).Return(&entity.ExptExecConf{})
				generator.EXPECT().GenID(gomock.Any()).Return(int64(18), nil)
				tickHeld := false
				locker.EXPECT().LockWithRenew(gomock.Any(), "expt_run_exec_lock:7:8", 5*time.Second, 5*time.Minute).DoAndReturn(func(ctx context.Context, _ string, _, _ time.Duration) (bool, context.Context, func(), error) {
					tickHeld = true
					return true, ctx, func() { tickHeld = false }, nil
				})
				holder := "8"
				if kind == "ordinary_holder" {
					holder = "evaluation"
				}
				locker.EXPECT().BackoffLockWithValue(gomock.Any(), "expt_run_mutex_lock:7", "18", gomock.Any(), time.Second).DoAndReturn(func(context.Context, string, string, time.Duration, time.Duration) (bool, string, error) {
					require.True(t, tickHeld)
					return false, holder, nil
				})
				run := &entity.ExptRunLog{ID: 8, SpaceID: 9, ExptID: 7, ExptRunID: 8, Mode: int32(entity.EvaluationModeRetryItems), Status: int64(entity.ExptStatus_Processing), ItemIds: []entity.ExptRunLogItems{{ItemIDs: []int64{10}}}}
				if kind == "ordinary_run" {
					run.Mode = int32(entity.EvaluationModeSubmit)
				}
				if kind == "duplicate" {
					run.ItemIds[0].ItemIDs = []int64{10, 11}
				}
				if kind != "ordinary_holder" {
					runs.EXPECT().Get(gomock.Any(), int64(7), int64(8)).Return(run, nil)
				}
				if kind != "ordinary_holder" && kind != "ordinary_run" {
					locker.EXPECT().Exists(gomock.Any(), "expt_completing_mutex_lock:7:8").Return(kind == "completing", nil)
				}
				if kind == "join" || kind == "duplicate" {
					runs.EXPECT().AppendRunItemIDs(gomock.Any(), int64(7), int64(8), int64(9), []int64{11}).DoAndReturn(func(context.Context, int64, int64, int64, []int64) error {
						require.True(t, tickHeld)
						return run.AppendItemIDs([]int64{11})
					})
				}
				core := service.NewExptManagerWithRunFinish(
					nil, experiments, runs, nil, nil, nil, nil,
					config, nil, locker, nil, nil, nil, generator, nil, nil,
					nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
					nil, nil, nil, nil, nil, nil, func() lock.ILocker { return locker }, finish,
				)
				manager.EXPECT().LogRetryItemsRun(gomock.Any(), int64(7), entity.EvaluationModeRetryItems, int64(9), []int64{11}, gomock.Any()).DoAndReturn(core.LogRetryItemsRun)
				app := &experimentApplication{manager: manager, auth: auth, idgen: generator}
				runID, err := retryC1Entry(t, public, app, auth, &exptpb.RetryExperimentRequest{WorkspaceID: gptr.Of(int64(9)), ExptID: gptr.Of(int64(7)), RetryMode: gptr.Of(domainexpt.ExptRetryMode_RetryTargetItems), ItemIds: []int64{11}})
				require.False(t, tickHeld)
				switch kind {
				case "join":
					require.NoError(t, err)
					require.Equal(t, int64(8), runID)
					require.Len(t, run.ItemIds, 2)
					require.Equal(t, []int64{11}, run.ItemIds[1].ItemIDs)
				default:
					require.Error(t, err)
					statusErr, ok := errorx.FromStatusError(err)
					require.True(t, ok)
					want := int32(errno.ExperimentRunningExistedCode)
					if kind == "duplicate" {
						want = errno.EvalItemAlreadyRetryingCode
					}
					if kind == "completing" {
						want = errno.ExperimentIsCompletingCode
					}
					require.Equal(t, want, statusErr.Code())
					require.Len(t, run.ItemIds, 1)
				}
			})
		}
	}
}
