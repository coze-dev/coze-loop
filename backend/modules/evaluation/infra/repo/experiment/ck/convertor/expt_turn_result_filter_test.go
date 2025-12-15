// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/ck/gorm_gen/model"
)

func TestExptTurnResultFilterEntity2PO_EvalTargetMetrics(t *testing.T) {
	tests := []struct {
		name         string
		filterEntity *entity.ExptTurnResultFilterEntity
		want         *model.ExptTurnResultFilter
	}{
		{
			name: "with EvalTargetMetrics",
			filterEntity: &entity.ExptTurnResultFilterEntity{
				SpaceID:          1,
				ExptID:           2,
				ItemID:           3,
				ItemIdx:          4,
				TurnID:           5,
				Status:           entity.ItemRunState_Success,
				EvalTargetData:   make(map[string]string),
				EvaluatorScore:   make(map[string]float64),
				AnnotationFloat:  make(map[string]float64),
				AnnotationBool:   make(map[string]bool),
				AnnotationString: make(map[string]string),
				EvalTargetMetrics: map[string]int64{
					"total_latency": 100,
					"input_tokens":  200,
					"output_tokens": 300,
					"total_tokens":  500,
				},
				CreatedDate:             time.Now(),
				EvaluatorScoreCorrected: false,
				EvalSetVersionID:        6,
				CreatedAt:               time.Now(),
				UpdatedAt:               time.Now(),
			},
			want: &model.ExptTurnResultFilter{
				EvalTargetMetrics: map[string]int64{
					"total_latency": 100,
					"input_tokens":  200,
					"output_tokens": 300,
					"total_tokens":  500,
				},
			},
		},
		{
			name: "with empty EvalTargetMetrics",
			filterEntity: &entity.ExptTurnResultFilterEntity{
				SpaceID:                 1,
				ExptID:                  2,
				ItemID:                  3,
				ItemIdx:                 4,
				TurnID:                  5,
				Status:                  entity.ItemRunState_Success,
				EvalTargetData:          make(map[string]string),
				EvaluatorScore:          make(map[string]float64),
				AnnotationFloat:         make(map[string]float64),
				AnnotationBool:          make(map[string]bool),
				AnnotationString:        make(map[string]string),
				EvalTargetMetrics:       make(map[string]int64),
				CreatedDate:             time.Now(),
				EvaluatorScoreCorrected: false,
				EvalSetVersionID:        6,
				CreatedAt:               time.Now(),
				UpdatedAt:               time.Now(),
			},
			want: &model.ExptTurnResultFilter{
				EvalTargetMetrics: make(map[string]int64),
			},
		},
		{
			name:         "nil filterEntity",
			filterEntity: nil,
			want:         nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExptTurnResultFilterEntity2PO(tt.filterEntity)
			if tt.want == nil {
				assert.Nil(t, got)
				return
			}
			assert.NotNil(t, got)
			assert.Equal(t, tt.want.EvalTargetMetrics, got.EvalTargetMetrics)
		})
	}
}

func TestExptTurnResultFilterPO2Entity_EvalTargetMetrics(t *testing.T) {
	tests := []struct {
		name     string
		filterPO *model.ExptTurnResultFilter
		want     *entity.ExptTurnResultFilterEntity
	}{
		{
			name: "with EvalTargetMetrics",
			filterPO: &model.ExptTurnResultFilter{
				SpaceID:          "1",
				ExptID:           "2",
				ItemID:           "3",
				ItemIdx:          4,
				TurnID:           "5",
				Status:           2,
				EvalTargetData:   make(map[string]string),
				EvaluatorScore:   make(map[string]float64),
				AnnotationFloat:  make(map[string]float64),
				AnnotationBool:   make(map[string]int8),
				AnnotationString: make(map[string]string),
				EvalTargetMetrics: map[string]int64{
					"total_latency": 100,
					"input_tokens":  200,
					"output_tokens": 300,
					"total_tokens":  500,
				},
				EvaluatorScoreCorrected: 0,
				EvalSetVersionID:        "6",
				CreatedDate:             time.Now(),
				CreatedAt:               time.Now(),
				UpdatedAt:               time.Now(),
			},
			want: &entity.ExptTurnResultFilterEntity{
				EvalTargetMetrics: map[string]int64{
					"total_latency": 100,
					"input_tokens":  200,
					"output_tokens": 300,
					"total_tokens":  500,
				},
			},
		},
		{
			name: "with empty EvalTargetMetrics",
			filterPO: &model.ExptTurnResultFilter{
				SpaceID:                 "1",
				ExptID:                  "2",
				ItemID:                  "3",
				ItemIdx:                 4,
				TurnID:                  "5",
				Status:                  2,
				EvalTargetData:          make(map[string]string),
				EvaluatorScore:          make(map[string]float64),
				AnnotationFloat:         make(map[string]float64),
				AnnotationBool:          make(map[string]int8),
				AnnotationString:        make(map[string]string),
				EvalTargetMetrics:       make(map[string]int64),
				EvaluatorScoreCorrected: 0,
				EvalSetVersionID:        "6",
				CreatedDate:             time.Now(),
				CreatedAt:               time.Now(),
				UpdatedAt:               time.Now(),
			},
			want: &entity.ExptTurnResultFilterEntity{
				EvalTargetMetrics: make(map[string]int64),
			},
		},
		{
			name:     "nil filterPO",
			filterPO: nil,
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExptTurnResultFilterPO2Entity(tt.filterPO)
			if tt.want == nil {
				assert.Nil(t, got)
				return
			}
			assert.NotNil(t, got)
			assert.Equal(t, tt.want.EvalTargetMetrics, got.EvalTargetMetrics)
		})
	}
}

func TestExptTurnResultFilterEntity2PO_And_PO2Entity_EvalTargetMetrics(t *testing.T) {
	original := &entity.ExptTurnResultFilterEntity{
		SpaceID:          1,
		ExptID:           2,
		ItemID:           3,
		ItemIdx:          4,
		TurnID:           5,
		Status:           entity.ItemRunState_Success,
		EvalTargetData:   map[string]string{"key1": "value1"},
		EvaluatorScore:   map[string]float64{"key1": 1.5},
		AnnotationFloat:  map[string]float64{"key1": 2.5},
		AnnotationBool:   map[string]bool{"key1": true},
		AnnotationString: map[string]string{"key1": "value1"},
		EvalTargetMetrics: map[string]int64{
			"total_latency": 100,
			"input_tokens":  200,
			"output_tokens": 300,
			"total_tokens":  500,
		},
		CreatedDate:             time.Now(),
		EvaluatorScoreCorrected: true,
		EvalSetVersionID:        6,
		CreatedAt:               time.Now(),
		UpdatedAt:               time.Now(),
	}

	// Entity -> PO
	po := ExptTurnResultFilterEntity2PO(original)
	assert.NotNil(t, po)
	assert.Equal(t, original.EvalTargetMetrics, po.EvalTargetMetrics)

	// PO -> Entity
	entity := ExptTurnResultFilterPO2Entity(po)
	assert.NotNil(t, entity)
	assert.Equal(t, original.EvalTargetMetrics, entity.EvalTargetMetrics)
}
