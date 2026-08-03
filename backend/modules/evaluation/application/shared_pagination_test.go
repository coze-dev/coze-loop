// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"encoding/base64"
	"strconv"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
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
	_, _, _, err = paginateShared(items, &pageSize, gptr.Of("***"))
	assert.Error(t, err)
}

func TestSharedPageTokenForPageNumber(t *testing.T) {
	token, err := sharedPageTokenForPageNumber(nil, nil)
	require.NoError(t, err)
	assert.Nil(t, token)

	pageOne := int32(1)
	token, err = sharedPageTokenForPageNumber(&pageOne, gptr.Of(int32(0)))
	require.NoError(t, err)
	assert.Nil(t, token)

	pageTwo := int32(2)
	token, err = sharedPageTokenForPageNumber(&pageTwo, nil)
	require.NoError(t, err)
	require.NotNil(t, token)
	assert.Equal(t, base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(defaultSharedPageSize))), *token)

	pageThree, pageSize := int32(3), int32(5)
	token, err = sharedPageTokenForPageNumber(&pageThree, &pageSize)
	require.NoError(t, err)
	require.NotNil(t, token)
	assert.Equal(t, "MTA", *token)

	for _, invalidSize := range []int32{0, maxSharedPageSize + 1} {
		_, err = sharedPageTokenForPageNumber(&pageTwo, &invalidSize)
		assert.Error(t, err)
	}
}

func TestPaginateSharedValidationAndBounds(t *testing.T) {
	items := make([]int, defaultSharedPageSize+1)
	page, token, hasMore, err := paginateShared(items, nil, gptr.Of("  "))
	require.NoError(t, err)
	assert.Len(t, page, defaultSharedPageSize)
	assert.True(t, hasMore)
	assert.NotNil(t, token)

	for _, invalidSize := range []int32{0, maxSharedPageSize + 1} {
		_, _, _, err = paginateShared(items, &invalidSize, nil)
		assert.Error(t, err)
	}

	nonNumeric := base64.RawURLEncoding.EncodeToString([]byte("not-a-number"))
	_, _, _, err = paginateShared(items, nil, &nonNumeric)
	assert.Error(t, err)

	negative := base64.RawURLEncoding.EncodeToString([]byte("-1"))
	_, _, _, err = paginateShared(items, nil, &negative)
	assert.Error(t, err)

	pastEnd := base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(len(items))))
	page, token, hasMore, err = paginateShared(items, nil, &pastEnd)
	require.NoError(t, err)
	assert.Empty(t, page)
	assert.Nil(t, token)
	assert.False(t, hasMore)
}

func TestNormalizeSharedAccessContexts(t *testing.T) {
	contexts := []*entity.ResourceAccessContext{
		nil,
		{ResourceSpaceID: 0, ResourceID: 1},
		{ResourceSpaceID: 2, ResourceID: 0},
		{ResourceSpaceID: 2, ResourceID: 20, AccessLevel: entity.SharedAccessLevelExecute},
		{ResourceSpaceID: 1, ResourceID: 30},
		{ResourceSpaceID: 1, ResourceID: 10},
		{ResourceSpaceID: 2, ResourceID: 20, AccessLevel: entity.SharedAccessLevelReadable},
	}

	got := normalizeSharedAccessContexts(contexts, []int64{10, 20})
	require.Len(t, got, 2)
	assert.Equal(t, int64(1), got[0].ResourceSpaceID)
	assert.Equal(t, int64(10), got[0].ResourceID)
	assert.Equal(t, int64(2), got[1].ResourceSpaceID)
	assert.Equal(t, int64(20), got[1].ResourceID)
	assert.Equal(t, entity.SharedAccessLevelReadable, got[1].AccessLevel)

	pageSize := int32(1)
	page, total, nextToken, hasMore, err := paginateSharedAccessContexts(contexts, nil, &pageSize, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, page, 1)
	assert.True(t, hasMore)
	assert.NotNil(t, nextToken)
}
