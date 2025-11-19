// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type MetricEvent struct {
	PlatformType string            `json:"platform_type"`
	WorkspaceID  string            `json:"workspace_id"`
	StartDate    string            `json:"start_date"`
	MetricName   string            `json:"metric_name"`
	MetricValue  string            `json:"metric_value"`
	ObjectKeys   map[string]string `json:"object_key_list"`
}
