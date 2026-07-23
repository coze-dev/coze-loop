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
	GetEvaluatorRecord(ctx context.Context, evaluatorRecordID int64, includeDeleted bool, opts ...entity.GetEvaluatorRecordOptionFn) (*entity.EvaluatorRecord, error)
	// BatchGetEvaluatorRecord 批量查询 evaluator_version 运行结果，withFullContent 为 true 时从 TOS 加载完整内容
	BatchGetEvaluatorRecord(ctx context.Context, evaluatorRecordIDs []int64, includeDeleted, withFullContent bool, opts ...entity.GetEvaluatorRecordOptionFn) ([]*entity.EvaluatorRecord, error)
	// BatchGetEvaluatorRecordForAggr 聚合专用窄查询：只取 id/score/status，绕过 input_data/output_data/ext
	// 三个大字段的查询与反序列化，只返回 status=Success 且 score 非 NULL 的行（与内存聚合 contributing 集一致）。
	BatchGetEvaluatorRecordForAggr(ctx context.Context, evaluatorRecordIDs []int64) ([]*entity.EvaluatorRecordAggr, error)
	UpdateEvaluatorRecordResult(ctx context.Context, recordID int64, status entity.EvaluatorRunStatus, outputData *entity.EvaluatorOutputData) error
}
