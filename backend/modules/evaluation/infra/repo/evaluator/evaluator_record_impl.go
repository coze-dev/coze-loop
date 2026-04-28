// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/storage"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

type EvaluatorRecordRepoImpl struct {
	idgen              idgen.IIDGenerator
	evaluatorRecordDao mysql.EvaluatorRecordDAO
	dbProvider         db.Provider
	recordDataStorage  *storage.RecordDataStorage
}

func NewEvaluatorRecordRepo(idgen idgen.IIDGenerator, provider db.Provider, evaluatorRecordDao mysql.EvaluatorRecordDAO, recordDataStorage *storage.RecordDataStorage) repo.IEvaluatorRecordRepo {
	singletonEvaluatorRecordRepo := &EvaluatorRecordRepoImpl{
		evaluatorRecordDao: evaluatorRecordDao,
		dbProvider:         provider,
		idgen:              idgen,
		recordDataStorage:  recordDataStorage,
	}
	return singletonEvaluatorRecordRepo
}

func (r *EvaluatorRecordRepoImpl) CreateEvaluatorRecord(ctx context.Context, evaluatorRecord *entity.EvaluatorRecord) error {
	if r.recordDataStorage != nil {
		if err := r.recordDataStorage.SaveEvaluatorRecordData(ctx, evaluatorRecord); err != nil {
			return err
		}
	}
	po := convertor.ConvertEvaluatorRecordDO2PO(evaluatorRecord)
	return r.evaluatorRecordDao.CreateEvaluatorRecord(ctx, po)
}

func (r *EvaluatorRecordRepoImpl) CorrectEvaluatorRecord(ctx context.Context, evaluatorRecord *entity.EvaluatorRecord) error {
	if r.recordDataStorage != nil {
		if err := r.recordDataStorage.SaveEvaluatorRecordData(ctx, evaluatorRecord); err != nil {
			return err
		}
	}
	po := convertor.ConvertEvaluatorRecordDO2PO(evaluatorRecord)
	return r.evaluatorRecordDao.UpdateEvaluatorRecord(ctx, po)
}

func (r *EvaluatorRecordRepoImpl) GetEvaluatorRecord(ctx context.Context, evaluatorRecordID int64, includeDeleted bool, opts ...entity.GetEvaluatorRecordOptionFn) (*entity.EvaluatorRecord, error) {
	opt := &entity.GetEvaluatorRecordOption{}
	for _, fn := range opts {
		fn(opt)
	}
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
	if !opt.WithoutLoadStorageData && r.recordDataStorage != nil {
		if err := r.recordDataStorage.LoadEvaluatorRecordData(ctx, evaluatorRecord); err != nil {
			return nil, err
		}
	}
	return evaluatorRecord, nil
}

func (r *EvaluatorRecordRepoImpl) BatchGetEvaluatorRecord(ctx context.Context, evaluatorRecordIDs []int64, includeDeleted, withFullContent bool) ([]*entity.EvaluatorRecord, error) {
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
			// BatchGet 用于列表/批量场景，返回 MySQL 中已裁剪的 evaluator_input_data 预览，不加载 TOS 完整内容
			// 完整内容需通过 GetEvaluatorRecord 单条查询获取
			evaluatorRecords = append(evaluatorRecords, evaluatorRecord)
		}
	}

	if withFullContent && r.recordDataStorage != nil {
		for _, record := range evaluatorRecords {
			if record != nil {
				if err := r.recordDataStorage.LoadEvaluatorRecordData(ctx, record); err != nil {
					return nil, err
				}
			}
		}
	}

	return evaluatorRecords, nil
}

func (r *EvaluatorRecordRepoImpl) UpdateEvaluatorRecordResult(ctx context.Context, recordID int64, status entity.EvaluatorRunStatus, outputData *entity.EvaluatorOutputData) error {
	var score float64
	if outputData != nil && outputData.EvaluatorResult != nil && outputData.EvaluatorResult.Score != nil {
		score = *outputData.EvaluatorResult.Score
	}

	var outputDataStr string
	if outputData != nil {
		outputDataStr = json.Jsonify(outputData)
	}

	return r.evaluatorRecordDao.UpdateEvaluatorRecordResult(ctx, recordID, int8(status), score, outputDataStr)
}
