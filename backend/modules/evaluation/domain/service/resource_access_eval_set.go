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
