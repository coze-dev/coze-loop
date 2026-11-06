// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package contexts

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCtxWithWriteDB(t *testing.T) {
	ctx := context.Background()
	ctx = WithCtxWriteDB(ctx)
	assert.True(t, CtxWriteDB(ctx))
	assert.False(t, CtxWriteDB(context.Background()))
}

func TestCtxExptSpaceID(t *testing.T) {
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	parent := WithCtxExptSpaceID(WithCtxWriteDB(base), 111)
	child := WithCtxExptSpaceID(parent, 222)

	assert.Zero(t, CtxExptSpaceID(base))
	assert.Equal(t, int64(111), CtxExptSpaceID(parent))
	assert.Equal(t, int64(222), CtxExptSpaceID(child))
	assert.True(t, CtxWriteDB(child))
	assert.Zero(t, CtxExptSpaceID(WithCtxExptSpaceID(parent, 0)))
	assert.Equal(t, int64(111), CtxExptSpaceID(parent))

	cancel()
	assert.ErrorIs(t, child.Err(), context.Canceled)
}
