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

// ExptItemTurnEvaluation evaluation execution process
type ExptItemTurnEvaluation interface {
	Eval(ctx context.Context, etec *entity.ExptTurnEvalCtx) *entity.ExptTurnRunResult
}

func NewExptTurnEvaluation(
	metric metrics.ExptMetric,
	evalTargetService IEvalTargetService,
	evaluatorService EvaluatorService,
	benefitService benefit.IBenefitService,
	evalAsyncRepo repo.IEvalAsyncRepo,
	evalSetItemSvc EvaluationSetItemService,
) ExptItemTurnEvaluation {
	return &DefaultExptTurnEvaluationImpl{
		metric:            metric,
		evalTargetService: evalTargetService,
		evaluatorService:  evaluatorService,
		benefitService:    benefitService,
		evalAsyncRepo:     evalAsyncRepo,
		evalSetItemSvc:    evalSetItemSvc,
	}
}

type DefaultExptTurnEvaluationImpl struct {
	metric            metrics.ExptMetric
	evalTargetService IEvalTargetService
	evaluatorService  EvaluatorService
	benefitService    benefit.IBenefitService
	evalAsyncRepo     repo.IEvalAsyncRepo
	evalSetItemSvc    EvaluationSetItemService
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

	if existRecord := e.existedTargetRecord(etec); !etec.Event.IgnoreExistedResult() && existRecord != nil {
		logs.CtxInfo(ctx, "CallTarget return with existed target record, record_id: %v", existRecord.ID)
		return existRecord, nil
	}

	if etec.Event.AsyncReportTrigger {
		if etec.ExptTurnRunResult == nil || etec.ExptTurnRunResult.TargetResult == nil {
			return nil, errorx.NewByCode(errno.CommonInternalErrorCode, errorx.WithExtraMsg("target result must not be nil in async reported event"))
		}
		return etec.ExptTurnRunResult.TargetResult, nil
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

	inputFields, err := func() (map[string]*entity.Content, error) {
		if targetConf.IngressConf == nil || targetConf.IngressConf.EvalSetAdapter == nil {
			return nil, nil
		}
		switch etec.Expt.Target.EvalTargetType {
		case entity.EvalTargetTypeCustomRPCServer:
			fields := gslice.ToMap(turn.FieldDataList, func(t *entity.FieldData) (string, *entity.Content) { return t.Name, t.Content })
			for name, content := range fields {
				if content.IsContentOmitted() {
					req := &entity.GetEvaluationSetItemFieldParam{
						SpaceID:         spaceID,
						EvaluationSetID: turn.EvalSetID,
						ItemPK:          turn.ItemID,
						FieldName:       name,
						TurnID:          gptr.Of(turn.ID),
					}
					logs.CtxInfo(ctx, "found omitted content turn, turn_info: %v", json.Jsonify(req))
					fd, err := e.evalSetItemSvc.GetEvaluationSetItemField(ctx, req)
					if err != nil {
						return nil, err
					}
					fields[name] = fd.Content
				}
			}
			return fields, nil
		default:
			return e.buildEvalSetFields(ctx, spaceID, targetConf.IngressConf.EvalSetAdapter.FieldConfs, turn)
		}
	}()
	if err != nil {
		return nil, err
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
		ExperimentID:    gptr.Of(etec.Event.ExptID),
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
		RecordID:    targetRecord.ID,
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

		if !etec.Event.IgnoreExistedResult() && existResult != nil && existResult.Status == entity.EvaluatorRunStatusSuccess {
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

		inputData, err := e.buildEvaluatorInputData(ctx, spaceID, ev.EvaluatorType, ec, turn, targetFields, ev.GetInputSchemas(), etec.Ext)
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
				Ext:                etec.Ext,
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

func (e *DefaultExptTurnEvaluationImpl) buildEvaluatorInputData(ctx context.Context, spaceID int64, evaluatorType entity.EvaluatorType,
	ec *entity.EvaluatorConf, evalSetTurn *entity.Turn, targetFields map[string]*entity.Content, inputSchemas []*entity.ArgsSchema, ext map[string]string,
) (*entity.EvaluatorInputData, error) {
	fromEvalSet, err := e.buildEvalSetFields(ctx, spaceID, ec.IngressConf.EvalSetAdapter.FieldConfs, evalSetTurn)
	if err != nil {
		return nil, err
	}
	fromTarget, err := e.buildFieldsFromSource(ctx, ec.IngressConf.TargetAdapter.FieldConfs, targetFields, evaluatorType, inputSchemas)
	if err != nil {
		return nil, err
	}

	res := &entity.EvaluatorInputData{InputFields: make(map[string]*entity.Content)}
	switch evaluatorType {
	case entity.EvaluatorTypeCode:
		res.EvaluateDatasetFields = fromEvalSet
		res.EvaluateTargetOutputFields = fromTarget
	case entity.EvaluatorTypeCustomRPC:
		if len(inputSchemas) == 0 { // 无input_schemas的自定义服务评估器
			res.EvaluateDatasetFields = fromEvalSet
			res.EvaluateTargetOutputFields = fromTarget
		} else { // 有input_schemas的自定义服务评估器
			for _, fieldCnt := range []map[string]*entity.Content{fromEvalSet, fromTarget} {
				for key, content := range fieldCnt {
					res.InputFields[key] = content
				}
			}
		}
	default:
		for _, fieldCnt := range []map[string]*entity.Content{fromEvalSet, fromTarget} {
			for key, content := range fieldCnt {
				res.InputFields[key] = content
			}
		}
	}

	res.Ext = e.buildEvaluatorInputDataExt(ext, ec.RunConf)

	return res, nil
}

// buildFieldsFromSource build field mapping from specified data source, extracting common field processing logic
func (e *DefaultExptTurnEvaluationImpl) buildFieldsFromSource(ctx context.Context, fieldConfs []*entity.FieldConf,
	sourceFields map[string]*entity.Content, evaluatorType entity.EvaluatorType, inputSchemas []*entity.ArgsSchema,
) (map[string]*entity.Content, error) {
	if evaluatorType == entity.EvaluatorTypeCode || (evaluatorType == entity.EvaluatorTypeCustomRPC && len(inputSchemas) == 0) {
		return sourceFields, nil
	}
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

func (e *DefaultExptTurnEvaluationImpl) buildEvalSetFields(ctx context.Context, spaceID int64, fcs []*entity.FieldConf, evalSetTurn *entity.Turn) (map[string]*entity.Content, error) {
	result := make(map[string]*entity.Content)
	fields := gcond.IfLazyL(evalSetTurn != nil && len(evalSetTurn.FieldDataList) > 0, func() map[string]*entity.Content {
		return gslice.ToMap(evalSetTurn.FieldDataList, func(t *entity.FieldData) (string, *entity.Content) { return t.Name, t.Content })
	}, nil)

	for _, fc := range fcs {
		content, err := e.getFieldContent(fc, fields)
		if err != nil {
			return nil, err
		}
		if content.IsContentOmitted() {
			req := &entity.GetEvaluationSetItemFieldParam{
				SpaceID:         spaceID,
				EvaluationSetID: evalSetTurn.EvalSetID,
				ItemPK:          evalSetTurn.ItemID,
				FieldName:       fc.FromField,
				TurnID:          gptr.Of(evalSetTurn.ID),
			}
			logs.CtxInfo(ctx, "found omitted content turn, turn_info: %v", json.Jsonify(req))
			fd, err := e.evalSetItemSvc.GetEvaluationSetItemField(ctx, req)
			if err != nil {
				return nil, err
			}
			content = fd.Content
		}
		result[fc.FieldName] = content
	}

	return result, nil
}

// getFieldContent get field content, handling JSON Path logic
func (e *DefaultExptTurnEvaluationImpl) getFieldContent(
	fc *entity.FieldConf,
	sourceFields map[string]*entity.Content,
) (*entity.Content, error) {
	firstField, err := json.GetFirstJSONPathField(fc.FromField)
	if err != nil {
		return nil, err
	}

	if firstField == fc.FromField {
		// No drill-down fields, return directly
		return sourceFields[fc.FromField], nil
	} else {
		// Has drill-down fields, process via JSON Path
		return e.getContentByJsonPath(sourceFields[firstField], fc.FromField)
	}
}

// Note: This function has specialized logic and cannot be directly reused; it removes the first level of jsonpath.
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

func (e *DefaultExptTurnEvaluationImpl) buildEvaluatorInputDataExt(ext map[string]string, runConf *entity.EvaluatorRunConfig) map[string]string {
	builtExt := gmap.Clone(ext)
	if builtExt == nil {
		builtExt = make(map[string]string)
	}
	if runConf != nil && runConf.EvaluatorRuntimeParam != nil && runConf.EvaluatorRuntimeParam.JSONValue != nil && len(*runConf.EvaluatorRuntimeParam.JSONValue) > 0 {
		builtExt[consts.FieldAdapterBuiltinFieldNameRuntimeParam] = *runConf.EvaluatorRuntimeParam.JSONValue
	}

	return builtExt
}
