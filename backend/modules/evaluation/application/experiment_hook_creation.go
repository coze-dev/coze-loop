// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"

	domain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func (e *experimentApplication) hookConfigCreateInput(ctx context.Context, spaceID int64, c *domain.LifecycleHookConf) (repo.HookConfigCreateInput, error) {
	conf, err := entity.ResolveLifecycleHookConf(nil, experiment.LifecycleHookConfDTO2DO(c))
	if err != nil {
		return repo.HookConfigCreateInput{}, err
	}
	if conf == nil || (conf.Before == nil && conf.After == nil) {
		return repo.HookConfigCreateInput{}, nil
	}
	if e.hooks == nil || nilHookApplicationDependency(e.hooks.Configs) || nilHookApplicationDependency(e.hooks.Runtime) {
		return repo.HookConfigCreateInput{}, entity.ErrHookConfigStorage
	}
	if hookApplicationEnabled(conf) {
		runtime, err := e.hooks.Runtime.GetRuntimeConfig(ctx)
		if err != nil || !runtime.AdmissionEnabled {
			return repo.HookConfigCreateInput{}, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("HOOK_FEATURE_DISABLED"))
		}
	}
	return repo.HookConfigCreateInput{WorkspaceID: spaceID, ExecutionScope: e.hooks.ExecutionScope, KeyID: e.hooks.ConfigKeyID, Config: conf}, nil
}

func (e *experimentApplication) hookTemplateCreateManager(ctx context.Context, manager service.IExptTemplateManager, param *entity.CreateExptTemplateParam, c *domain.LifecycleHookConf) (service.IExptTemplateManager, *entity.LifecycleHookConf, error) {
	in, err := e.hookConfigCreateInput(ctx, param.SpaceID, c)
	if err != nil {
		return nil, nil, err
	}
	if in.Config == nil {
		return manager, nil, nil
	}
	if hookTemplateHasSchedule(param.CronActivate, param.ExptSource) {
		return nil, nil, hookScheduleUnavailable()
	}
	scoped, err := service.WithExptTemplateHookConfigCreate(manager, e.hooks.Configs, in)
	return scoped, in.Config, err
}

func hookTemplateHasSchedule(cron bool, source *entity.ExptSource) bool {
	return cron || (source != nil && source.Scheduler != nil && source.Scheduler.Enabled != nil && *source.Scheduler.Enabled)
}

func hookScheduleUnavailable() error {
	return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("HOOK_SCHEDULE_IDENTITY_UNAVAILABLE"))
}
