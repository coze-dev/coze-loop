// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"encoding/json"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
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
	createTaskRunErrSeq []error
	finishErrSeq        []error
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

func (s *stubProcessor) OnUpdateTaskChange(context.Context, *entity.ObservabilityTask, entity.TaskStatus) error {
	s.updateCallCount++
	return s.updateErr
}

func (s *stubProcessor) OnFinishTaskChange(_ context.Context, req taskexe.OnFinishTaskChangeReq) error {
	idx := len(s.finishChangeReqs)
	s.finishChangeReqs = append(s.finishChangeReqs, req)
	s.finishChangeInvoked++
	if idx < len(s.finishErrSeq) {
		return s.finishErrSeq[idx]
	}
	return s.finishErr
}

func (s *stubProcessor) OnCreateTaskRunChange(_ context.Context, req taskexe.OnCreateTaskRunChangeReq) error {
	s.createTaskRunReqs = append(s.createTaskRunReqs, req)
	idx := len(s.createTaskRunReqs) - 1
	if idx >= 0 && idx < len(s.createTaskRunErrSeq) {
		if err := s.createTaskRunErrSeq[idx]; err != nil {
			return err
		}
	}
	return s.createTaskRunErr
}

func (s *stubProcessor) OnFinishTaskRunChange(context.Context, taskexe.OnFinishTaskRunChangeReq) error {
	return s.finishTaskRunErr
}

type stubConfigLoader struct {
	values map[string]any
}

func newStubConfigLoader() *stubConfigLoader {
	return &stubConfigLoader{values: make(map[string]any)}
}

func (s *stubConfigLoader) Set(key string, value any) {
	s.values[key] = value
}

func (s *stubConfigLoader) Get(_ context.Context, key string) any {
	if s.values == nil {
		return nil
	}
	return s.values[key]
}

func (s *stubConfigLoader) UnmarshalKey(_ context.Context, key string, value any, _ ...conf.DecodeOptionFn) error {
	v, ok := s.values[key]
	if !ok {
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, value)
}

func (s *stubConfigLoader) Unmarshal(context.Context, any, ...conf.DecodeOptionFn) error {
	return nil
}

func newEnabledConsumerLoader() *stubConfigLoader {
	loader := newStubConfigLoader()
	loader.Set("consumer_listening", &config.ConsumerListening{
		IsEnabled:  true,
		IsAllSpace: true,
	})
	return loader
}
