// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/stretchr/testify/require"
)

type lookupPlanRepo interface {
	MGetPlanItems(context.Context, entity.HookPlanLookupInput) (*entity.HookPlanLookupResult, error)
}

func planLookupRepo(t *testing.T) (lookupPlanRepo, sqlmock.Sqlmock) {
	t.Helper()
	r, m := planRepo(t)
	lookup, ok := r.(lookupPlanRepo)
	require.True(t, ok, "bounded membership lookup must be implemented by the real plan repository")
	return lookup, m
}

func lookupInput(ids ...int64) entity.HookPlanLookupInput {
	return entity.HookPlanLookupInput{Key: entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}, ExecutionScope: "local", ItemIDs: ids}
}

func lookupRows(rows ...model.ExptLifecycleRunItem) *sqlmock.Rows {
	out := sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "ordinal", "source_space_id", "eval_set_id", "eval_set_version_id", "item_id", "item_version_id"})
	for _, row := range rows {
		out.AddRow(row.ID, row.SpaceID, row.ExptID, row.ExptRunID, row.Ordinal, row.SourceSpaceID, row.EvalSetID, row.EvalSetVersionID, row.ItemID, row.ItemVersionID)
	}
	return out
}

func lookupItem(ordinal int64) model.ExptLifecycleRunItem {
	return model.ExptLifecycleRunItem{ID: 100 + ordinal, SpaceID: 10, ExptID: 20, ExptRunID: 30, Ordinal: ordinal, SourceSpaceID: 11, EvalSetID: 12, EvalSetVersionID: 13, ItemID: 200 + ordinal, ItemVersionID: 300 + ordinal}
}

func lookupExpectQuery(m sqlmock.Sqlmock, in entity.HookPlanLookupInput, count int64, rows *sqlmock.Rows) {
	args := []driver.Value{in.Key.WorkspaceID, in.Key.ExperimentID, in.Key.RunID}
	for _, id := range in.ItemIDs {
		args = append(args, id)
	}
	m.ExpectQuery("SELECT count.*FROM .expt_lifecycle_run_item.*item_id IN").WithArgs(args...).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(count))
	if count > 0 && count <= int64(len(in.ItemIDs)) {
		args = append(args, int64(len(in.ItemIDs)))
		m.ExpectQuery("SELECT .*FROM .expt_lifecycle_run_item.*item_id IN.*ORDER BY item_id ASC, id ASC LIMIT").WithArgs(args...).WillReturnRows(rows)
	}
}

func TestHookPlanLookupMixedMembershipAndOrder(t *testing.T) {
	r, m := planLookupRepo(t)
	in := lookupInput(202, 999, 200)
	m.ExpectBegin()
	planExpectLoad(t, m, planFixture(), true)
	lookupExpectQuery(m, in, 2, lookupRows(lookupItem(0), lookupItem(2)))
	m.ExpectCommit()
	got, err := r.MGetPlanItems(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, &entity.HookPlanLookupResult{Items: []entity.HookPlanItem{{ID: 102, SourceSpaceID: 11, EvalSetID: 12, EvalSetVersionID: 13, ItemID: 202, ItemVersionID: 302}, {ID: 100, SourceSpaceID: 11, EvalSetID: 12, EvalSetVersionID: 13, ItemID: 200, ItemVersionID: 300}}, RunVersion: 4, Count: 3, Ready: false}, got)
	require.Equal(t, []int64{202, 999, 200}, in.ItemIDs)
}

func TestHookPlanLookupInputValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*entity.HookPlanLookupInput)
		valid  bool
	}{
		{"valid", func(*entity.HookPlanLookupInput) {}, true},
		{"empty", func(in *entity.HookPlanLookupInput) { in.ItemIDs = nil }, false},
		{"zero ID", func(in *entity.HookPlanLookupInput) { in.ItemIDs = []int64{0} }, false},
		{"negative ID", func(in *entity.HookPlanLookupInput) { in.ItemIDs = []int64{-1} }, false},
		{"duplicate IDs", func(in *entity.HookPlanLookupInput) { in.ItemIDs = []int64{1, 1} }, false},
		{"too many", func(in *entity.HookPlanLookupInput) {
			in.ItemIDs = make([]int64, 101)
			for i := range in.ItemIDs {
				in.ItemIDs[i] = int64(i + 1)
			}
		}, false},
		{"bad key", func(in *entity.HookPlanLookupInput) { in.Key.ExperimentID = 0 }, false},
		{"missing scope", func(in *entity.HookPlanLookupInput) { in.ExecutionScope = "" }, false},
		{"invalid scope", func(in *entity.HookPlanLookupInput) { in.ExecutionScope = "local lane" }, false},
		{"long scope", func(in *entity.HookPlanLookupInput) { in.ExecutionScope = strings.Repeat("a", 129) }, false},
		{"100 candidates", func(in *entity.HookPlanLookupInput) {
			in.ItemIDs = make([]int64, 100)
			for i := range in.ItemIDs {
				in.ItemIDs[i] = int64(i + 1)
			}
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := lookupInput(200)
			tc.change(&in)
			validator, ok := any(in).(interface{ Validate() error })
			require.True(t, ok, "lookup candidates must be bounded and distinct before querying")
			if tc.valid {
				require.NoError(t, validator.Validate())
			} else {
				want := validator.Validate()
				require.Error(t, want)
				r, _ := planLookupRepo(t)
				got, err := r.MGetPlanItems(context.Background(), in)
				require.Error(t, err)
				status, ok := errorx.FromStatusError(err)
				require.True(t, ok)
				require.Equal(t, int32(errno.CommonInvalidParamCode), status.Code())
				require.Nil(t, got)
			}
		})
	}
}

func TestHookPlanLookupAbsentHistoryAndFutureLedger(t *testing.T) {
	for _, tc := range []struct {
		name  string
		ready bool
		count int64
		rows  []model.ExptLifecycleRunItem
		want  []int64
		bad   bool
	}{
		{"all absent", false, 3, nil, nil, false},
		{"historical preparing", false, 3, []model.ExptLifecycleRunItem{lookupItem(0)}, []int64{200}, false},
		{"ready ignores future", true, 3, []model.ExptLifecycleRunItem{lookupItem(0), lookupItem(3)}, []int64{200}, false},
		{"ready empty prefix", true, 0, []model.ExptLifecycleRunItem{lookupItem(3)}, nil, false},
		{"preparing cannot hide future", false, 3, []model.ExptLifecycleRunItem{lookupItem(3)}, nil, true},
		{"preparing count zero", false, 0, []model.ExptLifecycleRunItem{lookupItem(0)}, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, m := planLookupRepo(t)
			f := planFixture()
			f.expt.LatestRunID = 99
			f.life.PlanCount = tc.count
			if tc.ready {
				f.life.PlanState = 1
				f.life.PlanHash = gptr.Of(strings.Repeat("b", 64))
			}
			in := lookupInput(203, 999, 200)
			m.ExpectBegin()
			planExpectLoad(t, m, f, true)
			lookupExpectQuery(m, in, int64(len(tc.rows)), lookupRows(tc.rows...))
			if tc.bad {
				m.ExpectRollback()
			} else {
				m.ExpectCommit()
			}
			got, err := r.MGetPlanItems(context.Background(), in)
			if tc.bad {
				require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
				require.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got.Items)
			require.Equal(t, tc.count, got.Count)
			require.Equal(t, tc.ready, got.Ready)
			require.Equal(t, int64(4), got.RunVersion)
			ids := make([]int64, 0, len(got.Items))
			for _, item := range got.Items {
				ids = append(ids, item.ItemID)
			}
			require.Equal(t, append([]int64{}, tc.want...), ids)
		})
	}
}

func TestHookPlanLookupRejectsCorruptRows(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func([]model.ExptLifecycleRunItem)
	}{
		{"wrong workspace", func(rows []model.ExptLifecycleRunItem) { rows[0].SpaceID = 99 }},
		{"wrong experiment", func(rows []model.ExptLifecycleRunItem) { rows[0].ExptID = 99 }},
		{"wrong run", func(rows []model.ExptLifecycleRunItem) { rows[0].ExptRunID = 99 }},
		{"unrequested item", func(rows []model.ExptLifecycleRunItem) { rows[0].ItemID = 888 }},
		{"duplicate item", func(rows []model.ExptLifecycleRunItem) { rows[1].ItemID = rows[0].ItemID }},
		{"duplicate record ID", func(rows []model.ExptLifecycleRunItem) { rows[1].ID = rows[0].ID }},
		{"duplicate ordinal", func(rows []model.ExptLifecycleRunItem) { rows[1].Ordinal = rows[0].Ordinal }},
		{"negative ordinal", func(rows []model.ExptLifecycleRunItem) { rows[0].Ordinal = -1 }},
		{"missing record ID", func(rows []model.ExptLifecycleRunItem) { rows[0].ID = 0 }},
		{"missing source workspace", func(rows []model.ExptLifecycleRunItem) { rows[0].SourceSpaceID = 0 }},
		{"missing evalset", func(rows []model.ExptLifecycleRunItem) { rows[0].EvalSetID = 0 }},
		{"invalid evalset version", func(rows []model.ExptLifecycleRunItem) { rows[0].EvalSetVersionID = -1 }},
		{"invalid item version", func(rows []model.ExptLifecycleRunItem) { rows[0].ItemVersionID = -1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, m := planLookupRepo(t)
			in := lookupInput(200, 201)
			rows := []model.ExptLifecycleRunItem{lookupItem(0), lookupItem(1)}
			tc.change(rows)
			m.ExpectBegin()
			planExpectLoad(t, m, planFixture(), true)
			lookupExpectQuery(m, in, 2, lookupRows(rows...))
			m.ExpectRollback()
			got, err := r.MGetPlanItems(context.Background(), in)
			require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
			require.Nil(t, got)
		})
	}
}

func TestHookPlanLookupBoundsAndConsistency(t *testing.T) {
	t.Run("100 candidates", func(t *testing.T) {
		r, m := planLookupRepo(t)
		f := planFixture()
		f.life.PlanCount = 100
		ids := make([]int64, 100)
		rows := make([]model.ExptLifecycleRunItem, 100)
		for i := range ids {
			ids[i] = int64(299 - i)
			rows[i] = lookupItem(int64(i))
		}
		in := lookupInput(ids...)
		m.ExpectBegin()
		planExpectLoad(t, m, f, true)
		lookupExpectQuery(m, in, 100, lookupRows(rows...))
		m.ExpectCommit()
		got, err := r.MGetPlanItems(context.Background(), in)
		require.NoError(t, err)
		require.Len(t, got.Items, 100)
		require.Equal(t, int64(299), got.Items[0].ItemID)
		require.Equal(t, int64(200), got.Items[99].ItemID)
	})
	t.Run("too many rows", func(t *testing.T) {
		r, m := planLookupRepo(t)
		in := lookupInput(200)
		m.ExpectBegin()
		planExpectLoad(t, m, planFixture(), true)
		lookupExpectQuery(m, in, 2, nil)
		m.ExpectRollback()
		got, err := r.MGetPlanItems(context.Background(), in)
		require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
		require.Nil(t, got)
	})
	t.Run("short fetch", func(t *testing.T) {
		r, m := planLookupRepo(t)
		in := lookupInput(200, 201)
		m.ExpectBegin()
		planExpectLoad(t, m, planFixture(), true)
		lookupExpectQuery(m, in, 2, lookupRows(lookupItem(0)))
		m.ExpectRollback()
		got, err := r.MGetPlanItems(context.Background(), in)
		require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
		require.Nil(t, got)
	})
	t.Run("foreign scope", func(t *testing.T) {
		r, m := planLookupRepo(t)
		in := lookupInput(200)
		in.ExecutionScope = "foreign"
		m.ExpectBegin()
		planExpectLoad(t, m, planFixture(), true)
		m.ExpectRollback()
		got, err := r.MGetPlanItems(context.Background(), in)
		require.ErrorIs(t, err, entity.ErrHookStoreConflict)
		require.Nil(t, got)
	})
	t.Run("driver error is safe", func(t *testing.T) {
		r, m := planLookupRepo(t)
		in := lookupInput(200)
		m.ExpectBegin()
		planExpectLoad(t, m, planFixture(), true)
		m.ExpectQuery("SELECT count.*item_id IN").WillReturnError(errors.New("private SQL data"))
		m.ExpectRollback()
		got, err := r.MGetPlanItems(context.Background(), in)
		require.Equal(t, entity.ErrHookPlanStorage, err)
		require.Nil(t, got)
	})
}

func TestHookPlanLookupMissingMarkerAndUnavailable(t *testing.T) {
	in := lookupInput(200)
	got, err := NewHookPlanRepo(nil).MGetPlanItems(context.Background(), in)
	require.ErrorIs(t, err, entity.ErrHookPlanStorage)
	require.Nil(t, got)
	r, m := planLookupRepo(t)
	f := planFixture()
	f.log.LifecycleHookVersion = nil
	m.ExpectBegin()
	m.ExpectQuery("SELECT .*FROM .experiment.*FOR UPDATE").WillReturnRows(budgetProjectionRow(t, f.expt))
	m.ExpectQuery("SELECT .*FROM .expt_lifecycle_run.*FOR UPDATE").WillReturnRows(budgetProjectionRow(t, f.life))
	m.ExpectQuery("SELECT .*FROM .expt_run_log.*FOR UPDATE").WillReturnRows(budgetProjectionRow(t, f.log))
	m.ExpectRollback()
	got, err = r.MGetPlanItems(context.Background(), in)
	require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
	require.Nil(t, got)
}

func TestHookPlanLookupValidatesFutureMetadata(t *testing.T) {
	r, m := planLookupRepo(t)
	f := planFixture()
	f.life.PlanState = 1
	f.life.PlanHash = gptr.Of(strings.Repeat("a", 64))
	row := lookupItem(3)
	row.SourceSpaceID = 0
	in := lookupInput(row.ItemID)
	m.ExpectBegin()
	planExpectLoad(t, m, f, true)
	lookupExpectQuery(m, in, 1, lookupRows(row))
	m.ExpectRollback()
	got, err := r.MGetPlanItems(context.Background(), in)
	require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
	require.Nil(t, got)
}
