// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluation_set

import (
	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/tag"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_set"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func ResourceTagRefDTO2DOs(dtos []*eval_set.ResourceTagRef) []*entity.ResourceTagRef {
	if dtos == nil {
		return nil
	}
	res := make([]*entity.ResourceTagRef, 0, len(dtos))
	for _, dto := range dtos {
		res = append(res, ResourceTagRefDTO2DO(dto))
	}
	return res
}

func ResourceTagRefDTO2DO(dto *eval_set.ResourceTagRef) *entity.ResourceTagRef {
	if dto == nil {
		return nil
	}
	return &entity.ResourceTagRef{
		TagKeyID:   dto.GetTagKeyID(),
		TagValueID: dto.TagValueID,
	}
}

func ResourceTagDTO2DOs(dtos []*eval_set.ResourceTag) []*entity.ResourceTag {
	if dtos == nil {
		return nil
	}
	res := make([]*entity.ResourceTag, 0, len(dtos))
	for _, dto := range dtos {
		res = append(res, ResourceTagDTO2DO(dto))
	}
	return res
}

func ResourceTagDTO2DO(dto *eval_set.ResourceTag) *entity.ResourceTag {
	if dto == nil {
		return nil
	}
	return &entity.ResourceTag{
		TagKeyID:     dto.GetTagKeyID(),
		TagKeyName:   dto.GetTagKeyName(),
		TagValueID:   dto.TagValueID,
		TagValueName: dto.GetTagValueName(),
		ContentType:  string(dto.GetContentType()),
		Status:       string(dto.GetStatus()),
	}
}

func ResourceTagDO2DTOs(dos []*entity.ResourceTag) []*eval_set.ResourceTag {
	if dos == nil {
		return nil
	}
	res := make([]*eval_set.ResourceTag, 0, len(dos))
	for _, do := range dos {
		res = append(res, ResourceTagDO2DTO(do))
	}
	return res
}

func ResourceTagDO2DTO(do *entity.ResourceTag) *eval_set.ResourceTag {
	if do == nil {
		return nil
	}
	return &eval_set.ResourceTag{
		TagKeyID:     do.TagKeyID,
		TagKeyName:   gptr.Of(do.TagKeyName),
		TagValueID:   do.TagValueID,
		TagValueName: gptr.Of(do.TagValueName),
		ContentType:  gptr.Of(tag.TagContentType(do.ContentType)),
		Status:       gptr.Of(tag.TagStatus(do.Status)),
	}
}

func TagFilterDTO2DO(dto *eval_set.TagFilter) *entity.TagFilter {
	if dto == nil {
		return nil
	}
	var relation *entity.TagFilterRelation
	if dto.Relation != nil {
		relation = gptr.Of(entity.TagFilterRelation(dto.GetRelation()))
	}
	return &entity.TagFilter{
		TagKeyIDs: dto.GetTagKeyIds(),
		Tags:      ResourceTagRefDTO2DOs(dto.GetTags()),
		Relation:  relation,
	}
}
