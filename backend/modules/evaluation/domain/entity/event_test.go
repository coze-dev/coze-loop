// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/pkg/ctxcache"
)

func TestExptItemEvalEvent_WithCtxTargetCalled(t *testing.T) {
	t.Run("ctx not init: Store is no-op", func(t *testing.T) {
		ctx := context.Background()
		e := &ExptItemEvalEvent{}

		e.WithCtxTargetCalled(ctx)
		_, ok := ctxcache.Get[struct{}](ctx, ctxTargetCalledCacheKey{})
		assert.False(t, ok)
	})

	t.Run("ctx init: should be stored and retrievable", func(t *testing.T) {
		ctx := ctxcache.Init(context.Background())
		e := &ExptItemEvalEvent{}

		e.WithCtxTargetCalled(ctx)
		_, ok := ctxcache.Get[struct{}](ctx, ctxTargetCalledCacheKey{})
		assert.True(t, ok)
	})
}

func TestExptItemEvalEvent_CtxTargetCalled(t *testing.T) {
	t.Run("ctx not init: always false", func(t *testing.T) {
		ctx := context.Background()
		e := &ExptItemEvalEvent{}

		assert.False(t, e.CtxTargetCalled(ctx))
	})

	t.Run("ctx init but not marked: false", func(t *testing.T) {
		ctx := ctxcache.Init(context.Background())
		e := &ExptItemEvalEvent{}

		assert.False(t, e.CtxTargetCalled(ctx))
	})

	t.Run("ctx init and marked: true", func(t *testing.T) {
		ctx := ctxcache.Init(context.Background())
		e := &ExptItemEvalEvent{}

		e.WithCtxTargetCalled(ctx)
		assert.True(t, e.CtxTargetCalled(ctx))
	})

	t.Run("different contexts are isolated", func(t *testing.T) {
		ctx1 := ctxcache.Init(context.Background())
		ctx2 := ctxcache.Init(context.Background())
		e := &ExptItemEvalEvent{}

		e.WithCtxTargetCalled(ctx1)
		assert.True(t, e.CtxTargetCalled(ctx1))
		assert.False(t, e.CtxTargetCalled(ctx2))
	})
}

func TestExptItemEvalEvent_IgnoreExistedEvaluatorResult(t *testing.T) {
	t.Run("ctx init and target called: always false", func(t *testing.T) {
		ctx := ctxcache.Init(context.Background())

		tests := []struct {
			name    string
			runMode ExptRunMode
			retry   int
		}{
			{name: "retry all with retryTimes=0", runMode: EvaluationModeRetryAll, retry: 0},
			{name: "retry items with retryTimes=0", runMode: EvaluationModeRetryItems, retry: 0},
			{name: "retry all with retryTimes=1", runMode: EvaluationModeRetryAll, retry: 1},
			{name: "submit mode", runMode: EvaluationModeSubmit, retry: 0},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				e := &ExptItemEvalEvent{ExptRunMode: tt.runMode, RetryTimes: tt.retry}
				e.WithCtxTargetCalled(ctx)
				assert.False(t, e.IgnoreExistedEvaluatorResult(ctx))
			})
		}
	})

	t.Run("ctx init but target not called: falls back to ignoreExistedResult", func(t *testing.T) {
		ctx := ctxcache.Init(context.Background())

		e1 := &ExptItemEvalEvent{ExptRunMode: EvaluationModeRetryAll, RetryTimes: 0}
		assert.True(t, e1.IgnoreExistedEvaluatorResult(ctx))

		e2 := &ExptItemEvalEvent{ExptRunMode: EvaluationModeRetryItems, RetryTimes: 0}
		assert.True(t, e2.IgnoreExistedEvaluatorResult(ctx))

		e3 := &ExptItemEvalEvent{ExptRunMode: EvaluationModeRetryAll, RetryTimes: 1}
		assert.False(t, e3.IgnoreExistedEvaluatorResult(ctx))

		e4 := &ExptItemEvalEvent{ExptRunMode: EvaluationModeSubmit, RetryTimes: 0}
		assert.False(t, e4.IgnoreExistedEvaluatorResult(ctx))
	})

	t.Run("ctx not init: falls back to ignoreExistedResult", func(t *testing.T) {
		ctx := context.Background()

		e1 := &ExptItemEvalEvent{ExptRunMode: EvaluationModeRetryAll, RetryTimes: 0}
		assert.True(t, e1.IgnoreExistedEvaluatorResult(ctx))

		e2 := &ExptItemEvalEvent{ExptRunMode: EvaluationModeRetryItems, RetryTimes: 0}
		assert.True(t, e2.IgnoreExistedEvaluatorResult(ctx))

		e3 := &ExptItemEvalEvent{ExptRunMode: EvaluationModeRetryAll, RetryTimes: 1}
		assert.False(t, e3.IgnoreExistedEvaluatorResult(ctx))

		e4 := &ExptItemEvalEvent{ExptRunMode: EvaluationModeSubmit, RetryTimes: 0}
		assert.False(t, e4.IgnoreExistedEvaluatorResult(ctx))
	})
}
