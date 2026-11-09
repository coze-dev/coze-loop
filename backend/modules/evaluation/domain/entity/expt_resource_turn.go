// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

const ResourceTurnStartExtKey = "__evaluation_resource_turn_start"

// ResourceTurnStart is scoped to the persisted business generation, not a transport delivery.
type ResourceTurnStart struct {
	ItemRunLogID int64  `json:"item_run_log_id"`
	RequestID    string `json:"request_id"`
	Claimed      bool   `json:"claimed"`
}
