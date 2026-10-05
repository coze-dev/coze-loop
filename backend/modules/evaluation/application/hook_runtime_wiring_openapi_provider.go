// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/userinfo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
)

type HookRuntimeOpenAPIInputs struct {
	AsyncRepo                   repo.IEvalAsyncRepo
	Publisher                   events.ExptEventPublisher
	TargetSvc                   service.IEvalTargetService
	EvalTargetRepo              repo.IEvalTargetRepo
	Auth                        rpc.IAuthProvider
	EvaluationSetService        service.IEvaluationSetService
	EvaluationSetVersionService service.EvaluationSetVersionService
	EvaluationSetItemService    service.EvaluationSetItemService
	EvaluationSetSchemaService  service.EvaluationSetSchemaService
	Metric                      metrics.OpenAPIEvaluationMetrics
	SandboxAgentMetric          metrics.SandboxAgentMetrics
	UserInfoService             userinfo.UserInfoService
	ExperimentApp               IExperimentApplication
	Manager                     service.IExptManager
	ResultSvc                   service.ExptResultService
	AggResultSvc                service.ExptAggrResultService
	EvaluatorService            service.EvaluatorService
	EvaluatorRecordService      service.EvaluatorRecordService
	ExptTemplateManager         service.IExptTemplateManager
	Configer                    component.IConfiger
	SandboxSchedulerAdapter     rpc.ISandboxSchedulerAdapter
	FileProvider                rpc.IFileProvider
	CallbackDispatcher          service.IEvaluatorCallbackDispatcher
	ResourceAccessAuthorizer    service.ResourceAccessAuthorizer
}

func NewHookRuntimeOpenAPIApplication(in HookRuntimeOpenAPIInputs) (IEvalOpenAPIApplication, error) {
	base := NewEvalOpenAPIApplication(in.AsyncRepo, in.Publisher, in.TargetSvc, in.EvalTargetRepo, in.Auth, in.EvaluationSetService, in.EvaluationSetVersionService, in.EvaluationSetItemService, in.EvaluationSetSchemaService, in.Metric, in.SandboxAgentMetric, in.UserInfoService, in.ExperimentApp, in.Manager, in.ResultSvc, in.AggResultSvc, in.EvaluatorService, in.EvaluatorRecordService, in.ExptTemplateManager, in.Configer, in.SandboxSchedulerAdapter, in.FileProvider, in.CallbackDispatcher, in.ResourceAccessAuthorizer).(*EvalOpenAPIApplication)
	app, ok := in.ExperimentApp.(*experimentApplication)
	if !ok || app == nil {
		return nil, entity.ErrHookExecutionUnsupported
	}
	base.manager = app.manager
	return base, nil
}
