// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewClient creates a new Redis client
// TODO: define a Config type with json/yaml tags.
func NewClient(opts *redis.Options) (Cmdable, error) {
	cli := redis.NewClient(opts)
	return &provider{cli: cli}, nil
}

func Unwrap(cli Cmdable) (*redis.Client, bool) {
	if p, ok := cli.(interface {
		Raw() *redis.Client
	}); ok {
		return p.Raw(), true
	}
	return nil, false
}

type provider struct {
	cli *redis.Client
}

var _ Cmdable = (*provider)(nil)

// Raw returns the underlying Redis client.
// Use with caution.
func (p *provider) Raw() *redis.Client {
	return p.cli
}

func (p *provider) Exists(ctx context.Context, keys ...string) *redis.IntCmd {
	return p.cli.Exists(ctx, keys...)
}

func (p *provider) Decr(ctx context.Context, key string) *redis.IntCmd {
	return p.cli.Decr(ctx, key)
}

func (p *provider) DecrBy(ctx context.Context, key string, decrement int64) *redis.IntCmd {
	return p.cli.DecrBy(ctx, key, decrement)
}

func (p *provider) Get(ctx context.Context, key string) *redis.StringCmd {
	return p.cli.Get(ctx, key)
}

func (p *provider) Incr(ctx context.Context, key string) *redis.IntCmd {
	return p.cli.Incr(ctx, key)
}

func (p *provider) IncrBy(ctx context.Context, key string, increment int64) *redis.IntCmd {
	return p.cli.IncrBy(ctx, key, increment)
}

func (p *provider) MGet(ctx context.Context, keys ...string) *redis.SliceCmd {
	return p.cli.MGet(ctx, keys...)
}

func (p *provider) MSet(ctx context.Context, values ...any) *redis.StatusCmd {
	return p.cli.MSet(ctx, values...)
}

func (p *provider) MSetNX(ctx context.Context, values ...any) *redis.BoolCmd {
	return p.cli.MSetNX(ctx, values...)
}

func (p *provider) Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd {
	return p.cli.Set(ctx, key, value, expiration)
}

func (p *provider) SetNX(ctx context.Context, key string, value any, expiration time.Duration) *redis.BoolCmd {
	return p.cli.SetNX(ctx, key, value, expiration)
}

func (p *provider) HDel(ctx context.Context, key string, fields ...string) *redis.IntCmd {
	return p.cli.HDel(ctx, key, fields...)
}

func (p *provider) HExists(ctx context.Context, key, field string) *redis.BoolCmd {
	return p.cli.HExists(ctx, key, field)
}

func (p *provider) HGet(ctx context.Context, key, field string) *redis.StringCmd {
	return p.cli.HGet(ctx, key, field)
}

func (p *provider) HGetAll(ctx context.Context, key string) *redis.MapStringStringCmd {
	return p.cli.HGetAll(ctx, key)
}

func (p *provider) HIncrBy(ctx context.Context, key, field string, increment int64) *redis.IntCmd {
	return p.cli.HIncrBy(ctx, key, field, increment)
}

func (p *provider) HKeys(ctx context.Context, key string) *redis.StringSliceCmd {
	return p.cli.HKeys(ctx, key)
}

func (p *provider) HLen(ctx context.Context, key string) *redis.IntCmd {
	return p.cli.HLen(ctx, key)
}

func (p *provider) HMGet(ctx context.Context, key string, fields ...string) *redis.SliceCmd {
	return p.cli.HMGet(ctx, key, fields...)
}

func (p *provider) HSet(ctx context.Context, key string, values ...any) *redis.IntCmd {
	return p.cli.HSet(ctx, key, values...)
}

func (p *provider) HSetNX(ctx context.Context, key, field string, value any) *redis.BoolCmd {
	return p.cli.HSetNX(ctx, key, field, value)
}

func (p *provider) ZAdd(ctx context.Context, key string, members ...redis.Z) *redis.IntCmd {
	return p.cli.ZAdd(ctx, key, members...)
}

func (p *provider) ZAddNX(ctx context.Context, key string, members ...redis.Z) *redis.IntCmd {
	return p.cli.ZAddNX(ctx, key, members...)
}

func (p *provider) ZRange(ctx context.Context, key string, start, stop int64) *redis.StringSliceCmd {
	return p.cli.ZRange(ctx, key, start, stop)
}

func (p *provider) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	return p.cli.Del(ctx, keys...)
}

func (p *provider) Eval(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd {
	return p.cli.Eval(ctx, script, keys, args...)
}

func (p *provider) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	return p.cli.Expire(ctx, key, expiration)
}

func (p *provider) Pipeline() Pipeliner {
	pipe := p.cli.Pipeline()
	return pipe
}
