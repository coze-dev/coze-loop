// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// Notification config domain entities (T1.1). Mirror the IDL
// `ExptNotificationConf` model shipped in cozeloop-idl-commercial/open/:
// one filter shared across channels, one webhook conf (comma-separated URLs),
// one feishu conf.

// ExptNotificationFieldType enumerates supported filter fields.
type ExptNotificationFieldType int32

const (
	ExptNotificationFieldTypeUnknown    ExptNotificationFieldType = 0
	ExptNotificationFieldTypeExptStatus ExptNotificationFieldType = 1
)

// ExptNotificationOperator enumerates filter operators.
type ExptNotificationOperator int32

const (
	ExptNotificationOperatorUnknown ExptNotificationOperator = 0
	ExptNotificationOperatorIN      ExptNotificationOperator = 1
	ExptNotificationOperatorNOTIN   ExptNotificationOperator = 2
)

// ExptNotificationFilter is a single condition on lifecycle events.
type ExptNotificationFilter struct {
	Field    ExptNotificationFieldType `json:"field"`
	Operator ExptNotificationOperator  `json:"operator"`
	Values   []string                  `json:"values"`
}

// ExptNotificationRule bundles a filter with per-channel configs. It exists
// mainly as the shape the dispatcher iterates over; the IDL keeps a single
// filter + channels combo, and BITs internal injection appends internal rules
// through `DispatchRequest.InternalRules`.
type ExptNotificationRule struct {
	Filter         *ExptNotificationFilter  `json:"filter"`
	Webhook        *WebhookNotificationConf `json:"webhook"`
	Feishu         *FeishuNotificationConf  `json:"feishu"`
	InternalSource string                   `json:"internal_source"`
}

// WebhookNotificationConf mirrors IDL `WebhookNotificationConf`.
type WebhookNotificationConf struct {
	Enable bool     `json:"enable"`
	URLs   []string `json:"urls"`
}

// FeishuNotificationConf mirrors IDL `FeishuNotificationConf`.
type FeishuNotificationConf struct {
	Enable bool   `json:"enable"`
	UserID string `json:"user_id"`
}

// ExptNotificationConf is attached to Experiment / ExperimentTemplate.
type ExptNotificationConf struct {
	Filter             *ExptNotificationFilter  `json:"filter"`
	Webhook            *WebhookNotificationConf `json:"webhook"`
	FeishuNotification *FeishuNotificationConf  `json:"feishu_notification"`
}

// DefaultExptNotificationConf keeps behaviour equal to the pre-webhook path:
// feishu enabled, webhook disabled.
func DefaultExptNotificationConf() *ExptNotificationConf {
	return &ExptNotificationConf{
		Webhook:            &WebhookNotificationConf{Enable: false},
		FeishuNotification: &FeishuNotificationConf{Enable: true},
	}
}
