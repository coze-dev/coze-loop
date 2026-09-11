// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	common "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	domainExpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	openapiExperiment "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"
	expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	openapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

func TestVerificationConfigRoundTrip(t *testing.T) {
	config := &domainExpt.VerificationConfig{Mode: gptr.Of(domainExpt.VerificationMode("oracle_only"))}
	do := verificationConfigDTO2DO(config)
	require.Equal(t, entity.VerificationModeOracleOnly, do.Mode)
	require.Equal(t, config, verificationConfigDO2DTO(do))
	require.Equal(t, config, VerificationConfigOpenAPI2Domain(verificationConfigDomain2OpenAPI(config)))
	require.Nil(t, verificationConfigDTO2DO(nil))
	require.Nil(t, VerificationConfigOpenAPI2Domain(nil))

	request := &expt.CreateExperimentRequest{VerificationConfig: config, TargetRuntimeParam: &common.RuntimeParam{JSONValue: gptr.Of(`{"binary_version":"pinned"}`)}}
	conf, err := NewEvalConfConvert().ConvertToEntity(request, nil)
	require.NoError(t, err)
	require.Equal(t, do, conf.VerificationConfig)
}

func TestVerificationReadAndTemplatePropagation(t *testing.T) {
	config := &entity.VerificationConfig{Mode: entity.VerificationModeF2P}
	target := &entity.EvalTarget{EvalTargetType: entity.EvalTargetTypeSandboxAgent, EvalTargetVersion: &entity.EvalTargetVersion{SandboxAgent: &entity.SandboxAgent{Name: "Verify", SandboxCountMode: entity.SandboxCountModeDual}}}
	connector := entity.Connector{TargetConf: &entity.TargetConf{IngressConf: &entity.TargetIngressConf{CustomConf: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{
		FieldName: consts.FieldAdapterBuiltinFieldNameRuntimeParam,
		Value:     `{"verification":{"mode":"f2p"},"binary_version":"pinned"}`,
	}}}}}}
	experiment := &entity.Experiment{EvalConf: &entity.EvaluationConfiguration{ConnectorConf: connector}, Target: target}
	dto := ToExptDTO(experiment)
	require.Equal(t, verificationConfigDO2DTO(config), dto.VerificationConfig)
	require.JSONEq(t, `{"binary_version":"pinned"}`, dto.TargetRuntimeParam.GetJSONValue())
	require.Equal(t, "f2p", DomainExperimentDTO2OpenAPI(dto).VerificationConfig.GetMode())
	template := &entity.ExptTemplate{Meta: &entity.ExptTemplateMeta{ID: 1}, TemplateConf: &entity.ExptTemplateConfiguration{VerificationConfig: config, ConnectorConf: connector}, Target: target}
	template.FieldMappingConfig = &entity.ExptFieldMapping{TargetRuntimeParam: &entity.RuntimeParam{
		JSONValue: gptr.Of(connector.TargetConf.IngressConf.CustomConf.FieldConfs[0].Value),
	}}
	templateDTO := ToExptTemplateDTO(template)
	require.Equal(t, dto.VerificationConfig, templateDTO.VerificationConfig)
	require.NotNil(t, templateDTO.FieldMappingConfig)
	require.NotNil(t, templateDTO.FieldMappingConfig.TargetRuntimeParam)
	require.JSONEq(t, `{"binary_version":"pinned"}`, templateDTO.FieldMappingConfig.TargetRuntimeParam.GetJSONValue())
	require.JSONEq(t, `{"verification":{"mode":"f2p"},"binary_version":"pinned"}`, *template.FieldMappingConfig.TargetRuntimeParam.JSONValue)
	require.Equal(t, dto.VerificationConfig, TemplateToSubmitExperimentRequest(template, "copy", 1).VerificationConfig)
}

func TestTemplateVerificationUpdateOnly(t *testing.T) {
	for _, mode := range []domainExpt.VerificationMode{"nop_only", "oracle_only", "f2p"} {
		t.Run(mode, func(t *testing.T) {
			cfg := &domainExpt.VerificationConfig{Mode: gptr.Of(mode)}
			param, err := ConvertUpdateExptTemplateReq(&expt.UpdateExperimentTemplateRequest{VerificationConfig: cfg})
			require.NoError(t, err)
			require.NotNil(t, param.TemplateConf)
			require.Equal(t, verificationConfigDTO2DO(cfg), param.TemplateConf.VerificationConfig)
		})
	}
	param, err := ConvertUpdateExptTemplateReq(&expt.UpdateExperimentTemplateRequest{})
	require.NoError(t, err)
	require.Nil(t, param.TemplateConf, "an omitted verification field is not an explicit reset")
}

func TestOpenAPITemplateVerificationInput(t *testing.T) {
	cfg := &openapiExperiment.VerificationConfig{Mode: gptr.Of(openapiExperiment.VerificationMode("nop_only"))}
	created, err := OpenAPICreateExptTemplateReq2Domain(&openapi.CreateExptTemplateOApiRequest{VerificationConfig: cfg})
	require.NoError(t, err)
	require.Equal(t, entity.VerificationModeNopOnly, created.TemplateConf.VerificationConfig.Mode)
	updated, err := OpenAPIUpdateExptTemplateReq2Domain(&openapi.UpdateExptTemplateOApiRequest{VerificationConfig: cfg})
	require.NoError(t, err)
	require.Equal(t, created.TemplateConf.VerificationConfig, updated.TemplateConf.VerificationConfig)
}
