// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package component

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate mockgen -destination=mocks/expt_configer.go -package=mocks . IConfiger
type IConfiger interface {
	GetConsumerConf(ctx context.Context) *entity.ExptConsumerConf
	GetErrCtrl(ctx context.Context) *entity.ExptErrCtrl
	GetExptExecConf(ctx context.Context, spaceID int64) *entity.ExptExecConf
	GetErrRetryConf(ctx context.Context, spaceID int64, err error) *entity.RetryConf
	GetExptTurnResultFilterBmqProducerCfg(ctx context.Context) *entity.BmqProducerCfg
	GetCKDBName(ctx context.Context) *entity.CKDBConfig
}
