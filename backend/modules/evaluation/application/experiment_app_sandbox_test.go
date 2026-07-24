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

// TestSandboxInitConcurrency 归一化 + 双沙箱 2x 放大：
//   - nil / <=0 由 NormalizeSubmitItemConcurNum 兜底为 DefaultSubmitItemConcurNum。
//   - Dual 模式在归一化基础上翻倍，Single 保持原值。
func TestSandboxInitConcurrency(t *testing.T) {
	def := int32(entity.DefaultSubmitItemConcurNum)
	cases := []struct {
		name string
		in   *int
		dual bool
		want int32
	}{
		{name: "nil single -> default", in: nil, dual: false, want: def},
		{name: "nil dual -> default*2", in: nil, dual: true, want: def * 2},
		{name: "zero single -> default", in: gptr.Of(0), dual: false, want: def},
		{name: "negative single -> default", in: gptr.Of(-5), dual: false, want: def},
		{name: "positive single passthrough", in: gptr.Of(7), dual: false, want: 7},
		{name: "positive dual doubled", in: gptr.Of(7), dual: true, want: 14},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, sandboxInitConcurrency(c.in, c.dual))
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
