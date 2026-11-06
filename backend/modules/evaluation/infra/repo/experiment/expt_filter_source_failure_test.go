// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	dbmocks "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func newFilterScanFailureFixture(t *testing.T) (repo.IExptFilterSourceScanner, sqlmock.Sqlmock, *int) {
	t.Helper()
	conn, sql, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	orm, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)
	provider := dbmocks.NewMockProvider(gomock.NewController(t))
	sessions := 0
	provider.EXPECT().NewSession(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ ...db.Option) *gorm.DB {
		sessions++
		return orm.WithContext(ctx)
	}).AnyTimes()
	return NewExptFilterSourceScanner(provider), sql, &sessions
}

func TestFilterSourceErrorsDoNotAdvanceCursor(t *testing.T) {
	for _, stage := range []string{"experiments", "upper_bound", "turns"} {
		t.Run(stage, func(t *testing.T) {
			scanner, sql, _ := newFilterScanFailureFixture(t)
			fault := errors.New("source unavailable")
			scope := entity.ExptFilterScope{SpaceID: 1, ExptID: 2}
			switch stage {
			case "experiments":
				sql.ExpectQuery("SELECT .* FROM `experiment`").WithArgs(5, 20, 2).WillReturnError(fault)
				page, err := scanner.ScanExperiments(context.Background(), entity.ExptFilterScanBounds{MaxExptID: 20}, 5, 2)
				require.ErrorIs(t, err, fault)
				require.Empty(t, page.Scopes)
				require.Zero(t, page.NextExptID)
			case "upper_bound":
				sql.ExpectQuery("SELECT COALESCE").WithArgs(1, 2).WillReturnError(fault)
				upper, err := scanner.TurnUpperBound(context.Background(), scope)
				require.ErrorIs(t, err, fault)
				require.Zero(t, upper)
			case "turns":
				sql.ExpectQuery("SELECT .* FROM `expt_turn_result`").WithArgs(1, 2, 5, 20, 2).WillReturnError(fault)
				rows, next, err := scanner.ScanTurns(context.Background(), scope, 5, 20, 2)
				require.ErrorIs(t, err, fault)
				require.Nil(t, rows)
				require.Zero(t, next)
			}
			require.NoError(t, sql.ExpectationsWereMet())
		})
	}
}

func TestFilterSourceRejectsMalformedPagesAndScope(t *testing.T) {
	t.Run("out_of_order_experiment_discards_prefix", func(t *testing.T) {
		scanner, sql, _ := newFilterScanFailureFixture(t)
		sql.ExpectQuery("SELECT .* FROM `experiment`").WithArgs(5, 20, 2).WillReturnRows(sqlmock.NewRows([]string{"id", "space_id"}).AddRow(6, 1).AddRow(4, 1))
		page, err := scanner.ScanExperiments(context.Background(), entity.ExptFilterScanBounds{MaxExptID: 20}, 5, 2)
		require.Error(t, err)
		require.Empty(t, page.Scopes)
		require.Zero(t, page.NextExptID)
		require.NoError(t, sql.ExpectationsWereMet())
	})
	t.Run("foreign_turn_discards_prefix", func(t *testing.T) {
		scanner, sql, _ := newFilterScanFailureFixture(t)
		sql.ExpectQuery("SELECT .* FROM `expt_turn_result`").WithArgs(1, 2, 5, 20, 2).WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "expt_id", "item_id", "turn_id"}).AddRow(6, 1, 2, 3, 0).AddRow(7, 99, 2, 4, 0))
		rows, next, err := scanner.ScanTurns(context.Background(), entity.ExptFilterScope{SpaceID: 1, ExptID: 2}, 5, 20, 2)
		require.EqualError(t, err, "source_unknown: invalid turn cursor or ownership")
		require.Nil(t, rows)
		require.Zero(t, next)
		require.NoError(t, sql.ExpectationsWereMet())
	})
	t.Run("invalid_bounds_or_scope_never_query", func(t *testing.T) {
		scanner, sql, sessions := newFilterScanFailureFixture(t)
		page, err := scanner.ScanExperiments(context.Background(), entity.ExptFilterScanBounds{MaxExptID: 5}, 6, 1)
		require.ErrorContains(t, err, "invalid filter scan bounds")
		require.Empty(t, page.Scopes)
		_, err = scanner.TurnUpperBound(context.Background(), entity.ExptFilterScope{})
		require.ErrorContains(t, err, "invalid filter scan scope")
		_, next, err := scanner.ScanTurns(context.Background(), entity.ExptFilterScope{}, 0, 5, 1)
		require.ErrorContains(t, err, "invalid filter scan scope")
		require.Zero(t, next)
		require.Zero(t, *sessions, "invalid input must fail before opening any SQL session")
		require.NoError(t, sql.ExpectationsWereMet())
	})
}
