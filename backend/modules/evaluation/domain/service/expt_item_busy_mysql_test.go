// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

// Busy observations go through the same native source reader as execution;
// Redis SETNX/CAS/TTL are real locker operations over the Redis semantic fixture.
func TestItemBusyMySQLDisposition(t *testing.T) {
	for _, kind := range []string{
		"ordinary_processing_zero_version", "manual_processing", "queueing",
		"success_logged", "success_resulted", "fail_logged", "terminal_logged",
		"superseded_request", "superseded_run", "superseded_target_callback",
		"mode_conflict", "item_version_conflict", "turn_version_conflict",
		"manual_zero_version", "manual_missing_turn", "invalid_manual_ext",
		"legacy_async", "manual_async_without_source", "invalid_resume_source",
		"null_result_state", "completed_damaged_turn",
		"completed_mode_conflict", "completed_manual_async_without_source",
		"completed_invalid_resume_source", "completed_default_result_state",
	} {
		t.Run(kind, func(t *testing.T) {
			token := "rq1"
			if kind == "ordinary_processing_zero_version" || kind == "legacy_async" {
				token = ""
			}
			f := newLateWriteFixture(t, token)
			event := f.event()
			event.RequestID = token
			if token == "" {
				event.ExptRunMode = entity.EvaluationModeSubmit
			}
			ack := false
			switch kind {
			case "ordinary_processing_zero_version", "legacy_async":
				require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).Updates(map[string]any{"item_version_id": 0, "evaluator_result_ids": []byte(`null`)}).Error)
				event.AsyncReportTrigger = kind == "legacy_async"
			case "queueing":
				for _, table := range []any{&model.ExptItemResult{}, &model.ExptItemResultRunLog{}} {
					require.NoError(t, f.conn.Model(table).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", entity.ItemRunState_Queueing).Error)
				}
			case "success_logged", "success_resulted", "fail_logged", "terminal_logged", "completed_damaged_turn",
				"completed_mode_conflict", "completed_manual_async_without_source",
				"completed_invalid_resume_source", "completed_default_result_state":
				status := entity.ItemRunState_Success
				if kind == "fail_logged" {
					status = entity.ItemRunState_Fail
				}
				if kind == "terminal_logged" {
					status = entity.ItemRunState_Terminal
				}
				applied, err := f.port.CompleteCurrentItemRun(context.Background(), f.source(), &entity.ExptCurrentItemRunWrite{Status: status, FailPendingTurns: true})
				require.NoError(t, err)
				require.True(t, applied)
				ack = true
				if kind == "success_resulted" {
					require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("result_state", entity.ExptItemResultStateResulted).Error)
				}
				if kind == "completed_damaged_turn" {
					require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("id=?", f.base+500).Update("item_version_id", f.version+1).Error)
					ack = false
				}
				switch kind {
				case "completed_mode_conflict":
					event.ExptRunMode = entity.EvaluationModeRetryItems
					ack = false
				case "completed_manual_async_without_source":
					event.AsyncReportTrigger = true
					ack = false
				case "completed_invalid_resume_source":
					event.AsyncReportTrigger = true
					event.ResumeSource = &entity.ExptResumeSource{Kind: "target", TurnID: 0, RecordID: f.base + 41, InvokeID: "foreign"}
					ack = false
				case "completed_default_result_state":
					require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("id=?", f.base+200).Update("result_state", entity.ExptItemResultStateDefault).Error)
					ack = false
				}
			case "superseded_request":
				event.RequestID = "rq0"
				ack = true
			case "superseded_run":
				require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("expt_run_id", f.run+1).Error)
				// A partial projection move alone does not prove that the event is obsolete.
				ack = false
			case "superseded_target_callback":
				event.AsyncReportTrigger = true
				event.ResumeSource = &entity.ExptResumeSource{Kind: "target", TurnID: 0, RecordID: f.base + 40, InvokeID: strconv.FormatInt(f.base+40, 10)}
				ack = true
			case "mode_conflict":
				event.ExptRunMode = entity.EvaluationModeRetryItems
			case "item_version_conflict":
				require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("id=?", f.base+200).Update("item_version_id", f.version+1).Error)
			case "turn_version_conflict", "manual_zero_version":
				version := f.version + 1
				if kind == "manual_zero_version" {
					version = 0
				}
				require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("id=?", f.base+500).Update("item_version_id", version).Error)
			case "manual_missing_turn":
				require.NoError(t, f.conn.Where("id=?", f.base+500).Delete(&model.ExptTurnResultRunLog{}).Error)
			case "invalid_manual_ext":
				require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("id=?", f.base+100).Update("ext", []byte(`{"manual_retry":"broken"}`)).Error)
			case "manual_async_without_source":
				event.AsyncReportTrigger = true
			case "invalid_resume_source":
				event.AsyncReportTrigger = true
				event.ResumeSource = &entity.ExptResumeSource{Kind: "target", TurnID: 0, RecordID: f.base + 41, InvokeID: "foreign"}
			case "null_result_state":
				require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("id=?", f.base+200).Update("result_state", nil).Error)
			}
			factory, server := finishRealLockerFactory(t)
			key := fmt.Sprintf("expt_item_eval_run_lock:%d:%d", f.expt, f.item)
			holder := factory()
			locked, err := holder.Lock(context.Background(), key, time.Hour)
			require.NoError(t, err)
			require.True(t, locked)
			owner, err := server.Get(key)
			require.NoError(t, err)
			before := f.snapshot()
			svc := &ExptItemEventEvalServiceImpl{exptItemResultRepo: f.itemRepo(), exptTurnResultRepo: f.turnRepo(), itemLocker: factory}
			called := false
			endpoint := svc.HandleEventErr(svc.HandleEventLock(func(context.Context, *entity.ExptItemEvalEvent) error {
				called = true
				return nil
			}))
			got := endpoint(context.Background(), event)
			if ack {
				require.NoError(t, got, "validated completed/superseded delivery must converge while another holder owns the key")
			} else {
				require.True(t, errno.NeedMQRetry(got), "a live or damaged source cannot be ACKed: %v", got)
			}
			require.False(t, called)
			require.Equal(t, before, f.snapshot(), "busy observation must not mutate item, turns, stats or provider records")
			gotOwner, err := server.Get(key)
			require.NoError(t, err)
			require.Equal(t, owner, gotOwner)
		})
	}
}

// A SQL row holder must not make a lock-busy observation wait indefinitely.
func TestItemBusyMySQLObservationDeadline(t *testing.T) {
	f := newLateWriteFixture(t, "rq1")
	applied, err := f.port.CompleteCurrentItemRun(context.Background(), f.source(), &entity.ExptCurrentItemRunWrite{Status: entity.ItemRunState_Success, FailPendingTurns: true})
	require.NoError(t, err)
	require.True(t, applied)
	factory, server := finishRealLockerFactory(t)
	key := fmt.Sprintf("expt_item_eval_run_lock:%d:%d", f.expt, f.item)
	require.NoError(t, server.Set(key, "filter-holder"))
	tx := f.conn.Begin()
	require.NoError(t, tx.Error)
	defer tx.Rollback()
	require.NoError(t, tx.Exec("SELECT id FROM expt_item_result WHERE space_id=? AND expt_id=? AND item_id=? FOR UPDATE", f.space, f.expt, f.item).Error)
	svc := &ExptItemEventEvalServiceImpl{exptItemResultRepo: f.itemRepo(), itemLocker: factory}
	start := time.Now()
	err = svc.HandleEventErr(svc.HandleEventLock(func(context.Context, *entity.ExptItemEvalEvent) error {
		t.Fatal("SQL-blocked busy delivery must not execute")
		return nil
	}))(context.Background(), f.event())
	require.True(t, errno.NeedMQRetry(err))
	require.Less(t, time.Since(start), 3*time.Second)
}

func TestItemBusyMySQLRetainsUncommittedAdmission(t *testing.T) {
	f := newLateWriteFixture(t, "")
	factory, server := finishRealLockerFactory(t)
	itemID := f.item + 1000
	key := fmt.Sprintf("expt_item_eval_run_lock:%d:%d", f.expt, itemID)
	require.NoError(t, server.Set(key, "dispatch-transaction"))
	tx := f.conn.Begin()
	require.NoError(t, tx.Error)
	defer tx.Rollback()
	require.NoError(t, tx.Create(&model.ExptItemResult{ID: f.base + 1800, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: itemID, ItemVersionID: f.version, Status: int32(entity.ItemRunState_Queueing)}).Error)
	state := int32(entity.ExptItemResultStateDefault)
	require.NoError(t, tx.Create(&model.ExptItemResultRunLog{ID: f.base + 1801, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: itemID, ItemVersionID: f.version, Status: int32(entity.ItemRunState_Queueing), ResultState: &state}).Error)
	event := f.event()
	event.EvalSetItemID, event.RequestID, event.ExptRunMode = itemID, "", entity.EvaluationModeSubmit
	visible, err := f.itemRepo().GetCurrentRequest(context.Background(), f.space, f.expt, f.run, itemID)
	require.NoError(t, err)
	require.Nil(t, visible)
	svc := &ExptItemEventEvalServiceImpl{exptItemResultRepo: f.itemRepo(), itemLocker: factory}
	err = svc.HandleEventErr(svc.HandleEventLock(func(context.Context, *entity.ExptItemEvalEvent) error {
		t.Fatal("an uncommitted admission cannot execute")
		return nil
	}))(context.Background(), event)
	require.True(t, errno.NeedMQRetry(err), "absence before another holder commits is not proof of an obsolete delivery")
	require.NoError(t, tx.Commit().Error)
	visible, err = f.itemRepo().GetCurrentRequest(context.Background(), f.space, f.expt, f.run, itemID)
	require.NoError(t, err)
	require.NotNil(t, visible)
	require.Equal(t, entity.ItemRunState_Queueing, visible.Status)
	owner, err := server.Get(key)
	require.NoError(t, err)
	require.Equal(t, "dispatch-transaction", owner)
}

func TestItemBusyMySQLRetainsReplacementWhileSourceTransactionIsOpen(t *testing.T) {
	f := newLateWriteFixture(t, "rq1")
	factory, server := finishRealLockerFactory(t)
	key := fmt.Sprintf("expt_item_eval_run_lock:%d:%d", f.expt, f.item)
	require.NoError(t, server.Set(key, "manual-admission"))
	tx := f.conn.Begin()
	require.NoError(t, tx.Error)
	defer tx.Rollback()
	// Keep the existing row visible as rq1 until the replacement metadata commits.
	manual, err := json.Marshal(&entity.ManualRetryContext{RunID: f.run, RequestID: "rq2", Mode: entity.EvaluationModeFailRetry, Operator: "test", AcceptedAtMS: 1})
	require.NoError(t, err)
	ext, err := json.Marshal(map[string]string{entity.ManualRetryExtKey: string(manual)})
	require.NoError(t, err)
	require.NoError(t, tx.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("ext", ext).Error)
	event := f.event()
	event.RequestID = "rq2"
	visible, err := f.itemRepo().GetCurrentRequest(context.Background(), f.space, f.expt, f.run, f.item)
	require.NoError(t, err)
	require.Equal(t, "rq1", visible.RequestID)
	svc := &ExptItemEventEvalServiceImpl{exptItemResultRepo: f.itemRepo(), itemLocker: factory}
	start := time.Now()
	err = svc.HandleEventErr(svc.HandleEventLock(func(context.Context, *entity.ExptItemEvalEvent) error {
		t.Fatal("a pending replacement cannot execute through another holder")
		return nil
	}))(context.Background(), event)
	require.True(t, errno.NeedMQRetry(err), "visible old metadata cannot ACK a delivery while its replacement is uncommitted")
	require.Less(t, time.Since(start), 3*time.Second)
}
