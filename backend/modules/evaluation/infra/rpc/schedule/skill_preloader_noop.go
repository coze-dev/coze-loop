// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package schedule

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// noopSkillPreloader 开源版占位、真实现由商业版注入，仿 schedule noop。
// 不做任何预下载，保证依赖注入图可构建。
type noopSkillPreloader struct{}

// NewNoopSkillPreloader 返回不做实际预下载的空实现。
func NewNoopSkillPreloader() rpc.ISkillPreloader {
	return &noopSkillPreloader{}
}

func (n *noopSkillPreloader) PreloadAgentBuddySkills(ctx context.Context, exptID int64, spaceID int64, skillConfigs []*entity.SkillConfig, userJWT string) error {
	return nil
}
