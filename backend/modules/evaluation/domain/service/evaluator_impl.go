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
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/conf"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/utils"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
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
	cConfiger component.IConfiger,
	evalAsyncRepo repo.IEvalAsyncRepo,
	exptEventPublisher events.ExptEventPublisher,
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
			evalAsyncRepo:           evalAsyncRepo,
			exptEventPublisher:      exptEventPublisher,
			cConfiger:               cConfiger,
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
	evalAsyncRepo           repo.IEvalAsyncRepo
	exptEventPublisher      events.ExptEventPublisher

	cConfiger component.IConfiger
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

func (e *EvaluatorServiceImpl) ResolveBuiltinEvaluatorVisibleVersionID(ctx context.Context, evaluatorID int64, evaluatorName string) (int64, error) {
	if evaluatorID == 0 && evaluatorName == "" {
		return 0, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("builtin_evaluator_id or builtin_evaluator_name is required"))
	}

	builtinSpaceIDs := e.configer.GetBuiltinEvaluatorSpaceConf(ctx)
	if len(builtinSpaceIDs) == 0 {
		return 0, nil
	}
	spaceIDs := make([]int64, 0, len(builtinSpaceIDs))
	spaceIDSet := make(map[int64]struct{}, len(builtinSpaceIDs))
	for _, sid := range builtinSpaceIDs {
		if sid == "" {
			continue
		}
		v, err := strconv.ParseInt(sid, 10, 64)
		if err != nil {
			return 0, err
		}
		spaceIDs = append(spaceIDs, v)
		spaceIDSet[v] = struct{}{}
	}

	var meta *entity.Evaluator
	if evaluatorID > 0 {
		metas, err := e.evaluatorRepo.BatchGetEvaluatorMetaByID(ctx, []int64{evaluatorID}, false)
		if err != nil {
			return 0, err
		}
		if len(metas) == 0 || metas[0] == nil {
			return 0, nil
		}
		if _, ok := spaceIDSet[metas[0].SpaceID]; !ok {
			return 0, nil
		}
		if evaluatorName != "" && metas[0].Name != evaluatorName {
			return 0, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("builtin_evaluator_id and builtin_evaluator_name mismatch"))
		}
		meta = metas[0]
	} else {
		for _, sid := range spaceIDs {
			m, err := e.evaluatorRepo.GetEvaluatorMetaBySpaceIDAndName(ctx, sid, evaluatorName, false)
			if err != nil {
				return 0, err
			}
			if m == nil {
				continue
			}
			meta = m
			break
		}
		if meta == nil {
			return 0, nil
		}
	}

	if !meta.Builtin || meta.BuiltinVisibleVersion == "" {
		return 0, nil
	}

	pairs := [][2]interface{}{{meta.ID, meta.BuiltinVisibleVersion}}
	versions, err := e.evaluatorRepo.BatchGetEvaluatorVersionsByEvaluatorIDAndVersions(ctx, pairs)
	if err != nil {
		return 0, err
	}
	if len(versions) == 0 || versions[0] == nil {
		return 0, nil
	}
	return versions[0].GetEvaluatorVersionID(), nil
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
	if _, ok := entity.EvaluatorTypeSet[evaluator.EvaluatorType]; !ok {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("invalid evaluator type"))
	}
	switch evaluator.EvaluatorType {
	case entity.EvaluatorTypePrompt:
		if evaluator.PromptEvaluatorVersion == nil {
			return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("prompt evaluator version is required"))
		}
	case entity.EvaluatorTypeCode:
		if evaluator.CodeEvaluatorVersion == nil {
			return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("code evaluator version is required"))
		}
	case entity.EvaluatorTypeCustomRPC:
		if evaluator.CustomRPCEvaluatorVersion == nil {
			return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("custom rpc evaluator version is required"))
		}
	case entity.EvaluatorTypeAgent:
		if evaluator.AgentEvaluatorVersion == nil {
			return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("agent evaluator version is required"))
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
func (e *EvaluatorServiceImpl) GetEvaluatorVersion(ctx context.Context, spaceID *int64, evaluatorVersionID int64, includeDeleted, withTags bool) (*entity.Evaluator, error) {
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

// roundEvaluatorOutputScore 统一处理评估器输出数据中的分数，保留两位小数
func roundEvaluatorOutputScore(outputData *entity.EvaluatorOutputData) {
	if outputData == nil {
		return
	}
	if outputData.EvaluatorResult == nil {
		return
	}
	// 处理原始分数
	if outputData.EvaluatorResult.Score != nil {
		roundedScore := utils.RoundScoreToTwoDecimals(*outputData.EvaluatorResult.Score)
		outputData.EvaluatorResult.Score = &roundedScore
	}
	// 处理修正分数
	if outputData.EvaluatorResult.Correction != nil && outputData.EvaluatorResult.Correction.Score != nil {
		roundedScore := utils.RoundScoreToTwoDecimals(*outputData.EvaluatorResult.Correction.Score)
		outputData.EvaluatorResult.Correction.Score = &roundedScore
	}
}

// normalizeEvaluatorRecordSourceType 给运行期写入端兜底:
// 调用方未显式声明 source_type (零值) 时, 默认按 Builtin 处理, 与 evaluator_record 表语义对齐.
func normalizeEvaluatorRecordSourceType(t entity.EvaluatorRecordSourceType) entity.EvaluatorRecordSourceType {
	if t == entity.EvaluatorRecordSourceTypeUnknown {
		return entity.EvaluatorRecordSourceTypeBuiltin
	}
	return t
}

// ShouldInterceptEvaluator 判断评估器是否应劫持本次评估，劫持时创建记录并返回
func (e *EvaluatorServiceImpl) ShouldInterceptEvaluator(ctx context.Context, request *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, bool, error) {
	evaluatorDOList, err := e.evaluatorRepo.BatchGetEvaluatorByVersionID(ctx, nil, []int64{request.EvaluatorVersionID}, false, false)
	if err != nil {
		return nil, false, err
	}
	if len(evaluatorDOList) == 0 {
		return nil, false, errorx.NewByCode(errno.EvaluatorVersionNotFoundCode, errorx.WithExtraMsg("evaluator_version version not found"))
	}
	evaluatorDO := evaluatorDOList[0]
	evaluatorSourceService, ok := e.evaluatorSourceServices[evaluatorDO.EvaluatorType]
	if !ok {
		return nil, false, nil
	}
	output, runStatus, intercepted := evaluatorSourceService.ShouldIntercept(ctx, evaluatorDO, request.InputData)
	if !intercepted {
		return nil, false, nil
	}

	// 劫持时创建评估记录
	recordID, err := e.idgen.GenID(ctx)
	if err != nil {
		return nil, true, err
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
		LogID:               logID,
		EvaluatorInputData:  request.InputData,
		EvaluatorOutputData: output,
		Status:              runStatus,
		Ext:                 request.Ext,
		BaseInfo: &entity.BaseInfo{
			CreatedBy: &entity.UserInfo{
				UserID: gptr.Of(userIDInContext),
			},
		},
	}
	if err := e.evaluatorRecordRepo.CreateEvaluatorRecord(ctx, recordDO); err != nil {
		return nil, true, err
	}
	return recordDO, true, nil
}

// CreateSkippedEvaluatorRecord 行级 filter 不命中时落一条 Status=Skipped 的占位 record。
// 不调底层 evaluator, 不带 input/output (只留状态骨架); ref 表行由上层 storeTurnRunResult 自动跟上。
func (e *EvaluatorServiceImpl) CreateSkippedEvaluatorRecord(ctx context.Context, request *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
	recordID, err := e.idgen.GenID(ctx)
	if err != nil {
		return nil, err
	}
	userIDInContext := session.UserIDInCtxOrEmpty(ctx)
	logID := logs.GetLogID(ctx)

	recordDO := &entity.EvaluatorRecord{
		ID:                 recordID,
		SpaceID:            request.SpaceID,
		ExperimentID:       request.ExperimentID,
		ExperimentRunID:    request.ExperimentRunID,
		ItemID:             request.ItemID,
		TurnID:             request.TurnID,
		EvaluatorVersionID: request.EvaluatorVersionID,
		Alias:              request.Alias,
		SourceType:         normalizeEvaluatorRecordSourceType(request.SourceType),
		LogID:              logID,
		Status:             entity.EvaluatorRunStatusSkipped,
		Ext:                request.Ext,
		BaseInfo: &entity.BaseInfo{
			CreatedBy: &entity.UserInfo{
				UserID: gptr.Of(userIDInContext),
			},
		},
	}
	if err := e.evaluatorRecordRepo.CreateEvaluatorRecord(ctx, recordDO); err != nil {
		return nil, err
	}
	return recordDO, nil
}

// resolveEvaluatorSpaceID 返回评估器执行与落库所用的空间: 非 builtin 评估器恒用它自己的归属空间
// (跨空间调用时即来源空间; 模型 / Skill / 沙箱配置 / 空间服务账号与异步回写身份必须同属一个空间,
// 否则沙箱或用户 PSM 回写时的 workspace_id 与记录空间不符, 记录会永久停在 AsyncInvoking);
// builtin 预置评估器仍用调用方空间, 它的模型须在调用方空间解析。
// 同空间调用与 builtin 场景取值等于 callerSpaceID, 故存量行为不变。
func resolveEvaluatorSpaceID(evaluator *entity.Evaluator, callerSpaceID int64) int64 {
	if evaluator == nil || evaluator.Builtin {
		return callerSpaceID
	}
	return evaluator.SpaceID
}

// checkEvaluatorSpaceAccess 领域层的来源空间一致性校验: 未声明共享时要求评估器属于调用方空间;
// 声明共享时要求它属于所声明的来源空间。白名单授权由 app 层 AuthorizeRead 完成,
// 这里只保证"声明"与"事实"一致, 避免绕过 app 层的调用方拿到跨空间执行能力。
func checkEvaluatorSpaceAccess(evaluator *entity.Evaluator, callerSpaceID int64, opt *entity.SharedResourceOption) error {
	if evaluator == nil || evaluator.Builtin {
		return nil
	}
	if opt.Enabled() {
		if evaluator.SpaceID == *opt.SourceSpaceID {
			return nil
		}
		return errorx.NewByCode(errno.CommonNoPermissionCode, errorx.WithExtraMsg("evaluator_version does not belong to declared source space"))
	}
	if evaluator.SpaceID != callerSpaceID {
		return errorx.NewByCode(errno.EvaluatorVersionNotFoundCode, errorx.WithExtraMsg("evaluator_version not found in current space"))
	}
	return nil
}

// withCallerSpaceExt 跨空间调用时在记录 ext 里留下调用方空间: 记录归资源空间,
// 资源方排障的第一个问题是"这是谁跑的", 而这个信息事后无从反查。
func withCallerSpaceExt(ext map[string]string, callerSpaceID, resourceSpaceID int64) map[string]string {
	if callerSpaceID == resourceSpaceID {
		return ext
	}
	merged := make(map[string]string, len(ext)+1)
	for k, v := range ext {
		merged[k] = v
	}
	merged[consts.EvaluatorRecordExtKeyCallerSpaceID] = strconv.FormatInt(callerSpaceID, 10)
	return merged
}

// RunEvaluator evaluator_version 运行
func (e *EvaluatorServiceImpl) RunEvaluator(ctx context.Context, request *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
	logs.CtxInfo(ctx, "[RunEvaluator] RunEvaluator request: %v", request)
	// 使用 BatchGetEvaluatorByVersionID 查询，不传 spaceID，允许查询所有空间的 evaluator
	evaluatorDOList, err := e.evaluatorRepo.BatchGetEvaluatorByVersionID(ctx, nil, []int64{request.EvaluatorVersionID}, false, false)
	if err != nil {
		return nil, err
	}
	if len(evaluatorDOList) == 0 {
		return nil, errorx.NewByCode(errno.EvaluatorVersionNotFoundCode, errorx.WithExtraMsg("evaluator_version version not found"))
	}
	evaluatorDO := evaluatorDOList[0]
	if err := checkEvaluatorSpaceAccess(evaluatorDO, request.SpaceID, request.SharedOption); err != nil {
		return nil, err
	}
	resourceSpaceID := resolveEvaluatorSpaceID(evaluatorDO, request.SpaceID)
	if allow := e.limiter.AllowInvoke(ctx, request.SpaceID); !allow {
		return nil, errorx.NewByCode(errno.EvaluatorQPSLimitCode, errorx.WithExtraMsg("evaluator throttled due to space-level rate limit"))
	}
	// 资源空间自己的闸门: 评估器级限流按单个 evaluator.ID 计, 调用方并发打多个评估器可以绕过它
	if resourceSpaceID != request.SpaceID {
		if allow := e.limiter.AllowInvoke(ctx, resourceSpaceID); !allow {
			return nil, errorx.NewByCode(errno.EvaluatorQPSLimitCode, errorx.WithExtraMsg("evaluator throttled due to resource space rate limit"))
		}
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
	outputData, runStatus, traceID := evaluatorSourceService.Run(ctx, evaluatorDO, request.InputData, request.EvaluatorRunConf, resourceSpaceID, request.DisableTracing)
	// 统一处理评估器输出数据中的分数，保留两位小数
	roundEvaluatorOutputScore(outputData)
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
		SpaceID:             resourceSpaceID,
		ExperimentID:        request.ExperimentID,
		ExperimentRunID:     request.ExperimentRunID,
		ItemID:              request.ItemID,
		TurnID:              request.TurnID,
		EvaluatorVersionID:  request.EvaluatorVersionID,
		Alias:               request.Alias,
		SourceType:          normalizeEvaluatorRecordSourceType(request.SourceType),
		TraceID:             traceID,
		LogID:               logID,
		EvaluatorInputData:  request.InputData,
		EvaluatorOutputData: outputData,
		Status:              runStatus,
		Ext:                 withCallerSpaceExt(request.Ext, request.SpaceID, resourceSpaceID),

		BaseInfo: &entity.BaseInfo{
			CreatedBy: &entity.UserInfo{
				UserID: gptr.Of(userIDInContext),
			},
		},
	}
	if recordDO.EvaluatorOutputData != nil &&
		recordDO.EvaluatorOutputData.EvaluatorRunError != nil &&
		recordDO.EvaluatorOutputData.EvaluatorRunError.Code != int32(errno.CustomRPCEvaluatorRunFailedCode) &&
		len(recordDO.EvaluatorOutputData.EvaluatorRunError.Message) > 0 {
		recordDO.EvaluatorOutputData.EvaluatorRunError.Message = e.cConfiger.GetErrCtrl(ctx).ConvertErrMsg(recordDO.EvaluatorOutputData.EvaluatorRunError.Message)
	}
	err = e.evaluatorRecordRepo.CreateEvaluatorRecord(ctx, recordDO)
	if err != nil {
		return nil, err
	}
	return recordDO, nil
}

// CreateEvaluatorRunFailRecord creates a failed evaluator record for an evaluator run attempt that failed
// before RunEvaluator/AsyncRunEvaluator could persist its normal record. This keeps experiment turn results
// complete and preserves the original evaluator-level failure reason for users.
func (e *EvaluatorServiceImpl) CreateEvaluatorRunFailRecord(ctx context.Context, request *entity.RunEvaluatorRequest, runErr error) (*entity.EvaluatorRecord, error) {
	if request == nil {
		return nil, errorx.NewByCode(errno.CommonInternalErrorCode, errorx.WithExtraMsg("run evaluator request is nil"))
	}
	if runErr == nil {
		runErr = errorx.NewByCode(errno.CommonInternalErrorCode, errorx.WithExtraMsg("evaluator run failed"))
	}

	recordID, err := e.idgen.GenID(ctx)
	if err != nil {
		return nil, err
	}

	code := int32(errno.CommonInternalErrorCode)
	statusErr, isStatusErr := errorx.FromStatusError(runErr)
	if isStatusErr && statusErr.Code() > 0 {
		code = statusErr.Code()
	}
	errMsg := errorx.ErrorWithoutStack(runErr)
	if e.cConfiger != nil && (!isStatusErr || statusErr.Code() != errno.CustomRPCEvaluatorRunFailedCode) {
		if converted := e.cConfiger.GetErrCtrl(ctx).ConvertErrMsg(errMsg); converted != "" {
			errMsg = converted
		}
	}

	userIDInContext := session.UserIDInCtxOrEmpty(ctx)
	now := time.Now().UnixMilli()
	recordDO := &entity.EvaluatorRecord{
		ID:                 recordID,
		SpaceID:            request.SpaceID,
		ExperimentID:       request.ExperimentID,
		ExperimentRunID:    request.ExperimentRunID,
		ItemID:             request.ItemID,
		TurnID:             request.TurnID,
		EvaluatorVersionID: request.EvaluatorVersionID,
		LogID:              logs.GetLogID(ctx),
		EvaluatorInputData: request.InputData,
		EvaluatorOutputData: &entity.EvaluatorOutputData{
			EvaluatorRunError: &entity.EvaluatorRunError{
				Code:    code,
				Message: errMsg,
			},
		},
		Status: entity.EvaluatorRunStatusFail,
		Ext:    request.Ext,
		BaseInfo: &entity.BaseInfo{
			CreatedBy: &entity.UserInfo{UserID: gptr.Of(userIDInContext)},
			UpdatedBy: &entity.UserInfo{UserID: gptr.Of(userIDInContext)},
			CreatedAt: gptr.Of(now),
			UpdatedAt: gptr.Of(now),
		},
	}

	if err := e.evaluatorRecordRepo.CreateEvaluatorRecord(ctx, recordDO); err != nil {
		return nil, err
	}
	return recordDO, nil
}

// AsyncRunEvaluator coordinates evaluator async kickoff in the strict order Record -> Context -> Provider.
func (e *EvaluatorServiceImpl) AsyncRunEvaluator(ctx context.Context, request *entity.AsyncRunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
	evaluatorDOList, err := e.evaluatorRepo.BatchGetEvaluatorByVersionID(ctx, nil, []int64{request.EvaluatorVersionID}, false, false)
	if err != nil {
		return nil, err
	}
	if len(evaluatorDOList) == 0 {
		return nil, errorx.NewByCode(errno.EvaluatorVersionNotFoundCode, errorx.WithExtraMsg("evaluator_version version not found"))
	}
	evaluatorDO := evaluatorDOList[0]
	if evaluatorDO.EvaluatorType == entity.EvaluatorTypeCustomRPC && evaluatorDO.Builtin {
		return nil, errorx.NewByCode(errno.InvalidEvaluatorTypeCode, errorx.WithExtraMsg("builtin CustomRPC evaluator does not support async run"))
	}
	if !evaluatorDO.IsAsync() {
		return nil, errorx.NewByCode(errno.InvalidEvaluatorTypeCode, errorx.WithExtraMsg("evaluator does not support async run"))
	}
	if err := checkEvaluatorSpaceAccess(evaluatorDO, request.SpaceID, request.SharedOption); err != nil {
		return nil, err
	}
	resourceSpaceID := resolveEvaluatorSpaceID(evaluatorDO, request.SpaceID)
	if allow := e.limiter.AllowInvoke(ctx, request.SpaceID); !allow {
		return nil, errorx.NewByCode(errno.EvaluatorQPSLimitCode, errorx.WithExtraMsg("evaluator throttled due to space-level rate limit"))
	}
	// 资源空间自己的闸门: 评估器级限流按单个 evaluator.ID 计, 调用方并发打多个评估器可以绕过它
	if resourceSpaceID != request.SpaceID {
		if allow := e.limiter.AllowInvoke(ctx, resourceSpaceID); !allow {
			return nil, errorx.NewByCode(errno.EvaluatorQPSLimitCode, errorx.WithExtraMsg("evaluator throttled due to resource space rate limit"))
		}
	}
	if allow := e.plainRateLimiter.AllowInvokeWithKeyLimit(ctx, fmt.Sprintf("async_run_evaluator:%v", evaluatorDO.ID), evaluatorDO.GetRateLimit()); !allow {
		return nil, errorx.NewByCode(errno.EvaluatorQPSLimitCode, errorx.WithExtraMsg("evaluator throttled due to evaluator-level rate limit"))
	}
	invokeID, err := e.idgen.GenID(ctx)
	if err != nil {
		return nil, err
	}
	evaluatorSourceService, ok := e.evaluatorSourceServices[evaluatorDO.EvaluatorType]
	if !ok {
		return nil, errorx.NewByCode(errno.InvalidEvaluatorTypeCode, errorx.WithExtraMsg("evaluator source service not found for async type"))
	}

	now := time.Now().UnixMilli()
	userIDInContext := session.UserIDInCtxOrEmpty(ctx)
	recordDO := &entity.EvaluatorRecord{
		ID:                  invokeID,
		SpaceID:             resourceSpaceID,
		ExperimentID:        request.ExperimentID,
		ExperimentRunID:     request.ExperimentRunID,
		ItemID:              request.ItemID,
		TurnID:              request.TurnID,
		EvaluatorVersionID:  request.EvaluatorVersionID,
		Alias:               request.Alias,
		SourceType:          normalizeEvaluatorRecordSourceType(request.SourceType),
		LogID:               logs.GetLogID(ctx),
		EvaluatorInputData:  request.InputData,
		EvaluatorOutputData: &entity.EvaluatorOutputData{},
		Status:              entity.EvaluatorRunStatusAsyncInvoking,
		Ext:                 withCallerSpaceExt(request.Ext, request.SpaceID, resourceSpaceID),
		BaseInfo: &entity.BaseInfo{
			CreatedBy: &entity.UserInfo{UserID: gptr.Of(userIDInContext)},
			UpdatedBy: &entity.UserInfo{UserID: gptr.Of(userIDInContext)},
			CreatedAt: gptr.Of(now),
			UpdatedAt: gptr.Of(now),
		},
	}
	// CreateEvaluatorRecord may truncate oversized Content fields in-place before persisting them.
	// Persist a deep copy so the provider still receives the original, complete input.
	persistedRecord := *recordDO
	persistedRecord.EvaluatorInputData = deepCopyEvaluatorInputData(request.InputData)
	if persistedRecord.EvaluatorInputData == request.InputData && request.InputData != nil {
		return nil, errorx.New("deep copy evaluator input data failed")
	}
	if err := e.evaluatorRecordRepo.CreateEvaluatorRecord(ctx, &persistedRecord); err != nil {
		logs.CtxError(ctx, "[AsyncRunEvaluator] CreateEvaluatorRecord fail, invokeID: %d, err: %v", invokeID, err)
		return nil, err
	}

	asyncCtx := request.AsyncCtx
	if asyncCtx == nil {
		asyncCtx = &entity.EvalAsyncCtx{ResumeReady: true}
	}
	asyncCtx.RecordID = invokeID
	asyncCtx.EvaluatorVersionID = request.EvaluatorVersionID
	if asyncCtx.AsyncUnixMS == 0 {
		asyncCtx.AsyncUnixMS = now
	}
	if asyncCtx.Session == nil {
		asyncCtx.Session = &entity.Session{UserID: userIDInContext}
	}
	if e.evalAsyncRepo == nil {
		return e.failAsyncEvaluatorRecord(ctx, recordDO, errorx.New("eval async repo is nil"))
	}
	asyncCtxKey := fmt.Sprintf("evaluator:%d", invokeID)
	if err := e.evalAsyncRepo.SetEvalAsyncCtx(ctx, asyncCtxKey, asyncCtx); err != nil {
		return e.failAsyncEvaluatorRecord(ctx, recordDO, err)
	}

	asyncRunExt, traceID, err := evaluatorSourceService.AsyncRun(ctx, evaluatorDO, request.InputData, request.EvaluatorRunConf, resourceSpaceID, invokeID)
	if err != nil {
		logs.CtxError(ctx, "[AsyncRunEvaluator] AsyncRun fail, invokeID: %d, err: %v", invokeID, err)
		return e.failAsyncEvaluatorRecord(ctx, recordDO, err)
	}
	recordDO.TraceID = traceID
	recordDO.EvaluatorOutputData.Ext = asyncRunExt
	if traceID != "" || len(asyncRunExt) > 0 {
		if err := e.evaluatorRecordRepo.UpdateEvaluatorRecordAsyncDispatch(ctx, recordDO.ID, recordDO.SpaceID, traceID, recordDO.EvaluatorOutputData); err != nil {
			// The provider has already accepted the work. Treat dispatch metadata as best-effort;
			// marking the record failed here would create an uncertain-state split brain and reject a later valid callback.
			logs.CtxError(ctx, "[AsyncRunEvaluator] persist dispatch metadata fail, keep record async invoking, invokeID: %d, err: %v", invokeID, err)
		}
	}
	logs.CtxInfo(ctx, "[AsyncRunEvaluator] invokeID: %d, evaluatorVersionID: %d, spaceID: %d, record_ext: %v",
		invokeID, request.EvaluatorVersionID, request.SpaceID, json.Jsonify(recordDO.Ext))
	return recordDO, nil
}

func (e *EvaluatorServiceImpl) failAsyncEvaluatorRecord(ctx context.Context, record *entity.EvaluatorRecord, runErr error) (*entity.EvaluatorRecord, error) {
	if record == nil {
		return nil, runErr
	}
	errMsg := "evaluator async run failed"
	if runErr != nil {
		errMsg = errorx.ErrorWithoutStack(runErr)
	}
	output := &entity.EvaluatorOutputData{EvaluatorRunError: &entity.EvaluatorRunError{
		Code:    int32(errno.CommonInternalErrorCode),
		Message: errMsg,
	}}
	updated, casErr := e.evaluatorRecordRepo.CompareAndSwapEvaluatorRecordResult(ctx, record.ID, record.SpaceID, entity.EvaluatorRunStatusAsyncInvoking, entity.EvaluatorRunStatusFail, output)
	if casErr != nil {
		return record, errorx.Wrapf(casErr, "mark evaluator async kickoff failed, cause: %v", runErr)
	}
	if updated {
		record.Status = entity.EvaluatorRunStatusFail
		record.EvaluatorOutputData = output
		return record, runErr
	}

	// The provider call can return an ACK error after it has already accepted the task and
	// synchronously reported a terminal result. In that race the terminal CAS above loses by
	// design; re-read the primary and let the callback outcome win instead of failing the turn
	// with a stale dispatch error.
	latest, getErr := e.evaluatorRecordRepo.GetEvaluatorRecord(contexts.WithCtxWriteDB(ctx), record.ID, false, entity.WithoutLoadStorageData())
	if getErr != nil {
		return record, errorx.Wrapf(getErr, "reload evaluator record after async kickoff failure, cause: %v", runErr)
	}
	if latest != nil && (latest.Status == entity.EvaluatorRunStatusSuccess || latest.Status == entity.EvaluatorRunStatusFail) {
		return latest, nil
	}
	return record, runErr
}

// AsyncDebugEvaluator Agent evaluator_version 异步调试
func (e *EvaluatorServiceImpl) AsyncDebugEvaluator(ctx context.Context, request *entity.AsyncDebugEvaluatorRequest) (*entity.AsyncDebugEvaluatorResponse, error) {
	evaluatorDO := request.EvaluatorDO
	if evaluatorDO == nil {
		return nil, errorx.NewByCode(errno.EvaluatorNotExistCode, errorx.WithExtraMsg("evaluator is nil"))
	}
	if evaluatorDO.EvaluatorType != entity.EvaluatorTypeAgent {
		return nil, errorx.NewByCode(errno.InvalidEvaluatorTypeCode, errorx.WithExtraMsg("async debug only supports Agent evaluator type"))
	}
	if allow := e.limiter.AllowInvoke(ctx, request.SpaceID); !allow {
		return nil, errorx.NewByCode(errno.EvaluatorQPSLimitCode, errorx.WithExtraMsg("evaluator throttled due to space-level rate limit"))
	}
	invokeID, err := e.idgen.GenID(ctx)
	if err != nil {
		return nil, err
	}
	evaluatorSourceService, ok := e.evaluatorSourceServices[evaluatorDO.EvaluatorType]
	if !ok {
		return nil, errorx.NewByCode(errno.InvalidEvaluatorTypeCode, errorx.WithExtraMsg("evaluator source service not found for agent type"))
	}
	asyncDebugExt, traceID, err := evaluatorSourceService.AsyncDebug(ctx, evaluatorDO, request.InputData, request.EvaluatorRunConf, request.SpaceID, invokeID)
	if err != nil {
		logs.CtxError(ctx, "[AsyncDebugEvaluator] AsyncDebug fail, invokeID: %d, err: %v", invokeID, err)
		return nil, err
	}

	userIDInContext := session.UserIDInCtxOrEmpty(ctx)
	logID := logs.GetLogID(ctx)
	status := entity.EvaluatorRunStatusAsyncInvoking
	outputData := &entity.EvaluatorOutputData{
		Ext: asyncDebugExt,
	}
	recordDO := &entity.EvaluatorRecord{
		ID:                  invokeID,
		SpaceID:             request.SpaceID,
		EvaluatorVersionID:  evaluatorDO.GetEvaluatorVersionID(),
		TraceID:             traceID,
		LogID:               logID,
		EvaluatorInputData:  request.InputData,
		EvaluatorOutputData: outputData,
		Status:              status,
		BaseInfo: &entity.BaseInfo{
			CreatedBy: &entity.UserInfo{
				UserID: gptr.Of(userIDInContext),
			},
			UpdatedBy: &entity.UserInfo{
				UserID: gptr.Of(userIDInContext),
			},
			CreatedAt: gptr.Of(time.Now().UnixMilli()),
			UpdatedAt: gptr.Of(time.Now().UnixMilli()),
		},
	}
	if err := e.evaluatorRecordRepo.CreateEvaluatorRecord(ctx, recordDO); err != nil {
		logs.CtxError(ctx, "[AsyncDebugEvaluator] CreateEvaluatorRecord fail, invokeID: %d, err: %v", invokeID, err)
		return nil, err
	}

	logs.CtxInfo(ctx, "[AsyncDebugEvaluator] invokeID: %d, traceID: %s, spaceID: %d", invokeID, traceID, request.SpaceID)
	return &entity.AsyncDebugEvaluatorResponse{
		InvokeID: invokeID,
		TraceID:  traceID,
	}, nil
}

// ReportEvaluatorInvokeResult 上报评估器异步执行结果 using a terminal CAS.
func (e *EvaluatorServiceImpl) ReportEvaluatorInvokeResult(ctx context.Context, param *entity.ReportEvaluatorRecordParam) (entity.ReportEvaluatorResultOutcome, error) {
	if param == nil {
		return 0, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("report evaluator result param is nil"))
	}
	if param.Status != entity.EvaluatorRunStatusSuccess && param.Status != entity.EvaluatorRunStatusFail {
		return 0, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("report evaluator status must be success or fail"))
	}
	logs.CtxInfo(ctx, "[ReportEvaluatorInvokeResult] recordID: %d, spaceID: %d, status: %v", param.RecordID, param.SpaceID, param.Status)

	existingRecord, err := e.evaluatorRecordRepo.GetEvaluatorRecord(contexts.WithCtxWriteDB(ctx), param.RecordID, false)
	if err != nil {
		logs.CtxError(ctx, "[ReportEvaluatorInvokeResult] GetEvaluatorRecord fail, recordID: %d, err: %v", param.RecordID, err)
		return 0, err
	}
	if existingRecord == nil {
		return 0, errorx.NewByCode(errno.EvaluatorRecordNotFoundCode, errorx.WithExtraMsg("evaluator record not found"))
	}
	if existingRecord.SpaceID != param.SpaceID {
		logs.CtxWarn(ctx, "[ReportEvaluatorInvokeResult] spaceID mismatch, recordID: %d, requestSpaceID: %d, recordSpaceID: %d",
			param.RecordID, param.SpaceID, existingRecord.SpaceID)
		return 0, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("spaceID mismatch"))
	}

	if existingRecord.Status != entity.EvaluatorRunStatusAsyncInvoking {
		if existingRecord.Status == param.Status {
			return entity.ReportEvaluatorResultDuplicate, nil
		}
		return entity.ReportEvaluatorResultConflict, nil
	}

	mergedOutputData := param.OutputData
	if mergedOutputData == nil {
		mergedOutputData = &entity.EvaluatorOutputData{}
	}
	if mergedOutputData.TimeConsumingMS == 0 && existingRecord.BaseInfo != nil && existingRecord.BaseInfo.CreatedAt != nil {
		if elapsedMS := time.Now().UnixMilli() - gptr.Indirect(existingRecord.BaseInfo.CreatedAt); elapsedMS > 0 {
			mergedOutputData.TimeConsumingMS = elapsedMS
		}
	}
	if existingRecord.EvaluatorOutputData != nil && existingRecord.EvaluatorOutputData.Ext != nil {
		if mergedOutputData.Ext == nil {
			mergedOutputData.Ext = make(map[string]string)
		}
		for k, v := range existingRecord.EvaluatorOutputData.Ext {
			if _, exists := mergedOutputData.Ext[k]; !exists {
				mergedOutputData.Ext[k] = v
			}
		}
	}

	updated, err := e.evaluatorRecordRepo.CompareAndSwapEvaluatorRecordResult(ctx, param.RecordID, param.SpaceID, entity.EvaluatorRunStatusAsyncInvoking, param.Status, mergedOutputData)
	if err != nil {
		return 0, err
	}
	if updated {
		return entity.ReportEvaluatorResultApplied, nil
	}

	latestRecord, err := e.evaluatorRecordRepo.GetEvaluatorRecord(contexts.WithCtxWriteDB(ctx), param.RecordID, false)
	if err != nil {
		return 0, err
	}
	if latestRecord != nil && latestRecord.Status == param.Status {
		logs.CtxWarn(ctx, "[ReportEvaluatorInvokeResult] duplicate terminal callback, recordID: %d, status: %v", param.RecordID, param.Status)
		return entity.ReportEvaluatorResultDuplicate, nil
	}
	logs.CtxWarn(ctx, "[ReportEvaluatorInvokeResult] conflicting terminal callback, recordID: %d, reportStatus: %v", param.RecordID, param.Status)
	return entity.ReportEvaluatorResultConflict, nil
}

func (e *EvaluatorServiceImpl) ArmEvaluatorResume(ctx context.Context, recordID int64) error {
	if e.evalAsyncRepo == nil {
		return errorx.New("eval async repo is nil")
	}
	asyncCtxKey := fmt.Sprintf("evaluator:%d", recordID)
	var (
		actx *entity.EvalAsyncCtx
		err  error
	)
	for _, delay := range []time.Duration{0, 50 * time.Millisecond, 100 * time.Millisecond} {
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
		actx, err = e.evalAsyncRepo.MarkEvalAsyncResumeReady(ctx, asyncCtxKey)
		if err == nil {
			break
		}
	}
	if err != nil {
		return err
	}
	if actx == nil || actx.Event == nil {
		return nil
	}
	record, err := e.evaluatorRecordRepo.GetEvaluatorRecord(contexts.WithCtxWriteDB(ctx), recordID, false)
	if err != nil {
		return err
	}
	if record == nil {
		return errorx.NewByCode(errno.EvaluatorRecordNotFoundCode, errorx.WithExtraMsg("evaluator record not found"))
	}
	if record.Status == entity.EvaluatorRunStatusAsyncInvoking {
		return nil
	}
	return e.publishEvaluatorResumeEvent(ctx, actx.Event)
}

func (e *EvaluatorServiceImpl) publishEvaluatorResumeEvent(ctx context.Context, event *entity.ExptItemEvalEvent) error {
	if event == nil || e.exptEventPublisher == nil {
		return nil
	}
	delays := []time.Duration{0, 50 * time.Millisecond, 100 * time.Millisecond}
	var lastErr error
	for _, delay := range delays {
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
		lastErr = e.exptEventPublisher.PublishExptRecordEvalEvent(ctx, event, gptr.Of(time.Second*3), func(event *entity.ExptItemEvalEvent) {
			event.AsyncEvaluatorReportTrigger = true
		})
		if lastErr == nil {
			return nil
		}
	}
	return lastErr
}

// DebugEvaluator 调试 evaluator_version
func (e *EvaluatorServiceImpl) DebugEvaluator(ctx context.Context, evaluatorDO *entity.Evaluator, inputData *entity.EvaluatorInputData, evaluatorRunConf *entity.EvaluatorRunConfig, exptSpaceID int64) (*entity.EvaluatorOutputData, error) {
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
	outputData, err := evaluatorSourceService.Debug(ctx, evaluatorDO, inputData, evaluatorRunConf, exptSpaceID)
	if err != nil {
		return nil, err
	}
	// 调试场景也统一对输出分数保留两位小数
	roundEvaluatorOutputScore(outputData)
	return outputData, nil
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
