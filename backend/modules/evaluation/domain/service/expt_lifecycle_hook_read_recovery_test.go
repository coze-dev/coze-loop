// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"syscall"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/stretchr/testify/require"
)

// External storage is stateful in memory; loader, initializer, admission and dispatch are real.
type frozenRecoveryPorts struct {
	repo.IHookRepo
	repo.IHookSchedulerRepo
	repo.IHookPlanRepo
	IExptManager
	EvaluationSetItemService
	metrics.ExptMetric
	t             *testing.T
	f             *loaderFixture
	init          *executionTestRepo
	key           entity.HookRunKey
	reads         []int64
	itemReads     []int64
	failAt        int64
	failure       error
	onRead        func()
	dispatched    map[int64]bool
	terminal      entity.CompleteExptOption
	after         bool
	finalizations int
}

func (p *frozenRecoveryPorts) ReadPlanPage(ctx context.Context, in entity.HookPlanReadInput) (*entity.HookPlanReadPage, error) {
	require.Equal(p.t, p.key, in.Key)
	require.Equal(p.t, "local", in.ExecutionScope)
	p.reads = append(p.reads, in.StartOrdinal)
	if p.onRead != nil {
		p.onRead()
	}
	if in.StartOrdinal == p.failAt && p.failure != nil {
		err := p.failure
		p.failure = nil
		return nil, err
	}
	page := p.init.page(entity.HookExecutionInitializationReadInput{StartOrdinal: in.StartOrdinal, Limit: in.Limit})
	out := &entity.HookPlanReadPage{Ready: true, Count: page.Count, Hash: page.Hash, RunVersion: page.RunVersion, NextOrdinal: page.NextOrdinal, HasMore: page.HasMore}
	for _, row := range page.Items {
		out.Items = append(out.Items, row.Frozen)
	}
	return out, nil
}

func (p *frozenRecoveryPorts) GetEvaluationSetItemVersion(ctx context.Context, space, set, item int64, version *int64, name *string) (*entity.EvaluationSetItemVersion, error) {
	require.Equal(p.t, int64(10), space)
	require.Equal(p.t, int64(20), set)
	require.Equal(p.t, int64(4), gptr.Indirect(version))
	require.Nil(p.t, name)
	p.itemReads = append(p.itemReads, item)
	if p.f.items.version == nil {
		return nil, p.f.items.err
	}
	result := *p.f.items.version
	result.ItemID = item
	return &result, p.f.items.err
}

func (p *frozenRecoveryPorts) BatchGetEvaluationSetItems(ctx context.Context, in *entity.BatchGetEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, error) {
	return p.f.items.BatchGetEvaluationSetItems(ctx, in)
}

func (p *frozenRecoveryPorts) CanDispatch(context.Context, entity.HookRunKey) (entity.HookAdmissionDecision, error) {
	if p.after {
		return entity.HookAdmissionDecision{Gate: entity.HookGateClosed}, nil
	}
	if p.init.initialized {
		return entity.HookAdmissionDecision{Gate: entity.HookGateReady}, nil
	}
	return entity.HookAdmissionDecision{Gate: entity.HookGateWaiting, Reason: "HOOK_EXECUTION_PENDING"}, nil
}

func (p *frozenRecoveryPorts) GetRun(context.Context, entity.HookRunKey) (*entity.HookStoredRun, error) {
	return &entity.HookStoredRun{State: entity.HookRunState{Finalize: entity.HookFinalizeCommitted}}, nil
}

func (p *frozenRecoveryPorts) GetRunLog(context.Context, int64, int64, int64, *entity.Session) (*entity.ExptRunLog, error) {
	return &entity.ExptRunLog{Status: int64(entity.ExptStatus_Processing)}, nil
}

func (p *frozenRecoveryPorts) CompleteExpt(_ context.Context, expt int64, run *int64, space int64, _ *entity.Session, opts ...entity.CompleteExptOptionFn) error {
	require.Equal(p.t, p.key, entity.HookRunKey{WorkspaceID: space, ExperimentID: expt, RunID: *run})
	for _, opt := range opts {
		opt(&p.terminal)
	}
	p.finalizations++
	p.after = true
	return nil
}

func (p *frozenRecoveryPorts) ReadFinalizationSource(context.Context, entity.HookRunKey) (*entity.HookFinalizationSource, error) {
	return &entity.HookFinalizationSource{Key: p.key, Managed: true}, nil
}

func (p *frozenRecoveryPorts) PersistHookDispatch(_ context.Context, in entity.HookSchedulerDispatchInput) ([]int64, error) {
	require.Equal(p.t, p.key, in.Key)
	require.Equal(p.t, "local", in.ExecutionScope)
	for _, id := range in.ItemIDs {
		require.False(p.t, p.dispatched[id], "a resumed scan must not dispatch an initialized item twice")
		p.dispatched[id] = true
	}
	return in.ItemIDs, nil
}

func (*frozenRecoveryPorts) EmitItemExecEval(int64, int64, int) {}

func frozenRecoveryScheduler(t *testing.T, count int) (*ExptSchedulerImpl, *frozenRecoveryPorts, *executionTestIDs, *schedulerHookPublisher, *entity.ExptScheduleEvent) {
	t.Helper()
	f := exactLoaderFixture()
	r := &executionTestRepo{version: 7}
	for i := 0; i < count; i++ {
		tuple := f.plans.page.Items[0]
		tuple.ID += int64(i)
		tuple.ItemID += int64(i)
		r.rows = append(r.rows, entity.HookExecutionInitializationItem{Ordinal: int64(i), Frozen: tuple})
	}
	p := &frozenRecoveryPorts{t: t, f: f, init: r, key: f.input.Key, failAt: -1, dispatched: map[int64]bool{}}
	f.deps.Plans, f.deps.Items = p, p
	l, err := NewHookFrozenPlanLoader(f.deps)
	require.NoError(t, err)
	ids := &executionTestIDs{next: 50000}
	i, err := NewHookFrozenExecutionInitializer(HookFrozenExecutionInitializerDependencies{Repository: r, Loader: l, IDs: ids})
	require.NoError(t, err)
	init := i.(*hookFrozenExecutionInitializer)
	init.boundKey, init.boundScope = p.key, "local"
	pub := &schedulerHookPublisher{}
	s := &ExptSchedulerImpl{Manager: p, Publisher: pub, Configer: &schedulerHookConfig{}, Metric: p, ResultSvc: &schedulerHookResults{},
		hookGate: p, hookRuns: p, hookScheduler: p, hookSchedulerScope: "local", hookBoundInitializer: init, hookBoundMode: entity.EvaluationModeSubmit}
	// Isolate admission/dispatch from unrelated mode scans, clocks, Redis and result indexing.
	s.Endpoints = s.HandleEventCheck(func(ctx context.Context, event *entity.ExptScheduleEvent) error {
		var queue []*entity.ExptEvalItem
		for _, row := range r.rows {
			require.NotNil(t, row.Manifest)
			if !p.dispatched[row.Frozen.ItemID] {
				queue = append(queue, &entity.ExptEvalItem{ItemID: row.Frozen.ItemID, State: entity.ItemRunState_Queueing})
			}
		}
		return s.handleToSubmits(ctx, event, queue)
	})
	event := &entity.ExptScheduleEvent{SpaceID: p.key.WorkspaceID, ExptID: p.key.ExperimentID, ExptRunID: p.key.RunID, ExptRunMode: entity.EvaluationModeSubmit,
		CreatedAt: time.Now().Unix(), Session: &entity.Session{UserID: "original-user"}, Ext: map[string]string{"scope": "local"}}
	return s, p, ids, pub, event
}

func TestHookFrozenReadRecoveryScheduler(t *testing.T) {
	for _, partial := range []bool{false, true} {
		t.Run(fmt.Sprintf("partial_%t", partial), func(t *testing.T) {
			count, failAt := 1, int64(0)
			if partial {
				count, failAt = 101, 100
			}
			s, p, ids, pub, event := frozenRecoveryScheduler(t, count)
			p.failAt, p.failure = failAt, fmt.Errorf("private DSN token: %w", syscall.ECONNRESET)
			require.NoError(t, s.Schedule(context.Background(), event))
			require.Zero(t, p.finalizations, "transient frozen reads must not terminate the Run")
			require.False(t, p.after)
			require.False(t, p.init.initialized)
			require.Empty(t, p.init.completes)
			require.Zero(t, pub.itemDispatches)
			require.Len(t, pub.published, 1)
			require.Equal(t, event, pub.published[0])
			require.Equal(t, []time.Duration{5 * time.Second}, pub.delays)
			var committed entity.HookExecutionManifest
			if partial {
				require.Len(t, p.init.writes, 1)
				committed = p.init.rows[0].Manifest.Clone()
				require.NotNil(t, p.init.rows[99].Manifest)
			} else {
				require.Empty(t, p.init.writes)
				require.Empty(t, ids.counts)
			}
			require.Nil(t, p.init.rows[failAt].Manifest)
			require.NoError(t, s.Schedule(context.Background(), pub.published[0]))
			require.True(t, p.init.initialized)
			require.Len(t, p.init.completes, 1)
			require.Equal(t, int64(count), p.init.completes[0].ExpectedItemCount)
			require.Equal(t, int64(count*2), p.init.completes[0].ExpectedTurnCount)
			require.Len(t, p.dispatched, count)
			require.Equal(t, 1, pub.itemDispatches)
			if partial {
				require.Equal(t, committed, *p.init.rows[0].Manifest)
				require.Equal(t, []int64{0, 100, 100}, p.reads)
				require.Equal(t, []int{400, 4}, ids.counts)
			} else {
				require.Equal(t, []int64{0, 0}, p.reads)
				require.Equal(t, []int{4}, ids.counts)
			}
			require.Len(t, p.itemReads, count, "committed pages must not reload mutable source")
			require.NoError(t, s.Schedule(context.Background(), event))
			require.Len(t, p.init.completes, 1)
			require.Equal(t, 1, pub.itemDispatches)
			require.Len(t, pub.published, 1)
			require.Zero(t, p.finalizations)
			require.False(t, p.after)
		})
	}
}

func TestHookFrozenReadClassification(t *testing.T) {
	for _, boundary := range []string{"plan", "schema", "draft_schema", "version_item", "batch", "content"} {
		for _, candidates := range []bool{false, true} {
			if boundary == "plan" && candidates {
				continue
			}
			t.Run(fmt.Sprintf("%s/candidates_%t", boundary, candidates), func(t *testing.T) {
				for _, tc := range []struct {
					name      string
					err       error
					permanent bool
				}{
					{"reset", fmt.Errorf("private: %w", syscall.ECONNRESET), false},
					{"rpc", errorx.NewByCode(errno.CommonRPCErrorCode, errorx.WithExtraMsg("private")), false},
					{"database", errors.New("private database unavailable"), false},
					{"misleading_text", errors.New("permission denied; not found; private"), false},
					{"permission", fmt.Errorf("private: %w", fs.ErrPermission), true},
					{"evaluation_permission", errorx.NewByCode(errno.CommonNoPermissionCode), true},
					{"data_permission", errorx.NewByCode(601100101), true},
					{"data_unauthorized", kerrors.NewBizStatusError(601103002, "private"), true},
					{"data_json_errorx", errorx.WrapByCode(errors.New("private malformed schema"), 601103003), true},
					{"data_json_kitex", kerrors.NewBizStatusError(601103003, "private malformed schema"), true},
					{"foundation_permission", errorx.NewByCode(602000101), true},
					{"missing_row", sql.ErrNoRows, true},
					{"missing_file", fs.ErrNotExist, true},
					{"data_missing", errorx.NewByCode(601103001), true},
					{"content_missing", errorx.NewByCode(601104001), true},
					{"evaluation_missing", errorx.NewByCode(errno.ResourceNotFoundCode), true},
				} {
					t.Run(tc.name, func(t *testing.T) {
						f := exactLoaderFixture()
						if boundary == "batch" {
							f = newLoaderFixture()
						}
						switch boundary {
						case "plan":
							f.plans.err = tc.err
						case "schema":
							f.versions.err = tc.err
						case "draft_schema":
							f.plans.page.Items[0].EvalSetVersionID = 0
							f.sets.err = tc.err
						case "version_item", "batch":
							f.items.err = tc.err
						case "content":
							f.items.version.Turns[0].FieldDataList[0].Content.ContentOmitted = gptr.Of(true)
							f.deps.Resolver = &loaderResolver{fn: func(context.Context, entity.HookPlanItem, int64, string, *entity.Content) (*entity.Content, error) {
								return nil, tc.err
							}}
						}
						l, err := NewHookFrozenPlanLoader(f.deps)
						require.NoError(t, err)
						var page *entity.HookLoadedPlanPage
						if candidates {
							page, err = l.(retryItemsCandidateLoader).LoadCandidates(context.Background(), f.input, f.plans.page)
						} else {
							page, err = l.LoadPage(context.Background(), f.input)
						}
						require.Error(t, err)
						require.Nil(t, page)
						require.NotContains(t, err.Error(), "private")
						if (tc.name == "data_json_errorx" || tc.name == "data_json_kitex") && (boundary == "schema" || boundary == "draft_schema") {
							require.ErrorIs(t, err, entity.ErrHookFrozenSchemaInvalid)
						}
						require.Equal(t, tc.permanent, permanentHookInitializationError(err))
						if !tc.permanent {
							require.EqualError(t, err, "frozen hook source read unavailable")
						}
						require.Equal(t, tc.permanent, permanentHookInitializationError(executionInitializationError(context.Background(), err)))
					})
				}
			})
		}
	}
}

func TestHookFrozenReadRecoveryOtherBoundaries(t *testing.T) {
	for _, boundary := range []string{"schema", "draft_schema", "version_item", "batch", "content"} {
		t.Run(boundary, func(t *testing.T) {
			s, p, _, pub, event := frozenRecoveryScheduler(t, 1)
			failure := fmt.Errorf("private RPC endpoint: %w", syscall.ECONNRESET)
			switch boundary {
			case "schema":
				p.f.versions.err = failure
			case "draft_schema":
				p.init.rows[0].Frozen.EvalSetVersionID = 0
				p.f.sets.err = failure
			case "version_item":
				p.f.items.err = failure
			case "batch":
				p.init.rows[0].Frozen.ItemVersionID = 0
				p.f.items.err = failure
			case "content":
				p.f.items.version.Turns[0].FieldDataList[0].Content.ContentOmitted = gptr.Of(true)
				p.f.deps.Resolver = &loaderResolver{fn: func(context.Context, entity.HookPlanItem, int64, string, *entity.Content) (*entity.Content, error) {
					if failure != nil {
						return nil, failure
					}
					return &entity.Content{Text: gptr.Of("restored")}, nil
				}}
				l, err := NewHookFrozenPlanLoader(p.f.deps)
				require.NoError(t, err)
				s.hookBoundInitializer.deps.Loader = l
			}
			require.NoError(t, s.Schedule(context.Background(), event))
			require.Zero(t, p.finalizations)
			require.False(t, p.after)
			require.Zero(t, pub.itemDispatches)
			require.Empty(t, p.init.writes)
			require.Len(t, pub.published, 1)
			require.Equal(t, event, pub.published[0])
			failure, p.f.items.err, p.f.versions.err, p.f.sets.err = nil, nil, nil, nil
			require.NoError(t, s.Schedule(context.Background(), pub.published[0]))
			require.True(t, p.init.initialized)
			require.Len(t, p.init.writes, 1)
			require.Len(t, p.init.completes, 1)
			require.Equal(t, 1, pub.itemDispatches)
			require.NoError(t, s.Schedule(context.Background(), event))
			require.Len(t, p.init.completes, 1)
			require.Equal(t, 1, pub.itemDispatches)
		})
	}
}

func TestHookFrozenReadRecoveryPermanentControls(t *testing.T) {
	for _, kind := range []string{"permission", "data_json_errorx", "data_json_kitex", "missing", "corrupt_plan", "missing_item", "wrong_version", "cross_space", "invalid_content", "deadline"} {
		t.Run(kind, func(t *testing.T) {
			s, p, ids, pub, event := frozenRecoveryScheduler(t, 1)
			switch kind {
			case "permission":
				p.f.versions.err = kerrors.NewBizStatusError(601103002, "private credentials")
			case "data_json_errorx":
				p.f.versions.err = errorx.WrapByCode(errors.New("private malformed schema"), 601103003)
			case "data_json_kitex":
				p.f.versions.err = kerrors.NewBizStatusError(601103003, "private malformed schema")
			case "missing":
				p.failAt, p.failure = 0, entity.ErrHookStoreMissing
			case "corrupt_plan":
				p.failAt, p.failure = 0, entity.ErrHookFrozenPlanInvalid
			case "missing_item":
				p.f.items.version = nil
			case "wrong_version":
				p.f.items.version.ItemVersionID++
			case "cross_space":
				p.f.versions.value.SpaceID++
			case "invalid_content":
				p.f.items.version.Turns[0].FieldDataList[0].Content = nil
			case "deadline":
				event.CreatedAt = time.Now().Add(-time.Hour).Unix()
			}
			created := event.CreatedAt
			require.NoError(t, s.Schedule(context.Background(), event))
			require.Equal(t, 1, p.finalizations)
			require.Equal(t, entity.ExptStatus_SystemTerminated, p.terminal.Status)
			require.NotContains(t, p.terminal.StatusMessage, "private")
			require.True(t, p.after)
			require.Empty(t, p.init.writes)
			require.Empty(t, p.init.completes)
			require.Empty(t, ids.counts)
			require.Zero(t, pub.itemDispatches)
			require.Empty(t, pub.published)
			require.Equal(t, created, event.CreatedAt)
			require.NoError(t, s.Schedule(context.Background(), event))
			require.Equal(t, 1, p.finalizations)
			if kind == "deadline" {
				require.Empty(t, p.reads)
			}
		})
	}
}

func TestHookFrozenReadRecoveryContextControls(t *testing.T) {
	for _, kind := range []string{"canceled_entry", "canceled_read", "parent_deadline", "child_deadline", "child_canceled", "conflict"} {
		t.Run(kind, func(t *testing.T) {
			s, p, ids, pub, event := frozenRecoveryScheduler(t, 1)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			parentDone := kind == "canceled_entry" || kind == "canceled_read" || kind == "parent_deadline"
			switch kind {
			case "canceled_entry":
				cancel()
			case "canceled_read":
				p.onRead = cancel
				p.failAt, p.failure = 0, entity.ErrHookFrozenPlanInvalid
			case "parent_deadline":
				var stop context.CancelFunc
				ctx, stop = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
				defer stop()
			case "child_deadline":
				p.failAt, p.failure = 0, fmt.Errorf("private: %w", context.DeadlineExceeded)
			case "child_canceled":
				p.failAt, p.failure = 0, fmt.Errorf("private: %w", context.Canceled)
			case "conflict":
				p.failAt, p.failure = 0, entity.ErrHookStoreConflict
			}
			err := s.Schedule(ctx, event)
			if parentDone {
				require.ErrorIs(t, err, schedulerHookRetryError{})
				require.Empty(t, pub.published)
			} else {
				require.NoError(t, err)
				require.Len(t, pub.published, 1)
				require.Equal(t, event, pub.published[0])
			}
			require.Zero(t, p.finalizations)
			require.False(t, p.after)
			require.Zero(t, pub.itemDispatches)
			require.Empty(t, p.init.writes)
			require.Empty(t, p.init.completes)
			require.Empty(t, ids.counts)
			p.onRead = nil
			require.NoError(t, s.Schedule(context.Background(), event))
			require.True(t, p.init.initialized)
			require.Equal(t, 1, pub.itemDispatches)
		})
	}
}
