// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	infraHook "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	experiment "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// Platform bindings are deployment-owned. Runtime switches and endpoint policy
// are always read by the existing RuntimeConfigProvider, including on recovery.
type HookRuntimePlatform struct {
	Scope                   string
	StorageKeyID            string
	Config                  *infraHook.RuntimeConfigProvider
	Codec                   *infraHook.StorageCodec
	Identity                hook.IdentityProvider
	WorkspaceSigningSecrets hook.WorkspaceSigningSecretProvider
	Wake                    *HookRuntimeWake
	ProtectedBackend        bool
}

func NewOSSHookRuntimePlatform(cf conf.IConfigLoaderFactory, factory mq.IFactory, users rpc.IUserProvider) (*HookRuntimePlatform, error) {
	loader, err := cf.NewConfigLoader(consts.EvaluationConfigFileName)
	if err != nil {
		return nil, err
	}
	identity, err := infraHook.NewIdentityProvider(users, 500*time.Millisecond)
	if err != nil {
		return nil, err
	}
	scope := os.Getenv("PSM")
	if scope == "" {
		scope = "coze-loop"
	}
	runtimeEnv := os.Getenv("K_ENV")
	if runtimeEnv == "" {
		runtimeEnv = "local"
	}
	// OSS has no authenticated key backend. The codec rejects protected writes
	// and reads until an actual Protector is supplied by the deployment provider.
	return NewHookRuntimePlatform(loader, factory, scope+"/"+runtimeEnv, runtimeEnv, nil, nil, identity), nil
}

func NewHookRuntimePlatform(loader conf.IConfigLoader, factory mq.IFactory, scope, environment string, protector hook.Protector, secrets hook.SigningSecretProvider, identity hook.IdentityProvider) *HookRuntimePlatform {
	config := infraHook.NewRuntimeConfigProvider(loader, true)
	key, _ := config.StorageKeyID(context.Background())
	wake := NewHookRuntimeWake(loader, factory, config, scope, secrets)
	wake.environment = environment
	return &HookRuntimePlatform{Scope: scope, StorageKeyID: key, Config: config, Codec: infraHook.NewStorageCodec(protector), Identity: identity,
		Wake: wake, ProtectedBackend: !hookWorkerNil(protector) && !hookWorkerNil(secrets)}
}

type HookRuntimeStores struct {
	DB             db.Provider
	Runs           repo.IHookRepo
	Configs        repo.IHookConfigRepo
	Summaries      repo.IHookSummaryRepo
	Initialization repo.IHookRunInitializationRepo
	Finalization   repo.IHookFinalizationRepo
	Gate           repo.IHookGateRepo
	Source         repo.IHookItemSourceRepo
	Plans          repo.IHookPlanRepo
	Platform       *HookRuntimePlatform
	Redis          redis.Cmdable
}

func NewHookRuntimeStores(p db.Provider, c redis.Cmdable, platform *HookRuntimePlatform) *HookRuntimeStores {
	return &HookRuntimeStores{DB: p, Redis: c, Platform: platform, Runs: experiment.NewHookRunRepo(p),
		Configs: experiment.NewHookConfigRepo(p, platform.Codec), Summaries: experiment.NewHookSummaryRepo(p),
		Initialization: experiment.NewHookRunInitializationRepo(p), Finalization: experiment.NewHookFinalizationRepo(p),
		Gate:   experiment.NewHookExecutionGateRepo(p, func(context.Context) (string, error) { return platform.Scope, nil }),
		Source: experiment.NewHookItemSourceRepo(p), Plans: experiment.NewHookPlanRepo(p)}
}

func (s *HookRuntimeStores) manager(base service.IExptManager) (service.IExptManager, error) {
	p := s.Platform
	var err error
	if p.StorageKeyID != "" {
		base, err = service.NewExptManagerWithHooks(base, service.ExptManagerHookDependencies{
			Initialization: s.Initialization, Runs: s.Runs, Configs: s.Configs, Codec: p.Codec,
			Identity: p.Identity, Runtime: hookRuntimeAdmissionConfig{p}, Wake: p.Wake, ExecutionScope: p.Scope, SnapshotKeyID: p.StorageKeyID})
		if err != nil {
			return nil, err
		}
	}
	manager, err := service.NewExptManagerWithHookFinalization(base, service.ExptManagerFinalizationDependencies{
		Runs: s.Runs, Repository: s.Finalization, Owners: experiment.NewHookFinalizationOwnerReader(s.Redis),
		ExecutionScope: p.Scope, NewItemLocker: func() lock.ILocker { return lock.NewRedisLockerWithHolder(s.Redis, "evaluation") }})
	if err != nil {
		return nil, err
	}
	return service.NewExptManagerWithHookDeletion(manager, experiment.NewHookDeletionRepo(s.DB))
}

type HookRuntimeServices struct {
	Worker     *HookWorker
	Wake       *HookRuntimeWake
	mu         sync.Mutex
	started    bool
	generation uint64
}

func wireHookRuntime(app *experimentApplication, stores *HookRuntimeStores, items service.EvaluationSetItemService, versions service.EvaluationSetVersionService, sets service.IEvaluationSetService, refs repo.IExptItemRefRepo, turns repo.IExptTurnResultRepo, results repo.IExptItemResultRepo, expts repo.IExperimentRepo, ids idgen.IIDGenerator) (*experimentApplication, error) {
	p := stores.Platform
	loader, err := service.NewHookFrozenPlanLoader(service.HookFrozenPlanLoaderDependencies{Plans: stores.Plans, Items: items, Versions: versions, Sets: sets})
	if err != nil {
		return nil, err
	}
	factory, err := NewHookRuntimeExecutionFactory(HookRuntimeExecutionFactoryDependencies{
		DB: stores.DB, Runs: stores.Runs, Codec: p.Codec, Scope: p.Scope, Manager: app.manager,
		Scheduler: app.ExptSchedulerEvent, Consumer: app.ExptItemEvalEvent, Loader: loader, IDs: ids})
	if err != nil {
		return nil, err
	}
	initializer, err := service.NewHookFrozenExecutionInitializer(service.HookFrozenExecutionInitializerDependencies{
		Repository: experiment.NewHookExecutionInitializationRepo(stores.DB), Loader: loader, IDs: ids})
	if err != nil {
		return nil, err
	}
	scheduler, err := service.NewHookAwareExptSchedulerSvc(app.ExptSchedulerEvent, stores.Gate, initializer)
	if err != nil {
		return nil, err
	}
	consumer, err := service.NewHookAwareExptRecordEvalService(app.ExptItemEvalEvent, stores.Gate, stores.Source, stores.Runs,
		experiment.NewHookTurnProgressRepo(stores.DB, func(context.Context) (string, error) { return p.Scope, nil }))
	if err != nil {
		return nil, err
	}
	router := &hookRuntimeRouter{IExptManager: app.manager, source: stores.Finalization, initialization: stores.Initialization,
		runs: stores.Runs, codec: p.Codec, scope: p.Scope, factory: factory, scheduler: scheduler, consumer: consumer, admissionInstalled: p.StorageKeyID != "", stores: stores}
	preparer, err := service.NewHookPlanPreparer(service.HookPlanPreparerDependencies{Runs: stores.Runs, Plans: stores.Plans,
		Selector: service.NewHookPlanSelector(items, refs, turns, results), Codec: p.Codec, Experiments: expts,
		Initialization: stores.Initialization, ResultReader: experiment.NewHookPlanResultReader(stores.DB), IDs: ids})
	if err != nil {
		return nil, err
	}
	// Preparation must reach Schedule before execution initialization. Central
	// dispatch and consumers use the stricter execution Gate above.
	preparationGate := experiment.NewHookGateRepo(stores.DB, func(context.Context) (string, error) { return p.Scope, nil })
	coordinator, err := NewHookWorkerCoordinator(HookWorkerCoordinatorDependencies{ExecutionScope: p.Scope, Runs: stores.Runs,
		Preparer: preparer, Manager: router, HookWake: p.Wake, Gate: preparationGate, Codec: p.Codec, SchedulePublisher: router})
	if err != nil {
		return nil, err
	}
	transport := infraHook.NewHTTPTransport(infraHook.NewKeyResolver(p.Config, p.Wake.secrets, p.WorkspaceSigningSecrets), p.Config.EndpointPolicies, p.Config.ResolveEndpointPolicy)
	executor, err := service.NewHookAttemptExecutorWithConfig(stores.Runs, p.Codec, p.Codec, transport, infraHook.NewCompletionProjector(), p.Config)
	if err != nil {
		return nil, err
	}
	clock, err := experiment.NewHookWorkerClock(stores.DB)
	if err != nil {
		return nil, err
	}
	owner, err := os.Hostname()
	if err != nil || owner == "" {
		return nil, ErrHookWorkerConfiguration
	}
	worker, err := NewHookWorker(HookWorkerDependencies{ExecutionScope: p.Scope, Owner: owner, ScanRepo: experiment.NewHookScheduleScanRepo(stores.DB),
		Clock: clock, Config: p.Config, Executor: executor, IDs: ids, Coordinator: coordinator, Observer: hookRuntimeObserver{}})
	if err != nil {
		return nil, err
	}
	runtime := &HookRuntimeServices{Worker: worker, Wake: p.Wake}
	router.runtime = runtime
	copy := *app
	copy.manager, copy.ExptSchedulerEvent, copy.ExptItemEvalEvent = router, router, router
	// Missing platform keys must not disable managed-state reads. Explicit writes
	// still fail validation at the existing creation/update storage boundary.
	copy.hooks = &ExperimentHookApplicationDependencies{Configs: stores.Configs, Summaries: stores.Summaries, Runtime: hookRuntimeAdmissionConfig{p}, ExecutionScope: p.Scope, ConfigKeyID: p.StorageKeyID}
	return &copy, nil
}

type hookRuntimeAdmissionConfig struct{ platform *HookRuntimePlatform }

func (c hookRuntimeAdmissionConfig) GetRuntimeConfig(ctx context.Context) (entity.HookRuntimeConfig, error) {
	p := c.platform
	cfg, err := p.Config.GetRuntimeConfig(ctx)
	if err != nil || !cfg.AdmissionEnabled {
		return cfg, err
	}
	if cfg.WorkspaceAllowlistConfigured && hookWorkerNil(p.WorkspaceSigningSecrets) {
		return entity.HookRuntimeConfig{}, entity.ErrHookConfigStorage
	}
	key, err := p.Config.StorageKeyID(ctx)
	if err != nil || key != p.StorageKeyID || !p.ProtectedBackend {
		return entity.HookRuntimeConfig{}, entity.ErrHookConfigStorage
	}
	broker, err := p.Wake.brokerConfig(ctx)
	if err != nil || broker != nil && (broker.DisableProduce != nil && *broker.DisableProduce || broker.DisableConsume != nil && *broker.DisableConsume) {
		return entity.HookRuntimeConfig{}, ErrHookWakeUnavailable
	}
	return cfg, nil
}

func (e *experimentApplication) HookRuntimeServices() *HookRuntimeServices {
	router, ok := e.manager.(*hookRuntimeRouter)
	if !ok {
		return nil
	}
	return router.runtime
}

type hookRuntimeObserver struct{}

func (hookRuntimeObserver) Observe(event hook.WorkerEvent) {
	if event.Code != hook.WorkerPaused {
		logs.Warn("lifecycle hook worker: code=%s kind=%s", event.Code, event.Kind)
	}
}

func HookRuntimeFromApplication(app IExperimentApplication) (*HookRuntimeServices, error) {
	provider, ok := app.(interface{ HookRuntimeServices() *HookRuntimeServices })
	if !ok || provider.HookRuntimeServices() == nil {
		return nil, entity.ErrHookExecutionUnsupported
	}
	return provider.HookRuntimeServices(), nil
}
