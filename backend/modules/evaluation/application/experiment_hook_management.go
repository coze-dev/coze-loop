// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"reflect"
	"strings"

	domain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/experiment"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// These dependencies are platform-owned; request fields never select owner, scope or key.
type ExperimentHookApplicationDependencies struct {
	Configs        repo.IHookConfigRepo
	Summaries      repo.IHookSummaryRepo
	Runtime        hookcomponent.RuntimeConfigProvider
	ExecutionScope string
	ConfigKeyID    string
}

func NewExperimentApplicationWithHooks(base IExperimentApplication, deps ExperimentHookApplicationDependencies) (IExperimentApplication, error) {
	app, ok := base.(*experimentApplication)
	if !ok || app == nil || nilHookApplicationDependency(deps.Configs) || nilHookApplicationDependency(deps.Summaries) || nilHookApplicationDependency(deps.Runtime) || strings.TrimSpace(deps.ExecutionScope) != deps.ExecutionScope || deps.ExecutionScope == "" || len(deps.ExecutionScope) > 128 || strings.TrimSpace(deps.ConfigKeyID) == "" {
		return nil, entity.ErrHookConfigStorage
	}
	for _, c := range deps.ExecutionScope {
		if c < 33 || c > 126 {
			return nil, entity.ErrHookConfigStorage
		}
	}
	copied := *app
	copied.hooks = &deps
	return &copied, nil
}

func nilHookApplicationDependency(v any) bool {
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

func hasHookConfigChange(c *domain.LifecycleHookConf) bool {
	return c != nil && (c.Before != nil || c.After != nil)
}

func (e *experimentApplication) hookConfigUpdateInput(ctx context.Context, owner hookcomponent.ConfigOwner, c *domain.LifecycleHookConf) (entity.HookConfigUpdateInput, *entity.LifecycleHookConf, error) {
	if e.hooks == nil || nilHookApplicationDependency(e.hooks.Configs) {
		return entity.HookConfigUpdateInput{}, nil, entity.ErrHookConfigStorage
	}
	current, err := e.hooks.Configs.GetConfig(ctx, owner)
	if err != nil {
		return entity.HookConfigUpdateInput{}, nil, err
	}
	if current == nil {
		return entity.HookConfigUpdateInput{}, nil, entity.ErrHookConfigStorage
	}
	override := experiment.LifecycleHookConfDTO2DO(c)
	resolved, err := entity.ResolveLifecycleHookConf(current.Config, override)
	if err != nil {
		return entity.HookConfigUpdateInput{}, nil, err
	}
	if hookApplicationEnabled(override) {
		if nilHookApplicationDependency(e.hooks.Runtime) {
			return entity.HookConfigUpdateInput{}, nil, entity.ErrHookConfigStorage
		}
		config, err := e.hooks.Runtime.GetRuntimeConfig(ctx)
		if err != nil || !config.AdmissionEnabled {
			return entity.HookConfigUpdateInput{}, nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("HOOK_FEATURE_DISABLED"))
		}
	}
	return entity.HookConfigUpdateInput{Config: override, ExpectedRevision: current.Revision, KeyID: e.hooks.ConfigKeyID}, resolved, nil
}

func (e *experimentApplication) hookTemplateUpdateManager(ctx context.Context, manager service.IExptTemplateManager, got *entity.ExptTemplate, param *entity.UpdateExptTemplateParam, c *domain.LifecycleHookConf) (service.IExptTemplateManager, *entity.LifecycleHookConf, error) {
	if !hasHookConfigChange(c) {
		if e.hooks != nil && ((param.CronActivate != nil && *param.CronActivate) || hookTemplateHasSchedule(false, param.ExptSource)) {
			if got == nil || got.GetID() != param.TemplateID || got.GetSpaceID() != param.SpaceID || nilHookApplicationDependency(e.hooks.Configs) {
				return nil, nil, entity.ErrHookConfigStorage
			}
			record, err := e.hooks.Configs.GetConfig(ctx, hookcomponent.ConfigOwner{WorkspaceID: param.SpaceID, ObjectID: param.TemplateID, Kind: hookcomponent.ConfigOwnerTemplate, ExecutionScope: e.hooks.ExecutionScope})
			if err != nil {
				return nil, nil, err
			}
			if record == nil {
				return nil, nil, entity.ErrHookConfigStorage
			}
			if record.Config != nil && (record.Config.Before != nil || record.Config.After != nil) {
				return nil, nil, repo.ErrHookConfigScheduleBindingRequired
			}
		}
		return manager, nil, nil
	}
	if e.hooks == nil || got == nil || got.GetID() != param.TemplateID || got.GetSpaceID() != param.SpaceID {
		return nil, nil, entity.ErrHookConfigStorage
	}
	owner := hookcomponent.ConfigOwner{WorkspaceID: param.SpaceID, ObjectID: param.TemplateID, Kind: hookcomponent.ConfigOwnerTemplate, ExecutionScope: e.hooks.ExecutionScope}
	in, resolved, err := e.hookConfigUpdateInput(ctx, owner, c)
	if err != nil {
		return nil, nil, err
	}
	scoped, err := service.WithExptTemplateHookConfigUpdate(manager, e.hooks.Configs, owner, in)
	return scoped, resolved, err
}

// Called only after resource authorization; batch errors never fall back to per-object reads.
func (e *experimentApplication) readAuthorizedHookConfigs(ctx context.Context, owners []hookcomponent.ConfigOwner) ([]*entity.HookConfigRecord, error) {
	if len(owners) == 0 {
		return nil, nil
	}
	reader, ok := e.hooks.Configs.(repo.IHookConfigBatchReader)
	if !ok || nilHookApplicationDependency(reader) {
		return nil, entity.ErrHookConfigStorage
	}
	unique := make([]hookcomponent.ConfigOwner, 0, len(owners))
	positions := make(map[hookcomponent.ConfigOwner]int, len(owners))
	for _, owner := range owners {
		if _, exists := positions[owner]; !exists {
			positions[owner] = len(unique)
			unique = append(unique, owner)
		}
	}
	records := make([]*entity.HookConfigRecord, len(unique))
	for start := 0; start < len(unique); start += repo.MaxHookConfigBatchSize {
		end := start + repo.MaxHookConfigBatchSize
		if end > len(unique) {
			end = len(unique)
		}
		batch, err := reader.MGetConfigs(ctx, unique[start:end])
		if err != nil {
			return nil, err
		}
		if len(batch) != end-start {
			return nil, entity.ErrHookConfigStorage
		}
		for i, result := range batch {
			if result.Owner != unique[start+i] {
				return nil, entity.ErrHookConfigStorage
			}
			if result.Err != nil {
				return nil, result.Err
			}
			if result.Record == nil {
				return nil, entity.ErrHookConfigStorage
			}
			records[start+i] = result.Record
		}
	}
	out := make([]*entity.HookConfigRecord, len(owners))
	for i, owner := range owners {
		out[i] = records[positions[owner]]
	}
	return out, nil
}

func (e *experimentApplication) templateSubmitHooks(ctx context.Context, t *entity.ExptTemplate, override *domain.LifecycleHookConf, spaceID int64) (*domain.LifecycleHookConf, error) {
	if e.hooks == nil && override == nil {
		return nil, nil
	}
	dto := experiment.ToExptTemplateDTO(t)
	if err := e.readTemplateHooks(ctx, []*entity.ExptTemplate{t}, []*domain.ExptTemplate{dto}, spaceID); err != nil {
		return nil, err
	}
	resolved, err := entity.ResolveLifecycleHookConf(experiment.LifecycleHookConfDTO2DO(dto.LifecycleHookConf), experiment.LifecycleHookConfDTO2DO(override))
	if err != nil {
		return nil, err
	}
	if resolved != nil && (resolved.Before != nil || resolved.After != nil) {
		cron := t.ExptInfo != nil && t.ExptInfo.CronActivate
		if hookTemplateHasSchedule(cron, t.ExptSource) || entity.NewSession(ctx).UserID == "" {
			return nil, hookScheduleUnavailable()
		}
	}
	return experiment.LifecycleHookConfDO2DTO(resolved), nil
}

func hookApplicationEnabled(c *entity.LifecycleHookConf) bool {
	return c != nil && ((c.Before != nil && c.Before.Enabled != nil && *c.Before.Enabled) || (c.After != nil && c.After.Enabled != nil && *c.After.Enabled))
}

func (e *experimentApplication) readTemplateHooks(ctx context.Context, templates []*entity.ExptTemplate, dtos []*domain.ExptTemplate, spaceID int64) error {
	if e.hooks == nil || len(templates) == 0 {
		return nil
	}
	for _, t := range templates {
		if t == nil || t.GetID() <= 0 || t.GetSpaceID() != spaceID {
			return entity.ErrHookConfigStorage
		}
	}
	if err := e.AuthReadExptTemplates(ctx, templates, spaceID); err != nil {
		return err
	}
	owners := make([]hookcomponent.ConfigOwner, len(templates))
	for i, t := range templates {
		owners[i] = hookcomponent.ConfigOwner{WorkspaceID: spaceID, ObjectID: t.GetID(), Kind: hookcomponent.ConfigOwnerTemplate, ExecutionScope: e.hooks.ExecutionScope}
	}
	records, err := e.readAuthorizedHookConfigs(ctx, owners)
	if err != nil {
		return err
	}
	for i, record := range records {
		dtos[i].LifecycleHookConf = experiment.LifecycleHookConfDO2DTO(record.Config)
	}
	return nil
}

func (e *experimentApplication) readExperimentHooks(ctx context.Context, expts []*entity.Experiment, dtos []*domain.Experiment, spaceID int64) error {
	if e.hooks == nil || len(expts) == 0 {
		return nil
	}
	for _, x := range expts {
		if x == nil || x.ID <= 0 || x.SpaceID != spaceID {
			return entity.ErrHookConfigStorage
		}
	}
	if err := e.AuthReadExperiments(ctx, expts, spaceID); err != nil {
		return err
	}
	keys := make([]entity.HookRunKey, 0, len(expts))
	owners := make([]hookcomponent.ConfigOwner, len(expts))
	for i, x := range expts {
		owners[i] = hookcomponent.ConfigOwner{WorkspaceID: spaceID, ObjectID: x.ID, Kind: hookcomponent.ConfigOwnerExperiment, ExecutionScope: e.hooks.ExecutionScope}
	}
	records, err := e.readAuthorizedHookConfigs(ctx, owners)
	if err != nil {
		return err
	}
	for i, x := range expts {
		dtos[i].LifecycleHookConf = experiment.LifecycleHookConfDO2DTO(records[i].Config)
		if x.LatestRunID > 0 {
			keys = append(keys, entity.HookRunKey{WorkspaceID: spaceID, ExperimentID: x.ID, RunID: x.LatestRunID})
		}
	}
	summaries := make(map[entity.HookRunKey]*entity.LifecycleHookRunSummary, len(keys))
	for start := 0; start < len(keys); start += entity.HookSummaryBatchLimit {
		end := start + entity.HookSummaryBatchLimit
		if end > len(keys) {
			end = len(keys)
		}
		batch, err := e.hooks.Summaries.MGetSummaries(ctx, keys[start:end])
		if err != nil {
			return err
		}
		for _, key := range keys[start:end] {
			summary, ok := batch[key]
			if !ok || (summary != nil && summary.RunID != key.RunID) {
				return entity.ErrHookSummaryUnavailable
			}
			summaries[key] = summary
		}
	}
	for i, x := range expts {
		dtos[i].LifecycleHookSummary = experiment.LifecycleHookSummaryDO2DTO(summaries[entity.HookRunKey{WorkspaceID: spaceID, ExperimentID: x.ID, RunID: x.LatestRunID}])
	}
	return nil
}
