package tracehub

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	repo_mocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe/processor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	trace_service_mocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/mocks"
	span_filter_mocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/stretchr/testify/require"
)

func TestTraceHubServiceImpl_SpanTriggerSkipNoWorkspace(t *testing.T) {
	t.Parallel()

	impl := &TraceHubServiceImpl{}
	impl.taskCache.Store("ObjListWithTask", &TaskCacheInfo{})

	raw := &entity.RawSpan{
		TraceID: "trace",
		SpanID:  "span",
		LogID:   "log",
		Tags: map[string]any{
			"fornax_space_id": "space-1",
			"call_type":       "",
			"bot_id":          "bot-1",
		},
		SensitiveTags: &entity.SensitiveTags{},
		ServerEnv:     &entity.ServerInRawSpan{},
	}

	require.NoError(t, impl.SpanTrigger(context.Background(), raw))
}

func TestTraceHubServiceImpl_SpanTriggerDispatchError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	mockBuilder := trace_service_mocks.NewMockTraceFilterProcessorBuilder(ctrl)
	mockFilter := span_filter_mocks.NewMockFilter(ctrl)

	now := time.Now()
	workspaceID := int64(1)
	taskDO := &entity.ObservabilityTask{
		ID:          1,
		WorkspaceID: workspaceID,
		TaskType:    task.TaskTypeAutoEval,
		TaskStatus:  task.TaskStatusRunning,
		SpanFilter: &filter.SpanFilterFields{
			Filters:      &filter.FilterFields{FilterFields: []*filter.FilterField{}},
			PlatformType: ptr.Of(common.PlatformTypeLoopAll),
			SpanListType: ptr.Of(common.SpanListTypeAllSpan),
		},
		Sampler: &entity.Sampler{
			SampleRate: 1,
			SampleSize: 10,
			IsCycle:    false,
		},
		EffectiveTime: &entity.EffectiveTime{
			StartAt: now.Add(-time.Hour).UnixMilli(),
			EndAt:   now.Add(time.Hour).UnixMilli(),
		},
		TaskRuns: []*entity.TaskRun{
			{
				ID:          101,
				TaskID:      1,
				WorkspaceID: workspaceID,
				TaskType:    task.TaskRunTypeNewData,
				RunStatus:   task.TaskStatusRunning,
				RunStartAt:  now.Add(-30 * time.Minute),
				RunEndAt:    now.Add(30 * time.Minute),
			},
		},
	}

	mockRepo.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return([]*entity.ObservabilityTask{taskDO}, int64(0), nil)
	mockFilter.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return(nil, false, nil).AnyTimes()
	mockFilter.EXPECT().BuildALLSpanFilter(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	mockBuilder.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), gomock.Any()).Return(mockFilter, nil).AnyTimes()

	spanRun := &entity.TaskRun{
		ID:          201,
		TaskID:      1,
		WorkspaceID: workspaceID,
		TaskType:    task.TaskRunTypeNewData,
		RunStatus:   task.TaskStatusRunning,
		RunStartAt:  now.Add(-15 * time.Minute),
		RunEndAt:    now.Add(15 * time.Minute),
	}
	mockRepo.EXPECT().GetLatestNewDataTaskRun(gomock.Any(), gomock.AssignableToTypeOf(ptr.Of(int64(0))), int64(1)).Return(nil, nil)
	mockRepo.EXPECT().GetLatestNewDataTaskRun(gomock.Any(), gomock.Nil(), int64(1)).Return(spanRun, nil)

	proc := &stubProcessor{invokeErr: errors.New("invoke error"), createTaskRunErr: errors.New("create run error")}
	taskProcessor := processor.NewTaskProcessor()
	taskProcessor.Register(task.TaskTypeAutoEval, proc)

	impl := &TraceHubServiceImpl{
		taskRepo:      mockRepo,
		buildHelper:   mockBuilder,
		taskProcessor: taskProcessor,
	}
	impl.taskCache.Store("ObjListWithTask", &TaskCacheInfo{WorkspaceIDs: []string{"space-1"}})

	raw := &entity.RawSpan{
		TraceID:       "trace",
		SpanID:        "span",
		LogID:         "log",
		StartTimeInUs: now.UnixMicro(),
		Tags: map[string]any{
			"fornax_space_id": "space-1",
			"call_type":       "",
			"bot_id":          "bot-1",
		},
		SystemTags: map[string]any{
			loop_span.SpanFieldTenant: "tenant",
		},
		SensitiveTags: &entity.SensitiveTags{},
		ServerEnv:     &entity.ServerInRawSpan{},
	}

	err := impl.SpanTrigger(context.Background(), raw)
	require.Error(t, err)
	require.ErrorContains(t, err, "invoke error")
}
