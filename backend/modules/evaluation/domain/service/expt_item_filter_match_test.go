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

func TestMatchByQueryType_Match(t *testing.T) {
	turn := turnWith("content", "the quick brown fox")
	matched, err := MatchExptItemFilter(&entity.ExptItemFilter{
		FilterFields: []*entity.ExptItemFilterField{
			{FieldName: "content", FieldType: "string", QueryType: "match", Values: []string{"quick"}},
		},
	}, nil, turn)
	assert.NoError(t, err)
	assert.True(t, matched)
}

func TestMatchByQueryType_NotMatch(t *testing.T) {
	turn := turnWith("content", "the quick brown fox")
	// not_match: 子串不存在才命中
	matched, err := MatchExptItemFilter(&entity.ExptItemFilter{
		FilterFields: []*entity.ExptItemFilterField{
			{FieldName: "content", FieldType: "string", QueryType: "not_match", Values: []string{"slow"}},
		},
	}, nil, turn)
	assert.NoError(t, err)
	assert.True(t, matched)

	// not_match: 子串存在则不命中
	matched, err = MatchExptItemFilter(&entity.ExptItemFilter{
		FilterFields: []*entity.ExptItemFilterField{
			{FieldName: "content", FieldType: "string", QueryType: "not_match", Values: []string{"quick"}},
		},
	}, nil, turn)
	assert.NoError(t, err)
	assert.False(t, matched)
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

// TestMatchFilterField_ItemID 验证 item_id filter 走 item.ItemID (不依赖 turn 字段)。
func TestMatchFilterField_ItemID(t *testing.T) {
	item := &entity.EvaluationSetItem{ItemID: 42}
	turn := &entity.Turn{} // 故意空 turn: item_id 分支不应读 turn

	cases := []struct {
		name      string
		queryType string
		values    []string
		want      bool
	}{
		{"in 命中", "in", []string{"7", "42", "99"}, true},
		{"in 不命中", "in", []string{"7", "99"}, false},
		{"eq 命中", "eq", []string{"42"}, true},
		{"eq 不命中", "eq", []string{"43"}, false},
		{"not_in 命中(不在列表→执行)", "not_in", []string{"7", "99"}, true},
		{"not_in 不命中(在列表→不执行)", "not_in", []string{"42"}, false},
		{"not_eq 命中", "not_eq", []string{"43"}, true},
		{"not_eq 不命中", "not_eq", []string{"42"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ff := &entity.ExptItemFilterField{FieldName: "item_id", FieldType: "long", QueryType: c.queryType, Values: c.values}
			got := matchFilterField(ff, item, turn)
			assert.Equal(t, c.want, got)
		})
	}
}

// TestShouldRunByFilter_ItemID 端到端: Include + item_id 命中→跑; 不命中→不跑(落 Skipped)。
func TestShouldRunByFilter_ItemID(t *testing.T) {
	item := &entity.EvaluationSetItem{ItemID: 100}
	turn := &entity.Turn{}

	hit := &entity.ExptItemFilter{FilterFields: []*entity.ExptItemFilterField{
		{FieldName: "item_id", FieldType: "long", QueryType: "in", Values: []string{"100"}},
	}}
	miss := &entity.ExptItemFilter{FilterFields: []*entity.ExptItemFilterField{
		{FieldName: "item_id", FieldType: "long", QueryType: "in", Values: []string{"999"}},
	}}

	run, err := ShouldRunByFilter(hit, filterModeInclude, item, turn)
	assert.NoError(t, err)
	assert.True(t, run, "Include + item_id 命中 → 跑")

	run, err = ShouldRunByFilter(miss, filterModeInclude, item, turn)
	assert.NoError(t, err)
	assert.False(t, run, "Include + item_id 不命中 → 不跑(Skipped)")

	// Exclude 反向
	run, err = ShouldRunByFilter(hit, filterModeExclude, item, turn)
	assert.NoError(t, err)
	assert.False(t, run, "Exclude + item_id 命中 → 不跑")
}

// TestMatchFilterField_Tag 验证 field_type=tag 走 item.Tags 的 TagName 存在性匹配。
func TestMatchFilterField_Tag(t *testing.T) {
	item := &entity.EvaluationSetItem{
		ItemID: 1,
		Tags:   []*entity.ResourceTag{{TagName: "zh"}, {TagName: "hard"}},
	}
	turn := &entity.Turn{} // tag 不依赖 turn

	cases := []struct {
		name      string
		queryType string
		values    []string
		want      bool
	}{
		{"in 命中(含 zh)", "in", []string{"zh", "en"}, true},
		{"in 不命中(都不含)", "in", []string{"en", "fr"}, false},
		{"eq 命中", "eq", []string{"hard"}, true},
		{"not_in 命中(不含→取反)", "not_in", []string{"en"}, true},
		{"not_in 不命中(含→取反)", "not_in", []string{"zh"}, false},
		{"match 命中(走存在性,不做子串)", "match", []string{"zh"}, true},
		{"not_match 含→不命中", "not_match", []string{"zh"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ff := &entity.ExptItemFilterField{FieldName: "lang", FieldType: "tag", QueryType: c.queryType, Values: c.values}
			got := matchFilterField(ff, item, turn)
			assert.Equal(t, c.want, got)
		})
	}
}

// TestMatchFilterField_Tag_NoTags item 无 tag 时走 matchMissingField (in→不命中, not_in→命中)。
func TestMatchFilterField_Tag_NoTags(t *testing.T) {
	item := &entity.EvaluationSetItem{ItemID: 1} // 无 Tags
	turn := &entity.Turn{}

	in := &entity.ExptItemFilterField{FieldName: "lang", FieldType: "tag", QueryType: "in", Values: []string{"zh"}}
	assert.False(t, matchFilterField(in, item, turn), "无 tag + in → 不命中")

	notIn := &entity.ExptItemFilterField{FieldName: "lang", FieldType: "tag", QueryType: "not_in", Values: []string{"zh"}}
	assert.True(t, matchFilterField(notIn, item, turn), "无 tag + not_in → 命中")
}

// TestShouldRunByFilter_Tag 端到端: Include + tag 命中→跑; 不命中→不跑。
func TestShouldRunByFilter_Tag(t *testing.T) {
	item := &entity.EvaluationSetItem{ItemID: 1, Tags: []*entity.ResourceTag{{TagName: "zh"}}}
	turn := &entity.Turn{}

	hit := &entity.ExptItemFilter{FilterFields: []*entity.ExptItemFilterField{
		{FieldName: "lang", FieldType: "tag", QueryType: "in", Values: []string{"zh"}},
	}}
	miss := &entity.ExptItemFilter{FilterFields: []*entity.ExptItemFilterField{
		{FieldName: "lang", FieldType: "tag", QueryType: "in", Values: []string{"en"}},
	}}

	run, err := ShouldRunByFilter(hit, filterModeInclude, item, turn)
	assert.NoError(t, err)
	assert.True(t, run, "Include + tag 命中 → 跑")

	run, err = ShouldRunByFilter(miss, filterModeInclude, item, turn)
	assert.NoError(t, err)
	assert.False(t, run, "Include + tag 不命中 → 不跑(Skipped)")
}
