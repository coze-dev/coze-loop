// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package dataset

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/entity"
	"github.com/stretchr/testify/assert"
)

func TestMultiModalSpecDO2DTO(t *testing.T) {
	tests := []struct {
		name string
		sp   *entity.MultiModalSpec
		want *dataset.MultiModalSpec
	}{
		{
			name: "normal case",
			sp: &entity.MultiModalSpec{
				MaxFileCount:     10,
				MaxFileSize:      1024,
				SupportedFormats: []string{"jpg", "png"},
				MaxPartCount:     5,
				SupportedFormatsByType: map[entity.ContentType][]string{
					entity.ContentTypeImage: {"jpg", "png"},
					entity.ContentTypeAudio: {"mp3"},
				},
				MaxFileSizeByType: map[entity.ContentType]int64{
					entity.ContentTypeImage: 512,
					entity.ContentTypeAudio: 256,
				},
			},
			want: &dataset.MultiModalSpec{
				MaxFileCount:     gptr.Of(int64(10)),
				MaxFileSize:      gptr.Of(int64(1024)),
				SupportedFormats: []string{"jpg", "png"},
				MaxPartCount:     gptr.Of(int32(5)),
				SupportedFormatsByType: map[dataset.ContentType][]string{
					dataset.ContentType_Image: {"jpg", "png"},
					dataset.ContentType_Audio: {"mp3"},
				},
				MaxFileSizeByType: map[dataset.ContentType]int64{
					dataset.ContentType_Image: 512,
					dataset.ContentType_Audio: 256,
				},
			},
		},
		{
			name: "empty maps",
			sp: &entity.MultiModalSpec{
				MaxFileCount:           1,
				MaxFileSize:            100,
				SupportedFormats:       []string{},
				MaxPartCount:           1,
				SupportedFormatsByType: map[entity.ContentType][]string{},
				MaxFileSizeByType:      map[entity.ContentType]int64{},
			},
			want: &dataset.MultiModalSpec{
				MaxFileCount:           gptr.Of(int64(1)),
				MaxFileSize:            gptr.Of(int64(100)),
				SupportedFormats:       []string{},
				MaxPartCount:           gptr.Of(int32(1)),
				SupportedFormatsByType: map[dataset.ContentType][]string{},
				MaxFileSizeByType:      map[dataset.ContentType]int64{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MultiModalSpecDO2DTO(tt.sp)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMultiModalSpecDTO2DO(t *testing.T) {
	tests := []struct {
		name string
		s    *dataset.MultiModalSpec
		want *entity.MultiModalSpec
	}{
		{
			name: "nil input",
			s:    nil,
			want: nil,
		},
		{
			name: "normal case",
			s: &dataset.MultiModalSpec{
				MaxFileCount:     gptr.Of(int64(10)),
				MaxFileSize:      gptr.Of(int64(1024)),
				SupportedFormats: []string{"jpg", "png"},
				MaxPartCount:     gptr.Of(int32(5)),
				SupportedFormatsByType: map[dataset.ContentType][]string{
					dataset.ContentType_Image: {"jpg", "png"},
					dataset.ContentType_Audio: {"mp3"},
				},
				MaxFileSizeByType: map[dataset.ContentType]int64{
					dataset.ContentType_Image: 512,
					dataset.ContentType_Audio: 256,
				},
			},
			want: &entity.MultiModalSpec{
				MaxFileCount:     10,
				MaxFileSize:      1024,
				SupportedFormats: []string{"jpg", "png"},
				MaxPartCount:     5,
				SupportedFormatsByType: map[entity.ContentType][]string{
					entity.ContentTypeImage: {"jpg", "png"},
					entity.ContentTypeAudio: {"mp3"},
				},
				MaxFileSizeByType: map[entity.ContentType]int64{
					entity.ContentTypeImage: 512,
					entity.ContentTypeAudio: 256,
				},
			},
		},
		{
			name: "empty maps",
			s: &dataset.MultiModalSpec{
				MaxFileCount:           gptr.Of(int64(1)),
				MaxFileSize:            gptr.Of(int64(100)),
				SupportedFormats:       []string{},
				MaxPartCount:           gptr.Of(int32(1)),
				SupportedFormatsByType: map[dataset.ContentType][]string{},
				MaxFileSizeByType:      map[dataset.ContentType]int64{},
			},
			want: &entity.MultiModalSpec{
				MaxFileCount:           1,
				MaxFileSize:            100,
				SupportedFormats:       []string{},
				MaxPartCount:           1,
				SupportedFormatsByType: map[entity.ContentType][]string{},
				MaxFileSizeByType:      map[entity.ContentType]int64{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MultiModalSpecDTO2DO(tt.s)
			assert.Equal(t, tt.want, got)
		})
	}
}
