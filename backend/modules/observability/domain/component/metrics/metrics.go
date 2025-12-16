// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import "time"

//go:generate mockgen -destination=mocks/metrics.go -package=mocks . ITraceMetrics
type ITraceMetrics interface {
	EmitListSpans(workspaceId int64, spanType string, start time.Time, isError bool)
	EmitGetTrace(workspaceId int64, start time.Time, isError bool)
	EmitTraceOapi(method string, workspaceId int64, platformType, spanListType string, spanSize int64, errorCode int, start time.Time, isError bool)
	EmitSendMetric(start time.Time, isError bool)
}
