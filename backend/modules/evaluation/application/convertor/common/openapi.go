// Copyright (c) 2025 Bytedance Ltd. and/or its affiliates
// SPDX-License-Identifier: Apache-2.0

package common

import (
	commondto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/common"
	commonentity "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func OpenAPIBaseInfoDO2DTO(info *commonentity.BaseInfo) *commondto.BaseInfo {
	if info == nil {
		return nil
	}
	return &commondto.BaseInfo{
		CreatedBy: OpenAPIUserInfoDO2DTO(info.CreatedBy),
		UpdatedBy: OpenAPIUserInfoDO2DTO(info.UpdatedBy),
		CreatedAt: info.CreatedAt,
		UpdatedAt: info.UpdatedAt,
	}
}

func OpenAPIUserInfoDO2DTO(info *commonentity.UserInfo) *commondto.UserInfo {
	if info == nil {
		return nil
	}
	return &commondto.UserInfo{
		Name:      info.Name,
		AvatarURL: info.AvatarURL,
		UserID:    info.UserID,
		Email:     info.Email,
	}
}
