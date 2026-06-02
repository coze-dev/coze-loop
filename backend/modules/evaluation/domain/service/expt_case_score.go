// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"errors"
)

// CaseScoreItem 为单个评估器在该行的得分，对应 score_api.md 中 /score/case 的 evaluator_score 元素，
// 并额外携带 ExptID/EvaluatorID/EvaluatorVersionID 信息。
type CaseScoreItem struct {
	EvaluatorName      string  `json:"evaluator_name"`
	EvaluatorID        int64   `json:"evaluator_id"`
	EvaluatorVersionID int64   `json:"evaluator_version_id"`
	Score              float64 `json:"score"`
}

// CaseScoreRequest 为 /score/case 的请求体。
type CaseScoreRequest struct {
	ExptID         int64            `json:"expt_id"`
	EvaluatorScore []*CaseScoreItem `json:"evaluator_score"`
}

// ErrCaseScorerNotImplemented 表示 HTTP 打分能力在当前（开源）构建中未实现。
var ErrCaseScorerNotImplemented = errors.New("case scorer http call not implemented")
