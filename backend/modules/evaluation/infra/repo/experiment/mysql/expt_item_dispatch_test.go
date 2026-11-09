// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

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
