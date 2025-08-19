// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package foundation

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/user"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/user/userservice"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

type UserRPCAdapter struct {
	client userservice.Client
}

func NewUserRPCProvider(client userservice.Client) rpc.IUserProvider {
	return &UserRPCAdapter{
		client: client,
	}
}

func (u UserRPCAdapter) MGetUserInfo(ctx context.Context, userIDs []string) ([]*entity.UserInfo, error) {
	resp, err := u.client.MGetUserInfo(ctx, &user.MGetUserInfoRequest{
		UserIds: userIDs,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errorx.NewByCode(errno.CommonRPCErrorCode)
	}
	if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		return nil, errorx.NewByCode(resp.BaseResp.StatusCode, errorx.WithExtraMsg(resp.BaseResp.StatusMessage))
	}
	res := make([]*entity.UserInfo, 0)
	for _, userInfo := range resp.UserInfos {
		if userInfo == nil {
			continue
		}
		res = append(res, &entity.UserInfo{
			Name:      userInfo.NickName,
			AvatarURL: userInfo.AvatarURL,
			// AvatarThumb: userInfo.AvatarThumb,
			Email:  userInfo.Email,
			UserID: userInfo.UserID,
		})
	}
	return res, nil
}
