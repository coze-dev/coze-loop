// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import "time"

type ExptTurnResultFilter struct {
	SpaceID                 string    `gorm:"column:space_id"`
	ExptID                  string    `gorm:"column:expt_id"`
	ItemID                  string    `gorm:"column:item_id"`
	ItemIdx                 int32     `gorm:"column:item_idx"`
	TurnID                  string    `gorm:"column:turn_id"`
	Status                  int32     `gorm:"column:status"`
	ActualOutput            *string   `gorm:"column:actual_output"`
	EvaluatorScoreKey1      *float64  `gorm:"column:evaluator_score_key_1"`
	EvaluatorScoreKey2      *float64  `gorm:"column:evaluator_score_key_2"`
	EvaluatorScoreKey3      *float64  `gorm:"column:evaluator_score_key_3"`
	EvaluatorScoreKey4      *float64  `gorm:"column:evaluator_score_key_4"`
	EvaluatorScoreKey5      *float64  `gorm:"column:evaluator_score_key_5"`
	EvaluatorScoreKey6      *float64  `gorm:"column:evaluator_score_key_6"`
	EvaluatorScoreKey7      *float64  `gorm:"column:evaluator_score_key_7"`
	EvaluatorScoreKey8      *float64  `gorm:"column:evaluator_score_key_8"`
	EvaluatorScoreKey9      *float64  `gorm:"column:evaluator_score_key_9"`
	EvaluatorScoreKey10     *float64  `gorm:"column:evaluator_score_key_10"`
	EvaluatorScoreCorrected int32     `gorm:"column:evaluator_score_corrected"`
	EvalSetVersionID        string    `gorm:"column:eval_set_version_id"`
	CreatedDate             time.Time `gorm:"column:created_date"`
	UpdatedAt               time.Time `gorm:"column:updated_at"`
	CreatedAt               time.Time `gorm:"column:created_at"`
}
