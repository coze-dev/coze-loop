// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

// ignore_security_alert_file SQL_INJECTION
package evaluator

import (
	"context"
	"strconv"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
)

// EvaluatorRepoImpl 实现 EvaluatorRepo 接口
type EvaluatorRepoImpl struct {
	idgen                idgen.IIDGenerator
	evaluatorDao         mysql.EvaluatorDAO
	evaluatorVersionDao  mysql.EvaluatorVersionDAO
	tagDAO               mysql.EvaluatorTagDAO
	evaluatorTemplateDAO mysql.EvaluatorTemplateDAO
	dbProvider           db.Provider
	lwt                  platestwrite.ILatestWriteTracker
}

// BatchGetEvaluatorVersionsByEvaluatorIDAndVersions 批量根据 (evaluator_id, version) 获取版本
func (r *EvaluatorRepoImpl) BatchGetEvaluatorVersionsByEvaluatorIDAndVersions(ctx context.Context, pairs [][2]interface{}) ([]*entity.Evaluator, error) {
	pos, err := r.evaluatorVersionDao.BatchGetEvaluatorVersionsByEvaluatorIDAndVersions(ctx, pairs)
	if err != nil {
		return nil, err
	}
	result := make([]*entity.Evaluator, 0, len(pos))
	for _, po := range pos {
		do, err := convertor.ConvertEvaluatorVersionPO2DO(po)
		if err != nil {
			return nil, err
		}
		result = append(result, do)
	}
	return result, nil
}

func NewEvaluatorRepo(idgen idgen.IIDGenerator, provider db.Provider, evaluatorDao mysql.EvaluatorDAO, evaluatorVersionDao mysql.EvaluatorVersionDAO, tagDAO mysql.EvaluatorTagDAO, lwt platestwrite.ILatestWriteTracker, evaluatorTemplateDAO mysql.EvaluatorTemplateDAO) repo.IEvaluatorRepo {
	singletonEvaluatorRepo := &EvaluatorRepoImpl{
		evaluatorDao:         evaluatorDao,
		evaluatorVersionDao:  evaluatorVersionDao,
		tagDAO:               tagDAO,
		evaluatorTemplateDAO: evaluatorTemplateDAO,
		dbProvider:           provider,
		idgen:                idgen,
		lwt:                  lwt,
	}
	return singletonEvaluatorRepo
}

func (r *EvaluatorRepoImpl) SubmitEvaluatorVersion(ctx context.Context, evaluator *entity.Evaluator) error {
	err := r.dbProvider.Transaction(ctx, func(tx *gorm.DB) error {
		opt := db.WithTransaction(tx)
		// 更新Evaluator最新版本
		err := r.evaluatorDao.UpdateEvaluatorLatestVersion(ctx, evaluator.ID, evaluator.GetVersion(), gptr.Indirect(evaluator.BaseInfo.UpdatedBy.UserID), opt)
		if err != nil {
			return err
		}
		evaluatorVersionPO, err := convertor.ConvertEvaluatorVersionDO2PO(evaluator)
		if err != nil {
			return err
		}
		err = r.evaluatorVersionDao.CreateEvaluatorVersion(ctx, evaluatorVersionPO, opt)
		if err != nil {
			return err
		}
		// 提交版本成功后，根据模板ID为模板热度 +1（若可解析）
		r.incrTemplatePopularityByEvaluator(ctx, evaluator, opt)
		// 如果是预置评估器，且携带了标签，则为本次提交的版本ID创建tags
		if evaluator.Builtin && len(evaluator.Tags) > 0 {
			userID := session.UserIDInCtxOrEmpty(ctx)
			// 统计需要创建的总标签数
			total := 0
			for _, tagValues := range evaluator.Tags {
				total += len(tagValues)
			}
			if total > 0 {
				// 生成所需的ID
				ids, err := r.idgen.GenMultiIDs(ctx, total)
				if err != nil {
					return err
				}
				idx := 0
				evaluatorTags := make([]*model.EvaluatorTag, 0, total)
				for tagKey, tagValues := range evaluator.Tags {
					for _, tagValue := range tagValues {
						evaluatorTags = append(evaluatorTags, &model.EvaluatorTag{
							ID:        ids[idx],
							SourceID:  evaluatorVersionPO.ID,
							TagType:   int32(entity.EvaluatorTagKeyType_Evaluator),
							TagKey:    string(tagKey),
							TagValue:  tagValue,
							CreatedBy: userID,
							UpdatedBy: userID,
						})
						idx++
					}
				}
				if err := r.tagDAO.BatchCreateEvaluatorTags(ctx, evaluatorTags, opt); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

// incrTemplatePopularityByEvaluator 根据 Evaluator 的模板ID为模板热度 +1
func (r *EvaluatorRepoImpl) incrTemplatePopularityByEvaluator(ctx context.Context, evaluator *entity.Evaluator, opts ...db.Option) {
	var templateIDStr string
	switch evaluator.EvaluatorType {
	case entity.EvaluatorTypePrompt:
		if evaluator.PromptEvaluatorVersion != nil {
			templateIDStr = evaluator.PromptEvaluatorVersion.PromptTemplateKey
		}
	case entity.EvaluatorTypeCode:
		if evaluator.CodeEvaluatorVersion != nil && evaluator.CodeEvaluatorVersion.CodeTemplateKey != nil {
			templateIDStr = *evaluator.CodeEvaluatorVersion.CodeTemplateKey
		}
	}
	if templateIDStr == "" {
		return
	}
	// 模板key存的是模板ID（字符串），转换为int64
	if id, err := strconv.ParseInt(templateIDStr, 10, 64); err == nil {
		_ = r.evaluatorTemplateDAO.IncrPopularityByID(ctx, id, opts...)
	}
}

func (r *EvaluatorRepoImpl) UpdateEvaluatorDraft(ctx context.Context, evaluator *entity.Evaluator) error {
	po, err := convertor.ConvertEvaluatorVersionDO2PO(evaluator)
	if err != nil {
		return err
	}
	return r.dbProvider.Transaction(ctx, func(tx *gorm.DB) error {
		opt := db.WithTransaction(tx)
		// 更新Evaluator最新版本
		err := r.evaluatorDao.UpdateEvaluatorDraftSubmitted(ctx, po.EvaluatorID, false, gptr.Indirect(evaluator.BaseInfo.UpdatedBy.UserID), opt)
		if err != nil {
			return err
		}
		err = r.evaluatorVersionDao.UpdateEvaluatorDraft(ctx, po, opt)
		if err != nil {
			return err
		}
		return nil
	})
}

func (r *EvaluatorRepoImpl) BatchGetEvaluatorMetaByID(ctx context.Context, ids []int64, includeDeleted bool) ([]*entity.Evaluator, error) {
	evaluatorPOS, err := r.evaluatorDao.BatchGetEvaluatorByID(ctx, ids, includeDeleted)
	if err != nil {
		return nil, err
	}
	evaluatorDOs := make([]*entity.Evaluator, 0)
	for _, evaluatorPO := range evaluatorPOS {
		evaluatorDO := convertor.ConvertEvaluatorPO2DO(evaluatorPO)
		evaluatorDOs = append(evaluatorDOs, evaluatorDO)
	}
	return evaluatorDOs, nil
}

func (r *EvaluatorRepoImpl) BatchGetEvaluatorByVersionID(ctx context.Context, spaceID *int64, ids []int64, includeDeleted bool) ([]*entity.Evaluator, error) {
	evaluatorVersionPOS, err := r.evaluatorVersionDao.BatchGetEvaluatorVersionByID(ctx, spaceID, ids, includeDeleted)
	if err != nil {
		return nil, err
	}

	evaluatorPOS, err := r.evaluatorDao.BatchGetEvaluatorByID(ctx, gslice.Map(evaluatorVersionPOS, func(t *model.EvaluatorVersion) int64 {
		return t.EvaluatorID
	}), includeDeleted)
	if err != nil {
		return nil, err
	}
	evaluatorMap := make(map[int64]*model.Evaluator)
	for _, evaluatorPO := range evaluatorPOS {
		evaluatorMap[evaluatorPO.ID] = evaluatorPO
	}
	evaluatorDOList := make([]*entity.Evaluator, 0, len(evaluatorVersionPOS))
	for _, evaluatorVersionPO := range evaluatorVersionPOS {
		if evaluatorVersionPO.EvaluatorType == nil {
			continue
		}
		switch *evaluatorVersionPO.EvaluatorType {
		case int32(entity.EvaluatorTypePrompt):
			evaluatorVersionDO, err := convertor.ConvertEvaluatorVersionPO2DO(evaluatorVersionPO)
			if err != nil {
				return nil, err
			}
			evaluatorDO := convertor.ConvertEvaluatorPO2DO(evaluatorMap[evaluatorVersionPO.EvaluatorID])
			evaluatorDO.PromptEvaluatorVersion = evaluatorVersionDO.PromptEvaluatorVersion
			evaluatorDO.EvaluatorType = entity.EvaluatorTypePrompt
			evaluatorDOList = append(evaluatorDOList, evaluatorDO)
		case int32(entity.EvaluatorTypeCode):
			evaluatorVersionDO, err := convertor.ConvertEvaluatorVersionPO2DO(evaluatorVersionPO)
			if err != nil {
				return nil, err
			}
			evaluatorDO := convertor.ConvertEvaluatorPO2DO(evaluatorMap[evaluatorVersionPO.EvaluatorID])
			evaluatorDO.CodeEvaluatorVersion = evaluatorVersionDO.CodeEvaluatorVersion
			evaluatorDO.EvaluatorType = entity.EvaluatorTypeCode
			evaluatorDOList = append(evaluatorDOList, evaluatorDO)
		case int32(entity.EvaluatorTypeCustomRPC):
			evaluatorVersionDO, err := convertor.ConvertEvaluatorVersionPO2DO(evaluatorVersionPO)
			if err != nil {
				return nil, err
			}
			evaluatorDO := convertor.ConvertEvaluatorPO2DO(evaluatorMap[evaluatorVersionPO.EvaluatorID])
			evaluatorDO.CustomRPCEvaluatorVersion = evaluatorVersionDO.CustomRPCEvaluatorVersion
			evaluatorDO.EvaluatorType = entity.EvaluatorTypeCustomRPC
			evaluatorDOList = append(evaluatorDOList, evaluatorDO)
		default:
			continue
		}
	}
	return evaluatorDOList, nil
}

func (r *EvaluatorRepoImpl) BatchGetEvaluatorDraftByEvaluatorID(ctx context.Context, spaceID int64, ids []int64, includeDeleted bool) ([]*entity.Evaluator, error) {
	var opts []db.Option
	if r.lwt.CheckWriteFlagBySearchParam(ctx, platestwrite.ResourceTypeEvaluator, strconv.FormatInt(spaceID, 10)) {
		opts = append(opts, db.WithMaster())
	}
	evaluatorVersionPOS, err := r.evaluatorVersionDao.BatchGetEvaluatorDraftByEvaluatorID(ctx, ids, includeDeleted, opts...)
	if err != nil {
		return nil, err
	}
	evaluatorID2VersionPO := make(map[int64]*model.EvaluatorVersion)
	for _, evaluatorVersionPO := range evaluatorVersionPOS {
		evaluatorID2VersionPO[evaluatorVersionPO.EvaluatorID] = evaluatorVersionPO
	}
	evaluatorPOS, err := r.evaluatorDao.BatchGetEvaluatorByID(ctx, ids, includeDeleted, opts...)
	if err != nil {
		return nil, err
	}
	evaluatorMap := make(map[int64]*model.Evaluator)
	for _, evaluatorPO := range evaluatorPOS {
		evaluatorMap[evaluatorPO.ID] = evaluatorPO
	}
	// 如果是预置评估器，收集草稿版本ID用于批量查询tags（以版本ID为source_id）
	builtinVersionIDs := make([]int64, 0)
	for _, evaluatorPO := range evaluatorPOS {
		if evaluatorPO.Builtin == 1 {
			if evPO, ok := evaluatorID2VersionPO[evaluatorPO.ID]; ok && evPO != nil {
				builtinVersionIDs = append(builtinVersionIDs, evPO.ID)
			}
		}
	}
	// 批量查询所有tags
	var allTags []*model.EvaluatorTag
	if len(builtinVersionIDs) > 0 {
		allTags, err = r.tagDAO.BatchGetTagsBySourceIDsAndType(ctx, builtinVersionIDs, int32(entity.EvaluatorTagKeyType_Evaluator), opts...)
		if err != nil {
			allTags = []*model.EvaluatorTag{}
		}
	}
	// 将tags按sourceID分组
	tagsBySourceID := make(map[int64][]*model.EvaluatorTag)
	for _, tag := range allTags {
		tagsBySourceID[tag.SourceID] = append(tagsBySourceID[tag.SourceID], tag)
	}
	evaluatorDOList := make([]*entity.Evaluator, 0, len(evaluatorPOS))
	for _, evaluatorPO := range evaluatorPOS {
		evaluatorDO := convertor.ConvertEvaluatorPO2DO(evaluatorPO)
		if evaluatorVersionPO, exist := evaluatorID2VersionPO[evaluatorPO.ID]; exist {
			evaluatorVersionDO, err := convertor.ConvertEvaluatorVersionPO2DO(evaluatorVersionPO)
			if err != nil {
				return nil, err
			}
			evaluatorDO.SetEvaluatorVersion(evaluatorVersionDO)
			// 如果是预置评估器，回填该草稿版本的tags（以版本ID为source_id）
			if evaluatorPO.Builtin == 1 {
				r.setEvaluatorTags(evaluatorDO, evaluatorVersionPO.ID, tagsBySourceID)
			}
		}
		evaluatorDOList = append(evaluatorDOList, evaluatorDO)
	}
	return evaluatorDOList, nil
}

func (r *EvaluatorRepoImpl) BatchGetEvaluatorVersionsByEvaluatorIDs(ctx context.Context, evaluatorIDs []int64, includeDeleted bool) ([]*entity.Evaluator, error) {
	evaluatorVersionPOS, err := r.evaluatorVersionDao.BatchGetEvaluatorVersionsByEvaluatorIDs(ctx, evaluatorIDs, includeDeleted)
	if err != nil {
		return nil, err
	}
	evaluatorVersionDOList := make([]*entity.Evaluator, 0)
	for _, evaluatorVersionPO := range evaluatorVersionPOS {
		evaluatorVersionDO, err := convertor.ConvertEvaluatorVersionPO2DO(evaluatorVersionPO)
		if err != nil {
			return nil, err
		}
		evaluatorVersionDOList = append(evaluatorVersionDOList, evaluatorVersionDO)
	}
	return evaluatorVersionDOList, nil
}

func (r *EvaluatorRepoImpl) ListEvaluatorVersion(ctx context.Context, req *repo.ListEvaluatorVersionRequest) (*repo.ListEvaluatorVersionResponse, error) {
	daoOrderBy := make([]*mysql.OrderBy, len(req.OrderBy))
	for i, orderBy := range req.OrderBy {
		daoOrderBy[i] = &mysql.OrderBy{
			Field:  gptr.Indirect(orderBy.Field),
			ByDesc: !gptr.Indirect(orderBy.IsAsc),
		}
	}
	daoReq := &mysql.ListEvaluatorVersionRequest{
		EvaluatorID:   req.EvaluatorID,
		QueryVersions: req.QueryVersions,
		PageSize:      req.PageSize,
		PageNum:       req.PageNum,
		OrderBy:       daoOrderBy,
	}

	evaluatorVersionDaoResp, err := r.evaluatorVersionDao.ListEvaluatorVersion(ctx, daoReq)
	if err != nil {
		return nil, err
	}

	evaluatorVersionDOList := make([]*entity.Evaluator, 0, len(evaluatorVersionDaoResp.Versions))
	for _, evaluatorVersionPO := range evaluatorVersionDaoResp.Versions {
		evaluatorVersionDO, err := convertor.ConvertEvaluatorVersionPO2DO(evaluatorVersionPO)
		if err != nil {
			return nil, err
		}
		evaluatorVersionDOList = append(evaluatorVersionDOList, evaluatorVersionDO)
	}
	return &repo.ListEvaluatorVersionResponse{
		TotalCount: evaluatorVersionDaoResp.TotalCount,
		Versions:   evaluatorVersionDOList,
	}, nil
}

func (r *EvaluatorRepoImpl) CheckVersionExist(ctx context.Context, evaluatorID int64, version string) (bool, error) {
	return r.evaluatorVersionDao.CheckVersionExist(ctx, evaluatorID, version)
}

// CreateEvaluator 创建 Evaluator
func (r *EvaluatorRepoImpl) CreateEvaluator(ctx context.Context, do *entity.Evaluator) (evaluatorID int64, err error) {
	// 生成主键ID
	genIDs, err := r.idgen.GenMultiIDs(ctx, 3)
	if err != nil {
		return 0, err
	}

	evaluatorPO := convertor.ConvertEvaluatorDO2PO(do)
	evaluatorPO.ID = genIDs[0]
	evaluatorID = evaluatorPO.ID
	evaluatorPO.DraftSubmitted = gptr.Of(true) // 初始化创建时草稿统一已提交
	evaluatorPO.LatestVersion = do.GetVersion()
	evaluatorVersionPO, err := convertor.ConvertEvaluatorVersionDO2PO(do)
	if err != nil {
		return 0, err
	}

	evaluatorVersionPO.EvaluatorID = evaluatorPO.ID

	err = r.dbProvider.Transaction(ctx, func(tx *gorm.DB) error {
		opt := db.WithTransaction(tx)

		err = r.evaluatorDao.CreateEvaluator(ctx, evaluatorPO, opt)
		if err != nil {
			return err
		}

		evaluatorVersionPO.ID = genIDs[1]
		err = r.evaluatorVersionDao.CreateEvaluatorVersion(ctx, evaluatorVersionPO, opt)
		if err != nil {
			return err
		}
		evaluatorVersionPO.ID = genIDs[2]
		evaluatorVersionPO.Version = consts.EvaluatorVersionDraftKey
		evaluatorVersionPO.Description = gptr.Of("")
		err = r.evaluatorVersionDao.CreateEvaluatorVersion(ctx, evaluatorVersionPO, opt)
		if err != nil {
			return err
		}
		// 创建成功后，根据模板ID为模板热度 +1（若可解析）
		r.incrTemplatePopularityByEvaluator(ctx, do, opt)
		return nil
	})
	if err != nil {
		return 0, err
	}

	r.lwt.SetWriteFlag(ctx, platestwrite.ResourceTypeEvaluator, evaluatorPO.ID, platestwrite.SetWithSearchParam(strconv.FormatInt(evaluatorPO.SpaceID, 10)))
	return evaluatorID, nil
}

// BatchGetEvaluatorDraft 批量根据ID 获取 Evaluator
func (r *EvaluatorRepoImpl) BatchGetEvaluatorDraft(ctx context.Context, ids []int64, includeDeleted bool) ([]*entity.Evaluator, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	evaluatorPOList, err := r.evaluatorDao.BatchGetEvaluatorByID(ctx, ids, includeDeleted)
	if err != nil {
		return nil, err
	}
	evaluatorVersionPOList, err := r.evaluatorVersionDao.BatchGetEvaluatorVersionByID(ctx, nil, ids, includeDeleted)
	if err != nil {
		return nil, err
	}
	evaluatorVersionDOMap := make(map[int64]*entity.Evaluator)
	for _, evaluatorVersionPO := range evaluatorVersionPOList {
		evaluatorVersionDO, err := convertor.ConvertEvaluatorVersionPO2DO(evaluatorVersionPO)
		if err != nil {
			return nil, err
		}
		evaluatorVersionDOMap[evaluatorVersionPO.EvaluatorID] = evaluatorVersionDO
	}
	evaluatorDOList := make([]*entity.Evaluator, 0, len(evaluatorPOList))
	for _, evaluatorPO := range evaluatorPOList {
		evaluatorDO := convertor.ConvertEvaluatorPO2DO(evaluatorPO)
		if evaluatorVersionDO, exist := evaluatorVersionDOMap[evaluatorPO.ID]; exist {
			evaluatorDO.SetEvaluatorVersion(evaluatorVersionDO)
		}
		evaluatorDOList = append(evaluatorDOList, evaluatorDO)
	}
	return evaluatorDOList, nil
}

// UpdateEvaluatorMeta 更新 Evaluator
func (r *EvaluatorRepoImpl) UpdateEvaluatorMeta(ctx context.Context, req *entity.UpdateEvaluatorMetaRequest) error {
	po := &model.Evaluator{ID: req.ID, UpdatedBy: req.UpdatedBy}
	if req.Name != nil {
		po.Name = req.Name
	}
	if req.Description != nil {
		po.Description = req.Description
	}
	if req.Benchmark != nil {
		po.Benchmark = req.Benchmark
	}
	if req.Vendor != nil {
		po.Vendor = req.Vendor
	}
	if req.BuiltinVisibleVersion != nil {
		po.BuiltinVisibleVersion = gptr.Indirect(req.BuiltinVisibleVersion)
	}
	if req.Builtin != nil {
		// 将 bool 转为 1/2 存入
		if *req.Builtin {
			po.Builtin = 1
		} else {
			po.Builtin = 2
		}
	}
	return r.evaluatorDao.UpdateEvaluatorMeta(ctx, po)
}

// UpdateEvaluatorTags 根据评估器ID全量更新标签：不存在的新增，不在传入列表中的删除
func (r *EvaluatorRepoImpl) UpdateEvaluatorTags(ctx context.Context, evaluatorID int64, tags map[entity.EvaluatorTagKey][]string) error {
	return r.dbProvider.Transaction(ctx, func(tx *gorm.DB) error {
		opt := db.WithTransaction(tx)
		// 查询当前已有标签
		existingTags, err := r.tagDAO.BatchGetTagsBySourceIDsAndType(ctx, []int64{evaluatorID}, int32(entity.EvaluatorTagKeyType_Evaluator), opt)
		if err != nil {
			return err
		}
		// 构建现有集合
		existing := make(map[string]map[string]bool)
		for _, t := range existingTags {
			if _, ok := existing[t.TagKey]; !ok {
				existing[t.TagKey] = make(map[string]bool)
			}
			existing[t.TagKey][t.TagValue] = true
		}
		// 目标集
		target := make(map[string]map[string]bool)
		for k, vs := range tags {
			kstr := string(k)
			if _, ok := target[kstr]; !ok {
				target[kstr] = make(map[string]bool)
			}
			for _, v := range vs {
				target[kstr][v] = true
			}
		}
		// 计算需要删除
		del := make(map[string][]string)
		for k, vals := range existing {
			for v := range vals {
				if !target[k][v] {
					del[k] = append(del[k], v)
				}
			}
		}
		if len(del) > 0 {
			if err := r.tagDAO.DeleteEvaluatorTagsByConditions(ctx, evaluatorID, int32(entity.EvaluatorTagKeyType_Evaluator), del, opt); err != nil {
				return err
			}
		}
		// 计算需要新增
		add := make(map[string][]string)
		for k, vals := range target {
			for v := range vals {
				if !existing[k][v] {
					add[k] = append(add[k], v)
				}
			}
		}
		if len(add) > 0 {
			userID := session.UserIDInCtxOrEmpty(ctx)
			// 统计需要新增的标签数量
			total := 0
			for _, vals := range add {
				total += len(vals)
			}
			if total > 0 {
				ids, err := r.idgen.GenMultiIDs(ctx, total)
				if err != nil {
					return err
				}
				idx := 0
				evaluatorTags := make([]*model.EvaluatorTag, 0, total)
				for k, vals := range add {
					for _, v := range vals {
						evaluatorTags = append(evaluatorTags, &model.EvaluatorTag{
							ID:        ids[idx],
							SourceID:  evaluatorID,
							TagType:   int32(entity.EvaluatorTagKeyType_Evaluator),
							TagKey:    k,
							TagValue:  v,
							CreatedBy: userID,
							UpdatedBy: userID,
						})
						idx++
					}
				}
				if err := r.tagDAO.BatchCreateEvaluatorTags(ctx, evaluatorTags, opt); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// BatchDeleteEvaluator 根据 ID 删除 Evaluator
func (r *EvaluatorRepoImpl) BatchDeleteEvaluator(ctx context.Context, ids []int64, userID string) (err error) {
	return r.dbProvider.Transaction(ctx, func(tx *gorm.DB) error {
		opt := db.WithTransaction(tx)

		err = r.evaluatorDao.BatchDeleteEvaluator(ctx, ids, userID, opt)
		if err != nil {
			return err
		}
		err = r.evaluatorVersionDao.BatchDeleteEvaluatorVersionByEvaluatorIDs(ctx, ids, userID, opt)
		if err != nil {
			return err
		}
		return nil
	})
}

// CheckNameExist 校验当前名称是否存在
func (r *EvaluatorRepoImpl) CheckNameExist(ctx context.Context, spaceID, evaluatorID int64, name string) (bool, error) {
	return r.evaluatorDao.CheckNameExist(ctx, spaceID, evaluatorID, name)
}

func (r *EvaluatorRepoImpl) ListEvaluator(ctx context.Context, req *repo.ListEvaluatorRequest) (*repo.ListEvaluatorResponse, error) {
	evaluatorTypes := make([]int32, 0, len(req.EvaluatorType))
	for _, evaluatorType := range req.EvaluatorType {
		evaluatorTypes = append(evaluatorTypes, int32(evaluatorType))
	}
	orderBys := make([]*mysql.OrderBy, 0, len(req.OrderBy))
	for _, orderBy := range req.OrderBy {
		orderBys = append(orderBys, &mysql.OrderBy{
			Field:  gptr.Indirect(orderBy.Field), // ignore_security_alert
			ByDesc: !gptr.Indirect(orderBy.IsAsc),
		})
	}
	daoReq := &mysql.ListEvaluatorRequest{
		SpaceID:       req.SpaceID,
		SearchName:    req.SearchName,
		CreatorIDs:    req.CreatorIDs,
		EvaluatorType: evaluatorTypes,
		PageSize:      req.PageSize,
		PageNum:       req.PageNum,
		OrderBy:       orderBys,
	}
	evaluatorPOS, err := r.evaluatorDao.ListEvaluator(ctx, daoReq)
	if err != nil {
		return nil, err
	}
	resp := &repo.ListEvaluatorResponse{
		TotalCount: evaluatorPOS.TotalCount,
		Evaluators: make([]*entity.Evaluator, 0, len(evaluatorPOS.Evaluators)),
	}
	for _, evaluatorPO := range evaluatorPOS.Evaluators {
		evaluatorDO := convertor.ConvertEvaluatorPO2DO(evaluatorPO)
		resp.Evaluators = append(resp.Evaluators, evaluatorDO)
	}
	return resp, nil
}

// ListBuiltinEvaluator 根据筛选条件查询内置评估器列表，支持tag筛选和分页
func (r *EvaluatorRepoImpl) ListBuiltinEvaluator(ctx context.Context, req *repo.ListBuiltinEvaluatorRequest) (*repo.ListBuiltinEvaluatorResponse, error) {
	evaluatorIDs := []int64{}
	var err error

	// 处理筛选条件
	if req.FilterOption != nil {
		// 检查是否有有效的筛选条件
		hasValidFilters := false

		// 检查SearchKeyword是否有效
		if req.FilterOption.SearchKeyword != nil && *req.FilterOption.SearchKeyword != "" {
			hasValidFilters = true
		}

		// 检查FilterConditions是否有效
		if req.FilterOption.Filters != nil && len(req.FilterOption.Filters.FilterConditions) > 0 {
			hasValidFilters = true
		}

		// 如果有有效的筛选条件，进行标签查询
		if hasValidFilters {
			// 使用EvaluatorTagDAO查询符合条件的evaluator IDs（不分页）
			filteredIDs, _, err := r.tagDAO.GetSourceIDsByFilterConditions(ctx, int32(entity.EvaluatorTagKeyType_Evaluator), req.FilterOption, req.PageSize, req.PageNum)
			if err != nil {
				return nil, err
			}

			if len(filteredIDs) == 0 {
				return &repo.ListBuiltinEvaluatorResponse{
					TotalCount: 0,
					Evaluators: []*entity.Evaluator{},
				}, nil
			}

			// 使用筛选后的IDs
			evaluatorIDs = filteredIDs
		}
	}

	// 构建DAO层查询请求（专用内置接口，默认按 name 排序）
	daoReq := &mysql.ListBuiltinEvaluatorRequest{
		IDs:      evaluatorIDs,
		PageSize: req.PageSize,
		PageNum:  req.PageNum,
		OrderBy:  []*mysql.OrderBy{{Field: "name", ByDesc: false}},
	}

	// 调用DAO层查询
	daoResp, err := r.evaluatorDao.ListBuiltinEvaluator(ctx, daoReq)
	if err != nil {
		return nil, err
	}

	// 直接以 evaluatorID 为 source_id 批量查标签
	var allTags []*model.EvaluatorTag
	if len(daoResp.Evaluators) > 0 {
		ids := make([]int64, 0, len(daoResp.Evaluators))
		for _, po := range daoResp.Evaluators {
			ids = append(ids, po.ID)
		}
		var tagErr error
		allTags, tagErr = r.tagDAO.BatchGetTagsBySourceIDsAndType(ctx, ids, int32(entity.EvaluatorTagKeyType_Evaluator))
		if tagErr != nil {
			allTags = []*model.EvaluatorTag{}
		}
	}
	tagsBySourceID := make(map[int64][]*model.EvaluatorTag)
	for _, tag := range allTags {
		tagsBySourceID[tag.SourceID] = append(tagsBySourceID[tag.SourceID], tag)
	}
	evaluators := make([]*entity.Evaluator, 0, len(daoResp.Evaluators))
	for _, evaluatorPO := range daoResp.Evaluators {
		evaluatorDO := convertor.ConvertEvaluatorPO2DO(evaluatorPO)
		r.setEvaluatorTags(evaluatorDO, evaluatorPO.ID, tagsBySourceID)
		evaluators = append(evaluators, evaluatorDO)
	}
	return &repo.ListBuiltinEvaluatorResponse{TotalCount: daoResp.TotalCount, Evaluators: evaluators}, nil
}

// BatchGetBuiltinEvaluatorByVersionID 批量根据版本ID获取内置评估器，包含tag信息
func (r *EvaluatorRepoImpl) BatchGetBuiltinEvaluatorByVersionID(ctx context.Context, spaceID *int64, ids []int64, includeDeleted bool) ([]*entity.Evaluator, error) {
	// 先获取evaluator版本信息
	evaluatorVersionPOS, err := r.evaluatorVersionDao.BatchGetEvaluatorVersionByID(ctx, spaceID, ids, includeDeleted)
	if err != nil {
		return nil, err
	}

	// 获取evaluator基本信息
	evaluatorPOS, err := r.evaluatorDao.BatchGetEvaluatorByID(ctx, gslice.Map(evaluatorVersionPOS, func(t *model.EvaluatorVersion) int64 {
		return t.EvaluatorID
	}), includeDeleted)
	if err != nil {
		return nil, err
	}

	// 构建evaluator映射
	evaluatorMap := make(map[int64]*model.Evaluator)
	for _, evaluatorPO := range evaluatorPOS {
		evaluatorMap[evaluatorPO.ID] = evaluatorPO
	}

	// 收集所有 evaluator_version 的ID用于查询tags（以版本ID为source_id）
	versionIDs := make([]int64, 0, len(evaluatorVersionPOS))
	for _, ev := range evaluatorVersionPOS {
		versionIDs = append(versionIDs, ev.ID)
	}

	// 批量查询所有tags（以版本ID为source_id）
	var allTags []*model.EvaluatorTag
	if len(versionIDs) > 0 {
		allTags, err = r.tagDAO.BatchGetTagsBySourceIDsAndType(ctx, versionIDs, int32(entity.EvaluatorTagKeyType_Evaluator))
		if err != nil {
			// 如果批量查询tags失败，记录错误但继续处理
			allTags = []*model.EvaluatorTag{}
		}
	}

	// 将tags按sourceID分组
	tagsBySourceID := make(map[int64][]*model.EvaluatorTag)
	for _, tag := range allTags {
		tagsBySourceID[tag.SourceID] = append(tagsBySourceID[tag.SourceID], tag)
	}

	// 构建结果
	evaluatorDOList := make([]*entity.Evaluator, 0, len(evaluatorVersionPOS))
	for _, evaluatorVersionPO := range evaluatorVersionPOS {
		if evaluatorVersionPO.EvaluatorType == nil {
			continue
		}

		evaluatorPO, exists := evaluatorMap[evaluatorVersionPO.EvaluatorID]
		if !exists {
			continue
		}

		var evaluatorDO *entity.Evaluator
		switch *evaluatorVersionPO.EvaluatorType {
		case int32(entity.EvaluatorTypePrompt):
			evaluatorVersionDO, err := convertor.ConvertEvaluatorVersionPO2DO(evaluatorVersionPO)
			if err != nil {
				return nil, err
			}
			evaluatorDO = convertor.ConvertEvaluatorPO2DO(evaluatorPO)
			evaluatorDO.PromptEvaluatorVersion = evaluatorVersionDO.PromptEvaluatorVersion
			evaluatorDO.EvaluatorType = entity.EvaluatorTypePrompt

		case int32(entity.EvaluatorTypeCode):
			evaluatorVersionDO, err := convertor.ConvertEvaluatorVersionPO2DO(evaluatorVersionPO)
			if err != nil {
				return nil, err
			}
			evaluatorDO = convertor.ConvertEvaluatorPO2DO(evaluatorPO)
			evaluatorDO.CodeEvaluatorVersion = evaluatorVersionDO.CodeEvaluatorVersion
			evaluatorDO.EvaluatorType = entity.EvaluatorTypeCode

		case int32(entity.EvaluatorTypeCustomRPC):
			evaluatorVersionDO, err := convertor.ConvertEvaluatorVersionPO2DO(evaluatorVersionPO)
			if err != nil {
				return nil, err
			}
			evaluatorDO = convertor.ConvertEvaluatorPO2DO(evaluatorPO)
			evaluatorDO.CustomRPCEvaluatorVersion = evaluatorVersionDO.CustomRPCEvaluatorVersion
			evaluatorDO.EvaluatorType = entity.EvaluatorTypeCustomRPC

		default:
			continue
		}

		// 设置tags信息（以版本ID为source_id）
		r.setEvaluatorTags(evaluatorDO, evaluatorVersionPO.ID, tagsBySourceID)

		evaluatorDOList = append(evaluatorDOList, evaluatorDO)
	}

	return evaluatorDOList, nil
}

// setEvaluatorTags 设置评估器的tag信息
func (r *EvaluatorRepoImpl) setEvaluatorTags(evaluatorDO *entity.Evaluator, evaluatorID int64, tagsBySourceID map[int64][]*model.EvaluatorTag) {
	if tags, exists := tagsBySourceID[evaluatorID]; exists && len(tags) > 0 {
		tagMap := make(map[entity.EvaluatorTagKey][]string)
		for _, tag := range tags {
			tagKey := entity.EvaluatorTagKey(tag.TagKey)
			if tagMap[tagKey] == nil {
				tagMap[tagKey] = make([]string, 0)
			}
			tagMap[tagKey] = append(tagMap[tagKey], tag.TagValue)
		}
		evaluatorDO.Tags = tagMap
	}
}
