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

func TestConvert2DatasetOrderBys_Simple(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	tests := []struct {
		name     string
		input    []*entity.OrderBy
		expected []*dataset.OrderBy
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty slice",
			input:    []*entity.OrderBy{},
			expected: nil,
		},
		{
			name: "single order by",
			input: []*entity.OrderBy{
				{
					Field: gptr.Of("name"),
					IsAsc: gptr.Of(true),
				},
			},
			expected: []*dataset.OrderBy{
				{
					Field: gptr.Of("name"),
					IsAsc: gptr.Of(true),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := convert2DatasetOrderBys(ctx, tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvert2DatasetOrderBy_Simple(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	tests := []struct {
		name     string
		input    *entity.OrderBy
		expected *dataset.OrderBy
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "ascending order",
			input: &entity.OrderBy{
				Field: gptr.Of("name"),
				IsAsc: gptr.Of(true),
			},
			expected: &dataset.OrderBy{
				Field: gptr.Of("name"),
				IsAsc: gptr.Of(true),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := convert2DatasetOrderBy(ctx, tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsImageAttachment_Simple(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *dataset.ObjectStorage
		expected bool
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: false,
		},
		{
			name: "jpg image",
			input: &dataset.ObjectStorage{
				Name: gptr.Of("test.jpg"),
			},
			expected: true,
		},
		{
			name: "text file",
			input: &dataset.ObjectStorage{
				Name: gptr.Of("test.txt"),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := isImageAttachment(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsAudioAttachment_Simple(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *dataset.ObjectStorage
		expected bool
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: false,
		},
		{
			name: "mp3 audio",
			input: &dataset.ObjectStorage{
				Name: gptr.Of("test.mp3"),
			},
			expected: true,
		},
		{
			name: "text file",
			input: &dataset.ObjectStorage{
				Name: gptr.Of("test.txt"),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := isAudioAttachment(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvert2EvaluationSetSpec(t *testing.T) {
	ctx := context.Background()

	t.Run("spec is nil", func(t *testing.T) {
		var spec *dataset.DatasetSpec
		res := convert2EvaluationSetSpec(ctx, spec)
		assert.Nil(t, res)
	})

	t.Run("all fields set, MultiModalSpec is nil", func(t *testing.T) {
		spec := &dataset.DatasetSpec{
			MaxFieldCount:  gptr.Of(int32(10)),
			MaxItemCount:   gptr.Of(int64(20)),
			MaxItemSize:    gptr.Of(int64(30)),
			MultiModalSpec: nil,
		}
		res := convert2EvaluationSetSpec(ctx, spec)
		assert.NotNil(t, res)
		assert.Equal(t, int32(10), res.MaxFieldCount)
		assert.Equal(t, int64(20), res.MaxItemCount)
		assert.Equal(t, int64(30), res.MaxItemSize)
		assert.Nil(t, res.MultiModalSpec)
	})

	t.Run("MultiModalSpec is not nil", func(t *testing.T) {
		spec := &dataset.DatasetSpec{
			MaxFieldCount: gptr.Of(int32(1)),
			MaxItemCount:  gptr.Of(int64(2)),
			MaxItemSize:   gptr.Of(int64(3)),
			MultiModalSpec: &dataset.MultiModalSpec{
				MaxFileCount:     gptr.Of(int64(4)),
				MaxFileSize:      gptr.Of(int64(5)),
				SupportedFormats: []string{"jpg", "png"},
			},
		}
		res := convert2EvaluationSetSpec(ctx, spec)
		assert.NotNil(t, res)
		assert.NotNil(t, res.MultiModalSpec)
		assert.Equal(t, int64(4), res.MultiModalSpec.MaxFileCount)
		assert.Equal(t, int64(5), res.MultiModalSpec.MaxFileSize)
		assert.Equal(t, []string{"jpg", "png"}, res.MultiModalSpec.SupportedFormats)
	})
}
