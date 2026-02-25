// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

//go:build wireinject
// +build wireinject

package application

import (
	"context"

	"github.com/google/wire"

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
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/observabilitytraceservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/task/taskservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/promptmanageservice"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/userinfo"
	domainservice "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	evaltargetmetrics "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/metrics/eval_target"
	evaluationsetmetrics "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/metrics/evaluation_set"
	experimentmetrics "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/metrics/experiment"
	openapimetrics "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/metrics/openapi"
	experimentrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	agentrpc "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/rpc/agent"
	foundationrpc "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/rpc/foundation"
	notifyrpc "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/rpc/notify"
	tagrpc "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/rpc/tag"
	trajectoryrpc "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/rpc/trajectory"
	evalconf "github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/conf"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
)

var (
	flagSet = wire.NewSet(
		platestwrite.NewLatestWriteTracker,
	)

	experimentSet = wire.NewSet(
		NewExperimentApplication,
		// Domain Service Sets
		domainservice.ExperimentDomainServiceSet,
		domainservice.EvaluationSetDomainServiceSet,
		domainservice.TargetDomainServiceSet,
		domainservice.EvaluatorDomainServiceSet,
		// Infrastructure Sets
		experimentmetrics.ExperimentMetricsSet,
		evaltargetmetrics.EvalTargetMetricsSet,
		foundationrpc.FoundationRPCSet,
		tagrpc.TagRPCSet,
		agentrpc.AgentRPCSet,
		notifyrpc.NotifyRPCSet,
		userinfo.NewUserInfoServiceImpl,
		NewLock,
		flagSet,
		domainservice.NewDefaultURLProcessor,
	)

	evaluatorSet = wire.NewSet(
		NewEvaluatorHandlerImpl,
		// Domain Service Sets
		domainservice.EvaluatorDomainServiceSet,
		domainservice.EvaluationSetDomainServiceSet,
		domainservice.TargetDomainServiceSet,
		domainservice.NewExptResultService,
		domainservice.NewEvaluationAnalysisService,
		// Infrastructure Sets
		foundationrpc.FoundationRPCSet,
		tagrpc.TagRPCSet,
		trajectoryrpc.TrajectoryRPCSet,
		userinfo.NewUserInfoServiceImpl,
		experimentrepo.ExperimentRepoSet,
		experimentmetrics.ExperimentMetricsSet,
		evaltargetmetrics.EvalTargetMetricsSet,
		evalconf.NewConfiger,
		flagSet,
	)

	evaluationSetSet = wire.NewSet(
		NewEvaluationSetApplicationImpl,
		// Domain Service Sets
		domainservice.EvaluationSetDomainServiceSet,
		// Infrastructure Sets
		evaluationsetmetrics.EvaluationSetMetricsSet,
		foundationrpc.FoundationRPCSet,
		userinfo.NewUserInfoServiceImpl,
	)

	evalTargetSet = wire.NewSet(
		NewEvalTargetHandlerImpl,
		// Domain Service Sets
		domainservice.TargetDomainServiceSet,
		// Infrastructure Sets
		evaltargetmetrics.EvalTargetMetricsSet,
		foundationrpc.FoundationRPCSet,
		experimentrepo.ExperimentRepoSet,
		flagSet,
	)

	evalOpenAPISet = wire.NewSet(
		NewEvalOpenAPIApplication,
		experimentSet,
		evalconf.NewConfiger,
		openapimetrics.OpenAPIMetricsSet,
	)
)

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
	taskClient taskservice.Client,
	objectStorage fileserver.ObjectStorage,
	plainLimiterFactory limiter.IPlainRateLimiterFactory,
	trajectoryAdapter rpc.ITrajectoryAdapter,
) (IExperimentApplication, error) {
	wire.Build(
		experimentSet,
		evalconf.NewConfiger,
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
	plainLimiterFactory limiter.IPlainRateLimiterFactory,
	ckDb ck.Provider,
	tagClient tagservice.Client,
	promptClient promptmanageservice.Client,
	pec promptexecuteservice.Client,
	dataClient datasetservice.Client,
	tracerFactory func() observabilitytraceservice.Client,
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
	trajectoryAdapter rpc.ITrajectoryAdapter,
	configFactory conf.IConfigLoaderFactory,
) (evaluation.EvalTargetService, error) {
	wire.Build(
		evalTargetSet,
		evalconf.NewConfiger,
	)
	return nil, nil
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
	plainLimiterFactory limiter.IPlainRateLimiterFactory,
	trajectoryAdapter rpc.ITrajectoryAdapter,
	taskClient taskservice.Client,
) (IEvalOpenAPIApplication, error) {
	wire.Build(
		evalOpenAPISet,
	)
	return nil, nil
}
