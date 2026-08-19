// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package step_event

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassifyErrorType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		success bool
		code    int32
		want    string
	}{
		{name: "success_has_no_class", success: true, want: "-"},
		{name: "success_ignores_code", success: true, code: 600123, want: "-"},
		{name: "failure_without_code_is_unknown", success: false, code: 0, want: "unknown"},
		// 默认值反转：未在表里的错误码算工程错误，宁误报不静默豁免（spec D4）。
		{name: "unmapped_code_defaults_to_engineering", success: false, code: 600123, want: "engineering"},
		{name: "runtime_errno_defaults_to_engineering", success: false, code: 600001, want: "engineering"},
		// 前人的 coze-loop 侧工程码在本分类下同样是 engineering——不是因为命中表，
		// 而是因为默认值就是它。
		{name: "coze_loop_code_also_engineering", success: false, code: 601200702, want: "engineering"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ClassifyErrorType(tt.success, tt.code))
		})
	}
}

// TestNonEngineeringErrorCodes_TableIsRespected 用一个临时注入的码验证表命中分支可达，
// 避免「表为空 → 分支永远走不到 → 没人知道它坏了」。
func TestNonEngineeringErrorCodes_TableIsRespected(t *testing.T) {
	const probe int32 = 999999
	nonEngineeringErrorCodes[probe] = struct{}{}
	defer delete(nonEngineeringErrorCodes, probe)

	assert.Equal(t, "non_engineering", ClassifyErrorType(false, probe))
}
