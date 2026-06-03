// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "encoding/json"

// ExptNotificationConf 实验通知配置（对应 IDL ExptNotificationConf）
type ExptNotificationConf struct {
	Filter             json.RawMessage          `json:"filter,omitempty"`
	Webhook            *WebhookNotificationConf `json:"webhook,omitempty"`
	FeishuNotification *FeishuNotificationConf  `json:"feishu_notification,omitempty"`
}

// WebhookNotificationConf Webhook 通知配置
type WebhookNotificationConf struct {
	Enable bool   `json:"enable"`
	URLs   string `json:"urls,omitempty"` // 逗号分隔的多个 URL
}

// FeishuNotificationConf 飞书通知配置
type FeishuNotificationConf struct {
	Enable bool   `json:"enable"`
	UserID string `json:"user_id,omitempty"`
}

// DefaultNotificationConf 返回默认通知配置（与 PRD 保持一致）
// 默认条件：实验状态包含开始执行、运行成功、运行失败
// 飞书开启，Webhook 关闭
func DefaultNotificationConf() *ExptNotificationConf {
	return &ExptNotificationConf{
		FeishuNotification: &FeishuNotificationConf{Enable: true},
		Webhook:            &WebhookNotificationConf{Enable: false},
	}
}

// WebhookGlobalConf Webhook 全局配置（由 IConfiger 提供）
type WebhookGlobalConf struct {
	Enable                  bool  `json:"enable" mapstructure:"enable"`
	RetryIntervals          []int `json:"retry_intervals" mapstructure:"retry_intervals"`                       // 秒
	MaxRetries              int   `json:"max_retries" mapstructure:"max_retries"`
	HTTPTimeoutSec          int   `json:"http_timeout_sec" mapstructure:"http_timeout_sec"`
	MaxURLsPerExperiment    int   `json:"max_urls_per_experiment" mapstructure:"max_urls_per_experiment"`
	NonRetryableStatusCodes []int `json:"non_retryable_status_codes" mapstructure:"non_retryable_status_codes"`
}

// DefaultWebhookGlobalConf 返回默认 Webhook 全局配置
func DefaultWebhookGlobalConf() *WebhookGlobalConf {
	return &WebhookGlobalConf{
		Enable:                  true,
		RetryIntervals:          []int{60, 300, 1800},
		MaxRetries:              3,
		HTTPTimeoutSec:          5,
		MaxURLsPerExperiment:    10,
		NonRetryableStatusCodes: []int{400, 401, 403, 404},
	}
}
