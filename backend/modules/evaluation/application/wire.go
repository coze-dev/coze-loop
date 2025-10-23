// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

//go:build wireinject
// +build wireinject

package application

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/rpc/notify"
	"github.com/google/wire"
	"github.com/sirupsen/logrus"

	"github.com/coze-dev/coze-loop/backend/infra/ck"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/external/audit"
	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	"github.com/coze-dev/coze-loop/backend/infra/fileserver"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/limiter"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/infra/metrics"
	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/apis/promptexecuteservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/dataset/datasetservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/tag/tagservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation"
	evaluationservice "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/auth/authservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/file/fileservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/user/userservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/runtime/llmruntimeservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/promptmanageservice"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	mtr "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	componentrpc "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/userinfo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	domainservice "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	evaltargetmtr "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/metrics/eval_target"
	evalsetmtr "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/metrics/evaluation_set"
	evaluatormtr "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/metrics/evaluator"
	exptmtr "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/metrics/experiment"
	evalmtr "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/metrics/openapi"
	rmqproducer "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/mq/rocket/producer"
	evaluatorrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator"
	evaluatormysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptck "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/ck"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	exptredis "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/dao"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/idem"
	iredis "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/idem/redis"
	targetrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/rpc/agent"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/rpc/data"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/rpc/foundation"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/rpc/llm"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/rpc/prompt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/rpc/tag"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/runtime"
	evalconf "github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/conf"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
)

var (
	flagSet = wire.NewSet(
		platestwrite.NewLatestWriteTracker,
	)

	experimentSet = wire.NewSet(
		NewExperimentApplication,
		domainservice.NewExptManager,
		domainservice.NewExptResultService,
		domainservice.NewExptAggrResultService,
		domainservice.NewExptSchedulerSvc,
		domainservice.NewExptRecordEvalService,
		domainservice.NewExptAnnotateService,
		domainservice.NewExptResultExportService,
		domainservice.NewInsightAnalysisService,
		domainservice.NewSchedulerModeFactory,
		experiment.NewExptRepo,
		experiment.NewExptStatsRepo,
		experiment.NewExptAggrResultRepo,
		experiment.NewExptItemResultRepo,
		experiment.NewExptTurnResultRepo,
		experiment.NewExptRunLogRepo,
		experiment.NewExptTurnResultFilterRepo,
		experiment.NewExptAnnotateRepo,
		experiment.NewExptResultExportRecordRepo,
		experiment.NewExptInsightAnalysisRecordRepo,
		experiment.NewQuotaService,
		idem.NewIdempotentService,
		exptmysql.NewExptDAO,
		exptmysql.NewExptEvaluatorRefDAO,
		exptmysql.NewExptRunLogDAO,
		exptmysql.NewExptStatsDAO,
		exptmysql.NewExptTurnResultDAO,
		exptmysql.NewExptItemResultDAO,
		exptmysql.NewExptTurnEvaluatorResultRefDAO,
		exptmysql.NewExptTurnResultFilterKeyMappingDAO,
		exptmysql.NewExptAggrResultDAO,
		exptmysql.NewExptTurnAnnotateRecordRefDAO,
		exptmysql.NewAnnotateRecordDAO,
		exptmysql.NewExptTurnResultTagRefDAO,
		exptmysql.NewExptResultExportRecordDAO,
		exptmysql.NewExptInsightAnalysisRecordDAO,
		exptmysql.NewExptInsightAnalysisFeedbackVoteDAO,
		exptmysql.NewExptInsightAnalysisFeedbackCommentDAO,
		exptredis.NewQuotaDAO,
		iredis.NewIdemDAO,
		exptck.NewExptTurnResultFilterDAO,
		evalconf.NewExptConfiger,
		rmqproducer.NewExptEventPublisher,
		exptmtr.NewExperimentMetric,
		evaltargetmtr.NewEvalTargetMetrics,
		foundation.NewAuthRPCProvider,
		foundation.NewUserRPCProvider,
		tag.NewTagRPCProvider,
		agent.NewAgentAdapter,
		notify.NewNotifyRPCAdapter,
		userinfo.NewUserInfoServiceImpl,
		NewLock,
		evalSetDomainService,
		targetDomainService,
		evaluatorDomainService,
		flagSet,
		evalAsyncRepoSet,
	)

	evaluatorDomainService = wire.NewSet(
		domainservice.NewEvaluatorServiceImpl,
		domainservice.NewEvaluatorRecordServiceImpl,
		NewEvaluatorSourceServices,
		llm.NewLLMRPCProvider,
		NewRuntimeFactory,
		NewRuntimeManagerFromFactory,
		NewSandboxConfig,
		NewLogger,

		service.NewCodeBuilderFactory,
		evaluatorrepo.NewEvaluatorRepo,
		evaluatorrepo.NewEvaluatorRecordRepo,
		evaluatormysql.NewEvaluatorDAO,
		evaluatormysql.NewEvaluatorVersionDAO,
		evaluatormysql.NewEvaluatorRecordDAO,
		evaluatorrepo.NewRateLimiterImpl,
		evalconf.NewEvaluatorConfiger,
		evaluatormtr.NewEvaluatorMetrics,
		rmqproducer.NewEvaluatorEventPublisher,
	)

	evaluatorSet = wire.NewSet(
		NewEvaluatorHandlerImpl,
		foundation.NewAuthRPCProvider,
		foundation.NewFileRPCProvider,
		foundation.NewUserRPCProvider,
		userinfo.NewUserInfoServiceImpl,
		idem.NewIdempotentService,
		iredis.NewIdemDAO,
		rmqproducer.NewExptEventPublisher,
		evaluatorDomainService,
		flagSet,
		experiment.NewExptRepo,
		exptmysql.NewExptDAO,
		exptmysql.NewExptEvaluatorRefDAO,
	)

	evalSetDomainService = wire.NewSet(
		domainservice.NewEvaluationSetVersionServiceImpl,
		domainservice.NewEvaluationSetItemServiceImpl,
		data.NewDatasetRPCAdapter,
		domainservice.NewEvaluationSetServiceImpl,
	)

	evaluationSetSet = wire.NewSet(
		NewEvaluationSetApplicationImpl,
		evalSetDomainService,
		evalsetmtr.NewEvaluationSetMetrics,
		domainservice.NewEvaluationSetSchemaServiceImpl,
		foundation.NewAuthRPCProvider,
		foundation.NewUserRPCProvider,
		userinfo.NewUserInfoServiceImpl,
	)

	targetDomainService = wire.NewSet(
		domainservice.NewEvalTargetServiceImpl,
		NewSourceTargetOperators,
		prompt.NewPromptRPCAdapter,
		targetrepo.NewEvalTargetRepo,
		mysql.NewEvalTargetDAO,
		mysql.NewEvalTargetRecordDAO,
		mysql.NewEvalTargetVersionDAO,
	)

	evalTargetSet = wire.NewSet(
		NewEvalTargetHandlerImpl,
		evaltargetmtr.NewEvalTargetMetrics,
		foundation.NewAuthRPCProvider,
		targetDomainService,
		flagSet,
		evalAsyncRepoSet,
	)

	evalAsyncRepoSet = wire.NewSet(
		experiment.NewEvalAsyncRepo,
		exptredis.NewEvalAsyncDAO,
	)

	evalOpenAPISet = wire.NewSet(
		NewEvalOpenAPIApplication,
		experimentSet,
		evalmtr.NewEvaluationOApiMetrics,
		domainservice.NewEvaluationSetSchemaServiceImpl,
		data.NewDatasetRPCAdapter,
	)
)

func NewSourceTargetOperators(adapter rpc.IPromptRPCAdapter) map[entity.EvalTargetType]service.ISourceEvalTargetOperateService {
	return map[entity.EvalTargetType]service.ISourceEvalTargetOperateService{
		entity.EvalTargetTypeLoopPrompt: service.NewPromptSourceEvalTargetServiceImpl(adapter),
	}
}

func NewLock(cmdable redis.Cmdable) lock.ILocker {
	return lock.NewRedisLockerWithHolder(cmdable, "evaluation")
}

func InitExperimentApplication(
	ctx context.Context,
	idgen idgen.IIDGenerator,
	db db.Provider,
	configFactory conf.IConfigLoaderFactory,
	rmqFactory mq.IFactory,
	cmdable redis.Cmdable,
	auditClient audit.IAuditService,
	meter metrics.Meter,
	authClient authservice.Client,
	evalSetService evaluationservice.EvaluationSetService,
	evaluatorService evaluationservice.EvaluatorService,
	targetService evaluationservice.EvalTargetService,
	uc userservice.Client,
	pms promptmanageservice.Client,
	pes promptexecuteservice.Client,
	sds datasetservice.Client,
	limiterFactory limiter.IRateLimiterFactory,
	llmcli llmruntimeservice.Client,
	benefitSvc benefit.IBenefitService,
	ckDb ck.Provider,
	tagClient tagservice.Client,
	objectStorage fileserver.ObjectStorage,
) (IExperimentApplication, error) {
	wire.Build(
		experimentSet,
	)
	return nil, nil
}

func InitEvaluatorApplication(
	ctx context.Context,
	idgen idgen.IIDGenerator,
	authClient authservice.Client,
	db db.Provider,
	configFactory conf.IConfigLoaderFactory,
	rmqFactory mq.IFactory,
	llmClient llmruntimeservice.Client,
	meter metrics.Meter,
	userClient userservice.Client,
	auditClient audit.IAuditService,
	cmdable redis.Cmdable,
	benefitSvc benefit.IBenefitService,
	limiterFactory limiter.IRateLimiterFactory,
	fileClient fileservice.Client,
) (evaluation.EvaluatorService, error) {
	wire.Build(
		evaluatorSet,
	)
	return nil, nil
}

func InitEvaluationSetApplication(client datasetservice.Client,
	authClient authservice.Client,
	meter metrics.Meter,
	userClient userservice.Client,
) evaluation.EvaluationSetService {
	wire.Build(
		evaluationSetSet,
	)
	return nil
}

func InitEvalTargetApplication(ctx context.Context,
	idgen idgen.IIDGenerator,
	db db.Provider,
	client promptmanageservice.Client,
	executeClient promptexecuteservice.Client,
	authClient authservice.Client,
	cmdable redis.Cmdable,
	meter metrics.Meter,
) evaluation.EvalTargetService {
	wire.Build(
		evalTargetSet,
	)
	return nil
}

// NewSandboxConfig 创建默认沙箱配置
func NewSandboxConfig() *entity.SandboxConfig {
	return entity.DefaultSandboxConfig()
}

// NewLogger 创建默认日志记录器
func NewLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	return logger
}

// NewRuntimeFactory 创建运行时工厂
func NewRuntimeFactory(logger *logrus.Logger, sandboxConfig *entity.SandboxConfig) component.IRuntimeFactory {
	return runtime.NewRuntimeFactory(logger, sandboxConfig)
}

// NewRuntimeManagerFromFactory 从工厂创建运行时管理器
func NewRuntimeManagerFromFactory(factory component.IRuntimeFactory, logger *logrus.Logger) component.IRuntimeManager {
	return runtime.NewRuntimeManager(factory, logger)
}

func NewEvaluatorSourceServices(
	llmProvider componentrpc.ILLMProvider,
	metric mtr.EvaluatorExecMetrics,
	config evalconf.IConfiger,
	runtimeManager component.IRuntimeManager,
	codeBuilderFactory service.CodeBuilderFactory,
) map[entity.EvaluatorType]domainservice.EvaluatorSourceService {
	// 设置codeBuilderFactory的runtimeManager依赖
	codeBuilderFactory.SetRuntimeManager(runtimeManager)

	services := []domainservice.EvaluatorSourceService{
		domainservice.NewEvaluatorSourcePromptServiceImpl(llmProvider, metric, config),
		domainservice.NewEvaluatorSourceCodeServiceImpl(runtimeManager, codeBuilderFactory, metric),
	}

	serviceMap := make(map[entity.EvaluatorType]domainservice.EvaluatorSourceService)
	for _, svc := range services {
		serviceMap[svc.EvaluatorType()] = svc
	}
	return serviceMap
}

func InitEvalOpenAPIApplication(
	ctx context.Context,
	configFactory conf.IConfigLoaderFactory,
	rmqFactory mq.IFactory,
	cmdable redis.Cmdable,
	idgen idgen.IIDGenerator,
	db db.Provider,
	client promptmanageservice.Client,
	executeClient promptexecuteservice.Client,
	authClient authservice.Client,
	meter metrics.Meter,
	dataClient datasetservice.Client,
	userClient userservice.Client,
	llmClient llmruntimeservice.Client,
	tagClient tagservice.Client,
	limiterFactory limiter.IRateLimiterFactory,
	objectStorage fileserver.ObjectStorage,
	auditClient audit.IAuditService,
	benefitService benefit.IBenefitService,
	ckProvider ck.Provider,
) (IEvalOpenAPIApplication, error) {
	wire.Build(
		evalOpenAPISet,
	)
	return nil, nil
}
