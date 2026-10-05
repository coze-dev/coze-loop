// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

func TestHookStorageInvalidRequestsDoNotAccessDB(t *testing.T) {
	r := NewHookRunRepo(nil)
	ctx := context.Background()
	_, err := r.CreateRunWithHooks(ctx, entity.HookCreateRunInput{})
	require.Error(t, err)
	_, err = r.GetRun(ctx, entity.HookRunKey{})
	require.Error(t, err)
	_, err = r.AppendPlanPage(ctx, entity.HookPlanPageInput{})
	require.Error(t, err)
	_, err = r.FinishPlan(ctx, entity.HookFinishPlanInput{})
	require.Error(t, err)
	_, err = r.BeginFinalize(ctx, entity.HookFinalizeInput{})
	require.Error(t, err)
	_, err = r.CommitFinalize(ctx, entity.HookFinalizeInput{})
	require.Error(t, err)
	_, err = r.AdmitItem(ctx, entity.HookAdmitItemInput{})
	require.Error(t, err)
}

func TestHookTxRejectsIncompleteStoredState(t *testing.T) {
	for _, field := range []string{"plan_hash", "mode", "terminal_at"} {
		t.Run(field, func(t *testing.T) {
			f, in, item := readyHookLedger(t)
			ctx := context.Background()
			switch field {
			case "plan_hash":
				require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, in.Key.RunID).UpdateColumn(field, nil).Error)
			case "mode":
				require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, in.Key.RunID).UpdateColumn(field, 0).Error)
			case "terminal_at":
				_, err := f.repo.BeginFinalize(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: 1}, Intent: entity.HookTerminalIntent{Status: 13}})
				require.NoError(t, err)
				require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, in.Key.RunID).UpdateColumn(field, nil).Error)
			}
			_, err := f.repo.GetRun(ctx, in.Key)
			require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
			_, err = f.repo.AdmitItem(ctx, entity.HookAdmitItemInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: 1}, ItemID: item})
			require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
		})
	}
}

func TestHookTxLedgerWritesRollbackWithRunCAS(t *testing.T) {
	for _, method := range []string{"append", "finish", "admit"} {
		t.Run(method, func(t *testing.T) {
			f := newHookTxFixture(t)
			ctx := context.Background()
			in := f.input(method == "finish", 0)
			initial, err := f.repo.CreateRunWithHooks(ctx, in)
			require.NoError(t, err)
			p := hookTestPage(in.Key, 0, 0, "", "done", 1)
			if method != "append" {
				initial, err = f.repo.AppendPlanPage(ctx, p)
				require.NoError(t, err)
			}
			if method == "admit" {
				initial, err = f.repo.FinishPlan(ctx, entity.HookFinishPlanInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: initial.Run.Version}, Count: 1, Hash: strings.Repeat("a", 64)})
				require.NoError(t, err)
			}
			trigger := fmt.Sprintf("hook_cas_fail_%d", f.expt)
			require.NoError(t, f.sql.Exec(fmt.Sprintf("CREATE TRIGGER %s BEFORE UPDATE ON expt_lifecycle_run FOR EACH ROW BEGIN IF NEW.expt_id=%d THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='hook CAS failure'; END IF; END", trigger, f.expt)).Error)
			t.Cleanup(func() { require.NoError(t, f.sql.Exec("DROP TRIGGER "+trigger).Error) })
			switch method {
			case "append":
				_, err = f.repo.AppendPlanPage(ctx, p)
			case "finish":
				_, err = f.repo.FinishPlan(ctx, entity.HookFinishPlanInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: initial.Run.Version}, Count: 1, Hash: strings.Repeat("a", 64)})
			case "admit":
				_, err = f.repo.AdmitItem(ctx, entity.HookAdmitItemInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: initial.Run.Version}, ItemID: p.Items[0].ItemID})
			}
			require.ErrorContains(t, err, "hook CAS failure")
			read, err := f.repo.GetRun(ctx, in.Key)
			require.NoError(t, err)
			require.Equal(t, initial.Run, read)
			var count int64
			require.NoError(t, f.sql.Model(&model.ExptLifecycleRunItem{}).Where("space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, in.Key.RunID).Count(&count).Error)
			if method == "append" {
				require.Zero(t, count)
			} else {
				require.Equal(t, int64(1), count)
				var row model.ExptLifecycleRunItem
				require.NoError(t, f.sql.First(&row, "space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, in.Key.RunID).Error)
				require.Nil(t, row.AdmittedAt)
			}
		})
	}
}

func TestHookTxMissingEnabledPhaseIsCorrupt(t *testing.T) {
	for _, tc := range []struct {
		name, phase string
		beforeDone  bool
	}{
		{"missing after", "after", false},
		{"missing before", "before", false},
		{"missing after following before success", "after", true},
		{"missing successful before", "before", true},
	} {
		for _, method := range []string{"get", "begin", "commit"} {
			t.Run(tc.name+"/"+method, func(t *testing.T) {
				f := newHookTxFixture(t)
				ctx := context.Background()
				in := f.input(true, 0)
				created, err := f.repo.CreateRunWithHooks(ctx, in)
				require.NoError(t, err)
				version := created.Run.Version
				if tc.beforeDone {
					finished, err := f.repo.FinishPlan(ctx, entity.HookFinishPlanInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key}, Hash: strings.Repeat("a", 64)})
					require.NoError(t, err)
					version = finished.Run.Version
					require.NoError(t, f.sql.Model(&model.ExptLifecycleHookRun{}).Where("space_id=? AND expt_id=? AND expt_run_id=? AND phase='before'", f.space, f.expt, in.Key.RunID).UpdateColumns(map[string]any{"status": "succeeded", "next_attempt_at": nil}).Error)
					require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, in.Key.RunID).UpdateColumn("gate", 1).Error)
				}
				request := entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: version}, Intent: entity.HookTerminalIntent{Status: 13}}
				if method == "commit" {
					begun, err := f.repo.BeginFinalize(ctx, request)
					require.NoError(t, err)
					request.ExpectedVersion = begun.Run.Version
				}
				require.NoError(t, f.sql.Delete(&model.ExptLifecycleHookRun{}, "space_id=? AND expt_id=? AND expt_run_id=? AND phase=?", f.space, f.expt, in.Key.RunID, tc.phase).Error)
				var original model.ExptLifecycleRun
				require.NoError(t, f.sql.First(&original, "space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, in.Key.RunID).Error)
				switch method {
				case "get":
					_, err = f.repo.GetRun(ctx, in.Key)
				case "begin":
					_, err = f.repo.BeginFinalize(ctx, request)
				case "commit":
					_, err = f.repo.CommitFinalize(ctx, request)
				}
				require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
				var after model.ExptLifecycleRun
				require.NoError(t, f.sql.First(&after, "space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, in.Key.RunID).Error)
				require.Equal(t, original, after)
			})
		}
	}
}

func TestHookTxExpectedPhaseMarkersRejectInconsistency(t *testing.T) {
	for _, fault := range []string{"neither enabled", "extra before", "extra after", "before disabled", "after disabled"} {
		for _, method := range []string{"get", "begin", "commit"} {
			t.Run(fault+"/"+method, func(t *testing.T) {
				f := newHookTxFixture(t)
				ctx := context.Background()
				in := f.input(true, 0)
				created, err := f.repo.CreateRunWithHooks(ctx, in)
				require.NoError(t, err)
				request := entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: created.Run.Version}, Intent: entity.HookTerminalIntent{Status: 13}}
				if method == "commit" {
					begun, err := f.repo.BeginFinalize(ctx, request)
					require.NoError(t, err)
					request.ExpectedVersion = begun.Run.Version
				}
				switch fault {
				case "neither enabled":
					require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, in.Key.RunID).UpdateColumns(map[string]any{"before_enabled": false, "after_enabled": false}).Error)
				case "extra before":
					require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, in.Key.RunID).UpdateColumns(map[string]any{"before_enabled": false, "after_enabled": true}).Error)
				case "extra after":
					require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, in.Key.RunID).UpdateColumns(map[string]any{"before_enabled": true, "after_enabled": false}).Error)
				case "before disabled", "after disabled":
					phase := strings.TrimSuffix(fault, " disabled")
					require.NoError(t, f.sql.Model(&model.ExptLifecycleHookRun{}).Where("space_id=? AND expt_id=? AND expt_run_id=? AND phase=?", f.space, f.expt, in.Key.RunID, phase).UpdateColumn("status", "disabled").Error)
				}
				var original model.ExptLifecycleRun
				require.NoError(t, f.sql.First(&original, "space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, in.Key.RunID).Error)
				switch method {
				case "get":
					_, err = f.repo.GetRun(ctx, in.Key)
				case "begin":
					_, err = f.repo.BeginFinalize(ctx, request)
				case "commit":
					_, err = f.repo.CommitFinalize(ctx, request)
				}
				require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
				var after model.ExptLifecycleRun
				require.NoError(t, f.sql.First(&after, "space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, in.Key.RunID).Error)
				require.Equal(t, original, after)
			})
		}
	}
}

func TestHookTxSinglePhaseMarkersStayImmutable(t *testing.T) {
	for _, beforeOnly := range []bool{true, false} {
		t.Run(fmt.Sprint(beforeOnly), func(t *testing.T) {
			f := newHookTxFixture(t)
			ctx := context.Background()
			in := f.input(beforeOnly, 0)
			if beforeOnly {
				in.After = nil
			}
			created, err := f.repo.CreateRunWithHooks(ctx, in)
			require.NoError(t, err)
			checkMarkers := func() {
				var stored model.ExptLifecycleRun
				require.NoError(t, f.sql.First(&stored, "space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, in.Key.RunID).Error)
				require.Equal(t, beforeOnly, stored.BeforeEnabled)
				require.Equal(t, !beforeOnly, stored.AfterEnabled)
			}
			checkMarkers()
			replayed, err := f.repo.CreateRunWithHooks(ctx, in)
			require.NoError(t, err)
			require.False(t, replayed.Changed)
			require.Equal(t, created.Run, replayed.Run)
			changed := in
			other := f.input(true, 0)
			if beforeOnly {
				changed.After = other.After
			} else {
				changed.Before = other.Before
			}
			_, err = f.repo.CreateRunWithHooks(ctx, changed)
			require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
			finished, err := f.repo.FinishPlan(ctx, entity.HookFinishPlanInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key}, Hash: strings.Repeat("a", 64)})
			require.NoError(t, err)
			request := entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: finished.Run.Version}, Intent: entity.HookTerminalIntent{Status: 13}}
			begun, err := f.repo.BeginFinalize(ctx, request)
			require.NoError(t, err)
			request.ExpectedVersion = begun.Run.Version
			committed, err := f.repo.CommitFinalize(ctx, request)
			require.NoError(t, err)
			require.Equal(t, !beforeOnly, committed.Effects.ActivateAfter)
			read, err := f.repo.GetRun(ctx, in.Key)
			require.NoError(t, err)
			require.Equal(t, committed.Run, read)
			checkMarkers()
		})
	}
}
