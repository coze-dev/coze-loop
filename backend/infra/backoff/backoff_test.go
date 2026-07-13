// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package backoff

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cenk/backoff"
	"github.com/stretchr/testify/assert"
)

func Test_Backoff(t *testing.T) {
	ctx := context.Background()
	fn := func() error { return fmt.Errorf("mock err") }
	p := backoff.NewExponentialBackOff()
	p.InitialInterval = defaultRetryInterval
	p.MaxElapsedTime = defaultRetryInterval

	// Since we can't mock without mockey, we'll test the actual functions
	// These should fail as expected since fn always returns an error
	assert.NotNil(t, RetryOneSecond(ctx, fn))
	assert.NotNil(t, RetryThreeSeconds(ctx, fn))
	assert.NotNil(t, RetryFiveSeconds(ctx, fn))
	assert.NotNil(t, RetryTenSeconds(ctx, fn))
}

func Test_backoff(t *testing.T) {
	ctx := context.Background()

	t.Run("test success", func(t *testing.T) {
		fn := func() error { return nil }
		assert.Nil(t, RetryWithElapsedTime(ctx, time.Second, fn))
	})

	t.Run("test ctx cancel", func(t *testing.T) {
		cc, cancelFn := context.WithCancel(ctx)
		cnt := 0
		fn := func() error {
			cancelFn()
			cnt++
			return fmt.Errorf("mock err")
		}

		start := time.Now()
		assert.NotNil(t, RetryWithElapsedTime(cc, time.Second, fn))
		assert.Equal(t, 1, cnt)
		assert.True(t, time.Since(start) < time.Second)
	})
}

func TestRetryWithMaxTimes(t *testing.T) {
	ctx := context.Background()

	t.Run("test success", func(t *testing.T) {
		var count int
		err := RetryWithMaxTimes(ctx, 3, func() error {
			count++
			return fmt.Errorf("error")
		})
		assert.NotNil(t, err)
		assert.Equal(t, 4, count)
	})
}

func TestRetryWithMaxTimesAndInterval(t *testing.T) {
	ctx := context.Background()

	t.Run("succeeds after retries within max", func(t *testing.T) {
		attempts := 0
		err := RetryWithMaxTimesAndInterval(ctx, 2, 10*time.Millisecond, func() error {
			attempts++
			if attempts < 3 {
				return fmt.Errorf("transient err")
			}
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, 3, attempts) // 1 initial + 2 retries
	})

	t.Run("fails after exhausting max retries", func(t *testing.T) {
		attempts := 0
		err := RetryWithMaxTimesAndInterval(ctx, 2, 10*time.Millisecond, func() error {
			attempts++
			return fmt.Errorf("always err")
		})
		assert.Error(t, err)
		assert.Equal(t, 3, attempts) // 1 initial + 2 retries, then give up
	})

	t.Run("succeeds on first try, no retry", func(t *testing.T) {
		attempts := 0
		err := RetryWithMaxTimesAndInterval(ctx, 3, 10*time.Millisecond, func() error {
			attempts++
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, 1, attempts)
	})

	t.Run("respects the fixed interval between attempts", func(t *testing.T) {
		start := time.Now()
		attempts := 0
		_ = RetryWithMaxTimesAndInterval(ctx, 2, 50*time.Millisecond, func() error {
			attempts++
			return fmt.Errorf("err")
		})
		// 2 retries × ~50ms 固定间隔,总耗时应 >= 80ms(留裕度)
		assert.GreaterOrEqual(t, time.Since(start), 80*time.Millisecond)
		assert.Equal(t, 3, attempts)
	})
}
