// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	mysqlDriver "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	idmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	lwtmocks "github.com/coze-dev/coze-loop/backend/infra/platestwrite/mocks"
	dexpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	exptpb "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func TestRunningRetryMySQLApplicationIdentityRejection(t *testing.T) {
	dsn := os.Getenv("EXPT_RETRY_MYSQL_DSN")
	if dsn == "" {
		t.Skip("EXPT_RETRY_MYSQL_DSN not configured")
	}
	provider, err := db.NewDB(mysqlDriver.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	conn := provider.NewSession(context.Background(), db.WithMaster())
	var identity struct{ Version, Schema string }
	require.NoError(t, conn.Raw("SELECT VERSION() AS version, DATABASE() AS `schema`").Scan(&identity).Error)
	require.Equal(t, "expt_running_retry_test", identity.Schema)
	t.Logf("REAL_MYSQL_APPLICATION_EXECUTED version=%s schema=%s", identity.Version, identity.Schema)
	base := int64(870000000000000000) + time.Now().UnixNano()%1000000000000
	space, expt, run, item := base+1, base+2, base+3, base+11
	t.Cleanup(func() {
		for _, table := range []string{"expt_turn_evaluator_result_ref", "expt_turn_result_run_log", "expt_turn_result", "expt_item_result_run_log", "expt_item_result", "expt_item_ref", "expt_run_log", "expt_stats"} {
			require.NoError(t, conn.Exec("DELETE FROM "+table+" WHERE space_id=? AND expt_id=?", space, expt).Error)
		}
		require.NoError(t, conn.Exec("DELETE FROM experiment WHERE space_id=? AND id=?", space, expt).Error)
		sqlDB, err := conn.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, conn.Exec("INSERT INTO experiment(id,space_id,latest_run_id,status,expt_type,created_by,eval_conf,eval_set_id,eval_set_version_id) VALUES(?,?,?,?,?,?,?,?,?)", expt, space, run, entity.ExptStatus_Processing, entity.ExptType_Offline, "original", `{"item_retry_num":3,"item_concur_num":4}`, base+80, base+81).Error)
	raw, err := json.Marshal([]entity.ExptRunLogItems{{ItemIDs: []int64{item}}})
	require.NoError(t, err)
	require.NoError(t, conn.Create(&model.ExptRunLog{ID: run, SpaceID: space, ExptID: expt, ExptRunID: run, Status: gptr.Of(int64(entity.ExptStatus_Processing)), Mode: gptr.Of(int32(entity.EvaluationModeSubmit)), CreatedBy: "original", ItemIds: &raw, FailCnt: 1}).Error)
	require.NoError(t, conn.Create(&model.ExptStats{ID: base + 4, SpaceID: space, ExptID: expt, FailCnt: 1}).Error)
	require.NoError(t, conn.Create(&model.ExptItemResult{ID: base + 12, SpaceID: space, ExptID: expt, ExptRunID: run, ItemID: item, ItemVersionID: base + 91, Status: 3, Ext: gptr.Of([]byte(`{"business":"keep"}`))}).Error)
	require.NoError(t, conn.Create(&model.ExptItemResultRunLog{ID: base + 13, SpaceID: space, ExptID: expt, ExptRunID: run, ItemID: item, ItemVersionID: base + 91, Status: 3, ResultState: gptr.Of(int32(1)), RetryTimes: 3}).Error)
	require.NoError(t, conn.Create(&model.ExptItemRef{ID: base + 14, SpaceID: space, ExptID: expt, ItemID: item, ItemVersionID: base + 91, ItemConfig: gptr.Of([]byte(`{}`))}).Error)
	require.NoError(t, conn.Create(&model.ExptTurnResult{ID: base + 15, SpaceID: space, ExptID: expt, ExptRunID: run, ItemID: item, ItemVersionID: base + 91, TurnID: base + 21, Status: 2, TargetResultID: 123, WeightedScore: gptr.Of(0.0)}).Error)
	require.NoError(t, conn.Create(&model.ExptTurnResultRunLog{ID: base + 16, SpaceID: space, ExptID: expt, ExptRunID: run, ItemID: item, ItemVersionID: base + 91, TurnID: base + 21, Status: 2, TargetResultID: 123, EvaluatorResultIds: gptr.Of([]byte(`null`)), Ext: []byte(`{"other":"keep"}`)}).Error)
	ctrl := gomock.NewController(t)
	config := confmocks.NewMockIConfiger(ctrl)
	auth := rpcmocks.NewMockIAuthProvider(ctrl)
	generator := idmocks.NewMockIIDGenerator(ctrl)
	lwt := lwtmocks.NewMockILatestWriteTracker(ctrl)
	mutex := lockmocks.NewMockILocker(ctrl)
	publisher := eventmocks.NewMockExptEventPublisher(ctrl)
	lwt.EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), expt).Return(false).AnyTimes()
	auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	manager := service.NewExptManager(nil,
		exptrepo.NewExptRepo(exptmysql.NewExptDAO(provider), exptmysql.NewExptEvaluatorRefDAO(provider), generator),
		exptrepo.NewExptRunLogRepo(exptmysql.NewExptRunLogDAO(provider)), exptrepo.NewExptStatsRepo(exptmysql.NewExptStatsDAO(provider)), exptrepo.NewExptItemResultRepo(exptmysql.NewExptItemResultDAO(provider)), exptrepo.NewExptItemRefRepo(exptmysql.NewExptItemRefDAO(provider)), exptrepo.NewExptTurnResultRepo(generator, exptmysql.NewExptTurnResultDAO(provider), exptmysql.NewExptTurnEvaluatorResultRefDAO(provider)), config, nil, mutex, nil, publisher, nil, generator, nil, lwt, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, component.NewNoopCentralSchedulerScopeProvider(), component.NewNoopCentralAdmissionPolicy(), component.NewNoopCentralReservationGuard(), exptrepo.NewExptManualRetryRepo(provider, generator), nil, component.NewNoopCurrentRunSchedulerWaker(), func() lock.ILocker { return mutex })
	app := &experimentApplication{manager: manager, auth: auth, idgen: generator}
	req := &exptpb.RetryExperimentRequest{WorkspaceID: &space, ExptID: &expt, ExpectedRunID: &run, RetryMode: gptr.Of(dexpt.ExptRetryMode_RetryTargetItems), Ext: map[string]string{"caller": "identity"}}
	snapshot := func() []byte {
		t.Helper()
		state := map[string][]map[string]any{}
		for _, table := range []string{"experiment", "expt_run_log", "expt_stats", "expt_item_result", "expt_item_result_run_log", "expt_item_ref", "expt_turn_result", "expt_turn_result_run_log", "expt_turn_evaluator_result_ref"} {
			rows := []map[string]any{}
			query := conn.Table(table).Where("space_id=? AND expt_id=?", space, expt)
			if table == "experiment" {
				query = conn.Table(table).Where("space_id=? AND id=?", space, expt)
			}
			require.NoError(t, query.Order("id").Find(&rows).Error)
			state[table] = rows
		}
		body, err := json.Marshal(state)
		require.NoError(t, err)
		return body
	}
	for _, status := range []entity.ExptStatus{entity.ExptStatus_Processing, entity.ExptStatus_Success} {
		require.NoError(t, conn.Model(&model.Experiment{}).Where("id=?", expt).Update("status", status).Error)
		before := snapshot()
		for _, mode := range []dexpt.ExptRetryMode{dexpt.ExptRetryMode_RetryFailure, dexpt.ExptRetryMode_RetryTargetItems, dexpt.ExptRetryMode_RetryAll} {
			req.RetryMode = gptr.Of(mode)
			req.ItemIds = nil
			if mode == dexpt.ExptRetryMode_RetryTargetItems {
				req.ItemIds = []int64{item}
			}
			for _, expected := range []int64{run, 0} {
				req.ExpectedRunID = gptr.Of(expected)
				response, err := app.RetryExperiment(context.Background(), req)
				require.ErrorContains(t, err, "expected_run_id")
				require.Nil(t, response)
				require.Equal(t, before, snapshot())
			}
		}
	}
	t.Log("REAL_MYSQL_APPLICATION_RESULT_VERIFIED explicit identity rejection preserves all seeded tables")
}
