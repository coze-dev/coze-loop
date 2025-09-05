// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	config "github.com/coze-dev/coze-loop/backend/modules/data/domain/component/conf"
	tconv "github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/taskexe/processor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/consumer"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	goredis "github.com/redis/go-redis/v9"
)

type TraceHub struct {
	c        consumer.Consumer
	cfg      *config.ConsumerConfig
	redis    *goredis.Client
	ticker   *time.Ticker
	stopChan chan struct{}
	TaskRepo repo.ITaskRepo
}

type ITraceHubService interface {
	TraceHub(ctx context.Context, event *entity.RawSpan) error
}

func NewTraceHubImpl(
	tRepo repo.ITaskRepo,
) (ITraceHubService, error) {
	return &TraceHubServiceImpl{
		TaskRepo: tRepo,
	}, nil
}

type TraceHubServiceImpl struct {
	TaskRepo repo.ITaskRepo
}

func (t *TraceHubServiceImpl) TraceHub(ctx context.Context, span *entity.RawSpan) error {
	// 转换成
	return nil
}

func NewTraceHub(redisCli *goredis.Client, cfg *config.ConsumerConfig) (*TraceHub, error) {
	// 初始化tracehub结构体
	h := &TraceHub{
		c:        nil,
		cfg:      cfg,
		redis:    redisCli,
		ticker:   time.NewTicker(5 * time.Minute), // 每x分钟执行一次定时任务
		stopChan: make(chan struct{}),
	}
	// 定时任务处理？
	return h, nil
}

func (h *TraceHub) Start(ctx context.Context, handler entity.RawSpan) error {
	// 启动定时任务
	go func() {
		for {
			select {
			case <-h.ticker.C:
				// 执行定时任务
				h.runScheduledTask()
			case <-h.stopChan:
				// 停止定时任务
				h.ticker.Stop()
				return
			}
		}
	}()
	// 启动消费trace数据
	return nil
}
func (h *TraceHub) runScheduledTask() {
	ctx := context.Background()
	// 执行定时任务
	for {
		select {
		case <-h.ticker.C:
			logs.CtxInfo(ctx, "定时任务开始执行...")
			// 读取所有非终态（成功/禁用）任务
			taskPOs, err := h.TaskRepo.ListNonFinalTask(ctx)
			if err != nil {
				logs.CtxError(ctx, "ListNonFinalTask err:%v", err)
				continue
			}
			tasks := tconv.TaskPOs2DOs(ctx, taskPOs, nil)
			// 遍历任务
			for _, taskInfo := range tasks {
				logID := logs.NewLogID()
				ctx = logs.SetLogID(ctx, logID)

				endTime := time.Unix(0, taskInfo.GetRule().GetEffectiveTime().GetEndAt()*int64(time.Millisecond))
				startTime := time.Unix(0, taskInfo.GetRule().GetEffectiveTime().GetStartAt()*int64(time.Millisecond))
				proc, err := processor.NewProcessor(ctx, task.TaskTypeAutoEval)
				if err != nil {
					logs.CtxError(ctx, "NewProcessor err:%v", err)
					continue
				}
				// 达到任务时间期限
				// 到任务结束时间就结束
				if time.Now().After(endTime) {
					updateMap := map[string]interface{}{
						"task_status": task.TaskStatusSuccess,
					}
					err = h.TaskRepo.UpdateTaskWithOCC(ctx, taskInfo.GetID(), taskInfo.GetWorkspaceID(), updateMap)
					if err != nil {
						logs.CtxError(ctx, "[task-debug] UpdateTask err:%v", err)
						continue
					}
				}
				// 如果任务状态为unstarted，到任务开始时间就开始create
				if taskInfo.GetTaskStatus() == task.TaskStatusUnstarted && time.Now().After(startTime) {
					err = proc.OnChangeProcessor(ctx, taskInfo, task.TaskStatusUnstarted)
					if err != nil {
						logs.CtxError(ctx, "OnChangeProcessor err:%v", err)
						continue
					}
				}
			}
		case <-h.stopChan:
			h.ticker.Stop()
			logs.CtxInfo(ctx, "定时任务已停止")
			return
		}
	}
}
func (h *TraceHub) Stop() {
	// 停止定时任务
}
