// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	datadataset "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	repo_mocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	service_mocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type capturingProcessor struct {
	trigger *taskexe.Trigger
}

func (c *capturingProcessor) ValidateConfig(ctx context.Context, config any) error { return nil }
func (c *capturingProcessor) Invoke(ctx context.Context, trigger *taskexe.Trigger) error {
	c.trigger = trigger
	return nil
}

func (c *capturingProcessor) OnTaskRunCreated(ctx context.Context, param taskexe.OnTaskRunCreatedReq) error {
	return nil
}

func (c *capturingProcessor) OnTaskRunFinished(ctx context.Context, param taskexe.OnTaskRunFinishedReq) error {
	return nil
}

func (c *capturingProcessor) OnTaskFinished(ctx context.Context, param taskexe.OnTaskFinishedReq) error {
	return nil
}

func (c *capturingProcessor) OnTaskUpdated(ctx context.Context, currentTask *entity.ObservabilityTask, taskOp entity.TaskStatus) error {
	return nil
}

func (c *capturingProcessor) OnTaskCreated(ctx context.Context, currentTask *entity.ObservabilityTask) error {
	return nil
}

func TestSpanSubscriber_AddSpan_WithTrajectory(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	mockTraceService := service_mocks.NewMockITraceService(ctrl)
	processor := &capturingProcessor{}

	// Setup Task with Trajectory configuration
	task := &entity.ObservabilityTask{
		ID:          1,
		WorkspaceID: 7,
		TaskStatus:  entity.TaskStatusRunning,
		SpanFilter: &entity.SpanFilterFields{
			PlatformType: loop_span.PlatformCozeLoop,
			SpanListType: loop_span.SpanListTypeRootSpan,
		},
		TaskConfig: &entity.TaskConfig{
			DataReflowConfig: []*entity.DataReflowConfig{
				{
					FieldMappings: []dataset.FieldMapping{
						{
							FieldSchema: &dataset.FieldSchema{
								SchemaKey: ptr.Of(datadataset.SchemaKey_Trajectory),
							},
						},
					},
				},
			},
		},
	}

	sub := &spanSubscriber{
		taskID:       task.ID,
		t:            task,
		processor:    processor,
		taskRepo:     mockRepo,
		runType:      entity.TaskRunTypeNewData,
		traceService: mockTraceService,
		// buildHelper can be nil for this test as we don't test Match() logic deeply
	}

	// Mock TaskRun
	run := &entity.TaskRun{
		ID:          1001,
		TaskID:      task.ID,
		WorkspaceID: task.WorkspaceID,
		TaskType:    entity.TaskRunTypeNewData,
		RunStatus:   entity.TaskRunStatusRunning,
		RunStartAt:  time.Now().Add(-time.Hour),
		RunEndAt:    time.Now().Add(time.Hour),
	}
	mockRepo.EXPECT().GetLatestNewDataTaskRun(gomock.Any(), gomock.Nil(), task.ID).Return(run, nil)

	// Span
	span := &loop_span.Span{
		TraceID:   "trace123",
		SpanID:    "span123",
		StartTime: time.Now().UnixMilli(),
	}

	// Expect MergeHistoryMessagesByRespIDBatch
	mockTraceService.EXPECT().MergeHistoryMessagesByRespIDBatch(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	// Expect GetTrajectories
	expectedTrajectory := &loop_span.Trajectory{
		ID: ptr.Of("trace123"),
	}
	trajectoryMap := map[string]*loop_span.Trajectory{
		"trace123": expectedTrajectory,
	}
	mockTraceService.EXPECT().GetTrajectories(gomock.Any(), task.WorkspaceID, []string{"trace123"}, gomock.Any(), gomock.Any(), gomock.Any()).Return(trajectoryMap, nil)

	// Execute
	err := sub.AddSpan(context.Background(), span)
	assert.NoError(t, err)

	// Verify Trigger
	assert.NotNil(t, processor.trigger)
	assert.NotNil(t, processor.trigger.Trajectory)
	assert.Equal(t, expectedTrajectory, processor.trigger.Trajectory)
}

func TestSpanSubscriber_AddSpan_WithoutTrajectory(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	mockTraceService := service_mocks.NewMockITraceService(ctrl)
	processor := &capturingProcessor{}

	// Setup Task WITHOUT Trajectory configuration
	task := &entity.ObservabilityTask{
		ID:          2,
		WorkspaceID: 7,
		TaskStatus:  entity.TaskStatusRunning,
		SpanFilter: &entity.SpanFilterFields{
			PlatformType: loop_span.PlatformCozeLoop,
			SpanListType: loop_span.SpanListTypeRootSpan,
		},
		TaskConfig: &entity.TaskConfig{
			DataReflowConfig: []*entity.DataReflowConfig{
				{
					FieldMappings: []dataset.FieldMapping{
						{
							FieldSchema: &dataset.FieldSchema{
								SchemaKey: ptr.Of(datadataset.SchemaKey_String), // Not Trajectory
							},
						},
					},
				},
			},
		},
	}

	sub := &spanSubscriber{
		taskID:       task.ID,
		t:            task,
		processor:    processor,
		taskRepo:     mockRepo,
		runType:      entity.TaskRunTypeNewData,
		traceService: mockTraceService,
	}

	// Mock TaskRun
	run := &entity.TaskRun{
		ID:          1002,
		TaskID:      task.ID,
		WorkspaceID: task.WorkspaceID,
		TaskType:    entity.TaskRunTypeNewData,
		RunStatus:   entity.TaskRunStatusRunning,
		RunStartAt:  time.Now().Add(-time.Hour),
		RunEndAt:    time.Now().Add(time.Hour),
	}
	mockRepo.EXPECT().GetLatestNewDataTaskRun(gomock.Any(), gomock.Nil(), task.ID).Return(run, nil)

	// Span
	span := &loop_span.Span{
		TraceID:   "trace456",
		SpanID:    "span456",
		StartTime: time.Now().UnixMilli(),
	}

	// Expect MergeHistoryMessagesByRespIDBatch
	mockTraceService.EXPECT().MergeHistoryMessagesByRespIDBatch(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	// DO NOT Expect GetTrajectories (should not be called)

	// Execute
	err := sub.AddSpan(context.Background(), span)
	assert.NoError(t, err)

	// Verify Trigger
	assert.NotNil(t, processor.trigger)
	assert.Nil(t, processor.trigger.Trajectory)
}
