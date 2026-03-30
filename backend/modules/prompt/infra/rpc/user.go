// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/user"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/user/userservice"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/rpc/convertor"
)

type UserRPCAdapter struct {
	client userservice.Client
}

func NewUserRPCProvider(client userservice.Client) rpc.IUserProvider {
	return &UserRPCAdapter{
		client: client,
	}
}

func (u *UserRPCAdapter) MGetUserInfo(ctx context.Context, userIDs []string) (userInfos []*rpc.UserInfo, err error) {
	if len(userIDs) <= 0 {
		return nil, nil
	}

	req := &user.MGetUserInfoRequest{
		UserIds: userIDs,
	}
	resp, err := u.client.MGetUserInfo(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}
	return convertor.BatchUserDTO2DO(resp.GetUserInfos()), nil
}

func (u *UserRPCAdapter) GetUserIdInCtx(ctx context.Context) (string, bool) {
	return session.UserIDInCtx(ctx)
}
