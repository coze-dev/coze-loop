// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	driver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func hookColumnsTestDB(t *testing.T) db.Provider {
	t.Helper()
	dsn := os.Getenv("HOOK_MYSQL_TEST_DSN")
	if dsn == "" {
		return db.NewTestDB(t, &model.Experiment{}, &model.ExptTemplate{}, &model.ExptRunLog{})
	}
	cfg, err := driver.ParseDSN(dsn)
	require.NoError(t, err)
	require.Equal(t, "unix", cfg.Net, "integration tests require an isolated Unix socket")
	require.Equal(t, "hook_7378265404_schema", cfg.DBName, "refuse any other database")
	p, err := db.NewDB(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := p.NewSession(context.Background()).DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	return p
}

func TestHookLegacyWritesPreserveProtectedColumns(t *testing.T) {
	p := hookColumnsTestDB(t)
	ctx := context.Background()
	for _, tc := range []struct {
		name, table, columns string
		write                func(int64, bool) error
	}{
		{"experiment struct", "experiment", "lifecycle_hook_conf", func(id int64, zero bool) error {
			value := []byte("replacement")
			if zero {
				value = []byte{}
			}
			return NewExptDAO(p).Update(ctx, &model.Experiment{ID: id, Name: "changed", LifecycleHookConf: &value})
		}},
		{"experiment map", "experiment", "lifecycle_hook_conf", func(id int64, zero bool) error {
			var value any = []byte("replacement")
			if zero {
				value = nil
			}
			return NewExptDAO(p).UpdateFields(ctx, id, map[string]any{"name": "changed", "lifecycle_hook_conf": value})
		}},
		{"template struct", "expt_template", "lifecycle_hook_conf,schedule_run_binding", func(id int64, zero bool) error {
			value := []byte("replacement")
			if zero {
				value = []byte{}
			}
			return NewExptTemplateDAO(p).Update(ctx, &model.ExptTemplate{ID: id, Name: "changed", LifecycleHookConf: &value, ScheduleRunBinding: &value})
		}},
		{"template map", "expt_template", "lifecycle_hook_conf,schedule_run_binding", func(id int64, zero bool) error {
			var value any = []byte("replacement")
			if zero {
				value = nil
			}
			return NewExptTemplateDAO(p).UpdateFields(ctx, id, map[string]any{"name": "changed", "lifecycle_hook_conf": value, "schedule_run_binding": value})
		}},
		{"run log save", "expt_run_log", "lifecycle_hook_version", func(id int64, zero bool) error {
			dao := NewExptRunLogDAO(p)
			row, err := dao.Get(ctx, id, id)
			if err != nil {
				return err
			}
			row.SuccessCnt = 7
			row.LifecycleHookVersion = gptr.Of(int32(9))
			if zero {
				row.LifecycleHookVersion = nil
			}
			return dao.Save(ctx, row)
		}},
		{"run log map", "expt_run_log", "lifecycle_hook_version", func(id int64, zero bool) error {
			var marker any = 9
			if zero {
				marker = nil
			}
			return NewExptRunLogDAO(p).Update(ctx, id, id, map[string]any{"success_cnt": 7, "lifecycle_hook_version": marker})
		}},
	} {
		for _, zero := range []bool{false, true} {
			name := tc.name + "/replacement"
			if zero {
				name = tc.name + "/empty_or_nil"
			}
			t.Run(name, func(t *testing.T) {
				id := time.Now().UnixNano()
				session := p.NewSession(ctx)
				t.Cleanup(func() { require.NoError(t, session.Exec("DELETE FROM "+tc.table+" WHERE id = ?", id).Error) })
				switch tc.table {
				case "experiment":
					require.NoError(t, session.Create(&model.Experiment{ID: id, SpaceID: 1, Name: "before", LifecycleHookConf: gptr.Of([]byte("frozen-config"))}).Error)
				case "expt_template":
					require.NoError(t, session.Create(&model.ExptTemplate{ID: id, SpaceID: 1, Name: "before", LifecycleHookConf: gptr.Of([]byte("frozen-config")), ScheduleRunBinding: gptr.Of([]byte("frozen-binding"))}).Error)
				case "expt_run_log":
					require.NoError(t, session.Create(&model.ExptRunLog{ID: id, SpaceID: 1, ExptID: id, ExptRunID: id, LifecycleHookVersion: gptr.Of(int32(1))}).Error)
				}
				require.NoError(t, tc.write(id, zero))
				if tc.table == "expt_run_log" {
					var row struct {
						Marker     *int32
						SuccessCnt int32
					}
					require.NoError(t, session.Raw("SELECT lifecycle_hook_version AS marker,success_cnt FROM expt_run_log WHERE id = ?", id).Scan(&row).Error)
					require.Equal(t, gptr.Of(int32(1)), row.Marker)
					require.Equal(t, int32(7), row.SuccessCnt)
				} else {
					var row struct {
						Name               string
						LifecycleHookConf  []byte
						ScheduleRunBinding []byte
					}
					require.NoError(t, session.Raw("SELECT name,"+tc.columns+" FROM "+tc.table+" WHERE id = ?", id).Scan(&row).Error)
					require.Equal(t, "changed", row.Name)
					require.Equal(t, []byte("frozen-config"), row.LifecycleHookConf)
					if tc.table == "expt_template" {
						require.Equal(t, []byte("frozen-binding"), row.ScheduleRunBinding)
					}
				}
			})
		}
	}
}

func TestHookProtectedPOFieldsAreNotJSONExposed(t *testing.T) {
	for _, po := range []any{
		&model.Experiment{ID: 1, LifecycleHookConf: gptr.Of([]byte("private-config"))},
		&model.ExptTemplate{ID: 1, LifecycleHookConf: gptr.Of([]byte("private-config")), ScheduleRunBinding: gptr.Of([]byte("private-identity"))},
		&model.ExptRunLog{ID: 1, LifecycleHookVersion: gptr.Of(int32(1))},
		&model.ExptLifecycleRun{SnapshotCipher: []byte("private-snapshot")},
	} {
		body, err := json.Marshal(po)
		require.NoError(t, err)
		var fields map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(body, &fields))
		for _, key := range []string{"lifecycle_hook_conf", "schedule_run_binding", "lifecycle_hook_version", "snapshot_cipher"} {
			require.NotContains(t, fields, key)
		}
	}
}
