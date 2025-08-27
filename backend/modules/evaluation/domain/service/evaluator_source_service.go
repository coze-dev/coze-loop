// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// EvaluatorSourceService 定义 Evaluator 的 DO 接口
//
//go:generate mockgen -destination mocks/evaluator_source_service_mock.go -package mocks . EvaluatorSourceService
type EvaluatorSourceService interface {
	EvaluatorType() entity.EvaluatorType
	Run(ctx context.Context, evaluator *entity.Evaluator, input *entity.EvaluatorInputData, disableTracing bool) (output *entity.EvaluatorOutputData, runStatus entity.EvaluatorRunStatus, traceID string)
	Debug(ctx context.Context, evaluator *entity.Evaluator, input *entity.EvaluatorInputData) (output *entity.EvaluatorOutputData, err error)
	PreHandle(ctx context.Context, evaluator *entity.Evaluator) error
}