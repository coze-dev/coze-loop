// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

const (
	HookWorkerMaxConcurrency          int32 = 128
	HookWorkerMaxWorkspaceConcurrency int32 = 32
	HookWorkerMaxScanIntervalSeconds  int32 = 3600
)

// ValidateHookWorkerConfig rejects unsupported capacity rather than silently
// clamping it. These are memory-safety bounds, not measured production capacity.
func ValidateHookWorkerConfig(c HookRuntimeConfig) error {
	if c.WorkerConcurrency < 1 || c.WorkerConcurrency > HookWorkerMaxConcurrency ||
		c.WorkspaceConcurrency < 1 || c.WorkspaceConcurrency > HookWorkerMaxWorkspaceConcurrency || c.WorkspaceConcurrency > c.WorkerConcurrency ||
		c.ScanIntervalSeconds < 1 || c.ScanIntervalSeconds > HookWorkerMaxScanIntervalSeconds || c.ScanBatchSize < 1 || c.ScanBatchSize > 100 {
		return invalidParam("invalid hook worker capacity or scan configuration")
	}
	return nil
}
