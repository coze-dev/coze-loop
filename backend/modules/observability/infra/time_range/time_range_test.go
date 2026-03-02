// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package time_range

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewTimeRangeProvider(t *testing.T) {
	provider := NewTimeRangeProvider()
	assert.NotNil(t, provider)
	assert.IsType(t, &TimeRangeProvider{}, provider)
}

func TestTimeRangeProvider_GetTimeRange(t *testing.T) {
	provider := NewTimeRangeProvider()
	ctx := context.Background()

	t.Run("returns nil for any input", func(t *testing.T) {
		start, end := provider.GetTimeRange(ctx, "workspace1", "log1", "trace1")
		assert.Nil(t, start)
		assert.Nil(t, end)
	})
}
