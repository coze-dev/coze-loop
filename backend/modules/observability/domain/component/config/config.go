// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
)

type SystemView struct {
	ID           int64  `mapstructure:"id" json:"id"`
	ViewName     string `mapstructure:"view_name" json:"view_name"`
	Filters      string `mapstructure:"filters" json:"filters"`
	PlatformType string `mapstructure:"platform_type" json:"platform_type"`
	SpanListType string `mapstructure:"span_list_type" json:"span_list_type"`
}

type PlatformTenantsCfg struct {
	Config map[string][]string `mapstructure:"config" json:"config"`
}

type SpanTransHandlerConfig struct {
	PlatformCfg map[string]loop_span.SpanTransCfgList `mapstructure:"platform_cfg" json:"platform_cfg"`
}

type IngestConfig struct {
	MaxSpanLength int           `mapstructure:"max_span_length" json:"max_span_length"`
	MqProducer    MqProducerCfg `mapstructure:"mq_producer" json:"mq_producer"`
}

type MqProducerCfg struct {
	Addr          []string `mapstructure:"addr" json:"addr"`
	Timeout       int      `mapstructure:"timeout" json:"timeout"` // ms
	RetryTimes    int      `mapstructure:"retry_times" json:"retry_times"`
	Topic         string   `mapstructure:"topic" json:"topic"`
	ProducerGroup string   `mapstructure:"producer_group" json:"producer_group"`
}

type MqConsumerCfg struct {
	Addr          []string `mapstructure:"addr" json:"addr"`
	Timeout       int      `mapstructure:"timeout" json:"timeout"` // ms
	Topic         string   `mapstructure:"topic" json:"topic"`
	ConsumerGroup string   `mapstructure:"consumer_group" json:"consumer_group"`
	WorkerNum     int      `mapstructure:"worker_num" json:"worker_num"`
}

type TraceCKCfg struct {
	Hosts       []string        `mapstructure:"hosts" json:"hosts"`
	DataBase    string          `mapstructure:"database" json:"database"`
	UserName    string          `mapstructure:"username" json:"username"`
	Password    string          `mapstructure:"password" json:"password"`
	DialTimeout int             `mapstructure:"dial_timeout" json:"dial_timeout"` // seconds
	ReadTimeout int             `mapstructure:"read_timeout" json:"read_timeout"` // seconds
	SuperFields map[string]bool `mapstructure:"super_fields" json:"super_fields"`
}

type TableCfg struct {
	SpanTable string `mapstructure:"span_table" json:"span_table"`
	AnnoTable string `mapstructure:"anno_table" json:"anno_table"`
}

type TenantCfg struct {
	TenantTables             map[string]map[loop_span.TTL]TableCfg `mapstructure:"tenant_table" json:"tenant_table"`
	DefaultIngestTenant      string                                `mapstructure:"default_ingest_tenant" json:"default_ingest_tenant"`
	TenantsSupportAnnotation map[string]bool                       `mapstructure:"tenants_support_annotation" json:"tenants_support_annotation"`
}

type FieldMeta struct {
	FieldType     loop_span.FieldType       `mapstructure:"field_type" json:"field_type"`
	FilterTypes   []loop_span.QueryTypeEnum `mapstructure:"filter_types" json:"filter_types"`
	FieldOptions  *loop_span.FieldOptions   `mapstructure:"field_options" json:"field_options"`
	SupportCustom bool                      `mapstructure:"support_custom" json:"support_custom"`
}

type TraceAttrTosCfg struct {
	Template   string `mapstructure:"template" json:"template"`
	Format     string `mapstructure:"format" json:"format"`
	Expiration int    `mapstructure:"ttl" json:"ttl"` // seconds
}

// AvailableFields: 配置可查询的Tag
// FieldMetas定义不同场景可使用的Key
type TraceFieldMetaInfoCfg struct {
	AvailableFields map[string]*FieldMeta                                          `mapstructure:"available_fields" json:"available_fields"`
	FieldMetas      map[loop_span.PlatformType]map[loop_span.SpanListType][]string `mapstructure:"field_metas" json:"field_metas"`
}

type AnnotationSourceConfig struct {
	SourceCfg map[string]AnnotationConfig `mapstructure:"source_cfg" json:"source_cfg"`
}

type AnnotationConfig struct {
	Tenants        []string `mapstructure:"tenant" json:"tenant"`
	AnnotationType string   `mapstructure:"annotation_type" json:"annotation_type"`
}

type QueryTraceRateLimitConfig struct {
	DefaultMaxQPS int            `mapstructure:"default_max_qps" json:"default_max_qps"`
	SpaceMaxQPS   map[string]int `mapstructure:"space_max_qps" json:"space_max_qps"`
}

//go:generate mockgen -destination=mocks/config.go -package=mocks . ITraceConfig
type ITraceConfig interface {
	GetSystemViews(ctx context.Context) ([]*SystemView, error)
	GetPlatformTenants(ctx context.Context) (*PlatformTenantsCfg, error)
	GetPlatformSpansTrans(ctx context.Context) (*SpanTransHandlerConfig, error)
	GetTraceIngestTenantProducerCfg(ctx context.Context) (map[string]*IngestConfig, error)
	GetAnnotationMqProducerCfg(ctx context.Context) (*MqProducerCfg, error)
	GetTraceCkCfg(ctx context.Context) (*TraceCKCfg, error)
	GetTenantConfig(ctx context.Context) (*TenantCfg, error)
	GetTraceFieldMetaInfo(ctx context.Context) (*TraceFieldMetaInfoCfg, error)
	GetTraceDataMaxDurationDay(ctx context.Context, platformType *string) int64
	GetDefaultTraceTenant(ctx context.Context) string
	GetAnnotationSourceCfg(ctx context.Context) (*AnnotationSourceConfig, error)
	GetQueryMaxQPSBySpace(ctx context.Context, workspaceID int64) (int, error)

	conf.IConfigLoader
}
