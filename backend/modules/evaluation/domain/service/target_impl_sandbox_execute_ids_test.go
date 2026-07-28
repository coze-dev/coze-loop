// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// TestSandboxExecuteIDsOf 钉住销毁沙箱时 execute id 的取值来源：优先用 operator 回传在 output ext
// 的列表（双沙箱一次调用创建多个 execution，id 命名规则是 operator 的实现细节），缺省/非法时退回
// 裸 record.ID（单沙箱实现的 executeID 即 invokeID）。取错会导致销毁不存在的 id、沙箱只能等 TTL。
func TestSandboxExecuteIDsOf(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	extRecord := func(v string) *entity.EvalTargetRecord {
		return &entity.EvalTargetRecord{
			ID: 123,
			EvalTargetOutputData: &entity.EvalTargetOutputData{
				Ext: map[string]string{consts.OutputDataExtKeySandboxExecuteIDs: v},
			},
		}
	}

	tests := []struct {
		name   string
		record *entity.EvalTargetRecord
		want   []string
	}{
		{
			name:   "ext 带双沙箱列表 → 按列表销",
			record: extRecord(`["123-agent","123-orch"]`),
			want:   []string{"123-agent", "123-orch"},
		},
		{
			name:   "ext 带单个 id → 按列表销",
			record: extRecord(`["123"]`),
			want:   []string{"123"},
		},
		{
			name:   "ext 空字符串 → 退回 record.ID",
			record: extRecord(""),
			want:   []string{"123"},
		},
		{
			name:   "ext 非法 JSON → 退回 record.ID",
			record: extRecord(`not-json`),
			want:   []string{"123"},
		},
		{
			name:   "ext 空数组 → 退回 record.ID",
			record: extRecord(`[]`),
			want:   []string{"123"},
		},
		{
			name:   "ext 元素含空串 → 过滤掉",
			record: extRecord(`["123-agent","","123-orch"]`),
			want:   []string{"123-agent", "123-orch"},
		},
		{
			name:   "无 output data → 退回 record.ID",
			record: &entity.EvalTargetRecord{ID: 123},
			want:   []string{"123"},
		},
		{
			name:   "output data 无 ext → 退回 record.ID",
			record: &entity.EvalTargetRecord{ID: 123, EvalTargetOutputData: &entity.EvalTargetOutputData{}},
			want:   []string{"123"},
		},
	}
	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, sandboxExecuteIDsOf(ctx, tc.record))
		})
	}
}
