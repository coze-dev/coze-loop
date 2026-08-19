// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassifyStepErrorType(t *testing.T) {
	t.Parallel()

	configured := &ExptSandboxStepMetricConf{NonSLACode: []int32{600100101, 600500101, 600200101}}

	tests := []struct {
		name    string
		success bool
		code    int32
		cfg     *ExptSandboxStepMetricConf
		want    string
	}{
		{name: "success_has_no_class", success: true, cfg: configured, want: "-"},
		{name: "success_ignores_code", success: true, code: 600100101, cfg: configured, want: "-"},
		{name: "failure_without_code_is_unknown", success: false, code: 0, cfg: configured, want: "unknown"},
		{name: "configured_code_is_non_engineering", success: false, code: 600500101, cfg: configured, want: "non_engineering"},
		// 默认值反转：未配到的错误码算工程错误，宁误报不静默豁免（spec D4）。
		{name: "unmapped_code_defaults_to_engineering", success: false, code: 600123, cfg: configured, want: "engineering"},
		// 配置读不到 / 键不存在 → nil；值为空 → 空表。两者都是同一个安全出口。
		{name: "nil_conf_defaults_to_engineering", success: false, code: 600100101, cfg: nil, want: "engineering"},
		{name: "empty_conf_defaults_to_engineering", success: false, code: 600100101, cfg: &ExptSandboxStepMetricConf{}, want: "engineering"},
		{name: "nil_conf_still_reports_unknown", success: false, code: 0, cfg: nil, want: "unknown"},
		// 前人的 coze-loop 侧工程码在本分类下同样是 engineering——不是因为命中表，
		// 而是因为默认值就是它。
		{name: "coze_loop_code_also_engineering", success: false, code: 601200702, cfg: configured, want: "engineering"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ClassifyStepErrorType(tt.success, tt.code, tt.cfg))
		})
	}
}

func TestExptSandboxStepMetricConf_IsNonSLACode(t *testing.T) {
	t.Parallel()

	var nilConf *ExptSandboxStepMetricConf
	assert.False(t, nilConf.IsNonSLACode(600100101))
	assert.False(t, (&ExptSandboxStepMetricConf{}).IsNonSLACode(600100101))

	conf := &ExptSandboxStepMetricConf{NonSLACode: []int32{600100101, 600200101}}
	assert.True(t, conf.IsNonSLACode(600100101))
	assert.True(t, conf.IsNonSLACode(600200101))
	assert.False(t, conf.IsNonSLACode(600300101))
}
