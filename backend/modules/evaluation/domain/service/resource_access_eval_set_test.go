// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// version_policy=specified:命中 specifiedIDs 放行,未命中拒绝。
func TestIsSharedVersionAllowed_Specified(t *testing.T) {
	assert.True(t, IsSharedVersionAllowed(5, "", "", entity.SharedVersionPolicySpecified, []int64{3, 5, 7}))
	assert.False(t, IsSharedVersionAllowed(6, "", "", entity.SharedVersionPolicySpecified, []int64{3, 5, 7}))
}

// version_policy=all / 空:全部放行。
func TestIsSharedVersionAllowed_All(t *testing.T) {
	assert.True(t, IsSharedVersionAllowed(999, "", "", entity.SharedVersionPolicyAll, nil))
	assert.True(t, IsSharedVersionAllowed(999, "", "", "", nil))
}

// version_policy=latest:仅版本名 == 最新版本名放行;latest 未知则 fail-closed。
func TestIsSharedVersionAllowed_Latest(t *testing.T) {
	assert.True(t, IsSharedVersionAllowed(1, "v3", "v3", entity.SharedVersionPolicyLatest, nil))
	assert.False(t, IsSharedVersionAllowed(1, "v2", "v3", entity.SharedVersionPolicyLatest, nil))
	assert.False(t, IsSharedVersionAllowed(1, "v3", "", entity.SharedVersionPolicyLatest, nil))
}

// 未知策略 → fail-closed。
func TestIsSharedVersionAllowed_UnknownPolicy_Deny(t *testing.T) {
	assert.False(t, IsSharedVersionAllowed(1, "v1", "v1", "weird", nil))
}

// earlyCheckVersionPolicy:specified 策略下显式 version_id 不在白名单 → 早拒;其它策略/空放行。
func TestEarlyCheckVersionPolicy(t *testing.T) {
	vid := int64(6)
	assert.Error(t, earlyCheckVersionPolicy(entity.SharedVersionPolicySpecified, []int64{3, 5, 7}, &vid))

	vidOK := int64(5)
	assert.NoError(t, earlyCheckVersionPolicy(entity.SharedVersionPolicySpecified, []int64{3, 5, 7}, &vidOK))

	// 非 specified 策略不早拒
	assert.NoError(t, earlyCheckVersionPolicy(entity.SharedVersionPolicyLatest, nil, &vid))
	assert.NoError(t, earlyCheckVersionPolicy(entity.SharedVersionPolicyAll, nil, &vid))

	// version_id 为空不早拒
	assert.NoError(t, earlyCheckVersionPolicy(entity.SharedVersionPolicySpecified, []int64{3}, nil))
}

func TestSharedVersionNamePolicy(t *testing.T) {
	versions := []string{"v1", "v2"}
	assert.True(t, IsSharedVersionNameAllowed("v2", "", entity.SharedVersionPolicySpecified, versions))
	assert.False(t, IsSharedVersionNameAllowed("v3", "", entity.SharedVersionPolicySpecified, versions))
	assert.True(t, IsSharedVersionNameAllowed("v3", "v3", entity.SharedVersionPolicyLatest, nil))
	assert.False(t, IsSharedVersionNameAllowed("v2", "v3", entity.SharedVersionPolicyLatest, nil))

	version := "v2"
	assert.NoError(t, earlyCheckVersionNamePolicy(entity.SharedVersionPolicySpecified, versions, &version))
	version = "v3"
	assert.Error(t, earlyCheckVersionNamePolicy(entity.SharedVersionPolicySpecified, versions, &version))
	assert.NoError(t, earlyCheckVersionNamePolicy(entity.SharedVersionPolicySpecified, versions, nil))
}
