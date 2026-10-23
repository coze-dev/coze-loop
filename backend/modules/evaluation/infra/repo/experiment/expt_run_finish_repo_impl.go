// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package experiment

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
)

type ExptRunFinishRepoImpl struct{ dao repo.IExptRunFinishRepo }

func NewExptRunFinishRepo(provider db.Provider) repo.IExptRunFinishRepo {
	return &ExptRunFinishRepoImpl{dao: mysql.NewExptRunFinishDAO(provider)}
}

func (r *ExptRunFinishRepoImpl) FinishCurrentRun(ctx context.Context, request *entity.ExptRunFinishRequest) (*entity.ExptRunFinishResult, error) {
	return r.dao.FinishCurrentRun(ctx, request)
}

func (r *ExptRunFinishRepoImpl) TerminateCurrentRun(ctx context.Context, request *entity.ExptRunTerminationRequest) (*entity.ExptRunTerminationResult, error) {
	return r.dao.(repo.IExptRunTerminationRepo).TerminateCurrentRun(ctx, request)
}

func (r *ExptRunFinishRepoImpl) GetTerminationItemSource(ctx context.Context, space, expt, run, item int64) (*entity.ExptCurrentItemRunSource, error) {
	return r.dao.(repo.IExptRunTerminationRepo).GetTerminationItemSource(ctx, space, expt, run, item)
}
