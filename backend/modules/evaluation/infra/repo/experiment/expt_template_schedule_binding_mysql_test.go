// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	driver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	bindingMySQLSpace int64 = 7378265404009000
	bindingMySQLOwner       = "binding-mysql-fixture-7378265404"
)

func bindingMySQLAssert(t *testing.T, label string, ok bool) {
	t.Helper()
	t.Logf("ASSERT %s pass=%t", label, ok)
	if !ok {
		t.FailNow()
	}
}

func bindingMySQLProvider(t *testing.T) db.Provider {
	t.Helper()
	dsn := os.Getenv("HOOK_MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("SKIP: HOOK_MYSQL_TEST_DSN is absent; no MySQL evidence")
	}
	cfg, err := driver.ParseDSN(dsn)
	bindingMySQLAssert(t, "dsn_parse", assert.NoError(t, err))
	bindingMySQLAssert(t, "dsn_unix_only", assert.Equal(t, "unix", cfg.Net))
	bindingMySQLAssert(t, "dsn_isolated_schema", assert.Equal(t, "hook_7378265404_schema", cfg.DBName))
	bindingMySQLAssert(t, "dsn_parse_time", assert.True(t, cfg.ParseTime))
	bindingMySQLAssert(t, "dsn_utc", assert.Equal(t, "UTC", cfg.Loc.String()))
	bindingMySQLAssert(t, "dsn_session_timezone", assert.Equal(t, "'+00:00'", cfg.Params["time_zone"]))
	p, err := db.NewDB(mysql.Open(dsn), &gorm.Config{Logger: logger.Discard})
	bindingMySQLAssert(t, "real_provider_open", assert.NoError(t, err))
	s := p.NewSession(context.Background(), db.WithMaster())
	conn, err := s.DB()
	bindingMySQLAssert(t, "real_connection", assert.NoError(t, err))
	conn.SetMaxOpenConns(4)
	conn.SetMaxIdleConns(4)
	t.Cleanup(func() { bindingMySQLAssert(t, "connection_closed", assert.NoError(t, conn.Close())) })
	var env struct {
		Version, DatabaseName, Socket, TimeZone string
		SkipNetworking, LogBin                  int
	}
	err = s.Raw("SELECT VERSION() AS version, DATABASE() AS database_name, @@socket AS socket, @@session.time_zone AS time_zone, @@skip_networking AS skip_networking, @@log_bin AS log_bin").Scan(&env).Error
	bindingMySQLAssert(t, "server_readback", assert.NoError(t, err))
	bindingMySQLAssert(t, "server_database", assert.Equal(t, cfg.DBName, env.DatabaseName))
	bindingMySQLAssert(t, "server_socket", assert.Equal(t, cfg.Addr, env.Socket))
	bindingMySQLAssert(t, "server_utc", assert.Equal(t, "+00:00", env.TimeZone))
	var engine string
	err = s.Raw("SELECT ENGINE FROM information_schema.TABLES WHERE TABLE_SCHEMA=? AND TABLE_NAME='expt_template'", cfg.DBName).Scan(&engine).Error
	bindingMySQLAssert(t, "table_engine_query", assert.NoError(t, err))
	bindingMySQLAssert(t, "transactional_table", assert.Equal(t, "InnoDB", engine))
	t.Logf("MYSQL_ENABLED version=%s database=%s socket=%s skip_networking=%d log_bin=%d timezone=%s engine=%s", env.Version, env.DatabaseName, env.Socket, env.SkipNetworking, env.LogBin, env.TimeZone, engine)
	return p
}

func bindingMySQLSeed(t *testing.T, p db.Provider, id int64) entity.ExptTemplateScheduleBindingKey {
	t.Helper()
	s := p.NewSession(context.Background(), db.WithMaster())
	var rows []model.ExptTemplate
	err := s.Unscoped().Where("id=?", id).Find(&rows).Error
	bindingMySQLAssert(t, "fixture_existing_check", assert.NoError(t, err))
	for _, row := range rows {
		bindingMySQLAssert(t, "fixture_existing_space", assert.Equal(t, bindingMySQLSpace, row.SpaceID))
		bindingMySQLAssert(t, "fixture_existing_owner", assert.Equal(t, bindingMySQLOwner, row.CreatedBy))
	}
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		q := p.NewSession(ctx, db.WithMaster())
		res := q.Exec("DELETE FROM expt_template WHERE id=? AND space_id=? AND created_by=?", id, bindingMySQLSpace, bindingMySQLOwner)
		bindingMySQLAssert(t, "fixture_scoped_delete", assert.NoError(t, res.Error))
		var n int64
		err := q.Table("expt_template").Where("id=?", id).Count(&n).Error
		bindingMySQLAssert(t, "fixture_cleanup_query", assert.NoError(t, err))
		bindingMySQLAssert(t, "fixture_cleanup_zero", assert.Zero(t, n))
		t.Logf("FIXTURE_CLEANED id=%d space_id=%d rows_deleted=%d remaining=%d", id, bindingMySQLSpace, res.RowsAffected, n)
	}
	cleanup()
	t.Cleanup(cleanup)
	name := fmt.Sprintf("binding-mysql-%d", id)
	conf, hook := []byte(`{"fixture":"original"}`), []byte(`{"protected":"original"}`)
	err = s.Create(&model.ExptTemplate{ID: id, SpaceID: bindingMySQLSpace, Name: name, CreatedBy: bindingMySQLOwner,
		EvalSetID: 11, EvalSetVersionID: 12, TargetID: 13, TargetVersionID: 14, TemplateConf: &conf, LifecycleHookConf: &hook}).Error
	bindingMySQLAssert(t, "fixture_insert", assert.NoError(t, err))
	return entity.ExptTemplateScheduleBindingKey{SpaceID: bindingMySQLSpace, TemplateID: id, ExecutionScope: "local"}
}

func bindingMySQLInput(key entity.ExptTemplateScheduleBindingKey) *entity.ExptTemplateScheduleBinding {
	return &entity.ExptTemplateScheduleBinding{SchemaVersion: 1, BindingID: fmt.Sprintf("mysql-binding-%d", key.TemplateID), Version: 1,
		UserID: "mysql-user-original", IdentityType: "fornax_user", SpaceID: key.SpaceID, TemplateID: key.TemplateID, ExecutionScope: key.ExecutionScope,
		Namespace: "ns", Group: "group", BizKey: fmt.Sprintf("mysql-template.%d", key.TemplateID), Enabled: true, BoundAt: time.Unix(100, 0).UTC(),
		Callback: entity.ExptTemplateScheduleCallback{PSM: "test.evaluation", Method: "SubmitScheduledExptFromTemplate", Cluster: "default"}}
}

func bindingMySQLReceipt(key entity.ExptTemplateScheduleBindingKey) entity.ExptTemplateScheduleReceipt {
	return entity.ExptTemplateScheduleReceipt{JobID: "mysql-job", Namespace: "ns", Group: "group", BizKey: fmt.Sprintf("mysql-template.%d", key.TemplateID),
		Callback: entity.ExptTemplateScheduleCallback{PSM: "test.evaluation", Method: "SubmitScheduledExptFromTemplate", Cluster: "default"}}
}

func TestExptTemplateScheduleBindingMySQL(t *testing.T) {
	p := bindingMySQLProvider(t)
	r := NewExptTemplateScheduleBindingRepo(p)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	t.Run("save_read_activate_and_input_isolation", func(t *testing.T) {
		key := bindingMySQLSeed(t, p, 7378265404009101)
		got, err := r.Get(ctx, key)
		bindingMySQLAssert(t, "unbound_read", assert.NoError(t, err))
		bindingMySQLAssert(t, "unbound_nil", assert.Nil(t, got))
		input := bindingMySQLInput(key)
		changed, err := r.SaveCAS(ctx, key, 0, input)
		bindingMySQLAssert(t, "save", assert.NoError(t, err))
		bindingMySQLAssert(t, "save_changed", assert.True(t, changed))
		input.UserID, input.Callback.Method, input.Version = "rewritten-input", "rewritten-method", 90
		got, err = r.Get(ctx, key)
		bindingMySQLAssert(t, "persisted_read", assert.NoError(t, err))
		bindingMySQLAssert(t, "input_rewrite_not_persisted", assert.Equal(t, bindingMySQLInput(key), got))
		bindingMySQLAssert(t, "pending_not_active", assert.False(t, got.Active()))
		got.UserID = "rewritten-result"
		receipt := bindingMySQLReceipt(key)
		changed, err = r.ActivateCAS(ctx, key, input.BindingID, 1, receipt)
		bindingMySQLAssert(t, "activate", assert.NoError(t, err))
		bindingMySQLAssert(t, "activate_changed", assert.True(t, changed))
		receipt.JobID, receipt.Callback.PSM = "rewritten-job", "rewritten-psm"
		got, err = r.Get(ctx, key)
		bindingMySQLAssert(t, "active_read", assert.NoError(t, err))
		want := bindingMySQLInput(key)
		want.JobID = "mysql-job"
		bindingMySQLAssert(t, "receipt_and_result_rewrite_isolated", assert.Equal(t, want, got))
		bindingMySQLAssert(t, "registered_active", assert.True(t, got.Active()))
		changed, err = r.ActivateCAS(ctx, key, input.BindingID, 1, bindingMySQLReceipt(key))
		bindingMySQLAssert(t, "receipt_replay", assert.NoError(t, err))
		bindingMySQLAssert(t, "receipt_replay_no_write", assert.False(t, changed))
	})
	t.Run("same_version_one_winner", func(t *testing.T) {
		key := bindingMySQLSeed(t, p, 7378265404009102)
		_, err := r.SaveCAS(ctx, key, 0, bindingMySQLInput(key))
		bindingMySQLAssert(t, "race_seed", assert.NoError(t, err))
		type result struct {
			user    string
			changed bool
			err     error
		}
		ready, start, results := make(chan struct{}, 2), make(chan struct{}), make(chan result, 2)
		var workers sync.WaitGroup
		t.Cleanup(workers.Wait)
		for _, user := range []string{"mysql-user-a", "mysql-user-b"} {
			workers.Add(1)
			go func(user string) {
				defer workers.Done()
				next := bindingMySQLInput(key)
				next.Version, next.UserID = 2, user
				ready <- struct{}{}
				<-start
				changed, err := r.SaveCAS(ctx, key, 1, next)
				results <- result{user, changed, err}
			}(user)
		}
		<-ready
		<-ready
		close(start)
		winners, conflicts, winner := 0, 0, ""
		for i := 0; i < 2; i++ {
			select {
			case res := <-results:
				t.Logf("CAS_RESULT user=%s changed=%t err=%v", res.user, res.changed, res.err)
				if res.err == nil && res.changed {
					winners++
					winner = res.user
				} else {
					bindingMySQLAssert(t, "race_loser_conflict", assert.ErrorIs(t, res.err, entity.ErrExptTemplateScheduleBindingConflict))
					bindingMySQLAssert(t, "race_loser_not_changed", assert.False(t, res.changed))
					conflicts++
				}
			case <-ctx.Done():
				bindingMySQLAssert(t, "race_completed", assert.NoError(t, ctx.Err()))
			}
		}
		bindingMySQLAssert(t, "exactly_one_winner", assert.Equal(t, 1, winners))
		bindingMySQLAssert(t, "exactly_one_conflict", assert.Equal(t, 1, conflicts))
		got, err := r.Get(ctx, key)
		bindingMySQLAssert(t, "race_readback", assert.NoError(t, err))
		bindingMySQLAssert(t, "race_version_once", assert.Equal(t, int64(2), got.Version))
		bindingMySQLAssert(t, "race_winner_persisted", assert.Equal(t, winner, got.UserID))
	})
	t.Run("disable_rejects_old_receipt", func(t *testing.T) {
		key := bindingMySQLSeed(t, p, 7378265404009103)
		in := bindingMySQLInput(key)
		_, err := r.SaveCAS(ctx, key, 0, in)
		bindingMySQLAssert(t, "disable_seed", assert.NoError(t, err))
		_, err = r.ActivateCAS(ctx, key, in.BindingID, 1, bindingMySQLReceipt(key))
		bindingMySQLAssert(t, "disable_seed_active", assert.NoError(t, err))
		changed, err := r.DisableCAS(ctx, key, in.BindingID, 1)
		bindingMySQLAssert(t, "disable", assert.NoError(t, err))
		bindingMySQLAssert(t, "disable_changed", assert.True(t, changed))
		for _, version := range []int64{1, 2} {
			changed, err = r.ActivateCAS(ctx, key, in.BindingID, version, bindingMySQLReceipt(key))
			bindingMySQLAssert(t, fmt.Sprintf("disabled_receipt_v%d_rejected", version), assert.ErrorIs(t, err, entity.ErrExptTemplateScheduleBindingConflict))
			bindingMySQLAssert(t, fmt.Sprintf("disabled_receipt_v%d_no_write", version), assert.False(t, changed))
		}
		got, err := r.Get(ctx, key)
		bindingMySQLAssert(t, "disabled_readback", assert.NoError(t, err))
		bindingMySQLAssert(t, "disabled_version_incremented", assert.Equal(t, int64(2), got.Version))
		bindingMySQLAssert(t, "disabled_stays_disabled", assert.False(t, got.Enabled))
		bindingMySQLAssert(t, "disabled_stays_inactive", assert.False(t, got.Active()))
		bindingMySQLAssert(t, "disabled_receipt_retained", assert.Equal(t, "mysql-job", got.JobID))
	})
	for i, bound := range []bool{false, true} {
		t.Run(fmt.Sprintf("outer_transaction_rollback_bound_%t", bound), func(t *testing.T) {
			key := bindingMySQLSeed(t, p, 7378265404009104+int64(i))
			version := int64(0)
			if bound {
				_, err := r.SaveCAS(ctx, key, 0, bindingMySQLInput(key))
				bindingMySQLAssert(t, "rollback_seed_binding", assert.NoError(t, err))
				version = 1
			}
			s := p.NewSession(ctx, db.WithMaster())
			var before model.ExptTemplate
			err := s.Where("id=? AND space_id=?", key.TemplateID, key.SpaceID).Take(&before).Error
			bindingMySQLAssert(t, "rollback_before_read", assert.NoError(t, err))
			abort := errors.New("intentional outer transaction rollback")
			err = p.Transaction(ctx, func(tx *gorm.DB) error {
				res := tx.Model(&model.ExptTemplate{}).Where("id=? AND space_id=?", key.TemplateID, key.SpaceID).UpdateColumns(map[string]any{
					"name": "transaction-only", "template_conf": []byte(`{"fixture":"transaction"}`), "lifecycle_hook_conf": []byte(`{"protected":"transaction"}`)})
				bindingMySQLAssert(t, "outer_template_update", assert.NoError(t, res.Error))
				bindingMySQLAssert(t, "outer_template_updated_one", assert.Equal(t, int64(1), res.RowsAffected))
				next := bindingMySQLInput(key)
				next.Version, next.UserID = version+1, "transaction-user"
				changed, err := r.SaveCAS(ctx, key, version, next, db.WithTransaction(tx))
				bindingMySQLAssert(t, "inner_binding_save", assert.NoError(t, err))
				bindingMySQLAssert(t, "inner_binding_changed", assert.True(t, changed))
				changed, err = r.ActivateCAS(ctx, key, next.BindingID, next.Version, bindingMySQLReceipt(key), db.WithTransaction(tx))
				bindingMySQLAssert(t, "inner_binding_activate", assert.NoError(t, err))
				bindingMySQLAssert(t, "inner_activation_changed", assert.True(t, changed))
				got, err := r.Get(ctx, key, db.WithTransaction(tx))
				bindingMySQLAssert(t, "inner_binding_read", assert.NoError(t, err))
				bindingMySQLAssert(t, "inner_binding_sees_own_write", assert.Equal(t, "transaction-user", got.UserID))
				bindingMySQLAssert(t, "inner_binding_active", assert.True(t, got.Active()))
				var inside model.ExptTemplate
				bindingMySQLAssert(t, "inner_template_read", assert.NoError(t, tx.Where("id=? AND space_id=?", key.TemplateID, key.SpaceID).Take(&inside).Error))
				bindingMySQLAssert(t, "inner_template_sees_name", assert.Equal(t, "transaction-only", inside.Name))
				bindingMySQLAssert(t, "inner_template_sees_config", assert.Equal(t, []byte(`{"fixture":"transaction"}`), *inside.TemplateConf))
				return abort
			}, db.WithMaster())
			bindingMySQLAssert(t, "outer_abort_propagated", assert.ErrorIs(t, err, abort))
			var after model.ExptTemplate
			bindingMySQLAssert(t, "rollback_after_read", assert.NoError(t, s.Where("id=? AND space_id=?", key.TemplateID, key.SpaceID).Take(&after).Error))
			bindingMySQLAssert(t, "name_restored", assert.Equal(t, before.Name, after.Name))
			bindingMySQLAssert(t, "template_config_restored", assert.Equal(t, before.TemplateConf, after.TemplateConf))
			bindingMySQLAssert(t, "hook_config_restored", assert.Equal(t, before.LifecycleHookConf, after.LifecycleHookConf))
			bindingMySQLAssert(t, "binding_bytes_restored", assert.Equal(t, before.ScheduleRunBinding, after.ScheduleRunBinding))
			got, err := r.Get(ctx, key)
			bindingMySQLAssert(t, "rollback_binding_read", assert.NoError(t, err))
			if bound {
				bindingMySQLAssert(t, "original_binding_restored", assert.Equal(t, bindingMySQLInput(key), got))
			} else {
				bindingMySQLAssert(t, "unbound_null_restored", assert.Nil(t, got))
			}
		})
	}
}
