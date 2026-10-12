// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"encoding/json"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	openapiExperiment "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// QA 两项 (max_qa_turns / sua_qa_reply_template) 的**三态**透传守卫。
//
// 为什么单独钉三态: 这两项与前面的 SUA 四项有一个本质区别 —— 对 max_qa_turns 来说
// "未配置"和"显式配了 0/负数"是**两种不同处置**: 前者由 runtime 取默认值,
// 后者是用户配错、必须被 runtime 拒绝。任何一跳用 `GetMaxQaTurns() != 0` 或值类型 +
// omitempty 判定, 都会把显式 0 折回"未配置", 非法配置从此再也报不出来 —— 而且全程不报错,
// 只表现为"平台配了不生效"。
//
// 所以下面四跳全部按 IsSet/指针判定, 并对 0 与负数各走一遍。
func TestRunModeConfig_QAFieldsTriStateAcrossEveryConvertorHop(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		limit    *int32
		qaPE     *string
		wantKept bool
	}{
		{name: "unset_stays_unset", limit: nil, qaPE: nil, wantKept: false},
		{name: "explicit_zero_survives", limit: gptr.Of(int32(0)), qaPE: gptr.Of("qa-pe"), wantKept: true},
		{name: "negative_survives", limit: gptr.Of(int32(-3)), qaPE: gptr.Of("qa-pe"), wantKept: true},
		{name: "positive_survives", limit: gptr.Of(int32(2)), qaPE: gptr.Of("qa-pe"), wantKept: true},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// ① OpenAPI → domain DTO (外部调用方/fornax-cli 入口)
			dom, err := OpenAPIRunModeConfigDTO2Domain(&openapiExperiment.RunModeConfig{
				RunMode:            gptr.Of(openapiExperiment.ExptRunModeSuaMultiTurn),
				MaxQaTurns:         tc.limit,
				SuaQaReplyTemplate: tc.qaPE,
			})
			require.NoError(t, err)
			require.NotNil(t, dom)
			assert.Equal(t, tc.wantKept, dom.IsSetMaxQaTurns(), "OpenAPI→domain 丢了 max_qa_turns 的出现性")
			if tc.wantKept {
				assert.Equal(t, *tc.limit, dom.GetMaxQaTurns())
			}

			// ② domain DTO → entity (落库前最后一跳)
			do := runModeConfigDTO2DO(dom)
			require.NotNil(t, do)
			assert.Equal(t, tc.wantKept, do.MaxQATurns != nil, "domain→entity 丢了 max_qa_turns 的出现性")
			if tc.wantKept {
				assert.Equal(t, *tc.limit, *do.MaxQATurns)
			}
			assert.Equal(t, tc.wantKept, do.SuaQAReplyTemplate != "", "回答设定与限额的出现性必须各自独立")
			if tc.wantKept {
				assert.Equal(t, "qa-pe", do.SuaQAReplyTemplate)
			}
			// QA 两项不得影响原有 SUA 模板面。
			assert.Empty(t, do.SuaPETemplate)

			// ③ entity → domain DTO (回显/执行期下发 JSON 都走这条)
			back := runModeConfigDO2DTO(do)
			require.NotNil(t, back)
			assert.Equal(t, tc.wantKept, back.IsSetMaxQaTurns(), "entity→domain 回显丢了 max_qa_turns 的出现性")
			if tc.wantKept {
				assert.Equal(t, *tc.limit, back.GetMaxQaTurns())
			}

			// ④ domain DTO → OpenAPI (读接口回显)
			echo := RunModeConfigDomain2OpenAPI(back)
			require.NotNil(t, echo)
			assert.Equal(t, tc.wantKept, echo.IsSetMaxQaTurns(), "domain→OpenAPI 回显丢了 max_qa_turns 的出现性")
			if tc.wantKept {
				assert.Equal(t, *tc.limit, echo.GetMaxQaTurns())
			}
			assert.Equal(t, tc.wantKept, echo.SuaQaReplyTemplate != nil)

			// ⑤ 执行期那一跳是 **JSON 序列化** (expt_run_item_turn_impl 把 entity 塞进
			// builtin_run_mode_config), commercial 再反序列化。未配置时该 key 必须**不出现**,
			// 显式 0/负数必须**原样出现** —— 这是 runtime 区分三态的唯一依据。
			raw, err := json.Marshal(do)
			require.NoError(t, err)
			var wire map[string]any
			require.NoError(t, json.Unmarshal(raw, &wire))
			if tc.wantKept {
				assert.Contains(t, wire, "max_qa_turns", "显式配置的 max_qa_turns 必须出现在 Ext JSON 上")
				assert.Equal(t, float64(*tc.limit), wire["max_qa_turns"])
				assert.Equal(t, "qa-pe", wire["sua_qa_reply_template"])
			} else {
				assert.NotContains(t, wire, "max_qa_turns", "未配置的 max_qa_turns 不得出现在 Ext JSON 上")
				assert.NotContains(t, wire, "sua_qa_reply_template")
			}
		})
	}
}

// 老请求 (完全不带 QA 两项) 的行为不得改变: 所有跳都保持"未配置", 不产生新字段。
func TestRunModeConfig_QAFieldsLegacyRequestUnaffected(t *testing.T) {
	t.Parallel()

	dom, err := OpenAPIRunModeConfigDTO2Domain(&openapiExperiment.RunModeConfig{
		RunMode:    gptr.Of(openapiExperiment.ExptRunModeSuaMultiTurn),
		MaxTurns:   gptr.Of(int32(5)),
		SuaGoal:    gptr.Of("goal-x"),
		SuaPersona: gptr.Of("persona-x"),
	})
	require.NoError(t, err)

	do := runModeConfigDTO2DO(dom)
	require.NotNil(t, do)
	assert.Nil(t, do.MaxQATurns)
	assert.Empty(t, do.SuaQAReplyTemplate)
	assert.Equal(t, 5, do.MaxTurns)
	assert.Equal(t, "goal-x", do.SuaGoal)

	raw, err := json.Marshal(do)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "max_qa_turns")
	assert.NotContains(t, string(raw), "sua_qa_reply_template")
}

// nil 进 nil 出 (老实验没有 run_mode_config) 也必须继续成立。
func TestRunModeConfig_QAFieldsNilInNilOut(t *testing.T) {
	t.Parallel()

	assert.Nil(t, runModeConfigDTO2DO(nil))
	assert.Nil(t, runModeConfigDO2DTO(nil))
	assert.Nil(t, RunModeConfigDomain2OpenAPI(nil))
	back, err := OpenAPIRunModeConfigDTO2Domain(nil)
	require.NoError(t, err)
	assert.Nil(t, back)
}

// 题目级 (entity.ItemRunConf) 的同名字段同样是三态: nil ≠ 显式 0/负。
func TestItemRunConf_QAFieldsTriStateOnWire(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		limit *int32
		qaPE  string
		want  bool
	}{
		{name: "unset", limit: nil, want: false},
		{name: "explicit_zero", limit: gptr.Of(int32(0)), qaPE: "case-qa", want: true},
		{name: "negative", limit: gptr.Of(int32(-1)), qaPE: "case-qa", want: true},
		{name: "positive", limit: gptr.Of(int32(3)), qaPE: "case-qa", want: true},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			raw, err := json.Marshal(entity.ItemRunConf{MaxQATurns: tc.limit, SuaQAReplyTemplate: tc.qaPE})
			require.NoError(t, err)
			var wire map[string]any
			require.NoError(t, json.Unmarshal(raw, &wire))
			if tc.want {
				assert.Contains(t, wire, "max_qa_turns")
				assert.Equal(t, float64(*tc.limit), wire["max_qa_turns"])
				assert.Equal(t, "case-qa", wire["sua_qa_reply_template"])
			} else {
				assert.NotContains(t, wire, "max_qa_turns")
				assert.NotContains(t, wire, "sua_qa_reply_template")
			}
		})
	}
}
