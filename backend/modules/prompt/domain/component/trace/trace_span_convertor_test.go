// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"testing"

	"github.com/coze-dev/cozeloop-go/spec/tracespec"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func Test_parseBase64DataTypeSafe(t *testing.T) {
	tests := []struct {
		name       string
		base64Data *string
		wantImage  bool
		wantVideo  bool
	}{
		{
			name:       "nil_input",
			base64Data: nil,
			wantImage:  true,
			wantVideo:  false,
		},
		{
			name:       "empty_string",
			base64Data: ptr.Of(""),
			wantImage:  true,
			wantVideo:  false,
		},
		{
			name:       "valid_image_base64",
			base64Data: ptr.Of("data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="),
			wantImage:  true,
			wantVideo:  false,
		},
		{
			name:       "valid_image_jpeg_base64",
			base64Data: ptr.Of("data:image/jpeg;base64,/9j/4AAQSkZJRgABAQEAYABgAAD/2wBDAAgGBgcGBQgHBwcJCQgKDBQNDAsLDBkSEw8UHRofHh0aHBwgJC4nICIsIxwcKDcpLDAxNDQ0Hyc5PTgyPC4zNDL/2wBDAQkJCQwLDBgNDRgyIRwhMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjL/wAARCAABAAEDASIAAhEBAxEB/8QAFQABAQAAAAAAAAAAAAAAAAAAAAv/xAAUEAEAAAAAAAAAAAAAAAAAAAAA/8QAFQEBAQAAAAAAAAAAAAAAAAAAAAX/xAAUEQEAAAAAAAAAAAAAAAAAAAAA/9oADAMBAAIRAxEAPwCwAA8A/9k="),
			wantImage:  true,
			wantVideo:  false,
		},
		{
			name:       "valid_video_base64",
			base64Data: ptr.Of("data:video/mp4;base64,AAAAIGZ0eXBpc29tAAACAGlzb21pc28yYXZjMW1wNDEAAAAIZnJlZQAAAs1tZGF0"),
			wantImage:  false,
			wantVideo:  true,
		},
		{
			name:       "valid_video_webm_base64",
			base64Data: ptr.Of("data:video/webm;base64,GkXfo59ChoEBQveBAULygQRC84EIQoKEd2VibUKHgQRChYECGFOAZwH/////////FUmpZpkq17GDD0JATYCGQ2hyb21l"),
			wantImage:  false,
			wantVideo:  true,
		},
		{
			name:       "invalid_base64_format",
			base64Data: ptr.Of("invalid-base64-data"),
			wantImage:  true,
			wantVideo:  false,
		},
		{
			name:       "unknown_mime_type",
			base64Data: ptr.Of("data:application/octet-stream;base64,AAAAA=="),
			wantImage:  true,
			wantVideo:  false,
		},
		{
			name:       "audio_mime_type",
			base64Data: ptr.Of("data:audio/mp3;base64,//uQAAAAA=="),
			wantImage:  true,
			wantVideo:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotImage, gotVideo := parseBase64DataTypeSafe(tt.base64Data)
			assert.Equal(t, tt.wantImage, gotImage, "isImage mismatch")
			assert.Equal(t, tt.wantVideo, gotVideo, "isVideo mismatch")
		})
	}
}

func TestContentPartToSpanPart(t *testing.T) {
	tests := []struct {
		name string
		part *entity.ContentPart
		want *tracespec.ModelMessagePart
	}{
		{
			name: "nil_input",
			part: nil,
			want: nil,
		},
		{
			name: "text_content",
			part: &entity.ContentPart{
				Type: entity.ContentTypeText,
				Text: ptr.Of("Hello, world!"),
			},
			want: &tracespec.ModelMessagePart{
				Type:     tracespec.ModelMessagePartTypeText,
				Text:     "Hello, world!",
				ImageURL: nil,
				FileURL:  nil,
			},
		},
		{
			name: "image_url_content",
			part: &entity.ContentPart{
				Type: entity.ContentTypeImageURL,
				ImageURL: &entity.ImageURL{
					URL: "https://example.com/image.png",
				},
			},
			want: &tracespec.ModelMessagePart{
				Type: tracespec.ModelMessagePartTypeImage,
				Text: "",
				ImageURL: &tracespec.ModelImageURL{
					URL: "https://example.com/image.png",
				},
				FileURL: nil,
			},
		},
		{
			name: "video_url_content",
			part: &entity.ContentPart{
				Type: entity.ContentTypeVideoURL,
				VideoURL: &entity.VideoURL{
					URL: "https://example.com/video.mp4",
				},
			},
			want: &tracespec.ModelMessagePart{
				Type:     tracespec.ModelMessagePartTypeFile,
				Text:     "",
				ImageURL: nil,
				FileURL: &tracespec.ModelFileURL{
					URL: "https://example.com/video.mp4",
				},
			},
		},
		{
			name: "base64_image_data",
			part: &entity.ContentPart{
				Type:       entity.ContentTypeBase64Data,
				Base64Data: ptr.Of("data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="),
			},
			want: &tracespec.ModelMessagePart{
				Type: tracespec.ModelMessagePartTypeImage,
				Text: "",
				ImageURL: &tracespec.ModelImageURL{
					URL: "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
				},
				FileURL: nil,
			},
		},
		{
			name: "base64_video_data",
			part: &entity.ContentPart{
				Type:       entity.ContentTypeBase64Data,
				Base64Data: ptr.Of("data:video/mp4;base64,AAAAIGZ0eXBpc29tAAACAGlzb21pc28yYXZjMW1wNDEAAAAIZnJlZQAAAs1tZGF0"),
			},
			want: &tracespec.ModelMessagePart{
				Type:     tracespec.ModelMessagePartTypeFile,
				Text:     "",
				ImageURL: nil,
				FileURL: &tracespec.ModelFileURL{
					URL: "data:video/mp4;base64,AAAAIGZ0eXBpc29tAAACAGlzb21pc28yYXZjMW1wNDEAAAAIZnJlZQAAAs1tZGF0",
				},
			},
		},
		{
			name: "base64_invalid_data_defaults_to_image",
			part: &entity.ContentPart{
				Type:       entity.ContentTypeBase64Data,
				Base64Data: ptr.Of("invalid-base64-data"),
			},
			want: &tracespec.ModelMessagePart{
				Type: tracespec.ModelMessagePartTypeImage,
				Text: "",
				ImageURL: &tracespec.ModelImageURL{
					URL: "invalid-base64-data",
				},
				FileURL: nil,
			},
		},
		{
			name: "base64_empty_data_defaults_to_image",
			part: &entity.ContentPart{
				Type:       entity.ContentTypeBase64Data,
				Base64Data: ptr.Of(""),
			},
			want: &tracespec.ModelMessagePart{
				Type:     tracespec.ModelMessagePartTypeImage,
				Text:     "",
				ImageURL: &tracespec.ModelImageURL{URL: ""},
				FileURL:  nil,
			},
		},
		{
			name: "multi_part_variable",
			part: &entity.ContentPart{
				Type: entity.ContentTypeMultiPartVariable,
				Text: ptr.Of("some variable"),
			},
			want: &tracespec.ModelMessagePart{
				Type:     "multi_part_variable",
				Text:     "some variable",
				ImageURL: nil,
				FileURL:  nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContentPartToSpanPart(tt.part)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestContentTypeToSpanPartType(t *testing.T) {
	tests := []struct {
		name     string
		partType entity.ContentType
		want     tracespec.ModelMessagePartType
	}{
		{
			name:     "text_type",
			partType: entity.ContentTypeText,
			want:     tracespec.ModelMessagePartTypeText,
		},
		{
			name:     "image_url_type",
			partType: entity.ContentTypeImageURL,
			want:     tracespec.ModelMessagePartTypeImage,
		},
		{
			name:     "video_url_type",
			partType: entity.ContentTypeVideoURL,
			want:     tracespec.ModelMessagePartTypeFile,
		},
		{
			name:     "multi_part_variable_type",
			partType: entity.ContentTypeMultiPartVariable,
			want:     "multi_part_variable",
		},
		{
			name:     "unknown_type",
			partType: entity.ContentType("unknown"),
			want:     tracespec.ModelMessagePartType("unknown"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContentTypeToSpanPartType(tt.partType)
			assert.Equal(t, tt.want, got)
		})
	}
}
