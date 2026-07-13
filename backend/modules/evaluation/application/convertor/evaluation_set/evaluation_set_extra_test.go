// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluation_set

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_set"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestResourceTagRefDTO2DOs(t *testing.T) {
	t.Parallel()

	assert.Nil(t, ResourceTagRefDTO2DOs(nil))
	assert.Empty(t, ResourceTagRefDTO2DOs([]*eval_set.ResourceTagRef{}))

	dtos := []*eval_set.ResourceTagRef{
		nil,
		{TagName: "a"},
		{TagName: "b"},
	}
	got := ResourceTagRefDTO2DOs(dtos)
	if assert.Len(t, got, 2) {
		assert.Equal(t, "a", got[0].TagName)
		assert.Equal(t, "b", got[1].TagName)
	}
}

func TestResourceTagDO2DTO(t *testing.T) {
	t.Parallel()

	assert.Nil(t, ResourceTagDO2DTO(nil))

	// minimal DO
	got := ResourceTagDO2DTO(&entity.ResourceTag{TagName: "n"})
	if assert.NotNil(t, got) {
		assert.Equal(t, "n", got.TagName)
		assert.Nil(t, got.TagKeyID)
		assert.Nil(t, got.ContentType)
		assert.Nil(t, got.Status)
	}

	// full DO
	got = ResourceTagDO2DTO(&entity.ResourceTag{
		TagName:     "x",
		TagKeyID:    7,
		ContentType: "text",
		Status:      "active",
	})
	if assert.NotNil(t, got) {
		if assert.NotNil(t, got.TagKeyID) {
			assert.Equal(t, int64(7), *got.TagKeyID)
		}
		assert.NotNil(t, got.ContentType)
		assert.NotNil(t, got.Status)
	}
}

func TestResourceTagDO2DTOs(t *testing.T) {
	t.Parallel()

	assert.Nil(t, ResourceTagDO2DTOs(nil))
	assert.Empty(t, ResourceTagDO2DTOs([]*entity.ResourceTag{}))

	// nil entries are dropped
	got := ResourceTagDO2DTOs([]*entity.ResourceTag{
		nil,
		{TagName: "a"},
	})
	if assert.Len(t, got, 1) {
		assert.Equal(t, "a", got[0].TagName)
	}
}

func TestTagFilterDTO2DO_HappyDedupSort(t *testing.T) {
	t.Parallel()

	relOr := eval_set.TagFilterRelationOr
	got, err := TagFilterDTO2DO(&eval_set.TagFilter{
		TagNames: []string{" beta ", "alpha", "alpha", "gamma"},
		Relation: &relOr,
	})
	assert.NoError(t, err)
	if assert.NotNil(t, got) {
		// dedup + trim + sort
		assert.Equal(t, []string{"alpha", "beta", "gamma"}, got.TagNames)
		assert.Equal(t, entity.TagFilterRelationOr, got.Relation)
	}
}

func TestContentTypeDO2DTO(t *testing.T) {
	t.Parallel()

	assert.Nil(t, contentTypeDO2DTO(nil))

	cases := []struct {
		in   entity.ContentType
		want dataset.ContentType
	}{
		{entity.ContentTypeText, dataset.ContentType_Text},
		{entity.ContentTypeImage, dataset.ContentType_Image},
		{entity.ContentTypeAudio, dataset.ContentType_Audio},
		{entity.ContentTypeVideo, dataset.ContentType_Video},
		{entity.ContentTypeMultipart, dataset.ContentType_MultiPart},
	}
	for _, c := range cases {
		c := c
		t.Run(string(c.in), func(t *testing.T) {
			t.Parallel()
			got := contentTypeDO2DTO(&c.in)
			if assert.NotNil(t, got) {
				assert.Equal(t, c.want, *got)
			}
		})
	}

	unknown := entity.ContentType("unknown")
	assert.Nil(t, contentTypeDO2DTO(&unknown))
}
