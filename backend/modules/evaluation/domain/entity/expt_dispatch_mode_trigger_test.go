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

// TestShouldEnforceByTriggerAndType 在线实验必须被挡在 enforce 之外。
//
// 为什么这条不能只靠"在线实验不发 evalx trigger"这个约定：约定不在代码里，
// 而一旦破了，在线实验会进 enforce 并且**丢掉实验级 36h 超时兜底**
// （daemon 每拍刷 event.CreatedAt，那个绝对时钟永不到期）。
// 那意味着任何未被对账覆盖的窗口都会变成永久的额度泄漏 + item 丢失。
func TestShouldEnforceByTriggerAndType(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		trigger  string
		exptType ExptType
		want     bool
	}{
		// 离线 + evalx 是唯一进 enforce 的组合。
		"offline evalx": {trigger: "evalx", exptType: ExptType_Offline, want: true},

		// ★ 本次新增的闸：在线实验即使带 evalx 也不进（含大小写/空白变体，
		// 防止有人以为绕过 trim 就能进来）。
		"online evalx":        {trigger: "evalx", exptType: ExptType_Online, want: false},
		"online evalx spaces": {trigger: "  evalx  ", exptType: ExptType_Online, want: false},
		"online evalx mixed":  {trigger: "EvalX", exptType: ExptType_Online, want: false},
		"online non-evalx":    {trigger: "manual", exptType: ExptType_Online, want: false},
		"offline non-evalx":   {trigger: "manual", exptType: ExptType_Offline, want: false},
		// 零值 ExptType（未显式设置）不等于 Online，不该被这道闸挡掉 ——
		// 否则一批没填 expt_type 的调用方会静默失去中心调度。
		"zero type evalx": {trigger: "evalx", exptType: 0, want: true},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, c.want, ShouldEnforceByTriggerAndType(c.trigger, c.exptType))
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
