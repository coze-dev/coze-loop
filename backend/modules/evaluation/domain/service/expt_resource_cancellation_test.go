// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/stretchr/testify/require"
)

type cancellationExptStore struct {
	repo.IExperimentRepo
	expt    *entity.Experiment
	deleted bool
}

func (s *cancellationExptStore) GetByID(context.Context, int64, int64) (*entity.Experiment, error) {
	return s.expt, nil
}
func (s *cancellationExptStore) MGetByID(context.Context, []int64, int64) ([]*entity.Experiment, error) {
	return []*entity.Experiment{s.expt}, nil
}
func (s *cancellationExptStore) Update(_ context.Context, expt *entity.Experiment) error {
	s.expt.Status = expt.Status
	return nil
}
func (s *cancellationExptStore) Delete(context.Context, int64, int64) error {
	s.deleted = true
	return nil
}
func (s *cancellationExptStore) MDelete(context.Context, []int64, int64) error {
	s.deleted = true
	return nil
}

type cancellationResultService struct{ ExptResultService }

func (cancellationResultService) GetIncompleteTurns(context.Context, int64, int64, *entity.Session) ([]*entity.ItemTurnID, error) {
	return nil, nil
}
func (cancellationResultService) UpsertExptTurnResultFilter(context.Context, int64, int64, []int64) error {
	return nil
}

type cancellationTurnStore struct{ repo.IExptTurnResultRepo }

func (cancellationTurnStore) MGetItemTurnRunLogs(context.Context, int64, int64, []int64, int64) ([]*entity.ExptTurnResultRunLog, error) {
	return nil, nil
}
func (cancellationTurnStore) CreateOrUpdateItemsTurnRunLogStatus(context.Context, int64, int64, int64, []int64, entity.TurnRunState) error {
	return nil
}

type cancellationSandbox struct {
	IEvalTargetService
	stopErr error
	stopped bool
}

func (s *cancellationSandbox) TerminateSandboxAttemptsForRunItems(_ context.Context, _, _ int64, _ []int64, wait ...bool) error {
	if len(wait) == 0 || !wait[0] {
		return errors.New("stop must wait for real destruction")
	}
	if s.stopErr != nil {
		return s.stopErr
	}
	s.stopped = true
	return nil
}

type cancellationMutex struct{ lock.ILocker }

func (cancellationMutex) LockWithRenew(ctx context.Context, _ string, _, _ time.Duration) (bool, context.Context, func(), error) {
	return true, ctx, func() {}, nil
}
func (cancellationMutex) Unlock(string) (bool, error) { return true, nil }
func (s *resourceItemStore) ScanItemRunLogs(context.Context, int64, int64, *entity.ExptItemRunLogFilter, int64, int64, int64) ([]*entity.ExptItemResultRunLog, int64, error) {
	return nil, 0, nil
}
func (s *resourceItemStore) ScanItemRunLogsForCancellation(_ context.Context, _, _ int64, _ *int64, _ []int64, _, _ int64) ([]*entity.ExptItemResultRunLog, int64, error) {
	copy := *s.run
	return []*entity.ExptItemResultRunLog{&copy}, 0, nil
}
func (s *resourceItemStore) UpdateItemRunLogIfNonterminal(ctx context.Context, expt, run int64, ids []int64, fields map[string]any, space int64) error {
	if entity.IsItemRunFinished(entity.ItemRunState(s.run.Status)) {
		return nil
	}
	return s.UpdateItemRunLog(ctx, expt, run, ids, fields, space)
}
func cancellationFixture() (*ExptMangerImpl, *resourceItemStore, *resourceGuard, *cancellationExptStore, *cancellationSandbox) {
	_, items, guard, _, _ := resourceFixture()
	expts := &cancellationExptStore{expt: &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, TargetType: entity.EvalTargetTypeSandboxAgent, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: testScope}}
	sandbox := &cancellationSandbox{}
	mgr := &ExptMangerImpl{centralGuard: guard, exptRepo: expts, itemResultRepo: items, turnResultRepo: cancellationTurnStore{}, exptResultService: cancellationResultService{}, evalTargetService: sandbox, mutex: cancellationMutex{}}
	return mgr, items, guard, expts, sandbox
}
func (s *resourceItemStore) MGetItemRunLog(context.Context, int64, int64, []int64, int64) ([]*entity.ExptItemResultRunLog, error) {
	return []*entity.ExptItemResultRunLog{s.run}, nil
}
func (cancellationTurnStore) BatchGet(context.Context, int64, int64, []int64) ([]*entity.ExptTurnResult, error) {
	return nil, nil
}
func (cancellationResultService) CalculateStats(context.Context, int64, int64, *entity.Session) (*entity.ExptCalculateStats, error) {
	return nil, errors.New("stats reached before safe stop")
}
func TestResourceFinishingExperimentCannotStartLateItem(t *testing.T) {
	svc, _, guard, event, ctx := resourceFixture()
	guard.ready = true
	svc.experimentRepo = &cancellationExptStore{expt: &entity.Experiment{ID: 1, SpaceID: 3, Status: entity.ExptStatus_Terminating, ExptDispatchMode: entity.ExptDispatchModeEnforce}}
	started := false
	endpoint := svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error { started = true; return nil })
	require.NoError(t, endpoint(ctx, event))
	require.False(t, started, "the experiment cancellation barrier must be re-read within the Item mutex")
}

func TestResourceTerminalWithoutStopProofDoesNotFinalizeFromItemConsumer(t *testing.T) {
	svc, items, guard, event, _ := resourceFixture()
	items.run.Status = int32(entity.ItemRunState_Terminal)
	svc.endpoints = func(context.Context, *entity.ExptItemEvalEvent) error { return nil }
	require.NoError(t, svc.Eval(context.Background(), event))
	require.Empty(t, guard.finalized, "a failed cancellation's Terminal must not automatically release running budget")
}

func TestResourceCompleteExptStopsBeforeReleasingBudget(t *testing.T) {
	mgr, items, guard, _, sandbox := cancellationFixture()
	sandbox.stopErr = errors.New("running sandbox stop failed")
	require.ErrorIs(t, mgr.Kill(context.Background(), 1, nil, 3, "cancel", &entity.Session{}), sandbox.stopErr)
	require.Equal(t, int32(entity.ItemRunState_Terminal), items.run.Status)
	require.Empty(t, guard.finalized)
}

func TestResourceTerminateItemsRequiresReliableSandboxStop(t *testing.T) {
	mgr, items, guard, _, sandbox := cancellationFixture()
	sandbox.stopErr = errors.New("sandbox stop failed")
	require.ErrorIs(t, mgr.TerminateItems(context.Background(), 1, 2, 3, []int64{4}, &entity.Session{}), sandbox.stopErr)
	require.Equal(t, int32(entity.ItemRunState_Terminal), items.run.Status)
	require.Empty(t, guard.finalized)
}

func TestResourceDeleteRetriesStopWithoutRevertingTerminal(t *testing.T) {
	mgr, items, guard, expts, sandbox := cancellationFixture()
	sandbox.stopErr = errors.New("temporary stop failure")
	require.Error(t, mgr.Delete(context.Background(), 1, 3, &entity.Session{}))
	sandbox.stopErr = nil
	require.NoError(t, mgr.Delete(context.Background(), 1, 3, &entity.Session{}))
	require.True(t, sandbox.stopped)
	require.True(t, expts.deleted, "Cancel timeout must not undo proven Terminal or prevent soft-delete")
	require.Equal(t, int32(entity.ItemRunState_Terminal), items.run.Status)
	require.Len(t, guard.finalized, 1)
}
func TestResourceBatchDeleteBlocksOnTerminalWriteFailure(t *testing.T) {
	mgr, items, guard, expts, sandbox := cancellationFixture()
	items.writeErr = errors.New("primary Terminal write failed")
	require.ErrorIs(t, mgr.MDelete(context.Background(), []int64{1}, 3, &entity.Session{}), items.writeErr)
	require.False(t, expts.deleted)
	require.False(t, sandbox.stopped)
	require.Equal(t, int32(entity.ItemRunState_Queueing), items.run.Status)
	require.Empty(t, guard.finalized)
}
func TestResourceDeletePreservesCompletedItems(t *testing.T) {
	for _, state := range []entity.ItemRunState{entity.ItemRunState_Success, entity.ItemRunState_Fail} {
		mgr, items, guard, expts, sandbox := cancellationFixture()
		items.run.Status = int32(state)
		require.NoError(t, mgr.MDelete(context.Background(), []int64{1}, 3, &entity.Session{}))
		require.Equal(t, int32(state), items.run.Status)
		require.Empty(t, guard.finalized)
		require.False(t, sandbox.stopped)
		require.True(t, expts.deleted)
	}
}

func TestResourceDeleteBlocksUntilSandboxIsStopped(t *testing.T) {
	mgr, items, guard, expts, sandbox := cancellationFixture()
	sandbox.stopErr = errors.New("sandbox still running")
	require.ErrorIs(t, mgr.Delete(context.Background(), 1, 3, &entity.Session{}), sandbox.stopErr)
	require.False(t, expts.deleted)
	require.Equal(t, int32(entity.ItemRunState_Terminal), items.run.Status)
	require.Empty(t, guard.finalized, "Terminal alone is not permission to cancel held running budget")
}
