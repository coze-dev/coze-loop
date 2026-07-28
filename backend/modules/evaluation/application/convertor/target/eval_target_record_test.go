// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package target

import (
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

func TestEvalTargetRecordConversions(t *testing.T) {
	now := time.Now().UnixMilli()
	status := entity.EvalTargetRunStatusSuccess
	record := &entity.EvalTargetRecord{
		ID:              1,
		SpaceID:         2,
		TargetID:        3,
		TargetVersionID: 4,
		ExperimentRunID: 5,
		ItemID:          6,
		TurnID:          7,
		TraceID:         "trace",
		LogID:           "log",
		EvalTargetInputData: &entity.EvalTargetInputData{
			HistoryMessages: []*entity.Message{
				{
					Role: entity.RoleUser,
					Content: &entity.Content{
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("hello"),
					},
				},
			},
			InputFields: map[string]*entity.Content{
				"field": {
					ContentType: gptr.Of(entity.ContentTypeText),
					Text:        gptr.Of("value"),
				},
			},
			Ext: map[string]string{"extra": "ext"},
		},
		EvalTargetOutputData: &entity.EvalTargetOutputData{
			OutputFields: map[string]*entity.Content{
				"output": {
					ContentType: gptr.Of(entity.ContentTypeText),
					Text:        gptr.Of("out"),
				},
			},
			EvalTargetUsage: &entity.EvalTargetUsage{
				InputTokens:  10,
				OutputTokens: 20,
			},
			EvalTargetRunError: &entity.EvalTargetRunError{Code: errno.CommonInternalErrorCode, Message: "err"},
			TimeConsumingMS:    gptr.Of(int64(42)),
		},
		Status:   &status,
		BaseInfo: &entity.BaseInfo{CreatedAt: gptr.Of(now), UpdatedAt: gptr.Of(now + 1)},
	}

	dto := EvalTargetRecordDO2DTO(record)
	assert.NotNil(t, dto)
	assert.Equal(t, record.ID, dto.GetID())
	assert.Equal(t, record.TraceID, dto.GetTraceID())
	assert.Equal(t, "value", dto.GetEvalTargetInputData().GetInputFields()["field"].GetText())
	assert.Equal(t, int64(10), dto.GetEvalTargetOutputData().GetEvalTargetUsage().GetInputTokens())

	back := RecordDTO2DO(dto)
	assert.Equal(t, record.TargetID, back.TargetID)
	assert.Equal(t, record.EvalTargetOutputData.OutputFields["output"].GetText(), back.EvalTargetOutputData.OutputFields["output"].GetText())
	assert.Equal(t, record.Status, back.Status)

	var nilTime *int64
	assert.True(t, UnixMsPtr2Time(nilTime).IsZero())
	neg := gptr.Of(int64(-1))
	assert.True(t, UnixMsPtr2Time(neg).IsZero())
	assert.False(t, UnixMsPtr2Time(gptr.Of(int64(123))).IsZero())
}

func TestToInvokeOutputDataDO(t *testing.T) {
	successStatus := spi.InvokeEvalTargetStatus_SUCCESS
	contentType := spi.ContentTypeText
	successReq := &openapi.ReportEvalTargetInvokeResultRequest{
		Status: &successStatus,
		Output: &spi.InvokeEvalTargetOutput{
			ActualOutput: &spi.Content{
				ContentType: &contentType,
				Text:        gptr.Of("mock-output"),
				MultiPart: []*spi.Content{{
					ContentType: &contentType,
					Text:        gptr.Of("part"),
				}},
			},
		},
		Usage: &spi.InvokeEvalTargetUsage{
			InputTokens:  gptr.Of(int64(100)),
			OutputTokens: gptr.Of(int64(200)),
		},
	}

	successOutput := ToInvokeOutputDataDO(successReq)
	if assert.NotNil(t, successOutput) {
		assert.Contains(t, successOutput.OutputFields, consts.OutputSchemaKey)
		assert.NotNil(t, successOutput.EvalTargetUsage)
		assert.Equal(t, int64(100), successOutput.EvalTargetUsage.InputTokens)
		assert.Nil(t, successOutput.EvalTargetRunError)
	}

	failStatus := spi.InvokeEvalTargetStatus_FAILED
	failReq := &openapi.ReportEvalTargetInvokeResultRequest{
		Status:       &failStatus,
		ErrorMessage: gptr.Of("failed"),
	}

	failOutput := ToInvokeOutputDataDO(failReq)
	if assert.NotNil(t, failOutput) {
		assert.NotNil(t, failOutput.EvalTargetRunError)
		assert.Equal(t, int32(errno.CustomEvalTargetInvokeFailCode), failOutput.EvalTargetRunError.Code)
		assert.Equal(t, "failed", failOutput.EvalTargetRunError.Message)
		assert.Nil(t, failOutput.EvalTargetUsage)
	}

	unknownStatus := spi.InvokeEvalTargetStatus(99)
	unknownReq := &openapi.ReportEvalTargetInvokeResultRequest{Status: &unknownStatus}
	assert.Nil(t, ToInvokeOutputDataDO(unknownReq))
}

func TestToInvokeOutputDataDO_PartialData(t *testing.T) {
	successStatus := spi.InvokeEvalTargetStatus_SUCCESS
	req := &openapi.ReportEvalTargetInvokeResultRequest{
		Status: &successStatus,
		Output: &spi.InvokeEvalTargetOutput{},
		Usage:  &spi.InvokeEvalTargetUsage{},
	}

	output := ToInvokeOutputDataDO(req)
	if assert.NotNil(t, output) {
		assert.Empty(t, output.OutputFields)
		assert.Nil(t, output.EvalTargetUsage)
	}
}

func TestToSPIContentHelpers(t *testing.T) {
	textType := spi.ContentTypeText
	imageType := spi.ContentTypeImage
	spiContent := &spi.Content{
		ContentType: &textType,
		Text:        gptr.Of("root"),
		Image: &spi.Image{
			URL: gptr.Of("http://example.com/image.png"),
		},
		MultiPart: []*spi.Content{{
			ContentType: &imageType,
		}},
	}

	content := ToSPIContentDO(spiContent)
	if assert.NotNil(t, content) {
		assert.Equal(t, entity.ContentTypeText, *content.ContentType)
		assert.Len(t, content.MultiPart, 1)
		assert.Equal(t, entity.ContentTypeImage, *content.MultiPart[0].ContentType)
	}

	assert.Equal(t, entity.EvalTargetRunStatusSuccess, ToTargetRunStatsDO(spi.InvokeEvalTargetStatus_SUCCESS))
	assert.Equal(t, entity.EvalTargetRunStatusFail, ToTargetRunStatsDO(spi.InvokeEvalTargetStatus_FAILED))
	assert.Equal(t, entity.EvalTargetRunStatusUnknown, ToTargetRunStatsDO(spi.InvokeEvalTargetStatus(42)))
}

func TestNilInputBranches(t *testing.T) {
	t.Parallel()

	// InputDO2DTO nil input returns nil
	assert.Nil(t, InputDO2DTO(nil))

	// OutputDO2DTO nil input returns nil
	assert.Nil(t, OutputDO2DTO(nil))

	// InputDTO2ToDO nil input returns nil
	assert.Nil(t, InputDTO2ToDO(nil))

	// OutputDTO2ToDO nil input returns nil
	assert.Nil(t, OutputDTO2ToDO(nil))

	// StatusDO2DTO nil input returns nil
	assert.Nil(t, StatusDO2DTO(nil))

	// StatusDTO2DO nil input returns nil
	assert.Nil(t, StatusDTO2DO(nil))

	// getInt64Value nil input returns 0
	assert.Equal(t, int64(0), getInt64Value(nil))

	// getStringValue nil input returns ""
	assert.Equal(t, "", getStringValue(nil))

	// getInt32Value nil input returns 0
	assert.Equal(t, int32(0), getInt32Value(nil))

	// UsageDO2DTO nil input returns nil
	assert.Nil(t, UsageDO2DTO(nil))

	// RunErrorDO2DTO nil input returns nil
	assert.Nil(t, RunErrorDO2DTO(nil))

	// UsageDTO2DO nil input returns nil
	assert.Nil(t, UsageDTO2DO(nil))

	// RunErrorDTO2DO nil input returns nil
	assert.Nil(t, RunErrorDTO2DO(nil))

	// EvalTargetRecordDO2DTO nil input returns nil
	assert.Nil(t, EvalTargetRecordDO2DTO(nil))

	// RecordDTO2DO nil input returns nil
	assert.Nil(t, RecordDTO2DO(nil))
}

func TestContentDTO2DOs_NilAndEmpty(t *testing.T) {
	t.Parallel()

	// nil input
	res := ContentDTO2DOs(nil)
	assert.NotNil(t, res)
	assert.Empty(t, res)

	// empty input
	res = ContentDTO2DOs(map[string]*common.Content{})
	assert.NotNil(t, res)
	assert.Empty(t, res)

	// input with nil value
	res = ContentDTO2DOs(map[string]*common.Content{"key": nil})
	assert.Len(t, res, 1)
	assert.Nil(t, res["key"])
}

func TestMessagesDTO2DO_NilMessage(t *testing.T) {
	t.Parallel()

	// nil slice
	res := MessagesDTO2DO(nil)
	assert.NotNil(t, res)
	assert.Empty(t, res)

	// slice with nil element - should be skipped
	res = MessagesDTO2DO([]*common.Message{nil})
	assert.NotNil(t, res)
	assert.Empty(t, res)
}

func TestToSPIContentTypeDO_AudioAndVideo(t *testing.T) {
	t.Parallel()

	// Audio branch
	audioType := spi.ContentTypeAudio
	audioContent := &spi.Content{
		ContentType: &audioType,
		Audio: &spi.Audio{
			URL: gptr.Of("http://example.com/audio.mp3"),
		},
	}
	audioDO := ToSPIContentDO(audioContent)
	assert.NotNil(t, audioDO)
	assert.Equal(t, entity.ContentTypeAudio, *audioDO.ContentType)
	assert.NotNil(t, audioDO.Audio)
	assert.Equal(t, "http://example.com/audio.mp3", *audioDO.Audio.URL)

	// Video branch
	videoType := spi.ContentTypeVideo
	videoContent := &spi.Content{
		ContentType: &videoType,
		Video: &spi.Video{
			URL: gptr.Of("http://example.com/video.mp4"),
		},
	}
	videoDO := ToSPIContentDO(videoContent)
	assert.NotNil(t, videoDO)
	assert.Equal(t, entity.ContentTypeVideo, *videoDO.ContentType)
	assert.NotNil(t, videoDO.Video)
	assert.Equal(t, "http://example.com/video.mp4", *videoDO.Video.URL)

	// nil content
	assert.Nil(t, ToSPIContentDO(nil))

	// multipart type
	multipartType := spi.ContentTypeMultiPart
	mpContent := &spi.Content{
		ContentType: &multipartType,
	}
	mpDO := ToSPIContentDO(mpContent)
	assert.NotNil(t, mpDO)
	assert.Equal(t, entity.ContentTypeMultipart, *mpDO.ContentType)

	// unknown type falls back to Text (default branch)
	unknownType := spi.ContentType("unknown")
	unknownContent := &spi.Content{
		ContentType: &unknownType,
	}
	unknownDO := ToSPIContentDO(unknownContent)
	assert.NotNil(t, unknownDO)
	assert.Equal(t, entity.ContentTypeText, *unknownDO.ContentType)
}

func TestToInvokeOutputDataDO_ErrorCode(t *testing.T) {
	failStatus := spi.InvokeEvalTargetStatus_FAILED

	t.Run("explicit_error_code_overrides_default", func(t *testing.T) {
		req := &openapi.ReportEvalTargetInvokeResultRequest{
			Status:       &failStatus,
			ErrorMessage: gptr.Of("timeout"),
			ErrorCode:    gptr.Of(int32(601200701)),
		}
		out := ToInvokeOutputDataDO(req)
		assert.NotNil(t, out.EvalTargetRunError)
		assert.Equal(t, int32(601200701), out.EvalTargetRunError.Code)
		assert.Equal(t, "timeout", out.EvalTargetRunError.Message)
	})

	t.Run("zero_error_code_falls_back_to_default", func(t *testing.T) {
		req := &openapi.ReportEvalTargetInvokeResultRequest{
			Status:       &failStatus,
			ErrorMessage: gptr.Of("plain fail"),
		}
		out := ToInvokeOutputDataDO(req)
		assert.NotNil(t, out.EvalTargetRunError)
		assert.Equal(t, int32(errno.CustomEvalTargetInvokeFailCode), out.EvalTargetRunError.Code)
	})

	t.Run("error_code_only_no_message", func(t *testing.T) {
		req := &openapi.ReportEvalTargetInvokeResultRequest{
			Status:    &failStatus,
			ErrorCode: gptr.Of(int32(601299999)),
		}
		out := ToInvokeOutputDataDO(req)
		assert.NotNil(t, out.EvalTargetRunError)
		assert.Equal(t, int32(601299999), out.EvalTargetRunError.Code)
	})
}

// TestStepsConvert 覆盖 StepsDO2DTO / StepsDTO2DO 的完整分支：
// - nil/empty 返回 nil；
// - 元素含 nil 时跳过；
// - 字段完整回环 (round-trip) 语义等价。
func TestStepsConvert(t *testing.T) {
	t.Parallel()

	t.Run("nil and empty return nil", func(t *testing.T) {
		assert.Nil(t, StepsDO2DTO(nil))
		assert.Nil(t, StepsDO2DTO([]*entity.EvalTargetStep{}))
		assert.Nil(t, StepsDTO2DO(nil))
		assert.Nil(t, StepsDTO2DO([]*eval_target.EvalTargetStep{}))
	})

	t.Run("skip nil element", func(t *testing.T) {
		got := StepsDO2DTO([]*entity.EvalTargetStep{nil})
		assert.Empty(t, got)

		got2 := StepsDTO2DO([]*eval_target.EvalTargetStep{nil})
		assert.Empty(t, got2)
	})

	t.Run("round trip preserves all fields", func(t *testing.T) {
		src := []*entity.EvalTargetStep{
			{
				StepName:     "plan",
				EventType:    "STARTED",
				EventTimeMS:  1700000000000,
				Success:      true,
				ErrorCode:    0,
				ErrorMessage: "",
				DurationMS:   0,
			},
			{
				StepName:     "act",
				EventType:    "FINISHED",
				EventTimeMS:  1700000000500,
				Success:      false,
				ErrorCode:    601200999,
				ErrorMessage: "step failed",
				DurationMS:   1500,
			},
		}
		dto := StepsDO2DTO(src)
		if !assert.Len(t, dto, 2) {
			return
		}
		assert.Equal(t, "plan", dto[0].GetStepName())
		assert.Equal(t, "STARTED", dto[0].GetEventType())
		assert.Equal(t, int64(1700000000000), dto[0].GetEventTimeMs())
		assert.True(t, dto[0].GetSuccess())
		assert.Equal(t, "act", dto[1].GetStepName())
		assert.Equal(t, int32(601200999), dto[1].GetErrorCode())
		assert.Equal(t, "step failed", dto[1].GetErrorMessage())
		assert.Equal(t, int64(1500), dto[1].GetDurationMs())

		back := StepsDTO2DO(dto)
		assert.Equal(t, src, back)
	})
}

// TestMessagesAndContentDO2DTOs 覆盖 MessagesDO2DTOs / ContentDOToDTOs 的非空分支。
func TestMessagesAndContentDO2DTOs(t *testing.T) {
	t.Parallel()

	t.Run("MessagesDO2DTOs converts each element", func(t *testing.T) {
		msgs := []*entity.Message{
			{Role: entity.RoleUser, Content: &entity.Content{
				ContentType: gptr.Of(entity.ContentTypeText),
				Text:        gptr.Of("hi"),
			}},
			{Role: entity.RoleAssistant, Content: &entity.Content{
				ContentType: gptr.Of(entity.ContentTypeText),
				Text:        gptr.Of("world"),
			}},
		}
		res := MessagesDO2DTOs(msgs)
		assert.Len(t, res, 2)
		assert.Equal(t, "hi", res[0].GetContent().GetText())
		assert.Equal(t, "world", res[1].GetContent().GetText())
	})

	t.Run("ContentDOToDTOs converts each entry", func(t *testing.T) {
		src := map[string]*entity.Content{
			"a": {ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("A")},
			"b": {ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("B")},
		}
		res := ContentDOToDTOs(src)
		assert.Len(t, res, 2)
		assert.Equal(t, "A", res["a"].GetText())
		assert.Equal(t, "B", res["b"].GetText())
	})

	t.Run("empty inputs return empty non-nil", func(t *testing.T) {
		// nil / empty msgs 走 len==0 早退路径
		got := MessagesDO2DTOs(nil)
		assert.NotNil(t, got)
		assert.Empty(t, got)

		got2 := ContentDOToDTOs(nil)
		assert.NotNil(t, got2)
		assert.Empty(t, got2)
	})
}

// TestToInvokeOutputDataDO_FailedWithOutputExt 覆盖 FAILED 分支下 output.Ext 透传。
func TestToInvokeOutputDataDO_FailedWithOutputExt(t *testing.T) {
	t.Parallel()

	failStatus := spi.InvokeEvalTargetStatus_FAILED
	req := &openapi.ReportEvalTargetInvokeResultRequest{
		Status:       &failStatus,
		ErrorMessage: gptr.Of("failed"),
		ErrorCode:    gptr.Of(int32(601200999)),
		Output: &spi.InvokeEvalTargetOutput{
			Ext: map[string]string{"k": "v"},
		},
	}
	out := ToInvokeOutputDataDO(req)
	if assert.NotNil(t, out) {
		assert.Equal(t, "v", out.Ext["k"])
		assert.NotNil(t, out.EvalTargetRunError)
		assert.Equal(t, int32(601200999), out.EvalTargetRunError.Code)
	}
}

// TestToInvokeOutputDataDO_ExtOutputFields 覆盖 SUCCESS 分支下 ExtOutput 拷贝逻辑。
func TestToInvokeOutputDataDO_ExtOutputFields(t *testing.T) {
	t.Parallel()

	successStatus := spi.InvokeEvalTargetStatus_SUCCESS
	contentType := spi.ContentTypeText
	req := &openapi.ReportEvalTargetInvokeResultRequest{
		Status: &successStatus,
		Output: &spi.InvokeEvalTargetOutput{
			ExtOutput: map[string]*spi.Content{
				"extra": {ContentType: &contentType, Text: gptr.Of("extra-val")},
			},
		},
	}
	out := ToInvokeOutputDataDO(req)
	if assert.NotNil(t, out) {
		if assert.Contains(t, out.OutputFields, "extra") {
			assert.Equal(t, "extra-val", *out.OutputFields["extra"].Text)
		}
	}
}
