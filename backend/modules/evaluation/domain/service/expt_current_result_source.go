// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

var errExptNonCurrentSource = errors.New("execution source is no longer current")

type currentItemSourceKey struct{}
type currentItemExecution struct {
	Source          *entity.ExptCurrentItemRunSource
	LockLost        bool
	Completed       bool
	QuotaReleased   bool
	SourceValidated bool
	LogCtx          context.Context
}

func executionSource(ctx context.Context) *currentItemExecution {
	source, _ := ctx.Value(currentItemSourceKey{}).(*currentItemExecution)
	return source
}

func (e *ExptItemEventEvalServiceImpl) validateResumeSource(ctx context.Context, event *entity.ExptItemEvalEvent, current *entity.ExptItemCurrentRequest) (bool, error) {
	source := event.ResumeSource
	if source == nil {
		if event.AsyncReportTrigger || event.AsyncEvaluatorReportTrigger {
			// A legacy event has no invoke ID. Only an original AsyncCtx keyed by an
			// explicit identity can recover it; the latest turn record is not identity.
			return false, errno.NewMQRetryErr("legacy async event lacks original recoverable invocation identity")
		}
		return true, nil
	}
	if source.RecordID <= 0 || source.TurnID < 0 || source.InvokeID != strconv.FormatInt(source.RecordID, 10) || source.Kind != "target" && source.Kind != "evaluator" {
		return false, errno.NewMQRetryErr("invalid original resume source")
	}
	if e.exptTurnResultRepo == nil {
		return false, errno.NewMQRetryErr("turn source reader missing")
	}
	var logs []*entity.ExptTurnResultRunLog
	if execution := executionSource(ctx); execution != nil && execution.Source != nil {
		for _, log := range execution.Source.Turns {
			logs = append(logs, log)
		}
	} else {
		var err error
		logs, err = e.exptTurnResultRepo.GetItemTurnRunLogs(contexts.WithCtxWriteDB(ctx), event.ExptID, event.ExptRunID, event.EvalSetItemID, event.SpaceID)
		if err != nil {
			return false, errno.WrapMQRetryErr(err)
		}
	}
	var turn *entity.ExptTurnResultRunLog
	for _, log := range logs {
		if log == nil {
			return false, errno.NewMQRetryErr("incomplete current turn source")
		}
		if log.TurnID == source.TurnID {
			if turn != nil {
				return false, errno.NewMQRetryErr("duplicate current source turn")
			}
			turn = log
		}
	}
	if turn == nil {
		return false, errno.NewMQRetryErr("original source turn missing")
	}
	if turn.SpaceID != event.SpaceID || turn.ExptID != event.ExptID || turn.ExptRunID != event.ExptRunID || turn.ItemID != event.EvalSetItemID || turn.ItemVersionID != current.ItemVersionID {
		return false, errno.NewMQRetryErr("foreign or incomplete source turn")
	}
	if turn.Status < entity.TurnRunState_Queueing || turn.Status > entity.TurnRunState_Terminal {
		return false, errno.NewMQRetryErr("invalid source turn state")
	}
	if source.Kind == "target" {
		if turn.TargetResultID != source.RecordID {
			return false, nil
		}
		if e.evaTargetService == nil {
			return false, errno.NewMQRetryErr("target source reader missing")
		}
		targetSpace := event.SpaceID
		if e.manager != nil {
			detail, err := e.manager.GetDetail(contexts.WithCtxWriteDB(ctx), event.ExptID, event.SpaceID, event.Session)
			if err != nil {
				return false, errno.WrapMQRetryErr(err)
			}
			if detail == nil {
				return false, errno.NewMQRetryErr("original target resource source missing")
			}
			targetSpace = resolveLoadSpaceID(event.SpaceID, detail.TargetSpaceID)
		}
		record, err := e.evaTargetService.GetRecordByID(contexts.WithCtxWriteDB(ctx), targetSpace, source.RecordID)
		if err != nil {
			return false, errno.WrapMQRetryErr(err)
		}
		if record == nil {
			return false, errno.NewMQRetryErr("original target record missing")
		}
		if record.ID <= 0 || record.SpaceID <= 0 || record.ExperimentRunID <= 0 || record.ItemID <= 0 || record.TurnID < 0 || record.Status == nil || *record.Status < entity.EvalTargetRunStatusSuccess || *record.Status > entity.EvalTargetRunStatusAsyncInvoking {
			return false, errno.NewMQRetryErr("incomplete original target record")
		}
		return record.ID == source.RecordID && record.ExperimentRunID == event.ExptRunID && record.ItemID == event.EvalSetItemID && (record.ItemVersionID == 0 || record.ItemVersionID == current.ItemVersionID) && record.TurnID == source.TurnID, nil
	}
	if e.evaluatorRecordService == nil {
		return false, errno.NewMQRetryErr("evaluator source reader missing")
	}
	record, err := e.evaluatorRecordService.GetEvaluatorRecord(contexts.WithCtxWriteDB(ctx), source.RecordID, false)
	if err != nil {
		return false, errno.WrapMQRetryErr(err)
	}
	if record == nil {
		return false, errno.NewMQRetryErr("original evaluator record missing")
	}
	if record.ID <= 0 || record.SpaceID <= 0 || record.ExperimentID <= 0 || record.ExperimentRunID <= 0 || record.ItemID <= 0 || record.TurnID < 0 || record.Status < entity.EvaluatorRunStatusSuccess || record.Status > entity.EvaluatorRunStatusSkipped || record.SourceType < 0 || record.SourceType > entity.EvaluatorRecordSourceTypeInline {
		return false, errno.NewMQRetryErr("incomplete original evaluator record")
	}
	if record.ID != source.RecordID || record.ExperimentID != event.ExptID || record.ExperimentRunID != event.ExptRunID || record.ItemID != event.EvalSetItemID || record.ItemVersionID != 0 && record.ItemVersionID != current.ItemVersionID || record.TurnID != source.TurnID {
		return false, nil
	}
	if record.TargetRecordID != 0 && record.TargetRecordID != turn.TargetResultID {
		return false, nil
	}
	refs := turn.EvaluatorResultIds
	if refs == nil {
		return false, errno.NewMQRetryErr("current evaluator source references missing")
	}
	if record.SourceType == entity.EvaluatorRecordSourceTypeInline {
		if record.InlineKey == "" {
			return false, errno.NewMQRetryErr("incomplete original inline evaluator source")
		}
		for _, ref := range refs.Inline {
			if ref == nil {
				return false, errno.NewMQRetryErr("incomplete inline source reference")
			}
			if ref.InlineKey == record.InlineKey {
				return ref.RecordID == record.ID, nil
			}
		}
		return false, nil
	}
	if record.EvaluatorVersionID <= 0 {
		return false, errno.NewMQRetryErr("original evaluator version missing")
	}
	for _, ref := range refs.Registered {
		if ref == nil {
			return false, errno.NewMQRetryErr("incomplete registered source reference")
		}
		if ref.VersionID == record.EvaluatorVersionID && ref.Alias == record.Alias {
			return ref.RecordID == record.ID, nil
		}
	}
	return record.Alias == "" && refs.EvalVerIDToResID[record.EvaluatorVersionID] == record.ID, nil
}

func sourceForExecution(ctx context.Context, port repo.IExptCurrentResultRepo, event *entity.ExptItemEvalEvent) (*entity.ExptCurrentItemRunSource, error) {
	if err := ctx.Err(); err != nil {
		return nil, errno.WrapMQRetryErr(err)
	}
	if execution := executionSource(ctx); execution != nil && execution.Source != nil {
		if execution.LockLost {
			return nil, errno.NewMQRetryErr("item execution ownership lost")
		}
		return execution.Source, nil
	}
	source, err := port.GetCurrentItemRunSource(ctx, event.SpaceID, event.ExptID, event.ExptRunID, event.EvalSetItemID)
	if err != nil {
		return nil, errno.WrapMQRetryErr(err)
	}
	if source == nil || source.Current.RunID != event.ExptRunID || source.Current.RequestID != event.RequestID {
		return nil, errExptNonCurrentSource
	}
	if source.Current.Mode != event.ExptRunMode && event.ExptRunMode != entity.EvaluationModeUnknown {
		return nil, errno.NewMQRetryErr("event source mode mismatch")
	}
	return source, nil
}
func checkedCurrentWrite(ctx context.Context, applied bool, err error) error {
	if err != nil {
		return errno.WrapMQRetryErr(err)
	}
	if !applied {
		return errExptNonCurrentSource
	}
	if err := ctx.Err(); err != nil {
		return errno.WrapMQRetryErr(err)
	}
	return nil
}

// handleCurrentEventErr runs after the execution lock exits. It acquires a fresh
// holder and rechecks the original source before budget, Fail, publication or release.
func (e *ExptItemEventEvalServiceImpl) handleCurrentEventErr(ctx context.Context, event *entity.ExptItemEvalEvent, evalErr error, port repo.IExptCurrentResultRepo, execution *currentItemExecution) error {
	if execution.Source == nil {
		return errno.WrapMQRetryErr(evalErr)
	}
	if execution.LockLost {
		return errno.NewMQRetryErr("item execution ownership lost")
	}
	if e.itemLocker == nil {
		return errno.NewMQRetryErr("independent error item locker unavailable")
	}
	if execution.LogCtx != nil {
		ctx = logs.SetLogID(ctx, logs.GetLogID(execution.LogCtx))
	}
	mutex := e.itemLocker()
	if mutex == nil {
		return errno.NewMQRetryErr("error item locker unavailable")
	}
	key := fmt.Sprintf("expt_item_eval_run_lock:%d:%d", event.ExptID, event.EvalSetItemID)
	locked, lockCtx, cancel, err := mutex.LockWithRenew(ctx, key, 5*time.Second, time.Minute)
	if err != nil {
		return errno.WrapMQRetryErr(err)
	}
	if !locked {
		return errno.NewMQRetryErr("error item execution lock busy")
	}
	var debt bool
	err = func() error {
		defer cancel()
		defer func() {
			if _, err := mutex.Unlock(key); err != nil {
				logs.CtxWarn(ctx, "unlock error item: %v", err)
			}
		}()
		if err := lockCtx.Err(); err != nil {
			return errno.WrapMQRetryErr(err)
		}
		current, err := port.GetCurrentItemRunSource(lockCtx, event.SpaceID, event.ExptID, event.ExptRunID, event.EvalSetItemID)
		if err != nil {
			return errno.WrapMQRetryErr(err)
		}
		if !sameExecutionIdentity(execution.Source, current) {
			return nil
		}
		finished := entity.IsItemRunFinished(current.Current.Status)
		if finished && (!execution.Completed || current.Current.Status != entity.ItemRunState_Fail || current.Current.ResultState != entity.ExptItemResultStateLogged) {
			return nil
		}
		valid, err := e.validateResumeSource(lockCtx, event, &current.Current)
		if err != nil {
			return err
		}
		if !valid {
			return nil
		}
		if e.experimentRepo == nil {
			return errno.NewMQRetryErr("frozen experiment reader unavailable")
		}
		expt, err := e.experimentRepo.GetByID(contexts.WithCtxWriteDB(lockCtx), event.ExptID, event.SpaceID)
		if err != nil {
			return errno.WrapMQRetryErr(err)
		}
		event = event.Clone()
		event.RetryTimes = max(event.RetryTimes, int(current.Current.RetryTimes))
		conf := e.configer.GetErrRetryConf(lockCtx, event.SpaceID, evalErr)
		maxRetry := conf.GetRetryTimes()
		if event.MaxRetryTimes > 0 {
			maxRetry = event.MaxRetryTimes
		}
		// The frozen pointer distinguishes an explicit zero from an absent budget.
		if expt != nil && expt.EvalConf != nil && expt.EvalConf.ItemRetryNum != nil {
			maxRetry = *expt.EvalConf.ItemRetryNum
		}
		retry := event.RetryTimes < maxRetry && !event.CtxForceNoRetry(ctx)
		if e.metric != nil {
			code, stable, _ := errno.ParseStatusError(evalErr)
			e.metric.EmitItemExecResult(event.SpaceID, int64(event.ExptRunMode), true, retry, stable, int64(code), event.CreateAt)
		}
		debt = conf.IsInDebt
		if debt {
			return nil
		}
		if finished {
			return nil
		}
		write := &entity.ExptCurrentItemRunWrite{Status: entity.ItemRunState_Fail, ErrMsg: []byte(errno.SerializeErr(evalErr)), RetryTimes: int32(event.RetryTimes), FailPendingTurns: true}
		if retry {
			write.RetryTimes++
			write.RetryOnly = true
			write.FailPendingTurns = false
			if event.Ext[entity.RetryYieldExtKey] == "true" {
				write.RetryOnly = false
				write.Yield = true
			}
		}
		if err := e.prepareMissingRawTurnLogs(lockCtx, execution.Source, write); err != nil {
			return err
		}
		applied, err := port.CompleteCurrentItemRun(lockCtx, execution.Source, write)
		if err := checkedCurrentWrite(lockCtx, applied, err); err != nil {
			if errors.Is(err, errExptNonCurrentSource) {
				return nil
			}
			return err
		}
		if retry {
			if write.Yield {
				return nil
			}
			event.RetryTimes = int(write.RetryTimes)
			event.AsyncReportTrigger = false
			event.AsyncEvaluatorReportTrigger = false
			event.ResumeSource = nil
			return e.publisher.PublishExptRecordEvalEvent(lockCtx, event, gptr.Of(conf.GetRetryInterval()), nil)
		}
		e.releaseCentralQuotaOutsideGate(lockCtx, event, evalErr, "item unretriable pre-exec failure")
		return nil
	}()
	if err != nil {
		return err
	}
	if debt {
		if terminate, ok := e.manager.(IExptCurrentRunTerminator); ok {
			_, err := terminate.TerminateCurrentRun(ctx, event.ExptID, event.ExptRunID, event.SpaceID, event.Session, execution.Source, entity.WithStatus(entity.ExptStatus_Terminated), entity.WithStatusMessage(evalErr.Error()))
			return err
		}
		return errno.NewMQRetryErr("source-aware debt termination provider missing")
	}
	return nil
}
func sameExecutionIdentity(expected, actual *entity.ExptCurrentItemRunSource) bool {
	if expected == nil || actual == nil {
		return false
	}
	a, b := expected.Current, actual.Current
	if a.SpaceID != b.SpaceID || a.ExptID != b.ExptID || a.RunID != b.RunID || a.ItemID != b.ItemID || a.ItemVersionID != b.ItemVersionID || a.RequestID != b.RequestID || a.Mode != b.Mode || len(expected.Turns) != len(actual.Turns) {
		return false
	}
	if len(expected.CanonicalTurns) != len(actual.CanonicalTurns) {
		return false
	}
	for index, turn := range expected.CanonicalTurns {
		other := actual.CanonicalTurns[index]
		if turn == nil || other == nil || turn.ID != other.ID || turn.TurnID != other.TurnID || turn.ItemVersionID != other.ItemVersionID || turn.TargetResultID != other.TargetResultID {
			return false
		}
	}
	for id, turn := range expected.Turns {
		other := actual.Turns[id]
		if turn == nil || other == nil || turn.ID != other.ID || turn.TargetResultID != other.TargetResultID || !entity.SameCurrentEvaluatorRefs(turn.EvaluatorResultIds, other.EvaluatorResultIds) {
			return false
		}
	}
	return reflect.DeepEqual(expected.CurrentRefs, actual.CurrentRefs)
}

func (e *ExptSchedulerImpl) withCurrentScannedItem(ctx context.Context, event *entity.ExptScheduleEvent, item *entity.ExptEvalItem, callback func(context.Context, repo.IExptCurrentResultRepo) (bool, error)) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, errno.WrapMQRetryErr(err)
	}
	if item == nil || item.Source == nil {
		return false, errno.NewMQRetryErr("same-scan original item source missing")
	}
	port, ok := e.ExptItemResultRepo.(repo.IExptCurrentResultRepo)
	if !ok {
		return false, errno.NewMQRetryErr("source-conditional scan writer unavailable")
	}
	if e.itemLocker == nil {
		return false, errno.NewMQRetryErr("scan item locker unavailable")
	}
	mutex := e.itemLocker()
	if mutex == nil {
		return false, errno.NewMQRetryErr("scan item locker unavailable")
	}
	key := fmt.Sprintf("expt_item_eval_run_lock:%d:%d", event.ExptID, item.ItemID)
	locked, lockCtx, cancel, err := mutex.LockWithRenew(ctx, key, 5*time.Second, time.Minute)
	if err != nil {
		return false, errno.WrapMQRetryErr(err)
	}
	if !locked {
		return false, nil
	}
	defer cancel()
	defer func() {
		if _, err := mutex.Unlock(key); err != nil {
			logs.CtxWarn(ctx, "unlock scanned item: %v", err)
		}
	}()
	current, err := port.GetCurrentItemRunSource(lockCtx, event.SpaceID, event.ExptID, event.ExptRunID, item.ItemID)
	if err != nil {
		return false, errno.WrapMQRetryErr(err)
	}
	if !sameExecutionIdentity(item.Source, current) || current.Current.Status != entity.ItemRunState_Processing || item.Source.Current.RetryTimes != current.Current.RetryTimes || item.Source.Current.UpdatedAt == nil || current.Current.UpdatedAt == nil || !item.Source.Current.UpdatedAt.Equal(*current.Current.UpdatedAt) {
		return false, nil
	}
	if err := lockCtx.Err(); err != nil {
		return false, errno.WrapMQRetryErr(err)
	}
	lockCtx = context.WithValue(lockCtx, currentItemSourceKey{}, &currentItemExecution{Source: item.Source})
	applied, err := callback(lockCtx, port)
	if err != nil {
		return false, errno.WrapMQRetryErr(err)
	}
	return applied, nil
}

func (e *ExptSchedulerImpl) handleCurrentZombies(ctx context.Context, event *entity.ExptScheduleEvent, items []*entity.ExptEvalItem, expt *entity.Experiment, zombieSecond int, asyncExec bool) (alives, zombies []*entity.ExptEvalItem, err error) {
	for _, item := range items {
		if item == nil || item.State != entity.ItemRunState_Processing {
			continue
		}
		if item.UpdatedAt == nil || item.UpdatedAt.IsZero() || time.Since(*item.UpdatedAt).Seconds() <= float64(zombieSecond) {
			alives = append(alives, item)
			continue
		}
		applied, err := e.withCurrentScannedItem(ctx, event, item, func(lockCtx context.Context, port repo.IExptCurrentResultRepo) (bool, error) {
			zombieErr := errno.NewItemZombieTimeoutErr(zombieSecond, asyncExec)
			write := &entity.ExptCurrentItemRunWrite{Status: entity.ItemRunState_Fail, ErrMsg: []byte(errno.SerializeErr(zombieErr)), RetryTimes: item.RetryTimes, FailPendingTurns: true}
			if err := prepareMissingRawTurnLogs(lockCtx, e.IDGen, item.Source, write); err != nil {
				return false, err
			}
			if err := e.terminateZombieEvaluatorRecords(lockCtx, event, []int64{item.ItemID}); err != nil {
				return false, err
			}
			if err := lockCtx.Err(); err != nil {
				return false, errno.WrapMQRetryErr(err)
			}
			if err := e.terminateZombieEvalTargetRecords(lockCtx, event, expt, []int64{item.ItemID}); err != nil {
				return false, err
			}
			applied, err := port.CompleteCurrentItemRun(lockCtx, item.Source, write)
			if err != nil || !applied {
				return false, err
			}
			e.releaseCentralQuotaForItems(lockCtx, expt, event.ExptRunID, []int64{item.ItemID}, map[int64]string{item.ItemID: item.Source.Current.RequestID}, "item zombie timeout")
			if e.sandboxAgentNotifier != nil {
				if err := e.sandboxAgentNotifier.NotifyItemFail(lockCtx, expt, item.ItemID, zombieErr); err != nil {
					logs.CtxWarn(lockCtx, "notify zombie item: %v", err)
				}
			}
			return true, nil
		})
		if err != nil {
			return nil, nil, err
		}
		if applied {
			copy := *item
			copy.State = entity.ItemRunState_Fail
			zombies = append(zombies, &copy)
		} else {
			alives = append(alives, item)
		}
	}
	return alives, zombies, nil
}
func (e *ExptSchedulerImpl) sweepCurrentSandboxItems(ctx context.Context, event *entity.ExptScheduleEvent, items []*entity.ExptEvalItem, expt *entity.Experiment) (alives, terminated []*entity.ExptEvalItem, err error) {
	for _, item := range items {
		if item == nil || item.State != entity.ItemRunState_Processing {
			alives = append(alives, item)
			continue
		}
		applied, err := e.withCurrentScannedItem(ctx, event, item, func(lockCtx context.Context, port repo.IExptCurrentResultRepo) (bool, error) {
			ids := frozenTargetRecordIDs(item.Source)
			write := &entity.ExptCurrentItemRunWrite{Status: entity.ItemRunState_Fail, RetryTimes: item.RetryTimes, FailPendingTurns: true}
			// Prepare before any sandbox RPC or cleanup.
			if err := prepareMissingRawTurnLogs(lockCtx, e.IDGen, item.Source, write); err != nil {
				return false, err
			}

			if len(ids) == 0 {
				return false, nil
			}
			space := resolveLoadSpaceID(event.SpaceID, expt.TargetSpaceID)
			stopped, status := e.evalTargetService.CheckSandboxTerminated(lockCtx, space, ids)
			if len(stopped) == 0 {
				return false, nil
			}
			state := status[stopped[0]]
			if state == "" {
				state = "Terminated"
			}
			write.ErrMsg = []byte(errno.SerializeErr(errno.NewSandboxTerminatedBeforeReportErr(state)))
			for _, log := range write.MissingTurnLogs {
				log.ErrMsg = string(write.TurnError(log.Status))
			}
			if err := lockCtx.Err(); err != nil {
				return false, errno.WrapMQRetryErr(err)
			}
			_ = e.evalTargetService.TerminateAsyncRecordsAndDestroySandbox(lockCtx, space, stopped, int32(errno.SandboxTerminatedBeforeReportCode), fmt.Sprintf("sandbox execute reached terminal state (%s) before result was reported", state), false)
			applied, err := port.CompleteCurrentItemRun(lockCtx, item.Source, write)
			if err != nil || !applied {
				return false, err
			}
			e.releaseCentralQuotaForItems(lockCtx, expt, event.ExptRunID, []int64{item.ItemID}, map[int64]string{item.ItemID: item.Source.Current.RequestID}, "sandbox terminated before report")
			byRecord := map[int64][]int64{}
			for _, id := range stopped {
				byRecord[id] = []int64{item.ItemID}
			}
			e.emitSandboxSweptInvokeFinished(lockCtx, event, expt, stopped, byRecord)
			return true, nil
		})
		if err != nil {
			return nil, nil, err
		}
		if applied {
			copy := *item
			copy.State = entity.ItemRunState_Fail
			terminated = append(terminated, &copy)
		} else {
			alives = append(alives, item)
		}
	}
	return alives, terminated, nil
}

// IDs are prepared before SQL and under the item lock; the transaction never calls IDGen.
func (e *ExptItemEventEvalServiceImpl) prepareMissingRawTurnLogs(ctx context.Context, source *entity.ExptCurrentItemRunSource, write *entity.ExptCurrentItemRunWrite) error {
	return prepareMissingRawTurnLogs(ctx, e.idgen, source, write)
}
func prepareMissingRawTurnLogs(ctx context.Context, gen idgen.IIDGenerator, source *entity.ExptCurrentItemRunSource, write *entity.ExptCurrentItemRunWrite) error {
	if err := ctx.Err(); err != nil {
		return errno.WrapMQRetryErr(err)
	}
	if source.Current.RequestID != "" || write.RetryOnly {
		return nil
	}
	missing := []*entity.ExptTurnResult{}
	for _, turn := range source.CanonicalTurns {
		if source.Turns[turn.TurnID] == nil {
			missing = append(missing, turn)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	if gen == nil {
		return errno.NewMQRetryErr("raw turn log ID preparation unavailable")
	}
	ids, err := gen.GenMultiIDs(ctx, len(missing))
	if err != nil {
		return errno.WrapMQRetryErr(err)
	}
	if err := ctx.Err(); err != nil {
		return errno.WrapMQRetryErr(err)
	}
	if len(ids) != len(missing) {
		return errno.NewMQRetryErr("raw turn log ID preparation incomplete")
	}
	for index, turn := range missing {
		log := turn.ToRunLogDO()
		log.ID = ids[index]
		log.ExptRunID = source.Current.RunID
		log.Status = entity.TurnRunState_Fail
		if write.Yield {
			log.Status = entity.TurnRunState_Queueing
		}
		if write.Status == entity.ItemRunState_Terminal {
			log.Status = entity.TurnRunState_Terminal
		}
		log.ErrMsg = string(write.TurnError(log.Status))
		write.MissingTurnLogs = append(write.MissingTurnLogs, log)
	}
	return nil
}

func frozenTargetRecordIDs(source *entity.ExptCurrentItemRunSource) []int64 {
	seen := map[int64]bool{}
	ids := []int64{}
	for _, log := range source.Turns {
		if log != nil && log.TargetResultID > 0 && !seen[log.TargetResultID] {
			seen[log.TargetResultID] = true
			ids = append(ids, log.TargetResultID)
		}
	}
	for _, turn := range source.CanonicalTurns {
		if turn != nil && source.Turns[turn.TurnID] == nil && turn.TargetResultID > 0 && !seen[turn.TargetResultID] {
			seen[turn.TargetResultID] = true
			ids = append(ids, turn.TargetResultID)
		}
	}
	return ids
}
