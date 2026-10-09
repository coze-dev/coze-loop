// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

const (
	HookDefaultLeaseSeconds int32 = 30
	HookDefaultRenewSeconds int32 = 10
)

func ValidateHookLeaseTiming(leaseSeconds, renewSeconds int32) error {
	if leaseSeconds <= 0 || renewSeconds <= 0 || int64(renewSeconds)*2 >= int64(leaseSeconds) {
		return invalidParam("invalid hook lease or renewal interval")
	}
	return nil
}

// HookRuntimeConfig contains validated process-local controls, not a Run snapshot.
type HookRuntimeConfig struct {
	AdmissionEnabled            bool
	WorkerEnabled               bool
	MQWakeEnabled               bool
	WorkerConcurrency           int32
	WorkspaceConcurrency        int32
	ScanIntervalSeconds         int32
	ScanBatchSize               int32
	LeaseSeconds                int32
	RenewSeconds                int32
	IdentityEnrichmentTimeoutMS int32
	RetentionDays               int32
}
