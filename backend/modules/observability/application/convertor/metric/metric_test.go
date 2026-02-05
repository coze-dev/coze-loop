// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metric

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMinimizePie(t *testing.T) {
	t.Run("size <= 1000 should not change", func(t *testing.T) {
		pie := make(map[string]string)
		for i := 0; i < 500; i++ {
			pie[fmt.Sprintf("key-%d", i)] = "1"
		}
		result := minimizePie(pie)
		assert.Equal(t, 500, len(result))
	})

	t.Run("size > 1000 should truncate by custom sort", func(t *testing.T) {
		pie := make(map[string]string)
		// Add 1000 items with value "1" (length 1)
		for i := 0; i < 1000; i++ {
			pie[fmt.Sprintf("small-%d", i)] = "2"
		}

		// Add 5 items with value "100" (length 3)
		for i := 0; i < 5; i++ {
			pie[fmt.Sprintf("large-%d", i)] = "100"
		}

		// Add 5 items with value "10" (length 2)
		for i := 0; i < 5; i++ {
			pie[fmt.Sprintf("medium-%d", i)] = "91"
		}

		// Total 1010 items.
		// Sort order desc: "100" (len 3) > "10" (len 2) > "1" (len 1)
		// Expected kept:
		// 5 "large" items (len 3)
		// 5 "medium" items (len 2)
		// 990 "small" items (len 1)
		// Total kept: 1000.
		// Removed: 10 "small" items.

		result := minimizePie(pie)
		assert.Equal(t, 1000, len(result))

		// Verify large and medium are kept
		for i := 0; i < 5; i++ {
			_, ok := result[fmt.Sprintf("large-%d", i)]
			assert.True(t, ok, "large item %d missing", i)
			_, ok = result[fmt.Sprintf("medium-%d", i)]
			assert.True(t, ok, "medium item %d missing", i)
		}

		// Verify some small items are removed.
		// Since map iteration order is random when building the slice,
		// and the sort is stable for equal values (depends on implementation, actually sort.Slice is not guaranteed stable,
		// but here values are identical "1").
		// Wait, if values are identical "1", their relative order is undefined after sort.
		// So we can't deterministically say WHICH "small" items are removed, only that 10 are removed.

		removedCount := 0
		for i := 0; i < 1000; i++ {
			if _, ok := result[fmt.Sprintf("small-%d", i)]; !ok {
				removedCount++
			}
		}
		fmt.Println("===", removedCount)
		assert.Equal(t, 10, removedCount)
	})

	t.Run("size > 10000 should filter 0 and 1 first", func(t *testing.T) {
		pie := make(map[string]string)
		// 5000 items with "0"
		for i := 0; i < 5000; i++ {
			pie[fmt.Sprintf("zero-%d", i)] = "0"
		}
		// 5000 items with "1"
		for i := 0; i < 5000; i++ {
			pie[fmt.Sprintf("one-%d", i)] = "1"
		}
		// 500 items with "2"
		for i := 0; i < 500; i++ {
			pie[fmt.Sprintf("two-%d", i)] = "2"
		}

		// Total 10500.
		// Filter "0" and "1" -> removes 10000 items.
		// Remaining 500 items with "2".
		// 500 <= 1000, so no truncation.

		result := minimizePie(pie)
		assert.Equal(t, 500, len(result))

		for k, v := range result {
			assert.Equal(t, "2", v)
			assert.Contains(t, k, "two-")
		}
	})

	t.Run("size > 10000 filter 0 and 1 then truncate", func(t *testing.T) {
		pie := make(map[string]string)
		// 9000 items with "1"
		for i := 0; i < 9000; i++ {
			pie[fmt.Sprintf("one-%d", i)] = "1"
		}
		// 1500 items with "2"
		for i := 0; i < 1500; i++ {
			pie[fmt.Sprintf("two-%d", i)] = "2"
		}

		// Total 10500.
		// Filter "1" -> removes 9000 items.
		// Remaining 1500 items with "2".
		// 1500 > 1000, truncate to 1000.

		result := minimizePie(pie)
		assert.Equal(t, 1000, len(result))

		for _, v := range result {
			assert.Equal(t, "2", v)
		}
	})
}
