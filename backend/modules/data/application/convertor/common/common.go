// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/common"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/entity"
)

// ConvertUserInfoDO2DTO 将 UserInfo 结构体转换为 DTO
func ConvertUserInfoDO2DTO(info *entity.UserInfo) *common.UserInfo {
	if info == nil {
		return nil
	}
	return &common.UserInfo{
		Name:        info.Name,
		EnName:      info.EnName,
		AvatarURL:   info.AvatarURL,
		AvatarThumb: info.AvatarThumb,
		OpenID:      info.OpenID,
		UnionID:     info.UnionID,
		UserID:      info.UserID,
		Email:       info.Email,
	}
}
