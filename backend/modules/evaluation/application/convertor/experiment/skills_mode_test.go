// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"strings"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	openapiExperiment "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// skills_mode 是 SandboxAgent 跑法的技能模式 (merge / disable_test_case), 与 run_mode/sua_mode
// 同为受控枚举。入口 OpenAPIRunModeConfigDTO2Domain 做白名单校验: 合法值透传到 entity,
// 非法值报错 (不能静默透传 —— 会落到 case-file 让 runtime 收到无法识别的模式)。
// 本文件钉住「白名单放行/拒绝 + DO↔DTO 往返闭合 + 读侧回显」三段。
func TestOpenAPIRunModeConfigDTO2Domain_SkillsModeWhitelist(t *testing.T) {
	t.Parallel()

	t.Run("合法值 merge 透传", func(t *testing.T) {
		t.Parallel()
		dom, err := OpenAPIRunModeConfigDTO2Domain(&openapiExperiment.RunModeConfig{
			SkillsMode: gptr.Of("merge"),
		})
		require.NoError(t, err)
		require.NotNil(t, dom)
		assert.Equal(t, "merge", dom.GetSkillsMode())
	})

	t.Run("合法值 disable_test_case 透传", func(t *testing.T) {
		t.Parallel()
		dom, err := OpenAPIRunModeConfigDTO2Domain(&openapiExperiment.RunModeConfig{
			SkillsMode: gptr.Of("disable_test_case"),
		})
		require.NoError(t, err)
		require.NotNil(t, dom)
		assert.Equal(t, "disable_test_case", dom.GetSkillsMode())
	})

	t.Run("未设置(nil) 放行, 不产出 skills_mode", func(t *testing.T) {
		t.Parallel()
		dom, err := OpenAPIRunModeConfigDTO2Domain(&openapiExperiment.RunModeConfig{})
		require.NoError(t, err)
		require.NotNil(t, dom)
		assert.False(t, dom.IsSetSkillsMode(), "未配 skills_mode 不该被透传")
	})

	t.Run("空串 放行, 不产出 skills_mode", func(t *testing.T) {
		t.Parallel()
		dom, err := OpenAPIRunModeConfigDTO2Domain(&openapiExperiment.RunModeConfig{
			SkillsMode: gptr.Of(""),
		})
		require.NoError(t, err)
		require.NotNil(t, dom)
		assert.False(t, dom.IsSetSkillsMode(), "空串等同未配, 不该被透传")
	})

	t.Run("非法值 报错且含字段名", func(t *testing.T) {
		t.Parallel()
		_, err := OpenAPIRunModeConfigDTO2Domain(&openapiExperiment.RunModeConfig{
			SkillsMode: gptr.Of("bogus"),
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "skills_mode", "错误信息应点出出错的字段名")
	})
}

// DO↔DTO 往返: 入口合法值经 runModeConfigDTO2DO 落 entity, 再 runModeConfigDO2DTO 回 DTO,
// 必须闭合 —— 否则"照抄详情页配置重新提交"会丢失 skills_mode。
func TestRunModeConfig_SkillsModeRoundTrip(t *testing.T) {
	t.Parallel()

	for _, v := range []string{"merge", "disable_test_case"} {
		v := v
		t.Run(v, func(t *testing.T) {
			t.Parallel()
			dom, err := OpenAPIRunModeConfigDTO2Domain(&openapiExperiment.RunModeConfig{
				SkillsMode: gptr.Of(v),
			})
			require.NoError(t, err)

			do := runModeConfigDTO2DO(dom)
			require.NotNil(t, do)
			assert.Equal(t, v, do.SkillsMode, "DTO→DO skills_mode 丢失")

			back := runModeConfigDO2DTO(do)
			require.NotNil(t, back)
			assert.Equal(t, v, back.GetSkillsMode(), "DO→DTO 回显与落库值不一致")
		})
	}

	// 非空守卫: entity.SkillsMode 为空时, DTO 不得回显 (否则详情页凭空多出 skills_mode 键)。
	t.Run("空值不回显", func(t *testing.T) {
		t.Parallel()
		back := runModeConfigDO2DTO(&entity.RunModeConfig{RunMode: entity.RunModeSingleTurn})
		require.NotNil(t, back)
		assert.False(t, back.IsSetSkillsMode(), "未配 skills_mode 不该回显")
	})
}

// 读侧 Domain2OpenAPI 回显: 合法值带出, 非法(存量脏数据)值留空不猜。
func TestRunModeConfigDomain2OpenAPI_SkillsModeEcho(t *testing.T) {
	t.Parallel()

	t.Run("合法值回显", func(t *testing.T) {
		t.Parallel()
		got := RunModeConfigDomain2OpenAPI(&domain_expt.RunModeConfig{
			SkillsMode: gptr.Of("disable_test_case"),
		})
		require.NotNil(t, got)
		assert.Equal(t, "disable_test_case", gptr.Indirect(got.SkillsMode))
	})

	t.Run("未设值 nil 出 nil", func(t *testing.T) {
		t.Parallel()
		got := RunModeConfigDomain2OpenAPI(&domain_expt.RunModeConfig{})
		require.NotNil(t, got)
		assert.Nil(t, got.SkillsMode)
	})

	// 存量脏数据(校验加入前已落库的非法值): 回显路径不校验, 原样带出 ——
	// 校验只在入口做, 回显若再校验会把存量值静默吞掉, 详情页看不到实际落库内容。
	t.Run("存量脏值原样带出, 不校验不吞", func(t *testing.T) {
		t.Parallel()
		got := RunModeConfigDomain2OpenAPI(&domain_expt.RunModeConfig{
			SkillsMode: gptr.Of("legacy-bogus"),
		})
		require.NotNil(t, got)
		assert.Equal(t, "legacy-bogus", gptr.Indirect(got.SkillsMode))
	})
}

// 兜底: isValidSkillsMode 的全表, 防止有人新增枚举忘了同步白名单。
func TestIsValidSkillsMode(t *testing.T) {
	t.Parallel()
	assert.True(t, isValidSkillsMode("merge"))
	assert.True(t, isValidSkillsMode("disable_test_case"))
	assert.False(t, isValidSkillsMode(""))
	assert.False(t, isValidSkillsMode("MERGE"))
	assert.False(t, isValidSkillsMode("bogus"))
	assert.False(t, isValidSkillsMode(strings.Repeat("x", 100)))
}
