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
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// fullOpenAPISkill 返回一个带完整 dist (9 字段) + credentials_keys 的 OpenAPI 技能声明,
// 用于逐字段对账透传链路 (OpenAPI → domain → entity)。
func fullOpenAPISkill() *openapiExperiment.AgentSkillDeclare {
	return &openapiExperiment.AgentSkillDeclare{
		SkillKey:     gptr.Of("repo-search"),
		SkillVersion: gptr.Of("v1.2.3"),
		Dist: &openapiExperiment.SkillDistDeclare{
			ChannelType:            gptr.Of("git"),
			GitURL:                 gptr.Of("git@github.com:coze-dev/skills.git"),
			Branch:                 gptr.Of("main"),
			Dir:                    gptr.Of("repo-search"),
			CommitHash:             gptr.Of("674fb6e"),
			FileURL:                gptr.Of("https://example.com/skill.tar.gz"),
			AgentBuddySource:       gptr.Of("agent_buddy"),
			AgentBuddySkillName:    gptr.Of("repo-search"),
			AgentBuddySkillVersion: gptr.Of("1"),
		},
		SetupScript:     gptr.Of("pip install -r requirements.txt"),
		CredentialsKeys: []string{"GITHUB_TOKEN", "TOS_AK"},
	}
}

// 入口 → 落库全链: skills 逐元素逐字段透传 (含完整 dist), 平台只搬运不解释。
func TestOpenAPIRunModeConfigDTO2Domain_SkillsPassthrough(t *testing.T) {
	t.Parallel()

	skill := fullOpenAPISkill()
	// 第二个元素不带 dist / credentials_keys: 两者都是可选的, 不该因缺省报错。
	bare := &openapiExperiment.AgentSkillDeclare{SkillKey: gptr.Of("bare-skill")}

	dom, err := OpenAPIRunModeConfigDTO2Domain(&openapiExperiment.RunModeConfig{
		Skills: []*openapiExperiment.AgentSkillDeclare{skill, bare},
	})
	require.NoError(t, err)
	require.NotNil(t, dom)
	require.Len(t, dom.Skills, 2)

	do := runModeConfigDTO2DO(dom)
	require.NotNil(t, do)
	require.Len(t, do.Skills, 2)

	got := do.Skills[0]
	assert.Equal(t, "repo-search", got.SkillKey)
	assert.Equal(t, "v1.2.3", got.SkillVersion)
	assert.Equal(t, "pip install -r requirements.txt", got.SetupScript)
	assert.Equal(t, []string{"GITHUB_TOKEN", "TOS_AK"}, got.CredentialsKeys)
	require.NotNil(t, got.Dist, "dist 非空时必须落到 entity")
	assert.Equal(t, "git", got.Dist.ChannelType)
	assert.Equal(t, "https://example.com/skill.tar.gz", got.Dist.FileURL)
	assert.Equal(t, "agent_buddy", got.Dist.AgentBuddySource)
	assert.Equal(t, "repo-search", got.Dist.AgentBuddySkillName)
	assert.Equal(t, "1", got.Dist.AgentBuddySkillVersion)
	assert.Equal(t, "git@github.com:coze-dev/skills.git", got.Dist.GitURL)
	assert.Equal(t, "main", got.Dist.Branch)
	assert.Equal(t, "repo-search", got.Dist.Dir)
	assert.Equal(t, "674fb6e", got.Dist.CommitHash)

	assert.Equal(t, "bare-skill", do.Skills[1].SkillKey)
	assert.Nil(t, do.Skills[1].Dist, "未配 dist 不该被构造成空 struct")
	assert.Empty(t, do.Skills[1].CredentialsKeys)
}

// 平台仅校验 skill_key 非空: 空值报 CommonInvalidParamCode, 文案带下标定位;
// dist 结构怪异 (如 channel_type 为空) 不拦截 —— 结构/语义校验归 runtime。
func TestOpenAPIRunModeConfigDTO2Domain_SkillKeyRequired(t *testing.T) {
	t.Parallel()

	t.Run("skill_key 为空 报错且带下标", func(t *testing.T) {
		t.Parallel()
		_, err := OpenAPIRunModeConfigDTO2Domain(&openapiExperiment.RunModeConfig{
			Skills: []*openapiExperiment.AgentSkillDeclare{
				fullOpenAPISkill(),
				{SkillKey: gptr.Of("")},
			},
		})
		require.Error(t, err)
		se, ok := errorx.FromStatusError(err)
		require.True(t, ok, "应是 StatusError 以携带错误码")
		assert.Equal(t, int32(errno.CommonInvalidParamCode), se.Code())
		assert.Contains(t, err.Error(), "skills[1].skill_key is required")
	})

	t.Run("skill_key 未设置 等同为空", func(t *testing.T) {
		t.Parallel()
		_, err := OpenAPIRunModeConfigDTO2Domain(&openapiExperiment.RunModeConfig{
			Skills: []*openapiExperiment.AgentSkillDeclare{{}},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "skills[0].skill_key is required")
	})

	t.Run("dist 结构怪异不拦截 原样透传", func(t *testing.T) {
		t.Parallel()
		dom, err := OpenAPIRunModeConfigDTO2Domain(&openapiExperiment.RunModeConfig{
			Skills: []*openapiExperiment.AgentSkillDeclare{{
				SkillKey: gptr.Of("weird"),
				Dist:     &openapiExperiment.SkillDistDeclare{},
			}},
		})
		require.NoError(t, err)
		require.NotNil(t, dom.Skills[0].Dist)
		assert.Empty(t, dom.Skills[0].GetDist().GetChannelType())
	})
}

// 不传 skills 的存量请求体行为不变: 各层都不凭空构造空列表。
func TestOpenAPIRunModeConfigDTO2Domain_NoSkillsUnchanged(t *testing.T) {
	t.Parallel()

	dom, err := OpenAPIRunModeConfigDTO2Domain(&openapiExperiment.RunModeConfig{})
	require.NoError(t, err)
	require.NotNil(t, dom)
	assert.False(t, dom.IsSetSkills())

	do := runModeConfigDTO2DO(dom)
	require.NotNil(t, do)
	assert.Empty(t, do.Skills)
}

// 读侧 Domain2OpenAPI 回显: skills + skills_mode roundtrip 等值。
func TestRunModeConfigDomain2OpenAPI_SkillsEcho(t *testing.T) {
	t.Parallel()

	got := RunModeConfigDomain2OpenAPI(&domain_expt.RunModeConfig{
		SkillsMode: gptr.Of("merge_exp_first"),
		Skills: skillsDO2DTO([]*entity.AgentSkillDeclare{{
			SkillKey:        "repo-search",
			SkillVersion:    "v1.2.3",
			SetupScript:     "pip install -r requirements.txt",
			CredentialsKeys: []string{"GITHUB_TOKEN"},
			Dist: &entity.SkillDistDeclare{
				ChannelType: "git",
				GitURL:      "git@github.com:coze-dev/skills.git",
				Branch:      "main",
				Dir:         "repo-search",
				CommitHash:  "674fb6e",
			},
		}}),
	})
	require.NotNil(t, got)
	assert.Equal(t, "merge_exp_first", gptr.Indirect(got.SkillsMode))
	require.Len(t, got.Skills, 1)

	s := got.Skills[0]
	assert.Equal(t, "repo-search", s.GetSkillKey())
	assert.Equal(t, "v1.2.3", s.GetSkillVersion())
	assert.Equal(t, "pip install -r requirements.txt", s.GetSetupScript())
	assert.Equal(t, []string{"GITHUB_TOKEN"}, s.GetCredentialsKeys())
	require.NotNil(t, s.Dist)
	assert.Equal(t, "git", s.GetDist().GetChannelType())
	assert.Equal(t, "git@github.com:coze-dev/skills.git", s.GetDist().GetGitURL())
	assert.Equal(t, "main", s.GetDist().GetBranch())
	assert.Equal(t, "repo-search", s.GetDist().GetDir())
	assert.Equal(t, "674fb6e", s.GetDist().GetCommitHash())

	t.Run("未设值 nil 出 nil", func(t *testing.T) {
		t.Parallel()
		got := RunModeConfigDomain2OpenAPI(&domain_expt.RunModeConfig{})
		require.NotNil(t, got)
		assert.False(t, got.IsSetSkills())
	})
}

// DO→DTO 非空守卫: 存量 EvalConf JSON 无 skills 键 (entity.Skills 为空) 时回显不含该字段;
// 配了则逐元素带回 (照抄详情页配置重新提交不丢)。
func TestRunModeConfigDO2DTO_SkillsGuard(t *testing.T) {
	t.Parallel()

	t.Run("未配 skills 不回显", func(t *testing.T) {
		t.Parallel()
		dto := runModeConfigDO2DTO(&entity.RunModeConfig{RunMode: entity.RunModeSingleTurn})
		require.NotNil(t, dto)
		assert.False(t, dto.IsSetSkills())
	})

	t.Run("配了 skills 回显闭合", func(t *testing.T) {
		t.Parallel()
		do := &entity.RunModeConfig{
			SkillsMode: "merge",
			Skills: []*entity.AgentSkillDeclare{{
				SkillKey:     "repo-search",
				SkillVersion: "v1.2.3",
				Dist:         &entity.SkillDistDeclare{ChannelType: "git"},
			}},
		}
		dto := runModeConfigDO2DTO(do)
		require.NotNil(t, dto)
		require.Len(t, dto.Skills, 1)
		assert.Equal(t, "repo-search", dto.Skills[0].GetSkillKey())
		assert.Equal(t, "v1.2.3", dto.Skills[0].GetSkillVersion())
		require.NotNil(t, dto.Skills[0].Dist)
		assert.Equal(t, "git", dto.Skills[0].GetDist().GetChannelType())
	})
}
