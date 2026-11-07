// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	idgenMocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// retryJoinRaceLocker is an in-memory implementation of the two lock calls used
// by LogRetryItemsRun and ExptEnd. Lock only succeeds when the key is free, so
// the test observes exactly the mutual exclusion Redis provides.
type retryJoinRaceLocker struct {
	lock.ILocker
	mu      sync.Mutex
	held    map[string]string
	lockErr error
}

func newRetryJoinRaceLocker() *retryJoinRaceLocker {
	return &retryJoinRaceLocker{held: make(map[string]string)}
}

func (l *retryJoinRaceLocker) Lock(_ context.Context, key string, expiresIn time.Duration) (bool, error) {
	if l.lockErr != nil {
		return false, l.lockErr
	}
	if expiresIn < time.Second {
		return false, errorx.New("lock ttl too short")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.held[key]; ok {
		return false, nil
	}
	l.held[key] = "join"
	return true, nil
}

func (l *retryJoinRaceLocker) Unlock(key string) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.held[key]; !ok {
		return false, nil
	}
	delete(l.held, key)
	return true, nil
}

func (l *retryJoinRaceLocker) Exists(_ context.Context, key string) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, ok := l.held[key]
	return ok, nil
}

func (l *retryJoinRaceLocker) BackoffLockWithValue(_ context.Context, key, val string, expiresIn, _ time.Duration) (bool, string, error) {
	if expiresIn < time.Second {
		return false, "", errorx.New("lock ttl too short")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if holder, ok := l.held[key]; ok {
		return false, holder, nil
	}
	l.held[key] = val
	return true, val, nil
}

func (l *retryJoinRaceLocker) holder(key string) (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	holder, ok := l.held[key]
	return holder, ok
}

// retryJoinRaceRunLogRepo models the two run-log rows the race touches. Get
// serves the master snapshot only when WithCtxWriteDB is set, mirroring the
// production repo, so a stale replica read is distinguishable.
type retryJoinRaceRunLogRepo struct {
	repo.IExptRunLogRepo
	mu           sync.Mutex
	spaceID      int64
	status       int64
	itemIDs      []int64
	masterStatus *int64
	masterIDs    []int64
	replicaIDs   []int64
	masterReads  int
	replicaReads int
	appendCalls  int
	onAppend     func()
}

func (r *retryJoinRaceRunLogRepo) Get(ctx context.Context, exptID, runID int64) (*entity.ExptRunLog, error) {
	r.mu.Lock()
	status := r.status
	ids := r.itemIDs
	if contexts.CtxWriteDB(ctx) {
		r.masterReads++
		if r.masterStatus != nil {
			status = *r.masterStatus
		}
		if r.masterIDs != nil {
			ids = r.masterIDs
		}
	} else {
		r.replicaReads++
		if r.replicaIDs != nil {
			ids = r.replicaIDs
		}
	}
	ids = append([]int64(nil), ids...)
	spaceID := r.spaceID
	r.mu.Unlock()
	return &entity.ExptRunLog{
		ID: runID, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID,
		Mode: int32(entity.EvaluationModeRetryItems), Status: status,
		ItemIds: []entity.ExptRunLogItems{{ItemIDs: ids}},
	}, nil
}

func (r *retryJoinRaceRunLogRepo) AppendRunItemIDs(_ context.Context, _, _, _ int64, itemIDs []int64) error {
	// The hook runs before the append commits: it is the exact window in which
	// the finalizing tick may snapshot item_ids.
	if r.onAppend != nil {
		r.onAppend()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.appendCalls++
	for _, itemID := range itemIDs {
		for _, existing := range r.itemIDs {
			if existing == itemID {
				return errorx.NewByCode(errno.EvalItemAlreadyRetryingCode)
			}
		}
		r.itemIDs = append(r.itemIDs, itemID)
	}
	return nil
}

func (r *retryJoinRaceRunLogRepo) finalize() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status = int64(entity.ExptStatus_Success)
}

func (r *retryJoinRaceRunLogRepo) state() (int64, []int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status, append([]int64(nil), r.itemIDs...)
}

// retryJoinTickManager keeps the real completion-lock acquisition and only
// short-circuits the heavy completion writes, recording the terminal state the
// tick would have produced.
type retryJoinTickManager struct {
	*ExptMangerImpl
	repo      *retryJoinRaceRunLogRepo
	finalized bool
}

func (m *retryJoinTickManager) CompleteRun(context.Context, int64, int64, int64, *entity.Session, ...entity.CompleteExptOptionFn) error {
	m.finalized = true
	m.repo.finalize()
	return nil
}

func (m *retryJoinTickManager) CompleteExpt(context.Context, int64, *int64, int64, *entity.Session, ...entity.CompleteExptOptionFn) error {
	return nil
}

type retryJoinRaceHarness struct {
	ctrl     *gomock.Controller
	manager  *ExptMangerImpl
	locker   *retryJoinRaceLocker
	repo     *retryJoinRaceRunLogRepo
	tick     *retryJoinTickManager
	exec     *ExptRetryItemsExec
	event    *entity.ExptScheduleEvent
	expt     *entity.Experiment
	exptID   int64
	runID    int64
	spaceID  int64
	itemA    int64
	itemB    int64
	complete string
}

func newRetryJoinRaceHarness(t *testing.T) *retryJoinRaceHarness {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	h := &retryJoinRaceHarness{
		ctrl: ctrl, exptID: 1, runID: 2, spaceID: 3, itemA: 10, itemB: 11,
	}
	h.complete = makeCompletingLockKeyForTest(h.exptID, h.runID)

	h.manager = newTestExptManager(ctrl)
	h.locker = newRetryJoinRaceLocker()
	h.manager.mutex = h.locker
	h.repo = &retryJoinRaceRunLogRepo{
		spaceID: h.spaceID, status: int64(entity.ExptStatus_Processing), itemIDs: []int64{h.itemA},
	}
	h.manager.runLogRepo = h.repo
	h.manager.runFinishRepo = &originalCompletionRunFinishRepo{}
	h.manager.idgenerator.(*idgenMocks.MockIIDGenerator).EXPECT().GenID(gomock.Any()).Return(int64(9), nil).AnyTimes()

	// The experiment-level lock is held by the live run, so the retry joins it.
	h.locker.held[h.manager.makeExptMutexLockKey(h.exptID)] = "2"

	idem := idemmocks.NewMockIdempotentService(ctrl)
	idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil).AnyTimes()
	idem.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	items := repomocks.NewMockIExptItemResultRepo(ctrl)
	items.EXPECT().ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, int64(0), nil).AnyTimes()

	h.tick = &retryJoinTickManager{ExptMangerImpl: h.manager, repo: h.repo}
	h.exec = &ExptRetryItemsExec{
		manager: h.tick, idem: idem, configer: h.manager.configer,
		exptItemResultRepo: items, exptRunLogRepo: h.repo,
		publisher: eventmocks.NewMockExptEventPublisher(ctrl),
	}
	h.event = &entity.ExptScheduleEvent{
		SpaceID: h.spaceID, ExptID: h.exptID, ExptRunID: h.runID,
		ExptType: entity.ExptType_Offline, ExptRunMode: entity.EvaluationModeRetryItems,
		Session: &entity.Session{UserID: "u"}, ExecEvalSetItemIDs: []int64{h.itemA},
	}
	h.expt = &entity.Experiment{
		ID: h.exptID, SpaceID: h.spaceID, LatestRunID: h.runID,
		Status: entity.ExptStatus_Processing, ExptType: entity.ExptType_Offline,
	}
	return h
}

// The regression: the legacy join checked the completing lock, then appended
// without holding it. A completion tick that ran in that gap snapshotted the
// pre-append item_ids, finished the run, and the API still reported success for
// an item nobody would execute.
func TestLegacyRetryJoinCannotAppendAfterCompletionFinalizes(t *testing.T) {
	h := newRetryJoinRaceHarness(t)

	var (
		tickNext  bool
		tickErr   error
		tickCalls int
	)
	h.repo.onAppend = func() {
		tickCalls++
		tickNext, tickErr = h.exec.ExptEnd(context.Background(), h.event, h.expt, 0, 0)
	}

	runID, retried, err := h.manager.LogRetryItemsRun(
		context.Background(), h.exptID, entity.EvaluationModeRetryItems, h.spaceID,
		[]int64{h.itemB}, &entity.Session{UserID: "u"})
	require.NoError(t, err)
	require.True(t, retried)
	require.Equal(t, h.runID, runID)
	require.Equal(t, 1, tickCalls, "the finalizing tick must be attempted exactly once")
	require.False(t, tickNext)

	status, ids := h.repo.state()
	if h.tick.finalized {
		t.Fatalf("legacy join returned success after the finalizing tick finished the run: status=%d item_ids=%v", status, ids)
	}
	require.Equal(t, int64(entity.ExptStatus_Processing), status, "join success must leave the run open for the appended item")
	require.Equal(t, []int64{h.itemA, h.itemB}, ids)
	require.Equal(t, 1, h.repo.appendCalls)

	// A completion that reached the lock while the join held it must be rejected
	// as busy instead of finalizing on an item_ids snapshot that excludes the join.
	require.Error(t, tickErr, "the finalizing tick must not run while the legacy join holds the completing mutex")

	require.GreaterOrEqual(t, h.repo.masterReads, 1, "join must re-read the run from master after acquiring the mutex")
	require.Zero(t, h.repo.replicaReads, "join must not validate against a replica snapshot")
	if _, held := h.locker.holder(h.complete); held {
		t.Fatalf("join leaked the completing mutex it acquired")
	}
	if holder, held := h.locker.holder(h.manager.makeExptMutexLockKey(h.exptID)); !held || holder != "2" {
		t.Fatalf("join must not touch the live run's experiment lock, got holder=%q held=%v", holder, held)
	}
}

func TestRetryItemsCompletionSnapshotUsesMaster(t *testing.T) {
	h := newRetryJoinRaceHarness(t)
	h.repo.masterIDs = []int64{h.itemA, h.itemB}
	h.repo.replicaIDs = []int64{h.itemA}

	next, err := h.exec.ExptEnd(context.Background(), h.event, h.expt, 0, 0)
	require.NoError(t, err)
	require.True(t, next, "a committed append visible on master must keep the run ticking")
	require.False(t, h.tick.finalized, "a stale replica snapshot must not finalize the run")
	require.Equal(t, 1, h.repo.masterReads)
	require.Zero(t, h.repo.replicaReads)
	if _, held := h.locker.holder(h.complete); held {
		t.Fatalf("completion leaked the completing mutex")
	}
}

func TestLegacyRetryJoinRejectsBusyCompletingLock(t *testing.T) {
	h := newRetryJoinRaceHarness(t)
	h.locker.held[h.complete] = "completer"

	_, retried, err := h.manager.LogRetryItemsRun(
		context.Background(), h.exptID, entity.EvaluationModeRetryItems, h.spaceID,
		[]int64{h.itemB}, &entity.Session{UserID: "u"})
	require.Error(t, err)
	require.False(t, retried)
	statusErr, ok := errorx.FromStatusError(err)
	require.True(t, ok)
	require.Equal(t, int32(errno.ExperimentIsCompletingCode), statusErr.Code())

	status, ids := h.repo.state()
	require.Equal(t, int64(entity.ExptStatus_Processing), status)
	require.Equal(t, []int64{h.itemA}, ids)
	require.Zero(t, h.repo.appendCalls)
	if holder, held := h.locker.holder(h.complete); !held || holder != "completer" {
		t.Fatalf("a busy join must not release the completing lock it did not acquire, got holder=%q held=%v", holder, held)
	}
}

func TestLegacyRetryJoinLockerErrorIsNotReportedAsBusy(t *testing.T) {
	h := newRetryJoinRaceHarness(t)
	sentinel := errorx.New("redis unavailable")
	h.locker.lockErr = sentinel

	_, _, err := h.manager.LogRetryItemsRun(
		context.Background(), h.exptID, entity.EvaluationModeRetryItems, h.spaceID,
		[]int64{h.itemB}, &entity.Session{UserID: "u"})
	require.ErrorIs(t, err, sentinel)
	_, ok := errorx.FromStatusError(err)
	require.False(t, ok, "a locker error must stay an error, not become ExperimentIsCompletingCode")
	require.Zero(t, h.repo.appendCalls)
}

func TestLegacyRetryJoinRejectsTerminalRunAfterLockAcquisition(t *testing.T) {
	h := newRetryJoinRaceHarness(t)
	terminal := int64(entity.ExptStatus_Success)
	h.repo.masterStatus = &terminal

	_, retried, err := h.manager.LogRetryItemsRun(
		context.Background(), h.exptID, entity.EvaluationModeRetryItems, h.spaceID,
		[]int64{h.itemB}, &entity.Session{UserID: "u"})
	require.Error(t, err)
	require.False(t, retried)
	statusErr, ok := errorx.FromStatusError(err)
	require.True(t, ok)
	require.Equal(t, int32(errno.ExperimentRunningExistedCode), statusErr.Code())

	status, ids := h.repo.state()
	require.Equal(t, int64(entity.ExptStatus_Processing), status, "fixture's canonical status stays as configured")
	require.Equal(t, []int64{h.itemA}, ids)
	require.Zero(t, h.repo.appendCalls)
	require.GreaterOrEqual(t, h.repo.masterReads, 1)
	require.Zero(t, h.repo.replicaReads, "a stale replica read must not authorize the append")
	if _, held := h.locker.holder(h.complete); held {
		t.Fatalf("join leaked the completing mutex it acquired")
	}
}

func makeCompletingLockKeyForTest(exptID, runID int64) string {
	return (&ExptMangerImpl{}).makeExptCompletingLockKey(exptID, runID)
}
