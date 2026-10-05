// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestHookSummarySchemaUpgradePreservesOldRawRows(t *testing.T) {
	f := newHookTxFixture(t)
	docker := "../../../../../../release/deployment/docker-compose/bootstrap/mysql-init/"
	helm := "../../../../../../release/deployment/helm-chart/charts/app/bootstrap/init/mysql/"
	initSQL, err := os.ReadFile(docker + "init-sql/expt_lifecycle_hook_run.sql")
	require.NoError(t, err)
	patchSQL, err := os.ReadFile(docker + "patch-sql/expt_lifecycle_hook_alter.sql")
	require.NoError(t, err)
	for _, path := range []string{"init-sql/expt_lifecycle_hook_run.sql", "patch-sql/expt_lifecycle_hook_alter.sql"} {
		a, err := os.ReadFile(docker + path)
		require.NoError(t, err)
		b, err := os.ReadFile(helm + path)
		require.NoError(t, err)
		require.Equal(t, a, b)
	}
	// Temporary, connection-local tables exercise old insert syntax and both DDL
	// paths; never recreate or truncate a shared fixture table.
	require.NoError(t, f.sql.Connection(func(conn *gorm.DB) error {
		conn = conn.Session(&gorm.Session{NewDB: true})
		oldName := fmt.Sprint("hook_summary_old_", f.expt)
		newName := fmt.Sprint("hook_summary_new_", f.expt)
		defer conn.Exec("DROP TEMPORARY TABLE IF EXISTS " + oldName + "," + newName)
		rename := func(raw, name string) string {
			return strings.ReplaceAll(strings.ReplaceAll(raw, "CREATE TABLE IF NOT EXISTS", "CREATE TEMPORARY TABLE"), "`expt_lifecycle_hook_run`", "`"+name+"`")
		}
		var oldLines []string
		for _, line := range strings.Split(string(initSQL), "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "`updated_at`") {
				oldLines = append(oldLines, line)
			}
		}
		require.NoError(t, conn.Exec(rename(strings.Join(oldLines, "\n"), oldName)).Error)
		require.NoError(t, conn.Exec(rename(string(initSQL), newName)).Error)
		insert := func(name string, id int) {
			require.NoError(t, conn.Exec("INSERT INTO "+name+" (id,space_id,expt_id,expt_run_id,phase,operation_id,idempotency_key,status,execution_scope) VALUES (?,1,2,?,'after',?,?,'pending','test')", id, id, fmt.Sprint(id), fmt.Sprint("key", id)).Error)
		}
		insert(oldName, 1)
		var statement string
		for _, s := range strings.Split(string(patchSQL), ";") {
			if strings.HasPrefix(strings.TrimSpace(s), "ALTER TABLE `expt_lifecycle_hook_run`") {
				statement = rename(s, oldName)
			}
		}
		require.NotEmpty(t, statement)
		columns := func(name string) []map[string]any {
			var rows []map[string]any
			require.NoError(t, conn.Raw("SHOW COLUMNS FROM "+name).Scan(&rows).Error)
			return rows
		}
		for i := 0; i < 2; i++ {
			found := false
			for _, column := range columns(oldName) {
				if fmt.Sprint(column["Field"]) == "updated_at" {
					found = true
				}
			}
			if !found {
				require.NoError(t, conn.Exec(statement).Error)
			}
		}
		require.Equal(t, columns(newName), columns(oldName))
		insert(oldName, 2)
		insert(newName, 1)
		var rows []struct {
			ID        int64
			Status    string
			UpdatedAt time.Time
		}
		require.NoError(t, conn.Table(oldName).Select("id,status,updated_at").Order("id").Find(&rows).Error)
		require.Len(t, rows, 2)
		require.Equal(t, int64(1), rows[0].ID)
		for _, row := range rows {
			require.Equal(t, "pending", row.Status)
			require.False(t, row.UpdatedAt.IsZero())
		}
		return nil
	}))
}
