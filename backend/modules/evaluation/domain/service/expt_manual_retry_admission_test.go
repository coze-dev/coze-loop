// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"

	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestManualRetryAdmissionReceiptBeforeDependencies(t *testing.T) {
	req := admissionRequest()
	req.ItemIDs = []int64{9223372036854775806, 11, 11}
	_, hash, err := normalizeCurrentRunRetry(req)
	require.NoError(t, err)
	raw := `[{"ItemIDs":[11],"manual_retry_receipt":{"request_id":"72cc8921-40f4-4a43-b772-aaf2b5dcdf05","payload_hash":"` + hash + `","accepted_count":1,"rejected_items":[{"item_id":9223372036854775806,"reason":"busy"}],"snapshot_at_ms":1700000000123}},{"manual_retry_receipt":{"request_id":"later","payload_hash":"other","accepted_count":0}}]`
	port := &admissionMemoryRepo{}
	require.NoError(t, json.Unmarshal([]byte(raw), &port.chunks))
	// All record/config/experiment dependencies absent: replay must precede them.
	manager := &ExptMangerImpl{manualRetryRepo: port}
	got, err := manager.RetryInCurrentRun(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, int32(1), got.AcceptedCount)
	require.Equal(t, int64(1700000000123), got.SnapshotAtMS)
	require.Equal(t, int64(9223372036854775806), got.RejectedItems[0].ItemID)
	req.ItemIDs = []int64{11, 9223372036854775806}
	again, err := manager.RetryInCurrentRun(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, got, again)
	req.Ext = map[string]string{"business": "changed"}
	_, err = manager.RetryInCurrentRun(context.Background(), req)
	require.ErrorContains(t, err, "request_conflict")
}

type admissionReceiptRepo struct{}

func (*admissionReceiptRepo) FindReceipt(context.Context, int64, int64, int64, string, string) (*entity.ManualRetryReceipt, error) {
	return nil, nil
}

func (*admissionReceiptRepo) RequeueTerminalItems(context.Context, *entity.RequeueTerminalItemsRequest) (*entity.ManualRetryReceipt, error) {
	return nil, errors.New("unexpected write")
}

func admissionRequest() *entity.RetryInCurrentRunRequest {
	return &entity.RetryInCurrentRunRequest{SpaceID: 9, ExptID: 7, ExpectedRunID: 8, RequestID: "72cc8921-40f4-4a43-b772-aaf2b5dcdf05", Mode: entity.EvaluationModeRetryItems, ItemIDs: []int64{11}, Session: &entity.Session{UserID: "operator"}}
}

func TestManualRetryAdmissionGuards(t *testing.T) {
	for _, kind := range []string{"disabled", "bad_uuid", "malformed_uuid", "internal_ext", "failure_selection", "empty_selection", "retry_all", "negative_item", "foreign_run", "unknown_run", "pending", "terminating", "draining", "online", "run_busy", "lost_run_lock"} {
		t.Run(kind, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			config := componentmocks.NewMockIConfiger(ctrl)
			expts := repomocks.NewMockIExperimentRepo(ctrl)
			items := repomocks.NewMockIExptItemResultRepo(ctrl)
			mutex := lockmocks.NewMockILocker(ctrl)
			port := &admissionReceiptRepo{}
			manager := &ExptMangerImpl{manualRetryRepo: port, configer: config, exptRepo: expts, itemResultRepo: items, mutex: mutex, manualRetryLocker: func() lock.ILocker { return mutex }}
			req := admissionRequest()
			got := &entity.Experiment{ID: 7, SpaceID: 9, LatestRunID: 8, Status: entity.ExptStatus_Processing, ExptType: entity.ExptType_Offline}
			shape := false
			switch kind {
			case "bad_uuid":
				req.RequestID = "not-uuid"
				shape = true
			case "malformed_uuid":
				req.RequestID = "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
				shape = true
			case "internal_ext":
				req.Ext = map[string]string{entity.ManualRetryExtKey: "forged"}
				shape = true
			case "failure_selection":
				req.Mode = entity.EvaluationModeFailRetry
				shape = true
			case "empty_selection":
				req.ItemIDs = nil
				shape = true
			case "retry_all":
				req.Mode = entity.EvaluationModeRetryAll
				shape = true
			case "negative_item":
				req.ItemIDs = []int64{-1}
				shape = true
			case "foreign_run":
				got.LatestRunID = 18
			case "unknown_run":
				req.ExpectedRunID = got.LatestRunID + 1
			case "pending":
				got.Status = entity.ExptStatus_Pending
			case "terminating":
				got.Status = entity.ExptStatus_Terminating
			case "draining":
				got.Status = entity.ExptStatus_Draining
			case "online":
				got.ExptType = entity.ExptType_Online
			}
			if !shape {
				config.EXPECT().GetExptExecConf(gomock.Any(), int64(9)).Return(&entity.ExptExecConf{RunningRetryEnabled: gptr.Of(kind != "disabled")})
				if kind != "disabled" {
					expts.EXPECT().GetByID(gomock.Any(), int64(7), int64(9)).Return(got, nil)
				}
			}
			if kind == "run_busy" || kind == "lost_run_lock" {
				items.EXPECT().BatchGet(gomock.Any(), int64(9), int64(7), []int64{11}).Return(nil, nil)
				lockCtx, cancel := context.WithCancel(context.Background())
				defer cancel()
				if kind == "lost_run_lock" {
					cancel()
				}
				mutex.EXPECT().LockWithRenew(gomock.Any(), "expt_run_exec_lock:7:8", gomock.Any(), gomock.Any()).Return(kind != "run_busy", lockCtx, func() {}, nil)
			}
			receipt, err := manager.RetryInCurrentRun(context.Background(), req)
			require.Error(t, err)
			require.Nil(t, receipt)
			if kind == "run_busy" {
				require.ErrorContains(t, err, "retry_busy")
			}
			if kind == "foreign_run" || kind == "unknown_run" {
				require.ErrorContains(t, err, "run_conflict")
			}
			if kind == "lost_run_lock" {
				require.ErrorIs(t, err, context.Canceled)
			}
			if kind != "lost_run_lock" {
				biz, ok := kerrors.FromBizStatusError(err)
				require.True(t, ok, "admission error must survive the PacketAdapter BizStatus boundary")
				expected := int32(601204102)
				switch {
				case shape:
					expected = 601200202
				case kind == "disabled":
					expected = 601204106
				case kind == "foreign_run" || kind == "unknown_run":
					expected = 601204101
				case kind == "run_busy":
					expected = 601204105
				}
				require.Equal(t, expected, biz.BizStatusCode())
			}
		})
	}
}

func TestManualRetryPayloadIdentity(t *testing.T) {
	a := admissionRequest()
	a.ItemIDs = []int64{9223372036854775806, 11, 11}
	a.Ext = map[string]string{"z": "2", "a": "1"}
	normalized, hash, err := normalizeCurrentRunRetry(a)
	require.NoError(t, err)
	require.Equal(t, []int64{11, 9223372036854775806}, normalized.ItemIDs)
	b := admissionRequest()
	b.ItemIDs = []int64{11, 9223372036854775806}
	b.Ext = map[string]string{"a": "1", "z": "2"}
	_, same, err := normalizeCurrentRunRetry(b)
	require.NoError(t, err)
	require.Equal(t, hash, same)
	b.ItemIDs[1]--
	_, different, err := normalizeCurrentRunRetry(b)
	require.NoError(t, err)
	require.NotEqual(t, hash, different)
	b.ItemIDs = a.ItemIDs
	b.Ext["z"] = "3"
	_, different, err = normalizeCurrentRunRetry(b)
	require.NoError(t, err)
	require.NotEqual(t, hash, different)
	a.Ext["a"] = "changed"
	require.Equal(t, "1", normalized.Ext["a"])
	require.Len(t, hash, 64)
}

func (r *admissionReceiptRepo) PreflightTerminalItems(context.Context, *entity.RequeueTerminalItemsRequest) (*entity.ManualRetryPreparation, error) {
	return nil, errors.New("unexpected preflight")
}

type admissionMemoryRepo struct {
	chunks            []entity.ExptRunLogItems
	calls             []string
	preflightError    error
	commitError       error
	preflightRejected bool
	afterPreflight    func()
	beforeCommit      func()
}

func (r *admissionMemoryRepo) FindReceipt(_ context.Context, _, _, _ int64, id, hash string) (*entity.ManualRetryReceipt, error) {
	r.calls = append(r.calls, "receipt")
	for _, c := range r.chunks {
		if p := c.ManualRetryReceipt; p != nil && p.RequestID == id {
			if p.PayloadHash != hash {
				return nil, errors.New("request_conflict")
			}
			return p, nil
		}
	}
	return nil, nil
}

func (r *admissionMemoryRepo) PreflightTerminalItems(_ context.Context, req *entity.RequeueTerminalItemsRequest) (*entity.ManualRetryPreparation, error) {
	r.calls = append(r.calls, "preflight")
	if r.afterPreflight != nil {
		defer r.afterPreflight()
	}
	if r.preflightError != nil {
		return nil, r.preflightError
	}
	plan := &entity.ManualRetryPreparation{Items: req.Items, RejectedItems: req.RejectedItems, SnapshotAtMS: req.SnapshotAtMS}
	if r.preflightRejected {
		plan.Items = nil
		for _, item := range req.Items {
			plan.RejectedItems = append(plan.RejectedItems, &entity.ManualRetryRejectedItem{ItemID: item.SourceItem.ItemID, Reason: "source_changed"})
		}
	}
	return plan, nil
}

func (r *admissionMemoryRepo) RequeueTerminalItems(ctx context.Context, req *entity.RequeueTerminalItemsRequest) (*entity.ManualRetryReceipt, error) {
	r.calls = append(r.calls, "commit")
	if r.beforeCommit != nil {
		r.beforeCommit()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.commitError != nil {
		return nil, r.commitError
	}
	receipt := &entity.ManualRetryReceipt{RequestID: req.RequestID, PayloadHash: req.PayloadHash, AcceptedCount: int32(len(req.Items)), RejectedItems: req.RejectedItems, SnapshotAtMS: req.SnapshotAtMS}
	// Encode and decode the storage representation; assertions cannot pass through pointer identity.
	raw, err := json.Marshal(append(r.chunks, entity.ExptRunLogItems{ManualRetryReceipt: receipt}))
	if err != nil {
		return nil, err
	}
	requireDecode := json.Unmarshal(raw, &r.chunks)
	if requireDecode != nil {
		return nil, requireDecode
	}
	return r.chunks[len(r.chunks)-1].ManualRetryReceipt, nil
}

type admissionWaker struct {
	calls int
	err   error
}

func (w *admissionWaker) WakeCurrentRun(context.Context, string) error { w.calls++; return w.err }

func TestManualRetryAdmissionCommitOrdering(t *testing.T) {
	for _, kind := range []string{"legacy", "item_busy", "lost_item_lock", "lock_state_changed", "locked_receipt", "released", "absent", "mismatch", "release_error", "preflight_capacity", "preflight_source_changed", "db_error", "wake_error", "receipt_after_close", "receipt_after_replacement", "lost_during_preflight", "lost_after_release"} {
		t.Run(kind, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			config := componentmocks.NewMockIConfiger(ctrl)
			expts := repomocks.NewMockIExperimentRepo(ctrl)
			items := repomocks.NewMockIExptItemResultRepo(ctrl)
			turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
			refs := repomocks.NewMockIExptItemRefRepo(ctrl)
			runs := repomocks.NewMockIExptRunLogRepo(ctrl)
			mutex := lockmocks.NewMockILocker(ctrl)
			guard := componentmocks.NewMockICentralReservationGuard(ctrl)
			publisher := eventmocks.NewMockExptEventPublisher(ctrl)
			waker := &admissionWaker{}
			port := &admissionMemoryRepo{}
			req := admissionRequest()
			req.Ext = map[string]string{"business": "unchanged"}
			manager := &ExptMangerImpl{manualRetryRepo: port, configer: config, exptRepo: expts, itemResultRepo: items, turnResultRepo: turns, itemRefRepo: refs, runLogRepo: runs, mutex: mutex, centralGuard: guard, publisher: publisher, currentRunWaker: waker, manualRetryLocker: func() lock.ILocker { return mutex }}
			expt := &entity.Experiment{ID: 7, SpaceID: 9, LatestRunID: 8, Status: entity.ExptStatus_Processing, ExptType: entity.ExptType_Offline, EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig, EvalConf: &entity.EvaluationConfiguration{ItemRetryNum: gptr.Of(3), ItemConcurNum: gptr.Of(4)}}
			enforce := kind == "released" || kind == "absent" || kind == "mismatch" || kind == "release_error" || kind == "preflight_capacity" || kind == "preflight_source_changed" || kind == "lost_after_release" || kind == "wake_error"
			if enforce {
				expt.ExptDispatchMode = entity.ExptDispatchModeEnforce
				expt.SchedulerScope = "test-scope"
			}
			run := &entity.ExptRunLog{ID: 8, SpaceID: 9, ExptID: 7, ExptRunID: 8, Status: int64(entity.ExptStatus_Processing), Mode: int32(entity.EvaluationModeSubmit), CreatedBy: "original"}
			config.EXPECT().GetExptExecConf(gomock.Any(), int64(9)).Return(&entity.ExptExecConf{RunningRetryEnabled: gptr.Of(true)})
			expts.EXPECT().GetByID(gomock.Any(), int64(7), int64(9)).Return(expt, nil)
			oldRun := int64(6)
			oldToken := "prior-request"
			ext := map[string]string{"other": "keep", entity.ManualRetryExtKey: `{"run_id":6,"request_id":"prior-request","mode":5,"operator":"old","accepted_at_ms":1}`}
			source := &entity.ExptItemResult{ID: 101, SpaceID: 9, ExptID: 7, ExptRunID: oldRun, ItemID: 11, ItemVersionID: 91, Status: entity.ItemRunState_Fail, Ext: ext}
			items.EXPECT().BatchGet(gomock.Any(), int64(9), int64(7), []int64{11}).Return([]*entity.ExptItemResult{source}, nil)
			items.EXPECT().MGetItemRunLog(gomock.Any(), int64(7), int64(8), []int64{11}, int64(9)).Return(nil, nil)
			refs.EXPECT().GetByExptIDAndItemID(gomock.Any(), int64(9), int64(7), int64(11)).Return(&entity.ExptItemRef{SpaceID: 9, ExptID: 7, ItemID: 11, ItemVersionID: 91}, nil)
			turns.EXPECT().GetItemTurnResults(gomock.Any(), int64(7), int64(11), int64(9)).Return([]*entity.ExptTurnResult{{ID: 301, SpaceID: 9, ExptID: 7, ExptRunID: 6, ItemID: 11, ItemVersionID: 91, TurnID: 21, Status: 2}}, nil)
			turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), int64(7), int64(8), int64(11), int64(9)).Return(nil, nil)
			turns.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), int64(9), []int64{301}).Return(nil, nil)
			runCtx, runCancel := context.WithCancel(context.Background())
			defer runCancel()
			if kind == "lost_during_preflight" {
				port.afterPreflight = runCancel
			}
			mutex.EXPECT().LockWithRenew(gomock.Any(), "expt_run_exec_lock:7:8", 5*time.Second, 5*time.Minute).DoAndReturn(func(context.Context, string, time.Duration, time.Duration) (bool, context.Context, func(), error) {
				if kind == "locked_receipt" {
					_, hash, _ := normalizeCurrentRunRetry(req)
					port.chunks = []entity.ExptRunLogItems{{ManualRetryReceipt: &entity.ManualRetryReceipt{RequestID: req.RequestID, PayloadHash: hash, AcceptedCount: 0, SnapshotAtMS: 77}}}
				}
				return true, runCtx, runCancel, nil
			})
			if kind != "locked_receipt" {
				expts.EXPECT().GetByID(gomock.Any(), int64(7), int64(9)).DoAndReturn(func(context.Context, int64, int64) (*entity.Experiment, error) {
					if kind == "lock_state_changed" {
						copy := *expt
						copy.LatestRunID = 18
						return &copy, nil
					}
					return expt, nil
				})
			}
			if kind != "locked_receipt" && kind != "lock_state_changed" {
				runs.EXPECT().Get(gomock.Any(), int64(7), int64(8)).Return(run, nil)
				itemCtx, itemCancel := context.WithCancel(runCtx)
				defer itemCancel()
				mutex.EXPECT().LockWithRenew(gomock.Any(), "expt_item_eval_run_lock:7:11", 5*time.Second, 5*time.Minute).DoAndReturn(func(context.Context, string, time.Duration, time.Duration) (bool, context.Context, func(), error) {
					if kind == "lost_item_lock" {
						itemCancel()
					}
					return kind != "item_busy", itemCtx, itemCancel, nil
				})
			}
			if kind == "preflight_capacity" {
				port.preflightError = errors.New("receipt capacity exceeded")
			}
			if kind == "preflight_source_changed" {
				port.preflightRejected = true
			}
			if kind == "db_error" {
				port.commitError = errors.New("database commit failed")
			}
			if kind == "wake_error" {
				waker.err = errors.New("wake unavailable")
			}
			if enforce && kind != "preflight_capacity" && kind != "preflight_source_changed" {
				guard.EXPECT().ReleaseTerminalReservation(gomock.Any(), "test-scope", oldRun, int64(11), oldToken).DoAndReturn(func(context.Context, string, int64, int64, string) (component.TerminalReservationReleaseResult, error) {
					require.Equal(t, "preflight", port.calls[len(port.calls)-1])
					port.calls = append(port.calls, "release")
					switch kind {
					case "absent":
						return component.TerminalReservationAbsent, nil
					case "mismatch":
						return component.TerminalReservationMismatch, nil
					case "release_error":
						return 0, errors.New("redis unavailable")
					case "lost_after_release":
						runCancel()
						return component.TerminalReservationReleased, nil
					}
					return component.TerminalReservationReleased, nil
				})
			}
			failures := kind == "lost_item_lock" || kind == "lock_state_changed" || kind == "release_error" || kind == "preflight_capacity" || kind == "db_error" || kind == "lost_during_preflight" || kind == "lost_after_release"
			publisher.EXPECT().PublishExptScheduleEvent(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			callerCtx, callerCancel := context.WithCancel(context.Background())
			defer callerCancel()
			got, err := manager.RetryInCurrentRun(callerCtx, req)
			require.NoError(t, callerCtx.Err())
			if failures {
				require.Error(t, err)
				require.Nil(t, got)
				require.Empty(t, port.chunks)
				return
			}
			require.NoError(t, err)
			require.Equal(t, req.RequestID, got.RequestID)
			if kind == "locked_receipt" {
				require.Equal(t, int64(77), got.SnapshotAtMS)
				require.Equal(t, []string{"receipt", "receipt"}, port.calls)
				return
			}
			expected := int32(1)
			if kind == "item_busy" || kind == "mismatch" || kind == "preflight_source_changed" {
				expected = 0
			}
			require.Equal(t, expected, got.AcceptedCount)
			require.Equal(t, "commit", port.calls[len(port.calls)-1])
			require.Equal(t, got, port.chunks[len(port.chunks)-1].ManualRetryReceipt)
			require.Equal(t, int32(entity.EvaluationModeSubmit), run.Mode)
			require.Equal(t, "original", run.CreatedBy)
			if expected == 0 {
				require.Len(t, got.RejectedItems, 1)
			}
			require.Equal(t, entity.ExptStatus_Processing, expt.Status)
			require.Equal(t, int64(8), expt.LatestRunID)
			require.Equal(t, 4, *expt.EvalConf.ItemConcurNum)
			if enforce {
				require.Equal(t, 1, waker.calls)
			}
			if kind == "receipt_after_close" || kind == "receipt_after_replacement" {
				expt.Status = entity.ExptStatus_Success
				if kind == "receipt_after_replacement" {
					expt.LatestRunID = 18
					expt.Status = entity.ExptStatus_Processing
				}
				manager.configer = nil
				manager.exptRepo = nil
				manager.evaluatorRecords = nil
			}
			before := len(port.calls)
			again, err := manager.RetryInCurrentRun(context.Background(), req)
			require.NoError(t, err)
			require.Equal(t, got, again)
			require.Equal(t, before+1, len(port.calls))
			conflict := *req
			conflict.Ext = map[string]string{"business": "changed"}
			_, err = manager.RetryInCurrentRun(context.Background(), &conflict)
			require.ErrorContains(t, err, "request_conflict")
		})
	}
}

func TestManualRetryAdmissionUsesIndependentLocker(t *testing.T) {
	ctrl := gomock.NewController(t)
	config := componentmocks.NewMockIConfiger(ctrl)
	expts := repomocks.NewMockIExperimentRepo(ctrl)
	items := repomocks.NewMockIExptItemResultRepo(ctrl)
	shared := lockmocks.NewMockILocker(ctrl)
	owned := lockmocks.NewMockILocker(ctrl)
	manager := &ExptMangerImpl{manualRetryRepo: &admissionReceiptRepo{}, configer: config, exptRepo: expts, itemResultRepo: items, mutex: shared}
	req := admissionRequest()
	config.EXPECT().GetExptExecConf(gomock.Any(), int64(9)).Return(&entity.ExptExecConf{RunningRetryEnabled: gptr.Of(true)})
	expts.EXPECT().GetByID(gomock.Any(), int64(7), int64(9)).Return(&entity.Experiment{ID: 7, SpaceID: 9, LatestRunID: 8, Status: entity.ExptStatus_Processing, ExptType: entity.ExptType_Offline}, nil)
	items.EXPECT().BatchGet(gomock.Any(), int64(9), int64(7), []int64{11}).Return(nil, nil)
	owned.EXPECT().LockWithRenew(gomock.Any(), "expt_run_exec_lock:7:8", gomock.Any(), gomock.Any()).Return(false, context.Background(), func() {}, nil)
	// Use a factory so an old renewal can never share ownership with the next admission.
	manager.manualRetryLocker = func() lock.ILocker { return owned }
	_, err := manager.RetryInCurrentRun(context.Background(), req)
	require.ErrorContains(t, err, "retry_busy")
}

func TestManualRetryLegacyWakeWithoutSnapshot(t *testing.T) {
	ctrl := gomock.NewController(t)
	publisher := eventmocks.NewMockExptEventPublisher(ctrl)
	publisher.EXPECT().PublishExptScheduleEvent(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	manager := &ExptMangerImpl{publisher: publisher}
	expt := &entity.Experiment{ID: 7, SpaceID: 9, LatestRunID: 8, Status: entity.ExptStatus_Processing, ExptType: entity.ExptType_Offline, EvalConf: &entity.EvaluationConfiguration{ItemRetryNum: gptr.Of(3), ItemConcurNum: gptr.Of(4)}}
	run := &entity.ExptRunLog{ID: 8, SpaceID: 9, ExptID: 7, ExptRunID: 8, Mode: int32(entity.EvaluationModeSubmit), Status: int64(entity.ExptStatus_Processing), CreatedBy: "original"}
	exptBefore, err := json.Marshal(expt)
	require.NoError(t, err)
	runBefore, err := json.Marshal(run)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = manager.wakeCurrentRetryRun(ctx, expt, run)
	require.ErrorContains(t, err, "original schedule event snapshot unavailable")
	require.NoError(t, ctx.Err())
	exptAfter, err := json.Marshal(expt)
	require.NoError(t, err)
	require.Equal(t, exptBefore, exptAfter)
	runAfter, err := json.Marshal(run)
	require.NoError(t, err)
	require.Equal(t, runBefore, runAfter)
}

func TestManualRetryLegacyWakePreservesPeriodicTick(t *testing.T) {
	ctrl := gomock.NewController(t)
	publisher := eventmocks.NewMockExptEventPublisher(ctrl)
	config := componentmocks.NewMockIConfiger(ctrl)
	manager := &ExptMangerImpl{publisher: publisher}
	event := &entity.ExptScheduleEvent{SpaceID: 9, ExptID: 7, ExptRunID: 8, ExptRunMode: entity.EvaluationModeSubmit, ExptType: entity.ExptType_Offline, CreatedAt: 1700000000, Session: &entity.Session{UserID: "original", AppID: 566307}, ItemRetryTimes: 3, Ext: map[string]string{entity.RetryYieldExtKey: "true", "business": "frozen"}}
	before, err := json.Marshal(event)
	require.NoError(t, err)
	config.EXPECT().GetExptExecConf(gomock.Any(), int64(9)).Return(&entity.ExptExecConf{DaemonIntervalSecond: 5})
	publisher.EXPECT().PublishExptScheduleEvent(gomock.Any(), event, gptr.Of(5*time.Second)).DoAndReturn(func(_ context.Context, got *entity.ExptScheduleEvent, _ *time.Duration) error {
		require.Same(t, event, got)
		encoded, err := json.Marshal(got)
		require.NoError(t, err)
		require.Equal(t, before, encoded)
		return nil
	})
	require.ErrorContains(t, manager.wakeCurrentRetryRun(context.Background(), &entity.Experiment{ID: 7, SpaceID: 9, LatestRunID: 8}, &entity.ExptRunLog{ExptRunID: 8}), "original schedule event snapshot unavailable")
	mode := &ExptSubmitExec{publisher: publisher, configer: config}
	require.NoError(t, mode.NextTick(context.Background(), event, true))
	after, err := json.Marshal(event)
	require.NoError(t, err)
	require.Equal(t, before, after)
}

func TestManualRetryDependencyErrorRetainsCause(t *testing.T) {
	ctrl := gomock.NewController(t)
	config := componentmocks.NewMockIConfiger(ctrl)
	expts := repomocks.NewMockIExperimentRepo(ctrl)
	config.EXPECT().GetExptExecConf(gomock.Any(), int64(9)).Return(&entity.ExptExecConf{RunningRetryEnabled: gptr.Of(true)}).Times(2)
	cause := errors.New("RPC read unavailable")
	expts.EXPECT().GetByID(gomock.Any(), int64(7), int64(9)).Return(nil, cause)
	expts.EXPECT().GetByID(gomock.Any(), int64(7), int64(9)).Return(nil, nil)
	manager := &ExptMangerImpl{manualRetryRepo: &admissionReceiptRepo{}, configer: config, exptRepo: expts}
	_, err := manager.RetryInCurrentRun(context.Background(), admissionRequest())
	require.Same(t, cause, err)
	_, ok := kerrors.FromBizStatusError(err)
	require.False(t, ok)
	_, err = manager.RetryInCurrentRun(context.Background(), admissionRequest())
	require.ErrorContains(t, err, "experiment missing")
	_, ok = kerrors.FromBizStatusError(err)
	require.False(t, ok)
}
