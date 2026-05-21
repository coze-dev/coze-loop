// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"strconv"

	"github.com/bytedance/gg/gptr"

	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	openapiExperiment "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
)

// OpenAPIExportColumnSpecDTO2Inner 将 OpenAPI 的 ExptResultExportColumnSpec 转为内部 expt 包版本，
// 以便复用 experimentApp.ExportExptResult_。两者结构基本一致，差别只在评估器版本/标注 tag key 列表的类型：
// OpenAPI 使用 i64（带 js_conv），内部使用十进制字符串。
func OpenAPIExportColumnSpecDTO2Inner(from *openapiExperiment.ExptResultExportColumnSpec) *expt.ExptResultExportColumnSpec {
	if from == nil {
		return nil
	}
	to := &expt.ExptResultExportColumnSpec{}
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
		ids := from.GetEvaluatorVersionIds()
		to.EvaluatorVersionIds = make([]string, 0, len(ids))
		for _, id := range ids {
			to.EvaluatorVersionIds = append(to.EvaluatorVersionIds, strconv.FormatInt(id, 10))
		}
	}
	if from.IsSetTagKeyIds() {
		ids := from.GetTagKeyIds()
		to.TagKeyIds = make([]string, 0, len(ids))
		for _, id := range ids {
			to.TagKeyIds = append(to.TagKeyIds, strconv.FormatInt(id, 10))
		}
	}
	if from.WeightedScore != nil {
		v := *from.WeightedScore
		to.WeightedScore = &v
	}
	return to
}

// OpenAPIExportTypeDTO2Inner OpenAPI 导出类型 -> 内部 domain/expt 导出类型。空值默认 CSV。
func OpenAPIExportTypeDTO2Inner(from openapiExperiment.ExptResultExportType) domain_expt.ExptResultExportType {
	switch from {
	case openapiExperiment.ExptResultExportTypeCSV, "":
		return domain_expt.ExptResultExportTypeCSV
	default:
		return domain_expt.ExptResultExportTypeCSV
	}
}

// InnerExportRecordDTO2OpenAPI 将内部 domain_expt.ExptResultExportRecord（experimentApp.GetExptResultExportRecord
// 返回的 DTO）转为 openapi 版本。
func InnerExportRecordDTO2OpenAPI(in *domain_expt.ExptResultExportRecord) *openapiExperiment.ExptResultExportRecord {
	if in == nil {
		return nil
	}
	out := &openapiExperiment.ExptResultExportRecord{
		ExportID:        gptr.Of(in.GetExportID()),
		WorkspaceID:     gptr.Of(in.GetWorkspaceID()),
		ExptID:          gptr.Of(in.GetExptID()),
		CsvExportStatus: mapInnerCSVExportStatusToOpenAPI(in.GetCsvExportStatus()),
		Expired:         gptr.Of(in.GetExpired()),
	}
	if in.IsSetURL() {
		out.URL = gptr.Of(in.GetURL())
	}
	if in.IsSetStartTime() {
		out.StartTime = gptr.Of(in.GetStartTime())
	}
	if in.IsSetEndTime() {
		out.EndTime = gptr.Of(in.GetEndTime())
	}
	if in.IsSetError() {
		out.Error = &openapiExperiment.RunError{
			Code:    gptr.Of(in.GetError().GetCode()),
			Message: in.GetError().Message,
			Detail:  in.GetError().Detail,
		}
	}
	return out
}

func mapInnerCSVExportStatusToOpenAPI(s domain_expt.CSVExportStatus) *openapiExperiment.CSVExportStatus {
	var v openapiExperiment.CSVExportStatus
	switch s {
	case domain_expt.CSVExportStatusRunning:
		v = openapiExperiment.CSVExportStatusRunning
	case domain_expt.CSVExportStatusSuccess:
		v = openapiExperiment.CSVExportStatusSuccess
	case domain_expt.CSVExportStatusFailed:
		v = openapiExperiment.CSVExportStatusFailed
	default:
		v = openapiExperiment.CSVExportStatusUnknown
	}
	return &v
}
