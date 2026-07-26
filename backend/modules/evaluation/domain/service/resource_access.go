// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate mockgen -destination=mocks/resource_access.go -package=mocks . ResourceAccessAuthorizer

// ResourceAccessAuthorizer 跨空间共享资源的统一读授权抽象。
//
// 所有「带版本查询」的共享读接口统一经它把关，实现 fail-closed：
//   - 普通访问（未开启共享）：ResourceSpaceID = CallerSpaceID，走原本空间基础鉴权。
//   - 共享访问：校验来源空间把资源共享给调用方（access_level + version_policy），
//     命中才放行，任一步失败即拒绝；ResourceSpaceID = 来源空间。
type ResourceAccessAuthorizer interface {
	// AuthorizeRead 单资源读授权，返回鉴权结果载体（含真实资源空间 / 访问级别 / 版本策略）。
	AuthorizeRead(ctx context.Context, req *entity.AuthorizeResourceRequest) (*entity.ResourceAccessContext, error)
	// ListSharedResources 枚举「共享给调用方空间」的指定类型资源（list 接口用）。
	// SourceSpaceFilter 为空表示跨全部来源空间枚举；有值则限定该来源空间。
	// 空配置 / 无共享 → 返回空列表（fail-closed）。
	ListSharedResources(ctx context.Context, req *entity.ListSharedResourcesRequest) ([]*entity.ResourceAccessContext, error)
}
