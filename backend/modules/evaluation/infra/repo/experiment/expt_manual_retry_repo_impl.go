// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
)

type ExptManualRetryRepoImpl struct {
	dao repo.IExptManualRetryRepo
}

func NewExptManualRetryRepo(provider db.Provider, generator idgen.IIDGenerator) repo.IExptManualRetryRepo {
	return &ExptManualRetryRepoImpl{dao: mysql.NewExptManualRetryDAO(provider, generator)}
}

func (r *ExptManualRetryRepoImpl) FindReceipt(ctx context.Context, spaceID, exptID, runID int64, requestID, hash string) (*entity.ManualRetryReceipt, error) {
	return r.dao.FindReceipt(ctx, spaceID, exptID, runID, requestID, hash)
}

func (r *ExptManualRetryRepoImpl) RequeueTerminalItems(ctx context.Context, request *entity.RequeueTerminalItemsRequest) (*entity.ManualRetryReceipt, error) {
	return r.dao.RequeueTerminalItems(ctx, request)
}

func (r *ExptManualRetryRepoImpl) PreflightTerminalItems(ctx context.Context, request *entity.RequeueTerminalItemsRequest) (*entity.ManualRetryPreparation, error) {
	return r.dao.PreflightTerminalItems(ctx, request)
}
