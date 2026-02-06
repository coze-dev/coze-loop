// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	commondto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	commonentity "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestConvertContentTypeDTO2DO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected commonentity.ContentType
	}{
		{
			name:     "text content type",
			input:    "text",
			expected: commonentity.ContentType("text"),
		},
		{
			name:     "image content type",
			input:    "image",
			expected: commonentity.ContentType("image"),
		},
		{
			name:     "empty string",
			input:    "",
			expected: commonentity.ContentType(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertContentTypeDTO2DO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertContentTypeDO2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    commonentity.ContentType
		expected string
	}{
		{
			name:     "text content type",
			input:    commonentity.ContentTypeText,
			expected: "Text",
		},
		{
			name:     "image content type",
			input:    commonentity.ContentTypeImage,
			expected: "Image",
		},
		{
			name:     "empty content type",
			input:    commonentity.ContentType(""),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertContentTypeDO2DTO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertImageDTO2DO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *commondto.Image
		expected *commonentity.Image
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "complete image",
			input: &commondto.Image{
				Name:            gptr.Of("test.jpg"),
				URL:             gptr.Of("https://example.com/test.jpg"),
				URI:             gptr.Of("uri://test"),
				ThumbURL:        gptr.Of("https://example.com/thumb.jpg"),
				StorageProvider: gptr.Of(dataset.StorageProvider(1)),
			},
			expected: &commonentity.Image{
				Name:            gptr.Of("test.jpg"),
				URL:             gptr.Of("https://example.com/test.jpg"),
				URI:             gptr.Of("uri://test"),
				ThumbURL:        gptr.Of("https://example.com/thumb.jpg"),
				StorageProvider: gptr.Of(commonentity.StorageProvider(1)),
			},
		},
		{
			name: "minimal image",
			input: &commondto.Image{
				Name: gptr.Of("minimal.jpg"),
			},
			expected: &commonentity.Image{
				Name: gptr.Of("minimal.jpg"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertImageDTO2DO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertImageDO2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *commonentity.Image
		expected *commondto.Image
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "complete image",
			input: &commonentity.Image{
				Name:            gptr.Of("test.jpg"),
				URL:             gptr.Of("https://example.com/test.jpg"),
				URI:             gptr.Of("uri://test"),
				ThumbURL:        gptr.Of("https://example.com/thumb.jpg"),
				StorageProvider: gptr.Of(commonentity.StorageProvider_S3),
			},
			expected: &commondto.Image{
				Name:            gptr.Of("test.jpg"),
				URL:             gptr.Of("https://example.com/test.jpg"),
				URI:             gptr.Of("uri://test"),
				ThumbURL:        gptr.Of("https://example.com/thumb.jpg"),
				StorageProvider: gptr.Of(dataset.StorageProvider(commonentity.StorageProvider_S3)),
			},
		},
		{
			name: "minimal image",
			input: &commonentity.Image{
				Name: gptr.Of("minimal.jpg"),
			},
			expected: &commondto.Image{
				Name: gptr.Of("minimal.jpg"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertImageDO2DTO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertAudioDTO2DO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *commondto.Audio
		expected *commonentity.Audio
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "complete audio",
			input: &commondto.Audio{
				Format:          gptr.Of("mp3"),
				URL:             gptr.Of("https://example.com/audio.mp3"),
				Name:            gptr.Of("audio_test.mp3"),
				URI:             gptr.Of("example_dir/audio.mp3"),
				StorageProvider: gptr.Of(dataset.StorageProvider_ImageX),
			},
			expected: &commonentity.Audio{
				Format:          gptr.Of("mp3"),
				URL:             gptr.Of("https://example.com/audio.mp3"),
				Name:            gptr.Of("audio_test.mp3"),
				URI:             gptr.Of("example_dir/audio.mp3"),
				StorageProvider: gptr.Of(commonentity.StorageProvider_ImageX),
			},
		},
		{
			name: "minimal audio",
			input: &commondto.Audio{
				Format: gptr.Of("wav"),
			},
			expected: &commonentity.Audio{
				Format: gptr.Of("wav"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertAudioDTO2DO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertAudioDO2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *commonentity.Audio
		expected *commondto.Audio
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "complete audio",
			input: &commonentity.Audio{
				Format:          gptr.Of("mp3"),
				URL:             gptr.Of("https://example.com/audio.mp3"),
				Name:            gptr.Of("audio_test.mp3"),
				URI:             gptr.Of("example_dir/audio.mp3"),
				StorageProvider: gptr.Of(commonentity.StorageProvider_ImageX),
			},
			expected: &commondto.Audio{
				Format:          gptr.Of("mp3"),
				URL:             gptr.Of("https://example.com/audio.mp3"),
				Name:            gptr.Of("audio_test.mp3"),
				URI:             gptr.Of("example_dir/audio.mp3"),
				StorageProvider: gptr.Of(dataset.StorageProvider(commonentity.StorageProvider_ImageX)),
			},
		},
		{
			name: "minimal audio",
			input: &commonentity.Audio{
				Format: gptr.Of("wav"),
			},
			expected: &commondto.Audio{
				Format: gptr.Of("wav"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertAudioDO2DTO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertVideoDTO2DO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *commondto.Video
		expected *commonentity.Video
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "complete video",
			input: &commondto.Video{
				Name:            gptr.Of("test.mp4"),
				URL:             gptr.Of("https://example.com/test.mp4"),
				URI:             gptr.Of("uri://test.mp4"),
				ThumbURL:        gptr.Of("https://example.com/thumb.mp4"),
				StorageProvider: gptr.Of(dataset.StorageProvider_ImageX),
			},
			expected: &commonentity.Video{
				Name:            gptr.Of("test.mp4"),
				URL:             gptr.Of("https://example.com/test.mp4"),
				URI:             gptr.Of("uri://test.mp4"),
				ThumbURL:        gptr.Of("https://example.com/thumb.mp4"),
				StorageProvider: gptr.Of(commonentity.StorageProvider_ImageX),
			},
		},
		{
			name: "minimal video",
			input: &commondto.Video{
				Name: gptr.Of("minimal.mp4"),
			},
			expected: &commonentity.Video{
				Name: gptr.Of("minimal.mp4"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertVideoDTO2DO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertVideoDO2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *commonentity.Video
		expected *commondto.Video
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "complete video",
			input: &commonentity.Video{
				Name:            gptr.Of("test.mp4"),
				URL:             gptr.Of("https://example.com/test.mp4"),
				URI:             gptr.Of("uri://test.mp4"),
				ThumbURL:        gptr.Of("https://example.com/thumb.mp4"),
				StorageProvider: gptr.Of(commonentity.StorageProvider_S3),
			},
			expected: &commondto.Video{
				Name:            gptr.Of("test.mp4"),
				URL:             gptr.Of("https://example.com/test.mp4"),
				URI:             gptr.Of("uri://test.mp4"),
				ThumbURL:        gptr.Of("https://example.com/thumb.mp4"),
				StorageProvider: gptr.Of(dataset.StorageProvider(commonentity.StorageProvider_S3)),
			},
		},
		{
			name: "minimal video",
			input: &commonentity.Video{
				Name: gptr.Of("minimal.mp4"),
			},
			expected: &commondto.Video{
				Name: gptr.Of("minimal.mp4"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertVideoDO2DTO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertContentDTO2DO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *commondto.Content
		expected *commonentity.Content
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "text content",
			input: &commondto.Content{
				ContentType: gptr.Of("text"),
				Text:        gptr.Of("Hello World"),
			},
			expected: &commonentity.Content{
				ContentType: gptr.Of(commonentity.ContentType("text")),
				Text:        gptr.Of("Hello World"),
			},
		},
		{
			name: "image content",
			input: &commondto.Content{
				ContentType: gptr.Of("image"),
				Image: &commondto.Image{
					Name: gptr.Of("test.jpg"),
					URL:  gptr.Of("https://example.com/test.jpg"),
				},
			},
			expected: &commonentity.Content{
				ContentType: gptr.Of(commonentity.ContentType("image")),
				Image: &commonentity.Image{
					Name: gptr.Of("test.jpg"),
					URL:  gptr.Of("https://example.com/test.jpg"),
				},
			},
		},
		{
			name: "multipart content",
			input: &commondto.Content{
				ContentType: gptr.Of("multipart"),
				MultiPart: []*commondto.Content{
					{
						ContentType: gptr.Of("text"),
						Text:        gptr.Of("Part 1"),
					},
					{
						ContentType: gptr.Of("text"),
						Text:        gptr.Of("Part 2"),
					},
				},
			},
			expected: &commonentity.Content{
				ContentType: gptr.Of(commonentity.ContentType("multipart")),
				MultiPart: []*commonentity.Content{
					{
						ContentType: gptr.Of(commonentity.ContentType("text")),
						Text:        gptr.Of("Part 1"),
					},
					{
						ContentType: gptr.Of(commonentity.ContentType("text")),
						Text:        gptr.Of("Part 2"),
					},
				},
			},
		},
		{
			name: "audio content",
			input: &commondto.Content{
				ContentType: gptr.Of("audio"),
				Audio: &commondto.Audio{
					Format: gptr.Of("mp3"),
					URL:    gptr.Of("https://example.com/audio.mp3"),
				},
			},
			expected: &commonentity.Content{
				ContentType: gptr.Of(commonentity.ContentType("audio")),
				Audio: &commonentity.Audio{
					Format: gptr.Of("mp3"),
					URL:    gptr.Of("https://example.com/audio.mp3"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertContentDTO2DO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertContentDO2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *commonentity.Content
		expected *commondto.Content
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "text content",
			input: &commonentity.Content{
				ContentType: gptr.Of(commonentity.ContentType("text")),
				Text:        gptr.Of("Hello World"),
			},
			expected: &commondto.Content{
				ContentType: gptr.Of("text"),
				Text:        gptr.Of("Hello World"),
			},
		},
		{
			name: "image content",
			input: &commonentity.Content{
				ContentType: gptr.Of(commonentity.ContentType("image")),
				Image: &commonentity.Image{
					Name: gptr.Of("test.jpg"),
					URL:  gptr.Of("https://example.com/test.jpg"),
				},
			},
			expected: &commondto.Content{
				ContentType: gptr.Of("image"),
				Image: &commondto.Image{
					Name: gptr.Of("test.jpg"),
					URL:  gptr.Of("https://example.com/test.jpg"),
				},
			},
		},
		{
			name: "multipart content",
			input: &commonentity.Content{
				ContentType: gptr.Of(commonentity.ContentType("multipart")),
				MultiPart: []*commonentity.Content{
					{
						ContentType: gptr.Of(commonentity.ContentType("text")),
						Text:        gptr.Of("Part 1"),
					},
					{
						ContentType: gptr.Of(commonentity.ContentType("text")),
						Text:        gptr.Of("Part 2"),
					},
				},
			},
			expected: &commondto.Content{
				ContentType: gptr.Of("multipart"),
				MultiPart: []*commondto.Content{
					{
						ContentType: gptr.Of("text"),
						Text:        gptr.Of("Part 1"),
					},
					{
						ContentType: gptr.Of("text"),
						Text:        gptr.Of("Part 2"),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertContentDO2DTO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertOrderByDTO2DO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *commondto.OrderBy
		expected *commonentity.OrderBy
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "ascending order",
			input: &commondto.OrderBy{
				Field: gptr.Of("name"),
				IsAsc: gptr.Of(true),
			},
			expected: &commonentity.OrderBy{
				Field: gptr.Of("name"),
				IsAsc: gptr.Of(true),
			},
		},
		{
			name: "descending order",
			input: &commondto.OrderBy{
				Field: gptr.Of("created_at"),
				IsAsc: gptr.Of(false),
			},
			expected: &commonentity.OrderBy{
				Field: gptr.Of("created_at"),
				IsAsc: gptr.Of(false),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertOrderByDTO2DO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertOrderByDTO2DOs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []*commondto.OrderBy
		expected []*commonentity.OrderBy
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty slice",
			input:    []*commondto.OrderBy{},
			expected: []*commonentity.OrderBy{},
		},
		{
			name: "multiple orders",
			input: []*commondto.OrderBy{
				{
					Field: gptr.Of("name"),
					IsAsc: gptr.Of(true),
				},
				{
					Field: gptr.Of("created_at"),
					IsAsc: gptr.Of(false),
				},
			},
			expected: []*commonentity.OrderBy{
				{
					Field: gptr.Of("name"),
					IsAsc: gptr.Of(true),
				},
				{
					Field: gptr.Of("created_at"),
					IsAsc: gptr.Of(false),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertOrderByDTO2DOs(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertOrderByDO2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *commonentity.OrderBy
		expected *commondto.OrderBy
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "ascending order",
			input: &commonentity.OrderBy{
				Field: gptr.Of("name"),
				IsAsc: gptr.Of(true),
			},
			expected: &commondto.OrderBy{
				Field: gptr.Of("name"),
				IsAsc: gptr.Of(true),
			},
		},
		{
			name: "descending order",
			input: &commonentity.OrderBy{
				Field: gptr.Of("created_at"),
				IsAsc: gptr.Of(false),
			},
			expected: &commondto.OrderBy{
				Field: gptr.Of("created_at"),
				IsAsc: gptr.Of(false),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertOrderByDO2DTO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertRoleDTO2DO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    int64
		expected commonentity.Role
	}{
		{
			name:     "system role",
			input:    1,
			expected: commonentity.Role(1),
		},
		{
			name:     "user role",
			input:    2,
			expected: commonentity.Role(2),
		},
		{
			name:     "assistant role",
			input:    3,
			expected: commonentity.Role(3),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertRoleDTO2DO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertRoleDO2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    commonentity.Role
		expected int64
	}{
		{
			name:     "system role",
			input:    commonentity.RoleSystem,
			expected: int64(commonentity.RoleSystem),
		},
		{
			name:     "user role",
			input:    commonentity.RoleUser,
			expected: int64(commonentity.RoleUser),
		},
		{
			name:     "assistant role",
			input:    commonentity.RoleAssistant,
			expected: int64(commonentity.RoleAssistant),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertRoleDO2DTO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertMessageDTO2DO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *commondto.Message
		expected *commonentity.Message
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "complete message",
			input: &commondto.Message{
				Role: gptr.Of(commondto.Role(commonentity.RoleUser)),
				Content: &commondto.Content{
					ContentType: gptr.Of("text"),
					Text:        gptr.Of("Hello"),
				},
				Ext: map[string]string{"key": "value"},
			},
			expected: &commonentity.Message{
				Role: commonentity.RoleUser,
				Content: &commonentity.Content{
					ContentType: gptr.Of(commonentity.ContentType("text")),
					Text:        gptr.Of("Hello"),
				},
				Ext: map[string]string{"key": "value"},
			},
		},
		{
			name: "message without role",
			input: &commondto.Message{
				Content: &commondto.Content{
					ContentType: gptr.Of("text"),
					Text:        gptr.Of("Hello"),
				},
			},
			expected: &commonentity.Message{
				Role: commonentity.Role(0),
				Content: &commonentity.Content{
					ContentType: gptr.Of(commonentity.ContentType("text")),
					Text:        gptr.Of("Hello"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertMessageDTO2DO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertMessageDO2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *commonentity.Message
		expected *commondto.Message
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "complete message",
			input: &commonentity.Message{
				Role: commonentity.RoleUser,
				Content: &commonentity.Content{
					ContentType: gptr.Of(commonentity.ContentType("text")),
					Text:        gptr.Of("Hello"),
				},
				Ext: map[string]string{"key": "value"},
			},
			expected: &commondto.Message{
				Role: gptr.Of(commondto.Role(commonentity.RoleUser)),
				Content: &commondto.Content{
					ContentType: gptr.Of("text"),
					Text:        gptr.Of("Hello"),
				},
				Ext: map[string]string{"key": "value"},
			},
		},
		{
			name: "message with undefined role",
			input: &commonentity.Message{
				Role: commonentity.RoleUndefined,
				Content: &commonentity.Content{
					ContentType: gptr.Of(commonentity.ContentType("text")),
					Text:        gptr.Of("Hello"),
				},
			},
			expected: &commondto.Message{
				Role: nil,
				Content: &commondto.Content{
					ContentType: gptr.Of("text"),
					Text:        gptr.Of("Hello"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertMessageDO2DTO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertArgsSchemaDTO2DO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *commondto.ArgsSchema
		expected *commonentity.ArgsSchema
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "complete args schema",
			input: &commondto.ArgsSchema{
				Key:                 gptr.Of("test_key"),
				SupportContentTypes: []string{"text", "image"},
				JSONSchema:          gptr.Of(`{"type": "object"}`),
			},
			expected: &commonentity.ArgsSchema{
				Key: gptr.Of("test_key"),
				SupportContentTypes: []commonentity.ContentType{
					commonentity.ContentType("text"),
					commonentity.ContentType("image"),
				},
				JsonSchema: gptr.Of(`{"type": "object"}`),
			},
		},
		{
			name: "empty content types",
			input: &commondto.ArgsSchema{
				Key:                 gptr.Of("test_key"),
				SupportContentTypes: []string{},
				JSONSchema:          gptr.Of(`{"type": "object"}`),
			},
			expected: &commonentity.ArgsSchema{
				Key:                 gptr.Of("test_key"),
				SupportContentTypes: []commonentity.ContentType{},
				JsonSchema:          gptr.Of(`{"type": "object"}`),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertArgsSchemaDTO2DO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertArgsSchemaDO2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *commonentity.ArgsSchema
		expected *commondto.ArgsSchema
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "complete args schema",
			input: &commonentity.ArgsSchema{
				Key: gptr.Of("test_key"),
				SupportContentTypes: []commonentity.ContentType{
					commonentity.ContentType("text"),
					commonentity.ContentType("image"),
				},
				JsonSchema: gptr.Of(`{"type": "object"}`),
			},
			expected: &commondto.ArgsSchema{
				Key:                 gptr.Of("test_key"),
				SupportContentTypes: []string{"text", "image"},
				JSONSchema:          gptr.Of(`{"type": "object"}`),
			},
		},
		{
			name: "empty content types",
			input: &commonentity.ArgsSchema{
				Key:                 gptr.Of("test_key"),
				SupportContentTypes: []commonentity.ContentType{},
				JsonSchema:          gptr.Of(`{"type": "object"}`),
			},
			expected: &commondto.ArgsSchema{
				Key:                 gptr.Of("test_key"),
				SupportContentTypes: []string{},
				JSONSchema:          gptr.Of(`{"type": "object"}`),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertArgsSchemaDO2DTO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertUserInfoDTO2DO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *commondto.UserInfo
		expected *commonentity.UserInfo
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "complete user info",
			input: &commondto.UserInfo{
				Name:        gptr.Of("John Doe"),
				EnName:      gptr.Of("john.doe"),
				AvatarURL:   gptr.Of("https://example.com/avatar.jpg"),
				AvatarThumb: gptr.Of("https://example.com/thumb.jpg"),
				OpenID:      gptr.Of("open123"),
				UnionID:     gptr.Of("union456"),
				UserID:      gptr.Of("user789"),
				Email:       gptr.Of("john@example.com"),
			},
			expected: &commonentity.UserInfo{
				Name:        gptr.Of("John Doe"),
				EnName:      gptr.Of("john.doe"),
				AvatarURL:   gptr.Of("https://example.com/avatar.jpg"),
				AvatarThumb: gptr.Of("https://example.com/thumb.jpg"),
				OpenID:      gptr.Of("open123"),
				UnionID:     gptr.Of("union456"),
				UserID:      gptr.Of("user789"),
				Email:       gptr.Of("john@example.com"),
			},
		},
		{
			name: "minimal user info",
			input: &commondto.UserInfo{
				UserID: gptr.Of("user123"),
			},
			expected: &commonentity.UserInfo{
				UserID: gptr.Of("user123"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertUserInfoDTO2DO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertUserInfoDO2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *commonentity.UserInfo
		expected *commondto.UserInfo
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "complete user info",
			input: &commonentity.UserInfo{
				Name:        gptr.Of("John Doe"),
				EnName:      gptr.Of("john.doe"),
				AvatarURL:   gptr.Of("https://example.com/avatar.jpg"),
				AvatarThumb: gptr.Of("https://example.com/thumb.jpg"),
				OpenID:      gptr.Of("open123"),
				UnionID:     gptr.Of("union456"),
				UserID:      gptr.Of("user789"),
				Email:       gptr.Of("john@example.com"),
			},
			expected: &commondto.UserInfo{
				Name:        gptr.Of("John Doe"),
				EnName:      gptr.Of("john.doe"),
				AvatarURL:   gptr.Of("https://example.com/avatar.jpg"),
				AvatarThumb: gptr.Of("https://example.com/thumb.jpg"),
				OpenID:      gptr.Of("open123"),
				UnionID:     gptr.Of("union456"),
				UserID:      gptr.Of("user789"),
				Email:       gptr.Of("john@example.com"),
			},
		},
		{
			name: "minimal user info",
			input: &commonentity.UserInfo{
				UserID: gptr.Of("user123"),
			},
			expected: &commondto.UserInfo{
				UserID: gptr.Of("user123"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertUserInfoDO2DTO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertBaseInfoDTO2DO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *commondto.BaseInfo
		expected *commonentity.BaseInfo
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "complete base info",
			input: &commondto.BaseInfo{
				CreatedBy: &commondto.UserInfo{
					UserID: gptr.Of("creator123"),
					Name:   gptr.Of("Creator"),
				},
				UpdatedBy: &commondto.UserInfo{
					UserID: gptr.Of("updater456"),
					Name:   gptr.Of("Updater"),
				},
				CreatedAt: gptr.Of(int64(1640995200)),
				UpdatedAt: gptr.Of(int64(1640995300)),
				DeletedAt: gptr.Of(int64(1640995400)),
			},
			expected: &commonentity.BaseInfo{
				CreatedBy: &commonentity.UserInfo{
					UserID: gptr.Of("creator123"),
					Name:   gptr.Of("Creator"),
				},
				UpdatedBy: &commonentity.UserInfo{
					UserID: gptr.Of("updater456"),
					Name:   gptr.Of("Updater"),
				},
				CreatedAt: gptr.Of(int64(1640995200)),
				UpdatedAt: gptr.Of(int64(1640995300)),
				DeletedAt: gptr.Of(int64(1640995400)),
			},
		},
		{
			name: "minimal base info",
			input: &commondto.BaseInfo{
				CreatedAt: gptr.Of(int64(1640995200)),
			},
			expected: &commonentity.BaseInfo{
				CreatedAt: gptr.Of(int64(1640995200)),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertBaseInfoDTO2DO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertBaseInfoDO2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *commonentity.BaseInfo
		expected *commondto.BaseInfo
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "complete base info",
			input: &commonentity.BaseInfo{
				CreatedBy: &commonentity.UserInfo{
					UserID: gptr.Of("creator123"),
					Name:   gptr.Of("Creator"),
				},
				UpdatedBy: &commonentity.UserInfo{
					UserID: gptr.Of("updater456"),
					Name:   gptr.Of("Updater"),
				},
				CreatedAt: gptr.Of(int64(1640995200)),
				UpdatedAt: gptr.Of(int64(1640995300)),
				DeletedAt: gptr.Of(int64(1640995400)),
			},
			expected: &commondto.BaseInfo{
				CreatedBy: &commondto.UserInfo{
					UserID: gptr.Of("creator123"),
					Name:   gptr.Of("Creator"),
				},
				UpdatedBy: &commondto.UserInfo{
					UserID: gptr.Of("updater456"),
					Name:   gptr.Of("Updater"),
				},
				CreatedAt: gptr.Of(int64(1640995200)),
				UpdatedAt: gptr.Of(int64(1640995300)),
				DeletedAt: gptr.Of(int64(1640995400)),
			},
		},
		{
			name: "minimal base info",
			input: &commonentity.BaseInfo{
				CreatedAt: gptr.Of(int64(1640995200)),
			},
			expected: &commondto.BaseInfo{
				CreatedAt: gptr.Of(int64(1640995200)),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertBaseInfoDO2DTO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertModelConfigDTO2DO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *commondto.ModelConfig
		expected *commonentity.ModelConfig
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "complete model config",
			input: &commondto.ModelConfig{
				ModelID:     gptr.Of(int64(123)),
				ModelName:   gptr.Of("gpt-4"),
				Temperature: gptr.Of(0.7),
				MaxTokens:   gptr.Of(int32(2048)),
				TopP:        gptr.Of(0.9),
			},
			expected: &commonentity.ModelConfig{
				ModelID:     gptr.Of(int64(123)),
				ModelName:   "gpt-4",
				Temperature: gptr.Of(0.7),
				MaxTokens:   gptr.Of(int32(2048)),
				TopP:        gptr.Of(0.9),
			},
		},
		{
			name: "minimal model config",
			input: &commondto.ModelConfig{
				ModelID: gptr.Of(int64(456)),
			},
			expected: &commonentity.ModelConfig{
				ModelID: gptr.Of(int64(456)),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertModelConfigDTO2DO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertModelConfigDO2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *commonentity.ModelConfig
		expected *commondto.ModelConfig
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "complete model config with model ID",
			input: &commonentity.ModelConfig{
				ModelID:     gptr.Of(int64(123)),
				ModelName:   "gpt-4",
				Temperature: gptr.Of(0.7),
				MaxTokens:   gptr.Of(int32(2048)),
				TopP:        gptr.Of(0.9),
			},
			expected: &commondto.ModelConfig{
				ModelID:     gptr.Of(int64(123)),
				ModelName:   gptr.Of("gpt-4"),
				Temperature: gptr.Of(0.7),
				MaxTokens:   gptr.Of(int32(2048)),
				TopP:        gptr.Of(0.9),
			},
		},
		{
			name: "model config with provider model ID",
			input: &commonentity.ModelConfig{
				ModelID:         gptr.Of(int64(0)),
				ProviderModelID: gptr.Of("456"),
				ModelName:       "claude-3",
				Temperature:     gptr.Of(0.5),
			},
			expected: &commondto.ModelConfig{
				ModelID:     gptr.Of(int64(456)),
				ModelName:   gptr.Of("claude-3"),
				Temperature: gptr.Of(0.5),
			},
		},
		{
			name: "model config with invalid provider model ID",
			input: &commonentity.ModelConfig{
				ModelID:         gptr.Of(int64(0)),
				ProviderModelID: gptr.Of("invalid"),
				ModelName:       "claude-3",
			},
			expected: &commondto.ModelConfig{
				ModelID:   gptr.Of(int64(0)),
				ModelName: gptr.Of("claude-3"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertModelConfigDO2DTO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertFieldDisplayFormatDTO2DO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    int64
		expected commonentity.FieldDisplayFormat
	}{
		{
			name:     "text format",
			input:    1,
			expected: commonentity.FieldDisplayFormat(1),
		},
		{
			name:     "json format",
			input:    2,
			expected: commonentity.FieldDisplayFormat(2),
		},
		{
			name:     "zero format",
			input:    0,
			expected: commonentity.FieldDisplayFormat(0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertFieldDisplayFormatDTO2DO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertFieldDisplayFormatDO2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    commonentity.FieldDisplayFormat
		expected int64
	}{
		{
			name:     "text format",
			input:    commonentity.FieldDisplayFormat(1),
			expected: 1,
		},
		{
			name:     "json format",
			input:    commonentity.FieldDisplayFormat(2),
			expected: 2,
		},
		{
			name:     "zero format",
			input:    commonentity.FieldDisplayFormat(0),
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertFieldDisplayFormatDO2DTO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOpenAPIUserInfoDO2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *commonentity.UserInfo
		expected *commondto.UserInfo
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "complete user info",
			input: &commonentity.UserInfo{
				Name:      gptr.Of("Alice"),
				AvatarURL: gptr.Of("https://example.com/alice.png"),
				UserID:    gptr.Of("user_1"),
				Email:     gptr.Of("alice@example.com"),
			},
			expected: &commondto.UserInfo{
				Name:      gptr.Of("Alice"),
				AvatarURL: gptr.Of("https://example.com/alice.png"),
				UserID:    gptr.Of("user_1"),
				Email:     gptr.Of("alice@example.com"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := OpenAPIUserInfoDO2DTO(tt.input)
			if tt.expected == nil {
				assert.Nil(t, result)
				return
			}
			if assert.NotNil(t, result) {
				assert.Equal(t, gptr.Indirect(tt.expected.Name), gptr.Indirect(result.Name))
				assert.Equal(t, gptr.Indirect(tt.expected.AvatarURL), gptr.Indirect(result.AvatarURL))
				assert.Equal(t, gptr.Indirect(tt.expected.UserID), gptr.Indirect(result.UserID))
				assert.Equal(t, gptr.Indirect(tt.expected.Email), gptr.Indirect(result.Email))
			}
		})
	}
}

func TestOpenAPIBaseInfoDO2DTO(t *testing.T) {
	t.Parallel()

	createdAt := int64(1700000000)
	updatedAt := int64(1700000100)

	t.Run("nil input", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, OpenAPIBaseInfoDO2DTO(nil))
	})

	t.Run("complete base info", func(t *testing.T) {
		t.Parallel()
		input := &commonentity.BaseInfo{
			CreatedBy: &commonentity.UserInfo{UserID: gptr.Of("creator"), Name: gptr.Of("Creator")},
			UpdatedBy: &commonentity.UserInfo{UserID: gptr.Of("updater"), Name: gptr.Of("Updater")},
			CreatedAt: &createdAt,
			UpdatedAt: &updatedAt,
		}

		result := OpenAPIBaseInfoDO2DTO(input)
		if assert.NotNil(t, result) {
			assert.Equal(t, gptr.Indirect(input.CreatedAt), gptr.Indirect(result.CreatedAt))
			assert.Equal(t, gptr.Indirect(input.UpdatedAt), gptr.Indirect(result.UpdatedAt))
			if assert.NotNil(t, result.CreatedBy) {
				assert.Equal(t, gptr.Indirect(input.CreatedBy.UserID), gptr.Indirect(result.CreatedBy.UserID))
				assert.Equal(t, gptr.Indirect(input.CreatedBy.Name), gptr.Indirect(result.CreatedBy.Name))
			}
			if assert.NotNil(t, result.UpdatedBy) {
				assert.Equal(t, gptr.Indirect(input.UpdatedBy.UserID), gptr.Indirect(result.UpdatedBy.UserID))
				assert.Equal(t, gptr.Indirect(input.UpdatedBy.Name), gptr.Indirect(result.UpdatedBy.Name))
			}
		}
	})
}

func TestConvertRateLimitDO2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *commonentity.RateLimit
		expected *commondto.RateLimit
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "complete rate limit with period",
			input: &commonentity.RateLimit{
				Rate:   gptr.Of(int32(100)),
				Burst:  gptr.Of(int32(200)),
				Period: gptr.Of(time.Minute * 5),
			},
			expected: &commondto.RateLimit{
				Rate:   gptr.Of(int32(100)),
				Burst:  gptr.Of(int32(200)),
				Period: gptr.Of("5m0s"),
			},
		},
		{
			name: "rate limit without period",
			input: &commonentity.RateLimit{
				Rate:  gptr.Of(int32(50)),
				Burst: gptr.Of(int32(100)),
			},
			expected: &commondto.RateLimit{
				Rate:   gptr.Of(int32(50)),
				Burst:  gptr.Of(int32(100)),
				Period: nil,
			},
		},
		{
			name: "rate limit with second period",
			input: &commonentity.RateLimit{
				Rate:   gptr.Of(int32(10)),
				Burst:  gptr.Of(int32(20)),
				Period: gptr.Of(time.Second * 30),
			},
			expected: &commondto.RateLimit{
				Rate:   gptr.Of(int32(10)),
				Burst:  gptr.Of(int32(20)),
				Period: gptr.Of("30s"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertRateLimitDO2DTO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertRateLimitDTO2DO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   *commondto.RateLimit
		want    *commonentity.RateLimit
		wantErr bool
	}{
		{
			name:    "nil input",
			input:   nil,
			want:    nil,
			wantErr: false,
		},
		{
			name: "valid rate limit with period",
			input: &commondto.RateLimit{
				Rate:   gptr.Of(int32(100)),
				Burst:  gptr.Of(int32(200)),
				Period: gptr.Of("5m"),
			},
			want: &commonentity.RateLimit{
				Rate:   gptr.Of(int32(100)),
				Burst:  gptr.Of(int32(200)),
				Period: gptr.Of(time.Minute * 5),
			},
			wantErr: false,
		},
		{
			name: "rate limit without period",
			input: &commondto.RateLimit{
				Rate:  gptr.Of(int32(50)),
				Burst: gptr.Of(int32(100)),
			},
			want: &commonentity.RateLimit{
				Rate:   gptr.Of(int32(50)),
				Burst:  gptr.Of(int32(100)),
				Period: nil,
			},
			wantErr: false,
		},
		{
			name: "rate limit with second period",
			input: &commondto.RateLimit{
				Rate:   gptr.Of(int32(10)),
				Burst:  gptr.Of(int32(20)),
				Period: gptr.Of("30s"),
			},
			want: &commonentity.RateLimit{
				Rate:   gptr.Of(int32(10)),
				Burst:  gptr.Of(int32(20)),
				Period: gptr.Of(time.Second * 30),
			},
			wantErr: false,
		},
		{
			name: "rate limit with hour period",
			input: &commondto.RateLimit{
				Rate:   gptr.Of(int32(1000)),
				Burst:  gptr.Of(int32(2000)),
				Period: gptr.Of("1h"),
			},
			want: &commonentity.RateLimit{
				Rate:   gptr.Of(int32(1000)),
				Burst:  gptr.Of(int32(2000)),
				Period: gptr.Of(time.Hour),
			},
			wantErr: false,
		},
		{
			name: "invalid period format",
			input: &commondto.RateLimit{
				Rate:   gptr.Of(int32(100)),
				Burst:  gptr.Of(int32(200)),
				Period: gptr.Of("invalid"),
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ConvertRateLimitDTO2DO(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestConvertRuntimeParamDTO2DO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *commondto.RuntimeParam
		expected *commonentity.RuntimeParam
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "json value and demo",
			input: &commondto.RuntimeParam{
				JSONValue: gptr.Of(`{"model_config":{"model_id":"m-1"}}`),
				JSONDemo:  gptr.Of(`{"model_config":{"model_id":"demo"}}`),
			},
			expected: &commonentity.RuntimeParam{
				JSONValue: gptr.Of(`{"model_config":{"model_id":"m-1"}}`),
				JSONDemo:  gptr.Of(`{"model_config":{"model_id":"demo"}}`),
			},
		},
		{
			name: "empty fields",
			input: &commondto.RuntimeParam{
				JSONValue: nil,
				JSONDemo:  nil,
			},
			expected: &commonentity.RuntimeParam{
				JSONValue: nil,
				JSONDemo:  nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertRuntimeParamDTO2DO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertRuntimeParamDO2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *commonentity.RuntimeParam
		expected *commondto.RuntimeParam
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "json value and demo",
			input: &commonentity.RuntimeParam{
				JSONValue: gptr.Of(`{"model_config":{"temperature":0.8}}`),
				JSONDemo:  gptr.Of(`{"model_config":{"temperature":0.5}}`),
			},
			expected: &commondto.RuntimeParam{
				JSONValue: gptr.Of(`{"model_config":{"temperature":0.8}}`),
				JSONDemo:  gptr.Of(`{"model_config":{"temperature":0.5}}`),
			},
		},
		{
			name: "empty fields",
			input: &commonentity.RuntimeParam{
				JSONValue: nil,
				JSONDemo:  nil,
			},
			expected: &commondto.RuntimeParam{
				JSONValue: nil,
				JSONDemo:  nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertRuntimeParamDO2DTO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
