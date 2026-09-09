// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/ctxcache"
)

func TestRetryYield_PreservesTerminalAndCurrentRun(t *testing.T) {
	for _, transition := range []string{"processing control", "terminal logged", "terminal resulted", "new run", "duplicate yield", "old attempt", "run update failure", "item update failure", "stats update failure"} {
		t.Run(transition, func(t *testing.T) {
			ctx := ctxcache.Init(context.Background())
			provider, err := db.NewDB(sqlite.Open(":memory:"), &gorm.Config{})
			require.NoError(t, err)
			conn := provider.NewSession(ctx)
			sqlDB, err := conn.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
			for _, ddl := range []string{
				`CREATE TABLE expt_item_result (id INTEGER PRIMARY KEY,space_id INTEGER,expt_id INTEGER,expt_run_id INTEGER,item_id INTEGER,status INTEGER,err_msg BLOB,updated_at DATETIME,deleted_at DATETIME)`,
				`CREATE TABLE expt_item_result_run_log (id INTEGER PRIMARY KEY,space_id INTEGER,expt_id INTEGER,expt_run_id INTEGER,item_id INTEGER,status INTEGER,result_state INTEGER,retry_times INTEGER,err_msg BLOB,updated_at DATETIME,deleted_at DATETIME)`,
				`CREATE TABLE expt_stats (id INTEGER PRIMARY KEY,space_id INTEGER,expt_id INTEGER,pending_cnt INTEGER,processing_cnt INTEGER,terminated_cnt INTEGER,updated_at DATETIME,deleted_at DATETIME)`,
				`CREATE TABLE expt_turn_result (id INTEGER PRIMARY KEY,target_result_id INTEGER,weighted_score REAL)`,
				`CREATE TABLE expt_turn_evaluator_result_ref (id INTEGER PRIMARY KEY,evaluator_result_id INTEGER)`,
				`INSERT INTO expt_item_result VALUES (11,3,1,100,7,1,NULL,NULL,NULL)`,
				`INSERT INTO expt_item_result_run_log VALUES (12,3,1,100,7,1,0,1,NULL,NULL,NULL)`,
				`INSERT INTO expt_stats VALUES (14,3,1,0,1,0,NULL,NULL)`,
				`INSERT INTO expt_turn_result VALUES (13,77,0.5)`,
				`INSERT INTO expt_turn_evaluator_result_ref VALUES (15,88)`,
			} {
				require.NoError(t, conn.Exec(ddl).Error)
			}
			ctrl := gomock.NewController(t)
			configer := componentmocks.NewMockIConfiger(ctrl)
			configer.EXPECT().GetErrRetryConf(gomock.Any(), int64(3), gomock.Any()).AnyTimes().Return(&entity.RetryConf{RetryTimes: 3})
			metric := metricsmocks.NewMockExptMetric(ctrl)
			metric.EXPECT().EmitItemExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			service := &ExptItemEventEvalServiceImpl{
				exptItemResultRepo: exptrepo.NewExptItemResultRepo(exptmysql.NewExptItemResultDAO(provider)),
				configer:           configer, metric: metric, publisher: eventmocks.NewMockExptEventPublisher(ctrl),
			}
			event := &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 100, SpaceID: 3, EvalSetItemID: 7, RetryTimes: 1, Ext: map[string]string{entity.RetryYieldExtKey: "true"}}
			handler := service.HandleEventErr(func(context.Context, *entity.ExptItemEvalEvent) error {
				switch transition {
				case "terminal logged", "terminal resulted":
					resultState := entity.ExptItemResultStateLogged
					if transition == "terminal resulted" {
						resultState = entity.ExptItemResultStateResulted
					}
					require.NoError(t, conn.Exec(`UPDATE expt_item_result_run_log SET status=5,result_state=?,err_msg=? WHERE id=12`, int32(resultState), []byte("manually terminated")).Error)
					if transition == "terminal resulted" {
						require.NoError(t, conn.Exec(`UPDATE expt_item_result SET status=5 WHERE id=11`).Error)
						require.NoError(t, conn.Exec(`UPDATE expt_stats SET processing_cnt=0,terminated_cnt=1 WHERE id=14`).Error)
					}
				case "new run":
					require.NoError(t, conn.Exec(`UPDATE expt_item_result SET expt_run_id=200 WHERE id=11`).Error)
				case "old attempt":
					require.NoError(t, conn.Exec(`UPDATE expt_item_result_run_log SET retry_times=2 WHERE id=12`).Error)
				case "run update failure", "item update failure", "stats update failure":
					table := map[string]string{"run update failure": "expt_item_result_run_log", "item update failure": "expt_item_result", "stats update failure": "expt_stats"}[transition]
					require.NoError(t, conn.Exec(`CREATE TRIGGER fail_update BEFORE UPDATE ON `+table+` BEGIN SELECT RAISE(ABORT,'write unavailable'); END`).Error)
				}
				return errors.New("retryable downstream failure")
			})
			require.NoError(t, handler(ctx, event))
			if transition == "duplicate yield" {
				require.NoError(t, handler(ctx, event))
			}
			var item model.ExptItemResult
			var run model.ExptItemResultRunLog
			var counts struct{ PendingCnt, ProcessingCnt, TerminatedCnt int64 }
			var turn struct {
				TargetResultID int64
				WeightedScore  float64
			}
			var evaluatorID int64
			require.NoError(t, conn.First(&item, 11).Error)
			require.NoError(t, conn.First(&run, 12).Error)
			require.NoError(t, conn.Table("expt_stats").Select("pending_cnt,processing_cnt,terminated_cnt").Scan(&counts).Error)
			require.NoError(t, conn.Table("expt_turn_result").Select("target_result_id,weighted_score").Scan(&turn).Error)
			require.NoError(t, conn.Table("expt_turn_evaluator_result_ref").Select("evaluator_result_id").Scan(&evaluatorID).Error)
			assert.Equal(t, int64(77), turn.TargetResultID)
			assert.Equal(t, 0.5, turn.WeightedScore)
			assert.Equal(t, int64(88), evaluatorID)
			switch transition {
			case "processing control", "duplicate yield":
				assert.Equal(t, int32(entity.ItemRunState_Queueing), run.Status)
				assert.Equal(t, int32(entity.ItemRunState_Queueing), item.Status)
				assert.Equal(t, int32(2), run.RetryTimes)
				assert.Equal(t, int64(1), counts.PendingCnt)
				assert.Equal(t, int64(0), counts.ProcessingCnt)
			case "terminal logged", "terminal resulted":
				assert.Equal(t, int32(entity.ItemRunState_Terminal), run.Status)
				assert.Equal(t, []byte("manually terminated"), gptr.Indirect(run.ErrMsg))
				assert.Equal(t, int32(1), run.RetryTimes)
				assert.Equal(t, int64(0), counts.PendingCnt)
				if transition == "terminal resulted" {
					assert.Equal(t, int32(entity.ItemRunState_Terminal), item.Status)
					assert.Equal(t, int64(0), counts.ProcessingCnt)
					assert.Equal(t, int64(1), counts.TerminatedCnt)
				} else {
					assert.Equal(t, int32(entity.ItemRunState_Processing), item.Status)
					assert.Equal(t, int64(1), counts.ProcessingCnt)
				}
			default:
				assert.Equal(t, int32(entity.ItemRunState_Processing), run.Status)
				assert.Equal(t, int32(entity.ItemRunState_Processing), item.Status)
				assert.Equal(t, int64(0), counts.PendingCnt)
				assert.Equal(t, int64(1), counts.ProcessingCnt)
				if transition == "new run" {
					assert.Equal(t, int64(200), item.ExptRunID)
				}
				wantRetry := int32(1)
				if transition == "old attempt" {
					wantRetry = 2
				}
				assert.Equal(t, wantRetry, run.RetryTimes)
			}
		})
	}
}
