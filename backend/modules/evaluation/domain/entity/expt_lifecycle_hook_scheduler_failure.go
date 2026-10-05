// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "time"

type HookSchedulerFailureInput struct {
	ExecutionScope    string
	Item              *HookTerminationItem
	Zombie            bool
	ZombieSeconds     int
	Async             bool
	ExpiredBefore     time.Time
	SandboxStatus     string
	ObservedTargetIDs []int64
}
