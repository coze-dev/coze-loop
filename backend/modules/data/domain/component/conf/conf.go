// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package conf

import (
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/entity"
	common_entity "github.com/coze-dev/coze-loop/backend/modules/data/domain/entity"
	entity2 "github.com/coze-dev/coze-loop/backend/modules/data/domain/tag/entity"
)

//go:generate mockgen -destination=mocks/conf.go -package=mocks . IConfig
type IConfig interface {
	GetDatasetFeature() *DatasetFeature
	GetDatasetItemStorage() *DatasetItemStorage
	GetDatasetSpec() *DatasetSpec
	GetProducerConfig() *ProducerConfig
	GetSnapshotRetry() *SnapshotRetry
	GetConsumerConfigs() *ConsumerConfig
	GetTagSpec() *TagSpec
}

type DatasetFeature struct {
	Feature           *entity.DatasetFeatures                            `mapstructure:"feature"`
	FeatureByCategory map[entity.DatasetCategory]*entity.DatasetFeatures `mapstructure:"feature_by_category"` // key: check [mdataset.DatasetCategory] enums
}

type DatasetItemStorage struct {
	Providers []*DatasetItemProviderConfig `mapstructure:"providers"`
}

type DatasetItemProviderConfig struct {
	Provider common_entity.Provider `mapstructure:"provider"`
	MaxSize  int64                  `mapstructure:"max_size"`
}

type DatasetSpec struct {
	Spec            *entity.DatasetSpec                            `mapstructure:"spec"`
	SpecsByCategory map[entity.DatasetCategory]*entity.DatasetSpec `mapstructure:"specs_by_category"` // key: check [mdataset.DatasetCategory] enums
}

type ProducerConfig struct {
	Topic          string        `mapstructure:"topic"`
	Tag            string        `mapstructure:"tag"`
	Addr           []string      `mapstructure:"addr"`
	ProduceTimeout time.Duration `mapstructure:"produce_timeout"`
	ProducerGroup  string        `mapstructure:"producer_group"`
}

type SnapshotRetry struct {
	MaxRetryTimes      int64 `mapstructure:"max_retry_times"`       // 最大重试次数
	RetryIntervalMS    int64 `mapstructure:"retry_interval_ms"`     // 重试间隔，单位 ms
	MaxProcessingTimeS int64 `mapstructure:"max_processing_time_s"` // 最大处理时长，单位 s
}

type ConsumerConfig struct {
	Addr []string `mapstructure:"addr"`
	// Topic name
	Topic string `mapstructure:"topic"`
	// Consumer group name
	ConsumerGroup string `mapstructure:"consumer_group"`
	// Whether to consume orderly
	Orderly bool `mapstructure:"orderly"`
	// Consume specific tags, such as "tag" or "tag1 || tag2 || tag3"
	TagExpression string `mapstructure:"tag_expression"`
	// Max number of messages consumed concurrently
	ConsumeGoroutineNums int `mapstructure:"consume_goroutine_nums"`
	// Timeout for consumer one message
	ConsumeTimeout time.Duration `mapstructure:"consume_timeout"`
}

type TagSpec struct {
	DefaultSpec  *entity2.TagSpec           `mapstructure:"default_spec" json:"default_spec"`
	SpecsBySpace map[int64]*entity2.TagSpec `mapstructure:"space_specs" json:"space_specs"`
}

func (s *DatasetSpec) GetSpecByCategory(category entity.DatasetCategory) *entity.DatasetSpec {
	if s == nil {
		return nil
	}
	if s, ok := s.SpecsByCategory[category]; ok {
		return s
	}
	return s.Spec
}

func (f *DatasetFeature) GetFeatureByCategory(category entity.DatasetCategory) *entity.DatasetFeatures {
	if f == nil {
		return nil
	}
	if f, ok := f.FeatureByCategory[category]; ok {
		return f
	}
	return f.Feature
}

func (c *SnapshotRetry) GetRetryInterval() time.Duration {
	if c == nil {
		return 5 * time.Second
	}
	return time.Duration(c.RetryIntervalMS) * time.Millisecond
}

func (c *SnapshotRetry) GetMaxProcessingTime() time.Duration {
	const defaultTTL = 5 * time.Minute
	if c.MaxProcessingTimeS == 0 {
		return defaultTTL
	}
	return time.Duration(c.MaxProcessingTimeS) * time.Second
}

func (t *TagSpec) GetSpecBySpace(spaceID int64) *entity2.TagSpec {
	if t == nil {
		return nil
	}
	if s, ok := t.SpecsBySpace[spaceID]; ok {
		return s
	}
	if t.DefaultSpec == nil {
		return &entity2.TagSpec{
			MaxHeight: 1,
			MaxWidth:  20,
		}
	}
	return t.DefaultSpec
}
