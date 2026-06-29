// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"errors"
	"strings"
)

// ErrWebhookURLRequired Webhook 渠道启用但未配置任何回调 URL。
// 由 service 层在保存通知配置前调用 NotificationConf.Validate 触发，阻止非法配置落库
// （对齐 capability spec「Webhook 渠道配置」: 启用 Webhook 时 URL 不可为空）。
var ErrWebhookURLRequired = errors.New("webhook enabled but no url configured")

// NotificationFilterOperatorType 通知触发条件运算符。
// 与 IDL domain/expt.thrift 的 FilterOperatorType 对齐（In=7 / NotIn=8），
// 本期通知条件仅使用 In / NotIn 两种运算符。
type NotificationFilterOperatorType int64

const (
	// NotificationFilterOperatorType_In 包含：实验状态命中条件值集合时触发
	NotificationFilterOperatorType_In NotificationFilterOperatorType = 7
	// NotificationFilterOperatorType_NotIn 不包含：实验状态不在条件值集合时触发
	NotificationFilterOperatorType_NotIn NotificationFilterOperatorType = 8
)

// NotificationStatusValue 通知条件值（面向用户的可选项），与具体 ExptStatus 解耦。
// 本期条件值固定四选：开始执行 / 运行成功 / 运行失败 / 被终止。
type NotificationStatusValue int64

const (
	// NotificationStatusValue_Started 开始执行 -> Processing
	NotificationStatusValue_Started NotificationStatusValue = 1
	// NotificationStatusValue_Succeeded 运行成功 -> Success
	NotificationStatusValue_Succeeded NotificationStatusValue = 2
	// NotificationStatusValue_Failed 运行失败 -> Failed
	NotificationStatusValue_Failed NotificationStatusValue = 3
	// NotificationStatusValue_Terminated 被终止 -> Terminated + SystemTerminated（本期合并，不区分手动/系统终止）
	NotificationStatusValue_Terminated NotificationStatusValue = 4
)

// notificationStatusValueToExptStatuses 条件值 -> 实验状态集合映射。
// 「被终止」同时覆盖手动终止(Terminated)与系统终止(SystemTerminated)。
func notificationStatusValueToExptStatuses(v NotificationStatusValue) []ExptStatus {
	switch v {
	case NotificationStatusValue_Started:
		return []ExptStatus{ExptStatus_Processing}
	case NotificationStatusValue_Succeeded:
		return []ExptStatus{ExptStatus_Success}
	case NotificationStatusValue_Failed:
		return []ExptStatus{ExptStatus_Failed}
	case NotificationStatusValue_Terminated:
		return []ExptStatus{ExptStatus_Terminated, ExptStatus_SystemTerminated}
	default:
		return nil
	}
}

// ExptStatusToNotificationStatusValue 实验状态 -> 条件值反向映射。
// 用于把状态变更事件折叠为面向用户的 4 个条件值；
// Terminated 与 SystemTerminated 统一折叠为「被终止」。
// 中间过渡态（Pending / Terminating / Draining 等）无对应条件值，返回 (0,false)。
func ExptStatusToNotificationStatusValue(status ExptStatus) (NotificationStatusValue, bool) {
	switch status {
	case ExptStatus_Processing:
		return NotificationStatusValue_Started, true
	case ExptStatus_Success:
		return NotificationStatusValue_Succeeded, true
	case ExptStatus_Failed:
		return NotificationStatusValue_Failed, true
	case ExptStatus_Terminated, ExptStatus_SystemTerminated:
		return NotificationStatusValue_Terminated, true
	default:
		return 0, false
	}
}

// NotificationFilterCondition 单条通知触发条件，复用「字段 + 运算符 + 条件值」通用模型。
// 本期字段固定为「实验状态」（不可切换），运算符为 In / NotIn（可切换），
// 条件值为状态值多选（开始执行 / 运行成功 / 运行失败 / 被终止）。
//
// 字段语义与 IDL FilterCondition 对齐：
//   - FieldType 复用既有 entity.FieldType_ExptStatus(3)；
//   - Operator 复用 In(7) / NotIn(8)。
type NotificationFilterCondition struct {
	FieldType FieldType
	Operator  NotificationFilterOperatorType
	Values    []NotificationStatusValue
}

// Match 判定给定实验状态是否命中本条件。
// In：status 落在任一条件值映射的状态集合内即命中；
// NotIn：status 不落在任一条件值映射的状态集合内即命中。
// 无法映射为条件值的中间过渡态：In 视为不命中；NotIn 视为命中（不在集合内）。
func (c *NotificationFilterCondition) Match(status ExptStatus) bool {
	if c == nil {
		return false
	}
	inSet := c.statusInValueSet(status)
	switch c.Operator {
	case NotificationFilterOperatorType_In:
		return inSet
	case NotificationFilterOperatorType_NotIn:
		return !inSet
	default:
		return false
	}
}

// statusInValueSet 判定 status 是否落在条件值集合映射出的实验状态集合内。
func (c *NotificationFilterCondition) statusInValueSet(status ExptStatus) bool {
	for _, v := range c.Values {
		for _, s := range notificationStatusValueToExptStatuses(v) {
			if s == status {
				return true
			}
		}
	}
	return false
}

// WebhookNotificationConf Webhook 渠道配置。
type WebhookNotificationConf struct {
	// Enable 是否启用 Webhook 渠道，默认 false。
	Enable bool
	// URLs 回调地址列表；Enable=true 时由 service 层校验非空。
	URLs []string
}

// FeishuNotificationConf 飞书渠道配置。
type FeishuNotificationConf struct {
	// Enable 是否启用飞书渠道，默认 true（向后兼容现有飞书行为）。
	Enable bool
}

// NotificationConf 实验通知配置值对象，挂载于 Experiment / ExptTemplate。
// 一套通知条件（Filter）被 Webhook 与飞书两个渠道共享。
type NotificationConf struct {
	// Filter 通知触发条件（本期单条 condition，字段固定为实验状态）。
	Filter *NotificationFilterCondition
	// Webhook Webhook 渠道配置。
	Webhook *WebhookNotificationConf
	// Feishu 飞书渠道配置。
	Feishu *FeishuNotificationConf
}

// DefaultNotificationConf 返回与现有飞书行为对齐的默认通知配置：
// 条件 = 实验状态 / 包含 / [开始执行, 运行成功, 运行失败]；飞书 = 开启；Webhook = 关闭。
//
// 说明：默认 filter 含「开始执行」(Processing) 仅对 Webhook 有意义；
// 飞书侧仍仅对终态发卡片（由 base handler 控制），见后端 design D3/D5。
func DefaultNotificationConf() *NotificationConf {
	return &NotificationConf{
		Filter: &NotificationFilterCondition{
			FieldType: FieldType_ExptStatus,
			Operator:  NotificationFilterOperatorType_In,
			Values: []NotificationStatusValue{
				NotificationStatusValue_Started,
				NotificationStatusValue_Succeeded,
				NotificationStatusValue_Failed,
			},
		},
		Webhook: &WebhookNotificationConf{Enable: false},
		Feishu:  &FeishuNotificationConf{Enable: true},
	}
}

// GetNotificationConfOrDefault null-safe 取配置：nil 时返回默认配置，
// 保证历史实验/模板（无 notification_conf）零迁移地享有默认行为。
func (c *NotificationConf) GetNotificationConfOrDefault() *NotificationConf {
	if c == nil {
		return DefaultNotificationConf()
	}
	out := &NotificationConf{
		Filter:  c.Filter,
		Webhook: c.Webhook,
		Feishu:  c.Feishu,
	}
	def := DefaultNotificationConf()
	if out.Filter == nil {
		out.Filter = def.Filter
	}
	if out.Webhook == nil {
		out.Webhook = def.Webhook
	}
	if out.Feishu == nil {
		out.Feishu = def.Feishu
	}
	return out
}

// matchFilter null-safe 地按通知条件判定状态是否命中。
func (c *NotificationConf) matchFilter(status ExptStatus) bool {
	conf := c.GetNotificationConfOrDefault()
	return conf.Filter.Match(status)
}

// ShouldNotifyWebhook 判定给定实验状态是否应触发 Webhook 投递：
// Webhook 渠道开启 且 状态命中通知条件。null-safe。
func (c *NotificationConf) ShouldNotifyWebhook(status ExptStatus) bool {
	conf := c.GetNotificationConfOrDefault()
	if conf.Webhook == nil || !conf.Webhook.Enable {
		return false
	}
	return conf.matchFilter(status)
}

// Validate 校验通知配置的保存合法性，供 service 层在保存（创建/编辑实验、创建模板）前调用，
// 非法时返回错误以阻止落库。本期校验项：Webhook 渠道启用时回调 URL 不可为空。
//
// null-safe：nil 配置等价默认配置（Webhook 关闭），恒合法。
func (c *NotificationConf) Validate() error {
	if c == nil {
		return nil
	}
	return c.Webhook.Validate()
}

// Validate 校验 Webhook 渠道配置：Enable=true 时 URLs 至少有一个非空 URL。
// null-safe：nil（未配置 Webhook 渠道）等价未启用，恒合法。
func (w *WebhookNotificationConf) Validate() error {
	if w == nil || !w.Enable {
		return nil
	}
	for _, u := range w.URLs {
		if strings.TrimSpace(u) != "" {
			return nil
		}
	}
	return ErrWebhookURLRequired
}

// ShouldNotifyFeishu 判定给定实验状态是否应发送飞书消息：
// 飞书渠道开启 且 状态命中通知条件。null-safe。
//
// 注意：飞书侧最终仅对终态发卡片（由 base HandleLifecycleEventImpl 控制，
// Processing 等中间态即使命中条件也不发飞书），本方法只表达「条件 + 渠道开关」层面的判定。
func (c *NotificationConf) ShouldNotifyFeishu(status ExptStatus) bool {
	conf := c.GetNotificationConfOrDefault()
	if conf.Feishu == nil || !conf.Feishu.Enable {
		return false
	}
	return conf.matchFilter(status)
}
