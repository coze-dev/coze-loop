// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tag

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/tag"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestTagValueListDTO2DO(t *testing.T) {
	tests := []struct {
		name string
		dtos []*tag.TagValue
		want []*entity.TagValue
	}{{
		name: "转换多个值",
		dtos: []*tag.TagValue{
			{
				TagValueID:   ptr.Of(int64(1)),
				TagValueName: ptr.Of("值1"),
				Status:       ptr.Of("active"),
			},
			{
				TagValueID:   ptr.Of(int64(2)),
				TagValueName: ptr.Of("值2"),
				Status:       ptr.Of("inactive"),
			},
		},
		want: []*entity.TagValue{
			{
				TagValueId:   1,
				TagValueName: "值1",
				Status:       "active",
			},
			{
				TagValueId:   2,
				TagValueName: "值2",
				Status:       "inactive",
			},
		},
	}, {
		name: "空列表输入",
		dtos: []*tag.TagValue{},
		want: []*entity.TagValue{},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TagValueListDTO2DO(tt.dtos)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTagListDTO(t *testing.T) {
	tests := []struct {
		name string
		dtos []*tag.TagInfo
		want []*entity.TagInfo
	}{{
		name: "转换多个TagInfo",
		dtos: []*tag.TagInfo{
			{
				TagKeyID:   ptr.Of(int64(1)),
				TagKeyName: ptr.Of("标签A"),
				Status:     ptr.Of("active"),
			},
			{
				TagKeyID:   ptr.Of(int64(2)),
				TagKeyName: ptr.Of("标签B"),
				Status:     ptr.Of("inactive"),
			},
		},
		want: []*entity.TagInfo{
			{
				TagKeyId:       1,
				TagKeyName:     "标签A",
				InActive:       false,
				TagContentType: entity.TagContentType(""),
				TagStatus:      entity.TagStatus("active"),
				TagValues:      []*entity.TagValue{},
				TagContentSpec: nil,
			},
			{
				TagKeyId:       2,
				TagKeyName:     "标签B",
				InActive:       true,
				TagContentType: entity.TagContentType(""),
				TagStatus:      entity.TagStatus("inactive"),
				TagValues:      []*entity.TagValue{},
				TagContentSpec: nil,
			},
		},
	}, {
		name: "空列表输入",
		dtos: []*tag.TagInfo{},
		want: []*entity.TagInfo{},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TagListDTO2DO(tt.dtos)
			assert.Equal(t, tt.want, got)
		})
	}
}
