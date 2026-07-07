// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

// Package notifications 承接 SubmitExperimentOApi / UpdateExperimentOApi 入参
// notifications 字段的语义解析：nil (字段缺省 / 老实验 NULL) → PRD 默认单规则；
// [] (显式禁用) → 保留空数组；非空 → 交由 validator 校验合法性。
// 该包只做纯数据决策，不依赖 db / configer / handler。
package notifications

import "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"

// PRDDefault 返回 PRD 默认通知规则单条：contains + [Started, Succeeded, Failed]
// 三终态触发 + 仅飞书渠道（test_case 2 / test_case 6 已锁契约）。
// 每次调用返回独立切片，避免调用方修改污染其它实验。
func PRDDefault() []entity.NotificationRule {
	return []entity.NotificationRule{{
		Field:    "experiment.status",
		Operator: "contains",
		Triggers: []entity.NotificationTrigger{
			entity.NotificationTrigger_Started,
			entity.NotificationTrigger_Succeeded,
			entity.NotificationTrigger_Failed,
		},
		Actions: []entity.NotificationAction{{
			Type: entity.NotificationActionType_Feishu,
		}},
	}}
}

// ResolveOrDefault 把 caller 从 IDL 层拿到的 optional 字段落到最终落库形态。
//   - present=false → PRD 默认（test_case 2 缺省 / test_case 6 老实验 NULL）
//   - present=true 且 rules=nil / len==0 → 保留空数组显式禁用（test_case 4 二次 Update）
//   - present=true 且非空 → 原样返回，由 validator.Validate 保净
func ResolveOrDefault(present bool, rules []entity.NotificationRule) []entity.NotificationRule {
	if !present {
		return PRDDefault()
	}
	if rules == nil {
		return []entity.NotificationRule{}
	}
	return rules
}
