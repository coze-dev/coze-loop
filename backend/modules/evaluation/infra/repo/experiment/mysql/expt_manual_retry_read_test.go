// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	dbmock "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
)

func TestManualRetryPreparationReadsMaster(t *testing.T) {
	for _, kind := range []string{"item_run_log", "frozen_ref"} {
		t.Run(kind, func(t *testing.T) {
			conn, source, _ := newRunLogResolverTestDB(t)
			provider := dbmock.NewMockProvider(gomock.NewController(t))
			provider.EXPECT().NewSession(gomock.Any()).Return(conn)
			ctx := contexts.WithCtxWriteDB(context.Background())
			if kind == "item_run_log" {
				source.ExpectQuery("SELECT .* FROM `expt_item_result_run_log`").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
				_, err := NewExptItemResultDAO(provider).MGetItemRunLog(ctx, 7, 8, []int64{11}, 9)
				require.NoError(t, err)
			} else {
				source.ExpectQuery("SELECT .* FROM `expt_item_ref`").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
				_, err := NewExptItemRefDAO(provider).GetByExptIDAndItemID(ctx, 9, 7, 11)
				require.NoError(t, err)
			}
			require.NoError(t, source.ExpectationsWereMet())
		})
	}
}
