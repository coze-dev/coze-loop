// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ExptManagerHookDependencies struct {
	Initialization repo.IHookRunInitializationRepo
	Runs           repo.IHookRepo
	Configs        repo.IHookConfigRepo
	Codec          hookcomponent.StorageCodec
	Identity       hookcomponent.IdentityProvider
	Runtime        hookcomponent.RuntimeConfigProvider
	Wake           hookcomponent.WakePublisher
	// Trusted deployment configuration, never supplied by the business request.
	ExecutionScope, SnapshotKeyID string
}

// NewExptManagerWithHooks explicitly opts the existing Manager into atomic Run
// initialization. The legacy constructor and exported IExptManager stay unchanged.
func NewExptManagerWithHooks(base IExptManager, deps ExptManagerHookDependencies) (IExptManager, error) {
	manager, ok := base.(*ExptMangerImpl)
	if !ok || manager == nil || manager.hooks != nil {
		return nil, errors.New("invalid hook-aware experiment manager")
	}
	for _, value := range []any{deps.Initialization, deps.Runs, deps.Configs, deps.Codec, deps.Identity, deps.Runtime, deps.Wake, manager.idgenerator, manager.exptRepo, manager.evaluationSetVersionService} {
		if missingManagerHookDependency(value) {
			return nil, errors.New("missing hook initialization dependency")
		}
	}
	for _, value := range []string{deps.ExecutionScope, deps.SnapshotKeyID} {
		if value == "" || len(value) > 128 {
			return nil, errors.New("invalid hook initialization configuration")
		}
		for _, c := range value {
			if c < 33 || c > 126 {
				return nil, errors.New("invalid hook initialization configuration")
			}
		}
	}
	copy := *manager
	copy.hooks = &deps
	return &copy, nil
}

func missingManagerHookDependency(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	}
	return false
}

func (e *ExptMangerImpl) readHookRunInitialization(ctx context.Context, key entity.HookRunKey) (*entity.HookRunInitialization, error) {
	initial, err := e.hooks.Initialization.ReadRunInitialization(contexts.WithCtxWriteDB(ctx), key)
	if err != nil {
		return nil, managerHookError(err)
	}
	if initial == nil || initial.RunLog == nil && initial.Managed {
		return nil, entity.ErrHookStoreCorrupt
	}
	return initial, nil
}

func newManagerHookLockOwner(runID int64) (string, error) {
	if runID <= 0 {
		return "", entity.ErrHookStoreConflict
	}
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", errors.New("HOOK_LOCK_OWNER_UNAVAILABLE")
	}
	return fmt.Sprintf("hook_run:%d:%x", runID, token), nil
}

func managerHookLockRunID(owner string) (int64, error) {
	if strings.HasPrefix(owner, "hook_run:") {
		parts := strings.Split(owner, ":")
		if len(parts) != 3 || len(parts[2]) != 32 {
			return 0, entity.ErrHookStoreConflict
		}
		if _, err := hex.DecodeString(parts[2]); err != nil {
			return 0, entity.ErrHookStoreConflict
		}
		owner = parts[1]
	}
	id, err := strconv.ParseInt(owner, 10, 64)
	if err != nil || id <= 0 {
		return 0, entity.ErrHookStoreConflict
	}
	return id, nil
}

func (e *ExptMangerImpl) logHookRun(ctx context.Context, exptID, runID int64, mode entity.ExptRunMode, spaceID int64, itemIDs []int64, session *entity.Session, rawItemIDs *string, schedule ...*entity.HookScheduleSeed) error {
	if ctx == nil || session == nil {
		return errors.New("HOOK_IDENTITY_INVALID")
	}
	key := entity.HookRunKey{WorkspaceID: spaceID, ExperimentID: exptID, RunID: runID}
	initial, err := e.readHookRunInitialization(ctx, key)
	if err != nil {
		return err
	}
	return e.logHookRunFromInitialization(ctx, exptID, runID, mode, spaceID, itemIDs, session, rawItemIDs, initial, schedule...)
}

func (e *ExptMangerImpl) logHookRunFromInitialization(ctx context.Context, exptID, runID int64, mode entity.ExptRunMode, spaceID int64, itemIDs []int64, session *entity.Session, rawItemIDs *string, initial *entity.HookRunInitialization, schedule ...*entity.HookScheduleSeed) error {
	log := &entity.ExptRunLog{ID: runID, SpaceID: spaceID, CreatedBy: session.UserID, ExptID: exptID, ExptRunID: runID, Mode: int32(mode), Status: int64(entity.ExptStatus_Pending)}
	if len(itemIDs) > 0 {
		log.ItemIds = []entity.ExptRunLogItems{{ItemIDs: slices.Clone(itemIDs), CreateAt: gptr.Of(time.Now().Unix())}}
	}
	// A replay does not acquire or replace the current Run owner's lease.
	if initial.RunLog != nil {
		attempted := false
		return e.initializeHookRun(ctx, log, initial, &attempted, rawItemIDs, schedule...)
	}
	owner, err := newManagerHookLockOwner(runID)
	if err != nil {
		return err
	}
	ttl := time.Duration(e.configer.GetExptExecConf(ctx, spaceID).GetZombieIntervalSecond()) * time.Second
	locked, _, err := e.mutex.BackoffLockWithValue(ctx, e.makeExptMutexLockKey(exptID), owner, ttl, time.Second)
	if err != nil {
		return err
	}
	if !locked {
		return errorx.NewByCode(errno.ExperimentRunningExistedCode)
	}
	defer e.mtr.EmitExptExecRun(spaceID, int64(mode))
	return e.initializeOwnedHookRun(ctx, log, initial, owner, rawItemIDs, schedule...)
}

func (e *ExptMangerImpl) initializeOwnedHookRun(ctx context.Context, log *entity.ExptRunLog, initial *entity.HookRunInitialization, owner string, rawItemIDs *string, schedule ...*entity.HookScheduleSeed) error {
	attempted := false
	err := e.initializeHookRun(ctx, log, initial, &attempted, rawItemIDs, schedule...)
	if err == nil || initial.RunLog != nil {
		return err
	}
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if attempted {
		// The repository error alone cannot distinguish rollback from a lost commit receipt.
		key := entity.HookRunKey{WorkspaceID: log.SpaceID, ExperimentID: log.ExptID, RunID: log.ExptRunID}
		confirmed, readErr := e.readHookRunInitialization(cleanup, key)
		if readErr != nil || confirmed.RunLog != nil || confirmed.Managed || confirmed.LatestRunID == key.RunID {
			logs.CtxWarn(cleanup, "Hook initialization lease retained: Run commit outcome is not confirmed absent")
			return err
		}
	}
	// Compare-delete fences an expired/late caller from releasing a successor's lease.
	if _, unlockErr := e.mutex.UnlockWithValue(cleanup, e.makeExptMutexLockKey(log.ExptID), owner); unlockErr != nil {
		logs.CtxWarn(cleanup, "Hook initialization owner lease could not be released")
	}
	return err
}

func (e *ExptMangerImpl) initializeHookRun(ctx context.Context, log *entity.ExptRunLog, init *entity.HookRunInitialization, attempted *bool, rawItemIDs *string, schedule ...*entity.HookScheduleSeed) error {
	ctx = contexts.WithCtxWriteDB(ctx)
	h := e.hooks
	key := entity.HookRunKey{WorkspaceID: log.SpaceID, ExperimentID: log.ExptID, RunID: log.ExptRunID}
	var err error
	if init == nil {
		return entity.ErrHookStoreCorrupt
	}
	if err := validateLegacyHookInitialization(ctx, key, init); err != nil {
		return err
	}
	if old := init.RunLog; old != nil {
		if old.SpaceID != log.SpaceID || old.ExptID != log.ExptID || old.ExptRunID != log.ExptRunID || old.CreatedBy != log.CreatedBy || old.Mode != log.Mode {
			return entity.ErrHookStoreConflict
		}
		if rawItemIDs == nil && !slices.Equal(old.GetItemIDs(), log.GetItemIDs()) {
			return entity.ErrHookStoreConflict
		}
		if init.Managed {
			stored, err := h.Runs.GetRun(ctx, key)
			if err != nil {
				return managerHookError(err)
			}
			if stored == nil || stored.State.Key != key || stored.CreatedBy != old.CreatedBy || stored.Mode != entity.ExptRunMode(old.Mode) || stored.Snapshot.ExecutionScope != h.ExecutionScope {
				return entity.ErrHookStoreCorrupt
			}
			if err := e.restoreHookSelectionForReplay(ctx, log, stored, rawItemIDs); err != nil {
				return err
			}
			if len(schedule) > 0 {
				original, err := h.Codec.DecodeSnapshot(ctx, key, h.ExecutionScope, stored.Snapshot)
				if err != nil || original == nil || !reflect.DeepEqual(original.Input().Schedule, schedule[0]) {
					return entity.ErrHookStoreConflict
				}
			}
			if !slices.Equal(old.GetItemIDs(), log.GetItemIDs()) {
				return entity.ErrHookStoreConflict
			}
			e.wakeHookRun(ctx, stored)
		} else if !slices.Equal(old.GetItemIDs(), log.GetItemIDs()) {
			return entity.ErrHookStoreConflict
		}
		return nil
	}
	var expt *entity.Experiment
	if init.HooksEnabled || (isRetryRunMode(entity.ExptRunMode(log.Mode)) && e.centralGuard != nil && init.LatestRunID > 0) {
		expt, err = e.exptRepo.GetByID(ctx, key.ExperimentID, key.WorkspaceID)
		if err != nil && init.HooksEnabled {
			return managerHookError(err)
		}
		if err != nil || expt == nil || expt.ID != key.ExperimentID || expt.SpaceID != key.WorkspaceID || expt.LatestRunID != init.LatestRunID {
			if init.HooksEnabled {
				return entity.ErrHookStoreConflict
			}
			// Legacy quota cleanup is best effort; the creation CAS still fences Latest.
			expt = nil
			logs.CtxWarn(ctx, "Hook-aware legacy initialization could not load superseded quota metadata; recovery must reconcile it")
		}
	}
	if !init.HooksEnabled {
		if len(schedule) > 0 {
			return entity.ErrHookStoreConflict
		}
		*attempted = true
		changed, err := h.Initialization.CreateRunWithoutHooks(ctx, log, init.LatestRunID, init.ConfigRevision)
		if err != nil {
			return managerHookError(err)
		}
		if changed {
			e.releaseInitializedRunQuota(ctx, expt, init.LatestRunID, log)
		}
		return nil
	}
	config, err := h.Configs.GetConfig(ctx, hookcomponent.ConfigOwner{WorkspaceID: key.WorkspaceID, ObjectID: key.ExperimentID, Kind: hookcomponent.ConfigOwnerExperiment, ExecutionScope: h.ExecutionScope})
	if err != nil {
		return managerHookError(err)
	}
	if config == nil || config.Revision != init.ConfigRevision {
		return entity.ErrHookStoreConflict
	}
	conf, err := entity.ResolveLifecycleHookConf(nil, config.Config)
	if err != nil || conf == nil || !managerHookEnabled(conf.Before) && !managerHookEnabled(conf.After) {
		return entity.ErrHookConfigStorage
	}
	runtime, err := h.Runtime.GetRuntimeConfig(ctx)
	if err != nil || !runtime.AdmissionEnabled {
		return errors.New("HOOK_ADMISSION_UNAVAILABLE")
	}
	initiator, err := h.Identity.ResolveInitiator(ctx, log.CreatedBy)
	if err != nil || initiator == nil || initiator.GetUserID() != log.CreatedBy || initiator.GetIdentityType() != "fornax_user" {
		return errors.New("HOOK_IDENTITY_INVALID")
	}
	in, err := e.prepareHookRun(ctx, expt, log, conf, initiator, rawItemIDs, schedule...)
	if err != nil {
		return err
	}
	in.ExpectedLatestRunID, in.ExpectedConfigRevision = init.LatestRunID, init.ConfigRevision
	if isRetryRunMode(entity.ExptRunMode(log.Mode)) && init.LatestRunID > 0 {
		in.SourceRunID = gptr.Of(init.LatestRunID)
	}
	*attempted = true
	result, err := h.Runs.CreateRunWithHooks(ctx, in)
	if err != nil {
		return managerHookError(err)
	}
	if result.Changed {
		e.releaseInitializedRunQuota(ctx, expt, init.LatestRunID, log)
	}
	e.wakeHookRun(ctx, result.Run)
	return nil
}

func (e *ExptMangerImpl) prepareHookRun(ctx context.Context, expt *entity.Experiment, log *entity.ExptRunLog, conf *entity.LifecycleHookConf, initiator *spi.HookInitiator, rawItemIDs *string, schedule ...*entity.HookScheduleSeed) (entity.HookCreateRunInput, error) {
	h := e.hooks
	key := entity.HookRunKey{WorkspaceID: log.SpaceID, ExperimentID: log.ExptID, RunID: log.ExptRunID}
	runContext, err := e.hookRunContext(ctx, expt, log, initiator)
	if err != nil {
		return entity.HookCreateRunInput{}, managerHookError(err)
	}
	execution, err := newHookExecutionSnapshot(expt, log, h.ExecutionScope)
	if err != nil {
		return entity.HookCreateRunInput{}, err
	}
	var selection *entity.HookSelectionSeed
	if execution != nil || managerHookEnabled(conf.Before) || entity.HookExecutionInitializationRequired(managerHookEnabled(conf.After), entity.ExptRunMode(log.Mode), expt.ExptType, expt.EvalSetSourceType) {
		selection, err = newHookSelectionSeed(expt, log, rawItemIDs)
		if err != nil {
			return entity.HookCreateRunInput{}, err
		}
	}
	var scheduleSeed *entity.HookScheduleSeed
	if len(schedule) > 0 {
		scheduleSeed = schedule[0]
	}
	snapshot, err := entity.NewHookRunSnapshot(entity.HookRunSnapshotInput{Key: key, ExecutionScope: h.ExecutionScope, CreatedAt: time.Now(), Config: conf, Context: runContext, Selection: selection, Execution: execution, Schedule: scheduleSeed})
	if err != nil {
		return entity.HookCreateRunInput{}, errors.New("HOOK_SNAPSHOT_INVALID")
	}
	protected, err := h.Codec.EncodeSnapshot(ctx, h.SnapshotKeyID, snapshot)
	if err != nil {
		return entity.HookCreateRunInput{}, errors.New("HOOK_SNAPSHOT_INVALID")
	}
	in := entity.HookCreateRunInput{Key: key, RunLog: log, Snapshot: protected, ExpectedLatestRunID: 0}
	if managerHookEnabled(conf.Before) {
		in.Before, err = e.newHookOperation(ctx)
		if err != nil {
			return entity.HookCreateRunInput{}, err
		}
	}
	if managerHookEnabled(conf.After) {
		in.After, err = e.newHookOperation(ctx)
		if err != nil {
			return entity.HookCreateRunInput{}, err
		}
	}
	return in, nil
}

func managerHookEnabled(c *entity.HookConfig) bool { return c != nil && c.Enabled != nil && *c.Enabled }

func (e *ExptMangerImpl) newHookOperation(ctx context.Context) (*entity.HookOperationSeed, error) {
	id, err := e.idgenerator.GenID(ctx)
	if err != nil || id <= 0 {
		return nil, errors.New("HOOK_ID_ALLOCATION_FAILED")
	}
	return &entity.HookOperationSeed{ID: id, OperationID: fmt.Sprintf("hook_%d", id), IdempotencyKey: fmt.Sprintf("hook_key_%d", id)}, nil
}

func (e *ExptMangerImpl) wakeHookRun(ctx context.Context, run *entity.HookStoredRun) {
	if run == nil {
		return
	}
	for _, op := range run.Operations {
		if err := e.hooks.Wake.PublishWake(ctx, entity.HookWakeEvent{Run: run.State.Key, OperationID: op.OperationID, ExecutionScope: run.Snapshot.ExecutionScope}); err != nil {
			logs.CtxWarn(ctx, "Hook initialization wake failed; committed work remains recoverable by database scan")
		}
	}
}

func (e *ExptMangerImpl) releaseInitializedRunQuota(ctx context.Context, expt *entity.Experiment, oldRunID int64, log *entity.ExptRunLog) {
	if oldRunID > 0 && oldRunID != log.ExptRunID && isRetryRunMode(entity.ExptRunMode(log.Mode)) {
		e.releaseCentralQuotaForRun(ctx, expt, gptr.Of(oldRunID), fmt.Sprintf("superseded by retry run=%d", log.ExptRunID), "retry")
	}
}

func (e *ExptMangerImpl) hookRunContext(ctx context.Context, expt *entity.Experiment, log *entity.ExptRunLog, initiator *spi.HookInitiator) (*spi.HookRunContext, error) {
	if expt.TargetID < 0 || expt.TargetVersionID < 0 || expt.TargetID == 0 && expt.TargetVersionID != 0 || expt.TargetID > 0 && expt.TargetType.String() == "<UNSET>" {
		return nil, entity.ErrHookStoreCorrupt
	}
	if expt.EvalSetID < 0 || expt.EvalSetVersionID < 0 {
		return nil, entity.ErrHookStoreCorrupt
	}
	modes := map[entity.ExptRunMode]spi.HookRunMode{entity.EvaluationModeSubmit: spi.HookRunModeSubmit, entity.EvaluationModeFailRetry: spi.HookRunModeFailRetry, entity.EvaluationModeAppend: spi.HookRunModeAppend, entity.EvaluationModeRetryAll: spi.HookRunModeRetryAll, entity.EvaluationModeRetryItems: spi.HookRunModeRetryItems, entity.EvaluationModeTrialRun: spi.HookRunModeTrialRun}
	mode, ok := modes[entity.ExptRunMode(log.Mode)]
	if !ok {
		return nil, entity.ErrHookStoreConflict
	}
	kind := "offline"
	if expt.ExptType == entity.ExptType_Online {
		kind = "online"
	} else if expt.ExptType != entity.ExptType_Offline {
		return nil, entity.ErrHookStoreConflict
	}
	out := &spi.HookRunContext{WorkspaceID: gptr.Of(strconv.FormatInt(log.SpaceID, 10)), ExperimentID: gptr.Of(strconv.FormatInt(log.ExptID, 10)), RunID: gptr.Of(strconv.FormatInt(log.ExptRunID, 10)), RunMode: &mode, Initiator: initiator, Experiment: &spi.HookExperimentRef{Name: gptr.Of(expt.Name), Type: &kind}, EvalSets: make([]*spi.HookEvalSetRef, 0)}
	sets := []*entity.EvalSetConfig(nil)
	if expt.EvalConf != nil {
		sets = expt.EvalConf.EvalSetConfigs
	}
	if len(sets) == 0 && expt.EvalSetID == 0 && expt.EvalSetVersionID != 0 {
		return nil, entity.ErrHookStoreCorrupt
	}
	if len(sets) == 0 && expt.EvalSetID > 0 {
		sets = []*entity.EvalSetConfig{{EvalSetID: expt.EvalSetID, EvalSetVersionID: expt.EvalSetVersionID, SourceSpaceID: expt.EvalSetSpaceID}}
	}
	for _, set := range sets {
		if set == nil || set.EvalSetID <= 0 || set.EvalSetVersionID < 0 || set.SourceSpaceID < 0 {
			return nil, entity.ErrHookStoreCorrupt
		}
		space := set.SourceSpaceID
		if space == 0 {
			space = expt.SpaceID
		}
		ref := &spi.HookEvalSetRef{WorkspaceID: gptr.Of(strconv.FormatInt(space, 10)), ID: gptr.Of(strconv.FormatInt(set.EvalSetID, 10))}
		if !isDraftEvalSet(set.EvalSetID, set.EvalSetVersionID) {
			version, _, err := e.evaluationSetVersionService.GetEvaluationSetVersion(ctx, space, set.EvalSetVersionID, gptr.Of(true), nil)
			if err != nil || version == nil || version.ID != set.EvalSetVersionID || version.EvaluationSetID != set.EvalSetID || version.SpaceID != space {
				return nil, entity.ErrHookStoreCorrupt
			}
			ref.VersionID = gptr.Of(strconv.FormatInt(version.ID, 10))
			if version.Version != "" {
				ref.Version = gptr.Of(version.Version)
			}
		}
		out.EvalSets = append(out.EvalSets, ref)
	}
	if expt.TargetID > 0 {
		out.Target = &spi.HookTargetRef{ID: gptr.Of(strconv.FormatInt(expt.TargetID, 10)), Type: gptr.Of(expt.TargetType.String())}
		if expt.TargetVersionID > 0 {
			out.Target.VersionID = gptr.Of(strconv.FormatInt(expt.TargetVersionID, 10))
		}
	}
	return out, nil
}

func managerHookError(err error) error {
	for _, safe := range []error{entity.ErrHookStoreConflict, entity.ErrHookStoreMissing, entity.ErrHookStoreCorrupt, entity.ErrHookConfigStorage, entity.ErrHookAdmissionDenied} {
		if errors.Is(err, safe) {
			return safe
		}
	}
	return errors.New("HOOK_INITIALIZATION_FAILED")
}
