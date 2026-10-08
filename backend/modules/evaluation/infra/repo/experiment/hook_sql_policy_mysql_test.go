// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

// A successful MySQL parse is not a platform precheck. Check the two documented
// blocked rules on the columns MySQL actually creates in both deployment paths.
func TestHookSQLPolicyColumnMetadata(t *testing.T) {
	f := newHookTxFixture(t)
	for _, path := range []string{
		"../../../../../../release/deployment/docker-compose/bootstrap/mysql-init/init-sql/",
		"../../../../../../release/deployment/helm-chart/charts/app/bootstrap/init/mysql/init-sql/",
	} {
		for _, table := range []string{"expt_lifecycle_run", "expt_lifecycle_hook_run"} {
			t.Run(path+table, func(t *testing.T) {
				raw, err := os.ReadFile(path + table + ".sql")
				require.NoError(t, err)
				require.NoError(t, f.sql.Connection(func(conn *gorm.DB) error {
					name := fmt.Sprintf("hook_policy_%d", f.expt)
					ddl := strings.Replace(string(raw), "CREATE TABLE IF NOT EXISTS", "CREATE TEMPORARY TABLE", 1)
					ddl = strings.Replace(ddl, "`"+table+"`", "`"+name+"`", 1)
					require.NoError(t, conn.Exec(ddl).Error)
					defer conn.Exec("DROP TEMPORARY TABLE " + name)
					var columns []struct {
						Field, Type, Null string
						Collation         *string
						Default           *string
					}
					require.NoError(t, conn.Raw("SHOW FULL COLUMNS FROM "+name).Scan(&columns).Error)
					checked := 0
					for _, col := range columns {
						switch col.Field {
						case "snapshot_hash", "plan_hash", "request_hash":
							checked++
							require.Equal(t, "varchar(64)", col.Type, col.Field)
							require.NotNil(t, col.Collation)
							require.Equal(t, "ascii_bin", *col.Collation, col.Field)
						case "snapshot_cipher":
							checked++
							require.Equal(t, "mediumblob", col.Type)
							require.Equal(t, "YES", col.Null, "platform disallows BLOB NOT NULL")
							require.Nil(t, col.Default)
						}
					}
					if table == "expt_lifecycle_run" {
						require.Equal(t, 3, checked)
					} else {
						require.Equal(t, 1, checked)
					}
					return nil
				}))
			})
		}
	}
}

func TestHookSQLPolicySnapshotWriteInvariant(t *testing.T) {
	f := newHookTxFixture(t)
	ctx := context.Background()
	for _, cipher := range [][]byte{nil, {}} {
		in := f.input(true, 0)
		in.Snapshot.Cipher = cipher
		out, err := f.repo.CreateRunWithHooks(ctx, in)
		require.Error(t, err)
		require.Nil(t, out.Run)
		require.Zero(t, f.latest(t))
		var count int64
		require.NoError(t, hookRunScope(f.sql.Model(&model.ExptLifecycleRun{}), in.Key).Count(&count).Error)
		require.Zero(t, count)
	}
	in := f.input(true, 0)
	in.Snapshot.Cipher = bytes.Repeat([]byte{0, 255, 17}, 24000)
	created, err := f.repo.CreateRunWithHooks(ctx, in)
	require.NoError(t, err)
	require.True(t, created.Changed)
	var stored model.ExptLifecycleRun
	require.NoError(t, hookRunScope(f.sql, in.Key).First(&stored).Error)
	require.Equal(t, in.Snapshot.Cipher, stored.SnapshotCipher)
	require.Equal(t, strings.Repeat("a", 64), stored.SnapshotHash)
	read, err := f.repo.GetRun(ctx, in.Key)
	require.NoError(t, err)
	require.Equal(t, in.Snapshot.Cipher, read.Snapshot.Cipher)
}

func TestHookSQLPolicyHashStorageSemantics(t *testing.T) {
	f := newHookTxFixture(t)
	in := f.input(true, 0)
	_, err := f.repo.CreateRunWithHooks(context.Background(), in)
	require.NoError(t, err)
	for _, tc := range []struct{ table, column string }{
		{"expt_lifecycle_run", "snapshot_hash"},
		{"expt_lifecycle_run", "plan_hash"},
		{"expt_lifecycle_hook_run", "request_hash"},
	} {
		t.Run(tc.column, func(t *testing.T) {
			rows := func() *gorm.DB { return hookRunScope(f.sql.Table(tc.table), in.Key) }
			require.NoError(t, rows().UpdateColumn(tc.column, strings.Repeat("a", 64)).Error)
			var values []string
			require.NoError(t, rows().Pluck(tc.column, &values).Error)
			require.NotEmpty(t, values)
			for _, value := range values {
				require.Equal(t, strings.Repeat("a", 64), value)
			}
			var count int64
			require.NoError(t, rows().Where(tc.column+" = ?", strings.Repeat("A", 64)).Count(&count).Error)
			require.Zero(t, count, "ASCII hashes remain case sensitive")
			require.Error(t, rows().UpdateColumn(tc.column, strings.Repeat("a", 65)).Error)
			require.Error(t, rows().UpdateColumn(tc.column, "中").Error)
		})
	}
}

func TestHookSQLPolicyNullSnapshotFailsClosed(t *testing.T) {
	for _, phase := range []string{"before", "after"} {
		t.Run(phase, func(t *testing.T) {
			f := newHookTxFixture(t)
			ctx := context.Background()
			claim := hookAttemptInput(t, f, phase == "before")
			prior := hookAttemptRow(t, f, claim.OperationID)
			codec, protected := hookSQLPolicySnapshot(t, claim)
			claim.SnapshotHash = protected.Hash
			require.NoError(t, hookRunScope(f.sql.Model(&model.ExptLifecycleRun{}), claim.Key).Updates(map[string]any{"snapshot_cipher": protected.Cipher, "snapshot_key_id": protected.KeyID, "snapshot_hash": protected.Hash}).Error)
			valid, err := f.repo.GetRun(ctx, claim.Key)
			require.NoError(t, err)
			config, hash, err := codec.DecodePhase(ctx, claim.Key, valid.Snapshot.ExecutionScope, valid.Snapshot, claim.Phase)
			require.NoError(t, err)
			require.True(t, *config.Enabled)
			require.Equal(t, protected.Hash, hash)
			require.NoError(t, hookRunScope(f.sql.Model(&model.ExptLifecycleRun{}), claim.Key).UpdateColumn("snapshot_cipher", nil).Error)
			var row model.ExptLifecycleRun
			require.NoError(t, hookRunScope(f.sql, claim.Key).First(&row).Error)
			require.Nil(t, row.SnapshotCipher, "SQL NULL must scan into the unchanged generated []byte field")
			read, err := f.repo.GetRun(ctx, claim.Key)
			require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
			require.Nil(t, read)
			out, err := f.repo.ClaimAttempt(ctx, claim)
			require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
			require.Nil(t, out.Claim)
			require.False(t, out.Changed)
			require.Equal(t, prior, hookAttemptRow(t, f, claim.OperationID))
			var count int64
			require.NoError(t, f.sql.Model(&model.ExptLifecycleHookAttempt{}).Where("operation_id=?", claim.OperationID).Count(&count).Error)
			require.Zero(t, count)
			protected = entity.HookProtectedSnapshot{Cipher: row.SnapshotCipher, KeyID: row.SnapshotKeyID, Hash: row.SnapshotHash, ExecutionScope: row.ExecutionScope}
			decoded, err := codec.DecodeSnapshot(ctx, claim.Key, row.ExecutionScope, protected)
			require.Error(t, err)
			require.Nil(t, decoded)
			config, hash, err = codec.DecodePhase(ctx, claim.Key, row.ExecutionScope, protected, claim.Phase)
			require.Error(t, err)
			require.Nil(t, config)
			require.Empty(t, hash)
		})
	}
}

// Local authenticated encryption keeps the codec real without contacting DKMS.
type hookSQLPolicyProtector struct{ cipher.AEAD }

func (p hookSQLPolicyProtector) Protect(_ context.Context, _ string, plain []byte) ([]byte, error) {
	nonce := make([]byte, p.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return p.Seal(nonce, nonce, plain, nil), nil
}

func (p hookSQLPolicyProtector) Unprotect(_ context.Context, _ string, encrypted []byte) ([]byte, error) {
	if len(encrypted) < p.NonceSize() {
		return nil, errors.New("invalid local test ciphertext")
	}
	return p.Open(nil, encrypted[:p.NonceSize()], encrypted[p.NonceSize():], nil)
}

func hookSQLPolicySnapshot(t *testing.T, claim entity.HookClaimAttemptInput) (*hookinfra.StorageCodec, entity.HookProtectedSnapshot) {
	t.Helper()
	block, err := aes.NewCipher(make([]byte, 32))
	require.NoError(t, err)
	aead, err := cipher.NewGCM(block)
	require.NoError(t, err)
	codec := hookinfra.NewStorageCodec(hookSQLPolicyProtector{aead})
	conf := &entity.LifecycleHookConf{After: claim.Config}
	if claim.Phase == entity.HookPhaseBefore {
		conf.Before = claim.Config
	}
	snapshot, err := entity.NewHookRunSnapshot(entity.HookRunSnapshotInput{
		Key: claim.Key, ExecutionScope: claim.ExecutionScope, CreatedAt: time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC), Config: conf,
		Context: &spi.HookRunContext{
			WorkspaceID: gptr.Of(fmt.Sprint(claim.Key.WorkspaceID)), ExperimentID: gptr.Of(fmt.Sprint(claim.Key.ExperimentID)), RunID: gptr.Of(fmt.Sprint(claim.Key.RunID)), RunMode: gptr.Of("append"),
			Initiator:  &spi.HookInitiator{UserID: gptr.Of("user"), IdentityType: gptr.Of("fornax_user")},
			Experiment: &spi.HookExperimentRef{Name: gptr.Of("SQL policy test"), Type: gptr.Of("online")}, EvalSets: []*spi.HookEvalSetRef{},
		},
	})
	require.NoError(t, err)
	protected, err := codec.EncodeSnapshot(context.Background(), "local-test-key", snapshot)
	require.NoError(t, err)
	return codec, protected
}
