// Copyright (c) 2025 Bytedance Ltd. and/or its affiliates
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/common"
)

// convertOpenAPIContentTypeToDO 将OpenAPI的ContentType转换为Domain Entity的ContentType
func ConvertOpenAPIContentTypeToDO(contentType *common.ContentType) entity.ContentType {
	if contentType == nil {
		return entity.ContentTypeText // 默认值
	}

	switch *contentType {
	case common.ContentTypeText:
		return entity.ContentTypeText
	case common.ContentTypeImage:
		return entity.ContentTypeImage
	case common.ContentTypeAudio:
		return entity.ContentTypeAudio
	case common.ContentTypeMultiPart:
		return entity.ContentTypeMultipart
	default:
		return entity.ContentTypeText // 默认使用Text类型
	}
}

// convertDOContentTypeToOpenAPI 将Domain Entity的ContentType转换为OpenAPI的ContentType
func ConvertDOContentTypeToOpenAPI(contentType entity.ContentType) *common.ContentType {
	if contentType == "" {
		return nil
	}

	switch contentType {
	case entity.ContentTypeText:
		ct := common.ContentTypeText
		return &ct
	case entity.ContentTypeImage:
		ct := common.ContentTypeImage
		return &ct
	case entity.ContentTypeAudio:
		ct := common.ContentTypeAudio
		return &ct
	case entity.ContentTypeMultipart, entity.ContentTypeMultipartVariable:
		ct := common.ContentTypeMultiPart
		return &ct
	default:
		// 默认使用text类型
		ct := common.ContentTypeText
		return &ct
	}
}
