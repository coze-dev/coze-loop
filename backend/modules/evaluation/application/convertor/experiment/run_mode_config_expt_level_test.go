// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	openapiExperiment "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// 实验级 SUA 行为四项 + max_turns 的透传守卫。
//
// 这五项此前**只有题目级一半**: entity.ItemRunConf 有、RunModeConfig 没有, 于是"实验粒度配
// SUA 行为 / 轮数"从 IDL 到 entity 全程不存在 —— 用户配了 100% 无效, 且 runtime 读实验级的
// 那段代码因恒为空而是死代码。本测试钉住补齐后的两条入口链路都真的把值带到了 entity。
//
// 消费端早已就绪 (runtime casefile.go 读 experiment_info.sua 四字段 + orchestration.go
// suaConfig 按「题目级优先、实验级兜底」逐字段合并), 故这条链路一通即生效。
// 合并规则**不在平台侧实现** —— 平台只如实下发两级值, 谁赢由 runtime 裁决。

// OpenAPI 入口 (fornax-cli / OpenAPI 调用方走这条)。
func TestOpenAPIRunModeConfigDTO2Domain_CarriesExptLevelSuaAndMaxTurns(t *testing.T) {
	t.Parallel()

	dom, err := OpenAPIRunModeConfigDTO2Domain(&openapiExperiment.RunModeConfig{
		RunMode:                  gptr.Of(openapiExperiment.ExptRunModeSuaMultiTurn),
		SuaMode:                  gptr.Of(openapiExperiment.SuaModeLoop),
		SuaGoal:                  gptr.Of("把 lone-\\r 的 bug 修对"),
		SuaPersona:               gptr.Of("务实的资深 Go 工程师"),
		SuaBehavioralConstraints: gptr.Of("每轮只追问一个点"),
		SuaPeTemplate:            gptr.Of("上轮评估: {{eval_result}}"),
		MaxTurns:                 gptr.Of(int32(12)),
	})
	require.NoError(t, err)
	require.NotNil(t, dom)

	assert.Equal(t, "把 lone-\\r 的 bug 修对", dom.GetSuaGoal())
	assert.Equal(t, "务实的资深 Go 工程师", dom.GetSuaPersona())
	assert.Equal(t, "每轮只追问一个点", dom.GetSuaBehavioralConstraints())
	assert.Equal(t, "上轮评估: {{eval_result}}", dom.GetSuaPeTemplate())
	assert.Equal(t, int32(12), dom.GetMaxTurns())
}

// domain DTO → entity (落库前最后一跳; 前端/内部 RPC 也汇到这里)。
func TestRunModeConfigDTO2DO_CarriesExptLevelSuaAndMaxTurns(t *testing.T) {
	t.Parallel()

	do := runModeConfigDTO2DO(&domain_expt.RunModeConfig{
		RunMode:                  gptr.Of(domain_expt.ExptRunMode_SuaMultiTurn),
		SuaMode:                  gptr.Of(domain_expt.SuaMode_Loop),
		SuaGoal:                  gptr.Of("goal-x"),
		SuaPersona:               gptr.Of("persona-x"),
		SuaBehavioralConstraints: gptr.Of("constraint-x"),
		SuaPeTemplate:            gptr.Of("tpl {{eval_result}}"),
		MaxTurns:                 gptr.Of(int32(12)),
	})
	require.NotNil(t, do)

	assert.Equal(t, "goal-x", do.SuaGoal)
	assert.Equal(t, "persona-x", do.SuaPersona)
	assert.Equal(t, "constraint-x", do.SuaBehavioralConstraints)
	assert.Equal(t, "tpl {{eval_result}}", do.SuaPETemplate)
	assert.Equal(t, 12, do.MaxTurns)
}

// 回显 (详情页读它才能显示"这个实验配了什么")。
// 未配的字段必须保持 nil —— 让"没配"与"配了空串"可区分, 也不给详情页塞空字段。
func TestRunModeConfigDO2DTO_EchoesSetFieldsOnly(t *testing.T) {
	t.Parallel()

	full := runModeConfigDO2DTO(&entity.RunModeConfig{
		RunMode:                  entity.RunModeSUAMultiTurn,
		SuaMode:                  entity.SuaModeLoop,
		SuaGoal:                  "goal-x",
		SuaPersona:               "persona-x",
		SuaBehavioralConstraints: "constraint-x",
		SuaPETemplate:            "tpl {{eval_result}}",
		MaxTurns:                 12,
	})
	require.NotNil(t, full)
	assert.Equal(t, "goal-x", full.GetSuaGoal())
	assert.Equal(t, "persona-x", full.GetSuaPersona())
	assert.Equal(t, "constraint-x", full.GetSuaBehavioralConstraints())
	assert.Equal(t, "tpl {{eval_result}}", full.GetSuaPeTemplate())
	assert.Equal(t, int32(12), full.GetMaxTurns())

	// 一个字段都没配的实验 (存量实验就是这样): 五项全部不回显。
	bare := runModeConfigDO2DTO(&entity.RunModeConfig{RunMode: entity.RunModeSingleTurn})
	require.NotNil(t, bare)
	assert.Nil(t, bare.SuaGoal)
	assert.Nil(t, bare.SuaPersona)
	assert.Nil(t, bare.SuaBehavioralConstraints)
	assert.Nil(t, bare.SuaPeTemplate)
	assert.Nil(t, bare.MaxTurns)
}

// 存量提交 (不带这五项) 行为不变: 全部零值, 由题目级或默认值接管。
// optional 字段的向前兼容性保证 —— 老客户端不发这些字段, 落库与改动前完全一致。
func TestRunModeConfigDTO2DO_LegacyRequestUnaffected(t *testing.T) {
	t.Parallel()

	do := runModeConfigDTO2DO(&domain_expt.RunModeConfig{
		RunMode: gptr.Of(domain_expt.ExptRunMode_FixedScriptMultiTurn),
	})
	require.NotNil(t, do)

	assert.Empty(t, do.SuaGoal)
	assert.Empty(t, do.SuaPersona)
	assert.Empty(t, do.SuaBehavioralConstraints)
	assert.Empty(t, do.SuaPETemplate)
	assert.Zero(t, do.MaxTurns)
	assert.Equal(t, entity.RunModeFixedScriptMultiTurn, do.RunMode)
}
