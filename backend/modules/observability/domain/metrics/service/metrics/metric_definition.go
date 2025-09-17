// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metrics/entity"
)

// GetAllMetricDefinitions 获取所有指标定义
func GetAllMetricDefinitions() []entity.IMetricDefinition {
	return []entity.IMetricDefinition{
		&TotalCountMetric{},
		&FailRatioMetric{},
		&ModelFailRatioMetric{},
		&ModelLatencyAvgMetric{},
		&ModelTotalTokensMetric{},
		&ToolTotalCountMetric{},
		&ToolFailRatioMetric{},
		&ToolLatencyAvgMetric{},
	}
}