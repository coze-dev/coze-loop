// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package target

import (
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

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
