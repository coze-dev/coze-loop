// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type EvaluatorRecord struct {
	ID                 int64                `json:"id"`
	SpaceID            int64                `json:"space_id"`
	ExperimentID       int64                `json:"experiment_id"`
	ExperimentRunID    int64                `json:"experiment_run_id"`
	ItemID             int64                `json:"item_id"`
	ItemVersionID      int64                `json:"item_version_id"` // ★ 0=旧数据/无版本; 从 expt_item_ref 同步
	TurnID             int64                `json:"turn_id"`
	EvaluatorVersionID int64                `json:"evaluator_version_id"` // Inline 行写 0 哨兵
	// ★ 新增字段: 统一承载 Builtin / Builtin别名 / Inline 三种来源
	SourceType     EvaluatorRecordSourceType `json:"source_type"`     // 0=旧数据 / 1=Builtin / 2=Inline
	InlineKey      string                    `json:"inline_key"`      // 仅 Inline: __inline_evaluators__ 的 key
	Alias          string                    `json:"alias"`           // 仅 Builtin 别名实例
	TargetRecordID int64                     `json:"target_record_id"` // Inline 回指来源 eval_target_record.id; Builtin 为 0
	TraceID             string               `json:"trace_id"`
	LogID               string               `json:"log_id"`
	EvaluatorInputData  *EvaluatorInputData  `json:"evaluator_input_data"`
	EvaluatorOutputData *EvaluatorOutputData `json:"evaluator_output_data"`
	Status              EvaluatorRunStatus   `json:"status"`
	BaseInfo            *BaseInfo            `json:"base_info"`
	Ext                 map[string]string    `json:"ext,omitempty"`
}

// EvaluatorRecordSourceType evaluator record 来源类型
type EvaluatorRecordSourceType int32

const (
	EvaluatorRecordSourceTypeUnknown EvaluatorRecordSourceType = 0 // 旧数据, 语义同 Builtin
	EvaluatorRecordSourceTypeBuiltin EvaluatorRecordSourceType = 1 // 注册评估器 (含别名实例)
	EvaluatorRecordSourceTypeInline  EvaluatorRecordSourceType = 2 // target output 内嵌评分
)

type EvaluatorInputData struct {
	HistoryMessages            []*Message          `json:"history_messages,omitempty"`
	InputFields                map[string]*Content `json:"input_fields,omitempty"`
	EvaluateDatasetFields      map[string]*Content `json:"evaluate_dataset_fields,omitempty"`
	EvaluateTargetOutputFields map[string]*Content `json:"evaluate_target_output_fields,omitempty"`
	Ext                        map[string]string   `json:"ext,omitempty"`
}

type EvaluatorOutputData struct {
	EvaluatorResult   *EvaluatorResult             `json:"evaluator_result,omitempty"`
	EvaluatorUsage    *EvaluatorUsage              `json:"evaluator_usage,omitempty"`
	EvaluatorRunError *EvaluatorRunError           `json:"evaluator_run_error,omitempty"`
	TimeConsumingMS   int64                        `json:"time_consuming_ms,omitempty"`
	Stdout            string                       `json:"stdout,omitempty"`
	ExtraOutput       *EvaluatorExtraOutputContent `json:"extra_output,omitempty"`
	Ext               map[string]string            `json:"ext,omitempty"`
}

type EvaluatorExtraOutputType string

const (
	EvaluatorExtraOutputTypeHTML     EvaluatorExtraOutputType = "html"
	EvaluatorExtraOutputTypeMarkdown EvaluatorExtraOutputType = "markdown"
)

type EvaluatorExtraOutputContent struct {
	OutputType *EvaluatorExtraOutputType `json:"output_type,omitempty"`
	URI        *string                   `json:"uri,omitempty"`
	URL        *string                   `json:"url,omitempty"`
}

type Correction struct {
	Score     *float64 `json:"score,omitempty"`
	Explain   string   `json:"explain,omitempty"`
	UpdatedBy string   `json:"updated_by,omitempty"`
}

type EvaluatorResult struct {
	Score      *float64    `json:"score,omitempty"`
	Correction *Correction `json:"correction,omitempty"`
	Reasoning  string      `json:"reasoning,omitempty"`
}

type EvaluatorUsage struct {
	InputTokens  int64 `json:"input_tokens,omitempty"`
	OutputTokens int64 `json:"output_tokens,omitempty"`
}

type EvaluatorRunError struct {
	Code    int32  `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type EvaluatorRunStatus int64

const (
	EvaluatorRunStatusUnknown       EvaluatorRunStatus = 0
	EvaluatorRunStatusSuccess       EvaluatorRunStatus = 1
	EvaluatorRunStatusFail          EvaluatorRunStatus = 2
	EvaluatorRunStatusAsyncInvoking EvaluatorRunStatus = 3
	// ★ Skipped: filter 不命中, 不实际调用; 占位 record 供 GUI 展示, 聚合/数仓按 status 过滤
	EvaluatorRunStatusSkipped EvaluatorRunStatus = 4
)

func (e *EvaluatorRecord) GetBaseInfo() *BaseInfo {
	return e.BaseInfo
}

func (e *EvaluatorRecord) SetBaseInfo(info *BaseInfo) {
	e.BaseInfo = info
}

func (e *EvaluatorRecord) GetScore() *float64 {
	if e.EvaluatorOutputData == nil || e.EvaluatorOutputData.EvaluatorResult == nil {
		return nil
	}
	if e.EvaluatorOutputData.EvaluatorResult.Correction != nil {
		return e.EvaluatorOutputData.EvaluatorResult.Correction.Score
	}
	return e.EvaluatorOutputData.EvaluatorResult.Score
}

func (e *EvaluatorRecord) GetReasoning() string {
	if e.EvaluatorOutputData == nil || e.EvaluatorOutputData.EvaluatorResult == nil {
		return ""
	}
	if e.EvaluatorOutputData.EvaluatorResult.Correction != nil {
		return e.EvaluatorOutputData.EvaluatorResult.Correction.Explain
	}
	return e.EvaluatorOutputData.EvaluatorResult.Reasoning
}

func (e *EvaluatorRecord) GetCorrected() bool {
	if e.EvaluatorOutputData == nil || e.EvaluatorOutputData.EvaluatorResult == nil {
		return false
	}
	return e.EvaluatorOutputData.EvaluatorResult.Correction != nil
}
