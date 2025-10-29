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
	"github.com/coze-dev/coze-loop/backend/infra/lock"
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
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	metrics_entity "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	metric_service "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service"
	metric_general "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/general"
	metric_model "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/model"
	metric_service_def "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/service"
	metric_tool "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/tool"
	task_entity "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	trepo "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
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
	obcollector "github.com/coze-dev/coze-loop/backend/modules/observability/infra/collector"
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
		NewInitTaskProcessor,
		taskSvc.NewTaskServiceImpl,
		obrepo.NewTaskRepoImpl,
		// obrepo.NewTaskRunRepoImpl,
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
		obcollector.NewEventCollectorProvider,
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
		tracehub.NewTraceHubImpl,
		NewTaskApplication,
		auth.NewAuthProvider,
		user.NewUserRPCProvider,
		evaluation.NewEvaluationRPCProvider,
		NewTaskLocker,
		traceDomainSet,
	)
	metricsSet = wire.NewSet(
		NewMetricApplication,
		metric_service.NewMetricsService,
		obrepo.NewTraceMetricCKRepoImpl,
		tenant.NewTenantProvider,
		auth.NewAuthProvider,
		NewTraceConfigLoader,
		NewTraceProcessorBuilder,
		obconfig.NewTraceConfigCenter,
		NewMetricDefinitions,
		ckdao.NewSpansCkDaoImpl,
		ckdao.NewAnnotationCkDaoImpl,
		file.NewFileRPCProvider,
	)
)

func NewTaskLocker(cmdable redis.Cmdable) lock.ILocker {
	return lock.NewRedisLockerWithHolder(cmdable, "observability")
}

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

func NewMetricDefinitions() []metrics_entity.IMetricDefinition {
	return []metrics_entity.IMetricDefinition{
		metric_general.NewGeneralTotalCountMetric(),
		metric_general.NewGeneralFailRatioMetric(),
		metric_general.NewGeneralModelTotalTokensMetric(),
		metric_general.NewGeneralModelLatencyMetric(),
		metric_general.NewGeneralModelFailRatioMetric(),
		metric_general.NewGeneralToolTotalCountMetric(),
		metric_general.NewGeneralToolLatencyMetric(),
		metric_general.NewGeneralToolFailRatioMetric(),

		metric_model.NewModelDurationMetric(),
		metric_model.NewModelInputTokenCountMetric(),
		metric_model.NewModelOutputTokenCountMetric(),
		metric_model.NewModelNamePieMetric(),
		metric_model.NewModelQPMAllMetric(),
		metric_model.NewModelQPMFailMetric(),
		metric_model.NewModelQPMSuccessMetric(),
		metric_model.NewModelQPSAllMetric(),
		metric_model.NewModelQPSFailMetric(),
		metric_model.NewModelQPSSuccessMetric(),
		metric_model.NewModelSuccessRatioMetric(),
		metric_model.NewModelSystemTokenCountMetric(),
		metric_model.NewModelTokenCountMetric(),
		metric_model.NewModelTokenCountPieMetric(),
		metric_model.NewModelToolChoiceTokenCountMetric(),
		metric_model.NewModelTPMMetric(),
		metric_model.NewModelTPOTMetric(),
		metric_model.NewModelTPSMetric(),
		metric_model.NewModelTTFTMetric(),

		metric_service_def.NewServiceDurationMetric(),
		metric_service_def.NewServiceExecutionStepCountMetric(),
		metric_service_def.NewServiceMessageCountMetric(),
		metric_service_def.NewServiceQPMAllMetric(),
		metric_service_def.NewServiceQPMSuccessMetric(),
		metric_service_def.NewServiceQPMFailMetric(),
		metric_service_def.NewServiceQPSAllMetric(),
		metric_service_def.NewServiceQPSSuccessMetric(),
		metric_service_def.NewServiceQPSFailMetric(),
		metric_service_def.NewServiceSpanCountMetric(),
		metric_service_def.NewServiceSuccessRatioMetric(),
		metric_service_def.NewServiceTraceCountMetric(),
		metric_service_def.NewServiceUserCountMetric(),

		metric_tool.NewToolDurationMetric(),
		metric_tool.NewToolNamePieMetric(),
		metric_tool.NewToolSuccessRatioMetric(),
		metric_tool.NewToolTotalCountMetric(),
	}
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
	evaluationService rpc.IEvaluationRPCAdapter, taskRepo trepo.ITaskRepo,
) *task_processor.TaskProcessor {
	taskProcessor := task_processor.NewTaskProcessor()
	taskProcessor.Register(task_entity.TaskTypeAutoEval, task_processor.NewAutoEvaluteProcessor(0, datasetServiceProvider, evalService, evaluationService, taskRepo))
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

func InitMetricApplication(
	ckDb ck.Provider,
	configFactory conf.IConfigLoaderFactory,
	fileClient fileservice.Client,
	benefit benefit.IBenefitService,
	authClient authservice.Client,
) (IMetricApplication, error) {
	wire.Build(metricsSet)
	return nil, nil
}

func InitTraceIngestionApplication(
	configFactory conf.IConfigLoaderFactory,
	ckDb ck.Provider,
	mqFactory mq.IFactory,
) (ITraceIngestionApplication, error) {
	wire.Build(traceIngestionSet)
	return nil, nil
}

func InitTaskApplication(
	db db.Provider,
	idgen idgen.IIDGenerator,
	configFactory conf.IConfigLoaderFactory,
	benefit benefit.IBenefitService,
	ckDb ck.Provider,
	redis redis.Cmdable,
	mqFactory mq.IFactory,
	userClient userservice.Client,
	authClient authservice.Client,
	evalService evaluatorservice.Client,
	evalSetService evaluationsetservice.Client,
	exptService experimentservice.Client,
	datasetService datasetservice.Client,
	fileClient fileservice.Client,
	taskProcessor task_processor.TaskProcessor,
	aid int32,
) (ITaskApplication, error) {
	wire.Build(taskSet)
	return nil, nil
}
