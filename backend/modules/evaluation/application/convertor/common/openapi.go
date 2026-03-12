// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"strings"
	"time"

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

// OpenAPIContentTypeDO2DTO entity ContentType（首字母大写，如 Text）-> OpenAPI 小写（如 text），DB 存大写
func OpenAPIContentTypeDO2DTO(ct commonentity.ContentType) string {
	switch ct {
	case commonentity.ContentTypeText:
		return openapiCommon.ContentTypeText
	case commonentity.ContentTypeImage:
		return openapiCommon.ContentTypeImage
	case commonentity.ContentTypeAudio:
		return openapiCommon.ContentTypeAudio
	case commonentity.ContentTypeVideo:
		return openapiCommon.ContentTypeVideo
	case commonentity.ContentTypeMultipart:
		return openapiCommon.ContentTypeMultiPart
	case commonentity.ContentTypeMultipartVariable:
		return openapiCommon.ContentTypeMultiPartVariable
	default:
		s := strings.TrimSpace(string(ct))
		if len(s) >= 1 {
			return strings.ToLower(s[:1]) + s[1:]
		}
		return openapiCommon.ContentTypeText
	}
}

// OpenAPIContentTypeDTO2DO OpenAPI 小写（如 text）-> entity ContentType（首字母大写），DB 存大写
func OpenAPIContentTypeDTO2DO(ct string) commonentity.ContentType {
	switch ct {
	case openapiCommon.ContentTypeText:
		return commonentity.ContentTypeText
	case openapiCommon.ContentTypeImage:
		return commonentity.ContentTypeImage
	case openapiCommon.ContentTypeAudio:
		return commonentity.ContentTypeAudio
	case openapiCommon.ContentTypeVideo:
		return commonentity.ContentTypeVideo
	case openapiCommon.ContentTypeMultiPart:
		return commonentity.ContentTypeMultipart
	case openapiCommon.ContentTypeMultiPartVariable:
		return commonentity.ContentTypeMultipartVariable
	default:
		s := strings.TrimSpace(ct)
		if len(s) >= 1 {
			return commonentity.ContentType(strings.ToUpper(s[:1]) + strings.ToLower(s[1:]))
		}
		return commonentity.ContentTypeText
	}
}

func OpenAPIImageDO2DTO(img *commonentity.Image) *openapiCommon.Image {
	if img == nil {
		return nil
	}
	return &openapiCommon.Image{
		Name:     img.Name,
		URL:      img.URL,
		ThumbURL: img.ThumbURL,
	}
}

func OpenAPIImageDTO2DO(img *openapiCommon.Image) *commonentity.Image {
	if img == nil {
		return nil
	}
	return &commonentity.Image{
		Name:     img.Name,
		URL:      img.URL,
		URI:      nil, // openapi Image 无 URI 字段
		ThumbURL: img.ThumbURL,
	}
}

func OpenAPIVideoDO2DTO(v *commonentity.Video) *openapiCommon.Video {
	if v == nil {
		return nil
	}
	return &openapiCommon.Video{
		Name:     v.Name,
		URL:      v.URL,
		URI:      v.URI,
		ThumbURL: v.ThumbURL,
	}
}

func OpenAPIVideoDTO2DO(v *openapiCommon.Video) *commonentity.Video {
	if v == nil {
		return nil
	}
	return &commonentity.Video{
		Name:     v.Name,
		URL:      v.URL,
		URI:      v.URI,
		ThumbURL: v.ThumbURL,
	}
}

func OpenAPIAudioDO2DTO(a *commonentity.Audio) *openapiCommon.Audio {
	if a == nil {
		return nil
	}
	return &openapiCommon.Audio{
		Format: a.Format,
		URL:    a.URL,
		Name:   a.Name,
		URI:    a.URI,
	}
}

func OpenAPIAudioDTO2DO(a *openapiCommon.Audio) *commonentity.Audio {
	if a == nil {
		return nil
	}
	return &commonentity.Audio{
		Format: a.Format,
		URL:    a.URL,
		Name:   a.Name,
		URI:    a.URI,
	}
}

func OpenAPIArgsSchemaDO2DTO(schema *commonentity.ArgsSchema) *openapiCommon.ArgsSchema {
	if schema == nil {
		return nil
	}
	contentTypes := make([]string, 0, len(schema.SupportContentTypes))
	for _, ct := range schema.SupportContentTypes {
		contentTypes = append(contentTypes, OpenAPIContentTypeDO2DTO(ct))
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
		contentTypes = append(contentTypes, OpenAPIContentTypeDTO2DO(ct))
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
		str := OpenAPIContentTypeDO2DTO(*content.ContentType)
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
		Image:            OpenAPIImageDO2DTO(content.Image),
		Video:            OpenAPIVideoDO2DTO(content.Video),
		Audio:            OpenAPIAudioDO2DTO(content.Audio),
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
		ct := OpenAPIContentTypeDTO2DO(*content.ContentType)
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
		Image:            OpenAPIImageDTO2DO(content.Image),
		Video:            OpenAPIVideoDTO2DO(content.Video),
		Audio:            OpenAPIAudioDTO2DO(content.Audio),
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

// OpenAPIRuntimeParamDO2DTO entity.RuntimeParam -> openapi common.RuntimeParam
func OpenAPIRuntimeParamDO2DTO(do *commonentity.RuntimeParam) *openapiCommon.RuntimeParam {
	if do == nil {
		return nil
	}
	return &openapiCommon.RuntimeParam{
		JSONValue: do.JSONValue,
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

// OpenAPIRateLimitDO2DTO entity.RateLimit -> domain_openapi/common.RateLimit（用于 CustomRPCEvaluator 等）
func OpenAPIRateLimitDO2DTO(rateLimit *commonentity.RateLimit) *openapiCommon.RateLimit {
	if rateLimit == nil {
		return nil
	}
	var period *string
	if rateLimit.Period != nil {
		period = gptr.Of(rateLimit.Period.String())
	}
	return &openapiCommon.RateLimit{
		Rate:   rateLimit.Rate,
		Burst:  rateLimit.Burst,
		Period: period,
	}
}

// OpenAPIRateLimitDTO2DO domain_openapi/common.RateLimit -> entity.RateLimit
func OpenAPIRateLimitDTO2DO(limit *openapiCommon.RateLimit) (*commonentity.RateLimit, error) {
	if limit == nil {
		return nil, nil
	}
	var period *time.Duration
	if limit.Period != nil && *limit.Period != "" {
		p, err := time.ParseDuration(*limit.Period)
		if err != nil {
			return nil, err
		}
		period = &p
	}
	return &commonentity.RateLimit{
		Rate:   limit.Rate,
		Burst:  limit.Burst,
		Period: period,
	}, nil
}
