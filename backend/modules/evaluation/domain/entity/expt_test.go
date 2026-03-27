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

func TestCreateEvalTargetParam_IsNull(t *testing.T) {
	assert.True(t, ((*CreateEvalTargetParam)(nil)).IsNull())
	assert.True(t, (&CreateEvalTargetParam{}).IsNull())
	assert.False(t, (&CreateEvalTargetParam{EvalTargetType: gptr.Of(EvalTargetTypeCozeLoopPromptOnline)}).IsNull())
	s := "x"
	assert.False(t, (&CreateEvalTargetParam{SourceTargetID: &s}).IsNull())
}
