// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// NotificationTrigger 实验状态触发事件枚举（与 IDL enum 对齐）。
type NotificationTrigger int32

const (
	NotificationTrigger_Unknown    NotificationTrigger = 0
	NotificationTrigger_Started    NotificationTrigger = 1 // ExptStatus_Processing 首次进入
	NotificationTrigger_Succeeded  NotificationTrigger = 2 // ExptStatus_Success
	NotificationTrigger_Failed     NotificationTrigger = 3 // ExptStatus_Failed
	NotificationTrigger_Terminated NotificationTrigger = 4 // Terminated + SystemTerminated 合并
)

// NotificationActionType 通知渠道类型。
type NotificationActionType int32

const (
	NotificationActionType_Unknown NotificationActionType = 0
	NotificationActionType_Webhook NotificationActionType = 1
	NotificationActionType_Feishu  NotificationActionType = 2
)

// NotificationAction 单一通知动作，Webhook 时 URL 必填。
type NotificationAction struct {
	Type NotificationActionType `json:"type"`
	URL  string                 `json:"url,omitempty"`
}

// NotificationRule 一条通知规则；一条规则可 fan-out 多渠道。
type NotificationRule struct {
	Field    string                `json:"field"`    // 本期固定 "experiment.status"
	Operator string                `json:"operator"` // "contains" | "not_contains"
	Triggers []NotificationTrigger `json:"triggers"`
	Actions  []NotificationAction  `json:"actions"`
}
