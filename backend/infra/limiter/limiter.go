// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package limiter

import (
	"context"
	"time"
)

//go:generate mockgen -destination=mocks/rate_limiter_factory.go -package=mocks . IRateLimiterFactory
type IRateLimiterFactory interface {
	NewRateLimiter(opts ...FactoryOptionFn) IRateLimiter
}

type FactoryOptionFn func(opt *FactoryOption)

func WithRules(rules ...Rule) FactoryOptionFn {
	return func(c *FactoryOption) {
		c.Rules = rules
	}
}

type FactoryOption struct {
	Rules []Rule
}

//go:generate mockgen -destination=mocks/rate_limiter.go -package=mocks . IRateLimiter
type IRateLimiter interface {
	AllowN(ctx context.Context, key string, n int, opts ...LimitOptionFn) (*Result, error)
}

type LimitOptionFn func(opt *LimitOption)

func WithTags(tags ...Tag) LimitOptionFn {
	return func(c *LimitOption) {
		c.Tags = tags
	}
}

func WithLimit(limit *Limit) LimitOptionFn {
	return func(c *LimitOption) {
		c.Limit = limit
	}
}

type LimitOption struct {
	Tags  []Tag
	Limit *Limit
}

type Tag struct {
	K string
	V any
}

type Result struct {
	Allowed   bool
	OriginKey string
	LimitKey  string
}

type Rule struct {
	// Match Tags are matched with Match. If the match is successful,
	// rate limiting is applied; if the expression is empty, it matches all.
	Match string `json:"match" yaml:"match" mapstructure:"match"`

	// KeyExpr Tags are matched with KeyExpr. If the match is successful, the
	// matching result is used as the limiting key; if not, the OriginKey is
	// used as the limiting key.
	KeyExpr string `json:"key_expr" yaml:"key_expr" mapstructure:"key_expr"`

	Limit Limit `json:"limit" yaml:"limit" mapstructure:"limit"`
}

type Limit struct {
	// Rate is represented as number of events per second.
	Rate   int           `json:"rate" yaml:"rate" mapstructure:"rate"`
	Burst  int           `json:"burst" yaml:"burst" mapstructure:"burst"`
	Period time.Duration `json:"period" yaml:"period" mapstructure:"period"`
}

//go:generate mockgen -destination=mocks/rate_limiter_factory.go -package=mocks . IPlainRateLimiterFactory
type IPlainRateLimiterFactory interface {
	NewPlainRateLimiter(opts ...FactoryOptionFn) IPlainRateLimiter
}

//go:generate mockgen -destination=mocks/rate_limiter.go -package=mocks . IPlainRateLimiter
type IPlainRateLimiter interface {
	AllowN(ctx context.Context, key string, n int, opts ...LimitOptionFn) (*Result, error)
}
