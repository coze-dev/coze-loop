// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package target

import (
	"context"
	"testing"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

func TestExecutionContextRecordLookupExcludesDeletedRecords(t *testing.T) {
	provider := db.NewTestDB(t, &model.TargetRecord{})
	database := provider.NewSession(context.Background())
	require.NoError(t, database.Create(&model.TargetRecord{ID: 31, SpaceID: 1, ExperimentRunID: 21}).Error)
	repository := &EvalTargetRepoImpl{evalTargetRecordDao: mysql.NewEvalTargetRecordDAO(provider)}
	got, err := repository.GetEvalTargetRecordByIDAndSpaceID(context.Background(), 1, 31)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, int64(21), got.ExperimentRunID)
	wrong, err := repository.GetEvalTargetRecordByIDAndSpaceID(context.Background(), 9, 31)
	require.NoError(t, err)
	require.Nil(t, wrong)
	require.NoError(t, database.Delete(&model.TargetRecord{ID: 31}).Error)
	got, err = repository.GetEvalTargetRecordByIDAndSpaceID(context.Background(), 1, 31)
	require.NoError(t, err)
	require.Nil(t, got)
}
