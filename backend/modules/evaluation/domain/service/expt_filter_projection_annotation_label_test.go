// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

func TestProjectionAnnotationDisplayLabelKeepsStoredIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, kind, label string
		id                int64
	}{
		{"boolean false", "boolean", "false", 123},
		{"categorical alpha", "categorical", "alpha", 124},
		{"numeric label one", "categorical", "1", 124},
		{"numeric label zero", "categorical", "0", 124},
		{"boolean numeric label", "boolean", "0", 123},
		{"numeric text beyond int64 remains display", "categorical", "9223372036854775808", 8000000000000000123},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := &entity.AnnotateData{TagContentType: entity.TagContentType(tc.kind)}
			if tc.kind == "boolean" {
				data.BoolValue = &tc.label
			} else {
				data.Option = &tc.label
			}
			record := &entity.AnnotateRecord{TagValueID: tc.id, AnnotateData: data}
			before, err := json.Marshal(record)
			require.NoError(t, err)
			row := &entity.ExptTurnResultFilterEntity{AnnotationString: map[string]string{}}
			require.NoError(t, projectionAnnotation(row, "key1", record))
			require.Equal(t, tc.id, record.TagValueID)
			require.Equal(t, map[string]string{"key1": strconv.FormatInt(tc.id, 10)}, row.AnnotationString)
			after, err := json.Marshal(record)
			require.NoError(t, err)
			require.Equal(t, before, after)
		})
	}
}

func TestBuildExptFilterProjectionUsesOptionIDsForDisplayLabels(t *testing.T) {
	f := newProjectionFixture(t)
	falseLabel, zeroLabel := "false", "0"
	f.annotations[2].AnnotateData.BoolValue = &falseLabel
	f.tags = append(f.tags, &entity.ExptTurnResultTagRef{SpaceID: 1, ExptID: 2, TagKeyID: 85})
	f.mappings = append(f.mappings, &entity.ExptTurnResultFilterKeyMapping{SpaceID: 1, ExptID: 2, FieldType: entity.FieldTypeManualAnnotation, FromField: "85", ToKey: "85"})
	f.annotationRefs = append(f.annotationRefs, &entity.ExptTurnAnnotateRecordRef{SpaceID: 1, ExptID: 2, ExptTurnResultID: 10, TagKeyID: 85, AnnotateRecordID: 93})
	f.annotations = append(f.annotations, &entity.AnnotateRecord{ID: 93, SpaceID: 1, ExperimentID: 2, TagKeyID: 85, TagValueID: 84, AnnotateData: &entity.AnnotateData{TagContentType: entity.TagContentTypeCategorical, Option: &zeroLabel}})
	svc := f.service()
	rows, err := svc.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, map[string]string{"81": "", "82": "83", "85": "84"}, rows[0].AnnotationString)
	require.Equal(t, map[string]float64{"key1": 0}, rows[0].AnnotationFloat)
	require.Equal(t, 0.5, *rows[0].EvaluatorWeightedScore)
}
