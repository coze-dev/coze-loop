// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package entity

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/stretchr/testify/assert"
)

func TestExperiment_ToEvaluatorRefDO(t *testing.T) {
	e := &Experiment{
		ID:      1,
		SpaceID: 2,
		EvaluatorVersionRef: []*ExptEvaluatorVersionRef{
			{EvaluatorID: 3, EvaluatorVersionID: 4},
		},
	}
	refs := e.ToEvaluatorRefDO()
	assert.Len(t, refs, 1)
	assert.Equal(t, int64(3), refs[0].EvaluatorID)
	assert.Equal(t, int64(4), refs[0].EvaluatorVersionID)
	assert.Equal(t, int64(1), refs[0].ExptID)
	assert.Equal(t, int64(2), refs[0].SpaceID)

	// nil case
	var e2 *Experiment
	assert.Nil(t, e2.ToEvaluatorRefDO())
}

func TestExptEvaluatorVersionRef_String(t *testing.T) {
	ref := &ExptEvaluatorVersionRef{EvaluatorID: 1, EvaluatorVersionID: 2}
	str := ref.String()
	assert.Contains(t, str, "evaluator_id=")
	assert.Contains(t, str, "evaluator_version_id=")
}

func TestTargetConf_Valid(t *testing.T) {
	ctx := context.Background()
	// 合法
	conf := &TargetConf{
		TargetVersionID: 1,
		IngressConf: &TargetIngressConf{
			EvalSetAdapter: &FieldAdapter{FieldConfs: []*FieldConf{{}}},
		},
	}
	err := conf.Valid(ctx, EvalTargetTypeLoopPrompt)
	assert.NoError(t, err)
	// 非法
	conf = &TargetConf{}
	assert.Error(t, conf.Valid(ctx, EvalTargetTypeCozeBot))
}

func TestEvaluatorsConf_Valid_GetEvaluatorConf_GetEvaluatorConcurNum(t *testing.T) {
	ctx := context.Background()
	conf := &EvaluatorsConf{
		EvaluatorConcurNum: nil,
		EvaluatorConf:      []*EvaluatorConf{{EvaluatorVersionID: 1, IngressConf: &EvaluatorIngressConf{TargetAdapter: &FieldAdapter{}, EvalSetAdapter: &FieldAdapter{}}}},
	}
	assert.NoError(t, conf.Valid(ctx))
	assert.NotNil(t, conf.GetEvaluatorConf(1))
	assert.Equal(t, 3, conf.GetEvaluatorConcurNum())
	// 并发数自定义
	val := 5
	conf.EvaluatorConcurNum = &val
	assert.Equal(t, 5, conf.GetEvaluatorConcurNum())
	// 无法通过校验
	conf.EvaluatorConf[0].IngressConf = nil
	assert.Error(t, conf.Valid(ctx))
}

func TestEvaluatorConf_Valid(t *testing.T) {
	ctx := context.Background()
	conf := &EvaluatorConf{EvaluatorVersionID: 1, IngressConf: &EvaluatorIngressConf{TargetAdapter: &FieldAdapter{}, EvalSetAdapter: &FieldAdapter{}}}
	assert.NoError(t, conf.Valid(ctx))
	conf = &EvaluatorConf{}
	assert.Error(t, conf.Valid(ctx))
}

func TestExptUpdateFields_ToFieldMap(t *testing.T) {
	fields := &ExptUpdateFields{Name: "n", Desc: "d"}
	_, err := fields.ToFieldMap()
	assert.NoError(t, err)
}

func TestExptErrCtrl_ConvertErrMsg_GetErrRetryCtrl(t *testing.T) {
	ctrl := &ExptErrCtrl{
		ResultErrConverts: []*ResultErrConvert{{MatchedText: "foo", ToErrMsg: "bar"}},
		SpaceErrRetryCtrl: map[int64]*ErrRetryCtrl{1: {RetryConf: &RetryConf{RetryTimes: 2}}},
		ErrRetryCtrl:      &ErrRetryCtrl{RetryConf: &RetryConf{RetryTimes: 1}},
	}
	assert.Equal(t, "bar", ctrl.ConvertErrMsg("foo"))
	assert.Equal(t, "", ctrl.ConvertErrMsg("baz"))
	assert.Equal(t, 2, ctrl.GetErrRetryCtrl(1).RetryConf.RetryTimes)
	assert.Equal(t, 1, ctrl.GetErrRetryCtrl(2).RetryConf.RetryTimes)
}

func TestResultErrConvert_ConvertErrMsg(t *testing.T) {
	c := &ResultErrConvert{MatchedText: "foo", ToErrMsg: "bar"}
	ok, msg := c.ConvertErrMsg("foo")
	assert.True(t, ok)
	assert.Equal(t, "bar", msg)
	ok, _ = c.ConvertErrMsg("baz")
	assert.False(t, ok)
}

func TestRetryConf_GetRetryTimes_GetRetryInterval(t *testing.T) {
	conf := &RetryConf{RetryTimes: 3, RetryIntervalSecond: 2}
	assert.Equal(t, 3, conf.GetRetryTimes())
	assert.Equal(t, 2*time.Second, conf.GetRetryInterval())
	conf = &RetryConf{}
	assert.Equal(t, 0, conf.GetRetryTimes())
	assert.Equal(t, 20*time.Second, conf.GetRetryInterval())
}

func TestQuotaSpaceExpt_Serialize(t *testing.T) {
	q := &QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{1: 123}}
	b, err := q.Serialize()
	assert.NoError(t, err)
	assert.NotNil(t, b)
}

func TestExperiment_AsyncCallTarget_WebAgent(t *testing.T) {
	tests := []struct {
		name     string
		expt     *Experiment
		expected bool
	}{
		{
			name:     "nil实验返回false",
			expt:     nil,
			expected: false,
		},
		{
			name:     "nil Target返回false",
			expt:     &Experiment{Target: nil},
			expected: false,
		},
		{
			name: "WebAgent设置返回true",
			expt: &Experiment{
				Target: &EvalTarget{
					EvalTargetVersion: &EvalTargetVersion{
						WebAgent: &WebAgent{ID: 1, Name: "test-web-agent"},
					},
				},
			},
			expected: true,
		},
		{
			name: "CustomRPCServer异步IsAsync=true返回true",
			expt: &Experiment{
				Target: &EvalTarget{
					EvalTargetVersion: &EvalTargetVersion{
						CustomRPCServer: &CustomRPCServer{IsAsync: gptr.Of(true)},
					},
				},
			},
			expected: true,
		},
		{
			name: "无WebAgent且非异步CustomRPCServer返回false",
			expt: &Experiment{
				Target: &EvalTarget{
					EvalTargetVersion: &EvalTargetVersion{
						CustomRPCServer: &CustomRPCServer{IsAsync: gptr.Of(false)},
					},
				},
			},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.expt.AsyncCallTarget())
		})
	}
}

func TestTargetConf_Valid_WebAgent(t *testing.T) {
	ctx := context.Background()
	conf := &TargetConf{
		TargetVersionID: 1,
	}
	err := conf.Valid(ctx, EvalTargetTypeWebAgent)
	assert.NoError(t, err)
}

// TestExperiment_AsyncCallTarget_SandboxAgent 验证 SandboxAgent 评测对象走异步分支:
// EvalTargetType 为 SandboxAgent 或 SandboxAgent 配置非空时, AsyncCallTarget 应返回 true。
func TestExperiment_AsyncCallTarget_SandboxAgent(t *testing.T) {
	tests := []struct {
		name     string
		expt     *Experiment
		expected bool
	}{
		{
			name: "EvalTargetType 为 SandboxAgent 返回 true",
			expt: &Experiment{
				Target: &EvalTarget{
					EvalTargetVersion: &EvalTargetVersion{
						EvalTargetType: EvalTargetTypeSandboxAgent,
					},
				},
			},
			expected: true,
		},
		{
			name: "SandboxAgent 字段非空但类型不一致也返回 true",
			expt: &Experiment{
				Target: &EvalTarget{
					EvalTargetVersion: &EvalTargetVersion{
						SandboxAgent: &SandboxAgent{Name: "s"},
					},
				},
			},
			expected: true,
		},
		{
			name: "非 SandboxAgent 且 SandboxAgent 字段为 nil 返回 false",
			expt: &Experiment{
				Target: &EvalTarget{
					EvalTargetVersion: &EvalTargetVersion{
						EvalTargetType: EvalTargetTypeCozeBot,
					},
				},
			},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.expt.AsyncCallTarget())
		})
	}
}

func TestVisibility_Hidden(t *testing.T) {
	assert.Equal(t, Visibility(1), Visibility_Hidden)
}

func TestSourceType_IntelligentGen(t *testing.T) {
	assert.Equal(t, SourceType(4), SourceType_IntelligentGen)
}

func TestExperiment_AsyncExec(t *testing.T) {
	tests := []struct {
		name     string
		expt     *Experiment
		expected bool
	}{
		{
			name:     "nil实验返回false",
			expt:     nil,
			expected: false,
		},
		{
			name: "AsyncCallTarget为true返回true",
			expt: &Experiment{
				Target: &EvalTarget{
					EvalTargetVersion: &EvalTargetVersion{
						WebAgent: &WebAgent{ID: 1, Name: "agent"},
					},
				},
			},
			expected: true,
		},
		{
			name: "AsyncCallEvaluators为true返回true",
			expt: &Experiment{
				Evaluators: []*Evaluator{
					{EvaluatorType: EvaluatorTypeAgent},
				},
			},
			expected: true,
		},
		{
			name: "Both false返回false",
			expt: &Experiment{
				Target: &EvalTarget{
					EvalTargetVersion: &EvalTargetVersion{},
				},
				Evaluators: []*Evaluator{
					{EvaluatorType: EvaluatorTypePrompt},
				},
			},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.expt.AsyncExec())
		})
	}
}

func TestTargetConf_Valid_MoreBranches(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name       string
		conf       *TargetConf
		targetType EvalTargetType
		wantErr    bool
	}{
		{
			name:       "TargetVersionID为0返回错误",
			conf:       &TargetConf{TargetVersionID: 0},
			targetType: EvalTargetTypeCozeBot,
			wantErr:    true,
		},
		{
			name:       "LoopPrompt类型无需IngressConf",
			conf:       &TargetConf{TargetVersionID: 1},
			targetType: EvalTargetTypeLoopPrompt,
			wantErr:    false,
		},
		{
			name:       "CustomRPCServer类型无需IngressConf",
			conf:       &TargetConf{TargetVersionID: 1},
			targetType: EvalTargetTypeCustomRPCServer,
			wantErr:    false,
		},
		{
			name: "IngressConf的EvalSetAdapter为nil返回错误",
			conf: &TargetConf{
				TargetVersionID: 1,
				IngressConf:     &TargetIngressConf{EvalSetAdapter: nil},
			},
			targetType: EvalTargetTypeCozeBot,
			wantErr:    true,
		},
		{
			name: "有效IngressConf返回nil",
			conf: &TargetConf{
				TargetVersionID: 1,
				IngressConf: &TargetIngressConf{
					EvalSetAdapter: &FieldAdapter{FieldConfs: []*FieldConf{{}}},
				},
			},
			targetType: EvalTargetTypeCozeBot,
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.conf.Valid(ctx, tt.targetType)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWithOperationInstruction(t *testing.T) {
	instruction := "test instruction"
	opt := &Opt{}
	WithOperationInstruction(&instruction)(opt)
	assert.Equal(t, &instruction, opt.OperationInstruction)
}

func TestWithOptions(t *testing.T) {
	opt := &Opt{}

	pv := "v1"
	WithCozeBotPublishVersion(&pv)(opt)
	assert.Equal(t, &pv, opt.PublishVersion)

	WithCozeBotInfoType(CozeBotInfoType(1))(opt)
	assert.Equal(t, CozeBotInfoType(1), opt.BotInfoType)

	ct := &CustomEvalTarget{ID: gptr.Of("1")}
	WithCustomEvalTarget(ct)(opt)
	assert.Equal(t, ct, opt.CustomEvalTarget)

	region := Region("us-east")
	WithRegion(&region)(opt)
	assert.Equal(t, &region, opt.Region)

	env := "prod"
	WithEnv(&env)(opt)
	assert.Equal(t, &env, opt.Env)

	instruction := "do something"
	WithOperationInstruction(&instruction)(opt)
	assert.Equal(t, &instruction, opt.OperationInstruction)
}

func TestCreateEvalTargetParam_IsNull(t *testing.T) {
	assert.True(t, ((*CreateEvalTargetParam)(nil)).IsNull())
	assert.True(t, (&CreateEvalTargetParam{}).IsNull())
	assert.False(t, (&CreateEvalTargetParam{EvalTargetType: gptr.Of(EvalTargetTypeCozeLoopPromptOnline)}).IsNull())
	s := "x"
	assert.False(t, (&CreateEvalTargetParam{SourceTargetID: &s}).IsNull())
}

func TestGetItemIDs(t *testing.T) {
	tests := []struct {
		name     string
		runLog   *ExptRunLog
		expected []int64
	}{
		{
			name:     "空ItemIds返回nil",
			runLog:   &ExptRunLog{},
			expected: nil,
		},
		{
			name: "单个chunk",
			runLog: &ExptRunLog{
				ItemIds: []ExptRunLogItems{
					{ItemIDs: []int64{1, 2, 3}},
				},
			},
			expected: []int64{1, 2, 3},
		},
		{
			name: "多个chunk合并",
			runLog: &ExptRunLog{
				ItemIds: []ExptRunLogItems{
					{ItemIDs: []int64{1, 2}},
					{ItemIDs: []int64{3, 4}},
				},
			},
			expected: []int64{1, 2, 3, 4},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.runLog.GetItemIDs())
		})
	}
}

func TestAppendItemIDs(t *testing.T) {
	tests := []struct {
		name    string
		runLog  *ExptRunLog
		input   []int64
		wantErr bool
	}{
		{
			name:    "nil接收者返回错误",
			runLog:  nil,
			input:   []int64{1},
			wantErr: true,
		},
		{
			name:    "正常追加",
			runLog:  &ExptRunLog{},
			input:   []int64{1, 2},
			wantErr: false,
		},
		{
			name: "重复ID返回错误",
			runLog: &ExptRunLog{
				ItemIds: []ExptRunLogItems{
					{ItemIDs: []int64{1, 2}},
				},
			},
			input:   []int64{2, 3},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.runLog.AppendItemIDs(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				ids := tt.runLog.GetItemIDs()
				assert.Equal(t, tt.input, ids)
			}
		})
	}
}

func TestContainsEvalTarget(t *testing.T) {
	tests := []struct {
		name     string
		expt     *Experiment
		expected bool
	}{
		{
			name:     "nil实验返回false",
			expt:     nil,
			expected: false,
		},
		{
			name:     "TargetVersionID为0返回false",
			expt:     &Experiment{TargetVersionID: 0},
			expected: false,
		},
		{
			name:     "TargetVersionID大于0返回true",
			expt:     &Experiment{TargetVersionID: 1},
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.expt.ContainsEvalTarget())
		})
	}
}

func TestValidateExperimentName(t *testing.T) {
	maxLenValid := strings.Repeat("a", MaxExperimentNameLength)
	overLen := strings.Repeat("a", MaxExperimentNameLength+1)

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// 合法
		{"single letter", "a", false},
		{"letters and digits", "exp1", false},
		{"chinese", "实验", false},
		{"chinese mixed digits", "实验1", false},
		{"underscore", "exp_1", false},
		{"dash inside", "exp-1", false},
		{"dot inside", "exp.1", false},
		{"max length 50", maxLenValid, false},
		{"all allowed punctuation inside", "Aa1_-.b", false},

		// 长度不合法
		{"empty", "", true},
		{"length 51", overLen, true},

		// 首字符不合法
		{"leading underscore", "_exp", true},
		{"leading dash", "-exp", true},
		{"leading dot", ".exp", true},

		// 字符集不合法（典型 oncall case）
		{"bracket pair", "exp[]", true},
		{"slash", "exp/sub", true},
		{"space", "exp name", true},
		{"colon", "exp:1", true},
		{"comma", "exp,1", true},
		{"asterisk", "exp*1", true},
		{"emoji", "exp🚀", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExperimentName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateExperimentName(%q) err=%v, wantErr=%v", tt.input, err, tt.wantErr)
			}
			if err == nil {
				return
			}
			serr, ok := errorx.FromStatusError(err)
			assert.True(t, ok, "expect status error, got %v", err)
			assert.Equal(t, int32(errno.ExperimentNameInvalidFormatCode), serr.Code())
		})
	}
}

// TestRunModeToInt 钉住 case-file experiment_info.run_mode 的整数编号映射。
// 编号必须与评测运行时解析 case-file 的那套编号逐一一致 —— 错一位就是
// 整个实验按另一个跑法执行, 而实验照样 success, 只能靠人肉比对轨迹才能发现。
//
// 反向验证 (证明本测试真的在守): 把 RunModeToInt 里 goal 的返回值从 5 改回 4,
// 本测试 FAIL —— run_mode="goal" expected 5, actual 4。已还原。
func TestRunModeToInt(t *testing.T) {
	cases := map[RunMode]int{
		RunModeSingleTurn:            1,
		RunModeFixedScriptMultiTurn:  2,
		RunModeSUALoopMultiTurn:      3,
		RunModeSUAHumanLoopMultiTurn: 4,
		RunModeGoal:                  5,
		// 平台对外契约的聚合态: 跑法藏在 sua_mode 子字段里, 一个整数表达不了, 故没有独立
		// 编号。下发前必须由 commercial runtimeRunModeInt 折叠成 3/4; 漏折叠则落 default。
		RunModeSUAMultiTurn: 1,
		"unknown":           1,
		"":                  1,
	}
	for m, want := range cases {
		assert.Equal(t, want, RunModeToInt(m), "run_mode=%q", m)
	}
}

// TestRunModeToInt_NoDuplicateCodes 五个 runtime 跑法必须占五个互不相同的编号 ——
// 撞号意味着两个跑法在 wire 上不可区分 (这正是 goal 必须从 4 挪到 5 的原因)。
func TestRunModeToInt_NoDuplicateCodes(t *testing.T) {
	modes := []RunMode{
		RunModeSingleTurn,
		RunModeFixedScriptMultiTurn,
		RunModeSUALoopMultiTurn,
		RunModeSUAHumanLoopMultiTurn,
		RunModeGoal,
	}
	seen := make(map[int]RunMode, len(modes))
	for _, m := range modes {
		code := RunModeToInt(m)
		prev, dup := seen[code]
		assert.False(t, dup, "run_mode %q 与 %q 撞了同一个编号 %d", m, prev, code)
		seen[code] = m
	}
	assert.Len(t, seen, len(modes))
}

// TestSuaModeLiteralsMatchOpenAPIIDL 钉住 sua_mode 三个常量的**字面量**, 使其与 OpenAPI IDL
// 逐字一致 (cozeloop-idl-commercial open/coze/loop/evaluation/domain_openapi/experiment.thrift:
// SuaMode_HumanLoop = "human_loop" / SuaMode_Loop = "loop" / SuaMode_Fixed = "fixed";
// 生成码见 kitex_gen/.../domain_openapi/experiment.SuaModeHumanLoop)。
//
// 为什么必须对**裸字符串**断言: 全链路其它比对点 (commercial runtimeRunMode 等) 一律经
// SuaModeXxx 常量取值, 常量改成什么它们都照样过 —— 唯独察觉不到"常量与 IDL 漂了"。而这正是
// 本次修的 bug: 常量曾是无下划线的 "humanloop", 与 IDL 的 "human_loop" 不一致, 靠
// OpenAPI 字符串→domain int→entity 字符串的**往返**把差异吃掉 (openAPISuaModeToDomain +
// suaModeDTO2DO), 于是落库 eval_conf.run_mode_config.sua_mode 成了 "humanloop"
// (实测实验 7590112179985613570)。统一后往返两端同词, 无隐式归一。
//
// 反向验证 (证明本测试真的在守): 把 SuaModeHumanLoop 改回 "humanloop", 本测试 FAIL
// (expected "human_loop", actual "humanloop")。已还原。
func TestSuaModeLiteralsMatchOpenAPIIDL(t *testing.T) {
	assert.Equal(t, "human_loop", SuaModeHumanLoop,
		"sua_mode 以对外契约 (OpenAPI IDL) 为准; 旧值 humanloop 已废弃且刻意不兼容")
	assert.Equal(t, "loop", SuaModeLoop)
	assert.Equal(t, "fixed", SuaModeFixed)

	// 旧值不得再作为任何常量的值复活 —— 兼容双值会掩盖"某处仍在用旧值"这类真问题。
	for _, m := range []SuaMode{SuaModeHumanLoop, SuaModeLoop, SuaModeFixed} {
		assert.NotEqual(t, "humanloop", m, "旧值 humanloop 不该再出现在合法值集合里")
	}
}
