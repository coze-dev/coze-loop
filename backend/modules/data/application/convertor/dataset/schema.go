// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package dataset

import (
	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/entity"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/errno"
)

func FieldSchemaDO2DTO(s *entity.FieldSchema) (*dataset.FieldSchema, error) {
	dto := &dataset.FieldSchema{
		Key:           gptr.Of(s.Key),
		Name:          gptr.Of(s.Name),
		Description:   gptr.Of(s.Description),
		ContentType:   gptr.Of(ContentTypeDO2DTO(s.ContentType)),
		DefaultFormat: gptr.Of(FieldDisplayFormatDO2DTO(s.DefaultFormat)),
		SchemaKey:     gptr.Of(SchemaKeyDO2DTO(s.SchemaKey)),
		Status:        gptr.Of(FieldStatusDO2DTO(s.Status)),
		Hidden:        gptr.Of(s.Hidden),
	}
	if s.TextSchema != nil {
		dto.TextSchema = gptr.Of(string(s.TextSchema.Raw))
	}
	if s.MultiModelSpec != nil {
		dto.MultiModelSpec = MultiModalSpecDO2DTO(s.MultiModelSpec)
	}
	return dto, nil
}

func MultiModalSpecDO2DTO(sp *entity.MultiModalSpec) *dataset.MultiModalSpec {
	return &dataset.MultiModalSpec{
		MaxFileCount:     gptr.Of(sp.MaxFileCount),
		MaxFileSize:      gptr.Of(sp.MaxFileSize),
		SupportedFormats: sp.SupportedFormats,
	}
}

func ContentTypeDO2DTO(ct entity.ContentType) dataset.ContentType {
	switch ct {
	case entity.ContentTypeText:
		return dataset.ContentType_Text
	case entity.ContentTypeImage:
		return dataset.ContentType_Image
	case entity.ContentTypeAudio:
		return dataset.ContentType_Audio
	case entity.ContentTypeVideo:
		return dataset.ContentType_Video
	case entity.ContentTypeMultiPart:
		return dataset.ContentType_MultiPart
	default:
		return dataset.FieldData_ContentType_DEFAULT
	}
}

func FieldDisplayFormatDO2DTO(df entity.FieldDisplayFormat) dataset.FieldDisplayFormat {
	switch df {
	case entity.FieldDisplayFormatPlainText:
		return dataset.FieldDisplayFormat_PlainText
	case entity.FieldDisplayFormatMarkdown:
		return dataset.FieldDisplayFormat_Markdown
	case entity.FieldDisplayFormatJSON:
		return dataset.FieldDisplayFormat_JSON
	case entity.FieldDisplayFormatYAML:
		return dataset.FieldDisplayFormat_YAML
	case entity.FieldDisplayFormatCode:
		return dataset.FieldDisplayFormat_Code
	default:
		return dataset.FieldData_Format_DEFAULT
	}
}

func SchemaKeyDO2DTO(sk entity.SchemaKey) dataset.SchemaKey {
	switch sk {
	case entity.SchemaKeyString:
		return dataset.SchemaKey_String
	case entity.SchemaKeyInteger:
		return dataset.SchemaKey_Integer
	case entity.SchemaKeyFloat:
		return dataset.SchemaKey_Float
	case entity.SchemaKeyBool:
		return dataset.SchemaKey_Bool
	case entity.SchemaKeyMessage:
		return dataset.SchemaKey_Message
	default:
		return dataset.FieldSchema_SchemaKey_DEFAULT
	}
}

func FieldStatusDO2DTO(fs entity.FieldStatus) dataset.FieldStatus {
	switch fs {
	case entity.FieldStatusAvailable:
		return dataset.FieldStatus_Available
	case entity.FieldStatusDeleted:
		return dataset.FieldStatus_Deleted
	default:
		return dataset.FieldStatus_Available
	}
}

func FieldSchemaDTO2DO(s *dataset.FieldSchema) (t *entity.FieldSchema, err error) {
	if s == nil {
		return nil, nil
	}

	t = &entity.FieldSchema{
		Key:            s.GetKey(),
		Name:           s.GetName(),
		Description:    s.GetDescription(),
		ContentType:    ContentTypeDTO2DO(s.GetContentType()),
		DefaultFormat:  FieldDisplayFormatDTO2DO(s.GetDefaultFormat()),
		SchemaKey:      SchemaKeyDTO2DO(s.GetSchemaKey()),
		MultiModelSpec: MultiModalSpecDTO2DO(s.GetMultiModelSpec()),
		Status:         FieldStatusDTO2DO(s.GetStatus()),
		Hidden:         s.GetHidden(),
	}
	if s.GetContentType() == dataset.ContentType_Text && len(s.GetTextSchema()) > 0 {
		js, err := entity.NewJSONSchema(s.GetTextSchema())
		if err != nil {
			return nil, errno.JSONErr(err, "parse text json schema failed")
		}
		t.TextSchema = js
	}
	return t, nil
}

func ContentTypeDTO2DO(s dataset.ContentType) entity.ContentType {
	switch s {
	case dataset.ContentType_Text:
		return entity.ContentTypeText
	case dataset.ContentType_Image:
		return entity.ContentTypeImage
	case dataset.ContentType_Audio:
		return entity.ContentTypeAudio
	case dataset.ContentType_Video:
		return entity.ContentTypeVideo
	case dataset.ContentType_MultiPart:
		return entity.ContentTypeMultiPart
	}
	return entity.ContentTypeUnknown
}

func FieldDisplayFormatDTO2DO(s dataset.FieldDisplayFormat) entity.FieldDisplayFormat {
	switch s {
	case dataset.FieldDisplayFormat_PlainText:
		return entity.FieldDisplayFormatPlainText
	case dataset.FieldDisplayFormat_Markdown:
		return entity.FieldDisplayFormatMarkdown
	case dataset.FieldDisplayFormat_JSON:
		return entity.FieldDisplayFormatJSON
	case dataset.FieldDisplayFormat_YAML:
		return entity.FieldDisplayFormatYAML
	case dataset.FieldDisplayFormat_Code:
		return entity.FieldDisplayFormatCode
	}
	return entity.FieldDisplayFormatUnknown
}

func SchemaKeyDTO2DO(schemaKey dataset.SchemaKey) entity.SchemaKey {
	switch schemaKey {
	case dataset.SchemaKey_String:
		return entity.SchemaKeyString
	case dataset.SchemaKey_Integer:
		return entity.SchemaKeyInteger
	case dataset.SchemaKey_Float:
		return entity.SchemaKeyFloat
	case dataset.SchemaKey_Bool:
		return entity.SchemaKeyBool
	case dataset.SchemaKey_Message:
		return entity.SchemaKeyMessage
	default:
		return entity.SchemaKeyUnknown
	}
}

func MultiModalSpecDTO2DO(s *dataset.MultiModalSpec) *entity.MultiModalSpec {
	if s == nil {
		return nil
	}
	return &entity.MultiModalSpec{
		MaxFileCount:     s.GetMaxFileCount(),
		MaxFileSize:      s.GetMaxFileSize(),
		SupportedFormats: s.GetSupportedFormats(),
	}
}

func FieldStatusDTO2DO(status dataset.FieldStatus) entity.FieldStatus {
	switch status {
	case dataset.FieldStatus_Available:
		return entity.FieldStatusAvailable
	case dataset.FieldStatus_Deleted:
		return entity.FieldStatusDeleted
	default:
		return entity.FieldStatusUnknown
	}
}
