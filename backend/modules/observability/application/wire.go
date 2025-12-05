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
	mq3 "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/mq"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/scheduledtask"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/storage"
	metrics_entity "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	metric_repo "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/repo"
	metric_service "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service"
	metric_agent "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/agent"
	metric_general "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/general"
	metric_model "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/model"
	metric_service_def "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/service"
	metric_tool "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/tool"
	task_entity "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	trepo "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
	taskSvc "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service"
	task_processor "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe/processor"
	taskst "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe/scheduledtask"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe/tracehub"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/exporter"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/processor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/receiver"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
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
	redis2 "github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/redis"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/rpc/auth"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/rpc/dataset"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/rpc/evaluation"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/rpc/evaluationset"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/rpc/evaluator"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/rpc/file"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/rpc/tag"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/rpc/user"
	obstorage "github.com/coze-dev/coze-loop/backend/modules/observability/infra/storage"
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
		redis2.NewTaskDAO,
		redis2.NewTaskRunDAO,
		mysqldao.NewTaskRunDaoImpl,
		mq2.NewBackfillProducerImpl,
		NewScheduledTask,
	)
	traceDomainSet = wire.NewSet(
		service.NewTraceServiceImpl,
		service.NewTraceExportServiceImpl,
		provideTraceRepo,
		obstorage.NewTraceStorageProvider,
		obmetrics.NewTraceMetricsImpl,
		obcollector.NewEventCollectorProvider,
		mq2.NewTraceProducerImpl,
		mq2.NewAnnotationProducerImpl,
		mq2.NewSpanWithAnnotationProducerImpl,
		file.NewFileRPCProvider,
		NewTraceConfigLoader,
		NewTraceProcessorBuilder,
		obconfig.NewTraceConfigCenter,
		tenant.NewTenantProvider,
		workspace.NewWorkspaceProvider,
		evaluator.NewEvaluatorRPCProvider,
		NewDatasetServiceAdapter,
		redis2.NewSpansRedisDaoImpl,
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
		provideTraceRepo,
		obconfig.NewTraceConfigCenter,
		NewTraceConfigLoader,
		NewIngestionCollectorFactory,
		mq2.NewSpanWithAnnotationProducerImpl,
		redis2.NewSpansRedisDaoImpl,
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
		taskSvc.NewTaskCallbackServiceImpl,
	)
	metricsSet = wire.NewSet(
		NewMetricApplication,
		metric_service.NewMetricsService,
		provideTraceMetricRepo,
		obrepo.NewOfflineMetricRepoImpl,
		tenant.NewTenantProvider,
		auth.NewAuthProvider,
		NewTraceConfigLoader,
		NewTraceProcessorBuilder,
		obconfig.NewTraceConfigCenter,
		ckdao.NewOfflineMetricDaoImpl,
		file.NewFileRPCProvider,
		NewMetricsPlatformConfig,
	)
)

func provideTraceRepo(
	traceConfig config.ITraceConfig,
	storageProvider storage.IStorageProvider,
	spanRedisDao redis2.ISpansRedisDao,
	ckProvider ck.Provider,
	spanProducer mq3.ISpanProducer,
) (repo.ITraceRepo, error) {
	options, err := buildTraceRepoOptions(ckProvider)
	if err != nil {
		return nil, err
	}
	return obrepo.NewTraceRepoImpl(traceConfig, storageProvider, spanRedisDao, spanProducer, options...)
}

func provideTraceMetricRepo(
	traceConfig config.ITraceConfig,
	storageProvider storage.IStorageProvider,
	ckProvider ck.Provider,
) (metric_repo.IMetricRepo, error) {
	options, err := buildTraceRepoOptions(ckProvider)
	if err != nil {
		return nil, err
	}
	return obrepo.NewTraceMetricCKRepoImpl(traceConfig, storageProvider, options...)
}

func buildTraceRepoOptions(ckProvider ck.Provider) ([]obrepo.TraceRepoOption, error) {
	ckSpanDao, err := ckdao.NewSpansCkDaoImpl(ckProvider)
	if err != nil {
		return nil, err
	}
	ckAnnoDao, err := ckdao.NewAnnotationCkDaoImpl(ckProvider)
	if err != nil {
		return nil, err
	}
	return []obrepo.TraceRepoOption{
		obrepo.WithTraceStorageDaos(ckdao.TraceStorageTypeCK, ckSpanDao, ckAnnoDao),
	}, nil
}

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

func NewMetricsPlatformConfig() *metrics_entity.PlatformMetrics {
	return &metrics_entity.PlatformMetrics{
		DrillDownObjects: map[string]*loop_span.FilterField{
			"model_id": &loop_span.FilterField{
				FieldName: "model_name",
				FieldType: loop_span.FieldTypeString,
			},
			"span_name": &loop_span.FilterField{
				FieldName: loop_span.SpanFieldSpanName,
				FieldType: loop_span.FieldTypeString,
			},
			"status_code": &loop_span.FilterField{
				FieldName: loop_span.SpanFieldStatusCode,
				FieldType: loop_span.FieldTypeLong,
			},
		},
		MetricGroups: map[string]*metrics_entity.MetricGroup{
			"all": {
				MetricDefinitions: []metrics_entity.IMetricDefinition{
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
					metric_model.NewModelTotalCountPieMetric(),
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
					metric_model.NewModelTotalCountMetric(),
					metric_model.NewModelTotalSuccessCountMetric(),
					metric_model.NewModelTotalErrorCountMetricc(),

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
					metric_service_def.NewServiceSpanErrorCountMetric(),
					metric_service_def.NewServiceSuccessRatioMetric(),
					metric_service_def.NewServiceTraceCountMetric(),
					metric_service_def.NewServiceTraceSuccessCountMetric(),
					metric_service_def.NewServiceTraceErrorCountMetric(),
					metric_service_def.NewServiceUserCountMetric(),

					metric_tool.NewToolDurationMetric(),
					metric_tool.NewToolSuccessRatioMetric(),
					metric_tool.NewToolTotalCountMetric(),
					metric_tool.NewToolTotalCountPieMetric(),
					metric_tool.NewToolTotalSuccessCountMetric(),
					metric_tool.NewToolTotalErrorCountMetric(),

					metric_agent.NewAgentExecutionStepAvgMetric(),
					metric_agent.NewAgentToolExecutionStepAvgMetric(),
					metric_agent.NewAgentModelExecutionStepAvgMetric(),
				},
			},
		},
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
	taskProcessor.Register(task_entity.TaskTypeAutoEval, task_processor.NewAutoEvaluateProcessor(
		0, datasetServiceProvider, evalService, evaluationService, taskRepo, &task_processor.EvalTargetBuilderImpl{}))
	return taskProcessor
}

func NewScheduledTask(
	locker lock.ILocker,
	config config.ITraceConfig,
	traceHubService tracehub.ITraceHubService,
	taskService taskSvc.ITaskService,
	taskProcessor task_processor.TaskProcessor,
	taskRepo trepo.ITaskRepo,
) []scheduledtask.ScheduledTask {
	return []scheduledtask.ScheduledTask{
		taskst.NewStatusCheckTask(locker, config, traceHubService, taskService, taskProcessor, taskRepo),
		taskst.NewLocalCacheRefreshTask(traceHubService, taskRepo),
	}
}

func InitTraceApplication(
	db db.Provider,
	ckDb ck.Provider,
	redis redis.Cmdable,
	persistentCmdable redis.PersistentCmdable,
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
	persistentCmdable redis.PersistentCmdable,
) (IObservabilityOpenAPIApplication, error) {
	wire.Build(openApiSet)
	return nil, nil
}

func InitMetricApplication(
	ckDb ck.Provider,
	storageProvider storage.IStorageProvider,
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
	storageProvider storage.IStorageProvider,
	ckDb ck.Provider,
	mqFactory mq.IFactory,
	persistentCmdable redis.PersistentCmdable,
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
	persistentCmdable redis.PersistentCmdable,
) (ITaskApplication, error) {
	wire.Build(taskSet)
	return nil, nil
}
