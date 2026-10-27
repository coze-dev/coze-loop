// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestBuildListEvaluatorRequestDescription(t *testing.T) {
	req, err := buildListEvaluatorRequest(context.Background(), &entity.ListEvaluatorRequest{SpaceID: 10, SearchDescription: "CeShI", SearchName: "alpha", PageSize: 20, PageNum: 2})
	require.NoError(t, err)
	require.Equal(t, "CeShI", req.SearchDescription)
	require.Equal(t, "alpha", req.SearchName)
	require.Equal(t, int32(20), req.PageSize)
	require.Equal(t, int32(2), req.PageNum)
}
