// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package dist

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/infra/limiter"
	"github.com/coze-dev/coze-loop/backend/infra/redis"
)

func TestPlainLimiterFactory_NewPlainRateLimiter(t *testing.T) {
	t.Run("创建限流器成功", func(t *testing.T) {
		factory := &plainLimiterFactory{
			cmdable: redis.NewTestRedis(t),
		}

		limiter := factory.NewPlainRateLimiter()
		assert.NotNil(t, limiter)
	})

	t.Run("创建带规则的限流器", func(t *testing.T) {
		factory := &plainLimiterFactory{
			cmdable: redis.NewTestRedis(t),
		}

		rules := []limiter.Rule{
			{
				Match:   "itag==1",
				KeyExpr: "key+string(itag)",
				Limit: limiter.Limit{
					Rate:   10,
					Burst:  20,
					Period: 60,
				},
			},
		}

		limiter := factory.NewPlainRateLimiter(limiter.WithRules(rules...))
		assert.NotNil(t, limiter)
	})

	t.Run("限流器基本功能测试", func(t *testing.T) {
		factory := &plainLimiterFactory{
			cmdable: redis.NewTestRedis(t),
		}

		pl := factory.NewPlainRateLimiter()
		assert.NotNil(t, pl)

		ctx := context.Background()
		key := "test_key"

		// 测试基本限流功能
		result, err := pl.AllowN(ctx, key, 1, limiter.WithLimit(&limiter.Limit{
			Rate:   5,
			Burst:  10,
			Period: 60,
		}))

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Allowed)
		assert.Equal(t, key, result.OriginKey)
		assert.Equal(t, key, result.LimitKey)
	})

	t.Run("简单限流功能测试", func(t *testing.T) {
		factory := &plainLimiterFactory{
			cmdable: redis.NewTestRedis(t),
		}

		pl := factory.NewPlainRateLimiter()
		assert.NotNil(t, pl)

		ctx := context.Background()
		key := "test_key"

		// 测试基本限流功能 - 使用自定义限流选项
		result, err := pl.AllowN(ctx, key, 1, limiter.WithLimit(&limiter.Limit{
			Rate:   5,
			Burst:  10,
			Period: 60,
		}))

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Allowed)
		assert.Equal(t, key, result.OriginKey)
		assert.Equal(t, key, result.LimitKey)
	})

	t.Run("限流拒绝测试", func(t *testing.T) {
		factory := &plainLimiterFactory{
			cmdable: redis.NewTestRedis(t),
		}

		pl := factory.NewPlainRateLimiter()
		assert.NotNil(t, pl)

		ctx := context.Background()
		key := "test_key"

		// 设置一个很低的限流阈值，注意 Rate 是每秒的速率，Burst 是突发容量
		limit := &limiter.Limit{
			Rate:   1, // 每秒1个请求
			Burst:  1, // 突发容量为1
			Period: time.Second,
		}

		// 第一次请求应该成功
		result1, err := pl.AllowN(ctx, key, 1, limiter.WithLimit(limit))
		assert.NoError(t, err)
		assert.True(t, result1.Allowed)

		// 立即第二次请求应该被拒绝（burst为1）
		result2, err := pl.AllowN(ctx, key, 1, limiter.WithLimit(limit))
		assert.NoError(t, err)
		assert.False(t, result2.Allowed)

		// 等待一段时间后再次请求应该成功
		time.Sleep(time.Second + 100*time.Millisecond)
		result3, err := pl.AllowN(ctx, key, 1, limiter.WithLimit(limit))
		assert.NoError(t, err)
		assert.True(t, result3.Allowed)
	})

	t.Run("批量请求测试", func(t *testing.T) {
		factory := &plainLimiterFactory{
			cmdable: redis.NewTestRedis(t),
		}

		pl := factory.NewPlainRateLimiter()
		assert.NotNil(t, pl)

		ctx := context.Background()
		key := "test_key"

		limit := &limiter.Limit{
			Rate:   10,
			Burst:  20,
			Period: 60,
		}

		// 请求数量超过burst应该被拒绝
		result, err := pl.AllowN(ctx, key, 25, limiter.WithLimit(limit))
		assert.NoError(t, err)
		assert.False(t, result.Allowed)

		// 请求数量在burst范围内应该成功
		result2, err := pl.AllowN(ctx, key, 15, limiter.WithLimit(limit))
		assert.NoError(t, err)
		assert.True(t, result2.Allowed)
	})
}
