// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	dbmocks "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestFilterSourceScannerUsesBoundedCurrentRows(t *testing.T) {
	conn, sql, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	g, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)
	provider := dbmocks.NewMockProvider(gomock.NewController(t))
	provider.EXPECT().NewSession(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ ...db.Option) *gorm.DB { return g.WithContext(ctx) }).AnyTimes()
	scanner := NewExptFilterSourceScanner(provider)
	sql.ExpectQuery("SELECT COALESCE\\(MAX\\(id\\), 0\\) FROM `experiment` WHERE `experiment`.`deleted_at` IS NULL").WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(20))
	bounds, err := scanner.BeginScan(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(20), bounds.MaxExptID)
	sql.ExpectQuery("SELECT .* FROM `experiment` WHERE \\(id > \\? AND id <= \\?\\) AND `experiment`.`deleted_at` IS NULL ORDER BY id ASC LIMIT \\?").WithArgs(5, 20, 2).WillReturnRows(sqlmock.NewRows([]string{"id", "space_id"}).AddRow(8, 1).AddRow(15, 2))
	page, err := scanner.ScanExperiments(context.Background(), bounds, 5, 2)
	require.NoError(t, err)
	require.Equal(t, []entity.ExptFilterScope{{SpaceID: 1, ExptID: 8}, {SpaceID: 2, ExptID: 15}}, page.Scopes)
	require.Equal(t, int64(15), page.NextExptID)
	scope := entity.ExptFilterScope{SpaceID: 1, ExptID: 8}
	sql.ExpectQuery("SELECT COALESCE\\(MAX\\(id\\), 0\\) FROM `expt_turn_result` WHERE \\(space_id = \\? AND expt_id = \\?\\) AND `expt_turn_result`.`deleted_at` IS NULL").WithArgs(1, 8).WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(50))
	upper, err := scanner.TurnUpperBound(context.Background(), scope)
	require.NoError(t, err)
	require.Equal(t, int64(50), upper)
	sql.ExpectQuery("SELECT \\* FROM `expt_turn_result` WHERE \\(space_id = \\? AND expt_id = \\? AND id > \\? AND id <= \\?\\) AND `expt_turn_result`.`deleted_at` IS NULL ORDER BY id ASC LIMIT \\?").WithArgs(1, 8, 10, 50, 2).WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "expt_id", "item_id", "turn_id", "status"}).AddRow(11, 1, 8, 4, 0, 0).AddRow(49, 1, 8, 5, 1, 2))
	turns, next, err := scanner.ScanTurns(context.Background(), scope, 10, upper, 2)
	require.NoError(t, err)
	require.Len(t, turns, 2)
	require.Equal(t, int64(49), next)
	_, _, err = scanner.ScanTurns(context.Background(), scope, 10, upper, 0)
	require.Error(t, err)
	require.NoError(t, sql.ExpectationsWereMet())
}
