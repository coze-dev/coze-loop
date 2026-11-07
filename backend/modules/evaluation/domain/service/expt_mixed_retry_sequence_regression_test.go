// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

// The existing combination fixture only requeues materialized rows and its
// GenID returns a constant. Missing historical logs require unique IDs. Keep
// its other generator behavior while assigning distinct new SQL row IDs.
type mixedRetrySequenceLogIDs struct {
	idgen.IIDGenerator
	next atomic.Int64
}

func (g *mixedRetrySequenceLogIDs) GenID(context.Context) (int64, error) {
	return g.next.Add(1), nil
}

// Keep B running while FailRetry discovers historical C outside R, then
// switch A/C to RetryItems and submit an overlapping selection. All states
// after fixture initialization use the production manager, dispatch CAS,
// current-source completion and Resulted publication paths. Actors and the
// central quota ledger are not exercised by this SQL sequence.
func TestMixedRetrySequenceHistoricalFailAppendAndOverlappingAdmission(t *testing.T) {
	p := newCurrentRetryCombinationFixture(t, entity.EvaluationModeRetryItems)
	f := p.f
	ctx := context.Background()
	ids := &mixedRetrySequenceLogIDs{IIDGenerator: p.manager.idgenerator}
	ids.next.Store(f.base + 20000)
	p.manager.manualRetryRepo = exptrepo.NewExptManualRetryRepo(f.provider, ids)
	// The helper soft-deletes its old ref. Remove only that already-deleted
	// fixture row before taking raw-table snapshots: retry correctly purges
	// tombstones, while this sequence asserts a physically empty ref table.
	require.NoError(t, f.conn.Unscoped().Where("space_id=? AND expt_id=? AND deleted_at IS NOT NULL", f.space, f.expt).Delete(&model.ExptTurnEvaluatorResultRef{}).Error)
	var activeRefs int64
	require.NoError(t, f.conn.Model(&model.ExptTurnEvaluatorResultRef{}).Where("space_id=? AND expt_id=?", f.space, f.expt).Count(&activeRefs).Error)
	require.Zero(t, activeRefs, "the reused record-free fixture must have no active refs")
	a, b, c, d := f.item, f.item+1, f.item+2, f.item+3
	x := f.item + 999

	// The reused helper initially appends C/D. Seed the earlier legal
	// A/B-only run before the first request; never assign Resulted to a live
	// attempt to advance the test. C has one historical successful turn and
	// one failed turn, while D is an untouched historical successful item.
	originalChunk := p.identity(t).ItemIds[0]
	encoded, err := entity.EncodeExptRunLogItems([]entity.ExptRunLogItems{originalChunk})
	require.NoError(t, err)
	for _, table := range []string{"expt_item_result_run_log", "expt_turn_result_run_log"} {
		require.NoError(t, f.conn.Exec("DELETE FROM "+table+" WHERE space_id=? AND expt_id=? AND expt_run_id=? AND item_id IN ?", f.space, f.expt, f.run, []int64{c, d}).Error)
	}
	for _, historical := range []struct {
		item   int64
		status entity.ItemRunState
	}{{c, entity.ItemRunState_Fail}, {d, entity.ItemRunState_Success}} {
		require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, historical.item).Updates(map[string]any{"expt_run_id": p.oldRun, "status": int32(historical.status)}).Error)
		require.NoError(t, f.conn.Model(&model.ExptTurnResult{}).Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, historical.item).Updates(map[string]any{"expt_run_id": p.oldRun, "status": int32(entity.TurnRunState_Success), "target_result_id": 0}).Error)
		require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", f.space, f.expt, p.oldRun, historical.item).Update("status", int32(historical.status)).Error)
	}
	failedTurn := f.base + 23
	require.NoError(t, f.conn.Create(&model.ExptTurnResult{ID: f.base + 480, SpaceID: f.space, ExptID: f.expt, ExptRunID: p.oldRun, ItemID: c, ItemVersionID: f.version, TurnID: failedTurn, Status: int32(entity.TurnRunState_Fail)}).Error)
	require.NoError(t, f.conn.Create(&model.ExptTurnResultRunLog{ID: f.base + 780, SpaceID: f.space, ExptID: f.expt, ExptRunID: p.oldRun, ItemID: c, ItemVersionID: f.version, TurnID: failedTurn, Status: int32(entity.TurnRunState_Fail), EvaluatorResultIds: gptr.Of([]byte(`{}`))}).Error)
	require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, f.run).Updates(map[string]any{"item_ids": encoded, "pending_cnt": 0, "processing_cnt": 2, "success_cnt": 0, "fail_cnt": 0, "terminated_cnt": 0}).Error)
	require.NoError(t, f.conn.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", f.space, f.expt).Updates(map[string]any{"pending_cnt": 0, "processing_cnt": 2, "success_cnt": 1, "fail_cnt": 1, "terminated_cnt": 0}).Error)
	p.event.ExecEvalSetItemIDs = []int64{a, b}

	waitReleased := func() {
		t.Helper()
		require.Eventually(t, func() bool {
			if p.server.Exists(fmt.Sprintf("expt_run_exec_lock:%d:%d", f.expt, f.run)) {
				return false
			}
			for _, item := range []int64{a, b, c, d} {
				if p.server.Exists(fmt.Sprintf("expt_item_eval_run_lock:%d:%d", f.expt, item)) {
					return false
				}
			}
			return true
		}, 2*time.Second, time.Millisecond, "wait for real asynchronous lock cancellation on every selected item")
	}
	assertCounts := func(pending, processing, success, fail int32) {
		t.Helper()
		var stats model.ExptStats
		require.NoError(t, f.conn.Where("space_id=? AND expt_id=?", f.space, f.expt).Take(&stats).Error)
		require.Equal(t, []int32{pending, processing, success, fail, 0}, []int32{stats.PendingCnt, stats.ProcessingCnt, stats.SuccessCnt, stats.FailCnt, stats.TerminatedCnt})
	}
	assertRunCounts := func(pending, processing, success, fail int32) {
		t.Helper()
		run := p.identity(t)
		require.Equal(t, []int32{pending, processing, success, fail, 0}, []int32{run.PendingCnt, run.ProcessingCnt, run.SuccessCnt, run.FailCnt, run.TerminatedCnt})
	}
	readToken := func(item int64) string {
		t.Helper()
		source, err := f.port.GetCurrentItemRunSource(ctx, f.space, f.expt, f.run, item)
		require.NoError(t, err)
		require.NotNil(t, source)
		require.NotEmpty(t, source.Current.RequestID)
		return source.Current.RequestID
	}
	assertSource := func(item int64, token string, mode entity.ExptRunMode) *entity.ExptCurrentItemRunSource {
		t.Helper()
		source, err := f.port.GetCurrentItemRunSource(ctx, f.space, f.expt, f.run, item)
		require.NoError(t, err)
		require.NotNil(t, source)
		require.Equal(t, token, source.Current.RequestID)
		require.Equal(t, mode, source.Current.Mode)
		require.Equal(t, entity.ItemRunState_Queueing, source.Current.Status)
		require.Equal(t, f.run, source.Current.RunID)
		// This source DTO does not load quota_reservation_state. Read the
		// actual SQL column instead of treating a zero-value field as proof.
		var itemLog model.ExptItemResultRunLog
		require.NoError(t, f.conn.Select("quota_reservation_state").Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", f.space, f.expt, f.run, item).Take(&itemLog).Error)
		require.Equal(t, int32(entity.QuotaReservationStateNone), itemLog.QuotaReservationState)
		require.Empty(t, source.CurrentRefs)
		return source
	}
	request := func(mode entity.ExptRunMode, ids ...int64) *entity.RetryInCurrentRunRequest {
		return &entity.RetryInCurrentRunRequest{SpaceID: f.space, ExptID: f.expt, ExpectedRunID: f.run, Mode: mode, ItemIDs: ids, Session: p.event.Session}
	}

	require.Equal(t, []int64{a, b}, p.identity(t).GetItemIDs())
	var cLogs int64
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", f.space, f.expt, f.run, c).Count(&cLogs).Error)
	require.Zero(t, cLogs)
	untouchedB, err := f.port.GetCurrentItemRunSource(ctx, f.space, f.expt, f.run, b)
	require.NoError(t, err)
	require.Equal(t, entity.ItemRunState_Processing, untouchedB.Current.Status)
	historicalRecords := f.snapshot()
	p.completeAndPublish(t, a, entity.ItemRunState_Fail)
	waitReleased()
	assertCounts(0, 1, 1, 2)
	assertRunCounts(0, 2, 0, 0) // publication does not refresh run-log counters

	// No item IDs are supplied: the actual Fail scan must discover C.
	q1, err := p.manager.RetryInCurrentRun(ctx, request(entity.EvaluationModeFailRetry))
	require.NoError(t, err)
	require.Equal(t, int32(2), q1.AcceptedCount)
	require.Empty(t, q1.RejectedItems)
	require.Equal(t, f.run, q1.RunID)
	waitReleased()
	q1Token := readToken(a) // current receipt intentionally omits RequestID
	q1A := assertSource(a, q1Token, entity.EvaluationModeFailRetry)
	q1C := assertSource(c, q1Token, entity.EvaluationModeFailRetry)
	require.Len(t, q1A.Turns, 2)
	require.Len(t, q1C.Turns, 2)
	require.Equal(t, entity.TurnRunState_Success, q1A.Turns[0].Status)
	require.Equal(t, entity.TurnRunState_Success, q1C.Turns[0].Status)
	require.Equal(t, entity.TurnRunState_Queueing, q1C.Turns[failedTurn].Status)
	require.Equal(t, []int64{a, b, c}, p.identity(t).GetItemIDs())
	require.Equal(t, originalChunk, p.identity(t).ItemIds[0])
	assertCounts(2, 1, 1, 0)
	assertRunCounts(2, 1, 0, 0)
	t.Logf("MIXED_SEQUENCE_PHASE Q1_failed accepted=2 items=[%d,%d] appended=%d request=%s stats=2/1/1/0", a, c, c, q1Token)
	for _, item := range []int64{a, c} {
		p.completeAndPublish(t, item, entity.ItemRunState_Success)
		waitReleased()
	}
	assertCounts(0, 1, 3, 0)
	assertRunCounts(2, 1, 0, 0) // the next admission refreshes these counters

	// Once both real publications reached Resulted, RetryItems resets every
	// prepared turn. This is the S02 switch back, and C03's first selection.
	q2, err := p.manager.RetryInCurrentRun(ctx, request(entity.EvaluationModeRetryItems, a, c))
	require.NoError(t, err)
	require.Equal(t, int32(2), q2.AcceptedCount)
	require.Empty(t, q2.RejectedItems)
	waitReleased()
	q2Token := readToken(a)
	require.NotEqual(t, q1Token, q2Token)
	q2A := assertSource(a, q2Token, entity.EvaluationModeRetryItems)
	q2C := assertSource(c, q2Token, entity.EvaluationModeRetryItems)
	for _, source := range []*entity.ExptCurrentItemRunSource{q2A, q2C} {
		for _, turn := range source.Turns {
			require.Equal(t, entity.TurnRunState_Queueing, turn.Status)
			require.Zero(t, turn.TargetResultID)
			require.True(t, entity.SameCurrentEvaluatorRefs(turn.EvaluatorResultIds, nil))
		}
		for _, turn := range source.CanonicalTurns {
			require.Equal(t, int32(entity.TurnRunState_Queueing), turn.Status)
			require.Zero(t, turn.TargetResultID)
			require.Equal(t, f.run, turn.ExptRunID)
		}
		require.Len(t, source.CanonicalTurns, len(source.Turns))
		require.Empty(t, source.CurrentRefs)
	}
	assertCounts(2, 1, 1, 0)
	assertRunCounts(2, 1, 0, 0)
	t.Logf("MIXED_SEQUENCE_PHASE Q2_items accepted=2 items=[%d,%d] request=%s all_turns_queueing=true stats=2/1/1/0", a, c, q2Token)
	beforeOverlap := f.snapshot()

	// A/C remain Queueing. Only historical D is eligible; duplicate D is
	// accepted once. C/B/X receive distinct per-item rejection reasons.
	q3, err := p.manager.RetryInCurrentRun(ctx, request(entity.EvaluationModeRetryItems, c, d, b, x, d, c))
	require.NoError(t, err)
	require.Equal(t, int32(1), q3.AcceptedCount)
	require.Equal(t, f.run, q3.RunID)
	waitReleased()
	q3Token := readToken(d)
	require.NotEqual(t, q1Token, q3Token)
	require.NotEqual(t, q2Token, q3Token)
	reasons := make(map[int64]string)
	for _, rejected := range q3.RejectedItems {
		reasons[rejected.ItemID] = rejected.Reason
	}
	require.Len(t, q3.RejectedItems, 3)
	require.Equal(t, map[int64]string{b: "processing", c: "queueing", x: "not_found"}, reasons)
	assertSource(d, q3Token, entity.EvaluationModeRetryItems)
	require.Equal(t, q2A, assertSource(a, q2Token, entity.EvaluationModeRetryItems))
	require.Equal(t, q2C, assertSource(c, q2Token, entity.EvaluationModeRetryItems))
	afterB, err := f.port.GetCurrentItemRunSource(ctx, f.space, f.expt, f.run, b)
	require.NoError(t, err)
	require.Equal(t, untouchedB, afterB)
	require.Equal(t, []int64{a, b, c, d}, p.identity(t).GetItemIDs())
	require.Equal(t, originalChunk, p.identity(t).ItemIds[0])
	assertCounts(3, 1, 0, 0)
	assertRunCounts(3, 1, 0, 0)
	afterOverlap := f.snapshot()
	for _, table := range []string{"expt_turn_evaluator_result_ref", "eval_target_record", "evaluator_record"} {
		require.Equal(t, historicalRecords[table], afterOverlap[table], "controlled record-free results must not mutate actor records")
		require.Equal(t, beforeOverlap[table], afterOverlap[table])
	}
	t.Logf("MIXED_SEQUENCE_REAL_MYSQL S02_fail_scan_appended_C=true C03_partial_accepted=1 rejected=3 run=%d Q1=%s Q2=%s Q3=%s B_processing_preserved=true record_free=true", f.run, q1Token, q2Token, q3Token)
}

// Only gate one real run-tick call. Every lock result and lease still comes
// from the existing production locker against miniredis.
type mixedRetryWindowGate struct {
	entered chan struct{}
	resume  chan struct{}
	used    atomic.Bool
	opened  atomic.Bool
}

func (g *mixedRetryWindowGate) open() {
	if g.opened.CompareAndSwap(false, true) {
		close(g.resume)
	}
}

func (g *mixedRetryWindowGate) pause(ctx context.Context) error {
	close(g.entered)
	select {
	case <-g.resume:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type mixedRetryWindowLocker struct {
	lock.ILocker
	gate   *mixedRetryWindowGate
	runKey string
	before bool
}

func (l *mixedRetryWindowLocker) LockWithRenew(ctx context.Context, key string, ttl, hold time.Duration) (bool, context.Context, func(), error) {
	if key == l.runKey && l.before && l.gate.used.CompareAndSwap(false, true) {
		if err := l.gate.pause(ctx); err != nil {
			return false, ctx, func() {}, err
		}
	}
	locked, child, cancel, err := l.ILocker.LockWithRenew(ctx, key, ttl, hold)
	if key == l.runKey && !l.before && locked && err == nil && l.gate.used.CompareAndSwap(false, true) {
		if err := l.gate.pause(child); err != nil {
			cancel()
			return false, child, func() {}, err
		}
	}
	return locked, child, cancel, err
}

type mixedRetryWindowReply struct {
	receipt *entity.ManualRetryReceipt
	err     error
}

func beginMixedRetryWindow(t *testing.T, p *currentRetryCombinationFixture, req *entity.RetryInCurrentRunRequest, before bool) (*mixedRetryWindowGate, <-chan mixedRetryWindowReply) {
	t.Helper()
	gate := &mixedRetryWindowGate{entered: make(chan struct{}), resume: make(chan struct{})}
	base := p.manager.manualRetryLocker
	p.manager.manualRetryLocker = func() lock.ILocker {
		return &mixedRetryWindowLocker{ILocker: base(), gate: gate, runKey: fmt.Sprintf("expt_run_exec_lock:%d:%d", p.f.expt, p.f.run), before: before}
	}
	ctx, cancel := context.WithCancel(context.Background())
	replies := make(chan mixedRetryWindowReply, 1)
	done := make(chan struct{})
	t.Cleanup(func() {
		cancel()
		gate.open()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("controlled retry did not exit before fixture cleanup")
		}
	})
	go func() {
		defer close(done)
		receipt, err := p.manager.RetryInCurrentRun(ctx, req)
		replies <- mixedRetryWindowReply{receipt: receipt, err: err}
	}()
	return gate, replies
}

func awaitMixedRetryWindow(t *testing.T, gate *mixedRetryWindowGate) {
	t.Helper()
	select {
	case <-gate.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("retry did not reach the controlled run-tick boundary")
	}
}

func receiveMixedRetryWindow(t *testing.T, replies <-chan mixedRetryWindowReply) *entity.ManualRetryReceipt {
	t.Helper()
	select {
	case reply := <-replies:
		require.NoError(t, reply.err)
		require.NotNil(t, reply.receipt)
		return reply.receipt
	case <-time.After(10 * time.Second):
		t.Fatal("controlled retry did not finish after barrier release")
		return nil
	}
}

func prepareMixedRetryWindowIDs(t *testing.T, p *currentRetryCombinationFixture) {
	t.Helper()
	ids := &mixedRetrySequenceLogIDs{IIDGenerator: p.manager.idgenerator}
	ids.next.Store(p.f.base + 20000)
	p.manager.manualRetryRepo = exptrepo.NewExptManualRetryRepo(p.f.provider, ids)
	require.NoError(t, p.f.conn.Unscoped().Where("space_id=? AND expt_id=? AND deleted_at IS NOT NULL", p.f.space, p.f.expt).Delete(&model.ExptTurnEvaluatorResultRef{}).Error)
}

func awaitMixedRetryWindowLocks(t *testing.T, p *currentRetryCombinationFixture, items ...int64) {
	t.Helper()
	require.Eventually(t, func() bool {
		if p.server.Exists(fmt.Sprintf("expt_run_exec_lock:%d:%d", p.f.expt, p.f.run)) {
			return false
		}
		for _, item := range items {
			if p.server.Exists(fmt.Sprintf("expt_item_eval_run_lock:%d:%d", p.f.expt, item)) {
				return false
			}
		}
		return true
	}, 2*time.Second, time.Millisecond)
}

func mixedRetryWindowSource(t *testing.T, p *currentRetryCombinationFixture, item int64) *entity.ExptCurrentItemRunSource {
	t.Helper()
	source, err := p.f.port.GetCurrentItemRunSource(context.Background(), p.f.space, p.f.expt, p.f.run, item)
	require.NoError(t, err)
	require.NotNil(t, source)
	return source
}

func mixedRetryWindowQueuedSQL(t *testing.T, p *currentRetryCombinationFixture, item int64, mode entity.ExptRunMode) (*entity.ExptCurrentItemRunSource, model.ExptItemResultRunLog) {
	t.Helper()
	source := mixedRetryWindowSource(t, p, item)
	require.NotEmpty(t, source.Current.RequestID)
	require.Equal(t, entity.ItemRunState_Queueing, source.Current.Status)
	require.Equal(t, mode, source.Current.Mode)
	require.Empty(t, source.CurrentRefs)
	var row model.ExptItemResultRunLog
	require.NoError(t, p.f.conn.Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", p.f.space, p.f.expt, p.f.run, item).Take(&row).Error)
	require.Equal(t, int32(entity.ItemRunState_Queueing), row.Status)
	require.Equal(t, int32(entity.ExptItemResultStateDefault), gptr.Indirect(row.ResultState))
	require.Equal(t, int32(entity.QuotaReservationStateNone), row.QuotaReservationState)
	require.Equal(t, p.f.run, row.ExptRunID)
	return source, row
}

func assertMixedRetryWindowStats(t *testing.T, p *currentRetryCombinationFixture, global, current [4]int32) {
	t.Helper()
	var stats model.ExptStats
	require.NoError(t, p.f.conn.Where("space_id=? AND expt_id=?", p.f.space, p.f.expt).Take(&stats).Error)
	require.Equal(t, global, [4]int32{stats.PendingCnt, stats.ProcessingCnt, stats.SuccessCnt, stats.FailCnt})
	require.Zero(t, stats.TerminatedCnt)
	run := p.identity(t)
	require.Equal(t, current, [4]int32{run.PendingCnt, run.ProcessingCnt, run.SuccessCnt, run.FailCnt})
	require.Zero(t, run.TerminatedCnt)
}

func TestMixedRetryWindowFailScanThenNewFailure(t *testing.T) {
	p := newCurrentRetryCombinationFixture(t, entity.EvaluationModeRetryAll)
	prepareMixedRetryWindowIDs(t, p)
	f := p.f
	a, b := f.item, f.item+1
	p.completeAndPublish(t, a, entity.ItemRunState_Fail)
	awaitMixedRetryWindowLocks(t, p, a, b)
	require.Equal(t, entity.ItemRunState_Processing, mixedRetryWindowSource(t, p, b).Current.Status)
	originalChunks := p.identity(t).ItemIds
	gate, replies := beginMixedRetryWindow(t, p, p.request(entity.EvaluationModeFailRetry), true)
	awaitMixedRetryWindow(t, gate) // the complete candidate plan now exists
	enumeratedAtMS := time.Now().UnixMilli()
	require.False(t, p.server.Exists(fmt.Sprintf("expt_run_exec_lock:%d:%d", f.expt, f.run)), "pause must precede the actual run lock attempt")
	p.completeAndPublish(t, b, entity.ItemRunState_Fail)
	awaitMixedRetryWindowLocks(t, p, a, b)
	bPublishedAtMS := time.Now().UnixMilli()
	bTerminal := mixedRetryWindowSource(t, p, b)
	require.Equal(t, entity.ItemRunState_Fail, bTerminal.Current.Status)
	var bBefore model.ExptItemResultRunLog
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", f.space, f.expt, f.run, b).Take(&bBefore).Error)
	require.Equal(t, int32(entity.ExptItemResultStateResulted), gptr.Indirect(bBefore.ResultState))
	gate.open()
	q1 := receiveMixedRetryWindow(t, replies)
	require.Equal(t, int32(1), q1.AcceptedCount)
	require.Empty(t, q1.RejectedItems)
	require.Equal(t, f.run, q1.RunID)
	require.Positive(t, q1.SnapshotAtMS)
	require.LessOrEqual(t, q1.SnapshotAtMS, enumeratedAtMS)
	require.LessOrEqual(t, enumeratedAtMS, bPublishedAtMS)
	awaitMixedRetryWindowLocks(t, p, a, b)
	q1A, q1ARow := mixedRetryWindowQueuedSQL(t, p, a, entity.EvaluationModeFailRetry)
	require.Equal(t, bTerminal, mixedRetryWindowSource(t, p, b), "post-enumeration B failure must remain untouched by Q1")
	var bAfter model.ExptItemResultRunLog
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", f.space, f.expt, f.run, b).Take(&bAfter).Error)
	require.Equal(t, bBefore, bAfter)
	assertMixedRetryWindowStats(t, p, [4]int32{1, 0, 0, 1}, [4]int32{1, 0, 0, 1})
	q2, err := p.manager.RetryInCurrentRun(context.Background(), p.request(entity.EvaluationModeFailRetry))
	require.NoError(t, err)
	require.Equal(t, int32(1), q2.AcceptedCount)
	require.Empty(t, q2.RejectedItems)
	awaitMixedRetryWindowLocks(t, p, a, b)
	q2B, _ := mixedRetryWindowQueuedSQL(t, p, b, entity.EvaluationModeFailRetry)
	unchangedA, unchangedARow := mixedRetryWindowQueuedSQL(t, p, a, entity.EvaluationModeFailRetry)
	require.Equal(t, q1A, unchangedA)
	require.Equal(t, q1ARow, unchangedARow)
	require.NotEqual(t, q1A.Current.RequestID, q2B.Current.RequestID)
	require.Equal(t, originalChunks, p.identity(t).ItemIds)
	assertMixedRetryWindowStats(t, p, [4]int32{2, 0, 0, 0}, [4]int32{2, 0, 0, 0})
	t.Logf("MIXED_WINDOW_REAL_MYSQL S05 enum_then_B_Resulted_then_Q1_A_only_then_Q2_B_only=true run=%d A=%d B=%d Q1=%s Q2=%s snapshot_ms=%d enumerated_ms=%d B_published_ms=%d pagination_atomic_cutoff_not_tested=true", f.run, a, b, q1A.Current.RequestID, q2B.Current.RequestID, q1.SnapshotAtMS, enumeratedAtMS, bPublishedAtMS)
}

func TestMixedRetryWindowDifferentItemsCompeteForRunTick(t *testing.T) {
	for _, firstName := range []string{"A_first", "C_first"} {
		t.Run(firstName, func(t *testing.T) {
			p := newCurrentRetryCombinationFixture(t, entity.EvaluationModeRetryItems)
			prepareMixedRetryWindowIDs(t, p)
			f := p.f
			a, b, c, d := f.item, f.item+1, f.item+2, f.item+3
			// Initial fixture only: C is historical Success outside R; D
			// stays Queueing and B stays Processing to keep the run active.
			for _, table := range []string{"expt_item_result_run_log", "expt_turn_result_run_log"} {
				require.NoError(t, f.conn.Exec("DELETE FROM "+table+" WHERE space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", f.space, f.expt, f.run, c).Error)
			}
			require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, c).Updates(map[string]any{"expt_run_id": p.oldRun, "status": int32(entity.ItemRunState_Success)}).Error)
			require.NoError(t, f.conn.Model(&model.ExptTurnResult{}).Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, c).Updates(map[string]any{"expt_run_id": p.oldRun, "status": int32(entity.TurnRunState_Success)}).Error)
			chunks := p.identity(t).ItemIds
			require.Len(t, chunks, 2)
			chunks[1].ItemIDs = []int64{d}
			encoded, err := entity.EncodeExptRunLogItems(chunks)
			require.NoError(t, err)
			require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, f.run).Updates(map[string]any{"item_ids": encoded, "pending_cnt": 1, "processing_cnt": 2, "success_cnt": 0, "fail_cnt": 0}).Error)
			require.NoError(t, f.conn.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", f.space, f.expt).Updates(map[string]any{"pending_cnt": 1, "processing_cnt": 2, "success_cnt": 1, "fail_cnt": 0}).Error)
			p.completeAndPublish(t, a, entity.ItemRunState_Success)
			awaitMixedRetryWindowLocks(t, p, a, b, c, d)
			untouchedB, untouchedD := mixedRetryWindowSource(t, p, b), mixedRetryWindowSource(t, p, d)
			first, second := a, c
			if firstName == "C_first" {
				first, second = c, a
			}
			request := func(item int64) *entity.RetryInCurrentRunRequest {
				return &entity.RetryInCurrentRunRequest{SpaceID: f.space, ExptID: f.expt, ExpectedRunID: f.run, Mode: entity.EvaluationModeRetryItems, ItemIDs: []int64{item}, Session: p.event.Session}
			}
			runKey := fmt.Sprintf("expt_run_exec_lock:%d:%d", f.expt, f.run)
			require.False(t, p.server.Exists(runKey))
			before := f.snapshot()
			gate, replies := beginMixedRetryWindow(t, p, request(first), false)
			awaitMixedRetryWindow(t, gate) // actual SETNX/renew lease succeeded
			require.True(t, p.server.Exists(runKey))
			owner, err := p.server.Get(runKey)
			require.NoError(t, err)
			require.NotEmpty(t, owner)
			busy, err := p.manager.RetryInCurrentRun(context.Background(), request(second))
			require.ErrorContains(t, err, "retry_busy")
			require.Nil(t, busy)
			require.Equal(t, before, f.snapshot(), "losing the actual held tick must perform zero SQL writes")
			ownerAfter, err := p.server.Get(runKey)
			require.NoError(t, err)
			require.Equal(t, owner, ownerAfter)
			gate.open()
			q1 := receiveMixedRetryWindow(t, replies)
			require.Equal(t, int32(1), q1.AcceptedCount)
			require.Empty(t, q1.RejectedItems)
			awaitMixedRetryWindowLocks(t, p, a, b, c, d)
			firstSource, firstRow := mixedRetryWindowQueuedSQL(t, p, first, entity.EvaluationModeRetryItems)
			firstRun := p.identity(t)
			if first == a {
				require.Equal(t, []int64{a, b, d}, firstRun.GetItemIDs())
				assertMixedRetryWindowStats(t, p, [4]int32{2, 1, 1, 0}, [4]int32{2, 1, 0, 0})
			} else {
				require.Equal(t, []int64{a, b, d, c}, firstRun.GetItemIDs())
				assertMixedRetryWindowStats(t, p, [4]int32{2, 1, 1, 0}, [4]int32{2, 1, 1, 0})
			}
			secondItem, err := f.itemRepo().BatchGet(context.Background(), f.space, f.expt, []int64{second})
			require.NoError(t, err)
			require.Len(t, secondItem, 1)
			require.Equal(t, entity.ItemRunState_Success, secondItem[0].Status, "busy caller rereads the terminal source before making a fresh request")
			q2, err := p.manager.RetryInCurrentRun(context.Background(), request(second))
			require.NoError(t, err)
			require.Equal(t, int32(1), q2.AcceptedCount)
			require.Empty(t, q2.RejectedItems)
			awaitMixedRetryWindowLocks(t, p, a, b, c, d)
			secondSource, _ := mixedRetryWindowQueuedSQL(t, p, second, entity.EvaluationModeRetryItems)
			preservedSource, preservedRow := mixedRetryWindowQueuedSQL(t, p, first, entity.EvaluationModeRetryItems)
			require.Equal(t, firstSource, preservedSource)
			require.Equal(t, firstRow, preservedRow)
			require.NotEqual(t, firstSource.Current.RequestID, secondSource.Current.RequestID)
			finalRun := p.identity(t)
			require.Equal(t, []int64{a, b, d, c}, finalRun.GetItemIDs())
			require.Equal(t, chunks, finalRun.ItemIds[:2], "both original chunks, including unknown fields, survive the accepted union")
			assertMixedRetryWindowStats(t, p, [4]int32{3, 1, 0, 0}, [4]int32{3, 1, 0, 0})
			require.Equal(t, untouchedB, mixedRetryWindowSource(t, p, b))
			require.Equal(t, untouchedD, mixedRetryWindowSource(t, p, d))
			t.Logf("MIXED_WINDOW_REAL_MYSQL C02 order=%s actual_tick_held=true busy_zero_SQL=true fresh_second_accepted=true run=%d first=%d second=%d Q1=%s Q2=%s quota_none_direct_SQL=true union_preserved=true", firstName, f.run, first, second, firstSource.Current.RequestID, secondSource.Current.RequestID)
		})
	}
}
