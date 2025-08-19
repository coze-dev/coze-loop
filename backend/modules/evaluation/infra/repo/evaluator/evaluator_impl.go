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
	idgen               idgen.IIDGenerator
	evaluatorDao        mysql.EvaluatorDAO
	evaluatorVersionDao mysql.EvaluatorVersionDAO
	dbProvider          db.Provider
	lwt                 platestwrite.ILatestWriteTracker
}

func NewEvaluatorRepo(idgen idgen.IIDGenerator, provider db.Provider, evaluatorDao mysql.EvaluatorDAO, evaluatorVersionDao mysql.EvaluatorVersionDAO, lwt platestwrite.ILatestWriteTracker) repo.IEvaluatorRepo {
	singletonEvaluatorRepo := &EvaluatorRepoImpl{
		evaluatorDao:        evaluatorDao,
		evaluatorVersionDao: evaluatorVersionDao,
		dbProvider:          provider,
		idgen:               idgen,
		lwt:                 lwt,
	}
	return singletonEvaluatorRepo
}

func (r *EvaluatorRepoImpl) SubmitEvaluatorVersion(ctx context.Context, evaluator *entity.Evaluator) error {
	err := r.dbProvider.Transaction(ctx, func(tx *gorm.DB) error {
		opt := db.WithTransaction(tx)
		// 更新Evaluator最新版本
		err := r.evaluatorDao.UpdateEvaluatorLatestVersion(ctx, evaluator.ID, evaluator.GetEvaluatorVersion().GetVersion(), gptr.Indirect(evaluator.BaseInfo.UpdatedBy.UserID), opt)
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
		return nil
	})
	if err != nil {
		return err
	}
	return nil
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
	evaluatorDOList := make([]*entity.Evaluator, 0, len(evaluatorPOS))
	for _, evaluatorPO := range evaluatorPOS {
		evaluatorDO := convertor.ConvertEvaluatorPO2DO(evaluatorPO)
		if evaluatorVersionPO, exist := evaluatorID2VersionPO[evaluatorPO.ID]; exist {
			evaluatorVersionDO, err := convertor.ConvertEvaluatorVersionPO2DO(evaluatorVersionPO)
			if err != nil {
				return nil, err
			}
			evaluatorDO.SetEvaluatorVersion(evaluatorVersionDO)
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
	evaluatorPO.LatestVersion = do.GetEvaluatorVersion().GetVersion()
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
func (r *EvaluatorRepoImpl) UpdateEvaluatorMeta(ctx context.Context, id int64, name, description, userID string) error {
	po := &model.Evaluator{
		ID:          id,
		Name:        gptr.Of(name),
		Description: gptr.Of(description),
		UpdatedBy:   userID,
	}
	err := r.evaluatorDao.UpdateEvaluatorMeta(ctx, po)
	if err != nil {
		return err
	}
	return nil
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
