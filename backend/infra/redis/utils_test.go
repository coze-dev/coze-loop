// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package redis

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestIsNilError(t *testing.T) {
	assert.False(t, IsNilError(nil))
	assert.True(t, IsNilError(redis.Nil))

	p := NewTestRedis(t)
	ctx := context.TODO()

	_, err := p.Get(ctx, "redis_nil_key").Result()
	assert.True(t, IsNilError(err))
}
