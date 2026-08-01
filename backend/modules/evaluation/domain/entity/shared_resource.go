// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "strconv"

const (
	SharedVersionPolicyLatest    = "latest"
	SharedVersionPolicySpecified = "specified"
	SharedVersionPolicyAll       = "all"
)

const (
	// SharedResourceTypeEvalSet 评测集共享资源类型
	SharedResourceTypeEvalSet = "eval_set"
	// SharedResourceTypeEvalTarget 评测对象共享资源类型
	SharedResourceTypeEvalTarget = "eval_target"
)

const (
	// SharedAccessLevelReadable 可读：允许查看资源内容（如评测集 item），受版本策略约束
	SharedAccessLevelReadable = "readable"
	// SharedAccessLevelExecute 可执行：黑盒引用，不允许查看资源内容
	SharedAccessLevelExecute = "execute"
)

// sharedAccessTargetAll 表示共享给所有空间的通配符，readable 和 execute 均支持。
const sharedAccessTargetAll = "*"

type SharedResourceInfo struct {
	IsShared      bool
	SourceSpaceID int64
	AccessLevel   string
	VersionPolicy string
}

type SharedResourceOption struct {
	IsShared      bool
	SourceSpaceID *int64
}

// AccessMode 标记读取模式：direct（本空间直读）或 shared（跨空间共享读）
type AccessMode string

const (
	AccessModeDirect AccessMode = "direct"
	AccessModeShared AccessMode = "shared"
)

func (o *SharedResourceOption) Enabled() bool {
	return o != nil && o.IsShared && o.SourceSpaceID != nil && *o.SourceSpaceID > 0
}

// BuildSharedResourceInfo 构建回填给调用方的共享元信息（供 fill 流程使用）。
// consumerWorkspaceID == sourceSpaceID 时视为非共享，返回 nil。
func BuildSharedResourceInfo(consumerWorkspaceID, sourceSpaceID int64, accessLevel, versionPolicy string) *SharedResourceInfo {
	if sourceSpaceID <= 0 || sourceSpaceID == consumerWorkspaceID {
		return nil
	}
	return &SharedResourceInfo{
		IsShared:      true,
		SourceSpaceID: sourceSpaceID,
		AccessLevel:   accessLevel,
		VersionPolicy: versionPolicy,
	}
}

// SharedResourceConfig 跨空间共享资源配置的标准模型（与底层配置存储解耦）。
// key = 来源空间 id（资源真正归属的空间 B）。
type SharedResourceConfig struct {
	SpaceRules map[int64]*SpaceSharedRules
}

// SpaceSharedRules 单个来源空间对外共享的资源规则集合。
type SpaceSharedRules struct {
	Resources []*SharedResourceRule
}

// SharedResourceRule 单个资源的共享规则。
type SharedResourceRule struct {
	ResourceID        int64
	ResourceType      string // eval_set / eval_target
	TargetType        EvalTargetType
	VersionPolicy     string // latest / all / specified
	SpecifiedIDs      []int64
	SpecifiedVersions []string
	AccessRules       []*SharedAccessRule
}

// SharedAccessRule 资源的一条访问规则：把某访问级别授予一组目标空间。
type SharedAccessRule struct {
	AccessLevel string   // readable / execute
	Targets     []string // "*"（readable 和 execute 均允许）或调用方空间 id 字符串
}

// ResolvedShare 白名单命中后解析出的共享授权结论。
type ResolvedShare struct {
	AccessLevel       string
	TargetType        EvalTargetType
	VersionPolicy     string
	SpecifiedIDs      []int64
	SpecifiedVersions []string
}

// Lookup 按 (来源空间, 资源类型, 资源 id, 调用方空间) 查询共享授权。
// 未命中（来源空间未配置 / 资源未共享 / 未授权给该调用方）返回 nil，交由上层 fail-closed 处理。
// readable 和 execute 均支持通配符 "*"，也支持显式指定调用方空间。
func (c *SharedResourceConfig) Lookup(sourceSpaceID int64, resourceType string, targetType EvalTargetType, resourceID, callerSpaceID int64) *ResolvedShare {
	if c == nil || c.SpaceRules == nil {
		return nil
	}
	spaceRules, ok := c.SpaceRules[sourceSpaceID]
	if !ok || spaceRules == nil {
		return nil
	}
	callerKey := formatSpaceID(callerSpaceID)
	for _, res := range spaceRules.Resources {
		if !sharedResourceMatches(res, resourceType, targetType) || res.ResourceID != resourceID {
			continue
		}
		accessLevel, matched := matchAccessLevel(res.AccessRules, callerKey)
		if !matched {
			return nil
		}
		versionPolicy := res.VersionPolicy
		specifiedVersions := res.SpecifiedVersions
		if resourceType == SharedResourceTypeEvalTarget && targetType.ToOperatorBaseType() != EvalTargetTypeLoopPrompt {
			versionPolicy = SharedVersionPolicyAll
			specifiedVersions = nil
		} else if versionPolicy == "" {
			versionPolicy = SharedVersionPolicyAll
		}
		return &ResolvedShare{
			AccessLevel:       accessLevel,
			TargetType:        res.TargetType,
			VersionPolicy:     versionPolicy,
			SpecifiedIDs:      res.SpecifiedIDs,
			SpecifiedVersions: specifiedVersions,
		}
	}
	return nil
}

func sharedResourceMatches(res *SharedResourceRule, resourceType string, targetType EvalTargetType) bool {
	if res == nil || res.ResourceType != resourceType {
		return false
	}
	if resourceType != SharedResourceTypeEvalTarget {
		return true
	}
	return targetType != 0 && res.TargetType.ToOperatorBaseType() == targetType.ToOperatorBaseType()
}

// matchAccessLevel 在资源的访问规则中匹配调用方空间，返回命中的访问级别。
// 优先返回 readable（内容可读优先于黑盒执行）；未知访问级别默认拒绝。
func matchAccessLevel(accessRules []*SharedAccessRule, callerKey string) (string, bool) {
	var matchedExecute bool
	for _, rule := range accessRules {
		if rule == nil {
			continue
		}

		switch rule.AccessLevel {
		case SharedAccessLevelReadable:
			for _, target := range rule.Targets {
				if target == sharedAccessTargetAll || target == callerKey {
					return SharedAccessLevelReadable, true
				}
			}
		case SharedAccessLevelExecute:
			for _, target := range rule.Targets {
				if target == sharedAccessTargetAll || target == callerKey {
					matchedExecute = true
				}
			}
		default:
			// 配置错误或未来新增但当前代码不认识的枚举值不得产生权限。
			continue
		}
	}

	if matchedExecute {
		return SharedAccessLevelExecute, true
	}
	return "", false
}

func formatSpaceID(spaceID int64) string {
	return strconv.FormatInt(spaceID, 10)
}

// SharedResourceEntry 枚举「共享给某调用方空间」时的单条命中结果。
type SharedResourceEntry struct {
	SourceSpaceID     int64
	ResourceID        int64
	ResourceType      string
	TargetType        EvalTargetType
	AccessLevel       string
	VersionPolicy     string
	SpecifiedIDs      []int64
	SpecifiedVersions []string
}

// ListSharedTo 枚举配置中「共享给 callerSpaceID」的指定类型资源。
// sourceSpaceFilter 为 nil 时跨全部来源空间枚举；有值则仅枚举该来源空间。
// 复用与 Lookup 相同的 matchAccessLevel 判定，readable 和 execute 均支持通配或精确空间匹配。
func (c *SharedResourceConfig) ListSharedTo(callerSpaceID int64, resourceType string, targetType EvalTargetType, sourceSpaceFilter *int64) []*SharedResourceEntry {
	if c == nil || c.SpaceRules == nil {
		return nil
	}
	callerKey := formatSpaceID(callerSpaceID)
	entries := make([]*SharedResourceEntry, 0)
	for sourceSpaceID, spaceRules := range c.SpaceRules {
		if sourceSpaceFilter != nil && *sourceSpaceFilter != sourceSpaceID {
			continue
		}
		if spaceRules == nil {
			continue
		}
		for _, res := range spaceRules.Resources {
			if !sharedResourceMatches(res, resourceType, targetType) {
				continue
			}
			accessLevel, matched := matchAccessLevel(res.AccessRules, callerKey)
			if !matched {
				continue
			}
			versionPolicy := res.VersionPolicy
			specifiedVersions := res.SpecifiedVersions
			if resourceType == SharedResourceTypeEvalTarget && targetType.ToOperatorBaseType() != EvalTargetTypeLoopPrompt {
				versionPolicy = SharedVersionPolicyAll
				specifiedVersions = nil
			} else if versionPolicy == "" {
				versionPolicy = SharedVersionPolicyAll
			}
			entries = append(entries, &SharedResourceEntry{
				SourceSpaceID:     sourceSpaceID,
				ResourceID:        res.ResourceID,
				ResourceType:      res.ResourceType,
				TargetType:        res.TargetType,
				AccessLevel:       accessLevel,
				VersionPolicy:     versionPolicy,
				SpecifiedIDs:      res.SpecifiedIDs,
				SpecifiedVersions: specifiedVersions,
			})
		}
	}
	return entries
}
