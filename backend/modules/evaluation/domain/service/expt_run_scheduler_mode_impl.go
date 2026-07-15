// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"strconv"
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
	exptItemRefRepo repo.IExptItemRefRepo,
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
		exptItemRefRepo:          exptItemRefRepo,
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
	exptItemRefRepo          repo.IExptItemRefRepo // ★ MultiSetConfig 实验 exptStartMultiSet 用
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
		return NewExptSubmitMode(f.manager, f.exptItemResultRepo, f.exptStatsRepo, f.exptTurnResultRepo, f.idgenerator, f.evaluationSetItemService, f.exptRepo, f.idem, f.configer, f.publisher, f.evaluatorRecordService, f.resultSvc, f.templateManager, f.exptItemRefRepo), nil
	case entity.EvaluationModeTrialRun:
		return NewExptTrialRunMode(f.manager, f.exptItemResultRepo, f.exptStatsRepo, f.exptTurnResultRepo, f.idgenerator, f.evaluationSetItemService, f.exptRepo, f.idem, f.configer, f.publisher, f.evaluatorRecordService, f.resultSvc, f.templateManager, f.exptItemRefRepo), nil
	case entity.EvaluationModeFailRetry:
		return NewExptFailRetryMode(f.manager, f.exptItemResultRepo, f.exptStatsRepo, f.exptTurnResultRepo, f.idgenerator, f.exptRepo, f.idem, f.configer, f.publisher, f.evaluatorRecordService, f.templateManager), nil
	case entity.EvaluationModeAppend:
		return NewExptAppendMode(f.manager, f.exptItemResultRepo, f.exptStatsRepo, f.exptTurnResultRepo, f.idgenerator, f.evaluationSetItemService, f.exptRepo, f.idem, f.configer, f.publisher, f.evaluatorRecordService, f.templateManager, f.mutex), nil
	case entity.EvaluationModeRetryAll:
		return NewExptRetryAllExec(f.manager, f.exptItemResultRepo, f.exptStatsRepo, f.exptTurnResultRepo, f.idgenerator, f.evaluationSetItemService, f.exptRepo, f.idem, f.configer, f.publisher, f.evaluatorRecordService, f.templateManager, f.exptItemRefRepo), nil
	case entity.EvaluationModeRetryItems:
		return NewExptRetryItemsExec(f.manager, f.exptItemResultRepo, f.exptStatsRepo, f.exptTurnResultRepo, f.idgenerator, f.evaluationSetItemService, f.exptRepo, f.idem, f.configer, f.publisher, f.evaluatorRecordService, f.templateManager, f.exptRunLogRepo, f.exptItemRefRepo), nil
	default:
		return nil, fmt.Errorf("NewSchedulerMode with unknown mode: %v", mode)
	}
}

type ExptSubmitExec struct {
	manager                  IExptManager
	exptStatsRepo            repo.IExptStatsRepo
	exptItemResultRepo       repo.IExptItemResultRepo
	exptTurnResultRepo       repo.IExptTurnResultRepo
	exptItemRefRepo          repo.IExptItemRefRepo // ★
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
	exptItemRefRepo ...repo.IExptItemRefRepo, // variadic for backward compat
) *ExptSubmitExec {
	exec := &ExptSubmitExec{
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
	if len(exptItemRefRepo) > 0 {
		exec.exptItemRefRepo = exptItemRefRepo[0]
	}
	return exec
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
	exptItemRefRepo ...repo.IExptItemRefRepo, // variadic for backward compat (同 NewExptSubmitMode)
) *ExptTrialRunExec {
	return &ExptTrialRunExec{
		ExptSubmitExec: NewExptSubmitMode(manager, exptItemResultRepo, exptStatsRepo, exptTurnResultRepo, idgenerator, evaluationSetItemService, exptRepo, idem, configer, publisher, evaluatorRecordService, resultSvc, templateManager, exptItemRefRepo...),
	}
}

func (e *ExptSubmitExec) Mode() entity.ExptRunMode {
	return entity.EvaluationModeSubmit
}

func (e *ExptTrialRunExec) Mode() entity.ExptRunMode {
	return entity.EvaluationModeTrialRun
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
				VersionID:       resolveSetReadVersionID(evalSetID, evalSetVersionID),
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
				ID:            ids[idIdx],
				SpaceID:       event.SpaceID,
				ExptID:        event.ExptID,
				ExptRunID:     event.ExptRunID,
				ItemID:        item.ItemID,
				ItemVersionID: gptr.Indirect(item.ItemVersionID), // item 级版本 (版本评测集真值 / 无版本 0); 供单行执行按版本取数
				ItemIdx:       itemIdx,
				Status:        entity.ItemRunState_Queueing,
			}
			eirs = append(eirs, eir)
			itemIdx++
			idIdx++

			for turnIdx, turn := range item.Turns {
				etr := &entity.ExptTurnResult{
					ID:            ids[idIdx],
					SpaceID:       event.SpaceID,
					ExptID:        event.ExptID,
					ExptRunID:     event.ExptRunID,
					ItemID:        item.ItemID,
					ItemVersionID: gptr.Indirect(item.ItemVersionID), // item 级版本 (版本评测集真值 / 无版本 0)
					TurnID:        turn.ID,
					TurnIdx:       int32(turnIdx),
					Status:        int32(entity.TurnRunState_Queueing),
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
			VersionID:       resolveSetReadVersionID(evalSetID, evalSetVersionID),
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
				ID:            ids[idIdx],
				SpaceID:       event.SpaceID,
				ExptID:        event.ExptID,
				ExptRunID:     event.ExptRunID,
				ItemID:        item.ItemID,
				ItemVersionID: gptr.Indirect(item.ItemVersionID), // item 级版本 (版本评测集真值 / 无版本 0); 供单行执行按版本取数
				ItemIdx:       itemIdx,
				Status:        entity.ItemRunState_Queueing,
			}
			eirs = append(eirs, eir)
			itemIdx++
			idIdx++

			for turnIdx, turn := range item.Turns {
				etr := &entity.ExptTurnResult{
					ID:            ids[idIdx],
					SpaceID:       event.SpaceID,
					ExptID:        event.ExptID,
					ExptRunID:     event.ExptRunID,
					ItemID:        item.ItemID,
					ItemVersionID: gptr.Indirect(item.ItemVersionID), // item 级版本 (版本评测集真值 / 无版本 0)
					TurnID:        turn.ID,
					TurnIdx:       int32(turnIdx),
					Status:        int32(entity.TurnRunState_Queueing),
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

	// ★ 新路径: MultiSetConfig 走 expt_item_ref 扁平调度
	if expt.EvalSetSourceType == entity.ExptEvalSetSourceType_MultiSetConfig {
		return e.exptStartMultiSet(ctx, event, expt)
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
				VersionID:       resolveSetReadVersionID(evalSetID, evalSetVersionID),
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
				ID:            ids[idIdx],
				SpaceID:       event.SpaceID,
				ExptID:        event.ExptID,
				ExptRunID:     event.ExptRunID,
				ItemID:        item.ItemID,
				ItemVersionID: gptr.Indirect(item.ItemVersionID), // item 级版本 (版本评测集真值 / 无版本 0); 供单行执行按版本取数
				ItemIdx:       itemIdx,
				Status:        entity.ItemRunState_Queueing,
			}
			eirs = append(eirs, eir)
			itemIdx++
			idIdx++

			for turnIdx, turn := range item.Turns {
				etr := &entity.ExptTurnResult{
					ID:            ids[idIdx],
					SpaceID:       event.SpaceID,
					ExptID:        event.ExptID,
					ExptRunID:     event.ExptRunID,
					ItemID:        item.ItemID,
					ItemVersionID: gptr.Indirect(item.ItemVersionID), // item 级版本 (版本评测集真值 / 无版本 0)
					TurnID:        turn.ID,
					TurnIdx:       int32(turnIdx),
					Status:        int32(entity.TurnRunState_Queueing),
				}
				etrs = append(etrs, etr)
				idIdx++
			}
		}

		if err := e.createItemTurnResults(ctx, eirs, etrs, event.Session); err != nil {
			return err
		}

		if (total > 0 && itemCnt >= int(total)) || len(items) == 0 || pageToken == nil || *pageToken == "" {
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
			ID:            ids[idx],
			SpaceID:       eir.SpaceID,
			ExptID:        eir.ExptID,
			ExptRunID:     eir.ExptRunID,
			ItemID:        eir.ItemID,
			ItemVersionID: eir.ItemVersionID, // 从 item_result 平移 item 级版本, 供单行执行 GetItemRunLog 读回
			Status:        int32(eir.Status),
			ErrMsg:        conv.UnsafeStringToBytes(eir.ErrMsg),
			LogID:         eir.LogID,
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
		itemVersionIDs := make(map[int64]int64) // itemID → item 版本 (同 item 各 turn 共享), 平移到重试 run_log
		itemTurnIDs := make([]*entity.ItemTurnID, 0, len(turnResults))
		for _, tr := range turnResults {
			itemIDs[tr.ItemID] = true
			itemVersionIDs[tr.ItemID] = tr.ItemVersionID
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
				ID:            ids[idIdx],
				SpaceID:       event.SpaceID,
				ExptID:        event.ExptID,
				ExptRunID:     event.ExptRunID,
				ItemID:        itemID,
				ItemVersionID: itemVersionIDs[itemID], // 平移已落库的 item 版本, 供重试后单行执行按版本取数
				Status:        int32(entity.ItemRunState_Queueing),
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

type retryItemResetDeps struct {
	evaluationSetItemService EvaluationSetItemService
	exptItemResultRepo       repo.IExptItemResultRepo
	exptTurnResultRepo       repo.IExptTurnResultRepo
	idgenerator              idgen.IIDGenerator
}

func itemVersionIDPtr(versionID int64) *int64 {
	if versionID <= 0 {
		return nil
	}
	return gptr.Of(versionID)
}

func fetchEvaluationSetItemsByRefs(ctx context.Context, deps retryItemResetDeps, spaceID int64, refs []*entity.ExptItemRef) ([]*entity.EvaluationSetItem, map[int64]int64, error) {
	if len(refs) == 0 {
		return nil, nil, nil
	}

	type setVersionKey struct {
		evalSetID        int64
		evalSetVersionID int64
	}

	groups := make(map[setVersionKey][]*entity.ExptItemRef)
	itemVersionByItemID := make(map[int64]int64, len(refs))
	for _, ref := range refs {
		if ref == nil {
			continue
		}
		if ref.EvalSetID <= 0 {
			return nil, nil, errorx.New("invalid expt_item_ref eval_set_id, expt_id: %v, item_id: %v", ref.ExptID, ref.ItemID)
		}
		key := setVersionKey{evalSetID: ref.EvalSetID, evalSetVersionID: ref.EvalSetVersionID}
		groups[key] = append(groups[key], ref)
		if ref.ItemVersionID > 0 {
			itemVersionByItemID[ref.ItemID] = ref.ItemVersionID
		}
	}

	items := make([]*entity.EvaluationSetItem, 0, len(refs))
	for key, group := range groups {
		queries := make([]*entity.EvaluationItemVersionRef, 0, len(group))
		queryItemIDs := make([]int64, 0, len(group))
		for _, ref := range group {
			queries = append(queries, &entity.EvaluationItemVersionRef{ItemID: ref.ItemID, ItemVersionID: itemVersionIDPtr(ref.ItemVersionID)})
			queryItemIDs = append(queryItemIDs, ref.ItemID)
		}
		logs.CtxInfo(ctx, "fetchEvaluationSetItemsByRefs from expt_item_ref, space_id: %v, eval_set_id: %v, eval_set_version_id: %v, item_ids: %v", spaceID, key.evalSetID, key.evalSetVersionID, queryItemIDs)

		var got []*entity.EvaluationSetItem
		if err := backoff.RetryThreeSeconds(ctx, func() error {
			var retryErr error
			got, retryErr = deps.evaluationSetItemService.BatchGetEvaluationSetItems(ctx, &entity.BatchGetEvaluationSetItemsParam{
				SpaceID:            spaceID,
				EvaluationSetID:    key.evalSetID,
				VersionID:          resolveSetReadVersionID(key.evalSetID, key.evalSetVersionID),
				ItemVersionQueries: queries,
			})
			return retryErr
		}); err != nil {
			return nil, nil, err
		}
		items = append(items, got...)
	}

	return items, itemVersionByItemID, nil
}

func resetRetryRunLogsForItems(ctx context.Context, deps retryItemResetDeps, event *entity.ExptScheduleEvent, items []*entity.EvaluationSetItem, itemVersionByItemID map[int64]int64) ([]int64, error) {
	if len(items) == 0 {
		return nil, nil
	}

	turnCnt := 0
	for _, item := range items {
		turnCnt += len(item.Turns)
	}

	ids, err := deps.idgenerator.GenMultiIDs(ctx, len(items)+turnCnt)
	if err != nil {
		return nil, err
	}

	idIdx := 0
	itemIDs := make([]int64, 0, len(items))
	itemIDSet := make(map[int64]bool, len(items))
	itemTurnIDs := make([]*entity.ItemTurnID, 0, turnCnt)
	itemRunLogs := make([]*entity.ExptItemResultRunLog, 0, len(items))

	for _, item := range items {
		if item == nil {
			continue
		}
		if !itemIDSet[item.ItemID] {
			itemIDs = append(itemIDs, item.ItemID)
			itemIDSet[item.ItemID] = true
		}
		itemVersionID := gptr.Indirect(item.ItemVersionID)
		if itemVersionID == 0 {
			itemVersionID = itemVersionByItemID[item.ItemID]
		}
		itemRunLogs = append(itemRunLogs, &entity.ExptItemResultRunLog{
			ID:            ids[idIdx],
			SpaceID:       event.SpaceID,
			ExptID:        event.ExptID,
			ExptRunID:     event.ExptRunID,
			ItemID:        item.ItemID,
			ItemVersionID: itemVersionID,
			Status:        int32(entity.ItemRunState_Queueing),
		})
		idIdx++

		for _, turn := range item.Turns {
			itemTurnIDs = append(itemTurnIDs, &entity.ItemTurnID{ItemID: item.ItemID, TurnID: turn.ID})
		}
	}

	if len(itemIDs) == 0 {
		return nil, nil
	}
	if err := deps.exptItemResultRepo.UpdateItemsResult(ctx, event.SpaceID, event.ExptID, itemIDs, map[string]any{
		"status":      int32(entity.ItemRunState_Queueing),
		"expt_run_id": event.ExptRunID,
	}); err != nil {
		return nil, err
	}
	if err := deps.exptTurnResultRepo.UpdateTurnResults(ctx, event.ExptID, itemTurnIDs, event.SpaceID, map[string]any{
		"status":           int32(entity.TurnRunState_Queueing),
		"target_result_id": int64(0),
	}); err != nil {
		return nil, err
	}
	if err := clearExptTurnRunLogResultRefsOnItems(ctx, deps.exptTurnResultRepo, event.SpaceID, event.ExptID, event.ExptRunID, itemIDs); err != nil {
		return nil, err
	}
	if err := deps.exptItemResultRepo.BatchCreateNXRunLogs(ctx, itemRunLogs); err != nil {
		return nil, err
	}
	return itemIDs, nil
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
	exptItemRefRepo ...repo.IExptItemRefRepo,
) *ExptRetryAllExec {
	exec := &ExptRetryAllExec{
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
	if len(exptItemRefRepo) > 0 {
		exec.exptItemRefRepo = exptItemRefRepo[0]
	}
	return exec
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
	exptItemRefRepo          repo.IExptItemRefRepo
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
	if expt.EvalSetSourceType == entity.ExptEvalSetSourceType_MultiSetConfig {
		return e.exptStartMultiSet(ctx, event, expt, idemKey)
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
				VersionID:       resolveSetReadVersionID(evalSetID, evalSetVersionID),
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

		if (total > 0 && itemCnt >= int(total)) || len(items) == 0 || pageToken == nil || *pageToken == "" {
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
	exptItemRefRepo ...repo.IExptItemRefRepo,
) *ExptRetryItemsExec {
	exec := &ExptRetryItemsExec{
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
	if len(exptItemRefRepo) > 0 {
		exec.exptItemRefRepo = exptItemRefRepo[0]
	}
	return exec
}

func (e *ExptRetryAllExec) exptStartMultiSet(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment, idemKey string) error {
	if e.exptItemRefRepo == nil {
		return errorx.New("exptItemRefRepo is nil, cannot retry all multiset expt, expt_id=%d", event.ExptID)
	}

	deps := retryItemResetDeps{
		evaluationSetItemService: e.evaluationSetItemService,
		exptItemResultRepo:       e.exptItemResultRepo,
		exptTurnResultRepo:       e.exptTurnResultRepo,
		idgenerator:              e.idgenerator,
	}

	var cursor int64
	const limit int64 = 100
	for loop := 0; loop < 10000; loop++ {
		refs, nextCursor, err := e.exptItemRefRepo.ListByExptID(ctx, event.SpaceID, event.ExptID, cursor, limit)
		if err != nil {
			return err
		}
		if len(refs) == 0 {
			break
		}

		items, itemVersionByItemID, err := fetchEvaluationSetItemsByRefs(ctx, deps, event.SpaceID, refs)
		if err != nil {
			return err
		}
		if _, err := resetRetryRunLogsForItems(ctx, deps, event, items, itemVersionByItemID); err != nil {
			return err
		}

		if nextCursor == 0 {
			break
		}
		cursor = nextCursor
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

	if err := e.exptRepo.Update(ctx, &entity.Experiment{Status: entity.ExptStatus_Processing, ID: event.ExptID, SpaceID: event.SpaceID}); err != nil {
		return err
	}
	if e.templateManager != nil {
		var templateID int64
		if expt.ExptTemplateMeta != nil && expt.ExptTemplateMeta.ID > 0 {
			templateID = expt.ExptTemplateMeta.ID
		}
		if templateID > 0 {
			if err := e.templateManager.UpdateExptInfo(ctx, templateID, event.SpaceID, event.ExptID, entity.ExptStatus_Processing, 0, nil); err != nil {
				logs.CtxError(ctx, "UpdateExptInfo failed in ExptRetryAllExec.exptStartMultiSet, template_id: %v, expt_id: %v, err: %v", templateID, event.ExptID, err)
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
	exptItemRefRepo          repo.IExptItemRefRepo
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
	if expt.EvalSetSourceType == entity.ExptEvalSetSourceType_MultiSetConfig {
		return e.resetEvalItemsMultiSet(ctx, event, itemIDs, got)
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
			VersionID:       resolveSetReadVersionID(evalSetID, evalSetVersionID),
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

func (e *ExptRetryItemsExec) resetEvalItemsMultiSet(ctx context.Context, event *entity.ExptScheduleEvent, itemIDs []int64, got *entity.ExptStats) error {
	if len(itemIDs) == 0 {
		return nil
	}
	if e.exptItemRefRepo == nil {
		return errorx.New("exptItemRefRepo is nil, cannot retry multiset items, expt_id=%d", event.ExptID)
	}

	deps := retryItemResetDeps{
		evaluationSetItemService: e.evaluationSetItemService,
		exptItemResultRepo:       e.exptItemResultRepo,
		exptTurnResultRepo:       e.exptTurnResultRepo,
		idgenerator:              e.idgenerator,
	}
	const pageSize = int32(100)
	for _, chunk := range gslice.Chunk(itemIDs, int(pageSize)) {
		refs, err := e.exptItemRefRepo.MGetByExptIDAndItemIDs(ctx, event.SpaceID, event.ExptID, chunk)
		if err != nil {
			return err
		}
		if len(refs) != len(chunk) {
			return errorx.New("expt_item_ref missing for retry items, expt_id: %v, expected: %v, got: %v", event.ExptID, len(chunk), len(refs))
		}

		items, itemVersionByItemID, err := fetchEvaluationSetItemsByRefs(ctx, deps, event.SpaceID, refs)
		if err != nil {
			return err
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

		if _, err := resetRetryRunLogsForItems(ctx, deps, event, items, itemVersionByItemID); err != nil {
			return err
		}
		time.Sleep(time.Millisecond * 30)
	}

	if err := e.exptStatsRepo.Save(ctx, got); err != nil {
		return err
	}
	logs.CtxInfo(ctx, "ExptRetryItemsExec.resetEvalItemsMultiSet reset stat: %v, expt_id: %v", json.Jsonify(got), event.ExptID)
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

// =====================================================================================
// ★ exptStartMultiSet: MultiSetConfig 新路径 — 从 eval_conf.EvalSetConfigs 扁平化到 expt_item_ref
// =====================================================================================

// exptStartMultiSet 实现多评测集模式的首次调度:
//  1. 遍历 eval_conf.EvalSetConfigs
//  2. 按 set 分页拉取 item 列表 (adapter 层 ItemVersionID=0)
//  3. 每 item 构建 ExptItemRef (item_config 含该 set 的 evaluator/target 配置)
//  4. 批量写入 expt_item_ref
//  5. 批量创建 expt_item_result + expt_turn_result
func (e *ExptSubmitExec) exptStartMultiSet(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment) error {
	if e.exptItemRefRepo == nil {
		return errorx.New("exptItemRefRepo is nil, cannot run multi-set ExptStart")
	}
	evalConf := expt.EvalConf
	if evalConf == nil || len(evalConf.EvalSetConfigs) == 0 {
		return errorx.New("exptStartMultiSet: no eval_set_configs in eval_conf, expt_id=%d", expt.ID)
	}

	const pageSize = int32(100)
	pageSizePtr := pageSize
	itemIdx := int32(0)
	totalItemCnt := 0

	for _, setConf := range evalConf.EvalSetConfigs {
		if setConf == nil {
			continue
		}

		// 构建该 set 下每 item 共享的 item_config (per-set 级配置下沉到行)
		baseItemConfig := buildItemConfigFromSetConf(setConf)

		// 草稿哨兵: 草稿集读侧走 live (VersionID=nil), ref 落 0; committed 走 ByVersion 冻结。
		setReadVersionID := resolveSetReadVersionID(setConf.EvalSetID, setConf.EvalSetVersionID)
		setRefVersionID := resolveSetRefVersionID(setConf.EvalSetID, setConf.EvalSetVersionID)

		// 每批 item → 建 expt_item_ref / item_result / turn_result 并落库 (List 分页 / BatchGet 点选共用)
		persistBatch := func(items []*entity.EvaluationSetItem) error {
			if len(items) == 0 {
				return nil
			}
			itemRefs := make([]*entity.ExptItemRef, 0, len(items))
			eirs := make([]*entity.ExptItemResult, 0, len(items))
			var allTurns []*entity.ExptTurnResult

			turnCnt := 0
			for _, item := range items {
				turnCnt += len(item.Turns)
			}

			ids, err := e.idgenerator.GenMultiIDs(ctx, len(items)*2+turnCnt)
			if err != nil {
				return err
			}
			idIdx := 0

			for _, item := range items {
				// item 级版本: 新数据集 item.ItemVersionID 非 nil → 写真值; 老数据集 nil → 0 (无版本)
				itemVerID := gptr.Indirect(item.ItemVersionID)
				// ExptItemRef
				ref := &entity.ExptItemRef{
					ID:               ids[idIdx],
					SpaceID:          event.SpaceID,
					ExptID:           event.ExptID,
					ItemID:           item.ItemID,
					ItemVersionID:    itemVerID,
					EvalSetID:        setConf.EvalSetID,
					EvalSetVersionID: setRefVersionID,
					ItemConfig:       baseItemConfig,
					OrderIdx:         itemIdx,
				}
				itemRefs = append(itemRefs, ref)
				idIdx++

				// ExptItemResult
				eir := &entity.ExptItemResult{
					ID:            ids[idIdx],
					SpaceID:       event.SpaceID,
					ExptID:        event.ExptID,
					ExptRunID:     event.ExptRunID,
					ItemID:        item.ItemID,
					ItemVersionID: itemVerID,
					ItemIdx:       itemIdx,
					Status:        entity.ItemRunState_Queueing,
				}
				eirs = append(eirs, eir)
				itemIdx++
				idIdx++

				// ExptTurnResult (按 item_config.turn_indexes 过滤; 暂无 turn_indexes 则全量)
				for turnIdx, turn := range item.Turns {
					etr := &entity.ExptTurnResult{
						ID:            ids[idIdx],
						SpaceID:       event.SpaceID,
						ExptID:        event.ExptID,
						ExptRunID:     event.ExptRunID,
						ItemID:        item.ItemID,
						ItemVersionID: itemVerID,
						TurnID:        turn.ID,
						TurnIdx:       int32(turnIdx),
						Status:        int32(entity.TurnRunState_Queueing),
					}
					allTurns = append(allTurns, etr)
					idIdx++
				}
			}

			// 批量写入 expt_item_ref
			if err := e.exptItemRefRepo.BatchCreate(ctx, itemRefs); err != nil {
				return errorx.Wrapf(err, "exptStartMultiSet BatchCreate expt_item_ref fail, expt_id=%d", event.ExptID)
			}
			// 批量写入 expt_item_result + expt_turn_result
			if err := e.createItemTurnResults(ctx, eirs, allTurns, event.Session); err != nil {
				return err
			}
			totalItemCnt += len(items)
			return nil
		}

		// 解析 set 级 item_filter:
		//   - item_id 点选 (in/eq/not_in/not_eq) → 统一走 List 全集分页 + 内存过滤 (include 白名单 / exclude 黑名单)
		//   - 普通列 → 下游 Filter (commercial 走 ml_flow 服务端裁剪; 开源版无字段降级全量)
		//   - tag → 下游 TagFilter (同上)
		//
		// item_id 点选为何不走 BatchGet by-version-queries:
		//   versioned_item + committed 版本下, BatchGet 侧 handleByItemVersionQueries 强制每个 ref 带 item_version_id,
		//   而首次扫描只有用户点选的 item_id、拿不到 item 级版本 → 报 601100201。
		//   List 路径由下游从 snapshot 解析并回填 item_version_id, 故点选降级走 List + 内存 include 过滤即可拿到版本。
		//   TODO: 下游 List 暴露候选 item_id (candidateItemIDs) 入参后, 改服务端 item_id IN 下推, 省整集遍历。
		includeIDs, excludeIDs, _, ferr := extractItemIDFilter(setConf.ItemFilter)
		if ferr != nil {
			return ferr
		}
		nFilter := extractNormalColumnFilter(setConf.ItemFilter)
		tFilter := extractTagFilter(setConf.ItemFilter)
		excludeSet := make(map[int64]struct{}, len(excludeIDs))
		for _, id := range excludeIDs {
			excludeSet[id] = struct{}{}
		}
		includeSet := make(map[int64]struct{}, len(includeIDs))
		for _, id := range includeIDs {
			includeSet[id] = struct{}{}
		}

		// item_id 点选/排除 + 普通列/tag: List 分页, Filter/TagFilter 下传服务端裁剪, item_id 在内存过滤
		var pageToken *string
		pageTotalCnt := 0
		for loop := 0; loop < 10000; loop++ {
			logs.CtxInfo(ctx, "exptStartMultiSet scan item, expt_id=%d, set_id=%d, set_ver_id=%d, page_token=%v",
				event.ExptID, setConf.EvalSetID, setConf.EvalSetVersionID, gptr.Indirect(pageToken))

			var items []*entity.EvaluationSetItem
			var total *int64
			var nextPageToken *string
			if err := backoff.RetryThreeSeconds(ctx, func() error {
				var retryErr error
				items, total, _, nextPageToken, retryErr = e.evaluationSetItemService.ListEvaluationSetItems(ctx, &entity.ListEvaluationSetItemsParam{
					SpaceID:         event.SpaceID,
					EvaluationSetID: setConf.EvalSetID,
					VersionID:       setReadVersionID,
					PageSize:        &pageSizePtr,
					PageToken:       pageToken,
					Filter:          nFilter,
					TagFilter:       tFilter,
				})
				return retryErr
			}); err != nil {
				return err
			}

			pageTotalCnt += len(items)
			// item_id 点选/排除内存过滤 (每页 pageSize 逐页处理, 非全集进内存, set 只存 id):
			//   - include (in/eq): 只保留白名单内的 item_id
			//   - exclude (not_in/not_eq): 剔除黑名单内的 item_id
			// item 级版本由下游 List 从 snapshot 回填 (persistBatch 从 item.ItemVersionID 写库), 故此路径无 601100201 风险。
			// 下游既无 item_id 候选/排除专用字段, 暂只能内存过滤; 代价是点选/排除仍要分页遍历整集。
			// TODO: 下游 List 就绪 candidateItemIDs 后改服务端 item_id IN/NOT IN 下推, 省整集遍历。
			if len(includeSet) > 0 || len(excludeSet) > 0 {
				kept := items[:0]
				for _, it := range items {
					if len(includeSet) > 0 {
						if _, in := includeSet[it.ItemID]; !in {
							continue
						}
					}
					if _, ex := excludeSet[it.ItemID]; ex {
						continue
					}
					kept = append(kept, it)
				}
				items = kept
			}
			if err := persistBatch(items); err != nil {
				return err
			}

			pageToken = nextPageToken
			if (gptr.Indirect(total) > 0 && int64(pageTotalCnt) >= gptr.Indirect(total)) || len(items) == 0 || pageToken == nil || *pageToken == "" {
				break
			}
		}
	}

	return e.finishExptStart(ctx, event, expt, totalItemCnt, makeStartIdemKey(event))
}

// isDraftEvalSet 判定该 set 引用的是否为「草稿 / 不锁版本」。
//
// 草稿哨兵约定 (与执行侧 expt_run_item_event_impl.go BuildExptRecordEvalCtx 一致):
//   - EvalSetVersionID == 0            : 显式不锁版本 (虽然当前提交校验挡 0, 仍兜底)
//   - EvalSetVersionID == EvalSetID    : 提交侧为绕过「version_id 必填」用 set_id 当占位的草稿哨兵
//
// committed (已提交不可变) 版本: EvalSetVersionID 为真实 version_id (≠ set_id 且 ≠ 0)。
func isDraftEvalSet(evalSetID, evalSetVersionID int64) bool {
	return evalSetVersionID == 0 || evalSetVersionID == evalSetID
}

// resolveSetReadVersionID 计算扫描层拉取 item 时下传给读侧的集级 VersionID。
//   - 草稿: 返回 nil → 读侧走 BatchGetDatasetItems/ListDatasetItems (live, 读当前 dataset_item 草稿)
//   - committed: 返回 &version → 读侧走 ...ByVersion (冻结快照, 不可变, 行为完全不变)
func resolveSetReadVersionID(evalSetID, evalSetVersionID int64) *int64 {
	if isDraftEvalSet(evalSetID, evalSetVersionID) {
		return nil
	}
	return &evalSetVersionID
}

// resolveSetRefVersionID 计算落 expt_item_ref.eval_set_version_id 的值。
// 草稿落 0 (与老路径 ItemVersionID=0 口径一致, 全链路下游据此识别草稿 → live 读),
// committed 落真实 version_id (调度键, 配合 item_id 定位 dataset_item_snapshot 冻结快照)。
func resolveSetRefVersionID(evalSetID, evalSetVersionID int64) int64 {
	if isDraftEvalSet(evalSetID, evalSetVersionID) {
		return 0
	}
	return evalSetVersionID
}

// buildItemConfigFromSetConf 将 per-set 配置下沉为 ExptItemConfig (同 set 所有 item 共享)
func buildItemConfigFromSetConf(setConf *entity.EvalSetConfig) *entity.ExptItemConfig {
	cfg := &entity.ExptItemConfig{}

	// evaluator_conf
	for _, ec := range setConf.EvaluatorConfs {
		if ec == nil {
			continue
		}
		itemEv := &entity.ItemEvaluatorConf{
			EvaluatorVersionID: ec.EvaluatorVersionID,
			Alias:              ec.Alias,
			FromEvalSet:        ec.FromEvalSet,
			FromTarget:         ec.FromTarget,
			DynamicParam:       ec.RuntimeParam,
			Filter:             ec.Filter,
			FilterMode:         ec.FilterMode,
			ScoreWeight:        ec.ScoreWeight,
		}
		cfg.EvaluatorConfs = append(cfg.EvaluatorConfs, itemEv)
	}

	// eval_target_conf (本期只取第一个 target conf)
	if len(setConf.TargetConfs) > 0 && setConf.TargetConfs[0] != nil {
		tc := setConf.TargetConfs[0]
		cfg.EvalTargetConf = &entity.ItemTargetConf{
			TargetVersionID: tc.TargetVersionID,
			FieldMapping:    tc.FieldMapping,
			DynamicConf:     tc.RuntimeParam,
		}
	}

	return cfg
}

// extractItemIDFilter 从 set 级 ItemFilter 解析 item_id 点选条件。
// 仅处理 field_name=item_id, field_type=long: in/eq → include; not_in/not_eq → exclude。
// tag 圈选 (field_type=tag) 本期不在此消费 (执行侧裁剪未接, 见 tech debt), 返回 hasTagFilter=true 供上层 warn。
// 校验白名单已在 ValidateEvalSetConfigs 保证 field_type/query_type 合法, 这里只解析。
func extractItemIDFilter(f *entity.ExptItemFilter) (includeIDs, excludeIDs []int64, hasTagFilter bool, err error) {
	if f == nil || len(f.FilterFields) == 0 {
		return nil, nil, false, nil
	}
	for _, ff := range f.FilterFields {
		if ff == nil {
			continue
		}
		if ff.FieldType == "tag" {
			hasTagFilter = true
			continue
		}
		if ff.FieldName != "item_id" {
			continue
		}
		ids := make([]int64, 0, len(ff.Values))
		for _, v := range ff.Values {
			id, perr := strconv.ParseInt(v, 10, 64)
			if perr != nil {
				return nil, nil, hasTagFilter, errorx.New("extractItemIDFilter: invalid item_id %q, err=%v", v, perr)
			}
			ids = append(ids, id)
		}
		switch ff.QueryType {
		case "in", "eq":
			includeIDs = append(includeIDs, ids...)
		case "not_in", "not_eq":
			excludeIDs = append(excludeIDs, ids...)
		}
	}
	return includeIDs, excludeIDs, hasTagFilter, nil
}

// extractNormalColumnFilter 从 set 级 ItemFilter 抽出普通业务列条件 (非 item_id、非 tag),
// 组成下游 entity.Filter (= data_filter.Filter)。无普通列字段时返回 nil。
//
// 下游 commercial adapter 透传给 ml_flow 做服务端裁剪; 开源版下游无 filter 字段会丢弃 (降级全量)。
func extractNormalColumnFilter(f *entity.ExptItemFilter) *entity.Filter {
	if f == nil || len(f.FilterFields) == 0 {
		return nil
	}
	fields := make([]*entity.FilterField, 0, len(f.FilterFields))
	for _, ff := range f.FilterFields {
		if ff == nil {
			continue
		}
		if ff.FieldType == "tag" || ff.FieldName == "item_id" {
			continue
		}
		field := &entity.FilterField{
			FieldName: ff.FieldName,
			FieldType: ff.FieldType,
			Values:    ff.Values,
		}
		if ff.QueryType != "" {
			field.QueryType = gptr.Of(ff.QueryType)
		}
		fields = append(fields, field)
	}
	if len(fields) == 0 {
		return nil
	}
	out := &entity.Filter{FilterFields: fields}
	if f.QueryAndOr != "" {
		out.QueryAndOr = gptr.Of(f.QueryAndOr)
	}
	return out
}

// extractTagFilter 从 set 级 ItemFilter 抽出 tag 条件 (field_type=tag),
// 把各 tag field 的 values 扁平收集成下游 entity.TagFilter{TagNames, Relation}。无 tag 时返回 nil。
//
// Relation 由 ItemFilter.QueryAndOr 映射 (and→And, 其余→Or, 与下游 TagFilter 默认一致)。
func extractTagFilter(f *entity.ExptItemFilter) *entity.TagFilter {
	if f == nil || len(f.FilterFields) == 0 {
		return nil
	}
	var tagNames []string
	for _, ff := range f.FilterFields {
		if ff == nil || ff.FieldType != "tag" {
			continue
		}
		tagNames = append(tagNames, ff.Values...)
	}
	if len(tagNames) == 0 {
		return nil
	}
	relation := entity.TagFilterRelationOr
	if f.QueryAndOr == "and" {
		relation = entity.TagFilterRelationAnd
	}
	return &entity.TagFilter{TagNames: tagNames, Relation: relation}
}
