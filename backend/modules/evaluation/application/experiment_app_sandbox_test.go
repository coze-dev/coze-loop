// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	domain_eval_target "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	exptpb "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// TestSandboxInitConcurrency 归一化 + 单 task 双 execution 的 2x 放大 + buffer 余量：
//   - nil / <=0 由 NormalizeSubmitItemConcurNum 兜底为 DefaultSubmitItemConcurNum。
//   - 单个 task 每 item 占 2 个 execution 时在归一化基础上翻倍，否则不翻。
//   - 最后一律 ×sandboxConcurrencyBuffer 并向上取整留余量：配额是硬闸，卡住的 item 直接判永久失败、不重试不排队。
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
		// buffer=5。single: 5*5=25
		{name: "nil single -> default*buffer", in: nil, dual: false, want: 25},
		// dual: 5*2*5=50
		{name: "nil dual -> default*2*buffer", in: nil, dual: true, want: 50},
		{name: "zero single -> default*buffer", in: gptr.Of(0), dual: false, want: 25},
		{name: "negative single -> default*buffer", in: gptr.Of(-5), dual: false, want: 25},
		// 7*5=35
		{name: "positive single", in: gptr.Of(7), dual: false, want: 35},
		// 7*2*5=70
		{name: "positive dual doubled", in: gptr.Of(7), dual: true, want: 70},
		// 1*2*5=10: 单题双沙箱至少要 2, 余量再放大。这条是 concurrency=1 时
		// 100% 撞配额那个 bug 的回归钉子 —— 结果必须 >= 2。
		{name: "one item dual must fit two sandboxes", in: gptr.Of(1), dual: true, want: 10},
		// 1*5=5: 单沙箱单并发也留余量。
		{name: "one item single", in: gptr.Of(1), dual: false, want: 5},
		// 整数结果不该被余量多推一格: 5*2*5=50 恰好整数。
		{name: "exact integer no extra bump", in: gptr.Of(def), dual: true, want: 50},
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
// 只有 SandboxAgent.SandboxCountMode=Dual 时才升到 FornaxEvalGeneral，其余均为 Default。
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
			// dual 但无 run_mode_config → 旧链路租户（存量实验都落这一档）
			name: "dual mode without run_mode_config -> legacy tenant",
			in: &entity.Experiment{Target: &entity.EvalTarget{
				EvalTargetVersion: &entity.EvalTargetVersion{
					SandboxAgent: &entity.SandboxAgent{SandboxCountMode: entity.SandboxCountModeDual},
				},
			}},
			want: rpc.SandboxTenantFornaxTraeEvalDualSandbox,
		},
		{
			// dual + 合法 run_mode → 新链路租户
			name: "dual mode with valid run_mode -> new tenant",
			in: &entity.Experiment{
				Target: &entity.EvalTarget{
					EvalTargetVersion: &entity.EvalTargetVersion{
						SandboxAgent: &entity.SandboxAgent{SandboxCountMode: entity.SandboxCountModeDual},
					},
				},
				EvalConf: &entity.EvaluationConfiguration{
					RunModeConfig: &entity.RunModeConfig{RunMode: entity.RunModeSUAMultiTurn},
				},
			},
			want: rpc.SandboxTenantFornaxEvalGeneral,
		},
		{
			// ★ dual + config 在但 run_mode 为空（透传丢字段的形态）→ 必须落旧链路，不能误判成新链路
			name: "dual mode with empty run_mode -> legacy tenant",
			in: &entity.Experiment{
				Target: &entity.EvalTarget{
					EvalTargetVersion: &entity.EvalTargetVersion{
						SandboxAgent: &entity.SandboxAgent{SandboxCountMode: entity.SandboxCountModeDual},
					},
				},
				EvalConf: &entity.EvaluationConfiguration{
					RunModeConfig: &entity.RunModeConfig{},
				},
			},
			want: rpc.SandboxTenantFornaxTraeEvalDualSandbox,
		},
		{
			// ★ dual + run_mode 是非法字面量 → 落旧链路（RunModeToInt 会静默回落 1，不能用它判）
			name: "dual mode with invalid run_mode -> legacy tenant",
			in: &entity.Experiment{
				Target: &entity.EvalTarget{
					EvalTargetVersion: &entity.EvalTargetVersion{
						SandboxAgent: &entity.SandboxAgent{SandboxCountMode: entity.SandboxCountModeDual},
					},
				},
				EvalConf: &entity.EvaluationConfiguration{
					RunModeConfig: &entity.RunModeConfig{RunMode: entity.RunMode("bogus_mode")},
				},
			},
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
		{
			// mac_vm_plus_sandbox → GUI 专用租户（两 task 共用、靠 ResourceType 区分）
			name: "mac_vm_plus_sandbox -> GUI tenant",
			in: &entity.Experiment{Target: &entity.EvalTarget{
				EvalTargetVersion: &entity.EvalTargetVersion{
					SandboxAgent: &entity.SandboxAgent{SandboxCountMode: entity.SandboxCountModeMacVMPlusSandbox},
				},
			}},
			want: rpc.SandboxTenantFornaxEvalGeneralGUI,
		},
		{
			// mac_vm_plus_ssh 同样落 GUI 租户
			name: "mac_vm_plus_ssh -> GUI tenant",
			in: &entity.Experiment{Target: &entity.EvalTarget{
				EvalTargetVersion: &entity.EvalTargetVersion{
					SandboxAgent: &entity.SandboxAgent{SandboxCountMode: entity.SandboxCountModeMacVMPlusSSH},
				},
			}},
			want: rpc.SandboxTenantFornaxEvalGeneralGUI,
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
// 空 DTO / 无 SandboxAgent / 空 SandboxCountMode / 未识别值均返回 Default；仅 "dual" 升到 FornaxEvalGeneral。
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

	t.Run("dual mode without run_mode_config -> legacy tenant", func(t *testing.T) {
		assert.Equal(t, rpc.SandboxTenantFornaxTraeEvalDualSandbox, sandboxTenantForExperimentDTO(dtoWithMode(domain_eval_target.SandboxCountModeDual)))
	})

	t.Run("dual mode with valid run_mode -> new tenant", func(t *testing.T) {
		expt := dtoWithMode(domain_eval_target.SandboxCountModeDual)
		expt.RunModeConfig = &domain_expt.RunModeConfig{RunMode: domain_expt.ExptRunModePtr(domain_expt.ExptRunMode_SuaMultiTurn)}
		assert.Equal(t, rpc.SandboxTenantFornaxEvalGeneral, sandboxTenantForExperimentDTO(expt))
	})

	// ★ config 在但 run_mode 未设置 → 旧链路。透传丢字段的形态，不能误判成新链路。
	t.Run("dual mode with run_mode unset -> legacy tenant", func(t *testing.T) {
		expt := dtoWithMode(domain_eval_target.SandboxCountModeDual)
		expt.RunModeConfig = &domain_expt.RunModeConfig{}
		assert.Equal(t, rpc.SandboxTenantFornaxTraeEvalDualSandbox, sandboxTenantForExperimentDTO(expt))
	})

	// ★ run_mode 是枚举合法集外的值 → 旧链路（不能靠 convertor 的 default 回落 single_turn 判断）。
	t.Run("dual mode with invalid run_mode enum -> legacy tenant", func(t *testing.T) {
		expt := dtoWithMode(domain_eval_target.SandboxCountModeDual)
		expt.RunModeConfig = &domain_expt.RunModeConfig{RunMode: domain_expt.ExptRunModePtr(domain_expt.ExptRunMode(999))}
		assert.Equal(t, rpc.SandboxTenantFornaxTraeEvalDualSandbox, sandboxTenantForExperimentDTO(expt))
	})

	t.Run("unrecognized mode -> Default", func(t *testing.T) {
		assert.Equal(t, rpc.SandboxTenantDefault, sandboxTenantForExperimentDTO(dtoWithMode("triple")))
	})

	// mac_vm_plus_sandbox / mac_vm_plus_ssh → GUI 专用租户（无需 run_mode_config）。
	t.Run("mac_vm_plus_sandbox -> GUI tenant", func(t *testing.T) {
		assert.Equal(t, rpc.SandboxTenantFornaxEvalGeneralGUI, sandboxTenantForExperimentDTO(dtoWithMode(string(entity.SandboxCountModeMacVMPlusSandbox))))
	})

	t.Run("mac_vm_plus_ssh -> GUI tenant", func(t *testing.T) {
		assert.Equal(t, rpc.SandboxTenantFornaxEvalGeneralGUI, sandboxTenantForExperimentDTO(dtoWithMode(string(entity.SandboxCountModeMacVMPlusSSH))))
	})
}

// TestSandboxTaskConcurrencyForMode 钉住 mode → task 粒度 execution 配额。
// mac_vm_plus_ssh 不能按 GUI tenant 整体翻倍：基础 task 有 ssh+orch 两个 execution，
// -macvm task 每 item 仍只有一个；mac_vm_plus_sandbox 则两个 task 都只有一个。
func TestSandboxTaskConcurrencyForMode(t *testing.T) {
	itemConcurNum := gptr.Of(7)
	cases := []struct {
		name        string
		mode        entity.SandboxCountMode
		wantSandbox int32
		wantMacVM   int32
	}{
		{name: "single", mode: entity.SandboxCountModeSingle, wantSandbox: 35, wantMacVM: 35},
		{name: "dual", mode: entity.SandboxCountModeDual, wantSandbox: 70, wantMacVM: 35},
		{name: "mac vm plus sandbox", mode: entity.SandboxCountModeMacVMPlusSandbox, wantSandbox: 35, wantMacVM: 35},
		{name: "mac vm plus ssh", mode: entity.SandboxCountModeMacVMPlusSSH, wantSandbox: 70, wantMacVM: 35},
		{name: "unknown falls back to single", mode: entity.SandboxCountMode("triple"), wantSandbox: 35, wantMacVM: 35},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got := sandboxTaskConcurrencyForMode(itemConcurNum, c.mode)
			assert.Equal(t, c.wantSandbox, got.sandbox)
			assert.Equal(t, c.wantMacVM, got.macVM)
		})
	}
}

// TestIsValidExptSuaModeDTO 钉住入口 sua_mode 合法集。
//
// Fixed **必须算合法**: 它是已废弃但仍受理的值 (等价降级为 fixed_script_multi_turn),
// 与"拼错/未来新增值"不同, 不能在入口拒掉 —— 否则存量提交会被误拒。
func TestIsValidExptSuaModeDTO(t *testing.T) {
	assert.True(t, isValidExptSuaModeDTO(domain_expt.SuaMode_HumanLoop))
	assert.True(t, isValidExptSuaModeDTO(domain_expt.SuaMode_Loop))
	assert.True(t, isValidExptSuaModeDTO(domain_expt.SuaMode_Fixed), "Fixed 已废弃但仍受理, 入口不能拒")
	assert.False(t, isValidExptSuaModeDTO(domain_expt.SuaMode(0)), "0 = 未设置, 非合法枚举值")
	assert.False(t, isValidExptSuaModeDTO(domain_expt.SuaMode(999)))
}

// TestCreateExperimentRunModeConfigEntryGate 钉住 **CreateExperiment 入口**对 run_mode_config
// 枚举的拒绝行为。
//
// ⚠️ 本用例必须真的调用 CreateExperiment, 不能只测 isValidExptRunModeDTO / isValidExptSuaModeDTO
// 这几个纯函数 —— 那些函数在入口校验加进来之前就已存在且行为不变, 断言它们**摘掉整段 gate
// 依然全绿**, 拦不住任何人把闸删掉 (本用例的前身就犯了这个错)。判定标准只有一条:
// **把 CreateExperiment 里那段 gate 删掉, 本用例必须转红。**
//
// 校验的必要性: 非法 run_mode 整数若被放进来, 会被 convertor 的 suaRunModeDTO2DO default
// 静默回落成 single_turn 落库, 于是 Submit 侧按"非法"判旧链路租户(3)、Retry 侧从 eval_conf
// 读出合法的 single_turn 判新链路租户(4), 两次 Init 租户不一致 → 沙箱侧拒绝
// "cannot change tenant of active task"。
//
// mock 一律不设期望: 这段 gate 在所有 mock 调用之前早失败 (与同文件 eval_set_source_type
// 那组校验同一位置), 真走到下游就说明 gate 没生效。
func TestCreateExperimentRunModeConfigEntryGate(t *testing.T) {
	const workspaceID int64 = 123

	newReq := func(rmc *domain_expt.RunModeConfig) *exptpb.CreateExperimentRequest {
		return &exptpb.CreateExperimentRequest{
			WorkspaceID:   workspaceID,
			RunModeConfig: rmc,
		}
	}

	t.Run("invalid run_mode enum -> rejected at entry", func(t *testing.T) {
		app := &experimentApplication{}
		_, err := app.CreateExperiment(context.Background(),
			newReq(&domain_expt.RunModeConfig{RunMode: domain_expt.ExptRunModePtr(domain_expt.ExptRunMode(999))}))

		assert.Error(t, err, "非法 run_mode 必须在入口被拒, 否则会静默回落成 single_turn 落库")
		statusErr, ok := errorx.FromStatusError(err)
		assert.True(t, ok)
		assert.Equal(t, int32(errno.CommonInvalidParamCode), statusErr.Code())
		assert.Contains(t, err.Error(), "invalid run_mode")
	})

	t.Run("invalid sua_mode enum -> rejected at entry", func(t *testing.T) {
		app := &experimentApplication{}
		_, err := app.CreateExperiment(context.Background(),
			newReq(&domain_expt.RunModeConfig{SuaMode: domain_expt.SuaModePtr(domain_expt.SuaMode(999))}))

		assert.Error(t, err)
		statusErr, ok := errorx.FromStatusError(err)
		assert.True(t, ok)
		assert.Equal(t, int32(errno.CommonInvalidParamCode), statusErr.Code())
		assert.Contains(t, err.Error(), "invalid sua_mode")
	})

	// 合法值不能被这道闸拦住。它们会因为缺 evaluator/target 等在**更靠后**的地方失败,
	// 所以这里不断言 NoError, 只断言"失败原因不是 invalid run_mode/sua_mode"
	// —— 否则一个过度收紧的 gate (例如把 Fixed 或某个新枚举值也拒了) 就会溜过去。
	t.Run("valid enums must pass this gate", func(t *testing.T) {
		valid := []*domain_expt.RunModeConfig{
			{RunMode: domain_expt.ExptRunModePtr(domain_expt.ExptRunMode_SingleTurn)},
			{RunMode: domain_expt.ExptRunModePtr(domain_expt.ExptRunMode_FixedScriptMultiTurn)},
			{RunMode: domain_expt.ExptRunModePtr(domain_expt.ExptRunMode_SuaMultiTurn)},
			{RunMode: domain_expt.ExptRunModePtr(domain_expt.ExptRunMode_Goal)},
			{SuaMode: domain_expt.SuaModePtr(domain_expt.SuaMode_HumanLoop)},
			{SuaMode: domain_expt.SuaModePtr(domain_expt.SuaMode_Loop)},
			// Fixed 已废弃但**仍受理** (等价降级 fixed_script_multi_turn), 入口不能拒。
			{SuaMode: domain_expt.SuaModePtr(domain_expt.SuaMode_Fixed)},
			// run_mode_config 非 nil 但两个字段都没设 = 老调用方形态, 同样不能拒。
			{},
		}
		for _, rmc := range valid {
			assertPassesEntryGate(t, newReq(rmc))
		}
	})

	// run_mode_config 完全不传 = 绝大多数存量调用, 不能因为这道闸多出任何失败。
	t.Run("nil run_mode_config must not be rejected by this gate", func(t *testing.T) {
		assertPassesEntryGate(t, newReq(nil))
	})
}

// assertPassesEntryGate 断言请求**没有被 run_mode_config 那道入口闸拒掉**。
//
// 为什么要 recover: 这里刻意用零值 experimentApplication (依赖全 nil), 请求一旦越过入口闸,
// 就会在下游 (resolveEvaluatorVersionIDs / manager.CreateExpt 等) 空指针 panic。
// **panic 恰恰是"通过了这道闸"的证据** —— 它证明失败点在闸之后, 而不是闸本身拒的。
// 反过来若闸误拒了合法值, 这里拿到的是 error 而非 panic, 断言随即失败。
//
// 这样写是为了不给本用例接一整套 mock: 本用例只关心"闸放不放行", 下游能否跑通由
// experiment_app_test.go 的 TestCreateExperiment 覆盖。
//
// ⚠️ **给后人的路标**: 本函数依赖"越过闸后下游第一个动作恰好会 nil panic"这个实现细节。
// 若哪天有人在 CreateExperiment 前段加了新的早退分支 (例如某个 nil 检查提前 return error),
// 这几条合法值就会从"绿"变成报「合法值被入口误拒」—— 而失败信息指向的是闸, 排查方向就偏了。
// **本用例开始报"合法值被拒"时, 先确认是不是新增了前置早退分支, 再怀疑闸本身。**
func assertPassesEntryGate(t *testing.T, req *exptpb.CreateExperimentRequest) {
	t.Helper()
	defer func() {
		// 越过闸后在下游 panic = 通过。吞掉它。
		_ = recover()
	}()
	app := &experimentApplication{}
	_, err := app.CreateExperiment(context.Background(), req)
	if err != nil {
		assert.NotContains(t, err.Error(), "invalid run_mode", "合法值被入口误拒: %+v", req.GetRunModeConfig())
		assert.NotContains(t, err.Error(), "invalid sua_mode", "合法值被入口误拒: %+v", req.GetRunModeConfig())
	}
}
