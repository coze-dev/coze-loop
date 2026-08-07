// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// realFixedQueryListJSON 是实验 7590116350194896130 (item_key 02040135, 16 轮营销文案题)
// 真实题目 run_conf 的形态精简版 —— **每轮只有 complex_query、没有 query**。
// 多模态 / 带附件的题目就是这么写的, 而这正是暴露 bug 的那个形态。
//
// 原始现场: logid 021786092636008fdbddc03001b0406305a1222270000ff2fe9be
const realFixedQueryListJSON = `{
  "max_turns": 16,
  "evaluator_trigger": "final_turn_only",
  "fixed_query_list": [
    {"complex_query": {"parts": [{"content_type": "text", "text": "你好，我是一位新农人主播，主要从事公司农资产品的宣传和销售。"}]}},
    {"complex_query": {"parts": [{"content_type": "text", "text": "我的宣传渠道是抖音和视频号，先帮我列出 5 个适合短视频科普的选题方向。"}]}},
    {"complex_query": {"parts": [
      {"content_type": "text", "text": "参考这份资料改写。"},
      {"content_type": "file", "file": {"file_name": "厄尔尼诺防灾.docx", "file_url": "https://tosv.byted.org/obj/fornax-static/a.docx"}}
    ]}}
  ]
}`

// fixed_query_list[].complex_query 必须能被解析且**原样序列化回 wire**。
//
// # 这条测试钉的是一次真实的静默空跑
//
// case-file 的 dataset_item.run_conf 是唯一走强类型 struct 的字段 (兄弟字段
// evaluator_conf / artifacts_conf / extra / 题目顶层 complex_query 都是 json.RawMessage
// 原样透传)。强类型的代价是**字段必须齐全**: Unmarshal→struct→Marshal 这一趟会把 struct
// 上不存在的字段直接蒸发。FixedQuery 原先只有 Query + Evaluators, 于是"每轮只配
// complex_query"的题目, 每个元素解出来都是空 struct、两个字段都 omitempty, wire 上就是 `{}`。
//
// 后果不是报错而是**空跑**: 数组长度对、has_run_conf=true、全程零 error, 而 orchestrator
// 每轮拿到的 query 是空 —— 实测 16 轮每轮 1.5 秒跑完 (orchestrator 日志 16 行
// `round=N, next query=, complex_parts=0`), 实验 success, 分数完全无意义。
//
// 所以这条测试**必须同时断言两个方向**: 解得出 (Unmarshal) 且送得出 (Marshal)。
// 只断言长度不够 —— 长度在 bug 存在时也是对的, 那正是它藏了这么久的原因。
func TestFixedQuery_ComplexQuerySurvivesRoundTrip(t *testing.T) {
	t.Parallel()

	var rc ItemRunConf
	require.NoError(t, json.Unmarshal([]byte(realFixedQueryListJSON), &rc))
	require.Len(t, rc.FixedQueryList, 3)

	// --- 解析方向: complex_query 必须落到 struct 上 ---
	for i, q := range rc.FixedQueryList {
		require.NotNilf(t, q.ComplexQuery, "第 %d 轮 complex_query 被吞了", i+1)
		require.NotEmptyf(t, q.ComplexQuery.Parts, "第 %d 轮 parts 为空", i+1)
	}
	assert.Equal(t, "text", rc.FixedQueryList[0].ComplexQuery.Parts[0].ContentType)
	assert.Contains(t, rc.FixedQueryList[0].ComplexQuery.Parts[0].Text, "新农人主播")

	// 附件那一轮: file 块的两个字段都要在 (少 file_url 沙箱就下载不到附件)。
	third := rc.FixedQueryList[2].ComplexQuery.Parts
	require.Len(t, third, 2, "text + file 两个块")
	require.NotNil(t, third[1].File)
	assert.Equal(t, "file", third[1].ContentType)
	assert.Equal(t, "厄尔尼诺防灾.docx", third[1].File.FileName)
	assert.Equal(t, "https://tosv.byted.org/obj/fornax-static/a.docx", third[1].File.FileURL)

	// --- 序列化方向: case-file 那一趟 Marshal 后 wire 上不能出现空对象 ---
	out, err := json.Marshal(&rc)
	require.NoError(t, err)
	var back struct {
		FixedQueryList []map[string]any `json:"fixed_query_list"`
	}
	require.NoError(t, json.Unmarshal(out, &back))
	require.Len(t, back.FixedQueryList, 3)
	for i, m := range back.FixedQueryList {
		assert.NotEmptyf(t, m, "第 %d 轮在 wire 上是空对象 {} —— 这就是那次 16 轮空跑的现象", i+1)
		assert.Containsf(t, m, "complex_query", "第 %d 轮 wire 上缺 complex_query", i+1)
	}
}

// query 与 complex_query 并存时两份都要如实透传, 平台**不做二选一** ——
// 优先级由 runtime 的 TurnQuery 裁决 (平台先合并就等于替它做了决定, 且它再也看不到另一份)。
// 与 ItemRunConf 两级配置"平台只如实下发、runtime 负责裁决"是同一条原则。
func TestFixedQuery_QueryAndComplexQueryCoexist(t *testing.T) {
	t.Parallel()

	const raw = `{"fixed_query_list":[{"query":"纯文本回落","complex_query":{"parts":[{"content_type":"text","text":"结构化"}]}}]}`
	var rc ItemRunConf
	require.NoError(t, json.Unmarshal([]byte(raw), &rc))
	require.Len(t, rc.FixedQueryList, 1)

	assert.Equal(t, "纯文本回落", rc.FixedQueryList[0].Query)
	require.NotNil(t, rc.FixedQueryList[0].ComplexQuery)
	assert.Equal(t, "结构化", rc.FixedQueryList[0].ComplexQuery.Parts[0].Text)

	out, err := json.Marshal(&rc)
	require.NoError(t, err)
	assert.Contains(t, string(out), `"query":"纯文本回落"`)
	assert.Contains(t, string(out), `"text":"结构化"`)
}

// 未知 content_type 必须无损 round-trip。
//
// ContentType 刻意是 string 而不是枚举: 平台在这条链上只做透传, 若把它建模成受限枚举,
// 新 schema 版本引入的类型就会在平台这一跳被丢掉 —— 又是一次静默失效 (与本文件修的
// complex_query 同一个病)。故这里钉住"不认识也照样送出去"。
func TestContentPart_UnknownContentTypeRoundTrips(t *testing.T) {
	t.Parallel()

	const raw = `{"fixed_query_list":[{"complex_query":{"parts":[{"content_type":"audio_stream_v2","text":"x"}]}}]}`
	var rc ItemRunConf
	require.NoError(t, json.Unmarshal([]byte(raw), &rc))
	assert.Equal(t, "audio_stream_v2", rc.FixedQueryList[0].ComplexQuery.Parts[0].ContentType)

	out, err := json.Marshal(&rc)
	require.NoError(t, err)
	assert.Contains(t, string(out), "audio_stream_v2", "未知类型不能在平台这一跳被吞掉")
}

// ComplexQuery.Extra 是业务私有透传通道 (如 CozeClaw 的 coze_skill_ids), 必须原样过去。
func TestComplexQuery_ExtraPassesThrough(t *testing.T) {
	t.Parallel()

	const raw = `{"fixed_query_list":[{"complex_query":{"parts":[{"content_type":"text","text":"t"}],"extra":{"coze_skill_ids":"[\"create-ppt\"]"}}}]}`
	var rc ItemRunConf
	require.NoError(t, json.Unmarshal([]byte(raw), &rc))
	assert.Equal(t, `["create-ppt"]`, rc.FixedQueryList[0].ComplexQuery.Extra["coze_skill_ids"])

	out, err := json.Marshal(&rc)
	require.NoError(t, err)
	assert.Contains(t, string(out), "coze_skill_ids")
}

// 空 complex_query 不出现在 wire 上 (omitempty), 且不 panic —— 没配就是没配,
// 送一个空对象会让 runtime 的 TurnQuery 分不清"没配"与"配了但空"。
func TestFixedQuery_AbsentComplexQueryOmitted(t *testing.T) {
	t.Parallel()

	rc := ItemRunConf{FixedQueryList: []*FixedQuery{{Query: "only-text"}}}
	out, err := json.Marshal(&rc)
	require.NoError(t, err)
	assert.NotContains(t, string(out), "complex_query")
	assert.Contains(t, string(out), "only-text")
}
