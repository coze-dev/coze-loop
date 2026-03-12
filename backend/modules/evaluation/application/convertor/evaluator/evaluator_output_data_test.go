// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	evaluatorentity "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

func TestConvertEvaluatorOutputData_RoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dto  *evaluatordto.EvaluatorOutputData
		do   *evaluatorentity.EvaluatorOutputData
	}{
		{
			name: "nil",
		},
		{
			name: "dto->do",
			dto: &evaluatordto.EvaluatorOutputData{
				EvaluatorResult_: &evaluatordto.EvaluatorResult_{Score: gptr.Of(float64(0.8)), Reasoning: gptr.Of("ok")},
				EvaluatorUsage:   &evaluatordto.EvaluatorUsage{InputTokens: gptr.Of(int64(1)), OutputTokens: gptr.Of(int64(2))},
				EvaluatorRunError: &evaluatordto.EvaluatorRunError{
					Code:    gptr.Of(int32(123)),
					Message: gptr.Of("msg"),
				},
				TimeConsumingMs: gptr.Of(int64(10)),
				Stdout:          gptr.Of("stdout"),
				ExtraOutput: &evaluatordto.EvaluatorExtraOutputContent{
					URI:        gptr.Of("uri"),
					URL:        gptr.Of("url"),
					OutputType: gptr.Of(evaluatordto.EvaluatorExtraOutputTypeHTML),
				},
			},
		},
		{
			name: "do->dto",
			do: &evaluatorentity.EvaluatorOutputData{
				EvaluatorResult: &evaluatorentity.EvaluatorResult{
					Score:     gptr.Of(float64(0.5)),
					Reasoning: "r",
					Correction: &evaluatorentity.Correction{
						Score:     gptr.Of(float64(0.6)),
						Explain:   "e",
						UpdatedBy: "u",
					},
				},
				EvaluatorUsage: &evaluatorentity.EvaluatorUsage{InputTokens: 3, OutputTokens: 4},
				EvaluatorRunError: &evaluatorentity.EvaluatorRunError{
					Code:    321,
					Message: "err",
				},
				TimeConsumingMS: 11,
				Stdout:          "s",
				ExtraOutput: &evaluatorentity.EvaluatorExtraOutputContent{
					URI:        gptr.Of("uri2"),
					URL:        gptr.Of("url2"),
					OutputType: gptr.Of(evaluatorentity.EvaluatorExtraOutputTypeHTML),
				},
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.dto != nil || tc.name == "nil" {
				gotDO := ConvertEvaluatorOutputDataDTO2DO(tc.dto)
				if tc.dto == nil {
					assert.Nil(t, gotDO)
				} else {
					assert.NotNil(t, gotDO)
					assert.Equal(t, tc.dto.GetTimeConsumingMs(), gotDO.TimeConsumingMS)
					assert.Equal(t, tc.dto.GetStdout(), gotDO.Stdout)
					if assert.NotNil(t, gotDO.ExtraOutput) {
						assert.Equal(t, tc.dto.ExtraOutput.URI, gotDO.ExtraOutput.URI)
						assert.Equal(t, tc.dto.ExtraOutput.URL, gotDO.ExtraOutput.URL)
						if assert.NotNil(t, gotDO.ExtraOutput.OutputType) {
							assert.Equal(t, evaluatorentity.EvaluatorExtraOutputType(*tc.dto.ExtraOutput.OutputType), *gotDO.ExtraOutput.OutputType)
						}
					}
				}
			}

			if tc.do != nil || tc.name == "nil" {
				gotDTO := ConvertEvaluatorOutputDataDO2DTO(tc.do)
				if tc.do == nil {
					assert.Nil(t, gotDTO)
				} else {
					assert.NotNil(t, gotDTO)
					assert.Equal(t, tc.do.TimeConsumingMS, gotDTO.GetTimeConsumingMs())
					assert.Equal(t, tc.do.Stdout, gotDTO.GetStdout())
					if assert.NotNil(t, gotDTO.ExtraOutput) {
						assert.Equal(t, tc.do.ExtraOutput.URI, gotDTO.ExtraOutput.URI)
						assert.Equal(t, tc.do.ExtraOutput.URL, gotDTO.ExtraOutput.URL)
						if assert.NotNil(t, tc.do.ExtraOutput.OutputType) {
							assert.Equal(t, evaluatordto.EvaluatorExtraOutputType(*tc.do.ExtraOutput.OutputType), *gotDTO.ExtraOutput.OutputType)
						}
					}
				}
			}
		})
	}
}

func TestToEvaluatorRunStatusDO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		in     spi.InvokeEvaluatorRunStatus
		expect evaluatorentity.EvaluatorRunStatus
	}{
		{name: "failed", in: spi.InvokeEvaluatorRunStatus_FAILED, expect: evaluatorentity.EvaluatorRunStatusFail},
		{name: "success", in: spi.InvokeEvaluatorRunStatus_SUCCESS, expect: evaluatorentity.EvaluatorRunStatusSuccess},
		{name: "unknown", in: spi.InvokeEvaluatorRunStatus(999), expect: evaluatorentity.EvaluatorRunStatusUnknown},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expect, ToEvaluatorRunStatusDO(tc.in))
		})
	}
}

func TestToInvokeEvaluatorOutputDataDO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		in     *spi.InvokeEvaluatorOutputData
		status spi.InvokeEvaluatorRunStatus
		check  func(t *testing.T, got *evaluatorentity.EvaluatorOutputData)
	}{
		{
			name:   "nil input",
			in:     nil,
			status: spi.InvokeEvaluatorRunStatus_SUCCESS,
			check: func(t *testing.T, got *evaluatorentity.EvaluatorOutputData) {
				assert.Nil(t, got)
			},
		},
		{
			name: "success",
			in: &spi.InvokeEvaluatorOutputData{
				EvaluatorResult_: &spi.InvokeEvaluatorResult_{Score: gptr.Of(float64(0.9)), Reasoning: gptr.Of("r")},
				EvaluatorUsage:   &spi.InvokeEvaluatorUsage{InputTokens: gptr.Of(int64(1)), OutputTokens: gptr.Of(int64(2))},
				ExtraOutput:      &spi.EvaluatorExtraOutputContent{URI: gptr.Of("u"), URL: gptr.Of("l")},
			},
			status: spi.InvokeEvaluatorRunStatus_SUCCESS,
			check: func(t *testing.T, got *evaluatorentity.EvaluatorOutputData) {
				if assert.NotNil(t, got) {
					assert.NotNil(t, got.EvaluatorResult)
					assert.NotNil(t, got.EvaluatorUsage)
					assert.Nil(t, got.EvaluatorRunError)
					assert.Equal(t, float64(0.9), gptr.Indirect(got.EvaluatorResult.Score))
					assert.Equal(t, int64(1), got.EvaluatorUsage.InputTokens)
					assert.Equal(t, "u", gptr.Indirect(got.ExtraOutput.URI))
				}
			},
		},
		{
			name: "failed with nil run error uses default",
			in: &spi.InvokeEvaluatorOutputData{
				EvaluatorRunError: nil,
			},
			status: spi.InvokeEvaluatorRunStatus_FAILED,
			check: func(t *testing.T, got *evaluatorentity.EvaluatorOutputData) {
				if assert.NotNil(t, got) && assert.NotNil(t, got.EvaluatorRunError) {
					assert.Equal(t, int32(errno.RunEvaluatorFailCode), got.EvaluatorRunError.Code)
					assert.Equal(t, "unknown error", got.EvaluatorRunError.Message)
					assert.Nil(t, got.EvaluatorResult)
					assert.Nil(t, got.EvaluatorUsage)
				}
			},
		},
		{
			name: "unknown status returns nil",
			in: &spi.InvokeEvaluatorOutputData{
				EvaluatorUsage: &spi.InvokeEvaluatorUsage{InputTokens: gptr.Of(int64(1))},
			},
			status: spi.InvokeEvaluatorRunStatus(999),
			check: func(t *testing.T, got *evaluatorentity.EvaluatorOutputData) {
				assert.Nil(t, got)
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.check(t, ToInvokeEvaluatorOutputDataDO(tc.in, tc.status))
		})
	}
}
