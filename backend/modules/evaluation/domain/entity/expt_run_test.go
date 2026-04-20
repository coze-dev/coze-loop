// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/ctxcache"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestIsExptFinishing(t *testing.T) {
	tests := []struct {
		name   string
		status ExptStatus
		want   bool
	}{
		{
			name:   "terminating status should return true",
			status: ExptStatus_Terminating,
			want:   true,
		},
		{
			name:   "draining status should return true",
			status: ExptStatus_Draining,
			want:   true,
		},
		{
			name:   "processing status should return false",
			status: ExptStatus_Processing,
			want:   false,
		},
		{
			name:   "pending status should return false",
			status: ExptStatus_Pending,
			want:   false,
		},
		{
			name:   "success status should return false",
			status: ExptStatus_Success,
			want:   false,
		},
		{
			name:   "failed status should return false",
			status: ExptStatus_Failed,
			want:   false,
		},
		{
			name:   "terminated status should return false",
			status: ExptStatus_Terminated,
			want:   false,
		},
		{
			name:   "system terminated status should return false",
			status: ExptStatus_SystemTerminated,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsExptFinishing(tt.status)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExptTurnRunResult_AbortWithTargetResult(t *testing.T) {
	tests := []struct {
		name            string
		turnRunResult   *ExptTurnRunResult
		experiment      *Experiment
		expectedAbort   bool
		expectedErr     bool
		expectedErrMsg  string
		checkAsyncAbort bool
	}{
		{
			name: "TargetResult为nil，应该中止并设置错误",
			turnRunResult: &ExptTurnRunResult{
				TargetResult: nil,
			},
			experiment: &Experiment{
				Target: &EvalTarget{
					EvalTargetVersion: &EvalTargetVersion{
						CustomRPCServer: &CustomRPCServer{
							IsAsync: gptr.Of(false),
						},
					},
				},
			},
			expectedAbort:  true,
			expectedErr:    true,
			expectedErrMsg: "target result is nil",
		},
		{
			name: "TargetResult有执行错误，应该中止",
			turnRunResult: &ExptTurnRunResult{
				TargetResult: &EvalTargetRecord{
					EvalTargetOutputData: &EvalTargetOutputData{
						EvalTargetRunError: &EvalTargetRunError{
							Code:    500,
							Message: "execution failed",
						},
					},
				},
			},
			experiment: &Experiment{
				Target: &EvalTarget{
					EvalTargetVersion: &EvalTargetVersion{
						CustomRPCServer: &CustomRPCServer{
							IsAsync: gptr.Of(false),
						},
					},
				},
			},
			expectedAbort: true,
			expectedErr:   false,
		},
		{
			name: "TargetResult无执行错误，非异步调用，不应该中止",
			turnRunResult: &ExptTurnRunResult{
				TargetResult: &EvalTargetRecord{
					EvalTargetOutputData: &EvalTargetOutputData{
						EvalTargetRunError: nil,
					},
					Status: gptr.Of(EvalTargetRunStatusSuccess),
				},
			},
			experiment: &Experiment{
				Target: &EvalTarget{
					EvalTargetVersion: &EvalTargetVersion{
						CustomRPCServer: &CustomRPCServer{
							IsAsync: gptr.Of(false),
						},
					},
				},
			},
			expectedAbort: false,
			expectedErr:   false,
		},
		{
			name: "异步调用且状态为AsyncInvoking，应该中止并设置AsyncAbort",
			turnRunResult: &ExptTurnRunResult{
				TargetResult: &EvalTargetRecord{
					EvalTargetOutputData: &EvalTargetOutputData{
						EvalTargetRunError: nil,
					},
					Status: gptr.Of(EvalTargetRunStatusAsyncInvoking),
				},
			},
			experiment: &Experiment{
				Target: &EvalTarget{
					EvalTargetVersion: &EvalTargetVersion{
						CustomRPCServer: &CustomRPCServer{
							IsAsync: gptr.Of(true),
						},
					},
				},
			},
			expectedAbort:   true,
			expectedErr:     false,
			checkAsyncAbort: true,
		},
		{
			name: "异步调用但状态不是AsyncInvoking，不应该中止",
			turnRunResult: &ExptTurnRunResult{
				TargetResult: &EvalTargetRecord{
					EvalTargetOutputData: &EvalTargetOutputData{
						EvalTargetRunError: nil,
					},
					Status: gptr.Of(EvalTargetRunStatusSuccess),
				},
			},
			experiment: &Experiment{
				Target: &EvalTarget{
					EvalTargetVersion: &EvalTargetVersion{
						CustomRPCServer: &CustomRPCServer{
							IsAsync: gptr.Of(true),
						},
					},
				},
			},
			expectedAbort: false,
			expectedErr:   false,
		},
		{
			name: "非异步调用但状态为AsyncInvoking，不应该中止",
			turnRunResult: &ExptTurnRunResult{
				TargetResult: &EvalTargetRecord{
					EvalTargetOutputData: &EvalTargetOutputData{
						EvalTargetRunError: nil,
					},
					Status: gptr.Of(EvalTargetRunStatusAsyncInvoking),
				},
			},
			experiment: &Experiment{
				Target: &EvalTarget{
					EvalTargetVersion: &EvalTargetVersion{
						CustomRPCServer: &CustomRPCServer{
							IsAsync: gptr.Of(false),
						},
					},
				},
			},
			expectedAbort: false,
			expectedErr:   false,
		},
		{
			name: "Experiment为nil，AsyncCallTarget返回false，不应该中止",
			turnRunResult: &ExptTurnRunResult{
				TargetResult: &EvalTargetRecord{
					EvalTargetOutputData: &EvalTargetOutputData{
						EvalTargetRunError: nil,
					},
					Status: gptr.Of(EvalTargetRunStatusAsyncInvoking),
				},
			},
			experiment:    nil,
			expectedAbort: false,
			expectedErr:   false,
		},
		{
			name: "Experiment.Target为nil，AsyncCallTarget返回false，不应该中止",
			turnRunResult: &ExptTurnRunResult{
				TargetResult: &EvalTargetRecord{
					EvalTargetOutputData: &EvalTargetOutputData{
						EvalTargetRunError: nil,
					},
					Status: gptr.Of(EvalTargetRunStatusAsyncInvoking),
				},
			},
			experiment: &Experiment{
				Target: nil,
			},
			expectedAbort: false,
			expectedErr:   false,
		},
		{
			name: "Experiment.Target.EvalTargetVersion为nil，AsyncCallTarget返回false，不应该中止",
			turnRunResult: &ExptTurnRunResult{
				TargetResult: &EvalTargetRecord{
					EvalTargetOutputData: &EvalTargetOutputData{
						EvalTargetRunError: nil,
					},
					Status: gptr.Of(EvalTargetRunStatusAsyncInvoking),
				},
			},
			experiment: &Experiment{
				Target: &EvalTarget{
					EvalTargetVersion: nil,
				},
			},
			expectedAbort: false,
			expectedErr:   false,
		},
		{
			name: "Experiment.Target.EvalTargetVersion.CustomRPCServer为nil，AsyncCallTarget返回false，不应该中止",
			turnRunResult: &ExptTurnRunResult{
				TargetResult: &EvalTargetRecord{
					EvalTargetOutputData: &EvalTargetOutputData{
						EvalTargetRunError: nil,
					},
					Status: gptr.Of(EvalTargetRunStatusAsyncInvoking),
				},
			},
			experiment: &Experiment{
				Target: &EvalTarget{
					EvalTargetVersion: &EvalTargetVersion{
						CustomRPCServer: nil,
					},
				},
			},
			expectedAbort: false,
			expectedErr:   false,
		},
		{
			name: "EvalTargetOutputData为nil，不应该中止",
			turnRunResult: &ExptTurnRunResult{
				TargetResult: &EvalTargetRecord{
					EvalTargetOutputData: nil,
					Status:               gptr.Of(EvalTargetRunStatusSuccess),
				},
			},
			experiment: &Experiment{
				Target: &EvalTarget{
					EvalTargetVersion: &EvalTargetVersion{
						CustomRPCServer: &CustomRPCServer{
							IsAsync: gptr.Of(false),
						},
					},
				},
			},
			expectedAbort: false,
			expectedErr:   false,
		},
		{
			name: "Status为nil，不应该中止",
			turnRunResult: &ExptTurnRunResult{
				TargetResult: &EvalTargetRecord{
					EvalTargetOutputData: &EvalTargetOutputData{
						EvalTargetRunError: nil,
					},
					Status: nil,
				},
			},
			experiment: &Experiment{
				Target: &EvalTarget{
					EvalTargetVersion: &EvalTargetVersion{
						CustomRPCServer: &CustomRPCServer{
							IsAsync: gptr.Of(true),
						},
					},
				},
			},
			expectedAbort: false,
			expectedErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.turnRunResult.AbortWithTargetResult(tt.experiment)

			assert.Equal(t, tt.expectedAbort, result)

			if tt.expectedErr {
				assert.Error(t, tt.turnRunResult.GetEvalErr())
				if tt.expectedErrMsg != "" {
					assert.Contains(t, tt.turnRunResult.GetEvalErr().Error(), tt.expectedErrMsg)
				}
				statusErr, ok := errorx.FromStatusError(tt.turnRunResult.GetEvalErr())
				assert.True(t, ok)
				assert.Equal(t, int32(errno.CommonInternalErrorCode), statusErr.Code())
			} else {
				assert.NoError(t, tt.turnRunResult.GetEvalErr())
			}

			if tt.checkAsyncAbort {
				assert.True(t, tt.turnRunResult.AsyncAbort)
			} else {
				assert.False(t, tt.turnRunResult.AsyncAbort)
			}
		})
	}
}

func TestExptTurnRunResult_AbortWithEvaluatorResults(t *testing.T) {
	tests := []struct {
		name          string
		evaluatorRes  map[int64]*EvaluatorRecord
		expectedAbort bool
		expectedAsync bool
	}{
		{
			name:          "EvaluatorResults 为 nil 不中止",
			evaluatorRes:  nil,
			expectedAbort: false,
			expectedAsync: false,
		},
		{
			name: "全部成功不中止",
			evaluatorRes: map[int64]*EvaluatorRecord{
				1: {ID: 100, EvaluatorVersionID: 1, Status: EvaluatorRunStatusSuccess},
				2: {ID: 200, EvaluatorVersionID: 2, Status: EvaluatorRunStatusSuccess},
			},
			expectedAbort: false,
			expectedAsync: false,
		},
		{
			name: "存在 AsyncInvoking 中止并标记 AsyncAbort",
			evaluatorRes: map[int64]*EvaluatorRecord{
				1: {ID: 100, EvaluatorVersionID: 1, Status: EvaluatorRunStatusSuccess},
				2: {ID: 200, EvaluatorVersionID: 2, Status: EvaluatorRunStatusAsyncInvoking},
			},
			expectedAbort: true,
			expectedAsync: true,
		},
		{
			name: "包含 nil record 不影响判断",
			evaluatorRes: map[int64]*EvaluatorRecord{
				1: nil,
				2: {ID: 200, EvaluatorVersionID: 2, Status: EvaluatorRunStatusAsyncInvoking},
			},
			expectedAbort: true,
			expectedAsync: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trr := &ExptTurnRunResult{EvaluatorResults: tt.evaluatorRes}
			ctx := ctxcache.Init(context.Background())
			event := &ExptItemEvalEvent{}

			got := trr.AbortWithEvaluatorResults(ctx, event)
			assert.Equal(t, tt.expectedAbort, got)
			assert.Equal(t, tt.expectedAsync, trr.AsyncAbort)
		})
	}
}

func TestExptTurnRunResult_SetEvalErr(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected error
	}{
		{
			name:     "设置nil错误",
			err:      nil,
			expected: nil,
		},
		{
			name:     "设置非nil错误",
			err:      errorx.New("test error"),
			expected: errorx.New("test error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ExptTurnRunResult{}
			result.SetEvalErr(tt.err)

			if tt.expected == nil {
				assert.Nil(t, result.GetEvalErr())
			} else {
				assert.NotNil(t, result.GetEvalErr())
				assert.Contains(t, result.GetEvalErr().Error(), "test error")
			}
		})
	}
}

func TestExptTurnRunResult_SetTargetResult(t *testing.T) {
	tests := []struct {
		name         string
		targetResult *EvalTargetRecord
		expected     *EvalTargetRecord
	}{
		{
			name:         "设置nil TargetResult",
			targetResult: nil,
			expected:     nil,
		},
		{
			name: "设置非nil TargetResult",
			targetResult: &EvalTargetRecord{
				ID:      123,
				SpaceID: 456,
			},
			expected: &EvalTargetRecord{
				ID:      123,
				SpaceID: 456,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ExptTurnRunResult{}
			returned := result.SetTargetResult(tt.targetResult)

			assert.Equal(t, result, returned)

			assert.Equal(t, tt.expected, result.TargetResult)
		})
	}
}

func TestExptTurnRunResult_SetEvaluatorResults(t *testing.T) {
	tests := []struct {
		name             string
		evaluatorResults map[int64]*EvaluatorRecord
		expected         map[int64]*EvaluatorRecord
	}{
		{
			name:             "设置nil EvaluatorResults",
			evaluatorResults: nil,
			expected:         nil,
		},
		{
			name: "设置非nil EvaluatorResults",
			evaluatorResults: map[int64]*EvaluatorRecord{
				1: {ID: 100, EvaluatorVersionID: 1},
				2: {ID: 200, EvaluatorVersionID: 2},
			},
			expected: map[int64]*EvaluatorRecord{
				1: {ID: 100, EvaluatorVersionID: 1},
				2: {ID: 200, EvaluatorVersionID: 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ExptTurnRunResult{}
			returned := result.SetEvaluatorResults(tt.evaluatorResults)

			assert.Equal(t, result, returned)

			assert.Equal(t, tt.expected, result.EvaluatorResults)
		})
	}
}

func TestExptTurnRunResult_GetEvaluatorRecord(t *testing.T) {
	tests := []struct {
		name               string
		turnRunResult      *ExptTurnRunResult
		evaluatorVersionID int64
		expected           *EvaluatorRecord
	}{
		{
			name:               "ExptTurnRunResult为nil",
			turnRunResult:      nil,
			evaluatorVersionID: 1,
			expected:           nil,
		},
		{
			name: "EvaluatorResults为nil",
			turnRunResult: &ExptTurnRunResult{
				EvaluatorResults: nil,
			},
			evaluatorVersionID: 1,
			expected:           nil,
		},
		{
			name: "EvaluatorResults为空map",
			turnRunResult: &ExptTurnRunResult{
				EvaluatorResults: map[int64]*EvaluatorRecord{},
			},
			evaluatorVersionID: 1,
			expected:           nil,
		},
		{
			name: "找到对应的EvaluatorRecord",
			turnRunResult: &ExptTurnRunResult{
				EvaluatorResults: map[int64]*EvaluatorRecord{
					1: {ID: 100, EvaluatorVersionID: 1},
					2: {ID: 200, EvaluatorVersionID: 2},
				},
			},
			evaluatorVersionID: 1,
			expected:           &EvaluatorRecord{ID: 100, EvaluatorVersionID: 1},
		},
		{
			name: "找不到对应的EvaluatorRecord",
			turnRunResult: &ExptTurnRunResult{
				EvaluatorResults: map[int64]*EvaluatorRecord{
					1: {ID: 100, EvaluatorVersionID: 1},
					2: {ID: 200, EvaluatorVersionID: 2},
				},
			},
			evaluatorVersionID: 3,
			expected:           nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result *EvaluatorRecord
			if tt.turnRunResult != nil {
				result = tt.turnRunResult.GetEvaluatorRecord(tt.evaluatorVersionID)
			} else {
				result = (*ExptTurnRunResult)(nil).GetEvaluatorRecord(tt.evaluatorVersionID)
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExptItemEvalConf_GetConcurNum(t *testing.T) {
	tests := []struct {
		name     string
		conf     *ExptItemEvalConf
		expected int
	}{
		{
			name:     "conf为nil，返回默认值",
			conf:     nil,
			expected: defaultItemEvalConcurNum,
		},
		{
			name:     "ConcurNum为0，返回默认值",
			conf:     &ExptItemEvalConf{ConcurNum: 0},
			expected: defaultItemEvalConcurNum,
		},
		{
			name:     "ConcurNum为负数，返回默认值",
			conf:     &ExptItemEvalConf{ConcurNum: -1},
			expected: defaultItemEvalConcurNum,
		},
		{
			name:     "ConcurNum为正数，返回设置值",
			conf:     &ExptItemEvalConf{ConcurNum: 5},
			expected: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result int
			if tt.conf != nil {
				result = tt.conf.GetConcurNum()
			} else {
				result = (*ExptItemEvalConf)(nil).GetConcurNum()
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExptItemEvalConf_GetInterval(t *testing.T) {
	tests := []struct {
		name     string
		conf     *ExptItemEvalConf
		expected time.Duration
	}{
		{
			name:     "conf为nil，返回默认值",
			conf:     nil,
			expected: defaultItemEvalInterval,
		},
		{
			name:     "IntervalSecond为0，返回默认值",
			conf:     &ExptItemEvalConf{IntervalSecond: 0},
			expected: defaultItemEvalInterval,
		},
		{
			name:     "IntervalSecond为负数，返回默认值",
			conf:     &ExptItemEvalConf{IntervalSecond: -1},
			expected: defaultItemEvalInterval,
		},
		{
			name:     "IntervalSecond为正数，返回设置值",
			conf:     &ExptItemEvalConf{IntervalSecond: 30},
			expected: 30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result time.Duration
			if tt.conf != nil {
				result = tt.conf.GetInterval()
			} else {
				result = (*ExptItemEvalConf)(nil).GetInterval()
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExptItemEvalConf_getZombieSecond(t *testing.T) {
	tests := []struct {
		name     string
		conf     *ExptItemEvalConf
		expected int
	}{
		{
			name:     "conf为nil，返回默认值",
			conf:     nil,
			expected: defaultItemZombieSecond,
		},
		{
			name:     "ZombieSecond为0，返回默认值",
			conf:     &ExptItemEvalConf{ZombieSecond: 0},
			expected: defaultItemZombieSecond,
		},
		{
			name:     "ZombieSecond为负数，返回默认值",
			conf:     &ExptItemEvalConf{ZombieSecond: -1},
			expected: defaultItemZombieSecond,
		},
		{
			name:     "ZombieSecond为正数，返回设置值",
			conf:     &ExptItemEvalConf{ZombieSecond: 1800},
			expected: 1800,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result int
			if tt.conf != nil {
				result = tt.conf.getZombieSecond()
			} else {
				result = (*ExptItemEvalConf)(nil).getZombieSecond()
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExptItemEvalConf_getAsyncZombieSecond(t *testing.T) {
	tests := []struct {
		name     string
		conf     *ExptItemEvalConf
		expected int
	}{
		{
			name:     "conf为nil，返回默认值",
			conf:     nil,
			expected: defaultItemAsyncZombieSecond,
		},
		{
			name:     "AsyncZombieSecond为0，返回默认值",
			conf:     &ExptItemEvalConf{AsyncZombieSecond: 0},
			expected: defaultItemAsyncZombieSecond,
		},
		{
			name:     "AsyncZombieSecond为负数，返回默认值",
			conf:     &ExptItemEvalConf{AsyncZombieSecond: -1},
			expected: defaultItemAsyncZombieSecond,
		},
		{
			name:     "AsyncZombieSecond为正数，返回设置值",
			conf:     &ExptItemEvalConf{AsyncZombieSecond: 7200},
			expected: 7200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result int
			if tt.conf != nil {
				result = tt.conf.getAsyncZombieSecond()
			} else {
				result = (*ExptItemEvalConf)(nil).getAsyncZombieSecond()
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExptItemEvalConf_GetItemZombieSecond(t *testing.T) {
	tests := []struct {
		name     string
		conf     *ExptItemEvalConf
		isAsync  bool
		expected int
	}{
		{
			name:     "conf为nil，isAsync为false，返回同步默认值",
			conf:     nil,
			isAsync:  false,
			expected: defaultItemZombieSecond,
		},
		{
			name:     "conf为nil，isAsync为true，返回异步默认值",
			conf:     nil,
			isAsync:  true,
			expected: defaultItemAsyncZombieSecond,
		},
		{
			name:     "conf有值，isAsync为false，返回同步设置值",
			conf:     &ExptItemEvalConf{ZombieSecond: 1800, AsyncZombieSecond: 7200},
			isAsync:  false,
			expected: 1800,
		},
		{
			name:     "conf有值，isAsync为true，返回异步设置值",
			conf:     &ExptItemEvalConf{ZombieSecond: 1800, AsyncZombieSecond: 7200},
			isAsync:  true,
			expected: 7200,
		},
		{
			name:     "conf有值但ZombieSecond为0，isAsync为false，返回同步默认值",
			conf:     &ExptItemEvalConf{ZombieSecond: 0, AsyncZombieSecond: 7200},
			isAsync:  false,
			expected: defaultItemZombieSecond,
		},
		{
			name:     "conf有值但AsyncZombieSecond为0，isAsync为true，返回异步默认值",
			conf:     &ExptItemEvalConf{ZombieSecond: 1800, AsyncZombieSecond: 0},
			isAsync:  true,
			expected: defaultItemAsyncZombieSecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result int
			if tt.conf != nil {
				result = tt.conf.GetItemZombieSecond(tt.isAsync)
			} else {
				result = (*ExptItemEvalConf)(nil).GetItemZombieSecond(tt.isAsync)
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsTurnRunFinished(t *testing.T) {
	tests := []struct {
		name  string
		state TurnRunState
		want  bool
	}{
		{name: "success should return true", state: TurnRunState_Success, want: true},
		{name: "fail should return true", state: TurnRunState_Fail, want: true},
		{name: "terminal should return true", state: TurnRunState_Terminal, want: true},
		{name: "queueing should return false", state: TurnRunState_Queueing, want: false},
		{name: "processing should return false", state: TurnRunState_Processing, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsTurnRunFinished(tt.state))
		})
	}
}

func TestIsExptFinished(t *testing.T) {
	tests := []struct {
		name   string
		status ExptStatus
		want   bool
	}{
		{name: "success should return true", status: ExptStatus_Success, want: true},
		{name: "failed should return true", status: ExptStatus_Failed, want: true},
		{name: "terminated should return true", status: ExptStatus_Terminated, want: true},
		{name: "system terminated should return true", status: ExptStatus_SystemTerminated, want: true},
		{name: "pending should return false", status: ExptStatus_Pending, want: false},
		{name: "processing should return false", status: ExptStatus_Processing, want: false},
		{name: "terminating should return false", status: ExptStatus_Terminating, want: false},
		{name: "draining should return false", status: ExptStatus_Draining, want: false},
		{name: "unknown should return false", status: ExptStatus_Unknown, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsExptFinished(tt.status))
		})
	}
}

func TestIsItemRunFinished(t *testing.T) {
	tests := []struct {
		name  string
		state ItemRunState
		want  bool
	}{
		{name: "success should return true", state: ItemRunState_Success, want: true},
		{name: "fail should return true", state: ItemRunState_Fail, want: true},
		{name: "terminal should return true", state: ItemRunState_Terminal, want: true},
		{name: "queueing should return false", state: ItemRunState_Queueing, want: false},
		{name: "processing should return false", state: ItemRunState_Processing, want: false},
		{name: "unknown should return false", state: ItemRunState_Unknown, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsItemRunFinished(tt.state))
		})
	}
}

func TestExptConsumerConf_GetExptExecConf(t *testing.T) {
	defaultConf := &ExptExecConf{DaemonIntervalSecond: 10}
	spaceConf := &ExptExecConf{DaemonIntervalSecond: 30}

	tests := []struct {
		name     string
		conf     *ExptConsumerConf
		spaceID  int64
		expected *ExptExecConf
	}{
		{
			name:     "conf为nil返回nil",
			conf:     nil,
			spaceID:  1,
			expected: nil,
		},
		{
			name: "space有专属配置返回space配置",
			conf: &ExptConsumerConf{
				ExptExecConf:      defaultConf,
				SpaceExptExecConf: map[int64]*ExptExecConf{100: spaceConf},
			},
			spaceID:  100,
			expected: spaceConf,
		},
		{
			name: "space无专属配置返回默认配置",
			conf: &ExptConsumerConf{
				ExptExecConf:      defaultConf,
				SpaceExptExecConf: map[int64]*ExptExecConf{100: spaceConf},
			},
			spaceID:  999,
			expected: defaultConf,
		},
		{
			name: "SpaceExptExecConf为nil返回默认配置",
			conf: &ExptConsumerConf{
				ExptExecConf: defaultConf,
			},
			spaceID:  1,
			expected: defaultConf,
		},
		{
			name:     "ExptExecConf也为nil返回nil",
			conf:     &ExptConsumerConf{},
			spaceID:  1,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result *ExptExecConf
			if tt.conf != nil {
				result = tt.conf.GetExptExecConf(tt.spaceID)
			} else {
				result = (*ExptConsumerConf)(nil).GetExptExecConf(tt.spaceID)
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExptConsumerConf_GetSchedulerAbortCtrl(t *testing.T) {
	ctrl := &SchedulerAbortCtrl{ExptIDCtrl: map[int64]bool{1: true}}

	tests := []struct {
		name     string
		conf     *ExptConsumerConf
		expected *SchedulerAbortCtrl
	}{
		{
			name:     "conf为nil返回nil",
			conf:     nil,
			expected: nil,
		},
		{
			name:     "SchedulerAbortCtrl为nil返回nil",
			conf:     &ExptConsumerConf{},
			expected: nil,
		},
		{
			name:     "SchedulerAbortCtrl有值返回对应值",
			conf:     &ExptConsumerConf{SchedulerAbortCtrl: ctrl},
			expected: ctrl,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.conf.GetSchedulerAbortCtrl()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSchedulerAbortCtrl_Abort(t *testing.T) {
	tests := []struct {
		name     string
		ctrl     *SchedulerAbortCtrl
		spaceID  int64
		exptID   int64
		userID   string
		exptType ExptType
		want     bool
	}{
		{
			name:     "ctrl为nil不abort",
			ctrl:     nil,
			spaceID:  1,
			exptID:   1,
			userID:   "user1",
			exptType: ExptType_Offline,
			want:     false,
		},
		{
			name: "exptID在ExptIDCtrl中且为true",
			ctrl: &SchedulerAbortCtrl{
				ExptIDCtrl: map[int64]bool{10: true},
			},
			spaceID:  1,
			exptID:   10,
			userID:   "user1",
			exptType: ExptType_Offline,
			want:     true,
		},
		{
			name: "exptID在ExptIDCtrl中但为false",
			ctrl: &SchedulerAbortCtrl{
				ExptIDCtrl: map[int64]bool{10: false},
			},
			spaceID:  1,
			exptID:   10,
			userID:   "user1",
			exptType: ExptType_Offline,
			want:     false,
		},
		{
			name: "exptID不在ExptIDCtrl中",
			ctrl: &SchedulerAbortCtrl{
				ExptIDCtrl: map[int64]bool{10: true},
			},
			spaceID:  1,
			exptID:   20,
			userID:   "user1",
			exptType: ExptType_Offline,
			want:     false,
		},
		{
			name: "space匹配SpaceExptTypeCtrl",
			ctrl: &SchedulerAbortCtrl{
				SpaceExptTypeCtrl: map[int64][]ExptType{100: {ExptType_Offline}},
			},
			spaceID:  100,
			exptID:   1,
			userID:   "user1",
			exptType: ExptType_Offline,
			want:     true,
		},
		{
			name: "space匹配但exptType不匹配",
			ctrl: &SchedulerAbortCtrl{
				SpaceExptTypeCtrl: map[int64][]ExptType{100: {ExptType_Offline}},
			},
			spaceID:  100,
			exptID:   1,
			userID:   "user1",
			exptType: ExptType_Online,
			want:     false,
		},
		{
			name: "user匹配UserExptTypeCtrl",
			ctrl: &SchedulerAbortCtrl{
				UserExptTypeCtrl: map[string][]ExptType{"user1": {ExptType_Online}},
			},
			spaceID:  1,
			exptID:   1,
			userID:   "user1",
			exptType: ExptType_Online,
			want:     true,
		},
		{
			name: "user匹配但exptType不匹配",
			ctrl: &SchedulerAbortCtrl{
				UserExptTypeCtrl: map[string][]ExptType{"user1": {ExptType_Online}},
			},
			spaceID:  1,
			exptID:   1,
			userID:   "user1",
			exptType: ExptType_Offline,
			want:     false,
		},
		{
			name: "ExptIDCtrl优先级最高",
			ctrl: &SchedulerAbortCtrl{
				ExptIDCtrl:        map[int64]bool{10: true},
				SpaceExptTypeCtrl: map[int64][]ExptType{100: {ExptType_Offline}},
				UserExptTypeCtrl:  map[string][]ExptType{"user1": {ExptType_Offline}},
			},
			spaceID:  100,
			exptID:   10,
			userID:   "user1",
			exptType: ExptType_Offline,
			want:     true,
		},
		{
			name:     "所有ctrl为nil不abort",
			ctrl:     &SchedulerAbortCtrl{},
			spaceID:  1,
			exptID:   1,
			userID:   "user1",
			exptType: ExptType_Offline,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.ctrl.Abort(tt.spaceID, tt.exptID, tt.userID, tt.exptType))
		})
	}
}

func TestExptExecConf_GetSpaceExptConcurLimit(t *testing.T) {
	tests := []struct {
		name     string
		conf     *ExptExecConf
		expected int
	}{
		{name: "nil返回默认值", conf: nil, expected: defaultSpaceExptConcurLimit},
		{name: "值为0返回默认值", conf: &ExptExecConf{SpaceExptConcurLimit: 0}, expected: defaultSpaceExptConcurLimit},
		{name: "值为负数返回默认值", conf: &ExptExecConf{SpaceExptConcurLimit: -1}, expected: defaultSpaceExptConcurLimit},
		{name: "值为正数返回设置值", conf: &ExptExecConf{SpaceExptConcurLimit: 500}, expected: 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result int
			if tt.conf != nil {
				result = tt.conf.GetSpaceExptConcurLimit()
			} else {
				result = (*ExptExecConf)(nil).GetSpaceExptConcurLimit()
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExptExecConf_GetDaemonInterval(t *testing.T) {
	tests := []struct {
		name     string
		conf     *ExptExecConf
		expected time.Duration
	}{
		{name: "nil返回默认值", conf: nil, expected: defaultDaemonInterval},
		{name: "值为0返回默认值", conf: &ExptExecConf{DaemonIntervalSecond: 0}, expected: defaultDaemonInterval},
		{name: "值为负数返回默认值", conf: &ExptExecConf{DaemonIntervalSecond: -1}, expected: defaultDaemonInterval},
		{name: "值为正数返回设置值", conf: &ExptExecConf{DaemonIntervalSecond: 60}, expected: 60 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result time.Duration
			if tt.conf != nil {
				result = tt.conf.GetDaemonInterval()
			} else {
				result = (*ExptExecConf)(nil).GetDaemonInterval()
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExptExecConf_GetZombieIntervalSecond(t *testing.T) {
	tests := []struct {
		name     string
		conf     *ExptExecConf
		expected int
	}{
		{name: "nil返回默认值", conf: nil, expected: defaultZombieIntervalSecond},
		{name: "值为0返回默认值", conf: &ExptExecConf{ZombieIntervalSecond: 0}, expected: defaultZombieIntervalSecond},
		{name: "值为负数返回默认值", conf: &ExptExecConf{ZombieIntervalSecond: -1}, expected: defaultZombieIntervalSecond},
		{name: "值为正数返回设置值", conf: &ExptExecConf{ZombieIntervalSecond: 7200}, expected: 7200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result int
			if tt.conf != nil {
				result = tt.conf.GetZombieIntervalSecond()
			} else {
				result = (*ExptExecConf)(nil).GetZombieIntervalSecond()
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExptExecConf_GetExptItemEvalConf(t *testing.T) {
	evalConf := &ExptItemEvalConf{ConcurNum: 10}

	tests := []struct {
		name     string
		conf     *ExptExecConf
		expected *ExptItemEvalConf
	}{
		{name: "nil返回nil", conf: nil, expected: nil},
		{name: "ExptItemEvalConf为nil返回nil", conf: &ExptExecConf{}, expected: nil},
		{name: "ExptItemEvalConf有值返回对应值", conf: &ExptExecConf{ExptItemEvalConf: evalConf}, expected: evalConf},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result *ExptItemEvalConf
			if tt.conf != nil {
				result = tt.conf.GetExptItemEvalConf()
			} else {
				result = (*ExptExecConf)(nil).GetExptItemEvalConf()
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultExptConsumerConf(t *testing.T) {
	conf := DefaultExptConsumerConf()
	assert.NotNil(t, conf)
	assert.Equal(t, 50, conf.ExptExecWorkerNum)
	assert.Equal(t, 200, conf.ExptItemEvalWorkerNum)
}

func TestDefaultExptErrCtrl(t *testing.T) {
	ctrl := DefaultExptErrCtrl()
	assert.NotNil(t, ctrl)
	assert.Nil(t, ctrl.ErrRetryCtrl)
	assert.Nil(t, ctrl.SpaceErrRetryCtrl)
	assert.Nil(t, ctrl.ResultErrConverts)
}

func TestResultErrConvert_ConvertErrMsg_Extended(t *testing.T) {
	tests := []struct {
		name        string
		convert     *ResultErrConvert
		msg         string
		wantConvert bool
		wantMsg     string
	}{
		{
			name:        "nil convert返回false",
			convert:     nil,
			msg:         "some error",
			wantConvert: false,
			wantMsg:     "",
		},
		{
			name:        "空消息返回false",
			convert:     &ResultErrConvert{MatchedText: "err", ToErrMsg: "converted"},
			msg:         "",
			wantConvert: false,
			wantMsg:     "",
		},
		{
			name:        "ToErrCode和ToErrMsg都为空返回false",
			convert:     &ResultErrConvert{MatchedText: "err"},
			msg:         "some error",
			wantConvert: false,
			wantMsg:     "",
		},
		{
			name:        "非默认模式且MatchedText为空返回false",
			convert:     &ResultErrConvert{ToErrMsg: "converted"},
			msg:         "some error",
			wantConvert: false,
			wantMsg:     "",
		},
		{
			name:        "非默认模式且MatchedText不匹配返回false",
			convert:     &ResultErrConvert{MatchedText: "not_found", ToErrMsg: "converted"},
			msg:         "some error",
			wantConvert: false,
			wantMsg:     "",
		},
		{
			name:        "非默认模式MatchedText匹配且有ToErrMsg",
			convert:     &ResultErrConvert{MatchedText: "error", ToErrMsg: "converted msg"},
			msg:         "some error happened",
			wantConvert: true,
			wantMsg:     "converted msg",
		},
		{
			name:        "非默认模式MatchedText匹配且有ToErrCode",
			convert:     &ResultErrConvert{MatchedText: "error", ToErrCode: errno.CommonInternalErrorCode},
			msg:         "some error happened",
			wantConvert: true,
		},
		{
			name:        "默认模式AsDefault为true有ToErrMsg",
			convert:     &ResultErrConvert{AsDefault: true, ToErrMsg: "default converted"},
			msg:         "any message",
			wantConvert: true,
			wantMsg:     "default converted",
		},
		{
			name:        "默认模式AsDefault为true有ToErrCode",
			convert:     &ResultErrConvert{AsDefault: true, ToErrCode: errno.CommonInternalErrorCode},
			msg:         "any message",
			wantConvert: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converted, msg := tt.convert.ConvertErrMsg(tt.msg)
			assert.Equal(t, tt.wantConvert, converted)
			if tt.wantMsg != "" {
				assert.Equal(t, tt.wantMsg, msg)
			}
		})
	}
}

func TestExptErrCtrl_GetErrRetryCtrl(t *testing.T) {
	defaultCtrl := &ErrRetryCtrl{RetryConf: &RetryConf{RetryTimes: 3}}
	spaceCtrl := &ErrRetryCtrl{RetryConf: &RetryConf{RetryTimes: 5}}

	tests := []struct {
		name     string
		ctrl     *ExptErrCtrl
		spaceID  int64
		expected *ErrRetryCtrl
	}{
		{
			name:    "ctrl为nil返回空ErrRetryCtrl",
			ctrl:    nil,
			spaceID: 1,
		},
		{
			name: "space有专属配置返回space配置",
			ctrl: &ExptErrCtrl{
				ErrRetryCtrl:      defaultCtrl,
				SpaceErrRetryCtrl: map[int64]*ErrRetryCtrl{100: spaceCtrl},
			},
			spaceID:  100,
			expected: spaceCtrl,
		},
		{
			name: "space无专属配置返回默认配置",
			ctrl: &ExptErrCtrl{
				ErrRetryCtrl:      defaultCtrl,
				SpaceErrRetryCtrl: map[int64]*ErrRetryCtrl{100: spaceCtrl},
			},
			spaceID:  999,
			expected: defaultCtrl,
		},
		{
			name: "SpaceErrRetryCtrl为nil返回默认配置",
			ctrl: &ExptErrCtrl{
				ErrRetryCtrl: defaultCtrl,
			},
			spaceID:  1,
			expected: defaultCtrl,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.ctrl.GetErrRetryCtrl(tt.spaceID)
			if tt.ctrl == nil {
				assert.NotNil(t, result)
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestExptErrCtrl_ConvertErrMsg(t *testing.T) {
	tests := []struct {
		name     string
		ctrl     *ExptErrCtrl
		msg      string
		expected string
	}{
		{
			name:     "ctrl为nil返回空",
			ctrl:     nil,
			msg:      "some error",
			expected: "",
		},
		{
			name:     "msg为空返回空",
			ctrl:     &ExptErrCtrl{},
			msg:      "",
			expected: "",
		},
		{
			name: "匹配非默认规则",
			ctrl: &ExptErrCtrl{
				ResultErrConverts: []*ResultErrConvert{
					{MatchedText: "timeout", ToErrMsg: "request timeout"},
					{AsDefault: true, ToErrMsg: "unknown error"},
				},
			},
			msg:      "connection timeout occurred",
			expected: "request timeout",
		},
		{
			name: "不匹配非默认规则回退到默认规则",
			ctrl: &ExptErrCtrl{
				ResultErrConverts: []*ResultErrConvert{
					{MatchedText: "timeout", ToErrMsg: "request timeout"},
					{AsDefault: true, ToErrMsg: "unknown error"},
				},
			},
			msg:      "some random error",
			expected: "unknown error",
		},
		{
			name: "无默认规则也不匹配",
			ctrl: &ExptErrCtrl{
				ResultErrConverts: []*ResultErrConvert{
					{MatchedText: "timeout", ToErrMsg: "request timeout"},
				},
			},
			msg:      "some random error",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.ctrl.ConvertErrMsg(tt.msg)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestErrRetryCtrl_GetRetryConf(t *testing.T) {
	defaultConf := &RetryConf{RetryTimes: 3}
	timeoutConf := &RetryConf{RetryTimes: 5}

	tests := []struct {
		name     string
		ctrl     *ErrRetryCtrl
		err      error
		expected *RetryConf
	}{
		{
			name:     "ctrl为nil返回nil",
			ctrl:     nil,
			err:      errors.New("some error"),
			expected: nil,
		},
		{
			name:     "err为nil返回nil",
			ctrl:     &ErrRetryCtrl{RetryConf: defaultConf},
			err:      nil,
			expected: nil,
		},
		{
			name: "匹配ErrRetryConf中的错误",
			ctrl: &ErrRetryCtrl{
				RetryConf:    defaultConf,
				ErrRetryConf: map[string]*RetryConf{"timeout": timeoutConf},
			},
			err:      errors.New("connection timeout"),
			expected: timeoutConf,
		},
		{
			name: "不匹配ErrRetryConf回退到默认RetryConf",
			ctrl: &ErrRetryCtrl{
				RetryConf:    defaultConf,
				ErrRetryConf: map[string]*RetryConf{"timeout": timeoutConf},
			},
			err:      errors.New("some other error"),
			expected: defaultConf,
		},
		{
			name: "ErrRetryConf为nil回退到默认RetryConf",
			ctrl: &ErrRetryCtrl{
				RetryConf: defaultConf,
			},
			err:      errors.New("some error"),
			expected: defaultConf,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.ctrl.GetRetryConf(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRetryConf_GetRetryTimes(t *testing.T) {
	tests := []struct {
		name     string
		conf     *RetryConf
		expected int
	}{
		{name: "nil返回0", conf: nil, expected: 0},
		{name: "RetryTimes为0返回0", conf: &RetryConf{RetryTimes: 0}, expected: 0},
		{name: "RetryTimes为正数返回设置值", conf: &RetryConf{RetryTimes: 5}, expected: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result int
			if tt.conf != nil {
				result = tt.conf.GetRetryTimes()
			} else {
				result = (*RetryConf)(nil).GetRetryTimes()
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRetryConf_GetRetryInterval(t *testing.T) {
	tests := []struct {
		name     string
		conf     *RetryConf
		expected time.Duration
	}{
		{name: "nil返回默认20s", conf: nil, expected: 20 * time.Second},
		{name: "值为0返回默认20s", conf: &RetryConf{RetryIntervalSecond: 0}, expected: 20 * time.Second},
		{name: "值为负数返回默认20s", conf: &RetryConf{RetryIntervalSecond: -1}, expected: 20 * time.Second},
		{name: "值为正数返回设置值", conf: &RetryConf{RetryIntervalSecond: 60}, expected: 60 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result time.Duration
			if tt.conf != nil {
				result = tt.conf.GetRetryInterval()
			} else {
				result = (*RetryConf)(nil).GetRetryInterval()
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestQuotaSpaceExpt_Serialize_Extended(t *testing.T) {
	t.Run("正常序列化", func(t *testing.T) {
		q := &QuotaSpaceExpt{
			ExptID2RunTime: map[int64]int64{1: 1000, 2: 2000},
		}
		bytes, err := q.Serialize()
		assert.NoError(t, err)
		assert.NotEmpty(t, bytes)
		assert.Contains(t, string(bytes), "ExptID2RunTime")
	})

	t.Run("空map序列化", func(t *testing.T) {
		q := &QuotaSpaceExpt{
			ExptID2RunTime: map[int64]int64{},
		}
		bytes, err := q.Serialize()
		assert.NoError(t, err)
		assert.NotEmpty(t, bytes)
	})

	t.Run("nil map序列化", func(t *testing.T) {
		q := &QuotaSpaceExpt{}
		bytes, err := q.Serialize()
		assert.NoError(t, err)
		assert.NotEmpty(t, bytes)
	})
}

func TestExptTurnRunResult_GetTargetResult(t *testing.T) {
	tr := &EvalTargetRecord{ID: 1, SpaceID: 100}

	tests := []struct {
		name     string
		result   *ExptTurnRunResult
		expected *EvalTargetRecord
	}{
		{name: "nil返回nil", result: nil, expected: nil},
		{name: "TargetResult为nil返回nil", result: &ExptTurnRunResult{}, expected: nil},
		{name: "TargetResult有值返回对应值", result: &ExptTurnRunResult{TargetResult: tr}, expected: tr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result *EvalTargetRecord
			if tt.result != nil {
				result = tt.result.GetTargetResult()
			} else {
				result = (*ExptTurnRunResult)(nil).GetTargetResult()
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExptTurnRunResult_GetEvalErr(t *testing.T) {
	tests := []struct {
		name     string
		result   *ExptTurnRunResult
		expected error
	}{
		{name: "nil返回nil", result: nil, expected: nil},
		{name: "EvalErr为nil返回nil", result: &ExptTurnRunResult{}, expected: nil},
		{name: "EvalErr有值返回对应值", result: &ExptTurnRunResult{EvalErr: fmt.Errorf("test")}, expected: fmt.Errorf("test")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result error
			if tt.result != nil {
				result = tt.result.GetEvalErr()
			} else {
				result = (*ExptTurnRunResult)(nil).GetEvalErr()
			}
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected.Error(), result.Error())
			}
		})
	}
}

func TestExptItemEvalCtx_GetExistItemResultLog(t *testing.T) {
	itemLog := &ExptItemResultRunLog{ID: 1, LogID: "log1"}

	tests := []struct {
		name     string
		ctx      *ExptItemEvalCtx
		expected *ExptItemResultRunLog
	}{
		{name: "ctx为nil返回nil", ctx: nil, expected: nil},
		{name: "ExistItemEvalResult为nil返回nil", ctx: &ExptItemEvalCtx{}, expected: nil},
		{
			name: "ItemResultRunLog为nil返回nil",
			ctx: &ExptItemEvalCtx{
				ExistItemEvalResult: &ExptItemEvalResult{},
			},
			expected: nil,
		},
		{
			name: "返回ItemResultRunLog",
			ctx: &ExptItemEvalCtx{
				ExistItemEvalResult: &ExptItemEvalResult{
					ItemResultRunLog: itemLog,
				},
			},
			expected: itemLog,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.ctx.GetExistItemResultLog()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExptItemEvalCtx_GetExistTurnResultLogs(t *testing.T) {
	turnLogs := map[int64]*ExptTurnResultRunLog{
		1: {ID: 1, LogID: "turn_log1"},
	}

	tests := []struct {
		name     string
		ctx      *ExptItemEvalCtx
		expected map[int64]*ExptTurnResultRunLog
	}{
		{name: "ctx为nil返回nil", ctx: nil, expected: nil},
		{name: "ExistItemEvalResult为nil返回nil", ctx: &ExptItemEvalCtx{}, expected: nil},
		{
			name: "返回TurnResultRunLogs",
			ctx: &ExptItemEvalCtx{
				ExistItemEvalResult: &ExptItemEvalResult{
					TurnResultRunLogs: turnLogs,
				},
			},
			expected: turnLogs,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.ctx.GetExistTurnResultLogs()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExptItemEvalCtx_GetExistTurnResultRunLog(t *testing.T) {
	turnLog := &ExptTurnResultRunLog{ID: 1, TurnID: 10, LogID: "log1"}

	tests := []struct {
		name     string
		ctx      *ExptItemEvalCtx
		turnID   int64
		expected *ExptTurnResultRunLog
	}{
		{
			name:     "ExistItemEvalResult为nil返回nil",
			ctx:      &ExptItemEvalCtx{},
			turnID:   10,
			expected: nil,
		},
		{
			name: "turnID存在返回对应日志",
			ctx: &ExptItemEvalCtx{
				ExistItemEvalResult: &ExptItemEvalResult{
					TurnResultRunLogs: map[int64]*ExptTurnResultRunLog{10: turnLog},
				},
			},
			turnID:   10,
			expected: turnLog,
		},
		{
			name: "turnID不存在返回nil",
			ctx: &ExptItemEvalCtx{
				ExistItemEvalResult: &ExptItemEvalResult{
					TurnResultRunLogs: map[int64]*ExptTurnResultRunLog{10: turnLog},
				},
			},
			turnID:   99,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.ctx.GetExistTurnResultRunLog(tt.turnID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExptItemEvalCtx_GetRecordEvalLogID(t *testing.T) {
	ctx := context.Background()

	t.Run("无ExistItemEvalResult生成新LogID", func(t *testing.T) {
		evalCtx := &ExptItemEvalCtx{}
		logID := evalCtx.GetRecordEvalLogID(ctx)
		assert.NotEmpty(t, logID)
	})

	t.Run("ItemResultRunLog为nil生成新LogID", func(t *testing.T) {
		evalCtx := &ExptItemEvalCtx{
			ExistItemEvalResult: &ExptItemEvalResult{},
		}
		logID := evalCtx.GetRecordEvalLogID(ctx)
		assert.NotEmpty(t, logID)
	})

	t.Run("ItemResultRunLog的LogID为空生成新LogID", func(t *testing.T) {
		evalCtx := &ExptItemEvalCtx{
			ExistItemEvalResult: &ExptItemEvalResult{
				ItemResultRunLog: &ExptItemResultRunLog{LogID: ""},
			},
		}
		logID := evalCtx.GetRecordEvalLogID(ctx)
		assert.NotEmpty(t, logID)
	})

	t.Run("ItemResultRunLog有LogID返回已有LogID", func(t *testing.T) {
		evalCtx := &ExptItemEvalCtx{
			ExistItemEvalResult: &ExptItemEvalResult{
				ItemResultRunLog: &ExptItemResultRunLog{LogID: "existing-log-id"},
			},
		}
		logID := evalCtx.GetRecordEvalLogID(ctx)
		assert.Equal(t, "existing-log-id", logID)
	})
}

func TestExptItemEvalCtx_GetTurnEvalLogID(t *testing.T) {
	ctx := context.Background()

	t.Run("无TurnResultRunLog生成新LogID", func(t *testing.T) {
		evalCtx := &ExptItemEvalCtx{}
		logID := evalCtx.GetTurnEvalLogID(ctx, 1)
		assert.NotEmpty(t, logID)
	})

	t.Run("TurnResultRunLog有LogID返回已有LogID", func(t *testing.T) {
		evalCtx := &ExptItemEvalCtx{
			ExistItemEvalResult: &ExptItemEvalResult{
				TurnResultRunLogs: map[int64]*ExptTurnResultRunLog{
					1: {LogID: "existing-turn-log"},
				},
			},
		}
		logID := evalCtx.GetTurnEvalLogID(ctx, 1)
		assert.Equal(t, "existing-turn-log", logID)
	})

	t.Run("TurnResultRunLog的LogID为空会生成新LogID并写回", func(t *testing.T) {
		turnLog := &ExptTurnResultRunLog{LogID: ""}
		evalCtx := &ExptItemEvalCtx{
			ExistItemEvalResult: &ExptItemEvalResult{
				TurnResultRunLogs: map[int64]*ExptTurnResultRunLog{
					1: turnLog,
				},
			},
		}
		logID := evalCtx.GetTurnEvalLogID(ctx, 1)
		assert.NotEmpty(t, logID)
		assert.Equal(t, logID, turnLog.LogID)
	})

	t.Run("turnID不存在生成新LogID", func(t *testing.T) {
		evalCtx := &ExptItemEvalCtx{
			ExistItemEvalResult: &ExptItemEvalResult{
				TurnResultRunLogs: map[int64]*ExptTurnResultRunLog{
					1: {LogID: "log1"},
				},
			},
		}
		logID := evalCtx.GetTurnEvalLogID(ctx, 99)
		assert.NotEmpty(t, logID)
	})
}
