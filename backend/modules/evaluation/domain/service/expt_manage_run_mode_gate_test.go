// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/external/audit"
	auditMocks "github.com/coze-dev/coze-loop/backend/infra/external/audit/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// CheckExpt 里的多轮/SUA 准入门: 启用多轮跑法 ⟺ 评测对象是 SandboxAgent 且实验是 MultiSetConfig。
//
// # 为什么这道门必须在提交入口拦, 而不是留给下游
//
// 题目级多轮配置只有一条落地路径: MultiSetConfig 实验在首次调度时把 RunConf 冻结进
// item_config, 执行期由 SandboxAgent 算子读出组 case-file。这条链上的每一段都对
// "不满足前提"是**无声的**:
//
//   - 非 SandboxAgent 对象 (Prompt / Workflow / CustomRPC): 它们的执行路径压根不看
//     ext["builtin_run_conf"], 多轮配置进去了也没人读 —— 实验按单轮跑完并 success;
//   - 老 DataSet (SingleSet) 实验: 这条路径不写 item_config, RunConf 无处冻结 ——
//     同样是配了但不生效。
//
// 两种情况用户都拿到"提交成功 + 实验成功", 只有对着轨迹逐条数轮数才能发现自己配的多轮没跑。
// 所以这道门的价值全在"提前显性失败", 一旦它被误改成放行, 回归完全无声。
func TestExptMangerImpl_CheckExpt_MultiTurnRunModeGate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mgr := newTestExptManager(ctrl)
	mgr.audit.(*auditMocks.MockIAuditService).EXPECT().
		Audit(gomock.Any(), gomock.Any()).
		Return(audit.AuditRecord{AuditStatus: audit.AuditStatus_Approved}, nil).AnyTimes()

	sandboxTarget := func() *entity.EvalTarget {
		return &entity.EvalTarget{EvalTargetVersion: &entity.EvalTargetVersion{
			EvalTargetType: entity.EvalTargetTypeSandboxAgent,
			SandboxAgent:   &entity.SandboxAgent{Name: "agent"},
		}}
	}
	promptTarget := func() *entity.EvalTarget {
		return &entity.EvalTarget{EvalTargetVersion: &entity.EvalTargetVersion{
			EvalTargetType: entity.EvalTargetTypeLoopPrompt,
		}}
	}
	buildExpt := func(target *entity.EvalTarget, sourceType entity.ExptEvalSetSourceType, cfg *entity.RunModeConfig) *entity.Experiment {
		return &entity.Experiment{
			ID: 1, SpaceID: 100, Name: "n", Description: "d",
			Target:            target,
			EvalSetSourceType: sourceType,
			EvalConf: &entity.EvaluationConfiguration{
				ItemConcurNum: gptr.Of(3),
				RunModeConfig: cfg,
			},
		}
	}

	cases := []struct {
		name      string
		expt      *entity.Experiment
		wantError bool
		// wantErrContains 只在 wantError 时校验: 两条拒绝原因必须能区分, 否则用户不知道
		// 是"换对象"还是"换评测集配置方式"。
		wantErrContains string
	}{
		{
			// 唯一的放行组合。
			name:      "SandboxAgent + MultiSetConfig + 多轮 → 放行",
			expt:      buildExpt(sandboxTarget(), entity.ExptEvalSetSourceType_MultiSetConfig, &entity.RunModeConfig{RunMode: entity.RunModeFixedScriptMultiTurn}),
			wantError: false,
		},
		{
			name:            "Prompt 对象 + 多轮 → 拒绝",
			expt:            buildExpt(promptTarget(), entity.ExptEvalSetSourceType_MultiSetConfig, &entity.RunModeConfig{RunMode: entity.RunModeSUAMultiTurn}),
			wantError:       true,
			wantErrContains: "SandboxAgent",
		},
		{
			// Target 还没 load (提交早期): 判据 nil-safe 且按"非沙箱"拒绝 —— 宁可拒也不放行,
			// 放行后多轮配置进了非沙箱实验就再没有报错面。
			name:            "Target 为 nil + 多轮 → 拒绝",
			expt:            buildExpt(nil, entity.ExptEvalSetSourceType_MultiSetConfig, &entity.RunModeConfig{RunMode: entity.RunModeGoal}),
			wantError:       true,
			wantErrContains: "SandboxAgent",
		},
		{
			name:            "SandboxAgent + 老 SingleSet 实验 + 多轮 → 拒绝",
			expt:            buildExpt(sandboxTarget(), entity.ExptEvalSetSourceType_SingleSet, &entity.RunModeConfig{RunMode: entity.RunModeFixedScriptMultiTurn}),
			wantError:       true,
			wantErrContains: "MultiSetConfig",
		},

		// --- 下面全是"不触发这道门"的形态: 存量实验必须一律不受影响 ---
		{
			name:      "存量实验: 无 RunModeConfig → 不触发",
			expt:      buildExpt(promptTarget(), entity.ExptEvalSetSourceType_SingleSet, nil),
			wantError: false,
		},
		{
			// run_mode 空 = 没配跑法 (透传丢字段也是这个形态): 不该因此拒绝一个普通实验。
			name:      "RunModeConfig 在但 run_mode 空 → 不触发",
			expt:      buildExpt(promptTarget(), entity.ExptEvalSetSourceType_SingleSet, &entity.RunModeConfig{MaxRunMinutes: 30}),
			wantError: false,
		},
		{
			// single_turn 显式配置不是"多轮", 任何对象/任何实验形态都该放行 ——
			// 这条钉住那个 `!= RunModeSingleTurn` 的例外, 少了它前端只要下发默认 single_turn,
			// 所有 Prompt 实验都会被拒 (显性但影响面极大的回归)。
			name:      "显式 single_turn + Prompt 对象 → 不触发",
			expt:      buildExpt(promptTarget(), entity.ExptEvalSetSourceType_SingleSet, &entity.RunModeConfig{RunMode: entity.RunModeSingleTurn}),
			wantError: false,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			err := mgr.CheckExpt(context.Background(), c.expt, &entity.Session{})
			if !c.wantError {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.wantErrContains,
				"拒绝原因要指名道姓, 否则用户不知道该换对象还是换评测集配置方式")
		})
	}
}
