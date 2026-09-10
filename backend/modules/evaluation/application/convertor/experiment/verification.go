// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"github.com/bytedance/gg/gptr"
	expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	openapiExperiment "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func VerificationConfigOpenAPI2Domain(config *openapiExperiment.VerificationConfig) *expt.VerificationConfig {
	if config == nil {
		return nil
	}
	return &expt.VerificationConfig{Mode: gptr.Of(config.GetMode())}
}

func verificationConfigDomain2OpenAPI(config *expt.VerificationConfig) *openapiExperiment.VerificationConfig {
	if config == nil {
		return nil
	}
	return &openapiExperiment.VerificationConfig{Mode: gptr.Of(config.GetMode())}
}

func verificationConfigDTO2DO(config *expt.VerificationConfig) *entity.VerificationConfig {
	if config == nil {
		return nil
	}
	return &entity.VerificationConfig{Mode: entity.VerificationMode(config.GetMode())}
}

func verificationConfigDO2DTO(config *entity.VerificationConfig) *expt.VerificationConfig {
	if config == nil {
		return nil
	}
	return &expt.VerificationConfig{Mode: gptr.Of(expt.VerificationMode(config.Mode))}
}

func templateVerificationConfig(template *entity.ExptTemplate) (*entity.VerificationConfig, string, error) {
	if template == nil || template.TemplateConf == nil {
		return nil, "", nil
	}
	config := &entity.EvaluationConfiguration{
		VerificationConfig: template.TemplateConf.VerificationConfig,
		ConnectorConf:      template.TemplateConf.ConnectorConf,
	}
	return config.ResolveVerificationConfig(template.Target)
}
