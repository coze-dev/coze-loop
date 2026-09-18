// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"testing"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

func TestRunContextLookupExcludesDeletedRuns(t *testing.T) {
	provider := db.NewTestDB(t, &model.ExptRunLog{})
	database := provider.NewSession(context.Background())
	require.NoError(t, database.Create(&model.ExptRunLog{ID: 21, ExptRunID: 21, ExptID: 10, SpaceID: 2, CreatedBy: "run-owner"}).Error)
	repository := NewExptRunLogRepo(mysql.NewExptRunLogDAO(provider))
	got, err := repository.GetByRunID(context.Background(), 21)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "run-owner", got.CreatedBy)
	require.NoError(t, database.Delete(&model.ExptRunLog{ID: 21}).Error)
	got, err = repository.GetByRunID(context.Background(), 21)
	require.NoError(t, err)
	require.Nil(t, got)
	got, err = repository.GetByRunID(context.Background(), 99)
	require.NoError(t, err)
	require.Nil(t, got)
}
