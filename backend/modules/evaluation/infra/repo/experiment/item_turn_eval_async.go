// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/dao"
)

type ExptItemTurnEvalAsyncRepoImpl struct {
	dao.IExptItemTurnEvalAsyncDAO
}

func NewExptItemTurnEvalAsyncRepo(dao dao.IExptItemTurnEvalAsyncDAO) repo.IExptItemTurnEvalAsyncRepo {
	return &ExptItemTurnEvalAsyncRepoImpl{IExptItemTurnEvalAsyncDAO: dao}
}
