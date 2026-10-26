// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	dbmock "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

func TestDispatchMySQLRejectsDamagedObservation(t *testing.T) {
	for _, damage := range []string{"missing_canonical", "soft_canonical", "bad_run", "bad_outer_json", "bad_inner_json", "missing_run", "unknown_run_status"} {
		t.Run(damage, func(t *testing.T) {
			f := newManualRetryFixture(t)
			switch damage {
			case "missing_canonical":
				require.NoError(t, f.conn.Exec("DELETE FROM expt_item_result WHERE space_id=? AND expt_id=? AND item_id=13", f.space, f.expt).Error)
			case "soft_canonical":
				require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND item_id=13", f.space, f.expt).Delete(&model.ExptItemResult{}).Error)
			case "bad_run":
				require.NoError(t, f.conn.Exec("UPDATE expt_item_result SET expt_run_id=? WHERE space_id=? AND expt_id=? AND item_id=13", f.run+1, f.space, f.expt).Error)
			case "bad_outer_json", "bad_inner_json":
				raw := []byte(`{`)
				if damage == "bad_inner_json" {
					raw = []byte(`{"manual_retry":"null"}`)
				}
				require.NoError(t, f.conn.Exec("UPDATE expt_item_result SET ext=? WHERE space_id=? AND expt_id=? AND item_id=13", raw, f.space, f.expt).Error)
			case "unknown_run_status":
				require.NoError(t, f.conn.Exec("UPDATE expt_run_log SET status=99 WHERE space_id=? AND expt_id=?", f.space, f.expt).Error)
			case "missing_run":
				require.NoError(t, f.conn.Where("space_id=? AND expt_id=?", f.space, f.expt).Delete(&model.ExptRunLog{}).Error)
			}
			before := f.snapshot()
			dao := NewExptItemDispatchDAO(f.provider)
			runtime, err := dao.LoadDispatchRuntime(context.Background(), f.space, f.expt, f.run, 1)
			require.Error(t, err, "incomplete primary snapshot must not become usable runtime")
			require.Nil(t, runtime)
			observations, err := dao.MGetDispatchObservations(context.Background(), f.space, f.expt, f.run, []int64{13})
			require.Error(t, err)
			require.Nil(t, observations)
			require.Equal(t, before, f.snapshot())
		})
	}
}

func TestDispatchMySQLStartAtomic(t *testing.T) {
	f := newManualRetryFixture(t)
	_, err := f.port.RequeueTerminalItems(context.Background(), f.request(entity.EvaluationModeFailRetry))
	require.NoError(t, err)
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND expt_id=? AND item_id=11", f.space, f.expt).Update("quota_reservation_state", 1).Error)
	before := f.snapshot()
	started, err := NewExptItemDispatchDAO(f.provider).StartReservedItem(context.Background(), f.space, f.expt, f.run, 11, "rq1")
	require.NoError(t, err)
	require.True(t, started.Started)
	var item model.ExptItemResult
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND item_id=11", f.space, f.expt).Take(&item).Error)
	require.Equal(t, int32(entity.ItemRunState_Processing), item.Status, "Start must atomically advance canonical")
	var stats model.ExptStats
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=?", f.space, f.expt).Take(&stats).Error)
	require.Equal(t, int32(1), stats.PendingCnt)
	require.Equal(t, int32(2), stats.ProcessingCnt)
	var success model.ExptTurnResult
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND turn_id=22", f.space, f.expt).Take(&success).Error)
	require.Equal(t, int32(entity.TurnRunState_Success), success.Status)
	require.Equal(t, 0.0, gptr.Indirect(success.WeightedScore))
	require.Equal(t, before["expt_turn_evaluator_result_ref"], f.snapshot()["expt_turn_evaluator_result_ref"])
}

func TestDispatchMySQLCanonicalTerminalAbsorbing(t *testing.T) {
	f := newManualRetryFixture(t)
	require.NoError(t, f.conn.Exec("UPDATE expt_item_result SET status=3 WHERE space_id=? AND expt_id=? AND item_id=13", f.space, f.expt).Error)
	before := f.snapshot()
	claimed, err := NewExptItemDispatchDAO(f.provider).ClaimQuotaReserved(context.Background(), f.space, f.expt, f.run, []int64{13}, map[int64]string{13: ""})
	require.NoError(t, err)
	require.Empty(t, claimed, "canonical terminal must absorb stale queueing log")
	require.Equal(t, before, f.snapshot())
}

func obs(itemID int64, status entity.ItemRunState, quota entity.QuotaReservationState) *repo.ExptDispatchObservation {
	return &repo.ExptDispatchObservation{
		ItemID:                itemID,
		Status:                int32(status),
		QuotaReservationState: quota,
	}
}

func TestClassifyDispatchRuntime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		observations   []*repo.ExptDispatchObservation
		candidateLimit int
		wantOccupied   []int64
		wantCandidates []int64
	}{
		{
			name: "Processing 计入占用",
			observations: []*repo.ExptDispatchObservation{
				obs(1, entity.ItemRunState_Processing, entity.QuotaReservationStateNone),
				obs(2, entity.ItemRunState_Processing, entity.QuotaReservationStateNone),
			},
			candidateLimit: 10,
			wantOccupied:   []int64{1, 2},
			wantCandidates: nil,
		},
		{
			name: "Queueing/reserved 计入占用且不进候选（本次改动的核心）",
			observations: []*repo.ExptDispatchObservation{
				obs(1, entity.ItemRunState_Queueing, entity.QuotaReservationStateReserved),
			},
			candidateLimit: 10,
			wantOccupied:   []int64{1},
			wantCandidates: nil,
		},
		{
			name: "Queueing/none 是唯一候选",
			observations: []*repo.ExptDispatchObservation{
				obs(1, entity.ItemRunState_Queueing, entity.QuotaReservationStateNone),
				obs(2, entity.ItemRunState_Queueing, entity.QuotaReservationStateNone),
			},
			candidateLimit: 10,
			wantOccupied:   nil,
			wantCandidates: []int64{1, 2},
		},
		{
			name: "混合场景：占用 3 候选 2",
			observations: []*repo.ExptDispatchObservation{
				obs(1, entity.ItemRunState_Processing, entity.QuotaReservationStateNone),
				obs(2, entity.ItemRunState_Queueing, entity.QuotaReservationStateReserved),
				obs(3, entity.ItemRunState_Queueing, entity.QuotaReservationStateNone),
				obs(4, entity.ItemRunState_Processing, entity.QuotaReservationStateNone),
				obs(5, entity.ItemRunState_Queueing, entity.QuotaReservationStateNone),
			},
			candidateLimit: 10,
			wantOccupied:   []int64{1, 2, 4},
			wantCandidates: []int64{3, 5},
		},
		{
			name: "候选数受 limit 约束，占用不受限",
			observations: []*repo.ExptDispatchObservation{
				obs(1, entity.ItemRunState_Queueing, entity.QuotaReservationStateNone),
				obs(2, entity.ItemRunState_Queueing, entity.QuotaReservationStateNone),
				obs(3, entity.ItemRunState_Queueing, entity.QuotaReservationStateNone),
				obs(4, entity.ItemRunState_Processing, entity.QuotaReservationStateNone),
				obs(5, entity.ItemRunState_Processing, entity.QuotaReservationStateNone),
			},
			candidateLimit: 2,
			wantOccupied:   []int64{4, 5},
			wantCandidates: []int64{1, 2},
		},
		{
			name: "Processing 即使带 reserved 也只计一次占用（不重复）",
			observations: []*repo.ExptDispatchObservation{
				// 理论上 StartReservedItem 会清掉 reserved，此处防御脏数据
				obs(1, entity.ItemRunState_Processing, entity.QuotaReservationStateReserved),
			},
			candidateLimit: 10,
			wantOccupied:   []int64{1},
			wantCandidates: nil,
		},
		{
			name:           "空观测",
			observations:   nil,
			candidateLimit: 10,
			wantOccupied:   nil,
			wantCandidates: nil,
		},
		{
			name: "nil 元素跳过",
			observations: []*repo.ExptDispatchObservation{
				nil,
				obs(1, entity.ItemRunState_Queueing, entity.QuotaReservationStateNone),
			},
			candidateLimit: 10,
			wantOccupied:   nil,
			wantCandidates: []int64{1},
		},
		{
			name: "limit<=0 不截断候选",
			observations: []*repo.ExptDispatchObservation{
				obs(1, entity.ItemRunState_Queueing, entity.QuotaReservationStateNone),
				obs(2, entity.ItemRunState_Queueing, entity.QuotaReservationStateNone),
			},
			candidateLimit: 0,
			wantOccupied:   nil,
			wantCandidates: []int64{1, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := classifyDispatchRuntime(tt.observations, tt.candidateLimit)
			assert.Equal(t, tt.wantOccupied, got.OccupiedItemIDs)
			assert.Equal(t, tt.wantCandidates, got.CandidateItemIDs)
			assert.Equal(t, len(tt.wantOccupied), got.OccupiedCount())
		})
	}
}

func TestExptDispatchRuntime_OccupiedCount_Nil(t *testing.T) {
	t.Parallel()
	var r *repo.ExptDispatchRuntime
	assert.Equal(t, 0, r.OccupiedCount())
}

func TestChunkInt64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []int64
		size int
		want [][]int64
	}{
		{name: "整除", in: []int64{1, 2, 3, 4}, size: 2, want: [][]int64{{1, 2}, {3, 4}}},
		{name: "有余数", in: []int64{1, 2, 3}, size: 2, want: [][]int64{{1, 2}, {3}}},
		{name: "size 大于长度", in: []int64{1}, size: 10, want: [][]int64{{1}}},
		{name: "空输入", in: nil, size: 10, want: nil},
		{name: "size<=0 返回 nil 防死循环", in: []int64{1, 2}, size: 0, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, chunkInt64(tt.in, tt.size))
		})
	}
}

func TestQuotaReservationState_IsQuotaReserved(t *testing.T) {
	t.Parallel()
	assert.True(t, entity.QuotaReservationStateReserved.IsQuotaReserved())
	assert.False(t, entity.QuotaReservationStateNone.IsQuotaReserved())
	// 未知值按未预占处理：安全侧是让它进候选被重新预占，而非当成已占用而永不派发
	assert.False(t, entity.QuotaReservationState(99).IsQuotaReserved())
}

func callDispatchForSource(t *testing.T, dao IExptItemDispatchDAO, name string, f *manualRetryFixture, item int64, token string) (any, error) {
	t.Helper()
	switch name {
	case "ClaimQuotaReserved":
		return dao.ClaimQuotaReserved(context.Background(), f.space, f.expt, f.run, []int64{item}, map[int64]string{item: token})
	case "ResetQuotaReserved":
		return dao.ResetQuotaReserved(context.Background(), f.space, f.expt, f.run, []int64{item}, map[int64]string{item: token})
	case "StartReservedItem":
		return dao.StartReservedItem(context.Background(), f.space, f.expt, f.run, item, token)
	case "RequeueProcessingItem":
		return dao.RequeueProcessingItem(context.Background(), f.space, f.expt, f.run, item, token)
	default:
		t.Fatalf("unknown dispatch method %s", name)
		return nil, nil
	}
}

func TestDispatchMySQLOldSourceZeroWrites(t *testing.T) {
	for _, action := range []string{"ClaimQuotaReserved", "ResetQuotaReserved", "StartReservedItem", "RequeueProcessingItem"} {
		for _, token := range []string{"", "rq1"} {
			t.Run(action+"/old_"+token, func(t *testing.T) {
				f := newManualRetryFixture(t)
				req := f.request(entity.EvaluationModeFailRetry)
				req.RequestID = "rq2"
				req.PayloadHash = "hash2"
				_, err := f.port.RequeueTerminalItems(context.Background(), req)
				require.NoError(t, err)
				state, quota := 0, 0
				if action == "ResetQuotaReserved" || action == "StartReservedItem" {
					quota = 1
				}
				if action == "RequeueProcessingItem" {
					state = 1
				}
				require.NoError(t, f.conn.Exec("UPDATE expt_item_result SET status=? WHERE space_id=? AND expt_id=? AND item_id=11", state, f.space, f.expt).Error)
				require.NoError(t, f.conn.Exec("UPDATE expt_item_result_run_log SET status=?,quota_reservation_state=? WHERE space_id=? AND expt_id=? AND item_id=11", state, quota, f.space, f.expt).Error)
				before := f.snapshot()
				_, err = callDispatchForSource(t, NewExptItemDispatchDAO(f.provider), action, f, 11, token)
				require.NoError(t, err)
				require.Equal(t, before, f.snapshot(), "old token may not alter any status, count, reference or timestamp")
			})
		}
	}
}

func prepareDispatchMySQL(t *testing.T) (*manualRetryFixture, IExptItemDispatchDAO) {
	t.Helper()
	f := newManualRetryFixture(t)
	_, err := f.port.RequeueTerminalItems(context.Background(), f.request(entity.EvaluationModeFailRetry))
	require.NoError(t, err)
	return f, NewExptItemDispatchDAO(f.provider)
}

func TestDispatchMySQLMixedPayloadAndSingleJoin(t *testing.T) {
	f, dao := prepareDispatchMySQL(t)
	high := int64(9223372036854775806)
	require.NoError(t, f.conn.Exec("UPDATE expt_item_result SET item_id=? WHERE space_id=? AND expt_id=? AND item_id=13", high, f.space, f.expt).Error)
	require.NoError(t, f.conn.Exec("UPDATE expt_item_result_run_log SET item_id=? WHERE space_id=? AND expt_id=? AND item_id=13", high, f.space, f.expt).Error)
	raw, err := entity.MergeManualRetryContext(nil, &entity.ManualRetryContext{RunID: f.run, RequestID: "rq2", Mode: entity.EvaluationModeRetryItems})
	require.NoError(t, err)
	require.NoError(t, f.conn.Exec("UPDATE expt_item_result SET ext=? WHERE space_id=? AND expt_id=? AND item_id=?", raw, f.space, f.expt, high).Error)
	require.NoError(t, f.conn.Exec("UPDATE expt_run_log SET mode=? WHERE space_id=? AND expt_id=?", entity.EvaluationModeAppend, f.space, f.expt).Error)
	require.NoError(t, f.conn.Exec("UPDATE expt_item_result_run_log SET retry_times=7 WHERE space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, high).Error)
	queries := []string{}
	name := "dispatch_single_join"
	require.NoError(t, f.conn.Callback().Query().After("gorm:query").Register(name, func(tx *gorm.DB) { queries = append(queries, tx.Statement.SQL.String()) }))
	defer func() { require.NoError(t, f.conn.Callback().Query().Remove(name)) }()
	runtime, err := dao.LoadDispatchRuntime(context.Background(), f.space, f.expt, f.run, 1)
	require.NoError(t, err)
	require.Len(t, queries, 1)
	require.Contains(t, queries[0], "LEFT JOIN expt_item_result AS c")
	require.Contains(t, queries[0], "c.deleted_at IS NULL")
	require.Contains(t, queries[0], "l.deleted_at IS NULL")
	require.Equal(t, []int64{12}, runtime.OccupiedItemIDs)
	require.Equal(t, []int64{11}, runtime.CandidateItemIDs)
	require.Equal(t, map[int64]string{11: "rq1", 12: ""}, runtime.ItemRequestIDs)
	queries = nil
	observations, err := dao.MGetDispatchObservations(context.Background(), f.space, f.expt, f.run, []int64{11, 12, high})
	require.NoError(t, err)
	require.Len(t, queries, 1)
	require.Equal(t, "rq1", observations[0].RequestID)
	require.Equal(t, "", observations[1].RequestID)
	require.Equal(t, high, observations[2].ItemID)
	require.Equal(t, "rq2", observations[2].RequestID)
	require.NoError(t, f.conn.Exec("UPDATE expt_item_result_run_log SET quota_reservation_state=1 WHERE space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, high).Error)
	runtime, err = dao.LoadDispatchRuntime(context.Background(), f.space, f.expt, f.run, 1)
	require.NoError(t, err)
	require.Equal(t, []int64{12, high}, runtime.OccupiedItemIDs)
	require.Equal(t, map[int64]string{11: "rq1", 12: "", high: "rq2"}, runtime.ItemRequestIDs)
	require.NoError(t, f.conn.Exec("UPDATE expt_item_result_run_log SET quota_reservation_state=0 WHERE space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, high).Error)
	require.NoError(t, f.conn.Exec("UPDATE expt_item_result SET status=0 WHERE space_id=? AND expt_id=? AND item_id=12", f.space, f.expt).Error)
	require.NoError(t, f.conn.Exec("UPDATE expt_item_result_run_log SET status=0 WHERE space_id=? AND expt_id=? AND item_id=12", f.space, f.expt).Error)
	require.NoError(t, f.conn.Callback().Query().Remove(name))
	queries = nil
	expected := map[int64]string{11: "rq1", 12: "", high: "rq2"}
	claimed, err := dao.ClaimQuotaReserved(context.Background(), f.space, f.expt, f.run, []int64{12, 11, high, 11}, expected)
	require.NoError(t, err)
	require.Equal(t, []repo.ClaimedDispatchItem{{ItemID: 12, Mode: entity.EvaluationModeAppend, RetryTimes: 3}, {ItemID: 11, RequestID: "rq1", Mode: entity.EvaluationModeFailRetry, RetryTimes: 3}, {ItemID: high, RequestID: "rq2", Mode: entity.EvaluationModeRetryItems, RetryTimes: 7}}, claimed)
	before := f.snapshot()
	claimed, err = dao.ClaimQuotaReserved(context.Background(), f.space, f.expt, f.run, []int64{11, 12, high}, expected)
	require.NoError(t, err)
	require.Empty(t, claimed)
	require.Equal(t, before, f.snapshot())
	reset, err := dao.ResetQuotaReserved(context.Background(), f.space, f.expt, f.run, []int64{11, 12, high, 11}, expected)
	require.NoError(t, err)
	require.Equal(t, []int64{11, 12, high}, reset)
	t.Log("REAL_MYSQL_DISPATCH_MIXED_JOIN_PAYLOAD tokens=empty/RQ1/RQ2 modes=Append/FailRetry/RetryItems high_i64_preserved occupied_uncapped=true")
}

func TestDispatchMySQLRequiredMapAndPartialSuccess(t *testing.T) {
	f, dao := prepareDispatchMySQL(t)
	for _, action := range []string{"claim", "reset"} {
		before := f.snapshot()
		var err error
		if action == "claim" {
			_, err = dao.ClaimQuotaReserved(context.Background(), f.space, f.expt, f.run, []int64{11, 13}, map[int64]string{11: "rq1"})
		} else {
			_, err = dao.ResetQuotaReserved(context.Background(), f.space, f.expt, f.run, []int64{11, 13}, map[int64]string{11: "rq1"})
		}
		require.ErrorContains(t, err, "missing")
		require.Equal(t, before, f.snapshot())
	}
	require.NoError(t, f.conn.Exec("UPDATE expt_item_result SET ext=? WHERE space_id=? AND expt_id=? AND item_id=13", []byte(`{`), f.space, f.expt).Error)
	claimed, err := dao.ClaimQuotaReserved(context.Background(), f.space, f.expt, f.run, []int64{11, 13}, map[int64]string{11: "rq1", 13: ""})
	require.Error(t, err)
	require.Equal(t, []repo.ClaimedDispatchItem{{ItemID: 11, RequestID: "rq1", Mode: entity.EvaluationModeFailRetry, RetryTimes: 3}}, claimed)
	reset, err := dao.ResetQuotaReserved(context.Background(), f.space, f.expt, f.run, []int64{11, 13}, map[int64]string{11: "rq1", 13: ""})
	require.Error(t, err)
	require.Equal(t, []int64{11}, reset)
}

func TestDispatchMySQLStartCurrentCombinationsAndRequeue(t *testing.T) {
	for _, original := range []bool{false, true} {
		t.Run(fmt.Sprintf("original_%t", original), func(t *testing.T) {
			f, dao := prepareDispatchMySQL(t)
			item, token := int64(11), "rq1"
			if original {
				item, token = 13, ""
				require.NoError(t, f.conn.Exec("UPDATE expt_item_result SET item_version_id=0 WHERE space_id=? AND expt_id=? AND item_id=13", f.space, f.expt).Error)
				require.NoError(t, f.conn.Exec("UPDATE expt_item_result_run_log SET item_version_id=0 WHERE space_id=? AND expt_id=? AND item_id=13", f.space, f.expt).Error)
			}
			expected := map[int64]string{item: token}
			before := f.snapshot()
			current, err := dao.StartReservedItem(context.Background(), f.space, f.expt, f.run, item, token)
			require.NoError(t, err)
			require.Equal(t, repo.StartReservedItemResult{CurrentRequest: true}, current)
			require.Equal(t, before, f.snapshot())
			_, err = dao.ClaimQuotaReserved(context.Background(), f.space, f.expt, f.run, []int64{item}, expected)
			require.NoError(t, err)
			started, err := dao.StartReservedItem(context.Background(), f.space, f.expt, f.run, item, token)
			require.NoError(t, err)
			require.Equal(t, repo.StartReservedItemResult{CurrentRequest: true, Started: true}, started)
			before = f.snapshot()
			duplicate, err := dao.StartReservedItem(context.Background(), f.space, f.expt, f.run, item, token)
			require.NoError(t, err)
			require.Equal(t, repo.StartReservedItemResult{CurrentRequest: true}, duplicate)
			require.Equal(t, before, f.snapshot())
			timestamps := map[string][]time.Time{}
			for _, table := range []string{"expt_item_result", "expt_item_result_run_log", "expt_turn_result", "expt_turn_result_run_log"} {
				require.NoError(t, f.conn.Exec("UPDATE "+table+" SET updated_at='2020-01-02 03:04:05' WHERE space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, item).Error)
				var rows []time.Time
				require.NoError(t, f.conn.Table(table).Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, item).Order("id").Pluck("updated_at", &rows).Error)
				timestamps[table] = rows
			}
			before = f.snapshot()
			requeued, err := dao.RequeueProcessingItem(context.Background(), f.space, f.expt, f.run, item, token)
			require.NoError(t, err)
			require.True(t, requeued)
			for table, want := range timestamps {
				var got []time.Time
				require.NoError(t, f.conn.Table(table).Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, item).Order("id").Pluck("updated_at", &got).Error)
				require.Equal(t, want, got, table)
			}
			after := f.snapshot()
			require.Equal(t, before["expt_turn_evaluator_result_ref"], after["expt_turn_evaluator_result_ref"])
			require.Equal(t, before["eval_target_record"], after["eval_target_record"])
			require.Equal(t, before["evaluator_record"], after["evaluator_record"])
			requeued, err = dao.RequeueProcessingItem(context.Background(), f.space, f.expt, f.run, item, token)
			require.NoError(t, err)
			require.False(t, requeued)
			require.Equal(t, after, f.snapshot())
			t.Log("REAL_MYSQL_DISPATCH_START_REQUEUE duplicate_current=true second_stats=false all_zombie_timestamps_preserved=true original_version_zero_valid=true")
		})
	}
}

func TestDispatchMySQLTerminalAndControlZeroWrites(t *testing.T) {
	for _, kind := range []string{"canonical_success", "canonical_fail", "canonical_terminal", "log_success", "log_fail", "log_terminal", "run_finished", "latest_changed", "soft_canonical", "soft_log"} {
		t.Run(kind, func(t *testing.T) {
			f, dao := prepareDispatchMySQL(t)
			require.NoError(t, f.conn.Exec("UPDATE expt_item_result_run_log SET quota_reservation_state=1 WHERE space_id=? AND expt_id=? AND item_id=11", f.space, f.expt).Error)
			table, status := "expt_item_result", int32(entity.ItemRunState_Success)
			switch kind {
			case "canonical_fail":
				status = int32(entity.ItemRunState_Fail)
			case "canonical_terminal":
				status = int32(entity.ItemRunState_Terminal)
			case "log_success":
				table = "expt_item_result_run_log"
			case "log_fail":
				table = "expt_item_result_run_log"
				status = int32(entity.ItemRunState_Fail)
			case "log_terminal":
				table = "expt_item_result_run_log"
				status = int32(entity.ItemRunState_Terminal)
			case "run_finished":
				table = "expt_run_log"
				status = int32(entity.ExptStatus_Success)
			case "latest_changed":
				require.NoError(t, f.conn.Exec("UPDATE experiment SET latest_run_id=? WHERE space_id=? AND id=?", f.run+1, f.space, f.expt).Error)
			case "soft_canonical":
				require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND item_id=11", f.space, f.expt).Delete(&model.ExptItemResult{}).Error)
			case "soft_log":
				require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND item_id=11", f.space, f.expt).Delete(&model.ExptItemResultRunLog{}).Error)
			}
			if !strings.HasPrefix(kind, "soft_") && kind != "latest_changed" {
				query := "UPDATE " + table + " SET status=? WHERE space_id=? AND expt_id=?"
				if table != "expt_run_log" {
					query += " AND item_id=11"
				}
				require.NoError(t, f.conn.Exec(query, status, f.space, f.expt).Error)
			}
			before := f.snapshot()
			result, err := dao.StartReservedItem(context.Background(), f.space, f.expt, f.run, 11, "rq1")
			if kind == "soft_log" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, repo.StartReservedItemResult{}, result)
			require.Equal(t, before, f.snapshot())
		})
	}
}

func TestDispatchMySQLAdmissionLockRejectsOldOperations(t *testing.T) {
	for _, action := range []string{"ClaimQuotaReserved", "ResetQuotaReserved", "StartReservedItem", "RequeueProcessingItem"} {
		t.Run(action, func(t *testing.T) {
			f, dao := prepareDispatchMySQL(t)
			observed, err := dao.MGetDispatchObservations(context.Background(), f.space, f.expt, f.run, []int64{11})
			require.NoError(t, err)
			require.Equal(t, "rq1", observed[0].RequestID)
			// Complete RQ1 before preparing RQ2 using the actual admission port.
			require.NoError(t, f.conn.Exec("UPDATE expt_item_result SET status=3 WHERE space_id=? AND expt_id=? AND item_id=11", f.space, f.expt).Error)
			require.NoError(t, f.conn.Exec("UPDATE expt_item_result_run_log SET status=3,result_state=1 WHERE space_id=? AND expt_id=? AND item_id=11", f.space, f.expt).Error)
			require.NoError(t, f.conn.Exec("UPDATE expt_stats SET pending_cnt=pending_cnt-1,fail_cnt=fail_cnt+1 WHERE space_id=? AND expt_id=?", f.space, f.expt).Error)
			req := f.request(entity.EvaluationModeFailRetry)
			req.RequestID = "rq2"
			req.PayloadHash = "hash2"
			held, resume, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
			var admittedSnapshot map[string]string
			var once sync.Once
			release := func() { once.Do(func() { close(resume) }) }
			defer release()
			var canonicalLocked atomic.Bool
			name := "dispatch_admission_lock"
			require.NoError(t, f.conn.Callback().Query().After("gorm:query").Register(name, func(tx *gorm.DB) {
				if tx.Statement.Table == "expt_item_result" {
					if _, ok := tx.Statement.Clauses["FOR"]; ok {
						canonicalLocked.Store(true)
					}
				}
			}))
			require.NoError(t, f.conn.Callback().Update().Before("gorm:update").Register(name, func(tx *gorm.DB) {
				if tx.Statement.Table == "expt_item_result" && canonicalLocked.Load() {
					close(held)
					select {
					case <-resume:
					case <-tx.Statement.Context.Done():
						_ = tx.AddError(tx.Statement.Context.Err())
					}
				}
			}))
			require.NoError(t, f.conn.Callback().Update().After("gorm:update").Register(name+"_snapshot", func(tx *gorm.DB) {
				if tx.Statement.Table == "expt_run_log" {
					capture := *f
					capture.conn = tx.Session(&gorm.Session{NewDB: true})
					admittedSnapshot = capture.snapshot()
				}
			}))
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			go func() { var err error; defer func() { done <- err }(); _, err = f.port.RequeueTerminalItems(ctx, req) }()
			select {
			case <-held:
			case err := <-done:
				t.Fatalf("admission did not hold canonical: %v", err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			oldDone := make(chan error, 1)
			go func() { _, err := callDispatchForSource(t, dao, action, f, 11, observed[0].RequestID); oldDone <- err }()
			select {
			case err := <-oldDone:
				t.Fatalf("old operation bypassed admission canonical lock: %v", err)
			case <-time.After(150 * time.Millisecond):
			}
			release()
			require.NoError(t, <-done)
			require.NoError(t, <-oldDone)
			require.NoError(t, f.conn.Callback().Update().Remove(name))
			require.NoError(t, f.conn.Callback().Query().Remove(name))
			after := f.snapshot()
			require.NotNil(t, admittedSnapshot)
			require.Equal(t, admittedSnapshot, after, "old waiter must not mutate admission write set")
			require.NoError(t, f.conn.Callback().Update().Remove(name+"_snapshot"))
			current, err := dao.MGetDispatchObservations(context.Background(), f.space, f.expt, f.run, []int64{11})
			require.NoError(t, err)
			require.Equal(t, "rq2", current[0].RequestID)
			require.Equal(t, int32(entity.ItemRunState_Queueing), current[0].Status)
			require.Zero(t, current[0].QuotaReservationState)
			for _, old := range []string{"", "rq1"} {
				_, err := callDispatchForSource(t, dao, action, f, 11, old)
				require.NoError(t, err)
				require.Equal(t, after, f.snapshot())
			}
			t.Logf("REAL_MYSQL_DISPATCH_LOCK_BLOCKING action=%s observed=RQ1 accepted=RQ2 blocked_before_release=true old_source_zero=true", action)
		})
	}
}

// Wrap a real MySQL transaction; commit failure rolls the actual connection
// back before reporting failure. This is not a sqlmock transaction substitute.
type dispatchCommitFailurePool struct{ base *sql.DB }

func (p *dispatchCommitFailurePool) PrepareContext(ctx context.Context, q string) (*sql.Stmt, error) {
	return p.base.PrepareContext(ctx, q)
}

func (p *dispatchCommitFailurePool) ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error) {
	return p.base.ExecContext(ctx, q, args...)
}

func (p *dispatchCommitFailurePool) QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	return p.base.QueryContext(ctx, q, args...)
}

func (p *dispatchCommitFailurePool) QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row {
	return p.base.QueryRowContext(ctx, q, args...)
}

func (p *dispatchCommitFailurePool) BeginTx(ctx context.Context, options *sql.TxOptions) (gorm.ConnPool, error) {
	tx, err := p.base.BeginTx(ctx, options)
	if err != nil {
		return nil, err
	}
	return &dispatchCommitFailureTx{Tx: tx}, nil
}

type dispatchCommitFailureTx struct{ *sql.Tx }

func (t *dispatchCommitFailureTx) Commit() error {
	if err := t.Rollback(); err != nil {
		return err
	}
	return errors.New("injected MySQL commit failure")
}

type dispatchCommitFailureProvider struct{ base db.Provider }

func (p *dispatchCommitFailureProvider) NewSession(ctx context.Context, options ...db.Option) *gorm.DB {
	session := p.base.NewSession(ctx, options...)
	if pool, ok := session.Statement.ConnPool.(*sql.DB); ok {
		session.Statement.ConnPool = &dispatchCommitFailurePool{base: pool}
	}
	return session
}

func (p *dispatchCommitFailureProvider) Transaction(ctx context.Context, callback func(*gorm.DB) error, options ...db.Option) error {
	return p.NewSession(ctx, options...).Transaction(callback)
}

func TestDispatchMySQLTransitionRollback(t *testing.T) {
	for _, action := range []string{"StartReservedItem", "RequeueProcessingItem"} {
		for _, table := range []string{"expt_item_result", "expt_item_result_run_log", "expt_turn_result", "expt_turn_result_run_log", "expt_stats", "commit"} {
			t.Run(action+"/"+table, func(t *testing.T) {
				f, dao := prepareDispatchMySQL(t)
				_, err := dao.ClaimQuotaReserved(context.Background(), f.space, f.expt, f.run, []int64{11}, map[int64]string{11: "rq1"})
				require.NoError(t, err)
				if action == "RequeueProcessingItem" {
					result, err := dao.StartReservedItem(context.Background(), f.space, f.expt, f.run, 11, "rq1")
					require.NoError(t, err)
					require.True(t, result.Started)
				}
				before := f.snapshot()
				hits := 0
				if table == "commit" {
					dao = NewExptItemDispatchDAO(&dispatchCommitFailureProvider{base: f.provider})
				} else {
					name := "dispatch_failure_" + table
					require.NoError(t, f.conn.Callback().Update().Before("gorm:update").Register(name, func(tx *gorm.DB) {
						if tx.Statement.Table == table {
							_, same := tx.Statement.ConnPool.(*sql.Tx)
							require.True(t, same)
							hits++
							_ = tx.AddError(errors.New("injected MySQL write failure"))
						}
					}))
					defer func() { require.NoError(t, f.conn.Callback().Update().Remove(name)) }()
				}
				value, err := callDispatchForSource(t, dao, action, f, 11, "rq1")
				require.ErrorContains(t, err, "injected MySQL")
				if action == "StartReservedItem" {
					require.Equal(t, repo.StartReservedItemResult{}, value)
				} else {
					require.Equal(t, false, value)
				}
				if table != "commit" {
					require.Equal(t, 1, hits)
				}
				require.Equal(t, before, f.snapshot(), "rollback includes all states/counts/timestamps/refs/scores/receipt")
				t.Logf("REAL_MYSQL_DISPATCH_ROLLBACK action=%s point=%s snapshots=11", action, table)
			})
		}
	}
}

func TestDispatchMySQLPendingTurnDamageRollback(t *testing.T) {
	for _, damage := range []string{"missing_turn_log", "wrong_version", "bad_turn_json", "missing_stats", "unknown_turn"} {
		t.Run(damage, func(t *testing.T) {
			f, dao := prepareDispatchMySQL(t)
			_, err := dao.ClaimQuotaReserved(context.Background(), f.space, f.expt, f.run, []int64{11}, map[int64]string{11: "rq1"})
			require.NoError(t, err)
			switch damage {
			case "missing_turn_log":
				require.NoError(t, f.conn.Exec("DELETE FROM expt_turn_result_run_log WHERE space_id=? AND expt_id=? AND turn_id=21", f.space, f.expt).Error)
			case "wrong_version":
				require.NoError(t, f.conn.Exec("UPDATE expt_turn_result_run_log SET item_version_id=0 WHERE space_id=? AND expt_id=? AND turn_id=21", f.space, f.expt).Error)
			case "bad_turn_json":
				require.NoError(t, f.conn.Exec("UPDATE expt_turn_result_run_log SET evaluator_result_ids=? WHERE space_id=? AND expt_id=? AND turn_id=21", []byte(`{`), f.space, f.expt).Error)
			case "missing_stats":
				require.NoError(t, f.conn.Exec("DELETE FROM expt_stats WHERE space_id=? AND expt_id=?", f.space, f.expt).Error)
			case "unknown_turn":
				require.NoError(t, f.conn.Exec("UPDATE expt_turn_result SET status=9 WHERE space_id=? AND expt_id=? AND turn_id=21", f.space, f.expt).Error)
			}
			before := f.snapshot()
			result, err := dao.StartReservedItem(context.Background(), f.space, f.expt, f.run, 11, "rq1")
			require.Error(t, err)
			require.Equal(t, repo.StartReservedItemResult{}, result)
			require.Equal(t, before, f.snapshot())
		})
	}
}

func TestDispatchMySQLSuccessTurnPayloadPreserved(t *testing.T) {
	for _, score := range []*float64{nil, gptr.Of(0.0)} {
		name := "nil"
		if score != nil {
			name = "zero"
		}
		t.Run(name, func(t *testing.T) {
			f, dao := prepareDispatchMySQL(t)
			require.NoError(t, f.conn.Model(&model.ExptTurnResult{}).Where("space_id=? AND expt_id=? AND turn_id=22", f.space, f.expt).UpdateColumn("weighted_score", score).Error)
			snapshot := func() map[string]any {
				out := map[string]any{}
				for _, table := range []string{"expt_turn_result", "expt_turn_result_run_log"} {
					var row map[string]any
					require.NoError(t, f.conn.Table(table).Where("space_id=? AND expt_id=? AND turn_id=22", f.space, f.expt).Take(&row).Error)
					out[table] = row
				}
				var refs []model.ExptTurnEvaluatorResultRef
				require.NoError(t, f.conn.Where("space_id=? AND expt_id=?", f.space, f.expt).Order("id").Find(&refs).Error)
				out["refs"] = refs
				return out
			}
			before := snapshot()
			_, err := dao.ClaimQuotaReserved(context.Background(), f.space, f.expt, f.run, []int64{11}, map[int64]string{11: "rq1"})
			require.NoError(t, err)
			started, err := dao.StartReservedItem(context.Background(), f.space, f.expt, f.run, 11, "rq1")
			require.NoError(t, err)
			require.True(t, started.Started)
			require.Equal(t, before, snapshot())
			requeued, err := dao.RequeueProcessingItem(context.Background(), f.space, f.expt, f.run, 11, "rq1")
			require.NoError(t, err)
			require.True(t, requeued)
			require.Equal(t, before, snapshot())
		})
	}
}

func TestDispatchReadForcesPrimary(t *testing.T) {
	for _, kind := range []string{"runtime", "observations"} {
		t.Run(kind, func(t *testing.T) {
			conn, primary, replica := newRunLogResolverTestDB(t)
			provider := dbmock.NewMockProvider(gomock.NewController(t))
			provider.EXPECT().NewSession(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, opts ...db.Option) *gorm.DB {
				require.True(t, db.ContainWithMasterOpt(opts))
				return conn.Clauses(dbresolver.Write).WithContext(ctx)
			})
			primary.ExpectQuery("SELECT .*l.retry_times.*LEFT JOIN expt_item_result AS c.*c.deleted_at IS NULL.*l.deleted_at IS NULL").WillReturnRows(sqlmock.NewRows([]string{"item_id", "item_version_id", "canonical_item_version_id", "canonical_id", "canonical_run_id", "status", "canonical_status", "retry_times", "quota_reservation_state", "mode", "run_status", "ext"}).AddRow(int64(11), int64(0), int64(0), int64(21), int64(8), int32(0), int32(0), int32(3), int32(0), int32(entity.EvaluationModeSubmit), int64(entity.ExptStatus_Processing), []byte(`{}`)))
			dao := NewExptItemDispatchDAO(provider)
			if kind == "runtime" {
				result, err := dao.LoadDispatchRuntime(context.Background(), 9, 7, 8, 1)
				require.NoError(t, err)
				require.Equal(t, map[int64]string{11: ""}, result.ItemRequestIDs)
			} else {
				result, err := dao.MGetDispatchObservations(context.Background(), 9, 7, 8, []int64{11})
				require.NoError(t, err)
				require.Len(t, result, 1)
				require.Empty(t, result[0].RequestID)
			}
			require.NoError(t, primary.ExpectationsWereMet())
			require.NoError(t, replica.ExpectationsWereMet())
		})
	}
}

func TestDispatchMySQLOrphanTurnLogRollsBack(t *testing.T) {
	f := newManualRetryFixture(t)
	dao := NewExptItemDispatchDAO(f.provider)
	// Original execution with canonical frozen turns, but one log bound to an
	// unrelated turn. Equal row counts must not make this a complete binding.
	require.NoError(t, f.conn.Exec("UPDATE expt_item_result SET status=0 WHERE space_id=? AND expt_id=? AND item_id=11", f.space, f.expt).Error)
	require.NoError(t, f.conn.Exec("UPDATE expt_item_result_run_log SET status=0,result_state=0,quota_reservation_state=1 WHERE space_id=? AND expt_id=? AND item_id=11", f.space, f.expt).Error)
	require.NoError(t, f.conn.Exec("UPDATE expt_turn_result SET status=0 WHERE space_id=? AND expt_id=?", f.space, f.expt).Error)
	require.NoError(t, f.conn.Exec("UPDATE expt_turn_result_run_log SET status=0,turn_id=99 WHERE space_id=? AND expt_id=? AND turn_id=21", f.space, f.expt).Error)
	before := f.snapshot()
	result, err := dao.StartReservedItem(context.Background(), f.space, f.expt, f.run, 11, "")
	require.Error(t, err)
	require.Equal(t, repo.StartReservedItemResult{}, result)
	require.Equal(t, before, f.snapshot())
}

func TestDispatchMySQLCanonicalProcessingQueueingLog(t *testing.T) {
	f := newManualRetryFixture(t)
	dao := NewExptItemDispatchDAO(f.provider)
	require.NoError(t, f.conn.Exec("UPDATE expt_item_result_run_log SET status=0 WHERE space_id=? AND expt_id=? AND item_id=12", f.space, f.expt).Error)
	expected := map[int64]string{12: ""}
	before := f.snapshot()
	claimed, err := dao.ClaimQuotaReserved(context.Background(), f.space, f.expt, f.run, []int64{12}, expected)
	require.NoError(t, err)
	require.Len(t, claimed, 1, "run-log queueing remains candidate with a nonterminal canonical")
	reset, err := dao.ResetQuotaReserved(context.Background(), f.space, f.expt, f.run, []int64{12}, expected)
	require.NoError(t, err)
	require.Equal(t, []int64{12}, reset)
	_, err = dao.ClaimQuotaReserved(context.Background(), f.space, f.expt, f.run, []int64{12}, expected)
	require.NoError(t, err)
	started, err := dao.StartReservedItem(context.Background(), f.space, f.expt, f.run, 12, "")
	require.NoError(t, err)
	require.True(t, started.Started)
	after := f.snapshot()
	require.Equal(t, before["expt_stats"], after["expt_stats"], "canonical already Processing must not be counted twice")
	require.Equal(t, before["expt_item_result"], after["expt_item_result"])
}
