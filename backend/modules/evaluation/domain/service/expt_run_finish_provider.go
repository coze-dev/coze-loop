// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

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
)

// NewExptManagerWithRunFinish is the production provider; the old constructor
// retains existing call sites while current repositories fail closed without it.
func NewExptManagerWithRunFinish(
	exptResultService ExptResultService,
	exptRepo repo.IExperimentRepo,
	exptRunLogRepo repo.IExptRunLogRepo,
	exptStatsRepo repo.IExptStatsRepo,
	exptItemResultRepo repo.IExptItemResultRepo,
	exptItemRefRepo repo.IExptItemRefRepo,
	exptTurnResultRepo repo.IExptTurnResultRepo,
	configer component.IConfiger,
	quotaRepo repo.QuotaRepo,
	mutex lock.ILocker,
	idem idem.IdempotentService,
	publisher events.ExptEventPublisher,
	audit audit.IAuditService,
	idgen idgen.IIDGenerator,
	metric metrics.ExptMetric,
	lwt platestwrite.ILatestWriteTracker,
	evaluationSetVersionService EvaluationSetVersionService,
	evaluationSetService IEvaluationSetService,
	evalTargetService IEvalTargetService,
	evaluatorService EvaluatorService,
	benefitService benefit.IBenefitService,
	exptAggrResultService ExptAggrResultService,
	templateRepo repo.IExptTemplateRepo,
	templateManager IExptTemplateManager,
	notifyRPCAdapter rpc.INotifyRPCAdapter,
	userProvider rpc.IUserProvider,
	pipelineListAdapter rpc.IPipelineListAdapter,
	resourceAccessAuthorizer ResourceAccessAuthorizer,
	sandboxAgentMetrics metrics.SandboxAgentMetrics,
	centralScopeProvider component.ICentralSchedulerScopeProvider,
	centralAdmissionPolicy component.ICentralAdmissionPolicy,
	centralGuard component.ICentralReservationGuard,
	manualRetryRepo repo.IExptManualRetryRepo,
	evaluatorRecords EvaluatorRecordService,
	currentRunWaker component.ICurrentRunSchedulerWaker,
	manualRetryLocker ManualRetryLockerFactory,
	runFinishRepo repo.IExptRunFinishRepo,
) IExptManager {
	manager := NewExptManager(exptResultService, exptRepo, exptRunLogRepo, exptStatsRepo, exptItemResultRepo, exptItemRefRepo, exptTurnResultRepo, configer, quotaRepo, mutex, idem, publisher, audit, idgen, metric, lwt, evaluationSetVersionService, evaluationSetService, evalTargetService, evaluatorService, benefitService, exptAggrResultService, templateRepo, templateManager, notifyRPCAdapter, userProvider, pipelineListAdapter, resourceAccessAuthorizer, sandboxAgentMetrics, centralScopeProvider, centralAdmissionPolicy, centralGuard, manualRetryRepo, evaluatorRecords, currentRunWaker, manualRetryLocker).(*ExptMangerImpl)
	manager.runFinishRepo = runFinishRepo
	return manager
}
