// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package component

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// SharedResourceConfigProvider 跨空间共享资源配置提供者。
//
// 该接口屏蔽底层配置存储（OSS 走本地配置/默认空实现，商业版走 TCC 等动态配置）的差异，
// 只对外暴露「标准共享配置模型」。授权判定（某来源空间是否把某资源共享给某调用方、访问级别、
// 版本策略）全部由上层基于返回的 entity.SharedResourceConfig.Lookup 完成。
//
// 安全默认：当无法获取到有效配置（返回 nil/空配置或 error）时，上层按 fail-closed 处理，
// 即拒绝一切跨空间共享读取。纯 OSS 环境默认即全拒，真正生效需商业版接入动态配置。
//
//go:generate mockgen -destination=mocks/resource_access_config.go -package=mocks . SharedResourceConfigProvider
type SharedResourceConfigProvider interface {
	// GetSharedResourceConfig 返回当前全量共享资源配置的标准模型。
	// 返回 nil 或空配置时，上层视为「无任何共享」并 fail-closed。
	GetSharedResourceConfig(ctx context.Context) (*entity.SharedResourceConfig, error)
}
