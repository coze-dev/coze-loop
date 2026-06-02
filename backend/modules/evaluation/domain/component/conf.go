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
	Provider string `mapstructure:"provider" json:"provider"` // RDS, S3 等
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
	GetExptTemplateUpdateEvalSetWhiteList(ctx context.Context) *entity.ExptTemplateUpdateEvalSetWhiteList
	// GetExptTurnScoreHookConf 读取行维度得分 HTTP 回调配置：根据 (spaceID, exptID, evaluatorVersionIDs)
	// 判定该实验是否命中外部打分回调，命中时返回回调调用配置（URL/Method/Timeout）。
	// 未命中返回 (nil, false)，此时由调用方回退本地等权计算。
	GetExptTurnScoreHookConf(ctx context.Context, spaceID, exptID int64, evaluatorVersionIDs []int64) (*entity.ExptTurnScoreHookConf, bool)
}
