// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package component

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// EvaluationRecordStorage 评测记录大对象存储配置，与 dataset 模块的 dataset_item_storage 语义一致
type EvaluationRecordStorage struct {
	Providers []*EvaluationRecordProviderConfig `mapstructure:"providers"`
}

// EvaluationRecordProviderConfig 单个存储 Provider 配置
type EvaluationRecordProviderConfig struct {
	Provider string `mapstructure:"provider" json:"provider"`  // RDS, S3 等
	MaxSize  int64  `mapstructure:"max_size" json:"max_size"`
}

//go:generate mockgen -destination=mocks/expt_configer.go -package=mocks . IConfiger
type IConfiger interface {
	GetEvaluationRecordStorage(ctx context.Context) *EvaluationRecordStorage
	GetConsumerConf(ctx context.Context) *entity.ExptConsumerConf
	GetErrCtrl(ctx context.Context) *entity.ExptErrCtrl
	GetExptExecConf(ctx context.Context, spaceID int64) *entity.ExptExecConf
	GetErrRetryConf(ctx context.Context, spaceID int64, err error) *entity.RetryConf
	GetExptTurnResultFilterBmqProducerCfg(ctx context.Context) *entity.BmqProducerCfg
	GetCKDBName(ctx context.Context) *entity.CKDBConfig
	GetExptExportWhiteList(ctx context.Context) *entity.ExptExportWhiteList
	GetMaintainerUserIDs(ctx context.Context) map[string]bool
	GetSchedulerAbortCtrl(ctx context.Context) *entity.SchedulerAbortCtrl
	GetTargetTrajectoryConf(ctx context.Context) *entity.TargetTrajectoryConf
}
