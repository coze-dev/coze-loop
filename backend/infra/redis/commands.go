// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

var _ redis.Cmdable = (*redis.Client)(nil) // import for links in go doc

type Cmdable interface {
	SimpleCmdable
	Pipeline() Pipeliner
}

type SimpleCmdable interface {
	StringCmdable
	HashCmdable
	SortedSetCmdable

	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Eval(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	Exists(ctx context.Context, keys ...string) *redis.IntCmd
}

// StringCmdable copy methods we need in [redis.StringCmdable]
type StringCmdable interface {
	Decr(ctx context.Context, key string) *redis.IntCmd
	DecrBy(ctx context.Context, key string, decrement int64) *redis.IntCmd
	Incr(ctx context.Context, key string) *redis.IntCmd
	IncrBy(ctx context.Context, key string, value int64) *redis.IntCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	MGet(ctx context.Context, keys ...string) *redis.SliceCmd
	Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd
	SetNX(ctx context.Context, key string, value any, expiration time.Duration) *redis.BoolCmd
	MSet(ctx context.Context, values ...any) *redis.StatusCmd
	MSetNX(ctx context.Context, values ...any) *redis.BoolCmd
}

// HashCmdable copy methods we need in [redis.HashCmdable]
type HashCmdable interface {
	// Notice: ALWAYS pass 2 elements(a field and its value) to values, or internal redis will throw error
	HSet(ctx context.Context, key string, values ...any) *redis.IntCmd
	HSetNX(ctx context.Context, key, field string, value any) *redis.BoolCmd
	HIncrBy(ctx context.Context, key, field string, incr int64) *redis.IntCmd
	HDel(ctx context.Context, key string, fields ...string) *redis.IntCmd
	HExists(ctx context.Context, key, field string) *redis.BoolCmd
	HKeys(ctx context.Context, key string) *redis.StringSliceCmd
	HLen(ctx context.Context, key string) *redis.IntCmd
	HGet(ctx context.Context, key, field string) *redis.StringCmd
	HGetAll(ctx context.Context, key string) *redis.MapStringStringCmd
	HMGet(ctx context.Context, key string, fields ...string) *redis.SliceCmd
}

// SortedSetCmdable copy methods we need in [redis.SortedSetCmdable]
type SortedSetCmdable interface {
	ZAdd(ctx context.Context, key string, members ...redis.Z) *redis.IntCmd
	ZAddNX(ctx context.Context, key string, members ...redis.Z) *redis.IntCmd
	ZRange(ctx context.Context, key string, start, stop int64) *redis.StringSliceCmd
}

type Pipeliner interface {
	SimpleCmdable

	Len() int
	Exec(ctx context.Context) ([]redis.Cmder, error)
	Discard()
}
