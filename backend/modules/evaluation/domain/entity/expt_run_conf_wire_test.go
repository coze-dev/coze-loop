// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ItemRunConf / RunModeConfig 是**跨仓 wire 契约**: 平台把它们 json.Marshal 进
// ext["builtin_run_conf"] / ext["builtin_run_mode_config"], 评测运行时按 **json tag** 反序列化。
// 跨仓对接看的是 tag 不是 Go 字段名, 所以改 tag(或删字段) 在本仓编译通过、在对端静默丢值 ——
// 本文件的测试全部围绕这一类"改了不报错、只是配置到不了"的回归。

// EvaluatorTrigger 是**题目级评估时机**的生产端, 也是这条链上最容易静默回落的字段。
//
// 它此前整条链路都缺: runtime 的 platform.runModeConfig 把 EvaluatorTrigger 硬编码成
// TriggerByTestCase, 于是另外三个值(never / after_each_turn / final_turn_only)配了 100% 无效 ——
// 枚举和 ShouldEvaluate 的 switch 分支都在, 唯独读 wire 的那一行不存在。runtime 已补读,
// 本字段是它的生产端: 只要这里的 tag 漂了或字段没了, 现象**不是报错而是回落 by_testcase_config**
// (即"每轮都评"或"按题目配置"), 6 轮重构题会被评 6 次、评分含义整个变掉, 而实验照样 success。
//
// 所以这里必须断言两个方向: 解得出(runtime 写来的题目配置能进 struct) 且送得出(Marshal 后 wire 上还在)。
func TestItemRunConf_EvaluatorTriggerSurvivesRoundTrip(t *testing.T) {
	t.Parallel()

	// 四个值都要能原样过 —— 平台不解释语义, 只如实透传 (语义在 runtime 的 ShouldEvaluate)。
	// 未知值同样透传: 平台在这条链上是搬运工, 因为不认识就丢掉会重演一次静默失效。
	for _, trigger := range []string{"never", "after_each_turn", "final_turn_only", "by_testcase_config", "future_new_trigger"} {
		trigger := trigger
		t.Run(trigger, func(t *testing.T) {
			t.Parallel()

			raw := `{"evaluator_trigger":"` + trigger + `"}`
			var rc ItemRunConf
			require.NoError(t, json.Unmarshal([]byte(raw), &rc))
			assert.Equal(t, trigger, rc.EvaluatorTrigger, "evaluator_trigger 没解进 struct —— tag 漂了?")

			out, err := json.Marshal(&rc)
			require.NoError(t, err)
			assert.JSONEq(t, raw, string(out), "wire 上必须原样送出, 少了就回落 by_testcase_config")
		})
	}

	// 没配时不能在 wire 上出现 —— 空串会让 runtime 分不清"没配"和"配了空", 而 omitempty
	// 缺省语义(空 = 未配 = 回落 by_testcase_config)是既有约定。
	out, err := json.Marshal(&ItemRunConf{MaxTurns: 3})
	require.NoError(t, err)
	assert.NotContains(t, string(out), "evaluator_trigger")
}

// 空 ItemRunConf 必须序列化成 "{}"。
//
// 这条断言看着像"复述实现", 其实是**另一处 bug 的前提锁定**: 调度侧
// itemRunConfFromRunModeConfig 一个字段都没搬到时必须返回 nil 而**不是空结构体** ——
// 因为空结构体全字段 omitempty ⇒ Marshal 成 "{}"; 执行侧闸门是 `RunConf != nil`, 空结构体能过闸,
// ext["builtin_run_conf"] 就被写成 "{}"; 而消费侧判"未配"用的是 `raw == ""`, "{}" 非空 ⇒
// 「Ext 为空则回退读题目自带 run_conf」这条回退**永远走不到**, 评测集题目里配的
// sua_goal / sua_persona / fixed_query_list 一个都到不了 runtime, 且实验照样 success。
//
// 若哪天给 ItemRunConf 加了**不带 omitempty** 的字段, "{}" 这个前提就不成立了, 调度侧那个
// "返回 nil" 的修法也随之失去意义 —— 本测试就是那道提醒。
func TestItemRunConf_EmptyMarshalsToEmptyObject(t *testing.T) {
	t.Parallel()

	out, err := json.Marshal(&ItemRunConf{})
	require.NoError(t, err)
	assert.Equal(t, "{}", string(out),
		`空 ItemRunConf 不再是 "{}" 说明有字段丢了 omitempty; 调度侧"无字段可搬则返回 nil"的修法依赖这个前提`)
}

// 钉住 ItemRunConf / FixedQuery / ComplexQuery / ContentPart / FileRef 的**完整 tag 集合**。
//
// 为什么要逐个列: 这条链上平台是生产端、runtime 是消费端, 双方靠 tag 对齐。改一个 tag
// (甚至只是删掉某个字段) 本仓编译照过、单测照过, 对端只是**少收到一项配置**并按缺省跑 ——
// 这正是 complex_query 缺失导致 16 轮空跑那次事故的形状。所以这里用"全集相等"而不是"包含":
// 少字段要 FAIL, 多字段也要 FAIL(提醒同步 runtime 的 testcase.RunConf, 否则新字段单向存在)。
func TestItemRunConf_WireTagsAreTheCrossRepoContract(t *testing.T) {
	t.Parallel()

	rc := &ItemRunConf{
		MaxTurns:                 4,
		MaxRunMinutes:            30,
		SuaGoal:                  "g",
		SuaPersona:               "p",
		SuaBehavioralConstraints: "c",
		SuaPETemplate:            "tpl {{eval_result}}",
		EvaluatorTrigger:         "final_turn_only",
		FixedQueryList: []*FixedQuery{{
			Query:      "q",
			Evaluators: map[string]interface{}{"acc": 1},
			ComplexQuery: &ComplexQuery{
				Extra: map[string]string{"k": "v"},
				Parts: []*ContentPart{
					{ContentType: "text", Text: "t"},
					{ContentType: "file", File: &FileRef{FileName: "a.docx", FileURL: "https://example.com/a.docx"}},
				},
			},
		}},
	}

	assert.ElementsMatch(t,
		[]string{
			"max_turns", "max_run_minutes", "fixed_query_list", "sua_goal", "sua_persona",
			"sua_behavioral_constraints", "sua_pe_template", "evaluator_trigger",
		},
		wireKeys(t, rc),
		"ItemRunConf 的 wire tag 集合变了 —— 必须同步 runtime 的 testcase.RunConf, 否则单向丢字段")

	list, ok := wireMap(t, rc)["fixed_query_list"].([]any)
	require.True(t, ok)
	require.Len(t, list, 1)
	round, ok := list[0].(map[string]any)
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"query", "complex_query", "evaluators"}, keysOf(round),
		"FixedQuery 的 wire tag 集合变了 —— complex_query 缺失曾让 16 轮题目静默空跑")

	cq, ok := round["complex_query"].(map[string]any)
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"parts", "extra"}, keysOf(cq))

	parts, ok := cq["parts"].([]any)
	require.True(t, ok)
	require.Len(t, parts, 2)
	textPart, ok := parts[0].(map[string]any)
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"content_type", "text"}, keysOf(textPart),
		"text 块不该带 file 键 (omitempty), 否则对端会看到一个空附件")
	filePart, ok := parts[1].(map[string]any)
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"content_type", "file"}, keysOf(filePart))
	fileRef, ok := filePart["file"].(map[string]any)
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"file_name", "file_url"}, keysOf(fileRef),
		"少 file_url 沙箱就下载不到附件, 而题目照样跑完")
}

// 钉住 RunModeConfig 落 eval_conf / 进 ext["builtin_run_mode_config"] 的 tag 全集。
//
// 两个消费方向都靠这些 tag: ① 落库 experiment.eval_conf 后回读(存量实验的 run_mode 就是从这里读);
// ② 执行期透传给算子填 case-file experiment_info。改 tag 会让**存量实验回读不到自己的跑法**
// (回落 single_turn) 且新实验的 SUA 行为四项到不了 sua-cli(它缺 persona/pe_template 会报
// INVALID_CONFIG, 但缺 goal 只是行为变化, 依然静默)。
func TestRunModeConfig_WireTagsAreTheCrossRepoContract(t *testing.T) {
	t.Parallel()

	cfg := &RunModeConfig{
		RunMode:                  RunModeSUAMultiTurn,
		MaxRunMinutes:            30,
		SuaMode:                  SuaModeLoop,
		SuaModelID:               123,
		SuaModelName:             "m",
		SuaGoal:                  "g",
		SuaPersona:               "p",
		SuaBehavioralConstraints: "c",
		SuaPETemplate:            "tpl {{eval_result}}",
		MaxTurns:                 8,
	}
	assert.ElementsMatch(t,
		[]string{
			"run_mode", "max_run_minutes", "sua_mode", "sua_model_id", "sua_model_name",
			"sua_goal", "sua_persona", "sua_behavioral_constraints", "sua_pe_template", "max_turns",
		},
		wireKeys(t, cfg),
		"RunModeConfig 的 wire tag 集合变了 —— 存量实验会回读不到自己的跑法而回落 single_turn")

	// 未配的实验级配置不该在 wire 上留空字段: 空 = 未配, 由题目级或默认值接管。
	out, err := json.Marshal(&RunModeConfig{RunMode: RunModeSingleTurn})
	require.NoError(t, err)
	assert.JSONEq(t, `{"run_mode":"single_turn"}`, string(out))
}

// RunMode 字面量是跨仓契约的另一半: run_mode 以字符串落库 / 下发, 改字面量等于改协议。
// 六个常量分两类 —— 前三个是**平台对外契约**(IDL / 前端 / 存量实验都用它), 后两个是
// **runtime 独有的折叠态**(平台只在下发 case-file 前用), goal 两边同名。
// 字面量与 runtime 的 orchestration RunMode 逐字一致, 漂了就是"跑法名对不上 → 回落"。
func TestRunModeLiterals(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "single_turn", RunModeSingleTurn)
	assert.Equal(t, "fixed_script_multi_turn", RunModeFixedScriptMultiTurn)
	assert.Equal(t, "sua_multi_turn", RunModeSUAMultiTurn)
	assert.Equal(t, "sua_loop_multi_turn", RunModeSUALoopMultiTurn)
	assert.Equal(t, "sua_human_loop_multi_turn", RunModeSUAHumanLoopMultiTurn)
	assert.Equal(t, "goal", RunModeGoal)

	// 六个字面量必须互不相同 —— 撞字面量会让两个跑法在 wire 上不可区分。
	seen := map[RunMode]bool{}
	for _, m := range []RunMode{
		RunModeSingleTurn, RunModeFixedScriptMultiTurn, RunModeSUAMultiTurn,
		RunModeSUALoopMultiTurn, RunModeSUAHumanLoopMultiTurn, RunModeGoal,
	} {
		assert.False(t, seen[m], "run_mode 字面量 %q 重复", m)
		seen[m] = true
	}
}

func wireMap(t *testing.T, v any) map[string]any {
	t.Helper()
	out, err := json.Marshal(v)
	require.NoError(t, err)
	m := map[string]any{}
	require.NoError(t, json.Unmarshal(out, &m))
	return m
}

func wireKeys(t *testing.T, v any) []string {
	t.Helper()
	return keysOf(wireMap(t, v))
}

func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
