// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

// WithExptTemplateHookConfigCreate does not accept or copy schedule identity bindings.
func WithExptTemplateHookConfigCreate(manager IExptTemplateManager, configs repo.IHookConfigRepo, in repo.HookConfigCreateInput) (IExptTemplateManager, error) {
	if in.Config == nil || (in.Config.Before == nil && in.Config.After == nil) {
		return manager, nil
	}
	base, ok := manager.(*ExptTemplateManagerImpl)
	if !ok || base == nil {
		return nil, entity.ErrHookConfigStorage
	}
	creator, normalized, err := hookConfigCreateDependencies(configs, in)
	if err != nil {
		return nil, err
	}
	scoped := *base
	if base.scheduleManagement != nil {
		if normalized.ExecutionScope != base.scheduleManagement.Scope {
			return nil, entity.ErrHookConfigStorage
		}
		scoped.scheduleHook = &templateScheduleHookPatch{SpaceID: normalized.WorkspaceID, Input: entity.HookConfigUpdateInput{KeyID: normalized.KeyID, Config: normalized.Config}}
		return &scoped, nil
	}
	scoped.templateRepo = &exptTemplateHookConfigCreateRepo{IExptTemplateRepo: base.templateRepo, creator: creator, ids: base.idgen, in: normalized}
	return &scoped, nil
}

type exptTemplateHookConfigCreateRepo struct {
	repo.IExptTemplateRepo
	creator repo.IHookConfigCreator
	ids     idgen.IIDGenerator
	in      repo.HookConfigCreateInput
}

func (r *exptTemplateHookConfigCreateRepo) Create(ctx context.Context, template *entity.ExptTemplate, refs []*entity.ExptTemplateEvaluatorRef) error {
	if template == nil || template.Meta == nil || template.Meta.ID <= 0 || template.Meta.WorkspaceID != r.in.WorkspaceID {
		return entity.ErrHookConfigStorage
	}
	for _, ref := range refs {
		if ref == nil || ref.SpaceID != template.Meta.WorkspaceID || ref.ExptTemplateID != template.Meta.ID {
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
	return r.creator.CreateTemplateWithHookConfig(ctx, template, allocated, r.in)
}
