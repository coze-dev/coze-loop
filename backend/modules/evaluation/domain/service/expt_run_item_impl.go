// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/jinzhu/copier"

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/consts"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/maps"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ExptItemEvaluation interface {
	Eval(ctx context.Context, eiec *entity.ExptItemEvalCtx) error
}

func NewExptItemEvaluation(
	turnResultRepo repo.IExptTurnResultRepo,
	itemResultRepo repo.IExptItemResultRepo,
	configer component.IConfiger,
	metric metrics.ExptMetric,
	evalTargetService IEvalTargetService,
	evaluatorRecordService EvaluatorRecordService,
	evaluatorService EvaluatorService,
	benefitService benefit.IBenefitService,
) ExptItemEvaluation {
	return &ExptItemEvalCtxExecutor{
		TurnResultRepo:         turnResultRepo,
		ItemResultRepo:         itemResultRepo,
		Configer:               configer,
		Metric:                 metric,
		evalTargetService:      evalTargetService,
		evaluatorRecordService: evaluatorRecordService,
		evaluatorService:       evaluatorService,
		benefitService:         benefitService,
	}
}

type ExptItemEvalCtxExecutor struct {
	TurnResultRepo         repo.IExptTurnResultRepo
	ItemResultRepo         repo.IExptItemResultRepo
	Configer               component.IConfiger
	Metric                 metrics.ExptMetric
	evalTargetService      IEvalTargetService
	evaluatorService       EvaluatorService
	evaluatorRecordService EvaluatorRecordService
	benefitService         benefit.IBenefitService
}

func (e *ExptItemEvalCtxExecutor) Eval(ctx context.Context, eiec *entity.ExptItemEvalCtx) error {
	event := eiec.Event

	// if err := e.SetItemRunProcessing(ctx, event.ExptID, event.ExptRunID, event.EvalSetItemID, event.SpaceID, event.Session); err != nil {
	//	return err
	// }
	if err := e.CompleteItemRun(ctx, event, e.EvalTurns(ctx, eiec)); err != nil {
		return err
	}

	return nil
}

func (e *ExptItemEvalCtxExecutor) EvalTurns(ctx context.Context, eiec *entity.ExptItemEvalCtx) error {
	var history []*entity.Message

	if eiec.EvalSetItem == nil {
		return fmt.Errorf("EvalTurns with invalid empty eval_set_item")
	}

	for _, turn := range eiec.EvalSetItem.Turns {
		etec, err := e.buildExptTurnEvalCtx(ctx, turn, eiec, history)
		if err != nil {
			return err
		}

		ctx = context.WithValue(ctx, consts.CtxKeyLogID, etec.GetTurnEvalLogID(ctx, turn.ID)) //nolint:staticcheck,SA1029

		turnRunRes := NewExptTurnEvaluation(e.Metric, e.evalTargetService, e.evaluatorService, e.benefitService).Eval(ctx, etec)

		if err := e.storeTurnRunResult(ctx, etec, turnRunRes); err != nil {
			return err
		}

		if err := turnRunRes.GetEvalErr(); err != nil {
			return err
		}

		history = append(history, buildHistoryMessage(ctx, turnRunRes)...)
	}

	time.Sleep(time.Second * 1) // 确保日志落库
	return nil
}

func (e *ExptItemEvalCtxExecutor) storeTurnRunResult(ctx context.Context, etec *entity.ExptTurnEvalCtx, result *entity.ExptTurnRunResult) error {
	if result == nil {
		return fmt.Errorf("StoreTurnRunResult with nil result")
	}

	turn := etec.Turn
	turnResultLog := etec.GetExistTurnResultLogs()[turn.ID]

	if turnResultLog == nil {
		return fmt.Errorf("storeTurnRunResult with invalid turn result log, expt_id: %v, item_id: %v, turn_id: %v", etec.Expt.ID, etec.EvalSetItem.ItemID, turn.ID)
	}

	clone := &entity.ExptTurnResultRunLog{}
	if err := copier.Copy(clone, turnResultLog); err != nil {
		return errorx.Wrapf(err, "ExptTurnResultRunLog copy fail")
	}

	var evalErr error

	clone.ExptRunID = etec.Event.ExptRunID
	if result.TargetResult != nil {
		clone.TargetResultID = result.TargetResult.ID
	}

	if result.TargetResult != nil && result.TargetResult.EvalTargetOutputData != nil && result.TargetResult.EvalTargetOutputData.EvalTargetRunError != nil && result.TargetResult.EvalTargetOutputData.EvalTargetRunError.Code > 0 {
		evalErr = errno.NewTargetResultErr(result.TargetResult.EvalTargetOutputData.EvalTargetRunError.Message)
	}

	clone.EvaluatorResultIds = &entity.EvaluatorResults{
		EvalVerIDToResID: make(map[int64]int64, len(result.EvaluatorResults)),
	}
	for _, er := range result.EvaluatorResults {
		clone.EvaluatorResultIds.EvalVerIDToResID[er.EvaluatorVersionID] = er.ID
		if er.EvaluatorOutputData != nil && er.EvaluatorOutputData.EvaluatorRunError != nil && er.EvaluatorOutputData.EvaluatorRunError.Code > 0 {
			evalErr = errno.NewEvaluatorResultErr(er.EvaluatorOutputData.EvaluatorRunError.Message)
		}
	}

	if result.EvalErr != nil {
		evalErr = result.EvalErr
	}

	if evalErr != nil {
		errMsg := e.Configer.GetErrCtrl(ctx).ConvertErrMsg(evalErr.Error())
		logs.CtxWarn(ctx, "[ExptTurnEval] store turn run err, before: %v, after: %v", evalErr, errMsg)

		ei, ok := errno.ParseErrImpl(evalErr)
		if !ok {
			clonedErr := errno.CloneErr(evalErr)
			evalErr = errno.NewTurnOtherErr(errMsg, clonedErr)
		} else {
			clonedErr := errno.CloneErr(evalErr)
			evalErr = ei.SetErrMsg(errMsg).SetCause(clonedErr)
		}

		clone.Status = entity.TurnRunState_Fail
		clone.ErrMsg = errno.SerializeErr(evalErr)
	} else {
		clone.Status = entity.TurnRunState_Success
	}

	result.SetEvalErr(evalErr)

	if err := e.TurnResultRepo.SaveTurnRunLogs(ctx, []*entity.ExptTurnResultRunLog{clone}); err != nil {
		return err
	}

	logs.CtxInfo(ctx, "[ExptTurnEval] expt turn eval finished, expt_id: %v, expt_run_id: %v, item_id: %v, turn_id: %v, run_log: %v, err: %v",
		etec.Expt.ID, etec.Event.ExptRunID, etec.EvalSetItem.ItemID, turn.ID, json.Jsonify(clone), result.EvalErr)

	return nil
}

func (e *ExptItemEvalCtxExecutor) SetItemRunProcessing(ctx context.Context, exptID, exptRunID, itemID int64, spaceID int64, session *entity.Session) error {
	return e.ItemResultRepo.UpdateItemRunLog(ctx, exptID, exptRunID, []int64{itemID}, map[string]any{"status": int32(entity.ItemRunState_Processing)}, spaceID)
}

func (e *ExptItemEvalCtxExecutor) buildExptTurnEvalCtx(ctx context.Context, turn *entity.Turn, eiec *entity.ExptItemEvalCtx, history []*entity.Message) (*entity.ExptTurnEvalCtx, error) {
	var (
		spaceID            = eiec.Event.SpaceID
		existTurnRunResult = eiec.GetExistTurnResultRunLog(turn.ID)
		etec               = &entity.ExptTurnEvalCtx{
			ExptItemEvalCtx:   eiec,
			Turn:              turn,
			ExptTurnRunResult: &entity.ExptTurnRunResult{},
			// History:           history,
		}
	)
	etec.Ext = make(map[string]string)
	for _, fieldData := range eiec.EvalSetItem.Turns[0].FieldDataList {
		if fieldData.Name == "span_id" {
			etec.Ext["span_id"] = fieldData.Content.GetText()
		}
	}
	etec.Ext["task_id"] = eiec.Expt.SourceID
	etec.Ext["workspace_id"] = strconv.FormatInt(eiec.Expt.SpaceID, 10)
	etec.Ext["start_time"] = strconv.FormatInt(gptr.Indirect(eiec.EvalSetItem.BaseInfo.CreatedAt)*1000, 10) // 存储是毫秒，需要存入微妙
	for k, v := range eiec.Event.Ext {
		etec.Ext[k] = v
	}
	if existTurnRunResult == nil {
		return etec, nil
	}

	if tid := existTurnRunResult.TargetResultID; tid > 0 {
		targetRecord, err := e.evalTargetService.GetRecordByID(ctx, spaceID, tid)
		if err != nil {
			return nil, err
		}
		etec.ExptTurnRunResult.TargetResult = targetRecord
	}

	if erids := existTurnRunResult.EvaluatorResultIds; erids != nil && len(erids.EvalVerIDToResID) > 0 {
		// evaluatorRecords, err := e.EvalCall.BatchGetEvaluatorRecord(ctx, spaceID, maps.ToSlice(erids.EvalVerIDToResID, func(k int64, v int64) int64 { return v }))
		evaluatorRecords, err := e.evaluatorRecordService.BatchGetEvaluatorRecord(ctx, maps.ToSlice(erids.EvalVerIDToResID, func(k int64, v int64) int64 { return v }), false)
		if err != nil {
			return nil, err
		}
		recordMap := make(map[int64]*entity.EvaluatorRecord)
		for _, record := range evaluatorRecords {
			recordMap[record.ID] = record
		}
		etec.ExptTurnRunResult.EvaluatorResults = recordMap
	}

	return etec, nil
}

func (e *ExptItemEvalCtxExecutor) CompleteItemRun(ctx context.Context, event *entity.ExptItemEvalEvent, evalErr error) error {
	if evalErr != nil {
		if retry, _ := e.evalErrNeedRetry(ctx, event.SpaceID, event.RetryTimes, evalErr); retry {
			return evalErr
		}
	}

	ufields := map[string]any{
		"result_state": entity.ExptItemResultStateLogged,
	}

	if evalErr != nil {
		ufields["status"] = int32(entity.ItemRunState_Fail)
		ufields["err_msg"] = errno.SerializeErr(evalErr)
	} else {
		ufields["status"] = int32(entity.ItemRunState_Success)
	}

	if err := e.ItemResultRepo.UpdateItemRunLog(ctx, event.ExptID, event.ExptRunID, []int64{event.EvalSetItemID}, ufields, event.SpaceID); err != nil {
		return err
	}

	if e.evalErrNeedTerminateExpt(ctx, event.SpaceID, evalErr) {
		logs.CtxWarn(ctx, "[ExptRecordEval] found error which should terminate expt, expt_id: %v, expt_run_id: %v, item_id: %v, err: %v", event.ExptID, event.ExptRunID, event.EvalSetItemID, evalErr)
		return evalErr
	}

	logs.CtxInfo(ctx, "[ExptRecordEval] expt item eval finished, expt_id: %v, expt_run_id: %v, success: %v, update_fields: %v", event.ExptID, event.ExptRunID, evalErr == nil, ufields)
	time.Sleep(time.Second * 2) // 确保日志落库
	return nil
}

func (e *ExptItemEvalCtxExecutor) evalErrNeedRetry(ctx context.Context, spaceID int64, retryTimes int, evalErr error) (bool, time.Duration) {
	if evalErr == nil {
		return false, 0
	}
	conf := e.Configer.GetErrRetryConf(ctx, spaceID, evalErr)
	return retryTimes < conf.GetRetryTimes(), conf.GetRetryInterval()
}

func (e *ExptItemEvalCtxExecutor) evalErrNeedTerminateExpt(ctx context.Context, spaceID int64, evalErr error) bool {
	if evalErr == nil {
		return false
	}
	conf := e.Configer.GetErrRetryConf(ctx, spaceID, evalErr)
	return conf.IsInDebt
}

func buildHistoryMessage(ctx context.Context, turnRunResult *entity.ExptTurnRunResult) []*entity.Message {
	return nil
}
