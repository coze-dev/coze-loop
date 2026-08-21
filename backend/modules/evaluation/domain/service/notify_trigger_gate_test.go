// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestIsFeishuNotifySuppressedByTrigger(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		expt *entity.Experiment
		want bool
	}{
		{name: "nil expt", expt: nil, want: false},
		{name: "empty trigger", expt: &entity.Experiment{TriggerType: ""}, want: false},
		{name: "manual", expt: &entity.Experiment{TriggerType: "manual"}, want: false},
		{name: "openapi", expt: &entity.Experiment{TriggerType: "openapi"}, want: false},
		{name: "schedule", expt: &entity.Experiment{TriggerType: "schedule"}, want: false},
		{name: "evalx", expt: &entity.Experiment{TriggerType: "evalx"}, want: true},
		// trigger_type 跨系统传递，上游可能带空白或大小写差异，一律按 evalx 处理。
		{name: "evalx with spaces", expt: &entity.Experiment{TriggerType: "  evalx  "}, want: true},
		{name: "evalx upper case", expt: &entity.Experiment{TriggerType: "EvalX"}, want: true},
		// 只有完全等于 evalx 才抑制，前后缀不同的值不误伤。
		{name: "evalx prefix only", expt: &entity.Experiment{TriggerType: "evalx_v2"}, want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, isFeishuNotifySuppressedByTrigger(tt.expt))
		})
	}
}
