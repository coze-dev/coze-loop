// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package conf

import (
	"context"
	"time"

	"github.com/bytedance/gg/gslice"
	"github.com/samber/lo"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func NewConfiger(configFactory conf.IConfigLoaderFactory) (component.IConfiger, error) {
	loader, err := configFactory.NewConfigLoader(consts.EvaluationConfigFileName)
	if err != nil {
		return nil, err
	}
	return &configer{
		loader: loader,
	}, nil
}

type configer struct {
	loader conf.IConfigLoader
}

func (c *configer) GetEvaluationRecordStorage(ctx context.Context) *component.EvaluationRecordStorage {
	const key = "evaluation_record_storage"
	var cfg *component.EvaluationRecordStorage
	if c.loader.UnmarshalKey(ctx, key, &cfg) == nil && cfg != nil && len(cfg.Providers) > 0 {
		return cfg
	}
	// 默认配置：200KB 以下 RDS，200KB 以上 S3
	return &component.EvaluationRecordStorage{
		Providers: []*component.EvaluationRecordProviderConfig{
			{Provider: "RDS", MaxSize: 204800},
			{Provider: "S3", MaxSize: 1 << 30},
		},
	}
}

func (c *configer) GetTargetTrajectoryConf(ctx context.Context) *entity.TargetTrajectoryConf {
	const key = "eval_target_trajectory_conf"
	cfg := &entity.TargetTrajectoryConf{}
	return lo.Ternary(c.loader.UnmarshalKey(ctx, key, cfg) == nil, cfg, nil)
}

// BuildEvalExt 构造评测记录落库时的 ext 扩展字段，默认空实现返回 nil。
func (c *configer) BuildEvalExt(ctx context.Context, spaceID int64, turn *entity.Turn) map[string]string {
	return nil
}

func (c *configer) GetSchedulerAbortCtrl(ctx context.Context) *entity.SchedulerAbortCtrl {
	return c.GetConsumerConf(ctx).GetSchedulerAbortCtrl()
}

func (c *configer) GetExptExecConf(ctx context.Context, spaceID int64) *entity.ExptExecConf {
	return c.GetConsumerConf(ctx).GetExptExecConf(spaceID)
}

func (c *configer) GetEvalAsyncCtxTTL(ctx context.Context, spaceID int64) time.Duration {
	return c.GetExptExecConf(ctx, spaceID).GetExptItemEvalConf().GetEvalAsyncCtxTTL()
}

func (c *configer) GetErrRetryConf(ctx context.Context, spaceID int64, err error) *entity.RetryConf {
	if rc := c.GetErrCtrl(ctx).GetErrRetryCtrl(spaceID).GetRetryConf(err); rc != nil {
		return rc
	}
	return &entity.RetryConf{}
}

func (c *configer) GetConsumerConf(ctx context.Context) (ecc *entity.ExptConsumerConf) {
	const key = "expt_consumer_conf"
	return lo.Ternary(c.loader.UnmarshalKey(ctx, key, &ecc) == nil, ecc, entity.DefaultExptConsumerConf())
}

func (c *configer) GetErrCtrl(ctx context.Context) (eec *entity.ExptErrCtrl) {
	const key = "expt_err_ctrl"
	return lo.Ternary(c.loader.UnmarshalKey(ctx, key, &eec) == nil, eec, entity.DefaultExptErrCtrl())
}

func (c *configer) GetExptTurnResultFilterBmqProducerCfg(ctx context.Context) *entity.BmqProducerCfg {
	return nil
}

func (c *configer) GetCKDBName(ctx context.Context) *entity.CKDBConfig {
	const key = "clickhouse_config"
	ckdb := &entity.CKDBConfig{}
	return lo.Ternary(c.loader.UnmarshalKey(ctx, key, ckdb) == nil, ckdb, &entity.CKDBConfig{})
}

func (c *configer) GetExptExportWhiteList(ctx context.Context) (eec *entity.ExptExportWhiteList) {
	const key = "expt_export_white_list"
	return lo.Ternary(c.loader.UnmarshalKey(ctx, key, &eec) == nil, eec, entity.DefaultExptExportWhiteList())
}

// GetExptSchedulingPrivilegeWhiteList 谁可以申报中心调度的特权参数
// （priority / expected_quota_consumption / trigger_type=evalx 三者同一判据）。
//
// 读取失败时回落到"谁都没有特权"而不是放行：这三样被滥用的后果都是"悄悄多占资源/插队"，
// 配置中心抖动时宁可让大家退回缺省行为（可见且无损），也不要静默放开特权。
func (c *configer) GetExptSchedulingPrivilegeWhiteList(ctx context.Context) (w *entity.ExptSchedulingPrivilegeWhiteList) {
	const key = "expt_scheduling_privilege_white_list"
	return lo.Ternary(c.loader.UnmarshalKey(ctx, key, &w) == nil, w, entity.DefaultExptSchedulingPrivilegeWhiteList())
}

func (c *configer) GetExptTemplateUpdateEvalSetWhiteList(ctx context.Context) (w *entity.ExptTemplateUpdateEvalSetWhiteList) {
	const key = "expt_template_update_eval_set_white_list"
	return lo.Ternary(c.loader.UnmarshalKey(ctx, key, &w) == nil, w, entity.DefaultExptTemplateUpdateEvalSetWhiteList())
}

func (c *configer) GetExptMultiSetWhiteList(ctx context.Context) (w *entity.ExptMultiSetWhiteList) {
	const key = "expt_multi_set_white_list"
	return lo.Ternary(c.loader.UnmarshalKey(ctx, key, &w) == nil, w, entity.DefaultExptMultiSetWhiteList())
}

func (c *configer) GetExptTurnScoreHookConf(ctx context.Context, spaceID, exptID int64, evaluatorRefs []*entity.ExptEvaluatorVersionRef) (*entity.ExptTurnScoreHookConf, bool) {
	return nil, false
}

func (c *configer) GetSandboxAgentNotifyConf(ctx context.Context) *entity.SandboxAgentNotifyConf {
	const key = "sandbox_agent_notify_conf"
	var cfg *entity.SandboxAgentNotifyConf
	if c.loader.UnmarshalKey(ctx, key, &cfg) == nil && cfg != nil {
		return cfg
	}
	return entity.DefaultSandboxAgentNotifyConf()
}

// GetCrossSpaceRecordReadEnforce 未配置时返回 false：跨空间读取评估记录只告警放行。
func (c *configer) GetCrossSpaceRecordReadEnforce(ctx context.Context) bool {
	const key = "cross_space_record_read_conf"
	var cfg *entity.CrossSpaceRecordReadConf
	if err := c.loader.UnmarshalKey(ctx, key, &cfg); err != nil || cfg == nil {
		return false
	}
	return cfg.Enforce
}

func (c *configer) GetMaintainerUserIDs(ctx context.Context) map[string]bool {
	const key = "system_maintainer_conf"
	var maintainerConf *entity.SystemMaintainerConf
	if err := c.loader.UnmarshalKey(ctx, key, &maintainerConf); err != nil {
		logs.CtxWarn(ctx, "cfg %s parse fail, err: %v", key, err)
		return nil
	}
	if maintainerConf != nil {
		return gslice.ToMap(maintainerConf.UserIDs, func(t string) (string, bool) { return t, true })
	}
	return nil
}
