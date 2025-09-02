// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package looptracer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/looptracer/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

//go:generate mockgen -destination mocks/cozeloop_client.go -package mocks github.com/coze-dev/cozeloop-go Client
//go:generate mockgen -destination mocks/cozeloop_span.go -package mocks github.com/coze-dev/cozeloop-go Span

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
		{
			name:   "WithChildOf选项",
			option: WithChildOf(mocks.NewMockSpan(nil)),
			checkResult: func(t *testing.T, opts *StartSpanOptions) {
				assert.True(t, opts.ChildOf != nil)
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

// Test NewTracer with valid client
func TestNewTracer_WithValidClient(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	tracer := NewTracer(mockClient)

	assert.NotNil(t, tracer)

	tracerImpl, ok := tracer.(*TracerImpl)
	assert.True(t, ok)
	assert.Equal(t, mockClient, tracerImpl.Client)
}

// Test TracerImpl.StartSpan method
func TestTracerImpl_StartSpan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setupMock   func(*mocks.MockClient, *mocks.MockSpan)
		ctx         context.Context
		spanName    string
		spanType    string
		opts        []StartSpanOption
		expectedCtx context.Context
	}{
		{
			name: "正常启动Span无选项",
			setupMock: func(mc *mocks.MockClient, ms *mocks.MockSpan) {
				mc.EXPECT().StartSpan(gomock.Any(), "test-span", "test-type", gomock.Any()).
					Return(context.Background(), ms)
			},
			ctx:         context.Background(),
			spanName:    "test-span",
			spanType:    "test-type",
			opts:        nil,
			expectedCtx: context.Background(),
		},
		{
			name: "带StartTime选项启动Span",
			setupMock: func(mc *mocks.MockClient, ms *mocks.MockSpan) {
				mc.EXPECT().StartSpan(gomock.Any(), "test-span", "test-type", gomock.Any()).
					Return(context.Background(), ms)
			},
			ctx:         context.Background(),
			spanName:    "test-span",
			spanType:    "test-type",
			opts:        []StartSpanOption{WithStartTime(time.Unix(1640995200, 0))},
			expectedCtx: context.Background(),
		},
		{
			name: "带StartNewTrace选项启动Span",
			setupMock: func(mc *mocks.MockClient, ms *mocks.MockSpan) {
				mc.EXPECT().StartSpan(gomock.Any(), "test-span", "test-type", gomock.Any()).
					Return(context.Background(), ms)
			},
			ctx:         context.Background(),
			spanName:    "test-span",
			spanType:    "test-type",
			opts:        []StartSpanOption{WithStartNewTrace()},
			expectedCtx: context.Background(),
		},
		{
			name: "带WorkspaceID选项启动Span",
			setupMock: func(mc *mocks.MockClient, ms *mocks.MockSpan) {
				mc.EXPECT().StartSpan(gomock.Any(), "test-span", "test-type", gomock.Any()).
					Return(context.Background(), ms)
			},
			ctx:         context.Background(),
			spanName:    "test-span",
			spanType:    "test-type",
			opts:        []StartSpanOption{WithSpanWorkspaceID("workspace-123")},
			expectedCtx: context.Background(),
		},
		{
			name: "多选项组合启动Span",
			setupMock: func(mc *mocks.MockClient, ms *mocks.MockSpan) {
				mc.EXPECT().StartSpan(gomock.Any(), "test-span", "test-type", gomock.Any()).
					Return(context.Background(), ms)
			},
			ctx:      context.Background(),
			spanName: "test-span",
			spanType: "test-type",
			opts: []StartSpanOption{
				WithStartTime(time.Unix(1640995200, 0)),
				WithStartNewTrace(),
				WithSpanWorkspaceID("workspace-123"),
			},
			expectedCtx: context.Background(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mocks.NewMockClient(ctrl)
			mockSpan := mocks.NewMockSpan(ctrl)
			tt.setupMock(mockClient, mockSpan)

			tracer := &TracerImpl{Client: mockClient}

			resultCtx, span := tracer.StartSpan(tt.ctx, tt.spanName, tt.spanType, tt.opts...)

			assert.Equal(t, tt.expectedCtx, resultCtx)
			assert.NotNil(t, span)
			assert.IsType(t, SpanImpl{}, span)
		})
	}
}

// Test TracerImpl.StartSpan with nil client
func TestTracerImpl_StartSpan_NilClient(t *testing.T) {
	t.Parallel()

	tracer := &TracerImpl{Client: nil}

	assert.Panics(t, func() {
		tracer.StartSpan(context.Background(), "test", "test")
	})
}

// Test TracerImpl.GetSpanFromContext method
func TestTracerImpl_GetSpanFromContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setupMock func(*mocks.MockClient, *mocks.MockSpan)
		ctx       context.Context
		expectNil bool
	}{
		{
			name: "从context获取有效Span",
			setupMock: func(mc *mocks.MockClient, ms *mocks.MockSpan) {
				mc.EXPECT().GetSpanFromContext(gomock.Any()).Return(ms)
			},
			ctx:       context.Background(),
			expectNil: false,
		},
		{
			name: "从context获取nil Span",
			setupMock: func(mc *mocks.MockClient, ms *mocks.MockSpan) {
				mc.EXPECT().GetSpanFromContext(gomock.Any()).Return(nil)
			},
			ctx:       context.Background(),
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mocks.NewMockClient(ctrl)
			mockSpan := mocks.NewMockSpan(ctrl)
			tt.setupMock(mockClient, mockSpan)

			tracer := &TracerImpl{Client: mockClient}
			span := tracer.GetSpanFromContext(tt.ctx)

			assert.NotNil(t, span) // SpanImpl 总是返回非nil
			spanImpl, ok := span.(SpanImpl)
			assert.True(t, ok)

			if tt.expectNil {
				assert.Nil(t, spanImpl.LoopSpan)
			} else {
				assert.NotNil(t, spanImpl.LoopSpan)
			}
		})
	}
}

// Test TracerImpl.Flush method
func TestTracerImpl_Flush(t *testing.T) {
	t.Parallel()

	t.Run("正常Flush调用", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockClient := mocks.NewMockClient(ctrl)
		mockClient.EXPECT().Flush(gomock.Any()).Return()

		tracer := &TracerImpl{Client: mockClient}

		assert.NotPanics(t, func() {
			tracer.Flush(context.Background())
		})
	})

	t.Run("nil Client Flush调用", func(t *testing.T) {
		t.Parallel()

		tracer := &TracerImpl{Client: nil}

		assert.Panics(t, func() {
			tracer.Flush(context.Background())
		})
	})
}

// Test TracerImpl.Inject method
func TestTracerImpl_Inject(t *testing.T) {
	t.Parallel()

	tracer := &TracerImpl{}
	ctx := context.Background()

	result := tracer.Inject(ctx)
	assert.Equal(t, ctx, result)
}

// Test TracerImpl.InjectW3CTraceContext method
func TestTracerImpl_InjectW3CTraceContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupMock      func(*mocks.MockClient, *mocks.MockSpan)
		expectedResult map[string]string
	}{
		{
			name: "正常W3C header转换",
			setupMock: func(mc *mocks.MockClient, ms *mocks.MockSpan) {
				mc.EXPECT().GetSpanFromContext(gomock.Any()).Return(ms)
				ms.EXPECT().ToHeader().Return(map[string]string{
					TraceContextHeaderParent:  "00-1234567890abcdef1234567890abcdef-1234567890abcdef-01",
					TraceContextHeaderBaggage: "key1=value1,key2=value2",
				}, nil)
			},
			expectedResult: map[string]string{
				TraceContextHeaderParentW3C:  "00-1234567890abcdef1234567890abcdef-1234567890abcdef-01",
				TraceContextHeaderBaggageW3C: "key1=value1,key2=value2",
			},
		},
		{
			name: "只有traceparent的header转换",
			setupMock: func(mc *mocks.MockClient, ms *mocks.MockSpan) {
				mc.EXPECT().GetSpanFromContext(gomock.Any()).Return(ms)
				ms.EXPECT().ToHeader().Return(map[string]string{
					TraceContextHeaderParent: "00-1234567890abcdef1234567890abcdef-1234567890abcdef-01",
				}, nil)
			},
			expectedResult: map[string]string{
				TraceContextHeaderParentW3C: "00-1234567890abcdef1234567890abcdef-1234567890abcdef-01",
			},
		},
		{
			name: "只有tracestate的header转换",
			setupMock: func(mc *mocks.MockClient, ms *mocks.MockSpan) {
				mc.EXPECT().GetSpanFromContext(gomock.Any()).Return(ms)
				ms.EXPECT().ToHeader().Return(map[string]string{
					TraceContextHeaderBaggage: "key1=value1",
				}, nil)
			},
			expectedResult: map[string]string{
				TraceContextHeaderBaggageW3C: "key1=value1",
			},
		},
		{
			name: "空header处理",
			setupMock: func(mc *mocks.MockClient, ms *mocks.MockSpan) {
				mc.EXPECT().GetSpanFromContext(gomock.Any()).Return(ms)
				ms.EXPECT().ToHeader().Return(map[string]string{}, nil)
			},
			expectedResult: map[string]string{},
		},
		{
			name: "ToHeader返回错误",
			setupMock: func(mc *mocks.MockClient, ms *mocks.MockSpan) {
				mc.EXPECT().GetSpanFromContext(gomock.Any()).Return(ms)
				ms.EXPECT().ToHeader().Return(map[string]string{}, errors.New("header error"))
			},
			expectedResult: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mocks.NewMockClient(ctrl)
			mockSpan := mocks.NewMockSpan(ctrl)
			tt.setupMock(mockClient, mockSpan)

			tracer := &TracerImpl{Client: mockClient}
			result := tracer.InjectW3CTraceContext(context.Background())

			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

// Test header constants
func TestHeaderConstants(t *testing.T) {
	t.Parallel()

	// 验证header常量值的正确性
	assert.Equal(t, "X-Cozeloop-Traceparent", TraceContextHeaderParent)
	assert.Equal(t, "X-Cozeloop-Tracestate", TraceContextHeaderBaggage)
	assert.Equal(t, "traceparent", TraceContextHeaderParentW3C)
	assert.Equal(t, "tracestate", TraceContextHeaderBaggageW3C)
}

// Test StartSpanOptions zero values
func TestStartSpanOptions_ZeroValues(t *testing.T) {
	t.Parallel()

	opts := &StartSpanOptions{}

	assert.True(t, opts.StartTime.IsZero())
	assert.False(t, opts.StartNewTrace)
	assert.Empty(t, opts.WorkspaceID)
}

// Test multiple options application
func TestStartSpanOptions_MultipleOptions(t *testing.T) {
	t.Parallel()

	startTime := time.Unix(1640995200, 0)
	workspaceID := "test-workspace-123"

	opts := &StartSpanOptions{}

	// 应用多个选项
	WithStartTime(startTime)(opts)
	WithStartNewTrace()(opts)
	WithSpanWorkspaceID(workspaceID)(opts)

	assert.Equal(t, startTime, opts.StartTime)
	assert.True(t, opts.StartNewTrace)
	assert.Equal(t, workspaceID, opts.WorkspaceID)
}

// Test noopTracer.Flush method
func TestNoopTracer_Flush(t *testing.T) {
	t.Parallel()

	tracer := &noopTracer{}
	assert.NotPanics(t, func() {
		tracer.Flush(context.Background())
	})
}

// Test noopTracer.SetCallType method
func TestNoopTracer_SetCallType(t *testing.T) {
	t.Parallel()

	tracer := &noopTracer{}
	assert.NotPanics(t, func() {
		tracer.SetCallType("test-call-type")
	})
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
