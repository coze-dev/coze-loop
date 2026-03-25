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
	Meta               *ExptTemplateMeta
	TripleConfig       *ExptTemplateTuple
	FieldMappingConfig *ExptFieldMapping

	// 关联数据（用于内部使用，不存储在数据库中）
	Target     *EvalTarget
	EvalSet    *EvaluationSet
	Evaluators []*Evaluator

	// 内部使用的评估器版本引用（从 TripleConfig.EvaluatorVersionIds 派生）
	EvaluatorVersionRef []*ExptTemplateEvaluatorVersionRef
	// 内部使用的模板配置（从 FieldMappingConfig 和 ScoreWeightConfig 派生，用于存储到数据库）
	TemplateConf *ExptTemplateConfiguration

	// BaseInfo 基础信息（创建时间、更新时间等）
	BaseInfo *BaseInfo

	// ExptInfo 实验运行状态信息（存储在数据库的 expt_info 字段中，JSON格式）
	ExptInfo *ExptInfo
}

// ExptInfo 实验模板关联的实验运行状态信息
type ExptInfo struct {
	// CreatedExptCount 当前模板创建实验数量
	CreatedExptCount int64 `json:"created_expt_count"`
	// LatestExptID 最后一次创建实验的ID
	LatestExptID int64 `json:"latest_expt_id"`
	// LatestExptStatus 最后一次创建实验的执行状态
	LatestExptStatus ExptStatus `json:"latest_expt_status"`
}

// ExptTemplateMeta 实验模板基础信息
type ExptTemplateMeta struct {
	ID          int64
	WorkspaceID int64
	Name        string
	Desc        string
	ExptType    ExptType
}

// EvaluatorIDVersionItem 评估器ID和版本映射项
// 对应 IDL 中的 evaluator.EvaluatorIDVersionItem
type EvaluatorIDVersionItem struct {
	EvaluatorID        int64   // 评估器ID
	Version            string  // 评估器版本号
	EvaluatorVersionID int64   // 评估器版本ID
	ScoreWeight        float64 // 得分权重（用于加权评分）
}

// ExptTemplateTuple 实验模板三元组配置
// 对应 IDL 中的 ExptTuple
type ExptTemplateTuple struct {
	EvalSetID               int64
	EvalSetVersionID        int64
	TargetID                int64
	TargetVersionID         int64
	TargetType              EvalTargetType
	EvaluatorVersionIds     []int64                   // 从 EvaluatorIDVersionItems 中提取的 evaluator_version_id 列表，用于内部处理（向后兼容）
	EvaluatorIDVersionItems []*EvaluatorIDVersionItem // 评估器ID版本项列表（包含完整信息）
}

// ExptFieldMapping 实验字段映射和运行时参数配置
type ExptFieldMapping struct {
	TargetFieldMapping    *TargetFieldMapping
	EvaluatorFieldMapping []*EvaluatorFieldMapping
	TargetRuntimeParam    *RuntimeParam
	ItemConcurNum         *int
}

// TargetFieldMapping 目标字段映射
type TargetFieldMapping struct {
	FromEvalSet []*ExptTemplateFieldMapping
}

// EvaluatorFieldMapping 评估器字段映射
type EvaluatorFieldMapping struct {
	EvaluatorVersionID int64
	EvaluatorID        int64  // 评估器ID（用于匹配回填 evaluator_version_id）
	Version            string // 评估器版本号（用于匹配回填 evaluator_version_id）
	FromEvalSet        []*ExptTemplateFieldMapping
	FromTarget         []*ExptTemplateFieldMapping
}

// ExptTemplateFieldMapping 实验模板字段映射
type ExptTemplateFieldMapping struct {
	FieldName     string
	ConstValue    string
	FromFieldName string
}

// ExptScoreWeight 实验评估器得分加权配置
type ExptScoreWeight struct {
	EnableWeightedScore   bool
	EvaluatorScoreWeights map[int64]float64
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
	ItemRetryNum        *int
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
			SpaceID:            e.GetSpaceID(),
			ExptTemplateID:     e.GetID(),
			EvaluatorID:        evr.EvaluatorID,
			EvaluatorVersionID: evr.EvaluatorVersionID,
		})
	}
	return refs
}

// ContainsEvalTarget 是否包含评估对象
func (e *ExptTemplate) ContainsEvalTarget() bool {
	return e != nil && e.GetTargetVersionID() > 0
}

// GetID 获取模板ID
func (e *ExptTemplate) GetID() int64 {
	if e == nil || e.Meta == nil {
		return 0
	}
	return e.Meta.ID
}

// GetSpaceID 获取空间ID
func (e *ExptTemplate) GetSpaceID() int64 {
	if e == nil || e.Meta == nil {
		return 0
	}
	return e.Meta.WorkspaceID
}

// GetName 获取模板名称
func (e *ExptTemplate) GetName() string {
	if e == nil || e.Meta == nil {
		return ""
	}
	return e.Meta.Name
}

// GetDescription 获取模板描述
func (e *ExptTemplate) GetDescription() string {
	if e == nil || e.Meta == nil {
		return ""
	}
	return e.Meta.Desc
}

// GetCreatedBy 获取创建者
func (e *ExptTemplate) GetCreatedBy() string {
	if e == nil || e.BaseInfo == nil || e.BaseInfo.CreatedBy == nil || e.BaseInfo.CreatedBy.UserID == nil {
		return ""
	}
	return *e.BaseInfo.CreatedBy.UserID
}

// GetExptType 获取实验类型
func (e *ExptTemplate) GetExptType() ExptType {
	if e == nil || e.Meta == nil {
		return 0
	}
	return e.Meta.ExptType
}

// GetEvalSetID 获取评测集ID
func (e *ExptTemplate) GetEvalSetID() int64 {
	if e == nil || e.TripleConfig == nil {
		return 0
	}
	return e.TripleConfig.EvalSetID
}

// GetEvalSetVersionID 获取评测集版本ID
func (e *ExptTemplate) GetEvalSetVersionID() int64 {
	if e == nil || e.TripleConfig == nil {
		return 0
	}
	return e.TripleConfig.EvalSetVersionID
}

// GetTargetID 获取评估对象ID
func (e *ExptTemplate) GetTargetID() int64 {
	if e == nil || e.TripleConfig == nil {
		return 0
	}
	return e.TripleConfig.TargetID
}

// GetTargetVersionID 获取评估对象版本ID
func (e *ExptTemplate) GetTargetVersionID() int64 {
	if e == nil || e.TripleConfig == nil {
		return 0
	}
	return e.TripleConfig.TargetVersionID
}

// GetTargetType 获取评估对象类型
func (e *ExptTemplate) GetTargetType() EvalTargetType {
	if e == nil || e.TripleConfig == nil {
		return 0
	}
	return e.TripleConfig.TargetType
}

// GetEvaluatorVersionIds 获取评估器版本ID列表
func (e *ExptTemplate) GetEvaluatorVersionIds() []int64 {
	if e == nil || e.TripleConfig == nil {
		return nil
	}
	// 优先从 EvaluatorIDVersionItems 中提取
	if len(e.TripleConfig.EvaluatorIDVersionItems) > 0 {
		ids := make([]int64, 0, len(e.TripleConfig.EvaluatorIDVersionItems))
		for _, item := range e.TripleConfig.EvaluatorIDVersionItems {
			if item != nil && item.EvaluatorVersionID > 0 {
				ids = append(ids, item.EvaluatorVersionID)
			}
		}
		return ids
	}
	// 向后兼容：从 EvaluatorVersionIds 获取
	return e.TripleConfig.EvaluatorVersionIds
}

// GetEvaluatorIDVersionItems 获取评估器ID版本项列表
func (e *ExptTemplate) GetEvaluatorIDVersionItems() []*EvaluatorIDVersionItem {
	if e == nil || e.TripleConfig == nil {
		return nil
	}
	return e.TripleConfig.EvaluatorIDVersionItems
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
	SpaceID                 int64
	Name                    string
	Description             string
	EvalSetID               int64
	EvalSetVersionID        int64
	TargetID                int64
	TargetVersionID         int64
	EvaluatorIDVersionItems []*EvaluatorIDVersionItem // 评估器ID版本项列表（包含完整信息）
	TemplateConf            *ExptTemplateConfiguration
	ExptType                ExptType
	CreateEvalTargetParam   *CreateEvalTargetParam
}

// UpdateExptTemplateParam 更新实验模板参数
type UpdateExptTemplateParam struct {
	TemplateID              int64
	SpaceID                 int64
	Name                    string
	Description             string
	EvalSetVersionID        int64
	TargetVersionID         int64
	EvaluatorIDVersionItems []*EvaluatorIDVersionItem // 评估器ID版本项列表（包含完整信息）
	TemplateConf            *ExptTemplateConfiguration
	ExptType                ExptType
	CreateEvalTargetParam   *CreateEvalTargetParam
}

// UpdateExptTemplateMetaParam 更新实验模板 Meta 参数
type UpdateExptTemplateMetaParam struct {
	TemplateID  int64
	SpaceID     int64
	Name        string
	Description string
	ExptType    ExptType
}
