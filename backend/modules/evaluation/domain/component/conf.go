// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package component

import (
	"context"
	"time"

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
	// GetEvalAsyncCtxTTL 返回 invoke_id → EvalAsyncCtx 的 Redis TTL。
	// 未显式配 eval_async_ctx_ttl_second 时按该空间的 async_zombie_second 推导，
	// 保证 ctx 始终活得比行僵尸判定久。spaceID 为 0（调试等无空间上下文场景）时取全局默认。
	GetEvalAsyncCtxTTL(ctx context.Context, spaceID int64) time.Duration
	GetErrRetryConf(ctx context.Context, spaceID int64, err error) *entity.RetryConf
	GetExptTurnResultFilterBmqProducerCfg(ctx context.Context) *entity.BmqProducerCfg
	GetCKDBName(ctx context.Context) *entity.CKDBConfig
	GetExptExportWhiteList(ctx context.Context) *entity.ExptExportWhiteList
	// GetExptSchedulingPrivilegeWhiteList 谁可以申报中心调度的特权参数
	// （priority / expected_quota_consumption / trigger_type=evalx 三者同一判据）。
	// 未命中者的申报值一律被忽略：priority 走缺省、向量丢弃、trigger 降级 manual。
	// 读取失败时按"谁都没有特权"处理。
	GetExptSchedulingPrivilegeWhiteList(ctx context.Context) *entity.ExptSchedulingPrivilegeWhiteList
	GetMaintainerUserIDs(ctx context.Context) map[string]bool
	GetSchedulerAbortCtrl(ctx context.Context) *entity.SchedulerAbortCtrl
	GetTargetTrajectoryConf(ctx context.Context) *entity.TargetTrajectoryConf
	GetExptTemplateUpdateEvalSetWhiteList(ctx context.Context) *entity.ExptTemplateUpdateEvalSetWhiteList
	GetExptMultiSetWhiteList(ctx context.Context) *entity.ExptMultiSetWhiteList
	GetExptTurnScoreHookConf(ctx context.Context, spaceID, exptID int64, evaluatorRefs []*entity.ExptEvaluatorVersionRef) (*entity.ExptTurnScoreHookConf, bool)
	// GetSandboxAgentNotifyConf 沙箱 agent 通知配置（进度卡间隔等）。返回 nil 表示读取失败，
	// 上层应回落到 entity.DefaultSandboxAgentNotifyConf。
	GetSandboxAgentNotifyConf(ctx context.Context) *entity.SandboxAgentNotifyConf
	// BuildEvalExt 构造评测记录（EvaluatorRecord/EvalTargetRecord/ExptTurnResultRunLog）落库时的 ext 扩展字段。
	// turn 为评测集中的轮次数据（部分调用点不可用时为 nil），spaceID 为空间 id。默认空实现返回 nil。
	BuildEvalExt(ctx context.Context, spaceID int64, turn *entity.Turn) map[string]string
	// GetCrossSpaceRecordReadEnforce 跨空间读取评估记录且未命中共享白名单时是否拒绝。
	// 未配置默认 false：只告警放行，用于观察存量调用后再收紧。
	GetCrossSpaceRecordReadEnforce(ctx context.Context) bool
}
