// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
)

type AnalysisType = string

const (
	AnalysisTypeInsight    = "insight"
	AnalysisTypeTrajectory = "trajectory"
)

type AnalysisRecordRef struct {
	ID                         int64            `json:"id"`
	WorkspaceID                int64            `json:"workspace_id"`
	UniqueKey                  string           `json:"unique_key"`
	TrajectoryAnalysisRecordID int64            `json:"trajectory_analysis_record_id"`
	BaseInfo                   *common.BaseInfo `json:"base_info,omitempty"`
}

type AnalysisRecord struct {
	ID          int64                 `json:"id"`
	WorkspaceID int64                 `json:"workspace_id"`
	Type        AnalysisType          `json:"type,omitempty"`
	Status      InsightAnalysisStatus `json:"status,omitempty"`

	// type=trajectory的metaInfo、result填充在此
	TrajectoryMetaInfo       *TrajectoryMetaInfo       `json:"trajectory_meta_info,omitempty"`
	TrajectoryAnalysisResult *TrajectoryAnalysisResult `json:"result,omitempty"`
	BaseInfo                 *common.BaseInfo          `json:"base_info,omitempty"`
}

type TrajectoryMetaInfo struct {
	ExptID                 int64                    `json:"expt_id,omitempty"`
	ItemID                 int64                    `json:"item_id,omitempty"`
	TurnID                 int64                    `json:"turn_id,omitempty"`
	Trajectory             *Trajectory              `json:"trajectory,omitempty"`
	EvaluatorAnalysisInfos []*EvaluatorAnalysisInfo `json:"evaluator_analysis_infos,omitempty"`
}

type EvaluatorAnalysisInfo struct {
	EvaluatorID     int64            `json:"evaluator_id,omitempty"`
	VersionID       int64            `json:"version_id,omitempty"`
	Name            string           `json:"name,omitempty"`
	Description     string           `json:"description,omitempty"`
	EvaluatorResult *EvaluatorResult `json:"evaluator_result,omitempty"`
}

type TrajectoryAnalysisResult struct {
	RootResult   *RootTrajectoryAnalysisResult    `json:"root_result,omitempty"`
	AgentResults []*AgentTrajectoryAnalysisResult `json:"agent_results,omitempty"`
}

type RootTrajectoryAnalysisResult struct {
	StepID  string                     `json:"step_id,omitempty"`
	Status  InsightAnalysisStatus      `json:"status,omitempty"`
	Summary string                     `json:"summary,omitempty"`
	Issues  []*TrajectoryAnalysisIssue `json:"issues,omitempty"`
}

type AgentTrajectoryAnalysisResult struct {
	StepID  string                     `json:"step_id,omitempty"`
	Status  InsightAnalysisStatus      `json:"status,omitempty"`
	Summary string                     `json:"summary,omitempty"`
	Issues  []*TrajectoryAnalysisIssue `json:"issues,omitempty"`
}

type TrajectoryAnalysisIssue struct {
	Type       string `json:"type,omitempty"`
	Desc       string `json:"desc,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

type AnalysisEvent struct {
	RecordID int64 `json:"record_id"`
}
