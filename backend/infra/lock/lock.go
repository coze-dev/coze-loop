// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package lock

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/bytedance/gg/gvalue"
	"github.com/cenk/backoff"
	"github.com/pkg/errors"
	"github.com/rs/xid"

	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/goroutine"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

//go:generate mockgen -destination=mocks/lock.go -package=mocks . ILocker
type ILocker interface {
	WithHolder(holder string) ILocker
	Lock(ctx context.Context, key string, expiresIn time.Duration) (bool, error)
	Unlock(key string) (bool, error)
	ExpireLockIn(key string, expiresIn time.Duration) (bool, error)
	LockBackoff(ctx context.Context, key string, expiresIn time.Duration, maxWait time.Duration) (bool, error)
	// LockBackoffWithRenew 获取锁并异步保持定时续期，每次锁保持时间为 ttl，到达 maxHold 时间或被 cancel
	// 后退出续期。调用方做写操作前应检查 ctx.Done 以确认仍持有锁，发生错误时应调用 cancel 以主动释放锁。
	LockBackoffWithRenew(parent context.Context, key string, ttl time.Duration, maxHold time.Duration) (locked bool, ctx context.Context, cancel func(), err error)
	LockWithRenew(parent context.Context, key string, ttl time.Duration, maxHold time.Duration) (locked bool, ctx context.Context, cancel func(), err error)
}

func NewRedisLocker(c redis.Cmdable) ILocker {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown_hostname"
	}
	return &redisLocker{
		c:      c,
		holder: fmt.Sprintf("%s-%s", hostname, xid.New().String()),
	}
}

func NewRedisLockerWithHolder(c redis.Cmdable, holder string) ILocker {
	return &redisLocker{
		c:      c,
		holder: holder,
	}
}

type redisLocker struct {
	c      redis.Cmdable
	holder string
}

func (r *redisLocker) WithHolder(holder string) ILocker {
	r.holder = holder
	return r
}

func (r *redisLocker) LockBackoffWithRenew(parent context.Context, key string, ttl time.Duration, maxHold time.Duration) (
	locked bool, ctx context.Context, cancel func(), err error,
) {
	nop := func() {}
	locked, err = r.LockBackoff(parent, key, ttl, ttl+time.Second)
	if err != nil || !locked {
		return locked, parent, nop, err
	}

	ctx, cancel = context.WithCancel(parent)
	goroutine.Go(parent, func() {
		defer cancel()
		r.renewLock(ctx, key, ttl, maxHold)
	})
	return locked, ctx, cancel, nil
}

func (r *redisLocker) LockWithRenew(parent context.Context, key string, ttl time.Duration, maxHold time.Duration) (locked bool, ctx context.Context, cancel func(), err error) {
	nop := func() {}
	locked, err = r.Lock(parent, key, ttl)
	if err != nil || !locked {
		return locked, parent, nop, err
	}

	ctx, cancel = context.WithCancel(parent)
	goroutine.Go(parent, func() {
		defer cancel()
		r.renewLock(ctx, key, ttl, maxHold)
	})
	return locked, ctx, cancel, nil
}

func (r *redisLocker) LockBackoff(ctx context.Context, key string, expiresIn time.Duration, maxWait time.Duration) (bool, error) {
	var ok bool

	bf := backoff.NewExponentialBackOff()
	bf.InitialInterval = 50 * time.Millisecond
	bf.MaxInterval = 300 * time.Millisecond
	bf.MaxElapsedTime = maxWait

	errNotLocked := errors.New("lock hold by others")
	err := backoff.Retry(func() error {
		var err error
		ok, err = r.Lock(ctx, key, expiresIn)
		if err != nil {
			return err
		}
		if !ok {
			return errNotLocked
		}
		return nil
	}, bf)
	if err != nil {
		if errors.Is(err, errNotLocked) {
			return false, nil
		}
		return false, err
	}
	return ok, nil
}

func (r *redisLocker) Lock(ctx context.Context, key string, expiresIn time.Duration) (bool, error) {
	if expiresIn < time.Second {
		return false, fmt.Errorf("lock ttl too short")
	}
	return r.c.SetNX(ctx, key, r.holder, expiresIn).Result()
}

func (r *redisLocker) Unlock(key string) (bool, error) {
	const script = `if redis.call('GET', KEYS[1]) == ARGV[1] then redis.call('DEL', KEYS[1]); return 1; end; return 0;`
	result, err := r.c.Eval(context.Background(), script, []string{key}, r.holder).Result()
	if err != nil {
		return false, errors.WithMessage(err, "unlock with lua script")
	}
	rt, ok := result.(int64)
	if !ok {
		return false, errors.Errorf("unknown result type %T", result)
	}
	return rt == 1, nil
}

func (r *redisLocker) renewLock(ctx context.Context, key string, ttl time.Duration, maxHold time.Duration) {
	t1 := time.After(maxHold)
	t2 := time.NewTicker(gvalue.Max(time.Second, ttl>>2))
	unlock := func() {
		if _, err := r.Unlock(key); err != nil {
			logs.CtxWarn(ctx, "renew defer unlock failed, key=%s, err=%v", key, err)
		}
	}

	defer t2.Stop()
	for {
		select {
		case <-ctx.Done():
			logs.CtxInfo(ctx, "renew lock got context done, key=%s", key)
			unlock()
			return

		case <-t1:
			logs.CtxInfo(ctx, "renew lock reached max hold duration, key=%s", key)
			unlock()
			return

		case <-t2.C:
			var renewed bool
			bf := backoff.NewExponentialBackOff()
			bf.InitialInterval = 20 * time.Millisecond
			bf.MaxInterval = 100 * time.Millisecond
			bf.MaxElapsedTime = time.Millisecond * 300
			if err := backoff.Retry(func() error {
				ok, err := r.ExpireLockIn(key, ttl)
				if err != nil {
					return err
				}
				logs.CtxInfo(ctx, "renew lock success, key=%v", key)
				renewed = ok
				return nil
			}, bf); err != nil {
				logs.CtxError(ctx, "renew lock fail, key=%s, err=%v", key, err)
				return
			}
			if !renewed {
				logs.CtxInfo(ctx, "renew lock fail, mutex has already been locked, key=%s", key)
				return
			}
		}
	}
}

func (r *redisLocker) ExpireLockIn(key string, expiresIn time.Duration) (bool, error) {
	const script = `if redis.call('GET', KEYS[1]) == ARGV[1] then redis.call('PEXPIRE', KEYS[1], ARGV[2]); return 1; end; return 0;`
	result, err := r.c.Eval(context.Background(), script, []string{key}, r.holder, int64(expiresIn/time.Millisecond)).Result()
	if err != nil {
		return false, errors.WithMessage(err, "extend lock")
	}
	rt, ok := result.(int64)
	if !ok {
		return false, errors.New("unknown result type")
	}
	return rt == 1, nil
}
