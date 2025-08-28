// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/conv"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/maps"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// SchedulerModeFactory 定义创建 ExptSchedulerMode 实例的接口
type SchedulerModeFactory interface {
	NewSchedulerMode(
		mode entity.ExptRunMode,
	) (entity.ExptSchedulerMode, error)
}

func NewSchedulerModeFactory(
	manager IExptManager,
	exptItemResultRepo repo.IExptItemResultRepo,
	exptStatsRepo repo.IExptStatsRepo,
	exptTurnResultRepo repo.IExptTurnResultRepo,
	idgenerator idgen.IIDGenerator,
	evaluationSetItemService EvaluationSetItemService,
	exptRepo repo.IExperimentRepo,
	idem idem.IdempotentService,
	configer component.IConfiger,
	publisher events.ExptEventPublisher,
	evaluatorRecordService EvaluatorRecordService,
	resultSvc ExptResultService,
) SchedulerModeFactory {
	return &DefaultSchedulerModeFactory{
		manager:                  manager,
		exptItemResultRepo:       exptItemResultRepo,
		exptStatsRepo:            exptStatsRepo,
		exptTurnResultRepo:       exptTurnResultRepo,
		idgenerator:              idgenerator,
		evaluationSetItemService: evaluationSetItemService,
		exptRepo:                 exptRepo,
		idem:                     idem,
		configer:                 configer,
		publisher:                publisher,
		evaluatorRecordService:   evaluatorRecordService,
		resultSvc:                resultSvc,
	}
}

// DefaultSchedulerModeFactory 实现 SchedulerModeFactory 接口，使用实际的 NewSchedulerMode 函数
type DefaultSchedulerModeFactory struct {
	manager                  IExptManager
	exptItemResultRepo       repo.IExptItemResultRepo
	exptStatsRepo            repo.IExptStatsRepo
	exptTurnResultRepo       repo.IExptTurnResultRepo
	idgenerator              idgen.IIDGenerator
	evaluationSetItemService EvaluationSetItemService
	exptRepo                 repo.IExperimentRepo
	idem                     idem.IdempotentService
	configer                 component.IConfiger
	publisher                events.ExptEventPublisher
	evaluatorRecordService   EvaluatorRecordService
	resultSvc                ExptResultService
}

func (f *DefaultSchedulerModeFactory) NewSchedulerMode(
	mode entity.ExptRunMode,
) (entity.ExptSchedulerMode, error) {
	switch mode {
	case entity.EvaluationModeSubmit:
		return NewExptSubmitMode(f.manager, f.exptItemResultRepo, f.exptStatsRepo, f.exptTurnResultRepo, f.idgenerator, f.evaluationSetItemService, f.exptRepo, f.idem, f.configer, f.publisher, f.evaluatorRecordService, f.resultSvc), nil
	case entity.EvaluationModeFailRetry:
		return NewExptFailRetryMode(f.manager, f.exptItemResultRepo, f.exptStatsRepo, f.exptTurnResultRepo, f.idgenerator, f.exptRepo, f.idem, f.configer, f.publisher, f.evaluatorRecordService), nil
	case entity.EvaluationModeAppend:
		return NewExptAppendMode(f.manager, f.exptItemResultRepo, f.exptStatsRepo, f.exptTurnResultRepo, f.idgenerator, f.evaluationSetItemService, f.exptRepo, f.idem, f.configer, f.publisher, f.evaluatorRecordService), nil
	default:
		return nil, fmt.Errorf("NewSchedulerMode with unknown mode: %v", mode)
	}
}

type ExptSubmitExec struct {
	manager                  IExptManager
	exptStatsRepo            repo.IExptStatsRepo
	exptItemResultRepo       repo.IExptItemResultRepo
	exptTurnResultRepo       repo.IExptTurnResultRepo
	idgenerator              idgen.IIDGenerator
	evaluationSetItemService EvaluationSetItemService
	exptRepo                 repo.IExperimentRepo
	idem                     idem.IdempotentService
	configer                 component.IConfiger
	publisher                events.ExptEventPublisher
	evaluatorRecordService   EvaluatorRecordService
	resultSvc                ExptResultService
}

func NewExptSubmitMode(
	manager IExptManager,
	exptItemResultRepo repo.IExptItemResultRepo,
	exptStatsRepo repo.IExptStatsRepo,
	exptTurnResultRepo repo.IExptTurnResultRepo,
	idgenerator idgen.IIDGenerator,
	evaluationSetItemService EvaluationSetItemService,
	exptRepo repo.IExperimentRepo,
	idem idem.IdempotentService,
	configer component.IConfiger,
	publisher events.ExptEventPublisher,
	evaluatorRecordService EvaluatorRecordService,
	resultSvc ExptResultService,
) *ExptSubmitExec {
	return &ExptSubmitExec{
		manager:                  manager,
		exptItemResultRepo:       exptItemResultRepo,
		exptStatsRepo:            exptStatsRepo,
		exptTurnResultRepo:       exptTurnResultRepo,
		idgenerator:              idgenerator,
		evaluationSetItemService: evaluationSetItemService,
		exptRepo:                 exptRepo,
		idem:                     idem,
		configer:                 configer,
		publisher:                publisher,
		evaluatorRecordService:   evaluatorRecordService,
		resultSvc:                resultSvc,
	}
}

func (e *ExptSubmitExec) Mode() entity.ExptRunMode {
	return entity.EvaluationModeSubmit
}

func (e *ExptSubmitExec) ExptStart(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment) error {
	idemKey := makeStartIdemKey(event)

	exist, err := e.idem.Exist(ctx, idemKey)
	if err != nil {
		return err
	}

	if exist {
		return nil
	}

	var (
		evalSetID        = expt.EvalSet.ID
		evalSetVersionID = expt.EvalSet.EvaluationSetVersion.ID

		maxLoop = 10000
		itemIdx = int32(0)

		page     = int32(1)
		pageSize = int32(100)
		itemCnt  = 0
		total    = int64(0)
	)

	for i := 0; i < maxLoop; i++ {
		logs.CtxInfo(ctx, "ExptSubmitExec.ExptStart scan item, expt_id: %v, expt_run_id: %v, eval_set_id: %v, eval_set_ver_id: %v, page: %v, limit: %v, cur_cnt: %v, total: %v",
			event.ExptID, event.ExptRunID, evalSetID, evalSetVersionID, page, pageSize, itemCnt, total)

		items, t, _, err := e.evaluationSetItemService.ListEvaluationSetItems(ctx, &entity.ListEvaluationSetItemsParam{
			SpaceID:         event.SpaceID,
			EvaluationSetID: evalSetID,
			VersionID:       &evalSetVersionID,
			PageNumber:      &page,
			PageSize:        &pageSize,
		})
		if err != nil {
			return err
		}

		itemCnt += len(items)
		page++
		total = gptr.Indirect(t)

		turnCnt := 0
		for _, item := range items {
			turnCnt += len(item.Turns)
		}

		ids, err := e.idgenerator.GenMultiIDs(ctx, len(items)+turnCnt)
		if err != nil {
			return err
		}

		idIdx := 0
		eirs := make([]*entity.ExptItemResult, 0, len(items))
		etrs := make([]*entity.ExptTurnResult, 0, len(items))
		for _, item := range items {
			eir := &entity.ExptItemResult{
				ID:        ids[idIdx],
				SpaceID:   event.SpaceID,
				ExptID:    event.ExptID,
				ExptRunID: event.ExptRunID,
				ItemID:    item.ItemID,
				ItemIdx:   itemIdx,
				Status:    entity.ItemRunState_Queueing,
			}
			eirs = append(eirs, eir)
			itemIdx++
			idIdx++

			for turnIdx, turn := range item.Turns {
				etr := &entity.ExptTurnResult{
					ID:        ids[idIdx],
					SpaceID:   event.SpaceID,
					ExptID:    event.ExptID,
					ExptRunID: event.ExptRunID,
					ItemID:    item.ItemID,
					TurnID:    turn.ID,
					TurnIdx:   int32(turnIdx),
					Status:    int32(entity.TurnRunState_Queueing),
				}
				etrs = append(etrs, etr)
				idIdx++
			}
		}

		if err := e.createItemTurnResults(ctx, eirs, etrs, event.Session); err != nil {
			return err
		}

		if itemCnt >= int(total) || len(items) == 0 {
			break
		}

		time.Sleep(time.Millisecond * 30)
	}
	err = e.resultSvc.UpsertExptTurnResultFilter(ctx, event.SpaceID, event.ExptID, nil)
	if err != nil {
		logs.CtxError(ctx, "ExptSubmitExec.ExptStart UpsertExptTurnResultFilter fail, expt_id: %v, err: %v", event.ExptID, err)
	}
	logs.CtxInfo(ctx, "ExptSubmitExec ExptStart UpsertExptTurnResultFilter done, expt_id: %v, err: %v", event.ExptID, err)
	if err := e.exptStatsRepo.UpdateByExptID(ctx, event.ExptID, event.SpaceID,
		&entity.ExptStats{
			ExptID:         event.ExptID,
			SpaceID:        event.SpaceID,
			PendingItemCnt: int32(itemCnt),
		}); err != nil {
		return err
	}

	exptDo := &entity.Experiment{
		Status:  entity.ExptStatus_Processing,
		ID:      event.ExptID,
		SpaceID: event.SpaceID,
	}

	if err := e.exptRepo.Update(ctx, exptDo); err != nil {
		return err
	}

	duration := time.Duration(e.configer.GetExptExecConf(ctx, event.SpaceID).GetZombieIntervalSecond()) * time.Second * 2
	if err := e.idem.Set(ctx, idemKey, duration); err != nil {
		return err
	}

	time.Sleep(time.Second * 3)

	return nil
}

func (e *ExptSubmitExec) createItemTurnResults(ctx context.Context, eirs []*entity.ExptItemResult, etrs []*entity.ExptTurnResult, session *entity.Session) error {
	if err := e.exptTurnResultRepo.BatchCreateNX(ctx, etrs); err != nil {
		return err
	}

	if err := e.exptItemResultRepo.BatchCreateNX(ctx, eirs); err != nil {
		return err
	}

	ids, err := e.idgenerator.GenMultiIDs(ctx, len(eirs))
	if err != nil {
		return err
	}

	eirLogs := make([]*entity.ExptItemResultRunLog, 0, len(eirs))
	for idx, eir := range eirs {
		eirLog := &entity.ExptItemResultRunLog{
			ID:        ids[idx],
			SpaceID:   eir.SpaceID,
			ExptID:    eir.ExptID,
			ExptRunID: eir.ExptRunID,
			ItemID:    eir.ItemID,
			Status:    int32(eir.Status),
			ErrMsg:    conv.UnsafeStringToBytes(eir.ErrMsg),
			LogID:     eir.LogID,
		}
		eirLogs = append(eirLogs, eirLog)
	}

	if err := e.exptItemResultRepo.BatchCreateNXRunLogs(ctx, eirLogs); err != nil {
		return err
	}

	return nil
}

func (e *ExptSubmitExec) ScanEvalItems(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment) (toSubmit, incomplete, complete []*entity.ExptEvalItem, err error) {
	return newExptBaseExec(e.manager, e.idem, e.configer, e.exptItemResultRepo, e.publisher, e.evaluatorRecordService).ScanEvalItems(ctx, event, expt)
}

func (e *ExptSubmitExec) ExptEnd(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment, toSubmit, incomplete int) (nextTick bool, err error) {
	if toSubmit == 0 && incomplete == 0 {
		logs.CtxInfo(ctx, "[ExptEval] expt daemon finished, expt_id: %v, expt_run_id: %v", event.ExptID, event.ExptRunID)
		return false, newExptBaseExec(e.manager, e.idem, e.configer, e.exptItemResultRepo, e.publisher, e.evaluatorRecordService).exptEnd(ctx, event, expt)
	}
	return true, nil
}

func (e *ExptSubmitExec) ScheduleEnd(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment, toSubmit, incomplete int) error {
	return nil
}

func (e *ExptSubmitExec) ScheduleStart(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment) error {
	return nil
}

func (e *ExptSubmitExec) NextTick(ctx context.Context, event *entity.ExptScheduleEvent, nextTick bool) error {
	if !nextTick {
		return nil
	}
	time.Sleep(time.Second * 3)
	interval := e.configer.GetExptExecConf(ctx, event.SpaceID).GetDaemonInterval()
	return e.publisher.PublishExptScheduleEvent(ctx, event, gptr.Of(interval))
}

func (e *ExptSubmitExec) PublishResult(ctx context.Context, turnEvaluatorRefs []*entity.ExptTurnEvaluatorResultRef, event *entity.ExptScheduleEvent) error {
	return nil
}

type ExptFailRetryExec struct {
	manager                IExptManager
	exptTurnResultRepo     repo.IExptTurnResultRepo
	exptItemResultRepo     repo.IExptItemResultRepo
	exptStatsRepo          repo.IExptStatsRepo
	idgenerator            idgen.IIDGenerator
	exptRepo               repo.IExperimentRepo
	idem                   idem.IdempotentService
	configer               component.IConfiger
	publisher              events.ExptEventPublisher
	evaluatorRecordService EvaluatorRecordService
}

func NewExptFailRetryMode(
	manager IExptManager,
	exptItemResultRepo repo.IExptItemResultRepo,
	exptStatsRepo repo.IExptStatsRepo,
	exptTurnResultRepo repo.IExptTurnResultRepo,
	idgenerator idgen.IIDGenerator,
	exptRepo repo.IExperimentRepo,
	idem idem.IdempotentService,
	configer component.IConfiger,
	publisher events.ExptEventPublisher,
	evaluatorRecordService EvaluatorRecordService,
) *ExptFailRetryExec {
	return &ExptFailRetryExec{
		manager:                manager,
		exptItemResultRepo:     exptItemResultRepo,
		exptStatsRepo:          exptStatsRepo,
		exptTurnResultRepo:     exptTurnResultRepo,
		idgenerator:            idgenerator,
		exptRepo:               exptRepo,
		idem:                   idem,
		configer:               configer,
		publisher:              publisher,
		evaluatorRecordService: evaluatorRecordService,
	}
}

func (e *ExptFailRetryExec) Mode() entity.ExptRunMode {
	return entity.EvaluationModeFailRetry
}

func (e *ExptFailRetryExec) ExptStart(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment) error {
	idemKey := makeStartIdemKey(event)

	exist, err := e.idem.Exist(ctx, idemKey)
	if err != nil {
		return err
	}

	if exist {
		return nil
	}

	var (
		maxLoop = 10000
		cursor  = int64(0)
		limit   = int64(50)
		status  = []int32{int32(entity.TurnRunState_Terminal), int32(entity.TurnRunState_Queueing), int32(entity.TurnRunState_Fail), int32(entity.TurnRunState_Processing)}
	)

	for i := 0; i < maxLoop; i++ {
		logs.CtxInfo(ctx, "ExptFailRetryExec.ExptStart scan unsucess item result, expt_id: %v, expt_run_id: %v, cursor: %v, limit: %v", event.ExptID, event.ExptRunID, cursor, limit)

		turnResults, ncursor, err := e.exptTurnResultRepo.ScanTurnResults(ctx, event.ExptID, status, cursor, limit, event.SpaceID)
		if err != nil {
			return err
		}

		cursor = ncursor

		if len(turnResults) == 0 {
			break
		}

		itemIDs := make(map[int64]bool)
		itemTurnIDs := make([]*entity.ItemTurnID, 0, len(turnResults))
		for _, tr := range turnResults {
			itemIDs[tr.ItemID] = true
			itemTurnIDs = append(itemTurnIDs, &entity.ItemTurnID{
				ItemID: tr.ItemID,
				TurnID: tr.TurnID,
			})
		}

		ids, err := e.idgenerator.GenMultiIDs(ctx, len(turnResults))
		if err != nil {
			return err
		}

		idIdx := 0
		itemRunLogs := make([]*entity.ExptItemResultRunLog, 0, len(itemIDs))
		for itemID := range itemIDs {
			itemRunLogs = append(itemRunLogs, &entity.ExptItemResultRunLog{
				ID:        ids[idIdx],
				SpaceID:   event.SpaceID,
				ExptID:    event.ExptID,
				ExptRunID: event.ExptRunID,
				ItemID:    itemID,
				Status:    int32(entity.ItemRunState_Queueing),
			})
			idIdx++
		}

		if err := e.exptItemResultRepo.UpdateItemsResult(ctx, event.SpaceID, event.ExptID, maps.ToSlice(itemIDs, func(k int64, v bool) int64 { return k }), map[string]any{
			"status":      int32(entity.ItemRunState_Queueing),
			"expt_run_id": event.ExptRunID,
		}); err != nil {
			return err
		}

		if err := e.exptTurnResultRepo.UpdateTurnResults(ctx, event.ExptID, itemTurnIDs, event.SpaceID, map[string]any{
			"status": int32(entity.TurnRunState_Queueing),
		}); err != nil {
			return err
		}

		if err := e.exptItemResultRepo.BatchCreateNXRunLogs(ctx, itemRunLogs); err != nil {
			return err
		}

		time.Sleep(time.Millisecond * 30)
	}

	got, err := e.exptStatsRepo.Get(ctx, event.ExptID, event.SpaceID)
	if err != nil {
		return err
	}

	pendingCnt := got.PendingItemCnt + got.FailItemCnt + got.TerminatedItemCnt + got.ProcessingItemCnt
	got.PendingItemCnt = pendingCnt
	got.FailItemCnt = 0
	got.TerminatedItemCnt = 0
	got.ProcessingItemCnt = 0

	if err := e.exptStatsRepo.Save(ctx, got); err != nil {
		return err
	}

	logs.CtxInfo(ctx, "ExptFailRetryExec.ExptStart reset pending_cnt: %v, expt_id: %v", pendingCnt, event.ExptID)

	exptDo := &entity.Experiment{
		Status:  entity.ExptStatus_Processing,
		ID:      event.ExptID,
		SpaceID: event.SpaceID,
	}

	if err := e.exptRepo.Update(ctx, exptDo); err != nil {
		return err
	}

	duration := time.Duration(e.configer.GetExptExecConf(ctx, event.SpaceID).GetZombieIntervalSecond()) * time.Second * 2
	if err := e.idem.Set(ctx, idemKey, duration); err != nil {
		return err
	}

	time.Sleep(time.Second * 3)

	return nil
}

func (e *ExptFailRetryExec) ScanEvalItems(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment) (toSubmit, incomplete, complete []*entity.ExptEvalItem, err error) {
	return newExptBaseExec(e.manager, e.idem, e.configer, e.exptItemResultRepo, e.publisher, e.evaluatorRecordService).ScanEvalItems(ctx, event, expt)
}

func (e *ExptFailRetryExec) ExptEnd(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment, toSubmit, incomplete int) (nextTick bool, err error) {
	if toSubmit == 0 && incomplete == 0 {
		logs.CtxInfo(ctx, "[ExptEval] expt daemon finished, expt_id: %v, expt_run_id: %v", event.ExptID, event.ExptRunID)
		return false, newExptBaseExec(e.manager, e.idem, e.configer, e.exptItemResultRepo, e.publisher, e.evaluatorRecordService).exptEnd(ctx, event, expt)
	}
	return true, nil
}

func (e *ExptFailRetryExec) ScheduleEnd(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment, toSubmit, incomplete int) error {
	return nil
}

func (e *ExptFailRetryExec) ScheduleStart(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment) error {
	return nil
}

func (e *ExptFailRetryExec) NextTick(ctx context.Context, event *entity.ExptScheduleEvent, nextTick bool) error {
	if !nextTick {
		return nil
	}
	time.Sleep(time.Second * 3)
	interval := e.configer.GetExptExecConf(ctx, event.SpaceID).GetDaemonInterval()
	return e.publisher.PublishExptScheduleEvent(ctx, event, gptr.Of(interval))
}

func (e *ExptFailRetryExec) PublishResult(ctx context.Context, turnEvaluatorRefs []*entity.ExptTurnEvaluatorResultRef, event *entity.ExptScheduleEvent) error {
	if event.ExptType != entity.ExptType_Offline { // 不等于offline用于兼容历史数据，不带type的都先放行
		return newExptBaseExec(e.manager, e.idem, e.configer, e.exptItemResultRepo, e.publisher, e.evaluatorRecordService).publishResult(ctx, turnEvaluatorRefs, event)
	}
	return nil
}

type ExptAppendExec struct {
	manager                  IExptManager
	exptRepo                 repo.IExperimentRepo
	exptStatsRepo            repo.IExptStatsRepo
	exptItemResultRepo       repo.IExptItemResultRepo
	exptTurnResultRepo       repo.IExptTurnResultRepo
	idgenerator              idgen.IIDGenerator
	evaluationSetItemService EvaluationSetItemService
	idem                     idem.IdempotentService
	configer                 component.IConfiger
	publisher                events.ExptEventPublisher
	evaluatorRecordService   EvaluatorRecordService
}

func NewExptAppendMode(
	manager IExptManager,
	exptItemResultRepo repo.IExptItemResultRepo,
	exptStatsRepo repo.IExptStatsRepo,
	exptTurnResultRepo repo.IExptTurnResultRepo,
	idgenerator idgen.IIDGenerator,
	evaluationSetItemService EvaluationSetItemService,
	exptRepo repo.IExperimentRepo,
	idem idem.IdempotentService,
	configer component.IConfiger,
	publisher events.ExptEventPublisher,
	evaluatorRecordService EvaluatorRecordService,
) *ExptAppendExec {
	return &ExptAppendExec{
		manager:                  manager,
		exptItemResultRepo:       exptItemResultRepo,
		exptStatsRepo:            exptStatsRepo,
		exptTurnResultRepo:       exptTurnResultRepo,
		idgenerator:              idgenerator,
		evaluationSetItemService: evaluationSetItemService,
		exptRepo:                 exptRepo,
		idem:                     idem,
		configer:                 configer,
		publisher:                publisher,
		evaluatorRecordService:   evaluatorRecordService,
	}
}

func (e *ExptAppendExec) Mode() entity.ExptRunMode {
	return entity.EvaluationModeAppend
}

func (e *ExptAppendExec) ScanEvalItems(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment) (toSubmit, incomplete, complete []*entity.ExptEvalItem, err error) {
	toSubmit, incomplete, complete, err = newExptBaseExec(e.manager, e.idem, e.configer, e.exptItemResultRepo, e.publisher, e.evaluatorRecordService).ScanEvalItems(ctx, event, expt)
	if err != nil {
		logs.CtxError(ctx, "[ExptEval] expt daemon scan eval items failed, expt_id: %v, expt_run_id: %v, err: %v", event.ExptID, event.ExptRunID, err)
	}
	return toSubmit, incomplete, complete, err
}

func (e *ExptAppendExec) ExptEnd(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment, toSubmit, incomplete int) (nextTick bool, err error) {
	if toSubmit == 0 && incomplete == 0 && expt.Status == entity.ExptStatus_Draining {
		logs.CtxInfo(ctx, "[ExptEval] expt daemon finished, expt_id: %v, expt_run_id: %v", event.ExptID, event.ExptRunID)
		if err = newExptBaseExec(e.manager, e.idem, e.configer, e.exptItemResultRepo, e.publisher, e.evaluatorRecordService).exptEnd(ctx, event, expt); err != nil {
			logs.CtxError(ctx, "[ExptEval] expt daemon end failed, expt_id: %v, expt_run_id: %v, err: %v", event.ExptID, event.ExptRunID, err)
		}
		return false, nil
	}
	return true, nil
}

func (e *ExptAppendExec) ExptStart(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment) error {
	return nil
}

func (e *ExptAppendExec) ScheduleEnd(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment, toSubmit, incomplete int) error {
	if toSubmit == 0 && incomplete == 0 && (expt.Status == entity.ExptStatus_Processing || expt.Status == entity.ExptStatus_Pending) {
		// 没有数据且未完成，计算一次stats
		logs.CtxInfo(ctx, "[ExptEval] expt daemon found no data, expt_id: %v, expt_run_id: %v", event.ExptID, event.ExptRunID)
		if err := e.manager.PendRun(ctx, event.ExptID, event.ExptRunID, event.SpaceID, event.Session); err != nil {
			logs.CtxError(ctx, "[ExptEval] expt daemon pend run failed, expt_id: %v, expt_run_id: %v, err: %v", event.ExptID, event.ExptRunID, err)
		}
		if err := e.manager.PendExpt(ctx, event.ExptID, event.SpaceID, event.Session); err != nil {
			logs.CtxError(ctx, "[ExptEval] expt daemon pend expt failed, expt_id: %v, expt_run_id: %v, err: %v", event.ExptID, event.ExptRunID, err)
		}
		time.Sleep(time.Second * 60)
	} else if entity.IsExptFinished(expt.Status) {
		logs.CtxInfo(ctx, "[ExptEval] online expt finished, expt_id: %v, expt_run_id: %v", event.ExptID, event.ExptRunID)
	}
	return nil
}

func (e *ExptAppendExec) ScheduleStart(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment) error {
	// 先检查是否需要结束
	logs.CtxInfo(ctx, "ExptAppendExec.ScheduleStart, expt_id: %v, expt_run_id: %v", event.ExptID, event.ExptRunID)
	deadline := expt.StartAt.Add(time.Duration(expt.MaxAliveTime) * time.Millisecond)
	if (expt.Status == entity.ExptStatus_Processing || expt.Status == entity.ExptStatus_Pending) && expt.MaxAliveTime > 0 && time.Now().After(deadline) {
		expt.Status = entity.ExptStatus_Draining
		logs.CtxInfo(ctx, "expt max alive time exceeded, expt_id: %v, expt_run_id: %v, deadline: %v", event.ExptID, event.ExptRunID, deadline)
		if err := e.exptRepo.Update(ctx, &entity.Experiment{
			ID:      event.ExptID,
			SpaceID: event.SpaceID,
			Status:  entity.ExptStatus_Draining,
		}); err != nil {
			logs.CtxError(ctx, "update expt status failed, expt_id: %v, expt_run_id: %v, err: %v", event.ExptID, event.ExptRunID, err)
		}
	} else if expt.Status == entity.ExptStatus_Pending {
		if err := e.exptRepo.Update(ctx, &entity.Experiment{
			ID:      event.ExptID,
			SpaceID: event.SpaceID,
			Status:  entity.ExptStatus_Processing,
		}); err != nil {
			logs.CtxError(ctx, "update expt status failed, expt_id: %v, expt_run_id: %v, err: %v", event.ExptID, event.ExptRunID, err)
		}
	}
	return nil
}

func (e *ExptAppendExec) NextTick(ctx context.Context, event *entity.ExptScheduleEvent, nextTick bool) error {
	if !nextTick {
		return nil
	}
	time.Sleep(time.Second * 3)
	interval := e.configer.GetExptExecConf(ctx, event.SpaceID).GetDaemonInterval()
	event.CreatedAt = time.Now().Unix()
	return e.publisher.PublishExptScheduleEvent(ctx, event, gptr.Of(interval))
}

func (e *ExptAppendExec) PublishResult(ctx context.Context, turnEvaluatorRefs []*entity.ExptTurnEvaluatorResultRef, event *entity.ExptScheduleEvent) error {
	return newExptBaseExec(e.manager, e.idem, e.configer, e.exptItemResultRepo, e.publisher, e.evaluatorRecordService).publishResult(ctx, turnEvaluatorRefs, event)
}

type exptBaseExec struct {
	Manager                IExptManager
	idem                   idem.IdempotentService
	configer               component.IConfiger
	exptItemResultRepo     repo.IExptItemResultRepo
	evaluatorRecordService EvaluatorRecordService
	publisher              events.ExptEventPublisher
}

func newExptBaseExec(
	manager IExptManager,
	idem idem.IdempotentService,
	configer component.IConfiger,
	exptItemResultRepo repo.IExptItemResultRepo,
	publisher events.ExptEventPublisher,
	evaluatorRecordService EvaluatorRecordService,
) *exptBaseExec {
	return &exptBaseExec{
		Manager:                manager,
		idem:                   idem,
		configer:               configer,
		exptItemResultRepo:     exptItemResultRepo,
		evaluatorRecordService: evaluatorRecordService,
		publisher:              publisher,
	}
}

func (e *exptBaseExec) ScanEvalItems(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment) (toSubmit, incomplete, complete []*entity.ExptEvalItem, err error) {
	incomplete, err = e.ScanRunLogEvalItems(ctx, event, expt, &entity.ExptItemRunLogFilter{
		Status: []entity.ItemRunState{entity.ItemRunState_Processing},
	}, 0)
	if err != nil {
		return nil, nil, nil, err
	}

	if submitCnt := e.getItemConcurNum(ctx, expt) - len(incomplete); submitCnt > 0 {
		toSubmit, err = e.ScanRunLogEvalItems(ctx, event, expt, &entity.ExptItemRunLogFilter{Status: []entity.ItemRunState{entity.ItemRunState_Queueing}}, int64(submitCnt))
		if err != nil {
			return nil, nil, nil, err
		}
	}

	complete, err = e.ScanRunLogEvalItems(ctx, event, expt, &entity.ExptItemRunLogFilter{
		ResultState: gptr.Of(entity.ExptItemResultStateLogged),
	}, 0)
	if err != nil {
		return nil, nil, nil, err
	}

	return toSubmit, incomplete, complete, nil
}

func (e *exptBaseExec) getItemConcurNum(ctx context.Context, expt *entity.Experiment) int {
	if val := gptr.Indirect(expt.EvalConf.ItemConcurNum); val > 0 && val <= consts.MaxItemConcurrentNum {
		return val
	}
	concurNum := e.configer.GetExptExecConf(ctx, expt.SpaceID).GetExptItemEvalConf().GetConcurNum()
	logs.CtxInfo(ctx, "GetConcurNum, expt_id: %v, concur_num: %v", expt.ID, concurNum)
	return concurNum
}

func (e *exptBaseExec) ScanRunLogEvalItems(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment, filter *entity.ExptItemRunLogFilter, limit int64) (items []*entity.ExptEvalItem, err error) {
	rls, _, err := e.exptItemResultRepo.ScanItemRunLogs(ctx, event.ExptID, event.ExptRunID, filter, 0, limit, event.SpaceID)
	if err != nil {
		return nil, err
	}

	items = make([]*entity.ExptEvalItem, 0, len(rls))
	for _, log := range rls {
		items = append(items, &entity.ExptEvalItem{
			ExptID:           event.ExptID,
			EvalSetVersionID: expt.EvalSet.EvaluationSetVersion.ID,
			ItemID:           log.ItemID,
			State:            entity.ItemRunState(log.Status),
			UpdatedAt:        log.UpdatedAt,
		})
	}
	return items, nil
}

func (e *exptBaseExec) exptEnd(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment) error {
	idemKey := makeEndIdemKey(event)

	exist, err := e.idem.Exist(ctx, idemKey)
	if err != nil {
		return err
	}

	if exist {
		return nil
	}

	completeCID := fmt.Sprintf("exptexec:onend:%d", event.ExptRunID)
	if err := e.Manager.CompleteRun(ctx, event.ExptID, event.ExptRunID, event.ExptRunMode, event.SpaceID, event.Session, entity.WithCID(completeCID)); err != nil {
		return err
	}

	if err := e.Manager.CompleteExpt(ctx, event.ExptID, event.SpaceID, event.Session, entity.WithCID(completeCID)); err != nil {
		return err
	}

	duration := time.Duration(e.configer.GetExptExecConf(ctx, event.SpaceID).GetZombieIntervalSecond()) * time.Second * 2
	if err := e.idem.Set(ctx, idemKey, duration); err != nil {
		logs.CtxError(ctx, "ExptSchedulerImpl set end idem key fail, err: %v", err)
	}
	return nil
}

func (e *exptBaseExec) publishResult(ctx context.Context, turnEvaluatorRefs []*entity.ExptTurnEvaluatorResultRef, event *entity.ExptScheduleEvent) error {
	if len(turnEvaluatorRefs) == 0 {
		return nil
	}
	exptID := turnEvaluatorRefs[0].ExptID
	evaluatorResultIDs := make([]int64, 0, len(turnEvaluatorRefs))
	for _, ref := range turnEvaluatorRefs {
		evaluatorResultIDs = append(evaluatorResultIDs, ref.EvaluatorResultID)
	}
	evaluatorRecords, err := e.evaluatorRecordService.BatchGetEvaluatorRecord(ctx, evaluatorResultIDs, true)
	if err != nil {
		return err
	}
	onlineExptTurnEvalResults := make([]*entity.OnlineExptTurnEvalResult, 0, len(evaluatorRecords))
	for _, record := range evaluatorRecords {
		onlineExptTurnEvalResult := &entity.OnlineExptTurnEvalResult{
			EvaluatorVersionId: record.EvaluatorVersionID,
			EvaluatorRecordId:  record.ID,
			Status:             int32(record.Status),
			Ext:                record.Ext,
			BaseInfo:           record.BaseInfo,
		}
		if record.EvaluatorOutputData != nil {
			if record.Status == entity.EvaluatorRunStatusFail && record.EvaluatorOutputData.EvaluatorRunError != nil {
				onlineExptTurnEvalResult.EvaluatorRunError = &entity.EvaluatorRunError{
					Code:    record.EvaluatorOutputData.EvaluatorRunError.Code,
					Message: record.EvaluatorOutputData.EvaluatorRunError.Message,
				}
			} else if record.Status == entity.EvaluatorRunStatusSuccess && record.EvaluatorOutputData.EvaluatorResult != nil {
				onlineExptTurnEvalResult.Score = gptr.Indirect(record.EvaluatorOutputData.EvaluatorResult.Score)
				onlineExptTurnEvalResult.Reasoning = record.EvaluatorOutputData.EvaluatorResult.Reasoning
			}
		}

		onlineExptTurnEvalResults = append(onlineExptTurnEvalResults, onlineExptTurnEvalResult)
	}

	// 发送评估结果Event
	err = e.publisher.PublishExptOnlineEvalResult(ctx, &entity.OnlineExptEvalResultEvent{
		ExptId:          exptID,
		TurnEvalResults: onlineExptTurnEvalResults,
	}, gptr.Of(time.Second*3))
	if err != nil {
		return err
	}
	return nil
}

func makeStartIdemKey(event *entity.ExptScheduleEvent) string {
	return fmt.Sprintf("expt_start:%v%v", event.ExptID, event.ExptRunID)
}

func makeEndIdemKey(event *entity.ExptScheduleEvent) string {
	return fmt.Sprintf("expt_end:%v%v", event.ExptID, event.ExptRunID)
}
