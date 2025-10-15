package tracehub

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe"
)

func floatPtr(v float64) *float64 { return &v }

func int64Ptr(v int64) *int64 { return &v }

func boolPtr(v bool) *bool { return &v }

type stubProcessor struct {
	invokeErr           error
	finishErr           error
	updateErr           error
	createTaskErr       error
	finishTaskRunErr    error
	validateErr         error
	createTaskRunErr    error
	finishChangeInvoked int
	invokeCalled        bool
}

func (s *stubProcessor) ValidateConfig(context.Context, any) error {
	return s.validateErr
}

func (s *stubProcessor) Invoke(context.Context, *taskexe.Trigger) error {
	s.invokeCalled = true
	return s.invokeErr
}

func (s *stubProcessor) OnCreateTaskChange(context.Context, *entity.ObservabilityTask) error {
	return s.createTaskErr
}

func (s *stubProcessor) OnUpdateTaskChange(context.Context, *entity.ObservabilityTask, string) error {
	return s.updateErr
}

func (s *stubProcessor) OnFinishTaskChange(context.Context, taskexe.OnFinishTaskChangeReq) error {
	s.finishChangeInvoked++
	return s.finishErr
}

func (s *stubProcessor) OnCreateTaskRunChange(context.Context, taskexe.OnCreateTaskRunChangeReq) error {
	return s.createTaskRunErr
}

func (s *stubProcessor) OnFinishTaskRunChange(context.Context, taskexe.OnFinishTaskRunChangeReq) error {
	return s.finishTaskRunErr
}

type errorBackfillProducer struct {
	called bool
	err    error
}

func (e *errorBackfillProducer) SendBackfill(context.Context, *entity.BackFillEvent) error {
	e.called = true
	if e.err != nil {
		return e.err
	}
	return nil
}
