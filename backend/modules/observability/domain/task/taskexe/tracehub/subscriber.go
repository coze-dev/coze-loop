// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/taskexe"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type spanSubscriber struct {
	taskID       int64
	sync.RWMutex // protect t, buf
	t            *task.Task
	processor    taskexe.Processor
	buf          []*loop_span.Span
	bufCap       int // max buffer size

	flushWait        sync.WaitGroup
	maxFlushInterval time.Duration
	taskRepo         repo.ITaskRepo
	taskRunRepo      repo.ITaskRunRepo
	runType          task.TaskRunType
}

// Sampled 根据采样率计算是否被采样；采样数量将在 flush 时强制校验。
func (s *spanSubscriber) Sampled() bool {
	t := s.getTask()
	if t == nil || t.Rule == nil || t.Rule.Sampler == nil {
		return false
	}

	const base = 10000
	threshold := int64(float64(base) * t.GetRule().GetSampler().GetSampleRate())
	r := rand.Int63n(base) // todo: rand seed
	return r <= threshold
}
func (s *spanSubscriber) getTask() *task.Task {
	s.RLock()
	defer s.RUnlock()
	return s.t
}

// Match 检查 span 是否与 task 的 filter 匹配。
func (s *spanSubscriber) Match(ctx context.Context, span *loop_span.Span) (bool, error) {

	task := s.t
	if task == nil || task.Rule == nil {
		return false, nil
	}

	return true, nil
}
func (s *spanSubscriber) Creative(ctx context.Context) error {
	err := s.processor.OnChangeProcessor(ctx, s.t)
	if err != nil {
		return err
	}
	return nil
}

func (s *spanSubscriber) AddSpan(ctx context.Context, span *loop_span.Span) error {
	var taskRunConfig *entity.TaskRun
	var err error
	if s.runType == task.TaskRunTypeNewData {
		taskRunConfig, err = s.taskRunRepo.GetLatestNewDataTaskRun(ctx, nil, s.t.GetID())
		if err != nil {
			logs.CtxWarn(ctx, "get latest new data task run failed, task_id=%d, err: %v", s.t.GetID(), err)
			return err
		}
	} else {
		taskRunConfig, err = s.taskRunRepo.GetBackfillTaskRun(ctx, nil, s.t.GetID())
		if err != nil {
			logs.CtxWarn(ctx, "get backfill task run failed, task_id=%d, err: %v", s.t.GetID(), err)
			return err
		}
	}

	if taskRunConfig == nil {
		logs.CtxWarn(ctx, "no taskRunConfig：%v", taskRunConfig)
		return nil
	}

	if taskRunConfig.RunEndAt.UnixMilli() < time.Now().UnixMilli() || taskRunConfig.RunStartAt.UnixMilli() > time.Now().UnixMilli() {
		return nil
	}
	if span.StartTime < taskRunConfig.RunStartAt.UnixMilli() {
		logs.CtxWarn(ctx, "span start time is before task cycle start time, trace_id=%s, span_id=%s", span.TraceID, span.SpanID)
		return nil
	}
	trigger := &taskexe.Trigger{Task: s.t, Span: span}
	logs.CtxInfo(ctx, "invoke processor, trigger: %v", trigger)
	err = s.processor.Invoke(ctx, taskRunConfig, trigger)
	if err != nil {
		logs.CtxWarn(ctx, "invoke processor failed, trace_id=%s, span_id=%s, err: %v", span.TraceID, span.SpanID, err)
		return err
	}

	return nil
}
