// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	domain_eval_target "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// TestSandboxInitConcurrency 归一化 + 双沙箱 2x 放大 + 1.2 倍余量向上取整：
//   - nil / <=0 由 NormalizeSubmitItemConcurNum 兜底为 DefaultSubmitItemConcurNum。
//   - Dual 模式在归一化基础上翻倍（一次评测占 2 个 sandbox execute），Single 不翻。
//   - 最后一律 ×1.2 并向上取整留余量：配额是硬闸，卡住的 item 直接判永久失败、不重试不排队。
//
// 期望值写成字面量而非重算一遍公式：用公式算等于把实现抄进断言，实现算错时测试跟着错。
func TestSandboxInitConcurrency(t *testing.T) {
	def := entity.DefaultSubmitItemConcurNum // 5
	cases := []struct {
		name string
		in   *int
		dual bool
		want int32
	}{
		// single: 5*1.2=6
		{name: "nil single -> default*1.2", in: nil, dual: false, want: 6},
		// dual: 5*2*1.2=12
		{name: "nil dual -> default*2*1.2", in: nil, dual: true, want: 12},
		{name: "zero single -> default*1.2", in: gptr.Of(0), dual: false, want: 6},
		{name: "negative single -> default*1.2", in: gptr.Of(-5), dual: false, want: 6},
		// 7*1.2=8.4 → ceil 9
		{name: "positive single ceil", in: gptr.Of(7), dual: false, want: 9},
		// 7*2*1.2=16.8 → ceil 17
		{name: "positive dual doubled then ceil", in: gptr.Of(7), dual: true, want: 17},
		// 1*2*1.2=2.4 → ceil 3: 单题双沙箱至少要 2, 余量再给 1。这条是 concurrency=1 时
		// 100% 撞配额那个 bug 的回归钉子 —— 结果必须 >= 2。
		{name: "one item dual must fit two sandboxes", in: gptr.Of(1), dual: true, want: 3},
		// 1*1.2=1.2 → ceil 2: 单沙箱单并发也留一点余量。
		{name: "one item single", in: gptr.Of(1), dual: false, want: 2},
		// 整数结果不该被余量多推一格: 5*2=10, 10*1.2=12 恰好整数。
		{name: "exact integer no extra bump", in: gptr.Of(def), dual: true, want: 12},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got := sandboxInitConcurrency(c.in, c.dual)
			assert.Equal(t, c.want, got)
			if c.dual {
				assert.GreaterOrEqual(t, got, int32(2),
					"双沙箱每 item 占 2 个 execution, 配额低于 2 时该 item 必然起不来")
			}
		})
	}
}

// TestSandboxTenantForExperimentEntity 校验 entity 层实验 → 沙箱租户推导。
// 只有 SandboxAgent.SandboxCountMode=Dual 时才升到 FornaxTraeEvalDualSandbox，其余均为 Default。
func TestSandboxTenantForExperimentEntity(t *testing.T) {
	cases := []struct {
		name string
		in   *entity.Experiment
		want rpc.SandboxTenant
	}{
		{name: "nil experiment", in: nil, want: rpc.SandboxTenantDefault},
		{name: "nil target", in: &entity.Experiment{}, want: rpc.SandboxTenantDefault},
		{name: "nil eval target version", in: &entity.Experiment{Target: &entity.EvalTarget{}}, want: rpc.SandboxTenantDefault},
		{
			name: "nil sandbox agent",
			in: &entity.Experiment{Target: &entity.EvalTarget{
				EvalTargetVersion: &entity.EvalTargetVersion{},
			}},
			want: rpc.SandboxTenantDefault,
		},
		{
			name: "single mode",
			in: &entity.Experiment{Target: &entity.EvalTarget{
				EvalTargetVersion: &entity.EvalTargetVersion{
					SandboxAgent: &entity.SandboxAgent{SandboxCountMode: entity.SandboxCountModeSingle},
				},
			}},
			want: rpc.SandboxTenantDefault,
		},
		{
			name: "dual mode",
			in: &entity.Experiment{Target: &entity.EvalTarget{
				EvalTargetVersion: &entity.EvalTargetVersion{
					SandboxAgent: &entity.SandboxAgent{SandboxCountMode: entity.SandboxCountModeDual},
				},
			}},
			want: rpc.SandboxTenantFornaxTraeEvalDualSandbox,
		},
		{
			name: "unrecognized mode falls back to Default",
			in: &entity.Experiment{Target: &entity.EvalTarget{
				EvalTargetVersion: &entity.EvalTargetVersion{
					SandboxAgent: &entity.SandboxAgent{SandboxCountMode: entity.SandboxCountMode("triple")},
				},
			}},
			want: rpc.SandboxTenantDefault,
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, sandboxTenantForExperimentEntity(c.in))
		})
	}
}

// TestSandboxTenantForExperimentDTO 校验 domain DTO 层实验 → 沙箱租户推导。
// 空 DTO / 无 SandboxAgent / 空 SandboxCountMode / 未识别值均返回 Default；仅 "dual" 升到 DualSandbox。
func TestSandboxTenantForExperimentDTO(t *testing.T) {
	dtoWithMode := func(mode string) *domain_expt.Experiment {
		return &domain_expt.Experiment{
			EvalTarget: &domain_eval_target.EvalTarget{
				EvalTargetVersion: &domain_eval_target.EvalTargetVersion{
					EvalTargetContent: &domain_eval_target.EvalTargetContent{
						SandboxAgent: &domain_eval_target.SandboxAgent{
							SandboxCountMode: gptr.Of(mode),
						},
					},
				},
			},
		}
	}

	t.Run("nil experiment", func(t *testing.T) {
		assert.Equal(t, rpc.SandboxTenantDefault, sandboxTenantForExperimentDTO(nil))
	})

	t.Run("nil eval target chain -> Default", func(t *testing.T) {
		assert.Equal(t, rpc.SandboxTenantDefault, sandboxTenantForExperimentDTO(&domain_expt.Experiment{}))
	})

	t.Run("nil sandbox agent -> Default", func(t *testing.T) {
		expt := &domain_expt.Experiment{
			EvalTarget: &domain_eval_target.EvalTarget{
				EvalTargetVersion: &domain_eval_target.EvalTargetVersion{
					EvalTargetContent: &domain_eval_target.EvalTargetContent{},
				},
			},
		}
		assert.Equal(t, rpc.SandboxTenantDefault, sandboxTenantForExperimentDTO(expt))
	})

	t.Run("empty mode -> Default", func(t *testing.T) {
		expt := &domain_expt.Experiment{
			EvalTarget: &domain_eval_target.EvalTarget{
				EvalTargetVersion: &domain_eval_target.EvalTargetVersion{
					EvalTargetContent: &domain_eval_target.EvalTargetContent{
						SandboxAgent: &domain_eval_target.SandboxAgent{},
					},
				},
			},
		}
		assert.Equal(t, rpc.SandboxTenantDefault, sandboxTenantForExperimentDTO(expt))
	})

	t.Run("single mode -> Default", func(t *testing.T) {
		assert.Equal(t, rpc.SandboxTenantDefault, sandboxTenantForExperimentDTO(dtoWithMode(domain_eval_target.SandboxCountModeSingle)))
	})

	t.Run("dual mode -> DualSandbox", func(t *testing.T) {
		assert.Equal(t, rpc.SandboxTenantFornaxTraeEvalDualSandbox, sandboxTenantForExperimentDTO(dtoWithMode(domain_eval_target.SandboxCountModeDual)))
	})

	t.Run("unrecognized mode -> Default", func(t *testing.T) {
		assert.Equal(t, rpc.SandboxTenantDefault, sandboxTenantForExperimentDTO(dtoWithMode("triple")))
	})
}
