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

// sua_mode 从 OpenAPI 请求到落库 entity 的这条链路上有**两次**枚举转换:
//
//	OpenAPI string ("human_loop")  --openAPISuaModeToDomain-->  domain int enum (SuaMode_HumanLoop=1)
//	domain int enum                --suaModeDTO2DO-->            entity string (SuaModeHumanLoop)
//
// 字符串→int→字符串的这个**往返**就是此前的隐式归一点: 入口收 IDL 的 "human_loop", 出口写
// entity 常量, 两者字面量不一致时差异被静默吃掉, 落库 eval_conf.run_mode_config.sua_mode
// 变成 "humanloop" (实测实验 7590112179985613570)。没有任何一行"归一代码", 全靠两端不同词。
//
// 本测试钉住往返的**闭合性**: 进去什么词, 出来必须还是同一个词。它是全链路一致性的护栏 ——
// 只要 entity 常量与 IDL 常量再次漂开, 这里立刻 FAIL, 而不是等落库后靠人查 DB 才发现。
//
// 反向验证 (证明本测试真的在守): 把 entity.SuaModeHumanLoop 改回 "humanloop", 本测试
// human_loop 那条 FAIL —— 送进 "human_loop", 往返后得到 "humanloop"。已还原。
func TestSuaMode_OpenAPIToEntityRoundTripIsClosed(t *testing.T) {
	t.Parallel()

	// 三个合法值逐个走完整往返。左边是 IDL 常量 (用户书写的形态), 右边是落库的 entity 常量。
	cases := []struct {
		openAPI openapiExperiment.SuaMode
		want    entity.SuaMode
	}{
		{openapiExperiment.SuaModeHumanLoop, entity.SuaModeHumanLoop},
		{openapiExperiment.SuaModeLoop, entity.SuaModeLoop},
		{openapiExperiment.SuaModeFixed, entity.SuaModeFixed},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.openAPI), func(t *testing.T) {
			t.Parallel()

			// 第一跳: OpenAPI DTO → domain int 枚举 (创建接口实际调的就是这个函数)。
			dom := OpenAPIRunModeConfigDTO2Domain(&openapiExperiment.RunModeConfig{
				RunMode: gptr.Of(openapiExperiment.ExptRunModeSuaMultiTurn),
				SuaMode: gptr.Of(tc.openAPI),
			})
			require.NotNil(t, dom)
			require.True(t, dom.IsSetSuaMode(), "合法 sua_mode 不该被丢弃")

			// 第二跳: domain int 枚举 → entity 字符串 (落 eval_conf 的那一步)。
			do := runModeConfigDTO2DO(&domain_expt.RunModeConfig{
				RunMode: dom.RunMode,
				SuaMode: dom.SuaMode,
			})
			require.NotNil(t, do)

			assert.Equal(t, tc.want, do.SuaMode)
			// 闭合性: 落库值必须与 OpenAPI 入参**逐字相同**, 不允许再有隐式归一。
			assert.Equal(t, string(tc.openAPI), string(do.SuaMode),
				"往返必须闭合: 入口收 %q 就该落库 %q, 中间不得偷偷改写字面量", tc.openAPI, tc.openAPI)
		})
	}
}

// 非法 sua_mode 在 OpenAPI 入口被**静默丢弃** (不设值), 而不是报错 —— 这是当前
// OpenAPIRunModeConfigDTO2Domain 的既有行为 (`if sm, ok := ...; ok` 的 ok=false 分支跳过)。
//
// 实测证据: 拿真非法值 "bogus" 发的实验 7590112175193295618, 落库 eval_conf 的
// run_mode_config 里**根本没有 sua_mode 这个 key** (只剩 run_mode/max_run_minutes/
// sua_model_id), 而不是带着 "bogus" 落进去。
//
// 由此推出一个重要结论, 本测试即为其护栏: **commercial runtimeRunMode 的"未知 sua_mode
// 报错"分支走不到 OpenAPI 这条路**。因为非法值在入口就被丢成了"没配", 到 commercial 时
// suaMode == "" 命中合法缺省分支, 按 human_loop 跑完。那条报错逻辑只对**绕过 OpenAPI
// 入口**的调用方 (内部 RPC / 直接构造 entity / 未来新接口) 生效。
//
// 这里只钉住"丢弃"这一事实, 不改它 —— 入口应当报错还是丢弃是产品决策 (报错更符合本次
// "拒绝静默降级"的取向), 留给薛一正定; 见调研报告「发现但没改的问题」。
func TestSuaMode_InvalidOpenAPIValueIsDroppedNotErrored(t *testing.T) {
	t.Parallel()

	for _, bad := range []openapiExperiment.SuaMode{
		"bogus",     // 实测发过的真非法值 (实验 7590112175193295618)
		"humanloop", // 统一前的旧值, 现在同样非法
		"HumanLoop",
		"",
	} {
		bad := bad
		t.Run(string(bad), func(t *testing.T) {
			t.Parallel()

			dom := OpenAPIRunModeConfigDTO2Domain(&openapiExperiment.RunModeConfig{
				RunMode: gptr.Of(openapiExperiment.ExptRunModeSuaMultiTurn),
				SuaMode: gptr.Of(bad),
			})
			require.NotNil(t, dom)
			assert.False(t, dom.IsSetSuaMode(),
				"非法 sua_mode %q 当前被静默丢弃(不设值); 若哪天改成报错/透传, 本断言应随之更新", bad)

			// run_mode 是合法值, 不受 sua_mode 非法的影响 —— 确认丢弃是**字段级**的,
			// 不会顺手把整个 RunModeConfig 废掉。
			require.True(t, dom.IsSetRunMode())
			assert.Equal(t, domain_expt.ExptRunMode_SuaMultiTurn, dom.GetRunMode())
		})
	}
}
