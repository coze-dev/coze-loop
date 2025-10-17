// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/dao"
)

type EvalAsyncRepoImpl struct {
	dao.IEvalAsyncDAO
}

func NewEvalAsyncRepo(dao dao.IEvalAsyncDAO) repo.IEvalAsyncRepo {
	return &EvalAsyncRepoImpl{IEvalAsyncDAO: dao}
}
