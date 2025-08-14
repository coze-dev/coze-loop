// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/data/domain/entity"
)

//go:generate mockgen -destination=mocks/user_provider.go -package=mocks . IUserProvider
type IUserProvider interface {
	MGetUserInfo(ctx context.Context, userIDs []string) ([]*entity.UserInfo, error)
}
