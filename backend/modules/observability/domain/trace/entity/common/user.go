// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package common

type UserInfo struct {
	Name        string
	EnName      string
	AvatarURL   string
	AvatarThumb string
	OpenID      string
	UnionID     string
	UserID      string
	Email       string
}

type BaseInfo struct {
	CreatedBy UserInfo
	UpdatedBy UserInfo
	CreatedAt int64
	UpdatedAt int64
}
