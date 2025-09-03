// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"strconv"
	"sync"

	"github.com/cloudwego/kitex/pkg/utils/kitexutil"

	"github.com/coze-dev/coze-loop/backend/infra/metrics"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

const (
	promptLabelVersionCacheMetricsName = "prompt_label_version_cache"
)

func promptLabelVersionCacheMtrTags() []string {
	return []string{
		tagMethod,
		tagHit,
	}
}

var (
	promptLabelVersionCacheMetrics         *PromptLabelVersionCacheMetrics
	promptLabelVersionCacheMetricsInitOnce sync.Once
)

func NewPromptLabelVersionCacheMetrics(meter metrics.Meter) *PromptLabelVersionCacheMetrics {
	if meter == nil {
		return nil
	}
	promptLabelVersionCacheMetricsInitOnce.Do(func() {
		metric, err := meter.NewMetric(promptLabelVersionCacheMetricsName, []metrics.MetricType{metrics.MetricTypeCounter}, promptLabelVersionCacheMtrTags())
		if err != nil {
			logs.CtxError(context.Background(), "new prompt label version cache metrics failed, err = %v", err)
		}
		promptLabelVersionCacheMetrics = &PromptLabelVersionCacheMetrics{metric: metric}
	})
	return promptLabelVersionCacheMetrics
}

type PromptLabelVersionCacheMetrics struct {
	metric metrics.Metric
}

type PromptLabelVersionCacheMetricsParam struct {
	HitNum  int
	MissNum int
}

func (p *PromptLabelVersionCacheMetrics) MEmit(ctx context.Context, param PromptLabelVersionCacheMetricsParam) {
	if p == nil || p.metric == nil {
		return
	}
	method, _ := kitexutil.GetMethod(ctx)
	if method == "" {
		method = "unknown"
	}

	// 发送命中的 metrics
	if param.HitNum > 0 {
		p.metric.Emit([]metrics.T{
			{Name: tagMethod, Value: method},
			{Name: tagHit, Value: strconv.FormatBool(true)},
		}, metrics.Counter(int64(param.HitNum), metrics.WithSuffix(getSuffix+throughputSuffix)))
	}

	// 发送未命中的 metrics
	if param.MissNum > 0 {
		p.metric.Emit([]metrics.T{
			{Name: tagMethod, Value: method},
			{Name: tagHit, Value: strconv.FormatBool(false)},
		}, metrics.Counter(int64(param.MissNum), metrics.WithSuffix(getSuffix+throughputSuffix)))
	}
}
