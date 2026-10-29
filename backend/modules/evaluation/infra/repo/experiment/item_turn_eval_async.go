// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/dao"
)

type EvalAsyncRepoImpl struct {
	dao.IEvalAsyncDAO
}

func NewEvalAsyncRepo(dao dao.IEvalAsyncDAO) repo.IEvalAsyncRepo {
	return &EvalAsyncRepoImpl{IEvalAsyncDAO: dao}
}

func (e *EvalAsyncRepoImpl) SetEvalAsyncCtxIfAbsent(ctx context.Context, id string, actx *entity.EvalAsyncCtx) (bool, error) {
	preparer, ok := e.IEvalAsyncDAO.(repo.EvalAsyncContextPreparer)
	if !ok {
		return false, fmt.Errorf("async context conditional registration unavailable")
	}
	return preparer.SetEvalAsyncCtxIfAbsent(ctx, id, actx)
}
