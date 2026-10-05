// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

type onlineCoordinatorPublisher struct {
	coordinatorSchedulePublisher
	f                   *coordinatorFixture
	prepared, published int
}

func (p *onlineCoordinatorPublisher) PrepareOnlinePlan(_ context.Context, key entity.HookRunKey) error {
	p.prepared++
	p.f.runs.run.PlanReady = true
	p.f.runs.run.State.Before.Activated = true
	return nil
}
func (p *onlineCoordinatorPublisher) PublishOnlineContinuation(_ context.Context, key entity.HookRunKey) error {
	p.published++
	return nil
}

func TestHookWorkerOnlineWaitingAndDrainingRecovery(t *testing.T) {
	f := newCoordinatorFixture(t)
	f.runs.run.Mode = entity.EvaluationModeAppend
	p := &onlineCoordinatorPublisher{f: f}
	f.deps.SchedulePublisher = p
	c := f.worker(t)
	require.NoError(t, c.PreparePlan(context.Background(), f.input()))
	require.Equal(t, 1, p.prepared)
	require.Zero(t, p.published)
	require.Len(t, f.wakes, 1)
	f.runs.run.State.Before.Status = entity.HookOperationSucceeded
	f.runs.run.State.Before.Activated = false
	f.runs.run.State.Gate = entity.HookGateReady
	f.runs.run.State.Status = entity.ExptStatus_Draining
	require.NoError(t, c.PreparePlan(context.Background(), f.input()))
	require.Equal(t, 1, p.published, "Draining must restore the original run and continue draining")
	require.Zero(t, f.prepared, "Online must not enter the offline plan selector")
	closed, err := entity.BeginHookFinalize(&f.runs.run.State, f.runs.run.State.Key, entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated})
	require.NoError(t, err)
	f.runs.run.State = closed.State
	require.NoError(t, c.PreparePlan(context.Background(), f.input()))
	require.Equal(t, 1, p.published)
}
