// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package entity

import (
	"context"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
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
