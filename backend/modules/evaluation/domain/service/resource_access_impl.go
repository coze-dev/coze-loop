// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"strconv"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// resourceAccessAuthorizerImpl 复用现有鉴权 provider + 共享配置 provider，
// 屏蔽底层配置存储差异（OSS 默认空配置 fail-closed，commercial 通过 wire 绑定 TCC 实现）。
type resourceAccessAuthorizerImpl struct {
	auth           rpc.IAuthProvider
	configProvider component.SharedResourceConfigProvider
}

func NewResourceAccessAuthorizer(
	auth rpc.IAuthProvider,
	configProvider component.SharedResourceConfigProvider,
) ResourceAccessAuthorizer {
	return &resourceAccessAuthorizerImpl{
		auth:           auth,
		configProvider: configProvider,
	}
}

// AuthorizeRead 统一读授权入口：
//  1. 非共享 → 本空间基础鉴权（ResourceSpaceID = CallerSpaceID），返回 direct context。
//  2. 共享 → 校验 source>0 且 !=caller；拉全量配置（失败/空即 fail-closed）；
//     Lookup 命中来源→资源→调用方；requireContentRead 校验 readable；版本策略校验；
//     本空间基础鉴权（SpaceID=caller, ResourceSpaceID=source）；返回 shared context。
func (a *resourceAccessAuthorizerImpl) AuthorizeRead(ctx context.Context, req *entity.AuthorizeResourceRequest) (*entity.ResourceAccessContext, error) {
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("authorize request is nil"))
	}
	if req.SharedOption == nil || !req.SharedOption.Enabled() {
		return a.authorizeDirectRead(ctx, req)
	}
	return a.authorizeSharedRead(ctx, req)
}

// authorizeDirectRead 普通访问：ResourceSpaceID = CallerSpaceID，行为与旧逻辑等价。
func (a *resourceAccessAuthorizerImpl) authorizeDirectRead(ctx context.Context, req *entity.AuthorizeResourceRequest) (*entity.ResourceAccessContext, error) {
	accessCtx := &entity.ResourceAccessContext{
		CallerSpaceID:   req.CallerSpaceID,
		ResourceSpaceID: req.CallerSpaceID,
		ResourceType:    req.ResourceType,
		ResourceID:      req.ResourceID,
		TargetType:      req.TargetType,
		AccessMode:      entity.AccessModeDirect,
		AccessLevel:     consts.Read,
		VersionPolicy:   entity.SharedVersionPolicyAll,
	}
	if err := a.callBaseAuth(ctx, accessCtx, req.Action, req.OwnerID); err != nil {
		return nil, err
	}
	return accessCtx, nil
}

// authorizeSharedRead 共享访问：fail-closed。
func (a *resourceAccessAuthorizerImpl) authorizeSharedRead(ctx context.Context, req *entity.AuthorizeResourceRequest) (*entity.ResourceAccessContext, error) {
	sourceSpaceID := *req.SharedOption.SourceSpaceID
	if sourceSpaceID <= 0 || sourceSpaceID == req.CallerSpaceID {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("invalid shared source space id"))
	}

	resolved, err := a.resolveShare(ctx, sourceSpaceID, req.ResourceType, req.TargetType, req.ResourceID, req.CallerSpaceID)
	if err != nil {
		return nil, err
	}
	if req.RequireContentRead && resolved.AccessLevel != entity.SharedAccessLevelReadable {
		return nil, errorx.NewByCode(errno.CommonNoPermissionCode, errorx.WithExtraMsg("shared access level does not allow content read"))
	}
	// 版本策略的逐版本过滤在资源加载后由 app 层用 IsSharedVersionAllowed 完成
	// （latest 需比对资源最新版本名，authorizer 不加载资源）。此处仅在 specified 策略下
	// 对显式请求的 version_id 做早拒，减少无谓加载。
	if req.ResourceType == entity.SharedResourceTypeEvalTarget {
		if err := earlyCheckVersionNamePolicy(resolved.VersionPolicy, resolved.SpecifiedVersions, req.VersionName); err != nil {
			return nil, err
		}
	} else {
		if err := earlyCheckVersionPolicy(resolved.VersionPolicy, resolved.SpecifiedIDs, req.VersionID); err != nil {
			return nil, err
		}
	}

	accessCtx := &entity.ResourceAccessContext{
		CallerSpaceID:     req.CallerSpaceID,
		ResourceSpaceID:   sourceSpaceID,
		ResourceType:      req.ResourceType,
		ResourceID:        req.ResourceID,
		TargetType:        resolved.TargetType,
		AccessMode:        entity.AccessModeShared,
		AccessLevel:       resolved.AccessLevel,
		VersionPolicy:     resolved.VersionPolicy,
		SpecifiedIDs:      resolved.SpecifiedIDs,
		SpecifiedVersions: resolved.SpecifiedVersions,
	}
	if err := a.callBaseAuth(ctx, accessCtx, req.Action, req.OwnerID); err != nil {
		return nil, err
	}
	return accessCtx, nil
}

// resolveShare 拉全量共享配置并 Lookup；任何失败 / 未命中一律 fail-closed。
func (a *resourceAccessAuthorizerImpl) resolveShare(ctx context.Context, sourceSpaceID int64, resourceType string, targetType entity.EvalTargetType, resourceID, callerSpaceID int64) (*entity.ResolvedShare, error) {
	if a.configProvider == nil {
		return nil, errorx.NewByCode(errno.CommonNoPermissionCode, errorx.WithExtraMsg("shared resource config unavailable"))
	}
	cfg, err := a.configProvider.GetSharedResourceConfig(ctx)
	if err != nil || cfg == nil {
		return nil, errorx.NewByCode(errno.CommonNoPermissionCode, errorx.WithExtraMsg("shared resource config unavailable"))
	}
	resolved := cfg.Lookup(sourceSpaceID, resourceType, targetType, resourceID, callerSpaceID)
	if resolved == nil {
		return nil, errorx.NewByCode(errno.CommonNoPermissionCode, errorx.WithExtraMsg("resource not shared to caller"))
	}
	return resolved, nil
}

// ListSharedResources 枚举共享给调用方的资源；空配置 → 空列表（fail-closed）。
func (a *resourceAccessAuthorizerImpl) ListSharedResources(ctx context.Context, req *entity.ListSharedResourcesRequest) ([]*entity.ResourceAccessContext, error) {
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("list shared request is nil"))
	}
	if req.SourceSpaceFilter != nil {
		src := *req.SourceSpaceFilter
		if src <= 0 || src == req.CallerSpaceID {
			return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("invalid shared source space id"))
		}
	}
	if a.configProvider == nil {
		return nil, nil
	}
	cfg, err := a.configProvider.GetSharedResourceConfig(ctx)
	if err != nil {
		return nil, errorx.NewByCode(errno.CommonNoPermissionCode, errorx.WithExtraMsg("shared resource config unavailable"))
	}
	if cfg == nil {
		return nil, nil
	}
	entries := cfg.ListSharedTo(req.CallerSpaceID, req.ResourceType, req.TargetType, req.SourceSpaceFilter)
	res := make([]*entity.ResourceAccessContext, 0, len(entries))
	for _, e := range entries {
		if e == nil {
			continue
		}
		res = append(res, &entity.ResourceAccessContext{
			CallerSpaceID:     req.CallerSpaceID,
			ResourceSpaceID:   e.SourceSpaceID,
			ResourceType:      e.ResourceType,
			ResourceID:        e.ResourceID,
			TargetType:        e.TargetType,
			AccessMode:        entity.AccessModeShared,
			AccessLevel:       e.AccessLevel,
			VersionPolicy:     e.VersionPolicy,
			SpecifiedIDs:      e.SpecifiedIDs,
			SpecifiedVersions: e.SpecifiedVersions,
		})
	}
	return res, nil
}

// callBaseAuth 统一封装基础鉴权调用：SpaceID=调用方（准入/审计），ResourceSpaceID=资源空间（归属校验）。
func (a *resourceAccessAuthorizerImpl) callBaseAuth(ctx context.Context, accessCtx *entity.ResourceAccessContext, action string, ownerID *string) error {
	entityType, err := mapAuthEntityType(accessCtx.ResourceType)
	if err != nil {
		return err
	}
	if action == "" {
		action = consts.Read
	}
	return a.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(accessCtx.ResourceID, 10),
		SpaceID:         accessCtx.CallerSpaceID,
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(action), EntityType: gptr.Of(entityType)}},
		OwnerID:         ownerID,
		ResourceSpaceID: accessCtx.ResourceSpaceID,
	})
}

func mapAuthEntityType(resourceType string) (rpc.AuthEntityType, error) {
	switch resourceType {
	case entity.SharedResourceTypeEvalSet:
		return rpc.AuthEntityType_EvaluationSet, nil
	case entity.SharedResourceTypeEvalTarget:
		return rpc.AuthEntityType_EvaluationTarget, nil
	default:
		return "", errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("unsupported resource type"))
	}
}
