// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metric

import (
	"sort"
	"strings"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/metric"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/samber/lo"
)

const (
	maxPieCount = 10000
	retPieCount = 1000
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
	if len(ret.Pie) > retPieCount {
		ret.Pie = minimizePie(ret.Pie)
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

func minimizePie(pie map[string]string) map[string]string {
	if len(pie) > maxPieCount {
		for k, v := range pie {
			if v == "0" || v == "1" {
				delete(pie, k)
			}
		}
	}
	if len(pie) > retPieCount {
		keys := lo.Keys(pie)
		// 假设没有浮点数, pie都是整数
		sort.Slice(keys, func(i, j int) bool {
			a, b := pie[keys[i]], pie[keys[j]]
			if len(a) != len(b) {
				return len(a) > len(b)
			}
			return strings.Compare(a, b) > 0
		})
		ret := make(map[string]string)
		for _, key := range keys[:retPieCount] {
			ret[key] = pie[key]
		}
		pie = ret
	}
	return pie
}
