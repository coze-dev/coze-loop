// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

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
			// 执行测试
			result := tt.turnRunResult.AbortWithTargetResult(tt.experiment)

			// 验证返回值
			assert.Equal(t, tt.expectedAbort, result)

			// 验证错误设置
			if tt.expectedErr {
				assert.Error(t, tt.turnRunResult.GetEvalErr())
				if tt.expectedErrMsg != "" {
					assert.Contains(t, tt.turnRunResult.GetEvalErr().Error(), tt.expectedErrMsg)
				}
				// 验证错误码
				statusErr, ok := errorx.FromStatusError(tt.turnRunResult.GetEvalErr())
				assert.True(t, ok)
				assert.Equal(t, int32(errno.CommonInternalErrorCode), statusErr.Code())
			} else {
				assert.NoError(t, tt.turnRunResult.GetEvalErr())
			}

			// 验证AsyncAbort设置
			if tt.checkAsyncAbort {
				assert.True(t, tt.turnRunResult.AsyncAbort)
			} else {
				assert.False(t, tt.turnRunResult.AsyncAbort)
			}
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

			// 验证返回值是同一个实例
			assert.Equal(t, result, returned)

			// 验证TargetResult被正确设置
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

			// 验证返回值是同一个实例
			assert.Equal(t, result, returned)

			// 验证EvaluatorResults被正确设置
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
