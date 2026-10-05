// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"fmt"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"

	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

// Run only after the parent allocates the shared MySQL fixture.
func TestHookConfigCreateMySQLAtomic(t *testing.T) {
	for _, template := range []bool{false, true} {
		for _, conflict := range []bool{false, true} {
			t.Run(fmt.Sprintf("template=%v/ref_conflict=%v", template, conflict), func(t *testing.T) {
				f := newHookTxFixture(t)
				ctx := context.Background()
				objectID, refID := hookTxSequence.Add(1), hookTxSequence.Add(1)
				in := repo.HookConfigCreateInput{WorkspaceID: f.space, ExecutionScope: "local", KeyID: "test-key",
					Config: &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(`{"private":"copy"}`)}}}
				configs := NewHookConfigRepo(f.p, configTestCodec(t))
				creator, ok := configs.(repo.IHookConfigCreator)
				require.True(t, ok)
				owner := hookcomponent.ConfigOwner{WorkspaceID: f.space, ObjectID: objectID, Kind: hookcomponent.ConfigOwnerExperiment, ExecutionScope: "local"}
				table, refTable := model.TableNameExperiment, model.TableNameExptEvaluatorRef
				if template {
					owner.Kind = hookcomponent.ConfigOwnerTemplate
					table, refTable = model.TableNameExptTemplate, model.TableNameExptTemplateEvaluatorRef
				}
				t.Cleanup(func() {
					require.NoError(t, f.sql.Exec("DELETE FROM "+refTable+" WHERE space_id=? AND id=?", f.space, refID).Error)
					require.NoError(t, f.sql.Exec("DELETE FROM "+table+" WHERE space_id=? AND id=?", f.space, objectID).Error)
				})
				if conflict {
					if template {
						require.NoError(t, f.sql.Create(&model.ExptTemplateEvaluatorRef{ID: refID, SpaceID: f.space, ExptTemplateID: f.expt, EvaluatorVersionID: 42}).Error)
					} else {
						require.NoError(t, f.sql.Create(&model.ExptEvaluatorRef{ID: refID, SpaceID: f.space, ExptID: f.expt, EvaluatorVersionID: 42}).Error)
					}
				}
				var err error
				if template {
					err = creator.CreateTemplateWithHookConfig(ctx, &entity.ExptTemplate{Meta: &entity.ExptTemplateMeta{ID: objectID, WorkspaceID: f.space, Name: fmt.Sprint(objectID)}},
						[]*entity.ExptTemplateEvaluatorRef{{ID: refID, SpaceID: f.space, ExptTemplateID: objectID, EvaluatorVersionID: 42}}, in)
				} else {
					err = creator.CreateExperimentWithHookConfig(ctx, &entity.Experiment{ID: objectID, SpaceID: f.space, Name: fmt.Sprint(objectID)},
						[]*entity.ExptEvaluatorRef{{ID: refID, SpaceID: f.space, ExptID: objectID, EvaluatorVersionID: 42}}, in)
				}
				var count int64
				require.NoError(t, f.sql.Table(table).Where("space_id=? AND id=?", f.space, objectID).Count(&count).Error)
				if conflict {
					require.ErrorIs(t, err, entity.ErrHookConfigStorage)
					require.Zero(t, count, "ref insert failure must roll back the newly encrypted object")
					return
				}
				require.NoError(t, err)
				require.Equal(t, int64(1), count)
				got, err := configs.GetConfig(ctx, owner)
				require.NoError(t, err)
				require.False(t, *got.Config.Before.Enabled)
				require.JSONEq(t, `{"private":"copy"}`, *got.Config.Before.ParametersJSON)
				require.NotEmpty(t, got.Revision)
				require.NoError(t, f.sql.Table(refTable).Where("space_id=? AND id=?", f.space, refID).Count(&count).Error)
				require.Equal(t, int64(1), count)
				if template {
					var row model.ExptTemplate
					require.NoError(t, f.sql.Take(&row, "space_id=? AND id=?", f.space, objectID).Error)
					require.Nil(t, row.ScheduleRunBinding)
				}
			})
		}
	}
}
