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
	UpdateEvaluatorRecordResult(ctx context.Context, recordID int64, status entity.EvaluatorRunStatus, outputData *entity.EvaluatorOutputData) error
	// TerminateAsyncInvokingByExptRunItems 将指定实验 run 下行内仍处于异步执行中的评测器记录置为失败（如行级僵尸超时），避免后续重试仍复用旧 invoke。
	TerminateAsyncInvokingByExptRunItems(ctx context.Context, spaceID, exptID, exptRunID int64, itemIDs []int64, failOutput *entity.EvaluatorOutputData) error
}
