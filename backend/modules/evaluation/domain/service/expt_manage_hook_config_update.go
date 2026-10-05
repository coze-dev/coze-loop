// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"strings"

	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

// WithExptHookConfigUpdate retains Manager.Update audit; callers retain the existing CheckName step.
func WithExptHookConfigUpdate(manager IExptManager, configs repo.IHookConfigRepo, owner hookcomponent.ConfigOwner, in entity.HookConfigUpdateInput) (IExptManager, error) {
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
	if !ok || base == nil || owner.Kind != hookcomponent.ConfigOwnerExperiment {
		return nil, entity.ErrHookConfigStorage
	}
	updater, normalized, err := hookConfigUpdateDependencies(configs, owner, in)
	if err != nil {
		return nil, err
	}
	scoped := *base
	scoped.exptRepo = &exptHookConfigUpdateRepo{IExperimentRepo: base.exptRepo, updater: updater, owner: owner, in: normalized}
	return &scoped, nil
}

type exptHookConfigUpdateRepo struct {
	repo.IExperimentRepo
	updater repo.IHookConfigMetadataUpdater
	owner   hookcomponent.ConfigOwner
	in      entity.HookConfigUpdateInput
}

func (r *exptHookConfigUpdateRepo) Update(ctx context.Context, expt *entity.Experiment) error {
	if expt == nil || expt.ID != r.owner.ObjectID || expt.SpaceID != r.owner.WorkspaceID {
		return entity.ErrHookConfigStorage
	}
	return r.updater.UpdateExperimentWithHookConfig(ctx, r.owner, expt, r.in)
}

func hookConfigUpdateDependencies(configs repo.IHookConfigRepo, owner hookcomponent.ConfigOwner, in entity.HookConfigUpdateInput) (repo.IHookConfigMetadataUpdater, entity.HookConfigUpdateInput, error) {
	updater, ok := configs.(repo.IHookConfigMetadataUpdater)
	if !ok || missingManagerHookDependency(updater) || owner.WorkspaceID <= 0 || owner.ObjectID <= 0 || strings.TrimSpace(in.KeyID) == "" || len(owner.ExecutionScope) == 0 || len(owner.ExecutionScope) > 128 {
		return nil, in, entity.ErrHookConfigStorage
	}
	for _, c := range owner.ExecutionScope {
		if c < 33 || c > 126 {
			return nil, in, entity.ErrHookConfigStorage
		}
	}
	conf, err := entity.ResolveLifecycleHookConf(nil, in.Config)
	if err != nil {
		return nil, in, err
	}
	in.Config = conf
	return updater, in, nil
}
