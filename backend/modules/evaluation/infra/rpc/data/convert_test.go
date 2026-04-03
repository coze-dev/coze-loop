// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package data

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset_job"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestConvert2EvaluationSetFieldData(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		input    *dataset.FieldData
		expected *entity.FieldData
	}{
		{
			name:     "nil_input",
			input:    nil,
			expected: nil,
		},
		{
			name: "empty_field_data",
			input: &dataset.FieldData{
				Key:  gptr.Of(""),
				Name: gptr.Of(""),
			},
			expected: &entity.FieldData{
				Key:  "",
				Name: "",
				Content: &entity.Content{
					ContentType: gptr.Of(entity.ContentType("<UNSET>")),
					Format:      gptr.Of(entity.FieldDisplayFormat(0)),
					Text:        nil,
					Image:       nil,
					Audio:       nil,
					MultiPart:   nil,
				},
			},
		},
		{
			name: "basic_key_name",
			input: &dataset.FieldData{
				Key:  gptr.Of("test_key"),
				Name: gptr.Of("Test Name"),
			},
			expected: &entity.FieldData{
				Key:  "test_key",
				Name: "Test Name",
				Content: &entity.Content{
					ContentType: gptr.Of(entity.ContentType("<UNSET>")),
					Format:      gptr.Of(entity.FieldDisplayFormat(0)),
					Text:        nil,
					Image:       nil,
					Audio:       nil,
					MultiPart:   nil,
				},
			},
		},
		{
			name: "text_content",
			input: &dataset.FieldData{
				Key:         gptr.Of("text_key"),
				Name:        gptr.Of("Text Field"),
				ContentType: gptr.Of(dataset.ContentType_Text),
				Format:      gptr.Of(dataset.FieldDisplayFormat_PlainText),
				Content:     gptr.Of("Hello, World!"),
			},
			expected: &entity.FieldData{
				Key:  "text_key",
				Name: "Text Field",
				Content: &entity.Content{
					ContentType: gptr.Of(entity.ContentType("Text")),
					Format:      gptr.Of(entity.FieldDisplayFormat(1)),
					Text:        gptr.Of("Hello, World!"),
					Image:       nil,
					Audio:       nil,
					MultiPart:   nil,
				},
			},
		},
		{
			name: "with_content_type_and_format",
			input: &dataset.FieldData{
				Key:         gptr.Of("formatted_key"),
				Name:        gptr.Of("Formatted Field"),
				ContentType: gptr.Of(dataset.ContentType_Text),
				Format:      gptr.Of(dataset.FieldDisplayFormat_Markdown),
				Content:     gptr.Of("# Markdown Content"),
			},
			expected: &entity.FieldData{
				Key:  "formatted_key",
				Name: "Formatted Field",
				Content: &entity.Content{
					ContentType: gptr.Of(entity.ContentType("Text")),
					Format:      gptr.Of(entity.FieldDisplayFormat(2)),
					Text:        gptr.Of("# Markdown Content"),
					Image:       nil,
					Audio:       nil,
					MultiPart:   nil,
				},
			},
		},
		{
			name: "image_attachment",
			input: &dataset.FieldData{
				Key:         gptr.Of("image_key"),
				Name:        gptr.Of("Image Field"),
				ContentType: gptr.Of(dataset.ContentType_Image),
				Format:      gptr.Of(dataset.FieldDisplayFormat_PlainText),
				Content:     gptr.Of("Image description"),
				Attachments: []*dataset.ObjectStorage{
					{
						Name:     gptr.Of("test.jpg"),
						URL:      gptr.Of("https://example.com/test.jpg"),
						URI:      gptr.Of("tos://bucket/test.jpg"),
						ThumbURL: gptr.Of("https://example.com/test_thumb.jpg"),
						Provider: gptr.Of(dataset.StorageProvider_TOS),
					},
				},
			},
			expected: &entity.FieldData{
				Key:  "image_key",
				Name: "Image Field",
				Content: &entity.Content{
					ContentType: gptr.Of(entity.ContentType("Image")),
					Format:      gptr.Of(entity.FieldDisplayFormat(1)),
					Text:        gptr.Of("Image description"),
					Image: &entity.Image{
						Name:            gptr.Of("test.jpg"),
						URL:             gptr.Of("https://example.com/test.jpg"),
						URI:             gptr.Of("tos://bucket/test.jpg"),
						ThumbURL:        gptr.Of("https://example.com/test_thumb.jpg"),
						StorageProvider: gptr.Of(entity.StorageProvider_TOS),
					},
					Audio:     nil,
					MultiPart: nil,
				},
			},
		},
		{
			name: "audio_attachment",
			input: &dataset.FieldData{
				Key:         gptr.Of("audio_key"),
				Name:        gptr.Of("Audio Field"),
				ContentType: gptr.Of(dataset.ContentType_Audio),
				Format:      gptr.Of(dataset.FieldDisplayFormat_PlainText),
				Content:     gptr.Of("Audio description"),
				Attachments: []*dataset.ObjectStorage{
					{
						Name: gptr.Of("test.mp3"),
						URL:  gptr.Of("https://example.com/test.mp3"),
					},
				},
			},
			expected: &entity.FieldData{
				Key:  "audio_key",
				Name: "Audio Field",
				Content: &entity.Content{
					ContentType: gptr.Of(entity.ContentType("Audio")),
					Format:      gptr.Of(entity.FieldDisplayFormat(1)),
					Text:        gptr.Of("Audio description"),
					Image:       nil,
					Audio: &entity.Audio{
						Name:   gptr.Of("test.mp3"),
						Format: gptr.Of("mp3"),
						URL:    gptr.Of("https://example.com/test.mp3"),
					},
					MultiPart: nil,
				},
			},
		},
		{
			name: "mixed_attachments",
			input: &dataset.FieldData{
				Key:         gptr.Of("mixed_key"),
				Name:        gptr.Of("Mixed Field"),
				ContentType: gptr.Of(dataset.ContentType_MultiPart),
				Format:      gptr.Of(dataset.FieldDisplayFormat_PlainText),
				Content:     gptr.Of("Mixed content"),
				Attachments: []*dataset.ObjectStorage{
					{
						Name:     gptr.Of("image.png"),
						URL:      gptr.Of("https://example.com/image.png"),
						Provider: gptr.Of(dataset.StorageProvider_ImageX),
					},
					{
						Name: gptr.Of("audio.wav"),
						URL:  gptr.Of("https://example.com/audio.wav"),
					},
				},
			},
			expected: &entity.FieldData{
				Key:  "mixed_key",
				Name: "Mixed Field",
				Content: &entity.Content{
					ContentType: gptr.Of(entity.ContentType("MultiPart")),
					Format:      gptr.Of(entity.FieldDisplayFormat(1)),
					Text:        gptr.Of("Mixed content"),
					Image: &entity.Image{
						Name:            gptr.Of("image.png"),
						URL:             gptr.Of("https://example.com/image.png"),
						StorageProvider: gptr.Of(entity.StorageProvider_ImageX),
					},
					Audio: &entity.Audio{
						Name:   gptr.Of("audio.wav"),
						Format: gptr.Of("wav"),
						URL:    gptr.Of("https://example.com/audio.wav"),
					},
					MultiPart: nil,
				},
			},
		},
		{
			name: "single_part",
			input: &dataset.FieldData{
				Key:         gptr.Of("part_key"),
				Name:        gptr.Of("Part Field"),
				ContentType: gptr.Of(dataset.ContentType_MultiPart),
				Format:      gptr.Of(dataset.FieldDisplayFormat_PlainText),
				Content:     gptr.Of("Main content"),
				Parts: []*dataset.FieldData{
					{
						Key:         gptr.Of("part1"),
						Name:        gptr.Of("Part 1"),
						ContentType: gptr.Of(dataset.ContentType_Text),
						Format:      gptr.Of(dataset.FieldDisplayFormat_PlainText),
						Content:     gptr.Of("Part 1 content"),
					},
				},
			},
			expected: &entity.FieldData{
				Key:  "part_key",
				Name: "Part Field",
				Content: &entity.Content{
					ContentType: gptr.Of(entity.ContentType("MultiPart")),
					Format:      gptr.Of(entity.FieldDisplayFormat(1)),
					Text:        gptr.Of("Main content"),
					Image:       nil,
					Audio:       nil,
					MultiPart: []*entity.Content{
						{
							ContentType: gptr.Of(entity.ContentType("Text")),
							Format:      gptr.Of(entity.FieldDisplayFormat(1)),
							Text:        gptr.Of("Part 1 content"),
							Image:       nil,
							Audio:       nil,
							MultiPart:   nil,
						},
					},
				},
			},
		},
		{
			name: "multiple_parts",
			input: &dataset.FieldData{
				Key:         gptr.Of("multi_part_key"),
				Name:        gptr.Of("Multi Part Field"),
				ContentType: gptr.Of(dataset.ContentType_MultiPart),
				Format:      gptr.Of(dataset.FieldDisplayFormat_JSON),
				Content:     gptr.Of("Main content"),
				Parts: []*dataset.FieldData{
					{
						Key:         gptr.Of("part1"),
						Name:        gptr.Of("Part 1"),
						ContentType: gptr.Of(dataset.ContentType_Text),
						Format:      gptr.Of(dataset.FieldDisplayFormat_PlainText),
						Content:     gptr.Of("Part 1 content"),
					},
					{
						Key:         gptr.Of("part2"),
						Name:        gptr.Of("Part 2"),
						ContentType: gptr.Of(dataset.ContentType_Image),
						Format:      gptr.Of(dataset.FieldDisplayFormat_PlainText),
						Content:     gptr.Of("Part 2 content"),
						Attachments: []*dataset.ObjectStorage{
							{
								Name: gptr.Of("part2.jpg"),
								URL:  gptr.Of("https://example.com/part2.jpg"),
							},
						},
					},
				},
			},
			expected: &entity.FieldData{
				Key:  "multi_part_key",
				Name: "Multi Part Field",
				Content: &entity.Content{
					ContentType: gptr.Of(entity.ContentType("MultiPart")),
					Format:      gptr.Of(entity.FieldDisplayFormat(3)),
					Text:        gptr.Of("Main content"),
					Image:       nil,
					Audio:       nil,
					MultiPart: []*entity.Content{
						{
							ContentType: gptr.Of(entity.ContentType("Text")),
							Format:      gptr.Of(entity.FieldDisplayFormat(1)),
							Text:        gptr.Of("Part 1 content"),
							Image:       nil,
							Audio:       nil,
							MultiPart:   nil,
						},
						{
							ContentType: gptr.Of(entity.ContentType("Image")),
							Format:      gptr.Of(entity.FieldDisplayFormat(1)),
							Text:        gptr.Of("Part 2 content"),
							Image: &entity.Image{
								Name: gptr.Of("part2.jpg"),
								URL:  gptr.Of("https://example.com/part2.jpg"),
							},
							Audio:     nil,
							MultiPart: nil,
						},
					},
				},
			},
		},
		{
			name: "nested_parts",
			input: &dataset.FieldData{
				Key:         gptr.Of("nested_key"),
				Name:        gptr.Of("Nested Field"),
				ContentType: gptr.Of(dataset.ContentType_MultiPart),
				Format:      gptr.Of(dataset.FieldDisplayFormat_PlainText),
				Content:     gptr.Of("Root content"),
				Parts: []*dataset.FieldData{
					{
						Key:         gptr.Of("level1"),
						Name:        gptr.Of("Level 1"),
						ContentType: gptr.Of(dataset.ContentType_MultiPart),
						Format:      gptr.Of(dataset.FieldDisplayFormat_PlainText),
						Content:     gptr.Of("Level 1 content"),
						Parts: []*dataset.FieldData{
							{
								Key:         gptr.Of("level2"),
								Name:        gptr.Of("Level 2"),
								ContentType: gptr.Of(dataset.ContentType_Text),
								Format:      gptr.Of(dataset.FieldDisplayFormat_PlainText),
								Content:     gptr.Of("Level 2 content"),
							},
						},
					},
				},
			},
			expected: &entity.FieldData{
				Key:  "nested_key",
				Name: "Nested Field",
				Content: &entity.Content{
					ContentType: gptr.Of(entity.ContentType("MultiPart")),
					Format:      gptr.Of(entity.FieldDisplayFormat(1)),
					Text:        gptr.Of("Root content"),
					Image:       nil,
					Audio:       nil,
					MultiPart: []*entity.Content{
						{
							ContentType: gptr.Of(entity.ContentType("MultiPart")),
							Format:      gptr.Of(entity.FieldDisplayFormat(1)),
							Text:        gptr.Of("Level 1 content"),
							Image:       nil,
							Audio:       nil,
							MultiPart: []*entity.Content{
								{
									ContentType: gptr.Of(entity.ContentType("Text")),
									Format:      gptr.Of(entity.FieldDisplayFormat(1)),
									Text:        gptr.Of("Level 2 content"),
									Image:       nil,
									Audio:       nil,
									MultiPart:   nil,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "complex_nested_with_multimedia",
			input: &dataset.FieldData{
				Key:         gptr.Of("complex_key"),
				Name:        gptr.Of("Complex Field"),
				ContentType: gptr.Of(dataset.ContentType_MultiPart),
				Format:      gptr.Of(dataset.FieldDisplayFormat_Markdown),
				Content:     gptr.Of("# Complex Content"),
				Attachments: []*dataset.ObjectStorage{
					{
						Name: gptr.Of("main.png"),
						URL:  gptr.Of("https://example.com/main.png"),
					},
				},
				Parts: []*dataset.FieldData{
					{
						Key:         gptr.Of("text_part"),
						Name:        gptr.Of("Text Part"),
						ContentType: gptr.Of(dataset.ContentType_Text),
						Format:      gptr.Of(dataset.FieldDisplayFormat_PlainText),
						Content:     gptr.Of("Text part content"),
					},
					{
						Key:         gptr.Of("media_part"),
						Name:        gptr.Of("Media Part"),
						ContentType: gptr.Of(dataset.ContentType_MultiPart),
						Format:      gptr.Of(dataset.FieldDisplayFormat_PlainText),
						Content:     gptr.Of("Media part content"),
						Attachments: []*dataset.ObjectStorage{
							{
								Name: gptr.Of("media.jpg"),
								URL:  gptr.Of("https://example.com/media.jpg"),
							},
							{
								Name: gptr.Of("sound.mp3"),
								URL:  gptr.Of("https://example.com/sound.mp3"),
							},
						},
						Parts: []*dataset.FieldData{
							{
								Key:         gptr.Of("nested_audio"),
								Name:        gptr.Of("Nested Audio"),
								ContentType: gptr.Of(dataset.ContentType_Audio),
								Format:      gptr.Of(dataset.FieldDisplayFormat_PlainText),
								Content:     gptr.Of("Nested audio content"),
								Attachments: []*dataset.ObjectStorage{
									{
										Name: gptr.Of("nested.wav"),
										URL:  gptr.Of("https://example.com/nested.wav"),
									},
								},
							},
						},
					},
				},
			},
			expected: &entity.FieldData{
				Key:  "complex_key",
				Name: "Complex Field",
				Content: &entity.Content{
					ContentType: gptr.Of(entity.ContentType("MultiPart")),
					Format:      gptr.Of(entity.FieldDisplayFormat(2)),
					Text:        gptr.Of("# Complex Content"),
					Image: &entity.Image{
						Name: gptr.Of("main.png"),
						URL:  gptr.Of("https://example.com/main.png"),
					},
					Audio: nil,
					MultiPart: []*entity.Content{
						{
							ContentType: gptr.Of(entity.ContentType("Text")),
							Format:      gptr.Of(entity.FieldDisplayFormat(1)),
							Text:        gptr.Of("Text part content"),
							Image:       nil,
							Audio:       nil,
							MultiPart:   nil,
						},
						{
							ContentType: gptr.Of(entity.ContentType("MultiPart")),
							Format:      gptr.Of(entity.FieldDisplayFormat(1)),
							Text:        gptr.Of("Media part content"),
							Image: &entity.Image{
								Name: gptr.Of("media.jpg"),
								URL:  gptr.Of("https://example.com/media.jpg"),
							},
							Audio: &entity.Audio{
								Name:   gptr.Of("sound.mp3"),
								Format: gptr.Of("mp3"),
								URL:    gptr.Of("https://example.com/sound.mp3"),
							},
							MultiPart: []*entity.Content{
								{
									ContentType: gptr.Of(entity.ContentType("Audio")),
									Format:      gptr.Of(entity.FieldDisplayFormat(1)),
									Text:        gptr.Of("Nested audio content"),
									Image:       nil,
									Audio: &entity.Audio{
										Name:   gptr.Of("nested.wav"),
										Format: gptr.Of("wav"),
										URL:    gptr.Of("https://example.com/nested.wav"),
									},
									MultiPart: nil,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "invalid_attachment_format",
			input: &dataset.FieldData{
				Key:         gptr.Of("invalid_key"),
				Name:        gptr.Of("Invalid Field"),
				ContentType: gptr.Of(dataset.ContentType_Text),
				Format:      gptr.Of(dataset.FieldDisplayFormat_PlainText),
				Content:     gptr.Of("Content with invalid attachment"),
				Attachments: []*dataset.ObjectStorage{
					{
						Name: gptr.Of("document.pdf"), // 不是图片或音频格式
						URL:  gptr.Of("https://example.com/document.pdf"),
					},
				},
			},
			expected: &entity.FieldData{
				Key:  "invalid_key",
				Name: "Invalid Field",
				Content: &entity.Content{
					ContentType: gptr.Of(entity.ContentType("Text")),
					Format:      gptr.Of(entity.FieldDisplayFormat(1)),
					Text:        gptr.Of("Content with invalid attachment"),
					Image:       nil, // 因为 PDF 不是图片格式
					Audio:       nil, // 因为 PDF 不是音频格式
					MultiPart:   nil,
				},
			},
		},
		{
			name: "empty_parts_array",
			input: &dataset.FieldData{
				Key:         gptr.Of("empty_parts_key"),
				Name:        gptr.Of("Empty Parts Field"),
				ContentType: gptr.Of(dataset.ContentType_MultiPart),
				Format:      gptr.Of(dataset.FieldDisplayFormat_PlainText),
				Content:     gptr.Of("Content with empty parts"),
				Parts:       []*dataset.FieldData{}, // 空的 Parts 数组
			},
			expected: &entity.FieldData{
				Key:  "empty_parts_key",
				Name: "Empty Parts Field",
				Content: &entity.Content{
					ContentType: gptr.Of(entity.ContentType("MultiPart")),
					Format:      gptr.Of(entity.FieldDisplayFormat(1)),
					Text:        gptr.Of("Content with empty parts"),
					Image:       nil,
					Audio:       nil,
					MultiPart:   nil, // 空数组会被转换为 nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convert2EvaluationSetFieldData(ctx, tt.input)
			assert.Equal(t, tt.expected, result, "convert2EvaluationSetFieldData() result mismatch")
		})
	}
}

// 辅助函数：创建嵌套的 Parts 结构
func createNestedParts(depth int) []*dataset.FieldData {
	if depth <= 0 {
		return nil
	}

	parts := []*dataset.FieldData{
		{
			Key:         gptr.Of("nested_part"),
			Name:        gptr.Of("Nested Part"),
			ContentType: gptr.Of(dataset.ContentType_Text),
			Format:      gptr.Of(dataset.FieldDisplayFormat_PlainText),
			Content:     gptr.Of("Nested content"),
		},
	}

	if depth > 1 {
		parts[0].Parts = createNestedParts(depth - 1)
	}

	return parts
}

// TestConvert2EvaluationSetFieldData_EdgeCases 测试边界情况
func TestConvert2EvaluationSetFieldData_EdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("nil_attachment_in_list", func(t *testing.T) {
		input := &dataset.FieldData{
			Key:         gptr.Of("nil_attachment_key"),
			Name:        gptr.Of("Nil Attachment Field"),
			ContentType: gptr.Of(dataset.ContentType_Image),
			Attachments: []*dataset.ObjectStorage{
				nil, // nil 附件
				{
					Name: gptr.Of("valid.jpg"),
					URL:  gptr.Of("https://example.com/valid.jpg"),
				},
			},
		}

		result := convert2EvaluationSetFieldData(ctx, input)
		assert.NotNil(t, result)
		assert.NotNil(t, result.Content.Image)
		assert.Equal(t, "valid.jpg", *result.Content.Image.Name)
	})

	t.Run("attachment_with_nil_name", func(t *testing.T) {
		input := &dataset.FieldData{
			Key:         gptr.Of("nil_name_key"),
			Name:        gptr.Of("Nil Name Field"),
			ContentType: gptr.Of(dataset.ContentType_Image),
			Attachments: []*dataset.ObjectStorage{
				{
					Name: nil, // nil 名称
					URL:  gptr.Of("https://example.com/unknown.jpg"),
				},
			},
		}

		result := convert2EvaluationSetFieldData(ctx, input)
		assert.NotNil(t, result)
		assert.Nil(t, result.Content.Image) // 因为名称为 nil，无法判断文件类型
	})

	t.Run("case_insensitive_extensions", func(t *testing.T) {
		input := &dataset.FieldData{
			Key:         gptr.Of("case_key"),
			Name:        gptr.Of("Case Field"),
			ContentType: gptr.Of(dataset.ContentType_Image),
			Attachments: []*dataset.ObjectStorage{
				{
					Name: gptr.Of("image.JPG"), // 大写扩展名
					URL:  gptr.Of("https://example.com/image.JPG"),
				},
			},
		}

		result := convert2EvaluationSetFieldData(ctx, input)
		assert.NotNil(t, result)
		assert.NotNil(t, result.Content.Image)
		assert.Equal(t, "image.JPG", *result.Content.Image.Name)
	})

	t.Run("deep_nesting", func(t *testing.T) {
		// 创建深度嵌套的结构
		input := &dataset.FieldData{
			Key:         gptr.Of("deep_key"),
			Name:        gptr.Of("Deep Field"),
			ContentType: gptr.Of(dataset.ContentType_MultiPart),
			Parts:       createNestedParts(5), // 5层嵌套
		}

		result := convert2EvaluationSetFieldData(ctx, input)
		assert.NotNil(t, result)
		assert.NotNil(t, result.Content.MultiPart)
		assert.Len(t, result.Content.MultiPart, 1)

		// 验证嵌套结构
		current := result.Content.MultiPart[0]
		depth := 1
		for current != nil && current.MultiPart != nil && len(current.MultiPart) > 0 {
			current = current.MultiPart[0]
			depth++
		}
		assert.Equal(t, 5, depth) // 应该有 5 层嵌套（因为最后一层没有 MultiPart）
	})
}

// TestConvert2EvaluationSetFieldData_RealWorldScenarios 测试真实业务场景
func TestConvert2EvaluationSetFieldData_RealWorldScenarios(t *testing.T) {
	ctx := context.Background()

	t.Run("conversation_with_mixed_content", func(t *testing.T) {
		// 模拟对话场景，包含文本、图片和音频
		input := &dataset.FieldData{
			Key:         gptr.Of("conversation"),
			Name:        gptr.Of("User Message"),
			ContentType: gptr.Of(dataset.ContentType_MultiPart),
			Format:      gptr.Of(dataset.FieldDisplayFormat_Markdown),
			Content:     gptr.Of("Please analyze this image and audio:"),
			Parts: []*dataset.FieldData{
				{
					Key:         gptr.Of("text_instruction"),
					Name:        gptr.Of("Text Instruction"),
					ContentType: gptr.Of(dataset.ContentType_Text),
					Format:      gptr.Of(dataset.FieldDisplayFormat_PlainText),
					Content:     gptr.Of("What do you see in this image?"),
				},
				{
					Key:         gptr.Of("uploaded_image"),
					Name:        gptr.Of("Uploaded Image"),
					ContentType: gptr.Of(dataset.ContentType_Image),
					Format:      gptr.Of(dataset.FieldDisplayFormat_PlainText),
					Content:     gptr.Of("User uploaded image"),
					Attachments: []*dataset.ObjectStorage{
						{
							Name:     gptr.Of("screenshot.png"),
							URL:      gptr.Of("https://cdn.example.com/screenshot.png"),
							URI:      gptr.Of("tos://bucket/user123/screenshot.png"),
							ThumbURL: gptr.Of("https://cdn.example.com/screenshot_thumb.png"),
							Provider: gptr.Of(dataset.StorageProvider_TOS),
						},
					},
				},
				{
					Key:         gptr.Of("voice_note"),
					Name:        gptr.Of("Voice Note"),
					ContentType: gptr.Of(dataset.ContentType_Audio),
					Format:      gptr.Of(dataset.FieldDisplayFormat_PlainText),
					Content:     gptr.Of("User voice note"),
					Attachments: []*dataset.ObjectStorage{
						{
							Name: gptr.Of("voice_note.m4a"),
							URL:  gptr.Of("https://cdn.example.com/voice_note.m4a"),
						},
					},
				},
			},
		}

		result := convert2EvaluationSetFieldData(ctx, input)
		assert.NotNil(t, result)
		assert.Equal(t, "conversation", result.Key)
		assert.Equal(t, "User Message", result.Name)
		assert.NotNil(t, result.Content)
		assert.Equal(t, entity.ContentType("MultiPart"), *result.Content.ContentType)
		assert.Equal(t, "Please analyze this image and audio:", *result.Content.Text)
		assert.Len(t, result.Content.MultiPart, 3)

		// 验证文本部分
		textPart := result.Content.MultiPart[0]
		assert.Equal(t, entity.ContentType("Text"), *textPart.ContentType)
		assert.Equal(t, "What do you see in this image?", *textPart.Text)
		assert.Nil(t, textPart.Image)
		assert.Nil(t, textPart.Audio)

		// 验证图片部分
		imagePart := result.Content.MultiPart[1]
		assert.Equal(t, entity.ContentType("Image"), *imagePart.ContentType)
		assert.Equal(t, "User uploaded image", *imagePart.Text)
		assert.NotNil(t, imagePart.Image)
		assert.Equal(t, "screenshot.png", *imagePart.Image.Name)
		assert.Equal(t, "https://cdn.example.com/screenshot.png", *imagePart.Image.URL)

		// 验证音频部分
		audioPart := result.Content.MultiPart[2]
		assert.Equal(t, entity.ContentType("Audio"), *audioPart.ContentType)
		assert.Equal(t, "User voice note", *audioPart.Text)
		assert.NotNil(t, audioPart.Audio)
		assert.Equal(t, "m4a", *audioPart.Audio.Format)
		assert.Equal(t, "https://cdn.example.com/voice_note.m4a", *audioPart.Audio.URL)
	})

	t.Run("code_review_scenario", func(t *testing.T) {
		// 模拟代码审查场景
		input := &dataset.FieldData{
			Key:         gptr.Of("code_review"),
			Name:        gptr.Of("Code Review Request"),
			ContentType: gptr.Of(dataset.ContentType_MultiPart),
			Format:      gptr.Of(dataset.FieldDisplayFormat_Markdown),
			Content:     gptr.Of("# Code Review Request\n\nPlease review the following changes:"),
			Parts: []*dataset.FieldData{
				{
					Key:         gptr.Of("code_diff"),
					Name:        gptr.Of("Code Diff"),
					ContentType: gptr.Of(dataset.ContentType_Text),
					Format:      gptr.Of(dataset.FieldDisplayFormat_Code),
					Content:     gptr.Of("```diff\n+ function newFeature() {\n+   return 'implemented';\n+ }\n```"),
				},
				{
					Key:         gptr.Of("test_results"),
					Name:        gptr.Of("Test Results"),
					ContentType: gptr.Of(dataset.ContentType_Text),
					Format:      gptr.Of(dataset.FieldDisplayFormat_JSON),
					Content:     gptr.Of(`{"passed": 15, "failed": 0, "coverage": "95%"}`),
				},
			},
		}

		result := convert2EvaluationSetFieldData(ctx, input)
		assert.NotNil(t, result)
		assert.Equal(t, "code_review", result.Key)
		assert.NotNil(t, result.Content)
		assert.Len(t, result.Content.MultiPart, 2)

		// 验证代码差异部分
		codePart := result.Content.MultiPart[0]
		assert.Equal(t, entity.ContentType("Text"), *codePart.ContentType)
		assert.Equal(t, entity.FieldDisplayFormat(5), *codePart.Format) // Code format
		assert.Contains(t, *codePart.Text, "function newFeature")

		// 验证测试结果部分
		testPart := result.Content.MultiPart[1]
		assert.Equal(t, entity.ContentType("Text"), *testPart.ContentType)
		assert.Equal(t, entity.FieldDisplayFormat(3), *testPart.Format) // JSON format
		assert.Contains(t, *testPart.Text, "coverage")
	})
}

func TestConvert2EvaluationSetTurn(t *testing.T) {
	ctx := context.Background()
	t.Run("nil_or_empty_data_returns_nil", func(t *testing.T) {
		// Data 为 nil
		item1 := &dataset.DatasetItem{
			ItemID:    gptr.Of(int64(100)),
			DatasetID: gptr.Of(int64(200)),
			Data:      nil,
		}
		turns1 := convert2EvaluationSetTurn(ctx, item1)
		assert.Nil(t, turns1)

		// Data 为空切片
		item2 := &dataset.DatasetItem{
			ItemID:    gptr.Of(int64(101)),
			DatasetID: gptr.Of(int64(201)),
			Data:      []*dataset.FieldData{},
		}
		turns2 := convert2EvaluationSetTurn(ctx, item2)
		assert.Nil(t, turns2)
	})

	t.Run("single_field_converts_to_single_turn", func(t *testing.T) {
		item := &dataset.DatasetItem{
			ItemID:    gptr.Of(int64(123)),
			DatasetID: gptr.Of(int64(456)),
			Data: []*dataset.FieldData{
				{
					Key:         gptr.Of("k1"),
					Name:        gptr.Of("n1"),
					ContentType: gptr.Of(dataset.ContentType_Text),
					Format:      gptr.Of(dataset.FieldDisplayFormat_PlainText),
					Content:     gptr.Of("hello"),
				},
			},
		}

		turns := convert2EvaluationSetTurn(ctx, item)
		assert.NotNil(t, turns)
		assert.Len(t, turns, 1)
		turn := turns[0]
		assert.Equal(t, int64(123), turn.ItemID)
		assert.Equal(t, int64(456), turn.EvalSetID)
		assert.Len(t, turn.FieldDataList, 1)
		fd := turn.FieldDataList[0]
		assert.Equal(t, "k1", fd.Key)
		assert.Equal(t, "n1", fd.Name)
		if assert.NotNil(t, fd.Content) {
			assert.Equal(t, entity.ContentType("Text"), *fd.Content.ContentType)
			assert.Equal(t, "hello", *fd.Content.Text)
		}
	})

	t.Run("multiple_fields_and_parts_are_preserved", func(t *testing.T) {
		item := &dataset.DatasetItem{
			ItemID:    gptr.Of(int64(321)),
			DatasetID: gptr.Of(int64(654)),
			Data: []*dataset.FieldData{
				{
					Key:         gptr.Of("text1"),
					Name:        gptr.Of("Text 1"),
					ContentType: gptr.Of(dataset.ContentType_Text),
					Format:      gptr.Of(dataset.FieldDisplayFormat_Markdown),
					Content:     gptr.Of("# title"),
				},
				{
					Key:         gptr.Of("mp1"),
					Name:        gptr.Of("MP 1"),
					ContentType: gptr.Of(dataset.ContentType_MultiPart),
					Format:      gptr.Of(dataset.FieldDisplayFormat_PlainText),
					Content:     gptr.Of("root"),
					Parts: []*dataset.FieldData{
						{
							Key:         gptr.Of("p-text"),
							Name:        gptr.Of("P Text"),
							ContentType: gptr.Of(dataset.ContentType_Text),
							Format:      gptr.Of(dataset.FieldDisplayFormat_PlainText),
							Content:     gptr.Of("child text"),
						},
					},
				},
			},
		}

		turns := convert2EvaluationSetTurn(ctx, item)
		assert.NotNil(t, turns)
		assert.Len(t, turns, 1)
		turn := turns[0]
		assert.Equal(t, int64(321), turn.ItemID)
		assert.Equal(t, int64(654), turn.EvalSetID)
		assert.Len(t, turn.FieldDataList, 2)

		// 验证第二个字段的 MultiPart 被保留
		fd2 := turn.FieldDataList[1]
		if assert.NotNil(t, fd2.Content) {
			assert.Equal(t, entity.ContentType("MultiPart"), *fd2.Content.ContentType)
			assert.NotNil(t, fd2.Content.MultiPart)
			assert.Len(t, fd2.Content.MultiPart, 1)
			assert.Equal(t, "child text", *fd2.Content.MultiPart[0].Text)
		}
	})
}

func TestToSchemaKey(t *testing.T) {
	tests := []struct {
		name     string
		input    *dataset.SchemaKey
		expected *entity.SchemaKey
	}{
		{"nil_input", nil, nil},
		{"string", dataset.SchemaKeyPtr(dataset.SchemaKey_String), gptr.Of(entity.SchemaKey_String)},
		{"integer", dataset.SchemaKeyPtr(dataset.SchemaKey_Integer), gptr.Of(entity.SchemaKey_Integer)},
		{"float", dataset.SchemaKeyPtr(dataset.SchemaKey_Float), gptr.Of(entity.SchemaKey_Float)},
		{"bool", dataset.SchemaKeyPtr(dataset.SchemaKey_Bool), gptr.Of(entity.SchemaKey_Bool)},
		{"message", dataset.SchemaKeyPtr(dataset.SchemaKey_Message), gptr.Of(entity.SchemaKey_Message)},
		{"single_choice", dataset.SchemaKeyPtr(dataset.SchemaKey_SingleChoice), gptr.Of(entity.SchemaKey_SingleChoice)},
		{"trajectory", dataset.SchemaKeyPtr(dataset.SchemaKey_Trajectory), gptr.Of(entity.SchemaKey_Trajectory)},
		{"unknown_value", dataset.SchemaKeyPtr(dataset.SchemaKey(999)), nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toSchemaKey(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestConvert2DatasetMultiModalSpec(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		input    *entity.MultiModalSpec
		expected *dataset.MultiModalSpec
	}{
		{
			name:     "nil_input",
			input:    nil,
			expected: nil,
		},
		{
			name: "normal_case",
			input: &entity.MultiModalSpec{
				MaxFileCount:     10,
				MaxFileSize:      1024,
				SupportedFormats: []string{"jpg", "png"},
				MaxPartCount:     5,
				MaxFileSizeByType: map[entity.ContentType]int64{
					entity.ContentTypeImage: 512,
				},
				SupportedFormatsByType: map[entity.ContentType][]string{
					entity.ContentTypeImage: {"jpg", "png"},
				},
			},
			expected: &dataset.MultiModalSpec{
				MaxFileCount:     gptr.Of(int64(10)),
				MaxFileSize:      gptr.Of(int64(1024)),
				SupportedFormats: []string{"jpg", "png"},
				MaxPartCount:     gptr.Of(int32(5)),
				MaxFileSizeByType: map[dataset.ContentType]int64{
					dataset.ContentType_Image: 512,
				},
				SupportedFormatsByType: map[dataset.ContentType][]string{
					dataset.ContentType_Image: {"jpg", "png"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convert2DatasetMultiModalSpec(ctx, tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestConvert2EvaluationSetMultiModalSpec(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		input    *dataset.MultiModalSpec
		expected *entity.MultiModalSpec
	}{
		{
			name:     "nil_input",
			input:    nil,
			expected: nil,
		},
		{
			name: "normal_case",
			input: &dataset.MultiModalSpec{
				MaxFileCount:     gptr.Of(int64(5)),
				MaxFileSize:      gptr.Of(int64(2048)),
				SupportedFormats: []string{"mp4", "mov"},
				MaxPartCount:     gptr.Of(int32(3)),
				MaxFileSizeByType: map[dataset.ContentType]int64{
					dataset.ContentType_Video: 1024,
				},
				SupportedFormatsByType: map[dataset.ContentType][]string{
					dataset.ContentType_Video: {"mp4", "mov"},
				},
			},
			expected: &entity.MultiModalSpec{
				MaxFileCount:     5,
				MaxFileSize:      2048,
				SupportedFormats: []string{"mp4", "mov"},
				MaxPartCount:     3,
				MaxFileSizeByType: map[entity.ContentType]int64{
					entity.ContentTypeVideo: 1024,
				},
				SupportedFormatsByType: map[entity.ContentType][]string{
					entity.ContentTypeVideo: {"mp4", "mov"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convert2EvaluationSetMultiModalSpec(ctx, tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestConvertObjectStorageToVideo(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		input    []*dataset.ObjectStorage
		expected *entity.Video
	}{
		{
			name:     "nil_attachments",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty_attachments",
			input:    []*dataset.ObjectStorage{},
			expected: nil,
		},
		{
			name: "valid_video_attachment",
			input: []*dataset.ObjectStorage{
				{
					Name:     gptr.Of("test.mp4"),
					URL:      gptr.Of("https://example.com/test.mp4"),
					URI:      gptr.Of("tos://bucket/test.mp4"),
					ThumbURL: gptr.Of("https://example.com/thumb.jpg"),
					Provider: gptr.Of(dataset.StorageProvider_TOS),
				},
			},
			expected: &entity.Video{
				Name:            gptr.Of("test.mp4"),
				URL:             gptr.Of("https://example.com/test.mp4"),
				URI:             gptr.Of("tos://bucket/test.mp4"),
				ThumbURL:        gptr.Of("https://example.com/thumb.jpg"),
				StorageProvider: gptr.Of(entity.StorageProvider_TOS),
			},
		},
		{
			name: "case_insensitive_extension",
			input: []*dataset.ObjectStorage{
				{
					Name: gptr.Of("MOVIE.AVI"),
					URL:  gptr.Of("https://example.com/movie.avi"),
				},
			},
			expected: &entity.Video{
				Name: gptr.Of("MOVIE.AVI"),
				URL:  gptr.Of("https://example.com/movie.avi"),
			},
		},
		{
			name: "invalid_extension",
			input: []*dataset.ObjectStorage{
				{
					Name: gptr.Of("notes.txt"),
					URL:  gptr.Of("https://example.com/notes.txt"),
				},
			},
			expected: nil,
		},
		{
			name: "mixed_attachments",
			input: []*dataset.ObjectStorage{
				{
					Name: gptr.Of("image.jpg"),
					URL:  gptr.Of("https://example.com/image.jpg"),
				},
				{
					Name: gptr.Of("video.mp4"),
					URL:  gptr.Of("https://example.com/video.mp4"),
				},
			},
			expected: &entity.Video{
				Name: gptr.Of("video.mp4"),
				URL:  gptr.Of("https://example.com/video.mp4"),
			},
		},
		{
			name: "nil_and_valid_attachments",
			input: []*dataset.ObjectStorage{
				nil,
				{
					Name: gptr.Of("valid.mp4"),
					URL:  gptr.Of("https://example.com/valid.mp4"),
				},
			},
			expected: &entity.Video{
				Name: gptr.Of("valid.mp4"),
				URL:  gptr.Of("https://example.com/valid.mp4"),
			},
		},
		{
			name: "attachment_with_nil_name",
			input: []*dataset.ObjectStorage{
				{
					Name: nil,
					URL:  gptr.Of("https://example.com/no-name"),
				},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertObjectStorageToVideo(ctx, tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestConvert2DatasetIOJob(t *testing.T) {
	t.Parallel()

	assert.Nil(t, convert2DatasetIOJob(context.Background(), nil))

	job := &dataset_job.DatasetIOJob{
		ID:        1,
		AppID:     gptr.Of(int32(2)),
		SpaceID:   3,
		DatasetID: 4,
		JobType:   dataset_job.JobType(1),
		Source: &dataset_job.DatasetIOEndpoint{
			File: &dataset_job.DatasetIOFile{
				Path: "path1",
			},
		},
		Target: &dataset_job.DatasetIOEndpoint{
			Dataset: &dataset_job.DatasetIODataset{
				DatasetID: 2,
			},
		},
		FieldMappings: []*dataset_job.FieldMapping{
			{Source: "s", Target: "t"},
		},
		Option: &dataset_job.DatasetIOJobOption{
			OverwriteDataset: gptr.Of(true),
		},
		Status: gptr.Of(dataset_job.JobStatus(1)),
		Progress: &dataset_job.DatasetIOJobProgress{
			Total: gptr.Of(int64(10)),
		},
		Errors: []*dataset.ItemErrorGroup{
			{Summary: gptr.Of("err")},
		},
		CreatedBy: gptr.Of("user1"),
		CreatedAt: gptr.Of(int64(100)),
		UpdatedBy: gptr.Of("user2"),
		UpdatedAt: gptr.Of(int64(200)),
		StartedAt: gptr.Of(int64(150)),
		EndedAt:   gptr.Of(int64(180)),
	}

	entityJob := convert2DatasetIOJob(context.Background(), job)
	assert.NotNil(t, entityJob)
	assert.Equal(t, int64(1), entityJob.ID)
	assert.Equal(t, gptr.Of(int32(2)), entityJob.AppID)
	assert.Equal(t, int64(3), entityJob.SpaceID)
	assert.Equal(t, int64(4), entityJob.DatasetID)
	assert.NotNil(t, entityJob.Source)
	assert.NotNil(t, entityJob.Target)
	assert.Len(t, entityJob.FieldMappings, 1)
	assert.NotNil(t, entityJob.Option)
	assert.NotNil(t, entityJob.Status)
	assert.NotNil(t, entityJob.Progress)
	assert.Len(t, entityJob.Errors, 1)
	assert.Equal(t, gptr.Of("user1"), entityJob.CreatedBy)
	assert.Equal(t, gptr.Of(int64(100)), entityJob.CreatedAt)
	assert.Equal(t, gptr.Of("user2"), entityJob.UpdatedBy)
	assert.Equal(t, gptr.Of(int64(200)), entityJob.UpdatedAt)
	assert.Equal(t, gptr.Of(int64(150)), entityJob.StartedAt)
	assert.Equal(t, gptr.Of(int64(180)), entityJob.EndedAt)
}

func TestConvert2DatasetIOFile(t *testing.T) {
	t.Parallel()

	assert.Nil(t, convert2DatasetIOFile(context.Background(), nil))

	file := &dataset_job.DatasetIOFile{
		Provider:         dataset.StorageProvider(1),
		Path:             "path",
		Format:           gptr.Of(dataset_job.FileFormat(1)),
		CompressFormat:   gptr.Of(dataset_job.FileFormat(2)),
		Files:            []string{"f1"},
		OriginalFileName: gptr.Of("name"),
		DownloadURL:      gptr.Of("url"),
		ProviderID:       gptr.Of("pid"),
		ProviderAuth: &dataset_job.ProviderAuth{
			ProviderAccountID: gptr.Of(int64(1)),
		},
	}

	entityFile := convert2DatasetIOFile(context.Background(), file)
	assert.NotNil(t, entityFile)
	assert.Equal(t, "path", entityFile.Path)
	assert.Len(t, entityFile.Files, 1)
	assert.Equal(t, gptr.Of("name"), entityFile.OriginalFileName)
	assert.Equal(t, gptr.Of("url"), entityFile.DownloadURL)
	assert.Equal(t, gptr.Of("pid"), entityFile.ProviderID)
	assert.NotNil(t, entityFile.ProviderAuth)
}

func TestConvert2DatasetIODataset(t *testing.T) {
	t.Parallel()

	assert.Nil(t, convert2DatasetIODataset(context.Background(), nil))

	ds := &dataset_job.DatasetIODataset{
		SpaceID:   gptr.Of(int64(1)),
		DatasetID: 2,
		VersionID: gptr.Of(int64(3)),
	}

	entityDs := convert2DatasetIODataset(context.Background(), ds)
	assert.NotNil(t, entityDs)
	assert.Equal(t, gptr.Of(int64(1)), entityDs.SpaceID)
	assert.Equal(t, int64(2), entityDs.DatasetID)
	assert.Equal(t, gptr.Of(int64(3)), entityDs.VersionID)
}

func TestConvert2FieldMappings(t *testing.T) {
	t.Parallel()

	assert.Nil(t, convert2FieldMappings(context.Background(), nil))
	assert.Nil(t, convert2FieldMappings(context.Background(), []*dataset_job.FieldMapping{}))

	mappings := []*dataset_job.FieldMapping{
		{Source: "s", Target: "t"},
	}

	entityMappings := convert2FieldMappings(context.Background(), mappings)
	assert.Len(t, entityMappings, 1)
	assert.Equal(t, "s", entityMappings[0].Source)
	assert.Equal(t, "t", entityMappings[0].Target)
}

func TestConvert2DatasetIOJobOption(t *testing.T) {
	t.Parallel()

	assert.Nil(t, convert2DatasetIOJobOption(context.Background(), nil))

	opt := &dataset_job.DatasetIOJobOption{
		OverwriteDataset: gptr.Of(true),
	}

	entityOpt := convert2DatasetIOJobOption(context.Background(), opt)
	assert.NotNil(t, entityOpt)
	assert.Equal(t, gptr.Of(true), entityOpt.OverwriteDataset)
}

func TestConvert2DatasetIOJobProgress(t *testing.T) {
	t.Parallel()

	assert.Nil(t, convert2DatasetIOJobProgress(context.Background(), nil))

	progress := &dataset_job.DatasetIOJobProgress{
		Total:     gptr.Of(int64(10)),
		Processed: gptr.Of(int64(5)),
		Added:     gptr.Of(int64(4)),
		Name:      gptr.Of("n"),
		SubProgresses: []*dataset_job.DatasetIOJobProgress{
			{Total: gptr.Of(int64(2))},
		},
	}

	entityProgress := convert2DatasetIOJobProgress(context.Background(), progress)
	assert.NotNil(t, entityProgress)
	assert.Equal(t, gptr.Of(int64(10)), entityProgress.Total)
	assert.Equal(t, gptr.Of(int64(5)), entityProgress.Processed)
	assert.Equal(t, gptr.Of(int64(4)), entityProgress.Added)
	assert.Equal(t, gptr.Of("n"), entityProgress.Name)
	assert.Len(t, entityProgress.SubProgresses, 1)
}

func TestConvert2DatasetIOJobSubProgresses(t *testing.T) {
	t.Parallel()

	assert.Nil(t, convert2DatasetIOJobSubProgresses(context.Background(), nil))
	assert.Nil(t, convert2DatasetIOJobSubProgresses(context.Background(), []*dataset_job.DatasetIOJobProgress{}))

	progresses := []*dataset_job.DatasetIOJobProgress{
		{Total: gptr.Of(int64(1))},
	}

	entityProgresses := convert2DatasetIOJobSubProgresses(context.Background(), progresses)
	assert.Len(t, entityProgresses, 1)
}

func TestConvert2ThriftDatasetIOFile(t *testing.T) {
	t.Parallel()

	assert.Nil(t, convert2ThriftDatasetIOFile(context.Background(), nil))

	file := &entity.DatasetIOFile{
		Provider:         entity.StorageProvider(1),
		Path:             "path",
		Format:           gptr.Of(entity.FileFormat(1)),
		CompressFormat:   gptr.Of(entity.FileFormat(2)),
		Files:            []string{"f1"},
		OriginalFileName: gptr.Of("name"),
		DownloadURL:      gptr.Of("url"),
		ProviderID:       gptr.Of("pid"),
		ProviderAuth: &entity.ProviderAuth{
			ProviderAccountID: gptr.Of(int64(1)),
		},
	}

	thriftFile := convert2ThriftDatasetIOFile(context.Background(), file)
	assert.NotNil(t, thriftFile)
	assert.Equal(t, "path", thriftFile.Path)
	assert.Len(t, thriftFile.Files, 1)
	assert.Equal(t, gptr.Of("name"), thriftFile.OriginalFileName)
	assert.Equal(t, gptr.Of("url"), thriftFile.DownloadURL)
	assert.Equal(t, gptr.Of("pid"), thriftFile.ProviderID)
	assert.NotNil(t, thriftFile.ProviderAuth)
}

func TestConvert2ThriftFieldMappings(t *testing.T) {
	t.Parallel()

	assert.Nil(t, convert2ThriftFieldMappings(context.Background(), nil))
	assert.Nil(t, convert2ThriftFieldMappings(context.Background(), []*entity.FieldMapping{}))

	mappings := []*entity.FieldMapping{
		{Source: "s", Target: "t"},
	}

	thriftMappings := convert2ThriftFieldMappings(context.Background(), mappings)
	assert.Len(t, thriftMappings, 1)
	assert.Equal(t, "s", thriftMappings[0].Source)
	assert.Equal(t, "t", thriftMappings[0].Target)
}

func TestConvert2ThriftDatasetIOJobOption(t *testing.T) {
	t.Parallel()

	assert.Nil(t, convert2ThriftDatasetIOJobOption(context.Background(), nil))

	opt := &entity.DatasetIOJobOption{
		OverwriteDataset: gptr.Of(true),
	}

	thriftOpt := convert2ThriftDatasetIOJobOption(context.Background(), opt)
	assert.NotNil(t, thriftOpt)
	assert.Equal(t, gptr.Of(true), thriftOpt.OverwriteDataset)
}

func TestConvert2DatasetMultiModalStoreOption(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	assert.Nil(t, convert2DatasetMultiModalStoreOption(ctx, nil))

	strategy := entity.MultiModalStoreStrategyStore
	tests := []struct {
		name        string
		contentType *entity.ContentType
		expected    *dataset.ContentType
	}{
		{"text", gptr.Of(entity.ContentTypeText), gptr.Of(dataset.ContentType_Text)},
		{"image", gptr.Of(entity.ContentTypeImage), gptr.Of(dataset.ContentType_Image)},
		{"audio", gptr.Of(entity.ContentTypeAudio), gptr.Of(dataset.ContentType_Audio)},
		{"video", gptr.Of(entity.ContentTypeVideo), gptr.Of(dataset.ContentType_Video)},
		{"multipart", gptr.Of(entity.ContentTypeMultipart), gptr.Of(dataset.ContentType_MultiPart)},
		{"unknown", gptr.Of(entity.ContentType("unknown")), nil},
		{"nil", nil, nil},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := convert2DatasetMultiModalStoreOption(ctx, &entity.MultiModalStoreOption{
				MultiModalStoreStrategy: &strategy,
				ContentType:             tt.contentType,
			})
			if assert.NotNil(t, got) {
				assert.NotNil(t, got.MultiModalStoreStrategy)
				assert.Equal(t, dataset.MultiModalStoreStrategyStore, *got.MultiModalStoreStrategy)
				assert.Equal(t, tt.expected, got.ContentType)
			}
		})
	}
}

func TestConvert2DatasetFieldWriteOptionAndOptions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	assert.Nil(t, convert2DatasetFieldWriteOption(ctx, nil))
	assert.Nil(t, convert2DatasetFieldWriteOptions(ctx, nil))
	assert.Nil(t, convert2DatasetFieldWriteOptions(ctx, []*entity.FieldWriteOption{}))

	fieldName := "field"
	fieldKey := "key"
	strategy := entity.MultiModalStoreStrategyPassthrough
	contentType := entity.ContentTypeImage
	opt := &entity.FieldWriteOption{
		FieldName: &fieldName,
		FieldKey:  &fieldKey,
		MultiModalStoreOpt: &entity.MultiModalStoreOption{
			MultiModalStoreStrategy: &strategy,
			ContentType:             &contentType,
		},
	}

	got := convert2DatasetFieldWriteOption(ctx, opt)
	if assert.NotNil(t, got) {
		assert.Equal(t, &fieldName, got.FieldName)
		assert.Equal(t, &fieldKey, got.FieldKey)
		assert.NotNil(t, got.MultiModalStoreOpt)
		assert.Equal(t, dataset.MultiModalStoreStrategyPassthrough, *got.MultiModalStoreOpt.MultiModalStoreStrategy)
		assert.Equal(t, dataset.ContentType_Image, *got.MultiModalStoreOpt.ContentType)
	}
	assert.Len(t, convert2DatasetFieldWriteOptions(ctx, []*entity.FieldWriteOption{opt}), 1)
}

func TestConvert2ThriftDatasetIOHelpers(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	assert.Nil(t, convert2ThriftDatasetIOFile(ctx, nil))
	assert.Nil(t, convert2ThriftProviderAuth(ctx, nil))
	assert.Nil(t, convert2ThriftFieldMappings(ctx, nil))
	assert.Nil(t, convert2ThriftFieldMappings(ctx, []*entity.FieldMapping{}))
	assert.Nil(t, convert2ThriftDatasetIOJobOption(ctx, nil))

	providerAccountID := int64(9)
	format := entity.FileFormat(1)
	compress := entity.FileFormat(2)
	file := &entity.DatasetIOFile{
		Provider:         entity.StorageProvider(3),
		Path:             "s3://bucket/file.csv",
		Format:           &format,
		CompressFormat:   &compress,
		Files:            []string{"a", "b"},
		OriginalFileName: gptr.Of("file.csv"),
		DownloadURL:      gptr.Of("https://x"),
		ProviderID:       gptr.Of("pid"),
		ProviderAuth:     &entity.ProviderAuth{ProviderAccountID: &providerAccountID},
	}
	gotFile := convert2ThriftDatasetIOFile(ctx, file)
	if assert.NotNil(t, gotFile) {
		assert.Equal(t, "s3://bucket/file.csv", gotFile.Path)
		assert.Equal(t, []string{"a", "b"}, gotFile.Files)
		assert.NotNil(t, gotFile.ProviderAuth)
	}

	mappings := convert2ThriftFieldMappings(ctx, []*entity.FieldMapping{{Source: "s", Target: "t"}})
	assert.Equal(t, []*dataset_job.FieldMapping{{Source: "s", Target: "t"}}, mappings)

	overwrite := true
	jobOpt := convert2ThriftDatasetIOJobOption(ctx, &entity.DatasetIOJobOption{
		OverwriteDataset: &overwrite,
		FieldWriteOptions: []*entity.FieldWriteOption{{
			FieldName: gptr.Of("field"),
		}},
	})
	if assert.NotNil(t, jobOpt) {
		assert.Equal(t, &overwrite, jobOpt.OverwriteDataset)
		assert.Len(t, jobOpt.FieldWriteOptions, 1)
	}
}

func TestConvertObjectStorageHelpers(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	provider := dataset.StorageProvider(7)
	imageName := "image.PNG"
	audioName := "sound.MP3"
	videoName := "clip.MP4"
	unknownName := "doc.txt"

	assert.Nil(t, convertObjectStorageToImage(ctx, nil))
	assert.Nil(t, convertObjectStorageToAudio(ctx, nil))
	assert.Nil(t, convertObjectStorageToVideo(ctx, nil))
	assert.Nil(t, convertStorageProvider(nil))
	assert.Nil(t, getAudioFormat(nil))
	assert.False(t, isImageAttachment(nil))
	assert.False(t, isAudioAttachment(nil))
	assert.False(t, isVideoAttachment(nil))

	attachments := []*dataset.ObjectStorage{
		{Name: &unknownName},
		{Name: &imageName, URL: gptr.Of("img"), Provider: &provider},
		{Name: &audioName, URL: gptr.Of("aud"), Provider: &provider},
		{Name: &videoName, URL: gptr.Of("vid"), Provider: &provider},
	}

	img := convertObjectStorageToImage(ctx, attachments)
	if assert.NotNil(t, img) {
		assert.Equal(t, gptr.Of("img"), img.URL)
	}
	audio := convertObjectStorageToAudio(ctx, attachments)
	if assert.NotNil(t, audio) {
		assert.Equal(t, "MP3", *audio.Format)
	}
	video := convertObjectStorageToVideo(ctx, attachments)
	if assert.NotNil(t, video) {
		assert.Equal(t, gptr.Of("vid"), video.URL)
	}
	assert.True(t, isImageAttachment(&dataset.ObjectStorage{Name: &imageName}))
	assert.True(t, isAudioAttachment(&dataset.ObjectStorage{Name: &audioName}))
	assert.True(t, isVideoAttachment(&dataset.ObjectStorage{Name: &videoName}))
	assert.False(t, isImageAttachment(&dataset.ObjectStorage{Name: &unknownName}))
	assert.NotNil(t, convertStorageProvider(&provider))
}

func TestToSchemaKey_AllBranches(t *testing.T) {
	t.Parallel()

	assert.Nil(t, toSchemaKey(nil))
	assert.Equal(t, gptr.Of(entity.SchemaKey_String), toSchemaKey(gptr.Of(dataset.SchemaKey_String)))
	assert.Equal(t, gptr.Of(entity.SchemaKey_Integer), toSchemaKey(gptr.Of(dataset.SchemaKey_Integer)))
	assert.Equal(t, gptr.Of(entity.SchemaKey_Float), toSchemaKey(gptr.Of(dataset.SchemaKey_Float)))
	assert.Equal(t, gptr.Of(entity.SchemaKey_Bool), toSchemaKey(gptr.Of(dataset.SchemaKey_Bool)))
	assert.Equal(t, gptr.Of(entity.SchemaKey_Message), toSchemaKey(gptr.Of(dataset.SchemaKey_Message)))
	assert.Equal(t, gptr.Of(entity.SchemaKey_SingleChoice), toSchemaKey(gptr.Of(dataset.SchemaKey_SingleChoice)))
	assert.Equal(t, gptr.Of(entity.SchemaKey_Trajectory), toSchemaKey(gptr.Of(dataset.SchemaKey_Trajectory)))
	assert.Nil(t, toSchemaKey(gptr.Of(dataset.SchemaKey(999))))
}
