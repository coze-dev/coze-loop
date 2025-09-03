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
