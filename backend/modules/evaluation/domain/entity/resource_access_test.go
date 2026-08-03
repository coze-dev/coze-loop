// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResourceAccessContext_IsSharedAndQuerySpaceID(t *testing.T) {
	var nilCtx *ResourceAccessContext
	assert.False(t, nilCtx.IsShared())
	assert.Zero(t, nilCtx.QuerySpaceID())

	direct := &ResourceAccessContext{
		CallerSpaceID:   100,
		ResourceSpaceID: 100,
		AccessMode:      AccessModeDirect,
	}
	assert.False(t, direct.IsShared())
	assert.Equal(t, int64(100), direct.QuerySpaceID())

	invalidShared := &ResourceAccessContext{
		CallerSpaceID:   100,
		ResourceSpaceID: 0,
		AccessMode:      AccessModeShared,
	}
	assert.False(t, invalidShared.IsShared())
	assert.Equal(t, int64(100), invalidShared.QuerySpaceID())

	shared := &ResourceAccessContext{
		CallerSpaceID:   100,
		ResourceSpaceID: 200,
		AccessMode:      AccessModeShared,
	}
	assert.True(t, shared.IsShared())
	assert.Equal(t, int64(200), shared.QuerySpaceID())
}

func TestResourceAccessContext_SharedInfo(t *testing.T) {
	direct := &ResourceAccessContext{
		CallerSpaceID:   100,
		ResourceSpaceID: 100,
		AccessMode:      AccessModeDirect,
	}
	assert.Nil(t, direct.SharedInfo())

	shared := &ResourceAccessContext{
		CallerSpaceID:   100,
		ResourceSpaceID: 200,
		AccessMode:      AccessModeShared,
		AccessLevel:     SharedAccessLevelReadable,
		VersionPolicy:   SharedVersionPolicySpecified,
	}
	info := shared.SharedInfo()
	require.NotNil(t, info)
	assert.True(t, info.IsShared)
	assert.Equal(t, int64(200), info.SourceSpaceID)
	assert.Equal(t, SharedAccessLevelReadable, info.AccessLevel)
	assert.Equal(t, SharedVersionPolicySpecified, info.VersionPolicy)
}
