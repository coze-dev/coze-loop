// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"math"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

func TestProjectionAnnotationLegacyJSON(t *testing.T) {
	tests := []struct {
		name       string
		payload    string
		tagValueID int64
		wantFloat  map[string]float64
		wantString map[string]string
		wantErr    bool
	}{
		{name: "nil data stays absent", payload: `null`},
		{name: "legacy current score", payload: `{"Score":2.1,"TagContentType":""}`, wantFloat: map[string]float64{"80": 2.1}},
		{name: "legacy present zero", payload: `{"Score":0}`, wantFloat: map[string]float64{"80": 0}},
		{name: "legacy current text", payload: `{"TextValue":"文本1","TagContentType":""}`, wantString: map[string]string{"80": "文本1"}},
		{name: "legacy present empty text", payload: `{"TextValue":""}`, wantString: map[string]string{"80": ""}},
		{name: "legacy categorical option", payload: `{"Option":"123"}`, wantString: map[string]string{"80": "123"}},
		{name: "legacy large option ID stays exact", payload: `{"Option":"8000000000000000123"}`, wantString: map[string]string{"80": "8000000000000000123"}},
		{name: "legacy boolean option", payload: `{"BoolValue":"124"}`, wantString: map[string]string{"80": "124"}},
		{name: "legacy absent fields", payload: `{}`, wantErr: true},
		{name: "legacy id alone cannot infer type", payload: `{}`, tagValueID: 123, wantErr: true},
		{name: "legacy score and text ambiguous", payload: `{"Score":2.1,"TextValue":"text"}`, wantErr: true},
		{name: "legacy zero and empty remain ambiguous", payload: `{"Score":0,"TextValue":""}`, wantErr: true},
		{name: "legacy option types ambiguous", payload: `{"Option":"123","BoolValue":"123"}`, wantErr: true},
		{name: "unknown type does not infer score", payload: `{"Score":2.1,"TagContentType":"unknown"}`, wantErr: true},
		{name: "unknown type does not infer option", payload: `{"Option":"123","TagContentType":"unknown"}`, wantErr: true},
		{name: "explicit score requires present value", payload: `{"TagContentType":"continuous_number"}`, wantErr: true},
		{name: "explicit text requires present value", payload: `{"TagContentType":"free_text"}`, wantErr: true},
		{name: "explicit type keeps field selection", payload: `{"Score":2.1,"TextValue":"text","TagContentType":"continuous_number"}`, wantFloat: map[string]float64{"80": 2.1}},
		{name: "explicit categorical id", payload: `{"TagContentType":"categorical"}`, tagValueID: 123, wantString: map[string]string{"80": "123"}},
		{name: "explicit boolean id", payload: `{"TagContentType":"boolean"}`, tagValueID: 124, wantString: map[string]string{"80": "124"}},
		{name: "categorical JSON without denormalized id", payload: `{"Option":"123","TagContentType":"categorical"}`, wantString: map[string]string{"80": "123"}},
		{name: "boolean JSON without denormalized id", payload: `{"BoolValue":"124","TagContentType":"boolean"}`, wantString: map[string]string{"80": "124"}},
		{name: "matching categorical ids", payload: `{"Option":"123","TagContentType":"categorical"}`, tagValueID: 123, wantString: map[string]string{"80": "123"}},
		{name: "matching boolean ids", payload: `{"BoolValue":"124","TagContentType":"boolean"}`, tagValueID: 124, wantString: map[string]string{"80": "124"}},
		{name: "conflicting categorical ids", payload: `{"Option":"123","TagContentType":"categorical"}`, tagValueID: 125, wantErr: true},
		{name: "conflicting boolean ids", payload: `{"BoolValue":"124","TagContentType":"boolean"}`, tagValueID: 125, wantErr: true},
		{name: "missing categorical id is not zero", payload: `{"TagContentType":"categorical"}`, wantErr: true},
		{name: "missing boolean id is not zero", payload: `{"TagContentType":"boolean"}`, wantErr: true},
		{name: "present empty option cannot fall back", payload: `{"Option":"","TagContentType":"categorical"}`, tagValueID: 123, wantErr: true},
		{name: "zero option id rejected", payload: `{"Option":"0"}`, wantErr: true},
		{name: "negative option id rejected", payload: `{"Option":"-1"}`, wantErr: true},
		{name: "boolean label is not option id", payload: `{"BoolValue":"true"}`, wantErr: true},
		{name: "option id overflow rejected", payload: `{"Option":"9223372036854775808"}`, wantErr: true},
		{name: "negative record id cannot fall back", payload: `{"Option":"123","TagContentType":"categorical"}`, tagValueID: -1, wantErr: true},
		{name: "invalid explicit JSON cannot fall back", payload: `{"Option":"invalid","TagContentType":"categorical"}`, tagValueID: 123, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var data *entity.AnnotateData
			require.NoError(t, json.Unmarshal([]byte(tt.payload), &data))
			record := &entity.AnnotateRecord{ID: 90, TagKeyID: 80, TagValueID: tt.tagValueID, AnnotateData: data}
			before, err := json.Marshal(record)
			require.NoError(t, err)
			row := &entity.ExptTurnResultFilterEntity{AnnotationFloat: map[string]float64{}, AnnotationString: map[string]string{}}
			err = projectionAnnotation(row, "80", record)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if tt.wantFloat == nil {
				require.Empty(t, row.AnnotationFloat)
			} else {
				require.Equal(t, tt.wantFloat, row.AnnotationFloat)
			}
			if tt.wantString == nil {
				require.Empty(t, row.AnnotationString)
			} else {
				require.Equal(t, tt.wantString, row.AnnotationString)
			}
			after, err := json.Marshal(record)
			require.NoError(t, err)
			require.Equal(t, before, after, "projection must not rewrite annotation data")
		})
	}
}

func TestProjectionAnnotationRejectsNonfiniteLegacyScore(t *testing.T) {
	for _, contentType := range []entity.TagContentType{"", entity.TagContentTypeContinuousNumber} {
		for _, score := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
			row := &entity.ExptTurnResultFilterEntity{AnnotationFloat: map[string]float64{}}
			err := projectionAnnotation(row, "80", &entity.AnnotateRecord{AnnotateData: &entity.AnnotateData{Score: &score, TagContentType: contentType}})
			require.Error(t, err)
			require.Empty(t, row.AnnotationFloat)
		}
	}
}

func TestBuildExptFilterProjectionLegacyAnnotationsKeepCurrentJSON(t *testing.T) {
	f := newProjectionFixture(t)
	for _, record := range f.annotations {
		record.AnnotateData = &entity.AnnotateData{}
	}
	require.NoError(t, json.Unmarshal([]byte(`{"Score":2.1,"TextValue":null,"BoolValue":null,"Option":null,"TagContentType":""}`), f.annotations[0].AnnotateData))
	require.NoError(t, json.Unmarshal([]byte(`{"Score":null,"TextValue":"文本1","BoolValue":null,"Option":null,"TagContentType":""}`), f.annotations[1].AnnotateData))
	require.NoError(t, json.Unmarshal([]byte(`{"BoolValue":"83","TagContentType":"boolean"}`), f.annotations[2].AnnotateData))
	f.annotations[2].TagValueID = 0
	before, err := json.Marshal(f.annotations)
	require.NoError(t, err)
	s := f.service()
	rows, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, map[string]float64{"key1": 2.1}, rows[0].AnnotationFloat)
	require.Equal(t, map[string]string{"81": "文本1", "82": "83"}, rows[0].AnnotationString)
	require.Equal(t, map[string]float64{"key11": 0, "key12": 1}, rows[0].EvaluatorScore)
	require.Equal(t, 0.5, *rows[0].EvaluatorWeightedScore)
	after, err := json.Marshal(f.annotations)
	require.NoError(t, err)
	require.Equal(t, before, after)
}
