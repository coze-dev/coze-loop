// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package data

import (
	"context"
	"strings"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/common"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func convert2DatasetOrderBys(ctx context.Context, orderBys []*entity.OrderBy) (datasetOrderBys []*dataset.OrderBy) {
	if len(orderBys) == 0 {
		return nil
	}
	datasetOrderBys = make([]*dataset.OrderBy, 0)
	for _, orderBy := range orderBys {
		datasetOrderBys = append(datasetOrderBys, convert2DatasetOrderBy(ctx, orderBy))
	}
	return datasetOrderBys
}

func convert2DatasetOrderBy(ctx context.Context, orderBy *entity.OrderBy) (datasetOrderBy *dataset.OrderBy) {
	if orderBy == nil {
		return nil
	}
	return &dataset.OrderBy{
		Field: orderBy.Field,
		IsAsc: orderBy.IsAsc,
	}
}

func convert2DatasetMultiModalSpec(ctx context.Context, multiModalSpec *entity.MultiModalSpec) (datasetMultiModalSpec *dataset.MultiModalSpec) {
	if multiModalSpec == nil {
		return nil
	}
	return &dataset.MultiModalSpec{
		MaxFileCount:     &multiModalSpec.MaxFileCount,
		MaxFileSize:      &multiModalSpec.MaxFileSize,
		SupportedFormats: multiModalSpec.SupportedFormats,
	}
}

func convert2DatasetFieldSchemas(ctx context.Context, schemas []*entity.FieldSchema) (fieldSchemas []*dataset.FieldSchema, err error) {
	if len(schemas) == 0 {
		return nil, nil
	}
	fieldSchemas = make([]*dataset.FieldSchema, 0)
	for _, schema := range schemas {
		fieldSchema, err := convert2DatasetFieldSchema(ctx, schema)
		if err != nil {
			return nil, err
		}
		fieldSchemas = append(fieldSchemas, fieldSchema)
	}
	return fieldSchemas, nil
}

func convert2DatasetFieldSchema(ctx context.Context, schema *entity.FieldSchema) (fieldSchema *dataset.FieldSchema, err error) {
	if schema == nil {
		return nil, nil
	}
	var contentType *dataset.ContentType
	if schema.ContentType != "" {
		convRes, err := dataset.ContentTypeFromString(common.ConvertContentTypeDO2DTO(schema.ContentType))
		if err != nil {
			return nil, err
		}
		contentType = &convRes
	}
	fieldSchema = &dataset.FieldSchema{
		Key:            &schema.Key,
		Name:           &schema.Name,
		Description:    &schema.Description,
		ContentType:    contentType,
		DefaultFormat:  gptr.Of(dataset.FieldDisplayFormat(schema.DefaultDisplayFormat)),
		Status:         gptr.Of(dataset.FieldStatus(schema.Status)),
		MultiModelSpec: convert2DatasetMultiModalSpec(ctx, schema.MultiModelSpec),
		TextSchema:     &schema.TextSchema,
		Hidden:         &schema.Hidden,
	}
	return fieldSchema, nil
}

func convert2DatasetData(ctx context.Context, turns []*entity.Turn) (data []*dataset.FieldData, err error) {
	if len(turns) == 0 {
		return nil, nil
	}
	// 单轮只取第一个元素
	turn := turns[0]
	data = make([]*dataset.FieldData, 0)
	for _, e := range turn.FieldDataList {
		fieldData, err := convert2DatasetFieldData(ctx, e)
		if err != nil {
			return nil, err
		}
		data = append(data, fieldData)
	}
	return data, nil
}

func convert2DatasetFieldData(ctx context.Context, fieldData *entity.FieldData) (datasetFieldData *dataset.FieldData, err error) {
	if fieldData == nil {
		return nil, nil
	}
	datasetFieldData = &dataset.FieldData{
		Key:  &fieldData.Key,
		Name: &fieldData.Name,
	}
	if fieldData.Content != nil {
		var contentType *dataset.ContentType
		if fieldData.Content.ContentType != nil {
			convRes, err := dataset.ContentTypeFromString(common.ConvertContentTypeDO2DTO(gptr.Indirect(fieldData.Content.ContentType)))
			if err != nil {
				return nil, err
			}
			contentType = &convRes
		}
		datasetFieldData.ContentType = contentType
		datasetFieldData.Format = gptr.Of(dataset.FieldDisplayFormat(gptr.Indirect(fieldData.Content.Format)))
		// TODO image multi-parts本期不支持，故暂不实现
		datasetFieldData.Content = fieldData.Content.Text
	}
	return datasetFieldData, nil
}

func convert2DatasetItem(ctx context.Context, item *entity.EvaluationSetItem) (datasetItem *dataset.DatasetItem, err error) {
	if item == nil {
		return nil, nil
	}
	data, err := convert2DatasetData(ctx, item.Turns)
	if err != nil {
		return nil, err
	}
	datasetItem = &dataset.DatasetItem{
		ID:        &item.ID,
		AppID:     &item.AppID,
		SpaceID:   &item.SpaceID,
		DatasetID: &item.EvaluationSetID,
		SchemaID:  &item.SchemaID,
		ItemID:    &item.ItemID,
		ItemKey:   &item.ItemKey,
		Data:      data,
	}
	return datasetItem, nil
}

func convert2DatasetItems(ctx context.Context, items []*entity.EvaluationSetItem) (datasetItems []*dataset.DatasetItem, err error) {
	if len(items) == 0 {
		return nil, nil
	}
	datasetItems = make([]*dataset.DatasetItem, 0)
	for _, item := range items {
		datasetItem, err := convert2DatasetItem(ctx, item)
		if err != nil {
			return nil, err
		}
		datasetItems = append(datasetItems, datasetItem)
	}
	return datasetItems, nil
}

func convert2EvaluationSetSpec(ctx context.Context, spec *dataset.DatasetSpec) (evaluationSetSpec *entity.DatasetSpec) {
	if spec == nil {
		return nil
	}
	evaluationSetSpec = &entity.DatasetSpec{
		MaxFieldCount:  gptr.Indirect(spec.MaxFieldCount),
		MaxItemCount:   gptr.Indirect(spec.MaxItemCount),
		MaxItemSize:    gptr.Indirect(spec.MaxItemSize),
		MultiModalSpec: convert2EvaluationSetMultiModalSpec(ctx, spec.MultiModalSpec),
	}
	return evaluationSetSpec
}

func convert2DatasetFeatures(ctx context.Context, features *dataset.DatasetFeatures) (evaluationSetFeatures *entity.DatasetFeatures) {
	if features == nil {
		return nil
	}
	evaluationSetFeatures = &entity.DatasetFeatures{
		EditSchema:   gptr.Indirect(features.EditSchema),
		RepeatedData: gptr.Indirect(features.RepeatedData),
		MultiModal:   gptr.Indirect(features.MultiModal),
	}
	return evaluationSetFeatures
}

func convert2EvaluationSetMultiModalSpec(ctx context.Context, multiModalSpec *dataset.MultiModalSpec) (evaluationSetMultiModalSpec *entity.MultiModalSpec) {
	if multiModalSpec == nil {
		return nil
	}
	return &entity.MultiModalSpec{
		MaxFileCount:     gptr.Indirect(multiModalSpec.MaxFileCount),
		MaxFileSize:      gptr.Indirect(multiModalSpec.MaxFileSize),
		SupportedFormats: multiModalSpec.SupportedFormats,
	}
}

func convert2EvaluationSetFieldSchemas(ctx context.Context, schemas []*dataset.FieldSchema) (fieldSchemas []*entity.FieldSchema) {
	if len(schemas) == 0 {
		return nil
	}
	fieldSchemas = make([]*entity.FieldSchema, 0)
	for _, schema := range schemas {
		fieldSchemas = append(fieldSchemas, convert2EvaluationSetFieldSchema(ctx, schema))
	}
	return fieldSchemas
}

func convert2EvaluationSetFieldSchema(ctx context.Context, schema *dataset.FieldSchema) (fieldSchema *entity.FieldSchema) {
	if schema == nil {
		return nil
	}
	fieldSchema = &entity.FieldSchema{
		Key:                  gptr.Indirect(schema.Key),
		Name:                 gptr.Indirect(schema.Name),
		Description:          gptr.Indirect(schema.Description),
		ContentType:          common.ConvertContentTypeDTO2DO(schema.ContentType.String()),
		DefaultDisplayFormat: entity.FieldDisplayFormat(gptr.Indirect(schema.DefaultFormat)),
		Status:               entity.FieldStatus(gptr.Indirect(schema.Status)),
		MultiModelSpec:       convert2EvaluationSetMultiModalSpec(ctx, schema.MultiModelSpec),
		TextSchema:           gptr.Indirect(schema.TextSchema),
		Hidden:               gptr.Indirect(schema.Hidden),
	}
	return fieldSchema
}

func convert2EvaluationSetSchema(ctx context.Context, schema *dataset.DatasetSchema) (datasetSchema *entity.EvaluationSetSchema) {
	if schema == nil {
		return nil
	}
	datasetSchema = &entity.EvaluationSetSchema{
		ID:              gptr.Indirect(schema.ID),
		AppID:           gptr.Indirect(schema.AppID),
		SpaceID:         gptr.Indirect(schema.SpaceID),
		EvaluationSetID: gptr.Indirect(schema.DatasetID),
		FieldSchemas:    convert2EvaluationSetFieldSchemas(ctx, schema.Fields),
		BaseInfo: &entity.BaseInfo{
			CreatedAt: schema.CreatedAt,
			UpdatedAt: schema.UpdatedAt,
			CreatedBy: &entity.UserInfo{UserID: schema.CreatedBy},
			UpdatedBy: &entity.UserInfo{UserID: schema.UpdatedBy},
		},
	}
	return datasetSchema
}

func convert2EvaluationSetDraftVersion(ctx context.Context, dataset *dataset.Dataset) (evaluationSetVersion *entity.EvaluationSetVersion) {
	if dataset == nil {
		return nil
	}
	evaluationSetVersion = &entity.EvaluationSetVersion{
		ID:                  dataset.ID,
		AppID:               gptr.Indirect(dataset.AppID),
		SpaceID:             dataset.SpaceID,
		EvaluationSetID:     dataset.ID,
		Description:         gptr.Indirect(dataset.Description),
		EvaluationSetSchema: convert2EvaluationSetSchema(ctx, dataset.Schema),
		ItemCount:           gptr.Indirect(dataset.ItemCount),
		BaseInfo: &entity.BaseInfo{
			CreatedAt: dataset.CreatedAt,
			CreatedBy: &entity.UserInfo{UserID: dataset.CreatedBy},
		},
	}
	return evaluationSetVersion
}

func convert2EvaluationSets(ctx context.Context, datasets []*dataset.Dataset) (evaluationSets []*entity.EvaluationSet) {
	if len(datasets) == 0 {
		return nil
	}
	evaluationSets = make([]*entity.EvaluationSet, 0)
	for _, dataset := range datasets {
		evaluationSets = append(evaluationSets, convert2EvaluationSet(ctx, dataset))
	}
	return evaluationSets
}

func convert2EvaluationSet(ctx context.Context, dataset *dataset.Dataset) (evaluationSet *entity.EvaluationSet) {
	if dataset == nil {
		return nil
	}
	evaluationSet = &entity.EvaluationSet{
		ID:                   dataset.ID,
		AppID:                gptr.Indirect(dataset.AppID),
		SpaceID:              dataset.SpaceID,
		Name:                 gptr.Indirect(dataset.Name),
		Description:          gptr.Indirect(dataset.Description),
		Status:               entity.DatasetStatus(gptr.Indirect(dataset.Status)),
		Spec:                 convert2EvaluationSetSpec(ctx, dataset.Spec),
		Features:             convert2DatasetFeatures(ctx, dataset.Features),
		ItemCount:            gptr.Indirect(dataset.ItemCount),
		ChangeUncommitted:    gptr.Indirect(dataset.ChangeUncommitted),
		EvaluationSetVersion: convert2EvaluationSetDraftVersion(ctx, dataset),
		LatestVersion:        gptr.Indirect(dataset.LatestVersion),
		NextVersionNum:       gptr.Indirect(dataset.NextVersionNum),
		BaseInfo: &entity.BaseInfo{
			CreatedAt: dataset.CreatedAt,
			UpdatedAt: dataset.UpdatedAt,
			CreatedBy: &entity.UserInfo{UserID: dataset.CreatedBy},
			UpdatedBy: &entity.UserInfo{UserID: dataset.UpdatedBy},
		},
	}
	return evaluationSet
}

func convert2EvaluationSetVersions(ctx context.Context, versions []*dataset.DatasetVersion) (evaluationSetVersions []*entity.EvaluationSetVersion) {
	if len(versions) == 0 {
		return nil
	}
	evaluationSetVersions = make([]*entity.EvaluationSetVersion, 0)
	for _, version := range versions {
		evaluationSetVersions = append(evaluationSetVersions, convert2EvaluationSetVersion(ctx, version, &dataset.Dataset{}))
	}
	return evaluationSetVersions
}

func convert2EvaluationSetVersion(ctx context.Context, version *dataset.DatasetVersion, dataset *dataset.Dataset) (evaluationSetVersion *entity.EvaluationSetVersion) {
	if version == nil {
		return nil
	}
	evaluationSetVersion = &entity.EvaluationSetVersion{
		ID:              version.ID,
		AppID:           gptr.Indirect(version.AppID),
		SpaceID:         version.SpaceID,
		EvaluationSetID: version.DatasetID,
		Version:         gptr.Indirect(version.Version),
		VersionNum:      gptr.Indirect(version.VersionNum),
		Description:     gptr.Indirect(version.Description),
		ItemCount:       gptr.Indirect(version.ItemCount),
		BaseInfo: &entity.BaseInfo{
			CreatedAt: version.CreatedAt,
			CreatedBy: &entity.UserInfo{UserID: version.CreatedBy},
		},
	}
	if dataset != nil {
		evaluationSetVersion.EvaluationSetSchema = convert2EvaluationSetSchema(ctx, dataset.Schema)
	}
	return evaluationSetVersion
}

func convert2EvaluationSetFieldData(ctx context.Context, fieldData *dataset.FieldData) (evalSetFieldData *entity.FieldData) {
	if fieldData == nil {
		return nil
	}

	// 转换 Parts 为 MultiPart
	var multiPart []*entity.Content
	if len(fieldData.Parts) > 0 {
		multiPart = make([]*entity.Content, 0, len(fieldData.Parts))
		for _, part := range fieldData.Parts {
			// 为每个 part 创建 Content，包含完整的多媒体转换
			partContent := &entity.Content{
				ContentType: gptr.Of(common.ConvertContentTypeDTO2DO(part.GetContentType().String())),
				Format:      gptr.Of(common.ConvertFieldDisplayFormatDTO2DO(int64(gptr.Indirect(part.Format)))),
				Text:        part.Content,
				Image:       convertObjectStorageToImage(ctx, part.Attachments),
				Audio:       convertObjectStorageToAudio(ctx, part.Attachments),
			}

			// 如果 part 还有嵌套的 Parts，递归处理
			if len(part.Parts) > 0 {
				nestedMultiPart := make([]*entity.Content, 0, len(part.Parts))
				for _, nestedPart := range part.Parts {
					nestedFieldData := convert2EvaluationSetFieldData(ctx, nestedPart)
					if nestedFieldData != nil && nestedFieldData.Content != nil {
						nestedMultiPart = append(nestedMultiPart, nestedFieldData.Content)
					}
				}
				partContent.MultiPart = nestedMultiPart
			}

			multiPart = append(multiPart, partContent)
		}
	}

	evalSetFieldData = &entity.FieldData{
		Key:  gptr.Indirect(fieldData.Key),
		Name: gptr.Indirect(fieldData.Name),
		Content: &entity.Content{
			ContentType: gptr.Of(common.ConvertContentTypeDTO2DO(fieldData.GetContentType().String())),
			Format:      gptr.Of(common.ConvertFieldDisplayFormatDTO2DO(int64(gptr.Indirect(fieldData.Format)))),
			Text:        fieldData.Content,
			Image:       convertObjectStorageToImage(ctx, fieldData.Attachments),
			Audio:       convertObjectStorageToAudio(ctx, fieldData.Attachments),
			MultiPart:   multiPart,
		},
	}
	return evalSetFieldData
}

// convertObjectStorageToImage 从 ObjectStorage 列表中提取图片信息并转换为 entity.Image
func convertObjectStorageToImage(ctx context.Context, attachments []*dataset.ObjectStorage) *entity.Image {
	if len(attachments) == 0 {
		return nil
	}

	// 查找第一个图片类型的 attachment
	for _, attachment := range attachments {
		if attachment == nil {
			continue
		}

		// 根据文件名或其他信息判断是否为图片
		if isImageAttachment(attachment) {
			return &entity.Image{
				Name:            attachment.Name,
				URL:             attachment.URL,
				URI:             attachment.URI,
				ThumbURL:        attachment.ThumbURL,
				StorageProvider: convertStorageProvider(attachment.Provider),
			}
		}
	}

	return nil
}

// convertObjectStorageToAudio 从 ObjectStorage 列表中提取音频信息并转换为 entity.Audio
func convertObjectStorageToAudio(ctx context.Context, attachments []*dataset.ObjectStorage) *entity.Audio {
	if len(attachments) == 0 {
		return nil
	}

	// 查找第一个音频类型的 attachment
	for _, attachment := range attachments {
		if attachment == nil {
			continue
		}

		// 根据文件名或其他信息判断是否为音频
		if isAudioAttachment(attachment) {
			return &entity.Audio{
				Format: getAudioFormat(attachment),
				URL:    attachment.URL,
			}
		}
	}

	return nil
}

// isImageAttachment 判断 ObjectStorage 是否为图片类型
func isImageAttachment(attachment *dataset.ObjectStorage) bool {
	if attachment == nil || attachment.Name == nil {
		return false
	}

	name := *attachment.Name
	// 根据文件扩展名判断是否为图片
	imageExtensions := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".svg", ".ico", ".tiff", ".tif"}
	for _, ext := range imageExtensions {
		if len(name) >= len(ext) && name[len(name)-len(ext):] == ext {
			return true
		}
		// 也检查大写版本
		if len(name) >= len(ext) {
			nameExt := name[len(name)-len(ext):]
			if nameExt == strings.ToUpper(ext) {
				return true
			}
		}
	}

	return false
}

// isAudioAttachment 判断 ObjectStorage 是否为音频类型
func isAudioAttachment(attachment *dataset.ObjectStorage) bool {
	if attachment == nil || attachment.Name == nil {
		return false
	}

	name := *attachment.Name
	// 根据文件扩展名判断是否为音频
	audioExtensions := []string{".mp3", ".wav", ".flac", ".aac", ".ogg", ".m4a", ".wma", ".opus", ".amr"}
	for _, ext := range audioExtensions {
		if len(name) >= len(ext) && name[len(name)-len(ext):] == ext {
			return true
		}
		// 也检查大写版本
		if len(name) >= len(ext) {
			nameExt := name[len(name)-len(ext):]
			if nameExt == strings.ToUpper(ext) {
				return true
			}
		}
	}

	return false
}

// getAudioFormat 从 ObjectStorage 中获取音频格式
func getAudioFormat(attachment *dataset.ObjectStorage) *string {
	if attachment == nil || attachment.Name == nil {
		return nil
	}

	name := *attachment.Name
	// 提取文件扩展名作为格式
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			format := name[i+1:]
			return &format
		}
	}

	return nil
}

// convertStorageProvider 转换存储提供商类型
func convertStorageProvider(provider *dataset.StorageProvider) *entity.StorageProvider {
	if provider == nil {
		return nil
	}

	// 将 dataset.StorageProvider 转换为 entity.StorageProvider
	entityProvider := entity.StorageProvider(*provider)
	return &entityProvider
}

func convert2EvaluationSetTurn(ctx context.Context, data []*dataset.FieldData) (turns []*entity.Turn) {
	if len(data) == 0 {
		return nil
	}
	turn := &entity.Turn{
		FieldDataList: make([]*entity.FieldData, 0),
	}
	for _, e := range data {
		turn.FieldDataList = append(turn.FieldDataList, convert2EvaluationSetFieldData(ctx, e))
	}
	turns = append(turns, turn)
	return turns
}

func convert2EvaluationSetItem(ctx context.Context, item *dataset.DatasetItem) (datasetItem *entity.EvaluationSetItem) {
	if item == nil {
		return nil
	}
	datasetItem = &entity.EvaluationSetItem{
		ID:              gptr.Indirect(item.ID),
		AppID:           gptr.Indirect(item.AppID),
		SpaceID:         gptr.Indirect(item.SpaceID),
		EvaluationSetID: gptr.Indirect(item.DatasetID),
		SchemaID:        gptr.Indirect(item.SchemaID),
		ItemID:          gptr.Indirect(item.ItemID),
		ItemKey:         gptr.Indirect(item.ItemKey),
		Turns:           convert2EvaluationSetTurn(ctx, item.Data),
		BaseInfo: &entity.BaseInfo{
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
			CreatedBy: &entity.UserInfo{UserID: item.CreatedBy},
			UpdatedBy: &entity.UserInfo{UserID: item.UpdatedBy},
		},
	}
	return datasetItem
}

func convert2EvaluationSetItems(ctx context.Context, items []*dataset.DatasetItem) (evalSetItems []*entity.EvaluationSetItem) {
	if len(items) == 0 {
		return nil
	}
	evalSetItems = make([]*entity.EvaluationSetItem, 0)
	for _, item := range items {
		evalSetItems = append(evalSetItems, convert2EvaluationSetItem(ctx, item))
	}
	return evalSetItems
}

func convert2EvaluationSetErrorGroups(ctx context.Context, errors []*dataset.ItemErrorGroup) (res []*entity.ItemErrorGroup) {
	if len(errors) == 0 {
		return nil
	}
	res = make([]*entity.ItemErrorGroup, 0)
	for _, err := range errors {
		res = append(res, convert2EvaluationSetErrorGroup(ctx, err))
	}
	return res
}

func convert2EvaluationSetErrorGroup(ctx context.Context, errorGroup *dataset.ItemErrorGroup) (res *entity.ItemErrorGroup) {
	if errorGroup == nil {
		return nil
	}
	res = &entity.ItemErrorGroup{
		Type:       gptr.Of(entity.ItemErrorType(gptr.Indirect(errorGroup.Type))),
		Summary:    errorGroup.Summary,
		ErrorCount: errorGroup.ErrorCount,
		Details:    convert2EvaluationSetErrorDetails(ctx, errorGroup.Details),
	}
	return res
}

func convert2EvaluationSetErrorDetails(ctx context.Context, errorDetails []*dataset.ItemErrorDetail) (res []*entity.ItemErrorDetail) {
	if len(errorDetails) == 0 {
		return nil
	}
	res = make([]*entity.ItemErrorDetail, 0)
	for _, detail := range errorDetails {
		res = append(res, convert2EvaluationSetErrorDetail(ctx, detail))
	}
	return res
}

func convert2EvaluationSetErrorDetail(ctx context.Context, errorDetail *dataset.ItemErrorDetail) (res *entity.ItemErrorDetail) {
	if errorDetail == nil {
		return nil
	}
	res = &entity.ItemErrorDetail{
		Message:    errorDetail.Message,
		Index:      errorDetail.Index,
		StartIndex: errorDetail.StartIndex,
		EndIndex:   errorDetail.EndIndex,
	}
	return res
}
