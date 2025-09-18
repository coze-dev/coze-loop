// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package dao

import (
	"context"
	"fmt"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/convert"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/conv"
)

type IExptItemTurnEvalAsyncDAO interface {
	SetAsyncCtx(ctx context.Context, invokeID string, actx *entity.ExptItemTurnEvalAsyncCtx) error
	GetAsyncCtx(ctx context.Context, invokeID string) (*entity.ExptItemTurnEvalAsyncCtx, error)
}

func NewExptItemTurnEvalAsyncDAO(cmdable redis.Cmdable) IExptItemTurnEvalAsyncDAO {
	const table = "experiment"
	return &exptItemTurnEvalAsyncDAOImpl{cmdable: cmdable, table: table}
}

type exptItemTurnEvalAsyncDAOImpl struct {
	cmdable redis.Cmdable
	table   string
}

func (e *exptItemTurnEvalAsyncDAOImpl) makeExptItemTurnEvalAsyncCtxKey(invokeID string) string {
	return fmt.Sprintf("[%s]item_turn_eval_async_ctx:%s", e.table, invokeID)
}

func (e *exptItemTurnEvalAsyncDAOImpl) SetAsyncCtx(ctx context.Context, invokeID string, actx *entity.ExptItemTurnEvalAsyncCtx) error {
	bytes, err := convert.NewExptItemTurnEvalAsyncCtx().FromDO(actx)
	if err != nil {
		return err
	}
	key := e.makeExptItemTurnEvalAsyncCtxKey(invokeID)
	if err := e.cmdable.Set(ctx, key, bytes, time.Hour*12).Err(); err != nil {
		return errorx.Wrapf(err, "redis set key: %v", key)
	}
	return nil
}

func (e *exptItemTurnEvalAsyncDAOImpl) GetAsyncCtx(ctx context.Context, invokeID string) (*entity.ExptItemTurnEvalAsyncCtx, error) {
	key := e.makeExptItemTurnEvalAsyncCtxKey(invokeID)
	got, err := e.cmdable.Get(ctx, key).Result()
	if err != nil && !redis.IsNilError(err) {
		return nil, errorx.Wrapf(err, "redis get fail, key: %v", key)
	}
	return convert.NewExptItemTurnEvalAsyncCtx().ToDO(conv.UnsafeStringToBytes(got))
}
