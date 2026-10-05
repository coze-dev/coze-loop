// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type hookTargetSandboxCleaner interface {
	CleanupHookTargetSandboxes(context.Context, entity.HookRunKey, []*entity.EvalTargetRecord) error
}

var _ hookTargetSandboxCleaner = (*EvalTargetServiceImpl)(nil)

var errHookTargetCleanupUncertain = errors.New("hook target sandbox cleanup is unconfirmed")

// CleanupHookTargetSandboxes accepts records selected by the caller's original-Run manifest.
// Success confirms durable cancellation or terminal state, not physical teardown; records stay intact.
func (e *EvalTargetServiceImpl) CleanupHookTargetSandboxes(ctx context.Context, key entity.HookRunKey, records []*entity.EvalTargetRecord) error {
	if key.WorkspaceID <= 0 || key.ExperimentID <= 0 || key.RunID <= 0 {
		return errors.New("invalid hook target cleanup run key")
	}
	// Validate the entire page before issuing any cancellation, including cross-space records.
	for _, record := range records {
		if record == nil || record.ID <= 0 || record.SpaceID <= 0 || record.TargetID <= 0 || record.TargetVersionID <= 0 || record.ExperimentRunID != key.RunID {
			return errors.New("hook target cleanup record does not belong to the original run")
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}
	if e == nil || hookExecutionNil(e.evalTargetRepo) {
		return errors.New("hook target cleanup repository is unavailable")
	}
	for _, record := range records {
		target, err := e.evalTargetRepo.GetEvalTargetVersion(ctx, record.SpaceID, record.TargetVersionID)
		if err != nil {
			return fmt.Errorf("hook target cleanup version lookup: %w", err)
		}
		if target == nil || target.EvalTargetVersion == nil || target.ID != record.TargetID || target.SpaceID != record.SpaceID ||
			target.EvalTargetVersion.ID != record.TargetVersionID || target.EvalTargetVersion.SpaceID != record.SpaceID || target.EvalTargetVersion.TargetID != record.TargetID {
			return errors.New("hook target cleanup target version identity is unconfirmed")
		}
		if target.EvalTargetType != entity.EvalTargetTypeSandboxAgent {
			continue
		}
		if hookExecutionNil(e.sandboxSchedulerAdapter) {
			return errors.New("hook target cleanup sandbox scheduler is unavailable")
		}
		for _, executeID := range sandboxExecuteIDsOf(ctx, record) {
			taskID := strconv.FormatInt(key.ExperimentID, 10)
			if strings.HasSuffix(executeID, sandboxMacVMExecuteIDSuffix) {
				taskID += sandboxMacVMTaskIDSuffix
			}
			if err := e.cleanupHookSandboxExecute(ctx, record.SpaceID, taskID, executeID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *EvalTargetServiceImpl) cleanupHookSandboxExecute(ctx context.Context, workspaceID int64, taskID, executeID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// A task is shared across Runs; single-execute requests also make affected_count unambiguous.
	ack, destroyErr := e.sandboxSchedulerAdapter.Destroy(ctx, &rpc.SandboxDestroyRequest{
		TaskID: taskID, DestroyType: rpc.SandboxDestroyTypeExecute,
		ExecuteIDs: []string{executeID}, WorkspaceID: workspaceID,
	})
	if destroyErr == nil && ack != nil && ack.AffectedCount > 0 {
		return nil
	}
	// Destroy may swallow read/CAS errors and return zero,nil. Absence is not proof of cleanup.
	state, getErr := e.sandboxSchedulerAdapter.Get(ctx, &rpc.SandboxGetRequest{ExecuteID: executeID, WorkspaceID: workspaceID})
	if getErr == nil && state != nil && state.ExecuteInfo != nil {
		info := state.ExecuteInfo
		if info.ExecuteID == executeID && info.TaskID == taskID {
			switch info.Status {
			case rpc.SandboxExecuteStatusCanceling, rpc.SandboxExecuteStatusSucceeded, rpc.SandboxExecuteStatusFailed,
				rpc.SandboxExecuteStatusCanceled, rpc.SandboxExecuteStatusFinished:
				return nil
			}
		}
	}
	return errors.Join(fmt.Errorf("%w: task=%s execute=%s", errHookTargetCleanupUncertain, taskID, executeID), destroyErr, getErr)
}
