// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service_test

import (
	"context"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	domain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	"github.com/stretchr/testify/require"
	"testing"
)

type retryItemsAuth struct{ rpc.IAuthProvider }

func (retryItemsAuth) AuthorizationWithoutSPI(context.Context, *rpc.AuthorizationWithoutSPIParam) error {
	return nil
}

func TestHookRetryItemsApplicationRouterToAfterMySQL(t *testing.T) {
	service.RetryItemsAppChainForTest(t, func(e service.RetryItemsAppTestEnvironment) service.RetryItemsAppTestDriver {
		config := hookinfra.NewRuntimeConfigProvider(nil, false)
		codec, ok := e.Codec.(*hookinfra.StorageCodec)
		require.True(t, ok, "the application fixture must use the production storage codec")
		platform := &application.HookRuntimePlatform{Scope: "local", StorageKeyID: "key", Config: config, Codec: codec, Identity: e.Identity, ProtectedBackend: true, Wake: application.NewHookRuntimeWake(nil, nil, config, "local", nil)}
		stores := application.NewHookRuntimeStores(e.DB, e.Redis, platform)
		app, err := application.NewHookRuntimeExperimentApplication(application.HookRuntimeExperimentApplicationInputs{Manager: e.Manager, Scheduler: e.Scheduler, RecordEval: e.Consumer, Idgen: e.IDs, Configer: e.Config, Auth: retryItemsAuth{}, EvaluationSetItemService: e.Items}, stores, e.Versions, e.Sets, e.Refs, e.Turns, e.Results, e.Experiments)
		require.NoError(t, err)
		calls := 0
		return service.RetryItemsAppTestDriver{Scheduler: app, Consumer: app, Start: func(ctx context.Context, ids []int64) (int64, error) {
			user, route := "original-user", "original"
			if calls > 0 {
				user, route = "different-user", "changed"
			}
			calls++
			ctx = session.WithCtxUser(ctx, &session.User{ID: user})
			out, err := app.RetryExperiment(ctx, &expt.RetryExperimentRequest{WorkspaceID: gptr.Of(e.SpaceID), ExptID: gptr.Of(e.ExptID), RetryMode: gptr.Of(domain.ExptRetryMode_RetryTargetItems), ItemIds: ids, Ext: map[string]string{"route": route}})
			if err != nil {
				return 0, err
			}
			return out.GetRunID(), nil
		}}
	})
}
