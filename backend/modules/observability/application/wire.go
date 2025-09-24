// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

//go:build wireinject
// +build wireinject

package application

import (
	"github.com/coze-dev/coze-loop/backend/infra/ck"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/limiter"
	"github.com/coze-dev/coze-loop/backend/infra/metrics"
	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/dataset/datasetservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/tag/tagservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/evaluationsetservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/evaluatorservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/experimentservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/auth/authservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/file/fileservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/user/userservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	taskSvc "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service"
	task_processor "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe/processor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe/tracehub"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/exporter"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/processor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/receiver"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/collector/exporter/clickhouseexporter"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/collector/processor/queueprocessor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/collector/receiver/rmqreceiver"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_processor"
	obconfig "github.com/coze-dev/coze-loop/backend/modules/observability/infra/config"
	obmetrics "github.com/coze-dev/coze-loop/backend/modules/observability/infra/metrics"
	mq2 "github.com/coze-dev/coze-loop/backend/modules/observability/infra/mq/producer"
	obrepo "github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo"
	ckdao "github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/ck"
	mysqldao "github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql"
	tredis "github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/redis/dao"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/rpc/auth"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/rpc/dataset"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/rpc/evaluation"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/rpc/evaluationset"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/rpc/evaluator"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/rpc/file"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/rpc/tag"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/rpc/user"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/tenant"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/workspace"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	"github.com/google/wire"
)

var (
	taskDomainSet = wire.NewSet(
		taskSvc.NewTaskServiceImpl,
		obrepo.NewTaskRepoImpl,
		obrepo.NewTaskRunRepoImpl,
		mysqldao.NewTaskDaoImpl,
		tredis.NewTaskDAO,
		tredis.NewTaskRunDAO,
		mysqldao.NewTaskRunDaoImpl,
		mq2.NewBackfillProducerImpl,
	)
	traceDomainSet = wire.NewSet(
		service.NewTraceServiceImpl,
		service.NewTraceExportServiceImpl,
		obrepo.NewTraceCKRepoImpl,
		ckdao.NewSpansCkDaoImpl,
		ckdao.NewAnnotationCkDaoImpl,
		obmetrics.NewTraceMetricsImpl,
		mq2.NewTraceProducerImpl,
		mq2.NewAnnotationProducerImpl,
		file.NewFileRPCProvider,
		NewTraceConfigLoader,
		NewTraceProcessorBuilder,
		obconfig.NewTraceConfigCenter,
		tenant.NewTenantProvider,
		workspace.NewWorkspaceProvider,
		evaluator.NewEvaluatorRPCProvider,
		NewDatasetServiceAdapter,
		taskDomainSet,
	)
	traceSet = wire.NewSet(
		NewTraceApplication,
		obrepo.NewViewRepoImpl,
		mysqldao.NewViewDaoImpl,
		auth.NewAuthProvider,
		user.NewUserRPCProvider,
		tag.NewTagRPCProvider,
		traceDomainSet,
	)
	traceIngestionSet = wire.NewSet(
		NewIngestionApplication,
		service.NewIngestionServiceImpl,
		obrepo.NewTraceCKRepoImpl,
		ckdao.NewSpansCkDaoImpl,
		ckdao.NewAnnotationCkDaoImpl,
		obconfig.NewTraceConfigCenter,
		NewTraceConfigLoader,
		NewIngestionCollectorFactory,
	)
	openApiSet = wire.NewSet(
		NewOpenAPIApplication,
		auth.NewAuthProvider,
		traceDomainSet,
	)
	taskSet = wire.NewSet(
		//NewDatasetServiceAdapter,
		NewInitTaskProcessor,
		tracehub.NewTraceHubImpl,
		NewTaskApplication,
		auth.NewAuthProvider,
		user.NewUserRPCProvider,
		//evaluator.NewEvaluatorRPCProvider,
		evaluation.NewEvaluationRPCProvider,
		traceDomainSet,
	)
)

func NewTraceProcessorBuilder(
	traceConfig config.ITraceConfig,
	fileProvider rpc.IFileProvider,
	benefitSvc benefit.IBenefitService,
) service.TraceFilterProcessorBuilder {
	return service.NewTraceFilterProcessorBuilder(
		span_filter.NewPlatformFilterFactory(
			[]span_filter.Factory{
				span_filter.NewCozeLoopFilterFactory(),
				span_filter.NewPromptFilterFactory(traceConfig),
				span_filter.NewEvaluatorFilterFactory(),
				span_filter.NewEvalTargetFilterFactory(),
			}),
		// get trace processors
		[]span_processor.Factory{
			span_processor.NewPlatformProcessorFactory(traceConfig),
			span_processor.NewCheckProcessorFactory(),
			span_processor.NewAttrTosProcessorFactory(fileProvider),
			span_processor.NewExpireErrorProcessorFactory(benefitSvc),
		},
		// list spans processors
		[]span_processor.Factory{
			span_processor.NewPlatformProcessorFactory(traceConfig),
			span_processor.NewExpireErrorProcessorFactory(benefitSvc),
		},
		// batch get advance info processors
		[]span_processor.Factory{
			span_processor.NewCheckProcessorFactory(),
		},
		// ingest trace processors
		[]span_processor.Factory{},
		// search trace open api processors
		[]span_processor.Factory{
			span_processor.NewPlatformProcessorFactory(traceConfig),
			span_processor.NewCheckProcessorFactory(),
			span_processor.NewAttrTosProcessorFactory(fileProvider),
			span_processor.NewExpireErrorProcessorFactory(benefitSvc),
		},
		// list trace open api processors
		[]span_processor.Factory{
			span_processor.NewPlatformProcessorFactory(traceConfig),
			span_processor.NewExpireErrorProcessorFactory(benefitSvc),
		})
}

func NewIngestionCollectorFactory(mqFactory mq.IFactory, traceRepo repo.ITraceRepo) service.IngestionCollectorFactory {
	return service.NewIngestionCollectorFactory(
		[]receiver.Factory{
			rmqreceiver.NewFactory(mqFactory),
		},
		[]processor.Factory{
			queueprocessor.NewFactory(),
		},
		[]exporter.Factory{
			clickhouseexporter.NewFactory(traceRepo),
		},
	)
}

func NewTraceConfigLoader(confFactory conf.IConfigLoaderFactory) (conf.IConfigLoader, error) {
	return confFactory.NewConfigLoader("observability.yaml")
}

func NewDatasetServiceAdapter(evalSetService evaluationsetservice.Client, datasetService datasetservice.Client) *service.DatasetServiceAdaptor {
	adapter := service.NewDatasetServiceAdaptor()
	datasetProvider := dataset.NewDatasetProvider(datasetService)
	adapter.Register(entity.DatasetCategory_Evaluation, evaluationset.NewEvaluationSetProvider(evalSetService, datasetProvider))
	return adapter
}

func NewInitTaskProcessor(datasetServiceProvider *service.DatasetServiceAdaptor, evalService rpc.IEvaluatorRPCAdapter,
	evaluationService rpc.IEvaluationRPCAdapter, taskRepo repo.ITaskRepo, taskRunRepo repo.ITaskRunRepo) *task_processor.TaskProcessor {
	taskProcessor := task_processor.NewTaskProcessor()
	taskProcessor.Register(task.TaskTypeAutoEval, task_processor.NewAutoEvaluteProcessor(datasetServiceProvider, evalService, evaluationService, taskRepo, taskRunRepo))
	return taskProcessor
}

func InitTraceApplication(
	db db.Provider,
	ckDb ck.Provider,
	redis redis.Cmdable,
	meter metrics.Meter,
	mqFactory mq.IFactory,
	configFactory conf.IConfigLoaderFactory,
	idgen idgen.IIDGenerator,
	fileClient fileservice.Client,
	benefit benefit.IBenefitService,
	authClient authservice.Client,
	userClient userservice.Client,
	evalService evaluatorservice.Client,
	evalSetService evaluationsetservice.Client,
	tagService tagservice.Client,
	datasetService datasetservice.Client,
) (ITraceApplication, error) {
	wire.Build(traceSet)
	return nil, nil
}

func InitOpenAPIApplication(
	mqFactory mq.IFactory,
	configFactory conf.IConfigLoaderFactory,
	fileClient fileservice.Client,
	ckDb ck.Provider,
	benefit benefit.IBenefitService,
	limiterFactory limiter.IRateLimiterFactory,
	authClient authservice.Client,
	meter metrics.Meter,
	db db.Provider,
	redis redis.Cmdable,
	idgen idgen.IIDGenerator,
	evalService evaluatorservice.Client,
) (IObservabilityOpenAPIApplication, error) {
	wire.Build(openApiSet)
	return nil, nil
}

func InitTraceIngestionApplication(
	configFactory conf.IConfigLoaderFactory,
	ckDb ck.Provider,
	mqFactory mq.IFactory) (ITraceIngestionApplication, error) {
	wire.Build(traceIngestionSet)
	return nil, nil
}

func InitTaskApplication(
	db db.Provider,
	idgen idgen.IIDGenerator,
	configFactory conf.IConfigLoaderFactory,
	ckDb ck.Provider,
	redis redis.Cmdable,
	mqFactory mq.IFactory,
	userClient userservice.Client,
	authClient authservice.Client,
	evalService evaluatorservice.Client,
	evalSetService evaluationsetservice.Client,
	exptService experimentservice.Client,
	datasetService datasetservice.Client,
	benefit benefit.IBenefitService,
	fileClient fileservice.Client) (ITaskApplication, error) {
	wire.Build(taskSet)
	return nil, nil
}
