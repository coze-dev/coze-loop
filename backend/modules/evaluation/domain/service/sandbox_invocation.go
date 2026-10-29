// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"strconv"
	"time"
)

// WithSandboxLeasePendingRegistrar connects the commercial durable reconciliation queue.
func (e *EvalTargetServiceImpl) WithSandboxLeasePendingRegistrar(register func(context.Context, *entity.EvalTargetRecord, int64, int64) error) {
	e.sandboxLeasePendingRegistrar = register
}

type sandboxInvocationLifecycle struct {
	service *EvalTargetServiceImpl
	spaceID int64
	target  *entity.EvalTarget
	param   *entity.ExecuteTargetCtx
	input   *entity.EvalTargetInputData
	record  *entity.EvalTargetRecord
	resumed bool
	traceID string
}

func (l *sandboxInvocationLifecycle) schedulerSpaceID() int64 { return l.spaceID }
func (l *sandboxInvocationLifecycle) experimentSpaceID() int64 {
	if l.param.ExptSpaceID > 0 {
		return l.param.ExptSpaceID
	}
	return l.spaceID
}

func (l *sandboxInvocationLifecycle) check(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if l.record == nil {
		return nil
	}
	active, err := l.service.evalTargetRepo.(repo.SandboxInvocationRepo).SandboxInvocationOwnerActive(ctx, l.experimentSpaceID(), gptr.Indirect(l.param.ExperimentID), l.record.ExperimentRunID, l.record.ItemID)
	if err != nil {
		return err
	}
	if !active {
		return errorx.NewByCode(errno.AsyncEvalTargetTerminatedCode)
	}
	current, err := l.service.evalTargetRepo.GetEvalTargetRecordByIDAndSpaceID(contexts.WithCtxWriteDB(ctx), l.spaceID, l.record.ID)
	if err != nil {
		return err
	}
	if current == nil || gptr.Indirect(current.Status) != entity.EvalTargetRunStatusAsyncInvoking {
		return errorx.NewByCode(errno.AsyncEvalTargetTerminatedCode)
	}
	return nil
}

func (l *sandboxInvocationLifecycle) prepare(ctx context.Context, invokeID int64, ids []string) (int64, error) {
	// Debug calls have no durable experiment owner and keep their existing lifecycle.
	if gptr.Indirect(l.param.ExperimentID) == 0 || gptr.Indirect(l.param.ExperimentRunID) == 0 {
		return 0, nil
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	expectedID := strconv.FormatInt(invokeID, 10)
	if len(ids) != 2 || ids[0] != expectedID+"-windows" || ids[1] != expectedID+"-orch" {
		return 0, fmt.Errorf("invalid Windows sandbox execution identities")
	}
	store, ok := l.service.evalTargetRepo.(repo.SandboxInvocationRepo)
	if !ok || l.service.sandboxLeasePendingRegistrar == nil {
		return 0, fmt.Errorf("sandbox durable lifecycle is not configured")
	}
	old, err := store.LatestSandboxInvocation(ctx, l.spaceID, gptr.Indirect(l.param.ExperimentRunID), l.param.ItemID, l.param.TurnID)
	if err != nil {
		return 0, err
	}
	if old != nil && old.Ext["sandbox_lifecycle_version"] == "1" && (old.TargetID != l.target.ID || old.TargetVersionID != l.target.EvalTargetVersion.ID || old.Ext["sandbox_lifecycle_expt_id"] != strconv.FormatInt(gptr.Indirect(l.param.ExperimentID), 10) || old.Ext["sandbox_lifecycle_scheduler_space_id"] != strconv.FormatInt(l.schedulerSpaceID(), 10) || old.Ext["sandbox_lifecycle_experiment_space_id"] != strconv.FormatInt(l.experimentSpaceID(), 10)) {
		return 0, fmt.Errorf("sandbox invocation owner mismatch")
	}
	if sandboxInvocationReported(old) {
		l.record, l.resumed = old, true
		return old.ID, nil
	}
	if old != nil && old.Ext["sandbox_lifecycle_version"] == "1" && gptr.Indirect(old.Status) == entity.EvalTargetRunStatusAsyncInvoking {
		if old.Ext["sandbox_lifecycle_phase"] == "ready" {
			l.record, l.resumed = old, true
			if done, err := l.loadReported(ctx); err != nil {
				return 0, err
			} else if done {
				return old.ID, nil
			}
			if err := l.check(ctx); err != nil {
				if done, _ := l.loadReported(ctx); done {
					return old.ID, nil
				}
				return 0, err
			}
			return old.ID, nil
		}
		// The item lock is renewed throughout initialization. A replacement holder may
		// fence an interrupted initializer, but must never replay its shell commands.
		old.Status = gptr.Of(entity.EvalTargetRunStatusFail)
		old.Ext["sandbox_lifecycle_phase"] = "aborted"
		if old.EvalTargetOutputData == nil {
			old.EvalTargetOutputData = &entity.EvalTargetOutputData{}
		}
		old.EvalTargetOutputData.EvalTargetRunError = &entity.EvalTargetRunError{Code: int32(errno.CommonInternalErrorCode), Message: "sandbox initialization interrupted before ready"}
		transitioned, err := store.TransitionSandboxInvocation(ctx, old, "initializing")
		if err != nil {
			return 0, err
		}
		if !transitioned {
			return 0, fmt.Errorf("sandbox initialization ownership changed")
		}
		l.service.destroySandboxExecute(ctx, strconv.FormatInt(gptr.Indirect(l.param.ExperimentID), 10), l.schedulerSpaceID(), sandboxExecuteIDsOf(ctx, old), false)
	}
	ext := make(map[string]string)
	for k, v := range l.input.GetExt() {
		ext[k] = v
	}
	ext["sandbox_lifecycle_version"] = "1"
	ext["sandbox_lifecycle_phase"] = "initializing"
	ext["sandbox_lifecycle_experiment_space_id"] = strconv.FormatInt(l.experimentSpaceID(), 10)
	ext["sandbox_lifecycle_expt_id"] = strconv.FormatInt(gptr.Indirect(l.param.ExperimentID), 10)
	ext["sandbox_lifecycle_execute_ids"] = json.Jsonify(ids)
	ext["sandbox_lifecycle_scheduler_space_id"] = strconv.FormatInt(l.schedulerSpaceID(), 10)
	l.record = &entity.EvalTargetRecord{
		ID: invokeID, SpaceID: l.spaceID, TargetID: l.target.ID,
		TargetVersionID: l.target.EvalTargetVersion.ID,
		ExperimentRunID: gptr.Indirect(l.param.ExperimentRunID),
		ItemID:          l.param.ItemID, TurnID: l.param.TurnID,
		EvalTargetInputData: l.input,
		EvalTargetOutputData: &entity.EvalTargetOutputData{
			OutputFields:    map[string]*entity.Content{},
			EvalTargetUsage: &entity.EvalTargetUsage{}, TimeConsumingMS: gptr.Of(int64(0)),
			Ext: map[string]string{"sandbox_execute_ids": json.Jsonify(ids)},
		},
		Status: gptr.Of(entity.EvalTargetRunStatusAsyncInvoking), Ext: ext,
		LogID: logs.GetLogID(ctx), TraceID: l.traceID,
		BaseInfo: &entity.BaseInfo{
			CreatedBy: &entity.UserInfo{UserID: gptr.Of(session.UserIDInCtxOrEmpty(ctx))},
			UpdatedBy: &entity.UserInfo{UserID: gptr.Of(session.UserIDInCtxOrEmpty(ctx))},
			CreatedAt: gptr.Of(time.Now().UnixMilli()), UpdatedAt: gptr.Of(time.Now().UnixMilli()),
		},
	}
	if _, err := l.service.evalTargetRepo.CreateEvalTargetRecord(ctx, l.record, l.param.TruncateLargeContent); err != nil {
		return 0, err
	}
	if err := l.service.sandboxLeasePendingRegistrar(ctx, l.record, gptr.Indirect(l.param.ExperimentID), l.schedulerSpaceID()); err != nil {
		return 0, err
	}
	return 0, l.check(ctx)
}

func (l *sandboxInvocationLifecycle) finish(ctx context.Context) error {
	if done, err := l.loadReported(ctx); err != nil {
		return err
	} else if done {
		return nil
	}
	if err := l.check(ctx); err != nil {
		if done, _ := l.loadReported(ctx); done {
			return nil
		}
		l.service.destroySandboxExecute(ctx, strconv.FormatInt(gptr.Indirect(l.param.ExperimentID), 10), l.schedulerSpaceID(), sandboxExecuteIDsOf(ctx, l.record), false)
		return err
	}
	if l.resumed {
		return nil
	}
	l.record.Ext["sandbox_lifecycle_phase"] = "ready"
	ok, err := l.service.evalTargetRepo.(repo.SandboxInvocationRepo).TransitionSandboxInvocation(ctx, l.record, "initializing")
	if err != nil {
		return err
	}
	if !ok {
		if done, err := l.loadReported(ctx); err != nil {
			return err
		} else if done {
			return nil
		}
		return errorx.NewByCode(errno.AsyncEvalTargetTerminatedCode)
	}
	return nil
}

// TerminateSandboxAttemptsForRunItems discovers pre-dispatch and superseded attempts
// which have not yet been attached to a turn's TargetResultID.
func (e *EvalTargetServiceImpl) TerminateSandboxAttemptsForRunItems(ctx context.Context, spaceID, runID int64, itemIDs []int64) error {
	ctx = contexts.WithCtxWriteDB(ctx)
	attempts, err := e.evalTargetRepo.ListItemRetryRecordsByRun(ctx, spaceID, runID, itemIDs)
	if err != nil {
		return err
	}
	var ids []int64
	for _, records := range attempts {
		for _, record := range records {
			if record != nil {
				ids = append(ids, record.RecordID)
			}
		}
	}
	e.TerminateAsyncRecordsAndDestroySandbox(ctx, spaceID, ids, int32(errno.AsyncEvalTargetTerminatedCode), "async eval target terminated: experiment cancelled", false)
	return nil
}

func (l *sandboxInvocationLifecycle) abort(ctx context.Context, cause error) {
	if l.record == nil || l.resumed {
		return
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), evalTargetRecordPersistTimeout)
	defer cancel()
	l.record.Status = gptr.Of(entity.EvalTargetRunStatusFail)
	l.record.Ext["sandbox_lifecycle_phase"] = "aborted"
	l.record.EvalTargetOutputData.EvalTargetRunError = &entity.EvalTargetRunError{Code: int32(errno.CommonInternalErrorCode), Message: cause.Error()}
	if store, ok := l.service.evalTargetRepo.(repo.SandboxInvocationRepo); ok {
		if _, err := store.TransitionSandboxInvocation(cleanupCtx, l.record, "initializing"); err != nil {
			logs.CtxWarn(cleanupCtx, "sandbox initialization failure persistence failed, record_id=%d, err=%v", l.record.ID, err)
		}
	}
	l.service.destroySandboxExecute(cleanupCtx, strconv.FormatInt(gptr.Indirect(l.param.ExperimentID), 10), l.schedulerSpaceID(), sandboxExecuteIDsOf(cleanupCtx, l.record), false)
}

func sandboxRecordSchedulerSpaceID(r *entity.EvalTargetRecord) int64 {
	if r.Ext["sandbox_lifecycle_version"] == "1" {
		if spaceID, err := strconv.ParseInt(r.Ext["sandbox_lifecycle_scheduler_space_id"], 10, 64); err == nil && spaceID > 0 {
			return spaceID
		}
	}
	return r.SpaceID
}

func sandboxInvocationReported(r *entity.EvalTargetRecord) bool {
	return r != nil && r.Ext["sandbox_lifecycle_version"] == "1" && r.Ext["sandbox_lifecycle_phase"] != "aborted" && (gptr.Indirect(r.Status) == entity.EvalTargetRunStatusSuccess || gptr.Indirect(r.Status) == entity.EvalTargetRunStatusFail)
}
func (l *sandboxInvocationLifecycle) loadReported(ctx context.Context) (bool, error) {
	r, err := l.service.evalTargetRepo.GetEvalTargetRecordByIDAndSpaceID(contexts.WithCtxWriteDB(ctx), l.spaceID, l.record.ID)
	if err != nil {
		return false, err
	}
	if sandboxInvocationReported(r) {
		l.record = r
		return true, nil
	}
	return false, nil
}
