// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"

	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func TestHookConfigOwnershipDeletionAndTamper(t *testing.T) {
	for _, template := range []bool{false, true} {
		for _, change := range []string{"workspace", "id", "scope", "kind", "deleted", "tampered", "invalid-kind", "invalid-scope"} {
			t.Run(fmt.Sprintf("%v/%s", template, change), func(t *testing.T) {
				f := newHookTxFixture(t)
				o := configOwner(f)
				if template {
					o = configTemplate(t, f)
				}
				ctx := context.Background()
				r := NewHookConfigRepo(f.p, configTestCodec(t))
				_, err := r.UpdateConfig(ctx, o, entity.HookConfigUpdateInput{Config: configBefore(), KeyID: "test-key"})
				require.NoError(t, err)
				before, err := r.GetConfig(ctx, o)
				require.NoError(t, err)
				raw := configRaw(t, f, o)
				wrong := o
				switch change {
				case "workspace":
					wrong.WorkspaceID++
				case "id":
					wrong.ObjectID++
				case "scope":
					wrong.ExecutionScope = "ppe-other"
				case "kind":
					if template {
						wrong.Kind = hookcomponent.ConfigOwnerExperiment
					} else {
						wrong = configTemplate(t, f)
					}
					require.NoError(t, f.sql.Table(configTable(wrong)).Where("id=?", wrong.ObjectID).UpdateColumn("lifecycle_hook_conf", raw).Error)
				case "deleted":
					require.NoError(t, f.sql.Exec("UPDATE "+configTable(o)+" SET deleted_at=NOW() WHERE id=?", o.ObjectID).Error)
				case "tampered":
					raw = []byte(`{"private":"tampered-secret"}`)
					require.NoError(t, f.sql.Table(configTable(o)).Where("id=?", o.ObjectID).UpdateColumn("lifecycle_hook_conf", raw).Error)
				case "invalid-kind":
					wrong.Kind = "invalid"
				case "invalid-scope":
					wrong.ExecutionScope = "invalid scope"
				}
				got, err := r.GetConfig(ctx, wrong)
				require.Error(t, err)
				require.Nil(t, got)
				require.NotContains(t, err.Error(), "secret")
				changed, err := r.UpdateConfig(ctx, wrong, entity.HookConfigUpdateInput{Config: configBefore(), ExpectedRevision: before.Revision, KeyID: "test-key"})
				require.Error(t, err)
				require.False(t, changed)
				require.Equal(t, raw, configRaw(t, f, o))
			})
		}
	}
}

func TestHookConfigFailuresLeaveOriginalBytes(t *testing.T) {
	for _, template := range []bool{false, true} {
		for _, failure := range []string{"decode", "encode", "missing-key", "invalid-config", "stale-revision", "sql"} {
			t.Run(fmt.Sprintf("%v/%s", template, failure), func(t *testing.T) {
				f := newHookTxFixture(t)
				o := configOwner(f)
				if template {
					o = configTemplate(t, f)
				}
				ctx := context.Background()
				codec := configTestCodec(t)
				r := NewHookConfigRepo(f.p, codec)
				_, err := r.UpdateConfig(ctx, o, entity.HookConfigUpdateInput{Config: configBefore(), KeyID: "test-key"})
				require.NoError(t, err)
				before, err := r.GetConfig(ctx, o)
				require.NoError(t, err)
				raw := configRaw(t, f, o)
				in := entity.HookConfigUpdateInput{Config: &entity.LifecycleHookConf{After: &entity.HookConfig{Enabled: gptr.Of(false)}}, ExpectedRevision: before.Revision, KeyID: "test-key"}
				switch failure {
				case "decode":
					r = NewHookConfigRepo(f.p, configBoundaryCodec{StorageCodec: codec, decode: func() error { return errors.New("private-dependency-secret") }})
				case "encode":
					r = NewHookConfigRepo(f.p, configBoundaryCodec{StorageCodec: codec, encode: func() error { return errors.New("private-dependency-secret") }})
				case "missing-key":
					r = NewHookConfigRepo(f.p, hookinfra.NewStorageCodec(nil))
				case "invalid-config":
					in.Config.After.ParametersJSON = gptr.Of("[private-dependency-secret]")
				case "stale-revision":
					in.ExpectedRevision = strings.Repeat("0", 64)
				case "sql":
					trigger := fmt.Sprintf("hook_config_fail_%d", o.ObjectID)
					require.NoError(t, f.sql.Exec(fmt.Sprintf("CREATE TRIGGER %s BEFORE UPDATE ON %s FOR EACH ROW BEGIN IF NEW.id=%d THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='private-dependency-secret'; END IF; END", trigger, configTable(o), o.ObjectID)).Error)
					t.Cleanup(func() { require.NoError(t, f.sql.Exec("DROP TRIGGER "+trigger).Error) })
				}
				changed, err := r.UpdateConfig(ctx, o, in)
				require.Error(t, err)
				require.False(t, changed)
				require.NotContains(t, err.Error(), "private-dependency-secret")
				require.Equal(t, raw, configRaw(t, f, o))
			})
		}
	}
}

func TestHookConfigTemplateMayUpdateAfterExperimentRun(t *testing.T) {
	f := newHookTxFixture(t)
	o := configTemplate(t, f)
	ctx := context.Background()
	_, err := f.repo.CreateRunWithHooks(ctx, f.input(true, 0))
	require.NoError(t, err)
	r := NewHookConfigRepo(f.p, configTestCodec(t))
	changed, err := r.UpdateConfig(ctx, o, entity.HookConfigUpdateInput{Config: configBefore(), KeyID: "test-key"})
	require.NoError(t, err)
	require.True(t, changed)
	got, err := r.GetConfig(ctx, o)
	require.NoError(t, err)
	require.True(t, *got.Config.Before.Enabled)
	var row struct{ ScheduleRunBinding []byte }
	require.NoError(t, f.sql.Table("expt_template").Where("id=?", o.ObjectID).Select("schedule_run_binding").Take(&row).Error)
	require.Equal(t, []byte("original-binding"), row.ScheduleRunBinding)
}

func TestHookConfigOtherWorkspaceHistoryDoesNotFreezeDraft(t *testing.T) {
	f := newHookTxFixture(t)
	row := &model.ExptRunLog{ID: hookTxSequence.Add(1), SpaceID: f.space + 1000, ExptID: f.expt, ExptRunID: hookTxSequence.Add(1)}
	require.NoError(t, f.sql.Create(row).Error)
	t.Cleanup(func() { require.NoError(t, f.sql.Unscoped().Delete(&model.ExptRunLog{}, "id=?", row.ID).Error) })
	changed, err := NewHookConfigRepo(f.p, configTestCodec(t)).UpdateConfig(context.Background(), configOwner(f), entity.HookConfigUpdateInput{Config: configBefore(), KeyID: "test-key"})
	require.NoError(t, err)
	require.True(t, changed)
}
