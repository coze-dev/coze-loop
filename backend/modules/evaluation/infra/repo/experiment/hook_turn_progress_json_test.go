// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"os"
	"testing"

	driver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestHookTurnResultJSONDriverEncodingMySQL(t *testing.T) {
	if os.Getenv("HOOK_MYSQL_TX_DSN") == "" {
		t.Skip("requires existing isolated MySQL fixture")
	}
	cfg, err := driver.ParseDSN(os.Getenv("HOOK_MYSQL_TX_DSN"))
	require.NoError(t, err)
	cfg.InterpolateParams = true
	t.Setenv("HOOK_MYSQL_TX_DSN", cfg.FormatDSN())
	f := newHookProgressFixture(t)
	base := f.read(t)
	next := *base
	next.Status = entity.TurnRunState_Success
	next.Ext = map[string]string{"keep": "original", "value": "中文 'quoted' \"JSON\""}
	r := NewHookTurnProgressRepo(f.p, func(context.Context) (string, error) { return f.scope, nil }).(*hookTurnProgressRepo)
	_, err = r.WriteTurnResult(context.Background(), entity.HookTurnProgressInput{Base: base, Progress: &next})
	require.NoError(t, err)
	got := f.read(t)
	require.Equal(t, entity.TurnRunState_Success, got.Status)
	require.Equal(t, next.Ext, got.Ext)
}
