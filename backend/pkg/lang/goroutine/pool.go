// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package goroutine

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/panjf2000/ants/v2"
)

func NewPool(size int) (IPool, error) {
	if size <= 0 {
		return nil, fmt.Errorf("pool size must be greater than 0")
	}
	p, err := ants.NewPool(size)
	if err != nil {
		return nil, fmt.Errorf("ants new pool fail, size=%d, err=%w", size, err)
	}
	return &pool{
		p:     p,
		tasks: make([]task, 0),
	}, nil
}

type IPool interface {
	Add(task func() error)
	Exec(ctx context.Context) error
	ExecAll(ctx context.Context) error
	// Release 释放底层 ants pool 及其常驻协程(purge / ticktock)。幂等,可多次调用。
	// 调用方应在 NewPool 成功后立即 defer Release(),确保即使不走 Exec/ExecAll(如中途提前 return)
	// 也能释放,避免协程泄漏。
	Release()
}

type task = func() error

type pool struct {
	p           *ants.Pool
	tasks       []task
	releaseOnce sync.Once
}

func (p *pool) Add(task func() error) {
	p.tasks = append(p.tasks, task)
}

// Release 幂等释放底层 ants pool;exec() 的 defer 与调用方的 defer 都会走到这里,sync.Once 保证只释放一次。
func (p *pool) Release() {
	p.releaseOnce.Do(func() {
		p.p.Release()
	})
}

func (p *pool) Exec(ctx context.Context) error {
	return p.exec(ctx, false)
}

func (p *pool) ExecAll(ctx context.Context) error {
	return p.exec(ctx, true)
}

func (p *pool) exec(ctx context.Context, ignoreErr bool) error {
	defer p.Release()

	var (
		mu   sync.Mutex
		errs []error
		wg   sync.WaitGroup
	)

	appendErr := func(err error) {
		mu.Lock()
		errs = append(errs, err)
		mu.Unlock()
	}

	hasErr := func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(errs) > 0
	}

	for idx := range p.tasks {
		if !ignoreErr && hasErr() {
			break
		}

		t := p.tasks[idx]

		wg.Add(1)
		if err := p.p.Submit(func() {
			defer wg.Done()
			defer Recovery(ctx)

			select {
			case <-ctx.Done():
				appendErr(ctx.Err())
				return

			default:
				if !ignoreErr && hasErr() {
					return
				}
				if err := t(); err != nil {
					appendErr(err)
				}
				return
			}
		}); err != nil {
			return fmt.Errorf("pool submit fail, err=%w", err)
		}
	}

	wg.Wait()

	if len(errs) == 0 {
		return nil
	}
	if ignoreErr {
		return errors.Join(errs...)
	}
	return errs[0]
}
