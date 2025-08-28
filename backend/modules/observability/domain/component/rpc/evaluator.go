// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import "context"

type Evaluator struct {
	EvaluatorVersionID int64
	EvaluatorName      string
	EvaluatorVersion   string
}

type BatchGetEvaluatorVersionsParam struct {
	WorkspaceID         int64
	EvaluatorVersionIds []int64
}
type UpdateEvaluatorRecordParam struct {
	WorkspaceID       string
	EvaluatorRecordID int64
	Score             float64
	Reasoning         string
	UpdatedBy         string
}

//go:generate mockgen -destination=mocks/evaluator.go -package=mocks . IEvaluatorRPCAdapter
type IEvaluatorRPCAdapter interface {
	BatchGetEvaluatorVersions(ctx context.Context, param *BatchGetEvaluatorVersionsParam) ([]*Evaluator, map[int64]*Evaluator, error)
	UpdateEvaluatorRecord(ctx context.Context, param *UpdateEvaluatorRecordParam) error
}
