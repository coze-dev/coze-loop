// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"crypto/sha256"
	"errors"
	"reflect"
	"slices"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
	"gorm.io/hints"
)

type preparedHookDeletion struct {
	provider db.Provider
	ids      []int64
	spaceID  int64
	scope    string
	parents  map[int64]*preparedHookDeletionParent
}
type preparedHookDeletionParent struct {
	typ, source int32
	markers     map[int64]int32
	runs        map[int64]*preparedHookDeletionRun
}
type preparedHookDeletionRun struct {
	source     entity.HookExecutionInitializationSource
	keyID      string
	cipherHash [32]byte
	committed  bool
	binding    *boundHookExecution
}

// PrepareHookDeletion authenticates immutable original snapshots before the deletion transaction.
func PrepareHookDeletion(ctx context.Context, p db.Provider, runs repo.IHookRepo, codec hook.StorageCodec, ids []int64, spaceID int64, scope string) (repo.IHookDeletionRepo, error) {
	if ctx == nil || spaceID <= 0 || !hookGateASCII(scope) {
		return nil, entity.ErrHookStoreCorrupt
	}
	for _, v := range []any{p, runs, codec} {
		if batchDeletionNil(v) {
			return nil, entity.ErrHookStoreCorrupt
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ids, err := hookDeletionIDs(ids)
	if err != nil {
		return nil, err
	}
	out := &preparedHookDeletion{provider: p, ids: ids, spaceID: spaceID, scope: scope, parents: map[int64]*preparedHookDeletionParent{}}
	for _, id := range ids {
		var expt model.Experiment
		if err := p.NewSession(ctx, db.WithMaster()).Unscoped().Where("id=? AND space_id=?", id, spaceID).First(&expt).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, err
		}
		if expt.DeletedAt.Valid {
			continue
		}
		if expt.ID != id || expt.SpaceID != spaceID {
			return nil, entity.ErrHookStoreCorrupt
		}
		parent := &preparedHookDeletionParent{typ: expt.ExptType, source: expt.EvalSetSourceType, markers: map[int64]int32{}, runs: map[int64]*preparedHookDeletionRun{}}
		out.parents[id] = parent
		for cursor := int64(0); ; {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			var page []model.ExptRunLog
			if err := p.NewSession(ctx, db.WithMaster()).Unscoped().Select("expt_run_id", "lifecycle_hook_version").Clauses(hints.ForceIndex("uk_expt_run")).Where("space_id=? AND expt_id=? AND expt_run_id>?", spaceID, id, cursor).Order("expt_run_id").Limit(100).Find(&page).Error; err != nil {
				return nil, err
			}
			if len(page) > 100 {
				return nil, entity.ErrHookStoreCorrupt
			}
			for _, log := range page {
				if log.ExptRunID <= cursor {
					return nil, entity.ErrHookStoreCorrupt
				}
				cursor = log.ExptRunID
				marker := gptr.Indirect(log.LifecycleHookVersion)
				if marker != 0 && marker != 1 {
					return nil, entity.ErrHookStoreCorrupt
				}
				parent.markers[cursor] = marker
				if marker == 0 {
					continue
				}
				key := entity.HookRunKey{WorkspaceID: spaceID, ExperimentID: id, RunID: cursor}
				run, err := runs.GetRun(ctx, key)
				if err != nil {
					return nil, err
				}
				if run == nil || run.State.Key != key || run.Snapshot.ExecutionScope != scope || entity.ValidateHookStorageState(&run.State) != nil {
					return nil, entity.ErrHookStoreConflict
				}
				if run.State.Finalize == entity.HookFinalizeCommitted {
					// No new terminal work: preserve the old committed-Run deletion path.
					parent.runs[cursor] = hookDeletionRunMetadata(run)
					continue
				}
				// GetRun's read transaction has returned; the codec never sees a SQL transaction.
				decoded, err := codec.DecodeSnapshot(ctx, key, scope, run.Snapshot)
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				if err != nil {
					return nil, entity.ErrHookStoreCorrupt
				}
				entry, err := prepareHookDeletionRun(p, &expt, run, decoded)
				if err != nil {
					return nil, err
				}
				parent.runs[cursor] = entry
			}
			if len(page) < 100 {
				break
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func prepareHookDeletionRun(p db.Provider, expt *model.Experiment, run *entity.HookStoredRun, decoded *entity.HookRunSnapshot) (*preparedHookDeletionRun, error) {
	if decoded == nil {
		return nil, entity.ErrHookStoreCorrupt
	}
	snapshot, err := entity.NewHookRunSnapshot(decoded.Input())
	if err != nil {
		return nil, entity.ErrHookStoreCorrupt
	}
	in := snapshot.Input()
	before := in.Config.Before != nil && gptr.Indirect(in.Config.Before.Enabled)
	after := in.Config.After != nil && gptr.Indirect(in.Config.After.Enabled)
	modes := [...]string{"", "submit", "fail_retry", "append", "retry_all", "retry_items", "trial_run"}
	if run.Mode < 1 || int(run.Mode) >= len(modes) || in.Key != run.State.Key || in.ExecutionScope != run.Snapshot.ExecutionScope || in.Context.GetInitiator().GetUserID() != run.CreatedBy || in.Context.GetRunMode() != modes[run.Mode] || before != (run.State.Before.Status != entity.HookOperationDisabled) || after != (run.State.After.Status != entity.HookOperationDisabled) {
		return nil, entity.ErrHookStoreConflict
	}
	entry := hookDeletionRunMetadata(run)
	if in.Execution != nil {
		binding, err := entity.NewHookExecutionInitializationBinding(run, snapshot)
		if err != nil {
			return nil, err
		}
		single := expt.EvalSetSourceType == 0 || entity.ExptEvalSetSourceType(expt.EvalSetSourceType) == entity.ExptEvalSetSourceType_SingleSet
		online := run.Mode == entity.EvaluationModeAppend && entity.ExptType(expt.ExptType) == entity.ExptType_Online
		if (!online && (entity.ExptType(expt.ExptType) != entity.ExptType_Offline || in.Execution.SingleSet != single || !single && entity.ExptEvalSetSourceType(expt.EvalSetSourceType) != entity.ExptEvalSetSourceType_MultiSetConfig)) || online && !in.Execution.SingleSet || in.Execution.EvaluatorFallback == nil {
			return nil, entity.ErrHookExecutionUnsupported
		}
		bound, err := NewBoundHookExecutionInitializationRepo(p, binding)
		if err != nil {
			return nil, err
		}
		entry.binding = bound.(*hookRunRepo).executionBinding
	} else if in.Context.GetExperiment().GetType() != "offline" || !entity.HookExecutionInitializationRequired(before || after, run.Mode, entity.ExptType(expt.ExptType), entity.ExptEvalSetSourceType(expt.EvalSetSourceType)) {
		return nil, entity.ErrHookExecutionUnsupported
	}
	return entry, nil
}

func hookDeletionRunMetadata(run *entity.HookStoredRun) *preparedHookDeletionRun {
	return &preparedHookDeletionRun{source: entity.HookExecutionInitializationSource{Key: run.State.Key, ExecutionScope: run.Snapshot.ExecutionScope, SnapshotHash: run.Snapshot.Hash, CreatedBy: run.CreatedBy, Mode: run.Mode, BeforeEnabled: run.State.Before.Status != entity.HookOperationDisabled, AfterEnabled: run.State.After.Status != entity.HookOperationDisabled, SourceRunID: gptr.Indirect(run.SourceRunID)}, keyID: run.Snapshot.KeyID, cipherHash: sha256.Sum256(run.Snapshot.Cipher), committed: run.State.Finalize == entity.HookFinalizeCommitted}
}

func (p *preparedHookDeletionRun) check(run *lockedHookRun) error {
	v := p.source
	s := run.view
	if s.State.Key != v.Key || s.Snapshot.ExecutionScope != v.ExecutionScope || s.Snapshot.Hash != v.SnapshotHash || s.Snapshot.KeyID != p.keyID || sha256.Sum256(s.Snapshot.Cipher) != p.cipherHash || s.Mode != v.Mode || s.CreatedBy != v.CreatedBy || run.life.BeforeEnabled != v.BeforeEnabled || run.life.AfterEnabled != v.AfterEnabled || gptr.Indirect(s.SourceRunID) != v.SourceRunID || p.committed && s.State.Finalize != entity.HookFinalizeCommitted {
		return entity.ErrHookStoreConflict
	}
	return nil
}

func (p *preparedHookDeletion) HookDeletionScope() string { return p.scope }
func (p *preparedHookDeletion) DeleteExperiments(ctx context.Context, ids []int64, spaceID int64, scope string) ([]*entity.Experiment, error) {
	ids, err := hookDeletionIDs(ids)
	if err != nil {
		return nil, err
	}
	if p == nil || scope != p.scope || spaceID != p.spaceID || !slices.Equal(ids, p.ids) {
		return nil, entity.ErrHookStoreConflict
	}
	return deleteExperimentsWithHookBindings(ctx, p.provider, ids, spaceID, scope, nil, p)
}

func hookDeletionIDs(ids []int64) ([]int64, error) {
	ids = slices.Clone(ids)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	for _, id := range ids {
		if id <= 0 {
			return nil, entity.ErrHookStoreCorrupt
		}
	}
	return ids, nil
}
func batchDeletionNil(v any) bool {
	if v == nil {
		return true
	}
	r := reflect.ValueOf(v)
	switch r.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return r.IsNil()
	}
	return false
}
