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

func TestOpenAPIArgsSchemaDTO2DO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, OpenAPIArgsSchemaDTO2DO(nil))
	})

	t.Run("normal input", func(t *testing.T) {
		dto := &openapiCommon.ArgsSchema{
			Key:                 gptr.Of("k1"),
			SupportContentTypes: []string{"Text"},
			JSONSchema:          gptr.Of("{}"),
		}
		do := OpenAPIArgsSchemaDTO2DO(dto)
		assert.NotNil(t, do)
		assert.Equal(t, "k1", *do.Key)
		assert.Equal(t, commonentity.ContentTypeText, do.SupportContentTypes[0])
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
		assert.Equal(t, "MultiPart", *dto.ContentType)
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
			ContentType: gptr.Of("MultiPart"),
			MultiPart: []*openapiCommon.Content{
				{Text: gptr.Of("part1")},
			},
		}
		do := OpenAPIContentDTO2DO(dto)
		assert.NotNil(t, do)
		assert.Equal(t, commonentity.ContentTypeMultipart, *do.ContentType)
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
	assert.Equal(t, "Text", dto.SupportContentTypes[0])
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
