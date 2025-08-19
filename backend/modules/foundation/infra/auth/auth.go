// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"context"
	"strconv"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/auth"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/auth/authservice"
	authentity "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/domain/auth"
	"github.com/coze-dev/coze-loop/backend/modules/foundation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/foundation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
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

	spaceID, err := strconv.ParseInt(workspaceId, 10, 64)
	if err != nil {
		return errorx.NewByCode(errno.CommonInternalErrorCode)
	}
	req := &auth.MCheckPermissionRequest{
		Auths:   authInfos,
		SpaceID: ptr.Of(spaceID),
	}
	resp, err := a.cli.MCheckPermission(ctx, req)
	if err != nil {
		return errorx.NewByCode(errno.CommonRPCErrorCode, errorx.WithExtraMsg(err.Error()))
	} else if resp == nil {
		return errorx.NewByCode(errno.CommonRPCErrorCode)
	} else if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		return errorx.NewByCode(errno.CommonRPCErrorCode, errorx.WithExtraMsg(resp.BaseResp.StatusMessage))
	}
	for _, r := range resp.AuthRes {
		if r != nil && !r.GetIsAllowed() {
			return errorx.NewByCode(errno.CommonNoPermissionCode)
		}
	}
	return nil
}

func NewAuthProvider(cli authservice.Client) rpc.IAuthProvider {
	return &AuthProviderImpl{
		cli: cli,
	}
}
