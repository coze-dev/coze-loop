// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	evaluatorservice "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/evaluator"
)

func TestBuildSrvListEvaluatorRequestDescription(t *testing.T) {
	var req evaluatorservice.ListEvaluatorsRequest
	require.NoError(t, json.Unmarshal([]byte(`{"workspace_id":10,"search_name":"alpha","search_description":"CeShI","page_size":20,"page_number":2}`), &req))
	result := buildSrvListEvaluatorRequest(&req)
	require.Equal(t, "CeShI", result.SearchDescription)
	require.Equal(t, "alpha", result.SearchName)
	require.Equal(t, int64(10), result.SpaceID)
	require.Equal(t, int32(20), result.PageSize)
	require.Equal(t, int32(2), result.PageNum)
}
