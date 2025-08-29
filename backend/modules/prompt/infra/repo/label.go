// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	metricsinfra "github.com/coze-dev/coze-loop/backend/modules/prompt/infra/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/redis"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type LabelRepoImpl struct {
	db                             db.Provider
	idgen                          idgen.IIDGenerator
	labelDAO                       mysql.ILabelDAO
	commitLabelMappingDAO          mysql.ICommitLabelMappingDAO
	promptBasicDAO                 mysql.IPromptBasicDAO
	promptLabelVersionDAO          redis.IPromptLabelVersionDAO
	promptLabelVersionCacheMetrics *metricsinfra.PromptLabelVersionCacheMetrics
}

func NewLabelRepo(
	db db.Provider,
	idgen idgen.IIDGenerator,
	meter metrics.Meter,
	labelDAO mysql.ILabelDAO,
	commitLabelMappingDAO mysql.ICommitLabelMappingDAO,
	promptBasicDAO mysql.IPromptBasicDAO,
	promptLabelVersionDAO redis.IPromptLabelVersionDAO,
) repo.ILabelRepo {
	return &LabelRepoImpl{
		db:                             db,
		idgen:                          idgen,
		labelDAO:                       labelDAO,
		commitLabelMappingDAO:          commitLabelMappingDAO,
		promptBasicDAO:                 promptBasicDAO,
		promptLabelVersionDAO:          promptLabelVersionDAO,
		promptLabelVersionCacheMetrics: metricsinfra.NewPromptLabelVersionCacheMetrics(meter),
	}
}

func (r *LabelRepoImpl) CreateLabel(ctx context.Context, labelDO *entity.PromptLabel) error {
	if labelDO == nil {
		return nil
	}

	// 生成ID
	id, err := r.idgen.GenID(ctx)
	if err != nil {
		return err
	}

	// 转换为PO
	labelPO := convertor.PromptLabelDO2PO(labelDO)
	labelPO.ID = id

	// 调用DAO创建
	return r.labelDAO.Create(ctx, labelPO)
}

func (r *LabelRepoImpl) ListLabel(ctx context.Context, param repo.ListLabelParam) ([]*entity.PromptLabel, *int64, error) {
	// 构建DAO参数
	daoParam := mysql.ListLabelDAOParam{
		SpaceID:      param.SpaceID,
		LabelKeyLike: param.LabelKeyLike,
		Limit:        param.PageSize + 1, // 多查一个判断是否有下一页
	}

	if param.PageToken != nil {
		daoParam.Cursor = param.PageToken
	}

	// 查询数据
	labelPOs, err := r.labelDAO.List(ctx, daoParam)
	if err != nil {
		return nil, nil, err
	}

	// 处理分页
	var nextPageToken *int64
	if len(labelPOs) > param.PageSize {
		// 有下一页
		if len(labelPOs) > 0 {
			nextPageToken = &labelPOs[len(labelPOs)-1].ID
		}
		labelPOs = labelPOs[:param.PageSize]
	}

	// 转换为DO
	labelDOs := convertor.BatchPromptLabelPO2DO(labelPOs)
	return labelDOs, nextPageToken, nil
}

func (r *LabelRepoImpl) BatchGetLabel(ctx context.Context, spaceID int64, labelKeys []string) (labelDOs []*entity.PromptLabel, err error) {
	if len(labelKeys) == 0 {
		return nil, nil
	}

	// 调用DAO
	labelPOs, err := r.labelDAO.BatchGet(ctx, spaceID, labelKeys)
	if err != nil {
		return nil, err
	}

	// 转换为DO
	return convertor.BatchPromptLabelPO2DO(labelPOs), nil
}

func (r *LabelRepoImpl) UpdateCommitLabels(ctx context.Context, param repo.UpdateCommitLabelsParam) error {
	// 构建需要删除的缓存查询参数
	var cacheQueries []redis.PromptLabelVersionQuery
	// 删除当前传入的所有label的缓存
	for _, labelKey := range param.LabelKeys {
		cacheQueries = append(cacheQueries, redis.PromptLabelVersionQuery{
			PromptID: param.PromptID,
			LabelKey: labelKey,
		})
	}
	// 在事务中执行所有操作
	err := r.db.Transaction(ctx, func(tx *gorm.DB) error {
		opt := db.WithTransaction(tx)

		// 1. 首先对prompt加锁，确保并发安全
		_, err := r.promptBasicDAO.Get(ctx, param.PromptID, opt, db.WithSelectForUpdate())
		if err != nil {
			return err
		}

		// 2. 根据prompt_id和label_keys查询现有的标签映射
		labelExistMappings, err := r.commitLabelMappingDAO.ListByPromptIDAndLabelKeys(ctx, param.PromptID, param.LabelKeys, opt)
		if err != nil {
			return err
		}

		existingLabelMappings := make(map[string]*model.PromptCommitLabelMapping)
		for _, mapping := range labelExistMappings {
			existingLabelMappings[mapping.LabelKey] = mapping
		}

		// 2. 需要创建的映射
		var toCreate []*model.PromptCommitLabelMapping
		ids, err := r.idgen.GenMultiIDs(ctx, len(param.LabelKeys))
		if err != nil {
			return err
		}
		for i, labelKey := range param.LabelKeys {
			if _, exists := existingLabelMappings[labelKey]; !exists {
				mappingPO := &model.PromptCommitLabelMapping{
					ID:            ids[i],
					SpaceID:       param.SpaceID,
					PromptID:      param.PromptID,
					LabelKey:      labelKey,
					PromptVersion: param.CommitVersion,
					CreatedBy:     param.UpdatedBy,
					UpdatedBy:     param.UpdatedBy,
				}
				toCreate = append(toCreate, mappingPO)
			}
		}

		// 3. 需要更新的映射
		newLabelKeys := make(map[string]bool)
		for _, labelKey := range param.LabelKeys {
			newLabelKeys[labelKey] = true
		}
		var toUpdate []*model.PromptCommitLabelMapping
		for labelKey, mapping := range existingLabelMappings {
			if newLabelKeys[labelKey] {
				// 需要更新的映射
				mapping.PromptVersion = param.CommitVersion
				mapping.UpdatedBy = param.UpdatedBy
				toUpdate = append(toUpdate, mapping)
			}
		}

		// 4. 需要删除的映射
		var toDeleteMappingIDs []int64
		versionExistMappings, err := r.commitLabelMappingDAO.ListByPromptIDAndVersions(ctx, param.PromptID, []string{param.CommitVersion})
		if err != nil {
			return err
		}
		for _, mapping := range versionExistMappings {
			if !newLabelKeys[mapping.LabelKey] {
				toDeleteMappingIDs = append(toDeleteMappingIDs, mapping.ID)

				// 需要删除的缓存
				cacheQueries = append(cacheQueries, redis.PromptLabelVersionQuery{
					PromptID: param.PromptID,
					LabelKey: mapping.LabelKey,
				})
			}
		}

		// 4. 执行数据库操作
		if len(toCreate) > 0 {
			err = r.commitLabelMappingDAO.BatchCreate(ctx, toCreate, opt)
			if err != nil {
				return err
			}
		}

		if len(toUpdate) > 0 {
			err = r.commitLabelMappingDAO.BatchUpdate(ctx, toUpdate, opt)
			if err != nil {
				return err
			}
		}

		if len(toDeleteMappingIDs) > 0 {
			err = r.commitLabelMappingDAO.BatchDelete(ctx, toDeleteMappingIDs, opt)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	// 执行缓存删除，失败不影响主流程
	if len(cacheQueries) > 0 {
		err = r.promptLabelVersionDAO.MDel(ctx, cacheQueries)
		if err != nil {
			logs.CtxError(ctx, "failed to delete cache, err: %v", err)
		}
	}

	return nil
}

func (r *LabelRepoImpl) GetCommitLabels(ctx context.Context, promptID int64, commitVersions []string) (map[string][]*entity.PromptLabel, error) {
	// 调用DAO
	mappingPOs, err := r.commitLabelMappingDAO.ListByPromptIDAndVersions(ctx, promptID, commitVersions)
	if err != nil {
		return nil, err
	}
	commitLabels := make(map[string][]*entity.PromptLabel)
	for _, mappingPO := range mappingPOs {
		commitLabels[mappingPO.PromptVersion] = append(commitLabels[mappingPO.PromptVersion], &entity.PromptLabel{
			LabelKey: mappingPO.LabelKey,
		})
	}
	return commitLabels, nil
}

func (r *LabelRepoImpl) BatchGetPromptVersionByLabel(ctx context.Context, queries []repo.PromptLabelQuery, opts ...repo.GetLabelMappingOptionFunc) (map[repo.PromptLabelQuery]string, error) {
	// 处理options
	options := &repo.GetLabelMappingOption{}
	for _, opt := range opts {
		opt(options)
	}

	result := make(map[repo.PromptLabelQuery]string)
	var missedQueries []repo.PromptLabelQuery

	var hitNum, missNum int

	// 1. 如果启用缓存，先从缓存中获取
	if options.CacheEnable {
		redisQueries := make([]redis.PromptLabelVersionQuery, len(queries))
		for i, query := range queries {
			redisQueries[i] = redis.PromptLabelVersionQuery{
				PromptID: query.PromptID,
				LabelKey: query.LabelKey,
			}
		}

		versionMap, err := r.promptLabelVersionDAO.MGet(ctx, redisQueries)
		if err != nil {
			logs.CtxError(ctx, "[BatchGetPromptVersionByLabel] get from cache failed: %v", err)
		}

		// 处理缓存结果
		for _, query := range queries {
			redisQuery := redis.PromptLabelVersionQuery{
				PromptID: query.PromptID,
				LabelKey: query.LabelKey,
			}

			if version, exists := versionMap[redisQuery]; exists && version != "" {
				// 缓存命中
				hitNum++
				result[query] = version
			} else {
				// 缓存未命中，需要从数据库查询
				missNum++
				missedQueries = append(missedQueries, query)
			}
		}

		// 2. 发送缓存命中率 metrics
		r.promptLabelVersionCacheMetrics.MEmit(ctx, metricsinfra.PromptLabelVersionCacheMetricsParam{
			HitNum:  hitNum,
			MissNum: missNum,
		})
	} else {
		// 不启用缓存，直接查询数据库
		missedQueries = queries
	}

	// 2. 从数据库获取缓存未命中的数据
	if len(missedQueries) > 0 {
		// 构建DAO参数
		daoParam := mysql.MGetPromptVersionByLabelQueryParam{
			Queries: make([]mysql.PromptLabelQuery, len(missedQueries)),
		}

		for i, query := range missedQueries {
			daoParam.Queries[i] = mysql.PromptLabelQuery{
				PromptID: query.PromptID,
				LabelKey: query.LabelKey,
			}
		}

		// 调用DAO方法
		mappingPOs, err := r.commitLabelMappingDAO.MGetPromptVersionByLabelQuery(ctx, daoParam)
		if err != nil {
			return nil, err
		}

		// 处理数据库结果
		var cacheMappings []redis.PromptLabelVersionMapping
		for _, mapping := range mappingPOs {
			query := repo.PromptLabelQuery{
				PromptID: mapping.PromptID,
				LabelKey: mapping.LabelKey,
			}
			result[query] = mapping.PromptVersion

			// 准备缓存数据
			if options.CacheEnable {
				cacheMappings = append(cacheMappings, redis.PromptLabelVersionMapping{
					PromptID: mapping.PromptID,
					LabelKey: mapping.LabelKey,
					Version:  mapping.PromptVersion,
				})
			}
		}

		// 3. 如果启用缓存，将数据库结果写入缓存
		if options.CacheEnable && len(cacheMappings) > 0 {
			err = r.promptLabelVersionDAO.MSet(ctx, cacheMappings)
			if err != nil {
				logs.CtxError(ctx, "[BatchGetPromptVersionByLabel] set cache failed: %v", err)
			}
		}
	}

	return result, nil
}
