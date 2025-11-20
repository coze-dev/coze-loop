// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package dist

import (
	"fmt"

	"github.com/go-redis/redis_rate/v10"

	"github.com/coze-dev/coze-loop/backend/infra/limiter"
	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/coze-dev/coze-loop/backend/pkg/mcache/byted"
)

type plainLimiterFactory struct {
	cmdable redis.Cmdable
}

func (f *plainLimiterFactory) NewPlainRateLimiter(opts ...limiter.FactoryOptionFn) limiter.IPlainRateLimiter {
	opt := &limiter.FactoryOption{}
	for _, fn := range opts {
		fn(opt)
	}

	rawRedis, ok := redis.Unwrap(f.cmdable)
	if !ok {
		panic(fmt.Errorf("redis cmdable must be unwrappable"))
	}

	rl := &rateLimiter{
		rules:   make([]*rule, 0, len(opt.Rules)),
		limiter: redis_rate.NewLimiter(rawRedis),
		vmCache: byted.NewLRUCache(5 * 1024 * 1024), // 默认5MB缓存
	}

	for _, r := range opt.Rules {
		if rr, err := rl.newRule(r); err != nil {
			logs.Error("rate limiter set rule failed, rule: %v, err: %v", r, err)
		} else {
			rl.addRule(rr)
		}
	}

	return rl
}
