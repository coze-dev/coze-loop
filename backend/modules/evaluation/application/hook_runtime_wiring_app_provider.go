// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	metricscomp "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/userinfo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
)

type HookRuntimeExperimentApplicationInputs struct {
	AggResultSvc               service.ExptAggrResultService
	ResultSvc                  service.ExptResultService
	Manager                    service.IExptManager
	Scheduler                  service.ExptSchedulerEvent
	RecordEval                 service.ExptItemEvalEvent
	Idgen                      idgen.IIDGenerator
	Configer                   component.IConfiger
	Auth                       rpc.IAuthProvider
	UserInfoService            userinfo.UserInfoService
	EvalTargetService          service.IEvalTargetService
	EvaluationSetItemService   service.EvaluationSetItemService
	AnnotateService            service.IExptAnnotateService
	TagRPCAdapter              rpc.ITagRPCAdapter
	ExptResultExportService    service.IExptResultExportService
	ExptInsightAnalysisService service.IExptInsightAnalysisService
	EvaluatorService           service.EvaluatorService
	TemplateManager            service.IExptTemplateManager
	FileProvider               rpc.IFileProvider
	LifecycleEventHandler      service.ExptLifecycleEventHandler
	SandboxSchedulerAdapter    rpc.ISandboxSchedulerAdapter
	SandboxAgentMetrics        metricscomp.SandboxAgentMetrics
}

func NewHookRuntimeExperimentApplication(in HookRuntimeExperimentApplicationInputs, stores *HookRuntimeStores, versions service.EvaluationSetVersionService, sets service.IEvaluationSetService, refs repo.IExptItemRefRepo, turns repo.IExptTurnResultRepo, results repo.IExptItemResultRepo, experiments repo.IExperimentRepo) (IExperimentApplication, error) {
	base := NewExperimentApplication(in.AggResultSvc, in.ResultSvc, in.Manager, in.Scheduler, in.RecordEval, in.Idgen, in.Configer, in.Auth, in.UserInfoService, in.EvalTargetService, in.EvaluationSetItemService, in.AnnotateService, in.TagRPCAdapter, in.ExptResultExportService, in.ExptInsightAnalysisService, in.EvaluatorService, in.TemplateManager, in.FileProvider, in.LifecycleEventHandler, in.SandboxSchedulerAdapter, in.SandboxAgentMetrics).(*experimentApplication)
	return wireHookRuntime(base, stores, in.EvaluationSetItemService, versions, sets, refs, turns, results, experiments, in.Idgen)
}
