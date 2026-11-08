// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	dbmock "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
)

func TestExptRunLogDAOImpl_GetWithWriteContextUsesPrimary(t *testing.T) {
	for _, primary := range []bool{false, true} {
		name := "replica_control"
		if primary {
			name = "primary"
		}
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			gormDB, sourceMock, replicaMock := newRunLogResolverTestDB(t)
			provider := dbmock.NewMockProvider(ctrl)
			provider.EXPECT().NewSession(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, opts ...db.Option) *gorm.DB {
				if db.ContainWithMasterOpt(opts) {
					return gormDB.Clauses(dbresolver.Write)
				}
				return gormDB
			}).AnyTimes()
			chosen := replicaMock
			ctx := context.Background()
			if primary {
				chosen = sourceMock
				ctx = contexts.WithCtxWriteDB(ctx)
			}
			chosen.ExpectQuery("SELECT .* FROM `expt_run_log`").WillReturnRows(sqlmock.NewRows([]string{"id", "expt_id", "expt_run_id", "status"}).AddRow(int64(7), int64(2), int64(7), int64(3)))
			row, err := NewExptRunLogDAO(provider).Get(ctx, 2, 7)
			require.NoError(t, err, "primary context must not query a stale replica")
			require.Equal(t, int64(7), row.ID)
			require.NoError(t, sourceMock.ExpectationsWereMet())
			require.NoError(t, replicaMock.ExpectationsWereMet())
		})
	}
}
