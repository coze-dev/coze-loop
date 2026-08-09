// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// IsSandboxAgentTarget 是**多轮/SUA 准入门的守卫函数**: CheckExpt 用它拒绝
// "非沙箱对象却配了多轮跑法" 的实验。它判错的两种后果不对称, 但都很贵 ——
//
//   - 假阴性 (沙箱实验被判为非沙箱): 用户合法的多轮实验在提交时被拒, 显性故障。
//   - 假阳性 (非沙箱实验被判为沙箱): 多轮配置放行, 但下游 item_config.RunConf 这条链
//     只有 SandboxAgent 算子会读 —— Prompt/Workflow 对象拿到多轮配置直接无视,
//     实验按单轮跑完并 success, 用户以为自己跑了多轮。这类"配了但没生效"最难发现。
//
// 判据刻意与 AsyncCallTarget 同惯例: **类型 OR SandboxAgent 配置**双判, 而不是只看类型 ——
// 版本级 EvalTargetType 在部分读路径上会是零值(只填了 SandboxAgent 配置), 只看类型会漏判。
func TestExperiment_IsSandboxAgentTarget(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		expt *Experiment
		want bool
	}{
		// --- nil 安全: CheckExpt 在提交早期调用, 那时 Target 常常还没 load 全 ---
		{name: "nil experiment", expt: nil, want: false},
		{name: "nil target", expt: &Experiment{}, want: false},
		{name: "nil target version", expt: &Experiment{Target: &EvalTarget{}}, want: false},

		// --- 双判的两条腿各自都要能单独成立 ---
		{
			name: "仅版本级类型为 SandboxAgent",
			expt: &Experiment{Target: &EvalTarget{EvalTargetVersion: &EvalTargetVersion{
				EvalTargetType: EvalTargetTypeSandboxAgent,
			}}},
			want: true,
		},
		{
			// 只填了 SandboxAgent 配置、类型是零值 —— 只看类型的实现会在这里漏判,
			// 于是合法的沙箱多轮实验被拒 (显性故障)。
			name: "类型零值但带 SandboxAgent 配置",
			expt: &Experiment{Target: &EvalTarget{EvalTargetVersion: &EvalTargetVersion{
				SandboxAgent: &SandboxAgent{Name: "agent"},
			}}},
			want: true,
		},

		// --- 非沙箱对象必须判 false, 否则多轮配置被放行却无人消费 ---
		{
			name: "Prompt 对象",
			expt: &Experiment{Target: &EvalTarget{EvalTargetVersion: &EvalTargetVersion{
				EvalTargetType: EvalTargetTypeLoopPrompt,
			}}},
			want: false,
		},
		{
			// WebAgent 也走异步执行 (AsyncCallTarget 为 true), 但它不是沙箱、不读 RunConf。
			// 这条用来钉住"沙箱判据 ≠ 异步判据", 别把两者混用。
			name: "WebAgent 对象 (异步但非沙箱)",
			expt: &Experiment{Target: &EvalTarget{EvalTargetVersion: &EvalTargetVersion{
				EvalTargetType: EvalTargetTypeWebAgent,
				WebAgent:       &WebAgent{},
			}}},
			want: false,
		},
		{
			// 顶层 EvalTarget.EvalTargetType 是沙箱、版本级却不是: 判据刻意只看**版本级**
			// (跑法配置消费方拿到的是版本级配置)。这条钉住"别退化成看顶层类型"。
			name: "仅顶层类型为 SandboxAgent, 版本级不是",
			expt: &Experiment{Target: &EvalTarget{
				EvalTargetType:    EvalTargetTypeSandboxAgent,
				EvalTargetVersion: &EvalTargetVersion{EvalTargetType: EvalTargetTypeLoopPrompt},
			}},
			want: false,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, c.want, c.expt.IsSandboxAgentTarget())
		})
	}
}
