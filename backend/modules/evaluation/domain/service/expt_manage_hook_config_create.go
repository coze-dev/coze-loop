// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"strings"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

// WithExptHookConfigCreate preserves the original creation flow on a request-local manager.
func WithExptHookConfigCreate(manager IExptManager, configs repo.IHookConfigRepo, in repo.HookConfigCreateInput) (IExptManager, error) {
	if in.Config == nil || (in.Config.Before == nil && in.Config.After == nil) {
		return manager, nil
	}
	base, ok := manager.(*ExptMangerImpl)
	if !ok {
		if router, wrapped := manager.(interface{ HookConfigBaseManager() *ExptMangerImpl }); wrapped && !missingManagerHookDependency(router) {
			base = router.HookConfigBaseManager()
			ok = base != nil
		}
	}
	if !ok || base == nil {
		return nil, entity.ErrHookConfigStorage
	}
	creator, normalized, err := hookConfigCreateDependencies(configs, in)
	if err != nil {
		return nil, err
	}
	scoped := *base
	scoped.exptRepo = &exptHookConfigCreateRepo{IExperimentRepo: base.exptRepo, creator: creator, ids: base.idgenerator, in: normalized}
	return &scoped, nil
}

type exptHookConfigCreateRepo struct {
	repo.IExperimentRepo
	creator repo.IHookConfigCreator
	ids     idgen.IIDGenerator
	in      repo.HookConfigCreateInput
}

func (r *exptHookConfigCreateRepo) Create(ctx context.Context, expt *entity.Experiment, refs []*entity.ExptEvaluatorRef) error {
	if expt == nil || expt.ID <= 0 || expt.SpaceID != r.in.WorkspaceID {
		return entity.ErrHookConfigStorage
	}
	for _, ref := range refs {
		if ref == nil || ref.SpaceID != expt.SpaceID || ref.ExptID != expt.ID {
			return entity.ErrHookConfigStorage
		}
	}
	ids, err := hookConfigCreateRefIDs(ctx, r.ids, len(refs))
	if err != nil {
		return err
	}
	allocated := make([]*entity.ExptEvaluatorRef, len(refs))
	for i, ref := range refs {
		copy := *ref
		copy.ID = ids[i]
		allocated[i] = &copy
	}
	return r.creator.CreateExperimentWithHookConfig(ctx, expt, allocated, r.in)
}

func hookConfigCreateDependencies(configs repo.IHookConfigRepo, in repo.HookConfigCreateInput) (repo.IHookConfigCreator, repo.HookConfigCreateInput, error) {
	creator, ok := configs.(repo.IHookConfigCreator)
	if !ok || missingManagerHookDependency(creator) || in.WorkspaceID <= 0 || strings.TrimSpace(in.KeyID) == "" || len(in.ExecutionScope) == 0 || len(in.ExecutionScope) > 128 {
		return nil, in, entity.ErrHookConfigStorage
	}
	for _, c := range in.ExecutionScope {
		if c < 33 || c > 126 {
			return nil, in, entity.ErrHookConfigStorage
		}
	}
	conf, err := entity.ResolveLifecycleHookConf(nil, in.Config)
	if err != nil {
		return nil, in, err
	}
	in.Config = conf
	return creator, in, nil
}

func hookConfigCreateRefIDs(ctx context.Context, generator idgen.IIDGenerator, count int) ([]int64, error) {
	if count == 0 {
		return nil, nil
	}
	if missingManagerHookDependency(generator) {
		return nil, entity.ErrHookConfigStorage
	}
	ids, err := generator.GenMultiIDs(ctx, count)
	if err != nil {
		return nil, err
	}
	if len(ids) != count {
		return nil, entity.ErrHookConfigStorage
	}
	return ids, nil
}
