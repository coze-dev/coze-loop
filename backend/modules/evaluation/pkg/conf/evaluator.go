// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package conf

import (
	"context"
	"fmt"

	"github.com/samber/lo"

	"github.com/coze-dev/coze-loop/backend/infra/limiter"
	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	"github.com/coze-dev/coze-loop/backend/pkg/contexts"
)

//go:generate mockgen -destination=mocks/evaluator_configer.go -package=mocks . IConfiger
type IConfiger interface {
	GetEvaluatorTemplateConf(ctx context.Context) (etf map[string]map[string]*evaluatordto.EvaluatorContent)
	GetEvaluatorToolConf(ctx context.Context) (etf map[string]*evaluatordto.Tool) // tool_key -> tool
	GetRateLimiterConf(ctx context.Context) (rlc []limiter.Rule)
	GetEvaluatorToolMapping(ctx context.Context) (etf map[string]string)            // prompt_template_key -> tool_key
	GetEvaluatorPromptSuffix(ctx context.Context) (suffix map[string]string)        // suffix_key -> suffix
	GetEvaluatorPromptSuffixMapping(ctx context.Context) (suffix map[string]string) // model_id -> suffix_key
	// 新增方法：专门为Code类型模板提供配置
	GetCodeEvaluatorTemplateConf(ctx context.Context) (etf map[string]map[string]*evaluatordto.EvaluatorContent)
	// 新增方法：专门为Custom类型模板提供配置
	GetCustomCodeEvaluatorTemplateConf(ctx context.Context) (etf map[string]map[string]*evaluatordto.EvaluatorContent)
}

func NewEvaluatorConfiger(configFactory conf.IConfigLoaderFactory) IConfiger {
	loader, err := configFactory.NewConfigLoader("evaluation.yaml")
	if err != nil {
		return nil
	}
	return &evaluatorConfiger{
		loader: loader,
	}
}
func (c *evaluatorConfiger) GetEvaluatorTemplateConf(ctx context.Context) (etf map[string]map[string]*evaluatordto.EvaluatorContent) {
	const key = "evaluator_template_conf"

	if locale := contexts.CtxLocale(ctx); c.loader.UnmarshalKey(ctx, fmt.Sprintf("%s_%s", key, locale), &etf) == nil && len(etf) > 0 {
		return etf
	}
	if c.loader.UnmarshalKey(ctx, key, &etf) == nil && len(etf) > 0 {
		return etf
	}
	return DefaultEvaluatorTemplateConf()
}

func DefaultEvaluatorTemplateConf() map[string]map[string]*evaluatordto.EvaluatorContent {
	return map[string]map[string]*evaluatordto.EvaluatorContent{}
}
func (c *evaluatorConfiger) GetEvaluatorToolConf(ctx context.Context) (etf map[string]*evaluatordto.Tool) {
	const key = "evaluator_tool_conf"

	if locale := contexts.CtxLocale(ctx); c.loader.UnmarshalKey(ctx, fmt.Sprintf("%s_%s", key, locale), &etf) == nil && len(etf) > 0 {
		return etf
	}
	if c.loader.UnmarshalKey(ctx, key, &etf) == nil && len(etf) > 0 {
		return etf
	}
	return DefaultEvaluatorToolConf()
}

func DefaultEvaluatorToolConf() map[string]*evaluatordto.Tool {
	return make(map[string]*evaluatordto.Tool, 0)
}
func (c *evaluatorConfiger) GetRateLimiterConf(ctx context.Context) (rlc []limiter.Rule) {
	const key = "rate_limiter_conf"
	return lo.Ternary(c.loader.UnmarshalKey(ctx, key, &rlc) == nil, rlc, DefaultRateLimiterConf())
}

func DefaultRateLimiterConf() []limiter.Rule {
	return make([]limiter.Rule, 0)
}
func (c *evaluatorConfiger) GetEvaluatorToolMapping(ctx context.Context) (etf map[string]string) {
	const key = "evaluator_tool_mapping"
	return lo.Ternary(c.loader.UnmarshalKey(ctx, key, &etf) == nil, etf, DefaultEvaluatorToolMapping())
}

func DefaultEvaluatorToolMapping() map[string]string {
	return make(map[string]string)
}
func (c *evaluatorConfiger) GetEvaluatorPromptSuffix(ctx context.Context) (suffix map[string]string) {
	const key = "evaluator_prompt_suffix"

	if locale := contexts.CtxLocale(ctx); c.loader.UnmarshalKey(ctx, fmt.Sprintf("%s_%s", key, locale), &suffix) == nil && len(suffix) > 0 {
		return suffix
	}
	if c.loader.UnmarshalKey(ctx, key, &suffix) == nil && len(suffix) > 0 {
		return suffix
	}
	return DefaultEvaluatorPromptSuffix()
}

func DefaultEvaluatorPromptSuffix() map[string]string {
	return make(map[string]string)
}
func (c *evaluatorConfiger) GetEvaluatorPromptSuffixMapping(ctx context.Context) (suffix map[string]string) {
	const key = "evaluator_prompt_mapping"
	return lo.Ternary(c.loader.UnmarshalKey(ctx, key, &suffix) == nil, suffix, DefaultEvaluatorPromptMapping())
}

func DefaultEvaluatorPromptMapping() map[string]string {
	return make(map[string]string)
}
func (c *evaluatorConfiger) GetCodeEvaluatorTemplateConf(ctx context.Context) (etf map[string]map[string]*evaluatordto.EvaluatorContent) {
	const key = "code_evaluator_template_conf"
	if c.loader.UnmarshalKey(ctx, key, &etf) == nil && len(etf) > 0 {
		return etf
	}
	return DefaultCodeEvaluatorTemplateConf()
}

func DefaultCodeEvaluatorTemplateConf() map[string]map[string]*evaluatordto.EvaluatorContent {
	return map[string]map[string]*evaluatordto.EvaluatorContent{}
}

func (c *evaluatorConfiger) GetCustomCodeEvaluatorTemplateConf(ctx context.Context) (etf map[string]map[string]*evaluatordto.EvaluatorContent) {
	const key = "custom_code_evaluator_template_conf"
	if c.loader.UnmarshalKey(ctx, key, &etf) == nil && len(etf) > 0 {
		return etf
	}
	return DefaultCustomCodeEvaluatorTemplateConf()
}

func DefaultCustomCodeEvaluatorTemplateConf() map[string]map[string]*evaluatordto.EvaluatorContent {
	return map[string]map[string]*evaluatordto.EvaluatorContent{}
}

type evaluatorConfiger struct {
	loader conf.IConfigLoader
}