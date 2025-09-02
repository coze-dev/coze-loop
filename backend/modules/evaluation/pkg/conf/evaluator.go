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
	GetCodeEvaluatorTemplateConf(ctx context.Context) (etf map[string]*evaluatordto.EvaluatorContent)
}

func NewEvaluatorConfiger(configFactory conf.IConfigLoaderFactory) IConfiger {
	loader, err := configFactory.NewConfigLoader("evaluation.yaml")
	if err != nil {
		return nil
	}
	return &configer{
		loader: loader,
	}
}

func (c *configer) GetEvaluatorTemplateConf(ctx context.Context) (etf map[string]map[string]*evaluatordto.EvaluatorContent) {
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

func (c *configer) GetEvaluatorToolConf(ctx context.Context) (etf map[string]*evaluatordto.Tool) {
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

func (c *configer) GetRateLimiterConf(ctx context.Context) (rlc []limiter.Rule) {
	const key = "rate_limiter_conf"
	return lo.Ternary(c.loader.UnmarshalKey(ctx, key, &rlc) == nil, rlc, DefaultRateLimiterConf())
}

func DefaultRateLimiterConf() []limiter.Rule {
	return make([]limiter.Rule, 0)
}

func (c *configer) GetEvaluatorToolMapping(ctx context.Context) (etf map[string]string) {
	const key = "evaluator_tool_mapping"
	return lo.Ternary(c.loader.UnmarshalKey(ctx, key, &etf) == nil, etf, DefaultEvaluatorToolMapping())
}

func DefaultEvaluatorToolMapping() map[string]string {
	return make(map[string]string)
}

func (c *configer) GetEvaluatorPromptSuffix(ctx context.Context) (suffix map[string]string) {
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

func (c *configer) GetEvaluatorPromptSuffixMapping(ctx context.Context) (suffix map[string]string) {
	const key = "evaluator_prompt_mapping"
	return lo.Ternary(c.loader.UnmarshalKey(ctx, key, &suffix) == nil, suffix, DefaultEvaluatorPromptMapping())
}

func DefaultEvaluatorPromptMapping() map[string]string {
	return make(map[string]string)
}

func (c *configer) GetCodeEvaluatorTemplateConf(ctx context.Context) (etf map[string]*evaluatordto.EvaluatorContent) {
	// 获取原始配置
	allTemplates := c.GetEvaluatorTemplateConf(ctx)
	codeTemplates := allTemplates["code"]
	
	if codeTemplates == nil {
		return make(map[string]*evaluatordto.EvaluatorContent)
	}
	
	// 转换为新的数据结构：template_key + "_" + language_type 作为key
	result := make(map[string]*evaluatordto.EvaluatorContent)
	for _, template := range codeTemplates {
		if template.GetCodeEvaluator() != nil {
			templateKey := template.GetCodeEvaluator().GetCodeTemplateKey()
			languageType := template.GetCodeEvaluator().GetLanguageType()
			if templateKey != "" && languageType != "" {
				combinedKey := templateKey + "_" + languageType
				result[combinedKey] = template
			}
		}
	}
	
	return result
}