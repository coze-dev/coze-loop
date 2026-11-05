// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "maps"

type ExptFilterTarget string

const (
	ExptFilterTargetLegacy ExptFilterTarget = "legacy"
	ExptFilterTargetV2     ExptFilterTarget = "v2"
)

type ExptFilterOperation string

const (
	ExptFilterCurrent    ExptFilterOperation = "current"
	ExptFilterHistorical ExptFilterOperation = "historical"
)

type ExptFilterScope struct {
	SpaceID int64 `json:"space_id"`
	ExptID  int64 `json:"expt_id"`
}

type ExptFilterKey struct {
	SpaceID int64 `json:"space_id"`
	ExptID  int64 `json:"expt_id"`
	ItemID  int64 `json:"item_id"`
	TurnID  int64 `json:"turn_id"`
}

type ExptFilterPlan struct {
	WriteMode       string
	ReadTarget      ExptFilterTarget
	RequiredTargets []ExptFilterTarget
	ExptIDs         []int64
	ConfigHash      string
}

func (p *ExptFilterPlan) Clone() *ExptFilterPlan {
	if p == nil {
		return nil
	}
	copy := *p
	copy.RequiredTargets = append([]ExptFilterTarget(nil), p.RequiredTargets...)
	copy.ExptIDs = append([]int64(nil), p.ExptIDs...)
	return &copy
}

type ExptFilterRow struct {
	Filter           *ExptTurnResultFilterEntity
	WriteTimestampMS *uint64
}

type ExptFilterReadQuery struct {
	Scope       ExptFilterScope
	CreatedDate string
	ItemIDs     []int64
}

type ExptFilterStoredKey struct {
	Key         ExptFilterKey `json:"key"`
	CreatedDate string        `json:"created_date"`
}

type ExptFilterTargetCursor struct {
	CreatedDate string
	SpaceID     string
	ExptID      string
	ItemID      string
	TurnID      string
}

type ExptFilterScanBounds struct {
	MaxExptID int64 `json:"max_expt_id"`
}

type ExptFilterExperimentPage struct {
	Scopes     []ExptFilterScope
	NextExptID int64
}

// ExptFilterProjectionEqual compares the persisted business projection, not write ordering.
func ExptFilterProjectionEqual(a, b *ExptTurnResultFilterEntity) bool {
	if a == nil || b == nil {
		return a == b
	}
	weight := func(v *float64) float64 {
		if v == nil {
			return 0
		}
		return *v
	}
	return a.SpaceID == b.SpaceID && a.ExptID == b.ExptID && a.ItemID == b.ItemID && a.TurnID == b.TurnID &&
		a.ItemIdx == b.ItemIdx && a.Status == b.Status && a.CreatedDate.Format("2006-01-02") == b.CreatedDate.Format("2006-01-02") &&
		a.EvalSetID == b.EvalSetID && a.EvalSetVersionID == b.EvalSetVersionID &&
		a.EvaluatorScoreCorrected == b.EvaluatorScoreCorrected && weight(a.EvaluatorWeightedScore) == weight(b.EvaluatorWeightedScore) &&
		maps.Equal(a.EvalTargetData, b.EvalTargetData) && maps.Equal(a.EvaluatorScore, b.EvaluatorScore) &&
		maps.Equal(a.AnnotationFloat, b.AnnotationFloat) && maps.Equal(a.AnnotationBool, b.AnnotationBool) &&
		maps.Equal(a.AnnotationString, b.AnnotationString) && maps.Equal(a.EvalTargetMetrics, b.EvalTargetMetrics)
}
