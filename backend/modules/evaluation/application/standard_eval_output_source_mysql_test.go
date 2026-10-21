// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package application

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	mysqlDriver "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSourceStandardMySQLApplicationRealWiring(t *testing.T) {
	dsn := os.Getenv("EXPT_RETRY_MYSQL_DSN")
	if dsn == "" {
		t.Skip("real task MySQL DSN not configured")
	}
	provider, err := db.NewDB(mysqlDriver.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	conn := provider.NewSession(context.Background(), db.WithMaster())
	var identity struct{ Version, Schema string }
	require.NoError(t, conn.Raw("SELECT VERSION() AS version,DATABASE() AS `schema`").Scan(&identity).Error)
	require.Equal(t, "expt_running_retry_test", identity.Schema)
	base := int64(7384603622000000000) + time.Now().UnixNano()%1000000000000
	space, expt, run, item := base+1, base+2, base+3, base+11
	t.Cleanup(func() {
		for _, table := range []string{"expt_turn_evaluator_result_ref", "expt_turn_result_run_log", "expt_turn_result", "expt_item_result_run_log", "expt_item_result", "expt_item_ref", "expt_run_log"} {
			require.NoError(t, conn.Exec("DELETE FROM "+table+" WHERE space_id=? AND expt_id=?", space, expt).Error)
		}
		require.NoError(t, conn.Exec("DELETE FROM experiment WHERE space_id=? AND id=?", space, expt).Error)
		pool, err := conn.DB()
		require.NoError(t, err)
		require.NoError(t, pool.Close())
	})
	require.NoError(t, conn.Exec("INSERT INTO experiment(id,space_id,latest_run_id,status,expt_type) VALUES(?,?,?,?,?)", expt, space, run, entity.ExptStatus_Success, entity.ExptType_Offline).Error)
	require.NoError(t, conn.Create(&model.ExptRunLog{ID: run, SpaceID: space, ExptID: expt, ExptRunID: run, Status: gptr.Of(int64(entity.ExptStatus_Success)), Mode: gptr.Of(int32(entity.EvaluationModeSubmit)), ItemIds: gptr.Of([]byte(`[{"ItemIDs":[],"CreateAt":1700000000}]`))}).Error)
	require.NoError(t, conn.Create(&model.ExptItemResult{ID: base + 12, SpaceID: space, ExptID: expt, ExptRunID: run, ItemID: item, Status: int32(entity.ItemRunState_Success), Ext: gptr.Of([]byte(`{"business":"keep"}`))}).Error)
	require.NoError(t, conn.Create(&model.ExptItemResultRunLog{ID: base + 13, SpaceID: space, ExptID: expt, ExptRunID: run, ItemID: item, Status: int32(entity.ItemRunState_Success), ResultState: gptr.Of(int32(entity.ExptItemResultStateResulted))}).Error)
	require.NoError(t, conn.Create(&model.ExptItemRef{ID: base + 14, SpaceID: space, ExptID: expt, ItemID: item, ItemConfig: gptr.Of([]byte(`{}`))}).Error)
	require.NoError(t, conn.Create(&model.ExptTurnResult{ID: base + 15, SpaceID: space, ExptID: expt, ExptRunID: run, ItemID: item, TurnID: 0, TargetResultID: 0, Status: int32(entity.TurnRunState_Success)}).Error)
	require.NoError(t, conn.Create(&model.ExptTurnResultRunLog{ID: base + 16, SpaceID: space, ExptID: expt, ExptRunID: run, ItemID: item, TurnID: 0, TargetResultID: 0, Status: int32(entity.TurnRunState_Success)}).Error)
	ctrl := gomock.NewController(t)
	auth := rpcmocks.NewMockIAuthProvider(ctrl)
	auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	result := service.NewExptResultServiceWithPublication(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, exptrepo.NewExptResultPublishRepo(provider), nil)
	app := &experimentApplication{auth: auth, resultSvc: result}
	snapshot := func() map[string]string {
		out := map[string]string{}
		for _, table := range []string{"experiment", "expt_turn_evaluator_result_ref", "expt_turn_result_run_log", "expt_turn_result", "expt_item_result_run_log", "expt_item_result", "expt_item_ref", "expt_run_log", "expt_stats", "eval_target_record", "evaluator_record"} {
			var rows []map[string]any
			require.NoError(t, conn.Table(table).Where("space_id=?", space).Order("id").Find(&rows).Error)
			raw, err := json.Marshal(rows)
			require.NoError(t, err)
			out[table] = string(raw)
		}
		return out
	}
	before := snapshot()
	readRecords := 0
	require.NoError(t, conn.Callback().Query().After("gorm:query").Register("application_source_only_reader", func(tx *gorm.DB) {
		if tx.Statement.Table == "eval_target_record" || tx.Statement.Table == "evaluator_record" {
			readRecords++
		}
	}))
	frozen, err := result.(service.ExptStandardEvalOutputSourceService).GetStandardEvalOutputSource(context.Background(), space, expt, run, item)
	require.NoError(t, err)
	req := sourceStandardExpectedRequest(frozen)
	req.SourceOnly = gptr.Of(true)
	resp, err := app.MGetExperimentStandardEvalOutputs(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	require.Nil(t, resp.Items[0].Output)
	require.Zero(t, readRecords)
	require.NoError(t, conn.Callback().Query().Remove("application_source_only_reader"))
	require.Equal(t, before, snapshot())
	body, err := json.Marshal(resp.Items[0].SourceTurnRecords)
	require.NoError(t, err)
	records, err := service.StandardEvalOutputSourceRecords(frozen)
	require.NoError(t, err)
	mqbody, err := json.Marshal(records)
	require.NoError(t, err)
	require.JSONEq(t, string(mqbody), string(body))
	t.Logf("REAL_MYSQL_STANDARD_APPLICATION schema=%s expt=%d run=%d item=%d raw_request_present=true turn=0 target=0 record_reads=0 snapshots=11", identity.Schema, expt, run, item)
	// Same source during MQ-before-commit must retry until Resulted.
	require.NoError(t, conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=?", space).Update("result_state", int32(entity.ExptItemResultStateLogged)).Error)
	resp, err = app.MGetExperimentStandardEvalOutputs(context.Background(), req)
	require.Nil(t, resp)
	require.Equal(t, int32(errno.StandardEvalOutputSourceNotReadyCode), sourceStandardErrorCode(t, err))
	// A newer request remains stale even while it is not yet Resulted.
	manual, err := json.Marshal(&entity.ManualRetryContext{RunID: run, RequestID: "rq2", Mode: entity.EvaluationModeFailRetry, Operator: "test", AcceptedAtMS: 1})
	require.NoError(t, err)
	ext, err := json.Marshal(map[string]string{entity.ManualRetryExtKey: string(manual)})
	require.NoError(t, err)
	require.NoError(t, conn.Model(&model.ExptItemResult{}).Where("space_id=?", space).Update("ext", ext).Error)
	resp, err = app.MGetExperimentStandardEvalOutputs(context.Background(), req)
	require.Nil(t, resp)
	require.Equal(t, int32(errno.StandardEvalOutputSourceMismatchCode), sourceStandardErrorCode(t, err))

	// The content reader is a callback, but both freeze and post-read use the real production primary source port.
	for _, scenario := range []string{"same", "rq_changed", "record_changed", "latest_changed", "post_read_error"} {
		t.Run("full_"+scenario, func(t *testing.T) {
			require.NoError(t, conn.Model(&model.Experiment{}).Where("space_id=?", space).Update("latest_run_id", run).Error)
			require.NoError(t, conn.Model(&model.ExptItemResult{}).Where("space_id=?", space).Update("ext", []byte(`{"business":"keep"}`)).Error)
			require.NoError(t, conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=?", space).Update("result_state", int32(entity.ExptItemResultStateResulted)).Error)
			for _, table := range []string{"expt_turn_result", "expt_turn_result_run_log"} {
				require.NoError(t, conn.Table(table).Where("space_id=?", space).Update("target_result_id", 0).Error)
			}
			current, err := result.(service.ExptStandardEvalOutputSourceService).GetStandardEvalOutputSource(context.Background(), space, expt, run, item)
			require.NoError(t, err)
			fullReq := sourceStandardExpectedRequest(current)
			control := gomock.NewController(t)
			manager := servicemocks.NewMockIExptManager(control)
			metadata := makeStandardEvalOutputExpt(expt, space)
			metadata.LatestRunID = run
			manager.EXPECT().GetDetail(gomock.Any(), expt, space, gomock.Any()).Return(metadata, nil).AnyTimes()
			fault := errors.New("post-read primary unavailable")
			reader := &sourceStandardFullProductionService{ExptResultService: result}
			reader.read = func(_ context.Context, _ *entity.MGetExperimentResultParam) (*entity.MGetExperimentReportResult, error) {
				report := makeStandardEvalOutputReportResult(expt, run, item, 0, 0)
				payload := report.ItemResults[0].TurnResults[0].ExperimentResults[0].Payload
				payload.TargetOutput = nil
				payload.EvaluatorOutput = nil
				switch scenario {
				case "rq_changed":
					require.NoError(t, conn.Model(&model.ExptItemResult{}).Where("space_id=?", space).Update("ext", ext).Error)
				case "record_changed":
					for _, table := range []string{"expt_turn_result", "expt_turn_result_run_log"} {
						require.NoError(t, conn.Table(table).Where("space_id=?", space).Update("target_result_id", base+99).Error)
					}
				case "latest_changed":
					require.NoError(t, conn.Model(&model.Experiment{}).Where("space_id=?", space).Update("latest_run_id", run+1).Error)
				case "post_read_error":
					require.NoError(t, conn.Callback().Query().Before("gorm:query").Register("source_standard_post_read_fault", func(tx *gorm.DB) { _ = tx.AddError(fault) }))
				}
				return report, nil
			}
			fullApp := &experimentApplication{auth: auth, resultSvc: reader, manager: manager}
			response, err := fullApp.MGetExperimentStandardEvalOutputs(context.Background(), fullReq)
			switch scenario {
			case "post_read_error":
				require.NoError(t, conn.Callback().Query().Remove("source_standard_post_read_fault"))
				require.ErrorIs(t, err, fault)
				require.Nil(t, response)
			case "same":
				require.NoError(t, err)
				require.NotNil(t, response.Items[0].Output)
			default:
				require.Equal(t, int32(errno.StandardEvalOutputSourceMismatchCode), sourceStandardErrorCode(t, err))
				require.Nil(t, response, "changed actual primary source returns zero contents")
			}
		})
	}
}

type sourceStandardFullProductionService struct {
	service.ExptResultService
	read func(context.Context, *entity.MGetExperimentResultParam) (*entity.MGetExperimentReportResult, error)
}

func (s *sourceStandardFullProductionService) MGetExperimentResult(ctx context.Context, param *entity.MGetExperimentResultParam) (*entity.MGetExperimentReportResult, error) {
	return s.read(ctx, param)
}

func (s *sourceStandardFullProductionService) GetStandardEvalOutputSource(ctx context.Context, space, expt, run, item int64, expected ...*string) (*entity.ExptStandardEvalOutputSource, error) {
	return s.ExptResultService.(service.ExptStandardEvalOutputSourceService).GetStandardEvalOutputSource(ctx, space, expt, run, item, expected...)
}
