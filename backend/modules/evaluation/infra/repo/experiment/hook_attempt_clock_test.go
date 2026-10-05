// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type hookClockDelayedProvider struct{ db.Provider }

func (p hookClockDelayedProvider) Transaction(ctx context.Context, fn func(*gorm.DB) error, opts ...db.Option) error {
	err := p.Provider.Transaction(ctx, fn, opts...)
	time.Sleep(80 * time.Millisecond)
	return err
}

func TestHookAttemptClockIncludesTransactionReturnDelay(t *testing.T) {
	f := newHookTxFixture(t)
	in := hookAttemptInput(t, f, true)
	r := NewHookRunRepo(hookClockDelayedProvider{f.p})
	claimed, err := r.ClaimAttempt(context.Background(), in)
	require.NoError(t, err)
	require.NotNil(t, claimed.Clock, "claim must return its actual DB query bracket")
	require.Equal(t, claimed.Claim.StartedAt, claimed.Clock.DBTime)
	window, err := claimed.Clock.Window(time.Now())
	require.NoError(t, err)
	require.True(t, window.Earliest.After(claimed.Claim.StartedAt.Add(70*time.Millisecond)))
	renewed, err := r.RenewAttempt(context.Background(), entity.HookRenewAttemptInput{HookAttemptScope: entity.HookAttemptScope{Key: in.Key, OperationID: in.OperationID, Phase: in.Phase, ExpectedVersion: claimed.Claim.Version, ExecutionScope: in.ExecutionScope, SnapshotHash: in.SnapshotHash}, HookAttemptIdentity: claimed.Claim.HookAttemptIdentity})
	require.NoError(t, err)
	require.NotNil(t, renewed.Clock)
	require.True(t, renewed.Clock.LocalBefore.After(claimed.Clock.LocalAfter))
	require.Equal(t, renewed.Claim.AttemptDeadline.Add(5*time.Second), renewed.Claim.LeaseUntil)
}

func TestHookAttemptReadReturnsExactOwnerAndToken(t *testing.T) {
	f := newHookTxFixture(t)
	in := hookAttemptInput(t, f, true)
	claimed, err := f.repo.ClaimAttempt(context.Background(), in)
	require.NoError(t, err)
	reader := f.repo.(interface {
		ReadAttempt(context.Context, entity.HookAttemptScope) (entity.HookAttemptStoreResult, error)
	})
	out, err := reader.ReadAttempt(context.Background(), in.HookAttemptScope)
	require.NoError(t, err)
	require.Equal(t, claimed.Claim, out.Claim)
	require.NotNil(t, out.Clock)
	in.ExecutionScope = "other"
	_, err = reader.ReadAttempt(context.Background(), in.HookAttemptScope)
	require.Error(t, err)
}

func TestHookAttemptReadClockSurvivesCancellation(t *testing.T) {
	f := newHookTxFixture(t)
	in := hookAttemptInput(t, f, true)
	_, err := f.repo.ClaimAttempt(context.Background(), in)
	require.NoError(t, err)
	run, err := f.repo.GetRun(context.Background(), in.Key)
	require.NoError(t, err)
	_, err = f.repo.BeginFinalize(context.Background(), entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: run.Version}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated, Reason: "cancel"}})
	require.NoError(t, err)
	out, err := f.repo.(interface {
		ReadAttempt(context.Context, entity.HookAttemptScope) (entity.HookAttemptStoreResult, error)
	}).ReadAttempt(context.Background(), in.HookAttemptScope)
	require.NoError(t, err)
	require.Nil(t, out.Claim)
	require.NotNil(t, out.Clock)
	require.Equal(t, entity.HookGateClosed, out.Run.State.Gate)
}
