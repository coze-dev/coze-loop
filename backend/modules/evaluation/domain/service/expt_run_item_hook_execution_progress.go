// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"sync"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type itemHookProgressContextKey struct{}
type itemHookProgressBinding struct {
	key         entity.HookRunKey
	itemID      int64
	itemVersion int64
	repo        repo.IHookTurnProgressRepo
	targets     *sync.Map
}

func rememberItemHookTarget(ctx context.Context, etec *entity.ExptTurnEvalCtx, record *entity.EvalTargetRecord) {
	b, ok := ctx.Value(itemHookProgressContextKey{}).(itemHookProgressBinding)
	if !ok || b.targets == nil || record == nil || record.ID <= 0 {
		return
	}
	b.targets.Store(etec.Turn.ID, record.ID)
}

func itemHookProgressHasRecord(refs *entity.EvaluatorResults, id int64) bool {
	if refs == nil {
		return false
	}
	for _, v := range refs.EvalVerIDToResID {
		if v == id {
			return true
		}
	}
	for _, v := range refs.Registered {
		if v != nil && v.RecordID == id {
			return true
		}
	}
	for _, v := range refs.Inline {
		if v != nil && v.RecordID == id {
			return true
		}
	}
	return false
}

func itemHookProgressFor(ctx context.Context, etec *entity.ExptTurnEvalCtx) (itemHookProgressBinding, error) {
	b, ok := ctx.Value(itemHookProgressContextKey{}).(itemHookProgressBinding)
	if !ok || hookExecutionNil(b.repo) || etec == nil || etec.Event == nil || etec.Turn == nil || etec.EvalSetItem == nil || etec.Expt == nil || etec.Expt.ID != b.key.ExperimentID || etec.Expt.SpaceID != b.key.WorkspaceID || b.key != itemHookKey(etec.Event) || b.itemID != etec.Event.EvalSetItemID || b.itemID != etec.EvalSetItem.ItemID {
		return itemHookProgressBinding{}, itemHookControlError{wait: true}
	}
	return b, nil
}

func readItemHookContinuation(ctx context.Context, etec *entity.ExptTurnEvalCtx) (*entity.ExptTurnResultRunLog, error) {
	if ctx.Value(itemHookProgressContextKey{}) == nil || !etec.Event.HookControlContinuation {
		return nil, nil
	}
	b, err := itemHookProgressFor(ctx, etec)
	if err != nil {
		return nil, err
	}
	k := entity.HookTurnProgressIdentity(etec.GetExistTurnResultRunLog(etec.Turn.ID))
	if k.HookRunKey != b.key || k.ItemID != b.itemID || k.TurnID != etec.Turn.ID || k.Validate() != nil {
		return nil, itemHookControlError{wait: true}
	}
	readCtx, cancel := context.WithTimeout(ctx, itemHookDependencyTimeout)
	defer cancel()
	current, err := b.repo.ReadTurnProgress(readCtx, k)
	if err != nil || readCtx.Err() != nil || entity.HookTurnProgressIdentity(current) != k {
		return nil, itemHookControlError{wait: true}
	}
	if current.Status == entity.TurnRunState_Terminal {
		return nil, itemHookControlError{}
	}
	return current, nil
}

func writeItemHookProgress(ctx context.Context, etec *entity.ExptTurnEvalCtx, base, next *entity.ExptTurnResultRunLog) (*entity.ExptTurnResultRunLog, error) {
	b, err := itemHookProgressFor(ctx, etec)
	if err != nil {
		return nil, err
	}
	k := entity.HookTurnProgressIdentity(base)
	if k.HookRunKey != b.key || k.ItemID != b.itemID || k.TurnID != etec.Turn.ID {
		return nil, itemHookControlError{wait: true}
	}
	current, err := b.repo.WriteTurnProgress(ctx, entity.HookTurnProgressInput{Base: base, Progress: next})
	if err != nil || entity.HookTurnProgressIdentity(current) != k {
		return nil, itemHookControlError{wait: true}
	}
	return current, nil
}

func itemHookManagedWrite(ctx context.Context) bool {
	return ctx.Value(itemHookProgressContextKey{}) != nil || ctx.Value(itemHookExecutionKey{}) != nil
}

func writeItemHookResult(ctx context.Context, etec *entity.ExptTurnEvalCtx, base, next *entity.ExptTurnResultRunLog) (*entity.ExptTurnResultRunLog, error) {
	b, err := itemHookProgressFor(ctx, etec)
	if err != nil {
		logs.CtxWarn(ctx, "hook result write rejected: stage=context_binding")
		return nil, err
	}
	k := entity.HookTurnProgressIdentity(base)
	writer, ok := b.repo.(repo.IHookTurnResultWriteRepo)
	if !ok || hookExecutionNil(writer) || k.Validate() != nil || k.HookRunKey != b.key || k.ItemID != b.itemID || k.ItemVersionID != b.itemVersion || k.TurnID != etec.Turn.ID || k != entity.HookTurnProgressIdentity(next) {
		logs.CtxWarn(ctx, "hook result write rejected: stage=identity run=%d item=%d turn=%d version=%d expected_version=%d writer=%t", b.key.RunID, k.ItemID, k.TurnID, k.ItemVersionID, b.itemVersion, ok)
		return nil, itemHookControlError{wait: true}
	}
	current, err := writer.WriteTurnResult(ctx, entity.HookTurnProgressInput{Base: base, Progress: next})
	if err != nil || entity.HookTurnProgressIdentity(current) != k {
		logs.CtxWarn(ctx, "hook result write rejected: stage=repository run=%d item=%d turn=%d err=%v", b.key.RunID, k.ItemID, k.TurnID, err)
		return nil, itemHookControlError{wait: true}
	}
	return current, nil
}

func writeItemHookRun(ctx context.Context, key entity.HookRunKey, itemID int64, status entity.ItemRunState, errMsg *string) (bool, error) {
	b, ok := ctx.Value(itemHookProgressContextKey{}).(itemHookProgressBinding)
	if !ok || b.key != key || b.itemID != itemID || hookExecutionNil(b.repo) {
		return false, itemHookControlError{wait: true}
	}
	writer, ok := b.repo.(repo.IHookItemRunWriteRepo)
	if !ok || hookExecutionNil(writer) {
		return false, itemHookControlError{wait: true}
	}
	terminal, err := writer.WriteItemRun(ctx, entity.HookItemRunWriteInput{HookRunKey: key, ItemID: itemID, ItemVersionID: b.itemVersion, Status: status, ErrMsg: errMsg})
	if err != nil {
		return false, itemHookControlError{wait: true}
	}
	return terminal, nil
}

func reuseHookTarget(etec *entity.ExptTurnEvalCtx, current *entity.ExptTurnResultRunLog, record *entity.EvalTargetRecord, ordinaryReuse bool) (bool, error) {
	if ordinaryReuse && etec.Event.ExptRunMode == entity.EvaluationModeFailRetry && record != nil && record.ExperimentRunID > 0 && record.ExperimentRunID != current.ExptRunID {
		if m := etec.HookManifest; m != nil {
			if m.TargetRecordRun(record.ID) != record.ExperimentRunID || record.ID != current.TargetResultID || record.ItemID != current.ItemID || record.ItemVersionID != current.ItemVersionID || record.TurnID != current.TurnID || record.SpaceID != resolveLoadSpaceID(etec.Event.SpaceID, etec.TargetSourceSpaceID()) || record.TargetID != etec.Expt.TargetID || record.TargetVersionID != etec.Expt.TargetVersionID || gptr.Indirect(record.Status) != entity.EvalTargetRunStatusSuccess {
				return false, itemHookControlError{wait: true}
			}
		}
		if record.ID == current.TargetResultID {
			return true, nil
		}
		return false, itemHookControlError{wait: true}
	}
	if record == nil && current.TargetResultID == 0 {
		return false, nil
	}
	if record == nil || record.ID <= 0 || record.ID != current.TargetResultID || record.SpaceID != resolveLoadSpaceID(etec.Event.SpaceID, etec.TargetSourceSpaceID()) || record.ExperimentRunID != current.ExptRunID || record.ItemID != current.ItemID || record.ItemVersionID != current.ItemVersionID || record.TurnID != current.TurnID || etec.Expt.Target == nil || etec.Expt.Target.EvalTargetVersion == nil || record.TargetID != etec.Expt.Target.ID || record.TargetVersionID != etec.Expt.Target.EvalTargetVersion.ID {
		return false, itemHookControlError{wait: true}
	}
	status := gptr.Indirect(record.Status)
	return status == entity.EvalTargetRunStatusSuccess || status == entity.EvalTargetRunStatusAsyncInvoking, nil
}

func reuseHookEvaluator(ctx context.Context, etec *entity.ExptTurnEvalCtx, current *entity.ExptTurnResultRunLog, target *entity.EvalTargetRecord, record *entity.EvaluatorRecord, version int64, alias string, ordinaryReuse bool) (bool, error) {
	if target != nil && target.ID != current.TargetResultID {
		b, _ := ctx.Value(itemHookProgressContextKey{}).(itemHookProgressBinding)
		if b.targets != nil {
			if id, executed := b.targets.Load(etec.Turn.ID); executed && id == target.ID {
				return false, nil
			}
		}
		return false, itemHookControlError{wait: true}
	}
	var id int64
	if refs := current.EvaluatorResultIds; refs != nil {
		if alias == "" {
			id = refs.EvalVerIDToResID[version]
		}
		for _, ref := range refs.Registered {
			if ref != nil && ref.VersionID == version && ref.Alias == alias {
				if id != 0 && id != ref.RecordID {
					return false, itemHookControlError{wait: true}
				}
				id = ref.RecordID
			}
		}
	}
	if record == nil && id == 0 {
		return false, nil
	}
	if ordinaryReuse && etec.Event.ExptRunMode == entity.EvaluationModeFailRetry && record != nil && record.ExperimentRunID > 0 && record.ExperimentRunID != current.ExptRunID {
		if m := etec.HookManifest; m != nil {
			if m.EvaluatorRecordRun(record.ID) != record.ExperimentRunID || record.ItemID != current.ItemID || record.ItemVersionID != current.ItemVersionID || record.TurnID != current.TurnID || record.ExperimentID != current.ExptID || record.EvaluatorVersionID != version || record.Alias != alias || record.Status != entity.EvaluatorRunStatusSuccess {
				return false, itemHookControlError{wait: true}
			}
		}
		if id > 0 && record.ID == id {
			return true, nil
		}
		return false, itemHookControlError{wait: true}
	}
	var resourceSpace int64
	for _, evaluator := range etec.Expt.Evaluators {
		if evaluator != nil && evaluator.GetEvaluatorVersionID() == version {
			resourceSpace = resolveEvaluatorSpaceID(evaluator, etec.Event.SpaceID)
			break
		}
	}
	// Intercepted records belong to the consumer; normal shared execution uses the resource owner.
	if record == nil || id <= 0 || record.ID != id || resourceSpace <= 0 || (record.SpaceID != resourceSpace && record.SpaceID != etec.Event.SpaceID) || record.ExperimentID != current.ExptID || record.ExperimentRunID != current.ExptRunID || record.ItemID != current.ItemID || record.ItemVersionID != current.ItemVersionID || record.TurnID != current.TurnID || record.EvaluatorVersionID != version || record.Alias != alias || record.SourceType == entity.EvaluatorRecordSourceTypeInline {
		return false, itemHookControlError{wait: true}
	}
	return record.Status == entity.EvaluatorRunStatusSuccess || record.Status == entity.EvaluatorRunStatusAsyncInvoking, nil
}
