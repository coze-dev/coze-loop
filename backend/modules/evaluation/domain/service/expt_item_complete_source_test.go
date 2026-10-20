// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	entitymocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity/mocks"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	runtimejson "github.com/coze-dev/coze-loop/backend/pkg/json"
)

const itemCompleteSourceBase int64 = 7384603622000000000

func itemCompleteSourcePublication() *entity.ExptItemResultPublication {
	b := itemCompleteSourceBase
	c := entity.ExptItemCurrentRequest{SpaceID: b + 1, ExptID: b + 2, RunID: b + 3, ItemID: b + 4, ItemVersionID: b + 5, RequestID: "f059e038-4d6e-41c8-a20d-68f8801adeaf"}
	turns := []*entity.ExptTurnResult{
		{ID: b + 10, SpaceID: c.SpaceID, ExptID: c.ExptID, ExptRunID: c.RunID, ItemID: c.ItemID, ItemVersionID: c.ItemVersionID, TurnID: 0, TargetResultID: 0, Status: int32(entity.TurnRunState_Success)},
		{ID: b + 11, SpaceID: c.SpaceID, ExptID: c.ExptID, ExptRunID: c.RunID, ItemID: c.ItemID, ItemVersionID: c.ItemVersionID, TurnID: b + 20, TargetResultID: b + 21, Status: int32(entity.TurnRunState_Success)},
	}
	source := &entity.ExptCurrentItemRunSource{Current: c, CanonicalTurns: turns, Turns: map[int64]*entity.ExptTurnResultRunLog{}}
	for _, turn := range turns {
		source.Turns[turn.TurnID] = turn.ToRunLogDO()
	}
	source.Turns[0].EvaluatorResultIds = &entity.EvaluatorResults{
		Registered: []*entity.RegisteredEvalResult{{VersionID: b + 30, Alias: "z", RecordID: b + 40}, {VersionID: b + 30, Alias: "a", RecordID: b + 41}},
		Inline:     []*entity.InlineEvalResult{{InlineKey: "z", RecordID: b + 42}, {InlineKey: "a", RecordID: b + 43}},
	}
	source.Turns[b+20].EvaluatorResultIds = &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{b + 31: b + 44}}
	publication := &entity.ExptItemResultPublication{Source: &entity.ExptItemResultPublishSource{
		Source:  source,
		Item:    &entity.ExptItemResult{ID: b + 6, SpaceID: c.SpaceID, ExptID: c.ExptID, ExptRunID: c.RunID, ItemID: c.ItemID, ItemVersionID: c.ItemVersionID, Status: entity.ItemRunState_Success},
		ItemLog: &entity.ExptItemResultRunLog{ID: b + 7, SpaceID: c.SpaceID, ExptID: c.ExptID, ExptRunID: c.RunID, ItemID: c.ItemID, ItemVersionID: c.ItemVersionID, Status: int32(entity.ItemRunState_Success), ResultState: int32(entity.ExptItemResultStateLogged)},
	}, Turns: []*entity.ExptTurnResult{turns[1], turns[0]}}
	for _, turn := range turns {
		refs := NewTurnEvaluatorResultRefs(0, c.ExptID, turn.ID, c.SpaceID, source.Turns[turn.TurnID].EvaluatorResultIds)
		for i, ref := range refs {
			ref.ID = b + 100 + int64(i)
		}
		publication.Refs = append(publication.Refs, refs...)
	}
	return publication
}

func buildSourceEvent(p *entity.ExptItemResultPublication) (*component.ItemCompleteEvent, error) {
	c := p.Source.Source.Current
	return BuildItemCompleteEventFromSchedulerWithSource(c.SpaceID, c.ExptID, c.RunID, nil, &entity.ExptEvalItem{ItemID: c.ItemID, RequestID: c.RequestID}, nil, 0, p)
}

func TestItemCompleteSourceAssemblerStableAndOwned(t *testing.T) {
	p := itemCompleteSourcePublication()
	event, err := buildSourceEvent(p)
	require.NoError(t, err)
	body, err := runtimejson.Marshal(event)
	require.NoError(t, err)
	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &payload))
	require.JSONEq(t, `[{"turn_id":"0","target_record_id":"0","registered":[{"version_id":"7384603622000000030","alias":"a","record_id":"7384603622000000041"},{"version_id":"7384603622000000030","alias":"z","record_id":"7384603622000000040"}],"inline":[{"inline_key":"a","record_id":"7384603622000000043"},{"inline_key":"z","record_id":"7384603622000000042"}]},{"turn_id":"7384603622000000020","target_record_id":"7384603622000000021","registered":[{"version_id":"7384603622000000031","alias":"","record_id":"7384603622000000044"}]}]`, string(payload["source_turn_records"]))
	second, err := buildSourceEvent(p)
	require.NoError(t, err)
	p.Source.Source.Current.RequestID = "new-request"
	p.Source.Source.Turns[0].EvaluatorResultIds.Registered[0].RecordID++
	p.Refs[0].EvaluatorResultID++
	*second.RequestID = "mutated-event"
	second.SourceTurnRecords[0].Registered[0].RecordID = "1"
	after, err := runtimejson.Marshal(event)
	require.NoError(t, err)
	require.Equal(t, body, after, "each built event owns its request pointer and nested source slices")
}

func TestItemCompleteSourceLegacyAndRawPresence(t *testing.T) {
	legacy := BuildItemCompleteEventFromScheduler(1, 2, 3, nil, &entity.ExptEvalItem{ItemID: 4, RequestID: "must-not-invent-source"}, nil, 0)
	body, err := runtimejson.Marshal(legacy)
	require.NoError(t, err)
	require.Equal(t, `{"eval_target_workspace_id":"","eval_target_id":"","source_target_id":"","expt_workspace_id":"1","expt_id":"2","expt_run_id":"3","dataset_workspace_id":"","dataset_id":"","dataset_version_id":"","dataset_key":"","dataset_version_name":"","experiment_group_key":"","item_id":"4","item_key":"","enable_analysis":false,"created_by":""}`, string(body))
	p := itemCompleteSourcePublication()
	p.Source.Source.Current.RequestID = ""
	raw, err := buildSourceEvent(p)
	require.NoError(t, err)
	require.NotNil(t, raw.RequestID, "raw execution has an explicitly present empty request")
	rawBody, err := runtimejson.Marshal(raw)
	require.NoError(t, err)
	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rawBody, &payload))
	require.Equal(t, `""`, string(payload["request_id"]))
	require.NotEmpty(t, payload["source_turn_records"])
	p.Turns = nil
	ev, err := buildSourceEvent(p)
	require.Error(t, err)
	require.Nil(t, ev, "explicitly missing turns cannot become a legacy message")
}

func TestItemCompleteSourceVersionZeroAndRefIDsDoNotAlterIdentity(t *testing.T) {
	p := itemCompleteSourcePublication()
	p.Source.Source.Current.ItemVersionID = 0
	p.Source.Item.ItemVersionID = 0
	p.Source.ItemLog.ItemVersionID = 0
	for _, turn := range p.Turns {
		turn.ItemVersionID = 0
	}
	for _, turn := range p.Source.Source.Turns {
		turn.ItemVersionID = 0
	}
	a, err := buildSourceEvent(p)
	require.NoError(t, err)
	for _, ref := range p.Refs {
		ref.ID += 500
	}
	b, err := buildSourceEvent(p)
	require.NoError(t, err)
	aBody, err := runtimejson.Marshal(a)
	require.NoError(t, err)
	bBody, err := runtimejson.Marshal(b)
	require.NoError(t, err)
	require.Equal(t, aBody, bBody, "ref primary keys are not evaluator record identity")
}

func TestItemCompleteSourceWorkerAndSchedulerScope(t *testing.T) {
	p := itemCompleteSourcePublication()
	c := p.Source.Source.Current
	worker := &entity.ExptItemEvalCtx{Event: &entity.ExptItemEvalEvent{SpaceID: c.SpaceID, ExptID: c.ExptID, ExptRunID: c.RunID, EvalSetItemID: c.ItemID, RequestID: c.RequestID}}
	a, err := buildItemCompleteEventWithSource(worker, p)
	require.NoError(t, err)
	b, err := buildSourceEvent(p)
	require.NoError(t, err)
	require.Equal(t, a, b)
	for _, kind := range []string{"run", "item", "request", "space"} {
		t.Run(kind, func(t *testing.T) {
			copyCtx := *worker
			copyEvent := *worker.Event
			copyCtx.Event = &copyEvent
			switch kind {
			case "run":
				copyEvent.ExptRunID++
			case "item":
				copyEvent.EvalSetItemID++
			case "request":
				copyEvent.RequestID = "different-uuid"
			case "space":
				copyEvent.SpaceID++
			}
			ev, err := buildItemCompleteEventWithSource(&copyCtx, p)
			require.Error(t, err)
			require.Nil(t, ev)
		})
	}
}

func TestItemCompleteSourceAssemblerRejectsBrokenSource(t *testing.T) {
	for _, kind := range []string{"duplicate_turn", "negative_turn", "negative_target", "nil_log", "nil_ref", "duplicate_registered", "duplicate_inline", "missing_ref", "wrong_record", "foreign_ref", "foreign_turn", "bad_version", "empty_inline", "nil_registered", "nil_inline", "negative_record", "wrong_target", "unknown_ref_type", "ref_inline_version", "legacy_duplicate", "missing_item", "wrong_item_scope"} {
		t.Run(kind, func(t *testing.T) {
			p := itemCompleteSourcePublication()
			b := itemCompleteSourceBase
			s := p.Source.Source
			switch kind {
			case "duplicate_turn":
				p.Turns[0] = p.Turns[1]
			case "negative_turn":
				p.Turns[0].TurnID = -1
			case "negative_target":
				p.Turns[0].TargetResultID = -1
			case "nil_log":
				s.Turns[0] = nil
			case "nil_ref":
				p.Refs[0] = nil
			case "duplicate_registered":
				s.Turns[0].EvaluatorResultIds.Registered = append(s.Turns[0].EvaluatorResultIds.Registered, s.Turns[0].EvaluatorResultIds.Registered[0])
			case "duplicate_inline":
				s.Turns[0].EvaluatorResultIds.Inline = append(s.Turns[0].EvaluatorResultIds.Inline, s.Turns[0].EvaluatorResultIds.Inline[0])
			case "missing_ref":
				p.Refs = p.Refs[1:]
			case "wrong_record":
				p.Refs[0].EvaluatorResultID++
			case "foreign_ref":
				p.Refs[0].ExptTurnResultID++
			case "foreign_turn":
				p.Turns[0].ItemID++
			case "bad_version":
				s.Turns[0].EvaluatorResultIds.Registered[0].VersionID = 0
			case "empty_inline":
				s.Turns[0].EvaluatorResultIds.Inline[0].InlineKey = ""
			case "nil_registered":
				s.Turns[0].EvaluatorResultIds.Registered[0] = nil
			case "nil_inline":
				s.Turns[0].EvaluatorResultIds.Inline[0] = nil
			case "negative_record":
				p.Refs[0].EvaluatorResultID = -1
			case "wrong_target":
				s.Turns[0].TargetResultID++
			case "unknown_ref_type":
				p.Refs[0].SourceType = 99
			case "ref_inline_version":
				p.Refs[2].EvaluatorVersionID = b + 30
			case "legacy_duplicate":
				s.Turns[0].EvaluatorResultIds.Registered[0].Alias = ""
				s.Turns[0].EvaluatorResultIds.EvalVerIDToResID = map[int64]int64{b + 30: b + 40}
			case "missing_item":
				p.Source.Item = nil
			case "wrong_item_scope":
				p.Source.Item.ItemID++
			}
			ev, err := buildSourceEvent(p)
			require.Error(t, err, "damaged source must not silently publish an incomplete or different tuple")
			require.Nil(t, ev)
		})
	}
}

type serializedItemCompletePublisher struct {
	bodies [][]byte
	events []*component.ItemCompleteEvent
	err    error
}

func (p *serializedItemCompletePublisher) PublishItemComplete(ctx context.Context, event *component.ItemCompleteEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	body, err := runtimejson.Marshal(event)
	if err != nil {
		return err
	}
	p.bodies = append(p.bodies, body)
	p.events = append(p.events, event)
	return p.err
}

func TestItemCompleteSourceSchedulerRealSQLRetryPayload(t *testing.T) {
	f, svc, records, expt := publicationFixture(t)
	request := "f059e038-4d6e-41c8-a20d-68f8801adeaf"
	f.token(request)
	ctrl := gomock.NewController(t)
	expt.TargetID = f.base + 90
	expt.Target = &entity.EvalTarget{SourceTargetID: "business-object", EvalTargetVersion: &entity.EvalTargetVersion{SandboxAgent: &entity.SandboxAgent{EnableAnalysis: true}}}
	expt.EvalSet = &entity.EvaluationSet{ID: f.base + 70, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: f.base + 71}}
	records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).Return([]*entity.EvaluatorRecord{publicationRecord(f, gptr.Of(0.0))}, nil).AnyTimes()
	dataset := svcmocks.NewMockEvaluationSetItemService(ctrl)
	dataset.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{{ItemID: f.item, EvaluationSetID: f.base + 70, ItemKey: "item-key"}}, nil).AnyTimes()
	notify := &serializedItemCompletePublisher{}
	mode := entitymocks.NewMockExptSchedulerMode(ctrl)
	mode.EXPECT().PublishResult(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
	events := eventmocks.NewMockExptEventPublisher(ctrl)
	events.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
	scheduler := &ExptSchedulerImpl{ResultSvc: &publicationSchedulerResult{svc}, ExptItemResultRepo: f.itemRepo(), itemCompletePublisher: notify, evaluationSetItemService: dataset, Publisher: events}
	source, err := svc.GetItemResultPublishSource(context.Background(), f.space, f.expt, f.run, f.item)
	require.NoError(t, err)
	event := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run}
	item := &entity.ExptEvalItem{ItemID: f.item, State: entity.ItemRunState_Success, RequestID: request, Publication: source, PublicationObserved: true}
	before := f.snapshot()
	fault := errors.New("item-complete commit fault")
	require.NoError(t, f.conn.Callback().Update().Before("gorm:update").Register("source_payload_commit_fault", func(tx *gorm.DB) {
		if tx.Statement.Table == "expt_item_result_run_log" {
			_ = tx.AddError(fault)
		}
	}))
	err = scheduler.recordEvalItemRunLogs(context.Background(), event, []*entity.ExptEvalItem{item}, mode, expt)
	require.NoError(t, f.conn.Callback().Update().Remove("source_payload_commit_fault"))
	require.ErrorIs(t, err, fault)
	require.Equal(t, before, f.snapshot(), "send succeeds but the entire publication transaction rolls back")
	require.NoError(t, scheduler.recordEvalItemRunLogs(context.Background(), event, []*entity.ExptEvalItem{item}, mode, expt))
	require.Len(t, notify.bodies, 2)
	require.Equal(t, notify.bodies[0], notify.bodies[1], "new projection ref row IDs must not change a repeated source payload")
	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(notify.bodies[0], &payload))
	require.JSONEq(t, strconv.Quote(request), string(payload["request_id"]))
	turns := `[{"turn_id":"0","target_record_id":"` + strconv.FormatInt(f.base+41, 10) + `","registered":[{"version_id":"` + strconv.FormatInt(f.base+51, 10) + `","alias":"alias","record_id":"` + strconv.FormatInt(f.base+61, 10) + `"}]}]`
	require.JSONEq(t, turns, string(payload["source_turn_records"]))
	require.JSONEq(t, strconv.Quote(strconv.FormatInt(f.run, 10)), string(payload["expt_run_id"]))
	require.JSONEq(t, `"business-object"`, string(payload["source_target_id"]))
	require.JSONEq(t, `true`, string(payload["enable_analysis"]))
	after := f.snapshot()
	require.NoError(t, scheduler.recordEvalItemRunLogs(context.Background(), event, []*entity.ExptEvalItem{item}, mode, expt))
	require.Len(t, notify.bodies, 2, "Resulted duplicate must not send")
	require.Equal(t, after, f.snapshot())
}

func TestItemCompleteSourceSchedulerFrozenRecordsAndNewRequest(t *testing.T) {
	f, svc, records, expt := publicationFixture(t)
	firstRequest := "f059e038-4d6e-41c8-a20d-68f8801adeaf"
	secondRequest := "79bcdd8f-b312-477c-9803-5072d82e0661"
	f.token(firstRequest)
	refs := &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: f.base + 51, Alias: "alias", RecordID: f.base + 61}, {VersionID: f.base + 51, Alias: "second", RecordID: f.base + 62}}, Inline: []*entity.InlineEvalResult{{InlineKey: "inline", RecordID: f.base + 63}}}
	raw, err := json.Marshal(refs)
	require.NoError(t, err)
	require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("evaluator_result_ids", raw).Error)
	ctrl := gomock.NewController(t)
	expt.EvalSet = &entity.EvaluationSet{ID: f.base + 70, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: f.base + 71}}
	dataset := svcmocks.NewMockEvaluationSetItemService(ctrl)
	dataset.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{{ItemID: f.item, EvaluationSetID: f.base + 70}}, nil).AnyTimes()
	first := publicationRecord(f, gptr.Of(0.0))
	first.ExperimentRunID = f.run - 1 // successful prior-stage records are reused by explicit ID
	second := *first
	second.ID, second.Alias = f.base+62, "second"
	inline := *first
	inline.ID, inline.SourceType, inline.EvaluatorVersionID, inline.Alias, inline.InlineKey = f.base+63, entity.EvaluatorRecordSourceTypeInline, 0, "", "inline"
	records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{f.base + 61, f.base + 62, f.base + 63}, false, false).Return([]*entity.EvaluatorRecord{first, &second, &inline}, nil).AnyTimes()
	notify := &serializedItemCompletePublisher{}
	mode := entitymocks.NewMockExptSchedulerMode(ctrl)
	mode.EXPECT().PublishResult(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
	events := eventmocks.NewMockExptEventPublisher(ctrl)
	events.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
	scheduler := &ExptSchedulerImpl{ResultSvc: &publicationSchedulerResult{svc}, ExptItemResultRepo: f.itemRepo(), itemCompletePublisher: notify, evaluationSetItemService: dataset, Publisher: events}
	event := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run}
	source, err := svc.GetItemResultPublishSource(context.Background(), f.space, f.expt, f.run, f.item)
	require.NoError(t, err)
	item := &entity.ExptEvalItem{ItemID: f.item, State: entity.ItemRunState_Success, RequestID: firstRequest, Publication: source, PublicationObserved: true}
	require.NoError(t, scheduler.recordEvalItemRunLogs(context.Background(), event, []*entity.ExptEvalItem{item}, mode, expt))
	require.Len(t, notify.bodies, 1)
	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(notify.bodies[0], &payload))
	require.JSONEq(t, `[{"turn_id":"0","target_record_id":"`+strconv.FormatInt(f.base+41, 10)+`","registered":[{"version_id":"`+strconv.FormatInt(f.base+51, 10)+`","alias":"alias","record_id":"`+strconv.FormatInt(f.base+61, 10)+`"},{"version_id":"`+strconv.FormatInt(f.base+51, 10)+`","alias":"second","record_id":"`+strconv.FormatInt(f.base+62, 10)+`"}],"inline":[{"inline_key":"inline","record_id":"`+strconv.FormatInt(f.base+63, 10)+`"}]}]`, string(payload["source_turn_records"]))
	// The next admitted identity may reuse the same successful records; its UUID still differs.
	f.token(secondRequest)
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("result_state", int32(entity.ExptItemResultStateLogged)).Error)
	before := f.snapshot()
	require.NoError(t, scheduler.recordEvalItemRunLogs(context.Background(), event, []*entity.ExptEvalItem{item}, mode, expt))
	require.Len(t, notify.bodies, 1, "old preparation cannot send for the next request")
	require.Equal(t, before, f.snapshot())
	newSource, err := svc.GetItemResultPublishSource(context.Background(), f.space, f.expt, f.run, f.item)
	require.NoError(t, err)
	newItem := &entity.ExptEvalItem{ItemID: f.item, State: entity.ItemRunState_Success, RequestID: secondRequest, Publication: newSource, PublicationObserved: true}
	require.NoError(t, scheduler.recordEvalItemRunLogs(context.Background(), event, []*entity.ExptEvalItem{newItem}, mode, expt))
	require.Len(t, notify.bodies, 2)
	var next map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(notify.bodies[1], &next))
	require.Equal(t, payload["source_turn_records"], next["source_turn_records"])
	require.NotEqual(t, payload["request_id"], next["request_id"])
	require.Equal(t, strconv.Quote(secondRequest), string(next["request_id"]))
	require.Equal(t, firstRequest, *notify.events[0].RequestID)
	var committed []model.ExptTurnEvaluatorResultRef
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=?", f.space, f.expt).Order("evaluator_result_id").Find(&committed).Error)
	require.Len(t, committed, 3)
	require.Equal(t, []int64{f.base + 61, f.base + 62, f.base + 63}, []int64{committed[0].EvaluatorResultID, committed[1].EvaluatorResultID, committed[2].EvaluatorResultID})
}

func TestItemCompleteSourceSchedulerCancelAndBadScopeNoEffects(t *testing.T) {
	for _, kind := range []string{"cancelled", "lost_lease", "wrong_run", "source_changed", "send_failure"} {
		t.Run(kind, func(t *testing.T) {
			f, svc, records, expt := publicationFixture(t)
			ctrl := gomock.NewController(t)
			expt.EvalSet = &entity.EvaluationSet{ID: f.base + 70, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: f.base + 71}}
			dataset := svcmocks.NewMockEvaluationSetItemService(ctrl)
			dataset.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{{ItemID: f.item, EvaluationSetID: f.base + 70}}, nil).AnyTimes()
			records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).Return([]*entity.EvaluatorRecord{publicationRecord(f, nil)}, nil).AnyTimes()
			notify := &serializedItemCompletePublisher{}
			scheduler := &ExptSchedulerImpl{ResultSvc: &publicationSchedulerResult{svc}, ExptItemResultRepo: f.itemRepo(), itemCompletePublisher: notify, evaluationSetItemService: dataset}
			source, err := svc.GetItemResultPublishSource(context.Background(), f.space, f.expt, f.run, f.item)
			require.NoError(t, err)
			item := &entity.ExptEvalItem{ItemID: f.item, State: entity.ItemRunState_Success, RequestID: "rq1", Publication: source, PublicationObserved: true}
			event := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run}
			ctx := context.Background()
			if kind == "cancelled" {
				child, cancel := context.WithCancel(ctx)
				cancel()
				ctx = child
			} else if kind == "lost_lease" {
				locker := lockmocks.NewMockILocker(ctrl)
				locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ string, _ time.Duration, _ time.Duration) (bool, context.Context, func(), error) {
					child, cancel := context.WithCancel(ctx)
					cancel()
					return true, child, cancel, nil
				})
				locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
				svc.resultPublishLocker = func() lock.ILocker { return locker }
			} else if kind == "wrong_run" {
				event.ExptRunID++
			} else if kind == "source_changed" {
				f.token("79bcdd8f-b312-477c-9803-5072d82e0661")
			} else {
				notify.err = errors.New("MQ unavailable")
			}
			before := f.snapshot()
			err = scheduler.recordEvalItemRunLogs(ctx, event, []*entity.ExptEvalItem{item}, entitymocks.NewMockExptSchedulerMode(ctrl), expt)
			if kind == "source_changed" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			if kind == "send_failure" {
				require.Len(t, notify.bodies, 1)
			} else {
				require.Empty(t, notify.bodies)
			}
			require.Equal(t, before, f.snapshot(), "failed, cancelled or stale publication must leave all eleven tables unchanged")
		})
	}
}

func TestItemCompleteSourceSchedulerMultipleItemsRawAndManual(t *testing.T) {
	f, svc, records, expt := publicationFixture(t)
	request := "f059e038-4d6e-41c8-a20d-68f8801adeaf"
	f.token(request)
	require.NoError(t, f.conn.Create(&model.ExptTurnResult{ID: f.base + 410, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: f.item + 1, ItemVersionID: f.version, TurnID: 0, Status: int32(entity.TurnRunState_Processing)}).Error)
	require.NoError(t, f.conn.Create(&model.ExptTurnResultRunLog{ID: f.base + 510, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: f.item + 1, ItemVersionID: f.version, TurnID: 0, Status: int32(entity.TurnRunState_Success)}).Error)
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item+1).Updates(map[string]any{"status": int32(entity.ItemRunState_Success), "result_state": int32(entity.ExptItemResultStateLogged)}).Error)
	ctrl := gomock.NewController(t)
	expt.EvalSet = &entity.EvaluationSet{ID: f.base + 70, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: f.base + 71}}
	dataset := svcmocks.NewMockEvaluationSetItemService(ctrl)
	dataset.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{{ItemID: f.item, EvaluationSetID: f.base + 70}, {ItemID: f.item + 1, EvaluationSetID: f.base + 70}}, nil).Times(1)
	records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{f.base + 61}, false, false).Return([]*entity.EvaluatorRecord{publicationRecord(f, nil)}, nil).Times(1)
	notify := &serializedItemCompletePublisher{}
	mode := entitymocks.NewMockExptSchedulerMode(ctrl)
	mode.EXPECT().PublishResult(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
	events := eventmocks.NewMockExptEventPublisher(ctrl)
	events.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
	scheduler := &ExptSchedulerImpl{ResultSvc: &publicationSchedulerResult{svc}, ExptItemResultRepo: f.itemRepo(), itemCompletePublisher: notify, evaluationSetItemService: dataset, Publisher: events}
	items := []*entity.ExptEvalItem{}
	for _, id := range []int64{f.item, f.item + 1} {
		source, err := svc.GetItemResultPublishSource(context.Background(), f.space, f.expt, f.run, id)
		require.NoError(t, err)
		items = append(items, &entity.ExptEvalItem{ItemID: id, State: entity.ItemRunState_Success, RequestID: source.Source.Current.RequestID, Publication: source, PublicationObserved: true})
	}
	require.NoError(t, scheduler.recordEvalItemRunLogs(context.Background(), &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run}, items, mode, expt))
	require.Len(t, notify.bodies, 2)
	require.Equal(t, request, *notify.events[0].RequestID)
	require.Equal(t, "", *notify.events[1].RequestID)
	require.Equal(t, "0", notify.events[1].SourceTurnRecords[0].TargetRecordID)
	require.Empty(t, notify.events[1].SourceTurnRecords[0].Registered)
	*notify.events[1].RequestID = "different-event"
	notify.events[1].SourceTurnRecords[0].TargetRecordID = "99"
	first, err := runtimejson.Marshal(notify.events[0])
	require.NoError(t, err)
	require.Equal(t, notify.bodies[0], first, "subsequent items cannot mutate an earlier event source")
	stats, err := svc.ExptStatsRepo.Get(context.Background(), f.expt, f.space)
	require.NoError(t, err)
	require.Equal(t, int32(2), stats.SuccessItemCnt)
	require.Zero(t, stats.ProcessingItemCnt)
	require.Equal(t, int32(1), stats.PendingItemCnt)
}
