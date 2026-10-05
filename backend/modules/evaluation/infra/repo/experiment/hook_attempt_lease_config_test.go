// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

func TestHookAttemptConfiguredLeaseMySQL(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		lease, timeout, want int32
	}{
		{"default", 0, 180, 30}, {"twenty", 20, 180, 20}, {"sixty", 60, 180, 60}, {"deadline_cap", 60, 10, 15}, {"largest_positive", 2147483647, 180, 185},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newHookTxFixture(t)
			in := hookAttemptInput(t, f, true)
			in.Config.TimeoutSeconds = gptr.Of(tc.timeout)
			if tc.lease != 0 {
				in.LeaseSeconds = gptr.Of(tc.lease)
			}
			out, err := f.repo.ClaimAttempt(context.Background(), in)
			require.NoError(t, err)
			require.NotNil(t, out.Claim)
			require.NotNil(t, out.Clock)
			require.Equal(t, time.Duration(tc.want)*time.Second, out.Claim.LeaseUntil.Sub(out.Claim.StartedAt), "Claim must use the requested lease and existing deadline cap")
			renew := entity.HookRenewAttemptInput{HookAttemptScope: in.HookAttemptScope, HookAttemptIdentity: out.Claim.HookAttemptIdentity}
			renew.ExpectedVersion = out.Claim.Version
			if tc.lease != 0 {
				renew.LeaseSeconds = gptr.Of(tc.lease)
			}
			got, err := f.repo.RenewAttempt(context.Background(), renew)
			require.NoError(t, err)
			require.NotNil(t, got.Claim)
			require.NotNil(t, got.Clock)
			want := 30 * time.Second
			if tc.lease != 0 {
				want = time.Duration(tc.lease) * time.Second
			}
			expected := got.Clock.DBTime.Add(want)
			if limit := out.Claim.AttemptDeadline.Add(5 * time.Second); expected.After(limit) {
				expected = limit
			}
			require.True(t, expected.Equal(got.Claim.LeaseUntil))
			require.Equal(t, out.Claim.AttemptDeadline, got.Claim.AttemptDeadline)
			require.Equal(t, out.Claim.HookAttemptIdentity, got.Claim.HookAttemptIdentity)
			require.NoError(t, f.sql.Model(&model.ExptLifecycleHookRun{}).Where("operation_id=?", in.OperationID).UpdateColumn("lease_until", time.Now().UTC().Add(-time.Second)).Error)
			renew.ExpectedVersion = got.Claim.Version
			_, err = f.repo.RenewAttempt(context.Background(), renew)
			require.ErrorIs(t, err, entity.ErrHookStoreConflict, "expired leases cannot be revived")
		})
	}
}

func TestHookAttemptRejectsInvalidLeaseBeforeSQL(t *testing.T) {
	for _, seconds := range []int32{0, -1} {
		f := newHookTxFixture(t)
		in := hookAttemptInput(t, f, true)
		in.LeaseSeconds = gptr.Of(seconds)
		_, err := f.repo.ClaimAttempt(context.Background(), in)
		require.Error(t, err)
		require.Zero(t, hookAttemptRow(t, f, in.OperationID).Attempt)
		in.LeaseSeconds = nil
		out, err := f.repo.ClaimAttempt(context.Background(), in)
		require.NoError(t, err)
		renew := entity.HookRenewAttemptInput{HookAttemptScope: in.HookAttemptScope, HookAttemptIdentity: out.Claim.HookAttemptIdentity, LeaseSeconds: gptr.Of(seconds)}
		renew.ExpectedVersion = out.Claim.Version
		_, err = f.repo.RenewAttempt(context.Background(), renew)
		require.Error(t, err)
		row := hookAttemptRow(t, f, in.OperationID)
		require.Equal(t, out.Claim.Version, row.Version)
		require.True(t, out.Claim.LeaseUntil.Equal(gptr.Indirect(row.LeaseUntil)))
	}
}
