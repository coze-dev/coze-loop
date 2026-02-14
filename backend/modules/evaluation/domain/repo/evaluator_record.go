// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate mockgen -destination mocks/evaluator_record_mock.go -package mocks . IEvaluatorRecordRepo
type IEvaluatorRecordRepo interface {
	CreateEvaluatorRecord(ctx context.Context, evaluatorRecord *entity.EvaluatorRecord) error
	CorrectEvaluatorRecord(ctx context.Context, evaluatorRecordDO *entity.EvaluatorRecord) error
	GetEvaluatorRecord(ctx context.Context, evaluatorRecordID int64, includeDeleted bool) (*entity.EvaluatorRecord, error)
	BatchGetEvaluatorRecord(ctx context.Context, evaluatorRecordIDs []int64, includeDeleted bool) ([]*entity.EvaluatorRecord, error)
	UpdateEvaluatorRecordResult(ctx context.Context, recordID int64, status entity.EvaluatorRunStatus, outputData *entity.EvaluatorOutputData) error
}
