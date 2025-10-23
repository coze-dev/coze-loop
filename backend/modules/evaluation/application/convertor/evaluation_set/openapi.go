// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluation_set

import (
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/common"
	openapi_eval_set "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/eval_set"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// convertOpenAPIContentTypeToDO 将OpenAPI的ContentType转换为Domain Entity的ContentType
func convertOpenAPIContentTypeToDO(contentType *common.ContentType) entity.ContentType {
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
func convertDOContentTypeToOpenAPI(contentType entity.ContentType) *common.ContentType {
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

// convertOpenAPIDisplayFormatToDO 将OpenAPI的DefaultDisplayFormat转换为Domain Entity的DefaultDisplayFormat
func convertOpenAPIDisplayFormatToDO(format *openapi_eval_set.FieldDisplayFormat) entity.FieldDisplayFormat {
	if format == nil {
		return entity.FieldDisplayFormat_PlainText // 默认值
	}

	switch *format {
	case openapi_eval_set.FieldDisplayFormatPlainText:
		return entity.FieldDisplayFormat_PlainText
	case openapi_eval_set.FieldDisplayFormatMarkdown:
		return entity.FieldDisplayFormat_Markdown
	case openapi_eval_set.FieldDisplayFormatJSON:
		return entity.FieldDisplayFormat_JSON
	case openapi_eval_set.FieldDisplayFormateYAML:
		return entity.FieldDisplayFormat_YAML
	case openapi_eval_set.FieldDisplayFormateCode:
		return entity.FieldDisplayFormat_Code
	default:
		return entity.FieldDisplayFormat_PlainText
	}
}

// convertDODisplayFormatToOpenAPI 将Domain Entity的DefaultDisplayFormat转换为OpenAPI的DefaultDisplayFormat
func convertDODisplayFormatToOpenAPI(format entity.FieldDisplayFormat) *openapi_eval_set.FieldDisplayFormat {
	var displayFormat *openapi_eval_set.FieldDisplayFormat

	switch format {
	case entity.FieldDisplayFormat_PlainText:
		f := openapi_eval_set.FieldDisplayFormatPlainText
		displayFormat = &f
	case entity.FieldDisplayFormat_Markdown:
		f := openapi_eval_set.FieldDisplayFormatMarkdown
		displayFormat = &f
	case entity.FieldDisplayFormat_JSON:
		f := openapi_eval_set.FieldDisplayFormatJSON
		displayFormat = &f
	case entity.FieldDisplayFormat_YAML:
		f := openapi_eval_set.FieldDisplayFormateYAML
		displayFormat = &f
	case entity.FieldDisplayFormat_Code:
		f := openapi_eval_set.FieldDisplayFormateCode
		displayFormat = &f
	}

	return displayFormat
}

// convertDOStatusToOpenAPI 将Domain Entity的DatasetStatus转换为OpenAPI的EvaluationSetStatus
func convertDOStatusToOpenAPI(status entity.DatasetStatus) openapi_eval_set.EvaluationSetStatus {
	switch status {
	case entity.DatasetStatus_Available:
		return openapi_eval_set.EvaluationSetStatusActive
	case entity.DatasetStatus_Deleted, entity.DatasetStatus_Expired:
		return openapi_eval_set.EvaluationSetStatusArchived
	default:
		// 默认使用active状态
		return openapi_eval_set.EvaluationSetStatusActive
	}
}

// convertOpenAPIStatusToDO 将OpenAPI的EvaluationSetStatus转换为Domain Entity的DatasetStatus
func convertOpenAPIStatusToDO(status openapi_eval_set.EvaluationSetStatus) entity.DatasetStatus {
	switch status {
	case openapi_eval_set.EvaluationSetStatusActive:
		return entity.DatasetStatus_Available
	case openapi_eval_set.EvaluationSetStatusArchived:
		return entity.DatasetStatus_Deleted
	default:
		// 默认使用Available状态
		return entity.DatasetStatus_Available
	}
}

// OpenAPI Schema 转换
func OpenAPIEvaluationSetSchemaDTO2DO(dto *openapi_eval_set.EvaluationSetSchema) *entity.EvaluationSetSchema {
	if dto == nil {
		return nil
	}
	return &entity.EvaluationSetSchema{
		FieldSchemas: OpenAPIFieldSchemaDTO2DOs(dto.FieldSchemas),
	}
}

func OpenAPIFieldSchemaDTO2DOs(dtos []*openapi_eval_set.FieldSchema) []*entity.FieldSchema {
	if dtos == nil {
		return nil
	}
	result := make([]*entity.FieldSchema, 0)
	for _, dto := range dtos {
		result = append(result, OpenAPIFieldSchemaDTO2DO(dto))
	}
	return result
}

func OpenAPIFieldSchemaDTO2DO(dto *openapi_eval_set.FieldSchema) *entity.FieldSchema {
	if dto == nil {
		return nil
	}

	var description string
	if dto.Description != nil {
		description = *dto.Description
	}

	var textSchema string
	if dto.TextSchema != nil {
		textSchema = *dto.TextSchema
	}

	contentType := convertOpenAPIContentTypeToDO(dto.ContentType)

	displayFormat := convertOpenAPIDisplayFormatToDO(dto.DefaultDisplayFormat)

	return &entity.FieldSchema{
		Name:                 gptr.Indirect(dto.Name),
		Description:          description,
		ContentType:          contentType,
		DefaultDisplayFormat: displayFormat,
		IsRequired:           gptr.Indirect(dto.IsRequired),
		TextSchema:           textSchema,
	}
}

// OpenAPI OrderBy 转换
func OrderByDTO2DOs(dtos []*common.OrderBy) []*entity.OrderBy {
	if dtos == nil {
		return nil
	}
	result := make([]*entity.OrderBy, 0)
	for _, dto := range dtos {
		result = append(result, OrderByDTO2DO(dto))
	}
	return result
}

func OrderByDTO2DO(dto *common.OrderBy) *entity.OrderBy {
	if dto == nil {
		return nil
	}

	return &entity.OrderBy{
		Field: dto.Field,
		IsAsc: dto.IsAsc,
	}
}

// 内部DTO转OpenAPI DTO
func OpenAPIEvaluationSetDO2DTO(do *entity.EvaluationSet) *openapi_eval_set.EvaluationSet {
	if do == nil {
		return nil
	}

	return &openapi_eval_set.EvaluationSet{
		ID:                  gptr.Of(do.ID),
		Name:                gptr.Of(do.Name),
		Description:         gptr.Of(do.Description),
		Status:              gptr.Of(convertDOStatusToOpenAPI(do.Status)),
		ItemCount:           gptr.Of(do.ItemCount),
		LatestVersion:       gptr.Of(do.LatestVersion),
		IsChangeUncommitted: gptr.Of(do.ChangeUncommitted),
		CurrentVersion:      OpenAPIEvaluationSetVersionDO2DTO(do.EvaluationSetVersion),
		BaseInfo:            ConvertBaseInfoDO2DTO(do.BaseInfo),
	}
}

func OpenAPIEvaluationSetDO2DTOs(dos []*entity.EvaluationSet) []*openapi_eval_set.EvaluationSet {
	if dos == nil {
		return nil
	}
	result := make([]*openapi_eval_set.EvaluationSet, 0)
	for _, do := range dos {
		result = append(result, OpenAPIEvaluationSetDO2DTO(do))
	}
	return result
}

func OpenAPIEvaluationSetVersionDO2DTOs(dos []*entity.EvaluationSetVersion) []*openapi_eval_set.EvaluationSetVersion {
	if dos == nil {
		return nil
	}
	result := make([]*openapi_eval_set.EvaluationSetVersion, 0)
	for _, do := range dos {
		result = append(result, OpenAPIEvaluationSetVersionDO2DTO(do))
	}
	return result
}

func OpenAPIEvaluationSetVersionDO2DTO(do *entity.EvaluationSetVersion) *openapi_eval_set.EvaluationSetVersion {
	if do == nil {
		return nil
	}
	return &openapi_eval_set.EvaluationSetVersion{
		ID:                  gptr.Of(do.ID),
		Version:             gptr.Of(do.Version),
		Description:         gptr.Of(do.Description),
		EvaluationSetSchema: OpenAPIEvaluationSetSchemaDO2DTO(do.EvaluationSetSchema),
		ItemCount:           gptr.Of(do.ItemCount),
		BaseInfo:            ConvertBaseInfoDO2DTO(do.BaseInfo),
	}
}

func OpenAPIEvaluationSetSchemaDO2DTO(do *entity.EvaluationSetSchema) *openapi_eval_set.EvaluationSetSchema {
	if do == nil {
		return nil
	}
	return &openapi_eval_set.EvaluationSetSchema{
		FieldSchemas: OpenAPIFieldSchemaDO2DTOs(do.FieldSchemas),
	}
}

func OpenAPIFieldSchemaDO2DTOs(dos []*entity.FieldSchema) []*openapi_eval_set.FieldSchema {
	if dos == nil {
		return nil
	}
	result := make([]*openapi_eval_set.FieldSchema, 0)
	for _, do := range dos {
		result = append(result, OpenAPIFieldSchemaDO2DTO(do))
	}
	return result
}

func OpenAPIFieldSchemaDO2DTO(do *entity.FieldSchema) *openapi_eval_set.FieldSchema {
	if do == nil {
		return nil
	}

	displayFormat := convertDODisplayFormatToOpenAPI(do.DefaultDisplayFormat)

	contentType := convertDOContentTypeToOpenAPI(do.ContentType)

	return &openapi_eval_set.FieldSchema{
		Name:                 gptr.Of(do.Name),
		Description:          gptr.Of(do.Description),
		ContentType:          contentType,
		DefaultDisplayFormat: displayFormat,
		IsRequired:           gptr.Of(do.IsRequired),
		TextSchema:           gptr.Of(do.TextSchema),
	}
}

func ConvertBaseInfoDO2DTO(info *entity.BaseInfo) *common.BaseInfo {
	if info == nil {
		return nil
	}
	return &common.BaseInfo{
		CreatedBy: ConvertUserInfoDO2DTO(info.CreatedBy),
		UpdatedBy: ConvertUserInfoDO2DTO(info.UpdatedBy),
		CreatedAt: info.CreatedAt,
		UpdatedAt: info.UpdatedAt,
	}
}

func ConvertUserInfoDO2DTO(info *entity.UserInfo) *common.UserInfo {
	if info == nil {
		return nil
	}
	return &common.UserInfo{
		Name:      info.Name,
		AvatarURL: info.AvatarURL,
		UserID:    info.UserID,
		Email:     info.Email,
	}
}

func OpenAPIUserInfoDO2DTO(do *entity.UserInfo) *common.UserInfo {
	if do == nil {
		return nil
	}
	return &common.UserInfo{
		UserID:    do.UserID,
		Name:      do.Name,
		AvatarURL: do.AvatarURL,
		Email:     do.Email,
	}
}

// OpenAPI EvaluationSetItem 转换
func OpenAPIItemDTO2DOs(dtos []*openapi_eval_set.EvaluationSetItem) []*entity.EvaluationSetItem {
	if dtos == nil {
		return nil
	}
	result := make([]*entity.EvaluationSetItem, 0)
	for _, dto := range dtos {
		result = append(result, OpenAPIItemDTO2DO(dto))
	}
	return result
}

func OpenAPIItemDTO2DO(dto *openapi_eval_set.EvaluationSetItem) *entity.EvaluationSetItem {
	if dto == nil {
		return nil
	}
	return &entity.EvaluationSetItem{
		ItemID:  gptr.Indirect(dto.ID),
		ItemKey: gptr.Indirect(dto.ItemKey),
		Turns:   OpenAPITurnDTO2DOs(dto.Turns),
	}
}

func OpenAPITurnDTO2DOs(dtos []*openapi_eval_set.Turn) []*entity.Turn {
	if dtos == nil {
		return nil
	}
	result := make([]*entity.Turn, 0)
	for _, dto := range dtos {
		result = append(result, OpenAPITurnDTO2DO(dto))
	}
	return result
}

func OpenAPITurnDTO2DO(dto *openapi_eval_set.Turn) *entity.Turn {
	if dto == nil {
		return nil
	}
	return &entity.Turn{
		ID:            gptr.Indirect(dto.ID),
		FieldDataList: OpenAPIFieldDataDTO2DOs(dto.FieldDatas),
	}
}

func OpenAPIFieldDataDTO2DOs(dtos []*openapi_eval_set.FieldData) []*entity.FieldData {
	if dtos == nil {
		return nil
	}
	result := make([]*entity.FieldData, 0)
	for _, dto := range dtos {
		result = append(result, OpenAPIFieldDataDTO2DO(dto))
	}
	return result
}

func OpenAPIFieldDataDTO2DO(dto *openapi_eval_set.FieldData) *entity.FieldData {
	if dto == nil {
		return nil
	}
	return &entity.FieldData{
		Name:    gptr.Indirect(dto.Name),
		Content: OpenAPIContentDTO2DO(dto.Content),
	}
}

func OpenAPIContentDTO2DO(content *common.Content) *entity.Content {
	if content == nil {
		return nil
	}

	var multiPart []*entity.Content
	if content.MultiPart != nil {
		multiPart = make([]*entity.Content, 0, len(content.MultiPart))
		for _, part := range content.MultiPart {
			multiPart = append(multiPart, OpenAPIContentDTO2DO(part))
		}
	}
	return &entity.Content{
		ContentType: gptr.Of(convertOpenAPIContentTypeToDO(content.ContentType)),
		Text:        content.Text,
		Image:       ConvertImageDTO2DO(content.Image),
		MultiPart:   multiPart,
	}
}

func ConvertImageDTO2DO(img *common.Image) *entity.Image {
	if img == nil {
		return nil
	}
	return &entity.Image{
		Name:     img.Name,
		URL:      img.URL,
		ThumbURL: img.ThumbURL,
	}
}

func OpenAPIItemDO2DTOs(dos []*entity.EvaluationSetItem) []*openapi_eval_set.EvaluationSetItem {
	if dos == nil {
		return nil
	}
	result := make([]*openapi_eval_set.EvaluationSetItem, 0)
	for _, do := range dos {
		result = append(result, OpenAPIItemDO2DTO(do))
	}
	return result
}

func OpenAPIItemDO2DTO(do *entity.EvaluationSetItem) *openapi_eval_set.EvaluationSetItem {
	if do == nil {
		return nil
	}
	return &openapi_eval_set.EvaluationSetItem{
		ID:       gptr.Of(do.ID),
		ItemKey:  gptr.Of(do.ItemKey),
		Turns:    OpenAPITurnDO2DTOs(do.Turns),
		BaseInfo: ConvertBaseInfoDO2DTO(do.BaseInfo),
	}
}

func OpenAPITurnDO2DTOs(dos []*entity.Turn) []*openapi_eval_set.Turn {
	if dos == nil {
		return nil
	}
	result := make([]*openapi_eval_set.Turn, 0)
	for _, do := range dos {
		result = append(result, OpenAPITurnDO2DTO(do))
	}
	return result
}

func OpenAPITurnDO2DTO(do *entity.Turn) *openapi_eval_set.Turn {
	if do == nil {
		return nil
	}
	return &openapi_eval_set.Turn{
		ID:         gptr.Of(do.ID),
		FieldDatas: OpenAPIFieldDataDO2DTOs(do.FieldDataList),
	}
}

func OpenAPIFieldDataDO2DTOs(dos []*entity.FieldData) []*openapi_eval_set.FieldData {
	if dos == nil {
		return nil
	}
	result := make([]*openapi_eval_set.FieldData, 0)
	for _, do := range dos {
		result = append(result, OpenAPIFieldDataDO2DTO(do))
	}
	return result
}

func OpenAPIFieldDataDO2DTO(do *entity.FieldData) *openapi_eval_set.FieldData {
	if do == nil {
		return nil
	}
	return &openapi_eval_set.FieldData{
		Name:    gptr.Of(do.Name),
		Content: OpenAPIContentDO2DTO(do.Content),
	}
}

func OpenAPIContentDO2DTO(content *entity.Content) *common.Content {
	if content == nil {
		return nil
	}
	var multiPart []*common.Content
	if content.MultiPart != nil {
		multiPart = make([]*common.Content, 0, len(content.MultiPart))
		for _, part := range content.MultiPart {
			multiPart = append(multiPart, OpenAPIContentDO2DTO(part))
		}
	}
	return &common.Content{
		ContentType: convertDOContentTypeToOpenAPI(gptr.Indirect(content.ContentType)),
		Text:        content.Text,
		Image:       ConvertImageDO2DTO(content.Image),
		MultiPart:   multiPart,
	}
}

func ConvertImageDO2DTO(img *entity.Image) *common.Image {
	if img == nil {
		return nil
	}
	return &common.Image{
		Name:     img.Name,
		URL:      img.URL,
		ThumbURL: img.ThumbURL,
	}
}

func ConvertAudioDO2DTO(audio *entity.Audio) *common.Audio {
	if audio == nil {
		return nil
	}
	return &common.Audio{
		Format: audio.Format,
		URL:    audio.URL,
	}
}

func OpenAPIItemErrorGroupDO2DTOs(dos []*entity.ItemErrorGroup) []*openapi_eval_set.ItemErrorGroup {
	if dos == nil {
		return nil
	}
	result := make([]*openapi_eval_set.ItemErrorGroup, 0)
	for _, do := range dos {
		result = append(result, OpenAPIItemErrorGroupDO2DTO(do))
	}
	return result
}

func OpenAPIItemErrorGroupDO2DTO(do *entity.ItemErrorGroup) *openapi_eval_set.ItemErrorGroup {
	if do == nil {
		return nil
	}
	return &openapi_eval_set.ItemErrorGroup{
		ErrorCode:    gptr.Of(int32(gptr.Indirect(do.Type))),
		ErrorMessage: do.Summary,
		ErrorCount:   do.ErrorCount,
		Details:      OpenAPIItemErrorDetailDO2DTOs(do.Details),
	}
}

func OpenAPIItemErrorDetailDO2DTOs(dos []*entity.ItemErrorDetail) []*openapi_eval_set.ItemErrorDetail {
	if dos == nil {
		return nil
	}
	result := make([]*openapi_eval_set.ItemErrorDetail, 0)
	for _, do := range dos {
		result = append(result, OpenAPIItemErrorDetailDO2DTO(do))
	}
	return result
}

func OpenAPIItemErrorDetailDO2DTO(do *entity.ItemErrorDetail) *openapi_eval_set.ItemErrorDetail {
	if do == nil {
		return nil
	}
	return &openapi_eval_set.ItemErrorDetail{
		Message:    do.Message,
		Index:      do.Index,
		StartIndex: do.StartIndex,
		EndIndex:   do.EndIndex,
	}
}

func OpenAPIDatasetItemOutputDO2DTOs(dos []*entity.DatasetItemOutput) []*openapi_eval_set.DatasetItemOutput {
	if dos == nil {
		return nil
	}
	result := make([]*openapi_eval_set.DatasetItemOutput, 0)
	for _, do := range dos {
		result = append(result, OpenAPIDatasetItemOutputDO2DTO(do))
	}
	return result
}

func OpenAPIDatasetItemOutputDO2DTO(do *entity.DatasetItemOutput) *openapi_eval_set.DatasetItemOutput {
	if do == nil {
		return nil
	}
	return &openapi_eval_set.DatasetItemOutput{
		ItemIndex: do.ItemIndex,
		ItemKey:   do.ItemKey,
		ItemID:    do.ItemID,
		IsNewItem: do.IsNewItem,
	}
}
