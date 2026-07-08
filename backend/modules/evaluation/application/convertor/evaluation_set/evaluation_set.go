// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluation_set

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	data_tag "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/tag"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_set"
	app_eval_set "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_set"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/common"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func EvaluationSetDO2DTOs(dos []*entity.EvaluationSet) []*eval_set.EvaluationSet {
	if dos == nil {
		return nil
	}
	result := make([]*eval_set.EvaluationSet, 0)
	for _, do := range dos {
		result = append(result, EvaluationSetDO2DTO(do))
	}
	return result
}

func EvaluationSetDO2DTO(do *entity.EvaluationSet) *eval_set.EvaluationSet {
	if do == nil {
		return nil
	}
	var spec *dataset.DatasetSpec
	if do.Spec != nil {
		spec = &dataset.DatasetSpec{
			MaxItemCount:           gptr.Of(do.Spec.MaxItemCount),
			MaxFieldCount:          gptr.Of(do.Spec.MaxFieldCount),
			MaxItemSize:            gptr.Of(do.Spec.MaxItemSize),
			MaxItemDataNestedDepth: gptr.Of(do.Spec.MaxItemDataNestedDepth),
			MultiModalSpec:         MultiModalSpecDO2DTO(do.Spec.MultiModalSpec),
		}
	}
	var features *dataset.DatasetFeatures
	if do.Features != nil {
		features = &dataset.DatasetFeatures{
			EditSchema:   gptr.Of(do.Features.EditSchema),
			RepeatedData: gptr.Of(do.Features.RepeatedData),
			MultiModal:   gptr.Of(do.Features.MultiModal),
		}
	}

	return &eval_set.EvaluationSet{
		ID:                   gptr.Of(do.ID),
		AppID:                gptr.Of(do.AppID),
		WorkspaceID:          gptr.Of(do.SpaceID),
		Name:                 gptr.Of(do.Name),
		Description:          gptr.Of(do.Description),
		Status:               gptr.Of(dataset.DatasetStatus(do.Status)),
		Spec:                 spec,
		Features:             features,
		ItemCount:            gptr.Of(do.ItemCount),
		ChangeUncommitted:    gptr.Of(do.ChangeUncommitted),
		EvaluationSetVersion: VersionDO2DTO(do.EvaluationSetVersion),
		LatestVersion:        gptr.Of(do.LatestVersion),
		NextVersionNum:       gptr.Of(do.NextVersionNum),
		BaseInfo:             common.ConvertBaseInfoDO2DTO(do.BaseInfo),
		Type:                 do.DatasetType,
		Tags:                 ResourceTagDO2DTOs(do.Tags),
		DatasetKey:           gptr.Of(do.DatasetKey),
	}
}

func ResourceTagRefDTO2DOs(dtos []*eval_set.ResourceTagRef) []*entity.ResourceTagRef {
	if dtos == nil {
		return nil
	}
	result := make([]*entity.ResourceTagRef, 0, len(dtos))
	for _, dto := range dtos {
		if dto == nil {
			continue
		}
		result = append(result, &entity.ResourceTagRef{
			TagName: dto.GetTagName(),
		})
	}
	return result
}

func ResourceTagDO2DTOs(dos []*entity.ResourceTag) []*eval_set.ResourceTag {
	if dos == nil {
		return nil
	}
	result := make([]*eval_set.ResourceTag, 0, len(dos))
	for _, do := range dos {
		dto := ResourceTagDO2DTO(do)
		if dto != nil {
			result = append(result, dto)
		}
	}
	return result
}

func ResourceTagDO2DTO(do *entity.ResourceTag) *eval_set.ResourceTag {
	if do == nil {
		return nil
	}
	dto := &eval_set.ResourceTag{
		TagName: do.TagName,
	}
	if do.TagKeyID != 0 {
		dto.TagKeyID = gptr.Of(do.TagKeyID)
	}
	if do.ContentType != "" {
		contentType := data_tag.TagContentType(do.ContentType)
		dto.ContentType = &contentType
	}
	if do.Status != "" {
		status := data_tag.TagStatus(do.Status)
		dto.Status = &status
	}
	return dto
}

func TagFilterDTO2DO(dto *eval_set.TagFilter) (*entity.TagFilter, error) {
	if dto == nil {
		return nil, nil
	}
	tagNames, err := normalizeTagNames(dto.GetTagNames())
	if err != nil {
		return nil, err
	}
	if len(tagNames) == 0 {
		return nil, fmt.Errorf("tag_filter.tag_names is required")
	}
	relation := entity.TagFilterRelationOr
	switch dto.GetRelation() {
	case "", eval_set.TagFilterRelationOr:
	case eval_set.TagFilterRelationAnd:
		relation = entity.TagFilterRelationAnd
	default:
		return nil, fmt.Errorf("tag_filter.relation must be or or and")
	}
	return &entity.TagFilter{
		TagNames: tagNames,
		Relation: relation,
	}, nil
}

func normalizeTagNames(tagNames []string) ([]string, error) {
	uniq := make(map[string]struct{}, len(tagNames))
	for _, tagName := range tagNames {
		normalized := strings.TrimSpace(tagName)
		if normalized == "" {
			return nil, fmt.Errorf("tag_name is required")
		}
		uniq[normalized] = struct{}{}
	}
	res := make([]string, 0, len(uniq))
	for tagName := range uniq {
		res = append(res, tagName)
	}
	sort.Strings(res)
	return res, nil
}

func FieldWriteOptionDTO2DOs(dtos []*dataset.FieldWriteOption) []*entity.FieldWriteOption {
	if dtos == nil {
		return nil
	}
	var res []*entity.FieldWriteOption
	for _, dto := range dtos {
		res = append(res, FieldWriteOptionDTO2DO(dto))
	}
	return res
}

func FieldWriteOptionDTO2DO(dto *dataset.FieldWriteOption) *entity.FieldWriteOption {
	if dto == nil {
		return nil
	}
	var messageListStrategy *entity.MultiModalStoreStrategy
	if dto.MessageListStoreStrategy != nil {
		s := entity.MultiModalStoreStrategy(*dto.MessageListStoreStrategy)
		messageListStrategy = &s
	}
	return &entity.FieldWriteOption{
		FieldName:                dto.FieldName,
		FieldKey:                 dto.FieldKey,
		MultiModalStoreOpt:       MultiModalStoreOptionDTO2DO(dto.MultiModalStoreOpt),
		MessageListStoreStrategy: messageListStrategy,
	}
}

func MultiModalStoreOptionDTO2DO(dto *dataset.MultiModalStoreOption) *entity.MultiModalStoreOption {
	if dto == nil {
		return nil
	}
	var strategy *entity.MultiModalStoreStrategy
	if dto.MultiModalStoreStrategy != nil {
		s := entity.MultiModalStoreStrategy(*dto.MultiModalStoreStrategy)
		strategy = &s
	}
	var contentType *entity.ContentType
	if dto.ContentType != nil {
		var t entity.ContentType
		switch *dto.ContentType {
		case dataset.ContentType_Text:
			t = entity.ContentTypeText
		case dataset.ContentType_Image:
			t = entity.ContentTypeImage
		case dataset.ContentType_Audio:
			t = entity.ContentTypeAudio
		case dataset.ContentType_Video:
			t = entity.ContentTypeVideo
		case dataset.ContentType_MultiPart:
			t = entity.ContentTypeMultipart
		}
		if t != "" {
			contentType = &t
		}
	}
	return &entity.MultiModalStoreOption{
		MultiModalStoreStrategy: strategy,
		ContentType:             contentType,
	}
}

func UploadAttachmentDetailsDO2DTOs(dos []*entity.UploadAttachmentDetail) []*app_eval_set.UploadAttachmentDetail {
	if dos == nil {
		return nil
	}
	res := make([]*app_eval_set.UploadAttachmentDetail, 0, len(dos))
	for _, do := range dos {
		res = append(res, UploadAttachmentDetailDO2DTO(do))
	}
	return res
}

func UploadAttachmentDetailDO2DTO(do *entity.UploadAttachmentDetail) *app_eval_set.UploadAttachmentDetail {
	if do == nil {
		return nil
	}
	dto := &app_eval_set.UploadAttachmentDetail{
		ContentType:     contentTypeDO2DTO(do.ContentType),
		ImagexServiceID: do.ImagexServiceID,
		OriginImage:     common.ConvertImageDO2DTO(do.OriginImage),
		Image:           common.ConvertImageDO2DTO(do.Image),
		OriginAudio:     common.ConvertAudioDO2DTO(do.OriginAudio),
		Audio:           common.ConvertAudioDO2DTO(do.Audio),
		OriginVideo:     common.ConvertVideoDO2DTO(do.OriginVideo),
		Video:           common.ConvertVideoDO2DTO(do.Video),
		ErrMsg:          do.ErrMsg,
	}
	if do.ErrorType != nil {
		dto.ErrorType = gptr.Of(dataset.ItemErrorType(gptr.Indirect(do.ErrorType)))
	}
	return dto
}

func contentTypeDO2DTO(ct *entity.ContentType) *dataset.ContentType {
	if ct == nil {
		return nil
	}
	var t dataset.ContentType
	switch *ct {
	case entity.ContentTypeText:
		t = dataset.ContentType_Text
	case entity.ContentTypeImage:
		t = dataset.ContentType_Image
	case entity.ContentTypeAudio:
		t = dataset.ContentType_Audio
	case entity.ContentTypeVideo:
		t = dataset.ContentType_Video
	case entity.ContentTypeMultipart:
		t = dataset.ContentType_MultiPart
	default:
		return nil
	}
	return &t
}
