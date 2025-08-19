// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/conf"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

var (
	singletonEvaluatorService EvaluatorService
	onceEvaluatorService      = sync.Once{}
)

// NewEvaluatorServiceImpl 创建 EvaluatorService 实例
func NewEvaluatorServiceImpl(
	idgen idgen.IIDGenerator,
	limiter repo.RateLimiter,
	mqFactory mq.IFactory,
	evaluatorRepo repo.IEvaluatorRepo,
	evaluatorRecordRepo repo.IEvaluatorRecordRepo,
	idem idem.IdempotentService,
	configer conf.IConfiger,
	evaluatorSourceServices []EvaluatorSourceService,
) EvaluatorService {
	onceEvaluatorService.Do(func() {
		singletonEvaluatorService = &EvaluatorServiceImpl{
			limiter:             limiter,
			mqFactory:           mqFactory,
			evaluatorRepo:       evaluatorRepo,
			evaluatorRecordRepo: evaluatorRecordRepo,
			idgen:               idgen,
			idem:                idem,
			configer:            configer,
			evaluatorSourceServices: gslice.ToMap(evaluatorSourceServices, func(t EvaluatorSourceService) (entity.EvaluatorType, EvaluatorSourceService) {
				return t.EvaluatorType(), t
			}),
		}
	})
	return singletonEvaluatorService
}

// EvaluatorServiceImpl 实现 EvaluatorService 接口
type EvaluatorServiceImpl struct {
	idgen                   idgen.IIDGenerator
	limiter                 repo.RateLimiter
	mqFactory               mq.IFactory
	evaluatorRepo           repo.IEvaluatorRepo
	evaluatorRecordRepo     repo.IEvaluatorRecordRepo
	idem                    idem.IdempotentService
	configer                conf.IConfiger
	evaluatorSourceServices map[entity.EvaluatorType]EvaluatorSourceService
}

// ListEvaluator 按查询条件查询 evaluator_version
func (e *EvaluatorServiceImpl) ListEvaluator(ctx context.Context, request *entity.ListEvaluatorRequest) ([]*entity.Evaluator, int64, error) {
	repoReq, err := buildListEvaluatorRequest(ctx, request)
	if err != nil {
		return nil, 0, err
	}

	// 调用repo层接口
	result, err := e.evaluatorRepo.ListEvaluator(ctx, repoReq)
	if err != nil {
		return nil, 0, err
	}
	if !request.WithVersion {
		return result.Evaluators, result.TotalCount, nil
	}

	evaluatorID2DO := make(map[int64]*entity.Evaluator, len(result.Evaluators))
	for _, evaluator := range result.Evaluators {
		evaluatorID2DO[evaluator.ID] = evaluator
	}

	// 批量获取版本信息
	evaluatorIDs := make([]int64, 0, len(result.Evaluators))
	for _, evaluator := range result.Evaluators {
		evaluatorIDs = append(evaluatorIDs, evaluator.ID)
	}
	evaluatorVersions, err := e.evaluatorRepo.BatchGetEvaluatorVersionsByEvaluatorIDs(ctx, evaluatorIDs, false)
	if err != nil {
		return nil, 0, err
	}
	// 组装版本信息
	for _, evaluatorVersion := range evaluatorVersions {
		evaluatorDO, ok := evaluatorID2DO[evaluatorVersion.GetEvaluatorVersion().GetEvaluatorID()]
		if !ok {
			continue
		}
		evaluatorVersion.ID = evaluatorDO.ID
		evaluatorVersion.SpaceID = evaluatorDO.SpaceID
		evaluatorVersion.Description = evaluatorDO.Description
		evaluatorVersion.BaseInfo = evaluatorDO.BaseInfo
		evaluatorVersion.Name = evaluatorDO.Name
		evaluatorVersion.EvaluatorType = evaluatorDO.EvaluatorType
		evaluatorVersion.Description = evaluatorDO.Description
		evaluatorVersion.DraftSubmitted = evaluatorDO.DraftSubmitted
		evaluatorVersion.LatestVersion = evaluatorDO.LatestVersion
	}

	return evaluatorVersions, int64(len(evaluatorVersions)), nil
}

func buildListEvaluatorRequest(ctx context.Context, request *entity.ListEvaluatorRequest) (*repo.ListEvaluatorRequest, error) {
	// 转换请求参数为repo层结构
	req := &repo.ListEvaluatorRequest{
		SpaceID:    request.SpaceID,
		SearchName: request.SearchName,
		CreatorIDs: request.CreatorIDs,
		PageSize:   request.PageSize,
		PageNum:    request.PageNum,
	}
	evaluatorType := make([]entity.EvaluatorType, 0, len(request.EvaluatorType))
	evaluatorType = append(evaluatorType, request.EvaluatorType...)
	req.EvaluatorType = evaluatorType

	// 默认排序
	if len(request.OrderBys) == 0 {
		req.OrderBy = []*entity.OrderBy{
			{
				Field: gptr.Of("updated_at"),
				IsAsc: gptr.Of(false),
			},
		}
	} else {
		orderBy := make([]*entity.OrderBy, 0, len(request.OrderBys))
		for _, ob := range request.OrderBys {
			orderBy = append(orderBy, &entity.OrderBy{
				Field: ob.Field,
				IsAsc: ob.IsAsc,
			})
		}
		req.OrderBy = orderBy
	}
	return req, nil
}

// BatchGetEvaluator 按 id 批量查询 evaluator草稿
func (e *EvaluatorServiceImpl) BatchGetEvaluator(ctx context.Context, spaceID int64, evaluatorIDs []int64, includeDeleted bool) ([]*entity.Evaluator, error) {
	return e.evaluatorRepo.BatchGetEvaluatorDraftByEvaluatorID(ctx, spaceID, evaluatorIDs, includeDeleted)
}

// GetEvaluator 按 id 单个查询 evaluator元信息和草稿
func (e *EvaluatorServiceImpl) GetEvaluator(ctx context.Context, spaceID int64, evaluatorID int64, includeDeleted bool) (*entity.Evaluator, error) {
	// 修改参数处理方式
	if evaluatorID == 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("evaluatorID id is nil"))
	}
	drafts, err := e.evaluatorRepo.BatchGetEvaluatorDraftByEvaluatorID(ctx, spaceID, []int64{evaluatorID}, includeDeleted)
	if err != nil {
		return nil, err
	}

	if len(drafts) == 0 {
		return nil, nil
	}

	return drafts[0], nil
}

// CreateEvaluator 创建 evaluator_version
func (e *EvaluatorServiceImpl) CreateEvaluator(ctx context.Context, evaluator *entity.Evaluator, cid string) (int64, error) {
	err := e.idem.Set(ctx, e.makeCreateIdemKey(cid), time.Second*10)
	if err != nil {
		return 0, errorx.NewByCode(errno.ActionRepeatedCode, errorx.WithExtraMsg(fmt.Sprintf("[CreateEvaluator] idempotent error, %s", err)))
	}
	validateErr := e.validateCreateEvaluatorRequest(ctx, evaluator)
	if validateErr != nil {
		return 0, validateErr
	}
	e.injectUserInfo(ctx, evaluator)
	evaluatorID, err := e.evaluatorRepo.CreateEvaluator(ctx, evaluator)
	if err != nil {
		return 0, err
	}

	// 返回创建结果
	return evaluatorID, nil
}

func (e *EvaluatorServiceImpl) makeCreateIdemKey(cid string) string {
	return consts.IdemKeyCreateEvaluator + cid
}

// 校验CreateEvaluator参数合法性
func (e *EvaluatorServiceImpl) validateCreateEvaluatorRequest(ctx context.Context, evaluator *entity.Evaluator) error {
	// 校验参数是否为空
	if evaluator == nil {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("evaluator_version is nil"))
	}
	if evaluator.SpaceID == 0 {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("space id is nil"))
	}
	// 校验评估器名称是否已存在
	if evaluator.Name != "" {
		exist, err := e.evaluatorRepo.CheckNameExist(ctx, evaluator.SpaceID, consts.EvaluatorEmptyID, evaluator.Name)
		if err != nil {
			return err
		}
		if exist {
			return errorx.NewByCode(errno.EvaluatorNameExistCode)
		}
	}
	return nil
}

// UpdateEvaluatorMeta 修改 evaluator_version
func (e *EvaluatorServiceImpl) UpdateEvaluatorMeta(ctx context.Context, id, spaceID int64, name, description, userID string) error {
	validateErr := e.validateUpdateEvaluatorMetaRequest(ctx, id, spaceID, name)
	if validateErr != nil {
		return validateErr
	}

	if err := e.evaluatorRepo.UpdateEvaluatorMeta(ctx, id, name, description, userID); err != nil {
		return err
	}
	return nil
}

// 校验UpdateEvaluator参数合法性
func (e *EvaluatorServiceImpl) validateUpdateEvaluatorMetaRequest(ctx context.Context, id, spaceID int64, name string) error {
	// 校验评估器名称是否已存在
	if name != "" {
		exist, err := e.evaluatorRepo.CheckNameExist(ctx, spaceID, id, name)
		if err != nil {
			return err
		}
		if exist {
			return errorx.NewByCode(errno.EvaluatorNameExistCode)
		}
	}
	return nil
}

// UpdateEvaluatorDraft 修改 evaluator_version
func (e *EvaluatorServiceImpl) UpdateEvaluatorDraft(ctx context.Context, versionDO *entity.Evaluator) error {
	versionDO.BaseInfo.SetUpdatedAt(gptr.Of(time.Now().UnixMilli()))
	userIDInContext := session.UserIDInCtxOrEmpty(ctx)
	versionDO.BaseInfo.SetUpdatedBy(&entity.UserInfo{
		UserID: gptr.Of(userIDInContext),
	})
	return e.evaluatorRepo.UpdateEvaluatorDraft(ctx, versionDO)
}

// DeleteEvaluator 删除 evaluator_version
func (e *EvaluatorServiceImpl) DeleteEvaluator(ctx context.Context, evaluatorIDs []int64, userID string) error {
	return e.evaluatorRepo.BatchDeleteEvaluator(ctx, evaluatorIDs, userID)
}

// ListEvaluatorVersion 按查询条件查询 evaluator_version version
func (e *EvaluatorServiceImpl) ListEvaluatorVersion(ctx context.Context, request *entity.ListEvaluatorVersionRequest) (evaluatorVersions []*entity.Evaluator, total int64, err error) {
	// 转换请求参数为repo层结构
	req, err := buildListEvaluatorVersionRequest(ctx, request)
	if err != nil {
		return nil, 0, err
	}

	// 调用repo层接口
	result, err := e.evaluatorRepo.ListEvaluatorVersion(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	return result.Versions, result.TotalCount, nil
}

func buildListEvaluatorVersionRequest(ctx context.Context, request *entity.ListEvaluatorVersionRequest) (*repo.ListEvaluatorVersionRequest, error) {
	// 转换请求参数为repo层结构
	req := &repo.ListEvaluatorVersionRequest{
		EvaluatorID:   request.EvaluatorID,
		QueryVersions: request.QueryVersions,
		PageSize:      request.PageSize,
		PageNum:       request.PageNum,
	}
	if len(request.OrderBys) == 0 {
		req.OrderBy = []*entity.OrderBy{
			{
				Field: gptr.Of(entity.OrderByUpdatedAt),
				IsAsc: gptr.Of(false),
			},
		}
	} else {
		orderBy := make([]*entity.OrderBy, 0, len(request.OrderBys))
		for _, ob := range request.OrderBys {
			if _, ok := entity.OrderBySet[gptr.Indirect(ob.Field)]; ok {
				orderBy = append(orderBy, &entity.OrderBy{
					Field: ob.Field,
					IsAsc: ob.IsAsc,
				})
			}
		}
		req.OrderBy = orderBy
	}
	return req, nil
}

// GetEvaluatorVersion 按 id 和版本号单个查询 evaluator_version version
func (e *EvaluatorServiceImpl) GetEvaluatorVersion(ctx context.Context, evaluatorVersionID int64, includeDeleted bool) (*entity.Evaluator, error) {
	// 获取 evaluator_version 元信息和版本内容
	evaluatorDOList, err := e.evaluatorRepo.BatchGetEvaluatorByVersionID(ctx, nil, []int64{evaluatorVersionID}, includeDeleted)
	if err != nil {
		return nil, err
	}
	if len(evaluatorDOList) == 0 {
		return nil, nil
	}
	return evaluatorDOList[0], nil
}

func (e *EvaluatorServiceImpl) BatchGetEvaluatorVersion(ctx context.Context, spaceID *int64, evaluatorVersionIDs []int64, includeDeleted bool) ([]*entity.Evaluator, error) {
	return e.evaluatorRepo.BatchGetEvaluatorByVersionID(ctx, spaceID, evaluatorVersionIDs, includeDeleted)
}

// SubmitEvaluatorVersion 提交 evaluator_version 版本
func (e *EvaluatorServiceImpl) SubmitEvaluatorVersion(ctx context.Context, evaluatorDO *entity.Evaluator, version, description, cid string) (*entity.Evaluator, error) {
	err := e.idem.Set(ctx, e.makeSubmitIdemKey(cid), time.Second*10)
	if err != nil {
		return nil, errorx.NewByCode(errno.ActionRepeatedCode, errorx.WithExtraMsg(fmt.Sprintf("[CreateEvaluator] idempotent error, %s", err)))
	}
	versionID, err := e.idgen.GenID(ctx)
	if err != nil {
		return nil, err
	}
	userIDInContext := session.UserIDInCtxOrEmpty(ctx)

	if err = evaluatorDO.GetEvaluatorVersion().ValidateBaseInfo(); err != nil {
		return nil, err
	}
	versionExist, err := e.evaluatorRepo.CheckVersionExist(ctx, evaluatorDO.ID, version)
	if err != nil {
		return nil, err
	}
	if versionExist {
		return nil, errorx.NewByCode(errno.EvaluatorVersionExistCode, errorx.WithExtraMsg("version already exists"))
	}
	evaluatorDO.GetEvaluatorVersion().SetID(versionID)
	evaluatorDO.GetEvaluatorVersion().SetVersion(version)
	evaluatorDO.GetEvaluatorVersion().SetDescription(description)
	// 回传提交后的状态
	evaluatorDO.BaseInfo = &entity.BaseInfo{
		UpdatedBy: &entity.UserInfo{
			UserID: gptr.Of(userIDInContext),
		},
		UpdatedAt: gptr.Of(time.Now().UnixMilli()),
	}
	evaluatorDO.GetEvaluatorVersion().SetBaseInfo(&entity.BaseInfo{
		CreatedBy: &entity.UserInfo{
			UserID: gptr.Of(userIDInContext),
		},
		UpdatedBy: &entity.UserInfo{
			UserID: gptr.Of(userIDInContext),
		},
		UpdatedAt: gptr.Of(time.Now().UnixMilli()),
		CreatedAt: gptr.Of(time.Now().UnixMilli()),
	})
	evaluatorDO.LatestVersion = version
	evaluatorDO.DraftSubmitted = true
	return evaluatorDO, e.evaluatorRepo.SubmitEvaluatorVersion(ctx, evaluatorDO)
}

func (e *EvaluatorServiceImpl) makeSubmitIdemKey(cid string) string {
	return consts.IdemKeySubmitEvaluator + cid
}

// RunEvaluator evaluator_version 运行
func (e *EvaluatorServiceImpl) RunEvaluator(ctx context.Context, request *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
	evaluatorDOList, err := e.evaluatorRepo.BatchGetEvaluatorByVersionID(ctx, ptr.Of(request.SpaceID), []int64{request.EvaluatorVersionID}, false)
	if err != nil {
		return nil, err
	}
	if len(evaluatorDOList) == 0 {
		return nil, errorx.NewByCode(errno.EvaluatorVersionNotFoundCode, errorx.WithExtraMsg("evaluator_version version not found"))
	}
	evaluatorDO := evaluatorDOList[0]
	allow := e.limiter.AllowInvoke(ctx, request.SpaceID)
	if !allow {
		return nil, errorx.NewByCode(errno.EvaluatorQPSLimitCode)
	}
	evaluatorSourceService, ok := e.evaluatorSourceServices[evaluatorDO.EvaluatorType]
	if !ok {
		return nil, errorx.NewByCode(errno.EvaluatorNotExistCode)
	}
	if evaluatorSourceService.PreHandle(ctx, evaluatorDO) != nil {
		return nil, err
	}
	outputData, runStatus, traceID := evaluatorSourceService.Run(ctx, evaluatorDO, request.InputData)
	if runStatus == entity.EvaluatorRunStatusFail {
		logs.CtxWarn(ctx, "[RunEvaluator] Run fail, exptID: %d, exptRunID: %d, itemID: %d, turnID: %d, evaluatorVersionID: %d, traceID: %s, err: %v", request.ExperimentID, request.ExperimentRunID, request.ItemID, request.TurnID, request.EvaluatorVersionID, traceID, outputData.EvaluatorRunError)
	}
	recordID, err := e.idgen.GenID(ctx)
	if err != nil {
		return nil, err
	}
	userIDInContext := session.UserIDInCtxOrEmpty(ctx)
	logID := logs.GetLogID(ctx)
	recordDO := &entity.EvaluatorRecord{
		ID:                  recordID,
		SpaceID:             request.SpaceID,
		ExperimentID:        request.ExperimentID,
		ExperimentRunID:     request.ExperimentRunID,
		ItemID:              request.ItemID,
		TurnID:              request.TurnID,
		EvaluatorVersionID:  request.EvaluatorVersionID,
		TraceID:             traceID,
		LogID:               logID,
		EvaluatorInputData:  request.InputData,
		EvaluatorOutputData: outputData,
		Status:              runStatus,
		Ext:                 request.Ext,

		BaseInfo: &entity.BaseInfo{
			CreatedBy: &entity.UserInfo{
				UserID: gptr.Of(userIDInContext),
			},
		},
	}
	err = e.evaluatorRecordRepo.CreateEvaluatorRecord(ctx, recordDO)
	if err != nil {
		return nil, err
	}
	return recordDO, nil
}

// DebugEvaluator 调试 evaluator_version
func (e *EvaluatorServiceImpl) DebugEvaluator(ctx context.Context, evaluatorDO *entity.Evaluator, inputData *entity.EvaluatorInputData) (*entity.EvaluatorOutputData, error) {
	if evaluatorDO == nil || evaluatorDO.GetEvaluatorVersion() == nil {
		return nil, errorx.NewByCode(errno.EvaluatorNotExistCode)
	}
	evaluatorSourceService, ok := e.evaluatorSourceServices[evaluatorDO.EvaluatorType]
	if !ok {
		return nil, errorx.NewByCode(errno.EvaluatorNotExistCode)
	}
	err := evaluatorSourceService.PreHandle(ctx, evaluatorDO)
	if err != nil {
		return nil, err
	}
	return evaluatorSourceService.Debug(ctx, evaluatorDO, inputData)
}

func (e *EvaluatorServiceImpl) CheckNameExist(ctx context.Context, spaceID, evaluatorID int64, name string) (bool, error) {
	return e.evaluatorRepo.CheckNameExist(ctx, spaceID, evaluatorID, name)
}

func (e *EvaluatorServiceImpl) injectUserInfo(ctx context.Context, evaluatorDO *entity.Evaluator) {
	// 注入创建人信息
	userIDInContext := session.UserIDInCtxOrEmpty(ctx)
	evaluatorDO.BaseInfo = &entity.BaseInfo{
		CreatedBy: &entity.UserInfo{
			UserID: gptr.Of(userIDInContext),
		},
		UpdatedBy: &entity.UserInfo{
			UserID: gptr.Of(userIDInContext),
		},
		CreatedAt: gptr.Of(time.Now().UnixMilli()),
		UpdatedAt: gptr.Of(time.Now().UnixMilli()),
	}
	evaluatorDO.GetEvaluatorVersion().SetBaseInfo(&entity.BaseInfo{
		CreatedBy: &entity.UserInfo{
			UserID: gptr.Of(userIDInContext),
		},
		UpdatedBy: &entity.UserInfo{
			UserID: gptr.Of(userIDInContext),
		},
		CreatedAt: gptr.Of(time.Now().UnixMilli()),
		UpdatedAt: gptr.Of(time.Now().UnixMilli()),
	})
}
