// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	openapiExperiment "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"
)

func TestOpenAPIExportColumnSpecDTO2Inner(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		from *openapiExperiment.ExptResultExportColumnSpec
		run  func(t *testing.T, from *openapiExperiment.ExptResultExportColumnSpec)
	}{
		{
			name: "nil input returns nil",
			from: nil,
			run: func(t *testing.T, from *openapiExperiment.ExptResultExportColumnSpec) {
				got := OpenAPIExportColumnSpecDTO2Inner(from)
				assert.Nil(t, got)
			},
		},
		{
			name: "empty struct all fields nil",
			from: &openapiExperiment.ExptResultExportColumnSpec{},
			run: func(t *testing.T, from *openapiExperiment.ExptResultExportColumnSpec) {
				got := OpenAPIExportColumnSpecDTO2Inner(from)
				assert.NotNil(t, got)
				assert.Nil(t, got.EvalSetFields)
				assert.Nil(t, got.EvalTargetOutputs)
				assert.Nil(t, got.Metrics)
				assert.Nil(t, got.EvaluatorVersionIds)
				assert.Nil(t, got.TagKeyIds)
				assert.Nil(t, got.WeightedScore)
			},
		},
		{
			name: "all fields set with i64 -> decimal string conversion",
			from: &openapiExperiment.ExptResultExportColumnSpec{
				EvalSetFields:       []string{"f1", "f2"},
				EvalTargetOutputs:   []string{"o1"},
				Metrics:             []string{"m1", "m2"},
				EvaluatorVersionIds: []int64{100, 200},
				TagKeyIds:           []int64{42, 43},
				WeightedScore:       gptr.Of(true),
			},
			run: func(t *testing.T, from *openapiExperiment.ExptResultExportColumnSpec) {
				got := OpenAPIExportColumnSpecDTO2Inner(from)
				assert.NotNil(t, got)
				assert.Equal(t, []string{"f1", "f2"}, got.EvalSetFields)
				assert.Equal(t, []string{"o1"}, got.EvalTargetOutputs)
				assert.Equal(t, []string{"m1", "m2"}, got.Metrics)
				assert.Equal(t, []string{"100", "200"}, got.EvaluatorVersionIds)
				assert.Equal(t, []string{"42", "43"}, got.TagKeyIds)
				assert.NotNil(t, got.WeightedScore)
				assert.True(t, *got.WeightedScore)
			},
		},
		{
			name: "weighted score false",
			from: &openapiExperiment.ExptResultExportColumnSpec{
				WeightedScore: gptr.Of(false),
			},
			run: func(t *testing.T, from *openapiExperiment.ExptResultExportColumnSpec) {
				got := OpenAPIExportColumnSpecDTO2Inner(from)
				assert.NotNil(t, got.WeightedScore)
				assert.False(t, *got.WeightedScore)
			},
		},
		{
			name: "deep copy slices not shared with input",
			from: &openapiExperiment.ExptResultExportColumnSpec{
				EvalSetFields:     []string{"a", "b"},
				EvalTargetOutputs: []string{"c"},
				Metrics:           []string{"d"},
				WeightedScore:     gptr.Of(true),
			},
			run: func(t *testing.T, from *openapiExperiment.ExptResultExportColumnSpec) {
				got := OpenAPIExportColumnSpecDTO2Inner(from)

				from.EvalSetFields[0] = "CHANGED"
				from.EvalTargetOutputs[0] = "CHANGED"
				from.Metrics[0] = "CHANGED"
				*from.WeightedScore = false

				assert.Equal(t, "a", got.EvalSetFields[0])
				assert.Equal(t, "c", got.EvalTargetOutputs[0])
				assert.Equal(t, "d", got.Metrics[0])
				assert.True(t, *got.WeightedScore)
			},
		},
		{
			name: "empty slices stay nil (append onto nil source produces nil)",
			from: &openapiExperiment.ExptResultExportColumnSpec{
				EvalSetFields:       []string{},
				EvalTargetOutputs:   []string{},
				Metrics:             []string{},
				EvaluatorVersionIds: []int64{},
				TagKeyIds:           []int64{},
			},
			run: func(t *testing.T, from *openapiExperiment.ExptResultExportColumnSpec) {
				got := OpenAPIExportColumnSpecDTO2Inner(from)
				assert.NotNil(t, got)
				assert.Nil(t, got.EvalSetFields)
				assert.Nil(t, got.EvalTargetOutputs)
				assert.Nil(t, got.Metrics)
				assert.Empty(t, got.EvaluatorVersionIds)
				assert.Empty(t, got.TagKeyIds)
			},
		},
		{
			name: "only evaluator_version_ids set",
			from: &openapiExperiment.ExptResultExportColumnSpec{
				EvaluatorVersionIds: []int64{1, 2, 3},
			},
			run: func(t *testing.T, from *openapiExperiment.ExptResultExportColumnSpec) {
				got := OpenAPIExportColumnSpecDTO2Inner(from)
				assert.Equal(t, []string{"1", "2", "3"}, got.EvaluatorVersionIds)
				assert.Nil(t, got.TagKeyIds)
			},
		},
		{
			name: "only tag_key_ids set",
			from: &openapiExperiment.ExptResultExportColumnSpec{
				TagKeyIds: []int64{9, 10},
			},
			run: func(t *testing.T, from *openapiExperiment.ExptResultExportColumnSpec) {
				got := OpenAPIExportColumnSpecDTO2Inner(from)
				assert.Equal(t, []string{"9", "10"}, got.TagKeyIds)
				assert.Nil(t, got.EvaluatorVersionIds)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t, tt.from)
		})
	}
}

func TestOpenAPIExportTypeDTO2Inner(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   openapiExperiment.ExptResultExportType
		want domain_expt.ExptResultExportType
	}{
		{name: "csv", in: openapiExperiment.ExptResultExportTypeCSV, want: domain_expt.ExptResultExportTypeCSV},
		{name: "empty defaults to csv", in: "", want: domain_expt.ExptResultExportTypeCSV},
		{name: "unknown defaults to csv", in: openapiExperiment.ExptResultExportType("unknown"), want: domain_expt.ExptResultExportTypeCSV},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPIExportTypeDTO2Inner(tt.in))
		})
	}
}

func TestMapInnerCSVExportStatusToOpenAPI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   domain_expt.CSVExportStatus
		want openapiExperiment.CSVExportStatus
	}{
		{name: "running", in: domain_expt.CSVExportStatusRunning, want: openapiExperiment.CSVExportStatusRunning},
		{name: "success", in: domain_expt.CSVExportStatusSuccess, want: openapiExperiment.CSVExportStatusSuccess},
		{name: "failed", in: domain_expt.CSVExportStatusFailed, want: openapiExperiment.CSVExportStatusFailed},
		{name: "unknown maps to unknown", in: domain_expt.CSVExportStatusUnknown, want: openapiExperiment.CSVExportStatusUnknown},
		{name: "garbage defaults to unknown", in: domain_expt.CSVExportStatus("garbage"), want: openapiExperiment.CSVExportStatusUnknown},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := mapInnerCSVExportStatusToOpenAPI(tt.in)
			if assert.NotNil(t, got) {
				assert.Equal(t, tt.want, *got)
			}
		})
	}
}

func TestInnerExportRecordDTO2OpenAPI(t *testing.T) {
	t.Parallel()

	t.Run("nil input returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, InnerExportRecordDTO2OpenAPI(nil))
	})

	t.Run("empty struct fills required fields with zero values", func(t *testing.T) {
		t.Parallel()
		got := InnerExportRecordDTO2OpenAPI(&domain_expt.ExptResultExportRecord{})
		if assert.NotNil(t, got) {
			assert.Equal(t, int64(0), got.GetExportID())
			assert.Equal(t, int64(0), got.GetWorkspaceID())
			assert.Equal(t, int64(0), got.GetExptID())
			assert.NotNil(t, got.CsvExportStatus)
			assert.Equal(t, openapiExperiment.CSVExportStatusUnknown, *got.CsvExportStatus)
			assert.False(t, got.GetExpired())
			assert.Nil(t, got.URL)
			assert.Nil(t, got.StartTime)
			assert.Nil(t, got.EndTime)
			assert.Nil(t, got.Error)
		}
	})

	t.Run("all fields populated", func(t *testing.T) {
		t.Parallel()
		errCode := int64(5001)
		errMsg := "boom"
		errDetail := "stack-trace"
		url := "https://example.com/file.csv"
		from := &domain_expt.ExptResultExportRecord{
			ExportID:        100,
			WorkspaceID:     200,
			ExptID:          300,
			CsvExportStatus: domain_expt.CSVExportStatusFailed,
			StartTime:       gptr.Of(int64(1000)),
			EndTime:         gptr.Of(int64(2000)),
			URL:             gptr.Of(url),
			Expired:         gptr.Of(true),
			Error: &domain_expt.RunError{
				Code:    errCode,
				Message: gptr.Of(errMsg),
				Detail:  gptr.Of(errDetail),
			},
		}
		got := InnerExportRecordDTO2OpenAPI(from)
		if assert.NotNil(t, got) {
			assert.Equal(t, int64(100), got.GetExportID())
			assert.Equal(t, int64(200), got.GetWorkspaceID())
			assert.Equal(t, int64(300), got.GetExptID())
			assert.NotNil(t, got.CsvExportStatus)
			assert.Equal(t, openapiExperiment.CSVExportStatusFailed, *got.CsvExportStatus)
			assert.Equal(t, int64(1000), got.GetStartTime())
			assert.Equal(t, int64(2000), got.GetEndTime())
			assert.Equal(t, url, got.GetURL())
			assert.True(t, got.GetExpired())
			if assert.NotNil(t, got.Error) {
				assert.Equal(t, errCode, got.Error.GetCode())
				assert.Equal(t, errMsg, got.Error.GetMessage())
				assert.Equal(t, errDetail, got.Error.GetDetail())
			}
		}
	})

	t.Run("optional time and url unset", func(t *testing.T) {
		t.Parallel()
		from := &domain_expt.ExptResultExportRecord{
			ExportID:        1,
			WorkspaceID:     2,
			ExptID:          3,
			CsvExportStatus: domain_expt.CSVExportStatusSuccess,
			Expired:         gptr.Of(false),
		}
		got := InnerExportRecordDTO2OpenAPI(from)
		if assert.NotNil(t, got) {
			assert.Nil(t, got.URL)
			assert.Nil(t, got.StartTime)
			assert.Nil(t, got.EndTime)
			assert.Nil(t, got.Error)
			assert.False(t, got.GetExpired())
		}
	})

	t.Run("error with only code", func(t *testing.T) {
		t.Parallel()
		from := &domain_expt.ExptResultExportRecord{
			ExportID:        1,
			WorkspaceID:     2,
			ExptID:          3,
			CsvExportStatus: domain_expt.CSVExportStatusFailed,
			Error:           &domain_expt.RunError{Code: 999},
		}
		got := InnerExportRecordDTO2OpenAPI(from)
		if assert.NotNil(t, got) && assert.NotNil(t, got.Error) {
			assert.Equal(t, int64(999), got.Error.GetCode())
			assert.Nil(t, got.Error.Message)
			assert.Nil(t, got.Error.Detail)
		}
	})
}
