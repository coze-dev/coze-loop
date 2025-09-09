// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package goroutine

import (
	"context"
	"runtime"

	"github.com/bytedance/gopkg/util/logger"
)

// GoSafe Safely start a goroutine, which will automatically recover from panics and print stack information.
func GoSafe(ctx context.Context, fn func()) {
	go func() {
		defer func() {
			if e := recover(); e != nil {
				const size = 64 << 10
				buf := make([]byte, size)
				buf = buf[:runtime.Stack(buf, false)]
				logger.CtxErrorf(ctx, "goroutine panic: %s: %s", e, buf)
			}
		}()
		fn()
	}()
}
