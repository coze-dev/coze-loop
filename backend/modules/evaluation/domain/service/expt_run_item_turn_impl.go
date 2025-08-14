// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
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

func NewExptTurnEvaluation(metric metrics.ExptMetric,
	evalTargetService IEvalTargetService,
	evaluatorService EvaluatorService,
	benefitService benefit.IBenefitService,
) ExptItemTurnEvaluation {
	return &DefaultExptTurnEvaluationImpl{
		metric:            metric,
		evalTargetService: evalTargetService,
		evaluatorService:  evaluatorService,
		benefitService:    benefitService,
	}
}

type DefaultExptTurnEvaluationImpl struct {
	metric            metrics.ExptMetric
	evalTargetService IEvalTargetService
	evaluatorService  EvaluatorService
	benefitService    benefit.IBenefitService
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

	var targetResult *entity.EvalTargetRecord
	var err error
	targetResult, err = e.CallTarget(ctx, etec)
	if err != nil {
		logs.CtxError(ctx, "[ExptTurnEval] call target fail, err: %v", err)
		return trr.SetEvalErr(err)
	}
	logs.CtxInfo(ctx, "[ExptTurnEval] call target success, target_result: %v", json.Jsonify(targetResult))

	trr.SetTargetResult(targetResult)
	if targetResult != nil && targetResult.EvalTargetOutputData != nil && targetResult.EvalTargetOutputData.EvalTargetRunError != nil {
		return trr
	}

	if targetResult == nil {
		err = errorx.NewByCode(errno.CommonInternalErrorCode, errorx.WithExtraMsg("target result is nil"))
		return trr.SetEvalErr(err)

	}

	evaluatorResults, err := e.CallEvaluators(ctx, etec, targetResult)
	if err != nil {
		logs.CtxError(ctx, "[ExptTurnEval] call evaluators fail, err: %v", err)
		return trr.SetEvaluatorResults(evaluatorResults).SetEvalErr(err)
	}
	logs.CtxInfo(ctx, "[ExptTurnEval] call evaluators success, evaluator_results: %v", json.Jsonify(evaluatorResults))
	trr.SetEvaluatorResults(evaluatorResults)

	return trr
}

func (e *DefaultExptTurnEvaluationImpl) CallTarget(ctx context.Context, etec *entity.ExptTurnEvalCtx) (*entity.EvalTargetRecord, error) {
	if etec.Expt.ExptType == entity.ExptType_Online {
		logs.CtxInfo(ctx, "[ExptTurnEval] expt type is online, skip call target, expt_id: %v, expt_run_id: %v, item_id: %v, turn_id: %v")
		return &entity.EvalTargetRecord{
			EvalTargetOutputData: &entity.EvalTargetOutputData{
				OutputFields: make(map[string]*entity.Content),
			},
		}, nil
	}
	existResult := etec.ExptTurnRunResult.TargetResult

	if existResult != nil && existResult.Status != nil && *existResult.Status == entity.EvalTargetRunStatusSuccess {
		return existResult, nil
	}

	if err := e.CheckBenefit(ctx, etec.Event.ExptID, etec.Event.SpaceID, etec.Expt.CreditCost == entity.CreditCostFree, etec.Event.Session); err != nil {
		return nil, err
	}

	return e.callTarget(ctx, etec, etec.History, etec.Event.SpaceID)
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
	logs.CtxInfo(ctx, "[ExptTurnEval] call target, etec: %v", etec)
	defer e.metric.EmitTurnExecTargetResult(etec.Event.SpaceID, err != nil)

	turn := etec.Turn
	targetConf := etec.Expt.EvalConf.ConnectorConf.TargetConf

	if err := targetConf.Valid(ctx, etec.Expt.Target.EvalTargetType); err != nil {
		return nil, err
	}

	turnFields := gslice.ToMap(turn.FieldDataList, func(t *entity.FieldData) (string, *entity.Content) {
		return t.Name, t.Content
	})

	fieldConfs := targetConf.IngressConf.EvalSetAdapter.FieldConfs
	fields := make(map[string]*entity.Content, len(fieldConfs))
	for _, fc := range fieldConfs {
		firstField, err := json.GetFirstJSONPathField(fc.FromField)
		if err != nil {
			return nil, err
		}
		if firstField == fc.FromField { // 没有下钻字段
			fields[fc.FieldName] = turnFields[fc.FromField]
			continue
		}
		content, err := e.getContentByJsonPath(turnFields[firstField], fc.FromField)
		if err != nil {
			return nil, err
		}
		fields[fc.FieldName] = content
	}

	targetRecord, err := e.evalTargetService.ExecuteTarget(ctx, spaceID, etec.Expt.Target.ID, etec.Expt.Target.EvalTargetVersion.ID, &entity.ExecuteTargetCtx{
		ExperimentRunID: gptr.Of(etec.Event.ExptRunID),
		ItemID:          etec.EvalSetItem.ItemID,
		TurnID:          etec.Turn.ID,
	}, &entity.EvalTargetInputData{
		HistoryMessages: history,
		InputFields:     fields,
		Ext:             etec.Ext,
	})
	if err != nil {
		return nil, err
	}

	return targetRecord, nil
}

func (e *DefaultExptTurnEvaluationImpl) CallEvaluators(ctx context.Context, etec *entity.ExptTurnEvalCtx, targetResult *entity.EvalTargetRecord) (map[int64]*entity.EvaluatorRecord, error) {
	expt := etec.Expt
	evaluatorResults := make(map[int64]*entity.EvaluatorRecord)
	pendingEvaluatorVersionIDs := make([]int64, 0, len(expt.Evaluators))

	for _, evaluatorVersion := range expt.Evaluators {
		existResult := etec.ExptTurnRunResult.GetEvaluatorRecord(evaluatorVersion.GetEvaluatorVersion().GetID())

		if existResult != nil && existResult.Status == entity.EvaluatorRunStatusSuccess {
			evaluatorResults[existResult.ID] = existResult
			continue
		}

		pendingEvaluatorVersionIDs = append(pendingEvaluatorVersionIDs, evaluatorVersion.GetEvaluatorVersion().GetID())
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
	logs.CtxInfo(ctx, "[ExptTurnEval] call evaluators, etec: %v", etec)
	logs.CtxInfo(ctx, "[ExptTurnEval] call evaluators, target_result: %v", json.Jsonify(targetResult))
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

	turnFields := gslice.ToMap(turn.FieldDataList, func(t *entity.FieldData) (string, *entity.Content) {
		return t.Name, t.Content
	})
	targetFields := targetResult.EvalTargetOutputData.OutputFields

	pool, err := goroutine.NewPool(evaluatorsConf.GetEvaluatorConcurNum())
	if err != nil {
		return nil, err
	}

	for idx := range expt.Evaluators {
		ev := expt.Evaluators[idx]
		versionID := ev.GetEvaluatorVersion().GetID()

		if !execEvalVerIDMap[versionID] {
			continue
		}

		ec := evaluatorsConf.GetEvaluatorConf(versionID)
		if ec == nil {
			return nil, fmt.Errorf("expt's evaluator conf not found, evaluator_version_id: %d", versionID)
		}

		curFields := make(map[string]*entity.Content)

		for _, fc := range ec.IngressConf.TargetAdapter.FieldConfs {
			firstField, err := json.GetFirstJSONPathField(fc.FromField)
			if err != nil {
				return nil, err
			}
			if firstField == fc.FromField { // 没有下钻字段
				curFields[fc.FieldName] = targetFields[fc.FromField]
				continue
			}
			content, err := e.getContentByJsonPath(targetFields[firstField], fc.FromField)
			if err != nil {
				return nil, err
			}
			curFields[fc.FieldName] = content
		}
		for _, fc := range ec.IngressConf.EvalSetAdapter.FieldConfs {
			firstField, err := json.GetFirstJSONPathField(fc.FromField)
			if err != nil {
				return nil, err
			}
			if firstField == fc.FromField { // 没有下钻字段
				curFields[fc.FieldName] = turnFields[fc.FromField]
				continue
			}
			content, err := e.getContentByJsonPath(turnFields[firstField], fc.FromField)
			if err != nil {
				return nil, err
			}
			curFields[fc.FieldName] = content
		}

		pool.Add(func() error {
			var err error
			defer e.metric.EmitTurnExecEvaluatorResult(spaceID, err != nil)

			// 转换 InputFields
			inputFields := make(map[string]*entity.Content)
			for key, contentDO := range curFields {
				inputFields[key] = contentDO
			}
			evaluatorRecord, err := e.evaluatorService.RunEvaluator(ctx, &entity.RunEvaluatorRequest{
				SpaceID:            spaceID,
				Name:               "",
				EvaluatorVersionID: ev.GetEvaluatorVersion().GetID(),
				InputData: &entity.EvaluatorInputData{
					HistoryMessages: nil,
					InputFields:     inputFields,
				},
				ExperimentID:    etec.Event.ExptID,
				ExperimentRunID: etec.Event.ExptRunID,
				ItemID:          item.ItemID,
				TurnID:          turn.ID,
				Ext:             etec.Ext,
			})
			if err != nil {
				return err
			}

			recordMap.Store(ev.GetEvaluatorVersion().GetID(), evaluatorRecord)
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
