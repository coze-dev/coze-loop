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
	Run(ctx context.Context, evaluator *entity.Evaluator, input *entity.EvaluatorInputData, evaluatorRunConf *entity.EvaluatorRunConfig, exptSpaceID int64, disableTracing bool) (output *entity.EvaluatorOutputData, runStatus entity.EvaluatorRunStatus, traceID string)
	AsyncRun(ctx context.Context, evaluator *entity.Evaluator, input *entity.EvaluatorInputData, evaluatorRunConf *entity.EvaluatorRunConfig, exptSpaceID int64, invokeID int64) error
	GetAsyncRunResult(ctx context.Context, spaceID int64, invokeID int64) (*entity.EvaluatorOutputData, entity.EvaluatorRunStatus, error)
	Debug(ctx context.Context, evaluator *entity.Evaluator, input *entity.EvaluatorInputData, evaluatorRunConf *entity.EvaluatorRunConfig, exptSpaceID int64) (output *entity.EvaluatorOutputData, err error)
	AsyncDebug(ctx context.Context, evaluator *entity.Evaluator, input *entity.EvaluatorInputData, evaluatorRunConf *entity.EvaluatorRunConfig, exptSpaceID int64, invokeID int64) error
	GetAsyncDebugResult(ctx context.Context, spaceID int64, invokeID int64) (*entity.GetAsyncDebugEvaluatorInvokeResultResponse, error)
	PreHandle(ctx context.Context, evaluator *entity.Evaluator) error
	// Validate 验证评估器
	Validate(ctx context.Context, evaluator *entity.Evaluator) error
}
