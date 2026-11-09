// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package component

import "context"

type evaluationResourceRequestIDKey struct{}

// WithEvaluationResourceRequestID carries a validated source generation to resource ports.
func WithEvaluationResourceRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, evaluationResourceRequestIDKey{}, requestID)
}

// EvaluationResourceRequestID distinguishes the known empty legacy generation from absent source identity.
func EvaluationResourceRequestID(ctx context.Context) (string, bool) {
	requestID, known := ctx.Value(evaluationResourceRequestIDKey{}).(string)
	return requestID, known
}
