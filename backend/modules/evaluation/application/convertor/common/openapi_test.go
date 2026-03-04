// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	openapiCommon "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/common"
	commonentity "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/assert"
)

func TestOpenAPIContentTypeDO2DTOAndDTO2DO(t *testing.T) {
	// OpenAPI 小写 <-> entity 首字母大写，DB 存大写
	assert.Equal(t, "text", OpenAPIContentTypeDO2DTO(commonentity.ContentTypeText))
	assert.Equal(t, "image", OpenAPIContentTypeDO2DTO(commonentity.ContentTypeImage))
	assert.Equal(t, "audio", OpenAPIContentTypeDO2DTO(commonentity.ContentTypeAudio))
	assert.Equal(t, "video", OpenAPIContentTypeDO2DTO(commonentity.ContentTypeVideo))
	assert.Equal(t, "multi_part", OpenAPIContentTypeDO2DTO(commonentity.ContentTypeMultipart))
	assert.Equal(t, "multi_part_variable", OpenAPIContentTypeDO2DTO(commonentity.ContentTypeMultipartVariable))
	assert.Equal(t, commonentity.ContentTypeText, OpenAPIContentTypeDTO2DO("text"))
	assert.Equal(t, commonentity.ContentTypeImage, OpenAPIContentTypeDTO2DO("image"))
	assert.Equal(t, commonentity.ContentTypeMultipart, OpenAPIContentTypeDTO2DO("multi_part"))
	assert.Equal(t, commonentity.ContentTypeMultipartVariable, OpenAPIContentTypeDTO2DO("multi_part_variable"))
}

func TestOpenAPIContentTypeDO2DTO_DefaultCase(t *testing.T) {
	// 46-59: default 分支，未知 ContentType 转小写首字母+其余
	t.Run("unknown_content_type", func(t *testing.T) {
		got := OpenAPIContentTypeDO2DTO(commonentity.ContentType("Foo"))
		assert.Equal(t, "foo", got)
	})
	t.Run("unknown_single_char", func(t *testing.T) {
		got := OpenAPIContentTypeDO2DTO(commonentity.ContentType("X"))
		assert.Equal(t, "x", got)
	})
	t.Run("empty_after_trim_returns_text", func(t *testing.T) {
		got := OpenAPIContentTypeDO2DTO(commonentity.ContentType("  "))
		assert.Equal(t, openapiCommon.ContentTypeText, got)
	})
	t.Run("empty_string_returns_text", func(t *testing.T) {
		got := OpenAPIContentTypeDO2DTO(commonentity.ContentType(""))
		assert.Equal(t, openapiCommon.ContentTypeText, got)
	})
}

func TestOpenAPIContentTypeDTO2DO_DefaultCase(t *testing.T) {
	// 70-94: default 分支，未知 string 转首字母大写+其余小写
	t.Run("unknown_string", func(t *testing.T) {
		got := OpenAPIContentTypeDTO2DO("unknown")
		assert.Equal(t, commonentity.ContentType("Unknown"), got)
	})
	t.Run("foo_to_Foo", func(t *testing.T) {
		got := OpenAPIContentTypeDTO2DO("foo")
		assert.Equal(t, commonentity.ContentType("Foo"), got)
	})
	t.Run("empty_after_trim_returns_text", func(t *testing.T) {
		got := OpenAPIContentTypeDTO2DO("  ")
		assert.Equal(t, commonentity.ContentTypeText, got)
	})
	t.Run("empty_string_returns_text", func(t *testing.T) {
		got := OpenAPIContentTypeDTO2DO("")
		assert.Equal(t, commonentity.ContentTypeText, got)
	})
}

func TestOpenAPIArgsSchemaDTO2DO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, OpenAPIArgsSchemaDTO2DO(nil))
	})

	t.Run("normal input", func(t *testing.T) {
		dto := &openapiCommon.ArgsSchema{
			Key:                 gptr.Of("k1"),
			SupportContentTypes: []string{"text"}, // OpenAPI IDL 小写
			JSONSchema:          gptr.Of("{}"),
		}
		do := OpenAPIArgsSchemaDTO2DO(dto)
		assert.NotNil(t, do)
		assert.Equal(t, "k1", *do.Key)
		assert.Equal(t, commonentity.ContentTypeText, do.SupportContentTypes[0]) // DB/entity 存首字母大写
	})
}

func TestOpenAPIMessageDO2DTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, OpenAPIMessageDO2DTO(nil))
	})

	t.Run("normal input", func(t *testing.T) {
		do := &commonentity.Message{
			Role: commonentity.RoleUser,
			Content: &commonentity.Content{
				Text: gptr.Of("hi"),
			},
		}
		dto := OpenAPIMessageDO2DTO(do)
		assert.NotNil(t, dto)
		assert.Equal(t, openapiCommon.RoleUser, *dto.Role)
		assert.Equal(t, "hi", *dto.Content.Text)
	})
}

func TestOpenAPIContentDO2DTO(t *testing.T) {
	t.Run("normal input with multipart", func(t *testing.T) {
		do := &commonentity.Content{
			ContentType: gptr.Of(commonentity.ContentTypeMultipart),
			MultiPart: []*commonentity.Content{
				{Text: gptr.Of("part1")},
			},
		}
		dto := OpenAPIContentDO2DTO(do)
		assert.NotNil(t, dto)
		assert.Equal(t, "multi_part", *dto.ContentType) // OpenAPI 小写
		assert.Len(t, dto.MultiPart, 1)
		assert.Equal(t, "part1", *dto.MultiPart[0].Text)
	})
}

func TestOpenAPIContentDTO2DO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, OpenAPIContentDTO2DO(nil))
	})

	t.Run("normal input with multipart", func(t *testing.T) {
		dto := &openapiCommon.Content{
			ContentType: gptr.Of("multi_part"), // OpenAPI IDL 小写
			MultiPart: []*openapiCommon.Content{
				{Text: gptr.Of("part1")},
			},
		}
		do := OpenAPIContentDTO2DO(dto)
		assert.NotNil(t, do)
		assert.Equal(t, commonentity.ContentTypeMultipart, *do.ContentType) // DB/entity 存首字母大写
		assert.Len(t, do.MultiPart, 1)
		assert.Equal(t, "part1", *do.MultiPart[0].Text)
	})
}

func TestOpenAPIRoleDTO2DO(t *testing.T) {
	assert.Equal(t, commonentity.RoleSystem, OpenAPIRoleDTO2DO(gptr.Of(openapiCommon.RoleSystem)))
	assert.Equal(t, commonentity.RoleUser, OpenAPIRoleDTO2DO(gptr.Of(openapiCommon.RoleUser)))
	assert.Equal(t, commonentity.RoleAssistant, OpenAPIRoleDTO2DO(gptr.Of(openapiCommon.RoleAssistant)))
	assert.Equal(t, commonentity.RoleUndefined, OpenAPIRoleDTO2DO(nil))
	assert.Equal(t, commonentity.RoleUndefined, OpenAPIRoleDTO2DO(gptr.Of(openapiCommon.Role("999"))))
}

func TestOpenAPIArgsSchemaDO2DTO(t *testing.T) {
	assert.Nil(t, OpenAPIArgsSchemaDO2DTO(nil))
	do := &commonentity.ArgsSchema{
		Key:                 gptr.Of("k"),
		SupportContentTypes: []commonentity.ContentType{commonentity.ContentTypeText},
	}
	dto := OpenAPIArgsSchemaDO2DTO(do)
	assert.Equal(t, "k", *dto.Key)
	assert.Equal(t, "text", dto.SupportContentTypes[0]) // OpenAPI 小写
}

func TestOpenAPIArgsSchemaDO2DTOs(t *testing.T) {
	assert.Nil(t, OpenAPIArgsSchemaDO2DTOs(nil))
	res := OpenAPIArgsSchemaDO2DTOs([]*commonentity.ArgsSchema{{Key: gptr.Of("k")}})
	assert.Len(t, res, 1)
}

func TestOpenAPIArgsSchemaDTO2DOs(t *testing.T) {
	assert.Nil(t, OpenAPIArgsSchemaDTO2DOs(nil))
	res := OpenAPIArgsSchemaDTO2DOs([]*openapiCommon.ArgsSchema{{Key: gptr.Of("k")}})
	assert.Len(t, res, 1)
}

func TestOpenAPIContentDTO2DOs(t *testing.T) {
	assert.Nil(t, OpenAPIContentDTO2DOs(nil))
	res := OpenAPIContentDTO2DOs(map[string]*openapiCommon.Content{"k": {Text: gptr.Of("v")}})
	assert.Len(t, res, 1)
	assert.Equal(t, "v", *res["k"].Text)
}

func TestOpenAPIMessageDO2DTOs(t *testing.T) {
	assert.Nil(t, OpenAPIMessageDO2DTOs(nil))
	res := OpenAPIMessageDO2DTOs([]*commonentity.Message{{Ext: map[string]string{"a": "b"}}})
	assert.Len(t, res, 1)
}

func TestOpenAPIMessageDTO2DO(t *testing.T) {
	assert.Nil(t, OpenAPIMessageDTO2DO(nil))
	dto := &openapiCommon.Message{Ext: map[string]string{"a": "b"}}
	do := OpenAPIMessageDTO2DO(dto)
	assert.Equal(t, "b", do.Ext["a"])
}

func TestOpenAPIMessageDTO2DOs(t *testing.T) {
	assert.Nil(t, OpenAPIMessageDTO2DOs(nil))
	res := OpenAPIMessageDTO2DOs([]*openapiCommon.Message{{Ext: map[string]string{"a": "b"}}})
	assert.Len(t, res, 1)
}

func TestOpenAPIRoleDO2DTO(t *testing.T) {
	assert.Equal(t, openapiCommon.RoleSystem, OpenAPIRoleDO2DTO(commonentity.RoleSystem))
	assert.Equal(t, openapiCommon.RoleUser, OpenAPIRoleDO2DTO(commonentity.RoleUser))
	assert.Equal(t, openapiCommon.RoleAssistant, OpenAPIRoleDO2DTO(commonentity.RoleAssistant))
	assert.Equal(t, openapiCommon.Role(""), OpenAPIRoleDO2DTO(commonentity.RoleUndefined))
}

func TestOpenAPIModelConfigDO2DTO(t *testing.T) {
	assert.Nil(t, OpenAPIModelConfigDO2DTO(nil))
	do := &commonentity.ModelConfig{ModelID: gptr.Of(int64(1)), ModelName: "m"}
	dto := OpenAPIModelConfigDO2DTO(do)
	assert.Equal(t, int64(1), *dto.ModelID)
	assert.Equal(t, "m", *dto.ModelName)
}

func TestOpenAPIModelConfigDTO2DO(t *testing.T) {
	assert.Nil(t, OpenAPIModelConfigDTO2DO(nil))
	dto := &openapiCommon.ModelConfig{ModelID: gptr.Of(int64(1)), ModelName: gptr.Of("m")}
	do := OpenAPIModelConfigDTO2DO(dto)
	assert.Equal(t, int64(1), *do.ModelID)
	assert.Equal(t, "m", do.ModelName)
}

func TestOpenAPIRuntimeParamDTO2DO(t *testing.T) {
	assert.Nil(t, OpenAPIRuntimeParamDTO2DO(nil))
	dto := &openapiCommon.RuntimeParam{JSONValue: gptr.Of("{}")}
	do := OpenAPIRuntimeParamDTO2DO(dto)
	assert.Equal(t, "{}", *do.JSONValue)
}

func TestOpenAPIOrderBysDTO2DO(t *testing.T) {
	assert.Nil(t, OpenAPIOrderBysDTO2DO(nil))
	dtos := []*openapiCommon.OrderBy{{Field: gptr.Of("f"), IsAsc: gptr.Of(true)}}
	res := OpenAPIOrderBysDTO2DO(dtos)
	assert.Len(t, res, 1)
	assert.Equal(t, "f", *res[0].Field)
	assert.True(t, *res[0].IsAsc)
}

func TestOpenAPIImageDO2DTO(t *testing.T) {
	// 102-106: 覆盖非 nil Image 转换
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, OpenAPIImageDO2DTO(nil))
	})
	t.Run("non_nil with all fields", func(t *testing.T) {
		do := &commonentity.Image{
			Name:     gptr.Of("img1"),
			URL:      gptr.Of("https://example.com/img.png"),
			ThumbURL: gptr.Of("https://example.com/thumb.png"),
		}
		dto := OpenAPIImageDO2DTO(do)
		assert.NotNil(t, dto)
		assert.Equal(t, "img1", *dto.Name)
		assert.Equal(t, "https://example.com/img.png", *dto.URL)
		assert.Equal(t, "https://example.com/thumb.png", *dto.ThumbURL)
	})
}

func TestOpenAPIImageDTO2DO(t *testing.T) {
	// 114-118: 覆盖非 nil Image DTO 转 DO
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, OpenAPIImageDTO2DO(nil))
	})
	t.Run("non_nil with all fields", func(t *testing.T) {
		dto := &openapiCommon.Image{
			Name:     gptr.Of("img1"),
			URL:      gptr.Of("https://example.com/img.png"),
			ThumbURL: gptr.Of("https://example.com/thumb.png"),
		}
		do := OpenAPIImageDTO2DO(dto)
		assert.NotNil(t, do)
		assert.Equal(t, "img1", *do.Name)
		assert.Equal(t, "https://example.com/img.png", *do.URL)
		assert.Equal(t, "https://example.com/thumb.png", *do.ThumbURL)
		assert.Nil(t, do.URI)
	})
}

func TestOpenAPIVideoDO2DTO(t *testing.T) {
	// 126-130: 覆盖非 nil Video 转换
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, OpenAPIVideoDO2DTO(nil))
	})
	t.Run("non_nil with all fields", func(t *testing.T) {
		do := &commonentity.Video{
			Name:     gptr.Of("vid1"),
			URL:      gptr.Of("https://example.com/vid.mp4"),
			URI:      gptr.Of("uri://vid"),
			ThumbURL: gptr.Of("https://example.com/thumb.png"),
		}
		dto := OpenAPIVideoDO2DTO(do)
		assert.NotNil(t, dto)
		assert.Equal(t, "vid1", *dto.Name)
		assert.Equal(t, "https://example.com/vid.mp4", *dto.URL)
		assert.Equal(t, "uri://vid", *dto.URI)
		assert.Equal(t, "https://example.com/thumb.png", *dto.ThumbURL)
	})
}

func TestOpenAPIVideoDTO2DO(t *testing.T) {
	// 138-142: 覆盖非 nil Video DTO 转 DO
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, OpenAPIVideoDTO2DO(nil))
	})
	t.Run("non_nil with all fields", func(t *testing.T) {
		dto := &openapiCommon.Video{
			Name:     gptr.Of("vid1"),
			URL:      gptr.Of("https://example.com/vid.mp4"),
			URI:      gptr.Of("uri://vid"),
			ThumbURL: gptr.Of("https://example.com/thumb.png"),
		}
		do := OpenAPIVideoDTO2DO(dto)
		assert.NotNil(t, do)
		assert.Equal(t, "vid1", *do.Name)
		assert.Equal(t, "https://example.com/vid.mp4", *do.URL)
		assert.Equal(t, "uri://vid", *do.URI)
		assert.Equal(t, "https://example.com/thumb.png", *do.ThumbURL)
	})
}

func TestOpenAPIAudioDO2DTO(t *testing.T) {
	// 150-154: 覆盖非 nil Audio 转换
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, OpenAPIAudioDO2DTO(nil))
	})
	t.Run("non_nil with all fields", func(t *testing.T) {
		do := &commonentity.Audio{
			Format: gptr.Of("mp3"),
			URL:    gptr.Of("https://example.com/audio.mp3"),
			Name:   gptr.Of("audio1"),
			URI:    gptr.Of("uri://audio"),
		}
		dto := OpenAPIAudioDO2DTO(do)
		assert.NotNil(t, dto)
		assert.Equal(t, "mp3", *dto.Format)
		assert.Equal(t, "https://example.com/audio.mp3", *dto.URL)
		assert.Equal(t, "audio1", *dto.Name)
		assert.Equal(t, "uri://audio", *dto.URI)
	})
}

func TestOpenAPIAudioDTO2DO(t *testing.T) {
	// 146-155: 覆盖非 nil Audio DTO 转 DO
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, OpenAPIAudioDTO2DO(nil))
	})
	t.Run("non_nil with all fields", func(t *testing.T) {
		dto := &openapiCommon.Audio{
			Format: gptr.Of("mp3"),
			URL:    gptr.Of("https://example.com/audio.mp3"),
			Name:   gptr.Of("audio1"),
			URI:    gptr.Of("uri://audio"),
		}
		do := OpenAPIAudioDTO2DO(dto)
		assert.NotNil(t, do)
		assert.Equal(t, "mp3", *do.Format)
		assert.Equal(t, "https://example.com/audio.mp3", *do.URL)
		assert.Equal(t, "audio1", *do.Name)
		assert.Equal(t, "uri://audio", *do.URI)
	})
}
