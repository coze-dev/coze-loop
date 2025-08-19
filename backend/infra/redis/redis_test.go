// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package redis

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvider(t *testing.T) {
	p := NewTestRedis(t)
	ctx := context.TODO()

	t.Run("exists", func(t *testing.T) {
		key := "exists_key"
		_, err := p.Set(ctx, key, "1", time.Minute).Result()
		require.NoError(t, err)

		got, err := p.Exists(ctx, key).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(1), got)
	})

	t.Run(`number`, func(t *testing.T) {
		key := `number_key`

		got, err := p.Incr(ctx, key).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(1), got)

		got, err = p.IncrBy(ctx, key, 10).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(11), got)

		got, err = p.Decr(ctx, key).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(10), got)

		got, err = p.DecrBy(ctx, key, 5).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(5), got)
	})

	t.Run(`string`, func(t *testing.T) {
		key := `string_key`

		_, err := p.Set(ctx, key, `v1`, time.Hour).Result()
		require.NoError(t, err)

		_, err = p.SetNX(ctx, key, `v2`, time.Hour).Result()
		require.NoError(t, err)

		got, err := p.Get(ctx, key).Result()
		require.NoError(t, err)
		assert.Equal(t, `v1`, got)
	})

	t.Run(`multi`, func(t *testing.T) {
		key1 := `string_key1`
		key2 := `string_key2`
		key3 := `string_key3`

		_, err := p.MSet(ctx, key1, `v1`, key2, `v2`, key3, `v3`).Result()
		require.NoError(t, err)

		_, err = p.MSetNX(ctx, key1, `v10`, key2, `v20`, key3, `v30`).Result()
		require.NoError(t, err)

		got, err := p.MGet(ctx, key1, key2, key3).Result()
		require.NoError(t, err)
		assert.Equal(t, []any{`v1`, `v2`, `v3`}, got)
	})

	t.Run(`hash`, func(t *testing.T) {
		key := `hash_key`

		require.NoError(t, p.HSet(ctx, key, `field1`, `value1`).Err())
		require.NoError(t, p.HSetNX(ctx, key, `field1`, `value10`).Err())
		require.NoError(t, p.HSetNX(ctx, key, `field2`, 1).Err())
		require.NoError(t, p.HIncrBy(ctx, key, `field2`, 10).Err())

		got1, err := p.HGetAll(ctx, key).Result()
		require.NoError(t, err)
		assert.Equal(t, map[string]string{`field1`: `value1`, `field2`: "11"}, got1)

		got2, err := p.HKeys(ctx, key).Result()
		require.NoError(t, err)
		assert.Equal(t, []string{`field1`, `field2`}, got2)

		got3, err := p.HLen(ctx, key).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(2), got3)

		got4, err := p.HMGet(ctx, key, `field1`, `field2`).Result()
		require.NoError(t, err)
		assert.Equal(t, []any{`value1`, "11"}, got4)

		got5, err := p.HGet(ctx, key, `field1`).Result()
		require.NoError(t, err)
		assert.Equal(t, `value1`, got5)

		got6, err := p.HExists(ctx, key, `field1`).Result()
		require.NoError(t, err)
		assert.Equal(t, true, got6)

		got7, err := p.HDel(ctx, key, `field1`).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(1), got7)

		got8, err := p.HExists(ctx, key, `field1`).Result()
		require.NoError(t, err)
		assert.Equal(t, false, got8)

		got9, err := p.HExists(ctx, key, `field2`).Result()
		require.NoError(t, err)
		assert.Equal(t, true, got9)
	})

	t.Run(`sorted set`, func(t *testing.T) {
		key := `sorted_set_key`

		require.NoError(t, p.ZAdd(ctx, key, redis.Z{Score: 1, Member: `value1`}).Err())
		require.NoError(t, p.ZAdd(ctx, key, redis.Z{Score: 2, Member: `value2`}).Err())

		got1, err := p.ZRange(ctx, key, 0, -1).Result()
		require.NoError(t, err)
		assert.Equal(t, []string{`value1`, `value2`}, got1)
	})

	t.Run(`misc`, func(t *testing.T) {
		key := `misc_key`

		require.NoError(t, p.Set(ctx, key, `v1`, time.Minute).Err())

		got1, err := p.Expire(ctx, key, time.Hour).Result()
		require.NoError(t, err)
		assert.Equal(t, true, got1)

		got2, err := p.Del(ctx, key).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(1), got2)

		assert.ErrorIs(t, p.Get(ctx, key).Err(), redis.Nil)

		script := `
			local key = KEYS[1]
			local value = ARGV[1]
			redis.call("SET", key, value)
			return "OK"
		`
		got3, err := p.Eval(ctx, script, []string{key}, `v2`).Result()
		require.NoError(t, err)
		assert.Equal(t, `OK`, got3)
	})

	t.Run(`pipeline`, func(t *testing.T) {
		key1 := `pipeline_key1`
		key2 := `pipeline_key2`

		pipe := p.Pipeline()
		cmd1 := pipe.Set(ctx, key1, `v1`, time.Minute)
		cmd2 := pipe.Incr(ctx, key2)

		cmds, err := pipe.Exec(ctx)
		require.NoError(t, err)
		_ = cmds
		assert.Equal(t, `OK`, cmd1.Val())
		assert.Equal(t, int64(1), cmd2.Val())
		assert.Equal(t, `OK`, cmds[0].(*redis.StatusCmd).Val())
		assert.Equal(t, int64(1), cmds[1].(*redis.IntCmd).Val())
	})
}

func TestUnwrap(t *testing.T) {
	p := NewTestRedis(t)

	got, ok := Unwrap(p)
	assert.True(t, ok)
	assert.NotNil(t, got)

	var x Cmdable
	got, ok = Unwrap(x)
	assert.False(t, ok)
	assert.Nil(t, got)
}
