// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service_test

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	evalset "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_set"
	domain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	"github.com/stretchr/testify/require"
)

func TestHookOnlineApplicationRunInvokeFinishMySQL(t *testing.T) {
	service.OnlineAppChainForTest(t, onlineApplicationBuilder(t))
}

func TestHookOnlineApplicationRecoveryWithoutWriteKeyMySQL(t *testing.T) {
	service.OnlineAppChainForTest(t, onlineApplicationBuilder(t), true)
}

func TestHookOnlineApplicationCancelWaitingMySQL(t *testing.T) {
	service.OnlineAppCancelChainForTest(t, onlineApplicationBuilder(t), "accepted")
}

func TestHookOnlineApplicationCancelBoundariesMySQL(t *testing.T) {
	for _, scenario := range []string{"wrong_run", "corrupt_binding", "wrong_scope", "missing_marker", "no_hook_draft", "offline_pending", "permission_denied"} {
		t.Run(scenario, func(t *testing.T) {
			service.OnlineAppCancelChainForTest(t, onlineApplicationBuilder(t), scenario)
		})
	}
}

type onlineApplicationConfig struct{ component.IConfiger }

func (onlineApplicationConfig) GetMaintainerUserIDs(context.Context) map[string]bool {
	return map[string]bool{}
}

type onlineApplicationAuth struct {
	retryItemsAuth
	err error
}

func (a *onlineApplicationAuth) AuthorizationWithoutSPI(context.Context, *rpc.AuthorizationWithoutSPIParam) error {
	return a.err
}

func onlineApplicationBuilder(t *testing.T) func(service.RetryItemsAppTestEnvironment, service.ExptResultService) service.OnlineAppTestDriver {
	calls := 0
	return func(e service.RetryItemsAppTestEnvironment, result service.ExptResultService) service.OnlineAppTestDriver {
		config := hookinfra.NewRuntimeConfigProvider(nil, false)
		codec, ok := e.Codec.(*hookinfra.StorageCodec)
		require.True(t, ok)
		key := "key"
		if calls > 0 {
			key = ""
		}
		calls++
		platform := &application.HookRuntimePlatform{Scope: "local", StorageKeyID: key, Config: config, Codec: codec, Identity: e.Identity, ProtectedBackend: true, Wake: application.NewHookRuntimeWake(nil, nil, config, "local", nil)}
		stores := application.NewHookRuntimeStores(e.DB, e.Redis, platform)
		auth := new(onlineApplicationAuth)
		app, err := application.NewHookRuntimeExperimentApplication(application.HookRuntimeExperimentApplicationInputs{Manager: e.Manager, Scheduler: e.Scheduler, RecordEval: e.Consumer, Idgen: e.IDs, Configer: onlineApplicationConfig{e.Config}, Auth: auth, EvaluationSetItemService: e.Items, ResultSvc: result}, stores, e.Versions, e.Sets, e.Refs, e.Turns, e.Results, e.Experiments)
		require.NoError(t, err)
		return service.OnlineAppTestDriver{Scheduler: app, Consumer: app,
			SetAuthorizationError: func(err error) { auth.err = err },
			Kill: func(ctx context.Context) error {
				_, err := app.KillExperiment(session.WithCtxUser(ctx, &session.User{ID: "cancel-caller"}), &expt.KillExperimentRequest{WorkspaceID: gptr.Of(e.SpaceID), ExptID: gptr.Of(e.ExptID)})
				return err
			},
			Start: func(ctx context.Context) (int64, error) {
				out, err := app.RunExperiment(session.WithCtxUser(ctx, &session.User{ID: "original-user"}), &expt.RunExperimentRequest{WorkspaceID: gptr.Of(e.SpaceID), ExptID: gptr.Of(e.ExptID), ExptType: gptr.Of(domain.ExptType_Online), Session: &common.Session{UserID: gptr.Of(int64(999))}})
				if err != nil {
					return 0, err
				}
				return out.GetRunID(), nil
			},
			Invoke: func(ctx context.Context, runID, itemID int64) error {
				_, err := app.InvokeExperiment(session.WithCtxUser(ctx, &session.User{ID: "invoke-caller"}), &expt.InvokeExperimentRequest{WorkspaceID: e.SpaceID, EvaluationSetID: 71, ExperimentID: gptr.Of(e.ExptID), ExperimentRunID: gptr.Of(runID), Items: []*evalset.EvaluationSetItem{{ItemID: gptr.Of(itemID), Turns: []*evalset.Turn{{ID: gptr.Of(int64(0))}}}}, Session: &common.Session{UserID: gptr.Of(int64(999))}})
				return err
			},
			Finish: func(ctx context.Context, runID int64) error {
				_, err := app.FinishExperiment(session.WithCtxUser(ctx, &session.User{ID: "finish-caller"}), &expt.FinishExperimentRequest{WorkspaceID: gptr.Of(e.SpaceID), ExperimentID: gptr.Of(e.ExptID), ExperimentRunID: gptr.Of(runID), Session: &common.Session{UserID: gptr.Of(int64(999))}})
				return err
			},
		}
	}
}
