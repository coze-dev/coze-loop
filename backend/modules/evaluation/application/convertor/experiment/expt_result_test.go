// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestItemResultsDO2DTO_ExtField(t *testing.T) {
	tests := []struct {
		name    string
		from    *entity.ItemResult
		wantExt map[string]string
		wantNil bool
	}{
		{
			name: "Ext字段有值",
			from: &entity.ItemResult{
				ItemID:      1,
				TurnResults: []*entity.TurnResult{},
				ItemIndex:   gptr.Of(int64(10)),
				Ext: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
			wantExt: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			wantNil: false,
		},
		{
			name: "Ext字段为空map",
			from: &entity.ItemResult{
				ItemID:      1,
				TurnResults: []*entity.TurnResult{},
				ItemIndex:   gptr.Of(int64(10)),
				Ext:         map[string]string{},
			},
			wantExt: nil,
			wantNil: true,
		},
		{
			name: "Ext字段为nil",
			from: &entity.ItemResult{
				ItemID:      1,
				TurnResults: []*entity.TurnResult{},
				ItemIndex:   gptr.Of(int64(10)),
				Ext:         nil,
			},
			wantExt: nil,
			wantNil: true,
		},
		{
			name: "Ext字段有多个值",
			from: &entity.ItemResult{
				ItemID:      1,
				TurnResults: []*entity.TurnResult{},
				ItemIndex:   gptr.Of(int64(10)),
				Ext: map[string]string{
					"span_id":  "span-123",
					"trace_id": "trace-456",
					"log_id":   "log-789",
				},
			},
			wantExt: map[string]string{
				"span_id":  "span-123",
				"trace_id": "trace-456",
				"log_id":   "log-789",
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ItemResultsDO2DTO(tt.from)
			assert.NotNil(t, got)
			assert.Equal(t, tt.from.ItemID, got.ItemID)
			assert.Equal(t, tt.from.ItemIndex, got.ItemIndex)

			if tt.wantNil {
				assert.Nil(t, got.Ext)
			} else {
				assert.NotNil(t, got.Ext)
				assert.Equal(t, tt.wantExt, got.Ext)
			}
		})
	}
}

func TestItemResultsDO2DTOs_ExtField(t *testing.T) {
	tests := []struct {
		name string
		from []*entity.ItemResult
		want []map[string]string
	}{
		{
			name: "多个ItemResult，Ext字段都有值",
			from: []*entity.ItemResult{
				{
					ItemID:      1,
					TurnResults: []*entity.TurnResult{},
					Ext: map[string]string{
						"key1": "value1",
					},
				},
				{
					ItemID:      2,
					TurnResults: []*entity.TurnResult{},
					Ext: map[string]string{
						"key2": "value2",
					},
				},
			},
			want: []map[string]string{
				{"key1": "value1"},
				{"key2": "value2"},
			},
		},
		{
			name: "多个ItemResult，部分Ext字段为空",
			from: []*entity.ItemResult{
				{
					ItemID:      1,
					TurnResults: []*entity.TurnResult{},
					Ext: map[string]string{
						"key1": "value1",
					},
				},
				{
					ItemID:      2,
					TurnResults: []*entity.TurnResult{},
					Ext:         map[string]string{},
				},
			},
			want: []map[string]string{
				{"key1": "value1"},
				nil,
			},
		},
		{
			name: "空列表",
			from: []*entity.ItemResult{},
			want: []map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ItemResultsDO2DTOs(tt.from)
			assert.Equal(t, len(tt.from), len(got))
			for i, item := range got {
				if i < len(tt.want) {
					if tt.want[i] == nil {
						assert.Nil(t, item.Ext)
					} else {
						assert.Equal(t, tt.want[i], item.Ext)
					}
				}
			}
		})
	}
}
