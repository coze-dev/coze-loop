// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment_test

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bytedance/gg/gptr"
	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	apiExpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	apiOpen "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	appconvert "github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/experiment"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	infrarepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	mysqldao "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func patchTestDB(t *testing.T) (db.Provider, sqlmock.Sqlmock) {
	t.Helper()
	conn, m, err := sqlmock.New()
	require.NoError(t, err)
	p, err := db.NewDB(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, m.ExpectationsWereMet()); _ = conn.Close() })
	return p, m
}

// Parent-allocated MySQL only; exercises the actual wrapper, legacy Manager, repos and transaction.
func TestHookManagementMySQLPatchRefs(t *testing.T) {
	for _, clear := range []bool{false, true} {
		for _, fail := range []bool{false, true} {
			t.Run(fmt.Sprintf("clear=%v/fail=%v", clear, fail), func(t *testing.T) {
				dsn := os.Getenv("HOOK_MYSQL_TX_DSN")
				if dsn == "" {
					t.Skip("requires parent-allocated HOOK_MYSQL_TX_DSN")
				}
				cfg, err := mysqldriver.ParseDSN(dsn)
				require.NoError(t, err)
				require.Equal(t, "unix", cfg.Net)
				require.Equal(t, "hook_7378265404_tx", cfg.DBName)
				p, err := db.NewDB(mysql.Open(dsn), &gorm.Config{Logger: logger.Discard})
				require.NoError(t, err)
				ctx := context.Background()
				sql := p.NewSession(ctx)
				conn, err := sql.DB()
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, conn.Close()) })
				id := time.Now().UnixNano() / 1000
				space := id + 1
				require.NoError(t, sql.Create(&model.ExptTemplate{ID: id, SpaceID: space, Name: "original", EvalSetID: 11, EvalSetVersionID: 12, TargetID: 13, TargetVersionID: 14}).Error)
				t.Cleanup(func() {
					require.NoError(t, sql.Exec("DELETE FROM expt_template_evaluator_ref WHERE space_id=? AND expt_template_id=?", space, id).Error)
					require.NoError(t, sql.Exec("DELETE FROM expt_template WHERE space_id=? AND id=?", space, id).Error)
				})
				require.NoError(t, sql.Create(&model.ExptTemplateEvaluatorRef{ID: id + 2, SpaceID: space, ExptTemplateID: id, EvaluatorID: 101, EvaluatorVersionID: 201}).Error)
				require.NoError(t, sql.Create(&model.ExptTemplateEvaluatorRef{ID: id + 3, SpaceID: space, ExptTemplateID: id, EvaluatorID: 102, EvaluatorVersionID: 202, DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}}).Error)
				oldRepo := infrarepo.NewExptTemplateRepo(mysqldao.NewExptTemplateDAO(p), mysqldao.NewExptTemplateEvaluatorRefDAO(p), &patchReservedIDs{ids: []int64{id + 4}})
				configs := infrarepo.NewHookConfigRepo(p, patchTestCodec(t))
				stop := errors.New("committed; stop before unrelated hydration")
				persist := &hookManagementPersistStop{IHookConfigRepo: configs, IHookConfigMetadataUpdater: configs.(repo.IHookConfigMetadataUpdater), stop: stop}
				manager := service.NewExptTemplateManager(oldRepo, &patchReservedIDs{ids: []int64{id + 5}}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
				owner := hookcomponent.ConfigOwner{WorkspaceID: space, ObjectID: id, Kind: hookcomponent.ConfigOwnerTemplate, ExecutionScope: "local"}
				scoped, err := service.WithExptTemplateHookConfigUpdate(manager, persist, owner, entity.HookConfigUpdateInput{KeyID: "key", Config: patchConfig()})
				require.NoError(t, err)
				if fail {
					callback := fmt.Sprintf("patch_config_fail_%d", id)
					require.NoError(t, sql.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
						if tx.Statement.Table != "expt_template" {
							return
						}
						if values, ok := tx.Statement.Dest.(map[string]any); ok && values["lifecycle_hook_conf"] != nil {
							tx.AddError(errors.New("injected config write failure"))
						}
					}))
					t.Cleanup(func() { require.NoError(t, sql.Callback().Update().Remove(callback)) })
				}
				param := &entity.UpdateExptTemplateParam{TemplateID: id, SpaceID: space, Description: "changed metadata"}
				if clear {
					param.EvaluatorIDVersionItems = []*entity.EvaluatorIDVersionItem{}
				}
				_, err = scoped.Update(ctx, param, &entity.Session{UserID: "authorized"})
				if fail {
					require.ErrorIs(t, err, entity.ErrHookConfigStorage)
				} else {
					require.ErrorIs(t, err, stop)
				}
				got, err := oldRepo.GetByID(ctx, id, &space)
				require.NoError(t, err)
				want := []int64{201}
				if clear && !fail {
					want = []int64{}
				}
				require.Equal(t, want, got.TripleConfig.EvaluatorVersionIds)
				require.Len(t, got.EvaluatorVersionRef, len(want))
				require.Equal(t, int64(11), got.TripleConfig.EvalSetID)
				require.Equal(t, int64(12), got.TripleConfig.EvalSetVersionID)
				require.Equal(t, int64(13), got.TripleConfig.TargetID)
				require.Equal(t, int64(14), got.TripleConfig.TargetVersionID)
				var refs []*model.ExptTemplateEvaluatorRef
				require.NoError(t, sql.Unscoped().Where("space_id=? AND expt_template_id=?", space, id).Order("id").Find(&refs).Error)
				require.Len(t, refs, 2)
				require.Equal(t, id+2, refs[0].ID)
				require.Equal(t, clear && !fail, refs[0].DeletedAt.Valid)
				require.Equal(t, id+3, refs[1].ID)
				require.True(t, refs[1].DeletedAt.Valid, "historical deleted refs must not be revived")
				cfgRecord, err := configs.GetConfig(ctx, owner)
				require.NoError(t, err)
				if fail {
					require.Nil(t, cfgRecord.Config)
					require.Empty(t, got.Meta.Desc)
				} else {
					require.False(t, *cfgRecord.Config.After.Enabled)
					require.Equal(t, "changed metadata", got.Meta.Desc)
				}
				if !clear {
					require.Nil(t, param.EvaluatorIDVersionItems)
				}
			})
		}
	}
}

type patchProtector struct{ cipher.AEAD }

func (p patchProtector) Protect(_ context.Context, key string, plain []byte) ([]byte, error) {
	nonce := make([]byte, p.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return p.Seal(nonce, nonce, plain, []byte(key)), nil
}
func (p patchProtector) Unprotect(_ context.Context, key string, raw []byte) ([]byte, error) {
	n := p.NonceSize()
	if len(raw) < n {
		return nil, errors.New("invalid test cipher")
	}
	return p.Open(nil, raw[:n], raw[n:], []byte(key))
}
func patchTestCodec(t *testing.T) hookcomponent.StorageCodec {
	t.Helper()
	block, err := aes.NewCipher(make([]byte, 32))
	require.NoError(t, err)
	aead, err := cipher.NewGCM(block)
	require.NoError(t, err)
	return hookinfra.NewStorageCodec(patchProtector{aead})
}
func patchConfig() *entity.LifecycleHookConf {
	return &entity.LifecycleHookConf{After: &entity.HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(`{"private":"patch"}`)}}
}

type patchReservedIDs struct{ ids []int64 }

func (g *patchReservedIDs) GenID(ctx context.Context) (int64, error) {
	ids, err := g.GenMultiIDs(ctx, 1)
	if err != nil {
		return 0, err
	}
	return ids[0], nil
}
func (g *patchReservedIDs) GenMultiIDs(_ context.Context, n int) ([]int64, error) {
	if n < 0 || n > len(g.ids) {
		return nil, errors.New("test ID reserve exhausted")
	}
	out := append([]int64(nil), g.ids[:n]...)
	g.ids = g.ids[n:]
	return out, nil
}

func TestHookManagementTemplatePresenceContract(t *testing.T) {
	for _, tc := range []struct {
		name, body   string
		web, wantNil bool
	}{
		{"web_omitted", `{}`, true, true},
		{"web_empty_config", `{"triple_config":{}}`, true, false},
		{"web_empty_items", `{"triple_config":{"evaluator_id_version_items":[]}}`, true, false},
		{"openapi_omitted", `{}`, false, true},
		{"openapi_empty_config", `{"triple_config":{}}`, false, true},
		{"openapi_empty_items", `{"triple_config":{"evaluator_id_version_items":[]}}`, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var param *entity.UpdateExptTemplateParam
			var err error
			if tc.web {
				var req apiExpt.UpdateExperimentTemplateRequest
				require.NoError(t, json.Unmarshal([]byte(tc.body), &req))
				param, err = appconvert.ConvertUpdateExptTemplateReq(&req)
			} else {
				var req apiOpen.UpdateExptTemplateOApiRequest
				require.NoError(t, json.Unmarshal([]byte(tc.body), &req))
				param, err = appconvert.OpenAPIUpdateExptTemplateReq2Domain(&req)
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantNil, param.EvaluatorIDVersionItems == nil)
			require.Empty(t, param.EvaluatorIDVersionItems)
		})
	}
}

type hookManagementPatchReadRepo struct {
	repo.IExptTemplateRepo
	template *entity.ExptTemplate
}

func (r hookManagementPatchReadRepo) GetByID(context.Context, int64, *int64) (*entity.ExptTemplate, error) {
	return r.template, nil
}
func (r hookManagementPatchReadRepo) GetByName(context.Context, string, int64, entity.ExptType) (*entity.ExptTemplate, bool, error) {
	return nil, false, nil
}

// Stop after the actual updater returns, before unrelated tuple hydration/external services.
type hookManagementPersistStop struct {
	repo.IHookConfigRepo
	repo.IHookConfigMetadataUpdater
	template *entity.ExptTemplate
	refs     []*entity.ExptTemplateEvaluatorRef
	stop     error
}

func (p *hookManagementPersistStop) UpdateTemplateWithHookConfig(ctx context.Context, owner hookcomponent.ConfigOwner, template *entity.ExptTemplate, refs []*entity.ExptTemplateEvaluatorRef, in entity.HookConfigUpdateInput) error {
	p.template, p.refs = template, refs
	if err := p.IHookConfigMetadataUpdater.UpdateTemplateWithHookConfig(ctx, owner, template, refs, in); err != nil {
		return err
	}
	return p.stop
}

func TestHookManagementTemplatePatchSQL(t *testing.T) {
	for _, clear := range []bool{false, true} {
		for _, fail := range []bool{false, true} {
			t.Run(fmt.Sprintf("clear=%v/fail=%v", clear, fail), func(t *testing.T) {
				p, m := patchTestDB(t)
				codec := patchTestCodec(t)
				configs := infrarepo.NewHookConfigRepo(p, codec)
				stop := errors.New("committed; stop before unrelated hydration")
				persist := &hookManagementPersistStop{IHookConfigRepo: configs, IHookConfigMetadataUpdater: configs.(repo.IHookConfigMetadataUpdater), stop: stop}
				original := &entity.ExptTemplate{Meta: &entity.ExptTemplateMeta{ID: 20, WorkspaceID: 10, Name: "old"},
					TripleConfig:        &entity.ExptTemplateTuple{EvaluatorIDVersionItems: []*entity.EvaluatorIDVersionItem{{EvaluatorID: 101, EvaluatorVersionID: 201}}, EvaluatorVersionIds: []int64{201}},
					EvaluatorVersionRef: []*entity.ExptTemplateEvaluatorVersionRef{{EvaluatorID: 101, EvaluatorVersionID: 201}}}
				base := service.NewExptTemplateManager(hookManagementPatchReadRepo{template: original}, &patchReservedIDs{ids: []int64{301}}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
				owner := hookcomponent.ConfigOwner{WorkspaceID: 10, ObjectID: 20, Kind: hookcomponent.ConfigOwnerTemplate, ExecutionScope: "local"}
				scoped, err := service.WithExptTemplateHookConfigUpdate(base, persist, owner, entity.HookConfigUpdateInput{KeyID: "key", Config: patchConfig()})
				require.NoError(t, err)
				m.ExpectQuery("SELECT .*FROM .expt_template.").WillReturnRows(sqlmock.NewRows([]string{"lifecycle_hook_conf"}).AddRow(nil))
				m.ExpectBegin()
				m.ExpectQuery("SELECT .*FROM .expt_template.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"lifecycle_hook_conf", "cron_activate", "schedule_run_binding", "template_conf"}).AddRow(nil, false, nil, nil))
				m.ExpectExec("UPDATE .expt_template. SET").WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectQuery("SELECT .*FROM .expt_template_evaluator_ref.").WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "expt_template_id", "evaluator_id", "evaluator_version_id", "deleted_at"}).AddRow(99, 10, 20, 101, 201, nil))
				if clear {
					m.ExpectExec("UPDATE .expt_template_evaluator_ref. SET .deleted_at.").WillReturnResult(sqlmock.NewResult(0, 1))
				}
				q := m.ExpectExec("UPDATE .expt_template. SET .lifecycle_hook_conf.")
				if fail {
					q.WillReturnError(errors.New("private config failure"))
					m.ExpectRollback()
				} else {
					q.WillReturnResult(sqlmock.NewResult(0, 1))
					m.ExpectCommit()
				}
				param := &entity.UpdateExptTemplateParam{TemplateID: 20, SpaceID: 10}
				if clear {
					param.EvaluatorIDVersionItems = []*entity.EvaluatorIDVersionItem{}
				}
				_, err = scoped.Update(context.Background(), param, &entity.Session{UserID: "authorized"})
				if fail {
					require.ErrorIs(t, err, entity.ErrHookConfigStorage)
				} else {
					require.ErrorIs(t, err, stop)
				}
				want := []int64{201}
				if clear {
					want = []int64{}
				}
				require.Equal(t, want, persist.template.TripleConfig.EvaluatorVersionIds)
				require.Len(t, persist.refs, len(want))
				require.Equal(t, []int64{201}, original.TripleConfig.EvaluatorVersionIds)
				if !clear {
					require.Nil(t, param.EvaluatorIDVersionItems)
				}
			})
		}
	}
}
