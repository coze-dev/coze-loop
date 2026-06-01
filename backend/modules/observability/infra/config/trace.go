// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

const (
	systemViewsCfgKey                  = "trace_system_view_cfg"
	platformTenantCfgKey               = "trace_platform_tenants"
	platformSpanHandlerCfgKey          = "trace_platform_span_handler_config"
	traceIngestTenantCfgKey            = "trace_ingest_tenant_config"
	annotationMqProducerCfgKey         = "annotation_mq_producer_config"
	spanWithAnnotationMqProducerCfgKey = "span_with_annotation_mq_producer_config"
	tenantTablesCfgKey                 = "trace_tenant_cfg"
	traceCkCfgKey                      = "trace_ck_cfg"
	traceFieldMetaInfoCfgKey           = "trace_field_meta_info"
	traceMaxDurationDay                = "trace_max_duration_day"
	annotationSourceCfgKey             = "annotation_source_cfg"
	queryTraceRateLimitCfgKey          = "query_trace_rate_limit_config"
	keySpanTypeCfgKey                  = "key_span_type"
	backfillMqProducerCfgKey           = "backfill_mq_producer_config"
	consumerListeningCfgKey            = "consumer_listening"
	metricPlatformTenantCfgKey         = "metric_platform_tenants"
	metricQueryConfigKey               = "metric_query_config"
	backfillCfgKey                     = "backfill_config"
	reflowInsertCfgKey                 = "reflow_insert_config"
	searchTraceTreeMaxSpanLimitCfgKey  = "search_trace_tree_max_span_limit"
	trajectoryMetadataCfgKey           = "trajectory_metadata_config"

	defaultBackfillDispatchBatchSize   = 10
	defaultBackfillDispatchIntervalMs  = 1000
	defaultBackfillCkQueryLimit        = 100
	defaultEvalSetInvokeBatchSize      = 1
	defaultDatasetInvokeBatchSize      = 1
	defaultSearchTraceTreeMaxSpanLimit = 10000
)

type TraceConfigCenter struct {
	conf.IConfigLoader
	// glocal config, just in case
	traceDefaultTenant string
}

func (t *TraceConfigCenter) GetSystemViews(ctx context.Context) ([]*config.SystemView, error) {
	systemViews := make([]*config.SystemView, 0)
	if err := t.UnmarshalKey(ctx, systemViewsCfgKey, &systemViews); err != nil {
		return nil, err
	}
	return systemViews, nil
}

func (t *TraceConfigCenter) GetPlatformTenants(ctx context.Context) (*config.PlatformTenantsCfg, error) {
	cfg := new(config.PlatformTenantsCfg)
	if err := t.UnmarshalKey(ctx, platformTenantCfgKey, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (t *TraceConfigCenter) GetPlatformSpansTrans(ctx context.Context) (*config.SpanTransHandlerConfig, error) {
	cfg := new(config.SpanTransHandlerConfig)
	if err := t.UnmarshalKey(ctx, platformSpanHandlerCfgKey, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (t *TraceConfigCenter) GetTraceIngestTenantProducerCfg(ctx context.Context) (map[string]*config.IngestConfig, error) {
	cfg := make(map[string]*config.IngestConfig)
	if err := t.UnmarshalKey(context.Background(), traceIngestTenantCfgKey, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (t *TraceConfigCenter) GetAnnotationMqProducerCfg(ctx context.Context) (*config.MqProducerCfg, error) {
	cfg := new(config.MqProducerCfg)
	if err := t.UnmarshalKey(context.Background(), annotationMqProducerCfgKey, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (t *TraceConfigCenter) GetSpanWithAnnotationMqProducerCfg(ctx context.Context) (*config.MqProducerCfg, error) {
	cfg := new(config.MqProducerCfg)
	if err := t.UnmarshalKey(context.Background(), spanWithAnnotationMqProducerCfgKey, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (t *TraceConfigCenter) GetBackfillMqProducerCfg(ctx context.Context) (*config.MqProducerCfg, error) {
	cfg := new(config.MqProducerCfg)
	if err := t.UnmarshalKey(context.Background(), backfillMqProducerCfgKey, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (t *TraceConfigCenter) GetTraceCkCfg(ctx context.Context) (*config.TraceCKCfg, error) {
	cfg := new(config.TraceCKCfg)
	if err := t.UnmarshalKey(context.Background(), traceCkCfgKey, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (t *TraceConfigCenter) GetTenantConfig(ctx context.Context) (*config.TenantCfg, error) {
	tenantTableCfg := new(config.TenantCfg)
	if err := t.UnmarshalKey(ctx, tenantTablesCfgKey, &tenantTableCfg); err != nil {
		return nil, err
	}
	return tenantTableCfg, nil
}

func (t *TraceConfigCenter) GetTraceFieldMetaInfo(ctx context.Context) (*config.TraceFieldMetaInfoCfg, error) {
	traceFieldMetaInfoCfg := new(config.TraceFieldMetaInfoCfg)
	if err := t.UnmarshalKey(ctx, traceFieldMetaInfoCfgKey, &traceFieldMetaInfoCfg); err != nil {
		return nil, err
	}
	return traceFieldMetaInfoCfg, nil
}

func (t *TraceConfigCenter) GetTraceDataMaxDurationDay(ctx context.Context, platformPtr *string) int64 {
	defaultDuration := int64(7)
	var platformType string
	if platformPtr == nil {
		platformType = "default"
	} else {
		platformType = *platformPtr
	}
	mp := make(map[string]int64)
	err := t.UnmarshalKey(ctx, traceMaxDurationDay, &mp)
	if err != nil {
		logs.CtxWarn(ctx, "fail to unmarshal max duration cfg, %v", err)
		return defaultDuration
	}
	if mp[platformType] > 0 {
		return mp[platformType]
	} else {
		return defaultDuration
	}
}

func (t *TraceConfigCenter) GetDefaultTraceTenant(ctx context.Context) string {
	return t.traceDefaultTenant
}

func (t *TraceConfigCenter) getDefaultTraceTenant(ctx context.Context) (string, error) {
	if t.traceDefaultTenant != "" {
		return t.traceDefaultTenant, nil
	}
	cfg, err := t.GetTenantConfig(ctx)
	if err != nil {
		return "", err
	} else if cfg.DefaultIngestTenant == "" {
		return "", fmt.Errorf("default trace tenant not exist")
	}
	return cfg.DefaultIngestTenant, nil
}

func (t *TraceConfigCenter) GetAnnotationSourceCfg(ctx context.Context) (*config.AnnotationSourceConfig, error) {
	annotationSourceCfg := new(config.AnnotationSourceConfig)
	if err := t.UnmarshalKey(ctx, annotationSourceCfgKey, &annotationSourceCfg); err != nil {
		return nil, err
	}
	return annotationSourceCfg, nil
}

func (t *TraceConfigCenter) GetQueryMaxQPS(ctx context.Context, key string) (int, error) {
	qpsConfig := new(config.QueryTraceRateLimitConfig)
	if err := t.UnmarshalKey(ctx, queryTraceRateLimitCfgKey, &qpsConfig); err != nil {
		return 0, err
	}
	if qps, ok := qpsConfig.SpaceMaxQPS[key]; ok {
		return qps, nil
	}
	return qpsConfig.DefaultMaxQPS, nil
}

func (t *TraceConfigCenter) GetKeySpanTypes(ctx context.Context) map[string][]string {
	keyColumns := make(map[string][]string)
	if err := t.UnmarshalKey(ctx, keySpanTypeCfgKey, &keyColumns); err != nil {
		return keyColumns
	}
	return keyColumns
}

func (t *TraceConfigCenter) GetConsumerListening(ctx context.Context) (*config.ConsumerListening, error) {
	consumerListening := new(config.ConsumerListening)
	if err := t.UnmarshalKey(ctx, consumerListeningCfgKey, &consumerListening); err != nil {
		return nil, err
	}
	return consumerListening, nil
}

func (t *TraceConfigCenter) GetMetricPlatformTenants(ctx context.Context) (*config.PlatformTenantsCfg, error) {
	cfg := new(config.PlatformTenantsCfg)
	if err := t.UnmarshalKey(ctx, metricPlatformTenantCfgKey, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (t *TraceConfigCenter) GetMetricQueryConfig(ctx context.Context) *config.MetricQueryConfig {
	cfg := new(config.MetricQueryConfig)
	if err := t.UnmarshalKey(ctx, metricQueryConfigKey, cfg); err != nil {
		logs.CtxWarn(ctx, "fail to get metric query cfg, %v", err)
		return &config.MetricQueryConfig{
			SupportOffline: false,
		}
	}
	return cfg
}

func (t *TraceConfigCenter) GetBackfillConfig(ctx context.Context) *config.BackfillConfig {
	cfg := &config.BackfillConfig{}
	if err := t.UnmarshalKey(ctx, backfillCfgKey, cfg); err != nil {
		logs.CtxWarn(ctx, "fail to get backfill config, %v", err)
		return &config.BackfillConfig{
			DispatchBatchSize:  config.SpaceAwareParam[int]{Default: defaultBackfillDispatchBatchSize},
			DispatchIntervalMs: config.SpaceAwareParam[int]{Default: defaultBackfillDispatchIntervalMs},
			CkQueryLimit:       config.SpaceAwareParam[int]{Default: defaultBackfillCkQueryLimit},
		}
	}
	if cfg.DispatchBatchSize.Default <= 0 {
		cfg.DispatchBatchSize.Default = defaultBackfillDispatchBatchSize
	}
	if cfg.DispatchIntervalMs.Default < 0 {
		cfg.DispatchIntervalMs.Default = defaultBackfillDispatchIntervalMs
	}
	if cfg.CkQueryLimit.Default <= 0 {
		cfg.CkQueryLimit.Default = defaultBackfillCkQueryLimit
	}
	return cfg
}

func (t *TraceConfigCenter) GetReflowInsertConfig(ctx context.Context) *config.ReflowInsertConfig {
	cfg := &config.ReflowInsertConfig{}
	if err := t.UnmarshalKey(ctx, reflowInsertCfgKey, cfg); err != nil {
		logs.CtxWarn(ctx, "fail to get reflow insert config, %v", err)
		return &config.ReflowInsertConfig{
			EvalSetInvokeBatchSize: config.SpaceAwareParam[int]{Default: defaultEvalSetInvokeBatchSize},
			DatasetInvokeBatchSize: config.SpaceAwareParam[int]{Default: defaultDatasetInvokeBatchSize},
		}
	}
	if cfg.EvalSetInvokeBatchSize.Default <= 0 {
		cfg.EvalSetInvokeBatchSize.Default = defaultEvalSetInvokeBatchSize
	}
	if cfg.DatasetInvokeBatchSize.Default <= 0 {
		cfg.DatasetInvokeBatchSize.Default = defaultDatasetInvokeBatchSize
	}
	return cfg
}

func (t *TraceConfigCenter) GetTrajectoryMetadataConfig(ctx context.Context) config.TrajectoryMetadataConfig {
	cfg := make(config.TrajectoryMetadataConfig)
	if err := t.UnmarshalKey(ctx, trajectoryMetadataCfgKey, &cfg); err != nil {
		logs.CtxWarn(ctx, "fail to unmarshal trajectory metadata cfg, %v", err)
		return nil
	}
	return cfg
}

func (t *TraceConfigCenter) GetSearchTraceTreeMaxSpanLimit(ctx context.Context, workspaceID int64) int32 {
	cfg := &config.SpaceAwareParam[int32]{}
	if err := t.UnmarshalKey(ctx, searchTraceTreeMaxSpanLimitCfgKey, cfg); err != nil {
		logs.CtxWarn(ctx, "fail to get search trace tree max span limit config, %v", err)
		return defaultSearchTraceTreeMaxSpanLimit
	}
	limit := cfg.Get(workspaceID)
	if limit <= 0 {
		return defaultSearchTraceTreeMaxSpanLimit
	}
	return limit
}

func NewTraceConfigCenter(confP conf.IConfigLoader) config.ITraceConfig {
	ret := &TraceConfigCenter{
		IConfigLoader: confP,
	}
	tenant, err := ret.getDefaultTraceTenant(context.Background())
	if err != nil {
		panic(err)
	}
	logs.Info("default trace ingest tenant is %s", tenant)
	ret.traceDefaultTenant = tenant
	return ret
}
