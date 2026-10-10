// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	hook "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
)

type ScheduledTemplateResources struct {
	Permissions []rpc.ScheduledRunAuthorizationResource
	Create      *entity.CreateExptParam
	resolved    *exptCreationResources
	leaseOwner  string
}

type exptCreationResources struct {
	tuple                         *entity.ExptTuple
	versionedTargetID             *entity.VersionedTargetID
	evalSetSpaceID, targetSpaceID int64
	evalSetAccessLevel            string
}

type preparedExptCreation struct {
	experiment *entity.Experiment
	stats      *entity.ExptStats
	mappings   []*entity.ExptTurnResultFilterKeyMapping
}

func (e *ExptMangerImpl) prepareScheduledCreation(ctx context.Context, req *entity.CreateExptParam, session *entity.Session, resources *exptCreationResources, ids []int64) (*preparedExptCreation, error) {
	return e.prepareExptCreation(ctx, req, session, resources, ids, false)
}

// IScheduledTemplatePreparation keeps external preparation and publication outside Commit.
type IScheduledTemplatePreparation interface {
	Resolve(context.Context, *repo.ExptTemplateScheduleState) (*ScheduledTemplateResources, error)
	Prepare(context.Context, entity.ScheduledRunTrigger, *repo.ExptTemplateScheduleState, *ScheduledTemplateResources) (*entity.PreparedScheduledExpt, error)
	Publish(context.Context, entity.ScheduledRunTrigger) error
	Abort(context.Context, entity.ScheduledRunTrigger, *ScheduledTemplateResources)
}

type ScheduledTemplateConverter func(context.Context, *entity.ExptTemplate) (*entity.CreateExptParam, error)
type scheduledTemplatePreparation struct {
	manager   *ExptMangerImpl
	convert   ScheduledTemplateConverter
	preflight func(context.Context, *entity.Experiment) error
}

func NewScheduledTemplatePreparation(manager IExptManager, convert ScheduledTemplateConverter, preflight ...func(context.Context, *entity.Experiment) error) (IScheduledTemplatePreparation, error) {
	base, ok := manager.(*ExptMangerImpl)
	if !ok {
		if router, yes := manager.(interface{ HookConfigBaseManager() *ExptMangerImpl }); yes {
			base = router.HookConfigBaseManager()
		}
	}
	if base == nil || base.hooks == nil || convert == nil {
		return nil, entity.ErrHookConfigStorage
	}
	for _, dep := range []any{base.idgenerator, base.configer, base.mutex, base.exptRepo, base.evaluationSetService, base.evaluationSetVersionService, base.evalTargetService, base.evaluatorService, base.audit, base.benefitService, base.quotaRepo, base.publisher, base.hooks.Codec, base.hooks.Runtime, base.hooks.Initialization, base.hooks.Runs, base.hooks.Wake} {
		if missingManagerHookDependency(dep) {
			return nil, entity.ErrHookConfigStorage
		}
	}
	if len(preflight) > 1 {
		return nil, entity.ErrHookConfigStorage
	}
	p := &scheduledTemplatePreparation{manager: base, convert: convert}
	if len(preflight) == 1 {
		p.preflight = preflight[0]
	}
	return p, nil
}

func (p *scheduledTemplatePreparation) Resolve(ctx context.Context, state *repo.ExptTemplateScheduleState) (*ScheduledTemplateResources, error) {
	if ctx == nil || state == nil || state.Template == nil || !state.Binding.Active() {
		return nil, entity.ErrHookStoreConflict
	}
	m, b := p.manager, state.Binding
	req, err := p.convert(ctx, state.Template)
	if err != nil {
		return nil, err
	}
	if req != nil {
		if err := req.PrepareVerificationTarget(); err != nil {
			return nil, err
		}
	}
	if req == nil || req.WorkspaceID != b.SpaceID || req.ExptTemplateID != b.TemplateID || !req.CreateEvalTargetParam.IsNull() {
		return nil, entity.ErrHookStoreConflict
	}
	if req.EvalSetID > 0 {
		set, err := m.evaluationSetService.GetEvaluationSet(ctx, nil, req.EvalSetID, gptr.Of(false), nil)
		if err != nil {
			return nil, err
		}
		if set == nil || set.ID != req.EvalSetID || set.SpaceID <= 0 {
			return nil, entity.ErrHookStoreConflict
		}
		if set.SpaceID != b.SpaceID {
			if missingManagerHookDependency(m.resourceAccessAuthorizer) {
				return nil, rpc.ErrScheduledRunAuthorizationUnavailable
			}
			req.EvalSetSharedOption = &entity.SharedResourceOption{IsShared: true, SourceSpaceID: gptr.Of(set.SpaceID)}
		}
	}
	if id := gptr.Indirect(req.TargetID); id > 0 {
		target, err := m.evalTargetService.GetEvalTarget(ctx, id)
		if err != nil {
			return nil, err
		}
		if target == nil || target.ID != id || target.SpaceID <= 0 {
			return nil, entity.ErrHookStoreConflict
		}
		if target.SpaceID != b.SpaceID {
			if missingManagerHookDependency(m.resourceAccessAuthorizer) {
				return nil, rpc.ErrScheduledRunAuthorizationUnavailable
			}
			req.TargetSharedOption = &entity.SharedResourceOption{IsShared: true, SourceSpaceID: gptr.Of(target.SpaceID)}
		}
	}
	// No delegated Session exists while resolving resources and source grants.
	resolved, err := m.resolveExptCreation(ctx, req, nil)
	if err != nil {
		return nil, err
	}
	permissions := make([]rpc.ScheduledRunAuthorizationResource, 0, 2)
	if set := resolved.tuple.EvalSet; req.EvalSetID > 0 {
		if set == nil || set.ID != req.EvalSetID || set.SpaceID != resolved.evalSetSpaceID {
			return nil, entity.ErrHookStoreConflict
		}
		if req.EvalSetVersionID != req.EvalSetID && (set.EvaluationSetVersion == nil || set.EvaluationSetVersion.ID != req.EvalSetVersionID || set.EvaluationSetVersion.EvaluationSetID != set.ID || set.EvaluationSetVersion.SpaceID != set.SpaceID) {
			return nil, entity.ErrHookStoreConflict
		}
		permissions = append(permissions, rpc.ScheduledRunAuthorizationResource{ObjectID: strconv.FormatInt(set.ID, 10), SpaceID: set.SpaceID, EntityType: rpc.AuthEntityType_EvaluationSet, Action: consts.Read})
	}
	if target := resolved.tuple.Target; gptr.Indirect(req.TargetID) > 0 {
		if target == nil || target.ID != gptr.Indirect(req.TargetID) || target.SpaceID != resolved.targetSpaceID {
			return nil, entity.ErrHookStoreConflict
		}
		if target.EvalTargetVersion == nil || target.EvalTargetVersion.ID != req.TargetVersionID || target.EvalTargetVersion.TargetID != target.ID || target.EvalTargetVersion.SpaceID != target.SpaceID {
			return nil, entity.ErrHookStoreConflict
		}
		permissions = append(permissions, rpc.ScheduledRunAuthorizationResource{ObjectID: strconv.FormatInt(target.ID, 10), SpaceID: target.SpaceID, EntityType: rpc.AuthEntityType_EvaluationTarget, Action: consts.Run})
	}
	versions := map[int64]bool{}
	for _, ev := range resolved.tuple.Evaluators {
		if ev == nil {
			return nil, entity.ErrHookStoreConflict
		}
		versions[ev.GetEvaluatorVersionID()] = true
	}
	for _, id := range req.EvaluatorVersionIds {
		if !versions[id] {
			return nil, entity.ErrHookStoreConflict
		}
	}
	return &ScheduledTemplateResources{Permissions: permissions, Create: req, resolved: resolved}, nil
}
func (p *scheduledTemplatePreparation) Prepare(ctx context.Context, tr entity.ScheduledRunTrigger, state *repo.ExptTemplateScheduleState, resources *ScheduledTemplateResources) (*entity.PreparedScheduledExpt, error) {
	if ctx == nil || ctx.Err() != nil || state == nil || !state.Binding.Active() || state.Config == nil || resources == nil || resources.Create == nil || resources.resolved == nil || tr.Validate() != nil {
		return nil, entity.ErrHookStoreConflict
	}
	m, b := p.manager, state.Binding
	if b.ExecutionScope != m.hooks.ExecutionScope || tr.BindingID != b.BindingID || tr.BindingVersion != b.Version || tr.SpaceID != b.SpaceID || tr.TemplateID != b.TemplateID || tr.Status != entity.ScheduledRunTriggerPending {
		return nil, entity.ErrHookStoreConflict
	}
	runtime, err := m.hooks.Runtime.GetRuntimeConfig(ctx)
	if err != nil || !runtime.AllowsWorkspace(tr.SpaceID) {
		return nil, entity.ErrHookAdmissionDenied
	}
	config, err := entity.ResolveLifecycleHookConf(nil, state.Config.Config)
	if err != nil || config == nil || !managerHookEnabled(config.Before) && !managerHookEnabled(config.After) {
		return nil, entity.ErrHookConfigStorage
	}
	ids, err := m.idgenerator.GenMultiIDs(ctx, 1)
	if err != nil {
		return nil, err
	}
	if len(ids) != 1 || ids[0] <= 0 {
		return nil, entity.ErrHookStoreConflict
	}
	request := *resources.Create
	name := []rune(state.Template.GetName())
	if len(name) > 80 {
		name = name[:80]
	}
	request.Name = fmt.Sprintf("%s_%d", string(name), tr.ExperimentID)
	request.TriggerType = "schedule"
	session := &entity.Session{UserID: b.UserID}
	created, err := m.prepareScheduledCreation(ctx, &request, session, resources.resolved, []int64{tr.ExperimentID, ids[0]})
	if err != nil {
		return nil, err
	}
	valid, err := m.CheckName(contexts.WithCtxWriteDB(ctx), created.experiment.Name, b.SpaceID, session)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, entity.ErrHookStoreConflict
	}
	refs := created.experiment.ToEvaluatorRefDO()
	refIDs, err := hookConfigCreateRefIDs(ctx, m.idgenerator, len(refs))
	if err != nil {
		return nil, err
	}
	for i := range refs {
		refs[i].ID = refIDs[i]
	}
	key := entity.HookRunKey{WorkspaceID: b.SpaceID, ExperimentID: tr.ExperimentID, RunID: tr.RunID}
	mode := entity.EvaluationModeSubmit
	if created.experiment.ExptType == entity.ExptType_Online {
		mode = entity.EvaluationModeAppend
	}
	log := &entity.ExptRunLog{ID: tr.RunID, SpaceID: b.SpaceID, ExptID: tr.ExperimentID, ExptRunID: tr.RunID, CreatedBy: b.UserID, Mode: int32(mode), Status: int64(entity.ExptStatus_Pending)}
	seed := &entity.HookScheduleSeed{Version: 1, Key: key, ExecutionScope: b.ExecutionScope, Mode: mode, CreatedAt: time.Now().Unix(), ItemRetryTimes: gptr.Indirect(request.ExptConf.ItemRetryNum), Session: session, Ext: withRetryYieldExt(nil, m.configer.GetRetryYieldEnabled(ctx, b.SpaceID))}
	initiator := &spi.HookInitiator{UserID: gptr.Of(b.UserID), IdentityType: gptr.Of(b.IdentityType)}
	run, err := m.prepareHookRun(ctx, created.experiment, log, config, initiator, nil, seed)
	if err != nil {
		return nil, err
	}
	cipher, err := m.hooks.Codec.EncodeConfig(ctx, m.hooks.SnapshotKeyID, hook.ConfigOwner{WorkspaceID: b.SpaceID, ObjectID: tr.ExperimentID, Kind: hook.ConfigOwnerExperiment, ExecutionScope: b.ExecutionScope}, config)
	if err != nil {
		return nil, err
	}
	if p.preflight != nil {
		if err := p.preflight(ctx, created.experiment); err != nil {
			return nil, err
		}
	} else if created.experiment.IsSandboxAgentTarget() {
		return nil, entity.ErrHookConfigStorage
	}
	owner, err := newManagerHookLockOwner(tr.RunID)
	if err != nil {
		return nil, err
	}
	ttl := time.Duration(m.configer.GetExptExecConf(ctx, b.SpaceID).GetZombieIntervalSecond()) * time.Second
	locked, _, err := m.mutex.BackoffLockWithValue(ctx, m.makeExptMutexLockKey(tr.ExperimentID), owner, ttl, time.Second)
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, entity.ErrHookStoreConflict
	}
	resources.leaseOwner = owner
	return &entity.PreparedScheduledExpt{Binding: *b, TemplateRevision: state.Revision, Experiment: created.experiment, Stats: created.stats, Refs: refs, Mappings: created.mappings, ConfigCipher: cipher, Run: run}, nil
}
func (p *scheduledTemplatePreparation) Publish(ctx context.Context, tr entity.ScheduledRunTrigger) error {
	key := entity.HookRunKey{WorkspaceID: tr.SpaceID, ExperimentID: tr.ExperimentID, RunID: tr.RunID}
	run, event, err := p.manager.readHookSchedule(ctx, key)
	if err != nil {
		return err
	}
	p.manager.wakeHookRun(ctx, run)
	return p.manager.PublishHookSchedule(ctx, event)
}

func (p *scheduledTemplatePreparation) Abort(ctx context.Context, tr entity.ScheduledRunTrigger, resources *ScheduledTemplateResources) {
	if resources == nil || resources.leaseOwner == "" {
		return
	}
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	key := entity.HookRunKey{WorkspaceID: tr.SpaceID, ExperimentID: tr.ExperimentID, RunID: tr.RunID}
	initial, err := p.manager.hooks.Initialization.ReadRunInitialization(cleanup, key)
	if err != nil && !errors.Is(err, entity.ErrHookStoreMissing) {
		return
	}
	if initial != nil && (initial.Managed || initial.RunLog != nil || initial.LatestRunID == tr.RunID) {
		return
	}
	_, _ = p.manager.mutex.UnlockWithValue(cleanup, p.manager.makeExptMutexLockKey(tr.ExperimentID), resources.leaseOwner)
}
