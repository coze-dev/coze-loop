// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package goroutine

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	gslice "github.com/coze-dev/coze-loop/backend/pkg/lang/slices"
)

func TestNewPool(t *testing.T) {
	tests := []struct {
		name    string
		size    int
		wantErr bool
	}{
		{
			name:    "valid pool size",
			size:    10,
			wantErr: false,
		},
		{
			name:    "invalid pool size",
			size:    -1,
			wantErr: true,
		},
		{
			name:    "zero pool size",
			size:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := NewPool(tt.size)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, pool)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pool)
			}
		})
	}
}

func TestPool_Exec(t *testing.T) {
	t.Run("successful execution", func(t *testing.T) {
		ctx := context.Background()
		var result int

		pool, err := NewPool(2)
		assert.NoError(t, err)

		pool.Add(func() error {
			result = 1
			return nil
		})

		err = pool.Exec(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 1, result)
	})

	t.Run("error execution", func(t *testing.T) {
		ctx := context.Background()
		expectedErr := errors.New("test error")

		pool, err := NewPool(2)
		assert.NoError(t, err)

		pool.Add(func() error {
			return expectedErr
		})

		err = pool.Exec(ctx)
		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		pool, err := NewPool(2)
		assert.NoError(t, err)

		pool.Add(func() error {
			time.Sleep(100 * time.Millisecond)
			return nil
		})

		err = pool.Exec(ctx)
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})

	t.Run("fail fast returns a single error not aggregated", func(t *testing.T) {
		// Exec 模式快速失败，只应返回单个 error，而非 errors.Join 聚合的多 error。
		ctx := context.Background()
		sentinel := errors.New("sentinel")

		pool, err := NewPool(1)
		assert.NoError(t, err)

		pool.Add(func() error { return sentinel })
		pool.Add(func() error { return errors.New("should not run or be aggregated") })

		err = pool.Exec(ctx)
		assert.Error(t, err)
		// 返回的就是那个 error 本身，未被 Join 包裹（Join 会改变 Error() 文案为多行）
		assert.Equal(t, sentinel, err)
	})
}

func TestPool_ExecAll(t *testing.T) {
	t.Run("continue on error", func(t *testing.T) {
		ctx := context.Background()
		var (
			results []int
			mutex   = sync.Mutex{}
		)

		pool, err := NewPool(2)
		assert.NoError(t, err)

		pool.Add(func() error {
			mutex.Lock()
			results = append(results, 1)
			mutex.Unlock()
			return errors.New("first error")
		})

		pool.Add(func() error {
			mutex.Lock()
			results = append(results, 2)
			mutex.Unlock()
			return nil
		})

		err = pool.ExecAll(ctx)
		assert.Error(t, err)
		assert.Equal(t,
			gslice.ToMap([]int{1, 2}, func(v int) (int, bool) { return v, true }),
			gslice.ToMap(results, func(v int) (int, bool) { return v, true }),
		)
	})

	t.Run("aggregate multiple errors", func(t *testing.T) {
		ctx := context.Background()
		err1 := errors.New("err1")
		err2 := errors.New("err2")
		err3 := errors.New("err3")

		pool, err := NewPool(3)
		assert.NoError(t, err)

		pool.Add(func() error { return err1 })
		pool.Add(func() error { return nil })
		pool.Add(func() error { return err2 })
		pool.Add(func() error { return err3 })

		err = pool.ExecAll(ctx)
		assert.Error(t, err)
		// errors.Join 聚合后，每个 error 都应能被 errors.Is 匹配到
		assert.True(t, errors.Is(err, err1))
		assert.True(t, errors.Is(err, err2))
		assert.True(t, errors.Is(err, err3))
	})

	t.Run("aggregated error supports errors.As", func(t *testing.T) {
		ctx := context.Background()

		pool, err := NewPool(2)
		assert.NoError(t, err)

		pool.Add(func() error { return errors.New("plain error") })
		pool.Add(func() error { return &customError{msg: "custom"} })

		err = pool.ExecAll(ctx)
		assert.Error(t, err)
		var ce *customError
		assert.True(t, errors.As(err, &ce))
		assert.Equal(t, "custom", ce.msg)
	})

	t.Run("no error returns nil", func(t *testing.T) {
		ctx := context.Background()

		pool, err := NewPool(3)
		assert.NoError(t, err)

		for i := 0; i < 5; i++ {
			pool.Add(func() error { return nil })
		}

		err = pool.ExecAll(ctx)
		assert.NoError(t, err)
	})

	t.Run("concurrent errors of different concrete types do not panic", func(t *testing.T) {
		// 回归测试：早期实现用 atomic.Value 存 error，不同具体类型的 error 并发
		// Store 会 panic。此处用多种具体类型的 error 并发返回，验证不再 panic。
		ctx := context.Background()

		pool, err := NewPool(8)
		assert.NoError(t, err)

		for i := 0; i < 50; i++ {
			idx := i
			pool.Add(func() error {
				switch idx % 3 {
				case 0:
					return errors.New("plain")
				case 1:
					return &customError{msg: "custom"}
				default:
					return fmt.Errorf("wrapped %d: %w", idx, errors.New("inner"))
				}
			})
		}

		assert.NotPanics(t, func() {
			err = pool.ExecAll(ctx)
		})
		assert.Error(t, err)
	})
}

type customError struct {
	msg string
}

func (e *customError) Error() string { return e.msg }

func Test_pool_execute(t *testing.T) {
	t.Run("execute tasks with pool size equal to task count", func(t *testing.T) {
		ctx := context.Background()
		size := 5

		p, err := NewPool(size)
		assert.NoError(t, err)

		for i := 0; i < size; i++ {
			p.Add(func() error {
				time.Sleep(time.Duration(rand.Intn(5)) * time.Second)
				return nil
			})
		}

		err = p.Exec(ctx)
		assert.NoError(t, err)
	})

	t.Run("execute tasks with pool size greater than task count", func(t *testing.T) {
		ctx := context.Background()
		size := 5

		p, err := NewPool(size)
		assert.NoError(t, err)

		for i := 0; i < size>>1; i++ {
			p.Add(func() error {
				time.Sleep(time.Duration(rand.Intn(5)) * time.Second)
				return nil
			})
		}

		err = p.Exec(ctx)
		assert.NoError(t, err)
	})

	t.Run("execute tasks with pool size less than task count", func(t *testing.T) {
		ctx := context.Background()
		size := 5

		p, err := NewPool(size)
		assert.NoError(t, err)

		for i := 0; i < size<<1; i++ {
			p.Add(func() error {
				time.Sleep(time.Duration(rand.Intn(5)) * time.Second)
				return nil
			})
		}

		err = p.Exec(ctx)
		assert.NoError(t, err)
	})

	t.Run("execute tasks with error", func(t *testing.T) {
		ctx := context.Background()
		size := 5

		p, err := NewPool(size)
		assert.NoError(t, err)

		for i := 0; i < size; i++ {
			idx := i

			p.Add(func() error {
				time.Sleep(time.Duration(rand.Intn(5)) * time.Second)
				if idx == 2 {
					return errors.New("test err")
				}
				return nil
			})
		}

		err = p.Exec(ctx)
		assert.Error(t, err)
		assert.Equal(t, "test err", err.Error())
	})
}
