// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// ExportColumnSpecThrift2Entity Thrift → 领域结构。
func ExportColumnSpecThrift2Entity(from *expt.ExptResultExportColumnSpec) *entity.ExptResultExportColumnSpec {
	if from == nil {
		return nil
	}
	to := &entity.ExptResultExportColumnSpec{}
	if from.IsSetEvalSetFields() {
		to.EvalSetFields = append([]string(nil), from.GetEvalSetFields()...)
	}
	if from.IsSetEvalTargetOutputs() {
		to.EvalTargetOutputs = append([]string(nil), from.GetEvalTargetOutputs()...)
	}
	if from.IsSetMetrics() {
		to.Metrics = append([]string(nil), from.GetMetrics()...)
	}
	if from.IsSetEvaluatorVersionIds() {
		to.EvaluatorVersionIds = append([]string(nil), from.GetEvaluatorVersionIds()...)
	}
	return to
}
