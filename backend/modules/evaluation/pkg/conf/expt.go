// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package conf

import (
	"context"

	"github.com/samber/lo"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
)

func NewExptConfiger(configFactory conf.IConfigLoaderFactory) (component.IConfiger, error) {
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
	return nil
}
