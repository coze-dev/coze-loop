// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	mysqldao "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

type configTestProtector struct{ aead cipher.AEAD }

func (p configTestProtector) Protect(_ context.Context, key string, b []byte) ([]byte, error) {
	nonce := make([]byte, p.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return p.aead.Seal(nonce, nonce, b, []byte(key)), nil
}
func (p configTestProtector) Unprotect(_ context.Context, key string, b []byte) ([]byte, error) {
	n := p.aead.NonceSize()
	if len(b) < n {
		return nil, errors.New("invalid test ciphertext")
	}
	return p.aead.Open(nil, b[:n], b[n:], []byte(key))
}
func configTestCodec(t *testing.T) hookcomponent.StorageCodec {
	t.Helper()
	b, err := aes.NewCipher(make([]byte, 32))
	require.NoError(t, err)
	a, err := cipher.NewGCM(b)
	require.NoError(t, err)
	return hookinfra.NewStorageCodec(configTestProtector{a})
}
func configOwner(f *hookTxFixture) hookcomponent.ConfigOwner {
	return hookcomponent.ConfigOwner{WorkspaceID: f.space, ObjectID: f.expt, Kind: hookcomponent.ConfigOwnerExperiment, ExecutionScope: "local"}
}
func configTemplate(t *testing.T, f *hookTxFixture) hookcomponent.ConfigOwner {
	t.Helper()
	o := configOwner(f)
	o.Kind = hookcomponent.ConfigOwnerTemplate
	require.NoError(t, f.sql.Create(&model.ExptTemplate{ID: o.ObjectID, SpaceID: o.WorkspaceID, Name: fmt.Sprint(o.ObjectID), ScheduleRunBinding: gptr.Of([]byte("original-binding"))}).Error)
	t.Cleanup(func() { require.NoError(t, f.sql.Unscoped().Delete(&model.ExptTemplate{}, "id=?", o.ObjectID).Error) })
	return o
}
func configTable(o hookcomponent.ConfigOwner) string {
	if o.Kind == hookcomponent.ConfigOwnerTemplate {
		return "expt_template"
	}
	return "experiment"
}
func configRaw(t *testing.T, f *hookTxFixture, o hookcomponent.ConfigOwner) []byte {
	t.Helper()
	var row struct{ LifecycleHookConf []byte }
	require.NoError(t, f.sql.Table(configTable(o)).Select("lifecycle_hook_conf").Where("id=? AND space_id=?", o.ObjectID, o.WorkspaceID).Take(&row).Error)
	return row.LifecycleHookConf
}
func configBefore() *entity.LifecycleHookConf {
	return &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://hook.example/before")}, ParametersJSON: gptr.Of(`{"private":"original","nested":{"x":1}}`)}}
}

func TestHookConfigAbsentAndNoop(t *testing.T) {
	for _, kind := range []hookcomponent.ConfigOwnerKind{hookcomponent.ConfigOwnerExperiment, hookcomponent.ConfigOwnerTemplate} {
		t.Run(string(kind), func(t *testing.T) {
			f := newHookTxFixture(t)
			o := configOwner(f)
			if kind == hookcomponent.ConfigOwnerTemplate {
				o = configTemplate(t, f)
			}
			r := NewHookConfigRepo(f.p, hookinfra.NewStorageCodec(nil))
			for _, raw := range [][]byte{nil, {}, []byte("null"), []byte("{}"), []byte(" { } ")} {
				require.NoError(t, f.sql.Table(configTable(o)).Where("id=?", o.ObjectID).UpdateColumn("lifecycle_hook_conf", raw).Error)
				got, err := r.GetConfig(context.Background(), o)
				require.NoError(t, err)
				require.Nil(t, got.Config)
				for _, conf := range []*entity.LifecycleHookConf{nil, {}} {
					changed, err := r.UpdateConfig(context.Background(), o, entity.HookConfigUpdateInput{Config: conf})
					require.NoError(t, err)
					require.False(t, changed)
					require.Equal(t, raw, configRaw(t, f, o))
				}
			}
		})
	}
	// An unrelated legacy edit must not acquire a DB or key dependency.
	changed, err := NewHookConfigRepo(nil, nil).UpdateConfig(context.Background(), hookcomponent.ConfigOwner{}, entity.HookConfigUpdateInput{})
	require.NoError(t, err)
	require.False(t, changed)
}

func TestHookConfigRoundTripStageReplacementAndLegacySave(t *testing.T) {
	for _, template := range []bool{false, true} {
		t.Run(fmt.Sprint(template), func(t *testing.T) {
			f := newHookTxFixture(t)
			o := configOwner(f)
			if template {
				o = configTemplate(t, f)
			}
			ctx := context.Background()
			r := NewHookConfigRepo(f.p, configTestCodec(t))
			require.NoError(t, f.sql.Table(configTable(o)).Where("id=?", o.ObjectID).UpdateColumn("notification_conf", []byte(`{"old":true}`)).Error)
			changed, err := r.UpdateConfig(ctx, o, entity.HookConfigUpdateInput{Config: configBefore(), KeyID: "test-key"})
			require.NoError(t, err)
			require.True(t, changed)
			first, err := r.GetConfig(ctx, o)
			require.NoError(t, err)
			require.NotEmpty(t, first.Revision)
			raw := configRaw(t, f, o)
			require.NotContains(t, string(raw), "hook.example")
			require.NotContains(t, string(raw), "original")
			changed, err = r.UpdateConfig(ctx, o, entity.HookConfigUpdateInput{Config: &entity.LifecycleHookConf{After: &entity.HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(`{"keep":7}`)}}, ExpectedRevision: first.Revision, KeyID: "test-key"})
			require.NoError(t, err)
			require.True(t, changed)
			second, err := r.GetConfig(ctx, o)
			require.NoError(t, err)
			require.Equal(t, "https://hook.example/before", *second.Config.Before.InvokeHTTPInfo.URL)
			require.False(t, *second.Config.After.Enabled)
			require.JSONEq(t, `{"keep":7}`, *second.Config.After.ParametersJSON)
			require.NotEqual(t, first.Revision, second.Revision)
			// Explicit stages replace, not deep-merge, including a disabled stage.
			_, err = r.UpdateConfig(ctx, o, entity.HookConfigUpdateInput{Config: &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(`{"new":2}`)}}, ExpectedRevision: second.Revision, KeyID: "test-key"})
			require.NoError(t, err)
			third, err := r.GetConfig(ctx, o)
			require.NoError(t, err)
			require.Nil(t, third.Config.Before.InvokeHTTPInfo.URL)
			require.JSONEq(t, `{"new":2}`, *third.Config.Before.ParametersJSON)
			require.JSONEq(t, `{"keep":7}`, *third.Config.After.ParametersJSON)
			raw = configRaw(t, f, o)
			if template {
				err = mysqldao.NewExptTemplateDAO(f.p).Update(ctx, &model.ExptTemplate{ID: o.ObjectID, Name: "legacy-edit"})
			} else {
				err = mysqldao.NewExptDAO(f.p).Update(ctx, &model.Experiment{ID: o.ObjectID, Name: "legacy-edit"})
			}
			require.NoError(t, err)
			require.Equal(t, raw, configRaw(t, f, o))
			var old struct {
				Name               string
				NotificationConf   []byte
				ScheduleRunBinding []byte
			}
			cols := "name,notification_conf"
			if template {
				cols += ",schedule_run_binding"
			}
			require.NoError(t, f.sql.Table(configTable(o)).Select(cols).Where("id=?", o.ObjectID).Take(&old).Error)
			require.Equal(t, "legacy-edit", old.Name)
			require.Equal(t, []byte(`{"old":true}`), old.NotificationConf)
			if template {
				require.Equal(t, []byte("original-binding"), old.ScheduleRunBinding)
			}
			body, err := json.Marshal(third)
			require.NoError(t, err)
			require.NotContains(t, string(body), "private")
		})
	}
}

func TestHookConfigRejectsRunHistory(t *testing.T) {
	for _, state := range []string{"latest-only", "legacy", "soft-deleted-legacy", "hook-marker"} {
		t.Run(state, func(t *testing.T) {
			f := newHookTxFixture(t)
			o := configOwner(f)
			ctx := context.Background()
			if state == "latest-only" {
				require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumn("latest_run_id", 123).Error)
			} else {
				row := &model.ExptRunLog{ID: hookTxSequence.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: hookTxSequence.Add(1)}
				if state == "hook-marker" {
					row.LifecycleHookVersion = gptr.Of(int32(1))
				}
				if state == "soft-deleted-legacy" {
					row.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
				}
				require.NoError(t, f.sql.Create(row).Error)
			}
			r := NewHookConfigRepo(f.p, configTestCodec(t))
			changed, err := r.UpdateConfig(ctx, o, entity.HookConfigUpdateInput{Config: configBefore(), KeyID: "test-key"})
			require.ErrorIs(t, err, entity.ErrHookConfigImmutable)
			require.False(t, changed)
			require.Nil(t, configRaw(t, f, o))
			changed, err = r.UpdateConfig(ctx, o, entity.HookConfigUpdateInput{Config: &entity.LifecycleHookConf{}})
			require.NoError(t, err)
			require.False(t, changed)
		})
	}
}

func TestHookConfigRunRejectsUnversionedProtectedConfig(t *testing.T) {
	f := newHookTxFixture(t)
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumn("lifecycle_hook_conf", []byte(`{"changed":true}`)).Error)
	got, err := f.repo.CreateRunWithHooks(context.Background(), f.input(true, 0))
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	require.Nil(t, got.Run)
	require.Zero(t, f.latest(t))
}
