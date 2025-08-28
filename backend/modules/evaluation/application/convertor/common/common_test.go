// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

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
				Name:            gptr.Of("minimal.jpg"),
				StorageProvider: gptr.Of(commonentity.StorageProvider(0)),
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
				Name:            gptr.Of("minimal.jpg"),
				StorageProvider: gptr.Of(dataset.StorageProvider(0)),
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
				Format: gptr.Of("mp3"),
				URL:    gptr.Of("https://example.com/audio.mp3"),
			},
			expected: &commonentity.Audio{
				Format: gptr.Of("mp3"),
				URL:    gptr.Of("https://example.com/audio.mp3"),
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
				Format: gptr.Of("mp3"),
				URL:    gptr.Of("https://example.com/audio.mp3"),
			},
			expected: &commondto.Audio{
				Format: gptr.Of("mp3"),
				URL:    gptr.Of("https://example.com/audio.mp3"),
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
					Name:            gptr.Of("test.jpg"),
					URL:             gptr.Of("https://example.com/test.jpg"),
					StorageProvider: gptr.Of(commonentity.StorageProvider(0)),
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
					Name:            gptr.Of("test.jpg"),
					URL:             gptr.Of("https://example.com/test.jpg"),
					StorageProvider: gptr.Of(dataset.StorageProvider(0)),
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
				ModelID:     123,
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
				ModelID: 456,
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
				ModelID:     123,
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
				ModelID:         0,
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
				ModelID:         0,
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
