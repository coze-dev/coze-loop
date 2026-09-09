// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	dbmocks "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestItemRunTransitions_AtomicSQLAndRollback(t *testing.T) {
	for _, action := range []string{"yield", "claim", "rollback"} {
		for _, failAt := range []string{"", "owner", "run", "run-write", "item-write", "turn-write", "stats-write", "run-zero", "item-zero", "stats-zero", "commit"} {
			if action == "yield" && failAt == "turn-write" {
				continue
			}
			t.Run(action+"/"+failAt, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				gormDB, primary, replica := newRunLogResolverTestDB(t)
				provider := dbmocks.NewMockProvider(ctrl)
				provider.EXPECT().NewSession(gomock.Any()).Return(gormDB)
				dao := &exptItemResultDAOImpl{provider: provider}
				from := entity.ItemRunState_Processing
				if action == "claim" {
					from = entity.ItemRunState_Queueing
				}
				sentinel := errors.New("transition unavailable")
				primary.ExpectBegin()
				owner := primary.ExpectQuery("SELECT .*expt_item_result.*FOR UPDATE").WithArgs(int64(3), int64(1), int64(7), 1)
				if failAt == "owner" {
					owner.WillReturnError(sentinel)
				} else {
					owner.WillReturnRows(sqlmock.NewRows([]string{"id", "expt_run_id", "status"}).AddRow(11, 100, int32(from)))
					run := primary.ExpectQuery("SELECT .*expt_item_result_run_log.*FOR UPDATE").WithArgs(int64(3), int64(1), int64(100), int64(7), 1)
					if failAt == "run" {
						run.WillReturnError(sentinel)
					} else {
						run.WillReturnRows(sqlmock.NewRows([]string{"id", "status", "retry_times", "result_state"}).AddRow(12, int32(from), 1, 0))
					}
				}
				if failAt != "owner" && failAt != "run" {
					steps := []struct{ name, sql string }{{"run", "UPDATE `expt_item_result_run_log` SET .*WHERE .*space_id.*expt_id.*expt_run_id.*item_id.*status.*retry_times"}, {"item", "UPDATE `expt_item_result` SET .*WHERE .*space_id.*expt_id.*item_id.*expt_run_id.*status"}}
					if action != "yield" {
						steps = append(steps, struct{ name, sql string }{"turn", "UPDATE `expt_turn_result` SET .*WHERE .*space_id.*expt_id.*item_id.*expt_run_id.*status <>"})
					}
					steps = append(steps, struct{ name, sql string }{"stats", "UPDATE `expt_stats` SET .*pending_cnt.*processing_cnt.*WHERE .*space_id.*expt_id"})
					for _, step := range steps {
						write := primary.ExpectExec(step.sql)
						if failAt == step.name+"-write" {
							write.WillReturnError(sentinel)
							break
						}
						if failAt == step.name+"-zero" {
							write.WillReturnResult(sqlmock.NewResult(0, 0))
							break
						}
						write.WillReturnResult(sqlmock.NewResult(0, 1))
					}
				}
				switch failAt {
				case "commit":
					primary.ExpectCommit().WillReturnError(sentinel)
				case "":
					primary.ExpectCommit()
				default:
					primary.ExpectRollback()
				}
				var applied bool
				var err error
				switch action {
				case "yield":
					applied, err = dao.YieldItemRunForRetry(context.Background(), 1, 100, 7, 3, 1, "retryable")
				case "claim":
					applied, err = dao.ClaimItemRunForSubmit(context.Background(), 1, 100, 7, 3, 1)
				case "rollback":
					applied, err = dao.RollbackItemRunSubmit(context.Background(), 1, 100, 7, 3, 1)
				}
				if failAt == "" {
					require.NoError(t, err)
					assert.True(t, applied)
				} else {
					assert.Error(t, err)
					assert.False(t, applied)
				}
				require.NoError(t, primary.ExpectationsWereMet())
				require.NoError(t, replica.ExpectationsWereMet())
			})
		}
	}
}

func TestItemRunTransitions_StaleAndTerminalDoNotWrite(t *testing.T) {
	for _, action := range []string{"yield", "claim", "rollback"} {
		for _, reason := range []string{"new owner", "canonical terminal", "run terminal", "old attempt", "already transitioned", "completed"} {
			if action == "yield" && reason == "completed" {
				continue
			}
			t.Run(action+"/"+reason, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				gormDB, primary, replica := newRunLogResolverTestDB(t)
				provider := dbmocks.NewMockProvider(ctrl)
				provider.EXPECT().NewSession(gomock.Any()).Return(gormDB)
				dao := &exptItemResultDAOImpl{provider: provider}
				status := entity.ItemRunState_Processing
				if action == "claim" {
					status = entity.ItemRunState_Queueing
				}
				owner, canonical := int64(100), status
				if reason == "new owner" {
					owner = 200
				}
				if reason == "canonical terminal" {
					canonical = entity.ItemRunState_Terminal
				}
				primary.ExpectBegin()
				primary.ExpectQuery("SELECT .*expt_item_result.*FOR UPDATE").WithArgs(int64(3), int64(1), int64(7), 1).WillReturnRows(sqlmock.NewRows([]string{"id", "expt_run_id", "status"}).AddRow(11, owner, int32(canonical)))
				if owner == 100 && canonical == status {
					attempt, resultState := int32(1), int32(entity.ExptItemResultStateDefault)
					switch reason {
					case "run terminal":
						status = entity.ItemRunState_Terminal
					case "old attempt":
						attempt = 2
					case "already transitioned":
						status = entity.ItemRunState_Success
					case "completed":
						resultState = int32(entity.ExptItemResultStateLogged)
					}
					primary.ExpectQuery("SELECT .*expt_item_result_run_log.*FOR UPDATE").WithArgs(int64(3), int64(1), int64(100), int64(7), 1).WillReturnRows(sqlmock.NewRows([]string{"id", "status", "retry_times", "result_state"}).AddRow(12, int32(status), attempt, resultState))
				}
				primary.ExpectCommit()
				var applied bool
				var err error
				switch action {
				case "yield":
					applied, err = dao.YieldItemRunForRetry(context.Background(), 1, 100, 7, 3, 1, "retryable")
				case "claim":
					applied, err = dao.ClaimItemRunForSubmit(context.Background(), 1, 100, 7, 3, 1)
				case "rollback":
					applied, err = dao.RollbackItemRunSubmit(context.Background(), 1, 100, 7, 3, 1)
				}
				require.NoError(t, err)
				assert.False(t, applied)
				require.NoError(t, primary.ExpectationsWereMet())
				require.NoError(t, replica.ExpectationsWereMet())
			})
		}
	}
}
