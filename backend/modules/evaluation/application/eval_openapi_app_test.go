// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

func TestEvalOpenAPIApplication_ReportEvalTargetInvokeResult(t *testing.T) {
	t.Parallel()

	repoErrorReq := newSuccessInvokeResultReq(11, 101)
	reportErrorReq := newSuccessInvokeResultReq(22, 202)
	publisherErrorReq := newSuccessInvokeResultReq(33, 303)
	successReq := newSuccessInvokeResultReq(44, 404)
	failedReq := newFailedInvokeResultReq(55, 505, "invoke failed")

	tests := []struct {
		name    string
		req     *openapi.ReportEvalTargetInvokeResultRequest
		setup   func(t *testing.T, asyncRepo *repomocks.MockIEvalAsyncRepo, targetSvc *servicemocks.MockIEvalTargetService, publisher *eventmocks.MockExptEventPublisher)
		wantErr bool
	}{
		{
			name: "repo returns error",
			req:  repoErrorReq,
			setup: func(t *testing.T, asyncRepo *repomocks.MockIEvalAsyncRepo, _ *servicemocks.MockIEvalTargetService, _ *eventmocks.MockExptEventPublisher) {
				asyncRepo.EXPECT().GetEvalAsyncCtx(gomock.Any(), strconv.FormatInt(repoErrorReq.GetInvokeID(), 10)).Return(nil, errors.New("repo error"))
			},
			wantErr: true,
		},
		{
			name: "report invoke records returns error",
			req:  reportErrorReq,
			setup: func(t *testing.T, asyncRepo *repomocks.MockIEvalAsyncRepo, targetSvc *servicemocks.MockIEvalTargetService, publisher *eventmocks.MockExptEventPublisher) {
				actx := &entity.EvalAsyncCtx{AsyncUnixMS: time.Now().Add(-200 * time.Millisecond).UnixMilli()}
				asyncRepo.EXPECT().GetEvalAsyncCtx(gomock.Any(), strconv.FormatInt(reportErrorReq.GetInvokeID(), 10)).Return(actx, nil)
				targetSvc.EXPECT().ReportInvokeRecords(gomock.Any(), gomock.AssignableToTypeOf(&entity.ReportTargetRecordParam{})).DoAndReturn(func(_ context.Context, param *entity.ReportTargetRecordParam) error {
					assert.Equal(t, reportErrorReq.GetWorkspaceID(), param.SpaceID)
					assert.Equal(t, reportErrorReq.GetInvokeID(), param.RecordID)
					assert.Equal(t, entity.EvalTargetRunStatusSuccess, param.Status)
					if assert.NotNil(t, param.OutputData) {
						assert.NotNil(t, param.OutputData.EvalTargetUsage)
						assert.NotNil(t, param.OutputData.TimeConsumingMS)
						if param.OutputData.TimeConsumingMS != nil {
							assert.Greater(t, *param.OutputData.TimeConsumingMS, int64(0))
						}
					}
					assert.Nil(t, param.Session)
					return errors.New("report error")
				})
				publisher.EXPECT().PublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr: true,
		},
		{
			name: "publisher returns error",
			req:  publisherErrorReq,
			setup: func(t *testing.T, asyncRepo *repomocks.MockIEvalAsyncRepo, targetSvc *servicemocks.MockIEvalTargetService, publisher *eventmocks.MockExptEventPublisher) {
				session := &entity.Session{UserID: "user"}
				event := &entity.ExptItemEvalEvent{}
				actx := &entity.EvalAsyncCtx{AsyncUnixMS: time.Now().Add(-150 * time.Millisecond).UnixMilli(), Event: event, Session: session}
				asyncRepo.EXPECT().GetEvalAsyncCtx(gomock.Any(), strconv.FormatInt(publisherErrorReq.GetInvokeID(), 10)).Return(actx, nil)
				targetSvc.EXPECT().ReportInvokeRecords(gomock.Any(), gomock.AssignableToTypeOf(&entity.ReportTargetRecordParam{})).DoAndReturn(func(_ context.Context, param *entity.ReportTargetRecordParam) error {
					assert.Equal(t, session, param.Session)
					return nil
				})
				publisher.EXPECT().PublishExptRecordEvalEvent(gomock.Any(), event, gomock.Any()).DoAndReturn(func(_ context.Context, evt *entity.ExptItemEvalEvent, duration *time.Duration) error {
					assert.Equal(t, event, evt)
					if assert.NotNil(t, duration) {
						assert.Equal(t, 3*time.Second, *duration)
					}
					return errors.New("publish error")
				})
			},
			wantErr: true,
		},
		{
			name: "success without event",
			req:  successReq,
			setup: func(t *testing.T, asyncRepo *repomocks.MockIEvalAsyncRepo, targetSvc *servicemocks.MockIEvalTargetService, publisher *eventmocks.MockExptEventPublisher) {
				actx := &entity.EvalAsyncCtx{AsyncUnixMS: time.Now().Add(-100 * time.Millisecond).UnixMilli()}
				asyncRepo.EXPECT().GetEvalAsyncCtx(gomock.Any(), strconv.FormatInt(successReq.GetInvokeID(), 10)).Return(actx, nil)
				targetSvc.EXPECT().ReportInvokeRecords(gomock.Any(), gomock.AssignableToTypeOf(&entity.ReportTargetRecordParam{})).DoAndReturn(func(_ context.Context, param *entity.ReportTargetRecordParam) error {
					assert.Nil(t, param.Session)
					return nil
				})
				publisher.EXPECT().PublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr: false,
		},
		{
			name: "success with event on failure status",
			req:  failedReq,
			setup: func(t *testing.T, asyncRepo *repomocks.MockIEvalAsyncRepo, targetSvc *servicemocks.MockIEvalTargetService, publisher *eventmocks.MockExptEventPublisher) {
				session := &entity.Session{UserID: "owner"}
				event := &entity.ExptItemEvalEvent{}
				actx := &entity.EvalAsyncCtx{AsyncUnixMS: time.Now().Add(-120 * time.Millisecond).UnixMilli(), Event: event, Session: session}
				asyncRepo.EXPECT().GetEvalAsyncCtx(gomock.Any(), strconv.FormatInt(failedReq.GetInvokeID(), 10)).Return(actx, nil)
				targetSvc.EXPECT().ReportInvokeRecords(gomock.Any(), gomock.AssignableToTypeOf(&entity.ReportTargetRecordParam{})).DoAndReturn(func(_ context.Context, param *entity.ReportTargetRecordParam) error {
					assert.Equal(t, entity.EvalTargetRunStatusFail, param.Status)
					if assert.NotNil(t, param.OutputData) {
						if assert.NotNil(t, param.OutputData.EvalTargetRunError) {
							assert.Equal(t, failedReq.GetErrorMessage(), param.OutputData.EvalTargetRunError.Message)
						}
						assert.NotNil(t, param.OutputData.TimeConsumingMS)
					}
					assert.Equal(t, session, param.Session)
					return nil
				})
				publisher.EXPECT().PublishExptRecordEvalEvent(gomock.Any(), event, gomock.Any()).DoAndReturn(func(_ context.Context, evt *entity.ExptItemEvalEvent, duration *time.Duration) error {
					assert.Equal(t, event, evt)
					if assert.NotNil(t, duration) {
						assert.Equal(t, 3*time.Second, *duration)
					}
					return nil
				})
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		caseData := tc
		t.Run(caseData.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			asyncRepo := repomocks.NewMockIEvalAsyncRepo(ctrl)
			targetSvc := servicemocks.NewMockIEvalTargetService(ctrl)
			publisher := eventmocks.NewMockExptEventPublisher(ctrl)

			app := &EvalOpenAPIApplication{
				targetSvc: targetSvc,
				asyncRepo: asyncRepo,
				publisher: publisher,
			}

			caseData.setup(t, asyncRepo, targetSvc, publisher)

			resp, err := app.ReportEvalTargetInvokeResult_(context.Background(), caseData.req)
			if caseData.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
				return
			}

			assert.NoError(t, err)
			if assert.NotNil(t, resp) {
				assert.NotNil(t, resp.BaseResp)
			}
		})
	}
}

func newSuccessInvokeResultReq(workspaceID, invokeID int64) *openapi.ReportEvalTargetInvokeResultRequest {
	status := spi.InvokeEvalTargetStatus_SUCCESS
	contentType := spi.ContentTypeText
	text := "result"
	inputTokens := int64(10)
	outputTokens := int64(20)

	return &openapi.ReportEvalTargetInvokeResultRequest{
		WorkspaceID: gptr.Of(workspaceID),
		InvokeID:    gptr.Of(invokeID),
		Status:      &status,
		Output: &spi.InvokeEvalTargetOutput{
			ActualOutput: &spi.Content{
				ContentType: &contentType,
				Text:        gptr.Of(text),
			},
		},
		Usage: &spi.InvokeEvalTargetUsage{
			InputTokens:  &inputTokens,
			OutputTokens: &outputTokens,
		},
	}
}

func newFailedInvokeResultReq(workspaceID, invokeID int64, errorMessage string) *openapi.ReportEvalTargetInvokeResultRequest {
	status := spi.InvokeEvalTargetStatus_FAILED

	return &openapi.ReportEvalTargetInvokeResultRequest{
		WorkspaceID:  gptr.Of(workspaceID),
		InvokeID:     gptr.Of(invokeID),
		Status:       &status,
		ErrorMessage: gptr.Of(errorMessage),
	}
}

func TestNewEvalOpenAPIApplication(t *testing.T) {
	app := NewEvalOpenAPIApplication(nil, nil, nil)
	assert.NotNil(t, app)
}
