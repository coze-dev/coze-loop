// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"sync"

	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
)

// EvaluatorRecordDAO 定义 EvaluatorRecord 的 Dao 接口
//
//go:generate mockgen -destination mocks/evaluator_record_mock.go -package=mocks . EvaluatorRecordDAO
type EvaluatorRecordDAO interface {
	CreateEvaluatorRecord(ctx context.Context, evaluatorRecord *model.EvaluatorRecord, opts ...db.Option) error
	UpdateEvaluatorRecord(ctx context.Context, evaluatorRecord *model.EvaluatorRecord, opts ...db.Option) error
	UpdateEvaluatorRecordResult(ctx context.Context, recordID int64, status int8, score *float64, outputData string, opts ...db.Option) error
	GetEvaluatorRecord(ctx context.Context, evaluatorRecordID int64, includeDeleted bool, opts ...db.Option) (*model.EvaluatorRecord, error)
	BatchGetEvaluatorRecord(ctx context.Context, evaluatorRecordIDs []int64, includeDeleted bool, opts ...db.Option) ([]*model.EvaluatorRecord, error)
	// BatchGetEvaluatorRecordForAggr 聚合专用窄查询: 只 SELECT id, score, status, 不取 input_data/output_data/ext
	// 三个 mediumblob, 且只返回 status=Success 且 score 非 NULL 的行 (与内存聚合的 contributing 集一致)。
	BatchGetEvaluatorRecordForAggr(ctx context.Context, evaluatorRecordIDs []int64, opts ...db.Option) ([]*model.EvaluatorRecord, error)
}

var (
	evaluatorRecordDaoOnce      = sync.Once{}
	singletonEvaluatorRecordDao EvaluatorRecordDAO
)

type EvaluatorRecordDAOImpl struct {
	provider db.Provider
}

func NewEvaluatorRecordDAO(p db.Provider) EvaluatorRecordDAO {
	evaluatorRecordDaoOnce.Do(func() {
		singletonEvaluatorRecordDao = &EvaluatorRecordDAOImpl{
			provider: p,
		}
	})
	return singletonEvaluatorRecordDao
}

func (dao *EvaluatorRecordDAOImpl) CreateEvaluatorRecord(ctx context.Context, evaluatorRecord *model.EvaluatorRecord, opts ...db.Option) error {
	// 通过opts获取当前的db session实例
	dbsession := dao.provider.NewSession(ctx, opts...)

	return dbsession.WithContext(ctx).Create(evaluatorRecord).Error
}

func (dao *EvaluatorRecordDAOImpl) UpdateEvaluatorRecord(ctx context.Context, evaluatorRecord *model.EvaluatorRecord, opts ...db.Option) error {
	if evaluatorRecord == nil {
		// FIXME: errno
		// return errno.New(experiment.EvaluatorRecordNotFoundCode)
		return errors.New("evaluation.EvaluatorRecordNotFoundCode")
	}

	// 通过opts获取当前的db session实例
	dbsession := dao.provider.NewSession(ctx, opts...)

	return dbsession.WithContext(ctx).
		Model(&model.EvaluatorRecord{}).
		Where("id = ? AND deleted_at IS NULL", evaluatorRecord.ID).
		Save(evaluatorRecord).Error
}

func (dao *EvaluatorRecordDAOImpl) UpdateEvaluatorRecordResult(ctx context.Context, recordID int64, status int8, score *float64, outputData string, opts ...db.Option) error {
	dbsession := dao.provider.NewSession(ctx, opts...)

	// score 传 nil 时写 NULL(而非 0): 失败/无有效分数的 record 不应在 score 列留下 0,
	// 否则聚合窄查询 (status=Success AND score IS NOT NULL) 无法区分"真 0 分"与"无分数", 会把无分数误算进均值/分布。
	return dbsession.WithContext(ctx).
		Model(&model.EvaluatorRecord{}).
		Where("id = ? AND deleted_at IS NULL", recordID).
		Updates(map[string]interface{}{
			"status":      status,
			"score":       score,
			"output_data": outputData,
		}).Error
}

func (dao *EvaluatorRecordDAOImpl) GetEvaluatorRecord(ctx context.Context, evaluatorRecordID int64, includeDeleted bool, opts ...db.Option) (*model.EvaluatorRecord, error) {
	po := &model.EvaluatorRecord{}

	// 通过opts获取当前的db session实例
	dbsession := dao.provider.NewSession(ctx, opts...)

	query := dbsession.WithContext(ctx).Where("id = ?", evaluatorRecordID)
	if includeDeleted {
		query = query.Unscoped() // 解除软删除过滤
	}
	err := query.First(po).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return po, nil
}

func (dao *EvaluatorRecordDAOImpl) BatchGetEvaluatorRecord(ctx context.Context, evaluatorRecordIDs []int64, includeDeleted bool, opts ...db.Option) ([]*model.EvaluatorRecord, error) {
	var pos []*model.EvaluatorRecord

	// 通过opts获取当前的db session实例
	dbsession := dao.provider.NewSession(ctx, opts...)

	query := dbsession.WithContext(ctx).Where("id IN (?)", evaluatorRecordIDs)
	if contexts.CtxWriteDB(ctx) {
		// 使用 FOR UPDATE 语句，强制使用写库
		query = query.Clauses(dbresolver.Write)
	}
	if includeDeleted {
		query = query.Unscoped() // 解除软删除过滤
	}
	err := query.Find(&pos).Error
	if err != nil {
		return nil, err
	}
	return pos, nil
}

// BatchGetEvaluatorRecordForAggr 聚合专用窄查询。
//
// 只 SELECT id, score, status 三列, 完全跳过 input_data/output_data/ext 三个 mediumblob:
// 单条 input_data 可达 190KB, 大实验 (数万条) 全量取回 + json.Unmarshal 曾把评估消费侧内存
// 顶到 GB 级触发 SIGKILL, 本方法把单次聚合内存降到数量级更低。
//
// WHERE 过滤条件与内存聚合的 contributing 集严格对齐:
//   - status = Success(1): 只有成功的 record 才在内存路径贡献分数; Skipped(4)/Fail(2)/AsyncInvoking(3)
//     在内存路径因 nil EvaluatorResult / 显式 Skipped 过滤被跳过。注意 async 僵尸失败会把 score 列写成 0
//     (见 UpdateEvaluatorRecordResult), 若只按 status != Skipped 过滤会把这些 0 分误算进聚合, 故必须白名单 Success。
//   - score IS NOT NULL: 防御性兜底, 对齐旧内存路径 "取不到有效分数则跳过" 的语义。
func (dao *EvaluatorRecordDAOImpl) BatchGetEvaluatorRecordForAggr(ctx context.Context, evaluatorRecordIDs []int64, opts ...db.Option) ([]*model.EvaluatorRecord, error) {
	var pos []*model.EvaluatorRecord

	dbsession := dao.provider.NewSession(ctx, opts...)

	query := dbsession.WithContext(ctx).
		Model(&model.EvaluatorRecord{}).
		Select("id", "score", "status").
		Where("id IN (?)", evaluatorRecordIDs).
		Where("status = ?", int32(entity.EvaluatorRunStatusSuccess)).
		Where("score IS NOT NULL")
	if contexts.CtxWriteDB(ctx) {
		query = query.Clauses(dbresolver.Write)
	}
	err := query.Find(&pos).Error
	if err != nil {
		return nil, err
	}
	return pos, nil
}
