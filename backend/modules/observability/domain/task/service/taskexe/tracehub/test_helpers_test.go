// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

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
	createTaskRunReqs   []taskexe.OnCreateTaskRunChangeReq
	finishChangeReqs    []taskexe.OnFinishTaskChangeReq
	updateCallCount     int
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
	s.updateCallCount++
	return s.updateErr
}

func (s *stubProcessor) OnFinishTaskChange(_ context.Context, req taskexe.OnFinishTaskChangeReq) error {
	s.finishChangeInvoked++
	s.finishChangeReqs = append(s.finishChangeReqs, req)
	return s.finishErr
}

func (s *stubProcessor) OnCreateTaskRunChange(_ context.Context, req taskexe.OnCreateTaskRunChangeReq) error {
	s.createTaskRunReqs = append(s.createTaskRunReqs, req)
	return s.createTaskRunErr
}

func (s *stubProcessor) OnFinishTaskRunChange(context.Context, taskexe.OnFinishTaskRunChangeReq) error {
	return s.finishTaskRunErr
}
