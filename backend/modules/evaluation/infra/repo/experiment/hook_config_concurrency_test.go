// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

type configBoundaryCodec struct {
	hookcomponent.StorageCodec
	encode func() error
	decode func() error
}

func (c configBoundaryCodec) EncodeConfig(ctx context.Context, key string, o hookcomponent.ConfigOwner, conf *entity.LifecycleHookConf) ([]byte, error) {
	if c.encode != nil {
		if err := c.encode(); err != nil {
			return nil, err
		}
	}
	return c.StorageCodec.EncodeConfig(ctx, key, o, conf)
}
func (c configBoundaryCodec) DecodeConfig(ctx context.Context, o hookcomponent.ConfigOwner, b []byte) (*entity.LifecycleHookConf, error) {
	if c.decode != nil {
		if err := c.decode(); err != nil {
			return nil, err
		}
	}
	return c.StorageCodec.DecodeConfig(ctx, o, b)
}
func configWait(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("configuration operation did not reach boundary")
	}
}

func TestHookConfigConcurrentCAS(t *testing.T) {
	for _, template := range []bool{false, true} {
		t.Run(fmt.Sprint(template), func(t *testing.T) {
			f := newHookTxFixture(t)
			o := configOwner(f)
			if template {
				o = configTemplate(t, f)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			codec := configTestCodec(t)
			r := NewHookConfigRepo(f.p, codec)
			_, err := r.UpdateConfig(ctx, o, entity.HookConfigUpdateInput{Config: configBefore(), KeyID: "test-key"})
			require.NoError(t, err)
			before, err := r.GetConfig(ctx, o)
			require.NoError(t, err)
			arrived := make(chan struct{}, 2)
			release := make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			r = NewHookConfigRepo(f.p, configBoundaryCodec{StorageCodec: codec, encode: func() error {
				arrived <- struct{}{}
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}})
			var changed [2]bool
			var errs [2]error
			var wg sync.WaitGroup
			for i := 0; i < 2; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					changed[i], errs[i] = r.UpdateConfig(ctx, o, entity.HookConfigUpdateInput{ExpectedRevision: before.Revision, KeyID: "test-key", Config: &entity.LifecycleHookConf{After: &entity.HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(fmt.Sprintf(`{"winner":%d}`, i))}}})
				}(i)
			}
			configWait(t, arrived)
			configWait(t, arrived)
			unblock()
			wg.Wait()
			winner := -1
			for i := range changed {
				if changed[i] {
					require.Equal(t, -1, winner)
					winner = i
					require.NoError(t, errs[i])
				} else {
					require.ErrorIs(t, errs[i], entity.ErrHookStoreConflict)
				}
			}
			require.NotEqual(t, -1, winner)
			after, err := NewHookConfigRepo(f.p, codec).GetConfig(ctx, o)
			require.NoError(t, err)
			if winner == 0 {
				require.JSONEq(t, `{"winner":0}`, *after.Config.After.ParametersJSON)
			} else {
				require.JSONEq(t, `{"winner":1}`, *after.Config.After.ParametersJSON)
			}
			require.Equal(t, "https://hook.example/before", *after.Config.Before.InvokeHTTPInfo.URL)
		})
	}
}

func TestHookConfigCryptoOutsideLockAndRunWins(t *testing.T) {
	for _, boundary := range []string{"decode", "encode"} {
		t.Run(boundary, func(t *testing.T) {
			f := newHookTxFixture(t)
			o := configOwner(f)
			codec := configTestCodec(t)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			r := NewHookConfigRepo(f.p, codec)
			_, err := r.UpdateConfig(ctx, o, entity.HookConfigUpdateInput{Config: configBefore(), KeyID: "test-key"})
			require.NoError(t, err)
			before, err := r.GetConfig(ctx, o)
			require.NoError(t, err)
			raw := configRaw(t, f, o)
			arrived := make(chan struct{})
			release := make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			pause := func() error {
				close(arrived)
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			wrapped := configBoundaryCodec{StorageCodec: codec}
			if boundary == "decode" {
				wrapped.decode = pause
			} else {
				wrapped.encode = pause
			}
			var updateErr error
			var changed bool
			done := make(chan struct{})
			go func() {
				defer close(done)
				changed, updateErr = NewHookConfigRepo(f.p, wrapped).UpdateConfig(ctx, o, entity.HookConfigUpdateInput{Config: &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(false)}}, ExpectedRevision: before.Revision, KeyID: "test-key"})
			}()
			configWait(t, arrived)
			// A second MySQL connection can acquire the physical experiment lock while crypto waits.
			err = f.p.Transaction(ctx, func(tx *gorm.DB) error {
				var row model.Experiment
				return tx.Select("id").Where("id=?", f.expt).Clauses(clause.Locking{Strength: "UPDATE", Options: "NOWAIT"}).Take(&row).Error
			})
			require.NoError(t, err)
			in := f.input(true, 0)
			in.ExpectedConfigRevision = before.Revision
			created, err := f.repo.CreateRunWithHooks(ctx, in)
			require.NoError(t, err)
			require.True(t, created.Changed)
			unblock()
			configWait(t, done)
			require.ErrorIs(t, updateErr, entity.ErrHookConfigImmutable)
			require.False(t, changed)
			require.Equal(t, raw, configRaw(t, f, o))
			require.Equal(t, in.Key.RunID, f.latest(t))
		})
	}
}

func TestHookConfigUpdateWinsAndRunReplay(t *testing.T) {
	f := newHookTxFixture(t)
	o := configOwner(f)
	ctx := context.Background()
	r := NewHookConfigRepo(f.p, configTestCodec(t))
	_, err := r.UpdateConfig(ctx, o, entity.HookConfigUpdateInput{Config: configBefore(), KeyID: "test-key"})
	require.NoError(t, err)
	before, err := r.GetConfig(ctx, o)
	require.NoError(t, err)
	in := f.input(true, 0)
	in.ExpectedConfigRevision = before.Revision
	_, err = r.UpdateConfig(ctx, o, entity.HookConfigUpdateInput{Config: &entity.LifecycleHookConf{After: &entity.HookConfig{Enabled: gptr.Of(false)}}, ExpectedRevision: before.Revision, KeyID: "test-key"})
	require.NoError(t, err)
	got, err := f.repo.CreateRunWithHooks(ctx, in)
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	require.Nil(t, got.Run)
	require.Zero(t, f.latest(t))
	for _, table := range []string{"expt_run_log", "expt_lifecycle_run", "expt_lifecycle_hook_run"} {
		var count int64
		require.NoError(t, f.sql.Table(table).Where("expt_id=?", f.expt).Count(&count).Error)
		require.Zero(t, count)
	}
	after, err := r.GetConfig(ctx, o)
	require.NoError(t, err)
	in.ExpectedConfigRevision = after.Revision
	got, err = f.repo.CreateRunWithHooks(ctx, in)
	require.NoError(t, err)
	require.True(t, got.Changed)
	in.ExpectedConfigRevision = before.Revision
	replay, err := f.repo.CreateRunWithHooks(ctx, in)
	require.NoError(t, err)
	require.False(t, replay.Changed)
	require.Equal(t, got.Run, replay.Run)
}

func TestHookConfigConcurrentUpdateAndCreateHaveOneWinner(t *testing.T) {
	for i := 0; i < 8; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			f := newHookTxFixture(t)
			o := configOwner(f)
			ctx := context.Background()
			r := NewHookConfigRepo(f.p, configTestCodec(t))
			start := make(chan struct{})
			var wg sync.WaitGroup
			wg.Add(2)
			var changed bool
			var updateErr, runErr error
			var created entity.HookStoreResult
			in := f.input(true, 0)
			go func() {
				defer wg.Done()
				<-start
				changed, updateErr = r.UpdateConfig(ctx, o, entity.HookConfigUpdateInput{Config: configBefore(), KeyID: "test-key"})
			}()
			go func() { defer wg.Done(); <-start; created, runErr = f.repo.CreateRunWithHooks(ctx, in) }()
			close(start)
			wg.Wait()
			require.NotEqual(t, changed, created.Changed)
			if changed {
				require.NoError(t, updateErr)
				require.ErrorIs(t, runErr, entity.ErrHookStoreConflict)
				require.Zero(t, f.latest(t))
				require.NotEmpty(t, configRaw(t, f, o))
			} else {
				require.NoError(t, runErr)
				require.True(t, errors.Is(updateErr, entity.ErrHookConfigImmutable) || errors.Is(updateErr, entity.ErrHookStoreConflict))
				require.Nil(t, configRaw(t, f, o))
				require.Equal(t, in.Key.RunID, f.latest(t))
			}
		})
	}
}

func TestHookConfigRechecksDeletionAndUnmarkedHistoryAfterCrypto(t *testing.T) {
	for _, change := range []string{"experiment-delete", "template-delete", "unmarked-history"} {
		t.Run(change, func(t *testing.T) {
			f := newHookTxFixture(t)
			o := configOwner(f)
			if change == "template-delete" {
				o = configTemplate(t, f)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			arrived, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			codec := configBoundaryCodec{StorageCodec: configTestCodec(t), encode: func() error {
				close(arrived)
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}}
			var changed bool
			var updateErr error
			go func() {
				defer close(done)
				changed, updateErr = NewHookConfigRepo(f.p, codec).UpdateConfig(ctx, o, entity.HookConfigUpdateInput{Config: configBefore(), KeyID: "test-key"})
			}()
			configWait(t, arrived)
			if change == "unmarked-history" {
				require.NoError(t, f.sql.Create(&model.ExptRunLog{ID: hookTxSequence.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: hookTxSequence.Add(1)}).Error)
			} else {
				require.NoError(t, f.sql.Exec("UPDATE "+configTable(o)+" SET deleted_at=NOW() WHERE id=?", o.ObjectID).Error)
			}
			unblock()
			configWait(t, done)
			if change == "unmarked-history" {
				require.ErrorIs(t, updateErr, entity.ErrHookConfigImmutable)
			} else {
				require.ErrorIs(t, updateErr, entity.ErrHookStoreMissing)
			}
			require.False(t, changed)
			require.Nil(t, configRaw(t, f, o))
		})
	}
}
