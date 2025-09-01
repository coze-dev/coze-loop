// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/prompt"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type batchLabelDO2DTOTestCase struct {
	name     string
	input    []*entity.PromptLabel
	expected []*prompt.Label
}

func mockBatchLabelDO2DTOCases() []batchLabelDO2DTOTestCase {
	now := time.Now()
	
	return []batchLabelDO2DTOTestCase{
		{
			name:     "nil_input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty_slice",
			input:    []*entity.PromptLabel{},
			expected: nil,
		},
		{
			name: "single_valid_label",
			input: []*entity.PromptLabel{
				{
					ID:        1,
					SpaceID:   100,
					LabelKey:  "test_label",
					CreatedBy: "user1",
					CreatedAt: now,
					UpdatedBy: "user1",
					UpdatedAt: now,
				},
			},
			expected: []*prompt.Label{
				{
					Key: ptr.Of("test_label"),
				},
			},
		},
		{
			name: "multiple_valid_labels",
			input: []*entity.PromptLabel{
				{
					ID:        1,
					SpaceID:   100,
					LabelKey:  "label1",
					CreatedBy: "user1",
					CreatedAt: now,
					UpdatedBy: "user1",
					UpdatedAt: now,
				},
				{
					ID:        2,
					SpaceID:   100,
					LabelKey:  "label2",
					CreatedBy: "user2",
					CreatedAt: now,
					UpdatedBy: "user2",
					UpdatedAt: now,
				},
				{
					ID:        3,
					SpaceID:   100,
					LabelKey:  "label3",
					CreatedBy: "user3",
					CreatedAt: now,
					UpdatedBy: "user3",
					UpdatedAt: now,
				},
			},
			expected: []*prompt.Label{
				{
					Key: ptr.Of("label1"),
				},
				{
					Key: ptr.Of("label2"),
				},
				{
					Key: ptr.Of("label3"),
				},
			},
		},
		{
			name: "contains_nil_elements",
			input: []*entity.PromptLabel{
				nil,
				{
					ID:        1,
					SpaceID:   100,
					LabelKey:  "valid_label",
					CreatedBy: "user1",
					CreatedAt: now,
					UpdatedBy: "user1",
					UpdatedAt: now,
				},
				nil,
			},
			expected: []*prompt.Label{
				{
					Key: ptr.Of("valid_label"),
				},
			},
		},
		{
			name: "empty_label_key",
			input: []*entity.PromptLabel{
				{
					ID:        1,
					SpaceID:   100,
					LabelKey:  "",
					CreatedBy: "user1",
					CreatedAt: now,
					UpdatedBy: "user1",
					UpdatedAt: now,
				},
			},
			expected: []*prompt.Label{
				{
					Key: ptr.Of(""),
				},
			},
		},
		{
			name: "special_characters_in_key",
			input: []*entity.PromptLabel{
				{
					ID:        1,
					SpaceID:   100,
					LabelKey:  "label-with_special.chars@123!",
					CreatedBy: "user1",
					CreatedAt: now,
					UpdatedBy: "user1",
					UpdatedAt: now,
				},
			},
			expected: []*prompt.Label{
				{
					Key: ptr.Of("label-with_special.chars@123!"),
				},
			},
		},
		{
			name: "mixed_valid_and_nil",
			input: []*entity.PromptLabel{
				{
					ID:        1,
					SpaceID:   100,
					LabelKey:  "first_label",
					CreatedBy: "user1",
					CreatedAt: now,
					UpdatedBy: "user1",
					UpdatedAt: now,
				},
				nil,
				{
					ID:        2,
					SpaceID:   100,
					LabelKey:  "second_label",
					CreatedBy: "user2",
					CreatedAt: now,
					UpdatedBy: "user2",
					UpdatedAt: now,
				},
				nil,
				{
					ID:        3,
					SpaceID:   100,
					LabelKey:  "",
					CreatedBy: "user3",
					CreatedAt: now,
					UpdatedBy: "user3",
					UpdatedAt: now,
				},
			},
			expected: []*prompt.Label{
				{
					Key: ptr.Of("first_label"),
				},
				{
					Key: ptr.Of("second_label"),
				},
				{
					Key: ptr.Of(""),
				},
			},
		},
	}
}

func TestBatchLabelDO2DTO(t *testing.T) {
	for _, tt := range mockBatchLabelDO2DTOCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actual := BatchLabelDO2DTO(tt.input)
			assert.Equal(t, tt.expected, actual)
		})
	}
}