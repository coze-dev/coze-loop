// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// UserInfo 用户信息结构体
type UserInfo struct {
	Name        *string `json:"name,omitempty"`
	EnName      *string `json:"en_name,omitempty"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
	AvatarThumb *string `json:"avatar_thumb,omitempty"`
	OpenID      *string `json:"open_id,omitempty"`
	UnionID     *string `json:"union_id,omitempty"`
	UserID      *string `json:"user_id,omitempty"`
	Email       *string `json:"email,omitempty"`
}

// BaseInfo 基础信息结构体
type BaseInfo struct {
	CreatedBy *UserInfo `json:"created_by,omitempty"`
	UpdatedBy *UserInfo `json:"updated_by,omitempty"`
	CreatedAt *int64    `json:"created_at,omitempty"`
	UpdatedAt *int64    `json:"updated_at,omitempty"`
	DeletedAt *int64    `json:"deleted_at,omitempty"`
}

func (do *BaseInfo) GetCreatedBy() *UserInfo {
	return do.CreatedBy
}

func (do *BaseInfo) SetCreatedBy(createdBy *UserInfo) {
	do.CreatedBy = createdBy
}

func (do *BaseInfo) GetUpdatedBy() *UserInfo {
	return do.UpdatedBy
}

func (do *BaseInfo) SetUpdatedBy(updatedBy *UserInfo) {
	do.UpdatedBy = updatedBy
}
