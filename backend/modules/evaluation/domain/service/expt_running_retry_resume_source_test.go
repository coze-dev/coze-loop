// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	datasetdto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/dataset"
	datasetentity "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	datasetadapter "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/rpc/data"
	datasetmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/rpc/data/mocks"
)

func TestAsyncSourceFixArmTurn0(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := datasetmocks.NewMockClient(ctrl)
	client.EXPECT().BatchGetDatasetItems(gomock.Any(), gomock.Any()).Return(&datasetdto.BatchGetDatasetItemsResponse{Items: []*datasetentity.DatasetItem{{ItemID: gptr.Of(int64(4)), Data: []*datasetentity.FieldData{{Name: gptr.Of("input"), Content: gptr.Of("single turn")}}}}}, nil)
	items, err := datasetadapter.NewDatasetRPCAdapter(client).BatchGetDatasetItems(context.Background(), &rpc.BatchGetDatasetItemsParam{})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Len(t, items[0].Turns, 1)
	turn := items[0].Turns[0]
	require.Zero(t, turn.ID)
	asyncRepo := repomocks.NewMockIEvalAsyncRepo(ctrl)
	recordRepo := repomocks.NewMockIEvaluatorRecordRepo(ctrl)
	publisher := eventmocks.NewMockExptEventPublisher(ctrl)
	original := &entity.ExptItemEvalEvent{ExptID: 2, ExptRunID: 3, EvalSetItemID: turn.ItemID, RequestID: "original-rq"}
	asyncRepo.EXPECT().MarkEvalAsyncResumeReady(gomock.Any(), "evaluator:100").Return(&entity.EvalAsyncCtx{Event: original, ResumeReady: true}, nil)
	recordRepo.EXPECT().GetEvaluatorRecord(gomock.Any(), int64(100), false).Return(&entity.EvaluatorRecord{ID: 100, TurnID: turn.ID, Status: entity.EvaluatorRunStatusSuccess}, nil)
	published := 0
	publisher.EXPECT().PublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, event *entity.ExptItemEvalEvent, _ *time.Duration, _ func(*entity.ExptItemEvalEvent)) error {
		require.NotNil(t, event.ResumeSource)
		assert.Zero(t, event.ResumeSource.TurnID)
		assert.Equal(t, int64(100), event.ResumeSource.RecordID)
		assert.Equal(t, "100", event.ResumeSource.InvokeID)
		assert.Equal(t, "evaluator", event.ResumeSource.Kind)
		assert.Equal(t, original.RequestID, event.RequestID)
		bytes, err := json.Marshal(event)
		require.NoError(t, err)
		var roundtrip entity.ExptItemEvalEvent
		require.NoError(t, json.Unmarshal(bytes, &roundtrip))
		require.NotNil(t, roundtrip.ResumeSource)
		assert.Zero(t, roundtrip.ResumeSource.TurnID)
		published++
		return nil
	}).MaxTimes(1)
	svc := &EvaluatorServiceImpl{evalAsyncRepo: asyncRepo, evaluatorRecordRepo: recordRepo, exptEventPublisher: publisher}
	require.NoError(t, svc.ArmEvaluatorResume(context.Background(), 100))
	assert.Equal(t, 1, published)
}

func TestAsyncSourceFixArmInvalidSource(t *testing.T) {
	for _, source := range []string{"negative turn", "wrong record"} {
		t.Run(source, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			asyncRepo := repomocks.NewMockIEvalAsyncRepo(ctrl)
			recordRepo := repomocks.NewMockIEvaluatorRecordRepo(ctrl)
			publisher := eventmocks.NewMockExptEventPublisher(ctrl)
			asyncRepo.EXPECT().MarkEvalAsyncResumeReady(gomock.Any(), "evaluator:100").Return(&entity.EvalAsyncCtx{Event: &entity.ExptItemEvalEvent{RequestID: "original-rq"}, ResumeReady: true}, nil)
			record := &entity.EvaluatorRecord{ID: 100, TurnID: 0, Status: entity.EvaluatorRunStatusSuccess}
			if source == "negative turn" {
				record.TurnID = -1
			} else {
				record.ID = 200
			}
			recordRepo.EXPECT().GetEvaluatorRecord(gomock.Any(), int64(100), false).Return(record, nil)
			svc := &EvaluatorServiceImpl{evalAsyncRepo: asyncRepo, evaluatorRecordRepo: recordRepo, exptEventPublisher: publisher}
			require.Error(t, svc.ArmEvaluatorResume(context.Background(), 100))
		})
	}
}

func TestAsyncSourceArmEvaluatorResume(t *testing.T) {
	ctrl := gomock.NewController(t)
	asyncRepo := repomocks.NewMockIEvalAsyncRepo(ctrl)
	recordRepo := repomocks.NewMockIEvaluatorRecordRepo(ctrl)
	publisher := eventmocks.NewMockExptEventPublisher(ctrl)
	const recordID, turnID = int64(7384603622000000301), int64(7384603622000000302)
	original := &entity.ExptItemEvalEvent{ExptID: 2, ExptRunID: 3, EvalSetItemID: 4, RequestID: "original-rq", Ext: map[string]string{"inputExpr": "original"}, Session: &entity.Session{UserID: "owner"}}
	asyncRepo.EXPECT().MarkEvalAsyncResumeReady(gomock.Any(), "evaluator:"+strconv.FormatInt(recordID, 10)).Return(&entity.EvalAsyncCtx{Event: original, ResumeReady: true}, nil)
	recordRepo.EXPECT().GetEvaluatorRecord(gomock.Any(), recordID, false).Return(&entity.EvaluatorRecord{ID: recordID, TurnID: turnID, Status: entity.EvaluatorRunStatusSuccess}, nil)
	var events []*entity.ExptItemEvalEvent
	publisher.EXPECT().PublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, ev *entity.ExptItemEvalEvent, _ *time.Duration, modify func(*entity.ExptItemEvalEvent)) error {
		assert.NotSame(t, original, ev)
		assert.Equal(t, "original-rq", ev.RequestID)
		assert.Equal(t, "original", ev.Ext["inputExpr"])
		assert.Equal(t, "owner", ev.Session.UserID)
		encoded, err := json.Marshal(ev)
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
		require.NotNil(t, wire.ResumeSource, "Arm must publish the original evaluator record source")
		assert.Equal(t, "evaluator", wire.ResumeSource.Kind)
		assert.Equal(t, turnID, wire.ResumeSource.TurnID)
		assert.Equal(t, recordID, wire.ResumeSource.RecordID)
		assert.Equal(t, strconv.FormatInt(recordID, 10), wire.ResumeSource.InvokeID)
		modify(ev)
		ev.Ext["inputExpr"] = "mutation"
		ev.Session.UserID = "mutation"
		events = append(events, ev)
		if len(events) == 1 {
			ev.ResumeSource.TurnID = 99
			return errors.New("publish retry")
		}
		return nil
	}).Times(2)
	s := &EvaluatorServiceImpl{evalAsyncRepo: asyncRepo, evaluatorRecordRepo: recordRepo, exptEventPublisher: publisher}
	require.NoError(t, s.ArmEvaluatorResume(context.Background(), recordID))
	require.Len(t, events, 2)
	assert.NotSame(t, events[0], events[1])
	assert.False(t, original.AsyncEvaluatorReportTrigger)
	assert.Equal(t, "original", original.Ext["inputExpr"])
	assert.Equal(t, "owner", original.Session.UserID)
}

// Mutating one stored context must not change the item event or another evaluator's context.
func TestAsyncSourceEvaluatorContextCopies(t *testing.T) {
	ctrl := gomock.NewController(t)
	metric := metricsmocks.NewMockExptMetric(ctrl)
	svc := svcmocks.NewMockEvaluatorService(ctrl)
	s := &DefaultExptTurnEvaluationImpl{metric: metric, evaluatorService: svc}
	original := &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 2, ExptRunID: 3, EvalSetItemID: 4, RequestID: "original-rq", Ext: map[string]string{"inputExpr": "original"}, Session: &entity.Session{UserID: "owner"}}
	// Decode optional metadata so this test compiles before the new field exists.
	require.NoError(t, json.Unmarshal([]byte(`{"ResumeSource":{"Kind":"target","TurnID":7384603622000000401,"RecordID":7384603622000000402,"InvokeID":"7384603622000000402"}}`), original))
	etec := &entity.ExptTurnEvalCtx{ExptItemEvalCtx: &entity.ExptItemEvalCtx{Event: original, EvalSetItem: &entity.EvaluationSetItem{ItemID: 4}}, Turn: &entity.Turn{ID: 5}, Ext: map[string]string{"inputExpr": "eval expression"}}
	ev := &entity.Evaluator{EvaluatorType: entity.EvaluatorTypeAgent, AgentEvaluatorVersion: &entity.AgentEvaluatorVersion{ID: 6}}
	input := &entity.EvaluatorInputData{Ext: map[string]string{"inputExpr": "input expression"}}
	metric.EXPECT().EmitTurnExecEvaluatorResult(int64(1), false).Times(2)
	var saved []*entity.EvalAsyncCtx
	svc.EXPECT().AsyncRunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *entity.AsyncRunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
		require.NotNil(t, req.AsyncCtx)
		assert.NotSame(t, original, req.AsyncCtx.Event)
		assert.NotSame(t, original.Session, req.AsyncCtx.Session)
		assert.NotSame(t, req.AsyncCtx.Session, req.AsyncCtx.Event.Session)
		assert.Same(t, input, req.InputData)
		assert.Equal(t, "eval expression", req.Ext["inputExpr"])
		saved = append(saved, req.AsyncCtx)
		return &entity.EvaluatorRecord{ID: int64(len(saved)), TurnID: 5}, nil
	}).Times(2)
	collector := &evalRecordCollector{}
	require.NoError(t, s.asyncCallEvaluator(context.Background(), ev, &entity.EvaluatorConf{}, etec, input, collector))
	require.NoError(t, s.asyncCallEvaluatorWithAlias(context.Background(), ev, nil, "alias", etec, input, collector))
	require.Len(t, saved, 2)
	saved[0].Event.Ext["inputExpr"] = "mutation"
	saved[0].Event.Session.UserID = "mutation"
	saved[0].Session.UserID = "async mutation"
	saved[0].Event.AsyncEvaluatorReportTrigger = true
	require.NotNil(t, saved[0].Event.ResumeSource)
	saved[0].Event.ResumeSource.TurnID = 99
	assert.Equal(t, int64(7384603622000000401), saved[1].Event.ResumeSource.TurnID)
	assert.Equal(t, int64(7384603622000000401), original.ResumeSource.TurnID)
	assert.Equal(t, "original", saved[1].Event.Ext["inputExpr"])
	assert.Equal(t, "owner", saved[1].Session.UserID)
	assert.Equal(t, "owner", saved[1].Event.Session.UserID)
	assert.False(t, saved[1].Event.AsyncEvaluatorReportTrigger)
	assert.Equal(t, "original", original.Ext["inputExpr"])
	assert.Equal(t, "owner", original.Session.UserID)
	assert.False(t, original.AsyncEvaluatorReportTrigger)
}

func TestAsyncSourceContextRoundTripAndConcurrentCopies(t *testing.T) {
	const recordID, turnID = int64(7384603622000000402), int64(7384603622000000401)
	original := &entity.EvalAsyncCtx{
		Event:    &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 2, ExptRunID: 3, EvalSetItemID: 4, ExptRunMode: entity.ExptRunMode(2), RetryTimes: 2, MaxRetryTimes: 3, CreateAt: 123, Ext: map[string]string{"inputExpr": "expression", "manual_retry": `{"request_id":"rq"}`}, Session: &entity.Session{UserID: "owner", AppID: 6}, ResumeSource: &entity.ExptResumeSource{Kind: "target", TurnID: turnID, RecordID: recordID, InvokeID: strconv.FormatInt(recordID, 10)}},
		RecordID: recordID, AsyncUnixMS: 1234, Session: &entity.Session{UserID: "reporter"}, EnableExtractTrajectory: new(bool), ResumeReady: true, CallbackURL: "callback", Callee: "callee", EvaluatorVersionID: 7, TargetID: 8, DatasetID: 9, DatasetVersionID: 10, ItemKey: "item", DatasetKey: "dataset", AgentName: "agent", ApplicationID: "application",
	}
	encoded, err := json.Marshal(original)
	require.NoError(t, err)
	var roundtrip entity.EvalAsyncCtx
	require.NoError(t, json.Unmarshal(encoded, &roundtrip))
	assert.Equal(t, original, &roundtrip, "all existing fields and 19-digit IDs survive AsyncCtx JSON")
	var old entity.EvalAsyncCtx
	require.NoError(t, json.Unmarshal([]byte(`{"Event":{"ExptID":2,"ExptRunID":3,"EvalSetItemID":4},"RecordID":7384603622000000402}`), &old))
	assert.Nil(t, old.Event.ResumeSource)
	assert.Empty(t, old.Event.RequestID)
	oldBytes, err := json.Marshal(&old)
	require.NoError(t, err)
	assert.NotContains(t, string(oldBytes), "ResumeSource")
	assert.NotContains(t, string(oldBytes), "RequestID")
	var nilEvent *entity.ExptItemEvalEvent
	var nilAsync *entity.EvalAsyncCtx
	assert.Nil(t, nilEvent.Clone())
	assert.Nil(t, nilAsync.Clone())
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			copy := original.Clone()
			copy.Event.Ext["inputExpr"] = "mutation"
			copy.Event.Session.UserID = "mutation"
			copy.Session.UserID = "mutation"
			copy.Event.ResumeSource.TurnID = 99
			*copy.EnableExtractTrajectory = true
		}()
	}
	wg.Wait()
	assert.Equal(t, "expression", original.Event.Ext["inputExpr"])
	assert.Equal(t, "owner", original.Event.Session.UserID)
	assert.Equal(t, "reporter", original.Session.UserID)
	assert.Equal(t, turnID, original.Event.ResumeSource.TurnID)
	assert.False(t, *original.EnableExtractTrajectory)
}

func assertIndependentTargetAsyncContext(t *testing.T, original *entity.ExptItemEvalEvent, actx *entity.EvalAsyncCtx) {
	t.Helper()
	assert.NotSame(t, original, actx.Event)
	assert.NotSame(t, original.Session, actx.Session)
	assert.NotSame(t, actx.Event.Session, actx.Session)
	actx.Event.AsyncReportTrigger = true
	actx.Event.Session.UserID = "mutation"
	assert.False(t, original.AsyncReportTrigger)
	assert.Equal(t, "user", original.Session.UserID)
	assert.Equal(t, "user", actx.Session.UserID)
}
