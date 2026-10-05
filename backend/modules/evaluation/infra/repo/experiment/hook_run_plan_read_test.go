// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

type planMasterProvider struct {
	db.Provider
	t *testing.T
}

func (p planMasterProvider) Transaction(ctx context.Context, fn func(*gorm.DB) error, opts ...db.Option) error {
	require.True(p.t, db.ContainWithMasterOpt(opts), "plan header and rows must use one primary transaction")
	return p.Provider.Transaction(ctx, fn, opts...)
}

func (p planMasterProvider) NewSession(context.Context, ...db.Option) *gorm.DB {
	p.t.Fatal("plan reads and writes must use the transaction session")
	return nil
}

type planReadFixture struct {
	expt model.Experiment
	life model.ExptLifecycleRun
	log  model.ExptRunLog
	op   model.ExptLifecycleHookRun
}

func planFixture() planReadFixture {
	return planReadFixture{
		expt: model.Experiment{ID: 20, SpaceID: 10, LatestRunID: 30, Status: 3},
		life: model.ExptLifecycleRun{SpaceID: 10, ExptID: 20, ExptRunID: 30, BeforeEnabled: true, Gate: 0, Version: 4, PlanCount: 3, PlanCursor: gptr.Of("c1"), SnapshotCipher: []byte("cipher"), SnapshotKeyID: "key", SnapshotHash: strings.Repeat("a", 64), ExecutionScope: "local"},
		log:  model.ExptRunLog{ID: 30, SpaceID: 10, ExptID: 20, ExptRunID: 30, Status: gptr.Of(int64(3)), Mode: gptr.Of(int32(1)), CreatedBy: "user", LifecycleHookVersion: gptr.Of(int32(1))},
		op:   model.ExptLifecycleHookRun{ID: 40, SpaceID: 10, ExptID: 20, ExptRunID: 30, Phase: "before", OperationID: "hook_40", IdempotencyKey: "stable", Status: "pending", ExecutionScope: "local"},
	}
}

func planRepo(t *testing.T) (repo.IHookPlanRepo, sqlmock.Sqlmock) {
	t.Helper()
	p, m := initializationDB(t)
	r := NewHookPlanRepo(planMasterProvider{p, t})
	return r, m
}

func planExpectLoad(t *testing.T, m sqlmock.Sqlmock, f planReadFixture, parent bool) {
	t.Helper()
	if parent {
		m.ExpectQuery("SELECT .*FROM .experiment.*FOR UPDATE").WillReturnRows(budgetProjectionRow(t, f.expt))
	}
	m.ExpectQuery("SELECT .*FROM .expt_lifecycle_run.*FOR UPDATE").WillReturnRows(budgetProjectionRow(t, f.life))
	m.ExpectQuery("SELECT .*FROM .expt_run_log.*FOR UPDATE").WillReturnRows(budgetProjectionRow(t, f.log))
	m.ExpectQuery("SELECT .*FROM .expt_lifecycle_hook_run.*FOR UPDATE").WillReturnRows(budgetProjectionRow(t, f.op))
}

func planRows(ordinals ...int64) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "ordinal", "source_space_id", "eval_set_id", "eval_set_version_id", "item_id", "item_version_id"})
	for _, n := range ordinals {
		rows.AddRow(100+n, 10, 20, 30, n, 11, 12, 13, 200+n, 300+n)
	}
	return rows
}

func TestHookPlanReadPreparingPage(t *testing.T) {
	r, m := planRepo(t)
	m.ExpectBegin()
	planExpectLoad(t, m, planFixture(), true)
	m.ExpectQuery("SELECT count.*FROM .expt_lifecycle_run_item.*ordinal=").WithArgs(int64(10), int64(20), int64(30), int64(2)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	m.ExpectQuery("SELECT count.*FROM .expt_lifecycle_run_item.*ordinal>=.*ordinal<").WithArgs(int64(10), int64(20), int64(30), int64(0), int64(2)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	m.ExpectQuery("SELECT .*FROM .expt_lifecycle_run_item.*ordinal>=.*ordinal<.*ORDER BY ordinal ASC, id ASC LIMIT").WithArgs(int64(10), int64(20), int64(30), int64(0), int64(2), int64(2)).WillReturnRows(planRows(0, 1))
	m.ExpectCommit()
	got, err := r.ReadPlanPage(context.Background(), entity.HookPlanReadInput{Key: entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}, ExecutionScope: "local", Limit: 2})
	require.NoError(t, err)
	require.Equal(t, &entity.HookPlanReadPage{Items: []entity.HookPlanItem{{ID: 100, SourceSpaceID: 11, EvalSetID: 12, EvalSetVersionID: 13, ItemID: 200, ItemVersionID: 300}, {ID: 101, SourceSpaceID: 11, EvalSetID: 12, EvalSetVersionID: 13, ItemID: 201, ItemVersionID: 301}}, NextOrdinal: 2, Count: 3, RunVersion: 4, HasMore: true}, got)
}

func TestHookPlanAdvanceCursorOnly(t *testing.T) {
	r, m := planRepo(t)
	f := planFixture()
	m.ExpectBegin()
	planExpectLoad(t, m, f, true)
	now := time.Unix(1700000000, 0)
	m.ExpectQuery("SELECT CURRENT_TIMESTAMP").WillReturnRows(sqlmock.NewRows([]string{"now"}).AddRow(now))
	m.ExpectExec("UPDATE .expt_lifecycle_run.*SET .plan_cursor.=.*,.*updated_at.=.*,.*version.=").WithArgs("c2", now, int64(5), int64(10), int64(20), int64(30), int64(4)).WillReturnResult(sqlmock.NewResult(0, 1))
	f.life.PlanCursor = gptr.Of("c2")
	f.life.Version = 5
	planExpectLoad(t, m, f, false)
	m.ExpectCommit()
	got, err := r.AdvancePlanCursor(context.Background(), entity.HookAdvancePlanInput{HookStoreGuard: entity.HookStoreGuard{Key: entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}, ExpectedVersion: 4}, ExecutionScope: "local", ExpectedCount: 3, Cursor: "c1", NextCursor: "c2"})
	require.NoError(t, err)
	require.True(t, got.Changed)
	require.False(t, got.LatestProjected)
	require.Equal(t, "c2", got.Run.PlanCursor)
	require.Equal(t, int64(5), got.Run.Version)
	require.Equal(t, int64(3), got.Run.PlanCount)
	require.False(t, got.Run.PlanReady)
	require.Empty(t, got.Run.PlanHash)
	require.Equal(t, entity.HookGateWaiting, got.Run.State.Gate)
	require.False(t, got.Run.State.Before.Activated)
	require.Empty(t, got.Effects)
}

func TestHookPlanReadPageBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name           string
		count, start   int64
		limit          int32
		history, ready bool
	}{
		{"historical ready prefix", 3, 2, 100, true, true},
		{"end of prefix", 3, 3, 1, false, false},
		{"empty prefix", 0, 0, 100, false, false},
		{"page size one", 3, 1, 1, false, false},
		{"no ordinal overflow", math.MaxInt64, math.MaxInt64 - 1, 100, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, m := planRepo(t)
			f := planFixture()
			f.life.PlanCount = tc.count
			if tc.history {
				f.expt.LatestRunID = 99
				f.expt.DeletedAt = gorm.DeletedAt{Time: time.Unix(100, 0), Valid: true}
				f.life.Gate, f.life.FinalizeState = 2, 2
				f.life.TerminalStatus = gptr.Of(int32(13))
				f.life.TerminalAt = gptr.Of(time.Unix(100, 0))
				f.log.Status = gptr.Of(int64(13))
				f.op.Status = "failed"
			}
			if tc.ready {
				f.life.PlanState = 1
				f.life.PlanHash = gptr.Of(strings.Repeat("b", 64))
			}
			m.ExpectBegin()
			planExpectLoad(t, m, f, true)
			n := min(int64(tc.limit), tc.count-tc.start)
			end := tc.start + n
			if tc.count > 0 && (n == 0 || end < tc.count) {
				m.ExpectQuery("SELECT count.*ordinal=").WithArgs(int64(10), int64(20), int64(30), tc.count-1).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			}
			if n > 0 {
				m.ExpectQuery("SELECT count.*ordinal>=.*ordinal<").WithArgs(int64(10), int64(20), int64(30), tc.start, end).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(n))
				rows := sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "ordinal", "source_space_id", "eval_set_id", "item_id"}).AddRow(101, 10, 20, 30, tc.start, 11, 12, 201)
				m.ExpectQuery("SELECT .*ordinal>=.*ordinal<.*LIMIT").WithArgs(int64(10), int64(20), int64(30), tc.start, end, n).WillReturnRows(rows)
			}
			m.ExpectCommit()
			page, err := r.ReadPlanPage(context.Background(), entity.HookPlanReadInput{Key: entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}, ExecutionScope: "local", StartOrdinal: tc.start, Limit: tc.limit})
			require.NoError(t, err)
			require.NotNil(t, page.Items)
			require.Len(t, page.Items, int(n))
			require.Equal(t, end, page.NextOrdinal)
			require.Equal(t, tc.count, page.Count)
			require.Equal(t, end < tc.count, page.HasMore)
			require.Equal(t, tc.ready, page.Ready)
			if tc.ready {
				require.Equal(t, strings.Repeat("b", 64), page.Hash)
			}
		})
	}
}

func TestHookPlanReadRejectsBrokenPrefix(t *testing.T) {
	for _, tc := range []struct {
		name        string
		tail, count int64
		ordinals    []int64
	}{
		{"missing header tail", 0, 2, nil},
		{"duplicate header tail", 2, 2, nil},
		{"short page count", 1, 1, nil},
		{"extra page count", 1, 3, nil},
		{"missing fetched row", 1, 2, []int64{0}},
		{"duplicate ordinal", 1, 2, []int64{0, 0}},
		{"bad ordinal", 1, 2, []int64{0, 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, m := planRepo(t)
			m.ExpectBegin()
			planExpectLoad(t, m, planFixture(), true)
			m.ExpectQuery("SELECT count.*ordinal=").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(tc.tail))
			if tc.tail == 1 {
				m.ExpectQuery("SELECT count.*ordinal>=.*ordinal<").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(tc.count))
				if tc.count == 2 {
					m.ExpectQuery("SELECT .*ordinal>=.*ordinal<.*LIMIT").WillReturnRows(planRows(tc.ordinals...))
				}
			}
			m.ExpectRollback()
			page, err := r.ReadPlanPage(context.Background(), entity.HookPlanReadInput{Key: entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}, ExecutionScope: "local", Limit: 2})
			require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
			require.Nil(t, page)
		})
	}
}

func TestHookPlanReadOwnershipAndHeader(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*planReadFixture, *entity.HookPlanReadInput)
		want   error
	}{
		{"foreign scope", func(_ *planReadFixture, in *entity.HookPlanReadInput) { in.ExecutionScope = "foreign" }, entity.ErrHookStoreConflict},
		{"past end", func(_ *planReadFixture, in *entity.HookPlanReadInput) { in.StartOrdinal = 4 }, entity.ErrHookStoreConflict},
		{"negative header", func(f *planReadFixture, _ *entity.HookPlanReadInput) { f.life.PlanCount = -1 }, entity.ErrHookStoreCorrupt},
		{"broken ready hash", func(f *planReadFixture, _ *entity.HookPlanReadInput) {
			f.life.PlanState = 1
			f.life.PlanHash = gptr.Of("bad")
		}, entity.ErrHookStoreCorrupt},
		{"wrong operation scope", func(f *planReadFixture, _ *entity.HookPlanReadInput) { f.op.ExecutionScope = "foreign" }, entity.ErrHookStoreCorrupt},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, m := planRepo(t)
			f := planFixture()
			in := entity.HookPlanReadInput{Key: entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}, ExecutionScope: "local", Limit: 2}
			tc.change(&f, &in)
			m.ExpectBegin()
			planExpectLoad(t, m, f, true)
			m.ExpectRollback()
			page, err := r.ReadPlanPage(context.Background(), in)
			require.ErrorIs(t, err, tc.want)
			require.Nil(t, page)
		})
	}
}

func TestHookPlanReadMissingStateAndMarker(t *testing.T) {
	for _, part := range []string{"parent", "lifecycle", "log", "marker", "operation"} {
		t.Run(part, func(t *testing.T) {
			r, m := planRepo(t)
			f := planFixture()
			m.ExpectBegin()
			parent := m.ExpectQuery("SELECT .*FROM .experiment.*FOR UPDATE")
			if part == "parent" {
				parent.WillReturnRows(sqlmock.NewRows([]string{"id"}))
			} else {
				parent.WillReturnRows(budgetProjectionRow(t, f.expt))
				life := m.ExpectQuery("SELECT .*FROM .expt_lifecycle_run.*FOR UPDATE")
				if part == "lifecycle" {
					life.WillReturnRows(sqlmock.NewRows([]string{"space_id"}))
				} else {
					life.WillReturnRows(budgetProjectionRow(t, f.life))
					log := m.ExpectQuery("SELECT .*FROM .expt_run_log.*FOR UPDATE")
					if part == "log" {
						log.WillReturnRows(sqlmock.NewRows([]string{"id"}))
					} else {
						if part == "marker" {
							f.log.LifecycleHookVersion = nil
						}
						log.WillReturnRows(budgetProjectionRow(t, f.log))
						if part != "marker" {
							m.ExpectQuery("SELECT .*FROM .expt_lifecycle_hook_run.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id"}))
						}
					}
				}
			}
			m.ExpectRollback()
			page, err := r.ReadPlanPage(context.Background(), entity.HookPlanReadInput{Key: entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}, ExecutionScope: "local", Limit: 2})
			require.Error(t, err)
			require.Nil(t, page)
		})
	}
}

func TestHookPlanAdvanceFencesAndReplay(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*planReadFixture, *entity.HookAdvancePlanInput)
		replay bool
	}{
		{"last CAS replay", func(f *planReadFixture, _ *entity.HookAdvancePlanInput) {
			f.life.Version = 5
			f.life.PlanCursor = gptr.Of("c2")
		}, true},
		{"stale version", func(_ *planReadFixture, in *entity.HookAdvancePlanInput) { in.ExpectedVersion = 2 }, false},
		{"wrong count", func(_ *planReadFixture, in *entity.HookAdvancePlanInput) { in.ExpectedCount = 2 }, false},
		{"wrong cursor", func(_ *planReadFixture, in *entity.HookAdvancePlanInput) { in.Cursor = "wrong" }, false},
		{"foreign scope", func(_ *planReadFixture, in *entity.HookAdvancePlanInput) { in.ExecutionScope = "foreign" }, false},
		{"not latest", func(f *planReadFixture, _ *entity.HookAdvancePlanInput) { f.expt.LatestRunID = 99 }, false},
		{"closed", func(f *planReadFixture, _ *entity.HookAdvancePlanInput) { f.life.Gate = 2 }, false},
		{"terminal", func(f *planReadFixture, _ *entity.HookAdvancePlanInput) {
			f.log.Status = gptr.Of(int64(13))
			f.life.Gate = 2
		}, false},
		{"terminating", func(f *planReadFixture, _ *entity.HookAdvancePlanInput) { f.log.Status = gptr.Of(int64(15)) }, false},
		{"ready", func(f *planReadFixture, _ *entity.HookAdvancePlanInput) {
			f.life.PlanState = 1
			f.life.PlanHash = gptr.Of(strings.Repeat("b", 64))
		}, false},
		{"deleted", func(f *planReadFixture, _ *entity.HookAdvancePlanInput) {
			f.expt.DeletedAt = gorm.DeletedAt{Time: time.Unix(100, 0), Valid: true}
		}, false},
		{"replay too old", func(f *planReadFixture, _ *entity.HookAdvancePlanInput) {
			f.life.Version = 6
			f.life.PlanCursor = gptr.Of("c2")
		}, false},
		{"replay wrong next", func(f *planReadFixture, in *entity.HookAdvancePlanInput) {
			f.life.Version = 5
			f.life.PlanCursor = gptr.Of("c2")
			in.NextCursor = "c3"
		}, false},
		{"ready cannot replay", func(f *planReadFixture, _ *entity.HookAdvancePlanInput) {
			f.life.Version = 5
			f.life.PlanCursor = gptr.Of("c2")
			f.life.PlanState = 1
			f.life.PlanHash = gptr.Of(strings.Repeat("b", 64))
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, m := planRepo(t)
			f := planFixture()
			in := entity.HookAdvancePlanInput{HookStoreGuard: entity.HookStoreGuard{Key: entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}, ExpectedVersion: 4}, ExecutionScope: "local", ExpectedCount: 3, Cursor: "c1", NextCursor: "c2"}
			tc.change(&f, &in)
			m.ExpectBegin()
			planExpectLoad(t, m, f, true)
			if tc.replay {
				m.ExpectCommit()
			} else {
				m.ExpectRollback()
			}
			got, err := r.AdvancePlanCursor(context.Background(), in)
			if tc.replay {
				require.NoError(t, err)
				require.NotNil(t, got.Run)
				require.Equal(t, int64(5), got.Run.Version)
				require.Equal(t, "c2", got.Run.PlanCursor)
			} else {
				require.ErrorIs(t, err, entity.ErrHookStoreConflict)
				require.Nil(t, got.Run)
			}
			require.False(t, got.Changed)
			require.Empty(t, got.Effects)
		})
	}
}

func TestHookPlanAdvanceRollsBack(t *testing.T) {
	for _, failure := range []string{"update error", "lost CAS", "reload error"} {
		t.Run(failure, func(t *testing.T) {
			r, m := planRepo(t)
			m.ExpectBegin()
			planExpectLoad(t, m, planFixture(), true)
			m.ExpectQuery("SELECT CURRENT_TIMESTAMP").WillReturnRows(sqlmock.NewRows([]string{"now"}).AddRow(time.Unix(1700000000, 0)))
			update := m.ExpectExec("UPDATE .expt_lifecycle_run.*SET .plan_cursor.=.*,.*updated_at.=.*,.*version.=")
			want := entity.ErrHookPlanStorage
			switch failure {
			case "update error":
				update.WillReturnError(errors.New("driver error with private SQL"))
			case "lost CAS":
				update.WillReturnResult(sqlmock.NewResult(0, 0))
				want = entity.ErrHookStoreConflict
			case "reload error":
				update.WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectQuery("SELECT .*FROM .expt_lifecycle_run.*FOR UPDATE").WillReturnError(errors.New("private cipher"))
			}
			m.ExpectRollback()
			got, err := r.AdvancePlanCursor(context.Background(), entity.HookAdvancePlanInput{HookStoreGuard: entity.HookStoreGuard{Key: entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}, ExpectedVersion: 4}, ExecutionScope: "local", ExpectedCount: 3, Cursor: "c1", NextCursor: "c2"})
			require.ErrorIs(t, err, want)
			require.Equal(t, want.Error(), err.Error())
			require.Equal(t, entity.HookStoreResult{}, got)
		})
	}
}

func TestHookPlanUnavailableAndInvalidInput(t *testing.T) {
	read := entity.HookPlanReadInput{Key: entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}, ExecutionScope: "local", Limit: 1}
	advance := entity.HookAdvancePlanInput{HookStoreGuard: entity.HookStoreGuard{Key: read.Key}, ExecutionScope: "local", NextCursor: "next"}
	var typedNil *planMasterProvider
	for _, p := range []db.Provider{nil, typedNil} {
		r := NewHookPlanRepo(p)
		got, err := r.ReadPlanPage(context.Background(), read)
		require.Nil(t, got)
		require.ErrorIs(t, err, entity.ErrHookPlanStorage)
		out, err := r.AdvancePlanCursor(context.Background(), advance)
		require.Empty(t, out)
		require.ErrorIs(t, err, entity.ErrHookPlanStorage)
	}
	r, m := planRepo(t)
	read.Limit = 101
	_, err := r.ReadPlanPage(context.Background(), read)
	require.Error(t, err)
	advance.NextCursor = ""
	_, err = r.AdvancePlanCursor(context.Background(), advance)
	require.Error(t, err)
	read.Limit = 1
	m.ExpectBegin()
	m.ExpectQuery("SELECT .*FROM .experiment.*FOR UPDATE").WillReturnError(errors.New("sensitive SQL"))
	m.ExpectRollback()
	page, err := r.ReadPlanPage(context.Background(), read)
	require.Nil(t, page)
	require.Equal(t, entity.ErrHookPlanStorage, err)
}

func TestHookPlanReadHundredRowBound(t *testing.T) {
	r, m := planRepo(t)
	f := planFixture()
	f.life.PlanCount = 150
	m.ExpectBegin()
	planExpectLoad(t, m, f, true)
	m.ExpectQuery("SELECT count.*ordinal=").WithArgs(int64(10), int64(20), int64(30), int64(149)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	m.ExpectQuery("SELECT count.*ordinal>=.*ordinal<").WithArgs(int64(10), int64(20), int64(30), int64(0), int64(100)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(100))
	ordinals := make([]int64, 100)
	for i := range ordinals {
		ordinals[i] = int64(i)
	}
	m.ExpectQuery("SELECT .*ordinal>=.*ordinal<.*LIMIT").WithArgs(int64(10), int64(20), int64(30), int64(0), int64(100), int64(100)).WillReturnRows(planRows(ordinals...))
	m.ExpectCommit()
	got, err := r.ReadPlanPage(context.Background(), entity.HookPlanReadInput{Key: entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}, ExecutionScope: "local", Limit: 100})
	require.NoError(t, err)
	require.Len(t, got.Items, 100)
	require.Equal(t, int64(100), got.NextOrdinal)
	require.True(t, got.HasMore)
}
