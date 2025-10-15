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
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/dataset/datasetservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/tag/tagservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/evaluationsetservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/evaluatorservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/auth/authservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/file/fileservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/user/userservice"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	metrics_entity "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	metric_service "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/general"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/model"
	service_metric "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/service"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/tool"
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
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/rpc/auth"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/rpc/dataset"
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
		NewDatasetServiceAdapter,
	)
	traceSet = wire.NewSet(
		NewTraceApplication,
		obrepo.NewViewRepoImpl,
		mysqldao.NewViewDaoImpl,
		auth.NewAuthProvider,
		user.NewUserRPCProvider,
		tag.NewTagRPCProvider,
		evaluator.NewEvaluatorRPCProvider,
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
		// General 指标概览
		general.NewGeneralTotalCountMetric(),
		general.NewGeneralFailRatioMetric(),
		general.NewGeneralModelFailRatioMetric(),
		general.NewGeneralModelLatencyAvgMetric(),
		general.NewGeneralModelTotalTokensMetric(),
		general.NewGeneralToolTotalCountMetric(),
		general.NewGeneralToolFailRatioMetric(),
		general.NewGeneralToolLatencyAvgMetric(),
		// Model 模型统计指标
		model.NewModelTokenCountMetric(),
		model.NewModelInputTokenCountMetric(),
		model.NewModelOutputTokenCountMetric(),
		model.NewModelQPSMetric(),
		model.NewModelQPMMetric(),
		model.NewModelSuccessRatioMetric(),
		model.NewModelTPSAvgMetric(),
		model.NewModelTPSMinMetric(),
		model.NewModelTPSMaxMetric(),
		model.NewModelTPSPct50Metric(),
		model.NewModelTPSPct90Metric(),
		model.NewModelTPSPct99Metric(),
		model.NewModelTPMAvgMetric(),
		model.NewModelTPMMinMetric(),
		model.NewModelTPMMaxMetric(),
		model.NewModelTPMPct50Metric(),
		model.NewModelTPMPct90Metric(),
		model.NewModelTPMPct99Metric(),
		model.NewModelDurationAvgMetric(),
		model.NewModelDurationMinMetric(),
		model.NewModelDurationMaxMetric(),
		model.NewModelDurationPct50Metric(),
		model.NewModelDurationPct90Metric(),
		model.NewModelDurationPct99Metric(),
		model.NewModelTTFTAvgMetric(),
		model.NewModelTTFTMinMetric(),
		model.NewModelTTFTMaxMetric(),
		model.NewModelTTFTPct50Metric(),
		model.NewModelTTFTPct90Metric(),
		model.NewModelTTFTPct99Metric(),
		model.NewModelTPOTAvgMetric(),
		model.NewModelTPOTMinMetric(),
		model.NewModelTPOTMaxMetric(),
		model.NewModelTPOTPct50Metric(),
		model.NewModelTPOTPct90Metric(),
		model.NewModelTPOTPct99Metric(),
		model.NewModelNamePieMetric(),
		// Tool 工具统计指标
		tool.NewToolTotalCountMetric(),
		tool.NewToolDurationAvgMetric(),
		tool.NewToolDurationMinMetric(),
		tool.NewToolDurationMaxMetric(),
		tool.NewToolDurationPct50Metric(),
		tool.NewToolDurationPct90Metric(),
		tool.NewToolDurationPct99Metric(),
		tool.NewToolSuccessRatioMetric(),
		tool.NewToolNamePieMetric(),
		// Service 服务调用指标
		service_metric.NewServiceTraceCountTotalMetric(),
		service_metric.NewServiceTraceCountMetric(),
		service_metric.NewServiceSpanCountMetric(),
		service_metric.NewServiceUserCountMetric(),
		service_metric.NewServiceMessageCountMetric(),
		service_metric.NewServiceQPSAllMetric(),
		service_metric.NewServiceQPSSuccessMetric(),
		service_metric.NewServiceQPSFailMetric(),
		service_metric.NewServiceQPMAllMetric(),
		service_metric.NewServiceQPMSuccessMetric(),
		service_metric.NewServiceQPMFailMetric(),
		service_metric.NewServiceDurationAvgMetric(),
		service_metric.NewServiceDurationMinMetric(),
		service_metric.NewServiceDurationMaxMetric(),
		service_metric.NewServiceDurationPct50Metric(),
		service_metric.NewServiceDurationPct90Metric(),
		service_metric.NewServiceDurationPct99Metric(),
		service_metric.NewServiceSuccessRatioMetric(),
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

func InitTraceApplication(
	db db.Provider,
	ckDb ck.Provider,
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
	mqFactory mq.IFactory) (ITraceIngestionApplication, error) {
	wire.Build(traceIngestionSet)
	return nil, nil
}
