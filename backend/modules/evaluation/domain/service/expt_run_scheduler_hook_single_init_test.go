// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

type singleHookInitializer struct {
	gate  *schedulerHookGate
	err   error
	calls int
}

func (i *singleHookInitializer) InitializeExecution(_ context.Context, key entity.HookRunKey, scope string) (entity.HookExecutionInitializationCompletion, error) {
	i.calls++
	if key != (entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}) || scope != "single" {
		return entity.HookExecutionInitializationCompletion{}, errors.New("wrong binding")
	}
	if i.err != nil {
		return entity.HookExecutionInitializationCompletion{}, i.err
	}
	i.gate.decision = entity.HookAdmissionDecision{Gate: entity.HookGateReady}
	return entity.HookExecutionInitializationCompletion{Initialized: true}, nil
}

func TestSingleHookSchedulerInitializesBeforeDispatch(t *testing.T) {
	for _, tc := range []struct {
		name, reason string
		gate         entity.HookGateState
		fail         bool
		calls        int
		wait         bool
	}{
		{name: "initialize", reason: "HOOK_EXECUTION_PENDING", gate: entity.HookGateWaiting, calls: 1},
		{name: "hook_not_done", reason: "HOOK_BEFORE_PENDING", gate: entity.HookGateWaiting, wait: true},
		{name: "legacy", gate: entity.HookGateReady},
		{name: "closed", gate: entity.HookGateClosed, wait: true},
		{name: "temporary_error", reason: "HOOK_EXECUTION_PENDING", gate: entity.HookGateWaiting, fail: true, calls: 1, wait: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gate := &schedulerHookGate{decision: entity.HookAdmissionDecision{Gate: tc.gate, Reason: tc.reason}}
			init := &singleHookInitializer{gate: gate}
			if tc.fail {
				init.err = errors.New("temporary")
			}
			pub := &schedulerHookPublisher{}
			s := &ExptSchedulerImpl{hookGate: gate, hookFrozenInitializer: init, hookSchedulerScope: "single", Publisher: pub}
			wait, err := s.waitForHookAdmission(context.Background(), &entity.ExptScheduleEvent{SpaceID: 1, ExptID: 2, ExptRunID: 3})
			require.NoError(t, err)
			require.Equal(t, tc.wait, wait)
			require.Equal(t, tc.calls, init.calls)
			if !tc.wait {
				require.Empty(t, pub.published)
			}
		})
	}
}
