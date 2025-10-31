// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package span_processor

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

type PlatformProcessor struct {
	cfg loop_span.SpanTransCfgList
}

func (p *PlatformProcessor) Transform(ctx context.Context, spans loop_span.SpanList) (loop_span.SpanList, error) {
	return p.cfg.Transform(ctx, spans)
}

type PlatformProcessorFactory struct {
	traceConfig config.ITraceConfig
}

func (p *PlatformProcessorFactory) CreateProcessor(ctx context.Context, set Settings) (Processor, error) {
	cfg, err := p.traceConfig.GetPlatformSpansTrans(ctx, set.PlatformType)
	if err != nil {
		return nil, err
	}
	return &PlatformProcessor{
		cfg: cfg,
	}, nil
}

func NewPlatformProcessorFactory(traceConfig config.ITraceConfig) Factory {
	return &PlatformProcessorFactory{
		traceConfig: traceConfig,
	}
}
