// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/stretchr/testify/require"
)

type committedHookFilterPublisher struct {
	events.ExptEventPublisher
	check func()
}

func (p committedHookFilterPublisher) PublishExptTurnResultFilterEvent(ctx context.Context, event *entity.ExptTurnResultFilterEvent, delay *time.Duration) error {
	p.check()
	return p.ExptEventPublisher.PublishExptTurnResultFilterEvent(ctx, event, delay)
}

func TestHookTerminationPublishesCommittedFilterRefresh(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		count                       int
		superseded, failPublication bool
	}{{name: "latest", count: 2}, {name: "superseded", count: 2, superseded: true}, {name: "bounded_chunks", count: 101}, {name: "best_effort", count: 2, failPublication: true}} {
		t.Run(tc.name, func(t *testing.T) {
			states := make([]entity.ItemRunState, tc.count)
			for i := range states {
				states[i] = entity.ItemRunState_Queueing
			}
			states[len(states)-1] = entity.ItemRunState_Processing
			f, ms := activeTerminationFixture(t, states...)
			f.manager.publisher = committedHookFilterPublisher{ExptEventPublisher: f.manager.publisher, check: func() {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				run, err := f.repo.GetRun(ctx, f.key)
				require.NoError(t, err, "index publication must be outside the database transaction")
				require.Equal(t, entity.HookFinalizeCommitted, run.State.Finalize)
			}}
			if tc.failPublication {
				f.filterRefreshError = errors.New("filter transport unavailable")
			}
			checkLatest := func() {}
			if tc.superseded {
				checkLatest = lateProofSupersede(t, f)
			}
			require.NoError(t, f.manager.Kill(context.Background(), f.expt, &f.key.RunID, f.space, "cancel", nil))
			require.Equal(t, entity.HookFinalizeCommitted, finalizationRead(t, f).State.Finalize)
			if tc.superseded {
				require.Empty(t, f.filterRefreshes)
			} else {
				require.Len(t, f.filterRefreshes, (tc.count+99)/100, "terminated SQL rows need bounded asynchronous index refresh")
				var want, got []int64
				for _, m := range ms {
					want = append(want, m.Frozen.ItemID)
				}
				for _, event := range f.filterRefreshes {
					require.LessOrEqual(t, len(event.ItemID), 100)
					got = append(got, event.ItemID...)
					require.Equal(t, entity.UpsertExptTurnResultFilterTypeAuto, gptr.Indirect(event.FilterType))
				}
				require.ElementsMatch(t, want, got)
			}
			before := len(f.filterRefreshes)
			require.NoError(t, f.manager.FinalizeRun(context.Background(), f.key, entity.HookTerminalIntent{}))
			require.Len(t, f.filterRefreshes, before, "committed replay must not duplicate completion publication")
			checkLatest()
		})
	}
}

func TestHookNormalCompletionDoesNotAddTerminationRefresh(t *testing.T) {
	f := newFinalizationManagerFixture(t)
	require.NoError(t, f.manager.FinalizeRun(context.Background(), f.key, entity.HookTerminalIntent{}))
	require.Empty(t, f.filterRefreshes)
}
