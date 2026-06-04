// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
)

const (
	notificationDefaultStatusValues = "3,11,12"
	maxWebhookURLsPerExperiment     = 10
)

type NotificationFilterOperatorType int64

const (
	NotificationFilterOperatorType_In    NotificationFilterOperatorType = 7
	NotificationFilterOperatorType_NotIn NotificationFilterOperatorType = 8
)

type ExptNotificationConf struct {
	Filter             *NotificationFilter      `json:"filter,omitempty"`
	Webhook            *WebhookNotificationConf `json:"webhook,omitempty"`
	FeishuNotification *FeishuNotificationConf  `json:"feishu_notification,omitempty"`
}

type NotificationFilter struct {
	FilterConditions []*NotificationFilterCondition `json:"filter_conditions,omitempty"`
}

type NotificationFilterCondition struct {
	Field    *NotificationFilterField       `json:"field,omitempty"`
	Operator NotificationFilterOperatorType `json:"operator,omitempty"`
	Value    string                         `json:"value,omitempty"`
}

type NotificationFilterField struct {
	FieldType FieldType `json:"field_type,omitempty"`
	FieldKey  string    `json:"field_key,omitempty"`
}

type WebhookNotificationConf struct {
	Enable bool   `json:"enable"`
	URLs   string `json:"urls,omitempty"`
}

type FeishuNotificationConf struct {
	Enable bool   `json:"enable"`
	UserID string `json:"user_id,omitempty"`
}

func DefaultNotificationConf() *ExptNotificationConf {
	return &ExptNotificationConf{
		Filter: &NotificationFilter{
			FilterConditions: []*NotificationFilterCondition{
				{
					Field:    &NotificationFilterField{FieldType: FieldType_ExptStatus},
					Operator: NotificationFilterOperatorType_In,
					Value:    notificationDefaultStatusValues,
				},
			},
		},
		Webhook:            &WebhookNotificationConf{Enable: false},
		FeishuNotification: &FeishuNotificationConf{Enable: true},
	}
}

func (c *ExptNotificationConf) MatchStatus(status ExptStatus) bool {
	if c == nil {
		return true
	}
	return MatchNotificationFilter(c.Filter, status)
}

func MatchNotificationFilter(filter *NotificationFilter, status ExptStatus) bool {
	if filter == nil || len(filter.FilterConditions) == 0 {
		return true
	}
	for _, condition := range filter.FilterConditions {
		if condition == nil || condition.Field == nil || condition.Field.FieldType != FieldType_ExptStatus {
			continue
		}
		contains := containsNotificationStatus(condition.Value, status)
		switch condition.Operator {
		case NotificationFilterOperatorType_In:
			return contains
		case NotificationFilterOperatorType_NotIn:
			return !contains
		}
	}
	return true
}

func containsNotificationStatus(values string, status ExptStatus) bool {
	targets := notificationStatusAliases(status)
	for _, raw := range strings.Split(values, ",") {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if targets[value] {
			return true
		}
	}
	return false
}

func notificationStatusAliases(status ExptStatus) map[string]bool {
	result := map[string]bool{strconv.FormatInt(int64(status), 10): true}
	if status == ExptStatus_Terminated {
		result[strconv.FormatInt(int64(ExptStatus_SystemTerminated), 10)] = true
	}
	if status == ExptStatus_SystemTerminated {
		result[strconv.FormatInt(int64(ExptStatus_Terminated), 10)] = true
	}
	return result
}

func (c *WebhookNotificationConf) GetWebhookURLs() []string {
	if c == nil || strings.TrimSpace(c.URLs) == "" {
		return nil
	}
	rawURLs := strings.Split(c.URLs, ",")
	urls := make([]string, 0, len(rawURLs))
	for _, raw := range rawURLs {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		urls = append(urls, trimmed)
	}
	return urls
}

func (c *ExptNotificationConf) Validate() error {
	if c == nil || c.Webhook == nil || !c.Webhook.Enable {
		return nil
	}
	urls := c.Webhook.GetWebhookURLs()
	if len(urls) == 0 {
		return errors.New("Webhook URL is required when webhook is enabled")
	}
	if len(urls) > maxWebhookURLsPerExperiment {
		return errors.New("Maximum 10 webhook URLs allowed per experiment")
	}
	seen := make(map[string]struct{}, len(urls))
	for _, raw := range urls {
		if _, ok := seen[raw]; ok {
			return errors.New("Duplicate webhook URLs are not allowed")
		}
		seen[raw] = struct{}{}
		parsed, err := url.Parse(raw)
		if err != nil || parsed == nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return errors.New("Invalid webhook URL format, must start with http:// or https://")
		}
	}
	return nil
}
