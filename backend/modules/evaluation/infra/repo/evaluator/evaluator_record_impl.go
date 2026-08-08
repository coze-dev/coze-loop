// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"context"

	"gorm.io/gorm"

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

func (r *EvaluatorRecordRepoImpl) BatchGetEvaluatorRecord(ctx context.Context, evaluatorRecordIDs []int64, includeDeleted, withFullContent bool, opts ...entity.GetEvaluatorRecordOptionFn) ([]*entity.EvaluatorRecord, error) {
	opt := &entity.GetEvaluatorRecordOption{}
	for _, fn := range opts {
		fn(opt)
	}
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

	if withFullContent && !opt.WithoutLoadStorageData && r.recordDataStorage != nil {
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

// BatchGetEvaluatorRecordForAggr 聚合专用窄查询: 分批 (batchSize=50) SELECT id, score, status,
// 绕过 input_data/output_data/ext 三个 mediumblob 的查询与反序列化, 只返回 status=Success 且 score 非 NULL 的行。
// 无 TOS 加载、无 ConvertPO2DO 大字段展开, 是评估聚合链路避免大字段全量反序列化 OOM 的核心路径。
func (r *EvaluatorRecordRepoImpl) BatchGetEvaluatorRecordForAggr(ctx context.Context, evaluatorRecordIDs []int64) ([]*entity.EvaluatorRecordAggr, error) {
	const batchSize = 50
	totalIDs := len(evaluatorRecordIDs)
	if totalIDs == 0 {
		return []*entity.EvaluatorRecordAggr{}, nil
	}

	aggrRecords := make([]*entity.EvaluatorRecordAggr, 0, totalIDs)
	for start := 0; start < totalIDs; start += batchSize {
		end := start + batchSize
		if end > totalIDs {
			end = totalIDs
		}

		batchIDs := evaluatorRecordIDs[start:end]
		pos, err := r.evaluatorRecordDao.BatchGetEvaluatorRecordForAggr(ctx, batchIDs)
		if err != nil {
			return nil, err
		}
		for _, po := range pos {
			if aggr := convertor.ConvertEvaluatorRecordPO2AggrDO(po); aggr != nil {
				aggrRecords = append(aggrRecords, aggr)
			}
		}
	}

	return aggrRecords, nil
}

func evaluatorRecordResultValues(outputData *entity.EvaluatorOutputData) (score float64, outputDataStr string) {
	if outputData != nil && outputData.EvaluatorResult != nil && outputData.EvaluatorResult.Score != nil {
		score = *outputData.EvaluatorResult.Score
	}
	if outputData != nil {
		outputDataStr = json.Jsonify(outputData)
	}
	return score, outputDataStr
}

func mergeEvaluatorOutputExt(dst, src *entity.EvaluatorOutputData) *entity.EvaluatorOutputData {
	if dst == nil {
		dst = &entity.EvaluatorOutputData{}
	}
	if src == nil || len(src.Ext) == 0 {
		return dst
	}
	if dst.Ext == nil {
		dst.Ext = make(map[string]string, len(src.Ext))
	}
	for k, v := range src.Ext {
		if _, exists := dst.Ext[k]; !exists {
			dst.Ext[k] = v
		}
	}
	return dst
}

func (r *EvaluatorRecordRepoImpl) UpdateEvaluatorRecordResult(ctx context.Context, recordID int64, status entity.EvaluatorRunStatus, outputData *entity.EvaluatorOutputData) error {
	// This legacy entry point is used by async-zombie termination. Resolve space on the primary and
	// funnel the terminal write through the same AsyncInvoking CAS so a concurrent callback cannot be overwritten.
	po, err := r.evaluatorRecordDao.GetEvaluatorRecord(ctx, recordID, false, db.WithMaster())
	if err != nil || po == nil {
		return err
	}
	_, err = r.CompareAndSwapEvaluatorRecordResult(ctx, recordID, po.SpaceID, entity.EvaluatorRunStatusAsyncInvoking, status, outputData)
	return err
}

func (r *EvaluatorRecordRepoImpl) CompareAndSwapEvaluatorRecordResult(ctx context.Context, recordID, spaceID int64, fromStatus, toStatus entity.EvaluatorRunStatus, outputData *entity.EvaluatorOutputData) (bool, error) {
	if r.dbProvider == nil {
		score, outputDataStr := evaluatorRecordResultValues(outputData)
		rows, err := r.evaluatorRecordDao.CompareAndSwapEvaluatorRecordResult(ctx, recordID, spaceID, int8(fromStatus), int8(toStatus), score, outputDataStr)
		return rows > 0, err
	}
	var updated bool
	err := r.dbProvider.Transaction(ctx, func(tx *gorm.DB) error {
		opt := db.WithTransaction(tx)
		po, err := r.evaluatorRecordDao.GetEvaluatorRecord(ctx, recordID, false, opt, db.WithSelectForUpdate())
		if err != nil || po == nil || po.SpaceID != spaceID || entity.EvaluatorRunStatus(po.Status) != fromStatus {
			return err
		}
		current, err := convertor.ConvertEvaluatorRecordPO2DO(po)
		if err != nil {
			return err
		}
		if current != nil {
			outputData = mergeEvaluatorOutputExt(outputData, current.EvaluatorOutputData)
		}
		score, outputDataStr := evaluatorRecordResultValues(outputData)
		rows, err := r.evaluatorRecordDao.CompareAndSwapEvaluatorRecordResult(ctx, recordID, spaceID, int8(fromStatus), int8(toStatus), score, outputDataStr, opt)
		updated = rows > 0
		return err
	})
	return updated, err
}

func (r *EvaluatorRecordRepoImpl) UpdateEvaluatorRecordAsyncDispatch(ctx context.Context, recordID, spaceID int64, traceID string, outputData *entity.EvaluatorOutputData) error {
	if r.dbProvider == nil {
		_, outputDataStr := evaluatorRecordResultValues(outputData)
		return r.evaluatorRecordDao.UpdateEvaluatorRecordAsyncDispatch(ctx, recordID, spaceID, traceID, outputDataStr)
	}
	return r.dbProvider.Transaction(ctx, func(tx *gorm.DB) error {
		opt := db.WithTransaction(tx)
		po, err := r.evaluatorRecordDao.GetEvaluatorRecord(ctx, recordID, false, opt, db.WithSelectForUpdate())
		if err != nil || po == nil || po.SpaceID != spaceID {
			return err
		}
		current, err := convertor.ConvertEvaluatorRecordPO2DO(po)
		if err != nil {
			return err
		}
		if current != nil {
			outputData = mergeEvaluatorOutputExt(current.EvaluatorOutputData, outputData)
		}
		_, outputDataStr := evaluatorRecordResultValues(outputData)
		return r.evaluatorRecordDao.UpdateEvaluatorRecordAsyncDispatch(ctx, recordID, spaceID, traceID, outputDataStr, opt)
	})
}
