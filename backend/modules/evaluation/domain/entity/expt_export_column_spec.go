// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// ExptResultExportColumnSpec 与 ExportExptResultRequest.export_columns 对齐：四个一级分组（字符串列表）+ weighted_score 开关。
// 请求未带 export_columns（指针为 nil）：导出全量（含标注列等）。一旦携带 export_columns（含空对象 {}）：按白名单导出，
// 仅 item_id、status 等必填列 + 各分组非空列表中的列；子字段 nil 与 [] 对该组均表示不导出。
// 非空列表：仅导出列表内且报告中存在的列名/token。
// WeightedScore 为 true 时导出加权总分列。
type ExptResultExportColumnSpec struct {
	// 四个切片不可使用 json omitempty：[] 必须能经 JSON 与 nil 区分；nil 与 [] 在白名单模式下对该组等价（均不导出）。
	EvalSetFields       []string `json:"eval_set_fields"`
	EvalTargetOutputs   []string `json:"eval_target_outputs"`
	Metrics             []string `json:"metrics"`
	// 每项为评估器版本 ID（十进制字符串）；列出的每个版本同时导出分数与原因列。
	EvaluatorVersionIds []string `json:"evaluator_version_ids"`
	// WeightedScore 仅在为 true 时生效；nil/false 表示不因此单独增加加权分列。
	WeightedScore *bool `json:"weighted_score,omitempty"`
}
