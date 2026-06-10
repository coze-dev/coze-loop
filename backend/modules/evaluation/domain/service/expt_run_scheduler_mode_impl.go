// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"
	"gorm.io/gorm/clause"

	"github.com/coze-dev/coze-loop/backend/infra/backoff"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/conv"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/maps"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// emptyEvaluatorResultIDsJSONForRunLogUpdate 返回写入 expt_turn_result_run_log.evaluator_result_ids 的空 JSON。
// GORM Updates(map) 会忽略值为 nil 的键，传 nil 无法清空 blob 列，旧 EvalVerIDToResID 会残留。
func emptyEvaluatorResultIDsJSONForRunLogUpdate() []byte {
	b, _ := (&entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{}}).Serialize()
	return b
}

func clearExptTurnRunLogResultRefsOnItems(ctx context.Context, turnResultRepo repo.IExptTurnResultRepo, spaceID, exptID, exptRunID int64, itemIDs []int64) error {
	if len(itemIDs) == 0 {
		return nil
	}
	return turnResultRepo.UpdateTurnRunLogWithItemIDs(ctx, spaceID, exptID, exptRunID, itemIDs, map[string]any{
		"target_result_id":     int64(0),
		"evaluator_result_ids": emptyEvaluatorResultIDsJSONForRunLogUpdate(),
	})
}

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
	templateManager IExptTemplateManager,
	exptRunLogRepo repo.IExptRunLogRepo,
	mutex lock.ILocker,
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
		templateManager:          templateManager,
		exptRunLogRepo:           exptRunLogRepo,
		mutex:                    mutex,
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
	templateManager          IExptTemplateManager
	exptRunLogRepo           repo.IExptRunLogRepo
	mutex                    lock.ILocker
}

func (f *DefaultSchedulerModeFactory) NewSchedulerMode(
	mode entity.ExptRunMode,
) (entity.ExptSchedulerMode, error) {
	switch mode {
	case entity.EvaluationModeSubmit:
		return NewExptSubmitMode(f.manager, f.exptItemResultRepo, f.exptStatsRepo, f.exptTurnResultRepo, f.idgenerator, f.evaluationSetItemService, f.exptRepo, f.idem, f.configer, f.publisher, f.evaluatorRecordService, f.resultSvc, f.templateManager), nil
	case entity.EvaluationModeTrialRun:
		return NewExptTrialRunMode(f.manager, f.exptItemResultRepo, f.exptStatsRepo, f.exptTurnResultRepo, f.idgenerator, f.evaluationSetItemService, f.exptRepo, f.idem, f.configer, f.publisher, f.evaluatorRecordService, f.resultSvc, f.templateManager), nil
	case entity.EvaluationModeFailRetry:
		return NewExptFailRetryMode(f.manager, f.exptItemResultRepo, f.exptStatsRepo, f.exptTurnResultRepo, f.idgenerator, f.exptRepo, f.idem, f.configer, f.publisher, f.evaluatorRecordService, f.templateManager), nil
	case entity.EvaluationModeAppend:
		return NewExptAppendMode(f.manager, f.exptItemResultRepo, f.exptStatsRepo, f.exptTurnResultRepo, f.idgenerator, f.evaluationSetItemService, f.exptRepo, f.idem, f.configer, f.publisher, f.evaluatorRecordService, f.templateManager, f.mutex), nil
	case entity.EvaluationModeRetryAll:
		return NewExptRetryAllExec(f.manager, f.exptItemResultRepo, f.exptStatsRepo, f.exptTurnResultRepo, f.idgenerator, f.evaluationSetItemService, f.exptRepo, f.idem, f.configer, f.publisher, f.evaluatorRecordService, f.templateManager), nil
	case entity.EvaluationModeRetryItems:
		return NewExptRetryItemsExec(f.manager, f.exptItemResultRepo, f.exptStatsRepo, f.exptTurnResultRepo, f.idgenerator, f.evaluationSetItemService, f.exptRepo, f.idem, f.configer, f.publisher, f.evaluatorRecordService, f.templateManager, f.exptRunLogRepo), nil
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
	templateManager          IExptTemplateManager
}

type ExptTrialRunExec struct {
	*ExptSubmitExec
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
	templateManager IExptTemplateManager,
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
		templateManager:          templateManager,
	}
}

func NewExptTrialRunMode(
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
	templateManager IExptTemplateManager,
) *ExptTrialRunExec {
	return &ExptTrialRunExec{
		ExptSubmitExec: NewExptSubmitMode(manager, exptItemResultRepo, exptStatsRepo, exptTurnResultRepo, idgenerator, evaluationSetItemService, exptRepo, idem, configer, publisher, evaluatorRecordService, resultSvc, templateManager),
	}
}

func (e *ExptSubmitExec) Mode() entity.ExptRunMode {
	return entity.EvaluationModeSubmit
}

func (e *ExptTrialRunExec) Mode() entity.ExptRunMode {
	return entity.EvaluationModeTrialRun
}

// sendExptStartedEvent 在实验进入 Processing 状态成功后发布 started 生命周期事件（决策1）。
// 与 sendExptCompleteEvent 并列、独立发布，不放宽其 IsExptFinished 判断；
// 仅当 fromStatus != Processing 时发布，避免重入双发。事件为 FromStatus=Pending, ToStatus=Processing。
// 发布失败仅记录日志，不阻塞实验启动主流程。
func (e *ExptSubmitExec) sendExptStartedEvent(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment) {
	if e.publisher == nil {
		return
	}
	if expt != nil && expt.Status == entity.ExptStatus_Processing {
		// 已是 Processing（重入/重试场景），不重复发布。
		return
	}
	lifecycleEvent := &entity.ExptLifecycleEvent{
		ExptID:     event.ExptID,
		ExptRunID:  gptr.Of(event.ExptRunID),
		SpaceID:    event.SpaceID,
		FromStatus: entity.ExptStatus_Pending,
		ToStatus:   entity.ExptStatus_Processing,
		ExptType:   event.ExptType,
	}
	if expt != nil {
		lifecycleEvent.ExptType = expt.ExptType
		lifecycleEvent.SourceType = expt.SourceType
	}
	if err := backoff.RetryWithElapsedTime(ctx, 15*time.Second, func() error {
		return e.publisher.PublishExptLifecycleEvent(ctx, lifecycleEvent, gptr.Of(time.Second*3))
	}); err != nil {
		logs.CtxWarn(ctx, "[ExptEval] PublishExptLifecycleEvent(started) failed after retry, expt_id: %v, err: %v", event.ExptID, err)
	}
}

// finishExptStart 完成实验启动的收尾逻辑：更新统计、状态、模板信息、幂等标记
func (e *ExptSubmitExec) finishExptStart(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment, itemCnt int, idemKey string) error {
	err := e.resultSvc.UpsertExptTurnResultFilter(ctx, event.SpaceID, event.ExptID, nil)
	if err != nil {
		logs.CtxError(ctx, "finishExptStart UpsertExptTurnResultFilter fail, expt_id: %v, err: %v", event.ExptID, err)
	}

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

	// 进入 Processing 成功后发布 started 生命周期事件（决策1）。
	e.sendExptStartedEvent(ctx, event, expt)

	var templateID int64
	if expt.ExptTemplateMeta != nil && expt.ExptTemplateMeta.ID > 0 {
		templateID = expt.ExptTemplateMeta.ID
	} else {
		updatedExpt, err := e.exptRepo.GetByID(ctx, event.ExptID, event.SpaceID)
		if err == nil && updatedExpt != nil && updatedExpt.ExptTemplateMeta != nil && updatedExpt.ExptTemplateMeta.ID > 0 {
			templateID = updatedExpt.ExptTemplateMeta.ID
		}
	}
	if templateID > 0 && e.templateManager != nil {
		if err := e.templateManager.UpdateExptInfo(ctx, templateID, event.SpaceID, event.ExptID, entity.ExptStatus_Processing, 0, nil); err != nil {
			logs.CtxError(ctx, "UpdateExptInfo failed in finishExptStart, template_id: %v, expt_id: %v, err: %v",
				templateID, event.ExptID, err)
		}
	}

	duration := time.Duration(e.configer.GetExptExecConf(ctx, event.SpaceID).GetZombieIntervalSecond()) * time.Second * 2
	if err := e.idem.Set(ctx, idemKey, duration); err != nil {
		return err
	}

	time.Sleep(time.Second * 3)

	return nil
}

func (e *ExptTrialRunExec) ExptStart(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment) error {
	if e.ExptSubmitExec == nil {
		return nil
	}
	if expt == nil || expt.TrialRunItemCount <= 0 {
		return e.ExptSubmitExec.ExptStart(ctx, event, expt)
	}

	// 如果 ext 中携带了指定的 item_ids，按ID获取条目执行
	if itemIdsStr, ok := event.Ext["__item_ids"]; ok && itemIdsStr != "" {
		return e.exptStartByItemIds(ctx, event, expt, itemIdsStr)
	}

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

		pageSize  = int32(100)
		itemCnt   = 0
		total     = int64(0)
		limit     = int(expt.TrialRunItemCount)
		pageToken *string
	)
	if limit > 0 && int(pageSize) > limit {
		pageSize = int32(limit)
	}
	orderByDesc := gptr.Of(false)
	orderByField := gptr.Of("item_id")

	for i := 0; i < maxLoop; i++ {
		logs.CtxInfo(ctx, "ExptTrialRunExec.ExptStart scan item, expt_id: %v, expt_run_id: %v, eval_set_id: %v, eval_set_ver_id: %v, page_token: %v, limit: %v, cur_cnt: %v, total: %v",
			event.ExptID, event.ExptRunID, evalSetID, evalSetVersionID, gptr.Indirect(pageToken), pageSize, itemCnt, total)

		var items []*entity.EvaluationSetItem
		var t *int64
		var nextPageToken *string
		if err := backoff.RetryThreeSeconds(ctx, func() error {
			var retryErr error
			items, t, _, nextPageToken, retryErr = e.evaluationSetItemService.ListEvaluationSetItems(ctx, &entity.ListEvaluationSetItemsParam{
				SpaceID:         event.SpaceID,
				EvaluationSetID: evalSetID,
				VersionID:       &evalSetVersionID,
				PageSize:        &pageSize,
				PageToken:       pageToken,
				OrderBys: []*entity.OrderBy{{
					Field: orderByField,
					IsAsc: orderByDesc,
				}},
			})
			return retryErr
		}); err != nil {
			return err
		}

		if t != nil {
			total = gptr.Indirect(t)
		}

		remain := limit - itemCnt
		if remain <= 0 {
			break
		}
		if len(items) > remain {
			items = items[:remain]
		}

		itemCnt += len(items)
		pageToken = nextPageToken

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

		if itemCnt >= limit || len(items) == 0 || itemCnt >= int(total) || pageToken == nil || *pageToken == "" {
			break
		}

		time.Sleep(time.Millisecond * 30)
	}

	return e.finishExptStart(ctx, event, expt, itemCnt, idemKey)
}

// exptStartByItemIds 按指定的 item_ids 获取评测集条目并创建执行记录
func (e *ExptTrialRunExec) exptStartByItemIds(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment, itemIdsStr string) error {
	var itemIds []int64
	if err := json.Unmarshal([]byte(itemIdsStr), &itemIds); err != nil {
		return fmt.Errorf("ExptTrialRunExec.exptStartByItemIds unmarshal item_ids failed: %w", err)
	}
	if len(itemIds) == 0 {
		return e.ExptSubmitExec.ExptStart(ctx, event, expt)
	}

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
		itemIdx          = int32(0)
		itemCnt          = 0
		pageSize         = int32(100)
	)

	for _, chunk := range gslice.Chunk(itemIds, int(pageSize)) {
		logs.CtxInfo(ctx, "ExptTrialRunExec.exptStartByItemIds scan item, expt_id: %v, expt_run_id: %v, eval_set_id: %v, eval_set_ver_id: %v, item_ids: %v",
			event.ExptID, event.ExptRunID, evalSetID, evalSetVersionID, chunk)

		items, err := e.evaluationSetItemService.BatchGetEvaluationSetItems(ctx, &entity.BatchGetEvaluationSetItemsParam{
			SpaceID:         event.SpaceID,
			EvaluationSetID: evalSetID,
			VersionID:       &evalSetVersionID,
			ItemIDs:         chunk,
		})
		if err != nil {
			return err
		}

		itemCnt += len(items)

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

		time.Sleep(time.Millisecond * 30)
	}

	return e.finishExptStart(ctx, event, expt, itemCnt, idemKey)
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

		pageSize  = int32(100)
		itemCnt   = 0
		total     = int64(0)
		pageToken *string
	)

	for i := 0; i < maxLoop; i++ {
		logs.CtxInfo(ctx, "ExptSubmitExec.ExptStart scan item, expt_id: %v, expt_run_id: %v, eval_set_id: %v, eval_set_ver_id: %v, page_token: %v, limit: %v, cur_cnt: %v, total: %v",
			event.ExptID, event.ExptRunID, evalSetID, evalSetVersionID, gptr.Indirect(pageToken), pageSize, itemCnt, total)

		var items []*entity.EvaluationSetItem
		var t *int64
		var nextPageToken *string
		if err := backoff.RetryThreeSeconds(ctx, func() error {
			var retryErr error
			items, t, _, nextPageToken, retryErr = e.evaluationSetItemService.ListEvaluationSetItems(ctx, &entity.ListEvaluationSetItemsParam{
				SpaceID:         event.SpaceID,
				EvaluationSetID: evalSetID,
				VersionID:       &evalSetVersionID,
				PageSize:        &pageSize,
				PageToken:       pageToken,
			})
			return retryErr
		}); err != nil {
			return err
		}

		itemCnt += len(items)
		pageToken = nextPageToken
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

		if itemCnt >= int(total) || len(items) == 0 || pageToken == nil || *pageToken == "" {
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

	// 进入 Processing 成功后发布 started 生命周期事件（决策1）。
	e.sendExptStartedEvent(ctx, event, expt)

	// 如果实验关联了模板，更新模板的 ExptInfo
	var templateID int64
	if expt.ExptTemplateMeta != nil && expt.ExptTemplateMeta.ID > 0 {
		templateID = expt.ExptTemplateMeta.ID
	} else {
		// 如果 ExptTemplateMeta 为 nil，尝试从数据库重新获取实验对象
		updatedExpt, err := e.exptRepo.GetByID(ctx, event.ExptID, event.SpaceID)
		if err == nil && updatedExpt != nil && updatedExpt.ExptTemplateMeta != nil && updatedExpt.ExptTemplateMeta.ID > 0 {
			templateID = updatedExpt.ExptTemplateMeta.ID
		}
	}
	if templateID > 0 && e.templateManager != nil {
		// 离线实验开始执行，状态变更，数量不变
		if err := e.templateManager.UpdateExptInfo(ctx, templateID, event.SpaceID, event.ExptID, entity.ExptStatus_Processing, 0, nil); err != nil {
			logs.CtxError(ctx, "UpdateExptInfo failed in ExptSubmitExec.ExptStart, template_id: %v, expt_id: %v, err: %v",
				templateID, event.ExptID, err)
		} else {
			logs.CtxInfo(ctx, "UpdateExptInfo succeeded in ExptSubmitExec.ExptStart, template_id: %v, expt_id: %v, status: %v",
				templateID, event.ExptID, entity.ExptStatus_Processing)
		}
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

func (e *ExptSubmitExec) ScheduleStart(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment) error {
	return nil
}

func (e *ExptSubmitExec) NextTick(ctx context.Context, event *entity.ExptScheduleEvent, nextTick bool) error {
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
	templateManager        IExptTemplateManager
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
	templateManager IExptTemplateManager,
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
		templateManager:        templateManager,
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

		if err := clearExptTurnRunLogResultRefsOnItems(ctx, e.exptTurnResultRepo, event.SpaceID, event.ExptID, event.ExptRunID, maps.ToSlice(itemIDs, func(k int64, v bool) int64 { return k })); err != nil {
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

	// 如果实验关联了模板，在 FailRetry 模式下重新开始时，也需要更新模板上的最新实验状态
	if e.templateManager != nil {
		var templateID int64
		if expt != nil && expt.ExptTemplateMeta != nil && expt.ExptTemplateMeta.ID > 0 {
			templateID = expt.ExptTemplateMeta.ID
		} else {
			// 兜底：从数据库重新获取实验对象
			if updatedExpt, err := e.exptRepo.GetByID(ctx, event.ExptID, event.SpaceID); err == nil && updatedExpt != nil && updatedExpt.ExptTemplateMeta != nil && updatedExpt.ExptTemplateMeta.ID > 0 {
				templateID = updatedExpt.ExptTemplateMeta.ID
			}
		}
		if templateID > 0 {
			if err := e.templateManager.UpdateExptInfo(ctx, templateID, event.SpaceID, event.ExptID, entity.ExptStatus_Processing, 0, nil); err != nil {
				logs.CtxError(ctx, "UpdateExptInfo failed in ExptFailRetryExec.ExptStart, template_id: %v, expt_id: %v, err: %v", templateID, event.ExptID, err)
			} else {
				logs.CtxInfo(ctx, "UpdateExptInfo succeeded in ExptFailRetryExec.ExptStart, template_id: %v, expt_id: %v, status: %v", templateID, event.ExptID, entity.ExptStatus_Processing)
			}
		}
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

func (e *ExptFailRetryExec) ScheduleStart(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment) error {
	return nil
}

func (e *ExptFailRetryExec) NextTick(ctx context.Context, event *entity.ExptScheduleEvent, nextTick bool) error {
	interval := e.configer.GetExptExecConf(ctx, event.SpaceID).GetDaemonInterval()
	return e.publisher.PublishExptScheduleEvent(ctx, event, gptr.Of(interval))
}

func (e *ExptFailRetryExec) PublishResult(ctx context.Context, turnEvaluatorRefs []*entity.ExptTurnEvaluatorResultRef, event *entity.ExptScheduleEvent) error {
	if event.ExptType != entity.ExptType_Offline { // 不等于offline用于兼容历史数据，不带type的都先放行
		logs.CtxInfo(ctx, "[ExptEval] ExptFailRetryExec publishResult, expt_id: %v, event: %v", event.ExptID, event)
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
	templateManager          IExptTemplateManager
	mutex                    lock.ILocker
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
	templateManager IExptTemplateManager,
	mutex lock.ILocker,
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
		templateManager:          templateManager,
		mutex:                    mutex,
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
	// 数据锁加锁，加锁后重新扫描 item
	dataLockKey := fmt.Sprintf("expt_online_data_lock:%d:%d", event.ExptID, event.ExptRunID)
	logs.CtxInfo(ctx, "[ScheduleLock][Data][ExptEval] online expt data lock acquiring, expt_id: %v, expt_run_id: %v, space_id: %v", event.ExptID, event.ExptRunID, event.SpaceID)
	locked, err := e.mutex.LockBackoff(ctx, dataLockKey, time.Second*30, time.Minute*5)
	if err != nil {
		logs.CtxError(ctx, "[ScheduleLock][Data][ExptEval] online expt data lock err, expt_id: %v, run_id: %v, err: %v", event.ExptID, event.ExptRunID, err)
		return false, err
	}
	if !locked {
		logs.CtxError(ctx, "[ScheduleLock][Data][ExptEval] online expt data lock timeout, expt_id: %v, run_id: %v", event.ExptID, event.ExptRunID)
		return false, errorx.New("[ExptEval] online expt data lock timeout")
	}
	logs.CtxInfo(ctx, "[ScheduleLock][Data][ExptEval] online expt data lock acquired, expt_id: %v, expt_run_id: %v, space_id: %v", event.ExptID, event.ExptRunID, event.SpaceID)
	defer func() {
		logs.CtxInfo(ctx, "[ScheduleLock][Data][ExptEval] online expt data lock releasing, expt_id: %v, expt_run_id: %v, space_id: %v", event.ExptID, event.ExptRunID, event.SpaceID)
		if _, uerr := e.mutex.Unlock(dataLockKey); uerr != nil {
			logs.CtxWarn(ctx, "[ScheduleLock][Data][ExptEval] online expt data unlock err, expt_id: %v, run_id: %v, err: %v", event.ExptID, event.ExptRunID, uerr)
		} else {
			logs.CtxInfo(ctx, "[ScheduleLock][Data][ExptEval] online expt data lock released, expt_id: %v, expt_run_id: %v, space_id: %v", event.ExptID, event.ExptRunID, event.SpaceID)
		}
	}()

	// 加锁后重新扫描 item，不使用之前的 scan 结果
	exptDetail, err := e.manager.GetDetail(contexts.WithCtxWriteDB(ctx), event.ExptID, event.SpaceID, event.Session)
	if err != nil {
		return false, err
	}
	toSubmitItems, incompleteItems, completeItems, err := e.ScanEvalItems(ctx, event, exptDetail)
	if err != nil {
		return false, err
	}
	toSubmitCnt := len(toSubmitItems)
	incompleteCnt := len(incompleteItems)
	complete := len(completeItems)
	logs.CtxInfo(ctx, "[ExptEval] expt append ExptEnd scan item, to_submit: %v, incomplete: %v, complete: %v", toSubmitCnt, incompleteCnt, complete)
	// 用新的 toSubmit、incomplete、complete 判断是否结束，需 Complete 数量也为零才不发送下一个心跳
	if toSubmitCnt == 0 && incompleteCnt == 0 && complete == 0 {
		switch exptDetail.Status {
		case entity.ExptStatus_Draining:
			logs.CtxInfo(ctx, "[ExptEval] expt daemon drained, expt_id: %v, expt_run_id: %v", event.ExptID, event.ExptRunID)
			if err = newExptBaseExec(e.manager, e.idem, e.configer, e.exptItemResultRepo, e.publisher, e.evaluatorRecordService).exptEnd(ctx, event, exptDetail); err != nil {
				logs.CtxError(ctx, "[ExptEval] expt daemon end failed, expt_id: %v, expt_run_id: %v, err: %v", event.ExptID, event.ExptRunID, err)
			}
		case entity.ExptStatus_Processing, entity.ExptStatus_Pending:
			logs.CtxInfo(ctx, "[ExptEval] expt daemon found no data, expt_id: %v, expt_run_id: %v", event.ExptID, event.ExptRunID)
			if err := e.manager.RecordExptData(ctx, event.ExptID, event.ExptRunID, event.SpaceID, event.Session); err != nil {
				logs.CtxError(ctx, "[ExptEval] expt daemon record expt data failed, expt_id: %v, expt_run_id: %v, err: %v", event.ExptID, event.ExptRunID, err)
			}
		}
		// 在线实验 daemon 结束：主动释放 Redis 心跳锁，适配分布式架构（ExptEnd 可能在任意实例执行）
		lockKey := fmt.Sprintf("expt_online_daemon_lock:%d:%d", event.ExptID, event.ExptRunID)
		logs.CtxInfo(ctx, "[ScheduleLock][HeartBeat][ExptEval] online expt heartbeat lock releasing, expt_id: %v, expt_run_id: %v, space_id: %v", event.ExptID, event.ExptRunID, event.SpaceID)
		if released, uerr := e.mutex.UnlockForce(ctx, lockKey); uerr != nil {
			logs.CtxWarn(ctx, "[ScheduleLock][HeartBeat][ExptEval] online expt heartbeat lock UnlockForce err, expt_id: %v, expt_run_id: %v, err: %v", event.ExptID, event.ExptRunID, uerr)
		} else if released {
			logs.CtxInfo(ctx, "[ScheduleLock][HeartBeat][ExptEval] online expt heartbeat lock released, expt_id: %v, expt_run_id: %v, space_id: %v", event.ExptID, event.ExptRunID, event.SpaceID)
		} else {
			logs.CtxInfo(ctx, "[ScheduleLock][HeartBeat][ExptEval] online expt heartbeat lock already released or not held, expt_id: %v, expt_run_id: %v, space_id: %v", event.ExptID, event.ExptRunID, event.SpaceID)
		}
		return false, nil
	}

	// 若未结束且有新数据或待 Complete，则发送下一次 tick；否则不发送
	return toSubmitCnt > 0 || incompleteCnt > 0 || complete > 0, nil
}

func (e *ExptAppendExec) ExptStart(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment) error {
	return nil
}

func (e *ExptAppendExec) ScheduleStart(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment) error {
	// 先检查是否需要结束
	logs.CtxInfo(ctx, "ExptAppendExec.ScheduleStart, expt_id: %v, expt_run_id: %v", event.ExptID, event.ExptRunID)
	deadline := expt.StartAt.Add(time.Duration(expt.MaxAliveTime) * time.Millisecond)
	if (expt.Status == entity.ExptStatus_Processing || expt.Status == entity.ExptStatus_Pending) && expt.MaxAliveTime > 0 && time.Now().After(deadline) {
		newStatus := entity.ExptStatus_Draining
		logs.CtxInfo(ctx, "expt max alive time exceeded, expt_id: %v, expt_run_id: %v, deadline: %v", event.ExptID, event.ExptRunID, deadline)
		if err := e.exptRepo.Update(ctx, &entity.Experiment{
			ID:      event.ExptID,
			SpaceID: event.SpaceID,
			Status:  newStatus,
		}); err != nil {
			logs.CtxError(ctx, "update expt status failed, expt_id: %v, expt_run_id: %v, err: %v", event.ExptID, event.ExptRunID, err)
		} else {
			// 如果实验关联了模板，更新模板的 ExptInfo（状态变更，数量不变）
			if expt.ExptTemplateMeta != nil && expt.ExptTemplateMeta.ID > 0 && e.templateManager != nil {
				if err := e.templateManager.UpdateExptInfo(ctx, expt.ExptTemplateMeta.ID, event.SpaceID, event.ExptID, newStatus, 0, nil); err != nil {
					logs.CtxError(ctx, "UpdateExptInfo failed in ScheduleStart (Draining), template_id: %v, expt_id: %v, err: %v",
						expt.ExptTemplateMeta.ID, event.ExptID, err)
				}
			}
		}
	} else if expt.Status == entity.ExptStatus_Pending {
		newStatus := entity.ExptStatus_Processing
		if err := e.exptRepo.Update(ctx, &entity.Experiment{
			ID:      event.ExptID,
			SpaceID: event.SpaceID,
			Status:  newStatus,
		}); err != nil {
			logs.CtxError(ctx, "update expt status failed, expt_id: %v, expt_run_id: %v, err: %v", event.ExptID, event.ExptRunID, err)
		} else {
			// 如果实验关联了模板，更新模板的 ExptInfo（状态变更，数量不变）
			if expt.ExptTemplateMeta != nil && expt.ExptTemplateMeta.ID > 0 && e.templateManager != nil {
				if err := e.templateManager.UpdateExptInfo(ctx, expt.ExptTemplateMeta.ID, event.SpaceID, event.ExptID, newStatus, 0, nil); err != nil {
					logs.CtxError(ctx, "UpdateExptInfo failed in ScheduleStart, template_id: %v, expt_id: %v, err: %v",
						expt.ExptTemplateMeta.ID, event.ExptID, err)
				}
			}
		}
	}
	return nil
}

func (e *ExptAppendExec) NextTick(ctx context.Context, event *entity.ExptScheduleEvent, nextTick bool) error {
	conf := e.configer.GetExptExecConf(ctx, event.SpaceID)
	interval := 20 * time.Second
	if conf != nil {
		interval = conf.GetDaemonInterval()
	}
	// 锁由 scheduler 的自动续期 goroutine 持有，此处直接发送
	event.CreatedAt = time.Now().Unix()
	return e.publisher.PublishExptScheduleEvent(ctx, event, gptr.Of(interval))
}

func (e *ExptAppendExec) PublishResult(ctx context.Context, turnEvaluatorRefs []*entity.ExptTurnEvaluatorResultRef, event *entity.ExptScheduleEvent) error {
	logs.CtxInfo(ctx, "[ExptEval] ExptAppendExec publishResult, expt_id: %v, event: %v", event.ExptID, event)
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
	incomplete, complete, err = e.scanIncompleteAndComplete(ctx, event, expt)
	if err != nil {
		return nil, nil, nil, err
	}

	if submitCnt := e.getItemConcurNum(ctx, expt) - len(incomplete); submitCnt > 0 {
		toSubmit, err = e.scanToSubmit(ctx, event, expt, int64(submitCnt))
		if err != nil {
			return nil, nil, nil, err
		}
	}

	return toSubmit, incomplete, complete, nil
}

func (e *exptBaseExec) scanIncompleteAndComplete(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment) (incomplete, complete []*entity.ExptEvalItem, err error) {
	rls, _, err := e.exptItemResultRepo.ScanItemRunLogs(ctx, event.ExptID, event.ExptRunID, &entity.ExptItemRunLogFilter{
		RawFilter: true,
		RawCond:   clause.Expr{SQL: "status IN (?) OR result_state = ?", Vars: []interface{}{[]int32{int32(entity.ItemRunState_Processing)}, int32(entity.ExptItemResultStateLogged)}},
	}, 0, 0, event.SpaceID)
	if err != nil {
		return nil, nil, err
	}
	incomplete = make([]*entity.ExptEvalItem, 0)
	complete = make([]*entity.ExptEvalItem, 0)
	evalSetVersionID := expt.EvalSet.EvaluationSetVersion.ID
	for _, log := range rls {
		item := &entity.ExptEvalItem{
			ExptID:           event.ExptID,
			EvalSetVersionID: evalSetVersionID,
			ItemID:           log.ItemID,
			State:            entity.ItemRunState(log.Status),
			UpdatedAt:        log.UpdatedAt,
		}
		if log.Status == int32(entity.ItemRunState_Processing) {
			incomplete = append(incomplete, item)
		}
		if log.ResultState == int32(entity.ExptItemResultStateLogged) {
			complete = append(complete, item)
		}
	}
	return incomplete, complete, nil
}

func (e *exptBaseExec) getItemConcurNum(ctx context.Context, expt *entity.Experiment) int {
	maxItemConcurNum := e.configer.GetExptExecConf(ctx, expt.SpaceID).GetExptItemEvalConf().GetMaxItemConcurNum()
	if val := gptr.Indirect(expt.EvalConf.ItemConcurNum); val > 0 && val <= maxItemConcurNum {
		return val
	}
	concurNum := e.configer.GetExptExecConf(ctx, expt.SpaceID).GetExptItemEvalConf().GetConcurNum()
	logs.CtxInfo(ctx, "GetConcurNum, expt_id: %v, concur_num: %v", expt.ID, concurNum)
	return concurNum
}

func (e *exptBaseExec) scanToSubmit(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment, limit int64) (items []*entity.ExptEvalItem, err error) {
	rls, _, err := e.exptItemResultRepo.ScanItemRunLogs(ctx, event.ExptID, event.ExptRunID, &entity.ExptItemRunLogFilter{Status: []entity.ItemRunState{entity.ItemRunState_Queueing}}, 0, limit, event.SpaceID)
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
	if err := e.Manager.CompleteRun(ctx, event.ExptID, event.ExptRunID, event.SpaceID, event.Session, entity.WithCID(completeCID), entity.WithCompleteInterval(time.Second*2)); err != nil {
		return err
	}

	if err := e.Manager.CompleteExpt(ctx, event.ExptID, &event.ExptRunID, event.SpaceID, event.Session, entity.WithCID(completeCID), entity.WithCompleteInterval(time.Second*2)); err != nil {
		return err
	}

	duration := time.Duration(e.configer.GetExptExecConf(ctx, event.SpaceID).GetZombieIntervalSecond()) * time.Second * 2
	if err := e.idem.Set(ctx, idemKey, duration); err != nil {
		logs.CtxError(ctx, "ExptSchedulerImpl set end idem key fail, err: %v", err)
	}
	return nil
}

func (e *exptBaseExec) publishResult(ctx context.Context, turnEvaluatorRefs []*entity.ExptTurnEvaluatorResultRef, event *entity.ExptScheduleEvent) error {
	logs.CtxInfo(ctx, "[ExptEval] publishResult, expt_id: %v, event: %v", event.ExptID, event)
	if len(turnEvaluatorRefs) == 0 {
		return nil
	}
	exptID := turnEvaluatorRefs[0].ExptID
	evaluatorResultIDs := make([]int64, 0, len(turnEvaluatorRefs))
	for _, ref := range turnEvaluatorRefs {
		evaluatorResultIDs = append(evaluatorResultIDs, ref.EvaluatorResultID)
	}
	evaluatorRecords, err := e.evaluatorRecordService.BatchGetEvaluatorRecord(ctx, evaluatorResultIDs, true, false)
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

func NewExptRetryAllExec(
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
	templateManager IExptTemplateManager,
) *ExptRetryAllExec {
	return &ExptRetryAllExec{
		configer:                 configer,
		evaluationSetItemService: evaluationSetItemService,
		evaluatorRecordService:   evaluatorRecordService,
		exptItemResultRepo:       exptItemResultRepo,
		exptRepo:                 exptRepo,
		exptStatsRepo:            exptStatsRepo,
		exptTurnResultRepo:       exptTurnResultRepo,
		idem:                     idem,
		idgenerator:              idgenerator,
		manager:                  manager,
		publisher:                publisher,
		templateManager:          templateManager,
	}
}

type ExptRetryAllExec struct {
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
	templateManager          IExptTemplateManager
}

func (e *ExptRetryAllExec) Mode() entity.ExptRunMode {
	return entity.EvaluationModeRetryAll
}

func (e *ExptRetryAllExec) ExptStart(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment) error {
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

		maxLoop   = 10000
		pageSize  = int32(100)
		itemCnt   = 0
		total     = int64(0)
		pageToken *string
	)

	for i := 0; i < maxLoop; i++ {
		logs.CtxInfo(ctx, "ExptRetryAllExec.ExptStart scan item, expt_id: %v, expt_run_id: %v, eval_set_id: %v, eval_set_ver_id: %v, page_token: %v, limit: %v, cur_cnt: %v, total: %v",
			event.ExptID, event.ExptRunID, evalSetID, evalSetVersionID, gptr.Indirect(pageToken), pageSize, itemCnt, total)

		var items []*entity.EvaluationSetItem
		var t *int64
		var nextPageToken *string
		if err := backoff.RetryThreeSeconds(ctx, func() error {
			var retryErr error
			items, t, _, nextPageToken, retryErr = e.evaluationSetItemService.ListEvaluationSetItems(ctx, &entity.ListEvaluationSetItemsParam{
				SpaceID:         event.SpaceID,
				EvaluationSetID: evalSetID,
				VersionID:       &evalSetVersionID,
				PageSize:        &pageSize,
				PageToken:       pageToken,
			})
			return retryErr
		}); err != nil {
			return err
		}

		itemCnt += len(items)
		pageToken = nextPageToken
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
		itemIDs := gslice.ToMap(items, func(t *entity.EvaluationSetItem) (int64, bool) { return t.ItemID, true })
		itemTurnIDs := make([]*entity.ItemTurnID, 0, len(items))
		for _, item := range items {
			for _, turn := range item.Turns {
				itemIDs[item.ItemID] = true
				itemTurnIDs = append(itemTurnIDs, &entity.ItemTurnID{
					ItemID: item.ItemID,
					TurnID: turn.ID,
				})
			}
		}

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
			"status":           int32(entity.TurnRunState_Queueing),
			"target_result_id": int64(0),
		}); err != nil {
			return err
		}

		if err := clearExptTurnRunLogResultRefsOnItems(ctx, e.exptTurnResultRepo, event.SpaceID, event.ExptID, event.ExptRunID, maps.ToSlice(itemIDs, func(k int64, v bool) int64 { return k })); err != nil {
			return err
		}

		if err := e.exptItemResultRepo.BatchCreateNXRunLogs(ctx, itemRunLogs); err != nil {
			return err
		}

		if itemCnt >= int(total) || len(items) == 0 || pageToken == nil || *pageToken == "" {
			break
		}

		time.Sleep(time.Millisecond * 30)
	}

	got, err := e.exptStatsRepo.Get(ctx, event.ExptID, event.SpaceID)
	if err != nil {
		return err
	}

	pendingCnt := got.PendingItemCnt + got.FailItemCnt + got.TerminatedItemCnt + got.ProcessingItemCnt + got.SuccessItemCnt
	got.PendingItemCnt = pendingCnt
	got.FailItemCnt = 0
	got.TerminatedItemCnt = 0
	got.ProcessingItemCnt = 0
	got.SuccessItemCnt = 0

	if err := e.exptStatsRepo.Save(ctx, got); err != nil {
		return err
	}

	logs.CtxInfo(ctx, "ExptRetryAllExec.ExptStart reset pending_cnt: %v, expt_id: %v", pendingCnt, event.ExptID)

	exptDo := &entity.Experiment{
		Status:  entity.ExptStatus_Processing,
		ID:      event.ExptID,
		SpaceID: event.SpaceID,
	}

	if err := e.exptRepo.Update(ctx, exptDo); err != nil {
		return err
	}

	if e.templateManager != nil {
		var templateID int64
		if expt.ExptTemplateMeta != nil && expt.ExptTemplateMeta.ID > 0 {
			templateID = expt.ExptTemplateMeta.ID
		}
		if templateID > 0 {
			if err := e.templateManager.UpdateExptInfo(ctx, templateID, event.SpaceID, event.ExptID, entity.ExptStatus_Processing, 0, nil); err != nil {
				logs.CtxError(ctx, "UpdateExptInfo failed in ExptRetryAllExec.ExptStart, template_id: %v, expt_id: %v, err: %v", templateID, event.ExptID, err)
			}
		}
	}

	duration := time.Duration(e.configer.GetExptExecConf(ctx, event.SpaceID).GetZombieIntervalSecond()) * time.Second * 2
	if err := e.idem.Set(ctx, idemKey, duration); err != nil {
		return err
	}

	time.Sleep(time.Second * 3)

	return nil
}

func (e *ExptRetryAllExec) ScanEvalItems(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment) (toSubmit, incomplete, complete []*entity.ExptEvalItem, err error) {
	return newExptBaseExec(e.manager, e.idem, e.configer, e.exptItemResultRepo, e.publisher, e.evaluatorRecordService).ScanEvalItems(ctx, event, expt)
}

func (e *ExptRetryAllExec) ExptEnd(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment, toSubmit, incomplete int) (nextTick bool, err error) {
	if toSubmit == 0 && incomplete == 0 {
		logs.CtxInfo(ctx, "[ExptEval] expt daemon finished, expt_id: %v, expt_run_id: %v", event.ExptID, event.ExptRunID)
		return false, newExptBaseExec(e.manager, e.idem, e.configer, e.exptItemResultRepo, e.publisher, e.evaluatorRecordService).exptEnd(ctx, event, expt)
	}
	return true, nil
}

func (e *ExptRetryAllExec) ScheduleStart(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment) error {
	return nil
}

func (e *ExptRetryAllExec) NextTick(ctx context.Context, event *entity.ExptScheduleEvent, nextTick bool) error {
	interval := e.configer.GetExptExecConf(ctx, event.SpaceID).GetDaemonInterval()
	return e.publisher.PublishExptScheduleEvent(ctx, event, gptr.Of(interval))
}

func (e *ExptRetryAllExec) PublishResult(ctx context.Context, turnEvaluatorRefs []*entity.ExptTurnEvaluatorResultRef, event *entity.ExptScheduleEvent) error {
	if event.ExptType == entity.ExptType_Offline {
		return nil
	}
	return newExptBaseExec(e.manager, e.idem, e.configer, e.exptItemResultRepo, e.publisher, e.evaluatorRecordService).publishResult(ctx, turnEvaluatorRefs, event)
}

func NewExptRetryItemsExec(
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
	templateManager IExptTemplateManager,
	exptRunLogRepo repo.IExptRunLogRepo,
) *ExptRetryItemsExec {
	return &ExptRetryItemsExec{
		configer:                 configer,
		evaluationSetItemService: evaluationSetItemService,
		evaluatorRecordService:   evaluatorRecordService,
		exptItemResultRepo:       exptItemResultRepo,
		exptRepo:                 exptRepo,
		exptStatsRepo:            exptStatsRepo,
		exptTurnResultRepo:       exptTurnResultRepo,
		idem:                     idem,
		idgenerator:              idgenerator,
		manager:                  manager,
		publisher:                publisher,
		templateManager:          templateManager,
		exptRunLogRepo:           exptRunLogRepo,
	}
}

type ExptRetryItemsExec struct {
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
	templateManager          IExptTemplateManager
	exptRunLogRepo           repo.IExptRunLogRepo
}

func (e *ExptRetryItemsExec) Mode() entity.ExptRunMode {
	return entity.EvaluationModeRetryItems
}

func (e *ExptRetryItemsExec) ExptStart(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment) error {
	idemKey := makeStartIdemKey(event)
	exist, err := e.idem.Exist(ctx, idemKey)
	if err != nil {
		return err
	}
	if exist {
		return nil
	}

	if err := e.resetEvalItems(ctx, event, expt, event.ExecEvalSetItemIDs); err != nil {
		return err
	}

	if err := e.exptRepo.Update(ctx, &entity.Experiment{
		Status:  entity.ExptStatus_Processing,
		ID:      event.ExptID,
		SpaceID: event.SpaceID,
	}); err != nil {
		return err
	}

	if e.templateManager != nil {
		var templateID int64
		if expt.ExptTemplateMeta != nil && expt.ExptTemplateMeta.ID > 0 {
			templateID = expt.ExptTemplateMeta.ID
		}
		if templateID > 0 {
			if err := e.templateManager.UpdateExptInfo(ctx, templateID, event.SpaceID, event.ExptID, entity.ExptStatus_Processing, 0, nil); err != nil {
				logs.CtxError(ctx, "UpdateExptInfo failed in ExptRetryItemsExec.ExptStart, template_id: %v, expt_id: %v, err: %v", templateID, event.ExptID, err)
			}
		}
	}

	duration := time.Duration(e.configer.GetExptExecConf(ctx, event.SpaceID).GetZombieIntervalSecond()) * time.Second * 2
	if err := e.idem.Set(ctx, idemKey, duration); err != nil {
		return err
	}

	time.Sleep(time.Second * 3)

	return nil
}

func (e *ExptRetryItemsExec) resetEvalItems(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment, itemIDs []int64) error {
	got, err := e.exptStatsRepo.Get(ctx, event.ExptID, event.SpaceID)
	if err != nil {
		return err
	}

	var (
		evalSetID        = expt.EvalSet.ID
		evalSetVersionID = expt.EvalSet.EvaluationSetVersion.ID
		pageSize         = int32(100)
	)

	for _, chunk := range gslice.Chunk(itemIDs, int(pageSize)) {
		logs.CtxInfo(ctx, "ExptRetryItemsExec.resetEvalItems scan item, expt_id: %v, expt_run_id: %v, eval_set_id: %v, eval_set_ver_id: %v, item_ids: %v",
			event.ExptID, event.ExptRunID, evalSetID, evalSetVersionID, chunk)

		items, err := e.evaluationSetItemService.BatchGetEvaluationSetItems(ctx, &entity.BatchGetEvaluationSetItemsParam{
			SpaceID:         event.SpaceID,
			EvaluationSetID: evalSetID,
			VersionID:       &evalSetVersionID,
			ItemIDs:         chunk,
		})
		if err != nil {
			return err
		}

		turnCnt := 0
		for _, item := range items {
			turnCnt += len(item.Turns)
		}

		ids, err := e.idgenerator.GenMultiIDs(ctx, len(items)+turnCnt)
		if err != nil {
			return err
		}

		idIdx := 0
		itemIDMap := gslice.ToMap(items, func(t *entity.EvaluationSetItem) (int64, bool) { return t.ItemID, true })
		itemTurnIDs := make([]*entity.ItemTurnID, 0, len(items))
		for _, item := range items {
			for _, turn := range item.Turns {
				itemIDMap[item.ItemID] = true
				itemTurnIDs = append(itemTurnIDs, &entity.ItemTurnID{
					ItemID: item.ItemID,
					TurnID: turn.ID,
				})
			}
		}

		itemRunLogs := make([]*entity.ExptItemResultRunLog, 0, len(itemIDMap))
		for itemID := range itemIDMap {
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

		irs, err := e.exptItemResultRepo.MGetItemResults(ctx, event.ExptID, chunk, event.SpaceID)
		if err != nil {
			return err
		}

		for _, ir := range irs {
			switch ir.Status {
			case entity.ItemRunState_Processing:
				got.ProcessingItemCnt--
				got.PendingItemCnt++
			case entity.ItemRunState_Success:
				got.SuccessItemCnt--
				got.PendingItemCnt++
			case entity.ItemRunState_Fail:
				got.FailItemCnt--
				got.PendingItemCnt++
			case entity.ItemRunState_Terminal:
				got.TerminatedItemCnt--
				got.PendingItemCnt++
			default:
			}
		}

		if err := e.exptItemResultRepo.UpdateItemsResult(ctx, event.SpaceID, event.ExptID, maps.ToSlice(itemIDMap, func(k int64, v bool) int64 { return k }), map[string]any{
			"status":      int32(entity.ItemRunState_Queueing),
			"expt_run_id": event.ExptRunID,
		}); err != nil {
			return err
		}

		if err := e.exptTurnResultRepo.UpdateTurnResults(ctx, event.ExptID, itemTurnIDs, event.SpaceID, map[string]any{
			"status":           int32(entity.TurnRunState_Queueing),
			"target_result_id": int64(0),
		}); err != nil {
			return err
		}

		if err := clearExptTurnRunLogResultRefsOnItems(ctx, e.exptTurnResultRepo, event.SpaceID, event.ExptID, event.ExptRunID, maps.ToSlice(itemIDMap, func(k int64, v bool) int64 { return k })); err != nil {
			return err
		}

		if err := e.exptItemResultRepo.BatchCreateNXRunLogs(ctx, itemRunLogs); err != nil {
			return err
		}

		time.Sleep(time.Millisecond * 30)
	}

	if err := e.exptStatsRepo.Save(ctx, got); err != nil {
		return err
	}

	logs.CtxInfo(ctx, "ExptRetryItemsExec.resetEvalItems reset stat: %v, expt_id: %v", json.Jsonify(got), event.ExptID)
	time.Sleep(time.Second * 3)
	return nil
}

func (e *ExptRetryItemsExec) ScanEvalItems(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment) (toSubmit, incomplete, complete []*entity.ExptEvalItem, err error) {
	return newExptBaseExec(e.manager, e.idem, e.configer, e.exptItemResultRepo, e.publisher, e.evaluatorRecordService).ScanEvalItems(ctx, event, expt)
}

func (e *ExptRetryItemsExec) ExptEnd(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment, toSubmit, incomplete int) (nextTick bool, err error) {
	if toSubmit > 0 || incomplete > 0 {
		return true, nil
	}

	if err := e.manager.LockCompletingRun(ctx, event.ExptID, event.ExptRunID, event.SpaceID, event.Session); err != nil {
		return false, err
	}
	defer func() {
		_ = e.manager.UnlockCompletingRun(ctx, event.ExptID, event.ExptRunID, event.SpaceID, event.Session)
	}()

	logs.CtxInfo(ctx, "[ExptEval] expt daemon finished, expt_id: %v, expt_run_id: %v", event.ExptID, event.ExptRunID)

	got, err := e.exptRunLogRepo.Get(ctx, event.ExptID, event.ExptRunID)
	if err != nil {
		return false, err
	}

	exist := gslice.ToMap(event.ExecEvalSetItemIDs, func(t int64) (int64, bool) { return t, true })
	for _, itemID := range got.GetItemIDs() {
		if !exist[itemID] {
			return true, nil
		}
	}

	if err := newExptBaseExec(e.manager, e.idem, e.configer, e.exptItemResultRepo, e.publisher, e.evaluatorRecordService).exptEnd(ctx, event, expt); err != nil {
		return false, err
	}
	return false, nil
}

func (e *ExptRetryItemsExec) ScheduleStart(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment) error {
	rl, err := e.exptRunLogRepo.Get(ctx, event.ExptID, event.ExptRunID)
	if err != nil {
		return err
	}

	var (
		absence []int64
		all     = rl.GetItemIDs()
		exist   = gslice.ToMap(event.ExecEvalSetItemIDs, func(t int64) (int64, bool) { return t, true })
	)
	for _, itemID := range all {
		if !exist[itemID] {
			absence = append(absence, itemID)
		}
	}
	event.ExecEvalSetItemIDs = all
	logs.CtxInfo(ctx, "ExptRetryItemsExec.ScheduleStart found absent item_id: %v, expt_id: %v", absence, event.ExptID)

	return e.resetEvalItems(ctx, event, expt, absence)
}

func (e *ExptRetryItemsExec) NextTick(ctx context.Context, event *entity.ExptScheduleEvent, nextTick bool) error {
	interval := e.configer.GetExptExecConf(ctx, event.SpaceID).GetDaemonInterval()
	return e.publisher.PublishExptScheduleEvent(ctx, event, gptr.Of(interval))
}

func (e *ExptRetryItemsExec) PublishResult(ctx context.Context, turnEvaluatorRefs []*entity.ExptTurnEvaluatorResultRef, event *entity.ExptScheduleEvent) error {
	if event.ExptType == entity.ExptType_Offline {
		return nil
	}
	return newExptBaseExec(e.manager, e.idem, e.configer, e.exptItemResultRepo, e.publisher, e.evaluatorRecordService).publishResult(ctx, turnEvaluatorRefs, event)
}
