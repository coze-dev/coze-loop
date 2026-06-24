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
}

type task = func() error

type pool struct {
	p     *ants.Pool
	tasks []task
}

func (p *pool) Add(task func() error) {
	p.tasks = append(p.tasks, task)
}

func (p *pool) Exec(ctx context.Context) error {
	return p.exec(ctx, false)
}

func (p *pool) ExecAll(ctx context.Context) error {
	return p.exec(ctx, true)
}

func (p *pool) exec(ctx context.Context, ignoreErr bool) error {
	defer p.p.Release()

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
