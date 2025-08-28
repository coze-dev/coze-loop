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

func TestSchemaDTO2DO_Simple(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *eval_set.EvaluationSetSchema
		expected *entity.EvaluationSetSchema
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "minimal schema",
			input: &eval_set.EvaluationSetSchema{
				ID:              gptr.Of(int64(1)),
				AppID:           gptr.Of(int32(1)),
				WorkspaceID:     gptr.Of(int64(1)),
				EvaluationSetID: gptr.Of(int64(1)),
			},
			expected: &entity.EvaluationSetSchema{
				ID:              1,
				AppID:           1,
				SpaceID:         1,
				EvaluationSetID: 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := SchemaDTO2DO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSchemaDO2DTO_Simple(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *entity.EvaluationSetSchema
		expected *eval_set.EvaluationSetSchema
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "minimal schema",
			input: &entity.EvaluationSetSchema{
				ID:              1,
				AppID:           1,
				SpaceID:         1,
				EvaluationSetID: 1,
			},
			expected: &eval_set.EvaluationSetSchema{
				ID:              gptr.Of(int64(1)),
				AppID:           gptr.Of(int32(1)),
				WorkspaceID:     gptr.Of(int64(1)),
				EvaluationSetID: gptr.Of(int64(1)),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := SchemaDO2DTO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMultiModalSpecDO2DTO_Simple(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *entity.MultiModalSpec
		expected *dataset.MultiModalSpec
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "complete multimodal spec",
			input: &entity.MultiModalSpec{
				MaxFileCount:     5,
				MaxFileSize:      1048576,
				SupportedFormats: []string{"jpg", "png", "mp4"},
				MaxPartCount:     10,
			},
			expected: &dataset.MultiModalSpec{
				MaxFileCount:     gptr.Of(int64(5)),
				MaxFileSize:      gptr.Of(int64(1048576)),
				SupportedFormats: []string{"jpg", "png", "mp4"},
				MaxPartCount:     gptr.Of(int32(10)),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := MultiModalSpecDO2DTO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
