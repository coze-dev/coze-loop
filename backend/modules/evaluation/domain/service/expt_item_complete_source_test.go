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

func TestItemCompleteSchedulerRealSQLRetryPayload(t *testing.T) {
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
	require.Equal(t, notify.bodies[0], notify.bodies[1], "a rolled-back publication must resend the original notification unchanged")
	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(notify.bodies[0], &payload))
	require.NotContains(t, payload, "request_id", "internal retry request must not change the original notification")
	require.NotContains(t, payload, "source_turn_records", "item-complete must keep its original payload")
	require.JSONEq(t, strconv.Quote(strconv.FormatInt(f.run, 10)), string(payload["expt_run_id"]))
	require.JSONEq(t, `"business-object"`, string(payload["source_target_id"]))
	require.JSONEq(t, `true`, string(payload["enable_analysis"]))
	after := f.snapshot()
	require.NoError(t, scheduler.recordEvalItemRunLogs(context.Background(), event, []*entity.ExptEvalItem{item}, mode, expt))
	require.Len(t, notify.bodies, 2, "Resulted replay must not republish the committed projection")
	require.Equal(t, after, f.snapshot())
}

func TestItemCompleteSchedulerFrozenRecordsAndNewRequest(t *testing.T) {
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
	require.NotContains(t, payload, "request_id")
	require.NotContains(t, payload, "source_turn_records")
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
	require.NotContains(t, next, "request_id")
	require.NotContains(t, next, "source_turn_records")
	require.Equal(t, notify.bodies[0], notify.bodies[1], "new internal requests still send the same original consumer identity")
	var committed []model.ExptTurnEvaluatorResultRef
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=?", f.space, f.expt).Order("evaluator_result_id").Find(&committed).Error)
	require.Len(t, committed, 3)
	require.Equal(t, []int64{f.base + 61, f.base + 62, f.base + 63}, []int64{committed[0].EvaluatorResultID, committed[1].EvaluatorResultID, committed[2].EvaluatorResultID})
}

func TestItemCompleteSchedulerCancelAndBadScopeNoEffects(t *testing.T) {
	for _, kind := range []string{"cancelled", "lost_lease", "wrong_run", "wrong_space", "wrong_experiment", "wrong_item", "wrong_request", "source_changed", "send_failure"} {
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
			switch kind {
			case "cancelled":
				child, cancel := context.WithCancel(ctx)
				cancel()
				ctx = child
			case "lost_lease":
				locker := lockmocks.NewMockILocker(ctrl)
				locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ string, _ time.Duration, _ time.Duration) (bool, context.Context, func(), error) {
					child, cancel := context.WithCancel(ctx)
					cancel()
					return true, child, cancel, nil
				})
				locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
				svc.resultPublishLocker = func() lock.ILocker { return locker }
			case "wrong_run":
				event.ExptRunID++
			case "wrong_space":
				event.SpaceID++
			case "wrong_experiment":
				event.ExptID++
			case "wrong_item":
				item.ItemID++
			case "wrong_request":
				item.RequestID = "another-request"
			case "source_changed":
				f.token("79bcdd8f-b312-477c-9803-5072d82e0661")
			default:
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

func TestItemCompleteSchedulerMultipleItemsRawAndManual(t *testing.T) {
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
	for _, body := range notify.bodies {
		var payload map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(body, &payload))
		require.NotContains(t, payload, "request_id")
		require.NotContains(t, payload, "source_turn_records")
	}
	require.Equal(t, strconv.FormatInt(f.item, 10), notify.events[0].ItemID)
	require.Equal(t, strconv.FormatInt(f.item+1, 10), notify.events[1].ItemID)
	notify.events[1].ItemKey = "changed-item"
	first, err := runtimejson.Marshal(notify.events[0])
	require.NoError(t, err)
	require.Equal(t, notify.bodies[0], first, "subsequent items cannot mutate an earlier notification")
	stats, err := svc.ExptStatsRepo.Get(context.Background(), f.expt, f.space)
	require.NoError(t, err)
	require.Equal(t, int32(2), stats.SuccessItemCnt)
	require.Zero(t, stats.ProcessingItemCnt)
	require.Equal(t, int32(1), stats.PendingItemCnt)
}

// The existing sender may deliver the same notification repeatedly. Its consumer
// identity remains independent of internal manual retry and result source data.
func TestItemCompleteSchedulerDuplicateSendOriginalPayload(t *testing.T) {
	const expected = `{"eval_target_workspace_id":"","eval_target_id":"","source_target_id":"","expt_workspace_id":"7384603622000000001","expt_id":"7384603622000000002","expt_run_id":"7384603622000000003","dataset_workspace_id":"7384603622000000001","dataset_id":"7384603622000000005","dataset_version_id":"7384603622000000006","dataset_key":"","dataset_version_name":"","experiment_group_key":"","item_id":"7384603622000000004","item_key":"case","enable_analysis":false,"created_by":""}`
	for _, request := range []string{"", "manual-retry-request"} {
		t.Run("request="+request, func(t *testing.T) {
			notify := &serializedItemCompletePublisher{}
			scheduler := &ExptSchedulerImpl{itemCompletePublisher: notify}
			event := &entity.ExptScheduleEvent{SpaceID: 7384603622000000001, ExptID: 7384603622000000002, ExptRunID: 7384603622000000003}
			item := &entity.ExptEvalItem{
				ItemID: 7384603622000000004, State: entity.ItemRunState_Success, RequestID: request,
				Publication: &entity.ExptItemResultPublishSource{}, PublicationObserved: true,
			}
			meta := &entity.EvaluationSetItem{SpaceID: event.SpaceID, EvaluationSetID: 7384603622000000005, ItemID: item.ItemID, ItemKey: "case"}
			for delivery := 0; delivery < 2; delivery++ {
				require.NoError(t, scheduler.sendItemComplete(context.Background(), event, nil, item, meta, 7384603622000000006))
			}
			require.Len(t, notify.bodies, 2)
			require.Equal(t, expected, string(notify.bodies[0]))
			require.Equal(t, notify.bodies[0], notify.bodies[1])
			worker := buildItemCompleteEvent(&entity.ExptItemEvalCtx{Event: &entity.ExptItemEvalEvent{SpaceID: event.SpaceID, ExptID: event.ExptID, ExptRunID: event.ExptRunID, EvalSetItemID: item.ItemID, RequestID: request}, EvalSetItem: meta, EvalSetVersionID: 7384603622000000006})
			body, err := runtimejson.Marshal(worker)
			require.NoError(t, err)
			require.Equal(t, notify.bodies[0], body, "worker and scheduler retain the same original payload")
		})
	}
}
