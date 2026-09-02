// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// statsBucket 用生产映射函数 ItemRunStateStatsField 把 StatsCntArithOp 累加进五类计数,
// 从而让「设计 §7 定义的增减量是否守恒」这一断言真正跑过生产的状态→列映射(而非测试自己另写一套)。
type statsBucket struct {
	m map[string]int
}

func newStatsBucket(itemCnt int) *statsBucket {
	return &statsBucket{m: map[string]int{
		"pending_cnt":    itemCnt, // 初始全部 Queueing
		"processing_cnt": 0,
		"success_cnt":    0,
		"fail_cnt":       0,
		"terminated_cnt": 0,
	}}
}

func (b *statsBucket) apply(op *entity.StatsCntArithOp) {
	for state, delta := range op.OpStatusCnt {
		col := ItemRunStateStatsField(state)
		if col == "" {
			continue
		}
		b.m[col] += delta
	}
}

func (b *statsBucket) total() int {
	return b.m["pending_cnt"] + b.m["processing_cnt"] + b.m["success_cnt"] + b.m["fail_cnt"] + b.m["terminated_cnt"]
}

func opQtoP() *entity.StatsCntArithOp {
	return &entity.StatsCntArithOp{OpStatusCnt: map[entity.ItemRunState]int{entity.ItemRunState_Processing: 1, entity.ItemRunState_Queueing: -1}}
}

func opPtoQ() *entity.StatsCntArithOp { // ★ 让位
	return &entity.StatsCntArithOp{OpStatusCnt: map[entity.ItemRunState]int{entity.ItemRunState_Processing: -1, entity.ItemRunState_Queueing: 1}}
}

func opPtoS() *entity.StatsCntArithOp {
	return &entity.StatsCntArithOp{OpStatusCnt: map[entity.ItemRunState]int{entity.ItemRunState_Processing: -1, entity.ItemRunState_Success: 1}}
}

func opPtoF() *entity.StatsCntArithOp {
	return &entity.StatsCntArithOp{OpStatusCnt: map[entity.ItemRunState]int{entity.ItemRunState_Processing: -1, entity.ItemRunState_Fail: 1}}
}

func opPtoT() *entity.StatsCntArithOp {
	return &entity.StatsCntArithOp{OpStatusCnt: map[entity.ItemRunState]int{entity.ItemRunState_Processing: -1, entity.ItemRunState_Terminal: 1}}
}

func opQtoT() *entity.StatsCntArithOp { // 人工终止直接覆盖让位后的 Queueing 行
	return &entity.StatsCntArithOp{OpStatusCnt: map[entity.ItemRunState]int{entity.ItemRunState_Queueing: -1, entity.ItemRunState_Terminal: 1}}
}

// TestStatsConservation_Paths 覆盖 §6.5: design §7 的 7 条路径 × §7.1 四条守恒不变式。
// 每条路径以「初始 1 行 Queueing」为起点, 逐步 apply 该路径的 StatsCntArithOp 序列,
// 断言: (1) 五类之和恒 == item_cnt; (2) 到终态 processing==0 && pending==0;
// (3) success/fail 只被贡献一次(无论重试多少次)。
func TestStatsConservation_Paths(t *testing.T) {
	tests := []struct {
		name       string
		ops        []*entity.StatsCntArithOp
		wantEndCol string // 终态所在列
		terminal   bool   // 该路径是否到达终态(让位是中间态, 不适用终态不变式)
	}{
		{
			name:       "path1 normal success: Q->P->S",
			ops:        []*entity.StatsCntArithOp{opQtoP(), opPtoS()},
			wantEndCol: "success_cnt",
			terminal:   true,
		},
		{
			name:       "path2 retryable yield: Q->P->Q (yield), back to pending",
			ops:        []*entity.StatsCntArithOp{opQtoP(), opPtoQ()},
			wantEndCol: "pending_cnt",
			terminal:   false, // 让位是中间态, 行回到 Queueing 等待重新提交
		},
		{
			name:       "path3 retry-then-success: Q->P->Q->P->S",
			ops:        []*entity.StatsCntArithOp{opQtoP(), opPtoQ(), opQtoP(), opPtoS()},
			wantEndCol: "success_cnt",
			terminal:   true,
		},
		{
			name:       "path4 exhaust retries -> Fail: Q->P->(Q->P)x2->F",
			ops:        []*entity.StatsCntArithOp{opQtoP(), opPtoQ(), opQtoP(), opPtoQ(), opQtoP(), opPtoF()},
			wantEndCol: "fail_cnt",
			terminal:   true,
		},
		{
			name: "path5 zombie timeout -> Fail: Q->P->F (via RecordItemRunLogs statsCntOp)",
			ops:  []*entity.StatsCntArithOp{opQtoP(), opPtoF()},
			// zombie 场景不预写 item_result.status, 净效果仍是 P->F 一次
			wantEndCol: "fail_cnt",
			terminal:   true,
		},
		{
			name:       "path6 quota terminate: Q->P->T",
			ops:        []*entity.StatsCntArithOp{opQtoP(), opPtoT()},
			wantEndCol: "terminated_cnt",
			terminal:   true,
		},
		{
			name:       "path7 manual terminate covers yielded queueing row: Q->P->Q(yield)->T",
			ops:        []*entity.StatsCntArithOp{opQtoP(), opPtoQ(), opQtoT()},
			wantEndCol: "terminated_cnt",
			terminal:   true,
		},
	}

	const itemCnt = 1
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := newStatsBucket(itemCnt)
			// 不变式1: 每步之后五类之和恒 == item_cnt
			for i, op := range tt.ops {
				b.apply(op)
				assert.Equalf(t, itemCnt, b.total(), "invariant#1 broken after op %d", i)
				// 任意时刻各列非负(计数不应变负)
				for col, v := range b.m {
					assert.GreaterOrEqualf(t, v, 0, "col %s went negative after op %d", col, i)
				}
			}
			// 终态那一类恰好计 1(success/fail/terminated 只贡献一次, 不因重试多加)
			assert.Equal(t, itemCnt, b.m[tt.wantEndCol], "terminal bucket must be exactly item_cnt")
			if tt.terminal {
				// 不变式2: 终态 processing==0 && pending==0
				assert.Equal(t, 0, b.m["processing_cnt"], "invariant#2: processing must be zero at terminal")
				assert.Equal(t, 0, b.m["pending_cnt"], "invariant#2: pending must be zero at terminal")
			} else {
				// 让位后: processing 已释放归零, 行回到 pending(名额已让出)
				assert.Equal(t, 0, b.m["processing_cnt"], "yield must release the processing slot")
			}
		})
	}
}
