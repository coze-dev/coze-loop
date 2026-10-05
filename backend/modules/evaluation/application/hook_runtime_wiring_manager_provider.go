// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"github.com/coze-dev/coze-loop/backend/infra/external/audit"
	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
)

type HookRuntimeManagerInputs struct {
	ExptResultService           service.ExptResultService
	ExptRepo                    repo.IExperimentRepo
	ExptRunLogRepo              repo.IExptRunLogRepo
	ExptStatsRepo               repo.IExptStatsRepo
	ExptItemResultRepo          repo.IExptItemResultRepo
	ExptItemRefRepo             repo.IExptItemRefRepo
	ExptTurnResultRepo          repo.IExptTurnResultRepo
	Configer                    component.IConfiger
	QuotaRepo                   repo.QuotaRepo
	Mutex                       lock.ILocker
	Idem                        idem.IdempotentService
	Publisher                   events.ExptEventPublisher
	Audit                       audit.IAuditService
	Idgen                       idgen.IIDGenerator
	Metric                      metrics.ExptMetric
	Lwt                         platestwrite.ILatestWriteTracker
	EvaluationSetVersionService service.EvaluationSetVersionService
	EvaluationSetService        service.IEvaluationSetService
	EvalTargetService           service.IEvalTargetService
	EvaluatorService            service.EvaluatorService
	BenefitService              benefit.IBenefitService
	ExptAggrResultService       service.ExptAggrResultService
	TemplateRepo                repo.IExptTemplateRepo
	TemplateManager             service.IExptTemplateManager
	NotifyRPCAdapter            rpc.INotifyRPCAdapter
	UserProvider                rpc.IUserProvider
	PipelineListAdapter         rpc.IPipelineListAdapter
	ResourceAccessAuthorizer    service.ResourceAccessAuthorizer
	SandboxAgentMetrics         metrics.SandboxAgentMetrics
	CentralScopeProvider        component.ICentralSchedulerScopeProvider
	CentralAdmissionPolicy      component.ICentralAdmissionPolicy
	CentralGuard                component.ICentralReservationGuard
}

func NewHookRuntimeManager(in HookRuntimeManagerInputs, stores *HookRuntimeStores) (service.IExptManager, error) {
	base := service.NewExptManager(in.ExptResultService, in.ExptRepo, in.ExptRunLogRepo, in.ExptStatsRepo, in.ExptItemResultRepo, in.ExptItemRefRepo, in.ExptTurnResultRepo, in.Configer, in.QuotaRepo, in.Mutex, in.Idem, in.Publisher, in.Audit, in.Idgen, in.Metric, in.Lwt, in.EvaluationSetVersionService, in.EvaluationSetService, in.EvalTargetService, in.EvaluatorService, in.BenefitService, in.ExptAggrResultService, in.TemplateRepo, in.TemplateManager, in.NotifyRPCAdapter, in.UserProvider, in.PipelineListAdapter, in.ResourceAccessAuthorizer, in.SandboxAgentMetrics, in.CentralScopeProvider, in.CentralAdmissionPolicy, in.CentralGuard)
	return stores.manager(base)
}
