// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package conf

import (
	"context"

	"github.com/samber/lo"

	dataconf "github.com/coze-dev/coze-loop/backend/modules/data/domain/component/conf"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/consts"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
)

func NewConfigerFactory(configerFactory conf.IConfigLoaderFactory) (conf.IConfigLoader, error) {
	return configerFactory.NewConfigLoader(consts.DataConfigFileName)
}

func NewConfiger(configLoader conf.IConfigLoader) dataconf.IConfig {
	return &configer{
		loader: configLoader,
	}
}

type configer struct {
	loader conf.IConfigLoader
}

func (c *configer) GetConsumerConfigs() *dataconf.ConsumerConfig {
	const key = "consumer_configs"
	var conf *dataconf.ConsumerConfig
	return lo.Ternary(c.loader.UnmarshalKey(context.Background(), key, &conf) == nil, conf, &dataconf.ConsumerConfig{})
}

func (c *configer) GetSnapshotRetry() *dataconf.SnapshotRetry {
	const key = "snapshot_retry"
	var conf *dataconf.SnapshotRetry
	return lo.Ternary(c.loader.UnmarshalKey(context.Background(), key, &conf) == nil, conf, &dataconf.SnapshotRetry{})
}

func (c *configer) GetProducerConfig() *dataconf.ProducerConfig {
	const key = "job_mq_producer"
	var conf *dataconf.ProducerConfig
	return lo.Ternary(c.loader.UnmarshalKey(context.Background(), key, &conf) == nil, conf, &dataconf.ProducerConfig{})
}

func (c *configer) GetDatasetFeature() *dataconf.DatasetFeature {
	const key = "default_dataset_feature"
	var conf *dataconf.DatasetFeature
	return lo.Ternary(c.loader.UnmarshalKey(context.Background(), key, &conf) == nil, conf, &dataconf.DatasetFeature{})
}

func (c *configer) GetDatasetItemStorage() *dataconf.DatasetItemStorage {
	const key = "dataset_item_storage"
	var conf *dataconf.DatasetItemStorage
	return lo.Ternary(c.loader.UnmarshalKey(context.Background(), key, &conf) == nil, conf, &dataconf.DatasetItemStorage{})
}

func (c *configer) GetDatasetSpec() *dataconf.DatasetSpec {
	const key = "default_dataset_spec"
	var conf *dataconf.DatasetSpec
	return lo.Ternary(c.loader.UnmarshalKey(context.Background(), key, &conf) == nil, conf, &dataconf.DatasetSpec{})
}

func (c *configer) GetTagSpec() *dataconf.TagSpec {
	const key = "default_tag_spec"
	var conf *dataconf.TagSpec
	return lo.Ternary(c.loader.UnmarshalKey(context.Background(), key, &conf) == nil, conf, &dataconf.TagSpec{})
}
