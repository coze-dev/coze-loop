// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"

	domain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
)

type retryStartStore struct {
	*startStore
	sourceRun *entity.ExptRunLog
}

func (s *retryStartStore) CreateRunWithoutHooks(context.Context, *entity.ExptRunLog, int64, string) (bool, error) {
	return false, errors.New("legacy initialization reached")
}

func (s *retryStartStore) ReadRunInitialization(ctx context.Context, key entity.HookRunKey) (*entity.HookRunInitialization, error) {
	if key.RunID == 70 {
		return &entity.HookRunInitialization{LatestRunID: 70, RunLog: s.sourceRun}, nil
	}
	return s.startStore.ReadRunInitialization(ctx, key)
}
func (s *retryStartStore) CreateRunWithHooks(ctx context.Context, in entity.HookCreateRunInput) (entity.HookStoreResult, error) {
	out, err := s.startStore.CreateRunWithHooks(ctx, in)
	if err == nil {
		out.Run.SourceRunID = in.SourceRunID
	}
	return out, err
}

func newRetryApplication(t *testing.T, installed bool, config *entity.LifecycleHookConf, authErr error) (*experimentApplication, *retryStartStore, *startLogs, *startQuota, *startPublisher) {
	t.Helper()
	app, base, logs, quota, publisher := newStartApplication(t, false, config, authErr)
	base.initial.LatestRunID = 70
	base.source.LatestRunID = 70
	base.source.EvalSetID = 11
	base.source.EvalSetVersionID = 11
	base.source.EvalConf = &entity.EvaluationConfiguration{ItemRetryNum: gptr.Of(4)}
	store := &retryStartStore{startStore: base, sourceRun: &entity.ExptRunLog{ID: 70, ExptID: 42, ExptRunID: 70, SpaceID: 7, CreatedBy: "old-user", Mode: int32(entity.EvaluationModeSubmit), Status: int64(entity.ExptStatus_Failed)}}
	if installed {
		identity, err := hookinfra.NewIdentityProvider(nil, 0)
		require.NoError(t, err)
		app.manager, err = service.NewExptManagerWithHooks(app.manager, service.ExptManagerHookDependencies{Initialization: store, Runs: store, Configs: store, Codec: store.codec, Identity: identity, Runtime: hookApplicationRuntime{enabled: true}, Wake: startWake{}, ExecutionScope: "test-scope", SnapshotKeyID: "operator-key"})
		require.NoError(t, err)
	}
	return app, store, logs, quota, publisher
}
func retryRequest(mode domain.ExptRetryMode) *expt.RetryExperimentRequest {
	return &expt.RetryExperimentRequest{WorkspaceID: gptr.Of(int64(7)), ExptID: gptr.Of(int64(42)), RetryMode: gptr.Of(mode), Ext: map[string]string{"route": "original", entity.RetryYieldExtKey: "false"}}
}

func TestLifecycleHookRetryStarterPersistsAndPublishesOnce(t *testing.T) {
	for _, mode := range []domain.ExptRetryMode{domain.ExptRetryMode_RetryFailure, domain.ExptRetryMode_RetryAll} {
		for _, fail := range []bool{false, true} {
			t.Run(fmt.Sprintf("%v/error=%v", mode, fail), func(t *testing.T) {
				app, store, logs, quota, publisher := newRetryApplication(t, true, enabledStartConfig(), nil)
				failure := errors.New("retry MQ unavailable")
				if fail {
					publisher.err = failure
				}
				req := retryRequest(mode)
				out, err := app.RetryExperiment(hookCreationContext(), req)
				if fail {
					require.Nil(t, out)
					require.ErrorIs(t, err, failure)
				} else {
					require.NoError(t, err)
					require.Equal(t, int64(71), out.GetRunID())
				}
				require.Equal(t, 1, store.creates)
				require.Zero(t, logs.creates)
				require.Equal(t, 1, quota.calls)
				require.Len(t, publisher.sent, 1)
				require.NotNil(t, store.run.SourceRunID)
				require.Equal(t, int64(70), *store.run.SourceRunID)
				wantMode := entity.EvaluationModeFailRetry
				if mode == domain.ExptRetryMode_RetryAll {
					wantMode = entity.EvaluationModeRetryAll
				}
				require.Equal(t, wantMode, publisher.sent[0].ExptRunMode)
				require.Equal(t, 4, publisher.sent[0].ItemRetryTimes)
				require.Equal(t, "trusted-user", publisher.sent[0].Session.UserID)
				require.Equal(t, "original", publisher.sent[0].Ext["route"])
				require.Equal(t, "false", req.Ext[entity.RetryYieldExtKey])
				require.Equal(t, "true", publisher.sent[0].Ext[entity.RetryYieldExtKey])
				snapshot, err := store.codec.DecodeSnapshot(context.Background(), store.run.State.Key, "test-scope", store.run.Snapshot)
				require.NoError(t, err)
				require.Equal(t, wantMode, snapshot.Input().Schedule.Mode)
			})
		}
	}
}

func TestLifecycleHookRetryEnabledCannotFallBack(t *testing.T) {
	for _, mode := range []domain.ExptRetryMode{domain.ExptRetryMode_RetryFailure, domain.ExptRetryMode_RetryAll} {
		for _, kind := range []string{"missing_runtime", "missing_starter", "unhandled", "read_error", "missing_source"} {
			t.Run(fmt.Sprintf("%v/%s", mode, kind), func(t *testing.T) {
				installed := kind == "unhandled" || kind == "missing_source"
				app, store, logs, quota, publisher := newRetryApplication(t, installed, enabledStartConfig(), nil)
				quota.err = errors.New("legacy Run reached")
				switch kind {
				case "missing_starter":
					app.manager = struct{ service.IExptManager }{app.manager}
				case "unhandled":
					store.initial.HooksEnabled = false
				case "read_error":
					store.readErr = entity.ErrHookConfigStorage
				case "missing_source":
					store.sourceRun = nil
				}
				out, err := app.RetryExperiment(hookCreationContext(), retryRequest(mode))
				require.Nil(t, out)
				switch kind {
				case "read_error":
					require.ErrorIs(t, err, entity.ErrHookConfigStorage)
				case "missing_source":
					require.ErrorIs(t, err, service.ErrHookScheduleRetrySourceMissing)
				default:
					require.ErrorContains(t, err, "HOOK_SCHEDULE_RUNTIME_UNAVAILABLE")
				}
				require.Zero(t, logs.creates)
				require.Zero(t, store.creates)
				require.Zero(t, quota.calls)
				require.Empty(t, publisher.sent)
			})
		}
	}
}

func TestLifecycleHookRetryLegacyAndPermissionBoundaries(t *testing.T) {
	for _, mode := range []domain.ExptRetryMode{domain.ExptRetryMode_RetryFailure, domain.ExptRetryMode_RetryAll} {
		for _, kind := range []string{"nil", "disabled", "legacy_app", "denied"} {
			t.Run(fmt.Sprintf("%v/%s", mode, kind), func(t *testing.T) {
				var conf *entity.LifecycleHookConf
				var authErr error
				if kind == "disabled" {
					conf = &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(false)}}
				}
				if kind == "denied" {
					conf = enabledStartConfig()
					authErr = errors.New("denied")
				}
				app, store, logs, quota, publisher := newRetryApplication(t, false, conf, authErr)
				if kind == "legacy_app" {
					app.hooks = nil
				}
				stop := errors.New("legacy quota boundary")
				quota.err = stop
				out, err := app.RetryExperiment(hookCreationContext(), retryRequest(mode))
				require.Nil(t, out)
				if kind == "denied" {
					require.ErrorIs(t, err, authErr)
					require.Zero(t, store.configReads)
					require.Zero(t, logs.creates)
					require.Zero(t, quota.calls)
				} else {
					require.ErrorIs(t, err, stop)
					require.Equal(t, 1, logs.creates)
					require.Equal(t, 1, quota.calls)
					require.Equal(t, "trusted-user", logs.user)
				}
				require.Empty(t, publisher.sent)
				require.Zero(t, store.creates)
			})
		}
	}
}

func TestLifecycleHookRetryDefaultModePreservesSourceOnReplay(t *testing.T) {
	app, store, logs, _, publisher := newRetryApplication(t, true, enabledStartConfig(), nil)
	req := retryRequest(domain.ExptRetryMode_RetryFailure)
	req.RetryMode = nil
	first, err := app.RetryExperiment(hookCreationContext(), req)
	require.NoError(t, err)
	require.Equal(t, domain.ExptRetryMode_RetryFailure, req.GetRetryMode())
	hash := store.run.Snapshot.Hash
	second, err := app.RetryExperiment(hookCreationContext(), req)
	require.NoError(t, err)
	require.Equal(t, first.GetRunID(), second.GetRunID())
	require.Equal(t, 1, store.creates)
	require.Equal(t, hash, store.run.Snapshot.Hash)
	require.Equal(t, int64(70), *store.run.SourceRunID)
	require.Zero(t, logs.creates)
	require.Len(t, publisher.sent, 2)
	require.Equal(t, publisher.sent[0], publisher.sent[1])
}
