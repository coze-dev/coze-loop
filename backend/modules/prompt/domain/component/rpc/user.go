// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
)

type UserInfo struct {
	UserID    string `json:"user_id"`
	UserName  string `json:"user_name"`
	NickName  string `json:"nick_name"`
	AvatarURL string `json:"avatar_url"`
	Email     string `json:"email"`
	Mobile    string `json:"mobile"`
}

//go:generate mockgen -destination=mocks/user_provider.go -package=mocks . IUserProvider
type IUserProvider interface {
	MGetUserInfo(ctx context.Context, userIDs []string) (userInfos []*UserInfo, err error)
	GetUserIdInCtx(ctx context.Context) (string, bool)
}
