// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application_test

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	driver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

var executorIDs = func() *atomic.Int64 { v := new(atomic.Int64); v.Store(time.Now().UnixNano() / 1000); return v }()

type executorProtector struct{ cipher.AEAD }

func (p executorProtector) Protect(_ context.Context, _ string, b []byte) ([]byte, error) {
	n := make([]byte, p.NonceSize())
	if _, err := rand.Read(n); err != nil {
		return nil, err
	}
	return p.Seal(n, n, b, nil), nil
}
func (p executorProtector) Unprotect(_ context.Context, _ string, b []byte) ([]byte, error) {
	if len(b) < p.NonceSize() {
		return nil, errors.New("invalid cipher")
	}
	return p.Open(nil, b[:p.NonceSize()], b[p.NonceSize():], nil)
}

type executorTransport func(context.Context, entity.HookTransportInput) entity.HookTransportResult

func (f executorTransport) Invoke(c context.Context, in entity.HookTransportInput) entity.HookTransportResult {
	return f(c, in)
}

type executorProjection func(context.Context, entity.HookRunKey, entity.HookPhase, *spi.InvokeExperimentHookResponse) (map[string]string, error)

func (f executorProjection) Project(c context.Context, k entity.HookRunKey, p entity.HookPhase, r *spi.InvokeExperimentHookResponse) (map[string]string, error) {
	return f(c, k, p, r)
}

func (f executorProjection) ProjectError(context.Context, entity.HookRunKey, entity.HookPhase, *spi.InvokeExperimentHookResponse) (*spi.HookError, error) {
	return &spi.HookError{Code: gptr.Of("HOOK_FAILED"), Message: gptr.Of("HOOK_FAILED"), Retryable: gptr.Of(false)}, nil
}

type executorFixture struct {
	sql   *gorm.DB
	p     db.Provider
	repo  repo.IHookRepo
	codec *hookinfra.StorageCodec
	input hookcomponent.AttemptExecutionInput
}

func newExecutorFixture(t *testing.T, phase entity.HookPhase, timeout int32) *executorFixture {
	t.Helper()
	dsn := os.Getenv("HOOK_MYSQL_TX_DSN")
	if dsn == "" {
		t.Skip("requires isolated HOOK_MYSQL_TX_DSN")
	}
	cfg, err := driver.ParseDSN(dsn)
	require.NoError(t, err)
	require.Equal(t, "unix", cfg.Net)
	require.Equal(t, "hook_7378265404_tx", cfg.DBName)
	require.True(t, cfg.ParseTime)
	require.Equal(t, "UTC", cfg.Loc.String())
	require.Equal(t, "'+00:00'", cfg.Params["time_zone"])
	p, err := db.NewDB(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	s := p.NewSession(context.Background())
	sqlDB, err := s.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(8)
	key := entity.HookRunKey{WorkspaceID: executorIDs.Add(1), ExperimentID: executorIDs.Add(1), RunID: executorIDs.Add(1)}
	t.Cleanup(func() {
		require.NoError(t, cleanupExecutorFixture(s, key.WorkspaceID, key.ExperimentID))
	})
	require.NoError(t, s.Create(&model.Experiment{ID: key.ExperimentID, SpaceID: key.WorkspaceID, Name: "executor", Status: 3}).Error)
	block, err := aes.NewCipher(make([]byte, 32))
	require.NoError(t, err)
	aead, err := cipher.NewGCM(block)
	require.NoError(t, err)
	codec := hookinfra.NewStorageCodec(executorProtector{aead})
	conf := &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.com/hook")}, TimeoutSeconds: &timeout, ParametersJSON: gptr.Of(`{"business_key":"generic","number":"9007199254740993"}`)}
	lifecycle := &entity.LifecycleHookConf{}
	if phase == entity.HookPhaseBefore {
		lifecycle.Before = conf
	} else {
		lifecycle.After = conf
	}
	ss, err := entity.NewHookRunSnapshot(entity.HookRunSnapshotInput{Key: key, ExecutionScope: "executor-local", CreatedAt: time.Now(), Config: lifecycle, Context: &spi.HookRunContext{WorkspaceID: gptr.Of(strconv.FormatInt(key.WorkspaceID, 10)), ExperimentID: gptr.Of(strconv.FormatInt(key.ExperimentID, 10)), RunID: gptr.Of(strconv.FormatInt(key.RunID, 10)), RunMode: gptr.Of("submit"), Initiator: &spi.HookInitiator{UserID: gptr.Of("opaque-user"), IdentityType: gptr.Of("fornax_user")}, Experiment: &spi.HookExperimentRef{Name: gptr.Of("original name"), Type: gptr.Of("online")}, EvalSets: []*spi.HookEvalSetRef{}}})
	require.NoError(t, err)
	protected, err := codec.EncodeSnapshot(context.Background(), "local-test-key", ss)
	require.NoError(t, err)
	op := &entity.HookOperationSeed{ID: executorIDs.Add(1), OperationID: fmt.Sprint("hook_executor_", key.RunID), IdempotencyKey: fmt.Sprint("stable_", key.RunID)}
	seed := entity.HookCreateRunInput{Key: key, RunLog: &entity.ExptRunLog{ID: key.RunID, ExptRunID: key.RunID, SpaceID: key.WorkspaceID, ExptID: key.ExperimentID, CreatedBy: "opaque-user", Mode: 1, Status: 3}, Snapshot: protected}
	if phase == entity.HookPhaseBefore {
		seed.Before = op
	} else {
		seed.After = op
	}
	r := experiment.NewHookRunRepo(p)
	created, err := r.CreateRunWithHooks(context.Background(), seed)
	require.NoError(t, err)
	if phase == entity.HookPhaseBefore {
		_, err = r.FinishPlan(context.Background(), entity.HookFinishPlanInput{HookStoreGuard: entity.HookStoreGuard{Key: key, ExpectedVersion: created.Run.Version}, Count: 0, Hash: protected.Hash})
	} else {
		intent := entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated, Reason: "cancel"}
		created, err = r.BeginFinalize(context.Background(), entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: key, ExpectedVersion: created.Run.Version}, Intent: intent})
		require.NoError(t, err)
		_, err = r.CommitFinalize(context.Background(), entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: key, ExpectedVersion: created.Run.Version}, Intent: intent})
	}
	require.NoError(t, err)
	return &executorFixture{sql: s, p: p, repo: r, codec: codec, input: hookcomponent.AttemptExecutionInput{Key: key, OperationID: op.OperationID, Phase: phase, ExecutionScope: "executor-local", Owner: "executor-test", AttemptID: executorIDs.Add(1)}}
}

func (f *executorFixture) executor(t *testing.T, tr hookcomponent.HTTPTransport, projection hookcomponent.CompletionProjector) *service.HookAttemptExecutor {
	t.Helper()
	e, err := service.NewHookAttemptExecutor(f.repo, f.codec, f.codec, tr, projection)
	require.NoError(t, err)
	return e
}
func executorSafeProjection(context.Context, entity.HookRunKey, entity.HookPhase, *spi.InvokeExperimentHookResponse) (map[string]string, error) {
	return map[string]string{"public": "redacted"}, nil
}
func executorSuccess() entity.HookTransportResult {
	return entity.HookTransportResult{Outcome: entity.HookOutcome{Code: entity.HookSucceeded, HTTPStatus: 200}, Response: &spi.InvokeExperimentHookResponse{Status: gptr.Of("succeeded"), Result_: map[string]string{"credential": "must-not-persist"}}, LocalCompletedAt: time.Now()}
}

type executorRuntimeConfig struct {
	mu    sync.Mutex
	value entity.HookRuntimeConfig
	err   error
	reads atomic.Int32
}

func (p *executorRuntimeConfig) GetRuntimeConfig(context.Context) (entity.HookRuntimeConfig, error) {
	p.reads.Add(1)
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.value, p.err
}
func (p *executorRuntimeConfig) change(lease, renew int32) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.value.LeaseSeconds = lease
	p.value.RenewSeconds = renew
}

func TestHookExecutorRuntimeLeaseClaim(t *testing.T) {
	for _, tc := range []struct {
		name               string
		lease, renew, want int32
		legacy             bool
	}{
		{"legacy_default", 0, 0, 30, true}, {"nil_config_default", 0, 0, 30, false}, {"typed_nil_config_default", 0, 0, 30, false}, {"20_5", 20, 5, 20, false}, {"60_20", 60, 20, 60, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newExecutorFixture(t, entity.HookPhaseBefore, 180)
			var config hookcomponent.RuntimeConfigProvider
			if tc.name == "typed_nil_config_default" {
				var provider *executorRuntimeConfig
				config = provider
			}
			if tc.lease != 0 {
				config = &executorRuntimeConfig{value: entity.HookRuntimeConfig{LeaseSeconds: tc.lease, RenewSeconds: tc.renew}}
			}
			tr := executorTransport(func(context.Context, entity.HookTransportInput) entity.HookTransportResult {
				row := f.operation(t)
				audit := model.ExptLifecycleHookAttempt{}
				require.NoError(t, f.sql.Where("operation_id=? AND attempt=?", f.input.OperationID, 1).First(&audit).Error)
				require.Equal(t, time.Duration(tc.want)*time.Second, gptr.Indirect(row.LeaseUntil).Sub(audit.StartedAt), "the configured executor must persist its lease in MySQL")
				return executorSuccess()
			})
			var e *service.HookAttemptExecutor
			var err error
			if tc.legacy {
				var constructor func(repo.IHookRepo, hookcomponent.StorageCodec, hookcomponent.RequestBuilder, hookcomponent.HTTPTransport, hookcomponent.CompletionProjector) (*service.HookAttemptExecutor, error) = service.NewHookAttemptExecutor
				e, err = constructor(f.repo, f.codec, f.codec, tr, executorProjection(executorSafeProjection))
			} else {
				e, err = service.NewHookAttemptExecutorWithConfig(f.repo, f.codec, f.codec, tr, executorProjection(executorSafeProjection), config)
			}
			require.NoError(t, err)
			_, err = e.Execute(context.Background(), f.input)
			require.NoError(t, err)
			require.Equal(t, "succeeded", f.operation(t).Status)
		})
	}
}

func TestHookExecutorRuntimeConfigFailureBeforeClaim(t *testing.T) {
	for _, tc := range []struct {
		name         string
		lease, renew int32
		err          error
	}{
		{"read_error", 20, 5, errors.New("secret provider diagnostic")}, {"zero_lease", 0, 5, nil}, {"zero_renew", 20, 0, nil}, {"negative_lease", -1, 1, nil}, {"half", 20, 10, nil}, {"overflow_safe", 2147483647, 2147483647, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newExecutorFixture(t, entity.HookPhaseBefore, 180)
			provider := &executorRuntimeConfig{value: entity.HookRuntimeConfig{LeaseSeconds: tc.lease, RenewSeconds: tc.renew}, err: tc.err}
			var calls atomic.Int32
			e, err := service.NewHookAttemptExecutorWithConfig(f.repo, f.codec, f.codec, executorTransport(func(context.Context, entity.HookTransportInput) entity.HookTransportResult {
				calls.Add(1)
				return executorSuccess()
			}), executorProjection(executorSafeProjection), provider)
			require.NoError(t, err)
			_, err = e.Execute(context.Background(), f.input)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "secret")
			require.Zero(t, calls.Load())
			require.Zero(t, f.operation(t).Attempt)
		})
	}
}

func TestHookExecutorRuntimeRenewPinsConfiguration(t *testing.T) {
	f := newExecutorFixture(t, entity.HookPhaseBefore, 180)
	provider := &executorRuntimeConfig{value: entity.HookRuntimeConfig{LeaseSeconds: 20, RenewSeconds: 5}}
	original := f.repo
	renewed := make(chan entity.HookAttemptStoreResult, 1)
	f.repo = &executorRepoProbe{IHookRepo: original, renew: func(ctx context.Context, in entity.HookRenewAttemptInput) (entity.HookAttemptStoreResult, error) {
		out, err := original.RenewAttempt(ctx, in)
		if err == nil {
			renewed <- out
		}
		return out, err
	}}
	tr := executorTransport(func(ctx context.Context, _ entity.HookTransportInput) entity.HookTransportResult {
		row := f.operation(t)
		var audit model.ExptLifecycleHookAttempt
		require.NoError(t, f.sql.Where("operation_id=? AND attempt=?", f.input.OperationID, 1).First(&audit).Error)
		require.Equal(t, 20*time.Second, gptr.Indirect(row.LeaseUntil).Sub(audit.StartedAt))
		provider.change(60, 20)
		select {
		case out := <-renewed:
			require.Equal(t, 20*time.Second, out.Claim.LeaseUntil.Sub(out.Clock.DBTime), "renew must retain this execution's 20/5 despite config mutation")
			require.Equal(t, int32(1), provider.reads.Load())
			return executorSuccess()
		case <-ctx.Done():
			return entity.HookTransportResult{Outcome: entity.HookOutcome{Code: entity.HookTimeoutUncertain}, LocalCompletedAt: time.Now()}
		}
	})
	e, err := service.NewHookAttemptExecutorWithConfig(f.repo, f.codec, f.codec, tr, executorProjection(executorSafeProjection), provider)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	_, err = e.Execute(ctx, f.input)
	require.NoError(t, err)
	require.Equal(t, "succeeded", f.operation(t).Status)
	require.Equal(t, int32(1), f.operation(t).Attempt)
}

func TestHookExecutorRuntimeCancellationAndExpiry(t *testing.T) {
	for _, kind := range []string{"parent_cancel", "lease_expiry"} {
		t.Run(kind, func(t *testing.T) {
			f := newExecutorFixture(t, entity.HookPhaseBefore, 180)
			provider := &executorRuntimeConfig{value: entity.HookRuntimeConfig{LeaseSeconds: 20, RenewSeconds: 5}}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			var invoked atomic.Bool
			if kind == "lease_expiry" {
				original := f.repo
				f.repo = &executorRepoProbe{IHookRepo: original, claim: func(ctx context.Context, in entity.HookClaimAttemptInput) (entity.HookAttemptStoreResult, error) {
					out, err := original.ClaimAttempt(ctx, in)
					if err != nil {
						return out, err
					}
					out.Claim.LeaseUntil = out.Clock.DBTime.Add(200 * time.Millisecond)
					err = f.sql.WithContext(ctx).Model(&model.ExptLifecycleHookRun{}).Where("operation_id=?", in.OperationID).UpdateColumn("lease_until", out.Claim.LeaseUntil).Error
					if err != nil {
						return out, err
					}
					return original.(hookcomponent.AttemptReader).ReadAttempt(ctx, in.HookAttemptScope)
				}}
			}
			e, err := service.NewHookAttemptExecutorWithConfig(f.repo, f.codec, f.codec, executorTransport(func(invokeCtx context.Context, _ entity.HookTransportInput) entity.HookTransportResult {
				invoked.Store(true)
				if kind == "parent_cancel" {
					cancel()
				}
				<-invokeCtx.Done()
				return executorSuccess()
			}), executorProjection(executorSafeProjection), provider)
			require.NoError(t, err)
			_, err = e.Execute(ctx, f.input)
			if kind == "parent_cancel" {
				require.ErrorIs(t, err, context.Canceled)
			} else {
				require.ErrorIs(t, err, service.ErrHookExecutionLeaseLost)
			}
			require.True(t, invoked.Load())
			require.Equal(t, int32(1), provider.reads.Load())
			require.Equal(t, "running", f.operation(t).Status)
			var audit model.ExptLifecycleHookAttempt
			require.NoError(t, f.sql.Where("operation_id=?", f.input.OperationID).First(&audit).Error)
			require.Nil(t, audit.ResultCategory, "a late success cannot complete a cancelled or expired lease")
		})
	}
}
func (f *executorFixture) operation(t *testing.T) model.ExptLifecycleHookRun {
	t.Helper()
	var v model.ExptLifecycleHookRun
	require.NoError(t, f.sql.Where("operation_id=?", f.input.OperationID).First(&v).Error)
	return v
}

func TestHookExecutorSuccessDuplicateAndFrozenRequest(t *testing.T) {
	f := newExecutorFixture(t, entity.HookPhaseBefore, 10)
	var calls atomic.Int32
	e := f.executor(t, executorTransport(func(_ context.Context, in entity.HookTransportInput) entity.HookTransportResult {
		calls.Add(1)
		require.Equal(t, "original name", in.Request.Context.Experiment.GetName())
		require.Equal(t, "opaque-user", in.Request.Context.Initiator.GetUserID())
		require.Equal(t, int32(1), in.Request.GetAttempt())
		require.Equal(t, `{"business_key":"generic","number":"9007199254740993"}`, in.Request.GetParameters())
		require.Positive(t, in.Remaining)
		require.Less(t, in.Remaining, 10*time.Second)
		return executorSuccess()
	}), executorProjection(executorSafeProjection))
	out, err := e.Execute(context.Background(), f.input)
	require.NoError(t, err)
	require.Equal(t, entity.HookGateReady, out.Run.State.Gate)
	require.False(t, out.RecoveryPending)
	row := f.operation(t)
	require.Equal(t, "succeeded", row.Status)
	require.JSONEq(t, `{"public":"redacted"}`, string(gptr.Indirect(row.ResultRedacted)))
	require.Nil(t, row.ErrorMessage)
	_, err = e.Execute(context.Background(), f.input)
	require.NoError(t, err)
	require.Equal(t, int32(1), calls.Load())
	require.Equal(t, int32(1), f.operation(t).Attempt)
}

func TestHookExecutorUncertainOnlyRecoversOriginalAttempt(t *testing.T) {
	f := newExecutorFixture(t, entity.HookPhaseBefore, 10)
	var calls atomic.Int32
	e := f.executor(t, executorTransport(func(context.Context, entity.HookTransportInput) entity.HookTransportResult {
		calls.Add(1)
		return entity.HookTransportResult{Outcome: entity.HookOutcome{Code: entity.HookTimeoutUncertain, Retryable: true}, LocalCompletedAt: time.Now()}
	}), executorProjection(executorSafeProjection))
	for i := 0; i < 2; i++ {
		out, err := e.Execute(context.Background(), f.input)
		require.NoError(t, err)
		require.True(t, out.RecoveryPending)
		require.Equal(t, "running", f.operation(t).Status)
	}
	require.Equal(t, int32(1), calls.Load())
	require.NoError(t, f.sql.Exec("UPDATE expt_lifecycle_hook_run SET lease_until=DATE_SUB(CURRENT_TIMESTAMP(3),INTERVAL 1 SECOND),attempt_deadline=DATE_SUB(CURRENT_TIMESTAMP(3),INTERVAL 1 SECOND) WHERE operation_id=?", f.input.OperationID).Error)
	out, err := e.Execute(context.Background(), f.input)
	require.NoError(t, err)
	require.True(t, out.Effects.Retry.Retry)
	require.Equal(t, "retry_wait", f.operation(t).Status)
	require.Equal(t, int32(1), f.operation(t).Attempt)
	var audit model.ExptLifecycleHookAttempt
	require.NoError(t, f.sql.Where("operation_id=?", f.input.OperationID).First(&audit).Error)
	require.Equal(t, "TIMEOUT_UNCERTAIN", *audit.ResultCategory)
	require.Nil(t, audit.FinishedAt)
	require.Equal(t, int32(1), calls.Load())
}

func TestHookExecutorAfterOldRunAndSoftDelete(t *testing.T) {
	f := newExecutorFixture(t, entity.HookPhaseAfter, 10)
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.input.Key.ExperimentID).UpdateColumn("latest_run_id", f.input.Key.RunID+100).Error)
	require.NoError(t, f.sql.Delete(&model.Experiment{}, "id=?", f.input.Key.ExperimentID).Error)
	e := f.executor(t, executorTransport(func(_ context.Context, in entity.HookTransportInput) entity.HookTransportResult {
		require.Equal(t, strconv.FormatInt(f.input.Key.RunID, 10), in.Request.Context.GetRunID())
		require.Equal(t, "terminated", in.Request.Context.GetTerminalStatus())
		require.Equal(t, "cancel", in.Request.Context.GetTerminalReason())
		return executorSuccess()
	}), executorProjection(executorSafeProjection))
	out, err := e.Execute(context.Background(), f.input)
	require.NoError(t, err)
	require.Equal(t, entity.HookGateClosed, out.Run.State.Gate)
	require.Equal(t, "succeeded", f.operation(t).Status)
	require.False(t, out.LatestProjected)
}

func TestHookExecutorProjectionRequiredAndFailureSafe(t *testing.T) {
	f := newExecutorFixture(t, entity.HookPhaseBefore, 10)
	_, err := service.NewHookAttemptExecutor(f.repo, f.codec, f.codec, executorTransport(func(context.Context, entity.HookTransportInput) entity.HookTransportResult { return executorSuccess() }), nil)
	require.Error(t, err)
	e := f.executor(t, executorTransport(func(context.Context, entity.HookTransportInput) entity.HookTransportResult { return executorSuccess() }), executorProjection(func(context.Context, entity.HookRunKey, entity.HookPhase, *spi.InvokeExperimentHookResponse) (map[string]string, error) {
		return nil, errors.New("secret=user@example.com")
	}))
	out, err := e.Execute(context.Background(), f.input)
	require.NoError(t, err)
	require.True(t, out.Effects.BeginFinalize)
	row := f.operation(t)
	require.Equal(t, "failed", row.Status)
	require.Empty(t, row.ResultRedacted)
	require.NotContains(t, gptr.Indirect(row.ErrorMessage), "secret")
	require.Equal(t, "HOOK_SECURITY_ERROR", gptr.Indirect(row.ErrorCode))
}

type executorRepoProbe struct {
	repo.IHookRepo
	read     func(context.Context, entity.HookAttemptScope) (entity.HookAttemptStoreResult, error)
	complete func(context.Context, entity.HookCompleteAttemptInput) (entity.HookAttemptStoreResult, error)
	claim    func(context.Context, entity.HookClaimAttemptInput) (entity.HookAttemptStoreResult, error)
	renew    func(context.Context, entity.HookRenewAttemptInput) (entity.HookAttemptStoreResult, error)
}

func (p *executorRepoProbe) ReadAttempt(c context.Context, in entity.HookAttemptScope) (entity.HookAttemptStoreResult, error) {
	if p.read != nil {
		return p.read(c, in)
	}
	return p.IHookRepo.(hookcomponent.AttemptReader).ReadAttempt(c, in)
}
func (p *executorRepoProbe) CompleteAttempt(c context.Context, in entity.HookCompleteAttemptInput) (entity.HookAttemptStoreResult, error) {
	if p.complete != nil {
		return p.complete(c, in)
	}
	return p.IHookRepo.CompleteAttempt(c, in)
}
func (p *executorRepoProbe) ClaimAttempt(c context.Context, in entity.HookClaimAttemptInput) (entity.HookAttemptStoreResult, error) {
	if p.claim != nil {
		return p.claim(c, in)
	}
	return p.IHookRepo.ClaimAttempt(c, in)
}
func (p *executorRepoProbe) RenewAttempt(c context.Context, in entity.HookRenewAttemptInput) (entity.HookAttemptStoreResult, error) {
	if p.renew != nil {
		return p.renew(c, in)
	}
	return p.IHookRepo.RenewAttempt(c, in)
}

func TestHookExecutorCompletionRetriesRenewVersionWithOriginalTime(t *testing.T) {
	f := newExecutorFixture(t, entity.HookPhaseBefore, 10)
	original := f.repo
	var observed []time.Time
	f.repo = &executorRepoProbe{IHookRepo: original, complete: func(c context.Context, in entity.HookCompleteAttemptInput) (entity.HookAttemptStoreResult, error) {
		observed = append(observed, in.CompletedAt)
		if len(observed) == 1 {
			_, err := original.RenewAttempt(c, in.HookRenewAttemptInput)
			if err != nil {
				return entity.HookAttemptStoreResult{}, err
			}
		}
		return original.CompleteAttempt(c, in)
	}}
	e := f.executor(t, executorTransport(func(context.Context, entity.HookTransportInput) entity.HookTransportResult { return executorSuccess() }), executorProjection(executorSafeProjection))
	out, err := e.Execute(context.Background(), f.input)
	require.NoError(t, err)
	require.Equal(t, "succeeded", f.operation(t).Status)
	require.Len(t, observed, 2)
	require.Equal(t, observed[0], observed[1])
	require.Equal(t, observed[0], out.CompletionWindow.Latest)
}

func TestHookExecutorRenewsDuringSingleHTTP(t *testing.T) {
	f := newExecutorFixture(t, entity.HookPhaseBefore, 60)
	original := f.repo
	renewed := make(chan struct{})
	var once sync.Once
	f.repo = &executorRepoProbe{IHookRepo: original, renew: func(c context.Context, in entity.HookRenewAttemptInput) (entity.HookAttemptStoreResult, error) {
		out, err := original.RenewAttempt(c, in)
		if err == nil {
			once.Do(func() { close(renewed) })
		}
		return out, err
	}}
	var calls atomic.Int32
	e := f.executor(t, executorTransport(func(c context.Context, _ entity.HookTransportInput) entity.HookTransportResult {
		calls.Add(1)
		select {
		case <-renewed:
			return executorSuccess()
		case <-c.Done():
			return entity.HookTransportResult{Outcome: entity.HookOutcome{Code: entity.HookTimeoutUncertain}, LocalCompletedAt: time.Now()}
		}
	}), executorProjection(executorSafeProjection))
	c, cancel := context.WithTimeout(context.Background(), 13*time.Second)
	defer cancel()
	out, err := e.Execute(c, f.input)
	require.NoError(t, err)
	require.False(t, out.RecoveryPending)
	require.Equal(t, "succeeded", f.operation(t).Status)
	require.Equal(t, int32(1), calls.Load())
}

func TestHookExecutorLeaseExpiryCancelsLocalHTTP(t *testing.T) {
	f := newExecutorFixture(t, entity.HookPhaseBefore, 10)
	original := f.repo
	f.repo = &executorRepoProbe{IHookRepo: original, claim: func(c context.Context, in entity.HookClaimAttemptInput) (entity.HookAttemptStoreResult, error) {
		out, err := original.ClaimAttempt(c, in)
		if err != nil {
			return out, err
		}
		err = f.sql.Exec("UPDATE expt_lifecycle_hook_run SET lease_until=DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL 100000 MICROSECOND) WHERE operation_id=?", in.OperationID).Error
		if err != nil {
			return out, err
		}
		return original.(hookcomponent.AttemptReader).ReadAttempt(c, in.HookAttemptScope)
	}}
	var stopped atomic.Bool
	e := f.executor(t, executorTransport(func(c context.Context, _ entity.HookTransportInput) entity.HookTransportResult {
		<-c.Done()
		stopped.Store(true)
		return executorSuccess()
	}), executorProjection(executorSafeProjection))
	c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := e.Execute(c, f.input)
	require.Error(t, err)
	require.NoError(t, c.Err(), "lease loss must stop HTTP before the parent deadline")
	require.True(t, stopped.Load())
	require.Equal(t, "running", f.operation(t).Status)
	require.Equal(t, int32(1), f.operation(t).Attempt)
}

func TestHookExecutorParentCancellationNeverCompletes(t *testing.T) {
	f := newExecutorFixture(t, entity.HookPhaseBefore, 10)
	c, cancel := context.WithCancel(context.Background())
	defer cancel()
	e := f.executor(t, executorTransport(func(c context.Context, _ entity.HookTransportInput) entity.HookTransportResult {
		cancel()
		<-c.Done()
		return executorSuccess()
	}), executorProjection(executorSafeProjection))
	_, err := e.Execute(c, f.input)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, "running", f.operation(t).Status)
	var audit model.ExptLifecycleHookAttempt
	require.NoError(t, f.sql.Where("operation_id=?", f.input.OperationID).First(&audit).Error)
	require.Nil(t, audit.ResultCategory)
}

func TestHookExecutorAmbiguousCompletionWindowNeverSucceeds(t *testing.T) {
	f := newExecutorFixture(t, entity.HookPhaseBefore, 1)
	original := f.repo
	f.repo = &executorRepoProbe{IHookRepo: original, claim: func(c context.Context, in entity.HookClaimAttemptInput) (entity.HookAttemptStoreResult, error) {
		out, err := original.ClaimAttempt(c, in)
		if err == nil {
			out.Clock.LocalBefore = out.Clock.LocalBefore.Add(-800 * time.Millisecond)
		}
		return out, err
	}}
	e := f.executor(t, executorTransport(func(context.Context, entity.HookTransportInput) entity.HookTransportResult {
		time.Sleep(300 * time.Millisecond)
		return executorSuccess()
	}), executorProjection(executorSafeProjection))
	out, err := e.Execute(context.Background(), f.input)
	require.NoError(t, err)
	require.True(t, out.RecoveryPending)
	require.Equal(t, "running", f.operation(t).Status)
}

func TestHookExecutorCompletionConflictIsBounded(t *testing.T) {
	f := newExecutorFixture(t, entity.HookPhaseBefore, 10)
	original := f.repo
	var times []time.Time
	f.repo = &executorRepoProbe{IHookRepo: original, complete: func(c context.Context, in entity.HookCompleteAttemptInput) (entity.HookAttemptStoreResult, error) {
		times = append(times, in.CompletedAt)
		_, err := original.RenewAttempt(c, in.HookRenewAttemptInput)
		if err != nil {
			return entity.HookAttemptStoreResult{}, err
		}
		return original.CompleteAttempt(c, in)
	}}
	e := f.executor(t, executorTransport(func(context.Context, entity.HookTransportInput) entity.HookTransportResult { return executorSuccess() }), executorProjection(executorSafeProjection))
	_, err := e.Execute(context.Background(), f.input)
	require.ErrorIs(t, err, service.ErrHookExecutionUnavailable)
	require.Len(t, times, 3)
	require.Equal(t, times[0], times[1])
	require.Equal(t, times[0], times[2])
	require.Equal(t, "running", f.operation(t).Status)
	require.Equal(t, int32(1), f.operation(t).Attempt)
}

func TestHookExecutorCancelDuringCompletionCannotReopen(t *testing.T) {
	f := newExecutorFixture(t, entity.HookPhaseBefore, 10)
	original := f.repo
	f.repo = &executorRepoProbe{IHookRepo: original, complete: func(c context.Context, in entity.HookCompleteAttemptInput) (entity.HookAttemptStoreResult, error) {
		run, err := original.GetRun(c, in.Key)
		if err != nil {
			return entity.HookAttemptStoreResult{}, err
		}
		_, err = original.BeginFinalize(c, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: run.Version}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated, Reason: "cancel"}})
		if err != nil {
			return entity.HookAttemptStoreResult{}, err
		}
		return original.CompleteAttempt(c, in)
	}}
	e := f.executor(t, executorTransport(func(context.Context, entity.HookTransportInput) entity.HookTransportResult { return executorSuccess() }), executorProjection(executorSafeProjection))
	out, err := e.Execute(context.Background(), f.input)
	require.NoError(t, err)
	require.True(t, out.Effects.LateIgnored)
	require.Equal(t, entity.HookGateClosed, out.Run.State.Gate)
	var audit model.ExptLifecycleHookAttempt
	require.NoError(t, f.sql.Where("operation_id=?", f.input.OperationID).First(&audit).Error)
	require.True(t, audit.LateIgnored)
}

func TestHookExecutorLocalHTTPCodecAndErrorPrivacy(t *testing.T) {
	for _, response := range []string{`{"status":"succeeded","result":{"sensitive":"email=user@example.com"}}`, `{"status":"failed","error":{"message":"password=secret","code":"token=secret","retryable":false}}`} {
		t.Run(response[:20], func(t *testing.T) {
			f := newExecutorFixture(t, entity.HookPhaseBefore, 10)
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				var body map[string]any
				if json.NewDecoder(r.Body).Decode(&body) != nil {
					http.Error(w, "invalid", 400)
					return
				}
				parameters, ok := body["parameters"].(map[string]any)
				if !ok || parameters["business_key"] != "generic" {
					http.Error(w, "invalid", 400)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(response))
			}))
			defer server.Close()
			// Local-only adapter uses the production wire codecs; egress/signing are
			// independently covered by the unchanged real HTTPTransport TLS suite.
			tr := executorTransport(func(c context.Context, in entity.HookTransportInput) (out entity.HookTransportResult) {
				defer func() { out.LocalCompletedAt = time.Now() }()
				body, err := hookinfra.BuildRequest(in.Request)
				if err != nil {
					out.Outcome = entity.HookOutcome{Code: entity.HookSecurityError}
					return
				}
				c, cancel := context.WithTimeout(c, in.Remaining)
				defer cancel()
				req, err := http.NewRequestWithContext(c, http.MethodPost, server.URL, bytes.NewReader(body))
				if err != nil {
					out.Outcome = entity.HookOutcome{Code: entity.HookTransportError}
					return
				}
				resp, err := server.Client().Do(req)
				if err != nil {
					out.Outcome = entity.HookOutcome{Code: entity.HookTransportError}
					return
				}
				defer resp.Body.Close()
				out.Response, out.Outcome = hookinfra.DecodeResponse(resp.StatusCode, resp.Header.Get("Content-Type"), resp.Body)
				return
			})
			e := f.executor(t, tr, executorProjection(executorSafeProjection))
			out, err := e.Execute(context.Background(), f.input)
			require.NoError(t, err)
			require.NotNil(t, out.Run)
			require.Equal(t, int32(1), calls.Load())
			row := f.operation(t)
			saved := string(gptr.Indirect(row.ResultRedacted)) + gptr.Indirect(row.ErrorMessage) + gptr.Indirect(row.ErrorCode)
			require.NotContains(t, saved, "secret")
			require.NotContains(t, saved, "user@example.com")
			if strings.Contains(response, `"succeeded"`) {
				require.JSONEq(t, `{"public":"redacted"}`, string(gptr.Indirect(row.ResultRedacted)))
			} else {
				require.Equal(t, "HOOK_FAILED", gptr.Indirect(row.ErrorCode))
				require.Equal(t, "HOOK_FAILED", gptr.Indirect(row.ErrorMessage))
			}
		})
	}
}

func TestHookExecutorRealHTTPTransportMissingKeyFailsClosed(t *testing.T) {
	f := newExecutorFixture(t, entity.HookPhaseBefore, 10)
	e := f.executor(t, hookinfra.NewHTTPTransport(nil, nil), executorProjection(executorSafeProjection))
	out, err := e.Execute(context.Background(), f.input)
	require.NoError(t, err)
	require.True(t, out.Effects.BeginFinalize)
	require.Equal(t, "HOOK_SECURITY_ERROR", gptr.Indirect(f.operation(t).ErrorCode))
}

func TestHookExecutorProjectedResultBoundsAndTypedNil(t *testing.T) {
	for _, name := range []string{"empty key", "long value", "invalid UTF8", "many keys", "large json"} {
		t.Run(name, func(t *testing.T) {
			f := newExecutorFixture(t, entity.HookPhaseBefore, 10)
			projection := executorProjection(func(context.Context, entity.HookRunKey, entity.HookPhase, *spi.InvokeExperimentHookResponse) (map[string]string, error) {
				switch name {
				case "empty key":
					return map[string]string{"": "v"}, nil
				case "long value":
					return map[string]string{"k": strings.Repeat("v", 8193)}, nil
				case "invalid UTF8":
					return map[string]string{"k": string([]byte{255})}, nil
				case "large json":
					return map[string]string{"a": strings.Repeat("a", 8192), "b": strings.Repeat("b", 8192), "c": strings.Repeat("c", 8192), "d": strings.Repeat("d", 8192)}, nil
				}
				m := map[string]string{}
				for i := 0; i < 129; i++ {
					m[strconv.Itoa(i)] = "v"
				}
				return m, nil
			})
			e := f.executor(t, executorTransport(func(context.Context, entity.HookTransportInput) entity.HookTransportResult { return executorSuccess() }), projection)
			_, err := e.Execute(context.Background(), f.input)
			require.NoError(t, err)
			require.Equal(t, "failed", f.operation(t).Status)
			require.Empty(t, gptr.Indirect(f.operation(t).ResultRedacted))
		})
	}
	f := newExecutorFixture(t, entity.HookPhaseBefore, 10)
	var missing executorProjection
	_, err := service.NewHookAttemptExecutor(f.repo, f.codec, f.codec, executorTransport(func(context.Context, entity.HookTransportInput) entity.HookTransportResult { return executorSuccess() }), missing)
	require.Error(t, err)
	require.Zero(t, f.operation(t).Attempt)
}

func TestHookExecutorAbsoluteAttemptDeadlineCancelsTransport(t *testing.T) {
	f := newExecutorFixture(t, entity.HookPhaseBefore, 1)
	e := f.executor(t, executorTransport(func(ctx context.Context, _ entity.HookTransportInput) entity.HookTransportResult {
		<-ctx.Done()
		return entity.HookTransportResult{Outcome: entity.HookOutcome{Code: entity.HookTimeoutUncertain}, LocalCompletedAt: time.Now()}
	}), executorProjection(executorSafeProjection))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := e.Execute(ctx, f.input)
	require.NoError(t, err)
	require.NoError(t, ctx.Err())
	require.True(t, out.RecoveryPending)
	require.Equal(t, "running", f.operation(t).Status)
}

func TestHookExecutorDelayedProjectionPreservesHTTPCompletion(t *testing.T) {
	for _, fails := range []bool{false, true} {
		t.Run(fmt.Sprint(fails), func(t *testing.T) {
			f := newExecutorFixture(t, entity.HookPhaseBefore, 1)
			e := f.executor(t, executorTransport(func(context.Context, entity.HookTransportInput) entity.HookTransportResult { return executorSuccess() }), executorProjection(func(context.Context, entity.HookRunKey, entity.HookPhase, *spi.InvokeExperimentHookResponse) (map[string]string, error) {
				time.Sleep(1100 * time.Millisecond)
				if fails {
					return nil, errors.New("secret")
				}
				return map[string]string{"public": "redacted"}, nil
			}))
			out, err := e.Execute(context.Background(), f.input)
			require.NoError(t, err)
			if fails {
				require.True(t, out.RecoveryPending)
				require.Equal(t, "running", f.operation(t).Status)
			} else {
				require.Equal(t, "succeeded", f.operation(t).Status)
				require.True(t, out.CompletionWindow.Latest.Before(*f.operation(t).AttemptDeadline))
			}
		})
	}
}

func TestHookExecutorConcurrentWakeHasOneDelivery(t *testing.T) {
	f := newExecutorFixture(t, entity.HookPhaseBefore, 10)
	var calls atomic.Int32
	e := f.executor(t, executorTransport(func(context.Context, entity.HookTransportInput) entity.HookTransportResult {
		calls.Add(1)
		return executorSuccess()
	}), executorProjection(executorSafeProjection))
	start := make(chan struct{})
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		in := f.input
		in.AttemptID = executorIDs.Add(1)
		go func() { <-start; _, err := e.Execute(context.Background(), in); errs <- err }()
	}
	close(start)
	var results []error
	for i := 0; i < 4; i++ {
		results = append(results, <-errs)
	}
	for _, err := range results {
		require.NoError(t, err)
	}
	require.Equal(t, int32(1), calls.Load())
	require.Equal(t, int32(1), f.operation(t).Attempt)
	var count int64
	require.NoError(t, f.sql.Model(&model.ExptLifecycleHookAttempt{}).Where("operation_id=?", f.input.OperationID).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

type executorBadDecoder struct{ hookcomponent.StorageCodec }

func (d executorBadDecoder) DecodePhase(context.Context, entity.HookRunKey, string, entity.HookProtectedSnapshot, entity.HookPhase) (*entity.HookConfig, string, error) {
	return nil, "", errors.New("user@example.com secret")
}

type executorBadBuilder struct{}

func (executorBadBuilder) BuildClaimedRequest(context.Context, string, *entity.HookStoredRun, *entity.HookAttemptClaim) (*spi.InvokeExperimentHookRequest, string, error) {
	return nil, "", errors.New("user@example.com secret")
}

func TestHookExecutorDependencyFailuresNeverExposeRawErrors(t *testing.T) {
	for _, stage := range []string{"decode", "build"} {
		t.Run(stage, func(t *testing.T) {
			f := newExecutorFixture(t, entity.HookPhaseBefore, 10)
			var calls atomic.Int32
			var codec hookcomponent.StorageCodec = f.codec
			var builder hookcomponent.RequestBuilder = f.codec
			if stage == "decode" {
				codec = executorBadDecoder{f.codec}
			} else {
				builder = executorBadBuilder{}
			}
			e, err := service.NewHookAttemptExecutor(f.repo, codec, builder, executorTransport(func(context.Context, entity.HookTransportInput) entity.HookTransportResult {
				calls.Add(1)
				return executorSuccess()
			}), executorProjection(executorSafeProjection))
			require.NoError(t, err)
			_, err = e.Execute(context.Background(), f.input)
			if stage == "decode" {
				require.ErrorIs(t, err, service.ErrHookExecutionUnavailable)
				require.Zero(t, f.operation(t).Attempt)
			} else {
				require.NoError(t, err)
				require.Equal(t, "HOOK_SECURITY_ERROR", gptr.Indirect(f.operation(t).ErrorCode))
				require.NotContains(t, gptr.Indirect(f.operation(t).ErrorMessage), "secret")
			}
			require.Zero(t, calls.Load())
		})
	}
}

func TestHookExecutorRenewalRevocationStopsLocalHTTP(t *testing.T) {
	f := newExecutorFixture(t, entity.HookPhaseBefore, 60)
	original := f.repo
	f.repo = &executorRepoProbe{IHookRepo: original, renew: func(c context.Context, in entity.HookRenewAttemptInput) (entity.HookAttemptStoreResult, error) {
		run, err := original.GetRun(c, in.Key)
		if err != nil {
			return entity.HookAttemptStoreResult{}, err
		}
		_, err = original.BeginFinalize(c, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: run.Version}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated, Reason: "cancel"}})
		if err != nil {
			return entity.HookAttemptStoreResult{}, err
		}
		return original.RenewAttempt(c, in)
	}}
	var cancelled atomic.Bool
	e := f.executor(t, executorTransport(func(c context.Context, _ entity.HookTransportInput) entity.HookTransportResult {
		<-c.Done()
		cancelled.Store(true)
		return executorSuccess()
	}), executorProjection(executorSafeProjection))
	c, cancel := context.WithTimeout(context.Background(), 13*time.Second)
	defer cancel()
	_, err := e.Execute(c, f.input)
	require.ErrorIs(t, err, service.ErrHookExecutionLeaseLost)
	require.NoError(t, c.Err())
	require.True(t, cancelled.Load())
	run, err := original.GetRun(context.Background(), f.input.Key)
	require.NoError(t, err)
	require.Equal(t, entity.HookGateClosed, run.State.Gate)
}

func TestHookExecutorDBCompletionWitnessRejectsClockStep(t *testing.T) {
	for _, delta := range []time.Duration{2 * time.Second, -50 * time.Millisecond} {
		t.Run(delta.String(), func(t *testing.T) {
			f := newExecutorFixture(t, entity.HookPhaseBefore, 1)
			original := f.repo
			f.repo = &executorRepoProbe{IHookRepo: original, read: func(ctx context.Context, in entity.HookAttemptScope) (entity.HookAttemptStoreResult, error) {
				out, err := original.(hookcomponent.AttemptReader).ReadAttempt(ctx, in)
				if err == nil {
					out.Clock.DBTime = out.Clock.DBTime.Add(delta)
				}
				return out, err
			}}
			e := f.executor(t, executorTransport(func(ctx context.Context, _ entity.HookTransportInput) entity.HookTransportResult {
				require.NoError(t, executorClockTestWait(ctx, 100*time.Millisecond))
				// Model the discontinuity in the DB calibration, never by fabricating
				// an earlier LocalCompletedAt on an otherwise steady clock.
				return executorSuccess()
			}), executorProjection(executorSafeProjection))
			out, err := e.Execute(context.Background(), f.input)
			require.NoError(t, err)
			require.True(t, out.RecoveryPending)
			require.Equal(t, "running", f.operation(t).Status)
			require.Equal(t, int32(1), f.operation(t).Attempt)
		})
	}
}

func TestHookExecutorLateResponseUsesActualCompletionTime(t *testing.T) {
	f := newExecutorFixture(t, entity.HookPhaseBefore, 1)
	e := f.executor(t, executorTransport(func(context.Context, entity.HookTransportInput) entity.HookTransportResult {
		time.Sleep(1100 * time.Millisecond)
		return executorSuccess()
	}), executorProjection(executorSafeProjection))
	out, err := e.Execute(context.Background(), f.input)
	require.NoError(t, err)
	require.True(t, out.RecoveryPending)
	require.Equal(t, "running", f.operation(t).Status)
	require.Equal(t, int32(1), f.operation(t).Attempt)
	var audit model.ExptLifecycleHookAttempt
	require.NoError(t, f.sql.Where("operation_id=?", f.input.OperationID).First(&audit).Error)
	require.Nil(t, audit.FinishedAt)
	require.Nil(t, audit.ResultCategory)
}

func TestHookExecutorAuditIdentifiesConservativeCompletionBound(t *testing.T) {
	f := newExecutorFixture(t, entity.HookPhaseBefore, 10)
	e := f.executor(t, executorTransport(func(context.Context, entity.HookTransportInput) entity.HookTransportResult { return executorSuccess() }), executorProjection(executorSafeProjection))
	out, err := e.Execute(context.Background(), f.input)
	require.NoError(t, err)
	var audit model.ExptLifecycleHookAttempt
	require.NoError(t, f.sql.Where("operation_id=?", f.input.OperationID).First(&audit).Error)
	require.NotNil(t, audit.ErrorRedacted, "audit must distinguish a conservative bound from an exact HTTP timestamp")
	var metadata struct {
		CompletionTime struct {
			Convention       string `json:"convention"`
			Earliest, Latest time.Time
		} `json:"completion_time"`
	}
	require.NoError(t, json.Unmarshal(gptr.Indirect(audit.ErrorRedacted), &metadata))
	require.Equal(t, "conservative_upper_bound", metadata.CompletionTime.Convention)
	require.Equal(t, out.CompletionWindow.Earliest, metadata.CompletionTime.Earliest)
	require.Equal(t, out.CompletionWindow.Latest, metadata.CompletionTime.Latest)
	require.True(t, audit.FinishedAt.Equal(metadata.CompletionTime.Latest))
}

func TestHookExecutorTimelyHTTPWithDelayedDB(t *testing.T) {
	for _, name := range []string{"witness_queue", "witness_mysql_lock", "commit_queue"} {
		t.Run(name, func(t *testing.T) {
			f := newExecutorFixture(t, entity.HookPhaseBefore, 1)
			original := f.repo
			var claim *entity.HookAttemptClaim
			var completeCalls, recoverReadCalls, sends int
			var submitted entity.HookCompleteAttemptInput
			var witnessQueueStarted, witnessSampled time.Time
			probe := &executorRepoProbe{IHookRepo: original}
			probe.claim = func(ctx context.Context, in entity.HookClaimAttemptInput) (entity.HookAttemptStoreResult, error) {
				out, err := original.ClaimAttempt(ctx, in)
				claim = out.Claim
				return out, err
			}
			probe.read = func(ctx context.Context, in entity.HookAttemptScope) (entity.HookAttemptStoreResult, error) {
				recoverReadCalls++
				witnessQueueStarted = time.Now()
				if name == "witness_queue" {
					if err := executorClockTestWait(ctx, 1100*time.Millisecond); err != nil {
						return entity.HookAttemptStoreResult{}, err
					}
				}
				out, err := original.(hookcomponent.AttemptReader).ReadAttempt(ctx, in)
				if out.Clock != nil {
					witnessSampled = out.Clock.LocalBefore
				}
				return out, err
			}
			probe.complete = func(ctx context.Context, in entity.HookCompleteAttemptInput) (entity.HookAttemptStoreResult, error) {
				completeCalls++
				submitted = in
				if name == "commit_queue" {
					if err := executorClockTestWait(ctx, 1100*time.Millisecond); err != nil {
						return entity.HookAttemptStoreResult{}, err
					}
				}
				return original.CompleteAttempt(ctx, in)
			}
			f.repo = probe
			var release func()
			defer func() {
				if release != nil {
					release()
				}
			}()
			var httpElapsed time.Duration
			e := f.executor(t, executorTransport(func(ctx context.Context, _ entity.HookTransportInput) entity.HookTransportResult {
				sends++
				started := time.Now()
				require.NoError(t, executorClockTestWait(ctx, 20*time.Millisecond))
				if name == "witness_mysql_lock" {
					release = executorClockHoldRunLock(t, f, 1100*time.Millisecond)
				}
				httpElapsed = time.Since(started)
				return executorSuccess()
			}), executorProjection(executorSafeProjection))
			out, err := e.Execute(context.Background(), f.input)
			require.NoError(t, err)
			require.NotNil(t, claim)
			require.Equal(t, time.Second, claim.AttemptDeadline.Sub(claim.StartedAt))
			require.Equal(t, 6*time.Second, claim.LeaseUntil.Sub(claim.StartedAt))
			require.Less(t, httpElapsed, 500*time.Millisecond)
			if name != "commit_queue" {
				require.GreaterOrEqual(t, witnessSampled.Sub(witnessQueueStarted), time.Second)
			}
			t.Logf("HTTP=%s witness_queue=%s sends=%d complete_calls=%d status=%s", httpElapsed, witnessSampled.Sub(witnessQueueStarted), sends, completeCalls, f.operation(t).Status)
			require.Equal(t, "succeeded", f.operation(t).Status, "SDD3.6: completed HTTP is independent of subsequent DB queue")
			require.False(t, out.RecoveryPending)
			require.Equal(t, 1, sends)
			require.Equal(t, 1, completeCalls)
			require.Equal(t, 1, recoverReadCalls)
			require.True(t, submitted.CompletedAt.Before(claim.AttemptDeadline))
			require.Less(t, submitted.CompletedAt.Sub(claim.StartedAt), 500*time.Millisecond)
			require.Equal(t, claim.HookAttemptIdentity, submitted.HookAttemptIdentity)
			var audit model.ExptLifecycleHookAttempt
			require.NoError(t, f.sql.Where("operation_id=?", f.input.OperationID).First(&audit).Error)
			require.True(t, audit.FinishedAt.Equal(submitted.CompletedAt))
			require.Equal(t, int32(1), audit.Attempt)
		})
	}
}

func executorClockTestWait(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func executorClockHoldRunLock(t *testing.T, f *executorFixture, d time.Duration) func() {
	t.Helper()
	tx := f.sql.Begin()
	require.NoError(t, tx.Error)
	if err := tx.Exec("SELECT id FROM experiment WHERE id=? AND space_id=? FOR UPDATE", f.input.Key.ExperimentID, f.input.Key.WorkspaceID).Error; err != nil {
		tx.Rollback()
		require.NoError(t, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _ = executorClockTestWait(ctx, d); done <- tx.Rollback().Error }()
	return func() { cancel(); require.NoError(t, <-done) }
}
