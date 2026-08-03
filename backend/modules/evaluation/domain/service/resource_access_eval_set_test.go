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

// checkSharedEvalSetVersion 加载后校验:
//   - 非共享 (direct ctx) / nil ctx / set 或 version 缺失 → 放行 (不误拒同空间发起)
//   - 跨空间 latest: 版本名 == LatestVersion 放行, 否则拒 (只共享 latest 却用历史版本被拦)
//   - 跨空间 specified: version ID 命中 SpecifiedIDs 放行, 否则拒
func TestCheckSharedEvalSetVersion(t *testing.T) {
	directCtx := &entity.ResourceAccessContext{
		CallerSpaceID: 1, ResourceSpaceID: 1, AccessMode: entity.AccessModeDirect,
		VersionPolicy: entity.SharedVersionPolicyAll,
	}
	sharedLatest := &entity.ResourceAccessContext{
		CallerSpaceID: 1, ResourceSpaceID: 2, AccessMode: entity.AccessModeShared,
		VersionPolicy: entity.SharedVersionPolicyLatest,
	}
	sharedSpecified := &entity.ResourceAccessContext{
		CallerSpaceID: 1, ResourceSpaceID: 2, AccessMode: entity.AccessModeShared,
		VersionPolicy: entity.SharedVersionPolicySpecified, SpecifiedIDs: []int64{10},
	}
	setV2Latest := &entity.EvaluationSet{
		LatestVersion:        "v3",
		EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 10, Version: "v2"},
	}
	setV3Latest := &entity.EvaluationSet{
		LatestVersion:        "v3",
		EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 11, Version: "v3"},
	}

	// 同空间/nil/缺失 → 放行
	assert.NoError(t, checkSharedEvalSetVersion(nil, setV2Latest))
	assert.NoError(t, checkSharedEvalSetVersion(directCtx, setV2Latest))
	assert.NoError(t, checkSharedEvalSetVersion(sharedLatest, nil))
	assert.NoError(t, checkSharedEvalSetVersion(sharedLatest, &entity.EvaluationSet{}))

	// 跨空间 latest: 用历史版本(v2!=v3)被拒, 用最新版本(v3)放行
	assert.Error(t, checkSharedEvalSetVersion(sharedLatest, setV2Latest))
	assert.NoError(t, checkSharedEvalSetVersion(sharedLatest, setV3Latest))

	// 跨空间 specified: 命中(ID=10)放行, 未命中(ID=11)拒
	assert.NoError(t, checkSharedEvalSetVersion(sharedSpecified, setV2Latest))
	assert.Error(t, checkSharedEvalSetVersion(sharedSpecified, setV3Latest))
}

// checkSharedTargetVersion 加载后校验 (LoopPrompt 才有版本策略, 其它类型恒 all):
//   - 非共享/nil/缺失 → 放行
//   - 跨空间 specified: SourceTargetVersion 命中 SpecifiedVersions 放行, 否则拒
//   - 跨空间 latest: 服务层无来源最新版本解析能力, 显式放行 (避免误拒合法发起)
func TestCheckSharedTargetVersion(t *testing.T) {
	sharedSpecified := &entity.ResourceAccessContext{
		CallerSpaceID: 1, ResourceSpaceID: 2, AccessMode: entity.AccessModeShared,
		VersionPolicy: entity.SharedVersionPolicySpecified, SpecifiedVersions: []string{"v1", "v2"},
	}
	sharedLatest := &entity.ResourceAccessContext{
		CallerSpaceID: 1, ResourceSpaceID: 2, AccessMode: entity.AccessModeShared,
		VersionPolicy: entity.SharedVersionPolicyLatest,
	}
	sharedAll := &entity.ResourceAccessContext{
		CallerSpaceID: 1, ResourceSpaceID: 2, AccessMode: entity.AccessModeShared,
		VersionPolicy: entity.SharedVersionPolicyAll,
	}
	targetV1 := &entity.EvalTarget{EvalTargetVersion: &entity.EvalTargetVersion{SourceTargetVersion: "v1"}}
	targetV9 := &entity.EvalTarget{EvalTargetVersion: &entity.EvalTargetVersion{SourceTargetVersion: "v9"}}

	assert.NoError(t, checkSharedTargetVersion(nil, targetV1))
	assert.NoError(t, checkSharedTargetVersion(sharedSpecified, nil))
	assert.NoError(t, checkSharedTargetVersion(sharedSpecified, &entity.EvalTarget{}))

	// specified: 命中放行, 未命中拒
	assert.NoError(t, checkSharedTargetVersion(sharedSpecified, targetV1))
	assert.Error(t, checkSharedTargetVersion(sharedSpecified, targetV9))

	// latest 显式放行; all 放行
	assert.NoError(t, checkSharedTargetVersion(sharedLatest, targetV9))
	assert.NoError(t, checkSharedTargetVersion(sharedAll, targetV9))
}
