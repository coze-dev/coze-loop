// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// authorizeEvaluatorAccess 评估器（含跨空间共享）的统一读/执行授权。
//
//   - builtin 预置评估器：本就允许跨空间使用，直接返回调用方空间的 direct 上下文，不做额外鉴权
//     （与改动前各入口 `if !Builtin { auth }` 的行为一致）。
//   - 未声明共享：评估器必须属于调用方空间，否则返回 ResourceNotFound —— 不泄漏"该评估器存在"。
//   - 声明共享：走白名单 AuthorizeRead（fail-closed），并校验声明的来源空间与评估器真实归属一致，
//     防止调用方声明 B 却命中一个实际属于 C 的 version id。
//
// requireContentRead=true 用于会暴露评估器内容（prompt / input schema）的读取入口，
// 此时白名单必须命中 readable；执行入口传 false，execute 亦可放行。
func authorizeEvaluatorAccess(
	ctx context.Context,
	authorizer service.ResourceAccessAuthorizer,
	evaluator *entity.Evaluator,
	callerSpaceID int64,
	sharedOption *entity.SharedResourceOption,
	action string,
	requireContentRead bool,
) (*entity.ResourceAccessContext, error) {
	if evaluator == nil {
		return nil, errorx.NewByCode(errno.EvaluatorNotExistCode)
	}
	if evaluator.Builtin {
		return &entity.ResourceAccessContext{
			CallerSpaceID:   callerSpaceID,
			ResourceSpaceID: callerSpaceID,
			ResourceType:    entity.SharedResourceTypeEvaluator,
			ResourceID:      evaluator.ID,
			AccessMode:      entity.AccessModeDirect,
			VersionPolicy:   entity.SharedVersionPolicyAll,
		}, nil
	}
	if !sharedOption.Enabled() && evaluator.SpaceID != callerSpaceID {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("evaluator version not found"))
	}

	var ownerID *string
	if evaluator.BaseInfo != nil && evaluator.BaseInfo.CreatedBy != nil {
		ownerID = evaluator.BaseInfo.CreatedBy.UserID
	}
	accessCtx, err := authorizer.AuthorizeRead(ctx, &entity.AuthorizeResourceRequest{
		CallerSpaceID:      callerSpaceID,
		ResourceType:       entity.SharedResourceTypeEvaluator,
		ResourceID:         evaluator.ID,
		SharedOption:       sharedOption,
		Action:             action,
		OwnerID:            ownerID,
		RequireContentRead: requireContentRead,
	})
	if err != nil {
		return nil, err
	}
	if accessCtx.ResourceSpaceID != evaluator.SpaceID {
		return nil, errorx.NewByCode(errno.CommonNoPermissionCode, errorx.WithExtraMsg("evaluator version does not belong to declared source space"))
	}
	if accessCtx.IsShared() {
		logs.CtxInfo(ctx, "cross space evaluator authorized; evaluatorID=%d sourceSpace=%d callerSpace=%d accessLevel=%s",
			evaluator.ID, accessCtx.ResourceSpaceID, callerSpaceID, accessCtx.AccessLevel)
	}
	return accessCtx, nil
}
