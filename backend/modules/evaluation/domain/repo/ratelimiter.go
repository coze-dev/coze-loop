// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	commonentity "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate mockgen -destination mocks/ratelimiter_mock.go -package mocks . RateLimiter
type RateLimiter interface {
	AllowInvoke(ctx context.Context, spaceID int64) bool
	AllowInvokeWithKeyLimit(ctx context.Context, key string, limit *commonentity.RateLimit) bool
}
