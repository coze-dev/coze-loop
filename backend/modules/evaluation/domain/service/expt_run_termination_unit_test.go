// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/google/uuid"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// termDualRepo exposes one object that serves both the finish and the
// termination ports, matching the production wiring contract.
type termDualRepo struct {
	*repomocks.MockIExptRunFinishRepo
	*repomocks.MockIExptRunTerminationRepo
}

func termUnitSource(itemID int64, status entity.ItemRunState) *entity.ExptCurrentItemRunSource {
	return &entity.ExptCurrentItemRunSource{
		Current: entity.ExptItemCurrentRequest{
			SpaceID: 3, ExptID: 1, RunID: 2, ItemID: itemID, ItemVersionID: 7,
			RequestID: "req", Mode: entity.EvaluationModeFailRetry, Status: status,
		},
		Turns: map[int64]*entity.ExptTurnResultRunLog{},
	}
}

type termUnitDeps struct {
	ctrl        *gomock.Controller
	finish      *repomocks.MockIExptRunFinishRepo
	termination *repomocks.MockIExptRunTerminationRepo
	expts       *repomocks.MockIExperimentRepo
	items       *repomocks.MockIExptItemResultRepo
	locker      *lockmocks.MockILocker
	guard       *componentmocks.MockICentralReservationGuard
	targets     *svcmocks.MockIEvalTargetService
}

func newTermUnitDeps(t *testing.T) *termUnitDeps {
	t.Helper()
	ctrl := gomock.NewController(t)
	return &termUnitDeps{
		ctrl:        ctrl,
		finish:      repomocks.NewMockIExptRunFinishRepo(ctrl),
		termination: repomocks.NewMockIExptRunTerminationRepo(ctrl),
		expts:       repomocks.NewMockIExperimentRepo(ctrl),
		items:       repomocks.NewMockIExptItemResultRepo(ctrl),
		locker:      lockmocks.NewMockILocker(ctrl),
		guard:       componentmocks.NewMockICentralReservationGuard(ctrl),
		targets:     svcmocks.NewMockIEvalTargetService(ctrl),
	}
}

func (d *termUnitDeps) manager() *ExptMangerImpl {
	m := &ExptMangerImpl{
		runFinishRepo:     &termDualRepo{MockIExptRunFinishRepo: d.finish, MockIExptRunTerminationRepo: d.termination},
		manualRetryLocker: func() lock.ILocker { return d.locker },
		exptRepo:          d.expts,
		itemResultRepo:    d.items,
		centralGuard:      d.guard,
		evalTargetService: d.targets,
	}
	return m
}

func (d *termUnitDeps) expectLock(locked bool, lockCtx context.Context) {
	d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, _ any, _ any) (bool, context.Context, func(), error) {
			return locked, lockCtx, func() {}, nil
		}).AnyTimes()
}

func TestTerminationRepoResolution(t *testing.T) {
	d := newTermUnitDeps(t)
	m := d.manager()
	port, err := m.terminationRepo()
	require.NoError(t, err)
	require.NotNil(t, port)

	noPort := &ExptMangerImpl{manualRetryLocker: func() lock.ILocker { return d.locker }}
	_, err = noPort.terminationRepo()
	require.True(t, errno.NeedMQRetry(err))

	noLocker := &ExptMangerImpl{runFinishRepo: &termDualRepo{MockIExptRunTerminationRepo: d.termination}}
	_, err = noLocker.terminationRepo()
	require.True(t, errno.NeedMQRetry(err))
}

func TestWithTerminationRunLockBranches(t *testing.T) {
	t.Run("missing_factory", func(t *testing.T) {
		m := &ExptMangerImpl{}
		require.True(t, errno.NeedMQRetry(m.withTerminationRunLock(context.Background(), 1, 2, func(context.Context) error { return nil })))
	})
	t.Run("nil_locker", func(t *testing.T) {
		m := &ExptMangerImpl{manualRetryLocker: func() lock.ILocker { return nil }}
		require.True(t, errno.NeedMQRetry(m.withTerminationRunLock(context.Background(), 1, 2, func(context.Context) error { return nil })))
	})
	t.Run("lock_error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		locker := lockmocks.NewMockILocker(ctrl)
		locker.EXPECT().LockWithRenew(gomock.Any(), "expt_run_exec_lock:1:2", gomock.Any(), gomock.Any()).Return(false, nil, nil, errors.New("redis down"))
		m := &ExptMangerImpl{manualRetryLocker: func() lock.ILocker { return locker }}
		require.True(t, errno.NeedMQRetry(m.withTerminationRunLock(context.Background(), 1, 2, func(context.Context) error { return nil })))
	})
	t.Run("busy", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		locker := lockmocks.NewMockILocker(ctrl)
		locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(false, context.Background(), func() {}, nil)
		m := &ExptMangerImpl{manualRetryLocker: func() lock.ILocker { return locker }}
		require.True(t, errno.NeedMQRetry(m.withTerminationRunLock(context.Background(), 1, 2, func(context.Context) error { return nil })))
	})
	t.Run("lock_ctx_cancelled", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		locker := lockmocks.NewMockILocker(ctrl)
		cancelled, cancel := context.WithCancel(context.Background())
		cancel()
		locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, cancelled, func() {}, nil)
		m := &ExptMangerImpl{manualRetryLocker: func() lock.ILocker { return locker }}
		require.True(t, errno.NeedMQRetry(m.withTerminationRunLock(context.Background(), 1, 2, func(context.Context) error { return nil })))
	})
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		locker := lockmocks.NewMockILocker(ctrl)
		locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		m := &ExptMangerImpl{manualRetryLocker: func() lock.ILocker { return locker }}
		called := false
		require.NoError(t, m.withTerminationRunLock(context.Background(), 1, 2, func(context.Context) error { called = true; return nil }))
		require.True(t, called)
	})
}

func TestBeginCurrentRunTermination(t *testing.T) {
	t.Run("port_error", func(t *testing.T) {
		m := &ExptMangerImpl{}
		require.Error(t, m.beginCurrentRunTermination(context.Background(), 1, 2, 3))
	})
	t.Run("terminate_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expectLock(true, context.Background())
		d.termination.EXPECT().TerminateCurrentRun(gomock.Any(), gomock.Any()).Return(nil, errors.New("db down"))
		require.True(t, errno.NeedMQRetry(m.beginCurrentRunTermination(context.Background(), 1, 2, 3)))
	})
	t.Run("nil_result", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expectLock(true, context.Background())
		d.termination.EXPECT().TerminateCurrentRun(gomock.Any(), gomock.Any()).Return(nil, nil)
		require.True(t, errno.NeedMQRetry(m.beginCurrentRunTermination(context.Background(), 1, 2, 3)))
	})
	t.Run("non_current", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expectLock(true, context.Background())
		d.termination.EXPECT().TerminateCurrentRun(gomock.Any(), gomock.Any()).Return(&entity.ExptRunTerminationResult{NonCurrent: true}, nil)
		require.True(t, errno.NeedMQRetry(m.beginCurrentRunTermination(context.Background(), 1, 2, 3)))
	})
	t.Run("success", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expectLock(true, context.Background())
		d.termination.EXPECT().TerminateCurrentRun(gomock.Any(), gomock.Any()).Return(&entity.ExptRunTerminationResult{Applied: true}, nil)
		require.NoError(t, m.beginCurrentRunTermination(context.Background(), 1, 2, 3))
	})
}

func TestCaptureTerminationSources(t *testing.T) {
	d := newTermUnitDeps(t)
	m := d.manager()

	t.Run("invalid_id", func(t *testing.T) {
		_, err := m.captureTerminationSources(context.Background(), d.termination, 3, 1, 2, []int64{0})
		require.Error(t, err)
	})
	t.Run("source_error", func(t *testing.T) {
		d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), int64(3), int64(1), int64(2), int64(9)).Return(nil, errors.New("read failed"))
		_, err := m.captureTerminationSources(context.Background(), d.termination, 3, 1, 2, []int64{9})
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("missing_source", func(t *testing.T) {
		d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), int64(9)).Return(nil, nil)
		_, err := m.captureTerminationSources(context.Background(), d.termination, 3, 1, 2, []int64{9})
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("dedupe_and_sort", func(t *testing.T) {
		d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), int64(9)).Return(termUnitSource(9, entity.ItemRunState_Processing), nil)
		d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), int64(11)).Return(termUnitSource(11, entity.ItemRunState_Processing), nil)
		sources, err := m.captureTerminationSources(context.Background(), d.termination, 3, 1, 2, []int64{11, 9, 11})
		require.NoError(t, err)
		require.Len(t, sources, 2)
		require.Equal(t, int64(9), sources[0].Current.ItemID)
		require.Equal(t, int64(11), sources[1].Current.ItemID)
	})
}

func TestCommitTerminationItems(t *testing.T) {
	t.Run("terminal_items_skip_raw_logs", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expectLock(true, context.Background())
		d.termination.EXPECT().TerminateCurrentRun(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *entity.ExptRunTerminationRequest) (*entity.ExptRunTerminationResult, error) {
			require.Empty(t, req.MissingTurnLogs)
			return &entity.ExptRunTerminationResult{Applied: true}, nil
		})
		result, err := m.commitTerminationItems(context.Background(), d.termination, &entity.ExptRunTerminationRequest{
			SpaceID: 3, ExptID: 1, RunID: 2,
			Items: []*entity.ExptCurrentItemRunSource{
				termUnitSource(9, entity.ItemRunState_Success),
				termUnitSource(10, entity.ItemRunState_Fail),
			},
		})
		require.NoError(t, err)
		require.True(t, result.Applied)
	})
	t.Run("nil_locker", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := &ExptMangerImpl{manualRetryLocker: func() lock.ILocker { return nil }}
		_, err := m.commitTerminationItems(context.Background(), d.termination, &entity.ExptRunTerminationRequest{Items: []*entity.ExptCurrentItemRunSource{termUnitSource(9, entity.ItemRunState_Processing)}})
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("lock_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil, nil, errors.New("lock error"))
		_, err := m.commitTerminationItems(context.Background(), d.termination, &entity.ExptRunTerminationRequest{Items: []*entity.ExptCurrentItemRunSource{termUnitSource(9, entity.ItemRunState_Processing)}})
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("busy", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expectLock(false, context.Background())
		_, err := m.commitTerminationItems(context.Background(), d.termination, &entity.ExptRunTerminationRequest{Items: []*entity.ExptCurrentItemRunSource{termUnitSource(9, entity.ItemRunState_Processing)}})
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("cancelled_context", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := m.commitTerminationItems(ctx, d.termination, &entity.ExptRunTerminationRequest{Items: []*entity.ExptCurrentItemRunSource{termUnitSource(9, entity.ItemRunState_Processing)}})
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("terminate_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expectLock(true, context.Background())
		d.termination.EXPECT().TerminateCurrentRun(gomock.Any(), gomock.Any()).Return(nil, errors.New("commit failed"))
		_, err := m.commitTerminationItems(context.Background(), d.termination, &entity.ExptRunTerminationRequest{Items: []*entity.ExptCurrentItemRunSource{termUnitSource(9, entity.ItemRunState_Processing)}})
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("nil_result", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expectLock(true, context.Background())
		d.termination.EXPECT().TerminateCurrentRun(gomock.Any(), gomock.Any()).Return(nil, nil)
		_, err := m.commitTerminationItems(context.Background(), d.termination, &entity.ExptRunTerminationRequest{Items: []*entity.ExptCurrentItemRunSource{termUnitSource(9, entity.ItemRunState_Processing)}})
		require.True(t, errno.NeedMQRetry(err))
	})
}

func TestTerminateCurrentItems(t *testing.T) {
	t.Run("port_missing", func(t *testing.T) {
		m := &ExptMangerImpl{}
		require.Error(t, m.terminateCurrentItems(context.Background(), 1, 2, 3, []int64{9}, &entity.Session{}))
	})
	t.Run("capture_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		require.Error(t, m.terminateCurrentItems(context.Background(), 1, 2, 3, []int64{0}, &entity.Session{}))
	})
	t.Run("experiment_read_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), int64(9)).Return(termUnitSource(9, entity.ItemRunState_Processing), nil)
		d.expectLock(true, context.Background())
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(nil, errors.New("read failed"))
		require.True(t, errno.NeedMQRetry(m.terminateCurrentItems(context.Background(), 1, 2, 3, []int64{9}, &entity.Session{})))
	})
	t.Run("experiment_missing", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), int64(9)).Return(termUnitSource(9, entity.ItemRunState_Processing), nil)
		d.expectLock(true, context.Background())
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(nil, nil)
		require.True(t, errno.NeedMQRetry(m.terminateCurrentItems(context.Background(), 1, 2, 3, []int64{9}, &entity.Session{})))
	})
	t.Run("non_current_run", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), int64(9)).Return(termUnitSource(9, entity.ItemRunState_Processing), nil)
		d.expectLock(true, context.Background())
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 99, Status: entity.ExptStatus_Processing, ExptType: entity.ExptType_Offline}, nil)
		require.NoError(t, m.terminateCurrentItems(context.Background(), 1, 2, 3, []int64{9}, &entity.Session{}))
	})
	t.Run("unsupported_experiment", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), int64(9)).Return(termUnitSource(9, entity.ItemRunState_Processing), nil)
		d.expectLock(true, context.Background())
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Processing, ExptType: entity.ExptType_Online}, nil)
		require.True(t, errno.NeedMQRetry(m.terminateCurrentItems(context.Background(), 1, 2, 3, []int64{9}, &entity.Session{})))
	})
	t.Run("success", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), int64(9)).Return(termUnitSource(9, entity.ItemRunState_Processing), nil)
		d.expectLock(true, context.Background())
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Processing, ExptType: entity.ExptType_Offline}, nil)
		d.termination.EXPECT().TerminateCurrentRun(gomock.Any(), gomock.Any()).Return(&entity.ExptRunTerminationResult{Applied: true}, nil)
		require.NoError(t, m.terminateCurrentItems(context.Background(), 1, 2, 3, []int64{9}, &entity.Session{}))
	})
	t.Run("non_current_result", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), int64(9)).Return(termUnitSource(9, entity.ItemRunState_Processing), nil)
		d.expectLock(true, context.Background())
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Processing, ExptType: entity.ExptType_Offline}, nil)
		d.termination.EXPECT().TerminateCurrentRun(gomock.Any(), gomock.Any()).Return(&entity.ExptRunTerminationResult{NonCurrent: true}, nil)
		require.NoError(t, m.terminateCurrentItems(context.Background(), 1, 2, 3, []int64{9}, &entity.Session{}))
	})
}

func TestTerminateCurrentRunUnit(t *testing.T) {
	t.Run("port_missing", func(t *testing.T) {
		m := &ExptMangerImpl{}
		applied, err := m.TerminateCurrentRun(context.Background(), 1, 2, 3, &entity.Session{}, nil)
		require.False(t, applied)
		require.Error(t, err)
	})
	t.Run("lock_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil, nil, errors.New("lock"))
		applied, err := m.TerminateCurrentRun(context.Background(), 1, 2, 3, &entity.Session{}, nil)
		require.False(t, applied)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("experiment_read_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expectLock(true, context.Background())
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(nil, errors.New("read"))
		applied, err := m.TerminateCurrentRun(context.Background(), 1, 2, 3, &entity.Session{}, nil)
		require.False(t, applied)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("experiment_missing", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expectLock(true, context.Background())
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(nil, nil)
		applied, err := m.TerminateCurrentRun(context.Background(), 1, 2, 3, &entity.Session{}, nil)
		require.False(t, applied)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("non_current_run", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expectLock(true, context.Background())
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 99}, nil)
		applied, err := m.TerminateCurrentRun(context.Background(), 1, 2, 3, &entity.Session{}, nil)
		require.False(t, applied)
		require.NoError(t, err)
	})
	t.Run("expected_source_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expectLock(true, context.Background())
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2}, nil)
		d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), int64(3), int64(1), int64(2), int64(9)).Return(nil, errors.New("read"))
		applied, err := m.TerminateCurrentRun(context.Background(), 1, 2, 3, &entity.Session{}, termUnitSource(9, entity.ItemRunState_Processing))
		require.False(t, applied)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("expected_source_changed", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expectLock(true, context.Background())
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2}, nil)
		changed := termUnitSource(9, entity.ItemRunState_Processing)
		changed.Current.RequestID = "other"
		d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), int64(3), int64(1), int64(2), int64(9)).Return(changed, nil)
		applied, err := m.TerminateCurrentRun(context.Background(), 1, 2, 3, &entity.Session{}, termUnitSource(9, entity.ItemRunState_Processing))
		require.False(t, applied)
		require.NoError(t, err)
	})
	t.Run("scan_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expectLock(true, context.Background())
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2}, nil)
		d.items.EXPECT().ScanItemRunLogs(gomock.Any(), int64(1), int64(2), gomock.Any(), int64(0), int64(500), int64(3)).Return(nil, int64(0), errors.New("scan"))
		applied, err := m.TerminateCurrentRun(context.Background(), 1, 2, 3, &entity.Session{}, nil)
		require.False(t, applied)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("nil_log", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expectLock(true, context.Background())
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2}, nil)
		d.items.EXPECT().ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResultRunLog{nil}, int64(0), nil)
		applied, err := m.TerminateCurrentRun(context.Background(), 1, 2, 3, &entity.Session{}, nil)
		require.False(t, applied)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("scan_cursor_stalled", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expectLock(true, context.Background())
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2}, nil)
		d.items.EXPECT().ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResultRunLog{{ItemID: 9}}, int64(0), nil)
		d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), int64(9)).Return(termUnitSource(9, entity.ItemRunState_Processing), nil)
		d.expectLock(true, context.Background())
		d.termination.EXPECT().TerminateCurrentRun(gomock.Any(), gomock.Any()).Return(&entity.ExptRunTerminationResult{Applied: true, NonCurrent: true}, nil)
		applied, err := m.TerminateCurrentRun(context.Background(), 1, 2, 3, &entity.Session{}, nil)
		require.False(t, applied)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("cleanup_failure_after_apply", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expectLock(true, context.Background())
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2}, nil)
		d.items.EXPECT().ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
		d.termination.EXPECT().TerminateCurrentRun(gomock.Any(), gomock.Any()).Return(&entity.ExptRunTerminationResult{Applied: true, Experiment: &entity.Experiment{ID: 99}}, nil)
		applied, err := m.TerminateCurrentRun(context.Background(), 1, 2, 3, &entity.Session{}, nil)
		require.True(t, applied)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("non_current_with_expected", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expectLock(true, context.Background())
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2}, nil)
		expected := termUnitSource(9, entity.ItemRunState_Processing)
		d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), int64(3), int64(1), int64(2), int64(9)).Return(termUnitSource(9, entity.ItemRunState_Processing), nil)
		d.items.EXPECT().ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
		d.termination.EXPECT().TerminateCurrentRun(gomock.Any(), gomock.Any()).Return(&entity.ExptRunTerminationResult{NonCurrent: true}, nil)
		applied, err := m.TerminateCurrentRun(context.Background(), 1, 2, 3, &entity.Session{}, expected)
		require.False(t, applied)
		require.NoError(t, err)
	})
}

func TestReleaseTerminationResourcesUnit(t *testing.T) {
	t.Run("nil_source", func(t *testing.T) {
		m := &ExptMangerImpl{}
		require.Error(t, m.releaseTerminationResources(context.Background(), &entity.Experiment{}, 2, []*entity.ExptCurrentItemRunSource{nil}, false, &entity.Session{}))
	})
	t.Run("finished_skipped", func(t *testing.T) {
		m := &ExptMangerImpl{}
		require.NoError(t, m.releaseTerminationResources(context.Background(), &entity.Experiment{}, 2, []*entity.ExptCurrentItemRunSource{termUnitSource(9, entity.ItemRunState_Success)}, false, &entity.Session{}))
	})
	t.Run("cancelled", func(t *testing.T) {
		m := &ExptMangerImpl{}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		require.True(t, errno.NeedMQRetry(m.releaseTerminationResources(ctx, &entity.Experiment{}, 2, []*entity.ExptCurrentItemRunSource{termUnitSource(9, entity.ItemRunState_Processing)}, false, &entity.Session{})))
	})
	t.Run("central_scope_missing", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		err := m.releaseTerminationResources(context.Background(), &entity.Experiment{ExptDispatchMode: entity.ExptDispatchModeEnforce}, 2, []*entity.ExptCurrentItemRunSource{termUnitSource(9, entity.ItemRunState_Processing)}, false, &entity.Session{})
		require.Error(t, err)
	})
	t.Run("central_release_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.guard.EXPECT().Release(gomock.Any(), "scope", int64(2), int64(9), "req", gomock.Any()).Return(errors.New("redis"))
		err := m.releaseTerminationResources(context.Background(), &entity.Experiment{ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: "scope"}, 2, []*entity.ExptCurrentItemRunSource{termUnitSource(9, entity.ItemRunState_Processing)}, false, &entity.Session{})
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("target_records_released", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		source := termUnitSource(9, entity.ItemRunState_Processing)
		source.CanonicalTurns = []*entity.ExptTurnResult{{ID: 71, TurnID: 0, TargetResultID: 88}}
		d.targets.EXPECT().TerminateAsyncRecordsAndDestroySandbox(gomock.Any(), int64(3), []int64{88}, gomock.Any(), gomock.Any(), false).Return(errors.New("rpc failed"))
		require.NoError(t, m.releaseTerminationResources(context.Background(), &entity.Experiment{ID: 1, SpaceID: 3}, 2, []*entity.ExptCurrentItemRunSource{source}, false, &entity.Session{}))
	})
	t.Run("whole_releases_quota", func(t *testing.T) {
		d := newTermUnitDeps(t)
		quota := repomocks.NewMockQuotaRepo(d.ctrl)
		quota.EXPECT().CreateOrUpdate(gomock.Any(), int64(3), gomock.Any(), gomock.Any()).Return(nil)
		m := d.manager()
		m.quotaRepo = quota
		m.configer = nil
		require.NoError(t, m.releaseTerminationResources(context.Background(), &entity.Experiment{ID: 1, SpaceID: 3}, 2, nil, true, &entity.Session{}))
	})
	t.Run("whole_quota_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		quota := repomocks.NewMockQuotaRepo(d.ctrl)
		quota.EXPECT().CreateOrUpdate(gomock.Any(), int64(3), gomock.Any(), gomock.Any()).Return(errors.New("quota"))
		m := d.manager()
		m.quotaRepo = quota
		require.Error(t, m.releaseTerminationResources(context.Background(), &entity.Experiment{ID: 1, SpaceID: 3}, 2, nil, true, &entity.Session{}))
	})
}

func TestCleanupTerminatedRunUnit(t *testing.T) {
	expt := func() *entity.Experiment {
		return &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Terminated}
	}
	event := func() *entity.ExptScheduleEvent {
		return &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}
	}

	t.Run("frozen_experiment_mismatch", func(t *testing.T) {
		m := &ExptMangerImpl{}
		require.True(t, errno.NeedMQRetry(m.cleanupTerminatedRun(context.Background(), event(), &entity.Experiment{ID: 9}, nil, &entity.CompleteExptOption{})))
	})
	t.Run("nil_locker", func(t *testing.T) {
		m := &ExptMangerImpl{manualRetryLocker: func() lock.ILocker { return nil }}
		require.True(t, errno.NeedMQRetry(m.cleanupTerminatedRun(context.Background(), event(), expt(), nil, &entity.CompleteExptOption{})))
	})
	t.Run("unlock_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.locker.EXPECT().UnlockWithValue(gomock.Any(), "expt_run_mutex_lock:1", "2").Return(false, errors.New("unlock"))
		require.True(t, errno.NeedMQRetry(m.cleanupTerminatedRun(context.Background(), event(), expt(), nil, &entity.CompleteExptOption{})))
	})
	t.Run("cancelled_before_owner", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		d.locker.EXPECT().UnlockWithValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil, nil, errors.New("owner"))
		require.Error(t, m.cleanupTerminatedRun(ctx, event(), expt(), nil, &entity.CompleteExptOption{}))
	})
	t.Run("owner_unavailable", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.locker.EXPECT().UnlockWithValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(false, context.Background(), func() {}, nil)
		require.True(t, errno.NeedMQRetry(m.cleanupTerminatedRun(context.Background(), event(), expt(), nil, &entity.CompleteExptOption{})))
	})
	t.Run("experiment_missing", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.locker.EXPECT().UnlockWithValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(nil, nil)
		require.True(t, errno.NeedMQRetry(m.cleanupTerminatedRun(context.Background(), event(), expt(), nil, &entity.CompleteExptOption{})))
	})
	t.Run("latest_run_changed", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.locker.EXPECT().UnlockWithValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 99}, nil)
		require.NoError(t, m.cleanupTerminatedRun(context.Background(), event(), expt(), nil, &entity.CompleteExptOption{}))
	})
	t.Run("run_still_active", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.locker.EXPECT().UnlockWithValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		active := expt()
		active.Status = entity.ExptStatus_Processing
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(active, nil)
		require.True(t, errno.NeedMQRetry(m.cleanupTerminatedRun(context.Background(), event(), expt(), nil, &entity.CompleteExptOption{})))
	})
	t.Run("resource_release_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.locker.EXPECT().UnlockWithValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(expt(), nil)
		err := m.cleanupTerminatedRun(context.Background(), event(), expt(), []*entity.ExptCurrentItemRunSource{nil}, &entity.CompleteExptOption{})
		require.Error(t, err)
	})
	t.Run("idem_missing", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.locker.EXPECT().UnlockWithValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(expt(), nil)
		require.True(t, errno.NeedMQRetry(m.cleanupTerminatedRun(context.Background(), event(), expt(), nil, &entity.CompleteExptOption{NoCompleteItemTurn: true})))
	})
	t.Run("idem_exist_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		idem := idemmocks.NewMockIdempotentService(d.ctrl)
		idem.EXPECT().Exist(gomock.Any(), "finish-cleanup:1:2").Return(false, errors.New("idem"))
		m := d.manager()
		m.idem = idem
		d.locker.EXPECT().UnlockWithValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(expt(), nil)
		require.Error(t, m.cleanupTerminatedRun(context.Background(), event(), expt(), nil, &entity.CompleteExptOption{NoCompleteItemTurn: true}))
	})
	t.Run("already_notified", func(t *testing.T) {
		d := newTermUnitDeps(t)
		idem := idemmocks.NewMockIdempotentService(d.ctrl)
		idem.EXPECT().Exist(gomock.Any(), "finish-cleanup:1:2").Return(true, nil)
		m := d.manager()
		m.idem = idem
		d.locker.EXPECT().UnlockWithValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(expt(), nil)
		require.NoError(t, m.cleanupTerminatedRun(context.Background(), event(), expt(), nil, &entity.CompleteExptOption{NoCompleteItemTurn: true}))
	})
	t.Run("success", func(t *testing.T) {
		d := newTermUnitDeps(t)
		idem := idemmocks.NewMockIdempotentService(d.ctrl)
		idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil)
		idem.EXPECT().Set(gomock.Any(), "finish-cleanup:1:2", gomock.Any()).Return(nil)
		m := d.manager()
		m.idem = idem
		d.locker.EXPECT().UnlockWithValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(expt(), nil)
		require.NoError(t, m.cleanupTerminatedRun(context.Background(), event(), expt(), nil, &entity.CompleteExptOption{NoCompleteItemTurn: true}))
	})
}

func TestCleanupCancelledTerminationResourcesUnit(t *testing.T) {
	event := &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}
	frozen := &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Terminated}
	t.Run("nil_locker", func(t *testing.T) {
		m := &ExptMangerImpl{manualRetryLocker: func() lock.ILocker { return nil }}
		require.True(t, errno.NeedMQRetry(m.cleanupCancelledTerminationResources(context.Background(), event, frozen, nil)))
	})
	t.Run("lock_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil, nil, errors.New("lock"))
		require.True(t, errno.NeedMQRetry(m.cleanupCancelledTerminationResources(context.Background(), event, frozen, nil)))
	})
	t.Run("owner_unavailable", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(false, context.Background(), func() {}, nil)
		require.True(t, errno.NeedMQRetry(m.cleanupCancelledTerminationResources(context.Background(), event, frozen, nil)))
	})
	t.Run("release_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		require.Error(t, m.cleanupCancelledTerminationResources(context.Background(), event, frozen, []*entity.ExptCurrentItemRunSource{nil}))
	})
	t.Run("cancelled_returns_retry", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		require.True(t, errno.NeedMQRetry(m.cleanupCancelledTerminationResources(ctx, event, frozen, nil)))
	})
	t.Run("success", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		// The trailing unwrap always reports a retryable envelope, preserving replay.
		require.Error(t, m.cleanupCancelledTerminationResources(context.Background(), event, frozen, nil))
	})
}

func TestRefreshTerminatedRunFiltersUnit(t *testing.T) {
	event := &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}
	t.Run("port_missing", func(t *testing.T) {
		m := &ExptMangerImpl{}
		require.Error(t, m.refreshTerminatedRunFilters(context.Background(), event, nil))
	})
	t.Run("nil_source", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		require.True(t, errno.NeedMQRetry(m.refreshTerminatedRunFilters(context.Background(), event, []*entity.ExptCurrentItemRunSource{nil})))
	})
	t.Run("finished_skipped", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		require.NoError(t, m.refreshTerminatedRunFilters(context.Background(), event, []*entity.ExptCurrentItemRunSource{termUnitSource(9, entity.ItemRunState_Fail)}))
	})
	t.Run("legacy_service_missing", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		m.exptResultService = nil
		require.True(t, errno.NeedMQRetry(m.refreshTerminatedRunFilters(context.Background(), event, []*entity.ExptCurrentItemRunSource{termUnitSource(9, entity.ItemRunState_Processing)})))
	})
	t.Run("legacy_upsert_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		svc := svcmocks.NewMockExptResultService(d.ctrl)
		svc.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), int64(3), int64(1), []int64{9}).Return(errors.New("upsert"))
		m := d.manager()
		m.exptResultService = svc
		require.Error(t, m.refreshTerminatedRunFilters(context.Background(), event, []*entity.ExptCurrentItemRunSource{termUnitSource(9, entity.ItemRunState_Processing)}))
	})
	t.Run("legacy_upsert_success", func(t *testing.T) {
		d := newTermUnitDeps(t)
		svc := svcmocks.NewMockExptResultService(d.ctrl)
		svc.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), int64(3), int64(1), []int64{9}).Return(nil)
		m := d.manager()
		m.exptResultService = svc
		require.NoError(t, m.refreshTerminatedRunFilters(context.Background(), event, []*entity.ExptCurrentItemRunSource{termUnitSource(9, entity.ItemRunState_Processing)}))
	})
	t.Run("local_source_changed", func(t *testing.T) {
		d := newTermUnitDeps(t)
		local := &ExptResultServiceImpl{resultPublishLocker: func() lock.ILocker { return d.locker }}
		m := d.manager()
		m.exptResultService = local
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
		d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), int64(3), int64(1), int64(2), int64(9)).Return(nil, nil)
		require.True(t, errno.NeedMQRetry(m.refreshTerminatedRunFilters(context.Background(), event, []*entity.ExptCurrentItemRunSource{termUnitSource(9, entity.ItemRunState_Processing)})))
	})
	t.Run("local_read_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		local := &ExptResultServiceImpl{resultPublishLocker: func() lock.ILocker { return d.locker }}
		m := d.manager()
		m.exptResultService = local
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
		d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("read"))
		require.Error(t, m.refreshTerminatedRunFilters(context.Background(), event, []*entity.ExptCurrentItemRunSource{termUnitSource(9, entity.ItemRunState_Processing)}))
	})
}

func TestCleanupCurrentUnscheduledRunUnit(t *testing.T) {
	t.Run("invalid_run", func(t *testing.T) {
		m := &ExptMangerImpl{}
		require.Error(t, m.cleanupCurrentUnscheduledRun(context.Background(), 1, 0, 3, errors.New("cause")))
	})
	t.Run("experiment_read_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		m.mutex = d.locker
		d.locker.EXPECT().UnlockWithValue(gomock.Any(), "expt_run_mutex_lock:1", "2").Return(true, nil)
		d.expectLock(true, context.Background())
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(nil, errors.New("read"))
		require.Error(t, m.cleanupCurrentUnscheduledRun(context.Background(), 1, 2, 3, errors.New("cause")))
	})
	t.Run("latest_run_finished", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		m.mutex = d.locker
		d.locker.EXPECT().UnlockWithValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
		d.expectLock(true, context.Background())
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Success}, nil)
		require.NoError(t, m.cleanupCurrentUnscheduledRun(context.Background(), 1, 2, 3, errors.New("cause")))
	})
	t.Run("online_legacy_cleanup", func(t *testing.T) {
		d := newTermUnitDeps(t)
		runLogs := repomocks.NewMockIExptRunLogRepo(d.ctrl)
		runLogs.EXPECT().Update(gomock.Any(), int64(1), int64(2), int64(3), gomock.Any()).Return(nil)
		m := d.manager()
		m.mutex = d.locker
		m.runLogRepo = runLogs
		d.locker.EXPECT().UnlockWithValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()
		d.expectLock(true, context.Background())
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Processing, ExptType: entity.ExptType_Online}, nil)
		require.NoError(t, m.cleanupCurrentUnscheduledRun(context.Background(), 1, 2, 3, errors.New("cause")))
	})
	t.Run("pending_setup_abort", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		m.mutex = d.locker
		d.locker.EXPECT().UnlockWithValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
		d.expectLock(true, context.Background())
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Pending, ExptType: entity.ExptType_Offline}, nil)
		d.termination.EXPECT().TerminateCurrentRun(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *entity.ExptRunTerminationRequest) (*entity.ExptRunTerminationResult, error) {
			require.True(t, req.SetupAbort)
			return &entity.ExptRunTerminationResult{}, nil
		})
		require.NoError(t, m.cleanupCurrentUnscheduledRun(context.Background(), 1, 2, 3, errors.New("cause")))
	})
	t.Run("unlock_error_joined", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		m.mutex = d.locker
		d.locker.EXPECT().UnlockWithValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, errors.New("unlock"))
		d.expectLock(true, context.Background())
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 99}, nil)
		require.Error(t, m.cleanupCurrentUnscheduledRun(context.Background(), 1, 2, 3, errors.New("cause")))
	})
}

func TestReleaseCurrentSupersededRunQuotaUnit(t *testing.T) {
	t.Run("not_retry_mode", func(t *testing.T) {
		m := &ExptMangerImpl{}
		require.NoError(t, m.releaseCurrentSupersededRunQuota(context.Background(), 1, 3, 2, entity.EvaluationModeSubmit))
	})
	t.Run("experiment_read_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(nil, errors.New("read"))
		require.True(t, errno.NeedMQRetry(m.releaseCurrentSupersededRunQuota(context.Background(), 1, 3, 2, entity.EvaluationModeFailRetry)))
	})
	t.Run("experiment_missing", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(nil, nil)
		require.True(t, errno.NeedMQRetry(m.releaseCurrentSupersededRunQuota(context.Background(), 1, 3, 2, entity.EvaluationModeFailRetry)))
	})
	t.Run("non_central", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 5}, nil)
		require.NoError(t, m.releaseCurrentSupersededRunQuota(context.Background(), 1, 3, 2, entity.EvaluationModeFailRetry))
	})
	t.Run("scope_missing", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 5, ExptDispatchMode: entity.ExptDispatchModeEnforce}, nil)
		require.True(t, errno.NeedMQRetry(m.releaseCurrentSupersededRunQuota(context.Background(), 1, 3, 2, entity.EvaluationModeFailRetry)))
	})
	t.Run("scan_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 5, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: "scope"}, nil)
		d.items.EXPECT().ScanItemRunLogs(gomock.Any(), int64(1), int64(5), gomock.Any(), int64(0), int64(500), int64(3)).Return(nil, int64(0), errors.New("scan"))
		require.True(t, errno.NeedMQRetry(m.releaseCurrentSupersededRunQuota(context.Background(), 1, 3, 2, entity.EvaluationModeFailRetry)))
	})
	t.Run("nil_log", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 5, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: "scope"}, nil)
		d.items.EXPECT().ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResultRunLog{nil}, int64(0), nil)
		require.True(t, errno.NeedMQRetry(m.releaseCurrentSupersededRunQuota(context.Background(), 1, 3, 2, entity.EvaluationModeFailRetry)))
	})
	t.Run("source_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 5, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: "scope"}, nil)
		d.items.EXPECT().ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResultRunLog{{ItemID: 9}}, int64(0), nil)
		d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), int64(3), int64(1), int64(5), int64(9)).Return(nil, errors.New("read"))
		require.True(t, errno.NeedMQRetry(m.releaseCurrentSupersededRunQuota(context.Background(), 1, 3, 2, entity.EvaluationModeFailRetry)))
	})
	t.Run("source_missing", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 5, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: "scope"}, nil)
		d.items.EXPECT().ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResultRunLog{{ItemID: 9}}, int64(0), nil)
		d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		require.True(t, errno.NeedMQRetry(m.releaseCurrentSupersededRunQuota(context.Background(), 1, 3, 2, entity.EvaluationModeFailRetry)))
	})
	t.Run("release_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 5, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: "scope"}, nil)
		d.items.EXPECT().ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResultRunLog{{ItemID: 9}}, int64(0), nil)
		d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(termUnitSource(9, entity.ItemRunState_Processing), nil)
		d.guard.EXPECT().Release(gomock.Any(), "scope", int64(5), int64(9), "req", gomock.Any()).Return(errors.New("release"))
		require.True(t, errno.NeedMQRetry(m.releaseCurrentSupersededRunQuota(context.Background(), 1, 3, 2, entity.EvaluationModeFailRetry)))
	})
	t.Run("success", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 5, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: "scope"}, nil)
		d.items.EXPECT().ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResultRunLog{{ItemID: 9}}, int64(0), nil)
		d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(termUnitSource(9, entity.ItemRunState_Processing), nil)
		d.guard.EXPECT().Release(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		require.NoError(t, m.releaseCurrentSupersededRunQuota(context.Background(), 1, 3, 2, entity.EvaluationModeFailRetry))
	})
}

func TestResetItemsForNewRunUnit(t *testing.T) {
	t.Run("legacy_repo", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := &ExptMangerImpl{itemResultRepo: d.items}
		d.items.EXPECT().UpdateItemsResult(gomock.Any(), int64(3), int64(1), []int64{9}, map[string]any{"status": 1}).Return(nil)
		require.NoError(t, m.ResetItemsForNewRun(context.Background(), 3, 1, 2, []int64{9}, map[string]any{"status": 1}))
	})
	t.Run("cancelled", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		require.True(t, errno.NeedMQRetry(m.ResetItemsForNewRun(ctx, 3, 1, 2, []int64{9}, nil)))
	})
	t.Run("experiment_read_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(nil, errors.New("read"))
		require.True(t, errno.NeedMQRetry(m.ResetItemsForNewRun(context.Background(), 3, 1, 2, []int64{9}, nil)))
	})
	t.Run("experiment_missing", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(nil, nil)
		require.True(t, errno.NeedMQRetry(m.ResetItemsForNewRun(context.Background(), 3, 1, 2, []int64{9}, nil)))
	})
	t.Run("run_replaced", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 99}, nil)
		require.True(t, errno.NeedMQRetry(m.ResetItemsForNewRun(context.Background(), 3, 1, 2, []int64{9}, nil)))
	})
	t.Run("invalid_item", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2}, nil)
		require.Error(t, m.ResetItemsForNewRun(context.Background(), 3, 1, 2, []int64{0}, nil))
	})
	t.Run("success_dedupe", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2}, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.items.EXPECT().BatchGet(gomock.Any(), int64(3), int64(1), []int64{9}).Return([]*entity.ExptItemResult{{ItemID: 9, ExptRunID: 2}}, nil)
		d.items.EXPECT().UpdateItemsResult(gomock.Any(), int64(3), int64(1), []int64{9}, gomock.Any()).Return(nil)
		require.NoError(t, m.ResetItemsForNewRun(context.Background(), 3, 1, 2, []int64{9, 9}, nil))
	})
	t.Run("lock_busy", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2}, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(false, context.Background(), func() {}, nil)
		require.True(t, errno.NeedMQRetry(m.ResetItemsForNewRun(context.Background(), 3, 1, 2, []int64{9}, nil)))
	})
	t.Run("canonical_item_missing", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2}, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.items.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		require.True(t, errno.NeedMQRetry(m.ResetItemsForNewRun(context.Background(), 3, 1, 2, []int64{9}, nil)))
	})
	t.Run("central_guard_missing", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		m.centralGuard = nil
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: "scope"}, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.items.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResult{{ItemID: 9, ExptRunID: 1}}, nil)
		require.True(t, errno.NeedMQRetry(m.ResetItemsForNewRun(context.Background(), 3, 1, 2, []int64{9}, nil)))
	})
	t.Run("central_release_mismatch", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: "scope"}, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.items.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResult{{ItemID: 9, ExptRunID: 1}}, nil)
		d.guard.EXPECT().ReleaseTerminalReservation(gomock.Any(), "scope", int64(1), int64(9), "").Return(component.TerminalReservationMismatch, nil)
		require.True(t, errno.NeedMQRetry(m.ResetItemsForNewRun(context.Background(), 3, 1, 2, []int64{9}, nil)))
	})
	t.Run("central_release_absent", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: "scope"}, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.items.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResult{{ItemID: 9, ExptRunID: 1}}, nil)
		d.guard.EXPECT().ReleaseTerminalReservation(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(component.TerminalReservationAbsent, nil)
		d.items.EXPECT().UpdateItemsResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		require.NoError(t, m.ResetItemsForNewRun(context.Background(), 3, 1, 2, []int64{9}, nil))
	})
}

func TestUpdateNewRunItemResultsUnit(t *testing.T) {
	t.Run("resetter", func(t *testing.T) {
		d := newTermUnitDeps(t)
		manager := svcmocks.NewMockIExptManager(d.ctrl)
		resetter := svcmocks.NewMockIExptNewRunItemResetter(d.ctrl)
		resetter.EXPECT().ResetItemsForNewRun(gomock.Any(), int64(3), int64(1), int64(2), []int64{9}, map[string]any{"k": "v"}).Return(nil)
		// The concrete manager type is required for the resetter assertion.
		_ = manager
		combined := &termResetterManager{IExptManager: manager, resetter: resetter}
		require.NoError(t, updateNewRunItemResults(context.Background(), combined, d.items, &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2}, []int64{9}, map[string]any{"k": "v"}))
	})
	t.Run("plain_repo", func(t *testing.T) {
		d := newTermUnitDeps(t)
		manager := svcmocks.NewMockIExptManager(d.ctrl)
		d.items.EXPECT().UpdateItemsResult(gomock.Any(), int64(3), int64(1), []int64{9}, map[string]any{"k": "v"}).Return(nil)
		require.NoError(t, updateNewRunItemResults(context.Background(), manager, d.items, &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2}, []int64{9}, map[string]any{"k": "v"}))
	})
}

type termResetterManager struct {
	IExptManager
	resetter IExptNewRunItemResetter
}

func (m *termResetterManager) ResetItemsForNewRun(ctx context.Context, space, expt, newRun int64, ids []int64, fields map[string]any) error {
	return m.resetter.ResetItemsForNewRun(ctx, space, expt, newRun, ids, fields)
}

// TestTerminationRemainingBranches covers failure and edge branches that the
// real-MySQL fixtures do not reach.
func TestTerminationRemainingBranches(t *testing.T) {
	event := func() *entity.ExptScheduleEvent {
		return &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}
	}
	expt := func() *entity.Experiment {
		return &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Terminated}
	}

	t.Run("commit_item_ctx_cancelled", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		cancelled, cancel := context.WithCancel(context.Background())
		cancel()
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, cancelled, func() {}, nil)
		_, err := m.commitTerminationItems(context.Background(), d.termination, &entity.ExptRunTerminationRequest{Items: []*entity.ExptCurrentItemRunSource{termUnitSource(9, entity.ItemRunState_Processing)}})
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("commit_raw_turn_prepare_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expectLock(true, context.Background())
		source := termUnitSource(9, entity.ItemRunState_Processing)
		source.Current.RequestID = ""
		source.CanonicalTurns = []*entity.ExptTurnResult{{ID: 1, TurnID: 0}}
		_, err := m.commitTerminationItems(context.Background(), d.termination, &entity.ExptRunTerminationRequest{Items: []*entity.ExptCurrentItemRunSource{source}})
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("terminate_current_items_release_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), int64(9)).Return(termUnitSource(9, entity.ItemRunState_Processing), nil)
		d.expectLock(true, context.Background())
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Processing, ExptType: entity.ExptType_Offline, ExptDispatchMode: entity.ExptDispatchModeEnforce}, nil)
		d.termination.EXPECT().TerminateCurrentRun(gomock.Any(), gomock.Any()).Return(&entity.ExptRunTerminationResult{Applied: true}, nil)
		require.Error(t, m.terminateCurrentItems(context.Background(), 1, 2, 3, []int64{9}, &entity.Session{}))
	})
	t.Run("scan_cursor_advance_then_stall", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expectLock(true, context.Background())
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2}, nil)
		d.items.EXPECT().ScanItemRunLogs(gomock.Any(), int64(1), int64(2), gomock.Any(), int64(0), int64(500), int64(3)).Return([]*entity.ExptItemResultRunLog{{ItemID: 9}}, int64(5), nil)
		d.items.EXPECT().ScanItemRunLogs(gomock.Any(), int64(1), int64(2), gomock.Any(), int64(5), int64(500), int64(3)).Return([]*entity.ExptItemResultRunLog{{ItemID: 9}}, int64(5), nil)
		_, err := m.TerminateCurrentRun(context.Background(), 1, 2, 3, &entity.Session{}, nil)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("capture_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expectLock(true, context.Background())
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2}, nil)
		d.items.EXPECT().ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResultRunLog{{ItemID: 9}}, int64(0), nil)
		d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), int64(9)).Return(nil, errors.New("read"))
		_, err := m.TerminateCurrentRun(context.Background(), 1, 2, 3, &entity.Session{}, nil)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("commit_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expectLock(true, context.Background())
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2}, nil)
		d.items.EXPECT().ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResultRunLog{{ItemID: 9}}, int64(0), nil)
		d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), int64(9)).Return(termUnitSource(9, entity.ItemRunState_Processing), nil)
		d.termination.EXPECT().TerminateCurrentRun(gomock.Any(), gomock.Any()).Return(nil, errors.New("commit"))
		_, err := m.TerminateCurrentRun(context.Background(), 1, 2, 3, &entity.Session{}, nil)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("terminated_experiment_missing", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expectLock(true, context.Background())
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2}, nil)
		d.items.EXPECT().ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
		d.termination.EXPECT().TerminateCurrentRun(gomock.Any(), gomock.Any()).Return(&entity.ExptRunTerminationResult{Applied: true}, nil)
		_, err := m.TerminateCurrentRun(context.Background(), 1, 2, 3, &entity.Session{}, nil)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("target_release_cancelled", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		ctx, cancel := context.WithCancel(context.Background())
		source := termUnitSource(9, entity.ItemRunState_Processing)
		source.CanonicalTurns = []*entity.ExptTurnResult{{ID: 71, TurnID: 0, TargetResultID: 88}}
		d.targets.EXPECT().TerminateAsyncRecordsAndDestroySandbox(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), false).DoAndReturn(func(context.Context, int64, []int64, int32, string, bool, ...bool) error {
			cancel()
			return nil
		})
		require.True(t, errno.NeedMQRetry(m.releaseTerminationResources(ctx, &entity.Experiment{ID: 1, SpaceID: 3}, 2, []*entity.ExptCurrentItemRunSource{source}, false, &entity.Session{})))
	})
	t.Run("cleanup_lock_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.locker.EXPECT().UnlockWithValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil, nil, errors.New("lock"))
		require.True(t, errno.NeedMQRetry(m.cleanupTerminatedRun(context.Background(), event(), expt(), nil, &entity.CompleteExptOption{NoCompleteItemTurn: true})))
	})
	t.Run("cleanup_read_error_while_cancelled", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		ctx, cancel := context.WithCancel(context.Background())
		d.locker.EXPECT().UnlockWithValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, ctx, func() {}, nil).Times(2)
		d.locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).DoAndReturn(func(context.Context, int64, int64) (*entity.Experiment, error) {
			cancel()
			return nil, errors.New("read")
		})
		require.Error(t, m.cleanupTerminatedRun(ctx, event(), expt(), nil, &entity.CompleteExptOption{NoCompleteItemTurn: true}))
	})
	t.Run("cleanup_release_error_while_cancelled", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		ctx, cancel := context.WithCancel(context.Background())
		d.locker.EXPECT().UnlockWithValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, ctx, func() {}, nil).Times(2)
		d.locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
		current := expt()
		current.ExptDispatchMode = entity.ExptDispatchModeEnforce
		current.SchedulerScope = "scope"
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(current, nil)
		d.guard.EXPECT().Release(gomock.Any(), "scope", int64(2), int64(9), "req", gomock.Any()).DoAndReturn(func(context.Context, string, int64, int64, string, string) error {
			cancel()
			return errors.New("release")
		})
		err := m.cleanupTerminatedRun(ctx, event(), expt(), []*entity.ExptCurrentItemRunSource{termUnitSource(9, entity.ItemRunState_Processing)}, &entity.CompleteExptOption{NoCompleteItemTurn: true})
		require.Error(t, err)
	})
	t.Run("cleanup_template_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		idem := idemmocks.NewMockIdempotentService(d.ctrl)
		idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil)
		template := svcmocks.NewMockIExptTemplateManager(d.ctrl)
		template.EXPECT().UpdateExptInfo(gomock.Any(), int64(4), int64(3), int64(1), gomock.Any(), int64(0), gomock.Any()).Return(errors.New("template"))
		m := d.manager()
		m.idem = idem
		m.templateManager = template
		current := expt()
		current.ExptTemplateMeta = &entity.ExptTemplateMeta{ID: 4}
		d.locker.EXPECT().UnlockWithValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(current, nil)
		require.Error(t, m.cleanupTerminatedRun(context.Background(), event(), expt(), nil, &entity.CompleteExptOption{NoCompleteItemTurn: true}))
	})
	t.Run("cleanup_aggr_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		idem := idemmocks.NewMockIdempotentService(d.ctrl)
		idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil)
		aggr := svcmocks.NewMockExptAggrResultService(d.ctrl)
		aggr.EXPECT().PublishExptAggrResultEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("aggr"))
		m := d.manager()
		m.idem = idem
		m.exptAggrResultService = aggr
		d.locker.EXPECT().UnlockWithValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(expt(), nil)
		require.Error(t, m.cleanupTerminatedRun(context.Background(), event(), expt(), nil, &entity.CompleteExptOption{NoCompleteItemTurn: true}))
	})
	t.Run("cleanup_workflow_notify", func(t *testing.T) {
		d := newTermUnitDeps(t)
		idem := idemmocks.NewMockIdempotentService(d.ctrl)
		idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil)
		idem.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		pipeline := rpcmocks.NewMockIPipelineListAdapter(d.ctrl)
		pipeline.EXPECT().PipelineNodeFinishCallback(gomock.Any(), int64(1), int64(3)).Return(errors.New("pipeline"))
		m := d.manager()
		m.idem = idem
		m.pipelineListAdapter = pipeline
		current := expt()
		current.SourceType = entity.SourceType_Workflow
		d.locker.EXPECT().UnlockWithValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(current, nil)
		require.NoError(t, m.cleanupTerminatedRun(context.Background(), event(), expt(), nil, &entity.CompleteExptOption{NoCompleteItemTurn: true}))
	})
	t.Run("refresh_local_source_changed", func(t *testing.T) {
		d := newTermUnitDeps(t)
		local := &ExptResultServiceImpl{resultPublishLocker: func() lock.ILocker { return d.locker }}
		m := d.manager()
		m.exptResultService = local
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
		changed := termUnitSource(9, entity.ItemRunState_Terminal)
		changed.Current.RequestID = "other"
		d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), int64(9)).Return(changed, nil)
		require.True(t, errno.NeedMQRetry(m.refreshTerminatedRunFilters(context.Background(), event(), []*entity.ExptCurrentItemRunSource{termUnitSource(9, entity.ItemRunState_Processing)})))
	})
	t.Run("unscheduled_experiment_missing", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		m.mutex = d.locker
		d.locker.EXPECT().UnlockWithValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
		d.expectLock(true, context.Background())
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(nil, nil)
		require.Error(t, m.cleanupCurrentUnscheduledRun(context.Background(), 1, 2, 3, errors.New("cause")))
	})
	t.Run("unscheduled_pending_port_missing", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := &ExptMangerImpl{runFinishRepo: d.finish, manualRetryLocker: func() lock.ILocker { return d.locker }, exptRepo: d.expts, mutex: d.locker}
		d.locker.EXPECT().UnlockWithValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
		d.expectLock(true, context.Background())
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Pending, ExptType: entity.ExptType_Offline}, nil)
		require.Error(t, m.cleanupCurrentUnscheduledRun(context.Background(), 1, 2, 3, errors.New("cause")))
	})
	t.Run("unscheduled_processing_terminates", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		m.mutex = d.locker
		d.locker.EXPECT().UnlockWithValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil, nil, errors.New("second lock"))
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Processing, ExptType: entity.ExptType_Offline}, nil)
		require.Error(t, m.cleanupCurrentUnscheduledRun(context.Background(), 1, 2, 3, errors.New("cause")))
	})
	t.Run("superseded_port_missing", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := &ExptMangerImpl{runFinishRepo: d.finish, manualRetryLocker: func() lock.ILocker { return d.locker }, exptRepo: d.expts, itemResultRepo: d.items}
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 5, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: "scope"}, nil)
		require.Error(t, m.releaseCurrentSupersededRunQuota(context.Background(), 1, 3, 2, entity.EvaluationModeFailRetry))
	})
	t.Run("superseded_cursor_stalled", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 5, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: "scope"}, nil)
		d.items.EXPECT().ScanItemRunLogs(gomock.Any(), int64(1), int64(5), gomock.Any(), int64(0), int64(500), int64(3)).Return([]*entity.ExptItemResultRunLog{{ItemID: 9}}, int64(5), nil)
		d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(termUnitSource(9, entity.ItemRunState_Processing), nil).AnyTimes()
		d.items.EXPECT().ScanItemRunLogs(gomock.Any(), int64(1), int64(5), gomock.Any(), int64(5), int64(500), int64(3)).Return([]*entity.ExptItemResultRunLog{{ItemID: 9}}, int64(5), nil)
		require.True(t, errno.NeedMQRetry(m.releaseCurrentSupersededRunQuota(context.Background(), 1, 3, 2, entity.EvaluationModeFailRetry)))
	})
	t.Run("reset_locker_missing", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		m.manualRetryLocker = func() lock.ILocker { return nil }
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2}, nil)
		require.True(t, errno.NeedMQRetry(m.ResetItemsForNewRun(context.Background(), 3, 1, 2, []int64{9}, nil)))
	})
	t.Run("reset_lock_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2}, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil, nil, errors.New("lock"))
		require.True(t, errno.NeedMQRetry(m.ResetItemsForNewRun(context.Background(), 3, 1, 2, []int64{9}, nil)))
	})
	t.Run("reset_item_ctx_cancelled", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		cancelled, cancel := context.WithCancel(context.Background())
		cancel()
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2}, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, cancelled, func() {}, nil)
		require.True(t, errno.NeedMQRetry(m.ResetItemsForNewRun(context.Background(), 3, 1, 2, []int64{9}, nil)))
	})
	t.Run("reset_batch_get_error", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2}, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.items.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("batch"))
		require.True(t, errno.NeedMQRetry(m.ResetItemsForNewRun(context.Background(), 3, 1, 2, []int64{9}, nil)))
	})
	t.Run("reset_manual_context_invalid", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: "scope"}, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.items.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResult{{ItemID: 9, ExptRunID: 1, Ext: map[string]string{entity.ManualRetryExtKey: "not-json"}}}, nil)
		require.True(t, errno.NeedMQRetry(m.ResetItemsForNewRun(context.Background(), 3, 1, 2, []int64{9}, nil)))
	})
	t.Run("reset_token_run_mismatch", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: "scope"}, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		ext := map[string]string{entity.ManualRetryExtKey: `{"run_id":99,"request_id":"req","mode":2}`}
		d.items.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResult{{ItemID: 9, ExptRunID: 1, Ext: ext}}, nil)
		require.True(t, errno.NeedMQRetry(m.ResetItemsForNewRun(context.Background(), 3, 1, 2, []int64{9}, nil)))
	})
	t.Run("reset_ctx_cancelled_before_update", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		var itemCancel func()
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2}, nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ string, _ any, _ any) (bool, context.Context, func(), error) {
			lockCtx, cancel := context.WithCancel(context.Background())
			itemCancel = cancel
			return true, lockCtx, func() {}, nil
		})
		d.items.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, int64, int64, []int64) ([]*entity.ExptItemResult, error) {
			itemCancel()
			return []*entity.ExptItemResult{{ItemID: 9, ExptRunID: 2}}, nil
		})
		require.True(t, errno.NeedMQRetry(m.ResetItemsForNewRun(context.Background(), 3, 1, 2, []int64{9}, nil)))
	})
}

func manualRetryUnitExperiment() *entity.Experiment {
	return &entity.Experiment{
		ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Processing, ExptType: entity.ExptType_Offline,
		EvalSetSourceType: entity.ExptEvalSetSourceType_SingleSet,
		EvalSetID:         5, EvalSetVersionID: 6, EvalSetSpaceID: 3,
		TargetID: 7, TargetVersionID: 8, TargetSpaceID: 3,
	}
}

func manualRetryUnitItem() *entity.ExptItemResult {
	return &entity.ExptItemResult{ItemID: 9, SpaceID: 3, ExptID: 1, ExptRunID: 1, ItemVersionID: 7, Status: entity.ItemRunState_Success}
}

func manualRetryUnitRequest(requestID string) *entity.RetryInCurrentRunRequest {
	return &entity.RetryInCurrentRunRequest{SpaceID: 3, ExptID: 1, ExpectedRunID: 2, RequestID: requestID, Mode: entity.EvaluationModeFailRetry}
}

func TestManualRetryNormalizeAndValidateUnit(t *testing.T) {
	t.Run("normalize_nil", func(t *testing.T) {
		_, _, err := normalizeCurrentRunRetry(nil)
		require.Error(t, err)
	})
	t.Run("normalize_dedupes_and_sorts", func(t *testing.T) {
		req := &entity.RetryInCurrentRunRequest{SpaceID: 3, ExptID: 1, ExpectedRunID: 2, RequestID: uuid.NewString(), Mode: entity.EvaluationModeRetryItems, ItemIDs: []int64{11, 9, 11}, Session: &entity.Session{UserID: "u"}}
		normalized, hash, err := normalizeCurrentRunRetry(req)
		require.NoError(t, err)
		require.Equal(t, []int64{9, 11}, normalized.ItemIDs)
		require.Len(t, hash, 64)
	})
	for name, mutate := range map[string]func(*entity.RetryInCurrentRunRequest){
		"zero_identity": func(r *entity.RetryInCurrentRunRequest) { r.SpaceID = 0 },
		"bad_uuid":      func(r *entity.RetryInCurrentRunRequest) { r.RequestID = "not-a-uuid" },
		"bad_mode":      func(r *entity.RetryInCurrentRunRequest) { r.Mode = entity.EvaluationModeRetryAll },
		"fail_with_items": func(r *entity.RetryInCurrentRunRequest) {
			r.Mode = entity.EvaluationModeFailRetry
			r.ItemIDs = []int64{9}
		},
		"items_empty": func(r *entity.RetryInCurrentRunRequest) { r.Mode = entity.EvaluationModeRetryItems },
		"invalid_item": func(r *entity.RetryInCurrentRunRequest) {
			r.Mode = entity.EvaluationModeRetryItems
			r.ItemIDs = []int64{0}
		},
		"internal_ext": func(r *entity.RetryInCurrentRunRequest) {
			r.Ext = map[string]string{entity.ManualRetryExtKey: "x"}
		},
	} {
		t.Run("validate_"+name, func(t *testing.T) {
			req := &entity.RetryInCurrentRunRequest{SpaceID: 3, ExptID: 1, ExpectedRunID: 2, RequestID: uuid.NewString(), Mode: entity.EvaluationModeFailRetry}
			mutate(req)
			require.Error(t, validateCurrentRunRetry(req))
		})
	}
	t.Run("validate_ok", func(t *testing.T) {
		require.NoError(t, validateCurrentRunRetry(manualRetryUnitRequest(uuid.NewString())))
	})
	t.Run("experiment_validation", func(t *testing.T) {
		req := manualRetryUnitRequest(uuid.NewString())
		require.Error(t, validateCurrentRetryExperiment(nil, req))
		require.Error(t, validateCurrentRetryExperiment(&entity.Experiment{ID: 9, SpaceID: 3, LatestRunID: 2}, req))
		require.Error(t, validateCurrentRetryExperiment(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Success, ExptType: entity.ExptType_Offline}, req))
		require.NoError(t, validateCurrentRetryExperiment(manualRetryUnitExperiment(), req))
	})
	t.Run("wake", func(t *testing.T) {
		m := &ExptMangerImpl{}
		require.Error(t, m.wakeCurrentRetryRun(context.Background(), &entity.Experiment{ExptDispatchMode: entity.ExptDispatchModeEnforce}, nil))
		require.Error(t, m.wakeCurrentRetryRun(context.Background(), &entity.Experiment{}, nil))
	})
}

func TestRetryInCurrentRunManagerUnit(t *testing.T) {
	newDeps := func(t *testing.T) (*termUnitDeps, *repomocks.MockIExptManualRetryRepo, *componentmocks.MockIConfiger, *repomocks.MockIExptRunLogRepo) {
		d := newTermUnitDeps(t)
		return d, repomocks.NewMockIExptManualRetryRepo(d.ctrl), componentmocks.NewMockIConfiger(d.ctrl), repomocks.NewMockIExptRunLogRepo(d.ctrl)
	}
	enabled := func(config *componentmocks.MockIConfiger) {
		config.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).Return(&entity.ExptExecConf{RunningRetryEnabled: gptr.Of(true)}).AnyTimes()
	}

	t.Run("repo_missing", func(t *testing.T) {
		d := newTermUnitDeps(t)
		m := d.manager()
		_, err := m.RetryInCurrentRun(context.Background(), manualRetryUnitRequest(uuid.NewString()))
		require.Error(t, err)
	})
	t.Run("existing_receipt", func(t *testing.T) {
		d, retryRepo, _, _ := newDeps(t)
		receipt := &entity.ManualRetryReceipt{RequestID: "r", AcceptedCount: 1}
		retryRepo.EXPECT().FindReceipt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(receipt, nil)
		m := d.manager()
		m.manualRetryRepo = retryRepo
		got, err := m.RetryInCurrentRun(context.Background(), manualRetryUnitRequest(uuid.NewString()))
		require.NoError(t, err)
		require.Same(t, receipt, got)
	})
	t.Run("disabled", func(t *testing.T) {
		d, retryRepo, config, _ := newDeps(t)
		retryRepo.EXPECT().FindReceipt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		config.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).Return(&entity.ExptExecConf{RunningRetryEnabled: gptr.Of(false)})
		m := d.manager()
		m.manualRetryRepo = retryRepo
		m.configer = config
		_, err := m.RetryInCurrentRun(context.Background(), manualRetryUnitRequest(uuid.NewString()))
		require.Error(t, err)
	})
	t.Run("lock_busy", func(t *testing.T) {
		d, retryRepo, config, _ := newDeps(t)
		retryRepo.EXPECT().FindReceipt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		enabled(config)
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(manualRetryUnitExperiment(), nil)
		d.items.EXPECT().ScanItemResults(gomock.Any(), int64(1), int64(0), int64(500), []int32{int32(entity.ItemRunState_Fail)}, int64(3)).Return(nil, int64(0), nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(false, context.Background(), func() {}, nil)
		m := d.manager()
		m.manualRetryRepo = retryRepo
		m.configer = config
		_, err := m.RetryInCurrentRun(context.Background(), manualRetryUnitRequest(uuid.NewString()))
		require.Error(t, err)
	})
	t.Run("run_finished", func(t *testing.T) {
		d, retryRepo, config, runs := newDeps(t)
		retryRepo.EXPECT().FindReceipt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).Times(2)
		enabled(config)
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(manualRetryUnitExperiment(), nil).Times(2)
		d.items.EXPECT().ScanItemResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		runs.EXPECT().Get(gomock.Any(), int64(1), int64(2)).Return(&entity.ExptRunLog{SpaceID: 3, ExptID: 1, ExptRunID: 2, Status: int64(entity.ExptStatus_Success)}, nil)
		m := d.manager()
		m.manualRetryRepo = retryRepo
		m.configer = config
		m.runLogRepo = runs
		_, err := m.RetryInCurrentRun(context.Background(), manualRetryUnitRequest(uuid.NewString()))
		require.Error(t, err)
	})
	t.Run("prepare_error", func(t *testing.T) {
		d, retryRepo, config, _ := newDeps(t)
		retryRepo.EXPECT().FindReceipt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		enabled(config)
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(manualRetryUnitExperiment(), nil)
		d.items.EXPECT().ScanItemResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, int64(0), errors.New("scan"))
		m := d.manager()
		m.manualRetryRepo = retryRepo
		m.configer = config
		_, err := m.RetryInCurrentRun(context.Background(), manualRetryUnitRequest(uuid.NewString()))
		require.Error(t, err)
	})
	t.Run("happy_path_empty_plan", func(t *testing.T) {
		d, retryRepo, config, runs := newDeps(t)
		retryRepo.EXPECT().FindReceipt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).Times(2)
		enabled(config)
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(manualRetryUnitExperiment(), nil).Times(2)
		d.items.EXPECT().ScanItemResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		runs.EXPECT().Get(gomock.Any(), int64(1), int64(2)).Return(&entity.ExptRunLog{SpaceID: 3, ExptID: 1, ExptRunID: 2, Status: int64(entity.ExptStatus_Processing)}, nil)
		retryRepo.EXPECT().PreflightTerminalItems(gomock.Any(), gomock.Any()).Return(&entity.ManualRetryPreparation{}, nil)
		receipt := &entity.ManualRetryReceipt{RequestID: "r", AcceptedCount: 1}
		retryRepo.EXPECT().RequeueTerminalItems(gomock.Any(), gomock.Any()).Return(receipt, nil)
		m := d.manager()
		m.manualRetryRepo = retryRepo
		m.configer = config
		m.runLogRepo = runs
		got, err := m.RetryInCurrentRun(context.Background(), manualRetryUnitRequest(uuid.NewString()))
		require.NoError(t, err)
		require.Same(t, receipt, got)
	})
}

func TestPrepareTerminalCandidatesUnit(t *testing.T) {
	newDateDeps := func(t *testing.T) (*termUnitDeps, *repomocks.MockIExptTurnResultRepo, *repomocks.MockIExptItemRefRepo) {
		d := newTermUnitDeps(t)
		return d, repomocks.NewMockIExptTurnResultRepo(d.ctrl), repomocks.NewMockIExptItemRefRepo(d.ctrl)
	}
	expt := func() *entity.Experiment { return manualRetryUnitExperiment() }

	t.Run("fail_retry_with_items", func(t *testing.T) {
		d, _, _ := newDateDeps(t)
		m := d.manager()
		_, err := m.prepareTerminalCandidatesAndReusableRefs(context.Background(), expt(), 2, entity.EvaluationModeFailRetry, []int64{9}, nil)
		require.Error(t, err)
	})
	t.Run("fail_retry_scan_error", func(t *testing.T) {
		d, _, _ := newDateDeps(t)
		m := d.manager()
		d.items.EXPECT().ScanItemResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, int64(0), errors.New("scan"))
		_, err := m.prepareTerminalCandidatesAndReusableRefs(context.Background(), expt(), 2, entity.EvaluationModeFailRetry, nil, nil)
		require.Error(t, err)
	})
	t.Run("fail_retry_cursor_stalled", func(t *testing.T) {
		d, _, _ := newDateDeps(t)
		m := d.manager()
		d.items.EXPECT().ScanItemResults(gomock.Any(), gomock.Any(), int64(0), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResult{manualRetryUnitItem()}, int64(5), nil)
		d.items.EXPECT().ScanItemResults(gomock.Any(), gomock.Any(), int64(5), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResult{manualRetryUnitItem()}, int64(5), nil)
		_, err := m.prepareTerminalCandidatesAndReusableRefs(context.Background(), expt(), 2, entity.EvaluationModeFailRetry, nil, nil)
		require.Error(t, err)
	})
	t.Run("retry_items_empty", func(t *testing.T) {
		d, _, _ := newDateDeps(t)
		m := d.manager()
		_, err := m.prepareTerminalCandidatesAndReusableRefs(context.Background(), expt(), 2, entity.EvaluationModeRetryItems, nil, nil)
		require.Error(t, err)
	})
	t.Run("retry_items_invalid_id", func(t *testing.T) {
		d, _, _ := newDateDeps(t)
		m := d.manager()
		_, err := m.prepareTerminalCandidatesAndReusableRefs(context.Background(), expt(), 2, entity.EvaluationModeRetryItems, []int64{0}, nil)
		require.Error(t, err)
	})
	t.Run("retry_items_batch_error", func(t *testing.T) {
		d, _, _ := newDateDeps(t)
		m := d.manager()
		d.items.EXPECT().BatchGet(gomock.Any(), int64(3), int64(1), []int64{9}).Return(nil, errors.New("batch"))
		_, err := m.prepareTerminalCandidatesAndReusableRefs(context.Background(), expt(), 2, entity.EvaluationModeRetryItems, []int64{9}, nil)
		require.Error(t, err)
	})
	t.Run("retry_items_not_found", func(t *testing.T) {
		d, _, _ := newDateDeps(t)
		m := d.manager()
		d.items.EXPECT().BatchGet(gomock.Any(), int64(3), int64(1), []int64{9}).Return(nil, nil)
		plan, err := m.prepareTerminalCandidatesAndReusableRefs(context.Background(), expt(), 2, entity.EvaluationModeRetryItems, []int64{9}, nil)
		require.NoError(t, err)
		require.Len(t, plan.RejectedItems, 1)
		require.Equal(t, "not_found", plan.RejectedItems[0].Reason)
	})
	t.Run("unsupported_mode", func(t *testing.T) {
		d, _, _ := newDateDeps(t)
		m := d.manager()
		_, err := m.prepareTerminalCandidatesAndReusableRefs(context.Background(), expt(), 2, entity.EvaluationModeRetryAll, nil, nil)
		require.Error(t, err)
	})
	t.Run("nil_item", func(t *testing.T) {
		d, _, _ := newDateDeps(t)
		m := d.manager()
		d.items.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResult{nil}, nil)
		_, err := m.prepareTerminalCandidatesAndReusableRefs(context.Background(), expt(), 2, entity.EvaluationModeRetryItems, []int64{9}, nil)
		require.Error(t, err)
	})
	t.Run("invalid_item_source", func(t *testing.T) {
		d, _, _ := newDateDeps(t)
		m := d.manager()
		item := manualRetryUnitItem()
		item.SpaceID = 99
		d.items.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResult{item}, nil)
		_, err := m.prepareTerminalCandidatesAndReusableRefs(context.Background(), expt(), 2, entity.EvaluationModeRetryItems, []int64{9}, nil)
		require.Error(t, err)
	})
	t.Run("manual_context_parse_error", func(t *testing.T) {
		d, _, _ := newDateDeps(t)
		m := d.manager()
		item := manualRetryUnitItem()
		item.Ext = map[string]string{entity.ManualRetryExtKey: "{bad"}
		d.items.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResult{item}, nil)
		_, err := m.prepareTerminalCandidatesAndReusableRefs(context.Background(), expt(), 2, entity.EvaluationModeRetryItems, []int64{9}, nil)
		require.Error(t, err)
	})
	t.Run("manual_context_run_mismatch", func(t *testing.T) {
		d, _, _ := newDateDeps(t)
		m := d.manager()
		item := manualRetryUnitItem()
		item.Ext = map[string]string{entity.ManualRetryExtKey: `{"run_id":99,"request_id":"req","mode":2}`}
		d.items.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResult{item}, nil)
		_, err := m.prepareTerminalCandidatesAndReusableRefs(context.Background(), expt(), 2, entity.EvaluationModeRetryItems, []int64{9}, nil)
		require.Error(t, err)
	})
	t.Run("item_log_read_error", func(t *testing.T) {
		d, _, _ := newDateDeps(t)
		m := d.manager()
		d.items.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResult{manualRetryUnitItem()}, nil)
		d.items.EXPECT().MGetItemRunLog(gomock.Any(), int64(1), int64(2), []int64{9}, int64(3)).Return(nil, errors.New("logs"))
		_, err := m.prepareTerminalCandidatesAndReusableRefs(context.Background(), expt(), 2, entity.EvaluationModeRetryItems, []int64{9}, nil)
		require.Error(t, err)
	})
	t.Run("duplicate_item_log", func(t *testing.T) {
		d, _, _ := newDateDeps(t)
		m := d.manager()
		log := &entity.ExptItemResultRunLog{SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, Status: int32(entity.ItemRunState_Success), ResultState: int32(entity.ExptItemResultStateResulted)}
		d.items.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResult{manualRetryUnitItem()}, nil)
		d.items.EXPECT().MGetItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResultRunLog{log, log}, nil)
		_, err := m.prepareTerminalCandidatesAndReusableRefs(context.Background(), expt(), 2, entity.EvaluationModeRetryItems, []int64{9}, nil)
		require.Error(t, err)
	})
	t.Run("current_log_not_ready", func(t *testing.T) {
		d, _, _ := newDateDeps(t)
		m := d.manager()
		log := &entity.ExptItemResultRunLog{SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, Status: int32(entity.ItemRunState_Success), ResultState: int32(entity.ExptItemResultStateDefault)}
		d.items.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResult{manualRetryUnitItem()}, nil)
		d.items.EXPECT().MGetItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResultRunLog{log}, nil)
		plan, err := m.prepareTerminalCandidatesAndReusableRefs(context.Background(), expt(), 2, entity.EvaluationModeRetryItems, []int64{9}, nil)
		require.NoError(t, err)
		require.Len(t, plan.RejectedItems, 1)
		require.Equal(t, "result_not_ready", plan.RejectedItems[0].Reason)
	})
	t.Run("current_log_missing", func(t *testing.T) {
		d, _, _ := newDateDeps(t)
		m := d.manager()
		item := manualRetryUnitItem()
		item.ExptRunID = 2
		d.items.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResult{item}, nil)
		d.items.EXPECT().MGetItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		_, err := m.prepareTerminalCandidatesAndReusableRefs(context.Background(), expt(), 2, entity.EvaluationModeRetryItems, []int64{9}, nil)
		require.Error(t, err)
	})
	t.Run("single_set_source_missing", func(t *testing.T) {
		d, _, _ := newDateDeps(t)
		m := d.manager()
		badExpt := expt()
		badExpt.EvalSetID = 0
		d.items.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResult{manualRetryUnitItem()}, nil)
		d.items.EXPECT().MGetItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		_, err := m.prepareTerminalCandidatesAndReusableRefs(context.Background(), badExpt, 2, entity.EvaluationModeRetryItems, []int64{9}, nil)
		require.Error(t, err)
	})
	t.Run("invalid_source_type", func(t *testing.T) {
		d, _, _ := newDateDeps(t)
		m := d.manager()
		badExpt := expt()
		badExpt.EvalSetSourceType = entity.ExptEvalSetSourceType(99)
		d.items.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResult{manualRetryUnitItem()}, nil)
		d.items.EXPECT().MGetItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		_, err := m.prepareTerminalCandidatesAndReusableRefs(context.Background(), badExpt, 2, entity.EvaluationModeRetryItems, []int64{9}, nil)
		require.Error(t, err)
	})
	t.Run("multiset_ref_error", func(t *testing.T) {
		d, _, refs := newDateDeps(t)
		m := d.manager()
		m.itemRefRepo = refs
		badExpt := expt()
		badExpt.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
		d.items.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResult{manualRetryUnitItem()}, nil)
		d.items.EXPECT().MGetItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		refs.EXPECT().GetByExptIDAndItemID(gomock.Any(), int64(3), int64(1), int64(9)).Return(nil, errors.New("ref"))
		_, err := m.prepareTerminalCandidatesAndReusableRefs(context.Background(), badExpt, 2, entity.EvaluationModeRetryItems, []int64{9}, nil)
		require.Error(t, err)
	})
	t.Run("multiset_ref_mismatch", func(t *testing.T) {
		d, _, refs := newDateDeps(t)
		m := d.manager()
		m.itemRefRepo = refs
		badExpt := expt()
		badExpt.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
		d.items.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResult{manualRetryUnitItem()}, nil)
		d.items.EXPECT().MGetItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		refs.EXPECT().GetByExptIDAndItemID(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.ExptItemRef{SpaceID: 3, ExptID: 1, ItemID: 9, ItemVersionID: 99}, nil)
		_, err := m.prepareTerminalCandidatesAndReusableRefs(context.Background(), badExpt, 2, entity.EvaluationModeRetryItems, []int64{9}, nil)
		require.Error(t, err)
	})
	t.Run("success_single_set", func(t *testing.T) {
		d, turns, _ := newDateDeps(t)
		m := d.manager()
		d.items.EXPECT().BatchGet(gomock.Any(), int64(3), int64(1), []int64{9}).Return([]*entity.ExptItemResult{manualRetryUnitItem()}, nil)
		d.items.EXPECT().MGetItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		turn := &entity.ExptTurnResult{ID: 10, SpaceID: 3, ExptID: 1, ItemID: 9, ExptRunID: 1, ItemVersionID: 7, TurnID: 0, Status: int32(entity.TurnRunState_Success)}
		turns.EXPECT().GetItemTurnResults(gomock.Any(), int64(1), int64(9), int64(3)).Return([]*entity.ExptTurnResult{turn}, nil)
		turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), int64(1), int64(2), int64(9), int64(3)).Return(nil, nil)
		turns.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), int64(3), []int64{10}).Return(nil, nil)
		m.turnResultRepo = turns
		plan, err := m.prepareTerminalCandidatesAndReusableRefs(context.Background(), expt(), 2, entity.EvaluationModeRetryItems, []int64{9}, nil)
		require.NoError(t, err)
		require.Len(t, plan.Items, 1)
		require.NotNil(t, plan.Items[0].SingleSetSource)
		require.Len(t, plan.Items[0].Turns, 1)
	})
	t.Run("turns_missing", func(t *testing.T) {
		d, turns, _ := newDateDeps(t)
		m := d.manager()
		d.items.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResult{manualRetryUnitItem()}, nil)
		d.items.EXPECT().MGetItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		turns.EXPECT().GetItemTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		m.turnResultRepo = turns
		_, err := m.prepareTerminalCandidatesAndReusableRefs(context.Background(), expt(), 2, entity.EvaluationModeRetryItems, []int64{9}, nil)
		require.Error(t, err)
	})
	t.Run("turn_log_read_error", func(t *testing.T) {
		d, turns, _ := newDateDeps(t)
		m := d.manager()
		d.items.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResult{manualRetryUnitItem()}, nil)
		d.items.EXPECT().MGetItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		turns.EXPECT().GetItemTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResult{{ID: 10, SpaceID: 3, ExptID: 1, ItemID: 9, ExptRunID: 1, ItemVersionID: 7, TurnID: 0}}, nil)
		turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("logs"))
		m.turnResultRepo = turns
		_, err := m.prepareTerminalCandidatesAndReusableRefs(context.Background(), expt(), 2, entity.EvaluationModeRetryItems, []int64{9}, nil)
		require.Error(t, err)
	})
}

func TestManualRetryTurnPreparationUnit(t *testing.T) {
	newDeps := func(t *testing.T) (*termUnitDeps, *repomocks.MockIExptTurnResultRepo) {
		d := newTermUnitDeps(t)
		return d, repomocks.NewMockIExptTurnResultRepo(d.ctrl)
	}
	expt := func() *entity.Experiment { return manualRetryUnitExperiment() }
	preparedItem := func() *entity.PreparedRetryItem {
		return &entity.PreparedRetryItem{SourceItem: manualRetryUnitItem(), PreviousRunID: 1, ItemVersionID: 7}
	}
	turn := func() *entity.ExptTurnResult {
		return &entity.ExptTurnResult{ID: 10, SpaceID: 3, ExptID: 1, ItemID: 9, ExptRunID: 1, ItemVersionID: 7, TurnID: 0, Status: int32(entity.TurnRunState_Success)}
	}

	t.Run("turn_read_error", func(t *testing.T) {
		d, turns := newDeps(t)
		m := d.manager()
		m.turnResultRepo = turns
		turns.EXPECT().GetItemTurnResults(gomock.Any(), int64(1), int64(9), int64(3)).Return(nil, errors.New("read"))
		require.Error(t, m.prepareManualRetryTurns(context.Background(), expt(), 2, entity.EvaluationModeRetryItems, preparedItem(), nil))
	})
	t.Run("invalid_log", func(t *testing.T) {
		d, turns := newDeps(t)
		m := d.manager()
		m.turnResultRepo = turns
		turns.EXPECT().GetItemTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResult{turn()}, nil)
		turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{{SpaceID: 99, TurnID: 0}}, nil)
		require.Error(t, m.prepareManualRetryTurns(context.Background(), expt(), 2, entity.EvaluationModeRetryItems, preparedItem(), nil))
	})
	t.Run("invalid_turn", func(t *testing.T) {
		d, turns := newDeps(t)
		m := d.manager()
		m.turnResultRepo = turns
		bad := turn()
		bad.ExptRunID = 99
		turns.EXPECT().GetItemTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResult{bad}, nil)
		turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		require.Error(t, m.prepareManualRetryTurns(context.Background(), expt(), 2, entity.EvaluationModeRetryItems, preparedItem(), nil))
	})
	t.Run("refs_error", func(t *testing.T) {
		d, turns := newDeps(t)
		m := d.manager()
		m.turnResultRepo = turns
		turns.EXPECT().GetItemTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResult{turn()}, nil)
		turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		turns.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), int64(3), []int64{10}).Return(nil, errors.New("refs"))
		require.Error(t, m.prepareManualRetryTurns(context.Background(), expt(), 2, entity.EvaluationModeRetryItems, preparedItem(), nil))
	})
	t.Run("invalid_ref", func(t *testing.T) {
		d, turns := newDeps(t)
		m := d.manager()
		m.turnResultRepo = turns
		turns.EXPECT().GetItemTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResult{turn()}, nil)
		turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		turns.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnEvaluatorResultRef{{SpaceID: 99}}, nil)
		require.Error(t, m.prepareManualRetryTurns(context.Background(), expt(), 2, entity.EvaluationModeRetryItems, preparedItem(), nil))
	})
	t.Run("missing_current_log", func(t *testing.T) {
		d, turns := newDeps(t)
		m := d.manager()
		m.turnResultRepo = turns
		item := preparedItem()
		item.CurrentItemLog = &entity.ExptItemResultRunLog{SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, ItemVersionID: 7, Status: int32(entity.ItemRunState_Success)}
		turns.EXPECT().GetItemTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResult{turn()}, nil)
		turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		turns.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		require.Error(t, m.prepareManualRetryTurns(context.Background(), expt(), 2, entity.EvaluationModeRetryItems, item, nil))
	})
	t.Run("turn_log_version_mismatch", func(t *testing.T) {
		d, turns := newDeps(t)
		m := d.manager()
		m.turnResultRepo = turns
		turns.EXPECT().GetItemTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResult{turn()}, nil)
		turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{{SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, ItemVersionID: 99, TurnID: 0}}, nil)
		turns.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		require.Error(t, m.prepareManualRetryTurns(context.Background(), expt(), 2, entity.EvaluationModeRetryItems, preparedItem(), nil))
	})
	t.Run("fail_retry_preserves_success", func(t *testing.T) {
		d, turns := newDeps(t)
		m := d.manager()
		m.turnResultRepo = turns
		item := preparedItem()
		turns.EXPECT().GetItemTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResult{turn()}, nil)
		turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		turns.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), gomock.Any(), []int64{10}).Return([]*entity.ExptTurnEvaluatorResultRef{{ID: 700, SpaceID: 3, ExptID: 1, ExptTurnResultID: 10, EvaluatorVersionID: 51, EvaluatorResultID: 61, Alias: "a", SourceType: int32(entity.EvaluatorRecordSourceTypeBuiltin)}}, nil)
		require.NoError(t, m.prepareManualRetryTurns(context.Background(), expt(), 2, entity.EvaluationModeFailRetry, item, nil))
		require.Len(t, item.Turns, 1)
		require.True(t, item.Turns[0].PreserveSuccess)
		require.Equal(t, []int64{700}, item.Turns[0].KeepRefIDs)
	})
	t.Run("sort_source_logs", func(t *testing.T) {
		d, turns := newDeps(t)
		m := d.manager()
		m.turnResultRepo = turns
		item := preparedItem()
		turns.EXPECT().GetItemTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResult{turn()}, nil)
		turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{{SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, ItemVersionID: 7, TurnID: 0}}, nil)
		turns.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		require.NoError(t, m.prepareManualRetryTurns(context.Background(), expt(), 2, entity.EvaluationModeRetryItems, item, nil))
		require.Len(t, item.SourceTurnLogs, 1)
	})
}

func TestManualRetryResultContainsUnit(t *testing.T) {
	inlineRef := &entity.ExptTurnEvaluatorResultRef{SourceType: int32(entity.EvaluatorRecordSourceTypeInline), EvaluatorResultID: 61, InlineKey: "k"}
	registeredRef := &entity.ExptTurnEvaluatorResultRef{SourceType: int32(entity.EvaluatorRecordSourceTypeBuiltin), EvaluatorResultID: 61, EvaluatorVersionID: 51, Alias: "a"}
	require.False(t, manualRetryResultContains(nil, inlineRef))
	require.True(t, manualRetryResultContains(&entity.EvaluatorResults{Inline: []*entity.InlineEvalResult{{InlineKey: "k", RecordID: 61}}}, inlineRef))
	require.False(t, manualRetryResultContains(&entity.EvaluatorResults{Inline: []*entity.InlineEvalResult{{InlineKey: "x", RecordID: 61}}}, inlineRef))
	require.True(t, manualRetryResultContains(&entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 51, Alias: "a", RecordID: 61}}}, registeredRef))
	require.False(t, manualRetryResultContains(&entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 52, Alias: "a", RecordID: 61}}}, registeredRef))
	legacy := &entity.ExptTurnEvaluatorResultRef{EvaluatorVersionID: 51, EvaluatorResultID: 61}
	require.True(t, manualRetryResultContains(&entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{51: 61}}, legacy))
	require.False(t, manualRetryResultContains(&entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{51: 62}}, legacy))
}

func TestManualRetryTargetReaderUnit(t *testing.T) {
	t.Run("service_missing", func(t *testing.T) {
		r := &manualRetryTargetReader{}
		_, err := r.GetRecordByID(context.Background(), 3, 61)
		require.Error(t, err)
	})
	t.Run("record_error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		targets := svcmocks.NewMockIEvalTargetService(ctrl)
		targets.EXPECT().GetRecordByID(gomock.Any(), int64(3), int64(61)).Return(nil, errors.New("rpc"))
		r := &manualRetryTargetReader{IEvalTargetService: targets}
		_, err := r.GetRecordByID(context.Background(), 3, 61)
		require.Error(t, err)
	})
	t.Run("record_mismatch", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		targets := svcmocks.NewMockIEvalTargetService(ctrl)
		targets.EXPECT().GetRecordByID(gomock.Any(), int64(3), int64(61)).Return(&entity.EvalTargetRecord{ID: 61, SpaceID: 3, TargetVersionID: 99, ExperimentRunID: 2, ItemID: 9, TurnID: 0}, nil)
		r := &manualRetryTargetReader{IEvalTargetService: targets, versionID: 8, turn: &entity.ExptTurnResult{ItemID: 9, TurnID: 0}}
		_, err := r.GetRecordByID(context.Background(), 3, 61)
		require.Error(t, err)
	})
	t.Run("success_status_without_source_log", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		targets := svcmocks.NewMockIEvalTargetService(ctrl)
		targets.EXPECT().GetRecordByID(gomock.Any(), int64(3), int64(61)).Return(&entity.EvalTargetRecord{ID: 61, SpaceID: 3, TargetVersionID: 8, ExperimentRunID: 2, ItemID: 9, TurnID: 0, Status: gptr.Of(entity.EvalTargetRunStatusSuccess)}, nil)
		r := &manualRetryTargetReader{IEvalTargetService: targets, versionID: 8, turn: &entity.ExptTurnResult{ItemID: 9, TurnID: 0}, source: &retryEvaluatorSourceLogs{repo: repomocks.NewMockIExptTurnResultRepo(ctrl), logsByRun: map[int64]map[int64]*entity.ExptTurnResultRunLog{2: {}}}}
		_, err := r.GetRecordByID(context.Background(), 3, 61)
		require.Error(t, err)
	})
}

func TestRetryInCurrentRunRemainingBranches(t *testing.T) {
	newDeps := func(t *testing.T) (*termUnitDeps, *repomocks.MockIExptManualRetryRepo, *componentmocks.MockIConfiger, *repomocks.MockIExptRunLogRepo) {
		d := newTermUnitDeps(t)
		return d, repomocks.NewMockIExptManualRetryRepo(d.ctrl), componentmocks.NewMockIConfiger(d.ctrl), repomocks.NewMockIExptRunLogRepo(d.ctrl)
	}
	enabled := func(config *componentmocks.MockIConfiger) {
		config.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).Return(&entity.ExptExecConf{RunningRetryEnabled: gptr.Of(true)}).AnyTimes()
	}
	req := func() *entity.RetryInCurrentRunRequest { return manualRetryUnitRequest(uuid.NewString()) }

	t.Run("find_receipt_error", func(t *testing.T) {
		d, retryRepo, _, _ := newDeps(t)
		retryRepo.EXPECT().FindReceipt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("find"))
		m := d.manager()
		m.manualRetryRepo = retryRepo
		_, err := m.RetryInCurrentRun(context.Background(), req())
		require.Error(t, err)
	})
	t.Run("locker_nil", func(t *testing.T) {
		d, retryRepo, config, _ := newDeps(t)
		retryRepo.EXPECT().FindReceipt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		enabled(config)
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(manualRetryUnitExperiment(), nil)
		d.items.EXPECT().ScanItemResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
		m := d.manager()
		m.manualRetryRepo = retryRepo
		m.configer = config
		m.manualRetryLocker = nil
		_, err := m.RetryInCurrentRun(context.Background(), req())
		require.Error(t, err)
	})
	t.Run("run_missing", func(t *testing.T) {
		d, retryRepo, config, runs := newDeps(t)
		retryRepo.EXPECT().FindReceipt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).Times(2)
		enabled(config)
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(manualRetryUnitExperiment(), nil).Times(2)
		d.items.EXPECT().ScanItemResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		runs.EXPECT().Get(gomock.Any(), int64(1), int64(2)).Return(nil, nil)
		m := d.manager()
		m.manualRetryRepo = retryRepo
		m.configer = config
		m.runLogRepo = runs
		_, err := m.RetryInCurrentRun(context.Background(), req())
		require.Error(t, err)
	})
	t.Run("run_mismatch", func(t *testing.T) {
		d, retryRepo, config, runs := newDeps(t)
		retryRepo.EXPECT().FindReceipt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).Times(2)
		enabled(config)
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(manualRetryUnitExperiment(), nil).Times(2)
		d.items.EXPECT().ScanItemResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		runs.EXPECT().Get(gomock.Any(), int64(1), int64(2)).Return(&entity.ExptRunLog{SpaceID: 99, ExptID: 1, ExptRunID: 2}, nil)
		m := d.manager()
		m.manualRetryRepo = retryRepo
		m.configer = config
		m.runLogRepo = runs
		_, err := m.RetryInCurrentRun(context.Background(), req())
		require.Error(t, err)
	})
	t.Run("preflight_error", func(t *testing.T) {
		d, retryRepo, config, runs := newDeps(t)
		retryRepo.EXPECT().FindReceipt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).Times(2)
		enabled(config)
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(manualRetryUnitExperiment(), nil).Times(2)
		d.items.EXPECT().ScanItemResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		runs.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.ExptRunLog{SpaceID: 3, ExptID: 1, ExptRunID: 2, Status: int64(entity.ExptStatus_Processing)}, nil)
		retryRepo.EXPECT().PreflightTerminalItems(gomock.Any(), gomock.Any()).Return(nil, errors.New("preflight"))
		m := d.manager()
		m.manualRetryRepo = retryRepo
		m.configer = config
		m.runLogRepo = runs
		_, err := m.RetryInCurrentRun(context.Background(), req())
		require.Error(t, err)
	})
	t.Run("requeue_error", func(t *testing.T) {
		d, retryRepo, config, runs := newDeps(t)
		retryRepo.EXPECT().FindReceipt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).Times(2)
		enabled(config)
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(manualRetryUnitExperiment(), nil).Times(2)
		d.items.EXPECT().ScanItemResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		runs.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.ExptRunLog{SpaceID: 3, ExptID: 1, ExptRunID: 2, Status: int64(entity.ExptStatus_Processing)}, nil)
		retryRepo.EXPECT().PreflightTerminalItems(gomock.Any(), gomock.Any()).Return(&entity.ManualRetryPreparation{}, nil)
		retryRepo.EXPECT().RequeueTerminalItems(gomock.Any(), gomock.Any()).Return(nil, errors.New("requeue"))
		m := d.manager()
		m.manualRetryRepo = retryRepo
		m.configer = config
		m.runLogRepo = runs
		_, err := m.RetryInCurrentRun(context.Background(), req())
		require.Error(t, err)
	})
}
