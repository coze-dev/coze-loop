// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNotificationConf_ShouldNotify(t *testing.T) {
	tests := []struct {
		name   string
		conf   *NotificationConf
		status ExptStatus
		want   bool
	}{
		{
			name:   "nil conf returns false",
			conf:   nil,
			status: ExptStatus_Success,
			want:   false,
		},
		{
			name:   "nil filter returns true (all events trigger)",
			conf:   &NotificationConf{Filter: nil},
			status: ExptStatus_Success,
			want:   true,
		},
		{
			name: "empty filter (nil Includes and Excludes) returns true",
			conf: &NotificationConf{Filter: &ExptListFilter{}},
			status: ExptStatus_Failed,
			want:   true,
		},
		{
			name: "includes match - status in includes list",
			conf: &NotificationConf{
				Filter: &ExptListFilter{
					Includes: &ExptFilterFields{
						Status: []int64{int64(ExptStatus_Success), int64(ExptStatus_Failed)},
					},
				},
			},
			status: ExptStatus_Success,
			want:   true,
		},
		{
			name: "includes no match - status not in includes list",
			conf: &NotificationConf{
				Filter: &ExptListFilter{
					Includes: &ExptFilterFields{
						Status: []int64{int64(ExptStatus_Success), int64(ExptStatus_Failed)},
					},
				},
			},
			status: ExptStatus_Processing,
			want:   false,
		},
		{
			name: "excludes match - status in excludes list returns false",
			conf: &NotificationConf{
				Filter: &ExptListFilter{
					Excludes: &ExptFilterFields{
						Status: []int64{int64(ExptStatus_Processing)},
					},
				},
			},
			status: ExptStatus_Processing,
			want:   false,
		},
		{
			name: "excludes no match - status not in excludes list returns true",
			conf: &NotificationConf{
				Filter: &ExptListFilter{
					Excludes: &ExptFilterFields{
						Status: []int64{int64(ExptStatus_Processing)},
					},
				},
			},
			status: ExptStatus_Success,
			want:   true,
		},
		{
			name: "includes with empty status list - falls through to excludes check",
			conf: &NotificationConf{
				Filter: &ExptListFilter{
					Includes: &ExptFilterFields{
						Status: []int64{},
					},
					Excludes: &ExptFilterFields{
						Status: []int64{int64(ExptStatus_Failed)},
					},
				},
			},
			status: ExptStatus_Failed,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.conf.ShouldNotify(tt.status)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNotificationConf_ShouldWebhook(t *testing.T) {
	tests := []struct {
		name   string
		conf   *NotificationConf
		status ExptStatus
		want   bool
	}{
		{
			name:   "nil conf returns false",
			conf:   nil,
			status: ExptStatus_Success,
			want:   false,
		},
		{
			name:   "nil webhook conf returns false",
			conf:   &NotificationConf{Webhook: nil},
			status: ExptStatus_Success,
			want:   false,
		},
		{
			name: "webhook disabled returns false",
			conf: &NotificationConf{
				Webhook: &WebhookConf{Enable: false, URL: "http://example.com/hook"},
			},
			status: ExptStatus_Success,
			want:   false,
		},
		{
			name: "webhook enabled but empty URL returns false",
			conf: &NotificationConf{
				Webhook: &WebhookConf{Enable: true, URL: ""},
			},
			status: ExptStatus_Success,
			want:   false,
		},
		{
			name: "webhook enabled with URL and matching status returns true",
			conf: &NotificationConf{
				Webhook: &WebhookConf{Enable: true, URL: "http://example.com/hook"},
				Filter: &ExptListFilter{
					Includes: &ExptFilterFields{
						Status: []int64{int64(ExptStatus_Success)},
					},
				},
			},
			status: ExptStatus_Success,
			want:   true,
		},
		{
			name: "webhook enabled with URL but non-matching status returns false",
			conf: &NotificationConf{
				Webhook: &WebhookConf{Enable: true, URL: "http://example.com/hook"},
				Filter: &ExptListFilter{
					Includes: &ExptFilterFields{
						Status: []int64{int64(ExptStatus_Success)},
					},
				},
			},
			status: ExptStatus_Failed,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.conf.ShouldWebhook(tt.status)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNotificationConf_ShouldFeishu(t *testing.T) {
	tests := []struct {
		name   string
		conf   *NotificationConf
		status ExptStatus
		want   bool
	}{
		{
			name:   "nil conf returns false",
			conf:   nil,
			status: ExptStatus_Success,
			want:   false,
		},
		{
			name:   "nil feishu notification conf returns false",
			conf:   &NotificationConf{FeishuNotification: nil},
			status: ExptStatus_Success,
			want:   false,
		},
		{
			name: "feishu disabled returns false",
			conf: &NotificationConf{
				FeishuNotification: &FeishuNotificationConf{Enable: false, UserID: "user123"},
			},
			status: ExptStatus_Success,
			want:   false,
		},
		{
			name: "feishu enabled with matching status returns true",
			conf: &NotificationConf{
				FeishuNotification: &FeishuNotificationConf{Enable: true, UserID: "user123"},
				Filter: &ExptListFilter{
					Includes: &ExptFilterFields{
						Status: []int64{int64(ExptStatus_Success), int64(ExptStatus_Failed)},
					},
				},
			},
			status: ExptStatus_Success,
			want:   true,
		},
		{
			name: "feishu enabled with non-matching status returns false",
			conf: &NotificationConf{
				FeishuNotification: &FeishuNotificationConf{Enable: true, UserID: "user123"},
				Filter: &ExptListFilter{
					Includes: &ExptFilterFields{
						Status: []int64{int64(ExptStatus_Success)},
					},
				},
			},
			status: ExptStatus_Processing,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.conf.ShouldFeishu(tt.status)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExptStatusToWebhookEvent(t *testing.T) {
	tests := []struct {
		name   string
		status ExptStatus
		want   WebhookEventType
	}{
		{
			name:   "Processing maps to experiment.started",
			status: ExptStatus_Processing,
			want:   WebhookEventStarted,
		},
		{
			name:   "Success maps to experiment.succeeded",
			status: ExptStatus_Success,
			want:   WebhookEventSucceeded,
		},
		{
			name:   "Failed maps to experiment.failed",
			status: ExptStatus_Failed,
			want:   WebhookEventFailed,
		},
		{
			name:   "Terminated maps to experiment.terminated",
			status: ExptStatus_Terminated,
			want:   WebhookEventTerminated,
		},
		{
			name:   "SystemTerminated maps to experiment.terminated",
			status: ExptStatus_SystemTerminated,
			want:   WebhookEventTerminated,
		},
		{
			name:   "Unknown status returns empty string",
			status: ExptStatus_Unknown,
			want:   "",
		},
		{
			name:   "Pending status returns empty string",
			status: ExptStatus_Pending,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExptStatusToWebhookEvent(tt.status)
			assert.Equal(t, tt.want, got)
		})
	}
}
