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
	GetExptMultiSetWhiteList(ctx context.Context) *entity.ExptMultiSetWhiteList
	GetExptTurnScoreHookConf(ctx context.Context, spaceID, exptID int64, evaluatorRefs []*entity.ExptEvaluatorVersionRef) (*entity.ExptTurnScoreHookConf, bool)
	// GetSandboxAgentNotifyConf 沙箱 agent 通知配置（进度卡间隔等）。返回 nil 表示读取失败，
	// 上层应回落到 entity.DefaultSandboxAgentNotifyConf。
	GetSandboxAgentNotifyConf(ctx context.Context) *entity.SandboxAgentNotifyConf
	// GetExptSandboxStepMetricConf 评测链路阶段埋点配置。读取失败 / 键不存在 / 值为空一律返回 nil，
	// 由 entity.ClassifyStepErrorType 把「查不到」判为工程错误（spec D4）。
	GetExptSandboxStepMetricConf(ctx context.Context) *entity.ExptSandboxStepMetricConf
	// BuildEvalExt 构造评测记录（EvaluatorRecord/EvalTargetRecord/ExptTurnResultRunLog）落库时的 ext 扩展字段。
	// turn 为评测集中的轮次数据（部分调用点不可用时为 nil），spaceID 为空间 id。默认空实现返回 nil。
	BuildEvalExt(ctx context.Context, spaceID int64, turn *entity.Turn) map[string]string
}
