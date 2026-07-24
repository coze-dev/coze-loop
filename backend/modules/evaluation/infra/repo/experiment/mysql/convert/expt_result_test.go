// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func TestExptItemResultConvertor_PO2DO_UpdatedAt(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(2026, time.July, 24, 16, 17, 18, 987654000, time.UTC)
	itemIdx := int32(9)
	errMsg := []byte("target execution failed")
	po := &model.ExptItemResult{
		ID:            101,
		SpaceID:       202,
		ExptID:        303,
		ExptRunID:     404,
		ItemID:        505,
		ItemVersionID: 606,
		ItemIdx:       &itemIdx,
		Status:        int32(entity.ItemRunState_Fail),
		ErrMsg:        &errMsg,
		LogID:         "log-707",
		UpdatedAt:     updatedAt,
	}

	got := NewExptItemResultConvertor().PO2DO(po)
	require.NotNil(t, got)
	require.Equal(t, int64(101), got.ID)
	require.Equal(t, int64(202), got.SpaceID)
	require.Equal(t, int64(303), got.ExptID)
	require.Equal(t, int64(404), got.ExptRunID)
	require.Equal(t, int64(505), got.ItemID)
	require.Equal(t, int64(606), got.ItemVersionID)
	require.Equal(t, int32(9), got.ItemIdx)
	require.Equal(t, entity.ItemRunState_Fail, got.Status)
	require.Equal(t, "target execution failed", got.ErrMsg)
	require.Equal(t, "log-707", got.LogID)
	require.NotNil(t, got.UpdatedAt)
	require.Equal(t, updatedAt, *got.UpdatedAt)
}
