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
	promptCacheMetricsName = "prompt_cache"

	getSuffix        = "get"
	throughputSuffix = ".throughput"

	tagQueryType  = "query_type"
	tagWithCommit = "with_commit"
	tagMethod     = "method"
	tagHit        = "hit"
)

func promptCacheMtrTags() []string {
	return []string{
		tagQueryType,
		tagWithCommit,
		tagMethod,
		tagHit,
	}
}

var (
	promptCacheMetrics         *PromptCacheMetrics
	promptCacheMetricsInitOnce sync.Once
)

func NewPromptCacheMetrics(meter metrics.Meter) *PromptCacheMetrics {
	if meter == nil {
		return nil
	}
	promptCacheMetricsInitOnce.Do(func() {
		metric, err := meter.NewMetric(promptCacheMetricsName, []metrics.MetricType{metrics.MetricTypeCounter}, promptCacheMtrTags())
		if err != nil {
			logs.CtxError(context.Background(), "new prompt cache metrics failed, err = %v", err)
		}
		promptCacheMetrics = &PromptCacheMetrics{metric: metric}
	})
	return promptCacheMetrics
}

type PromptCacheMetrics struct {
	metric metrics.Metric
}

type QueryType string

const (
	QueryTypePromptKey QueryType = "prompt_key"
	QueryTypePromptID  QueryType = "prompt_id"
)

type PromptCacheMetricsParam struct {
	QueryType  QueryType
	WithCommit bool
	HitNum     int
	MissNum    int
}

func (p *PromptCacheMetrics) MEmit(ctx context.Context, param PromptCacheMetricsParam) {
	if p == nil || p.metric == nil {
		return
	}
	method, _ := kitexutil.GetMethod(ctx)
	if method == "" {
		method = "unknown"
	}
	p.metric.Emit([]metrics.T{
		{Name: tagQueryType, Value: string(param.QueryType)},
		{Name: tagWithCommit, Value: strconv.FormatBool(param.WithCommit)},
		{Name: tagMethod, Value: method},
		{Name: tagHit, Value: strconv.FormatBool(true)},
	}, metrics.Counter(int64(param.HitNum), metrics.WithSuffix(getSuffix+throughputSuffix)))

	p.metric.Emit([]metrics.T{
		{Name: tagQueryType, Value: string(param.QueryType)},
		{Name: tagWithCommit, Value: strconv.FormatBool(param.WithCommit)},
		{Name: tagMethod, Value: method},
		{Name: tagHit, Value: strconv.FormatBool(false)},
	}, metrics.Counter(int64(param.MissNum), metrics.WithSuffix(getSuffix+throughputSuffix)))
}
