// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// ExptResultExportColumnSpec 与 ExportExptResultRequest.export_columns 对齐：四个一级分组，组内为字符串列表。
// nil：导出全部。非 nil 且四个切片均为 nil：等价 {}，仍表示全量导出。
// 某一字段不传（nil）：该组全选；传 []：该组不导出；传非空列表：仅导出列表内且报告中存在的列。
// 部分导出（任一维度显式配置）时不导出标注列；全量导出仍包含标注列。
type ExptResultExportColumnSpec struct {
	EvalSetFields     []string `json:"eval_set_fields,omitempty"`
	EvalTargetOutputs []string `json:"eval_target_outputs,omitempty"`
	// Metrics 性能指标列名（IDL/kitex: metrics，与 HTTP JSON 字段 metrics 一致）
	Metrics           []string `json:"metrics,omitempty"`
	EvaluatorVersionIds []string `json:"evaluator_version_ids,omitempty"`
}
