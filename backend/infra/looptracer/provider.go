// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package looptracer

import (
	"context"

	cozeloop "github.com/coze-dev/cozeloop-go"
)

var tracer Tracer = &noopTracer{c: &cozeloop.NoopClient{}}

type Tracer interface {
	// StartSpan Generate a span that automatically links to the previous span in the context.
	// The start time of the span starts counting from the call of StartSpan.
	// The generated span will be automatically written into the context.
	// Subsequent spans that need to be chained should call StartSpan based on the new context.
	StartSpan(ctx context.Context, name, spanType string, opts ...StartSpanOption) (context.Context, Span)
	// GetSpanFromContext Get the span from the context.
	GetSpanFromContext(ctx context.Context) Span
	// Flush Force the reporting of spans in the queue.
	Flush(ctx context.Context)
	// Inject Inject the tracer into the context.
	Inject(ctx context.Context) context.Context
}

type Span interface {
	cozeloop.Span
	SetCallType(callType string)
}

// GetTracer Get the tracer. Must call InitTracer first.
func GetTracer() Tracer {
	return tracer
}

// InitTracer Init the tracer. Must call before GetTracer.
func InitTracer(t Tracer) {
	tracer = t
}
