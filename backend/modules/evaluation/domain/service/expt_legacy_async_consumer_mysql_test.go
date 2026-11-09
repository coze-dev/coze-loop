// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	benefitmocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/infra/mq"
	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	userinfomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/userinfo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	evaluatorrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator"
	evaluatormysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	asyncdao "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/dao"
	targetmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql"
	targetconvert "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/convertor"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type legacyConsumerQueue struct {
	events.ExptEventPublisher
	mu     sync.Mutex
	bodies [][]byte
}

func (q *legacyConsumerQueue) PublishExptRecordEvalEvent(_ context.Context, event *entity.ExptItemEvalEvent, _ *time.Duration, opt func(*entity.ExptItemEvalEvent)) error {
	copy := event.Clone()
	if opt != nil {
		opt(copy)
	}
	body, err := json.Marshal(copy)
	if err != nil {
		return err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.bodies = append(q.bodies, body)
	return nil
}

func (q *legacyConsumerQueue) last(t *testing.T) []byte {
	t.Helper()
	q.mu.Lock()
	defer q.mu.Unlock()
	require.NotEmpty(t, q.bodies)
	return append([]byte(nil), q.bodies[len(q.bodies)-1]...)
}

type legacyConsumerRefsBarrier struct {
	repo.IExptItemResultRepo
	repo.IExptCurrentResultRepo
	once   sync.Once
	before func(context.Context) error
}

func (p *legacyConsumerRefsBarrier) StoreCurrentTurnRunResult(ctx context.Context, source *entity.ExptCurrentItemRunSource, row *entity.ExptTurnResultRunLog) (bool, error) {
	var err error
	if row.EvaluatorResultIds != nil && len(legacyEvaluatorRecordIDs(row.EvaluatorResultIds)) > 1 {
		p.once.Do(func() { err = p.before(ctx) })
	}
	if err != nil {
		return false, err
	}
	return p.IExptCurrentResultRepo.StoreCurrentTurnRunResult(ctx, source, row)
}

// The test uses the same serialized old event body and the production-installed
// Check/admission/lock/current-source/reservation/execution chain. Only remote
// providers, dataset reads and the external broker are substituted.
func TestLegacyAsyncPartialEvaluatorConsumerAndArm(t *testing.T) {
	for _, missing := range []bool{true, false} {
		t.Run(fmt.Sprintf("missing_%t", missing), func(t *testing.T) {
			p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
			p.publishSuccess(t, false)
			ctx := context.Background()
			f := p.f
			ctrl := gomock.NewController(t)
			receipt, err := retryThroughRealTickForTest(t, p.manager, ctx, &entity.RetryInCurrentRunRequest{SpaceID: f.space, ExptID: f.expt, ExpectedRunID: f.run, ItemIDs: []int64{f.item}, Mode: entity.EvaluationModeRetryItems, Session: p.event.Session})
			require.NoError(t, err)
			require.EqualValues(t, 1, receipt.AcceptedCount)
			request := f.source().Current.RequestID
			require.NotEmpty(t, request)
			config := confmocks.NewMockIConfiger(ctrl)
			config.EXPECT().BuildEvalExt(gomock.Any(), f.space, gomock.Any()).Return(nil).AnyTimes()
			config.EXPECT().GetEvalAsyncCtxTTL(gomock.Any(), f.space).Return(12 * time.Hour).AnyTimes()
			metric := metricsmocks.NewMockExptMetric(ctrl)
			metric.EXPECT().EmitItemExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			metric.EXPECT().EmitTurnExecEval(gomock.Any(), gomock.Any()).AnyTimes()
			metric.EXPECT().EmitTurnExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			metric.EXPECT().EmitTurnExecTargetResult(f.space, false).Times(1)
			metric.EXPECT().EmitTurnExecEvaluatorResult(f.space, false).MaxTimes(1)
			benefit := benefitmocks.NewMockIBenefitService(ctrl)
			benefit.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
			manager := svcmocks.NewMockIExptManager(ctrl)
			detail := *buildRetryFailureSingleSetExpt(f.space, f.base+601, f.base+602)
			detail.ID = f.expt
			detail.LatestRunID = f.run
			detail.Status = entity.ExptStatus_Processing
			detail.ExptType = entity.ExptType_Offline
			detail.EvalSet = &entity.EvaluationSet{ID: f.base + 80, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: f.base + 81, EvaluationSetID: f.base + 80}}
			detail.TargetID = f.base + 501
			detail.TargetVersionID = f.base + 502
			detail.TargetType = entity.EvalTargetTypeCustomRPCServer
			detail.Target = &entity.EvalTarget{ID: detail.TargetID, EvalTargetType: entity.EvalTargetTypeCustomRPCServer, EvalTargetVersion: &entity.EvalTargetVersion{ID: detail.TargetVersionID, EvalTargetType: entity.EvalTargetTypeCustomRPCServer, CustomRPCServer: &entity.CustomRPCServer{IsAsync: gptr.Of(true)}}}
			detail.EvalConf.ConnectorConf.TargetConf = &entity.TargetConf{TargetVersionID: detail.TargetVersionID}
			for _, e := range detail.Evaluators {
				version := e.GetEvaluatorVersionID()
				e.EvaluatorType = entity.EvaluatorTypeCustomRPC
				e.PromptEvaluatorVersion = nil
				e.CustomRPCEvaluatorVersion = &entity.CustomRPCEvaluatorVersion{ID: version, IsAsync: true}
			}
			manager.EXPECT().GetRunLog(gomock.Any(), f.expt, f.run, f.space, gomock.Any()).DoAndReturn(func(ctx context.Context, _, _, _ int64, _ *entity.Session) (*entity.ExptRunLog, error) {
				return p.manager.runLogRepo.Get(ctx, f.expt, f.run)
			}).AnyTimes()
			manager.EXPECT().GetDetail(gomock.Any(), f.expt, f.space, gomock.Any()).Return(&detail, nil).AnyTimes()
			dataset := svcmocks.NewMockEvaluationSetItemService(ctrl)
			dataset.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{{ItemID: f.item, ItemVersionID: &f.version, BaseInfo: &entity.BaseInfo{CreatedAt: gptr.Of(time.Now().Unix())}, Turns: []*entity.Turn{{ID: 0}}}}, nil).AnyTimes()
			target := svcmocks.NewMockIEvalTargetService(ctrl)
			targetDAO := targetmysql.NewEvalTargetRecordDAO(f.provider)
			targetID := f.base + 503
			targetCalls := 0
			target.EXPECT().AsyncExecuteTarget(gomock.Any(), f.space, detail.TargetID, detail.TargetVersionID, gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _, _, _ int64, run *entity.ExecuteTargetCtx, input *entity.EvalTargetInputData) (*entity.EvalTargetRecord, string, error) {
				targetCalls++
				record := &entity.EvalTargetRecord{ID: targetID, SpaceID: f.space, TargetID: detail.TargetID, TargetVersionID: detail.TargetVersionID, ExperimentRunID: f.run, ItemID: f.item, ItemVersionID: f.version, TurnID: 0, Status: gptr.Of(entity.EvalTargetRunStatusAsyncInvoking), EvalTargetInputData: input, EvalTargetOutputData: &entity.EvalTargetOutputData{OutputFields: map[string]*entity.Content{}}, BaseInfo: &entity.BaseInfo{CreatedAt: gptr.Of(time.Now().UnixMilli()), UpdatedAt: gptr.Of(time.Now().UnixMilli())}}
				po, err := targetconvert.EvalTargetRecordDO2PO(record)
				require.NoError(t, err)
				require.NoError(t, targetDAO.Save(ctx, po))
				return record, "custom_rpc", nil
			}).Times(1)
			target.EXPECT().GetRecordByID(gomock.Any(), f.space, targetID).DoAndReturn(func(ctx context.Context, _, _ int64) (*entity.EvalTargetRecord, error) {
				po, err := targetDAO.GetByIDAndSpaceID(ctx, targetID, f.space)
				if err != nil {
					return nil, err
				}
				return targetconvert.EvalTargetRecordPO2DO(po)
			}).AnyTimes()
			factory, mini := finishRealLockerFactory(t)
			addr := mini.Addr()
			if configured := os.Getenv("EXPT_RETRY_REAL_REDIS_ADDR"); configured != "" {
				addr = configured
			}
			client, err := infraredis.NewClient(&redis.Options{Addr: addr, ContextTimeoutEnabled: true})
			require.NoError(t, err)
			raw, _ := infraredis.Unwrap(client)
			t.Cleanup(func() { require.NoError(t, raw.Close()) })
			if os.Getenv("EXPT_RETRY_REAL_REDIS_ADDR") != "" {
				factory = func() lock.ILocker { return lock.NewRedisLocker(client) }
			}
			async := exptrepo.NewEvalAsyncRepo(asyncdao.NewEvalAsyncDAO(client, config))
			queue := &legacyConsumerQueue{}
			recordRepo := evaluatorrepo.NewEvaluatorRecordRepo(nil, f.provider, &legacyConsumerRecordDAO{EvaluatorRecordDAO: evaluatormysql.NewEvaluatorRecordDAO(f.provider), session: f.conn}, nil)
			userInfo := userinfomocks.NewMockUserInfoService(ctrl)
			userInfo.EXPECT().PackUserInfo(gomock.Any(), gomock.Any()).AnyTimes()
			records := &EvaluatorRecordServiceImpl{evaluatorRecordRepo: recordRepo, userInfoService: userInfo}
			actual := &EvaluatorServiceImpl{evaluatorRecordRepo: recordRepo, evalAsyncRepo: async, exptEventPublisher: queue}
			a, b := f.base+701, f.base+702
			t.Cleanup(func() {
				require.NoError(t, f.conn.Exec("DELETE FROM eval_target_record WHERE space_id=?", f.space).Error)
				require.NoError(t, f.conn.Exec("DELETE FROM evaluator_record WHERE space_id=?", f.space).Error)
				require.NoError(t, raw.Del(ctx, fmt.Sprintf("[experiment]item_turn_eval_async_ctx:evaluator:%d", a), fmt.Sprintf("[experiment]item_turn_eval_async_ctx:evaluator:%d", b), fmt.Sprintf("[experiment]item_turn_eval_async_ctx:%d", targetID)).Err())
			})
			event := &entity.ExptItemEvalEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, EvalSetItemID: f.item, ExptRunMode: entity.EvaluationModeRetryItems, RequestID: request, MaxRetryTimes: 1, Session: p.event.Session}
			create := func(id, version int64) error {
				row := &entity.EvaluatorRecord{ID: id, SpaceID: f.space, ExperimentID: f.expt, ExperimentRunID: f.run, ItemID: f.item, ItemVersionID: f.version, TurnID: 0, EvaluatorVersionID: version, TargetRecordID: targetID, Status: entity.EvaluatorRunStatusAsyncInvoking, EvaluatorOutputData: &entity.EvaluatorOutputData{}, BaseInfo: &entity.BaseInfo{CreatedAt: gptr.Of(time.Now().UnixMilli()), UpdatedAt: gptr.Of(time.Now().UnixMilli())}}
				if err := recordRepo.CreateEvaluatorRecord(ctx, row); err != nil {
					return err
				}
				return async.SetEvalAsyncCtx(ctx, fmt.Sprintf("evaluator:%d", id), &entity.EvalAsyncCtx{Event: event.Clone(), RecordID: id, EvaluatorVersionID: version, ResumeReady: false})
			}
			report := func(id int64) error {
				if driver := os.Getenv("FINAL_NATIVE_E6_DRIVER"); driver != "" {
					actx, err := async.GetEvalAsyncCtxStrong(ctx, fmt.Sprintf("evaluator:%d", id))
					if err != nil {
						return err
					}
					file := filepath.Join(t.TempDir(), "old-report.json")
					cmd := exec.Command(driver, fmt.Sprint(f.space), fmt.Sprint(id), file, "evaluator")
					cmd.Env = os.Environ()
					output, err := cmd.CombinedOutput()
					t.Logf("NATIVE_E6_EVALUATOR_REPORT_RAW %s", output)
					if !actx.ResumeReady {
						require.Contains(t, string(output), "retry_before_refs=true")
					}
					return err
				}
				_, err := actual.ReportEvaluatorInvokeResult(ctx, &entity.ReportEvaluatorRecordParam{SpaceID: f.space, RecordID: id, Status: entity.EvaluatorRunStatusSuccess, OutputData: &entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: gptr.Of(float64(1))}}})
				return err
			}

			evaluator := svcmocks.NewMockEvaluatorService(ctrl)
			calls := 0
			evaluator.EXPECT().ShouldInterceptEvaluator(gomock.Any(), gomock.Any()).Return(nil, false, nil).MaxTimes(1)
			evaluator.EXPECT().AsyncRunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req *entity.AsyncRunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
				calls++
				require.Equal(t, f.base+602, req.EvaluatorVersionID)
				require.NoError(t, create(b, req.EvaluatorVersionID))
				return recordRepo.GetEvaluatorRecord(ctx, b, false)
			}).MaxTimes(1)
			evaluator.EXPECT().ArmEvaluatorResume(gomock.Any(), gomock.Any()).DoAndReturn(actual.ArmEvaluatorResume).AnyTimes()
			barrier := &legacyConsumerRefsBarrier{IExptItemResultRepo: p.manager.itemResultRepo, IExptCurrentResultRepo: p.manager.itemResultRepo.(repo.IExptCurrentResultRepo)}
			early := false
			barrier.before = func(context.Context) error {
				if !missing {
					return nil
				}
				early = true
				require.NotContains(t, legacyEvaluatorRecordIDs(f.source().Turns[0].EvaluatorResultIds), b, "callback must precede B reference persistence")
				require.NoError(t, report(b))
				actx, err := async.GetEvalAsyncCtxStrong(ctx, fmt.Sprintf("evaluator:%d", b))
				require.NoError(t, err)
				require.False(t, actx.ResumeReady)
				return nil
			}
			svc := NewExptRecordEvalService(manager, config, queue, barrier, p.manager.turnResultRepo, p.manager.statsRepo, p.manager.exptRepo, p.manager.itemRefRepo, nil, p.manager.mutex, p.manager.idem, nil, metric, p.result, target, dataset, records, evaluator, p.manager.idgenerator, benefit, async, nil, nil, component.NewNoopCentralReservationGuard(), nil, component.NewNoopCentralSchedulerScopeOwner()).(*ExptItemEventEvalServiceImpl)
			svc.itemLocker = factory
			outer := &legacyAsyncTestConsumer{ExptItemEvalEvent: svc}
			consume := func(event *entity.ExptItemEvalEvent, old bool) error {
				body, err := json.Marshal(event)
				if err != nil {
					return err
				}
				if old {
					codec := &normalE6Event{}
					if err = json.Unmarshal(body, codec); err != nil {
						return err
					}
					body, err = json.Marshal(codec)
					if err != nil {
						return err
					}
				}
				decoded := &entity.ExptItemEvalEvent{}
				if err = json.Unmarshal(body, decoded); err != nil {
					return err
				}
				if old {
					require.Empty(t, decoded.RequestID)
					require.Nil(t, decoded.ResumeSource)
				}
				return outer.HandleMessage(ctx, &mq.MessageExt{Message: mq.Message{Body: body}, MsgID: "legacy-async-consumer"})
			}
			require.NoError(t, consume(event, false))
			require.Equal(t, 1, targetCalls)
			require.Zero(t, calls)
			if driver := os.Getenv("FINAL_NATIVE_E6_DRIVER"); driver != "" {
				file := filepath.Join(t.TempDir(), "old-target-report.json")
				cmd := exec.Command(driver, fmt.Sprint(f.space), fmt.Sprint(targetID), file)
				cmd.Env = os.Environ()
				output, err := cmd.CombinedOutput()
				t.Logf("NATIVE_E6_TARGET_REPORT_RAW %s", output)
				require.NoError(t, err)
			} else {
				po, err := targetDAO.GetByIDAndSpaceID(ctx, targetID, f.space)
				require.NoError(t, err)
				po.Status = int32(entity.EvalTargetRunStatusSuccess)
				require.NoError(t, targetDAO.Save(ctx, po))
			}

			require.NoError(t, create(a, f.base+601))
			require.NoError(t, report(a))
			source := f.source()
			turn := *source.Turns[0]
			turn.EvaluatorResultIds = &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{f.base + 601: a}}
			if !missing {
				require.NoError(t, create(b, f.base+602))
				turn.EvaluatorResultIds.EvalVerIDToResID[f.base+602] = b
			}
			applied, err := f.port.StoreCurrentTurnRunResult(ctx, source, &turn)
			require.NoError(t, err)
			require.True(t, applied)
			require.NoError(t, actual.ArmEvaluatorResume(ctx, a))
			notice := &entity.ExptItemEvalEvent{}
			require.NoError(t, json.Unmarshal(queue.last(t), notice))
			require.True(t, notice.AsyncEvaluatorReportTrigger)
			require.NoError(t, consume(notice, true))
			require.Equal(t, entity.ItemRunState_Processing, f.source().Current.Status, "an async execution result must not finish the item before its resume")
			require.Equal(t, request, f.source().Current.RequestID)
			require.Equal(t, 1, targetCalls)
			if missing {
				require.True(t, early)
				require.Equal(t, 1, calls)
			} else {
				require.Zero(t, calls)
				require.NoError(t, report(b))
				require.NoError(t, actual.ArmEvaluatorResume(ctx, b))
			}
			actx, err := async.GetEvalAsyncCtxStrong(ctx, fmt.Sprintf("evaluator:%d", b))
			require.NoError(t, err)
			require.True(t, actx.ResumeReady)
			if driver := os.Getenv("SINGLE_MR_NATIVE_REPORT_DRIVER"); driver != "" {
				file := filepath.Join(t.TempDir(), "old-arm.json")
				cmd := exec.Command(driver, fmt.Sprint(f.space), fmt.Sprint(b), file, "arm")
				cmd.Env = os.Environ()
				output, err := cmd.CombinedOutput()
				t.Logf("NATIVE_E6_ARM_RAW %s", output)
				require.NoError(t, err)
				body, err := os.ReadFile(file)
				require.NoError(t, err)
				notice = &entity.ExptItemEvalEvent{}
				require.NoError(t, json.Unmarshal(body, notice))
				require.Empty(t, notice.RequestID)
				require.Nil(t, notice.ResumeSource)
			} else {
				require.NoError(t, json.Unmarshal(queue.last(t), notice))
			}
			require.NoError(t, consume(notice, true))
			done := f.source()
			require.Equal(t, entity.ItemRunState_Success, done.Current.Status)
			require.Equal(t, entity.TurnRunState_Success, done.Turns[0].Status)
			require.Equal(t, request, done.Current.RequestID)
			require.ElementsMatch(t, []int64{a, b}, legacyEvaluatorRecordIDs(done.Turns[0].EvaluatorResultIds))
			before := f.snapshot()
			require.NoError(t, consume(notice, true))
			require.Equal(t, before, f.snapshot())
			require.Equal(t, 1, targetCalls)
			expected := 0
			if missing {
				expected = 1
			}
			require.Equal(t, expected, calls)
			t.Logf("LEGACY_PARTIAL_CONSUMER_ARM missing=%t request=%s target_calls=%d missing_evaluator_calls=%d early_callback=%t", missing, request, targetCalls, calls, early)
		})
	}
}
