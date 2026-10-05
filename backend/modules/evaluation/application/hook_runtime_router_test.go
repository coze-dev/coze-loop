// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"github.com/bytedance/gg/gptr"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	infraHook "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	"github.com/stretchr/testify/require"
)

type wiringSource struct {
	repo.IHookFinalizationRepo
	read func(entity.HookRunKey) (*entity.HookFinalizationSource, error)
}

func (s wiringSource) ReadFinalizationSource(_ context.Context, k entity.HookRunKey) (*entity.HookFinalizationSource, error) {
	return s.read(k)
}

type wiringExecutionFactory struct {
	calls     []entity.HookRunKey
	err       error
	execution *service.HookRuntimeExecution
}

func (f *wiringExecutionFactory) ForRun(_ context.Context, k entity.HookRunKey) (*service.HookRuntimeExecution, error) {
	f.calls = append(f.calls, k)
	return f.execution, f.err
}

func TestHookRuntimeRouterManagedScheduleUsesBoundInstance(t *testing.T) {
	key := entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}
	var seen *entity.ExptScheduleEvent
	bound := &service.ExptSchedulerImpl{Endpoints: func(_ context.Context, e *entity.ExptScheduleEvent) error { seen = e; return nil }}
	f := &wiringExecutionFactory{execution: &service.HookRuntimeExecution{Manager: &service.ExptMangerImpl{}, Scheduler: bound, Consumer: &service.ExptItemEventEvalServiceImpl{}}}
	legacy := &wiringLegacyScheduler{}
	r := &hookRuntimeRouter{source: wiringSource{read: func(entity.HookRunKey) (*entity.HookFinalizationSource, error) {
		return &entity.HookFinalizationSource{Key: key, Managed: true}, nil
	}}, factory: f, scheduler: legacy}
	event := &entity.ExptScheduleEvent{SpaceID: 1, ExptID: 2, ExptRunID: 3}
	require.NoError(t, r.Schedule(context.Background(), event))
	require.Same(t, event, seen)
	require.Equal(t, []entity.HookRunKey{key}, f.calls)
	require.Zero(t, legacy.calls)
}

func TestHookRuntimeRouterDefaultDisabledKeepsLegacyQuotaPath(t *testing.T) {
	app, store, logs, quota, publisher := newStartApplication(t, false, nil, nil)
	unavailable := errors.New("legacy quota storage unavailable")
	quota.err = unavailable
	app.manager = &hookRuntimeRouter{IExptManager: app.manager, initialization: store}
	out, err := app.RunExperiment(hookCreationContext(), startRequest())
	require.ErrorIs(t, err, unavailable)
	require.Nil(t, out)
	require.Equal(t, 1, logs.creates)
	require.Zero(t, store.creates)
	require.Equal(t, 1, quota.calls)
	require.Empty(t, publisher.sent)
}

func TestHookRuntimeRouterMissingBindingsRejectsNewHook(t *testing.T) {
	app, store, logs, _, publisher := newStartApplication(t, false, enabledStartConfig(), nil)
	app.manager = &hookRuntimeRouter{IExptManager: app.manager, initialization: store}
	out, err := app.RunExperiment(hookCreationContext(), startRequest())
	require.Error(t, err)
	require.Nil(t, out)
	require.Zero(t, logs.creates)
	require.Zero(t, store.creates)
	require.Empty(t, publisher.sent)
}

func TestHookRuntimeRouterRecoveryUsesStoredKeyWithoutNewAdmission(t *testing.T) {
	seeded, store, _, _, original := newStartApplication(t, true, enabledStartConfig(), nil)
	_, err := seeded.RunExperiment(hookCreationContext(), startRequest())
	require.NoError(t, err)
	require.Len(t, original.sent, 1)
	base, _, _, _, published := newStartApplication(t, false, nil, nil)
	identity, err := infraHook.NewIdentityProvider(nil, 0)
	require.NoError(t, err)
	config := infraHook.NewRuntimeConfigProvider(wiringLoader{}, true)
	platform := &HookRuntimePlatform{Scope: "test-scope", Config: config, Identity: identity, Wake: NewHookRuntimeWake(wiringLoader{}, nil, config, "test-scope", nil)}
	router := &hookRuntimeRouter{IExptManager: base.manager, initialization: store, runs: store, codec: store.codec, scope: "test-scope", stores: &HookRuntimeStores{Platform: platform, Configs: store}}
	require.NoError(t, router.PublishHookSchedule(context.Background(), original.sent[0]))
	require.Equal(t, original.sent, published.sent)
	require.Equal(t, 1, store.creates, "recovery must not create another Run")
	require.Empty(t, platform.StorageKeyID, "the missing current write key is not replaced")
	store.run.Snapshot.KeyID = ""
	require.Error(t, router.PublishHookSchedule(context.Background(), original.sent[0]))
	require.Len(t, published.sent, 1)
}

type wiringLegacyScheduler struct{ calls int }

func (s *wiringLegacyScheduler) Schedule(context.Context, *entity.ExptScheduleEvent) error {
	s.calls++
	return nil
}

type wiringLegacyConsumer struct{ calls int }

func (s *wiringLegacyConsumer) Eval(context.Context, *entity.ExptItemEvalEvent) error {
	s.calls++
	return nil
}

type wiringDeleteManager struct {
	service.IExptManager
	deletes, batches int
	reads            int
	missing          bool
}

func (m *wiringDeleteManager) MGet(ctx context.Context, ids []int64, space int64, s *entity.Session) ([]*entity.Experiment, error) {
	m.reads++
	if m.missing {
		return nil, nil
	}
	out := make([]*entity.Experiment, 0, len(ids))
	for _, id := range ids {
		x, err := m.Get(ctx, id, space, s)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, nil
}

func (m *wiringDeleteManager) Get(_ context.Context, id, space int64, _ *entity.Session) (*entity.Experiment, error) {
	m.reads++
	return &entity.Experiment{ID: id, SpaceID: space, LatestRunID: 3}, nil
}
func (m *wiringDeleteManager) Delete(context.Context, int64, int64, *entity.Session) error {
	m.deletes++
	return nil
}
func (m *wiringDeleteManager) MDelete(context.Context, []int64, int64, *entity.Session) error {
	m.batches++
	return nil
}

func TestHookRuntimeRouterLegacyDoesNotResolveSnapshot(t *testing.T) {
	key := entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}
	legacyScheduler, legacyConsumer := &wiringLegacyScheduler{}, &wiringLegacyConsumer{}
	f := &wiringExecutionFactory{err: errors.New("must not call")}
	r := &hookRuntimeRouter{source: wiringSource{read: func(k entity.HookRunKey) (*entity.HookFinalizationSource, error) {
		require.Equal(t, key, k)
		return &entity.HookFinalizationSource{Key: key, Managed: false}, nil
	}}, factory: f, scheduler: legacyScheduler, consumer: legacyConsumer}
	require.NoError(t, r.Schedule(context.Background(), &entity.ExptScheduleEvent{SpaceID: 1, ExptID: 2, ExptRunID: 3}))
	require.NoError(t, r.Eval(context.Background(), &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 2, ExptRunID: 3}))
	require.Equal(t, 1, legacyScheduler.calls)
	require.Equal(t, 1, legacyConsumer.calls)
	require.Empty(t, f.calls)
}

func TestHookRuntimeRouterManagedNeverFallsBack(t *testing.T) {
	key := entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}
	unavailable := errors.New("snapshot unavailable")
	legacyScheduler, legacyConsumer := &wiringLegacyScheduler{}, &wiringLegacyConsumer{}
	f := &wiringExecutionFactory{err: unavailable}
	r := &hookRuntimeRouter{source: wiringSource{read: func(entity.HookRunKey) (*entity.HookFinalizationSource, error) {
		return &entity.HookFinalizationSource{Key: key, Managed: true, Experiment: &entity.Experiment{EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig}}, nil
	}}, factory: f, scheduler: legacyScheduler, consumer: legacyConsumer}
	require.ErrorIs(t, r.Schedule(context.Background(), &entity.ExptScheduleEvent{SpaceID: 1, ExptID: 2, ExptRunID: 3}), unavailable)
	require.ErrorIs(t, r.Eval(context.Background(), &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 2, ExptRunID: 3}), unavailable)
	require.Equal(t, []entity.HookRunKey{key, key}, f.calls)
	require.Zero(t, legacyScheduler.calls)
	require.Zero(t, legacyConsumer.calls)
}

func TestHookRuntimeRouterLegacyRetryItemsSnapshotNeverFallsBack(t *testing.T) {
	app, store, _, _, _ := newStartApplication(t, true, enabledStartConfig(), nil)
	store.source.EvalSetID = 11
	store.source.EvalSetVersionID = 11
	_, err := app.RunExperiment(hookCreationContext(), startRequest())
	require.NoError(t, err)
	key := store.run.State.Key
	snapshot, err := store.codec.DecodeSnapshot(context.Background(), key, "test-scope", store.run.Snapshot)
	require.NoError(t, err)
	input := snapshot.Input()
	input.Context.RunMode = gptr.Of("retry_items")
	input.Schedule = nil
	input.Execution = nil
	snapshot, err = entity.NewHookRunSnapshot(input)
	require.NoError(t, err)
	store.run.Snapshot, err = store.codec.EncodeSnapshot(context.Background(), store.run.Snapshot.KeyID, snapshot)
	require.NoError(t, err)
	store.run.Mode = entity.EvaluationModeRetryItems
	legacy := &wiringLegacyScheduler{}
	r := &hookRuntimeRouter{source: wiringSource{read: func(entity.HookRunKey) (*entity.HookFinalizationSource, error) {
		return &entity.HookFinalizationSource{Key: key, Managed: true, Experiment: store.source}, nil
	}}, factory: &wiringExecutionFactory{err: entity.ErrHookExecutionUnsupported}, runs: store, codec: store.codec, scope: "test-scope", scheduler: legacy}
	err = r.Schedule(context.Background(), &entity.ExptScheduleEvent{SpaceID: key.WorkspaceID, ExptID: key.ExperimentID, ExptRunID: key.RunID, ExptRunMode: entity.EvaluationModeRetryItems})
	require.ErrorIs(t, err, entity.ErrHookExecutionUnsupported)
	require.Zero(t, legacy.calls)
}

func TestHookRuntimeRouterOwnershipErrorStopsLegacy(t *testing.T) {
	for _, wrong := range []bool{false, true} {
		t.Run(map[bool]string{false: "read_error", true: "wrong_owner"}[wrong], func(t *testing.T) {
			scheduler := &wiringLegacyScheduler{}
			r := &hookRuntimeRouter{source: wiringSource{read: func(entity.HookRunKey) (*entity.HookFinalizationSource, error) {
				if wrong {
					return &entity.HookFinalizationSource{Key: entity.HookRunKey{WorkspaceID: 9, ExperimentID: 2, RunID: 3}}, nil
				}
				return nil, entity.ErrHookStoreCorrupt
			}}, scheduler: scheduler}
			require.Error(t, r.Schedule(context.Background(), &entity.ExptScheduleEvent{SpaceID: 1, ExptID: 2, ExptRunID: 3}))
			require.Zero(t, scheduler.calls)
		})
	}
}
