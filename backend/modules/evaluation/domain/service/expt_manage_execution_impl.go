// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"
	"github.com/samber/lo"

	"github.com/coze-dev/coze-loop/backend/infra/external/audit"
	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/encoding"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/conv"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ExptCheckFn = func(ctx context.Context, expt *entity.Experiment, session *entity.Session) error

func (e *ExptMangerImpl) CheckRun(ctx context.Context, expt *entity.Experiment, spaceID int64, session *entity.Session, opts ...entity.ExptRunCheckOptionFn) error {
	opt := &entity.ExptRunCheckOption{}
	for _, fn := range opts {
		fn(opt)
	}

	checkers := []ExptCheckFn{
		e.CheckExpt,
		e.CheckTarget,
		e.CheckEvalSet,
		e.CheckEvaluators,
		e.CheckConnector,
	}

	if expt.ExptType == entity.ExptType_Offline {
		if opt.CheckBenefit {
			checkers = append(checkers, e.CheckBenefit)
		}
	}

	for _, check := range checkers {
		if err := check(ctx, expt, session); err != nil {
			return err
		}
	}

	return nil
}

func (e *ExptMangerImpl) CheckEvalSet(ctx context.Context, expt *entity.Experiment, session *entity.Session) error {
	switch expt.ExptType {
	case entity.ExptType_Offline:
		if expt.EvalSetVersionID == 0 || expt.EvalSet == nil || expt.EvalSet.EvaluationSetVersion == nil {
			return errorx.NewByCode(errno.ExperimentValidateFailCode, errorx.WithExtraMsg(fmt.Sprintf("with invalid EvalSetVersion %d", expt.EvalSetVersionID)))
		}
		if expt.EvalSet.EvaluationSetVersion.ItemCount <= 0 {
			return errorx.NewByCode(errno.ExperimentValidateFailCode, errorx.WithExtraMsg(fmt.Sprintf("with empty EvalSetVersion %d", expt.EvalSetVersionID)))
		}
	case entity.ExptType_Online:
		if expt.EvalSet == nil {
			return errorx.NewByCode(errno.ExperimentValidateFailCode, errorx.WithExtraMsg(fmt.Sprintf("with empty EvalSet: %d", expt.EvalSetID)))
		}
	default:
	}

	return nil
}

func (e *ExptMangerImpl) CheckExpt(ctx context.Context, expt *entity.Experiment, session *entity.Session) error {
	// audit
	data := map[string]string{
		"texts": strings.Join([]string{expt.Name, expt.Description}, ","),
	}
	record, err := e.audit.Audit(ctx, audit.AuditParam{
		ObjectID:  expt.ID,
		AuditType: audit.AuditType_CozeLoopExptModify,
		AuditData: data,
		ReqID:     encoding.Encode(ctx, data),
	})
	if err != nil {
		logs.CtxError(ctx, "audit: failed to audit, err=%v", err) // 审核服务不可用，默认通过
	}
	if record.AuditStatus == audit.AuditStatus_Rejected {
		return errorx.NewByCode(errno.RiskContentDetectedCode)
	}

	// evaluate configuration
	if expt.EvalConf == nil {
		return errorx.NewByCode(errno.ExperimentValidateFailCode, errorx.WithExtraMsg("EvalConfig is invalid"))
	}
	if gptr.Indirect(expt.EvalConf.ItemConcurNum) > consts.MaxItemConcurrentNum {
		return errorx.NewByCode(errno.ExperimentValidateFailCode, errorx.WithExtraMsg(fmt.Sprintf("item concurrent num must not be greater than %d", consts.MaxEvalSetItemLimit)))
	}

	return nil
}

func (e *ExptMangerImpl) CheckTarget(ctx context.Context, expt *entity.Experiment, session *entity.Session) error {
	if expt.EvalConf == nil || expt.EvalConf.ConnectorConf.TargetConf == nil {
		return nil
	}
	if expt.TargetID == 0 || expt.TargetVersionID == 0 || expt.Target == nil {
		return errorx.NewByCode(errno.ExperimentValidateFailCode, errorx.WithExtraMsg(fmt.Sprintf("with invalid target, target_id= %d target_version_id= %d", expt.TargetID, expt.TargetVersionID)))
	}
	return nil
}

func (e *ExptMangerImpl) CheckEvaluators(ctx context.Context, expt *entity.Experiment, session *entity.Session) error {
	if expt.EvalConf == nil || expt.EvalConf.ConnectorConf.EvaluatorsConf == nil || len(expt.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConf) == 0 {
		return nil
	}
	if len(expt.EvaluatorVersionRef) == 0 || len(expt.Evaluators) != len(expt.EvaluatorVersionRef) {
		return errorx.NewByCode(errno.ExperimentValidateFailCode, errorx.WithExtraMsg(fmt.Sprintf("with invalid evaluators %v", expt.EvaluatorVersionRef)))
	}
	return nil
}

func (e *ExptMangerImpl) CheckConnector(ctx context.Context, expt *entity.Experiment, session *entity.Session) error {
	if expt.EvalConf == nil {
		return errorx.NewByCode(errno.ExperimentValidateFailCode, errorx.WithExtraMsg("EvalConfig is nil"))
	}

	if err := e.checkTargetConnector(ctx, expt, session); err != nil {
		return err
	}

	if err := e.checkEvaluatorsConnector(ctx, expt, session); err != nil {
		return err
	}

	return nil
}

func (e *ExptMangerImpl) checkTargetConnector(ctx context.Context, expt *entity.Experiment, session *entity.Session) error {
	if expt.Target == nil ||
		expt.Target.EvalTargetType == entity.EvalTargetTypeLoopTrace {
		return nil
	}

	e.fixTargetConf(expt)
	connectorConf := expt.EvalConf.ConnectorConf
	if err := connectorConf.TargetConf.Valid(ctx, expt.Target.EvalTargetType); err != nil {
		return errorx.WrapByCode(err, errno.ExperimentValidateFailCode, errorx.WithExtraMsg("invalid target connector"))
	}

	evalSetFieldSchema := gslice.ToMap(expt.EvalSet.EvaluationSetVersion.EvaluationSetSchema.FieldSchemas, func(t *entity.FieldSchema) (string, *entity.FieldSchema) { return t.Name, t })
	for _, fc := range connectorConf.TargetConf.IngressConf.EvalSetAdapter.FieldConfs {
		firstField, err := json.GetFirstJSONPathField(fc.FromField)
		if err != nil {
			return errorx.WrapByCode(err, errno.ExperimentValidateFailCode, errorx.WithExtraMsg(fmt.Sprintf("invalid connector: target is expected to receive the missing evalset %v column, json parse error", fc.FromField)))
		}
		if esf := evalSetFieldSchema[firstField]; esf == nil {
			return errorx.NewByCode(errno.ExperimentValidateFailCode, errorx.WithExtraMsg(fmt.Sprintf("invalid connector: target is expected to receive the missing evalset %v column", fc.FromField)))
		}
	}

	if cc := expt.EvalConf.ConnectorConf.TargetConf.IngressConf.CustomConf; cc != nil {
		for _, fc := range cc.FieldConfs {
			if fc.FieldName == consts.FieldAdapterBuiltinFieldNameRuntimeParam {
				if err := e.evalTargetService.ValidateRuntimeParam(ctx, expt.TargetType, fc.Value); err != nil {
					logs.CtxError(ctx, "parse type %s runtime param fail, raw: %v, err: %v", expt.TargetType, fc.Value, err)
					return errorx.NewByCode(errno.ExperimentValidateFailCode, errorx.WithExtraMsg("invalid runtime param"))
				}
			}
		}
	}

	return nil
}

func (e *ExptMangerImpl) fixTargetConf(expt *entity.Experiment) {
	switch expt.TargetType {
	case entity.EvalTargetTypeLoopPrompt:
		if expt.EvalConf.ConnectorConf.TargetConf == nil {
			expt.EvalConf.ConnectorConf.TargetConf = &entity.TargetConf{
				TargetVersionID: expt.TargetVersionID,
				IngressConf: &entity.TargetIngressConf{
					EvalSetAdapter: &entity.FieldAdapter{},
				},
			}
		}
	default:
	}
}

func (e *ExptMangerImpl) checkEvaluatorsConnector(ctx context.Context, expt *entity.Experiment, session *entity.Session) error {
	if len(expt.Evaluators) == 0 {
		return nil
	}
	connectorConf := expt.EvalConf.ConnectorConf
	if err := connectorConf.EvaluatorsConf.Valid(ctx); err != nil {
		return errorx.WrapByCode(err, errno.ExperimentValidateFailCode, errorx.WithExtraMsg("invalid evaluator connector"))
	}

	targetOutputSchema := lo.TernaryF(expt.Target == nil || expt.Target.EvalTargetVersion == nil || expt.Target.EvalTargetVersion.OutputSchema == nil, func() map[string]*entity.ArgsSchema {
		return nil
	}, func() map[string]*entity.ArgsSchema {
		return gslice.ToMap(expt.Target.EvalTargetVersion.OutputSchema, func(t *entity.ArgsSchema) (string, *entity.ArgsSchema) {
			if t.Key == nil {
				return "", nil
			}
			return *t.Key, t
		})
	})

	evalSetFieldSchema := gslice.ToMap(expt.EvalSet.EvaluationSetVersion.EvaluationSetSchema.FieldSchemas, func(t *entity.FieldSchema) (string, *entity.FieldSchema) { return t.Name, t })
	for _, evaluatorConf := range connectorConf.EvaluatorsConf.EvaluatorConf {
		for _, fc := range evaluatorConf.IngressConf.EvalSetAdapter.FieldConfs {
			firstField, err := json.GetFirstJSONPathField(fc.FromField)
			if err != nil {
				return errorx.WrapByCode(err, errno.ExperimentValidateFailCode, errorx.WithExtraMsg(fmt.Sprintf("invalid connector: evaluator %v is expected to receive the missing evalset %v column, json parse error", evaluatorConf.EvaluatorVersionID, fc.FromField)))
			}
			if fs := evalSetFieldSchema[firstField]; fs == nil {
				return errorx.NewByCode(errno.ExperimentValidateFailCode, errorx.WithExtraMsg(fmt.Sprintf("invalid connector: evaluator %v is expected to receive the missing evalset %v column", evaluatorConf.EvaluatorVersionID, fc.FromField)))
			}
		}
		if expt.Target != nil && expt.Target.EvalTargetType != entity.EvalTargetTypeLoopTrace {
			for _, fc := range evaluatorConf.IngressConf.TargetAdapter.FieldConfs {
				firstField, err := json.GetFirstJSONPathField(fc.FromField)
				if err != nil {
					return errorx.WrapByCode(err, errno.ExperimentValidateFailCode, errorx.WithExtraMsg(fmt.Sprintf("invalid connector: evaluator %v is expected to receive the missing target %v column, json parse error", evaluatorConf.EvaluatorVersionID, fc.FromField)))
				}
				if s := targetOutputSchema[firstField]; s == nil {
					return errorx.NewByCode(errno.ExperimentValidateFailCode, errorx.WithExtraMsg(fmt.Sprintf("invalid connector: evaluator %v is expected to receive the missing target %v field", evaluatorConf.EvaluatorVersionID, fc.FromField)))
				}
			}
		}
	}

	return nil
}

func (e *ExptMangerImpl) CheckBenefit(ctx context.Context, expt *entity.Experiment, session *entity.Session) error {
	if expt.CreditCost == entity.CreditCostFree {
		logs.CtxInfo(ctx, "CheckBenefit with credit cost already freed, expt_id: %v", expt.ID)
		return nil
	}
	req := &benefit.CheckAndDeductEvalBenefitParams{
		ConnectorUID: session.UserID,
		SpaceID:      expt.SpaceID,
		ExperimentID: expt.ID,
		Ext:          map[string]string{benefit.ExtKeyExperimentFreeCost: strconv.FormatBool(expt.CreditCost == entity.CreditCostFree)},
	}

	result, err := e.benefitService.CheckAndDeductEvalBenefit(ctx, req)
	logs.CtxInfo(ctx, "[CheckAndDeductEvalBenefit][req = %s] [res = %s] [err = %v]", json.Jsonify(req), json.Jsonify(result))
	if err != nil {
		return errorx.Wrapf(err, "CheckAndDeductEvalBenefit fail, expt_id: %v, user_id: %v", expt.ID, session.UserID)
	}

	if result != nil && result.DenyReason != nil && result.DenyReason.ToErr() != nil {
		return result.DenyReason.ToErr()
	}

	if result.IsFreeEvaluate != nil && *result.IsFreeEvaluate {
		if err := e.exptRepo.Update(ctx, &entity.Experiment{
			ID:         expt.ID,
			SpaceID:    expt.SpaceID,
			CreditCost: entity.CreditCostFree,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (e *ExptMangerImpl) Run(ctx context.Context, exptID, runID, spaceID int64, session *entity.Session, runMode entity.ExptRunMode, ext map[string]string) error {
	if err := NewQuotaService(e.quotaRepo, e.configer).AllowExptRun(ctx, exptID, spaceID, session); err != nil {
		return err
	}

	expt, err := e.GetDetail(ctx, exptID, spaceID, session)
	if err != nil {
		return err
	}

	if err := e.publisher.PublishExptScheduleEvent(ctx, &entity.ExptScheduleEvent{
		SpaceID:     spaceID,
		ExptID:      exptID,
		ExptRunID:   runID,
		ExptRunMode: runMode,
		ExptType:    expt.ExptType,
		CreatedAt:   time.Now().Unix(),
		Session:     session,
		Ext:         ext,
	}, gptr.Of(time.Second*3)); err != nil {
		return err
	}

	return nil
}

func (e *ExptMangerImpl) RetryUnSuccess(ctx context.Context, exptID, runID, spaceID int64, session *entity.Session, ext map[string]string) error {
	if err := NewQuotaService(e.quotaRepo, e.configer).AllowExptRun(ctx, exptID, spaceID, session); err != nil {
		return err
	}

	expt, err := e.GetDetail(ctx, exptID, spaceID, session)
	if err != nil {
		return err
	}

	if err := e.publisher.PublishExptScheduleEvent(ctx, &entity.ExptScheduleEvent{
		SpaceID:     spaceID,
		ExptID:      exptID,
		ExptRunID:   runID,
		ExptRunMode: entity.EvaluationModeFailRetry,
		ExptType:    expt.ExptType,
		CreatedAt:   time.Now().Unix(),
		Session:     session,
		Ext:         ext,
	}, gptr.Of(time.Second*3)); err != nil {
		return err
	}

	return nil
}

func (e *ExptMangerImpl) CompleteRun(ctx context.Context, exptID, exptRunID int64, mode entity.ExptRunMode, spaceID int64, session *entity.Session, opts ...entity.CompleteExptOptionFn) error {
	const idemKeyPrefix = "CompleteRun:"

	opt := &entity.CompleteExptOption{}
	for _, fn := range opts {
		fn(opt)
	}
	time.Sleep(time.Second * 3)

	if len(opt.CID) > 0 {
		if exist, err := e.idem.Exist(ctx, idemKeyPrefix+opt.CID); err != nil {
			logs.CtxInfo(ctx, "Exist fail, key: %v", opt.CID)
		} else {
			if exist {
				logs.CtxInfo(ctx, "CompleteRun SetNX with duplicate request, cid: %v", opt.CID)
				return nil
			}
		}
	}

	runLog, err := e.runLogRepo.Get(ctx, exptID, exptRunID)
	if err != nil {
		return err
	}

	if err := e.calculateRunLogStats(ctx, exptID, exptRunID, runLog, spaceID, session); err != nil {
		return err
	}

	if _, err := e.mutex.Unlock(e.makeExptMutexLockKey(exptID)); err != nil {
		return err
	}

	if opt.Status > 0 {
		runLog.Status = int64(opt.Status)
	}
	if len(opt.StatusMessage) > 0 {
		runLog.StatusMessage = conv.UnsafeStringToBytes(opt.StatusMessage)
	}

	logs.CtxInfo(ctx, "[ExptEval] CompleteRun, expt_id: %v, expt_run_id: %v, status: %v, msg: %v", exptID, exptRunID, runLog.Status, opt.StatusMessage)

	if err := e.runLogRepo.Save(ctx, runLog); err != nil {
		return err
	}

	if len(opt.CID) > 0 {
		if err := e.idem.Set(ctx, idemKeyPrefix+opt.CID, time.Second*60*3); err != nil {
			logs.CtxWarn(ctx, "CompleteRun SetNX fail, err: %v", err)
		}
	}

	return nil
}

func (e *ExptMangerImpl) calculateRunLogStats(ctx context.Context, exptID, exptRunID int64, runLog *entity.ExptRunLog, spaceID int64, session *entity.Session) error {
	var (
		maxLoop = 10000
		limit   = 100
		total   = 0
		cnt     = 0
		page    = 1

		pendingCnt    = 0
		failCnt       = 0
		successCnt    = 0
		terminatedCnt = 0
		processingCnt = 0
	)

	for i := 0; i < maxLoop; i++ {
		logs.CtxInfo(ctx, "calculateRunLogStats scan turn result, expt_id: %v, expt_run_id: %v, page: %v, limit: %v, cur_cnt: %v, total: %v",
			exptID, exptRunID, page, limit, cnt, total)

		results, t, err := e.turnResultRepo.ListTurnResult(ctx, spaceID, exptID, nil, entity.NewPage(page, limit), false)
		if err != nil {
			return err
		}

		page++
		total = int(t)
		cnt += len(results)

		for _, tr := range results {
			switch entity.TurnRunState(tr.Status) {
			case entity.TurnRunState_Success:
				successCnt++
			case entity.TurnRunState_Fail:
				failCnt++
			case entity.TurnRunState_Terminal:
				terminatedCnt++
			case entity.TurnRunState_Queueing:
				pendingCnt++
			case entity.TurnRunState_Processing:
				processingCnt++
			default:
			}
		}

		if cnt >= total || len(results) == 0 {
			break
		}

		time.Sleep(time.Millisecond * 30)
	}

	runLog.PendingCnt = int32(pendingCnt)
	runLog.FailCnt = int32(failCnt)
	runLog.SuccessCnt = int32(successCnt)
	runLog.ProcessingCnt = int32(processingCnt)
	runLog.TerminatedCnt = int32(terminatedCnt)

	if runLog.PendingCnt > 0 || runLog.FailCnt > 0 {
		runLog.Status = int64(entity.ExptStatus_Failed)
	} else {
		runLog.Status = int64(entity.ExptStatus_Success)
	}

	logs.CtxInfo(ctx, "calculateRunLogStats done, expt_id: %v, scan turn cnt: %v, total: %v, run_log: %v, unsuccess_item_ids: %v", exptID, cnt, total, json.Jsonify(runLog))

	return nil
}

func (e *ExptMangerImpl) CompleteExpt(ctx context.Context, exptID, spaceID int64, session *entity.Session, opts ...entity.CompleteExptOptionFn) error {
	const idemKeyPrefix = "CompleteExpt:"

	opt := &entity.CompleteExptOption{}
	for _, fn := range opts {
		fn(opt)
	}
	time.Sleep(time.Second * 3)

	if len(opt.CID) > 0 {
		if exist, err := e.idem.Exist(ctx, idemKeyPrefix+opt.CID); err != nil {
			logs.CtxInfo(ctx, "Exist fail, key: %v", opt.CID)
		} else {
			if exist {
				logs.CtxInfo(ctx, "CompleteExpt SetNX with duplicate request, cid: %v", opt.CID)
				return nil
			}
		}
	}

	got, err := e.exptRepo.GetByID(ctx, exptID, spaceID)
	if err != nil {
		if se, ok := errorx.FromStatusError(err); ok && se.Code() == errno.ResourceNotFoundCode {
			logs.CtxInfo(ctx, "[ExptEval] CompleteExpt abort with deleted expt, expt_id: %v", exptID)
			return nil
		}
		return err
	}

	err = e.publisher.PublishExptAggrCalculateEvent(ctx, []*entity.AggrCalculateEvent{
		{
			ExperimentID:  exptID,
			SpaceID:       spaceID,
			CalculateMode: entity.CreateAllFields,
		},
	}, gptr.Of(time.Second*3))
	if err != nil {
		return err
	}

	stats, err := e.exptResultService.CalculateStats(ctx, exptID, spaceID, session)
	if err != nil {
		return err
	}

	exptStats := &entity.ExptStats{
		SuccessItemCnt:    int32(stats.SuccessItemCnt),
		PendingItemCnt:    int32(stats.PendingItemCnt),
		FailItemCnt:       int32(stats.FailItemCnt),
		ProcessingItemCnt: int32(stats.ProcessingItemCnt),
		TerminatedItemCnt: int32(stats.TerminatedItemCnt),
	}

	if err := e.statsRepo.UpdateByExptID(ctx, exptID, spaceID, exptStats); err != nil {
		return err
	}

	status := opt.Status
	if !entity.IsExptFinished(status) {
		if stats.FailItemCnt > 0 || stats.TerminatedItemCnt > 0 || len(stats.IncompleteTurnIDs) > 0 {
			status = entity.ExptStatus_Failed
		} else {
			status = entity.ExptStatus_Success
		}
	}

	if status == entity.ExptStatus_Terminated {
		for _, chunk := range gslice.Chunk(stats.IncompleteTurnIDs, 30) {
			if err := e.terminateItemTurns(ctx, exptID, chunk, spaceID, session); err != nil {
				logs.CtxWarn(ctx, "terminateItemTurns fail, err: %v", err)
				continue
			}
			time.Sleep(time.Millisecond * 50)
		}
	}

	exptDo := &entity.Experiment{
		ID:      exptID,
		SpaceID: spaceID,
		Status:  status,
		EndAt:   gptr.Of(time.Now()),
	}

	if len(opt.StatusMessage) > 0 {
		exptDo.StatusMessage = opt.StatusMessage
	}

	if err := e.exptRepo.Update(ctx, exptDo); err != nil {
		return err
	}

	if err := NewQuotaService(e.quotaRepo, e.configer).ReleaseExptRun(ctx, exptID, spaceID, session); err != nil {
		return err
	}

	if len(opt.CID) > 0 {
		if err := e.idem.Set(ctx, idemKeyPrefix+opt.CID, time.Second*60*3); err != nil {
			logs.CtxWarn(ctx, "CompleteExpt SetNX fail, err: %v", err)
		}
	}

	e.mtr.EmitExptExecResult(spaceID, int64(got.ExptType), int64(status), gptr.Indirect(got.StartAt))
	logs.CtxInfo(ctx, "[ExptEval] CompleteExpt success, expt_id: %v, status: %v, stats: %v", exptID, status, json.Jsonify(stats))

	return nil
}

func (e *ExptMangerImpl) terminateItemTurns(ctx context.Context, exptID int64, itemTurnIDs []*entity.ItemTurnID, spaceID int64, session *entity.Session) error {
	itemIDs := make([]int64, 0, len(itemTurnIDs))
	for _, itemTurnID := range itemTurnIDs {
		itemIDs = append(itemIDs, itemTurnID.ItemID)
	}

	logs.CtxInfo(ctx, "terminate expt item/turn result with item_ids: %v", itemIDs)

	if err := e.itemResultRepo.UpdateItemsResult(ctx, spaceID, exptID, itemIDs, map[string]any{
		"status": int32(entity.ItemRunState_Terminal),
	}); err != nil {
		return err
	}

	if err := e.turnResultRepo.UpdateTurnResults(ctx, exptID, itemTurnIDs, spaceID, map[string]any{
		"status": int32(entity.TurnRunState_Terminal),
	}); err != nil {
		return err
	}

	return nil
}

func (e *ExptMangerImpl) Kill(ctx context.Context, exptID, spaceID int64, msg string, session *entity.Session) error {
	return e.CompleteExpt(ctx, exptID, spaceID, session, entity.WithStatus(entity.ExptStatus_Terminated), entity.WithStatusMessage(msg))
}

func (e *ExptMangerImpl) Invoke(ctx context.Context, invokeExptReq *entity.InvokeExptReq) error {
	if len(invokeExptReq.Items) == 0 {
		return nil
	}
	var (
		itemIdx = int32(0)
		itemCnt = 0
		total   = int64(0)
	)
	existItemIDList, err := e.itemResultRepo.GetItemIDListByExptID(ctx, invokeExptReq.SpaceID, invokeExptReq.ExptID)
	if err != nil {
		return err
	}
	toSubmitItems := make([]*entity.EvaluationSetItem, 0, len(invokeExptReq.Items))
	for _, item := range invokeExptReq.Items {
		if gslice.Contains(existItemIDList, item.ItemID) {
			logs.CtxInfo(ctx, "InvokeExpt with exist item, expt_id: %v, item_id: %v", invokeExptReq.ExptID, item.ItemID)
			continue
		}
		toSubmitItems = append(toSubmitItems, item)
	}
	if len(toSubmitItems) == 0 {
		logs.CtxInfo(ctx, "InvokeExpt with no new item, expt_id: %v", invokeExptReq.ExptID)
		return nil
	}
	maxItemIdx, err := e.itemResultRepo.GetMaxItemIdxByExptID(ctx, invokeExptReq.ExptID, invokeExptReq.SpaceID)
	logs.CtxInfo(ctx, "GetMaxItemIdxByExptID, expt_id: %v, max_item_idx: %v", invokeExptReq.ExptID, maxItemIdx)
	if err != nil {
		logs.CtxError(ctx, "GetMaxItemIdxByExptID fail, err: %v", err)
	} else {
		itemIdx = maxItemIdx + 1
	}
	itemCnt += len(toSubmitItems)

	turnCnt := 0
	for _, item := range toSubmitItems {
		turnCnt += len(item.Turns)
	}

	ids, err := e.idgenerator.GenMultiIDs(ctx, len(toSubmitItems)+turnCnt)
	if err != nil {
		return err
	}

	idIdx := 0
	eirs := make([]*entity.ExptItemResult, 0, len(toSubmitItems))
	etrs := make([]*entity.ExptTurnResult, 0, len(toSubmitItems))
	for _, item := range toSubmitItems {
		eir := &entity.ExptItemResult{
			ID:        ids[idIdx],
			SpaceID:   invokeExptReq.SpaceID,
			ExptID:    invokeExptReq.ExptID,
			ExptRunID: invokeExptReq.RunID,
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
				SpaceID:   invokeExptReq.SpaceID,
				ExptID:    invokeExptReq.ExptID,
				ExptRunID: invokeExptReq.RunID,
				ItemID:    item.ItemID,
				TurnID:    turn.ID,
				TurnIdx:   int32(turnIdx),
				Status:    int32(entity.TurnRunState_Queueing),
			}
			etrs = append(etrs, etr)
			idIdx++
		}
	}

	// 创建result
	if err := e.createItemTurnResults(ctx, eirs, etrs); err != nil {
		return err
	}

	time.Sleep(time.Millisecond * 30)

	logs.CtxInfo(ctx, "ExptAppendExec.Append ListEvaluationSetItem done, expt_id: %v, itemCnt: %v, total: %v", invokeExptReq.ExptID, itemCnt, total)

	// 更新stats
	if err = e.statsRepo.ArithOperateCount(ctx, invokeExptReq.ExptID, invokeExptReq.SpaceID, &entity.StatsCntArithOp{
		OpStatusCnt: map[entity.ItemRunState]int{
			entity.ItemRunState_Queueing: itemCnt,
		},
	}); err != nil {
		return err
	}

	expt, err := e.GetDetail(ctx, invokeExptReq.ExptID, invokeExptReq.SpaceID, invokeExptReq.Session)
	if err != nil {
		return err
	}

	if err = e.publisher.PublishExptScheduleEvent(ctx, &entity.ExptScheduleEvent{
		SpaceID:     invokeExptReq.SpaceID,
		ExptID:      invokeExptReq.ExptID,
		ExptRunID:   invokeExptReq.RunID,
		ExptRunMode: entity.EvaluationModeAppend,
		ExptType:    expt.ExptType,
		CreatedAt:   time.Now().Unix(),
		Session:     invokeExptReq.Session,
		Ext:         invokeExptReq.Ext,
	}, gptr.Of(time.Second*3)); err != nil {
		return err
	}

	return nil
}

func (e *ExptMangerImpl) createItemTurnResults(ctx context.Context, eirs []*entity.ExptItemResult, etrs []*entity.ExptTurnResult) error {
	if err := e.turnResultRepo.BatchCreateNX(ctx, etrs); err != nil {
		return err
	}

	if err := e.itemResultRepo.BatchCreateNX(ctx, eirs); err != nil {
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

	if err = e.itemResultRepo.BatchCreateNXRunLogs(ctx, eirLogs); err != nil {
		return err
	}

	return nil
}

func (e *ExptMangerImpl) Finish(ctx context.Context, expt *entity.Experiment, exptRunID int64, session *entity.Session) error {
	const idemKeyPrefix = "FinishExpt:"
	if exist, err := e.idem.Exist(ctx, idemKeyPrefix+strconv.FormatInt(expt.ID, 10)); err != nil {
		logs.CtxInfo(ctx, "Exist fail, key: %v", strconv.FormatInt(expt.ID, 10))
	} else {
		if exist {
			logs.CtxInfo(ctx, "FinishExpt SetNX with duplicate request, expt_id: %v", strconv.FormatInt(expt.ID, 10))
			return nil
		}
	}

	exptDo := &entity.Experiment{
		ID:      expt.ID,
		SpaceID: expt.SpaceID,
		Status:  entity.ExptStatus_Draining,
	}
	err := e.exptRepo.Update(ctx, exptDo)
	if err != nil {
		return err
	}
	if err := e.publisher.PublishExptScheduleEvent(ctx, &entity.ExptScheduleEvent{
		SpaceID:     expt.SpaceID,
		ExptID:      expt.ID,
		ExptRunID:   exptRunID,
		ExptRunMode: entity.EvaluationModeAppend,
		ExptType:    expt.ExptType,
		CreatedAt:   time.Now().Unix(),
		Session:     session,
	}, gptr.Of(time.Second*3)); err != nil {
		return err
	}
	if err := e.idem.Set(ctx, idemKeyPrefix+strconv.FormatInt(expt.ID, 10), time.Second*60); err != nil {
		logs.CtxWarn(ctx, "FinishExpt SetNX fail, err: %v", err)
	}
	return nil
}

func (e *ExptMangerImpl) PendRun(ctx context.Context, exptID, exptRunID int64, spaceID int64, session *entity.Session) error {
	runLog, err := e.GetRunLog(ctx, exptID, exptRunID, spaceID, session)
	if err != nil {
		return err
	}

	if err := e.calculateRunLogStats(ctx, exptID, exptRunID, runLog, spaceID, session); err != nil {
		return err
	}
	runLog.Status = int64(entity.ExptStatus_Pending)

	logs.CtxInfo(ctx, "[ExptEval] PendRun, expt_id: %v, expt_run_id: %v, status: %v", exptID, exptRunID, runLog.Status)

	if err := e.runLogRepo.Save(ctx, runLog); err != nil {
		return err
	}

	return nil
}

func (e *ExptMangerImpl) PendExpt(ctx context.Context, exptID, spaceID int64, session *entity.Session, opts ...entity.CompleteExptOptionFn) error {
	stats, err := e.exptResultService.CalculateStats(ctx, exptID, spaceID, session)
	if err != nil {
		return err
	}

	exptStats := &entity.ExptStats{
		SuccessItemCnt:    int32(stats.SuccessItemCnt),
		PendingItemCnt:    int32(stats.PendingItemCnt),
		FailItemCnt:       int32(stats.FailItemCnt),
		ProcessingItemCnt: int32(stats.ProcessingItemCnt),
		TerminatedItemCnt: int32(stats.TerminatedItemCnt),
	}

	if err := e.statsRepo.UpdateByExptID(ctx, exptID, spaceID, exptStats); err != nil {
		return err
	}

	return nil
}

func (e *ExptMangerImpl) LogRun(ctx context.Context, exptID, exptRunID int64, mode entity.ExptRunMode, spaceID int64, session *entity.Session) error {
	duration := time.Duration(e.configer.GetExptExecConf(ctx, spaceID).GetZombieIntervalSecond()) * time.Second
	locked, err := e.mutex.LockBackoff(ctx, e.makeExptMutexLockKey(exptID), duration, time.Second)
	if err != nil {
		return err
	}
	if !locked {
		return errorx.NewByCode(errno.ExperimentRunningExistedCode)
	}

	defer e.mtr.EmitExptExecRun(spaceID, int64(mode))

	return e.runLogRepo.Create(ctx, &entity.ExptRunLog{
		ID:        exptRunID,
		SpaceID:   spaceID,
		CreatedBy: session.UserID,
		ExptID:    exptID,
		ExptRunID: exptRunID,
		Mode:      int32(mode),
		Status:    int64(entity.ExptStatus_Pending),
	})
}

func (e *ExptMangerImpl) GetRunLog(ctx context.Context, exptID, exptRunID, spaceID int64, session *entity.Session) (*entity.ExptRunLog, error) {
	return e.runLogRepo.Get(ctx, exptID, exptRunID)
}
