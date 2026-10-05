// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type hookManagementDecodeCheck struct {
	hookcomponent.StorageCodec
	check func()
}

func (c hookManagementDecodeCheck) DecodeConfig(ctx context.Context, owner hookcomponent.ConfigOwner, raw []byte) (*entity.LifecycleHookConf, error) {
	c.check()
	return c.StorageCodec.DecodeConfig(ctx, owner, raw)
}

type hookManagementNoSQL struct {
	db.Provider
	t *testing.T
}

func (p hookManagementNoSQL) NewSession(context.Context, ...db.Option) *gorm.DB {
	p.t.Fatal("invalid batch must be rejected before SQL")
	return nil
}

func TestHookManagementBatchOneQuery(t *testing.T) {
	p, m := initializationDB(t)
	codec := configTestCodec(t)
	reader, ok := NewHookConfigRepo(p, hookManagementDecodeCheck{codec, func() { require.NoError(t, m.ExpectationsWereMet(), "SQL rows must be closed before decrypting") }}).(repo.IHookConfigBatchReader)
	require.True(t, ok, "bounded batch reader capability is missing")
	owner := hookcomponent.ConfigOwner{WorkspaceID: 10, ObjectID: 20, Kind: hookcomponent.ConfigOwnerExperiment, ExecutionScope: "local"}
	raw, err := codec.EncodeConfig(context.Background(), "key", owner, configBefore())
	require.NoError(t, err)
	owners := []hookcomponent.ConfigOwner{owner, owner, owner, owner}
	owners[1].ObjectID, owners[2].ObjectID, owners[3].ObjectID = 21, 22, 23
	m.ExpectQuery("SELECT .*FROM .experiment.*space_id.*id IN").WillReturnRows(
		sqlmock.NewRows([]string{"id", "space_id", "lifecycle_hook_conf"}).AddRow(23, 10, raw).AddRow(21, 10, nil).AddRow(20, 10, raw)).RowsWillBeClosed()
	got, err := reader.MGetConfigs(context.Background(), owners)
	require.NoError(t, err)
	require.Len(t, got, 4)
	require.Equal(t, owners[0], got[0].Owner)
	require.NoError(t, got[0].Err)
	require.NotEmpty(t, got[0].Record.Revision)
	require.NoError(t, got[1].Err)
	require.Nil(t, got[1].Record.Config)
	require.Empty(t, got[1].Record.Revision)
	require.ErrorIs(t, got[2].Err, entity.ErrHookStoreMissing)
	require.Nil(t, got[2].Record)
	require.ErrorIs(t, got[3].Err, entity.ErrHookConfigStorage, "ciphertext from another object must fail")
	require.Nil(t, got[3].Record)
}

func TestHookManagementUpdaterCapability(t *testing.T) {
	_, ok := NewHookConfigRepo(nil, nil).(repo.IHookConfigMetadataUpdater)
	require.True(t, ok, "atomic metadata/config update capability is missing")
}

func TestHookManagementBatchBounds(t *testing.T) {
	reader := NewHookConfigRepo(hookManagementNoSQL{t: t}, configTestCodec(t)).(repo.IHookConfigBatchReader)
	owner := hookcomponent.ConfigOwner{WorkspaceID: 10, ObjectID: 20, Kind: hookcomponent.ConfigOwnerTemplate, ExecutionScope: "local"}
	for _, change := range []string{"kind", "workspace", "scope", "duplicate", "zero", "oversize"} {
		t.Run(change, func(t *testing.T) {
			owners := []hookcomponent.ConfigOwner{owner, owner}
			owners[1].ObjectID++
			switch change {
			case "kind":
				owners[1].Kind = hookcomponent.ConfigOwnerExperiment
			case "workspace":
				owners[1].WorkspaceID++
			case "scope":
				owners[1].ExecutionScope = "other"
			case "duplicate":
				owners[1] = owner
			case "zero":
				owners[1].ObjectID = 0
			case "oversize":
				owners = make([]hookcomponent.ConfigOwner, 101)
			}
			got, err := reader.MGetConfigs(context.Background(), owners)
			require.ErrorIs(t, err, entity.ErrHookConfigStorage)
			require.Nil(t, got)
		})
	}
	got, err := NewHookConfigRepo(nil, nil).(repo.IHookConfigBatchReader).MGetConfigs(context.Background(), nil)
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestHookManagementAtomicUpdateSQL(t *testing.T) {
	for _, template := range []bool{false, true} {
		for _, failure := range []string{"success", "stale", "locked_stale", "crypto", "metadata", "config", "latest", "history", "binding"} {
			if template && (failure == "latest" || failure == "history") || !template && failure == "binding" {
				continue
			}
			t.Run(fmt.Sprintf("template=%v/%s", template, failure), func(t *testing.T) {
				p, m := initializationDB(t)
				codec := &hookCreateCodec{StorageCodec: configTestCodec(t)}
				configs := NewHookConfigRepo(hookCreateProvider{p, t, codec}, codec)
				updater, ok := configs.(repo.IHookConfigMetadataUpdater)
				require.True(t, ok)
				owner := hookcomponent.ConfigOwner{WorkspaceID: 10, ObjectID: 20, Kind: hookcomponent.ConfigOwnerExperiment, ExecutionScope: "local"}
				table := "experiment"
				if template {
					owner.Kind = hookcomponent.ConfigOwnerTemplate
					table = "expt_template"
				}
				raw, err := codec.StorageCodec.EncodeConfig(context.Background(), "key", owner, configBefore())
				require.NoError(t, err)
				in := entity.HookConfigUpdateInput{ExpectedRevision: hookConfigRevision(raw), KeyID: "key", Config: &entity.LifecycleHookConf{After: &entity.HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(`{"new":2}`)}}}
				m.ExpectQuery("SELECT .*FROM ." + table + ".*").WillReturnRows(sqlmock.NewRows([]string{"lifecycle_hook_conf", "latest_run_id"}).AddRow(raw, 0))
				if failure == "stale" {
					in.ExpectedRevision = "stale"
				}
				if failure == "crypto" {
					codec.err = errors.New("private crypto failure")
				}
				want := entity.ErrHookConfigStorage
				if failure == "stale" || failure == "locked_stale" {
					want = entity.ErrHookStoreConflict
				}
				if failure == "latest" || failure == "history" {
					want = entity.ErrHookConfigImmutable
				}
				if failure == "binding" {
					want = repo.ErrHookConfigScheduleBindingRequired
				}
				if failure != "stale" && failure != "crypto" {
					m.ExpectBegin()
					lockedRaw := raw
					if failure == "locked_stale" {
						lockedRaw = []byte("different revision")
					}
					if template {
						var binding any
						if failure == "binding" {
							binding = []byte("existing binding")
						}
						m.ExpectQuery("SELECT .*FROM .expt_template.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"lifecycle_hook_conf", "cron_activate", "schedule_run_binding", "template_conf"}).AddRow(lockedRaw, false, binding, nil))
					} else {
						latest := int64(0)
						if failure == "latest" {
							latest = 99
						}
						m.ExpectQuery("SELECT .*FROM .experiment.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"lifecycle_hook_conf", "latest_run_id"}).AddRow(lockedRaw, latest))
						if failure != "latest" {
							rows := sqlmock.NewRows([]string{"id"})
							if failure == "history" {
								rows.AddRow(99)
							}
							m.ExpectQuery("SELECT .*FROM .expt_run_log.*FOR UPDATE").WillReturnRows(rows)
						}
					}
					if failure == "success" || failure == "metadata" || failure == "config" {
						q := m.ExpectExec("UPDATE ." + table + ". SET")
						if failure == "metadata" {
							q.WillReturnError(errors.New("private metadata error"))
						} else {
							q.WillReturnResult(sqlmock.NewResult(0, 1))
							if template {
								m.ExpectQuery("SELECT .*FROM .expt_template_evaluator_ref.").WillReturnRows(sqlmock.NewRows([]string{"id", "expt_template_id", "space_id", "evaluator_id", "evaluator_version_id", "deleted_at"}))
							}
							q = m.ExpectExec("UPDATE ." + table + ". SET .lifecycle_hook_conf.")
							if failure == "config" {
								q.WillReturnError(errors.New("private config error"))
							} else {
								q.WillReturnResult(sqlmock.NewResult(0, 1))
							}
						}
					}
					if failure == "success" {
						m.ExpectCommit()
					} else {
						m.ExpectRollback()
					}
				}
				if template {
					err = updater.UpdateTemplateWithHookConfig(context.Background(), owner, &entity.ExptTemplate{Meta: &entity.ExptTemplateMeta{ID: 20, WorkspaceID: 10, Name: "new"}}, nil, in)
				} else {
					err = updater.UpdateExperimentWithHookConfig(context.Background(), owner, &entity.Experiment{ID: 20, SpaceID: 10, Name: "new"}, in)
				}
				if failure == "success" {
					require.NoError(t, err)
					got, err := codec.DecodeConfig(context.Background(), owner, codec.raw)
					require.NoError(t, err)
					require.True(t, *got.Before.Enabled, "absent stage must be inherited")
					require.False(t, *got.After.Enabled)
					require.JSONEq(t, `{"new":2}`, *got.After.ParametersJSON)
				} else {
					require.ErrorIs(t, err, want)
					require.NotContains(t, err.Error(), "private")
				}
			})
		}
	}
}

func TestHookManagementTemplateRefDiffSQL(t *testing.T) {
	for _, failInsert := range []bool{false, true} {
		t.Run(fmt.Sprint(failInsert), func(t *testing.T) {
			p, m := initializationDB(t)
			codec := &hookCreateCodec{StorageCodec: configTestCodec(t)}
			owner := hookcomponent.ConfigOwner{WorkspaceID: 10, ObjectID: 20, Kind: hookcomponent.ConfigOwnerTemplate, ExecutionScope: "local"}
			updater := NewHookConfigRepo(hookCreateProvider{p, t, codec}, codec).(repo.IHookConfigMetadataUpdater)
			in := entity.HookConfigUpdateInput{KeyID: "key", Config: configBefore()}
			m.ExpectQuery("SELECT .*FROM .expt_template.").WillReturnRows(sqlmock.NewRows([]string{"lifecycle_hook_conf"}).AddRow(nil))
			m.ExpectBegin()
			m.ExpectQuery("SELECT .*FROM .expt_template.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"lifecycle_hook_conf", "cron_activate", "schedule_run_binding", "template_conf"}).AddRow(nil, false, nil, nil))
			m.ExpectExec("UPDATE .expt_template. SET").WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectQuery("SELECT .*FROM .expt_template_evaluator_ref.").WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "expt_template_id", "evaluator_id", "evaluator_version_id", "deleted_at"}).
				AddRow(101, 10, 20, 1, 10, nil).AddRow(102, 10, 20, 2, 20, time.Now()).AddRow(103, 10, 20, 3, 30, nil))
			m.ExpectExec("UPDATE .expt_template_evaluator_ref. SET .deleted_at.").WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec("UPDATE .expt_template_evaluator_ref. SET .deleted_at.").WillReturnResult(sqlmock.NewResult(0, 1))
			q := m.ExpectExec("INSERT INTO .expt_template_evaluator_ref.")
			if failInsert {
				q.WillReturnError(errors.New("private ref insertion error"))
				m.ExpectRollback()
			} else {
				q.WillReturnResult(sqlmock.NewResult(301, 1))
				m.ExpectExec("UPDATE .expt_template. SET .lifecycle_hook_conf.").WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectCommit()
			}
			refs := []*entity.ExptTemplateEvaluatorRef{
				{ID: 301, SpaceID: 10, ExptTemplateID: 20, EvaluatorID: 1, EvaluatorVersionID: 10},
				{ID: 302, SpaceID: 10, ExptTemplateID: 20, EvaluatorID: 2, EvaluatorVersionID: 20},
				{ID: 303, SpaceID: 10, ExptTemplateID: 20, EvaluatorID: 4, EvaluatorVersionID: 40},
			}
			err := updater.UpdateTemplateWithHookConfig(context.Background(), owner, &entity.ExptTemplate{Meta: &entity.ExptTemplateMeta{ID: 20, WorkspaceID: 10, Name: "changed"}}, refs, in)
			if failInsert {
				require.ErrorIs(t, err, entity.ErrHookConfigStorage)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, int64(303), refs[2].ID, "legacy allocation must not mutate caller refs")
		})
	}
}

func TestHookManagementBatchLimitAndSQLFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			p, m := initializationDB(t)
			reader := NewHookConfigRepo(p, configTestCodec(t)).(repo.IHookConfigBatchReader)
			owners := make([]hookcomponent.ConfigOwner, 100)
			rows := sqlmock.NewRows([]string{"id", "space_id", "lifecycle_hook_conf"})
			for i := range owners {
				owners[i] = hookcomponent.ConfigOwner{WorkspaceID: 10, ObjectID: int64(i + 1), Kind: hookcomponent.ConfigOwnerTemplate, ExecutionScope: "local"}
				rows.AddRow(i+1, 10, nil)
			}
			q := m.ExpectQuery("SELECT .*FROM .expt_template.*LIMIT")
			if fail {
				q.WillReturnError(errors.New("private SQL error"))
			} else {
				q.WillReturnRows(rows).RowsWillBeClosed()
			}
			got, err := reader.MGetConfigs(context.Background(), owners)
			if fail {
				require.ErrorIs(t, err, entity.ErrHookConfigStorage)
				require.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.Len(t, got, 100)
			for i, item := range got {
				require.Equal(t, owners[i], item.Owner)
				require.NoError(t, item.Err)
				require.NotNil(t, item.Record)
				require.Nil(t, item.Record.Config)
			}
		})
	}
}
