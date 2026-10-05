// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

type hookFinalizationOwnerReader struct{ client redis.Cmdable }

func NewHookFinalizationOwnerReader(c redis.Cmdable) repo.IHookFinalizationOwnerReader {
	return &hookFinalizationOwnerReader{c}
}

func (r *hookFinalizationOwnerReader) ReadFinalizationOwner(ctx context.Context, key string) (string, error) {
	value, err := r.client.Get(ctx, key).Result()
	if redis.IsNilError(err) {
		return "", nil
	}
	return value, err
}
