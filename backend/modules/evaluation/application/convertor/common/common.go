// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"strconv"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	commondto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	commonentity "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// ... 原有的结构体定义和常量 ...

// ConvertContentTypeDTO2DO 将 DTO 类型的字符串转换为 ContentType
func ConvertContentTypeDTO2DO(ct string) commonentity.ContentType {
	return commonentity.ContentType(ct)
}

// ConvertContentTypeDO2DTO 将 ContentType 转换为 DTO 类型
func ConvertContentTypeDO2DTO(ct commonentity.ContentType) string {
	return string(ct)
}

// ConvertImageDTO2DO 将 DTO 转换为 Image 结构体
func ConvertImageDTO2DO(img *commondto.Image) *commonentity.Image {
	if img == nil {
		return nil
	}
	return &commonentity.Image{
		Name:            img.Name,
		URL:             img.URL,
		URI:             img.URI,
		ThumbURL:        img.ThumbURL,
		StorageProvider: gptr.Of(commonentity.StorageProvider(gptr.Indirect(img.StorageProvider))),
	}
}

// ConvertImageDO2DTO 将 Image 结构体转换为 DTO
func ConvertImageDO2DTO(img *commonentity.Image) *commondto.Image {
	if img == nil {
		return nil
	}
	return &commondto.Image{
		Name:            img.Name,
		URL:             img.URL,
		URI:             img.URI,
		ThumbURL:        img.ThumbURL,
		StorageProvider: gptr.Of(dataset.StorageProvider(gptr.Indirect(img.StorageProvider))),
	}
}

func ConvertAudioDO2DTO(audio *commonentity.Audio) *commondto.Audio {
	if audio == nil {
		return nil
	}
	return &commondto.Audio{
		Format: audio.Format,
		URL:    audio.URL,
	}
}

// ConvertAudioDTO2DO 将 DTO 转换为 Audio 结构体
func ConvertAudioDTO2DO(audio *commondto.Audio) *commonentity.Audio {
	if audio == nil {
		return nil
	}
	return &commonentity.Audio{
		Format: audio.Format,
		URL:    audio.URL,
	}
}

// ConvertContentDTO2DO 将 DTO 转换为 Content 结构体
func ConvertContentDTO2DO(content *commondto.Content) *commonentity.Content {
	if content == nil {
		return nil
	}
	var contentType *commonentity.ContentType
	if content.ContentType != nil {
		ct := commonentity.ContentType(*content.ContentType)
		contentType = &ct
	}
	var format *commonentity.FieldDisplayFormat
	if content.Format != nil {
		f := commonentity.FieldDisplayFormat(*content.Format)
		format = &f
	}
	var multiPart []*commonentity.Content
	if content.MultiPart != nil {
		multiPart = make([]*commonentity.Content, 0, len(content.MultiPart))
		for _, part := range content.MultiPart {
			multiPart = append(multiPart, ConvertContentDTO2DO(part))
		}
	}
	return &commonentity.Content{
		ContentType: contentType,
		Format:      format,
		Text:        content.Text,
		Image:       ConvertImageDTO2DO(content.Image),
		MultiPart:   multiPart,
		Audio:       ConvertAudioDTO2DO(content.Audio),
	}
}

// ConvertContentDO2DTO 将 Content 结构体转换为 DTO
func ConvertContentDO2DTO(content *commonentity.Content) *commondto.Content {
	if content == nil {
		return nil
	}
	var contentTypeStr *string
	if content.ContentType != nil {
		str := string(*content.ContentType)
		contentTypeStr = &str
	}
	var multiPart []*commondto.Content
	if content.MultiPart != nil {
		multiPart = make([]*commondto.Content, 0, len(content.MultiPart))
		for _, part := range content.MultiPart {
			multiPart = append(multiPart, ConvertContentDO2DTO(part))
		}
	}
	return &commondto.Content{
		ContentType: contentTypeStr,
		Format:      (*dataset.FieldDisplayFormat)(content.Format),
		Text:        content.Text,
		Image:       ConvertImageDO2DTO(content.Image),
		MultiPart:   multiPart,
		Audio:       ConvertAudioDO2DTO(content.Audio),
	}
}

func ConvertOrderByDTO2DOs(orders []*commondto.OrderBy) []*commonentity.OrderBy {
	if orders == nil {
		return nil
	}
	res := make([]*commonentity.OrderBy, 0)
	for _, o := range orders {
		res = append(res, ConvertOrderByDTO2DO(o))
	}
	return res
}

// ConvertOrderByDTO2DO 将 DTO 转换为 OrderBy 结构体
func ConvertOrderByDTO2DO(order *commondto.OrderBy) *commonentity.OrderBy {
	if order == nil {
		return nil
	}
	return &commonentity.OrderBy{
		Field: order.Field,
		IsAsc: order.IsAsc,
	}
}

// ConvertOrderByDO2DTO 将 OrderBy 结构体转换为 DTO
func ConvertOrderByDO2DTO(order *commonentity.OrderBy) *commondto.OrderBy {
	if order == nil {
		return nil
	}
	return &commondto.OrderBy{
		Field: order.Field,
		IsAsc: order.IsAsc,
	}
}

// ConvertRoleDTO2DO 将 DTO 类型的 int64 转换为 Role 枚举
func ConvertRoleDTO2DO(role int64) commonentity.Role {
	return commonentity.Role(role)
}

// ConvertRoleDO2DTO 将 Role 枚举转换为 DTO 类型
func ConvertRoleDO2DTO(role commonentity.Role) int64 {
	return int64(role)
}

// ConvertMessageDTO2DO 将 DTO 转换为 Message 结构体
func ConvertMessageDTO2DO(msg *commondto.Message) *commonentity.Message {
	if msg == nil {
		return nil
	}
	var role commonentity.Role
	if msg.Role != nil {
		r := commonentity.Role(gptr.Indirect(msg.Role))
		role = r
	}
	return &commonentity.Message{
		Role:    role,
		Content: ConvertContentDTO2DO(msg.Content),
		Ext:     msg.Ext,
	}
}

// ConvertMessageDO2DTO 将 Message 结构体转换为 DTO
func ConvertMessageDO2DTO(msg *commonentity.Message) *commondto.Message {
	if msg == nil {
		return nil
	}
	var role *int64
	if msg.Role != commonentity.RoleUndefined {
		r := int64(msg.Role)
		role = &r
	}
	return &commondto.Message{
		Role:    (*commondto.Role)(role),
		Content: ConvertContentDO2DTO(msg.Content),
		Ext:     msg.Ext,
	}
}

// ConvertArgsSchemaDTO2DO 将 DTO 转换为 ArgsSchema 结构体
func ConvertArgsSchemaDTO2DO(schema *commondto.ArgsSchema) *commonentity.ArgsSchema {
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

// ConvertArgsSchemaDO2DTO 将 ArgsSchema 结构体转换为 DTO
func ConvertArgsSchemaDO2DTO(schema *commonentity.ArgsSchema) *commondto.ArgsSchema {
	if schema == nil {
		return nil
	}
	contentTypes := make([]string, 0, len(schema.SupportContentTypes))
	for _, ct := range schema.SupportContentTypes {
		contentTypes = append(contentTypes, string(ct))
	}
	return &commondto.ArgsSchema{
		Key:                 schema.Key,
		SupportContentTypes: contentTypes,
		JSONSchema:          schema.JsonSchema,
	}
}

// ConvertUserInfoDTO2DO 将 DTO 转换为 UserInfo 结构体
func ConvertUserInfoDTO2DO(info *commondto.UserInfo) *commonentity.UserInfo {
	if info == nil {
		return nil
	}
	return &commonentity.UserInfo{
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

// ConvertUserInfoDO2DTO 将 UserInfo 结构体转换为 DTO
func ConvertUserInfoDO2DTO(info *commonentity.UserInfo) *commondto.UserInfo {
	if info == nil {
		return nil
	}
	return &commondto.UserInfo{
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

// ConvertBaseInfoDTO2DO 将 DTO 转换为 BaseInfo 结构体
func ConvertBaseInfoDTO2DO(info *commondto.BaseInfo) *commonentity.BaseInfo {
	if info == nil {
		return nil
	}
	return &commonentity.BaseInfo{
		CreatedBy: ConvertUserInfoDTO2DO(info.CreatedBy),
		UpdatedBy: ConvertUserInfoDTO2DO(info.UpdatedBy),
		CreatedAt: info.CreatedAt,
		UpdatedAt: info.UpdatedAt,
		DeletedAt: info.DeletedAt,
	}
}

// ConvertBaseInfoDO2DTO 将 BaseInfo 结构体转换为 DTO
func ConvertBaseInfoDO2DTO(info *commonentity.BaseInfo) *commondto.BaseInfo {
	if info == nil {
		return nil
	}
	return &commondto.BaseInfo{
		CreatedBy: ConvertUserInfoDO2DTO(info.CreatedBy),
		UpdatedBy: ConvertUserInfoDO2DTO(info.UpdatedBy),
		CreatedAt: info.CreatedAt,
		UpdatedAt: info.UpdatedAt,
		DeletedAt: info.DeletedAt,
	}
}

// ConvertModelConfigDTO2DO 将 DTO 转换为 ModelConfig 结构体
func ConvertModelConfigDTO2DO(config *commondto.ModelConfig) *commonentity.ModelConfig {
	if config == nil {
		return nil
	}

	return &commonentity.ModelConfig{
		ModelID:     config.GetModelID(),
		ModelName:   gptr.Indirect(config.ModelName),
		Temperature: config.Temperature,
		MaxTokens:   config.MaxTokens,
		TopP:        config.TopP,
	}
}

// ConvertModelConfigDO2DTO 将 ModelConfig 结构体转换为 DTO
func ConvertModelConfigDO2DTO(config *commonentity.ModelConfig) *commondto.ModelConfig {
	if config == nil {
		return nil
	}

	dto := &commondto.ModelConfig{
		ModelID:     gptr.Of(config.ModelID),
		ModelName:   gptr.Of(config.ModelName),
		Temperature: config.Temperature,
		MaxTokens:   config.MaxTokens,
		TopP:        config.TopP,
	}
	if config.ModelID > 0 {
		dto.ModelID = gptr.Of(config.ModelID)
	} else if config.ProviderModelID != nil && len(gptr.Indirect(config.ProviderModelID)) > 0 {
		pModelID, err := strconv.ParseInt(gptr.Indirect(config.ProviderModelID), 10, 64)
		if err != nil {
			logs.CtxError(context.Background(), "failed to parse provider model id: %s, err: %v", gptr.Indirect(config.ProviderModelID), err)
		}
		dto.ModelID = gptr.Of(pModelID)
	}
	return dto
}

// ConvertFieldDisplayFormatDTO2DO 将 DTO 类型的 int64 转换为 FieldDisplayFormat 枚举
func ConvertFieldDisplayFormatDTO2DO(fdf int64) commonentity.FieldDisplayFormat {
	return commonentity.FieldDisplayFormat(fdf)
}

// ConvertFieldDisplayFormatDO2DTO 将 FieldDisplayFormat 枚举转换为 DTO 类型
func ConvertFieldDisplayFormatDO2DTO(fdf commonentity.FieldDisplayFormat) int64 {
	return int64(fdf)
}
