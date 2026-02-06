// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/tag"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestExptColumnEvalTargetDO2DTOs(t *testing.T) {
	label := gptr.Of("label-1")
	from := []*entity.ExptColumnEvalTarget{
		{
			ExptID: 101,
			Columns: []*entity.ColumnEvalTarget{
				{
					Name:  "col-1",
					Desc:  "desc-1",
					Label: label,
				},
				{
					Name: "col-2",
					Desc: "desc-2",
				},
			},
		},
		{
			ExptID: 202,
		},
	}

	got := ExptColumnEvalTargetDO2DTOs(from)

	assert.Len(t, got, len(from))
	assert.NotNil(t, got[0].ExperimentID)
	assert.Equal(t, from[0].ExptID, *got[0].ExperimentID)
	assert.Len(t, got[0].ColumnEvalTargets, len(from[0].Columns))
	assert.NotNil(t, got[0].ColumnEvalTargets[0].Name)
	assert.Equal(t, from[0].Columns[0].Name, *got[0].ColumnEvalTargets[0].Name)
	assert.NotNil(t, got[0].ColumnEvalTargets[0].Description)
	assert.Equal(t, from[0].Columns[0].Desc, *got[0].ColumnEvalTargets[0].Description)
	assert.Same(t, label, got[0].ColumnEvalTargets[0].Label)

	assert.NotNil(t, got[0].ColumnEvalTargets[1].Name)
	assert.Equal(t, from[0].Columns[1].Name, *got[0].ColumnEvalTargets[1].Name)
	assert.NotNil(t, got[0].ColumnEvalTargets[1].Description)
	assert.Equal(t, from[0].Columns[1].Desc, *got[0].ColumnEvalTargets[1].Description)
	assert.Nil(t, got[0].ColumnEvalTargets[1].Label)

	assert.NotNil(t, got[1].ExperimentID)
	assert.Equal(t, from[1].ExptID, *got[1].ExperimentID)
	assert.Len(t, got[1].ColumnEvalTargets, 0)
}

func TestColumnEvalTargetDO2DTOs(t *testing.T) {
	label := gptr.Of("label-1")
	from := []*entity.ColumnEvalTarget{
		{
			Name:  "col-1",
			Desc:  "desc-1",
			Label: label,
		},
		{
			Name: "col-2",
			Desc: "desc-2",
		},
	}

	got := ColumnEvalTargetDO2DTOs(from)

	assert.Len(t, got, len(from))

	assert.NotNil(t, got[0].Name)
	assert.Equal(t, from[0].Name, *got[0].Name)
	assert.NotNil(t, got[0].Description)
	assert.Equal(t, from[0].Desc, *got[0].Description)
	assert.Same(t, label, got[0].Label)

	assert.NotNil(t, got[1].Name)
	assert.Equal(t, from[1].Name, *got[1].Name)
	assert.NotNil(t, got[1].Description)
	assert.Equal(t, from[1].Desc, *got[1].Description)
	assert.Nil(t, got[1].Label)
}

func TestItemResultsDO2DTO_ExtField(t *testing.T) {
	tests := []struct {
		name    string
		from    *entity.ItemResult
		wantExt map[string]string
		wantNil bool
	}{
		{
			name: "Ext field has value",
			from: &entity.ItemResult{
				ItemID:      1,
				TurnResults: []*entity.TurnResult{},
				ItemIndex:   gptr.Of(int64(10)),
				Ext: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
			wantExt: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			wantNil: false,
		},
		{
			name: "Ext field is empty map",
			from: &entity.ItemResult{
				ItemID:      1,
				TurnResults: []*entity.TurnResult{},
				ItemIndex:   gptr.Of(int64(10)),
				Ext:         map[string]string{},
			},
			wantExt: nil,
			wantNil: true,
		},
		{
			name: "Ext field is nil",
			from: &entity.ItemResult{
				ItemID:      1,
				TurnResults: []*entity.TurnResult{},
				ItemIndex:   gptr.Of(int64(10)),
				Ext:         nil,
			},
			wantExt: nil,
			wantNil: true,
		},
		{
			name: "Ext field has multiple values",
			from: &entity.ItemResult{
				ItemID:      1,
				TurnResults: []*entity.TurnResult{},
				ItemIndex:   gptr.Of(int64(10)),
				Ext: map[string]string{
					"span_id":  "span-123",
					"trace_id": "trace-456",
					"log_id":   "log-789",
				},
			},
			wantExt: map[string]string{
				"span_id":  "span-123",
				"trace_id": "trace-456",
				"log_id":   "log-789",
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ItemResultsDO2DTO(tt.from)
			assert.NotNil(t, got)
			assert.Equal(t, tt.from.ItemID, got.ItemID)
			assert.Equal(t, tt.from.ItemIndex, got.ItemIndex)

			if tt.wantNil {
				assert.Nil(t, got.Ext)
			} else {
				assert.NotNil(t, got.Ext)
				assert.Equal(t, tt.wantExt, got.Ext)
			}
		})
	}
}

func TestColumnEvalSetFieldsDO2DTO(t *testing.T) {
	from := &entity.ColumnEvalSetField{
		Key:         gptr.Of("key1"),
		Name:        gptr.Of("name1"),
		Description: gptr.Of("desc1"),
		ContentType: "Text",
		TextSchema:  gptr.Of("schema1"),
		SchemaKey:   gptr.Of(entity.SchemaKey(dataset.SchemaKey_String)),
	}

	got := ColumnEvalSetFieldsDO2DTO(from)

	assert.Equal(t, *from.Key, *got.Key)
	assert.Equal(t, *from.Name, *got.Name)
	assert.Equal(t, *from.Description, *got.Description)
	assert.Equal(t, from.TextSchema, got.TextSchema)
	assert.Equal(t, dataset.SchemaKey_String, *got.SchemaKey)
}

func TestColumnEvalSetFieldsDO2DTOs(t *testing.T) {
	from := []*entity.ColumnEvalSetField{
		{
			Key: gptr.Of("key1"),
		},
		{
			Key: gptr.Of("key2"),
		},
	}

	got := ColumnEvalSetFieldsDO2DTOs(from)

	assert.Len(t, got, 2)
	assert.Equal(t, *from[0].Key, *got[0].Key)
	assert.Equal(t, *from[1].Key, *got[1].Key)
}

func TestColumnEvaluatorsDO2DTO(t *testing.T) {
	from := &entity.ColumnEvaluator{
		EvaluatorVersionID: 1,
		EvaluatorID:        2,
		EvaluatorType:      3,
		Name:               gptr.Of("name1"),
		Version:            gptr.Of("v1"),
		Description:        gptr.Of("desc1"),
		Builtin:            gptr.Of(true),
	}

	got := ColumnEvaluatorsDO2DTO(from)

	assert.Equal(t, from.EvaluatorVersionID, got.EvaluatorVersionID)
	assert.Equal(t, from.EvaluatorID, got.EvaluatorID)
	assert.Equal(t, evaluator.EvaluatorType(from.EvaluatorType), got.EvaluatorType)
	assert.Equal(t, *from.Name, *got.Name)
	assert.Equal(t, *from.Version, *got.Version)
	assert.Equal(t, *from.Description, *got.Description)
	assert.Equal(t, *from.Builtin, *got.Builtin)
}

func TestExptColumnEvaluatorsDO2DTOs(t *testing.T) {
	from := []*entity.ExptColumnEvaluator{
		{
			ExptID: 101,
			ColumnEvaluators: []*entity.ColumnEvaluator{
				{
					Name: gptr.Of("eval1"),
				},
			},
		},
	}

	got := ExptColumnEvaluatorsDO2DTOs(from)

	assert.Len(t, got, 1)
	assert.Equal(t, from[0].ExptID, got[0].ExperimentID)
	assert.Len(t, got[0].ColumnEvaluators, 1)
	assert.Equal(t, *from[0].ColumnEvaluators[0].Name, *got[0].ColumnEvaluators[0].Name)
}

func TestTagValueDO2DtO(t *testing.T) {
	from := &entity.TagValue{
		TagValueId:   1,
		TagValueName: "tag1",
		Status:       "active",
	}

	got := TagValueDO2DtO(from)

	assert.Equal(t, from.TagValueId, *got.TagValueID)
	assert.Equal(t, from.TagValueName, *got.TagValueName)
	assert.Equal(t, tag.TagStatus(from.Status), *got.Status)
}

func TestExptColumnAnnotationDO2DTOs(t *testing.T) {
	from := []*entity.ExptColumnAnnotation{
		{
			ExptID: 101,
			ColumnAnnotations: []*entity.ColumnAnnotation{
				{
					TagName: "tag1",
					TagContentSpec: &entity.TagContentSpec{
						ContinuousNumberSpec: &entity.ContinuousNumberSpec{
							MinValue: gptr.Of(float64(1)),
						},
					},
					TagStatus: "active",
				},
			},
		},
	}

	got := ExptColumnAnnotationDO2DTOs(from)

	assert.Len(t, got, 1)
	assert.Equal(t, from[0].ExptID, got[0].ExperimentID)
	assert.Len(t, got[0].ColumnAnnotations, 1)
	assert.Equal(t, from[0].ColumnAnnotations[0].TagName, *got[0].ColumnAnnotations[0].TagKeyName)
	assert.NotNil(t, got[0].ColumnAnnotations[0].ContentSpec)
	assert.Equal(t, *from[0].ColumnAnnotations[0].TagContentSpec.ContinuousNumberSpec.MinValue, *got[0].ColumnAnnotations[0].ContentSpec.ContinuousNumberSpec.MinValue)
}

func TestTurnResultsDO2DTO(t *testing.T) {
	from := &entity.TurnResult{
		TurnID:    1,
		TurnIndex: gptr.Of(int64(0)),
		ExperimentResults: []*entity.ExperimentResult{
			{
				ExperimentID: 101,
				Payload: &entity.ExperimentTurnPayload{
					TurnID: 1,
				},
			},
		},
	}

	got := TurnResultsDO2DTO(from)

	assert.Equal(t, from.TurnID, got.TurnID)
	assert.Equal(t, from.TurnIndex, got.TurnIndex)
	assert.Len(t, got.ExperimentResults, 1)
}

func TestTurnAnnotationDO2DTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		got := TurnAnnotationDO2DTO(nil)
		assert.NotNil(t, got)
		assert.Empty(t, got.AnnotateRecords)
	})

	t.Run("with records", func(t *testing.T) {
		from := &entity.TurnAnnotateResult{
			AnnotateRecords: map[int64]*entity.AnnotateRecord{
				1: {
					ID:       1,
					TagKeyID: 2,
					AnnotateData: &entity.AnnotateData{
						Score: gptr.Of(float64(4.5)),
					},
				},
			},
		}

		got := TurnAnnotationDO2DTO(from)

		assert.Len(t, got.AnnotateRecords, 1)
		assert.Equal(t, "4.5", *got.AnnotateRecords[1].Score)
	})
}

func TestTurnTrajectoryAnalysisResultDO2DTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		got := TurnTrajectoryAnalysisResultDO2DTO(nil)
		assert.NotNil(t, got)
	})

	t.Run("with data", func(t *testing.T) {
		from := &entity.AnalysisRecord{
			ID:     1,
			Status: 1,
		}
		got := TurnTrajectoryAnalysisResultDO2DTO(from)
		assert.Equal(t, from.ID, *got.RecordID)
	})
}

func TestTurnSystemInfoDO2DTO(t *testing.T) {
	from := &entity.TurnSystemInfo{
		TurnRunState: 1,
		LogID:        gptr.Of("log1"),
		Error: &entity.RunError{
			Code:    1,
			Message: gptr.Of("msg1"),
		},
	}

	got := TurnSystemInfoDO2DTO(from)

	assert.Equal(t, int32(from.TurnRunState), int32(*got.TurnRunState))
	assert.Equal(t, from.LogID, got.LogID)
	assert.Equal(t, from.Error.Code, got.Error.Code)
}

func TestExportRecordDO2DTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		got := ExportRecordDO2DTO(nil)
		assert.Nil(t, got)
	})

	t.Run("with data", func(t *testing.T) {
		now := time.Now()
		from := &entity.ExptResultExportRecord{
			ID:              1,
			SpaceID:         2,
			ExptID:          101,
			CsvExportStatus: entity.CSVExportStatus_Success,
			CreatedBy:       "3",
			URL:             gptr.Of("http://test.com"),
			Expired:         false,
			StartAt:         &now,
			EndAt:           &now,
			ErrMsg:          "",
		}

		got := ExportRecordDO2DTO(from)

		assert.Equal(t, from.ID, got.ExportID)
		assert.Equal(t, from.SpaceID, got.WorkspaceID)
		assert.Equal(t, from.ExptID, got.ExptID)
		assert.Equal(t, domain_expt.CSVExportStatusSuccess, got.CsvExportStatus)
		assert.Equal(t, *from.URL, *got.URL)
		assert.Equal(t, *from.URL, *got.URL_)
		assert.Equal(t, from.StartAt.Unix(), *got.StartTime)
		assert.Equal(t, from.EndAt.Unix(), *got.EndTime)
	})
}
