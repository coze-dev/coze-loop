// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package contexts

import (
	"context"
)

type ctxWriteDBKey struct{}

type ctxWriteDBVal struct{}

func WithCtxWriteDB(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxWriteDBKey{}, ctxWriteDBVal{})
}

func CtxWriteDB(ctx context.Context) bool {
	return ctx.Value(ctxWriteDBKey{}) != nil
}
