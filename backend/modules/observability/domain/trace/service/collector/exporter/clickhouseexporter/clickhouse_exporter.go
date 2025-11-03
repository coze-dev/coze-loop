// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/consumer"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ckExporter struct {
	config    *Config
	traceRepo repo.ITraceRepo
}

func (c *ckExporter) Start(ctx context.Context) error {
	logs.Info("ck exporter starting")
	return nil
}

func (c *ckExporter) Shutdown(ctx context.Context) error {
	logs.Info("ck exporter shutting down")
	return nil
}

func (c *ckExporter) ConsumeTraces(ctx context.Context, td consumer.Traces) error {
	tracesMap := make(map[loop_span.TTL]loop_span.SpanList)
	for _, td := range td.TraceData {
		ttl := td.TenantInfo.TTL
		if tracesMap[ttl] == nil {
			tracesMap[ttl] = make(loop_span.SpanList, 0)
		}
		tracesMap[ttl] = append(tracesMap[ttl], td.SpanList...)
	}
	for ttl, spans := range tracesMap {
		if err := c.traceRepo.InsertSpans(ctx, &repo.InsertTraceParam{
			Spans:  spans,
			Tenant: td.Tenant,
			TTL:    ttl,
		}); err != nil {
			logs.CtxError(ctx, "inert %d spans failed, %v", len(spans), err)
			return err
		}
		logs.CtxInfo(ctx, "inert %d spans successfully", len(spans))
	}
	return nil
}
