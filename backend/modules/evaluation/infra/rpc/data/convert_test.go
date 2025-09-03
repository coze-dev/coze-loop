// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package data

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
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
