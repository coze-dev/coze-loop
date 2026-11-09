// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package mysql

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// TestTerminalQuotaReservationStateMySQL 在真实 MySQL 上验证终止后额度投影对账的全部护栏。
//
// 为什么必须是真实 MySQL：这条 CAS 的保证来自「canonical 先锁、run log 后锁、WHERE 带旧值」
// 三件事的组合，sqlmock 只能验证 SQL 文本，验不了并发下的原子性与锁序。
func TestTerminalQuotaReservationStateMySQL(t *testing.T) {
	f := newManualRetryFixture(t)
	dao := NewExptItemResultDAO(f.provider)
	ctx := context.Background()

	setReservation := func(itemID int64, state int32) {
		t.Helper()
		require.NoError(t, f.conn.Exec("UPDATE expt_item_result_run_log SET quota_reservation_state=? WHERE space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?",
			state, f.space, f.expt, f.run, itemID).Error)
	}
	reservation := func(itemID int64) int32 {
		t.Helper()
		var got int32
		require.NoError(t, f.conn.Raw("SELECT quota_reservation_state FROM expt_item_result_run_log WHERE space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?",
			f.space, f.expt, f.run, itemID).Scan(&got).Error)
		return got
	}
	setRequest := func(itemID int64, requestID string) {
		t.Helper()
		raw, err := json.Marshal(map[string]string{})
		require.NoError(t, err)
		if requestID != "" {
			retry, err := json.Marshal(&entity.ManualRetryContext{RunID: f.run, RequestID: requestID, Mode: entity.EvaluationModeFailRetry})
			require.NoError(t, err)
			raw, err = json.Marshal(map[string]string{entity.ManualRetryExtKey: string(retry)})
			require.NoError(t, err)
		}
		require.NoError(t, f.conn.Exec("UPDATE expt_item_result SET ext=? WHERE space_id=? AND expt_id=? AND item_id=?",
			raw, f.space, f.expt, itemID).Error)
	}

	t.Run("clears_terminal_reserved_and_is_idempotent", func(t *testing.T) {
		setRequest(11, "")
		setReservation(11, 1)
		changed, err := dao.SetTerminalQuotaReservationState(ctx, f.space, f.expt, f.run, 11, "", entity.QuotaReservationStateNone)
		require.NoError(t, err)
		require.True(t, changed)
		require.Equal(t, int32(entity.QuotaReservationStateNone), reservation(11))

		again, err := dao.SetTerminalQuotaReservationState(ctx, f.space, f.expt, f.run, 11, "", entity.QuotaReservationStateNone)
		require.NoError(t, err)
		require.False(t, again, "second tick must not write again")
	})

	t.Run("marks_terminal_row_as_debt", func(t *testing.T) {
		setRequest(11, "")
		setReservation(11, 0)
		changed, err := dao.SetTerminalQuotaReservationState(ctx, f.space, f.expt, f.run, 11, "", entity.QuotaReservationStateReserved)
		require.NoError(t, err)
		require.True(t, changed)
		require.Equal(t, int32(entity.QuotaReservationStateReserved), reservation(11))
	})

	t.Run("request_identity_mismatch_is_noop", func(t *testing.T) {
		setRequest(11, "rq1")
		setReservation(11, 1)
		changed, err := dao.SetTerminalQuotaReservationState(ctx, f.space, f.expt, f.run, 11, "", entity.QuotaReservationStateNone)
		require.NoError(t, err)
		require.False(t, changed, "a stale observation must not touch a newer manual attempt")
		require.Equal(t, int32(entity.QuotaReservationStateReserved), reservation(11))

		changed, err = dao.SetTerminalQuotaReservationState(ctx, f.space, f.expt, f.run, 11, "rq1", entity.QuotaReservationStateNone)
		require.NoError(t, err)
		require.True(t, changed)
		require.Equal(t, int32(entity.QuotaReservationStateNone), reservation(11))
	})

	t.Run("non_terminal_rows_are_refused", func(t *testing.T) {
		for _, itemID := range []int64{12, 13} { // 12 = Processing, 13 = Queueing
			setRequest(itemID, "")
			setReservation(itemID, 1)
			changed, err := dao.SetTerminalQuotaReservationState(ctx, f.space, f.expt, f.run, itemID, "", entity.QuotaReservationStateNone)
			require.NoError(t, err)
			require.False(t, changed, "an in-flight or queued reservation must never be cleared by reconcile")
			require.Equal(t, int32(entity.QuotaReservationStateReserved), reservation(itemID))
		}
	})

	t.Run("foreign_run_is_refused", func(t *testing.T) {
		setRequest(11, "")
		setReservation(11, 1)
		changed, err := dao.SetTerminalQuotaReservationState(ctx, f.space, f.expt, f.run+1, 11, "", entity.QuotaReservationStateNone)
		require.NoError(t, err)
		require.False(t, changed)
		require.Equal(t, int32(entity.QuotaReservationStateReserved), reservation(11))
	})
}
