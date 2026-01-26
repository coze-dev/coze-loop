// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package conf

import (
	"context"
	"fmt"

	eval_set_dto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_set"
	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
)

//go:generate mockgen -destination=mocks/expt_configer.go -package=mocks . IConfiger
type IOnboardConfiger interface {
	// 根据模板ID获取onboard配置（评测集配置和评估器配置列表）
	GetOnboardConfigByTemplateID(ctx context.Context, templateID string) (*OnboardTemplateConfig, error)
}

// OnboardConfig onboard配置结构
// map key是template_id，value是对应的模板配置
type OnboardConfig map[string]*OnboardTemplateConfig

// NewOnboardConfiger 创建 OnboardConfiger 实例
func NewOnboardConfiger(configFactory conf.IConfigLoaderFactory) IOnboardConfiger {
	loader, err := configFactory.NewConfigLoader(";" + env.PSM())
	if err != nil {
		return nil
	}
	return &OnboardConfiger{
		loader: loader,
	}
}

type OnboardConfiger struct {
	loader conf.IConfigLoader
}

// OnboardTemplateConfig 单个模板的配置
// 配置结构与IDL定义保持一致，从TCC读取后可直接使用
type OnboardTemplateConfig struct {
	EvaluationSet *OnboardEvaluationSetConfig `json:"evaluation_set"`
	Evaluators    []*OnboardEvaluatorConfig   `json:"evaluators"`
	// Template 实验模板配置（用于创建实验模板）
	Template *OnboardExptTemplateConfig `json:"template"`
}

// OnboardEvaluationSetConfig 评测集配置
// 对应 CreateEvaluationSetRequest 的字段（除了workspace_id和session）
// 直接使用IDL生成的DTO类型，从TCC读取后可直接使用
type OnboardEvaluationSetConfig struct {
	Name                string                            `json:"name"`                  // 对应 CreateEvaluationSetRequest.name
	Description         string                            `json:"description"`           // 对应 CreateEvaluationSetRequest.description
	EvaluationSetSchema *eval_set_dto.EvaluationSetSchema `json:"evaluation_set_schema"` // 对应 CreateEvaluationSetRequest.evaluation_set_schema，直接使用IDL类型
	BizCategory         string                            `json:"biz_category"`          // 对应 CreateEvaluationSetRequest.biz_category
	Version             string                            `json:"version"`               // 对应 CreateEvaluationSetVersionRequest.version
	VersionDesc         string                            `json:"version_desc"`          // 对应 CreateEvaluationSetVersionRequest.desc
	Items               []*eval_set_dto.EvaluationSetItem `json:"items"`                 // 对应 BatchCreateEvaluationSetItemsRequest.items，直接使用IDL类型
	SkipInvalidItems    bool                              `json:"skip_invalid_items"`    // 对应 BatchCreateEvaluationSetItemsRequest.skip_invalid_items
	AllowPartialAdd     bool                              `json:"allow_partial_add"`     // 对应 BatchCreateEvaluationSetItemsRequest.allow_partial_add
}

// OnboardEvaluatorConfig 评估器配置
// 直接使用IDL生成的DTO类型，从TCC读取后可直接使用
type OnboardEvaluatorConfig struct {
	Name        string                         `json:"name"`
	Description string                         `json:"description"`
	Type        evaluatordto.EvaluatorType     `json:"type"` // evaluatordto.EvaluatorType
	Version     string                         `json:"version"`
	Content     *evaluatordto.EvaluatorContent `json:"content"` // *evaluatordto.EvaluatorContent
}

// GetOnboardConfigByTemplateID 根据模板ID获取onboard配置
func (c *OnboardConfiger) GetOnboardConfigByTemplateID(ctx context.Context, templateID string) (*OnboardTemplateConfig, error) {
	const key = "onboard_config"

	// 先读取整个配置map
	var onboardConfig OnboardConfig
	if err := c.loader.UnmarshalKey(ctx, key, &onboardConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal onboard config: %w", err)
	}

	// 根据templateID获取对应的配置
	templateConfig, ok := onboardConfig[templateID]
	if !ok {
		return nil, fmt.Errorf("onboard config not found for template_id: %s", templateID)
	}

	return templateConfig, nil
}

// OnboardExptTemplateConfig 实验模板配置
type OnboardExptTemplateConfig struct {
	// Name 模板名称
	Name string `json:"name"`
	// Description 模板描述
	Description string `json:"description"`
	// ExptType 实验类型
	ExptType int32 `json:"expt_type"`
	// FieldMappingConfig 字段映射配置
	FieldMappingConfig *OnboardFieldMappingConfig `json:"field_mapping_config"`
	// ItemConcurNum 评测项并发数
	ItemConcurNum *int32 `json:"item_concur_num"`
	// EvaluatorsConcurNum 评估器并发数
	EvaluatorsConcurNum *int32 `json:"evaluators_concur_num"`
	// EvaluatorScoreWeights 评估器评分权重（key: evaluator_id#version）
	EvaluatorScoreWeights map[string]float64 `json:"evaluator_score_weights"`
}

// OnboardFieldMappingConfig 字段映射配置
type OnboardFieldMappingConfig struct {
	// TargetFieldMapping 目标字段映射
	TargetFieldMapping *OnboardTargetFieldMapping `json:"target_field_mapping"`
	// EvaluatorFieldMapping 评估器字段映射
	EvaluatorFieldMapping []*OnboardEvaluatorFieldMapping `json:"evaluator_field_mapping"`
	// TargetRuntimeParam 目标运行时参数
	TargetRuntimeParam *OnboardRuntimeParam `json:"target_runtime_param"`
}

// OnboardTargetFieldMapping 目标字段映射
type OnboardTargetFieldMapping struct {
	// FromEvalSet 从评测集字段映射
	FromEvalSet []*OnboardFieldMapping `json:"from_eval_set"`
	// FromTarget 从目标字段映射
	FromTarget []*OnboardFieldMapping `json:"from_target"`
}

// OnboardEvaluatorFieldMapping 评估器字段映射
type OnboardEvaluatorFieldMapping struct {
	// EvaluatorID 评估器ID
	EvaluatorID int64 `json:"evaluator_id"`
	// Version 版本号
	Version string `json:"version"`
	// FromEvalSet 从评测集字段映射
	FromEvalSet []*OnboardFieldMapping `json:"from_eval_set"`
	// FromTarget 从目标字段映射
	FromTarget []*OnboardFieldMapping `json:"from_target"`
}

// OnboardFieldMapping 字段映射
type OnboardFieldMapping struct {
	// FieldName 字段名
	FieldName string `json:"field_name"`
	// FromFieldName 来源字段名
	FromFieldName string `json:"from_field_name"`
	// ConstValue 常量值
	ConstValue string `json:"const_value"`
}

// OnboardRuntimeParam 运行时参数
type OnboardRuntimeParam struct {
	// JSONValue JSON值
	JSONValue string `json:"json_value"`
}
