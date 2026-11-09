// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

// Package testutil supplies the original scheduler tick to external-package
// integration fixtures that call the blocking current-run retry API.
package testutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	redisc "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
)

func RetryRedis(t *testing.T) infraredis.Cmdable {
	t.Helper()
	addr := os.Getenv("EXPT_RETRY_REAL_REDIS_ADDR")
	require.NotEmpty(t, addr, "pending integration fixture requires Redis")
	client, err := infraredis.NewClient(&redisc.Options{Addr: addr})
	require.NoError(t, err)
	raw, ok := infraredis.Unwrap(client)
	require.True(t, ok)
	require.NoError(t, raw.Ping(context.Background()).Err())
	t.Cleanup(func() { _ = raw.Close() })
	return client
}

type pendingTickMode struct{ entity.ExptSchedulerMode }

var pendingConsumed = errors.New("test tick reached ScheduleStart after pending consumption")

func (*pendingTickMode) ExptStart(context.Context, *entity.ExptScheduleEvent, *entity.Experiment) error {
	return nil
}
func (*pendingTickMode) ScheduleStart(context.Context, *entity.ExptScheduleEvent, *entity.Experiment) error {
	return pendingConsumed
}
func (*pendingTickMode) NextTick(context.Context, *entity.ExptScheduleEvent, bool) error { return nil }

type pendingTickFactory struct{}

func (pendingTickFactory) NewSchedulerMode(entity.ExptRunMode) (entity.ExptSchedulerMode, error) {
	return &pendingTickMode{}, nil
}

// StartPendingTicks keeps source preparation, Redis registration/roundtrip,
// item leases, both SQL preflights and commit/receipt recovery real. These
// storage fixtures provide lightweight target/set display dependencies. The
// actual SQL-backed GetDetail and pending consumption run unchanged. Stop at
// ScheduleStart so
// the fixture's separately tested item execution is not run twice.
func StartPendingTicks(t *testing.T, manager service.IExptManager, port repo.IExptManualRetryPendingRepo, factory service.ManualRetryLockerFactory, event *entity.ExptScheduleEvent) {
	t.Helper()
	scheduler := service.NewExptSchedulerSvc(manager, nil, nil, nil, nil, nil, nil, nil, nil, nil, factory(), nil, nil, nil, nil, nil, nil, pendingTickFactory{}, nil, nil, nil, nil, nil).(*service.ExptSchedulerImpl)
	endpoint := scheduler.HandleEventLock(scheduler.HandleEventEndpoint(func(context.Context, *entity.ExptScheduleEvent) error { return nil }))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Millisecond):
			}
			pending, err := port.GetPendingRetry(ctx, event.SpaceID, event.ExptID, event.ExptRunID)
			if ctx.Err() != nil {
				return
			}
			require.NoError(t, err)
			if pending == nil {
				continue
			}
			copy := *event
			err = endpoint(ctx, &copy)
			if ctx.Err() != nil {
				return
			}
			require.ErrorIs(t, err, pendingConsumed, "real original tick must reach ScheduleStart, expt=%d run=%d", event.ExptID, event.ExptRunID)
		}
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Errorf("pending test tick did not stop: %s", fmt.Sprint(event.ExptRunID))
		}
	})
}

// Display dependencies preserve the manager's real GetDetail flow without
// inventing target execution records or replacing source readers.
type DisplayResult struct{ service.ExptResultService }

func (DisplayResult) MGetStats(context.Context, []int64, int64, *entity.Session) ([]*entity.ExptStats, error) {
	return nil, nil
}

type DisplayAggregate struct{ service.ExptAggrResultService }

func (DisplayAggregate) BatchGetExptAggrResultByExperimentIDs(context.Context, int64, []int64) ([]*entity.ExptAggregateResult, error) {
	return nil, nil
}

type DisplayVersions struct {
	service.EvaluationSetVersionService
	SetID int64
}

func (d DisplayVersions) GetEvaluationSetVersion(_ context.Context, space, version int64, _ *bool, _ *entity.SharedResourceOption) (*entity.EvaluationSetVersion, *entity.EvaluationSet, error) {
	return &entity.EvaluationSetVersion{ID: version}, &entity.EvaluationSet{ID: d.SetID, SpaceID: space}, nil
}

type DisplaySet struct{ service.IEvaluationSetService }

func (DisplaySet) GetEvaluationSet(_ context.Context, space *int64, set int64, _ *bool, _ *entity.SharedResourceOption) (*entity.EvaluationSet, error) {
	return &entity.EvaluationSet{ID: set}, nil
}

type DisplayTarget struct{ service.IEvalTargetService }

func (DisplayTarget) GetEvalTargetVersion(_ context.Context, space, version int64, _ bool) (*entity.EvalTarget, error) {
	return &entity.EvalTarget{SpaceID: space, EvalTargetVersion: &entity.EvalTargetVersion{ID: version}}, nil
}
