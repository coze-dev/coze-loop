// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	domaindoEvalTarget "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	openapiEvalTarget "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/eval_target"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// TestOpenAPICreateEvalTargetParamDTO2Domain_SandboxAgent 验证 SandboxAgent 分支:
// param.SandboxAgent 非空时, result.SandboxAgent 填充并保留所有字段
func TestOpenAPICreateEvalTargetParamDTO2Domain_SandboxAgent(t *testing.T) {
	t.Run("SandboxAgent 为 nil 时 result.SandboxAgent 也为 nil", func(t *testing.T) {
		param := &openapi.SubmitExperimentEvalTargetParam{}
		res, err := OpenAPICreateEvalTargetParamDTO2Domain(param)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Nil(t, res.SandboxAgent)
	})

	t.Run("SandboxAgent 非 nil 时 result.SandboxAgent 字段映射正确", func(t *testing.T) {
		typeVal := openapiEvalTarget.SandboxAgentType("single_run_cli")
		param := &openapi.SubmitExperimentEvalTargetParam{
			SandboxAgent: &openapiEvalTarget.SandboxAgent{
				Name:          gptr.Of("agent1"),
				Type:          &typeVal,
				ModelName:     gptr.Of("doubao"),
				AgentSetupCmd: gptr.Of("setup.sh"),
				AgentRunCmd:   gptr.Of("run.sh"),
				Envs: []*openapiEvalTarget.SandboxEnvVar{
					{Key: gptr.Of("K1"), Value: gptr.Of("V1")},
					nil, // nil env skip
				},
			},
		}
		res, err := OpenAPICreateEvalTargetParamDTO2Domain(param)
		assert.NoError(t, err)
		assert.NotNil(t, res.SandboxAgent)
		assert.Equal(t, gptr.Of("agent1"), res.SandboxAgent.Name)
		assert.Equal(t, gptr.Of("doubao"), res.SandboxAgent.ModelName)
		assert.Equal(t, gptr.Of("setup.sh"), res.SandboxAgent.AgentSetupCmd)
		assert.Equal(t, gptr.Of("run.sh"), res.SandboxAgent.AgentRunCmd)
		assert.Equal(t, 1, len(res.SandboxAgent.Envs))
		assert.Equal(t, gptr.Of("K1"), res.SandboxAgent.Envs[0].Key)
		assert.NotNil(t, res.SandboxAgent.Type)
		assert.Equal(t, domaindoEvalTarget.SandboxAgentType("single_run_cli"), *res.SandboxAgent.Type)
	})
}

// TestOpenAPISandboxAgentDTO2DO 验证 DTO → entity 的 SandboxAgent 转换。
// nil 输入返回 nil, nil envs 元素被过滤, 字段透传。
func TestOpenAPISandboxAgentDTO2DO(t *testing.T) {
	t.Run("nil 返回 nil", func(t *testing.T) {
		assert.Nil(t, OpenAPISandboxAgentDTO2DO(nil))
	})

	t.Run("字段完整 + nil env 被过滤", func(t *testing.T) {
		typ := openapiEvalTarget.SandboxAgentType("single_run_cli")
		dto := &openapiEvalTarget.SandboxAgent{
			Name:          gptr.Of("a"),
			Type:          &typ,
			ModelName:     gptr.Of("m"),
			AgentSetupCmd: gptr.Of("setup"),
			AgentRunCmd:   gptr.Of("run"),
			Envs: []*openapiEvalTarget.SandboxEnvVar{
				{Key: gptr.Of("K"), Value: gptr.Of("V")},
				nil,
				{Key: gptr.Of("K2"), Value: gptr.Of("V2")},
			},
		}
		got := OpenAPISandboxAgentDTO2DO(dto)
		assert.NotNil(t, got)
		assert.Equal(t, "a", got.Name)
		assert.Equal(t, entity.SandboxAgentType("single_run_cli"), got.Type)
		assert.Equal(t, "m", got.ModelName)
		assert.Equal(t, "setup", got.AgentSetupCmd)
		assert.Equal(t, "run", got.AgentRunCmd)
		assert.Len(t, got.Envs, 2)
		assert.Equal(t, "K", got.Envs[0].Key)
		assert.Equal(t, "V2", got.Envs[1].Value)
	})

	t.Run("Envs 为空切片返回空 entity envs", func(t *testing.T) {
		dto := &openapiEvalTarget.SandboxAgent{Name: gptr.Of("x")}
		got := OpenAPISandboxAgentDTO2DO(dto)
		assert.NotNil(t, got)
		assert.Equal(t, "x", got.Name)
		assert.Equal(t, 0, len(got.Envs))
	})
}

// TestOpenAPISandboxAgentDO2DTO_RoundTrip 校验 DO → DTO → DO 字段一致 (除 Type/CustomFieldSchemas)
func TestOpenAPISandboxAgentDO2DTO_RoundTrip(t *testing.T) {
	do := &entity.SandboxAgent{
		Name:          "demo",
		Type:          entity.SandboxAgentTypeSingleRunCLI,
		ModelName:     "doubao",
		AgentSetupCmd: "setup",
		AgentRunCmd:   "run",
		Envs:          []*entity.SandboxEnvVar{{Key: "K", Value: "V"}},
	}
	dto := OpenAPISandboxAgentDO2DTO(do)
	assert.NotNil(t, dto)
	roundTrip := OpenAPISandboxAgentDTO2DO(dto)
	assert.Equal(t, do.Name, roundTrip.Name)
	assert.Equal(t, do.ModelName, roundTrip.ModelName)
	assert.Equal(t, do.AgentSetupCmd, roundTrip.AgentSetupCmd)
	assert.Equal(t, do.AgentRunCmd, roundTrip.AgentRunCmd)
	assert.Equal(t, len(do.Envs), len(roundTrip.Envs))
	assert.Equal(t, do.Envs[0].Key, roundTrip.Envs[0].Key)
	assert.Equal(t, do.Envs[0].Value, roundTrip.Envs[0].Value)
}

// TestOpenAPICustomRPCServerDTO2DO 验证 DTO → entity 转换字段映射
func TestOpenAPICustomRPCServerDTO2DO(t *testing.T) {
	t.Run("nil 输入返回 nil", func(t *testing.T) {
		assert.Nil(t, OpenAPICustomRPCServerDTO2DO(nil))
	})

	t.Run("完整字段映射", func(t *testing.T) {
		invokeMethod := openapiEvalTarget.HTTPMethod("post")
		asyncMethod := openapiEvalTarget.HTTPMethod("post")
		searchMethod := openapiEvalTarget.HTTPMethod("get")
		dto := &openapiEvalTarget.CustomRPCServer{
			ID:             gptr.Of(int64(11)),
			Name:           gptr.Of("rpc-name"),
			Description:    gptr.Of("desc"),
			ServerName:     gptr.Of("server"),
			AccessProtocol: gptr.Of(openapiEvalTarget.AccessProtocol("rpc")),
			Regions:        []openapiEvalTarget.Region{"cn"},
			Cluster:        gptr.Of("default"),
			InvokeHTTPInfo: &openapiEvalTarget.HTTPInfo{
				Method: &invokeMethod, Path: gptr.Of("/invoke"),
			},
			AsyncInvokeHTTPInfo: &openapiEvalTarget.HTTPInfo{
				Method: &asyncMethod, Path: gptr.Of("/async"),
			},
			NeedSearchTarget: gptr.Of(true),
			SearchHTTPInfo: &openapiEvalTarget.HTTPInfo{
				Method: &searchMethod, Path: gptr.Of("/search"),
			},
			CustomEvalTarget: &openapiEvalTarget.CustomEvalTarget{
				ID:        gptr.Of("ce-1"),
				Name:      gptr.Of("ce-name"),
				AvatarURL: gptr.Of("http://avatar"),
				Ext:       map[string]string{"k": "v"},
			},
			IsAsync:      gptr.Of(true),
			ExecRegion:   gptr.Of(openapiEvalTarget.Region("cn")),
			ExecEnv:      gptr.Of("prod"),
			Timeout:      gptr.Of(int64(500)),
			AsyncTimeout: gptr.Of(int64(60000)),
			Ext:          map[string]string{"e": "f"},
		}
		got := OpenAPICustomRPCServerDTO2DO(dto)
		assert.NotNil(t, got)
		assert.Equal(t, int64(11), got.ID)
		assert.Equal(t, "rpc-name", got.Name)
		assert.Equal(t, "desc", got.Description)
		assert.Equal(t, "server", got.ServerName)
		assert.Equal(t, entity.AccessProtocol("rpc"), got.AccessProtocol)
		assert.Equal(t, []entity.Region{"cn"}, got.Regions)
		assert.Equal(t, "default", got.Cluster)
		assert.NotNil(t, got.InvokeHTTPInfo)
		assert.Equal(t, "post", got.InvokeHTTPInfo.Method)
		assert.Equal(t, "/invoke", got.InvokeHTTPInfo.Path)
		assert.NotNil(t, got.AsyncInvokeHTTPInfo)
		assert.Equal(t, "/async", got.AsyncInvokeHTTPInfo.Path)
		assert.Equal(t, gptr.Of(true), got.NeedSearchTarget)
		assert.NotNil(t, got.SearchHTTPInfo)
		assert.Equal(t, "/search", got.SearchHTTPInfo.Path)
		assert.NotNil(t, got.CustomEvalTarget)
		assert.Equal(t, gptr.Of("ce-1"), got.CustomEvalTarget.ID)
		assert.Equal(t, gptr.Of(true), got.IsAsync)
		assert.Equal(t, entity.Region("cn"), got.ExecRegion)
		assert.Equal(t, gptr.Of("prod"), got.ExecEnv)
		assert.Equal(t, gptr.Of(int64(500)), got.Timeout)
		assert.Equal(t, gptr.Of(int64(60000)), got.AsyncTimeout)
		assert.Equal(t, map[string]string{"e": "f"}, got.Ext)
	})

	t.Run("子结构 nil 时 entity 对应字段也 nil", func(t *testing.T) {
		dto := &openapiEvalTarget.CustomRPCServer{ID: gptr.Of(int64(1))}
		got := OpenAPICustomRPCServerDTO2DO(dto)
		assert.NotNil(t, got)
		assert.Nil(t, got.InvokeHTTPInfo)
		assert.Nil(t, got.AsyncInvokeHTTPInfo)
		assert.Nil(t, got.SearchHTTPInfo)
		assert.Nil(t, got.CustomEvalTarget)
	})
}

// TestOpenAPICreateEvalTargetParamDTO2DomainV2_SandboxAgent 验证 V2 路径同样支持 SandboxAgent
func TestOpenAPICreateEvalTargetParamDTO2DomainV2_SandboxAgent(t *testing.T) {
	param := &openapi.SubmitExperimentEvalTargetParam{
		SandboxAgent: &openapiEvalTarget.SandboxAgent{
			Name:      gptr.Of("a"),
			ModelName: gptr.Of("m"),
		},
	}
	res := OpenAPICreateEvalTargetParamDTO2DomainV2(param)
	assert.NotNil(t, res)
	assert.NotNil(t, res.SandboxAgent)
	assert.Equal(t, "a", res.SandboxAgent.Name)
	assert.Equal(t, "m", res.SandboxAgent.ModelName)
}

// TestOpenAPISandboxAgentDTO2Domain_SandboxCountMode 验证 openapi DTO -> domain DTO 时
// sandbox_count_mode 的透传：nil 保持 nil；非 nil 拷贝到新指针，避免与入参共享内存。
func TestOpenAPISandboxAgentDTO2Domain_SandboxCountMode(t *testing.T) {
	t.Run("nil mode -> nil", func(t *testing.T) {
		param := &openapi.SubmitExperimentEvalTargetParam{
			SandboxAgent: &openapiEvalTarget.SandboxAgent{Name: gptr.Of("a")},
		}
		res, err := OpenAPICreateEvalTargetParamDTO2Domain(param)
		assert.NoError(t, err)
		assert.NotNil(t, res.SandboxAgent)
		assert.Nil(t, res.SandboxAgent.SandboxCountMode)
	})

	t.Run("dual mode copied to a fresh pointer", func(t *testing.T) {
		mode := openapiEvalTarget.SandboxCountMode(openapiEvalTarget.SandboxCountModeDual)
		param := &openapi.SubmitExperimentEvalTargetParam{
			SandboxAgent: &openapiEvalTarget.SandboxAgent{
				Name:             gptr.Of("a"),
				SandboxCountMode: &mode,
			},
		}
		res, err := OpenAPICreateEvalTargetParamDTO2Domain(param)
		assert.NoError(t, err)
		assert.NotNil(t, res.SandboxAgent)
		if assert.NotNil(t, res.SandboxAgent.SandboxCountMode) {
			assert.Equal(t, domaindoEvalTarget.SandboxCountModeDual, *res.SandboxAgent.SandboxCountMode)
		}
	})
}

// TestOpenAPISandboxAgentDO2DTO_SandboxCountMode 验证 entity -> DTO 转换时对 SandboxCountMode 的空值保留：
// 空串保持 DTO nil（老 wire 契约，IsSet=false）；非空转成新指针。
func TestOpenAPISandboxAgentDO2DTO_SandboxCountMode(t *testing.T) {
	t.Run("empty mode preserves nil pointer on DTO", func(t *testing.T) {
		do := &entity.SandboxAgent{Name: "a"}
		got := OpenAPISandboxAgentDO2DTO(do)
		assert.NotNil(t, got)
		assert.Nil(t, got.SandboxCountMode)
	})

	t.Run("dual mode round-tripped as pointer", func(t *testing.T) {
		do := &entity.SandboxAgent{Name: "a", SandboxCountMode: entity.SandboxCountModeDual}
		got := OpenAPISandboxAgentDO2DTO(do)
		if assert.NotNil(t, got) && assert.NotNil(t, got.SandboxCountMode) {
			assert.Equal(t, openapiEvalTarget.SandboxCountModeDual, *got.SandboxCountMode)
		}
	})

	t.Run("DTO -> DO passes empty mode through as empty string", func(t *testing.T) {
		got := OpenAPISandboxAgentDTO2DO(&openapiEvalTarget.SandboxAgent{Name: gptr.Of("a")})
		assert.NotNil(t, got)
		assert.Equal(t, entity.SandboxCountMode(""), got.SandboxCountMode)
	})

	t.Run("DTO -> DO passes dual mode through", func(t *testing.T) {
		mode := openapiEvalTarget.SandboxCountMode(openapiEvalTarget.SandboxCountModeDual)
		got := OpenAPISandboxAgentDTO2DO(&openapiEvalTarget.SandboxAgent{
			Name:             gptr.Of("a"),
			SandboxCountMode: &mode,
		})
		assert.NotNil(t, got)
		assert.Equal(t, entity.SandboxCountModeDual, got.SandboxCountMode)
	})
}
