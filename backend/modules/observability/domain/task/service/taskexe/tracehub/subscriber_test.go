// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	gomock "go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	repo_mocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_processor"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type noopProcessor struct{ invoked bool }

func (n *noopProcessor) ValidateConfig(ctx context.Context, config any) error { return nil }
func (n *noopProcessor) Invoke(ctx context.Context, trigger *taskexe.Trigger) error {
	n.invoked = true
	return nil
}

func (n *noopProcessor) OnTaskRunCreated(ctx context.Context, param taskexe.OnTaskRunCreatedReq) error {
	return nil
}

func (n *noopProcessor) OnTaskRunFinished(ctx context.Context, param taskexe.OnTaskRunFinishedReq) error {
	return nil
}

func (n *noopProcessor) OnTaskFinished(ctx context.Context, param taskexe.OnTaskFinishedReq) error {
	return nil
}

func (n *noopProcessor) OnTaskUpdated(ctx context.Context, currentTask *entity.ObservabilityTask, taskOp entity.TaskStatus) error {
	return nil
}

func (n *noopProcessor) OnTaskCreated(ctx context.Context, currentTask *entity.ObservabilityTask) error {
	return nil
}

type fakeSpanFilter struct {
	basic []*loop_span.FilterField
	root  []*loop_span.FilterField
	llm   []*loop_span.FilterField
	all   []*loop_span.FilterField
	force bool
}

func (f *fakeSpanFilter) BuildBasicSpanFilter(ctx context.Context, env *span_filter.SpanEnv) ([]*loop_span.FilterField, bool, error) {
	return f.basic, f.force, nil
}

func (f *fakeSpanFilter) BuildRootSpanFilter(ctx context.Context, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return f.root, nil
}

func (f *fakeSpanFilter) BuildLLMSpanFilter(ctx context.Context, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return f.llm, nil
}

func (f *fakeSpanFilter) BuildALLSpanFilter(ctx context.Context, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return f.all, nil
}

type fakeBuilder struct{ f span_filter.Filter }

func (b *fakeBuilder) BuildPlatformRelatedFilter(ctx context.Context, pt loop_span.PlatformType) (span_filter.Filter, error) {
	return b.f, nil
}

func (b *fakeBuilder) BuildGetTraceProcessors(ctx context.Context, set span_processor.Settings) ([]span_processor.Processor, error) {
	return nil, nil
}

func (b *fakeBuilder) BuildListSpansProcessors(ctx context.Context, set span_processor.Settings) ([]span_processor.Processor, error) {
	return nil, nil
}

func (b *fakeBuilder) BuildAdvanceInfoProcessors(ctx context.Context, set span_processor.Settings) ([]span_processor.Processor, error) {
	return nil, nil
}

func (b *fakeBuilder) BuildIngestTraceProcessors(ctx context.Context, set span_processor.Settings) ([]span_processor.Processor, error) {
	return nil, nil
}

func (b *fakeBuilder) BuildSearchTraceOApiProcessors(ctx context.Context, set span_processor.Settings) ([]span_processor.Processor, error) {
	return nil, nil
}

func (b *fakeBuilder) BuildListSpansOApiProcessors(ctx context.Context, set span_processor.Settings) ([]span_processor.Processor, error) {
	return nil, nil
}

func TestSpanSubscriber_AddSpan_SkipNonRunning(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	proc := &noopProcessor{}

	task := &entity.ObservabilityTask{ID: 42, WorkspaceID: 7, TaskStatus: entity.TaskStatusRunning}
	sub := &spanSubscriber{
		taskID:    task.ID,
		t:         task,
		processor: proc,
		taskRepo:  mockRepo,
		runType:   entity.TaskRunTypeNewData,
	}

	run := &entity.TaskRun{
		ID:          1001,
		TaskID:      task.ID,
		WorkspaceID: task.WorkspaceID,
		TaskType:    entity.TaskRunTypeNewData,
		RunStatus:   entity.TaskRunStatusDone,
		RunStartAt:  time.Now().Add(-time.Minute),
		RunEndAt:    time.Now().Add(time.Minute),
	}
	mockRepo.EXPECT().GetLatestNewDataTaskRun(gomock.Any(), gomock.Nil(), task.ID).Return(run, nil)

	span := &loop_span.Span{TraceID: "trace", SpanID: "span", StartTime: time.Now().UnixMilli()}
	err := sub.AddSpan(context.Background(), span)
	assert.NoError(t, err)
	assert.False(t, proc.invoked, "Invoke should not be called for non-running TaskRun")
}

func TestSpanSubscriber_Match_PlatformAndTenant_Positive(t *testing.T) {
	t.Parallel()
	basic := []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldPSM,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"coze-loop"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
		},
	}
	f := &fakeSpanFilter{basic: basic, root: nil, force: true}
	task := &entity.ObservabilityTask{
		ID:          1,
		WorkspaceID: 7,
		TaskStatus:  entity.TaskStatusRunning,
		SpanFilter: &entity.SpanFilterFields{
			PlatformType: loop_span.PlatformCozeLoop,
			SpanListType: loop_span.SpanListTypeRootSpan,
		},
	}
	sub := &spanSubscriber{
		taskID:      task.ID,
		t:           task,
		processor:   &noopProcessor{},
		taskRepo:    nil,
		runType:     entity.TaskRunTypeNewData,
		buildHelper: &fakeBuilder{f: f},
		tenants:     []string{"tenant1", "tenant2"},
	}
	span := &loop_span.Span{
		SpanID:  "s1",
		TraceID: "t1",
		PSM:     "coze-loop",
		SystemTagsString: map[string]string{
			loop_span.SpanFieldTenant: "tenant1",
		},
		StartTime: time.Now().UnixMilli(),
	}
	matched, err := sub.Match(context.Background(), span)
	assert.NoError(t, err)
	assert.True(t, matched)
}

func TestSpanSubscriber_Match_NegativePSM(t *testing.T) {
	t.Parallel()
	basic := []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldPSM,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"coze-loop"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
		},
	}
	f := &fakeSpanFilter{basic: basic, root: nil, force: true}
	task := &entity.ObservabilityTask{
		ID:          2,
		WorkspaceID: 7,
		TaskStatus:  entity.TaskStatusRunning,
		SpanFilter: &entity.SpanFilterFields{
			PlatformType: loop_span.PlatformCozeLoop,
			SpanListType: loop_span.SpanListTypeRootSpan,
		},
	}
	sub := &spanSubscriber{
		taskID:      task.ID,
		t:           task,
		processor:   &noopProcessor{},
		taskRepo:    nil,
		runType:     entity.TaskRunTypeNewData,
		buildHelper: &fakeBuilder{f: f},
		tenants:     []string{"tenant1"},
	}
	span := &loop_span.Span{
		SpanID:  "s2",
		TraceID: "t2",
		PSM:     "other",
		SystemTagsString: map[string]string{
			loop_span.SpanFieldTenant: "tenant1",
		},
		StartTime: time.Now().UnixMilli(),
	}
	matched, err := sub.Match(context.Background(), span)
	assert.NoError(t, err)
	assert.False(t, matched)
}

func TestSpanSubscriber_Match_NegativeTenant(t *testing.T) {
	t.Parallel()
	basic := []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldPSM,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"coze-loop"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
		},
	}
	f := &fakeSpanFilter{basic: basic, root: nil, force: true}
	task := &entity.ObservabilityTask{
		ID:          3,
		WorkspaceID: 7,
		TaskStatus:  entity.TaskStatusRunning,
		SpanFilter: &entity.SpanFilterFields{
			PlatformType: loop_span.PlatformCozeLoop,
			SpanListType: loop_span.SpanListTypeRootSpan,
		},
	}
	sub := &spanSubscriber{
		taskID:      task.ID,
		t:           task,
		processor:   &noopProcessor{},
		taskRepo:    nil,
		runType:     entity.TaskRunTypeNewData,
		buildHelper: &fakeBuilder{f: f},
		tenants:     []string{"tenantX"},
	}
	span := &loop_span.Span{
		SpanID:  "s3",
		TraceID: "t3",
		PSM:     "coze-loop",
		SystemTagsString: map[string]string{
			loop_span.SpanFieldTenant: "tenant1",
		},
		StartTime: time.Now().UnixMilli(),
	}
	matched, err := sub.Match(context.Background(), span)
	assert.NoError(t, err)
	assert.False(t, matched)
}
