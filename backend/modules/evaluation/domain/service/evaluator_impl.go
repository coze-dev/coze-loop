// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/bytedance/gg/gptr"

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
	evaluatorSourceServices map[entity.EvaluatorType]EvaluatorSourceService,
	plainRateLimiter repo.IPlainRateLimiter,
) EvaluatorService {
	onceEvaluatorService.Do(func() {
		singletonEvaluatorService = &EvaluatorServiceImpl{
			limiter:                 limiter,
			mqFactory:               mqFactory,
			evaluatorRepo:           evaluatorRepo,
			evaluatorRecordRepo:     evaluatorRecordRepo,
			idgen:                   idgen,
			idem:                    idem,
			configer:                configer,
			evaluatorSourceServices: evaluatorSourceServices,
			plainRateLimiter:        plainRateLimiter,
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
	plainRateLimiter        repo.IPlainRateLimiter
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
		evaluatorDO, ok := evaluatorID2DO[evaluatorVersion.GetEvaluatorID()]
		if !ok {
			continue
		}
		// 设置 Evaluator.ID 为评估器ID（不是评估器版本ID）
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
		SpaceID:      request.SpaceID,
		SearchName:   request.SearchName,
		CreatorIDs:   request.CreatorIDs,
		FilterOption: request.FilterOption, // 传递FilterOption
		PageSize:     request.PageSize,
		PageNum:      request.PageNum,
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

// ListEvaluatorTags 根据 tagType 聚合标签并按字母序排序
func (e *EvaluatorServiceImpl) ListEvaluatorTags(ctx context.Context, tagType entity.EvaluatorTagKeyType) (map[entity.EvaluatorTagKey][]string, error) {
	if tagType == 0 {
		tagType = entity.EvaluatorTagKeyType_Evaluator
	}
	tags, err := e.evaluatorRepo.ListEvaluatorTags(ctx, tagType)
	if err != nil {
		return nil, err
	}
	for key, values := range tags {
		if len(values) == 0 {
			continue
		}
		sort.Strings(values)
		tags[key] = values
	}
	return tags, nil
}

// BatchGetEvaluator 按 id 批量查询 evaluator草稿
func (e *EvaluatorServiceImpl) BatchGetEvaluator(ctx context.Context, spaceID int64, evaluatorIDs []int64, includeDeleted bool) ([]*entity.Evaluator, error) {
	return e.evaluatorRepo.BatchGetEvaluatorDraftByEvaluatorID(ctx, spaceID, evaluatorIDs, includeDeleted)
}

// GetEvaluator 按 id 单个查询 evaluator元信息和草稿
func (e *EvaluatorServiceImpl) GetEvaluator(ctx context.Context, spaceID, evaluatorID int64, includeDeleted bool) (*entity.Evaluator, error) {
	// 修改参数处理方式
	if evaluatorID == 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("evaluatorID id is nil"))
	}
	drafts, err := e.evaluatorRepo.BatchGetEvaluatorDraftByEvaluatorID(ctx, spaceID, []int64{evaluatorID}, includeDeleted)
	if err != nil {
		return nil, err
	}

	if len(drafts) == 0 || drafts[0].SpaceID != spaceID {
		return nil, nil
	}

	return drafts[0], nil
}

// GetBuiltinEvaluator 根据 evaluatorID 查询元信息，若为预置评估器则按 builtin_visible_version 组装返回
// 非预置评估器或条件不满足时返回 nil
func (e *EvaluatorServiceImpl) GetBuiltinEvaluator(ctx context.Context, evaluatorID int64) (*entity.Evaluator, error) {
	if evaluatorID == 0 {
		return nil, nil
	}

	// 0) 查询元信息以判断是否为预置评估器及其可见版本
	metas, err := e.evaluatorRepo.BatchGetEvaluatorMetaByID(ctx, []int64{evaluatorID}, false)
	if err != nil {
		return nil, err
	}
	if len(metas) == 0 || metas[0] == nil {
		return nil, nil
	}
	meta := metas[0]
	if !meta.Builtin || meta.BuiltinVisibleVersion == "" {
		return nil, nil
	}

	// 1) 通过 (evaluator_id, builtin_visible_version) 获取对应版本
	pairs := [][2]interface{}{{evaluatorID, meta.BuiltinVisibleVersion}}
	versions, err := e.evaluatorRepo.BatchGetEvaluatorVersionsByEvaluatorIDAndVersions(ctx, pairs)
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, nil
	}

	// 2) 回填 metas（元信息）到返回的版本实体根字段
	v := versions[0]
	if v != nil && meta != nil {
		v.ID = meta.ID
		v.SpaceID = meta.SpaceID
		v.Name = meta.Name
		v.Description = meta.Description
		v.DraftSubmitted = meta.DraftSubmitted
		v.EvaluatorType = meta.EvaluatorType
		v.LatestVersion = meta.LatestVersion
		v.Builtin = meta.Builtin
		v.EvaluatorInfo = meta.EvaluatorInfo
		v.BuiltinVisibleVersion = meta.BuiltinVisibleVersion
		v.BoxType = meta.BoxType
		v.Tags = meta.Tags
	}

	return v, nil
}

// BatchGetBuiltinEvaluator 批量获取预置评估器（visible版本）
func (e *EvaluatorServiceImpl) BatchGetBuiltinEvaluator(ctx context.Context, evaluatorIDs []int64) ([]*entity.Evaluator, error) {
	if len(evaluatorIDs) == 0 {
		return []*entity.Evaluator{}, nil
	}
	// 批量获取元信息
	metas, err := e.evaluatorRepo.BatchGetEvaluatorMetaByID(ctx, evaluatorIDs, false)
	if err != nil {
		return nil, err
	}
	// 组装 (evaluator_id, builtin_visible_version) 对
	pairs := make([][2]interface{}, 0, len(metas))
	for _, meta := range metas {
		if meta == nil || !meta.Builtin || meta.BuiltinVisibleVersion == "" {
			continue
		}
		pairs = append(pairs, [2]interface{}{meta.ID, meta.BuiltinVisibleVersion})
	}
	if len(pairs) == 0 {
		return []*entity.Evaluator{}, nil
	}
	// 一次性批量获取版本
	versions, err := e.evaluatorRepo.BatchGetEvaluatorVersionsByEvaluatorIDAndVersions(ctx, pairs)
	if err != nil {
		return nil, err
	}

	// 回填 metas（元信息）到各版本实体根字段
	id2Meta := make(map[int64]*entity.Evaluator, len(metas))
	for _, m := range metas {
		if m != nil {
			id2Meta[m.ID] = m
		}
	}
	for _, v := range versions {
		if v == nil {
			continue
		}
		mid := v.GetEvaluatorID()
		if m, ok := id2Meta[mid]; ok && m != nil {
			v.ID = m.ID
			v.SpaceID = m.SpaceID
			v.Name = m.Name
			v.Description = m.Description
			v.DraftSubmitted = m.DraftSubmitted
			v.EvaluatorType = m.EvaluatorType
			v.LatestVersion = m.LatestVersion
			v.Builtin = m.Builtin
			v.EvaluatorInfo = m.EvaluatorInfo
			v.BuiltinVisibleVersion = m.BuiltinVisibleVersion
			v.BoxType = m.BoxType
			v.Tags = m.Tags
		}
	}
	return versions, nil
}

// BatchGetEvaluatorByIDAndVersion 批量根据 (evaluator_id, version) 查询具体版本
func (e *EvaluatorServiceImpl) BatchGetEvaluatorByIDAndVersion(ctx context.Context, pairs [][2]interface{}) ([]*entity.Evaluator, error) {
	if len(pairs) == 0 {
		return []*entity.Evaluator{}, nil
	}
	versions, err := e.evaluatorRepo.BatchGetEvaluatorVersionsByEvaluatorIDAndVersions(ctx, pairs)
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return versions, nil
	}

	// 收集 evaluator 元信息并回填至版本实体根字段
	evaluatorIDs := make([]int64, 0, len(versions))
	seen := make(map[int64]struct{}, len(versions))
	for _, v := range versions {
		if v == nil {
			continue
		}
		mid := v.GetEvaluatorID()
		if mid == 0 {
			continue
		}
		if _, ok := seen[mid]; ok {
			continue
		}
		seen[mid] = struct{}{}
		evaluatorIDs = append(evaluatorIDs, mid)
	}
	if len(evaluatorIDs) == 0 {
		return versions, nil
	}
	metas, err := e.evaluatorRepo.BatchGetEvaluatorMetaByID(ctx, evaluatorIDs, false)
	if err != nil {
		return nil, err
	}
	id2Meta := make(map[int64]*entity.Evaluator, len(metas))
	for _, m := range metas {
		if m != nil {
			id2Meta[m.ID] = m
		}
	}
	for _, v := range versions {
		if v == nil {
			continue
		}
		if m, ok := id2Meta[v.GetEvaluatorID()]; ok && m != nil {
			v.ID = m.ID
			v.SpaceID = m.SpaceID
			v.Name = m.Name
			v.Description = m.Description
			v.DraftSubmitted = m.DraftSubmitted
			v.EvaluatorType = m.EvaluatorType
			v.LatestVersion = m.LatestVersion
			v.Builtin = m.Builtin
			v.EvaluatorInfo = m.EvaluatorInfo
			v.BuiltinVisibleVersion = m.BuiltinVisibleVersion
			v.BoxType = m.BoxType
			v.Tags = m.Tags
		}
	}
	return versions, nil
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

// nolint:unused // 保留备用：内置评估器创建的幂等键构造
func (e *EvaluatorServiceImpl) makeCreateBuiltinIdemKey(cid string) string {
	return consts.IdemKeyCreateEvaluator + "_builtin_" + cid
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
func (e *EvaluatorServiceImpl) UpdateEvaluatorMeta(ctx context.Context, req *entity.UpdateEvaluatorMetaRequest) error {
	if req == nil {
		return errorx.NewByCode(errno.CommonInvalidParamCode)
	}
	name := ""
	if req.Name != nil {
		name = *req.Name
	}
	if err := e.validateUpdateEvaluatorMetaRequest(ctx, req.ID, req.SpaceID, name); err != nil {
		return err
	}
	return e.evaluatorRepo.UpdateEvaluatorMeta(ctx, req)
}

// UpdateBuiltinEvaluatorTags 根据 evaluatorID 全量对齐标签（多语言）
func (e *EvaluatorServiceImpl) UpdateBuiltinEvaluatorTags(ctx context.Context, evaluatorID int64, tags map[entity.EvaluatorTagLangType]map[entity.EvaluatorTagKey][]string) error {
	return e.evaluatorRepo.UpdateEvaluatorTags(ctx, evaluatorID, tags)
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
func (e *EvaluatorServiceImpl) GetEvaluatorVersion(ctx context.Context, spaceID *int64, evaluatorVersionID int64, includeDeleted bool, withTags bool) (*entity.Evaluator, error) {
	// 合并调用，根据 withTags 控制是否回填 tags
	evaluatorDOList, err := e.evaluatorRepo.BatchGetEvaluatorByVersionID(ctx, spaceID, []int64{evaluatorVersionID}, includeDeleted, withTags)
	if err != nil {
		return nil, err
	}
	if len(evaluatorDOList) == 0 {
		return nil, nil
	}
	return evaluatorDOList[0], nil
}

func (e *EvaluatorServiceImpl) BatchGetEvaluatorVersion(ctx context.Context, spaceID *int64, evaluatorVersionIDs []int64, includeDeleted bool) ([]*entity.Evaluator, error) {
	// 非builtin场景
	return e.evaluatorRepo.BatchGetEvaluatorByVersionID(ctx, spaceID, evaluatorVersionIDs, includeDeleted, false)
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

	if err = evaluatorDO.ValidateBaseInfo(); err != nil {
		return nil, err
	}

	// 新增：获取evaluatorSourceService并执行验证
	evaluatorSourceService, ok := e.evaluatorSourceServices[evaluatorDO.EvaluatorType]
	if ok {
		// 只执行Validate，不调用PreHandle
		err := evaluatorSourceService.Validate(ctx, evaluatorDO)
		if err != nil {
			return nil, err
		}
	}

	versionExist, err := e.evaluatorRepo.CheckVersionExist(ctx, evaluatorDO.ID, version)
	if err != nil {
		return nil, err
	}
	if versionExist {
		return nil, errorx.NewByCode(errno.EvaluatorVersionExistCode, errorx.WithExtraMsg("version already exists"))
	}
	evaluatorDO.SetEvaluatorVersionID(versionID)
	evaluatorDO.SetVersion(version)
	evaluatorDO.SetEvaluatorVersionDescription(description)
	// 回传提交后的状态
	evaluatorDO.BaseInfo = &entity.BaseInfo{
		UpdatedBy: &entity.UserInfo{
			UserID: gptr.Of(userIDInContext),
		},
		UpdatedAt: gptr.Of(time.Now().UnixMilli()),
	}
	evaluatorDO.SetBaseInfo(&entity.BaseInfo{
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
	// 使用 BatchGetEvaluatorByVersionID 查询，不传 spaceID，允许查询所有空间的 evaluator
	evaluatorDOList, err := e.evaluatorRepo.BatchGetEvaluatorByVersionID(ctx, nil, []int64{request.EvaluatorVersionID}, false, false)
	if err != nil {
		return nil, err
	}
	if len(evaluatorDOList) == 0 {
		return nil, errorx.NewByCode(errno.EvaluatorVersionNotFoundCode, errorx.WithExtraMsg("evaluator_version version not found"))
	}
	evaluatorDO := evaluatorDOList[0]
	// TODO: temp remove evaluator space auth for testing
	//// 如果是预置评估器（Builtin），直接执行后续流程
	//// 如果不是预置评估器，则根据 space_id 判断是否当前空间的 Evaluator
	//if !evaluatorDO.Builtin {
	//	if evaluatorDO.SpaceID != request.SpaceID {
	//		return nil, errorx.NewByCode(errno.EvaluatorVersionNotFoundCode, errorx.WithExtraMsg("evaluator_version not found in current space"))
	//	}
	//}
	if allow := e.limiter.AllowInvoke(ctx, request.SpaceID); !allow {
		return nil, errorx.NewByCode(errno.EvaluatorQPSLimitCode, errorx.WithExtraMsg("evaluator throttled due to space-level rate limit"))
	}
	if allow := e.plainRateLimiter.AllowInvokeWithKeyLimit(ctx, fmt.Sprintf("run_evaluator:%v", evaluatorDO.ID), evaluatorDO.GetRateLimit()); !allow {
		return nil, errorx.NewByCode(errno.EvaluatorQPSLimitCode, errorx.WithExtraMsg("evaluator throttled due to evaluator-level rate limit"))
	}
	evaluatorSourceService, ok := e.evaluatorSourceServices[evaluatorDO.EvaluatorType]
	if !ok {
		return nil, errorx.NewByCode(errno.EvaluatorNotExistCode)
	}
	if err = evaluatorSourceService.PreHandle(ctx, evaluatorDO); err != nil {
		return nil, err
	}
	outputData, runStatus, traceID := evaluatorSourceService.Run(ctx, evaluatorDO, request.InputData, request.SpaceID, request.DisableTracing)
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
func (e *EvaluatorServiceImpl) DebugEvaluator(ctx context.Context, evaluatorDO *entity.Evaluator, inputData *entity.EvaluatorInputData, exptSpaceID int64) (*entity.EvaluatorOutputData, error) {
	if evaluatorDO == nil || (evaluatorDO.EvaluatorType == entity.EvaluatorTypePrompt && evaluatorDO.PromptEvaluatorVersion == nil) {
		return nil, errorx.NewByCode(errno.EvaluatorNotExistCode)
	}
	evaluatorSourceService, ok := e.evaluatorSourceServices[evaluatorDO.EvaluatorType]
	if !ok {
		return nil, errorx.NewByCode(errno.EvaluatorNotExistCode)
	}
	// 1. 先执行PreHandle
	err := evaluatorSourceService.PreHandle(ctx, evaluatorDO)
	if err != nil {
		return nil, err
	}
	// 2. 新增：执行Validate
	err = evaluatorSourceService.Validate(ctx, evaluatorDO)
	if err != nil {
		return nil, err
	}
	// 3. 执行Debug
	// exptSpaceID 目前不影响执行路径，预留透传用途
	return evaluatorSourceService.Debug(ctx, evaluatorDO, inputData, exptSpaceID)
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
	evaluatorDO.SetBaseInfo(&entity.BaseInfo{
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

// ListBuiltinEvaluator 查询内置评估器
func (e *EvaluatorServiceImpl) ListBuiltinEvaluator(ctx context.Context, request *entity.ListBuiltinEvaluatorRequest) ([]*entity.Evaluator, int64, error) {
	// 构建ListBuiltinEvaluator请求
	repoReq := &repo.ListBuiltinEvaluatorRequest{
		FilterOption:   request.FilterOption, // 直接使用传入的FilterOption
		PageSize:       request.PageSize,
		PageNum:        request.PageNum,
		IncludeDeleted: false, // 内置评估器不包含已删除的
	}

	// 调用repo层的ListBuiltinEvaluator方法
	result, err := e.evaluatorRepo.ListBuiltinEvaluator(ctx, repoReq)
	if err != nil {
		return nil, 0, err
	}

	// 通过 evaluator_id + BuiltinVisibleVersion 批量查询版本并回填
	pairs := make([][2]interface{}, 0, len(result.Evaluators))
	for _, ev := range result.Evaluators {
		if ev == nil || ev.BuiltinVisibleVersion == "" {
			continue
		}
		pairs = append(pairs, [2]interface{}{ev.ID, ev.BuiltinVisibleVersion})
	}
	if len(pairs) > 0 {
		versions, err := e.evaluatorRepo.BatchGetEvaluatorVersionsByEvaluatorIDAndVersions(ctx, pairs)
		if err != nil {
			return nil, 0, err
		}
		// 建立 (evaluatorID, version) -> DO 映射
		verMap := make(map[string]*entity.Evaluator, len(versions))
		for _, ver := range versions {
			key := strconv.FormatInt(ver.GetEvaluatorID(), 10) + "#" + ver.GetVersion()
			verMap[key] = ver
		}
		// 回填
		for _, ev := range result.Evaluators {
			if ev == nil || ev.BuiltinVisibleVersion == "" {
				continue
			}
			key := strconv.FormatInt(ev.ID, 10) + "#" + ev.BuiltinVisibleVersion
			if v, ok := verMap[key]; ok {
				ev.SetEvaluatorVersion(v)
			}
		}
	}
	return result.Evaluators, result.TotalCount, nil
}
