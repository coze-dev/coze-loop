// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/stretchr/testify/require"
)

type routerDeletionFactory struct {
	wiringExecutionFactory
	calls   int
	ids     []int64
	space   int64
	ctx     context.Context
	manager *service.ExptMangerImpl
	failure error
}

func (f *routerDeletionFactory) ForDeletion(ctx context.Context, ids []int64, space int64) (*service.ExptMangerImpl, error) {
	f.calls++
	f.ids = slices.Clone(ids)
	f.space = space
	f.ctx = ctx
	return f.manager, f.failure
}

type routerPreparedDeletion struct {
	calls     int
	ids       []int64
	space     int64
	scope     string
	committed bool
	failure   error
}

func (*routerPreparedDeletion) HookDeletionScope() string { return "test-scope" }
func (p *routerPreparedDeletion) DeleteExperiments(_ context.Context, ids []int64, space int64, scope string) ([]*entity.Experiment, error) {
	p.calls++
	p.ids = slices.Clone(ids)
	p.space = space
	p.scope = scope
	if p.failure != nil {
		return nil, p.failure
	}
	p.committed = true
	return nil, nil
}

type routerDeletionReads struct {
	repo.IExperimentRepo
	calls, plainDeletes int
	ids                 []int64
}

func (r *routerDeletionReads) MGetByID(_ context.Context, ids []int64, _ int64) ([]*entity.Experiment, error) {
	r.calls++
	r.ids = slices.Clone(ids)
	return nil, nil
}
func (r *routerDeletionReads) MDelete(context.Context, []int64, int64) error {
	r.plainDeletes++
	return nil
}

func newRouterDeletionFixture(t *testing.T) (*hookRuntimeRouter, *routerDeletionFactory, *routerPreparedDeletion, *routerDeletionReads, *wiringDeleteManager) {
	t.Helper()
	reads := &routerDeletionReads{}
	stores := &HookRuntimeStores{Platform: &HookRuntimePlatform{Scope: "test-scope"}, Runs: struct{ repo.IHookRepo }{}, Finalization: struct{ repo.IHookFinalizationRepo }{}}
	canonical, err := NewHookRuntimeManager(HookRuntimeManagerInputs{
		ExptRepo: reads, QuotaRepo: struct{ repo.QuotaRepo }{}, Mutex: struct{ lock.ILocker }{},
		Publisher: struct{ events.ExptEventPublisher }{}, Metric: struct{ metrics.ExptMetric }{},
		ExptAggrResultService: struct{ service.ExptAggrResultService }{},
	}, stores)
	require.NoError(t, err)
	prepared := &routerPreparedDeletion{}
	manager, err := service.NewExptManagerForHookDeletion(canonical, prepared, "test-scope")
	require.NoError(t, err)
	factory := &routerDeletionFactory{manager: manager}
	legacy := &wiringDeleteManager{}
	router := &hookRuntimeRouter{IExptManager: legacy, factory: factory, source: wiringSource{read: func(key entity.HookRunKey) (*entity.HookFinalizationSource, error) {
		// A legacy Latest cannot establish that all older Runs are legacy.
		return &entity.HookFinalizationSource{Key: key, Managed: false}, nil
	}}}
	return router, factory, prepared, reads, legacy
}

func TestHookRuntimeRouterForDeletionWholeRequest(t *testing.T) {
	for _, tc := range []struct {
		name   string
		ids    []int64
		single bool
	}{
		{"single_all_original_runs_despite_legacy_latest", []int64{20}, true},
		{"batch_multiple_experiments", []int64{21, 20}, false},
		{"duplicates_and_missing_ids", []int64{21, 20, 20, 999}, false},
		{"missing_single_id", []int64{999}, true},
		{"legacy_and_default_disabled", []int64{20, 21}, false},
		{"empty_batch", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router, factory, prepared, reads, legacy := newRouterDeletionFixture(t)
			original := slices.Clone(tc.ids)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			session := &entity.Session{UserID: "authorized"}
			var err error
			if tc.single {
				err = router.Delete(ctx, tc.ids[0], 7, session)
			} else {
				err = router.MDelete(ctx, tc.ids, 7, session)
			}
			require.NoError(t, err)
			require.Equal(t, 1, factory.calls, "prepare the complete request once")
			require.Equal(t, original, factory.ids)
			require.Equal(t, int64(7), factory.space)
			require.Same(t, ctx, factory.ctx)
			require.Equal(t, 1, prepared.calls, "the actual returned Manager.MDelete reaches one prepared deletion")
			require.Equal(t, original, prepared.ids)
			require.Equal(t, int64(7), prepared.space)
			require.Equal(t, "test-scope", prepared.scope)
			require.True(t, prepared.committed)
			require.Equal(t, 1, reads.calls, "single and batch both retain Manager.MDelete, not per-experiment Delete")
			require.Equal(t, original, reads.ids)
			require.Zero(t, reads.plainDeletes)
			require.Zero(t, legacy.reads, "do not select a deletion binding from Latest")
			require.Zero(t, legacy.deletes)
			require.Zero(t, legacy.batches)
			require.Empty(t, factory.wiringExecutionFactory.calls, "ForRun remains finalization/execution only")
			require.Equal(t, original, tc.ids)
		})
	}
}

func TestHookRuntimeRouterForDeletionFailureHasNoDeleteSideEffects(t *testing.T) {
	for _, single := range []bool{false, true} {
		t.Run(map[bool]string{false: "batch", true: "single"}[single], func(t *testing.T) {
			router, factory, prepared, reads, legacy := newRouterDeletionFixture(t)
			denied := errors.New("snapshot preparation rejected")
			factory.failure = denied
			var err error
			if single {
				err = router.Delete(context.Background(), 20, 7, nil)
			} else {
				err = router.MDelete(context.Background(), []int64{20, 21}, 7, nil)
			}
			require.ErrorIs(t, err, denied)
			require.Equal(t, 1, factory.calls)
			require.Zero(t, prepared.calls)
			require.False(t, prepared.committed)
			require.Zero(t, reads.calls)
			require.Zero(t, reads.plainDeletes)
			require.Zero(t, legacy.reads)
			require.Zero(t, legacy.deletes)
			require.Zero(t, legacy.batches)
			require.Empty(t, factory.wiringExecutionFactory.calls)
		})
	}
}

func TestHookRuntimeRouterForDeletionCommitFailurePropagates(t *testing.T) {
	router, factory, prepared, reads, legacy := newRouterDeletionFixture(t)
	rollback := errors.New("prepared batch rolled back")
	prepared.failure = rollback
	require.ErrorIs(t, router.MDelete(context.Background(), []int64{20, 21}, 7, nil), rollback)
	require.Equal(t, 1, factory.calls)
	require.Equal(t, 1, prepared.calls)
	require.False(t, prepared.committed)
	require.Zero(t, reads.plainDeletes)
	require.Zero(t, legacy.batches)
	require.Zero(t, legacy.deletes)
}

func TestHookRuntimeRouterForDeletionMissingPortOrManagerFailsClosed(t *testing.T) {
	for _, kind := range []string{"missing_port", "typed_nil", "nil_result"} {
		t.Run(kind, func(t *testing.T) {
			router, factory, prepared, reads, legacy := newRouterDeletionFixture(t)
			switch kind {
			case "missing_port":
				router.factory = &wiringExecutionFactory{}
			case "typed_nil":
				var missing *routerDeletionFactory
				router.factory = missing
			case "nil_result":
				factory.manager = nil
			}
			require.ErrorIs(t, router.MDelete(context.Background(), []int64{20}, 7, nil), entity.ErrHookExecutionUnsupported)
			require.Zero(t, prepared.calls)
			require.Zero(t, reads.calls)
			require.Zero(t, legacy.batches)
			require.Zero(t, legacy.deletes)
		})
	}
}
