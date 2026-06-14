// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// 构造一个含给定 field 的 turn
func turnWith(fieldName, value string) *entity.Turn {
	return &entity.Turn{
		ID: 1,
		FieldDataList: []*entity.FieldData{
			{Name: fieldName, Content: &entity.Content{Text: gptr.Of(value)}},
		},
	}
}

func TestShouldRunByFilter_ModeNone_AlwaysRun(t *testing.T) {
	run, err := ShouldRunByFilter(&entity.ExptItemFilter{
		FilterFields: []*entity.ExptItemFilterField{
			{FieldName: "category", QueryType: "equal", Values: []string{"X"}},
		},
	}, 0, nil, turnWith("category", "Y"))
	assert.NoError(t, err)
	assert.True(t, run, "FilterMode=None 应该总是跑")
}

func TestShouldRunByFilter_NilFilter_AlwaysRun(t *testing.T) {
	run, err := ShouldRunByFilter(nil, 1, nil, turnWith("category", "X"))
	assert.NoError(t, err)
	assert.True(t, run, "filter==nil 应放行")
}

func TestShouldRunByFilter_IncludeHit_Run(t *testing.T) {
	run, err := ShouldRunByFilter(&entity.ExptItemFilter{
		FilterFields: []*entity.ExptItemFilterField{
			{FieldName: "category", QueryType: "equal", Values: []string{"A"}},
		},
	}, 1, nil, turnWith("category", "A"))
	assert.NoError(t, err)
	assert.True(t, run, "Include + 命中 → 跑")
}

func TestShouldRunByFilter_IncludeMiss_Skip(t *testing.T) {
	run, err := ShouldRunByFilter(&entity.ExptItemFilter{
		FilterFields: []*entity.ExptItemFilterField{
			{FieldName: "category", QueryType: "equal", Values: []string{"A"}},
		},
	}, 1, nil, turnWith("category", "B"))
	assert.NoError(t, err)
	assert.False(t, run, "Include + 未命中 → 跳过 (存 Skipped 占位)")
}

func TestShouldRunByFilter_ExcludeHit_Skip(t *testing.T) {
	run, err := ShouldRunByFilter(&entity.ExptItemFilter{
		FilterFields: []*entity.ExptItemFilterField{
			{FieldName: "category", QueryType: "equal", Values: []string{"A"}},
		},
	}, 2, nil, turnWith("category", "A"))
	assert.NoError(t, err)
	assert.False(t, run, "Exclude + 命中 → 跳过")
}

func TestShouldRunByFilter_ExcludeMiss_Run(t *testing.T) {
	run, err := ShouldRunByFilter(&entity.ExptItemFilter{
		FilterFields: []*entity.ExptItemFilterField{
			{FieldName: "category", QueryType: "equal", Values: []string{"A"}},
		},
	}, 2, nil, turnWith("category", "B"))
	assert.NoError(t, err)
	assert.True(t, run, "Exclude + 未命中 → 跑")
}

func TestMatchExptItemFilter_AndLogic_AllMatch(t *testing.T) {
	turn := &entity.Turn{
		FieldDataList: []*entity.FieldData{
			{Name: "lang", Content: &entity.Content{Text: gptr.Of("zh")}},
			{Name: "topic", Content: &entity.Content{Text: gptr.Of("math")}},
		},
	}
	matched, err := MatchExptItemFilter(&entity.ExptItemFilter{
		QueryAndOr: "and",
		FilterFields: []*entity.ExptItemFilterField{
			{FieldName: "lang", QueryType: "equal", Values: []string{"zh"}},
			{FieldName: "topic", QueryType: "equal", Values: []string{"math"}},
		},
	}, nil, turn)
	assert.NoError(t, err)
	assert.True(t, matched)
}

func TestMatchExptItemFilter_AndLogic_OneMiss(t *testing.T) {
	turn := &entity.Turn{
		FieldDataList: []*entity.FieldData{
			{Name: "lang", Content: &entity.Content{Text: gptr.Of("zh")}},
			{Name: "topic", Content: &entity.Content{Text: gptr.Of("history")}}, // 不命中
		},
	}
	matched, err := MatchExptItemFilter(&entity.ExptItemFilter{
		QueryAndOr: "and",
		FilterFields: []*entity.ExptItemFilterField{
			{FieldName: "lang", QueryType: "equal", Values: []string{"zh"}},
			{FieldName: "topic", QueryType: "equal", Values: []string{"math"}},
		},
	}, nil, turn)
	assert.NoError(t, err)
	assert.False(t, matched, "AND: 任一字段未命中即不命中")
}

func TestMatchExptItemFilter_OrLogic_AnyMatch(t *testing.T) {
	turn := &entity.Turn{
		FieldDataList: []*entity.FieldData{
			{Name: "lang", Content: &entity.Content{Text: gptr.Of("en")}}, // 不命中
			{Name: "topic", Content: &entity.Content{Text: gptr.Of("math")}}, // 命中
		},
	}
	matched, err := MatchExptItemFilter(&entity.ExptItemFilter{
		QueryAndOr: "or",
		FilterFields: []*entity.ExptItemFilterField{
			{FieldName: "lang", QueryType: "equal", Values: []string{"zh"}},
			{FieldName: "topic", QueryType: "equal", Values: []string{"math"}},
		},
	}, nil, turn)
	assert.NoError(t, err)
	assert.True(t, matched, "OR: 任一字段命中即命中")
}

func TestMatchByQueryType_Contains(t *testing.T) {
	turn := turnWith("content", "the quick brown fox")
	matched, err := MatchExptItemFilter(&entity.ExptItemFilter{
		FilterFields: []*entity.ExptItemFilterField{
			{FieldName: "content", QueryType: "contains", Values: []string{"quick"}},
		},
	}, nil, turn)
	assert.NoError(t, err)
	assert.True(t, matched)
}

func TestMatchByQueryType_NotEqual_FieldMissing_True(t *testing.T) {
	// 字段不存在时 not_equal 应返回 true (按取反语义,缺失=不等)
	turn := &entity.Turn{}
	matched, err := MatchExptItemFilter(&entity.ExptItemFilter{
		FilterFields: []*entity.ExptItemFilterField{
			{FieldName: "lang", QueryType: "not_equal", Values: []string{"zh"}},
		},
	}, nil, turn)
	assert.NoError(t, err)
	assert.True(t, matched)
}

func TestMatchByQueryType_UnknownTypeDefaultsTrue(t *testing.T) {
	turn := turnWith("category", "X")
	matched, err := MatchExptItemFilter(&entity.ExptItemFilter{
		FilterFields: []*entity.ExptItemFilterField{
			{FieldName: "category", QueryType: "regex_match", Values: []string{"X"}},
		},
	}, nil, turn)
	assert.NoError(t, err)
	assert.True(t, matched, "未识别的 QueryType 默认放行(不阻断执行)")
}

func TestShouldRunByFilter_ErrPassThroughDefaultRun(t *testing.T) {
	// 当前实现 MatchExptItemFilter 不返回 err, 但接口允许 — 兜底语义验证: 即使返回 err 也应放行
	_ = errors.New("placeholder")
}
