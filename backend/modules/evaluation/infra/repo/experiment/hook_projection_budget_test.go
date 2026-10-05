// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"database/sql/driver"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// The model supplies complete database fixtures, never the expected transition.
func budgetProjectionRow(t *testing.T, value any) *sqlmock.Rows {
	t.Helper()
	v := reflect.ValueOf(value)
	var columns []string
	var values []driver.Value
	for i := 0; i < v.NumField(); i++ {
		column := ""
		for _, tag := range strings.Split(v.Type().Field(i).Tag.Get("gorm"), ";") {
			if strings.HasPrefix(tag, "column:") {
				column = strings.TrimPrefix(tag, "column:")
			}
		}
		if column == "" {
			continue
		}
		field := v.Field(i)
		if field.Kind() == reflect.Pointer {
			if field.IsNil() {
				columns = append(columns, column)
				values = append(values, nil)
				continue
			}
			field = field.Elem()
		}
		converted, err := driver.DefaultParameterConverter.ConvertValue(field.Interface())
		require.NoError(t, err)
		columns = append(columns, column)
		values = append(values, converted)
	}
	return sqlmock.NewRows(columns).AddRow(values...)
}

func TestHookProjectionBudgetExhaustionClearsPreviousDisplay(t *testing.T) {
	for _, reason := range []string{"remaining_deadline", "attempt_budget"} {
		for _, policy := range []string{"before_block", "before_continue", "after"} {
			t.Run(reason+"/"+policy, func(t *testing.T) {
				var operationSQL string
				conn, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(want, got string) error {
					if strings.HasPrefix(got, "UPDATE `expt_lifecycle_hook_run`") {
						operationSQL = got
					}
					return sqlmock.QueryMatcherRegexp.Match(want, got)
				})))
				require.NoError(t, err)
				defer conn.Close()
				p, err := db.NewDB(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Discard})
				require.NoError(t, err)
				now := time.Date(2026, 9, 23, 8, 0, 0, 0, time.UTC)
				key := entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}
				hash := strings.Repeat("a", 64)
				phase := entity.HookPhaseBefore
				status := entity.ExptStatus_Processing
				life := model.ExptLifecycleRun{SpaceID: 1, ExptID: 2, ExptRunID: 3, BeforeEnabled: true, PlanState: 1, PlanHash: &hash, SnapshotCipher: []byte("cipher"), SnapshotKeyID: "key", SnapshotHash: hash, ExecutionScope: "budget", Version: 2}
				config := &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.com/hook")}, TimeoutSeconds: gptr.Of(int32(10))}
				if policy == "before_continue" {
					config.OnFailure = gptr.Of(entity.HookFailurePolicyContinue)
				}
				if policy == "after" {
					phase, status = entity.HookPhaseAfter, entity.ExptStatus_Terminated
					life.BeforeEnabled, life.AfterEnabled, life.Gate, life.FinalizeState = false, true, 2, 2
					life.TerminalAt, life.TerminalStatus, life.TerminalReason = &now, gptr.Of(int32(status)), gptr.Of("cancel")
				}
				runlog := model.ExptRunLog{ID: 3, SpaceID: 1, ExptID: 2, ExptRunID: 3, LifecycleHookVersion: gptr.Of(int32(1)), Status: gptr.Of(int64(status)), Mode: gptr.Of(int32(1)), CreatedBy: "user"}
				op := model.ExptLifecycleHookRun{ID: 4, SpaceID: 1, ExptID: 2, ExptRunID: 3, Phase: string(phase), OperationID: string(phase), IdempotencyKey: "stable", Status: "retry_wait", ExecutionScope: "budget", Attempt: 1, LeaseGeneration: 1, Version: 2, ActivatedAt: gptr.Of(now.Add(-75 * time.Second)), OccurredAt: gptr.Of(now.Add(-75 * time.Second)), NextAttemptAt: gptr.Of(now.Add(-70 * time.Second)), AttemptDeadline: gptr.Of(now.Add(-65 * time.Second)), OperationDeadline: gptr.Of(now.Add(5 * time.Second)), ErrorCode: gptr.Of("BUSINESS_RETRY"), ErrorMessage: gptr.Of("previous-business-message"), ResultRedacted: gptr.Of([]byte(`{"old":"previous-result"}`)), UpdatedAt: now}
				if reason == "attempt_budget" {
					op.Attempt, op.LeaseGeneration, op.OperationDeadline = 2, 2, gptr.Of(now.Add(time.Minute))
				}
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT .*FROM .experiment.").WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "latest_run_id", "status"}).AddRow(2, 1, 3, status))
				expectRun := func(l model.ExptLifecycleRun, o model.ExptLifecycleHookRun) {
					mock.ExpectQuery("SELECT .*FROM .expt_lifecycle_run.").WillReturnRows(budgetProjectionRow(t, l))
					mock.ExpectQuery("SELECT .*FROM .expt_run_log.").WillReturnRows(budgetProjectionRow(t, runlog))
					mock.ExpectQuery("SELECT .*FROM .expt_lifecycle_hook_run.").WillReturnRows(budgetProjectionRow(t, o))
				}
				expectRun(life, op)
				mock.ExpectQuery("SELECT .*FROM .expt_lifecycle_hook_attempt.").WillReturnRows(budgetProjectionRow(t, model.ExptLifecycleHookAttempt{ID: 5, OperationID: op.OperationID, Attempt: op.Attempt, LeaseGeneration: op.LeaseGeneration, ResultCategory: gptr.Of("HOOK_FAILED")}))
				mock.ExpectQuery("SELECT CURRENT_TIMESTAMP").WillReturnRows(sqlmock.NewRows([]string{"now"}).AddRow(now))
				update := "UPDATE `expt_lifecycle_hook_run` SET `error_code`=?,`error_message`=?,`lease_generation`=?,`lease_owner`=?,`lease_until`=?,`next_attempt_at`=?,`result_redacted`=?,`status`=?,`updated_at`=CURRENT_TIMESTAMP(3),`version`=? WHERE (space_id=? AND expt_id=? AND expt_run_id=?) AND (id=? AND operation_id=? AND phase=? AND version=? AND attempt=? AND lease_generation=?)"
				mock.ExpectExec(regexp.QuoteMeta(update)).WithArgs("TIMEOUT_UNCERTAIN", nil, op.LeaseGeneration+1, nil, nil, nil, nil, "failed", 3, 1, 2, 3, 4, op.OperationID, op.Phase, 2, op.Attempt, op.LeaseGeneration).WillReturnResult(sqlmock.NewResult(0, 1))
				if phase == entity.HookPhaseBefore {
					mock.ExpectExec("UPDATE .expt_lifecycle_run.").WillReturnResult(sqlmock.NewResult(0, 1))
				}
				finalLife, finalOp := life, op
				if policy == "before_block" {
					finalLife.Gate, finalLife.FinalizeState, finalLife.TerminalAt = 2, 1, &now
					finalLife.TerminalStatus, finalLife.TerminalReason = gptr.Of(int32(entity.ExptStatus_Terminated)), gptr.Of("HOOK_BEFORE_FAILED")
				} else if policy == "before_continue" {
					finalLife.Gate = 1
				}
				finalOp.Status, finalOp.LeaseGeneration, finalOp.Version = "failed", op.LeaseGeneration+1, 3
				finalOp.ErrorCode, finalOp.ErrorMessage, finalOp.ResultRedacted = gptr.Of("TIMEOUT_UNCERTAIN"), nil, nil
				expectRun(finalLife, finalOp)
				mock.ExpectCommit()
				in := entity.HookClaimAttemptInput{HookAttemptScope: entity.HookAttemptScope{Key: key, OperationID: op.OperationID, Phase: phase, ExpectedVersion: 2, ExecutionScope: "budget", SnapshotHash: hash}, Owner: "worker", AttemptID: 6, Config: config}
				out, err := NewHookRunRepo(p).ClaimAttempt(context.Background(), in)
				t.Logf("actual exhausted-claim SQL=%s", operationSQL)
				require.NoError(t, err)
				require.NoError(t, mock.ExpectationsWereMet())
				require.Nil(t, out.Claim)
				require.True(t, out.Changed)
				state := out.Run.State.Before
				if phase == entity.HookPhaseAfter {
					state = out.Run.State.After
				}
				require.Equal(t, entity.HookOperationFailed, state.Status)
				require.Equal(t, op.Attempt, state.Attempt)
				require.False(t, out.Effects.Retry.Retry)
				require.Equal(t, policy == "before_block", out.Effects.BeginFinalize)
				wantGate := entity.HookGateClosed
				if policy == "before_continue" {
					wantGate = entity.HookGateReady
				}
				require.Equal(t, wantGate, out.Run.State.Gate)
				require.Contains(t, operationSQL, "error_message")
				// Exact SQL arguments above prove NULL writes; exercise the real read projection.
				summary, err := hookOperationSummary(hookSummaryRow{OperationID: finalOp.OperationID, Status: finalOp.Status, Attempt: finalOp.Attempt, UpdatedAt: &now, ErrorCode: finalOp.ErrorCode, ErrorMessage: finalOp.ErrorMessage, ResultRedacted: gptr.Indirect(finalOp.ResultRedacted)})
				require.NoError(t, err)
				require.Equal(t, "TIMEOUT_UNCERTAIN", summary.Error.GetCode())
				require.Equal(t, "Hook execution failed", summary.Error.GetMessage())
				require.Nil(t, summary.Response)
			})
		}
	}
}

func TestHookProjectionBudgetExhaustionMySQLReadback(t *testing.T) {
	dsn := os.Getenv("HOOK_MYSQL_TX_DSN")
	if dsn == "" {
		t.Skip("requires isolated HOOK_MYSQL_TX_DSN; prepare-only runs must leave it unset")
	}
	cfg, err := mysqlDriver.ParseDSN(dsn)
	require.NoError(t, err)
	require.Equal(t, "unix", cfg.Net)
	require.Equal(t, "/Users/bytedance/fornax_workspace/_local/scratch/hook-mysql-20260922/m.sock", cfg.Addr)
	require.Equal(t, "hook_7378265404_tx", cfg.DBName)
	require.True(t, cfg.ParseTime)
	require.Equal(t, time.UTC, cfg.Loc)
	require.Equal(t, "'+00:00'", cfg.Params["time_zone"])

	f := newHookTxFixture(t)
	ctx := context.Background()
	in := hookAttemptInput(t, f, true)
	claimed, err := f.repo.ClaimAttempt(ctx, in)
	require.NoError(t, err)
	require.NotNil(t, claimed.Claim)
	require.Equal(t, int32(1), claimed.Claim.Token.Attempt)

	completion := hookCompletion(t, f, in, claimed.Claim)
	completion.Outcome = entity.HookOutcome{Code: entity.HookFailed, HTTPStatus: 200, Retryable: true}
	completion.DisplayErrorCode = "BUSINESS_RETRY"
	completion.ErrorMessage = "previous-business-message"
	completion.ResultRedacted = nil
	completion.ErrorRedacted = []byte(`{"test_audit":"original-business-attempt"}`)
	completed, err := f.repo.CompleteAttempt(ctx, completion)
	require.NoError(t, err)
	require.True(t, completed.Effects.Retry.Retry)
	before := hookAttemptRow(t, f, in.OperationID)
	require.Equal(t, "retry_wait", before.Status)
	require.Equal(t, "BUSINESS_RETRY", gptr.Indirect(before.ErrorCode))
	require.Equal(t, "previous-business-message", gptr.Indirect(before.ErrorMessage))
	require.Equal(t, int32(1), before.Attempt)
	auditBefore := hookAttemptAudit(t, f, in.AttemptID)
	require.Equal(t, "HOOK_FAILED", gptr.Indirect(auditBefore.ResultCategory))
	require.EqualValues(t, 200, gptr.Indirect(auditBefore.HTTPStatus))
	require.Equal(t, []byte(`{"test_audit":"original-business-attempt"}`), gptr.Indirect(auditBefore.ErrorRedacted))
	var auditCount int64
	require.NoError(t, f.sql.Model(&model.ExptLifecycleHookAttempt{}).Where("operation_id=?", in.OperationID).Count(&auditCount).Error)
	require.Equal(t, int64(1), auditCount)

	// Compress this operation's remaining time without touching display fields or audit history.
	adjusted := f.sql.Model(&model.ExptLifecycleHookRun{}).
		Where("space_id=? AND expt_id=? AND expt_run_id=? AND id=? AND operation_id=? AND phase=? AND version=? AND status=?", in.Key.WorkspaceID, in.Key.ExperimentID, in.Key.RunID, before.ID, in.OperationID, "before", before.Version, "retry_wait").
		UpdateColumns(map[string]any{"next_attempt_at": completion.CompletedAt, "attempt_deadline": completion.CompletedAt, "operation_deadline": completion.CompletedAt.Add(time.Millisecond), "version": gorm.Expr("version+1")})
	require.NoError(t, adjusted.Error)
	require.Equal(t, int64(1), adjusted.RowsAffected)
	ready := hookAttemptRow(t, f, in.OperationID)
	require.Equal(t, "previous-business-message", gptr.Indirect(ready.ErrorMessage))
	require.Equal(t, "BUSINESS_RETRY", gptr.Indirect(ready.ErrorCode))

	retry := in
	retry.ExpectedVersion = ready.Version
	retry.AttemptID = hookTxSequence.Add(1)
	exhausted, err := f.repo.ClaimAttempt(ctx, retry)
	require.NoError(t, err)
	require.Nil(t, exhausted.Claim)
	require.True(t, exhausted.Changed)
	require.False(t, exhausted.Effects.Retry.Retry)
	after := hookAttemptRow(t, f, in.OperationID)
	require.Equal(t, "failed", after.Status)
	require.Equal(t, int32(1), after.Attempt)
	require.Equal(t, "TIMEOUT_UNCERTAIN", gptr.Indirect(after.ErrorCode))
	require.Nil(t, after.ErrorMessage)
	require.Nil(t, after.ResultRedacted)
	require.NoError(t, f.sql.Model(&model.ExptLifecycleHookAttempt{}).Where("operation_id=?", in.OperationID).Count(&auditCount).Error)
	require.Equal(t, int64(1), auditCount)
	require.Equal(t, auditBefore, hookAttemptAudit(t, f, in.AttemptID))

	summaries, err := NewHookSummaryRepo(f.p).MGetSummaries(ctx, []entity.HookRunKey{in.Key})
	require.NoError(t, err)
	require.NotNil(t, summaries[in.Key])
	summary := summaries[in.Key].Before
	require.Equal(t, entity.HookOperationFailed, summary.Status)
	require.Nil(t, summary.Response)
	require.Equal(t, "TIMEOUT_UNCERTAIN", summary.Error.GetCode())
	require.Equal(t, "Hook execution failed", summary.Error.GetMessage())
}
