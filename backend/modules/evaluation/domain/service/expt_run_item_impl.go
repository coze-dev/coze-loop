// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
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
	evalAsyncRepo repo.IEvalAsyncRepo,
	evalSetItemSvc EvaluationSetItemService,
	itemCompletePublisher component.IItemCompletePublisher,
	sandboxAgentMetrics metrics.SandboxAgentMetrics, // 沙箱 agent 端到端 (turn 粒度) 打点; 可空
	sandboxAgentNotifier ...ISandboxAgentNotifier, // variadic 保持已有单测编译通过
) ExptItemEvaluation {
	exec := &ExptItemEvalCtxExecutor{
		TurnResultRepo:         turnResultRepo,
		ItemResultRepo:         itemResultRepo,
		Configer:               configer,
		Metric:                 metric,
		evalTargetService:      evalTargetService,
		evaluatorRecordService: evaluatorRecordService,
		evaluatorService:       evaluatorService,
		benefitService:         benefitService,
		evalAsyncRepo:          evalAsyncRepo,
		evalSetItemSvc:         evalSetItemSvc,
		itemCompletePublisher:  itemCompletePublisher,
		sandboxAgentMetrics:    sandboxAgentMetrics,
	}
	if len(sandboxAgentNotifier) > 0 {
		exec.sandboxAgentNotifier = sandboxAgentNotifier[0]
	}
	return exec
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
	evalAsyncRepo          repo.IEvalAsyncRepo
	evalSetItemSvc         EvaluationSetItemService
	itemCompletePublisher  component.IItemCompletePublisher
	sandboxAgentNotifier   ISandboxAgentNotifier       // 沙箱 agent 实验单行失败飞书通知; 可空
	sandboxAgentMetrics    metrics.SandboxAgentMetrics // 沙箱 agent 端到端 turn 打点; 可空 (nil 时不打点)
}

const exptRunLogPersistTimeout = 5 * time.Second

func (e *ExptItemEvalCtxExecutor) Eval(ctx context.Context, eiec *entity.ExptItemEvalCtx) error {
	// if err := e.SetItemRunProcessing(ctx, event.ExptID, event.ExptRunID, event.EvalSetItemID, event.SpaceID, event.Session); err != nil {
	//	return err
	// }

	asyncAbort, evalErr := e.EvalTurns(ctx, eiec)
	if asyncAbort {
		return nil
	}

	if err := e.CompleteItemRun(ctx, eiec, evalErr); err != nil {
		return err
	}

	return nil
}

func (e *ExptItemEvalCtxExecutor) EvalTurns(ctx context.Context, eiec *entity.ExptItemEvalCtx) (asyncAbort bool, err error) {
	var history []*entity.Message

	if eiec.EvalSetItem == nil {
		return false, fmt.Errorf("EvalTurns with invalid empty eval_set_item")
	}

	for _, turn := range eiec.EvalSetItem.Turns {
		// 行级终止的「不再继续」语义：每轮开始前查一次该行是否已被终止，是则 break，
		// 当前已跑完的轮次结果保留、后续轮次不再开始（对应 spec「执行中的行在当轮结束后停止」）。
		// 放在循环体首部（而非尾部）是为了同时覆盖「排队中被终止、事件已在途」的情形。
		if e.isItemTerminated(ctx, eiec) {
			logs.CtxInfo(ctx, "[ExptItemTerminate] stop eval turns on terminated item, expt_id: %v, expt_run_id: %v, item_id: %v, turn_id: %v",
				eiec.Event.ExptID, eiec.Event.ExptRunID, eiec.Event.EvalSetItemID, turn.ID)
			break
		}

		etec, err := e.buildExptTurnEvalCtx(ctx, turn, eiec, history)
		if err != nil {
			return false, err
		}

		ctx = context.WithValue(ctx, consts.CtxKeyLogID, etec.GetTurnEvalLogID(ctx, turn.ID)) //nolint:staticcheck

		// [sandbox_agent_metrics] 端到端 started: 只在该 turn 首次入调度且非 async 回调重进时打点。
		// 后续重试事件 (RetryTimes>0) / async 回调 (AsyncReport(Evaluator)Trigger) 不再重复计数，
		// 保证「一 turn 一次 e2e_started」语义。
		e.emitSandboxAgentE2EStarted(ctx, etec)

		turnRunRes := NewExptTurnEvaluation(e.Metric, e.evalTargetService, e.evaluatorService, e.benefitService, e.evalAsyncRepo, e.evalSetItemSvc, e.evaluatorRecordService).Eval(ctx, etec)

		if err := e.storeTurnRunResult(ctx, etec, turnRunRes); err != nil {
			return false, err
		}

		if turnRunRes.AsyncAbort {
			logs.CtxInfo(ctx, "[ExptTurnEval] eval async abort, expt_id: %v, item_id: %v, turn_id: %v", eiec.Event.ExptID, eiec.Event.EvalSetItemID, turn.ID)
			return true, nil
		}

		// [sandbox_agent_metrics] 端到端 finished: 该 turn 走到终态才 emit。
		// 终态判定: evalErr==nil (成功), 或 evalErr 存在但 evalErrNeedRetry 判为不再重试 (最后一次失败)。
		// 中间失败重试轮次不 emit, 由下次事件重进时的终态轮次统一 emit; duration 起点 = event.CreateAt,
		// 横跨所有重试。
		if turnErr := turnRunRes.GetEvalErr(); turnErr != nil {
			e.emitSandboxAgentE2EFinishedIfTerminal(ctx, etec, eiec.Event, turnRunRes, turnErr)
			return false, turnErr
		}
		e.emitSandboxAgentE2EFinishedIfTerminal(ctx, etec, eiec.Event, turnRunRes, nil)

		history = append(history, buildHistoryMessage(ctx, turnRunRes)...)
	}

	time.Sleep(time.Second * 1)

	return false, nil
}

// isItemTerminated 查该 item 当前 run log 是否已被行级终止置为 Terminal。
// 查询失败返回 false（不拦截）—— 宁可多跑一轮，也不能因一次查询抖动把正常行掐掉；
// 真被终止的行还有 CompleteItemRun 的 Terminal 覆盖保护兜底。
func (e *ExptItemEvalCtxExecutor) isItemTerminated(ctx context.Context, eiec *entity.ExptItemEvalCtx) bool {
	if eiec == nil || eiec.Event == nil || e.ItemResultRepo == nil {
		return false
	}
	event := eiec.Event
	itemRunLog, err := e.ItemResultRepo.GetItemRunLog(ctx, event.ExptID, event.ExptRunID, event.EvalSetItemID, event.SpaceID)
	if err != nil {
		logs.CtxWarn(ctx, "[ExptItemTerminate] check item terminated fail, expt_id: %v, expt_run_id: %v, item_id: %v, err: %v",
			event.ExptID, event.ExptRunID, event.EvalSetItemID, err)
		return false
	}
	return itemRunLog != nil && entity.ItemRunState(itemRunLog.Status) == entity.ItemRunState_Terminal
}

func (e *ExptItemEvalCtxExecutor) storeTurnRunResult(ctx context.Context, etec *entity.ExptTurnEvalCtx, result *entity.ExptTurnRunResult) error {
	if result == nil {
		return fmt.Errorf("StoreTurnRunResult with nil result")
	}
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), exptRunLogPersistTimeout)
	defer cancel()

	turn := etec.Turn
	turnResultLog := etec.GetExistTurnResultLogs()[turn.ID]

	if turnResultLog == nil {
		return fmt.Errorf("storeTurnRunResult with invalid turn result log, expt_id: %v, item_id: %v, turn_id: %v", etec.Expt.ID, etec.EvalSetItem.ItemID, turn.ID)
	}

	clone := &entity.ExptTurnResultRunLog{}
	if err := copier.Copy(clone, turnResultLog); err != nil {
		return errorx.Wrapf(err, "ExptTurnResultRunLog copy fail")
	}

	clone.Ext = etec.Ext

	var evalErr error

	clone.ExptRunID = etec.Event.ExptRunID
	if result.TargetResult != nil && result.TargetResult.ID > 0 {
		clone.TargetResultID = result.TargetResult.ID
	}

	if result.TargetResult != nil && result.TargetResult.EvalTargetOutputData != nil && result.TargetResult.EvalTargetOutputData.EvalTargetRunError != nil && result.TargetResult.EvalTargetOutputData.EvalTargetRunError.Code > 0 {
		evalErr = errno.NewTargetResultErr(result.TargetResult.EvalTargetOutputData.EvalTargetRunError.Message)
	}

	clone.EvaluatorResultIds = &entity.EvaluatorResults{
		Registered: make([]*entity.RegisteredEvalResult, 0, len(result.EvaluatorResults)),
	}
	for evID, er := range result.EvaluatorResults {
		if er == nil {
			logs.CtxWarn(ctx, "[ExptTurnEval] nil evaluator record, evaluator_version_id: %v", evID)
			continue
		}
		clone.EvaluatorResultIds.Registered = append(clone.EvaluatorResultIds.Registered, &entity.RegisteredEvalResult{
			VersionID: er.EvaluatorVersionID,
			Alias:     er.Alias,
			RecordID:  er.ID,
		})
		if er.EvaluatorOutputData != nil && er.EvaluatorOutputData.EvaluatorRunError != nil && er.EvaluatorOutputData.EvaluatorRunError.Code > 0 {
			evalErr = errno.NewEvaluatorResultErr(er.EvaluatorOutputData.EvaluatorRunError.Message)
		}
	}

	if result.EvalErr != nil {
		evalErr = result.EvalErr
	} else if evalErr == nil {
		evalErr = e.validateEvaluatorResultsComplete(etec, result)
	}

	if evalErr != nil {
		var errMsg string
		switch {
		case isSandboxAgentExpt(etec.Expt):
			// 沙箱 agent 评测对象加白:错误文案直接沿用异步上报方原文,不做 ConvertErrMsg 归一化。
			errMsg = errorx.ErrorWithoutStack(evalErr)
		case func() bool {
			se, ok := errorx.FromStatusError(evalErr)
			return ok && (se.Code() == errno.CustomEvalTargetInvokeFailCode || se.Code() == errno.CustomRPCEvaluatorRunFailedCode)
		}():
			errMsg = errorx.ErrorWithoutStack(evalErr)
		default:
			errMsg = e.Configer.GetErrCtrl(persistCtx).ConvertErrMsg(evalErr.Error())
		}

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
		if !result.AsyncAbort {
			clone.Status = entity.TurnRunState_Success
			clone.ErrMsg = ""
		}
	}

	result.SetEvalErr(evalErr)

	if err := e.TurnResultRepo.SaveTurnRunLogs(persistCtx, []*entity.ExptTurnResultRunLog{clone}); err != nil {
		return err
	}
	resumeCtx, resumeCancel := context.WithTimeout(context.WithoutCancel(ctx), exptRunLogPersistTimeout)
	defer resumeCancel()
	var resumeWG sync.WaitGroup
	for _, record := range result.EvaluatorResults {
		if record == nil || record.ID <= 0 || record.Status != entity.EvaluatorRunStatusAsyncInvoking {
			continue
		}
		recordID := record.ID
		resumeWG.Add(1)
		go func() {
			defer resumeWG.Done()
			if err := e.evaluatorService.ArmEvaluatorResume(resumeCtx, recordID); err != nil {
				// The turn references are already durable and the provider has accepted the work.
				// Failing the item here would be unretriable (AsyncAbort sets CtxForceNoRetry) and
				// could overwrite a valid terminal callback. Keep the item processing while the
				// provider retries the callback; the existing zombie policy remains the final fallback.
				logs.CtxError(ctx, "[ExptTurnEval] arm evaluator async resume failed after refs persisted, keep item processing, record_id: %d, err: %v", recordID, err)
			}
		}()
	}
	resumeWG.Wait()

	logs.CtxInfo(ctx, "[ExptTurnEval] expt turn eval finished, expt_id: %v, expt_run_id: %v, item_id: %v, turn_id: %v, run_log: %v, err: %v",
		etec.Expt.ID, etec.Event.ExptRunID, etec.EvalSetItem.ItemID, turn.ID, json.Jsonify(clone), result.EvalErr)

	return nil
}

func (e *ExptItemEvalCtxExecutor) validateEvaluatorResultsComplete(etec *entity.ExptTurnEvalCtx, result *entity.ExptTurnRunResult) error {
	if etec == nil || etec.Expt == nil || result == nil || result.AsyncAbort {
		return nil
	}
	if etec.Expt.EvalConf == nil || etec.Expt.EvalConf.ConnectorConf.EvaluatorsConf == nil {
		return nil
	}

	// ★ 新实验类型 (MultiSetConfig): 期望集必须取本评测集自己绑定的 ItemConfig.EvaluatorConfs,
	//   绝不能用 expt.Evaluators (那是所有评测集 evaluator 的并集): 否则 set1 的 item 会被要求
	//   也有 set2 evaluator 的结果、反之亦然, 双向 missing -> turn 全 fail。与执行侧 CallEvaluators
	//   / callEvaluatorsByItemConfig 同口径 (按 (versionID, alias) 双键 + Skipped 占位视为已满足)。
	//   本评测集未配 evaluator (ItemConfig nil / EvaluatorConfs 空) -> 期望 0 个, 合法, 直接放行。
	if etec.Expt.EvalSetSourceType == entity.ExptEvalSetSourceType_MultiSetConfig {
		return e.validateEvaluatorResultsCompleteByItemConfig(etec, result)
	}

	// 老实验 (SingleSet, ItemConfig 恒 nil): 期望集为实验级 expt.Evaluators, 按 versionID 单键匹配。
	if len(etec.Expt.Evaluators) == 0 {
		return nil
	}

	missing := make([]string, 0)
	for _, evaluator := range etec.Expt.Evaluators {
		if evaluator == nil {
			continue
		}
		evaluatorVersionID := evaluator.GetEvaluatorVersionID()
		if evaluatorVersionID == 0 {
			continue
		}
		record := result.GetEvaluatorRecord(evaluatorVersionID)
		if record == nil || record.ID == 0 {
			missing = append(missing, strconv.FormatInt(evaluatorVersionID, 10))
		}
	}
	if len(missing) == 0 {
		return nil
	}

	return errno.NewEvaluatorResultErr(fmt.Sprintf("evaluator result missing, evaluator_version_ids: %s", strings.Join(missing, ",")))
}

// validateEvaluatorResultsCompleteByItemConfig 校验 MultiSetConfig 实验的单行评估器结果完整性:
// 期望集 = etec.ItemConfig.EvaluatorConfs (本评测集绑定的 (versionID, alias) 列表)。
//   - ItemConfig nil / EvaluatorConfs 空: 本集没配评估器, 期望 0 个, 合法, 直接放行。
//   - 命中判定按 (versionID, alias) 双键 (对齐 callEvaluatorsByItemConfig 的落库口径)。
//   - Status=Skipped(4) 的占位 record 视为"已满足": filter 不命中的合法跳过, 不能判 missing。
func (e *ExptItemEvalCtxExecutor) validateEvaluatorResultsCompleteByItemConfig(etec *entity.ExptTurnEvalCtx, result *entity.ExptTurnRunResult) error {
	if etec.ItemConfig == nil || len(etec.ItemConfig.EvaluatorConfs) == 0 {
		return nil
	}

	missing := make([]string, 0)
	for _, icConf := range etec.ItemConfig.EvaluatorConfs {
		if icConf == nil {
			continue
		}
		evaluatorVersionID := icConf.EvaluatorVersionID
		if evaluatorVersionID == 0 {
			continue
		}
		// 命中判定按 (versionID, alias) 双键; Skipped 占位 record 也视为已满足 (filter 合法跳过)。
		record := result.GetEvaluatorRecordByVerAlias(evaluatorVersionID, icConf.Alias)
		if record == nil || record.ID == 0 {
			missing = append(missing, formatEvaluatorVerAlias(evaluatorVersionID, icConf.Alias))
		}
	}
	if len(missing) == 0 {
		return nil
	}

	return errno.NewEvaluatorResultErr(fmt.Sprintf("evaluator result missing, evaluator_version_ids: %s", strings.Join(missing, ",")))
}

// formatEvaluatorVerAlias 拼 (versionID, alias) 便于 missing 诊断; alias 为空退化为纯 versionID。
func formatEvaluatorVerAlias(versionID int64, alias string) string {
	if alias == "" {
		return strconv.FormatInt(versionID, 10)
	}
	return strconv.FormatInt(versionID, 10) + ":" + alias
}

func (e *ExptItemEvalCtxExecutor) SetItemRunProcessing(ctx context.Context, exptID, exptRunID, itemID, spaceID int64, session *entity.Session) error {
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
	for k, v := range eiec.Event.Ext {
		etec.Ext[k] = v
	}
	// 从 ExptItemResult 中获取 Ext 字段并合并到 etec.Ext
	itemResults, err := e.ItemResultRepo.BatchGet(ctx, spaceID, eiec.Event.ExptID, []int64{eiec.Event.EvalSetItemID})
	if err == nil && len(itemResults) > 0 && itemResults[0].Ext != nil {
		for k, v := range itemResults[0].Ext {
			etec.Ext[k] = v
		}
	}
	for _, fieldData := range eiec.EvalSetItem.Turns[0].FieldDataList {
		if fieldData.Name == "span_id" {
			etec.Ext["span_id"] = fieldData.Content.GetText()
		}
		if fieldData.Name == "run_id" {
			etec.Ext["run_id"] = fieldData.Content.GetText()
		}
		if fieldData.Name == "trace_id" {
			etec.Ext["trace_id"] = fieldData.Content.GetText()
		}
	}
	etec.Ext["task_id"] = eiec.Expt.SourceID
	etec.Ext["workspace_id"] = strconv.FormatInt(eiec.Expt.SpaceID, 10)
	etec.Ext["start_time"] = strconv.FormatInt(gptr.Indirect(eiec.EvalSetItem.BaseInfo.CreatedAt)*1000, 10) // 存储是毫秒，需要存入微妙

	// 统一在实验执行流程中构造 ext，合并进 etec.Ext，后续随 EvalTargetRecord/EvaluatorRecord/ExptTurnResultRunLog DO 落库
	evalExt := e.Configer.BuildEvalExt(ctx, spaceID, turn)
	for k, v := range evalExt {
		etec.Ext[k] = v
	}
	logs.CtxInfo(ctx, "[BuildEvalExt] buildExptTurnEvalCtx merged ext, expt_id: %v, item_id: %v, turn_id: %v, space_id: %v, build_eval_ext: %v, merged_etec_ext: %v",
		eiec.Event.ExptID, eiec.Event.EvalSetItemID, turn.ID, spaceID, json.Jsonify(evalExt), json.Jsonify(etec.Ext))

	if existTurnRunResult == nil {
		return etec, nil
	}

	if tid := existTurnRunResult.TargetResultID; tid > 0 {
		// ★ 跨空间共享: 评测对象执行记录随执行落来源空间(冻结 TargetSpaceID), 按来源空间读;
		// 用调用方空间读会得 nil → 异步回调 validateEvalTargetCtx 报 "target result must not be nil"。
		targetRecord, err := e.evalTargetService.GetRecordByID(ctx, resolveLoadSpaceID(spaceID, eiec.TargetSourceSpaceID()), tid)
		if err != nil {
			return nil, err
		}
		etec.ExptTurnRunResult.TargetResult = targetRecord
	}

	if erids := existTurnRunResult.EvaluatorResultIds; erids != nil {
		// 新格式 (Registered/Inline) 与老格式 (EvalVerIDToResID) 都要覆盖: 存储侧 storeTurnRunResult
		// 只写 Registered, 若此处仅读老 map, 异步上报重触发会读到空 evaluator 结果, 校验时误判为"评估器结果缺失"。
		recordIDs := make([]int64, 0)
		seen := make(map[int64]struct{})
		addID := func(id int64) {
			if id <= 0 {
				return
			}
			if _, ok := seen[id]; ok {
				return
			}
			seen[id] = struct{}{}
			recordIDs = append(recordIDs, id)
		}
		for _, r := range erids.Registered {
			if r != nil {
				addID(r.RecordID)
			}
		}
		for _, r := range erids.Inline {
			if r != nil {
				addID(r.RecordID)
			}
		}
		for _, id := range erids.EvalVerIDToResID {
			addID(id)
		}

		if len(recordIDs) > 0 {
			evaluatorRecords, err := e.evaluatorRecordService.BatchGetEvaluatorRecord(ctx, recordIDs, false, false)
			if err != nil {
				return nil, err
			}
			etec.ExptTurnRunResult.EvaluatorResults = evaluatorRecords
		}
	}

	return etec, nil
}

func (e *ExptItemEvalCtxExecutor) CompleteItemRun(ctx context.Context, eiec *entity.ExptItemEvalCtx, evalErr error) error {
	event := eiec.Event
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), exptRunLogPersistTimeout)
	defer cancel()

	if evalErr != nil {
		if retry, _ := e.evalErrNeedRetry(persistCtx, event, evalErr); retry {
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

	// Terminal 是吸收态：该行已被用户行级终止时，在途执行的成功/失败结果 MUST NOT 回写 status
	// （对应 spec「终止后到达的执行结果不覆盖终止状态」）。只写 result_state 让调度侧照常收口；
	// err_msg 也不覆盖 —— 用户要看到的是"被主动终止"，而不是终止前那次执行的报错。
	if e.isItemTerminated(persistCtx, eiec) {
		logs.CtxInfo(persistCtx, "[ExptItemTerminate] keep terminal status on complete item run, expt_id: %v, expt_run_id: %v, item_id: %v, dropped_fields: %v",
			event.ExptID, event.ExptRunID, event.EvalSetItemID, ufields)
		ufields = map[string]any{
			"result_state": entity.ExptItemResultStateLogged,
		}
	}

	if err := e.ItemResultRepo.UpdateItemRunLog(persistCtx, event.ExptID, event.ExptRunID, []int64{event.EvalSetItemID}, ufields, event.SpaceID); err != nil {
		return err
	}

	// 沙箱 agent 实验:单行走到终态失败,立即发一张飞书卡。Notifier 内部做 sandbox agent + Enable 判定,
	// 非目标 case 静默返回。发送失败仅 log,不阻塞主流程。
	if evalErr != nil && e.sandboxAgentNotifier != nil {
		if nerr := e.sandboxAgentNotifier.NotifyItemFail(persistCtx, eiec.Expt, event.EvalSetItemID, evalErr); nerr != nil {
			logs.CtxWarn(persistCtx, "[SandboxAgentNotify] item fail notify err, expt_id=%v, item_id=%v, err=%v", event.ExptID, event.EvalSetItemID, nerr)
		}
	}

	// item-complete(success) MQ 发送点已后移到链路B(scheduler daemon 的 recordEvalItemRunLogs),
	// 在 RecordItemRunLogs 写完读侧三张表后才发,消除"下游收到 success 反查读侧却未就绪"的竞态。
	// 此处仅写 result_state=Logged,不再发 MQ。

	if e.evalErrNeedTerminateExpt(persistCtx, event.SpaceID, evalErr) {
		logs.CtxWarn(ctx, "[ExptTurnEval] found error which should terminate expt, expt_id: %v, expt_run_id: %v, item_id: %v, err: %v", event.ExptID, event.ExptRunID, event.EvalSetItemID, evalErr)
		return evalErr
	}

	logs.CtxInfo(ctx, "[ExptTurnEval] expt item eval finished, expt_id: %v, expt_run_id: %v, success: %v, update_fields: %v", event.ExptID, event.ExptRunID, evalErr == nil, ufields)
	time.Sleep(time.Second * 2) // 确保日志落库
	return nil
}

// buildItemCompleteEvent 从单行评测上下文组装 item-complete 事件，全部取内存已有数据、零额外 IO。
// dataset 维度以 item 实际归属为准：dataset_id 从 EvalSetItem 取，dataset_version_id 用 per-item
// EvalSetVersionID（来自 expt_item_ref）；version_name / dataset_key 由 findEvalSetForItem 按
// experiment.eval_set_source_type 显式分流取值——多集从 EvalSetDetails 按归属集匹配（GetDetail 已批量
// 拉全所有集详情），单集/老实验直接用主集 eiec.Expt.EvalSet，均零额外 IO。
func buildItemCompleteEvent(eiec *entity.ExptItemEvalCtx) *component.ItemCompleteEvent {
	event := eiec.Event
	ev := &component.ItemCompleteEvent{
		ExptWorkspaceID: strconv.FormatInt(event.SpaceID, 10),
		ExptID:          strconv.FormatInt(event.ExptID, 10),
		ExptRunID:       strconv.FormatInt(event.ExptRunID, 10),
		ItemID:          strconv.FormatInt(event.EvalSetItemID, 10),
	}

	if expt := eiec.Expt; expt != nil {
		ev.EvalTargetID = strconv.FormatInt(expt.TargetID, 10)
		if expt.Target != nil {
			ev.EvalTargetWorkspaceID = strconv.FormatInt(expt.Target.SpaceID, 10)
			// source_target_id: 业务侧原始对象 ID，加载详情时经 EvalTargetPO2DO 已回填，直接透传。
			ev.SourceTargetID = expt.Target.SourceTargetID
			// enable_analysis: 评测对象是否开启分析，从 eval_target 版本对应子结构的固化字段取。
			// 各类型创建评测对象时从 application.usages（含 "analysis"）反查固化到自身子结构。
			// 覆盖走 application.usages 的全部类型；Prompt / CozeBot / CozeWorkflow 不经 usages、不支持分析。
			if ver := expt.Target.EvalTargetVersion; ver != nil {
				switch {
				case ver.SandboxAgent != nil:
					ev.EnableAnalysis = ver.SandboxAgent.EnableAnalysis
				case ver.CustomRPCServer != nil:
					ev.EnableAnalysis = ver.CustomRPCServer.EnableAnalysis
				case ver.CustomAgent != nil:
					ev.EnableAnalysis = ver.CustomAgent.EnableAnalysis
				case ver.VolcengineAgent != nil:
					ev.EnableAnalysis = ver.VolcengineAgent.EnableAnalysis
				case ver.WebAgent != nil:
					ev.EnableAnalysis = ver.WebAgent.EnableAnalysis
				case ver.A2AAgent != nil:
					ev.EnableAnalysis = ver.A2AAgent.EnableAnalysis
				}
			}
		}
		// experiment_group_key: 关联同组实验，默认为实验 ID。
		// PO→DO 转换已保证非空（空则填实验 ID），此处直接透传。
		ev.ExperimentGroupKey = expt.ExperimentGroupKey
		// created_by: 实验创建人 userID，实验级恒定，内存已加载零额外 IO。
		ev.CreatedBy = expt.CreatedBy
	}

	var datasetID int64
	if item := eiec.EvalSetItem; item != nil {
		datasetID = item.EvaluationSetID
		ev.DatasetWorkspaceID = strconv.FormatInt(item.SpaceID, 10)
		ev.DatasetID = strconv.FormatInt(datasetID, 10)
		// item_key 直接透传评测集 item 的实体 ItemKey（由下游 data 服务写入），
		// 空则保持空、不降级到 item_id，交由数据侧处理。
		ev.ItemKey = item.ItemKey
	}

	// dataset_version_id 用 per-item 归属集版本（多评测集非主集也正确，来自 expt_item_ref）。
	if eiec.EvalSetVersionID > 0 {
		ev.DatasetVersionID = strconv.FormatInt(eiec.EvalSetVersionID, 10)
	}

	// version_name / dataset_key: 按 item 归属集从内存查找（GetDetail 已批量拉全所有集详情）。
	if es := findEvalSetForItem(eiec.Expt, datasetID); es != nil {
		ev.DatasetKey = es.DatasetKey
		if ver := es.EvaluationSetVersion; ver != nil {
			if ev.DatasetVersionID == "" {
				ev.DatasetVersionID = strconv.FormatInt(ver.ID, 10)
			}
			ev.DatasetVersionName = ver.Version
		}
	}

	return ev
}

// buildItemCompleteEventFromScheduler 供链路B(scheduler daemon)组装 item-complete 事件。
// 链路B 循环里只有 event(*ExptScheduleEvent) + item(*ExptEvalItem, 仅 ItemID 可信) + expt(全量详情),
// 缺 ItemKey/归属集/per-item 版本; 调用方须在循环外用 expt_item_ref + BatchGetEvaluationSetItems 批量补出:
//   - evalSetItem: 提供 ItemKey / SpaceID / EvaluationSetID(归属集);
//   - evalSetVersionID: 该 item 归属集的 per-item 版本(来自 expt_item_ref, 多集非主集也正确)。
//     切勿用 ExptEvalItem.EvalSetVersionID —— 那是 scanIncompleteAndComplete 硬编码的主集版本(张冠李戴)。
//
// 组装逻辑复用 buildItemCompleteEvent(构造最小 ExptItemEvalCtx), 与链路A 逐字段等价、单一实现不漂移。
func buildItemCompleteEventFromScheduler(spaceID, exptID, exptRunID int64, expt *entity.Experiment, item *entity.ExptEvalItem, evalSetItem *entity.EvaluationSetItem, evalSetVersionID int64) *component.ItemCompleteEvent {
	eiec := &entity.ExptItemEvalCtx{
		Event: &entity.ExptItemEvalEvent{
			SpaceID:       spaceID,
			ExptID:        exptID,
			ExptRunID:     exptRunID,
			EvalSetItemID: item.ItemID,
		},
		Expt:             expt,
		EvalSetItem:      evalSetItem,
		EvalSetVersionID: evalSetVersionID,
	}
	return buildItemCompleteEvent(eiec)
}

// 按 experiment.eval_set_source_type 显式分流（权威分流开关，DB not null default 1）：
//   - MultiSetConfig(2) 新实验: 从 EvalSetDetails 按 datasetID 匹配归属集（GetDetail 已批量填充所有集详情）；
//     匹配不到即返回 nil，不回退主集，避免把主集版本误安到非主集 item 上（张冠李戴）。
//   - SingleSet(1) 老实验/单评测集: 直接用主集单数字段 eiec.Expt.EvalSet。
func findEvalSetForItem(expt *entity.Experiment, datasetID int64) *entity.EvaluationSet {
	if expt == nil {
		return nil
	}
	if expt.EvalSetSourceType == entity.ExptEvalSetSourceType_MultiSetConfig {
		// 多评测集: 只认 EvalSetDetails 里按归属 eval_set_id 命中的集，不回退主集。
		for _, d := range expt.EvalSetDetails {
			if d != nil && d.EvalSetID == datasetID && d.EvalSet != nil {
				return d.EvalSet
			}
		}
		return nil
	}
	// 单评测集/老实验（SingleSet 或未标记）: 用主集 EvalSet。
	if expt.EvalSet != nil && (datasetID == 0 || expt.EvalSet.ID == datasetID) {
		return expt.EvalSet
	}
	return nil
}

func (e *ExptItemEvalCtxExecutor) evalErrNeedRetry(ctx context.Context, event *entity.ExptItemEvalEvent, evalErr error) (bool, time.Duration) {
	if evalErr == nil {
		return false, 0
	}
	spaceID := event.SpaceID
	retryTimes := event.RetryTimes
	conf := e.Configer.GetErrRetryConf(ctx, spaceID, evalErr)
	maxRetryTimes := conf.GetRetryTimes()
	if event.MaxRetryTimes > 0 {
		maxRetryTimes = event.MaxRetryTimes
	}
	return retryTimes < maxRetryTimes, conf.GetRetryInterval()
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

// buildSandboxAgentE2ETags 从 turn 上下文拼装端到端打点 tag; 与 pick* 系列语义一致, 缺失字段占位交由 emit 层。
func buildSandboxAgentE2ETags(etec *entity.ExptTurnEvalCtx) metrics.SandboxAgentE2ETags {
	if etec == nil || etec.Event == nil {
		return metrics.SandboxAgentE2ETags{}
	}
	var turnID int64
	if etec.Turn != nil {
		turnID = etec.Turn.ID
	}
	var itemID int64
	if etec.EvalSetItem != nil {
		itemID = etec.EvalSetItem.ItemID
	}
	return metrics.SandboxAgentE2ETags{
		SpaceID:         etec.Event.SpaceID,
		ExperimentID:    etec.Event.ExptID,
		ExperimentRunID: etec.Event.ExptRunID,
		ItemID:          itemID,
		TurnID:          turnID,
		DatasetID:       pickDatasetID(etec),
		DatasetVersion:  etec.EvalSetVersionID,
		TargetID:        pickTargetID(etec),
		ItemKey:         pickItemKey(etec),
		DatasetKey:      pickDatasetKey(etec),
		AgentName:       pickAgentName(etec),
		ApplicationID:   pickApplicationID(etec),
	}
}

// emitSandboxAgentE2EStarted 只在满足「首次入调度 && 非 async 回调重进 && 沙箱 agent 实验」时打点 e2e_started。
// 后续重试事件 / async 回调重进都不再计数, 保证一 turn 一次 started 语义。sandboxAgentMetrics 为空时静默返回。
func (e *ExptItemEvalCtxExecutor) emitSandboxAgentE2EStarted(ctx context.Context, etec *entity.ExptTurnEvalCtx) {
	if e == nil || e.sandboxAgentMetrics == nil {
		return
	}
	if etec == nil || etec.Event == nil || etec.Expt == nil {
		return
	}
	if !isSandboxAgentExpt(etec.Expt) {
		return
	}
	if etec.Event.RetryTimes != 0 || etec.Event.AsyncReportTrigger || etec.Event.AsyncEvaluatorReportTrigger {
		return
	}
	tags := buildSandboxAgentE2ETags(etec)
	logs.CtxInfo(ctx, "[sandbox_agent_metrics] emit e2e_started, expt_id=%v, expt_run_id=%v, item_id=%v, turn_id=%v",
		tags.ExperimentID, tags.ExperimentRunID, tags.ItemID, tags.TurnID)
	e.sandboxAgentMetrics.EmitE2EStarted(tags)
}

// emitSandboxAgentE2EFinishedIfTerminal 只在该 turn 达到终态时打点 e2e_finished + e2e_duration。
// 终态 = evalErr==nil (成功) 或 evalErrNeedRetry 判为不再重试 (最后一次失败)。中间失败重试轮次不打点。
// duration 起点 = event.CreateAt (首次入库时间, 跨所有重试保持不变), 因此终态一次 duration 涵盖整条端到端链路。
func (e *ExptItemEvalCtxExecutor) emitSandboxAgentE2EFinishedIfTerminal(ctx context.Context, etec *entity.ExptTurnEvalCtx, event *entity.ExptItemEvalEvent, turnRunRes *entity.ExptTurnRunResult, evalErr error) {
	if e == nil || e.sandboxAgentMetrics == nil {
		return
	}
	if etec == nil || etec.Expt == nil || event == nil {
		return
	}
	if !isSandboxAgentExpt(etec.Expt) {
		return
	}
	if evalErr != nil {
		if retry, _ := e.evalErrNeedRetry(ctx, event, evalErr); retry {
			return
		}
	}
	tags := buildSandboxAgentE2ETags(etec)
	// event.CreateAt 单位是 Unix 秒 (由 expt_run_scheduler_event_impl.go 用 time.Now().Unix() 赋值),
	// 用 time.Unix(sec, 0) 反序列化; 用 UnixMilli 会把秒当毫秒 → duration 变成 1970 至今的差值 (~1.78e12ms).
	var startTime time.Time
	if event.CreateAt > 0 {
		startTime = time.Unix(event.CreateAt, 0)
	}
	// 抽 error_code tag:
	//   1) 优先从 evalErr 走 FromStatusError (CallTarget / CallEvaluators 直接返回 StatusError 的路径).
	//   2) 否则回落到 turnRunRes.TargetResult.EvalTargetRunError.Code —— storeTurnRunResult 里
	//      target 失败会被包成 errno.NewTargetResultErr (ErrImpl code=11) 丢掉真实码, 只有原始 record 里还留着.
	//   3) 再否则查 turnRunRes.EvaluatorResults[*].EvaluatorRunError.Code (评估器失败同理丢码).
	//   均无 → 走 0 (占位符 `-`).
	errCode := extractSandboxAgentErrCode(evalErr, turnRunRes)
	logs.CtxInfo(ctx, "[sandbox_agent_metrics] emit e2e_finished, expt_id=%v, expt_run_id=%v, item_id=%v, turn_id=%v, success=%v, err_code=%v",
		tags.ExperimentID, tags.ExperimentRunID, tags.ItemID, tags.TurnID, evalErr == nil, errCode)
	e.sandboxAgentMetrics.EmitE2EFinished(tags, evalErr, errCode, startTime)
}

// extractSandboxAgentErrCode 从 evalErr / turnRunRes 中抽出用于打点的错误码.
// 见 emitSandboxAgentE2EFinishedIfTerminal 中的三段回落说明.
func extractSandboxAgentErrCode(evalErr error, turnRunRes *entity.ExptTurnRunResult) int32 {
	if evalErr != nil {
		if se, ok := errorx.FromStatusError(evalErr); ok {
			return se.Code()
		}
	}
	if turnRunRes == nil {
		return 0
	}
	if tr := turnRunRes.TargetResult; tr != nil && tr.EvalTargetOutputData != nil && tr.EvalTargetOutputData.EvalTargetRunError != nil {
		if code := tr.EvalTargetOutputData.EvalTargetRunError.Code; code != 0 {
			return code
		}
	}
	for _, er := range turnRunRes.EvaluatorResults {
		if er == nil || er.EvaluatorOutputData == nil || er.EvaluatorOutputData.EvaluatorRunError == nil {
			continue
		}
		if code := er.EvaluatorOutputData.EvaluatorRunError.Code; code != 0 {
			return code
		}
	}
	return 0
}
