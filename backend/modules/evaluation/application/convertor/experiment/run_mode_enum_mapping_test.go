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

// run_mode 在这条链上有**三种表示**, 且第三种与第四种编号刻意不同:
//
//	OpenAPI 字符串   "sua_multi_turn"    (对外契约, 用户书写)
//	domain int 枚举  ExptRunMode = 3     (内部 RPC / 落库前)
//	entity 字符串    "sua_multi_turn"    (落 eval_conf)
//	case-file 整数   3 或 4 (按 sua_mode 折叠) / goal=5
//
// **domain 枚举的整数与 case-file 的整数不是同一套**: domain 里 Goal=4, 而 case-file 里
// goal=5 —— 因为 sua_multi_turn 在下发时要折叠成 sua_loop(3)/sua_human_loop(4) 两个跑法,
// 把 goal 顶到了 5。任何"拿 domain 枚举值直接当 case-file run_mode"的捷径都会让 goal 实验
// 按 sua_human_loop 跑完并 success。本文件钉住三种表示之间的映射, 以及那处刻意的编号分歧。

// 字符串→domain int 的全表。这一跳错了, 用户配 A 跑法、实验按 B 跑法执行且照样 success。
func TestOpenAPIRunModeToDomain_FullTable(t *testing.T) {
	t.Parallel()

	want := map[openapiExperiment.ExptRunMode]domain_expt.ExptRunMode{
		openapiExperiment.ExptRunModeSingleTurn:           domain_expt.ExptRunMode_SingleTurn,
		openapiExperiment.ExptRunModeFixedScriptMultiTurn: domain_expt.ExptRunMode_FixedScriptMultiTurn,
		openapiExperiment.ExptRunModeSuaMultiTurn:         domain_expt.ExptRunMode_SuaMultiTurn,
		openapiExperiment.ExptRunModeGoal:                 domain_expt.ExptRunMode_Goal,
	}
	for in, exp := range want {
		got, ok := openAPIRunModeToDomain(in)
		assert.True(t, ok, "合法 run_mode %q 应被识别", in)
		assert.Equal(t, exp, got, "run_mode %q 映射错了 —— 实验会按另一个跑法执行", in)
	}

	// 四个值必须映到四个互不相同的枚举 —— 撞值意味着两个跑法在内部不可区分。
	seen := map[domain_expt.ExptRunMode]openapiExperiment.ExptRunMode{}
	for in := range want {
		got, _ := openAPIRunModeToDomain(in)
		prev, dup := seen[got]
		assert.False(t, dup, "run_mode %q 与 %q 撞了同一个 domain 枚举 %d", in, prev, got)
		seen[got] = in
	}
	assert.Len(t, seen, 4)
}

// domain int → entity 字符串, 以及**每一个合法跑法的完整往返闭合**。
//
// 闭合性是这条链唯一的护栏: 中间任何一跳漂了, 现象都是"落库的跑法与用户填的不是同一个",
// 而实验依然 success。此前 sua_mode 就是这么漏的 (往返把字面量差异吃掉)。
func TestRunMode_OpenAPIToEntityRoundTripIsClosed(t *testing.T) {
	t.Parallel()

	cases := []struct {
		openAPI openapiExperiment.ExptRunMode
		want    entity.RunMode
	}{
		{openapiExperiment.ExptRunModeSingleTurn, entity.RunModeSingleTurn},
		{openapiExperiment.ExptRunModeFixedScriptMultiTurn, entity.RunModeFixedScriptMultiTurn},
		{openapiExperiment.ExptRunModeSuaMultiTurn, entity.RunModeSUAMultiTurn},
		{openapiExperiment.ExptRunModeGoal, entity.RunModeGoal},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.openAPI), func(t *testing.T) {
			t.Parallel()

			dom, err := OpenAPIRunModeConfigDTO2Domain(&openapiExperiment.RunModeConfig{RunMode: gptr.Of(tc.openAPI)})
			require.NoError(t, err)
			require.NotNil(t, dom)
			require.True(t, dom.IsSetRunMode(), "合法 run_mode 不该被丢弃")

			do := runModeConfigDTO2DO(&domain_expt.RunModeConfig{RunMode: dom.RunMode})
			require.NotNil(t, do)
			assert.Equal(t, tc.want, do.RunMode)
			// 闭合: 入口收什么词, 落库就该是同一个词 —— entity 常量与 IDL 常量不得再漂。
			assert.Equal(t, string(tc.openAPI), string(do.RunMode),
				"往返必须闭合: 入口 %q 就该落库 %q", tc.openAPI, tc.openAPI)

			// 再回显一次 (详情页读的就是这条): 回显值必须还能映回同一个 OpenAPI 字面量。
			back := runModeConfigDO2DTO(do)
			require.NotNil(t, back)
			assert.Equal(t, dom.GetRunMode(), back.GetRunMode(), "回显把跑法改写了, 详情页会显示错的跑法")
		})
	}
}

// **刻意的编号分歧**: domain 枚举的整数值与 case-file 消费的整数值不是同一套。
//
// domain: SingleTurn=1 FixedScript=2 SuaMultiTurn=3 Goal=4
// case-file: single_turn=1 fixed_script=2 sua_loop=3 sua_human_loop=4 goal=5
//
// 分歧的原因: sua_multi_turn 是平台对外契约的聚合态, 下发前按 sua_mode 折叠成
// sua_loop(3)/sua_human_loop(4) 两个 runtime 跑法, 把 goal 顶到了 5。
//
// 为什么这条测试值得存在: 两处都叫 "run_mode"、都是小整数, 排查时极易拿 domain 的 4
// 去对 case-file 日志里的 run_mode=4 —— 而那是 sua_human_loop 不是 goal。若哪天有人
// "顺手统一"两套编号 (把 RunModeToInt 的 goal 改回 4, 或直接用 domain 枚举值下发),
// goal 实验就会按 sua_human_loop 跑完并 success。这里同时钉住"分歧存在"这个事实。
func TestRunMode_DomainEnumAndCaseFileIntDeliberatelyDiverge(t *testing.T) {
	t.Parallel()

	// 三个跑法两套编号一致 —— 恰好一致, 不是设计目标, 但错了同样是 bug。
	assert.Equal(t, 1, entity.RunModeToInt(entity.RunModeSingleTurn))
	assert.EqualValues(t, 1, domain_expt.ExptRunMode_SingleTurn)
	assert.Equal(t, 2, entity.RunModeToInt(entity.RunModeFixedScriptMultiTurn))
	assert.EqualValues(t, 2, domain_expt.ExptRunMode_FixedScriptMultiTurn)

	// goal 是分歧点: domain 是 4, case-file 是 5。
	assert.EqualValues(t, 4, domain_expt.ExptRunMode_Goal, "domain 枚举里 goal 是 4")
	assert.Equal(t, 5, entity.RunModeToInt(entity.RunModeGoal), "case-file 里 goal 是 5")
	assert.NotEqualValues(t, domain_expt.ExptRunMode_Goal, entity.RunModeToInt(entity.RunModeGoal),
		"两套编号刻意不同; 若哪天相等, 说明有人统一了编号 —— 必须同步 runtime 的解析表, 否则 goal 实验会按 sua_human_loop 跑")

	// domain 的 4 在 case-file 编号里属于 sua_human_loop, 不是 goal。
	assert.Equal(t, 4, entity.RunModeToInt(entity.RunModeSUAHumanLoopMultiTurn),
		"case-file 的 4 = sua_human_loop; 排查时别拿它对 domain 的 Goal=4")
}

// entity → domain 的回显方向: 两个 **runtime 独有的折叠态**在对外枚举里没有对应值。
//
// 这条钉的是一条边界契约: eval_conf 里**只应存平台对外契约的聚合态** sua_multi_turn,
// 折叠成 sua_loop / sua_human_loop 只发生在下发 case-file 的那一跳、不落库。若哪天有人
// 把折叠结果写回了 eval_conf, 回显就会静默变成 single_turn (对外枚举没有这两个值, 只能兜底),
// 详情页显示"单轮"而实验实际跑的是 SUA 多轮。本测试把这个损失点显式记下来。
func TestSuaRunModeDO2DTO_FoldedRuntimeModesHaveNoExternalEnum(t *testing.T) {
	t.Parallel()

	// 对外契约的四个值各归各位。
	assert.Equal(t, domain_expt.ExptRunMode_SingleTurn, suaRunModeDO2DTO(entity.RunModeSingleTurn))
	assert.Equal(t, domain_expt.ExptRunMode_FixedScriptMultiTurn, suaRunModeDO2DTO(entity.RunModeFixedScriptMultiTurn))
	assert.Equal(t, domain_expt.ExptRunMode_SuaMultiTurn, suaRunModeDO2DTO(entity.RunModeSUAMultiTurn))
	assert.Equal(t, domain_expt.ExptRunMode_Goal, suaRunModeDO2DTO(entity.RunModeGoal))

	// 折叠态与未知值一律兜底 SingleTurn —— 记录这个已知损失: 折叠态不该出现在 eval_conf。
	for _, m := range []entity.RunMode{
		entity.RunModeSUALoopMultiTurn,
		entity.RunModeSUAHumanLoopMultiTurn,
		"", "bogus",
	} {
		assert.Equal(t, domain_expt.ExptRunMode_SingleTurn, suaRunModeDO2DTO(m),
			"run_mode %q 在对外枚举里没有对应值, 回显只能兜底 single_turn —— 所以它不该被写进 eval_conf", m)
	}
}

// entity → domain 的 sua_mode 回显。
//
// ⚠️ 这里的 default 是 **HumanLoop 而不是零值**, 与入口方向的 suaModeDTO2DO(default 返回空串)
// 刻意不对称: DTO 侧 sua_mode 是 optional 指针, 调用方
// runModeConfigDO2DTO 已经用 `if do.SuaMode != ""` 挡掉了"未配"这一档, 所以本函数只会在
// **确实配了某个值**时被调到, 兜底成 human_loop 等于"认不出的值按默认跑法显示"。
//
// 这条测试的价值在于**钉住那个前置守卫的必要性**: 若哪天有人去掉 runModeConfigDO2DTO 里的
// 空值判断改成无条件回显, 未配 sua_mode 的实验(单轮 / fixed_script 实验都是这样)会在详情页
// 凭空显示 "human_loop", 用户以为自己配过 SUA。所以下面既断言映射, 也断言那个守卫仍在。
func TestSuaModeDO2DTO_DefaultOnlyReachableWhenValueIsSet(t *testing.T) {
	t.Parallel()

	assert.Equal(t, domain_expt.SuaMode_HumanLoop, suaModeDO2DTO(entity.SuaModeHumanLoop))
	assert.Equal(t, domain_expt.SuaMode_Loop, suaModeDO2DTO(entity.SuaModeLoop))
	assert.Equal(t, domain_expt.SuaMode_Fixed, suaModeDO2DTO(entity.SuaModeFixed))
	// 旧值 "humanloop" 已废弃且刻意不兼容, 走 default 与未知值同级。
	assert.Equal(t, domain_expt.SuaMode_HumanLoop, suaModeDO2DTO("humanloop"))
	assert.Equal(t, domain_expt.SuaMode_HumanLoop, suaModeDO2DTO("bogus"))

	// 守卫: 未配 sua_mode 的实验回显时不得出现 sua_mode 键。
	bare := runModeConfigDO2DTO(&entity.RunModeConfig{RunMode: entity.RunModeSingleTurn})
	require.NotNil(t, bare)
	assert.Nil(t, bare.SuaMode,
		"未配 sua_mode 不能回显成 human_loop —— 否则单轮实验的详情页会凭空多出一个 SUA 配置")
}

// max_run_minutes / sua_model_name 的往返, 以及 **sua_model_id 刻意不回显**。
//
// sua_model_id 是运维侧配置(由平台配置中心的替换规则自带), 已从对外 IDL 移除; entity 上保留该字段只为
// commercial 解析模型密钥。这条测试钉住"它不会顺着回显路径泄回调用方" —— 它虽然不是密钥
// 本身, 但把内部模型 ID 回显出去等于把运维配置暴露成用户可见字段, 之后就很难再收回。
func TestRunModeConfig_MaxRunMinutesRoundTripAndSuaModelIDNotEchoed(t *testing.T) {
	t.Parallel()

	dom, err := OpenAPIRunModeConfigDTO2Domain(&openapiExperiment.RunModeConfig{
		RunMode:       gptr.Of(openapiExperiment.ExptRunModeSuaMultiTurn),
		MaxRunMinutes: gptr.Of(int32(45)),
		SuaModelName:  gptr.Of("debug-model"),
	})
	require.NoError(t, err)
	assert.Equal(t, int32(45), dom.GetMaxRunMinutes())
	assert.Equal(t, "debug-model", dom.GetSuaModelName())

	do := runModeConfigDTO2DO(dom)
	require.NotNil(t, do)
	assert.Equal(t, 45, do.MaxRunMinutes)
	assert.Equal(t, "debug-model", do.SuaModelName)
	assert.Zero(t, do.SuaModelID, "sua_model_id 已从入口移除, 不该从请求里被重新接上")

	// 回显: max_run_minutes 带回, 而运维侧注入的 sua_model_id 不带回。
	back := runModeConfigDO2DTO(&entity.RunModeConfig{
		RunMode:       entity.RunModeSUAMultiTurn,
		MaxRunMinutes: 45,
		SuaModelID:    987654,
	})
	require.NotNil(t, back)
	assert.Equal(t, int32(45), back.GetMaxRunMinutes())
	// DTO 上已无 sua_model_id 字段, 回显体里也不该出现该值的任何痕迹。
	assert.Nil(t, back.SuaModelName, "未配 sua_model_name 不回显")

	// max_run_minutes=0 视为未配, 不回显 (否则详情页会显示"最长运行 0 分钟")。
	zero := runModeConfigDO2DTO(&entity.RunModeConfig{RunMode: entity.RunModeSingleTurn})
	require.NotNil(t, zero)
	assert.Nil(t, zero.MaxRunMinutes)
}

// nil 边界: 三个转换器的 nil 入必须 nil 出、不 panic、且不产出空壳。
//
// "不产出空壳"是重点: 提交链路上 `createReq.RunModeConfig = <转换结果>` 是无条件赋值,
// 若 nil 入参返回 &RunModeConfig{}, 存量的单轮实验就会凭空带上一个空 run_mode_config,
// 落库 eval_conf 多一个键、下游 `RunModeConfig != nil` 的闸门全部被打开
// (执行期会开始透传 ext["builtin_run_mode_config"], CheckExpt 的多轮校验门也开始参与)。
func TestRunModeConfigConvertors_NilInNilOut(t *testing.T) {
	t.Parallel()

	dom, err := OpenAPIRunModeConfigDTO2Domain(nil)
	require.NoError(t, err, "未配 run_mode_config 是合法的, 不该报错")
	assert.Nil(t, dom, "nil 入参不能返回空壳, 否则存量单轮实验会被误判为配了跑法")

	assert.Nil(t, runModeConfigDTO2DO(nil))
	assert.Nil(t, runModeConfigDO2DTO(nil))

	// 空 DTO (对象在但一个字段都没设): 转换出的 entity 必须是"全空"的,
	// 尤其 RunMode 必须留空 —— 留空才会被 entity.IsNewRunModeLink 判为旧链路。
	do := runModeConfigDTO2DO(&domain_expt.RunModeConfig{})
	require.NotNil(t, do)
	assert.Empty(t, do.RunMode, "没设 run_mode 就不该凭空得到 single_turn —— 那会把旧链路实验判成新链路")
	assert.Empty(t, do.SuaMode)
	assert.False(t, entity.IsNewRunModeLink(do), "空配置必须仍被判为旧链路")
}

// 读侧回显: 写侧 SubmitExperiment 能配跑法, 读侧此前无字段 —— OpenAPI 调用方提交完
// **查不到自己配了什么跑法**, 而内部接口一直能回显。本组用例钉住补齐后的读路径。
func TestRunModeConfigDomain2OpenAPI_RoundTripsEveryField(t *testing.T) {
	t.Parallel()

	in := &domain_expt.RunModeConfig{
		RunMode:                  gptr.Of(domain_expt.ExptRunMode_SuaMultiTurn),
		SuaMode:                  gptr.Of(domain_expt.SuaMode_Loop),
		MaxRunMinutes:            gptr.Of(int32(240)),
		MaxTurns:                 gptr.Of(int32(16)),
		SuaModelName:             gptr.Of("some-model"),
		SuaGoal:                  gptr.Of("修到全部通过"),
		SuaPersona:               gptr.Of("资深工程师"),
		SuaBehavioralConstraints: gptr.Of("每轮只发一条指令"),
		SuaPeTemplate:            gptr.Of("评估结果: {{eval_result}}"),
	}

	got := RunModeConfigDomain2OpenAPI(in)
	require.NotNil(t, got)

	// 逐字段断言而不是只挑一两个: 这是手写的字段拷贝, 新增字段忘了接线会静默丢失 ——
	// 调用方看到的是"我没配这项", 而不是任何错误。
	assert.Equal(t, openapiExperiment.ExptRunModeSuaMultiTurn, gptr.Indirect(got.RunMode))
	assert.Equal(t, openapiExperiment.SuaModeLoop, gptr.Indirect(got.SuaMode))
	assert.Equal(t, int32(240), gptr.Indirect(got.MaxRunMinutes))
	assert.Equal(t, int32(16), gptr.Indirect(got.MaxTurns))
	assert.Equal(t, "some-model", gptr.Indirect(got.SuaModelName))
	assert.Equal(t, "修到全部通过", gptr.Indirect(got.SuaGoal))
	assert.Equal(t, "资深工程师", gptr.Indirect(got.SuaPersona))
	assert.Equal(t, "每轮只发一条指令", gptr.Indirect(got.SuaBehavioralConstraints))
	assert.Equal(t, "评估结果: {{eval_result}}", gptr.Indirect(got.SuaPeTemplate))
}

// 四个对外跑法都必须能回显, 且**回显值要能被写侧重新解析回同一个 domain 值** ——
// 读写不闭环就意味着"照抄详情页的配置重新提交一次"会得到另一个跑法。
func TestRunModeConfigDomain2OpenAPI_ClosesTheLoopWithWriteSide(t *testing.T) {
	t.Parallel()

	for _, dm := range []domain_expt.ExptRunMode{
		domain_expt.ExptRunMode_SingleTurn,
		domain_expt.ExptRunMode_FixedScriptMultiTurn,
		domain_expt.ExptRunMode_SuaMultiTurn,
		domain_expt.ExptRunMode_Goal,
	} {
		out := RunModeConfigDomain2OpenAPI(&domain_expt.RunModeConfig{RunMode: gptr.Of(dm)})
		require.NotNil(t, out.RunMode, "%v 必须能回显", dm)

		back, ok := openAPIRunModeToDomain(gptr.Indirect(out.RunMode))
		require.True(t, ok, "回显值 %q 必须能被写侧解析", gptr.Indirect(out.RunMode))
		assert.Equal(t, dm, back, "读写必须闭环, 否则照抄详情页重新提交会换跑法")
	}
}

// nil 必须出 nil, 不能出一个空壳: 空壳会让"这个实验没配跑法"看起来像"配了但全是默认值",
// 而下游多处以 `RunModeConfig != nil` 当"是否新链路"的判据。
func TestRunModeConfigDomain2OpenAPI_NilInNilOut(t *testing.T) {
	t.Parallel()
	assert.Nil(t, RunModeConfigDomain2OpenAPI(nil))
}

// 认不出的枚举值只留空**那一个字段**, 其余字段照常回显 —— 不能因为一个枚举不认识就整块丢掉,
// 也不能猜一个值填进去 (猜的值会被调用方当成真配置)。
func TestRunModeConfigDomain2OpenAPI_UnknownEnumLeavesOnlyThatFieldEmpty(t *testing.T) {
	t.Parallel()

	got := RunModeConfigDomain2OpenAPI(&domain_expt.RunModeConfig{
		RunMode:       gptr.Of(domain_expt.ExptRunMode(9999)),
		SuaMode:       gptr.Of(domain_expt.SuaMode(9999)),
		MaxRunMinutes: gptr.Of(int32(30)),
	})

	require.NotNil(t, got)
	assert.Nil(t, got.RunMode, "认不出的 run_mode 留空, 不猜")
	assert.Nil(t, got.SuaMode, "认不出的 sua_mode 留空, 不猜")
	assert.Equal(t, int32(30), gptr.Indirect(got.MaxRunMinutes), "其余字段不受影响")
}

// 两个 OpenAPI 读路径都要接线。DomainExperimentDTO2OpenAPI 走 DTO, OpenAPIExptDO2DTO 走
// entity 直转 —— 漏接任一个, 就会出现"列表能看到跑法、详情看不到"这类只在某个接口上缺字段的
// 现象, 比整体没有更难排查。
func TestBothOpenAPIReadPathsEchoRunModeConfig(t *testing.T) {
	t.Parallel()

	t.Run("DTO 路径", func(t *testing.T) {
		t.Parallel()
		out := DomainExperimentDTO2OpenAPI(&domain_expt.Experiment{
			RunModeConfig: &domain_expt.RunModeConfig{RunMode: gptr.Of(domain_expt.ExptRunMode_Goal)},
		})
		require.NotNil(t, out.RunModeConfig)
		assert.Equal(t, openapiExperiment.ExptRunModeGoal, gptr.Indirect(out.RunModeConfig.RunMode))
	})

	t.Run("entity 直转路径", func(t *testing.T) {
		t.Parallel()
		out := OpenAPIExptDO2DTO(&entity.Experiment{
			EvalConf: &entity.EvaluationConfiguration{
				RunModeConfig: &entity.RunModeConfig{RunMode: entity.RunModeGoal},
			},
		})
		require.NotNil(t, out.RunModeConfig)
		assert.Equal(t, openapiExperiment.ExptRunModeGoal, gptr.Indirect(out.RunModeConfig.RunMode))
	})
}
