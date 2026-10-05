// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

// WithExptTemplateHookConfigUpdate requires the optional schedule manager for scheduled templates.
func WithExptTemplateHookConfigUpdate(manager IExptTemplateManager, configs repo.IHookConfigRepo, owner hookcomponent.ConfigOwner, in entity.HookConfigUpdateInput) (IExptTemplateManager, error) {
	if in.Config == nil || (in.Config.Before == nil && in.Config.After == nil) {
		return manager, nil
	}
	base, ok := manager.(*ExptTemplateManagerImpl)
	if !ok || base == nil || owner.Kind != hookcomponent.ConfigOwnerTemplate {
		return nil, entity.ErrHookConfigStorage
	}
	updater, normalized, err := hookConfigUpdateDependencies(configs, owner, in)
	if err != nil {
		return nil, err
	}
	scoped := *base
	if base.scheduleManagement != nil {
		if owner.ExecutionScope != base.scheduleManagement.Scope {
			return nil, entity.ErrHookConfigStorage
		}
		scoped.scheduleHook = &templateScheduleHookPatch{SpaceID: owner.WorkspaceID, TemplateID: owner.ObjectID, Input: normalized}
		return &scoped, nil
	}
	scoped.templateRepo = &exptTemplateHookConfigUpdateRepo{IExptTemplateRepo: base.templateRepo, updater: updater, ids: base.idgen, owner: owner, in: normalized}
	return &exptTemplateHookConfigUpdateManager{ExptTemplateManagerImpl: &scoped, owner: owner}, nil
}

type exptTemplateHookConfigUpdateManager struct {
	*ExptTemplateManagerImpl
	owner hookcomponent.ConfigOwner
}

func (m *exptTemplateHookConfigUpdateManager) Update(ctx context.Context, param *entity.UpdateExptTemplateParam, session *entity.Session) (*entity.ExptTemplate, error) {
	if param == nil || param.TemplateID != m.owner.ObjectID || param.SpaceID != m.owner.WorkspaceID {
		return nil, entity.ErrHookConfigStorage
	}
	if param.CronActivate != nil && *param.CronActivate || hookTemplateHasScheduler(param.ExptSource, param.TemplateConf) {
		return nil, repo.ErrHookConfigScheduleBindingRequired
	}
	request := *param
	scoped := *m.ExptTemplateManagerImpl
	store := *scoped.templateRepo.(*exptTemplateHookConfigUpdateRepo)
	store.updateParam = &request
	scoped.templateRepo = &store
	return scoped.Update(ctx, &request, session)
}
func (m *exptTemplateHookConfigUpdateManager) UpdateMeta(context.Context, *entity.UpdateExptTemplateMetaParam, *entity.Session) (*entity.ExptTemplate, error) {
	// Hook changes use full Update validation, never the separate meta/scheduler side-effect path.
	return nil, entity.ErrHookConfigStorage
}

type exptTemplateHookConfigUpdateRepo struct {
	repo.IExptTemplateRepo
	updater     repo.IHookConfigMetadataUpdater
	ids         idgen.IIDGenerator
	owner       hookcomponent.ConfigOwner
	in          entity.HookConfigUpdateInput
	updateParam *entity.UpdateExptTemplateParam
}

func (r *exptTemplateHookConfigUpdateRepo) GetByID(ctx context.Context, id int64, space *int64) (*entity.ExptTemplate, error) {
	if id != r.owner.ObjectID || space == nil || *space != r.owner.WorkspaceID {
		return nil, entity.ErrHookConfigStorage
	}
	template, err := r.IExptTemplateRepo.GetByID(ctx, id, space)
	if err != nil || template == nil {
		return template, err
	}
	if err := r.validate(template); err != nil {
		return nil, err
	}
	// Hook patches inherit omitted refs from the same read used by legacy validation; [] still clears.
	if r.updateParam != nil && r.updateParam.EvaluatorIDVersionItems == nil {
		items := template.GetEvaluatorIDVersionItems()
		r.updateParam.EvaluatorIDVersionItems = make([]*entity.EvaluatorIDVersionItem, len(items))
		for i, item := range items {
			if item != nil {
				copy := *item
				r.updateParam.EvaluatorIDVersionItems[i] = &copy
			}
		}
	}
	return template, nil
}
func (r *exptTemplateHookConfigUpdateRepo) validate(template *entity.ExptTemplate) error {
	if template == nil || template.Meta == nil || template.Meta.ID != r.owner.ObjectID || template.Meta.WorkspaceID != r.owner.WorkspaceID {
		return entity.ErrHookConfigStorage
	}
	if template.ExptInfo != nil && template.ExptInfo.CronActivate || hookTemplateHasScheduler(template.ExptSource, template.TemplateConf) {
		return repo.ErrHookConfigScheduleBindingRequired
	}
	return nil
}
func hookTemplateHasScheduler(source *entity.ExptSource, conf *entity.ExptTemplateConfiguration) bool {
	return source != nil && source.Scheduler != nil || conf != nil && conf.ExptSource != nil && conf.ExptSource.Scheduler != nil
}
func (r *exptTemplateHookConfigUpdateRepo) UpdateWithRefs(ctx context.Context, template *entity.ExptTemplate, refs []*entity.ExptTemplateEvaluatorRef) error {
	if err := r.validate(template); err != nil {
		return err
	}
	for _, ref := range refs {
		if ref == nil || ref.SpaceID != r.owner.WorkspaceID || ref.ExptTemplateID != r.owner.ObjectID {
			return entity.ErrHookConfigStorage
		}
	}
	ids, err := hookConfigCreateRefIDs(ctx, r.ids, len(refs))
	if err != nil {
		return err
	}
	allocated := make([]*entity.ExptTemplateEvaluatorRef, len(refs))
	for i, ref := range refs {
		copy := *ref
		copy.ID = ids[i]
		allocated[i] = &copy
	}
	return r.updater.UpdateTemplateWithHookConfig(ctx, r.owner, template, allocated, r.in)
}
func (r *exptTemplateHookConfigUpdateRepo) Update(context.Context, *entity.ExptTemplate) error {
	return entity.ErrHookConfigStorage
}
func (r *exptTemplateHookConfigUpdateRepo) UpdateFields(context.Context, int64, map[string]any) error {
	return entity.ErrHookConfigStorage
}
