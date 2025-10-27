// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"
	"github.com/jinzhu/copier"

	"github.com/coze-dev/coze-loop/backend/infra/external/audit"
	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/ctxcache"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/goroutine"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ExptItemEventEvalServiceImpl struct {
	endpoints                RecordEvalEndPoint
	manager                  IExptManager
	publisher                events.ExptEventPublisher
	exptItemResultRepo       repo.IExptItemResultRepo
	exptTurnResultRepo       repo.IExptTurnResultRepo
	exptStatsRepo            repo.IExptStatsRepo
	experimentRepo           repo.IExperimentRepo
	configer                 component.IConfiger
	quotaRepo                repo.QuotaRepo
	mutex                    lock.ILocker
	idem                     idem.IdempotentService
	auditClient              audit.IAuditService
	metric                   metrics.ExptMetric
	resultSvc                ExptResultService
	evaluationSetItemService EvaluationSetItemService
	evaluatorService         EvaluatorService
	evaTargetService         IEvalTargetService
	evaluatorRecordService   EvaluatorRecordService
	idgen                    idgen.IIDGenerator
	benefitService           benefit.IBenefitService
}

func NewExptRecordEvalService(
	manager IExptManager,
	configer component.IConfiger,
	publisher events.ExptEventPublisher,
	exptItemResultRepo repo.IExptItemResultRepo,
	exptTurnResultRepo repo.IExptTurnResultRepo,
	exptStatsRepo repo.IExptStatsRepo,
	experimentRepo repo.IExperimentRepo,
	quotaRepo repo.QuotaRepo,
	mutex lock.ILocker,
	idem idem.IdempotentService,
	auditClient audit.IAuditService,
	metric metrics.ExptMetric,
	resultSvc ExptResultService,
	evaTargetService IEvalTargetService,
	evaluationSetItemService EvaluationSetItemService,
	evaluatorRecordService EvaluatorRecordService,
	evaluatorService EvaluatorService,
	idgen idgen.IIDGenerator,
	benefitService benefit.IBenefitService,
) ExptItemEvalEvent {
	i := &ExptItemEventEvalServiceImpl{
		manager:                  manager,
		publisher:                publisher,
		exptItemResultRepo:       exptItemResultRepo,
		exptTurnResultRepo:       exptTurnResultRepo,
		exptStatsRepo:            exptStatsRepo,
		experimentRepo:           experimentRepo,
		configer:                 configer,
		quotaRepo:                quotaRepo,
		mutex:                    mutex,
		idem:                     idem,
		auditClient:              auditClient,
		metric:                   metric,
		resultSvc:                resultSvc,
		evaTargetService:         evaTargetService,
		evaluationSetItemService: evaluationSetItemService,
		evaluatorRecordService:   evaluatorRecordService,
		evaluatorService:         evaluatorService,
		idgen:                    idgen,
		benefitService:           benefitService,
	}

	i.endpoints = RecordEvalChain(
		i.HandleEventErr,
		i.HandleEventCheck,
		i.HandleEventLock,
		i.HandleEventExec,
	)(func(_ context.Context, _ *entity.ExptItemEvalEvent) error { return nil })

	return i
}

func (e *ExptItemEventEvalServiceImpl) Eval(ctx context.Context, event *entity.ExptItemEvalEvent) error {
	ctx = ctxcache.Init(ctx)

	if err := e.endpoints(ctx, event); err != nil {
		logs.CtxError(ctx, "[ExptRecordEval] expt record eval fail, event: %v, err: %v", json.Jsonify(event), err)
		return err
	}

	return nil
}

type RecordEvalEndPoint func(ctx context.Context, event *entity.ExptItemEvalEvent) error

type RecordEvalMiddleware func(next RecordEvalEndPoint) RecordEvalEndPoint

func RecordEvalChain(mws ...RecordEvalMiddleware) RecordEvalMiddleware {
	return func(next RecordEvalEndPoint) RecordEvalEndPoint {
		for i := len(mws) - 1; i >= 0; i-- {
			next = mws[i](next)
		}
		return next
	}
}

func (e *ExptItemEventEvalServiceImpl) HandleEventCheck(next RecordEvalEndPoint) RecordEvalEndPoint {
	return func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		runLog, err := e.manager.GetRunLog(ctx, event.ExptID, event.ExptRunID, event.SpaceID, event.Session)
		if err != nil {
			return err
		}

		if entity.IsExptFinished(entity.ExptStatus(runLog.Status)) {
			logs.CtxInfo(ctx, "ExptRecordEvalConsumer consume finished expt run event, expt_id: %v, expt_run_id: %v", event.ExptID, event.ExptRunID)
			return nil
		}

		return next(ctx, event)
	}
}

func (e *ExptItemEventEvalServiceImpl) HandleEventErr(next RecordEvalEndPoint) RecordEvalEndPoint {
	return func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		nextErr := func(ctx context.Context, event *entity.ExptItemEvalEvent) (err error) {
			defer goroutine.Recover(ctx, &err)
			return next(ctx, event)
		}(ctx, event)

		retryConf := e.configer.GetErrRetryConf(ctx, event.SpaceID, nextErr)
		needRetry := event.RetryTimes < retryConf.GetRetryTimes()

		defer func() {
			code, stable, _ := errno.ParseStatusError(nextErr)
			e.metric.EmitItemExecResult(event.SpaceID, int64(event.ExptRunMode), nextErr != nil, needRetry, stable, int64(code), event.CreateAt)
		}()

		logs.CtxInfo(ctx, "[ExptRecordEval] handle event done, success: %v, retry: %v, retry_times: %v, err: %v, indebt: %v, event: %v",
			nextErr == nil, needRetry, retryConf.GetRetryTimes(), nextErr, retryConf.IsInDebt, json.Jsonify(event))

		if nextErr == nil {
			return nil
		}

		if retryConf.IsInDebt {
			completeCID := fmt.Sprintf("terminate:indebt:%d", event.ExptRunID)

			if err := e.manager.CompleteRun(ctx, event.ExptID, event.ExptRunID, event.ExptRunMode, event.SpaceID, event.Session, entity.WithCID(completeCID)); err != nil {
				return errorx.Wrapf(err, "terminate expt run fail, expt_id: %v", event.ExptID)
			}

			if err := e.manager.CompleteExpt(ctx, event.ExptID, event.SpaceID, event.Session, entity.WithStatus(entity.ExptStatus_Terminated),
				entity.WithStatusMessage(nextErr.Error()), entity.WithCID(completeCID)); err != nil {
				return errorx.Wrapf(err, "complete expt fail, expt_id: %v, expt_run_id: %v", event.ExptID, event.ExptRunID)
			}

			return nil
		}

		if needRetry {
			clone := &entity.ExptItemEvalEvent{}
			if err := copier.CopyWithOption(clone, event, copier.Option{DeepCopy: true}); err != nil {
				return errorx.Wrapf(err, "ExptItemEvalEvent copy fail")
			}

			clone.RetryTimes += 1

			return e.publisher.PublishExptRecordEvalEvent(ctx, clone, gptr.Of(retryConf.GetRetryInterval()))
		}

		return nil
	}
}

func (e *ExptItemEventEvalServiceImpl) HandleEventLock(next RecordEvalEndPoint) RecordEvalEndPoint {
	return func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		lockKey := fmt.Sprintf("expt_item_eval_run_lock:%d:%d", event.ExptID, event.EvalSetItemID)
		locked, ctx, unlock, err := e.mutex.LockWithRenew(ctx, lockKey, time.Second*20, time.Second*60*30)
		if err != nil {
			return err
		}

		if !locked {
			logs.CtxWarn(ctx, "ExptRecordEvalConsumer.HandleEventLock found locked item eval event: %v. Abort event, err: %v", json.Jsonify(event), err)
			return nil
		}

		defer unlock()

		return next(ctx, event)
	}
}

func (e *ExptItemEventEvalServiceImpl) HandleEventExec(next RecordEvalEndPoint) RecordEvalEndPoint {
	return func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		if err := e.eval(ctx, event); err != nil {
			return err
		}
		return next(ctx, event)
	}
}

func (e *ExptItemEventEvalServiceImpl) eval(ctx context.Context, event *entity.ExptItemEvalEvent) error {
	eiec, err := e.BuildExptRecordEvalCtx(ctx, event)
	if err != nil {
		return err
	}

	ctx = e.WithCtx(ctx, eiec)

	mode, err := NewRecordEvalMode(
		eiec.Event,
		e.exptItemResultRepo,
		e.exptTurnResultRepo,
		e.exptStatsRepo,
		e.experimentRepo,
		e.metric,
		e.resultSvc,
		e.idgen,
	)
	if err != nil {
		return err
	}

	if err := mode.PreEval(ctx, eiec); err != nil {
		return err
	}

	if err := NewExptItemEvaluation(e.exptTurnResultRepo, e.exptItemResultRepo, e.configer, e.metric, e.evaTargetService, e.evaluatorRecordService, e.evaluatorService, e.benefitService).
		Eval(ctx, eiec); err != nil {
		return err
	}

	if err := mode.PostEval(ctx, eiec); err != nil {
		return err
	}

	return nil
}

func (e *ExptItemEventEvalServiceImpl) WithCtx(ctx context.Context, eiec *entity.ExptItemEvalCtx) context.Context {
	return logs.SetLogID(ctx, eiec.GetRecordEvalLogID(ctx))
}

func (e *ExptItemEventEvalServiceImpl) BuildExptRecordEvalCtx(ctx context.Context, event *entity.ExptItemEvalEvent) (*entity.ExptItemEvalCtx, error) {
	exptDetail, err := e.manager.GetDetail(ctx, event.ExptID, event.SpaceID, event.Session)
	if err != nil {
		return nil, err
	}

	evalSetID := exptDetail.EvalSet.EvaluationSetVersion.EvaluationSetID
	evalSetVerID := exptDetail.EvalSet.EvaluationSetVersion.ID

	batchGetEvaluationSetItemsParam := &entity.BatchGetEvaluationSetItemsParam{
		SpaceID:         event.SpaceID,
		EvaluationSetID: evalSetID,
		VersionID:       gptr.Of(evalSetVerID),
		ItemIDs:         []int64{event.EvalSetItemID},
	}
	if evalSetID == evalSetVerID {
		batchGetEvaluationSetItemsParam.VersionID = nil
	}
	items, err := e.evaluationSetItemService.BatchGetEvaluationSetItems(ctx, batchGetEvaluationSetItemsParam)
	if err != nil {
		return nil, err
	}

	if len(items) != 1 {
		return nil, fmt.Errorf("BatchGetEvaluationSetItems with invalid item result, eval_set_id: %v, eval_set_ver_id: %v, item_id: %v, got items len: %v", evalSetID, evalSetVerID, event.EvalSetItemID, len(items))
	}

	existResult, err := e.GetExistExptRecordEvalResult(ctx, event)
	if err != nil {
		return nil, err
	}

	return &entity.ExptItemEvalCtx{
		Event:               event,
		Expt:                exptDetail,
		EvalSetItem:         items[0],
		ExistItemEvalResult: existResult,
	}, nil
}

func (e *ExptItemEventEvalServiceImpl) GetExistExptRecordEvalResult(ctx context.Context, event *entity.ExptItemEvalEvent) (*entity.ExptItemEvalResult, error) {
	turnRunLogs, err := e.exptTurnResultRepo.GetItemTurnRunLogs(ctx, event.ExptID, event.ExptRunID, event.EvalSetItemID, event.SpaceID)
	if err != nil {
		return nil, err
	}

	turnRunResultMap := make(map[int64]*entity.ExptTurnResultRunLog, len(turnRunLogs))
	for _, result := range turnRunLogs {
		turnRunResultMap[result.ItemID] = result
	}

	itemRunLog, err := e.exptItemResultRepo.GetItemRunLog(ctx, event.ExptID, event.ExptRunID, event.EvalSetItemID, event.SpaceID)
	if err != nil {
		return nil, err
	}

	return &entity.ExptItemEvalResult{
		ItemResultRunLog:  itemRunLog,
		TurnResultRunLogs: turnRunResultMap,
	}, nil
}

// RecordEvalMode 任务执行模式
type RecordEvalMode interface {
	PreEval(ctx context.Context, eiec *entity.ExptItemEvalCtx) error
	PostEval(ctx context.Context, eiec *entity.ExptItemEvalCtx) error
}

func NewRecordEvalMode(
	event *entity.ExptItemEvalEvent, exptItemResultRepo repo.IExptItemResultRepo,
	exptTurnResultRepo repo.IExptTurnResultRepo,
	exptStatsRepo repo.IExptStatsRepo,
	experimentRepo repo.IExperimentRepo,
	metric metrics.ExptMetric,
	resultSvc ExptResultService,
	idgen idgen.IIDGenerator,
) (RecordEvalMode, error) {
	switch event.ExptRunMode {
	case entity.EvaluationModeSubmit, entity.EvaluationModeAppend:
		return &ExptRecordEvalModeSubmit{
			exptItemResultRepo: exptItemResultRepo,
			exptTurnResultRepo: exptTurnResultRepo,
			exptRepo:           experimentRepo,
			idgen:              idgen,
		}, nil
	case entity.EvaluationModeFailRetry:
		return &ExptRecordEvalModeFailRetry{
			exptItemResultRepo: exptItemResultRepo,
			exptTurnResultRepo: exptTurnResultRepo,
			exptStatsRepo:      exptStatsRepo,
			experimentRepo:     experimentRepo,
			metric:             metric,
			resultSvc:          resultSvc,
			idgen:              idgen,
		}, nil
	default:
		return nil, fmt.Errorf("NewRecordEvalMode with unknown expt mode: %v", event.ExptRunMode)
	}
}

type ExptRecordEvalModeSubmit struct {
	exptItemResultRepo repo.IExptItemResultRepo
	exptTurnResultRepo repo.IExptTurnResultRepo
	exptRepo           repo.IExperimentRepo
	idgen              idgen.IIDGenerator
}

func (e *ExptRecordEvalModeSubmit) PreEval(ctx context.Context, eiec *entity.ExptItemEvalCtx) error {
	event := eiec.Event
	turns := eiec.EvalSetItem.Turns

	// if err := e.exptItemResultRepo.UpdateItemRunLog(ctx, event.ExptID, event.ExptRunID, []int64{event.EvalSetItemID}, map[string]any{"status": int32(entity.ItemRunState_Processing)},
	//	event.SpaceID); err != nil {
	//	return err
	// }

	absentRunLogTurnIDs := make([]int64, 0, len(turns))
	for _, turn := range turns {
		if turn == nil {
			continue
		}
		if eiec.GetExistTurnResultRunLog(turn.ID) == nil {
			absentRunLogTurnIDs = append(absentRunLogTurnIDs, turn.ID)
		}
	}

	if len(absentRunLogTurnIDs) > 0 {
		ids, err := e.idgen.GenMultiIDs(ctx, len(absentRunLogTurnIDs))
		if err != nil {
			return err
		}

		logID := logs.GetLogID(ctx)

		turnRunResults := make([]*entity.ExptTurnResultRunLog, 0, len(absentRunLogTurnIDs))
		for idx, turnID := range absentRunLogTurnIDs {
			turnRunResults = append(turnRunResults, &entity.ExptTurnResultRunLog{
				ID:        ids[idx],
				SpaceID:   event.SpaceID,
				ExptID:    event.ExptID,
				ExptRunID: event.ExptRunID,
				ItemID:    event.EvalSetItemID,
				TurnID:    turnID,
				Status:    entity.TurnRunState_Processing,
				LogID:     logID,
			})
		}

		if err := e.exptTurnResultRepo.BatchCreateNXRunLog(ctx, turnRunResults); err != nil {
			return err
		}

		// turnRunLogDOs := make([]*entity.ExptTurnResultRunLog, 0, len(turnRunResults))
		// for _, trr := range turnRunResults {
		//	_, err := convert2.NewExptTurnResultRunLogConvertor().ConvertModelToEntity(trr)
		//	if err != nil {
		//		return err
		//	}
		//	turnRunLogDOs = append(turnRunLogDOs, nil)
		// }
		//
		// eiec.ExistItemEvalResult.TurnResultRunLogs = gslice.ToMap(turnRunLogDOs, func(t *entity.ExptTurnResultRunLog) (int64, *entity.ExptTurnResultRunLog) {
		//	return t.TurnID, t
		// })

		eiec.ExistItemEvalResult.TurnResultRunLogs = gslice.ToMap(turnRunResults, func(t *entity.ExptTurnResultRunLog) (int64, *entity.ExptTurnResultRunLog) {
			return t.TurnID, t
		})
	}

	return nil
}

func (e *ExptRecordEvalModeSubmit) PostEval(ctx context.Context, eiec *entity.ExptItemEvalCtx) error {
	return nil
}

type ExptRecordEvalModeFailRetry struct {
	resultSvc          ExptResultService
	exptItemResultRepo repo.IExptItemResultRepo
	exptTurnResultRepo repo.IExptTurnResultRepo
	exptStatsRepo      repo.IExptStatsRepo
	experimentRepo     repo.IExperimentRepo
	metric             metrics.ExptMetric
	idgen              idgen.IIDGenerator
}

func (e *ExptRecordEvalModeFailRetry) PreEval(ctx context.Context, eiec *entity.ExptItemEvalCtx) error {
	itemTurnResults, err := e.resultSvc.GetExptItemTurnResults(ctx, eiec.Event.ExptID, eiec.Event.EvalSetItemID, eiec.Event.SpaceID, eiec.Event.Session)
	if err != nil {
		return err
	}

	ids, err := e.idgen.GenMultiIDs(ctx, len(itemTurnResults))
	if err != nil {
		return err
	}

	turnRunLogDOs := make([]*entity.ExptTurnResultRunLog, 0, len(itemTurnResults))
	for idx, tr := range itemTurnResults {
		runLog := tr.ToRunLogDO()
		runLog.ID = ids[idx]
		runLog.Status = entity.TurnRunState_Processing
		runLog.ExptRunID = eiec.Event.ExptRunID
		turnRunLogDOs = append(turnRunLogDOs, runLog)
	}

	if err := e.exptTurnResultRepo.BatchCreateNXRunLog(ctx, turnRunLogDOs); err != nil {
		return err
	}

	eiec.ExistItemEvalResult.TurnResultRunLogs = gslice.ToMap(turnRunLogDOs, func(t *entity.ExptTurnResultRunLog) (int64, *entity.ExptTurnResultRunLog) {
		return t.TurnID, t
	})

	return nil
}

func (e *ExptRecordEvalModeFailRetry) PostEval(ctx context.Context, eiec *entity.ExptItemEvalCtx) error {
	return nil
}
