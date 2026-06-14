// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExptTurnRunResult_GetEvaluatorRecordByVerAlias_NilReceiver(t *testing.T) {
	var r *ExptTurnRunResult
	assert.Nil(t, r.GetEvaluatorRecordByVerAlias(100, "alias_A"))
}

func TestExptTurnRunResult_GetEvaluatorRecordByVerAlias_EmptyResults(t *testing.T) {
	r := &ExptTurnRunResult{EvaluatorResults: nil}
	assert.Nil(t, r.GetEvaluatorRecordByVerAlias(100, ""))
}

func TestExptTurnRunResult_GetEvaluatorRecordByVerAlias_MatchVerAndAlias(t *testing.T) {
	// 同 versionID 不同 alias 应被独立定位
	r := &ExptTurnRunResult{
		EvaluatorResults: []*EvaluatorRecord{
			{ID: 1, EvaluatorVersionID: 100, Alias: "alias_A", Status: EvaluatorRunStatusSuccess},
			{ID: 2, EvaluatorVersionID: 100, Alias: "alias_B", Status: EvaluatorRunStatusSkipped},
			{ID: 3, EvaluatorVersionID: 200, Alias: "", Status: EvaluatorRunStatusSuccess},
		},
	}

	got := r.GetEvaluatorRecordByVerAlias(100, "alias_A")
	assert.NotNil(t, got)
	assert.Equal(t, int64(1), got.ID)

	got = r.GetEvaluatorRecordByVerAlias(100, "alias_B")
	assert.NotNil(t, got)
	assert.Equal(t, int64(2), got.ID, "同 versionID 不同 alias 应独立定位")

	// 不同 versionID, 空 alias (老实验类型)
	got = r.GetEvaluatorRecordByVerAlias(200, "")
	assert.NotNil(t, got)
	assert.Equal(t, int64(3), got.ID)

	// 不存在的 (versionID, alias) 组合
	assert.Nil(t, r.GetEvaluatorRecordByVerAlias(100, "alias_C"))
	assert.Nil(t, r.GetEvaluatorRecordByVerAlias(999, "alias_A"))
}

func TestExptTurnRunResult_GetEvaluatorRecordByVerAlias_OldGetEvaluatorRecord_Compat(t *testing.T) {
	// 老路径 GetEvaluatorRecord(versionID) 仍能用, 但在 alias 多实例下只返回首条
	r := &ExptTurnRunResult{
		EvaluatorResults: []*EvaluatorRecord{
			{ID: 1, EvaluatorVersionID: 100, Alias: "alias_A"},
			{ID: 2, EvaluatorVersionID: 100, Alias: "alias_B"},
		},
	}
	got := r.GetEvaluatorRecord(100)
	assert.NotNil(t, got)
	assert.Equal(t, int64(1), got.ID, "老 GetEvaluatorRecord 在 alias 多实例下仅返回首条")
}
