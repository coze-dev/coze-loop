// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// ISkillPreloader AgentBuddy skill 预下载入 TOS 的 Port 接口。
//
// 提交实验时，把该实验用到的 agent_buddy 来源 skill 用发起人 user JWT + 平台 SA JWT
// 下载一次并入库到 Fornax TOS，把 tos_key 记入实验 eval_conf 的 SkillTOSKeys 快照字段；
// 之后该实验的所有执行/重试批次都从 TOS 现签下发，与 AgentBuddy、user JWT 解耦。
//
// 开源版仅提供 Port 接口 + Noop 空实现供依赖注入图占位；真实现由商业版注入。
//
//go:generate mockgen -destination=mocks/skill_preloader.go -package=mocks . ISkillPreloader
type ISkillPreloader interface {
	// PreloadAgentBuddySkills 预下载指定实验用到的 agent_buddy skill 入库 TOS 并回写 tos_key。
	// exptID 实验 ID；spaceID 空间 ID；skillConfigs 该实验涉及的 skill 配置；userJWT 发起人透传的 user JWT。
	PreloadAgentBuddySkills(ctx context.Context, exptID int64, spaceID int64, skillConfigs []*entity.SkillConfig, userJWT string) error
}
