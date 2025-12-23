// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package conf

import (
	"context"

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

func (c *configer) GetTargetTrajectoryConf(ctx context.Context) *entity.TargetTrajectoryConf {
	const key = "eval_target_trajectory_conf"
	cfg := &entity.TargetTrajectoryConf{}
	return lo.Ternary(c.loader.UnmarshalKey(ctx, key, cfg) == nil, cfg, nil)
}

func (c *configer) GetSchedulerAbortCtrl(ctx context.Context) *entity.SchedulerAbortCtrl {
	return c.GetConsumerConf(ctx).GetSchedulerAbortCtrl()
}

func (c *configer) GetExptExecConf(ctx context.Context, spaceID int64) *entity.ExptExecConf {
	return c.GetConsumerConf(ctx).GetExptExecConf(spaceID)
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
