// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metric

import (
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/metric"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func MetricPointDO2DTO(m *entity.MetricPoint) *metric.MetricPoint {
	return &metric.MetricPoint{
		Timestamp: &m.Timestamp,
		Value:     &m.Value,
	}
}

func MetricPointListDO2DTO(m []*entity.MetricPoint) []*metric.MetricPoint {
	res := make([]*metric.MetricPoint, 0, len(m))
	for _, v := range m {
		res = append(res, MetricPointDO2DTO(v))
	}
	return res
}

func MetricDO2DTO(m *entity.Metric) *metric.Metric {
	ret := &metric.Metric{}
	if m.Summary != "" {
		ret.Summary = ptr.Of(m.Summary)
	}
	for k, v := range m.Pie {
		if ret.Pie == nil {
			ret.Pie = make(map[string]string)
		}
		ret.Pie[k] = v
	}
	for k, v := range m.TimeSeries {
		if ret.TimeSeries == nil {
			ret.TimeSeries = make(map[string][]*metric.MetricPoint)
		}
		ret.TimeSeries[k] = MetricPointListDO2DTO(v)
	}
	return ret
}

func CompareDTO2DO(c *metric.Compare) *entity.Compare {
	if c == nil {
		return &entity.Compare{}
	}
	return &entity.Compare{
		Type:  entity.MetricCompareType(ptr.From(c.CompareType)),
		Shift: ptr.From(c.ShiftSeconds),
	}
}
