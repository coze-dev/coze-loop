// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package benefit

import (
	"context"
)

// NoopBenefitServiceImpl 是 IBenefitService 接口的模拟实现结构体
type NoopBenefitServiceImpl struct{}

func NewNoopBenefitService() IBenefitService {
	return &NoopBenefitServiceImpl{}
}

func (n NoopBenefitServiceImpl) CheckTraceBenefit(ctx context.Context, param *CheckTraceBenefitParams) (result *CheckTraceBenefitResult, err error) {
	return &CheckTraceBenefitResult{
		AccountAvailable: true,
		IsEnough:         true,
		StorageDuration:  365,
		WhichIsEnough:    -1,
	}, nil
}

func (n NoopBenefitServiceImpl) DeductTraceBenefit(ctx context.Context, param *DeductTraceBenefitParams) (err error) {
	return nil
}

func (n NoopBenefitServiceImpl) ReplenishExtraTraceBenefit(ctx context.Context, param *ReplenishExtraTraceBenefitParams) (err error) {
	return nil
}

func (n NoopBenefitServiceImpl) CheckPromptBenefit(ctx context.Context, param *CheckPromptBenefitParams) (result *CheckPromptBenefitResult, err error) {
	return &CheckPromptBenefitResult{}, nil
}

func (n NoopBenefitServiceImpl) CheckEvaluatorBenefit(ctx context.Context, param *CheckEvaluatorBenefitParams) (result *CheckEvaluatorBenefitResult, err error) {
	return &CheckEvaluatorBenefitResult{}, nil
}

func (n NoopBenefitServiceImpl) CheckAndDeductEvalBenefit(ctx context.Context, param *CheckAndDeductEvalBenefitParams) (result *CheckAndDeductEvalBenefitResult, err error) {
	return &CheckAndDeductEvalBenefitResult{}, nil
}

func (n NoopBenefitServiceImpl) BatchCheckEnableTypeBenefit(ctx context.Context, param *BatchCheckEnableTypeBenefitParams) (result *BatchCheckEnableTypeBenefitResult, err error) {
	// 为所有请求的权益类型返回 true，表示在开源版本中所有权益都可用
	results := make(map[string]bool)
	for _, benefitType := range param.EnableTypeBenefits {
		results[benefitType] = true
	}
	return &BatchCheckEnableTypeBenefitResult{
		Results: results,
	}, nil
}
