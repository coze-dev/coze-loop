// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestHookWorkerConfigRejectsCapacityOverflowWithoutClamping(t *testing.T) {
	base := HookRuntimeConfig{WorkerConcurrency: 8, WorkspaceConcurrency: 2, ScanBatchSize: 100, ScanIntervalSeconds: 10}
	for _, tc := range []struct {
		name   string
		mutate func(*HookRuntimeConfig)
		valid  bool
	}{
		{"default", func(*HookRuntimeConfig) {}, true},
		{"supported_max", func(c *HookRuntimeConfig) {
			c.WorkerConcurrency = 128
			c.WorkspaceConcurrency = 32
			c.ScanIntervalSeconds = 3600
		}, true},
		{"int32_max_pool", func(c *HookRuntimeConfig) { c.WorkerConcurrency = 2147483647 }, false},
		{"int32_max_space", func(c *HookRuntimeConfig) { c.WorkspaceConcurrency = 2147483647 }, false},
		{"space_exceeds_pool", func(c *HookRuntimeConfig) { c.WorkspaceConcurrency = 9 }, false},
		{"zero_batch", func(c *HookRuntimeConfig) { c.ScanBatchSize = 0 }, false},
		{"large_batch", func(c *HookRuntimeConfig) { c.ScanBatchSize = 101 }, false},
		{"zero_interval", func(c *HookRuntimeConfig) { c.ScanIntervalSeconds = 0 }, false},
		{"large_interval", func(c *HookRuntimeConfig) { c.ScanIntervalSeconds = 3601 }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := base
			tc.mutate(&c)
			if tc.valid {
				require.NoError(t, ValidateHookWorkerConfig(c))
			} else {
				require.Error(t, ValidateHookWorkerConfig(c))
			}
		})
	}
}
