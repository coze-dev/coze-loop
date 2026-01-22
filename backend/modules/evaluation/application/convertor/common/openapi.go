// Copyright (c) 2025 coze-dev Authors
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

func OpenAPIArgsSchemaDO2DTOs(dos []*commonentity.ArgsSchema) []*commondto.ArgsSchema {
	if len(dos) == 0 {
		return nil
	}
	result := make([]*commondto.ArgsSchema, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		result = append(result, OpenAPIArgsSchemaDO2DTO(do))
	}
	return result
}

func OpenAPIArgsSchemaDO2DTO(do *commonentity.ArgsSchema) *commondto.ArgsSchema {
	if do == nil {
		return nil
	}
	contentTypes := make([]commondto.ContentType, 0, len(do.SupportContentTypes))
	for _, ct := range do.SupportContentTypes {
		contentTypes = append(contentTypes, commondto.ContentType(ct))
	}
	return &commondto.ArgsSchema{
		Key:                 &do.Key,
		SupportContentTypes: contentTypes,
		JSONSchema:          &do.JsonSchema,
	}
}
