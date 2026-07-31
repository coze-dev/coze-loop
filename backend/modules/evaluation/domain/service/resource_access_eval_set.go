// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// earlyCheckVersionPolicy 在资源加载前对「显式请求的 version_id」做早拒（仅 specified 策略）。
//
//   - specified：显式传了 version_id 且不在 SpecifiedIDs 内 → 直接拒绝（无谓加载）。
//   - latest / all / 空：不早拒，交由加载后的 IsSharedVersionAllowed 逐版本过滤
//     （latest 需比对资源最新版本名，authorizer 不加载资源）。
func earlyCheckVersionPolicy(versionPolicy string, specifiedIDs []int64, versionID *int64) error {
	if versionPolicy != entity.SharedVersionPolicySpecified {
		return nil
	}
	if versionID == nil || *versionID == 0 {
		return nil
	}
	for _, id := range specifiedIDs {
		if id == *versionID {
			return nil
		}
	}
	return errorx.NewByCode(errno.CommonNoPermissionCode, errorx.WithExtraMsg("version not in shared_versions"))
}

func earlyCheckVersionNamePolicy(versionPolicy string, specifiedVersions []string, versionName *string) error {
	if versionPolicy != entity.SharedVersionPolicySpecified {
		return nil
	}
	if versionName == nil || *versionName == "" {
		return nil
	}
	for _, version := range specifiedVersions {
		if version == *versionName {
			return nil
		}
	}
	return errorx.NewByCode(errno.CommonNoPermissionCode, errorx.WithExtraMsg("version not in shared_versions"))
}

// IsSharedVersionAllowed 判断某版本在共享版本策略下是否可见（资源加载后逐版本过滤）。
//
//   - ""/all：全部可见
//   - specified：仅命中 specifiedIDs（按版本 ID）
//   - latest：仅当该版本名 == 资源当前最新版本名（latestVersionName 为空表示未知 → 拒绝，fail-closed）
//   - 其他未知策略：拒绝（fail-closed）
//
// versionName / latestVersionName 仅在 latest 策略下需要；其它策略传空亦可。
func IsSharedVersionAllowed(versionID int64, versionName, latestVersionName, versionPolicy string, specifiedIDs []int64) bool {
	switch versionPolicy {
	case "", entity.SharedVersionPolicyAll:
		return true
	case entity.SharedVersionPolicySpecified:
		for _, id := range specifiedIDs {
			if id == versionID {
				return true
			}
		}
		return false
	case entity.SharedVersionPolicyLatest:
		return latestVersionName != "" && versionName == latestVersionName
	default:
		return false
	}
}

func IsSharedVersionNameAllowed(versionName, latestVersionName, versionPolicy string, specifiedVersions []string) bool {
	switch versionPolicy {
	case "", entity.SharedVersionPolicyAll:
		return true
	case entity.SharedVersionPolicySpecified:
		for _, version := range specifiedVersions {
			if version == versionName {
				return true
			}
		}
		return false
	case entity.SharedVersionPolicyLatest:
		return latestVersionName != "" && versionName == latestVersionName
	default:
		return false
	}
}

// checkSharedEvalSetVersion 发起链路加载 tuple 后, 对「跨空间共享」的评测集做版本策略硬校验。
// 复用读接口 (evaluation_set_app.go) 同款 IsSharedVersionAllowed: 用加载出的真实版本 ID/版本名
// 与来源空间共享白名单/最新版本名比对; latest 用评测集的 LatestVersion 作最新版名。
//   - accessCtx nil / 非共享 (direct, VersionPolicy=all): 放行 (同空间发起不受共享版本策略约束)。
//   - set / 其 version 缺失: 放行 (无版本信息不改变既有行为, 交由硬校验/加载报错兜底)。
//   - 命中共享白名单外的版本: 返回 601 无权限, msg 说明版本不在共享范围。
func checkSharedEvalSetVersion(accessCtx *entity.ResourceAccessContext, set *entity.EvaluationSet) error {
	if accessCtx == nil || !accessCtx.IsShared() || set == nil || set.EvaluationSetVersion == nil {
		return nil
	}
	if IsSharedVersionAllowed(
		set.EvaluationSetVersion.ID,
		set.EvaluationSetVersion.Version,
		set.LatestVersion,
		accessCtx.VersionPolicy,
		accessCtx.SpecifiedIDs,
	) {
		return nil
	}
	return errorx.NewByCode(errno.CommonNoPermissionCode, errorx.WithExtraMsg("eval set version not in shared_versions"))
}

// checkSharedTargetVersion 发起链路加载 tuple 后, 对「跨空间共享」的评测对象做版本策略硬校验。
// 仅 LoopPrompt (含 *Online 基类) 参与版本策略; 其它类型在 Lookup 已被强制 VersionPolicy=all (恒放行, no-op)。
// 复用读接口 (eval_target_app.go isSharedEvalTargetVersionAllowed) 同款 IsSharedVersionNameAllowed,
// 按来源版本名 (SourceTargetVersion) 校验 specified 白名单 / all。
//   - accessCtx nil / 非共享: 放行。
//   - latest 策略: 服务层无法解析来源最新版本名 (需 source operator, 归属 app 层), 保持与既有发起链路一致
//     不在此拦截 (由 all/specified 兜底安全边界); 记录以便后续在 app 层补齐。
//   - 命中白名单外版本: 返回 601 无权限。
func checkSharedTargetVersion(accessCtx *entity.ResourceAccessContext, target *entity.EvalTarget) error {
	if accessCtx == nil || !accessCtx.IsShared() || target == nil || target.EvalTargetVersion == nil {
		return nil
	}
	// 非 LoopPrompt 基类恒 all, IsSharedVersionNameAllowed 直接返回 true, 无需额外分支。
	// latest 策略下 latestVersionName 传空 → IsSharedVersionNameAllowed 返回 false 会误拒合法发起,
	// 故 latest 场景显式放行 (服务层不具备来源最新版本解析能力, 保持既有行为不回归)。
	if accessCtx.VersionPolicy == entity.SharedVersionPolicyLatest {
		return nil
	}
	if IsSharedVersionNameAllowed(
		target.EvalTargetVersion.SourceTargetVersion,
		"",
		accessCtx.VersionPolicy,
		accessCtx.SpecifiedVersions,
	) {
		return nil
	}
	return errorx.NewByCode(errno.CommonNoPermissionCode, errorx.WithExtraMsg("eval target version not in shared_versions"))
}
