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
			dom, err := OpenAPIRunModeConfigDTO2Domain(&openapiExperiment.RunModeConfig{
				RunMode: gptr.Of(openapiExperiment.ExptRunModeSuaMultiTurn),
				SuaMode: gptr.Of(tc.openAPI),
			})
			require.NoError(t, err, "合法 sua_mode 不该报错")
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

// 非空但认不出的 sua_mode 必须在**创建实验的入口**就报参数错误, 实验根本不该被创建。
//
// 这是本次修的第二个 bug 的护栏。此前入口是 `if v, ok := conv(..); ok { set }` —— ok==false
// 时什么都不做, 非法值被静默丢弃成"没配"(字段保持 nil → int 0), 再到 suaModeDTO2DO 的
// default 兜成 humanloop。实测拿真非法值 sua_mode="bogus" 发的实验 7590112175193295618
// 因此一路跑到 success、按 humanloop 跑了 22 轮, 落库 eval_conf.run_mode_config 里连
// sua_mode 这个 key 都没有; commercial runtimeRunMode 那个"未知 sua_mode 报错"分支压根走不到
// (到那里时 suaMode 已经是 "" 或 humanloop, 都是合法值) —— 所以那条报错逻辑从来没被真验过。
//
// 反向验证 (证明本测试真的在守): 把 OpenAPIRunModeConfigDTO2Domain 里 sua_mode 的
// `if !ok { return nil, err }` 改回静默丢弃 (`if ok { set }`), 本测试全部 case FAIL。已还原。
func TestSuaMode_InvalidValueRejectedAtSubmit(t *testing.T) {
	t.Parallel()

	for _, bad := range []openapiExperiment.SuaMode{
		"bogus",     // 实测发过的真非法值 (实验 7590112175193295618)
		"humanloop", // 统一前的旧值, 现在同样非法 (刻意不做双值兼容)
		"HumanLoop", // 大小写
		"human-loop",
		"loops",
	} {
		bad := bad
		t.Run(string(bad), func(t *testing.T) {
			t.Parallel()

			dom, err := OpenAPIRunModeConfigDTO2Domain(&openapiExperiment.RunModeConfig{
				RunMode: gptr.Of(openapiExperiment.ExptRunModeSuaMultiTurn),
				SuaMode: gptr.Of(bad),
			})
			require.Error(t, err, "非法 sua_mode %q 必须在入口报错, 不能静默丢弃", bad)
			assert.Contains(t, err.Error(), string(bad), "报错要指名道姓, 否则用户看不出是哪个值不对")
			assert.Nil(t, dom, "报错时不该返回半成品配置")
		})
	}
}

// run_mode 与 sua_mode 同理: 非空但认不出一律入口报错 (此前同样是静默丢弃)。
func TestRunMode_InvalidValueRejectedAtSubmit(t *testing.T) {
	t.Parallel()

	for _, bad := range []openapiExperiment.ExptRunMode{"bogus", "SingleTurn", "sua"} {
		bad := bad
		t.Run(string(bad), func(t *testing.T) {
			t.Parallel()
			dom, err := OpenAPIRunModeConfigDTO2Domain(&openapiExperiment.RunModeConfig{
				RunMode: gptr.Of(bad),
			})
			require.Error(t, err, "非法 run_mode %q 必须在入口报错", bad)
			assert.Contains(t, err.Error(), string(bad))
			assert.Nil(t, dom)
		})
	}
}

// 合法缺省: sua_mode / run_mode 都不传时不报错, 也不设值 —— "没配"必须能如实表达成"没配",
// 由下游既有的缺省逻辑 (commercial runtimeRunMode 的 `case "":` 按默认 human_loop 跑) 接管。
//
// **本测试与 suaModeDTO2DO 的 default 改动配对**: 原 default 把 int 0 (未设置) 兜成
// humanloop, 等于谎报"用户显式配了 human_loop"。现在 0 → 空串, 语义如实。
func TestSuaMode_UnsetIsLegalDefaultNotHumanLoop(t *testing.T) {
	t.Parallel()

	// 入口: 两个枚举都不传 → 不报错、不设值。
	dom, err := OpenAPIRunModeConfigDTO2Domain(&openapiExperiment.RunModeConfig{
		MaxRunMinutes: gptr.Of(int32(30)),
	})
	require.NoError(t, err, "缺省 (不传枚举) 是合法的")
	require.NotNil(t, dom)
	assert.False(t, dom.IsSetSuaMode(), "没传就不该设值")
	assert.False(t, dom.IsSetRunMode())

	// 第二跳: domain int 未设置/未识别必须落成**空串**, 而不是某个具体跑法。
	// 直接调 suaModeDTO2DO: runModeConfigDTO2DO 只在 IsSetSuaMode() 为真时才调它, 所以
	// "字段整个没传"这条路走不到 default; 真能走到 default 的是**显式设了 0** (int 零值,
	// thrift optional 字段被赋零值) 和**未识别的非 0 值** (未来新增枚举 / 内部调用方乱传)。
	for _, unset := range []domain_expt.SuaMode{0, 99} {
		assert.Empty(t, suaModeDTO2DO(unset),
			`sua_mode int %d 必须落空串(合法缺省), 不能兜成 human_loop —— "未设置/认不出"与"显式配了 human_loop"是两件事`, unset)
	}

	// 三个已识别值仍各归各位 (确认改 default 没把正常映射带坏)。
	assert.Equal(t, entity.SuaModeHumanLoop, suaModeDTO2DO(domain_expt.SuaMode_HumanLoop))
	assert.Equal(t, entity.SuaModeLoop, suaModeDTO2DO(domain_expt.SuaMode_Loop))
	assert.Equal(t, entity.SuaModeFixed, suaModeDTO2DO(domain_expt.SuaMode_Fixed))
}
