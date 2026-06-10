// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"github.com/bytedance/gg/gptr"

	expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// ConvertNotificationRulesToDomain 将 thrift NotificationRule 列表转换为 domain entity
func ConvertNotificationRulesToDomain(rules []*expt.NotificationRule) []entity.NotificationRule {
	if len(rules) == 0 {
		return nil
	}
	result := make([]entity.NotificationRule, 0, len(rules))
	for _, r := range rules {
		if r == nil {
			continue
		}
		rule := entity.NotificationRule{
			Trigger: gptr.Indirect(r.Trigger),
		}
		if len(r.Actions) > 0 {
			actions := make([]*entity.NotificationAction, 0, len(r.Actions))
			for _, a := range r.Actions {
				if a == nil {
					continue
				}
				actions = append(actions, &entity.NotificationAction{
					Type: gptr.Indirect(a.Type),
					URL:  gptr.Indirect(a.URL),
				})
			}
			rule.Actions = actions
		}
		result = append(result, rule)
	}
	return result
}

// ConvertNotificationRulesToDTO 将 domain entity NotificationRule 列表转换为 thrift DTO
func ConvertNotificationRulesToDTO(rules []entity.NotificationRule) []*expt.NotificationRule {
	if len(rules) == 0 {
		return nil
	}
	result := make([]*expt.NotificationRule, 0, len(rules))
	for _, r := range rules {
		rule := &expt.NotificationRule{
			Trigger: gptr.Of(r.Trigger),
		}
		if len(r.Actions) > 0 {
			actions := make([]*expt.NotificationAction, 0, len(r.Actions))
			for _, a := range r.Actions {
				if a == nil {
					continue
				}
				act := &expt.NotificationAction{
					Type: gptr.Of(a.Type),
				}
				if a.URL != "" {
					act.URL = gptr.Of(a.URL)
				}
				actions = append(actions, act)
			}
			rule.Actions = actions
		}
		result = append(result, rule)
	}
	return result
}
