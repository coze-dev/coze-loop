// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package dist

import (
	"github.com/coze-dev/coze-loop/backend/infra/limiter"
	"github.com/coze-dev/coze-loop/backend/infra/redis"
)

// NewRateLimiterFactory
// NewRateLimiter `Rule.KeyExpr/Rule.Match` are configured based on the `expr-lang` syntax.
// In the `AllowN` method, tags are matched against expressions using `Rule.Match`.
// When multiple rules are matched, the first rule will be used.
func NewRateLimiterFactory(cmdable redis.Cmdable, opts ...FactoryOpt) limiter.IRateLimiterFactory {
	opt := &factoryOpt{}
	for _, fn := range opts {
		fn(opt)
	}

	return &factory{
		cmdable:   cmdable,
		cacheSize: opt.exprCacheSize,
	}
}

type FactoryOpt func(opt *factoryOpt)

// WithExprCacheSize size is in bytes.
func WithExprCacheSize(size int) FactoryOpt {
	return func(c *factoryOpt) {
		c.exprCacheSize = size
	}
}

type factoryOpt struct {
	exprCacheSize int
}

func NewPlainLimiterFactory(cmdable redis.Cmdable) limiter.IPlainRateLimiterFactory {
	return &plainLimiterFactory{
		cmdable: cmdable,
	}
}
