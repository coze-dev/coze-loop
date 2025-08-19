// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"context"
	"strconv"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/auth"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/auth/authservice"
	authentity "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/domain/auth"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type AuthProviderImpl struct {
	cli authservice.Client
}

func (a *AuthProviderImpl) CheckWorkspacePermission(ctx context.Context, action, workspaceId string) error {
	authInfos := make([]*authentity.SubjectActionObjects, 0)
	authInfos = append(authInfos, &authentity.SubjectActionObjects{
		Subject: &authentity.AuthPrincipal{
			AuthPrincipalType: ptr.Of(authentity.AuthPrincipalType_CozeIdentifier),
			AuthCozeIdentifier: &authentity.AuthCozeIdentifier{
				IdentityTicket: nil,
			},
		},
		Action: ptr.Of(action),
		Objects: []*authentity.AuthEntity{
			{
				ID:         ptr.Of(workspaceId),
				EntityType: ptr.Of(authentity.AuthEntityTypeSpace),
			},
		},
	})

	// 将workspaceId字符串转换为int64
	spaceID, err := strconv.ParseInt(workspaceId, 10, 64)
	if err != nil {
		return errorx.NewByCode(obErrorx.CommonInternalErrorCode)
	}

	req := &auth.MCheckPermissionRequest{
		Auths:   authInfos,
		SpaceID: ptr.Of(spaceID),
	}
	resp, err := a.cli.MCheckPermission(ctx, req)
	if err != nil {
		return errorx.WrapByCode(err, obErrorx.CommercialCommonRPCErrorCodeCode)
	} else if resp == nil {
		logs.CtxWarn(ctx, "MCheckPermission returned nil response")
		return errorx.NewByCode(obErrorx.CommercialCommonRPCErrorCodeCode)
	} else if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		logs.CtxWarn(ctx, "MCheckPermission returned non-zero status code %d", resp.BaseResp.StatusCode)
		return errorx.NewByCode(obErrorx.CommercialCommonRPCErrorCodeCode)
	}
	for _, r := range resp.AuthRes {
		if r != nil && !r.GetIsAllowed() {
			return errorx.NewByCode(obErrorx.CommonNoPermissionCode)
		}
	}
	return nil
}

func (a *AuthProviderImpl) CheckViewPermission(ctx context.Context, action, workspaceId, viewId string) error {
	authInfos := make([]*authentity.SubjectActionObjects, 0)
	authInfos = append(authInfos, &authentity.SubjectActionObjects{
		Subject: &authentity.AuthPrincipal{
			AuthPrincipalType: ptr.Of(authentity.AuthPrincipalType_CozeIdentifier),
			AuthCozeIdentifier: &authentity.AuthCozeIdentifier{
				IdentityTicket: nil,
			},
		},
		Action: ptr.Of(action),
		Objects: []*authentity.AuthEntity{
			{
				ID:         ptr.Of(viewId),
				EntityType: ptr.Of(authentity.AuthEntityTypeTraceView),
			},
		},
	})

	// 将workspaceId字符串转换为int64
	spaceID, err := strconv.ParseInt(workspaceId, 10, 64)
	if err != nil {
		return errorx.NewByCode(obErrorx.CommonInternalErrorCode)
	}

	req := &auth.MCheckPermissionRequest{
		Auths:   authInfos,
		SpaceID: ptr.Of(spaceID),
	}
	resp, err := a.cli.MCheckPermission(ctx, req)
	if err != nil {
		return errorx.WrapByCode(err, obErrorx.CommercialCommonRPCErrorCodeCode)
	} else if resp == nil {
		logs.CtxWarn(ctx, "MCheckPermission returned nil response")
		return errorx.NewByCode(obErrorx.CommercialCommonRPCErrorCodeCode)
	} else if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		logs.CtxWarn(ctx, "MCheckPermission returned non-zero status code %d", resp.BaseResp.StatusCode)
		return errorx.NewByCode(obErrorx.CommercialCommonRPCErrorCodeCode)
	}
	for _, r := range resp.AuthRes {
		if r != nil && !r.GetIsAllowed() {
			return errorx.NewByCode(obErrorx.CommonNoPermissionCode)
		}
	}
	return nil
}

func NewAuthProvider(cli authservice.Client) rpc.IAuthProvider {
	return &AuthProviderImpl{
		cli: cli,
	}
}
