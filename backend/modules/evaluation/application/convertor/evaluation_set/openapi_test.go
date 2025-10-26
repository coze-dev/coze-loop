// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluation_set

import (
	"testing"

	"github.com/stretchr/testify/assert"

	common "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/common"
	openapi_eval_set "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/eval_set"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func ptr[T any](v T) *T { return &v }

func TestConvertOpenAPIContentTypeToDO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *common.ContentType
		expected entity.ContentType
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: entity.ContentTypeText,
		},
		{
			name:     "text",
			input:    ptr[common.ContentType](common.ContentTypeText),
			expected: entity.ContentTypeText,
		},
		{
			name:     "image",
			input:    ptr[common.ContentType](common.ContentTypeImage),
			expected: entity.ContentTypeImage,
		},
		{
			name:     "audio",
			input:    ptr[common.ContentType](common.ContentTypeAudio),
			expected: entity.ContentTypeAudio,
		},
		{
			name:     "multi-part",
			input:    ptr[common.ContentType](common.ContentTypeMultiPart),
			expected: entity.ContentTypeMultipart,
		},
		{
			name:     "unknown",
			input:    ptr[common.ContentType](common.ContentType("unknown")),
			expected: entity.ContentTypeText,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, convertOpenAPIContentTypeToDO(tt.input))
		})
	}
}

func TestConvertDOContentTypeToOpenAPI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    entity.ContentType
		expected *common.ContentType
	}{
		{
			name:     "empty",
			input:    "",
			expected: nil,
		},
		{
			name:     "text",
			input:    entity.ContentTypeText,
			expected: ptr[common.ContentType](common.ContentTypeText),
		},
		{
			name:     "image",
			input:    entity.ContentTypeImage,
			expected: ptr[common.ContentType](common.ContentTypeImage),
		},
		{
			name:     "audio",
			input:    entity.ContentTypeAudio,
			expected: ptr[common.ContentType](common.ContentTypeAudio),
		},
		{
			name:     "multipart",
			input:    entity.ContentTypeMultipart,
			expected: ptr[common.ContentType](common.ContentTypeMultiPart),
		},
		{
			name:     "multipart variable",
			input:    entity.ContentTypeMultipartVariable,
			expected: ptr[common.ContentType](common.ContentTypeMultiPart),
		},
		{
			name:     "unknown",
			input:    entity.ContentType("unknown"),
			expected: ptr[common.ContentType](common.ContentTypeText),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, convertDOContentTypeToOpenAPI(tt.input))
		})
	}
}

func TestConvertDisplayFormatConversions(t *testing.T) {
	t.Parallel()

	t.Run("openapi to do", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, entity.FieldDisplayFormat_PlainText, convertOpenAPIDisplayFormatToDO(nil))

		cases := []struct {
			name     string
			input    *openapi_eval_set.FieldDisplayFormat
			expected entity.FieldDisplayFormat
		}{
			{"plain", ptr[openapi_eval_set.FieldDisplayFormat](openapi_eval_set.FieldDisplayFormatPlainText), entity.FieldDisplayFormat_PlainText},
			{"markdown", ptr[openapi_eval_set.FieldDisplayFormat](openapi_eval_set.FieldDisplayFormatMarkdown), entity.FieldDisplayFormat_Markdown},
			{"json", ptr[openapi_eval_set.FieldDisplayFormat](openapi_eval_set.FieldDisplayFormatJSON), entity.FieldDisplayFormat_JSON},
			{"yaml", ptr[openapi_eval_set.FieldDisplayFormat](openapi_eval_set.FieldDisplayFormateYAML), entity.FieldDisplayFormat_YAML},
			{"code", ptr[openapi_eval_set.FieldDisplayFormat](openapi_eval_set.FieldDisplayFormateCode), entity.FieldDisplayFormat_Code},
			{"unknown", ptr[openapi_eval_set.FieldDisplayFormat](openapi_eval_set.FieldDisplayFormat("unknown")), entity.FieldDisplayFormat_PlainText},
		}

		for _, tc := range cases {
			c := tc
			t.Run(c.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, c.expected, convertOpenAPIDisplayFormatToDO(c.input))
			})
		}
	})

	t.Run("do to openapi", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, convertDODisplayFormatToOpenAPI(99))

		cases := []struct {
			name     string
			input    entity.FieldDisplayFormat
			expected *openapi_eval_set.FieldDisplayFormat
		}{
			{"plain", entity.FieldDisplayFormat_PlainText, ptr[openapi_eval_set.FieldDisplayFormat](openapi_eval_set.FieldDisplayFormatPlainText)},
			{"markdown", entity.FieldDisplayFormat_Markdown, ptr[openapi_eval_set.FieldDisplayFormat](openapi_eval_set.FieldDisplayFormatMarkdown)},
			{"json", entity.FieldDisplayFormat_JSON, ptr[openapi_eval_set.FieldDisplayFormat](openapi_eval_set.FieldDisplayFormatJSON)},
			{"yaml", entity.FieldDisplayFormat_YAML, ptr[openapi_eval_set.FieldDisplayFormat](openapi_eval_set.FieldDisplayFormateYAML)},
			{"code", entity.FieldDisplayFormat_Code, ptr[openapi_eval_set.FieldDisplayFormat](openapi_eval_set.FieldDisplayFormateCode)},
		}

		for _, tc := range cases {
			c := tc
			t.Run(c.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, c.expected, convertDODisplayFormatToOpenAPI(c.input))
			})
		}
	})
}

func TestConvertDOStatusToOpenAPI(t *testing.T) {
	t.Parallel()

	assert.Equal(t, openapi_eval_set.EvaluationSetStatusActive, convertDOStatusToOpenAPI(entity.DatasetStatus_Available))
	assert.Equal(t, openapi_eval_set.EvaluationSetStatusArchived, convertDOStatusToOpenAPI(entity.DatasetStatus_Deleted))
	assert.Equal(t, openapi_eval_set.EvaluationSetStatusArchived, convertDOStatusToOpenAPI(entity.DatasetStatus_Expired))
	assert.Equal(t, openapi_eval_set.EvaluationSetStatusActive, convertDOStatusToOpenAPI(entity.DatasetStatus_Importing))
}

func TestOpenAPIFieldSchemaConversions(t *testing.T) {
	t.Parallel()

	description := "desc"
	textSchema := "schema"
	key := "key"
	isRequired := true

	dto := &openapi_eval_set.FieldSchema{
		Name:                 ptr("field"),
		Description:          &description,
		ContentType:          ptr[common.ContentType](common.ContentTypeAudio),
		DefaultDisplayFormat: ptr[openapi_eval_set.FieldDisplayFormat](openapi_eval_set.FieldDisplayFormatMarkdown),
		IsRequired:           &isRequired,
		TextSchema:           &textSchema,
		Key:                  &key,
	}

	do := OpenAPIFieldSchemaDTO2DO(dto)
	expectedDO := &entity.FieldSchema{
		Name:                 "field",
		Description:          description,
		ContentType:          entity.ContentTypeAudio,
		DefaultDisplayFormat: entity.FieldDisplayFormat_Markdown,
		IsRequired:           isRequired,
		TextSchema:           textSchema,
		Key:                  key,
	}
	assert.Equal(t, expectedDO, do)
	assert.Equal(t, []*entity.FieldSchema{expectedDO}, OpenAPIFieldSchemaDTO2DOs([]*openapi_eval_set.FieldSchema{dto}))
	assert.Nil(t, OpenAPIFieldSchemaDTO2DO(nil))
	assert.Nil(t, OpenAPIFieldSchemaDTO2DOs(nil))

	assert.Nil(t, OpenAPIEvaluationSetSchemaDTO2DO(nil))
	schemaDO := OpenAPIEvaluationSetSchemaDTO2DO(&openapi_eval_set.EvaluationSetSchema{FieldSchemas: []*openapi_eval_set.FieldSchema{dto}})
	assert.Equal(t, &entity.EvaluationSetSchema{FieldSchemas: []*entity.FieldSchema{expectedDO}}, schemaDO)

	assert.Nil(t, OpenAPIEvaluationSetSchemaDO2DTO(nil))
	backDTO := OpenAPIEvaluationSetSchemaDO2DTO(&entity.EvaluationSetSchema{FieldSchemas: []*entity.FieldSchema{expectedDO}})
	assert.Equal(t, []*openapi_eval_set.FieldSchema{OpenAPIFieldSchemaDO2DTO(expectedDO)}, backDTO.FieldSchemas)

	assert.Nil(t, OpenAPIFieldSchemaDO2DTOs(nil))
	assert.Equal(t, []*openapi_eval_set.FieldSchema{OpenAPIFieldSchemaDO2DTO(expectedDO)}, OpenAPIFieldSchemaDO2DTOs([]*entity.FieldSchema{expectedDO}))
}

func TestOrderByConversions(t *testing.T) {
	t.Parallel()

	field := "created_at"
	isAsc := true
	dto := &common.OrderBy{
		Field: &field,
		IsAsc: &isAsc,
	}
	expected := &entity.OrderBy{
		Field: &field,
		IsAsc: &isAsc,
	}
	assert.Equal(t, expected, OrderByDTO2DO(dto))
	assert.Equal(t, []*entity.OrderBy{expected}, OrderByDTO2DOs([]*common.OrderBy{dto}))
	assert.Nil(t, OrderByDTO2DO(nil))
	assert.Nil(t, OrderByDTO2DOs(nil))
}

func TestEvaluationSetConversions(t *testing.T) {
	t.Parallel()

	createdAt := int64(123)
	updatedAt := int64(456)
	innerCreatedAt := int64(111)
	innerUpdatedAt := int64(222)
	creator := "creator"
	updater := "updater"
	innerCreator := "inner_creator"
	innerUpdater := "inner_updater"

	versionDO := &entity.EvaluationSetVersion{
		ID:          10,
		Version:     "v1",
		Description: "version desc",
		EvaluationSetSchema: &entity.EvaluationSetSchema{FieldSchemas: []*entity.FieldSchema{
			{
				Name:                 "field",
				Description:          "desc",
				ContentType:          entity.ContentTypeImage,
				DefaultDisplayFormat: entity.FieldDisplayFormat_Code,
				IsRequired:           true,
				TextSchema:           "schema",
				Key:                  "key",
			},
		}},
		ItemCount: 5,
		BaseInfo: &entity.BaseInfo{
			CreatedAt: &innerCreatedAt,
			UpdatedAt: &innerUpdatedAt,
			CreatedBy: &entity.UserInfo{Name: &innerCreator},
			UpdatedBy: &entity.UserInfo{Name: &innerUpdater},
		},
	}

	do := &entity.EvaluationSet{
		ID:                   1,
		Name:                 "evaluation",
		Description:          "desc",
		Status:               entity.DatasetStatus_Deleted,
		ItemCount:            3,
		ChangeUncommitted:    true,
		LatestVersion:        "latest",
		EvaluationSetVersion: versionDO,
		BaseInfo: &entity.BaseInfo{
			CreatedAt: &createdAt,
			UpdatedAt: &updatedAt,
			CreatedBy: &entity.UserInfo{Name: &creator},
			UpdatedBy: &entity.UserInfo{Name: &updater},
		},
	}

	result := OpenAPIEvaluationSetDO2DTO(do)
	expected := &openapi_eval_set.EvaluationSet{
		ID:                  ptr[int64](1),
		Name:                ptr("evaluation"),
		Description:         ptr("desc"),
		Status:              ptr[openapi_eval_set.EvaluationSetStatus](openapi_eval_set.EvaluationSetStatusArchived),
		ItemCount:           ptr[int64](3),
		LatestVersion:       ptr("latest"),
		IsChangeUncommitted: ptr(true),
		CurrentVersion: &openapi_eval_set.EvaluationSetVersion{
			ID:          ptr[int64](10),
			Version:     ptr("v1"),
			Description: ptr("version desc"),
			EvaluationSetSchema: &openapi_eval_set.EvaluationSetSchema{FieldSchemas: []*openapi_eval_set.FieldSchema{
				{
					Name:                 ptr("field"),
					Description:          ptr("desc"),
					ContentType:          ptr[common.ContentType](common.ContentTypeImage),
					DefaultDisplayFormat: ptr[openapi_eval_set.FieldDisplayFormat](openapi_eval_set.FieldDisplayFormateCode),
					IsRequired:           ptr(true),
					TextSchema:           ptr("schema"),
					Key:                  ptr("key"),
				},
			}},
			ItemCount: ptr[int64](5),
			BaseInfo: &common.BaseInfo{
				CreatedBy: &common.UserInfo{Name: &innerCreator},
				UpdatedBy: &common.UserInfo{Name: &innerUpdater},
				CreatedAt: &innerCreatedAt,
				UpdatedAt: &innerUpdatedAt,
			},
		},
		BaseInfo: &common.BaseInfo{
			CreatedBy: &common.UserInfo{Name: &creator},
			UpdatedBy: &common.UserInfo{Name: &updater},
			CreatedAt: &createdAt,
			UpdatedAt: &updatedAt,
		},
	}
	assert.Equal(t, expected, result)
	assert.Equal(t, []*openapi_eval_set.EvaluationSet{expected}, OpenAPIEvaluationSetDO2DTOs([]*entity.EvaluationSet{do}))
	assert.Nil(t, OpenAPIEvaluationSetDO2DTO(nil))
	assert.Nil(t, OpenAPIEvaluationSetDO2DTOs(nil))
	assert.Equal(t, expected.CurrentVersion, OpenAPIEvaluationSetVersionDO2DTO(versionDO))
	assert.Equal(t, []*openapi_eval_set.EvaluationSetVersion{expected.CurrentVersion}, OpenAPIEvaluationSetVersionDO2DTOs([]*entity.EvaluationSetVersion{versionDO}))
}

func TestBaseInfoAndUserInfoConversions(t *testing.T) {
	t.Parallel()

	name := "user"
	email := "user@example.com"
	createdAt := int64(1)
	updatedAt := int64(2)

	user := &entity.UserInfo{
		Name:  &name,
		Email: &email,
	}

	base := &entity.BaseInfo{
		CreatedBy: user,
		UpdatedBy: user,
		CreatedAt: &createdAt,
		UpdatedAt: &updatedAt,
	}

	convertedUser := ConvertUserInfoDO2DTO(user)
	assert.Equal(t, &common.UserInfo{Name: &name, Email: &email}, convertedUser)
	assert.Nil(t, ConvertUserInfoDO2DTO(nil))

	convertedBase := ConvertBaseInfoDO2DTO(base)
	assert.Equal(t, &common.BaseInfo{
		CreatedBy: convertedUser,
		UpdatedBy: convertedUser,
		CreatedAt: &createdAt,
		UpdatedAt: &updatedAt,
	}, convertedBase)
	assert.Nil(t, ConvertBaseInfoDO2DTO(nil))

	assert.Equal(t, convertedUser, OpenAPIUserInfoDO2DTO(user))
	assert.Nil(t, OpenAPIUserInfoDO2DTO(nil))
}

func TestOpenAPIItemConversions(t *testing.T) {
	t.Parallel()

	imageName := "image"
	imageURL := "url"
	thumbURL := "thumb"
	text := "text"
	multipartContent := &common.Content{
		ContentType: ptr[common.ContentType](common.ContentTypeText),
		Text:        ptr("part"),
	}

	contentDTO := &common.Content{
		ContentType: ptr[common.ContentType](common.ContentTypeMultiPart),
		Text:        &text,
		Image: &common.Image{
			Name:     &imageName,
			URL:      &imageURL,
			ThumbURL: &thumbURL,
		},
		MultiPart: []*common.Content{multipartContent},
	}

	turnDTO := &openapi_eval_set.Turn{
		ID: ptr[int64](1),
		FieldDatas: []*openapi_eval_set.FieldData{
			{
				Name:    ptr("field"),
				Content: contentDTO,
			},
		},
	}

	itemDTO := &openapi_eval_set.EvaluationSetItem{
		ID:      ptr[int64](2),
		ItemKey: ptr("key"),
		Turns:   []*openapi_eval_set.Turn{turnDTO},
	}

	do := OpenAPIItemDTO2DO(itemDTO)
	expectedContent := &entity.Content{
		ContentType: ptr(entity.ContentTypeMultipart),
		Text:        &text,
		Image: &entity.Image{
			Name:     &imageName,
			URL:      &imageURL,
			ThumbURL: &thumbURL,
		},
		MultiPart: []*entity.Content{
			{
				ContentType: ptr(entity.ContentTypeText),
				Text:        ptr("part"),
			},
		},
	}

	expectedDO := &entity.EvaluationSetItem{
		ItemID:  2,
		ItemKey: "key",
		Turns: []*entity.Turn{
			{
				ID: 1,
				FieldDataList: []*entity.FieldData{
					{
						Name:    "field",
						Content: expectedContent,
					},
				},
			},
		},
	}
	assert.Equal(t, expectedDO, do)
	assert.Nil(t, OpenAPIItemDTO2DO(nil))
	assert.Nil(t, OpenAPIItemDTO2DOs(nil))
	assert.Equal(t, []*entity.EvaluationSetItem{expectedDO}, OpenAPIItemDTO2DOs([]*openapi_eval_set.EvaluationSetItem{itemDTO}))

	assert.Equal(t, expectedDO.Turns[0], OpenAPITurnDTO2DO(turnDTO))
	assert.Nil(t, OpenAPITurnDTO2DO(nil))
	assert.Equal(t, []*entity.Turn{expectedDO.Turns[0]}, OpenAPITurnDTO2DOs([]*openapi_eval_set.Turn{turnDTO}))
	assert.Nil(t, OpenAPITurnDTO2DOs(nil))

	assert.Equal(t, expectedDO.Turns[0].FieldDataList[0], OpenAPIFieldDataDTO2DO(turnDTO.FieldDatas[0]))
	assert.Nil(t, OpenAPIFieldDataDTO2DO(nil))
	assert.Equal(t, []*entity.FieldData{expectedDO.Turns[0].FieldDataList[0]}, OpenAPIFieldDataDTO2DOs([]*openapi_eval_set.FieldData{turnDTO.FieldDatas[0]}))
	assert.Nil(t, OpenAPIFieldDataDTO2DOs(nil))

	assert.Equal(t, expectedContent, OpenAPIContentDTO2DO(contentDTO))
	assert.Nil(t, OpenAPIContentDTO2DO(nil))

	assert.Equal(t, expectedContent.Image, ConvertImageDTO2DO(contentDTO.Image))
	assert.Nil(t, ConvertImageDTO2DO(nil))
}

func TestOpenAPIItemDOToDTOConversions(t *testing.T) {
	t.Parallel()

	audioFormat := "mp3"
	audioURL := "audio"
	imageName := "image"
	imageURL := "url"
	thumbURL := "thumb"
	text := "body"

	doContent := &entity.Content{
		ContentType: ptr(entity.ContentTypeAudio),
		Text:        &text,
		Image: &entity.Image{
			Name:     &imageName,
			URL:      &imageURL,
			ThumbURL: &thumbURL,
		},
		MultiPart: []*entity.Content{
			{
				ContentType: ptr(entity.ContentTypeText),
				Text:        ptr("nested"),
			},
		},
		Audio: &entity.Audio{
			Format: &audioFormat,
			URL:    &audioURL,
		},
	}

	do := &entity.EvaluationSetItem{
		ID:      1,
		ItemKey: "key",
		Turns: []*entity.Turn{
			{
				ID: 2,
				FieldDataList: []*entity.FieldData{
					{
						Name:    "field",
						Content: doContent,
					},
				},
			},
		},
		BaseInfo: &entity.BaseInfo{},
	}

	result := OpenAPIItemDO2DTO(do)
	expected := &openapi_eval_set.EvaluationSetItem{
		ID:      ptr[int64](1),
		ItemKey: ptr("key"),
		Turns: []*openapi_eval_set.Turn{
			{
				ID: ptr[int64](2),
				FieldDatas: []*openapi_eval_set.FieldData{
					{
						Name: ptr("field"),
						Content: &common.Content{
							ContentType: ptr[common.ContentType](common.ContentTypeAudio),
							Text:        &text,
							Image: &common.Image{
								Name:     &imageName,
								URL:      &imageURL,
								ThumbURL: &thumbURL,
							},
							MultiPart: []*common.Content{
								{
									ContentType: ptr[common.ContentType](common.ContentTypeText),
									Text:        ptr("nested"),
								},
							},
						},
					},
				},
			},
		},
		BaseInfo: &common.BaseInfo{},
	}
	assert.Equal(t, expected, result)
	assert.Nil(t, OpenAPIItemDO2DTO(nil))
	assert.Nil(t, OpenAPIItemDO2DTOs(nil))
	assert.Equal(t, []*openapi_eval_set.EvaluationSetItem{expected}, OpenAPIItemDO2DTOs([]*entity.EvaluationSetItem{do}))

	assert.Equal(t, expected.Turns[0], OpenAPITurnDO2DTO(do.Turns[0]))
	assert.Nil(t, OpenAPITurnDO2DTO(nil))
	assert.Equal(t, []*openapi_eval_set.Turn{expected.Turns[0]}, OpenAPITurnDO2DTOs([]*entity.Turn{do.Turns[0]}))
	assert.Nil(t, OpenAPITurnDO2DTOs(nil))

	assert.Equal(t, expected.Turns[0].FieldDatas[0], OpenAPIFieldDataDO2DTO(do.Turns[0].FieldDataList[0]))
	assert.Nil(t, OpenAPIFieldDataDO2DTO(nil))
	assert.Equal(t, []*openapi_eval_set.FieldData{expected.Turns[0].FieldDatas[0]}, OpenAPIFieldDataDO2DTOs([]*entity.FieldData{do.Turns[0].FieldDataList[0]}))
	assert.Nil(t, OpenAPIFieldDataDO2DTOs(nil))

	assert.Equal(t, expected.Turns[0].FieldDatas[0].Content, OpenAPIContentDO2DTO(doContent))
	assert.Nil(t, OpenAPIContentDO2DTO(nil))

	assert.Equal(t, expected.Turns[0].FieldDatas[0].Content.Image, ConvertImageDO2DTO(doContent.Image))
	assert.Nil(t, ConvertImageDO2DTO(nil))

	assert.Equal(t, &common.Audio{Format: &audioFormat, URL: &audioURL}, ConvertAudioDO2DTO(doContent.Audio))
	assert.Nil(t, ConvertAudioDO2DTO(nil))
}

func TestItemErrorConversions(t *testing.T) {
	t.Parallel()

	errorType := entity.ItemErrorType_InternalError
	summary := "error"
	count := int32(3)
	message := "detail"
	index := int32(1)
	start := int32(2)
	end := int32(3)

	detail := &entity.ItemErrorDetail{
		Message:    &message,
		Index:      &index,
		StartIndex: &start,
		EndIndex:   &end,
	}

	group := &entity.ItemErrorGroup{
		Type:       &errorType,
		Summary:    &summary,
		ErrorCount: &count,
		Details:    []*entity.ItemErrorDetail{detail},
	}

	converted := OpenAPIItemErrorGroupDO2DTO(group)
	expected := &openapi_eval_set.ItemErrorGroup{
		ErrorCode:    ptr[int32](int32(errorType)),
		ErrorMessage: &summary,
		ErrorCount:   &count,
		Details: []*openapi_eval_set.ItemErrorDetail{
			{
				Message:    &message,
				Index:      &index,
				StartIndex: &start,
				EndIndex:   &end,
			},
		},
	}
	assert.Equal(t, expected, converted)
	assert.Nil(t, OpenAPIItemErrorGroupDO2DTO(nil))
	assert.Equal(t, []*openapi_eval_set.ItemErrorGroup{expected}, OpenAPIItemErrorGroupDO2DTOs([]*entity.ItemErrorGroup{group}))
	assert.Nil(t, OpenAPIItemErrorGroupDO2DTOs(nil))

	assert.Equal(t, expected.Details[0], OpenAPIItemErrorDetailDO2DTO(detail))
	assert.Nil(t, OpenAPIItemErrorDetailDO2DTO(nil))
	assert.Equal(t, []*openapi_eval_set.ItemErrorDetail{expected.Details[0]}, OpenAPIItemErrorDetailDO2DTOs([]*entity.ItemErrorDetail{detail}))
	assert.Nil(t, OpenAPIItemErrorDetailDO2DTOs(nil))
}

func TestDatasetItemOutputConversions(t *testing.T) {
	t.Parallel()

	index := int32(1)
	key := "key"
	id := int64(2)
	isNew := true

	do := &entity.DatasetItemOutput{
		ItemIndex: &index,
		ItemKey:   &key,
		ItemID:    &id,
		IsNewItem: &isNew,
	}

	converted := OpenAPIDatasetItemOutputDO2DTO(do)
	expected := &openapi_eval_set.DatasetItemOutput{
		ItemIndex: &index,
		ItemKey:   &key,
		ItemID:    &id,
		IsNewItem: &isNew,
	}
	assert.Equal(t, expected, converted)
	assert.Nil(t, OpenAPIDatasetItemOutputDO2DTO(nil))
	assert.Equal(t, []*openapi_eval_set.DatasetItemOutput{expected}, OpenAPIDatasetItemOutputDO2DTOs([]*entity.DatasetItemOutput{do}))
	assert.Nil(t, OpenAPIDatasetItemOutputDO2DTOs(nil))
}
