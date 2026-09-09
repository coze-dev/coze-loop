// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func TestSubmitSelectedItem_PublishRecovery(t *testing.T) {
	for _, scenario := range []string{"duplicate submit", "publish failure", "cancelled publish", "terminated during failed publish", "new owner during failed publish", "completed during failed publish", "new attempt during failed publish", "partial claim failure"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			provider, err := db.NewDB(sqlite.Open(":memory:"), &gorm.Config{})
			require.NoError(t, err)
			conn := provider.NewSession(context.Background())
			sqlDB, err := conn.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
			for _, statement := range []string{
				`CREATE TABLE expt_item_result (id INTEGER PRIMARY KEY,space_id INTEGER,expt_id INTEGER,expt_run_id INTEGER,item_id INTEGER,status INTEGER,updated_at DATETIME,deleted_at DATETIME)`,
				`CREATE TABLE expt_item_result_run_log (id INTEGER PRIMARY KEY,space_id INTEGER,expt_id INTEGER,expt_run_id INTEGER,item_id INTEGER,status INTEGER,result_state INTEGER,retry_times INTEGER,updated_at DATETIME,deleted_at DATETIME)`,
				`CREATE TABLE expt_turn_result (id INTEGER PRIMARY KEY,space_id INTEGER,expt_id INTEGER,expt_run_id INTEGER,item_id INTEGER,status INTEGER,target_result_id INTEGER,weighted_score REAL,updated_at DATETIME,deleted_at DATETIME)`,
				`CREATE TABLE expt_stats (id INTEGER PRIMARY KEY,space_id INTEGER,expt_id INTEGER,pending_cnt INTEGER,processing_cnt INTEGER,updated_at DATETIME,deleted_at DATETIME)`,
				`INSERT INTO expt_item_result VALUES (11,3,1,100,7,0,NULL,NULL)`,
				`INSERT INTO expt_item_result_run_log VALUES (12,3,1,100,7,0,0,0,NULL,NULL)`,
				`INSERT INTO expt_turn_result VALUES (13,3,1,100,7,0,77,0.5,NULL,NULL)`,
				`INSERT INTO expt_stats VALUES (14,3,1,1,0,NULL,NULL)`,
			} {
				require.NoError(t, conn.Exec(statement).Error)
			}
			ctrl := gomock.NewController(t)
			configer := configmocks.NewMockIConfiger(ctrl)
			configer.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).AnyTimes().Return(&entity.ExptExecConf{})
			metric := metricsmocks.NewMockExptMetric(ctrl)
			metric.EXPECT().EmitItemExecEval(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			resultSvc := svcmocks.NewMockExptResultService(ctrl)
			resultSvc.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), int64(3), int64(1), []int64{7}).AnyTimes().Return(nil)
			publisher := eventmocks.NewMockExptEventPublisher(ctrl)
			publishCalls := 0
			publishErr := errors.New("publish unavailable")
			publisher.EXPECT().BatchPublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(_ context.Context, events []*entity.ExptItemEvalEvent, _ *time.Duration) error {
				publishCalls++
				require.Len(t, events, 1)
				assert.Equal(t, int64(7), events[0].EvalSetItemID)
				switch scenario {
				case "duplicate submit":
					return nil
				case "cancelled publish":
					cancel()
				case "terminated during failed publish":
					require.NoError(t, conn.Exec(`UPDATE expt_item_result_run_log SET status=5,result_state=2 WHERE id=12`).Error)
					require.NoError(t, conn.Exec(`UPDATE expt_turn_result SET status=5 WHERE id=13`).Error)
				case "new owner during failed publish":
					require.NoError(t, conn.Exec(`UPDATE expt_item_result SET expt_run_id=200 WHERE id=11`).Error)
				case "completed during failed publish":
					require.NoError(t, conn.Exec(`UPDATE expt_item_result_run_log SET status=2,result_state=2 WHERE id=12`).Error)
				case "new attempt during failed publish":
					require.NoError(t, conn.Exec(`UPDATE expt_item_result_run_log SET retry_times=1 WHERE id=12`).Error)
				}
				return publishErr
			})
			scheduler := &ExptSchedulerImpl{
				Configer: configer, Metric: metric, ResultSvc: resultSvc, Publisher: publisher,
				ExptItemResultRepo: exptrepo.NewExptItemResultRepo(exptmysql.NewExptItemResultDAO(provider)),
			}
			selected := []*entity.ExptEvalItem{{ItemID: 7, State: entity.ItemRunState_Queueing}}
			if scenario == "partial claim failure" {
				selected = append(selected, &entity.ExptEvalItem{ItemID: 99, State: entity.ItemRunState_Queueing})
			}
			event := &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 100}
			err = scheduler.handleToSubmits(ctx, event, selected)
			if scenario == "duplicate submit" {
				require.NoError(t, err)
				require.NoError(t, scheduler.handleToSubmits(ctx, event, selected))
				assert.Equal(t, 1, publishCalls)
			} else {
				require.Error(t, err)
			}
			var item model.ExptItemResult
			var run model.ExptItemResultRunLog
			var counts struct{ PendingCnt, ProcessingCnt int64 }
			require.NoError(t, conn.First(&item, 11).Error)
			require.NoError(t, conn.First(&run, 12).Error)
			require.NoError(t, conn.Table("expt_stats").Select("pending_cnt,processing_cnt").Scan(&counts).Error)
			if scenario == "publish failure" || scenario == "cancelled publish" || scenario == "partial claim failure" {
				assert.Equal(t, int32(entity.ItemRunState_Queueing), item.Status)
				assert.Equal(t, int32(entity.ItemRunState_Queueing), run.Status)
				assert.Equal(t, int64(1), counts.PendingCnt)
				assert.Equal(t, int64(0), counts.ProcessingCnt)
			} else {
				assert.Equal(t, int32(entity.ItemRunState_Processing), item.Status)
				assert.Equal(t, int64(0), counts.PendingCnt)
				assert.Equal(t, int64(1), counts.ProcessingCnt)
			}
			if scenario == "terminated during failed publish" {
				assert.Equal(t, int32(entity.ItemRunState_Terminal), run.Status)
			}
			if scenario == "new owner during failed publish" {
				assert.Equal(t, int64(200), item.ExptRunID)
			}
			if scenario == "completed during failed publish" {
				assert.Equal(t, int32(entity.ItemRunState_Success), run.Status)
			}
			if scenario == "new attempt during failed publish" {
				assert.Equal(t, int32(1), run.RetryTimes)
			}
		})
	}
}
