// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaginateShared(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	pageSize := int32(2)

	firstPage, nextToken, hasMore, err := paginateShared(items, &pageSize, nil)
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2}, firstPage)
	assert.True(t, hasMore)
	require.NotNil(t, nextToken)
	assert.Equal(t, "Mg", *nextToken)

	secondPage, nextToken, hasMore, err := paginateShared(items, &pageSize, nextToken)
	require.NoError(t, err)
	assert.Equal(t, []int{3, 4}, secondPage)
	assert.True(t, hasMore)
	require.NotNil(t, nextToken)
	assert.Equal(t, "NA", *nextToken)

	lastPage, nextToken, hasMore, err := paginateShared(items, &pageSize, nextToken)
	require.NoError(t, err)
	assert.Equal(t, []int{5}, lastPage)
	assert.False(t, hasMore)
	assert.Nil(t, nextToken)

	_, _, _, err = paginateShared(items, &pageSize, gptr.Of("invalid"))
	assert.Error(t, err)
}
