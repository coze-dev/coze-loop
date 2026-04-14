// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLimitedWriter_NormalWrite(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	lw := &LimitedWriter{W: &buf, N: 100}

	n, err := lw.Write([]byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "hello", buf.String())
	assert.Equal(t, int64(95), lw.N)
}

func TestLimitedWriter_ExactLimit(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	lw := &LimitedWriter{W: &buf, N: 5}

	n, err := lw.Write([]byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "hello", buf.String())
	assert.Equal(t, int64(0), lw.N)
}

func TestLimitedWriter_ExceedsLimit(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	lw := &LimitedWriter{W: &buf, N: 3}

	n, err := lw.Write([]byte("hello"))
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrOutputSizeLimitExceeded))
	assert.Equal(t, 3, n)
	assert.Equal(t, "hel", buf.String())
}

func TestLimitedWriter_ZeroRemaining(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	lw := &LimitedWriter{W: &buf, N: 0}

	n, err := lw.Write([]byte("hello"))
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrOutputSizeLimitExceeded))
	assert.Equal(t, 0, n)
	assert.Equal(t, "", buf.String())
}

func TestLimitedWriter_MultipleWrites(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	lw := &LimitedWriter{W: &buf, N: 10}

	n, err := lw.Write([]byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)

	n, err = lw.Write([]byte("world!"))
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrOutputSizeLimitExceeded))
	assert.Equal(t, 5, n)
	assert.Equal(t, "helloworld", buf.String())
}
