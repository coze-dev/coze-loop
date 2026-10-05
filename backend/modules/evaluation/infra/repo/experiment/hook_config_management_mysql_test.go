// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The parent must allocate the shared real-MySQL fixture before running these tests.
func TestHookManagementMySQLAtomic(t *testing.T) {
	for _, template := range []bool{false, true} {
		for _, failConfig := range []bool{false, true} {
			t.Run(fmt.Sprintf("template=%v/config_failure=%v", template, failConfig), func(t *testing.T) {
				f := newHookTxFixture(t)
				ctx := context.Background()
				owner := configOwner(f)
				table := model.TableNameExperiment
				if template {
					owner.Kind = hookcomponent.ConfigOwnerTemplate
					table = model.TableNameExptTemplate
					require.NoError(t, f.sql.Create(&model.ExptTemplate{ID: f.expt, SpaceID: f.space, Name: "original"}).Error)
					t.Cleanup(func() {
						require.NoError(t, f.sql.Exec("DELETE FROM expt_template_evaluator_ref WHERE space_id=? AND expt_template_id=?", f.space, f.expt).Error)
						require.NoError(t, f.sql.Exec("DELETE FROM expt_template WHERE space_id=? AND id=?", f.space, f.expt).Error)
					})
				}
				codec := configTestCodec(t)
				raw, err := codec.EncodeConfig(ctx, "key", owner, configBefore())
				require.NoError(t, err)
				require.NoError(t, f.sql.Table(table).Where("id=? AND space_id=?", f.expt, f.space).Updates(map[string]any{"name": "original", "lifecycle_hook_conf": raw}).Error)
				refs := []*entity.ExptTemplateEvaluatorRef{}
				oldRefs := []*model.ExptTemplateEvaluatorRef{}
				if template {
					for i := int64(1); i <= 3; i++ {
						ref := &model.ExptTemplateEvaluatorRef{ID: hookTxSequence.Add(1), SpaceID: f.space, ExptTemplateID: f.expt, EvaluatorID: i, EvaluatorVersionID: i * 10}
						if i == 2 {
							ref.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
						}
						oldRefs = append(oldRefs, ref)
					}
					require.NoError(t, f.sql.Create(oldRefs).Error)
					for _, i := range []int64{1, 2, 4} {
						refs = append(refs, &entity.ExptTemplateEvaluatorRef{ID: hookTxSequence.Add(1), SpaceID: f.space, ExptTemplateID: f.expt, EvaluatorID: i, EvaluatorVersionID: i * 10})
					}
				}
				name := fmt.Sprintf("management_config_failure_%d", f.expt)
				if failConfig {
					require.NoError(t, f.sql.Callback().Update().Before("gorm:update").Register(name, func(tx *gorm.DB) {
						if tx.Statement.Table != table {
							return
						}
						if values, ok := tx.Statement.Dest.(map[string]any); ok && values["lifecycle_hook_conf"] != nil {
							tx.AddError(errors.New("injected config write failure"))
						}
					}))
					t.Cleanup(func() { require.NoError(t, f.sql.Callback().Update().Remove(name)) })
				}
				configs := NewHookConfigRepo(f.p, codec)
				updater := configs.(repo.IHookConfigMetadataUpdater)
				in := entity.HookConfigUpdateInput{ExpectedRevision: hookConfigRevision(raw), KeyID: "key", Config: &entity.LifecycleHookConf{After: &entity.HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(`{"saved":true}`)}}}
				if template {
					err = updater.UpdateTemplateWithHookConfig(ctx, owner, &entity.ExptTemplate{Meta: &entity.ExptTemplateMeta{ID: f.expt, WorkspaceID: f.space, Name: "changed"}}, refs, in)
				} else {
					err = updater.UpdateExperimentWithHookConfig(ctx, owner, &entity.Experiment{ID: f.expt, SpaceID: f.space, Name: "changed"}, in)
				}
				var row struct {
					Name               string
					LifecycleHookConf  []byte
					ScheduleRunBinding []byte
				}
				cols := "name,lifecycle_hook_conf"
				if template {
					cols += ",schedule_run_binding"
				}
				require.NoError(t, f.sql.Table(table).Select(cols).Where("id=? AND space_id=?", f.expt, f.space).Take(&row).Error)
				if failConfig {
					require.ErrorIs(t, err, entity.ErrHookConfigStorage)
					require.Equal(t, "original", row.Name)
					require.Equal(t, raw, row.LifecycleHookConf)
				} else {
					require.NoError(t, err)
					require.Equal(t, "changed", row.Name)
					require.NotEqual(t, raw, row.LifecycleHookConf)
					got, err := configs.GetConfig(ctx, owner)
					require.NoError(t, err)
					require.True(t, *got.Config.Before.Enabled)
					require.False(t, *got.Config.After.Enabled)
				}
				if template {
					require.Nil(t, row.ScheduleRunBinding)
					var got []*model.ExptTemplateEvaluatorRef
					require.NoError(t, f.sql.Unscoped().Where("space_id=? AND expt_template_id=?", f.space, f.expt).Order("evaluator_id").Find(&got).Error)
					if failConfig {
						require.Len(t, got, 3)
						for i, ref := range got {
							require.Equal(t, oldRefs[i].ID, ref.ID)
							require.Equal(t, i == 1, ref.DeletedAt.Valid)
						}
					} else {
						require.Len(t, got, 4)
						require.False(t, got[0].DeletedAt.Valid)
						require.False(t, got[1].DeletedAt.Valid)
						require.True(t, got[2].DeletedAt.Valid)
						require.False(t, got[3].DeletedAt.Valid)
						require.Equal(t, oldRefs[1].ID, got[1].ID, "restored ref must keep its old ID")
					}
				}
			})
		}
	}
}

func TestHookManagementMySQLBatchAndImmutable(t *testing.T) {
	f := newHookTxFixture(t)
	ctx := context.Background()
	owner := configOwner(f)
	configs := NewHookConfigRepo(f.p, configTestCodec(t))
	missing := owner
	missing.ObjectID = hookTxSequence.Add(1)
	got, err := configs.(repo.IHookConfigBatchReader).MGetConfigs(ctx, []hookcomponent.ConfigOwner{owner, missing})
	require.NoError(t, err)
	require.NoError(t, got[0].Err)
	require.Nil(t, got[0].Record.Config)
	require.ErrorIs(t, got[1].Err, entity.ErrHookStoreMissing)
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumn("name", "unchanged").Error)
	log := &model.ExptRunLog{ID: hookTxSequence.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: hookTxSequence.Add(1), DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}}
	require.NoError(t, f.sql.Create(log).Error)
	err = configs.(repo.IHookConfigMetadataUpdater).UpdateExperimentWithHookConfig(ctx, owner, &entity.Experiment{ID: f.expt, SpaceID: f.space, Name: "must-not-change"}, entity.HookConfigUpdateInput{KeyID: "key", Config: configBefore()})
	require.ErrorIs(t, err, entity.ErrHookConfigImmutable)
	var row model.Experiment
	require.NoError(t, f.sql.Take(&row, "id=?", f.expt).Error)
	require.Equal(t, "unchanged", row.Name)
	require.Nil(t, row.LifecycleHookConf)
}
