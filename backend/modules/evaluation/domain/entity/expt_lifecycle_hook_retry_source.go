// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"maps"
)

// Retry source is private provenance for the selected projection, never another Run's writable state.
type HookExecutionRetrySource struct {
	RunID            int64                    `json:"run_id"`
	Ordinal          int64                    `json:"ordinal"`
	ItemConfig       json.RawMessage          `json:"item_config"`
	Turns            []HookExecutionRetryTurn `json:"turns"`
	ReusedTargets    map[int64]int64          `json:"reused_targets,omitempty"`
	ReusedEvaluators map[int64]int64          `json:"reused_evaluators,omitempty"`
}

type HookExecutionRetryTurn struct {
	RunID          int64                         `json:"run_id"`
	TurnID         int64                         `json:"turn_id"`
	ResultID       int64                         `json:"result_id"`
	TargetResultID int64                         `json:"target_result_id"`
	Refs           []*ExptTurnEvaluatorResultRef `json:"refs,omitempty"`
}

func (s *HookExecutionRetrySource) Clone() *HookExecutionRetrySource {
	if s == nil {
		return nil
	}
	out := *s
	out.ItemConfig = append(json.RawMessage(nil), s.ItemConfig...)
	out.Turns = append([]HookExecutionRetryTurn(nil), s.Turns...)
	for i := range out.Turns {
		out.Turns[i].Refs = make([]*ExptTurnEvaluatorResultRef, len(s.Turns[i].Refs))
		for j, ref := range s.Turns[i].Refs {
			if ref != nil {
				copy := *ref
				out.Turns[i].Refs[j] = &copy
			}
		}
	}
	out.ReusedTargets = maps.Clone(s.ReusedTargets)
	out.ReusedEvaluators = maps.Clone(s.ReusedEvaluators)
	return &out
}

func (m HookExecutionManifest) ProjectionOrdinal() int64 {
	if m.Retry != nil {
		return m.Retry.Ordinal
	}
	return m.Ordinal
}
func (m HookExecutionManifest) TargetRecordRun(id int64) int64 {
	if m.Retry != nil && m.Retry.ReusedTargets[id] > 0 {
		return m.Retry.ReusedTargets[id]
	}
	return m.Key.RunID
}
func (m HookExecutionManifest) EvaluatorRecordRun(id int64) int64 {
	if m.Retry != nil && m.Retry.ReusedEvaluators[id] > 0 {
		return m.Retry.ReusedEvaluators[id]
	}
	return m.Key.RunID
}
