// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package contexts

import (
	"context"
)

type ctxWriteDBKey struct{}

type ctxWriteDBVal struct{}

type ctxExptSpaceIDKey struct{}

func WithCtxWriteDB(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxWriteDBKey{}, ctxWriteDBVal{})
}

func CtxWriteDB(ctx context.Context) bool {
	return ctx.Value(ctxWriteDBKey{}) != nil
}

// WithCtxExptSpaceID carries the experiment's space for model access and consumption decisions.
// Direct runs and debug use the caller's space.
// Resource ownership, tracing and callback workspace IDs remain separate.
func WithCtxExptSpaceID(ctx context.Context, exptSpaceID int64) context.Context {
	return context.WithValue(ctx, ctxExptSpaceIDKey{}, exptSpaceID)
}

// CtxExptSpaceID returns the experiment's space, or zero when no space was supplied.
func CtxExptSpaceID(ctx context.Context) int64 {
	exptSpaceID, _ := ctx.Value(ctxExptSpaceIDKey{}).(int64)
	return exptSpaceID
}
