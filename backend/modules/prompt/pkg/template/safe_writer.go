// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"fmt"
	"io"
	"time"
)

const (
	MaxTemplateOutputSize int64         = 1 << 20 // 1MB
	MaxTemplateTimeout    time.Duration = 10 * time.Second
	MaxRangeSize          int           = 10000
)

var ErrOutputSizeLimitExceeded = fmt.Errorf("template output size limit exceeded")

type LimitedWriter struct {
	W io.Writer
	N int64
}

func (lw *LimitedWriter) Write(p []byte) (n int, err error) {
	if lw.N <= 0 {
		return 0, ErrOutputSizeLimitExceeded
	}
	if int64(len(p)) > lw.N {
		p = p[:lw.N]
		lw.N = 0
		_, err = lw.W.Write(p)
		if err != nil {
			return 0, err
		}
		return len(p), ErrOutputSizeLimitExceeded
	}
	n, err = lw.W.Write(p)
	lw.N -= int64(n)
	return
}
