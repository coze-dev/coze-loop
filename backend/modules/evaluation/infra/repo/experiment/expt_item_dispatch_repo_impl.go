// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
)

// ExptItemDispatchRepoImpl 派发投影的 repo 实现，纯透传到 DAO。
// 保留这一层是为了让 domain 不依赖 gorm_gen model —— 与本包其它 repo 一致。
type ExptItemDispatchRepoImpl struct {
	dispatchDAO mysql.IExptItemDispatchDAO
}

func NewExptItemDispatchRepo(dispatchDAO mysql.IExptItemDispatchDAO) repo.IExptItemDispatchRepo {
	return &ExptItemDispatchRepoImpl{dispatchDAO: dispatchDAO}
}

func (e *ExptItemDispatchRepoImpl) ClaimQuotaReserved(ctx context.Context, spaceID, exptID, exptRunID int64, itemIDs []int64) ([]int64, error) {
	return e.dispatchDAO.ClaimQuotaReserved(ctx, spaceID, exptID, exptRunID, itemIDs)
}

func (e *ExptItemDispatchRepoImpl) ResetQuotaReserved(ctx context.Context, spaceID, exptID, exptRunID int64, itemIDs []int64) ([]int64, error) {
	return e.dispatchDAO.ResetQuotaReserved(ctx, spaceID, exptID, exptRunID, itemIDs)
}

func (e *ExptItemDispatchRepoImpl) LoadDispatchRuntime(ctx context.Context, spaceID, exptID, exptRunID int64, candidateLimit int) (*repo.ExptDispatchRuntime, error) {
	return e.dispatchDAO.LoadDispatchRuntime(ctx, spaceID, exptID, exptRunID, candidateLimit)
}

func (e *ExptItemDispatchRepoImpl) StartReservedItem(ctx context.Context, spaceID, exptID, exptRunID, itemID int64) (bool, error) {
	return e.dispatchDAO.StartReservedItem(ctx, spaceID, exptID, exptRunID, itemID)
}

func (e *ExptItemDispatchRepoImpl) RequeueProcessingItem(ctx context.Context, spaceID, exptID, exptRunID, itemID int64) (bool, error) {
	return e.dispatchDAO.RequeueProcessingItem(ctx, spaceID, exptID, exptRunID, itemID)
}

func (e *ExptItemDispatchRepoImpl) MGetDispatchObservations(ctx context.Context, spaceID, exptID, exptRunID int64, itemIDs []int64) ([]*repo.ExptDispatchObservation, error) {
	return e.dispatchDAO.MGetDispatchObservations(ctx, spaceID, exptID, exptRunID, itemIDs)
}
