// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"fmt"
	"net/url"
	"strings"
)

// NotificationActionType 通知渠道动作类型，与 IDL domain_expt.NotificationActionType 对齐
type NotificationActionType int32

const (
	NotificationActionType_Unknown NotificationActionType = 0
	NotificationActionType_Webhook NotificationActionType = 1 // HTTP 回调
	NotificationActionType_Feishu  NotificationActionType = 2 // 飞书消息（发实验创建人）
)

// NotificationFilterOperator 通知规则筛选运算符，本期仅支持 In / NotIn
type NotificationFilterOperator int32

const (
	NotificationFilterOperator_Unknown NotificationFilterOperator = 0
	NotificationFilterOperator_In      NotificationFilterOperator = 7 // 与 domain_expt.FilterOperatorType_In 对齐
	NotificationFilterOperator_NotIn   NotificationFilterOperator = 8 // 与 domain_expt.FilterOperatorType_NotIn 对齐
)

// NotificationFilterCondition 单条通知筛选条件
// 本期字段固定为实验状态（ExptStatus），运算符 In/NotIn，
// StatusValues 为目标状态集合（已从 FilterCondition.value 的逗号分隔编码解析后的整型集合）。
type NotificationFilterCondition struct {
	Operator     NotificationFilterOperator `json:"operator"`
	StatusValues []ExptStatus               `json:"status_values"`
}

// NotificationWebhookConf Webhook 渠道配置
type NotificationWebhookConf struct {
	URLs []string `json:"urls"`
}

// NotificationFeishuConf 飞书渠道配置（本期无可配字段，预留结构）
type NotificationFeishuConf struct{}

// NotificationAction 单个渠道动作
type NotificationAction struct {
	Type    NotificationActionType   `json:"type"`
	Webhook *NotificationWebhookConf `json:"webhook,omitempty"`
	Feishu  *NotificationFeishuConf  `json:"feishu,omitempty"`
}

// NotificationRule 单条通知规则：条件（filter）+ 动作（多渠道）
type NotificationRule struct {
	Conditions []*NotificationFilterCondition `json:"conditions,omitempty"`
	Actions    []*NotificationAction          `json:"actions,omitempty"`
}

// NotificationConf 实验通知配置（持久化到 experiment.notification_conf；模板放 template_conf 子键）
type NotificationConf struct {
	Rules []*NotificationRule `json:"rules,omitempty"`
}

// matchFilter 判断指定实验状态是否命中规则的筛选条件。
// 空条件（无 condition）视为「任意状态命中」；多条 condition 之间取「与」。
func (r *NotificationRule) matchFilter(status ExptStatus) bool {
	if r == nil {
		return false
	}
	if len(r.Conditions) == 0 {
		return true
	}
	for _, cond := range r.Conditions {
		if cond == nil {
			continue
		}
		hit := containsStatus(cond.StatusValues, status)
		switch cond.Operator {
		case NotificationFilterOperator_In:
			if !hit {
				return false
			}
		case NotificationFilterOperator_NotIn:
			if hit {
				return false
			}
		default:
			// 未知运算符视为不匹配
			return false
		}
	}
	return true
}

func containsStatus(statuses []ExptStatus, target ExptStatus) bool {
	for _, s := range statuses {
		if s == target {
			return true
		}
	}
	return false
}

// shouldAction 判断当前状态下是否存在命中规则的指定渠道动作，并返回首个命中的对应动作配置。
func (c *NotificationConf) hasAction(status ExptStatus, actionType NotificationActionType) bool {
	if c == nil {
		return false
	}
	for _, rule := range c.Rules {
		if rule == nil || !rule.matchFilter(status) {
			continue
		}
		for _, action := range rule.Actions {
			if action != nil && action.Type == actionType {
				return true
			}
		}
	}
	return false
}

// ShouldWebhook 当前状态下是否应触发 Webhook 通知。null 配置返回 false（向前兼容）。
func (c *NotificationConf) ShouldWebhook(status ExptStatus) bool {
	return c.hasAction(status, NotificationActionType_Webhook)
}

// ShouldFeishu 当前状态下是否应触发飞书通知。null 配置返回 false（向前兼容，由 base handler 兜底）。
func (c *NotificationConf) ShouldFeishu(status ExptStatus) bool {
	return c.hasAction(status, NotificationActionType_Feishu)
}

// Valid 校验通知配置：Webhook URL 非空且为合法 http/https；运算符仅允许 In/NotIn。
// null 配置视为合法（向前兼容）。
func (c *NotificationConf) Valid() error {
	if c == nil {
		return nil
	}
	for _, rule := range c.Rules {
		if rule == nil {
			continue
		}
		for _, cond := range rule.Conditions {
			if cond == nil {
				continue
			}
			if cond.Operator != NotificationFilterOperator_In && cond.Operator != NotificationFilterOperator_NotIn {
				return fmt.Errorf("invalid notification filter operator: %d (only In/NotIn allowed)", cond.Operator)
			}
		}
		for _, action := range rule.Actions {
			if action == nil {
				continue
			}
			switch action.Type {
			case NotificationActionType_Webhook:
				if action.Webhook == nil || len(action.Webhook.URLs) == 0 {
					return fmt.Errorf("webhook action requires at least one url")
				}
				for _, u := range action.Webhook.URLs {
					if err := validateWebhookURL(u); err != nil {
						return err
					}
				}
			case NotificationActionType_Feishu:
				// 飞书无可配字段，无需校验
			default:
				return fmt.Errorf("invalid notification action type: %d", action.Type)
			}
		}
	}
	return nil
}

func validateWebhookURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return fmt.Errorf("webhook url must not be empty")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid webhook url %q: %v", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("webhook url %q must be http or https", raw)
	}
	if u.Host == "" {
		return fmt.Errorf("webhook url %q must have a host", raw)
	}
	return nil
}

// WebhookURLs 返回当前状态下所有命中规则的 Webhook URL（去重保持顺序）。
func (c *NotificationConf) WebhookURLs(status ExptStatus) []string {
	if c == nil {
		return nil
	}
	seen := make(map[string]struct{})
	var urls []string
	for _, rule := range c.Rules {
		if rule == nil || !rule.matchFilter(status) {
			continue
		}
		for _, action := range rule.Actions {
			if action == nil || action.Type != NotificationActionType_Webhook || action.Webhook == nil {
				continue
			}
			for _, u := range action.Webhook.URLs {
				if u == "" {
					continue
				}
				if _, ok := seen[u]; ok {
					continue
				}
				seen[u] = struct{}{}
				urls = append(urls, u)
			}
		}
	}
	return urls
}
