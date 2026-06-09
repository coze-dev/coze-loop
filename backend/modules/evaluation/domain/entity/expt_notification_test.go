// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func webhookRule(statuses []ExptStatus, op NotificationFilterOperator, urls ...string) *NotificationRule {
	rule := &NotificationRule{
		Actions: []*NotificationAction{
			{
				Type:    NotificationActionType_Webhook,
				Webhook: &NotificationWebhookConf{URLs: urls},
			},
		},
	}
	if op != NotificationFilterOperator_Unknown {
		rule.Conditions = []*NotificationFilterCondition{
			{Operator: op, StatusValues: statuses},
		}
	}
	return rule
}

func TestNotificationRule_matchFilter(t *testing.T) {
	tests := []struct {
		name   string
		rule   *NotificationRule
		status ExptStatus
		want   bool
	}{
		{
			name:   "nil rule",
			rule:   nil,
			status: ExptStatus_Success,
			want:   false,
		},
		{
			name:   "no condition matches any status",
			rule:   &NotificationRule{},
			status: ExptStatus_Success,
			want:   true,
		},
		{
			name:   "In hit",
			rule:   &NotificationRule{Conditions: []*NotificationFilterCondition{{Operator: NotificationFilterOperator_In, StatusValues: []ExptStatus{ExptStatus_Success}}}},
			status: ExptStatus_Success,
			want:   true,
		},
		{
			name:   "In miss",
			rule:   &NotificationRule{Conditions: []*NotificationFilterCondition{{Operator: NotificationFilterOperator_In, StatusValues: []ExptStatus{ExptStatus_Success}}}},
			status: ExptStatus_Failed,
			want:   false,
		},
		{
			name:   "NotIn hit means excluded",
			rule:   &NotificationRule{Conditions: []*NotificationFilterCondition{{Operator: NotificationFilterOperator_NotIn, StatusValues: []ExptStatus{ExptStatus_Failed}}}},
			status: ExptStatus_Failed,
			want:   false,
		},
		{
			name:   "NotIn miss means included",
			rule:   &NotificationRule{Conditions: []*NotificationFilterCondition{{Operator: NotificationFilterOperator_NotIn, StatusValues: []ExptStatus{ExptStatus_Failed}}}},
			status: ExptStatus_Success,
			want:   true,
		},
		{
			name:   "unknown operator not matched",
			rule:   &NotificationRule{Conditions: []*NotificationFilterCondition{{Operator: NotificationFilterOperator_Unknown, StatusValues: []ExptStatus{ExptStatus_Success}}}},
			status: ExptStatus_Success,
			want:   false,
		},
		{
			name: "multiple conditions AND - all hit",
			rule: &NotificationRule{Conditions: []*NotificationFilterCondition{
				{Operator: NotificationFilterOperator_In, StatusValues: []ExptStatus{ExptStatus_Success, ExptStatus_Failed}},
				{Operator: NotificationFilterOperator_NotIn, StatusValues: []ExptStatus{ExptStatus_Failed}},
			}},
			status: ExptStatus_Success,
			want:   true,
		},
		{
			name: "multiple conditions AND - one fails",
			rule: &NotificationRule{Conditions: []*NotificationFilterCondition{
				{Operator: NotificationFilterOperator_In, StatusValues: []ExptStatus{ExptStatus_Success, ExptStatus_Failed}},
				{Operator: NotificationFilterOperator_NotIn, StatusValues: []ExptStatus{ExptStatus_Failed}},
			}},
			status: ExptStatus_Failed,
			want:   false,
		},
		{
			name:   "nil condition entry skipped",
			rule:   &NotificationRule{Conditions: []*NotificationFilterCondition{nil}},
			status: ExptStatus_Success,
			want:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.rule.matchFilter(tt.status))
		})
	}
}

func TestNotificationConf_ShouldWebhook(t *testing.T) {
	t.Run("nil conf returns false (forward-compat)", func(t *testing.T) {
		var c *NotificationConf
		assert.False(t, c.ShouldWebhook(ExptStatus_Success))
	})

	conf := &NotificationConf{Rules: []*NotificationRule{
		webhookRule([]ExptStatus{ExptStatus_Success}, NotificationFilterOperator_In, "https://example.com/hook"),
	}}
	assert.True(t, conf.ShouldWebhook(ExptStatus_Success))
	assert.False(t, conf.ShouldWebhook(ExptStatus_Failed))

	// terminated covers both Terminated and SystemTerminated when encoded together
	termConf := &NotificationConf{Rules: []*NotificationRule{
		webhookRule([]ExptStatus{ExptStatus_Terminated, ExptStatus_SystemTerminated}, NotificationFilterOperator_In, "https://example.com/hook"),
	}}
	assert.True(t, termConf.ShouldWebhook(ExptStatus_Terminated))
	assert.True(t, termConf.ShouldWebhook(ExptStatus_SystemTerminated))
	assert.False(t, termConf.ShouldWebhook(ExptStatus_Success))
}

func TestNotificationConf_ShouldFeishu(t *testing.T) {
	t.Run("nil conf returns false (base handler fallback)", func(t *testing.T) {
		var c *NotificationConf
		assert.False(t, c.ShouldFeishu(ExptStatus_Success))
	})

	conf := &NotificationConf{Rules: []*NotificationRule{
		{
			Conditions: []*NotificationFilterCondition{{Operator: NotificationFilterOperator_In, StatusValues: []ExptStatus{ExptStatus_Success}}},
			Actions:    []*NotificationAction{{Type: NotificationActionType_Feishu, Feishu: &NotificationFeishuConf{}}},
		},
	}}
	assert.True(t, conf.ShouldFeishu(ExptStatus_Success))
	assert.False(t, conf.ShouldFeishu(ExptStatus_Failed))
	assert.False(t, conf.ShouldWebhook(ExptStatus_Success))
}

func TestNotificationConf_WebhookURLs(t *testing.T) {
	t.Run("nil conf returns nil", func(t *testing.T) {
		var c *NotificationConf
		assert.Nil(t, c.WebhookURLs(ExptStatus_Success))
	})

	conf := &NotificationConf{Rules: []*NotificationRule{
		webhookRule([]ExptStatus{ExptStatus_Success}, NotificationFilterOperator_In, "https://a.com/h", "https://b.com/h"),
		webhookRule([]ExptStatus{ExptStatus_Success}, NotificationFilterOperator_In, "https://a.com/h", "https://c.com/h"), // dup a.com
		webhookRule([]ExptStatus{ExptStatus_Failed}, NotificationFilterOperator_In, "https://d.com/h"),                     // not matched
	}}
	got := conf.WebhookURLs(ExptStatus_Success)
	assert.Equal(t, []string{"https://a.com/h", "https://b.com/h", "https://c.com/h"}, got)
	assert.Empty(t, conf.WebhookURLs(ExptStatus_Pending))
}

func TestNotificationConf_Valid(t *testing.T) {
	tests := []struct {
		name    string
		conf    *NotificationConf
		wantErr bool
	}{
		{
			name:    "nil conf is valid (forward-compat)",
			conf:    nil,
			wantErr: false,
		},
		{
			name: "valid webhook conf",
			conf: &NotificationConf{Rules: []*NotificationRule{
				webhookRule([]ExptStatus{ExptStatus_Success}, NotificationFilterOperator_In, "https://example.com/hook"),
			}},
			wantErr: false,
		},
		{
			name: "valid feishu conf",
			conf: &NotificationConf{Rules: []*NotificationRule{{
				Actions: []*NotificationAction{{Type: NotificationActionType_Feishu, Feishu: &NotificationFeishuConf{}}},
			}}},
			wantErr: false,
		},
		{
			name: "invalid operator",
			conf: &NotificationConf{Rules: []*NotificationRule{{
				Conditions: []*NotificationFilterCondition{{Operator: NotificationFilterOperator(99), StatusValues: []ExptStatus{ExptStatus_Success}}},
			}}},
			wantErr: true,
		},
		{
			name: "webhook without urls",
			conf: &NotificationConf{Rules: []*NotificationRule{{
				Actions: []*NotificationAction{{Type: NotificationActionType_Webhook, Webhook: &NotificationWebhookConf{}}},
			}}},
			wantErr: true,
		},
		{
			name: "webhook nil conf",
			conf: &NotificationConf{Rules: []*NotificationRule{{
				Actions: []*NotificationAction{{Type: NotificationActionType_Webhook}},
			}}},
			wantErr: true,
		},
		{
			name: "webhook empty url",
			conf: &NotificationConf{Rules: []*NotificationRule{
				webhookRule(nil, NotificationFilterOperator_Unknown, "   "),
			}},
			wantErr: true,
		},
		{
			name: "webhook non-http scheme",
			conf: &NotificationConf{Rules: []*NotificationRule{
				webhookRule(nil, NotificationFilterOperator_Unknown, "ftp://example.com/h"),
			}},
			wantErr: true,
		},
		{
			name: "webhook missing host",
			conf: &NotificationConf{Rules: []*NotificationRule{
				webhookRule(nil, NotificationFilterOperator_Unknown, "https://"),
			}},
			wantErr: true,
		},
		{
			name: "unknown action type",
			conf: &NotificationConf{Rules: []*NotificationRule{{
				Actions: []*NotificationAction{{Type: NotificationActionType(99)}},
			}}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.conf.Valid()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestContainsStatus(t *testing.T) {
	assert.True(t, containsStatus([]ExptStatus{ExptStatus_Success, ExptStatus_Failed}, ExptStatus_Failed))
	assert.False(t, containsStatus([]ExptStatus{ExptStatus_Success}, ExptStatus_Failed))
	assert.False(t, containsStatus(nil, ExptStatus_Success))
}
