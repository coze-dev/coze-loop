// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// ExptNotificationConf 实验通知配置
type ExptNotificationConf struct {
	// 触发条件（复用 Filters 组件结构）
	Filter *NotificationFilter `json:"filter,omitempty"`
	// Webhook 渠道配置
	Webhook *WebhookNotificationConf `json:"webhook,omitempty"`
	// 飞书渠道配置
	FeishuNotification *FeishuNotificationConf `json:"feishu_notification,omitempty"`
}

// NotificationFilter 通知触发条件
type NotificationFilter struct {
	FilterConditions []*NotificationFilterCondition `json:"filter_conditions,omitempty"`
	LogicOp          *FilterLogicOp                 `json:"logic_op,omitempty"`
}

// NotificationFilterCondition 通知过滤条件项
type NotificationFilterCondition struct {
	Field    *NotificationFilterField `json:"field,omitempty"`
	Operator NotificationOperatorType `json:"operator"`
	Value    string                   `json:"value,omitempty"`
}

// NotificationFilterField 通知过滤字段
type NotificationFilterField struct {
	FieldType NotificationFieldType `json:"field_type"`
	FieldKey  *string               `json:"field_key,omitempty"`
}

// NotificationFieldType 通知过滤字段类型
type NotificationFieldType int64

const (
	NotificationFieldType_Unknown    NotificationFieldType = 0
	NotificationFieldType_ExptStatus NotificationFieldType = 3 // 实验状态
)

// NotificationOperatorType 通知过滤操作符
type NotificationOperatorType int64

const (
	NotificationOperatorType_Unknown  NotificationOperatorType = 0
	NotificationOperatorType_Equal    NotificationOperatorType = 1 // 等于
	NotificationOperatorType_NotEqual NotificationOperatorType = 2 // 不等于
	NotificationOperatorType_In       NotificationOperatorType = 7 // 包含于
	NotificationOperatorType_NotIn    NotificationOperatorType = 8 // 不包含于
)

// WebhookEnvironment Webhook 目标环境（决定是否附加泳道路由 header）
// 取值与 IDL enum WebhookEnvironment 对齐：Prod=1 / PPE=2 / BOE=3。
type WebhookEnvironment int64

const (
	WebhookEnvironment_Prod WebhookEnvironment = 1 // 默认，不加任何路由 header
	WebhookEnvironment_PPE  WebhookEnvironment = 2
	WebhookEnvironment_BOE  WebhookEnvironment = 3
)

// WebhookNotificationConf Webhook 通知配置
type WebhookNotificationConf struct {
	Enable bool    `json:"enable"`
	Urls   *string `json:"urls,omitempty"`
	// Environment 缺省（nil）=> Prod（向后兼容，历史数据反序列化为未设置）
	Environment *WebhookEnvironment `json:"environment,omitempty"`
	// Lane ppe/boe 泳道名；prod / 未设置时忽略
	Lane *string `json:"lane,omitempty"`
}

// FeishuNotificationConf 飞书通知配置
type FeishuNotificationConf struct {
	Enable bool    `json:"enable"`
	UserID *string `json:"user_id,omitempty"`
}

// defaultSandboxAgentProgressNotifyIntervalSec 沙箱 agent 进度卡默认间隔（1h）。
const defaultSandboxAgentProgressNotifyIntervalSec = 3600

// SandboxAgentNotifyConf 沙箱 agent 通知相关配置。
//
// 结构对齐 ExptConsumerConf: 顶层 SandboxAgentNotifyConfItem 为全局默认,
// SpaceConf 按 spaceID 覆盖 (只需要覆盖的字段, 缺失时回落全局默认)。
type SandboxAgentNotifyConf struct {
	// ProgressNotifyIntervalSec 进度卡间隔（秒）。<=0 时用 defaultSandboxAgentProgressNotifyIntervalSec 兜底。
	ProgressNotifyIntervalSec int64 `json:"progress_notify_interval_sec" mapstructure:"progress_notify_interval_sec"`
	// SpaceConf 按 spaceID 覆盖，key=spaceID；未命中时用外层字段。
	SpaceConf map[int64]*SandboxAgentNotifyConf `json:"space_conf" mapstructure:"space_conf"`
}

// GetProgressNotifyIntervalSec 返回指定 space 的进度卡间隔（秒）。
// 优先取 SpaceConf[spaceID].ProgressNotifyIntervalSec，其次全局默认，最后兜底常量。
func (c *SandboxAgentNotifyConf) GetProgressNotifyIntervalSec(spaceID int64) int64 {
	if c != nil {
		if sc, ok := c.SpaceConf[spaceID]; ok && sc != nil && sc.ProgressNotifyIntervalSec > 0 {
			return sc.ProgressNotifyIntervalSec
		}
		if c.ProgressNotifyIntervalSec > 0 {
			return c.ProgressNotifyIntervalSec
		}
	}
	return defaultSandboxAgentProgressNotifyIntervalSec
}

// DefaultSandboxAgentNotifyConf 全局默认配置：进度卡 1h。
func DefaultSandboxAgentNotifyConf() *SandboxAgentNotifyConf {
	return &SandboxAgentNotifyConf{
		ProgressNotifyIntervalSec: defaultSandboxAgentProgressNotifyIntervalSec,
	}
}
