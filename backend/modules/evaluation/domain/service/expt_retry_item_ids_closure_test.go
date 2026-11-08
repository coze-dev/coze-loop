// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	idmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	targetrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target"
	targetmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/ctxcache"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

type retryItemIDsClosureFixture struct {
	f        *lateWriteFixture
	manager  *ExptMangerImpl
	metadata []byte
}

func newRetryItemIDsClosureFixture(t *testing.T, parent entity.ExptRunMode, currentLog, listed, nearCapacity bool) *retryItemIDsClosureFixture {
	t.Helper()
	require.Contains(t, os.Getenv("EXPT_RETRY_MYSQL_DSN"), "@tcp(127.0.0.1:3306)/expt_running_retry_test?", "real local MySQL is required; this regression must not skip")
	f := newLateWriteFixture(t, "")
	client := busyRetryRedis(t)
	t.Cleanup(func() {
		require.Eventually(t, func() bool {
			count, err := client.Exists(context.Background(), fmt.Sprintf("expt_run_exec_lock:%d:%d", f.expt, f.run), fmt.Sprintf("expt_item_eval_run_lock:%d:%d", f.expt, f.item)).Result()
			require.NoError(t, err)
			return count == 0
		}, 2*time.Second, time.Millisecond, "wait for production lock cancellation before closing Redis")
	})
	ids := idmocks.NewMockIIDGenerator(gomock.NewController(t))
	nextID := f.base + 10000
	ids.EXPECT().GenID(gomock.Any()).DoAndReturn(func(context.Context) (int64, error) {
		nextID++
		return nextID, nil
	}).AnyTimes()
	manager := &ExptMangerImpl{
		exptRepo: f.experimentRepo(), runLogRepo: exptrepo.NewExptRunLogRepo(exptmysql.NewExptRunLogDAO(f.provider)),
		itemResultRepo: f.itemRepo(), turnResultRepo: f.turnRepo(),
		manualRetryRepo:   exptrepo.NewExptManualRetryRepo(f.provider, ids),
		manualRetryLocker: func() lock.ILocker { return lock.NewRedisLocker(client) },
	}

	// D is selected; A stays Processing; E is terminal and never selected.
	d, a, e := f.item, f.item+1, f.item+2
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=? AND id=?", f.space, f.expt).Updates(map[string]any{
		"eval_set_source_type": int32(entity.ExptEvalSetSourceType_SingleSet), "eval_set_id": f.base + 80, "eval_set_version_id": f.base + 81,
	}).Error)
	require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND expt_id=? AND item_id IN ?", f.space, f.expt, []int64{d, e}).Update("status", int32(entity.ItemRunState_Success)).Error)
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND expt_id=? AND item_id IN ?", f.space, f.expt, []int64{d, e}).Updates(map[string]any{
		"status": int32(entity.ItemRunState_Success), "result_state": int32(entity.ExptItemResultStateResulted),
	}).Error)
	for _, table := range []string{"expt_turn_result", "expt_turn_result_run_log"} {
		require.NoError(t, f.conn.Table(table).Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, d).Update("status", int32(entity.TurnRunState_Success)).Error)
	}
	for i, status := range []entity.TurnRunState{entity.TurnRunState_Processing, entity.TurnRunState_Success} {
		itemID := a + int64(i)
		require.NoError(t, f.conn.Create(&model.ExptTurnResult{ID: f.base + 450 + int64(i), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run,
			ItemID: itemID, ItemVersionID: f.version, TurnID: 0, Status: int32(status)}).Error)
		require.NoError(t, f.conn.Create(&model.ExptTurnResultRunLog{ID: f.base + 550 + int64(i), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run,
			ItemID: itemID, ItemVersionID: f.version, TurnID: 0, Status: int32(status), EvaluatorResultIds: gptr.Of([]byte(`{}`))}).Error)
	}
	if !currentLog {
		oldRun := f.base + 700
		oldMetadata := []byte(fmt.Sprintf(`[{"ItemIDs":[%d],"CreateAt":1700000000}]`, d))
		require.NoError(t, f.conn.Create(&model.ExptRunLog{ID: oldRun, SpaceID: f.space, ExptID: f.expt, ExptRunID: oldRun,
			Status: gptr.Of(int64(entity.ExptStatus_Success)), Mode: gptr.Of(int32(entity.EvaluationModeSubmit)), ItemIds: &oldMetadata, SuccessCnt: 1}).Error)
		for _, table := range []string{"expt_item_result", "expt_item_result_run_log", "expt_turn_result", "expt_turn_result_run_log"} {
			require.NoError(t, f.conn.Table(table).Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, d).Update("expt_run_id", oldRun).Error)
		}
	}
	initialIDs := []int64{a}
	if listed {
		initialIDs = []int64{a, d}
	}
	encodedIDs, err := json.Marshal(initialIDs)
	require.NoError(t, err)
	metadata := []byte(fmt.Sprintf(`[{"ItemIDs":%s,"CreateAt":1700000000,"future":"keep"}]`, encodedIDs))
	if nearCapacity {
		metadata = []byte(fmt.Sprintf(`[{"ItemIDs":%s,"CreateAt":1700000000,"future":"%s"}]`, encodedIDs, strings.Repeat("x", 65520-len(metadata)+4)))
		require.Len(t, metadata, 65520)
	}
	successCount := int32(1)
	if currentLog {
		successCount = 2
	}
	require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, f.run).Updates(map[string]any{
		"mode": int32(parent), "item_ids": metadata, "pending_cnt": 0, "processing_cnt": 1, "success_cnt": successCount,
		"fail_cnt": 0, "terminated_cnt": 0, "credit_cost": 12.5, "token_cost": 70,
	}).Error)
	require.NoError(t, f.conn.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", f.space, f.expt).Updates(map[string]any{
		"pending_cnt": 0, "processing_cnt": 1, "success_cnt": 2, "fail_cnt": 0, "terminated_cnt": 0,
		"credit_cost": 12.5, "input_token_cost": 30, "output_token_cost": 40,
	}).Error)
	return &retryItemIDsClosureFixture{f: f, manager: manager, metadata: metadata}
}

func (p *retryItemIDsClosureFixture) request() *entity.RetryInCurrentRunRequest {
	return &entity.RetryInCurrentRunRequest{SpaceID: p.f.space, ExptID: p.f.expt, ExpectedRunID: p.f.run,
		Mode: entity.EvaluationModeRetryItems, ItemIDs: []int64{p.f.item, p.f.item}, Session: &entity.Session{UserID: "item-ids-closure"}}
}

func (p *retryItemIDsClosureFixture) unchangedRows(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, table := range []string{"experiment", "expt_item_ref", "eval_target_record", "evaluator_record", "expt_item_result", "expt_turn_result", "expt_item_result_run_log", "expt_turn_result_run_log", "expt_run_log"} {
		query := p.f.conn.Table(table).Where("space_id=?", p.f.space)
		switch table {
		case "expt_item_result", "expt_turn_result":
			query = query.Where("item_id IN ?", []int64{p.f.item + 1, p.f.item + 2})
		case "expt_item_result_run_log", "expt_turn_result_run_log":
			query = query.Where("(item_id IN ? OR expt_run_id<>?)", []int64{p.f.item + 1, p.f.item + 2}, p.f.run)
		case "expt_run_log":
			query = query.Where("expt_run_id<>?", p.f.run)
		}
		var rows []map[string]any
		require.NoError(t, query.Order("id").Find(&rows).Error)
		raw, err := json.Marshal(rows)
		require.NoError(t, err)
		out[table] = string(raw)
	}
	return out
}

// Catches a log-existence predicate, a request-mode predicate, or unconditional
// append in the commit transaction; each changes real persisted membership.
func TestRetryItemIDsClosureRealMySQLAdmission(t *testing.T) {
	for _, parent := range []struct {
		name        string
		mode        entity.ExptRunMode
		addUnlisted bool
	}{
		{"RetryItems", entity.EvaluationModeRetryItems, true},
		{"Submit", entity.EvaluationModeSubmit, false},
		{"RetryAll", entity.EvaluationModeRetryAll, false},
		{"FailRetry", entity.EvaluationModeFailRetry, false},
	} {
		for _, source := range []struct {
			name       string
			currentLog bool
			listed     bool
		}{
			{"current_log_unlisted", true, false},
			{"current_log_listed", true, true},
			{"historical_log_unlisted", false, false},
			{"historical_log_listed", false, true},
		} {
			t.Run(parent.name+"/"+source.name, func(t *testing.T) {
				p := newRetryItemIDsClosureFixture(t, parent.mode, source.currentLog, source.listed, false)
				f := p.f
				before := p.unchangedRows(t)
				receipt, err := p.manager.RetryInCurrentRun(context.Background(), p.request())
				require.NoError(t, err)
				require.NotNil(t, receipt)
				require.Equal(t, f.run, receipt.RunID)
				require.Equal(t, int32(1), receipt.AcceptedCount)
				require.Empty(t, receipt.RejectedItems)

				var item model.ExptItemResult
				require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, f.item).First(&item).Error)
				require.Equal(t, f.base+100, item.ID)
				require.Equal(t, f.run, item.ExptRunID)
				require.Equal(t, int32(entity.ItemRunState_Queueing), item.Status)
				ext, err := entity.ParseExptItemResultExt(gptr.Indirect(item.Ext))
				require.NoError(t, err)
				require.Equal(t, "keep", ext["other"])
				retry, err := entity.ParseManualRetryContext(ext)
				require.NoError(t, err)
				require.NotNil(t, retry)
				require.Equal(t, f.run, retry.RunID)
				require.Equal(t, entity.EvaluationModeRetryItems, retry.Mode)
				require.NotEmpty(t, retry.RequestID)
				var logs []model.ExptItemResultRunLog
				require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", f.space, f.expt, f.run, f.item).Find(&logs).Error)
				require.Len(t, logs, 1)
				require.Equal(t, int32(entity.ItemRunState_Queueing), logs[0].Status)
				require.Equal(t, int32(entity.ExptItemResultStateDefault), gptr.Indirect(logs[0].ResultState))
				if source.currentLog {
					require.Equal(t, f.base+200, logs[0].ID)
					require.Equal(t, int32(1), logs[0].RetryTimes, "existing retry accounting is preserved")
				} else {
					require.NotEqual(t, f.base+200, logs[0].ID)
					require.Zero(t, logs[0].RetryTimes)
				}
				var turns []model.ExptTurnResultRunLog
				require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", f.space, f.expt, f.run, f.item).Find(&turns).Error)
				require.Len(t, turns, 2)
				for _, turn := range turns {
					require.Equal(t, f.version, turn.ItemVersionID)
					require.Equal(t, int32(entity.TurnRunState_Queueing), turn.Status)
				}
				var run model.ExptRunLog
				require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, f.run).First(&run).Error)
				var chunks []entity.ExptRunLogItems
				require.NoError(t, json.Unmarshal(gptr.Indirect(run.ItemIds), &chunks))
				wantIDs := []int64{f.item + 1}
				if source.listed || parent.addUnlisted {
					wantIDs = []int64{f.item + 1, f.item}
				}
				require.Equal(t, wantIDs, (&entity.ExptRunLog{ItemIds: chunks}).GetItemIDs(), "membership is determined by parent mode, independently of current log existence")
				var original, actual []json.RawMessage
				require.NoError(t, json.Unmarshal(p.metadata, &original))
				require.NoError(t, json.Unmarshal(gptr.Indirect(run.ItemIds), &actual))
				require.JSONEq(t, string(original[0]), string(actual[0]))
				wantChunks := 1
				if parent.addUnlisted && !source.listed {
					wantChunks = 2
				} else {
					require.Equal(t, p.metadata, gptr.Indirect(run.ItemIds))
				}
				require.Len(t, actual, wantChunks)
				require.Equal(t, int32(parent.mode), gptr.Indirect(run.Mode))
				require.Equal(t, int64(entity.ExptStatus_Processing), gptr.Indirect(run.Status))
				require.Equal(t, [5]int32{1, 1, 1, 0, 0}, [5]int32{run.PendingCnt, run.ProcessingCnt, run.SuccessCnt, run.FailCnt, run.TerminatedCnt})
				require.Equal(t, 12.5, run.CreditCost)
				require.Equal(t, int64(70), gptr.Indirect(run.TokenCost))
				var stats model.ExptStats
				require.NoError(t, f.conn.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&stats).Error)
				require.Equal(t, [5]int32{1, 1, 1, 0, 0}, [5]int32{stats.PendingCnt, stats.ProcessingCnt, stats.SuccessCnt, stats.FailCnt, stats.TerminatedCnt})
				require.Equal(t, 12.5, stats.CreditCost)
				require.Equal(t, int64(30), gptr.Indirect(stats.InputTokenCost))
				require.Equal(t, int64(40), gptr.Indirect(stats.OutputTokenCost))
				require.Equal(t, before, p.unchangedRows(t), "unselected A/E and historical logs must remain unchanged")

				requests, err := f.itemRepo().ScanCurrentRequests(context.Background(), f.space, f.expt, f.run, false)
				require.NoError(t, err)
				states := map[int64]entity.ItemRunState{}
				for _, request := range requests {
					states[request.ItemID] = request.Status
				}
				require.Equal(t, map[int64]entity.ItemRunState{f.item: entity.ItemRunState_Queueing, f.item + 1: entity.ItemRunState_Processing, f.item + 2: entity.ItemRunState_Success}, states, "the real scanner still observes current run logs, including D outside non-RetryItems metadata")
			})
		}
	}
}

// At 65,520 bytes, a missing business-ID chunk exceeds BLOB capacity. This
// exposes Preflight's independent membership predicate before any commit.
func TestRetryItemIDsClosureRealMySQLPreflightCapacity(t *testing.T) {
	for _, tc := range []struct {
		name       string
		parent     entity.ExptRunMode
		currentLog bool
		listed     bool
		capacity   bool
	}{
		{"RetryItems/current_log_unlisted", entity.EvaluationModeRetryItems, true, false, true},
		{"RetryItems/historical_log_unlisted", entity.EvaluationModeRetryItems, false, false, true},
		{"RetryItems/current_log_listed", entity.EvaluationModeRetryItems, true, true, false},
		{"RetryItems/historical_log_listed", entity.EvaluationModeRetryItems, false, true, false},
		{"Submit/current_log_unlisted", entity.EvaluationModeSubmit, true, false, false},
		{"Submit/historical_log_unlisted", entity.EvaluationModeSubmit, false, false, false},
		{"RetryAll/current_log_unlisted", entity.EvaluationModeRetryAll, true, false, false},
		{"RetryAll/historical_log_unlisted", entity.EvaluationModeRetryAll, false, false, false},
		{"FailRetry/current_log_unlisted", entity.EvaluationModeFailRetry, true, false, false},
		{"FailRetry/historical_log_unlisted", entity.EvaluationModeFailRetry, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newRetryItemIDsClosureFixture(t, tc.parent, tc.currentLog, tc.listed, true)
			f := p.f
			expt, err := p.manager.exptRepo.GetByID(context.Background(), f.expt, f.space)
			require.NoError(t, err)
			prepared, err := p.manager.prepareTerminalCandidatesAndReusableRefs(context.Background(), expt, f.run, entity.EvaluationModeRetryItems, []int64{f.item}, nil)
			require.NoError(t, err)
			require.Len(t, prepared.Items, 1)
			require.Empty(t, prepared.RejectedItems)
			request := &entity.RequeueTerminalItemsRequest{SpaceID: f.space, ExptID: f.expt, RunID: f.run,
				Mode: entity.EvaluationModeRetryItems, RequestID: "item-ids-preflight", Operator: "item-ids-closure",
				SnapshotAtMS: prepared.SnapshotAtMS, Items: prepared.Items}
			before := f.snapshot()
			checked, err := p.manager.manualRetryRepo.PreflightTerminalItems(context.Background(), request)
			if tc.capacity {
				require.Error(t, err)
				var statusErr errorx.StatusError
				require.ErrorAs(t, err, &statusErr)
				require.Equal(t, int32(errno.ManualRetryCapacityCode), statusErr.Code(), "Preflight must account for unlisted D even when D has a current log")
			} else {
				require.NoError(t, err, "no append is needed for a listed D or a non-RetryItems parent")
				require.Len(t, checked.Items, 1)
				require.Empty(t, checked.RejectedItems)
			}
			require.Equal(t, before, f.snapshot(), "Preflight must never write rows")
			receipt, err := p.manager.RetryInCurrentRun(context.Background(), p.request())
			if tc.capacity {
				require.Error(t, err)
				var statusErr errorx.StatusError
				require.ErrorAs(t, err, &statusErr)
				require.Equal(t, int32(errno.ManualRetryCapacityCode), statusErr.Code())
				require.Nil(t, receipt)
				require.Equal(t, before, f.snapshot(), "capacity rejection leaves terminal results and statistics intact")
			} else {
				require.NoError(t, err)
				require.NotNil(t, receipt)
				require.Equal(t, int32(1), receipt.AcceptedCount)
				require.Empty(t, receipt.RejectedItems)
				var run model.ExptRunLog
				require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, f.run).First(&run).Error)
				require.Equal(t, p.metadata, gptr.Indirect(run.ItemIds))
				current, err := f.itemRepo().GetCurrentRequest(context.Background(), f.space, f.expt, f.run, f.item)
				require.NoError(t, err)
				require.Equal(t, entity.ItemRunState_Queueing, current.Status)
			}
		})
	}
}

// The operator supplies a controlled RPC response; target record creation,
// item execution, status derivation and Resulted publication all stay real.
func TestRetryItemIDsClosureRealMySQLSchedulerExecution(t *testing.T) {
	p := newRetryItemIDsClosureFixture(t, entity.EvaluationModeRetryItems, true, false, false)
	f := p.f
	ctx := context.Background()
	for _, table := range []string{"eval_target", "eval_target_version"} {
		if !f.conn.Migrator().HasTable(table) {
			t.Skipf("NOT_COVERED: existing local fixture lacks %s; no schema changes authorized", table)
		}
	}
	for _, table := range []string{"expt_turn_result", "expt_turn_result_run_log"} {
		require.NoError(t, f.conn.Exec("DELETE FROM "+table+" WHERE space_id=? AND expt_id=? AND item_id=? AND turn_id<>0", f.space, f.expt, f.item).Error)
	}
	actor := newTurnResumeFixture(t, entity.EvaluationModeRetryItems, 0)
	expt := buildRetryFailureSingleSetExpt(f.space)
	expt.ID, expt.LatestRunID, expt.Status = f.expt, f.run, entity.ExptStatus_Processing
	expt.EvalSetID, expt.EvalSetVersionID = f.base+80, f.base+81
	expt.EvalSetSourceType = entity.ExptEvalSetSourceType_SingleSet
	expt.EvalSet = &entity.EvaluationSet{ID: expt.EvalSetID, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: expt.EvalSetVersionID, EvaluationSetID: expt.EvalSetID}}
	expt.EvalConf.ItemConcurNum = gptr.Of(4)
	expt.EvalConf.EnableExtractTrajectory = gptr.Of(false)
	expt.Target.EvalTargetType = entity.EvalTargetTypeCustomRPCServer
	expt.Target.EvalTargetVersion.EvalTargetType = entity.EvalTargetTypeCustomRPCServer
	expt.Target.EvalTargetVersion.SpaceID = f.space
	expt.Target.SpaceID, expt.Target.SourceTargetID = f.space, fmt.Sprint(f.item)
	expt.EvalConf.ConnectorConf.TargetConf.IngressConf = &entity.TargetIngressConf{EvalSetAdapter: &entity.FieldAdapter{}}
	ctrl := gomock.NewController(t)
	ids := idmocks.NewMockIIDGenerator(ctrl)
	ids.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{f.base + 15000, f.base + 15001}, nil)
	ids.EXPECT().GenID(gomock.Any()).Return(f.base+15002, nil)
	client := busyRetryRedis(t)
	targets := targetrepo.NewEvalTargetRepo(ids, f.provider, targetmysql.NewEvalTargetDAO(f.provider), targetmysql.NewEvalTargetVersionDAO(f.provider), targetmysql.NewEvalTargetRecordDAO(f.provider), nil, platestwrite.NewLatestWriteTracker(client))
	t.Cleanup(func() {
		for _, table := range []string{"eval_target_version", "eval_target"} {
			require.NoError(t, f.conn.Exec("DELETE FROM "+table+" WHERE space_id=?", f.space).Error)
		}
	})
	targetID, versionID, err := targets.CreateEvalTarget(ctx, expt.Target)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Del(ctx, fmt.Sprintf("db_last_write_flag:resource:eval_target:%d", targetID), fmt.Sprintf("db_last_write_flag:resource:eval_target_version:%d", versionID)).Err())
	})
	expt.TargetID, expt.TargetVersionID, expt.TargetType = targetID, versionID, entity.EvalTargetTypeCustomRPCServer
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=? AND id=?", f.space, f.expt).Updates(map[string]any{
		"target_id": targetID, "target_version_id": versionID, "target_type": int32(entity.EvalTargetTypeCustomRPCServer),
	}).Error)
	operator := svcmocks.NewMockISourceEvalTargetOperateService(ctrl)
	operator.EXPECT().ValidateInput(gomock.Any(), f.space, gomock.Any(), gomock.Any()).Return(nil)
	operator.EXPECT().Execute(gomock.Any(), f.space, gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, param *entity.ExecuteEvalTargetParam) (*entity.EvalTargetOutputData, entity.EvalTargetRunStatus, error) {
		require.Equal(t, f.expt, param.ExptID)
		require.Equal(t, f.item, gptr.Indirect(param.EvalSetItemID))
		require.Equal(t, int64(0), gptr.Indirect(param.EvalSetTurnID))
		output := resumeTarget(0, 0).EvalTargetOutputData
		for _, content := range output.OutputFields {
			content.ContentType = gptr.Of(entity.ContentTypeText)
		}
		return output, entity.EvalTargetRunStatusSuccess, nil
	})
	metric := metricsmocks.NewMockEvalTargetMetrics(ctrl)
	metric.EXPECT().EmitRun(f.space, nil, gomock.Any())
	targetService := &EvalTargetServiceImpl{evalTargetRepo: targets, idgen: ids, metric: metric,
		typedOperators: map[entity.EvalTargetType]ISourceEvalTargetOperateService{entity.EvalTargetTypeCustomRPCServer: operator}, configer: actor.config}
	actor.executor.ItemResultRepo, actor.executor.TurnResultRepo = f.itemRepo(), f.turnRepo()
	actor.executor.evalTargetService = targetService
	actor.config.EXPECT().BuildEvalExt(gomock.Any(), f.space, gomock.Any()).Return(nil).AnyTimes()
	actor.config.EXPECT().GetExptExecConf(gomock.Any(), f.space).Return(&entity.ExptExecConf{ExptItemEvalConf: &entity.ExptItemEvalConf{ConcurNum: 4, MaxItemConcurNum: 4}}).AnyTimes()
	actor.benefit.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil)
	actor.item.Expt = expt
	actor.item.EvalSetItem.ItemID, actor.item.EvalSetItem.ItemVersionID = f.item, &f.version
	actor.item.EvalSetItem.Turns = []*entity.Turn{{ID: 0, ItemID: f.item}}
	before := p.unchangedRows(t)
	delete(before, "eval_target_record")
	countsBefore, err := targets.CountItemRunsByRun(ctx, f.space, f.run, []int64{f.item, f.item + 2})
	require.NoError(t, err)
	require.Equal(t, int32(1), countsBefore[f.item])
	require.Zero(t, countsBefore[f.item+2])
	receipt, err := p.manager.RetryInCurrentRun(ctx, p.request())
	require.NoError(t, err)
	require.Equal(t, int32(1), receipt.AcceptedCount)
	require.Empty(t, receipt.RejectedItems)
	run, err := p.manager.runLogRepo.Get(ctx, f.expt, f.run)
	require.NoError(t, err)
	require.Equal(t, []int64{f.item + 1, f.item}, run.GetItemIDs())
	factory := func() lock.ILocker { return lock.NewRedisLocker(client) }
	waitReleased := func() {
		require.Eventually(t, func() bool {
			n, err := client.Exists(ctx, fmt.Sprintf("expt_run_exec_lock:%d:%d", f.expt, f.run), fmt.Sprintf("expt_item_eval_run_lock:%d:%d", f.expt, f.item)).Result()
			require.NoError(t, err)
			return n == 0
		}, 2*time.Second, time.Millisecond)
	}
	t.Cleanup(waitReleased)
	waitReleased()
	schedule := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ExptRunMode: entity.EvaluationModeRetryItems,
		ExptType: entity.ExptType_Offline, ExecEvalSetItemIDs: []int64{f.item + 1}, Session: p.request().Session}
	mode := &ExptRetryItemsExec{manager: p.manager, configer: actor.config, exptItemResultRepo: f.itemRepo(), exptRunLogRepo: p.manager.runLogRepo}
	snapshot := f.snapshot()
	require.NoError(t, mode.ScheduleStart(ctx, schedule, expt))
	require.Equal(t, snapshot, f.snapshot(), "ScheduleStart must preserve the already requeued current source")
	toSubmit, incomplete, _, err := mode.ScanEvalItems(ctx, schedule, expt)
	require.NoError(t, err)
	require.Len(t, toSubmit, 1)
	require.Equal(t, f.item, toSubmit[0].ItemID)
	require.Len(t, incomplete, 1)
	publisher := &busyCapturePublisher{published: make(chan *entity.ExptItemEvalEvent, 1)}
	scheduler := &ExptSchedulerImpl{ExptItemResultRepo: f.itemRepo(), Configer: actor.config, Publisher: publisher, itemLocker: factory}
	require.NoError(t, scheduler.submitCurrentItem(ctx, schedule, toSubmit[0], factory()))
	var event *entity.ExptItemEvalEvent
	select {
	case event = <-publisher.published:
	case <-time.After(5 * time.Second):
		t.Fatal("real scheduler did not publish D")
	}
	waitReleased()
	consumer := &ExptItemEventEvalServiceImpl{manager: p.manager, exptItemResultRepo: f.itemRepo(), itemLocker: factory}
	handler := RecordEvalChain(consumer.HandleEventCheck, consumer.HandleEventLock, consumer.HandleCurrentSource)(func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		actor.item.Event = event
		actor.item.ExistItemEvalResult = &entity.ExptItemEvalResult{TurnResultRunLogs: executionSource(ctx).Source.Turns}
		return actor.executor.Eval(ctxcache.Init(ctx), actor.item)
	})
	require.NoError(t, handler(ctx, event))
	waitReleased()
	current, err := f.itemRepo().GetCurrentRequest(ctx, f.space, f.expt, f.run, f.item)
	require.NoError(t, err)
	require.Equal(t, entity.ItemRunState_Success, current.Status)
	require.Equal(t, entity.ExptItemResultStateLogged, current.ResultState)
	stats := exptrepo.NewExptStatsRepo(exptmysql.NewExptStatsDAO(f.provider))
	result := &ExptResultServiceImpl{ExptItemResultRepo: f.itemRepo(), ExptTurnResultRepo: f.turnRepo(), ExptStatsRepo: stats,
		evalTargetService: targetService, evaluatorRecordService: actor.records,
		scoreCalculator: NewEvaluatorScoreCalculator(nil, nil), resultPublishRepo: exptrepo.NewExptResultPublishRepo(f.provider), resultPublishLocker: factory}
	_, err = result.RecordItemRunLogs(ctx, f.expt, f.run, f.item, f.space, expt)
	require.NoError(t, err)
	waitReleased()
	current, err = f.itemRepo().GetCurrentRequest(ctx, f.space, f.expt, f.run, f.item)
	require.NoError(t, err)
	require.Equal(t, entity.ExptItemResultStateResulted, current.ResultState)
	countsAfter, err := targets.CountItemRunsByRun(ctx, f.space, f.run, []int64{f.item, f.item + 2})
	require.NoError(t, err)
	require.Equal(t, int32(2), countsAfter[f.item], "the real target service must create exactly one additional D record")
	require.Zero(t, countsAfter[f.item+2])
	require.NoError(t, handler(ctx, event), "duplicate delivery after Resulted must not invoke the target again")
	waitReleased()
	countsDuplicate, err := targets.CountItemRunsByRun(ctx, f.space, f.run, []int64{f.item, f.item + 2})
	require.NoError(t, err)
	require.Equal(t, countsAfter, countsDuplicate)
	after := p.unchangedRows(t)
	delete(after, "eval_target_record")
	require.Equal(t, before, after, "A/E and historical data stay unchanged through real execution and publication")
	t.Log("REAL_SCHEDULER_ITEM_TARGET_PUBLICATION D_target_records_delta=1 E_delta=0 D=Resulted; controlled RPC operator, real target service/SQL/Redis/executor")
}
