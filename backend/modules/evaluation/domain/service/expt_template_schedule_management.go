// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bytedance/gg/gptr"
	sessions "github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"math"
	"strconv"
	"time"
)

type ExptTemplateScheduleDependencies struct {
	Store                   repo.IExptTemplateScheduleStore
	Bindings                repo.IExptTemplateScheduleBindingRepo
	Receipts                rpc.IExptScheduleReceiptAdapter
	Scope, Namespace, Group string
	Callback                entity.ExptTemplateScheduleCallback
}

var ErrExptTemplateSchedulePending = errors.New("template saved; schedule registration pending")

type ExptTemplateSchedulePendingError struct {
	TemplateID, BindingVersion int64
	cause                      error
}

func (e *ExptTemplateSchedulePendingError) Error() string {
	return ErrExptTemplateSchedulePending.Error()
}
func (e *ExptTemplateSchedulePendingError) Is(target error) bool {
	return target == ErrExptTemplateSchedulePending
}
func (e *ExptTemplateSchedulePendingError) Unwrap() error {
	if e.cause != nil {
		return e.cause
	}
	return ErrExptTemplateSchedulePending
}

// Install after management authorization; routing is deployment-owned, not request data.
func WithExptTemplateScheduleManagement(manager IExptTemplateManager, deps ExptTemplateScheduleDependencies) (IExptTemplateManager, error) {
	base, ok := manager.(*ExptTemplateManagerImpl)
	if !ok || base == nil || missingManagerHookDependency(deps.Store) || missingManagerHookDependency(deps.Bindings) || missingManagerHookDependency(deps.Receipts) || missingManagerHookDependency(base.scheduleAdapter) || deps.Callback.Validate() != nil || deps.Callback.Method != "SubmitScheduledExptFromTemplate" {
		return nil, entity.ErrExptTemplateScheduleBindingInvalid
	}
	for _, text := range []string{deps.Scope, deps.Namespace, deps.Group} {
		if len(text) == 0 || len(text) > 128 {
			return nil, entity.ErrExptTemplateScheduleBindingInvalid
		}
		for _, c := range text {
			if c < 33 || c > 126 {
				return nil, entity.ErrExptTemplateScheduleBindingInvalid
			}
		}
	}
	copy := *base
	copy.scheduleManagement = &deps
	return &copy, nil
}

type templateScheduleHookPatch struct {
	SpaceID, TemplateID int64
	Input               entity.HookConfigUpdateInput
}
type templateScheduleWriteRepo struct {
	repo.IExptTemplateRepo
	manager                  *ExptTemplateManagerImpl
	deps                     ExptTemplateScheduleDependencies
	key                      entity.ExptTemplateScheduleBindingKey
	state                    *repo.ExptTemplateScheduleState
	create, explicitSchedule bool
	patch                    entity.HookConfigUpdateInput
}

func (e *ExptTemplateManagerImpl) scopedTemplateSchedule(ctx context.Context, id, space int64, create bool) (*ExptTemplateManagerImpl, *templateScheduleWriteRepo, error) {
	d := *e.scheduleManagement
	key := entity.ExptTemplateScheduleBindingKey{SpaceID: space, TemplateID: id, ExecutionScope: d.Scope}
	r := &templateScheduleWriteRepo{IExptTemplateRepo: e.templateRepo, manager: e, deps: d, key: key, create: create}
	if e.scheduleHook != nil {
		p := e.scheduleHook
		if p.SpaceID != space || !create && p.TemplateID != id {
			return nil, nil, entity.ErrHookStoreConflict
		}
		r.patch = p.Input
	}
	if !create {
		var err error
		r.state, err = d.Store.Read(ctx, key)
		if err != nil {
			return nil, nil, err
		}
		if r.state == nil || r.state.Template == nil || r.state.Config == nil {
			return nil, nil, entity.ErrHookConfigStorage
		}
	}
	copy := *e
	copy.scheduleManagement = nil
	copy.scheduleHook = nil
	copy.templateRepo = r
	copy.scheduleAdapter = nil
	return &copy, r, nil
}
func templateScheduleSession(ctx context.Context, session *entity.Session) *entity.Session {
	if user, ok := sessions.UserIDInCtx(ctx); ok && user != "" {
		var copy entity.Session
		if session != nil {
			copy = *session
		}
		copy.UserID = user
		return &copy
	}
	return session
}
func templateScheduleSource(t *entity.ExptTemplate) *entity.ExptSource {
	if t == nil {
		return nil
	}
	if t.ExptSource != nil {
		return t.ExptSource
	}
	if t.TemplateConf != nil {
		return t.TemplateConf.ExptSource
	}
	return nil
}
func templateScheduleHookEnabled(c *entity.LifecycleHookConf) bool {
	return c != nil && (c.Before != nil && gptr.Indirect(c.Before.Enabled) || c.After != nil && gptr.Indirect(c.After.Enabled))
}

func mergeTemplateScheduleSource(old, patch *entity.ExptSource) *entity.ExptSource {
	copy := *patch
	if patch.Scheduler == nil {
		copy.Scheduler = old.Scheduler
		return &copy
	}
	scheduler := *patch.Scheduler
	if previous := old.Scheduler; previous != nil {
		if scheduler.Enabled == nil {
			scheduler.Enabled = previous.Enabled
		}
		if scheduler.Frequency == nil {
			scheduler.Frequency = previous.Frequency
		}
		if scheduler.TriggerAt == nil {
			scheduler.TriggerAt = previous.TriggerAt
		}
		if scheduler.StartTime == nil {
			scheduler.StartTime = previous.StartTime
		}
		if scheduler.EndTime == nil {
			scheduler.EndTime = previous.EndTime
		}
	}
	copy.Scheduler = &scheduler
	return &copy
}
func templateScheduleActive(t *entity.ExptTemplate) bool {
	s := templateScheduleSource(t)
	return t.ExptInfo != nil && t.ExptInfo.CronActivate && s != nil && s.SourceType == entity.SourceType_Evaluation && isSchedulerEnabled(s.Scheduler)
}

func (r *templateScheduleWriteRepo) GetByID(ctx context.Context, id int64, space *int64) (*entity.ExptTemplate, error) {
	if id != r.key.TemplateID || space == nil || *space != r.key.SpaceID {
		return nil, entity.ErrHookStoreConflict
	}
	return r.state.Template, nil
}
func (r *templateScheduleWriteRepo) Create(ctx context.Context, t *entity.ExptTemplate, refs []*entity.ExptTemplateEvaluatorRef) error {
	if !r.create || t.GetSpaceID() != r.key.SpaceID {
		return entity.ErrHookStoreConflict
	}
	r.key.TemplateID = t.GetID()
	return r.save(ctx, t, refs, nil)
}
func (r *templateScheduleWriteRepo) UpdateWithRefs(ctx context.Context, t *entity.ExptTemplate, refs []*entity.ExptTemplateEvaluatorRef) error {
	return r.save(ctx, t, refs, nil)
}
func (r *templateScheduleWriteRepo) UpdateFields(ctx context.Context, id int64, fields map[string]any) error {
	if id != r.key.TemplateID {
		return entity.ErrHookStoreConflict
	}
	raw, err := json.Marshal(r.state.Template)
	if err != nil {
		return err
	}
	var next entity.ExptTemplate
	if err = json.Unmarshal(raw, &next); err != nil {
		return err
	}
	if cron, ok := fields["cron_activate"].(bool); ok {
		if next.ExptInfo == nil {
			next.ExptInfo = &entity.ExptInfo{}
		}
		next.ExptInfo.CronActivate = cron
	}
	return r.save(ctx, &next, nil, fields)
}
func (r *templateScheduleWriteRepo) save(ctx context.Context, t *entity.ExptTemplate, refs []*entity.ExptTemplateEvaluatorRef, fields map[string]any) error {
	if t.ExptSource == nil && t.TemplateConf != nil {
		t.ExptSource = t.TemplateConf.ExptSource
	}
	if t.ExptSource != nil {
		if t.TemplateConf == nil {
			t.TemplateConf = &entity.ExptTemplateConfiguration{}
		} else {
			copy := *t.TemplateConf
			t.TemplateConf = &copy
		}
		t.TemplateConf.ExptSource = t.ExptSource
	}
	var oldConf *entity.LifecycleHookConf
	var oldBinding *entity.ExptTemplateScheduleBinding
	var revision string
	if r.state != nil {
		oldConf = r.state.Config.Config
		oldBinding = r.state.Binding
		revision = r.state.Revision
	}
	conf, err := entity.ResolveLifecycleHookConf(oldConf, r.patch.Config)
	if err != nil {
		return err
	}
	hooked := templateScheduleHookEnabled(conf)
	active := templateScheduleActive(t)
	explicitHook := r.patch.Config != nil && (r.patch.Config.Before != nil || r.patch.Config.After != nil)
	var next *entity.ExptTemplateScheduleBinding
	if hooked && active && (r.create || r.explicitSchedule || explicitHook) {
		user, ok := sessions.UserIDInCtx(ctx)
		if !ok || user == "" {
			return entity.ErrExptTemplateScheduleBindingInvalid
		}
		version := int64(1)
		if oldBinding != nil {
			if oldBinding.Version == math.MaxInt64 {
				return entity.ErrExptTemplateScheduleBindingConflict
			}
			version = oldBinding.Version + 1
		}
		if missingManagerHookDependency(r.manager.idgen) {
			return entity.ErrHookConfigStorage
		}
		id, err := r.manager.idgen.GenID(ctx)
		if err != nil {
			return err
		}
		if id <= 0 {
			return entity.ErrHookConfigStorage
		}
		next = &entity.ExptTemplateScheduleBinding{SchemaVersion: 1, BindingID: strconv.FormatInt(id, 10), Version: version, UserID: user, IdentityType: "fornax_user", SpaceID: r.key.SpaceID, TemplateID: r.key.TemplateID, ExecutionScope: r.key.ExecutionScope, Namespace: r.deps.Namespace, Group: r.deps.Group, BizKey: fmt.Sprintf("hook_template_%d_%d_%d_%d", r.key.SpaceID, r.key.TemplateID, id, version), Enabled: true, BoundAt: time.Now().UTC(), Callback: r.deps.Callback}
		if err = next.Validate(); err != nil {
			return err
		}
		if _, err = buildCreatePeriodicJobParam(next.BizKey, r.key.SpaceID, r.key.TemplateID, templateScheduleSource(t).Scheduler); err != nil {
			return err
		}
	}
	ids, err := hookConfigCreateRefIDs(ctx, r.manager.idgen, len(refs))
	if err != nil {
		return err
	}
	allocated := make([]*entity.ExptTemplateEvaluatorRef, len(refs))
	for i, ref := range refs {
		if ref == nil {
			return entity.ErrHookConfigStorage
		}
		copy := *ref
		copy.ID = ids[i]
		allocated[i] = &copy
	}
	in := repo.ExptTemplateScheduleWrite{Key: r.key, Create: r.create, ExpectedRevision: revision, Template: t, Refs: allocated, Fields: fields, Hook: r.patch, Binding: next, Disable: oldBinding != nil && oldBinding.Enabled && (!hooked || !active)}
	if fields != nil {
		in.Template = nil
	}
	if err = r.deps.Store.Write(ctx, in); err != nil {
		return err
	}
	committedVersion := int64(0)
	if next != nil {
		committedVersion = next.Version
	} else if oldBinding != nil {
		committedVersion = oldBinding.Version
		if in.Disable {
			committedVersion++
		}
	}
	// The write is durable even if confirmation cannot be read back.
	committed := &ExptTemplateSchedulePendingError{TemplateID: r.key.TemplateID, BindingVersion: committedVersion}
	state, err := r.deps.Store.Read(ctx, r.key)
	if err != nil {
		committed.cause = err
		return committed
	}
	r.state = state
	if oldBinding != nil && (next != nil || in.Disable) {
		if err = r.manager.scheduleAdapter.CloseJob(ctx, oldBinding.BizKey); err != nil {
			return r.pending()
		}
	}
	return r.sync(ctx)
}
func (r *templateScheduleWriteRepo) pending() error {
	v := int64(0)
	if r.state != nil && r.state.Binding != nil {
		v = r.state.Binding.Version
	}
	return &ExptTemplateSchedulePendingError{TemplateID: r.key.TemplateID, BindingVersion: v}
}
func (r *templateScheduleWriteRepo) sync(ctx context.Context) error {
	if r.state == nil || r.state.Template == nil || r.state.Config == nil {
		return entity.ErrHookConfigStorage
	}
	if b := r.state.Binding; b != nil && !b.Enabled {
		if err := r.manager.scheduleAdapter.CloseJob(ctx, b.BizKey); err != nil {
			return r.pending()
		}
	}
	if !templateScheduleHookEnabled(r.state.Config.Config) {
		r.manager.syncSchedulerForTemplate(ctx, r.state.Template)
		return nil
	}
	// Never fall back to the legacy callback while Hook configuration is enabled.
	if err := r.manager.scheduleAdapter.CloseJob(ctx, buildScheduleBizKey(r.key.SpaceID, r.key.TemplateID)); err != nil {
		return r.pending()
	}
	b := r.state.Binding
	if !templateScheduleActive(r.state.Template) {
		return nil
	}
	if b == nil || !b.Enabled {
		return repo.ErrHookConfigScheduleBindingRequired
	}
	if b.Active() {
		return nil
	}
	p, err := buildCreatePeriodicJobParam(b.BizKey, r.key.SpaceID, r.key.TemplateID, templateScheduleSource(r.state.Template).Scheduler)
	if err != nil {
		return r.pending()
	}
	p.CallbackMethod = b.Callback.Method
	params := struct {
		WorkspaceID    int64  `json:"workspace_id"`
		TemplateID     int64  `json:"template_id"`
		BindingID      string `json:"binding_id"`
		BindingVersion int64  `json:"binding_version"`
	}{b.SpaceID, b.TemplateID, b.BindingID, b.Version}
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	p.CallbackPayload = string(raw)
	receipt, err := r.deps.Receipts.CreatePeriodicJobWithReceipt(ctx, p, b.Namespace, b.Group)
	if err != nil || receipt == nil || receipt.Validate() != nil || receipt.Namespace != b.Namespace || receipt.Group != b.Group || receipt.BizKey != b.BizKey || receipt.Callback != b.Callback {
		return r.pending()
	}
	bindingID, bindingVersion := b.BindingID, b.Version
	_, err = r.deps.Bindings.ActivateCAS(ctx, r.key, bindingID, bindingVersion, *receipt)
	if err != nil {
		return err
	}
	committed := &ExptTemplateSchedulePendingError{TemplateID: r.key.TemplateID, BindingVersion: bindingVersion}
	state, err := r.deps.Store.Read(ctx, r.key)
	if err != nil {
		committed.cause = err
		return committed
	}
	r.state = state
	return nil
}

// ReconcileExptTemplateSchedule retries a persisted pending registration, without rebinding its user/version.
func ReconcileExptTemplateSchedule(ctx context.Context, manager IExptTemplateManager, space, id int64) error {
	base, ok := manager.(*ExptTemplateManagerImpl)
	if !ok || base == nil || base.scheduleManagement == nil {
		return entity.ErrExptTemplateScheduleBindingInvalid
	}
	_, r, err := base.scopedTemplateSchedule(ctx, id, space, false)
	if err != nil {
		return err
	}
	return r.sync(ctx)
}
