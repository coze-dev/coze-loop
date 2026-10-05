// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func gateManagedRow() map[string]driver.Value {
	return map[string]driver.Value{
		"space_id": int64(10), "expt_id": int64(20), "expt_run_id": int64(30), "before_enabled": true, "after_enabled": false,
		"execution_scope": "platform-local", "gate": int64(0), "plan_state": int64(0), "finalize_state": int64(0), "plan_hash_valid": false,
		"has_terminal_reason": false, "terminal_status": nil, "terminal_at": nil, "plan_count": int64(0), "version": int64(0),
		"operation_row_id": int64(40), "operation_expt_id": int64(20), "operation_id": "hook-before", "operation_scope": "platform-local",
		"phase": "before", "status": "pending", "attempt": int64(0), "updated_at": time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC),
		"activated": false, "lease_generation": int64(0), "lease_until": nil, "attempt_deadline": nil, "operation_deadline": nil, "operation_version": int64(0),
	}
}

func gateManagedRows(values ...map[string]driver.Value) *sqlmock.Rows {
	columns := []string{"space_id", "expt_id", "expt_run_id", "before_enabled", "after_enabled", "execution_scope", "gate", "plan_state", "finalize_state", "plan_hash_valid", "has_terminal_reason", "terminal_status", "terminal_at", "plan_count", "version", "operation_row_id", "operation_expt_id", "operation_id", "operation_scope", "phase", "status", "attempt", "updated_at", "activated", "lease_generation", "lease_until", "attempt_deadline", "operation_deadline", "operation_version"}
	out := sqlmock.NewRows(columns)
	for _, value := range values {
		row := make([]driver.Value, len(columns))
		for i, col := range columns {
			row[i] = value[col]
		}
		out.AddRow(row...)
	}
	return out
}

func TestHookGateManagedState(t *testing.T) {
	now := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name    string
		changes map[string]driver.Value
		gate    entity.HookGateState
		bad     bool
	}{
		{"preparing", nil, entity.HookGateWaiting, false},
		{"pending", map[string]driver.Value{"plan_state": int64(1), "plan_hash_valid": true, "activated": true}, entity.HookGateWaiting, false},
		{"retry wait", map[string]driver.Value{"status": "retry_wait", "attempt": int64(1), "activated": true}, entity.HookGateWaiting, false},
		{"running", map[string]driver.Value{"status": "running", "attempt": int64(1), "activated": true, "lease_generation": int64(1), "lease_until": now.Add(time.Minute), "attempt_deadline": now.Add(30 * time.Second), "operation_deadline": now.Add(2 * time.Minute)}, entity.HookGateWaiting, false},
		{"succeeded", map[string]driver.Value{"status": "succeeded", "attempt": int64(1), "gate": int64(1), "plan_state": int64(1), "plan_hash_valid": true}, entity.HookGateReady, false},
		{"continued failure", map[string]driver.Value{"status": "failed", "attempt": int64(1), "gate": int64(1), "plan_state": int64(1), "plan_hash_valid": true}, entity.HookGateReady, false},
		{"after only", map[string]driver.Value{"before_enabled": false, "after_enabled": true, "phase": "after", "gate": int64(1)}, entity.HookGateReady, false},
		{"closed", map[string]driver.Value{"gate": int64(2), "status": "failed", "finalize_state": int64(1), "terminal_status": int64(13), "terminal_at": time.Now(), "has_terminal_reason": true}, entity.HookGateClosed, false},
		{"unknown gate", map[string]driver.Value{"gate": int64(9)}, entity.HookGateWaiting, true},
		{"unknown plan", map[string]driver.Value{"plan_state": int64(9)}, entity.HookGateWaiting, true},
		{"bad plan hash", map[string]driver.Value{"plan_state": int64(1)}, entity.HookGateWaiting, true},
		{"bad finalize", map[string]driver.Value{"finalize_state": int64(9)}, entity.HookGateWaiting, true},
		{"ready pending", map[string]driver.Value{"gate": int64(1)}, entity.HookGateWaiting, true},
		{"missing after", map[string]driver.Value{"after_enabled": true}, entity.HookGateWaiting, true},
		{"no enabled phases", map[string]driver.Value{"before_enabled": false}, entity.HookGateWaiting, true},
		{"unknown phase", map[string]driver.Value{"phase": "private"}, entity.HookGateWaiting, true},
		{"disabled operation", map[string]driver.Value{"status": "disabled"}, entity.HookGateWaiting, true},
		{"unknown operation", map[string]driver.Value{"status": "private"}, entity.HookGateWaiting, true},
		{"missing operation", map[string]driver.Value{"operation_row_id": nil}, entity.HookGateWaiting, true},
		{"wrong lifecycle owner", map[string]driver.Value{"expt_id": int64(21)}, entity.HookGateWaiting, true},
		{"wrong operation owner", map[string]driver.Value{"operation_expt_id": int64(21)}, entity.HookGateWaiting, true},
		{"wrong platform scope", map[string]driver.Value{"execution_scope": "other"}, entity.HookGateWaiting, true},
		{"wrong operation scope", map[string]driver.Value{"operation_scope": "other"}, entity.HookGateWaiting, true},
		{"bad lifecycle version", map[string]driver.Value{"version": int64(-1)}, entity.HookGateWaiting, true},
		{"bad operation version", map[string]driver.Value{"operation_version": int64(-1)}, entity.HookGateWaiting, true},
		{"bad plan count", map[string]driver.Value{"plan_count": int64(-1)}, entity.HookGateWaiting, true},
		{"bad operation id", map[string]driver.Value{"operation_id": "private\nvalue"}, entity.HookGateWaiting, true},
		{"bad update time", map[string]driver.Value{"updated_at": nil}, entity.HookGateWaiting, true},
		{"negative generation", map[string]driver.Value{"lease_generation": int64(-1)}, entity.HookGateWaiting, true},
		{"too many attempts", map[string]driver.Value{"attempt": int64(12)}, entity.HookGateWaiting, true},
		{"missing terminal time", map[string]driver.Value{"gate": int64(2), "status": "failed", "finalize_state": int64(1), "terminal_status": int64(13)}, entity.HookGateWaiting, true},
		{"ready before plan", map[string]driver.Value{"gate": int64(1), "status": "succeeded", "attempt": int64(1)}, entity.HookGateWaiting, true},
		{"pending with attempt", map[string]driver.Value{"attempt": int64(1)}, entity.HookGateWaiting, true},
		{"succeeded without attempt", map[string]driver.Value{"status": "succeeded"}, entity.HookGateWaiting, true},
		{"running without lease", map[string]driver.Value{"status": "running", "attempt": int64(1)}, entity.HookGateWaiting, true},
		{"premature after", map[string]driver.Value{"before_enabled": false, "after_enabled": true, "phase": "after", "gate": int64(1), "activated": true}, entity.HookGateWaiting, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, m := gateMock(t)
			gateExpectLog(m, gateLogValues(int64(1)))
			row := gateManagedRow()
			for k, v := range tc.changes {
				row[k] = v
			}
			m.ExpectQuery("SELECT .* FROM expt_lifecycle_run AS l LEFT JOIN expt_lifecycle_hook_run AS o .* WHERE .* LIMIT").WithArgs(int64(10), int64(30), 3).WillReturnRows(gateManagedRows(row))
			if tc.bad {
				m.ExpectRollback()
			} else {
				m.ExpectCommit()
			}
			got, err := NewHookGateRepo(p, func(context.Context) (string, error) { return "platform-local", nil }).CanDispatch(context.Background(), gateTestKey)
			if tc.bad {
				require.ErrorIs(t, err, entity.ErrHookGateUnavailable)
				require.NotContains(t, err.Error(), "private")
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.gate, got.Gate)
		})
	}
}

func TestHookGateManagedMissingExtraAndReadFailure(t *testing.T) {
	for _, name := range []string{"missing", "duplicate", "extra", "query failure", "scope failure", "scope empty", "scope nil"} {
		t.Run(name, func(t *testing.T) {
			p, m := gateMock(t)
			gateExpectLog(m, gateLogValues(int64(1)))
			scope := func(context.Context) (string, error) { return "platform-local", nil }
			switch name {
			case "scope failure":
				scope = func(context.Context) (string, error) { return "", errors.New("private scope details") }
			case "scope empty":
				scope = func(context.Context) (string, error) { return "", nil }
			case "scope nil":
				scope = nil
			default:
				q := m.ExpectQuery("SELECT .* FROM expt_lifecycle_run")
				switch name {
				case "query failure":
					q.WillReturnError(errors.New("private SQL"))
				case "missing":
					q.WillReturnRows(gateManagedRows())
				case "duplicate":
					q.WillReturnRows(gateManagedRows(gateManagedRow(), gateManagedRow()))
				case "extra":
					q.WillReturnRows(gateManagedRows(gateManagedRow(), gateManagedRow(), gateManagedRow()))
				}
			}
			m.ExpectRollback()
			got, err := NewHookGateRepo(p, scope).CanDispatch(context.Background(), gateTestKey)
			require.Equal(t, entity.HookGateWaiting, got.Gate)
			require.ErrorIs(t, err, entity.ErrHookGateUnavailable)
		})
	}
}
