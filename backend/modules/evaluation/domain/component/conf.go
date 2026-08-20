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
	// GetExptPriorityWhiteList 谁可以在发起实验时指定调度优先级（user / space / caller PSM 三维 OR）。
	// 未命中者的申报值被忽略、强制走缺省优先级。读取失败时按"谁都不许"处理。
	GetExptPriorityWhiteList(ctx context.Context) *entity.ExptPriorityWhiteList
	// GetExptTriggerTrustConf 谁可以自称 EvalX 从而让实验进入 enforce（按 caller PSM 判定）。
	// 缺省不启用校验 —— 一律拒绝会让全部 EvalX 实验静默退回 legacy，比"配了没生效"隐蔽。
	GetExptTriggerTrustConf(ctx context.Context) *entity.ExptTriggerTrustConf
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
}
