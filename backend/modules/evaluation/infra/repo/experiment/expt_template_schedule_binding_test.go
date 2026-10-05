// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const scheduleBindingJSON = `{"schema_version":1,"binding_id":"binding-1","version":1,"user_id":"user-1","identity_type":"fornax_user","space_id":10,"template_id":20,"execution_scope":"local","namespace":"ns","group":"group","biz_key":"template.10.20","job_id":"","enabled":true,"bound_at":"1970-01-01T00:01:40Z","callback":{"psm":"test.evaluation","method":"SubmitScheduledExptFromTemplate","cluster":"default"}}`

func scheduleBindingKey() entity.ExptTemplateScheduleBindingKey {
	return entity.ExptTemplateScheduleBindingKey{SpaceID: 10, TemplateID: 20, ExecutionScope: "local"}
}

func scheduleBindingFixture(t *testing.T) *entity.ExptTemplateScheduleBinding {
	t.Helper()
	var b entity.ExptTemplateScheduleBinding
	require.NoError(t, json.Unmarshal([]byte(scheduleBindingJSON), &b))
	return &b
}

func scheduleReceipt() entity.ExptTemplateScheduleReceipt {
	return entity.ExptTemplateScheduleReceipt{JobID: "job-1", Namespace: "ns", Group: "group", BizKey: "template.10.20",
		Callback: entity.ExptTemplateScheduleCallback{PSM: "test.evaluation", Method: "SubmitScheduledExptFromTemplate", Cluster: "default"}}
}

type scheduleMasterDB struct {
	db.Provider
	t *testing.T
}

func (p scheduleMasterDB) NewSession(ctx context.Context, opts ...db.Option) *gorm.DB {
	require.True(p.t, db.ContainWithMasterOpt(opts), "binding reads must not use a replica")
	return p.Provider.NewSession(ctx, opts...)
}

func (p scheduleMasterDB) Transaction(ctx context.Context, fn func(*gorm.DB) error, opts ...db.Option) error {
	require.True(p.t, db.ContainWithMasterOpt(opts), "CAS must use the primary")
	return p.Provider.Transaction(ctx, fn, opts...)
}

func scheduleBindingStore(t *testing.T) (repo.IExptTemplateScheduleBindingRepo, db.Provider, sqlmock.Sqlmock) {
	t.Helper()
	p, m := initializationDB(t)
	return NewExptTemplateScheduleBindingRepo(scheduleMasterDB{p, t}), p, m
}

func scheduleRead(m sqlmock.Sqlmock, raw any, lock bool) {
	q := "SELECT .*schedule_run_binding.*FROM .expt_template.*id=\\? AND space_id=\\? AND deleted_at IS NULL.*LIMIT \\?"
	if lock {
		q += " FOR UPDATE"
	}
	m.ExpectQuery(q).WithArgs(int64(20), int64(10), 1).
		WillReturnRows(sqlmock.NewRows([]string{"schedule_run_binding"}).AddRow(raw))
}

type scheduleStored struct {
	version   int64
	user, job string
	enabled   bool
}

func (s scheduleStored) Match(value driver.Value) bool {
	b, ok := value.([]byte)
	if !ok {
		if v, yes := value.(string); yes {
			b = []byte(v)
		} else {
			return false
		}
	}
	var got entity.ExptTemplateScheduleBinding
	if json.Unmarshal(b, &got) != nil {
		return false
	}
	return got.SchemaVersion == 1 && got.BindingID == "binding-1" && got.Version == s.version && got.UserID == s.user &&
		got.IdentityType == "fornax_user" && got.SpaceID == 10 && got.TemplateID == 20 && got.ExecutionScope == "local" &&
		got.Namespace == "ns" && got.Group == "group" && got.BizKey == "template.10.20" && got.JobID == s.job && got.Enabled == s.enabled &&
		got.BoundAt.Equal(time.Unix(100, 0)) && got.Callback.PSM == "test.evaluation" &&
		got.Callback.Method == "SubmitScheduledExptFromTemplate" && got.Callback.Cluster == "default" &&
		got.Callback.IDLBranch == "" && got.Callback.VRegion == ""
}

func scheduleWrite(m sqlmock.Sqlmock, old any, next scheduleStored, rows int64) *sqlmock.ExpectedExec {
	q := "UPDATE .expt_template. SET .schedule_run_binding.=\\? WHERE \\(id=\\? AND space_id=\\? AND deleted_at IS NULL\\) AND schedule_run_binding"
	if old == nil {
		return m.ExpectExec(q+" IS NULL").WithArgs(next, int64(20), int64(10)).WillReturnResult(sqlmock.NewResult(0, rows))
	}
	return m.ExpectExec(q+" = \\?").WithArgs(next, int64(20), int64(10), old).WillReturnResult(sqlmock.NewResult(0, rows))
}

func TestExptTemplateScheduleBindingRead(t *testing.T) {
	for _, tc := range []struct {
		name   string
		raw    any
		want   error
		absent bool
	}{
		{"unbound", nil, nil, true},
		{"valid", []byte(scheduleBindingJSON), nil, false},
		{"empty blob is corrupt", []byte{}, entity.ErrExptTemplateScheduleBindingCorrupt, false},
		{"broken json", []byte(`{`), entity.ErrExptTemplateScheduleBindingCorrupt, false},
		{"null envelope", []byte(`null`), entity.ErrExptTemplateScheduleBindingCorrupt, false},
		{"unsupported schema", []byte(strings.Replace(scheduleBindingJSON, `"schema_version":1`, `"schema_version":2`, 1)), entity.ErrExptTemplateScheduleBindingCorrupt, false},
		{"extra secret field", []byte(strings.Replace(scheduleBindingJSON, `"schema_version":1`, `"ticket":"must-not-store","schema_version":1`, 1)), entity.ErrExptTemplateScheduleBindingCorrupt, false},
		{"trailing json", []byte(scheduleBindingJSON + `{}`), entity.ErrExptTemplateScheduleBindingCorrupt, false},
		{"wrong space", []byte(strings.Replace(scheduleBindingJSON, `"space_id":10`, `"space_id":11`, 1)), entity.ErrExptTemplateScheduleBindingConflict, false},
		{"wrong template", []byte(strings.Replace(scheduleBindingJSON, `"template_id":20`, `"template_id":21`, 1)), entity.ErrExptTemplateScheduleBindingConflict, false},
		{"wrong scope", []byte(strings.Replace(scheduleBindingJSON, `"local"`, `"other"`, 1)), entity.ErrExptTemplateScheduleBindingConflict, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, _, m := scheduleBindingStore(t)
			scheduleRead(m, tc.raw, false)
			got, err := r.Get(context.Background(), scheduleBindingKey())
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
				require.Nil(t, got)
				return
			}
			require.NoError(t, err)
			if tc.absent {
				require.Nil(t, got)
			} else {
				require.Equal(t, scheduleBindingFixture(t), got)
			}
		})
	}
	t.Run("missing template", func(t *testing.T) {
		r, _, m := scheduleBindingStore(t)
		m.ExpectQuery("SELECT .*expt_template").WithArgs(int64(20), int64(10), 1).WillReturnRows(sqlmock.NewRows([]string{"schedule_run_binding"}))
		got, err := r.Get(context.Background(), scheduleBindingKey())
		require.ErrorIs(t, err, entity.ErrExptTemplateScheduleBindingMissing)
		require.Nil(t, got)
	})
}

func TestExptTemplateScheduleBindingSaveCAS(t *testing.T) {
	r, _, m := scheduleBindingStore(t)
	b := scheduleBindingFixture(t)
	m.ExpectBegin()
	scheduleRead(m, nil, true)
	scheduleWrite(m, nil, scheduleStored{1, "user-1", "", true}, 1)
	m.ExpectCommit()
	changed, err := r.SaveCAS(context.Background(), scheduleBindingKey(), 0, b)
	require.NoError(t, err)
	require.True(t, changed)
	require.Empty(t, b.JobID)
	// Rebinding must replace only the dedicated blob and advance the version.
	b.Version, b.UserID = 2, "user-2"
	m.ExpectBegin()
	scheduleRead(m, []byte(scheduleBindingJSON), true)
	scheduleWrite(m, []byte(scheduleBindingJSON), scheduleStored{2, "user-2", "", true}, 1)
	m.ExpectCommit()
	changed, err = r.SaveCAS(context.Background(), scheduleBindingKey(), 1, b)
	require.NoError(t, err)
	require.True(t, changed)
}

func TestExptTemplateScheduleBindingCASFences(t *testing.T) {
	for _, tc := range []struct {
		name     string
		raw      any
		expected int64
		want     error
	}{
		{"already bound", []byte(scheduleBindingJSON), 0, entity.ErrExptTemplateScheduleBindingConflict},
		{"old version", []byte(scheduleBindingJSON), 2, entity.ErrExptTemplateScheduleBindingConflict},
		{"binding missing", nil, 1, entity.ErrExptTemplateScheduleBindingConflict},
		{"corrupt binding", []byte(`{}`), 1, entity.ErrExptTemplateScheduleBindingCorrupt},
		{"foreign scope", []byte(strings.Replace(scheduleBindingJSON, `"local"`, `"other"`, 1)), 1, entity.ErrExptTemplateScheduleBindingConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, _, m := scheduleBindingStore(t)
			b := scheduleBindingFixture(t)
			b.Version = tc.expected + 1
			m.ExpectBegin()
			scheduleRead(m, tc.raw, true)
			m.ExpectRollback()
			changed, err := r.SaveCAS(context.Background(), scheduleBindingKey(), tc.expected, b)
			require.ErrorIs(t, err, tc.want)
			require.False(t, changed)
		})
	}
	for _, tc := range []struct {
		name  string
		rows  int64
		dbErr error
	}{
		{"lost row CAS", 0, nil}, {"write failure", 0, errors.New("write failure")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, _, m := scheduleBindingStore(t)
			m.ExpectBegin()
			scheduleRead(m, nil, true)
			x := scheduleWrite(m, nil, scheduleStored{1, "user-1", "", true}, tc.rows)
			if tc.dbErr != nil {
				x.WillReturnError(tc.dbErr)
			}
			m.ExpectRollback()
			changed, err := r.SaveCAS(context.Background(), scheduleBindingKey(), 0, scheduleBindingFixture(t))
			require.Error(t, err)
			require.False(t, changed)
		})
	}
}

func TestExptTemplateScheduleBindingActivation(t *testing.T) {
	active := strings.Replace(scheduleBindingJSON, `"job_id":""`, `"job_id":"job-1"`, 1)
	disabled := strings.Replace(strings.Replace(active, `"enabled":true`, `"enabled":false`, 1), `"version":1`, `"version":2`, 1)
	for _, tc := range []struct {
		name, raw, id string
		version       int64
		want          error
		write         bool
	}{
		{"activate pending", scheduleBindingJSON, "binding-1", 1, nil, true},
		{"duplicate receipt", active, "binding-1", 1, nil, false},
		{"different job", strings.Replace(active, "job-1", "other-job", 1), "binding-1", 1, entity.ErrExptTemplateScheduleBindingConflict, false},
		{"wrong binding", scheduleBindingJSON, "other-binding", 1, entity.ErrExptTemplateScheduleBindingConflict, false},
		{"wrong version", scheduleBindingJSON, "binding-1", 2, entity.ErrExptTemplateScheduleBindingConflict, false},
		{"old receipt after disable", disabled, "binding-1", 1, entity.ErrExptTemplateScheduleBindingConflict, false},
		{"disabled current version", disabled, "binding-1", 2, entity.ErrExptTemplateScheduleBindingConflict, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, _, m := scheduleBindingStore(t)
			m.ExpectBegin()
			scheduleRead(m, []byte(tc.raw), true)
			if tc.write {
				scheduleWrite(m, []byte(tc.raw), scheduleStored{1, "user-1", "job-1", true}, 1)
			}
			if tc.want == nil {
				m.ExpectCommit()
			} else {
				m.ExpectRollback()
			}
			changed, err := r.ActivateCAS(context.Background(), scheduleBindingKey(), tc.id, tc.version, scheduleReceipt())
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.write, changed)
		})
	}
	for _, field := range []string{"namespace", "group", "biz key", "callback psm", "callback method", "callback cluster", "idl branch", "region"} {
		t.Run(field, func(t *testing.T) {
			r, _, m := scheduleBindingStore(t)
			receipt := scheduleReceipt()
			switch field {
			case "namespace":
				receipt.Namespace = "other"
			case "group":
				receipt.Group = "other"
			case "biz key":
				receipt.BizKey = "other"
			case "callback psm":
				receipt.Callback.PSM = "other"
			case "callback method":
				receipt.Callback.Method = "other"
			case "callback cluster":
				receipt.Callback.Cluster = "other"
			case "idl branch":
				receipt.Callback.IDLBranch = "other"
			case "region":
				receipt.Callback.VRegion = "other"
			}
			m.ExpectBegin()
			scheduleRead(m, []byte(scheduleBindingJSON), true)
			m.ExpectRollback()
			changed, err := r.ActivateCAS(context.Background(), scheduleBindingKey(), "binding-1", 1, receipt)
			require.ErrorIs(t, err, entity.ErrExptTemplateScheduleBindingConflict)
			require.False(t, changed)
		})
	}
}

func TestExptTemplateScheduleBindingDisable(t *testing.T) {
	for _, job := range []string{"", "job-1"} {
		t.Run("pending or active "+job, func(t *testing.T) {
			r, _, m := scheduleBindingStore(t)
			raw := strings.Replace(scheduleBindingJSON, `"job_id":""`, `"job_id":"`+job+`"`, 1)
			m.ExpectBegin()
			scheduleRead(m, []byte(raw), true)
			scheduleWrite(m, []byte(raw), scheduleStored{2, "user-1", job, false}, 1)
			m.ExpectCommit()
			changed, err := r.DisableCAS(context.Background(), scheduleBindingKey(), "binding-1", 1)
			require.NoError(t, err)
			require.True(t, changed)
		})
	}
}

func TestExptTemplateScheduleBindingInputRejectedBeforeTransaction(t *testing.T) {
	r, _, _ := scheduleBindingStore(t)
	for _, mutate := range []func(*entity.ExptTemplateScheduleBinding){
		func(b *entity.ExptTemplateScheduleBinding) { b.SpaceID++ },
		func(b *entity.ExptTemplateScheduleBinding) { b.TemplateID++ },
		func(b *entity.ExptTemplateScheduleBinding) { b.ExecutionScope = "other" },
		func(b *entity.ExptTemplateScheduleBinding) { b.Version = 2 },
		func(b *entity.ExptTemplateScheduleBinding) { b.JobID = "unverified-job" },
		func(b *entity.ExptTemplateScheduleBinding) { b.Enabled = false },
		func(b *entity.ExptTemplateScheduleBinding) { b.UserID = "" },
	} {
		b := scheduleBindingFixture(t)
		mutate(b)
		changed, err := r.SaveCAS(context.Background(), scheduleBindingKey(), 0, b)
		require.ErrorIs(t, err, entity.ErrExptTemplateScheduleBindingInvalid)
		require.False(t, changed)
	}
	_, err := r.SaveCAS(context.Background(), scheduleBindingKey(), math.MaxInt64, scheduleBindingFixture(t))
	require.ErrorIs(t, err, entity.ErrExptTemplateScheduleBindingInvalid)
	receipt := scheduleReceipt()
	receipt.JobID = ""
	_, err = r.ActivateCAS(context.Background(), scheduleBindingKey(), "binding-1", 1, receipt)
	require.ErrorIs(t, err, entity.ErrExptTemplateScheduleBindingInvalid)
	_, err = r.DisableCAS(context.Background(), scheduleBindingKey(), "binding-1", 0)
	require.ErrorIs(t, err, entity.ErrExptTemplateScheduleBindingInvalid)
}

func TestExptTemplateScheduleBindingJoinsExistingTransaction(t *testing.T) {
	r, p, m := scheduleBindingStore(t)
	m.ExpectBegin()
	m.ExpectExec("UPDATE expt_template SET name").WithArgs("new name", int64(20)).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec("SAVEPOINT sp[0-9]+").WillReturnResult(sqlmock.NewResult(0, 0))
	scheduleRead(m, nil, true)
	scheduleWrite(m, nil, scheduleStored{1, "user-1", "", true}, 1)
	scheduleRead(m, []byte(scheduleBindingJSON), false)
	m.ExpectRollback()
	abort := errors.New("abort outer template update")
	err := p.Transaction(context.Background(), func(tx *gorm.DB) error {
		if err := tx.Exec("UPDATE expt_template SET name=? WHERE id=?", "new name", int64(20)).Error; err != nil {
			return err
		}
		changed, err := r.SaveCAS(context.Background(), scheduleBindingKey(), 0, scheduleBindingFixture(t), db.WithTransaction(tx))
		require.NoError(t, err)
		require.True(t, changed)
		got, err := r.Get(context.Background(), scheduleBindingKey(), db.WithTransaction(tx))
		require.NoError(t, err)
		require.Equal(t, "binding-1", got.BindingID)
		return abort
	}, db.WithMaster())
	require.ErrorIs(t, err, abort)
}

func TestExptTemplateScheduleBindingDisableFences(t *testing.T) {
	disabled := strings.Replace(scheduleBindingJSON, `"enabled":true`, `"enabled":false`, 1)
	for _, tc := range []struct {
		name    string
		raw     any
		id      string
		version int64
		want    error
	}{
		{"missing", nil, "binding-1", 1, entity.ErrExptTemplateScheduleBindingConflict},
		{"wrong id", []byte(scheduleBindingJSON), "other", 1, entity.ErrExptTemplateScheduleBindingConflict},
		{"old version", []byte(strings.Replace(scheduleBindingJSON, `"version":1`, `"version":2`, 1)), "binding-1", 1, entity.ErrExptTemplateScheduleBindingConflict},
		{"already disabled", []byte(disabled), "binding-1", 1, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, _, m := scheduleBindingStore(t)
			m.ExpectBegin()
			scheduleRead(m, tc.raw, true)
			if tc.want != nil {
				m.ExpectRollback()
			} else {
				m.ExpectCommit()
			}
			changed, err := r.DisableCAS(context.Background(), scheduleBindingKey(), tc.id, tc.version)
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
			} else {
				require.NoError(t, err)
			}
			require.False(t, changed)
		})
	}
}

func TestExptTemplateScheduleBindingDatabaseFailures(t *testing.T) {
	for _, stage := range []string{"begin", "read", "commit"} {
		t.Run(stage, func(t *testing.T) {
			r, _, m := scheduleBindingStore(t)
			failure := errors.New("database " + stage + " failed")
			switch stage {
			case "begin":
				m.ExpectBegin().WillReturnError(failure)
			case "read":
				m.ExpectBegin()
				m.ExpectQuery("SELECT .*expt_template").WithArgs(int64(20), int64(10), 1).WillReturnError(failure)
				m.ExpectRollback()
			case "commit":
				m.ExpectBegin()
				scheduleRead(m, nil, true)
				scheduleWrite(m, nil, scheduleStored{1, "user-1", "", true}, 1)
				m.ExpectCommit().WillReturnError(failure)
			}
			changed, err := r.SaveCAS(context.Background(), scheduleBindingKey(), 0, scheduleBindingFixture(t))
			require.ErrorIs(t, err, failure)
			require.False(t, changed, "a failed commit must never report a durable change")
		})
	}
}

func TestExptTemplateScheduleBindingInvalidContextAndKey(t *testing.T) {
	r, _, _ := scheduleBindingStore(t)
	for _, key := range []entity.ExptTemplateScheduleBindingKey{
		{SpaceID: 0, TemplateID: 20, ExecutionScope: "local"},
		{SpaceID: 10, TemplateID: 0, ExecutionScope: "local"},
		{SpaceID: 10, TemplateID: 20, ExecutionScope: ""},
	} {
		_, err := r.Get(context.Background(), key)
		require.ErrorIs(t, err, entity.ErrExptTemplateScheduleBindingInvalid)
	}
	_, err := r.Get(nil, scheduleBindingKey())
	require.ErrorIs(t, err, entity.ErrExptTemplateScheduleBindingInvalid)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = r.Get(ctx, scheduleBindingKey())
	require.ErrorIs(t, err, context.Canceled)
	_, err = r.ActivateCAS(ctx, scheduleBindingKey(), "binding-1", 1, scheduleReceipt())
	require.ErrorIs(t, err, context.Canceled)
}

func TestExptTemplateScheduleBindingSaveCASUserIDContract(t *testing.T) {
	for _, tc := range []struct {
		name, id string
		valid    bool
	}{
		{"unicode opaque", "tenant:用户-α", true},
		{"beyond int64", "9223372036854775808", true},
		{"leading zero opaque", "00", true},
		{"format character", "user\u200bname", true},
		{"ascii 128 bytes", strings.Repeat("u", 128), true},
		{"unicode 128 bytes", strings.Repeat("界", 42) + "ab", true},
		{"empty", "", false},
		{"zero", "0", false},
		{"invalid utf8", "user\xff", false},
		{"ascii 129 bytes", strings.Repeat("u", 129), false},
		{"unicode 129 bytes", strings.Repeat("界", 43), false},
		{"leading space", " user", false},
		{"trailing space", "user ", false},
		{"unicode nbsp", "user\u00a0name", false},
		{"unicode em space", "user\u2003name", false},
		{"unicode narrow nbsp", "user\u202fname", false},
		{"unicode control", "user\u009fname", false},
		{"newline", "user\nname", false},
		{"tab", "user\tname", false},
		{"nul", "user\x00name", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, _, m := scheduleBindingStore(t)
			b := scheduleBindingFixture(t)
			b.UserID = tc.id
			if tc.valid {
				m.ExpectBegin()
				scheduleRead(m, nil, true)
				scheduleWrite(m, nil, scheduleStored{1, tc.id, "", true}, 1)
				m.ExpectCommit()
			}
			changed, err := r.SaveCAS(context.Background(), scheduleBindingKey(), 0, b)
			if !tc.valid {
				require.ErrorIs(t, err, entity.ErrExptTemplateScheduleBindingInvalid)
				require.False(t, changed)
				return
			}
			require.NoError(t, err)
			require.True(t, changed)
			require.Equal(t, tc.id, b.UserID)
			raw, err := json.Marshal(b)
			require.NoError(t, err)
			scheduleRead(m, raw, false)
			got, err := r.Get(context.Background(), scheduleBindingKey())
			require.NoError(t, err)
			require.Equal(t, tc.id, got.UserID)
		})
	}
}
