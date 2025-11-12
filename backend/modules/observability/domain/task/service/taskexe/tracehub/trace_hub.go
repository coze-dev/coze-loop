// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/mq"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe/processor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	trace_repo "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
)

//go:generate mockgen -destination=mocks/trace_hub_service.go -package=mocks . ITraceHubService

type ITraceHubService interface {
	SpanTrigger(ctx context.Context, span *loop_span.Span) error
	BackFill(ctx context.Context, event *entity.BackFillEvent) error
	StoneTaskCache(ctx context.Context, cacheInfo TaskCacheInfo) error
}

func NewTraceHubImpl(
	tRepo repo.ITaskRepo,
	traceRepo trace_repo.ITraceRepo,
	tenantProvider tenant.ITenantProvider,
	buildHelper service.TraceFilterProcessorBuilder,
	taskProcessor *processor.TaskProcessor,
	aid int32,
	backfillProducer mq.IBackfillProducer,
	locker lock.ILocker,
	config config.ITraceConfig,
) (ITraceHubService, error) {
	impl := &TraceHubServiceImpl{
		taskRepo:         tRepo,
		traceRepo:        traceRepo,
		tenantProvider:   tenantProvider,
		buildHelper:      buildHelper,
		taskProcessor:    taskProcessor,
		aid:              aid,
		backfillProducer: backfillProducer,
		locker:           locker,
		config:           config,
		localCache:       NewLocalCache(),
	}

	return impl, nil
}

type TraceHubServiceImpl struct {
	taskRepo         repo.ITaskRepo
	traceRepo        trace_repo.ITraceRepo
	tenantProvider   tenant.ITenantProvider
	taskProcessor    *processor.TaskProcessor
	buildHelper      service.TraceFilterProcessorBuilder
	backfillProducer mq.IBackfillProducer
	locker           lock.ILocker
	config           config.ITraceConfig

	// Local cache - caching non-terminal task information
	localCache *LocalCache

	aid int32
}

func (h *TraceHubServiceImpl) StoneTaskCache(ctx context.Context, cacheInfo TaskCacheInfo) error {
	h.localCache.StoneTaskCache(ctx, cacheInfo)
	return nil
}
