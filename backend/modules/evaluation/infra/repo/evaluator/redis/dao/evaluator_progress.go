// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package dao

import (
	"context"
	"fmt"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"

	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

type IEvaluatorProgressDAO interface {
	RPushEvaluatorProgress(ctx context.Context, invokeID int64, messages []*entity.EvaluatorProgressMessage) error
}

func NewEvaluatorProgressDAO(cmdable redis.Cmdable) IEvaluatorProgressDAO {
	const table = "evaluator"
	return &evaluatorProgressDAOImpl{cmdable: cmdable, table: table}
}

type evaluatorProgressDAOImpl struct {
	cmdable redis.Cmdable
	table   string
}

func (e *evaluatorProgressDAOImpl) makeEvaluatorProgressKey(invokeID int64) string {
	return fmt.Sprintf("[%s]invoke_progress:%d", e.table, invokeID)
}

func (e *evaluatorProgressDAOImpl) RPushEvaluatorProgress(ctx context.Context, invokeID int64, messages []*entity.EvaluatorProgressMessage) error {
	if len(messages) == 0 {
		return nil
	}

	key := e.makeEvaluatorProgressKey(invokeID)
	values := make([]any, 0, len(messages))
	for _, msg := range messages {
		bytes, err := json.Marshal(msg)
		if err != nil {
			return errorx.Wrapf(err, "marshal evaluator progress message fail")
		}
		values = append(values, string(bytes))
	}

	if err := e.cmdable.RPush(ctx, key, values...).Err(); err != nil {
		return errorx.Wrapf(err, "redis rpush fail, key: %v", key)
	}

	if err := e.cmdable.Expire(ctx, key, time.Hour*12).Err(); err != nil {
		return errorx.Wrapf(err, "redis expire fail, key: %v", key)
	}

	return nil
}
