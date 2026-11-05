// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	experimentrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	experimentmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// Only initialization is omitted. Scan, publication, dispatch, ExptEnd and
// NextTick execute their production code against native SQL and Redis CAS/TTL.
type localRecoveryTickMode struct{ *ExptSubmitExec }

func (*localRecoveryTickMode) ExptStart(context.Context, *entity.ExptScheduleEvent, *entity.Experiment) error {
	return nil
}

func (*localRecoveryTickMode) ScheduleStart(context.Context, *entity.ExptScheduleEvent, *entity.Experiment) error {
	return nil
}

func TestSchedulerLocalRecoveryPublishesAndDispatchesPastBlockedItem(t *testing.T) {
	for _, blocked := range []string{"busy", "damaged_processing", "damaged_logged"} {
		t.Run(blocked, func(t *testing.T) {
			f := newLateWriteFixture(t, "rq1")
			ctx := context.Background()
			ctrl := gomock.NewController(t)
			factory, server := finishRealLockerFactory(t)
			key := fmt.Sprintf("expt_item_eval_run_lock:%d:%d", f.expt, f.item)
			if blocked == "busy" {
				require.NoError(t, server.Set(key, "original-worker"))
				server.SetTTL(key, time.Minute)
			}
			// The second item has one complete, publishable ordinary legacy turn0.
			// Its persisted version zero must still normalize from the canonical turn.
			healthy := f.item + 1
			require.NoError(t, f.conn.Create(&model.ExptTurnResult{ID: f.base + 710, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: healthy, ItemVersionID: f.version, TurnID: 0, Status: int32(entity.TurnRunState_Processing)}).Error)
			require.NoError(t, f.conn.Create(&model.ExptTurnResultRunLog{ID: f.base + 711, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: healthy, ItemVersionID: 0, TurnID: 0, Status: int32(entity.TurnRunState_Success)}).Error)
			require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, healthy).Updates(map[string]any{"status": int32(entity.ItemRunState_Success), "result_state": int32(entity.ExptItemResultStateLogged)}).Error)
			if blocked != "damaged_processing" {
				require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Updates(map[string]any{"status": int32(entity.ItemRunState_Success), "result_state": int32(entity.ExptItemResultStateLogged)}).Error)
				require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Updates(map[string]any{"status": int32(entity.TurnRunState_Success), "target_result_id": 0, "evaluator_result_ids": nil}).Error)
			}
			if blocked != "busy" {
				require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND item_id=? AND turn_id=0", f.space, f.item).Update("item_version_id", f.version+1).Error)
			}
			var originalLog model.ExptItemResultRunLog
			require.NoError(t, f.conn.Where("space_id=? AND item_id=?", f.space, f.item).Take(&originalLog).Error)
			expt := &entity.Experiment{ID: f.expt, SpaceID: f.space, LatestRunID: f.run, Status: entity.ExptStatus_Processing, ExptType: entity.ExptType_Offline, EvalConf: &entity.EvaluationConfiguration{}, EvalSet: &entity.EvaluationSet{EvaluationSetVersion: &entity.EvaluationSetVersion{ID: f.base + 70}}}
			manager := svcmocks.NewMockIExptManager(ctrl)
			manager.EXPECT().GetDetail(gomock.Any(), f.expt, f.space, gomock.Any()).Return(expt, nil).AnyTimes()
			config := componentmocks.NewMockIConfiger(ctrl)
			config.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{}).AnyTimes()
			config.EXPECT().GetExptExecConf(gomock.Any(), f.space).Return(&entity.ExptExecConf{DaemonIntervalSecond: 1, ExptItemEvalConf: &entity.ExptItemEvalConf{ConcurNum: 3, MaxItemConcurNum: 3}}).AnyTimes()
			publisher := eventmocks.NewMockExptEventPublisher(ctrl)
			var delivered []*entity.ExptItemEvalEvent
			publisher.EXPECT().BatchPublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, events []*entity.ExptItemEvalEvent, _ *time.Duration) error {
				for _, event := range events {
					// Keep the actual event fields across the production JSON contract.
					raw, err := json.Marshal(event)
					require.NoError(t, err)
					var decoded entity.ExptItemEvalEvent
					require.NoError(t, json.Unmarshal(raw, &decoded))
					delivered = append(delivered, &decoded)
				}
				return nil
			})
			publisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			ticks := 0
			publisher.EXPECT().PublishExptScheduleEvent(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, event *entity.ExptScheduleEvent, _ *time.Duration) error {
				require.Equal(t, f.run, event.ExptRunID)
				ticks++
				return nil
			}).AnyTimes()
			svc := &publicationSchedulerResult{ExptResultServiceImpl: &ExptResultServiceImpl{ExptItemResultRepo: f.itemRepo(), ExptTurnResultRepo: f.turnRepo(), ExptStatsRepo: experimentrepo.NewExptStatsRepo(experimentmysql.NewExptStatsDAO(f.provider)), resultPublishRepo: experimentrepo.NewExptResultPublishRepo(f.provider), resultPublishLocker: factory}}
			mode := &localRecoveryTickMode{ExptSubmitExec: &ExptSubmitExec{manager: manager, configer: config, exptItemResultRepo: f.itemRepo(), publisher: publisher, resultSvc: svc}}
			modeFactory := svcmocks.NewMockSchedulerModeFactory(ctrl)
			modeFactory.EXPECT().NewSchedulerMode(entity.EvaluationModeSubmit).Return(mode, nil).AnyTimes()
			scheduler := &ExptSchedulerImpl{Manager: manager, Configer: config, Publisher: publisher, ExptItemResultRepo: f.itemRepo(), ResultSvc: svc, itemLocker: factory, schedulerModeFactory: modeFactory}
			event := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ExptType: entity.ExptType_Offline, ExptRunMode: entity.EvaluationModeSubmit}
			require.NoError(t, scheduler.schedule(ctx, event))
			require.Equal(t, 1, ticks, "blocked item must retain the normal tick driver")
			require.Len(t, delivered, 1)
			require.Equal(t, f.item+2, delivered[0].EvalSetItemID)
			require.Empty(t, delivered[0].RequestID)
			var after model.ExptItemResultRunLog
			require.NoError(t, f.conn.Where("space_id=? AND item_id=?", f.space, f.item).Take(&after).Error)
			require.Equal(t, originalLog, after, "busy/damaged original request remains untouched")
			after = model.ExptItemResultRunLog{}
			require.NoError(t, f.conn.Where("space_id=? AND item_id=?", f.space, healthy).Take(&after).Error)
			require.Equal(t, int32(entity.ExptItemResultStateResulted), gptr.Indirect(after.ResultState), "healthy Logged item must publish in the same tick")
			finish, err := experimentrepo.NewExptRunFinishRepo(f.provider).FinishCurrentRun(ctx, &entity.ExptRunFinishRequest{SpaceID: f.space, ExptID: f.expt, RunID: f.run})
			require.NoError(t, err)
			require.Equal(t, entity.ExptRunFinishPending, finish.Disposition)
			if blocked == "busy" {
				holder, err := server.Get(key)
				require.NoError(t, err)
				require.Equal(t, "original-worker", holder, "publication cannot unlock another request's holder")
				server.Del(key)
				rows, err := f.itemRepo().ScanCurrentRequests(ctx, f.space, f.expt, f.run, false)
				require.NoError(t, err)
				var ready []*entity.ExptEvalItem
				for _, row := range rows {
					if row.ItemID == f.item {
						ready = append(ready, &entity.ExptEvalItem{ItemID: row.ItemID, State: row.Status, RequestID: row.RequestID, Publication: row.Publication, PublicationObserved: row.PublicationObserved})
					}
				}
				require.NoError(t, scheduler.recordEvalItemRunLogs(ctx, event, ready, mode, expt))
				after = model.ExptItemResultRunLog{}
				require.NoError(t, f.conn.Where("space_id=? AND item_id=?", f.space, f.item).Take(&after).Error)
				require.Equal(t, int32(entity.ExptItemResultStateResulted), gptr.Indirect(after.ResultState), "the next observation resumes the previously busy request")
			}
		})
	}
}
