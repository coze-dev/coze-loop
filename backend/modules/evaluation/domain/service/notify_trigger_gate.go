// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"strings"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// isFeishuNotifySuppressedByTrigger 判断该实验是否因触发来源而整体抑制飞书通知。
//
// EvalX 与 eval_task 实验不发实验生命周期飞书卡片，只保留任务级通知。
//
// 判据取落库后的 expt.TriggerType：使用创建期裁决后的来源，而不是重新判断调度模式。
//
// 不覆盖洞察分析完成卡（insight analysis）：那是人在页面上主动点分析后的回执，
// 不属于实验生命周期通知，挡掉会让操作没有反馈。
func isFeishuNotifySuppressedByTrigger(expt *entity.Experiment) bool {
	if expt == nil {
		return false
	}
	return entity.ShouldEnforceByTrigger(expt.TriggerType) ||
		strings.EqualFold(strings.TrimSpace(expt.TriggerType), entity.ExptTriggerTypeEvalTask)
}
