// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// AuthorizeResourceRequest 统一的资源读授权入参（评测集 / 评测对象共用）。
type AuthorizeResourceRequest struct {
	// CallerSpaceID 调用方当前空间（发起实验 / 写记录 / 审计归属空间），例如 A。
	CallerSpaceID int64
	// ResourceType 资源类型：eval_set / eval_target。
	ResourceType string
	// ResourceID 资源 id。
	ResourceID int64
	// TargetType 评测对象类型；仅 ResourceType=eval_target 时参与共享配置匹配。
	TargetType EvalTargetType

	// VersionID / Version 供版本策略校验；不涉及版本的读取可为空。
	VersionID   *int64
	VersionName *string

	// SharedOption 共享访问声明（is_shared + source_space_id）；nil / 未开启即普通本空间读。
	SharedOption *SharedResourceOption
	// Action 基础鉴权动作（如 consts.Read）。
	Action string
	// OwnerID 资源 owner user id（用于本空间基础鉴权），可为 nil。
	OwnerID *string
	// RequireContentRead 为 true 表示本次读取会暴露资源内容（如评测集 item），
	// 共享场景下必须命中 readable 访问级别，execute（黑盒）会被拒绝。
	RequireContentRead bool
}

// ResourceAccessContext 评测服务内部的鉴权结果载体。
// 授权完成后，下游逻辑统一按它区分「调用方空间」和「资源真实空间」，不再信任请求参数。
type ResourceAccessContext struct {
	// CallerSpaceID 调用方当前空间（准入 / 审计 / 结果归属），例如 A。
	CallerSpaceID int64
	// ResourceSpaceID 资源真实空间；普通访问 = A，共享访问 = 来源空间 B。
	ResourceSpaceID int64

	ResourceType string
	ResourceID   int64
	TargetType   EvalTargetType

	AccessMode  AccessMode
	AccessLevel string
	// VersionPolicy latest / all / specified；SpecifiedIDs 仅 specified 有效。
	// SpecifiedVersions 用于按来源版本字符串配置白名单的评测对象。
	VersionPolicy     string
	SpecifiedIDs      []int64
	SpecifiedVersions []string
}

// IsShared 是否为跨空间共享访问。
func (c *ResourceAccessContext) IsShared() bool {
	return c != nil && c.AccessMode == AccessModeShared && c.ResourceSpaceID > 0 && c.ResourceSpaceID != c.CallerSpaceID
}

// QuerySpaceID 返回下游查询应使用的空间 id：共享读用来源空间，否则用调用方空间。
func (c *ResourceAccessContext) QuerySpaceID() int64 {
	if c == nil {
		return 0
	}
	if c.IsShared() && c.ResourceSpaceID > 0 {
		return c.ResourceSpaceID
	}
	return c.CallerSpaceID
}

// SharedInfo 生成回填给调用方的共享元信息；普通访问返回 nil（保持向后兼容）。
func (c *ResourceAccessContext) SharedInfo() *SharedResourceInfo {
	if !c.IsShared() {
		return nil
	}
	return &SharedResourceInfo{
		IsShared:      true,
		SourceSpaceID: c.ResourceSpaceID,
		AccessLevel:   c.AccessLevel,
		VersionPolicy: c.VersionPolicy,
	}
}

// ListSharedResourcesRequest 枚举「共享给调用方空间」的资源入参。
// SourceSpaceFilter 为空表示跨全部来源空间枚举；有值则限定该来源空间。
type ListSharedResourcesRequest struct {
	CallerSpaceID     int64
	ResourceType      string
	TargetType        EvalTargetType
	SourceSpaceFilter *int64
}
