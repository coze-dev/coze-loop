// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package looptracer

import (
	"context"
	"time"

	cozeloop "github.com/coze-dev/cozeloop-go"
)

var _ Tracer = (*TracerImpl)(nil)

type TracerImpl struct {
	cozeloop.Client
}

func NewTracer(client cozeloop.Client) Tracer {
	return &TracerImpl{Client: client}
}

type StartSpanOptions struct {
	StartTime     time.Time
	StartNewTrace bool
	WorkspaceID   string
}

type StartSpanOption = func(o *StartSpanOptions)

// WithStartTime Set the start time of the span.
// This field is optional. If not specified, the time when StartSpan is called will be used as the default.
func WithStartTime(t time.Time) StartSpanOption {
	return func(ops *StartSpanOptions) {
		ops.StartTime = t
	}
}

// WithStartNewTrace Set the parent span of the span.
// This field is optional. If specified, start a span of a new trace.
func WithStartNewTrace() StartSpanOption {
	return func(ops *StartSpanOptions) {
		ops.StartNewTrace = true
	}
}

// WithSpanWorkspaceID Set the workspaceID of the span.
// This field is inner field. You should not set it.
func WithSpanWorkspaceID(workspaceID string) StartSpanOption {
	return func(ops *StartSpanOptions) {
		ops.WorkspaceID = workspaceID
	}
}

func (t *TracerImpl) StartSpan(ctx context.Context, name, spanType string, opts ...StartSpanOption) (context.Context, Span) {
	options := &StartSpanOptions{}
	for _, opt := range opts {
		opt(options)
	}

	cozeLoopOpts := make([]cozeloop.StartSpanOption, 0, len(opts))
	if !options.StartTime.IsZero() {
		cozeLoopOpts = append(cozeLoopOpts, cozeloop.WithStartTime(options.StartTime))
	}
	if options.StartNewTrace {
		cozeLoopOpts = append(cozeLoopOpts, cozeloop.WithStartNewTrace())
	}
	if options.WorkspaceID != "" {
		cozeLoopOpts = append(cozeLoopOpts, cozeloop.WithSpanWorkspaceID(options.WorkspaceID))
	}
	ctx, span := t.Client.StartSpan(ctx, name, spanType, cozeLoopOpts...)
	return ctx, SpanImpl{
		LoopSpan: span,
	}
}

func (t *TracerImpl) GetSpanFromContext(ctx context.Context) Span {
	span := t.Client.GetSpanFromContext(ctx)
	return SpanImpl{
		LoopSpan: span,
	}
}

func (t *TracerImpl) Inject(ctx context.Context) context.Context {
	return ctx
}

type noopTracer struct {
	c cozeloop.Client
}

func (d *noopTracer) StartSpan(ctx context.Context, name, spanType string, opts ...StartSpanOption) (context.Context, Span) {
	return ctx, &noopSpan{}
}

func (d *noopTracer) GetSpanFromContext(ctx context.Context) Span {
	return &noopSpan{}
}

func (d *noopTracer) Flush(ctx context.Context) {}

func (d *noopTracer) Inject(ctx context.Context) context.Context {
	return ctx
}

func (d *noopTracer) SetCallType(callType string) {
}
