// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/user"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/user/userservice"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/common"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/samber/lo"
)

type UserRPCAdapter struct {
	client userservice.Client
}

func NewUserRPCProvider(client userservice.Client) rpc.IUserProvider {
	return &UserRPCAdapter{
		client: client,
	}
}

func (u *UserRPCAdapter) GetUserInfo(ctx context.Context, userIDs []string) ([]*common.UserInfo, map[string]*common.UserInfo, error) {
	if len(userIDs) == 0 {
		return nil, nil, nil
	}
	req := &user.MGetUserInfoRequest{
		UserIds: userIDs,
	}
	resp, err := u.client.MGetUserInfo(ctx, req)
	if err != nil {
		logs.CtxWarn(ctx, "get user info failed: %v", err)
		return nil, nil, err
	} else if resp == nil {
		return nil, nil, nil
	}
	userInfos := make([]*common.UserInfo, 0)
	for _, dto := range resp.GetUserInfos() {
		if dto == nil {
			continue
		}
		userInfos = append(userInfos, &common.UserInfo{
			UserID:    dto.GetUserID(),
			Name:      dto.GetNickName(),
			AvatarURL: dto.GetAvatarURL(),
			Email:     dto.GetEmail(),
		})
	}
	userMap := lo.Associate(userInfos, func(item *common.UserInfo) (string, *common.UserInfo) {
		return item.UserID, item
	})
	return userInfos, userMap, nil
}
