// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"encoding/json"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	evaluatorentity "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

func TestEvidenceArchive_RuntimeCallbackKeepsMetadataWithoutDiagnostics(t *testing.T) {
	t.Parallel()
	const payload = `{"evaluator_run_error":{"code":9,"message":"runtime timeout"},"extra_output":{"uri":"legacy/report.html","url":"https://legacy.example/report"},"evidence_archive":{"schema_version":"v1","object_key":"space/1/evaluator/2/evidence.tar.gz","status":"partial","trigger":"runtime_timeout","size_bytes":1234,"sha256":"abc","truncated_files":1,"deadline_at_unix_ms":123456789,"last_phase":"checkpoint","last_progress_at":123456700,"termination_source":"runtime_deadline","archive_executor":"ago-daemon","agent_termination_result":"terminated","error":"truncated","fornax_evaluator_log_url":"https://untrusted.example/archive"}}`
	var dto spi.InvokeEvaluatorOutputData
	require.NoError(t, json.Unmarshal([]byte(payload), &dto))
	do := ToInvokeEvaluatorOutputDataDO(&dto, spi.InvokeEvaluatorRunStatus_FAILED)
	require.NotNil(t, do)
	encoded, err := json.Marshal(do.EvidenceArchive)
	require.NoError(t, err)
	assert.JSONEq(t, `{"schema_version":"v1","object_key":"space/1/evaluator/2/evidence.tar.gz","status":"partial","trigger":"runtime_timeout","size_bytes":1234,"sha256":"abc","truncated_files":1,"error":"truncated"}`, string(encoded))
	assert.Empty(t, do.EvidenceArchive.FornaxEvaluatorLogURL)
	assert.Equal(t, "legacy/report.html", gptr.Indirect(do.ExtraOutput.URI))
	assert.Equal(t, "https://legacy.example/report", gptr.Indirect(do.ExtraOutput.URL))
	public, err := json.Marshal(ConvertEvaluatorEvidenceArchiveDO2DTO(do.EvidenceArchive))
	require.NoError(t, err)
	assert.JSONEq(t, string(encoded), string(public))
}

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
				EvidenceArchive: &evaluatordto.EvaluatorEvidenceArchive{
					SchemaVersion:         gptr.Of("1"),
					ObjectKey:             gptr.Of("evidence/record.tar.gz"),
					Status:                gptr.Of("uploaded"),
					Trigger:               gptr.Of("run_timeout"),
					SizeBytes:             gptr.Of(int64(4096)),
					Sha256:                gptr.Of("abc123"),
					TruncatedFiles:        gptr.Of(int64(1)),
					Error:                 gptr.Of(""),
					FornaxEvaluatorLogURL: gptr.Of("https://untrusted.example/callback-url"),
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
				EvidenceArchive: &evaluatorentity.EvaluatorEvidenceArchive{
					SchemaVersion:         "1",
					ObjectKey:             "evidence/record-2.tar.gz",
					Status:                "uploaded",
					Trigger:               "completed",
					SizeBytes:             8192,
					SHA256:                "def456",
					TruncatedFiles:        1,
					Error:                 "none",
					FornaxEvaluatorLogURL: "https://signed.example/evidence?ttl=600",
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
					if assert.NotNil(t, gotDO.EvidenceArchive) {
						assert.Equal(t, tc.dto.EvidenceArchive.GetObjectKey(), gotDO.EvidenceArchive.ObjectKey)
						assert.Equal(t, tc.dto.EvidenceArchive.GetSha256(), gotDO.EvidenceArchive.SHA256)
						assert.Equal(t, tc.dto.EvidenceArchive.GetTruncatedFiles(), gotDO.EvidenceArchive.TruncatedFiles)
						assert.Empty(t, gotDO.EvidenceArchive.FornaxEvaluatorLogURL, "callback/read-only URL must not enter persisted domain data")
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
					if assert.NotNil(t, gotDTO.EvidenceArchive) {
						assert.Equal(t, tc.do.EvidenceArchive.ObjectKey, gotDTO.EvidenceArchive.GetObjectKey())
						assert.Equal(t, tc.do.EvidenceArchive.SHA256, gotDTO.EvidenceArchive.GetSha256())
						assert.Equal(t, tc.do.EvidenceArchive.TruncatedFiles, gotDTO.EvidenceArchive.GetTruncatedFiles())
						assert.Equal(t, tc.do.EvidenceArchive.FornaxEvaluatorLogURL, gotDTO.EvidenceArchive.GetFornaxEvaluatorLogURL())
					}
				}
			}
		})
	}
}

func TestConvertCorrectionDTO2DO_Nil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, ConvertCorrectionDTO2DO(nil))
}

func TestConvertCorrectionDO2DTO_Nil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, ConvertCorrectionDO2DTO(nil))
}

func TestConvertEvaluatorResultDTO2DO_Nil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, ConvertEvaluatorResultDTO2DO(nil))
}

func TestConvertEvaluatorResultDO2DTO_Nil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, ConvertEvaluatorResultDO2DTO(nil))
}

func TestConvertEvaluatorUsageDTO2DO_Nil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, ConvertEvaluatorUsageDTO2DO(nil))
}

func TestConvertEvaluatorUsageDO2DTO_Nil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, ConvertEvaluatorUsageDO2DTO(nil))
}

func TestConvertEvaluatorRunErrorDTO2DO_Nil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, ConvertEvaluatorRunErrorDTO2DO(nil))
}

func TestConvertEvaluatorRunErrorDO2DTO_Nil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, ConvertEvaluatorRunErrorDO2DTO(nil))
}

func TestToInvokeEvaluatorResultDO_Nil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, toInvokeEvaluatorResultDO(nil))
}

func TestToInvokeEvaluatorUsageDO_Nil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, toInvokeEvaluatorUsageDO(nil))
}

func TestToInvokeEvaluatorRunErrorDO_Nil(t *testing.T) {
	t.Parallel()
	result := toInvokeEvaluatorRunErrorDO(nil)
	assert.NotNil(t, result)
	assert.Equal(t, int32(errno.RunEvaluatorFailCode), result.Code)
	assert.Equal(t, "unknown error", result.Message)
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
			name: "success preserves evidence archive separately from extra output",
			in: &spi.InvokeEvaluatorOutputData{
				EvaluatorResult_: &spi.InvokeEvaluatorResult_{Score: gptr.Of(float64(0.9)), Reasoning: gptr.Of("r")},
				EvaluatorUsage:   &spi.InvokeEvaluatorUsage{InputTokens: gptr.Of(int64(1)), OutputTokens: gptr.Of(int64(2))},
				ExtraOutput:      &spi.EvaluatorExtraOutputContent{URI: gptr.Of("u"), URL: gptr.Of("l")},
				EvidenceArchive: &spi.EvaluatorEvidenceArchive{
					SchemaVersion:         gptr.Of("1"),
					ObjectKey:             gptr.Of("evidence/callback.tar.gz"),
					Status:                gptr.Of("uploaded"),
					SizeBytes:             gptr.Of(int64(1024)),
					Sha256:                gptr.Of("sha-callback"),
					TruncatedFiles:        gptr.Of(int64(1)),
					FornaxEvaluatorLogURL: gptr.Of("https://untrusted.example/callback-url"),
				},
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
					assert.Equal(t, "evidence/callback.tar.gz", got.EvidenceArchive.ObjectKey)
					assert.Equal(t, "sha-callback", got.EvidenceArchive.SHA256)
					assert.Equal(t, int64(1), got.EvidenceArchive.TruncatedFiles)
					assert.Empty(t, got.EvidenceArchive.FornaxEvaluatorLogURL, "SPI callbacks cannot persist a read-only signed URL")
					assert.Equal(t, "u", gptr.Indirect(got.ExtraOutput.URI), "evidence archive must not overwrite extra_output")
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
					assert.Nil(t, got.EvidenceArchive, "legacy callbacks remain compatible when evidence_archive is absent")
				}
			},
		},
		{
			name: "failed preserves usage",
			in: &spi.InvokeEvaluatorOutputData{
				EvaluatorUsage: &spi.InvokeEvaluatorUsage{InputTokens: gptr.Of(int64(105119)), OutputTokens: gptr.Of(int64(1938))},
				EvaluatorRunError: &spi.InvokeEvaluatorRunError{
					Code:    gptr.Of(int32(3)),
					Message: gptr.Of("run error"),
				},
				EvidenceArchive: &spi.EvaluatorEvidenceArchive{
					ObjectKey:      gptr.Of("evidence/failed.tar.gz"),
					Status:         gptr.Of("failed"),
					Trigger:        gptr.Of("run_timeout"),
					TruncatedFiles: gptr.Of(int64(2)),
					Error:          gptr.Of("upload interrupted"),
				},
			},
			status: spi.InvokeEvaluatorRunStatus_FAILED,
			check: func(t *testing.T, got *evaluatorentity.EvaluatorOutputData) {
				if assert.NotNil(t, got) {
					assert.Nil(t, got.EvaluatorResult)
					if assert.NotNil(t, got.EvaluatorUsage) {
						assert.Equal(t, int64(105119), got.EvaluatorUsage.InputTokens)
						assert.Equal(t, int64(1938), got.EvaluatorUsage.OutputTokens)
					}
					if assert.NotNil(t, got.EvaluatorRunError) {
						assert.Equal(t, int32(3), got.EvaluatorRunError.Code)
						assert.Equal(t, "run error", got.EvaluatorRunError.Message)
					}
					if assert.NotNil(t, got.EvidenceArchive) {
						assert.Equal(t, "evidence/failed.tar.gz", got.EvidenceArchive.ObjectKey)
						assert.Equal(t, "failed", got.EvidenceArchive.Status)
						assert.Equal(t, int64(2), got.EvidenceArchive.TruncatedFiles)
						assert.Equal(t, "upload interrupted", got.EvidenceArchive.Error)
					}
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
