// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluation_set

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	eval_common "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_set"
	app_eval_set "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_set"
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

func TestUploadAttachmentDetailsDO2DTOs(t *testing.T) {
	t.Parallel()

	assert.Nil(t, UploadAttachmentDetailsDO2DTOs(nil))

	contentType := entity.ContentTypeImage
	input := []*entity.UploadAttachmentDetail{{
		ContentType: &contentType,
	}}
	got := UploadAttachmentDetailsDO2DTOs(input)
	if assert.Len(t, got, 1) {
		assert.Equal(t, dataset.ContentType_Image, *got[0].ContentType)
	}
}

func TestUploadAttachmentDetailDO2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *entity.UploadAttachmentDetail
		expected *app_eval_set.UploadAttachmentDetail
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:  "text content type",
			input: &entity.UploadAttachmentDetail{ContentType: gptr.Of(entity.ContentTypeText)},
			expected: &app_eval_set.UploadAttachmentDetail{
				ContentType: gptr.Of(dataset.ContentType_Text),
				ErrorType:   gptr.Of(dataset.ItemErrorType(0)),
			},
		},
		{
			name:  "image content type",
			input: &entity.UploadAttachmentDetail{ContentType: gptr.Of(entity.ContentTypeImage)},
			expected: &app_eval_set.UploadAttachmentDetail{
				ContentType: gptr.Of(dataset.ContentType_Image),
				ErrorType:   gptr.Of(dataset.ItemErrorType(0)),
			},
		},
		{
			name:  "audio content type",
			input: &entity.UploadAttachmentDetail{ContentType: gptr.Of(entity.ContentTypeAudio)},
			expected: &app_eval_set.UploadAttachmentDetail{
				ContentType: gptr.Of(dataset.ContentType_Audio),
				ErrorType:   gptr.Of(dataset.ItemErrorType(0)),
			},
		},
		{
			name:  "video content type",
			input: &entity.UploadAttachmentDetail{ContentType: gptr.Of(entity.ContentTypeVideo)},
			expected: &app_eval_set.UploadAttachmentDetail{
				ContentType: gptr.Of(dataset.ContentType_Video),
				ErrorType:   gptr.Of(dataset.ItemErrorType(0)),
			},
		},
		{
			name:  "multipart content type",
			input: &entity.UploadAttachmentDetail{ContentType: gptr.Of(entity.ContentTypeMultipart)},
			expected: &app_eval_set.UploadAttachmentDetail{
				ContentType: gptr.Of(dataset.ContentType_MultiPart),
				ErrorType:   gptr.Of(dataset.ItemErrorType(0)),
			},
		},
		{
			name:  "unknown content type",
			input: &entity.UploadAttachmentDetail{ContentType: gptr.Of(entity.ContentType("unknown"))},
			expected: &app_eval_set.UploadAttachmentDetail{
				ErrorType: gptr.Of(dataset.ItemErrorType(0)),
			},
		},
		{
			name:  "nil content type",
			input: &entity.UploadAttachmentDetail{},
			expected: &app_eval_set.UploadAttachmentDetail{
				ErrorType: gptr.Of(dataset.ItemErrorType(0)),
			},
		},
		{
			name: "full fields",
			input: &entity.UploadAttachmentDetail{
				ContentType:     gptr.Of(entity.ContentTypeVideo),
				ImagexServiceID: gptr.Of("imagex-service-id"),
				OriginImage: &entity.Image{
					Name:            gptr.Of("origin-image-name"),
					URL:             gptr.Of("origin-image-url"),
					URI:             gptr.Of("origin-image-uri"),
					ThumbURL:        gptr.Of("origin-image-thumb"),
					StorageProvider: gptr.Of(entity.StorageProvider_TOS),
				},
				Image: &entity.Image{
					Name:            gptr.Of("image-name"),
					URL:             gptr.Of("image-url"),
					URI:             gptr.Of("image-uri"),
					ThumbURL:        gptr.Of("image-thumb"),
					StorageProvider: gptr.Of(entity.StorageProvider_VETOS),
				},
				OriginAudio: &entity.Audio{
					Format:          gptr.Of("mp3"),
					URL:             gptr.Of("origin-audio-url"),
					Name:            gptr.Of("origin-audio-name"),
					URI:             gptr.Of("origin-audio-uri"),
					StorageProvider: gptr.Of(entity.StorageProvider_TOS),
				},
				Audio: &entity.Audio{
					Format:          gptr.Of("wav"),
					URL:             gptr.Of("audio-url"),
					Name:            gptr.Of("audio-name"),
					URI:             gptr.Of("audio-uri"),
					StorageProvider: gptr.Of(entity.StorageProvider_VETOS),
				},
				OriginVideo: &entity.Video{
					Name:            gptr.Of("origin-video-name"),
					URL:             gptr.Of("origin-video-url"),
					URI:             gptr.Of("origin-video-uri"),
					ThumbURL:        gptr.Of("origin-video-thumb"),
					StorageProvider: gptr.Of(entity.StorageProvider_TOS),
				},
				Video: &entity.Video{
					Name:            gptr.Of("video-name"),
					URL:             gptr.Of("video-url"),
					URI:             gptr.Of("video-uri"),
					ThumbURL:        gptr.Of("video-thumb"),
					StorageProvider: gptr.Of(entity.StorageProvider_VETOS),
				},
				ErrMsg:    gptr.Of("upload failed"),
				ErrorType: gptr.Of(entity.ItemErrorType_UploadImageFailed),
			},
			expected: &app_eval_set.UploadAttachmentDetail{
				ContentType:     gptr.Of(dataset.ContentType_Video),
				ImagexServiceID: gptr.Of("imagex-service-id"),
				OriginImage: &eval_common.Image{
					Name:            gptr.Of("origin-image-name"),
					URL:             gptr.Of("origin-image-url"),
					URI:             gptr.Of("origin-image-uri"),
					ThumbURL:        gptr.Of("origin-image-thumb"),
					StorageProvider: gptr.Of(dataset.StorageProvider_TOS),
				},
				Image: &eval_common.Image{
					Name:            gptr.Of("image-name"),
					URL:             gptr.Of("image-url"),
					URI:             gptr.Of("image-uri"),
					ThumbURL:        gptr.Of("image-thumb"),
					StorageProvider: gptr.Of(dataset.StorageProvider_VETOS),
				},
				OriginAudio: &eval_common.Audio{
					Format:          gptr.Of("mp3"),
					URL:             gptr.Of("origin-audio-url"),
					Name:            gptr.Of("origin-audio-name"),
					URI:             gptr.Of("origin-audio-uri"),
					StorageProvider: gptr.Of(dataset.StorageProvider_TOS),
				},
				Audio: &eval_common.Audio{
					Format:          gptr.Of("wav"),
					URL:             gptr.Of("audio-url"),
					Name:            gptr.Of("audio-name"),
					URI:             gptr.Of("audio-uri"),
					StorageProvider: gptr.Of(dataset.StorageProvider_VETOS),
				},
				OriginVideo: &eval_common.Video{
					Name:            gptr.Of("origin-video-name"),
					URL:             gptr.Of("origin-video-url"),
					URI:             gptr.Of("origin-video-uri"),
					ThumbURL:        gptr.Of("origin-video-thumb"),
					StorageProvider: gptr.Of(dataset.StorageProvider_TOS),
				},
				Video: &eval_common.Video{
					Name:            gptr.Of("video-name"),
					URL:             gptr.Of("video-url"),
					URI:             gptr.Of("video-uri"),
					ThumbURL:        gptr.Of("video-thumb"),
					StorageProvider: gptr.Of(dataset.StorageProvider_VETOS),
				},
				ErrMsg:    gptr.Of("upload failed"),
				ErrorType: gptr.Of(dataset.ItemErrorType_UploadImageFailed),
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, UploadAttachmentDetailDO2DTO(tt.input))
		})
	}
}
