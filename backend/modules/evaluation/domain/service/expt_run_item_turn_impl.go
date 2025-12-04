// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/bytedance/gg/gmap"
	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/goroutine"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// ExptItemTurnEvaluation 评测执行流程
type ExptItemTurnEvaluation interface {
	Eval(ctx context.Context, etec *entity.ExptTurnEvalCtx) *entity.ExptTurnRunResult
}

func NewExptTurnEvaluation(
	metric metrics.ExptMetric,
	evalTargetService IEvalTargetService,
	evaluatorService EvaluatorService,
	benefitService benefit.IBenefitService,
	evalAsyncRepo repo.IEvalAsyncRepo,
) ExptItemTurnEvaluation {
	return &DefaultExptTurnEvaluationImpl{
		metric:            metric,
		evalTargetService: evalTargetService,
		evaluatorService:  evaluatorService,
		benefitService:    benefitService,
		evalAsyncRepo:     evalAsyncRepo,
	}
}

type DefaultExptTurnEvaluationImpl struct {
	metric            metrics.ExptMetric
	evalTargetService IEvalTargetService
	evaluatorService  EvaluatorService
	benefitService    benefit.IBenefitService
	evalAsyncRepo     repo.IEvalAsyncRepo
}

func (e *DefaultExptTurnEvaluationImpl) Eval(ctx context.Context, etec *entity.ExptTurnEvalCtx) (trr *entity.ExptTurnRunResult) {
	defer e.metric.EmitTurnExecEval(etec.Event.SpaceID, int64(etec.Event.ExptRunMode))

	startTime := time.Now()
	trr = &entity.ExptTurnRunResult{}

	defer func() {
		code, stable, _ := errno.ParseStatusError(trr.EvalErr)
		e.metric.EmitTurnExecResult(etec.Event.SpaceID, int64(etec.Event.ExptRunMode), trr.EvalErr == nil, stable, int64(code), startTime)
	}()
	defer goroutine.Recover(ctx, &trr.EvalErr)

	targetResult, err := e.CallTarget(ctx, etec)
	if err != nil {
		logs.CtxError(ctx, "[ExptTurnEval] call target fail, err: %v", err)
		return trr.SetEvalErr(err)
	}

	logs.CtxInfo(ctx, "[ExptTurnEval] call target success, target_result: %v", json.Jsonify(targetResult))

	if trr.SetTargetResult(targetResult).AbortWithTargetResult(etec.Expt) {
		return trr
	}

	evaluatorResults, err := e.CallEvaluators(ctx, etec, targetResult)
	if err != nil {
		logs.CtxError(ctx, "[ExptTurnEval] call evaluators fail, err: %v", err)
		return trr.SetEvaluatorResults(evaluatorResults).SetEvalErr(err)
	}

	logs.CtxInfo(ctx, "[ExptTurnEval] call evaluators success, evaluator_results: %v", json.Jsonify(evaluatorResults))

	return trr.SetEvaluatorResults(evaluatorResults)
}

func (e *DefaultExptTurnEvaluationImpl) CallTarget(ctx context.Context, etec *entity.ExptTurnEvalCtx) (*entity.EvalTargetRecord, error) {
	if e.skipTargetNode(etec.Expt) {
		return &entity.EvalTargetRecord{EvalTargetOutputData: &entity.EvalTargetOutputData{OutputFields: make(map[string]*entity.Content)}}, nil
	}

	if existRecord := e.existedTargetRecord(etec); existRecord != nil {
		logs.CtxInfo(ctx, "CallTarget return with existed target record, record_id: %v", existRecord.ID)
		return existRecord, nil
	}

	if err := e.CheckBenefit(ctx, etec.Event.ExptID, etec.Event.SpaceID, etec.Expt.CreditCost == entity.CreditCostFree, etec.Event.Session); err != nil {
		return nil, err
	}

	return e.callTarget(ctx, etec, etec.History, etec.Event.SpaceID)
}

// skipTargetNode Whether target is called is determined by the target info bound in expt;
// ConnectorConf.TargetConf serves as the config info for executing the target, and CheckConnector completes the validity check when creating experiment.
func (e *DefaultExptTurnEvaluationImpl) skipTargetNode(expt *entity.Experiment) bool {
	if expt.TargetVersionID == 0 {
		return true
	}
	if expt.ExptType == entity.ExptType_Online {
		return true
	}
	return false
}

func (e *DefaultExptTurnEvaluationImpl) existedTargetRecord(etec *entity.ExptTurnEvalCtx) *entity.EvalTargetRecord {
	if etec == nil || etec.ExptTurnRunResult.TargetResult == nil {
		return nil
	}
	if gptr.Indirect(etec.ExptTurnRunResult.TargetResult.Status) == entity.EvalTargetRunStatusSuccess {
		return etec.ExptTurnRunResult.TargetResult
	}
	return nil
}

func (e *DefaultExptTurnEvaluationImpl) skipEvaluatorNode(expt *entity.Experiment) bool {
	return expt.EvalConf.ConnectorConf.EvaluatorsConf == nil
}

func (e *DefaultExptTurnEvaluationImpl) CheckBenefit(ctx context.Context, exptID, spaceID int64, freeCost bool, session *entity.Session) error {
	req := &benefit.CheckAndDeductEvalBenefitParams{
		ConnectorUID: session.UserID,
		SpaceID:      spaceID,
		ExperimentID: exptID,
		Ext:          map[string]string{benefit.ExtKeyExperimentFreeCost: strconv.FormatBool(freeCost)},
	}

	result, err := e.benefitService.CheckAndDeductEvalBenefit(ctx, req)
	logs.CtxInfo(ctx, "[CheckAndDeductEvalBenefit][req = %s] [res = %s] [err = %v]", json.Jsonify(req), json.Jsonify(result))
	if err != nil {
		return errorx.Wrapf(err, "CheckAndDeductEvalBenefit fail, expt_id: %v, user_id: %v", exptID, session.UserID)
	}

	if result != nil && result.DenyReason != nil && result.DenyReason.ToErr() != nil {
		return result.DenyReason.ToErr()
	}

	return nil
}

func (e *DefaultExptTurnEvaluationImpl) callTarget(ctx context.Context, etec *entity.ExptTurnEvalCtx, history []*entity.Message, spaceID int64) (record *entity.EvalTargetRecord, err error) {
	defer func() { e.metric.EmitTurnExecTargetResult(etec.Event.SpaceID, err != nil) }()

	turn := etec.Turn
	targetConf := etec.Expt.EvalConf.ConnectorConf.TargetConf

	if err := targetConf.Valid(ctx, etec.Expt.Target.EvalTargetType); err != nil {
		return nil, err
	}

	turnFields := gslice.ToMap(turn.FieldDataList, func(t *entity.FieldData) (string, *entity.Content) {
		return t.Name, t.Content
	})

	buildInputFields := func(fieldConfs []*entity.FieldConf) (map[string]*entity.Content, error) {
		fields := make(map[string]*entity.Content, len(fieldConfs))
		switch etec.Expt.Target.EvalTargetType {
		case entity.EvalTargetTypeCustomRPCServer:
			return turnFields, nil
		default:
			for _, fc := range fieldConfs {
				firstField, err := json.GetFirstJSONPathField(fc.FromField)
				if err != nil {
					return nil, err
				}
				if firstField == fc.FromField { // 没有下钻字段
					fields[fc.FieldName] = turnFields[fc.FromField]
				} else {
					content, err := e.getContentByJsonPath(turnFields[firstField], fc.FromField)
					if err != nil {
						return nil, err
					}
					fields[fc.FieldName] = content
				}
			}

		}
		return fields, nil
	}

	var inputFields map[string]*entity.Content
	if targetConf.IngressConf != nil && targetConf.IngressConf.EvalSetAdapter != nil {
		inputFields, err = buildInputFields(targetConf.IngressConf.EvalSetAdapter.FieldConfs)
		if err != nil {
			return nil, err
		}
	}

	ext := gmap.Clone(etec.Ext)
	if targetConf.IngressConf != nil && targetConf.IngressConf.CustomConf != nil {
		for _, fc := range targetConf.IngressConf.CustomConf.FieldConfs {
			if fc.FieldName == consts.FieldAdapterBuiltinFieldNameRuntimeParam {
				ext[consts.TargetExecuteExtRuntimeParamKey] = fc.Value
			}
		}
	}

	var targetRecord *entity.EvalTargetRecord
	etc := &entity.ExecuteTargetCtx{
		ExperimentRunID: gptr.Of(etec.Event.ExptRunID),
		ItemID:          etec.EvalSetItem.ItemID,
		TurnID:          etec.Turn.ID,
	}
	etid := &entity.EvalTargetInputData{
		HistoryMessages: history,
		InputFields:     inputFields,
		Ext:             ext,
	}

	if !etec.Expt.AsyncCallTarget() {
		return e.evalTargetService.ExecuteTarget(ctx, spaceID, etec.Expt.Target.ID, etec.Expt.Target.EvalTargetVersion.ID, etc, etid)
	}

	ts := time.Now()
	targetRecord, callee, err := e.evalTargetService.AsyncExecuteTarget(ctx, spaceID, etec.Expt.Target.ID, etec.Expt.Target.EvalTargetVersion.ID, etc, etid)
	if err != nil {
		return nil, err
	}

	if err := e.evalAsyncRepo.SetEvalAsyncCtx(ctx, strconv.FormatInt(targetRecord.ID, 10), &entity.EvalAsyncCtx{
		Event:       etec.Event,
		TurnID:      targetRecord.ID,
		AsyncUnixMS: ts.UnixMilli(),
		Session:     etec.Event.Session,
		Callee:      callee,
	}); err != nil {
		return nil, err
	}

	return targetRecord, nil
}

func (e *DefaultExptTurnEvaluationImpl) CallEvaluators(ctx context.Context, etec *entity.ExptTurnEvalCtx, targetResult *entity.EvalTargetRecord) (map[int64]*entity.EvaluatorRecord, error) {
	if e.skipEvaluatorNode(etec.Expt) {
		return make(map[int64]*entity.EvaluatorRecord), nil
	}

	expt := etec.Expt
	evaluatorResults := make(map[int64]*entity.EvaluatorRecord)
	pendingEvaluatorVersionIDs := make([]int64, 0, len(expt.Evaluators))

	for _, evaluatorVersion := range expt.Evaluators {
		existResult := etec.ExptTurnRunResult.GetEvaluatorRecord(evaluatorVersion.GetEvaluatorVersionID())

		if existResult != nil && existResult.Status == entity.EvaluatorRunStatusSuccess {
			evaluatorResults[existResult.ID] = existResult
			continue
		}

		pendingEvaluatorVersionIDs = append(pendingEvaluatorVersionIDs, evaluatorVersion.GetEvaluatorVersionID())
	}

	logs.CtxInfo(ctx, "CallEvaluators with pending evaluator version ids: %v", pendingEvaluatorVersionIDs)

	if len(pendingEvaluatorVersionIDs) == 0 {
		return evaluatorResults, nil
	}

	if err := e.CheckBenefit(ctx, etec.Event.ExptID, etec.Event.SpaceID, etec.Expt.CreditCost == entity.CreditCostFree, etec.Event.Session); err != nil {
		return nil, err
	}

	runEvalRes, evalErr := e.callEvaluators(ctx, pendingEvaluatorVersionIDs, etec, targetResult, etec.History)
	for evID, result := range runEvalRes {
		evaluatorResults[evID] = result
	}

	return evaluatorResults, evalErr
}

func (e *DefaultExptTurnEvaluationImpl) callEvaluators(ctx context.Context, execEvaluatorVersionIDs []int64, etec *entity.ExptTurnEvalCtx,
	targetResult *entity.EvalTargetRecord, history []*entity.Message,
) (map[int64]*entity.EvaluatorRecord, error) {
	var (
		recordMap      sync.Map
		item           = etec.EvalSetItem
		expt           = etec.Expt
		turn           = etec.Turn
		spaceID        = expt.SpaceID
		evaluatorsConf = expt.EvalConf.ConnectorConf.EvaluatorsConf
	)

	if err := evaluatorsConf.Valid(ctx); err != nil {
		return nil, err
	}

	execEvalVerIDMap := gslice.ToMap(execEvaluatorVersionIDs, func(t int64) (int64, bool) { return t, true })

	var turnFields map[string]*entity.Content
	if turn != nil && turn.FieldDataList != nil {
		turnFields = gslice.ToMap(turn.FieldDataList, func(t *entity.FieldData) (string, *entity.Content) {
			return t.Name, t.Content
		})
	} else {
		turnFields = make(map[string]*entity.Content)
	}
	targetFields := targetResult.EvalTargetOutputData.OutputFields

	pool, err := goroutine.NewPool(evaluatorsConf.GetEvaluatorConcurNum())
	if err != nil {
		return nil, err
	}

	for idx := range expt.Evaluators {
		ev := expt.Evaluators[idx]
		versionID := ev.GetEvaluatorVersionID()

		if !execEvalVerIDMap[versionID] {
			continue
		}

		ec := evaluatorsConf.GetEvaluatorConf(versionID)
		if ec == nil {
			return nil, fmt.Errorf("expt's evaluator conf not found, evaluator_version_id: %d", versionID)
		}

		// 根据评估器类型创建对应的输入数据
		inputData, err := e.buildEvaluatorInputData(ev.EvaluatorType, ev.GetInputSchemas(), ec, turnFields, targetFields)
		if err != nil {
			return nil, err
		}

		pool.Add(func() error {
			var err error
			defer e.metric.EmitTurnExecEvaluatorResult(spaceID, err != nil)

			evaluatorRecord, err := e.evaluatorService.RunEvaluator(ctx, &entity.RunEvaluatorRequest{
				SpaceID:            spaceID,
				Name:               "",
				EvaluatorVersionID: ev.GetEvaluatorVersionID(),
				InputData:          inputData,
				ExperimentID:       etec.Event.ExptID,
				ExperimentRunID:    etec.Event.ExptRunID,
				ItemID:             item.ItemID,
				TurnID:             turn.ID,
				Ext:                e.buildRunEvaluatorExt(etec.Ext, ec.RunConf),
				EvaluatorRunConf:   ec.RunConf,
			})
			if err != nil {
				return err
			}

			recordMap.Store(ev.GetEvaluatorVersionID(), evaluatorRecord)
			return nil
		})
	}

	err = pool.Exec(ctx)
	records := make(map[int64]*entity.EvaluatorRecord, len(expt.Evaluators))
	recordMap.Range(func(key, value interface{}) bool {
		record, _ := value.(*entity.EvaluatorRecord)
		records[key.(int64)] = record
		return true
	})

	return records, err
}

// buildEvaluatorInputData 根据评估器类型构建输入数据，提取公共字段映射逻辑
func (e *DefaultExptTurnEvaluationImpl) buildEvaluatorInputData(
	evaluatorType entity.EvaluatorType,
	inputSchemas []*entity.ArgsSchema,
	ec *entity.EvaluatorConf,
	turnFields map[string]*entity.Content,
	targetFields map[string]*entity.Content,
) (*entity.EvaluatorInputData, error) {
	if evaluatorType == entity.EvaluatorTypeCode || (evaluatorType == entity.EvaluatorTypeCustomRPC && len(inputSchemas) == 0) {
		// Code评估器、无input_schemas的自定义服务评估器：分离字段数据源
		evaluateDatasetFields, err := e.buildFieldsFromSource(ec.IngressConf.EvalSetAdapter.FieldConfs, turnFields)
		if err != nil {
			return nil, err
		}

		evaluateTargetOutputFields, err := e.buildFieldsFromSource(ec.IngressConf.TargetAdapter.FieldConfs, targetFields)
		if err != nil {
			return nil, err
		}

		return &entity.EvaluatorInputData{
			HistoryMessages:            nil,
			InputFields:                make(map[string]*entity.Content),
			EvaluateDatasetFields:      evaluateDatasetFields,
			EvaluateTargetOutputFields: evaluateTargetOutputFields,
		}, nil
	} else {
		// Prompt评估器等：保持现有逻辑，合并所有字段到InputFields
		inputFields := make(map[string]*entity.Content)

		// 处理来自评测对象的字段
		targetFieldsData, err := e.buildFieldsFromSource(ec.IngressConf.TargetAdapter.FieldConfs, targetFields)
		if err != nil {
			return nil, err
		}
		for key, content := range targetFieldsData {
			inputFields[key] = content
		}

		// 处理来自评测集的字段
		evalSetFieldsData, err := e.buildFieldsFromSource(ec.IngressConf.EvalSetAdapter.FieldConfs, turnFields)
		if err != nil {
			return nil, err
		}
		for key, content := range evalSetFieldsData {
			inputFields[key] = content
		}

		return &entity.EvaluatorInputData{
			HistoryMessages: nil,
			InputFields:     inputFields,
		}, nil
	}
}

// buildFieldsFromSource 从指定数据源构建字段映射，提取重复的字段处理逻辑
func (e *DefaultExptTurnEvaluationImpl) buildFieldsFromSource(
	fieldConfs []*entity.FieldConf,
	sourceFields map[string]*entity.Content,
) (map[string]*entity.Content, error) {
	result := make(map[string]*entity.Content)

	for _, fc := range fieldConfs {
		content, err := e.getFieldContent(fc, sourceFields)
		if err != nil {
			return nil, err
		}
		result[fc.FieldName] = content
	}

	return result, nil
}

// getFieldContent 获取字段内容，处理JSON Path逻辑
func (e *DefaultExptTurnEvaluationImpl) getFieldContent(
	fc *entity.FieldConf,
	sourceFields map[string]*entity.Content,
) (*entity.Content, error) {
	firstField, err := json.GetFirstJSONPathField(fc.FromField)
	if err != nil {
		return nil, err
	}

	if firstField == fc.FromField {
		// 没有下钻字段，直接返回
		return sourceFields[fc.FromField], nil
	} else {
		// 有下钻字段，需要通过JSON Path处理
		return e.getContentByJsonPath(sourceFields[firstField], fc.FromField)
	}
}

// 注意此函数有特化逻辑不可直接服用, 删除了jsonpath的第一级
func (e *DefaultExptTurnEvaluationImpl) getContentByJsonPath(content *entity.Content, jsonPath string) (*entity.Content, error) {
	logs.CtxInfo(context.Background(), "getContentByJsonPath, content: %v, jsonPath: %v", json.Jsonify(content), jsonPath)
	if content == nil {
		return nil, nil
	}
	if content.ContentType == nil || ptr.From(content.ContentType) != entity.ContentTypeText {
		return nil, nil
	}
	jsonPath, err := json.RemoveFirstJSONPathLevel(jsonPath)
	if err != nil {
		return nil, err
	}
	logs.CtxInfo(context.Background(), "RemoveFirstJSONPathLevel, jsonPath: %v", jsonPath)
	text, err := json.GetStringByJSONPath(ptr.From(content.Text), jsonPath)
	if err != nil {
		return nil, err
	}
	logs.CtxInfo(context.Background(), "getContentByJsonPath, text: %v", text)
	return &entity.Content{
		ContentType: ptr.Of(entity.ContentTypeText),
		Text:        ptr.Of(text),
	}, nil
}

func (e *DefaultExptTurnEvaluationImpl) buildRunEvaluatorExt(ext map[string]string, runConf *entity.EvaluatorRunConfig) map[string]string {
	builtExt := gmap.Clone(ext)
	if builtExt == nil {
		builtExt = make(map[string]string)
	}
	if runConf != nil && runConf.EvaluatorRuntimeParam != nil && runConf.EvaluatorRuntimeParam.JSONValue != nil && len(*runConf.EvaluatorRuntimeParam.JSONValue) > 0 {
		builtExt[consts.FieldAdapterBuiltinFieldNameRuntimeParam] = *runConf.EvaluatorRuntimeParam.JSONValue
	}

	return builtExt
}
