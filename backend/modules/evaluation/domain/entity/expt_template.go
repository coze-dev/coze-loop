// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"fmt"

	"github.com/mitchellh/mapstructure"

	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// ExptTemplate 实验模板实体
// 用于预先配置评测对象、评测集与评估器，并在创建实验时复用
type ExptTemplate struct {
	ID          int64
	SpaceID     int64
	CreatedBy   string
	Name        string
	Description string

	EvalSetVersionID    int64
	EvalSetID           int64
	TargetType          EvalTargetType
	TargetVersionID     int64
	TargetID            int64
	EvaluatorVersionRef []*ExptTemplateEvaluatorVersionRef
	TemplateConf        *ExptTemplateConfiguration

	Target     *EvalTarget
	EvalSet    *EvaluationSet
	Evaluators []*Evaluator

	ExptType ExptType
}

// ExptTemplateEvaluatorVersionRef 实验模板评估器版本引用
type ExptTemplateEvaluatorVersionRef struct {
	EvaluatorID        int64
	EvaluatorVersionID int64
}

func (e *ExptTemplateEvaluatorVersionRef) String() string {
	return fmt.Sprintf("evaluator_id= %v, evaluator_version_id= %v", e.EvaluatorID, e.EvaluatorVersionID)
}

// ExptTemplateConfiguration 实验模板配置
// 包含评估器列表、字段映射、加权配置、默认并发及调度等
// 该配置会序列化为JSON存储在数据库的template_conf字段中
type ExptTemplateConfiguration struct {
	// 字段映射 & 运行时参数（使用与EvaluationConfiguration类似的结构）
	ConnectorConf Connector
	ItemConcurNum *int

	// 默认评估器并发数
	EvaluatorsConcurNum *int
}

// ToEvaluatorRefDO 转换为评估器引用DO
func (e *ExptTemplate) ToEvaluatorRefDO() []*ExptTemplateEvaluatorRef {
	if e == nil {
		return nil
	}
	cnt := len(e.EvaluatorVersionRef)
	refs := make([]*ExptTemplateEvaluatorRef, 0, cnt)
	for _, evr := range e.EvaluatorVersionRef {
		refs = append(refs, &ExptTemplateEvaluatorRef{
			SpaceID:            e.SpaceID,
			ExptTemplateID:     e.ID,
			EvaluatorID:        evr.EvaluatorID,
			EvaluatorVersionID: evr.EvaluatorVersionID,
		})
	}
	return refs
}

// ContainsEvalTarget 是否包含评估对象
func (e *ExptTemplate) ContainsEvalTarget() bool {
	return e != nil && e.TargetVersionID > 0
}

// ExptTemplateEvaluatorRef 实验模板评估器引用DO
type ExptTemplateEvaluatorRef struct {
	ID                 int64
	SpaceID            int64
	ExptTemplateID     int64
	EvaluatorID        int64
	EvaluatorVersionID int64
}

// ExptTemplateUpdateFields 实验模板更新字段
type ExptTemplateUpdateFields struct {
	Name        string `mapstructure:"name,omitempty"`
	Description string `mapstructure:"description,omitempty"`
}

// ToFieldMap 转换为字段映射
func (e *ExptTemplateUpdateFields) ToFieldMap() (map[string]any, error) {
	m := make(map[string]any)
	if err := mapstructure.Decode(e, &m); err != nil {
		return nil, errorx.Wrapf(err, "ExptTemplateUpdateFields decode to map fail: %v", e)
	}
	return m, nil
}

// Valid 验证模板配置
func (c *ExptTemplateConfiguration) Valid(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("nil ExptTemplateConfiguration")
	}
	// 验证评估器得分加权配置（已移动到 ConnectorConf.EvaluatorsConf）
	if c.ConnectorConf.EvaluatorsConf != nil && c.ConnectorConf.EvaluatorsConf.EnableWeightedScore {
		if len(c.ConnectorConf.EvaluatorsConf.EvaluatorScoreWeights) == 0 {
			return fmt.Errorf("enable_weighted_score is true but evaluator_score_weights is empty")
		}
		// 验证权重总和是否大于0
		var totalWeight float64
		for _, weight := range c.ConnectorConf.EvaluatorsConf.EvaluatorScoreWeights {
			totalWeight += weight
		}
		if totalWeight <= 0 {
			return fmt.Errorf("total evaluator_score_weights must be greater than 0")
		}
	}
	// 验证并发数配置
	if c.ItemConcurNum != nil && *c.ItemConcurNum <= 0 {
		return fmt.Errorf("item_concur_num must be greater than 0")
	}
	if c.EvaluatorsConcurNum != nil && *c.EvaluatorsConcurNum <= 0 {
		return fmt.Errorf("evaluators_concur_num must be greater than 0")
	}
	// 验证ConnectorConf
	if c.ConnectorConf.EvaluatorsConf != nil {
		if err := c.ConnectorConf.EvaluatorsConf.Valid(ctx); err != nil {
			return err
		}
	}
	return nil
}

// GetDefaultItemConcurNum 获取默认评测集并发数
func (c *ExptTemplateConfiguration) GetDefaultItemConcurNum() int {
	const defaultConcurNum = 1
	if c == nil || c.ItemConcurNum == nil || *c.ItemConcurNum <= 0 {
		return defaultConcurNum
	}
	return *c.ItemConcurNum
}

// GetDefaultEvaluatorsConcurNum 获取默认评估器并发数
func (c *ExptTemplateConfiguration) GetDefaultEvaluatorsConcurNum() int {
	const defaultConcurNum = 3
	if c == nil || c.EvaluatorsConcurNum == nil || *c.EvaluatorsConcurNum <= 0 {
		return defaultConcurNum
	}
	return *c.EvaluatorsConcurNum
}

// CreateExptTemplateParam 创建实验模板参数
type CreateExptTemplateParam struct {
	SpaceID              int64
	Name                 string
	Description          string
	EvalSetID            int64
	EvalSetVersionID     int64
	TargetID             int64
	TargetVersionID      int64
	EvaluatorVersionIDs  []int64
	TemplateConf         *ExptTemplateConfiguration
	ExptType             ExptType
	CreateEvalTargetParam *CreateEvalTargetParam
}

// UpdateExptTemplateParam 更新实验模板参数
type UpdateExptTemplateParam struct {
	TemplateID           int64
	SpaceID              int64
	Name                 string
	Description          string
	EvalSetVersionID      int64
	TargetVersionID      int64
	EvaluatorVersionIDs  []int64
	TemplateConf         *ExptTemplateConfiguration
	ExptType             ExptType
	CreateEvalTargetParam *CreateEvalTargetParam
}
