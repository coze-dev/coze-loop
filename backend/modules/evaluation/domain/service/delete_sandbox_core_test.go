// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/stretchr/testify/require"
)

type deleteCoreTrace struct {
	sync.Mutex
	events []string
}

func (t *deleteCoreTrace) add(s string) { t.Lock(); defer t.Unlock(); t.events = append(t.events, s) }

type deleteCoreExpts struct {
	repo.IExperimentRepo
	row     *entity.Experiment
	trace   *deleteCoreTrace
	deleted int
}

func (r *deleteCoreExpts) GetByID(context.Context, int64, int64) (*entity.Experiment, error) {
	return r.row, nil
}

func (r *deleteCoreExpts) MGetByID(context.Context, []int64, int64) ([]*entity.Experiment, error) {
	return []*entity.Experiment{r.row}, nil
}

func (r *deleteCoreExpts) Delete(context.Context, int64, int64) error {
	r.deleted++
	r.trace.add("delete")
	return nil
}

func (r *deleteCoreExpts) MDelete(context.Context, []int64, int64) error {
	r.deleted++
	r.trace.add("delete")
	return nil
}

type deleteCoreSelection struct {
	ExptResultService
	calls int
	err   error
}

func (r *deleteCoreSelection) GetIncompleteTurns(context.Context, int64, int64, *entity.Session) ([]*entity.ItemTurnID, error) {
	r.calls++
	return []*entity.ItemTurnID{{ItemID: 1, TurnID: 1}}, r.err
}

type deleteCoreTurns struct {
	repo.IExptTurnResultRepo
	err error
}

func (r *deleteCoreTurns) MGetItemTurnRunLogs(context.Context, int64, int64, []int64, int64) ([]*entity.ExptTurnResultRunLog, error) {
	return []*entity.ExptTurnResultRunLog{{TargetResultID: 10}}, r.err
}

type deleteCoreRuns struct{ repo.IExptRunLogRepo }

func (*deleteCoreRuns) Get(context.Context, int64, int64) (*entity.ExptRunLog, error) {
	return &entity.ExptRunLog{ExptID: 701}, nil
}

type deleteCoreRecords struct {
	repo.IEvalTargetRepo
	record *entity.EvalTargetRecord
	trace  *deleteCoreTrace
	saves  int
}

func (r *deleteCoreRecords) ListItemRetryRecordsByRun(context.Context, int64, int64, []int64) (map[int64][]*entity.RetryRecord, error) {
	return nil, nil
}

func (r *deleteCoreRecords) ListEvalTargetRecordByIDsAndSpaceID(context.Context, int64, []int64) ([]*entity.EvalTargetRecord, error) {
	copy := *r.record
	return []*entity.EvalTargetRecord{&copy}, nil
}

func (*deleteCoreRecords) BatchGetEvalTargetVersion(context.Context, int64, []int64) ([]*entity.EvalTarget, error) {
	return []*entity.EvalTarget{{EvalTargetType: entity.EvalTargetTypeSandboxAgent, EvalTargetVersion: &entity.EvalTargetVersion{ID: 20}}}, nil
}

func (r *deleteCoreRecords) SaveEvalTargetRecord(_ context.Context, record *entity.EvalTargetRecord, _ *bool) error {
	copy := *record
	r.record = &copy
	r.saves++
	r.trace.add("save")
	return nil
}

type deleteCoreScheduler struct {
	rpc.ISandboxSchedulerAdapter
	trace                  *deleteCoreTrace
	calls                  int
	requests               []rpc.SandboxDestroyRequest
	failID                 string
	failFirst              bool
	entered, proceed, done chan struct{}
}

func (s *deleteCoreScheduler) Destroy(_ context.Context, r *rpc.SandboxDestroyRequest) (*rpc.SandboxDestroyResponse, error) {
	s.calls++
	s.requests = append(s.requests, *r)
	s.trace.add("cancel:" + r.ExecuteIDs[0])
	if s.entered != nil {
		close(s.entered)
		<-s.proceed
		close(s.done)
	}
	if (s.failFirst && s.calls == 1) || s.failID == r.ExecuteIDs[0] {
		return nil, errors.New("destroy failed")
	}
	return &rpc.SandboxDestroyResponse{AffectedCount: 1}, nil
}

func deleteCoreFixture() (*ExptMangerImpl, *deleteCoreExpts, *deleteCoreSelection, *deleteCoreTurns, *deleteCoreRecords, *deleteCoreScheduler) {
	trace := &deleteCoreTrace{}
	expts := &deleteCoreExpts{row: &entity.Experiment{ID: 701, SpaceID: 11, TargetSpaceID: 22, LatestRunID: 801, TargetType: entity.EvalTargetTypeSandboxAgent}, trace: trace}
	selection := &deleteCoreSelection{}
	turns := &deleteCoreTurns{}
	records := &deleteCoreRecords{trace: trace, record: &entity.EvalTargetRecord{ID: 10, SpaceID: 22, TargetVersionID: 20, ExperimentRunID: 801, Status: gptr.Of(entity.EvalTargetRunStatusAsyncInvoking), EvalTargetOutputData: &entity.EvalTargetOutputData{Ext: map[string]string{consts.OutputDataExtKeySandboxExecuteIDs: `["10-agent","10-orch"]`, "retained": "value"}}}}
	scheduler := &deleteCoreScheduler{trace: trace}
	target := &EvalTargetServiceImpl{evalTargetRepo: records, exptRunLogRepo: &deleteCoreRuns{}, sandboxSchedulerAdapter: scheduler, sandboxDestroyRetryBudget: time.Nanosecond}
	return &ExptMangerImpl{exptRepo: expts, exptResultService: selection, turnResultRepo: turns, evalTargetService: target}, expts, selection, turns, records, scheduler
}

func TestDeleteSandboxCoreCallsExistingCleanupBeforeSoftDelete(t *testing.T) {
	for _, batch := range []bool{false, true} {
		t.Run(map[bool]string{false: "single", true: "batch"}[batch], func(t *testing.T) {
			mgr, expts, _, _, records, scheduler := deleteCoreFixture()
			var err error
			if batch {
				err = mgr.MDelete(context.Background(), []int64{701}, 11, &entity.Session{})
			} else {
				err = mgr.Delete(context.Background(), 701, 11, &entity.Session{})
			}
			require.NoError(t, err)
			require.Equal(t, 1, expts.deleted)
			require.Equal(t, 2, scheduler.calls)
			for _, request := range scheduler.requests {
				require.EqualValues(t, 22, request.WorkspaceID)
				require.Equal(t, "701", request.TaskID)
			}
			require.Equal(t, []string{"cancel:10-agent", "cancel:10-orch", "save", "delete"}, expts.trace.events)
			require.Equal(t, entity.EvalTargetRunStatusFail, *records.record.Status)
		})
	}
}

func TestDeleteSandboxCoreFailureRetainsRecordsAndRetriesOriginalEntry(t *testing.T) {
	mgr, expts, _, _, records, scheduler := deleteCoreFixture()
	scheduler.failID = "10-orch"
	original := records.record.EvalTargetOutputData.Ext
	require.Error(t, mgr.Delete(context.Background(), 701, 11, &entity.Session{}))
	require.Zero(t, expts.deleted)
	require.Zero(t, records.saves)
	require.Equal(t, entity.EvalTargetRunStatusAsyncInvoking, *records.record.Status)
	require.Equal(t, original, records.record.EvalTargetOutputData.Ext)
	scheduler.failID = ""
	require.NoError(t, mgr.Delete(context.Background(), 701, 11, &entity.Session{}))
	require.Equal(t, 1, expts.deleted)
}

func TestDeleteSandboxCoreSelectionFailureDoesNotDelete(t *testing.T) {
	for _, turnQuery := range []bool{false, true} {
		t.Run(map[bool]string{false: "selection", true: "turns"}[turnQuery], func(t *testing.T) {
			mgr, expts, selection, turns, _, scheduler := deleteCoreFixture()
			if turnQuery {
				turns.err = errors.New("turn lookup failed")
			} else {
				selection.err = errors.New("selection failed")
			}
			require.Error(t, mgr.MDelete(context.Background(), []int64{701}, 11, &entity.Session{}))
			require.Zero(t, expts.deleted)
			require.Zero(t, scheduler.calls)
		})
	}
}

func TestDeleteSandboxCoreNonSandboxDoesNotLoadCleanup(t *testing.T) {
	mgr, expts, selection, _, _, scheduler := deleteCoreFixture()
	expts.row.TargetType = entity.EvalTargetTypeCustomRPCServer
	selection.err = errors.New("must not query")
	require.NoError(t, mgr.Delete(context.Background(), 701, 11, &entity.Session{}))
	require.Zero(t, selection.calls)
	require.Zero(t, scheduler.calls)
}

func TestBaselineDestroyRetryAcceptsReturnedErrors(t *testing.T) {
	_, _, _, _, records, scheduler := deleteCoreFixture()
	scheduler.failFirst = true
	target := &EvalTargetServiceImpl{evalTargetRepo: records, sandboxSchedulerAdapter: scheduler, sandboxDestroyRetryBudget: 500 * time.Millisecond}
	_ = target.destroySandboxExecuteSync(context.Background(), "701", 22, 22, []string{"10-agent"}, false)
	require.Equal(t, 2, scheduler.calls)
}

func TestDeleteSandboxCoreDefaultTerminationRemainsAsync(t *testing.T) {
	mgr, _, _, _, records, scheduler := deleteCoreFixture()
	records.record.EvalTargetOutputData.Ext[consts.OutputDataExtKeySandboxExecuteIDs] = `["10-agent"]`
	scheduler.entered = make(chan struct{})
	scheduler.proceed = make(chan struct{})
	scheduler.done = make(chan struct{})
	returned := make(chan struct{})
	go func() {
		_ = mgr.evalTargetService.TerminateAsyncRecordsAndDestroySandbox(context.Background(), 22, []int64{10}, 1, "cancel", false)
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(time.Second):
		close(scheduler.proceed)
		t.Fatal("default cancellation became synchronous")
	}
	<-scheduler.entered
	require.Equal(t, entity.EvalTargetRunStatusFail, *records.record.Status)
	close(scheduler.proceed)
	<-scheduler.done
}
