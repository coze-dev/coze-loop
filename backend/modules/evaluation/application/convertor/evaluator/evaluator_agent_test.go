// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	commondto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	evaluatordo "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestConvertEvaluatorContent2DO_Agent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		content   *evaluatordto.EvaluatorContent
		wantErr   bool
		assertion func(t *testing.T, got *evaluatordo.Evaluator)
	}{
		{
			name:    "missing agent content",
			content: &evaluatordto.EvaluatorContent{},
			wantErr: true,
		},
		{
			name: "agent ok",
			content: &evaluatordto.EvaluatorContent{
				InputSchemas:  []*commondto.ArgsSchema{{Key: gptr.Of("in")}},
				OutputSchemas: []*commondto.ArgsSchema{{Key: gptr.Of("out")}},
				AgentEvaluator: &evaluatordto.AgentEvaluator{
					AgentConfig: &commondto.AgentConfig{AgentType: gptr.Of("react_agent")},
					ModelConfig: &commondto.ModelConfig{ModelID: gptr.Of(int64(1))},
					SkillConfigs: []*commondto.SkillConfig{
						{SkillID: gptr.Of(int64(1)), Version: gptr.Of("v1")},
						nil,
					},
					PromptConfig: &evaluatordto.AgentEvaluatorPromptConfig{
						MessageList: []*commondto.Message{
							{Content: &commondto.Content{Text: gptr.Of("t1")}},
						},
						OutputRules: &evaluatordto.AgentEvaluatorPromptConfigOutputRules{
							ScorePrompt:       &commondto.Message{Content: &commondto.Content{Text: gptr.Of("score")}},
							ReasoningPrompt:   &commondto.Message{Content: &commondto.Content{Text: gptr.Of("reason")}},
							ExtraOutputPrompt: &commondto.Message{Content: &commondto.Content{Text: gptr.Of("extra")}},
						},
					},
				},
			},
			assertion: func(t *testing.T, got *evaluatordo.Evaluator) {
				if assert.NotNil(t, got) && assert.NotNil(t, got.AgentEvaluatorVersion) {
					assert.Equal(t, evaluatordo.EvaluatorTypeAgent, got.AgentEvaluatorVersion.EvaluatorType)
					assert.NotNil(t, got.AgentEvaluatorVersion.AgentConfig)
					assert.Equal(t, evaluatordo.AgentType("react_agent"), got.AgentEvaluatorVersion.AgentConfig.AgentType)
					assert.NotNil(t, got.AgentEvaluatorVersion.ModelConfig)
					assert.Len(t, got.AgentEvaluatorVersion.SkillConfigs, 1)
					assert.Equal(t, int64(1), gptr.Indirect(got.AgentEvaluatorVersion.SkillConfigs[0].SkillID))
					assert.NotNil(t, got.AgentEvaluatorVersion.PromptConfig)
					assert.Len(t, got.AgentEvaluatorVersion.PromptConfig.MessageList, 1)
					assert.NotNil(t, got.AgentEvaluatorVersion.PromptConfig.OutputRules)
					assert.Equal(t, "score", got.AgentEvaluatorVersion.PromptConfig.OutputRules.ScorePrompt.Content.GetText())
					assert.Len(t, got.AgentEvaluatorVersion.InputSchemas, 1)
					assert.Len(t, got.AgentEvaluatorVersion.OutputSchemas, 1)
				}
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ConvertEvaluatorContent2DO(tc.content, evaluatordto.EvaluatorType_Agent)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tc.assertion != nil {
				tc.assertion(t, got)
			}
		})
	}
}

func TestConvertAgentEvaluatorVersion_RoundTrip(t *testing.T) {
	t.Parallel()

	dto := &evaluatordto.EvaluatorVersion{
		ID:          gptr.Of(int64(10)),
		Version:     gptr.Of("1.0.0"),
		Description: gptr.Of("d"),
		EvaluatorContent: &evaluatordto.EvaluatorContent{
			InputSchemas:  []*commondto.ArgsSchema{{Key: gptr.Of("in")}},
			OutputSchemas: []*commondto.ArgsSchema{{Key: gptr.Of("out")}},
			AgentEvaluator: &evaluatordto.AgentEvaluator{
				AgentConfig: &commondto.AgentConfig{AgentType: gptr.Of("react_agent")},
				SkillConfigs: []*commondto.SkillConfig{
					{SkillID: gptr.Of(int64(1)), Version: gptr.Of("v1")},
				},
			},
		},
	}

	do := ConvertAgentEvaluatorVersionDTO2DO(1, 2, dto)
	if assert.NotNil(t, do) {
		assert.Equal(t, int64(10), do.ID)
		assert.Equal(t, int64(1), do.EvaluatorID)
		assert.Equal(t, int64(2), do.SpaceID)
		assert.Equal(t, "1.0.0", do.Version)
		assert.NotNil(t, do.AgentConfig)
		assert.Equal(t, evaluatordo.AgentType("react_agent"), do.AgentConfig.AgentType)
		assert.Len(t, do.SkillConfigs, 1)
	}

	back := ConvertAgentEvaluatorVersionDO2DTO(do)
	if assert.NotNil(t, back) && assert.NotNil(t, back.EvaluatorContent) && assert.NotNil(t, back.EvaluatorContent.AgentEvaluator) {
		assert.Equal(t, "1.0.0", back.GetVersion())
		assert.Equal(t, "react_agent", back.EvaluatorContent.AgentEvaluator.AgentConfig.GetAgentType())
		assert.Len(t, back.EvaluatorContent.AgentEvaluator.SkillConfigs, 1)
		assert.Equal(t, int64(1), back.EvaluatorContent.AgentEvaluator.SkillConfigs[0].GetSkillID())
	}
}

func TestConvertEvaluatorHTTPInfo_RoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dto  *evaluatordto.EvaluatorHTTPInfo
		do   *evaluatordo.EvaluatorHTTPInfo
	}{
		{name: "nil"},
		{name: "dto", dto: &evaluatordto.EvaluatorHTTPInfo{Method: gptr.Of("post"), Path: gptr.Of("/x")}},
		{name: "do", do: &evaluatordo.EvaluatorHTTPInfo{Method: gptr.Of("get"), Path: gptr.Of("/y")}},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotDO := ConvertEvaluatorHTTPInfoDTO2DO(tc.dto)
			if tc.dto == nil {
				assert.Nil(t, gotDO)
			} else {
				assert.Equal(t, tc.dto.Method, gotDO.Method)
				assert.Equal(t, tc.dto.Path, gotDO.Path)
			}

			gotDTO := ConvertEvaluatorHTTPInfoDO2DTO(tc.do)
			if tc.do == nil {
				assert.Nil(t, gotDTO)
			} else {
				assert.Equal(t, tc.do.Method, gotDTO.Method)
				assert.Equal(t, tc.do.Path, gotDTO.Path)
			}
		})
	}
}

func TestConvertEvaluatorRunConf_RoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dto  *evaluatordto.EvaluatorRunConfig
		do   *evaluatordo.EvaluatorRunConfig
	}{
		{name: "nil"},
		{name: "dto", dto: &evaluatordto.EvaluatorRunConfig{Env: gptr.Of("env")}},
		{name: "do", do: &evaluatordo.EvaluatorRunConfig{Env: gptr.Of("env2")}},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotDO := ConvertEvaluatorRunConfDTO2DO(tc.dto)
			if tc.dto == nil {
				assert.Nil(t, gotDO)
			} else {
				assert.Equal(t, tc.dto.Env, gotDO.Env)
			}

			gotDTO := ConvertEvaluatorRunConfDO2DTO(tc.do)
			if tc.do == nil {
				assert.Nil(t, gotDTO)
			} else {
				assert.Equal(t, tc.do.Env, gotDTO.Env)
			}
		})
	}
}

func TestConvertSkillConfigs_NilAndSkipNil(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []*commondto.SkillConfig
		out  []*evaluatordo.SkillConfig
	}{
		{name: "nil in", in: nil},
		{name: "skip nil elements", in: []*commondto.SkillConfig{{SkillID: gptr.Of(int64(1)), Version: gptr.Of("v1")}, nil}},
		{name: "nil out", out: nil},
		{name: "skip nil out elements", out: []*evaluatordo.SkillConfig{{SkillID: gptr.Of(int64(2)), Version: gptr.Of("v2")}, nil}},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.in != nil || tc.name == "nil in" {
				got := ConvertSkillConfigsDTO2DO(tc.in)
				if tc.in == nil {
					assert.Nil(t, got)
				} else {
					assert.Len(t, got, 1)
					assert.Equal(t, int64(1), gptr.Indirect(got[0].SkillID))
				}
			}
			if tc.out != nil || tc.name == "nil out" {
				got := ConvertSkillConfigsDO2DTO(tc.out)
				if tc.out == nil {
					assert.Nil(t, got)
				} else {
					assert.Len(t, got, 1)
					assert.Equal(t, int64(2), gptr.Indirect(got[0].SkillID))
				}
			}
		})
	}
}

func TestConvertBoxType_DefaultBranch(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "White", convertBoxTypeDO2DTO(evaluatordo.EvaluatorBoxType(999)))
}

func TestNormalizeLanguageType_Empty(t *testing.T) {
	t.Parallel()
	assert.Equal(t, evaluatordo.LanguageType(""), normalizeLanguageType(evaluatordo.LanguageType("")))
}
