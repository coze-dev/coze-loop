// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/bytedance/gg/gcond"
	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/goroutine"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/maps"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func NewExptResultService(
	exptItemResultRepo repo.IExptItemResultRepo,
	exptTurnResultRepo repo.IExptTurnResultRepo,
	exptStatsRepo repo.IExptStatsRepo,
	experimentRepo repo.IExperimentRepo,
	metric metrics.ExptMetric,
	lwt platestwrite.ILatestWriteTracker,
	idgen idgen.IIDGenerator,
	exptTurnResultFilterRepo repo.IExptTurnResultFilterRepo,
	evaluatorService EvaluatorService,
	evalTargetService IEvalTargetService,
	evaluationSetVersionService EvaluationSetVersionService,
	evaluationSetService IEvaluationSetService,
	evaluatorRecordService EvaluatorRecordService,
	evaluationSetItemService EvaluationSetItemService,
	publisher events.ExptEventPublisher,
) ExptResultService {
	return ExptResultServiceImpl{
		ExptItemResultRepo:          exptItemResultRepo,
		ExptTurnResultRepo:          exptTurnResultRepo,
		ExptStatsRepo:               exptStatsRepo,
		ExperimentRepo:              experimentRepo,
		Metric:                      metric,
		lwt:                         lwt,
		idgen:                       idgen,
		exptTurnResultFilterRepo:    exptTurnResultFilterRepo,
		evalTargetService:           evalTargetService,
		evaluationSetVersionService: evaluationSetVersionService,
		evaluationSetService:        evaluationSetService,
		evaluatorService:            evaluatorService,
		evaluatorRecordService:      evaluatorRecordService,
		evaluationSetItemService:    evaluationSetItemService,
		publisher:                   publisher,
	}
}

type ExptResultServiceImpl struct {
	ExptItemResultRepo       repo.IExptItemResultRepo
	ExptTurnResultRepo       repo.IExptTurnResultRepo
	ExptStatsRepo            repo.IExptStatsRepo
	ExperimentRepo           repo.IExperimentRepo
	Metric                   metrics.ExptMetric
	lwt                      platestwrite.ILatestWriteTracker
	idgen                    idgen.IIDGenerator
	exptTurnResultFilterRepo repo.IExptTurnResultFilterRepo

	evalTargetService           IEvalTargetService
	evaluationSetVersionService EvaluationSetVersionService
	evaluationSetService        IEvaluationSetService
	evaluatorService            EvaluatorService
	evaluatorRecordService      EvaluatorRecordService
	evaluationSetItemService    EvaluationSetItemService

	publisher events.ExptEventPublisher
}

func (e ExptResultServiceImpl) GetExptItemTurnResults(ctx context.Context, exptID, itemID int64, spaceID int64, session *entity.Session) ([]*entity.ExptTurnResult, error) {
	turnResults, err := e.ExptTurnResultRepo.GetItemTurnResults(ctx, exptID, itemID, spaceID)
	if err != nil {
		return nil, err
	}

	turnResultIDs := make([]int64, 0, len(turnResults))
	for _, tr := range turnResults {
		turnResultIDs = append(turnResultIDs, tr.ID)
	}
	refs, err := e.ExptTurnResultRepo.BatchGetTurnEvaluatorResultRef(ctx, spaceID, turnResultIDs)
	if err != nil {
		return nil, err
	}

	turnEvaluatorVerIDToResultID := make(map[int64]map[int64]int64, len(turnResults))
	for _, ref := range refs {
		if turnEvaluatorVerIDToResultID[ref.ExptTurnResultID] == nil {
			turnEvaluatorVerIDToResultID[ref.ExptTurnResultID] = make(map[int64]int64)
		}
		turnEvaluatorVerIDToResultID[ref.ExptTurnResultID][ref.EvaluatorVersionID] = ref.EvaluatorVersionID
	}

	res := make([]*entity.ExptTurnResult, 0, len(turnResults))
	for _, tr := range turnResults {
		evalVerID2ResultID := turnEvaluatorVerIDToResultID[tr.ID]
		tr.EvaluatorResults = &entity.EvaluatorResults{EvalVerIDToResID: evalVerID2ResultID}
		res = append(res, tr)
	}

	return res, nil
}

func (e ExptResultServiceImpl) RecordItemRunLogs(ctx context.Context, exptID, exptRunID int64, itemID int64, spaceID int64) ([]*entity.ExptTurnEvaluatorResultRef, error) {
	itemRunLog, err := e.ExptItemResultRepo.GetItemRunLog(ctx, exptID, exptRunID, itemID, spaceID)
	if err != nil {
		return nil, err
	}

	turnRunLogs, err := e.ExptTurnResultRepo.GetItemTurnRunLogs(ctx, exptID, exptRunID, itemID, spaceID)
	if err != nil {
		return nil, err
	}

	turnResults, err := e.ExptItemResultRepo.GetItemTurnResults(ctx, spaceID, exptID, itemID)
	if err != nil {
		return nil, err
	}

	itemResults, err := e.ExptItemResultRepo.BatchGet(ctx, spaceID, exptID, []int64{itemID})
	if err != nil {
		return nil, err
	}

	itemResult := itemResults[0]

	statsCntOp := &entity.StatsCntArithOp{OpStatusCnt: make(map[entity.ItemRunState]int)}
	statsCntOp.OpStatusCnt[itemResult.Status] = statsCntOp.OpStatusCnt[itemResult.Status] - 1
	statsCntOp.OpStatusCnt[entity.ItemRunState(itemRunLog.Status)] = statsCntOp.OpStatusCnt[entity.ItemRunState(itemRunLog.Status)] + 1
	turn2RunLog := make(map[int64]*entity.ExptTurnResultRunLog, len(turnRunLogs))
	for _, trl := range turnRunLogs {
		turn2RunLog[trl.TurnID] = trl
	}

	logs.CtxInfo(ctx, "[ExptEval] expt item result with recording run_log, expt_id=%v, expt_run_id=%v, item_id=%v, cnt_op: %v", exptID, exptRunID, itemID, json.Jsonify(statsCntOp))

	var (
		turnEvaluatorRefs []*entity.ExptTurnEvaluatorResultRef
		turn2Result       = gslice.ToMap(turnResults, func(t *entity.ExptTurnResult) (int64, *entity.ExptTurnResult) { return t.TurnID, t })
	)

	for tid, result := range turn2Result {
		rl := turn2RunLog[tid]
		if rl == nil {
			return nil, fmt.Errorf("RecordItemRunLogs found null turn log result, expt_id: %v, expt_run_id: %v, item: %v, tid: %v", exptID, exptRunID, itemID, tid)
		}

		result.Status = int32(rl.Status)
		result.TargetResultID = rl.TargetResultID
		result.ErrMsg = rl.ErrMsg
		result.LogID = rl.LogID
		result.ExptRunID = rl.ExptRunID

		turnEvaluatorRefs = append(turnEvaluatorRefs, NewTurnEvaluatorResultRefs(0, result.ExptID, result.ID, spaceID, rl.EvaluatorResultIds)...)
	}

	if len(turnEvaluatorRefs) > 0 {
		ids, err := e.idgen.GenMultiIDs(ctx, len(turnEvaluatorRefs))
		if err != nil {
			return nil, err
		}

		for idx, ref := range turnEvaluatorRefs {
			ref.ID = ids[idx]
		}

		if err := e.ExptTurnResultRepo.CreateTurnEvaluatorRefs(ctx, turnEvaluatorRefs); err != nil {
			return nil, err
		}
	}

	if err := e.ExptTurnResultRepo.SaveTurnResults(ctx, turnResults); err != nil {
		return nil, err
	}

	if err := e.ExptItemResultRepo.UpdateItemsResult(ctx, spaceID, exptID, []int64{itemID}, map[string]any{
		"status":  itemRunLog.Status,
		"log_id":  itemRunLog.LogID,
		"err_msg": itemRunLog.ErrMsg,
	}); err != nil {
		return nil, err
	}

	if err := e.ExptItemResultRepo.UpdateItemRunLog(ctx, exptID, exptRunID, []int64{itemID}, map[string]any{
		"result_state": int32(entity.ExptItemResultStateResulted),
	}, spaceID); err != nil {
		return nil, err
	}

	if err := e.ExptStatsRepo.ArithOperateCount(ctx, exptID, spaceID, statsCntOp); err != nil {
		return nil, err
	}

	return turnEvaluatorRefs, nil
}

func NewTurnEvaluatorResultRefs(id, exptID, turnResultID, spaceID int64, evaluatorResults *entity.EvaluatorResults) []*entity.ExptTurnEvaluatorResultRef {
	if evaluatorResults == nil {
		return nil
	}

	refs := make([]*entity.ExptTurnEvaluatorResultRef, 0, len(evaluatorResults.EvalVerIDToResID))
	for evalVerID, evalResID := range evaluatorResults.EvalVerIDToResID {
		refs = append(refs, &entity.ExptTurnEvaluatorResultRef{
			ID:                 id,
			ExptID:             exptID,
			SpaceID:            spaceID,
			ExptTurnResultID:   turnResultID,
			EvaluatorVersionID: evalVerID,
			EvaluatorResultID:  evalResID,
		})
	}
	return refs
}

func (e ExptResultServiceImpl) MGetExperimentResult(ctx context.Context, param *entity.MGetExperimentResultParam) (
	columnEvaluators []*entity.ColumnEvaluator, columnEvalSetFields []*entity.ColumnEvalSetField, itemResults []*entity.ItemResult, total int64, err error,
) {
	var (
		spaceID        = param.SpaceID
		exptIDs        = param.ExptIDs
		baselineExptID = param.BaseExptID
		turnResultDAOs []*entity.ExptTurnResult
	)

	defer e.Metric.EmitGetExptResult(spaceID, err != nil)

	if len(exptIDs) == 1 && e.lwt.CheckWriteFlagByID(ctx, platestwrite.ResourceTypeExperiment, exptIDs[0]) {
		ctx = contexts.WithCtxWriteDB(ctx)
	}

	var baseExptID int64
	if baselineExptID != nil {
		baseExptID = *baselineExptID
	}
	// 只有一个实验，且没有指定baseline
	if len(exptIDs) == 1 && baselineExptID == nil {
		baseExptID = exptIDs[0]
	}

	baseExpt, err := e.ExperimentRepo.GetByID(ctx, baseExptID, spaceID)
	if err != nil {
		return nil, nil, nil, 0, err
	}
	baseExptEvalSetVersionID := baseExpt.EvalSetVersionID

	columnEvaluators, err = e.getColumnEvaluators(ctx, spaceID, exptIDs)
	if err != nil {
		return nil, nil, nil, 0, err
	}

	columnEvalSetFields, err = e.getColumnEvalSetFields(ctx, spaceID, baseExpt.EvalSetID, baseExptEvalSetVersionID)
	if err != nil {
		return nil, nil, nil, 0, err
	}

	if baseExpt.ExptType == entity.ExptType_Online && len(exptIDs) > 1 {
		// 在线实验对比场景，不返回行级结果
		return columnEvaluators, columnEvalSetFields, nil, 0, nil
	}

	// 获取baseline 该分页的turn_result
	turnResultDAOs, itemID2ItemRunState, total, err := e.ListTurnResult(ctx, param, baseExpt)
	if err != nil {
		return nil, nil, nil, 0, err
	}

	if total == 0 {
		return columnEvaluators, columnEvalSetFields, nil, 0, nil
	}

	itemIDMap := make(map[int64]bool)
	for _, turnResult := range turnResultDAOs {
		itemIDMap[turnResult.ItemID] = true
	}
	itemIDs := maps.ToSlice(itemIDMap, func(k int64, v bool) int64 {
		return k
	})
	itemResultDAOs, err := e.ExptItemResultRepo.BatchGet(ctx, spaceID, baseExptID, itemIDs)
	if err != nil {
		return nil, nil, nil, 0, err
	}

	payloadBuilder := NewPayloadBuilder(ctx, param, baseExptID, turnResultDAOs, itemResultDAOs, e.ExperimentRepo,
		e.ExptTurnResultRepo, e.evalTargetService, e.evaluatorRecordService, e.evaluationSetItemService, nil, nil, itemID2ItemRunState)

	itemResults, err = payloadBuilder.BuildItemResults(ctx)
	if err != nil {
		return nil, nil, nil, 0, err
	}

	return columnEvaluators, columnEvalSetFields, itemResults, total, nil
}

func (e ExptResultServiceImpl) ListTurnResult(ctx context.Context, param *entity.MGetExperimentResultParam, expt *entity.Experiment) (turnResultDAOs []*entity.ExptTurnResult, itemID2ItemRunState map[int64]entity.ItemRunState, totalTurn int64, err error) {
	var (
		spaceID        = param.SpaceID
		baselineExptID = param.BaseExptID
		page           = param.Page
		total          int64
		baseExptID     int64
	)

	if baselineExptID != nil {
		baseExptID = *baselineExptID
	}
	if param.UseAccelerator {
		var filterAccelerator *entity.ExptTurnResultFilterAccelerator
		if len(param.FilterAccelerators) != 0 && param.FilterAccelerators[baseExptID] != nil {
			filterAccelerator = param.FilterAccelerators[baseExptID]
		}
		if filterAccelerator == nil {
			filterAccelerator = &entity.ExptTurnResultFilterAccelerator{}
		}
		filterAccelerator.ExptID = baseExptID
		filterAccelerator.SpaceID = spaceID
		filterAccelerator.CreatedDate = ptr.From(expt.StartAt)
		filterAccelerator.Page = param.Page
		errOccur := false
		if err = e.mapItemSnapshotFilter(ctx, filterAccelerator, expt, expt.EvalSetVersionID); err != nil {
			logs.CtxError(ctx, "mapItemSnapshotFilter failed: %v", err)
			errOccur = true
		}
		if !errOccur {
			if err = e.mapTurnResultFilterCond(ctx, filterAccelerator, spaceID, baseExptID); err != nil {
				logs.CtxError(ctx, "mapTurnResultFilterCond failed: %v", err)
				errOccur = true
			}
		}
		var itemIDs []int64
		if !errOccur {
			startTime := time.Now()

			itemID2ItemRunState, total, err = e.exptTurnResultFilterRepo.QueryItemIDStates(ctx, filterAccelerator)
			e.Metric.EmitExptTurnResultFilterQueryLatency(spaceID, startTime.Unix(), err != nil)
			if err != nil {
				logs.CtxError(ctx, "exptTurnResultFilterRepo QueryItemIDStates failed: %v", err)
				errOccur = true
			} else {
				if len(itemID2ItemRunState) == 0 {
					return nil, nil, 0, nil
				}
				itemIDs = maps.ToSlice(itemID2ItemRunState, func(k int64, v entity.ItemRunState) int64 {
					return k
				})
			}
		}

		// 如果errOccur为true，直接跳过后续filter流程，继续执行ListTurnResult
		if !errOccur {
			page = entity.Page{} // filter表查询后，后续无需再带分页条件
		}
		// 获取baseline 该分页的turn_result
		turnResultDAOs, totalTurn, err = e.ExptTurnResultRepo.ListTurnResultByItemIDs(ctx, spaceID, baseExptID, itemIDs, page, gcond.If(expt.ExptType == entity.ExptType_Online, true, false))
		if err != nil {
			return nil, nil, 0, err
		}
		if errOccur {
			total = totalTurn
		}
		if len(turnResultDAOs) == 0 {
			return nil, nil, 0, nil
		}
	} else {
		var filter *entity.ExptTurnResultFilter
		if len(param.Filters) != 0 && param.Filters[baseExptID] != nil {
			filter = param.Filters[baseExptID]
		}
		turnResultDAOs, total, err = e.ExptTurnResultRepo.ListTurnResult(ctx, spaceID, baseExptID, filter, page, gcond.If(expt.ExptType == entity.ExptType_Online, true, false))
		if err != nil {
			return nil, nil, 0, err
		}

		if len(turnResultDAOs) == 0 {
			return nil, nil, 0, nil
		}

	}
	return turnResultDAOs, itemID2ItemRunState, total, nil
}

// getColumnEvaluators 试验对比无需返回多试验的评估器合集,没有评估器的column,前端从实验接口获取评估器数据
func (e ExptResultServiceImpl) getColumnEvaluators(ctx context.Context, spaceID int64, exptIDs []int64) ([]*entity.ColumnEvaluator, error) {
	evaluatorRef, err := e.ExperimentRepo.GetEvaluatorRefByExptIDs(ctx, exptIDs, spaceID)
	if err != nil {
		return nil, err
	}
	// 去重
	evaluatorVersionIDMap := make(map[int64]bool)
	evaluatorIDMap := make(map[int64]bool)
	versionID2evaluatorID := make(map[int64]int64)
	for _, ref := range evaluatorRef {
		evaluatorVersionIDMap[ref.EvaluatorVersionID] = true
		evaluatorIDMap[ref.EvaluatorID] = true
		versionID2evaluatorID[ref.EvaluatorVersionID] = ref.EvaluatorID
	}

	evaluatorVersionIDs := maps.ToSlice(evaluatorVersionIDMap, func(k int64, v bool) int64 {
		return k
	})

	evaluatorVersions, err := e.evaluatorService.BatchGetEvaluatorVersion(ctx, nil, evaluatorVersionIDs, true)
	if err != nil {
		return nil, err
	}

	columnEvaluators := make([]*entity.ColumnEvaluator, 0)
	for _, e := range evaluatorVersions {
		evaluatorVersion := e.GetEvaluatorVersion()
		if evaluatorVersion == nil || !gslice.Contains(evaluatorVersionIDs, evaluatorVersion.GetID()) {
			continue
		}

		columnEvaluator := &entity.ColumnEvaluator{
			EvaluatorVersionID: evaluatorVersion.GetID(),
			EvaluatorID:        e.ID,
			EvaluatorType:      e.EvaluatorType,
			Name:               gptr.Of(e.Name),
			Version:            gptr.Of(e.GetEvaluatorVersion().GetVersion()),
			Description:        gptr.Of(e.Description),
		}
		columnEvaluators = append(columnEvaluators, columnEvaluator)
	}

	return columnEvaluators, nil
}

func (e ExptResultServiceImpl) getColumnEvalSetFields(ctx context.Context, spaceID int64, evalSetID, evalSetVersionID int64) ([]*entity.ColumnEvalSetField, error) {
	var version *entity.EvaluationSetVersion
	if evalSetID == evalSetVersionID {
		evalSet, err := e.evaluationSetService.GetEvaluationSet(ctx, gptr.Of(spaceID), evalSetID, gptr.Of(true))
		if err != nil {
			return nil, err
		}
		version = evalSet.EvaluationSetVersion
	} else {
		var err error
		version, _, err = e.evaluationSetVersionService.GetEvaluationSetVersion(ctx, spaceID, evalSetVersionID, gptr.Of(true))
		if err != nil {
			return nil, err
		}
	}

	var fieldSchema []*entity.FieldSchema
	if version != nil && version.EvaluationSetSchema != nil {
		fieldSchema = version.EvaluationSetSchema.FieldSchemas
	}

	columnEvalSetFields := make([]*entity.ColumnEvalSetField, 0)
	for _, field := range fieldSchema {
		columnEvalSetFields = append(columnEvalSetFields, &entity.ColumnEvalSetField{
			Key:         gptr.Of(field.Key),
			Name:        gptr.Of(field.Name),
			Description: gptr.Of(field.Description),
			ContentType: field.ContentType,
			TextSchema:  gptr.Of(field.TextSchema),
		})
	}

	return columnEvalSetFields, nil
}

type PayloadBuilder struct {
	BaselineExptID       int64
	SpaceID              int64
	ExptIDs              []int64
	BaseExptTurnResultDO []*entity.ExptTurnResult
	BaseExptItemResultDO []*entity.ExptItemResult

	ItemIDs   []int64
	TurnIDMap map[int64]bool

	ItemResults           []*entity.ItemResult // 最终结果
	ExptTurnResultFilters []*entity.ExptTurnResultFilterEntity
	ExptResultBuilders    []*ExptResultBuilder // 每个实验的结果builder以及build result

	ExperimentRepo     repo.IExperimentRepo
	ExptTurnResultRepo repo.IExptTurnResultRepo

	EvaluationSetItemService                    EvaluationSetItemService
	EvalTargetService                           IEvalTargetService
	EvaluatorRecordService                      EvaluatorRecordService
	ExptTurnResultFilterKeyMappingEvaluatorMap  map[string]*entity.ExptTurnResultFilterKeyMapping
	ExptTurnResultFilterKeyMappingAnnotationMap map[string]*entity.ExptTurnResultFilterKeyMapping
}

func NewPayloadBuilder(ctx context.Context, param *entity.MGetExperimentResultParam, baselineExptID int64, baselineTurnResults []*entity.ExptTurnResult,
	baselineItemResults []*entity.ExptItemResult, experimentRepo repo.IExperimentRepo,
	exptTurnResultRepo repo.IExptTurnResultRepo,
	evalTargetService IEvalTargetService,
	evaluatorRecordService EvaluatorRecordService,
	evaluationSetItemService EvaluationSetItemService,
	exptTurnResultFilterKeyMappingEvaluatorMap map[string]*entity.ExptTurnResultFilterKeyMapping,
	exptTurnResultFilterKeyMappingAnnotationMap map[string]*entity.ExptTurnResultFilterKeyMapping,
	itemID2ItemRunState map[int64]entity.ItemRunState,
) *PayloadBuilder {
	builder := &PayloadBuilder{
		BaselineExptID:           baselineExptID,
		SpaceID:                  param.SpaceID,
		ExptIDs:                  param.ExptIDs,
		BaseExptTurnResultDO:     baselineTurnResults,
		BaseExptItemResultDO:     baselineItemResults,
		ExperimentRepo:           experimentRepo,
		ExptTurnResultRepo:       exptTurnResultRepo,
		EvaluationSetItemService: evaluationSetItemService,
		EvalTargetService:        evalTargetService,
		EvaluatorRecordService:   evaluatorRecordService,
		ExptTurnResultFilterKeyMappingEvaluatorMap:  exptTurnResultFilterKeyMappingEvaluatorMap,
		ExptTurnResultFilterKeyMappingAnnotationMap: exptTurnResultFilterKeyMappingAnnotationMap,
	}

	builder.ItemResults = make([]*entity.ItemResult, 0)

	// 需要分实验获取的数据范围
	itemIDs := make([]int64, 0)                              // itemID列表 有序
	itemID2TurnIDs := make(map[int64][]int64)                // itemID -> turnIDs列表 turnIDs有序
	itemIDMap := make(map[int64]bool)                        // 去重
	itemIDTurnIDTurnIndex := make(map[int64]map[int64]int64) // itemID -> turnID -> turnIndex
	itemIDItemResultPO := make(map[int64]*entity.ExptItemResult)

	turnIDMap := make(map[int64]bool)
	turnID2ItemID := make(map[int64]int64)

	for _, itemResult := range baselineItemResults {
		itemIDItemResultPO[itemResult.ItemID] = itemResult
	}

	for _, turnResultDO := range builder.BaseExptTurnResultDO {
		if _, ok := itemIDMap[turnResultDO.ItemID]; !ok {
			itemIDs = append(itemIDs, turnResultDO.ItemID) // 使用turnResultDO中的itemID append确保item有序
		}
		itemIDMap[turnResultDO.ItemID] = true

		if itemIDTurnIDTurnIndex[turnResultDO.ItemID] == nil {
			itemIDTurnIDTurnIndex[turnResultDO.ItemID] = make(map[int64]int64)
		}
		itemIDTurnIDTurnIndex[turnResultDO.ItemID][turnResultDO.TurnID] = int64(turnResultDO.TurnIdx)

		if turnResultDO.TurnID != 0 {
			turnIDMap[turnResultDO.TurnID] = true
			turnID2ItemID[turnResultDO.TurnID] = turnResultDO.ItemID
		}

		if _, ok := itemID2TurnIDs[turnResultDO.ItemID]; !ok {
			itemID2TurnIDs[turnResultDO.ItemID] = make([]int64, 0)
		}
		itemID2TurnIDs[turnResultDO.ItemID] = append(itemID2TurnIDs[turnResultDO.ItemID], turnResultDO.TurnID)
	}

	builder.ItemIDs = itemIDs
	builder.TurnIDMap = turnIDMap

	// 初始化payload结构
	for _, itemID := range itemIDs {
		if itemIDItemResultPO[itemID] == nil {
			continue
		}
		itemResultPO := itemIDItemResultPO[itemID]

		itemResult := &entity.ItemResult{
			ItemID:      itemID,
			TurnResults: make([]*entity.TurnResult, 0),
			ItemIndex:   gptr.Of(int64(itemResultPO.ItemIdx)),
		}
		if state, ok := itemID2ItemRunState[itemID]; ok {
			itemResult.SystemInfo = &entity.ItemSystemInfo{
				RunState: state,
			}
		} else {
			itemResult.SystemInfo = &entity.ItemSystemInfo{
				RunState: itemResultPO.Status,
			}
		}
		for _, turnID := range itemID2TurnIDs[itemID] {
			turnIndex := int64(0)
			if itemIDTurnIDTurnIndex[itemID] != nil {
				turnIndex = itemIDTurnIDTurnIndex[itemID][turnID]
			}
			itemResult.TurnResults = append(itemResult.TurnResults, &entity.TurnResult{
				TurnID:            turnID,
				ExperimentResults: make([]*entity.ExperimentResult, 0),
				TurnIndex:         gptr.Of(turnIndex),
			})

		}

		builder.ItemResults = append(builder.ItemResults, itemResult)
	}

	return builder
}

// ExptResultBuilder 构建单实验结果
type ExptResultBuilder struct {
	ExptID                    int64
	BaselineExptID            int64
	SpaceID                   int64
	ItemIDs                   []int64        // 基准实验的itemID, 未匹配的不展示
	TurnIDMap                 map[int64]bool // 由于是itemID查询，对于多轮需要用turnID过滤. 对于单轮长度为0
	ItemIDTurnID2TurnResultID map[int64]map[int64]int64

	exptDO       *entity.Experiment
	turnResultDO []*entity.ExptTurnResult

	// 获取的结果
	turnResultID2EvaluatorVersionID2Result map[int64]map[int64]*entity.EvaluatorRecord // turn_result_id -> evaluator_version_id -> result
	turnResultID2TargetOutput              map[int64]*entity.TurnTargetOutput
	itemIDTurnID2Turn                      map[int64]map[int64]*entity.TurnEvalSet
	turnResultID2ScoreCorrected            map[int64]bool

	// 错误信息
	Err error

	ExperimentRepo     repo.IExperimentRepo
	ExptTurnResultRepo repo.IExptTurnResultRepo

	evaluationSetItemService EvaluationSetItemService
	evalTargetService        IEvalTargetService
	evaluatorRecordService   EvaluatorRecordService
}

// 1.确定当前分页下数据范围
// 2.分实验batch get 所需数据
// 3.组装数据
func (b *PayloadBuilder) BuildItemResults(ctx context.Context) ([]*entity.ItemResult, error) {
	// 分实验获取数据
	exptResultBuilders := make([]*ExptResultBuilder, 0)
	for _, exptID := range b.ExptIDs {
		exptResultBuilder := &ExptResultBuilder{
			ExptID:                   exptID,
			BaselineExptID:           b.BaselineExptID,
			SpaceID:                  b.SpaceID,
			ItemIDs:                  b.ItemIDs,
			TurnIDMap:                b.TurnIDMap,
			ExperimentRepo:           b.ExperimentRepo,
			ExptTurnResultRepo:       b.ExptTurnResultRepo,
			evalTargetService:        b.EvalTargetService,
			evaluatorRecordService:   b.EvaluatorRecordService,
			evaluationSetItemService: b.EvaluationSetItemService,
		}

		if exptID == b.BaselineExptID {
			// 不用重复获取基准实验的数据
			exptResultBuilder.turnResultDO = b.BaseExptTurnResultDO
		}

		exptResultBuilders = append(exptResultBuilders, exptResultBuilder)
	}

	var wg sync.WaitGroup
	resultCh := make(chan *ExptResultBuilder, len(exptResultBuilders)) // 缓冲通道，收集结果和错误

	for _, exptResultBuilder := range exptResultBuilders {
		wg.Add(1)
		go func(builder *ExptResultBuilder) {
			defer wg.Done()
			defer goroutine.Recovery(ctx)
			err := builder.build(ctx)
			builder.Err = err
			resultCh <- builder
		}(exptResultBuilder)
	}

	wg.Wait()
	close(resultCh)

	var exptResultBuildersWithResult []*ExptResultBuilder // 任务结果
	var errors []error
	for exptResultBuilder := range resultCh {
		if exptResultBuilder.Err != nil {
			errors = append(errors, fmt.Errorf("ExptID %d: %v", exptResultBuilder.ExptID, exptResultBuilder.Err))
		} else {
			exptResultBuildersWithResult = append(exptResultBuildersWithResult, exptResultBuilder)
		}
	}
	if len(errors) > 0 {
		logs.CtxError(ctx, "build expt result fail, errors:%v", errors)
		return nil, fmt.Errorf("build expt result fail, errors:%v", errors)
	}

	b.ExptResultBuilders = exptResultBuildersWithResult

	// 填充数据
	err := b.fillItemResults(ctx)
	if err != nil {
		return nil, err
	}

	return b.ItemResults, nil
}

func (b *PayloadBuilder) BuildTurnResultFilter(ctx context.Context) ([]*entity.ExptTurnResultFilterEntity, error) {
	// 分实验获取数据
	exptResultBuilder := &ExptResultBuilder{
		ExptID:                   b.BaselineExptID,
		BaselineExptID:           b.BaselineExptID,
		SpaceID:                  b.SpaceID,
		ItemIDs:                  b.ItemIDs,
		TurnIDMap:                b.TurnIDMap,
		ExperimentRepo:           b.ExperimentRepo,
		ExptTurnResultRepo:       b.ExptTurnResultRepo,
		evalTargetService:        b.EvalTargetService,
		evaluatorRecordService:   b.EvaluatorRecordService,
		evaluationSetItemService: b.EvaluationSetItemService,
		turnResultDO:             b.BaseExptTurnResultDO,
	}

	exptDO, err := exptResultBuilder.ExperimentRepo.GetByID(ctx, exptResultBuilder.ExptID, exptResultBuilder.SpaceID)
	if err != nil {
		return nil, err
	}
	exptResultBuilder.exptDO = exptDO

	if len(exptResultBuilder.turnResultDO) == 0 {
		return nil, nil
	}

	// 由于turnID可能为0，以turn_result_id为行的唯一标识聚合数据，组装payload数据时再通过turn_result_id与item_id(单轮)或turn_id(多轮)映射进行组装
	exptResultBuilder.ItemIDTurnID2TurnResultID = make(map[int64]map[int64]int64) // itemID -> turnID -> turn_result_id
	for _, turnResult := range exptResultBuilder.turnResultDO {
		if exptResultBuilder.ItemIDTurnID2TurnResultID[turnResult.ItemID] == nil {
			exptResultBuilder.ItemIDTurnID2TurnResultID[turnResult.ItemID] = make(map[int64]int64)
		}
		exptResultBuilder.ItemIDTurnID2TurnResultID[turnResult.ItemID][turnResult.TurnID] = turnResult.ID
	}

	err = exptResultBuilder.buildEvaluatorResult(ctx)
	if err != nil {
		return nil, err
	}
	if exptDO.ExptType != entity.ExptType_Online {
		err = exptResultBuilder.buildTargetOutput(ctx)
		if err != nil {
			return nil, err
		}
	}

	b.ExptResultBuilders = []*ExptResultBuilder{exptResultBuilder}

	// 填充数据
	err = b.fillExptTurnResultFilters(ctx, exptDO.StartAt, exptDO.EvalSetVersionID)
	if err != nil {
		return nil, err
	}

	return b.ExptTurnResultFilters, nil
}

func (b *PayloadBuilder) fillExptTurnResultFilters(ctx context.Context, createdDate *time.Time, evalSetVersionID int64) error {
	exptResultBuilder := b.ExptResultBuilders[0]
	b.ExptTurnResultFilters = make([]*entity.ExptTurnResultFilterEntity, 0)
	// 处理 createdDate，只保留到天级别的日期
	if createdDate != nil {
		truncatedDate := createdDate.Truncate(24 * time.Hour)
		createdDate = &truncatedDate
	}
	itemID2ItemIdx := make(map[int64]*entity.ExptItemResult)
	for _, itemResult := range b.BaseExptItemResultDO {
		itemID2ItemIdx[itemResult.ItemID] = itemResult
	}
	updatedAt := time.Now()
	for _, exptTurnResult := range b.BaseExptTurnResultDO {
		exptTurnResultFilter := &entity.ExptTurnResultFilterEntity{
			SpaceID:          b.SpaceID,
			ExptID:           b.BaselineExptID,
			ItemID:           exptTurnResult.ItemID,
			TurnID:           exptTurnResult.TurnID,
			EvalTargetData:   make(map[string]string),
			EvaluatorScore:   make(map[string]float64),
			AnnotationFloat:  make(map[string]float64),
			AnnotationBool:   make(map[string]bool),
			AnnotationString: make(map[string]string),
			CreatedDate:      ptr.From(createdDate),
			EvalSetVersionID: evalSetVersionID,
		}
		exptTurnResultFilter.ExptID = b.BaselineExptID
		exptTurnResultFilter.SpaceID = b.SpaceID
		if itemID2ItemIdx[exptTurnResult.ItemID] != nil {
			exptTurnResultFilter.ItemIdx = itemID2ItemIdx[exptTurnResult.ItemID].ItemIdx
			exptTurnResultFilter.Status = itemID2ItemIdx[exptTurnResult.ItemID].Status
		}
		evaluatorVersionID2Result, ok := exptResultBuilder.turnResultID2EvaluatorVersionID2Result[exptTurnResult.ID]
		if ok {
			for evaluatorVersionID, result := range evaluatorVersionID2Result {
				if result.GetScore() != nil {
					if keyMapping, ok := b.ExptTurnResultFilterKeyMappingEvaluatorMap[fmt.Sprintf("%d", evaluatorVersionID)]; ok {
						exptTurnResultFilter.EvaluatorScore[keyMapping.ToKey] = ptr.From(result.GetScore())
					}
				}
			}
		}
		evalTargetOutput, ok := exptResultBuilder.turnResultID2TargetOutput[exptTurnResult.ID]
		if ok {
			for outputFieldKey, outputFieldValue := range evalTargetOutput.EvalTargetRecord.EvalTargetOutputData.OutputFields {
				exptTurnResultFilter.EvalTargetData[outputFieldKey] = outputFieldValue.GetText()
			}
		}
		evaluatorScoreCorrected, ok := exptResultBuilder.turnResultID2ScoreCorrected[exptTurnResult.ID]
		if ok {
			exptTurnResultFilter.EvaluatorScoreCorrected = evaluatorScoreCorrected
		}
		exptTurnResultFilter.UpdatedAt = updatedAt
		b.ExptTurnResultFilters = append(b.ExptTurnResultFilters, exptTurnResultFilter)
	}

	return nil
}

func (b *PayloadBuilder) fillItemResults(ctx context.Context) error {
	for i := range b.ItemResults {
		itemResult := b.ItemResults[i]
		itemID := itemResult.ItemID
		for j := range itemResult.TurnResults {
			turnResult := itemResult.TurnResults[j]
			if turnResult.ExperimentResults == nil {
				turnResult.ExperimentResults = make([]*entity.ExperimentResult, 0)
			}

			turnID := turnResult.TurnID

			for _, exptResultBuilder := range b.ExptResultBuilders {
				exptID := exptResultBuilder.ExptID
				exptResult := &entity.ExperimentResult{
					ExperimentID: exptID,
					Payload:      &entity.ExperimentTurnPayload{},
				}
				exptResult.Payload.TurnID = turnID
				exptResult.Payload.EvaluatorOutput = exptResultBuilder.getTurnEvaluatorResult(ctx, itemID, turnID)
				exptResult.Payload.EvalSet = exptResultBuilder.getTurnEvalSet(ctx, itemID, turnID)
				exptResult.Payload.TargetOutput = exptResultBuilder.getTurnTargetOutput(ctx, itemID, turnID)
				exptResult.Payload.SystemInfo = exptResultBuilder.getTurnSystemInfo(ctx, itemID, turnID)

				itemResult.TurnResults[j].ExperimentResults = append(itemResult.TurnResults[j].ExperimentResults, exptResult)
			}
		}
	}

	return nil
}

func (e *ExptResultBuilder) build(ctx context.Context) error {
	exptDO, err := e.ExperimentRepo.GetByID(ctx, e.ExptID, e.SpaceID)
	if err != nil {
		return err
	}
	e.exptDO = exptDO

	// 查询非基准实验的, turn_result. 基准实验跳过查询
	if e.ExptID != e.BaselineExptID {
		// 单轮的turnID始终是0
		// 索引（space_id, expt_id, item_id, turn_id）用item_id查询后过滤
		itemTurnResults, err := e.ExptTurnResultRepo.BatchGet(ctx, e.SpaceID, e.ExptID, e.ItemIDs)
		if err != nil {
			return err
		}

		// 由于是itemID查询，对于多轮需要用turnID过滤
		turnResults := make([]*entity.ExptTurnResult, 0)
		for _, itemTurnResult := range itemTurnResults {
			if itemTurnResult.TurnID == 0 {
				turnResults = append(turnResults, itemTurnResult)
				continue
			}
			if len(e.TurnIDMap) > 0 && e.TurnIDMap[itemTurnResult.ItemID] {
				turnResults = append(turnResults, itemTurnResult)
			}
		}
		e.turnResultDO = turnResults
	}

	if len(e.turnResultDO) == 0 {
		return nil
	}

	// 由于turnID可能为0，以turn_result_id为行的唯一标识聚合数据，组装payload数据时再通过turn_result_id与item_id(单轮)或turn_id(多轮)映射进行组装
	e.ItemIDTurnID2TurnResultID = make(map[int64]map[int64]int64) // itemID -> turnID -> turn_result_id
	for _, turnResult := range e.turnResultDO {
		if e.ItemIDTurnID2TurnResultID[turnResult.ItemID] == nil {
			e.ItemIDTurnID2TurnResultID[turnResult.ItemID] = make(map[int64]int64)
		}
		e.ItemIDTurnID2TurnResultID[turnResult.ItemID][turnResult.TurnID] = turnResult.ID
	}

	err = e.buildEvaluatorResult(ctx)
	if err != nil {
		return err
	}
	err = e.buildEvalSet(ctx)
	if err != nil {
		return err
	}
	err = e.buildTargetOutput(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (e *ExptResultBuilder) buildEvaluatorResult(ctx context.Context) error {
	turnResultIDs := make([]int64, 0)
	for _, turnResult := range e.turnResultDO {
		turnResultIDs = append(turnResultIDs, turnResult.ID)
	}
	turnEvaluatorResultRefs, err := e.ExptTurnResultRepo.BatchGetTurnEvaluatorResultRef(ctx, e.SpaceID, turnResultIDs)
	if err != nil {
		return err
	}

	evaluatorResultIDs := make([]int64, 0)
	evaluatorResultID2TurnResultID := make(map[int64]int64)
	for _, turnEvaluatorResultRef := range turnEvaluatorResultRefs {
		evaluatorResultIDs = append(evaluatorResultIDs, turnEvaluatorResultRef.EvaluatorResultID)

		evaluatorResultID2TurnResultID[turnEvaluatorResultRef.EvaluatorResultID] = turnEvaluatorResultRef.ExptTurnResultID
	}

	evaluatorRecords, err := e.evaluatorRecordService.BatchGetEvaluatorRecord(ctx, evaluatorResultIDs, false)
	if err != nil {
		return err
	}

	turnResultID2VersionID2Result := make(map[int64]map[int64]*entity.EvaluatorRecord) // turn_result_id -> version_id -> result
	turnResultID2ScoreCorrected := make(map[int64]bool)
	for _, evaluatorRecord := range evaluatorRecords {
		turnResultID, ok := evaluatorResultID2TurnResultID[evaluatorRecord.ID]
		if !ok {
			continue
		}
		if _, ok := turnResultID2VersionID2Result[turnResultID]; !ok {
			turnResultID2VersionID2Result[turnResultID] = make(map[int64]*entity.EvaluatorRecord)
		}
		turnResultID2VersionID2Result[turnResultID][evaluatorRecord.EvaluatorVersionID] = evaluatorRecord
		if evaluatorRecord.GetCorrected() {
			turnResultID2ScoreCorrected[turnResultID] = true
		} else {
			if _, ok := turnResultID2ScoreCorrected[turnResultID]; !ok {
				turnResultID2ScoreCorrected[turnResultID] = false
			}
		}
	}

	e.turnResultID2EvaluatorVersionID2Result = turnResultID2VersionID2Result
	e.turnResultID2ScoreCorrected = turnResultID2ScoreCorrected
	return nil
}

func (e *ExptResultBuilder) getTurnEvaluatorResult(ctx context.Context, itemID, turnID int64) *entity.TurnEvaluatorOutput {
	turnID2TurnResultID, ok := e.ItemIDTurnID2TurnResultID[itemID]
	if !ok {
		return &entity.TurnEvaluatorOutput{}
	}
	turnResultID, ok := turnID2TurnResultID[turnID]
	if !ok {
		return &entity.TurnEvaluatorOutput{}
	}

	evaluatorVersionID2Result, ok := e.turnResultID2EvaluatorVersionID2Result[turnResultID]
	if !ok {
		return &entity.TurnEvaluatorOutput{}
	}

	for _, evaluatorResult := range evaluatorVersionID2Result {
		if evaluatorResult == nil {
			continue
		}

		if evaluatorResult.EvaluatorOutputData != nil && evaluatorResult.EvaluatorOutputData.EvaluatorRunError != nil {
			evaluatorResult.EvaluatorOutputData.EvaluatorRunError.Message = errno.ServiceInternalErrMsg
		}
	}

	return &entity.TurnEvaluatorOutput{
		EvaluatorRecords: evaluatorVersionID2Result,
	}
}

func (e *ExptResultBuilder) buildEvalSet(ctx context.Context) error {
	if e.exptDO == nil {
		return fmt.Errorf("exptPO is nil")
	}
	evalSetID := e.exptDO.EvalSetID
	evalSetVersionID := e.exptDO.EvalSetVersionID

	param := &entity.BatchGetEvaluationSetItemsParam{
		SpaceID:         e.SpaceID,
		EvaluationSetID: evalSetID,
		ItemIDs:         e.ItemIDs,
	}
	if evalSetVersionID != evalSetID {
		param.VersionID = gptr.Of(evalSetVersionID)
	}

	items, err := e.evaluationSetItemService.BatchGetEvaluationSetItems(ctx, param)
	if err != nil {
		return err
	}

	itemIDTurnID2Turn := make(map[int64]map[int64]*entity.TurnEvalSet) // item_id -> turn_id -> turn
	for _, item := range items {
		for _, turn := range item.Turns {
			if itemIDTurnID2Turn[item.ItemID] == nil {
				itemIDTurnID2Turn[item.ItemID] = make(map[int64]*entity.TurnEvalSet)
			}
			turnEvalSet := &entity.TurnEvalSet{
				Turn: turn,
			}
			itemIDTurnID2Turn[item.ItemID][turn.ID] = turnEvalSet
		}
	}

	e.itemIDTurnID2Turn = itemIDTurnID2Turn

	return nil
}

func (e *ExptResultBuilder) getTurnEvalSet(ctx context.Context, itemID, turnID int64) *entity.TurnEvalSet {
	turnID2Turn, ok := e.itemIDTurnID2Turn[itemID]
	if !ok {
		return &entity.TurnEvalSet{}
	}
	turn, ok := turnID2Turn[turnID]
	if !ok {
		return &entity.TurnEvalSet{}
	}

	return turn
}

func (e *ExptResultBuilder) buildTargetOutput(ctx context.Context) error {
	if e.exptDO.ExptType == entity.ExptType_Online {
		return nil
	}
	targetResultIDs := make([]int64, 0)
	targetResultID2turnResultID := make(map[int64]int64)
	for _, turnResult := range e.turnResultDO {
		targetResultIDs = append(targetResultIDs, turnResult.TargetResultID)
		targetResultID2turnResultID[turnResult.TargetResultID] = turnResult.ID
	}
	targetRecords, err := e.evalTargetService.BatchGetRecordByIDs(ctx, e.SpaceID, targetResultIDs)
	if err != nil {
		return err
	}

	turnResultID2TargetOutput := make(map[int64]*entity.TurnTargetOutput) // turn_result_id -> version_id -> result
	for _, targetRecord := range targetRecords {
		turnResultID, ok := targetResultID2turnResultID[targetRecord.ID]
		if !ok {
			continue
		}

		turnResultID2TargetOutput[turnResultID] = &entity.TurnTargetOutput{
			EvalTargetRecord: targetRecord,
		}
	}

	e.turnResultID2TargetOutput = turnResultID2TargetOutput

	return nil
}

func (e *ExptResultBuilder) getTurnTargetOutput(ctx context.Context, itemID, turnID int64) *entity.TurnTargetOutput {
	if e.exptDO.ExptType == entity.ExptType_Online {
		return &entity.TurnTargetOutput{}
	}
	turnID2TurnResultID, ok := e.ItemIDTurnID2TurnResultID[itemID]
	if !ok {
		return &entity.TurnTargetOutput{}
	}
	turnResultID, ok := turnID2TurnResultID[turnID]
	if !ok {
		return &entity.TurnTargetOutput{}
	}

	turnTargetOutput, ok := e.turnResultID2TargetOutput[turnResultID]
	if !ok {
		return &entity.TurnTargetOutput{}
	}

	if turnTargetOutput.EvalTargetRecord != nil && turnTargetOutput.EvalTargetRecord.EvalTargetOutputData != nil && turnTargetOutput.EvalTargetRecord.EvalTargetOutputData.EvalTargetRunError != nil {
		turnTargetOutput.EvalTargetRecord.EvalTargetOutputData.EvalTargetRunError.Message = errno.ServiceInternalErrMsg
	}

	return turnTargetOutput
}

func (e *ExptResultBuilder) getTurnSystemInfo(ctx context.Context, itemID, turnID int64) *entity.TurnSystemInfo {
	turnResultID2TurnResult := gslice.ToMap(e.turnResultDO, func(t *entity.ExptTurnResult) (int64, *entity.ExptTurnResult) {
		return t.ID, t
	})

	turnID2TurnResultID, ok := e.ItemIDTurnID2TurnResultID[itemID]
	if !ok {
		return &entity.TurnSystemInfo{}
	}
	turnResultID, ok := turnID2TurnResultID[turnID]
	if !ok {
		return &entity.TurnSystemInfo{}
	}

	turnResult, ok := turnResultID2TurnResult[turnResultID]
	if !ok {
		return &entity.TurnSystemInfo{}
	}

	systemInfo := &entity.TurnSystemInfo{
		TurnRunState: entity.TurnRunState(turnResult.Status),
		LogID:        gptr.Of(turnResult.LogID),
	}

	if len(turnResult.ErrMsg) > 0 {
		// 仅吐出评估器和评估对象之外的error
		ok, errMsg := errno.ParseTurnOtherErr(errno.DeserializeErr([]byte(turnResult.ErrMsg)))
		if ok {
			systemInfo.Error = &entity.RunError{
				Detail: gptr.Of(errMsg),
			}
		}
	}

	return systemInfo
}

func (e ExptResultServiceImpl) MGetStats(ctx context.Context, exptIDs []int64, spaceID int64, session *entity.Session) ([]*entity.ExptStats, error) {
	models, err := e.ExptStatsRepo.MGet(ctx, exptIDs, spaceID)
	if err != nil {
		return nil, err
	}

	return models, nil
}

func (e ExptResultServiceImpl) GetStats(ctx context.Context, exptID int64, spaceID int64, session *entity.Session) (*entity.ExptStats, error) {
	stats, err := e.MGetStats(ctx, []int64{exptID}, spaceID, session)
	if err != nil {
		return nil, err
	}
	return stats[0], nil
}

func (e ExptResultServiceImpl) CreateStats(ctx context.Context, exptStats *entity.ExptStats, session *entity.Session) error {
	return e.ExptStatsRepo.Create(ctx, exptStats)
}

func (e ExptResultServiceImpl) CalculateStats(ctx context.Context, exptID, spaceID int64, session *entity.Session) (*entity.ExptCalculateStats, error) {
	var (
		maxLoop = 10000
		limit   = 100
		offset  = 1
		total   = 0
		cnt     = 0
		icnt    = 0
		ioffset = 1

		pendingCnt      = 0
		failCnt         = 0
		successCnt      = 0
		processingCnt   = 0
		terminatedCnt   = 0
		incompleteTurns []*entity.ItemTurnID
	)

	for i := 0; i < maxLoop; i++ {
		itemResultList, iTotal, err := e.ExptItemResultRepo.ListItemResultsByExptID(ctx, exptID, spaceID, entity.NewPage(ioffset, limit), false)
		if err != nil {
			return nil, err
		}
		icnt += len(itemResultList)
		ioffset++
		for _, item := range itemResultList {
			switch item.Status {
			case entity.ItemRunState_Success:
				successCnt++
			case entity.ItemRunState_Fail:
				failCnt++
			case entity.ItemRunState_Terminal:
				terminatedCnt++
			case entity.ItemRunState_Queueing:
				pendingCnt++
			case entity.ItemRunState_Processing:
				processingCnt++
			default:
			}
		}
		if icnt >= int(iTotal) || len(itemResultList) == 0 {
			break
		}
		time.Sleep(time.Millisecond * 30)
	}

	for i := 0; i < maxLoop; i++ {
		logs.CtxInfo(ctx, "ExptStatsImpl.CalculateStats scan turn result, expt_id: %v, page: %v, limit: %v, cur_cnt: %v, total: %v",
			exptID, offset, limit, cnt, total)

		results, t, err := e.ExptTurnResultRepo.ListTurnResult(ctx, spaceID, exptID, nil, entity.NewPage(offset, limit), false)
		if err != nil {
			return nil, err
		}

		total = int(t)
		cnt += len(results)
		offset++

		for _, tr := range results {
			switch entity.TurnRunState(tr.Status) {
			case entity.TurnRunState_Queueing:
				incompleteTurns = append(incompleteTurns, &entity.ItemTurnID{
					TurnID: tr.TurnID,
					ItemID: tr.ItemID,
				})
			case entity.TurnRunState_Processing:
				incompleteTurns = append(incompleteTurns, &entity.ItemTurnID{
					TurnID: tr.TurnID,
					ItemID: tr.ItemID,
				})
			default:
			}
		}

		if cnt >= total || len(results) == 0 {
			break
		}

		time.Sleep(time.Millisecond * 30)
	}

	stats := &entity.ExptCalculateStats{
		PendingItemCnt:    pendingCnt,
		FailItemCnt:       failCnt,
		SuccessItemCnt:    successCnt,
		ProcessingItemCnt: processingCnt,
		TerminatedItemCnt: terminatedCnt,
		IncompleteTurnIDs: incompleteTurns,
	}

	logs.CtxInfo(ctx, "ExptStatsImpl.CalculateStats scan turn result done, expt_id: %v, total_cnt: %v, incomplete_cnt: %v, total: %v, stats: %v", exptID, cnt, len(incompleteTurns), total, json.Jsonify(stats))

	return stats, nil
}

// ManualUpsertExptTurnResultFilter 手动更新实验结果过滤条件
func (e ExptResultServiceImpl) ManualUpsertExptTurnResultFilter(ctx context.Context, spaceID, exptID int64, itemIDs []int64) error {
	ctx = contexts.WithCtxWriteDB(ctx)
	if e.lwt.CheckWriteFlagByID(ctx, platestwrite.ResourceTypeExperiment, exptID) {
		ctx = contexts.WithCtxWriteDB(ctx)
	}

	expts, err := e.ExperimentRepo.MGetByID(ctx, []int64{exptID}, spaceID)
	if err != nil {
		return err
	}
	if len(expts) == 0 {
		return fmt.Errorf("ManualUpsertExptTurnResultFilter: 实验不存在")
	}
	expt := expts[0]

	exptTurnResultFilterKeyMappings := make([]*entity.ExptTurnResultFilterKeyMapping, 0)
	for i, ref := range expt.EvaluatorVersionRef {
		exptTurnResultFilterKeyMappings = append(exptTurnResultFilterKeyMappings, &entity.ExptTurnResultFilterKeyMapping{
			SpaceID:   spaceID,
			ExptID:    exptID,
			FromField: strconv.FormatInt(ref.EvaluatorVersionID, 10),
			ToKey:     "key" + strconv.Itoa(i+1),
			FieldType: entity.FieldTypeEvaluator,
		})
	}
	if err = e.InsertExptTurnResultFilterKeyMappings(ctx, exptTurnResultFilterKeyMappings); err != nil {
		return err
	}

	if err = e.publisher.PublishExptTurnResultFilterEvent(ctx, &entity.ExptTurnResultFilterEvent{
		ExperimentID: exptID,
		SpaceID:      spaceID,
	}, gptr.Of(time.Second*3)); err != nil {
		logs.CtxError(ctx, "Failed to send ExptTurnResultFilterEvent, err: %v", err)
	}

	return nil
}

func (e ExptResultServiceImpl) UpsertExptTurnResultFilter(ctx context.Context, spaceID, exptID int64, itemIDs []int64) error {
	// 当前方法中space_id和expt_id必填，item_ids选填
	if spaceID == 0 || exptID == 0 {
		return fmt.Errorf("UpsertExptTurnResultFilter: invalid space_id or expt_id")
	}
	ctx = contexts.WithCtxWriteDB(ctx) // 更新result时需要取最新的result

	const limit = 200
	offset := 1
	maxLoop := 10000
	loopCnt := 0
	var allTurnResults []*entity.ExptTurnResult
	for {
		if loopCnt >= maxLoop {
			return fmt.Errorf("UpsertExptTurnResultFilter: 超过最大循环次数，可能存在死循环，已查%d条", len(allTurnResults))
		}
		turnResults, total, err := e.ExptTurnResultRepo.ListTurnResultByItemIDs(ctx, spaceID, exptID, itemIDs, entity.NewPage(offset, limit), false)
		if err != nil {
			return err
		}
		if len(turnResults) == 0 {
			break
		}
		allTurnResults = append(allTurnResults, turnResults...)
		if len(allTurnResults) >= int(total) {
			break
		}
		offset++
		loopCnt++
	}
	if len(allTurnResults) == 0 {
		return nil
	}
	itemIDMap := make(map[int64]bool)
	for _, turnResult := range allTurnResults {
		itemIDMap[turnResult.ItemID] = true
	}
	itemIDs = maps.ToSlice(itemIDMap, func(k int64, v bool) int64 {
		return k
	})
	itemResults, err := e.ExptItemResultRepo.BatchGet(ctx, spaceID, exptID, itemIDs)
	if err != nil {
		return err
	}
	exptTurnResultFilterKeyMappings, err := e.exptTurnResultFilterRepo.GetExptTurnResultFilterKeyMappings(ctx, spaceID, exptID)
	if err != nil {
		return err
	}
	exptTurnResultFilterKeyMappingEvaluatorMap := make(map[string]*entity.ExptTurnResultFilterKeyMapping)
	exptTurnResultFilterKeyMappingAnnotationMap := make(map[string]*entity.ExptTurnResultFilterKeyMapping)
	for _, mapping := range exptTurnResultFilterKeyMappings {
		switch mapping.FieldType {
		case entity.FieldTypeEvaluator:
			exptTurnResultFilterKeyMappingEvaluatorMap[mapping.FromField] = mapping
		case entity.FieldTypeManualAnnotation:
			exptTurnResultFilterKeyMappingAnnotationMap[mapping.FromField] = mapping
		default:
			// 不处理
		}
	}
	param := &entity.MGetExperimentResultParam{
		SpaceID: spaceID,
		ExptIDs: []int64{exptID},
	}
	payloadBuilder := NewPayloadBuilder(ctx, param, exptID, allTurnResults, itemResults, e.ExperimentRepo,
		e.ExptTurnResultRepo, e.evalTargetService, e.evaluatorRecordService, e.evaluationSetItemService, exptTurnResultFilterKeyMappingEvaluatorMap, exptTurnResultFilterKeyMappingAnnotationMap, make(map[int64]entity.ItemRunState))

	exptTurnResultFilters, err := payloadBuilder.BuildTurnResultFilter(ctx)
	if err != nil {
		return err
	}

	if err = e.exptTurnResultFilterRepo.Save(ctx, exptTurnResultFilters); err != nil {
		return err
	}

	return nil
}

// 提取过滤器映射逻辑
func (e ExptResultServiceImpl) mapItemSnapshotFilter(ctx context.Context, filter *entity.ExptTurnResultFilterAccelerator, baseExpt *entity.Experiment, baseExptEvalSetVersionID int64) error {
	if (filter.ItemSnapshotCond == nil || len(filter.ItemSnapshotCond.StringMapFilters) == 0) && (filter.KeywordSearch == nil || filter.KeywordSearch.ItemSnapshotFilter == nil || len(filter.KeywordSearch.ItemSnapshotFilter.StringMapFilters) == 0) {
		return nil
	}
	if baseExpt.ExptType == entity.ExptType_Online {
		// todo 草稿版数据集不支持模糊搜索，本期暂不实现
		return nil
	}
	//evaluationSetVersion, _, err := e.evaluationSetVersionService.GetEvaluationSetVersion(ctx, baseExpt.SpaceID, baseExptEvalSetVersionID, ptr.Of(true))
	//if err != nil {
	//	return err
	//}
	itemSnapshotMappings, syncCkDate, err := e.evaluationSetService.QueryItemSnapshotMappings(ctx, baseExpt.SpaceID, baseExpt.EvalSetID, ptr.Of(baseExpt.EvalSetVersionID))
	if err != nil {
		return err
	}
	filter.EvalSetSyncCkDate = syncCkDate
	itemSnapshotMappingsMap := make(map[string]*entity.ItemSnapshotFieldMapping)
	for _, item := range itemSnapshotMappings {
		itemSnapshotMappingsMap[item.FieldKey] = item
	}
	itemSnapshotFilter := &entity.ItemSnapshotFilter{
		BoolMapFilters:   make([]*entity.FieldFilter, 0, len(filter.ItemSnapshotCond.BoolMapFilters)),
		FloatMapFilters:  make([]*entity.FieldFilter, 0, len(filter.ItemSnapshotCond.FloatMapFilters)),
		IntMapFilters:    make([]*entity.FieldFilter, 0, len(filter.ItemSnapshotCond.IntMapFilters)),
		StringMapFilters: make([]*entity.FieldFilter, 0, len(filter.ItemSnapshotCond.StringMapFilters)),
	}
	for _, item := range filter.ItemSnapshotCond.StringMapFilters {
		if itemSnapshotMappingsMap[item.Key] == nil {
			logs.CtxWarn(ctx, "MGetExperimentResult found itemSnapshotMappingsMap not found, key: %v", item.Key)
			continue
		}
		itemSnapshotMapping := itemSnapshotMappingsMap[item.Key]
		switch itemSnapshotMapping.MappingKey {
		case "string_map":
			itemSnapshotFilter.StringMapFilters = append(itemSnapshotFilter.StringMapFilters, &entity.FieldFilter{
				Key:    itemSnapshotMapping.MappingSubKey,
				Op:     item.Op,
				Values: item.Values,
			})
		case "float_map":
			itemSnapshotFilter.FloatMapFilters = append(itemSnapshotFilter.FloatMapFilters, &entity.FieldFilter{
				Key:    itemSnapshotMapping.MappingSubKey,
				Op:     item.Op,
				Values: item.Values,
			})
		case "int_map":
			itemSnapshotFilter.IntMapFilters = append(itemSnapshotFilter.IntMapFilters, &entity.FieldFilter{
				Key:    itemSnapshotMapping.MappingSubKey,
				Op:     item.Op,
				Values: item.Values,
			})
		case "bool_map":
			itemSnapshotFilter.BoolMapFilters = append(itemSnapshotFilter.BoolMapFilters, &entity.FieldFilter{
				Key:    itemSnapshotMapping.MappingSubKey,
				Op:     item.Op,
				Values: item.Values,
			})
		}
	}
	filter.ItemSnapshotCond = itemSnapshotFilter

	// 处理keyword search
	keywordItemSnapshotFilter := &entity.ItemSnapshotFilter{
		BoolMapFilters:   make([]*entity.FieldFilter, 0, len(filter.KeywordSearch.ItemSnapshotFilter.BoolMapFilters)),
		FloatMapFilters:  make([]*entity.FieldFilter, 0, len(filter.KeywordSearch.ItemSnapshotFilter.FloatMapFilters)),
		IntMapFilters:    make([]*entity.FieldFilter, 0, len(filter.KeywordSearch.ItemSnapshotFilter.IntMapFilters)),
		StringMapFilters: make([]*entity.FieldFilter, 0, len(filter.KeywordSearch.ItemSnapshotFilter.StringMapFilters)),
	}
	for _, item := range filter.KeywordSearch.ItemSnapshotFilter.StringMapFilters {
		if itemSnapshotMappingsMap[item.Key] == nil {
			logs.CtxWarn(ctx, "MGetExperimentResult found itemSnapshotMappingsMap not found, key: %v", item.Key)
			continue
		}
		itemSnapshotMapping := itemSnapshotMappingsMap[item.Key]
		switch itemSnapshotMapping.MappingKey {
		case "string_map":
			keywordItemSnapshotFilter.StringMapFilters = append(keywordItemSnapshotFilter.StringMapFilters, &entity.FieldFilter{
				Key:    itemSnapshotMapping.MappingSubKey,
				Op:     "LIKE",
				Values: item.Values,
			})
		case "float_map":
			keywordItemSnapshotFilter.FloatMapFilters = append(keywordItemSnapshotFilter.FloatMapFilters, &entity.FieldFilter{
				Key:    itemSnapshotMapping.MappingSubKey,
				Op:     "LIKE",
				Values: item.Values,
			})
		case "int_map":
			keywordItemSnapshotFilter.IntMapFilters = append(keywordItemSnapshotFilter.IntMapFilters, &entity.FieldFilter{
				Key:    itemSnapshotMapping.MappingSubKey,
				Op:     "LIKE",
				Values: item.Values,
			})
		case "bool_map":
			keywordItemSnapshotFilter.BoolMapFilters = append(keywordItemSnapshotFilter.BoolMapFilters, &entity.FieldFilter{
				Key:    itemSnapshotMapping.MappingSubKey,
				Op:     "LIKE",
				Values: item.Values,
			})
		}
	}
	filter.KeywordSearch.ItemSnapshotFilter = keywordItemSnapshotFilter

	return nil
}

// 提取MapCond映射逻辑
func (e ExptResultServiceImpl) mapTurnResultFilterCond(ctx context.Context, filter *entity.ExptTurnResultFilterAccelerator, spaceID, baseExptID int64) error {
	if filter.MapCond == nil {
		return nil
	}
	turnResultFilterKeyMappings, err := e.exptTurnResultFilterRepo.GetExptTurnResultFilterKeyMappings(ctx, spaceID, baseExptID)
	if err != nil {
		return err
	}
	turnResultFilterKeyMappingsMap := make(map[string]*entity.ExptTurnResultFilterKeyMapping)
	for _, mapping := range turnResultFilterKeyMappings {
		turnResultFilterKeyMappingsMap[mapping.FromField] = mapping
	}
	filter.MapCond.EvaluatorScoreFilters = e.filterMapFieldByType(filter.MapCond.EvaluatorScoreFilters, turnResultFilterKeyMappingsMap, entity.FieldTypeEvaluator)
	filter.MapCond.AnnotationFloatFilters = e.filterMapFieldByType(filter.MapCond.AnnotationFloatFilters, turnResultFilterKeyMappingsMap, entity.FieldTypeManualAnnotation)
	filter.MapCond.AnnotationBoolFilters = e.filterMapFieldByType(filter.MapCond.AnnotationBoolFilters, turnResultFilterKeyMappingsMap, entity.FieldTypeManualAnnotation)
	filter.MapCond.AnnotationStringFilters = e.filterMapFieldByType(filter.MapCond.AnnotationStringFilters, turnResultFilterKeyMappingsMap, entity.FieldTypeManualAnnotation)
	return nil
}

func (e ExptResultServiceImpl) filterMapFieldByType(filters []*entity.FieldFilter, mappingMap map[string]*entity.ExptTurnResultFilterKeyMapping, fieldType entity.FieldTypeMapping) []*entity.FieldFilter {
	res := make([]*entity.FieldFilter, 0, len(filters))
	for _, cond := range filters {
		mapping, ok := mappingMap[cond.Key]
		if !ok || mapping.FieldType != fieldType {
			continue
		}
		res = append(res, &entity.FieldFilter{
			Key:    mapping.ToKey,
			Op:     cond.Op,
			Values: cond.Values,
		})
	}
	return res
}

func (e ExptResultServiceImpl) InsertExptTurnResultFilterKeyMappings(ctx context.Context, mappings []*entity.ExptTurnResultFilterKeyMapping) error {
	return e.exptTurnResultFilterRepo.InsertExptTurnResultFilterKeyMappings(ctx, mappings)
}

func (e ExptResultServiceImpl) CompareExptTurnResultFilters(ctx context.Context, spaceID, exptID int64, itemIDs []int64, retryTimes int32) error {
	ctx = contexts.WithCtxWriteDB(ctx) // 更新result时需要取最新的result
	exptDO, err := e.ExperimentRepo.MGetByID(ctx, []int64{exptID}, spaceID)
	if err != nil {
		return err
	}
	createdDate := exptDO[0].StartAt.Format("2006-01-02")

	// 获取实验轮次结果过滤器
	startTime := time.Now()
	exptTurnResultFilters, err := e.exptTurnResultFilterRepo.GetByExptIDItemIDs(ctx, strconv.FormatInt(spaceID, 10), strconv.FormatInt(exptID, 10), createdDate, gslice.Map(itemIDs, func(itemID int64) string {
		return strconv.FormatInt(itemID, 10)
	}))
	if err != nil {
		return err
	}
	e.Metric.EmitExptTurnResultFilterQueryLatency(spaceID, startTime.Unix(), err != nil)
	turnKey2ExptTurnResultFilter := e.createTurnKeyToFilterMap(exptTurnResultFilters)

	// 获取实验轮次结果过滤器键映射
	exptTurnResultFilterKeyMappings, err := e.exptTurnResultFilterRepo.GetExptTurnResultFilterKeyMappings(ctx, spaceID, exptID)
	if err != nil {
		return err
	}
	evaluatorVersionID2Key := e.createEvaluatorVersionIDToKeyMap(exptTurnResultFilterKeyMappings)

	// 获取基准分页的轮次结果
	turnResultDAOs, itemIDs, err := e.getTurnResultDAOs(ctx, spaceID, exptID, itemIDs)
	if err != nil {
		return err
	}

	if len(turnResultDAOs) == 0 {
		logs.CtxWarn(ctx, "CompareExptTurnResultFilters turnResultDAOs is empty, spaceID: %v, exptID: %v", spaceID, exptID)
		return nil
	}

	// 获取实验项结果
	itemResultDAOs, err := e.ExptItemResultRepo.BatchGet(ctx, spaceID, exptID, itemIDs)
	if err != nil {
		return err
	}

	// 创建有效负载构建器并构建项结果
	param := &entity.MGetExperimentResultParam{
		SpaceID: spaceID,
		ExptIDs: []int64{exptID},
	}
	payloadBuilder := NewPayloadBuilder(ctx, param, exptID, turnResultDAOs, itemResultDAOs, e.ExperimentRepo,
		e.ExptTurnResultRepo, e.evalTargetService, e.evaluatorRecordService, e.evaluationSetItemService, nil, nil, make(map[int64]entity.ItemRunState))
	itemResults, err := payloadBuilder.BuildItemResults(ctx)
	if err != nil {
		return err
	}

	// 创建轮次键到轮次结果、项索引和项运行状态的映射
	turnKey2TurnResult, turnKey2ItemIdx, turnKey2ItemRunState := e.createTurnKeyMaps(itemResults)

	// 比较实验轮次结果过滤器
	for turnKey, exptTurnResultFilter := range turnKey2ExptTurnResultFilter {
		diffExist, evaluatorScoreDiff, actualOutputDiff := e.compareTurnResultFilter(
			ctx, turnKey, exptTurnResultFilter, turnKey2TurnResult, turnKey2ItemIdx, turnKey2ItemRunState, evaluatorVersionID2Key)

		if !diffExist {
			logs.CtxInfo(ctx, "CompareExptTurnResultFilters finish, all equal, turnKey: %v", turnKey)
			e.Metric.EmitExptTurnResultFilterCheck(spaceID, evaluatorScoreDiff, actualOutputDiff, diffExist)
		} else {
			const maxRetryTimes = 3
			if retryTimes >= maxRetryTimes {
				logs.CtxError(ctx, "CompareExptTurnResultFilters finish, diff exist, retryTimes >= maxRetryTimes, turnKey: %v", turnKey)
				e.Metric.EmitExptTurnResultFilterCheck(spaceID, evaluatorScoreDiff, actualOutputDiff, diffExist)
			} else {
				logs.CtxWarn(ctx, "CompareExptTurnResultFilters finish, diff exist, retrying, turnKey: %v", turnKey)
				err = e.publisher.PublishExptTurnResultFilterEvent(ctx, &entity.ExptTurnResultFilterEvent{
					ExperimentID: exptID,
					SpaceID:      spaceID,
					ItemID:       []int64{itemIDs[0]},
					RetryTimes:   ptr.Of(retryTimes + 1),
					FilterType:   ptr.Of(entity.UpsertExptTurnResultFilterTypeCheck),
				}, ptr.Of(10*time.Second))
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// createTurnKeyToFilterMap 创建轮次键到过滤器的映射
func (e ExptResultServiceImpl) createTurnKeyToFilterMap(exptTurnResultFilters []*entity.ExptTurnResultFilterEntity) map[string]*entity.ExptTurnResultFilterEntity {
	turnKey2ExptTurnResultFilter := make(map[string]*entity.ExptTurnResultFilterEntity)
	for _, filter := range exptTurnResultFilters {
		turnKey2ExptTurnResultFilter[strconv.FormatInt(filter.ExptID, 10)+"_"+
			strconv.FormatInt(filter.ItemID, 10)+"_"+
			strconv.FormatInt(filter.TurnID, 10)] = filter
	}
	return turnKey2ExptTurnResultFilter
}

// createEvaluatorVersionIDToKeyMap 创建评估器版本ID到键的映射
func (e ExptResultServiceImpl) createEvaluatorVersionIDToKeyMap(exptTurnResultFilterKeyMappings []*entity.ExptTurnResultFilterKeyMapping) map[string]string {
	evaluatorVersionID2Key := make(map[string]string)
	for _, mapping := range exptTurnResultFilterKeyMappings {
		if mapping.FieldType == entity.FieldTypeEvaluator {
			evaluatorVersionID2Key[mapping.FromField] = mapping.ToKey
		}
	}
	return evaluatorVersionID2Key
}

// getTurnResultDAOs 获取基准分页的轮次结果
func (e ExptResultServiceImpl) getTurnResultDAOs(ctx context.Context, spaceID, exptID int64, itemIDs []int64) ([]*entity.ExptTurnResult, []int64, error) {
	turnResultDAOs, _, err := e.ExptTurnResultRepo.ListTurnResultByItemIDs(ctx, spaceID, exptID, itemIDs, entity.Page{}, false)
	if err != nil {
		return nil, nil, err
	}

	itemIDMap := make(map[int64]bool)
	for _, turnResult := range turnResultDAOs {
		itemIDMap[turnResult.ItemID] = true
	}
	itemIDs = maps.ToSlice(itemIDMap, func(k int64, v bool) int64 {
		return k
	})
	return turnResultDAOs, itemIDs, nil
}

// createTurnKeyMaps 创建轮次键到轮次结果、项索引和项运行状态的映射
func (e ExptResultServiceImpl) createTurnKeyMaps(itemResults []*entity.ItemResult) (map[string]*entity.TurnResult, map[string]int64, map[string]entity.ItemRunState) {
	turnKey2TurnResult := make(map[string]*entity.TurnResult)
	turnKey2ItemIdx := make(map[string]int64)
	turnKey2ItemRunState := make(map[string]entity.ItemRunState)
	for _, itemResult := range itemResults {
		for _, turnResult := range itemResult.TurnResults {
			if len(turnResult.ExperimentResults) == 0 {
				continue
			}
			turnKey := strconv.FormatInt(turnResult.ExperimentResults[0].ExperimentID, 10) + "_" +
				strconv.FormatInt(itemResult.ItemID, 10) + "_" +
				strconv.FormatInt(turnResult.TurnID, 10)
			turnKey2TurnResult[turnKey] = turnResult
			turnKey2ItemIdx[turnKey] = ptr.From(itemResult.ItemIndex)
			turnKey2ItemRunState[turnKey] = itemResult.SystemInfo.RunState
		}
	}
	return turnKey2TurnResult, turnKey2ItemIdx, turnKey2ItemRunState
}

func (e ExptResultServiceImpl) compareTurnResultFilter(ctx context.Context, turnKey string, exptTurnResultFilter *entity.ExptTurnResultFilterEntity,
	turnKey2TurnResult map[string]*entity.TurnResult, turnKey2ItemIdx map[string]int64, turnKey2ItemRunState map[string]entity.ItemRunState,
	evaluatorVersionID2Key map[string]string,
) (bool, bool, bool) {
	diffExist := false
	evaluatorScoreDiff := false
	actualOutputDiff := false

	turnResult, ok := turnKey2TurnResult[turnKey]
	if !ok {
		logs.Warn("CompareExptTurnResultFilters turnKey not found in turnResult, turnKey: %v", turnKey)
		return false, false, false
	}

	if !entity.IsTurnRunFinished(turnResult.ExperimentResults[0].Payload.SystemInfo.TurnRunState) {
		logs.CtxInfo(ctx, "CompareExptTurnResultFilters turn not finished, turnKey: %v", turnKey)
		return false, false, false
	}
	// 比较实际输出
	if actualDiff := e.compareActualOutput(exptTurnResultFilter, turnResult, turnKey); actualDiff {
		diffExist = true
		actualOutputDiff = true
	}

	// 比较项索引
	if itemIdxDiff := e.compareItemIndex(exptTurnResultFilter, turnKey2ItemIdx, turnKey); itemIdxDiff {
		diffExist = true
	}

	// 比较状态
	if statusDiff := e.compareStatus(exptTurnResultFilter, turnKey2ItemRunState, turnKey); statusDiff {
		diffExist = true
	}

	// 比较评估器分数是否修正
	if scoreCorrectedDiff := e.compareEvaluatorScoreCorrected(exptTurnResultFilter, turnResult, turnKey); scoreCorrectedDiff {
		diffExist = true
	}

	// 比较评估器分数
	if scoreDiff := e.compareEvaluatorScore(exptTurnResultFilter, turnResult, evaluatorVersionID2Key, turnKey); scoreDiff {
		diffExist = true
		evaluatorScoreDiff = true
	}

	return diffExist, evaluatorScoreDiff, actualOutputDiff
}

// compareActualOutput 比较实际输出
func (e ExptResultServiceImpl) compareActualOutput(exptTurnResultFilter *entity.ExptTurnResultFilterEntity, turnResult *entity.TurnResult, turnKey string) bool {
	ckActualOutput := exptTurnResultFilter.EvalTargetData["actual_output"]
	var rdsActualOutput string
	if turnResult.ExperimentResults[0].Payload.TargetOutput == nil || turnResult.ExperimentResults[0].Payload.TargetOutput.EvalTargetRecord == nil || turnResult.ExperimentResults[0].Payload.TargetOutput.EvalTargetRecord.EvalTargetOutputData == nil ||
		turnResult.ExperimentResults[0].Payload.TargetOutput.EvalTargetRecord.EvalTargetOutputData.OutputFields["actual_output"] == nil {
		logs.Warn("CompareExptTurnResultFilters compareActualOutput actual_output is nil, turnKey: %v", turnKey)
		return true
	}
	rdsActualOutput = turnResult.ExperimentResults[0].Payload.TargetOutput.EvalTargetRecord.EvalTargetOutputData.OutputFields["actual_output"].GetText()
	if ckActualOutput != rdsActualOutput {
		logs.Warn("CompareExptTurnResultFilters diff actual_output not equal, turnKey: %v, ckActualOutput: %v, rdsActualOutput: %v", turnKey, ckActualOutput, rdsActualOutput)
		return true
	}
	return false
}

// compareItemIndex 比较项索引
func (e ExptResultServiceImpl) compareItemIndex(exptTurnResultFilter *entity.ExptTurnResultFilterEntity, turnKey2ItemIdx map[string]int64, turnKey string) bool {
	ckItemIdx := exptTurnResultFilter.ItemIdx
	rdsItemIdx := turnKey2ItemIdx[turnKey]

	if ckItemIdx != int32(rdsItemIdx) {
		logs.Warn("CompareExptTurnResultFilters diff item_idx not equal, turnKey: %v, ckItemIdx: %v, rdsItemIdx: %v", turnKey, ckItemIdx, rdsItemIdx)
		return true
	}
	return false
}

// compareStatus 比较状态
func (e ExptResultServiceImpl) compareStatus(exptTurnResultFilter *entity.ExptTurnResultFilterEntity, turnKey2ItemRunState map[string]entity.ItemRunState, turnKey string) bool {
	ckStatus := exptTurnResultFilter.Status
	rdsStatus := turnKey2ItemRunState[turnKey]

	if ckStatus != rdsStatus {
		logs.Warn("CompareExptTurnResultFilters diff status not equal, turnKey: %v, ckStatus: %v, rdsStatus: %v", turnKey, ckStatus, rdsStatus)
		return true
	}
	return false
}

// compareEvaluatorScoreCorrected 比较评估器分数是否修正
func (e ExptResultServiceImpl) compareEvaluatorScoreCorrected(exptTurnResultFilter *entity.ExptTurnResultFilterEntity, turnResult *entity.TurnResult, turnKey string) bool {
	ckEvaluatorScoreCorrected := exptTurnResultFilter.EvaluatorScoreCorrected
	rdsEvaluatorScoreCorrected := false

	for _, record := range turnResult.ExperimentResults[0].Payload.EvaluatorOutput.EvaluatorRecords {
		if record.EvaluatorOutputData.EvaluatorResult != nil && record.EvaluatorOutputData.EvaluatorResult.Correction != nil {
			rdsEvaluatorScoreCorrected = true
			break
		}
	}

	if ckEvaluatorScoreCorrected != rdsEvaluatorScoreCorrected {
		logs.Warn("CompareExptTurnResultFilters diff evaluator_score_corrected not equal, turnKey: %v, ckEvaluatorScoreCorrected: %v, rdsEvaluatorScoreCorrected: %v", turnKey, ckEvaluatorScoreCorrected, rdsEvaluatorScoreCorrected)
		return true
	}
	return false
}

// compareEvaluatorScore 比较评估器分数
func (e ExptResultServiceImpl) compareEvaluatorScore(exptTurnResultFilter *entity.ExptTurnResultFilterEntity, turnResult *entity.TurnResult, evaluatorVersionID2Key map[string]string, turnKey string) bool {
	if turnResult.ExperimentResults[0].Payload.EvaluatorOutput == nil || len(turnResult.ExperimentResults[0].Payload.EvaluatorOutput.EvaluatorRecords) == 0 {
		logs.Warn("CompareExptTurnResultFilters compareEvaluatorScore EvaluatorOutput is nil, turnKey: %v", turnKey)
		return true
	}
	for key, ckEvaluatorScore := range exptTurnResultFilter.EvaluatorScore {
		var rdsEvaluatorScore float64
		for _, record := range turnResult.ExperimentResults[0].Payload.EvaluatorOutput.EvaluatorRecords {
			if evaluatorVersionID2Key[strconv.FormatInt(record.EvaluatorVersionID, 10)] == key {
				if record.EvaluatorOutputData == nil || record.EvaluatorOutputData.EvaluatorResult == nil {
					continue
				}
				if record.EvaluatorOutputData.EvaluatorResult.Correction != nil {
					rdsEvaluatorScore = ptr.From(record.EvaluatorOutputData.EvaluatorResult.Correction.Score)
				} else {
					rdsEvaluatorScore = ptr.From(record.EvaluatorOutputData.EvaluatorResult.Score)
				}
				break
			}
		}
		if ckEvaluatorScore != rdsEvaluatorScore {
			logs.Warn("CompareExptTurnResultFilters diff evaluator_score not equal, turnKey: %v, ckEvaluatorScore: %v, rdsEvaluatorScore: %v", turnKey, ckEvaluatorScore, rdsEvaluatorScore)
			return true
		}
	}
	return false
}
