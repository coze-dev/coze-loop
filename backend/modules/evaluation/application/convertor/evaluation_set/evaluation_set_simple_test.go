// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluation_set

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_set"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestCreateDatasetItemOutputDO2DTOs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []*entity.DatasetItemOutput
		expected []*dataset.CreateDatasetItemOutput
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "empty slice",
			input: []*entity.DatasetItemOutput{
				{
					ItemIndex: gptr.Of(int32(1)),
					ItemKey:   gptr.Of("key1"),
					ItemID:    gptr.Of(int64(1)),
					IsNewItem: gptr.Of(true),
				},
			},
			expected: []*dataset.CreateDatasetItemOutput{
				{
					ItemIndex: gptr.Of(int32(1)),
					ItemKey:   gptr.Of("key1"),
					ItemID:    gptr.Of(int64(1)),
					IsNewItem: gptr.Of(true),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := CreateDatasetItemOutputDO2DTOs(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluationSetDO2DTOs_Simple(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []*entity.EvaluationSet
		expected []*eval_set.EvaluationSet
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty slice",
			input:    []*entity.EvaluationSet{},
			expected: []*eval_set.EvaluationSet{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := EvaluationSetDO2DTOs(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluationSetDO2DTO_Simple(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *entity.EvaluationSet
		expected *eval_set.EvaluationSet
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "minimal evaluation set",
			input: &entity.EvaluationSet{
				ID:      1,
				AppID:   1,
				SpaceID: 1,
				Name:    "Test Set",
			},
			expected: &eval_set.EvaluationSet{
				ID:                gptr.Of(int64(1)),
				AppID:             gptr.Of(int32(1)),
				WorkspaceID:       gptr.Of(int64(1)),
				Name:              gptr.Of("Test Set"),
				Description:       gptr.Of(""),
				Status:            gptr.Of(dataset.DatasetStatus(0)),
				ItemCount:         gptr.Of(int64(0)),
				ChangeUncommitted: gptr.Of(false),
				LatestVersion:     gptr.Of(""),
				NextVersionNum:    gptr.Of(int64(0)),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := EvaluationSetDO2DTO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFieldWriteOptionDTO2DOs(t *testing.T) {
	t.Parallel()

	assert.Nil(t, FieldWriteOptionDTO2DOs(nil))
	assert.Nil(t, FieldWriteOptionDTO2DO(nil))

	strategy := dataset.MultiModalStoreStrategy("store")
	contentType := dataset.ContentType_Image
	fieldName := "field"
	fieldKey := "key"
	dto := &dataset.FieldWriteOption{
		FieldName: &fieldName,
		FieldKey:  &fieldKey,
		MultiModalStoreOpt: &dataset.MultiModalStoreOption{
			MultiModalStoreStrategy: &strategy,
			ContentType:             &contentType,
		},
	}

	got := FieldWriteOptionDTO2DO(dto)
	if assert.NotNil(t, got) {
		assert.Equal(t, &fieldName, got.FieldName)
		assert.Equal(t, &fieldKey, got.FieldKey)
		assert.NotNil(t, got.MultiModalStoreOpt)
		assert.Equal(t, entity.MultiModalStoreStrategyStore, *got.MultiModalStoreOpt.MultiModalStoreStrategy)
		assert.Equal(t, entity.ContentTypeImage, *got.MultiModalStoreOpt.ContentType)
	}
	assert.Len(t, FieldWriteOptionDTO2DOs([]*dataset.FieldWriteOption{dto}), 1)
}

func TestMultiModalStoreOptionDTO2DO(t *testing.T) {
	t.Parallel()

	assert.Nil(t, MultiModalStoreOptionDTO2DO(nil))

	strategy := dataset.MultiModalStoreStrategy("passthrough")
	tests := []struct {
		name        string
		contentType *dataset.ContentType
		expected    *entity.ContentType
	}{
		{"text", gptr.Of(dataset.ContentType_Text), gptr.Of(entity.ContentTypeText)},
		{"image", gptr.Of(dataset.ContentType_Image), gptr.Of(entity.ContentTypeImage)},
		{"audio", gptr.Of(dataset.ContentType_Audio), gptr.Of(entity.ContentTypeAudio)},
		{"video", gptr.Of(dataset.ContentType_Video), gptr.Of(entity.ContentTypeVideo)},
		{"multipart", gptr.Of(dataset.ContentType_MultiPart), gptr.Of(entity.ContentTypeMultipart)},
		{"unknown", gptr.Of(dataset.ContentType(999)), nil},
		{"nil", nil, nil},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := MultiModalStoreOptionDTO2DO(&dataset.MultiModalStoreOption{
				MultiModalStoreStrategy: &strategy,
				ContentType:             tt.contentType,
			})
			if assert.NotNil(t, got) {
				assert.NotNil(t, got.MultiModalStoreStrategy)
				assert.Equal(t, entity.MultiModalStoreStrategyPassthrough, *got.MultiModalStoreStrategy)
				assert.Equal(t, tt.expected, got.ContentType)
			}
		})
	}
}

func TestEvaluationSetDO2DTO_WithSpecAndFeatures(t *testing.T) {
	t.Parallel()

	do := &entity.EvaluationSet{
		ID:      1,
		AppID:   2,
		SpaceID: 3,
		Name:    "set",
		Spec: &entity.DatasetSpec{
			MaxItemCount:           11,
			MaxFieldCount:          12,
			MaxItemSize:            13,
			MaxItemDataNestedDepth: 14,
			MultiModalSpec: &entity.MultiModalSpec{
				MaxFileCount: 1,
			},
		},
		Features: &entity.DatasetFeatures{
			EditSchema:   true,
			RepeatedData: true,
			MultiModal:   true,
		},
		EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 4},
	}

	got := EvaluationSetDO2DTO(do)
	if assert.NotNil(t, got) {
		assert.Equal(t, int64(11), got.Spec.GetMaxItemCount())
		assert.Equal(t, int32(12), got.Spec.GetMaxFieldCount())
		assert.Equal(t, int64(13), got.Spec.GetMaxItemSize())
		assert.Equal(t, int32(14), got.Spec.GetMaxItemDataNestedDepth())
		assert.True(t, got.Features.GetEditSchema())
		assert.True(t, got.Features.GetRepeatedData())
		assert.True(t, got.Features.GetMultiModal())
		assert.NotNil(t, got.EvaluationSetVersion)
	}
}

func TestCreateDatasetItemOutputDO2DTO(t *testing.T) {
	t.Parallel()

	assert.Nil(t, CreateDatasetItemOutputDO2DTO(nil))

	itemIndex := int32(1)
	itemKey := "k"
	itemID := int64(2)
	isNewItem := true
	got := CreateDatasetItemOutputDO2DTO(&entity.DatasetItemOutput{
		ItemIndex: &itemIndex,
		ItemKey:   &itemKey,
		ItemID:    &itemID,
		IsNewItem: &isNewItem,
	})
	assert.Equal(t, &dataset.CreateDatasetItemOutput{
		ItemIndex: &itemIndex,
		ItemKey:   &itemKey,
		ItemID:    &itemID,
		IsNewItem: &isNewItem,
	}, got)
}
