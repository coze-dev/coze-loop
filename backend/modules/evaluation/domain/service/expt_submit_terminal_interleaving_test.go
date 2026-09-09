// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
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

func TestSubmitSelectedItem_TerminalInterleaving(t *testing.T) {
	for _, tc := range []struct {
		name          string
		beforeSubmit  bool
		duringPublish bool
		resulted      bool
	}{
		{name: "queueing control"},
		{name: "terminated after selection", beforeSubmit: true},
		{name: "termination already projected", beforeSubmit: true, resulted: true},
		{name: "terminated during publish", duringPublish: true},
		{name: "termination projected during publish", duringPublish: true, resulted: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			provider, err := db.NewDB(sqlite.Open(":memory:"), &gorm.Config{})
			require.NoError(t, err)
			conn := provider.NewSession(ctx)
			sqlDB, err := conn.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
			for _, ddl := range []string{
				`CREATE TABLE expt_item_result (id INTEGER PRIMARY KEY, space_id INTEGER, expt_id INTEGER, expt_run_id INTEGER, item_id INTEGER, status INTEGER, err_msg BLOB, updated_at DATETIME, deleted_at DATETIME)`,
				`CREATE TABLE expt_item_result_run_log (id INTEGER PRIMARY KEY, space_id INTEGER, expt_id INTEGER, expt_run_id INTEGER, item_id INTEGER, status INTEGER, result_state INTEGER, retry_times INTEGER DEFAULT 0, err_msg BLOB, updated_at DATETIME, deleted_at DATETIME)`,
				`CREATE TABLE expt_turn_result (id INTEGER PRIMARY KEY, space_id INTEGER, expt_id INTEGER, expt_run_id INTEGER, item_id INTEGER, turn_id INTEGER, status INTEGER, target_result_id INTEGER, weighted_score REAL, updated_at DATETIME, deleted_at DATETIME)`,
				`CREATE TABLE expt_stats (id INTEGER PRIMARY KEY, space_id INTEGER, expt_id INTEGER, pending_cnt INTEGER, processing_cnt INTEGER, terminated_cnt INTEGER, success_cnt INTEGER, fail_cnt INTEGER, updated_at DATETIME, deleted_at DATETIME)`,
				`INSERT INTO expt_item_result VALUES (11,3,1,100,7,0,NULL,NULL,NULL)`,
				`INSERT INTO expt_item_result_run_log (id,space_id,expt_id,expt_run_id,item_id,status,result_state,retry_times) VALUES (12,3,1,100,7,0,0,0)`,
				`INSERT INTO expt_turn_result VALUES (13,3,1,100,7,8,0,77,0.5,NULL,NULL)`,
				`INSERT INTO expt_stats VALUES (14,3,1,1,0,0,0,0,NULL,NULL)`,
			} {
				require.NoError(t, conn.Exec(ddl).Error)
			}
			selected := []*entity.ExptEvalItem{{ItemID: 7, State: entity.ItemRunState_Queueing}}
			var terminatedItem model.ExptItemResult
			var terminatedCounts struct{ PendingCnt, ProcessingCnt, TerminatedCnt int64 }
			terminate := func() {
				// Commit the two observable stages of TerminateItems and scheduler projection, after selection.
				require.NoError(t, conn.Transaction(func(tx *gorm.DB) error {
					resultState := entity.ExptItemResultStateLogged
					if tc.resulted {
						resultState = entity.ExptItemResultStateResulted
					}
					if err := tx.Exec(`UPDATE expt_item_result_run_log SET status=?,result_state=?,err_msg=? WHERE id=12`, int32(entity.ItemRunState_Terminal), int32(resultState), []byte("manually terminated")).Error; err != nil {
						return err
					}
					if err := tx.Exec(`UPDATE expt_turn_result SET status=? WHERE id=13`, int32(entity.TurnRunState_Terminal)).Error; err != nil {
						return err
					}
					if tc.resulted {
						if err := tx.Exec(`UPDATE expt_item_result SET status=? WHERE id=11`, int32(entity.ItemRunState_Terminal)).Error; err != nil {
							return err
						}
						return tx.Exec(`UPDATE expt_stats SET pending_cnt=0,processing_cnt=0,terminated_cnt=1 WHERE id=14`).Error
					}
					return nil
				}))
				require.NoError(t, conn.First(&terminatedItem, 11).Error)
				require.NoError(t, conn.Table("expt_stats").Select("pending_cnt,processing_cnt,terminated_cnt").Scan(&terminatedCounts).Error)
			}
			if tc.beforeSubmit {
				terminate()
			}
			ctrl := gomock.NewController(t)
			configer := configmocks.NewMockIConfiger(ctrl)
			configer.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).AnyTimes().Return(&entity.ExptExecConf{})
			publisher := eventmocks.NewMockExptEventPublisher(ctrl)
			publishCalls := 0
			publisher.EXPECT().BatchPublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().
				DoAndReturn(func(_ context.Context, _ []*entity.ExptItemEvalEvent, _ *time.Duration) error {
					publishCalls++
					if tc.duringPublish {
						terminate()
					}
					return nil
				})
			metric := metricsmocks.NewMockExptMetric(ctrl)
			metric.EXPECT().EmitItemExecEval(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			resultSvc := svcmocks.NewMockExptResultService(ctrl)
			resultSvc.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), int64(3), int64(1), gomock.Any()).AnyTimes().Return(nil)
			scheduler := &ExptSchedulerImpl{
				Configer: configer, Publisher: publisher, Metric: metric, ResultSvc: resultSvc,
				ExptItemResultRepo: exptrepo.NewExptItemResultRepo(exptmysql.NewExptItemResultDAO(provider)),
				ExptTurnResultRepo: exptrepo.NewExptTurnResultRepo(nil, exptmysql.NewExptTurnResultDAO(provider), nil),
				ExptStatsRepo:      exptrepo.NewExptStatsRepo(exptmysql.NewExptStatsDAO(provider)),
			}
			require.NoError(t, scheduler.handleToSubmits(ctx, &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 100}, selected))
			var item model.ExptItemResult
			var runLog model.ExptItemResultRunLog
			var turn model.ExptTurnResult
			var counts struct{ PendingCnt, ProcessingCnt, TerminatedCnt int64 }
			require.NoError(t, conn.First(&item, 11).Error)
			require.NoError(t, conn.First(&runLog, 12).Error)
			require.NoError(t, conn.First(&turn, 13).Error)
			require.NoError(t, conn.Table("expt_stats").Select("pending_cnt,processing_cnt,terminated_cnt").Scan(&counts).Error)
			assert.Equal(t, int64(77), turn.TargetResultID)
			assert.Equal(t, gptr.Of(0.5), turn.WeightedScore)
			if !tc.beforeSubmit && !tc.duringPublish {
				assert.Equal(t, 1, publishCalls)
				assert.Equal(t, int32(entity.ItemRunState_Processing), runLog.Status)
				assert.Equal(t, int32(entity.ItemRunState_Processing), item.Status)
				assert.Equal(t, int64(0), counts.PendingCnt)
				assert.Equal(t, int64(1), counts.ProcessingCnt)
				return
			}
			if tc.duringPublish {
				assert.Equal(t, 1, publishCalls, "the termination must execute inside the real publish boundary")
			}
			assert.Equal(t, int32(entity.ItemRunState_Terminal), runLog.Status, "selected item must not revive after termination")
			assert.Equal(t, int32(entity.TurnRunState_Terminal), turn.Status)
			assert.Equal(t, []byte("manually terminated"), gptr.Indirect(runLog.ErrMsg))
			assert.Equal(t, terminatedCounts, counts, "termination cannot create an additional processing charge")
			assert.Equal(t, terminatedItem.Status, item.Status, "late submit cannot change the projection observed at termination")
			if tc.resulted {
				assert.Equal(t, int32(entity.ItemRunState_Terminal), item.Status)
				assert.Equal(t, int32(entity.ExptItemResultStateResulted), gptr.Indirect(runLog.ResultState))
				assert.Equal(t, int64(0), counts.PendingCnt)
				assert.Equal(t, int64(1), counts.TerminatedCnt)
			} else {
				assert.Equal(t, int32(entity.ExptItemResultStateLogged), gptr.Indirect(runLog.ResultState))
			}
		})
	}
}
