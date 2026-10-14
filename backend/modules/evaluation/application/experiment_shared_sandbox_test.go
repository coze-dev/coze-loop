// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"testing"

	"github.com/stretchr/testify/assert"

	eval_target "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// 共享沙箱必须落到通用评测租户，绝不能回落 Default —— Default 是**旧扁平链路**的租户，
// 落到那里实验会静默跑成沙箱内 sandbox-pipeline，runtime 压根不启动而没有任何一层报错。
func TestSandboxTenantForExperimentEntity_SharedGoesToGeneralNotDefault(t *testing.T) {
	expt := &entity.Experiment{
		Target: &entity.EvalTarget{
			EvalTargetVersion: &entity.EvalTargetVersion{
				EvalTargetType: entity.EvalTargetTypeSandboxAgent,
				SandboxAgent:   &entity.SandboxAgent{SandboxCountMode: entity.SandboxCountModeShared},
			},
		},
	}
	got := sandboxTenantForExperimentEntity(expt)
	assert.Equal(t, rpc.SandboxTenantFornaxEvalGeneral, got)
	assert.NotEqual(t, rpc.SandboxTenantDefault, got, "回落 Default 等于把实验送进旧扁平链路")
}

// 共享拓扑一个 item 只占一个 execution：两个进程不是两个 execution。
func TestSandboxTaskConcurrencyForMode_SharedIsOneExecutionPerItem(t *testing.T) {
	n := 10
	shared := sandboxTaskConcurrencyForMode(&n, entity.SandboxCountModeShared)
	single := sandboxTaskConcurrencyForMode(&n, entity.SandboxCountModeSingle)
	dual := sandboxTaskConcurrencyForMode(&n, entity.SandboxCountModeDual)

	assert.Equal(t, single.sandbox, shared.sandbox, "共享拓扑与单沙箱同为 1 execution/item")
	assert.Less(t, shared.sandbox, dual.sandbox, "共享拓扑不该按双沙箱的 2 倍给配额")
}

// DTO 侧与 entity 侧必须同租户：DTO 是实验提交 OpenAPI 的入参形态，回落 Default 同样会把
// 实验静默送进旧扁平链路（见 entity 版同处注释）。
func TestSandboxTenantForExperimentDTO_SharedGoesToGeneralNotDefault(t *testing.T) {
	agent := &eval_target.SandboxAgent{}
	mode := eval_target.SandboxCountMode(entity.SandboxCountModeShared)
	agent.SetSandboxCountMode(&mode)
	expt := &domain_expt.Experiment{
		EvalTarget: &eval_target.EvalTarget{
			EvalTargetVersion: &eval_target.EvalTargetVersion{
				EvalTargetContent: &eval_target.EvalTargetContent{SandboxAgent: agent},
			},
		},
	}
	got := sandboxTenantForExperimentDTO(expt)
	assert.Equal(t, rpc.SandboxTenantFornaxEvalGeneral, got)
	assert.NotEqual(t, rpc.SandboxTenantDefault, got, "回落 Default 等于把实验送进旧扁平链路")
}

// Debug 入口的租户推导：shared 走通用评测租户；Dual 走旧链路租户；未识别回落 Default。
func TestSandboxTenantForAsyncDebug(t *testing.T) {
	assert.Equal(t, rpc.SandboxTenantFornaxEvalGeneral, sandboxTenantForAsyncDebug(entity.SandboxCountModeShared))
	assert.Equal(t, rpc.SandboxTenantFornaxTraeEvalDualSandbox, sandboxTenantForAsyncDebug(entity.SandboxCountModeDual))
	assert.Equal(t, rpc.SandboxTenantFornaxEvalGeneralGUI, sandboxTenantForAsyncDebug(entity.SandboxCountModeMacVMPlusSandbox))
	assert.Equal(t, rpc.SandboxTenantFornaxEvalGeneralGUI, sandboxTenantForAsyncDebug(entity.SandboxCountModeMacVMPlusSSH))
	assert.Equal(t, rpc.SandboxTenantDefault, sandboxTenantForAsyncDebug(entity.SandboxCountModeSingle))
}
