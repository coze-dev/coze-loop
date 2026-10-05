// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
)

type fallbackRaceStore struct {
	*startStore
	timing                                              string
	initialReads, managedWrites, legacyWrites, casCalls int
	legacyUser, managedUser                             string
	expectedLatest                                      int64
	expectedRevision                                    string
}

func (s *fallbackRaceStore) enable() {
	s.config = enabledStartConfig()
	s.initial.HooksEnabled = true
	s.initial.ConfigRevision = "enabled-v2"
}
func (s *fallbackRaceStore) GetConfig(ctx context.Context, owner hookcomponent.ConfigOwner) (*entity.HookConfigRecord, error) {
	record, err := s.startStore.GetConfig(ctx, owner)
	if err == nil {
		record.Revision = s.initial.ConfigRevision
	}
	return record, err
}
func (s *fallbackRaceStore) ReadRunInitialization(_ context.Context, _ entity.HookRunKey) (*entity.HookRunInitialization, error) {
	s.initialReads++
	if s.initialReads == 2 && s.timing == "before_fallback_read" {
		s.enable()
	}
	if s.initialReads == 2 && s.timing == "managed_before_fallback_read" {
		s.initial.Managed = true
		s.initial.RunLog = &entity.ExptRunLog{ID: 71, ExptID: 42, ExptRunID: 71, SpaceID: 7, CreatedBy: "999", Mode: int32(entity.EvaluationModeSubmit)}
	}
	copy := s.initial
	if s.initialReads == 2 && s.timing == "after_fallback_read" {
		s.enable()
	}
	if s.initialReads == 2 && s.timing == "latest_after_fallback_read" {
		s.initial.LatestRunID = 99
		s.source.LatestRunID = 99
	}
	return &copy, nil
}
func (s *fallbackRaceStore) CreateRunWithHooks(ctx context.Context, in entity.HookCreateRunInput) (entity.HookStoreResult, error) {
	// The real codec accepts legacy snapshots without Schedule; do not mask that boundary.
	_, err := s.codec.DecodeSnapshot(ctx, in.Key, "test-scope", in.Snapshot)
	if err != nil {
		return entity.HookStoreResult{}, err
	}
	s.managedWrites++
	s.managedUser = in.RunLog.CreatedBy
	s.initial.Managed = true
	s.initial.RunLog = in.RunLog
	s.initial.LatestRunID = in.Key.RunID
	s.run = &entity.HookStoredRun{State: entity.HookRunState{Key: in.Key, Status: entity.ExptStatus_Pending, Gate: entity.HookGateWaiting, Finalize: entity.HookFinalizeNone, Before: entity.HookOperation{ID: in.Before.OperationID, Status: entity.HookOperationPending}, After: entity.HookOperation{Status: entity.HookOperationDisabled}}, Snapshot: in.Snapshot, Mode: entity.ExptRunMode(in.RunLog.Mode), CreatedBy: in.RunLog.CreatedBy}
	return entity.HookStoreResult{Changed: true, Run: s.run}, nil
}
func (s *fallbackRaceStore) CreateRunWithoutHooks(_ context.Context, log *entity.ExptRunLog, latest int64, revision string) (bool, error) {
	s.casCalls++
	s.expectedLatest = latest
	s.expectedRevision = revision
	if latest != s.initial.LatestRunID || revision != s.initial.ConfigRevision || s.initial.HooksEnabled || s.initial.Managed {
		return false, entity.ErrHookStoreConflict
	}
	s.legacyWrites++
	s.legacyUser = log.CreatedBy
	s.initial.RunLog = log
	s.initial.LatestRunID = log.ExptRunID
	s.source.LatestRunID = log.ExptRunID
	return true, nil
}

// Transparently retain the real starter while exercising the plain LogRun fallback.
type fallbackWithoutPlan struct {
	service.IExptManager
	starter service.IHookRunScheduleStarter
}

func (r fallbackWithoutPlan) StartRunWithHookSchedule(ctx context.Context, x, run, space int64, retries int, user *entity.Session, mode entity.ExptRunMode, ext map[string]string) (bool, error) {
	return r.starter.StartRunWithHookSchedule(ctx, x, run, space, retries, user, mode, ext)
}

func newFallbackRaceApplication(t *testing.T, route, timing string) (*experimentApplication, *fallbackRaceStore, *startLogs, *startQuota, *startPublisher) {
	t.Helper()
	disabled := &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(false)}}
	app, base, logs, quota, publisher := newStartApplication(t, false, disabled, nil)
	store := &fallbackRaceStore{startStore: base, timing: timing}
	if route != "router_disabled" {
		identity, err := hookinfra.NewIdentityProvider(nil, 0)
		require.NoError(t, err)
		app.manager, err = service.NewExptManagerWithHooks(app.manager, service.ExptManagerHookDependencies{Initialization: store, Runs: store, Configs: store, Codec: store.codec, Identity: identity, Runtime: hookApplicationRuntime{enabled: true}, Wake: startWake{}, ExecutionScope: "test-scope", SnapshotKeyID: "operator-key"})
		require.NoError(t, err)
	}
	if route != "direct" {
		app.manager = &hookRuntimeRouter{IExptManager: app.manager, initialization: store, admissionInstalled: route == "router_enabled"}
	}
	app.hooks.Configs = store
	return app, store, logs, quota, publisher
}

func TestLifecycleHookFallbackLateEnableNeverWritesManagedIdentity(t *testing.T) {
	for _, route := range []string{"direct", "router_enabled", "router_disabled"} {
		for _, plain := range []bool{false, true} {
			for _, timing := range []string{"before_fallback_read", "after_fallback_read", "managed_before_fallback_read", "latest_after_fallback_read"} {
				t.Run(fmt.Sprintf("%s/plain=%v/%s", route, plain, timing), func(t *testing.T) {
					app, store, logs, quota, publisher := newFallbackRaceApplication(t, route, timing)
					if plain {
						app.manager = fallbackWithoutPlan{IExptManager: app.manager, starter: app.manager.(service.IHookRunScheduleStarter)}
					}
					quota.err = errors.New("legacy Run reached")
					out, err := app.RunExperiment(hookCreationContext(), startRequest())
					assert.Nil(t, out)
					assert.ErrorIs(t, err, entity.ErrHookStoreConflict)
					assert.Zero(t, store.managedWrites, "managed CreatedBy=%q", store.managedUser)
					assert.Zero(t, store.legacyWrites)
					assert.Zero(t, logs.creates)
					assert.Zero(t, quota.calls)
					assert.Empty(t, publisher.sent)
					assert.Equal(t, 1, store.configReads, "reject before decoding a managed config")
					if timing == "after_fallback_read" || timing == "latest_after_fallback_read" {
						assert.Equal(t, 1, store.casCalls)
						assert.Equal(t, "v1", store.expectedRevision)
						assert.Zero(t, store.expectedLatest)
					}
				})
			}
		}
	}
}

func TestLifecycleHookFallbackUnavailableRouterBackendRejectsPlainLog(t *testing.T) {
	app, store, logs, quota, publisher := newFallbackRaceApplication(t, "router_disabled", "stable")
	router := app.manager.(*hookRuntimeRouter)
	router.initialization = nil
	// An older interface wrapper exposes only LogRun, not the optional starter/plan methods.
	app.manager = struct{ service.IExptManager }{router}
	out, err := app.RunExperiment(hookCreationContext(), startRequest())
	require.Nil(t, out)
	require.ErrorIs(t, err, entity.ErrHookConfigStorage)
	require.Zero(t, store.managedWrites)
	require.Zero(t, store.legacyWrites)
	require.Zero(t, logs.creates)
	require.Zero(t, quota.calls)
	require.Empty(t, publisher.sent)
}

func TestLifecycleHookFallbackStableDisabledKeepsLegacyMetadata(t *testing.T) {
	for _, route := range []string{"direct", "router_enabled", "router_disabled"} {
		for _, plain := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/plain=%v", route, plain), func(t *testing.T) {
				app, store, logs, quota, publisher := newFallbackRaceApplication(t, route, "stable")
				if plain {
					app.manager = fallbackWithoutPlan{IExptManager: app.manager, starter: app.manager.(service.IHookRunScheduleStarter)}
				}
				stop := errors.New("legacy quota boundary")
				quota.err = stop
				out, err := app.RunExperiment(hookCreationContext(), startRequest())
				require.Nil(t, out)
				require.ErrorIs(t, err, stop)
				require.Equal(t, 1, store.casCalls)
				require.Equal(t, 1, store.legacyWrites)
				require.Equal(t, "999", store.legacyUser)
				require.Equal(t, int64(71), store.initial.RunLog.ID)
				require.Equal(t, int64(42), store.initial.RunLog.ExptID)
				require.Equal(t, int64(7), store.initial.RunLog.SpaceID)
				require.Equal(t, int32(entity.EvaluationModeSubmit), store.initial.RunLog.Mode)
				require.Zero(t, store.managedWrites)
				require.Zero(t, logs.creates)
				require.Equal(t, 1, quota.calls)
				require.Empty(t, publisher.sent)
			})
		}
	}
}
