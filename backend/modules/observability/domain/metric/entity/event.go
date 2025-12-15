// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type MetricEvent struct {
	PlatformType string
	WorkspaceID  string
	StartDate    string
	MetricName   string
	MetricValue  string
	ObjectKeys   map[string]string
}
