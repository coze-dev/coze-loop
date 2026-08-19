// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldEnforceByTrigger(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		trigger string
		want    bool
	}{
		// 只有 EvalX 进 enforce：它会申报 priority 与消耗向量。
		"evalx":             {trigger: "evalx", want: true},
		"evalx with spaces": {trigger: "  evalx  ", want: true},
		"evalx mixed case":  {trigger: "EvalX", want: true},
		"evalx upper":       {trigger: "EVALX", want: true},
		// 其它入口保持 legacy，行为与引入中心调度前一致。
		"manual":   {trigger: "manual", want: false},
		"empty":    {trigger: "", want: false},
		"blank":    {trigger: "   ", want: false},
		"schedule": {trigger: "schedule", want: false},
		"openapi":  {trigger: "openapi", want: false},
		// 前缀/子串不得误命中 —— 否则一个叫 "evalx_v2" 的新来源会被静默拽进 enforce，
		// 而它可能并不申报消耗向量，实验会建好却一个 item 都不跑。
		"prefix only": {trigger: "evalx_v2", want: false},
		"suffix only": {trigger: "my_evalx", want: false},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, c.want, ShouldEnforceByTrigger(c.trigger))
		})
	}
}

func TestExptTriggerTypeEvalxValue(t *testing.T) {
	t.Parallel()

	// 该常量必须与 IDL 的 `const ExptTriggerType Evalx = "evalx"` 逐字一致。
	// domain 层不 import kitex_gen（分层约束），因此靠本断言守住两处不漂移 ——
	// 一旦漂移，EvalX 发起的实验会被判成 legacy，中心调度静默失去全部候选。
	assert.Equal(t, "evalx", ExptTriggerTypeEvalx)
}
