// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"testing"

	"github.com/stretchr/testify/assert"

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

// Debug 入口的租户推导：shared 走通用评测租户；Dual 走旧链路租户；未识别回落 Default。
func TestSandboxTenantForAsyncDebug(t *testing.T) {
	assert.Equal(t, rpc.SandboxTenantFornaxEvalGeneral, sandboxTenantForAsyncDebug(entity.SandboxCountModeShared))
	assert.Equal(t, rpc.SandboxTenantFornaxTraeEvalDualSandbox, sandboxTenantForAsyncDebug(entity.SandboxCountModeDual))
	assert.Equal(t, rpc.SandboxTenantFornaxEvalGeneralGUI, sandboxTenantForAsyncDebug(entity.SandboxCountModeMacVMPlusSandbox))
	assert.Equal(t, rpc.SandboxTenantFornaxEvalGeneralGUI, sandboxTenantForAsyncDebug(entity.SandboxCountModeMacVMPlusSSH))
	assert.Equal(t, rpc.SandboxTenantDefault, sandboxTenantForAsyncDebug(entity.SandboxCountModeSingle))
}
