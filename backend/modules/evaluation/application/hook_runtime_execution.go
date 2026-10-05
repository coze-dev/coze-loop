// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"reflect"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	experiment "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
)

type HookRuntimeExecutionFactoryDependencies struct {
	DB        db.Provider
	Runs      repo.IHookRepo
	Codec     hook.StorageCodec
	Scope     string
	Manager   service.IExptManager
	Scheduler service.ExptSchedulerEvent
	Consumer  service.ExptItemEvalEvent
	Loader    hook.PlanPageLoader
	IDs       idgen.IIDGenerator
}

type HookRuntimeExecutionFactory struct {
	deps HookRuntimeExecutionFactoryDependencies
}

func NewHookRuntimeExecutionFactory(d HookRuntimeExecutionFactoryDependencies) (*HookRuntimeExecutionFactory, error) {
	for _, v := range []any{d.DB, d.Runs, d.Codec, d.Manager, d.Scheduler, d.Consumer, d.Loader, d.IDs} {
		if v == nil || reflect.ValueOf(v).Kind() == reflect.Ptr && reflect.ValueOf(v).IsNil() {
			return nil, entity.ErrHookExecutionUnsupported
		}
	}
	if len(d.Scope) == 0 || len(d.Scope) > 128 {
		return nil, entity.ErrHookStoreCorrupt
	}
	for _, c := range d.Scope {
		if c < 33 || c > 126 {
			return nil, entity.ErrHookStoreCorrupt
		}
	}
	return &HookRuntimeExecutionFactory{deps: d}, nil
}

// ForRun is opt-in; existing no-Hook/single-set constructors and Wire are untouched.
func (f *HookRuntimeExecutionFactory) ForRun(ctx context.Context, key entity.HookRunKey) (*service.HookRuntimeExecution, error) {
	if f == nil {
		return nil, entity.ErrHookExecutionUnsupported
	}
	d := f.deps
	binding, err := service.LoadHookExecutionInitializationBinding(ctx, d.Runs, d.Codec, key, d.Scope)
	if err != nil {
		return nil, err
	}
	stores, err := experiment.NewBoundHookRuntimeRepositories(d.DB, binding)
	if err != nil {
		return nil, err
	}
	return service.NewBoundHookRuntimeExecution(service.HookRuntimeExecutionDependencies{Manager: d.Manager, Scheduler: d.Scheduler, Consumer: d.Consumer, Binding: binding, Repositories: stores, Loader: d.Loader, IDs: d.IDs})
}
