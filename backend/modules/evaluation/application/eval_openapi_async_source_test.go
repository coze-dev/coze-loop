// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

// Removing the real entrypoint's record-derived source or Event copy must fail these assertions.
func assertCallbackResumeSource(t *testing.T, event *entity.ExptItemEvalEvent, kind string, recordID, turnID int64) {
	t.Helper()
	encoded, err := json.Marshal(event)
	require.NoError(t, err)
	var wire struct {
		ResumeSource *struct {
			Kind     string
			TurnID   int64
			InvokeID string
			RecordID int64
		}
	}
	require.NoError(t, json.Unmarshal(encoded, &wire))
	require.NotNil(t, wire.ResumeSource, "real callback must publish the original record source")
	assert.Equal(t, kind, wire.ResumeSource.Kind)
	assert.Equal(t, recordID, wire.ResumeSource.RecordID)
	assert.Equal(t, strconv.FormatInt(recordID, 10), wire.ResumeSource.InvokeID)
	assert.Equal(t, turnID, wire.ResumeSource.TurnID)
}

func TestAsyncSourceTargetCallback(t *testing.T) {
	for _, readCase := range []string{"success", "legacy empty request", "read failure", "missing record", "missing turn", "wrong record"} {
		t.Run(readCase, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			asyncRepo := repomocks.NewMockIEvalAsyncRepo(ctrl)
			targetSvc := servicemocks.NewMockIEvalTargetService(ctrl)
			publisher := eventmocks.NewMockExptEventPublisher(ctrl)
			configer := configmocks.NewMockIConfiger(ctrl)
			const recordID, turnID = int64(7384603622000000101), int64(7384603622000000102)
			original := &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 2, ExptRunID: 3, EvalSetItemID: 4, RequestID: "original-rq", Ext: map[string]string{"inputExpr": "original"}, Session: &entity.Session{UserID: "owner"}}
			if readCase == "legacy empty request" {
				original.RequestID = ""
			}
			actx := &entity.EvalAsyncCtx{Event: original, RecordID: recordID, Session: original.Session}
			asyncRepo.EXPECT().GetEvalAsyncCtx(gomock.Any(), strconv.FormatInt(recordID, 10)).Return(actx, nil).Times(2)
			written := 0
			targetSvc.EXPECT().ReportInvokeRecords(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *entity.ReportTargetRecordParam) error {
				assert.Equal(t, recordID, p.RecordID)
				written++
				return nil
			}).Times(2)
			record := &entity.EvalTargetRecord{ID: recordID, TurnID: turnID, ExperimentRunID: 3, ItemID: 4}
			var readErr error
			if readCase == "read failure" {
				readErr = errors.New("source storage unavailable")
			}
			if readCase == "missing record" {
				record = nil
			}
			if readCase == "missing turn" {
				record.TurnID = 0
			}
			if readCase == "wrong record" {
				record.ID++
			}
			// Optional before implementation, so the red result is a source assertion, not an unmet mock.
			targetSvc.EXPECT().GetRecordByID(gomock.Any(), int64(1), recordID).Return(record, readErr).AnyTimes()
			configer.EXPECT().GetTargetTrajectoryConf(gomock.Any()).Return(&entity.TargetTrajectoryConf{}).AnyTimes()
			var published []*entity.ExptItemEvalEvent
			publisher.EXPECT().PublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, event *entity.ExptItemEvalEvent, _ *time.Duration, modify func(*entity.ExptItemEvalEvent)) error {
				assert.Greater(t, written, len(published), "record write must precede Publish")
				assert.NotSame(t, original, event)
				assert.Equal(t, original.RequestID, event.RequestID)
				assert.Equal(t, "original", event.Ext["inputExpr"])
				assert.Equal(t, "owner", event.Session.UserID)
				assert.Equal(t, int64(2), event.ExptID)
				assert.Equal(t, int64(3), event.ExptRunID)
				assert.Equal(t, int64(4), event.EvalSetItemID)
				assertCallbackResumeSource(t, event, "target", recordID, turnID)
				modify(event)
				published = append(published, event)
				event.Ext["inputExpr"] = "publisher mutation"
				event.Session.UserID = "publisher mutation"
				return nil
			}).AnyTimes()
			app := &EvalOpenAPIApplication{asyncRepo: asyncRepo, targetSvc: targetSvc, publisher: publisher, configer: configer}
			for i := 0; i < 2; i++ {
				_, err := app.ReportEvalTargetInvokeResult_(context.Background(), newSuccessInvokeResultReq(1, recordID))
				if readCase == "success" || readCase == "legacy empty request" {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
				}
			}
			assert.Equal(t, 2, written, "old records remain writable")
			assert.False(t, original.AsyncReportTrigger)
			assert.Equal(t, "original", original.Ext["inputExpr"])
			assert.Equal(t, "owner", original.Session.UserID)
			if readCase == "success" || readCase == "legacy empty request" {
				require.Len(t, published, 2)
				assert.NotSame(t, published[0], published[1])
			} else {
				assert.Empty(t, published)
			}
		})
	}
}

func TestAsyncSourceEvaluatorCallback(t *testing.T) {
	for _, outcome := range []entity.ReportEvaluatorResultOutcome{entity.ReportEvaluatorResultApplied, entity.ReportEvaluatorResultDuplicate, entity.ReportEvaluatorResultConflict} {
		t.Run(strconv.Itoa(int(outcome)), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			asyncRepo := repomocks.NewMockIEvalAsyncRepo(ctrl)
			svc := servicemocks.NewMockEvaluatorService(ctrl)
			recordSvc := servicemocks.NewMockEvaluatorRecordService(ctrl)
			publisher := eventmocks.NewMockExptEventPublisher(ctrl)
			const recordID, turnID = int64(7384603622000000201), int64(7384603622000000202)
			original := &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 2, ExptRunID: 3, EvalSetItemID: 4, RequestID: "original-rq", Ext: map[string]string{"inputExpr": "original"}, Session: &entity.Session{UserID: "owner"}}
			actx := &entity.EvalAsyncCtx{Event: original, RecordID: recordID, ResumeReady: true}
			auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
			asyncRepo.EXPECT().GetEvalAsyncCtxStrong(gomock.Any(), "evaluator:"+strconv.FormatInt(recordID, 10)).Return(actx, nil)
			written := false
			svc.EXPECT().ReportEvaluatorInvokeResult(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *entity.ReportEvaluatorRecordParam) (entity.ReportEvaluatorResultOutcome, error) {
				assert.Equal(t, recordID, p.RecordID)
				written = true
				return outcome, nil
			})
			recordSvc.EXPECT().GetEvaluatorRecord(gomock.Any(), recordID, false).Return(&entity.EvaluatorRecord{ID: recordID, TurnID: turnID, Status: entity.EvaluatorRunStatusSuccess}, nil).AnyTimes()
			published := false
			publisher.EXPECT().PublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, event *entity.ExptItemEvalEvent, _ *time.Duration, modify func(*entity.ExptItemEvalEvent)) error {
				require.True(t, written)
				assert.NotSame(t, original, event)
				assert.Equal(t, "original-rq", event.RequestID)
				assert.Equal(t, original.ExptID, event.ExptID)
				assert.Equal(t, original.ExptRunID, event.ExptRunID)
				assert.Equal(t, original.EvalSetItemID, event.EvalSetItemID)
				assertCallbackResumeSource(t, event, "evaluator", recordID, turnID)
				modify(event)
				event.Ext["inputExpr"] = "mutation"
				event.Session.UserID = "mutation"
				published = true
				return nil
			}).AnyTimes()
			app := &EvalOpenAPIApplication{auth: auth, asyncRepo: asyncRepo, evaluatorService: svc, evaluatorRecordService: recordSvc, publisher: publisher}
			_, err := app.ReportEvaluatorInvokeResult_(context.Background(), &openapi.ReportEvaluatorInvokeResultRequest{WorkspaceID: gptr.Of(int64(1)), InvokeID: gptr.Of(recordID), Status: gptr.Of(spi.InvokeEvaluatorRunStatus_SUCCESS), Output: &spi.InvokeEvaluatorOutputData{EvaluatorResult_: &spi.InvokeEvaluatorResult_{}}})
			require.NoError(t, err)
			assert.Equal(t, outcome != entity.ReportEvaluatorResultConflict, published)
			assert.False(t, original.AsyncEvaluatorReportTrigger)
			assert.Equal(t, "original", original.Ext["inputExpr"])
			assert.Equal(t, "owner", original.Session.UserID)
		})
	}
}

func TestAsyncSourceEvaluatorReadinessKeepsOriginalEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	auth := rpcmocks.NewMockIAuthProvider(ctrl)
	asyncRepo := repomocks.NewMockIEvalAsyncRepo(ctrl)
	svc := servicemocks.NewMockEvaluatorService(ctrl)
	recordSvc := servicemocks.NewMockEvaluatorRecordService(ctrl)
	publisher := eventmocks.NewMockExptEventPublisher(ctrl)
	original := &entity.ExptItemEvalEvent{ExptID: 2, ExptRunID: 3, EvalSetItemID: 4, RequestID: "original-rq"}
	auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
	gomock.InOrder(
		asyncRepo.EXPECT().GetEvalAsyncCtxStrong(gomock.Any(), "evaluator:100").Return(&entity.EvalAsyncCtx{Event: original}, nil),
		svc.EXPECT().ReportEvaluatorInvokeResult(gomock.Any(), gomock.Any()).Return(entity.ReportEvaluatorResultDuplicate, nil),
		asyncRepo.EXPECT().GetEvalAsyncCtxStrong(gomock.Any(), "evaluator:100").Return(&entity.EvalAsyncCtx{ResumeReady: true, Event: &entity.ExptItemEvalEvent{ExptID: 20, ExptRunID: 30, EvalSetItemID: 40, RequestID: "other-rq"}}, nil),
	)
	recordSvc.EXPECT().GetEvaluatorRecord(gomock.Any(), int64(100), false).Return(&entity.EvaluatorRecord{ID: 100, TurnID: 5, Status: entity.EvaluatorRunStatusSuccess}, nil)
	publisher.EXPECT().PublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, ev *entity.ExptItemEvalEvent, _ *time.Duration, _ func(*entity.ExptItemEvalEvent)) error {
		assert.Equal(t, "original-rq", ev.RequestID, "readiness reread cannot relabel a callback")
		assert.Equal(t, original.ExptID, ev.ExptID)
		assert.Equal(t, original.ExptRunID, ev.ExptRunID)
		assert.Equal(t, original.EvalSetItemID, ev.EvalSetItemID)
		assertCallbackResumeSource(t, ev, "evaluator", 100, 5)
		return nil
	})
	app := &EvalOpenAPIApplication{auth: auth, asyncRepo: asyncRepo, evaluatorService: svc, evaluatorRecordService: recordSvc, publisher: publisher}
	_, err := app.ReportEvaluatorInvokeResult_(context.Background(), &openapi.ReportEvaluatorInvokeResultRequest{WorkspaceID: gptr.Of(int64(1)), InvokeID: gptr.Of(int64(100)), Status: gptr.Of(spi.InvokeEvaluatorRunStatus_SUCCESS), Output: &spi.InvokeEvaluatorOutputData{EvaluatorResult_: &spi.InvokeEvaluatorResult_{}}})
	require.NoError(t, err)
}

func TestAsyncSourceEvaluatorReadFailures(t *testing.T) {
	for _, sourceCase := range []string{"read failure", "missing", "missing turn", "wrong record", "still invoking"} {
		t.Run(sourceCase, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			asyncRepo := repomocks.NewMockIEvalAsyncRepo(ctrl)
			svc := servicemocks.NewMockEvaluatorService(ctrl)
			recordSvc := servicemocks.NewMockEvaluatorRecordService(ctrl)
			publisher := eventmocks.NewMockExptEventPublisher(ctrl)
			auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
			asyncRepo.EXPECT().GetEvalAsyncCtxStrong(gomock.Any(), "evaluator:100").Return(&entity.EvalAsyncCtx{Event: &entity.ExptItemEvalEvent{RequestID: "original-rq"}, ResumeReady: true}, nil)
			svc.EXPECT().ReportEvaluatorInvokeResult(gomock.Any(), gomock.Any()).Return(entity.ReportEvaluatorResultDuplicate, nil)
			record := &entity.EvaluatorRecord{ID: 100, TurnID: 5, Status: entity.EvaluatorRunStatusSuccess}
			var readErr error
			switch sourceCase {
			case "read failure":
				readErr = errors.New("source storage unavailable")
			case "missing":
				record = nil
			case "missing turn":
				record.TurnID = 0
			case "wrong record":
				record.ID = 200
			case "still invoking":
				record.Status = entity.EvaluatorRunStatusAsyncInvoking
			}
			recordSvc.EXPECT().GetEvaluatorRecord(gomock.Any(), int64(100), false).Return(record, readErr)
			app := &EvalOpenAPIApplication{auth: auth, asyncRepo: asyncRepo, evaluatorService: svc, evaluatorRecordService: recordSvc, publisher: publisher}
			_, err := app.ReportEvaluatorInvokeResult_(context.Background(), &openapi.ReportEvaluatorInvokeResultRequest{WorkspaceID: gptr.Of(int64(1)), InvokeID: gptr.Of(int64(100)), Status: gptr.Of(spi.InvokeEvaluatorRunStatus_SUCCESS), Output: &spi.InvokeEvaluatorOutputData{EvaluatorResult_: &spi.InvokeEvaluatorResult_{}}})
			require.Error(t, err)
		})
	}
}
