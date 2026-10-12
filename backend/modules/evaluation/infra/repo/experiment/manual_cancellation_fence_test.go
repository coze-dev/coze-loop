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
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestManualCancellationFenceUsesSharedParentTransaction(t *testing.T) {
	for _, scenario := range []string{"commit", "rollback", "superseded", "deleted", "managed", "wrong-run-space"} {
		t.Run(scenario, func(t *testing.T) {
			conn, sql, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, sql.ExpectationsWereMet()); _ = conn.Close() })
			p, err := db.NewDB(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Discard})
			require.NoError(t, err)
			key := entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}
			sql.ExpectBegin()
			latest := int64(30)
			var deleted driver.Value
			if scenario == "superseded" {
				latest = 31
			}
			if scenario == "deleted" {
				deleted = time.Unix(500, 0)
			}
			sql.ExpectQuery("SELECT .* FROM `experiment` .* FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "latest_run_id", "status", "deleted_at"}).AddRow(20, 10, latest, 15, deleted))
			if scenario != "superseded" && scenario != "deleted" {
				var marker driver.Value
				changes := map[string]driver.Value{"status": 15, "run_status": 15}
				if scenario == "managed" {
					marker = int64(1)
				}
				if scenario == "wrong-run-space" {
					changes["run_space_id"] = int64(11)
				}
				sql.ExpectQuery("SELECT .* FROM experiment AS e LEFT JOIN expt_run_log AS l ON l.id=\\?").WithArgs(int64(30), int64(20), int64(10)).WillReturnRows(legacySourceRows(marker, changes))
			}
			if scenario == "commit" || scenario == "rollback" {
				sql.ExpectExec("UPDATE `expt_run_log`").WillReturnResult(sqlmock.NewResult(0, 1))
				sql.ExpectExec("UPDATE `experiment`").WillReturnResult(sqlmock.NewResult(0, 1))
				sql.ExpectExec("UPDATE `expt_item_result_run_log`").WillReturnResult(sqlmock.NewResult(0, 1))
			}
			if scenario == "commit" {
				sql.ExpectCommit()
			} else {
				sql.ExpectRollback()
			}
			called := false
			sentinel := errors.New("abort after scoped writes")
			err = NewHookFinalizationRepo(p).(repo.IManualCancellationFenceRepo).WithManualCancellationFence(context.Background(), key, func(ctx context.Context, stores repo.ManualCancellationRepositories) error {
				called = true
				if err := stores.Runs.Update(ctx, 20, 30, map[string]any{"status": int64(entity.ExptStatus_Terminated)}); err != nil {
					return err
				}
				if err := stores.Experiments.Update(ctx, &entity.Experiment{ID: 20, SpaceID: 10, Status: entity.ExptStatus_Terminated}); err != nil {
					return err
				}
				if err := stores.Items.UpdateItemRunLog(ctx, 20, 30, []int64{50}, map[string]any{"status": int32(entity.ItemRunState_Terminal)}, 10); err != nil {
					return err
				}
				if scenario == "rollback" {
					return sentinel
				}
				return nil
			})
			if scenario == "commit" {
				require.NoError(t, err)
				require.True(t, called)
			} else if scenario == "rollback" {
				require.ErrorIs(t, err, sentinel)
				require.True(t, called)
			} else {
				require.Error(t, err)
				require.False(t, called)
			}
		})
	}
}
