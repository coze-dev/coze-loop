// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"sync"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/mq"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe/processor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	trace_repo "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
)

type ITraceHubService interface {
	SpanTrigger(ctx context.Context, event *entity.RawSpan) error
	CallBack(ctx context.Context, event *entity.AutoEvalEvent) error
	Correction(ctx context.Context, event *entity.CorrectionEvent) error
	BackFill(ctx context.Context, event *entity.BackFillEvent) error
}

func NewTraceHubImpl(
	tRepo repo.ITaskRepo,
	traceRepo trace_repo.ITraceRepo,
	tenantProvider tenant.ITenantProvider,
	buildHelper service.TraceFilterProcessorBuilder,
	taskProcessor *processor.TaskProcessor,
	benefitSvc benefit.IBenefitService,
	aid int32,
	backfillProducer mq.IBackfillProducer,
) (ITraceHubService, error) {
	// Create two independent timers with different intervals
	scheduledTaskTicker := time.NewTicker(5 * time.Minute) // Task status lifecycle management - 5-minute interval
	syncTaskTicker := time.NewTicker(2 * time.Minute)      // Data synchronization - 1-minute interval
	impl := &TraceHubServiceImpl{
		taskRepo:            tRepo,
		scheduledTaskTicker: scheduledTaskTicker,
		syncTaskTicker:      syncTaskTicker,
		stopChan:            make(chan struct{}),
		traceRepo:           traceRepo,
		tenantProvider:      tenantProvider,
		buildHelper:         buildHelper,
		taskProcessor:       taskProcessor,
		benefitSvc:          benefitSvc,
		aid:                 aid,
		backfillProducer:    backfillProducer,
	}

	// Start the scheduled tasks immediately
	impl.startScheduledTask()

	// default+lane?+新集群？——定时任务和任务处理分开——内场
	return impl, nil
}

type TraceHubServiceImpl struct {
	scheduledTaskTicker *time.Ticker // Task status lifecycle management timer - 5-minute interval
	syncTaskTicker      *time.Ticker // Data synchronization timer - 1-minute interval
	stopChan            chan struct{}
	taskRepo            repo.ITaskRepo
	traceRepo           trace_repo.ITraceRepo
	tenantProvider      tenant.ITenantProvider
	taskProcessor       *processor.TaskProcessor
	buildHelper         service.TraceFilterProcessorBuilder
	benefitSvc          benefit.IBenefitService
	backfillProducer    mq.IBackfillProducer

	flushCh      chan *flushReq
	flushErrLock sync.Mutex
	flushErr     []error

	// Local cache - caching non-terminal task information
	taskCache     sync.Map
	taskCacheLock sync.RWMutex

	aid int32
}

type flushReq struct {
	retrievedSpanCount int64
	pageToken          string
	spans              []*loop_span.Span
	noMore             bool
}

const TagKeyResult = "tag_key"

func (h *TraceHubServiceImpl) Close() {
	close(h.stopChan)
}
