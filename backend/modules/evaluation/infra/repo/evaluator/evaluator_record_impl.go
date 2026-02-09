// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"context"

	evaluatorredis "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/redis/dao"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/convertor"
)

type EvaluatorRecordRepoImpl struct {
	idgen                idgen.IIDGenerator
	evaluatorRecordDao   mysql.EvaluatorRecordDAO
	dbProvider           db.Provider
	evaluatorProgressDao evaluatorredis.IEvaluatorProgressDAO
}

func NewEvaluatorRecordRepo(idgen idgen.IIDGenerator, provider db.Provider, evaluatorRecordDao mysql.EvaluatorRecordDAO, evaluatorProgressDao evaluatorredis.IEvaluatorProgressDAO) repo.IEvaluatorRecordRepo {
	singletonEvaluatorRecordRepo := &EvaluatorRecordRepoImpl{
		evaluatorRecordDao:   evaluatorRecordDao,
		dbProvider:           provider,
		idgen:                idgen,
		evaluatorProgressDao: evaluatorProgressDao,
	}
	return singletonEvaluatorRecordRepo
}

func (r *EvaluatorRecordRepoImpl) CreateEvaluatorRecord(ctx context.Context, evaluatorRecord *entity.EvaluatorRecord) error {
	po := convertor.ConvertEvaluatorRecordDO2PO(evaluatorRecord)
	return r.evaluatorRecordDao.CreateEvaluatorRecord(ctx, po)
}

func (r *EvaluatorRecordRepoImpl) CorrectEvaluatorRecord(ctx context.Context, evaluatorRecord *entity.EvaluatorRecord) error {
	po := convertor.ConvertEvaluatorRecordDO2PO(evaluatorRecord)
	return r.evaluatorRecordDao.UpdateEvaluatorRecord(ctx, po)
}

func (r *EvaluatorRecordRepoImpl) GetEvaluatorRecord(ctx context.Context, evaluatorRecordID int64, includeDeleted bool) (*entity.EvaluatorRecord, error) {
	po, err := r.evaluatorRecordDao.GetEvaluatorRecord(ctx, evaluatorRecordID, includeDeleted)
	if err != nil {
		return nil, err
	}
	if po == nil {
		return nil, nil
	}
	evaluatorRecord, err := convertor.ConvertEvaluatorRecordPO2DO(po)
	if err != nil {
		return nil, err
	}

	return evaluatorRecord, nil
}

func (r *EvaluatorRecordRepoImpl) BatchGetEvaluatorRecord(ctx context.Context, evaluatorRecordIDs []int64, includeDeleted bool) ([]*entity.EvaluatorRecord, error) {
	const batchSize = 50
	totalIDs := len(evaluatorRecordIDs)
	if totalIDs == 0 {
		return []*entity.EvaluatorRecord{}, nil
	}

	evaluatorRecords := make([]*entity.EvaluatorRecord, 0, totalIDs)

	for start := 0; start < totalIDs; start += batchSize {
		end := start + batchSize
		if end > totalIDs {
			end = totalIDs
		}

		batchIDs := evaluatorRecordIDs[start:end]
		pos, err := r.evaluatorRecordDao.BatchGetEvaluatorRecord(ctx, batchIDs, includeDeleted)
		if err != nil {
			return nil, err
		}

		for _, po := range pos {
			evaluatorRecord, err := convertor.ConvertEvaluatorRecordPO2DO(po)
			if err != nil {
				return nil, err
			}
			evaluatorRecords = append(evaluatorRecords, evaluatorRecord)
		}
	}

	return evaluatorRecords, nil
}

func (r *EvaluatorRecordRepoImpl) UpdateEvaluatorRecordResult(ctx context.Context, recordID int64, status entity.EvaluatorRunStatus, outputData *entity.EvaluatorOutputData) error {
	record := &entity.EvaluatorRecord{
		ID:                  recordID,
		EvaluatorOutputData: outputData,
		Status:              status,
	}
	po := convertor.ConvertEvaluatorRecordDO2PO(record)
	if po == nil {
		return nil
	}
	return r.evaluatorRecordDao.UpdateEvaluatorRecord(ctx, po)
}

func (r *EvaluatorRecordRepoImpl) RPushEvaluatorProgress(ctx context.Context, invokeID int64, messages []*entity.EvaluatorProgressMessage) error {
	return r.evaluatorProgressDao.RPushEvaluatorProgress(ctx, invokeID, messages)
}
