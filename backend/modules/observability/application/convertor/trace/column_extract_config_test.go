// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"testing"

	tracedto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/trace"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/stretchr/testify/assert"
)

func TestColumnExtractRulesDTO2DO(t *testing.T) {
	tests := []struct {
		name string
		dtos []*tracedto.ColumnExtractRule
		want []entity.ColumnExtractRule
	}{
		{
			name: "nil input",
			dtos: nil,
			want: nil,
		},
		{
			name: "empty input",
			dtos: []*tracedto.ColumnExtractRule{},
			want: nil,
		},
		{
			name: "single rule",
			dtos: []*tracedto.ColumnExtractRule{
				{Column: "input", JSONPath: "$.messages[0].content"},
			},
			want: []entity.ColumnExtractRule{
				{Column: "input", JSONPath: "$.messages[0].content"},
			},
		},
		{
			name: "multiple rules",
			dtos: []*tracedto.ColumnExtractRule{
				{Column: "input", JSONPath: "$.messages[0].content"},
				{Column: "output", JSONPath: "$.choices[0].message.content"},
			},
			want: []entity.ColumnExtractRule{
				{Column: "input", JSONPath: "$.messages[0].content"},
				{Column: "output", JSONPath: "$.choices[0].message.content"},
			},
		},
		{
			name: "nil element skipped",
			dtos: []*tracedto.ColumnExtractRule{
				{Column: "input", JSONPath: "$.messages[0].content"},
				nil,
				{Column: "output", JSONPath: "$.data"},
			},
			want: []entity.ColumnExtractRule{
				{Column: "input", JSONPath: "$.messages[0].content"},
				{Column: "output", JSONPath: "$.data"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ColumnExtractRulesDTO2DO(tt.dtos)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestColumnExtractRulesDO2DTO(t *testing.T) {
	tests := []struct {
		name string
		dos  []entity.ColumnExtractRule
		want []*tracedto.ColumnExtractRule
	}{
		{
			name: "nil input",
			dos:  nil,
			want: nil,
		},
		{
			name: "empty input",
			dos:  []entity.ColumnExtractRule{},
			want: nil,
		},
		{
			name: "single rule",
			dos: []entity.ColumnExtractRule{
				{Column: "input", JSONPath: "$.messages[0].content"},
			},
			want: []*tracedto.ColumnExtractRule{
				{Column: "input", JSONPath: "$.messages[0].content"},
			},
		},
		{
			name: "multiple rules",
			dos: []entity.ColumnExtractRule{
				{Column: "input", JSONPath: "$.messages[0].content"},
				{Column: "output", JSONPath: "$.choices[0].message.content"},
			},
			want: []*tracedto.ColumnExtractRule{
				{Column: "input", JSONPath: "$.messages[0].content"},
				{Column: "output", JSONPath: "$.choices[0].message.content"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ColumnExtractRulesDO2DTO(tt.dos)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestColumnExtractRulesRoundTrip(t *testing.T) {
	original := []*tracedto.ColumnExtractRule{
		{Column: "input", JSONPath: "$.messages[0].content"},
		{Column: "output", JSONPath: "$.choices[0].message.content"},
	}
	dos := ColumnExtractRulesDTO2DO(original)
	roundTripped := ColumnExtractRulesDO2DTO(dos)
	assert.Equal(t, original, roundTripped)
}
