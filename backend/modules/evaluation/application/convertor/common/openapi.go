// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"github.com/bytedance/gg/gptr"
	openapiCommon "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/common"
	commonentity "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func OpenAPIBaseInfoDO2DTO(info *commonentity.BaseInfo) *openapiCommon.BaseInfo {
	if info == nil {
		return nil
	}
	return &openapiCommon.BaseInfo{
		CreatedBy: OpenAPIUserInfoDO2DTO(info.CreatedBy),
		UpdatedBy: OpenAPIUserInfoDO2DTO(info.UpdatedBy),
		CreatedAt: info.CreatedAt,
		UpdatedAt: info.UpdatedAt,
	}
}

func OpenAPIUserInfoDO2DTO(info *commonentity.UserInfo) *openapiCommon.UserInfo {
	if info == nil {
		return nil
	}
	return &openapiCommon.UserInfo{
		Name:      info.Name,
		AvatarURL: info.AvatarURL,
		UserID:    info.UserID,
		Email:     info.Email,
	}
}

func OpenAPIArgsSchemaDO2DTO(schema *commonentity.ArgsSchema) *openapiCommon.ArgsSchema {
	if schema == nil {
		return nil
	}
	contentTypes := make([]string, 0, len(schema.SupportContentTypes))
	for _, ct := range schema.SupportContentTypes {
		contentTypes = append(contentTypes, string(ct))
	}
	return &openapiCommon.ArgsSchema{
		Key:                 schema.Key,
		SupportContentTypes: contentTypes,
		JSONSchema:          schema.JsonSchema,
	}
}

func OpenAPIArgsSchemaDO2DTOs(schemas []*commonentity.ArgsSchema) []*openapiCommon.ArgsSchema {
	if len(schemas) == 0 {
		return nil
	}
	res := make([]*openapiCommon.ArgsSchema, 0, len(schemas))
	for _, schema := range schemas {
		res = append(res, OpenAPIArgsSchemaDO2DTO(schema))
	}
	return res
}

func OpenAPIArgsSchemaDTO2DO(schema *openapiCommon.ArgsSchema) *commonentity.ArgsSchema {
	if schema == nil {
		return nil
	}
	contentTypes := make([]commonentity.ContentType, 0, len(schema.SupportContentTypes))
	for _, ct := range schema.SupportContentTypes {
		contentTypes = append(contentTypes, commonentity.ContentType(ct))
	}
	return &commonentity.ArgsSchema{
		Key:                 schema.Key,
		SupportContentTypes: contentTypes,
		JsonSchema:          schema.JSONSchema,
	}
}

func OpenAPIArgsSchemaDTO2DOs(schemas []*openapiCommon.ArgsSchema) []*commonentity.ArgsSchema {
	if len(schemas) == 0 {
		return nil
	}
	res := make([]*commonentity.ArgsSchema, 0, len(schemas))
	for _, schema := range schemas {
		res = append(res, OpenAPIArgsSchemaDTO2DO(schema))
	}
	return res
}

func OpenAPIContentDO2DTO(content *commonentity.Content) *openapiCommon.Content {
	if content == nil {
		return nil
	}
	var contentTypeStr *string
	if content.ContentType != nil {
		str := string(*content.ContentType)
		contentTypeStr = &str
	}
	var multiPart []*openapiCommon.Content
	if content.MultiPart != nil {
		multiPart = make([]*openapiCommon.Content, 0, len(content.MultiPart))
		for _, part := range content.MultiPart {
			multiPart = append(multiPart, OpenAPIContentDO2DTO(part))
		}
	}
	return &openapiCommon.Content{
		ContentType:      contentTypeStr,
		Text:             content.Text,
		MultiPart:        multiPart,
		ContentOmitted:   content.ContentOmitted,
		FullContentBytes: content.FullContentBytes,
	}
}

func OpenAPIContentDTO2DO(content *openapiCommon.Content) *commonentity.Content {
	if content == nil {
		return nil
	}
	var contentType *commonentity.ContentType
	if content.ContentType != nil {
		ct := commonentity.ContentType(*content.ContentType)
		contentType = &ct
	}
	var multiPart []*commonentity.Content
	if content.MultiPart != nil {
		multiPart = make([]*commonentity.Content, 0, len(content.MultiPart))
		for _, part := range content.MultiPart {
			multiPart = append(multiPart, OpenAPIContentDTO2DO(part))
		}
	}
	return &commonentity.Content{
		ContentType:      contentType,
		Text:             content.Text,
		MultiPart:        multiPart,
		ContentOmitted:   content.ContentOmitted,
		FullContentBytes: content.FullContentBytes,
	}
}

func OpenAPIContentDTO2DOs(contents map[string]*openapiCommon.Content) map[string]*commonentity.Content {
	if len(contents) == 0 {
		return nil
	}
	res := make(map[string]*commonentity.Content, len(contents))
	for k, v := range contents {
		res[k] = OpenAPIContentDTO2DO(v)
	}
	return res
}

func OpenAPIMessageDO2DTO(msg *commonentity.Message) *openapiCommon.Message {
	if msg == nil {
		return nil
	}
	role := OpenAPIRoleDO2DTO(msg.Role)
	return &openapiCommon.Message{
		Role:    &role,
		Content: OpenAPIContentDO2DTO(msg.Content),
		Ext:     msg.Ext,
	}
}

func OpenAPIMessageDO2DTOs(msgs []*commonentity.Message) []*openapiCommon.Message {
	if len(msgs) == 0 {
		return nil
	}
	res := make([]*openapiCommon.Message, 0, len(msgs))
	for _, msg := range msgs {
		res = append(res, OpenAPIMessageDO2DTO(msg))
	}
	return res
}

func OpenAPIMessageDTO2DO(msg *openapiCommon.Message) *commonentity.Message {
	if msg == nil {
		return nil
	}
	role := OpenAPIRoleDTO2DO(msg.Role)
	return &commonentity.Message{
		Role:    role,
		Content: OpenAPIContentDTO2DO(msg.Content),
		Ext:     msg.Ext,
	}
}

func OpenAPIRoleDO2DTO(role commonentity.Role) openapiCommon.Role {
	switch role {
	case commonentity.RoleSystem:
		return openapiCommon.RoleSystem
	case commonentity.RoleUser:
		return openapiCommon.RoleUser
	case commonentity.RoleAssistant:
		return openapiCommon.RoleAssistant
	default:
		return ""
	}
}

func OpenAPIRoleDTO2DO(role *openapiCommon.Role) commonentity.Role {
	if role == nil {
		return commonentity.RoleUndefined
	}
	switch *role {
	case openapiCommon.RoleSystem:
		return commonentity.RoleSystem
	case openapiCommon.RoleUser:
		return commonentity.RoleUser
	case openapiCommon.RoleAssistant:
		return commonentity.RoleAssistant
	default:
		return commonentity.RoleUndefined
	}
}

func OpenAPIMessageDTO2DOs(msgs []*openapiCommon.Message) []*commonentity.Message {
	if len(msgs) == 0 {
		return nil
	}
	res := make([]*commonentity.Message, 0, len(msgs))
	for _, msg := range msgs {
		res = append(res, OpenAPIMessageDTO2DO(msg))
	}
	return res
}

func OpenAPIModelConfigDO2DTO(config *commonentity.ModelConfig) *openapiCommon.ModelConfig {
	if config == nil {
		return nil
	}
	return &openapiCommon.ModelConfig{
		ModelID:     config.ModelID,
		ModelName:   gptr.Of(config.ModelName),
		Temperature: config.Temperature,
		MaxTokens:   config.MaxTokens,
		TopP:        config.TopP,
	}
}

func OpenAPIModelConfigDTO2DO(config *openapiCommon.ModelConfig) *commonentity.ModelConfig {
	if config == nil {
		return nil
	}
	return &commonentity.ModelConfig{
		ModelID:     config.ModelID,
		ModelName:   gptr.Indirect(config.ModelName),
		Temperature: config.Temperature,
		MaxTokens:   config.MaxTokens,
		TopP:        config.TopP,
	}
}

func OpenAPIRuntimeParamDTO2DO(dto *openapiCommon.RuntimeParam) *commonentity.RuntimeParam {
	if dto == nil {
		return nil
	}
	return &commonentity.RuntimeParam{
		JSONValue: dto.JSONValue,
	}
}

func OpenAPIOrderBysDTO2DO(dtos []*openapiCommon.OrderBy) []*commonentity.OrderBy {
	if len(dtos) == 0 {
		return nil
	}
	res := make([]*commonentity.OrderBy, 0, len(dtos))
	for _, dto := range dtos {
		res = append(res, &commonentity.OrderBy{
			Field: gptr.Of(dto.GetField()),
			IsAsc: gptr.Of(dto.GetIsAsc()),
		})
	}
	return res
}
