// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type deletionQuotaKey struct {
	scope     string
	run, item int64
}
type deletionQuotaGuard struct {
	component.ICentralReservationGuard
	held         map[deletionQuotaKey]string
	calls        []deletionQuotaKey
	beforeCommit []bool
	committed    func() bool
	failItem     int64
}

func (g *deletionQuotaGuard) Release(_ context.Context, scope string, run, item int64, _ string) error {
	k := deletionQuotaKey{scope, run, item}
	g.calls = append(g.calls, k)
	g.beforeCommit = append(g.beforeCommit, !g.committed())
	if item == g.failItem {
		return errors.New("partial external release failure")
	}
	delete(g.held, k)
	return nil
}

type deletionQuotaItems struct {
	repo.IExptItemResultRepo
	t     *testing.T
	rows  map[int64][]*entity.ExptItemResultRunLog
	scans []int64
}

func (r *deletionQuotaItems) ScanItemRunLogs(_ context.Context, expt, run int64, filter *entity.ExptItemRunLogFilter, cursor, limit, space int64) ([]*entity.ExptItemResultRunLog, int64, error) {
	require.ElementsMatch(r.t, []entity.ItemRunState{entity.ItemRunState_Queueing, entity.ItemRunState_Processing}, filter.Status)
	r.scans = append(r.scans, run)
	return r.rows[run], 0, nil
}

type deletionQuotaPreparedPort struct {
	err       error
	views     []*entity.Experiment
	committed bool
	calls     int
}

func (p *deletionQuotaPreparedPort) HookDeletionScope() string { return "scope" }
func (p *deletionQuotaPreparedPort) DeleteExperiments(context.Context, []int64, int64, string) ([]*entity.Experiment, error) {
	p.calls++
	if p.err != nil {
		return nil, p.err
	}
	if p.committed {
		return nil, nil
	}
	p.committed = true
	return p.views, nil
}

type deletionQuotaLegacyPort struct{ port *deletionQuotaPreparedPort }

func (p deletionQuotaLegacyPort) DeleteExperiments(ctx context.Context, ids []int64, space int64, scope string) ([]*entity.Experiment, error) {
	return p.port.DeleteExperiments(ctx, ids, space, scope)
}

func deletionQuotaFixture(t *testing.T) (*ExptMangerImpl, *deletionQuotaPreparedPort, *deletionQuotaGuard, *deletionQuotaItems) {
	t.Helper()
	base := newTestExptManager(gomock.NewController(t))
	base.finalization = &ExptManagerFinalizationDependencies{ExecutionScope: "scope", Runs: &batchDeleteRunOwner{}, Repository: &batchDeleteFinalization{}}
	views := []*entity.Experiment{{ID: 20, SpaceID: 1, LatestRunID: 31, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: "billing-scope"}, {ID: 21, SpaceID: 1, LatestRunID: 32, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: "billing-scope"}}
	port := &deletionQuotaPreparedPort{views: views}
	guard := &deletionQuotaGuard{held: map[deletionQuotaKey]string{{"billing-scope", 31, 101}: "running", {"billing-scope", 32, 102}: "running"}, committed: func() bool { return port.committed }}
	items := &deletionQuotaItems{t: t, rows: map[int64][]*entity.ExptItemResultRunLog{31: {{ItemID: 101, ExptRunID: 31, Status: int32(entity.ItemRunState_Processing)}}, 32: {{ItemID: 102, ExptRunID: 32, Status: int32(entity.ItemRunState_Processing)}}}}
	base.exptRepo = &batchDeleteManagerReads{rows: views}
	base.centralGuard = guard
	base.itemResultRepo = items
	m, err := NewExptManagerForHookDeletion(base, port, "scope")
	require.NoError(t, err)
	return m, port, guard, items
}

func TestHookBatchDeletionQuotaPureFailureRetainsRunningReservations(t *testing.T) {
	for _, failure := range []error{entity.ErrHookStoreConflict, errors.New("second parent write failed")} {
		m, p, g, items := deletionQuotaFixture(t)
		p.err = failure
		err := m.MDelete(context.Background(), []int64{20, 21}, 1, &entity.Session{UserID: "actor"})
		require.ErrorIs(t, err, failure)
		require.Equal(t, 1, p.calls)
		require.False(t, p.committed)
		require.Empty(t, g.calls, "a failed prepared delete must not release active quota")
		require.Empty(t, items.scans)
		require.Equal(t, map[deletionQuotaKey]string{{"billing-scope", 31, 101}: "running", {"billing-scope", 32, 102}: "running"}, g.held)
	}
}

func TestHookBatchDeletionQuotaPureSuccessUsesCommittedOriginalViews(t *testing.T) {
	m, p, g, items := deletionQuotaFixture(t)
	// The ordinary read can be stale; only the committed transaction view supplies cleanup identity.
	m.exptRepo = &batchDeleteManagerReads{rows: []*entity.Experiment{{ID: 20, SpaceID: 1, LatestRunID: 99, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: "successor-scope"}}}
	g.held[deletionQuotaKey{"successor-scope", 99, 999}] = "running"
	items.rows[99] = []*entity.ExptItemResultRunLog{{ItemID: 999, ExptRunID: 99, Status: int32(entity.ItemRunState_Processing)}}
	require.NoError(t, m.MDelete(context.Background(), []int64{20, 21}, 1, nil))
	require.True(t, p.committed)
	require.ElementsMatch(t, []deletionQuotaKey{{"billing-scope", 31, 101}, {"billing-scope", 32, 102}}, g.calls)
	require.Equal(t, []bool{false, false}, g.beforeCommit)
	require.Equal(t, map[deletionQuotaKey]string{{"successor-scope", 99, 999}: "running"}, g.held)
	require.NoError(t, m.MDelete(context.Background(), []int64{20, 21}, 1, nil))
	require.Len(t, g.calls, 2)
}

func TestHookBatchDeletionQuotaPurePartialReleaseKeepsCommittedBestEffort(t *testing.T) {
	m, p, g, _ := deletionQuotaFixture(t)
	g.failItem = 101
	require.NoError(t, m.MDelete(context.Background(), []int64{20, 21}, 1, nil))
	require.True(t, p.committed)
	require.ElementsMatch(t, []deletionQuotaKey{{"billing-scope", 31, 101}, {"billing-scope", 32, 102}}, g.calls)
	require.Equal(t, []bool{false, false}, g.beforeCommit)
	require.Equal(t, map[deletionQuotaKey]string{{"billing-scope", 31, 101}: "running"}, g.held)
	require.NoError(t, m.MDelete(context.Background(), []int64{20, 21}, 1, nil))
	require.Len(t, g.calls, 2)
}

func TestHookBatchDeletionQuotaPureNonPreparedKeepsLegacyOrder(t *testing.T) {
	m, p, g, _ := deletionQuotaFixture(t)
	m.deletion = nil
	p.err = errors.New("legacy delete failure")
	legacy, err := NewExptManagerWithHookDeletion(m, deletionQuotaLegacyPort{p})
	require.NoError(t, err)
	require.ErrorIs(t, legacy.MDelete(context.Background(), []int64{20, 21}, 1, nil), p.err)
	require.Len(t, g.calls, 2)
	require.Equal(t, []bool{true, true}, g.beforeCommit)
	require.Empty(t, g.held)
}
