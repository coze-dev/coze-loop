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
	Table  string              `mapstructure:"table" json:"table"`
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
	EnablePPE     *bool    `mapstructure:"enable_ppe" json:"enable_ppe"`
	IsEnabled     *bool    `mapstructure:"is_enabled" json:"is_enabled"`
	TagExpression *string  `mapstructure:"tag_expression" json:"tag_expression"`
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

type ConsumerListening struct {
	IsEnabled  bool     `json:"is_enabled"`
	Clusters   []string `json:"clusters"`
	IsAllSpace bool     `json:"is_all_space"`
	SpaceList  []int64  `json:"space_list"`
}

type MetricQueryConfig struct {
	SupportOffline       bool                    `mapstructure:"support_offline" json:"support_offline"`
	OfflineCriticalPoint int                     `mapstructure:"offline_critical_point" json:"offline_critical_point"`
	SpaceConfigs         map[string]*SpaceConfig `mapstructure:"space_configs" json:"space_configs"`
}

type SpaceConfig struct {
	DisableQuery bool `mapstructure:"disable_query" json:"disable_query"`
}

// SpaceAwareParam 支持按 workspace_id 获取特定值的配置参数
// Default 为兜底值，Overrides 为指定 workspace 的覆盖值
type SpaceAwareParam[T any] struct {
	Default   T           `mapstructure:"default" json:"default"`
	Overrides map[int64]T `mapstructure:"overrides" json:"overrides"`
}

// Get 根据 workspaceID 获取配置值，优先取 Overrides，未命中则返回 Default
func (p *SpaceAwareParam[T]) Get(workspaceID int64) T {
	if p.Overrides != nil {
		if v, ok := p.Overrides[workspaceID]; ok {
			return v
		}
	}
	return p.Default
}

type BackfillConfig struct {
	// DispatchBatchSize 每批分发处理的 span 条数
	DispatchBatchSize SpaceAwareParam[int] `mapstructure:"dispatch_batch_size" json:"dispatch_batch_size"`
	// DispatchIntervalMs 每批分发之间的休眠间隔（毫秒）
	DispatchIntervalMs SpaceAwareParam[int] `mapstructure:"dispatch_interval_ms" json:"dispatch_interval_ms"`
	// CkQueryLimit ClickHouse 分页查询每页大小
	CkQueryLimit SpaceAwareParam[int] `mapstructure:"ck_query_limit" json:"ck_query_limit"`
	// BatchDispatchGray 批量分发灰度配置
	BatchDispatchGray *BatchGrayConfig `mapstructure:"batch_dispatch_gray" json:"batch_dispatch_gray"`
}

// GetDispatchBatchSize 获取指定 workspace 的分发批大小
func (c *BackfillConfig) GetDispatchBatchSize(workspaceID int64) int {
	return c.DispatchBatchSize.Get(workspaceID)
}

// GetDispatchIntervalMs 获取指定 workspace 的分发间隔
func (c *BackfillConfig) GetDispatchIntervalMs(workspaceID int64) int {
	return c.DispatchIntervalMs.Get(workspaceID)
}

// GetCkQueryLimit 获取指定 workspace 的 CK 查询页大小
func (c *BackfillConfig) GetCkQueryLimit(workspaceID int64) int {
	return c.CkQueryLimit.Get(workspaceID)
}

// BatchGrayConfig 批量处理灰度开关配置
type BatchGrayConfig struct {
	// EnableAll 全开开关，为 true 时所有 workspace 走批量逻辑
	EnableAll bool `mapstructure:"enable_all" json:"enable_all"`
	// Whitelist workspace_id 白名单，在白名单中的无条件走批量逻辑
	Whitelist []int64 `mapstructure:"whitelist" json:"whitelist"`
	// Percentage 百分比灰度（0-100），按 workspace_id 哈希取模判断
	Percentage int `mapstructure:"percentage" json:"percentage"`
}

// IsBatchEnabled 判断指定 workspaceID 是否命中批量分发灰度
func (c *BackfillConfig) IsBatchEnabled(workspaceID int64) bool {
	if c.BatchDispatchGray == nil {
		return false
	}
	if c.BatchDispatchGray.EnableAll {
		return true
	}
	for _, wid := range c.BatchDispatchGray.Whitelist {
		if wid == workspaceID {
			return true
		}
	}
	if c.BatchDispatchGray.Percentage > 0 {
		return int(workspaceID%100) < c.BatchDispatchGray.Percentage
	}
	return false
}

// ReflowInsertConfig 数据回流过程中新增数据的批大小配置
type ReflowInsertConfig struct {
	// EvalSetInvokeBatchSize 每次调用下游评测集服务插入的数据行数
	EvalSetInvokeBatchSize SpaceAwareParam[int] `mapstructure:"eval_set_invoke_batch_size" json:"eval_set_invoke_batch_size"`
	// DatasetInvokeBatchSize 每次调用下游数据集服务插入的数据行数
	DatasetInvokeBatchSize SpaceAwareParam[int] `mapstructure:"dataset_invoke_batch_size" json:"dataset_invoke_batch_size"`
}

// GetEvalSetInvokeBatchSize 获取指定 workspace 的评测集插入批大小
func (c *ReflowInsertConfig) GetEvalSetInvokeBatchSize(workspaceID int64) int {
	return c.EvalSetInvokeBatchSize.Get(workspaceID)
}

// GetDatasetInvokeBatchSize 获取指定 workspace 的数据集插入批大小
func (c *ReflowInsertConfig) GetDatasetInvokeBatchSize(workspaceID int64) int {
	return c.DatasetInvokeBatchSize.Get(workspaceID)
}

// TrajectoryMetadataConfig 轨迹 metadata 写入配置
// key 为 workspace_id，value 为该 workspace 允许写入的 metadata key 规则列表
type TrajectoryMetadataConfig map[int64][]loop_span.MetaKeyRule

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
	GetQueryMaxQPS(ctx context.Context, key string) (int, error)
	GetKeySpanTypes(ctx context.Context) map[string][]string
	GetBackfillMqProducerCfg(ctx context.Context) (*MqProducerCfg, error)
	GetConsumerListening(ctx context.Context) (*ConsumerListening, error)
	GetSpanWithAnnotationMqProducerCfg(ctx context.Context) (*MqProducerCfg, error)
	GetMetricPlatformTenants(ctx context.Context) (*PlatformTenantsCfg, error)
	GetMetricQueryConfig(ctx context.Context) *MetricQueryConfig
	GetBackfillConfig(ctx context.Context) *BackfillConfig
	GetReflowInsertConfig(ctx context.Context) *ReflowInsertConfig
	GetSearchTraceTreeMaxSpanLimit(ctx context.Context, workspaceID int64) int32
	GetTrajectoryMetadataConfig(ctx context.Context) TrajectoryMetadataConfig

	conf.IConfigLoader
}
