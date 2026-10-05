// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestHookConfigCreateRejectsBeforeSQL(t *testing.T) {
	r, ok := NewHookConfigRepo(nil, nil).(repo.IHookConfigCreator)
	require.True(t, ok, "encrypted atomic creation capability is missing")
	in := repo.HookConfigCreateInput{WorkspaceID: 10, ExecutionScope: "local", KeyID: "key", Config: configBefore()}
	require.ErrorIs(t, r.CreateExperimentWithHookConfig(context.Background(), &entity.Experiment{ID: 20, SpaceID: 11}, nil, in), entity.ErrHookConfigStorage)
	require.ErrorIs(t, r.CreateTemplateWithHookConfig(context.Background(), &entity.ExptTemplate{Meta: &entity.ExptTemplateMeta{ID: 20, WorkspaceID: 11}}, nil, in), entity.ErrHookConfigStorage)
	require.ErrorIs(t, r.CreateExperimentWithHookConfig(context.Background(), &entity.Experiment{ID: 20, SpaceID: 10}, nil, in), entity.ErrHookConfigStorage)
}

type hookCreateCodec struct {
	hookcomponent.StorageCodec
	owner hookcomponent.ConfigOwner
	raw   []byte
	err   error
}

func (c *hookCreateCodec) EncodeConfig(ctx context.Context, key string, owner hookcomponent.ConfigOwner, conf *entity.LifecycleHookConf) ([]byte, error) {
	if c.err != nil {
		return nil, c.err
	}
	raw, err := c.StorageCodec.EncodeConfig(ctx, key, owner, conf)
	c.raw, c.owner = raw, owner
	return raw, err
}

type hookCreateProvider struct {
	db.Provider
	t     *testing.T
	codec *hookCreateCodec
}

func (p hookCreateProvider) Transaction(ctx context.Context, fn func(*gorm.DB) error, opts ...db.Option) error {
	require.NotEmpty(p.t, p.codec.raw, "encryption must finish before opening SQL transaction")
	require.True(p.t, db.ContainWithMasterOpt(opts))
	return p.Provider.Transaction(ctx, fn, opts...)
}
func TestHookConfigCreateSQLContract(t *testing.T) {
	for _, template := range []bool{false, true} {
		for _, failure := range []string{"success", "encode", "refs", "object"} {
			name := "experiment/" + failure
			if template {
				name = "template/" + failure
			}
			t.Run(name, func(t *testing.T) {
				sqlDB, m, err := sqlmock.New()
				require.NoError(t, err)
				defer sqlDB.Close()
				p, err := db.NewDB(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Discard})
				require.NoError(t, err)
				codec := &hookCreateCodec{StorageCodec: configTestCodec(t)}
				r, ok := NewHookConfigRepo(hookCreateProvider{p, t, codec}, codec).(repo.IHookConfigCreator)
				require.True(t, ok)
				in := repo.HookConfigCreateInput{WorkspaceID: 10, ExecutionScope: "local", KeyID: "key",
					Config: &entity.LifecycleHookConf{After: &entity.HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(`{"private":"retained"}`)}}}
				table, refsTable := "experiment", "expt_evaluator_ref"
				if template {
					table, refsTable = "expt_template", "expt_template_evaluator_ref"
				}
				seenObject := false
				require.NoError(t, p.NewSession(context.Background()).Callback().Create().Before("gorm:create").Register("hook_create_inspect", func(tx *gorm.DB) {
					switch po := tx.Statement.Dest.(type) {
					case *model.Experiment:
						seenObject = true
						require.Equal(t, codec.raw, gptr.Indirect(po.LifecycleHookConf))
						require.Equal(t, "kept-name", po.Name)
					case *model.ExptTemplate:
						seenObject = true
						require.Equal(t, codec.raw, gptr.Indirect(po.LifecycleHookConf))
						require.Nil(t, po.ScheduleRunBinding)
						require.Equal(t, "kept-name", po.Name)
					}
				}))
				if failure == "encode" {
					codec.err = errors.New("private codec detail")
				} else {
					m.ExpectBegin()
					q := m.ExpectExec("INSERT INTO ." + table + ".*lifecycle_hook_conf")
					if failure == "object" {
						q.WillReturnError(errors.New("private sql detail"))
					} else {
						q.WillReturnResult(sqlmock.NewResult(20, 1))
						refq := m.ExpectExec("INSERT INTO ." + refsTable + ".")
						if failure == "refs" {
							refq.WillReturnError(errors.New("private ref detail"))
						} else {
							refq.WillReturnResult(sqlmock.NewResult(31, 1))
						}
					}
					if failure == "success" {
						m.ExpectCommit()
					} else {
						m.ExpectRollback()
					}
				}
				if template {
					err = r.CreateTemplateWithHookConfig(context.Background(), &entity.ExptTemplate{Meta: &entity.ExptTemplateMeta{ID: 20, WorkspaceID: 10, Name: "kept-name"}}, []*entity.ExptTemplateEvaluatorRef{{ID: 31, ExptTemplateID: 20, SpaceID: 10}}, in)
				} else {
					err = r.CreateExperimentWithHookConfig(context.Background(), &entity.Experiment{ID: 20, SpaceID: 10, Name: "kept-name"}, []*entity.ExptEvaluatorRef{{ID: 31, ExptID: 20, SpaceID: 10}}, in)
				}
				if failure == "success" {
					require.NoError(t, err)
					require.True(t, seenObject)
					require.NotContains(t, string(codec.raw), "retained")
					require.Equal(t, int64(20), codec.owner.ObjectID)
					require.Equal(t, int64(10), codec.owner.WorkspaceID)
					wantKind := hookcomponent.ConfigOwnerExperiment
					if template {
						wantKind = hookcomponent.ConfigOwnerTemplate
					}
					require.Equal(t, wantKind, codec.owner.Kind)
					got, err := codec.DecodeConfig(context.Background(), codec.owner, codec.raw)
					require.NoError(t, err)
					require.False(t, *got.After.Enabled)
					require.JSONEq(t, `{"private":"retained"}`, *got.After.ParametersJSON)
					wrong := codec.owner
					wrong.ObjectID++
					_, err = codec.DecodeConfig(context.Background(), wrong, codec.raw)
					require.Error(t, err)
				} else {
					require.ErrorIs(t, err, entity.ErrHookConfigStorage)
					require.NotContains(t, err.Error(), "private")
				}
				require.NoError(t, m.ExpectationsWereMet())
			})
		}
	}
}
