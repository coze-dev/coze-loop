// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package looptracer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewTracer(t *testing.T) {
	t.Parallel()

	t.Run("nil客户端创建Tracer", func(t *testing.T) {
		t.Parallel()
		tracer := NewTracer(nil)
		assert.NotNil(t, tracer)

		tracerImpl, ok := tracer.(*TracerImpl)
		assert.True(t, ok)
		assert.Nil(t, tracerImpl.Client)
	})
}

func TestStartSpanOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		option      StartSpanOption
		checkResult func(t *testing.T, opts *StartSpanOptions)
	}{
		{
			name:   "WithStartTime选项",
			option: WithStartTime(time.Unix(1640995200, 0)),
			checkResult: func(t *testing.T, opts *StartSpanOptions) {
				assert.Equal(t, time.Unix(1640995200, 0), opts.StartTime)
			},
		},
		{
			name:   "WithStartNewTrace选项",
			option: WithStartNewTrace(),
			checkResult: func(t *testing.T, opts *StartSpanOptions) {
				assert.True(t, opts.StartNewTrace)
			},
		},
		{
			name:   "WithSpanWorkspaceID选项",
			option: WithSpanWorkspaceID("test-workspace"),
			checkResult: func(t *testing.T, opts *StartSpanOptions) {
				assert.Equal(t, "test-workspace", opts.WorkspaceID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := &StartSpanOptions{}
			tt.option(opts)

			if tt.checkResult != nil {
				tt.checkResult(t, opts)
			}
		})
	}
}

func TestNoopTracer(t *testing.T) {
	t.Parallel()

	tracer := &noopTracer{}
	ctx := context.Background()

	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "StartSpan返回原context和noop span",
			test: func(t *testing.T) {
				resultCtx, span := tracer.StartSpan(ctx, "test", "test")
				assert.Equal(t, ctx, resultCtx)
				assert.IsType(t, &noopSpan{}, span)
			},
		},
		{
			name: "GetSpanFromContext返回noop span",
			test: func(t *testing.T) {
				span := tracer.GetSpanFromContext(ctx)
				assert.IsType(t, &noopSpan{}, span)
			},
		},
		{
			name: "Flush不会panic",
			test: func(t *testing.T) {
				assert.NotPanics(t, func() {
					tracer.Flush(ctx)
				})
			},
		},
		{
			name: "Inject返回原context",
			test: func(t *testing.T) {
				result := tracer.Inject(ctx)
				assert.Equal(t, ctx, result)
			},
		},
		{
			name: "InjectW3CTraceContext返回空map",
			test: func(t *testing.T) {
				result := tracer.InjectW3CTraceContext(ctx)
				assert.Equal(t, map[string]string{}, result)
			},
		},
		{
			name: "SetCallType不会panic",
			test: func(t *testing.T) {
				assert.NotPanics(t, func() {
					tracer.SetCallType("test")
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.test(t)
		})
	}
}
