// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"
	"time"
)

func TestExptFilterProjectionEqualityDetectsEveryPersistedField(t *testing.T) {
	fixture := func() *ExptTurnResultFilterEntity {
		weight := 1.25
		return &ExptTurnResultFilterEntity{SpaceID: 1, ExptID: 2, ItemID: 3, TurnID: 0, ItemIdx: 4, Status: ItemRunState_Success, CreatedDate: time.Date(2025, 1, 2, 10, 0, 0, 0, time.UTC), EvalSetID: 5, EvalSetVersionID: 6, EvaluatorScoreCorrected: true, EvaluatorWeightedScore: &weight, EvalTargetData: map[string]string{"actual_output": "", "custom": "value"}, EvaluatorScore: map[string]float64{"key11": 0}, AnnotationFloat: map[string]float64{"key12": 0}, AnnotationBool: map[string]bool{"key13": false}, AnnotationString: map[string]string{"key14": ""}, EvalTargetMetrics: map[string]int64{"total_latency": 9007199254740993}}
	}
	mutations := map[string]func(*ExptTurnResultFilterEntity){
		"space":              func(r *ExptTurnResultFilterEntity) { r.SpaceID++ },
		"expt":               func(r *ExptTurnResultFilterEntity) { r.ExptID++ },
		"item":               func(r *ExptTurnResultFilterEntity) { r.ItemID++ },
		"turn":               func(r *ExptTurnResultFilterEntity) { r.TurnID++ },
		"idx":                func(r *ExptTurnResultFilterEntity) { r.ItemIdx++ },
		"status":             func(r *ExptTurnResultFilterEntity) { r.Status = ItemRunState_Fail },
		"date":               func(r *ExptTurnResultFilterEntity) { r.CreatedDate = r.CreatedDate.AddDate(0, 0, 1) },
		"set":                func(r *ExptTurnResultFilterEntity) { r.EvalSetID++ },
		"version":            func(r *ExptTurnResultFilterEntity) { r.EvalSetVersionID++ },
		"corrected":          func(r *ExptTurnResultFilterEntity) { r.EvaluatorScoreCorrected = false },
		"weight":             func(r *ExptTurnResultFilterEntity) { *r.EvaluatorWeightedScore = 0 },
		"actual_missing":     func(r *ExptTurnResultFilterEntity) { delete(r.EvalTargetData, "actual_output") },
		"ext":                func(r *ExptTurnResultFilterEntity) { r.EvalTargetData["custom"] = "changed" },
		"score_missing_zero": func(r *ExptTurnResultFilterEntity) { delete(r.EvaluatorScore, "key11") },
		"annotation_float":   func(r *ExptTurnResultFilterEntity) { delete(r.AnnotationFloat, "key12") },
		"annotation_bool":    func(r *ExptTurnResultFilterEntity) { delete(r.AnnotationBool, "key13") },
		"annotation_string":  func(r *ExptTurnResultFilterEntity) { delete(r.AnnotationString, "key14") },
		"metrics_precision":  func(r *ExptTurnResultFilterEntity) { r.EvalTargetMetrics["total_latency"]-- },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			want, got := fixture(), fixture()
			if !ExptFilterProjectionEqual(want, got) {
				t.Fatal("identical rows differ")
			}
			mutate(got)
			if ExptFilterProjectionEqual(want, got) {
				t.Fatal("persisted drift was ignored")
			}
		})
	}
	a, b := fixture(), fixture()
	a.EvaluatorWeightedScore = nil
	*b.EvaluatorWeightedScore = 0
	a.AnnotationFloat = nil
	b.AnnotationFloat = map[string]float64{}
	b.ItemVersionID = 100
	b.UpdatedAt = time.Now()
	b.CreatedDate = b.CreatedDate.Add(time.Hour)
	if !ExptFilterProjectionEqual(a, b) {
		t.Fatal("non-persisted fields or equivalent CK values affected equality")
	}
}
