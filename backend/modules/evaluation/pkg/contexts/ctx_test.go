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
