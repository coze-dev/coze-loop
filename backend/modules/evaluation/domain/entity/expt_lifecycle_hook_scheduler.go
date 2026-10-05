// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// Item identities are resolved from the original Run's frozen execution manifests.
type HookSchedulerDispatchInput struct {
	Key            HookRunKey
	ExecutionScope string
	ItemIDs        []int64
}
